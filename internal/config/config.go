// Package config provides configuration management for the analytics service.
package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the analytics service.
type Config struct {
	Addr         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	DBPath          string
	MaxOpenConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration

	APIKey string

	RetentionDays int

	DashboardAPIKey string

	CORSOrigins string
}

// Defaults returns a configuration with sensible defaults.
func Defaults() Config {
	return Config{
		Addr:             ":3001",
		ReadTimeout:      10 * time.Second,
		WriteTimeout:     10 * time.Second,
		DBPath:           "./analytics.db",
		MaxOpenConns:     25,
		ConnMaxLifetime:  5 * time.Minute,
		ConnMaxIdleTime:  1 * time.Minute,
		APIKey:           "",
		RetentionDays:    90,
		DashboardAPIKey:  "",
		CORSOrigins:      "",
	}
}

// Load loads configuration from environment variables and command-line flags.
func Load() Config {
	cfg := Defaults()

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
	if dbPath := os.Getenv("ANALYTICS_DB_PATH"); dbPath != "" {
		cfg.DBPath = dbPath
	}
	if maxOpen := os.Getenv("ANALYTICS_MAX_OPEN_CONNS"); maxOpen != "" {
		if n, err := strconv.Atoi(maxOpen); err == nil && n > 0 {
			cfg.MaxOpenConns = n
		}
	}
	if apiKey := os.Getenv("ANALYTICS_API_KEY"); apiKey != "" {
		cfg.APIKey = apiKey
	}
	if dashboardKey := os.Getenv("ANALYTICS_DASHBOARD_API_KEY"); dashboardKey != "" {
		cfg.DashboardAPIKey = dashboardKey
	}
	if retention := os.Getenv("ANALYTICS_RETENTION_DAYS"); retention != "" {
		if n, err := strconv.Atoi(retention); err == nil && n >= 0 {
			cfg.RetentionDays = n
		}
	}
	if origins := os.Getenv("ANALYTICS_CORS_ORIGINS"); origins != "" {
		cfg.CORSOrigins = origins
	}

	fs := flag.NewFlagSet("analytics", flag.ContinueOnError)
	fs.StringVar(&cfg.Addr, "addr", cfg.Addr, "Address to listen on")
	fs.StringVar(&cfg.DBPath, "db", cfg.DBPath, "Path to SQLite database")
	fs.IntVar(&cfg.MaxOpenConns, "max-open-conns", cfg.MaxOpenConns, "Max open DB connections")
	fs.DurationVar(&cfg.ConnMaxLifetime, "conn-max-lifetime", cfg.ConnMaxLifetime, "Max connection lifetime")
	fs.DurationVar(&cfg.ConnMaxIdleTime, "conn-max-idle-time", cfg.ConnMaxIdleTime, "Max connection idle time")
	fs.StringVar(&cfg.APIKey, "api-key", cfg.APIKey, "API key for write operations")
	fs.StringVar(&cfg.DashboardAPIKey, "dashboard-key", cfg.DashboardAPIKey, "API key for dashboard")
	fs.IntVar(&cfg.RetentionDays, "retention-days", cfg.RetentionDays, "Days to retain raw events (0=forever)")
	fs.StringVar(&cfg.CORSOrigins, "cors-origins", cfg.CORSOrigins, "Comma-separated allowed CORS origins (default: *)")

	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Analytics backend service")
		fmt.Fprintln(os.Stderr, "\nEnvironment variables:")
		fmt.Fprintln(os.Stderr, "  ANALYTICS_ADDR              Address to listen on (default \":3001\")")
		fmt.Fprintln(os.Stderr, "  ANALYTICS_DB_PATH           Path to SQLite database")
		fmt.Fprintln(os.Stderr, "  ANALYTICS_API_KEY           API key for write operations")
		fmt.Fprintln(os.Stderr, "  ANALYTICS_DASHBOARD_API_KEY API key for dashboard access")
		fmt.Fprintln(os.Stderr, "  ANALYTICS_RETENTION_DAYS    Days to retain raw events (0=forever)")
		fmt.Fprintln(os.Stderr, "  ANALYTICS_CORS_ORIGINS      Comma-separated allowed CORS origins")
		fmt.Fprintln(os.Stderr, "\nFlags:")
		fs.PrintDefaults()
	}

	_ = fs.Parse(os.Args[1:])

	if cfg.CORSOrigins == "" {
		cfg.CORSOrigins = "*"
	}

	return cfg
}
