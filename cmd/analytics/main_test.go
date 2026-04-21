package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"syscall"
	"testing"
	"time"

	"github.com/robindubreuil/analytics/internal/db"
	"github.com/robindubreuil/analytics/internal/testutil"
)

func TestRetentionJob(t *testing.T) {
	tdb := testutil.NewTestDB(t)

	now := time.Now().UnixMilli()
	oldTime := now - (91 * 24 * 3600 * 1000)

	_, err := tdb.DB.Exec(
		"INSERT INTO events (session_id, type, url, timestamp, created_at) VALUES (?, ?, ?, ?, ?)",
		"old-session", "pageview", "/old-page", oldTime, oldTime,
	)
	if err != nil {
		t.Fatalf("Failed to insert old event: %v", err)
	}

	_, err = tdb.DB.Exec(
		"INSERT INTO events (session_id, type, url, timestamp, created_at) VALUES (?, ?, ?, ?, ?)",
		"new-session", "pageview", "/new-page", now, now,
	)
	if err != nil {
		t.Fatalf("Failed to insert new event: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		retentionJob(ctx, tdb.DB, 90)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("retentionJob did not shut down gracefully")
	}
}

func TestShutdownOnSignal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cancelCalled := false
	cancel := func() { cancelCalled = true }

	cleanupCalled := false
	cleanup := func() { cleanupCalled = true }

	done := make(chan struct{})
	go func() {
		shutdownOnSignal(server.Config, cancel, cleanup)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)

	_ = syscall.Kill(syscall.Getpid(), syscall.SIGTERM)

	select {
	case <-done:
		if !cancelCalled {
			t.Error("expected cancel to be called during shutdown")
		}
		if !cleanupCalled {
			t.Error("expected cleanup func to be called during shutdown")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("shutdownOnSignal did not complete")
	}
}

func TestVersionOutput(t *testing.T) {
	if version == "" {
		t.Log("version variable is empty (expected in test builds)")
	}
	if buildTime == "" {
		t.Log("buildTime variable is empty (expected in test builds)")
	}
}

func TestRetentionJobDeletesOldEvents(t *testing.T) {
	tdb := testutil.NewTestDB(t)

	now := time.Now().UnixMilli()
	oldTime := now - (100 * 24 * 3600 * 1000)

	for i := 0; i < 5; i++ {
		_, err := tdb.DB.Exec(
			"INSERT INTO events (session_id, type, url, timestamp, created_at) VALUES (?, ?, ?, ?, ?)",
			fmt.Sprintf("old-%d", i), "pageview", "/old", oldTime+int64(i)*1000, oldTime+int64(i)*1000,
		)
		if err != nil {
			t.Fatalf("Failed to insert old event: %v", err)
		}
	}

	_, err := tdb.DB.Exec(
		"INSERT INTO events (session_id, type, url, timestamp, created_at) VALUES (?, ?, ?, ?, ?)",
		"new-session", "pageview", "/new", now, now,
	)
	if err != nil {
		t.Fatalf("Failed to insert new event: %v", err)
	}

	deleted, err := db.DeleteOldEvents(tdb.DB, 90)
	if err != nil {
		t.Fatalf("DeleteOldEvents failed: %v", err)
	}
	if deleted != 5 {
		t.Errorf("Expected 5 deleted events, got %d", deleted)
	}

	var count int
	err = tdb.DB.QueryRow("SELECT COUNT(*) FROM events").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count events: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 remaining event, got %d", count)
	}
}
