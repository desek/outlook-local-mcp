// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains tests for the add_account MCP tool, verifying label
// validation, credential setup, Graph client creation, inline authentication
// with elicitation, and registry registration.
package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/config"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
)

// addAccountTestConfig returns a Config suitable for add_account tests.
func addAccountTestConfig(t *testing.T) config.Config {
	t.Helper()
	dir := t.TempDir()
	return config.Config{
		ClientID:       "d3590ed6-52b3-4102-aeff-aad2292ab01c",
		TenantID:       "common",
		AuthRecordPath: dir + "/auth_record.json",
		CacheName:      "test-cache",
		AuthMethod:     "browser",
		AccountsPath:   dir + "/accounts.json",
	}
}

// fakeSetupCredential is a mock credential factory for add_account tests. It
// returns nil credential and authenticator values, which is sufficient because
// the mock authenticate function intercepts before any real credential usage.
func fakeSetupCredential(label, _, _, _, cacheName, authRecordDir, _ string) (
	azcore.TokenCredential, auth.Authenticator, string, string, error,
) {
	return nil, nil, authRecordDir + "/" + label + "_auth_record.json", cacheName + "-" + label, nil
}

// mockAuthState returns an addAccountState with a mock authenticate function
// that always succeeds, and elicitation functions that return not-supported.
func mockAuthState() *addAccountState {
	return &addAccountState{
		setupCredential: fakeSetupCredential,
		authenticate: func(_ context.Context, _ auth.Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
			return azidentity.AuthenticationRecord{}, nil
		},
		urlElicit: func(_ context.Context, _, _, _ string) (*mcp.ElicitationResult, error) {
			return nil, mcpserver.ErrElicitationNotSupported
		},
		elicit: func(_ context.Context, _ mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
			return nil, mcpserver.ErrElicitationNotSupported
		},
		openBrowser: func(_ string) error { return nil },
		scopes:      []string{"Calendars.ReadWrite"},
	}
}

// TestNewAddAccountTool_HasRequiredLabel verifies that the add_account tool
// definition includes the required "label" parameter.
func TestNewAddAccountTool_HasRequiredLabel(t *testing.T) {
	tool := NewAddAccountTool()
	props, ok := tool.InputSchema.Properties["label"]
	if !ok {
		t.Fatal("expected 'label' parameter in tool definition")
	}
	propMap, ok := props.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any for label property, got %T", props)
	}
	if propMap["type"] != "string" {
		t.Errorf("label type = %v, want string", propMap["type"])
	}

	// Check that label is in the required list.
	found := false
	for _, req := range tool.InputSchema.Required {
		if req == "label" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'label' to be in required parameters")
	}
}

// TestNewAddAccountTool_HasOptionalParams verifies that the add_account tool
// definition includes the optional client_id, tenant_id, and auth_method
// parameters.
func TestNewAddAccountTool_HasOptionalParams(t *testing.T) {
	tool := NewAddAccountTool()
	for _, param := range []string{"client_id", "tenant_id", "auth_method"} {
		if _, ok := tool.InputSchema.Properties[param]; !ok {
			t.Errorf("expected %q parameter in tool definition", param)
		}
	}
}

// TestHandleAddAccount_Success verifies that adding a new account with a valid
// label succeeds and the account is registered.
func TestHandleAddAccount_Success(t *testing.T) {
	registry := auth.NewAccountRegistry()
	cfg := addAccountTestConfig(t)

	// Inject a mock Graph client factory to avoid real credential usage.
	client, srv := newTestGraphClient(t, nil)
	defer srv.Close()
	origFactory := graphClientFactory
	graphClientFactory = func(_ azcore.TokenCredential) (*msgraphsdk.GraphServiceClient, error) {
		return client, nil
	}
	defer func() { graphClientFactory = origFactory }()

	state := mockAuthState()
	handler := state.handleAddAccount(registry, cfg)
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
	if resp["added"] != true {
		t.Errorf("added = %v, want true", resp["added"])
	}
	if resp["label"] != "work" {
		t.Errorf("label = %v, want work", resp["label"])
	}

	// Verify account is in registry with identity fields populated.
	entry, exists := registry.Get("work")
	if !exists {
		t.Fatal("expected account 'work' in registry")
	}
	if entry.Client != client {
		t.Error("expected the mock client to be stored in registry entry")
	}
	if entry.ClientID != cfg.ClientID {
		t.Errorf("ClientID = %q, want %q", entry.ClientID, cfg.ClientID)
	}
	if entry.TenantID != cfg.TenantID {
		t.Errorf("TenantID = %q, want %q", entry.TenantID, cfg.TenantID)
	}
	if entry.AuthMethod != cfg.AuthMethod {
		t.Errorf("AuthMethod = %q, want %q", entry.AuthMethod, cfg.AuthMethod)
	}
}

// TestHandleAddAccount_DuplicateLabel verifies that adding an account with an
// existing label returns an error result.
func TestHandleAddAccount_DuplicateLabel(t *testing.T) {
	registry := auth.NewAccountRegistry()
	if err := registry.Add(&auth.AccountEntry{Label: "work"}); err != nil {
		t.Fatalf("registry.Add() error: %v", err)
	}

	cfg := addAccountTestConfig(t)
	state := mockAuthState()
	handler := state.handleAddAccount(registry, cfg)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{"label": "work"}

	result, err := handler(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error result for duplicate label")
	}
}

// TestHandleAddAccount_MissingLabel verifies that the handler returns an error
// result when the "label" parameter is not provided.
func TestHandleAddAccount_MissingLabel(t *testing.T) {
	registry := auth.NewAccountRegistry()
	cfg := addAccountTestConfig(t)

	state := mockAuthState()
	handler := state.handleAddAccount(registry, cfg)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{}

	result, err := handler(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error result when label is missing")
	}
}

// TestHandleAddAccount_GraphClientFactoryError verifies that a Graph client
// creation failure returns an error result.
func TestHandleAddAccount_GraphClientFactoryError(t *testing.T) {
	registry := auth.NewAccountRegistry()
	cfg := addAccountTestConfig(t)

	// Inject a failing Graph client factory.
	origFactory := graphClientFactory
	graphClientFactory = func(_ azcore.TokenCredential) (*msgraphsdk.GraphServiceClient, error) {
		return nil, fmt.Errorf("mock client error")
	}
	defer func() { graphClientFactory = origFactory }()

	state := mockAuthState()
	handler := state.handleAddAccount(registry, cfg)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{"label": "fail-client"}

	result, err := handler(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error result when Graph client creation fails")
	}
}

// TestHandleAddAccount_UsesConfigDefaults verifies that when optional parameters
// are omitted, the handler uses defaults from the server config.
func TestHandleAddAccount_UsesConfigDefaults(t *testing.T) {
	registry := auth.NewAccountRegistry()
	cfg := addAccountTestConfig(t)

	client, srv := newTestGraphClient(t, nil)
	defer srv.Close()
	origFactory := graphClientFactory
	graphClientFactory = func(_ azcore.TokenCredential) (*msgraphsdk.GraphServiceClient, error) {
		return client, nil
	}
	defer func() { graphClientFactory = origFactory }()

	state := mockAuthState()
	handler := state.handleAddAccount(registry, cfg)
	request := mcp.CallToolRequest{}
	// Only label, no optional params.
	request.Params.Arguments = map[string]any{"label": "defaults-test"}

	result, err := handler(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %v", result)
	}

	entry, exists := registry.Get("defaults-test")
	if !exists {
		t.Fatal("expected account 'defaults-test' in registry")
	}
	// Verify cache name uses the config's base name.
	wantCacheName := cfg.CacheName + "-defaults-test"
	if entry.CacheName != wantCacheName {
		t.Errorf("CacheName = %q, want %q", entry.CacheName, wantCacheName)
	}
}

// TestAddAccount_BrowserAuth_Success verifies that add_account with browser auth
// calls URL mode elicitation (RequestURLElicitation) and completes authentication
// inline during the tool call.
func TestAddAccount_BrowserAuth_Success(t *testing.T) {
	registry := auth.NewAccountRegistry()
	cfg := addAccountTestConfig(t)
	cfg.AuthMethod = "browser"

	client, srv := newTestGraphClient(t, nil)
	defer srv.Close()
	origFactory := graphClientFactory
	graphClientFactory = func(_ azcore.TokenCredential) (*msgraphsdk.GraphServiceClient, error) {
		return client, nil
	}
	defer func() { graphClientFactory = origFactory }()

	var urlElicitCalled bool
	var capturedURL string
	var capturedMessage string

	state := &addAccountState{
		openBrowser:     func(_ string) error { return nil },
		setupCredential: fakeSetupCredential,
		authenticate: func(_ context.Context, _ auth.Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
			return azidentity.AuthenticationRecord{}, nil
		},
		urlElicit: func(_ context.Context, _, url, message string) (*mcp.ElicitationResult, error) {
			urlElicitCalled = true
			capturedURL = url
			capturedMessage = message
			return &mcp.ElicitationResult{
				ElicitationResponse: mcp.ElicitationResponse{
					Action: mcp.ElicitationResponseActionAccept,
				},
			}, nil
		},
		elicit: func(_ context.Context, _ mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
			t.Error("form elicitation should not be called for browser auth")
			return nil, mcpserver.ErrElicitationNotSupported
		},
	}

	handler := state.handleAddAccount(registry, cfg)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{"label": "browser-test"}

	result, err := handler(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %v", result)
	}

	if !urlElicitCalled {
		t.Error("URL elicitation should have been called for browser auth")
	}
	if capturedURL != "https://login.microsoftonline.com" {
		t.Errorf("URL = %q, want %q", capturedURL, "https://login.microsoftonline.com")
	}
	if !strings.Contains(capturedMessage, "Authentication required") {
		t.Errorf("message = %q, want to contain 'Authentication required'", capturedMessage)
	}

	// Verify account was registered.
	if _, exists := registry.Get("browser-test"); !exists {
		t.Error("expected account 'browser-test' in registry")
	}

	// Verify response message indicates authentication completed.
	text := extractText(t, result)
	if !strings.Contains(text, "authenticated successfully") {
		t.Errorf("response = %q, want to contain 'authenticated successfully'", text)
	}
}

// TestAddAccount_DeviceCode_Success verifies that add_account with device_code
// auth calls form mode elicitation (RequestElicitation) to display the device
// code and completes authentication inline during the tool call.
func TestAddAccount_DeviceCode_Success(t *testing.T) {
	registry := auth.NewAccountRegistry()
	cfg := addAccountTestConfig(t)
	cfg.AuthMethod = "device_code"

	client, srv := newTestGraphClient(t, nil)
	defer srv.Close()
	origFactory := graphClientFactory
	graphClientFactory = func(_ azcore.TokenCredential) (*msgraphsdk.GraphServiceClient, error) {
		return client, nil
	}
	defer func() { graphClientFactory = origFactory }()

	var formElicitCalled bool
	var capturedElicitMessage string

	state := &addAccountState{
		openBrowser:     func(_ string) error { return nil },
		setupCredential: fakeSetupCredential,
		authenticate: func(ctx context.Context, _ auth.Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
			// Simulate device code prompt via channel, then sleep briefly
			// to ensure the select in authenticateDeviceCode picks the
			// deviceCodeCh case before p.done closes. Without this delay,
			// the goroutine returns immediately and p.done may win the
			// select race on CI where goroutine scheduling differs.
			if ch, ok := ctx.Value(auth.DeviceCodeMsgKey).(chan string); ok {
				select {
				case ch <- "To sign in, visit https://microsoft.com/devicelogin and enter code TEST123":
				default:
				}
			}
			time.Sleep(100 * time.Millisecond)
			return azidentity.AuthenticationRecord{}, nil
		},
		urlElicit: func(_ context.Context, _, _, _ string) (*mcp.ElicitationResult, error) {
			t.Error("URL elicitation should not be called for device_code auth")
			return nil, mcpserver.ErrElicitationNotSupported
		},
		elicit: func(_ context.Context, req mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
			formElicitCalled = true
			capturedElicitMessage = req.Params.Message
			return &mcp.ElicitationResult{
				ElicitationResponse: mcp.ElicitationResponse{
					Action:  mcp.ElicitationResponseActionAccept,
					Content: map[string]any{"acknowledged": true},
				},
			}, nil
		},
		pending: make(map[string]*pendingAccount),
	}

	handler := state.handleAddAccount(registry, cfg)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{
		"label":       "device-test",
		"auth_method": "device_code",
	}

	result, err := handler(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %v", result)
	}

	if !formElicitCalled {
		t.Error("form elicitation should have been called for device_code auth")
	}
	if !strings.Contains(capturedElicitMessage, "devicelogin") {
		t.Errorf("elicitation message = %q, want to contain device code prompt", capturedElicitMessage)
	}

	// Verify account was registered.
	if _, exists := registry.Get("device-test"); !exists {
		t.Error("expected account 'device-test' in registry")
	}
}

// TestAddAccount_AuthFailure verifies that when authentication fails during
// add_account, the tool returns an error result and does not register the account.
func TestAddAccount_AuthFailure(t *testing.T) {
	registry := auth.NewAccountRegistry()
	cfg := addAccountTestConfig(t)

	client, srv := newTestGraphClient(t, nil)
	defer srv.Close()
	origFactory := graphClientFactory
	graphClientFactory = func(_ azcore.TokenCredential) (*msgraphsdk.GraphServiceClient, error) {
		return client, nil
	}
	defer func() { graphClientFactory = origFactory }()

	state := &addAccountState{
		openBrowser:     func(_ string) error { return nil },
		setupCredential: fakeSetupCredential,
		authenticate: func(_ context.Context, _ auth.Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
			return azidentity.AuthenticationRecord{}, fmt.Errorf("authentication failed: user cancelled")
		},
		urlElicit: func(_ context.Context, _, _, _ string) (*mcp.ElicitationResult, error) {
			return nil, mcpserver.ErrElicitationNotSupported
		},
		elicit: func(_ context.Context, _ mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
			return nil, mcpserver.ErrElicitationNotSupported
		},
	}

	handler := state.handleAddAccount(registry, cfg)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{"label": "fail-auth"}

	result, err := handler(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error result when authentication fails")
	}

	text := extractText(t, result)
	if !strings.Contains(text, "failed to authenticate") {
		t.Errorf("error = %q, want to contain 'failed to authenticate'", text)
	}

	// Verify account was NOT registered.
	if _, exists := registry.Get("fail-auth"); exists {
		t.Error("account should not be registered when authentication fails")
	}
}

// TestAddAccount_AuthCode_ElicitationSuccess verifies the full auth_code flow:
// auth URL is generated, form elicitation returns the redirect URL, code is
// exchanged, and authentication completes successfully.
func TestAddAccount_AuthCode_ElicitationSuccess(t *testing.T) {
	var elicitCalled bool
	var capturedMessage string

	mock := &mockAuthCodeCred{}

	state := &addAccountState{
		openBrowser:     func(_ string) error { return nil },
		setupCredential: fakeSetupCredential,
		authenticate: func(_ context.Context, _ auth.Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
			t.Error("authenticate should not be called for auth_code flow")
			return azidentity.AuthenticationRecord{}, nil
		},
		urlElicit: func(_ context.Context, _, _, _ string) (*mcp.ElicitationResult, error) {
			t.Error("URL elicitation should not be called for auth_code flow")
			return nil, mcpserver.ErrElicitationNotSupported
		},
		elicit: func(_ context.Context, req mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
			elicitCalled = true
			capturedMessage = req.Params.Message
			return &mcp.ElicitationResult{
				ElicitationResponse: mcp.ElicitationResponse{
					Action: mcp.ElicitationResponseActionAccept,
					Content: map[string]any{
						"redirect_url": "https://login.microsoftonline.com/common/oauth2/nativeclient?code=testcode123",
					},
				},
			}, nil
		},
	}

	err := state.authenticateAuthCode(context.Background(), mock, t.TempDir()+"/auth.json", "test-account", slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !elicitCalled {
		t.Error("form elicitation should have been called for auth_code flow")
	}
	if !strings.Contains(capturedMessage, "browser window has opened") {
		t.Errorf("message = %q, want to contain browser instructions", capturedMessage)
	}
	if mock.exchangedURL != "https://login.microsoftonline.com/common/oauth2/nativeclient?code=testcode123" {
		t.Errorf("ExchangeCode called with %q, want redirect URL", mock.exchangedURL)
	}
}

// TestAddAccount_AuthCode_ElicitationNotSupported verifies that when
// elicitation is not supported, authenticateAuthCode returns an error
// with complete_auth instructions and the account label.
func TestAddAccount_AuthCode_ElicitationNotSupported(t *testing.T) {
	mock := &mockAuthCodeCred{}

	state := &addAccountState{
		openBrowser:     func(_ string) error { return nil },
		setupCredential: fakeSetupCredential,
		authenticate: func(_ context.Context, _ auth.Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
			return azidentity.AuthenticationRecord{}, nil
		},
		urlElicit: func(_ context.Context, _, _, _ string) (*mcp.ElicitationResult, error) {
			return nil, mcpserver.ErrElicitationNotSupported
		},
		elicit: func(_ context.Context, _ mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
			return nil, mcpserver.ErrElicitationNotSupported
		},
	}

	err := state.authenticateAuthCode(context.Background(), mock, t.TempDir()+"/auth.json", "test-account", slog.Default())
	if err == nil {
		t.Fatal("expected error when elicitation is not supported")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "complete_auth") {
		t.Errorf("error = %q, want to contain 'complete_auth' instructions", errMsg)
	}
	if !strings.Contains(errMsg, "elicitation not supported") {
		t.Errorf("error = %q, want to contain 'elicitation not supported'", errMsg)
	}
	if !strings.Contains(errMsg, "test-account") {
		t.Errorf("error = %q, want to contain account label", errMsg)
	}

	// ExchangeCode should NOT have been called.
	if mock.exchangedURL != "" {
		t.Errorf("ExchangeCode should not have been called, but was called with %q", mock.exchangedURL)
	}
}

// TestAddAccount_AuthCode_ExchangeFailure verifies that when ExchangeCode
// fails, authenticateAuthCode returns the exchange error.
func TestAddAccount_AuthCode_ExchangeFailure(t *testing.T) {
	mock := &mockAuthCodeCred{
		exchangeErr: fmt.Errorf("token exchange failed"),
	}

	state := &addAccountState{
		openBrowser:     func(_ string) error { return nil },
		setupCredential: fakeSetupCredential,
		authenticate: func(_ context.Context, _ auth.Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
			return azidentity.AuthenticationRecord{}, nil
		},
		urlElicit: func(_ context.Context, _, _, _ string) (*mcp.ElicitationResult, error) {
			return nil, mcpserver.ErrElicitationNotSupported
		},
		elicit: func(_ context.Context, _ mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
			return &mcp.ElicitationResult{
				ElicitationResponse: mcp.ElicitationResponse{
					Action: mcp.ElicitationResponseActionAccept,
					Content: map[string]any{
						"redirect_url": "https://login.microsoftonline.com/common/oauth2/nativeclient?code=badcode",
					},
				},
			}, nil
		},
	}

	err := state.authenticateAuthCode(context.Background(), mock, t.TempDir()+"/auth.json", "test-account", slog.Default())
	if err == nil {
		t.Fatal("expected error when exchange fails")
	}

	if !strings.Contains(err.Error(), "token exchange failed") {
		t.Errorf("error = %q, want to contain 'token exchange failed'", err.Error())
	}
}

// TestAddAccount_ElicitationFallback verifies that when elicitation is not
// supported by the MCP client, add_account falls back gracefully and still
// completes authentication.
func TestAddAccount_ElicitationFallback(t *testing.T) {
	registry := auth.NewAccountRegistry()
	cfg := addAccountTestConfig(t)

	client, srv := newTestGraphClient(t, nil)
	defer srv.Close()
	origFactory := graphClientFactory
	graphClientFactory = func(_ azcore.TokenCredential) (*msgraphsdk.GraphServiceClient, error) {
		return client, nil
	}
	defer func() { graphClientFactory = origFactory }()

	var urlElicitCalled bool

	state := &addAccountState{
		openBrowser:     func(_ string) error { return nil },
		setupCredential: fakeSetupCredential,
		authenticate: func(_ context.Context, _ auth.Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
			return azidentity.AuthenticationRecord{}, nil
		},
		urlElicit: func(_ context.Context, _, _, _ string) (*mcp.ElicitationResult, error) {
			urlElicitCalled = true
			return nil, mcpserver.ErrElicitationNotSupported
		},
		elicit: func(_ context.Context, _ mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
			return nil, mcpserver.ErrElicitationNotSupported
		},
	}

	handler := state.handleAddAccount(registry, cfg)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{"label": "fallback-test"}

	result, err := handler(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %v", result)
	}

	if !urlElicitCalled {
		t.Error("URL elicitation should have been attempted (then fallen back)")
	}

	// Verify account was registered despite elicitation fallback.
	if _, exists := registry.Get("fallback-test"); !exists {
		t.Error("expected account 'fallback-test' in registry")
	}
}

// TestAuthenticateAuthCode_ElicitationError_ReturnsAuthURL verifies that any
// elicitation error (not just ErrElicitationNotSupported) triggers the fallback
// with auth URL and complete_auth instructions.
func TestAuthenticateAuthCode_ElicitationError_ReturnsAuthURL(t *testing.T) {
	mock := &mockAuthCodeCred{}

	state := &addAccountState{
		openBrowser:     func(_ string) error { return nil },
		setupCredential: fakeSetupCredential,
		authenticate: func(_ context.Context, _ auth.Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
			return azidentity.AuthenticationRecord{}, nil
		},
		urlElicit: func(_ context.Context, _, _, _ string) (*mcp.ElicitationResult, error) {
			return nil, fmt.Errorf("elicitation request failed: Method not found")
		},
		elicit: func(_ context.Context, _ mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
			// Return a generic error (not ErrElicitationNotSupported).
			return nil, fmt.Errorf("elicitation request failed: Method not found")
		},
	}

	err := state.authenticateAuthCode(context.Background(), mock, t.TempDir()+"/auth.json", "my-work", slog.Default())
	if err == nil {
		t.Fatal("expected error when elicitation fails")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "complete_auth") {
		t.Errorf("error = %q, want to contain 'complete_auth' instructions", errMsg)
	}
	if !strings.Contains(errMsg, "browser") {
		t.Errorf("error = %q, want to mention browser", errMsg)
	}
	if !strings.Contains(errMsg, "my-work") {
		t.Errorf("error = %q, want to contain account label", errMsg)
	}

	// ExchangeCode should NOT have been called.
	if mock.exchangedURL != "" {
		t.Errorf("ExchangeCode should not have been called, but was called with %q", mock.exchangedURL)
	}
}

// TestAuthenticateDeviceCode_ElicitationError_ReturnsDeviceCode verifies that
// when elicitation fails, the device code is returned as a successful tool
// result text via DeviceCodeFallbackError, and pending state is stored so the
// goroutine keeps running.
func TestAuthenticateDeviceCode_ElicitationError_ReturnsDeviceCode(t *testing.T) {
	registry := auth.NewAccountRegistry()
	cfg := addAccountTestConfig(t)
	cfg.AuthMethod = "device_code"

	deviceCodeMsg := "To sign in, visit https://microsoft.com/devicelogin and enter code TEST456"

	state := &addAccountState{
		openBrowser:     func(_ string) error { return nil },
		setupCredential: fakeSetupCredential,
		authenticate: func(ctx context.Context, _ auth.Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
			// Simulate device code prompt via channel.
			if ch, ok := ctx.Value(auth.DeviceCodeMsgKey).(chan string); ok {
				select {
				case ch <- deviceCodeMsg:
				default:
				}
			}
			// Block until context is cancelled (simulating waiting for user).
			<-ctx.Done()
			return azidentity.AuthenticationRecord{}, ctx.Err()
		},
		urlElicit: func(_ context.Context, _, _, _ string) (*mcp.ElicitationResult, error) {
			return nil, fmt.Errorf("elicitation request failed: Method not found")
		},
		elicit: func(_ context.Context, _ mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
			return nil, fmt.Errorf("elicitation request failed: Method not found")
		},
		pending: make(map[string]*pendingAccount),
	}

	handler := state.handleAddAccount(registry, cfg)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{
		"label":       "dc-fallback",
		"auth_method": "device_code",
	}

	result, err := handler(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Result should be successful (not error) with the device code message.
	if result.IsError {
		t.Fatalf("expected successful tool result, got error: %v", result)
	}

	text := extractText(t, result)
	if !strings.Contains(text, "TEST456") {
		t.Errorf("result = %q, want to contain device code", text)
	}
	if !strings.Contains(text, "dc-fallback") {
		t.Errorf("result = %q, want to contain account label", text)
	}
	if !strings.Contains(text, "account_add") {
		t.Errorf("result = %q, want to contain 'account_add' instructions", text)
	}

	// Verify pending state is stored (goroutine NOT cancelled).
	state.pendingMu.Lock()
	p, exists := state.pending["dc-fallback"]
	state.pendingMu.Unlock()
	if !exists {
		t.Fatal("expected pending entry for 'dc-fallback' after elicitation failure")
	}
	// The done channel should still be open (goroutine still running).
	select {
	case <-p.done:
		t.Error("pending goroutine should still be running (done channel should be open)")
	default:
		// Expected: goroutine still running.
	}
}

// TestAuthenticateDeviceCode_ElicitationError_DoesNotBlock verifies that when
// elicitation fails, the function returns promptly instead of blocking for the
// ~5 minute device code timeout, and pending state is stored.
func TestAuthenticateDeviceCode_ElicitationError_DoesNotBlock(t *testing.T) {
	deviceCodeMsg := "To sign in, visit https://microsoft.com/devicelogin and enter code BLOCK1"

	state := &addAccountState{
		openBrowser:     func(_ string) error { return nil },
		setupCredential: fakeSetupCredential,
		authenticate: func(ctx context.Context, _ auth.Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
			if ch, ok := ctx.Value(auth.DeviceCodeMsgKey).(chan string); ok {
				select {
				case ch <- deviceCodeMsg:
				default:
				}
			}
			// Block until cancelled -- simulates the full 5-minute timeout.
			<-ctx.Done()
			return azidentity.AuthenticationRecord{}, ctx.Err()
		},
		urlElicit: func(_ context.Context, _, _, _ string) (*mcp.ElicitationResult, error) {
			return nil, mcpserver.ErrElicitationNotSupported
		},
		elicit: func(_ context.Context, _ mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
			return nil, mcpserver.ErrElicitationNotSupported
		},
		pending: make(map[string]*pendingAccount),
	}

	start := time.Now()
	err := state.authenticateDeviceCode(context.Background(), nil, nil, "", "device_code", "", "", "", "block-test", slog.Default())
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected DeviceCodeFallbackError")
	}

	var dcErr *DeviceCodeFallbackError
	if !errors.As(err, &dcErr) {
		t.Fatalf("expected DeviceCodeFallbackError, got %T: %v", err, err)
	}

	// Should return within a few seconds, not the 300-second timeout.
	if elapsed > 5*time.Second {
		t.Errorf("function took %v, expected prompt return (< 5s)", elapsed)
	}

	// Verify pending state exists after prompt return.
	state.pendingMu.Lock()
	_, exists := state.pending["block-test"]
	state.pendingMu.Unlock()
	if !exists {
		t.Error("expected pending entry for 'block-test' after elicitation failure")
	}
}

// TestAuthenticateBrowser_Timeout_DescriptiveError verifies that when browser
// auth times out after elicitation failure, the error message mentions the
// browser window and suggests retrying.
func TestAuthenticateBrowser_Timeout_DescriptiveError(t *testing.T) {
	state := &addAccountState{
		openBrowser:     func(_ string) error { return nil },
		setupCredential: fakeSetupCredential,
		authenticate: func(_ context.Context, _ auth.Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
			return azidentity.AuthenticationRecord{}, fmt.Errorf("context deadline exceeded")
		},
		urlElicit: func(_ context.Context, _, _, _ string) (*mcp.ElicitationResult, error) {
			return nil, fmt.Errorf("elicitation request failed: Method not found")
		},
		elicit: func(_ context.Context, _ mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
			return nil, fmt.Errorf("elicitation request failed: Method not found")
		},
	}

	err := state.authenticateBrowser(context.Background(), nil, "", slog.Default())
	if err == nil {
		t.Fatal("expected error when auth times out")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "browser window was opened") {
		t.Errorf("error = %q, want to mention browser window", errMsg)
	}
	if !strings.Contains(errMsg, "try again") {
		t.Errorf("error = %q, want to suggest retrying", errMsg)
	}
}

// TestAuthenticateAuthCode_ElicitationSupported_NoFallback verifies that when
// elicitation succeeds, the normal auth flow completes without triggering any
// fallback behavior (AC-5).
func TestAuthenticateAuthCode_ElicitationSupported_NoFallback(t *testing.T) {
	mock := &mockAuthCodeCred{}

	state := &addAccountState{
		openBrowser:     func(_ string) error { return nil },
		setupCredential: fakeSetupCredential,
		authenticate: func(_ context.Context, _ auth.Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
			t.Error("authenticate should not be called for auth_code flow")
			return azidentity.AuthenticationRecord{}, nil
		},
		urlElicit: func(_ context.Context, _, _, _ string) (*mcp.ElicitationResult, error) {
			t.Error("URL elicitation should not be called for auth_code flow")
			return nil, mcpserver.ErrElicitationNotSupported
		},
		elicit: func(_ context.Context, _ mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
			return &mcp.ElicitationResult{
				ElicitationResponse: mcp.ElicitationResponse{
					Action: mcp.ElicitationResponseActionAccept,
					Content: map[string]any{
						"redirect_url": "https://login.microsoftonline.com/common/oauth2/nativeclient?code=happycode",
					},
				},
			}, nil
		},
	}

	err := state.authenticateAuthCode(context.Background(), mock, t.TempDir()+"/auth.json", "happy-account", slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// ExchangeCode MUST have been called with the redirect URL.
	if mock.exchangedURL != "https://login.microsoftonline.com/common/oauth2/nativeclient?code=happycode" {
		t.Errorf("ExchangeCode called with %q, want redirect URL", mock.exchangedURL)
	}
}

// TestHandleAddAccount_PersistsConfig verifies that add_account writes the
// account identity configuration to accounts.json after successful
// authentication and registry addition.
func TestHandleAddAccount_PersistsConfig(t *testing.T) {
	registry := auth.NewAccountRegistry()
	cfg := addAccountTestConfig(t)

	client, srv := newTestGraphClient(t, nil)
	defer srv.Close()
	origFactory := graphClientFactory
	graphClientFactory = func(_ azcore.TokenCredential) (*msgraphsdk.GraphServiceClient, error) {
		return client, nil
	}
	defer func() { graphClientFactory = origFactory }()

	state := mockAuthState()
	handler := state.handleAddAccount(registry, cfg)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{
		"label":     "persist-test",
		"client_id": "custom-client",
		"tenant_id": "custom-tenant",
	}

	result, err := handler(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %v", result)
	}

	// Verify accounts.json contains the new account.
	accounts, loadErr := auth.LoadAccounts(cfg.AccountsPath)
	if loadErr != nil {
		t.Fatalf("LoadAccounts: %v", loadErr)
	}
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account in accounts.json, got %d", len(accounts))
	}
	if accounts[0].Label != "persist-test" {
		t.Errorf("Label = %q, want %q", accounts[0].Label, "persist-test")
	}
	if accounts[0].ClientID != "custom-client" {
		t.Errorf("ClientID = %q, want %q", accounts[0].ClientID, "custom-client")
	}
	if accounts[0].TenantID != "custom-tenant" {
		t.Errorf("TenantID = %q, want %q", accounts[0].TenantID, "custom-tenant")
	}
	if accounts[0].AuthMethod != cfg.AuthMethod {
		t.Errorf("AuthMethod = %q, want %q", accounts[0].AuthMethod, cfg.AuthMethod)
	}
}

// TestAuthenticateDeviceCode_ElicitationError_NoStderrDependency verifies
// that the device code fallback returns the code via tool result text without
// depending on stderr or notifications being visible (AC-6).
func TestAuthenticateDeviceCode_ElicitationError_NoStderrDependency(t *testing.T) {
	deviceCodeMsg := "To sign in, visit https://microsoft.com/devicelogin and enter code NODEP1"

	state := &addAccountState{
		openBrowser:     func(_ string) error { return nil },
		setupCredential: fakeSetupCredential,
		authenticate: func(ctx context.Context, _ auth.Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
			if ch, ok := ctx.Value(auth.DeviceCodeMsgKey).(chan string); ok {
				select {
				case ch <- deviceCodeMsg:
				default:
				}
			}
			<-ctx.Done()
			return azidentity.AuthenticationRecord{}, ctx.Err()
		},
		urlElicit: func(_ context.Context, _, _, _ string) (*mcp.ElicitationResult, error) {
			return nil, mcpserver.ErrElicitationNotSupported
		},
		elicit: func(_ context.Context, _ mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
			return nil, mcpserver.ErrElicitationNotSupported
		},
		pending: make(map[string]*pendingAccount),
	}

	err := state.authenticateDeviceCode(context.Background(), nil, nil, "", "device_code", "", "", "", "stderr-test", slog.Default())
	if err == nil {
		t.Fatal("expected DeviceCodeFallbackError")
	}

	var dcErr *DeviceCodeFallbackError
	if !errors.As(err, &dcErr) {
		t.Fatalf("expected DeviceCodeFallbackError, got %T: %v", err, err)
	}

	// The message must contain the device code -- this is the only channel
	// the user will see (tool result text). No stderr or notification needed.
	if !strings.Contains(dcErr.Message, "NODEP1") {
		t.Errorf("message = %q, want to contain device code", dcErr.Message)
	}
	if !strings.Contains(dcErr.Message, "stderr-test") {
		t.Errorf("message = %q, want to contain account label", dcErr.Message)
	}
}

// TestDeviceCode_PendingAuth_CompletedSuccessfully verifies that the second
// add_account call picks up a completed pending auth and registers the account.
func TestDeviceCode_PendingAuth_CompletedSuccessfully(t *testing.T) {
	registry := auth.NewAccountRegistry()
	cfg := addAccountTestConfig(t)
	cfg.AuthMethod = "device_code"

	client, srv := newTestGraphClient(t, nil)
	defer srv.Close()
	origFactory := graphClientFactory
	graphClientFactory = func(_ azcore.TokenCredential) (*msgraphsdk.GraphServiceClient, error) {
		return client, nil
	}
	defer func() { graphClientFactory = origFactory }()

	// authCompleted signals the mock authenticate to finish successfully.
	authCompleted := make(chan struct{})

	state := &addAccountState{
		openBrowser:     func(_ string) error { return nil },
		setupCredential: fakeSetupCredential,
		authenticate: func(ctx context.Context, _ auth.Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
			// Send device code, then wait for signal to complete.
			if ch, ok := ctx.Value(auth.DeviceCodeMsgKey).(chan string); ok {
				select {
				case ch <- "To sign in, visit https://microsoft.com/devicelogin and enter code PEND1":
				default:
				}
			}
			select {
			case <-authCompleted:
				return azidentity.AuthenticationRecord{}, nil
			case <-ctx.Done():
				return azidentity.AuthenticationRecord{}, ctx.Err()
			}
		},
		urlElicit: func(_ context.Context, _, _, _ string) (*mcp.ElicitationResult, error) {
			return nil, mcpserver.ErrElicitationNotSupported
		},
		elicit: func(_ context.Context, _ mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
			return nil, mcpserver.ErrElicitationNotSupported
		},
		pending: make(map[string]*pendingAccount),
	}

	handler := state.handleAddAccount(registry, cfg)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{
		"label":       "pending-ok",
		"auth_method": "device_code",
	}

	// 1st call: elicitation fails, stores pending, returns device code.
	result1, err := handler(context.Background(), request)
	if err != nil {
		t.Fatalf("1st call: unexpected error: %v", err)
	}
	if result1.IsError {
		t.Fatalf("1st call: unexpected error result: %v", result1)
	}
	text1 := extractText(t, result1)
	if !strings.Contains(text1, "PEND1") {
		t.Errorf("1st call: result = %q, want device code", text1)
	}

	// Simulate user completing auth in browser.
	close(authCompleted)

	// Wait briefly for goroutine to finish.
	state.pendingMu.Lock()
	p := state.pending["pending-ok"]
	state.pendingMu.Unlock()
	if p == nil {
		t.Fatal("expected pending entry after 1st call")
	}
	select {
	case <-p.done:
	case <-time.After(5 * time.Second):
		t.Fatal("auth goroutine did not complete in time")
	}

	// 2nd call: picks up completed pending auth, registers account.
	result2, err := handler(context.Background(), request)
	if err != nil {
		t.Fatalf("2nd call: unexpected error: %v", err)
	}
	if result2.IsError {
		t.Fatalf("2nd call: unexpected error result: %v", result2)
	}

	text2 := extractText(t, result2)
	var resp map[string]any
	if jsonErr := json.Unmarshal([]byte(text2), &resp); jsonErr != nil {
		t.Fatalf("2nd call: json.Unmarshal() error: %v", jsonErr)
	}
	if resp["added"] != true {
		t.Errorf("2nd call: added = %v, want true", resp["added"])
	}

	// Verify account is in registry.
	if _, exists := registry.Get("pending-ok"); !exists {
		t.Error("expected account 'pending-ok' in registry after 2nd call")
	}

	// Verify pending map is clean.
	state.pendingMu.Lock()
	pendingLen := len(state.pending)
	state.pendingMu.Unlock()
	if pendingLen != 0 {
		t.Errorf("pending map should be empty after successful pickup, got %d entries", pendingLen)
	}
}

// TestDeviceCode_PendingAuth_StillInProgress verifies that the second call
// while auth is still running returns an in-progress message.
func TestDeviceCode_PendingAuth_StillInProgress(t *testing.T) {
	registry := auth.NewAccountRegistry()
	cfg := addAccountTestConfig(t)
	cfg.AuthMethod = "device_code"

	state := &addAccountState{
		openBrowser:     func(_ string) error { return nil },
		setupCredential: fakeSetupCredential,
		authenticate: func(ctx context.Context, _ auth.Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
			if ch, ok := ctx.Value(auth.DeviceCodeMsgKey).(chan string); ok {
				select {
				case ch <- "To sign in, visit https://microsoft.com/devicelogin and enter code PROG1":
				default:
				}
			}
			// Block indefinitely until context is cancelled.
			<-ctx.Done()
			return azidentity.AuthenticationRecord{}, ctx.Err()
		},
		urlElicit: func(_ context.Context, _, _, _ string) (*mcp.ElicitationResult, error) {
			return nil, mcpserver.ErrElicitationNotSupported
		},
		elicit: func(_ context.Context, _ mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
			return nil, mcpserver.ErrElicitationNotSupported
		},
		pending: make(map[string]*pendingAccount),
	}

	handler := state.handleAddAccount(registry, cfg)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{
		"label":       "in-progress",
		"auth_method": "device_code",
	}

	// 1st call: elicitation fails, stores pending.
	result1, err := handler(context.Background(), request)
	if err != nil {
		t.Fatalf("1st call: unexpected error: %v", err)
	}
	if result1.IsError {
		t.Fatalf("1st call: unexpected error result: %v", result1)
	}

	// 2nd call: goroutine still running, should return in-progress.
	result2, err := handler(context.Background(), request)
	if err != nil {
		t.Fatalf("2nd call: unexpected error: %v", err)
	}
	if result2.IsError {
		t.Error("2nd call: expected non-error result for in-progress")
	}

	text2 := extractText(t, result2)
	if !strings.Contains(text2, "still in progress") {
		t.Errorf("2nd call: result = %q, want 'still in progress'", text2)
	}
}

// TestDeviceCode_PendingAuth_Failed verifies that the second call after a
// failed auth goroutine returns an error and cleans up the pending entry.
func TestDeviceCode_PendingAuth_Failed(t *testing.T) {
	registry := auth.NewAccountRegistry()
	cfg := addAccountTestConfig(t)
	cfg.AuthMethod = "device_code"

	// authFailed signals the mock authenticate to return an error.
	authFailed := make(chan struct{})

	state := &addAccountState{
		openBrowser:     func(_ string) error { return nil },
		setupCredential: fakeSetupCredential,
		authenticate: func(ctx context.Context, _ auth.Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
			if ch, ok := ctx.Value(auth.DeviceCodeMsgKey).(chan string); ok {
				select {
				case ch <- "To sign in, visit https://microsoft.com/devicelogin and enter code FAIL1":
				default:
				}
			}
			select {
			case <-authFailed:
				return azidentity.AuthenticationRecord{}, fmt.Errorf("token polling failed: user denied")
			case <-ctx.Done():
				return azidentity.AuthenticationRecord{}, ctx.Err()
			}
		},
		urlElicit: func(_ context.Context, _, _, _ string) (*mcp.ElicitationResult, error) {
			return nil, mcpserver.ErrElicitationNotSupported
		},
		elicit: func(_ context.Context, _ mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
			return nil, mcpserver.ErrElicitationNotSupported
		},
		pending: make(map[string]*pendingAccount),
	}

	handler := state.handleAddAccount(registry, cfg)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{
		"label":       "fail-pending",
		"auth_method": "device_code",
	}

	// 1st call: stores pending.
	result1, err := handler(context.Background(), request)
	if err != nil {
		t.Fatalf("1st call: unexpected error: %v", err)
	}
	if result1.IsError {
		t.Fatalf("1st call: unexpected error result: %v", result1)
	}

	// Trigger auth failure.
	close(authFailed)

	// Wait for goroutine to complete.
	state.pendingMu.Lock()
	p := state.pending["fail-pending"]
	state.pendingMu.Unlock()
	if p == nil {
		t.Fatal("expected pending entry after 1st call")
	}
	select {
	case <-p.done:
	case <-time.After(5 * time.Second):
		t.Fatal("auth goroutine did not complete in time")
	}

	// 2nd call: pending failed, returns error with failure reason.
	result2, err := handler(context.Background(), request)
	if err != nil {
		t.Fatalf("2nd call: unexpected error: %v", err)
	}
	if !result2.IsError {
		t.Error("2nd call: expected error result for failed pending auth")
	}

	text2 := extractText(t, result2)
	if !strings.Contains(text2, "user denied") {
		t.Errorf("2nd call: result = %q, want to contain failure reason", text2)
	}
	if !strings.Contains(text2, "try account_add again") {
		t.Errorf("2nd call: result = %q, want retry instructions", text2)
	}

	// Verify pending map is clean.
	state.pendingMu.Lock()
	pendingLen := len(state.pending)
	state.pendingMu.Unlock()
	if pendingLen != 0 {
		t.Errorf("pending map should be empty after failed pickup, got %d entries", pendingLen)
	}
}

// TestDeviceCode_PendingAuth_GoroutineNotCancelled verifies that when
// elicitation fails, the authenticate function's context is NOT cancelled
// and the goroutine keeps running.
func TestDeviceCode_PendingAuth_GoroutineNotCancelled(t *testing.T) {
	var ctxCancelled atomic.Bool

	state := &addAccountState{
		openBrowser:     func(_ string) error { return nil },
		setupCredential: fakeSetupCredential,
		authenticate: func(ctx context.Context, _ auth.Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
			if ch, ok := ctx.Value(auth.DeviceCodeMsgKey).(chan string); ok {
				select {
				case ch <- "To sign in, visit https://microsoft.com/devicelogin and enter code NCANC":
				default:
				}
			}
			// Monitor context cancellation.
			<-ctx.Done()
			ctxCancelled.Store(true)
			return azidentity.AuthenticationRecord{}, ctx.Err()
		},
		urlElicit: func(_ context.Context, _, _, _ string) (*mcp.ElicitationResult, error) {
			return nil, mcpserver.ErrElicitationNotSupported
		},
		elicit: func(_ context.Context, _ mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
			return nil, mcpserver.ErrElicitationNotSupported
		},
		pending: make(map[string]*pendingAccount),
	}

	err := state.authenticateDeviceCode(
		context.Background(), nil, nil, "", "device_code", "", "", "", "nocancel-test", slog.Default(),
	)
	if err == nil {
		t.Fatal("expected DeviceCodeFallbackError")
	}

	var dcErr *DeviceCodeFallbackError
	if !errors.As(err, &dcErr) {
		t.Fatalf("expected DeviceCodeFallbackError, got %T: %v", err, err)
	}

	// After handler returns, the goroutine context should NOT be cancelled.
	// Give a brief window to detect premature cancellation.
	time.Sleep(50 * time.Millisecond)
	if ctxCancelled.Load() {
		t.Error("authenticate context was cancelled, but goroutine should keep running")
	}
}

// TestDeviceCode_PendingAuth_ThirdCallAfterFailure verifies that after a
// pending auth fails, the third call starts a fresh authentication flow.
func TestDeviceCode_PendingAuth_ThirdCallAfterFailure(t *testing.T) {
	registry := auth.NewAccountRegistry()
	cfg := addAccountTestConfig(t)
	cfg.AuthMethod = "device_code"

	client, srv := newTestGraphClient(t, nil)
	defer srv.Close()
	origFactory := graphClientFactory
	graphClientFactory = func(_ azcore.TokenCredential) (*msgraphsdk.GraphServiceClient, error) {
		return client, nil
	}
	defer func() { graphClientFactory = origFactory }()

	var authenticateCount atomic.Int32
	// firstAuthFailed signals the first authenticate to fail.
	firstAuthFailed := make(chan struct{})
	// secondAuthComplete signals the second authenticate to succeed.
	secondAuthComplete := make(chan struct{})

	state := &addAccountState{
		openBrowser:     func(_ string) error { return nil },
		setupCredential: fakeSetupCredential,
		authenticate: func(ctx context.Context, _ auth.Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
			callNum := authenticateCount.Add(1)
			if ch, ok := ctx.Value(auth.DeviceCodeMsgKey).(chan string); ok {
				code := fmt.Sprintf("CODE%d", callNum)
				select {
				case ch <- "To sign in, visit https://microsoft.com/devicelogin and enter code " + code:
				default:
				}
			}
			if callNum == 1 {
				select {
				case <-firstAuthFailed:
					return azidentity.AuthenticationRecord{}, fmt.Errorf("auth timeout")
				case <-ctx.Done():
					return azidentity.AuthenticationRecord{}, ctx.Err()
				}
			}
			// Second authenticate call: wait for signal, then succeed.
			select {
			case <-secondAuthComplete:
				return azidentity.AuthenticationRecord{}, nil
			case <-ctx.Done():
				return azidentity.AuthenticationRecord{}, ctx.Err()
			}
		},
		urlElicit: func(_ context.Context, _, _, _ string) (*mcp.ElicitationResult, error) {
			return nil, mcpserver.ErrElicitationNotSupported
		},
		elicit: func(_ context.Context, _ mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
			return nil, mcpserver.ErrElicitationNotSupported
		},
		pending: make(map[string]*pendingAccount),
	}

	handler := state.handleAddAccount(registry, cfg)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{
		"label":       "third-call",
		"auth_method": "device_code",
	}

	// 1st call: stores pending.
	_, err := handler(context.Background(), request)
	if err != nil {
		t.Fatalf("1st call: unexpected error: %v", err)
	}

	// Fail the first auth goroutine.
	close(firstAuthFailed)
	state.pendingMu.Lock()
	p := state.pending["third-call"]
	state.pendingMu.Unlock()
	if p != nil {
		select {
		case <-p.done:
		case <-time.After(5 * time.Second):
			t.Fatal("1st auth goroutine did not complete")
		}
	}

	// 2nd call: picks up failed pending, returns error.
	result2, err := handler(context.Background(), request)
	if err != nil {
		t.Fatalf("2nd call: unexpected error: %v", err)
	}
	if !result2.IsError {
		t.Error("2nd call: expected error result for failed pending")
	}

	// 3rd call: should start a fresh auth flow (new authenticate call).
	result3, err := handler(context.Background(), request)
	if err != nil {
		t.Fatalf("3rd call: unexpected error: %v", err)
	}
	if result3.IsError {
		t.Fatalf("3rd call: unexpected error result: %v", result3)
	}

	// Verify a second authenticate call was made.
	if authenticateCount.Load() < 2 {
		t.Errorf("expected at least 2 authenticate calls, got %d", authenticateCount.Load())
	}

	// The 3rd call should have returned a new device code (fresh flow).
	text3 := extractText(t, result3)
	if !strings.Contains(text3, "CODE2") {
		t.Errorf("3rd call: result = %q, want new device code 'CODE2'", text3)
	}
}

// TestDeviceCode_ElicitationSupported_NoPendingState verifies that when
// elicitation succeeds, no pending entry is created and the account
// registers inline.
func TestDeviceCode_ElicitationSupported_NoPendingState(t *testing.T) {
	registry := auth.NewAccountRegistry()
	cfg := addAccountTestConfig(t)
	cfg.AuthMethod = "device_code"

	client, srv := newTestGraphClient(t, nil)
	defer srv.Close()
	origFactory := graphClientFactory
	graphClientFactory = func(_ azcore.TokenCredential) (*msgraphsdk.GraphServiceClient, error) {
		return client, nil
	}
	defer func() { graphClientFactory = origFactory }()

	state := &addAccountState{
		openBrowser:     func(_ string) error { return nil },
		setupCredential: fakeSetupCredential,
		authenticate: func(ctx context.Context, _ auth.Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
			if ch, ok := ctx.Value(auth.DeviceCodeMsgKey).(chan string); ok {
				select {
				case ch <- "To sign in, visit https://microsoft.com/devicelogin and enter code ELICIT1":
				default:
				}
			}
			return azidentity.AuthenticationRecord{}, nil
		},
		urlElicit: func(_ context.Context, _, _, _ string) (*mcp.ElicitationResult, error) {
			return nil, mcpserver.ErrElicitationNotSupported
		},
		elicit: func(_ context.Context, _ mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
			// Elicitation succeeds.
			return &mcp.ElicitationResult{
				ElicitationResponse: mcp.ElicitationResponse{
					Action:  mcp.ElicitationResponseActionAccept,
					Content: map[string]any{"acknowledged": true},
				},
			}, nil
		},
		pending: make(map[string]*pendingAccount),
	}

	handler := state.handleAddAccount(registry, cfg)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{
		"label":       "elicit-ok",
		"auth_method": "device_code",
	}

	result, err := handler(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %v", result)
	}

	// Verify account was registered inline.
	if _, exists := registry.Get("elicit-ok"); !exists {
		t.Error("expected account 'elicit-ok' in registry")
	}

	// Verify no pending entries were created.
	state.pendingMu.Lock()
	pendingLen := len(state.pending)
	state.pendingMu.Unlock()
	if pendingLen != 0 {
		t.Errorf("pending map should be empty when elicitation succeeds, got %d entries", pendingLen)
	}
}

// TestBrowserAuth_NoPendingState verifies that browser auth does not create
// pending entries.
func TestBrowserAuth_NoPendingState(t *testing.T) {
	registry := auth.NewAccountRegistry()
	cfg := addAccountTestConfig(t)
	cfg.AuthMethod = "browser"

	client, srv := newTestGraphClient(t, nil)
	defer srv.Close()
	origFactory := graphClientFactory
	graphClientFactory = func(_ azcore.TokenCredential) (*msgraphsdk.GraphServiceClient, error) {
		return client, nil
	}
	defer func() { graphClientFactory = origFactory }()

	state := &addAccountState{
		openBrowser:     func(_ string) error { return nil },
		setupCredential: fakeSetupCredential,
		authenticate: func(_ context.Context, _ auth.Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
			return azidentity.AuthenticationRecord{}, nil
		},
		urlElicit: func(_ context.Context, _, _, _ string) (*mcp.ElicitationResult, error) {
			return &mcp.ElicitationResult{
				ElicitationResponse: mcp.ElicitationResponse{
					Action: mcp.ElicitationResponseActionAccept,
				},
			}, nil
		},
		elicit: func(_ context.Context, _ mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
			return nil, mcpserver.ErrElicitationNotSupported
		},
		pending: make(map[string]*pendingAccount),
	}

	handler := state.handleAddAccount(registry, cfg)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{"label": "browser-pending"}

	result, err := handler(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %v", result)
	}

	// Verify account was registered.
	if _, exists := registry.Get("browser-pending"); !exists {
		t.Error("expected account 'browser-pending' in registry")
	}

	// Verify no pending entries.
	state.pendingMu.Lock()
	pendingLen := len(state.pending)
	state.pendingMu.Unlock()
	if pendingLen != 0 {
		t.Errorf("pending map should be empty for browser auth, got %d entries", pendingLen)
	}
}

// TestAuthCodeAuth_NoPendingState verifies that auth_code auth does not create
// pending entries. Tests through authenticateInline with a mock AuthCodeFlow
// credential since the handler calls SetupCredentialForAccount which creates
// real credentials unsuitable for unit tests.
func TestAuthCodeAuth_NoPendingState(t *testing.T) {
	mock := &mockAuthCodeCred{}

	state := &addAccountState{
		openBrowser:     func(_ string) error { return nil },
		setupCredential: fakeSetupCredential,
		authenticate: func(_ context.Context, _ auth.Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
			return azidentity.AuthenticationRecord{}, nil
		},
		urlElicit: func(_ context.Context, _, _, _ string) (*mcp.ElicitationResult, error) {
			return nil, mcpserver.ErrElicitationNotSupported
		},
		elicit: func(_ context.Context, _ mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
			return &mcp.ElicitationResult{
				ElicitationResponse: mcp.ElicitationResponse{
					Action: mcp.ElicitationResponseActionAccept,
					Content: map[string]any{
						"redirect_url": "https://login.microsoftonline.com/common/oauth2/nativeclient?code=testcode",
					},
				},
			}, nil
		},
		pending: make(map[string]*pendingAccount),
	}

	err := state.authenticateInline(
		context.Background(), nil, mock, t.TempDir()+"/auth.json",
		"auth_code", "", "", "", "authcode-pending", slog.Default(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify no pending entries.
	state.pendingMu.Lock()
	pendingLen := len(state.pending)
	state.pendingMu.Unlock()
	if pendingLen != 0 {
		t.Errorf("pending map should be empty for auth_code auth, got %d entries", pendingLen)
	}
}

// TestDeviceCode_PendingAuth_Timeout verifies that a pending auth goroutine
// that times out is cleaned up on the next call, allowing a fresh retry.
func TestDeviceCode_PendingAuth_Timeout(t *testing.T) {
	registry := auth.NewAccountRegistry()
	cfg := addAccountTestConfig(t)
	cfg.AuthMethod = "device_code"

	var authenticateCount atomic.Int32

	state := &addAccountState{
		openBrowser:     func(_ string) error { return nil },
		setupCredential: fakeSetupCredential,
		authenticate: func(ctx context.Context, _ auth.Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
			authenticateCount.Add(1)
			if ch, ok := ctx.Value(auth.DeviceCodeMsgKey).(chan string); ok {
				select {
				case ch <- "To sign in, visit https://microsoft.com/devicelogin and enter code TIMEOUT1":
				default:
				}
			}
			// Block until context expires (simulates nobody completing the flow).
			<-ctx.Done()
			return azidentity.AuthenticationRecord{}, ctx.Err()
		},
		urlElicit: func(_ context.Context, _, _, _ string) (*mcp.ElicitationResult, error) {
			return nil, mcpserver.ErrElicitationNotSupported
		},
		elicit: func(_ context.Context, _ mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
			return nil, mcpserver.ErrElicitationNotSupported
		},
		pending: make(map[string]*pendingAccount),
	}

	// Directly create a pending entry with a very short timeout to simulate
	// the 300s timeout expiring.
	shortCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	deviceCodeCh := make(chan string, 1)
	shortCtx = context.WithValue(shortCtx, auth.DeviceCodeMsgKey, deviceCodeCh)

	p := &pendingAccount{
		cred:       nil,
		authMethod: "device_code",
		cancel:     cancel,
		done:       make(chan struct{}),
	}
	go func() {
		defer close(p.done)
		_, p.err = state.authenticate(shortCtx, nil, "", state.scopes)
	}()

	// Drain the device code from the channel so the goroutine proceeds.
	select {
	case <-deviceCodeCh:
	case <-time.After(2 * time.Second):
		t.Fatal("device code not received")
	}

	state.storePending("timeout-test", p)

	// Wait for the short context to expire and goroutine to finish.
	select {
	case <-p.done:
	case <-time.After(5 * time.Second):
		t.Fatal("goroutine did not complete after timeout")
	}

	// Next handler call should clean up the timed-out pending entry.
	handler := state.handleAddAccount(registry, cfg)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{
		"label":       "timeout-test",
		"auth_method": "device_code",
	}

	result, err := handler(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error result for timed-out pending auth")
	}

	text := extractText(t, result)
	if !strings.Contains(text, "failed") || !strings.Contains(text, "try account_add again") {
		t.Errorf("result = %q, want timeout failure with retry instructions", text)
	}

	// Verify pending entry was cleaned up.
	state.pendingMu.Lock()
	pendingLen := len(state.pending)
	state.pendingMu.Unlock()
	if pendingLen != 0 {
		t.Errorf("pending map should be empty after timeout cleanup, got %d entries", pendingLen)
	}
}

// TestAddAccount_PersistsUPN verifies the AC-1 contract: after account_add
// completes, the corresponding entry in accounts.json contains a non-empty
// "upn" value matching the identity resolved from the Graph /me endpoint.
//
// HandleAddAccount cannot be exercised end-to-end without a real interactive
// authentication flow and a live Graph client. Instead, this test drives the
// closest testable surface: it seeds accounts.json with a freshly added
// account (UPN empty, as add_account writes it before /me resolves), mirrors
// what HandleAddAccount does after authentication (entry.Email is populated
// from the Graph /me response), and then invokes
// auth.EnsureEmailAndPersistUPN — the same helper HandleAddAccount calls on
// the success path at add_account.go:256,340. The assertion verifies that
// the persisted accounts.json now carries the resolved UPN.
func TestAddAccount_PersistsUPN(t *testing.T) {
	dir := t.TempDir()
	accountsPath := dir + "/accounts.json"

	// Seed accounts.json as HandleAddAccount does right after label
	// validation and before /me resolution: the record exists, but UPN is
	// empty and will be backfilled once the Graph /me lookup succeeds.
	seed := []auth.AccountConfig{
		{Label: "work", ClientID: "cid", TenantID: "tid", AuthMethod: "browser"},
	}
	if err := auth.SaveAccounts(accountsPath, seed); err != nil {
		t.Fatalf("SaveAccounts: %v", err)
	}

	// Simulate the post-authentication state: EnsureEmail has already set
	// entry.Email from Graph /me. Client is nil here so EnsureEmail
	// early-returns, exercising only the persistence branch — which is the
	// behavior under test.
	entry := &auth.AccountEntry{Label: "work", Email: "alice@contoso.com"}
	auth.EnsureEmailAndPersistUPN(context.Background(), entry, accountsPath)

	got, err := auth.LoadAccounts(accountsPath)
	if err != nil {
		t.Fatalf("LoadAccounts: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("accounts = %d, want 1", len(got))
	}
	if got[0].UPN != "alice@contoso.com" {
		t.Errorf("persisted UPN = %q, want %q", got[0].UPN, "alice@contoso.com")
	}
}
