package render

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/chromedp/chromedp"

	"github.com/paveljanda/calvin/internal/calendar"
	"github.com/paveljanda/calvin/internal/weather"
)

// TemplateData contains all data passed to the HTML template
type TemplateData struct {
	Width       int
	Height      int
	GeneratedAt time.Time
	MonthName   string
	Year        int
	Weeks       []WeekData
}

// WeekData represents a single week row in the calendar
type WeekData struct {
	Days []DayData
}

// DayData represents a single day for template rendering
type DayData struct {
	Date           string
	DayNum         string
	MonthShort     string
	IsToday        bool
	IsWeekend      bool
	IsCurrentMonth bool
	DayTemp        string
	NightTemp      string
	Events         []EventData
}

// EventData represents a single event for template rendering
type EventData struct {
	Time    string
	Summary string
	AllDay  bool
}

// RenderHTML generates HTML from template and data
func RenderHTML(templatePath string, data TemplateData) (string, error) {
	funcMap := template.FuncMap{
		"safe": func(s string) template.HTML {
			return template.HTML(s)
		},
	}

	tmpl, err := template.New(filepath.Base(templatePath)).Funcs(funcMap).ParseFiles(templatePath)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// HTMLToPNG converts HTML content to a PNG image using chromedp
func HTMLToPNG(ctx context.Context, html string, width, height int, outputPath string) error {
	// Create chromedp context with options suitable for headless rendering
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.WindowSize(width, height),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, opts...)
	defer allocCancel()

	chromeCtx, chromeCancel := chromedp.NewContext(allocCtx)
	defer chromeCancel()

	// Set timeout
	chromeCtx, cancel := context.WithTimeout(chromeCtx, 30*time.Second)
	defer cancel()

	var buf []byte

	// Navigate to data URL with our HTML (URL-encoded to handle special characters)
	dataURL := "data:text/html;charset=utf-8," + url.PathEscape(html)

	err := chromedp.Run(chromeCtx,
		chromedp.Navigate(dataURL),
		chromedp.EmulateViewport(int64(width), int64(height)),
		chromedp.WaitReady("body"),
		chromedp.Sleep(500*time.Millisecond), // Allow fonts to load
		chromedp.FullScreenshot(&buf, 100),
	)
	if err != nil {
		return fmt.Errorf("chromedp failed: %w", err)
	}

	// Write PNG to file
	if err := os.WriteFile(outputPath, buf, 0644); err != nil {
		return fmt.Errorf("failed to write PNG: %w", err)
	}

	return nil
}

// PrepareMonthData prepares calendar data for month view rendering
func PrepareMonthData(
	width, height int,
	weatherData *weather.Forecast,
	events []calendar.Event,
	maxEventsPerDay int,
) TemplateData {
	now := time.Now()
	currentMonth := now.Month()
	currentYear := now.Year()

	data := TemplateData{
		Width:       width,
		Height:      height,
		GeneratedAt: now,
		MonthName:   currentMonth.String(),
		Year:        currentYear,
		Weeks:       make([]WeekData, 0),
	}

	// Build events map by date
	eventsByDate := make(map[string][]calendar.Event)
	for _, event := range events {
		dateKey := event.Start.Format("2006-01-02")
		eventsByDate[dateKey] = append(eventsByDate[dateKey], event)
	}

	// Find first day of month and calculate start of calendar grid
	firstOfMonth := time.Date(currentYear, currentMonth, 1, 0, 0, 0, 0, now.Location())

	// Start from Monday of the week containing the first of month
	// Go's Weekday: Sunday = 0, Monday = 1, ..., Saturday = 6
	// We want Monday = 0, so we adjust
	weekday := int(firstOfMonth.Weekday())
	if weekday == 0 {
		weekday = 7 // Sunday becomes 7
	}
	startDate := firstOfMonth.AddDate(0, 0, -(weekday - 1))

	// Find last day of month
	lastOfMonth := firstOfMonth.AddDate(0, 1, -1)

	// End on Sunday of the week containing the last of month
	weekday = int(lastOfMonth.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	endDate := lastOfMonth.AddDate(0, 0, 7-weekday)

	// Build weeks
	currentDate := startDate
	for currentDate.Before(endDate) || currentDate.Equal(endDate) {
		week := WeekData{Days: make([]DayData, 0, 7)}

		for i := 0; i < 7; i++ {
			dateKey := currentDate.Format("2006-01-02")
			dayEvents := eventsByDate[dateKey]

			// Sort and limit events
			dayEvents = calendar.SortEvents(dayEvents)
			if len(dayEvents) > maxEventsPerDay {
				dayEvents = dayEvents[:maxEventsPerDay]
			}

			// Convert to template events
			templateEvents := make([]EventData, 0, len(dayEvents))
			for _, ev := range dayEvents {
				eventData := EventData{
					Summary: ev.Summary,
					AllDay:  ev.AllDay,
				}
				if !ev.AllDay {
					eventData.Time = ev.Start.Format("15:04")
				}
				templateEvents = append(templateEvents, eventData)
			}

			// Get temperatures from weather data (only for next 8 days starting from today)
			dayTemp := ""
			nightTemp := ""
			today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			eightDaysFromNow := today.AddDate(0, 0, 8)

			if weatherData != nil && (currentDate.Equal(today) || currentDate.After(today)) && currentDate.Before(eightDaysFromNow) {
				dayTempValue := weatherData.GetDayTemperature(currentDate)
				nightTempValue := weatherData.GetNightTemperature(currentDate)
				if dayTempValue != 0 || nightTempValue != 0 {
					dayTemp = fmt.Sprintf("%.0f°", dayTempValue)
					nightTemp = fmt.Sprintf("%.0f°", nightTempValue)
				}
			}

			dayData := DayData{
				Date:           dateKey,
				DayNum:         currentDate.Format("2"),
				MonthShort:     currentDate.Format("Jan"),
				IsToday:        calendar.IsToday(currentDate),
				IsWeekend:      calendar.IsWeekend(currentDate),
				IsCurrentMonth: currentDate.Month() == currentMonth,
				DayTemp:        dayTemp,
				NightTemp:      nightTemp,
				Events:         templateEvents,
			}

			week.Days = append(week.Days, dayData)
			currentDate = currentDate.AddDate(0, 0, 1)
		}

		data.Weeks = append(data.Weeks, week)
	}

	return data
}
