package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/robindubreuil/analytics/internal/db"
	"github.com/robindubreuil/analytics/internal/testutil"
)

func TestIntegration_FullFlow(t *testing.T) {
	ts := testutil.NewTestServer(t)

	events := []map[string]any{
		{
			"sessionId": "sess1",
			"type":      "pageview",
			"url":       "/home",
			"referrer":  "https://google.com",
			"title":     "Home Page",
			"userAgent": "Mozilla/5.0",
			"timestamp": time.Now().UnixMilli(),
			"data":      testDataMap(),
		},
		{
			"sessionId": "sess1",
			"type":      "pageview",
			"url":       "/about",
			"timestamp": time.Now().UnixMilli() + 5000,
			"data": map[string]any{
				"screenWidth":    1920,
				"screenHeight":   1080,
				"viewportWidth":  1920,
				"viewportHeight": 900,
				"scrollDepth":    50,
				"engagementTime": 30,
			},
		},
		{
			"sessionId": "sess2",
			"type":      "pageview",
			"url":       "/contact",
			"timestamp": time.Now().UnixMilli(),
			"data":      testDataMap(),
		},
		{
			"sessionId": "sess1",
			"type":      "event",
			"event":     "button_click",
			"url":       "/about",
			"timestamp": time.Now().UnixMilli() + 10000,
			"data":      testDataMap(),
		},
	}

	body, err := json.Marshal(events)
	if err != nil {
		t.Fatalf("Failed to marshal events: %v", err)
	}

	req, _ := http.NewRequest("POST", ts.Server.URL+"/api/analytics", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-api-key")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	today := time.Now().Format("2006-01-02")
	summaryURL := fmt.Sprintf("%s/api/dashboard/summary?start=%s&end=%s", ts.Server.URL, today, today)
	req, _ = http.NewRequest("GET", summaryURL, nil)
	req.Header.Set("X-API-Key", ts.DashboardKey)

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to get summary: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var summary map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&summary); err != nil {
		t.Fatalf("Failed to decode summary: %v", err)
	}

	if summary["pageviews"] != float64(3) {
		t.Errorf("Expected 3 pageviews, got %v", summary["pageviews"])
	}

	pagesURL := fmt.Sprintf("%s/api/dashboard/pages?start=%s&end=%s", ts.Server.URL, today, today)
	req, _ = http.NewRequest("GET", pagesURL, nil)
	req.Header.Set("X-API-Key", ts.DashboardKey)

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to get pages: %v", err)
	}
	defer resp.Body.Close()

	var pagesResp map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&pagesResp); err != nil {
		t.Fatalf("Failed to decode pages: %v", err)
	}

	pages := pagesResp["pages"].([]any)
	if len(pages) == 0 {
		t.Error("Expected at least one page")
	}

	eventsURL := fmt.Sprintf("%s/api/dashboard/events?start=%s&end=%s", ts.Server.URL, today, today)
	req, _ = http.NewRequest("GET", eventsURL, nil)
	req.Header.Set("X-API-Key", ts.DashboardKey)

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to get events: %v", err)
	}
	defer resp.Body.Close()

	var eventsResp map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&eventsResp); err != nil {
		t.Fatalf("Failed to decode events: %v", err)
	}

	customEvents := eventsResp["events"].([]any)
	if len(customEvents) == 0 {
		t.Error("Expected at least one custom event")
	}
}

func TestIntegration_Authentication(t *testing.T) {
	ts := testutil.NewTestServer(t)

	tests := []struct {
		name       string
		endpoint   string
		apiKey     string
		expectCode int
	}{
		{
			name:       "ingest with valid key",
			endpoint:   "/api/analytics",
			apiKey:     "test-api-key",
			expectCode: http.StatusBadRequest,
		},
		{
			name:       "ingest with invalid key",
			endpoint:   "/api/analytics",
			apiKey:     "wrong-key",
			expectCode: http.StatusUnauthorized,
		},
		{
			name:       "ingest without key",
			endpoint:   "/api/analytics",
			apiKey:     "",
			expectCode: http.StatusUnauthorized,
		},
		{
			name:       "dashboard with valid key",
			endpoint:   "/api/dashboard/health",
			apiKey:     ts.DashboardKey,
			expectCode: http.StatusOK,
		},
		{
			name:       "dashboard with invalid key",
			endpoint:   "/api/dashboard/summary",
			apiKey:     "wrong-key",
			expectCode: http.StatusUnauthorized,
		},
		{
			name:       "dashboard without key",
			endpoint:   "/api/dashboard/summary",
			apiKey:     "",
			expectCode: http.StatusUnauthorized,
		},
		{
			name:       "health endpoint no auth needed",
			endpoint:   "/api/dashboard/health",
			apiKey:     "",
			expectCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := ts.Server.URL + tt.endpoint
			var body bytes.Reader

			if tt.endpoint == "/api/analytics" {
				events := []map[string]any{}
				jsonBytes, _ := json.Marshal(events)
				body = *bytes.NewReader(jsonBytes)
			} else {
				body = *bytes.NewReader(nil)
			}

			req, _ := http.NewRequest("POST", url, &body)
			if tt.endpoint != "/api/analytics" {
				req, _ = http.NewRequest("GET", url, nil)
			}
			req.Header.Set("Content-Type", "application/json")
			if tt.apiKey != "" {
				req.Header.Set("X-API-Key", tt.apiKey)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Failed to send request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectCode {
				t.Errorf("Expected status %d, got %d", tt.expectCode, resp.StatusCode)
			}
		})
	}
}

func TestIntegration_RateLimiting(t *testing.T) {
	ts := testutil.NewTestServer(t)

	event := []map[string]any{
		{
			"sessionId": "sess-rate",
			"type":      "pageview",
			"url":       "/test",
			"timestamp": time.Now().UnixMilli(),
			"data":      testDataMap(),
		},
	}

	body, _ := json.Marshal(event)

	successCount := 0
	rateLimitedCount := 0

	for i := 0; i < 105; i++ {
		req, _ := http.NewRequest("POST", ts.Server.URL+"/api/analytics", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", "test-api-key")
		req.Header.Set("X-Forwarded-For", "10.0.0.1")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Request %d failed: %v", i, err)
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			successCount++
		} else if resp.StatusCode == http.StatusTooManyRequests {
			rateLimitedCount++
		}
	}

	if rateLimitedCount == 0 {
		t.Errorf("Expected some requests to be rate limited, but all %d succeeded", successCount)
	}
	if successCount > 100 {
		t.Errorf("Expected at most 100 successful requests, got %d", successCount)
	}
}

func TestIntegration_CORS(t *testing.T) {
	ts := testutil.NewTestServer(t)

	req, _ := http.NewRequest("OPTIONS", ts.Server.URL+"/api/analytics", nil)
	req.Header.Set("Origin", "https://example.com")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", resp.StatusCode)
	}

	origin := resp.Header.Get("Access-Control-Allow-Origin")
	if origin != "*" && origin != "https://example.com" {
		t.Errorf("Expected origin header (* or https://example.com), got %s", origin)
	}
	if methods := resp.Header.Get("Access-Control-Allow-Methods"); methods == "" {
		t.Error("Expected methods header")
	}
}

func TestIntegration_PanicRecovery(t *testing.T) {
	ts := testutil.NewTestServer(t)

	req, _ := http.NewRequest("GET", ts.Server.URL+"/api/test-panic", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status 500 from panic endpoint, got %d", resp.StatusCode)
	}

	req, _ = http.NewRequest("GET", ts.Server.URL+"/api/dashboard/health", nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to send health check after panic: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 from health after panic, got %d", resp.StatusCode)
	}
}

func TestIntegration_SessionAggregation(t *testing.T) {
	ts := testutil.NewTestServer(t)

	sessionID := "sess-aggregation"

	events := []map[string]any{
		{
			"sessionId": sessionID,
			"type":      "pageview",
			"url":       "/page1",
			"timestamp": time.Now().UnixMilli(),
			"data":      testDataMap(),
		},
		{
			"sessionId": sessionID,
			"type":      "pageview",
			"url":       "/page2",
			"timestamp": time.Now().UnixMilli() + 5000,
			"data":      testDataMap(),
		},
		{
			"sessionId": sessionID,
			"type":      "pageview",
			"url":       "/page3",
			"timestamp": time.Now().UnixMilli() + 10000,
			"data":      testDataMap(),
		},
	}

	for _, event := range events {
		body, _ := json.Marshal([]map[string]any{event})
		req, _ := http.NewRequest("POST", ts.Server.URL+"/api/analytics", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", "test-api-key")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to send request: %v", err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	}

	var pageviews int
	err := ts.DB.QueryRow("SELECT pageviews FROM sessions WHERE session_id = ?", sessionID).Scan(&pageviews)
	if err != nil {
		t.Fatalf("Failed to query session: %v", err)
	}

	if pageviews != 3 {
		t.Errorf("Expected 3 pageviews for session, got %d", pageviews)
	}
}

func TestIntegration_DataRetention(t *testing.T) {
	ts := testutil.NewTestServer(t)

	oldTime := time.Now().AddDate(0, 0, -100).UnixMilli()
	_, err := ts.DB.Exec(
		"INSERT INTO events (session_id, type, url, timestamp, created_at) VALUES (?, ?, ?, ?, ?)",
		"old-session", "pageview", "/old-page", oldTime, oldTime,
	)
	if err != nil {
		t.Fatalf("Failed to insert old event: %v", err)
	}

	var count int
	err = ts.DB.QueryRow("SELECT COUNT(*) FROM events").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count events: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 event, got %d", count)
	}

	deleted, err := db.DeleteOldEvents(ts.DB, 90)
	if err != nil {
		t.Fatalf("Failed to delete old events: %v", err)
	}

	if deleted != 1 {
		t.Errorf("Expected 1 deleted event, got %d", deleted)
	}

	err = ts.DB.QueryRow("SELECT COUNT(*) FROM events").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count events: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 events after retention, got %d", count)
	}
}

func TestIntegration_MultipleSessions(t *testing.T) {
	ts := testutil.NewTestServer(t)

	numSessions := 50
	events := make([]map[string]any, numSessions)

	now := time.Now().UnixMilli()
	for i := 0; i < numSessions; i++ {
		events[i] = map[string]any{
			"sessionId": fmt.Sprintf("sess-%d", i),
			"type":      "pageview",
			"url":       fmt.Sprintf("/page-%d", i),
			"timestamp": now + int64(i)*1000,
			"data":      testDataMap(),
		}
	}

	body, _ := json.Marshal(events)
	req, _ := http.NewRequest("POST", ts.Server.URL+"/api/analytics", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-api-key")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var sessionCount int
	err = ts.DB.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&sessionCount)
	if err != nil {
		t.Fatalf("Failed to count sessions: %v", err)
	}

	if sessionCount != numSessions {
		t.Errorf("Expected %d sessions, got %d", numSessions, sessionCount)
	}
}

func TestIntegration_ConcurrentIngest(t *testing.T) {
	ts := testutil.NewTestServer(t)

	const numGoroutines = 20
	const eventsPerGoroutine = 5

	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()

			events := make([]map[string]any, eventsPerGoroutine)
			now := time.Now().UnixMilli()

			for j := 0; j < eventsPerGoroutine; j++ {
				events[j] = map[string]any{
					"sessionId": fmt.Sprintf("sess-%d-%d", id, j),
					"type":      "pageview",
					"url":       fmt.Sprintf("/page-%d", j),
					"timestamp": now + int64(j)*1000,
					"data":      testDataMap(),
				}
			}

			body, _ := json.Marshal(events)
			req, _ := http.NewRequest("POST", ts.Server.URL+"/api/analytics", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-API-Key", "test-api-key")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Errorf("Goroutine %d: Request failed: %v", id, err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Goroutine %d: Expected status 200, got %d", id, resp.StatusCode)
			}
		}(i)
	}

	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	var count int
	err := ts.DB.QueryRow("SELECT COUNT(*) FROM events").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count events: %v", err)
	}

	expected := numGoroutines * eventsPerGoroutine
	if count != expected {
		t.Errorf("Expected %d events, got %d", expected, count)
	}
}

func TestIntegration_ErrorResponses(t *testing.T) {
	ts := testutil.NewTestServer(t)

	tests := []struct {
		name       string
		body       string
		expectCode int
		checkError string
	}{
		{
			name:       "empty body",
			body:       "",
			expectCode: http.StatusBadRequest,
			checkError: "empty_body",
		},
		{
			name:       "invalid JSON",
			body:       "not json",
			expectCode: http.StatusBadRequest,
			checkError: "invalid_json",
		},
		{
			name:       "empty array",
			body:       "[]",
			expectCode: http.StatusBadRequest,
			checkError: "no_events",
		},
		{
			name:       "missing required fields",
			body:       `[{"type":"pageview"}]`,
			expectCode: http.StatusBadRequest,
			checkError: "validation_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("POST", ts.Server.URL+"/api/analytics", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-API-Key", "test-api-key")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Failed to send request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectCode {
				t.Errorf("Expected status %d, got %d", tt.expectCode, resp.StatusCode)
			}

			var errResp map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
				t.Fatalf("Failed to decode error response: %v", err)
			}

			if tt.checkError != "" {
				if errResp["error"] != tt.checkError {
					t.Errorf("Expected error code %s, got %v", tt.checkError, errResp["error"])
				}
			}
		})
	}
}

func TestIntegration_Pagination(t *testing.T) {
	ts := testutil.NewTestServer(t)

	numSessions := 25
	events := make([]map[string]any, numSessions)
	now := time.Now().UnixMilli()

	for i := 0; i < numSessions; i++ {
		events[i] = map[string]any{
			"sessionId": fmt.Sprintf("sess-page-%d", i),
			"type":      "pageview",
			"url":       "/page",
			"timestamp": now + int64(i)*1000,
			"data":      testDataMap(),
		}
	}

	body, _ := json.Marshal(events)
	req, _ := http.NewRequest("POST", ts.Server.URL+"/api/analytics", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-api-key")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to ingest events: %d", resp.StatusCode)
	}

	startTime := now - 1000
	endTime := now + int64(numSessions)*1000 + 1000

	sessionsURL := fmt.Sprintf("%s/api/dashboard/sessions?start=%d&end=%d&limit=10&offset=0",
		ts.Server.URL, startTime, endTime)
	req, _ = http.NewRequest("GET", sessionsURL, nil)
	req.Header.Set("X-API-Key", ts.DashboardKey)

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to get sessions: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var page1 map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&page1); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	sessions1 := page1["sessions"].([]any)
	if len(sessions1) != 10 {
		t.Errorf("Expected 10 sessions on first page, got %d", len(sessions1))
	}

	sessionsURL = fmt.Sprintf("%s/api/dashboard/sessions?start=%d&end=%d&limit=10&offset=10",
		ts.Server.URL, startTime, endTime)
	req, _ = http.NewRequest("GET", sessionsURL, nil)
	req.Header.Set("X-API-Key", ts.DashboardKey)

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to get sessions: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var page2 map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&page2); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	sessions2 := page2["sessions"].([]any)
	if len(sessions2) != 10 {
		t.Errorf("Expected 10 sessions on second page, got %d", len(sessions2))
	}
}

func testDataMap() map[string]any {
	return map[string]any{
		"screenWidth":    1920,
		"screenHeight":   1080,
		"viewportWidth":  1920,
		"viewportHeight": 900,
		"scrollDepth":    0,
		"engagementTime": 0,
	}
}
