package buildinfo

import (
	"os"
	"path/filepath"
	"strings"
)

// Distribution returns a best-effort hint about how the binary was installed.
//
// The returned value is one of:
//   - "homebrew"  – executable path contains "/Cellar/" or "/homebrew/".
//   - "scoop"     – executable path contains "\\scoop\\" (Windows only).
//   - "container" – IsContainer() is true.
//   - "go-install" – executable path is under $GOBIN or $GOPATH/bin.
//   - "binary"    – none of the above (downloaded archive or manual install).
//   - "unknown"   – os.Executable() returned an error or other failure.
//
// This function never panics. Any filesystem or environment access error
// degrades to "unknown".
func Distribution() string {
	if IsContainer() {
		return "container"
	}

	exe, err := os.Executable()
	if err != nil {
		return "unknown"
	}

	// Resolve symlinks so that Homebrew cellar paths surface correctly.
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	if isHomebrew(exe) {
		return "homebrew"
	}
	if isScoop(exe) {
		return "scoop"
	}
	if isGoInstall(exe) {
		return "go-install"
	}
	return "binary"
}

// isHomebrew reports whether exe is under a Homebrew cellar or prefix.
func isHomebrew(exe string) bool {
	lower := strings.ToLower(filepath.ToSlash(exe))
	return strings.Contains(lower, "/cellar/") || strings.Contains(lower, "/homebrew/")
}

// isScoop reports whether exe is under a Scoop bucket (Windows).
func isScoop(exe string) bool {
	return strings.Contains(strings.ToLower(exe), "\\scoop\\")
}

// isGoInstall reports whether exe is under $GOBIN or $GOPATH/bin.
func isGoInstall(exe string) bool {
	gobin := os.Getenv("GOBIN")
	if gobin != "" && strings.HasPrefix(exe, gobin) {
		return true
	}
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		// Default GOPATH is ~/go.
		if home, err := os.UserHomeDir(); err == nil {
			gopath = filepath.Join(home, "go")
		}
	}
	if gopath != "" && strings.HasPrefix(exe, filepath.Join(gopath, "bin")) {
		return true
	}
	return false
}
