// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains the dispatch overhead benchmark required by CR-0060 AC-13
// and NFR-2. It measures the added latency from the aggregate dispatch layer
// (operation routing) over a no-op stub handler, confirming the p99 overhead
// is within the 1 ms budget.
package tools

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// BenchmarkDispatch_Overhead measures the dispatch overhead added by
// buildDispatchHandler routing an incoming MCP call to a no-op stub handler
// (AC-13 / NFR-2: p99 added latency must be ≤ 1 ms).
//
// The benchmark isolates the map lookup + argument extraction performed by
// the dispatcher itself, excluding any handler business logic, by using a
// stub handler that returns immediately. Run with -benchtime=5s for a stable
// measurement. Results should be recorded in docs/cr/CR-0060-validation-report.md.
func BenchmarkDispatch_Overhead(b *testing.B) {
	// Build a minimal registry with one verb that does nothing.
	registry := VerbRegistry{
		"do_thing": {
			Name:    "do_thing",
			Summary: "bench stub",
			Handler: func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return mcp.NewToolResultText("ok"), nil
			},
		},
	}

	dispatch := buildDispatchHandler("bench", registry)

	req := buildRequest("bench", map[string]any{"operation": "do_thing"})
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		result, err := dispatch(ctx, req)
		if err != nil || result == nil || result.IsError {
			b.Fatalf("unexpected dispatch error or nil result")
		}
	}
}
