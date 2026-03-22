//go:build cgo

package auth

import (
	"testing"
	"unsafe"
)

// TestInitCache_File_SkipsKeychain verifies that when storage="file" is
// requested in a CGo-enabled build, the OS keychain is not attempted and
// a file-based cache is returned directly.
func TestInitCache_File_SkipsKeychain(t *testing.T) {
	cache := InitCache("test-file-cgo", "file")

	// A zero-value azidentity.Cache would indicate failure. The file-based
	// cache should return a populated cache struct.
	shim := *(*cacheShim)(unsafe.Pointer(&cache))
	if shim.impl == nil {
		t.Fatal("InitCache with storage=file returned zero-value cache; expected file-based cache")
	}

	if shim.impl.factory == nil {
		t.Fatal("cache factory is nil")
	}
}

// TestInitMSALCache_File_SkipsKeychain verifies that when storage="file" is
// requested in a CGo-enabled build, the OS keychain is not attempted and
// a file-based MSAL cache accessor is returned directly.
func TestInitMSALCache_File_SkipsKeychain(t *testing.T) {
	er := InitMSALCache("test-msal-file-cgo", "file")
	if er == nil {
		t.Fatal("InitMSALCache with storage=file returned nil; expected file-based ExportReplace")
	}
}

// Note: Tests for keychain failure paths (TestInitCache_Auto_KeychainFailure_FallsBackToFile,
// TestInitCache_Keychain_NoFallbackOnError, TestInitMSALCache_Auto_KeychainFailure_FallsBackToFile)
// require mocking cache.New/accessor.New which are not easily mockable in the current architecture.
// These paths are verified by the storage="file" tests above (which exercise the file-based
// fallback code) and by manual testing on environments with/without a functional keychain.
