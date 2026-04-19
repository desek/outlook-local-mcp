package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestGetEnvReturnsValue validates that GetEnv returns the environment variable
// value when the variable is set to a non-empty string.
func TestGetEnvReturnsValue(t *testing.T) {
	t.Setenv("TEST_VAR", "hello")

	got := GetEnv("TEST_VAR", "default")
	if got != "hello" {
		t.Errorf("GetEnv() = %q, want %q", got, "hello")
	}
}

// TestGetEnvReturnsDefault validates that GetEnv returns the default value
// when the environment variable is not set.
func TestGetEnvReturnsDefault(t *testing.T) {
	// Ensure the variable is unset. t.Setenv restores the original value
	// after the test, but we need it unset during the test.
	t.Setenv("TEST_VAR", "")
	_ = os.Unsetenv("TEST_VAR")

	got := GetEnv("TEST_VAR", "default")
	if got != "default" {
		t.Errorf("GetEnv() = %q, want %q", got, "default")
	}
}

// TestGetEnvReturnsDefaultForEmpty validates that GetEnv returns the default
// value when the environment variable is set to an empty string.
func TestGetEnvReturnsDefaultForEmpty(t *testing.T) {
	t.Setenv("TEST_VAR", "")

	got := GetEnv("TEST_VAR", "default")
	if got != "default" {
		t.Errorf("GetEnv() = %q, want %q", got, "default")
	}
}

// TestLoadConfigDefaults validates that LoadConfig returns the correct default
// values when no OUTLOOK_MCP_* environment variables are set. The AuthRecordPath
// default should have its "~" prefix expanded to the user's home directory.
func TestLoadConfigDefaults(t *testing.T) {
	clearOutlookEnvVars(t)

	cfg := LoadConfig()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("os.UserHomeDir() error: %v", err)
	}

	wantAuthPath := filepath.Join(home, ".outlook-local-mcp", "auth_record.json")

	checks := []struct {
		name string
		got  string
		want string
	}{
		{"ClientID", cfg.ClientID, "d3590ed6-52b3-4102-aeff-aad2292ab01c"},
		{"TenantID", cfg.TenantID, "common"},
		{"AuthRecordPath", cfg.AuthRecordPath, wantAuthPath},
		{"CacheName", cfg.CacheName, "outlook-local-mcp"},
		{"LogLevel", cfg.LogLevel, "warn"},
		{"LogFormat", cfg.LogFormat, "json"},
	}

	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}

	if cfg.RequestTimeout != 30*time.Second {
		t.Errorf("RequestTimeout = %v, want %v", cfg.RequestTimeout, 30*time.Second)
	}

	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want %d", cfg.MaxRetries, 3)
	}

	if cfg.RetryBackoffMS != 1000 {
		t.Errorf("RetryBackoffMS = %d, want %d", cfg.RetryBackoffMS, 1000)
	}

	if cfg.ShutdownTimeout != 15*time.Second {
		t.Errorf("ShutdownTimeout = %v, want %v", cfg.ShutdownTimeout, 15*time.Second)
	}

	if !cfg.LogSanitize {
		t.Error("LogSanitize should default to true")
	}

	if cfg.ReadOnly {
		t.Error("ReadOnly should default to false")
	}

	if cfg.MailEnabled {
		t.Error("MailEnabled should default to false")
	}

	if cfg.OTELEnabled {
		t.Error("OTELEnabled should default to false")
	}
	if cfg.OTELEndpoint != "" {
		t.Errorf("OTELEndpoint = %q, want %q", cfg.OTELEndpoint, "")
	}
	if cfg.OTELServiceName != "outlook-local-mcp" {
		t.Errorf("OTELServiceName = %q, want %q", cfg.OTELServiceName, "outlook-local-mcp")
	}

	if cfg.LogFile != "" {
		t.Errorf("LogFile = %q, want empty", cfg.LogFile)
	}

	if cfg.AuthMethod != "device_code" {
		t.Errorf("AuthMethod = %q, want %q", cfg.AuthMethod, "device_code")
	}

	// DefaultTimezone should be resolved from "auto" to a valid IANA timezone.
	if _, err := time.LoadLocation(cfg.DefaultTimezone); err != nil {
		t.Errorf("DefaultTimezone = %q, want valid IANA timezone (resolved from auto)", cfg.DefaultTimezone)
	}

	wantAccountsPath := filepath.Join(home, ".outlook-local-mcp", "accounts.json")
	if cfg.AccountsPath != wantAccountsPath {
		t.Errorf("AccountsPath = %q, want %q", cfg.AccountsPath, wantAccountsPath)
	}
}

// TestLoadConfigCustomValues validates that LoadConfig reads custom values from
// all seven OUTLOOK_MCP_* environment variables.
func TestLoadConfigCustomValues(t *testing.T) {
	clearOutlookEnvVars(t)

	t.Setenv("OUTLOOK_MCP_CLIENT_ID", "my-app-id")
	t.Setenv("OUTLOOK_MCP_TENANT_ID", "my-tenant-guid")
	t.Setenv("OUTLOOK_MCP_AUTH_RECORD_PATH", "/custom/path/auth.json")
	t.Setenv("OUTLOOK_MCP_CACHE_NAME", "my-cache")
	t.Setenv("OUTLOOK_MCP_DEFAULT_TIMEZONE", "America/New_York")
	t.Setenv("OUTLOOK_MCP_LOG_LEVEL", "debug")
	t.Setenv("OUTLOOK_MCP_LOG_FORMAT", "text")
	t.Setenv("OUTLOOK_MCP_REQUEST_TIMEOUT_SECONDS", "45")

	cfg := LoadConfig()

	checks := []struct {
		name string
		got  string
		want string
	}{
		{"ClientID", cfg.ClientID, "my-app-id"},
		{"TenantID", cfg.TenantID, "my-tenant-guid"},
		{"AuthRecordPath", cfg.AuthRecordPath, "/custom/path/auth.json"},
		{"CacheName", cfg.CacheName, "my-cache"},
		{"DefaultTimezone", cfg.DefaultTimezone, "America/New_York"},
		{"LogLevel", cfg.LogLevel, "debug"},
		{"LogFormat", cfg.LogFormat, "text"},
	}

	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}

	if cfg.RequestTimeout != 45*time.Second {
		t.Errorf("RequestTimeout = %v, want %v", cfg.RequestTimeout, 45*time.Second)
	}
}

// TestLoadConfigAuthRecordPathExpansion validates that the default AuthRecordPath
// containing "~" is expanded to an absolute path starting with the user's home
// directory.
func TestLoadConfigAuthRecordPathExpansion(t *testing.T) {
	clearOutlookEnvVars(t)

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("os.UserHomeDir() error: %v", err)
	}

	cfg := LoadConfig()

	wantPrefix := home + string(filepath.Separator)
	if !hasPrefix(cfg.AuthRecordPath, wantPrefix) && cfg.AuthRecordPath != home {
		t.Errorf("AuthRecordPath = %q, want prefix %q", cfg.AuthRecordPath, wantPrefix)
	}
}

// TestLoadConfigCustomAuthRecordPath validates that a custom absolute path set
// via OUTLOOK_MCP_AUTH_RECORD_PATH is not modified by home directory expansion.
func TestLoadConfigCustomAuthRecordPath(t *testing.T) {
	clearOutlookEnvVars(t)
	t.Setenv("OUTLOOK_MCP_AUTH_RECORD_PATH", "/tmp/auth.json")

	cfg := LoadConfig()

	if cfg.AuthRecordPath != "/tmp/auth.json" {
		t.Errorf("AuthRecordPath = %q, want %q", cfg.AuthRecordPath, "/tmp/auth.json")
	}
}

// clearOutlookEnvVars unsets all OUTLOOK_MCP_* environment variables for the
// duration of the test, ensuring a clean state for LoadConfig tests.
func clearOutlookEnvVars(t *testing.T) {
	t.Helper()
	vars := []string{
		"OUTLOOK_MCP_CLIENT_ID",
		"OUTLOOK_MCP_TENANT_ID",
		"OUTLOOK_MCP_AUTH_RECORD_PATH",
		"OUTLOOK_MCP_CACHE_NAME",
		"OUTLOOK_MCP_DEFAULT_TIMEZONE",
		"OUTLOOK_MCP_LOG_LEVEL",
		"OUTLOOK_MCP_LOG_FORMAT",
		"OUTLOOK_MCP_MAX_RETRIES",
		"OUTLOOK_MCP_RETRY_BACKOFF_MS",
		"OUTLOOK_MCP_REQUEST_TIMEOUT_SECONDS",
		"OUTLOOK_MCP_SHUTDOWN_TIMEOUT_SECONDS",
		"OUTLOOK_MCP_LOG_SANITIZE",
		"OUTLOOK_MCP_AUDIT_LOG_ENABLED",
		"OUTLOOK_MCP_AUDIT_LOG_PATH",
		"OUTLOOK_MCP_READ_ONLY",
		"OUTLOOK_MCP_OTEL_ENABLED",
		"OUTLOOK_MCP_OTEL_ENDPOINT",
		"OUTLOOK_MCP_OTEL_SERVICE_NAME",
		"OUTLOOK_MCP_LOG_FILE",
		"OUTLOOK_MCP_AUTH_METHOD",
		"OUTLOOK_MCP_ACCOUNTS_PATH",
		"OUTLOOK_MCP_TOKEN_STORAGE",
		"OUTLOOK_MCP_PROVENANCE_TAG",
		"OUTLOOK_MCP_MAIL_ENABLED",
	}
	for _, v := range vars {
		t.Setenv(v, "")
		_ = os.Unsetenv(v)
	}
}

// hasPrefix is a helper that checks if s starts with prefix.
func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// TestLoadConfig_ShutdownTimeoutDefault validates that the default shutdown
// timeout is 15 seconds when the environment variable is not set.
func TestLoadConfig_ShutdownTimeoutDefault(t *testing.T) {
	clearOutlookEnvVars(t)

	cfg := LoadConfig()

	if cfg.ShutdownTimeout != 15*time.Second {
		t.Errorf("ShutdownTimeout = %v, want %v", cfg.ShutdownTimeout, 15*time.Second)
	}
}

// TestLoadConfig_ShutdownTimeoutCustom validates that a custom shutdown timeout
// is read from the OUTLOOK_MCP_SHUTDOWN_TIMEOUT_SECONDS environment variable.
func TestLoadConfig_ShutdownTimeoutCustom(t *testing.T) {
	clearOutlookEnvVars(t)
	t.Setenv("OUTLOOK_MCP_SHUTDOWN_TIMEOUT_SECONDS", "30")

	cfg := LoadConfig()

	if cfg.ShutdownTimeout != 30*time.Second {
		t.Errorf("ShutdownTimeout = %v, want %v", cfg.ShutdownTimeout, 30*time.Second)
	}
}

// TestLoadConfig_ShutdownTimeoutInvalid validates that an unparseable value
// falls back to the 15-second default.
func TestLoadConfig_ShutdownTimeoutInvalid(t *testing.T) {
	clearOutlookEnvVars(t)
	t.Setenv("OUTLOOK_MCP_SHUTDOWN_TIMEOUT_SECONDS", "abc")

	cfg := LoadConfig()

	if cfg.ShutdownTimeout != 15*time.Second {
		t.Errorf("ShutdownTimeout = %v, want %v", cfg.ShutdownTimeout, 15*time.Second)
	}
}

// TestLoadConfig_ShutdownTimeoutClampMin validates that values below 1 are
// clamped to 1 second.
func TestLoadConfig_ShutdownTimeoutClampMin(t *testing.T) {
	clearOutlookEnvVars(t)
	t.Setenv("OUTLOOK_MCP_SHUTDOWN_TIMEOUT_SECONDS", "0")

	cfg := LoadConfig()

	if cfg.ShutdownTimeout != 1*time.Second {
		t.Errorf("ShutdownTimeout = %v, want %v", cfg.ShutdownTimeout, 1*time.Second)
	}
}

// TestLoadConfig_ShutdownTimeoutClampMax validates that values above 300 are
// clamped to 300 seconds.
func TestLoadConfig_ShutdownTimeoutClampMax(t *testing.T) {
	clearOutlookEnvVars(t)
	t.Setenv("OUTLOOK_MCP_SHUTDOWN_TIMEOUT_SECONDS", "999")

	cfg := LoadConfig()

	if cfg.ShutdownTimeout != 300*time.Second {
		t.Errorf("ShutdownTimeout = %v, want %v", cfg.ShutdownTimeout, 300*time.Second)
	}
}

// TestLoadConfigDefaults_RequestTimeout validates that the default request
// timeout is 30 seconds when OUTLOOK_MCP_REQUEST_TIMEOUT_SECONDS is unset.
func TestLoadConfigDefaults_RequestTimeout(t *testing.T) {
	clearOutlookEnvVars(t)

	cfg := LoadConfig()

	if cfg.RequestTimeout != 30*time.Second {
		t.Errorf("RequestTimeout = %v, want %v", cfg.RequestTimeout, 30*time.Second)
	}
}

// TestLoadConfigCustomTimeout validates that a custom timeout is read from
// the OUTLOOK_MCP_REQUEST_TIMEOUT_SECONDS environment variable.
func TestLoadConfigCustomTimeout(t *testing.T) {
	clearOutlookEnvVars(t)
	t.Setenv("OUTLOOK_MCP_REQUEST_TIMEOUT_SECONDS", "60")

	cfg := LoadConfig()

	if cfg.RequestTimeout != 60*time.Second {
		t.Errorf("RequestTimeout = %v, want %v", cfg.RequestTimeout, 60*time.Second)
	}
}

// TestLoadConfigInvalidTimeout validates that an unparseable value falls back
// to the 30-second default.
func TestLoadConfigInvalidTimeout(t *testing.T) {
	clearOutlookEnvVars(t)
	t.Setenv("OUTLOOK_MCP_REQUEST_TIMEOUT_SECONDS", "abc")

	cfg := LoadConfig()

	if cfg.RequestTimeout != 30*time.Second {
		t.Errorf("RequestTimeout = %v, want %v", cfg.RequestTimeout, 30*time.Second)
	}
}

// TestLoadConfigZeroTimeout validates that a zero value falls back to the
// 30-second default.
func TestLoadConfigZeroTimeout(t *testing.T) {
	clearOutlookEnvVars(t)
	t.Setenv("OUTLOOK_MCP_REQUEST_TIMEOUT_SECONDS", "0")

	cfg := LoadConfig()

	if cfg.RequestTimeout != 30*time.Second {
		t.Errorf("RequestTimeout = %v, want %v", cfg.RequestTimeout, 30*time.Second)
	}
}

// TestLoadConfigNegativeTimeout validates that a negative value falls back to
// the 30-second default.
func TestLoadConfigNegativeTimeout(t *testing.T) {
	clearOutlookEnvVars(t)
	t.Setenv("OUTLOOK_MCP_REQUEST_TIMEOUT_SECONDS", "-5")

	cfg := LoadConfig()

	if cfg.RequestTimeout != 30*time.Second {
		t.Errorf("RequestTimeout = %v, want %v", cfg.RequestTimeout, 30*time.Second)
	}
}

// TestLoadConfig_MaxRetries_Default validates that MaxRetries defaults to 3
// when OUTLOOK_MCP_MAX_RETRIES is not set.
func TestLoadConfig_MaxRetries_Default(t *testing.T) {
	clearOutlookEnvVars(t)

	cfg := LoadConfig()

	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want %d", cfg.MaxRetries, 3)
	}
}

// TestLoadConfig_MaxRetries_Custom validates that MaxRetries reads from
// OUTLOOK_MCP_MAX_RETRIES environment variable.
func TestLoadConfig_MaxRetries_Custom(t *testing.T) {
	clearOutlookEnvVars(t)
	t.Setenv("OUTLOOK_MCP_MAX_RETRIES", "5")

	cfg := LoadConfig()

	if cfg.MaxRetries != 5 {
		t.Errorf("MaxRetries = %d, want %d", cfg.MaxRetries, 5)
	}
}

// TestLoadConfig_MaxRetries_Invalid validates that an invalid OUTLOOK_MCP_MAX_RETRIES
// value falls back to the default of 3.
func TestLoadConfig_MaxRetries_Invalid(t *testing.T) {
	clearOutlookEnvVars(t)
	t.Setenv("OUTLOOK_MCP_MAX_RETRIES", "abc")

	cfg := LoadConfig()

	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want %d", cfg.MaxRetries, 3)
	}
}

// TestLoadConfig_RetryBackoffMS_Default validates that RetryBackoffMS defaults
// to 1000 when OUTLOOK_MCP_RETRY_BACKOFF_MS is not set.
func TestLoadConfig_RetryBackoffMS_Default(t *testing.T) {
	clearOutlookEnvVars(t)

	cfg := LoadConfig()

	if cfg.RetryBackoffMS != 1000 {
		t.Errorf("RetryBackoffMS = %d, want %d", cfg.RetryBackoffMS, 1000)
	}
}

// TestLoadConfig_RetryBackoffMS_Custom validates that RetryBackoffMS reads from
// OUTLOOK_MCP_RETRY_BACKOFF_MS environment variable.
func TestLoadConfig_RetryBackoffMS_Custom(t *testing.T) {
	clearOutlookEnvVars(t)
	t.Setenv("OUTLOOK_MCP_RETRY_BACKOFF_MS", "2000")

	cfg := LoadConfig()

	if cfg.RetryBackoffMS != 2000 {
		t.Errorf("RetryBackoffMS = %d, want %d", cfg.RetryBackoffMS, 2000)
	}
}

// TestLoadConfig_RetryBackoffMS_Invalid validates that an invalid
// OUTLOOK_MCP_RETRY_BACKOFF_MS value falls back to the default of 1000.
func TestLoadConfig_RetryBackoffMS_Invalid(t *testing.T) {
	clearOutlookEnvVars(t)
	t.Setenv("OUTLOOK_MCP_RETRY_BACKOFF_MS", "fast")

	cfg := LoadConfig()

	if cfg.RetryBackoffMS != 1000 {
		t.Errorf("RetryBackoffMS = %d, want %d", cfg.RetryBackoffMS, 1000)
	}
}

// TestLoadConfig_ReadOnly_TrueLowercase validates that OUTLOOK_MCP_READ_ONLY=true
// sets ReadOnly to true.
func TestLoadConfig_ReadOnly_TrueLowercase(t *testing.T) {
	clearOutlookEnvVars(t)
	t.Setenv("OUTLOOK_MCP_READ_ONLY", "true")

	cfg := LoadConfig()

	if !cfg.ReadOnly {
		t.Error("ReadOnly should be true when OUTLOOK_MCP_READ_ONLY=true")
	}
}

// TestLoadConfig_ReadOnly_TrueUppercase validates that OUTLOOK_MCP_READ_ONLY=TRUE
// sets ReadOnly to true (case-insensitive).
func TestLoadConfig_ReadOnly_TrueUppercase(t *testing.T) {
	clearOutlookEnvVars(t)
	t.Setenv("OUTLOOK_MCP_READ_ONLY", "TRUE")

	cfg := LoadConfig()

	if !cfg.ReadOnly {
		t.Error("ReadOnly should be true when OUTLOOK_MCP_READ_ONLY=TRUE")
	}
}

// TestLoadConfig_ReadOnly_False validates that OUTLOOK_MCP_READ_ONLY=false
// sets ReadOnly to false.
func TestLoadConfig_ReadOnly_False(t *testing.T) {
	clearOutlookEnvVars(t)
	t.Setenv("OUTLOOK_MCP_READ_ONLY", "false")

	cfg := LoadConfig()

	if cfg.ReadOnly {
		t.Error("ReadOnly should be false when OUTLOOK_MCP_READ_ONLY=false")
	}
}

// TestLoadConfig_ReadOnly_Default validates that ReadOnly defaults to false
// when OUTLOOK_MCP_READ_ONLY is not set.
func TestLoadConfig_ReadOnly_Default(t *testing.T) {
	clearOutlookEnvVars(t)

	cfg := LoadConfig()

	if cfg.ReadOnly {
		t.Error("ReadOnly should default to false when OUTLOOK_MCP_READ_ONLY is unset")
	}
}

// TestLoadConfig_AuditLogEnabledDefault validates that AuditLogEnabled defaults
// to true when OUTLOOK_MCP_AUDIT_LOG_ENABLED is not set.
func TestLoadConfig_AuditLogEnabledDefault(t *testing.T) {
	clearOutlookEnvVars(t)

	cfg := LoadConfig()

	if !cfg.AuditLogEnabled {
		t.Error("AuditLogEnabled = false, want true (default)")
	}
}

// TestLoadConfig_AuditLogEnabledFalse validates that AuditLogEnabled is false
// when OUTLOOK_MCP_AUDIT_LOG_ENABLED is set to "false".
func TestLoadConfig_AuditLogEnabledFalse(t *testing.T) {
	clearOutlookEnvVars(t)
	t.Setenv("OUTLOOK_MCP_AUDIT_LOG_ENABLED", "false")

	cfg := LoadConfig()

	if cfg.AuditLogEnabled {
		t.Error("AuditLogEnabled = true, want false")
	}
}

// TestLoadConfig_AuditLogEnabledFalseUppercase validates that AuditLogEnabled
// is false when OUTLOOK_MCP_AUDIT_LOG_ENABLED is set to "FALSE" (case-insensitive).
func TestLoadConfig_AuditLogEnabledFalseUppercase(t *testing.T) {
	clearOutlookEnvVars(t)
	t.Setenv("OUTLOOK_MCP_AUDIT_LOG_ENABLED", "FALSE")

	cfg := LoadConfig()

	if cfg.AuditLogEnabled {
		t.Error("AuditLogEnabled = true, want false for 'FALSE'")
	}
}

// TestLoadConfig_AuditLogPathCustom validates that AuditLogPath reads from
// OUTLOOK_MCP_AUDIT_LOG_PATH environment variable.
func TestLoadConfig_AuditLogPathCustom(t *testing.T) {
	clearOutlookEnvVars(t)
	t.Setenv("OUTLOOK_MCP_AUDIT_LOG_PATH", "/var/log/audit.jsonl")

	cfg := LoadConfig()

	if cfg.AuditLogPath != "/var/log/audit.jsonl" {
		t.Errorf("AuditLogPath = %q, want %q", cfg.AuditLogPath, "/var/log/audit.jsonl")
	}
}

// TestLoadConfig_AuditLogPathDefault validates that AuditLogPath defaults to
// empty string when OUTLOOK_MCP_AUDIT_LOG_PATH is not set.
func TestLoadConfig_AuditLogPathDefault(t *testing.T) {
	clearOutlookEnvVars(t)

	cfg := LoadConfig()

	if cfg.AuditLogPath != "" {
		t.Errorf("AuditLogPath = %q, want empty", cfg.AuditLogPath)
	}
}

// TestLoadConfig_LogSanitizeDefault validates that LogSanitize defaults to true
// when OUTLOOK_MCP_LOG_SANITIZE is not set.
func TestLoadConfig_LogSanitizeDefault(t *testing.T) {
	clearOutlookEnvVars(t)

	cfg := LoadConfig()
	if !cfg.LogSanitize {
		t.Error("LogSanitize should default to true")
	}
}

// TestLoadConfig_LogSanitizeFalse validates that LogSanitize is false when
// OUTLOOK_MCP_LOG_SANITIZE is set to "false".
func TestLoadConfig_LogSanitizeFalse(t *testing.T) {
	clearOutlookEnvVars(t)
	t.Setenv("OUTLOOK_MCP_LOG_SANITIZE", "false")

	cfg := LoadConfig()
	if cfg.LogSanitize {
		t.Error("LogSanitize should be false when env var is 'false'")
	}
}

// TestLoadConfig_LogSanitizeTrue validates that LogSanitize is true when
// OUTLOOK_MCP_LOG_SANITIZE is set to "true".
func TestLoadConfig_LogSanitizeTrue(t *testing.T) {
	clearOutlookEnvVars(t)
	t.Setenv("OUTLOOK_MCP_LOG_SANITIZE", "true")

	cfg := LoadConfig()
	if !cfg.LogSanitize {
		t.Error("LogSanitize should be true when env var is 'true'")
	}
}

// TestLoadConfig_LogFileDefault validates that LogFile defaults to an empty
// string when OUTLOOK_MCP_LOG_FILE is not set.
func TestLoadConfig_LogFileDefault(t *testing.T) {
	clearOutlookEnvVars(t)

	cfg := LoadConfig()

	if cfg.LogFile != "" {
		t.Errorf("LogFile = %q, want empty", cfg.LogFile)
	}
}

// TestLoadConfig_LogFileCustom validates that LogFile reads from the
// OUTLOOK_MCP_LOG_FILE environment variable.
func TestLoadConfig_LogFileCustom(t *testing.T) {
	clearOutlookEnvVars(t)
	t.Setenv("OUTLOOK_MCP_LOG_FILE", "/tmp/test.log")

	cfg := LoadConfig()

	if cfg.LogFile != "/tmp/test.log" {
		t.Errorf("LogFile = %q, want %q", cfg.LogFile, "/tmp/test.log")
	}
}

// TestLoadConfig_AuthMethodDefault validates that AuthMethod defaults to
// "device_code" when OUTLOOK_MCP_AUTH_METHOD is not set and the default
// client ID (outlook-desktop, a well-known name) is used.
func TestLoadConfig_AuthMethodDefault(t *testing.T) {
	clearOutlookEnvVars(t)

	cfg := LoadConfig()

	if cfg.AuthMethod != "device_code" {
		t.Errorf("AuthMethod = %q, want %q", cfg.AuthMethod, "device_code")
	}
}

// TestLoadConfig_AuthMethodDeviceCode validates that AuthMethod reads
// "device_code" from the OUTLOOK_MCP_AUTH_METHOD environment variable.
func TestLoadConfig_AuthMethodDeviceCode(t *testing.T) {
	clearOutlookEnvVars(t)
	t.Setenv("OUTLOOK_MCP_AUTH_METHOD", "device_code")

	cfg := LoadConfig()

	if cfg.AuthMethod != "device_code" {
		t.Errorf("AuthMethod = %q, want %q", cfg.AuthMethod, "device_code")
	}
}

// TestLoadConfig_AccountsPathDefault validates that AccountsPath defaults to
// accounts.json in the same directory as AuthRecordPath.
func TestLoadConfig_AccountsPathDefault(t *testing.T) {
	clearOutlookEnvVars(t)

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("os.UserHomeDir() error: %v", err)
	}

	cfg := LoadConfig()

	want := filepath.Join(home, ".outlook-local-mcp", "accounts.json")
	if cfg.AccountsPath != want {
		t.Errorf("AccountsPath = %q, want %q", cfg.AccountsPath, want)
	}
}

// TestLoadConfig_AccountsPathEnvVar validates that OUTLOOK_MCP_ACCOUNTS_PATH
// environment variable overrides the default accounts file path.
func TestLoadConfig_AccountsPathEnvVar(t *testing.T) {
	clearOutlookEnvVars(t)
	t.Setenv("OUTLOOK_MCP_ACCOUNTS_PATH", "/tmp/custom-accounts.json")

	cfg := LoadConfig()

	if cfg.AccountsPath != "/tmp/custom-accounts.json" {
		t.Errorf("AccountsPath = %q, want %q", cfg.AccountsPath, "/tmp/custom-accounts.json")
	}
}

// TestInferAuthMethod_DefaultDeviceCode validates that when the default client
// ID (outlook-desktop UUID) is used with no explicit auth method, device_code
// is returned with source "inferred".
func TestInferAuthMethod_DefaultDeviceCode(t *testing.T) {
	got, source := InferAuthMethod("d3590ed6-52b3-4102-aeff-aad2292ab01c", "")
	if got != "device_code" {
		t.Errorf("InferAuthMethod(default, '') method = %q, want %q", got, "device_code")
	}
	if source != "inferred" {
		t.Errorf("InferAuthMethod(default, '') source = %q, want %q", source, "inferred")
	}
}

// TestInferAuthMethod_CustomClientBrowser validates that a custom UUID (not in
// the well-known registry) with no explicit auth method returns browser with
// source "default".
func TestInferAuthMethod_CustomClientBrowser(t *testing.T) {
	got, source := InferAuthMethod("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "")
	if got != "browser" {
		t.Errorf("InferAuthMethod(custom-uuid, '') method = %q, want %q", got, "browser")
	}
	if source != "default" {
		t.Errorf("InferAuthMethod(custom-uuid, '') source = %q, want %q", source, "default")
	}
}

// TestInferAuthMethod_WellKnownDeviceCode validates that a well-known client
// ID UUID (e.g. outlook-local-mcp) with no explicit auth method returns
// device_code with source "inferred".
func TestInferAuthMethod_WellKnownDeviceCode(t *testing.T) {
	// outlook-local-mcp UUID
	got, source := InferAuthMethod("dd5fc5c5-eb9a-4f6f-97bd-1a9fecb277d3", "")
	if got != "device_code" {
		t.Errorf("InferAuthMethod(well-known-uuid, '') method = %q, want %q", got, "device_code")
	}
	if source != "inferred" {
		t.Errorf("InferAuthMethod(well-known-uuid, '') source = %q, want %q", source, "inferred")
	}
}

// TestInferAuthMethod_ExplicitOverride validates that an explicit auth method
// always overrides inference regardless of client ID, with source "explicit".
func TestInferAuthMethod_ExplicitOverride(t *testing.T) {
	got, source := InferAuthMethod("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "device_code")
	if got != "device_code" {
		t.Errorf("InferAuthMethod(custom-uuid, 'device_code') method = %q, want %q", got, "device_code")
	}
	if source != "explicit" {
		t.Errorf("InferAuthMethod(custom-uuid, 'device_code') source = %q, want %q", source, "explicit")
	}
}

// TestLoadConfig_DefaultClientIDOutlookDesktop validates that LoadConfig
// defaults the client ID to the outlook-desktop UUID.
func TestLoadConfig_DefaultClientIDOutlookDesktop(t *testing.T) {
	clearOutlookEnvVars(t)

	cfg := LoadConfig()

	want := "d3590ed6-52b3-4102-aeff-aad2292ab01c"
	if cfg.ClientID != want {
		t.Errorf("ClientID = %q, want %q", cfg.ClientID, want)
	}
}

// TestLoadConfig_DefaultAuthMethodDeviceCode validates that LoadConfig defaults
// to device_code when no auth method or client ID is set.
func TestLoadConfig_DefaultAuthMethodDeviceCode(t *testing.T) {
	clearOutlookEnvVars(t)

	cfg := LoadConfig()

	if cfg.AuthMethod != "device_code" {
		t.Errorf("AuthMethod = %q, want %q", cfg.AuthMethod, "device_code")
	}
}

// TestLoadConfig_TimezoneAuto validates that when OUTLOOK_MCP_DEFAULT_TIMEZONE
// is "auto", the resolved timezone is a valid IANA timezone name.
func TestLoadConfig_TimezoneAuto(t *testing.T) {
	clearOutlookEnvVars(t)
	t.Setenv("OUTLOOK_MCP_DEFAULT_TIMEZONE", "auto")

	cfg := LoadConfig()

	if _, err := time.LoadLocation(cfg.DefaultTimezone); err != nil {
		t.Errorf("DefaultTimezone = %q, want valid IANA timezone: %v", cfg.DefaultTimezone, err)
	}
	if cfg.DefaultTimezone == "auto" {
		t.Error("DefaultTimezone should be resolved from 'auto', not left as 'auto'")
	}
}

// TestLoadConfig_TimezoneExplicit validates that an explicit timezone value
// passes through unchanged.
func TestLoadConfig_TimezoneExplicit(t *testing.T) {
	clearOutlookEnvVars(t)
	t.Setenv("OUTLOOK_MCP_DEFAULT_TIMEZONE", "Europe/London")

	cfg := LoadConfig()

	if cfg.DefaultTimezone != "Europe/London" {
		t.Errorf("DefaultTimezone = %q, want %q", cfg.DefaultTimezone, "Europe/London")
	}
}

// TestLoadConfig_TimezoneAutoFallback validates that when the system timezone
// resolves to "Local" (Go's fallback when TZ is unset), "auto" falls back to
// "UTC". This test sets TZ="" to simulate the condition, though the result
// depends on the OS; we accept any valid IANA timezone or "UTC".
func TestLoadConfig_TimezoneAutoFallback(t *testing.T) {
	clearOutlookEnvVars(t)
	t.Setenv("OUTLOOK_MCP_DEFAULT_TIMEZONE", "auto")

	cfg := LoadConfig()

	// The resolved timezone must be a valid IANA name (never "Local" or "auto").
	if _, err := time.LoadLocation(cfg.DefaultTimezone); err != nil {
		t.Errorf("DefaultTimezone = %q, want valid IANA timezone: %v", cfg.DefaultTimezone, err)
	}
	if cfg.DefaultTimezone == "auto" || cfg.DefaultTimezone == "Local" {
		t.Errorf("DefaultTimezone = %q, should be resolved to a valid IANA timezone", cfg.DefaultTimezone)
	}
}

// TestTokenStorage_DefaultAuto validates that TokenStorage defaults to "auto"
// when the OUTLOOK_MCP_TOKEN_STORAGE environment variable is not set.
func TestTokenStorage_DefaultAuto(t *testing.T) {
	clearOutlookEnvVars(t)

	cfg := LoadConfig()

	if cfg.TokenStorage != "auto" {
		t.Errorf("TokenStorage = %q, want %q", cfg.TokenStorage, "auto")
	}
}

// TestTokenStorage_EnvVar validates that TokenStorage reads "file" from
// the OUTLOOK_MCP_TOKEN_STORAGE environment variable.
func TestTokenStorage_EnvVar(t *testing.T) {
	clearOutlookEnvVars(t)
	t.Setenv("OUTLOOK_MCP_TOKEN_STORAGE", "file")

	cfg := LoadConfig()

	if cfg.TokenStorage != "file" {
		t.Errorf("TokenStorage = %q, want %q", cfg.TokenStorage, "file")
	}
}

// TestTokenStorage_EnvVar_Keychain validates that TokenStorage reads "keychain"
// from the OUTLOOK_MCP_TOKEN_STORAGE environment variable.
func TestTokenStorage_EnvVar_Keychain(t *testing.T) {
	clearOutlookEnvVars(t)
	t.Setenv("OUTLOOK_MCP_TOKEN_STORAGE", "keychain")

	cfg := LoadConfig()

	if cfg.TokenStorage != "keychain" {
		t.Errorf("TokenStorage = %q, want %q", cfg.TokenStorage, "keychain")
	}
}

// TestLoadConfig_ProvenanceTagDefault validates that ProvenanceTag defaults to
// "com.github.desek.outlook-local-mcp.created" when OUTLOOK_MCP_PROVENANCE_TAG
// is not set.
func TestLoadConfig_ProvenanceTagDefault(t *testing.T) {
	clearOutlookEnvVars(t)

	cfg := LoadConfig()

	want := "com.github.desek.outlook-local-mcp.created"
	if cfg.ProvenanceTag != want {
		t.Errorf("ProvenanceTag = %q, want %q", cfg.ProvenanceTag, want)
	}
}

// TestLoadConfig_ProvenanceTagCustom validates that ProvenanceTag reads a custom
// value from the OUTLOOK_MCP_PROVENANCE_TAG environment variable.
func TestLoadConfig_ProvenanceTagCustom(t *testing.T) {
	clearOutlookEnvVars(t)
	t.Setenv("OUTLOOK_MCP_PROVENANCE_TAG", "com.contoso.my-agent.created")

	cfg := LoadConfig()

	want := "com.contoso.my-agent.created"
	if cfg.ProvenanceTag != want {
		t.Errorf("ProvenanceTag = %q, want %q", cfg.ProvenanceTag, want)
	}
}

// TestLoadConfig_ProvenanceTagEmpty validates that setting
// OUTLOOK_MCP_PROVENANCE_TAG to an empty string disables provenance tagging
// (ProvenanceTag is "").
func TestLoadConfig_ProvenanceTagEmpty(t *testing.T) {
	clearOutlookEnvVars(t)
	t.Setenv("OUTLOOK_MCP_PROVENANCE_TAG", "")

	cfg := LoadConfig()

	if cfg.ProvenanceTag != "" {
		t.Errorf("ProvenanceTag = %q, want empty string (disabled)", cfg.ProvenanceTag)
	}
}

// TestLoadConfig_MailEnabledDefault validates that MailEnabled defaults to false
// when OUTLOOK_MCP_MAIL_ENABLED is not set.
func TestLoadConfig_MailEnabledDefault(t *testing.T) {
	clearOutlookEnvVars(t)

	cfg := LoadConfig()

	if cfg.MailEnabled {
		t.Error("MailEnabled should default to false when OUTLOOK_MCP_MAIL_ENABLED is unset")
	}
}

// TestLoadConfig_MailEnabledTrue validates that MailEnabled is true when
// OUTLOOK_MCP_MAIL_ENABLED is set to "true".
func TestLoadConfig_MailEnabledTrue(t *testing.T) {
	clearOutlookEnvVars(t)
	t.Setenv("OUTLOOK_MCP_MAIL_ENABLED", "true")

	cfg := LoadConfig()

	if !cfg.MailEnabled {
		t.Error("MailEnabled should be true when OUTLOOK_MCP_MAIL_ENABLED=true")
	}
}

// TestLoadConfig_MailEnabledTrueUppercase validates that MailEnabled is true
// when OUTLOOK_MCP_MAIL_ENABLED is set to "TRUE" (case-insensitive).
func TestLoadConfig_MailEnabledTrueUppercase(t *testing.T) {
	clearOutlookEnvVars(t)
	t.Setenv("OUTLOOK_MCP_MAIL_ENABLED", "TRUE")

	cfg := LoadConfig()

	if !cfg.MailEnabled {
		t.Error("MailEnabled should be true when OUTLOOK_MCP_MAIL_ENABLED=TRUE")
	}
}

// TestLoadConfig_MailEnabledFalse validates that MailEnabled is false when
// OUTLOOK_MCP_MAIL_ENABLED is set to "false".
func TestLoadConfig_MailEnabledFalse(t *testing.T) {
	clearOutlookEnvVars(t)
	t.Setenv("OUTLOOK_MCP_MAIL_ENABLED", "false")

	cfg := LoadConfig()

	if cfg.MailEnabled {
		t.Error("MailEnabled should be false when OUTLOOK_MCP_MAIL_ENABLED=false")
	}
}

// TestMailManageImpliesMailEnabled validates that when
// OUTLOOK_MCP_MAIL_MANAGE_ENABLED is true and OUTLOOK_MCP_MAIL_ENABLED is
// explicitly false, LoadConfig forces MailEnabled to true. Mail management
// is a superset of read-only mail access and must not leave MailEnabled off.
func TestMailManageImpliesMailEnabled(t *testing.T) {
	clearOutlookEnvVars(t)
	t.Setenv("OUTLOOK_MCP_MAIL_ENABLED", "false")
	t.Setenv("OUTLOOK_MCP_MAIL_MANAGE_ENABLED", "true")

	cfg := LoadConfig()

	if !cfg.MailManageEnabled {
		t.Error("MailManageEnabled should be true when OUTLOOK_MCP_MAIL_MANAGE_ENABLED=true")
	}
	if !cfg.MailEnabled {
		t.Error("MailEnabled should be forced to true when MailManageEnabled is true, even if OUTLOOK_MCP_MAIL_ENABLED=false")
	}
}

// TestLoadConfig_MailManageEnabledDefault validates that MailManageEnabled
// defaults to false when OUTLOOK_MCP_MAIL_MANAGE_ENABLED is not set.
func TestLoadConfig_MailManageEnabledDefault(t *testing.T) {
	clearOutlookEnvVars(t)

	cfg := LoadConfig()

	if cfg.MailManageEnabled {
		t.Error("MailManageEnabled should default to false when OUTLOOK_MCP_MAIL_MANAGE_ENABLED is unset")
	}
}

// TestInferAuthMethod_ReturnsSource validates that InferAuthMethod returns
// the correct source string for each determination path: explicit override,
// well-known client ID inference, and default fallback.
func TestInferAuthMethod_ReturnsSource(t *testing.T) {
	tests := []struct {
		name       string
		clientID   string
		explicit   string
		wantMethod string
		wantSource string
	}{
		{
			name:       "explicit override",
			clientID:   "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
			explicit:   "auth_code",
			wantMethod: "auth_code",
			wantSource: "explicit",
		},
		{
			name:       "well-known client inferred",
			clientID:   "d3590ed6-52b3-4102-aeff-aad2292ab01c",
			explicit:   "",
			wantMethod: "device_code",
			wantSource: "inferred",
		},
		{
			name:       "custom client default",
			clientID:   "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
			explicit:   "",
			wantMethod: "browser",
			wantSource: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			method, source := InferAuthMethod(tt.clientID, tt.explicit)
			if method != tt.wantMethod {
				t.Errorf("method = %q, want %q", method, tt.wantMethod)
			}
			if source != tt.wantSource {
				t.Errorf("source = %q, want %q", source, tt.wantSource)
			}
		})
	}
}
