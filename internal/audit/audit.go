package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// auditMaxParamLen is the maximum length for a parameter value in an audit
// entry before it is truncated with a "...[truncated]" suffix.
const auditMaxParamLen = 200

// auditEmailPattern matches email addresses in strings for masking in audit
// entries. This is separate from the logging package's emailRegex because the
// audit subsystem uses a different masking strategy (first character only).
var auditEmailPattern = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)

// Module-level audit state, initialized by InitAuditLog and used by EmitAuditLog.
var (
	auditWriter  io.Writer
	auditMu      sync.Mutex
	auditEnabled bool
)

// AuditEntry represents a single audit log record emitted after each tool
// invocation. It captures the tool name, operation classification, sanitized
// parameters, outcome, timing, and relevant resource identifiers. The Audit
// field is always true and serves as a discriminator when audit entries are
// interleaved with slog output on stderr.
type AuditEntry struct {
	// Audit is always true, used to distinguish audit entries from slog output.
	Audit bool `json:"audit"`

	// Timestamp is the RFC 3339 time when the audit entry was created.
	Timestamp string `json:"timestamp"`

	// ToolName is the name of the MCP tool that was invoked.
	ToolName string `json:"tool_name"`

	// OperationType classifies the tool as "read", "write", or "delete".
	OperationType string `json:"operation_type"`

	// Parameters contains sanitized key-value pairs from the tool request.
	Parameters map[string]string `json:"parameters"`

	// Outcome is "success" or "error" based on the handler result.
	Outcome string `json:"outcome"`

	// DurationMs is the wall-clock duration of the handler invocation in
	// milliseconds.
	DurationMs int64 `json:"duration_ms"`

	// ErrorMessage contains the error description when Outcome is "error".
	ErrorMessage string `json:"error_message,omitempty"`

	// EventID is the event identifier extracted from parameters, if present.
	EventID string `json:"event_id,omitempty"`

	// CalendarID is the calendar identifier extracted from parameters, if present.
	CalendarID string `json:"calendar_id,omitempty"`
}

// MaskAuditEmail masks an email address by keeping only the first character of
// the local part and replacing the rest with "***". Non-email strings (those
// without an "@") are returned unchanged. Empty strings are returned as-is.
// This is separate from MaskEmail in the logging package which preserves 2
// characters.
//
// Parameters:
//   - email: the email address string to mask.
//
// Returns the masked email (e.g., "a***@example.com") or the original string
// if it does not contain an "@".
func MaskAuditEmail(email string) string {
	if email == "" {
		return ""
	}
	at := strings.Index(email, "@")
	if at < 0 {
		return email
	}
	return string(email[0]) + "***" + email[at:]
}

// TruncateAuditString truncates s to maxLen characters and appends
// "...[truncated]" if s exceeds maxLen. Strings at or below maxLen are
// returned unchanged.
//
// Parameters:
//   - s: the string to potentially truncate.
//   - maxLen: the maximum allowed length before truncation.
//
// Returns the original string if within limits, or the truncated version
// with the suffix.
func TruncateAuditString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "...[truncated]"
}

// SanitizeAuditParams converts tool parameters into a sanitized map[string]string
// suitable for audit logging. The "body" parameter is excluded entirely.
// Email addresses found in values are masked using MaskAuditEmail. String values
// exceeding auditMaxParamLen are truncated.
//
// Parameters:
//   - params: the raw tool parameters from CallToolRequest.GetArguments().
//
// Returns a map of sanitized string key-value pairs.
func SanitizeAuditParams(params map[string]any) map[string]string {
	result := make(map[string]string, len(params))
	for k, v := range params {
		if k == "body" {
			continue
		}
		s := fmt.Sprintf("%v", v)
		// Mask any email addresses found in the value.
		s = auditEmailPattern.ReplaceAllStringFunc(s, MaskAuditEmail)
		s = TruncateAuditString(s, auditMaxParamLen)
		result[k] = s
	}
	return result
}

// InitAuditLog initializes the audit logging subsystem. When enabled is true
// and path is non-empty, it opens the file in append-only mode. If the file
// cannot be opened, it logs an error via slog and falls back to os.Stderr.
// When path is empty, audit entries are written to os.Stderr. When enabled is
// false, the subsystem is disabled and EmitAuditLog becomes a no-op.
//
// Parameters:
//   - enabled: whether audit logging is active.
//   - path: filesystem path for the audit log file, or empty for stderr.
//
// Side effects: opens a file handle (kept open for the server lifetime),
// sets module-level audit state. Logs an slog.Error on file open failure.
func InitAuditLog(enabled bool, path string) {
	auditEnabled = enabled
	if !enabled {
		return
	}
	if path == "" {
		auditWriter = os.Stderr
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		slog.Error("audit log file open failed", "path", path, "error", err)
		auditWriter = os.Stderr
		return
	}
	auditWriter = f
}

// EmitAuditLog serializes the given AuditEntry as a single JSON line and writes
// it to the configured audit destination. The write is protected by a mutex for
// concurrency safety. If the writer implements io.Flusher (e.g., *os.File via
// Sync), the entry is flushed immediately for crash safety. When audit logging
// is disabled, this function is a no-op.
//
// Parameters:
//   - entry: the audit entry to serialize and write.
//
// Side effects: writes to the audit writer, acquires and releases auditMu.
// Logs an slog.Error if JSON marshaling or writing fails.
func EmitAuditLog(entry AuditEntry) {
	if !auditEnabled {
		return
	}
	data, err := json.Marshal(entry)
	if err != nil {
		slog.Error("audit log marshal failed", "error", err)
		return
	}
	data = append(data, '\n')

	auditMu.Lock()
	defer auditMu.Unlock()

	if _, err := auditWriter.Write(data); err != nil {
		slog.Error("audit log write failed", "error", err)
		return
	}
	// Flush for crash safety: *os.File implements Sync().
	if f, ok := auditWriter.(*os.File); ok {
		_ = f.Sync()
	}
}

// AuditWrap wraps a tool handler function with audit logging. It records the
// start time, calls the inner handler, computes the duration, classifies the
// outcome (success or error), extracts event_id and calendar_id from parameters,
// sanitizes parameters, and emits an audit entry via EmitAuditLog. The handler's
// return values are passed through unchanged.
//
// Parameters:
//   - toolName: the MCP tool name for the audit entry.
//   - opType: the operation type classification ("read", "write", or "delete").
//   - handler: the original tool handler function to wrap.
//
// Returns a new tool handler function that emits an audit entry after each
// invocation.
func AuditWrap(toolName, opType string, handler server.ToolHandlerFunc) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		start := time.Now()
		result, err := handler(ctx, request)
		durationMs := time.Since(start).Milliseconds()

		// Classify outcome.
		outcome := "success"
		var errMsg string
		if err != nil {
			outcome = "error"
			errMsg = err.Error()
		} else if result != nil && result.IsError {
			outcome = "error"
			// Extract error message from the result content.
			if len(result.Content) > 0 {
				if tc, ok := result.Content[0].(mcp.TextContent); ok {
					errMsg = tc.Text
				}
			}
		}

		// Extract and sanitize parameters.
		args := request.GetArguments()
		params := SanitizeAuditParams(args)

		// Extract resource identifiers.
		eventID := ""
		if v, ok := args["event_id"].(string); ok {
			eventID = v
		}
		calendarID := ""
		if v, ok := args["calendar_id"].(string); ok {
			calendarID = v
		}

		entry := AuditEntry{
			Audit:         true,
			Timestamp:     start.UTC().Format(time.RFC3339),
			ToolName:      toolName,
			OperationType: opType,
			Parameters:    params,
			Outcome:       outcome,
			DurationMs:    durationMs,
			ErrorMessage:  errMsg,
			EventID:       eventID,
			CalendarID:    calendarID,
		}
		EmitAuditLog(entry)

		return result, err
	}
}
