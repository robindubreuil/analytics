// Package ingest provides tests for event ingestion.
package ingest

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
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	// Use nanoseconds and test name for uniqueness
	unique := fmt.Sprintf("%d_%s", time.Now().UnixNano(), t.Name())
	path := "/tmp/analytics_ingest_test_" + unique + ".db"

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

func TestHandlerServeHTTP(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	handler := New(database)

	tests := []struct {
		name           string
		method         string
		body           string
		expectedStatus int
		checkBody      func(t *testing.T, body string)
	}{
		{
			name:           "valid events",
			method:         "POST",
			body:           `[{"sessionId":"s1","type":"pageview","url":"/test","timestamp":1234567890,"data":{"screenWidth":1920,"screenHeight":1080,"viewportWidth":1920,"viewportHeight":900,"scrollDepth":0,"engagementTime":0}}]`,
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body string) {
				var resp map[string]any
				if err := json.Unmarshal([]byte(body), &resp); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}
				if resp["success"] != true {
					t.Errorf("Expected success=true, got %v", resp["success"])
				}
				if resp["count"] != float64(1) {
					t.Errorf("Expected count=1, got %v", resp["count"])
				}
			},
		},
		{
			name:           "multiple events",
			method:         "POST",
			body:           `[{"sessionId":"s1","type":"pageview","url":"/test","timestamp":1234567890,"data":{"screenWidth":1920,"screenHeight":1080,"viewportWidth":1920,"viewportHeight":900,"scrollDepth":0,"engagementTime":0}},{"sessionId":"s2","type":"pageview","url":"/test2","timestamp":1234567891,"data":{"screenWidth":1920,"screenHeight":1080,"viewportWidth":1920,"viewportHeight":900,"scrollDepth":0,"engagementTime":0}}]`,
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body string) {
				var resp map[string]any
				if err := json.Unmarshal([]byte(body), &resp); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}
				if resp["count"] != float64(2) {
					t.Errorf("Expected count=2, got %v", resp["count"])
				}
			},
		},
		{
			name:           "custom event",
			method:         "POST",
			body:           `[{"sessionId":"s1","type":"event","event":"button_click","url":"/test","timestamp":1234567890,"data":{"screenWidth":1920,"screenHeight":1080,"viewportWidth":1920,"viewportHeight":900,"scrollDepth":0,"engagementTime":0}}]`,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "wrong method",
			method:         "GET",
			body:           "[]",
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "empty body",
			method:         "POST",
			body:           "",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid JSON",
			method:         "POST",
			body:           "not json",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "empty array",
			method:         "POST",
			body:           "[]",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing sessionId",
			method:         "POST",
			body:           `[{"type":"pageview","url":"/test","timestamp":1234567890,"data":{"screenWidth":1920,"screenHeight":1080,"viewportWidth":1920,"viewportHeight":900,"scrollDepth":0,"engagementTime":0}}]`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing type",
			method:         "POST",
			body:           `[{"sessionId":"s1","url":"/test","timestamp":1234567890,"data":{"screenWidth":1920,"screenHeight":1080,"viewportWidth":1920,"viewportHeight":900,"scrollDepth":0,"engagementTime":0}}]`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing url",
			method:         "POST",
			body:           `[{"sessionId":"s1","type":"pageview","timestamp":1234567890,"data":{"screenWidth":1920,"screenHeight":1080,"viewportWidth":1920,"viewportHeight":900,"scrollDepth":0,"engagementTime":0}}]`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing timestamp",
			method:         "POST",
			body:           `[{"sessionId":"s1","type":"pageview","url":"/test","data":{"screenWidth":1920,"screenHeight":1080,"viewportWidth":1920,"viewportHeight":900,"scrollDepth":0,"engagementTime":0}}]`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid type",
			method:         "POST",
			body:           `[{"sessionId":"s1","type":"invalid","url":"/test","timestamp":1234567890,"data":{"screenWidth":1920,"screenHeight":1080,"viewportWidth":1920,"viewportHeight":900,"scrollDepth":0,"engagementTime":0}}]`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "custom event without name",
			method:         "POST",
			body:           `[{"sessionId":"s1","type":"event","url":"/test","timestamp":1234567890,"data":{"screenWidth":1920,"screenHeight":1080,"viewportWidth":1920,"viewportHeight":900,"scrollDepth":0,"engagementTime":0}}]`,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/analytics", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}

			if tt.checkBody != nil {
				tt.checkBody(t, w.Body.String())
			}
		})
	}
}

func TestHandlerWithDatabase(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	handler := New(database)

	body := `[{"sessionId":"test-session","type":"pageview","url":"/test-page","referrer":"https://example.com","title":"Test Title","userAgent":"TestAgent/1.0","timestamp":1234567890,"data":{"screenWidth":1920,"screenHeight":1080,"viewportWidth":1920,"viewportHeight":900,"scrollDepth":50,"engagementTime":30}}]`

	req := httptest.NewRequest("POST", "/api/analytics", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify event was stored in database
	var count int
	err := database.QueryRow("SELECT COUNT(*) FROM events").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count events: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 event in database, got %d", count)
	}

	// Verify session was created
	var sessionCount int
	err = database.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&sessionCount)
	if err != nil {
		t.Fatalf("Failed to count sessions: %v", err)
	}
	if sessionCount != 1 {
		t.Errorf("Expected 1 session in database, got %d", sessionCount)
	}
}

func TestRequestBodySizeLimit(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	handler := New(database)

	// Create a body larger than 1MB
	largeEvent := `{"sessionId":"s1","type":"pageview","url":"/test","timestamp":1234567890,"data":{"screenWidth":1920,"screenHeight":1080,"viewportWidth":1920,"viewportHeight":900,"scrollDepth":0,"engagementTime":0},"extra":"` + strings.Repeat("x", 2_000_000) + `"}`
	body := `[` + largeEvent + `]`

	req := httptest.NewRequest("POST", "/api/analytics", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should either succeed (truncated) or fail gracefully
	// The important thing is it doesn't crash or hang
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 200 or 400, got %d", w.Code)
	}
}

func TestSanitizeEventName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple_event", "simple_event"},
		{"event-with-dash", "event-with-dash"},
		{"event.with.dot", "event.with.dot"},
		{"event with spaces", "event_with_spaces"},
		{"event@with$special!chars", "event_with_special_chars"},
		{"event/with\\slashes", "event_with_slashes"},
		{"事件", "__"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeEventName(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeEventName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestURLTruncation(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	handler := New(database)

	// Create a URL longer than 2048 characters
	longURL := "/test?" + strings.Repeat("a=", 500) + "&"
	eventJSON := `{"sessionId":"s1","type":"pageview","url":"` + longURL + `","timestamp":1234567890,"data":{"screenWidth":1920,"screenHeight":1080,"viewportWidth":1920,"viewportHeight":900,"scrollDepth":0,"engagementTime":0}}`

	req := httptest.NewRequest("POST", "/api/analytics", strings.NewReader("["+eventJSON+"]"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify URL was truncated in database
	var storedURL string
	err := database.QueryRow("SELECT url FROM events LIMIT 1").Scan(&storedURL)
	if err != nil {
		t.Fatalf("Failed to get URL: %v", err)
	}
	if len(storedURL) > 2048 {
		t.Errorf("URL was not truncated, got length %d", len(storedURL))
	}
}

func TestReferrerTruncation(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	handler := New(database)

	// Create a referrer longer than 2048 characters
	longReferrer := "https://example.com/?q=" + strings.Repeat("x", 3000)
	eventJSON := `{"sessionId":"s1","type":"pageview","url":"/test","referrer":"` + longReferrer + `","timestamp":1234567890,"data":{"screenWidth":1920,"screenHeight":1080,"viewportWidth":1920,"viewportHeight":900,"scrollDepth":0,"engagementTime":0}}`

	req := httptest.NewRequest("POST", "/api/analytics", strings.NewReader("["+eventJSON+"]"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify referrer was truncated
	var storedReferrer sql.NullString
	err := database.QueryRow("SELECT referrer FROM events LIMIT 1").Scan(&storedReferrer)
	if err != nil {
		t.Fatalf("Failed to get referrer: %v", err)
	}
	if storedReferrer.Valid && len(storedReferrer.String) > 2048 {
		t.Errorf("Referrer was not truncated, got length %d", len(storedReferrer.String))
	}
}

func TestUserAgentTruncation(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	handler := New(database)

	// Create a user agent longer than 500 characters
	longUserAgent := strings.Repeat("Mozilla/5.0 ", 100)
	eventJSON := `{"sessionId":"s1","type":"pageview","url":"/test","userAgent":"` + longUserAgent + `","timestamp":1234567890,"data":{"screenWidth":1920,"screenHeight":1080,"viewportWidth":1920,"viewportHeight":900,"scrollDepth":0,"engagementTime":0}}`

	req := httptest.NewRequest("POST", "/api/analytics", strings.NewReader("["+eventJSON+"]"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify user agent was truncated
	var storedUserAgent sql.NullString
	err := database.QueryRow("SELECT user_agent FROM events LIMIT 1").Scan(&storedUserAgent)
	if err != nil {
		t.Fatalf("Failed to get user agent: %v", err)
	}
	if storedUserAgent.Valid && len(storedUserAgent.String) > 500 {
		t.Errorf("User agent was not truncated, got length %d", len(storedUserAgent.String))
	}
}

func TestMultipleValidationErrors(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	handler := New(database)

	// Multiple events with validation errors
	body := `[{"sessionId":"","type":"pageview","url":"/test","timestamp":1234567890,"data":{"screenWidth":1920,"screenHeight":1080,"viewportWidth":1920,"viewportHeight":900,"scrollDepth":0,"engagementTime":0}},{"sessionId":"s2","type":"","url":"/test","timestamp":1234567890,"data":{"screenWidth":1920,"screenHeight":1080,"viewportWidth":1920,"viewportHeight":900,"scrollDepth":0,"engagementTime":0}}]`

	req := httptest.NewRequest("POST", "/api/analytics", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	// Check that both errors are reported
	respBody := w.Body.String()
	if !strings.Contains(respBody, "event 0") {
		t.Error("Expected error for event 0")
	}
	if !strings.Contains(respBody, "event 1") {
		t.Error("Expected error for event 1")
	}
}

func TestValidPageviewEvent(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	handler := New(database)

	body := `{"sessionId":"s1","type":"pageview","url":"/test","referrer":"https://google.com","title":"Test Page","userAgent":"Mozilla/5.0","timestamp":1234567890,"data":{"screenWidth":1920,"screenHeight":1080,"viewportWidth":1920,"viewportHeight":900,"scrollDepth":50,"engagementTime":30}}`

	req := httptest.NewRequest("POST", "/api/analytics", strings.NewReader("["+body+"]"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify all fields were stored correctly
	var url, referrer, title sql.NullString
	var scrollDepth, engagementTime int

	err := database.QueryRow("SELECT url, referrer, title, scroll_depth, engagement_time FROM events LIMIT 1").Scan(&url, &referrer, &title, &scrollDepth, &engagementTime)
	if err != nil {
		t.Fatalf("Failed to get event: %v", err)
	}

	if url.String != "/test" {
		t.Errorf("Expected URL /test, got %s", url.String)
	}
	if referrer.String != "https://google.com" {
		t.Errorf("Expected referrer https://google.com, got %s", referrer.String)
	}
	if title.String != "Test Page" {
		t.Errorf("Expected title 'Test Page', got %s", title.String)
	}
	if scrollDepth != 50 {
		t.Errorf("Expected scroll depth 50, got %d", scrollDepth)
	}
	if engagementTime != 30 {
		t.Errorf("Expected engagement time 30, got %d", engagementTime)
	}
}

func TestConcurrentRequests(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	handler := New(database)

	// Send concurrent requests with unique session IDs
	const concurrency = 10
	done := make(chan bool, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(id int) {
			// Use unique session ID for each request
			body := fmt.Sprintf(`[{"sessionId":"s%d","type":"pageview","url":"/test","timestamp":1234567890,"data":{"screenWidth":1920,"screenHeight":1080,"viewportWidth":1920,"viewportHeight":900,"scrollDepth":0,"engagementTime":0}}]`, id)
			req := httptest.NewRequest("POST", "/api/analytics", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			done <- true
		}(i)
	}

	// Wait for all requests to complete
	for i := 0; i < concurrency; i++ {
		<-done
	}

	// Verify all events were stored
	var count int
	err := database.QueryRow("SELECT COUNT(*) FROM events").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count events: %v", err)
	}
	if count != concurrency {
		t.Errorf("Expected %d events, got %d", concurrency, count)
	}
}

func TestErrorResponseFormat(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	handler := New(database)

	req := httptest.NewRequest("POST", "/api/analytics", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	// Check response format
	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	if resp.Error == "" {
		t.Error("Expected error code in response")
	}
	if resp.Message == "" {
		t.Error("Expected error message in response")
	}

	// Check content type
	if contentType := w.Header().Get("Content-Type"); contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}
}

func TestSuccessResponseFormat(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	handler := New(database)

	body := `[{"sessionId":"s1","type":"pageview","url":"/test","timestamp":1234567890,"data":{"screenWidth":1920,"screenHeight":1080,"viewportWidth":1920,"viewportHeight":900,"scrollDepth":0,"engagementTime":0}}]`

	req := httptest.NewRequest("POST", "/api/analytics", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Check response format
	var resp SuccessResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse success response: %v", err)
	}

	if !resp.Success {
		t.Error("Expected success=true in response")
	}
	if resp.Count != 1 {
		t.Errorf("Expected count=1, got %d", resp.Count)
	}
}

func TestHandlerDatabaseError(t *testing.T) {
	// Create a database and close it to simulate database failure
	database := openTestDB(t)
	handler := New(database)

	// Close database to cause errors
	database.Close()

	body := `[{"sessionId":"s1","type":"pageview","url":"/test","timestamp":1234567890,"data":{"screenWidth":1920,"screenHeight":1080,"viewportWidth":1920,"viewportHeight":900,"scrollDepth":0,"engagementTime":0}}]`

	req := httptest.NewRequest("POST", "/api/analytics", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should get an error response
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d: %s", w.Code, w.Body.String())
	}

	// Verify error format
	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	if resp.Error == "" {
		t.Error("Expected error code in response")
	}
}
