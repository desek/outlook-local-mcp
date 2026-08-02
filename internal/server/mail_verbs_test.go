// Package server — this file holds the description-derived test that binds the
// caller-facing search_messages documentation to the runtime normalisation, so
// a documented example Graph would reject fails the build rather than shipping.
//
// The test lives in the server package, not tools, because the examples it reads
// are owned by buildSearchMessagesVerb here, and server imports tools (not the
// reverse), so it can call the tools-package normalisation while a tools-package
// test could not reach the verb registry.
package server

import (
	"regexp"
	"testing"

	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/tools"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// forbiddenPropertyPhrase matches the double-quoted property phrase form
// (subject:"Design Review") that measured behaviour shows Graph rejects. Its
// presence anywhere in the verb description is the specific drift requirement 9
// prohibits, so the description prose is guarded against it directly rather than
// only through the extracted examples.
var forbiddenPropertyPhrase = regexp.MustCompile(`\w+:"`)

// TestDocumentedExamplesAreCanonical asserts that every example documented by
// the search_messages verb is already in the canonical form Microsoft Graph
// accepts, deriving its cases from the verb registry entry rather than from a
// hand-maintained list.
//
// Two assertions run against each documented example:
//
//  1. It normalises without error, so no documented example teaches a query the
//     verb would refuse.
//  2. NormaliseSearchQuery(example) equals example, so each example is already a
//     normalisation fixed point.
//
// The second assertion is what gives the check teeth against re-introduction. A
// re-added double-quoted example such as subject:"Design Review" normalises to
// the parenthesised form "subject:(Design Review)", so the equality fails and
// the build fails. Documentation is deliberately held to a stricter standard
// than caller input: normalisation still accepts and translates the
// double-quoted form at runtime for callers (requirement 2), but a documented
// example must already be canonical. A later reader must read that asymmetry as
// intentional, not as an inconsistency between this test and the runtime.
//
// The description prose is additionally guarded against the forbidden
// double-quoted property phrase, because that is the surface the broken syntax
// lived on and prose examples are not carried in the structured Examples slice.
func TestDocumentedExamplesAreCanonical(t *testing.T) {
	// The wrap stub is never invoked; the test reads the verb's documentation,
	// not its handler. A nil Handler is sufficient.
	wrap := func(_ string, _ string, _ mcpserver.ToolHandlerFunc) tools.Handler {
		return nil
	}
	verb := buildSearchMessagesVerb(mailVerbsConfig{}, graph.RetryConfig{}, wrap)

	if len(verb.Examples) == 0 {
		t.Fatal("search_messages verb documents no examples; expected at least one to govern")
	}

	for i, ex := range verb.Examples {
		raw, ok := ex.Args["query"]
		if !ok {
			t.Fatalf("example %d has no query argument to check", i)
		}
		example, ok := raw.(string)
		if !ok {
			t.Fatalf("example %d query argument is %T, want string", i, raw)
		}

		normalised, err := tools.NormaliseSearchQuery(example)
		if err != nil {
			t.Errorf("documented example %q does not normalise: %v", example, err)
			continue
		}
		if normalised != example {
			t.Errorf("documented example %q is not canonical: normalises to %q; write the canonical form so a documented example is a normalisation fixed point", example, normalised)
		}
	}

	if forbiddenPropertyPhrase.MatchString(verb.Description) {
		t.Errorf("verb description documents the double-quoted property phrase form Graph rejects; write a multi-word property value in parentheses, for example subject:(Design Review)")
	}
}
