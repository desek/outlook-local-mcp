package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
)

// mockTokenCredential implements azcore.TokenCredential for testing.
type mockTokenCredential struct {
	// getTokenDelay is how long GetToken blocks before returning.
	getTokenDelay time.Duration

	// getTokenErr is the error returned by GetToken. Nil means success.
	getTokenErr error

	// called tracks whether GetToken was invoked.
	called bool
}

// GetToken implements azcore.TokenCredential.
func (m *mockTokenCredential) GetToken(ctx context.Context, _ policy.TokenRequestOptions) (azcore.AccessToken, error) {
	m.called = true
	if m.getTokenDelay > 0 {
		select {
		case <-time.After(m.getTokenDelay):
		case <-ctx.Done():
			return azcore.AccessToken{}, ctx.Err()
		}
	}
	if m.getTokenErr != nil {
		return azcore.AccessToken{}, m.getTokenErr
	}
	return azcore.AccessToken{
		Token:     "test-token",
		ExpiresOn: time.Now().Add(time.Hour),
	}, nil
}

// TestStartup_SkipsImplicitDefault_WhenAccountsJsonCoversCfg verifies that
// shouldAddImplicitDefault returns false when accounts.json contains an entry
// whose client_id and tenant_id match the env cfg, even under a different label.
func TestStartup_SkipsImplicitDefault_WhenAccountsJsonCoversCfg(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "accounts.json")

	const cfgClientID = "cfg-client-id"
	const cfgTenantID = "cfg-tenant-id"

	content := `{"accounts":[{"label":"primary","client_id":"cfg-client-id","tenant_id":"cfg-tenant-id","auth_method":"browser"}]}`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	if shouldAddImplicitDefault(path, cfgClientID, cfgTenantID) {
		t.Error("shouldAddImplicitDefault returned true; want false when accounts.json covers cfg identity")
	}
}

// TestStartup_SkipsImplicitDefault_WhenDefaultLabelInAccountsJson verifies that
// shouldAddImplicitDefault returns false when accounts.json already contains an
// entry with the literal label "default", regardless of client_id/tenant_id.
func TestStartup_SkipsImplicitDefault_WhenDefaultLabelInAccountsJson(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "accounts.json")

	content := `{"accounts":[{"label":"default","client_id":"other-client","tenant_id":"other-tenant","auth_method":"browser"}]}`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	if shouldAddImplicitDefault(path, "cfg-client-id", "cfg-tenant-id") {
		t.Error("shouldAddImplicitDefault returned true; want false when accounts.json has explicit default label")
	}
}

// TestStartup_AddsImplicitDefault_WhenAccountsJsonEmpty verifies that
// shouldAddImplicitDefault returns true when accounts.json does not exist,
// preserving the single-account env-only configuration UX.
func TestStartup_AddsImplicitDefault_WhenAccountsJsonEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent", "accounts.json")

	if !shouldAddImplicitDefault(path, "cfg-client-id", "cfg-tenant-id") {
		t.Error("shouldAddImplicitDefault returned false; want true when accounts.json is absent")
	}
}

// TestStartupTokenProbe_CompletesWithin5Seconds verifies that the startup
// token probe completes within the 5-second bound, does not trigger
// interactive authentication, and calls markPreAuthenticated on success.
// This validates AC-8.
func TestStartupTokenProbe_CompletesWithin5Seconds(t *testing.T) {
	t.Run("success_calls_markPreAuthenticated", func(t *testing.T) {
		cred := &mockTokenCredential{}
		preAuthCalled := false

		start := time.Now()
		probeStartupToken(cred, "browser", "", func() { preAuthCalled = true }, []string{"Calendars.ReadWrite"})
		elapsed := time.Since(start)

		if elapsed > 5*time.Second {
			t.Errorf("probe took %v, want < 5s (AC-8)", elapsed)
		}
		if !cred.called {
			t.Error("GetToken should be called for browser auth method")
		}
		if !preAuthCalled {
			t.Error("markPreAuthenticated should be called on successful probe")
		}
	})

	t.Run("failure_does_not_call_markPreAuthenticated", func(t *testing.T) {
		cred := &mockTokenCredential{
			getTokenErr: fmt.Errorf("authentication required: no cached token"),
		}
		preAuthCalled := false

		start := time.Now()
		probeStartupToken(cred, "auth_code", "", func() { preAuthCalled = true }, []string{"Calendars.ReadWrite"})
		elapsed := time.Since(start)

		if elapsed > 5*time.Second {
			t.Errorf("probe took %v, want < 5s (AC-8)", elapsed)
		}
		if !cred.called {
			t.Error("GetToken should be called for auth_code auth method")
		}
		if preAuthCalled {
			t.Error("markPreAuthenticated should not be called when probe fails")
		}
	})

	t.Run("device_code_skipped_no_auth_record", func(t *testing.T) {
		cred := &mockTokenCredential{}
		preAuthCalled := false

		start := time.Now()
		probeStartupToken(cred, "device_code", "/nonexistent/path", func() { preAuthCalled = true }, []string{"Calendars.ReadWrite"})
		elapsed := time.Since(start)

		if elapsed > 5*time.Second {
			t.Errorf("probe took %v, want < 5s (AC-8)", elapsed)
		}
		if cred.called {
			t.Error("GetToken should NOT be called for device_code (would trigger interactive auth)")
		}
		if preAuthCalled {
			t.Error("markPreAuthenticated should not be called when no auth record exists")
		}
	})

	t.Run("device_code_with_auth_record_marks_pre_authenticated", func(t *testing.T) {
		// Create a temporary auth record file to simulate a previous session.
		tmpDir := t.TempDir()
		authRecord := filepath.Join(tmpDir, "auth_record.json")
		if err := os.WriteFile(authRecord, []byte(`{}`), 0600); err != nil {
			t.Fatal(err)
		}

		cred := &mockTokenCredential{}
		preAuthCalled := false

		start := time.Now()
		probeStartupToken(cred, "device_code", authRecord, func() { preAuthCalled = true }, []string{"Calendars.ReadWrite"})
		elapsed := time.Since(start)

		if elapsed > 5*time.Second {
			t.Errorf("probe took %v, want < 5s (AC-8)", elapsed)
		}
		if cred.called {
			t.Error("GetToken should NOT be called for device_code")
		}
		if !preAuthCalled {
			t.Error("markPreAuthenticated should be called when auth record exists (cached tokens likely valid)")
		}
	})

	t.Run("slow_credential_respects_timeout", func(t *testing.T) {
		cred := &mockTokenCredential{
			getTokenDelay: 10 * time.Second, // longer than the 5s timeout
		}
		preAuthCalled := false

		start := time.Now()
		probeStartupToken(cred, "browser", "", func() { preAuthCalled = true }, []string{"Calendars.ReadWrite"})
		elapsed := time.Since(start)

		if elapsed > 6*time.Second {
			t.Errorf("probe took %v, want < 6s (bounded by 5s timeout + margin)", elapsed)
		}
		if !cred.called {
			t.Error("GetToken should be called even for slow credentials")
		}
		if preAuthCalled {
			t.Error("markPreAuthenticated should not be called when probe times out")
		}
	})
}
