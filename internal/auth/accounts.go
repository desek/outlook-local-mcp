// Package auth provides account persistence for multi-account configurations.
//
// This file implements the JSON-based persistence layer for per-account
// identity configuration (client_id, tenant_id, auth_method). The accounts
// file stores only non-secret metadata; tokens and credentials are managed
// separately by the OS-native token cache.
package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// AccountConfig holds the identity configuration for a single account.
// These fields are persisted to the accounts JSON file and used to
// reconstruct credentials after a server restart.
type AccountConfig struct {
	// Label is the unique human-readable identifier for this account.
	Label string `json:"label"`

	// ClientID is the OAuth 2.0 client (application) ID for this account's
	// app registration.
	ClientID string `json:"client_id"`

	// TenantID is the Entra ID tenant identifier for this account
	// (e.g., "common", "organizations", or a specific tenant GUID).
	TenantID string `json:"tenant_id"`

	// AuthMethod is the authentication method used for this account
	// (e.g., "auth_code", "browser", "device_code").
	AuthMethod string `json:"auth_method"`
}

// AccountsFile is the top-level structure of the persistent accounts JSON file.
// It wraps a slice of AccountConfig entries.
type AccountsFile struct {
	// Accounts is the list of persisted account configurations.
	Accounts []AccountConfig `json:"accounts"`
}

// LoadAccounts reads account configurations from the JSON file at the given path.
// If the file does not exist, an empty slice is returned with no error.
//
// Parameters:
//   - path: absolute filesystem path to the accounts JSON file.
//
// Returns the loaded account configurations, or an error if the file exists
// but cannot be read or parsed.
func LoadAccounts(path string) ([]AccountConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []AccountConfig{}, nil
		}
		return nil, fmt.Errorf("read accounts file: %w", err)
	}

	var file AccountsFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse accounts file: %w", err)
	}

	return file.Accounts, nil
}

// SaveAccounts writes account configurations to the JSON file at the given path.
// The write is atomic: data is written to a temporary file in the same directory,
// then renamed to the target path. This prevents corruption from crashes during write.
//
// Parameters:
//   - path: absolute filesystem path to the accounts JSON file.
//   - accounts: the account configurations to persist.
//
// Returns an error if the file cannot be written.
//
// Side effects: creates or overwrites the file at path. Creates the parent
// directory if it does not exist.
func SaveAccounts(path string, accounts []AccountConfig) error {
	file := AccountsFile{Accounts: accounts}

	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal accounts file: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create accounts directory: %w", err)
	}

	tmp, err := os.CreateTemp(dir, "accounts-*.json.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()        //nolint:errcheck // best-effort cleanup on write failure
		os.Remove(tmpPath) //nolint:errcheck // best-effort cleanup on write failure
		return fmt.Errorf("write temp file: %w", err)
	}

	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath) //nolint:errcheck // best-effort cleanup on close failure
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath) //nolint:errcheck // best-effort cleanup on rename failure
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

// AddAccountConfig appends an account configuration to the persistent accounts
// file. The file is loaded, the new config is appended, and the file is saved
// atomically.
//
// Parameters:
//   - path: absolute filesystem path to the accounts JSON file.
//   - config: the account configuration to add.
//
// Returns an error if the file cannot be loaded or saved.
//
// Side effects: modifies the accounts file at path.
func AddAccountConfig(path string, config AccountConfig) error {
	accounts, err := LoadAccounts(path)
	if err != nil {
		return err
	}

	accounts = append(accounts, config)

	return SaveAccounts(path, accounts)
}

// RemoveAccountConfig removes an account configuration by label from the
// persistent accounts file. If the label is not found, no error is returned
// and the file is unchanged.
//
// Parameters:
//   - path: absolute filesystem path to the accounts JSON file.
//   - label: the label of the account to remove.
//
// Returns an error if the file cannot be loaded or saved.
//
// Side effects: modifies the accounts file at path if the label is found.
func RemoveAccountConfig(path string, label string) error {
	accounts, err := LoadAccounts(path)
	if err != nil {
		return err
	}

	filtered := make([]AccountConfig, 0, len(accounts))
	for _, a := range accounts {
		if a.Label != label {
			filtered = append(filtered, a)
		}
	}

	// Only write if something was actually removed.
	if len(filtered) == len(accounts) {
		return nil
	}

	return SaveAccounts(path, filtered)
}
