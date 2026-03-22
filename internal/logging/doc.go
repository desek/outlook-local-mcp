// Package logging provides structured logging initialization, log
// sanitization, and optional file output for the Outlook Calendar MCP Server.
// It configures the process-wide default slog logger, routing all output to
// os.Stderr to preserve os.Stdout for the MCP JSON-RPC transport. When file
// logging is enabled via OUTLOOK_MCP_LOG_FILE, a MultiHandler fans out each
// log record to both stderr and the specified file. When sanitization is
// enabled, a custom slog.Handler wrapper automatically masks PII (email
// addresses, credential values, event body content) in all log output.
// CloseLogFile must be called during shutdown to flush and close the file
// handle when file logging is active.
package logging
