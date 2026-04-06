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
	return FormatAccountLine(info.Label, info.Email)
}
