# Calvin 📅

E-ink calendar display for Raspberry Pi Zero. Renders Google Calendar + weather forecast to PNG using HTML/CSS.

## Features

- 📅 Month view calendar with current month
- 🌡️ 8-day weather forecast (day/night temperatures shown in top-right corner of each day)
- 🎨 Optimized for Waveshare e-ink displays (4-color: white, black, red, grey)
- 📆 Multi-day events span across all days
- ⏰ Past events displayed in grey
- 🔴 Current/future event times shown in red

## Setup

### 1. Google Calendar API

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create project → Enable **Google Calendar API**
3. Create **OAuth client ID** (Desktop app)
4. Download JSON → save as `credentials.json`

### 2. Build & Run

```bash
go mod tidy
go build -o calvin .
cp config.example.yaml config.yaml
# Edit config.yaml with your location

./calvin  # First run opens auth flow
```

### 3. Cross-compile for Pi Zero

```bash
GOOS=linux GOARCH=arm GOARM=6 go build -o calvin-arm .
```

## Config

```yaml
display:
  width: 1304
  height: 984

weather:
  latitude: 49.9585
  longitude: 14.2888
  timezone: "Europe/Prague"

calendar:
  credentials_file: "credentials.json"
  token_file: "token.json"
  calendars:
    - id: "primary"
      name: "Personal"
  max_events_per_day: 10

output:
  path: "calendar.png"
```

## Commands

```bash
./calvin                  # Generate calendar.png
./calvin --list-calendars # Show available calendars
./calvin --dump-html      # Save html to a file for development purposes
```

## License

MIT
