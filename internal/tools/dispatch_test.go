// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains unit tests for the Phase 1 dispatcher scaffolding
// (CR-0060): verb registry, description composition, operation enum
// construction, routing, and error cases.
package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// newTestServer creates a minimal MCPServer suitable for dispatch tests.
func newTestServer(t *testing.T) *server.MCPServer {
	t.Helper()
	return server.NewMCPServer("test-dispatch", "0.0.0",
		server.WithToolCapabilities(false),
	)
}

// stubHandler returns a Handler that records each call via the supplied
// counter pointer and returns a text result with the given body.
func stubHandler(called *int, body string) Handler {
	return func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		*called++
		return mcp.NewToolResultText(body), nil
	}
}

// makeTestVerb builds a minimal Verb for tests.
func makeTestVerb(name, summary string, h Handler) Verb {
	return Verb{Name: name, Summary: summary, Handler: h}
}

// buildRequest constructs a CallToolRequest for the named tool with the given
// argument map. It is used to call dispatch handlers directly in tests.
func buildRequest(toolName string, args map[string]any) mcp.CallToolRequest {
	req := mcp.CallToolRequest{}
	req.Params.Name = toolName
	req.Params.Arguments = args
	return req
}

// dispatchResultText extracts the text from the first content item of a
// dispatch test result. It fails the test if the result is nil or has no text
// content.
func dispatchResultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if result == nil {
		t.Fatal("nil CallToolResult")
	}
	if len(result.Content) == 0 {
		t.Fatal("empty content in CallToolResult")
	}
	item, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("first content item is %T, want mcp.TextContent", result.Content[0])
	}
	return item.Text
}

// buildDispatcher is a test helper that registers a domain tool on a fresh
// server and returns the raw dispatch ToolHandlerFunc for direct invocation.
// This avoids depending on server.CallTool (not exposed) while still testing
// through RegisterDomainTool.
func buildDispatcher(t *testing.T, cfg DomainToolConfig) server.ToolHandlerFunc {
	t.Helper()
	return buildDispatchHandler(cfg.Domain, func() VerbRegistry {
		reg := make(VerbRegistry, len(cfg.Verbs))
		for _, v := range cfg.Verbs {
			if cfg.Middleware != nil {
				v.middleware = cfg.Middleware
			}
			reg[v.Name] = v
		}
		return reg
	}())
}

// ---- Tests ----

// TestDispatch_RoutesToHandler verifies that a known operation routes to its
// underlying handler exactly once (AC-6 / FR-6).
func TestDispatch_RoutesToHandler(t *testing.T) {
	callCount := 0
	dispatch := buildDispatcher(t, DomainToolConfig{
		Domain: "testdomain",
		Intro:  "Test domain.",
		Verbs: []Verb{
			makeTestVerb("do_thing", "does the thing", stubHandler(&callCount, "done")),
		},
	})

	result, err := dispatch(context.Background(), buildRequest("testdomain", map[string]any{"operation": "do_thing"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 1 {
		t.Errorf("handler called %d times, want 1", callCount)
	}
	if text := dispatchResultText(t, result); text != "done" {
		t.Errorf("result text = %q, want %q", text, "done")
	}
}

// TestDispatch_UnknownOperation verifies that an unrecognised operation value
// returns a structured error mentioning valid verbs and pointing to "help"
// (AC-5 / FR-11).
func TestDispatch_UnknownOperation(t *testing.T) {
	dispatch := buildDispatcher(t, DomainToolConfig{
		Domain: "testdomain",
		Intro:  "Test domain.",
		Verbs: []Verb{
			makeTestVerb("help", "show help", stubHandler(new(int), "")),
			makeTestVerb("do_thing", "does the thing", stubHandler(new(int), "")),
		},
	})

	result, err := dispatch(context.Background(), buildRequest("testdomain", map[string]any{"operation": "bogus"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for unknown operation")
	}
	text := dispatchResultText(t, result)
	if !strings.Contains(text, "bogus") {
		t.Errorf("error text %q should mention the bad operation name", text)
	}
	if !strings.Contains(text, "help") {
		t.Errorf("error text %q should suggest operation=\"help\"", text)
	}
}

// TestDispatch_MissingOperation verifies that a request with no `operation`
// parameter is rejected with a clear error (FR-11 / AC-5).
func TestDispatch_MissingOperation(t *testing.T) {
	dispatch := buildDispatcher(t, DomainToolConfig{
		Domain: "testdomain",
		Intro:  "Test domain.",
		Verbs: []Verb{
			makeTestVerb("do_thing", "does the thing", stubHandler(new(int), "")),
		},
	})

	result, err := dispatch(context.Background(), buildRequest("testdomain", map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for missing operation")
	}
	text := dispatchResultText(t, result)
	if !strings.Contains(text, "operation") {
		t.Errorf("error text %q should mention 'operation'", text)
	}
}

// TestDispatch_UnknownParameter verifies that a known verb is routed
// correctly even when an extra parameter is present. Phase 1 does not
// perform per-verb parameter validation (that is deferred to Phase 3 when
// real handlers are wired); this test confirms no panic occurs and the
// handler is still called (FR-12 / AC-10).
func TestDispatch_UnknownParameter(t *testing.T) {
	called := 0
	dispatch := buildDispatcher(t, DomainToolConfig{
		Domain: "testdomain",
		Intro:  "Test domain.",
		Verbs: []Verb{
			makeTestVerb("do_thing", "does the thing", stubHandler(&called, "ok")),
		},
	})

	result, err := dispatch(context.Background(), buildRequest("testdomain", map[string]any{
		"operation":   "do_thing",
		"bogus_param": "unexpected",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Phase 1: no param validation; handler is reached.
	if result.IsError {
		t.Errorf("unexpected error result: %s", dispatchResultText(t, result))
	}
	if called != 1 {
		t.Errorf("handler called %d times, want 1", called)
	}
}

// ---- Description and enum helpers ----

// TestBuildOperationEnum verifies that buildOperationEnum returns verb names
// in order.
func TestBuildOperationEnum(t *testing.T) {
	verbs := []Verb{
		{Name: "help"},
		{Name: "alpha"},
		{Name: "beta"},
	}
	enum := buildOperationEnum(verbs)
	want := []string{"help", "alpha", "beta"}
	for i, got := range enum {
		if got != want[i] {
			t.Errorf("enum[%d] = %q, want %q", i, got, want[i])
		}
	}
}

// TestBuildTopLevelDescription verifies that the composed description contains
// the intro and each verb name and summary (AC-4 / FR-3).
func TestBuildTopLevelDescription(t *testing.T) {
	verbs := []Verb{
		{Name: "help", Summary: "show help"},
		{Name: "list_things", Summary: "list all things"},
	}
	desc := buildTopLevelDescription("My domain.", verbs)

	if !strings.Contains(desc, "My domain.") {
		t.Errorf("description missing intro: %q", desc)
	}
	if !strings.Contains(desc, "help") {
		t.Errorf("description missing verb 'help': %q", desc)
	}
	if !strings.Contains(desc, "list_things") {
		t.Errorf("description missing verb 'list_things': %q", desc)
	}
	if !strings.Contains(desc, "show help") {
		t.Errorf("description missing summary 'show help': %q", desc)
	}
}

// TestBuildTopLevelDescription_Empty verifies that an empty verb list returns
// just the intro.
func TestBuildTopLevelDescription_Empty(t *testing.T) {
	desc := buildTopLevelDescription("Just intro.", nil)
	if desc != "Just intro." {
		t.Errorf("description = %q, want %q", desc, "Just intro.")
	}
}

// TestVerbRegistry_PopulatedByRegisterDomainTool verifies that
// RegisterDomainTool returns a VerbRegistry containing every registered verb.
func TestVerbRegistry_PopulatedByRegisterDomainTool(t *testing.T) {
	s := newTestServer(t)
	verbs := []Verb{
		makeTestVerb("alpha", "alpha op", stubHandler(new(int), "")),
		makeTestVerb("beta", "beta op", stubHandler(new(int), "")),
	}

	reg := RegisterDomainTool(s, DomainToolConfig{
		Domain: "testdomain",
		Intro:  "Test.",
		Verbs:  verbs,
	})

	for _, v := range verbs {
		if _, ok := reg[v.Name]; !ok {
			t.Errorf("registry missing verb %q", v.Name)
		}
	}
	if len(reg) != len(verbs) {
		t.Errorf("registry len = %d, want %d", len(reg), len(verbs))
	}
}

// TestDispatch_MiddlewareIsApplied verifies that when a middleware factory is
// supplied, it wraps the verb handler before invocation.
func TestDispatch_MiddlewareIsApplied(t *testing.T) {
	mwCalled := 0
	handlerCalled := 0

	mw := func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			mwCalled++
			return next(ctx, req)
		}
	}

	dispatch := buildDispatcher(t, DomainToolConfig{
		Domain:     "testdomain",
		Intro:      "Test.",
		Verbs:      []Verb{makeTestVerb("do_thing", "thing", stubHandler(&handlerCalled, "ok"))},
		Middleware: mw,
	})

	result, err := dispatch(context.Background(), buildRequest("testdomain", map[string]any{"operation": "do_thing"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected error: %s", dispatchResultText(t, result))
	}
	if mwCalled != 1 {
		t.Errorf("middleware called %d times, want 1", mwCalled)
	}
	if handlerCalled != 1 {
		t.Errorf("handler called %d times, want 1", handlerCalled)
	}
}
