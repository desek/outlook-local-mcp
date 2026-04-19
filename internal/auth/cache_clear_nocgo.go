//go:build !cgo

package auth

// clearKeychainTokenCache is a no-op on non-CGo builds, where the OS keychain
// is not available. File cache artifacts are cleared by ClearTokenCache
// directly.
//
// Parameters:
//   - name: the cache partition name (unused in non-CGo builds).
func clearKeychainTokenCache(_ string) {}
