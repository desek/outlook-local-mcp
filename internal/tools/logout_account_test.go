// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains tests for the account_logout MCP tool (CR-0056), verifying
// successful disconnection of an authenticated account, rejection of already
// disconnected accounts, preservation of persisted configuration, and token
// cache clearing (FR-19).
package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/mark3labs/mcp-go/mcp"
)

// newLogoutTestRegistry registers a single authenticated entry named "work"
// with the given cache partition name. The entry carries non-nil Client,
// Credential, and Authenticator sentinels so tests can assert they are cleared
// on logout.
func newLogoutTestRegistry(t *testing.T, cacheName string) *auth.AccountRegistry {
	t.Helper()
	registry := auth.NewAccountRegistry()
	entry := &auth.AccountEntry{
		Label:         "work",
		ClientID:      "client",
		TenantID:      "tenant",
		AuthMethod:    "browser",
		CacheName:     cacheName,
		Authenticated: true,
	}
	if err := registry.Add(entry); err != nil {
		t.Fatalf("registry.Add() error: %v", err)
	}
	return registry
}

// extractLogoutResponse parses the tool result body as the expected
// account_logout JSON map and returns it.
func extractLogoutResponse(t *testing.T, result *mcp.CallToolResult) map[string]any {
	t.Helper()
	text := extractText(t, result)
	var resp map[string]any
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error: %v (text=%q)", err, text)
	}
	return resp
}

// TestLogoutAccount_Success verifies account_logout flips Authenticated to
// false and clears the in-memory credential, client, and authenticator
// handles on a connected account.
func TestLogoutAccount_Success(t *testing.T) {
	registry := newLogoutTestRegistry(t, "test-cache-work")

	handler := HandleLogoutAccount(registry)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{"label": "work"}

	result, err := handler(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %v", result)
	}
	resp := extractLogoutResponse(t, result)
	if resp["logged_out"] != true {
		t.Errorf("logged_out = %v, want true", resp["logged_out"])
	}
	if resp["label"] != "work" {
		t.Errorf("label = %v, want work", resp["label"])
	}

	entry, ok := registry.Get("work")
	if !ok {
		t.Fatal("expected 'work' entry to remain in registry after logout")
	}
	if entry.Authenticated {
		t.Error("expected entry.Authenticated = false after logout")
	}
	if entry.Client != nil {
		t.Error("expected entry.Client = nil after logout")
	}
	if entry.Credential != nil {
		t.Error("expected entry.Credential = nil after logout")
	}
	if entry.Authenticator != nil {
		t.Error("expected entry.Authenticator = nil after logout")
	}
}

// TestLogoutAccount_AlreadyDisconnected verifies account_logout rejects
// accounts whose Authenticated flag is already false.
func TestLogoutAccount_AlreadyDisconnected(t *testing.T) {
	registry := auth.NewAccountRegistry()
	if err := registry.Add(&auth.AccountEntry{
		Label:         "work",
		Authenticated: false,
	}); err != nil {
		t.Fatalf("registry.Add() error: %v", err)
	}

	handler := HandleLogoutAccount(registry)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{"label": "work"}

	result, err := handler(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for already-disconnected account")
	}
	text := extractText(t, result)
	if !contains(text, "already disconnected") {
		t.Errorf("error text = %q, want to contain 'already disconnected'", text)
	}
}

// TestLogoutAccount_PreservesConfig verifies that logging out does not remove
// the account from the registry or from accounts.json — the persisted
// configuration remains intact so account_login can restore the account.
func TestLogoutAccount_PreservesConfig(t *testing.T) {
	dir := t.TempDir()
	accountsPath := filepath.Join(dir, "accounts.json")
	if err := auth.AddAccountConfig(accountsPath, auth.AccountConfig{
		Label:      "work",
		ClientID:   "client",
		TenantID:   "tenant",
		AuthMethod: "browser",
	}); err != nil {
		t.Fatalf("AddAccountConfig() error: %v", err)
	}

	registry := newLogoutTestRegistry(t, "test-cache-preserve")

	handler := HandleLogoutAccount(registry)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{"label": "work"}

	if _, err := handler(context.Background(), request); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := registry.Get("work"); !ok {
		t.Fatal("expected 'work' entry to remain in registry after logout")
	}

	configs, err := auth.LoadAccounts(accountsPath)
	if err != nil {
		t.Fatalf("LoadAccounts() error: %v", err)
	}
	found := false
	for _, c := range configs {
		if c.Label == "work" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected accounts.json to still contain 'work' after logout")
	}
}

// TestLogoutAccount_ClearsTokenCache verifies FR-19: logout removes the
// persisted file-cache artifacts for the account's CacheName. Uses a
// temp-dir HOME override so the real user's cache is untouched.
func TestLogoutAccount_ClearsTokenCache(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome) // Windows parity

	cacheName := "test-cache-clears"
	cacheDir := filepath.Join(tmpHome, ".outlook-local-mcp")
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		t.Fatalf("mkdir cache dir: %v", err)
	}
	artifacts := []string{
		filepath.Join(cacheDir, cacheName+".bin"),
		filepath.Join(cacheDir, cacheName+".cae.bin"),
		filepath.Join(cacheDir, cacheName+"_msal.bin"),
	}
	for _, p := range artifacts {
		if err := os.WriteFile(p, []byte("token"), 0o600); err != nil {
			t.Fatalf("write artifact %s: %v", p, err)
		}
	}

	registry := newLogoutTestRegistry(t, cacheName)
	handler := HandleLogoutAccount(registry)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{"label": "work"}

	if _, err := handler(context.Background(), request); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, p := range artifacts {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("expected artifact %s to be removed, stat err = %v", p, err)
		}
	}
}
