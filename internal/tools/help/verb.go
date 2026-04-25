// Package help — this file exposes NewHelpVerb, the constructor that domain
// tools use to register the help verb into their VerbRegistry (CR-0060 Phase 2).
package help

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/desek/outlook-local-mcp/internal/tools"
)

// NewHelpVerb constructs a tools.Verb for the "help" operation that renders
// documentation for the given registry. The returned Verb is ready to be
// prepended to the verbs slice passed to RegisterDomainTool.
//
// The help verb accepts two optional parameters:
//   - "verb" (string): when set, output is scoped to the named verb; an unknown
//     verb name returns a structured error.
//   - "output" (string): "text" (default), "summary", or "raw" per CR-0051.
//
// The registry pointer is captured at construction time. Because Go maps are
// reference types, the help verb reflects any verbs added to the registry
// before the first call (i.e., after RegisterDomainTool completes).
//
// No external I/O or Microsoft Graph calls are performed.
//
// Parameters:
//   - registry: the VerbRegistry for this domain, populated after registration.
//
// Returns a Verb ready for use as the "help" entry in a DomainToolConfig.
func NewHelpVerb(registry *tools.VerbRegistry) tools.Verb {
	handler := func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		verbName := req.GetString("verb", "")
		output := req.GetString("output", "text")
		return Render(*registry, verbName, output)
	}

	return tools.Verb{
		Name:    "help",
		Summary: "show detailed documentation for all verbs or a single named verb",
		Handler: tools.Handler(handler),
	}
}
