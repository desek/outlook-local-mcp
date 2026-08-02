// Package surface — tests for deterministic serialization of the Record
// (CR-0073 Phase 1).
//
// @agents-index: tests for byte-identical, environment-free serialization.
package surface

import (
	"bytes"
	"strings"
	"testing"
)

// TestSerializationIsDeterministic asserts that serializing an independently
// built record twice produces byte-identical output, the property the drift
// check depends on (FR-5, NFR-1).
func TestSerializationIsDeterministic(t *testing.T) {
	first, err := Serialize(BuildRecord())
	if err != nil {
		t.Fatalf("first serialize: %v", err)
	}
	second, err := Serialize(BuildRecord())
	if err != nil {
		t.Fatalf("second serialize: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Errorf("serialization is not deterministic:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

// TestSerializationCarriesNoEnvironmentValue asserts the serialized manifest
// contains no timestamp, commit, build, or host-environment field, so an
// unchanged tree always serializes identically regardless of when or where the
// generator runs (FR-5).
func TestSerializationCarriesNoEnvironmentValue(t *testing.T) {
	out, err := Serialize(BuildRecord())
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	text := string(out)

	// Field names that would indicate a non-deterministic or environment value
	// leaked into the manifest.
	forbidden := []string{
		"timestamp", "generatedAt", "generated_at", "buildDate", "build_date",
		"commit", "\"date\"", "hostname", "\"host\"",
	}
	for _, f := range forbidden {
		if strings.Contains(text, f) {
			t.Errorf("serialized manifest contains forbidden token %q", f)
		}
	}
}
