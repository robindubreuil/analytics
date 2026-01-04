// Package db provides tests for database operations.
package db

import (
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"
)

// TestHelper creates a test database and cleans up after.
type TestHelper struct {
	DB   *sql.DB
	Path string
}

// NewTestHelper creates a new test database with a unique path.
func NewTestHelper(t *testing.T) *TestHelper {
	t.Helper()

	// Use test name + nanoseconds for uniqueness
	unique := time.Now().Format("20060102150405") + fmt.Sprintf("%d", time.Now().Nanosecond())
	path := "/tmp/analytics_test_" + unique + ".db"

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	t.Cleanup(func() {
		db.Close()
		os.Remove(path)
	})

	return &TestHelper{
		DB:   db,
		Path: path,
	}
}

// Close closes the database and removes the file.
func (th *TestHelper) Close(t *testing.T) {
	t.Helper()
	if err := th.DB.Close(); err != nil {
		t.Errorf("Failed to close database: %v", err)
	}
	if err := os.Remove(th.Path); err != nil {
		t.Logf("Warning: Failed to remove test database file: %v", err)
	}
}

func TestOpen(t *testing.T) {
	th := NewTestHelper(t)
	defer th.Close(t)

	// Check that database is open
	if err := th.DB.Ping(); err != nil {
		t.Errorf("Failed to ping database: %v", err)
	}
}

func TestOpenCreatesDirectory(t *testing.T) {
	path := "/tmp/test_analytics_nested/" + time.Now().Format("20060102150405") + ".db"
	defer os.RemoveAll("/tmp/test_analytics_nested")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Failed to open database with nested path: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Errorf("Failed to ping database: %v", err)
	}
}

func TestMigrate(t *testing.T) {
	th := NewTestHelper(t)
	defer th.Close(t)

	// Check that tables were created
	tables := []string{
		"events", "sessions", "daily_stats", "page_stats", "event_stats", "schema_migrations",
	}

	for _, table := range tables {
		var count int
		err := th.DB.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&count)
		if err != nil {
			t.Errorf("Failed to check for table %s: %v", table, err)
		}
		if count != 1 {
			t.Errorf("Table %s was not created", table)
		}
	}

	// Check migration version
	var version int
	err := th.DB.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&version)
	if err != nil {
		t.Errorf("Failed to get migration version: %v", err)
	}
	// Version should be 1 (001_initial.sql)
	if version != 1 {
		t.Errorf("Expected migration version 1, got %d", version)
	}

	// Check migration name was recorded
	var name string
	err = th.DB.QueryRow("SELECT name FROM schema_migrations WHERE version = 1").Scan(&name)
	if err != nil {
		t.Errorf("Failed to get migration name: %v", err)
	}
	if name != "001_initial.sql" {
		t.Errorf("Expected migration name '001_initial.sql', got %s", name)
	}
}

func TestMigrateIdempotent(t *testing.T) {
	// Opening an already migrated database should not fail
	th := NewTestHelper(t)

	// Close and reopen to trigger migration again
	if err := th.DB.Close(); err != nil {
		t.Fatalf("Failed to close database: %v", err)
	}

	db2, err := Open(th.Path)
	if err != nil {
		t.Fatalf("Failed to reopen database: %v", err)
	}
	defer db2.Close()
	th.Close(t)

	// Should still work
	if err := db2.Ping(); err != nil {
		t.Errorf("Failed to ping reopened database: %v", err)
	}
}

func TestOptimize(t *testing.T) {
	th := NewTestHelper(t)
	defer th.Close(t)

	// Check WAL mode
	var journalMode string
	err := th.DB.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
	if err != nil {
		t.Errorf("Failed to check journal mode: %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("Expected WAL mode, got %s", journalMode)
	}

	// Check synchronous mode
	var syncMode string
	err = th.DB.QueryRow("PRAGMA synchronous").Scan(&syncMode)
	if err != nil {
		t.Errorf("Failed to check synchronous mode: %v", err)
	}
	if syncMode != "1" { // 1 = NORMAL
		t.Errorf("Expected synchronous mode 1 (NORMAL), got %s", syncMode)
	}
}

func TestClose(t *testing.T) {
	path := "/tmp/analytics_close_test.db"
	defer os.Remove(path)

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	if err := Close(db); err != nil {
		t.Errorf("Failed to close database: %v", err)
	}

	// Database should be closed now
	if err := db.Ping(); err == nil {
		t.Error("Expected error when pinging closed database")
	}
}

func TestOpenInCurrentDirectory(t *testing.T) {
	// Test opening a database in the current directory (no path separators)
	path := "analytics_test_current_dir.db"
	defer os.Remove(path)

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Failed to open database in current directory: %v", err)
	}
	defer db.Close()

	// Verify database works
	if err := db.Ping(); err != nil {
		t.Errorf("Failed to ping database: %v", err)
	}
}

func TestOpenInTmpDirectory(t *testing.T) {
	// Test opening in /tmp which always exists
	path := "/tmp/analytics_tmp_test.db"
	defer os.Remove(path)

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Failed to open database in /tmp: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Errorf("Failed to ping database: %v", err)
	}
}

func TestNullString(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		{"", nil},
		{"test", "test"},
		{" ", " "},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := nullString(tt.input)
			if result != tt.expected {
				t.Errorf("nullString(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestToUTCDate(t *testing.T) {
	// 2024-01-15 12:30:45 UTC = 1705317045000 ms
	ms := int64(1705317045000)
	result := toUTCDate(ms)
	expected := "2024-01-15"
	if result != expected {
		t.Errorf("toUTCDate(%d) = %s, want %s", ms, result, expected)
	}
}

func TestToUTCDateTimezones(t *testing.T) {
	// Test that timestamps are correctly converted to UTC date
	tests := []struct {
		timestamp int64
		expected  string
	}{
		{0, "1970-01-01"},                              // Unix epoch
		{86400000, "1970-01-02"},                        // 1 day later
		{-86400000, "1969-12-31"},                       // 1 day before
		{1704067200000, "2024-01-01"},                  // 2024-01-01 00:00:00 UTC
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := toUTCDate(tt.timestamp)
			if result != tt.expected {
				t.Errorf("toUTCDate(%d) = %s, want %s", tt.timestamp, result, tt.expected)
			}
		})
	}
}

func TestIsBusyError(t *testing.T) {
	tests := []struct {
		name  string
		err   error
		want  bool
	}{
		{
			name: "database is locked",
			err:  &testError{"database is locked"},
			want: true,
		},
		{
			name: "SQLITE_BUSY",
			err:  &testError{"SQLITE_BUSY"},
			want: true,
		},
		{
			name: "error code 5",
			err:  &testError{"(5)"},
			want: true,
		},
		{
			name: "other error",
			err:  &testError{"some other error"},
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isBusyError(tt.err); got != tt.want {
				t.Errorf("isBusyError() = %v, want %v", got, tt.want)
			}
		})
	}
}

// testError implements error interface for testing.
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
