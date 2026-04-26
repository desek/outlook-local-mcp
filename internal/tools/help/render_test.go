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

// TestRenderHelp_TextIncludesParameters verifies that text output lists
// parameters with name, type, required-ness, description, and enum values
// (CR-0060 follow-up: parameter discoverability).
func TestRenderHelp_TextIncludesParameters(t *testing.T) {
	reg := tools.VerbRegistry{
		"delete_draft": {
			Name:    "delete_draft",
			Summary: "permanently delete a draft message",
			Schema: []mcp.ToolOption{
				mcp.WithString("message_id",
					mcp.Required(),
					mcp.Description("The unique identifier of the draft message to delete."),
				),
				mcp.WithString("output",
					mcp.Description("Output mode."),
					mcp.Enum("text", "summary", "raw"),
				),
			},
		},
	}
	result, err := Render(reg, "delete_draft", "text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := resultText(t, result)
	for _, want := range []string{
		"Parameters:",
		"message_id",
		"required",
		"The unique identifier of the draft message to delete.",
		"output",
		"optional",
		"[text|summary|raw]",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("text help missing %q\noutput:\n%s", want, text)
		}
	}
}

// TestRenderHelp_SummaryIncludesParameters verifies that summary JSON contains
// the parameters array per verb.
func TestRenderHelp_SummaryIncludesParameters(t *testing.T) {
	reg := tools.VerbRegistry{
		"delete_draft": {
			Name:    "delete_draft",
			Summary: "permanently delete a draft message",
			Schema: []mcp.ToolOption{
				mcp.WithString("message_id", mcp.Required(), mcp.Description("draft id")),
			},
		},
	}
	result, err := Render(reg, "", "summary")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := resultText(t, result)
	var payload struct {
		Operations []struct {
			Name       string      `json:"name"`
			Parameters []paramSpec `json:"parameters"`
		} `json:"operations"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("summary JSON unmarshal: %v\n%s", err, text)
	}
	if len(payload.Operations) != 1 || len(payload.Operations[0].Parameters) != 1 {
		t.Fatalf("expected 1 op with 1 param, got %+v", payload)
	}
	p := payload.Operations[0].Parameters[0]
	if p.Name != "message_id" || !p.Required || p.Type != "string" {
		t.Errorf("unexpected paramSpec: %+v", p)
	}
}

// TestRenderTextIncludesDescription verifies that text output includes each
// verb's Description (CR-0065 AC-5, FR-10).
func TestRenderTextIncludesDescription(t *testing.T) {
	reg := tools.VerbRegistry{
		"list_events": {
			Name:        "list_events",
			Summary:     "list events in a time window",
			Description: "Returns calendar events in the specified window, expanding recurrences.",
		},
		"help": {
			Name:        "help",
			Summary:     "show documentation",
			Description: "Renders documentation for this domain.",
		},
	}
	result, err := Render(reg, "", "text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := resultText(t, result)
	for _, want := range []string{
		"Returns calendar events",
		"Renders documentation for this domain",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("text output missing description %q\noutput:\n%s", want, text)
		}
	}
}

// TestRenderRawIncludesExamplesAndSeeDocs verifies that raw JSON output
// contains examples and see_docs fields per verb (CR-0065 AC-5, FR-10).
func TestRenderRawIncludesExamplesAndSeeDocs(t *testing.T) {
	reg := tools.VerbRegistry{
		"list_events": {
			Name:        "list_events",
			Summary:     "list events in a time window",
			Description: "Returns calendar events in the specified window.",
			Examples: []tools.Example{
				{Args: map[string]any{"date": "today"}, Comment: "list today's events"},
			},
			SeeDocs: []string{"concepts#output-tiers"},
		},
	}
	result, err := Render(reg, "", "raw")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := resultText(t, result)

	var payload struct {
		Operations []map[string]json.RawMessage `json:"operations"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("raw JSON unmarshal: %v\n%s", err, text)
	}
	if len(payload.Operations) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(payload.Operations))
	}
	op := payload.Operations[0]
	if _, ok := op["examples"]; !ok {
		t.Errorf("raw JSON missing 'examples' key: %v", op)
	}
	if _, ok := op["see_docs"]; !ok {
		t.Errorf("raw JSON missing 'see_docs' key: %v", op)
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
