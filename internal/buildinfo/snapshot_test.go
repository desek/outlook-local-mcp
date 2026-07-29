package buildinfo

import (
	goruntime "runtime"
	"testing"
)

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
