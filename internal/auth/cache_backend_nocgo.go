//go:build !cgo

package auth

// ResolveTokenCacheBackend determines which token storage backend is actually
// in use at runtime based on the configured storage preference. This is the
// non-CGo build variant where the OS keychain is unavailable.
//
// Without CGo, the OS keychain is never available. All storage preferences
// resolve to "file" (file-based AES-256-GCM encrypted cache).
//
// Parameters:
//   - name: the cache partition name (unused in non-CGo builds, included for
//     interface compatibility with the CGo variant).
//   - storage: the token storage backend preference (unused; always "file").
//
// Returns "file".
func ResolveTokenCacheBackend(_, _ string) string {
	return "file"
}
