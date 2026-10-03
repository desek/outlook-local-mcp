// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains tests for the calendar list_event_attachments handler:
// the happy-path enumeration, the lightweight field selection that keeps
// content bytes off the wire, and identifier validation preceding any Graph
// request.
//
// @agents-index: Tests for the calendar.list_event_attachments handler,
// covering enumeration, the restricted field selection, and pre-call
// identifier validation.
package tools

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/mark3labs/mcp-go/mcp"
)

// eventAttachmentsCollectionBody is a canned Graph reply carrying two file
// attachments on one event, with the lightweight fields the collection read
// selects and no content bytes.
const eventAttachmentsCollectionBody = `{"value":[` +
	`{"@odata.type":"#microsoft.graph.fileAttachment","id":"att-agenda","name":"agenda.pdf","contentType":"application/pdf","size":2048,"isInline":false},` +
	`{"@odata.type":"#microsoft.graph.fileAttachment","id":"att-deck","name":"deck.pptx","contentType":"application/vnd.ms-powerpoint","size":4096,"isInline":false}` +
	`]}`

// recordingHandler serves a fixed JSON body and appends every request's method
// and full URL to the given slice, so a test can assert what actually reached
// the wire rather than what the handler intended.
func recordingHandler(body string, methods *[]string, urls *[]string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*methods = append(*methods, r.Method)
		*urls = append(*urls, r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})
}

// callListEventAttachments drives the handler against a recording test server
// and returns the tool result together with the requests observed.
func callListEventAttachments(t *testing.T, args map[string]any) (*mcp.CallToolResult, []string, []string) {
	t.Helper()
	var methods, urls []string
	client, srv := newTestGraphClient(t, recordingHandler(eventAttachmentsCollectionBody, &methods, &urls))
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	handler := NewHandleListEventAttachments(graph.RetryConfig{}, 0)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = args
	result, err := handler(ctx, request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return result, methods, urls
}

// TestListEventAttachments_Success asserts the collection read names both
// attachments and reaches Graph exactly once.
func TestListEventAttachments_Success(t *testing.T) {
	result, methods, _ := callListEventAttachments(t, map[string]any{"event_id": "evt-1"})
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", result.Content[0].(mcp.TextContent).Text)
	}
	text := result.Content[0].(mcp.TextContent).Text
	for _, want := range []string{"agenda.pdf", "deck.pptx", "att-agenda", "att-deck"} {
		if !strings.Contains(text, want) {
			t.Errorf("result missing %q, got %q", want, text)
		}
	}
	if len(methods) != 1 {
		t.Errorf("expected exactly one Graph request, got %v", methods)
	}
}

// TestListEventAttachments_NoContentBytesFetched asserts the request carries
// the shared lightweight field selection, so attachment content never crosses
// the wire on this verb.
func TestListEventAttachments_NoContentBytesFetched(t *testing.T) {
	_, _, urls := callListEventAttachments(t, map[string]any{"event_id": "evt-1"})
	if len(urls) != 1 {
		t.Fatalf("expected exactly one request, got %v", urls)
	}
	for _, field := range listAttachmentsSelectFields {
		if !strings.Contains(urls[0], field) {
			t.Errorf("request URL %q does not select %q", urls[0], field)
		}
	}
	if strings.Contains(urls[0], "contentBytes") {
		t.Errorf("request URL %q selects content bytes", urls[0])
	}
}

// TestListEventAttachments_InvalidEventIDRejectedBeforeCall asserts an
// identifier that cannot name an event is refused with a correction and that
// no request is issued.
func TestListEventAttachments_InvalidEventIDRejectedBeforeCall(t *testing.T) {
	var methods, urls []string
	client, srv := newTestGraphClient(t, recordingHandler(eventAttachmentsCollectionBody, &methods, &urls))
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	handler := NewHandleListEventAttachments(graph.RetryConfig{}, 0)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{"event_id": strings.Repeat("x", 4096)}
	result, err := handler(ctx, request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected an error result for an over-long event_id")
	}
	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "event_id") {
		t.Errorf("error does not name event_id: %q", text)
	}
	if !strings.Contains(text, listEventAttachmentsIDFix) {
		t.Errorf("error carries no fix instruction: %q", text)
	}
	if len(methods) != 0 {
		t.Errorf("expected no Graph request, got %v", methods)
	}
}

// TestListEventAttachments_MissingEventIDRejected asserts the required
// parameter is named when it is absent.
func TestListEventAttachments_MissingEventIDRejected(t *testing.T) {
	result, methods, _ := callListEventAttachments(t, map[string]any{})
	if !result.IsError {
		t.Fatal("expected an error result when event_id is absent")
	}
	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "event_id") || !strings.Contains(text, listEventAttachmentsIDFix) {
		t.Errorf("error does not name event_id with a fix: %q", text)
	}
	if len(methods) != 0 {
		t.Errorf("expected no Graph request, got %v", methods)
	}
}

// TestListEventAttachments_Deterministic asserts repeating the read against an
// unchanged event returns byte-identical output at every tier, so the handler
// introduces no ordering or map-iteration variation.
func TestListEventAttachments_Deterministic(t *testing.T) {
	var methods, urls []string
	client, srv := newTestGraphClient(t, recordingHandler(eventAttachmentsCollectionBody, &methods, &urls))
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)
	handler := NewHandleListEventAttachments(graph.RetryConfig{}, 0)

	for _, mode := range []string{"text", "summary", "raw"} {
		request := mcp.CallToolRequest{}
		request.Params.Arguments = map[string]any{"event_id": "evt-1", "output": mode}
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
