package surface

import (
	"os"
	"testing"
)

// committedManifestPath is the committed manifest location relative to this
// package directory (internal/surface), two levels below the repository root.
const committedManifestPath = "../../site/src/generated/surface.json"

// TestCommittedManifestMatchesRecord asserts that the committed
// site/src/generated/surface.json is byte-identical to a freshly built and
// serialized Record. This is the drift check at the Go level: a change to the
// verb builders or the configuration inventory that is not accompanied by a
// regenerated manifest fails the build here rather than shipping a stale site
// (CR-0073 FR-22, NFR-1).
//
// The failure message names the make target that regenerates the manifest so a
// contributor who trips this check knows the fix without reading the source.
func TestCommittedManifestMatchesRecord(t *testing.T) {
	onDisk, err := os.ReadFile(committedManifestPath)
	if err != nil {
		t.Fatalf("failed to read committed manifest %s: %v (run 'make surface-manifest' to generate it)", committedManifestPath, err)
	}

	fresh, err := Serialize(BuildRecord())
	if err != nil {
		t.Fatalf("failed to serialize a fresh record: %v", err)
	}

	if string(onDisk) != string(fresh) {
		t.Errorf("committed manifest %s is stale; run 'make surface-manifest' and commit the result", committedManifestPath)
	}
}
