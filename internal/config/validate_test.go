package config

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// validConfig returns a Config with all valid default values for reuse
// across validation tests. Each field is set to a known-valid value.
func validConfig() Config {
	return Config{
		ClientID:        "d3590ed6-52b3-4102-aeff-aad2292ab01c",
		TenantID:        "common",
		AuthRecordPath:  "/tmp/auth_record.json",
		CacheName:       "outlook-local-mcp",
		DefaultTimezone: "UTC",
		LogLevel:        "info",
		LogFormat:       "json",
		MaxRetries:      3,
		RetryBackoffMS:  1000,
		RequestTimeout:  30 * time.Second,
		AuthMethod:      "browser",
		TokenStorage:    "auto",
	}
}

// TestValidateConfig_AllValid verifies that a fully valid config returns nil.
func TestValidateConfig_AllValid(t *testing.T) {
	if err := ValidateConfig(validConfig()); err != nil {
		t.Errorf("ValidateConfig() returned error for valid config: %v", err)
	}
}

// TestValidateConfig_InvalidClientID verifies that invalid ClientID values
// are rejected, including empty, partial UUID, and non-hex characters.
func TestValidateConfig_InvalidClientID(t *testing.T) {
	tests := []struct {
		name     string
		clientID string
	}{
		{"empty", ""},
		{"not uuid", "not-a-uuid"},
		{"partial uuid", "d3590ed6-52b3-4102-aeff"},
		{"too short segment", "d3590ed6-52b3-410-aeff-aad2292ab01c"},
		{"non-hex chars", "g3590ed6-52b3-4102-aeff-aad2292ab01c"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.ClientID = tt.clientID
			err := ValidateConfig(cfg)
			if err == nil {
				t.Error("expected error for invalid ClientID")
			}
			if err != nil && !strings.Contains(err.Error(), "ClientID") {
				t.Errorf("error should mention ClientID: %v", err)
			}
		})
	}
}

// TestValidateConfig_TenantID verifies that TenantID validation accepts
// well-known aliases (case-insensitive) and UUIDs, and rejects invalid values.
func TestValidateConfig_TenantID(t *testing.T) {
	validCases := []struct {
		name     string
		tenantID string
	}{
		{"common", "common"},
		{"organizations", "organizations"},
		{"consumers", "consumers"},
		{"Common uppercase", "Common"},
		{"ORGANIZATIONS uppercase", "ORGANIZATIONS"},
		{"uuid", "a1b2c3d4-e5f6-7890-abcd-ef1234567890"},
	}
	for _, tt := range validCases {
		t.Run("valid_"+tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.TenantID = tt.tenantID
			if err := ValidateConfig(cfg); err != nil {
				t.Errorf("expected no error for TenantID %q: %v", tt.tenantID, err)
			}
		})
	}

	invalidCases := []struct {
		name     string
		tenantID string
	}{
		{"empty", ""},
		{"random string", "my-tenant"},
		{"almost uuid", "d3590ed6-52b3-4102-aeff"},
	}
	for _, tt := range invalidCases {
		t.Run("invalid_"+tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.TenantID = tt.tenantID
			err := ValidateConfig(cfg)
			if err == nil {
				t.Errorf("expected error for TenantID %q", tt.tenantID)
			}
		})
	}
}

// TestValidateConfig_DefaultTimezone verifies that valid IANA timezones pass
// and invalid timezone strings are rejected.
func TestValidateConfig_DefaultTimezone(t *testing.T) {
	validCases := []string{"UTC", "America/New_York", "Europe/London", "Asia/Tokyo"}
	for _, tz := range validCases {
		t.Run("valid_"+tz, func(t *testing.T) {
			cfg := validConfig()
			cfg.DefaultTimezone = tz
			if err := ValidateConfig(cfg); err != nil {
				t.Errorf("expected no error for timezone %q: %v", tz, err)
			}
		})
	}

	invalidCases := []string{"", "Not/A/Timezone", "invalid"}
	for _, tz := range invalidCases {
		t.Run("invalid_"+tz, func(t *testing.T) {
			cfg := validConfig()
			cfg.DefaultTimezone = tz
			err := ValidateConfig(cfg)
			if err == nil {
				t.Errorf("expected error for timezone %q", tz)
			}
		})
	}
}

// TestValidateConfig_LogLevel verifies that all four valid log levels pass
// (case-insensitive) and invalid levels are rejected.
func TestValidateConfig_LogLevel(t *testing.T) {
	validCases := []string{"debug", "info", "warn", "error", "DEBUG", "Info", "WARN", "Error"}
	for _, lvl := range validCases {
		t.Run("valid_"+lvl, func(t *testing.T) {
			cfg := validConfig()
			cfg.LogLevel = lvl
			if err := ValidateConfig(cfg); err != nil {
				t.Errorf("expected no error for LogLevel %q: %v", lvl, err)
			}
		})
	}

	invalidCases := []string{"", "trace", "fatal", "verbose"}
	for _, lvl := range invalidCases {
		t.Run("invalid_"+lvl, func(t *testing.T) {
			cfg := validConfig()
			cfg.LogLevel = lvl
			err := ValidateConfig(cfg)
			if err == nil {
				t.Errorf("expected error for LogLevel %q", lvl)
			}
		})
	}
}

// TestValidateConfig_LogFormat verifies that "json" and "text" pass
// (case-insensitive) and invalid formats are rejected.
func TestValidateConfig_LogFormat(t *testing.T) {
	validCases := []string{"json", "text", "JSON", "Text", "TEXT"}
	for _, fmt := range validCases {
		t.Run("valid_"+fmt, func(t *testing.T) {
			cfg := validConfig()
			cfg.LogFormat = fmt
			if err := ValidateConfig(cfg); err != nil {
				t.Errorf("expected no error for LogFormat %q: %v", fmt, err)
			}
		})
	}

	invalidCases := []string{"", "xml", "csv", "yaml"}
	for _, fmt := range invalidCases {
		t.Run("invalid_"+fmt, func(t *testing.T) {
			cfg := validConfig()
			cfg.LogFormat = fmt
			err := ValidateConfig(cfg)
			if err == nil {
				t.Errorf("expected error for LogFormat %q", fmt)
			}
		})
	}
}

// TestValidateConfig_CacheName verifies that empty and oversized cache names
// are rejected, while valid names and max-length names pass.
func TestValidateConfig_CacheName(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		cfg := validConfig()
		cfg.CacheName = ""
		err := ValidateConfig(cfg)
		if err == nil {
			t.Error("expected error for empty CacheName")
		}
	})

	t.Run("too long", func(t *testing.T) {
		cfg := validConfig()
		cfg.CacheName = strings.Repeat("a", 129)
		err := ValidateConfig(cfg)
		if err == nil {
			t.Error("expected error for CacheName > 128 chars")
		}
	})

	t.Run("max length", func(t *testing.T) {
		cfg := validConfig()
		cfg.CacheName = strings.Repeat("a", 128)
		if err := ValidateConfig(cfg); err != nil {
			t.Errorf("expected no error for CacheName at 128 chars: %v", err)
		}
	})

	t.Run("normal", func(t *testing.T) {
		cfg := validConfig()
		cfg.CacheName = "my-cache"
		if err := ValidateConfig(cfg); err != nil {
			t.Errorf("expected no error for valid CacheName: %v", err)
		}
	})
}

// TestValidateConfig_MaxRetries verifies that values outside 0-10 are rejected
// and boundary values are accepted.
func TestValidateConfig_MaxRetries(t *testing.T) {
	tests := []struct {
		name    string
		value   int
		wantErr bool
	}{
		{"below range", -1, true},
		{"above range", 11, true},
		{"min boundary", 0, false},
		{"max boundary", 10, false},
		{"mid range", 5, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.MaxRetries = tt.value
			err := ValidateConfig(cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("MaxRetries=%d: got err=%v, wantErr=%v", tt.value, err, tt.wantErr)
			}
		})
	}
}

// TestValidateConfig_RetryBackoffMS verifies that values outside 100-30000
// are rejected and boundary values are accepted.
func TestValidateConfig_RetryBackoffMS(t *testing.T) {
	tests := []struct {
		name    string
		value   int
		wantErr bool
	}{
		{"below range", 99, true},
		{"above range", 30001, true},
		{"min boundary", 100, false},
		{"max boundary", 30000, false},
		{"mid range", 5000, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.RetryBackoffMS = tt.value
			err := ValidateConfig(cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("RetryBackoffMS=%d: got err=%v, wantErr=%v", tt.value, err, tt.wantErr)
			}
		})
	}
}

// TestValidateConfig_RequestTimeout verifies that timeouts outside 1-300
// seconds are rejected and boundary values are accepted.
func TestValidateConfig_RequestTimeout(t *testing.T) {
	tests := []struct {
		name    string
		value   time.Duration
		wantErr bool
	}{
		{"below range", 0 * time.Second, true},
		{"above range", 301 * time.Second, true},
		{"min boundary", 1 * time.Second, false},
		{"max boundary", 300 * time.Second, false},
		{"mid range", 30 * time.Second, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.RequestTimeout = tt.value
			err := ValidateConfig(cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("RequestTimeout=%v: got err=%v, wantErr=%v", tt.value, err, tt.wantErr)
			}
		})
	}
}

// TestValidateConfig_MultipleErrors verifies that multiple invalid fields
// produce a combined error string containing all violations separated by "; ".
func TestValidateConfig_MultipleErrors(t *testing.T) {
	cfg := validConfig()
	cfg.ClientID = "bad"
	cfg.LogLevel = "invalid"

	err := ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected error for multiple invalid fields")
	}

	msg := err.Error()
	if !strings.Contains(msg, "ClientID") {
		t.Error("error should mention ClientID")
	}
	if !strings.Contains(msg, "LogLevel") {
		t.Error("error should mention LogLevel")
	}
	if !strings.Contains(msg, "; ") {
		t.Error("multiple errors should be separated by '; '")
	}
}

// TestValidateConfig_AllFieldsInvalid verifies that when every validatable
// field is invalid, all violations are reported in a single error.
func TestValidateConfig_AllFieldsInvalid(t *testing.T) {
	cfg := Config{
		ClientID:        "not-uuid",
		TenantID:        "bad-tenant",
		DefaultTimezone: "Invalid/Zone",
		LogLevel:        "verbose",
		LogFormat:       "xml",
		CacheName:       "",
		AuthRecordPath:  "/nonexistent/dir/auth.json",
		MaxRetries:      -1,
		RetryBackoffMS:  50,
		RequestTimeout:  0,
		AuthMethod:      "oauth",
		TokenStorage:    "memory",
	}

	err := ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected error when all fields are invalid")
	}

	msg := err.Error()
	expectedFields := []string{
		"ClientID", "TenantID", "DefaultTimezone",
		"LogLevel", "LogFormat", "CacheName",
		"MaxRetries", "RetryBackoffMS", "RequestTimeout",
		"AuthMethod", "TokenStorage",
	}
	for _, field := range expectedFields {
		if !strings.Contains(msg, field) {
			t.Errorf("error should mention %s: %s", field, msg)
		}
	}
}

// TestValidateConfig_AuthRecordPath verifies that a missing parent directory
// does not cause a validation error (best-effort warning only), and an
// existing parent directory produces no warning or error.
func TestValidateConfig_AuthRecordPath(t *testing.T) {
	t.Run("missing parent no error", func(t *testing.T) {
		cfg := validConfig()
		cfg.AuthRecordPath = "/nonexistent/deeply/nested/path/auth.json"
		// Should not fail -- missing parent is a warning only.
		if err := ValidateConfig(cfg); err != nil {
			t.Errorf("expected no error for missing parent dir: %v", err)
		}
	})

	t.Run("existing parent no error", func(t *testing.T) {
		cfg := validConfig()
		cfg.AuthRecordPath = "/tmp/auth_record.json"
		if err := ValidateConfig(cfg); err != nil {
			t.Errorf("expected no error for existing parent dir: %v", err)
		}
	})
}

// TestValidateConfig_LogFileParentExists verifies that no warning is logged
// when the LogFile parent directory exists.
func TestValidateConfig_LogFileParentExists(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	slog.SetDefault(logger)

	cfg := validConfig()
	cfg.LogFile = "/tmp/test.log"

	_ = ValidateConfig(cfg)

	logOutput := buf.String()
	if strings.Contains(logOutput, "LogFile parent directory does not exist") {
		t.Errorf("unexpected LogFile warning for existing parent dir: %s", logOutput)
	}
}

// TestValidateConfig_LogFileParentMissing verifies that a warning is logged
// when the LogFile parent directory does not exist.
func TestValidateConfig_LogFileParentMissing(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	slog.SetDefault(logger)

	cfg := validConfig()
	cfg.LogFile = "/nonexistent/dir/test.log"

	err := ValidateConfig(cfg)
	if err != nil {
		t.Errorf("expected no error for missing LogFile parent dir (warning only): %v", err)
	}

	logOutput := buf.String()
	if !strings.Contains(logOutput, "LogFile parent directory does not exist") {
		t.Errorf("expected LogFile warning for missing parent dir, got: %s", logOutput)
	}
}

// TestValidateConfig_LogFileEmpty verifies that no validation action is taken
// when LogFile is empty.
func TestValidateConfig_LogFileEmpty(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	slog.SetDefault(logger)

	cfg := validConfig()
	cfg.LogFile = ""

	_ = ValidateConfig(cfg)

	logOutput := buf.String()
	if strings.Contains(logOutput, "LogFile") {
		t.Errorf("unexpected LogFile log output when LogFile is empty: %s", logOutput)
	}
}

// TestValidateConfig_ReadOnlyNonStandardWarning verifies that non-standard
// OUTLOOK_MCP_READ_ONLY values (e.g., "yes", "1") produce a slog.Warn log,
// and that standard values ("true", "false", empty/unset) do not.
func TestValidateConfig_ReadOnlyNonStandardWarning(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		wantWarn bool
	}{
		{"non-standard yes", "yes", true},
		{"non-standard 1", "1", true},
		{"non-standard enabled", "enabled", true},
		{"standard true", "true", false},
		{"standard TRUE", "TRUE", false},
		{"standard false", "false", false},
		{"standard FALSE", "FALSE", false},
		{"empty treated as unset", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("OUTLOOK_MCP_READ_ONLY", tt.envValue)

			var buf bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
			slog.SetDefault(logger)

			cfg := validConfig()
			_ = ValidateConfig(cfg)

			logOutput := buf.String()
			hasWarn := strings.Contains(logOutput, "non-standard OUTLOOK_MCP_READ_ONLY")
			if hasWarn != tt.wantWarn {
				t.Errorf("OUTLOOK_MCP_READ_ONLY=%q: gotWarn=%v, wantWarn=%v, log=%q",
					tt.envValue, hasWarn, tt.wantWarn, logOutput)
			}
		})
	}
}

// TestValidateConfig_AuthMethodAuthCode verifies that "auth_code" passes
// AuthMethod validation.
func TestValidateConfig_AuthMethodAuthCode(t *testing.T) {
	cfg := validConfig()
	cfg.AuthMethod = "auth_code"

	if err := ValidateConfig(cfg); err != nil {
		t.Errorf("expected no error for AuthMethod %q: %v", cfg.AuthMethod, err)
	}
}

// TestValidateConfig_AuthMethodBrowser verifies that "browser" passes
// AuthMethod validation.
func TestValidateConfig_AuthMethodBrowser(t *testing.T) {
	cfg := validConfig()
	cfg.AuthMethod = "browser"

	if err := ValidateConfig(cfg); err != nil {
		t.Errorf("expected no error for AuthMethod %q: %v", cfg.AuthMethod, err)
	}
}

// TestValidateConfig_AuthMethodDeviceCode verifies that "device_code" passes
// AuthMethod validation.
func TestValidateConfig_AuthMethodDeviceCode(t *testing.T) {
	cfg := validConfig()
	cfg.AuthMethod = "device_code"

	if err := ValidateConfig(cfg); err != nil {
		t.Errorf("expected no error for AuthMethod %q: %v", cfg.AuthMethod, err)
	}
}

// TestValidateConfig_AuthMethodInvalid verifies that invalid AuthMethod values
// produce a validation error mentioning "AuthMethod".
func TestValidateConfig_AuthMethodInvalid(t *testing.T) {
	tests := []struct {
		name       string
		authMethod string
	}{
		{"empty", ""},
		{"oauth", "oauth"},
		{"certificate", "certificate"},
		{"token", "token"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.AuthMethod = tt.authMethod
			err := ValidateConfig(cfg)
			if err == nil {
				t.Errorf("expected error for AuthMethod %q", tt.authMethod)
			}
			if err != nil && !strings.Contains(err.Error(), "AuthMethod") {
				t.Errorf("error should mention AuthMethod: %v", err)
			}
		})
	}
}

// TestValidateConfig_BadTimezoneMessage verifies that an invalid timezone value
// produces an error message that includes example valid values.
func TestValidateConfig_BadTimezoneMessage(t *testing.T) {
	cfg := validConfig()
	cfg.DefaultTimezone = "NotATimezone"

	err := ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected error for invalid timezone")
	}

	msg := err.Error()
	if !strings.Contains(msg, "Examples:") {
		t.Errorf("error should contain 'Examples:', got: %s", msg)
	}
	if !strings.Contains(msg, "NotATimezone") {
		t.Errorf("error should contain the invalid value 'NotATimezone', got: %s", msg)
	}
}

// TestValidateConfig_TokenStorage_Valid verifies that all three valid
// TokenStorage values pass validation.
func TestValidateConfig_TokenStorage_Valid(t *testing.T) {
	for _, val := range []string{"auto", "keychain", "file"} {
		t.Run(val, func(t *testing.T) {
			cfg := validConfig()
			cfg.TokenStorage = val
			if err := ValidateConfig(cfg); err != nil {
				t.Errorf("expected no error for TokenStorage %q: %v", val, err)
			}
		})
	}
}

// TestValidateConfig_TokenStorage_Invalid verifies that invalid TokenStorage
// values produce a validation error mentioning "TokenStorage".
func TestValidateConfig_TokenStorage_Invalid(t *testing.T) {
	for _, val := range []string{"", "memory", "vault", "dpapi"} {
		t.Run(val, func(t *testing.T) {
			cfg := validConfig()
			cfg.TokenStorage = val
			err := ValidateConfig(cfg)
			if err == nil {
				t.Errorf("expected error for TokenStorage %q", val)
			}
			if err != nil && !strings.Contains(err.Error(), "TokenStorage") {
				t.Errorf("error should mention TokenStorage: %v", err)
			}
		})
	}
}
