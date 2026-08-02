// Package surface — tests for the surface Record construction (CR-0073 Phase 1).
//
// These tests assert the counts are derived from the built verb slices rather
// than stated as literals, that the default count excludes gated verbs each
// naming its gate, and that every verb carries a summary and an explicit gate
// value.
//
// @agents-index: tests for surface Record counts, gates, and completeness.
package surface

import (
	"testing"

	"github.com/desek/outlook-local-mcp/internal/config"
	"github.com/desek/outlook-local-mcp/internal/server"
)

// inventoryNames returns the set of configuration variable names, used to
// validate that every gate references a real, enumerated variable.
func inventoryNames() map[string]bool {
	s := make(map[string]bool)
	for _, v := range config.Inventory() {
		s[v.Name] = true
	}
	return s
}

// TestRecordCountsMatchBuiltVerbs asserts that the record's per-domain and total
// full counts equal the lengths of the verb slices built with every gate open,
// so the counts are counted rather than transcribed (FR-3).
func TestRecordCountsMatchBuiltVerbs(t *testing.T) {
	rec := BuildRecord()
	built := server.BuildVerbsForInspection(fullConfig())

	total := 0
	for _, d := range rec.Domains {
		want := len(built[d.Name])
		if d.FullCount != want {
			t.Errorf("domain %q FullCount = %d, want %d (built slice length)", d.Name, d.FullCount, want)
		}
		if len(d.Verbs) != want {
			t.Errorf("domain %q has %d verb entries, want %d", d.Name, len(d.Verbs), want)
		}
		total += want
	}
	if rec.Totals.FullCount != total {
		t.Errorf("Totals.FullCount = %d, want %d", rec.Totals.FullCount, total)
	}
}

// TestDefaultCountExcludesGatedVerbs asserts that the default configuration
// exposes strictly fewer verbs than the full surface, and that every verb the
// default omits carries a gate naming the configuration key that gates it
// (FR-2, FR-3).
func TestDefaultCountExcludesGatedVerbs(t *testing.T) {
	rec := BuildRecord()
	names := inventoryNames()

	if rec.Totals.DefaultCount >= rec.Totals.FullCount {
		t.Fatalf("DefaultCount %d not below FullCount %d; gating not reflected",
			rec.Totals.DefaultCount, rec.Totals.FullCount)
	}

	gatedTotal := 0
	for _, d := range rec.Domains {
		gatedInDomain := 0
		for _, v := range d.Verbs {
			if v.Gate == nil {
				continue
			}
			gatedInDomain++
			if !names[*v.Gate] {
				t.Errorf("domain %q verb %q gate %q is not an enumerated config variable",
					d.Name, v.Name, *v.Gate)
			}
		}
		// The number of gated verbs must equal the full-minus-default gap.
		if got, want := gatedInDomain, d.FullCount-d.DefaultCount; got != want {
			t.Errorf("domain %q has %d gated verbs, but FullCount-DefaultCount = %d",
				d.Name, got, want)
		}
		gatedTotal += gatedInDomain
	}

	if gatedTotal == 0 {
		t.Fatal("no gated verbs found; the gating derivation is broken")
	}
}

// TestEveryVerbCarriesSummaryAndGate asserts that no verb enters the record
// without a non-empty summary, and that its gate field is explicit: either nil
// (ungated) or a valid enumerated configuration variable name (FR-2).
func TestEveryVerbCarriesSummaryAndGate(t *testing.T) {
	rec := BuildRecord()
	names := inventoryNames()

	seen := 0
	for _, d := range rec.Domains {
		for _, v := range d.Verbs {
			seen++
			if v.Summary == "" {
				t.Errorf("domain %q verb %q has an empty summary", d.Name, v.Name)
			}
			if v.Gate != nil && !names[*v.Gate] {
				t.Errorf("domain %q verb %q gate %q is not enumerated", d.Name, v.Name, *v.Gate)
			}
		}
	}
	if seen == 0 {
		t.Fatal("record contains no verbs")
	}
}
