package auth

import (
	"sort"
	"sync"
	"testing"
)

func newTestEntry(label string) *AccountEntry {
	return &AccountEntry{Label: label}
}

func TestAdd_ValidEntry(t *testing.T) {
	r := NewAccountRegistry()
	if err := r.Add(newTestEntry("work")); err != nil {
		t.Fatalf("Add valid entry: %v", err)
	}
	if r.Count() != 1 {
		t.Errorf("Count = %d, want 1", r.Count())
	}
}

func TestAdd_DuplicateLabel(t *testing.T) {
	r := NewAccountRegistry()
	_ = r.Add(newTestEntry("work"))
	if err := r.Add(newTestEntry("work")); err == nil {
		t.Fatal("expected error for duplicate label")
	}
}

func TestAdd_InvalidLabels(t *testing.T) {
	r := NewAccountRegistry()

	cases := []struct {
		name  string
		label string
	}{
		{"empty", ""},
		{"too long", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}, // 65 chars
		{"special chars", "work@home"},
		{"spaces", "my account"},
		{"unicode", "werk\u00fc"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := r.Add(newTestEntry(tc.label)); err == nil {
				t.Errorf("expected error for label %q", tc.label)
			}
		})
	}
}

func TestAdd_NilEntry(t *testing.T) {
	r := NewAccountRegistry()
	if err := r.Add(nil); err == nil {
		t.Fatal("expected error for nil entry")
	}
}

func TestRemove_ExistingAccount(t *testing.T) {
	r := NewAccountRegistry()
	_ = r.Add(newTestEntry("work"))

	if err := r.Remove("work"); err != nil {
		t.Fatalf("Remove existing: %v", err)
	}
	if r.Count() != 0 {
		t.Errorf("Count = %d, want 0", r.Count())
	}
}

func TestRemove_DefaultRejected(t *testing.T) {
	r := NewAccountRegistry()
	_ = r.Add(newTestEntry("default"))

	if err := r.Remove("default"); err == nil {
		t.Fatal("expected error when removing default account")
	}
	if r.Count() != 1 {
		t.Error("default account should still exist")
	}
}

func TestRemove_NonExistent(t *testing.T) {
	r := NewAccountRegistry()
	if err := r.Remove("ghost"); err == nil {
		t.Fatal("expected error for non-existent account")
	}
}

func TestGet_Existing(t *testing.T) {
	r := NewAccountRegistry()
	_ = r.Add(newTestEntry("personal"))

	entry, ok := r.Get("personal")
	if !ok {
		t.Fatal("expected ok=true for existing account")
	}
	if entry.Label != "personal" {
		t.Errorf("Label = %q, want %q", entry.Label, "personal")
	}
}

func TestGet_NonExistent(t *testing.T) {
	r := NewAccountRegistry()

	_, ok := r.Get("ghost")
	if ok {
		t.Fatal("expected ok=false for non-existent account")
	}
}

func TestList_Sorted(t *testing.T) {
	r := NewAccountRegistry()
	_ = r.Add(newTestEntry("charlie"))
	_ = r.Add(newTestEntry("alpha"))
	_ = r.Add(newTestEntry("bravo"))

	entries := r.List()
	if len(entries) != 3 {
		t.Fatalf("List len = %d, want 3", len(entries))
	}

	got := make([]string, len(entries))
	for i, e := range entries {
		got[i] = e.Label
	}

	want := []string{"alpha", "bravo", "charlie"}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("List[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestCount_AfterAddRemove(t *testing.T) {
	r := NewAccountRegistry()

	if r.Count() != 0 {
		t.Errorf("initial Count = %d, want 0", r.Count())
	}

	_ = r.Add(newTestEntry("a"))
	_ = r.Add(newTestEntry("b"))
	if r.Count() != 2 {
		t.Errorf("Count after 2 adds = %d, want 2", r.Count())
	}

	_ = r.Remove("a")
	if r.Count() != 1 {
		t.Errorf("Count after remove = %d, want 1", r.Count())
	}
}

func TestLabels_Sorted(t *testing.T) {
	r := NewAccountRegistry()
	_ = r.Add(newTestEntry("zulu"))
	_ = r.Add(newTestEntry("alpha"))
	_ = r.Add(newTestEntry("mike"))

	labels := r.Labels()
	if !sort.StringsAreSorted(labels) {
		t.Errorf("Labels not sorted: %v", labels)
	}

	want := []string{"alpha", "mike", "zulu"}
	if len(labels) != len(want) {
		t.Fatalf("Labels len = %d, want %d", len(labels), len(want))
	}
	for i := range labels {
		if labels[i] != want[i] {
			t.Errorf("Labels[%d] = %q, want %q", i, labels[i], want[i])
		}
	}
}

func TestConcurrentAccess(t *testing.T) {
	r := NewAccountRegistry()
	var wg sync.WaitGroup

	// Spawn goroutines that add accounts concurrently.
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			label := "account-" + string(rune('A'+n%26)) + string(rune('0'+n/26))
			_ = r.Add(newTestEntry(label))
		}(i)
	}

	// Spawn goroutines that read concurrently.
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = r.Count()
			_ = r.Labels()
			_ = r.List()
		}()
	}

	wg.Wait()

	// Verify no panic and count is reasonable (some adds may have
	// duplicate labels due to mod 26, so we just check > 0).
	if r.Count() == 0 {
		t.Error("expected at least one account after concurrent adds")
	}
}

// TestAccountRegistry_CachePartitionIsolation verifies that each account in the
// registry has a unique CacheName, ensuring token cache isolation.
func TestAccountRegistry_CachePartitionIsolation(t *testing.T) {
	r := NewAccountRegistry()

	entries := []*AccountEntry{
		{Label: "default", CacheName: "outlook-mcp"},
		{Label: "work", CacheName: "outlook-mcp-work"},
		{Label: "personal", CacheName: "outlook-mcp-personal"},
	}

	for _, e := range entries {
		if err := r.Add(e); err != nil {
			t.Fatalf("Add(%q) error: %v", e.Label, err)
		}
	}

	// Verify each account has a distinct CacheName.
	seen := make(map[string]string)
	for _, e := range r.List() {
		if prev, exists := seen[e.CacheName]; exists {
			t.Errorf("CacheName %q shared by accounts %q and %q", e.CacheName, prev, e.Label)
		}
		seen[e.CacheName] = e.Label
	}

	if len(seen) != 3 {
		t.Errorf("expected 3 unique CacheNames, got %d", len(seen))
	}
}

// TestAccountEntry_IdentityFields verifies that identity configuration fields
// (ClientID, TenantID, AuthMethod) are stored and retrievable from the registry.
func TestAccountEntry_IdentityFields(t *testing.T) {
	r := NewAccountRegistry()

	entry := &AccountEntry{
		Label:      "work",
		ClientID:   "aaaa-bbbb-cccc",
		TenantID:   "tenant-a",
		AuthMethod: "browser",
	}

	if err := r.Add(entry); err != nil {
		t.Fatalf("Add: %v", err)
	}

	got, ok := r.Get("work")
	if !ok {
		t.Fatal("Get returned ok=false for existing account")
	}

	if got.ClientID != "aaaa-bbbb-cccc" {
		t.Errorf("ClientID = %q, want %q", got.ClientID, "aaaa-bbbb-cccc")
	}
	if got.TenantID != "tenant-a" {
		t.Errorf("TenantID = %q, want %q", got.TenantID, "tenant-a")
	}
	if got.AuthMethod != "browser" {
		t.Errorf("AuthMethod = %q, want %q", got.AuthMethod, "browser")
	}
}

// TestListAuthenticated_FiltersCorrectly verifies that ListAuthenticated
// returns only entries with Authenticated == true, sorted alphabetically.
func TestListAuthenticated_FiltersCorrectly(t *testing.T) {
	r := NewAccountRegistry()
	_ = r.Add(&AccountEntry{Label: "default", Authenticated: true})
	_ = r.Add(&AccountEntry{Label: "work", Authenticated: true})
	_ = r.Add(&AccountEntry{Label: "account-2", Authenticated: false})
	_ = r.Add(&AccountEntry{Label: "stale", Authenticated: false})

	authenticated := r.ListAuthenticated()
	if len(authenticated) != 2 {
		t.Fatalf("ListAuthenticated len = %d, want 2", len(authenticated))
	}

	got := make([]string, len(authenticated))
	for i, e := range authenticated {
		got[i] = e.Label
	}

	want := []string{"default", "work"}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("ListAuthenticated[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestGetByUPN_Found verifies that GetByUPN returns the account entry whose
// Email matches the queried UPN, using case-insensitive comparison per CR-0056.
func TestGetByUPN_Found(t *testing.T) {
	r := NewAccountRegistry()
	entry := &AccountEntry{Label: "work", Email: "Alice@Contoso.com"}
	if err := r.Add(entry); err != nil {
		t.Fatalf("Add: %v", err)
	}

	got, ok := r.GetByUPN("alice@contoso.com")
	if !ok {
		t.Fatal("GetByUPN returned ok=false for matching UPN")
	}
	if got.Label != "work" {
		t.Errorf("got Label = %q, want %q", got.Label, "work")
	}
}

// TestGetByUPN_NotFound verifies that GetByUPN returns ok=false when no
// registered account has a matching Email.
func TestGetByUPN_NotFound(t *testing.T) {
	r := NewAccountRegistry()
	_ = r.Add(&AccountEntry{Label: "work", Email: "alice@contoso.com"})

	if _, ok := r.GetByUPN("nobody@example.com"); ok {
		t.Fatal("GetByUPN returned ok=true for non-matching UPN")
	}
}

// TestUpdate_ModifiesEntry verifies that Update applies the callback to the
// live registry entry and the mutation is visible to subsequent readers.
func TestUpdate_ModifiesEntry(t *testing.T) {
	r := NewAccountRegistry()
	_ = r.Add(&AccountEntry{Label: "work", Authenticated: true})

	err := r.Update("work", func(e *AccountEntry) {
		e.Authenticated = false
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, _ := r.Get("work")
	if got.Authenticated {
		t.Error("expected Authenticated=false after Update")
	}
}

// TestUpdate_NotFound verifies that Update returns an error when the label
// is not present in the registry.
func TestUpdate_NotFound(t *testing.T) {
	r := NewAccountRegistry()

	err := r.Update("ghost", func(e *AccountEntry) {})
	if err == nil {
		t.Fatal("expected error when updating non-existent account")
	}
}

func TestAdd_ValidLabelEdgeCases(t *testing.T) {
	r := NewAccountRegistry()

	cases := []struct {
		name  string
		label string
	}{
		{"single char", "a"},
		{"max length 64", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}, // exactly 64
		{"with hyphens", "my-account"},
		{"with underscores", "my_account"},
		{"digits only", "123"},
		{"mixed", "Work-Account_01"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := r.Add(newTestEntry(tc.label)); err != nil {
				t.Errorf("expected valid label %q, got error: %v", tc.label, err)
			}
		})
	}
}
