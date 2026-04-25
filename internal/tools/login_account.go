// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the account_login MCP tool (CR-0056), which re-authenticates
// an existing disconnected account without removing and re-adding it. It reuses
// the inline authentication machinery from add_account.go (via addAccountState)
// to drive browser, device_code, or auth_code flows against the account's
// persisted client_id, tenant_id, and auth_method.
package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/config"
	"github.com/desek/outlook-local-mcp/internal/logging"
	"github.com/mark3labs/mcp-go/mcp"
)

// NewLoginAccountTool creates the MCP tool definition for account_login. The
// tool accepts a required "label" parameter identifying a previously registered
// but currently disconnected account. Annotations (CR-0052): ReadOnly=false,
// Destructive=false, Idempotent=false (a fresh authentication state is produced
// each invocation), OpenWorld=true (calls Microsoft identity and Graph APIs).
//
// Returns the configured mcp.Tool ready for registration with server.AddTool.
func NewLoginAccountTool() mcp.Tool {
	return mcp.NewTool("account_login",
		mcp.WithTitleAnnotation("Log In Account"),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithDescription(
			"Re-authenticate an existing disconnected account without removing it. "+
				"Uses the account's persisted auth_method, client_id, and tenant_id to drive the same inline authentication flow as account_add. "+
				"Proactively suggest this tool to the user whenever account_list, status, or a resolver advisory surfaces a disconnected account that may be relevant to the user's intent — do not silently skip a disconnected account. "+
				"Never assume a default account: before calling, inspect account_list (or the current account landscape) and consider every registered account, including disconnected ones. "+
				"When intent is ambiguous, ask the user which account to reconnect rather than guessing. "+
				"Returns an error if the named account is already connected.",
		),
		mcp.WithString("label",
			mcp.Required(),
			mcp.Description("Label of the disconnected account to re-authenticate. Must already exist in the registry (created previously via account_add)."),
		),
	)
}

// HandleLoginAccount creates a tool handler that re-authenticates an existing
// disconnected account. The handler looks up the entry by label, rejects
// already-connected accounts, constructs a credential from the persisted
// client_id / tenant_id / auth_method, runs the same inline authentication
// flow used by add_account, builds a fresh Graph client, and updates the
// registry entry atomically via registry.Update. On success it refreshes the
// UPN from /me and persists it to accounts.json.
//
// Parameters:
//   - registry: the account registry holding the target entry.
//   - cfg: server configuration supplying scopes, cache base name, and the
//     accounts.json path used for UPN persistence.
//
// Returns a tool handler function compatible with the MCP server's AddTool
// method. The handler returns a JSON success message with the account label
// and refreshed UPN, or an error result if lookup, authentication, or Graph
// client creation fails.
//
// Side effects: creates a new credential with the account's keychain cache
// partition, runs interactive authentication, mutates the registry entry
// (Credential, Authenticator, Client, CacheName, AuthRecordPath, Authenticated,
// Email), and may rewrite accounts.json to persist a resolved UPN.
func HandleLoginAccount(registry *auth.AccountRegistry, cfg config.Config) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	scopes := auth.Scopes(cfg)
	if graphClientFactory == nil {
		graphClientFactory = auth.NewDefaultGraphClientFactory(scopes)
	}
	state := defaultAddAccountState(scopes)
	return handleLoginAccount(state, registry, cfg)
}

// handleLoginAccount is the internal implementation that uses an injectable
// addAccountState so tests can replace credential setup, authentication, and
// elicitation. It is structurally analogous to add_account but targets an
// existing registry entry instead of creating a new one.
func handleLoginAccount(s *addAccountState, registry *auth.AccountRegistry, cfg config.Config) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
		if entry.Authenticated {
			return mcp.NewToolResultError(fmt.Sprintf("Account %q is already connected.", label)), nil
		}

		clientID := entry.ClientID
		if clientID == "" {
			clientID = cfg.ClientID
		}
		tenantID := entry.TenantID
		if tenantID == "" {
			tenantID = cfg.TenantID
		}
		authMethod := entry.AuthMethod
		if authMethod == "" {
			authMethod = cfg.AuthMethod
		}

		authRecordDir := filepath.Dir(cfg.AuthRecordPath)

		cred, authenticator, authRecordPath, cacheName, err := s.setupCredential(
			label, clientID, tenantID, authMethod, cfg.CacheName, authRecordDir,
		)
		if err != nil {
			logger.Error("credential setup failed", "label", label, "error", err.Error())
			return mcp.NewToolResultError(fmt.Sprintf("failed to set up credential for account %q: %s", label, err.Error())), nil
		}

		authErr := s.authenticateInline(ctx, cred, authenticator, authRecordPath, authMethod, cacheName, clientID, tenantID, label, logger)
		if authErr != nil {
			var dcErr *DeviceCodeFallbackError
			if errors.As(authErr, &dcErr) {
				return mcp.NewToolResultText(dcErr.Message), nil
			}
			logger.Error("authentication failed during account_login", "label", label, "error", authErr.Error())
			return mcp.NewToolResultError(fmt.Sprintf("failed to authenticate account %q: %s", label, authErr.Error())), nil
		}

		client, err := graphClientFactory(cred)
		if err != nil {
			logger.Error("graph client creation failed", "label", label, "error", err.Error())
			return mcp.NewToolResultError(fmt.Sprintf("failed to create Graph client for account %q: %s", label, err.Error())), nil
		}

		if err := registry.Update(label, func(e *auth.AccountEntry) {
			e.ClientID = clientID
			e.TenantID = tenantID
			e.AuthMethod = authMethod
			e.Credential = cred
			e.Authenticator = authenticator
			e.Client = client
			e.AuthRecordPath = authRecordPath
			e.CacheName = cacheName
			e.Authenticated = true
			e.Email = ""
		}); err != nil {
			logger.Error("registry update failed", "label", label, "error", err.Error())
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Refresh UPN from /me and backfill accounts.json when empty.
		refreshed, _ := registry.Get(label)
		if refreshed != nil {
			auth.EnsureEmailAndPersistUPN(ctx, refreshed, cfg.AccountsPath)
		}

		upn := ""
		if refreshed != nil {
			upn = refreshed.Email
		}

		result := map[string]any{
			"logged_in": true,
			"label":     label,
			"upn":       upn,
			"message":   fmt.Sprintf("Account %q re-authenticated successfully.", label),
		}
		data, err := json.Marshal(result)
		if err != nil {
			return mcp.NewToolResultError("failed to serialize response"), nil
		}

		logger.Info("account re-authenticated", "label", label, "upn", upn)
		return mcp.NewToolResultText(string(data)), nil
	}
}
