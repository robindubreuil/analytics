// Package config provides configuration management for the analytics service.
package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration for the analytics service.
type Config struct {
	// Server settings
	Addr         string        // Address to listen on (e.g., ":3000")
	ReadTimeout  time.Duration // Maximum duration for reading the entire request
	WriteTimeout time.Duration // Maximum duration before timing out writes

	// Database settings
	DBPath          string        // Path to SQLite database file
	MaxOpenConns    int           // Maximum number of open connections
	ConnMaxLifetime time.Duration // Maximum amount of time a connection may be reused
	ConnMaxIdleTime time.Duration // Maximum amount of time a connection may be idle

	// Security
	APIKey string // Optional API key for write operations

	// Analytics settings
	EnableAnonymousTracking bool // Whether to track anonymous sessions
	RetentionDays           int  // Days to retain raw events (0 = forever)

	// Dashboard settings
	DashboardAPIKey string // Optional API key for dashboard read access
}

// Defaults returns a configuration with sensible defaults.
func Defaults() Config {
	return Config{
		Addr:                    ":3001",
		ReadTimeout:             10 * time.Second,
		WriteTimeout:            10 * time.Second,
		DBPath:                  "./analytics.db",
		MaxOpenConns:            25,
		ConnMaxLifetime:         5 * time.Minute,
		ConnMaxIdleTime:         1 * time.Minute,
		APIKey:                  "",
		EnableAnonymousTracking: true,
		RetentionDays:           90,
		DashboardAPIKey:         "",
	}
}

// Load loads configuration from environment variables and command-line flags.
// Flags take precedence over environment variables.
func Load() Config {
	cfg := Defaults()

	// Server settings
	if addr := os.Getenv("ANALYTICS_ADDR"); addr != "" {
		cfg.Addr = addr
	}
	if rt := os.Getenv("ANALYTICS_READ_TIMEOUT"); rt != "" {
		if d, err := time.ParseDuration(rt); err == nil {
			cfg.ReadTimeout = d
		}
	}
	if wt := os.Getenv("ANALYTICS_WRITE_TIMEOUT"); wt != "" {
		if d, err := time.ParseDuration(wt); err == nil {
			cfg.WriteTimeout = d
		}
	}

	// Database settings
	if dbPath := os.Getenv("ANALYTICS_DB_PATH"); dbPath != "" {
		cfg.DBPath = dbPath
	}
	if maxOpen := os.Getenv("ANALYTICS_MAX_OPEN_CONNS"); maxOpen != "" {
		if n, err := strconv.Atoi(maxOpen); err == nil && n > 0 {
			cfg.MaxOpenConns = n
		}
	}

	// Security
	if apiKey := os.Getenv("ANALYTICS_API_KEY"); apiKey != "" {
		cfg.APIKey = apiKey
	}
	if dashboardKey := os.Getenv("ANALYTICS_DASHBOARD_API_KEY"); dashboardKey != "" {
		cfg.DashboardAPIKey = dashboardKey
	}

	// Analytics settings
	if anon := os.Getenv("ANALYTICS_ENABLE_ANONYMOUS"); anon != "" {
		lowerAnon := strings.ToLower(anon)
		if lowerAnon == "true" || anon == "1" {
			cfg.EnableAnonymousTracking = true
		} else if lowerAnon == "false" || anon == "0" {
			cfg.EnableAnonymousTracking = false
		}
		// Invalid values are ignored, keeping the default
	}
	if retention := os.Getenv("ANALYTICS_RETENTION_DAYS"); retention != "" {
		if n, err := strconv.Atoi(retention); err == nil && n >= 0 {
			cfg.RetentionDays = n
		}
	}

	// Parse command-line flags (override env vars)
	fs := flag.NewFlagSet("analytics", flag.ContinueOnError)
	fs.StringVar(&cfg.Addr, "addr", cfg.Addr, "Address to listen on")
	fs.StringVar(&cfg.DBPath, "db", cfg.DBPath, "Path to SQLite database")
	fs.IntVar(&cfg.MaxOpenConns, "max-open-conns", cfg.MaxOpenConns, "Max open DB connections")
	fs.StringVar(&cfg.APIKey, "api-key", cfg.APIKey, "API key for write operations")
	fs.StringVar(&cfg.DashboardAPIKey, "dashboard-key", cfg.DashboardAPIKey, "API key for dashboard")
	fs.BoolVar(&cfg.EnableAnonymousTracking, "enable-anonymous", cfg.EnableAnonymousTracking, "Enable anonymous tracking")
	fs.IntVar(&cfg.RetentionDays, "retention-days", cfg.RetentionDays, "Days to retain raw events (0=forever)")

	// Custom usage to avoid flag parsing errors from being fatal
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Analytics backend service")
		fmt.Fprintln(os.Stderr, "\nEnvironment variables:")
		fmt.Fprintln(os.Stderr, "  ANALYTICS_ADDR              Address to listen on (default \":3001\")")
		fmt.Fprintln(os.Stderr, "  ANALYTICS_DB_PATH           Path to SQLite database")
		fmt.Fprintln(os.Stderr, "  ANALYTICS_API_KEY           API key for write operations")
		fmt.Fprintln(os.Stderr, "  ANALYTICS_DASHBOARD_API_KEY API key for dashboard access")
		fmt.Fprintln(os.Stderr, "  ANALYTICS_RETENTION_DAYS    Days to retain raw events (0=forever)")
		fmt.Fprintln(os.Stderr, "\nFlags:")
		fs.PrintDefaults()
	}

	// Parse flags, but don't exit on error (ignore unknown flags in tests)
	_ = fs.Parse(os.Args[1:])

	return cfg
}
