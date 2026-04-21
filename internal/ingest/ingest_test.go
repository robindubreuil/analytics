package ingest

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/robindubreuil/analytics/internal/db"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

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

func ts() int64 { return time.Now().UnixMilli() }

func makeEventJSON(sid, evType, url string) string {
	return fmt.Sprintf(`{"sessionId":"%s","type":"%s","url":"%s","timestamp":%d,"data":{"screenWidth":1920,"screenHeight":1080,"viewportWidth":1920,"viewportHeight":900,"scrollDepth":0,"engagementTime":0}}`, sid, evType, url, ts())
}

func makeEventJSONWithExtra(sid, evType, url string, extra string) string {
	return fmt.Sprintf(`{"sessionId":"%s","type":"%s","url":"%s","timestamp":%d,"data":{"screenWidth":1920,"screenHeight":1080,"viewportWidth":1920,"viewportHeight":900,"scrollDepth":0,"engagementTime":0}%s}`, sid, evType, url, ts(), extra)
}

func TestHandlerServeHTTP(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	handler := New(database, 3)

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
			body:           "[" + makeEventJSON("s1", "pageview", "/test") + "]",
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
			body:           "[" + makeEventJSON("s1", "pageview", "/test") + "," + makeEventJSON("s2", "pageview", "/test2") + "]",
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
			name:   "custom event",
			method: "POST",
			body: fmt.Sprintf(`[{"sessionId":"s1","type":"event","event":"button_click","url":"/test","timestamp":%d,"data":{"screenWidth":1920,"screenHeight":1080,"viewportWidth":1920,"viewportHeight":900,"scrollDepth":0,"engagementTime":0}}]`, ts()),
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
			name:           "invalid json",
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/analytics", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.checkBody != nil {
				tt.checkBody(t, w.Body.String())
			}
		})
	}
}

func TestValidation(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	handler := New(database, 3)

	tests := []struct {
		name           string
		body           string
		expectedStatus int
	}{
		{
			name:           "missing sessionId",
			body:           fmt.Sprintf(`[{"type":"pageview","url":"/test","timestamp":%d,"data":{"screenWidth":1920,"screenHeight":1080,"viewportWidth":1920,"viewportHeight":900,"scrollDepth":0,"engagementTime":0}}]`, ts()),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing type",
			body:           fmt.Sprintf(`[{"sessionId":"s1","url":"/test","timestamp":%d,"data":{"screenWidth":1920,"screenHeight":1080,"viewportWidth":1920,"viewportHeight":900,"scrollDepth":0,"engagementTime":0}}]`, ts()),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing url",
			body:           fmt.Sprintf(`[{"sessionId":"s1","type":"pageview","timestamp":%d,"data":{"screenWidth":1920,"screenHeight":1080,"viewportWidth":1920,"viewportHeight":900,"scrollDepth":0,"engagementTime":0}}]`, ts()),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing timestamp",
			body:           `[{"sessionId":"s1","type":"pageview","url":"/test","data":{"screenWidth":1920,"screenHeight":1080,"viewportWidth":1920,"viewportHeight":900,"scrollDepth":0,"engagementTime":0}}]`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid type",
			body:           fmt.Sprintf(`[{"sessionId":"s1","type":"invalid","url":"/test","timestamp":%d,"data":{"screenWidth":1920,"screenHeight":1080,"viewportWidth":1920,"viewportHeight":900,"scrollDepth":0,"engagementTime":0}}]`, ts()),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "event without name",
			body:           fmt.Sprintf(`[{"sessionId":"s1","type":"event","url":"/test","timestamp":%d,"data":{"screenWidth":1920,"screenHeight":1080,"viewportWidth":1920,"viewportHeight":900,"scrollDepth":0,"engagementTime":0}}]`, ts()),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "timestamp in future",
			body:           `[{"sessionId":"s1","type":"pageview","url":"/test","timestamp":9999999999999,"data":{"screenWidth":1920,"screenHeight":1080,"viewportWidth":1920,"viewportHeight":900,"scrollDepth":0,"engagementTime":0}}]`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "timestamp in 1970",
			body:           `[{"sessionId":"s1","type":"pageview","url":"/test","timestamp":1234567890,"data":{"screenWidth":1920,"screenHeight":1080,"viewportWidth":1920,"viewportHeight":900,"scrollDepth":0,"engagementTime":0}}]`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "sessionId too long",
			body:           fmt.Sprintf(`[{"sessionId":"%s","type":"pageview","url":"/test","timestamp":%d,"data":{"screenWidth":1920,"screenHeight":1080,"viewportWidth":1920,"viewportHeight":900,"scrollDepth":0,"engagementTime":0}}]`, strings.Repeat("x", 200), ts()),
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/analytics", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d: %s", tt.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestHandlerFullEvent(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	handler := New(database, 3)

	now := ts()
	body := fmt.Sprintf(`[{"sessionId":"test-session","type":"pageview","url":"/test-page","referrer":"https://example.com","title":"Test Title","userAgent":"TestAgent/1.0","timestamp":%d,"data":{"screenWidth":1920,"screenHeight":1080,"viewportWidth":1920,"viewportHeight":900,"scrollDepth":50,"engagementTime":30}}]`, now)

	req := httptest.NewRequest("POST", "/api/analytics", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp["success"] != true {
		t.Errorf("Expected success=true, got %v", resp["success"])
	}
	if resp["count"] != float64(1) {
		t.Errorf("Expected count=1, got %v", resp["count"])
	}
}

func TestHandlerBodySizeLimit(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	handler := New(database, 3)

	largeEvent := makeEventJSONWithExtra("s1", "pageview", "/test", fmt.Sprintf(`,"extra":"%s"`, strings.Repeat("x", 2_000_000)))
	body := "[" + largeEvent + "]"

	req := httptest.NewRequest("POST", "/api/analytics", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for oversized body, got %d", w.Code)
	}
}

func TestHandlerURLSanitization(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	handler := New(database, 3)

	longURL := "/test?param=" + strings.Repeat("x", 3000)
	body := fmt.Sprintf(`[{"sessionId":"s1","type":"pageview","url":"%s","timestamp":%d,"data":{"screenWidth":1920,"screenHeight":1080,"viewportWidth":1920,"viewportHeight":900,"scrollDepth":0,"engagementTime":0}}]`, longURL, ts())

	req := httptest.NewRequest("POST", "/api/analytics", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlerReferrerSanitization(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	handler := New(database, 3)

	longReferrer := "https://example.com/" + strings.Repeat("x", 3000)
	body := fmt.Sprintf(`[{"sessionId":"s1","type":"pageview","url":"/test","referrer":"%s","timestamp":%d,"data":{"screenWidth":1920,"screenHeight":1080,"viewportWidth":1920,"viewportHeight":900,"scrollDepth":0,"engagementTime":0}}]`, longReferrer, ts())

	req := httptest.NewRequest("POST", "/api/analytics", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlerUserAgentSanitization(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	handler := New(database, 3)

	longUA := "Mozilla/" + strings.Repeat("x", 600)
	body := fmt.Sprintf(`[{"sessionId":"s1","type":"pageview","url":"/test","userAgent":"%s","timestamp":%d,"data":{"screenWidth":1920,"screenHeight":1080,"viewportWidth":1920,"viewportHeight":900,"scrollDepth":0,"engagementTime":0}}]`, longUA, ts())

	req := httptest.NewRequest("POST", "/api/analytics", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlerMultipleValidationErrors(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	handler := New(database, 3)

	now := ts()
	body := fmt.Sprintf(`[{"sessionId":"","type":"pageview","url":"/test","timestamp":%d,"data":{"screenWidth":1920,"screenHeight":1080,"viewportWidth":1920,"viewportHeight":900,"scrollDepth":0,"engagementTime":0}},{"sessionId":"s2","type":"","url":"/test","timestamp":%d,"data":{"screenWidth":1920,"screenHeight":1080,"viewportWidth":1920,"viewportHeight":900,"scrollDepth":0,"engagementTime":0}}]`, now, now)

	req := httptest.NewRequest("POST", "/api/analytics", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp["error"] != "validation_error" {
		t.Errorf("Expected validation_error, got %v", resp["error"])
	}
}

func TestHandlerSingleObject(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	handler := New(database, 3)

	now := ts()
	body := fmt.Sprintf(`{"sessionId":"s1","type":"pageview","url":"/test","referrer":"https://google.com","title":"Test Page","userAgent":"Mozilla/5.0","timestamp":%d,"data":{"screenWidth":1920,"screenHeight":1080,"viewportWidth":1920,"viewportHeight":900,"scrollDepth":50,"engagementTime":30}}`, now)

	req := httptest.NewRequest("POST", "/api/analytics", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for non-array JSON, got %d", w.Code)
	}
}

func TestConcurrentRequests(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	handler := New(database, 3)

	var wg sync.WaitGroup
	errors := make(chan error, 20)

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			body := fmt.Sprintf(`[{"sessionId":"s%d","type":"pageview","url":"/test","timestamp":%d,"data":{"screenWidth":1920,"screenHeight":1080,"viewportWidth":1920,"viewportHeight":900,"scrollDepth":0,"engagementTime":0}}]`, id, ts())

			req := httptest.NewRequest("POST", "/api/analytics", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				errors <- fmt.Errorf("request %d: expected 200, got %d", id, w.Code)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}
}

func TestSanitizeEventName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"click", "click"},
		{"button_click", "button_click"},
		{"scroll-depth", "scroll-depth"},
		{"event.name", "event.name"},
		{"click!", "click_"},
		{"<script>", "_script_"},
		{"a b c", "a_b_c"},
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

func TestClampPositive(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{0, 0},
		{100, 100},
		{-1, 0},
		{-100, 0},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d", tt.input), func(t *testing.T) {
			result := clampPositive(tt.input)
			if result != tt.expected {
				t.Errorf("clampPositive(%d) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestBatchSizeLimit(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	handler := New(database, 3)

	events := make([]string, 101)
	for i := range events {
		events[i] = makeEventJSON(fmt.Sprintf("s%d", i), "pageview", "/test")
	}
	body := "[" + strings.Join(events, ",") + "]"

	req := httptest.NewRequest("POST", "/api/analytics", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for oversized batch, got %d", w.Code)
	}
}
