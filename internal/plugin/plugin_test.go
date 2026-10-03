// @agents-index Tests that pin the plugin manifest, folder contents, version coupling, checksums, licence, and skill text.
package plugin

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

const manifestPath = "plugin/.claude-plugin/plugin.json"

// TestPluginManifestDeclaresListingFields pins the fields the directory
// listing reads, because a missing field fails the directory validation.
func TestPluginManifestDeclaresListingFields(t *testing.T) {
	m := readJSON(t, manifestPath)
	for _, k := range []string{"name", "displayName", "version", "description", "author", "homepage", "repository", "license", "icon", "privacyPolicyUrl", "userConfig", "mcpServers"} {
		if v, ok := m[k]; !ok || v == "" || v == nil {
			t.Errorf("plugin.json field %q is missing or empty", k)
		}
	}
	if _, err := os.Stat(filepath.Join(repoRoot(t), "plugin", "icon.png")); err != nil {
		t.Errorf("plugin icon: %v", err)
	}
}

// TestPluginServerEnvReferencesDeclaredOptions requires every user_config
// reference in the server env to name a declared option with a default, so the
// server never starts with an unresolved placeholder.
func TestPluginServerEnvReferencesDeclaredOptions(t *testing.T) {
	m := readJSON(t, manifestPath)
	uc, _ := m["userConfig"].(map[string]any)
	servers, _ := m["mcpServers"].(map[string]any)
	if len(servers) == 0 {
		t.Fatal("plugin.json declares no mcpServers")
	}
	ref := regexp.MustCompile(`\$\{user_config\.([A-Za-z0-9_]+)\}`)
	for name, s := range servers {
		env, _ := s.(map[string]any)["env"].(map[string]any)
		for k, v := range env {
			for _, mm := range ref.FindAllStringSubmatch(v.(string), -1) {
				opt, ok := uc[mm[1]].(map[string]any)
				if !ok {
					t.Errorf("server %s env %s references undeclared option %q", name, k, mm[1])
					continue
				}
				if _, ok := opt["default"]; !ok {
					t.Errorf("option %q has no default", mm[1])
				}
			}
		}
	}
}

// TestPluginAuthMethodNamesAcceptedValues pins the auth_method option to the
// server's accepted values by its description and default. The directory
// validator does not yet accept an "options" list on a userConfig field, so the
// accepted values live in the description text and no "options" key may be
// present.
func TestPluginAuthMethodNamesAcceptedValues(t *testing.T) {
	m := readJSON(t, manifestPath)
	opt := m["userConfig"].(map[string]any)["auth_method"].(map[string]any)
	if _, has := opt["options"]; has {
		t.Fatal("auth_method declares options; the directory validator rejects that key")
	}
	if opt["default"] != "device_code" {
		t.Errorf("auth_method default = %v, want device_code", opt["default"])
	}
	desc, _ := opt["description"].(string)
	for _, v := range []string{"device_code", "browser", "auth_code"} {
		if !strings.Contains(desc, v) {
			t.Errorf("auth_method description does not name accepted value %q", v)
		}
	}
}

// TestPluginFolderHasNoBlockedFiles keeps the folder small and free of
// binaries, bundles, and symlinks, which the directory review rejects.
func TestPluginFolderHasNoBlockedFiles(t *testing.T) {
	root := filepath.Join(repoRoot(t), "plugin")
	count := 0
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		if d.IsDir() {
			if d.Name() == "bin" {
				t.Errorf("blocked directory %s", rel)
			}
			return nil
		}
		count++
		info, err := os.Lstat(p)
		if err != nil {
			return err
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			t.Errorf("symlink %s", rel)
		}
		if info.Size() > 256*1024 {
			t.Errorf("%s is %d bytes, over 256 KiB", rel, info.Size())
		}
		if ext := filepath.Ext(p); ext == ".mcpb" || ext == ".dxt" {
			t.Errorf("bundle file %s", rel)
		}
		exec := info.Mode()&0o111 != 0
		if exec != (filepath.ToSlash(rel) == "scripts/outlook-local-mcp.sh") {
			t.Errorf("%s executable=%v; only scripts/outlook-local-mcp.sh is executable", rel, exec)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count >= 20 {
		t.Errorf("plugin/ has %d files, want fewer than 20", count)
	}
}

// TestPluginVersionMatchesReleaseManifest couples the plugin version to the
// release version, and requires release-please to bump it.
func TestPluginVersionMatchesReleaseManifest(t *testing.T) {
	rel := readJSON(t, ".release-please-manifest.json")
	if got := readJSON(t, manifestPath)["version"]; got != rel["."] {
		t.Errorf("plugin.json version %v != release manifest %v", got, rel["."])
	}
	// VERSION is what the launcher reads: first token on the first line, followed
	// by the annotation release-please's generic updater rewrites.
	line := strings.TrimSpace(strings.SplitN(string(readRepoFile(t, "plugin/VERSION")), "\n", 2)[0])
	fields := strings.Fields(line)
	if len(fields) == 0 || fields[0] != rel["."] {
		t.Errorf("plugin/VERSION first token %q != release manifest %v", line, rel["."])
	}
	if !strings.Contains(line, "x-release-please-version") {
		t.Errorf("plugin/VERSION lacks the x-release-please-version annotation: %q", line)
	}
	pkg := readJSON(t, "release-please-config.json")["packages"].(map[string]any)["."].(map[string]any)
	extras, _ := pkg["extra-files"].([]any)
	var haveJSON, haveGeneric bool
	for _, e := range extras {
		ef, _ := e.(map[string]any)
		if ef["path"] == manifestPath && ef["jsonpath"] == "$.version" && ef["type"] == "json" {
			haveJSON = true
		}
		if ef["path"] == "plugin/VERSION" && ef["type"] == "generic" {
			haveGeneric = true
		}
	}
	if !haveJSON {
		t.Errorf("release-please-config.json extra-files does not bump %s at $.version", manifestPath)
	}
	if !haveGeneric {
		t.Error("release-please-config.json extra-files does not bump plugin/VERSION (generic)")
	}
}

// TestPluginChecksumsCoverPublishedPlatforms requires each pinned version
// block to hold a digest for every desktop platform the release builds.
func TestPluginChecksumsCoverPublishedPlatforms(t *testing.T) {
	platforms := desktopPlatforms(t)
	blocks := map[string]map[string]bool{}
	cur := ""
	for _, line := range strings.Split(string(readRepoFile(t, "plugin/checksums.txt")), "\n") {
		if strings.HasPrefix(line, "# v") {
			cur = strings.TrimPrefix(line, "# ")
			blocks[cur] = map[string]bool{}
			continue
		}
		if strings.HasPrefix(line, "#") {
			cur = ""
			continue
		}
		if f := strings.Fields(line); cur != "" && len(f) == 2 {
			blocks[cur][f[1]] = true
		}
	}
	for v, assets := range blocks {
		for _, p := range platforms {
			if !assets["outlook-local-mcp-"+p] {
				t.Errorf("block %s has no digest for outlook-local-mcp-%s", v, p)
			}
		}
	}
}

// desktopPlatforms parses the goos and goarch pairs of the desktop build
// matrix, with the .exe suffix that the raw Windows asset carries.
func desktopPlatforms(t *testing.T) []string {
	t.Helper()
	re := regexp.MustCompile(`goos: (\w+)\n\s+goarch: (\w+)`)
	var out []string
	for _, m := range re.FindAllStringSubmatch(string(readRepoFile(t, ".github/workflows/release.yml")), -1) {
		p := m[1] + "-" + m[2]
		if m[1] == "windows" {
			p += ".exe"
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		t.Fatal("no desktop matrix platforms found in release.yml")
	}
	sort.Strings(out)
	return out
}

// TestPluginLicenseMatchesRoot keeps the bundled licence identical to the
// repository licence.
func TestPluginLicenseMatchesRoot(t *testing.T) {
	if string(readRepoFile(t, "plugin/LICENSE")) != string(readRepoFile(t, "LICENSE")) {
		t.Error("plugin/LICENSE differs from LICENSE")
	}
}

// TestPluginSkillNamesDomainsAndGates requires the skill to name every domain
// tool and the variables that gate the opt-in ones.
func TestPluginSkillNamesDomainsAndGates(t *testing.T) {
	s := string(readRepoFile(t, "plugin/skills/outlook/SKILL.md"))
	for _, w := range []string{"`calendar`", "`mail`", "`account`", "`system`", "`contacts`", "`teams`", "OUTLOOK_MCP_CONTACTS_ENABLED", "OUTLOOK_MCP_TEAMS_ENABLED", "OUTLOOK_MCP_MAIL_MANAGE_ENABLED", `operation="help"`} {
		if !strings.Contains(s, w) {
			t.Errorf("SKILL.md does not name %s", w)
		}
	}
}
