// Package api provides tests for HTTP middleware.
package api

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestAPIKeyWithNoKey(t *testing.T) {
	handler := APIKey("")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	if w.Body.String() != "success" {
		t.Errorf("Expected body 'success', got %s", w.Body.String())
	}
}

func TestAPIKeyWithValidKey(t *testing.T) {
	handler := APIKey("secret-key")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	tests := []struct {
		name   string
		header string
		query  string
	}{
		{
			name:   "via header",
			header: "secret-key",
		},
		{
			name:  "via query param",
			query: "secret-key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.header != "" {
				req.Header.Set("X-API-Key", tt.header)
			}
			if tt.query != "" {
				req.URL.RawQuery = "api_key=" + tt.query
			}
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
			}
		})
	}
}

func TestAPIKeyWithInvalidKey(t *testing.T) {
	handler := APIKey("secret-key")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name   string
		header string
		query  string
	}{
		{
			name:   "wrong header",
			header: "wrong-key",
		},
		{
			name: "empty header",
		},
		{
			name:  "wrong query param",
			query: "wrong-key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.header != "" {
				req.Header.Set("X-API-Key", tt.header)
			}
			if tt.query != "" {
				req.URL.RawQuery = "api_key=" + tt.query
			}
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Errorf("Expected status 401, got %d", w.Code)
			}
		})
	}
}

func TestAPIKeyQueryTakesPrecedence(t *testing.T) {
	handler := APIKey("secret-key")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/?api_key=wrong-key", nil)
	req.Header.Set("X-API-Key", "secret-key")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Query param should be checked first, so wrong-key fails
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 when query param is wrong, got %d", w.Code)
	}
}

func TestRateLimiterBasic(t *testing.T) {
	handler := RateLimiter(2, time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	ip := "192.168.1.1"
	successCount := 0

	// First 2 requests should succeed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = ip
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code == http.StatusOK {
			successCount++
		}
	}

	if successCount != 2 {
		t.Errorf("Expected 2 successful requests, got %d", successCount)
	}

	// Third request should be rate limited
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = ip
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", w.Code)
	}
}

func TestRateLimiterDifferentIPs(t *testing.T) {
	handler := RateLimiter(1, time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Two different IPs should each get their allowance
	ips := []string{"192.168.1.1", "192.168.1.2"}

	for _, ip := range ips {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = ip
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("IP %s: Expected status 200, got %d", ip, w.Code)
		}
	}
}

func TestRateLimiterTokenRefill(t *testing.T) {
	handler := RateLimiter(1, 100*time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	ip := "192.168.1.1"

	// First request
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = ip
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("First request should succeed, got %d", w.Code)
	}

	// Immediate second request should be rate limited
	req = httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = ip
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Second immediate request should be rate limited, got %d", w.Code)
	}

	// Wait for token refill
	time.Sleep(150 * time.Millisecond)

	// Third request after refill should succeed
	req = httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = ip
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Third request after refill should succeed, got %d", w.Code)
	}
}

func TestRateLimiterXForwardedFor(t *testing.T) {
	handler := RateLimiter(1, time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Test X-Forwarded-For header
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.1")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Second request from same forwarded IP should be rate limited
	req = httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.1")
	w = httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", w.Code)
	}
}

func TestRateLimiterXRealIP(t *testing.T) {
	handler := RateLimiter(1, time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Test X-Real-IP header
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Real-IP", "10.0.0.2")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestRateLimiterConcurrency(t *testing.T) {
	// This test uses -race to detect data races
	handler := RateLimiter(100, time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	const numGoroutines = 50
	const requestsPerGoroutine = 20

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			ip := "192.168.1." + string(rune('1'+id%10))

			for j := 0; j < requestsPerGoroutine; j++ {
				req := httptest.NewRequest("GET", "/", nil)
				req.RemoteAddr = ip
				w := httptest.NewRecorder()

				handler.ServeHTTP(w, req)
			}
		}(i)
	}

	wg.Wait()
	// If we get here without race detector complaining, the test passes
}

func TestRateLimiterCleanup(t *testing.T) {
	handler := RateLimiter(1, 50*time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Create a visitor
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.100"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Wait for cleanup (5x window + cleanup interval)
	// Cleanup runs every minute by default, but we need to wait for our short window
	time.Sleep(300 * time.Millisecond)

	// Same IP should be allowed again after cleanup
	req = httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.100"
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 after cleanup, got %d", w.Code)
	}
}

func TestRecoveryMiddleware(t *testing.T) {
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	handler := Recovery(panicHandler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	// Should not panic
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}

	// Check response contains error
	if w.Body.String() == "" {
		t.Error("Expected non-empty response body")
	}
}

func TestRecoveryMiddlewareNoPanic(t *testing.T) {
	handler := Recovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestLoggerMiddleware(t *testing.T) {
	handler := Logger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test/path", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	// Logger just logs, we can't easily test output without capturing log
}

func TestCORSMiddleware(t *testing.T) {
	origins := []string{"https://example.com", "https://test.com"}
	handler := CORS(origins)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name         string
		origin       string
		expectHeader bool
	}{
		{
			name:         "allowed origin",
			origin:       "https://example.com",
			expectHeader: true,
		},
		{
			name:         "not allowed origin",
			origin:       "https://evil.com",
			expectHeader: false,
		},
		{
			name:         "no origin",
			origin:       "",
			expectHeader: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if tt.expectHeader {
				if origin := w.Header().Get("Access-Control-Allow-Origin"); origin != tt.origin {
					t.Errorf("Expected origin %s, got %s", tt.origin, origin)
				}
			} else {
				if origin := w.Header().Get("Access-Control-Allow-Origin"); origin != "" {
					t.Errorf("Expected no origin header, got %s", origin)
				}
			}
		})
	}
}

func TestCORSMiddlewareWildcard(t *testing.T) {
	handler := CORS([]string{"*"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://any-origin.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Wildcard should return "*" as the allowed origin (CORS spec behavior)
	if origin := w.Header().Get("Access-Control-Allow-Origin"); origin != "*" {
		t.Errorf("Expected origin '*', got %s", origin)
	}
}

func TestCORSPreflight(t *testing.T) {
	handler := CORS([]string{"*"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("OPTIONS", "/", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", w.Code)
	}

	// Check CORS headers
	if w.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("Expected Methods header")
	}
	if w.Header().Get("Access-Control-Allow-Headers") == "" {
		t.Error("Expected Headers header")
	}
	if w.Header().Get("Access-Control-Max-Age") == "" {
		t.Error("Expected Max-Age header")
	}
}

func TestCORSEmptyOrigins(t *testing.T) {
	// Empty origins slice should allow all
	handler := CORS([]string{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://any-origin.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Empty slice should allow any origin (nil check)
	if origin := w.Header().Get("Access-Control-Allow-Origin"); origin != "https://any-origin.com" {
		t.Errorf("Expected origin to be reflected with empty slice, got %s", origin)
	}
}

func TestMiddlewareChaining(t *testing.T) {
	// Test that multiple middlewares can be chained
	baseHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	handler := Recovery(
		Logger(
			CORS([]string{"*"})(baseHandler),
		),
	)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	if w.Body.String() != "success" {
		t.Errorf("Expected body 'success', got %s", w.Body.String())
	}
}

// BenchmarkRateLimiter benchmarks the rate limiter performance.
func BenchmarkRateLimiter(b *testing.B) {
	handler := RateLimiter(1000, time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ip := 1
		for pb.Next() {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = "192.168.1." + string(rune('0'+ip%10))
			ip++
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
		}
	})
}
