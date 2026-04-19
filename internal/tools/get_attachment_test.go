// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains tests for the mail_get_attachment tool: registration,
// parameter schema, handler construction, happy-path attachment download, and
// enforcement of the configurable maximum attachment size.
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

// attachmentStubHandler returns a handler that serves a FileAttachment with
// the configured size in bytes. The base64 content is always "aGVsbG8=" ("hello").
func attachmentStubHandler(size int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body := fmt.Sprintf(`{"@odata.type":"#microsoft.graph.fileAttachment","id":"att1","name":"hello.txt","contentType":"text/plain","size":%d,"isInline":false,"contentBytes":"aGVsbG8="}`, size)
		_, _ = w.Write([]byte(body))
	})
}

// TestGetAttachmentTool_Registration validates tool identity and annotations.
func TestGetAttachmentTool_Registration(t *testing.T) {
	tool := NewGetAttachmentTool()
	if tool.Name != "mail_get_attachment" {
		t.Errorf("tool name = %q, want mail_get_attachment", tool.Name)
	}
	if tool.Annotations.ReadOnlyHint == nil || !*tool.Annotations.ReadOnlyHint {
		t.Error("expected ReadOnlyHint true")
	}
}

// TestGetAttachmentTool_HasParameters validates the required parameters are
// present on the tool schema.
func TestGetAttachmentTool_HasParameters(t *testing.T) {
	schema := NewGetAttachmentTool().InputSchema
	if len(schema.Required) != 2 {
		t.Errorf("expected 2 required params, got %v", schema.Required)
	}
	for _, p := range []string{"message_id", "attachment_id", "account", "output"} {
		if _, ok := schema.Properties[p]; !ok {
			t.Errorf("missing property %q", p)
		}
	}
}

// TestNewHandleGetAttachment_ReturnsHandler validates handler construction.
func TestNewHandleGetAttachment_ReturnsHandler(t *testing.T) {
	if NewHandleGetAttachment(graph.RetryConfig{}, 0, 10485760) == nil {
		t.Fatal("expected non-nil handler")
	}
}

// TestGetAttachment_Success validates that a within-limit attachment returns
// base64 content in summary mode.
func TestGetAttachment_Success(t *testing.T) {
	client, srv := newTestGraphClient(t, attachmentStubHandler(5))
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	handler := NewHandleGetAttachment(graph.RetryConfig{}, 0, 10485760)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{
		"message_id":    "msg-1",
		"attachment_id": "att-1",
		"output":        "summary",
	}
	result, err := handler(ctx, request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", result.Content[0].(mcp.TextContent).Text)
	}
	text := result.Content[0].(mcp.TextContent).Text
	var payload map[string]any
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if cb, _ := payload["contentBytes"].(string); cb != "aGVsbG8=" {
		t.Errorf("contentBytes = %q, want aGVsbG8=", cb)
	}
	if name, _ := payload["name"].(string); name != "hello.txt" {
		t.Errorf("name = %q, want hello.txt", name)
	}
}

// TestGetAttachment_TooLarge validates that an attachment exceeding the
// configured maximum size returns an error without exposing content.
func TestGetAttachment_TooLarge(t *testing.T) {
	// Stub reports a 1 MB attachment; limit is set to 100 bytes.
	client, srv := newTestGraphClient(t, attachmentStubHandler(1048576))
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	handler := NewHandleGetAttachment(graph.RetryConfig{}, 0, 100)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{
		"message_id":    "msg-1",
		"attachment_id": "att-1",
	}
	result, err := handler(ctx, request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for oversize attachment")
	}
	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "exceeds maximum allowed") {
		t.Errorf("expected size-limit error, got %q", text)
	}
}
