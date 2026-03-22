package server

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// ReadOnlyGuard wraps a tool handler with read-only mode protection. When
// readOnly is false, the handler is returned unchanged (zero overhead — same
// function reference). When readOnly is true, a closure is returned that logs
// the blocked operation and returns a tool error without calling the underlying
// handler.
//
// Parameters:
//   - toolName: the MCP tool name, included in the log message and error text.
//   - readOnly: whether read-only mode is active. When false, the handler passes
//     through unchanged.
//   - handler: the original tool handler function to protect.
//
// Returns the original handler when readOnly is false, or a blocking closure
// when readOnly is true.
//
// Side effects: when readOnly is true and the returned closure is invoked, it
// logs a warning via slog.Warn with fields "tool", "mode", and "action".
func ReadOnlyGuard(toolName string, readOnly bool, handler mcpserver.ToolHandlerFunc) mcpserver.ToolHandlerFunc {
	if !readOnly {
		return handler
	}
	return func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		slog.Warn("operation blocked in read-only mode",
			"tool", toolName,
			"mode", "read-only",
			"action", "blocked",
		)
		return mcp.NewToolResultError(
			fmt.Sprintf("operation blocked: %s is not allowed in read-only mode", toolName),
		), nil
	}
}
