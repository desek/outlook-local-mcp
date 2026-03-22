package server

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
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
