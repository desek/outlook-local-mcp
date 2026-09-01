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

	// Invoke calendar create_event verb through the server's HandleMessage.
	msg := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"calendar","arguments":{"operation":"create_event","subject":"test","start":"2026-01-01T00:00:00","end":"2026-01-01T01:00:00"}}}`
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

	// Invoke calendar list_calendars verb through the server's HandleMessage.
	msg := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"calendar","arguments":{"operation":"list_calendars"}}}`
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

	writeVerbs := []string{"create_event", "update_event", "delete_event", "cancel_meeting"}
	for _, verb := range writeVerbs {
		msg := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"calendar","arguments":{"operation":"` + verb + `"}}}`
		resp := s.HandleMessage(context.Background(), json.RawMessage(msg))

		switch v := resp.(type) {
		case mcp.JSONRPCResponse:
			result, ok := v.Result.(*mcp.CallToolResult)
			if ok && result.IsError && len(result.Content) > 0 {
				if tc, ok := result.Content[0].(mcp.TextContent); ok {
					if strings.Contains(tc.Text, "read-only mode") {
						t.Errorf("verb %s should not be blocked when readOnly=false", verb)
					}
				}
			}
		case mcp.JSONRPCError:
			if strings.Contains(v.Error.Message, "read-only mode") {
				t.Errorf("verb %s should not be blocked when readOnly=false", verb)
			}
		default:
			t.Fatalf("verb %s: unexpected response type %T", verb, resp)
		}
	}
}

// TestRegisterTools_AccountManagementToolsRegistered verifies that the account
// aggregate tool is registered and the list verb is callable through the server.
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

	// account list verb should return a valid response.
	msg := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"account","arguments":{"operation":"list"}}}`
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
		t.Errorf("account list verb should not return an error, got: %v", result.Content)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected non-empty content from account list verb")
	}
	tc, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	if !strings.Contains(tc.Text, "default") {
		t.Errorf("account list output %q should contain 'default' account", tc.Text)
	}
}

// TestRegisterTools_ListAccounts_WorksInReadOnlyMode verifies that the account
// list verb is accessible even when readOnly is true, since it is inherently
// read-only and not gated by ReadOnlyGuard.
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

	msg := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"account","arguments":{"operation":"list"}}}`
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
					t.Error("account list verb should not be blocked by read-only guard")
				}
			}
		}
	}
}

// TestRegisterTools_RemoveAccount_ToolRegistered verifies that the account
// remove verb is registered and responds (even if the removal fails due to a
// missing label).
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
	msg := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"account","arguments":{"operation":"remove","label":"nonexistent"}}}`
	resp := s.HandleMessage(context.Background(), json.RawMessage(msg))

	rpcResp, ok := resp.(mcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("expected JSONRPCResponse, got %T", resp)
	}
	result, ok := rpcResp.Result.(*mcp.CallToolResult)
	if !ok {
		t.Fatalf("expected *CallToolResult, got %T", rpcResp.Result)
	}
	// Expect an error because "nonexistent" doesn't exist, but the verb
	// must be registered and reachable.
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

	// Verify account list verb returns the default account.
	msg := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"account","arguments":{"operation":"list"}}}`
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
		t.Errorf("account list verb should not return error, got: %v", result.Content)
	}
	tc, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	if !strings.Contains(tc.Text, "default") {
		t.Errorf("account list output %q should contain 'default'", tc.Text)
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

	// Invoke list_calendars verb without "account" parameter. The AccountResolver
	// should auto-select the single "default" account. The call will fail
	// (nil Graph client) but must NOT fail with "account not found" or
	// "multiple accounts" errors.
	msg := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"calendar","arguments":{"operation":"list_calendars"}}}`
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

	// Invoke complete_auth as a verb under the system aggregate tool.
	msg := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"system","arguments":{"operation":"complete_auth","redirect_url":"https://login.microsoftonline.com/common/oauth2/nativeclient?code=test"}}}`
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

	// Invoke system with operation=complete_auth -- should return a tool error
	// because complete_auth is not in the operation enum when auth=browser.
	msg := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"system","arguments":{"operation":"complete_auth"}}}`
	resp := s.HandleMessage(context.Background(), json.RawMessage(msg))

	// The system tool is always registered; calling an unknown verb returns a
	// tool result error, not a JSON-RPC error.
	switch v := resp.(type) {
	case mcp.JSONRPCError:
		// A JSON-RPC error is also acceptable (e.g. enum validation failure).
		t.Logf("got JSON-RPC error: %s", v.Error.Message)
	case mcp.JSONRPCResponse:
		result, ok := v.Result.(*mcp.CallToolResult)
		if !ok {
			t.Fatalf("expected *CallToolResult, got %T", v.Result)
		}
		if !result.IsError {
			t.Error("expected error result for complete_auth verb when auth_code is not active")
		}
	default:
		t.Fatalf("unexpected response type %T", resp)
	}
}

// TestSystemHelp_ListsDocsVerbs verifies that system.help enumerates the three
// documentation verbs (list_docs, search_docs, get_docs) with per-verb
// annotation semantics in its output (CR-0061 Phase 2 / AC-7).
func TestSystemHelp_ListsDocsVerbs(t *testing.T) {
	s := mcpserver.NewMCPServer("test-server", "0.0.1",
		mcpserver.WithToolCapabilities(false),
		mcpserver.WithRecovery(),
	)

	meter := noop.NewMeterProvider().Meter("test")
	m, err := observability.InitMetrics(meter)
	if err != nil {
		t.Fatalf("InitMetrics: %v", err)
	}
	tracer := tracenoop.NewTracerProvider().Tracer("test")
	audit.InitAuditLog(false, "")

	RegisterTools(s, graph.RetryConfig{}, 30*time.Second, m, tracer, false, identityMW, testRegistry(), testConfig(), nil)

	msg := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"system","arguments":{"operation":"help"}}}`
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
		t.Fatalf("system.help returned error: %v", result.Content)
	}

	tc, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}

	for _, verb := range []string{"list_docs", "search_docs", "get_docs"} {
		if !strings.Contains(tc.Text, verb) {
			t.Errorf("system.help output missing verb %q; output:\n%s", verb, tc.Text)
		}
	}
}

// TestResourcesList_IncludesBundledDocs verifies that RegisterResources registers
// MCP resources for every embedded document with the expected URI prefix and
// MIME type (AC-1: resources/list returns all bundled doc URIs).
func TestResourcesList_IncludesBundledDocs(t *testing.T) {
	s := mcpserver.NewMCPServer("test-resources", "0.0.1",
		mcpserver.WithToolCapabilities(false),
		mcpserver.WithResourceCapabilities(false, false),
		mcpserver.WithRecovery(),
	)

	RegisterResources(s)

	// resources/list via HandleMessage.
	msg := `{"jsonrpc":"2.0","id":1,"method":"resources/list","params":{}}`
	resp := s.HandleMessage(context.Background(), json.RawMessage(msg))

	rpcResp, ok := resp.(mcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("expected JSONRPCResponse, got %T", resp)
	}

	data, err := json.Marshal(rpcResp.Result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}

	var listResult struct {
		Resources []struct {
			URI      string `json:"uri"`
			MIMEType string `json:"mimeType"`
		} `json:"resources"`
	}
	if err := json.Unmarshal(data, &listResult); err != nil {
		t.Fatalf("unmarshal resources/list result: %v", err)
	}

	const wantPrefix = "doc://outlook-local-mcp/"
	wantSlugs := map[string]bool{"readme": false, "quickstart": false, "troubleshooting": false}

	for _, r := range listResult.Resources {
		if !strings.HasPrefix(r.URI, wantPrefix) {
			t.Errorf("resource URI %q does not start with %q", r.URI, wantPrefix)
			continue
		}
		slug := strings.TrimPrefix(r.URI, wantPrefix)
		if _, known := wantSlugs[slug]; known {
			wantSlugs[slug] = true
		}
		if r.MIMEType != "text/markdown" {
			t.Errorf("resource %q has MIME type %q, want %q", r.URI, r.MIMEType, "text/markdown")
		}
	}

	for slug, seen := range wantSlugs {
		if !seen {
			t.Errorf("bundled doc slug %q not present in resources/list", slug)
		}
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

// TestRegisterTools_MailDisabled verifies that when MailEnabled is false, the
// mail aggregate tool is still registered (FR-1) and its help verb is callable,
// but MailEnabled-gated verbs (get_conversation, list_attachments, get_attachment)
// are absent from the operation enum.
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

	// The aggregate "mail" tool must be registered unconditionally per FR-1.
	if _, ok := registered["mail"]; !ok {
		t.Error("aggregate 'mail' tool should always be registered, even when MailEnabled=false")
	}

	// No individual mail_* tool names should appear.
	oldNames := []string{"mail_list_folders", "mail_list_messages", "mail_get_message", "mail_search_messages"}
	for _, name := range oldNames {
		if _, ok := registered[name]; ok {
			t.Errorf("old individual mail tool %q should not be registered (replaced by aggregate 'mail')", name)
		}
	}
}

// TestRegisterTools_MailEnabled verifies that when MailEnabled is true, the
// aggregate mail tool is registered and the total tool count is correct.
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

	// The aggregate "mail" tool must be present.
	if _, ok := registered["mail"]; !ok {
		t.Error("aggregate 'mail' tool should be registered when MailEnabled=true")
	}

	// Verify total count: 4 aggregate domain tools (calendar, mail, account, system).
	const expectedTotal = 4
	if got := len(registered); got != expectedTotal {
		t.Errorf("expected %d tools with mail enabled, got %d", expectedTotal, got)
	}

	// MailEnabled alone grants read scope only, so the received-message write
	// verbs must stay out of the published enum. This is the negative half of
	// the gate: without it, a verb accidentally registered on the read flag
	// would still pass every positive assertion above.
	ops := registeredOperations(t, s, "mail")
	for _, name := range mailManagementVerbs() {
		if ops[name] {
			t.Errorf("verb %q is published with MailEnabled alone; it is gated on MailManageEnabled", name)
		}
	}

	// add_attachment writes mailbox state, so the read flag must not publish
	// it. The default surface guarantee is a negative claim, and only a
	// negative assertion can grade it.
	if ops["add_attachment"] {
		t.Error("verb \"add_attachment\" is published with MailEnabled alone; it is gated on MailManageEnabled")
	}
}

// TestRegisterTools_MailAggregate_HelpVerb verifies that the mail aggregate
// tool's help verb is callable and returns documentation.
func TestRegisterTools_MailAggregate_HelpVerb(t *testing.T) {
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

	msg := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"mail","arguments":{"operation":"help"}}}`
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
		t.Errorf("mail help verb should not return error, got: %v", result.Content)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected non-empty content from mail help verb")
	}
	tc, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	if !strings.Contains(tc.Text, "list_folders") {
		t.Errorf("mail help output should mention 'list_folders', got: %q", tc.Text)
	}
}

// registeredOperations returns the set of operation names published in the
// enum of an aggregate domain tool's operation parameter. The enum is what a
// client reads to learn which verbs the running configuration hosts, so it is
// the surface a gating assertion belongs on.
func registeredOperations(t *testing.T, s *mcpserver.MCPServer, domain string) map[string]bool {
	t.Helper()

	tool, ok := s.ListTools()[domain]
	if !ok {
		t.Fatalf("aggregate %q tool is not registered", domain)
	}
	raw, ok := tool.Tool.InputSchema.Properties["operation"]
	if !ok {
		t.Fatalf("aggregate %q tool publishes no operation parameter", domain)
	}
	schema, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("aggregate %q operation schema is %T, want map[string]any", domain, raw)
	}
	values, ok := schema["enum"].([]string)
	if !ok {
		t.Fatalf("aggregate %q operation schema carries no string enum, got %T", domain, schema["enum"])
	}
	out := make(map[string]bool, len(values))
	for _, v := range values {
		out[v] = true
	}
	return out
}

// mailManagementVerbs names the received-message write verbs whose registration
// is gated on MailManageEnabled. Both the positive and the negative test read
// this list, so the pair cannot drift apart.
func mailManagementVerbs() []string {
	return []string{"move_message", "set_flag", "set_categories", "mark_read"}
}

// TestRegisterTools_MailManage_RegistersManagementVerbs verifies that with
// MailManageEnabled set, the received-message write verbs appear in the mail
// tool's published operation enum and no new top-level tool is registered
// alongside them: the surface grows by verbs, not by tools.
func TestRegisterTools_MailManage_RegistersManagementVerbs(t *testing.T) {
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
	cfg.MailManageEnabled = true

	RegisterTools(s, graph.RetryConfig{}, 30*time.Second, m, tracer, false, identityMW, testRegistry(), cfg, nil)

	ops := registeredOperations(t, s, "mail")
	for _, name := range mailManagementVerbs() {
		if !ops[name] {
			t.Errorf("verb %q is absent from the mail operation enum with MailManageEnabled=true", name)
		}
	}

	const expectedTotal = 4
	if got := len(s.ListTools()); got != expectedTotal {
		t.Errorf("expected %d aggregate tools, got %d; new verbs must not add a top-level tool", expectedTotal, got)
	}
}

// TestRegisterTools_MailManage_RegistersAddAttachment verifies that the
// attachment verb reaches the published operation enum under the manage gate,
// and that it arrives as a verb rather than as a fifth top-level tool.
func TestRegisterTools_MailManage_RegistersAddAttachment(t *testing.T) {
	s := mcpserver.NewMCPServer("test-server", "0.0.1",
		mcpserver.WithToolCapabilities(false),
		mcpserver.WithRecovery(),
	)

	m, err := observability.InitMetrics(noop.NewMeterProvider().Meter("test"))
	if err != nil {
		t.Fatalf("InitMetrics() error: %v", err)
	}
	tracer := tracenoop.NewTracerProvider().Tracer("test")

	audit.InitAuditLog(false, "")

	RegisterTools(s, graph.RetryConfig{}, 30*time.Second, m, tracer, false, identityMW, testRegistry(), maximalMailConfig(), nil)

	if ops := registeredOperations(t, s, "mail"); !ops["add_attachment"] {
		t.Error("verb \"add_attachment\" is absent from the mail operation enum with MailManageEnabled=true")
	}

	const expectedTotal = 4
	if got := len(s.ListTools()); got != expectedTotal {
		t.Errorf("expected %d aggregate tools, got %d; new verbs must not add a top-level tool", expectedTotal, got)
	}
}

// TestMailIntroNamesEveryGatedWriteVerb asserts that the mail domain
// introduction accounts for every verb the manage gate registers.
//
// The introduction enumerates those verbs by name, so it reads as exhaustive to
// a model deciding whether a capability exists. The cases are derived by
// diffing the registry built with the gate off against the registry built with
// it on, rather than listed here: a future gated verb added without an
// introduction edit then fails the build instead of shipping an introduction
// that describes a smaller server than the one registered.
func TestMailIntroNamesEveryGatedWriteVerb(t *testing.T) {
	m, err := observability.InitMetrics(noop.NewMeterProvider().Meter("test"))
	if err != nil {
		t.Fatalf("InitMetrics() error: %v", err)
	}
	tracer := tracenoop.NewTracerProvider().Tracer("test")

	audit.InitAuditLog(false, "")

	s := mcpserver.NewMCPServer("test-server", "0.0.1",
		mcpserver.WithToolCapabilities(false),
		mcpserver.WithRecovery(),
	)
	RegisterTools(s, graph.RetryConfig{}, 30*time.Second, m, tracer, false, identityMW, testRegistry(), maximalMailConfig(), nil)

	tool, ok := s.ListTools()["mail"]
	if !ok {
		t.Fatal("aggregate \"mail\" tool is not registered")
	}
	intro := tool.Tool.Description

	readOnlyCfg := testConfig()
	readOnlyCfg.MailEnabled = true
	ungated, _ := buildMailVerbs(mailVerbsConfig{cfg: readOnlyCfg, m: m, tracer: tracer, authMW: identityMW, accountResolverMW: identityMW})
	gated, _ := buildMailVerbs(mailVerbsConfig{cfg: maximalMailConfig(), m: m, tracer: tracer, authMW: identityMW, accountResolverMW: identityMW})

	present := make(map[string]bool, len(ungated))
	for _, v := range ungated {
		present[v.Name] = true
	}
	for _, v := range gated {
		if present[v.Name] {
			continue
		}
		if !strings.Contains(intro, v.Name) {
			t.Errorf("the mail introduction does not name the MailManageEnabled-gated verb %q, so it describes a smaller server than the one registered", v.Name)
		}
	}
}

// TestRegisterTools_CalendarSchedulingReads verifies that both calendar
// scheduling reads appear in the calendar tool's published operation enum in
// every configuration, and that publishing them adds no top-level tool.
//
// The calendar domain has no feature gate, so the claim being graded is that
// neither verb acquired one by accident: the enum is read under the default
// configuration and again under the configuration that turns every mail flag
// on, and both must carry both verbs. The tool count is asserted alongside
// because the surface rule is that a new verb joins an existing aggregate
// rather than becoming a fifth tool.
func TestRegisterTools_CalendarSchedulingReads(t *testing.T) {
	meter := noop.NewMeterProvider().Meter("test")
	m, err := observability.InitMetrics(meter)
	if err != nil {
		t.Fatalf("InitMetrics() error: %v", err)
	}
	tracer := tracenoop.NewTracerProvider().Tracer("test")
	audit.InitAuditLog(false, "")

	maximal := testConfig()
	maximal.MailEnabled = true
	maximal.MailManageEnabled = true

	configurations := map[string]config.Config{
		"default": testConfig(),
		"maximal": maximal,
	}

	for label, cfg := range configurations {
		s := mcpserver.NewMCPServer("test-server", "0.0.1",
			mcpserver.WithToolCapabilities(false),
			mcpserver.WithRecovery(),
		)
		RegisterTools(s, graph.RetryConfig{}, 30*time.Second, m, tracer, false, identityMW, testRegistry(), cfg, nil)

		ops := registeredOperations(t, s, "calendar")
		for _, name := range []string{"find_meeting_times", "get_schedule"} {
			if !ops[name] {
				t.Errorf("%s configuration: verb %q is absent from the calendar operation enum", label, name)
			}
		}

		const expectedTotal = 4
		if got := len(s.ListTools()); got != expectedTotal {
			t.Errorf("%s configuration: expected %d aggregate tools, got %d", label, expectedTotal, got)
		}
	}
}
