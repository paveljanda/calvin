# Calvin 📅

E-ink calendar display for Raspberry Pi Zero. Renders Google Calendar + weather to PNG using HTML/CSS.

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
  latitude: 50.0755
  longitude: 14.4378
  timezone: "Europe/Prague"

calendar:
  credentials_file: "credentials.json"
  token_file: "token.json"
  calendars:
    - id: "primary"
      name: "Personal"
      color: "#2563eb"
  days_ahead: 7
  max_events_per_day: 10

output:
  path: "calendar.png"
```

## Commands

```bash
./calvin                  # Generate calendar.png
./calvin --list-calendars # Show available calendars
```

## License

MIT
