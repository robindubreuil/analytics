-- Analytics schema for privacy-focused web analytics
-- All timestamps stored as Unix milliseconds (UTC)

-- Raw events table (append-only)
CREATE TABLE IF NOT EXISTS events (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id      TEXT NOT NULL,
    type            TEXT NOT NULL,        -- 'pageview' or 'event'
    event_name      TEXT,                 -- null for pageviews, set for custom events
    url             TEXT NOT NULL,
    referrer        TEXT,
    title           TEXT,
    user_agent      TEXT,
    timestamp       INTEGER NOT NULL,     -- client timestamp in milliseconds
    screen_width    INTEGER,
    screen_height   INTEGER,
    viewport_width  INTEGER,
    viewport_height INTEGER,
    scroll_depth    INTEGER,
    engagement_time INTEGER,              -- seconds
    created_at      INTEGER NOT NULL      -- server ingestion time
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_events_session ON events(session_id);
CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp);
CREATE INDEX IF NOT EXISTS idx_events_type ON events(type);
CREATE INDEX IF NOT EXISTS idx_events_url ON events(url);
CREATE INDEX IF NOT EXISTS idx_events_created_at ON events(created_at);

-- Sessions table (materialized, updated on ingest)
CREATE TABLE IF NOT EXISTS sessions (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id        TEXT NOT NULL UNIQUE,
    first_seen        INTEGER NOT NULL,
    last_seen         INTEGER NOT NULL,
    pageviews         INTEGER NOT NULL DEFAULT 0,
    events            INTEGER NOT NULL DEFAULT 0,
    total_engagement  INTEGER NOT NULL DEFAULT 0,
    max_scroll_depth  INTEGER NOT NULL DEFAULT 0,
    entry_url         TEXT,
    exit_url          TEXT,
    referrer          TEXT,
    user_agent        TEXT
);

CREATE INDEX IF NOT EXISTS idx_sessions_first_seen ON sessions(first_seen);
CREATE INDEX IF NOT EXISTS idx_sessions_last_seen ON sessions(last_seen);

-- Daily statistics (precomputed for fast dashboard queries)
CREATE TABLE IF NOT EXISTS daily_stats (
    date            TEXT NOT NULL,        -- YYYY-MM-DD
    pageviews       INTEGER NOT NULL DEFAULT 0,
    sessions        INTEGER NOT NULL DEFAULT 0,
    unique_visitors INTEGER NOT NULL DEFAULT 0,
    avg_engagement  INTEGER NOT NULL DEFAULT 0,
    total_engagement INTEGER NOT NULL DEFAULT 0,
    bounced_sessions INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (date)
);

-- Page performance statistics
CREATE TABLE IF NOT EXISTS page_stats (
    url             TEXT NOT NULL,
    date            TEXT NOT NULL,        -- YYYY-MM-DD
    pageviews       INTEGER NOT NULL DEFAULT 0,
    sessions        INTEGER NOT NULL DEFAULT 0,
    avg_engagement  INTEGER NOT NULL DEFAULT 0,
    max_scroll_depth INTEGER NOT NULL DEFAULT 0,
    exits           INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (url, date)
);

-- Custom event statistics
CREATE TABLE IF NOT EXISTS event_stats (
    event_name      TEXT NOT NULL,
    date            TEXT NOT NULL,        -- YYYY-MM-DD
    count           INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (event_name, date)
);

-- Migration tracking
CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at INTEGER NOT NULL
);
