// Package auth account registry for multi-account support.
//
// This file defines the AccountRegistry, a thread-safe in-memory store for
// managing multiple authenticated Microsoft accounts. Each account is
// identified by a unique label and holds the credential, Graph client, and
// authentication metadata needed for per-request account resolution.
package auth

import (
	"fmt"
	"regexp"
	"sort"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
)

// labelPattern defines the valid format for account labels: 1-64 characters
// of alphanumeric, underscore, or hyphen.
var labelPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// AccountEntry holds all state for a single authenticated account in the
// registry. Each field is populated during account creation (add_account
// tool or startup registration of the default account).
type AccountEntry struct {
	// Label is the unique human-readable identifier for this account
	// (e.g., "work", "personal", "default").
	Label string

	// ClientID is the OAuth 2.0 client (application) ID used to authenticate
	// this account. Stored for persistence so the account can be reconstructed
	// after a server restart.
	ClientID string

	// TenantID is the Azure AD tenant identifier used to authenticate this
	// account (e.g., "common", "organizations", or a specific tenant GUID).
	// Stored for persistence so the account can be reconstructed after a
	// server restart.
	TenantID string

	// AuthMethod is the authentication method used for this account
	// (e.g., "auth_code", "browser", "device_code"). Stored for persistence
	// so the account can be reconstructed after a server restart.
	AuthMethod string

	// Credential is the Azure token credential used by the Graph SDK to
	// obtain access tokens for API calls.
	Credential azcore.TokenCredential

	// Authenticator is the credential's Authenticator interface for
	// triggering interactive re-authentication when tokens expire.
	Authenticator Authenticator

	// Client is the Microsoft Graph SDK client configured with this
	// account's credential.
	Client *msgraphsdk.GraphServiceClient

	// AuthRecordPath is the filesystem path where this account's
	// AuthenticationRecord is persisted.
	AuthRecordPath string

	// CacheName is the OS keychain partition name for this account's
	// persistent token cache.
	CacheName string

	// Authenticated indicates whether this account has a valid credential
	// and Graph client. Accounts restored from disk without a cached token
	// may have Authenticated == false until the user re-authenticates.
	Authenticated bool
}

// AccountRegistry is a thread-safe store for multiple authenticated accounts.
// It uses a sync.RWMutex to allow concurrent reads while serializing writes.
type AccountRegistry struct {
	mu       sync.RWMutex
	accounts map[string]*AccountEntry
}

// NewAccountRegistry creates an empty AccountRegistry ready for use.
//
// Returns a new AccountRegistry with an initialized internal map.
func NewAccountRegistry() *AccountRegistry {
	return &AccountRegistry{
		accounts: make(map[string]*AccountEntry),
	}
}

// Add registers a new account entry in the registry. The entry's Label must
// be non-empty, match the pattern ^[a-zA-Z0-9_-]{1,64}$, and not already
// exist in the registry.
//
// Parameters:
//   - entry: the account entry to register. Must have a valid, unique Label.
//
// Returns an error if the label is invalid or already registered.
//
// Side effects: stores the entry in the registry under its Label.
func (r *AccountRegistry) Add(entry *AccountEntry) error {
	if entry == nil {
		return fmt.Errorf("account entry must not be nil")
	}
	if !labelPattern.MatchString(entry.Label) {
		return fmt.Errorf("invalid account label %q: must match %s", entry.Label, labelPattern.String())
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.accounts[entry.Label]; exists {
		return fmt.Errorf("account %q already registered", entry.Label)
	}

	r.accounts[entry.Label] = entry
	return nil
}

// Remove deletes an account from the registry by label. The "default" account
// cannot be removed, as it serves as the fallback for single-account
// operation and elicitation-unsupported clients.
//
// Parameters:
//   - label: the label of the account to remove.
//
// Returns an error if the label is "default" or the account does not exist.
//
// Side effects: removes the entry from the registry.
func (r *AccountRegistry) Remove(label string) error {
	if label == "default" {
		return fmt.Errorf("cannot remove the default account")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.accounts[label]; !exists {
		return fmt.Errorf("account %q not found", label)
	}

	delete(r.accounts, label)
	return nil
}

// Get retrieves an account entry by label.
//
// Parameters:
//   - label: the label of the account to retrieve.
//
// Returns the entry and true if found, or nil and false if not.
func (r *AccountRegistry) Get(label string) (*AccountEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, exists := r.accounts[label]
	return entry, exists
}

// List returns all registered account entries sorted alphabetically by label.
//
// Returns a slice of account entries. The returned slice is a copy; modifying
// it does not affect the registry.
func (r *AccountRegistry) List() []*AccountEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entries := make([]*AccountEntry, 0, len(r.accounts))
	for _, entry := range r.accounts {
		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Label < entries[j].Label
	})

	return entries
}

// Count returns the number of registered accounts.
func (r *AccountRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.accounts)
}

// ListAuthenticated returns all registered account entries that have
// Authenticated == true, sorted alphabetically by label.
//
// Returns a slice of authenticated account entries. The returned slice is a
// copy; modifying it does not affect the registry.
func (r *AccountRegistry) ListAuthenticated() []*AccountEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entries := make([]*AccountEntry, 0, len(r.accounts))
	for _, entry := range r.accounts {
		if entry.Authenticated {
			entries = append(entries, entry)
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Label < entries[j].Label
	})

	return entries
}

// Labels returns a sorted list of all registered account labels.
//
// Returns a slice of label strings in alphabetical order.
func (r *AccountRegistry) Labels() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	labels := make([]string, 0, len(r.accounts))
	for label := range r.accounts {
		labels = append(labels, label)
	}

	sort.Strings(labels)
	return labels
}
