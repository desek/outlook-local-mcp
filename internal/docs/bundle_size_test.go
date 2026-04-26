package docs_test

import (
	"io/fs"
	"testing"

	extdocs "github.com/desek/outlook-local-mcp/docs"
)

// maxBundleBytes is the hard upper limit for the total uncompressed size of all
// files in the embedded bundle (2 MiB). Enforced by NFR-1 in CR-0061.
const maxBundleBytes = 2 * 1024 * 1024

// TestBundleSizeUnder2MiB verifies that the total uncompressed size of all
// embedded files stays within the 2 MiB budget defined by CR-0061 NFR-1.
// A failure here means a file was added to the bundle without checking its size.
func TestBundleSizeUnder2MiB(t *testing.T) {
	t.Parallel()

	var total int64
	err := fs.WalkDir(extdocs.Bundle, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir(Bundle) error: %v", err)
	}

	if total > maxBundleBytes {
		t.Fatalf("bundle size %d bytes exceeds 2 MiB limit (%d bytes)", total, maxBundleBytes)
	}
	t.Logf("bundle total size: %d bytes (limit: %d)", total, maxBundleBytes)
}
