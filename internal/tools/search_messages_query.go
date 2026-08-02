// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the pure query-normalisation seam for the search_messages
// tool. Microsoft Graph requires a message $search value to be enclosed in
// double quotes, and it rejects the double-quoted property phrase the tool's
// own description teaches. An unquoted multi-word query is worse than rejected:
// Graph accepts it, binds only the first token, and returns the newest
// unrelated messages as if they matched. This function converts a caller's
// query into a value Graph accepts while preserving the caller's intent, with
// no Graph dependency and no network call, so the quoting decision is testable
// without an account.
//
// @agents-index: Pure normalisation of a search_messages KQL query into a
// Graph-accepted $search value, preserving caller intent.
package tools

import (
	"fmt"
	"regexp"
	"strings"
)

// propertyPhrasePattern matches a property value delimited by double quotes,
// the phrase form the verb description documents (for example subject:"Design
// Review"). The first group captures the property keyword and the second
// captures the value between the quotes. The pattern fires only on a double
// quote immediately following a property keyword and its colon; anything else
// is left untouched, so the rewrite cannot mis-parse a query it should have
// passed through.
var propertyPhrasePattern = regexp.MustCompile(`(\w+):"([^"]*)"`)

// NormaliseSearchQuery converts a caller's search_messages query into a value
// Microsoft Graph accepts as a message $search value, preserving the caller's
// intent rather than merely producing something legal.
//
// The four behaviours are applied in this order:
//
//  1. Translate a property value delimited by double quotes into the
//     parenthesised form: subject:"Design Review" becomes subject:(Design
//     Review). The parenthesised form scopes to the property; the double-quoted
//     form is rejected by Graph.
//  2. Pass through unchanged a query already enclosed in one matched pair of
//     double quotes with no remaining interior double quote, so a caller who
//     found the hand-quoted workaround keeps working.
//  3. Wrap a query carrying no double quote in exactly one enclosing pair, so a
//     multi-word query filters rather than being silently discarded.
//  4. Reject a double quote that neither delimits a property value nor forms the
//     single enclosing pair, before any Graph call, returning an error that
//     names the parenthesised form as the correction.
//
// Deleting a quoted property value's inner quotes is deliberately not a
// behaviour: it always yields a legal expression but binds only the first token
// to the property and turns the rest into free-text terms, a silent wrong
// answer worse than an error.
//
// The function is deterministic and idempotent: normalising an already-
// normalised query returns it unchanged.
//
// It returns the normalised query and a nil error on success, or an empty
// string and an actionable error when the query cannot be translated.
func NormaliseSearchQuery(query string) (string, error) {
	// Behaviour 1: rewrite every property phrase to the parenthesised form.
	// Doing this first means an inner property phrase inside an over-wrapped
	// query is translated away before the enclosing-pair check runs.
	translated := propertyPhrasePattern.ReplaceAllString(query, "$1:($2)")

	// Behaviour 2: an already-enclosed query with no interior double quote is
	// the canonical wire form and passes through byte-identically. Checked
	// before the reject in behaviour 4 so a hand-quoted query is preserved.
	if isSingleEnclosedPair(translated) {
		return translated, nil
	}

	// Behaviour 3: a query with no double quote left gains exactly one pair.
	if !strings.Contains(translated, `"`) {
		return `"` + translated + `"`, nil
	}

	// Behaviour 4: a stray double quote that delimits nothing is refused with a
	// correction, rather than passed to Graph to fail with a character-position
	// parse error that names neither the cause nor the fix.
	return "", fmt.Errorf(
		"search query %q contains a double quote that does not delimit a property value or enclose the whole query; "+
			"write a multi-word property value in parentheses, for example subject:(Design Review), "+
			"and remove any other double quote, then retry",
		query,
	)
}

// isSingleEnclosedPair reports whether s is enclosed in exactly one matched pair
// of double quotes with no interior double quote, for example "subject:Contoso".
// It is the test for behaviour 2 of NormaliseSearchQuery: such a query is
// already a valid Graph $search value and must pass through unchanged.
func isSingleEnclosedPair(s string) bool {
	return len(s) >= 2 &&
		strings.HasPrefix(s, `"`) &&
		strings.HasSuffix(s, `"`) &&
		strings.Count(s, `"`) == 2
}
