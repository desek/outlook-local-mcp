// Package help — this file contains shared helpers used by all three tier
// renderers (CR-0060 Phase 2).
package help

import (
	"sort"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/desek/outlook-local-mcp/internal/tools"
)

// orderedVerbs returns the verbs from registry in a deterministic order:
// "help" first (when present), then all remaining verbs sorted alphabetically.
// This ordering is used consistently across all three output tiers.
//
// Parameters:
//   - registry: the VerbRegistry to extract verbs from.
//
// Returns an ordered slice of Verb values.
func orderedVerbs(registry tools.VerbRegistry) []tools.Verb {
	out := make([]tools.Verb, 0, len(registry))
	if v, ok := registry["help"]; ok {
		out = append(out, v)
	}
	rest := make([]tools.Verb, 0, len(registry))
	for name, v := range registry {
		if name != "help" {
			rest = append(rest, v)
		}
	}
	sort.Slice(rest, func(i, j int) bool { return rest[i].Name < rest[j].Name })
	return append(out, rest...)
}

// renderOne dispatches to the appropriate tier renderer for a single verb.
//
// Parameters:
//   - v: the Verb to render.
//   - output: "text" (default/empty), "summary", or "raw".
//
// Returns a CallToolResult containing the rendered documentation.
func renderOne(v tools.Verb, output string) *mcp.CallToolResult {
	return renderAll([]tools.Verb{v}, output)
}

// renderAll dispatches to the appropriate tier renderer for multiple verbs.
//
// Parameters:
//   - verbs: ordered list of Verb entries to render.
//   - output: "text" (default/empty), "summary", or "raw".
//
// Returns a CallToolResult containing the rendered documentation.
func renderAll(verbs []tools.Verb, output string) *mcp.CallToolResult {
	switch output {
	case "summary":
		return renderSummary(verbs)
	case "raw":
		return renderRaw(verbs)
	default:
		return renderText(verbs)
	}
}
