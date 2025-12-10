package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type HourlyForecast struct {
	Time          time.Time
	Temperature   float64
	WeatherCode   int
	Precipitation float64
	WindSpeed     float64
}

type Forecast struct {
	Hourly []HourlyForecast
}

type openMeteoResponse struct {
	Hourly struct {
		Time          []string  `json:"time"`
		Temperature2m []float64 `json:"temperature_2m"`
		WeatherCode   []int     `json:"weather_code"`
		Precipitation []float64 `json:"precipitation"`
		WindSpeed10m  []float64 `json:"wind_speed_10m"`
	} `json:"hourly"`
}

func Fetch(lat, lon float64, timezone string) (*Forecast, error) {
	url := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%.4f&longitude=%.4f&hourly=temperature_2m,weather_code,precipitation,wind_speed_10m&timezone=%s&forecast_days=8",
		lat, lon, timezone,
	)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch weather: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("weather API returned status %d", resp.StatusCode)
	}

	var data openMeteoResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode weather response: %w", err)
	}

	forecast := &Forecast{
		Hourly: make([]HourlyForecast, 0, len(data.Hourly.Time)),
	}

	for i, timeStr := range data.Hourly.Time {
		t, err := time.Parse("2006-01-02T15:04", timeStr)
		if err != nil {
			continue
		}

		forecast.Hourly = append(forecast.Hourly, HourlyForecast{
			Time:          t,
			Temperature:   data.Hourly.Temperature2m[i],
			WeatherCode:   data.Hourly.WeatherCode[i],
			Precipitation: data.Hourly.Precipitation[i],
			WindSpeed:     data.Hourly.WindSpeed10m[i],
		})
	}

	return forecast, nil
}

func (f *Forecast) GetDayTemperature(date time.Time) float64 {
	return f.getAverageTemperature(date, 12, 18)
}

func (f *Forecast) GetNightTemperature(date time.Time) float64 {
	return f.getAverageTemperature(date, 0, 6)
}

func (f *Forecast) getAverageTemperature(date time.Time, startHour, endHour int) float64 {
	var sum float64
	var count int

	for _, h := range f.Hourly {
		if h.Time.Year() == date.Year() && h.Time.Month() == date.Month() && h.Time.Day() == date.Day() {
			hour := h.Time.Hour()
			if hour >= startHour && hour < endHour {
				sum += h.Temperature
				count++
			}
		}
	}

	if count == 0 {
		return 0
	}
	return sum / float64(count)
}
