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

// TestManifest_NewTools verifies that the CR-0060 Phase 3b+3c aggregate tools
// are present in the extension manifest, replacing the individual account_* and
// mail_* entries (AC-12).
func TestManifest_NewTools(t *testing.T) {
	data, err := os.ReadFile("manifest.json")
	if err != nil {
		t.Fatalf("read manifest.json: %v", err)
	}
	var m manifestDoc
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal manifest.json: %v", err)
	}

	// After CR-0060 Phase 3b+3c the individual account_* and mail_* tools are
	// replaced by single aggregate tools. Verify they are present.
	required := []string{"account", "mail"}
	// Verify no old individual account_* or mail_* names remain.
	deprecated := []string{
		"account_login", "account_logout", "account_refresh", "account_add", "account_remove", "account_list",
		"mail_list_folders", "mail_list_messages", "mail_get_message", "mail_search_messages",
		"mail_get_conversation", "mail_get_attachment", "mail_list_attachments",
		"mail_create_draft", "mail_create_reply_draft", "mail_create_forward_draft",
		"mail_update_draft", "mail_delete_draft",
	}

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
			t.Errorf("manifest.json tools[] should not contain deprecated entry %q (replaced by aggregate tool)", name)
		}
	}
}
