package calendar

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	gcal "google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// Event represents a calendar event
type Event struct {
	Summary      string
	Description  string
	Location     string
	Start        time.Time
	End          time.Time
	AllDay       bool
	CalendarName string
}

// DayEvents groups events by date
type DayEvents struct {
	Date   time.Time
	Events []Event
}

// CalendarConfig holds configuration for a single calendar
type CalendarConfig struct {
	ID   string // Calendar ID (e.g., "primary" or email address)
	Name string // Display name
}

// Client wraps Google Calendar API client
type Client struct {
	service  *gcal.Service
	location *time.Location
}

// NewClient creates a new Google Calendar API client
func NewClient(ctx context.Context, credentialsPath, tokenPath string, timezone string) (*Client, error) {
	// Load credentials.json
	credBytes, err := os.ReadFile(credentialsPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read credentials file: %w", err)
	}

	// Parse credentials - support both OAuth client and service account
	config, err := google.ConfigFromJSON(credBytes, gcal.CalendarReadonlyScope)
	if err != nil {
		return nil, fmt.Errorf("unable to parse credentials: %w", err)
	}

	// Get OAuth token
	token, err := tokenFromFile(tokenPath)
	if err != nil {
		// Token doesn't exist, need to get one
		token, err = getTokenFromWeb(ctx, config)
		if err != nil {
			return nil, fmt.Errorf("unable to get token: %w", err)
		}
		if err := saveToken(tokenPath, token); err != nil {
			return nil, fmt.Errorf("unable to save token: %w", err)
		}
	}

	// Create HTTP client with token and timeout
	httpClient := config.Client(ctx, token)
	httpClient.Timeout = 30 * time.Second

	// Create Calendar service
	service, err := gcal.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("unable to create calendar service: %w", err)
	}

	// Parse timezone
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.Local
	}

	return &Client{
		service:  service,
		location: loc,
	}, nil
}

// tokenFromFile retrieves a token from a local file
func tokenFromFile(path string) (*oauth2.Token, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	token := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(token)
	return token, err
}

// getTokenFromWeb requests a token from the web, then returns the retrieved token
func getTokenFromWeb(ctx context.Context, config *oauth2.Config) (*oauth2.Token, error) {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)

	fmt.Println("╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("║              Google Calendar Authorization Required            ║")
	fmt.Println("╠════════════════════════════════════════════════════════════════╣")
	fmt.Println("║ Go to the following link in your browser:                      ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println(authURL)
	fmt.Println()
	fmt.Print("Enter the authorization code: ")

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		return nil, fmt.Errorf("unable to read authorization code: %w", err)
	}

	token, err := config.Exchange(ctx, authCode)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve token from web: %w", err)
	}

	return token, nil
}

// saveToken saves a token to a file path
func saveToken(path string, token *oauth2.Token) error {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("unable to create token file: %w", err)
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(token)
}

// FetchEventsForMonth retrieves events for the entire month view (including padding days)
func (c *Client) FetchEventsForMonth(ctx context.Context, calendarID string, calendarName string) ([]Event, error) {
	now := time.Now().In(c.location)

	// Get first day of current month
	firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, c.location)

	// Start from Monday of the week containing the first of month
	weekday := int(firstOfMonth.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	startDate := firstOfMonth.AddDate(0, 0, -(weekday - 1))

	// Find last day of month
	lastOfMonth := firstOfMonth.AddDate(0, 1, -1)

	// End on Sunday of the week containing the last of month
	weekday = int(lastOfMonth.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	endDate := lastOfMonth.AddDate(0, 0, 7-weekday+1) // +1 to include the last day

	timeMin := startDate.Format(time.RFC3339)
	timeMax := endDate.Format(time.RFC3339)

	events, err := c.service.Events.List(calendarID).
		ShowDeleted(false).
		SingleEvents(true).
		TimeMin(timeMin).
		TimeMax(timeMax).
		OrderBy("startTime").
		Do()
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve events: %w", err)
	}

	var result []Event

	for _, item := range events.Items {
		event := Event{
			Summary:      item.Summary,
			Description:  item.Description,
			Location:     item.Location,
			CalendarName: calendarName,
		}

		// Parse start time
		if item.Start.DateTime != "" {
			// Timed event
			t, err := time.Parse(time.RFC3339, item.Start.DateTime)
			if err == nil {
				event.Start = t.In(c.location)
			}
			event.AllDay = false
		} else if item.Start.Date != "" {
			// All-day event
			t, err := time.Parse("2006-01-02", item.Start.Date)
			if err == nil {
				event.Start = t
			}
			event.AllDay = true
		}

		// Parse end time
		if item.End.DateTime != "" {
			t, err := time.Parse(time.RFC3339, item.End.DateTime)
			if err == nil {
				event.End = t.In(c.location)
			}
		} else if item.End.Date != "" {
			t, err := time.Parse("2006-01-02", item.End.Date)
			if err == nil {
				event.End = t
			}
		}

		result = append(result, event)
	}

	return result, nil
}

// ListCalendars returns all accessible calendars
func (c *Client) ListCalendars() ([]CalendarConfig, error) {
	calendarList, err := c.service.CalendarList.List().Do()
	if err != nil {
		return nil, fmt.Errorf("unable to list calendars: %w", err)
	}

	var calendars []CalendarConfig
	for _, item := range calendarList.Items {
		calendars = append(calendars, CalendarConfig{
			ID:   item.Id,
			Name: item.Summary,
		})
	}

	return calendars, nil
}

// SortEvents sorts events: all-day first, then by start time
func SortEvents(events []Event) []Event {
	sorted := make([]Event, len(events))
	copy(sorted, events)

	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].AllDay && !sorted[j].AllDay {
			return true
		}
		if !sorted[i].AllDay && sorted[j].AllDay {
			return false
		}
		return sorted[i].Start.Before(sorted[j].Start)
	})

	return sorted
}

// IsToday checks if a date is today
func IsToday(t time.Time) bool {
	now := time.Now()
	return t.Year() == now.Year() && t.YearDay() == now.YearDay()
}

// IsWeekend checks if a date is Saturday or Sunday
func IsWeekend(t time.Time) bool {
	day := t.Weekday()
	return day == time.Saturday || day == time.Sunday
}
