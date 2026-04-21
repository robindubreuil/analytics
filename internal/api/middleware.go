// Package api provides HTTP middleware for the analytics service.
package api

import (
	"crypto/subtle"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// getClientIP extracts the client IP from the request, safely handling
// X-Forwarded-For and X-Real-IP headers to prevent spoofing.
func getClientIP(r *http.Request) string {
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		if ip := parseIP(realIP); ip != "" {
			return ip
		}
	}

	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		ips := strings.Split(fwd, ",")
		for i := len(ips) - 1; i >= 0; i-- {
			if ip := parseIP(strings.TrimSpace(ips[i])); ip != "" {
				return ip
			}
		}
	}

	return parseIP(r.RemoteAddr)
}

// parseIP extracts a bare IP address from a host:port string,
// handling both IPv4 and IPv6 (including bracketed [::1]:8080 format).
func parseIP(host string) string {
	if strings.HasPrefix(host, "[") {
		if idx := strings.Index(host, "]"); idx != -1 {
			return host[1:idx]
		}
	}

	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}

	if net.ParseIP(host) != nil {
		return host
	}

	return host
}

// constantTimeEqual compares two strings in constant time.
func constantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// APIKey creates middleware that validates API keys for write operations.
// If no key is configured, the middleware allows all requests.
// The key is read from the X-API-Key header. The query parameter ?api_key=
// is also accepted as a fallback for sendBeacon (which cannot set headers).
func APIKey(apiKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if apiKey == "" {
			return next
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("X-API-Key")
			if key == "" {
				key = r.URL.Query().Get("api_key")
			}

			if !constantTimeEqual(key, apiKey) {
				http.Error(w, `{"error":"unauthorized","message":"Invalid or missing API key"}`, http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RateLimiter creates a simple rate limiter using token bucket algorithm.
// It limits requests per IP address.
func RateLimiter(requests int, window time.Duration) (func(http.Handler) http.Handler, func()) {
	limiter := &ipRateLimiter{
		visitors: make(map[string]*visitor),
		mu:       sync.RWMutex{},
		rate:     requests,
		window:   window,
		stop:     make(chan struct{}),
	}

	go limiter.cleanupVisitors(time.Minute)

	middleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := getClientIP(r)

			if !limiter.Allow(ip) {
				log.Printf("[RateLimit] Blocked request from %s", ip)
				http.Error(w, `{"error":"rate_limit_exceeded","message":"Too many requests"}`, http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}

	cleanup := func() {
		limiter.Stop()
	}

	return middleware, cleanup
}

type visitor struct {
	tokens   int
	lastSeen time.Time
	mu       sync.Mutex
}

type ipRateLimiter struct {
	visitors map[string]*visitor
	mu       sync.RWMutex
	rate     int
	window   time.Duration
	stop     chan struct{}
}

func (rl *ipRateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	v, exists := rl.visitors[ip]
	if !exists {
		v = &visitor{tokens: rl.rate - 1, lastSeen: time.Now()}
		rl.visitors[ip] = v
		rl.mu.Unlock()
		return true
	}
	v.mu.Lock()
	rl.mu.Unlock()

	defer v.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(v.lastSeen)
	if elapsed >= rl.window {
		v.tokens = rl.rate - 1
		v.lastSeen = now
		return true
	}

	if v.tokens > 0 {
		v.tokens--
		v.lastSeen = now
		return true
	}

	return false
}

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
