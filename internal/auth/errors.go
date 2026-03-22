package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/microsoftgraph/msgraph-sdk-go/models/odataerrors"
)

// authErrorPatterns lists substrings whose presence in an error message
// indicates an authentication failure. Each pattern corresponds to a
// known error surface from the Azure Identity SDK or Azure AD:
//   - "DeviceCodeCredential" -- azidentity DeviceCodeCredential failures.
//   - "InteractiveBrowserCredential" -- azidentity InteractiveBrowserCredential failures.
//   - "authentication required" -- explicit auth-needed signals.
//   - "AADSTS" -- Azure AD Security Token Service error codes.
var authErrorPatterns = []string{
	"DeviceCodeCredential",
	"InteractiveBrowserCredential",
	"authentication required",
	"AADSTS",
}

// IsAuthError reports whether err represents an authentication failure
// that should trigger re-authentication.
//
// The function checks three categories of evidence:
//  1. The error message contains a known authentication-related substring
//     (DeviceCodeCredential, InteractiveBrowserCredential, authentication
//     required, or AADSTS).
//  2. The error is a context deadline exceeded error originating from a
//     credential operation (identified by "DeviceCodeCredential" or
//     "InteractiveBrowserCredential" in the error chain text combined
//     with context.DeadlineExceeded).
//  3. The error is an ODataError with HTTP status 401 Unauthorized.
//
// Parameters:
//   - err: the error to inspect. May be nil.
//
// Returns true when the error matches any authentication failure pattern,
// false otherwise. A nil error always returns false.
//
// IsAuthError is safe for concurrent use.
func IsAuthError(err error) bool {
	if err == nil {
		return false
	}

	// Check for HTTP 401 Unauthorized via ODataError first, before calling
	// err.Error(). The ODataError.Error() method panics when GetErrorEscaped()
	// returns nil, so status code extraction must precede string inspection.
	var odataErr *odataerrors.ODataError
	if errors.As(err, &odataErr) {
		return odataErr.ResponseStatusCode == 401
	}

	msg := err.Error()

	// Check known authentication error substrings.
	for _, pattern := range authErrorPatterns {
		if strings.Contains(msg, pattern) {
			return true
		}
	}

	// Check for context deadline exceeded from a credential operation.
	if errors.Is(err, context.DeadlineExceeded) &&
		(strings.Contains(msg, "DeviceCodeCredential") || strings.Contains(msg, "InteractiveBrowserCredential")) {
		return true
	}

	return false
}

// FormatAuthError returns an LLM-actionable error message for the given
// authentication error. The output includes a plain-language description of
// the failure and specific MCP tool names the LLM should call to recover.
// Raw Azure SDK class names (e.g., DeviceCodeCredential,
// InteractiveBrowserCredential) are stripped from the output so the LLM
// does not hallucinate SDK-level troubleshooting advice.
//
// Parameters:
//   - err: the authentication error to format. Must not be nil.
//
// Returns a multi-line string containing a plain-language description and
// recovery instructions referencing MCP tool names.
//
// FormatAuthError is safe for concurrent use.
func FormatAuthError(err error) string {
	description := classifyAuthError(err)
	return description + "\n\nTo recover:\n" +
		"1. Call account_list to check which accounts need authentication\n" +
		"2. Call account_add with the account label to start a new authentication flow\n" +
		"3. After authenticating, retry your original request"
}

// classifyAuthError returns a plain-language description of the authentication
// failure. Azure SDK class names are stripped from the output.
//
// Parameters:
//   - err: the authentication error to classify.
//
// Returns a human-readable description of the error.
func classifyAuthError(err error) string {
	msg := safeErrorString(err)

	// Check for context deadline / timeout errors.
	if strings.Contains(msg, "context deadline exceeded") ||
		strings.Contains(msg, "context canceled") {
		return "Authentication timed out or was cancelled for this account."
	}

	// Check for AADSTS error codes from Azure AD.
	if strings.Contains(msg, "AADSTS") {
		// Extract a sanitized version without SDK class names.
		sanitized := stripSDKClassNames(msg)
		return "Azure AD rejected the authentication request: " + sanitized
	}

	// Check for HTTP 401 via OData.
	var odataErr *odataerrors.ODataError
	if errors.As(err, &odataErr) && odataErr.ResponseStatusCode == 401 {
		return "The server received an unauthorized response from Microsoft Graph. The account token may have expired or been revoked."
	}

	// Check for "authentication required" signals.
	if strings.Contains(msg, "authentication required") {
		return "Authentication is required for this account."
	}

	// Generic auth failure — strip SDK class names.
	sanitized := stripSDKClassNames(msg)
	return "Authentication failed for this account: " + sanitized
}

// sdkClassNames lists Azure SDK class names that must be stripped from
// user-facing error messages to avoid LLM confusion.
var sdkClassNames = []string{
	"DeviceCodeCredential",
	"InteractiveBrowserCredential",
	"AuthorizationCodeCredential",
	"ClientSecretCredential",
	"ManagedIdentityCredential",
}

// stripSDKClassNames removes known Azure SDK credential class names and
// surrounding punctuation from the error string to produce LLM-friendly
// output.
//
// Parameters:
//   - msg: the raw error message string.
//
// Returns the message with SDK class names removed and cleaned up.
func stripSDKClassNames(msg string) string {
	result := msg
	for _, name := range sdkClassNames {
		result = strings.ReplaceAll(result, name+": ", "")
		result = strings.ReplaceAll(result, name, "")
	}
	return strings.TrimSpace(result)
}

// safeErrorString returns the error message string without panicking.
// ODataError.Error() panics when GetErrorEscaped() returns nil, so this
// function extracts a safe string representation for ODataError instances
// by falling back to the embedded ApiError.Error() method.
//
// Parameters:
//   - err: the error to convert to a string.
//
// Returns the error message string.
func safeErrorString(err error) string {
	var odataErr *odataerrors.ODataError
	if errors.As(err, &odataErr) {
		if mainErr := odataErr.GetErrorEscaped(); mainErr != nil {
			code := ""
			if mainErr.GetCode() != nil {
				code = *mainErr.GetCode()
			}
			msg := ""
			if mainErr.GetMessage() != nil {
				msg = *mainErr.GetMessage()
			}
			return fmt.Sprintf("Graph API error [%s]: %s", code, msg)
		}
		return odataErr.ApiError.Error()
	}
	return err.Error()
}
