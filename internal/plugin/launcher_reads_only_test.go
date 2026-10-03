// Package plugin tests: what files the launcher references.
//
// @agents-index: Test pinning that the launcher references only VERSION and
// checksums.txt inside the plugin, so the directory scanner never follows it to
// plugin.json and the icon it names.
package plugin

import (
	"strings"
	"testing"
)

// TestLauncherReferencesOnlyVersionAndChecksums pins the file set the launcher
// names under CLAUDE_PLUGIN_ROOT. Naming plugin.json would pull its icon
// reference into the directory's scan of the launcher and hold the version.
func TestLauncherReferencesOnlyVersionAndChecksums(t *testing.T) {
	src := string(readRepoFile(t, "plugin/scripts/outlook-local-mcp.sh"))
	if strings.Contains(src, "plugin.json") {
		t.Error("launcher references plugin.json; read the version from VERSION instead")
	}
	for _, want := range []string{`"$ROOT/VERSION"`, `"$ROOT/checksums.txt"`} {
		if !strings.Contains(src, want) {
			t.Errorf("launcher does not reference %s", want)
		}
	}
}
