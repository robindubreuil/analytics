## Analytics

A privacy-focused web analytics service with two components:
- **Backend**: Go 1.23 server using SQLite with `modernc.org/sqlite` (pure Go, no CGO)
- **Frontend**: Vanilla JavaScript dashboard built with Vite

The system ingests analytics events via `POST /api/analytics` and provides a dashboard API under `/api/dashboard/`. All timestamps are stored as Unix milliseconds.

## Commands

### Backend (Go)
```bash
# Build
go build -o analytics ./cmd/analytics

# Run (default: :3001, ./analytics.db)
./analytics

# Configuration via environment variables
ANALYTICS_ADDR=:8080 ANALYTICS_API_KEY=your-key ./analytics

# Configuration via flags
./analytics -addr :8080 -db ./analytics.db -api-key your-key -dashboard-key dash-key

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

# Run tests
npm test

# Preview production build
npm run preview
```

### Build & Package
```bash
make build           # Build binary
make build-deb       # Build Debian package
make docker          # Build Docker image
make test            # Run tests with race detection
make lint            # Run golangci-lint
```

## Architecture

### Backend Structure
```
cmd/analytics/     # Main entry point (server setup, graceful shutdown, retention job)
internal/
  ├── api/         # Dashboard HTTP handlers, middleware (CORS, rate limiting, auth)
  ├── config/      # Configuration from env vars and CLI flags (env vars take precedence)
  ├── db/          # Database schema, migrations, queries, retention job
  ├── ingest/      # Event ingestion handler
  └── testutil/    # Shared test helpers (TestDB, TestServer)
tests/             # End-to-end integration tests
```

### Tracker
`analytics.js` — Client-side tracking snippet. Respects DNT, uses sendBeacon for reliable delivery, supports batch events. Embed via:
```html
<script src="/analytics.js" data-endpoint="/api/analytics" data-api-key="key"></script>
```

### Data Flow
1. Events are posted to `POST /api/analytics` with API key auth
2. Ingest handler validates and writes to `events` table (append-only)
3. Materialized views (`sessions`, `daily_stats`, `page_stats`, `event_stats`) are updated incrementally
4. Dashboard endpoints query aggregated data from materialized views

### Middleware Architecture
The server uses a two-layer middleware design:

**Outer layer** (wraps the entire mux):
```
Recovery -> Logger -> CORS -> mux
```

**Inner layer** (per-route, inside the mux):
- Ingest route: `APIKey -> RateLimiter -> IngestHandler`
- Dashboard routes: `withAuth -> DashboardHandler` (health endpoint skips auth)

### Database
- SQLite with WAL mode, 64MB cache
- All timestamps in **Unix milliseconds**
- Materialized views for fast queries (no raw event dumps in dashboard)
- Retention job deletes events older than configurable days (default 90)

### Key Patterns
- **Concurrent writes limited to 3** (semaphore in ingest handler) to respect SQLite constraints
- **No external dependencies** beyond the SQLite driver
- **Dashboard authentication** is separate from ingest API key (set different keys via `-api-key` and `-dashboard-key`)
- **Config precedence**: defaults < env vars < CLI flags

## Testing

- **Unit tests**: Each package has its own `_test.go` files using table-driven patterns
- **Integration tests**: `tests/` uses `testutil.NewTestServer()` which creates an isolated SQLite database and HTTP test server with full middleware
- **Frontend tests**: `dashboard/js/__tests__/` uses Vitest for pure function tests
- **Shared helpers**: `internal/testutil` provides `NewTestDB()` and `NewTestServer()` to avoid duplication

When writing new integration tests, use `testutil.NewTestServer(t)` for full-stack tests and `testutil.NewTestDB(t)` for database-only tests.

## Frontend Notes

- Pure vanilla JS with ES modules
- CSS organized by layer: `base/`, `components/`, `layout/`, `utilities/`
- Auto-refreshing dashboard every 60 seconds
- Dark/light theme toggle persists to localStorage
