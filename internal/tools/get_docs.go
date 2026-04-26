// Package tools — this file implements the system.get_docs verb handler
// (CR-0061 Phase 2). It fetches a document (or a specific section) from the
// embedded bundle by slug, with optional section slicing by heading anchor and
// optional output-tier selection (text default, raw markdown).
package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/desek/outlook-local-mcp/internal/docs"
	"github.com/mark3labs/mcp-go/mcp"
)

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

	// Collect lines from start until the next H2 (or EOF).
	var out []string
	for i := start; i < len(lines); i++ {
		if i > start && strings.HasPrefix(lines[i], "## ") {
			break
		}
		out = append(out, lines[i])
	}
	return strings.Join(out, "\n"), nil
}

// headingToAnchor converts a markdown heading string to its GitHub-flavoured
// anchor form: lower-case, spaces replaced by hyphens, non-alphanumeric
// characters (except hyphens) removed.
func headingToAnchor(heading string) string {
	heading = strings.ToLower(strings.TrimSpace(heading))
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
