// Package db handles database connections, migrations, and schema management.
package db

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaFS embed.FS

const (
	currentSchemaVersion = 1
	walMode              = "PRAGMA journal_mode=WAL;"
	synchronousMode      = "PRAGMA synchronous=NORMAL;"
	cacheSize            = "PRAGMA cache_size=-64000;" // 64MB
	tempStore            = "PRAGMA temp_store=MEMORY;"
)

// Open opens a SQLite database at the given path, applying migrations if needed.
// It creates parent directories if they don't exist.
func Open(path string) (*sql.DB, error) {
	// Ensure parent directory exists (if path contains directories)
	if idx := maxPathSepIndex(path); idx >= 0 {
		if err := os.MkdirAll(path[:idx], 0755); err != nil {
			return nil, fmt.Errorf("create db directory: %w", err)
		}
	}

	// Open database with SQLite driver
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Apply performance optimizations
	if err := optimize(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("optimize database: %w", err)
	}

	// Run migrations
	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}

	return db, nil
}

// optimize applies SQLite performance optimizations.
func optimize(db *sql.DB) error {
	optimizations := []string{
		walMode,
		synchronousMode,
		cacheSize,
		tempStore,
	}

	for _, pragma := range optimizations {
		if _, err := db.Exec(pragma); err != nil {
			return fmt.Errorf("exec pragma %q: %w", pragma, err)
		}
	}
	return nil
}

// migrate runs database migrations.
func migrate(db *sql.DB) error {
	// Ensure migrations table exists
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at INTEGER NOT NULL
		)
	`); err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	// Get current version
	var version int
	err := db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version)
	if err != nil {
		return fmt.Errorf("query migration version: %w", err)
	}

	// If already at current version, we're done
	if version >= currentSchemaVersion {
		return nil
	}

	// Read schema file
	schema, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		return fmt.Errorf("read schema file: %w", err)
	}

	// Begin transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Execute schema
	if _, err := tx.Exec(string(schema)); err != nil {
		return fmt.Errorf("exec schema: %w", err)
	}

	// Record migration
	if _, err := tx.Exec(
		"INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)",
		currentSchemaVersion, time.Now().UnixMilli(),
	); err != nil {
		return fmt.Errorf("record migration: %w", err)
	}

	return tx.Commit()
}

// maxPathSepIndex returns the index of the last path separator.
func maxPathSepIndex(path string) int {
	maxIdx := -1
	for i, c := range path {
		if c == '/' || c == '\\' {
			maxIdx = i
		}
	}
	return maxIdx
}

// Close closes the database connection.
func Close(db *sql.DB) error {
	return db.Close()
}
