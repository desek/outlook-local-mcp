// Package help — this file implements the tier-3 raw JSON renderer
// (CR-0060 Phase 2).
package help

import (
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/desek/outlook-local-mcp/internal/tools"
)

// verbRaw is the full structured representation of a verb for raw-tier output
// (CR-0060, extended by CR-0065). Includes name, summary, description,
// examples, see_docs, and parameters. Fields that require runtime introspection
// of mcp.ToolOption values (Annotations, Schema) are omitted because those are
// opaque function values.
type verbRaw struct {
	// Name is the operation identifier.
	Name string `json:"name"`

	// Summary is the ≤80-character human-readable description.
	Summary string `json:"summary"`

	// Description is the full prose explanation of the verb (CR-0065 FR-10).
	Description string `json:"description,omitempty"`

	// Examples holds illustrative invocations (CR-0065 FR-10).
	Examples []exampleJSON `json:"examples,omitempty"`

	// SeeDocs holds documentation references in slug or slug#anchor form (CR-0065 FR-10).
	SeeDocs []string `json:"see_docs,omitempty"`

	// Parameters lists the verb's input parameters extracted from the Schema
	// options (name, type, required, description, enum). Omitted when the
	// verb takes no parameters.
	Parameters []paramSpec `json:"parameters,omitempty"`
}

// renderRaw produces the full structured JSON payload for the given list of
// verbs (tier 3 output). The JSON object contains a single "operations" key
// whose value is an array of verbRaw objects.
//
// Per CR-0060 FR-4 / help verb contract, raw output is the most complete
// machine-readable representation available at render time. Fields that
// cannot be serialised (handler func, mcp.ToolOption slices) are excluded.
//
// Parameters:
//   - verbs: ordered list of Verb entries to document.
//
// Returns a text CallToolResult containing the JSON, or an error result if
// JSON marshalling fails (should never happen in practice).
func renderRaw(verbs []tools.Verb) *mcp.CallToolResult {
	raws := make([]verbRaw, len(verbs))
	for i, v := range verbs {
		raws[i] = verbRaw{
			Name:        v.Name,
			Summary:     v.Summary,
			Description: v.Description,
			Examples:    toExampleJSON(v.Examples),
			SeeDocs:     v.SeeDocs,
			Parameters:  verbParameters(v),
		}
	}

	payload := map[string]any{"operations": raws}
	data, err := json.Marshal(payload)
	if err != nil {
		return mcp.NewToolResultError("internal error: failed to marshal raw JSON: " + err.Error())
	}
	return mcp.NewToolResultText(string(data))
}
