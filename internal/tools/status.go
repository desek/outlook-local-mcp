// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the status MCP tool, a read-only diagnostic that returns
// a JSON summary of server health including version, timezone, per-account
// authentication state, uptime, and the full effective runtime configuration
// grouped into six categories. It makes no Graph API calls and completes
// within 100ms.
package tools

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/config"
	"github.com/mark3labs/mcp-go/mcp"
)

// NewStatusTool creates the MCP tool definition for the status diagnostic tool.
// The tool takes no required parameters and is annotated as read-only since it
// only inspects in-memory state without side effects or network calls.
//
// Returns the configured mcp.Tool ready for registration with server.AddTool.
func NewStatusTool() mcp.Tool {
	return mcp.NewTool("status",
		mcp.WithDescription("Return server health summary: version, timezone, account authentication state, and uptime. No parameters required. Does not call Graph API."),
		mcp.WithTitleAnnotation("Server Status"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
		mcp.WithString("output",
			mcp.Description("Output mode: 'text' (default) returns plain-text health summary, 'summary' returns compact JSON with full config, 'raw' returns full JSON with all config groups."),
			mcp.Enum("text", "summary", "raw"),
		),
	)
}

// statusResponse is the JSON structure returned by the status diagnostic tool.
type statusResponse struct {
	// Version is the server version string (e.g., "1.0.0" or "dev").
	Version string `json:"version"`

	// Timezone is the IANA timezone name configured for calendar operations.
	Timezone string `json:"timezone"`

	// Accounts lists each registered account with its label and auth state.
	Accounts []statusAccount `json:"accounts"`

	// ServerUptimeSeconds is the elapsed time since the server started, in
	// whole seconds.
	ServerUptimeSeconds int64 `json:"server_uptime_seconds"`

	// Config contains the server's effective runtime configuration grouped
	// into six categories: identity, logging, storage, graph_api, features,
	// and observability.
	Config statusConfig `json:"config"`
}

// statusAccount represents a single account's label and authentication state
// in the status response.
type statusAccount struct {
	// Label is the unique human-readable identifier for this account.
	Label string `json:"label"`

	// Authenticated indicates whether this account has a valid credential.
	Authenticated bool `json:"authenticated"`
}

// statusConfig contains all six configuration groups exposed by the status
// tool. Each group maps to a logical category of the server's runtime
// configuration.
type statusConfig struct {
	// Identity contains OAuth identity configuration: client ID, tenant,
	// auth method, and how the auth method was determined.
	Identity statusConfigIdentity `json:"identity"`

	// Logging contains log output configuration: level, format, file path,
	// sanitization, and audit settings.
	Logging statusConfigLogging `json:"logging"`

	// Storage contains token and data persistence configuration: storage
	// backend, cache paths, and partition name.
	Storage statusConfigStorage `json:"storage"`

	// GraphAPI contains Graph API client configuration: retry counts,
	// backoff, and timeout durations.
	GraphAPI statusConfigGraphAPI `json:"graph_api"`

	// Features contains feature flags and behavioral settings: read-only
	// mode, mail access, and provenance tagging.
	Features statusConfigFeatures `json:"features"`

	// Observability contains OpenTelemetry configuration: enabled state,
	// endpoint, and service name.
	Observability statusConfigObservability `json:"observability"`
}

// statusConfigIdentity holds the identity-related configuration fields
// exposed by the status tool.
type statusConfigIdentity struct {
	// ClientID is the OAuth 2.0 application (client) ID.
	ClientID string `json:"client_id"`

	// TenantID is the Entra ID tenant identifier.
	TenantID string `json:"tenant_id"`

	// AuthMethod is the effective authentication method (e.g., "device_code",
	// "browser", "auth_code").
	AuthMethod string `json:"auth_method"`

	// AuthMethodSource indicates how the auth method was determined:
	// "explicit" (env var set), "inferred" (well-known client ID), or
	// "default" (fallback).
	AuthMethodSource string `json:"auth_method_source"`
}

// statusConfigLogging holds the logging-related configuration fields
// exposed by the status tool.
type statusConfigLogging struct {
	// LogLevel is the minimum severity level for log output.
	LogLevel string `json:"log_level"`

	// LogFormat is the structured log output format ("json" or "text").
	LogFormat string `json:"log_format"`

	// LogFile is the optional filesystem path for log file output.
	LogFile string `json:"log_file"`

	// LogSanitize indicates whether log output is sanitized to mask PII.
	LogSanitize bool `json:"log_sanitize"`

	// AuditLogEnabled indicates whether the audit logging subsystem is active.
	AuditLogEnabled bool `json:"audit_log_enabled"`

	// AuditLogPath is the filesystem path for the audit log file.
	AuditLogPath string `json:"audit_log_path"`
}

// statusConfigStorage holds the storage-related configuration fields
// exposed by the status tool.
type statusConfigStorage struct {
	// TokenStorage is the configured token storage preference ("auto",
	// "keychain", or "file").
	TokenStorage string `json:"token_storage"`

	// TokenCacheBackend is the actual resolved backend ("keychain" or
	// "file"), which may differ from TokenStorage when "auto" is configured.
	TokenCacheBackend string `json:"token_cache_backend"`

	// AuthRecordPath is the filesystem path for the authentication record.
	AuthRecordPath string `json:"auth_record_path"`

	// AccountsPath is the filesystem path for the persistent accounts file.
	AccountsPath string `json:"accounts_path"`

	// CacheName is the partition name for the persistent token cache.
	CacheName string `json:"cache_name"`
}

// statusConfigGraphAPI holds the Graph API client configuration fields
// exposed by the status tool.
type statusConfigGraphAPI struct {
	// MaxRetries is the maximum number of retry attempts for transient
	// Graph API failures.
	MaxRetries int `json:"max_retries"`

	// RetryBackoffMS is the initial backoff duration in milliseconds.
	RetryBackoffMS int `json:"retry_backoff_ms"`

	// RequestTimeoutSeconds is the maximum duration for a single Graph API
	// request, in seconds.
	RequestTimeoutSeconds int `json:"request_timeout_seconds"`

	// ShutdownTimeoutSeconds is the maximum duration to wait for in-flight
	// requests on shutdown, in seconds.
	ShutdownTimeoutSeconds int `json:"shutdown_timeout_seconds"`
}

// statusConfigFeatures holds the feature flag configuration fields
// exposed by the status tool.
type statusConfigFeatures struct {
	// ReadOnly indicates whether write operations are disabled.
	ReadOnly bool `json:"read_only"`

	// MailEnabled indicates whether read-only email access is active.
	MailEnabled bool `json:"mail_enabled"`

	// ProvenanceTag is the extended property name for MCP-created events.
	ProvenanceTag string `json:"provenance_tag"`
}

// statusConfigObservability holds the OpenTelemetry configuration fields
// exposed by the status tool.
type statusConfigObservability struct {
	// OTELEnabled indicates whether OpenTelemetry is active.
	OTELEnabled bool `json:"otel_enabled"`

	// OTELEndpoint is the OTLP gRPC endpoint for exporting telemetry.
	OTELEndpoint string `json:"otel_endpoint"`

	// OTELServiceName is the service.name resource attribute for telemetry.
	OTELServiceName string `json:"otel_service_name"`
}

// HandleStatus creates a tool handler that returns a JSON summary of server
// health including version, timezone, accounts, uptime, and the full effective
// runtime configuration. The handler captures its dependencies at construction
// time and makes no network calls, ensuring it completes within 100ms.
//
// Parameters:
//   - cfg: the server configuration struct containing all runtime settings.
//   - registry: the account registry to query for account states.
//   - startTime: the time the server started, used to compute uptime.
//
// Returns a tool handler function compatible with the MCP server's AddTool
// method. The handler returns a JSON object via mcp.NewToolResultText, or an
// error result if JSON serialization fails.
//
// Side effects: none. The handler only reads from the registry and computes
// elapsed time.
func HandleStatus(cfg config.Config, registry *auth.AccountRegistry, startTime time.Time) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := slog.With("tool", "status")
		logger.Debug("tool called")

		// Validate output mode.
		outputMode, err := ValidateOutputMode(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		entries := registry.List()
		accounts := make([]statusAccount, 0, len(entries))
		for _, entry := range entries {
			accounts = append(accounts, statusAccount{
				Label:         entry.Label,
				Authenticated: entry.Authenticated,
			})
		}

		resp := statusResponse{
			Version:             cfg.Version,
			Timezone:            cfg.DefaultTimezone,
			Accounts:            accounts,
			ServerUptimeSeconds: int64(time.Since(startTime).Seconds()),
			Config: statusConfig{
				Identity: statusConfigIdentity{
					ClientID:         cfg.ClientID,
					TenantID:         cfg.TenantID,
					AuthMethod:       cfg.AuthMethod,
					AuthMethodSource: cfg.AuthMethodSource,
				},
				Logging: statusConfigLogging{
					LogLevel:        cfg.LogLevel,
					LogFormat:       cfg.LogFormat,
					LogFile:         cfg.LogFile,
					LogSanitize:     cfg.LogSanitize,
					AuditLogEnabled: cfg.AuditLogEnabled,
					AuditLogPath:    cfg.AuditLogPath,
				},
				Storage: statusConfigStorage{
					TokenStorage:      cfg.TokenStorage,
					TokenCacheBackend: cfg.TokenCacheBackend,
					AuthRecordPath:    cfg.AuthRecordPath,
					AccountsPath:      cfg.AccountsPath,
					CacheName:         cfg.CacheName,
				},
				GraphAPI: statusConfigGraphAPI{
					MaxRetries:             cfg.MaxRetries,
					RetryBackoffMS:         cfg.RetryBackoffMS,
					RequestTimeoutSeconds:  int(cfg.RequestTimeout.Seconds()),
					ShutdownTimeoutSeconds: int(cfg.ShutdownTimeout.Seconds()),
				},
				Features: statusConfigFeatures{
					ReadOnly:      cfg.ReadOnly,
					MailEnabled:   cfg.MailEnabled,
					ProvenanceTag: cfg.ProvenanceTag,
				},
				Observability: statusConfigObservability{
					OTELEnabled:     cfg.OTELEnabled,
					OTELEndpoint:    cfg.OTELEndpoint,
					OTELServiceName: cfg.OTELServiceName,
				},
			},
		}

		// Return text output when requested.
		if outputMode == "text" {
			logger.Info("tool completed", "accounts", len(accounts))
			return mcp.NewToolResultText(FormatStatusText(resp)), nil
		}

		data, err := json.Marshal(resp)
		if err != nil {
			return mcp.NewToolResultError("failed to serialize status"), nil
		}

		logger.Info("tool completed", "accounts", len(accounts))
		return mcp.NewToolResultText(string(data)), nil
	}
}
