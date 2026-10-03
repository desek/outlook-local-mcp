// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file holds the page bound and the truncation marker shared by the Teams
// listing verbs. Each of those verbs reads exactly one Graph page, so the caller
// must be told how large that page may be and whether Graph holds more; a page
// presented without that statement reads as the whole collection.
//
// @agents-index: Page-size bound and truncation marker for the Teams listing
// verbs, which read one Graph page and must say when more exist.
package tools

import (
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// teamsMaxPageSize is the largest $top the Teams message and chat collections
// accept; Graph documents 50 as the maximum for each of them.
const teamsMaxPageSize = 50

// teamsPageSize reads the optional max_results argument and clamps it to the
// range Graph accepts, so an out-of-range value narrows the page instead of
// causing a Graph refusal. An absent or non-positive value yields the maximum.
func teamsPageSize(request mcp.CallToolRequest) int32 {
	size := int(request.GetFloat("max_results", teamsMaxPageSize))
	if size < 1 || size > teamsMaxPageSize {
		size = teamsMaxPageSize
	}
	return int32(size)
}

// teamsTruncationNote states that Graph holds more records than this page, and
// names the two corrections: a narrower read through teams search, or a larger
// max_results when the page was below the maximum.
func teamsTruncationNote(noun string, shown int) string {
	return fmt.Sprintf("truncated: true. Graph holds more %s than the %d shown; this listing reads one page only. "+
		"Fix: find the item with the teams search operation, or raise max_results (at most %d) if it is lower. "+
		"Verify: the item you need appears in the result.", noun, shown, teamsMaxPageSize)
}

// teamsPagedText rewrites a formatter's "N x total." footer when Graph signalled
// more pages, because the count is then only the size of this page, and appends
// the truncation note. A complete page is returned unchanged.
func teamsPagedText(text, noun string, shown int, more bool) string {
	if !more {
		return text
	}
	text = strings.TrimSuffix(text, " total.") + " shown on this page."
	return text + "\n" + teamsTruncationNote(noun, shown)
}

// teamsPagedJSON returns the JSON tiers' result. The record array stays the
// first content block so its shape does not change; when Graph signalled more
// pages, a second text block carries the truncation note.
func teamsPagedJSON(jsonText, noun string, shown int, more bool) *mcp.CallToolResult {
	result := mcp.NewToolResultText(jsonText)
	if more {
		result.Content = append(result.Content, mcp.NewTextContent(teamsTruncationNote(noun, shown)))
	}
	return result
}
