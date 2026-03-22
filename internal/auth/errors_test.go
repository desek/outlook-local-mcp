package auth

import (
	"context"
	"fmt"
	"strings"
	"testing"

	abstractions "github.com/microsoft/kiota-abstractions-go"
	"github.com/microsoftgraph/msgraph-sdk-go/models/odataerrors"
)

// newODataErrorWithStatus creates an *odataerrors.ODataError with the given
// HTTP status code for use in auth error detection tests.
func newODataErrorWithStatus(statusCode int) *odataerrors.ODataError {
	odataErr := odataerrors.NewODataError()
	odataErr.ApiError = *abstractions.NewApiError()
	odataErr.ResponseStatusCode = statusCode
	return odataErr
}

// TestIsAuthError_DeviceCodeCredential verifies that errors containing
// "DeviceCodeCredential" are detected as authentication errors.
func TestIsAuthError_DeviceCodeCredential(t *testing.T) {
	err := fmt.Errorf("DeviceCodeCredential: context deadline exceeded")
	if !IsAuthError(err) {
		t.Error("IsAuthError() = false, want true for DeviceCodeCredential error")
	}
}

// TestIsAuthError_InteractiveBrowserCredential verifies that errors containing
// "InteractiveBrowserCredential" are detected as authentication errors.
func TestIsAuthError_InteractiveBrowserCredential(t *testing.T) {
	err := fmt.Errorf("InteractiveBrowserCredential: authentication failed")
	if !IsAuthError(err) {
		t.Error("IsAuthError() = false, want true for InteractiveBrowserCredential error")
	}
}

// TestIsAuthError_AuthenticationRequired verifies that errors containing
// "authentication required" are detected as authentication errors.
func TestIsAuthError_AuthenticationRequired(t *testing.T) {
	err := fmt.Errorf("authentication required")
	if !IsAuthError(err) {
		t.Error("IsAuthError() = false, want true for authentication required error")
	}
}

// TestIsAuthError_AADSTSError verifies that errors containing Azure AD STS
// error codes (AADSTS prefix) are detected as authentication errors.
func TestIsAuthError_AADSTSError(t *testing.T) {
	err := fmt.Errorf("AADSTS70000: error")
	if !IsAuthError(err) {
		t.Error("IsAuthError() = false, want true for AADSTS error")
	}
}

// TestIsAuthError_HTTP401 verifies that an ODataError with HTTP 401 status
// is detected as an authentication error.
func TestIsAuthError_HTTP401(t *testing.T) {
	err := newODataErrorWithStatus(401)
	if !IsAuthError(err) {
		t.Error("IsAuthError() = false, want true for HTTP 401 ODataError")
	}
}

// TestIsAuthError_NonAuthError verifies that non-authentication errors
// (e.g., network timeouts) are not misidentified as auth errors.
func TestIsAuthError_NonAuthError(t *testing.T) {
	err := fmt.Errorf("network timeout")
	if IsAuthError(err) {
		t.Error("IsAuthError() = true, want false for network timeout error")
	}
}

// TestIsAuthError_NilError verifies that a nil error returns false.
func TestIsAuthError_NilError(t *testing.T) {
	if IsAuthError(nil) {
		t.Error("IsAuthError(nil) = true, want false")
	}
}

// TestIsAuthError_HTTP403_NotAuth verifies that an ODataError with HTTP 403
// status is not detected as an authentication error (403 is authorization,
// not authentication).
func TestIsAuthError_HTTP403_NotAuth(t *testing.T) {
	err := newODataErrorWithStatus(403)
	if IsAuthError(err) {
		t.Error("IsAuthError() = true, want false for HTTP 403 ODataError")
	}
}

// TestIsAuthError_HTTP404_NotAuth verifies that an ODataError with HTTP 404
// status is not detected as an authentication error.
func TestIsAuthError_HTTP404_NotAuth(t *testing.T) {
	err := newODataErrorWithStatus(404)
	if IsAuthError(err) {
		t.Error("IsAuthError() = true, want false for HTTP 404 ODataError")
	}
}

// TestIsAuthError_InteractiveBrowserCredential_DeadlineExceeded verifies that
// a context.DeadlineExceeded error wrapping InteractiveBrowserCredential text
// is detected as an authentication error.
func TestIsAuthError_InteractiveBrowserCredential_DeadlineExceeded(t *testing.T) {
	err := fmt.Errorf("InteractiveBrowserCredential: %w", context.DeadlineExceeded)
	if !IsAuthError(err) {
		t.Error("IsAuthError() = false, want true for InteractiveBrowserCredential with DeadlineExceeded")
	}
}

// TestIsAuthError_DeviceCodeCredential_DeadlineExceeded verifies that a
// context.DeadlineExceeded error wrapping DeviceCodeCredential text is
// detected as an authentication error (preserved behavior).
func TestIsAuthError_DeviceCodeCredential_DeadlineExceeded(t *testing.T) {
	err := fmt.Errorf("DeviceCodeCredential: %w", context.DeadlineExceeded)
	if !IsAuthError(err) {
		t.Error("IsAuthError() = false, want true for DeviceCodeCredential with DeadlineExceeded")
	}
}

// TestFormatAuthError_IncludesTroubleshooting verifies that the formatted
// error message includes a plain-language description and LLM-actionable
// recovery steps referencing MCP tool names.
func TestFormatAuthError_IncludesTroubleshooting(t *testing.T) {
	err := fmt.Errorf("authentication failed: token expired")
	got := FormatAuthError(err)

	requiredSubstrings := []string{
		"Authentication failed",
		"account_list",
		"account_add",
		"retry your original request",
	}

	for _, sub := range requiredSubstrings {
		if !strings.Contains(got, sub) {
			t.Errorf("FormatAuthError() missing %q in output:\n%s", sub, got)
		}
	}
}

// TestFormatAuthError_CredentialAgnostic verifies that the FormatAuthError
// output does not contain device-code-specific text, ensuring the guidance
// is applicable to both browser and device code authentication flows.
func TestFormatAuthError_CredentialAgnostic(t *testing.T) {
	err := fmt.Errorf("some auth error")
	got := FormatAuthError(err)

	deviceCodeSpecific := []string{
		"device code was entered correctly",
		"device code",
	}

	for _, sub := range deviceCodeSpecific {
		if strings.Contains(got, sub) {
			t.Errorf("FormatAuthError() contains device-code-specific text %q in output:\n%s", sub, got)
		}
	}
}

// TestFormatAuthError_NoRawSDKStrings verifies that LLM-actionable output
// from FormatAuthError contains no Azure SDK class names regardless of
// which credential type produced the error (AC-10).
func TestFormatAuthError_NoRawSDKStrings(t *testing.T) {
	sdkErrors := []error{
		fmt.Errorf("DeviceCodeCredential: context deadline exceeded"),
		fmt.Errorf("InteractiveBrowserCredential: authentication failed"),
		fmt.Errorf("AuthorizationCodeCredential: token request failed"),
		fmt.Errorf("DeviceCodeCredential: %w", context.DeadlineExceeded),
		fmt.Errorf("AADSTS70000: DeviceCodeCredential interaction required"),
	}

	bannedSubstrings := []string{
		"DeviceCodeCredential",
		"InteractiveBrowserCredential",
		"AuthorizationCodeCredential",
		"ClientSecretCredential",
		"ManagedIdentityCredential",
	}

	for _, err := range sdkErrors {
		got := FormatAuthError(err)
		for _, banned := range bannedSubstrings {
			if strings.Contains(got, banned) {
				t.Errorf("FormatAuthError(%q) contains raw SDK class name %q in output:\n%s",
					err, banned, got)
			}
		}
	}
}

// TestFormatAuthError_IncludesRecoverySteps verifies that every error output
// from FormatAuthError includes the MCP tool names list_accounts and
// add_account for LLM recovery guidance (AC-10).
func TestFormatAuthError_IncludesRecoverySteps(t *testing.T) {
	testErrors := []error{
		fmt.Errorf("DeviceCodeCredential: context deadline exceeded"),
		fmt.Errorf("InteractiveBrowserCredential: authentication failed"),
		fmt.Errorf("authentication required"),
		fmt.Errorf("AADSTS70000: error"),
		newODataErrorWithStatus(401),
	}

	for _, err := range testErrors {
		got := FormatAuthError(err)
		if !strings.Contains(got, "account_list") {
			t.Errorf("FormatAuthError(%q) missing recovery tool 'account_list' in output:\n%s",
				err, got)
		}
		if !strings.Contains(got, "account_add") {
			t.Errorf("FormatAuthError(%q) missing recovery tool 'add_account' in output:\n%s",
				err, got)
		}
	}
}
