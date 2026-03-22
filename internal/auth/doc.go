// Package auth implements the authentication subsystem for the Outlook Calendar
// MCP Server. It provides configurable credential construction (interactive
// browser or device code flow, see CR-0024), persistent OS-native token cache
// initialization, authentication record persistence, authentication error
// detection, and an AuthMiddleware that intercepts auth errors from tool
// handlers to trigger re-authentication via MCP client notifications.
// Authentication is lazy: it is deferred to the first tool call rather than
// blocking at startup (see CR-0022). The Authenticator interface abstracts
// over both credential types, allowing the middleware to work with either
// without coupling to a specific implementation.
package auth
