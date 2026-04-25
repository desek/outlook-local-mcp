// Package help — this file implements the tier-1 text renderer (CR-0060 Phase 2).
package help

import (
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/desek/outlook-local-mcp/internal/tools"
)

// paramReqLabel returns the required/optional label rendered in text output.
func paramReqLabel(required bool) string {
	if required {
		return "required"
	}
	return "optional"
}

// renderText produces a CLI-like plain-text help document for the given list
// of verbs (tier 1 output). Each verb is rendered as a numbered section with
// its name and summary. This is the default output mode when `output` is
// empty or "text".
//
// Parameters:
//   - verbs: ordered list of Verb entries to document.
//
// Returns a text CallToolResult.
func renderText(verbs []tools.Verb) *mcp.CallToolResult {
	var b strings.Builder
	for i, v := range verbs {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(formatVerbText(v, i+1))
	}
	return mcp.NewToolResultText(b.String())
}

// formatVerbText formats a single Verb as a numbered plain-text entry.
//
// Parameters:
//   - v: the Verb to format.
//   - n: the 1-based index in the listing.
//
// Returns the formatted string for the verb entry.
func formatVerbText(v tools.Verb, n int) string {
	var b strings.Builder
	b.WriteString(formatInt(n))
	b.WriteString(". ")
	b.WriteString(v.Name)
	b.WriteString("\n")
	if v.Summary != "" {
		b.WriteString("   ")
		b.WriteString(v.Summary)
		b.WriteString("\n")
	}
	params := verbParameters(v)
	if len(params) > 0 {
		b.WriteString("   Parameters:\n")
		for _, p := range params {
			b.WriteString("     ")
			b.WriteString(p.Name)
			b.WriteString(" (")
			if p.Type != "" {
				b.WriteString(p.Type)
				b.WriteString(", ")
			}
			b.WriteString(paramReqLabel(p.Required))
			b.WriteString(")")
			if p.Description != "" {
				b.WriteString("  ")
				b.WriteString(p.Description)
			}
			if len(p.Enum) > 0 {
				b.WriteString(" [")
				b.WriteString(strings.Join(p.Enum, "|"))
				b.WriteString("]")
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}

// formatInt converts a small positive integer to a decimal string without
// importing strconv to keep the file self-contained.
func formatInt(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
