// Package config — this file declares the canonical inventory of every
// environment variable the loader reads, so the surface manifest and any other
// consumer can enumerate the configuration surface instead of re-deriving it
// from scattered string literals (CR-0073 Phase 1).
//
// The env-name constants below are the single spelling of each variable name.
// LoadConfig and ValidateConfig reference these constants rather than inline
// string literals, so a variable cannot be read without being enumerated here
// (FR-9). The Inventory slice pairs each name with its default and a one-line
// description, and is the source the generated manifest consumes (FR-8).
//
// @agents-index: declarative inventory of OUTLOOK_MCP_ environment variables.
package config

// Environment variable name constants. These are the only place each
// OUTLOOK_MCP_ name is spelled; the loader and validator reference them so that
// a read implies an inventory entry.
const (
	EnvClientID               = "OUTLOOK_MCP_CLIENT_ID"
	EnvTenantID               = "OUTLOOK_MCP_TENANT_ID"
	EnvAuthRecordPath         = "OUTLOOK_MCP_AUTH_RECORD_PATH"
	EnvCacheName              = "OUTLOOK_MCP_CACHE_NAME"
	EnvDefaultTimezone        = "OUTLOOK_MCP_DEFAULT_TIMEZONE"
	EnvLogLevel               = "OUTLOOK_MCP_LOG_LEVEL"
	EnvLogFormat              = "OUTLOOK_MCP_LOG_FORMAT"
	EnvMaxRetries             = "OUTLOOK_MCP_MAX_RETRIES"
	EnvRetryBackoffMS         = "OUTLOOK_MCP_RETRY_BACKOFF_MS"
	EnvRequestTimeoutSeconds  = "OUTLOOK_MCP_REQUEST_TIMEOUT_SECONDS"
	EnvShutdownTimeoutSeconds = "OUTLOOK_MCP_SHUTDOWN_TIMEOUT_SECONDS"
	EnvLogSanitize            = "OUTLOOK_MCP_LOG_SANITIZE"
	EnvAuditLogEnabled        = "OUTLOOK_MCP_AUDIT_LOG_ENABLED"
	EnvAuditLogPath           = "OUTLOOK_MCP_AUDIT_LOG_PATH"
	EnvReadOnly               = "OUTLOOK_MCP_READ_ONLY"
	EnvOTELEnabled            = "OUTLOOK_MCP_OTEL_ENABLED"
	EnvOTELEndpoint           = "OUTLOOK_MCP_OTEL_ENDPOINT"
	EnvOTELServiceName        = "OUTLOOK_MCP_OTEL_SERVICE_NAME"
	EnvLogFile                = "OUTLOOK_MCP_LOG_FILE"
	EnvAuthMethod             = "OUTLOOK_MCP_AUTH_METHOD"
	EnvAccountsPath           = "OUTLOOK_MCP_ACCOUNTS_PATH"
	EnvTokenStorage           = "OUTLOOK_MCP_TOKEN_STORAGE"
	EnvProvenanceTag          = "OUTLOOK_MCP_PROVENANCE_TAG"
	EnvMailEnabled            = "OUTLOOK_MCP_MAIL_ENABLED"
	EnvMailManageEnabled      = "OUTLOOK_MCP_MAIL_MANAGE_ENABLED"
	EnvMaxAttachmentSizeBytes = "OUTLOOK_MCP_MAX_ATTACHMENT_SIZE_BYTES"
)

// Variable describes one environment variable the loader reads: its full
// environment variable name, its default value where one applies (the empty
// string when the variable defaults to unset or a derived value), and a
// one-line description of its effect.
type Variable struct {
	// Name is the full OUTLOOK_MCP_ environment variable name.
	Name string

	// Default is the documented default value, or the empty string when the
	// variable has no fixed default (unset, or derived from another value).
	Default string

	// Description is a single-line explanation of what the variable controls.
	Description string
}

// inventory is the ordered, canonical list of every environment variable the
// loader reads. Order is fixed so that any consumer serializing it produces a
// stable result.
var inventory = []Variable{
	{EnvClientID, "outlook-desktop", "OAuth client (application) ID, or a well-known client name, used for authentication."},
	{EnvTenantID, "common", "Entra ID tenant: common, organizations, consumers, or a specific tenant GUID."},
	{EnvAuthRecordPath, "~/.outlook-local-mcp/auth_record.json", "Filesystem path where the non-secret authentication record is persisted."},
	{EnvCacheName, "outlook-local-mcp", "Partition name for the OS-native persistent token cache."},
	{EnvDefaultTimezone, "auto", "IANA timezone for calendar operations when the caller omits one; 'auto' detects the host timezone."},
	{EnvLogLevel, "warn", "Minimum log severity: debug, info, warn, or error."},
	{EnvLogFormat, "json", "Structured log output format: json or text."},
	{EnvMaxRetries, "3", "Maximum retry attempts for transient Graph API failures (range 0-10)."},
	{EnvRetryBackoffMS, "1000", "Initial exponential-backoff duration in milliseconds for retryable Graph errors (range 100-30000)."},
	{EnvRequestTimeoutSeconds, "30", "Maximum duration in seconds for a single Graph API request (range 1-300)."},
	{EnvShutdownTimeoutSeconds, "15", "Maximum duration in seconds to wait for in-flight requests after a shutdown signal (range 1-300)."},
	{EnvLogSanitize, "true", "Mask PII such as email addresses and body content in log output."},
	{EnvAuditLogEnabled, "true", "Emit a structured audit entry for every tool invocation."},
	{EnvAuditLogPath, "", "Filesystem path for the audit log file; empty writes audit entries to stderr."},
	{EnvReadOnly, "false", "Disable all write operations, registering only read verbs."},
	{EnvOTELEnabled, "false", "Enable OpenTelemetry metrics and tracing."},
	{EnvOTELEndpoint, "", "OTLP gRPC endpoint for exporting telemetry; empty resolves to localhost:4317 at startup."},
	{EnvOTELServiceName, "outlook-local-mcp", "service.name resource attribute for OpenTelemetry telemetry."},
	{EnvLogFile, "", "Optional filesystem path for log file output in addition to stderr."},
	{EnvAuthMethod, "", "Authentication method: device_code, browser, or auth_code; empty infers from the client ID."},
	{EnvAccountsPath, "", "Filesystem path for the accounts.json registry; empty derives it from the auth record directory."},
	{EnvTokenStorage, "auto", "Token storage backend: auto, keychain, or file."},
	{EnvProvenanceTag, "com.github.desek.outlook-local-mcp.created", "Extended-property tag name for MCP-created events; empty disables provenance tagging."},
	{EnvMailEnabled, "false", "Enable read-only mail access and request the Mail.Read scope."},
	{EnvMailManageEnabled, "false", "Enable draft management and request the Mail.ReadWrite scope; implies MAIL_ENABLED."},
	{EnvMaxAttachmentSizeBytes, "10485760", "Maximum attachment size in bytes returned by get_attachment (default 10 MB)."},
}

// Inventory returns the canonical, ordered inventory of every environment
// variable the loader reads. The returned slice is a copy so callers cannot
// mutate the package-level source.
//
// Returns the ordered []Variable inventory.
func Inventory() []Variable {
	out := make([]Variable, len(inventory))
	copy(out, inventory)
	return out
}
