package auth

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/public"
)

// testClientID is a synthetic client ID used in tests. It does not correspond
// to a real Azure AD application registration.
const testClientID = "d3590ed6-52b3-4102-aeff-aad2292ab01c"

// TestNewAuthCodeCredential_Success verifies that NewAuthCodeCredential
// constructs a valid AuthCodeCredential without error.
func TestNewAuthCodeCredential_Success(t *testing.T) {
	cred, err := NewAuthCodeCredential(testClientID, "common")
	if err != nil {
		t.Fatalf("NewAuthCodeCredential() error: %v", err)
	}
	if cred == nil {
		t.Fatal("NewAuthCodeCredential() returned nil credential")
	}
	if cred.clientID != testClientID {
		t.Errorf("clientID = %q, want %q", cred.clientID, testClientID)
	}
	if cred.redirectURI != nativeclientRedirectURI {
		t.Errorf("redirectURI = %q, want %q", cred.redirectURI, nativeclientRedirectURI)
	}
}

// TestNewAuthCodeCredential_SpecificTenant verifies that a specific tenant ID
// (GUID format) is accepted by the MSAL public client.
func TestNewAuthCodeCredential_SpecificTenant(t *testing.T) {
	cred, err := NewAuthCodeCredential(testClientID, "00000000-0000-0000-0000-000000000001")
	if err != nil {
		t.Fatalf("NewAuthCodeCredential() error: %v", err)
	}
	if cred == nil {
		t.Fatal("NewAuthCodeCredential() returned nil credential")
	}
}

// TestAuthCodeURL_ReturnsValidURL verifies that AuthCodeURL returns a URL
// containing the expected query parameters (client_id, redirect_uri, scope).
func TestAuthCodeURL_ReturnsValidURL(t *testing.T) {
	cred, err := NewAuthCodeCredential(testClientID, "common")
	if err != nil {
		t.Fatalf("NewAuthCodeCredential() error: %v", err)
	}

	authURL, err := cred.AuthCodeURL(context.Background(), []string{"Calendars.ReadWrite"})
	if err != nil {
		t.Fatalf("AuthCodeURL() error: %v", err)
	}

	if authURL == "" {
		t.Fatal("AuthCodeURL() returned empty string")
	}

	// Verify the URL contains expected parameters.
	checks := []struct {
		name   string
		substr string
	}{
		{"client_id", "client_id=" + testClientID},
		{"redirect_uri", "redirect_uri="},
		{"scope", "scope="},
		{"response_type", "response_type=code"},
		{"code_challenge", "code_challenge="},
		{"code_challenge_method", "code_challenge_method=S256"},
	}
	for _, check := range checks {
		if !containsSubstring(authURL, check.substr) {
			t.Errorf("AuthCodeURL() missing %s: URL = %s", check.name, authURL)
		}
	}
}

// TestExchangeCode_InvalidPrefix verifies that ExchangeCode rejects redirect
// URLs that do not start with the nativeclient redirect URI.
func TestExchangeCode_InvalidPrefix(t *testing.T) {
	cred, err := NewAuthCodeCredential(testClientID, "common")
	if err != nil {
		t.Fatalf("NewAuthCodeCredential() error: %v", err)
	}

	tests := []struct {
		name string
		url  string
	}{
		{"evil domain", "https://evil.com?code=abc123"},
		{"http instead of https", "http://login.microsoftonline.com/common/oauth2/nativeclient?code=abc"},
		{"partial match", "https://login.microsoftonline.com/common/oauth2/native?code=abc"},
		{"empty string", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cred.ExchangeCode(context.Background(), tt.url, []string{"Calendars.ReadWrite"})
			if err == nil {
				t.Error("ExchangeCode() expected error for invalid prefix, got nil")
			}
		})
	}
}

// TestExchangeCode_MissingCode verifies that ExchangeCode rejects redirect
// URLs that start with the correct prefix but do not contain a "code" query
// parameter.
func TestExchangeCode_MissingCode(t *testing.T) {
	cred, err := NewAuthCodeCredential(testClientID, "common")
	if err != nil {
		t.Fatalf("NewAuthCodeCredential() error: %v", err)
	}

	tests := []struct {
		name string
		url  string
	}{
		{"no query params", nativeclientRedirectURI},
		{"state only", nativeclientRedirectURI + "?state=xyz"},
		{"empty code", nativeclientRedirectURI + "?code="},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cred.ExchangeCode(context.Background(), tt.url, []string{"Calendars.ReadWrite"})
			if err == nil {
				t.Error("ExchangeCode() expected error for missing code, got nil")
			}
		})
	}
}

// TestExchangeCode_MalformedURL verifies that ExchangeCode rejects URLs that
// cannot be parsed.
func TestExchangeCode_MalformedURL(t *testing.T) {
	cred, err := NewAuthCodeCredential(testClientID, "common")
	if err != nil {
		t.Fatalf("NewAuthCodeCredential() error: %v", err)
	}

	// A URL starting with nativeclient prefix but with invalid characters
	// that make query parsing yield no code. The prefix check passes but
	// the code extraction returns empty.
	malformed := nativeclientRedirectURI + "?%invalid"
	err = cred.ExchangeCode(context.Background(), malformed, []string{"Calendars.ReadWrite"})
	if err == nil {
		t.Error("ExchangeCode() expected error for malformed URL, got nil")
	}
}

// TestGetToken_NoAccount_ReturnsAuthError verifies that GetToken returns an
// error recognized by IsAuthError when no account has been cached (i.e.,
// before ExchangeCode has been called).
func TestGetToken_NoAccount_ReturnsAuthError(t *testing.T) {
	cred, err := NewAuthCodeCredential(testClientID, "common")
	if err != nil {
		t.Fatalf("NewAuthCodeCredential() error: %v", err)
	}

	_, err = cred.GetToken(context.Background(), policy.TokenRequestOptions{
		Scopes: []string{"Calendars.ReadWrite"},
	})
	if err == nil {
		t.Fatal("GetToken() expected error when no account cached, got nil")
	}

	// The error message must contain "authentication required" to be
	// recognized by IsAuthError.
	if !IsAuthError(err) {
		t.Errorf("GetToken() error not recognized by IsAuthError: %v", err)
	}
}

// TestAuthenticate_ReturnsAuthError verifies that Authenticate returns an
// error (since auth_code uses AuthCodeURL + ExchangeCode instead) and that
// the error is recognized by IsAuthError.
func TestAuthenticate_ReturnsAuthError(t *testing.T) {
	cred, err := NewAuthCodeCredential(testClientID, "common")
	if err != nil {
		t.Fatalf("NewAuthCodeCredential() error: %v", err)
	}

	record, err := cred.Authenticate(context.Background(), &policy.TokenRequestOptions{
		Scopes: []string{"Calendars.ReadWrite"},
	})
	if err == nil {
		t.Fatal("Authenticate() expected error for auth_code credential, got nil")
	}
	if record != (azidentity.AuthenticationRecord{}) {
		t.Errorf("Authenticate() returned non-zero record: %+v", record)
	}
	if !IsAuthError(err) {
		t.Errorf("Authenticate() error not recognized by IsAuthError: %v", err)
	}
}

// TestAuthCodeCredential_InterfaceCompliance verifies at compile time that
// AuthCodeCredential satisfies the expected interfaces.
func TestAuthCodeCredential_InterfaceCompliance(t *testing.T) {
	cred, err := NewAuthCodeCredential(testClientID, "common")
	if err != nil {
		t.Fatalf("NewAuthCodeCredential() error: %v", err)
	}

	var _ azcore.TokenCredential = cred
	var _ Authenticator = cred
	var _ AuthCodeFlow = cred
}

// TestSetAccount_And_Account verifies that SetAccount stores the account and
// Account retrieves it correctly.
func TestSetAccount_And_Account(t *testing.T) {
	cred, err := NewAuthCodeCredential(testClientID, "common")
	if err != nil {
		t.Fatalf("NewAuthCodeCredential() error: %v", err)
	}

	// Initially no account.
	_, hasAcct := cred.Account()
	if hasAcct {
		t.Error("expected no account before SetAccount")
	}

	// Set an account.
	want := public.Account{
		HomeAccountID: "test-home-id",
		Environment:   "login.microsoftonline.com",
		Realm:         "common",
	}
	cred.SetAccount(want)

	got, hasAcct := cred.Account()
	if !hasAcct {
		t.Error("expected account after SetAccount")
	}
	if got.HomeAccountID != want.HomeAccountID {
		t.Errorf("HomeAccountID = %q, want %q", got.HomeAccountID, want.HomeAccountID)
	}
	if got.Environment != want.Environment {
		t.Errorf("Environment = %q, want %q", got.Environment, want.Environment)
	}
	if got.Realm != want.Realm {
		t.Errorf("Realm = %q, want %q", got.Realm, want.Realm)
	}
}

// containsSubstring reports whether s contains substr.
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (substr == "" || findSubstring(s, substr))
}

// findSubstring performs a simple substring search.
func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestSaveLoadAuthCodeAccount_RoundTrip verifies that saving and loading an
// account produces an identical result.
func TestSaveLoadAuthCodeAccount_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "account.json")

	want := public.Account{
		HomeAccountID:     "uid.utid",
		Environment:       "login.microsoftonline.com",
		Realm:             "contoso.com",
		PreferredUsername: "user@contoso.com",
	}

	if err := saveAuthCodeAccount(path, want); err != nil {
		t.Fatalf("saveAuthCodeAccount() error: %v", err)
	}

	got, found, err := loadAuthCodeAccount(path)
	if err != nil {
		t.Fatalf("loadAuthCodeAccount() error: %v", err)
	}
	if !found {
		t.Fatal("loadAuthCodeAccount() found = false, want true")
	}
	if got.HomeAccountID != want.HomeAccountID {
		t.Errorf("HomeAccountID = %q, want %q", got.HomeAccountID, want.HomeAccountID)
	}
	if got.Environment != want.Environment {
		t.Errorf("Environment = %q, want %q", got.Environment, want.Environment)
	}
	if got.Realm != want.Realm {
		t.Errorf("Realm = %q, want %q", got.Realm, want.Realm)
	}
	if got.PreferredUsername != want.PreferredUsername {
		t.Errorf("PreferredUsername = %q, want %q", got.PreferredUsername, want.PreferredUsername)
	}
}

// TestSaveAuthCodeAccount_FilePermissions verifies that the saved account file
// has 0600 permissions (owner read/write only).
func TestSaveAuthCodeAccount_FilePermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "account.json")

	account := public.Account{HomeAccountID: "test"}
	if err := saveAuthCodeAccount(path, account); err != nil {
		t.Fatalf("saveAuthCodeAccount() error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("os.Stat() error: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("file permissions = %04o, want 0600", perm)
	}
}

// TestLoadAuthCodeAccount_FileNotFound verifies that a missing file returns a
// zero-value account with found=false and no error.
func TestLoadAuthCodeAccount_FileNotFound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.json")

	account, found, err := loadAuthCodeAccount(path)
	if err != nil {
		t.Fatalf("loadAuthCodeAccount() error: %v", err)
	}
	if found {
		t.Error("loadAuthCodeAccount() found = true, want false")
	}
	if account.HomeAccountID != "" {
		t.Errorf("loadAuthCodeAccount() returned non-zero HomeAccountID: %q", account.HomeAccountID)
	}
}

// TestLoadAuthCodeAccount_InvalidJSON verifies that a corrupt file returns a
// zero-value account with found=false and no error.
func TestLoadAuthCodeAccount_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "corrupt.json")

	if err := os.WriteFile(path, []byte("{not valid json"), 0600); err != nil {
		t.Fatalf("os.WriteFile() error: %v", err)
	}

	account, found, err := loadAuthCodeAccount(path)
	if err != nil {
		t.Fatalf("loadAuthCodeAccount() error: %v", err)
	}
	if found {
		t.Error("loadAuthCodeAccount() found = true, want false")
	}
	if account.HomeAccountID != "" {
		t.Errorf("loadAuthCodeAccount() returned non-zero HomeAccountID: %q", account.HomeAccountID)
	}
}

// TestLoadPersistedAccount_RestoresAccount verifies that LoadPersistedAccount
// restores a previously persisted account into the credential.
func TestLoadPersistedAccount_RestoresAccount(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "account.json")

	want := public.Account{
		HomeAccountID:     "uid.utid",
		Environment:       "login.microsoftonline.com",
		Realm:             "common",
		PreferredUsername: "test@example.com",
	}

	if err := saveAuthCodeAccount(path, want); err != nil {
		t.Fatalf("saveAuthCodeAccount() error: %v", err)
	}

	cred, err := NewAuthCodeCredential(testClientID, "common")
	if err != nil {
		t.Fatalf("NewAuthCodeCredential() error: %v", err)
	}

	if err := cred.LoadPersistedAccount(path); err != nil {
		t.Fatalf("LoadPersistedAccount() error: %v", err)
	}

	got, hasAcct := cred.Account()
	if !hasAcct {
		t.Fatal("expected account after LoadPersistedAccount")
	}
	if got.HomeAccountID != want.HomeAccountID {
		t.Errorf("HomeAccountID = %q, want %q", got.HomeAccountID, want.HomeAccountID)
	}
}

// TestPersistAccount_SavesAccount verifies that PersistAccount writes the
// credential's current account to disk.
func TestPersistAccount_SavesAccount(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "account.json")

	cred, err := NewAuthCodeCredential(testClientID, "common")
	if err != nil {
		t.Fatalf("NewAuthCodeCredential() error: %v", err)
	}

	want := public.Account{
		HomeAccountID: "uid.utid",
		Environment:   "login.microsoftonline.com",
		Realm:         "common",
	}
	cred.SetAccount(want)

	if err := cred.PersistAccount(path); err != nil {
		t.Fatalf("PersistAccount() error: %v", err)
	}

	got, found, err := loadAuthCodeAccount(path)
	if err != nil {
		t.Fatalf("loadAuthCodeAccount() error: %v", err)
	}
	if !found {
		t.Fatal("loadAuthCodeAccount() found = false, want true")
	}
	if got.HomeAccountID != want.HomeAccountID {
		t.Errorf("HomeAccountID = %q, want %q", got.HomeAccountID, want.HomeAccountID)
	}
}

// TestPersistAccount_NoAccount_IsNoop verifies that PersistAccount is a no-op
// when no account has been set on the credential.
func TestPersistAccount_NoAccount_IsNoop(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "account.json")

	cred, err := NewAuthCodeCredential(testClientID, "common")
	if err != nil {
		t.Fatalf("NewAuthCodeCredential() error: %v", err)
	}

	if err := cred.PersistAccount(path); err != nil {
		t.Fatalf("PersistAccount() error: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("PersistAccount() created file when no account was set")
	}
}

// TestInitMSALCache_ReturnsNonNil verifies that InitMSALCache returns a
// non-nil cache accessor on platforms with keychain support. On platforms
// where the keychain is unavailable, it returns nil (fallback behavior).
func TestInitMSALCache_Fallback(t *testing.T) {
	// InitMSALCache should not panic regardless of platform support.
	// The return value depends on OS keychain availability.
	cache := InitMSALCache("outlook-mcp-test-cache", "auto")
	// On CI or headless environments, cache may be nil (fallback).
	// On developer machines with keychain, cache is non-nil.
	// We only verify no panic and the function returns.
	_ = cache
}

// TestNewAuthCodeCredential_WithCacheAccessor verifies that the WithCacheAccessor
// option does not cause construction to fail.
func TestNewAuthCodeCredential_WithCacheAccessor(t *testing.T) {
	// Use nil cache accessor (simulates fallback).
	cred, err := NewAuthCodeCredential(testClientID, "common", WithCacheAccessor(nil))
	if err != nil {
		t.Fatalf("NewAuthCodeCredential() with nil cache error: %v", err)
	}
	if cred == nil {
		t.Fatal("NewAuthCodeCredential() returned nil credential")
	}
}

// TestSaveAuthCodeAccount_CreatesDirectory verifies that saveAuthCodeAccount
// creates the parent directory with 0700 permissions if it does not exist.
func TestSaveAuthCodeAccount_CreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	path := filepath.Join(dir, "account.json")

	account := public.Account{HomeAccountID: "test"}
	if err := saveAuthCodeAccount(path, account); err != nil {
		t.Fatalf("saveAuthCodeAccount() error: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("os.Stat() error: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory to be created")
	}
	perm := info.Mode().Perm()
	if perm != 0700 {
		t.Errorf("directory permissions = %04o, want 0700", perm)
	}
}
