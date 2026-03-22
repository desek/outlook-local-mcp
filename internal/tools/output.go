// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides a shared helper for validating the output mode parameter
// on read tools. The output mode controls whether tool responses return a
// compact summary or the full raw Graph API serialization.
package tools

import (
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// ValidateOutputMode extracts and validates the output parameter from an MCP
// tool request. Returns "text" (the default when the parameter is missing
// or empty), "summary", or "raw". Returns an error for any other value.
//
// Parameters:
//   - request: the MCP tool call request containing optional arguments.
//
// Returns the validated output mode string, or an error if the value is
// not "text", "summary", "raw", or empty.
//
// Side effects: none.
func ValidateOutputMode(request mcp.CallToolRequest) (string, error) {
	mode := request.GetString("output", "")
	if mode == "" {
		return "text", nil
	}
	if mode == "summary" || mode == "raw" || mode == "text" {
		return mode, nil
	}
	return "", fmt.Errorf("output must be 'summary', 'raw', or 'text'")
}
