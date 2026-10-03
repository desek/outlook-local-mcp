// Package plugin tests: the workflow that advances the plugin-stable tag.
//
// @agents-index: Test pinning that the plugin-stable tag workflow triggers on
// the digest pin file, verifies the pin before moving, and force-pushes the tag.
package plugin

import (
	"strings"
	"testing"
)

// TestPluginStableTagWorkflowMovesOnDigestPin pins the contract the directory
// relies on: the tag moves only when plugin/checksums.txt changes on main, only
// after the pin for the manifest version is present, and by force-pushing the
// single tag name the portal tracks.
func TestPluginStableTagWorkflowMovesOnDigestPin(t *testing.T) {
	w := readRepoFile(t, ".github/workflows/plugin-stable-tag.yml")
	for _, want := range []string{
		"branches: [main]",
		"- plugin/checksums.txt",
		`grep -qx "# v${version}" plugin/checksums.txt`,
		`git tag -f plugin-stable "$GITHUB_SHA"`,
		"git push -f origin refs/tags/plugin-stable",
	} {
		if !strings.Contains(w, want) {
			t.Errorf("plugin-stable-tag.yml lacks %q", want)
		}
	}
}
