// Package graph provides Microsoft Graph API utilities.
//
// This file contains tests for the EscapeOData utility function, validating
// single-quote escaping, empty string handling, and passthrough for strings
// without special characters.
package graph

import "testing"

// TestEscapeOData_SingleQuotes validates that single quotes are doubled for
// OData string literal escaping. Input "O'Brien" must become "O”Brien".
func TestEscapeOData_SingleQuotes(t *testing.T) {
	got := EscapeOData("O'Brien")
	want := "O''Brien"
	if got != want {
		t.Errorf("EscapeOData(%q) = %q, want %q", "O'Brien", got, want)
	}
}

// TestEscapeOData_EmptyString validates that an empty string passes through
// unchanged.
func TestEscapeOData_EmptyString(t *testing.T) {
	got := EscapeOData("")
	if got != "" {
		t.Errorf("EscapeOData(%q) = %q, want %q", "", got, "")
	}
}

// TestEscapeOData_NoSpecialChars validates that a string without single quotes
// passes through unchanged.
func TestEscapeOData_NoSpecialChars(t *testing.T) {
	got := EscapeOData("meeting")
	want := "meeting"
	if got != want {
		t.Errorf("EscapeOData(%q) = %q, want %q", "meeting", got, want)
	}
}

// TestEscapeOData_MultipleSingleQuotes validates that multiple single quotes in
// a string are all doubled independently.
func TestEscapeOData_MultipleSingleQuotes(t *testing.T) {
	got := EscapeOData("it's a 'test'")
	want := "it''s a ''test''"
	if got != want {
		t.Errorf("EscapeOData(%q) = %q, want %q", "it's a 'test'", got, want)
	}
}
