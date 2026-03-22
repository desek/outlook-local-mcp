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
