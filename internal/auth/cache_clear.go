// Package auth clears token cache artifacts for an account's cache partition.
//
// This file provides ClearTokenCache, used by account_logout (CR-0056) to
// delete the persisted token cache for a given partition name. It targets the
// file-based cache artifacts (CR-0037/CR-0038) unconditionally and attempts a
// best-effort OS keychain deletion via the build-tagged helper
// clearKeychainTokenCache (see cache_clear_cgo.go / cache_clear_nocgo.go).
package auth

import (
	"errors"
	"os"
	"path/filepath"
)

// ClearTokenCache removes the persisted token cache for the given partition
// name. It deletes the file-based cache artifacts (`{name}.bin`,
// `{name}.cae.bin`, `{name}_msal.bin`) from the shared cache directory and
// attempts to delete the OS keychain entry on CGo-enabled builds. Missing
// files or unavailable keychain entries are not treated as errors; this
// function is best-effort (see CR-0056 Risk 3).
//
// Parameters:
//   - name: the cache partition name (AccountEntry.CacheName).
//
// Returns the first unexpected error encountered while deleting file cache
// artifacts, or nil on success. Missing-file conditions are ignored.
func ClearTokenCache(name string) error {
	if name == "" {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	cacheDir := filepath.Join(home, defaultCacheDir)
	paths := []string{
		filepath.Join(cacheDir, name+".bin"),
		filepath.Join(cacheDir, name+".cae.bin"),
		filepath.Join(cacheDir, name+"_msal.bin"),
	}
	for _, p := range paths {
		if removeErr := os.Remove(p); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return removeErr
		}
	}
	// Best-effort keychain deletion; ignored when unavailable.
	clearKeychainTokenCache(name)
	return nil
}
