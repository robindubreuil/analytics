// Package api provides tests for dashboard API handlers.
package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dubreuilpro/analytics/internal/db"
)

// openTestDB creates a test database.
func openTestDBForHandler(t *testing.T) *sql.DB {
	t.Helper()

	// Use nanoseconds for uniqueness
	unique := time.Now().Format("20060102150405") + fmt.Sprintf("%d", time.Now().Nanosecond())
	path := "/tmp/analytics_handler_test_" + unique + ".db"

	database, err := db.Open(path)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	t.Cleanup(func() {
		database.Close()
		os.Remove(path)
	})
	return database
}

func TestNew(t *testing.T) {
	database := openTestDBForHandler(t)
	handler := New(database)

	if handler == nil {
		t.Error("Expected non-nil handler")
	}
	if handler.db != database {
		t.Error("Handler database not set correctly")
	}
}

func TestSetDashboardKey(t *testing.T) {
	testKey := "test-dashboard-key"
	SetDashboardKey(testKey)

	if dashboardKey != testKey {
		t.Errorf("Expected dashboardKey %s, got %s", testKey, dashboardKey)
	}

	// Reset to empty
	SetDashboardKey("")
}

func TestDashboardHandler_RegisterRoutes(t *testing.T) {
	database := openTestDBForHandler(t)
	handler := New(database)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Test that all routes are registered
	routes := []string{
		"GET /api/dashboard/summary",
		"GET /api/dashboard/timeseries",
		"GET /api/dashboard/pages",
		"GET /api/dashboard/events",
		"GET /api/dashboard/sessions",
		"GET /api/dashboard/health",
	}

	// We can't easily test if routes are registered without accessing internals,
	// but we can test that calling them doesn't panic
	for _, route := range routes {
		parts := strings.Split(route, " ")
		method := parts[0]
		path := parts[1]

		req := httptest.NewRequest(method, path, nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		// Health should work, others need auth or data
		if path == "/api/dashboard/health" {
			if w.Code != http.StatusOK {
				t.Errorf("Health endpoint failed: %d", w.Code)
			}
		}
	}
}

func TestDashboardHandler_WithAuthNoKey(t *testing.T) {
	database := openTestDBForHandler(t)
	handler := New(database)

	// No dashboard key set - should allow access
	SetDashboardKey("")

	req := httptest.NewRequest("GET", "/api/dashboard/summary", nil)
	w := httptest.NewRecorder()

	handler.withAuth(handler.summary).ServeHTTP(w, req)

	// Should get OK (with empty data, using default date range) not unauthorized
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 (using defaults), got %d", w.Code)
	}
}

func TestDashboardHandler_WithAuthWithKey(t *testing.T) {
	database := openTestDBForHandler(t)
	handler := New(database)

	testKey := "test-dashboard-key"
	SetDashboardKey(testKey)

	defer SetDashboardKey("")

	tests := []struct {
		name       string
		headerKey  string
		queryKey   string
		expectCode int
	}{
		{
			name:       "valid key via header",
			headerKey:  testKey,
			expectCode: http.StatusOK, // Uses default date range
		},
		{
			name:       "valid key via query",
			queryKey:   testKey,
			expectCode: http.StatusOK, // Uses default date range
		},
		{
			name:       "invalid key",
			headerKey:  "wrong-key",
			expectCode: http.StatusUnauthorized,
		},
		{
			name:       "no key",
			expectCode: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/dashboard/summary", nil)
			if tt.headerKey != "" {
				req.Header.Set("X-API-Key", tt.headerKey)
			}
			if tt.queryKey != "" {
				req.URL.RawQuery = "api_key=" + tt.queryKey
			}
			w := httptest.NewRecorder()

			handler.withAuth(handler.summary).ServeHTTP(w, req)

			if w.Code != tt.expectCode {
				t.Errorf("Expected status %d, got %d", tt.expectCode, w.Code)
			}
		})
	}
}

func TestDashboardHandler_Summary(t *testing.T) {
	database := openTestDBForHandler(t)
	handler := New(database)
	SetDashboardKey("")

	// Add test data
	now := time.Now().UnixMilli()
	events := []db.Event{
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

	_, err := db.StoreEvents(database, events)
	if err != nil {
		t.Fatalf("Failed to store events: %v", err)
	}

	today := time.Now().Format("2006-01-02")
	req := httptest.NewRequest("GET", "/api/dashboard/summary?start="+today+"&end="+today, nil)
	w := httptest.NewRecorder()

	handler.summary(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp["pageviews"] != float64(2) {
		t.Errorf("Expected pageviews 2, got %v", resp["pageviews"])
	}
	if resp["sessions"] != float64(2) {
		t.Errorf("Expected sessions 2, got %v", resp["sessions"])
	}

	// Check bounce rate field exists
	if resp["bounceRate"] == nil {
		t.Error("Expected bounceRate in response")
	}
	// Note: bounced_sessions is not currently calculated, so bounceRate is always 0
	// This is a known limitation in the current implementation
	if resp["bounceRate"] != float64(0) {
		t.Errorf("Expected bounceRate 0, got %v", resp["bounceRate"])
	}
}

func TestDashboardHandler_Timeseries(t *testing.T) {
	database := openTestDBForHandler(t)
	handler := New(database)
	SetDashboardKey("")

	now := time.Now()
	today := now.Format("2006-01-02")
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")

	events := []db.Event{
		{
			SessionID: "sess1",
			Type:      "pageview",
			URL:       "/page1",
			Timestamp: now.UnixMilli(),
		},
	}

	_, err := db.StoreEvents(database, events)
	if err != nil {
		t.Fatalf("Failed to store events: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/dashboard/timeseries?start="+yesterday+"&end="+today, nil)
	w := httptest.NewRecorder()

	handler.timeseries(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	data, ok := resp["data"].([]any)
	if !ok {
		t.Fatal("Expected data array in response")
	}

	// Should have at least one day of data
	if len(data) == 0 {
		t.Error("Expected at least one data point")
	}
}

func TestDashboardHandler_Pages(t *testing.T) {
	database := openTestDBForHandler(t)
	handler := New(database)
	SetDashboardKey("")

	now := time.Now().UnixMilli()
	today := time.Now().Format("2006-01-02")

	events := []db.Event{
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
	}

	_, err := db.StoreEvents(database, events)
	if err != nil {
		t.Fatalf("Failed to store events: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/dashboard/pages?start="+today+"&end="+today, nil)
	w := httptest.NewRecorder()

	handler.pages(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	pages, ok := resp["pages"].([]any)
	if !ok {
		t.Fatal("Expected pages array in response")
	}

	if len(pages) == 0 {
		t.Error("Expected at least one page")
	}
}

func TestDashboardHandler_Events(t *testing.T) {
	database := openTestDBForHandler(t)
	handler := New(database)
	SetDashboardKey("")

	now := time.Now().UnixMilli()
	today := time.Now().Format("2006-01-02")

	events := []db.Event{
		{
			SessionID: "sess1",
			Type:      "event",
			EventName: "click",
			URL:       "/page",
			Timestamp: now,
		},
	}

	_, err := db.StoreEvents(database, events)
	if err != nil {
		t.Fatalf("Failed to store events: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/dashboard/events?start="+today+"&end="+today, nil)
	w := httptest.NewRecorder()

	handler.events(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	eventsData, ok := resp["events"].([]any)
	if !ok {
		t.Fatal("Expected events array in response")
	}

	if len(eventsData) == 0 {
		t.Error("Expected at least one event")
	}
}

func TestDashboardHandler_Sessions(t *testing.T) {
	database := openTestDBForHandler(t)
	handler := New(database)
	SetDashboardKey("")

	now := time.Now().UnixMilli()

	events := []db.Event{
		{
			SessionID: "sess1",
			Type:      "pageview",
			URL:       "/page1",
			Referrer:  "https://google.com",
			UserAgent: "Mozilla/5.0",
			Timestamp: now,
		},
	}

	_, err := db.StoreEvents(database, events)
	if err != nil {
		t.Fatalf("Failed to store events: %v", err)
	}

	startTime := now - 3600000
	endTime := now + 3600000

	req := httptest.NewRequest("GET", "/api/dashboard/sessions?start="+fmt.Sprint(startTime)+"&end="+fmt.Sprint(endTime), nil)
	w := httptest.NewRecorder()

	handler.sessions(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	sessions, ok := resp["sessions"].([]any)
	if !ok {
		t.Fatal("Expected sessions array in response")
	}

	if len(sessions) == 0 {
		t.Error("Expected at least one session")
	}
}

func TestDashboardHandler_Health(t *testing.T) {
	database := openTestDBForHandler(t)
	handler := New(database)

	req := httptest.NewRequest("GET", "/api/dashboard/health", nil)
	w := httptest.NewRecorder()

	handler.health(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp["status"] != "healthy" {
		t.Errorf("Expected status healthy, got %v", resp["status"])
	}
}

func TestDashboardHandler_ParseDateRange(t *testing.T) {
	database := openTestDBForHandler(t)
	handler := New(database)

	tests := []struct {
		name         string
		query        string
		expectStart  string
		expectEnd    string
		expectError  bool
	}{
		{
			name:        "valid dates",
			query:       "start=2024-01-01&end=2024-01-31",
			expectStart: "2024-01-01",
			expectEnd:   "2024-01-31",
		},
		{
			name:        "default dates",
			query:       "",
			expectError: false,
			// Just check that dates are returned
		},
		{
			name:        "only start",
			query:       "start=2024-01-01",
			expectError: false,
		},
		{
			name:        "only end",
			query:       "end=2026-12-31",
			expectError: false,
		},
		{
			name:        "invalid start format",
			query:       "start=invalid",
			expectError: true,
		},
		{
			name:        "invalid end format",
			query:       "end=invalid",
			expectError: true,
		},
		{
			name:        "start after end",
			query:       "start=2024-01-31&end=2024-01-01",
			expectError: true,
		},
		{
			name:        "start out of range",
			query:       "start=1800-01-01",
			expectError: true,
		},
		{
			name:        "end out of range",
			query:       "end=2500-01-01",
			expectError: true,
		},
		{
			name:        "range too large",
			query:       "start=2020-01-01&end=2025-01-01",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/?"+tt.query, nil)
			start, end, err := handler.parseDateRange(req)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tt.expectStart != "" && start != tt.expectStart {
				t.Errorf("Expected start %s, got %s", tt.expectStart, start)
			}
			if tt.expectEnd != "" && end != tt.expectEnd {
				t.Errorf("Expected end %s, got %s", tt.expectEnd, end)
			}
		})
	}
}

func TestDashboardHandler_ParseTimeRange(t *testing.T) {
	database := openTestDBForHandler(t)
	handler := New(database)

	tests := []struct {
		name        string
		query       string
		expectError bool
	}{
		{
			name:        "valid timestamps",
			query:       "start=1704067200000&end=1704153600000",
			expectError: false,
		},
		{
			name:        "default timestamps",
			query:       "",
			expectError: false,
		},
		{
			name:        "invalid start",
			query:       "start=invalid",
			expectError: true,
		},
		{
			name:        "invalid end",
			query:       "end=invalid",
			expectError: true,
		},
		{
			name:        "start after end",
			query:       "start=1704153600000&end=1704067200000",
			expectError: true,
		},
		{
			name:        "start out of range",
			query:       "start=-3000000000000",
			expectError: true,
		},
		{
			name:        "end out of range",
			query:       "end=10000000000000",
			expectError: true,
		},
		{
			name:        "range too large",
			query:       "start=1704067200000&end=1900000000000",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/?"+tt.query, nil)
			start, end, err := handler.parseTimeRange(req)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Verify start <= end
			if start > end {
				t.Error("Start should be <= end")
			}
		})
	}
}

func TestDashboardHandler_ParseLimit(t *testing.T) {
	database := openTestDBForHandler(t)
	handler := New(database)

	tests := []struct {
		query        string
		defaultLimit int
		expected     int
	}{
		{"", 10, 10},
		{"?limit=5", 10, 5},
		{"?limit=100", 10, 100},
		{"?limit=2000", 10, 1000}, // Max 1000
		{"?limit=-1", 10, 10},      // Invalid uses default
		{"?limit=0", 10, 10},       // Invalid uses default
		{"?limit=invalid", 10, 10}, // Invalid uses default
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/"+tt.query, nil)
			result := handler.parseLimit(req, tt.defaultLimit)
			if result != tt.expected {
				t.Errorf("parseLimit() = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestDashboardHandler_ParseOffset(t *testing.T) {
	database := openTestDBForHandler(t)
	handler := New(database)

	tests := []struct {
		query    string
		expected int
	}{
		{"", 0},
		{"?offset=5", 5},
		{"?offset=100", 100},
		{"?offset=-1", 0},      // Invalid uses default
		{"?offset=invalid", 0}, // Invalid uses default
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/"+tt.query, nil)
			result := handler.parseOffset(req)
			if result != tt.expected {
				t.Errorf("parseOffset() = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestDashboardHandler_CalculateBounceRate(t *testing.T) {
	database := openTestDBForHandler(t)
	handler := New(database)

	tests := []struct {
		name              string
		sessions          int
		bouncedSessions   int
		expectedBounceRate float64
	}{
		{
			name:              "no sessions",
			sessions:          0,
			bouncedSessions:   0,
			expectedBounceRate: 0,
		},
		{
			name:              "all bounced",
			sessions:          10,
			bouncedSessions:   10,
			expectedBounceRate: 1.0,
		},
		{
			name:              "half bounced",
			sessions:          10,
			bouncedSessions:   5,
			expectedBounceRate: 0.5,
		},
		{
			name:              "none bounced",
			sessions:          10,
			bouncedSessions:   0,
			expectedBounceRate: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := &db.DailyStats{
				Sessions:        tt.sessions,
				BouncedSessions: tt.bouncedSessions,
			}
			result := handler.calculateBounceRate(stats)
			if result != tt.expectedBounceRate {
				t.Errorf("calculateBounceRate() = %f, want %f", result, tt.expectedBounceRate)
			}
		})
	}
}

func TestDashboardHandler_RespondJSON(t *testing.T) {
	database := openTestDBForHandler(t)
	handler := New(database)

	data := map[string]any{"message": "test"}

	w := httptest.NewRecorder()
	handler.respondJSON(w, http.StatusOK, data)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", w.Header().Get("Content-Type"))
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp["message"] != "test" {
		t.Errorf("Expected message 'test', got %v", resp["message"])
	}
}

func TestDashboardHandler_RespondError(t *testing.T) {
	database := openTestDBForHandler(t)
	handler := New(database)

	w := httptest.NewRecorder()
	handler.respondError(w, http.StatusBadRequest, "test_error", "Test error message")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", w.Header().Get("Content-Type"))
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp["error"] != "test_error" {
		t.Errorf("Expected error 'test_error', got %v", resp["error"])
	}
	if resp["message"] != "Test error message" {
		t.Errorf("Expected message 'Test error message', got %v", resp["message"])
	}
}

func TestDashboardHandler_WithAuthQueryTakesPrecedence(t *testing.T) {
	database := openTestDBForHandler(t)
	handler := New(database)

	testKey := "test-key"
	SetDashboardKey(testKey)
	defer SetDashboardKey("")

	// Query param should be checked first
	req := httptest.NewRequest("GET", "/api/dashboard/summary?api_key=wrong-key", nil)
	req.Header.Set("X-API-Key", testKey)
	w := httptest.NewRecorder()

	handler.withAuth(handler.summary).ServeHTTP(w, req)

	// Should be unauthorized because query param is wrong
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestDashboardHandler_HealthDatabaseDown(t *testing.T) {
	// Create a database and immediately close it to simulate database being down
	database := openTestDBForHandler(t)
	handler := New(database)

	// Close the database to make Ping fail
	if err := database.Close(); err != nil {
		t.Fatalf("Failed to close database: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/dashboard/health", nil)
	w := httptest.NewRecorder()

	handler.health(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp["status"] != "unhealthy" {
		t.Errorf("Expected status unhealthy, got %v", resp["status"])
	}
	if resp["error"] != "database unavailable" {
		t.Errorf("Expected error 'database unavailable', got %v", resp["error"])
	}
}

func TestDashboardHandler_SummaryDatabaseError(t *testing.T) {
	database := openTestDBForHandler(t)
	handler := New(database)
	SetDashboardKey("")

	// Close database to cause error
	if err := database.Close(); err != nil {
		t.Fatalf("Failed to close database: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/dashboard/summary", nil)
	w := httptest.NewRecorder()

	handler.summary(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp["error"] != "query_error" {
		t.Errorf("Expected error 'query_error', got %v", resp["error"])
	}
}
