// Package server — this file holds the derived check that keeps
// extension/manifest.json in step with the verb registry.
//
// The manifest is what Claude Desktop reads to describe the aggregate tools,
// and nothing else in the build reads it, so a verb added to a domain
// registry without a matching manifest edit ships an extension that describes a
// surface that no longer exists. The cases here are derived from the registry
// under the maximal configuration rather than listed, so a future verb is
// covered without anyone remembering to add it.
//
// @agents-index: derived check that the extension manifest's per-domain
// descriptions name every verb the registry registers for that domain.
package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/desek/outlook-local-mcp/internal/config"
)

// manifestTool is the subset of an extension/manifest.json tools entry this
// check reads: the domain name and the prose description that must enumerate
// the domain's verbs.
type manifestTool struct {
	// Name is the aggregate tool name, matching a domain key of the registry.
	Name string `json:"name"`

	// Description is the published prose, which must name every registered verb.
	Description string `json:"description"`
}

// manifestDocument is the subset of extension/manifest.json this check reads.
type manifestDocument struct {
	// Tools is the published aggregate tool list, which must stay at six
	// entries because the MCP surface is four aggregate domain tools by
	// default plus the opt-in contacts and teams domains.
	Tools []manifestTool `json:"tools"`
}

// readExtensionManifest loads extension/manifest.json relative to this package,
// failing the test rather than returning an error, since a manifest that cannot
// be read is a build problem and not a condition callers recover from.
func readExtensionManifest(t *testing.T) manifestDocument {
	t.Helper()

	path := filepath.Join("..", "..", "extension", "manifest.json")
	raw, err := os.ReadFile(path) //nolint:gosec // fixed repository-relative test fixture path
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	var doc manifestDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return doc
}

// maximalSurfaceConfig returns the configuration in which every optional gate is
// open, so the registry-derived cases cover the complete verb surface rather
// than the default subset. It mirrors internal/surface's fullConfig; the small
// duplication keeps this test from importing the surface package for a struct
// literal.
func maximalSurfaceConfig() config.Config {
	return config.Config{
		MailEnabled:       true,
		MailManageEnabled: true,
		ContactsEnabled:   true,
		TeamsEnabled:      true,
		AuthMethod:        "auth_code",
	}
}

// TestManifestDescribesEveryRegisteredVerb asserts that every verb the registry
// registers for a domain under the maximal configuration is named in that
// domain's extension/manifest.json description, and that the tools array holds
// exactly the four default aggregate domain tools plus the opt-in contacts and
// teams domains.
//
// The verb name is matched on word boundaries so a shorter name cannot be
// satisfied by a longer one that contains it: Go's \b treats the underscore as a
// word character, so `list` does not match inside `list_docs`, which is the
// collision this domain actually has.
func TestManifestDescribesEveryRegisteredVerb(t *testing.T) {
	doc := readExtensionManifest(t)

	if len(doc.Tools) != 6 {
		t.Fatalf("extension/manifest.json declares %d tools, want exactly 6 aggregate domain tools", len(doc.Tools))
	}

	described := make(map[string]string, len(doc.Tools))
	for _, tool := range doc.Tools {
		described[tool.Name] = tool.Description
	}

	sets := BuildVerbsForInspection(maximalSurfaceConfig())
	if len(sets) == 0 {
		t.Fatal("registry built no domains under the maximal configuration")
	}

	checked := 0
	for domain, verbs := range sets {
		desc, ok := described[domain]
		if !ok {
			t.Errorf("domain %q is registered but has no entry in extension/manifest.json", domain)
			continue
		}
		for _, v := range verbs {
			// help is rendered by every domain and is named in every
			// description already; it is checked like any other verb.
			named := regexp.MustCompile(`\b` + regexp.QuoteMeta(v.Name) + `\b`)
			if !named.MatchString(desc) {
				t.Errorf("verb %s.%s is registered but not named in the extension manifest description for %q; add it there in the same change", domain, v.Name, domain)
			}
			checked++
		}
	}

	if checked == 0 {
		t.Fatal("no verbs were checked; the registry-derived case set is empty, which would make this check vacuous")
	}
	t.Logf("checked %d registered verbs against %d manifest descriptions", checked, len(described))
}
