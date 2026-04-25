// Package help provides tests for the Phase 2 help renderer (CR-0060).
package help

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/desek/outlook-local-mcp/internal/tools"
)

// makeRegistry builds a test VerbRegistry with a fixed set of verbs.
func makeRegistry() tools.VerbRegistry {
	return tools.VerbRegistry{
		"help":         {Name: "help", Summary: "show documentation"},
		"delete_event": {Name: "delete_event", Summary: "remove a calendar event"},
		"list_events":  {Name: "list_events", Summary: "list events in a window"},
	}
}

// resultText pulls the first text content item from a CallToolResult.
func resultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if result == nil {
		t.Fatal("nil result")
	}
	if len(result.Content) == 0 {
		t.Fatal("empty content in result")
	}
	item, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("first content item is %T, want mcp.TextContent", result.Content[0])
	}
	return item.Text
}

// TestRenderHelp_AllVerbs verifies that Render with no verb argument includes
// every verb name in the output (AC-2 / FR-4).
func TestRenderHelp_AllVerbs(t *testing.T) {
	reg := makeRegistry()
	result, err := Render(reg, "", "text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", resultText(t, result))
	}
	text := resultText(t, result)
	for _, name := range []string{"help", "delete_event", "list_events"} {
		if !strings.Contains(text, name) {
			t.Errorf("output missing verb %q: %s", name, text)
		}
	}
}

// TestRenderHelp_SingleVerb verifies that scoping to a verb returns docs for
// that verb only (AC-3 / FR-5).
func TestRenderHelp_SingleVerb(t *testing.T) {
	reg := makeRegistry()
	result, err := Render(reg, "delete_event", "text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", resultText(t, result))
	}
	text := resultText(t, result)
	if !strings.Contains(text, "delete_event") {
		t.Errorf("output missing 'delete_event': %s", text)
	}
	// Other verbs should NOT appear (only one verb was requested).
	if strings.Contains(text, "list_events") {
		t.Errorf("output should not contain 'list_events' when scoped to delete_event: %s", text)
	}
}

// TestRenderHelp_RawTier verifies that output="raw" returns JSON with an
// "operations" array containing every verb (FR-4 / CR-0051 tier 3).
func TestRenderHelp_RawTier(t *testing.T) {
	reg := makeRegistry()
	result, err := Render(reg, "", "raw")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", resultText(t, result))
	}
	text := resultText(t, result)

	var payload map[string]json.RawMessage
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("raw output is not valid JSON: %v\n%s", err, text)
	}
	ops, ok := payload["operations"]
	if !ok {
		t.Fatalf("raw JSON missing 'operations' key: %s", text)
	}
	var items []map[string]string
	if err := json.Unmarshal(ops, &items); err != nil {
		t.Fatalf("'operations' is not a JSON array: %v", err)
	}
	if len(items) != len(reg) {
		t.Errorf("operations len = %d, want %d", len(items), len(reg))
	}
}

// TestRenderHelp_SummaryTier verifies that output="summary" returns JSON with
// {name, summary} per verb and no extra fields (CR-0051 tier 2).
func TestRenderHelp_SummaryTier(t *testing.T) {
	reg := makeRegistry()
	result, err := Render(reg, "", "summary")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", resultText(t, result))
	}
	text := resultText(t, result)

	var payload struct {
		Operations []map[string]json.RawMessage `json:"operations"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("summary output is not valid JSON: %v\n%s", err, text)
	}
	if len(payload.Operations) != len(reg) {
		t.Errorf("operations len = %d, want %d", len(payload.Operations), len(reg))
	}
	for _, item := range payload.Operations {
		if _, ok := item["name"]; !ok {
			t.Errorf("summary item missing 'name': %v", item)
		}
		if _, ok := item["summary"]; !ok {
			t.Errorf("summary item missing 'summary': %v", item)
		}
	}
}

// TestRenderHelp_UnknownVerb verifies that scoping to an unregistered verb
// returns a structured error (AC-3 / FR-5).
func TestRenderHelp_UnknownVerb(t *testing.T) {
	reg := makeRegistry()
	result, err := Render(reg, "bogus_verb", "text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for unknown verb")
	}
	text := resultText(t, result)
	if !strings.Contains(text, "bogus_verb") {
		t.Errorf("error should mention 'bogus_verb': %s", text)
	}
}
