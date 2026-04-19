// Package auth account resolver middleware for multi-account support.
//
// This file implements the AccountResolver middleware that resolves the
// correct Graph client for each tool call based on the "account" parameter,
// single-account auto-selection, or MCP Elicitation API prompting.
package auth

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// elicitFunc is the function signature for requesting elicitation from the
// MCP client. The default implementation uses ServerFromContext to obtain
// the MCPServer and calls RequestElicitation. Tests replace this to avoid
// requiring a real MCP server in context.
type elicitFunc func(ctx context.Context, request mcp.ElicitationRequest) (*mcp.ElicitationResult, error)

// defaultElicit retrieves the MCPServer from context and calls
// RequestElicitation. This is the production implementation of elicitFunc.
//
// Parameters:
//   - ctx: the context containing the MCPServer and client session.
//   - request: the elicitation request to send to the client.
//
// Returns the elicitation result, or an error if the server is not in
// context or the client does not support elicitation.
func defaultElicit(ctx context.Context, request mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
	srv := mcpserver.ServerFromContext(ctx)
	if srv == nil {
		return nil, fmt.Errorf("no MCP server in context")
	}
	return srv.RequestElicitation(ctx, request)
}

// accountResolverState holds the configuration for the AccountResolver
// middleware. It is separated from the middleware closure to allow tests
// to inject a mock elicitation function.
type accountResolverState struct {
	// registry is the account registry to look up accounts in.
	registry *AccountRegistry

	// elicit is the function called to request account selection from the
	// user when multiple accounts are registered and no account parameter
	// is provided. Defaults to defaultElicit.
	elicit elicitFunc
}

// AccountResolver returns a middleware factory that resolves the correct
// Graph client for each tool call. The resolution strategy considers only
// authenticated accounts (Authenticated == true):
//
//  1. If the request includes an "account" parameter, look up that label in
//     the registry. Return an error if not found.
//  2. If no "account" parameter and zero authenticated accounts exist,
//     return an error instructing the user to authenticate via add_account.
//  3. If no "account" parameter and exactly one authenticated account exists,
//     auto-select it without elicitation.
//  4. If no "account" parameter and multiple authenticated accounts exist,
//     use the MCP Elicitation API to prompt the user to select an account.
//  5. If elicitation returns accept, use the selected account. If decline
//     or cancel, return an error. On any elicitation error, fall back to
//     the "default" account.
//
// After resolution, the middleware injects the Graph client via
// WithGraphClient and the account auth details via WithAccountAuth, then
// calls the next handler.
//
// Parameters:
//   - registry: the account registry containing all authenticated accounts.
//
// Returns a middleware function compatible with the tool handler wrapping
// pattern used in RegisterTools.
func AccountResolver(registry *AccountRegistry) func(mcpserver.ToolHandlerFunc) mcpserver.ToolHandlerFunc {
	state := &accountResolverState{
		registry: registry,
		elicit:   defaultElicit,
	}
	return state.middleware
}

// middleware is the actual middleware implementation. It wraps each tool
// handler with account resolution logic.
//
// Parameters:
//   - next: the inner tool handler to call after resolving the account.
//
// Returns a wrapped tool handler that resolves the account before calling next.
func (s *accountResolverState) middleware(next mcpserver.ToolHandlerFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		entry, err := s.resolveAccount(ctx, request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		ctx = WithGraphClient(ctx, entry.Client)
		ctx = WithAccountAuth(ctx, AccountAuth{
			Authenticator:  entry.Authenticator,
			AuthRecordPath: entry.AuthRecordPath,
			AuthMethod:     inferAuthMethod(entry),
		})
		ctx = WithAccountInfo(ctx, AccountInfo{
			Label:    entry.Label,
			Email:    entry.Email,
			Advisory: s.disconnectedAdvisory(entry, request),
		})

		return next(ctx, request)
	}
}

// disconnectedAdvisory returns a human-readable note when the resolver
// auto-selected the sole authenticated account while disconnected accounts
// also exist in the registry. The advisory names the disconnected accounts
// by UPN so the LLM can raise them to the user instead of silently
// operating on the single connected mailbox (CR-0056 FR-52 / AC-17).
//
// An empty string is returned when no advisory is warranted: the user
// passed an explicit account parameter, elicitation occurred, or there are
// no disconnected accounts to surface.
//
// Parameters:
//   - entry: the resolved account entry.
//   - request: the originating tool call (used to detect an explicit account
//     parameter, in which case no advisory is attached).
//
// Returns the advisory text or an empty string.
func (s *accountResolverState) disconnectedAdvisory(entry *AccountEntry, request mcp.CallToolRequest) string {
	// Only auto-select path warrants an advisory. If the caller passed an
	// explicit `account` parameter, the choice was intentional.
	if raw, ok := request.GetArguments()["account"]; ok {
		if str, isStr := raw.(string); isStr && str != "" {
			return ""
		}
	}

	authenticated := s.registry.ListAuthenticated()
	if len(authenticated) != 1 || authenticated[0] != entry {
		return ""
	}

	disconnected := s.disconnectedAccounts()
	if len(disconnected) == 0 {
		return ""
	}

	return formatDisconnectedAdvisory(disconnected)
}

// disconnectedAccounts returns the registry entries whose Authenticated flag
// is false, sorted by label. Callers use this list to surface disconnected
// accounts in elicitation enums and error messages.
func (s *accountResolverState) disconnectedAccounts() []*AccountEntry {
	out := make([]*AccountEntry, 0)
	for _, e := range s.registry.List() {
		if !e.Authenticated {
			out = append(out, e)
		}
	}
	return out
}

// formatDisconnectedAdvisory builds the human-readable advisory naming each
// disconnected account by label and UPN. The output is intentionally terse so
// downstream formatters can embed it on a single line.
func formatDisconnectedAdvisory(disconnected []*AccountEntry) string {
	parts := make([]string, 0, len(disconnected))
	for _, e := range disconnected {
		parts = append(parts, formatAccountIdentity(e))
	}
	return fmt.Sprintf(
		"Other disconnected accounts exist: %s. Use account_login to reconnect if one was intended.",
		strings.Join(parts, ", "),
	)
}

// formatAccountIdentity renders an account as "label (upn)" when the UPN is
// known, or just the label otherwise. Used by elicitation enum values,
// advisory messages, and disconnected-account error listings.
func formatAccountIdentity(entry *AccountEntry) string {
	if entry.Email == "" {
		return entry.Label
	}
	return fmt.Sprintf("%s (%s)", entry.Label, entry.Email)
}

// formatElicitationChoice renders an elicitation enum value. Disconnected
// accounts are suffixed with " — disconnected" so the user sees each
// account's state in the picker (CR-0056 FR-35 / AC-8).
func formatElicitationChoice(entry *AccountEntry) string {
	base := formatAccountIdentity(entry)
	if !entry.Authenticated {
		return base + " — disconnected"
	}
	return base
}

// parseElicitationChoice extracts the label from an elicitation enum value
// produced by formatElicitationChoice. The accepted shapes are:
//
//	"label"
//	"label (upn)"
//	"label (upn) — disconnected"
//	"label — disconnected"
//
// The label is always the leading token up to the first space.
func parseElicitationChoice(value string) string {
	if idx := strings.Index(value, " "); idx >= 0 {
		return value[:idx]
	}
	return value
}

// resolveAccount determines which account entry to use for the current
// request. It implements the resolution strategy documented on AccountResolver.
// Only authenticated accounts are considered for auto-selection and
// elicitation; unauthenticated accounts are excluded from the resolution
// strategy (though they can still be selected explicitly by label).
//
// Parameters:
//   - ctx: the context for elicitation calls.
//   - request: the tool call request potentially containing an "account" param.
//
// Returns the resolved account entry, or an error if resolution fails.
func (s *accountResolverState) resolveAccount(ctx context.Context, request mcp.CallToolRequest) (*AccountEntry, error) {
	// Check if an explicit account parameter was provided.
	args := request.GetArguments()
	if accountLabel, ok := args["account"]; ok {
		label, isStr := accountLabel.(string)
		if !isStr || label == "" {
			return nil, fmt.Errorf("invalid account parameter: must be a non-empty string")
		}
		// Label first, then UPN fallback (CR-0056 FR-6/FR-8).
		entry, found := s.registry.Get(label)
		if !found {
			entry, found = s.registry.GetByUPN(label)
		}
		if !found {
			return nil, fmt.Errorf("account %q not found", label)
		}
		if !entry.Authenticated {
			return nil, disconnectedExplicitError(entry)
		}
		return entry, nil
	}

	// No explicit account parameter: consider only authenticated accounts.
	authenticated := s.registry.ListAuthenticated()

	if len(authenticated) == 0 {
		disconnected := s.disconnectedAccounts()
		if len(disconnected) == 0 {
			// Zero accounts total — only account_add applies (FR-46).
			return nil, fmt.Errorf("no accounts registered. Use account_add to authenticate")
		}
		return nil, zeroAuthenticatedWithDisconnectedError(disconnected)
	}

	if len(authenticated) == 1 {
		return authenticated[0], nil
	}

	// Multiple authenticated accounts: elicit selection from the user.
	return s.elicitAccountSelection(ctx)
}

// disconnectedExplicitError builds the actionable error returned when a
// caller explicitly targets a disconnected account via the `account`
// parameter (CR-0056 FR-38 / AC-9).
func disconnectedExplicitError(entry *AccountEntry) error {
	return fmt.Errorf(
		"account %q (%s) is disconnected. Use account_login with label %q to re-authenticate",
		entry.Label, entry.Email, entry.Label,
	)
}

// zeroAuthenticatedWithDisconnectedError builds the error returned when no
// authenticated accounts exist but one or more disconnected accounts are
// registered (CR-0056 FR-37 / AC-12). The message enumerates every
// disconnected account by UPN and suggests both `account_login` and
// `account_add` as recovery paths.
func zeroAuthenticatedWithDisconnectedError(disconnected []*AccountEntry) error {
	parts := make([]string, 0, len(disconnected))
	for _, e := range disconnected {
		parts = append(parts, formatAccountIdentity(e))
	}
	return fmt.Errorf(
		"no authenticated accounts. %d disconnected account(s): %s. "+
			"Use account_login to re-authenticate, or account_add to add a new account",
		len(disconnected), strings.Join(parts, ", "),
	)
}

// elicitAccountSelection uses the MCP Elicitation API to prompt the user
// to select an account from the registry. On any elicitation error (not
// just ErrElicitationNotSupported), it falls back to the "default" account.
// When no "default" account exists, the error message lists all available
// account labels and hints about the "account" parameter so the LLM can
// self-correct.
//
// Parameters:
//   - ctx: the context for the elicitation call.
//
// Returns the selected account entry, or an error if selection fails.
func (s *accountResolverState) elicitAccountSelection(ctx context.Context) (*AccountEntry, error) {
	// Build enum values for ALL registered accounts — authenticated and
	// disconnected — so the user can see the full landscape in the picker
	// (CR-0056 FR-34/FR-35 / AC-8).
	all := s.registry.List()
	choices := make([]string, 0, len(all))
	for _, e := range all {
		choices = append(choices, formatElicitationChoice(e))
	}

	elicitationRequest := mcp.ElicitationRequest{
		Params: mcp.ElicitationParams{
			Message: "Multiple accounts are registered. Please select an account to use.",
			RequestedSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"account": map[string]any{
						"type":        "string",
						"description": "Select an account",
						"enum":        choices,
					},
				},
				"required": []string{"account"},
			},
		},
	}

	result, err := s.elicit(ctx, elicitationRequest)
	if err != nil {
		// Fall back to "default" account on any elicitation error.
		slog.Warn("elicitation failed, falling back to default account", "error", err)
		entry, found := s.registry.Get("default")
		if !found {
			return nil, fmt.Errorf(
				"multiple accounts registered (%s) but elicitation is not available and no "+
					"\"default\" account exists. Specify the account explicitly using the "+
					"'account' parameter",
				strings.Join(s.registry.Labels(), ", "))
		}
		if !entry.Authenticated {
			return nil, disconnectedExplicitError(entry)
		}
		return entry, nil
	}

	switch result.Action {
	case mcp.ElicitationResponseActionAccept:
		content, ok := result.Content.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("unexpected elicitation response content type")
		}
		selectedValue, ok := content["account"].(string)
		if !ok || selectedValue == "" {
			return nil, fmt.Errorf("no account selected in elicitation response")
		}
		// The enum value is "label (upn)[ — disconnected]"; strip to label.
		selectedLabel := parseElicitationChoice(selectedValue)
		entry, found := s.registry.Get(selectedLabel)
		if !found {
			return nil, fmt.Errorf("selected account %q not found", selectedLabel)
		}
		if !entry.Authenticated {
			return nil, disconnectedExplicitError(entry)
		}
		return entry, nil

	case mcp.ElicitationResponseActionDecline:
		return nil, fmt.Errorf("account selection declined by user")

	case mcp.ElicitationResponseActionCancel:
		return nil, fmt.Errorf("account selection cancelled by user")

	default:
		return nil, fmt.Errorf("unexpected elicitation action: %s", result.Action)
	}
}

// inferAuthMethod determines the auth method string for an account entry by
// checking whether the entry's Authenticator implements the AuthCodeFlow
// interface. If so, the method is "auth_code"; otherwise it defaults to
// "browser" as the primary auth method per CR-0024.
//
// Parameters:
//   - entry: the account entry to infer the auth method from.
//
// Returns "auth_code" when the Authenticator implements AuthCodeFlow,
// or "browser" otherwise.
func inferAuthMethod(entry *AccountEntry) string {
	if _, ok := entry.Authenticator.(AuthCodeFlow); ok {
		return "auth_code"
	}
	return "browser"
}
