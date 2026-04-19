// Package extension houses the MCPB extension bundle (manifest.json, icons,
// and platform-specific binaries) shipped to Claude Desktop for tool
// discovery. No Go runtime code lives here; this file exists solely to make
// the directory a valid Go package so that manifest-validation tests can
// be compiled and executed by `go test ./...`.
package extension
