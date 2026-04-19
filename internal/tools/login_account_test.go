// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains tests for the account_login MCP tool (CR-0056), verifying
// successful re-authentication of a disconnected account, rejection of
// already-connected accounts, and not-found handling.
package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/mark3labs/mcp-go/mcp"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
)

// TestLoginAccount_Success verifies that account_login re-authenticates an
// existing disconnected account: the entry's Authenticated flag flips to true,
// a Graph client is attached, and the persisted identity fields are retained.
func TestLoginAccount_Success(t *testing.T) {
	registry := auth.NewAccountRegistry()
	cfg := addAccountTestConfig(t)

	// Pre-register a disconnected account (as RestoreAccounts would).
	if err := registry.Add(&auth.AccountEntry{
		Label:         "work",
		ClientID:      cfg.ClientID,
		TenantID:      cfg.TenantID,
		AuthMethod:    "browser",
		Authenticated: false,
	}); err != nil {
		t.Fatalf("registry.Add() error: %v", err)
	}

	client, srv := newTestGraphClient(t, nil)
	defer srv.Close()
	origFactory := graphClientFactory
	graphClientFactory = func(_ azcore.TokenCredential) (*msgraphsdk.GraphServiceClient, error) {
		return client, nil
	}
	defer func() { graphClientFactory = origFactory }()

	state := mockAuthState()
	handler := handleLoginAccount(state, registry, cfg)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{"label": "work"}

	result, err := handler(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %v", result)
	}

	text := extractText(t, result)
	var resp map[string]any
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}
	if resp["logged_in"] != true {
		t.Errorf("logged_in = %v, want true", resp["logged_in"])
	}
	if resp["label"] != "work" {
		t.Errorf("label = %v, want work", resp["label"])
	}

	entry, ok := registry.Get("work")
	if !ok {
		t.Fatal("expected 'work' entry to remain in registry")
	}
	if !entry.Authenticated {
		t.Error("expected entry.Authenticated = true after login")
	}
	if entry.Client != client {
		t.Error("expected Graph client attached to entry after login")
	}
}

// TestLoginAccount_AlreadyConnected verifies that account_login rejects an
// already-authenticated account with a message mentioning connection state.
func TestLoginAccount_AlreadyConnected(t *testing.T) {
	registry := auth.NewAccountRegistry()
	cfg := addAccountTestConfig(t)

	if err := registry.Add(&auth.AccountEntry{
		Label:         "work",
		Authenticated: true,
	}); err != nil {
		t.Fatalf("registry.Add() error: %v", err)
	}

	state := mockAuthState()
	handler := handleLoginAccount(state, registry, cfg)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{"label": "work"}

	result, err := handler(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for already-connected account")
	}
	text := extractText(t, result)
	if !contains(text, "already connected") {
		t.Errorf("error text = %q, want to contain 'already connected'", text)
	}
}

// TestLoginAccount_NotFound verifies that account_login returns an error when
// the requested label is not present in the registry.
func TestLoginAccount_NotFound(t *testing.T) {
	registry := auth.NewAccountRegistry()
	cfg := addAccountTestConfig(t)

	state := mockAuthState()
	handler := handleLoginAccount(state, registry, cfg)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{"label": "nonexistent"}

	result, err := handler(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for missing account")
	}
	text := extractText(t, result)
	if !contains(text, "not found") {
		t.Errorf("error text = %q, want to contain 'not found'", text)
	}
}

// contains is a tiny substring helper to keep the test assertions readable
// without importing strings in this file.
func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

// indexOf returns the index of sub in s, or -1. Avoids importing strings.
func indexOf(s, sub string) int {
	n, m := len(s), len(sub)
	for i := 0; i+m <= n; i++ {
		if s[i:i+m] == sub {
			return i
		}
	}
	return -1
}
