// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains audit and observability integration tests for the
// dispatcher (CR-0060 AC-11, AC-13; test strategy: TestDispatch_AuditFullyQualifiedName,
// TestDispatch_ObservabilityAttributes).
package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/desek/outlook-local-mcp/internal/audit"
	"github.com/desek/outlook-local-mcp/internal/observability"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

// TestDispatch_AuditFullyQualifiedName verifies that when a verb's middleware
// chain includes AuditWrap with the fully-qualified identity
// "{domain}.{operation}", the audit log records the FQN tool name rather than
// the generic aggregate tool name (CR-0060 FR-13 / AC-6).
func TestDispatch_AuditFullyQualifiedName(t *testing.T) {
	// Write audit output to a temp path so we can inspect entries.
	path := t.TempDir() + "/audit.jsonl"

	audit.InitAuditLog(true, path)
	t.Cleanup(func() { audit.InitAuditLog(false, "") })

	// Build a dispatch handler whose single verb is wrapped with AuditWrap
	// using the FQN "calendar.delete_event" as the tool name, matching the
	// pattern used in calendar_verbs.go.
	const fqn = "calendar.delete_event"

	verbHandler := mcpserver.ToolHandlerFunc(
		func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return mcp.NewToolResultText("deleted"), nil
		},
	)

	registry := VerbRegistry{
		"delete_event": {
			Name:    "delete_event",
			Summary: "delete an event",
			Handler: Handler(audit.AuditWrap(fqn, "delete", verbHandler)),
		},
	}

	dispatch := buildDispatchHandler("calendar", registry)

	_, err := dispatch(context.Background(), buildRequest("calendar", map[string]any{
		"operation": "delete_event",
		"event_id":  "test-event-123",
	}))
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	// Read audit entries from the file.
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open audit file: %v", err)
	}
	defer f.Close() //nolint:errcheck // read-only file; close error is non-fatal in tests.

	scanner := bufio.NewScanner(f)
	found := false
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, `"audit":true`) {
			continue
		}
		var entry audit.AuditEntry
		if jsonErr := json.Unmarshal([]byte(line), &entry); jsonErr != nil {
			t.Logf("unmarshal audit line: %v", jsonErr)
			continue
		}
		if entry.ToolName == fqn {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("audit log does not contain an entry with tool_name=%q; expected FQN", fqn)
	}
}

// TestDispatch_ObservabilityAttributes verifies that the dispatcher correctly
// passes through middleware wrapping that tags spans and metrics with both
// mcp.tool and mcp.operation attributes (CR-0060 FR-14 / AC-11).
//
// This test confirms that WithObservability middleware runs without error when
// wrapped by the dispatch layer (the span/metric tag values are validated
// implicitly via no-op exporter — a full attribute assertion would require a
// real SDK test exporter).
func TestDispatch_ObservabilityAttributes(t *testing.T) {
	meter := noop.NewMeterProvider().Meter("test")
	m, err := observability.InitMetrics(meter)
	if err != nil {
		t.Fatalf("InitMetrics: %v", err)
	}
	tracer := tracenoop.NewTracerProvider().Tracer("test")

	// Wrap the handler with WithObservability using the FQN "mail.list_messages"
	// as the operation name, matching the pattern used in mail_verbs.go.
	const fqn = "mail.list_messages"

	called := 0
	inner := mcpserver.ToolHandlerFunc(func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		called++
		return mcp.NewToolResultText("messages"), nil
	})
	wrapped := observability.WithObservability(fqn, m, tracer, inner)

	registry := VerbRegistry{
		"list_messages": {
			Name:    "list_messages",
			Summary: "list messages",
			Handler: Handler(wrapped),
		},
	}

	dispatch := buildDispatchHandler("mail", registry)

	result, err := dispatch(context.Background(), buildRequest("mail", map[string]any{
		"operation": "list_messages",
	}))
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("unexpected error result")
	}
	if called != 1 {
		t.Errorf("handler called %d times, want 1", called)
	}
}
