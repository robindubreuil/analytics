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

// RespondJSON writes a JSON response.
func RespondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("[HTTP] Failed to encode response: %v", err)
	}
}

// RespondError writes an error response.
func RespondError(w http.ResponseWriter, status int, errCode, errMsg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(map[string]any{
		"error":   errCode,
		"message": errMsg,
	})
}

// DashboardHandler handles dashboard API requests.
type DashboardHandler struct {
	db           *sql.DB
	dashboardKey string
}

// New creates a new dashboard handler.
func New(database *sql.DB, dashboardKey string) *DashboardHandler {
	return &DashboardHandler{db: database, dashboardKey: dashboardKey}
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
		if h.dashboardKey == "" {
			next(w, r)
			return
		}

		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			apiKey = r.URL.Query().Get("api_key")
		}

		if !constantTimeEqual(apiKey, h.dashboardKey) {
			RespondError(w, http.StatusUnauthorized, "unauthorized", "Invalid API key")
			return
		}

		next(w, r)
	}
}

// summary returns overall statistics for a date range.
func (h *DashboardHandler) summary(w http.ResponseWriter, r *http.Request) {
	start, end, err := parseDateRange(r)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "invalid_date", err.Error())
		return
	}

	summary, err := db.GetSummary(h.db, start, end)
	if err != nil {
		log.Printf("[Dashboard] Summary query error: %v", err)
		RespondError(w, http.StatusInternalServerError, "query_error", "Failed to retrieve summary")
		return
	}

	topPages, err := db.GetTopPages(h.db, start, end, 10)
	if err != nil {
		log.Printf("[Dashboard] Top pages query error: %v", err)
		RespondError(w, http.StatusInternalServerError, "query_error", "Failed to retrieve top pages")
		return
	}

	topEvents, err := db.GetTopEvents(h.db, start, end, 10)
	if err != nil {
		log.Printf("[Dashboard] Top events query error: %v", err)
		RespondError(w, http.StatusInternalServerError, "query_error", "Failed to retrieve top events")
		return
	}

	bounceRate := float64(0)
	if summary.Sessions > 0 {
		bounceRate = float64(summary.BouncedSessions) / float64(summary.Sessions)
	}

	RespondJSON(w, http.StatusOK, map[string]any{
		"period": map[string]string{
			"start": start,
			"end":   end,
		},
		"pageviews":      summary.Pageviews,
		"sessions":       summary.Sessions,
		"uniqueVisitors": summary.UniqueVisitors,
		"avgEngagement":  summary.AvgEngagement,
		"bounceRate":     bounceRate,
		"topPages":       topPages,
		"topEvents":      topEvents,
	})
}

// timeseries returns daily statistics over a date range.
func (h *DashboardHandler) timeseries(w http.ResponseWriter, r *http.Request) {
	start, end, err := parseDateRange(r)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "invalid_date", err.Error())
		return
	}

	stats, err := db.GetTimeSeries(h.db, start, end)
	if err != nil {
		log.Printf("[Dashboard] Timeseries query error: %v", err)
		RespondError(w, http.StatusInternalServerError, "query_error", "Failed to retrieve timeseries")
		return
	}

	RespondJSON(w, http.StatusOK, map[string]any{
		"period": map[string]string{
			"start": start,
			"end":   end,
		},
		"data": stats,
	})
}

// pages returns page performance statistics.
func (h *DashboardHandler) pages(w http.ResponseWriter, r *http.Request) {
	start, end, err := parseDateRange(r)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "invalid_date", err.Error())
		return
	}

	limit := parseLimit(r, 50)

	pages, err := db.GetTopPages(h.db, start, end, limit)
	if err != nil {
		log.Printf("[Dashboard] Pages query error: %v", err)
		RespondError(w, http.StatusInternalServerError, "query_error", "Failed to retrieve pages")
		return
	}

	RespondJSON(w, http.StatusOK, map[string]any{
		"period": map[string]string{
			"start": start,
			"end":   end,
		},
		"pages": pages,
	})
}

// events returns custom event statistics.
func (h *DashboardHandler) events(w http.ResponseWriter, r *http.Request) {
	start, end, err := parseDateRange(r)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "invalid_date", err.Error())
		return
	}

	limit := parseLimit(r, 50)

	events, err := db.GetTopEvents(h.db, start, end, limit)
	if err != nil {
		log.Printf("[Dashboard] Events query error: %v", err)
		RespondError(w, http.StatusInternalServerError, "query_error", "Failed to retrieve events")
		return
	}

	RespondJSON(w, http.StatusOK, map[string]any{
		"period": map[string]string{
			"start": start,
			"end":   end,
		},
		"events": events,
	})
}

// sessions returns session data.
func (h *DashboardHandler) sessions(w http.ResponseWriter, r *http.Request) {
	startTime, endTime, err := parseTimeRange(r)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "invalid_date", err.Error())
		return
	}

	limit := parseLimit(r, 100)
	offset := parseOffset(r)

	sessions, err := db.GetSessions(h.db, startTime, endTime, limit, offset)
	if err != nil {
		log.Printf("[Dashboard] Sessions query error: %v", err)
		RespondError(w, http.StatusInternalServerError, "query_error", "Failed to retrieve sessions")
		return
	}

	RespondJSON(w, http.StatusOK, map[string]any{
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
	if err := h.db.Ping(); err != nil {
		RespondJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status": "unhealthy",
			"error":  "database unavailable",
		})
		return
	}

	RespondJSON(w, http.StatusOK, map[string]any{
		"status": "healthy",
	})
}

// parseDateRange parses start/end date query params (YYYY-MM-DD format).
func parseDateRange(r *http.Request) (start, end string, err error) {
	start = r.URL.Query().Get("start")
	end = r.URL.Query().Get("end")

	if start == "" {
		start = time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	}
	if end == "" {
		end = time.Now().Format("2006-01-02")
	}

	startTime, err := time.Parse("2006-01-02", start)
	if err != nil {
		return "", "", fmt.Errorf("invalid start date format, expected YYYY-MM-DD")
	}
	endTime, err := time.Parse("2006-01-02", end)
	if err != nil {
		return "", "", fmt.Errorf("invalid end date format, expected YYYY-MM-DD")
	}

	if startTime.Year() < 1900 || startTime.Year() > 2200 {
		return "", "", fmt.Errorf("start date out of valid range (1900-2200)")
	}
	if endTime.Year() < 1900 || endTime.Year() > 2200 {
		return "", "", fmt.Errorf("end date out of valid range (1900-2200)")
	}

	if startTime.After(endTime) {
		return "", "", fmt.Errorf("start date must be before or equal to end date")
	}

	if endTime.Sub(startTime) > 5*365*24*time.Hour {
		return "", "", fmt.Errorf("date range too large, maximum 5 years")
	}

	return start, end, nil
}

// parseTimeRange parses start/end timestamp query params.
func parseTimeRange(r *http.Request) (start, end int64, err error) {
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")

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

	const minTimestamp int64 = -2208988800000
	const maxTimestamp int64 = 7258118400000

	if start < minTimestamp || start > maxTimestamp {
		return 0, 0, fmt.Errorf("start timestamp out of valid range")
	}
	if end < minTimestamp || end > maxTimestamp {
		return 0, 0, fmt.Errorf("end timestamp out of valid range")
	}

	if start > end {
		return 0, 0, fmt.Errorf("start timestamp must be before or equal to end timestamp")
	}

	const maxRange = 5 * 365 * 24 * time.Hour
	if time.UnixMilli(end).Sub(time.UnixMilli(start)) > maxRange {
		return 0, 0, fmt.Errorf("time range too large, maximum 5 years")
	}

	return start, end, nil
}

// parseLimit parses limit from query params.
func parseLimit(r *http.Request, defaultLimit int) int {
	limitStr := r.URL.Query().Get("limit")
	if limitStr == "" {
		return defaultLimit
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		return defaultLimit
	}
	if limit > 1000 {
		return 1000
	}
	return limit
}

// parseOffset parses offset from query params.
func parseOffset(r *http.Request) int {
	offsetStr := r.URL.Query().Get("offset")
	if offsetStr == "" {
		return 0
	}
	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		return 0
	}
	if offset > 10000 {
		return 10000
	}
	return offset
}
