// Package server provides MCP server wiring for the Outlook Calendar MCP
// Server. It contains the RegisterTools function that serves as the single
// registration point for all tool handlers, the ReadOnlyGuard middleware for
// write-protection in read-only mode, and the AwaitShutdownSignal function
// for graceful OS signal handling and process shutdown.
package server
