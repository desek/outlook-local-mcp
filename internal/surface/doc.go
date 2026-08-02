// Package surface builds the surface manifest: a deterministic, code-derived
// record of the server's tool surface (domains, verbs, gates, counts) and its
// configuration variable inventory (CR-0073 Phase 1).
//
// The manifest is the single path by which a fact about the server reaches the
// website. It is built from the live domain verb builders in internal/server
// and the configuration inventory in internal/config, without any network call
// or credential, so it runs in continuous integration and offline. It
// serializes to byte-identical output on repeated runs against an unchanged
// tree, which the drift check depends on.
//
// @agents-index: package building the code-derived surface manifest record.
package surface
