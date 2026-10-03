// Package docs_test: @agents-index Asserts PRIVACY.md covers the required policy areas and the READMEs link it.
package docs_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// readRepoFile reads a file relative to the repository root. PRIVACY.md is not
// part of the embedded bundle, so the tests read the working tree instead.
func readRepoFile(t *testing.T, rel string) (string, error) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", rel))
	return string(data), err
}

// h2Headings returns the lower-cased text of every level-two heading.
func h2Headings(doc string) []string {
	var out []string
	for _, line := range strings.Split(doc, "\n") {
		if strings.HasPrefix(line, "## ") {
			out = append(out, strings.ToLower(strings.TrimPrefix(line, "## ")))
		}
	}
	return out
}

// TestPrivacyPolicyCoversRequiredAreas verifies that the privacy policy has a
// section for each area a directory submission requires, and names every
// resource class the server can read, so the policy cannot silently fall
// behind the tool surface.
func TestPrivacyPolicyCoversRequiredAreas(t *testing.T) {
	t.Parallel()
	doc, err := readRepoFile(t, "PRIVACY.md")
	if err != nil {
		t.Fatalf("read PRIVACY.md: %v", err)
	}
	headings := h2Headings(doc)
	areas := map[string][]string{
		"data access or collection": {"access", "collect"},
		"processing or usage":       {"process", "usage", "use"},
		"storage":                   {"storage", "stor"},
		"third-party services":      {"third-party", "sharing"},
		"data retention":            {"retention"},
		"contact":                   {"contact"},
	}
	for area, keys := range areas {
		found := false
		for _, h := range headings {
			for _, k := range keys {
				if strings.Contains(h, k) {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("PRIVACY.md has no H2 heading for %s (keys %v); headings: %v", area, keys, headings)
		}
	}
	for _, word := range []string{"calendar", "attachment", "mail", "folder", "draft", "free/busy", "contacts", "people", "Teams", "channel", "meeting", "transcript"} {
		if !strings.Contains(strings.ToLower(doc), strings.ToLower(word)) {
			t.Errorf("PRIVACY.md does not mention %q; add it to the data-access section", word)
		}
	}
}

// TestReadmesLinkPrivacyPolicy verifies that the repository README and the
// plugin README each carry a Privacy Policy section linking the policy.
func TestReadmesLinkPrivacyPolicy(t *testing.T) {
	t.Parallel()
	cases := []struct {
		file string
		link *regexp.Regexp
	}{
		{"README.md", regexp.MustCompile(`\]\(PRIVACY\.md\)`)},
		{"plugin/README.md", regexp.MustCompile(`\]\((\.\./)?PRIVACY\.md\)`)},
	}
	for _, c := range cases {
		doc, err := readRepoFile(t, c.file)
		if errors.Is(err, fs.ErrNotExist) {
			t.Logf("skip %s: file absent", c.file)
			continue
		}
		if err != nil {
			t.Fatalf("read %s: %v", c.file, err)
		}
		idx := strings.Index(doc, "\n## Privacy Policy")
		if idx < 0 {
			t.Errorf("%s has no \"## Privacy Policy\" section", c.file)
			continue
		}
		section := doc[idx+1:]
		if end := strings.Index(section[3:], "\n## "); end >= 0 {
			section = section[:end+3]
		}
		if !c.link.MatchString(section) {
			t.Errorf("%s Privacy Policy section does not link the policy (want %s)", c.file, c.link)
		}
	}
}
