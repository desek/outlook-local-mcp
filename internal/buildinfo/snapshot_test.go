package buildinfo

import (
	"os"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"strings"
	"testing"
)

// findMiseToml walks upward from the current working directory looking for a
// ".mise.toml" file, returning its path and true when found. It stops at the
// filesystem root. It is used so the toolchain-pin test can locate the repo
// root's pin regardless of the package directory the test runs from.
func findMiseToml() (string, bool) {
	dir, err := os.Getwd()
	if err != nil {
		return "", false
	}
	for {
		candidate := filepath.Join(dir, ".mise.toml")
		if _, statErr := os.Stat(candidate); statErr == nil {
			return candidate, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// parseMiseGoPin extracts the pinned Go version (e.g. "1.25.12") from the
// go = "..." line of a .mise.toml file's contents. It returns the version and
// true on success, or the empty string and false when no go pin is present.
func parseMiseGoPin(contents string) (string, bool) {
	for _, line := range strings.Split(contents, "\n") {
		line = strings.TrimSpace(line)
		rest, ok := strings.CutPrefix(line, "go")
		if !ok {
			continue
		}
		rest = strings.TrimSpace(rest)
		rest, ok = strings.CutPrefix(rest, "=")
		if !ok {
			continue
		}
		rest = strings.TrimSpace(rest)
		rest = strings.Trim(rest, `"`)
		if rest != "" {
			return rest, true
		}
	}
	return "", false
}

// parseGoVersionTriple parses a dotted Go version like "1.25.12" or the
// "go1.25.12" reported by runtime.Version() into a comparable [3]int of
// major, minor, patch. A missing component is treated as zero, and any
// trailing pre-release or devel suffix is ignored.
func parseGoVersionTriple(v string) [3]int {
	v = strings.TrimPrefix(v, "go")
	var out [3]int
	parts := strings.SplitN(v, ".", 3)
	for i := 0; i < len(parts) && i < 3; i++ {
		// Stop at the first non-numeric run so suffixes like "rc1" or
		// "-devel" do not break parsing.
		numeric := parts[i]
		for j, r := range parts[i] {
			if r < '0' || r > '9' {
				numeric = parts[i][:j]
				break
			}
		}
		n, err := strconv.Atoi(numeric)
		if err != nil {
			break
		}
		out[i] = n
	}
	return out
}

// TestGoVersionMatchesToolchainPin asserts the built toolchain is not older than
// the Go version .mise.toml pins (CR-0071 AC-5). A toolchain bump applied to some
// files and not others then fails "make test" here rather than surfacing later
// as a govulncheck finding. The test SKIPS when .mise.toml is absent so a
// module-cache extraction (which carries no repo files) does not fail.
func TestGoVersionMatchesToolchainPin(t *testing.T) {
	path, ok := findMiseToml()
	if !ok {
		t.Skip(".mise.toml not found; skipping toolchain-pin check")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("cannot read %s: %v; skipping toolchain-pin check", path, err)
	}

	pinned, ok := parseMiseGoPin(string(data))
	if !ok {
		t.Skipf("%s declares no go pin; skipping toolchain-pin check", path)
	}

	built := goruntime.Version()
	pinnedTriple := parseGoVersionTriple(pinned)
	builtTriple := parseGoVersionTriple(built)

	if builtTriple[0] < pinnedTriple[0] ||
		(builtTriple[0] == pinnedTriple[0] && builtTriple[1] < pinnedTriple[1]) ||
		(builtTriple[0] == pinnedTriple[0] && builtTriple[1] == pinnedTriple[1] && builtTriple[2] < pinnedTriple[2]) {
		t.Errorf("built Go toolchain %q is older than the .mise.toml pin %q; raise the toolchain to build (see %s)",
			built, pinned, path)
	}
}

// TestSnapshotAllFieldsPopulated verifies that Snapshot returns a fully
// populated Info struct with no empty required fields.
func TestSnapshotAllFieldsPopulated(t *testing.T) {
	t.Parallel()

	info := Snapshot("v0.6.1", "abc1234", "2026-04-27T00:00:00Z", "keychain")

	if info.Version != "v0.6.1" {
		t.Errorf("Version = %q, want %q", info.Version, "v0.6.1")
	}
	if info.Commit != "abc1234" {
		t.Errorf("Commit = %q, want %q", info.Commit, "abc1234")
	}
	if info.BuildDate != "2026-04-27T00:00:00Z" {
		t.Errorf("BuildDate = %q, want %q", info.BuildDate, "2026-04-27T00:00:00Z")
	}
	if info.GoVersion != goruntime.Version() {
		t.Errorf("GoVersion = %q, want %q", info.GoVersion, goruntime.Version())
	}
	if info.OS != goruntime.GOOS {
		t.Errorf("OS = %q, want %q", info.OS, goruntime.GOOS)
	}
	if info.Arch != goruntime.GOARCH {
		t.Errorf("Arch = %q, want %q", info.Arch, goruntime.GOARCH)
	}
	if info.Runtime == "" {
		t.Error("Runtime is empty")
	}
	if info.Distribution == "" {
		t.Error("Distribution is empty")
	}
	if info.AuthBackend != "keychain" {
		t.Errorf("AuthBackend = %q, want %q", info.AuthBackend, "keychain")
	}
	if info.Homepage != ProjectHomepage {
		t.Errorf("Homepage = %q, want %q", info.Homepage, ProjectHomepage)
	}
	if info.IssueTracker != ProjectIssueTracker {
		t.Errorf("IssueTracker = %q, want %q", info.IssueTracker, ProjectIssueTracker)
	}
	if info.DocsBase != ProjectDocsBase {
		t.Errorf("DocsBase = %q, want %q", info.DocsBase, ProjectDocsBase)
	}
}

// TestSnapshotAuthBackendFile verifies that the authBackend field reflects the
// "file" value when passed by the caller.
func TestSnapshotAuthBackendFile(t *testing.T) {
	t.Parallel()

	info := Snapshot("dev", "unknown", "unknown", "file")
	if info.AuthBackend != "file" {
		t.Errorf("AuthBackend = %q, want %q", info.AuthBackend, "file")
	}
}

// TestSnapshotDefaultLdflagsAbsent verifies the expected default values when
// ldflags are not injected (the "go run" scenario).
func TestSnapshotDefaultLdflagsAbsent(t *testing.T) {
	t.Parallel()

	// When callers pass the default values, Snapshot should attempt the
	// ReadBuildInfo fallback. In a test binary the VCS data may or may not
	// be present, so we only assert the function does not panic and that
	// the struct is returned.
	info := Snapshot("dev", "unknown", "unknown", "file")
	if info.Version != "dev" {
		t.Errorf("Version = %q, want %q", info.Version, "dev")
	}
}

// TestSnapshotLdflagsPrecedenceOverFallback verifies that explicit ldflags
// values are not overwritten by the ReadBuildInfo fallback.
func TestSnapshotLdflagsPrecedenceOverFallback(t *testing.T) {
	t.Parallel()

	info := Snapshot("v1.0.0", "deadbee", "2026-01-01T00:00:00Z", "keychain")
	if info.Commit != "deadbee" {
		t.Errorf("Commit = %q, want %q (ldflags value should not be overwritten)", info.Commit, "deadbee")
	}
	if info.BuildDate != "2026-01-01T00:00:00Z" {
		t.Errorf("BuildDate = %q, want %q (ldflags value should not be overwritten)", info.BuildDate, "2026-01-01T00:00:00Z")
	}
}

// TestRuntimeClassValues verifies that RuntimeClass returns a value from the
// allowed set and never panics.
func TestRuntimeClassValues(t *testing.T) {
	t.Parallel()

	allowed := map[string]bool{
		"macos":     true,
		"linux":     true,
		"windows":   true,
		"container": true,
		"unknown":   true,
	}
	result := RuntimeClass()
	if !allowed[result] {
		t.Errorf("RuntimeClass() = %q, not in allowed set", result)
	}
}
