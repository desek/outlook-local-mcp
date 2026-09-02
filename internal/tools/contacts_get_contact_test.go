// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains the handler tests for contacts get_contact: the
// identifier refusal that must happen before any request, the single request on
// the success path, and the three output tiers.
//
// @agents-index: Handler tests for contacts.get_contact covering pre-flight
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

// singleContactJSON is a canned single-contact response carrying two addresses
// and the detail only the raw tier states.
const singleContactJSON = `{
	"id": "AAMkContact1",
	"displayName": "Alex Stone",
	"companyName": "Northwind",
	"jobTitle": "Analyst",
	"emailAddresses": [
		{"name": "Alex Stone", "address": "alex@example.com"},
		{"name": "Alex home", "address": "alex@home.example"}
	],
	"businessPhones": ["+1 555 0100"],
	"mobilePhone": "+1 555 0111",
	"businessAddress": {"street": "1 Main Street", "city": "Redmond"}
}`

// runGetContact invokes the handler against a recording endpoint and returns
// the result together with the recorder.
func runGetContact(t *testing.T, args map[string]any) (*mcp.CallToolResult, *contactsRecorder) {
	t.Helper()

	recorder := &contactsRecorder{contactsResponse: singleContactJSON}
	client, srv := newTestGraphClient(t, recorder)
	defer srv.Close()

	handler := NewHandleGetContact(graph.RetryConfig{}, 30*time.Second)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	result, err := handler(auth.WithGraphClient(context.Background(), client), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return result, recorder
}

// TestGetContact_ReturnsEveryAddress validates that the default tier states the
// contact's name and every address it holds, in one request.
func TestGetContact_ReturnsEveryAddress(t *testing.T) {
	result, recorder := runGetContact(t, map[string]any{"contact_id": "AAMkContact1"})

	if result.IsError {
		t.Fatalf("expected success, got %q", resultText(t, result))
	}
	if got := recorder.callCount(); got != 1 {
		t.Fatalf("graph request count = %d, want 1", got)
	}

	text := resultText(t, result)
	if !strings.Contains(text, "Contact: Alex Stone") {
		t.Errorf("text output does not name the contact: %s", text)
	}
	if !strings.Contains(text, "alex@example.com") || !strings.Contains(text, "alex@home.example") {
		t.Errorf("text output does not state every address: %s", text)
	}
}

// TestGetContact_RawTierCarriesDetail validates that escalating to raw answers
// the phones and postal address the summary tier deliberately omits.
func TestGetContact_RawTierCarriesDetail(t *testing.T) {
	result, _ := runGetContact(t, map[string]any{
		"contact_id": "AAMkContact1",
		"output":     "raw",
	})

	var record map[string]any
	if err := json.Unmarshal([]byte(resultText(t, result)), &record); err != nil {
		t.Fatalf("raw output is not JSON: %v", err)
	}
	if record["mobilePhone"] != "+1 555 0111" {
		t.Errorf("mobilePhone = %v, want +1 555 0111", record["mobilePhone"])
	}
	if record["jobTitle"] != "Analyst" {
		t.Errorf("jobTitle = %v, want Analyst", record["jobTitle"])
	}
}

// TestGetContact_SummaryTierCarriesResolutionFields validates that the summary
// tier states the fields a resolution flow acts on.
func TestGetContact_SummaryTierCarriesResolutionFields(t *testing.T) {
	result, _ := runGetContact(t, map[string]any{
		"contact_id": "AAMkContact1",
		"output":     "summary",
	})

	var record map[string]any
	if err := json.Unmarshal([]byte(resultText(t, result)), &record); err != nil {
		t.Fatalf("summary output is not JSON: %v", err)
	}
	if record["emailAddress"] != "alex@example.com" {
		t.Errorf("emailAddress = %v, want alex@example.com", record["emailAddress"])
	}
	if _, present := record["mobilePhone"]; present {
		t.Error("summary tier carries raw-only detail")
	}
}

// TestGetContact_RejectsEmptyIdentifier validates that a call naming no
// contact is refused before any request, with a correction naming where the
// identifier comes from.
func TestGetContact_RejectsEmptyIdentifier(t *testing.T) {
	result, recorder := runGetContact(t, map[string]any{})

	if !result.IsError {
		t.Fatal("expected an error result when contact_id is empty")
	}
	text := resultText(t, result)
	if !strings.Contains(text, "contact_id") || !strings.Contains(text, getContactIDFix) {
		t.Errorf("error does not name the parameter and its correction: %s", text)
	}
	if got := recorder.callCount(); got != 0 {
		t.Errorf("graph request count = %d, want 0", got)
	}
}

// TestGetContact_RejectsOverlongIdentifier validates the other bound the shared
// identifier validator enforces, since it bounds emptiness and length and
// rejects no character set.
func TestGetContact_RejectsOverlongIdentifier(t *testing.T) {
	result, recorder := runGetContact(t, map[string]any{
		"contact_id": strings.Repeat("a", 2048),
	})

	if !result.IsError {
		t.Fatal("expected an error result for an over-length contact_id")
	}
	if got := recorder.callCount(); got != 0 {
		t.Errorf("graph request count = %d, want 0", got)
	}
}

// TestGetContact_RejectsInvalidOutputMode validates that an unknown tier is
// refused before any request.
func TestGetContact_RejectsInvalidOutputMode(t *testing.T) {
	result, recorder := runGetContact(t, map[string]any{
		"contact_id": "AAMkContact1",
		"output":     "verbose",
	})

	if !result.IsError {
		t.Fatal("expected an error result for an unknown output mode")
	}
	if got := recorder.callCount(); got != 0 {
		t.Errorf("graph request count = %d, want 0", got)
	}
}
