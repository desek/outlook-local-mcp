// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the remove_account MCP tool, which removes a previously
// registered account from the account registry. Per CR-0056 FR-44/FR-45 the
// tool succeeds for any registered label regardless of connection state and
// leaves a clean zero-account state when the last account is removed.
package tools

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/mark3labs/mcp-go/mcp"
)

// NewRemoveAccountTool creates the MCP tool definition for remove_account.
// The tool accepts a required "label" parameter identifying the account to
// remove. It is not annotated as read-only since it mutates the registry.
//
// Returns the configured mcp.Tool ready for registration with server.AddTool.
func NewRemoveAccountTool() mcp.Tool {
	return mcp.NewTool("account_remove",
		mcp.WithTitleAnnotation("Remove Account"),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
		mcp.WithDescription(
			"Remove a registered account from the registry and accounts.json and clear its cached tokens. "+
				"Works regardless of the account's connection state — both authenticated and disconnected accounts can be removed. "+
				"This operation is irreversible and destroys the account's configuration (client_id, tenant_id, auth_method, persisted UPN). "+
				"When the user only wants to disconnect an account temporarily, suggest account_logout instead — it preserves the configuration so account_login can reconnect the account later. "+
				"Proactively suggest account_remove only when the user clearly intends to discard the account permanently (for example, when a disconnected account surfaced via account_list is no longer relevant). "+
				"Never assume a default account: before calling, inspect account_list (or the current account landscape) and consider every registered account, including disconnected ones. "+
				"When intent is ambiguous, ask the user which account to remove rather than guessing.",
		),
		mcp.WithString("label",
			mcp.Required(),
			mcp.Description("The label of the account to remove. Must match a registered account label (UPN-based lookup is not accepted by this tool)."),
		),
	)
}

// HandleRemoveAccount creates a tool handler that removes an account from the
// account registry by label. After removing from the in-memory registry, it
// also removes the account's identity configuration from accounts.json.
// Per CR-0056 FR-44/FR-45 the handler succeeds regardless of the account's
// Authenticated state and leaves a valid zero-account state when the last
// account is removed; no label is protected from removal.
//
// Parameters:
//   - registry: the account registry from which the account will be removed.
//   - accountsPath: the filesystem path to the persistent accounts.json file.
//
// Returns a tool handler function compatible with the MCP server's AddTool
// method. The handler returns a plain-text confirmation naming the removed
// account (label plus UPN when known) via mcp.NewToolResultText, or an error
// result if the label is missing or does not refer to a registered account.
//
// Side effects: removes the account entry from the registry and the persistent
// accounts file on success.
func HandleRemoveAccount(registry *auth.AccountRegistry, accountsPath string) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := slog.With("tool", "account_remove")

		label, err := request.RequireString("label")
		if err != nil {
			return mcp.NewToolResultError("missing required parameter: label"), nil
		}

		logger.Debug("tool called", "label", label)

		// Capture the cache partition name and UPN before removing from the
		// registry so we can clear the persisted token cache after removal
		// (CR-0056 FR-43) and include the UPN in the confirmation text.
		var cacheName, upn string
		if entry, ok := registry.Get(label); ok {
			cacheName = entry.CacheName
			upn = entry.Email
		}

		if err := registry.Remove(label); err != nil {
			logger.Warn("remove account failed", "label", label, "error", err.Error())
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Remove the account identity configuration from accounts.json. When
		// this removes the last entry, RemoveAccountConfig leaves a valid
		// accounts.json with an empty accounts array (FR-45); the subsequent
		// resolver error path directs the user to account_add.
		if err := auth.RemoveAccountConfig(accountsPath, label); err != nil {
			logger.Warn("failed to remove account config from accounts.json", "label", label, "error", err.Error())
		}

		// Best-effort keychain and file cache cleanup (CR-0056 FR-43).
		if cacheName != "" {
			if err := auth.ClearTokenCache(cacheName); err != nil {
				logger.Warn("failed to clear token cache", "label", label, "error", err.Error())
			}
		}

		// Plain-text confirmation per CLAUDE.md "MCP Tool Response Tiering":
		// write tools return text confirmations unconditionally.
		var b strings.Builder
		header := fmt.Sprintf("Account removed: %s", label)
		if upn != "" {
			header = fmt.Sprintf("%s (%s)", header, upn)
		}
		b.WriteString(header)
		b.WriteString("\n")
		b.WriteString("Token cache cleared.")
		if line := AccountInfoLine(ctx); line != "" {
			b.WriteString("\n")
			b.WriteString(line)
		}

		logger.Info("account removed", "label", label)
		return mcp.NewToolResultText(b.String()), nil
	}
}
