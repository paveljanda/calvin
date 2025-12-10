package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/paveljanda/calvin/internal/calendar"
	"github.com/paveljanda/calvin/internal/config"
	"github.com/paveljanda/calvin/internal/render"
	"github.com/paveljanda/calvin/internal/weather"
)

func Run(ctx context.Context, cfg *config.Config, dumpHTML bool, noShutdown bool) error {
	log.Println("Connecting to Google Calendar API...")
	calClient, err := calendar.NewClient(ctx, cfg.Calendar.CredentialsFile, cfg.Calendar.TokenFile, cfg.Weather.Timezone)
	if err != nil {
		return fmt.Errorf("failed to create calendar client: %w", err)
	}

	log.Printf("Calvin - E-Ink Calendar Generator")
	log.Printf("Display: %dx%d", cfg.Display.Width, cfg.Display.Height)
	log.Printf("Output: %s", cfg.Output.Path)

	log.Println("Fetching weather data...")
	weatherData, weatherErr := weather.Fetch(cfg.Weather.Latitude, cfg.Weather.Longitude, cfg.Weather.Timezone)
	if weatherErr != nil {
		log.Printf("Warning: Failed to fetch weather: %v", weatherErr)
	}

	allEvents, err := fetchAllCalendarEvents(ctx, cfg, calClient)
	if err != nil {
		return err
	}

	html, err := generateHTML(cfg, weatherData, weatherErr, allEvents)
	if err != nil {
		return err
	}

	if dumpHTML {
		log.Println("Dumping HTML to file...")
		err := os.WriteFile("calendar.html", []byte(html), 0644)
		if err != nil {
			return fmt.Errorf("failed to write HTML to file: %w", err)
		}
		log.Println("HTML dumped to calendar.html")
		return nil
	}

	err = generatePNG(ctx, cfg, html)
	if err != nil {
		return err
	}

	if noShutdown {
		log.Println("Dry-run or list-calendars mode: skipping alarm and shutdown")
		return nil
	}

	err = handlePiSugar(ctx)
	if err != nil {
		return err
	}

	log.Println("Shutting down system...")
	if err := exec.Command("sudo", "shutdown", "-h", "now").Run(); err != nil {
		return fmt.Errorf("failed to shutdown: %w", err)
	}

	return nil
}

func handlePiSugar(ctx context.Context) error {
	nextHour := time.Now().Add(time.Hour).Truncate(time.Hour)
	alarmTime := nextHour.Format("2006-01-02 15:04:05")

	log.Printf("Setting PiSugar alarm for: %s", alarmTime)

	output, err := exec.CommandContext(ctx, "sudo", "pisugar-cli", "--set-alarm", alarmTime).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set PiSugar alarm: pisugar-cli failed: %w, output: %s", err, string(output))
	}

	log.Printf("PiSugar response: %s", string(output))

	return nil
}

func fetchAllCalendarEvents(ctx context.Context, cfg *config.Config, calClient *calendar.Client) ([]calendar.Event, error) {
	log.Println("Fetching calendar events for month view...")
	var allEvents []calendar.Event

	for _, calCfg := range cfg.Calendar.Calendars {
		name := calCfg.Name
		if name == "" {
			name = calCfg.ID
		}
		log.Printf("  Fetching: %s", name)

		events, err := calClient.FetchEventsForMonth(ctx, calCfg.ID, name)
		if err != nil {
			log.Printf("  Warning: Failed to fetch %s: %v", name, err)
			continue
		}
		log.Printf("  Found %d events", len(events))
		allEvents = append(allEvents, events...)
	}

	return allEvents, nil
}

func generateHTML(cfg *config.Config, weatherData *weather.Forecast, weatherErr error, allEvents []calendar.Event) (string, error) {
	templateData := render.PrepareMonthData(cfg.Display.Width, cfg.Display.Height, weatherData, weatherErr, allEvents, cfg.Calendar.MaxEventsPerDay)

	log.Println("Rendering HTML...")
	html, err := render.RenderHTML("calendar.html", templateData)
	if err != nil {
		return "", fmt.Errorf("failed to render HTML: %w", err)
	}

	return html, nil
}

func generatePNG(ctx context.Context, cfg *config.Config, html string) error {
	log.Println("Generating PNG with chromedp...")

	if err := render.HTMLToPNG(ctx, html, cfg.Display.Width, cfg.Display.Height, cfg.Output.Path); err != nil {
		return fmt.Errorf("failed to generate PNG: %w", err)
	}

	if info, err := os.Stat(cfg.Output.Path); err == nil {
		log.Printf("Generated: %s (%.1f KB)", cfg.Output.Path, float64(info.Size())/1024)
	}

	log.Println("Calendar image generated successfully!")

	return nil
}
