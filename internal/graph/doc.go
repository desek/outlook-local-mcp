// Package graph provides Microsoft Graph API utilities for the Outlook
// Calendar MCP Server. It includes error formatting, null-safe pointer
// helpers, event serialization, OData escaping, enum parsing, recurrence
// building, retry logic with exponential backoff, request timeout
// management, and provenance tagging helpers for identifying MCP-created
// events via single-value extended properties (see CR-0040). These
// utilities are shared across all MCP tool handlers.
package graph
