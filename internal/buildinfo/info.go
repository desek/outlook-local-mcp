package buildinfo

// Info is a read-only snapshot of build identity and host environment
// metadata. Every field is a string so the struct serialises cleanly to JSON
// without special handling.
//
// Fields are populated once per process by Snapshot and are safe to cache
// for the lifetime of the server.
type Info struct {
	// Version is the semver release tag, e.g. "v0.6.1". Set to "dev" when
	// built without ldflags injection.
	Version string `json:"version"`

	// Commit is the short Git SHA injected via ldflags at build time.
	// Set to "unknown" when not injected; may fall back to VCS data from
	// runtime/debug.ReadBuildInfo when ldflags are absent.
	Commit string `json:"commit"`

	// BuildDate is the UTC RFC3339 timestamp of the build, injected via
	// ldflags. Set to "unknown" when not injected.
	BuildDate string `json:"buildDate"`

	// GoVersion is the Go toolchain version string, e.g. "go1.23.4".
	GoVersion string `json:"goVersion"`

	// OS is the operating system as reported by runtime.GOOS.
	OS string `json:"os"`

	// Arch is the CPU architecture as reported by runtime.GOARCH.
	Arch string `json:"arch"`

	// Runtime is a friendly classification of the execution environment.
	// One of: "macos", "linux", "windows", "container", "unknown".
	// Container detection takes precedence over OS detection.
	Runtime string `json:"runtime"`

	// Distribution is a best-effort hint about how the binary was installed.
	// One of: "homebrew", "scoop", "container", "go-install", "binary",
	// "unknown". Documented as informational — not a guarantee.
	Distribution string `json:"distribution"`

	// AuthBackend is the active token cache storage backend, either
	// "keychain" or "file". Passed in by the caller at request time.
	AuthBackend string `json:"authBackend"`

	// Homepage is the project URL.
	Homepage string `json:"homepage"`

	// IssueTracker is the URL for filing bug reports.
	IssueTracker string `json:"issueTracker"`

	// DocsBase is the base URI for in-server documentation.
	DocsBase string `json:"docsBase"`
}

const (
	// ProjectHomepage is the canonical project URL.
	ProjectHomepage = "https://github.com/desek/outlook-local-mcp"

	// ProjectIssueTracker is the canonical bug-report URL.
	ProjectIssueTracker = "https://github.com/desek/outlook-local-mcp/issues"

	// ProjectDocsBase is the base URI for the embedded documentation surface.
	ProjectDocsBase = "doc://outlook-local-mcp/"
)
