// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the account_logout MCP tool (CR-0056), which disconnects
// an authenticated account without removing it from the registry or
// accounts.json. The tool flips the entry's Authenticated flag to false,
// clears the in-memory credential/client handles, and clears the persisted
// token cache for the account's cache partition. The account remains visible
// in account_list and status as "disconnected" and can be re-authenticated
// via account_login.
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/logging"
	"github.com/mark3labs/mcp-go/mcp"
)

// NewLogoutAccountTool creates the MCP tool definition for account_logout.
// The tool accepts a required "label" parameter identifying a currently
// authenticated account. Annotations (CR-0052): ReadOnly=false (it mutates
// registry state), Destructive=false (configuration is preserved and logout
// is reversible via account_login), Idempotent=true (repeat calls on an
// already-disconnected account are rejected with the same deterministic
// error), OpenWorld=false (the handler is local-only: no Graph or identity
// API calls).
//
// Returns the configured mcp.Tool ready for registration with server.AddTool.
func NewLogoutAccountTool() mcp.Tool {
	return mcp.NewTool("account_logout",
		mcp.WithTitleAnnotation("Log Out Account"),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
		mcp.WithDescription(
			"Disconnect an authenticated account without removing it from the registry or accounts.json. "+
				"Clears the in-memory credential, Graph client, and authenticator, flips Authenticated to false, "+
				"and clears the cached token from the account's keychain/file cache partition. "+
				"The account remains visible in account_list and status as 'disconnected' and can be re-connected later via account_login. "+
				"Proactively suggest this tool whenever the user indicates they want to sign out, step away from a shared machine, or otherwise pause an account without losing its configuration — do not silently fall back to account_remove, which is irreversible. "+
				"Never assume a default account: before calling, inspect account_list (or the current account landscape) and consider every registered account, including disconnected ones. "+
				"When intent is ambiguous, ask the user which account to disconnect rather than guessing. "+
				"Returns an error if the named account is already disconnected.",
		),
		mcp.WithString("label",
			mcp.Required(),
			mcp.Description("Label of the authenticated account to disconnect. Must already exist in the registry."),
		),
	)
}

// HandleLogoutAccount creates a tool handler that disconnects an authenticated
// account. The handler looks up the entry by label, rejects already-disconnected
// accounts, and atomically mutates the entry via registry.Update to clear
// Credential, Authenticator, and Client and to set Authenticated=false. It
// then clears the persisted token cache for the entry's CacheName via
// auth.ClearTokenCache. The registry entry and accounts.json are preserved
// so the account can be re-authenticated via account_login.
//
// Parameters:
//   - registry: the account registry holding the target entry.
//
// Returns a tool handler function compatible with the MCP server's AddTool
// method. The handler returns a JSON success message with the account label
// on success, or an error result if lookup fails or the account is already
// disconnected.
//
// Side effects: mutates the registry entry (Credential=nil, Authenticator=nil,
// Client=nil, Authenticated=false) and best-effort deletes the token cache
// artifacts for the account's CacheName.
func HandleLogoutAccount(registry *auth.AccountRegistry) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := logging.Logger(ctx)

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
			return mcp.NewToolResultError(fmt.Sprintf("Account %q is already disconnected.", label)), nil
		}

		cacheName := entry.CacheName

		if updateErr := registry.Update(label, func(e *auth.AccountEntry) {
			e.Authenticated = false
			e.Client = nil
			e.Credential = nil
			e.Authenticator = nil
		}); updateErr != nil {
			logger.Error("registry update failed", "label", label, "error", updateErr.Error())
			return mcp.NewToolResultError(updateErr.Error()), nil
		}

		if clearErr := auth.ClearTokenCache(cacheName); clearErr != nil {
			// Best-effort: log and continue. The account is still marked
			// disconnected; next login will overwrite any stale cache entry.
			logger.Warn("failed to clear token cache", "label", label, "cache_name", cacheName, "error", clearErr.Error())
		}

		result := map[string]any{
			"logged_out": true,
			"label":      label,
			"message":    fmt.Sprintf("Account %q disconnected. Configuration preserved; use account_login to reconnect.", label),
		}
		data, err := json.Marshal(result)
		if err != nil {
			return mcp.NewToolResultError("failed to serialize response"), nil
		}

		logger.Info("account disconnected", "label", label)
		return mcp.NewToolResultText(string(data)), nil
	}
}
