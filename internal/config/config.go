package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Display  DisplayConfig  `yaml:"display"`
	Weather  WeatherConfig  `yaml:"weather"`
	Calendar CalendarConfig `yaml:"calendar"`
	Output   OutputConfig   `yaml:"output"`
}

type DisplayConfig struct {
	Width  int `yaml:"width"`
	Height int `yaml:"height"`
}

type WeatherConfig struct {
	Latitude  float64 `yaml:"latitude"`
	Longitude float64 `yaml:"longitude"`
	Timezone  string  `yaml:"timezone"`
}

type CalendarConfig struct {
	CredentialsFile string           `yaml:"credentials_file"`
	TokenFile       string           `yaml:"token_file"`
	Calendars       []CalendarSource `yaml:"calendars"`
	DaysAhead       int              `yaml:"days_ahead"`
	MaxEventsPerDay int              `yaml:"max_events_per_day"`
}

type CalendarSource struct {
	ID   string `yaml:"id"`   // Calendar ID ("primary" or email address)
	Name string `yaml:"name"` // Display name (optional, fetched from API if empty)
}

type OutputConfig struct {
	Path string `yaml:"path"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Set defaults
	if cfg.Display.Width == 0 {
		cfg.Display.Width = 800
	}
	if cfg.Display.Height == 0 {
		cfg.Display.Height = 480
	}
	if cfg.Calendar.DaysAhead == 0 {
		cfg.Calendar.DaysAhead = 7
	}
	if cfg.Calendar.MaxEventsPerDay == 0 {
		cfg.Calendar.MaxEventsPerDay = 10
	}
	if cfg.Calendar.CredentialsFile == "" {
		cfg.Calendar.CredentialsFile = "credentials.json"
	}
	if cfg.Calendar.TokenFile == "" {
		cfg.Calendar.TokenFile = "token.json"
	}
	if cfg.Output.Path == "" {
		cfg.Output.Path = "calendar.png"
	}
	if cfg.Weather.Timezone == "" {
		cfg.Weather.Timezone = "UTC"
	}

	// Default to primary calendar if none specified
	if len(cfg.Calendar.Calendars) == 0 {
		cfg.Calendar.Calendars = []CalendarSource{
			{ID: "primary", Name: "Primary"},
		}
	}

	return &cfg, nil
}
