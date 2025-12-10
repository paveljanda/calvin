package render

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"
	"net/url"
	"os"
	"time"

	"github.com/chromedp/chromedp"

	"github.com/paveljanda/calvin/internal/calendar"
	"github.com/paveljanda/calvin/internal/weather"
)

//go:embed templates/*.html
var templatesFS embed.FS

type TemplateData struct {
	Width        int
	Height       int
	MonthName    string
	Year         int
	GeneratedAt  string
	WeatherError string
	Weeks        []WeekData
}

type ErrorTemplateData struct {
	Width       int
	Height      int
	ErrorMsg    string
	Details     map[string]string
	GeneratedAt string
}

type WeekData struct {
	Days []DayData
}

type DayData struct {
	Date           string
	DayNum         string
	MonthShort     string
	IsToday        bool
	IsPast         bool
	IsWeekend      bool
	IsCurrentMonth bool
	DayTemp        string
	NightTemp      string
	Events         []EventData
}

type EventData struct {
	Time    string
	Summary string
	AllDay  bool
}

func RenderHTML(templateName string, data any) (string, error) {
	funcMap := template.FuncMap{
		"safe": func(s string) template.HTML {
			return template.HTML(s)
		},
	}

	tmpl, err := template.New(templateName).Funcs(funcMap).ParseFS(templatesFS, "templates/"+templateName)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

func HTMLToPNG(ctx context.Context, html string, width, height int, outputPath string) error {
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

	chromeCtx, cancel := context.WithTimeout(chromeCtx, 30*time.Second)
	defer cancel()

	var buf []byte
	dataURL := "data:text/html;charset=utf-8," + url.PathEscape(html)

	err := chromedp.Run(chromeCtx,
		chromedp.Navigate(dataURL),
		chromedp.EmulateViewport(int64(width), int64(height)),
		chromedp.WaitReady("body"),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.FullScreenshot(&buf, 100),
	)
	if err != nil {
		return fmt.Errorf("chromedp failed: %w", err)
	}

	if err := os.WriteFile(outputPath, buf, 0644); err != nil {
		return fmt.Errorf("failed to write PNG: %w", err)
	}

	return nil
}

func RenderErrorToPNG(ctx context.Context, width, height int, errorMsg string, errorDetails map[string]string, outputPath string) error {
	data := ErrorTemplateData{
		Width:       width,
		Height:      height,
		ErrorMsg:    errorMsg,
		Details:     errorDetails,
		GeneratedAt: time.Now().Format("2006-01-02 15:04:05 MST"),
	}

	html, err := RenderHTML("error.html", data)
	if err != nil {
		return fmt.Errorf("failed to render error HTML: %w", err)
	}

	return HTMLToPNG(ctx, html, width, height, outputPath)
}

func PrepareMonthData(width, height int, weatherData *weather.Forecast, weatherErr error, events []calendar.Event, maxEventsPerDay int) TemplateData {
	now := time.Now()

	weatherError := ""
	if weatherErr != nil {
		weatherError = fmt.Sprintf("Weather: %v", weatherErr)
	}

	data := TemplateData{
		Width:        width,
		Height:       height,
		MonthName:    now.Month().String(),
		Year:         now.Year(),
		GeneratedAt:  now.Format("2006-01-02 15:04:05"),
		WeatherError: weatherError,
		Weeks:        buildWeeks(now, buildEventsByDate(events), weatherData, maxEventsPerDay),
	}

	return data
}

func buildEventsByDate(events []calendar.Event) map[string][]calendar.Event {
	eventsByDate := make(map[string][]calendar.Event)

	for _, event := range events {
		startDate := time.Date(event.Start.Year(), event.Start.Month(), event.Start.Day(), 0, 0, 0, 0, event.Start.Location())
		endDate := time.Date(event.End.Year(), event.End.Month(), event.End.Day(), 0, 0, 0, 0, event.End.Location())

		if event.AllDay && endDate.After(startDate) {
			endDate = endDate.AddDate(0, 0, -1)
		}

		for currentDate := startDate; currentDate.Before(endDate) || currentDate.Equal(endDate); currentDate = currentDate.AddDate(0, 0, 1) {
			dateKey := currentDate.Format("2006-01-02")
			eventsByDate[dateKey] = append(eventsByDate[dateKey], event)
		}
	}

	return eventsByDate
}

func buildWeeks(now time.Time, eventsByDate map[string][]calendar.Event, weatherData *weather.Forecast, maxEventsPerDay int) []WeekData {
	startDate, endDate := getMonthGridRange(now)
	currentMonth := now.Month()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	var weeks []WeekData
	currentDate := startDate

	for currentDate.Before(endDate) || currentDate.Equal(endDate) {
		week := WeekData{Days: make([]DayData, 0, 7)}

		for i := 0; i < 7; i++ {
			dayData := buildDayData(currentDate, today, currentMonth, eventsByDate, weatherData, maxEventsPerDay)
			week.Days = append(week.Days, dayData)
			currentDate = currentDate.AddDate(0, 0, 1)
		}

		weeks = append(weeks, week)
	}

	return weeks
}

func buildDayData(date, today time.Time, currentMonth time.Month, eventsByDate map[string][]calendar.Event, weatherData *weather.Forecast, maxEventsPerDay int) DayData {
	dateKey := date.Format("2006-01-02")
	dayEvents := calendar.SortEvents(eventsByDate[dateKey])

	if len(dayEvents) > maxEventsPerDay {
		dayEvents = dayEvents[:maxEventsPerDay]
	}

	templateEvents := make([]EventData, 0, len(dayEvents))
	for _, ev := range dayEvents {
		eventData := EventData{Summary: ev.Summary, AllDay: ev.AllDay}
		if !ev.AllDay {
			eventData.Time = ev.Start.Format("15:04")
		}
		templateEvents = append(templateEvents, eventData)
	}

	dayTemp, nightTemp := getTemperatures(date, today, weatherData)

	return DayData{
		Date:           dateKey,
		DayNum:         date.Format("2"),
		MonthShort:     date.Format("Jan"),
		IsToday:        calendar.IsToday(date),
		IsPast:         date.Before(today),
		IsWeekend:      calendar.IsWeekend(date),
		IsCurrentMonth: date.Month() == currentMonth,
		DayTemp:        dayTemp,
		NightTemp:      nightTemp,
		Events:         templateEvents,
	}
}

func getTemperatures(date, today time.Time, weatherData *weather.Forecast) (string, string) {
	if weatherData == nil {
		return "", ""
	}

	eightDaysFromNow := today.AddDate(0, 0, 8)
	if date.Before(today) || !date.Before(eightDaysFromNow) {
		return "", ""
	}

	dayTempValue := weatherData.GetDayTemperature(date)
	nightTempValue := weatherData.GetNightTemperature(date)

	if dayTempValue == 0 && nightTempValue == 0 {
		return "", ""
	}

	return fmt.Sprintf("%.0f°", dayTempValue), fmt.Sprintf("%.0f°", nightTempValue)
}

func getMonthGridRange(now time.Time) (time.Time, time.Time) {
	firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	lastOfMonth := firstOfMonth.AddDate(0, 1, -1)

	startDate := firstOfMonth.AddDate(0, 0, -(mondayWeekday(firstOfMonth) - 1))
	endDate := lastOfMonth.AddDate(0, 0, 7-mondayWeekday(lastOfMonth))

	return startDate, endDate
}

func mondayWeekday(t time.Time) int {
	weekday := int(t.Weekday())
	if weekday == 0 {
		return 7
	}
	return weekday
}
