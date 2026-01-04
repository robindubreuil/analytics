// Package api provides HTTP middleware for the analytics service.
package api

import (
	"log"
	"net/http"
	"sync"
	"time"
)

// APIKey creates middleware that validates API keys for write operations.
// If no key is configured, the middleware allows all requests.
func APIKey(apiKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if apiKey == "" {
			// No authentication configured
			return next
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Query param takes precedence over header (allows override)
			key := r.URL.Query().Get("api_key")
			if key == "" {
				key = r.Header.Get("X-API-Key")
			}

			if key != apiKey {
				http.Error(w, `{"error":"unauthorized","message":"Invalid or missing API key"}`, http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RateLimiter creates a simple rate limiter using token bucket algorithm.
// It limits requests per IP address.
// The cleanup goroutine stops when the context is cancelled.
func RateLimiter(requests int, window time.Duration) func(http.Handler) http.Handler {
	limiter := &ipRateLimiter{
		visitors: make(map[string]*visitor),
		mu:       sync.RWMutex{},
		rate:     requests,
		window:   window,
		stop:     make(chan struct{}),
	}

	// Cleanup old visitors every minute
	go limiter.cleanupVisitors(time.Minute)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get client IP
			ip := r.RemoteAddr
			if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
				// Take first IP from X-Forwarded-For
				ip = fwd
			} else if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
				ip = realIP
			}

			if !limiter.Allow(ip) {
				log.Printf("[RateLimit] Blocked request from %s", ip)
				http.Error(w, `{"error":"rate_limit_exceeded","message":"Too many requests"}`, http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// visitor tracks request count for a single IP.
type visitor struct {
	tokens    int
	lastSeen  time.Time
	mu        sync.Mutex
}

// ipRateLimiter tracks visitors and their request rates.
type ipRateLimiter struct {
	visitors map[string]*visitor
	mu       sync.RWMutex
	rate     int
	window   time.Duration
	stop     chan struct{}
}

// Allow checks if a request from the given IP should be allowed.
func (rl *ipRateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	v, exists := rl.visitors[ip]
	if !exists {
		v = &visitor{tokens: rl.rate - 1, lastSeen: time.Now()}
		rl.visitors[ip] = v
		rl.mu.Unlock()
		return true
	}
	// Keep the map locked while we lock the visitor to prevent deletion
	v.mu.Lock()
	rl.mu.Unlock()

	defer v.mu.Unlock()

	// Refill tokens based on time passed
	now := time.Now()
	elapsed := now.Sub(v.lastSeen)
	if elapsed >= rl.window {
		v.tokens = rl.rate - 1
		v.lastSeen = now
		return true
	}

	// Check if we have tokens available
	if v.tokens > 0 {
		v.tokens--
		v.lastSeen = now
		return true
	}

	return false
}

// cleanupVisitors removes stale visitor entries.
func (rl *ipRateLimiter) cleanupVisitors(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-rl.stop:
			return
		case <-ticker.C:
			rl.mu.Lock()
			now := time.Now()
			for ip, v := range rl.visitors {
				v.mu.Lock()
				if now.Sub(v.lastSeen) > rl.window*5 {
					delete(rl.visitors, ip)
				}
				v.mu.Unlock()
			}
			rl.mu.Unlock()
		}
	}
}

// Stop stops the cleanup goroutine.
func (rl *ipRateLimiter) Stop() {
	close(rl.stop)
}

// Recovery creates middleware that recovers from panics.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rvr := recover(); rvr != nil {
				log.Printf("[Panic] Recovered: %v", rvr)
				http.Error(w, `{"error":"internal_error","message":"Internal server error"}`, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// Logger creates middleware that logs HTTP requests.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("[HTTP] %s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

// CORS creates middleware that adds CORS headers.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	originMap := make(map[string]bool)
	hasWildcard := false
	for _, o := range allowedOrigins {
		originMap[o] = true
		if o == "*" {
			hasWildcard = true
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Set CORS headers for OPTIONS preflight requests
			if r.Method == "OPTIONS" {
				if origin != "" {
					if hasWildcard || originMap[origin] || len(allowedOrigins) == 0 {
						if hasWildcard {
							w.Header().Set("Access-Control-Allow-Origin", "*")
						} else {
							w.Header().Set("Access-Control-Allow-Origin", origin)
						}
						w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
						w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key")
						w.Header().Set("Access-Control-Max-Age", "86400")
					}
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}

			// Set CORS headers for regular requests
			if origin != "" {
				if hasWildcard {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				} else if originMap[origin] || len(allowedOrigins) == 0 {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key")
					w.Header().Set("Access-Control-Max-Age", "86400")
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
