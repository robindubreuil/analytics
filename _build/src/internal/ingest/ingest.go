// Package ingest handles event ingestion from the frontend analytics.
package ingest

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/dubreuilpro/analytics/internal/db"
)

// Global semaphore to limit concurrent database writes.
// SQLite handles concurrent writes poorly, so we limit to 3 concurrent writes.
var writeSemaphore = make(chan struct{}, 3)

// ClientEvent represents the event structure sent by the frontend.
type ClientEvent struct {
	SessionID      string            `json:"sessionId"`
	Type           string            `json:"type"`            // "pageview" or "event"
	Event          string            `json:"event,omitempty"` // event name for type="event"
	URL            string            `json:"url"`
	Referrer       string            `json:"referrer"`
	Title          string            `json:"title,omitempty"`
	UserAgent      string            `json:"userAgent"`
	Timestamp      int64             `json:"timestamp"`
	Data           ClientEventData   `json:"data"`
}

// ClientEventData holds the data object from client events.
type ClientEventData struct {
	ScreenWidth    int `json:"screenWidth"`
	ScreenHeight   int `json:"screenHeight"`
	ViewportWidth  int `json:"viewportWidth"`
	ViewportHeight int `json:"viewportHeight"`
	ScrollDepth    int `json:"scrollDepth"`
	EngagementTime int `json:"engagementTime"`
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// Handler handles analytics event ingestion.
type Handler struct {
	db *sql.DB
}

// New creates a new ingest handler.
func New(database *sql.DB) *Handler {
	return &Handler{db: database}
}

// ServeHTTP implements the http.Handler interface.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.respondError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only POST is allowed")
		return
	}

	// Read body with size limit (max 1MB)
	limitedReader := io.LimitReader(r.Body, 1<<20)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "read_error", "Failed to read request body")
		return
	}
	defer r.Body.Close()

	if len(body) == 0 {
		h.respondError(w, http.StatusBadRequest, "empty_body", "Request body is empty")
		return
	}

	// Parse JSON array
	var clientEvents []ClientEvent
	if err := json.Unmarshal(body, &clientEvents); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_json", "Failed to parse JSON")
		return
	}

	if len(clientEvents) == 0 {
		h.respondError(w, http.StatusBadRequest, "no_events", "No events in request")
		return
	}

	// Validate and convert events
	events, validationErr := h.validateAndConvert(clientEvents)
	if validationErr != nil {
		h.respondError(w, http.StatusBadRequest, "validation_error", validationErr.Error())
		return
	}

	// Acquire semaphore to limit concurrent database writes
	writeSemaphore <- struct{}{}
	defer func() { <-writeSemaphore }()

	// Store events
	count, err := db.StoreEvents(h.db, events)
	if err != nil {
		log.Printf("[Ingest] Failed to store events: %v", err)
		h.respondError(w, http.StatusInternalServerError, "storage_error", "Failed to store events")
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"count":   count,
	})
}

// validateAndConvert validates client events and converts them to db.Events.
func (h *Handler) validateAndConvert(clientEvents []ClientEvent) ([]db.Event, error) {
	events := make([]db.Event, 0, len(clientEvents))

	var errs []string
	for i, e := range clientEvents {
		// Validate required fields
		if e.SessionID == "" {
			errs = append(errs, fmt.Sprintf("event %d: missing sessionId", i))
			continue
		}
		if e.Type == "" {
			errs = append(errs, fmt.Sprintf("event %d: missing type", i))
			continue
		}
		if e.URL == "" {
			errs = append(errs, fmt.Sprintf("event %d: missing url", i))
			continue
		}
		if e.Timestamp == 0 {
			errs = append(errs, fmt.Sprintf("event %d: missing timestamp", i))
			continue
		}

		// Validate type
		if e.Type != "pageview" && e.Type != "event" {
			errs = append(errs, fmt.Sprintf("event %d: invalid type '%s'", i, e.Type))
			continue
		}

		// Validate event name for custom events
		eventName := ""
		if e.Type == "event" {
			if e.Event == "" {
				errs = append(errs, fmt.Sprintf("event %d: custom event missing event name", i))
				continue
			}
			eventName = sanitizeEventName(e.Event)
		}

		// Sanitize URL (prevent excessively long URLs)
		if len(e.URL) > 2048 {
			e.URL = e.URL[:2048]
		}

		// Sanitize referrer
		if len(e.Referrer) > 2048 {
			e.Referrer = e.Referrer[:2048]
		}

		// Sanitize user agent
		if len(e.UserAgent) > 500 {
			e.UserAgent = e.UserAgent[:500]
		}

		events = append(events, db.Event{
			SessionID:      e.SessionID,
			Type:           e.Type,
			EventName:      eventName,
			URL:            e.URL,
			Referrer:       e.Referrer,
			Title:          e.Title,
			UserAgent:      e.UserAgent,
			Timestamp:      e.Timestamp,
			ScreenWidth:    e.Data.ScreenWidth,
			ScreenHeight:   e.Data.ScreenHeight,
			ViewportWidth:  e.Data.ViewportWidth,
			ViewportHeight: e.Data.ViewportHeight,
			ScrollDepth:    e.Data.ScrollDepth,
			EngagementTime: e.Data.EngagementTime,
		})
	}

	if len(errs) > 0 {
		return nil, errors.New(strings.Join(errs, "; "))
	}

	return events, nil
}

// sanitizeEventName removes potentially dangerous characters from event names.
func sanitizeEventName(name string) string {
	// Allow alphanumeric, underscore, dash, and dot
	var result strings.Builder
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '_' || c == '-' || c == '.' {
			result.WriteRune(c)
		} else {
			result.WriteRune('_')
		}
	}
	return result.String()
}

// respondJSON writes a JSON response.
func (h *Handler) respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("[Ingest] Failed to encode response: %v", err)
	}
}

// respondError writes an error response.
func (h *Handler) respondError(w http.ResponseWriter, status int, errCode, errMsg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Error:   errCode,
		Message: errMsg,
	})
}

// SuccessResponse is returned on successful event ingestion.
type SuccessResponse struct {
	Success bool `json:"success"`
	Count   int  `json:"count"`
}
