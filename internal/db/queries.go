// Package db provides database query operations for analytics.
package db

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Event represents a raw analytics event from the client.
type Event struct {
	SessionID      string
	Type           string // "pageview" or "event"
	EventName      string // empty for pageviews
	URL            string
	Referrer       string
	Title          string
	UserAgent      string
	Timestamp      int64 // unix milliseconds
	ScreenWidth    int
	ScreenHeight   int
	ViewportWidth  int
	ViewportHeight int
	ScrollDepth    int
	EngagementTime int // seconds
}

// Session represents a user session.
type Session struct {
	SessionID       string `json:"sessionId"`
	FirstSeen       int64  `json:"firstSeen"`
	LastSeen        int64  `json:"lastSeen"`
	Pageviews       int    `json:"pageviews"`
	Events          int    `json:"events"`
	TotalEngagement int    `json:"totalEngagement"`
	MaxScrollDepth  int    `json:"maxScrollDepth"`
	EntryURL        string `json:"entryUrl"`
	ExitURL         string `json:"exitUrl"`
	Referrer        string `json:"referrer"`
	UserAgent       string `json:"userAgent"`
}

// DailyStats represents aggregated daily statistics.
type DailyStats struct {
	Date            string `json:"date"`
	Pageviews       int    `json:"pageviews"`
	Sessions        int    `json:"sessions"`
	UniqueVisitors  int    `json:"uniqueVisitors"`
	AvgEngagement   int    `json:"avgEngagement"`
	TotalEngagement int    `json:"totalEngagement"`
	BouncedSessions int    `json:"bouncedSessions"`
}

// PageStats represents statistics for a specific page.
type PageStats struct {
	URL            string `json:"url"`
	Pageviews      int    `json:"pageviews"`
	Sessions       int    `json:"sessions"`
	AvgEngagement  int    `json:"avgEngagement"`
	MaxScrollDepth int    `json:"maxScrollDepth"`
	Exits          int    `json:"exits"`
}

// EventStats represents statistics for a custom event.
type EventStats struct {
	EventName string `json:"eventName"`
	Count     int    `json:"count"`
}

// StoreEvents inserts a batch of events into the database.
// Returns the number of events successfully stored.
func StoreEvents(database *sql.DB, events []Event) (int, error) {
	if len(events) == 0 {
		return 0, nil
	}

	const maxRetries = 10
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoff := 10 * (1 << (attempt - 1))
			time.Sleep(time.Duration(backoff) * time.Millisecond)
		}

		tx, err := database.Begin()
		if err != nil {
			if isBusyError(err) && attempt < maxRetries-1 {
				lastErr = err
				continue
			}
			return 0, fmt.Errorf("begin transaction: %w", err)
		}

		count, err := storeEventsInTx(tx, events)
		if err != nil {
			_ = tx.Rollback()
			if isBusyError(err) && attempt < maxRetries-1 {
				lastErr = err
				continue
			}
			return count, fmt.Errorf("store events: %w", err)
		}

		if err := tx.Commit(); err != nil {
			_ = tx.Rollback()
			if isBusyError(err) && attempt < maxRetries-1 {
				lastErr = err
				continue
			}
			return count, fmt.Errorf("commit transaction: %w", err)
		}

		return count, nil
	}

	return 0, fmt.Errorf("database busy after %d retries: %w", maxRetries, lastErr)
}

// storeEventsInTx performs the actual database operations within a transaction.
func storeEventsInTx(tx *sql.Tx, events []Event) (int, error) {
	now := time.Now().UnixMilli()

	eventStmt, err := tx.Prepare(`
		INSERT INTO events (
			session_id, type, event_name, url, referrer, title, user_agent,
			timestamp, screen_width, screen_height, viewport_width, viewport_height,
			scroll_depth, engagement_time, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return 0, fmt.Errorf("prepare event insert: %w", err)
	}
	defer eventStmt.Close()

	sessionStmt, err := tx.Prepare(`
		INSERT INTO sessions (
			session_id, first_seen, last_seen, pageviews, events,
			total_engagement, max_scroll_depth, entry_url, exit_url, referrer, user_agent
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(session_id) DO UPDATE SET
			last_seen = CASE WHEN excluded.last_seen > last_seen THEN excluded.last_seen ELSE last_seen END,
			pageviews = pageviews + excluded.pageviews,
			events = events + excluded.events,
			total_engagement = total_engagement + excluded.total_engagement,
			max_scroll_depth = MAX(max_scroll_depth, excluded.max_scroll_depth),
			exit_url = excluded.exit_url
	`)
	if err != nil {
		return 0, fmt.Errorf("prepare session upsert: %w", err)
	}
	defer sessionStmt.Close()

	dailyStmt, err := tx.Prepare(`
		INSERT INTO daily_stats (
			date, pageviews, sessions, unique_visitors,
			total_engagement, bounced_sessions
		) VALUES (?, ?, 0, 0, ?, 0)
		ON CONFLICT(date) DO UPDATE SET
			pageviews = pageviews + excluded.pageviews,
			total_engagement = total_engagement + excluded.total_engagement
	`)
	if err != nil {
		return 0, fmt.Errorf("prepare daily upsert: %w", err)
	}
	defer dailyStmt.Close()

	sessionCheckStmt, err := tx.Prepare(`
		SELECT 1 FROM sessions WHERE session_id = ?
	`)
	if err != nil {
		return 0, fmt.Errorf("prepare session check: %w", err)
	}
	defer sessionCheckStmt.Close()

	dailyNewSessionStmt, err := tx.Prepare(`
		INSERT INTO daily_stats (
			date, pageviews, sessions, unique_visitors,
			total_engagement, bounced_sessions
		) VALUES (?, 0, 1, 1, 0, 0)
		ON CONFLICT(date) DO UPDATE SET
			sessions = sessions + 1,
			unique_visitors = unique_visitors + 1
	`)
	if err != nil {
		return 0, fmt.Errorf("prepare daily new session: %w", err)
	}
	defer dailyNewSessionStmt.Close()

	pageStmt, err := tx.Prepare(`
		INSERT INTO page_stats (
			url, date, pageviews, sessions, avg_engagement,
			max_scroll_depth, exits
		) VALUES (?, ?, 1, 1, ?, ?, 0)
		ON CONFLICT(url, date) DO UPDATE SET
			pageviews = pageviews + 1,
			sessions = sessions + 1,
			avg_engagement = (avg_engagement * (pageviews - 1) + excluded.avg_engagement) / pageviews,
			max_scroll_depth = MAX(max_scroll_depth, excluded.max_scroll_depth)
	`)
	if err != nil {
		return 0, fmt.Errorf("prepare page upsert: %w", err)
	}
	defer pageStmt.Close()

	eventStmt2, err := tx.Prepare(`
		INSERT INTO event_stats (event_name, date, count)
		VALUES (?, ?, 1)
		ON CONFLICT(event_name, date) DO UPDATE SET
			count = count + 1
	`)
	if err != nil {
		return 0, fmt.Errorf("prepare event stat upsert: %w", err)
	}
	defer eventStmt2.Close()

	bounceStmt, err := tx.Prepare(`
		INSERT INTO daily_stats (date, pageviews, sessions, unique_visitors, total_engagement, bounced_sessions)
		VALUES (?, 0, 0, 0, 0, 0)
		ON CONFLICT(date) DO UPDATE SET
			bounced_sessions = bounced_sessions - 1
	`)
	if err != nil {
		return 0, fmt.Errorf("prepare bounce decrement: %w", err)
	}
	defer bounceStmt.Close()

	bounceIncStmt, err := tx.Prepare(`
		INSERT INTO daily_stats (date, pageviews, sessions, unique_visitors, total_engagement, bounced_sessions)
		VALUES (?, 0, 0, 0, 0, 1)
		ON CONFLICT(date) DO UPDATE SET
			bounced_sessions = bounced_sessions + 1
	`)
	if err != nil {
		return 0, fmt.Errorf("prepare bounce increment: %w", err)
	}
	defer bounceIncStmt.Close()

	sessionEvents := make(map[string][]Event)
	for _, e := range events {
		sessionEvents[e.SessionID] = append(sessionEvents[e.SessionID], e)
	}

	count := 0
	for _, e := range events {
		_, err := eventStmt.Exec(
			e.SessionID, e.Type, nullString(e.EventName), e.URL,
			nullString(e.Referrer), nullString(e.Title), nullString(e.UserAgent),
			e.Timestamp, e.ScreenWidth, e.ScreenHeight, e.ViewportWidth,
			e.ViewportHeight, e.ScrollDepth, e.EngagementTime, now,
		)
		if err != nil {
			return count, fmt.Errorf("insert event: %w", err)
		}
		count++
	}

	for sessionID, sessEvents := range sessionEvents {
		sortEventsByTimestamp(sessEvents)
		firstEvent := sessEvents[0]
		lastEvent := sessEvents[len(sessEvents)-1]

		pageviews := 0
		customEvents := 0
		totalEngagement := 0
		maxScrollDepth := 0

		for _, e := range sessEvents {
			if e.Type == "pageview" {
				pageviews++
			} else {
				customEvents++
			}
			totalEngagement += e.EngagementTime
			if e.ScrollDepth > maxScrollDepth {
				maxScrollDepth = e.ScrollDepth
			}
		}

		date := toUTCDate(firstEvent.Timestamp)

		var existingSession int
		isNewSession := true
		err := sessionCheckStmt.QueryRow(sessionID).Scan(&existingSession)
		if err == nil {
			isNewSession = false
		}

		_, err = sessionStmt.Exec(
			sessionID, firstEvent.Timestamp, lastEvent.Timestamp,
			pageviews, customEvents, totalEngagement, maxScrollDepth,
			firstEvent.URL, lastEvent.URL,
			nullString(firstEvent.Referrer), nullString(firstEvent.UserAgent),
		)
		if err != nil {
			return count, fmt.Errorf("upsert session: %w", err)
		}

		_, err = dailyStmt.Exec(date, pageviews, totalEngagement)
		if err != nil {
			return count, fmt.Errorf("upsert daily stats: %w", err)
		}

		if isNewSession {
			_, err = dailyNewSessionStmt.Exec(date)
			if err != nil {
				return count, fmt.Errorf("upsert daily new session: %w", err)
			}

			if pageviews <= 1 && customEvents == 0 && totalEngagement < 10 {
				_, err = bounceIncStmt.Exec(date)
				if err != nil {
					return count, fmt.Errorf("upsert bounce increment: %w", err)
				}
			}
		} else {
			prevPageviews := 0
			var prevEngagement int
			err := tx.QueryRow(`SELECT pageviews, total_engagement FROM sessions WHERE session_id = ?`, sessionID).Scan(&prevPageviews, &prevEngagement)
			if err == nil && prevPageviews <= 1 && prevEngagement < 10 {
				_, _ = bounceStmt.Exec(date)
			}
		}

		for _, e := range sessEvents {
			if e.Type == "pageview" {
				_, err := pageStmt.Exec(
					e.URL, toUTCDate(e.Timestamp),
					e.EngagementTime, e.ScrollDepth,
				)
				if err != nil {
					return count, fmt.Errorf("upsert page: %w", err)
				}
			}
		}

		for _, e := range sessEvents {
			if e.Type == "event" && e.EventName != "" {
				_, err := eventStmt2.Exec(e.EventName, toUTCDate(e.Timestamp))
				if err != nil {
					return count, fmt.Errorf("upsert event stat: %w", err)
				}
			}
		}
	}

	return count, nil
}

// GetSummary retrieves summary statistics for a date range.
func GetSummary(database *sql.DB, startDate, endDate string) (*DailyStats, error) {
	var stats DailyStats

	err := database.QueryRow(`
		SELECT
			COALESCE(SUM(pageviews), 0) as pageviews,
			COALESCE(SUM(sessions), 0) as sessions,
			COALESCE(SUM(unique_visitors), 0) as unique_visitors,
			COALESCE(SUM(total_engagement) / NULLIF(SUM(sessions), 0), 0) as avg_engagement,
			COALESCE(SUM(total_engagement), 0) as total_engagement,
			COALESCE(SUM(bounced_sessions), 0) as bounced_sessions
		FROM daily_stats
		WHERE date >= ? AND date <= ?
	`, startDate, endDate).Scan(
		&stats.Pageviews, &stats.Sessions, &stats.UniqueVisitors,
		&stats.AvgEngagement, &stats.TotalEngagement, &stats.BouncedSessions,
	)

	if err != nil {
		return nil, fmt.Errorf("query summary: %w", err)
	}

	return &stats, nil
}

// GetTimeSeries retrieves daily statistics for a date range.
func GetTimeSeries(database *sql.DB, startDate, endDate string) ([]DailyStats, error) {
	rows, err := database.Query(`
		SELECT date, pageviews, sessions, unique_visitors,
			COALESCE(total_engagement / NULLIF(sessions, 0), 0) as avg_engagement,
			total_engagement, bounced_sessions
		FROM daily_stats
		WHERE date >= ? AND date <= ?
		ORDER BY date ASC
	`, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("query timeseries: %w", err)
	}
	defer rows.Close()

	var stats []DailyStats
	for rows.Next() {
		var s DailyStats
		if err := rows.Scan(&s.Date, &s.Pageviews, &s.Sessions,
			&s.UniqueVisitors, &s.AvgEngagement, &s.TotalEngagement, &s.BouncedSessions); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		stats = append(stats, s)
	}

	return stats, rows.Err()
}

// GetTopPages retrieves top pages by pageviews for a date range.
func GetTopPages(database *sql.DB, startDate, endDate string, limit int) ([]PageStats, error) {
	rows, err := database.Query(`
		SELECT url, SUM(pageviews) as pageviews, SUM(sessions) as sessions,
			COALESCE(SUM(avg_engagement * pageviews) / NULLIF(SUM(pageviews), 0), 0) as avg_engagement,
			COALESCE(MAX(max_scroll_depth), 0) as max_scroll_depth
		FROM page_stats
		WHERE date >= ? AND date <= ?
		GROUP BY url
		ORDER BY pageviews DESC
		LIMIT ?
	`, startDate, endDate, limit)
	if err != nil {
		return nil, fmt.Errorf("query top pages: %w", err)
	}
	defer rows.Close()

	var stats []PageStats
	for rows.Next() {
		var s PageStats
		if err := rows.Scan(&s.URL, &s.Pageviews, &s.Sessions,
			&s.AvgEngagement, &s.MaxScrollDepth); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		stats = append(stats, s)
	}

	return stats, rows.Err()
}

// GetTopEvents retrieves top custom events by count for a date range.
func GetTopEvents(database *sql.DB, startDate, endDate string, limit int) ([]EventStats, error) {
	rows, err := database.Query(`
		SELECT event_name, SUM(count) as count
		FROM event_stats
		WHERE date >= ? AND date <= ?
		GROUP BY event_name
		ORDER BY count DESC
		LIMIT ?
	`, startDate, endDate, limit)
	if err != nil {
		return nil, fmt.Errorf("query top events: %w", err)
	}
	defer rows.Close()

	var stats []EventStats
	for rows.Next() {
		var s EventStats
		if err := rows.Scan(&s.EventName, &s.Count); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		stats = append(stats, s)
	}

	return stats, rows.Err()
}

// GetSessions retrieves sessions within a time range.
func GetSessions(database *sql.DB, startTime, endTime int64, limit, offset int) ([]Session, error) {
	rows, err := database.Query(`
		SELECT session_id, first_seen, last_seen, pageviews, events,
			total_engagement, max_scroll_depth, entry_url, exit_url, referrer, user_agent
		FROM sessions
		WHERE first_seen >= ? AND first_seen <= ?
		ORDER BY first_seen DESC
		LIMIT ? OFFSET ?
	`, startTime, endTime, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query sessions: %w", err)
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var s Session
		var referrer, userAgent sql.NullString
		if err := rows.Scan(&s.SessionID, &s.FirstSeen, &s.LastSeen,
			&s.Pageviews, &s.Events, &s.TotalEngagement, &s.MaxScrollDepth,
			&s.EntryURL, &s.ExitURL, &referrer, &userAgent); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		s.Referrer = referrer.String
		s.UserAgent = userAgent.String
		sessions = append(sessions, s)
	}

	return sessions, rows.Err()
}

// DeleteOldEvents removes raw events older than the specified number of days.
func DeleteOldEvents(database *sql.DB, retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		return 0, nil
	}

	cutoffTime := time.Now().AddDate(0, 0, -retentionDays).UnixMilli()

	result, err := database.Exec(`DELETE FROM events WHERE timestamp < ?`, cutoffTime)
	if err != nil {
		return 0, fmt.Errorf("delete old events: %w", err)
	}

	return result.RowsAffected()
}

// Vacuum runs VACUUM to reclaim database space.
func Vacuum(database *sql.DB) error {
	_, err := database.Exec(`VACUUM`)
	return err
}

func nullString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func toUTCDate(ms int64) string {
	return time.UnixMilli(ms).UTC().Format("2006-01-02")
}

var sqliteBusyRe = regexp.MustCompile(`(?i)(database is locked|SQLITE_BUSY|sqlite busy)`)

func isBusyError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return sqliteBusyRe.MatchString(errMsg) ||
		strings.Contains(errMsg, "(5)")
}

func sortEventsByTimestamp(events []Event) {
	for i := 1; i < len(events); i++ {
		for j := i; j > 0 && events[j].Timestamp < events[j-1].Timestamp; j-- {
			events[j], events[j-1] = events[j-1], events[j]
		}
	}
}
