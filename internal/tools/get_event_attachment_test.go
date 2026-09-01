// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains tests for the calendar get_event_attachment handler: the
// content download, the configurable size ceiling and its refusal text, and
// the requirement that both identifiers are supplied before any Graph request.
//
// @agents-index: Tests for the calendar.get_event_attachment handler, covering
// content download, the size-ceiling refusal, and identifier requirements.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/mark3labs/mcp-go/mcp"
)

// eventAttachmentItemBody returns a canned Graph reply for one file attachment
// on an event, reporting the given size in bytes.
func eventAttachmentItemBody(size int) string {
	return fmt.Sprintf(`{"@odata.type":"#microsoft.graph.fileAttachment","id":"att-agenda","name":"agenda.pdf","contentType":"application/pdf","size":%d,"isInline":false,"contentBytes":"aGVsbG8="}`, size)
}

// TestGetEventAttachment_ReturnsContent asserts the item read returns the
// attachment metadata together with its base64 content, in one Graph request.
func TestGetEventAttachment_ReturnsContent(t *testing.T) {
	var methods, urls []string
	client, srv := newTestGraphClient(t, recordingHandler(eventAttachmentItemBody(5), &methods, &urls))
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	handler := NewHandleGetEventAttachment(graph.RetryConfig{}, 0, 10485760)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{
		"event_id":      "evt-1",
		"attachment_id": "att-agenda",
		"output":        "summary",
	}
	result, err := handler(ctx, request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", result.Content[0].(mcp.TextContent).Text)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &payload); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if cb, _ := payload["contentBytes"].(string); cb != "aGVsbG8=" {
		t.Errorf("contentBytes = %q, want aGVsbG8=", cb)
	}
	if name, _ := payload["name"].(string); name != "agenda.pdf" {
		t.Errorf("name = %q, want agenda.pdf", name)
	}
	if len(methods) != 1 || methods[0] != http.MethodGet {
		t.Errorf("expected exactly one GET, got %v", methods)
	}
}

// TestGetEventAttachment_RefusesOverSizeCeiling asserts an attachment above
// the configured ceiling is refused with an error naming the limit and the
// environment variable that raises it, and that no content is returned.
func TestGetEventAttachment_RefusesOverSizeCeiling(t *testing.T) {
	var methods, urls []string
	client, srv := newTestGraphClient(t, recordingHandler(eventAttachmentItemBody(1048576), &methods, &urls))
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	handler := NewHandleGetEventAttachment(graph.RetryConfig{}, 0, 100)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{
		"event_id":      "evt-1",
		"attachment_id": "att-agenda",
	}
	result, err := handler(ctx, request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected an error result for an oversize attachment")
	}
	text := result.Content[0].(mcp.TextContent).Text
	for _, want := range []string{"exceeds maximum allowed", "100", "OUTLOOK_MCP_MAX_ATTACHMENT_SIZE_BYTES"} {
		if !strings.Contains(text, want) {
			t.Errorf("refusal missing %q, got %q", want, text)
		}
	}
	if strings.Contains(text, "aGVsbG8=") {
		t.Errorf("refusal leaked attachment content: %q", text)
	}
}

// TestGetEventAttachment_RequiresAttachmentID asserts a call naming only the
// event is refused with a correction, before any Graph request.
func TestGetEventAttachment_RequiresAttachmentID(t *testing.T) {
	var methods, urls []string
	client, srv := newTestGraphClient(t, recordingHandler(eventAttachmentItemBody(5), &methods, &urls))
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	handler := NewHandleGetEventAttachment(graph.RetryConfig{}, 0, 10485760)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{"event_id": "evt-1"}
	result, err := handler(ctx, request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected an error result when attachment_id is absent")
	}
	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "attachment_id") {
		t.Errorf("error does not name attachment_id: %q", text)
	}
	if !strings.Contains(text, getEventAttachmentAttachmentIDFix) {
		t.Errorf("error carries no fix instruction: %q", text)
	}
	if len(methods) != 0 {
		t.Errorf("expected no Graph request, got %v", methods)
	}
}

// TestGetEventAttachment_RejectsInvalidEventIDBeforeCall asserts identifier
// validation precedes the Graph call for the event identifier too.
func TestGetEventAttachment_RejectsInvalidEventIDBeforeCall(t *testing.T) {
	var methods, urls []string
	client, srv := newTestGraphClient(t, recordingHandler(eventAttachmentItemBody(5), &methods, &urls))
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	handler := NewHandleGetEventAttachment(graph.RetryConfig{}, 0, 10485760)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{
		"event_id":      strings.Repeat("x", 4096),
		"attachment_id": "att-agenda",
	}
	result, err := handler(ctx, request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected an error result for an over-long event_id")
	}
	if !strings.Contains(result.Content[0].(mcp.TextContent).Text, getEventAttachmentEventIDFix) {
		t.Errorf("error carries no fix instruction: %q", result.Content[0].(mcp.TextContent).Text)
	}
	if len(methods) != 0 {
		t.Errorf("expected no Graph request, got %v", methods)
	}
}

// TestGetEventAttachment_Deterministic asserts repeating the read against an
// unchanged attachment returns byte-identical output at every tier.
func TestGetEventAttachment_Deterministic(t *testing.T) {
	var methods, urls []string
	client, srv := newTestGraphClient(t, recordingHandler(eventAttachmentItemBody(5), &methods, &urls))
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)
	handler := NewHandleGetEventAttachment(graph.RetryConfig{}, 0, 10485760)

	for _, mode := range []string{"text", "summary", "raw"} {
		request := mcp.CallToolRequest{}
		request.Params.Arguments = map[string]any{
			"event_id":      "evt-1",
			"attachment_id": "att-agenda",
			"output":        mode,
		}
		first, err := handler(ctx, request)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		second, err := handler(ctx, request)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		a := first.Content[0].(mcp.TextContent).Text
		b := second.Content[0].(mcp.TextContent).Text
		if a != b {
			t.Errorf("output mode %q is not deterministic:\nfirst:  %s\nsecond: %s", mode, a, b)
		}
	}
}
