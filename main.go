package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/paveljanda/calvin/internal/app"
	"github.com/paveljanda/calvin/internal/config"
	"github.com/paveljanda/calvin/internal/render"
	"github.com/paveljanda/calvin/internal/support"
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	listCalendars := flag.Bool("list-calendars", false, "List available calendars and exit")
	noShutdown := flag.Bool("no-shutdown", false, "Don't shutdown or set alarm (for testing) after app run")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	ctx := context.Background()

	if *listCalendars {
		err = support.ListCalendars(ctx, cfg)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		return
	}

	err = app.Run(ctx, cfg, *noShutdown)
	if err != nil {
		renderError(ctx, cfg, err)
		log.Fatalf("Error: %v", err)
	}
}

func renderError(ctx context.Context, cfg *config.Config, err error) {
	errorDetails := map[string]string{
		"Error":      err.Error(),
		"Time":       time.Now().Format("2006-01-02 15:04:05 MST"),
		"Args":       fmt.Sprintf("%v", os.Args),
		"Go Version": runtime.Version(),
		"OS/Arch":    fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}

	if renderErr := render.RenderErrorToPNG(cfg.Display.Width, cfg.Display.Height, err.Error(), errorDetails, cfg.Output.Path); renderErr != nil {
		log.Printf("Failed to render error to PNG: %v", renderErr)
	} else {
		log.Printf("Error details rendered to: %s", cfg.Output.Path)
	}
}
