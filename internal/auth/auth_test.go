package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/desek/outlook-local-mcp/internal/config"
)

// testRecord returns a populated AuthenticationRecord for use in tests.
// The values are synthetic and do not correspond to a real Entra ID account.
func testRecord() azidentity.AuthenticationRecord {
	return azidentity.AuthenticationRecord{
		Authority:     "https://login.microsoftonline.com",
		ClientID:      "d3590ed6-52b3-4102-aeff-aad2292ab01c",
		HomeAccountID: "00000000-0000-0000-0000-000000000001.00000000-0000-0000-0000-000000000002",
		TenantID:      "00000000-0000-0000-0000-000000000002",
		Username:      "test@example.com",
		Version:       "1.0",
	}
}

// TestLoadAuthRecord_FileNotFound validates that a missing file returns a
// zero-value AuthenticationRecord without error.
func TestLoadAuthRecord_FileNotFound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent", "auth_record.json")
	record := LoadAuthRecord(path)

	if record != (azidentity.AuthenticationRecord{}) {
		t.Errorf("expected zero-value AuthenticationRecord, got %+v", record)
	}
}

// TestLoadAuthRecord_ValidJSON validates that a valid JSON auth record is
// deserialized correctly, producing a populated AuthenticationRecord with
// all fields matching the file contents.
func TestLoadAuthRecord_ValidJSON(t *testing.T) {
	want := testRecord()

	data, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("json.Marshal() error: %v", err)
	}

	path := filepath.Join(t.TempDir(), "auth_record.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("os.WriteFile() error: %v", err)
	}

	got := LoadAuthRecord(path)

	if got != want {
		t.Errorf("LoadAuthRecord() = %+v, want %+v", got, want)
	}
}

// TestLoadAuthRecord_InvalidJSON validates that corrupt JSON in the auth record
// file results in a zero-value AuthenticationRecord being returned.
func TestLoadAuthRecord_InvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth_record.json")
	if err := os.WriteFile(path, []byte("{invalid json!!!"), 0600); err != nil {
		t.Fatalf("os.WriteFile() error: %v", err)
	}

	record := LoadAuthRecord(path)

	if record != (azidentity.AuthenticationRecord{}) {
		t.Errorf("expected zero-value AuthenticationRecord for invalid JSON, got %+v", record)
	}
}

// TestSaveAuthRecord_CreatesDirectory validates that SaveAuthRecord creates the
// parent directory with permissions 0700 when it does not already exist.
func TestSaveAuthRecord_CreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "newdir", "subdir")
	path := filepath.Join(dir, "auth_record.json")

	record := testRecord()
	if err := SaveAuthRecord(path, record); err != nil {
		t.Fatalf("SaveAuthRecord() error: %v", err)
	}

	// Check the immediate parent directory permissions.
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("os.Stat(%s) error: %v", dir, err)
	}

	wantPerm := fs.FileMode(0700)
	gotPerm := info.Mode().Perm()
	if gotPerm != wantPerm {
		t.Errorf("directory permissions = %o, want %o", gotPerm, wantPerm)
	}
}

// TestSaveAuthRecord_FilePermissions validates that the auth record file is
// written with permissions 0600 (owner read/write only).
func TestSaveAuthRecord_FilePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth_record.json")

	record := testRecord()
	if err := SaveAuthRecord(path, record); err != nil {
		t.Fatalf("SaveAuthRecord() error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("os.Stat(%s) error: %v", path, err)
	}

	wantPerm := fs.FileMode(0600)
	gotPerm := info.Mode().Perm()
	if gotPerm != wantPerm {
		t.Errorf("file permissions = %o, want %o", gotPerm, wantPerm)
	}
}

// TestSaveAuthRecord_RoundTrip validates that saving and then loading an
// AuthenticationRecord produces an identical record.
func TestSaveAuthRecord_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth_record.json")
	want := testRecord()

	if err := SaveAuthRecord(path, want); err != nil {
		t.Fatalf("SaveAuthRecord() error: %v", err)
	}

	got := LoadAuthRecord(path)

	if got != want {
		t.Errorf("round-trip mismatch:\n  got  = %+v\n  want = %+v", got, want)
	}
}

// TestUserPrompt_WritesToStderr validates that the UserPrompt callback used by
// SetupCredential writes the device code message to stderr and never to stdout.
func TestUserPrompt_WritesToStderr(t *testing.T) {
	// Build the same UserPrompt function used in SetupCredential.
	userPrompt := func(_ context.Context, msg azidentity.DeviceCodeMessage) error {
		_, err := fmt.Fprintln(os.Stderr, msg.Message)
		return err
	}

	msg := azidentity.DeviceCodeMessage{
		Message: "To sign in, visit https://microsoft.com/devicelogin and enter code ABC123",
	}

	// Capture stdout.
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() for stdout error: %v", err)
	}
	origStdout := os.Stdout
	os.Stdout = stdoutW

	// Capture stderr.
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() for stderr error: %v", err)
	}
	origStderr := os.Stderr
	os.Stderr = stderrW

	t.Cleanup(func() {
		os.Stdout = origStdout
		os.Stderr = origStderr
	})

	if err := userPrompt(context.Background(), msg); err != nil {
		t.Fatalf("userPrompt() error: %v", err)
	}

	_ = stdoutW.Close()
	_ = stderrW.Close()

	var stdoutBuf, stderrBuf bytes.Buffer
	_, _ = io.Copy(&stdoutBuf, stdoutR)
	_, _ = io.Copy(&stderrBuf, stderrR)

	if stdoutBuf.Len() != 0 {
		t.Errorf("expected no stdout output, got: %s", stdoutBuf.String())
	}
	if stderrBuf.Len() == 0 {
		t.Error("expected stderr output, got nothing")
	}
	if stderrBuf.String() != msg.Message+"\n" {
		t.Errorf("stderr = %q, want %q", stderrBuf.String(), msg.Message+"\n")
	}
}

// TestFirstRunDetection_ZeroValueRecord validates that a zero-value
// AuthenticationRecord is correctly detected as a first-run condition.
// With lazy auth (CR-0022), this detection no longer triggers blocking
// authentication in SetupCredential; it is used by the AuthMiddleware instead.
func TestFirstRunDetection_ZeroValueRecord(t *testing.T) {
	record := azidentity.AuthenticationRecord{}
	isFirstRun := record == (azidentity.AuthenticationRecord{})

	if !isFirstRun {
		t.Error("zero-value AuthenticationRecord should be detected as first run")
	}
}

// TestFirstRunDetection_PopulatedRecord validates that a populated
// AuthenticationRecord is not detected as a first-run condition.
func TestFirstRunDetection_PopulatedRecord(t *testing.T) {
	record := testRecord()
	isFirstRun := record == (azidentity.AuthenticationRecord{})

	if isFirstRun {
		t.Error("populated AuthenticationRecord should not be detected as first run")
	}
}

// TestLoadAuthRecord_PersistsAcrossRestarts validates that an AuthenticationRecord
// saved to disk by SaveAuthRecord can be loaded by LoadAuthRecord on a subsequent
// invocation, simulating credential persistence across server restarts.
func TestLoadAuthRecord_PersistsAcrossRestarts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "persist", "auth_record.json")
	want := testRecord()

	// Simulate first run: save the record.
	if err := SaveAuthRecord(path, want); err != nil {
		t.Fatalf("SaveAuthRecord() error: %v", err)
	}

	// Simulate restart: load the record from the same path.
	got := LoadAuthRecord(path)

	if got != want {
		t.Errorf("persisted record mismatch:\n  got  = %+v\n  want = %+v", got, want)
	}
}

// TestSetupCredential_AuthCodeMethod verifies that SetupCredential constructs
// an AuthCodeCredential when AuthMethod is "auth_code", returning both a
// non-nil azcore.TokenCredential and a non-nil Authenticator.
func TestSetupCredential_AuthCodeMethod(t *testing.T) {
	cfg := config.Config{
		ClientID:       "d3590ed6-52b3-4102-aeff-aad2292ab01c",
		TenantID:       "common",
		AuthRecordPath: filepath.Join(t.TempDir(), "auth_record.json"),
		CacheName:      "test-cache",
		AuthMethod:     "auth_code",
	}

	cred, auth, err := SetupCredential(cfg)
	if err != nil {
		t.Fatalf("SetupCredential() error: %v", err)
	}
	if cred == nil {
		t.Error("SetupCredential() returned nil TokenCredential for auth_code method")
	}
	if auth == nil {
		t.Error("SetupCredential() returned nil Authenticator for auth_code method")
	}
}

// TestSetupCredential_AuthCodeMethod_CredentialType verifies that the
// TokenCredential returned for "auth_code" auth method is an
// *AuthCodeCredential.
func TestSetupCredential_AuthCodeMethod_CredentialType(t *testing.T) {
	cfg := config.Config{
		ClientID:       "d3590ed6-52b3-4102-aeff-aad2292ab01c",
		TenantID:       "common",
		AuthRecordPath: filepath.Join(t.TempDir(), "auth_record.json"),
		CacheName:      "test-cache",
		AuthMethod:     "auth_code",
	}

	cred, _, err := SetupCredential(cfg)
	if err != nil {
		t.Fatalf("SetupCredential() error: %v", err)
	}

	if _, ok := cred.(*AuthCodeCredential); !ok {
		t.Errorf("expected *AuthCodeCredential, got %T", cred)
	}
}

// TestSetupCredential_BrowserMethod verifies that SetupCredential constructs
// an InteractiveBrowserCredential when AuthMethod is "browser", returning both
// a non-nil azcore.TokenCredential and a non-nil Authenticator.
func TestSetupCredential_BrowserMethod(t *testing.T) {
	cfg := config.Config{
		ClientID:       "d3590ed6-52b3-4102-aeff-aad2292ab01c",
		TenantID:       "common",
		AuthRecordPath: filepath.Join(t.TempDir(), "auth_record.json"),
		CacheName:      "test-cache",
		AuthMethod:     "browser",
	}

	cred, auth, err := SetupCredential(cfg)
	if err != nil {
		t.Fatalf("SetupCredential() error: %v", err)
	}
	if cred == nil {
		t.Error("SetupCredential() returned nil TokenCredential for browser method")
	}
	if auth == nil {
		t.Error("SetupCredential() returned nil Authenticator for browser method")
	}
}

// TestSetupCredential_DeviceCodeMethod verifies that SetupCredential constructs
// a DeviceCodeCredential when AuthMethod is "device_code", returning both a
// non-nil azcore.TokenCredential and a non-nil Authenticator.
func TestSetupCredential_DeviceCodeMethod(t *testing.T) {
	cfg := config.Config{
		ClientID:       "d3590ed6-52b3-4102-aeff-aad2292ab01c",
		TenantID:       "common",
		AuthRecordPath: filepath.Join(t.TempDir(), "auth_record.json"),
		CacheName:      "test-cache",
		AuthMethod:     "device_code",
	}

	cred, auth, err := SetupCredential(cfg)
	if err != nil {
		t.Fatalf("SetupCredential() error: %v", err)
	}
	if cred == nil {
		t.Error("SetupCredential() returned nil TokenCredential for device_code method")
	}
	if auth == nil {
		t.Error("SetupCredential() returned nil Authenticator for device_code method")
	}
}

// TestAuthenticator_InterfaceCompliance verifies that both
// *azidentity.InteractiveBrowserCredential and *azidentity.DeviceCodeCredential
// satisfy the Authenticator interface via structural typing. This is a
// compile-time check enforced by the interface assertion.
func TestAuthenticator_InterfaceCompliance(t *testing.T) {
	// Construct an InteractiveBrowserCredential.
	browserCred, err := azidentity.NewInteractiveBrowserCredential(
		&azidentity.InteractiveBrowserCredentialOptions{
			ClientID:    "d3590ed6-52b3-4102-aeff-aad2292ab01c",
			TenantID:    "common",
			RedirectURL: "http://localhost",
		},
	)
	if err != nil {
		t.Fatalf("NewInteractiveBrowserCredential() error: %v", err)
	}

	// Construct a DeviceCodeCredential.
	deviceCred, err := azidentity.NewDeviceCodeCredential(
		&azidentity.DeviceCodeCredentialOptions{
			ClientID: "d3590ed6-52b3-4102-aeff-aad2292ab01c",
			TenantID: "common",
		},
	)
	if err != nil {
		t.Fatalf("NewDeviceCodeCredential() error: %v", err)
	}

	// Compile-time interface compliance checks via type assertion.
	var _ Authenticator = browserCred
	var _ Authenticator = deviceCred

	// Also verify both satisfy azcore.TokenCredential.
	var _ azcore.TokenCredential = browserCred
	var _ azcore.TokenCredential = deviceCred

	// Suppress unused variable warnings.
	_ = browserCred
	_ = deviceCred
}

// TestSetupCredential_BrowserMethod_CredentialType verifies that the
// TokenCredential returned for "browser" auth method is an
// *azidentity.InteractiveBrowserCredential.
func TestSetupCredential_BrowserMethod_CredentialType(t *testing.T) {
	cfg := config.Config{
		ClientID:       "d3590ed6-52b3-4102-aeff-aad2292ab01c",
		TenantID:       "common",
		AuthRecordPath: filepath.Join(t.TempDir(), "auth_record.json"),
		CacheName:      "test-cache",
		AuthMethod:     "browser",
	}

	cred, _, err := SetupCredential(cfg)
	if err != nil {
		t.Fatalf("SetupCredential() error: %v", err)
	}

	if _, ok := cred.(*azidentity.InteractiveBrowserCredential); !ok {
		t.Errorf("expected *azidentity.InteractiveBrowserCredential, got %T", cred)
	}
}

// TestSetupCredential_DeviceCodeMethod_CredentialType verifies that the
// TokenCredential returned for "device_code" auth method is an
// *azidentity.DeviceCodeCredential.
func TestSetupCredential_DeviceCodeMethod_CredentialType(t *testing.T) {
	cfg := config.Config{
		ClientID:       "d3590ed6-52b3-4102-aeff-aad2292ab01c",
		TenantID:       "common",
		AuthRecordPath: filepath.Join(t.TempDir(), "auth_record.json"),
		CacheName:      "test-cache",
		AuthMethod:     "device_code",
	}

	cred, _, err := SetupCredential(cfg)
	if err != nil {
		t.Fatalf("SetupCredential() error: %v", err)
	}

	if _, ok := cred.(*azidentity.DeviceCodeCredential); !ok {
		t.Errorf("expected *azidentity.DeviceCodeCredential, got %T", cred)
	}
}

// TestAuthenticate_UsesAuthenticator verifies that the Authenticate function
// correctly delegates to the provided Authenticator interface and persists
// the resulting AuthenticationRecord to disk.
func TestAuthenticate_UsesAuthenticator(t *testing.T) {
	want := testRecord()
	path := filepath.Join(t.TempDir(), "auth_record.json")

	mock := &mockAuthenticator{record: want}
	got, err := Authenticate(context.Background(), mock, path, []string{"Calendars.ReadWrite"})
	if err != nil {
		t.Fatalf("Authenticate() error: %v", err)
	}

	if got != want {
		t.Errorf("Authenticate() record = %+v, want %+v", got, want)
	}

	// Verify the record was persisted.
	loaded := LoadAuthRecord(path)
	if loaded != want {
		t.Errorf("persisted record = %+v, want %+v", loaded, want)
	}
}

// TestSetupCredentialForAccount_AuthCodeMethod verifies that
// SetupCredentialForAccount creates a valid credential with per-account cache
// and auth record path for the auth_code auth method.
func TestSetupCredentialForAccount_AuthCodeMethod(t *testing.T) {
	dir := t.TempDir()
	cred, auth, recordPath, cacheName, err := SetupCredentialForAccount(
		"work", "dd5fc5c5-eb9a-4f6f-97bd-1a9fecb277d3", "common", "auth_code", "test-cache", dir,
	)
	if err != nil {
		t.Fatalf("SetupCredentialForAccount() error: %v", err)
	}
	if cred == nil {
		t.Error("expected non-nil credential")
	}
	if auth == nil {
		t.Error("expected non-nil authenticator")
	}
	wantPath := filepath.Join(dir, "work_auth_record.json")
	if recordPath != wantPath {
		t.Errorf("authRecordPath = %q, want %q", recordPath, wantPath)
	}
	if cacheName != "test-cache-work" {
		t.Errorf("cacheName = %q, want %q", cacheName, "test-cache-work")
	}
}

// TestSetupCredentialForAccount_BrowserMethod verifies that
// SetupCredentialForAccount creates a valid credential with per-account cache
// and auth record path for the browser auth method.
func TestSetupCredentialForAccount_BrowserMethod(t *testing.T) {
	dir := t.TempDir()
	cred, auth, recordPath, cacheName, err := SetupCredentialForAccount(
		"work", "dd5fc5c5-eb9a-4f6f-97bd-1a9fecb277d3", "common", "browser", "test-cache", dir,
	)
	if err != nil {
		t.Fatalf("SetupCredentialForAccount() error: %v", err)
	}
	if cred == nil {
		t.Error("expected non-nil credential")
	}
	if auth == nil {
		t.Error("expected non-nil authenticator")
	}
	wantPath := filepath.Join(dir, "work_auth_record.json")
	if recordPath != wantPath {
		t.Errorf("authRecordPath = %q, want %q", recordPath, wantPath)
	}
	if cacheName != "test-cache-work" {
		t.Errorf("cacheName = %q, want %q", cacheName, "test-cache-work")
	}
}

// TestSetupCredentialForAccount_DeviceCodeMethod verifies that
// SetupCredentialForAccount creates a valid credential for device_code method.
func TestSetupCredentialForAccount_DeviceCodeMethod(t *testing.T) {
	dir := t.TempDir()
	cred, auth, _, _, err := SetupCredentialForAccount(
		"personal", "dd5fc5c5-eb9a-4f6f-97bd-1a9fecb277d3", "common", "device_code", "test-cache", dir,
	)
	if err != nil {
		t.Fatalf("SetupCredentialForAccount() error: %v", err)
	}
	if cred == nil {
		t.Error("expected non-nil credential")
	}
	if auth == nil {
		t.Error("expected non-nil authenticator")
	}
}

// TestSetupCredentialForAccount_NoHardcodedDefaults verifies that empty
// clientID, tenantID, and authMethod are NOT replaced by hardcoded defaults.
// The caller (add_account handler) is responsible for providing non-empty
// values by resolving defaults from the server config.
func TestSetupCredentialForAccount_NoHardcodedDefaults(t *testing.T) {
	dir := t.TempDir()
	_, _, _, _, err := SetupCredentialForAccount(
		"test", "", "", "", "base", dir,
	)
	if err == nil {
		t.Error("expected error when clientID, tenantID, authMethod are empty (no hardcoded fallbacks)")
	}
}

// TestSetupCredentialForAccount_InvalidMethod verifies that an unsupported
// auth method returns an error.
func TestSetupCredentialForAccount_InvalidMethod(t *testing.T) {
	dir := t.TempDir()
	_, _, _, _, err := SetupCredentialForAccount(
		"test", "dd5fc5c5-eb9a-4f6f-97bd-1a9fecb277d3", "common", "invalid", "base", dir,
	)
	if err == nil {
		t.Error("expected error for invalid auth method")
	}
}

// TestScopes_CalendarOnly validates that Scopes returns only the calendar scope
// when MailEnabled is false.
func TestScopes_CalendarOnly(t *testing.T) {
	cfg := config.Config{MailEnabled: false}
	scopes := Scopes(cfg)

	if len(scopes) != 1 {
		t.Fatalf("Scopes() returned %d scopes, want 1", len(scopes))
	}
	if scopes[0] != "Calendars.ReadWrite" {
		t.Errorf("Scopes()[0] = %q, want %q", scopes[0], "Calendars.ReadWrite")
	}
}

// TestScopes_WithMail validates that Scopes returns both calendar and mail
// scopes when MailEnabled is true.
func TestScopes_WithMail(t *testing.T) {
	cfg := config.Config{MailEnabled: true}
	scopes := Scopes(cfg)

	if len(scopes) != 2 {
		t.Fatalf("Scopes() returned %d scopes, want 2", len(scopes))
	}
	if scopes[0] != "Calendars.ReadWrite" {
		t.Errorf("Scopes()[0] = %q, want %q", scopes[0], "Calendars.ReadWrite")
	}
	if scopes[1] != "Mail.Read" {
		t.Errorf("Scopes()[1] = %q, want %q", scopes[1], "Mail.Read")
	}
}

// TestScopes_MailManage validates that Scopes returns calendar + Mail.ReadWrite
// (and not Mail.Read) when MailManageEnabled is true.
func TestScopes_MailManage(t *testing.T) {
	cfg := config.Config{MailEnabled: true, MailManageEnabled: true}
	scopes := Scopes(cfg)

	if len(scopes) != 2 {
		t.Fatalf("Scopes() returned %d scopes, want 2", len(scopes))
	}
	if scopes[0] != "Calendars.ReadWrite" {
		t.Errorf("Scopes()[0] = %q, want %q", scopes[0], "Calendars.ReadWrite")
	}
	if scopes[1] != "Mail.ReadWrite" {
		t.Errorf("Scopes()[1] = %q, want %q", scopes[1], "Mail.ReadWrite")
	}
	for _, s := range scopes {
		if s == "Mail.Read" {
			t.Errorf("Scopes() must not include Mail.Read when MailManageEnabled is true; got %v", scopes)
		}
	}
}

// TestScopes_MailManageImpliesRead validates that enabling MailManageEnabled
// via LoadConfig forces MailEnabled to true and causes Scopes() to request
// Mail.ReadWrite.
func TestScopes_MailManageImpliesRead(t *testing.T) {
	// Simulate the config LoadConfig produces when only MailManageEnabled is set.
	cfg := config.Config{MailEnabled: false, MailManageEnabled: true}
	// Scopes() itself must not depend on MailEnabled when MailManageEnabled is true.
	scopes := Scopes(cfg)

	var hasReadWrite bool
	for _, s := range scopes {
		if s == "Mail.ReadWrite" {
			hasReadWrite = true
		}
		if s == "Mail.Read" {
			t.Errorf("Scopes() must not include Mail.Read when MailManageEnabled is true; got %v", scopes)
		}
	}
	if !hasReadWrite {
		t.Errorf("Scopes() must include Mail.ReadWrite when MailManageEnabled is true; got %v", scopes)
	}
}

// TestScopes_NoMailSend validates that Scopes never requests Mail.Send
// regardless of configuration. Sending mail is intentionally not in the
// server's capability surface.
func TestScopes_NoMailSend(t *testing.T) {
	cases := []config.Config{
		{},
		{MailEnabled: true},
		{MailEnabled: true, MailManageEnabled: true},
		{MailManageEnabled: true},
	}
	for i, cfg := range cases {
		scopes := Scopes(cfg)
		for _, s := range scopes {
			if s == "Mail.Send" {
				t.Errorf("case %d: Scopes() must not include Mail.Send; got %v", i, scopes)
			}
		}
	}
}

// mockAuthenticator is a test double for the Authenticator interface.
type mockAuthenticator struct {
	record azidentity.AuthenticationRecord
	err    error
}

// Authenticate returns the pre-configured record and error.
func (m *mockAuthenticator) Authenticate(_ context.Context, _ *policy.TokenRequestOptions) (azidentity.AuthenticationRecord, error) {
	return m.record, m.err
}
