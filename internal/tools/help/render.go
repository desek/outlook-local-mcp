// Package help provides the help renderer for domain aggregate tools (CR-0060
// Phase 2). It takes a verb registry and emits documentation in three output
// tiers: text (tier 1, default), summary JSON (tier 2), and raw JSON (tier 3).
//
// The renderer is intentionally pure: it performs no I/O, no Microsoft Graph
// calls, and no external network activity. All output is derived solely from
// the Verb entries present in the supplied registry.
//
// Usage:
//
//	result, err := help.Render(registry, verb, output)
//
// Where verb is the optional verb name to scope docs to (empty = all verbs),
// and output is one of "text" (default), "summary", or "raw".
package help

import (
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/desek/outlook-local-mcp/internal/tools"
)

// Render produces help output for the given verb registry, optionally scoped
// to a single verb. It honours the CR-0051 three-tier output contract.
//
// Parameters:
//   - registry: the populated VerbRegistry for the domain.
//   - verbName: if non-empty, output is scoped to the named verb; an unknown
//     verb name returns a structured error result.
//   - output: "text" (default/empty), "summary", or "raw".
//
// Returns a CallToolResult containing the rendered documentation, or a
// structured error result if verbName is not found in the registry.
//
// Never returns a non-nil Go error; all error conditions are expressed as
// structured MCP error results so the LLM receives an actionable message.
func Render(registry tools.VerbRegistry, verbName, output string) (*mcp.CallToolResult, error) {
	// Scope to a single verb when requested.
	if verbName != "" {
		v, ok := registry[verbName]
		if !ok {
			return mcp.NewToolResultError(
				fmt.Sprintf("unknown verb %q. Call with operation=\"help\" (no verb argument) to see all supported verbs.", verbName),
			), nil
		}
		return renderOne(v, output), nil
	}

	// All verbs.
	verbs := orderedVerbs(registry)
	return renderAll(verbs, output), nil
}
