# Analytics Backend

Privacy-focused web analytics backend in Go. Stores events from the frontend analytics module and serves aggregated data for dashboards.

## Features

- **Privacy-first**: No IP logging, configurable data retention
- **SQLite storage**: Single file database, no external dependencies
- **Batch ingestion**: Efficient event processing
- **Dashboard API**: Ready-to-use endpoints for analytics dashboards
- **Authentication**: Optional API key protection for ingest and dashboard
- **Rate limiting**: Built-in protection against abuse
- **Graceful shutdown**: Clean handling of SIGINT/SIGTERM

## Quick Start

```bash
# Build
go build -o analytics ./cmd/analytics

# Run (defaults)
./analytics

# Run with custom config
ANALYTICS_ADDR=:3001 \
ANALYTICS_DB_PATH=./data/analytics.db \
ANALYTICS_API_KEY=your-secret-key \
./analytics
```

## Configuration

| Environment Variable | Flag | Default | Description |
|----------------------|------|---------|-------------|
| `ANALYTICS_ADDR` | `-addr` | `:3001` | Server address |
| `ANALYTICS_DB_PATH` | `-db` | `./analytics.db` | SQLite database path |
| `ANALYTICS_API_KEY` | `-api-key` | (none) | API key for ingest endpoint |
| `ANALYTICS_DASHBOARD_API_KEY` | `-dashboard-key` | (none) | API key for dashboard endpoints |
| `ANALYTICS_RETENTION_DAYS` | `-retention-days` | `90` | Days to retain raw events |

## API Endpoints

### Ingest

**POST /api/analytics**

Accepts a JSON array of analytics events:

```json
[
  {
    "sessionId": "uuid-v4",
    "type": "pageview",
    "url": "/about",
    "referrer": "https://example.com",
    "title": "About Us",
    "userAgent": "Mozilla/5.0...",
    "timestamp": 1704067200000,
    "data": {
      "screenWidth": 1920,
      "screenHeight": 1080,
      "viewportWidth": 1200,
      "viewportHeight": 800,
      "scrollDepth": 75,
      "engagementTime": 30
    }
  }
]
```

Response:

```json
{
  "success": true,
  "count": 1
}
```

### Dashboard

All dashboard endpoints support `?start=YYYY-MM-DD&end=YYYY-MM-DD` query parameters.

**GET /api/dashboard/summary**

Overall statistics for the date range:

```json
{
  "period": { "start": "2025-01-01", "end": "2025-01-07" },
  "pageviews": 1234,
  "sessions": 456,
  "unique_visitors": 389,
  "avg_engagement": 45,
  "bounce_rate": 0.42,
  "top_pages": [...],
  "top_events": [...]
}
```

**GET /api/dashboard/timeseries**

Daily statistics over time:

```json
{
  "period": { "start": "2025-01-01", "end": "2025-01-07" },
  "data": [
    { "date": "2025-01-01", "pageviews": 123, "sessions": 45, ... },
    ...
  ]
}
```

**GET /api/dashboard/pages?limit=10**

Top pages by pageviews.

**GET /api/dashboard/events?limit=10**

Top custom events by count.

**GET /api/dashboard/sessions?start=<ms>&end=<ms>&limit=100&offset=0**

Individual session details.

**GET /api/dashboard/health**

Health check endpoint.

## Authentication

When API keys are configured, include them in the request header:

```
X-API-Key: your-secret-key
```

Or as a query parameter:

```
?api_key=your-secret-key
```

## Project Structure

```
analytics/
├── cmd/analytics/          # Application entry point
│   └── main.go
├── internal/
│   ├── api/                # Dashboard HTTP handlers
│   │   ├── handler.go
│   │   └── middleware.go
│   ├── config/             # Configuration management
│   │   └── config.go
│   ├── db/                 # Database layer
│   │   ├── db.go           # Connection, migrations
│   │   ├── schema.sql      # Database schema
│   │   └── queries.go      # CRUD operations
│   └── ingest/             # Event ingestion
│       └── ingest.go
├── go.mod
└── README.md
```

## Database Schema

- **events** - Raw analytics events (append-only)
- **sessions** - Aggregated session data
- **daily_stats** - Precomputed daily statistics
- **page_stats** - Page-level metrics
- **event_stats** - Custom event counts

## Deployment

### systemd Service

```ini
[Unit]
Description=Analytics Backend
After=network.target

[Service]
Type=simple
User=analytics
WorkingDirectory=/opt/analytics
ExecStart=/opt/analytics/analytics -addr=:3001 -db=/var/lib/analytics/db
Environment=ANALYTICS_API_KEY=your-key
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

### Docker

```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /src
COPY . .
RUN go build -o analytics ./cmd/analytics

FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY --from=builder /src/analytics /usr/local/bin/
EXPOSE 3001
CMD ["analytics"]
```

## Development

```bash
# Run with hot reload (using air)
air

# Run tests
go test ./...

# Run with race detector
go run -race ./cmd/analytics
```

## License

MIT
