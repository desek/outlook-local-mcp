// Package buildinfo provides a read-only snapshot of build identity and host
// environment metadata for the outlook-local-mcp server.
//
// It is consumed by the system.about verb to give the LLM a single, stable,
// cacheable call that answers "what am I talking to?" without making any
// Microsoft Graph request. The package carries no mutable state and may be
// called from any goroutine without synchronisation.
package buildinfo
