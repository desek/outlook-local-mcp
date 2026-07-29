package auth

import "testing"

// TestActiveBackendDefault verifies that the zero state returns "file".
func TestActiveBackendDefault(t *testing.T) {
	t.Parallel()

	// Reset to default so this test is order-independent.
	setActiveBackend("file")

	got := ActiveBackend()
	if got != "file" {
		t.Errorf("ActiveBackend() = %q, want %q", got, "file")
	}
}

// TestActiveBackendSetKeychain verifies that setActiveBackend("keychain")
// is reflected by ActiveBackend.
func TestActiveBackendSetKeychain(t *testing.T) {
	// Not parallel: mutates package-level state.
	setActiveBackend("keychain")
	t.Cleanup(func() { setActiveBackend("file") })

	got := ActiveBackend()
	if got != "keychain" {
		t.Errorf("ActiveBackend() = %q, want %q", got, "keychain")
	}
}

// TestActiveBackendSetFile verifies that setActiveBackend("file")
// is reflected by ActiveBackend.
func TestActiveBackendSetFile(t *testing.T) {
	// Not parallel: mutates package-level state.
	setActiveBackend("keychain")
	setActiveBackend("file")
	t.Cleanup(func() { setActiveBackend("file") })

	got := ActiveBackend()
	if got != "file" {
		t.Errorf("ActiveBackend() = %q, want %q", got, "file")
	}
}
