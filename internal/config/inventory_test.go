// Package config — tests for the declarative environment variable inventory.
//
// These tests are the guard that keeps the inventory the single authoritative
// list of the loader's environment surface (CR-0073 Phase 1). One asserts that
// no OUTLOOK_MCP_ literal can exist in the package without an inventory entry;
// the other asserts every entry is complete. Without the first, the inventory
// would be a second hand-maintained list that could silently drift from the
// loader, which is the exact defect the manifest exists to remove.
//
// @agents-index: tests enforcing inventory completeness and enumeration.
package config

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// envLiteralPattern matches any OUTLOOK_MCP_ environment variable name.
var envLiteralPattern = regexp.MustCompile(`OUTLOOK_MCP_[A-Z_]+`)

// TestEveryEnvLiteralIsEnumerated scans every non-test Go source file in the
// package for OUTLOOK_MCP_ literals and asserts that each one appears as an
// inventory entry name. A literal read (or even named in a comment) without an
// inventory entry fails the build, so a new variable cannot be introduced
// without enumerating it (FR-8, FR-9).
func TestEveryEnvLiteralIsEnumerated(t *testing.T) {
	enumerated := make(map[string]bool)
	for _, v := range Inventory() {
		enumerated[v.Name] = true
	}

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}

	found := make(map[string]bool)
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for _, lit := range envLiteralPattern.FindAllString(string(data), -1) {
			found[lit] = true
			if !enumerated[lit] {
				t.Errorf("%s references %q which is not in the inventory; add it to inventory.go", name, lit)
			}
		}
	}

	if len(found) == 0 {
		t.Fatal("no OUTLOOK_MCP_ literals found in package source; the scan is broken")
	}
}

// TestInventoryEntriesAreComplete asserts every inventory entry carries a
// well-formed name and a non-empty description, and that names are unique. The
// default may legitimately be empty (unset or derived), so it is not required
// to be non-empty (FR-8).
func TestInventoryEntriesAreComplete(t *testing.T) {
	seen := make(map[string]bool)
	for i, v := range Inventory() {
		if !strings.HasPrefix(v.Name, "OUTLOOK_MCP_") {
			t.Errorf("entry %d: name %q lacks the OUTLOOK_MCP_ prefix", i, v.Name)
		}
		if strings.TrimSpace(v.Description) == "" {
			t.Errorf("entry %d (%s): empty description", i, v.Name)
		}
		if seen[v.Name] {
			t.Errorf("entry %d: duplicate name %q", i, v.Name)
		}
		seen[v.Name] = true
	}
	if len(seen) == 0 {
		t.Fatal("inventory is empty")
	}
}
