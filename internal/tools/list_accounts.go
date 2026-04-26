// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the list_accounts MCP tool, which returns all registered
// accounts in the account registry. For each authenticated account, the email
// address is lazily fetched from the Microsoft Graph /me endpoint and cached
// on the AccountEntry for use in subsequent tool confirmations.
package tools

import (
	"context"
	"encoding/json"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/logging"
	"github.com/mark3labs/mcp-go/mcp"
)

// NewListAccountsTool creates the MCP tool definition for list_accounts.
// The tool takes no parameters and is annotated as read-only since it only
// reads from the in-memory account registry without side effects.
//
// Returns the configured mcp.Tool ready for registration with server.AddTool.
func NewListAccountsTool() mcp.Tool {
	return mcp.NewTool("account_list",
		mcp.WithDescription(
			"List every registered account — authenticated and disconnected — with label, User Principal Name (UPN), authentication state, and auth_method. "+
				"This output is the authoritative source for account selection decisions: always call account_list before acting when account intent is ambiguous, and consider every entry (including disconnected ones) — disconnected accounts MUST NOT be ignored. "+
				"When reporting results to the user, present each account's UPN alongside its label and state so the user can confirm which mailbox each label maps to. "+
				"When a disconnected account is surfaced, proactively offer account_login (to reconnect) or account_remove (to discard) rather than silently falling back to another account.",
		),
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
// from the account registry. For each authenticated account, it calls EnsureEmail
// to lazily populate the email address from the Microsoft Graph /me endpoint.
// Each account is serialized as a JSON object with "label", "authenticated",
// and "email" fields.
//
// Parameters:
//   - registry: the account registry to query for registered accounts.
//
// Returns a tool handler function compatible with the MCP server's AddTool
// method. The handler returns a JSON array of account objects via
// mcp.NewToolResultText, or an error result if JSON serialization fails.
//
// Side effects: calls GET /me on the Graph API for each authenticated account
// whose email has not yet been fetched.
func HandleListAccounts(registry *auth.AccountRegistry) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := logging.Logger(ctx)
		logger.Debug("tool called")

		// Validate output mode.
		outputMode, err := ValidateOutputMode(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		entries := registry.List()
		results := make([]map[string]any, 0, len(entries))
		for _, entry := range entries {
			// Lazily fetch email for authenticated accounts.
			if entry.Client != nil {
				auth.EnsureEmail(ctx, entry)
			}
			results = append(results, map[string]any{
				"label":         entry.Label,
				"authenticated": entry.Authenticated,
				"email":         entry.Email,
				"auth_method":   entry.AuthMethod,
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
