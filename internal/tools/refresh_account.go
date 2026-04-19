// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the account_refresh MCP tool (CR-0056), which forces a
// token refresh for an authenticated account. It calls GetToken on the account's
// persisted credential with policy.TokenRequestOptions (EnableCAE=true) so the
// credential returns a fresh, CAE-enabled access token. The new token expiry
// time is surfaced in the tool response (FR-27) so the LLM and user can confirm
// the refresh took effect.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/config"
	"github.com/mark3labs/mcp-go/mcp"
)

// NewRefreshAccountTool creates the MCP tool definition for account_refresh.
// The tool accepts a required "label" parameter identifying a currently
// authenticated account. Annotations (CR-0052): ReadOnly=false (refreshes the
// cached token state), Destructive=false (no data is removed and the account
// remains connected), Idempotent=true (repeat calls on the same account with
// the same arguments consistently produce a refreshed access token with an
// updated expiry), OpenWorld=true (the handler calls Microsoft identity to
// obtain a new token).
//
// Returns the configured mcp.Tool ready for registration with server.AddTool.
func NewRefreshAccountTool() mcp.Tool {
	return mcp.NewTool("account_refresh",
		mcp.WithTitleAnnotation("Refresh Account Token"),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithDescription(
			"Force a token refresh for an authenticated account. "+
				"Calls GetToken on the account's credential with CAE enabled to obtain a new access token, "+
				"then returns the new token expiry time so the caller can confirm the refresh. "+
				"Useful when a permission change in Entra ID, a conditional access policy update, or suspected token staleness warrants an explicit refresh rather than waiting for silent renewal. "+
				"Proactively suggest this tool when the user mentions permission or role changes, reports stale authorization errors, or otherwise needs to confirm that a fresh token is in use — do not silently rely on the SDK's automatic refresh. "+
				"Never assume a default account: before calling, inspect account_list (or the current account landscape) and consider every registered account, including disconnected ones. "+
				"When intent is ambiguous, ask the user which account to refresh rather than guessing. "+
				"Returns an error if the named account is disconnected (use account_login first).",
		),
		mcp.WithString("label",
			mcp.Required(),
			mcp.Description("Label of the authenticated account whose token should be refreshed. Must already exist in the registry and be connected."),
		),
	)
}

// HandleRefreshAccount creates a tool handler that forces a token refresh for
// an authenticated account. The handler looks up the entry by label, rejects
// disconnected accounts, and calls GetToken on the entry's credential with a
// CAE-enabled policy.TokenRequestOptions so the underlying azidentity credential
// issues a fresh token. On success the new token expiry time (UTC, RFC3339) is
// included in the response payload.
//
// Parameters:
//   - registry: the account registry holding the target entry.
//   - cfg: server configuration supplying the OAuth scope set used for the
//     token request.
//
// Returns a tool handler function compatible with the MCP server's AddTool
// method. The handler returns a JSON success message with the label and new
// token expiry on success, or an error result if lookup fails, the account is
// disconnected, or the credential rejects the token request.
//
// Side effects: invokes the Microsoft identity platform via the credential's
// GetToken method. The underlying credential's token cache is updated with the
// refreshed token.
func HandleRefreshAccount(registry *auth.AccountRegistry, cfg config.Config) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	scopes := auth.Scopes(cfg)
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := slog.With("tool", "account_refresh")

		label, err := request.RequireString("label")
		if err != nil {
			return mcp.NewToolResultError("missing required parameter: label"), nil
		}

		logger.Debug("tool called", "label", label)

		entry, exists := registry.Get(label)
		if !exists {
			return mcp.NewToolResultError(fmt.Sprintf("account %q not found", label)), nil
		}
		if !entry.Authenticated {
			return mcp.NewToolResultError(fmt.Sprintf("Account %q is disconnected. Use account_login with label %q to re-authenticate before refreshing.", label, label)), nil
		}
		if entry.Credential == nil {
			return mcp.NewToolResultError(fmt.Sprintf("Account %q has no credential attached; use account_login to re-authenticate.", label)), nil
		}

		tok, err := entry.Credential.GetToken(ctx, policy.TokenRequestOptions{
			Scopes:    scopes,
			EnableCAE: true,
		})
		if err != nil {
			logger.Error("token refresh failed", "label", label, "error", err.Error())
			return mcp.NewToolResultError(fmt.Sprintf("failed to refresh token for account %q: %s", label, err.Error())), nil
		}

		expiry := tok.ExpiresOn.UTC().Format(time.RFC3339)

		result := map[string]any{
			"refreshed": true,
			"label":     label,
			"expiry":    expiry,
			"message":   fmt.Sprintf("Account %q token refreshed; new expiry %s.", label, expiry),
		}
		data, err := json.Marshal(result)
		if err != nil {
			return mcp.NewToolResultError("failed to serialize response"), nil
		}

		logger.Info("account token refreshed", "label", label, "expiry", expiry)
		return mcp.NewToolResultText(string(data)), nil
	}
}
