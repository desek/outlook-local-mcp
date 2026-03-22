// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains tests for the list_mail_folders tool, including the
// serializeMailFolder function, tool registration, and handler construction.
package tools

import (
	"context"
	"testing"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

// TestListMailFoldersTool_Registration validates that NewListMailFoldersTool is
// properly defined with the expected name and read-only annotation.
func TestListMailFoldersTool_Registration(t *testing.T) {
	tool := NewListMailFoldersTool()
	if tool.Name != "mail_list_folders" {
		t.Errorf("tool name = %q, want %q", tool.Name, "mail_list_folders")
	}

	annotations := tool.Annotations
	if annotations.ReadOnlyHint == nil || !*annotations.ReadOnlyHint {
		t.Error("expected ReadOnlyHint to be true")
	}
}

// TestListMailFoldersTool_HasParameters validates that NewListMailFoldersTool
// defines account and max_results parameters, both optional.
func TestListMailFoldersTool_HasParameters(t *testing.T) {
	tool := NewListMailFoldersTool()
	schema := tool.InputSchema

	if len(schema.Required) != 0 {
		t.Errorf("expected 0 required properties, got %d", len(schema.Required))
	}

	if _, ok := schema.Properties["account"]; !ok {
		t.Error("expected account property to be defined")
	}
	if _, ok := schema.Properties["max_results"]; !ok {
		t.Error("expected max_results property to be defined")
	}
}

// TestNewHandleListMailFolders_ReturnsHandler validates that
// NewHandleListMailFolders returns a non-nil handler function.
func TestNewHandleListMailFolders_ReturnsHandler(t *testing.T) {
	handler := NewHandleListMailFolders(graph.RetryConfig{}, 0)
	if handler == nil {
		t.Fatal("expected non-nil handler function")
	}
}

// TestListMailFoldersToolCanBeAddedToServer validates that
// NewListMailFoldersTool and its handler can be registered on an MCP server
// without error or panic.
func TestListMailFoldersToolCanBeAddedToServer(t *testing.T) {
	s := server.NewMCPServer("test-server", "0.0.1",
		server.WithToolCapabilities(false),
		server.WithRecovery(),
	)
	// Must not panic.
	s.AddTool(NewListMailFoldersTool(), NewHandleListMailFolders(graph.RetryConfig{}, 0))
}

// TestListMailFolders_NoClient validates that the handler returns a tool error
// when no Graph client is present in the context (TestListMailFolders_NoClient
// from the CR test plan).
func TestListMailFolders_NoClient(t *testing.T) {
	handler := NewHandleListMailFolders(graph.RetryConfig{}, 0)
	request := mcp.CallToolRequest{}

	result, err := handler(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result when no client in context")
	}
	text := result.Content[0].(mcp.TextContent).Text
	if text != "no account selected" {
		t.Errorf("error text = %q, want %q", text, "no account selected")
	}
}

// TestListMailFolders_Success validates that the handler proceeds past the
// client lookup when a Graph client is in the context. The handler will fail
// at the Graph API call (no mock response), but should not return the
// "no account selected" error (TestListMailFolders_Success from the CR test plan).
func TestListMailFolders_Success(t *testing.T) {
	handler := NewHandleListMailFolders(graph.RetryConfig{}, 0)
	request := mcp.CallToolRequest{}

	client, srv := newTestGraphClient(t, nil)
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	result, err := handler(ctx, request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The call may fail (no mock response), but it should not be "no account selected".
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		if text == "no account selected" {
			t.Error("expected error other than 'no account selected' when client is in context")
		}
	}
}

// TestSerializeMailFolder_AllFields validates that serializeMailFolder correctly
// extracts all fields from a fully populated MailFolderable.
func TestSerializeMailFolder_AllFields(t *testing.T) {
	folder := models.NewMailFolder()
	folder.SetId(ptr("folder-123"))
	folder.SetDisplayName(ptr("Inbox"))
	unread := int32(5)
	folder.SetUnreadItemCount(&unread)
	total := int32(42)
	folder.SetTotalItemCount(&total)

	result := serializeMailFolder(folder)

	if got := result["id"]; got != "folder-123" {
		t.Errorf("id = %q, want %q", got, "folder-123")
	}
	if got := result["displayName"]; got != "Inbox" {
		t.Errorf("displayName = %q, want %q", got, "Inbox")
	}
	if got := result["unreadItemCount"]; got != int32(5) {
		t.Errorf("unreadItemCount = %v, want %v", got, 5)
	}
	if got := result["totalItemCount"]; got != int32(42) {
		t.Errorf("totalItemCount = %v, want %v", got, 42)
	}
}

// TestSerializeMailFolder_NilFields validates that serializeMailFolder does not
// panic when all optional fields on the MailFolderable return nil. A freshly
// constructed MailFolder with no setters called has nil for all optional fields.
func TestSerializeMailFolder_NilFields(t *testing.T) {
	folder := models.NewMailFolder()

	result := serializeMailFolder(folder)

	if got := result["id"]; got != "" {
		t.Errorf("id = %q, want %q", got, "")
	}
	if got := result["displayName"]; got != "" {
		t.Errorf("displayName = %q, want %q", got, "")
	}
	if got := result["unreadItemCount"]; got != int32(0) {
		t.Errorf("unreadItemCount = %v, want %v", got, 0)
	}
	if got := result["totalItemCount"]; got != int32(0) {
		t.Errorf("totalItemCount = %v, want %v", got, 0)
	}
}
