package docs_test

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/desek/outlook-local-mcp/internal/docs"
)

// allowedSlugs is the exhaustive set of slugs permitted in the embedded bundle.
// Engineering documentation (CRs, reference spec, research notes, CHANGELOG)
// must never appear here. This allowlist enforces FR-10 from CR-0061.
var allowedSlugs = map[string]struct{}{
	"readme":          {},
	"quickstart":      {},
	"troubleshooting": {},
}

// forbiddenPathPrefixes lists path fragments that must never appear in any
// embedded file path. A match indicates an engineering doc was accidentally
// added to the bundle.
var forbiddenPathPrefixes = []string{
	"docs/cr/",
	"docs/reference/",
	"docs/research/",
	"CHANGELOG",
	"changelog",
}

// TestBundle_OnlyAllowedSlugsPresent verifies that the embedded bundle contains
// exactly the allowed slugs and no engineering documentation paths.
//
// This is a build-time gate: if a developer adds a disallowed file to the
// embed directive in embed.go, this test fails the build.
func TestBundle_OnlyAllowedSlugsPresent(t *testing.T) {
	t.Parallel()

	err := fs.WalkDir(docs.Bundle, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		// Check forbidden prefixes.
		for _, prefix := range forbiddenPathPrefixes {
			if strings.Contains(path, prefix) {
				t.Errorf("forbidden engineering doc path found in bundle: %q (matches prefix %q)", path, prefix)
			}
		}

		// Derive slug from path "files/{slug}.md".
		if !strings.HasPrefix(path, "files/") || !strings.HasSuffix(path, ".md") {
			t.Errorf("unexpected file path in bundle: %q (expected files/*.md)", path)
			return nil
		}
		slug := strings.TrimPrefix(strings.TrimSuffix(path, ".md"), "files/")
		if _, ok := allowedSlugs[slug]; !ok {
			t.Errorf("unexpected slug %q in bundle (not in allowlist)", slug)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir(Bundle) error: %v", err)
	}
}

// TestBundle_AllAllowedSlugsPresent is the complement of TestBundle_OnlyAllowedSlugsPresent:
// it ensures every slug in the allowlist is actually present in the bundle,
// preventing silent omissions.
func TestBundle_AllAllowedSlugsPresent(t *testing.T) {
	t.Parallel()

	found := make(map[string]bool, len(allowedSlugs))
	err := fs.WalkDir(docs.Bundle, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasPrefix(path, "files/") && strings.HasSuffix(path, ".md") {
			slug := strings.TrimPrefix(strings.TrimSuffix(path, ".md"), "files/")
			found[slug] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir(Bundle) error: %v", err)
	}

	for slug := range allowedSlugs {
		if !found[slug] {
			t.Errorf("allowed slug %q is missing from the bundle", slug)
		}
	}
}
