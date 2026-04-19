package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/desek/outlook-local-mcp/internal/audit"
	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/config"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/observability"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

// identityMW is a no-op auth middleware that passes the handler through
// unchanged. Used in tests that do not exercise authentication behavior.
func identityMW(h mcpserver.ToolHandlerFunc) mcpserver.ToolHandlerFunc { return h }

// testRegistry creates a minimal AccountRegistry with a "default" entry for
// use in tests. The entry has nil credential and client, which is sufficient
// for registration tests that don't invoke Graph API calls. Authenticated is
// set to true so the account resolver considers the entry for auto-selection.
func testRegistry() *auth.AccountRegistry {
	r := auth.NewAccountRegistry()
	_ = r.Add(&auth.AccountEntry{Label: "default", Authenticated: true})
	return r
}

// testConfig returns a minimal config.Config for use in tests.
func testConfig() config.Config {
	return config.Config{
		AuthRecordPath: "/tmp/test-auth-record",
		CacheName:      "test-cache",
		AuthMethod:     "browser",
	}
}

// TestRegisterTools_NoTools validates that RegisterTools executes without error
// or panic when called with noop metrics and tracer and a nil Graph client.
func TestRegisterTools_NoTools(t *testing.T) {
	s := mcpserver.NewMCPServer("test-server", "0.0.1",
		mcpserver.WithToolCapabilities(false),
		mcpserver.WithRecovery(),
	)

	meter := noop.NewMeterProvider().Meter("test")
	m, err := observability.InitMetrics(meter)
	if err != nil {
		t.Fatalf("InitMetrics() error: %v", err)
	}
	tracer := tracenoop.NewTracerProvider().Tracer("test")

	// Must not panic.
	RegisterTools(s, graph.RetryConfig{}, 30*time.Second, m, tracer, false, identityMW, testRegistry(), testConfig(), nil)
}

// TestMCPServerCreation validates that the MCP server is created successfully
// with the expected name, version, and options. The returned server must be
// non-nil.
func TestMCPServerCreation(t *testing.T) {
	s := mcpserver.NewMCPServer("outlook-local", "1.0.0",
		mcpserver.WithToolCapabilities(false),
		mcpserver.WithRecovery(),
	)

	if s == nil {
		t.Fatal("expected non-nil *MCPServer")
	}
}

// TestRegisterTools_ReadOnly_BlocksWriteTool verifies that when readOnly is
// true, calling a write tool through the registered server returns a tool error
// with "read-only mode" in the message.
func TestRegisterTools_ReadOnly_BlocksWriteTool(t *testing.T) {
	s := mcpserver.NewMCPServer("test-server", "0.0.1",
		mcpserver.WithToolCapabilities(false),
		mcpserver.WithRecovery(),
	)

	meter := noop.NewMeterProvider().Meter("test")
	m, err := observability.InitMetrics(meter)
	if err != nil {
		t.Fatalf("InitMetrics() error: %v", err)
	}
	tracer := tracenoop.NewTracerProvider().Tracer("test")

	// Disable audit logging for test isolation.
	audit.InitAuditLog(false, "")

	RegisterTools(s, graph.RetryConfig{}, 30*time.Second, m, tracer, true, identityMW, testRegistry(), testConfig(), nil)

	// Invoke calendar_create_event through the server's HandleMessage.
	msg := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"calendar_create_event","arguments":{"subject":"test","start":"2026-01-01T00:00:00","end":"2026-01-01T01:00:00"}}}`
	resp := s.HandleMessage(context.Background(), json.RawMessage(msg))

	rpcResp, ok := resp.(mcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("expected JSONRPCResponse, got %T", resp)
	}

	result, ok := rpcResp.Result.(*mcp.CallToolResult)
	if !ok {
		t.Fatalf("expected *CallToolResult, got %T", rpcResp.Result)
	}
	if !result.IsError {
		t.Error("expected IsError=true for blocked write tool")
	}
	if len(result.Content) == 0 {
		t.Fatal("expected non-empty content")
	}
	tc, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	if !strings.Contains(tc.Text, "read-only mode") {
		t.Errorf("error message %q should contain 'read-only mode'", tc.Text)
	}
}

// TestRegisterTools_ReadOnly_AllowsReadTool verifies that when readOnly is
// true, calling a read tool through the registered server does NOT return the
// read-only mode blocked error. The handler may fail for other reasons (nil
// Graph client), but the guard must not intercept it.
func TestRegisterTools_ReadOnly_AllowsReadTool(t *testing.T) {
	s := mcpserver.NewMCPServer("test-server", "0.0.1",
		mcpserver.WithToolCapabilities(false),
		mcpserver.WithRecovery(),
	)

	meter := noop.NewMeterProvider().Meter("test")
	m, err := observability.InitMetrics(meter)
	if err != nil {
		t.Fatalf("InitMetrics() error: %v", err)
	}
	tracer := tracenoop.NewTracerProvider().Tracer("test")

	// Disable audit logging for test isolation.
	audit.InitAuditLog(false, "")

	RegisterTools(s, graph.RetryConfig{}, 30*time.Second, m, tracer, true, identityMW, testRegistry(), testConfig(), nil)

	// Invoke calendar_list through the server's HandleMessage.
	msg := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"calendar_list","arguments":{}}}`
	resp := s.HandleMessage(context.Background(), json.RawMessage(msg))

	// The response may be an error (nil Graph client panic recovered), but
	// it must NOT be the read-only guard message.
	switch v := resp.(type) {
	case mcp.JSONRPCResponse:
		result, ok := v.Result.(*mcp.CallToolResult)
		if ok && result.IsError && len(result.Content) > 0 {
			if tc, ok := result.Content[0].(mcp.TextContent); ok {
				if strings.Contains(tc.Text, "read-only mode") {
					t.Error("read tool should not be blocked by read-only guard")
				}
			}
		}
	case mcp.JSONRPCError:
		// An internal error from nil graph client is acceptable; verify it is
		// not the read-only guard.
		if strings.Contains(v.Error.Message, "read-only mode") {
			t.Error("read tool should not be blocked by read-only guard")
		}
	default:
		t.Fatalf("unexpected response type %T", resp)
	}
}

// TestRegisterTools_ReadOnly_False_AllWriteToolsPass verifies that when
// readOnly is false, write tools are not blocked (they fail for other reasons
// with a nil Graph client, but the read-only guard message is absent).
func TestRegisterTools_ReadOnly_False_AllWriteToolsPass(t *testing.T) {
	s := mcpserver.NewMCPServer("test-server", "0.0.1",
		mcpserver.WithToolCapabilities(false),
		mcpserver.WithRecovery(),
	)

	meter := noop.NewMeterProvider().Meter("test")
	m, err := observability.InitMetrics(meter)
	if err != nil {
		t.Fatalf("InitMetrics() error: %v", err)
	}
	tracer := tracenoop.NewTracerProvider().Tracer("test")

	// Disable audit logging for test isolation.
	audit.InitAuditLog(false, "")

	RegisterTools(s, graph.RetryConfig{}, 30*time.Second, m, tracer, false, identityMW, testRegistry(), testConfig(), nil)

	writeTools := []string{"calendar_create_event", "calendar_update_event", "calendar_delete_event", "calendar_cancel_meeting"}
	for _, toolName := range writeTools {
		msg := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` + toolName + `","arguments":{}}}`
		resp := s.HandleMessage(context.Background(), json.RawMessage(msg))

		switch v := resp.(type) {
		case mcp.JSONRPCResponse:
			result, ok := v.Result.(*mcp.CallToolResult)
			if ok && result.IsError && len(result.Content) > 0 {
				if tc, ok := result.Content[0].(mcp.TextContent); ok {
					if strings.Contains(tc.Text, "read-only mode") {
						t.Errorf("tool %s should not be blocked when readOnly=false", toolName)
					}
				}
			}
		case mcp.JSONRPCError:
			if strings.Contains(v.Error.Message, "read-only mode") {
				t.Errorf("tool %s should not be blocked when readOnly=false", toolName)
			}
		default:
			t.Fatalf("tool %s: unexpected response type %T", toolName, resp)
		}
	}
}

// TestRegisterTools_AccountManagementToolsRegistered verifies that the three
// account management tools (add_account, list_accounts, remove_account) are
// registered and callable through the server.
func TestRegisterTools_AccountManagementToolsRegistered(t *testing.T) {
	s := mcpserver.NewMCPServer("test-server", "0.0.1",
		mcpserver.WithToolCapabilities(false),
		mcpserver.WithRecovery(),
	)

	meter := noop.NewMeterProvider().Meter("test")
	m, err := observability.InitMetrics(meter)
	if err != nil {
		t.Fatalf("InitMetrics() error: %v", err)
	}
	tracer := tracenoop.NewTracerProvider().Tracer("test")

	audit.InitAuditLog(false, "")

	RegisterTools(s, graph.RetryConfig{}, 30*time.Second, m, tracer, false, identityMW, testRegistry(), testConfig(), nil)

	// list_accounts should return a valid JSON response.
	msg := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"account_list","arguments":{}}}`
	resp := s.HandleMessage(context.Background(), json.RawMessage(msg))

	rpcResp, ok := resp.(mcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("expected JSONRPCResponse, got %T", resp)
	}
	result, ok := rpcResp.Result.(*mcp.CallToolResult)
	if !ok {
		t.Fatalf("expected *CallToolResult, got %T", rpcResp.Result)
	}
	if result.IsError {
		t.Errorf("list_accounts should not return an error, got: %v", result.Content)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected non-empty content from list_accounts")
	}
	tc, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	if !strings.Contains(tc.Text, "default") {
		t.Errorf("list_accounts output %q should contain 'default' account", tc.Text)
	}
}

// TestRegisterTools_ListAccounts_WorksInReadOnlyMode verifies that
// list_accounts is accessible even when readOnly is true, since it is
// inherently read-only and not gated by ReadOnlyGuard.
func TestRegisterTools_ListAccounts_WorksInReadOnlyMode(t *testing.T) {
	s := mcpserver.NewMCPServer("test-server", "0.0.1",
		mcpserver.WithToolCapabilities(false),
		mcpserver.WithRecovery(),
	)

	meter := noop.NewMeterProvider().Meter("test")
	m, err := observability.InitMetrics(meter)
	if err != nil {
		t.Fatalf("InitMetrics() error: %v", err)
	}
	tracer := tracenoop.NewTracerProvider().Tracer("test")

	audit.InitAuditLog(false, "")

	RegisterTools(s, graph.RetryConfig{}, 30*time.Second, m, tracer, true, identityMW, testRegistry(), testConfig(), nil)

	msg := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"account_list","arguments":{}}}`
	resp := s.HandleMessage(context.Background(), json.RawMessage(msg))

	rpcResp, ok := resp.(mcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("expected JSONRPCResponse, got %T", resp)
	}
	result, ok := rpcResp.Result.(*mcp.CallToolResult)
	if !ok {
		t.Fatalf("expected *CallToolResult, got %T", rpcResp.Result)
	}
	if result.IsError {
		if len(result.Content) > 0 {
			if tc, ok := result.Content[0].(mcp.TextContent); ok {
				if strings.Contains(tc.Text, "read-only mode") {
					t.Error("list_accounts should not be blocked by read-only guard")
				}
			}
		}
	}
}

// TestRegisterTools_RemoveAccount_ToolRegistered verifies that remove_account
// is registered and responds (even if the removal fails due to missing label).
func TestRegisterTools_RemoveAccount_ToolRegistered(t *testing.T) {
	s := mcpserver.NewMCPServer("test-server", "0.0.1",
		mcpserver.WithToolCapabilities(false),
		mcpserver.WithRecovery(),
	)

	meter := noop.NewMeterProvider().Meter("test")
	m, err := observability.InitMetrics(meter)
	if err != nil {
		t.Fatalf("InitMetrics() error: %v", err)
	}
	tracer := tracenoop.NewTracerProvider().Tracer("test")

	audit.InitAuditLog(false, "")

	RegisterTools(s, graph.RetryConfig{}, 30*time.Second, m, tracer, false, identityMW, testRegistry(), testConfig(), nil)

	// Attempt to remove "nonexistent" — should return an error result (not a
	// panic or unknown tool error).
	msg := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"account_remove","arguments":{"label":"nonexistent"}}}`
	resp := s.HandleMessage(context.Background(), json.RawMessage(msg))

	rpcResp, ok := resp.(mcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("expected JSONRPCResponse, got %T", resp)
	}
	result, ok := rpcResp.Result.(*mcp.CallToolResult)
	if !ok {
		t.Fatalf("expected *CallToolResult, got %T", rpcResp.Result)
	}
	// Expect an error because "nonexistent" doesn't exist, but the tool
	// itself must be registered and reachable.
	if !result.IsError {
		t.Error("expected IsError=true for removing nonexistent account")
	}
}

// TestMCPServer_ElicitationCapability verifies that a server created with
// WithElicitation() declares the elicitation capability.
func TestMCPServer_ElicitationCapability(t *testing.T) {
	s := mcpserver.NewMCPServer("test-server", "0.0.1",
		mcpserver.WithToolCapabilities(false),
		mcpserver.WithRecovery(),
		mcpserver.WithElicitation(),
	)

	if s == nil {
		t.Fatal("expected non-nil *MCPServer with elicitation")
	}

	// Verify by sending an initialize request and checking capabilities.
	msg := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"0.0.1"}}}`
	resp := s.HandleMessage(context.Background(), json.RawMessage(msg))

	rpcResp, ok := resp.(mcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("expected JSONRPCResponse, got %T", resp)
	}

	// Marshal and unmarshal to inspect the result as a map for capability check.
	data, err := json.Marshal(rpcResp.Result)
	if err != nil {
		t.Fatalf("failed to marshal result: %v", err)
	}
	var initResult map[string]any
	if err := json.Unmarshal(data, &initResult); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	caps, ok := initResult["capabilities"].(map[string]any)
	if !ok {
		t.Fatalf("expected capabilities map, got %T", initResult["capabilities"])
	}
	if _, hasElicitation := caps["elicitation"]; !hasElicitation {
		t.Error("server capabilities should include 'elicitation' when WithElicitation() is used")
	}
}

// TestRegisterTools_DefaultAccountAtStartup verifies that RegisterTools works
// correctly when the registry contains a "default" account, matching the
// startup registration pattern in main.go.
func TestRegisterTools_DefaultAccountAtStartup(t *testing.T) {
	s := mcpserver.NewMCPServer("test-server", "0.0.1",
		mcpserver.WithToolCapabilities(false),
		mcpserver.WithRecovery(),
	)

	meter := noop.NewMeterProvider().Meter("test")
	m, err := observability.InitMetrics(meter)
	if err != nil {
		t.Fatalf("InitMetrics() error: %v", err)
	}
	tracer := tracenoop.NewTracerProvider().Tracer("test")

	audit.InitAuditLog(false, "")

	// Create registry with "default" entry (same pattern as main.go).
	registry := auth.NewAccountRegistry()
	if err := registry.Add(&auth.AccountEntry{Label: "default"}); err != nil {
		t.Fatalf("registry.Add() error: %v", err)
	}

	// Must not panic and tools must be callable.
	RegisterTools(s, graph.RetryConfig{}, 30*time.Second, m, tracer, false, identityMW, registry, testConfig(), nil)

	// Verify list_accounts returns the default account.
	msg := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"account_list","arguments":{}}}`
	resp := s.HandleMessage(context.Background(), json.RawMessage(msg))

	rpcResp, ok := resp.(mcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("expected JSONRPCResponse, got %T", resp)
	}
	result, ok := rpcResp.Result.(*mcp.CallToolResult)
	if !ok {
		t.Fatalf("expected *CallToolResult, got %T", rpcResp.Result)
	}
	if result.IsError {
		t.Errorf("list_accounts should not return error, got: %v", result.Content)
	}
	tc, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	if !strings.Contains(tc.Text, "default") {
		t.Errorf("list_accounts output %q should contain 'default'", tc.Text)
	}
}

// TestRegisterTools_BackwardCompatSingleAccount verifies backward compatibility
// when operating with a single "default" account. Calendar tool calls should
// work without requiring an explicit "account" parameter.
func TestRegisterTools_BackwardCompatSingleAccount(t *testing.T) {
	s := mcpserver.NewMCPServer("test-server", "0.0.1",
		mcpserver.WithToolCapabilities(false),
		mcpserver.WithRecovery(),
	)

	meter := noop.NewMeterProvider().Meter("test")
	m, err := observability.InitMetrics(meter)
	if err != nil {
		t.Fatalf("InitMetrics() error: %v", err)
	}
	tracer := tracenoop.NewTracerProvider().Tracer("test")

	audit.InitAuditLog(false, "")

	// Single-account registry (backward compat scenario).
	registry := testRegistry()

	RegisterTools(s, graph.RetryConfig{}, 30*time.Second, m, tracer, false, identityMW, registry, testConfig(), nil)

	// Invoke list_calendars without "account" parameter. The AccountResolver
	// should auto-select the single "default" account. The call will fail
	// (nil Graph client) but must NOT fail with "account not found" or
	// "multiple accounts" errors.
	msg := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"calendar_list","arguments":{}}}`
	resp := s.HandleMessage(context.Background(), json.RawMessage(msg))

	switch v := resp.(type) {
	case mcp.JSONRPCResponse:
		result, ok := v.Result.(*mcp.CallToolResult)
		if ok && result.IsError && len(result.Content) > 0 {
			if tc, ok := result.Content[0].(mcp.TextContent); ok {
				if strings.Contains(tc.Text, "account not found") {
					t.Error("single-account mode should auto-select default, not require explicit account")
				}
				if strings.Contains(tc.Text, "multiple accounts") {
					t.Error("single-account mode should not trigger multi-account elicitation")
				}
			}
		}
	case mcp.JSONRPCError:
		if strings.Contains(v.Error.Message, "account not found") {
			t.Error("single-account mode should auto-select default")
		}
	default:
		t.Fatalf("unexpected response type %T", resp)
	}
}

// TestRegisterTools_CompleteAuthRegistered verifies that the complete_auth tool
// is registered when AuthMethod is "auth_code".
func TestRegisterTools_CompleteAuthRegistered(t *testing.T) {
	s := mcpserver.NewMCPServer("test-server", "0.0.1",
		mcpserver.WithToolCapabilities(false),
		mcpserver.WithRecovery(),
	)

	meter := noop.NewMeterProvider().Meter("test")
	m, err := observability.InitMetrics(meter)
	if err != nil {
		t.Fatalf("InitMetrics() error: %v", err)
	}
	tracer := tracenoop.NewTracerProvider().Tracer("test")

	audit.InitAuditLog(false, "")

	cfg := testConfig()
	cfg.AuthMethod = "auth_code"

	// Use a mock credential that implements AuthCodeFlow.
	mock := &mockAuthCodeFlowCred{}

	RegisterTools(s, graph.RetryConfig{}, 30*time.Second, m, tracer, false, identityMW, testRegistry(), cfg, mock)

	// Invoke complete_auth -- the tool should be registered and reachable.
	msg := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"complete_auth","arguments":{"redirect_url":"https://login.microsoftonline.com/common/oauth2/nativeclient?code=test"}}}`
	resp := s.HandleMessage(context.Background(), json.RawMessage(msg))

	rpcResp, ok := resp.(mcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("expected JSONRPCResponse, got %T", resp)
	}
	result, ok := rpcResp.Result.(*mcp.CallToolResult)
	if !ok {
		t.Fatalf("expected *CallToolResult, got %T", rpcResp.Result)
	}

	// The mock ExchangeCode succeeds, so we expect a success result.
	if result.IsError {
		if len(result.Content) > 0 {
			if tc, ok := result.Content[0].(mcp.TextContent); ok {
				t.Errorf("expected success, got error: %s", tc.Text)
			}
		}
	}
}

// TestRegisterTools_CompleteAuthNotRegistered verifies that the complete_auth
// tool is NOT registered when AuthMethod is "browser".
func TestRegisterTools_CompleteAuthNotRegistered(t *testing.T) {
	s := mcpserver.NewMCPServer("test-server", "0.0.1",
		mcpserver.WithToolCapabilities(false),
		mcpserver.WithRecovery(),
	)

	meter := noop.NewMeterProvider().Meter("test")
	m, err := observability.InitMetrics(meter)
	if err != nil {
		t.Fatalf("InitMetrics() error: %v", err)
	}
	tracer := tracenoop.NewTracerProvider().Tracer("test")

	audit.InitAuditLog(false, "")

	cfg := testConfig()
	cfg.AuthMethod = "browser"

	RegisterTools(s, graph.RetryConfig{}, 30*time.Second, m, tracer, false, identityMW, testRegistry(), cfg, nil)

	// Invoke complete_auth -- should return an error since the tool is not registered.
	msg := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"complete_auth","arguments":{"redirect_url":"https://example.com"}}}`
	resp := s.HandleMessage(context.Background(), json.RawMessage(msg))

	// The response should be a JSON-RPC error (tool not found).
	switch v := resp.(type) {
	case mcp.JSONRPCError:
		if !strings.Contains(v.Error.Message, "complete_auth") {
			t.Logf("error message: %s", v.Error.Message)
		}
		// This is expected -- tool not found.
	case mcp.JSONRPCResponse:
		// Some MCP server implementations return a tool error result
		// for unknown tools rather than a JSON-RPC error.
		result, ok := v.Result.(*mcp.CallToolResult)
		if ok && !result.IsError {
			t.Error("expected error response for unregistered complete_auth tool")
		}
	default:
		t.Fatalf("unexpected response type %T", resp)
	}
}

// mockAuthCodeFlowCred implements both auth.Authenticator and auth.AuthCodeFlow
// for server registration tests. ExchangeCode always succeeds.
type mockAuthCodeFlowCred struct{}

// Authenticate satisfies auth.Authenticator.
func (m *mockAuthCodeFlowCred) Authenticate(_ context.Context, _ *policy.TokenRequestOptions) (azidentity.AuthenticationRecord, error) {
	return azidentity.AuthenticationRecord{}, nil
}

// AuthCodeURL satisfies auth.AuthCodeFlow.
func (m *mockAuthCodeFlowCred) AuthCodeURL(_ context.Context, _ []string) (string, error) {
	return "https://login.microsoftonline.com/test", nil
}

// ExchangeCode satisfies auth.AuthCodeFlow.
func (m *mockAuthCodeFlowCred) ExchangeCode(_ context.Context, _ string, _ []string) error {
	return nil
}

// mailToolNames lists the six read-only mail tools gated by MailEnabled:
// four from CR-0043 and the two conversation/attachment additions from CR-0058.
var mailToolNames = []string{"mail_list_folders", "mail_list_messages", "mail_search_messages", "mail_get_message", "mail_get_conversation", "mail_get_attachment"}

// TestRegisterTools_MailDisabled verifies that when MailEnabled is false,
// none of the four mail tools are registered on the server.
func TestRegisterTools_MailDisabled(t *testing.T) {
	s := mcpserver.NewMCPServer("test-server", "0.0.1",
		mcpserver.WithToolCapabilities(false),
		mcpserver.WithRecovery(),
	)

	meter := noop.NewMeterProvider().Meter("test")
	m, err := observability.InitMetrics(meter)
	if err != nil {
		t.Fatalf("InitMetrics() error: %v", err)
	}
	tracer := tracenoop.NewTracerProvider().Tracer("test")

	audit.InitAuditLog(false, "")

	cfg := testConfig()
	cfg.MailEnabled = false

	RegisterTools(s, graph.RetryConfig{}, 30*time.Second, m, tracer, false, identityMW, testRegistry(), cfg, nil)

	registered := s.ListTools()
	for _, name := range mailToolNames {
		if _, ok := registered[name]; ok {
			t.Errorf("mail tool %q should NOT be registered when MailEnabled=false", name)
		}
	}
}

// TestRegisterTools_MailEnabled verifies that when MailEnabled is true,
// all four mail tools are registered and the total tool count includes them.
func TestRegisterTools_MailEnabled(t *testing.T) {
	s := mcpserver.NewMCPServer("test-server", "0.0.1",
		mcpserver.WithToolCapabilities(false),
		mcpserver.WithRecovery(),
	)

	meter := noop.NewMeterProvider().Meter("test")
	m, err := observability.InitMetrics(meter)
	if err != nil {
		t.Fatalf("InitMetrics() error: %v", err)
	}
	tracer := tracenoop.NewTracerProvider().Tracer("test")

	audit.InitAuditLog(false, "")

	cfg := testConfig()
	cfg.MailEnabled = true

	RegisterTools(s, graph.RetryConfig{}, 30*time.Second, m, tracer, false, identityMW, testRegistry(), cfg, nil)

	registered := s.ListTools()

	// Verify all 4 mail tools are present.
	for _, name := range mailToolNames {
		if _, ok := registered[name]; !ok {
			t.Errorf("mail tool %q should be registered when MailEnabled=true", name)
		}
	}

	// Verify total count: 21 base + 6 mail (read-only) = 27.
	const expectedTotal = 27
	if got := len(registered); got != expectedTotal {
		t.Errorf("expected %d tools with mail enabled, got %d", expectedTotal, got)
	}
}

// TestRegisterTools_MailToolsReadOnly verifies that all four mail tools have
// ReadOnlyHint set to true in their tool annotations.
func TestRegisterTools_MailToolsReadOnly(t *testing.T) {
	s := mcpserver.NewMCPServer("test-server", "0.0.1",
		mcpserver.WithToolCapabilities(false),
		mcpserver.WithRecovery(),
	)

	meter := noop.NewMeterProvider().Meter("test")
	m, err := observability.InitMetrics(meter)
	if err != nil {
		t.Fatalf("InitMetrics() error: %v", err)
	}
	tracer := tracenoop.NewTracerProvider().Tracer("test")

	audit.InitAuditLog(false, "")

	cfg := testConfig()
	cfg.MailEnabled = true

	RegisterTools(s, graph.RetryConfig{}, 30*time.Second, m, tracer, false, identityMW, testRegistry(), cfg, nil)

	registered := s.ListTools()
	for _, name := range mailToolNames {
		st, ok := registered[name]
		if !ok {
			t.Fatalf("mail tool %q not registered", name)
		}
		hint := st.Tool.Annotations.ReadOnlyHint
		if hint == nil || !*hint {
			t.Errorf("mail tool %q should have ReadOnlyHint=true", name)
		}
	}
}
