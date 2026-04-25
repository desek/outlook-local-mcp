package graph

import (
	"errors"
	"strings"
	"testing"

	"github.com/microsoftgraph/msgraph-sdk-go/models/odataerrors"
)

// TestFormatGraphError_ODataError validates that FormatGraphError correctly
// unwraps an *odataerrors.ODataError with a non-nil MainError, extracting the
// code and message into the "Graph API error [CODE]: MESSAGE" format.
func TestFormatGraphError_ODataError(t *testing.T) {
	code := "NotFound"
	msg := "Item not found"

	mainErr := odataerrors.NewMainError()
	mainErr.SetCode(&code)
	mainErr.SetMessage(&msg)

	odataErr := odataerrors.NewODataError()
	odataErr.SetErrorEscaped(mainErr)

	got := FormatGraphError(odataErr)
	want := "Graph API error [NotFound]: Item not found"
	if got != want {
		t.Errorf("FormatGraphError() = %q, want %q", got, want)
	}
}

// TestFormatGraphError_ODataError_NilInner validates that FormatGraphError
// falls back to the embedded ApiError.Error() when the ODataError has no inner
// MainError (GetErrorEscaped() returns nil). The SDK's ODataError.Error()
// panics in this case, so FormatGraphError calls the parent method directly.
func TestFormatGraphError_ODataError_NilInner(t *testing.T) {
	odataErr := odataerrors.NewODataError()

	got := FormatGraphError(odataErr)
	want := odataErr.ApiError.Error()
	if got != want {
		t.Errorf("FormatGraphError() = %q, want %q", got, want)
	}
}

// TestFormatGraphError_GenericError validates that FormatGraphError falls back
// to err.Error() when the error is not an *odataerrors.ODataError.
func TestFormatGraphError_GenericError(t *testing.T) {
	err := errors.New("timeout")

	got := FormatGraphError(err)
	want := "timeout"
	if got != want {
		t.Errorf("FormatGraphError() = %q, want %q", got, want)
	}
}

// TestRedactGraphError_WithEmail verifies that email addresses in Graph error
// messages are replaced with "[email redacted]".
func TestRedactGraphError_WithEmail(t *testing.T) {
	mainErr := odataerrors.NewMainError()
	code := "MailboxNotFound"
	msg := "The mailbox user@example.com was not found"
	mainErr.SetCode(&code)
	mainErr.SetMessage(&msg)

	odataErr := odataerrors.NewODataError()
	odataErr.SetErrorEscaped(mainErr)

	got := RedactGraphError(odataErr)
	if strings.Contains(got, "user@example.com") {
		t.Errorf("RedactGraphError should not contain email, got %q", got)
	}
	if !strings.Contains(got, "[email redacted]") {
		t.Errorf("expected [email redacted], got %q", got)
	}
}

// TestRedactGraphError_WithoutEmail verifies that error messages without email
// addresses pass through unchanged from FormatGraphError.
func TestRedactGraphError_WithoutEmail(t *testing.T) {
	mainErr := odataerrors.NewMainError()
	code := "NotFound"
	msg := "Resource not found"
	mainErr.SetCode(&code)
	mainErr.SetMessage(&msg)

	odataErr := odataerrors.NewODataError()
	odataErr.SetErrorEscaped(mainErr)

	got := RedactGraphError(odataErr)
	want := FormatGraphError(odataErr)
	if got != want {
		t.Errorf("RedactGraphError() = %q, want %q", got, want)
	}
}

// TestRedactGraphError_NonODataError verifies that non-OData errors with email
// addresses are also redacted.
func TestRedactGraphError_NonODataError(t *testing.T) {
	err := errors.New("user alice@a.com failed")
	got := RedactGraphError(err)
	if strings.Contains(got, "alice@a.com") {
		t.Errorf("RedactGraphError should not contain email, got %q", got)
	}
	if !strings.Contains(got, "[email redacted]") {
		t.Errorf("expected [email redacted], got %q", got)
	}
}

// TestErrorSeeHint_InefficientFilter verifies that a Graph ErrorInvalidRequest
// or InefficientFilter OData error returns the troubleshooting URI for the
// inefficient-filter anchor. This assertion acts as a build guard: if the
// mapping is removed or the anchor changes, this test fails.
func TestErrorSeeHint_InefficientFilter(t *testing.T) {
	code := "ErrorInvalidRequest"
	msg := "The request filter is inefficient"

	mainErr := odataerrors.NewMainError()
	mainErr.SetCode(&code)
	mainErr.SetMessage(&msg)

	odataErr := odataerrors.NewODataError()
	odataErr.SetErrorEscaped(mainErr)

	got := ErrorSeeHint(odataErr)
	want := "doc://outlook-local-mcp/troubleshooting#inefficient-filter"
	if got != want {
		t.Errorf("ErrorSeeHint() = %q, want %q", got, want)
	}
}

// TestErrorSeeHint_Throttling verifies that TooManyRequests OData errors
// resolve to the graph-429-throttling anchor.
func TestErrorSeeHint_Throttling(t *testing.T) {
	code := "TooManyRequests"
	msg := "Too many requests"

	mainErr := odataerrors.NewMainError()
	mainErr.SetCode(&code)
	mainErr.SetMessage(&msg)

	odataErr := odataerrors.NewODataError()
	odataErr.SetErrorEscaped(mainErr)

	got := ErrorSeeHint(odataErr)
	want := "doc://outlook-local-mcp/troubleshooting#graph-429-throttling"
	if got != want {
		t.Errorf("ErrorSeeHint() = %q, want %q", got, want)
	}
}

// TestErrorSeeHint_SentinelString verifies that the sentinel string
// "auth_expired" embedded in a plain error is mapped to the token-refresh
// anchor via the fallback string-scan path.
func TestErrorSeeHint_SentinelString(t *testing.T) {
	err := errors.New("auth_expired: token cache empty")
	got := ErrorSeeHint(err)
	want := "doc://outlook-local-mcp/troubleshooting#token-refresh"
	if got != want {
		t.Errorf("ErrorSeeHint() = %q, want %q", got, want)
	}
}

// TestErrorSeeHint_Nil verifies that a nil error returns an empty string.
func TestErrorSeeHint_Nil(t *testing.T) {
	if got := ErrorSeeHint(nil); got != "" {
		t.Errorf("ErrorSeeHint(nil) = %q, want empty", got)
	}
}

// TestErrorSeeHint_Unknown verifies that an unmapped error returns an empty
// string rather than a garbage URI.
func TestErrorSeeHint_Unknown(t *testing.T) {
	err := errors.New("completely unknown error class")
	if got := ErrorSeeHint(err); got != "" {
		t.Errorf("ErrorSeeHint(unknown) = %q, want empty", got)
	}
}
