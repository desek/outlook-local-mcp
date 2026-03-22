// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains tests for the get_message tool, including tool
// registration, handler construction, parameter validation, required
// message_id enforcement, and error handling for missing Graph client.
package tools

import (
	"context"
	"testing"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// TestGetMessageTool_Registration validates that NewGetMessageTool is properly
// defined with the expected name and read-only annotation.
func TestGetMessageTool_Registration(t *testing.T) {
	tool := NewGetMessageTool()
	if tool.Name != "mail_get_message" {
		t.Errorf("tool name = %q, want %q", tool.Name, "mail_get_message")
	}

	annotations := tool.Annotations
	if annotations.ReadOnlyHint == nil || !*annotations.ReadOnlyHint {
		t.Error("expected ReadOnlyHint to be true")
	}
}

// TestGetMessageTool_HasParameters validates that NewGetMessageTool defines
// all expected parameters, with message_id being required.
func TestGetMessageTool_HasParameters(t *testing.T) {
	tool := NewGetMessageTool()
	schema := tool.InputSchema

	if len(schema.Required) != 1 || schema.Required[0] != "message_id" {
		t.Errorf("expected required = [message_id], got %v", schema.Required)
	}

	expectedParams := []string{
		"message_id", "account", "output",
	}
	for _, param := range expectedParams {
		if _, ok := schema.Properties[param]; !ok {
			t.Errorf("expected %q property to be defined", param)
		}
	}
}

// TestNewHandleGetMessage_ReturnsHandler validates that NewHandleGetMessage
// returns a non-nil handler function.
func TestNewHandleGetMessage_ReturnsHandler(t *testing.T) {
	handler := NewHandleGetMessage(graph.RetryConfig{}, 0)
	if handler == nil {
		t.Fatal("expected non-nil handler function")
	}
}

// TestGetMessageToolCanBeAddedToServer validates that NewGetMessageTool and
// its handler can be registered on an MCP server without error or panic.
func TestGetMessageToolCanBeAddedToServer(t *testing.T) {
	s := server.NewMCPServer("test-server", "0.0.1",
		server.WithToolCapabilities(false),
		server.WithRecovery(),
	)
	s.AddTool(NewGetMessageTool(), NewHandleGetMessage(graph.RetryConfig{}, 0))
}

// TestGetMessage_Success validates that the handler proceeds past the client
// lookup and message_id validation when a Graph client is in the context and a
// message_id is provided. The handler will fail at the Graph API call (no mock
// response), but should not return "no account selected" or the missing
// message_id error.
func TestGetMessage_Success(t *testing.T) {
	handler := NewHandleGetMessage(graph.RetryConfig{}, 0)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{
		"message_id": "AAMkAGI2TGULAAA=",
	}

	client, srv := newTestGraphClient(t, nil)
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	result, err := handler(ctx, request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		if text == "no account selected" {
			t.Error("expected error other than 'no account selected' when client is in context")
		}
		if text == "missing required parameter: message_id. Tip: Use mail_list_messages or mail_search_messages to find the message ID." {
			t.Error("expected error other than missing message_id when message_id is provided")
		}
	}
}

// TestGetMessage_NoMessageId validates that the handler returns a tool error
// when the required message_id parameter is empty or missing.
func TestGetMessage_NoMessageId(t *testing.T) {
	handler := NewHandleGetMessage(graph.RetryConfig{}, 0)

	client, srv := newTestGraphClient(t, nil)
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	// Test with no arguments at all.
	request := mcp.CallToolRequest{}
	result, err := handler(ctx, request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result when message_id is missing")
	}
	text := result.Content[0].(mcp.TextContent).Text
	expected := "missing required parameter: message_id. Tip: Use mail_list_messages or mail_search_messages to find the message ID."
	if text != expected {
		t.Errorf("error text = %q, want %q", text, expected)
	}

	// Test with explicit empty string.
	request.Params.Arguments = map[string]any{
		"message_id": "",
	}
	result, err = handler(ctx, request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result when message_id is empty string")
	}
}

// TestGetMessage_NotFound validates that the handler returns a tool error when
// no Graph client is present in the context (simulating a scenario where the
// account is not configured or the message cannot be found).
func TestGetMessage_NotFound(t *testing.T) {
	handler := NewHandleGetMessage(graph.RetryConfig{}, 0)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{
		"message_id": "AAMkAGI2TGULAAA=",
	}

	// No Graph client in context simulates "no account selected".
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
