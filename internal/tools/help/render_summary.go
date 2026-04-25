// Package help — this file implements the tier-2 summary JSON renderer
// (CR-0060 Phase 2).
package help

import (
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/desek/outlook-local-mcp/internal/tools"
)

// verbSummary is the intentionally curated field set for summary-tier output
// (CR-0051). Only {name, summary} are included; all other Verb fields are
// omitted to keep token consumption low.
type verbSummary struct {
	// Name is the operation identifier.
	Name string `json:"name"`

	// Summary is the ≤80-character human-readable description.
	Summary string `json:"summary"`
}

// renderSummary produces compact JSON for the given list of verbs (tier 2
// output). The JSON object contains a single "operations" key whose value is
// an array of {name, summary} objects, one per verb.
//
// Parameters:
//   - verbs: ordered list of Verb entries to document.
//
// Returns a text CallToolResult containing the JSON, or an error result if
// JSON marshalling fails (should never happen in practice).
func renderSummary(verbs []tools.Verb) *mcp.CallToolResult {
	summaries := make([]verbSummary, len(verbs))
	for i, v := range verbs {
		summaries[i] = verbSummary{Name: v.Name, Summary: v.Summary}
	}

	payload := map[string]any{"operations": summaries}
	data, err := json.Marshal(payload)
	if err != nil {
		return mcp.NewToolResultError("internal error: failed to marshal summary JSON: " + err.Error())
	}
	return mcp.NewToolResultText(string(data))
}
