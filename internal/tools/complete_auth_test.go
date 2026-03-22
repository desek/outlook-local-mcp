// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains tests for the complete_auth MCP tool, verifying that it
// correctly exchanges authorization codes via the AuthCodeFlow interface and
// handles error cases (missing parameters, invalid URLs, exchange failures).
package tools

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/mark3labs/mcp-go/mcp"
)

// mockAuthCodeCred is a test double that implements both auth.Authenticator and
// auth.AuthCodeFlow. It records calls to ExchangeCode and returns configurable
// errors.
type mockAuthCodeCred struct {
	exchangeErr  error
	exchangedURL string
}

// Authenticate satisfies the auth.Authenticator interface. For the auth_code
// flow, this method is not used by complete_auth.
func (m *mockAuthCodeCred) Authenticate(_ context.Context, _ *policy.TokenRequestOptions) (azidentity.AuthenticationRecord, error) {
	return azidentity.AuthenticationRecord{}, fmt.Errorf("not implemented")
}

// AuthCodeURL satisfies the auth.AuthCodeFlow interface. Not used by
// complete_auth directly.
func (m *mockAuthCodeCred) AuthCodeURL(_ context.Context, _ []string) (string, error) {
	return "https://login.microsoftonline.com/test", nil
}

// ExchangeCode satisfies the auth.AuthCodeFlow interface. It records the
// redirect URL and returns the configured error.
func (m *mockAuthCodeCred) ExchangeCode(_ context.Context, redirectURL string, _ []string) error {
	m.exchangedURL = redirectURL
	return m.exchangeErr
}

// Compile-time interface compliance checks.
var (
	_ auth.Authenticator = (*mockAuthCodeCred)(nil)
	_ auth.AuthCodeFlow  = (*mockAuthCodeCred)(nil)
)

// mockNonAuthCodeCred implements auth.Authenticator but NOT auth.AuthCodeFlow.
// Used to test the type-assertion failure path.
type mockNonAuthCodeCred struct{}

// Authenticate satisfies the auth.Authenticator interface.
func (m *mockNonAuthCodeCred) Authenticate(_ context.Context, _ *policy.TokenRequestOptions) (azidentity.AuthenticationRecord, error) {
	return azidentity.AuthenticationRecord{}, fmt.Errorf("not implemented")
}

// Compile-time interface compliance check.
var _ auth.Authenticator = (*mockNonAuthCodeCred)(nil)

// buildCallToolRequest creates a mcp.CallToolRequest with the given arguments.
func buildCallToolRequest(args map[string]any) mcp.CallToolRequest {
	req := mcp.CallToolRequest{}
	req.Params.Name = "complete_auth"
	req.Params.Arguments = args
	return req
}

// TestCompleteAuth_ValidURL verifies that a valid nativeclient redirect URL
// with an authorization code results in a successful exchange and a success
// message in the tool result.
func TestCompleteAuth_ValidURL(t *testing.T) {
	mock := &mockAuthCodeCred{}
	handler := HandleCompleteAuth(mock, "/tmp/test-auth-record.json", nil, []string{"Calendars.ReadWrite"})

	redirectURL := "https://login.microsoftonline.com/common/oauth2/nativeclient?code=abc123&state=xyz"
	req := buildCallToolRequest(map[string]any{"redirect_url": redirectURL})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %v", result.Content)
	}

	text := extractText(t, result)
	if !strings.Contains(text, "success") {
		t.Errorf("result text %q should contain 'success'", text)
	}

	if mock.exchangedURL != redirectURL {
		t.Errorf("ExchangeCode called with %q, want %q", mock.exchangedURL, redirectURL)
	}
}

// TestCompleteAuth_InvalidURL verifies that when ExchangeCode returns an error
// for an invalid URL, the tool returns a descriptive error result.
func TestCompleteAuth_InvalidURL(t *testing.T) {
	mock := &mockAuthCodeCred{
		exchangeErr: fmt.Errorf("invalid redirect URL: must start with https://login.microsoftonline.com/common/oauth2/nativeclient"),
	}
	handler := HandleCompleteAuth(mock, "/tmp/test-auth-record.json", nil, []string{"Calendars.ReadWrite"})

	req := buildCallToolRequest(map[string]any{"redirect_url": "https://evil.com/callback?code=abc"})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for invalid URL")
	}

	text := extractText(t, result)
	if !strings.Contains(text, "Failed to exchange") {
		t.Errorf("error text %q should contain 'Failed to exchange'", text)
	}
}

// TestCompleteAuth_MissingParam verifies that calling the tool without the
// redirect_url parameter returns an error.
func TestCompleteAuth_MissingParam(t *testing.T) {
	mock := &mockAuthCodeCred{}
	handler := HandleCompleteAuth(mock, "/tmp/test-auth-record.json", nil, []string{"Calendars.ReadWrite"})

	// No redirect_url in arguments.
	req := buildCallToolRequest(map[string]any{})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for missing redirect_url")
	}

	text := extractText(t, result)
	if !strings.Contains(text, "redirect_url") {
		t.Errorf("error text %q should mention 'redirect_url'", text)
	}
}

// TestCompleteAuth_EmptyParam verifies that an empty redirect_url returns
// an error.
func TestCompleteAuth_EmptyParam(t *testing.T) {
	mock := &mockAuthCodeCred{}
	handler := HandleCompleteAuth(mock, "/tmp/test-auth-record.json", nil, []string{"Calendars.ReadWrite"})

	req := buildCallToolRequest(map[string]any{"redirect_url": ""})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for empty redirect_url")
	}

	text := extractText(t, result)
	if !strings.Contains(text, "must not be empty") {
		t.Errorf("error text %q should contain 'must not be empty'", text)
	}
}

// TestCompleteAuth_ExchangeFails verifies that when the token exchange fails,
// the tool returns a descriptive error with troubleshooting guidance.
func TestCompleteAuth_ExchangeFails(t *testing.T) {
	mock := &mockAuthCodeCred{
		exchangeErr: fmt.Errorf("exchange authorization code: token expired"),
	}
	handler := HandleCompleteAuth(mock, "/tmp/test-auth-record.json", nil, []string{"Calendars.ReadWrite"})

	redirectURL := "https://login.microsoftonline.com/common/oauth2/nativeclient?code=expired123"
	req := buildCallToolRequest(map[string]any{"redirect_url": redirectURL})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result when exchange fails")
	}

	text := extractText(t, result)
	if !strings.Contains(text, "token expired") {
		t.Errorf("error text %q should contain the exchange error", text)
	}
	if !strings.Contains(text, "Troubleshooting") {
		t.Errorf("error text %q should contain troubleshooting guidance", text)
	}
}

// TestCompleteAuth_NonAuthCodeCredential verifies that if the credential does
// not implement AuthCodeFlow, the tool returns a descriptive internal error.
func TestCompleteAuth_NonAuthCodeCredential(t *testing.T) {
	mock := &mockNonAuthCodeCred{}
	handler := HandleCompleteAuth(mock, "/tmp/test-auth-record.json", nil, []string{"Calendars.ReadWrite"})

	redirectURL := "https://login.microsoftonline.com/common/oauth2/nativeclient?code=abc"
	req := buildCallToolRequest(map[string]any{"redirect_url": redirectURL})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for non-AuthCodeFlow credential")
	}

	text := extractText(t, result)
	if !strings.Contains(text, "does not support the auth_code flow") {
		t.Errorf("error text %q should indicate auth_code not supported", text)
	}
}

// TestNewCompleteAuthTool_Definition verifies the tool definition has the
// expected name and parameters.
func TestNewCompleteAuthTool_Definition(t *testing.T) {
	tool := NewCompleteAuthTool()

	if tool.Name != "complete_auth" {
		t.Errorf("tool name = %q, want %q", tool.Name, "complete_auth")
	}

	schema, ok := tool.InputSchema.Properties["redirect_url"]
	if !ok {
		t.Fatal("expected redirect_url property in input schema")
	}

	// Verify redirect_url is in the schema.
	schemaMap, ok := schema.(map[string]any)
	if !ok {
		t.Fatalf("expected redirect_url schema to be map, got %T", schema)
	}
	if schemaMap["type"] != "string" {
		t.Errorf("redirect_url type = %v, want 'string'", schemaMap["type"])
	}

	// Verify redirect_url is required.
	found := false
	for _, req := range tool.InputSchema.Required {
		if req == "redirect_url" {
			found = true
			break
		}
	}
	if !found {
		t.Error("redirect_url should be in required list")
	}

	// Verify account parameter exists (optional).
	if _, ok := tool.InputSchema.Properties["account"]; !ok {
		t.Error("expected account property in input schema")
	}
}

// TestCompleteAuth_WithAccountParam verifies that when the "account" parameter
// is provided, the handler looks up the account in the registry and uses its
// credential for the code exchange.
func TestCompleteAuth_WithAccountParam(t *testing.T) {
	// Default credential (should NOT be used).
	defaultMock := &mockAuthCodeCred{}

	// Account credential (should be used).
	accountMock := &mockAuthCodeCred{}

	registry := auth.NewAccountRegistry()
	if err := registry.Add(&auth.AccountEntry{
		Label:          "work",
		Authenticator:  accountMock,
		AuthRecordPath: t.TempDir() + "/work_auth.json",
	}); err != nil {
		t.Fatalf("registry.Add() error: %v", err)
	}

	handler := HandleCompleteAuth(defaultMock, "/tmp/default-auth-record.json", registry, []string{"Calendars.ReadWrite"})

	redirectURL := "https://login.microsoftonline.com/common/oauth2/nativeclient?code=acct123"
	req := buildCallToolRequest(map[string]any{
		"redirect_url": redirectURL,
		"account":      "work",
	})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %v", result.Content)
	}

	// Verify the account's credential was used, not the default.
	if accountMock.exchangedURL != redirectURL {
		t.Errorf("account ExchangeCode called with %q, want %q", accountMock.exchangedURL, redirectURL)
	}
	if defaultMock.exchangedURL != "" {
		t.Error("default credential should not have been used when account param is provided")
	}
}

// TestCompleteAuth_UnknownAccount verifies that an unknown account label
// returns an error result.
func TestCompleteAuth_UnknownAccount(t *testing.T) {
	defaultMock := &mockAuthCodeCred{}
	registry := auth.NewAccountRegistry()

	handler := HandleCompleteAuth(defaultMock, "/tmp/default-auth-record.json", registry, []string{"Calendars.ReadWrite"})

	redirectURL := "https://login.microsoftonline.com/common/oauth2/nativeclient?code=abc"
	req := buildCallToolRequest(map[string]any{
		"redirect_url": redirectURL,
		"account":      "nonexistent",
	})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for unknown account")
	}

	text := extractText(t, result)
	if !strings.Contains(text, "not found") {
		t.Errorf("error text %q should contain 'not found'", text)
	}

	// Default credential should not have been used.
	if defaultMock.exchangedURL != "" {
		t.Error("default credential should not have been used for unknown account")
	}
}
