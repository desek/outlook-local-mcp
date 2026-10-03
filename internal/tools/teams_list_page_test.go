// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file holds the request-recording fake the Teams listing tests use to
// assert the query options and headers actually sent, and the tests of the
// shared page bound and truncation marker.
//
// @agents-index: Query-and-header recording fake for the Teams listing tests,
// plus tests of the shared page-size clamp and truncation marker.
package tools

import (
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// teamsQueryRecorder answers every request with a canned body and records the
// decoded query and the Prefer header, so a test asserts what reached the
// service rather than what the handler intended to send.
type teamsQueryRecorder struct {
	mu       sync.Mutex
	queries  []url.Values
	prefers  []string
	response string
}

// ServeHTTP records the request and answers with the canned response.
func (r *teamsQueryRecorder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mu.Lock()
	r.queries = append(r.queries, req.URL.Query())
	r.prefers = append(r.prefers, strings.Join(req.Header.Values("Prefer"), ","))
	r.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	//nolint:errcheck // test helper
	w.Write([]byte(r.response))
}

// runTeamsQueryHandler runs a Teams handler against a query recorder and
// returns the result with the single recorded query and Prefer header.
func runTeamsQueryHandler(t *testing.T, response string, ctor teamsHandlerCtor, args map[string]any) (*mcp.CallToolResult, url.Values, string) {
	t.Helper()
	rec := &teamsQueryRecorder{response: response}
	result := runTeamsHandlerWith(t, rec, ctor, args, 30*time.Second)
	if result.IsError {
		t.Fatalf("expected success, got %q", resultText(t, result))
	}
	if len(rec.queries) != 1 {
		t.Fatalf("graph request count = %d, want 1", len(rec.queries))
	}
	return result, rec.queries[0], rec.prefers[0]
}

// withNextLink adds an @odata.nextLink to a canned collection body, which is
// how Graph signals that the page is not the whole collection.
func withNextLink(body string) string {
	return strings.Replace(body, "{", `{"@odata.nextLink": "https://graph.microsoft.com/v1.0/next?$skiptoken=x",`, 1)
}

// TestTeamsPageSize_ClampsToGraphMaximum validates the default and the clamp:
// Graph refuses $top above 50, so larger or invalid values fall back to 50.
func TestTeamsPageSize_ClampsToGraphMaximum(t *testing.T) {
	cases := map[string]struct {
		args map[string]any
		want int32
	}{
		"absent":   {map[string]any{}, 50},
		"in range": {map[string]any{"max_results": float64(7)}, 7},
		"too big":  {map[string]any{"max_results": float64(500)}, 50},
		"zero":     {map[string]any{"max_results": float64(0)}, 50},
	}
	for name, tc := range cases {
		req := mcp.CallToolRequest{}
		req.Params.Arguments = tc.args
		if got := teamsPageSize(req); got != tc.want {
			t.Errorf("%s: page size = %d, want %d", name, got, tc.want)
		}
	}
}

// TestTeamsPagedText_RewritesTotalOnlyWhenMore validates that a complete page
// keeps its total and a truncated one no longer claims to be the total.
func TestTeamsPagedText_RewritesTotalOnlyWhenMore(t *testing.T) {
	if got := teamsPagedText("x\n2 message(s) total.", "messages", 2, false); got != "x\n2 message(s) total." {
		t.Errorf("complete page changed: %q", got)
	}
	got := teamsPagedText("x\n2 message(s) total.", "messages", 2, true)
	if strings.Contains(got, "total.") || !strings.Contains(got, "2 message(s) shown on this page.") || !strings.Contains(got, "truncated: true") {
		t.Errorf("truncated page text = %q", got)
	}
}
