package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// uuidRegex matches a standard UUID string (8-4-4-4-12 hex format).
// Compiled once at package level for reuse across validation calls.
var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// validLogLevels is the set of accepted log severity levels.
var validLogLevels = map[string]bool{
	"debug": true,
	"info":  true,
	"warn":  true,
	"error": true,
}

// validLogFormats is the set of accepted log output formats.
var validLogFormats = map[string]bool{
	"json": true,
	"text": true,
}

// validAuthMethods is the set of accepted authentication method values.
// "auth_code" uses AuthCodeCredential; "browser" uses InteractiveBrowserCredential;
// "device_code" uses DeviceCodeCredential.
var validAuthMethods = map[string]bool{
	"auth_code":   true,
	"browser":     true,
	"device_code": true,
}

// validTokenStorageValues is the set of accepted token storage backend values.
// "auto" tries OS keychain first with file-based fallback; "keychain" uses OS
// keychain only; "file" uses file-based AES-256-GCM only.
var validTokenStorageValues = map[string]bool{
	"auto":     true,
	"keychain": true,
	"file":     true,
}

// wellKnownTenants is the set of accepted well-known Azure AD tenant aliases.
var wellKnownTenants = map[string]bool{
	"common":        true,
	"organizations": true,
	"consumers":     true,
}

// ValidateConfig checks all fields in the given Config against their
// documented constraints. Each invalid field is logged at slog.Error level
// with the field name, provided value, and expected constraint. If one or
// more fields are invalid, a combined error listing all violations is returned.
//
// Parameters:
//   - cfg: the Config struct to validate.
//
// Returns nil if all fields are valid, or an error describing all violations.
//
// Side effects: logs each validation failure at slog.Error level; logs a
// warning if the AuthRecordPath parent directory does not exist.
func ValidateConfig(cfg Config) error {
	var errs []string

	// ClientID must be a valid UUID.
	if !uuidRegex.MatchString(cfg.ClientID) {
		slog.Error("invalid configuration", "field", "ClientID", "value", cfg.ClientID, "expected", "UUID format (8-4-4-4-12 hex)")
		errs = append(errs, "ClientID must be a valid UUID")
	}

	// TenantID must be a well-known alias or a valid UUID.
	if !wellKnownTenants[strings.ToLower(cfg.TenantID)] && !uuidRegex.MatchString(cfg.TenantID) {
		slog.Error("invalid configuration", "field", "TenantID", "value", cfg.TenantID, "expected", "common, organizations, consumers, or UUID")
		errs = append(errs, "TenantID must be common, organizations, consumers, or a valid UUID")
	}

	// DefaultTimezone must be non-empty and a valid IANA timezone.
	// time.LoadLocation("") returns UTC without error, so emptiness is checked explicitly.
	if cfg.DefaultTimezone == "" {
		slog.Error("invalid configuration", "field", "DefaultTimezone", "value", cfg.DefaultTimezone, "expected", "valid IANA timezone")
		errs = append(errs, fmt.Sprintf("DefaultTimezone '%s' is not a valid IANA timezone. Examples: America/New_York, Europe/London, Asia/Tokyo, UTC", cfg.DefaultTimezone))
	} else if _, err := time.LoadLocation(cfg.DefaultTimezone); err != nil {
		slog.Error("invalid configuration", "field", "DefaultTimezone", "value", cfg.DefaultTimezone, "expected", "valid IANA timezone")
		errs = append(errs, fmt.Sprintf("DefaultTimezone '%s' is not a valid IANA timezone. Examples: America/New_York, Europe/London, Asia/Tokyo, UTC", cfg.DefaultTimezone))
	}

	// LogLevel must be one of the accepted severity levels.
	if !validLogLevels[strings.ToLower(cfg.LogLevel)] {
		slog.Error("invalid configuration", "field", "LogLevel", "value", cfg.LogLevel, "expected", "debug, info, warn, error")
		errs = append(errs, "LogLevel must be debug, info, warn, or error")
	}

	// LogFormat must be one of the accepted output formats.
	if !validLogFormats[strings.ToLower(cfg.LogFormat)] {
		slog.Error("invalid configuration", "field", "LogFormat", "value", cfg.LogFormat, "expected", "json, text")
		errs = append(errs, "LogFormat must be json or text")
	}

	// AuthRecordPath parent directory: best-effort check, warn only.
	parentDir := filepath.Dir(cfg.AuthRecordPath)
	if _, err := os.Stat(parentDir); err != nil {
		slog.Warn("AuthRecordPath parent directory does not exist", "field", "AuthRecordPath", "path", parentDir)
	}

	// CacheName must be non-empty and at most 128 characters.
	if cfg.CacheName == "" {
		slog.Error("invalid configuration", "field", "CacheName", "value", cfg.CacheName, "expected", "non-empty string, max 128 chars")
		errs = append(errs, "CacheName must be non-empty")
	} else if len(cfg.CacheName) > 128 {
		slog.Error("invalid configuration", "field", "CacheName", "value", cfg.CacheName, "expected", "max 128 chars")
		errs = append(errs, "CacheName must be at most 128 characters")
	}

	// MaxRetries must be between 0 and 10 inclusive.
	if cfg.MaxRetries < 0 || cfg.MaxRetries > 10 {
		slog.Error("invalid configuration", "field", "MaxRetries", "value", cfg.MaxRetries, "expected", "0-10")
		errs = append(errs, "MaxRetries must be between 0 and 10")
	}

	// RetryBackoffMS must be between 100 and 30000 inclusive.
	if cfg.RetryBackoffMS < 100 || cfg.RetryBackoffMS > 30000 {
		slog.Error("invalid configuration", "field", "RetryBackoffMS", "value", cfg.RetryBackoffMS, "expected", "100-30000")
		errs = append(errs, "RetryBackoffMS must be between 100 and 30000")
	}

	// RequestTimeout must be between 1 and 300 seconds inclusive.
	timeoutSec := int(cfg.RequestTimeout.Seconds())
	if timeoutSec < 1 || timeoutSec > 300 {
		slog.Error("invalid configuration", "field", "RequestTimeout", "value", timeoutSec, "expected", "1-300 seconds")
		errs = append(errs, "RequestTimeout must be between 1 and 300 seconds")
	}

	// LogFile parent directory: best-effort check, warn only (same pattern as AuthRecordPath).
	if cfg.LogFile != "" {
		logFileDir := filepath.Dir(cfg.LogFile)
		if _, err := os.Stat(logFileDir); err != nil {
			slog.Warn("LogFile parent directory does not exist", "field", "LogFile", "path", logFileDir)
		}
	}

	// AuthMethod must be one of the accepted authentication methods.
	if !validAuthMethods[strings.ToLower(cfg.AuthMethod)] {
		slog.Error("invalid configuration", "field", "AuthMethod", "value", cfg.AuthMethod, "expected", "auth_code, browser, device_code")
		errs = append(errs, "AuthMethod must be auth_code, browser, or device_code")
	}

	// TokenStorage must be one of the accepted token storage backend values.
	if !validTokenStorageValues[strings.ToLower(cfg.TokenStorage)] {
		slog.Error("invalid configuration", "field", "TokenStorage", "value", cfg.TokenStorage, "expected", "auto, keychain, file")
		errs = append(errs, "TokenStorage must be auto, keychain, or file")
	}

	// OUTLOOK_MCP_READ_ONLY: warn if set to a non-standard value.
	// Only "", "true", and "false" (case-insensitive) are recognized.
	// Any other value is silently treated as false by LoadConfig, so
	// a warning helps operators catch unintentional misconfiguration.
	if rawReadOnly := os.Getenv("OUTLOOK_MCP_READ_ONLY"); rawReadOnly != "" {
		lower := strings.ToLower(rawReadOnly)
		if lower != "true" && lower != "false" {
			slog.Warn("non-standard OUTLOOK_MCP_READ_ONLY value, treating as false",
				"field", "ReadOnly", "value", rawReadOnly, "expected", "true or false")
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("configuration validation failed: %s", strings.Join(errs, "; "))
	}
	return nil
}
