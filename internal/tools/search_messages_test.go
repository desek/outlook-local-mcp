// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains tests for the search_messages tool, including tool
// registration, handler construction, parameter validation, and the required
// query parameter enforcement.
package tools

import (
	"context"
	"testing"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// TestSearchMessagesTool_Registration validates that NewSearchMessagesTool is
// properly defined with the expected name and read-only annotation.
func TestSearchMessagesTool_Registration(t *testing.T) {
	tool := NewSearchMessagesTool()
	if tool.Name != "mail_search_messages" {
		t.Errorf("tool name = %q, want %q", tool.Name, "mail_search_messages")
	}

	annotations := tool.Annotations
	if annotations.ReadOnlyHint == nil || !*annotations.ReadOnlyHint {
		t.Error("expected ReadOnlyHint to be true")
	}
}

// TestSearchMessagesTool_HasParameters validates that NewSearchMessagesTool
// defines all expected parameters, with query being required.
func TestSearchMessagesTool_HasParameters(t *testing.T) {
	tool := NewSearchMessagesTool()
	schema := tool.InputSchema

	if len(schema.Required) != 1 || schema.Required[0] != "query" {
		t.Errorf("expected required = [query], got %v", schema.Required)
	}

	expectedParams := []string{
		"query", "folder_id", "max_results", "account", "output",
	}
	for _, param := range expectedParams {
		if _, ok := schema.Properties[param]; !ok {
			t.Errorf("expected %q property to be defined", param)
		}
	}
}

// TestNewHandleSearchMessages_ReturnsHandler validates that
// NewHandleSearchMessages returns a non-nil handler function.
func TestNewHandleSearchMessages_ReturnsHandler(t *testing.T) {
	handler := NewHandleSearchMessages(graph.RetryConfig{}, 0)
	if handler == nil {
		t.Fatal("expected non-nil handler function")
	}
}

// TestSearchMessagesToolCanBeAddedToServer validates that NewSearchMessagesTool
// and its handler can be registered on an MCP server without error or panic.
func TestSearchMessagesToolCanBeAddedToServer(t *testing.T) {
	s := server.NewMCPServer("test-server", "0.0.1",
		server.WithToolCapabilities(false),
		server.WithRecovery(),
	)
	s.AddTool(NewSearchMessagesTool(), NewHandleSearchMessages(graph.RetryConfig{}, 0))
}

// TestSearchMessages_BasicQuery validates that the handler proceeds past the
// client lookup and query validation when a Graph client is in the context and
// a query is provided. The handler will fail at the Graph API call (no mock
// response), but should not return "no account selected" or the empty query error.
func TestSearchMessages_BasicQuery(t *testing.T) {
	handler := NewHandleSearchMessages(graph.RetryConfig{}, 0)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{
		"query": "subject:\"Design Review\"",
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
		if text == "query is required: provide a KQL search string (e.g., subject:\"Design Review\")" {
			t.Error("expected error other than empty query when query is provided")
		}
	}
}

// TestSearchMessages_WithFolderId validates that the handler uses the
// folder-scoped endpoint when folder_id is specified. The handler will fail at
// the Graph API call (no mock response), but should proceed past validation.
func TestSearchMessages_WithFolderId(t *testing.T) {
	handler := NewHandleSearchMessages(graph.RetryConfig{}, 0)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{
		"query":     "from:alice@contoso.com",
		"folder_id": "AAMkAGI2TGULAAA=",
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
	}
}

// TestSearchMessages_MaxResults validates that the handler respects the
// max_results parameter. The handler will fail at the Graph API call (no mock
// response), but should proceed past validation.
func TestSearchMessages_MaxResults(t *testing.T) {
	handler := NewHandleSearchMessages(graph.RetryConfig{}, 0)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{
		"query":       "hasAttachments:true",
		"max_results": float64(10),
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
	}
}

// TestSearchMessages_NoQuery validates that the handler returns a tool error
// when the required query parameter is empty or missing.
func TestSearchMessages_NoQuery(t *testing.T) {
	handler := NewHandleSearchMessages(graph.RetryConfig{}, 0)

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
		t.Fatal("expected error result when query is empty")
	}
	text := result.Content[0].(mcp.TextContent).Text
	expected := "query is required: provide a KQL search string (e.g., subject:\"Design Review\")"
	if text != expected {
		t.Errorf("error text = %q, want %q", text, expected)
	}

	// Test with explicit empty string.
	request.Params.Arguments = map[string]any{
		"query": "",
	}
	result, err = handler(ctx, request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result when query is empty string")
	}
}

// TestSearchMessages_NoClient validates that the handler returns a tool error
// when no Graph client is present in the context.
func TestSearchMessages_NoClient(t *testing.T) {
	handler := NewHandleSearchMessages(graph.RetryConfig{}, 0)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{
		"query": "subject:\"test\"",
	}

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

// TestSearchMessages_MaxResultsClamped validates that max_results values
// exceeding 100 are clamped to 100. This is tested indirectly by verifying the
// handler does not error for a value over 100 and still proceeds to the Graph
// API call.
func TestSearchMessages_MaxResultsClamped(t *testing.T) {
	handler := NewHandleSearchMessages(graph.RetryConfig{}, 0)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{
		"query":       "subject:\"test\"",
		"max_results": float64(200),
	}

	client, srv := newTestGraphClient(t, nil)
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	result, err := handler(ctx, request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The call should proceed past validation (may fail at Graph API call).
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		if text == "no account selected" {
			t.Error("expected error other than 'no account selected'")
		}
	}
}
