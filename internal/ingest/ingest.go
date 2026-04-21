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
	"time"

	"github.com/robindubreuil/analytics/internal/api"
	"github.com/robindubreuil/analytics/internal/db"
)

// ClientEvent represents the event structure sent by the frontend.
type ClientEvent struct {
	SessionID string          `json:"sessionId"`
	Type      string          `json:"type"`
	Event     string          `json:"event,omitempty"`
	URL       string          `json:"url"`
	Referrer  string          `json:"referrer"`
	Title     string          `json:"title,omitempty"`
	UserAgent string          `json:"userAgent"`
	Timestamp int64           `json:"timestamp"`
	Data      ClientEventData `json:"data"`
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

// Handler handles analytics event ingestion.
type Handler struct {
	db       *sql.DB
	sem      chan struct{}
	maxBatch int
}

// New creates a new ingest handler.
func New(database *sql.DB, maxConcurrentWrites int) *Handler {
	if maxConcurrentWrites <= 0 {
		maxConcurrentWrites = 3
	}
	return &Handler{
		db:       database,
		sem:      make(chan struct{}, maxConcurrentWrites),
		maxBatch: 100,
	}
}

// ServeHTTP implements the http.Handler interface.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.RespondError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only POST is allowed")
		return
	}

	limitedReader := io.LimitReader(r.Body, 1<<20)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		api.RespondError(w, http.StatusBadRequest, "read_error", "Failed to read request body")
		return
	}
	defer r.Body.Close()

	if len(body) == 0 {
		api.RespondError(w, http.StatusBadRequest, "empty_body", "Request body is empty")
		return
	}

	var clientEvents []ClientEvent
	if err := json.Unmarshal(body, &clientEvents); err != nil {
		api.RespondError(w, http.StatusBadRequest, "invalid_json", "Failed to parse JSON")
		return
	}

	if len(clientEvents) == 0 {
		api.RespondError(w, http.StatusBadRequest, "no_events", "No events in request")
		return
	}

	if len(clientEvents) > h.maxBatch {
		api.RespondError(w, http.StatusBadRequest, "too_many_events", fmt.Sprintf("Batch size exceeds maximum of %d", h.maxBatch))
		return
	}

	events, validationErr := validateAndConvert(clientEvents)
	if validationErr != nil {
		api.RespondError(w, http.StatusBadRequest, "validation_error", validationErr.Error())
		return
	}

	h.sem <- struct{}{}
	defer func() { <-h.sem }()

	count, err := db.StoreEvents(h.db, events)
	if err != nil {
		log.Printf("[Ingest] Failed to store events: %v", err)
		api.RespondError(w, http.StatusInternalServerError, "storage_error", "Failed to store events")
		return
	}

	api.RespondJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"count":   count,
	})
}

// validateAndConvert validates client events and converts them to db.Events.
func validateAndConvert(clientEvents []ClientEvent) ([]db.Event, error) {
	events := make([]db.Event, 0, len(clientEvents))

	var errs []string
	for i, e := range clientEvents {
		if e.SessionID == "" {
			errs = append(errs, fmt.Sprintf("event %d: missing sessionId", i))
			continue
		}
		if len(e.SessionID) > 128 {
			errs = append(errs, fmt.Sprintf("event %d: sessionId too long (max 128)", i))
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

		ts := time.UnixMilli(e.Timestamp)
		now := time.Now()
		minTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		if ts.After(now.Add(time.Hour)) || ts.Before(minTime) {
			errs = append(errs, fmt.Sprintf("event %d: timestamp out of valid range", i))
			continue
		}

		if e.Type != "pageview" && e.Type != "event" {
			errs = append(errs, fmt.Sprintf("event %d: invalid type '%s'", i, e.Type))
			continue
		}

		eventName := ""
		if e.Type == "event" {
			if e.Event == "" {
				errs = append(errs, fmt.Sprintf("event %d: custom event missing event name", i))
				continue
			}
			eventName = sanitizeEventName(e.Event)
		}

		if len(e.URL) > 2048 {
			e.URL = e.URL[:2048]
		}
		if len(e.Referrer) > 2048 {
			e.Referrer = e.Referrer[:2048]
		}
		if len(e.UserAgent) > 500 {
			e.UserAgent = e.UserAgent[:500]
		}

		screenW := clampPositive(e.Data.ScreenWidth)
		screenH := clampPositive(e.Data.ScreenHeight)
		viewW := clampPositive(e.Data.ViewportWidth)
		viewH := clampPositive(e.Data.ViewportHeight)
		scrollD := clampPositive(e.Data.ScrollDepth)
		engageT := clampPositive(e.Data.EngagementTime)

		events = append(events, db.Event{
			SessionID:      e.SessionID,
			Type:           e.Type,
			EventName:      eventName,
			URL:            e.URL,
			Referrer:       e.Referrer,
			Title:          e.Title,
			UserAgent:      e.UserAgent,
			Timestamp:      e.Timestamp,
			ScreenWidth:    screenW,
			ScreenHeight:   screenH,
			ViewportWidth:  viewW,
			ViewportHeight: viewH,
			ScrollDepth:    scrollD,
			EngagementTime: engageT,
		})
	}

	if len(errs) > 0 {
		return nil, errors.New(strings.Join(errs, "; "))
	}

	return events, nil
}

func clampPositive(v int) int {
	if v < 0 {
		return 0
	}
	return v
}

func sanitizeEventName(name string) string {
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
