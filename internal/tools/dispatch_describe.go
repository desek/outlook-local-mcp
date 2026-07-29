// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the description composer for domain aggregate tools
// (CR-0060). It builds the top-level tool description that lists every
// registered verb with its one-line summary, and constructs the operation
// enum passed to mcp.NewTool as a JSON Schema constraint.
package tools

import (
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// buildOperationEnum returns a slice of verb names extracted from the given
// registry in the order they were appended to the verbs slice. The returned
// slice is used as the value for mcp.Enum in the aggregate tool's `operation`
// parameter, so the MCP client sees only the verbs registered at server start
// (after feature-flag gating).
//
// Parameters:
//   - verbs: the ordered slice of Verb descriptors for a domain.
//
// Returns a slice of operation name strings, including "help" if present.
func buildOperationEnum(verbs []Verb) []string {
	names := make([]string, 0, len(verbs))
	for _, v := range verbs {
		names = append(names, v.Name)
	}
	return names
}

// buildTopLevelDescription composes the top-level description string for an
// aggregate domain tool. The description begins with the provided intro
// sentence, followed by a verb inventory in which every registered verb
// occupies its own line. Each line names the verb, its one-line summary, and
// the parameters that verb requires so an MCP client can select a verb and
// construct its arguments from tools/list alone, without a help round-trip
// (CR-0068 FR-9, FR-10, OBS-2, OBS-4).
//
// The required-parameter list per verb is derived from the verb's own Schema
// (via verbRequiredParams) rather than hand-written per verb, so it cannot
// drift from the schema the dispatcher validates against.
//
// The intro SHOULD be a short sentence identifying the domain (e.g.,
// "Calendar operations for Microsoft Graph."). For the mail domain the intro
// additionally discloses the gated write verbs and their configuration keys
// (CR-0068 FR-11); that disclosure is supplied by the caller in the intro
// string, not synthesised here.
//
// Each verb summary MUST be ≤80 characters per AC-4 / FR-3. The composed
// description MUST stay under 4000 characters per NFR-3; the derived per-verb
// required-parameter lists keep growth bounded.
//
// Parameters:
//   - intro: a one-sentence domain description prepended to the verb list.
//   - verbs: the ordered slice of Verb descriptors for the domain.
//
// Returns the composed description string ready for use as the mcp.Tool
// description argument.
func buildTopLevelDescription(intro string, verbs []Verb) string {
	if len(verbs) == 0 {
		return intro
	}

	var b strings.Builder
	b.WriteString(intro)
	b.WriteString("\n\nSet the required `operation` parameter to one of the verbs below. ")
	b.WriteString("Each line names the verb, what it does, and the parameters that verb requires. ")
	b.WriteString("Optional parameters are omitted here; call operation=\"help\" for the full parameter reference.")

	for _, v := range verbs {
		b.WriteString("\n- `")
		b.WriteString(v.Name)
		b.WriteString("`")
		if v.Summary != "" {
			b.WriteString(": ")
			b.WriteString(v.Summary)
		}
		b.WriteString(". ")

		required := verbRequiredParams(v)
		if len(required) == 0 {
			b.WriteString("No required parameters.")
		} else {
			b.WriteString("Requires: ")
			b.WriteString(strings.Join(required, ", "))
			b.WriteString(".")
		}
	}

	return b.String()
}

// verbRequiredParams returns the names of the parameters that the given verb
// requires, derived from the verb's Schema ToolOptions rather than hand-written
// per verb (CR-0068 FR-9). It materialises the schema by applying the options
// to a throwaway mcp.Tool and reading the resulting InputSchema.Required slice,
// the same materialisation strategy used for verb annotations.
//
// The implicit "operation" selector is never part of a verb's own Schema (it is
// added only at aggregate registration time), so it never appears here. The
// returned order follows the schema's Required slice. A verb with no required
// parameters yields an empty slice.
//
// The function is pure: it performs no I/O and does not mutate the verb.
func verbRequiredParams(v Verb) []string {
	if len(v.Schema) == 0 {
		return nil
	}
	t := mcp.NewTool("_introspect", v.Schema...)
	required := t.InputSchema.Required
	out := make([]string, 0, len(required))
	for _, name := range required {
		if name == "operation" {
			continue
		}
		out = append(out, name)
	}
	return out
}
