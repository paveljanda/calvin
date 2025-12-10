package support

import (
	"context"
	"fmt"
	"log"

	"github.com/paveljanda/calvin/internal/calendar"
	"github.com/paveljanda/calvin/internal/config"
)

func ListCalendars(ctx context.Context, cfg *config.Config) error {
	calClient, err := calendar.NewClient(ctx, cfg.Calendar.CredentialsFile, cfg.Calendar.TokenFile, cfg.Weather.Timezone)
	if err != nil {
		return fmt.Errorf("failed to create calendar client: %w", err)
	}

	calendars, err := calClient.ListCalendars()
	if err != nil {
		return fmt.Errorf("failed to list calendars: %w", err)
	}

	log.Println("\nAvailable calendars:")
	log.Println("─────────────────────────────────────────────────────────────")
	for _, cal := range calendars {
		log.Printf("  ID:    %s\n", cal.ID)
		log.Printf("  Name:  %s\n", cal.Name)
		log.Println("─────────────────────────────────────────────────────────────")
	}

	return nil
}
