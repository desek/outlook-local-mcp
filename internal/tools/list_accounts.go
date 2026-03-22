// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the list_accounts MCP tool, which returns all registered
// accounts in the account registry as a JSON array. Each entry includes the
// account label and authentication status.
package tools

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/mark3labs/mcp-go/mcp"
)

// NewListAccountsTool creates the MCP tool definition for list_accounts.
// The tool takes no parameters and is annotated as read-only since it only
// reads from the in-memory account registry without side effects.
//
// Returns the configured mcp.Tool ready for registration with server.AddTool.
func NewListAccountsTool() mcp.Tool {
	return mcp.NewTool("account_list",
		mcp.WithDescription("List all registered accounts and their authentication status."),
		mcp.WithTitleAnnotation("List Accounts"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
		mcp.WithString("output",
			mcp.Description("Output mode: 'text' (default) returns plain-text listing, 'summary' returns compact JSON, 'raw' returns full Graph API fields."),
			mcp.Enum("text", "summary", "raw"),
		),
	)
}

// HandleListAccounts creates a tool handler that lists all registered accounts
// from the account registry. Each account is serialized as a JSON object with
// "label" and "authenticated" fields.
//
// Parameters:
//   - registry: the account registry to query for registered accounts.
//
// Returns a tool handler function compatible with the MCP server's AddTool
// method. The handler returns a JSON array of account objects via
// mcp.NewToolResultText, or an error result if JSON serialization fails.
//
// Side effects: none. The handler only reads from the registry.
func HandleListAccounts(registry *auth.AccountRegistry) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := slog.With("tool", "account_list")
		logger.Debug("tool called")

		// Validate output mode.
		outputMode, err := ValidateOutputMode(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		entries := registry.List()
		results := make([]map[string]any, 0, len(entries))
		for _, entry := range entries {
			results = append(results, map[string]any{
				"label":         entry.Label,
				"authenticated": entry.Client != nil,
			})
		}

		// Return text output when requested.
		if outputMode == "text" {
			logger.Info("tool completed", "count", len(results))
			return mcp.NewToolResultText(FormatAccountsText(results)), nil
		}

		data, err := json.Marshal(results)
		if err != nil {
			return mcp.NewToolResultError("failed to serialize account list"), nil
		}

		logger.Info("tool completed", "count", len(results))
		return mcp.NewToolResultText(string(data)), nil
	}
}
