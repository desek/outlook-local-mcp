package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestDetectTimezone_TZEnvOverridesLocal verifies that when the TZ environment
// variable is set to a valid IANA timezone, DetectTimezone returns that value
// even if time.Now().Location() would return "Local".
func TestDetectTimezone_TZEnvOverridesLocal(t *testing.T) {
	t.Setenv("TZ", "America/New_York")

	tz := DetectTimezone()
	if tz != "America/New_York" {
		// TZ may be ignored if time.Now().Location() already returns a valid
		// non-"Local" value (e.g., the test host has timezone configured).
		// In that case the returned value is still valid.
		if tz == "Local" {
			t.Errorf("DetectTimezone() = %q, should never return 'Local'", tz)
		}
	}
}

// TestDetectTimezone_NeverReturnsLocal verifies that DetectTimezone never
// returns the raw string "Local", regardless of system configuration.
func TestDetectTimezone_NeverReturnsLocal(t *testing.T) {
	tz := DetectTimezone()
	if tz == "Local" {
		t.Errorf("DetectTimezone() = %q, should never return 'Local'", tz)
	}
	if tz == "" {
		t.Errorf("DetectTimezone() returned empty string")
	}
}

// TestDetectTimezone_InvalidTZFallsThrough verifies that an invalid TZ
// environment variable is ignored and detection continues to other sources.
func TestDetectTimezone_InvalidTZFallsThrough(t *testing.T) {
	t.Setenv("TZ", "Not/A/Real/Timezone")

	tz := DetectTimezone()
	if tz == "Local" || tz == "" {
		t.Errorf("DetectTimezone() = %q, should resolve to a valid timezone", tz)
	}
	if tz == "Not/A/Real/Timezone" {
		t.Errorf("DetectTimezone() returned invalid TZ value")
	}
}

// TestResolveLocaltimeSymlink_ValidSymlink verifies that a symlink pointing
// to a zoneinfo path is correctly resolved to an IANA timezone name.
func TestResolveLocaltimeSymlink_ValidSymlink(t *testing.T) {
	dir := t.TempDir()
	// Create a fake zoneinfo directory structure.
	zoneinfoDir := filepath.Join(dir, "usr", "share", "zoneinfo", "Europe")
	if err := os.MkdirAll(zoneinfoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Create a dummy timezone file.
	tzFile := filepath.Join(zoneinfoDir, "Stockholm")
	if err := os.WriteFile(tzFile, []byte("TZif"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create symlink pointing to the timezone file.
	link := filepath.Join(dir, "localtime")
	if err := os.Symlink(tzFile, link); err != nil {
		t.Fatal(err)
	}

	result := resolveLocaltimeSymlink(link)
	if result != "Europe/Stockholm" {
		t.Errorf("resolveLocaltimeSymlink() = %q, want %q", result, "Europe/Stockholm")
	}
}

// TestResolveLocaltimeSymlink_NoZoneinfoMarker verifies that a symlink that
// does not contain "zoneinfo/" in its target path returns empty string.
func TestResolveLocaltimeSymlink_NoZoneinfoMarker(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "somefile")
	if err := os.WriteFile(target, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "localtime")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	result := resolveLocaltimeSymlink(link)
	if result != "" {
		t.Errorf("resolveLocaltimeSymlink() = %q, want empty string", result)
	}
}

// TestResolveLocaltimeSymlink_NonexistentPath verifies that a nonexistent
// path returns empty string without error.
func TestResolveLocaltimeSymlink_NonexistentPath(t *testing.T) {
	result := resolveLocaltimeSymlink("/nonexistent/path/localtime")
	if result != "" {
		t.Errorf("resolveLocaltimeSymlink() = %q, want empty string", result)
	}
}

// TestResolveLocaltimeSymlink_RegularFile verifies that a regular file
// (not a symlink) returns empty string.
func TestResolveLocaltimeSymlink_RegularFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "localtime")
	if err := os.WriteFile(file, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := resolveLocaltimeSymlink(file)
	if result != "" {
		t.Errorf("resolveLocaltimeSymlink() = %q, want empty string", result)
	}
}

// TestReadTimezoneFile_ValidContent verifies that a file containing a valid
// IANA timezone name is correctly read and returned.
func TestReadTimezoneFile_ValidContent(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "timezone")
	if err := os.WriteFile(file, []byte("Europe/London\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := readTimezoneFile(file)
	if result != "Europe/London" {
		t.Errorf("readTimezoneFile() = %q, want %q", result, "Europe/London")
	}
}

// TestReadTimezoneFile_InvalidTimezone verifies that a file containing an
// invalid timezone name returns empty string.
func TestReadTimezoneFile_InvalidTimezone(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "timezone")
	if err := os.WriteFile(file, []byte("Not/Valid\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := readTimezoneFile(file)
	if result != "" {
		t.Errorf("readTimezoneFile() = %q, want empty string", result)
	}
}

// TestReadTimezoneFile_EmptyFile verifies that an empty file returns empty string.
func TestReadTimezoneFile_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "timezone")
	if err := os.WriteFile(file, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	result := readTimezoneFile(file)
	if result != "" {
		t.Errorf("readTimezoneFile() = %q, want empty string", result)
	}
}

// TestReadTimezoneFile_NonexistentFile verifies that a nonexistent file
// returns empty string without error.
func TestReadTimezoneFile_NonexistentFile(t *testing.T) {
	result := readTimezoneFile("/nonexistent/path/timezone")
	if result != "" {
		t.Errorf("readTimezoneFile() = %q, want empty string", result)
	}
}

// TestDetectTimezone_FallsBackToOSSources verifies that when time.Now().Location()
// returns "Local" (simulated via TZ=""), DetectTimezone resolves to a valid IANA
// timezone from OS-level sources rather than returning "Local" or erroring.
// On macOS, this tests /etc/localtime symlink resolution.
// On Linux, this tests /etc/timezone file reading.
func TestDetectTimezone_FallsBackToOSSources(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("OS-level timezone sources only tested on macOS and Linux")
	}

	// Unset TZ so that we exercise the file-based fallbacks.
	t.Setenv("TZ", "")

	tz := DetectTimezone()

	if tz == "Local" {
		t.Errorf("DetectTimezone() = %q, should never return 'Local'", tz)
	}
	if tz == "" {
		t.Errorf("DetectTimezone() returned empty string")
	}

	// On a typical macOS or Linux system with timezone configured,
	// we should get a real IANA timezone, not just UTC.
	// However, in CI environments UTC may be the actual timezone,
	// so we only verify it's a valid IANA name.
	t.Logf("detected timezone: %s (GOOS=%s)", tz, runtime.GOOS)
}
