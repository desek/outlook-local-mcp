package graph

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/microsoftgraph/msgraph-sdk-go/models/odataerrors"
)

// docBase is the URI prefix for embedded documentation resources. It is used
// by ErrorSeeHint to build doc:// URIs that point the LLM at the relevant
// troubleshooting section when a known error class is encountered.
const docBase = "doc://outlook-local-mcp/troubleshooting#"

// errorSeeTable maps Graph OData error codes (and sentinel strings used by
// internal error wrappers) to troubleshooting anchor slugs in the embedded
// docs/troubleshooting.md document. Every entry must correspond to a real ##
// heading in that file; the mapping is verified by TestErrorSeeHint_* tests.
var errorSeeTable = map[string]string{
	// Graph throttling
	"TooManyRequests":      "graph-429-throttling",
	"ApplicationThrottled": "graph-429-throttling",

	// Filter / query errors
	"ErrorInvalidRequest": "inefficient-filter",
	"InefficientFilter":   "inefficient-filter",

	// Auth / token errors
	"InvalidAuthenticationToken":  "token-refresh",
	"AuthenticationError":         "authentication-failures",
	"InvalidClientSecretProvided": "authentication-failures",
	"auth_expired":                "token-refresh",

	// Mail feature flags
	"mail_disabled":            "mail-disabled",
	"mail_management_disabled": "mail-management-disabled",
}

// ErrorSeeHint returns the doc:// URI of the troubleshooting section that
// addresses the given error, or an empty string when no mapping exists.
//
// The function first attempts to look up the OData error code from an
// *odataerrors.ODataError. When the error is not an ODataError, the raw
// err.Error() string is scanned against the sentinel keys in errorSeeTable
// (e.g., "auth_expired", "mail_disabled").
//
// Parameters:
//   - err: any error value; may be nil (returns "").
//
// Returns a "doc://outlook-local-mcp/troubleshooting#anchor" URI string, or
// "" when no mapping is found.
//
// Side effects: none.
func ErrorSeeHint(err error) string {
	if err == nil {
		return ""
	}

	// Try to extract the OData error code first.
	var odataErr *odataerrors.ODataError
	if errors.As(err, &odataErr) {
		if mainErr := odataErr.GetErrorEscaped(); mainErr != nil {
			if code := SafeStr(mainErr.GetCode()); code != "" {
				if anchor, ok := errorSeeTable[code]; ok {
					return docBase + anchor
				}
			}
		}
	}

	// Fall back to scanning the raw error string for sentinel keys.
	msg := err.Error()
	for key, anchor := range errorSeeTable {
		if strings.Contains(msg, key) {
			return docBase + anchor
		}
	}

	return ""
}

// emailRedactPattern matches standard email address patterns in arbitrary strings.
// It is compiled once at package initialization time to avoid per-call
// compilation overhead. The pattern covers common Graph API email formats
// (user@domain.tld) without attempting full RFC 5322 compliance.
var emailRedactPattern = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)

// FormatGraphError extracts a human-readable error message from a Graph API error.
// It attempts to unwrap the error as an *odataerrors.ODataError and extract the
// inner error code and message for a structured error string.
//
// Parameters:
//   - err: the error returned by a Microsoft Graph API call.
//
// Returns a formatted string following one of three patterns:
//   - "Graph API error [CODE]: MESSAGE" when the error is an ODataError with a
//     non-nil inner MainError containing code and message fields.
//   - The result of the embedded ApiError.Error() when the error is an ODataError
//     but GetErrorEscaped() returns nil. The parent method is called directly
//     because the SDK's ODataError.Error() unconditionally dereferences the inner
//     error and panics when it is nil.
//   - The result of err.Error() when the error is not an ODataError.
func FormatGraphError(err error) string {
	var odataErr *odataerrors.ODataError
	if errors.As(err, &odataErr) {
		if mainErr := odataErr.GetErrorEscaped(); mainErr != nil {
			code := SafeStr(mainErr.GetCode())
			msg := SafeStr(mainErr.GetMessage())
			return fmt.Sprintf("Graph API error [%s]: %s", code, msg)
		}
		// Call the embedded ApiError.Error() to avoid the panic in
		// ODataError.Error() when GetErrorEscaped() returns nil.
		return odataErr.ApiError.Error()
	}
	return err.Error()
}

// RedactGraphError processes a Graph API error to produce a redacted error
// message suitable for returning to the MCP client. It first calls
// FormatGraphError to extract the structured error string, then replaces
// any email addresses found in the message with "[email redacted]".
//
// Parameters:
//   - err: the error returned by a Microsoft Graph API call.
//
// Returns a redacted error string with email addresses replaced.
//
// RedactGraphError is safe for concurrent use.
func RedactGraphError(err error) string {
	msg := FormatGraphError(err)
	return emailRedactPattern.ReplaceAllString(msg, "[email redacted]")
}
