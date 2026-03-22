package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// DetectTimezone determines the IANA timezone name for the current system.
// It first checks time.Now().Location(); if that returns "Local" (common in
// sandboxed environments like MCPB), it falls back through OS-level sources:
//
//  1. The TZ environment variable.
//  2. On macOS (darwin): the /etc/localtime symlink target, which typically
//     points to a path under /var/db/timezone/zoneinfo/ or /usr/share/zoneinfo/.
//  3. On Linux: the contents of /etc/timezone.
//  4. UTC as a last resort.
//
// Returns a valid IANA timezone name (e.g., "Europe/Stockholm", "America/New_York",
// or "UTC" if no OS-level source is available).
//
// Side effects: reads environment variables, reads filesystem paths, logs warnings.
func DetectTimezone() string {
	tz := time.Now().Location().String()
	if tz != "Local" {
		return tz
	}

	slog.Debug("timezone auto-detection returned 'Local', trying OS-level sources")

	// Source 1: TZ environment variable.
	if envTZ := os.Getenv("TZ"); envTZ != "" {
		if _, err := time.LoadLocation(envTZ); err == nil {
			slog.Info("timezone resolved from TZ environment variable", "timezone", envTZ)
			return envTZ
		}
		slog.Warn("TZ environment variable contains invalid timezone", "TZ", envTZ)
	}

	// Source 2 (macOS): /etc/localtime symlink target.
	if runtime.GOOS == "darwin" {
		if resolved := resolveLocaltimeSymlink("/etc/localtime"); resolved != "" {
			slog.Info("timezone resolved from /etc/localtime symlink", "timezone", resolved)
			return resolved
		}
	}

	// Source 3 (Linux): /etc/timezone file contents.
	if runtime.GOOS == "linux" {
		if resolved := readTimezoneFile("/etc/timezone"); resolved != "" {
			slog.Info("timezone resolved from /etc/timezone", "timezone", resolved)
			return resolved
		}
	}

	slog.Warn("timezone auto-detection returned 'Local', falling back to UTC")
	return "UTC"
}

// resolveLocaltimeSymlink reads the symlink at path and extracts the IANA
// timezone name from the target. The symlink typically points to a path like
// /var/db/timezone/zoneinfo/Europe/Stockholm or /usr/share/zoneinfo/US/Eastern.
// The function searches for a "zoneinfo/" component in the resolved path and
// returns everything after it.
//
// Parameters:
//   - path: the filesystem path to read as a symlink (typically /etc/localtime).
//
// Returns the extracted IANA timezone name, or "" if the symlink cannot be
// resolved or does not contain a zoneinfo path component.
func resolveLocaltimeSymlink(path string) string {
	target, err := os.Readlink(path)
	if err != nil {
		return ""
	}

	// Resolve to absolute path in case the symlink is relative.
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(path), target)
	}
	target = filepath.Clean(target)

	const marker = "zoneinfo/"
	idx := strings.Index(target, marker)
	if idx < 0 {
		return ""
	}

	iana := target[idx+len(marker):]
	if iana == "" {
		return ""
	}

	// Validate the extracted name.
	if _, err := time.LoadLocation(iana); err != nil {
		return ""
	}
	return iana
}

// readTimezoneFile reads the contents of the given file path and returns the
// trimmed content as an IANA timezone name, if valid.
//
// Parameters:
//   - path: the filesystem path to read (typically /etc/timezone).
//
// Returns the IANA timezone name from the file, or "" if the file cannot be
// read or contains an invalid timezone name.
func readTimezoneFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	iana := strings.TrimSpace(string(data))
	if iana == "" {
		return ""
	}

	if _, err := time.LoadLocation(iana); err != nil {
		return ""
	}
	return iana
}
