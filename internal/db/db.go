// Package db handles database connections, migrations, and schema management.
package db

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

const dsnSuffix = "?_pragma=busy_timeout(5000)" +
	"&_pragma=foreign_keys(1)" +
	"&_pragma=journal_mode(WAL)" +
	"&_pragma=synchronous(NORMAL)" +
	"&_pragma=cache_size(-64000)" + // 64MB
	"&_pragma=temp_store(MEMORY)"

// Open opens a SQLite database at the given path, applying migrations if needed.
// It creates parent directories if they don't exist.
// Performance and safety pragmas are set in the DSN so they apply to every
// pooled connection, not just the one that happens to run first.
func Open(path string) (*sql.DB, error) {
	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create db directory: %w", err)
		}
	}

	// Open database with SQLite driver
	db, err := sql.Open("sqlite", path+dsnSuffix)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Run migrations
	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}

	return db, nil
}

// migrate runs database migrations from the embedded migrations directory.
// Migrations are applied in numeric order (001, 002, etc.).
func migrate(db *sql.DB) error {
	// Ensure migrations tracking table exists
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at INTEGER NOT NULL
		)
	`); err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	// Get applied migrations
	appliedVersions := make(map[int64]bool)
	rows, err := db.Query("SELECT version FROM schema_migrations")
	if err != nil {
		return fmt.Errorf("query applied migrations: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			return fmt.Errorf("scan migration version: %w", err)
		}
		appliedVersions[v] = true
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate applied migrations: %w", err)
	}

	// Find all migration files
	migrationFiles, err := embedFSReadDir(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations directory: %w", err)
	}

	// Sort migrations by version number
	sort.Slice(migrationFiles, func(i, j int) bool {
		return extractVersion(migrationFiles[i]) < extractVersion(migrationFiles[j])
	})

	// Apply pending migrations
	for _, file := range migrationFiles {
		if !strings.HasSuffix(file, ".sql") {
			continue
		}

		version := extractVersion(file)
		if version == 0 {
			continue // Skip files that don't match naming pattern
		}

		if appliedVersions[version] {
			continue // Already applied
		}

		// Read migration file
		migrationPath := "migrations/" + file
		content, err := migrationsFS.ReadFile(migrationPath)
		if err != nil {
			return fmt.Errorf("read migration file %s: %w", file, err)
		}

		// Execute migration in transaction
		if err := applyMigration(db, version, file, content); err != nil {
			return fmt.Errorf("apply migration %s: %w", file, err)
		}
	}

	return nil
}

// applyMigration executes a single migration within a transaction.
func applyMigration(db *sql.DB, version int64, name string, content []byte) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Execute migration SQL
	if _, err := tx.Exec(string(content)); err != nil {
		return fmt.Errorf("exec migration SQL: %w", err)
	}

	// Record migration
	if _, err := tx.Exec(
		"INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)",
		version, name, time.Now().UnixMilli(),
	); err != nil {
		return fmt.Errorf("record migration: %w", err)
	}

	return tx.Commit()
}

// extractVersion extracts the version number from a migration filename.
// Expected format: 001_description.sql, 002_add_feature.sql, etc.
// Returns 0 if the filename doesn't match the expected pattern.
func extractVersion(filename string) int64 {
	// Remove .sql extension
	base := strings.TrimSuffix(filename, ".sql")

	// Split by underscore
	parts := strings.SplitN(base, "_", 2)
	if len(parts) == 0 {
		return 0
	}

	// Parse numeric prefix
	version, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0
	}

	return version
}

// embedFSReadDir reads a directory from an embed.FS and returns file names.
func embedFSReadDir(fs embed.FS, dir string) ([]string, error) {
	entries, err := fs.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names, nil
}

// Close closes the database connection.
func Close(db *sql.DB) error {
	return db.Close()
}
