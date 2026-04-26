// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains tests for the account_login MCP tool (CR-0056 / CR-0064),
// verifying successful re-authentication of a disconnected account, rejection
// of already-connected accounts, not-found handling, and the Phase 3
// silent-first reconnect path.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/mark3labs/mcp-go/mcp"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
)

// loginSuccessCred implements azcore.TokenCredential and always returns a
// valid access token. Used to simulate a warm file-cache credential.
type loginSuccessCred struct{}

// GetToken returns a mock token without any network call.
func (c *loginSuccessCred) GetToken(_ context.Context, _ policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{Token: "mock-silent-token"}, nil
}

// loginFailCred implements azcore.TokenCredential and always returns an error,
// simulating a cold cache that requires interactive authentication.
type loginFailCred struct{}

// GetToken returns an error indicating no cached token is available.
func (c *loginFailCred) GetToken(_ context.Context, _ policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{}, fmt.Errorf("no cached token")
}

// mockSetupCredentialWith returns a setupCredential function that produces the
// given credential. It mirrors fakeSetupCredential for path derivation.
func mockSetupCredentialWith(cred azcore.TokenCredential) func(label, clientID, tenantID, authMethod, cacheName, authRecordDir, tokenStorage string) (
	azcore.TokenCredential, auth.Authenticator, string, string, error,
) {
	return func(label, _, _, _, cacheName, authRecordDir, _ string) (
		azcore.TokenCredential, auth.Authenticator, string, string, error,
	) {
		return cred, nil, authRecordDir + "/" + label + "_auth_record.json", cacheName + "-" + label, nil
	}
}

// mockAuthStateWithCred returns an addAccountState that uses the given
// credential for setupCredential and tracks how many times the authenticate
// function is called.
func mockAuthStateWithCred(cred azcore.TokenCredential, authCallCount *atomic.Int32) *addAccountState {
	return &addAccountState{
		setupCredential: mockSetupCredentialWith(cred),
		authenticate: func(_ context.Context, _ auth.Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
			if authCallCount != nil {
				authCallCount.Add(1)
			}
			return azidentity.AuthenticationRecord{}, nil
		},
		urlElicit: func(_ context.Context, _, _, _ string) (*mcp.ElicitationResult, error) {
			return nil, fmt.Errorf("elicitation not supported")
		},
		elicit: func(_ context.Context, _ mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
			return nil, fmt.Errorf("elicitation not supported")
		},
		openBrowser: func(_ string) error { return nil },
		scopes:      []string{"Calendars.ReadWrite"},
	}
}

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

// TestHandleLoginAccount_SilentSucceeds_SkipsInteractive verifies CR-0064
// Phase 3: when silent token acquisition succeeds, authenticateInline is NOT
// invoked and the response message contains "silent refresh".
func TestHandleLoginAccount_SilentSucceeds_SkipsInteractive(t *testing.T) {
	registry := auth.NewAccountRegistry()
	cfg := addAccountTestConfig(t)

	if err := registry.Add(&auth.AccountEntry{
		Label:         "warm",
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

	var authCalls atomic.Int32
	state := mockAuthStateWithCred(&loginSuccessCred{}, &authCalls)
	handler := handleLoginAccount(state, registry, cfg)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{"label": "warm"}

	result, err := handler(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %v", extractText(t, result))
	}

	text := extractText(t, result)
	var resp map[string]any
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}
	if resp["logged_in"] != true {
		t.Errorf("logged_in = %v, want true", resp["logged_in"])
	}
	msg, _ := resp["message"].(string)
	if !contains(msg, "silent refresh") {
		t.Errorf("message = %q, want to contain 'silent refresh'", msg)
	}

	// authenticate must NOT have been called because silent succeeded.
	if n := authCalls.Load(); n != 0 {
		t.Errorf("authenticate called %d times, want 0 (silent succeeded)", n)
	}
}

// TestHandleLoginAccount_SilentFails_FallsBackToInteractive verifies CR-0064
// Phase 3: when silent token acquisition fails, authenticateInline IS invoked
// (interactive fallback) and the response message does not mention "silent refresh".
func TestHandleLoginAccount_SilentFails_FallsBackToInteractive(t *testing.T) {
	registry := auth.NewAccountRegistry()
	cfg := addAccountTestConfig(t)

	if err := registry.Add(&auth.AccountEntry{
		Label:         "cold",
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

	var authCalls atomic.Int32
	state := mockAuthStateWithCred(&loginFailCred{}, &authCalls)
	handler := handleLoginAccount(state, registry, cfg)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{"label": "cold"}

	result, err := handler(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %v", extractText(t, result))
	}

	text := extractText(t, result)
	var resp map[string]any
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}
	if resp["logged_in"] != true {
		t.Errorf("logged_in = %v, want true", resp["logged_in"])
	}
	msg, _ := resp["message"].(string)
	if contains(msg, "silent refresh") {
		t.Errorf("message = %q, must NOT contain 'silent refresh' when interactive was used", msg)
	}

	// authenticate MUST have been called once (interactive fallback).
	if n := authCalls.Load(); n != 1 {
		t.Errorf("authenticate called %d times, want 1 (interactive fallback)", n)
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
