package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/paveljanda/calvin/internal/calendar"
	"github.com/paveljanda/calvin/internal/config"
	"github.com/paveljanda/calvin/internal/render"
	"github.com/paveljanda/calvin/internal/weather"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	templatePath := flag.String("template", "templates/calendar.html", "Path to HTML template")
	outputPath := flag.String("output", "", "Output PNG path (overrides config)")
	dumpHTML := flag.Bool("dump-html", false, "Output HTML to path")
	listCalendars := flag.Bool("list-calendars", false, "List available calendars and exit")
	dryRun := flag.Bool("dry-run", false, "Don't shutdown or set alarm (for testing)")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Override output path if provided via flag
	if *outputPath != "" {
		cfg.Output.Path = *outputPath
	}

	ctx := context.Background()

	// Create Google Calendar client
	log.Println("Connecting to Google Calendar API...")
	calClient, err := calendar.NewClient(
		ctx,
		cfg.Calendar.CredentialsFile,
		cfg.Calendar.TokenFile,
		cfg.Weather.Timezone,
	)
	if err != nil {
		renderError(ctx, cfg, fmt.Errorf("failed to create calendar client: %w", err))
		log.Fatalf("Failed to create calendar client: %v", err)
	}

	// List calendars mode
	if *listCalendars {
		calendars, err := calClient.ListCalendars()
		if err != nil {
			renderError(ctx, cfg, fmt.Errorf("failed to list calendars: %w", err))
			log.Fatalf("Failed to list calendars: %v", err)
		}

		fmt.Println("\nAvailable calendars:")
		fmt.Println("─────────────────────────────────────────────────────────────")
		for _, cal := range calendars {
			fmt.Printf("  ID:    %s\n", cal.ID)
			fmt.Printf("  Name:  %s\n", cal.Name)
			fmt.Println("─────────────────────────────────────────────────────────────")
		}
		return
	}

	log.Printf("Calvin - E-Ink Calendar Generator")
	log.Printf("Display: %dx%d", cfg.Display.Width, cfg.Display.Height)
	log.Printf("Output: %s", cfg.Output.Path)

	// Fetch weather data
	log.Println("Fetching weather data...")
	weatherData, err := weather.Fetch(cfg.Weather.Latitude, cfg.Weather.Longitude, cfg.Weather.Timezone)
	if err != nil {
		log.Printf("Warning: Failed to fetch weather: %v", err)
		weatherData = nil
	}

	// Fetch calendar events from all configured calendars
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

	// Prepare template data for month view
	templateData := render.PrepareMonthData(
		cfg.Display.Width,
		cfg.Display.Height,
		weatherData,
		allEvents,
		cfg.Calendar.MaxEventsPerDay,
	)

	// Resolve template path
	absTemplatePath, err := filepath.Abs(*templatePath)
	if err != nil {
		renderError(ctx, cfg, fmt.Errorf("failed to resolve template path: %w", err))
		log.Fatalf("Failed to resolve template path: %v", err)
	}

	// Check if template exists
	if _, err := os.Stat(absTemplatePath); os.IsNotExist(err) {
		renderError(ctx, cfg, fmt.Errorf("template not found: %s", absTemplatePath))
		log.Fatalf("Template not found: %s", absTemplatePath)
	}

	// Render HTML
	log.Println("Rendering HTML...")
	html, err := render.RenderHTML(absTemplatePath, templateData)
	if err != nil {
		renderError(ctx, cfg, fmt.Errorf("failed to render HTML: %w", err))
		log.Fatalf("Failed to render HTML: %v", err)
	}

	if *dumpHTML {
		log.Println("Dumping HTML to file...")
		err := os.WriteFile("calendar.html", []byte(html), 0644)
		if err != nil {
			renderError(ctx, cfg, fmt.Errorf("failed to write HTML to file: %w", err))
			log.Fatalf("Error writing to file: %v", err)
		}
		log.Println("HTML dumped to calendar.html")
		return
	}

	// Convert HTML to PNG
	log.Println("Generating PNG with chromedp...")

	if err := render.HTMLToPNG(ctx, html, cfg.Display.Width, cfg.Display.Height, cfg.Output.Path); err != nil {
		renderError(ctx, cfg, fmt.Errorf("failed to generate PNG: %w", err))
		log.Fatalf("Failed to generate PNG: %v", err)
	}

	// Get output file info
	if info, err := os.Stat(cfg.Output.Path); err == nil {
		log.Printf("Generated: %s (%.1f KB)", cfg.Output.Path, float64(info.Size())/1024)
	}

	fmt.Println("✓ Calendar image generated successfully!")

	if !*dryRun {
		// Set PiSugar alarm for next hour (unless dry-run)
		if err := setPiSugarAlarm(); err != nil {
			renderError(ctx, cfg, fmt.Errorf("failed to set PiSugar alarm PNG: %w", err))
			log.Printf("Warning: Failed to set PiSugar alarm: %v", err)
		} else {
			log.Println("PiSugar alarm set for next hour at :00")
		}

		// Shutdown the system
		log.Println("Shutting down system...")
		cmd := exec.Command("sudo", "shutdown", "-h", "now")
		if err := cmd.Run(); err != nil {
			renderError(ctx, cfg, fmt.Errorf("failed to shutdown: %w", err))
			log.Fatalf("Failed to shutdown: %v", err)
		}
	} else {
		log.Println("Dry-run mode: skipping alarm and shutdown")
	}
}

// setPiSugarAlarm sets the PiSugar alarm for the next hour at :00
func setPiSugarAlarm() error {
	now := time.Now()
	// Calculate next hour at :00
	nextHour := now.Add(time.Hour).Truncate(time.Hour)

	// PiSugar alarm format: YYYY-MM-DD HH:MM:SS
	alarmTime := nextHour.Format("2006-01-02 15:04:05")

	log.Printf("Setting PiSugar alarm for: %s", alarmTime)

	// Call pisugar-cli to set alarm
	cmd := exec.Command("sudo", "pisugar-cli", "--set-alarm", alarmTime)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pisugar-cli failed: %w, output: %s", err, string(output))
	}

	log.Printf("PiSugar response: %s", string(output))
	return nil
}

// renderError renders an error to the PNG output for debugging
func renderError(ctx context.Context, cfg *config.Config, err error) {
	errorDetails := map[string]string{
		"Error":      err.Error(),
		"Time":       time.Now().Format("2006-01-02 15:04:05 MST"),
		"Args":       fmt.Sprintf("%v", os.Args),
		"Go Version": runtime.Version(),
		"OS/Arch":    fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}

	if renderErr := render.RenderErrorToPNG(ctx, cfg.Display.Width, cfg.Display.Height, err.Error(), errorDetails, cfg.Output.Path); renderErr != nil {
		log.Printf("Failed to render error to PNG: %v", renderErr)
	} else {
		log.Printf("Error details rendered to: %s", cfg.Output.Path)
	}
}
