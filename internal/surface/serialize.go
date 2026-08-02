// Package surface — this file serializes the Record to the manifest's canonical
// JSON form (CR-0073 Phase 1).
//
// The serialization is deterministic by construction: the Record contains no
// maps, every slice is built in a fixed order, and no timestamp, commit, or
// environment value is present. Repeated runs against an unchanged tree
// therefore produce byte-identical output, which the drift check depends on.
//
// @agents-index: deterministic JSON serialization of the surface Record.
package surface

import (
	"bytes"
	"encoding/json"
)

// Serialize renders the Record as indented JSON with a trailing newline.
//
// HTML escaping is disabled so characters such as '<' in descriptions are not
// re-encoded, keeping the output human-readable and stable. The output is
// byte-identical across runs for an equal Record.
//
// Parameters:
//   - rec: the Record to serialize.
//
// Returns the JSON bytes, or an error if encoding fails.
func Serialize(rec Record) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(rec); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
