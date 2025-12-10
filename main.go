package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

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
		log.Fatalf("Failed to create calendar client: %v", err)
	}

	// List calendars mode
	if *listCalendars {
		calendars, err := calClient.ListCalendars(ctx)
		if err != nil {
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
		log.Fatalf("Failed to resolve template path: %v", err)
	}

	// Check if template exists
	if _, err := os.Stat(absTemplatePath); os.IsNotExist(err) {
		log.Fatalf("Template not found: %s", absTemplatePath)
	}

	// Render HTML
	log.Println("Rendering HTML...")
	html, err := render.RenderHTML(absTemplatePath, templateData)
	if err != nil {
		log.Fatalf("Failed to render HTML: %v", err)
	}

	if *dumpHTML {
		log.Println("Dumping HTML to file...")
		err := os.WriteFile("calendar.html", []byte(html), 0644)
		if err != nil {
			log.Fatalf("Error writing to file: %v", err)
			return
		}
		return
	}

	// Convert HTML to PNG
	log.Println("Generating PNG with chromedp...")

	if err := render.HTMLToPNG(ctx, html, cfg.Display.Width, cfg.Display.Height, cfg.Output.Path); err != nil {
		log.Fatalf("Failed to generate PNG: %v", err)
	}

	// Get output file info
	if info, err := os.Stat(cfg.Output.Path); err == nil {
		log.Printf("Generated: %s (%.1f KB)", cfg.Output.Path, float64(info.Size())/1024)
	}

	fmt.Println("✓ Calendar image generated successfully!")
}
