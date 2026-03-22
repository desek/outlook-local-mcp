// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains tests for the GraphClient helper function, verifying it
// correctly retrieves the Graph client from context and returns an error when
// the client is not present.
package tools

import (
	"context"
	"testing"

	"github.com/desek/outlook-local-mcp/internal/auth"
)

// TestGraphClient_ReturnsClientFromContext validates that GraphClient
// retrieves the client when it has been injected via auth.WithGraphClient.
func TestGraphClient_ReturnsClientFromContext(t *testing.T) {
	client, srv := newTestGraphClient(t, nil)
	defer srv.Close()

	ctx := auth.WithGraphClient(context.Background(), client)

	got, err := GraphClient(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != client {
		t.Error("expected the same client instance from context")
	}
}

// TestGraphClient_ReturnsErrorWhenNotInContext validates that GraphClient
// returns an error when no client has been injected into the context.
func TestGraphClient_ReturnsErrorWhenNotInContext(t *testing.T) {
	got, err := GraphClient(context.Background())
	if err == nil {
		t.Fatal("expected error when no client in context")
	}
	if got != nil {
		t.Error("expected nil client when not in context")
	}
	if err.Error() != "no account selected" {
		t.Errorf("error = %q, want %q", err.Error(), "no account selected")
	}
}
