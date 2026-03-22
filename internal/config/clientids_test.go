package config

import "testing"

// TestResolveClientID_WellKnownName validates that a well-known friendly name
// resolves to the corresponding UUID.
func TestResolveClientID_WellKnownName(t *testing.T) {
	got := ResolveClientID("outlook-desktop")
	want := "d3590ed6-52b3-4102-aeff-aad2292ab01c"
	if got != want {
		t.Errorf("ResolveClientID(%q) = %q, want %q", "outlook-desktop", got, want)
	}
}

// TestResolveClientID_CaseInsensitive validates that well-known name resolution
// is case-insensitive.
func TestResolveClientID_CaseInsensitive(t *testing.T) {
	got := ResolveClientID("Teams-Desktop")
	want := "1fec8e78-bce4-4aaf-ab1b-5451cc387264"
	if got != want {
		t.Errorf("ResolveClientID(%q) = %q, want %q", "Teams-Desktop", got, want)
	}
}

// TestResolveClientID_RawUUID validates that a raw UUID string passes through
// unchanged.
func TestResolveClientID_RawUUID(t *testing.T) {
	input := "dd5fc5c5-eb9a-4f6f-97bd-1a9fecb277d3"
	got := ResolveClientID(input)
	if got != input {
		t.Errorf("ResolveClientID(%q) = %q, want %q", input, got, input)
	}
}

// TestResolveClientID_UnknownName validates that an unknown value containing a
// hyphen (UUID-like) passes through unchanged.
func TestResolveClientID_UnknownName(t *testing.T) {
	input := "my-custom-app"
	got := ResolveClientID(input)
	if got != input {
		t.Errorf("ResolveClientID(%q) = %q, want %q", input, got, input)
	}
}

// TestResolveClientID_Default validates that the project default friendly name
// resolves to the expected UUID.
func TestResolveClientID_Default(t *testing.T) {
	got := ResolveClientID("outlook-local-mcp")
	want := "dd5fc5c5-eb9a-4f6f-97bd-1a9fecb277d3"
	if got != want {
		t.Errorf("ResolveClientID(%q) = %q, want %q", "outlook-local-mcp", got, want)
	}
}
