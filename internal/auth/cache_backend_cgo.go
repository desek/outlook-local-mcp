//go:build cgo

package auth

import (
	"github.com/AzureAD/microsoft-authentication-extensions-for-go/cache/accessor"
)

// ResolveTokenCacheBackend determines which token storage backend is actually
// in use at runtime based on the configured storage preference. This is the
// CGo-enabled build variant where the OS keychain is available.
//
// Resolution logic mirrors InitCache:
//   - "file": returns "file" (file-based AES-256-GCM, keychain skipped).
//   - "keychain": returns "keychain" if the OS keychain probe succeeds,
//     otherwise returns "file" (no fallback -- but the probe here is
//     informational; the actual InitCache may have returned zero-value).
//   - "auto": returns "keychain" if the OS keychain probe succeeds,
//     otherwise returns "file".
//
// Parameters:
//   - name: the cache partition name (same value passed to InitCache).
//   - storage: the token storage backend preference ("auto", "keychain", or
//     "file").
//
// Returns "keychain" or "file".
func ResolveTokenCacheBackend(name, storage string) string {
	if storage == "file" {
		return "file"
	}

	// Probe OS keychain availability using the same accessor used by InitMSALCache.
	_, err := accessor.New(name)
	if err != nil {
		return "file"
	}
	return "keychain"
}
