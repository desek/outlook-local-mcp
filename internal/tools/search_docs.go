// Package tools — this file implements the system.search_docs verb handler
// (CR-0061 Phase 2). It performs case-insensitive keyword search across the
// embedded documentation bundle and returns ranked results with slug, snippet,
// and 1-based line numbers.
package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/desek/outlook-local-mcp/internal/docs"
	"github.com/mark3labs/mcp-go/mcp"
)

// HandleSearchDocs returns a handler for the system.search_docs verb.
//
// The handler delegates to [docs.Search] using the required "query" parameter
// and formats the results as plain text by default. An empty query or a query
// that produces no results returns a structured zero-results response rather
// than an error, consistent with the body-escalation pattern in CLAUDE.md.
//
// Parameters extracted from the request:
//   - query (required): the search term to match across the bundle.
//   - output (optional): "text" (default) or "raw"; "summary" behaves as "raw".
//
// Returns a CallToolResult with ranked snippets.
func HandleSearchDocs() func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := strings.TrimSpace(req.GetString("query", ""))
		if query == "" {
			return mcp.NewToolResultText("No results: query is empty.\n"), nil
		}

		outputMode, err := ValidateOutputMode(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		results, err := docs.Search(query)
		if err != nil {
			return mcp.NewToolResultError("search_docs: " + err.Error()), nil
		}

		if len(results) == 0 {
			return mcp.NewToolResultText(fmt.Sprintf("No results for query %q.\n", query)), nil
		}

		if outputMode == "raw" || outputMode == "summary" {
			return mcp.NewToolResultText(formatSearchDocsJSON(results)), nil
		}

		return mcp.NewToolResultText(formatSearchDocsText(query, results)), nil
	}
}

// formatSearchDocsText renders search results as a numbered plain-text list.
func formatSearchDocsText(query string, results []docs.Result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Search results for %q (%d match(es)):\n\n", query, len(results))
	for i, r := range results {
		fmt.Fprintf(&b, "%d. [%s] lines %d–%d\n", i+1, r.Slug, r.StartLine, r.EndLine)
		b.WriteString(r.Snippet)
		if !strings.HasSuffix(r.Snippet, "\n") {
			b.WriteString("\n")
		}
		if i < len(results)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// formatSearchDocsJSON renders search results as a JSON array.
func formatSearchDocsJSON(results []docs.Result) string {
	var b strings.Builder
	b.WriteString("[")
	for i, r := range results {
		if i > 0 {
			b.WriteString(",")
		}
		snippet := strings.ReplaceAll(r.Snippet, `"`, `\"`)
		snippet = strings.ReplaceAll(snippet, "\n", `\n`)
		fmt.Fprintf(&b, `{"slug":%q,"start_line":%d,"end_line":%d,"score":%d,"snippet":%q}`,
			r.Slug, r.StartLine, r.EndLine, r.Score, snippet)
	}
	b.WriteString("]")
	return b.String()
}
