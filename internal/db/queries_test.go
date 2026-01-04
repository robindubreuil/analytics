// Package db provides tests for database query operations.
package db

import (
	"database/sql"
	"testing"
	"time"
)

func TestStoreEventsEmpty(t *testing.T) {
	th := NewTestHelper(t)
	defer th.Close(t)

	count, err := StoreEvents(th.DB, []Event{})
	if err != nil {
		t.Errorf("Expected no error for empty events, got %v", err)
	}
	if count != 0 {
		t.Errorf("Expected count 0, got %d", count)
	}
}

func TestStoreEventsSingle(t *testing.T) {
	th := NewTestHelper(t)
	defer th.Close(t)

	now := time.Now().UnixMilli()
	events := []Event{
		{
			SessionID:      "sess1",
			Type:           "pageview",
			URL:            "/test",
			Referrer:       "https://example.com",
			Title:          "Test Page",
			UserAgent:      "TestAgent/1.0",
			Timestamp:      now,
			ScreenWidth:    1920,
			ScreenHeight:   1080,
			ViewportWidth:  1920,
			ViewportHeight: 900,
			ScrollDepth:    50,
			EngagementTime: 30,
		},
	}

	count, err := StoreEvents(th.DB, events)
	if err != nil {
		t.Fatalf("Failed to store events: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected count 1, got %d", count)
	}

	// Verify event was stored
	var eventCount int
	err = th.DB.QueryRow("SELECT COUNT(*) FROM events").Scan(&eventCount)
	if err != nil {
		t.Fatalf("Failed to count events: %v", err)
	}
	if eventCount != 1 {
		t.Errorf("Expected 1 event in database, got %d", eventCount)
	}

	// Verify session was created
	var sessionCount int
	err = th.DB.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&sessionCount)
	if err != nil {
		t.Fatalf("Failed to count sessions: %v", err)
	}
	if sessionCount != 1 {
		t.Errorf("Expected 1 session in database, got %d", sessionCount)
	}
}

func TestStoreEventsMultiple(t *testing.T) {
	th := NewTestHelper(t)
	defer th.Close(t)

	now := time.Now().UnixMilli()
	events := []Event{
		{
			SessionID: "sess1",
			Type:      "pageview",
			URL:       "/page1",
			Timestamp: now,
		},
		{
			SessionID: "sess1",
			Type:      "pageview",
			URL:       "/page2",
			Timestamp: now + 1000,
		},
		{
			SessionID: "sess2",
			Type:      "pageview",
			URL:       "/page3",
			Timestamp: now,
		},
	}

	count, err := StoreEvents(th.DB, events)
	if err != nil {
		t.Fatalf("Failed to store events: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected count 3, got %d", count)
	}

	// Verify sessions
	var sessionCount int
	err = th.DB.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&sessionCount)
	if err != nil {
		t.Fatalf("Failed to count sessions: %v", err)
	}
	if sessionCount != 2 {
		t.Errorf("Expected 2 sessions in database, got %d", sessionCount)
	}
}

func TestStoreEventsCustomEvent(t *testing.T) {
	th := NewTestHelper(t)
	defer th.Close(t)

	now := time.Now().UnixMilli()
	events := []Event{
		{
			SessionID: "sess1",
			Type:      "event",
			EventName: "button_click",
			URL:       "/page",
			Timestamp: now,
		},
	}

	count, err := StoreEvents(th.DB, events)
	if err != nil {
		t.Fatalf("Failed to store events: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected count 1, got %d", count)
	}

	// Verify event stats
	var eventCount int
	err = th.DB.QueryRow("SELECT COUNT(*) FROM event_stats").Scan(&eventCount)
	if err != nil {
		t.Fatalf("Failed to count event stats: %v", err)
	}
	if eventCount != 1 {
		t.Errorf("Expected 1 event stat in database, got %d", eventCount)
	}
}

func TestStoreEventsSessionUpdate(t *testing.T) {
	th := NewTestHelper(t)
	defer th.Close(t)

	now := time.Now().UnixMilli()

	// Store first batch of events
	events1 := []Event{
		{
			SessionID:      "sess1",
			Type:           "pageview",
			URL:            "/page1",
			EngagementTime: 10,
			Timestamp:      now,
		},
	}

	_, err := StoreEvents(th.DB, events1)
	if err != nil {
		t.Fatalf("Failed to store first events: %v", err)
	}

	// Store more events for same session
	events2 := []Event{
		{
			SessionID:      "sess1",
			Type:           "pageview",
			URL:            "/page2",
			EngagementTime: 20,
			Timestamp:      now + 5000,
		},
	}

	_, err = StoreEvents(th.DB, events2)
	if err != nil {
		t.Fatalf("Failed to store second events: %v", err)
	}

	// Verify session was updated, not duplicated
	var sessionCount int
	err = th.DB.QueryRow("SELECT COUNT(*) FROM sessions WHERE session_id = 'sess1'").Scan(&sessionCount)
	if err != nil {
		t.Fatalf("Failed to count sessions: %v", err)
	}
	if sessionCount != 1 {
		t.Errorf("Expected 1 session, got %d", sessionCount)
	}

	// Verify pageviews increased
	var pageviews int
	err = th.DB.QueryRow("SELECT pageviews FROM sessions WHERE session_id = 'sess1'").Scan(&pageviews)
	if err != nil {
		t.Fatalf("Failed to get pageviews: %v", err)
	}
	if pageviews != 2 {
		t.Errorf("Expected 2 pageviews, got %d", pageviews)
	}
}

func TestStoreEventsEmptyStrings(t *testing.T) {
	th := NewTestHelper(t)
	defer th.Close(t)

	now := time.Now().UnixMilli()
	events := []Event{
		{
			SessionID: "sess1",
			Type:      "pageview",
			URL:       "/test",
			// Referrer, Title, UserAgent are empty (should be NULL)
			Timestamp: now,
		},
	}

	_, err := StoreEvents(th.DB, events)
	if err != nil {
		t.Fatalf("Failed to store events: %v", err)
	}

	// Verify empty strings were stored as NULL
	var referrer sql.NullString
	err = th.DB.QueryRow("SELECT referrer FROM events LIMIT 1").Scan(&referrer)
	if err != nil {
		t.Fatalf("Failed to get referrer: %v", err)
	}
	if referrer.Valid {
		t.Errorf("Expected NULL referrer, got %s", referrer.String)
	}
}

func TestGetSummary(t *testing.T) {
	th := NewTestHelper(t)
	defer th.Close(t)

	// Store some test data
	now := time.Now().UnixMilli()
	today := time.Now().Format("2006-01-02")
	events := []Event{
		{
			SessionID:      "sess1",
			Type:           "pageview",
			URL:            "/page1",
			EngagementTime: 30,
			Timestamp:      now,
		},
		{
			SessionID:      "sess2",
			Type:           "pageview",
			URL:            "/page2",
			EngagementTime: 60,
			Timestamp:      now,
		},
	}

	_, err := StoreEvents(th.DB, events)
	if err != nil {
		t.Fatalf("Failed to store events: %v", err)
	}

	summary, err := GetSummary(th.DB, today, today)
	if err != nil {
		t.Fatalf("Failed to get summary: %v", err)
	}

	if summary.Pageviews != 2 {
		t.Errorf("Expected 2 pageviews, got %d", summary.Pageviews)
	}
	if summary.Sessions != 2 {
		t.Errorf("Expected 2 sessions, got %d", summary.Sessions)
	}
	if summary.UniqueVisitors != 2 {
		t.Errorf("Expected 2 unique visitors, got %d", summary.UniqueVisitors)
	}
}

func TestGetSummaryEmpty(t *testing.T) {
	th := NewTestHelper(t)
	defer th.Close(t)

	summary, err := GetSummary(th.DB, "2024-01-01", "2024-01-31")
	if err != nil {
		t.Fatalf("Failed to get summary: %v", err)
	}

	if summary.Pageviews != 0 {
		t.Errorf("Expected 0 pageviews, got %d", summary.Pageviews)
	}
	if summary.Sessions != 0 {
		t.Errorf("Expected 0 sessions, got %d", summary.Sessions)
	}
}

func TestGetTimeSeries(t *testing.T) {
	th := NewTestHelper(t)
	defer th.Close(t)

	now := time.Now()
	today := now.Format("2006-01-02")
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")

	// Store events for today and yesterday
	events := []Event{
		{
			SessionID: "sess1",
			Type:      "pageview",
			URL:       "/page1",
			Timestamp: now.Add(-24 * time.Hour).UnixMilli(), // yesterday
		},
		{
			SessionID: "sess2",
			Type:      "pageview",
			URL:       "/page2",
			Timestamp: now.UnixMilli(), // today
		},
	}

	_, err := StoreEvents(th.DB, events)
	if err != nil {
		t.Fatalf("Failed to store events: %v", err)
	}

	stats, err := GetTimeSeries(th.DB, yesterday, today)
	if err != nil {
		t.Fatalf("Failed to get timeseries: %v", err)
	}

	if len(stats) != 2 {
		t.Errorf("Expected 2 days of stats, got %d", len(stats))
	}
}

func TestGetTopPages(t *testing.T) {
	th := NewTestHelper(t)
	defer th.Close(t)

	now := time.Now().UnixMilli()
	today := time.Now().Format("2006-01-02")

	events := []Event{
		{
			SessionID: "sess1",
			Type:      "pageview",
			URL:       "/popular",
			Timestamp: now,
		},
		{
			SessionID: "sess2",
			Type:      "pageview",
			URL:       "/popular",
			Timestamp: now + 1000,
		},
		{
			SessionID: "sess3",
			Type:      "pageview",
			URL:       "/unpopular",
			Timestamp: now,
		},
	}

	_, err := StoreEvents(th.DB, events)
	if err != nil {
		t.Fatalf("Failed to store events: %v", err)
	}

	pages, err := GetTopPages(th.DB, today, today, 10)
	if err != nil {
		t.Fatalf("Failed to get top pages: %v", err)
	}

	if len(pages) == 0 {
		t.Error("Expected at least 1 page")
	}

	// /popular should be first
	if pages[0].URL != "/popular" {
		t.Errorf("Expected /popular to be first, got %s", pages[0].URL)
	}
	if pages[0].Pageviews != 2 {
		t.Errorf("Expected /popular to have 2 pageviews, got %d", pages[0].Pageviews)
	}
}

func TestGetTopEvents(t *testing.T) {
	th := NewTestHelper(t)
	defer th.Close(t)

	now := time.Now().UnixMilli()
	today := time.Now().Format("2006-01-02")

	events := []Event{
		{
			SessionID: "sess1",
			Type:      "event",
			EventName: "click",
			URL:       "/page",
			Timestamp: now,
		},
		{
			SessionID: "sess2",
			Type:      "event",
			EventName: "click",
			URL:       "/page",
			Timestamp: now + 1000,
		},
		{
			SessionID: "sess3",
			Type:      "event",
			EventName: "submit",
			URL:       "/page",
			Timestamp: now,
		},
	}

	_, err := StoreEvents(th.DB, events)
	if err != nil {
		t.Fatalf("Failed to store events: %v", err)
	}

	topEvents, err := GetTopEvents(th.DB, today, today, 10)
	if err != nil {
		t.Fatalf("Failed to get top events: %v", err)
	}

	if len(topEvents) == 0 {
		t.Error("Expected at least 1 event")
	}

	// click should be first with count 2
	if topEvents[0].EventName != "click" {
		t.Errorf("Expected click to be first, got %s", topEvents[0].EventName)
	}
	if topEvents[0].Count != 2 {
		t.Errorf("Expected click to have count 2, got %d", topEvents[0].Count)
	}
}

func TestGetSessions(t *testing.T) {
	th := NewTestHelper(t)
	defer th.Close(t)

	now := time.Now().UnixMilli()
	startTime := now - 3600000 // 1 hour ago
	endTime := now + 3600000   // 1 hour from now

	events := []Event{
		{
			SessionID: "sess1",
			Type:      "pageview",
			URL:       "/page1",
			Referrer:  "https://google.com",
			UserAgent: "Mozilla/5.0",
			Timestamp: now,
		},
	}

	_, err := StoreEvents(th.DB, events)
	if err != nil {
		t.Fatalf("Failed to store events: %v", err)
	}

	sessions, err := GetSessions(th.DB, startTime, endTime, 10, 0)
	if err != nil {
		t.Fatalf("Failed to get sessions: %v", err)
	}

	if len(sessions) == 0 {
		t.Error("Expected at least 1 session")
	}

	if sessions[0].SessionID != "sess1" {
		t.Errorf("Expected session ID sess1, got %s", sessions[0].SessionID)
	}
	if sessions[0].Referrer != "https://google.com" {
		t.Errorf("Expected referrer, got %s", sessions[0].Referrer)
	}
	if sessions[0].UserAgent != "Mozilla/5.0" {
		t.Errorf("Expected user agent, got %s", sessions[0].UserAgent)
	}
}

func TestGetSessionsPagination(t *testing.T) {
	th := NewTestHelper(t)
	defer th.Close(t)

	now := time.Now().UnixMilli()

	// Create 5 sessions
	for i := 1; i <= 5; i++ {
		events := []Event{
			{
				SessionID: "sess" + string(rune('0'+i)),
				Type:      "pageview",
				URL:       "/page",
				Timestamp: now + int64(i)*1000,
			},
		}
		_, err := StoreEvents(th.DB, events)
		if err != nil {
			t.Fatalf("Failed to store events: %v", err)
		}
	}

	// Get first page
	sessions1, err := GetSessions(th.DB, now, now+10000, 2, 0)
	if err != nil {
		t.Fatalf("Failed to get first page: %v", err)
	}
	if len(sessions1) != 2 {
		t.Errorf("Expected 2 sessions on first page, got %d", len(sessions1))
	}

	// Get second page
	sessions2, err := GetSessions(th.DB, now, now+10000, 2, 2)
	if err != nil {
		t.Fatalf("Failed to get second page: %v", err)
	}
	if len(sessions2) != 2 {
		t.Errorf("Expected 2 sessions on second page, got %d", len(sessions2))
	}

	// Verify different sessions
	if sessions1[0].SessionID == sessions2[0].SessionID {
		t.Error("Expected different sessions on different pages")
	}
}

func TestDeleteOldEvents(t *testing.T) {
	th := NewTestHelper(t)
	defer th.Close(t)

	now := time.Now().UnixMilli()
	oldTime := now - (91 * 24 * 3600 * 1000) // 91 days ago

	events := []Event{
		{
			SessionID: "sess_old",
			Type:      "pageview",
			URL:       "/old",
			Timestamp: oldTime,
		},
		{
			SessionID: "sess_new",
			Type:      "pageview",
			URL:       "/new",
			Timestamp: now,
		},
	}

	_, err := StoreEvents(th.DB, events)
	if err != nil {
		t.Fatalf("Failed to store events: %v", err)
	}

	// Delete events older than 90 days
	deleted, err := DeleteOldEvents(th.DB, 90)
	if err != nil {
		t.Fatalf("Failed to delete old events: %v", err)
	}

	if deleted != 1 {
		t.Errorf("Expected 1 deleted event, got %d", deleted)
	}

	// Verify only old event was deleted
	var count int
	err = th.DB.QueryRow("SELECT COUNT(*) FROM events").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count events: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 event remaining, got %d", count)
	}
}

func TestDeleteOldEventsZeroRetention(t *testing.T) {
	th := NewTestHelper(t)
	defer th.Close(t)

	// Zero retention means no deletion
	deleted, err := DeleteOldEvents(th.DB, 0)
	if err != nil {
		t.Fatalf("Failed to delete old events: %v", err)
	}

	if deleted != 0 {
		t.Errorf("Expected 0 deleted events with zero retention, got %d", deleted)
	}
}

func TestVacuum(t *testing.T) {
	th := NewTestHelper(t)
	defer th.Close(t)

	// Vacuum should not error
	if err := Vacuum(th.DB); err != nil {
		t.Errorf("Vacuum failed: %v", err)
	}
}

func TestStoreEventsMaxScrollDepth(t *testing.T) {
	th := NewTestHelper(t)
	defer th.Close(t)

	now := time.Now().UnixMilli()
	events := []Event{
		{
			SessionID:      "sess1",
			Type:           "pageview",
			URL:            "/page1",
			ScrollDepth:    25,
			EngagementTime: 10,
			Timestamp:      now,
		},
		{
			SessionID:      "sess1",
			Type:           "pageview",
			URL:            "/page2",
			ScrollDepth:    75,
			EngagementTime: 20,
			Timestamp:      now + 5000,
		},
	}

	_, err := StoreEvents(th.DB, events)
	if err != nil {
		t.Fatalf("Failed to store events: %v", err)
	}

	// Verify max scroll depth
	var maxScroll int
	err = th.DB.QueryRow("SELECT max_scroll_depth FROM sessions WHERE session_id = 'sess1'").Scan(&maxScroll)
	if err != nil {
		t.Fatalf("Failed to get max scroll depth: %v", err)
	}
	if maxScroll != 75 {
		t.Errorf("Expected max scroll depth 75, got %d", maxScroll)
	}
}

func TestStoreEventsTotalEngagement(t *testing.T) {
	th := NewTestHelper(t)
	defer th.Close(t)

	now := time.Now().UnixMilli()
	events := []Event{
		{
			SessionID:      "sess1",
			Type:           "pageview",
			URL:            "/page1",
			EngagementTime: 30,
			Timestamp:      now,
		},
		{
			SessionID:      "sess1",
			Type:           "pageview",
			URL:            "/page2",
			EngagementTime: 45,
			Timestamp:      now + 5000,
		},
	}

	_, err := StoreEvents(th.DB, events)
	if err != nil {
		t.Fatalf("Failed to store events: %v", err)
	}

	// Verify total engagement
	var totalEngagement int
	err = th.DB.QueryRow("SELECT total_engagement FROM sessions WHERE session_id = 'sess1'").Scan(&totalEngagement)
	if err != nil {
		t.Fatalf("Failed to get total engagement: %v", err)
	}
	if totalEngagement != 75 {
		t.Errorf("Expected total engagement 75, got %d", totalEngagement)
	}
}

func TestDailyStatsAggregation(t *testing.T) {
	th := NewTestHelper(t)
	defer th.Close(t)

	now := time.Now()
	nowMs := now.UnixMilli()
	today := now.Format("2006-01-02")

	// Create multiple sessions on the same day
	for i := 0; i < 3; i++ {
		events := []Event{
			{
				SessionID:      "sess" + string(rune('1'+i)),
				Type:           "pageview",
				URL:            "/page",
				EngagementTime: 30,
				Timestamp:      nowMs + int64(i)*1000,
			},
		}
		_, err := StoreEvents(th.DB, events)
		if err != nil {
			t.Fatalf("Failed to store events: %v", err)
		}
	}

	// Check daily stats
	var stats DailyStats
	err := th.DB.QueryRow(`
		SELECT date, pageviews, sessions, unique_visitors, total_engagement
		FROM daily_stats WHERE date = ?
	`, today).Scan(&stats.Date, &stats.Pageviews, &stats.Sessions, &stats.UniqueVisitors, &stats.TotalEngagement)

	if err != nil {
		if err == sql.ErrNoRows {
			t.Fatal("No daily stats found")
		}
		t.Fatalf("Failed to get daily stats: %v", err)
	}

	if stats.Pageviews != 3 {
		t.Errorf("Expected 3 pageviews, got %d", stats.Pageviews)
	}
	if stats.Sessions != 3 {
		t.Errorf("Expected 3 sessions, got %d", stats.Sessions)
	}
}

// BenchmarkStoreEvents benchmarks event storage performance.
func BenchmarkStoreEvents(b *testing.B) {
	th := NewTestHelper(&testing.T{})
	defer th.Close(&testing.T{})

	now := time.Now().UnixMilli()
	events := make([]Event, 100)
	for i := 0; i < 100; i++ {
		events[i] = Event{
			SessionID: "sess_bench",
			Type:      "pageview",
			URL:       "/page",
			Timestamp: now + int64(i)*1000,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = StoreEvents(th.DB, events)
	}
}
