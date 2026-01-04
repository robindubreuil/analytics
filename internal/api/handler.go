// Package api provides HTTP handlers for the analytics dashboard API.
package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/dubreuilpro/analytics/internal/db"
)

// DashboardHandler handles dashboard API requests.
type DashboardHandler struct {
	db *sql.DB
}

// New creates a new dashboard handler.
func New(database *sql.DB) *DashboardHandler {
	return &DashboardHandler{db: database}
}

// RegisterRoutes registers all dashboard routes with the given mux.
func (h *DashboardHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/dashboard/summary", h.withAuth(h.summary))
	mux.HandleFunc("GET /api/dashboard/timeseries", h.withAuth(h.timeseries))
	mux.HandleFunc("GET /api/dashboard/pages", h.withAuth(h.pages))
	mux.HandleFunc("GET /api/dashboard/events", h.withAuth(h.events))
	mux.HandleFunc("GET /api/dashboard/sessions", h.withAuth(h.sessions))
	mux.HandleFunc("GET /api/dashboard/health", h.health)
}

// withAuth wraps a handler with optional API key authentication.
func (h *DashboardHandler) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check for API key if configured
		// Query param takes precedence over header (allows override)
		apiKey := r.URL.Query().Get("api_key")
		if apiKey == "" {
			apiKey = r.Header.Get("X-API-Key")
		}

		// If no key is configured, allow access (development mode)
		if dashboardKey == "" {
			next(w, r)
			return
		}

		// If key is configured, validate it
		if apiKey != dashboardKey {
			h.respondError(w, http.StatusUnauthorized, "unauthorized", "Invalid API key")
			return
		}

		next(w, r)
	}
}

// dashboardKey is the configured API key for dashboard access.
// This is set by SetDashboardKey.
var dashboardKey = ""

// SetDashboardKey sets the dashboard API key.
func SetDashboardKey(key string) {
	dashboardKey = key
}

// summary returns overall statistics for a date range.
func (h *DashboardHandler) summary(w http.ResponseWriter, r *http.Request) {
	start, end, err := h.parseDateRange(r)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_date", err.Error())
		return
	}

	summary, err := db.GetSummary(h.db, start, end)
	if err != nil {
		log.Printf("[Dashboard] Summary query error: %v", err)
		h.respondError(w, http.StatusInternalServerError, "query_error", "Failed to retrieve summary")
		return
	}

	// Get top pages
	topPages, err := db.GetTopPages(h.db, start, end, 10)
	if err != nil {
		log.Printf("[Dashboard] Top pages query error: %v", err)
	}

	// Get top events
	topEvents, err := db.GetTopEvents(h.db, start, end, 10)
	if err != nil {
		log.Printf("[Dashboard] Top events query error: %v", err)
	}

	h.respondJSON(w, http.StatusOK, map[string]any{
		"period": map[string]string{
			"start": start,
			"end":   end,
		},
		"pageviews":      summary.Pageviews,
		"sessions":       summary.Sessions,
		"uniqueVisitors": summary.UniqueVisitors,
		"avgEngagement":  summary.AvgEngagement,
		"bounceRate":     h.calculateBounceRate(summary),
		"topPages":       topPages,
		"topEvents":      topEvents,
	})
}

// timeseries returns daily statistics over a date range.
func (h *DashboardHandler) timeseries(w http.ResponseWriter, r *http.Request) {
	start, end, err := h.parseDateRange(r)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_date", err.Error())
		return
	}

	stats, err := db.GetTimeSeries(h.db, start, end)
	if err != nil {
		log.Printf("[Dashboard] Timeseries query error: %v", err)
		h.respondError(w, http.StatusInternalServerError, "query_error", "Failed to retrieve timeseries")
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]any{
		"period": map[string]string{
			"start": start,
			"end":   end,
		},
		"data": stats,
	})
}

// pages returns page performance statistics.
func (h *DashboardHandler) pages(w http.ResponseWriter, r *http.Request) {
	start, end, err := h.parseDateRange(r)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_date", err.Error())
		return
	}

	limit := h.parseLimit(r, 50)

	pages, err := db.GetTopPages(h.db, start, end, limit)
	if err != nil {
		log.Printf("[Dashboard] Pages query error: %v", err)
		h.respondError(w, http.StatusInternalServerError, "query_error", "Failed to retrieve pages")
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]any{
		"period": map[string]string{
			"start": start,
			"end":   end,
		},
		"pages": pages,
	})
}

// events returns custom event statistics.
func (h *DashboardHandler) events(w http.ResponseWriter, r *http.Request) {
	start, end, err := h.parseDateRange(r)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_date", err.Error())
		return
	}

	limit := h.parseLimit(r, 50)

	events, err := db.GetTopEvents(h.db, start, end, limit)
	if err != nil {
		log.Printf("[Dashboard] Events query error: %v", err)
		h.respondError(w, http.StatusInternalServerError, "query_error", "Failed to retrieve events")
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]any{
		"period": map[string]string{
			"start": start,
			"end":   end,
		},
		"events": events,
	})
}

// sessions returns session data.
func (h *DashboardHandler) sessions(w http.ResponseWriter, r *http.Request) {
	startTime, endTime, err := h.parseTimeRange(r)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_date", err.Error())
		return
	}

	limit := h.parseLimit(r, 100)
	offset := h.parseOffset(r)

	sessions, err := db.GetSessions(h.db, startTime, endTime, limit, offset)
	if err != nil {
		log.Printf("[Dashboard] Sessions query error: %v", err)
		h.respondError(w, http.StatusInternalServerError, "query_error", "Failed to retrieve sessions")
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]any{
		"period": map[string]int64{
			"start": startTime,
			"end":   endTime,
		},
		"sessions": sessions,
		"limit":    limit,
		"offset":   offset,
	})
}

// health returns the health status of the service.
func (h *DashboardHandler) health(w http.ResponseWriter, r *http.Request) {
	// Check database connection
	if err := h.db.Ping(); err != nil {
		h.respondJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status": "unhealthy",
			"error":  "database unavailable",
		})
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]any{
		"status": "healthy",
	})
}

// Helper: parse date range from query params (YYYY-MM-DD format).
func (h *DashboardHandler) parseDateRange(r *http.Request) (start, end string, err error) {
	start = r.URL.Query().Get("start")
	end = r.URL.Query().Get("end")

	// Default to last 30 days if not specified
	if start == "" {
		start = time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	}
	if end == "" {
		end = time.Now().Format("2006-01-02")
	}

	// Validate date format and logical validity
	startTime, err := time.Parse("2006-01-02", start)
	if err != nil {
		return "", "", fmt.Errorf("invalid start date format, expected YYYY-MM-DD")
	}
	endTime, err := time.Parse("2006-01-02", end)
	if err != nil {
		return "", "", fmt.Errorf("invalid end date format, expected YYYY-MM-DD")
	}

	// Check if dates are within reasonable bounds (1900-2200)
	const minYear = 1900
	const maxYear = 2200
	if startTime.Year() < minYear || startTime.Year() > maxYear {
		return "", "", fmt.Errorf("start date out of valid range (1900-2200)")
	}
	if endTime.Year() < minYear || endTime.Year() > maxYear {
		return "", "", fmt.Errorf("end date out of valid range (1900-2200)")
	}

	// Validate that start is before or equal to end
	if startTime.After(endTime) {
		return "", "", fmt.Errorf("start date must be before or equal to end date")
	}

	// Limit date range to maximum 5 years
	if endTime.Sub(startTime) > 5*365*24*time.Hour {
		return "", "", fmt.Errorf("date range too large, maximum 5 years")
	}

	return start, end, nil
}

// Helper: parse time range from query params (unix timestamps).
func (h *DashboardHandler) parseTimeRange(r *http.Request) (start, end int64, err error) {
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")

	// Default to last 7 days if not specified
	if startStr == "" {
		start = time.Now().AddDate(0, 0, -7).UnixMilli()
	} else {
		start, err = strconv.ParseInt(startStr, 10, 64)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid start timestamp")
		}
	}

	if endStr == "" {
		end = time.Now().UnixMilli()
	} else {
		end, err = strconv.ParseInt(endStr, 10, 64)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid end timestamp")
		}
	}

	// Validate timestamp bounds (year 1900 to 2200)
	const minTimestamp int64 = -2208988800000 // 1900-01-01 in milliseconds
	const maxTimestamp int64 = 7258118400000  // 2200-01-01 in milliseconds

	if start < minTimestamp || start > maxTimestamp {
		return 0, 0, fmt.Errorf("start timestamp out of valid range")
	}
	if end < minTimestamp || end > maxTimestamp {
		return 0, 0, fmt.Errorf("end timestamp out of valid range")
	}

	// Validate that start is before or equal to end
	if start > end {
		return 0, 0, fmt.Errorf("start timestamp must be before or equal to end timestamp")
	}

	// Limit time range to maximum 5 years
	const maxRange = 5 * 365 * 24 * time.Hour
	if time.UnixMilli(end).Sub(time.UnixMilli(start)) > maxRange {
		return 0, 0, fmt.Errorf("time range too large, maximum 5 years")
	}

	return start, end, nil
}

// Helper: parse limit from query params.
func (h *DashboardHandler) parseLimit(r *http.Request, defaultLimit int) int {
	limitStr := r.URL.Query().Get("limit")
	if limitStr == "" {
		return defaultLimit
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		return defaultLimit
	}
	if limit > 1000 {
		return 1000 // Max limit
	}
	return limit
}

// Helper: parse offset from query params.
func (h *DashboardHandler) parseOffset(r *http.Request) int {
	offsetStr := r.URL.Query().Get("offset")
	if offsetStr == "" {
		return 0
	}
	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		return 0
	}
	return offset
}

// Helper: calculate bounce rate.
func (h *DashboardHandler) calculateBounceRate(stats *db.DailyStats) float64 {
	if stats.Sessions == 0 {
		return 0
	}
	return float64(stats.BouncedSessions) / float64(stats.Sessions)
}

// respondJSON writes a JSON response.
func (h *DashboardHandler) respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("[Dashboard] Failed to encode response: %v", err)
	}
}

// respondError writes an error response.
func (h *DashboardHandler) respondError(w http.ResponseWriter, status int, errCode, errMsg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(map[string]any{
		"error":   errCode,
		"message": errMsg,
	})
}
