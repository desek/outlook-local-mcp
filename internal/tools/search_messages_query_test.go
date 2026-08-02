// Package tools tests the pure search_messages query-normalisation seam.
//
// The authoritative specification for these cases is the twelve rows of
// behaviour measured against a live mailbox: each row records a value that
// reached Microsoft Graph and the outcome Graph produced. They are treated as
// empirical fact here. Where an implementation choice would disagree with a
// measured row, the row wins.
//
// @agents-index: Tests for NormaliseSearchQuery, driven by the twelve measured
// Graph-behaviour rows plus the ordered-behaviour named cases.
package tools

import (
	"strings"
	"testing"
)

// measuredRow encodes one of the twelve rows measured against a live mailbox,
// re-read here as a normalisation fixture. The Value is fed to
// NormaliseSearchQuery as a caller query; Want is the value the function must
// produce so that a value Graph accepts and interprets correctly reaches Graph.
type measuredRow struct {
	// Num is the row number in the CR's Measured behaviour table, so a reader
	// can trace each fixture back to the observation that justifies it.
	Num int
	// Value is the caller query, the string the row observed reaching Graph.
	Value string
	// Want is the required normalised output.
	Want string
	// Why records the measured Graph outcome the fixture defends against.
	Why string
}

// measuredRows is the twelve-row fixture corpus. None of the twelve triggers a
// rejection: rows 1, 3, and 4 carry no double quote and are wrapped; the rest
// are already wrapped and either pass through or have an inner property phrase
// translated. The stray-quote rejection is exercised by TestStrayQuoteIsRejected.
var measuredRows = []measuredRow{
	{1, `subject:Contoso`, `"subject:Contoso"`, "unwrapped, Graph errored on the colon; wrapping repairs it into row 2"},
	{2, `"subject:Contoso"`, `"subject:Contoso"`, "wrapped, accepted and correct; the hand-quoted workaround passes through"},
	{3, `Zzzqqxx`, `"Zzzqqxx"`, "a single bare term is accepted and correct; wrapping preserves that"},
	{4, `Zzzqqxx Wwwyyzz`, `"Zzzqqxx Wwwyyzz"`, "unwrapped multi-word returned newest unrelated mail, silently wrong; wrapping repairs it into row 5"},
	{5, `"Zzzqqxx Wwwyyzz"`, `"Zzzqqxx Wwwyyzz"`, "wrapped multi-word, accepted and correct; passes through"},
	{6, `"subject:"Contoso Quarterly""`, `"subject:(Contoso Quarterly)"`, "over-wrapped double-quoted phrase Graph rejected; the inner phrase is translated into the row 9 form"},
	{7, `"subject:'Contoso Teams'"`, `"subject:'Contoso Teams'"`, "single quotes do not scope but are not double quotes; the enclosed query passes through unchanged"},
	{8, `"subject:(Contoso Teams)"`, `"subject:(Contoso Teams)"`, "the canonical parenthesised form, accepted and scoping correctly; passes through"},
	{9, `"subject:(Contoso Quarterly)"`, `"subject:(Contoso Quarterly)"`, "parenthesised form with correct matches; passes through"},
	{10, `"subject:(daily Contoso)"`, `"subject:(daily Contoso)"`, "parenthesised form is an unordered token AND; passes through"},
	{11, `"subject:Contoso Teams"`, `"subject:Contoso Teams"`, "already wrapped by the caller; only the first token binds, a documented limitation, so it passes through unchanged"},
	{12, `"subject:Contoso Zzzqqxx"`, `"subject:Contoso Zzzqqxx"`, "already wrapped by the caller; passes through unchanged"},
}

// TestMeasuredRows asserts NormaliseSearchQuery produces the required value for
// every measured row, so each case is anchored to the observation behind it.
func TestMeasuredRows(t *testing.T) {
	for _, row := range measuredRows {
		row := row
		t.Run(row.Value, func(t *testing.T) {
			got, err := NormaliseSearchQuery(row.Value)
			if err != nil {
				t.Fatalf("row %d %q: unexpected error: %v (%s)", row.Num, row.Value, err, row.Why)
			}
			if got != row.Want {
				t.Fatalf("row %d %q: got %q, want %q (%s)", row.Num, row.Value, got, row.Want, row.Why)
			}
		})
	}
}

// TestUnquotedQueryIsWrapped pins behaviour 3: a query with no double quote
// gains exactly one enclosing pair (measured row 1).
func TestUnquotedQueryIsWrapped(t *testing.T) {
	got, err := NormaliseSearchQuery(`subject:Contoso`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != `"subject:Contoso"` {
		t.Fatalf("got %q, want %q", got, `"subject:Contoso"`)
	}
}

// TestMultiWordBareQueryIsWrapped pins behaviour 3 for the silent-discard case:
// two bare words are wrapped so they filter rather than returning newest mail
// (measured row 4).
func TestMultiWordBareQueryIsWrapped(t *testing.T) {
	got, err := NormaliseSearchQuery(`Zzzqqxx Wwwyyzz`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != `"Zzzqqxx Wwwyyzz"` {
		t.Fatalf("got %q, want %q", got, `"Zzzqqxx Wwwyyzz"`)
	}
}

// TestQuotedPropertyValueBecomesParenthesised pins behaviour 1: the documented
// double-quoted phrase form is translated to the parenthesised form Graph
// accepts (measured rows 6 and 8).
func TestQuotedPropertyValueBecomesParenthesised(t *testing.T) {
	got, err := NormaliseSearchQuery(`subject:"Design Review"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != `"subject:(Design Review)"` {
		t.Fatalf("got %q, want %q", got, `"subject:(Design Review)"`)
	}
}

// TestQuotedPropertyValueIsNotUnquotedInPlace guards the load-bearing
// prohibition: translation must never degrade to deleting the inner quotes,
// which would bind only the first token to the property (measured row 11).
func TestQuotedPropertyValueIsNotUnquotedInPlace(t *testing.T) {
	got, err := NormaliseSearchQuery(`subject:"Design Review"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "(") || !strings.Contains(got, ")") {
		t.Fatalf("output %q must hold the parenthesised form, not a quote-deleted value", got)
	}
	if got == `"subject:Design Review"` {
		t.Fatalf("output %q is the quote-deleted form that binds only the first token", got)
	}
}

// TestAlreadyEnclosedQueryPassesThrough pins behaviour 2: a query enclosed in
// one matched pair with no interior double quote is byte-identical after
// normalisation (measured row 2).
func TestAlreadyEnclosedQueryPassesThrough(t *testing.T) {
	in := `"subject:Contoso"`
	got, err := NormaliseSearchQuery(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != in {
		t.Fatalf("got %q, want byte-identical %q", got, in)
	}
}

// TestStrayQuoteIsRejected pins behaviour 4: a double quote that delimits no
// property value and forms no enclosing pair is refused, and the error names
// the parenthesised form as the correction rather than only diagnosing.
func TestStrayQuoteIsRejected(t *testing.T) {
	got, err := NormaliseSearchQuery(`subject:Design" Review`)
	if err == nil {
		t.Fatalf("expected an error, got output %q", got)
	}
	if got != "" {
		t.Fatalf("expected empty output on error, got %q", got)
	}
	if !strings.Contains(err.Error(), "parentheses") {
		t.Fatalf("error must name the parenthesised form as the correction, got: %v", err)
	}
	if !strings.Contains(err.Error(), "subject:(Design Review)") {
		t.Fatalf("error must carry a concrete parenthesised example, got: %v", err)
	}
}

// TestBooleanExpressionIsWrappedOnce pins that a compound expression gains a
// single enclosing pair, not one pair per clause.
func TestBooleanExpressionIsWrappedOnce(t *testing.T) {
	got, err := NormaliseSearchQuery(`subject:Sprint AND from:alice@contoso.com`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := `"subject:Sprint AND from:alice@contoso.com"`; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if strings.Count(got, `"`) != 2 {
		t.Fatalf("expected exactly one enclosing pair, got %d quotes in %q", strings.Count(got, `"`), got)
	}
}

// idempotenceCases collects every input exercised elsewhere, so idempotence is
// asserted across the whole corpus. Idempotence is the cheap invariant that
// catches a translation firing twice; it is checked for both accepted and
// rejected inputs.
func idempotenceCases() []string {
	cases := []string{
		`subject:Contoso`,
		`Zzzqqxx Wwwyyzz`,
		`subject:"Design Review"`,
		`"subject:Contoso"`,
		`subject:Design" Review`,
		`subject:Sprint AND from:alice@contoso.com`,
	}
	for _, row := range measuredRows {
		cases = append(cases, row.Value)
	}
	return cases
}

// TestNormalisationIsIdempotent asserts that normalising an already-normalised
// query returns it unchanged, across every fixture. For a rejected input, both
// runs must error; for an accepted input, both runs must produce identical
// output.
func TestNormalisationIsIdempotent(t *testing.T) {
	for _, in := range idempotenceCases() {
		in := in
		t.Run(in, func(t *testing.T) {
			first, err1 := NormaliseSearchQuery(in)
			if err1 != nil {
				// A rejected input must stay rejected on a second pass. There is
				// no normalised output to re-feed, so idempotence here means the
				// rejection is stable.
				if _, err2 := NormaliseSearchQuery(in); err2 == nil {
					t.Fatalf("input %q errored then succeeded: %v", in, err1)
				}
				return
			}
			second, err2 := NormaliseSearchQuery(first)
			if err2 != nil {
				t.Fatalf("normalising the normalised %q errored: %v", first, err2)
			}
			if second != first {
				t.Fatalf("not idempotent: %q -> %q -> %q", in, first, second)
			}
		})
	}
}
