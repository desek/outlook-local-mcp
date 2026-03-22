// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains tests for the ValidateOutputMode helper function,
// verifying default behavior, valid values, empty strings, and invalid values.
package tools

import (
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// TestValidateOutputMode_Default validates that when no output parameter is
// provided, the default mode "text" is returned.
func TestValidateOutputMode_Default(t *testing.T) {
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{}

	mode, err := ValidateOutputMode(request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != "text" {
		t.Errorf("mode = %q, want %q", mode, "text")
	}
}

// TestValidateOutputMode_Summary validates that output=summary returns
// "summary" without error.
func TestValidateOutputMode_Summary(t *testing.T) {
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{"output": "summary"}

	mode, err := ValidateOutputMode(request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != "summary" {
		t.Errorf("mode = %q, want %q", mode, "summary")
	}
}

// TestValidateOutputMode_Raw validates that output=raw returns "raw"
// without error.
func TestValidateOutputMode_Raw(t *testing.T) {
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{"output": "raw"}

	mode, err := ValidateOutputMode(request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != "raw" {
		t.Errorf("mode = %q, want %q", mode, "raw")
	}
}

// TestValidateOutputMode_Empty validates that an empty string output parameter
// defaults to "text".
func TestValidateOutputMode_Empty(t *testing.T) {
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{"output": ""}

	mode, err := ValidateOutputMode(request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != "text" {
		t.Errorf("mode = %q, want %q", mode, "text")
	}
}

// TestValidateOutputMode_DefaultIsText validates that the default output mode
// is "text" when the output parameter is completely absent from the request.
func TestValidateOutputMode_DefaultIsText(t *testing.T) {
	request := mcp.CallToolRequest{}

	mode, err := ValidateOutputMode(request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != "text" {
		t.Errorf("mode = %q, want %q", mode, "text")
	}
}

// TestValidateOutputMode_Text validates that output=text returns "text"
// without error (FR-17).
func TestValidateOutputMode_Text(t *testing.T) {
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{"output": "text"}

	mode, err := ValidateOutputMode(request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != "text" {
		t.Errorf("mode = %q, want %q", mode, "text")
	}
}

// TestValidateOutputMode_Invalid validates that an invalid value returns
// an error with the expected message.
func TestValidateOutputMode_Invalid(t *testing.T) {
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{"output": "verbose"}

	_, err := ValidateOutputMode(request)
	if err == nil {
		t.Fatal("expected error for invalid output mode")
	}
	if err.Error() != "output must be 'summary', 'raw', or 'text'" {
		t.Errorf("error = %q, want %q", err.Error(), "output must be 'summary', 'raw', or 'text'")
	}
}
