package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration values for the Outlook Calendar MCP Server.
// Each field corresponds to an environment variable with a sensible default.
// The configuration is loaded once at startup via LoadConfig and passed to
// subsystem initializers (logging, authentication, Graph client, MCP server).
type Config struct {
	// ClientID is the OAuth 2.0 client (application) ID used for device code
	// authentication with Microsoft identity platform. Defaults to the
	// Microsoft Office first-party client ID.
	ClientID string

	// TenantID is the Entra ID tenant identifier. Accepts "common" (any account),
	// "organizations" (work/school only), "consumers" (personal only), or a
	// specific tenant GUID.
	TenantID string

	// AuthRecordPath is the filesystem path where the authentication record
	// (non-secret metadata from a previous authentication) is persisted. The
	// "~" prefix is expanded to the user's home directory at load time.
	AuthRecordPath string

	// CacheName is the partition name for the OS-native persistent token cache
	// (Keychain on macOS, libsecret on Linux, Credential Manager on Windows).
	CacheName string

	// DefaultTimezone is the IANA timezone name used for calendar operations
	// when the caller does not specify a timezone.
	DefaultTimezone string

	// LogLevel is the minimum severity level for log output.
	// Valid values: "debug", "info", "warn", "error".
	LogLevel string

	// LogFormat is the structured log output format.
	// Valid values: "json", "text".
	LogFormat string

	// MaxRetries is the maximum number of retry attempts for transient Graph
	// API failures (HTTP 429, 503, 504). Configurable via
	// OUTLOOK_MCP_MAX_RETRIES (default: 3, range: 0-10).
	MaxRetries int

	// RetryBackoffMS is the initial backoff duration in milliseconds for
	// exponential backoff on retryable Graph API errors. Configurable via
	// OUTLOOK_MCP_RETRY_BACKOFF_MS (default: 1000, range: 100-30000).
	RetryBackoffMS int

	// RequestTimeout is the maximum duration for a single Graph API request.
	// Applied via context.WithTimeout before each Graph API call. Configurable
	// via OUTLOOK_MCP_REQUEST_TIMEOUT_SECONDS (default: 30, range: 1-300).
	RequestTimeout time.Duration

	// ShutdownTimeout is the maximum duration to wait for in-flight requests
	// to complete after a shutdown signal is received. Configurable via
	// OUTLOOK_MCP_SHUTDOWN_TIMEOUT_SECONDS (default: 15, range: 1-300).
	ShutdownTimeout time.Duration

	// LogSanitize controls whether log output is sanitized to mask PII such
	// as email addresses, event body content, and credential values. When true
	// (the default), a sanitizingHandler wraps the underlying slog handler.
	// Configurable via OUTLOOK_MCP_LOG_SANITIZE (default: "true").
	LogSanitize bool

	// AuditLogEnabled controls whether the audit logging subsystem is active.
	// When true (the default), every tool invocation emits a structured JSON
	// audit entry. Configurable via OUTLOOK_MCP_AUDIT_LOG_ENABLED (default: "true").
	AuditLogEnabled bool

	// AuditLogPath is the filesystem path for the audit log file. When empty
	// (the default), audit entries are written to os.Stderr alongside operational
	// logs. Configurable via OUTLOOK_MCP_AUDIT_LOG_PATH.
	AuditLogPath string

	// ReadOnly controls whether write operations (create, update, delete, cancel)
	// are disabled. When true, only read tools are registered. Configurable via
	// OUTLOOK_MCP_READ_ONLY (default: "false").
	ReadOnly bool

	// OTELEnabled controls whether OpenTelemetry metrics and tracing are active.
	// When false (the default), noop providers are used with zero overhead.
	OTELEnabled bool

	// OTELEndpoint is the OTLP gRPC endpoint for exporting telemetry.
	// Defaults to "" which resolves to localhost:4317 at initialization time.
	OTELEndpoint string

	// OTELServiceName is the service.name resource attribute for OTEL telemetry.
	// Defaults to "outlook-local-mcp".
	OTELServiceName string

	// LogFile is the optional filesystem path for log file output. When set,
	// log records are written to both os.Stderr and the specified file.
	// When empty (the default), only stderr logging is active. Populated from
	// the OUTLOOK_MCP_LOG_FILE environment variable.
	LogFile string

	// AuthMethod controls which Azure Identity credential type is used for
	// authentication with Microsoft Graph API. Valid values are "device_code"
	// (DeviceCodeCredential, the default), "browser" (InteractiveBrowserCredential),
	// and "auth_code" (AuthCodeCredential). Populated from the
	// OUTLOOK_MCP_AUTH_METHOD environment variable. Device code auth displays
	// a code the user enters at a URL; browser auth opens the system browser
	// for OAuth login; auth code uses the manual authorization code flow with PKCE.
	AuthMethod string

	// AccountsPath is the filesystem path for the persistent accounts
	// configuration file (accounts.json). This file stores per-account identity
	// metadata (label, client_id, tenant_id, auth_method) for multi-account
	// support. Defaults to the same directory as AuthRecordPath. Configurable
	// via OUTLOOK_MCP_ACCOUNTS_PATH.
	AccountsPath string

	// TokenStorage controls which token storage backend is used for caching
	// OAuth tokens. Valid values are "auto" (try OS keychain first, fall back
	// to file-based), "keychain" (OS keychain only), and "file" (file-based
	// AES-256-GCM only). Populated from the OUTLOOK_MCP_TOKEN_STORAGE
	// environment variable. Defaults to "auto".
	TokenStorage string

	// MailEnabled controls whether read-only email access is active. When true,
	// the Mail.Read OAuth scope is requested during authentication and the mail
	// tools (list_mail_folders, list_messages, search_messages, get_message) are
	// registered. When false (the default), no mail tools are registered and
	// Mail.Read is not requested. Configurable via OUTLOOK_MCP_MAIL_ENABLED
	// (default: "false").
	MailEnabled bool

	// MailManageEnabled controls whether draft-centric mail management is active.
	// When true, the Mail.ReadWrite OAuth scope is requested (instead of Mail.Read)
	// so that the server can create, update, and delete drafts in addition to
	// reading mail. Enabling this option also implicitly enables MailEnabled:
	// LoadConfig forces MailEnabled to true whenever MailManageEnabled is true,
	// since management capabilities are a superset of read-only mail access.
	// Mail.Send is never requested; sending remains a user-only action. Configurable
	// via OUTLOOK_MCP_MAIL_MANAGE_ENABLED (default: "false").
	MailManageEnabled bool

	// ProvenanceTag is the name component of the single-value extended property
	// used to tag MCP-created calendar events. The full property ID is built by
	// combining a dedicated GUID with this tag name at startup. When set to an
	// empty string, provenance tagging is disabled entirely: no extended property
	// is stamped on create, no $expand is added to reads, and the created_by_mcp
	// filter is not available on search_events. Configurable via
	// OUTLOOK_MCP_PROVENANCE_TAG (default: "com.github.desek.outlook-local-mcp.created").
	ProvenanceTag string

	// Version is the application version string, injected by the binary entry
	// point after LoadConfig returns. Used by the status diagnostic tool to
	// report the running server version.
	Version string

	// TokenCacheBackend is the actual resolved token storage backend in use
	// at runtime. Set after token cache initialization in the binary entry
	// point. Valid values are "keychain" (OS keychain) or "file" (file-based
	// AES-256-GCM encrypted cache). This is a runtime-resolved value, not
	// read from an environment variable.
	TokenCacheBackend string

	// AuthMethodSource indicates how the AuthMethod value was determined.
	// "explicit" means the user set OUTLOOK_MCP_AUTH_METHOD; "inferred" means
	// the method was determined from a well-known client ID in WellKnownClientIDs;
	// "default" means the client ID did not match any well-known UUID and the
	// fallback method ("browser") was used.
	AuthMethodSource string
}

// GetEnv returns the value of the environment variable identified by key.
// If the variable is unset or set to an empty string, defaultValue is returned.
//
// Parameters:
//   - key: the environment variable name to look up.
//   - defaultValue: the fallback value when the variable is absent or empty.
//
// Returns the environment variable value, or defaultValue if absent or empty.
func GetEnv(key, defaultValue string) string {
	v := os.Getenv(key)
	if v == "" {
		return defaultValue
	}
	return v
}

// LoadConfig reads all configuration values from environment variables and
// returns a populated Config struct. Each environment variable has a default
// value that is used when the variable is unset or empty.
//
// The function expands a leading "~/" in AuthRecordPath to the current user's
// home directory. If os.UserHomeDir fails, the path is left unexpanded.
//
// Returns a fully populated Config struct ready for use by subsystem initializers.
//
// Side effects: reads environment variables and calls os.UserHomeDir.
func LoadConfig() Config {
	cfg := Config{
		ClientID:        ResolveClientID(GetEnv("OUTLOOK_MCP_CLIENT_ID", "outlook-desktop")),
		TenantID:        GetEnv("OUTLOOK_MCP_TENANT_ID", "common"),
		AuthRecordPath:  GetEnv("OUTLOOK_MCP_AUTH_RECORD_PATH", "~/.outlook-local-mcp/auth_record.json"),
		CacheName:       GetEnv("OUTLOOK_MCP_CACHE_NAME", "outlook-local-mcp"),
		DefaultTimezone: GetEnv("OUTLOOK_MCP_DEFAULT_TIMEZONE", "auto"),
		LogLevel:        GetEnv("OUTLOOK_MCP_LOG_LEVEL", "warn"),
		LogFormat:       GetEnv("OUTLOOK_MCP_LOG_FORMAT", "json"),
	}

	if strings.HasPrefix(cfg.AuthRecordPath, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			cfg.AuthRecordPath = filepath.Join(home, cfg.AuthRecordPath[2:])
		}
	}

	maxRetries, err := strconv.Atoi(GetEnv("OUTLOOK_MCP_MAX_RETRIES", "3"))
	if err != nil {
		slog.Warn("invalid OUTLOOK_MCP_MAX_RETRIES, using default", "value", GetEnv("OUTLOOK_MCP_MAX_RETRIES", "3"), "default", 3)
		maxRetries = 3
	}
	cfg.MaxRetries = maxRetries

	retryBackoffMS, err := strconv.Atoi(GetEnv("OUTLOOK_MCP_RETRY_BACKOFF_MS", "1000"))
	if err != nil {
		slog.Warn("invalid OUTLOOK_MCP_RETRY_BACKOFF_MS, using default", "value", GetEnv("OUTLOOK_MCP_RETRY_BACKOFF_MS", "1000"), "default", 1000)
		retryBackoffMS = 1000
	}
	cfg.RetryBackoffMS = retryBackoffMS

	timeoutStr := GetEnv("OUTLOOK_MCP_REQUEST_TIMEOUT_SECONDS", "30")
	timeoutSec, err := strconv.Atoi(timeoutStr)
	if err != nil || timeoutSec <= 0 {
		slog.Warn("invalid OUTLOOK_MCP_REQUEST_TIMEOUT_SECONDS, using default", "value", timeoutStr, "default", 30)
		timeoutSec = 30
	}
	cfg.RequestTimeout = time.Duration(timeoutSec) * time.Second

	cfg.LogSanitize = strings.ToLower(GetEnv("OUTLOOK_MCP_LOG_SANITIZE", "true")) != "false"

	shutdownStr := GetEnv("OUTLOOK_MCP_SHUTDOWN_TIMEOUT_SECONDS", "15")
	shutdownSec, err := strconv.Atoi(shutdownStr)
	if err != nil {
		slog.Warn("invalid OUTLOOK_MCP_SHUTDOWN_TIMEOUT_SECONDS, using default",
			"value", shutdownStr, "default", 15)
		shutdownSec = 15
	}
	if shutdownSec < 1 {
		slog.Warn("OUTLOOK_MCP_SHUTDOWN_TIMEOUT_SECONDS below minimum, clamping to 1",
			"value", shutdownSec)
		shutdownSec = 1
	} else if shutdownSec > 300 {
		slog.Warn("OUTLOOK_MCP_SHUTDOWN_TIMEOUT_SECONDS above maximum, clamping to 300",
			"value", shutdownSec)
		shutdownSec = 300
	}
	cfg.ShutdownTimeout = time.Duration(shutdownSec) * time.Second

	cfg.AuditLogEnabled = strings.ToLower(GetEnv("OUTLOOK_MCP_AUDIT_LOG_ENABLED", "true")) != "false"
	cfg.AuditLogPath = GetEnv("OUTLOOK_MCP_AUDIT_LOG_PATH", "")

	cfg.ReadOnly = strings.EqualFold(GetEnv("OUTLOOK_MCP_READ_ONLY", "false"), "true")

	cfg.OTELEnabled = strings.EqualFold(GetEnv("OUTLOOK_MCP_OTEL_ENABLED", "false"), "true")
	cfg.OTELEndpoint = GetEnv("OUTLOOK_MCP_OTEL_ENDPOINT", "")
	cfg.OTELServiceName = GetEnv("OUTLOOK_MCP_OTEL_SERVICE_NAME", "outlook-local-mcp")

	cfg.LogFile = GetEnv("OUTLOOK_MCP_LOG_FILE", "")

	// Resolve timezone auto-detection before validation.
	if cfg.DefaultTimezone == "auto" {
		cfg.DefaultTimezone = DetectTimezone()
	}

	explicitAuthMethod := GetEnv("OUTLOOK_MCP_AUTH_METHOD", "")
	cfg.AuthMethod, cfg.AuthMethodSource = InferAuthMethod(cfg.ClientID, explicitAuthMethod)

	cfg.AccountsPath = GetEnv("OUTLOOK_MCP_ACCOUNTS_PATH", "")
	if cfg.AccountsPath == "" {
		cfg.AccountsPath = filepath.Join(filepath.Dir(cfg.AuthRecordPath), "accounts.json")
	}

	cfg.TokenStorage = GetEnv("OUTLOOK_MCP_TOKEN_STORAGE", "auto")

	cfg.MailEnabled = strings.EqualFold(GetEnv("OUTLOOK_MCP_MAIL_ENABLED", "false"), "true")
	cfg.MailManageEnabled = strings.EqualFold(GetEnv("OUTLOOK_MCP_MAIL_MANAGE_ENABLED", "false"), "true")

	// Mail management is a superset of read-only mail access. Enabling
	// MailManageEnabled implicitly enables MailEnabled so that mail tool
	// registration and scope selection remain consistent.
	if cfg.MailManageEnabled {
		cfg.MailEnabled = true
	}

	// ProvenanceTag uses os.Getenv directly so that an explicit empty value
	// disables tagging, while an unset variable uses the default tag name.
	if v, ok := os.LookupEnv("OUTLOOK_MCP_PROVENANCE_TAG"); ok {
		cfg.ProvenanceTag = v
	} else {
		cfg.ProvenanceTag = "com.github.desek.outlook-local-mcp.created"
	}

	return cfg
}

// InferAuthMethod determines the effective authentication method and its source
// based on the resolved client ID and an explicitly provided auth method.
//
// When explicitAuthMethod is non-empty, it is returned as-is with source
// "explicit" (the user's explicit choice always wins). When the client ID
// matches a well-known UUID from the WellKnownClientIDs registry, "device_code"
// is returned with source "inferred" because these apps support device code
// flow without additional redirect URI configuration. When the client ID is a
// custom value (not in the well-known registry), "browser" is returned with
// source "default" because custom app registrations typically have
// http://localhost redirect URIs configured.
//
// Parameters:
//   - clientID: the resolved (UUID) client ID from configuration.
//   - explicitAuthMethod: the value of OUTLOOK_MCP_AUTH_METHOD, or "" if unset.
//
// Returns the effective auth method string and its source ("explicit",
// "inferred", or "default").
func InferAuthMethod(clientID, explicitAuthMethod string) (string, string) {
	if explicitAuthMethod != "" {
		return explicitAuthMethod, "explicit"
	}

	for _, uuid := range WellKnownClientIDs {
		if strings.EqualFold(clientID, uuid) {
			return "device_code", "inferred"
		}
	}

	return "browser", "default"
}
