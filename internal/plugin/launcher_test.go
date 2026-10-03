// @agents-index Tests that drive the plugin launcher script against a fake release server and pin its download, verification, and refusal rules.
package plugin

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
)

const (
	testVersion = "0.0.0-test"
	marker      = "fake-server-started"
	fakeBinary  = "#!/bin/sh\necho " + marker + "\n"
)

// launcherEnv is one isolated launcher run: a plugin copy, a data dir, a
// shim dir for uname, and a fake release server that counts requests.
type launcherEnv struct {
	root, data, shims string
	server            *httptest.Server
	requests          atomic.Int32
	asset             []byte
	extraEnv          []string
}

// newLauncherEnv builds the environment with uname reporting kernel/machine.
func newLauncherEnv(t *testing.T, kernel, machine string) *launcherEnv {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the launcher is a POSIX shell script")
	}
	e := &launcherEnv{data: t.TempDir(), shims: t.TempDir(), asset: []byte(fakeBinary)}
	e.root = copyPlugin(t)
	writeFile(t, filepath.Join(e.shims, "uname"), "#!/bin/sh\ncase \"$1\" in -s) echo "+kernel+";; -m) echo "+machine+";; esac\n", 0o755)
	sum := sha256.Sum256([]byte(fakeBinary))
	sums := hex.EncodeToString(sum[:]) + "  outlook-local-mcp-darwin-arm64\n"
	e.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		e.requests.Add(1)
		switch r.URL.Path {
		case "/v" + testVersion + "/outlook-local-mcp-darwin-arm64":
			_, _ = w.Write(e.asset)
		case "/v" + testVersion + "/checksums.txt":
			_, _ = w.Write([]byte(sums))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(e.server.Close)
	return e
}

// copyPlugin copies plugin/ to a temp dir with the test version in plugin.json.
func copyPlugin(t *testing.T) string {
	t.Helper()
	src := filepath.Join(repoRoot(t), "plugin")
	dst := t.TempDir()
	err := filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if info.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), b, info.Mode().Perm())
	})
	if err != nil {
		t.Fatal(err)
	}
	mp := filepath.Join(dst, ".claude-plugin", "plugin.json")
	b, _ := os.ReadFile(mp)
	v, _ := readJSON(t, manifestPath)["version"].(string)
	writeFile(t, mp, strings.Replace(string(b), `"version": "`+v+`"`, `"version": "`+testVersion+`"`, 1), 0o644)
	return dst
}

// writeFile writes content to path with mode, failing the test on error.
func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

// run executes the launcher with /bin/sh and returns stdout, stderr, and the error.
func (e *launcherEnv) run(t *testing.T) (string, string, error) {
	t.Helper()
	cmd := exec.Command("/bin/sh", filepath.Join(e.root, "scripts", "outlook-local-mcp.sh"))
	cmd.Env = append([]string{
		"PATH=" + e.shims + ":/usr/bin:/bin:/usr/sbin:/sbin",
		"CLAUDE_PLUGIN_ROOT=" + e.root,
		"CLAUDE_PLUGIN_DATA=" + e.data,
		"OUTLOOK_MCP_PLUGIN_RELEASE_BASE=" + e.server.URL,
	}, e.extraEnv...)
	var out, errb strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	return out.String(), errb.String(), err
}

// binPath is where the launcher caches the verified binary.
func (e *launcherEnv) binPath() string {
	return filepath.Join(e.data, testVersion, "outlook-local-mcp")
}

// TestLauncherColdStartFetchesVerifiesAndExecs pins the cold start without a
// committed pin: fetch asset and release checksums, warn, cache, and exec.
func TestLauncherColdStartFetchesVerifiesAndExecs(t *testing.T) {
	e := newLauncherEnv(t, "Darwin", "arm64")
	out, errOut, err := e.run(t)
	if err != nil {
		t.Fatalf("launcher failed: %v\n%s", err, errOut)
	}
	if n := e.requests.Load(); n != 2 {
		t.Errorf("requests = %d, want 2", n)
	}
	if !strings.Contains(errOut, "repository pin is absent") {
		t.Errorf("stderr has no pin-absent notice: %q", errOut)
	}
	if _, err := os.Stat(e.binPath()); err != nil {
		t.Errorf("binary not cached: %v", err)
	}
	if strings.TrimSpace(out) != marker {
		t.Errorf("stdout = %q, want the exec'd marker", out)
	}
}

// TestLauncherPrefersCommittedDigest pins that a committed version block
// replaces the release checksums download and silences the notice.
func TestLauncherPrefersCommittedDigest(t *testing.T) {
	e := newLauncherEnv(t, "Darwin", "arm64")
	sum := sha256.Sum256([]byte(fakeBinary))
	f, _ := os.OpenFile(filepath.Join(e.root, "checksums.txt"), os.O_APPEND|os.O_WRONLY, 0o644)
	_, _ = f.WriteString("# v" + testVersion + "\n" + hex.EncodeToString(sum[:]) + "  outlook-local-mcp-darwin-arm64\n")
	_ = f.Close()
	out, errOut, err := e.run(t)
	if err != nil {
		t.Fatalf("launcher failed: %v\n%s", err, errOut)
	}
	if n := e.requests.Load(); n != 1 {
		t.Errorf("requests = %d, want 1", n)
	}
	if strings.Contains(errOut, "pin is absent") {
		t.Errorf("unexpected notice: %q", errOut)
	}
	if strings.TrimSpace(out) != marker {
		t.Errorf("stdout = %q", out)
	}
}

// TestLauncherWarmStartMakesNoRequest pins that a cached binary runs offline.
func TestLauncherWarmStartMakesNoRequest(t *testing.T) {
	e := newLauncherEnv(t, "Darwin", "arm64")
	if _, _, err := e.run(t); err != nil {
		t.Fatal(err)
	}
	e.requests.Store(0)
	out, _, err := e.run(t)
	if err != nil || strings.TrimSpace(out) != marker {
		t.Fatalf("warm start: err=%v out=%q", err, out)
	}
	if n := e.requests.Load(); n != 0 {
		t.Errorf("warm start made %d requests", n)
	}
}

// TestLauncherRefusesChecksumMismatch pins that a tampered asset is deleted
// and refused with both digests named.
func TestLauncherRefusesChecksumMismatch(t *testing.T) {
	e := newLauncherEnv(t, "Darwin", "arm64")
	e.asset = []byte("#!/bin/sh\necho tampered\n")
	out, errOut, err := e.run(t)
	if err == nil {
		t.Fatal("launcher accepted a tampered asset")
	}
	good := sha256.Sum256([]byte(fakeBinary))
	bad := sha256.Sum256(e.asset)
	for _, d := range []string{hex.EncodeToString(good[:]), hex.EncodeToString(bad[:])} {
		if !strings.Contains(errOut, d) {
			t.Errorf("stderr does not name digest %s: %q", d, errOut)
		}
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty", out)
	}
	entries, _ := os.ReadDir(filepath.Join(e.data, testVersion))
	for _, en := range entries {
		t.Errorf("left behind %s", en.Name())
	}
}

// TestLauncherRefusesUnsupportedPlatform pins the refusal before any request
// on a platform with no published plugin binary.
func TestLauncherRefusesUnsupportedPlatform(t *testing.T) {
	e := newLauncherEnv(t, "Plan9", "mips")
	out, errOut, err := e.run(t)
	if err == nil {
		t.Fatal("launcher accepted an unsupported platform")
	}
	if n := e.requests.Load(); n != 0 {
		t.Errorf("requests = %d, want 0", n)
	}
	if !strings.Contains(errOut, ".mcpb") || !strings.Contains(errOut, "/releases") {
		t.Errorf("stderr does not name the .mcpb and releases page: %q", errOut)
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty", out)
	}
}

// TestLauncherHonoursBinaryOverride pins that the override runs a local
// binary with no request, and refuses a path that is not executable.
func TestLauncherHonoursBinaryOverride(t *testing.T) {
	e := newLauncherEnv(t, "Darwin", "arm64")
	bin := filepath.Join(t.TempDir(), "local")
	writeFile(t, bin, fakeBinary, 0o755)
	e.extraEnv = []string{"OUTLOOK_MCP_PLUGIN_BIN=" + bin}
	out, errOut, err := e.run(t)
	if err != nil || strings.TrimSpace(out) != marker {
		t.Fatalf("override: err=%v out=%q stderr=%q", err, out, errOut)
	}
	if n := e.requests.Load(); n != 0 {
		t.Errorf("override made %d requests", n)
	}
	if err := os.Chmod(bin, 0o644); err != nil {
		t.Fatal(err)
	}
	out, _, err = e.run(t)
	if err == nil {
		t.Error("launcher accepted a non-executable override")
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty", out)
	}
}

// TestLauncherWritesNothingToStdoutBeforeExec pins that every refusal path
// keeps stdout empty, because stdout carries the MCP protocol.
func TestLauncherWritesNothingToStdoutBeforeExec(t *testing.T) {
	cases := map[string]func(e *launcherEnv){
		"mismatch": func(e *launcherEnv) { e.asset = []byte("x") },
		"missing asset": func(e *launcherEnv) {
			e.extraEnv = []string{"OUTLOOK_MCP_PLUGIN_RELEASE_BASE=" + e.server.URL + "/none"}
		},
		"bad override":   func(e *launcherEnv) { e.extraEnv = []string{"OUTLOOK_MCP_PLUGIN_BIN=/nonexistent"} },
		"no plugin data": func(e *launcherEnv) { e.data = "" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			e := newLauncherEnv(t, "Darwin", "arm64")
			mutate(e)
			out, _, err := e.run(t)
			if err == nil {
				t.Fatal("launcher did not refuse")
			}
			if out != "" {
				t.Errorf("stdout = %q, want empty", out)
			}
		})
	}
	t.Run("unsupported platform", func(t *testing.T) {
		e := newLauncherEnv(t, "Plan9", "mips")
		if out, _, _ := e.run(t); out != "" {
			t.Errorf("stdout = %q, want empty", out)
		}
	})
}
