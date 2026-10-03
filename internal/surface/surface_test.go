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

// TestContactsDomainRecordedGatedAndFull asserts that the record carries the
// contacts domain with all five of its verbs and none of them in the default
// set, each attributed to the contacts gate.
//
// The domain is gated whole rather than verb by verb, which is a shape no other
// domain has: every one of its verbs must carry the gate, and its default count
// must be zero rather than merely lower.
//
// The domain is selected by name rather than by position. Position is not the
// property under test, and a later gated domain appended after this one would
// otherwise fail this check while nothing about contacts had changed.
func TestContactsDomainRecordedGatedAndFull(t *testing.T) {
	rec := BuildRecord()

	if len(rec.Domains) == 0 {
		t.Fatal("record contains no domains")
	}

	var contacts *Domain
	for i := range rec.Domains {
		if rec.Domains[i].Name == "contacts" {
			contacts = &rec.Domains[i]
			break
		}
	}
	if contacts == nil {
		t.Fatal("the record carries no contacts domain; domainOrder omits it")
	}

	if contacts.FullCount != 5 {
		t.Errorf("contacts FullCount = %d, want 5", contacts.FullCount)
	}
	if contacts.DefaultCount != 0 {
		t.Errorf("contacts DefaultCount = %d, want 0; the whole domain is gated", contacts.DefaultCount)
	}

	for _, v := range contacts.Verbs {
		if v.Gate == nil {
			t.Errorf("contacts verb %q carries no gate although the domain is gated whole", v.Name)
			continue
		}
		if *v.Gate != config.EnvContactsEnabled {
			t.Errorf("contacts verb %q gate = %q, want %q", v.Name, *v.Gate, config.EnvContactsEnabled)
		}
	}
}

// TestTeamsDomainRecordedGatedAndFull asserts that the record carries the teams
// domain with all twelve of its verbs and none of them in the default set,
// each attributed to the teams gate, and that the gate variable itself is
// enumerated in the record's configuration inventory.
//
// The website derives every figure it states about the tool surface from this
// record and states no number of its own, so the domain's counts and its gate
// attribution are the published contract rather than an internal detail. The
// configuration clause is asserted here too: a verb attributed to a variable the
// inventory does not enumerate would render as a gate a reader cannot look up.
//
// The domain is selected by name rather than by position, so a later gated
// domain appended after this one does not fail this check while nothing about
// teams has changed.
func TestTeamsDomainRecordedGatedAndFull(t *testing.T) {
	rec := BuildRecord()

	var teams *Domain
	for i := range rec.Domains {
		if rec.Domains[i].Name == "teams" {
			teams = &rec.Domains[i]
			break
		}
	}
	if teams == nil {
		t.Fatal("the record carries no teams domain; domainOrder omits it")
	}

	if teams.FullCount != 12 {
		t.Errorf("teams FullCount = %d, want 12", teams.FullCount)
	}
	if teams.DefaultCount != 0 {
		t.Errorf("teams DefaultCount = %d, want 0; the whole domain is gated", teams.DefaultCount)
	}
	if len(teams.Verbs) != 12 {
		t.Errorf("teams records %d verbs, want 12", len(teams.Verbs))
	}

	for _, v := range teams.Verbs {
		if v.Gate == nil {
			t.Errorf("teams verb %q carries no gate although the domain is gated whole", v.Name)
			continue
		}
		if *v.Gate != config.EnvTeamsEnabled {
			t.Errorf("teams verb %q gate = %q, want %q", v.Name, *v.Gate, config.EnvTeamsEnabled)
		}
	}

	var enumerated bool
	for _, c := range rec.Config {
		if c.Name == config.EnvTeamsEnabled {
			enumerated = true
			break
		}
	}
	if !enumerated {
		t.Errorf("the record's config inventory does not enumerate %s, so every teams verb is attributed to a gate a reader cannot look up", config.EnvTeamsEnabled)
	}
}
