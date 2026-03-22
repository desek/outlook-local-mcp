package graph

import "testing"

// TestSafeStr_Nil validates that SafeStr returns an empty string when given
// a nil *string pointer.
func TestSafeStr_Nil(t *testing.T) {
	got := SafeStr(nil)
	if got != "" {
		t.Errorf("SafeStr(nil) = %q, want %q", got, "")
	}
}

// TestSafeStr_NonNil validates that SafeStr returns the dereferenced value
// when given a non-nil *string pointer.
func TestSafeStr_NonNil(t *testing.T) {
	s := "hello"
	got := SafeStr(&s)
	if got != "hello" {
		t.Errorf("SafeStr(&%q) = %q, want %q", s, got, "hello")
	}
}

// TestSafeStr_EmptyString validates that SafeStr returns an empty string
// when given a pointer to an empty string.
func TestSafeStr_EmptyString(t *testing.T) {
	s := ""
	got := SafeStr(&s)
	if got != "" {
		t.Errorf("SafeStr(&%q) = %q, want %q", s, got, "")
	}
}

// TestSafeBool_Nil validates that SafeBool returns false when given a nil
// *bool pointer.
func TestSafeBool_Nil(t *testing.T) {
	got := SafeBool(nil)
	if got != false {
		t.Errorf("SafeBool(nil) = %v, want false", got)
	}
}

// TestSafeBool_True validates that SafeBool returns true when given a pointer
// to a true bool value.
func TestSafeBool_True(t *testing.T) {
	b := true
	got := SafeBool(&b)
	if got != true {
		t.Errorf("SafeBool(&true) = %v, want true", got)
	}
}

// TestSafeBool_False validates that SafeBool returns false when given a pointer
// to a false bool value.
func TestSafeBool_False(t *testing.T) {
	b := false
	got := SafeBool(&b)
	if got != false {
		t.Errorf("SafeBool(&false) = %v, want false", got)
	}
}
