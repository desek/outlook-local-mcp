// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains the standing assertion that the two event-attachment read
// handlers expose reads only. The annotation hints declare the intent; this
// asserts the behaviour, by driving each handler against a server that records
// every HTTP method it sees and failing on anything that is not a GET. The
// attachment-item request builder has a Delete method, deliberately unwired,
// and this test is what keeps it that way when the handlers are next edited.
//
// @agents-index: Tests asserting the two event-attachment read handlers issue
// only GET requests and never a mutating method.
package tools

import (
	"context"
	"net/http"
	"testing"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/mark3labs/mcp-go/mcp"
)

// TestEventAttachmentReadsIssueOnlyGet drives both read handlers against a
// method-recording server and asserts every request either handler produces is
// a GET, so no mutation reaches Graph from a verb classified read-only.
func TestEventAttachmentReadsIssueOnlyGet(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		args    map[string]any
		handler func() func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)
	}{
		{
			name: "list_event_attachments",
			body: eventAttachmentsCollectionBody,
			args: map[string]any{"event_id": "evt-1"},
			handler: func() func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return NewHandleListEventAttachments(graph.RetryConfig{}, 0)
			},
		},
		{
			name: "get_event_attachment",
			body: eventAttachmentItemBody(5),
			args: map[string]any{"event_id": "evt-1", "attachment_id": "att-agenda"},
			handler: func() func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return NewHandleGetEventAttachment(graph.RetryConfig{}, 0, 10485760)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var methods, urls []string
			client, srv := newTestGraphClient(t, recordingHandler(tc.body, &methods, &urls))
			defer srv.Close()
			ctx := auth.WithGraphClient(context.Background(), client)

			request := mcp.CallToolRequest{}
			request.Params.Arguments = tc.args
			result, err := tc.handler()(ctx, request)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.IsError {
				t.Fatalf("unexpected tool error: %s", result.Content[0].(mcp.TextContent).Text)
			}

			if len(methods) == 0 {
				t.Fatal("expected at least one Graph request")
			}
			for i, m := range methods {
				if m != http.MethodGet {
					t.Errorf("request %d used method %s against %s; the read verbs must issue GET only", i, m, urls[i])
				}
			}
		})
	}
}
