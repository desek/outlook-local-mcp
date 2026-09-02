// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains the handler tests for contacts list_people: that Graph's
// relevance order survives the projection, that one request answers the verb,
// and that an empty mailbox reads as an answer rather than a failure.
//
// @agents-index: Handler tests for contacts.list_people covering relevance
// order preservation, request count, and output tiering.
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

// runListPeople invokes the handler against a recording endpoint serving the
// supplied response, and returns the result together with the recorder.
func runListPeople(t *testing.T, response string, args map[string]any) (*mcp.CallToolResult, *contactsRecorder) {
	t.Helper()

	recorder := &contactsRecorder{peopleResponse: response}
	client, srv := newTestGraphClient(t, recorder)
	defer srv.Close()

	handler := NewHandleListPeople(graph.RetryConfig{}, 30*time.Second)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	result, err := handler(auth.WithGraphClient(context.Background(), client), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return result, recorder
}

// TestListPeople_PreservesRelevanceOrder validates that the listing states
// Graph's own order, most relevant first, since the position of an entry is
// itself the information this endpoint adds.
func TestListPeople_PreservesRelevanceOrder(t *testing.T) {
	result, recorder := runListPeople(t, peopleCollectionJSON, map[string]any{"output": "summary"})

	if result.IsError {
		t.Fatalf("expected success, got %q", resultText(t, result))
	}
	if got := recorder.callCount(); got != 1 {
		t.Fatalf("graph request count = %d, want 1", got)
	}

	var people []map[string]any
	if err := json.Unmarshal([]byte(resultText(t, result)), &people); err != nil {
		t.Fatalf("summary output is not JSON: %v", err)
	}
	if len(people) != 2 {
		t.Fatalf("person count = %d, want 2", len(people))
	}
	if people[0]["displayName"] != "Alex Ranked" || people[1]["displayName"] != "Alexandra Second" {
		t.Errorf("people out of relevance order: %v then %v", people[0]["displayName"], people[1]["displayName"])
	}
}

// TestListPeople_TextStatesTotalAndOrder validates that the default tier
// renders a numbered listing with a total count that names the ordering.
func TestListPeople_TextStatesTotalAndOrder(t *testing.T) {
	result, _ := runListPeople(t, peopleCollectionJSON, map[string]any{})

	text := resultText(t, result)
	if !strings.Contains(text, "1. Alex Ranked") {
		t.Errorf("text output does not number the listing: %s", text)
	}
	if !strings.Contains(text, "alex.ranked@example.com") {
		t.Errorf("text output states no address: %s", text)
	}
	if !strings.Contains(text, "2 person/people total, most relevant first.") {
		t.Errorf("text output does not state the total and the ordering: %s", text)
	}
}

// TestListPeople_EmptyCollectionIsAnAnswer validates that a mailbox with no
// ranked people reads as a stated result rather than as a failure.
func TestListPeople_EmptyCollectionIsAnAnswer(t *testing.T) {
	result, _ := runListPeople(t, `{"value": []}`, map[string]any{})

	if result.IsError {
		t.Fatalf("expected success, got %q", resultText(t, result))
	}
	if resultText(t, result) != "No people found." {
		t.Errorf("text output = %q, want the stated absence", resultText(t, result))
	}
}

// TestListPeople_RawTierCarriesDetail validates that escalating to raw answers
// the fields the summary tier omits.
func TestListPeople_RawTierCarriesDetail(t *testing.T) {
	result, _ := runListPeople(t, peopleCollectionJSON, map[string]any{"output": "raw"})

	var people []map[string]any
	if err := json.Unmarshal([]byte(resultText(t, result)), &people); err != nil {
		t.Fatalf("raw output is not JSON: %v", err)
	}
	if _, present := people[0]["scoredEmailAddresses"]; !present {
		t.Errorf("raw tier carries no scored addresses: %v", people[0])
	}
}

// TestListPeople_NoAccountCarriesFix validates that a call with no resolved
// account is told how to select one.
func TestListPeople_NoAccountCarriesFix(t *testing.T) {
	handler := NewHandleListPeople(graph.RetryConfig{}, 30*time.Second)

	result, err := handler(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError || resultText(t, result) != listPeopleAccountFix {
		t.Errorf("result = %q, want the account correction", resultText(t, result))
	}
}
