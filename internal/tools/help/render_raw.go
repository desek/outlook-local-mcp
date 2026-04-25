// Package help — this file implements the tier-3 raw JSON renderer
// (CR-0060 Phase 2).
package help

import (
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/desek/outlook-local-mcp/internal/tools"
)

// verbRaw is the full structured representation of a verb for raw-tier output.
// It includes every field that can be derived from the Verb struct at render
// time. Fields that require runtime introspection of mcp.ToolOption values
// (Annotations, Schema) are omitted because those are opaque function values.
type verbRaw struct {
	// Name is the operation identifier.
	Name string `json:"name"`

	// Summary is the ≤80-character human-readable description.
	Summary string `json:"summary"`
}

// renderRaw produces the full structured JSON payload for the given list of
// verbs (tier 3 output). The JSON object contains a single "operations" key
// whose value is an array of verbRaw objects.
//
// Per CR-0060 FR-4 / help verb contract, raw output is the most complete
// machine-readable representation available at render time. Fields that
// cannot be serialised (handler func, mcp.ToolOption slices) are excluded;
// the summary and name are always present.
//
// Parameters:
//   - verbs: ordered list of Verb entries to document.
//
// Returns a text CallToolResult containing the JSON, or an error result if
// JSON marshalling fails (should never happen in practice).
func renderRaw(verbs []tools.Verb) *mcp.CallToolResult {
	raws := make([]verbRaw, len(verbs))
	for i, v := range verbs {
		raws[i] = verbRaw{Name: v.Name, Summary: v.Summary}
	}

	payload := map[string]any{"operations": raws}
	data, err := json.Marshal(payload)
	if err != nil {
		return mcp.NewToolResultError("internal error: failed to marshal raw JSON: " + err.Error())
	}
	return mcp.NewToolResultText(string(data))
}
