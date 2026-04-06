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
		ctx = WithAccountInfo(ctx, AccountInfo{Label: entry.Label, Email: entry.Email})

		return next(ctx, request)
	}
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
		entry, found := s.registry.Get(label)
		if !found {
			return nil, fmt.Errorf("account %q not found", label)
		}
		return entry, nil
	}

	// No explicit account parameter: consider only authenticated accounts.
	authenticated := s.registry.ListAuthenticated()

	if len(authenticated) == 0 {
		return nil, fmt.Errorf("no authenticated accounts. Use account_add to authenticate")
	}

	if len(authenticated) == 1 {
		return authenticated[0], nil
	}

	// Multiple authenticated accounts: elicit selection from the user.
	return s.elicitAccountSelection(ctx)
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
	labels := s.registry.Labels()

	elicitationRequest := mcp.ElicitationRequest{
		Params: mcp.ElicitationParams{
			Message: "Multiple accounts are registered. Please select an account to use.",
			RequestedSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"account": map[string]any{
						"type":        "string",
						"description": "Select an account",
						"enum":        labels,
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
				strings.Join(labels, ", "))
		}
		return entry, nil
	}

	switch result.Action {
	case mcp.ElicitationResponseActionAccept:
		content, ok := result.Content.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("unexpected elicitation response content type")
		}
		selectedLabel, ok := content["account"].(string)
		if !ok || selectedLabel == "" {
			return nil, fmt.Errorf("no account selected in elicitation response")
		}
		entry, found := s.registry.Get(selectedLabel)
		if !found {
			return nil, fmt.Errorf("selected account %q not found", selectedLabel)
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
