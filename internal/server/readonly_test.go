package server

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/desek/outlook-local-mcp/internal/audit"
	"github.com/desek/outlook-local-mcp/internal/observability"
	"github.com/desek/outlook-local-mcp/internal/tools"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

// dummyHandler is a test helper that returns a fixed tool result. It is used
// to verify that ReadOnlyGuard either blocks or passes through to the handler.
func dummyHandler(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultText("ok"), nil
}

// TestReadOnlyGuard_Enabled_BlocksHandler verifies that when read-only mode is
// enabled, the underlying handler is never called and an error result is returned.
func TestReadOnlyGuard_Enabled_BlocksHandler(t *testing.T) {
	called := false
	inner := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		called = true
		return mcp.NewToolResultText("should not reach"), nil
	}

	guarded := ReadOnlyGuard("calendar_create_event", true, inner)
	result, err := guarded(context.Background(), mcp.CallToolRequest{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("handler should not have been called in read-only mode")
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if !result.IsError {
		t.Error("result should have IsError=true")
	}
}

// TestReadOnlyGuard_Disabled_PassesThrough verifies that when read-only mode is
// disabled, the underlying handler is called and its result is returned.
func TestReadOnlyGuard_Disabled_PassesThrough(t *testing.T) {
	called := false
	inner := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		called = true
		return mcp.NewToolResultText("passed"), nil
	}

	guarded := ReadOnlyGuard("calendar_create_event", false, inner)
	result, err := guarded(context.Background(), mcp.CallToolRequest{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("handler should have been called when read-only is disabled")
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if result.IsError {
		t.Error("result should not be an error")
	}
}

// TestReadOnlyGuard_Enabled_ErrorMessageFormat verifies that the error message
// returned in read-only mode contains the tool name.
func TestReadOnlyGuard_Enabled_ErrorMessageFormat(t *testing.T) {
	guarded := ReadOnlyGuard("calendar_delete_event", true, dummyHandler)
	result, err := guarded(context.Background(), mcp.CallToolRequest{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result content")
	}

	tc, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	if !strings.Contains(tc.Text, "calendar_delete_event") {
		t.Errorf("error message %q should contain tool name %q", tc.Text, "calendar_delete_event")
	}
}

// TestReadOnlyGuard_Enabled_ErrorIsToolError verifies that the result returned
// in read-only mode has IsError set to true.
func TestReadOnlyGuard_Enabled_ErrorIsToolError(t *testing.T) {
	guarded := ReadOnlyGuard("calendar_update_event", true, dummyHandler)
	result, err := guarded(context.Background(), mcp.CallToolRequest{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if !result.IsError {
		t.Error("result.IsError should be true in read-only mode")
	}
}

// TestReadOnlyGuard_Disabled_NoOverhead verifies that when read-only mode is
// disabled, the returned function is the same reference as the input handler,
// ensuring zero wrapping overhead.
func TestReadOnlyGuard_Disabled_NoOverhead(t *testing.T) {
	var original mcpserver.ToolHandlerFunc = dummyHandler

	guarded := ReadOnlyGuard("calendar_list_events", false, original)

	// Compare function pointers via fmt to verify same reference. Go does not
	// support direct == comparison of function values, so we use %p formatting.
	origPtr := funcAddr(original)
	guardedPtr := funcAddr(guarded)
	if origPtr != guardedPtr {
		t.Errorf("expected same function reference, got original=%s guarded=%s", origPtr, guardedPtr)
	}
}

// funcAddr returns the pointer address of a function value as a string for
// comparison purposes.
func funcAddr(f mcpserver.ToolHandlerFunc) string {
	return fmt.Sprintf("%p", f)
}

// TestReadOnlyBlocksMailManagementVerbs invokes each received-message write
// verb through its own registered middleware chain with read-only mode enabled,
// and asserts the refusal names the mail.<verb> identity.
//
// The guard is asserted here rather than at ReadOnlyGuard alone because the
// chain is where a verb can be missed: a write verb wired through the read
// wrapper would still pass every ReadOnlyGuard unit test in this file while
// executing in read-only mode. Invoking the built verb is what grades the
// wiring.
//
// The handlers are invoked with no Graph client bound, so a verb that was not
// guarded would fail at account resolution with a different message; only a
// guarded verb produces the refusal named below.
func TestReadOnlyBlocksMailManagementVerbs(t *testing.T) {
	m, err := observability.InitMetrics(noop.NewMeterProvider().Meter("test"))
	if err != nil {
		t.Fatalf("InitMetrics() error: %v", err)
	}
	audit.InitAuditLog(false, "")

	identity := func(h mcpserver.ToolHandlerFunc) mcpserver.ToolHandlerFunc { return h }
	verbs, _ := buildMailVerbs(mailVerbsConfig{
		cfg:               maximalMailConfig(),
		m:                 m,
		tracer:            tracenoop.NewTracerProvider().Tracer("test"),
		authMW:            identity,
		accountResolverMW: identity,
		readOnly:          true,
	})

	byName := make(map[string]tools.Verb, len(verbs))
	for _, v := range verbs {
		byName[v.Name] = v
	}

	for _, name := range mailManagementVerbs() {
		v, ok := byName[name]
		if !ok {
			t.Errorf("verb %q is not registered under the maximal configuration", name)
			continue
		}
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{"message_id": "msg-1"}
		result, err := v.Handler(context.Background(), req)
		if err != nil {
			t.Fatalf("%s handler returned a transport error: %v", name, err)
		}
		if !result.IsError {
			t.Errorf("verb %q was not blocked in read-only mode", name)
			continue
		}
		tc, ok := result.Content[0].(mcp.TextContent)
		if !ok {
			t.Fatalf("%s: expected TextContent, got %T", name, result.Content[0])
		}
		if !strings.Contains(tc.Text, "read-only") {
			t.Errorf("verb %q refusal %q does not name read-only mode", name, tc.Text)
		}
		if !strings.Contains(tc.Text, "mail."+name) {
			t.Errorf("verb %q refusal %q does not carry the mail.<verb> identity", name, tc.Text)
		}
	}
}

// TestReadOnlyBlocksAddAttachment invokes the attachment verb through its own
// registered middleware chain with read-only mode enabled and asserts the
// refusal names the mail.add_attachment identity.
//
// The verb is asserted separately from the received-message writes because it
// takes a different argument set: the shared loop supplies only a message_id,
// which would reach argument validation rather than the guard if the guard were
// missing, producing a passing error for the wrong reason.
func TestReadOnlyBlocksAddAttachment(t *testing.T) {
	m, err := observability.InitMetrics(noop.NewMeterProvider().Meter("test"))
	if err != nil {
		t.Fatalf("InitMetrics() error: %v", err)
	}
	audit.InitAuditLog(false, "")

	identity := func(h mcpserver.ToolHandlerFunc) mcpserver.ToolHandlerFunc { return h }
	verbs, _ := buildMailVerbs(mailVerbsConfig{
		cfg:               maximalMailConfig(),
		m:                 m,
		tracer:            tracenoop.NewTracerProvider().Tracer("test"),
		authMW:            identity,
		accountResolverMW: identity,
		readOnly:          true,
	})

	var verb *tools.Verb
	for i := range verbs {
		if verbs[i].Name == "add_attachment" {
			verb = &verbs[i]
			break
		}
	}
	if verb == nil {
		t.Fatal("verb \"add_attachment\" is not registered under the maximal configuration")
	}

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"message_id":    "msg-1",
		"name":          "note.txt",
		"content_bytes": "aGVsbG8=",
	}
	result, err := verb.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("add_attachment handler returned a transport error: %v", err)
	}
	if !result.IsError {
		t.Fatal("verb \"add_attachment\" was not blocked in read-only mode")
	}
	tc, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("add_attachment: expected TextContent, got %T", result.Content[0])
	}
	if !strings.Contains(tc.Text, "read-only") {
		t.Errorf("add_attachment refusal %q does not name read-only mode", tc.Text)
	}
	if !strings.Contains(tc.Text, "mail.add_attachment") {
		t.Errorf("add_attachment refusal %q does not carry the mail.<verb> identity", tc.Text)
	}
}
