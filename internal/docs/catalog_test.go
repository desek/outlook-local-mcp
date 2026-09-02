package docs_test

import (
	"strings"
	"testing"

	"github.com/desek/outlook-local-mcp/internal/docs"
)

// TestCatalog_AllSlugsResolve verifies that every entry in the catalog maps to
// a non-empty file in the embedded bundle. A failure here indicates that the
// catalog metadata and the embed directive are out of sync.
func TestCatalog_AllSlugsResolve(t *testing.T) {
	t.Parallel()

	catalog, err := docs.Catalog()
	if err != nil {
		t.Fatalf("Catalog() error: %v", err)
	}
	if len(catalog) == 0 {
		t.Fatal("Catalog() returned empty slice")
	}

	for _, entry := range catalog {
		t.Run(entry.Slug, func(t *testing.T) {
			t.Parallel()

			data, err := docs.ReadSlug(entry.Slug)
			if err != nil {
				t.Fatalf("ReadSlug(%q) error: %v", entry.Slug, err)
			}
			if len(data) == 0 {
				t.Fatalf("ReadSlug(%q) returned empty content", entry.Slug)
			}
			if entry.Size <= 0 {
				t.Fatalf("catalog entry %q has non-positive Size: %d", entry.Slug, entry.Size)
			}
			if entry.Title == "" {
				t.Fatalf("catalog entry %q has empty Title", entry.Slug)
			}
			if entry.Summary == "" {
				t.Fatalf("catalog entry %q has empty Summary", entry.Slug)
			}
		})
	}
}

// TestMailGatingRowNamesMessageManagement asserts that the embedded concepts
// document's MAIL_MANAGE_ENABLED row stops reading as exhaustive over drafts.
//
// The row previously read "including draft management", which an LLM reading the
// embedded bundle mid-session would take as the complete list of what the gate
// unlocks, so the four received-message write verbs would look unavailable even
// when registered. The assertion reads the embedded bundle rather than the file
// on disk, because the bundle is what a running server serves.
func TestMailGatingRowNamesMessageManagement(t *testing.T) {
	t.Parallel()

	row := mailGatingRow(t)

	if !strings.Contains(row, "draft management") {
		t.Errorf("MAIL_MANAGE_ENABLED row no longer names draft management: %q", row)
	}
	if !strings.Contains(row, "received-message management") {
		t.Errorf("MAIL_MANAGE_ENABLED row names only draft management, so it reads as exhaustive; it must also name received-message management: %q", row)
	}
	for _, verb := range []string{"move_message", "set_flag", "set_categories", "mark_read"} {
		if !strings.Contains(row, verb) {
			t.Errorf("MAIL_MANAGE_ENABLED row does not name the gated verb %s: %q", verb, row)
		}
	}
}

// TestMailGatingRowNamesDraftAttachments asserts that the embedded gating row
// also names the attachment capability the gate unlocks.
//
// A row naming draft management and received-message management reads as a
// complete account of the gate, so an LLM serving itself from the embedded
// bundle would conclude a draft cannot carry a file and refuse the request
// rather than issuing the verb that exists.
func TestMailGatingRowNamesDraftAttachments(t *testing.T) {
	t.Parallel()

	row := mailGatingRow(t)

	if !strings.Contains(row, "add_attachment") {
		t.Errorf("MAIL_MANAGE_ENABLED row does not name the gated verb add_attachment: %q", row)
	}
	if !strings.Contains(row, "draft attachments") {
		t.Errorf("MAIL_MANAGE_ENABLED row does not name draft attachments as a capability of the gate: %q", row)
	}
}

// TestTeamsScopeRowsNameEveryRequestedScope asserts that the embedded concepts
// document's canonical per-feature scopes table carries both polarities of the
// Teams gate, and that the enabled row names all four scopes the gate causes to
// be requested.
//
// The gating section elsewhere in the document states the same scopes, but a
// reader answering "what will this consent prompt ask for" consults the
// per-feature table, and a gate absent from that table reads as a gate that
// changes no consent. The assertion reads the embedded bundle rather than the
// file on disk, because the bundle is what a running server serves.
func TestTeamsScopeRowsNameEveryRequestedScope(t *testing.T) {
	t.Parallel()

	disabled, enabled := teamsScopeRows(t)

	if !strings.Contains(disabled, "*(none)*") {
		t.Errorf("the disabled Teams row does not state that no scope is requested: %q", disabled)
	}
	for _, scope := range []string{
		"Chat.Read",
		"ChannelMessage.Read.All",
		"OnlineMeetings.Read",
		"OnlineMeetingTranscript.Read.All",
	} {
		if !strings.Contains(enabled, scope) {
			t.Errorf("the enabled Teams row does not name the requested scope %s: %q", scope, enabled)
		}
	}
}

// teamsScopeRows returns the disabled-polarity and enabled-polarity
// OUTLOOK_MCP_TEAMS_ENABLED rows of the embedded concepts document's
// per-feature scopes table, failing the test when either is absent.
//
// The rows are told apart by the "=false" marker rather than by their order, so
// a later reordering of the table does not silently swap the two assertions.
func teamsScopeRows(t *testing.T) (disabled, enabled string) {
	t.Helper()

	data, err := docs.ReadSlug("concepts")
	if err != nil {
		t.Fatalf("ReadSlug(\"concepts\") error: %v", err)
	}

	for _, line := range strings.Split(string(data), "\n") {
		// The "=" form is what the per-feature scopes table uses. The gating
		// section states the same variable in a Variable/Value column pair
		// instead, so keying on it selects the canonical table alone.
		if !strings.HasPrefix(line, "|") || !strings.Contains(line, "OUTLOOK_MCP_TEAMS_ENABLED=") {
			continue
		}
		if strings.Contains(line, "OUTLOOK_MCP_TEAMS_ENABLED=false") {
			disabled = line
			continue
		}
		enabled = line
	}

	if disabled == "" {
		t.Fatal("the embedded concepts scopes table carries no OUTLOOK_MCP_TEAMS_ENABLED=false row, so a reader cannot see that the default gate requests nothing")
	}
	if enabled == "" {
		t.Fatal("the embedded concepts scopes table carries no OUTLOOK_MCP_TEAMS_ENABLED=true row, so the four scopes the gate requests are absent from the canonical table")
	}
	return disabled, enabled
}

// mailGatingRow returns the MAIL_MANAGE_ENABLED row of the embedded concepts
// document, failing the test when the row is absent.
//
// It reads the embedded bundle rather than the file on disk, because the bundle
// is what a running server serves to an LLM mid-session.
func mailGatingRow(t *testing.T) string {
	t.Helper()

	data, err := docs.ReadSlug("concepts")
	if err != nil {
		t.Fatalf("ReadSlug(\"concepts\") error: %v", err)
	}

	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, "`MAIL_MANAGE_ENABLED`") {
			return line
		}
	}
	t.Fatal("no MAIL_MANAGE_ENABLED row found in the embedded concepts document")
	return ""
}
