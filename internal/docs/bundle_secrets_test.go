package docs_test

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/desek/outlook-local-mcp/internal/docs"
)

// secretPatterns is the denylist of substrings that must not appear in any
// bundled file. This mirrors the lint patterns in `make docs-bundle` and
// enforces NFR-4 from CR-0061.
var secretPatterns = []string{
	"eyJ",           // Base64-encoded JWT prefix
	"sk-",           // OpenAI / generic API key prefix
	"client_secret", // OAuth client secret field name
	"refresh_token", // OAuth refresh token field name
}

// TestBundleContainsNoSecrets scans every file in the embedded bundle for
// common secret and token patterns. A failure means a sensitive string was
// accidentally included in user-facing documentation.
func TestBundleContainsNoSecrets(t *testing.T) {
	t.Parallel()

	err := fs.WalkDir(docs.Bundle, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := docs.Bundle.ReadFile(path)
		if err != nil {
			return err
		}
		content := string(data)
		for _, pattern := range secretPatterns {
			if strings.Contains(content, pattern) {
				t.Errorf("file %q contains secret pattern %q", path, pattern)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir(Bundle) error: %v", err)
	}
}
