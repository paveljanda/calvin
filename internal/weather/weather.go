package weather

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// HourlyForecast represents weather data for a single hour
type HourlyForecast struct {
	Time          time.Time
	Temperature   float64 // Celsius
	WeatherCode   int     // WMO weather code
	Precipitation float64 // mm
	WindSpeed     float64 // km/h
}

// Forecast contains weather forecast data
type Forecast struct {
	Hourly []HourlyForecast
}

// openMeteoResponse represents the API response from Open-Meteo
type openMeteoResponse struct {
	Hourly struct {
		Time          []string  `json:"time"`
		Temperature2m []float64 `json:"temperature_2m"`
		WeatherCode   []int     `json:"weather_code"`
		Precipitation []float64 `json:"precipitation"`
		WindSpeed10m  []float64 `json:"wind_speed_10m"`
	} `json:"hourly"`
}

// Fetch retrieves weather forecast from Open-Meteo API
func Fetch(lat, lon float64, timezone string) (*Forecast, error) {
	url := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%.4f&longitude=%.4f&hourly=temperature_2m,weather_code,precipitation,wind_speed_10m&timezone=%s&forecast_days=7",
		lat, lon, timezone,
	)

	resp, err := http.Get(url)
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

// GetNext24Hours returns hourly forecasts for the next 24 hours
func (f *Forecast) GetNext24Hours() []HourlyForecast {
	now := time.Now()
	var result []HourlyForecast

	for _, h := range f.Hourly {
		if h.Time.After(now) && h.Time.Before(now.Add(24*time.Hour)) {
			result = append(result, h)
		}
	}

	return result
}

// WeatherCodeToIcon converts WMO weather code to an emoji/icon
func WeatherCodeToIcon(code int) string {
	switch {
	case code == 0:
		return "☀️" // Clear sky
	case code == 1, code == 2, code == 3:
		return "⛅" // Partly cloudy
	case code >= 45 && code <= 48:
		return "🌫️" // Fog
	case code >= 51 && code <= 55:
		return "🌧️" // Drizzle
	case code >= 56 && code <= 57:
		return "🌧️" // Freezing drizzle
	case code >= 61 && code <= 65:
		return "🌧️" // Rain
	case code >= 66 && code <= 67:
		return "🌧️" // Freezing rain
	case code >= 71 && code <= 77:
		return "❄️" // Snow
	case code >= 80 && code <= 82:
		return "🌧️" // Rain showers
	case code >= 85 && code <= 86:
		return "❄️" // Snow showers
	case code >= 95 && code <= 99:
		return "⛈️" // Thunderstorm
	default:
		return "☁️"
	}
}

// WeatherCodeToDescription converts WMO weather code to text
func WeatherCodeToDescription(code int) string {
	switch {
	case code == 0:
		return "Clear"
	case code == 1:
		return "Mostly clear"
	case code == 2:
		return "Partly cloudy"
	case code == 3:
		return "Overcast"
	case code >= 45 && code <= 48:
		return "Fog"
	case code >= 51 && code <= 55:
		return "Drizzle"
	case code >= 56 && code <= 57:
		return "Freezing drizzle"
	case code >= 61 && code <= 65:
		return "Rain"
	case code >= 66 && code <= 67:
		return "Freezing rain"
	case code >= 71 && code <= 75:
		return "Snow"
	case code == 77:
		return "Snow grains"
	case code >= 80 && code <= 82:
		return "Rain showers"
	case code >= 85 && code <= 86:
		return "Snow showers"
	case code == 95:
		return "Thunderstorm"
	case code >= 96 && code <= 99:
		return "Thunderstorm with hail"
	default:
		return "Unknown"
	}
}

// GetDayTemperature returns the average temperature during day hours (12:00-18:00) for a given date
func (f *Forecast) GetDayTemperature(date time.Time) float64 {
	var sum float64
	var count int

	for _, h := range f.Hourly {
		if h.Time.Year() == date.Year() && h.Time.Month() == date.Month() && h.Time.Day() == date.Day() {
			hour := h.Time.Hour()
			if hour >= 12 && hour < 18 {
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

// GetNightTemperature returns the average temperature during night hours (00:00-06:00) for a given date
func (f *Forecast) GetNightTemperature(date time.Time) float64 {
	var sum float64
	var count int

	for _, h := range f.Hourly {
		if h.Time.Year() == date.Year() && h.Time.Month() == date.Month() && h.Time.Day() == date.Day() {
			hour := h.Time.Hour()
			if hour >= 0 && hour < 6 {
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


