// Package extension_test contains tests that validate the MCPB extension
// manifest shipped to Claude Desktop. The manifest is the source of truth
// Claude Desktop uses for tool discovery, so every tool registered in
// internal/server must also appear here.
package extension_test

import (
	"encoding/json"
	"os"
	"testing"
)

// manifestTool mirrors the minimal subset of fields required to verify tool
// registration in the manifest. Additional manifest fields are ignored by
// the JSON decoder.
type manifestTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// manifestDoc models the top-level manifest structure. Only the tools array
// is asserted on here.
type manifestDoc struct {
	Tools []manifestTool `json:"tools"`
}

// TestManifest_NewTools verifies that the CR-0060 Phase 3b account aggregate
// tool is present in the extension manifest, replacing the individual
// account_login, account_logout, and account_refresh entries (AC-12).
func TestManifest_NewTools(t *testing.T) {
	data, err := os.ReadFile("manifest.json")
	if err != nil {
		t.Fatalf("read manifest.json: %v", err)
	}
	var m manifestDoc
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal manifest.json: %v", err)
	}

	// After CR-0060 Phase 3b the individual account_* tools are replaced by the
	// single "account" aggregate tool. Verify it is present.
	required := []string{"account"}
	// Verify no old individual account_* names remain.
	deprecated := []string{"account_login", "account_logout", "account_refresh", "account_add", "account_remove", "account_list"}

	present := make(map[string]bool, len(m.Tools))
	for _, tool := range m.Tools {
		present[tool.Name] = true
	}
	for _, name := range required {
		if !present[name] {
			t.Errorf("manifest.json tools[] missing required entry %q", name)
		}
	}
	for _, name := range deprecated {
		if present[name] {
			t.Errorf("manifest.json tools[] should not contain deprecated entry %q (replaced by 'account' aggregate)", name)
		}
	}
}
