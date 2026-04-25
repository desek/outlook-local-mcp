package auth

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSaveAndLoadAccounts verifies that account configurations survive a
// round-trip through SaveAccounts and LoadAccounts.
func TestSaveAndLoadAccounts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "accounts.json")

	want := []AccountConfig{
		{Label: "work", ClientID: "aaaa", TenantID: "tenant-a", AuthMethod: "browser"},
		{Label: "personal", ClientID: "bbbb", TenantID: "common", AuthMethod: "device_code"},
	}

	if err := SaveAccounts(path, want); err != nil {
		t.Fatalf("SaveAccounts: %v", err)
	}

	got, err := LoadAccounts(path)
	if err != nil {
		t.Fatalf("LoadAccounts: %v", err)
	}

	if len(got) != len(want) {
		t.Fatalf("LoadAccounts returned %d accounts, want %d", len(got), len(want))
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("account[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestLoadAccounts_FileNotExist verifies that LoadAccounts returns an empty
// slice with no error when the accounts file does not exist.
func TestLoadAccounts_FileNotExist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent", "accounts.json")

	accounts, err := LoadAccounts(path)
	if err != nil {
		t.Fatalf("LoadAccounts: %v", err)
	}

	if len(accounts) != 0 {
		t.Errorf("LoadAccounts returned %d accounts, want 0", len(accounts))
	}
}

// TestAddAccountConfig verifies that AddAccountConfig appends a new account
// to the existing accounts file.
func TestAddAccountConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "accounts.json")

	initial := []AccountConfig{
		{Label: "work", ClientID: "aaaa", TenantID: "tenant-a", AuthMethod: "browser"},
	}
	if err := SaveAccounts(path, initial); err != nil {
		t.Fatalf("SaveAccounts: %v", err)
	}

	newConfig := AccountConfig{
		Label:      "personal",
		ClientID:   "bbbb",
		TenantID:   "common",
		AuthMethod: "device_code",
	}
	if err := AddAccountConfig(path, newConfig); err != nil {
		t.Fatalf("AddAccountConfig: %v", err)
	}

	got, err := LoadAccounts(path)
	if err != nil {
		t.Fatalf("LoadAccounts: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("LoadAccounts returned %d accounts, want 2", len(got))
	}

	if got[0].Label != "work" {
		t.Errorf("account[0].Label = %q, want %q", got[0].Label, "work")
	}
	if got[1].Label != "personal" {
		t.Errorf("account[1].Label = %q, want %q", got[1].Label, "personal")
	}
}

// TestRemoveAccountConfig verifies that RemoveAccountConfig removes the
// account with the given label from the accounts file.
func TestRemoveAccountConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "accounts.json")

	initial := []AccountConfig{
		{Label: "work", ClientID: "aaaa", TenantID: "tenant-a", AuthMethod: "browser"},
		{Label: "personal", ClientID: "bbbb", TenantID: "common", AuthMethod: "device_code"},
	}
	if err := SaveAccounts(path, initial); err != nil {
		t.Fatalf("SaveAccounts: %v", err)
	}

	if err := RemoveAccountConfig(path, "work"); err != nil {
		t.Fatalf("RemoveAccountConfig: %v", err)
	}

	got, err := LoadAccounts(path)
	if err != nil {
		t.Fatalf("LoadAccounts: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("LoadAccounts returned %d accounts, want 1", len(got))
	}

	if got[0].Label != "personal" {
		t.Errorf("remaining account Label = %q, want %q", got[0].Label, "personal")
	}
}

// TestUpdateAccountUPN_Success verifies that UpdateAccountUPN persists the
// supplied UPN onto the matching account entry in accounts.json.
func TestUpdateAccountUPN_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "accounts.json")

	initial := []AccountConfig{
		{Label: "work", ClientID: "aaaa", TenantID: "tenant-a", AuthMethod: "browser"},
	}
	if err := SaveAccounts(path, initial); err != nil {
		t.Fatalf("SaveAccounts: %v", err)
	}

	if err := UpdateAccountUPN(path, "work", "alice@contoso.com"); err != nil {
		t.Fatalf("UpdateAccountUPN: %v", err)
	}

	got, err := LoadAccounts(path)
	if err != nil {
		t.Fatalf("LoadAccounts: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("LoadAccounts returned %d accounts, want 1", len(got))
	}
	if got[0].UPN != "alice@contoso.com" {
		t.Errorf("UPN = %q, want %q", got[0].UPN, "alice@contoso.com")
	}
}

// TestUpdateAccountUPN_NotFound verifies that UpdateAccountUPN is a silent
// no-op when the label is not present in accounts.json, leaving the file
// unchanged.
func TestUpdateAccountUPN_NotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "accounts.json")

	initial := []AccountConfig{
		{Label: "work", ClientID: "aaaa", TenantID: "tenant-a", AuthMethod: "browser"},
	}
	if err := SaveAccounts(path, initial); err != nil {
		t.Fatalf("SaveAccounts: %v", err)
	}

	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile before: %v", err)
	}

	if err := UpdateAccountUPN(path, "ghost", "nobody@example.com"); err != nil {
		t.Fatalf("UpdateAccountUPN: %v", err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile after: %v", err)
	}

	if string(before) != string(after) {
		t.Error("file content changed when label was not found")
	}
}

// TestFindByIdentity_Match verifies that FindByIdentity returns the first entry
// whose ClientID and TenantID both match the supplied arguments.
func TestFindByIdentity_Match(t *testing.T) {
	accounts := []AccountConfig{
		{Label: "other", ClientID: "aaa", TenantID: "tenant-x", AuthMethod: "browser"},
		{Label: "work", ClientID: "cid-1", TenantID: "tid-1", AuthMethod: "browser"},
	}

	got, ok := FindByIdentity(accounts, "cid-1", "tid-1")
	if !ok {
		t.Fatal("FindByIdentity returned false, want true")
	}
	if got.Label != "work" {
		t.Errorf("Label = %q, want %q", got.Label, "work")
	}
}

// TestFindByIdentity_NoMatch verifies that FindByIdentity returns (zero, false)
// when no entry matches the supplied clientID and tenantID.
func TestFindByIdentity_NoMatch(t *testing.T) {
	accounts := []AccountConfig{
		{Label: "work", ClientID: "cid-1", TenantID: "tid-1", AuthMethod: "browser"},
	}

	got, ok := FindByIdentity(accounts, "cid-99", "tid-99")
	if ok {
		t.Fatalf("FindByIdentity returned true for non-matching identity, entry = %+v", got)
	}
	if got != (AccountConfig{}) {
		t.Errorf("FindByIdentity returned non-zero entry on no-match: %+v", got)
	}
}

// TestFindByIdentity_EmptyArgs verifies that FindByIdentity returns (zero, false)
// when either clientID or tenantID is empty, preventing spurious matches.
func TestFindByIdentity_EmptyArgs(t *testing.T) {
	accounts := []AccountConfig{
		{Label: "work", ClientID: "cid-1", TenantID: "tid-1", AuthMethod: "browser"},
	}

	tests := []struct {
		name     string
		clientID string
		tenantID string
	}{
		{"empty clientID", "", "tid-1"},
		{"empty tenantID", "cid-1", ""},
		{"both empty", "", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := FindByIdentity(accounts, tc.clientID, tc.tenantID)
			if ok {
				t.Fatalf("FindByIdentity returned true for empty arg case %q, entry = %+v", tc.name, got)
			}
			if got != (AccountConfig{}) {
				t.Errorf("FindByIdentity returned non-zero entry: %+v", got)
			}
		})
	}
}

// TestRemoveAccountConfig_NotFound verifies that RemoveAccountConfig returns
// no error and leaves the file unchanged when the label is not found.
func TestRemoveAccountConfig_NotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "accounts.json")

	initial := []AccountConfig{
		{Label: "work", ClientID: "aaaa", TenantID: "tenant-a", AuthMethod: "browser"},
	}
	if err := SaveAccounts(path, initial); err != nil {
		t.Fatalf("SaveAccounts: %v", err)
	}

	// Capture file content before removal attempt.
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile before: %v", err)
	}

	if err := RemoveAccountConfig(path, "ghost"); err != nil {
		t.Fatalf("RemoveAccountConfig: %v", err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile after: %v", err)
	}

	if string(before) != string(after) {
		t.Error("file content changed after removing non-existent label")
	}
}
