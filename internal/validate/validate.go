package validate

import (
	"fmt"
	"net/mail"
	"strings"
	"time"
)

// Validation limits for string parameters across MCP tools.
const (
	// MaxSubjectLen is the maximum allowed length for event subject strings.
	MaxSubjectLen = 255

	// MaxBodyLen is the maximum allowed length for event body content.
	MaxBodyLen = 32000

	// MaxLocationLen is the maximum allowed length for location display names.
	MaxLocationLen = 255

	// MaxQueryLen is the maximum allowed length for search query strings.
	MaxQueryLen = 255

	// MaxCommentLen is the maximum allowed length for cancellation comment strings.
	MaxCommentLen = 255

	// MaxCategoriesLen is the maximum allowed length for comma-separated category strings.
	MaxCategoriesLen = 1000

	// MaxResourceIDLen is the maximum allowed length for resource identifiers
	// (event IDs, calendar IDs).
	MaxResourceIDLen = 512
)

// datetimeFormats lists the accepted ISO 8601 datetime formats, tried in order
// by ValidateDatetime. The order matters: more specific formats are tried first
// to avoid ambiguous matches.
var datetimeFormats = []string{
	"2006-01-02T15:04:05.000Z07:00",
	"2006-01-02T15:04:05.000Z",
	time.RFC3339,
	"2006-01-02T15:04:05Z",
	"2006-01-02T15:04:05",
}

// ValidateDatetime validates that value conforms to one of the accepted ISO 8601
// datetime formats. The paramName is included in the error message to identify
// the offending parameter.
//
// Parameters:
//   - value: the datetime string to validate.
//   - paramName: the parameter name for error messages.
//
// Returns nil if valid, or an error describing the expected format.
func ValidateDatetime(value, paramName string) error {
	for _, layout := range datetimeFormats {
		if _, err := time.Parse(layout, value); err == nil {
			return nil
		}
	}
	return fmt.Errorf("invalid %s: expected ISO 8601 format (e.g., 2026-04-15T09:00:00), got %q", paramName, Truncate(value, 50))
}

// ValidateTimezone validates that value is a recognized IANA timezone name
// using time.LoadLocation.
//
// Parameters:
//   - value: the timezone string to validate.
//   - paramName: the parameter name for error messages.
//
// Returns nil if valid, or an error describing the expected format.
func ValidateTimezone(value, paramName string) error {
	_, err := time.LoadLocation(value)
	if err != nil {
		return fmt.Errorf("invalid %s: unrecognized timezone %q", paramName, Truncate(value, 50))
	}
	return nil
}

// ValidateEmail validates that email is a well-formed email address using
// net/mail.ParseAddress.
//
// Parameters:
//   - email: the email address string to validate.
//
// Returns nil if valid, or an error describing the issue.
func ValidateEmail(email string) error {
	_, err := mail.ParseAddress(email)
	if err != nil {
		return fmt.Errorf("invalid email address: %q", Truncate(email, 50))
	}
	return nil
}

// ValidateStringLength validates that value does not exceed maxLen characters.
// The paramName is included in the error message to identify the offending
// parameter.
//
// Parameters:
//   - value: the string to check.
//   - paramName: the parameter name for error messages.
//   - maxLen: the maximum allowed length in characters.
//
// Returns nil if within limit, or an error stating the exceeded length.
func ValidateStringLength(value, paramName string, maxLen int) error {
	if len(value) > maxLen {
		return fmt.Errorf("%s exceeds maximum length of %d characters (got %d)", paramName, maxLen, len(value))
	}
	return nil
}

// ValidateResourceID validates that value is a non-empty string within the
// maximum resource ID length. Used for event IDs and calendar IDs.
//
// Parameters:
//   - value: the resource identifier to validate.
//   - paramName: the parameter name for error messages.
//
// Returns nil if valid, or an error describing the issue.
func ValidateResourceID(value, paramName string) error {
	if value == "" {
		return fmt.Errorf("%s must not be empty", paramName)
	}
	if len(value) > MaxResourceIDLen {
		return fmt.Errorf("%s exceeds maximum length of %d characters (got %d)", paramName, MaxResourceIDLen, len(value))
	}
	return nil
}

// ValidateImportance validates that value is one of the accepted importance
// values: low, normal, or high (case-insensitive).
//
// Parameters:
//   - value: the importance string to validate.
//
// Returns nil if valid, or an error listing the accepted values.
func ValidateImportance(value string) error {
	switch strings.ToLower(value) {
	case "low", "normal", "high":
		return nil
	default:
		return fmt.Errorf("invalid importance: %q (accepted: low, normal, high)", value)
	}
}

// ValidateSensitivity validates that value is one of the accepted sensitivity
// values: normal, personal, private, or confidential (case-insensitive).
//
// Parameters:
//   - value: the sensitivity string to validate.
//
// Returns nil if valid, or an error listing the accepted values.
func ValidateSensitivity(value string) error {
	switch strings.ToLower(value) {
	case "normal", "personal", "private", "confidential":
		return nil
	default:
		return fmt.Errorf("invalid sensitivity: %q (accepted: normal, personal, private, confidential)", value)
	}
}

// ValidateShowAs validates that value is one of the accepted free/busy status
// values: free, tentative, busy, oof, or workingElsewhere (case-insensitive).
//
// Parameters:
//   - value: the show_as string to validate.
//
// Returns nil if valid, or an error listing the accepted values.
func ValidateShowAs(value string) error {
	switch strings.ToLower(value) {
	case "free", "tentative", "busy", "oof", "workingelsewhere":
		return nil
	default:
		return fmt.Errorf("invalid show_as: %q (accepted: free, tentative, busy, oof, workingElsewhere)", value)
	}
}

// ValidateRecipients parses a comma-separated list of email addresses and
// validates each entry with ValidateEmail. Empty-after-trim entries are
// skipped. The paramName is included in error messages to identify the
// offending parameter.
//
// Parameters:
//   - value: the comma-separated recipients string. May be empty.
//   - paramName: the parameter name for error messages.
//
// Returns the parsed slice of trimmed email addresses, or an error when any
// entry is not a well-formed email address. Returns an empty (nil) slice
// when value is empty or contains only whitespace.
//
// Side effects: none.
func ValidateRecipients(value, paramName string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parts := strings.Split(value, ",")
	var out []string
	for _, p := range parts {
		addr := strings.TrimSpace(p)
		if addr == "" {
			continue
		}
		if err := ValidateEmail(addr); err != nil {
			return nil, fmt.Errorf("invalid %s: %w", paramName, err)
		}
		out = append(out, addr)
	}
	return out, nil
}

// ValidateContentType validates that value is one of the accepted mail body
// content types: "text" or "html" (case-insensitive).
//
// Parameters:
//   - value: the content type string to validate.
//
// Returns nil if valid, or an error listing the accepted values.
func ValidateContentType(value string) error {
	switch strings.ToLower(value) {
	case "text", "html":
		return nil
	default:
		return fmt.Errorf("invalid content_type: %q (accepted: text, html)", value)
	}
}

// ValidateAttendeeType validates that value is one of the accepted attendee
// type values: required, optional, or resource (case-insensitive).
//
// Parameters:
//   - value: the attendee type string to validate.
//
// Returns nil if valid, or an error listing the accepted values.
func ValidateAttendeeType(value string) error {
	switch strings.ToLower(value) {
	case "required", "optional", "resource":
		return nil
	default:
		return fmt.Errorf("invalid attendee type: %q (accepted: required, optional, resource)", value)
	}
}

// Truncate returns s shortened to maxLen characters with "..." appended if
// truncation occurred. If s is at or below maxLen, it is returned unchanged.
//
// Parameters:
//   - s: the string to truncate.
//   - maxLen: the maximum length before truncation.
//
// Returns the possibly truncated string.
func Truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
