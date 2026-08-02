// Package tools — this file implements the system.get_docs verb handler
// (CR-0061 Phase 2). It fetches a document (or a specific section) from the
// embedded bundle by slug, with optional section slicing by heading anchor and
// optional output-tier selection (text default, raw markdown).
package tools

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/desek/outlook-local-mcp/internal/docs"
	"github.com/mark3labs/mcp-go/mcp"
)

// headingAnchorTagRe matches a trailing "{#custom-id}" anchor tag on a heading
// line and captures the identifier. It is anchored to the end of the line ($)
// with only optional trailing whitespace after the closing brace, so it matches
// the GitHub-flavoured Markdown explicit-anchor convention only when it appears
// as a trailing tag — a heading that legitimately contains "{#...}" earlier in
// its text is left untouched (CR-0074 Risk 2).
var headingAnchorTagRe = regexp.MustCompile(`\s*\{#([^}]+)\}\s*$`)

// HandleGetDocs returns a handler for the system.get_docs verb.
//
// The handler reads the document identified by the required "slug" parameter
// from the embedded bundle. When "section" is provided it extracts the body of
// the matching H2 heading (case-insensitive anchor match). When "output" is
// "raw" the unmodified markdown is returned; the default "text" output strips
// leading/trailing whitespace.
//
// Parameters extracted from the request:
//   - slug (required): the document identifier (e.g., "troubleshooting").
//   - section (optional): heading anchor to extract (e.g., "token-refresh").
//   - output (optional): "text" (default) or "raw".
//
// Returns mcp.NewToolResultError when the slug is not in the bundle or the
// section heading is not found. The error is returned as a tool result (not a
// Go error) so the LLM can read it.
func HandleGetDocs() func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		slug := strings.TrimSpace(req.GetString("slug", ""))
		if slug == "" {
			return mcp.NewToolResultError("get_docs: 'slug' parameter is required"), nil
		}

		section := strings.TrimSpace(req.GetString("section", ""))

		outputMode, err := ValidateOutputMode(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		data, err := docs.ReadSlug(slug)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("get_docs: unknown slug %q — use list_docs to see available documents", slug)), nil
		}

		content := string(data)
		if section != "" {
			extracted, extractErr := extractSection(content, section)
			if extractErr != nil {
				return mcp.NewToolResultError(fmt.Sprintf("get_docs: section %q not found in %q", section, slug)), nil
			}
			content = extracted
		}

		if outputMode == "raw" {
			return mcp.NewToolResultText(content), nil
		}

		// text mode: trim surrounding whitespace.
		return mcp.NewToolResultText(strings.TrimSpace(content) + "\n"), nil
	}
}

// extractSection returns the body of the first H2 heading whose anchor matches
// the given section string. The anchor is computed by lower-casing the heading
// text and replacing spaces with hyphens (GitHub-flavoured markdown convention).
//
// The returned string includes the heading line and all subsequent lines until
// the next H2 heading (or end of file).
//
// Parameters:
//   - content: the full markdown document text.
//   - section: the heading anchor to match (e.g., "token-refresh").
//
// Returns the matched section text, or an error when the anchor is not found.
func extractSection(content, section string) (string, error) {
	lines := strings.Split(content, "\n")
	target := strings.ToLower(strings.TrimSpace(section))

	start := -1
	for i, line := range lines {
		if !strings.HasPrefix(line, "## ") {
			continue
		}
		heading := strings.TrimPrefix(line, "## ")
		anchor := headingToAnchor(heading)
		if anchor == target {
			start = i
			break
		}
	}
	if start == -1 {
		return "", fmt.Errorf("section %q not found", section)
	}

	// Collect lines from start until the next H2 (or EOF). The heading line
	// itself has any trailing "{#...}" anchor tag stripped so the caller never
	// sees the raw markup (CR-0074 FR-4).
	var out []string
	for i := start; i < len(lines); i++ {
		if i > start && strings.HasPrefix(lines[i], "## ") {
			break
		}
		line := lines[i]
		if i == start {
			line = headingAnchorTagRe.ReplaceAllString(line, "")
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n"), nil
}

// headingToAnchor converts a markdown heading string to the anchor get_docs
// resolves it under.
//
// When the heading ends with an explicit "{#custom-id}" tag, that identifier is
// used verbatim (lower-cased) and the heading text is ignored (CR-0074 FR-1).
// This is deliberately exclusive: a heading carrying an explicit anchor does not
// also resolve under its text-derived form, so the author's explicit choice
// stays authoritative and two names never address one section (CR-0074 FR-3).
//
// Otherwise the anchor is derived from the heading text in the GitHub-flavoured
// form: lower-case, spaces replaced by hyphens, non-alphanumeric characters
// (except hyphens) removed (CR-0074 FR-2).
func headingToAnchor(heading string) string {
	heading = strings.TrimSpace(heading)
	if m := headingAnchorTagRe.FindStringSubmatch(heading); m != nil {
		return strings.ToLower(strings.TrimSpace(m[1]))
	}
	heading = strings.ToLower(heading)
	var b strings.Builder
	for _, r := range heading {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune('-')
		}
	}
	return b.String()
}
