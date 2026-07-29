package buildinfo

import (
	goruntime "runtime"
	"runtime/debug"
)

// Snapshot constructs an Info from the provided build-time values and the
// current process environment.
//
// Parameters:
//   - version: semver tag injected via ldflags (e.g. "v0.6.1"); use "dev" as
//     the default when ldflags are absent.
//   - commit: short Git SHA injected via ldflags; use "unknown" as default.
//   - buildDate: RFC3339 UTC build timestamp injected via ldflags; use
//     "unknown" as default.
//   - authBackend: the active token cache backend, either "keychain" or
//     "file", provided by the auth subsystem at request time.
//
// When commit or buildDate equal "unknown", Snapshot falls back to VCS
// metadata from runtime/debug.ReadBuildInfo (available for go-installed
// binaries built with Go 1.18+). This satisfies alternative approach #4 from
// the CR: ldflags take precedence, ReadBuildInfo is the fallback.
//
// Snapshot never panics. All detection helpers degrade to safe defaults on
// error.
func Snapshot(version, commit, buildDate, authBackend string) Info {
	commit, buildDate = applyBuildInfoFallback(commit, buildDate)

	return Info{
		Version:      version,
		Commit:       commit,
		BuildDate:    buildDate,
		GoVersion:    goruntime.Version(),
		OS:           goruntime.GOOS,
		Arch:         goruntime.GOARCH,
		Runtime:      RuntimeClass(),
		Distribution: Distribution(),
		AuthBackend:  authBackend,
		Homepage:     ProjectHomepage,
		IssueTracker: ProjectIssueTracker,
		DocsBase:     ProjectDocsBase,
	}
}

// applyBuildInfoFallback uses runtime/debug.ReadBuildInfo VCS settings to
// populate commit and/or buildDate when the ldflags values are "unknown".
// It returns the original values unchanged when ldflags are present or when
// ReadBuildInfo does not carry VCS data.
func applyBuildInfoFallback(commit, buildDate string) (string, string) {
	if commit != "unknown" && buildDate != "unknown" {
		return commit, buildDate
	}
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return commit, buildDate
	}
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			if commit == "unknown" && len(s.Value) >= 7 {
				commit = s.Value[:7]
			}
		case "vcs.time":
			if buildDate == "unknown" && s.Value != "" {
				buildDate = s.Value
			}
		}
	}
	return commit, buildDate
}
