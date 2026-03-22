// Package audit provides the audit logging subsystem for the Outlook Calendar
// MCP Server. It defines the AuditEntry struct, sanitization helpers (email
// masking, body exclusion, string truncation), the audit writer with
// mutex-protected file/stderr output, and the AuditWrap middleware that
// automatically emits a structured audit entry after each tool invocation. The
// audit log operates independently of the operational slog log level, producing
// a JSON Lines append-only trail suitable for compliance and security review.
package audit
