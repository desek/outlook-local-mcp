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
// sentence, followed by "Required `operation`:" and then a comma-separated
// list of "name (summary)" entries for every registered verb.
//
// The intro SHOULD be a short sentence identifying the domain (e.g.,
// "Calendar operations for Microsoft Graph.").
//
// Each verb summary MUST be ≤80 characters per AC-4 / FR-3.
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
	b.WriteString(" Required `operation`: ")

	for i, v := range verbs {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("`")
		b.WriteString(v.Name)
		b.WriteString("`")
		if v.Summary != "" {
			b.WriteString(" (")
			b.WriteString(v.Summary)
			b.WriteString(")")
		}
	}

	b.WriteString(".")
	return b.String()
}
