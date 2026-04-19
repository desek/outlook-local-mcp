// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains tests for the list_accounts MCP tool, verifying that it
// correctly serializes registered accounts from the registry.
package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/mark3labs/mcp-go/mcp"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
)

// TestNewListAccountsTool_ReadOnly verifies that the list_accounts tool
// definition is annotated as read-only.
func TestNewListAccountsTool_ReadOnly(t *testing.T) {
	tool := NewListAccountsTool()
	if tool.Annotations.ReadOnlyHint == nil || !*tool.Annotations.ReadOnlyHint {
		t.Error("expected list_accounts to be annotated as read-only")
	}
}

// TestListAccounts_HasOutputParam verifies that the list_accounts tool schema
// includes an "output" parameter with text/summary/raw enum values.
func TestListAccounts_HasOutputParam(t *testing.T) {
	tool := NewListAccountsTool()
	if _, ok := tool.InputSchema.Properties["output"]; !ok {
		t.Error("expected list_accounts tool schema to have an 'output' property")
	}
}

// TestHandleListAccounts_EmptyRegistry verifies that the handler returns an
// empty JSON array when no accounts are registered.
func TestHandleListAccounts_EmptyRegistry(t *testing.T) {
	registry := auth.NewAccountRegistry()
	handler := HandleListAccounts(registry)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"output": "summary"}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := extractText(t, result)
	var accounts []map[string]any
	if err := json.Unmarshal([]byte(text), &accounts); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}
	if len(accounts) != 0 {
		t.Errorf("expected 0 accounts, got %d", len(accounts))
	}
}

// TestHandleListAccounts_WithAccounts verifies that the handler returns
// all registered accounts sorted alphabetically by label.
func TestHandleListAccounts_WithAccounts(t *testing.T) {
	registry := auth.NewAccountRegistry()

	// Add a "default" account with a client (authenticated).
	client, srv := newTestGraphClient(t, nil)
	defer srv.Close()
	if err := registry.Add(&auth.AccountEntry{
		Label:         "default",
		Client:        client,
		Authenticated: true,
	}); err != nil {
		t.Fatalf("registry.Add(default) error: %v", err)
	}

	// Add a "work" account without a client (not authenticated).
	if err := registry.Add(&auth.AccountEntry{
		Label: "work",
	}); err != nil {
		t.Fatalf("registry.Add(work) error: %v", err)
	}

	handler := HandleListAccounts(registry)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"output": "summary"}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := extractText(t, result)
	var accounts []map[string]any
	if err := json.Unmarshal([]byte(text), &accounts); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}

	if len(accounts) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(accounts))
	}

	// Sorted alphabetically: "default" first, then "work".
	if accounts[0]["label"] != "default" {
		t.Errorf("first account label = %q, want %q", accounts[0]["label"], "default")
	}
	if accounts[0]["authenticated"] != true {
		t.Errorf("default authenticated = %v, want true", accounts[0]["authenticated"])
	}
	if accounts[1]["label"] != "work" {
		t.Errorf("second account label = %q, want %q", accounts[1]["label"], "work")
	}
	if accounts[1]["authenticated"] != false {
		t.Errorf("work authenticated = %v, want false", accounts[1]["authenticated"])
	}
}

// TestHandleListAccounts_AuthenticatedStatus verifies that the "authenticated"
// field is true when the account has a non-nil Client and false otherwise.
func TestHandleListAccounts_AuthenticatedStatus(t *testing.T) {
	registry := auth.NewAccountRegistry()

	// Account with nil Client.
	if err := registry.Add(&auth.AccountEntry{
		Label:  "no-client",
		Client: (*msgraphsdk.GraphServiceClient)(nil),
	}); err != nil {
		t.Fatalf("registry.Add() error: %v", err)
	}

	handler := HandleListAccounts(registry)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"output": "summary"}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := extractText(t, result)
	var accounts []map[string]any
	if err := json.Unmarshal([]byte(text), &accounts); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}

	if len(accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts))
	}
	if accounts[0]["authenticated"] != false {
		t.Errorf("authenticated = %v, want false for nil client", accounts[0]["authenticated"])
	}
}

// TestListAccounts_ZeroAccounts verifies that the text-mode output renders
// the "No accounts registered." message when the registry is empty (CR-0056
// FR-39).
func TestListAccounts_ZeroAccounts(t *testing.T) {
	registry := auth.NewAccountRegistry()
	handler := HandleListAccounts(registry)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"output": "text"}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %v", result)
	}

	text := extractText(t, result)
	if text != "No accounts registered." {
		t.Errorf("result = %q, want %q", text, "No accounts registered.")
	}
}

// extractText extracts the text content from a CallToolResult, failing the
// test if the result is empty or not text.
func extractText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if result == nil {
		t.Fatal("result is nil")
	}
	if len(result.Content) == 0 {
		t.Fatal("result has no content")
	}
	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	return textContent.Text
}
