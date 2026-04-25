// Package help — this file implements the tier-2 summary JSON renderer
// (CR-0060 Phase 2).
package help

import (
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/desek/outlook-local-mcp/internal/tools"
)

// verbSummary is the intentionally curated field set for summary-tier output
// (CR-0051, extended by CR-0065). Includes name, summary, description,
// examples, see_docs, and parameters for structured LLM reasoning.
type verbSummary struct {
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

	// Parameters lists the verb's input parameters with name, type, and
	// required-ness so callers can construct valid invocations without
	// trial-and-error. Omitted when the verb takes no parameters.
	Parameters []paramSpec `json:"parameters,omitempty"`
}

// exampleJSON is the JSON representation of a tools.Example for help output.
type exampleJSON struct {
	// Args is the argument map for the example invocation.
	Args map[string]any `json:"args"`

	// Comment describes what this example demonstrates.
	Comment string `json:"comment,omitempty"`
}

// toExampleJSON converts a tools.Example slice to the JSON form used in
// summary and raw help output.
func toExampleJSON(examples []tools.Example) []exampleJSON {
	if len(examples) == 0 {
		return nil
	}
	out := make([]exampleJSON, len(examples))
	for i, ex := range examples {
		out[i] = exampleJSON{Args: ex.Args, Comment: ex.Comment}
	}
	return out
}

// renderSummary produces compact JSON for the given list of verbs (tier 2
// output). The JSON object contains a single "operations" key whose value is
// an array of verbSummary objects, one per verb.
//
// Parameters:
//   - verbs: ordered list of Verb entries to document.
//
// Returns a text CallToolResult containing the JSON, or an error result if
// JSON marshalling fails (should never happen in practice).
func renderSummary(verbs []tools.Verb) *mcp.CallToolResult {
	summaries := make([]verbSummary, len(verbs))
	for i, v := range verbs {
		summaries[i] = verbSummary{
			Name:        v.Name,
			Summary:     v.Summary,
			Description: v.Description,
			Examples:    toExampleJSON(v.Examples),
			SeeDocs:     v.SeeDocs,
			Parameters:  verbParameters(v),
		}
	}

	payload := map[string]any{"operations": summaries}
	data, err := json.Marshal(payload)
	if err != nil {
		return mcp.NewToolResultError("internal error: failed to marshal summary JSON: " + err.Error())
	}
	return mcp.NewToolResultText(string(data))
}
