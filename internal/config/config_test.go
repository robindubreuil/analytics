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
		name string
		check func() bool
	}{
		{
			name: "Addr should be :3001",
			check: func() bool { return cfg.Addr == ":3001" },
		},
		{
			name: "ReadTimeout should be 10s",
			check: func() bool { return cfg.ReadTimeout == 10*time.Second },
		},
		{
			name: "WriteTimeout should be 10s",
			check: func() bool { return cfg.WriteTimeout == 10*time.Second },
		},
		{
			name: "DBPath should be ./analytics.db",
			check: func() bool { return cfg.DBPath == "./analytics.db" },
		},
		{
			name: "MaxOpenConns should be 25",
			check: func() bool { return cfg.MaxOpenConns == 25 },
		},
		{
			name: "ConnMaxLifetime should be 5m",
			check: func() bool { return cfg.ConnMaxLifetime == 5*time.Minute },
		},
		{
			name: "ConnMaxIdleTime should be 1m",
			check: func() bool { return cfg.ConnMaxIdleTime == 1*time.Minute },
		},
		{
			name: "APIKey should be empty",
			check: func() bool { return cfg.APIKey == "" },
		},
		{
			name: "EnableAnonymousTracking should be true",
			check: func() bool { return cfg.EnableAnonymousTracking == true },
		},
		{
			name: "RetentionDays should be 90",
			check: func() bool { return cfg.RetentionDays == 90 },
		},
		{
			name: "DashboardAPIKey should be empty",
			check: func() bool { return cfg.DashboardAPIKey == "" },
		},
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
	// Save original env vars
	origAddr := os.Getenv("ANALYTICS_ADDR")
	origDBPath := os.Getenv("ANALYTICS_DB_PATH")
	origMaxOpen := os.Getenv("ANALYTICS_MAX_OPEN_CONNS")
	origAPIKey := os.Getenv("ANALYTICS_API_KEY")
	origDashboardKey := os.Getenv("ANALYTICS_DASHBOARD_API_KEY")
	origAnon := os.Getenv("ANALYTICS_ENABLE_ANONYMOUS")
	origRetention := os.Getenv("ANALYTICS_RETENTION_DAYS")
	origReadTimeout := os.Getenv("ANALYTICS_READ_TIMEOUT")
	origWriteTimeout := os.Getenv("ANALYTICS_WRITE_TIMEOUT")

	defer func() {
		// Restore original env vars
		if origAddr != "" {
			os.Setenv("ANALYTICS_ADDR", origAddr)
		} else {
			os.Unsetenv("ANALYTICS_ADDR")
		}
		if origDBPath != "" {
			os.Setenv("ANALYTICS_DB_PATH", origDBPath)
		} else {
			os.Unsetenv("ANALYTICS_DB_PATH")
		}
		if origMaxOpen != "" {
			os.Setenv("ANALYTICS_MAX_OPEN_CONNS", origMaxOpen)
		} else {
			os.Unsetenv("ANALYTICS_MAX_OPEN_CONNS")
		}
		if origAPIKey != "" {
			os.Setenv("ANALYTICS_API_KEY", origAPIKey)
		} else {
			os.Unsetenv("ANALYTICS_API_KEY")
		}
		if origDashboardKey != "" {
			os.Setenv("ANALYTICS_DASHBOARD_API_KEY", origDashboardKey)
		} else {
			os.Unsetenv("ANALYTICS_DASHBOARD_API_KEY")
		}
		if origAnon != "" {
			os.Setenv("ANALYTICS_ENABLE_ANONYMOUS", origAnon)
		} else {
			os.Unsetenv("ANALYTICS_ENABLE_ANONYMOUS")
		}
		if origRetention != "" {
			os.Setenv("ANALYTICS_RETENTION_DAYS", origRetention)
		} else {
			os.Unsetenv("ANALYTICS_RETENTION_DAYS")
		}
		if origReadTimeout != "" {
			os.Setenv("ANALYTICS_READ_TIMEOUT", origReadTimeout)
		} else {
			os.Unsetenv("ANALYTICS_READ_TIMEOUT")
		}
		if origWriteTimeout != "" {
			os.Setenv("ANALYTICS_WRITE_TIMEOUT", origWriteTimeout)
		} else {
			os.Unsetenv("ANALYTICS_WRITE_TIMEOUT")
		}
	}()

	// Set test env vars
	os.Setenv("ANALYTICS_ADDR", ":8080")
	os.Setenv("ANALYTICS_DB_PATH", "/tmp/test.db")
	os.Setenv("ANALYTICS_MAX_OPEN_CONNS", "50")
	os.Setenv("ANALYTICS_API_KEY", "test-api-key")
	os.Setenv("ANALYTICS_DASHBOARD_API_KEY", "test-dashboard-key")
	os.Setenv("ANALYTICS_ENABLE_ANONYMOUS", "false")
	os.Setenv("ANALYTICS_RETENTION_DAYS", "30")
	os.Setenv("ANALYTICS_READ_TIMEOUT", "30s")
	os.Setenv("ANALYTICS_WRITE_TIMEOUT", "30s")

	// Clear args for testing
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
	if cfg.EnableAnonymousTracking {
		t.Errorf("Expected EnableAnonymousTracking false, got %v", cfg.EnableAnonymousTracking)
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
}

func TestLoadWithInvalidEnvVars(t *testing.T) {
	// Test that invalid env vars are ignored and defaults are used
	origMaxOpen := os.Getenv("ANALYTICS_MAX_OPEN_CONNS")
	origRetention := os.Getenv("ANALYTICS_RETENTION_DAYS")
	origAnon := os.Getenv("ANALYTICS_ENABLE_ANONYMOUS")

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
		if origAnon != "" {
			os.Setenv("ANALYTICS_ENABLE_ANONYMOUS", origAnon)
		} else {
			os.Unsetenv("ANALYTICS_ENABLE_ANONYMOUS")
		}
	}()

	// Set invalid env vars
	os.Setenv("ANALYTICS_MAX_OPEN_CONNS", "invalid")
	os.Setenv("ANALYTICS_RETENTION_DAYS", "-1")
	os.Setenv("ANALYTICS_ENABLE_ANONYMOUS", "yes")

	// Clear args for testing
	oldArgs := os.Args
	os.Args = []string{"analytics"}
	defer func() { os.Args = oldArgs }()

	cfg := Load()

	// Should use defaults for invalid values
	if cfg.MaxOpenConns != 25 {
		t.Errorf("Expected default MaxOpenConns 25 for invalid input, got %d", cfg.MaxOpenConns)
	}
	if cfg.RetentionDays != 90 {
		t.Errorf("Expected default RetentionDays 90 for invalid input, got %d", cfg.RetentionDays)
	}
	// "yes" is not a valid boolean value, so it's ignored and default (true) is kept
	if cfg.EnableAnonymousTracking != true {
		t.Errorf("Expected default EnableAnonymousTracking true for invalid value 'yes', got %v", cfg.EnableAnonymousTracking)
	}
}

func TestLoadWithInvalidTimeout(t *testing.T) {
	// Test that invalid timeout values are ignored
	origReadTimeout := os.Getenv("ANALYTICS_READ_TIMEOUT")
	origWriteTimeout := os.Getenv("ANALYTICS_WRITE_TIMEOUT")

	defer func() {
		if origReadTimeout != "" {
			os.Setenv("ANALYTICS_READ_TIMEOUT", origReadTimeout)
		} else {
			os.Unsetenv("ANALYTICS_READ_TIMEOUT")
		}
		if origWriteTimeout != "" {
			os.Setenv("ANALYTICS_WRITE_TIMEOUT", origWriteTimeout)
		} else {
			os.Unsetenv("ANALYTICS_WRITE_TIMEOUT")
		}
	}()

	os.Setenv("ANALYTICS_READ_TIMEOUT", "invalid")

	// Clear args for testing
	oldArgs := os.Args
	os.Args = []string{"analytics"}
	defer func() { os.Args = oldArgs }()

	cfg := Load()

	// Should use default for invalid timeout
	if cfg.ReadTimeout != 10*time.Second {
		t.Errorf("Expected default ReadTimeout 10s for invalid input, got %v", cfg.ReadTimeout)
	}
}

func TestEnableAnonymousTrackingVariants(t *testing.T) {
	testCases := []struct {
		value    string
		expected bool
	}{
		{"true", true},
		{"TRUE", true},   // case-insensitive
		{"True", true},   // case-insensitive
		{"1", true},
		{"false", false},
		{"0", false},
		{"", true},  // empty uses default (true)
		{"no", true},   // invalid value, uses default (true)
		{"yes", true},  // invalid value, uses default (true)
	}

	for _, tc := range testCases {
		t.Run(tc.value, func(t *testing.T) {
			origAnon := os.Getenv("ANALYTICS_ENABLE_ANONYMOUS")
			defer func() {
				if origAnon != "" {
					os.Setenv("ANALYTICS_ENABLE_ANONYMOUS", origAnon)
				} else {
					os.Unsetenv("ANALYTICS_ENABLE_ANONYMOUS")
				}
			}()

			if tc.value != "" {
				os.Setenv("ANALYTICS_ENABLE_ANONYMOUS", tc.value)
			} else {
				os.Unsetenv("ANALYTICS_ENABLE_ANONYMOUS")
			}

			// Clear args for testing
			oldArgs := os.Args
			os.Args = []string{"analytics"}
			defer func() { os.Args = oldArgs }()

			cfg := Load()
			if cfg.EnableAnonymousTracking != tc.expected {
				t.Errorf("For value %q, expected %v, got %v", tc.value, tc.expected, cfg.EnableAnonymousTracking)
			}
		})
	}
}

func TestRetentionDaysZeroAndNegative(t *testing.T) {
	testCases := []struct {
		value    string
		expected int
	}{
		{"0", 0},       // zero means forever
		{"-1", 90},     // negative uses default
		{"100", 100},   // valid positive
		{"365", 365},   // valid larger value
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

			// Clear args for testing
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
		{"0", 25},     // zero uses default
		{"-1", 25},    // negative uses default
		{"1", 1},      // minimum valid
		{"100", 100},  // valid
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

			// Clear args for testing
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
