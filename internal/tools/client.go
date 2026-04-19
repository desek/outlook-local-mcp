// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides a helper function to retrieve the Graph client from
// the request context. Tool handlers call GraphClient at the start of each
// invocation to obtain the per-request client injected by the AccountResolver
// middleware.
package tools

import (
	"context"
	"fmt"

	"github.com/desek/outlook-local-mcp/internal/auth"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
)

// AccountParamDescription is the shared description text applied to the
// `account` parameter of every tool that supports account selection (CR-0056
// FR-42, FR-49, FR-50). The text is deliberately explicit about three things:
//
//  1. Both the account label AND its User Principal Name (UPN, e.g.,
//     "alice@contoso.com") are accepted. The resolver performs label lookup
//     first, then UPN fallback.
//  2. Auto-selection is NOT a generic "default account" behavior. Omitting
//     `account` only auto-selects when exactly one account is authenticated
//     AND zero other accounts (connected or disconnected) are registered.
//     When the single authenticated account coexists with disconnected
//     siblings the resolver still auto-selects but emits an advisory so the
//     LLM can surface the disconnected siblings to the user.
//  3. The LLM must never silently assume a "default account" — it must
//     inspect the current account landscape (via account_list or prior
//     context), consider disconnected accounts, and ask the user when intent
//     is ambiguous.
const AccountParamDescription = "Account to use — accepts either the account label OR the User Principal Name (UPN, e.g., alice@contoso.com); lookup tries label first, then UPN. " +
	"Never assume a default account: inspect account_list (or the current account landscape) and consider every registered account, including disconnected ones, before acting. " +
	"Omitting this parameter yields silent auto-selection ONLY when exactly one account is authenticated and no other accounts (connected or disconnected) are registered. " +
	"When a single authenticated account coexists with disconnected accounts, auto-selection still occurs but the response surfaces an advisory naming the disconnected accounts — raise that advisory to the user rather than ignoring it. " +
	"In every other situation determine the account explicitly: ask the user or rely on prior context. " +
	"Use account_list to enumerate all registered accounts (authenticated and disconnected) with their UPNs."

// GraphClient retrieves the Microsoft Graph client from the request context.
// The AccountResolver middleware injects the client via auth.WithGraphClient
// before the handler runs. If the client is not in context (e.g., no account
// has been selected), an error is returned.
//
// Parameters:
//   - ctx: the request context containing the injected Graph client.
//
// Returns the Graph client, or an error if the client is not present.
func GraphClient(ctx context.Context) (*msgraphsdk.GraphServiceClient, error) {
	client, ok := auth.GraphClientFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("no account selected")
	}
	return client, nil
}

// AccountInfoLine returns a formatted "Account: label (email)" line from the
// AccountInfo stored in context by the AccountResolver middleware. The line is
// suitable for appending to write-tool confirmation responses. Returns an empty
// string when no AccountInfo is in context or the label is empty.
//
// Parameters:
//   - ctx: the request context containing the injected AccountInfo.
//
// Returns a formatted account line, or "" when not available.
//
// Side effects: none.
func AccountInfoLine(ctx context.Context) string {
	info, ok := auth.AccountInfoFromContext(ctx)
	if !ok {
		return ""
	}
	return FormatAccountLine(info.Label, info.Email, info.Advisory)
}
