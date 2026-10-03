// Package plugin holds tests that pin the contract of the Claude Code plugin bundle in plugin/ and its release coupling.
//
// @agents-index Test-only package: asserts the plugin/ bundle, its launcher, and the release workflow steps that publish and pin its binaries.
//
// The package has no production code. The plugin is a folder of JSON, Markdown,
// and one POSIX shell launcher, so its contract is checked here, beside the Go
// test suite, so that a drift fails `make test` rather than a directory review.
package plugin
