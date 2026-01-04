# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

A privacy-focused web analytics service with two components:
- **Backend**: Go 1.23 server using SQLite with `modernc.org/sqlite` (pure Go, no CGO)
- **Frontend**: Vanilla JavaScript dashboard built with Vite

The system ingests analytics events via `POST /api/analytics` and provides a dashboard API under `/api/dashboard/`. All timestamps are stored as Unix milliseconds.

## Commands

### Backend (Go)
```bash
# Build
go build -o analytics-bin ./cmd/analytics

# Run (default: :3001, ./analytics.db)
./analytics-bin

# Run with custom config
./analytics-bin -addr ":8080" -db "./analytics.db" -api-key "your-key" -dashboard-key "dash-key"

# Run tests
go test ./...
go test ./... -v         # verbose
go test ./... -cover      # with coverage

# Run specific test
go test -v ./tests -run TestIntegration_FullFlow
```

### Frontend (Dashboard)
```bash
cd dashboard

# Install dependencies
npm install

# Dev server (proxies API to backend on :8080)
npm run dev

# Build for production
npm run build

# Preview production build
npm run preview
```

## Architecture

### Backend Structure
```
cmd/analytics/     # Main entry point
internal/
  ├── api/         # Dashboard HTTP handlers, middleware (CORS, rate limiting, auth)
  ├── config/      # Configuration from env/flags
  ├── db/          # Database schema, migrations, queries, retention job
  └── ingest/      # Event ingestion handler
tests/             # Integration tests with test server helper
```

### Data Flow
1. Events are posted to `POST /api/analytics` with API key auth
2. Ingest handler validates and writes to `events` table (append-only)
3. Materialized views (`sessions`, `daily_stats`, `page_stats`, `event_stats`) are updated incrementally
4. Dashboard endpoints query aggregated data from materialized views

### Database
- SQLite with WAL mode, 64MB cache
- All timestamps in **Unix milliseconds**
- Materialized views for fast queries (no raw event dumps in dashboard)
- Retention job deletes events older than configurable days (default 90)

### Key Patterns
- **Middleware chain**: `Recovery -> Logger -> CORS -> RateLimiter -> APIKey -> Handler`
- **Concurrent writes limited to 3** to respect SQLite constraints
- **No external dependencies** beyond the SQLite driver
- **Dashboard authentication** is separate from ingest API key (set different keys via flags)

## Testing

Integration tests use a `NewTestServer(t)` helper that creates an isolated SQLite database and HTTP test server. The test server automatically cleans up the database after tests complete.

When writing new integration tests, use `testDataMap()` for consistent event data payload structure.

## Frontend Notes

- Pure vanilla JS with ES modules
- CSS organized by layer: `base/`, `components/`, `layout/`, `utilities/`
- Auto-refreshing dashboard every 30 seconds
- Dark/light theme toggle persists to localStorage
