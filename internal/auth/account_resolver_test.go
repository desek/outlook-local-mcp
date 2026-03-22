package auth

import (
	"context"
	"fmt"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
)

// newTestRegistry creates a registry with the given account labels. Each
// account gets a non-nil Client and Authenticated == true so that
// WithGraphClient receives a valid pointer and the account resolver
// considers the accounts for auto-selection.
func newTestRegistry(labels ...string) *AccountRegistry {
	reg := NewAccountRegistry()
	for _, label := range labels {
		_ = reg.Add(&AccountEntry{
			Label:         label,
			Client:        &msgraphsdk.GraphServiceClient{},
			Authenticated: true,
		})
	}
	return reg
}

// newTestResolverState creates an accountResolverState with the given
// registry and mock elicit function for testing.
func newTestResolverState(registry *AccountRegistry, elicit elicitFunc) *accountResolverState {
	return &accountResolverState{
		registry: registry,
		elicit:   elicit,
	}
}

// makeRequest creates a CallToolRequest with the given arguments map.
func makeRequest(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "test_tool",
			Arguments: args,
		},
	}
}

// passthrough is a no-op handler that returns success for middleware tests.
func passthrough(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultText("ok"), nil
}

// noElicit is an elicit function that should never be called. If called,
// it fails the test.
func noElicit(t *testing.T) elicitFunc {
	t.Helper()
	return func(_ context.Context, _ mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
		t.Fatal("elicit should not have been called")
		return nil, nil
	}
}

// TestAccountResolver_SingleAccount verifies that when exactly one account
// is registered, it is auto-selected without elicitation.
func TestAccountResolver_SingleAccount(t *testing.T) {
	reg := newTestRegistry("default")
	state := newTestResolverState(reg, noElicit(t))
	handler := state.middleware(passthrough)

	result, err := handler(context.Background(), makeRequest(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result)
	}
}

// TestAccountResolver_ExplicitAccount verifies that an explicit "account"
// parameter selects the correct account from the registry.
func TestAccountResolver_ExplicitAccount(t *testing.T) {
	reg := newTestRegistry("work", "personal")
	var resolvedClient *msgraphsdk.GraphServiceClient

	state := newTestResolverState(reg, noElicit(t))
	handler := state.middleware(func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		client, ok := GraphClientFromContext(ctx)
		if !ok {
			t.Fatal("no client in context")
		}
		resolvedClient = client
		return mcp.NewToolResultText("ok"), nil
	})

	workEntry, _ := reg.Get("work")
	result, err := handler(context.Background(), makeRequest(map[string]any{"account": "work"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result)
	}
	if resolvedClient != workEntry.Client {
		t.Error("resolved client does not match expected 'work' account")
	}
}

// TestAccountResolver_ExplicitAccountNotFound verifies that an explicit
// "account" parameter with a non-existent label returns a tool error.
func TestAccountResolver_ExplicitAccountNotFound(t *testing.T) {
	reg := newTestRegistry("default")
	state := newTestResolverState(reg, noElicit(t))
	handler := state.middleware(passthrough)

	result, err := handler(context.Background(), makeRequest(map[string]any{"account": "nonexistent"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected tool error for non-existent account")
	}
	text := extractResultText(result)
	if text == "" || !contains(text, "not found") {
		t.Errorf("expected 'not found' in error, got: %s", text)
	}
}

// TestAccountResolver_MultipleAccountsTriggersElicitation verifies that
// when multiple accounts exist and no "account" param is provided, the
// middleware calls the elicitation function.
func TestAccountResolver_MultipleAccountsTriggersElicitation(t *testing.T) {
	reg := newTestRegistry("work", "personal")
	elicitCalled := false

	elicit := func(_ context.Context, req mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
		elicitCalled = true
		// Verify the request contains the account labels as enum.
		schema, ok := req.Params.RequestedSchema.(map[string]any)
		if !ok {
			t.Fatal("expected schema to be a map")
		}
		props, ok := schema["properties"].(map[string]any)
		if !ok {
			t.Fatal("expected properties in schema")
		}
		accountProp, ok := props["account"].(map[string]any)
		if !ok {
			t.Fatal("expected account property in schema")
		}
		enumVals, ok := accountProp["enum"].([]string)
		if !ok {
			t.Fatal("expected enum to be []string")
		}
		if len(enumVals) != 2 {
			t.Errorf("expected 2 enum values, got %d", len(enumVals))
		}

		return &mcp.ElicitationResult{
			ElicitationResponse: mcp.ElicitationResponse{
				Action: mcp.ElicitationResponseActionAccept,
				Content: map[string]any{
					"account": "work",
				},
			},
		}, nil
	}

	state := newTestResolverState(reg, elicit)
	handler := state.middleware(passthrough)

	result, err := handler(context.Background(), makeRequest(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result)
	}
	if !elicitCalled {
		t.Error("elicitation was not triggered")
	}
}

// TestAccountResolver_ElicitationAccept verifies that accepting the
// elicitation prompt selects the chosen account and injects it into context.
func TestAccountResolver_ElicitationAccept(t *testing.T) {
	reg := newTestRegistry("work", "personal")
	workEntry, _ := reg.Get("work")

	elicit := func(_ context.Context, _ mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
		return &mcp.ElicitationResult{
			ElicitationResponse: mcp.ElicitationResponse{
				Action: mcp.ElicitationResponseActionAccept,
				Content: map[string]any{
					"account": "work",
				},
			},
		}, nil
	}

	var resolvedClient *msgraphsdk.GraphServiceClient
	state := newTestResolverState(reg, elicit)
	handler := state.middleware(func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		client, ok := GraphClientFromContext(ctx)
		if !ok {
			t.Fatal("no client in context")
		}
		resolvedClient = client
		return mcp.NewToolResultText("ok"), nil
	})

	result, err := handler(context.Background(), makeRequest(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result)
	}
	if resolvedClient != workEntry.Client {
		t.Error("resolved client does not match expected 'work' account")
	}
}

// TestAccountResolver_ElicitationDecline verifies that declining the
// elicitation prompt returns an appropriate error.
func TestAccountResolver_ElicitationDecline(t *testing.T) {
	reg := newTestRegistry("work", "personal")

	elicit := func(_ context.Context, _ mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
		return &mcp.ElicitationResult{
			ElicitationResponse: mcp.ElicitationResponse{
				Action: mcp.ElicitationResponseActionDecline,
			},
		}, nil
	}

	state := newTestResolverState(reg, elicit)
	handler := state.middleware(passthrough)

	result, err := handler(context.Background(), makeRequest(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected tool error for declined elicitation")
	}
	text := extractResultText(result)
	if !contains(text, "declined") {
		t.Errorf("expected 'declined' in error, got: %s", text)
	}
}

// TestAccountResolver_ElicitationCancel verifies that cancelling the
// elicitation prompt returns an appropriate error.
func TestAccountResolver_ElicitationCancel(t *testing.T) {
	reg := newTestRegistry("work", "personal")

	elicit := func(_ context.Context, _ mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
		return &mcp.ElicitationResult{
			ElicitationResponse: mcp.ElicitationResponse{
				Action: mcp.ElicitationResponseActionCancel,
			},
		}, nil
	}

	state := newTestResolverState(reg, elicit)
	handler := state.middleware(passthrough)

	result, err := handler(context.Background(), makeRequest(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected tool error for cancelled elicitation")
	}
	text := extractResultText(result)
	if !contains(text, "cancelled") {
		t.Errorf("expected 'cancelled' in error, got: %s", text)
	}
}

// TestAccountResolver_ElicitationNotSupported verifies that when
// ErrElicitationNotSupported is returned (one of many possible elicitation
// errors), the middleware falls back to the "default" account.
func TestAccountResolver_ElicitationNotSupported(t *testing.T) {
	reg := newTestRegistry("default", "personal")
	defaultEntry, _ := reg.Get("default")

	elicit := func(_ context.Context, _ mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
		return nil, mcpserver.ErrElicitationNotSupported
	}

	var resolvedClient *msgraphsdk.GraphServiceClient
	state := newTestResolverState(reg, elicit)
	handler := state.middleware(func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		client, ok := GraphClientFromContext(ctx)
		if !ok {
			t.Fatal("no client in context")
		}
		resolvedClient = client
		return mcp.NewToolResultText("ok"), nil
	})

	result, err := handler(context.Background(), makeRequest(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result)
	}
	if resolvedClient != defaultEntry.Client {
		t.Error("resolved client does not match expected 'default' account")
	}
}

// TestAccountResolver_ZeroAccounts verifies that the middleware returns an
// error when no accounts are registered.
func TestAccountResolver_ZeroAccounts(t *testing.T) {
	reg := NewAccountRegistry()
	state := newTestResolverState(reg, noElicit(t))
	handler := state.middleware(passthrough)

	result, err := handler(context.Background(), makeRequest(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected tool error for zero accounts")
	}
	text := extractResultText(result)
	if !contains(text, "no authenticated accounts") {
		t.Errorf("expected 'no authenticated accounts' in error, got: %s", text)
	}
}

// TestAccountResolver_ElicitationNotSupportedNoDefault verifies that when
// elicitation fails and no "default" account exists, an error is returned
// that lists available accounts and hints about the account parameter.
func TestAccountResolver_ElicitationNotSupportedNoDefault(t *testing.T) {
	reg := newTestRegistry("work", "personal")

	elicit := func(_ context.Context, _ mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
		return nil, mcpserver.ErrElicitationNotSupported
	}

	state := newTestResolverState(reg, elicit)
	handler := state.middleware(passthrough)

	result, err := handler(context.Background(), makeRequest(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected tool error when no default account and elicitation unsupported")
	}
	text := extractResultText(result)
	if !contains(text, "account") || !contains(text, "parameter") {
		t.Errorf("expected account parameter hint in error, got: %s", text)
	}
	if !contains(text, "personal") || !contains(text, "work") {
		t.Errorf("expected account labels in error, got: %s", text)
	}
}

// TestAccountResolver_AccountAuthInjected verifies that WithAccountAuth
// injects the correct AccountAuth into context after resolution.
func TestAccountResolver_AccountAuthInjected(t *testing.T) {
	reg := newTestRegistry("default")
	state := newTestResolverState(reg, noElicit(t))

	handler := state.middleware(func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		auth, ok := AccountAuthFromContext(ctx)
		if !ok {
			t.Fatal("no AccountAuth in context")
		}
		if auth.AuthMethod != "browser" {
			t.Errorf("expected auth method 'browser', got %q", auth.AuthMethod)
		}
		return mcp.NewToolResultText("ok"), nil
	})

	result, err := handler(context.Background(), makeRequest(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result)
	}
}

// mockAuthCodeFlowAuthenticator implements both Authenticator and AuthCodeFlow
// for testing inferAuthMethod.
type mockAuthCodeFlowAuthenticator struct{}

func (m *mockAuthCodeFlowAuthenticator) Authenticate(_ context.Context, _ *policy.TokenRequestOptions) (azidentity.AuthenticationRecord, error) {
	return azidentity.AuthenticationRecord{}, nil
}

func (m *mockAuthCodeFlowAuthenticator) AuthCodeURL(_ context.Context, _ []string) (string, error) {
	return "", nil
}

func (m *mockAuthCodeFlowAuthenticator) ExchangeCode(_ context.Context, _ string, _ []string) error {
	return nil
}

// mockBrowserAuthenticator implements only Authenticator (no AuthCodeFlow)
// for testing inferAuthMethod.
type mockBrowserAuthenticator struct{}

func (m *mockBrowserAuthenticator) Authenticate(_ context.Context, _ *policy.TokenRequestOptions) (azidentity.AuthenticationRecord, error) {
	return azidentity.AuthenticationRecord{}, nil
}

// TestInferAuthMethod_AuthCodeFlow verifies that inferAuthMethod returns
// "auth_code" when the entry's Authenticator implements AuthCodeFlow.
func TestInferAuthMethod_AuthCodeFlow(t *testing.T) {
	entry := &AccountEntry{
		Authenticator: &mockAuthCodeFlowAuthenticator{},
	}
	got := inferAuthMethod(entry)
	if got != "auth_code" {
		t.Errorf("inferAuthMethod() = %q, want %q", got, "auth_code")
	}
}

// TestInferAuthMethod_Browser verifies that inferAuthMethod returns "browser"
// when the entry's Authenticator does NOT implement AuthCodeFlow.
func TestInferAuthMethod_Browser(t *testing.T) {
	entry := &AccountEntry{
		Authenticator: &mockBrowserAuthenticator{},
	}
	got := inferAuthMethod(entry)
	if got != "browser" {
		t.Errorf("inferAuthMethod() = %q, want %q", got, "browser")
	}
}

// TestInferAuthMethod_NilAuthenticator verifies that inferAuthMethod returns
// "browser" when the entry's Authenticator is nil.
func TestInferAuthMethod_NilAuthenticator(t *testing.T) {
	entry := &AccountEntry{
		Authenticator: nil,
	}
	got := inferAuthMethod(entry)
	if got != "browser" {
		t.Errorf("inferAuthMethod() = %q, want %q", got, "browser")
	}
}

// TestElicitAccountSelection_AnyError_FallsBackToDefault verifies that any
// elicitation error (not just ErrElicitationNotSupported) triggers fallback
// to the "default" account. This covers Claude Desktop's "Method not found"
// JSON-RPC error.
func TestElicitAccountSelection_AnyError_FallsBackToDefault(t *testing.T) {
	reg := newTestRegistry("default", "personal")
	defaultEntry, _ := reg.Get("default")

	elicit := func(_ context.Context, _ mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
		return nil, fmt.Errorf("elicitation request failed: Method not found")
	}

	var resolvedClient *msgraphsdk.GraphServiceClient
	state := newTestResolverState(reg, elicit)
	handler := state.middleware(func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		client, ok := GraphClientFromContext(ctx)
		if !ok {
			t.Fatal("no client in context")
		}
		resolvedClient = client
		return mcp.NewToolResultText("ok"), nil
	})

	result, err := handler(context.Background(), makeRequest(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result)
	}
	if resolvedClient != defaultEntry.Client {
		t.Error("resolved client does not match expected 'default' account")
	}
}

// TestElicitAccountSelection_AnyError_NoDefault_ReturnsAccountList verifies
// that when elicitation fails and no "default" account exists, the error
// message lists available account labels and hints about the account parameter.
func TestElicitAccountSelection_AnyError_NoDefault_ReturnsAccountList(t *testing.T) {
	reg := newTestRegistry("work", "personal")

	elicit := func(_ context.Context, _ mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
		return nil, fmt.Errorf("elicitation request failed: Method not found")
	}

	state := newTestResolverState(reg, elicit)
	handler := state.middleware(passthrough)

	result, err := handler(context.Background(), makeRequest(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected tool error when no default account and elicitation fails")
	}
	text := extractResultText(result)
	if !contains(text, "personal") || !contains(text, "work") {
		t.Errorf("expected account labels in error, got: %s", text)
	}
	if !contains(text, "'account' parameter") {
		t.Errorf("expected account parameter hint in error, got: %s", text)
	}
}

// TestResolveAccount_OnlyAuthenticatedConsidered verifies that unauthenticated
// accounts are excluded from auto-selection. When 1 authenticated and 1
// unauthenticated account exist, the authenticated account is auto-selected
// without elicitation.
func TestResolveAccount_OnlyAuthenticatedConsidered(t *testing.T) {
	reg := NewAccountRegistry()
	_ = reg.Add(&AccountEntry{
		Label:         "default",
		Client:        &msgraphsdk.GraphServiceClient{},
		Authenticated: true,
	})
	_ = reg.Add(&AccountEntry{
		Label:         "unauthenticated",
		Client:        nil,
		Authenticated: false,
	})

	defaultEntry, _ := reg.Get("default")

	var resolvedClient *msgraphsdk.GraphServiceClient
	state := newTestResolverState(reg, noElicit(t))
	handler := state.middleware(func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		client, ok := GraphClientFromContext(ctx)
		if !ok {
			t.Fatal("no client in context")
		}
		resolvedClient = client
		return mcp.NewToolResultText("ok"), nil
	})

	result, err := handler(context.Background(), makeRequest(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result)
	}
	if resolvedClient != defaultEntry.Client {
		t.Error("resolved client does not match expected 'default' account")
	}
}

// TestResolveAccount_ZeroAuthenticated_ReturnsError verifies that when all
// registered accounts are unauthenticated, the resolver returns an error
// instructing the user to authenticate via add_account.
func TestResolveAccount_ZeroAuthenticated_ReturnsError(t *testing.T) {
	reg := NewAccountRegistry()
	_ = reg.Add(&AccountEntry{
		Label:         "account-1",
		Client:        nil,
		Authenticated: false,
	})
	_ = reg.Add(&AccountEntry{
		Label:         "account-2",
		Client:        nil,
		Authenticated: false,
	})

	state := newTestResolverState(reg, noElicit(t))
	handler := state.middleware(passthrough)

	result, err := handler(context.Background(), makeRequest(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected tool error for zero authenticated accounts")
	}
	text := extractResultText(result)
	if !contains(text, "no authenticated accounts") {
		t.Errorf("expected 'no authenticated accounts' in error, got: %s", text)
	}
	if !contains(text, "account_add") {
		t.Errorf("expected 'account_add' hint in error, got: %s", text)
	}
}

// TestResolveAccount_MultipleAuthenticated_ElicitsSelection verifies that
// when 2+ authenticated accounts exist and no account parameter is provided,
// elicitation is triggered.
func TestResolveAccount_MultipleAuthenticated_ElicitsSelection(t *testing.T) {
	reg := newTestRegistry("work", "personal")
	elicitCalled := false

	elicit := func(_ context.Context, _ mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
		elicitCalled = true
		return &mcp.ElicitationResult{
			ElicitationResponse: mcp.ElicitationResponse{
				Action: mcp.ElicitationResponseActionAccept,
				Content: map[string]any{
					"account": "work",
				},
			},
		}, nil
	}

	state := newTestResolverState(reg, elicit)
	handler := state.middleware(passthrough)

	result, err := handler(context.Background(), makeRequest(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result)
	}
	if !elicitCalled {
		t.Error("elicitation was not triggered for multiple authenticated accounts")
	}
}

// TestResolveAccount_ElicitationClient_NoFallback verifies that when
// elicitation succeeds, the selected account is returned without triggering
// the fallback path. This ensures elicitation-supporting clients are not
// affected by the broadened fallback logic (AC-16).
func TestResolveAccount_ElicitationClient_NoFallback(t *testing.T) {
	reg := newTestRegistry("default", "work", "personal")
	workEntry, _ := reg.Get("work")

	elicit := func(_ context.Context, _ mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
		return &mcp.ElicitationResult{
			ElicitationResponse: mcp.ElicitationResponse{
				Action: mcp.ElicitationResponseActionAccept,
				Content: map[string]any{
					"account": "work",
				},
			},
		}, nil
	}

	var resolvedClient *msgraphsdk.GraphServiceClient
	state := newTestResolverState(reg, elicit)
	handler := state.middleware(func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		client, ok := GraphClientFromContext(ctx)
		if !ok {
			t.Fatal("no client in context")
		}
		resolvedClient = client
		return mcp.NewToolResultText("ok"), nil
	})

	result, err := handler(context.Background(), makeRequest(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result)
	}
	// Verify "work" was selected, not "default" (which would indicate fallback).
	if resolvedClient != workEntry.Client {
		t.Error("resolved client does not match expected 'work' account; fallback may have triggered incorrectly")
	}
}

// contains is a test helper that checks if s contains substr.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsSubstr(s, substr)
}

// containsSubstr checks if s contains substr using a simple scan.
func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
