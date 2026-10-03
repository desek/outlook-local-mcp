// @agents-index Test that pins the release workflow: raw binary assets, the digest pin pull request, and the unchanged MCPB steps.
package plugin

import (
	"strings"
	"testing"
)

// mcpbStepsGolden is the MCPB packaging block of the release job, copied
// verbatim. The plugin release steps must leave it byte-identical, because the
// Claude Desktop extension ships from it.
const mcpbStepsGolden = `      - name: Prepare MCPB extension
        run: |
          mkdir -p extension/bin
          DARWIN_ARM64=$(find dist-parts -path "*darwin_arm64*/outlook-local-mcp" -type f)
          WINDOWS_AMD64=$(find dist-parts -path "*windows_amd64*/outlook-local-mcp.exe" -type f)
          cp "$DARWIN_ARM64" extension/bin/outlook-local-mcp-darwin-arm64
          cp "$WINDOWS_AMD64" extension/bin/outlook-local-mcp-win32-x64.exe
          chmod +x extension/bin/outlook-local-mcp-darwin-arm64
      - name: Inject version into manifest
        run: |
          VERSION="${{ needs.release-please.outputs.tag_name }}"
          VERSION="${VERSION#v}"
          jq --arg v "$VERSION" '.version = $v' extension/manifest.json > extension/manifest.tmp.json
          mv extension/manifest.tmp.json extension/manifest.json
      - name: Validate MCPB manifest
        run: mcpb validate extension/manifest.json
      - name: Pack MCPB bundle
        run: mcpb pack extension/ outlook-local-mcp.mcpb
`

// TestReleaseWorkflowPublishesRawBinariesAndPinsDigests pins the release
// coupling of the plugin: raw binaries beside the archives, a digest pin pull
// request after the upload, and an untouched MCPB block.
func TestReleaseWorkflowPublishesRawBinariesAndPinsDigests(t *testing.T) {
	wf := string(readRepoFile(t, ".github/workflows/release.yml"))
	archive := section(t, wf, "      - name: Create archives and checksums", "      - name: Prepare MCPB extension")
	for _, want := range []string{
		`cp "$dir/outlook-local-mcp" "$RELEASE_DIR/outlook-local-mcp-${os}-${arch}"`,
		`cp "$dir/outlook-local-mcp.exe" "$RELEASE_DIR/outlook-local-mcp-${os}-${arch}.exe"`,
	} {
		if !strings.Contains(archive, want) {
			t.Errorf("archive step does not copy the raw binary: %s", want)
		}
	}
	if strings.Index(archive, "cp \"$dir/outlook-local-mcp\"") > strings.Index(archive, "sha256sum") {
		t.Error("raw binaries are copied after checksums.txt is written")
	}
	if !strings.Contains(wf, mcpbStepsGolden) {
		t.Error("the MCPB steps of the release job changed")
	}
	upload := strings.Index(wf, "      - name: Upload release assets")
	if !strings.Contains(wf[upload:], "outlook-local-mcp.mcpb \\\n") {
		t.Error("Upload release assets no longer uploads outlook-local-mcp.mcpb")
	}
	pin := strings.Index(wf, "      - name: Pin release digests")
	if pin < upload {
		t.Fatal("no Pin release digests step after Upload release assets")
	}
	step := wf[pin:]
	if next := strings.Index(step[1:], "\n  container:"); next > 0 {
		step = step[:next+1]
	}
	for _, want := range []string{`--title "chore(plugin): pin release digests ${TAG}"`, "plugin/checksums.txt", "gh pr merge", "--auto --squash"} {
		if !strings.Contains(step, want) {
			t.Errorf("Pin release digests step lacks %q", want)
		}
	}
}

// section returns the text between two markers of the workflow.
func section(t *testing.T, s, from, to string) string {
	t.Helper()
	a, b := strings.Index(s, from), strings.Index(s, to)
	if a < 0 || b < a {
		t.Fatalf("workflow markers %q..%q not found in order", from, to)
	}
	return s[a:b]
}
