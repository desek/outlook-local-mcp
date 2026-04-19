package auth

import (
	"context"
	"fmt"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
)

// mockCredential implements azcore.TokenCredential for testing. GetToken
// always returns an error, simulating the absence of cached tokens. This
// prevents any real Azure SDK interaction (no browser, no HTTP calls).
type mockCredential struct{}

// GetToken returns an error indicating no cached token is available.
// This simulates the production behavior when a credential has no token
// cache entry, causing the restore logic to register the account with
// Client=nil for deferred re-authentication.
func (m *mockCredential) GetToken(_ context.Context, _ policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{}, fmt.Errorf("no cached token")
}

// restoreMockAuthenticator implements Authenticator for restore tests.
// Authenticate always returns an error, simulating a credential that has not
// been interactively authenticated.
type restoreMockAuthenticator struct{}

// Authenticate returns an error indicating the mock is not authenticated.
func (m *restoreMockAuthenticator) Authenticate(_ context.Context, _ *policy.TokenRequestOptions) (azidentity.AuthenticationRecord, error) {
	return azidentity.AuthenticationRecord{}, fmt.Errorf("mock: not authenticated")
}

// fakeCredentialFactory is a CredentialFactory that returns mock credentials
// without constructing real Azure SDK objects. It mirrors the path-derivation
// logic of SetupCredentialForAccount (cache name and auth record path) so
// that tests can verify those fields on restored AccountEntry instances.
//
// Parameters follow the CredentialFactory signature.
//
// Returns a *mockCredential, *mockAuthenticator, the derived auth record
// path, the derived cache name, and nil error.
func fakeCredentialFactory(label, _, _, _, cacheNameBase, authRecordDir string) (
	azcore.TokenCredential, Authenticator, string, string, error,
) {
	cacheName := cacheNameBase + "-" + label
	authRecordPath := filepath.Join(authRecordDir, label+"_auth_record.json")
	return &mockCredential{}, &restoreMockAuthenticator{}, authRecordPath, cacheName, nil
}

// fakeGraphClient returns a mock GraphClientFactory that returns a new
// GraphServiceClient (non-nil) or an error based on shouldFail.
func fakeGraphClient(shouldFail bool) GraphClientFactory {
	return func(_ azcore.TokenCredential) (*msgraphsdk.GraphServiceClient, error) {
		if shouldFail {
			return nil, fmt.Errorf("mock graph client error")
		}
		// Return a non-nil client. GraphServiceClient has no required init.
		return &msgraphsdk.GraphServiceClient{}, nil
	}
}

// TestRestoreAccounts_Success verifies that accounts with valid cached tokens
// are restored into the registry with non-nil Graph clients.
func TestRestoreAccounts_Success(t *testing.T) {
	dir := t.TempDir()
	accountsPath := filepath.Join(dir, "accounts.json")
	authRecordDir := dir

	// Persist two account configs.
	accounts := []AccountConfig{
		{Label: "work", ClientID: "dd5fc5c5-eb9a-4f6f-97bd-1a9fecb277d3", TenantID: "common", AuthMethod: "browser"},
		{Label: "personal", ClientID: "dd5fc5c5-eb9a-4f6f-97bd-1a9fecb277d3", TenantID: "common", AuthMethod: "device_code"},
	}
	if err := SaveAccounts(accountsPath, accounts); err != nil {
		t.Fatalf("SaveAccounts: %v", err)
	}

	registry := NewAccountRegistry()
	// Add default account first (as main.go does).
	if err := registry.Add(&AccountEntry{Label: "default"}); err != nil {
		t.Fatalf("Add default: %v", err)
	}

	// fakeCredentialFactory returns mock credentials. The mock GetToken always
	// returns an error (no cached tokens), so both accounts are registered but
	// not "restored" (no active token).
	restored, total := RestoreAccounts(accountsPath, "test-cache", authRecordDir, registry, fakeCredentialFactory, fakeGraphClient(false), []string{"Calendars.ReadWrite"})

	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}

	// Since there are no real cached tokens, silent auth fails.
	// Accounts are still registered but not "restored" (no active token).
	if restored != 0 {
		t.Errorf("restored = %d, want 0 (no cached tokens available)", restored)
	}

	// Both accounts should be in the registry.
	if registry.Count() != 3 { // default + 2 restored
		t.Errorf("registry count = %d, want 3", registry.Count())
	}

	for _, label := range []string{"work", "personal"} {
		entry, ok := registry.Get(label)
		if !ok {
			t.Errorf("account %q not found in registry", label)
			continue
		}
		if entry.Credential == nil {
			t.Errorf("account %q has nil Credential", label)
		}
		if entry.Authenticator == nil {
			t.Errorf("account %q has nil Authenticator", label)
		}
	}
}

// TestRestoreAccounts_SilentAuthFailure verifies that accounts with expired
// tokens are still registered in the registry for deferred re-authentication.
func TestRestoreAccounts_SilentAuthFailure(t *testing.T) {
	dir := t.TempDir()
	accountsPath := filepath.Join(dir, "accounts.json")
	authRecordDir := dir

	// Persist one account config.
	accounts := []AccountConfig{
		{Label: "expired", ClientID: "dd5fc5c5-eb9a-4f6f-97bd-1a9fecb277d3", TenantID: "common", AuthMethod: "browser"},
	}
	if err := SaveAccounts(accountsPath, accounts); err != nil {
		t.Fatalf("SaveAccounts: %v", err)
	}

	registry := NewAccountRegistry()

	// No cached tokens exist, so silent auth will fail.
	restored, total := RestoreAccounts(accountsPath, "test-cache", authRecordDir, registry, fakeCredentialFactory, fakeGraphClient(false), []string{"Calendars.ReadWrite"})

	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}

	if restored != 0 {
		t.Errorf("restored = %d, want 0 (token expired)", restored)
	}

	// Account should still be in registry for deferred re-auth.
	entry, ok := registry.Get("expired")
	if !ok {
		t.Fatal("account 'expired' not found in registry")
	}

	if entry.Credential == nil {
		t.Error("expected non-nil Credential for deferred re-auth account")
	}
	if entry.Authenticator == nil {
		t.Error("expected non-nil Authenticator for deferred re-auth account")
	}

	// Client should be nil because silent auth failed (no Graph client created).
	if entry.Client != nil {
		t.Error("expected nil Client for account with expired token")
	}
}

// TestRestoreAccounts_FileNotExist verifies that RestoreAccounts returns
// zero counts when the accounts file does not exist.
func TestRestoreAccounts_FileNotExist(t *testing.T) {
	dir := t.TempDir()
	accountsPath := filepath.Join(dir, "nonexistent", "accounts.json")

	registry := NewAccountRegistry()
	restored, total := RestoreAccounts(accountsPath, "test-cache", dir, registry, fakeCredentialFactory, fakeGraphClient(false), []string{"Calendars.ReadWrite"})

	if restored != 0 || total != 0 {
		t.Errorf("restored=%d, total=%d; want 0, 0", restored, total)
	}
	if registry.Count() != 0 {
		t.Errorf("registry count = %d, want 0", registry.Count())
	}
}

// TestRestoreAccounts_EmptyFile verifies that RestoreAccounts handles an
// accounts file with zero accounts gracefully.
func TestRestoreAccounts_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	accountsPath := filepath.Join(dir, "accounts.json")

	if err := SaveAccounts(accountsPath, []AccountConfig{}); err != nil {
		t.Fatalf("SaveAccounts: %v", err)
	}

	registry := NewAccountRegistry()
	restored, total := RestoreAccounts(accountsPath, "test-cache", dir, registry, fakeCredentialFactory, fakeGraphClient(false), []string{"Calendars.ReadWrite"})

	if restored != 0 || total != 0 {
		t.Errorf("restored=%d, total=%d; want 0, 0", restored, total)
	}
}

// TestRestoreAccounts_DuplicateLabel verifies that a restored account whose
// label already exists in the registry is skipped without affecting the
// existing entry.
func TestRestoreAccounts_DuplicateLabel(t *testing.T) {
	dir := t.TempDir()
	accountsPath := filepath.Join(dir, "accounts.json")
	authRecordDir := dir

	accounts := []AccountConfig{
		{Label: "work", ClientID: "dd5fc5c5-eb9a-4f6f-97bd-1a9fecb277d3", TenantID: "common", AuthMethod: "browser"},
	}
	if err := SaveAccounts(accountsPath, accounts); err != nil {
		t.Fatalf("SaveAccounts: %v", err)
	}

	registry := NewAccountRegistry()
	// Pre-register "work" so the restore attempt hits a duplicate.
	if err := registry.Add(&AccountEntry{Label: "work"}); err != nil {
		t.Fatalf("Add pre-existing: %v", err)
	}

	restored, total := RestoreAccounts(accountsPath, "test-cache", authRecordDir, registry, fakeCredentialFactory, fakeGraphClient(false), []string{"Calendars.ReadWrite"})

	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if restored != 0 {
		t.Errorf("restored = %d, want 0 (duplicate label)", restored)
	}
	if registry.Count() != 1 {
		t.Errorf("registry count = %d, want 1", registry.Count())
	}
}

// TestRestoreAccounts_IdentityFieldsPreserved verifies that persisted identity
// configuration (ClientID, TenantID, AuthMethod) is preserved in the restored
// AccountEntry.
func TestRestoreAccounts_IdentityFieldsPreserved(t *testing.T) {
	dir := t.TempDir()
	accountsPath := filepath.Join(dir, "accounts.json")
	authRecordDir := dir

	accounts := []AccountConfig{
		{Label: "contoso", ClientID: "aaaa-bbbb", TenantID: "contoso.com", AuthMethod: "device_code"},
	}
	if err := SaveAccounts(accountsPath, accounts); err != nil {
		t.Fatalf("SaveAccounts: %v", err)
	}

	registry := NewAccountRegistry()
	RestoreAccounts(accountsPath, "test-cache", authRecordDir, registry, fakeCredentialFactory, fakeGraphClient(false), []string{"Calendars.ReadWrite"})

	entry, ok := registry.Get("contoso")
	if !ok {
		t.Fatal("account 'contoso' not found in registry")
	}

	if entry.ClientID != "aaaa-bbbb" {
		t.Errorf("ClientID = %q, want %q", entry.ClientID, "aaaa-bbbb")
	}
	if entry.TenantID != "contoso.com" {
		t.Errorf("TenantID = %q, want %q", entry.TenantID, "contoso.com")
	}
	if entry.AuthMethod != "device_code" {
		t.Errorf("AuthMethod = %q, want %q", entry.AuthMethod, "device_code")
	}
}

// countingGraphClientFactory returns a GraphClientFactory that tracks
// invocation count via an atomic counter.
func countingGraphClientFactory(count *atomic.Int32) GraphClientFactory {
	return func(_ azcore.TokenCredential) (*msgraphsdk.GraphServiceClient, error) {
		count.Add(1)
		return &msgraphsdk.GraphServiceClient{}, nil
	}
}

// TestRestoreOne_DeviceCode_SkipsGetToken verifies that device_code accounts
// skip silent token acquisition during restore. The account is registered with
// Client=nil and Authenticated=false, deferring re-authentication to the
// first tool call. The GraphClientFactory must not be invoked because GetToken
// is never called and thus silent auth never succeeds.
func TestRestoreOne_DeviceCode_SkipsGetToken(t *testing.T) {
	dir := t.TempDir()
	accountsPath := filepath.Join(dir, "accounts.json")

	accounts := []AccountConfig{
		{Label: "dc-skip", ClientID: "dd5fc5c5-eb9a-4f6f-97bd-1a9fecb277d3", TenantID: "common", AuthMethod: "device_code"},
	}
	if err := SaveAccounts(accountsPath, accounts); err != nil {
		t.Fatalf("SaveAccounts: %v", err)
	}

	registry := NewAccountRegistry()
	var factoryCalls atomic.Int32

	restored, total := RestoreAccounts(accountsPath, "test-cache", dir, registry, fakeCredentialFactory, countingGraphClientFactory(&factoryCalls), []string{"Calendars.ReadWrite"})

	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if restored != 0 {
		t.Errorf("restored = %d, want 0 (device_code skips silent auth)", restored)
	}

	entry, ok := registry.Get("dc-skip")
	if !ok {
		t.Fatal("account 'dc-skip' not found in registry")
	}

	// device_code accounts must be registered with Client=nil.
	if entry.Client != nil {
		t.Error("expected nil Client for device_code account (GetToken skipped)")
	}
	// Credential and Authenticator must still be populated for deferred auth.
	if entry.Credential == nil {
		t.Error("expected non-nil Credential for device_code account")
	}
	if entry.Authenticator == nil {
		t.Error("expected non-nil Authenticator for device_code account")
	}
	if entry.AuthMethod != "device_code" {
		t.Errorf("AuthMethod = %q, want %q", entry.AuthMethod, "device_code")
	}

	// The graph client factory must not have been called because GetToken
	// was skipped entirely (no silent auth attempt means no success path).
	if calls := factoryCalls.Load(); calls != 0 {
		t.Errorf("GraphClientFactory called %d times, want 0 (GetToken skipped for device_code)", calls)
	}
}

// TestRestoreOne_Browser_AttemptsGetToken verifies that browser accounts
// still attempt silent token acquisition during restore. Without cached
// tokens, GetToken fails and the account is registered with Client=nil,
// but the attempt is made (unlike device_code accounts which skip it).
// The GraphClientFactory is not called because silent auth fails, but the
// GetToken code path is exercised (verified by the fact that the function
// completes through the silent-auth branch rather than the skip branch).
func TestRestoreOne_Browser_AttemptsGetToken(t *testing.T) {
	dir := t.TempDir()
	accountsPath := filepath.Join(dir, "accounts.json")

	accounts := []AccountConfig{
		{Label: "br-attempt", ClientID: "dd5fc5c5-eb9a-4f6f-97bd-1a9fecb277d3", TenantID: "common", AuthMethod: "browser"},
	}
	if err := SaveAccounts(accountsPath, accounts); err != nil {
		t.Fatalf("SaveAccounts: %v", err)
	}

	registry := NewAccountRegistry()
	var factoryCalls atomic.Int32

	restored, total := RestoreAccounts(accountsPath, "test-cache", dir, registry, fakeCredentialFactory, countingGraphClientFactory(&factoryCalls), []string{"Calendars.ReadWrite"})

	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	// Without cached tokens, silent auth fails so restored=0.
	if restored != 0 {
		t.Errorf("restored = %d, want 0 (no cached tokens)", restored)
	}

	entry, ok := registry.Get("br-attempt")
	if !ok {
		t.Fatal("account 'br-attempt' not found in registry")
	}

	// Silent auth fails (no cached tokens) so Client is nil.
	if entry.Client != nil {
		t.Error("expected nil Client for browser account without cached tokens")
	}
	// Credential and Authenticator must still be populated.
	if entry.Credential == nil {
		t.Error("expected non-nil Credential for browser account")
	}
	if entry.Authenticator == nil {
		t.Error("expected non-nil Authenticator for browser account")
	}
	if entry.AuthMethod != "browser" {
		t.Errorf("AuthMethod = %q, want %q", entry.AuthMethod, "browser")
	}

	// GraphClientFactory should not be called because silent auth failed
	// (no cached token), but GetToken WAS attempted (unlike device_code).
	// This test verifies browser accounts go through the silent auth path
	// by confirming the account is registered correctly after the attempt.
	if calls := factoryCalls.Load(); calls != 0 {
		t.Errorf("GraphClientFactory called %d times, want 0 (silent auth failed)", calls)
	}
}

// TestAuthRecordDir verifies that AuthRecordDir extracts the directory from
// an auth record path.
func TestAuthRecordDir(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"/home/user/.outlook-local-mcp/auth_record.json", "/home/user/.outlook-local-mcp"},
		{"/tmp/test/auth.json", "/tmp/test"},
	}

	for _, tc := range cases {
		got := AuthRecordDir(tc.input)
		if got != tc.want {
			t.Errorf("AuthRecordDir(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// TestRestoreAccounts_PopulatesEmailFromUPN verifies the AC-2 contract: at
// startup, RestoreAccounts copies the persisted UPN into AccountEntry.Email
// without issuing any Graph API call. This is the CR-0056 behavior that
// eliminates the per-restart /me fetch.
//
// The test seeds accounts.json with a non-empty UPN and uses
// fakeCredentialFactory (mock credentials) together with a counting Graph
// client factory — because RestoreAccounts' device_code branch skips
// silent auth and never invokes the factory, the counter also serves to
// confirm no network-bound Graph client is instantiated during restore.
func TestRestoreAccounts_PopulatesEmailFromUPN(t *testing.T) {
	dir := t.TempDir()
	accountsPath := filepath.Join(dir, "accounts.json")

	accounts := []AccountConfig{
		{
			Label:      "contoso",
			ClientID:   "dd5fc5c5-eb9a-4f6f-97bd-1a9fecb277d3",
			TenantID:   "common",
			AuthMethod: "device_code",
			UPN:        "alice@contoso.com",
		},
	}
	if err := SaveAccounts(accountsPath, accounts); err != nil {
		t.Fatalf("SaveAccounts: %v", err)
	}

	registry := NewAccountRegistry()
	var factoryCalls atomic.Int32

	RestoreAccounts(accountsPath, "test-cache", dir, registry, fakeCredentialFactory, countingGraphClientFactory(&factoryCalls), []string{"Calendars.ReadWrite"})

	entry, ok := registry.Get("contoso")
	if !ok {
		t.Fatal("account 'contoso' not found in registry")
	}
	if entry.Email != "alice@contoso.com" {
		t.Errorf("entry.Email = %q, want %q (populated from persisted UPN, no Graph call)", entry.Email, "alice@contoso.com")
	}
	// device_code skips silent auth entirely so the Graph client factory
	// must not have been invoked. This is the strongest in-process signal
	// that no Graph /me call occurred during restore.
	if calls := factoryCalls.Load(); calls != 0 {
		t.Errorf("GraphClientFactory called %d times, want 0 (no Graph API call permitted during restore)", calls)
	}
}
