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

func openTestDBForHandler(t *testing.T) *sql.DB {
	t.Helper()

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
	handler := New(database, "")

	if handler == nil {
		t.Error("Expected non-nil handler")
	}
	if handler.dashboardKey != "" {
		t.Error("Expected empty dashboard key")
	}
}

func TestNewWithKey(t *testing.T) {
	database := openTestDBForHandler(t)
	handler := New(database, "test-key")

	if handler.dashboardKey != "test-key" {
		t.Errorf("Expected dashboard key 'test-key', got %s", handler.dashboardKey)
	}
}

func TestDashboardHandler_RegisterRoutes(t *testing.T) {
	database := openTestDBForHandler(t)
	handler := New(database, "")

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	routes := []string{
		"GET /api/dashboard/summary",
		"GET /api/dashboard/timeseries",
		"GET /api/dashboard/pages",
		"GET /api/dashboard/events",
		"GET /api/dashboard/sessions",
		"GET /api/dashboard/health",
	}

	for _, route := range routes {
		parts := strings.Split(route, " ")
		method := parts[0]
		path := parts[1]

		req := httptest.NewRequest(method, path, nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		if path == "/api/dashboard/health" {
			if w.Code != http.StatusOK {
				t.Errorf("Health endpoint failed: %d", w.Code)
			}
		}
	}
}

func TestDashboardHandler_WithAuthNoKey(t *testing.T) {
	database := openTestDBForHandler(t)
	handler := New(database, "")

	req := httptest.NewRequest("GET", "/api/dashboard/summary", nil)
	w := httptest.NewRecorder()

	handler.withAuth(handler.summary).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestDashboardHandler_WithAuthWithKey(t *testing.T) {
	database := openTestDBForHandler(t)
	testKey := "test-dashboard-key"
	handler := New(database, testKey)

	tests := []struct {
		name       string
		headerKey  string
		queryKey   string
		expectCode int
	}{
		{
			name:       "valid key via header",
			headerKey:  testKey,
			expectCode: http.StatusOK,
		},
		{
			name:       "valid key via query",
			queryKey:   testKey,
			expectCode: http.StatusOK,
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

func TestDashboardHandler_WithAuthHeaderTakesPrecedence(t *testing.T) {
	database := openTestDBForHandler(t)
	testKey := "test-key"
	handler := New(database, testKey)

	req := httptest.NewRequest("GET", "/api/dashboard/summary?api_key=wrong-key", nil)
	req.Header.Set("X-API-Key", testKey)
	w := httptest.NewRecorder()

	handler.withAuth(handler.summary).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 (header should take precedence), got %d", w.Code)
	}
}

func TestDashboardHandler_Summary(t *testing.T) {
	database := openTestDBForHandler(t)
	handler := New(database, "")

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
	if resp["bounceRate"] == nil {
		t.Error("Expected bounceRate in response")
	}
}

func TestDashboardHandler_Timeseries(t *testing.T) {
	database := openTestDBForHandler(t)
	handler := New(database, "")

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
	if len(data) == 0 {
		t.Error("Expected at least one data point")
	}
}

func TestDashboardHandler_Pages(t *testing.T) {
	database := openTestDBForHandler(t)
	handler := New(database, "")

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
	handler := New(database, "")

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
	handler := New(database, "")

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
	handler := New(database, "")

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

func TestParseDateRange(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		expectStart string
		expectEnd   string
		expectError bool
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
			start, end, err := parseDateRange(req)

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

func TestParseTimeRange(t *testing.T) {
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
			start, end, err := parseTimeRange(req)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if start > end {
				t.Error("Start should be <= end")
			}
		})
	}
}

func TestParseLimit(t *testing.T) {
	tests := []struct {
		query        string
		defaultLimit int
		expected     int
	}{
		{"", 10, 10},
		{"?limit=5", 10, 5},
		{"?limit=100", 10, 100},
		{"?limit=2000", 10, 1000},
		{"?limit=-1", 10, 10},
		{"?limit=0", 10, 10},
		{"?limit=invalid", 10, 10},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/"+tt.query, nil)
			result := parseLimit(req, tt.defaultLimit)
			if result != tt.expected {
				t.Errorf("parseLimit() = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestParseOffset(t *testing.T) {
	tests := []struct {
		query    string
		expected int
	}{
		{"", 0},
		{"?offset=5", 5},
		{"?offset=100", 100},
		{"?offset=-1", 0},
		{"?offset=invalid", 0},
		{"?offset=50000", 10000},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/"+tt.query, nil)
			result := parseOffset(req)
			if result != tt.expected {
				t.Errorf("parseOffset() = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestRespondJSON(t *testing.T) {
	data := map[string]any{"message": "test"}

	w := httptest.NewRecorder()
	RespondJSON(w, http.StatusOK, data)

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

func TestRespondError(t *testing.T) {
	w := httptest.NewRecorder()
	RespondError(w, http.StatusBadRequest, "test_error", "Test error message")

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

func TestDashboardHandler_HealthDatabaseDown(t *testing.T) {
	database := openTestDBForHandler(t)
	handler := New(database, "")

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
}

func TestDashboardHandler_SummaryDatabaseError(t *testing.T) {
	database := openTestDBForHandler(t)
	handler := New(database, "")

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
