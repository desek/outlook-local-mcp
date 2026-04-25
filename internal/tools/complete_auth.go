// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the complete_auth MCP tool, a fallback for MCP clients
// that do not support elicitation. When the auth_code authentication method
// is active and the middleware cannot elicit the redirect URL inline, it
// instructs the user to call this tool with the nativeclient redirect URL
// from the browser's address bar. The tool extracts the authorization code
// and exchanges it for tokens via the AuthCodeFlow interface.
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/logging"
	"github.com/mark3labs/mcp-go/mcp"
)

// NewCompleteAuthTool creates the MCP tool definition for complete_auth.
// The tool accepts a required "redirect_url" string parameter (the full
// nativeclient redirect URL from the browser) and an optional "account"
// string parameter for multi-account scenarios (Phase 5).
//
// Returns the configured mcp.Tool ready for registration with server.AddTool.
func NewCompleteAuthTool() mcp.Tool {
	return mcp.NewTool("complete_auth",
		mcp.WithTitleAnnotation("Complete Authentication"),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithDescription(
			"Complete the authorization code flow by exchanging the redirect URL for tokens. "+
				"After signing in via the browser, copy the full URL from the address bar "+
				"(starting with https://login.microsoftonline.com/common/oauth2/nativeclient) "+
				"and pass it as the redirect_url parameter.",
		),
		mcp.WithString("redirect_url",
			mcp.Required(),
			mcp.Description("The full URL from the browser's address bar after signing in."),
		),
		mcp.WithString("account",
			mcp.Description("Account label (or UPN) that was provided to account_add when initiating this auth_code authentication. This ties the returned redirect URL back to the correct pending credential; do not assume a default account."),
		),
	)
}

// HandleCompleteAuth creates a tool handler that completes the auth code flow
// by exchanging the redirect URL for tokens. When an "account" parameter is
// provided, the handler looks up the account in the registry and uses its
// credential; otherwise it falls back to the default credential.
//
// Parameters:
//   - cred: the default account's Authenticator that must also implement
//     auth.AuthCodeFlow.
//   - authRecordPath: the filesystem path for persisting account metadata
//     after successful token exchange (used for the default account).
//   - registry: the account registry for looking up named accounts. May be
//     nil when multi-account is not active.
//   - scopes: OAuth scopes to request during token exchange (from auth.Scopes(cfg)).
//
// Returns a tool handler function compatible with the MCP server's AddTool
// method. The handler returns a JSON success message via mcp.NewToolResultText,
// or an error result if the redirect URL is missing, the credential does not
// support auth_code, or the token exchange fails.
//
// Side effects: exchanges the authorization code for tokens via MSAL,
// persists account metadata to disk on success.
func HandleCompleteAuth(cred auth.Authenticator, authRecordPath string, registry *auth.AccountRegistry, scopes []string) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := logging.Logger(ctx)

		redirectURL, err := request.RequireString("redirect_url")
		if err != nil {
			return mcp.NewToolResultError("missing required parameter: redirect_url"), nil
		}

		if redirectURL == "" {
			return mcp.NewToolResultError("redirect_url must not be empty"), nil
		}

		logger.Debug("tool called")

		// Resolve the target credential and auth record path.
		targetCred, targetPath := cred, authRecordPath
		if accountLabel := request.GetString("account", ""); accountLabel != "" {
			if registry == nil {
				return mcp.NewToolResultError(fmt.Sprintf("account %q specified but no account registry available", accountLabel)), nil
			}
			entry, found := registry.Get(accountLabel)
			if !found {
				return mcp.NewToolResultError(fmt.Sprintf("account %q not found", accountLabel)), nil
			}
			targetCred = entry.Authenticator
			targetPath = entry.AuthRecordPath
			logger = logger.With("account", accountLabel)
		}

		// Type-assert to AuthCodeFlow interface.
		acf, ok := targetCred.(auth.AuthCodeFlow)
		if !ok {
			return mcp.NewToolResultError(
				"Internal error: credential does not support the auth_code flow. " +
					"Check that OUTLOOK_MCP_AUTH_METHOD is set to 'auth_code'."), nil
		}

		// Exchange the authorization code for tokens.
		if exchangeErr := acf.ExchangeCode(ctx, redirectURL, scopes); exchangeErr != nil {
			logger.Error("code exchange failed", "error", exchangeErr.Error())
			return mcp.NewToolResultError(fmt.Sprintf(
				"Failed to exchange authorization code: %v\n\n"+
					"Troubleshooting:\n"+
					"1. Make sure you copied the entire URL from the browser's address bar.\n"+
					"2. The URL should start with https://login.microsoftonline.com/common/oauth2/nativeclient\n"+
					"3. Authorization codes expire after ~10 minutes. Try authenticating again.",
				exchangeErr)), nil
		}

		// Persist account metadata to disk.
		if acc, isACC := targetCred.(*auth.AuthCodeCredential); isACC {
			if persistErr := acc.PersistAccount(targetPath); persistErr != nil {
				logger.Warn("failed to persist auth code account", "error", persistErr)
			}
		}

		result := map[string]any{
			"success": true,
			"message": "Authentication completed successfully. You can now retry your original request.",
		}

		data, err := json.Marshal(result)
		if err != nil {
			return mcp.NewToolResultError("failed to serialize response"), nil
		}

		logger.Info("auth code exchange completed successfully")
		return mcp.NewToolResultText(string(data)), nil
	}
}
