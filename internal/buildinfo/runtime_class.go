package buildinfo

import goruntime "runtime"

// RuntimeClass returns a friendly classification of the execution environment.
//
// Container detection takes precedence. When the process is inside a
// container, "container" is returned regardless of GOOS. Otherwise GOOS is
// mapped to a friendly name:
//   - "darwin"  → "macos"
//   - "linux"   → "linux"
//   - "windows" → "windows"
//   - other     → "unknown"
func RuntimeClass() string {
	if IsContainer() {
		return "container"
	}
	switch goruntime.GOOS {
	case "darwin":
		return "macos"
	case "linux":
		return "linux"
	case "windows":
		return "windows"
	default:
		return "unknown"
	}
}
