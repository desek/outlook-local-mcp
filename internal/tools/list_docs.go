// Package tools — this file implements the system.list_docs verb handler
// (CR-0061 Phase 2). It returns the catalog of embedded documentation
// documents with slug, title, summary, tags, and size.
package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/desek/outlook-local-mcp/internal/docs"
	"github.com/mark3labs/mcp-go/mcp"
)

// HandleListDocs returns a handler for the system.list_docs verb.
//
// The handler reads the embedded documentation catalog via [docs.Catalog] and
// formats it according to the requested output tier (text default, raw JSON).
// No Graph API calls are made; the catalog is resolved from the embedded bundle
// at call time.
//
// The response is a numbered plain-text list by default. When output="raw" the
// handler returns JSON. The "summary" mode is identical to "raw" for catalog
// output because all catalog fields are already curated.
//
// Returns a CallToolResult or an error result when the catalog cannot be read
// (broken build).
func HandleListDocs() func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		outputMode, err := ValidateOutputMode(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		entries, err := docs.Catalog()
		if err != nil {
			return mcp.NewToolResultError("list_docs: " + err.Error()), nil
		}

		if outputMode == "raw" || outputMode == "summary" {
			return mcp.NewToolResultText(formatListDocsJSON(entries)), nil
		}

		return mcp.NewToolResultText(formatListDocsText(entries)), nil
	}
}

// formatListDocsText renders the catalog as a numbered plain-text list.
func formatListDocsText(entries []docs.Entry) string {
	var b strings.Builder
	for i, e := range entries {
		fmt.Fprintf(&b, "%d. %s  (slug: %s, %d bytes)\n", i+1, e.Title, e.Slug, e.Size)
		fmt.Fprintf(&b, "   %s\n", e.Summary)
		if len(e.Tags) > 0 {
			fmt.Fprintf(&b, "   Tags: %s\n", strings.Join(e.Tags, ", "))
		}
		fmt.Fprintf(&b, "   URI: doc://outlook-local-mcp/%s\n", e.Slug)
		if i < len(entries)-1 {
			b.WriteString("\n")
		}
	}
	fmt.Fprintf(&b, "\nTotal: %d document(s)\n", len(entries))
	return b.String()
}

// formatListDocsJSON renders the catalog as a JSON array.
func formatListDocsJSON(entries []docs.Entry) string {
	var b strings.Builder
	b.WriteString("[")
	for i, e := range entries {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `{"slug":%q,"title":%q,"summary":%q,"tags":%s,"size_bytes":%d,"uri":%q}`,
			e.Slug, e.Title, e.Summary, tagsJSON(e.Tags), e.Size,
			"doc://outlook-local-mcp/"+e.Slug)
	}
	b.WriteString("]")
	return b.String()
}

// tagsJSON renders a string slice as a JSON array literal.
func tagsJSON(tags []string) string {
	if len(tags) == 0 {
		return "[]"
	}
	var b strings.Builder
	b.WriteString("[")
	for i, t := range tags {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, "%q", t)
	}
	b.WriteString("]")
	return b.String()
}
