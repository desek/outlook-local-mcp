package buildinfo

import (
	"testing"
)

// TestIsHomebrew verifies the Homebrew path heuristic.
func TestIsHomebrew(t *testing.T) {
	t.Parallel()

	cases := []struct {
		path string
		want bool
	}{
		{"/opt/homebrew/Cellar/outlook-local-mcp/0.6.1/bin/outlook-local-mcp", true},
		{"/home/user/go/bin/outlook-local-mcp", false},
		{"/usr/local/homebrew/bin/outlook-local-mcp", true},
		{"/usr/local/bin/outlook-local-mcp", false},
	}
	for _, tc := range cases {
		got := isHomebrew(tc.path)
		if got != tc.want {
			t.Errorf("isHomebrew(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

// TestIsScoop verifies the Scoop path heuristic.
func TestIsScoop(t *testing.T) {
	t.Parallel()

	cases := []struct {
		path string
		want bool
	}{
		{`C:\Users\user\scoop\apps\outlook-local-mcp\current\outlook-local-mcp.exe`, true},
		{`C:\Program Files\outlook-local-mcp\outlook-local-mcp.exe`, false},
	}
	for _, tc := range cases {
		got := isScoop(tc.path)
		if got != tc.want {
			t.Errorf("isScoop(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

// TestIsGoInstallGOBIN verifies detection when GOBIN is set.
func TestIsGoInstallGOBIN(t *testing.T) {
	t.Setenv("GOBIN", "/custom/gobin")
	if !isGoInstall("/custom/gobin/outlook-local-mcp") {
		t.Error("isGoInstall() = false for GOBIN path, want true")
	}
	if isGoInstall("/other/path/outlook-local-mcp") {
		t.Error("isGoInstall() = true for non-GOBIN path, want false")
	}
}

// TestIsGoInstallGOPATH verifies detection when GOPATH is set.
func TestIsGoInstallGOPATH(t *testing.T) {
	t.Setenv("GOBIN", "")
	t.Setenv("GOPATH", "/home/user/go")
	if !isGoInstall("/home/user/go/bin/outlook-local-mcp") {
		t.Error("isGoInstall() = false for GOPATH/bin path, want true")
	}
}

// TestDistributionNoPanic verifies that Distribution never panics regardless
// of the executable path.
func TestDistributionNoPanic(t *testing.T) {
	t.Parallel()

	// Simply call it — if it panics, the test will fail.
	result := Distribution()
	validValues := map[string]bool{
		"homebrew":   true,
		"scoop":      true,
		"container":  true,
		"go-install": true,
		"binary":     true,
		"unknown":    true,
	}
	if !validValues[result] {
		t.Errorf("Distribution() = %q, not in allowed set", result)
	}
}
