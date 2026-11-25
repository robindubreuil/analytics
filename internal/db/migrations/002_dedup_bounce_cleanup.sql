-- 002: Idempotent ingestion, honest bounce accounting, drop dead columns
-- dedup_key makes retries safe: a client that re-sends events after a lost
-- HTTP response no longer double-counts pageviews, sessions or aggregates.
ALTER TABLE events ADD COLUMN dedup_key TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_events_dedup ON events(dedup_key) WHERE dedup_key IS NOT NULL;

-- bounced tracks whether a session has been counted as a bounce in
-- daily_stats, so the "un-bounce" transition happens exactly once.
ALTER TABLE sessions ADD COLUMN bounced INTEGER NOT NULL DEFAULT 0;

-- exits and avg_engagement were never written (always 0) and never queried;
-- exits is derivable from sessions.exit_url if ever needed.
ALTER TABLE page_stats DROP COLUMN exits;
ALTER TABLE daily_stats DROP COLUMN avg_engagement;
