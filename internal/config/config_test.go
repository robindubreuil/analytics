// Package config provides tests for configuration management.
package config

import (
	"os"
	"testing"
	"time"
)

func TestDefaults(t *testing.T) {
	cfg := Defaults()

	tests := []struct {
		name  string
		check func() bool
	}{
		{"Addr should be :3001", func() bool { return cfg.Addr == ":3001" }},
		{"ReadTimeout should be 10s", func() bool { return cfg.ReadTimeout == 10*time.Second }},
		{"WriteTimeout should be 10s", func() bool { return cfg.WriteTimeout == 10*time.Second }},
		{"DBPath should be ./analytics.db", func() bool { return cfg.DBPath == "./analytics.db" }},
		{"MaxOpenConns should be 25", func() bool { return cfg.MaxOpenConns == 25 }},
		{"ConnMaxLifetime should be 5m", func() bool { return cfg.ConnMaxLifetime == 5*time.Minute }},
		{"ConnMaxIdleTime should be 1m", func() bool { return cfg.ConnMaxIdleTime == 1*time.Minute }},
		{"APIKey should be empty", func() bool { return cfg.APIKey == "" }},
		{"RetentionDays should be 90", func() bool { return cfg.RetentionDays == 90 }},
		{"DashboardAPIKey should be empty", func() bool { return cfg.DashboardAPIKey == "" }},
		{"CORSOrigins should be empty", func() bool { return cfg.CORSOrigins == "" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.check() {
				t.Errorf("Check failed for: %s", tt.name)
			}
		})
	}
}

func TestLoadWithEnvVars(t *testing.T) {
	envVars := []string{
		"ANALYTICS_ADDR", "ANALYTICS_DB_PATH", "ANALYTICS_MAX_OPEN_CONNS",
		"ANALYTICS_API_KEY", "ANALYTICS_DASHBOARD_API_KEY", "ANALYTICS_RETENTION_DAYS",
		"ANALYTICS_READ_TIMEOUT", "ANALYTICS_WRITE_TIMEOUT", "ANALYTICS_CORS_ORIGINS",
	}

	origValues := make(map[string]string)
	for _, key := range envVars {
		origValues[key] = os.Getenv(key)
	}
	defer func() {
		for _, key := range envVars {
			if origValues[key] != "" {
				os.Setenv(key, origValues[key])
			} else {
				os.Unsetenv(key)
			}
		}
	}()

	os.Setenv("ANALYTICS_ADDR", ":8080")
	os.Setenv("ANALYTICS_DB_PATH", "/tmp/test.db")
	os.Setenv("ANALYTICS_MAX_OPEN_CONNS", "50")
	os.Setenv("ANALYTICS_API_KEY", "test-api-key")
	os.Setenv("ANALYTICS_DASHBOARD_API_KEY", "test-dashboard-key")
	os.Setenv("ANALYTICS_RETENTION_DAYS", "30")
	os.Setenv("ANALYTICS_READ_TIMEOUT", "30s")
	os.Setenv("ANALYTICS_WRITE_TIMEOUT", "30s")
	os.Setenv("ANALYTICS_CORS_ORIGINS", "https://example.com,https://analytics.example.com")

	oldArgs := os.Args
	os.Args = []string{"analytics"}
	defer func() { os.Args = oldArgs }()

	cfg := Load()

	if cfg.Addr != ":8080" {
		t.Errorf("Expected Addr :8080, got %s", cfg.Addr)
	}
	if cfg.DBPath != "/tmp/test.db" {
		t.Errorf("Expected DBPath /tmp/test.db, got %s", cfg.DBPath)
	}
	if cfg.MaxOpenConns != 50 {
		t.Errorf("Expected MaxOpenConns 50, got %d", cfg.MaxOpenConns)
	}
	if cfg.APIKey != "test-api-key" {
		t.Errorf("Expected APIKey test-api-key, got %s", cfg.APIKey)
	}
	if cfg.DashboardAPIKey != "test-dashboard-key" {
		t.Errorf("Expected DashboardAPIKey test-dashboard-key, got %s", cfg.DashboardAPIKey)
	}
	if cfg.RetentionDays != 30 {
		t.Errorf("Expected RetentionDays 30, got %d", cfg.RetentionDays)
	}
	if cfg.ReadTimeout != 30*time.Second {
		t.Errorf("Expected ReadTimeout 30s, got %v", cfg.ReadTimeout)
	}
	if cfg.WriteTimeout != 30*time.Second {
		t.Errorf("Expected WriteTimeout 30s, got %v", cfg.WriteTimeout)
	}
	if cfg.CORSOrigins != "https://example.com,https://analytics.example.com" {
		t.Errorf("Expected CORS origins, got %s", cfg.CORSOrigins)
	}
}

func TestLoadWithInvalidEnvVars(t *testing.T) {
	origMaxOpen := os.Getenv("ANALYTICS_MAX_OPEN_CONNS")
	origRetention := os.Getenv("ANALYTICS_RETENTION_DAYS")

	defer func() {
		if origMaxOpen != "" {
			os.Setenv("ANALYTICS_MAX_OPEN_CONNS", origMaxOpen)
		} else {
			os.Unsetenv("ANALYTICS_MAX_OPEN_CONNS")
		}
		if origRetention != "" {
			os.Setenv("ANALYTICS_RETENTION_DAYS", origRetention)
		} else {
			os.Unsetenv("ANALYTICS_RETENTION_DAYS")
		}
	}()

	os.Setenv("ANALYTICS_MAX_OPEN_CONNS", "invalid")
	os.Setenv("ANALYTICS_RETENTION_DAYS", "-1")

	oldArgs := os.Args
	os.Args = []string{"analytics"}
	defer func() { os.Args = oldArgs }()

	cfg := Load()

	if cfg.MaxOpenConns != 25 {
		t.Errorf("Expected default MaxOpenConns 25 for invalid input, got %d", cfg.MaxOpenConns)
	}
	if cfg.RetentionDays != 90 {
		t.Errorf("Expected default RetentionDays 90 for invalid input, got %d", cfg.RetentionDays)
	}
}

func TestLoadWithInvalidTimeout(t *testing.T) {
	origReadTimeout := os.Getenv("ANALYTICS_READ_TIMEOUT")

	defer func() {
		if origReadTimeout != "" {
			os.Setenv("ANALYTICS_READ_TIMEOUT", origReadTimeout)
		} else {
			os.Unsetenv("ANALYTICS_READ_TIMEOUT")
		}
	}()

	os.Setenv("ANALYTICS_READ_TIMEOUT", "invalid")

	oldArgs := os.Args
	os.Args = []string{"analytics"}
	defer func() { os.Args = oldArgs }()

	cfg := Load()

	if cfg.ReadTimeout != 10*time.Second {
		t.Errorf("Expected default ReadTimeout 10s for invalid input, got %v", cfg.ReadTimeout)
	}
}

func TestRetentionDaysZeroAndNegative(t *testing.T) {
	testCases := []struct {
		value    string
		expected int
	}{
		{"0", 0},
		{"-1", 90},
		{"100", 100},
		{"365", 365},
	}

	for _, tc := range testCases {
		t.Run(tc.value, func(t *testing.T) {
			origRetention := os.Getenv("ANALYTICS_RETENTION_DAYS")
			defer func() {
				if origRetention != "" {
					os.Setenv("ANALYTICS_RETENTION_DAYS", origRetention)
				} else {
					os.Unsetenv("ANALYTICS_RETENTION_DAYS")
				}
			}()

			os.Setenv("ANALYTICS_RETENTION_DAYS", tc.value)

			oldArgs := os.Args
			os.Args = []string{"analytics"}
			defer func() { os.Args = oldArgs }()

			cfg := Load()
			if cfg.RetentionDays != tc.expected {
				t.Errorf("For value %q, expected %d, got %d", tc.value, tc.expected, cfg.RetentionDays)
			}
		})
	}
}

func TestMaxOpenConnsValidation(t *testing.T) {
	testCases := []struct {
		value    string
		expected int
	}{
		{"0", 25},
		{"-1", 25},
		{"1", 1},
		{"100", 100},
	}

	for _, tc := range testCases {
		t.Run(tc.value, func(t *testing.T) {
			origMaxOpen := os.Getenv("ANALYTICS_MAX_OPEN_CONNS")
			defer func() {
				if origMaxOpen != "" {
					os.Setenv("ANALYTICS_MAX_OPEN_CONNS", origMaxOpen)
				} else {
					os.Unsetenv("ANALYTICS_MAX_OPEN_CONNS")
				}
			}()

			os.Setenv("ANALYTICS_MAX_OPEN_CONNS", tc.value)

			oldArgs := os.Args
			os.Args = []string{"analytics"}
			defer func() { os.Args = oldArgs }()

			cfg := Load()
			if cfg.MaxOpenConns != tc.expected {
				t.Errorf("For value %q, expected %d, got %d", tc.value, tc.expected, cfg.MaxOpenConns)
			}
		})
	}
}

func TestCORSOriginsDefault(t *testing.T) {
	origCORS := os.Getenv("ANALYTICS_CORS_ORIGINS")
	defer func() {
		if origCORS != "" {
			os.Setenv("ANALYTICS_CORS_ORIGINS", origCORS)
		} else {
			os.Unsetenv("ANALYTICS_CORS_ORIGINS")
		}
	}()

	os.Unsetenv("ANALYTICS_CORS_ORIGINS")

	oldArgs := os.Args
	os.Args = []string{"analytics"}
	defer func() { os.Args = oldArgs }()

	cfg := Load()

	if cfg.CORSOrigins != "*" {
		t.Errorf("Expected default CORSOrigins '*', got %s", cfg.CORSOrigins)
	}
}
