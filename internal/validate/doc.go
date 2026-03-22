// Package validate provides pure validation functions for input sanitization
// across all MCP tool handlers. Each function validates a single concern
// (datetime format, timezone, email, string length, resource ID, or enum
// membership) and returns a descriptive error on invalid input. These
// functions have no side effects and do not log.
package validate
