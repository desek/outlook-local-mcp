// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains the tests that hold for all three event-attachment
// handlers rather than for one of them: that timeout reporting and Graph-error
// redaction are shared behaviour rather than per-handler improvisation, that
// every refusal's correction reaches both the tool result and the log record,
// and that the two reads introduce no run-to-run variation.
//
// Asserting these across the set rather than inside each handler's own file is
// what catches the third handler being written without the property the first
// two have.
//
// @agents-index: Cross-handler tests for the three calendar event-attachment
// verbs, covering timeout reporting, Graph-error redaction, fix instructions on
// both channels, and read determinism.
package tools

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/mark3labs/mcp-go/mcp"
)

// eventAttachmentHandler names one of the three verbs together with a minimal
// valid argument set, so a property can be asserted across the set without each
// case restating how to drive a handler.
type eventAttachmentHandler struct {
	// name identifies the verb in a failure message.
	name string

	// build constructs the handler under the given timeout and size ceiling.
	build func(timeout time.Duration) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)

	// args is a valid argument set that reaches the Graph call, so a forced
	// server-side failure is what the handler reports rather than a validation
	// refusal short-circuiting it.
	args map[string]any
}

// eventAttachmentHandlers returns the three verbs under test.
func eventAttachmentHandlers() []eventAttachmentHandler {
	return []eventAttachmentHandler{
		{
			name: "list_event_attachments",
			build: func(timeout time.Duration) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return NewHandleListEventAttachments(graph.RetryConfig{}, timeout)
			},
			args: map[string]any{"event_id": "evt-1"},
		},
		{
			name: "get_event_attachment",
			build: func(timeout time.Duration) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return NewHandleGetEventAttachment(graph.RetryConfig{}, timeout, 10485760)
			},
			args: map[string]any{"event_id": "evt-1", "attachment_id": "att-agenda"},
		},
		{
			name: "add_event_attachment",
			build: func(timeout time.Duration) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return NewHandleAddEventAttachment(graph.RetryConfig{}, timeout, 10485760)
			},
			// aGVsbG8= is "hello", small enough to take the direct path, so the
			// failure this drives is the one the Graph call reports.
			args: map[string]any{"event_id": "evt-1", "name": "agenda.pdf", "content_bytes": "aGVsbG8="},
		},
	}
}

// driveEventAttachmentHandler runs one verb against a test server and returns
// the tool result text.
func driveEventAttachmentHandler(t *testing.T, h eventAttachmentHandler, handler http.Handler, timeout time.Duration) string {
	t.Helper()
	client, srv := newTestGraphClient(t, handler)
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	request := mcp.CallToolRequest{}
	request.Params.Arguments = h.args
	result, err := h.build(timeout)(ctx, request)
	if err != nil {
		t.Fatalf("%s: unexpected error: %v", h.name, err)
	}
	if !result.IsError {
		t.Fatalf("%s: expected an error result, got: %s", h.name, resultText(t, result))
	}
	return resultText(t, result)
}

// TestEventAttachmentVerbsHonourTimeoutAndRedaction asserts every one of the
// three verbs reports a stalled call as a timeout naming the configured bound,
// and passes a Graph failure through the shared redactor so no token-like
// string in the service's message survives into the result.
func TestEventAttachmentVerbsHonourTimeoutAndRedaction(t *testing.T) {
	const bound = 50 * time.Millisecond
	const stallFor = 10 * time.Second
	const leaked = "eyJhbGciOiJIUzI1NiJ9.token-shaped-secret@contoso.com"

	stalling := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		stall(req, stallFor)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	})
	failing := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"code":"ErrorAccessDenied","message":"Access denied for ` + leaked + `"}}`))
	})

	for _, h := range eventAttachmentHandlers() {
		t.Run(h.name+" reports a stalled call as a timeout naming its bound", func(t *testing.T) {
			text := driveEventAttachmentHandler(t, h, stalling, bound)
			if want := graph.TimeoutErrorMessage(int(bound.Seconds())); !strings.Contains(text, want) {
				t.Errorf("expected the timeout message %q, got: %s", want, text)
			}
		})
		t.Run(h.name+" redacts a Graph failure", func(t *testing.T) {
			text := driveEventAttachmentHandler(t, h, failing, 30*time.Second)
			if strings.Contains(text, leaked) {
				t.Errorf("the service message reached the result unredacted: %s", text)
			}
		})
	}
}

// TestEventAttachmentErrorsReachBothChannels asserts a forced Graph failure in
// each verb carries a correction on the tool result and the same correction on
// the emitted log record. Grading the tool result alone would leave ungraded
// the one channel a headless caller reading a persisted log actually has.
func TestEventAttachmentErrorsReachBothChannels(t *testing.T) {
	fixes := map[string]string{
		"list_event_attachments": listEventAttachmentsGraphFix,
		"get_event_attachment":   getEventAttachmentGraphFix,
		"add_event_attachment":   addEventAttachmentEventFix,
	}
	failing := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"code":"ErrorAccessDenied","message":"Access is denied."}}`))
	})

	for _, h := range eventAttachmentHandlers() {
		t.Run(h.name, func(t *testing.T) {
			var logged bytes.Buffer
			restore := slog.Default()
			slog.SetDefault(slog.New(slog.NewTextHandler(&logged, nil)))
			defer slog.SetDefault(restore)

			text := driveEventAttachmentHandler(t, h, failing, 30*time.Second)
			fix := fixes[h.name]
			if !strings.Contains(text, fix) {
				t.Errorf("the tool result carries no fix instruction, got: %s", text)
			}
			if !strings.Contains(logged.String(), fix) {
				t.Errorf("the log record carries no fix instruction, got: %q", logged.String())
			}
		})
	}
}

// TestEventAttachmentReadsDeterministic asserts the two reads return
// byte-identical output when the same canned response is read twice at every
// tier, so a caller diffing two results sees a change in the mailbox rather
// than a change in the serialization.
func TestEventAttachmentReadsDeterministic(t *testing.T) {
	cases := []struct {
		name string
		body string
		args map[string]any
		call func(timeout time.Duration) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
	}{
		{
			name: "list_event_attachments",
			body: eventAttachmentsCollectionBody,
			args: map[string]any{"event_id": "evt-1"},
			call: func(timeout time.Duration) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return NewHandleListEventAttachments(graph.RetryConfig{}, timeout)
			},
		},
		{
			name: "get_event_attachment",
			body: eventAttachmentItemBody(5),
			args: map[string]any{"event_id": "evt-1", "attachment_id": "att-agenda"},
			call: func(timeout time.Duration) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return NewHandleGetEventAttachment(graph.RetryConfig{}, timeout, 10485760)
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var methods, urls []string
			client, srv := newTestGraphClient(t, recordingHandler(tc.body, &methods, &urls))
			defer srv.Close()
			ctx := auth.WithGraphClient(context.Background(), client)
			handler := tc.call(30 * time.Second)

			for _, mode := range []string{"text", "summary", "raw"} {
				args := map[string]any{"output": mode}
				for k, v := range tc.args {
					args[k] = v
				}
				request := mcp.CallToolRequest{}
				request.Params.Arguments = args

				first, err := handler(ctx, request)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				second, err := handler(ctx, request)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if a, b := resultText(t, first), resultText(t, second); a != b {
					t.Errorf("output mode %q is not deterministic:\nfirst:  %s\nsecond: %s", mode, a, b)
				}
			}
		})
	}
}
