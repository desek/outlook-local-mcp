// @agents-index Shared test helpers: locate the repository root and read plugin and release files.
package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// repoRoot returns the repository root, two levels above this package, so
// the tests read the committed files regardless of the working directory.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed; cannot locate the repository root")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

// readRepoFile returns the contents of a file named relative to the repository root.
func readRepoFile(t *testing.T, rel string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return b
}

// readJSON decodes a repository file into a generic map.
func readJSON(t *testing.T, rel string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(readRepoFile(t, rel), &m); err != nil {
		t.Fatalf("decode %s: %v", rel, err)
	}
	return m
}
