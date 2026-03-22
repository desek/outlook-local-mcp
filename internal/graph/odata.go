package graph

import "strings"

// EscapeOData escapes single quotes in a string for safe embedding in OData
// filter expressions. In OData, single quotes delimit string literals, so a
// literal single quote must be represented as two consecutive single quotes.
// For example, "O'Brien" becomes "O”Brien".
//
// Parameters:
//   - s: the raw user-provided string to escape.
//
// Returns the escaped string with all single quotes doubled.
//
// Side effects: none.
func EscapeOData(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
