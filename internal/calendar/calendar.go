package calendar

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	gcal "google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

type Event struct {
	Summary      string
	Description  string
	Location     string
	Start        time.Time
	End          time.Time
	AllDay       bool
	CalendarName string
}

type DayEvents struct {
	Date   time.Time
	Events []Event
}

type CalendarConfig struct {
	ID   string
	Name string
}

type Client struct {
	service  *gcal.Service
	location *time.Location
}

func NewClient(ctx context.Context, credentialsPath, tokenPath string, timezone string) (*Client, error) {
	credBytes, err := os.ReadFile(credentialsPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read credentials file: %w", err)
	}

	config, err := google.ConfigFromJSON(credBytes, gcal.CalendarReadonlyScope)
	if err != nil {
		return nil, fmt.Errorf("unable to parse credentials: %w", err)
	}

	token, err := tokenFromFile(tokenPath)
	if err != nil {
		token, err = getTokenFromWeb(ctx, config)
		if err != nil {
			return nil, fmt.Errorf("unable to get token: %w", err)
		}
		if err := saveToken(tokenPath, token); err != nil {
			return nil, fmt.Errorf("unable to save token: %w", err)
		}
	}

	httpClient := config.Client(ctx, token)
	httpClient.Timeout = 30 * time.Second

	service, err := gcal.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("unable to create calendar service: %w", err)
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.Local
	}

	return &Client{
		service:  service,
		location: loc,
	}, nil
}

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

func getTokenFromWeb(ctx context.Context, config *oauth2.Config) (*oauth2.Token, error) {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)

	log.Println("╔════════════════════════════════════════════════════════════════╗")
	log.Println("║              Google Calendar Authorization Required            ║")
	log.Println("╠════════════════════════════════════════════════════════════════╣")
	log.Println("║ Go to the following link in your browser:                      ║")
	log.Println("╚════════════════════════════════════════════════════════════════╝")
	log.Println()
	log.Println(authURL)
	log.Println()
	log.Print("Enter the authorization code: ")

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

func saveToken(path string, token *oauth2.Token) error {
	log.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("unable to create token file: %w", err)
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(token)
}

func (c *Client) FetchEventsForMonth(ctx context.Context, calendarID string, calendarName string) ([]Event, error) {
	startDate, endDate := c.getMonthDateRange()

	events, err := c.service.Events.List(calendarID).
		ShowDeleted(false).
		SingleEvents(true).
		TimeMin(startDate.Format(time.RFC3339)).
		TimeMax(endDate.Format(time.RFC3339)).
		OrderBy("startTime").
		Do()
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve events: %w", err)
	}

	result := make([]Event, 0, len(events.Items))
	for _, item := range events.Items {
		result = append(result, c.parseGoogleEvent(item, calendarName))
	}

	return result, nil
}

func (c *Client) getMonthDateRange() (time.Time, time.Time) {
	now := time.Now().In(c.location)
	firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, c.location)
	lastOfMonth := firstOfMonth.AddDate(0, 1, -1)

	startDate := firstOfMonth.AddDate(0, 0, -(mondayWeekday(firstOfMonth) - 1))
	endDate := lastOfMonth.AddDate(0, 0, 7-mondayWeekday(lastOfMonth)+1)

	return startDate, endDate
}

func (c *Client) parseGoogleEvent(item *gcal.Event, calendarName string) Event {
	event := Event{
		Summary:      item.Summary,
		Description:  item.Description,
		Location:     item.Location,
		CalendarName: calendarName,
	}

	if item.Start.DateTime != "" {
		if t, err := time.Parse(time.RFC3339, item.Start.DateTime); err == nil {
			event.Start = t.In(c.location)
		}
		event.AllDay = false
	} else if item.Start.Date != "" {
		if t, err := time.Parse("2006-01-02", item.Start.Date); err == nil {
			event.Start = t
		}
		event.AllDay = true
	}

	if item.End.DateTime != "" {
		if t, err := time.Parse(time.RFC3339, item.End.DateTime); err == nil {
			event.End = t.In(c.location)
		}
	} else if item.End.Date != "" {
		if t, err := time.Parse("2006-01-02", item.End.Date); err == nil {
			event.End = t
		}
	}

	return event
}

func mondayWeekday(t time.Time) int {
	weekday := int(t.Weekday())
	if weekday == 0 {
		return 7
	}
	return weekday
}

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

func IsToday(t time.Time) bool {
	now := time.Now()
	return t.Year() == now.Year() && t.YearDay() == now.YearDay()
}

func IsWeekend(t time.Time) bool {
	day := t.Weekday()
	return day == time.Saturday || day == time.Sunday
}
