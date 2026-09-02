// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains the handler tests for contacts get_person: the identifier
// refusal before any request, the single request on the success path, and that
// an address is labelled with the person's own display name, since a person
// carries no per-address name.
//
// @agents-index: Handler tests for contacts.get_person covering pre-flight
// identifier validation, request count, and output tiering.
package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/mark3labs/mcp-go/mcp"
)

// singlePersonJSON is a canned single-person response carrying two scored
// addresses and no per-address name, which is the shape Graph returns.
const singlePersonJSON = `{
	"id": "person-1",
	"displayName": "Alex Ranked",
	"jobTitle": "Analyst",
	"scoredEmailAddresses": [
		{"address": "alex.ranked@example.com", "relevanceScore": 12.5},
		{"address": "alex.alt@example.com", "relevanceScore": 4.0}
	]
}`

// runGetPerson invokes the handler against a recording endpoint and returns the
// result together with the recorder.
func runGetPerson(t *testing.T, args map[string]any) (*mcp.CallToolResult, *contactsRecorder) {
	t.Helper()

	recorder := &contactsRecorder{peopleResponse: singlePersonJSON}
	client, srv := newTestGraphClient(t, recorder)
	defer srv.Close()

	handler := NewHandleGetPerson(graph.RetryConfig{}, 30*time.Second)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	result, err := handler(auth.WithGraphClient(context.Background(), client), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return result, recorder
}

// TestGetPerson_LabelsAddressesWithDisplayName validates that the rendered
// label is the person's own display name, because the resource exposes no name
// per address.
func TestGetPerson_LabelsAddressesWithDisplayName(t *testing.T) {
	result, recorder := runGetPerson(t, map[string]any{"person_id": "person-1"})

	if result.IsError {
		t.Fatalf("expected success, got %q", resultText(t, result))
	}
	if got := recorder.callCount(); got != 1 {
		t.Fatalf("graph request count = %d, want 1", got)
	}

	text := resultText(t, result)
	if !strings.Contains(text, "Person: Alex Ranked") {
		t.Errorf("text output does not label the record with the display name: %s", text)
	}
	if !strings.Contains(text, "alex.ranked@example.com") || !strings.Contains(text, "alex.alt@example.com") {
		t.Errorf("text output does not state every address: %s", text)
	}
	if !strings.Contains(text, "Relevance: 12.50") {
		t.Errorf("text output states no relevance score: %s", text)
	}
}

// TestGetPerson_RawTierCarriesScoredAddresses validates that the raw tier
// states each address with its own score.
func TestGetPerson_RawTierCarriesScoredAddresses(t *testing.T) {
	result, _ := runGetPerson(t, map[string]any{
		"person_id": "person-1",
		"output":    "raw",
	})

	var record map[string]any
	if err := json.Unmarshal([]byte(resultText(t, result)), &record); err != nil {
		t.Fatalf("raw output is not JSON: %v", err)
	}
	scored, ok := record["scoredEmailAddresses"].([]any)
	if !ok || len(scored) != 2 {
		t.Fatalf("scoredEmailAddresses = %v, want two entries", record["scoredEmailAddresses"])
	}
}

// TestGetPerson_RejectsEmptyIdentifier validates that a call naming no person
// is refused before any request, with a correction naming where the identifier
// comes from.
func TestGetPerson_RejectsEmptyIdentifier(t *testing.T) {
	result, recorder := runGetPerson(t, map[string]any{})

	if !result.IsError {
		t.Fatal("expected an error result when person_id is empty")
	}
	text := resultText(t, result)
	if !strings.Contains(text, "person_id") || !strings.Contains(text, getPersonIDFix) {
		t.Errorf("error does not name the parameter and its correction: %s", text)
	}
	if got := recorder.callCount(); got != 0 {
		t.Errorf("graph request count = %d, want 0", got)
	}
}

// TestGetPerson_RejectsOverlongIdentifier validates the length bound the shared
// identifier validator enforces.
func TestGetPerson_RejectsOverlongIdentifier(t *testing.T) {
	result, recorder := runGetPerson(t, map[string]any{
		"person_id": strings.Repeat("a", 2048),
	})

	if !result.IsError {
		t.Fatal("expected an error result for an over-length person_id")
	}
	if got := recorder.callCount(); got != 0 {
		t.Errorf("graph request count = %d, want 0", got)
	}
}
