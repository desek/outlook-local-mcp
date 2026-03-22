// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains tests for the remove_account MCP tool, verifying that it
// correctly removes accounts from the registry, rejects protected labels, and
// persists the removal to accounts.json.
package tools

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/mark3labs/mcp-go/mcp"
)

// TestNewRemoveAccountTool_HasLabelParam verifies that the remove_account
// tool definition includes the required "label" parameter.
func TestNewRemoveAccountTool_HasLabelParam(t *testing.T) {
	tool := NewRemoveAccountTool()
	props, ok := tool.InputSchema.Properties["label"]
	if !ok {
		t.Fatal("expected 'label' parameter in tool definition")
	}
	propMap, ok := props.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any for label property, got %T", props)
	}
	if propMap["type"] != "string" {
		t.Errorf("label type = %v, want string", propMap["type"])
	}
}

// TestHandleRemoveAccount_Success verifies that removing an existing non-default
// account succeeds and the account is no longer in the registry.
func TestHandleRemoveAccount_Success(t *testing.T) {
	registry := auth.NewAccountRegistry()
	if err := registry.Add(&auth.AccountEntry{Label: "work"}); err != nil {
		t.Fatalf("registry.Add() error: %v", err)
	}

	accountsPath := filepath.Join(t.TempDir(), "accounts.json")
	handler := HandleRemoveAccount(registry, accountsPath)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{"label": "work"}

	result, err := handler(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := extractText(t, result)
	var resp map[string]any
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}
	if resp["removed"] != true {
		t.Errorf("removed = %v, want true", resp["removed"])
	}
	if resp["label"] != "work" {
		t.Errorf("label = %v, want work", resp["label"])
	}

	// Verify account no longer in registry.
	if _, exists := registry.Get("work"); exists {
		t.Error("expected account 'work' to be removed from registry")
	}
}

// TestHandleRemoveAccount_DefaultBlocked verifies that removing the "default"
// account is rejected by the registry.
func TestHandleRemoveAccount_DefaultBlocked(t *testing.T) {
	registry := auth.NewAccountRegistry()
	if err := registry.Add(&auth.AccountEntry{Label: "default"}); err != nil {
		t.Fatalf("registry.Add() error: %v", err)
	}

	accountsPath := filepath.Join(t.TempDir(), "accounts.json")
	handler := HandleRemoveAccount(registry, accountsPath)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{"label": "default"}

	result, err := handler(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Error("expected error result when removing default account")
	}
}

// TestHandleRemoveAccount_NotFound verifies that removing a non-existent
// account returns an error result.
func TestHandleRemoveAccount_NotFound(t *testing.T) {
	registry := auth.NewAccountRegistry()

	accountsPath := filepath.Join(t.TempDir(), "accounts.json")
	handler := HandleRemoveAccount(registry, accountsPath)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{"label": "nonexistent"}

	result, err := handler(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Error("expected error result when removing non-existent account")
	}
}

// TestHandleRemoveAccount_MissingLabel verifies that the handler returns an
// error result when the "label" parameter is not provided.
func TestHandleRemoveAccount_MissingLabel(t *testing.T) {
	registry := auth.NewAccountRegistry()

	accountsPath := filepath.Join(t.TempDir(), "accounts.json")
	handler := HandleRemoveAccount(registry, accountsPath)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{}

	result, err := handler(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Error("expected error result when label is missing")
	}
}

// TestHandleRemoveAccount_CleansUpConfig verifies that remove_account removes
// the account's identity configuration from accounts.json.
func TestHandleRemoveAccount_CleansUpConfig(t *testing.T) {
	registry := auth.NewAccountRegistry()
	if err := registry.Add(&auth.AccountEntry{Label: "contoso"}); err != nil {
		t.Fatalf("registry.Add() error: %v", err)
	}
	if err := registry.Add(&auth.AccountEntry{Label: "redeploy"}); err != nil {
		t.Fatalf("registry.Add() error: %v", err)
	}

	// Pre-populate accounts.json with both accounts.
	accountsPath := filepath.Join(t.TempDir(), "accounts.json")
	if err := auth.SaveAccounts(accountsPath, []auth.AccountConfig{
		{Label: "contoso", ClientID: "aaaa", TenantID: "tenant-a", AuthMethod: "browser"},
		{Label: "redeploy", ClientID: "bbbb", TenantID: "common", AuthMethod: "device_code"},
	}); err != nil {
		t.Fatalf("SaveAccounts: %v", err)
	}

	handler := HandleRemoveAccount(registry, accountsPath)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{"label": "contoso"}

	result, err := handler(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %v", result)
	}

	// Verify accounts.json no longer contains "contoso" but still has "redeploy".
	accounts, err := auth.LoadAccounts(accountsPath)
	if err != nil {
		t.Fatalf("LoadAccounts: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account in accounts.json, got %d", len(accounts))
	}
	if accounts[0].Label != "redeploy" {
		t.Errorf("remaining account = %q, want %q", accounts[0].Label, "redeploy")
	}
}
