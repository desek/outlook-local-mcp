// Package tools_test contains unit tests for the system.about verb handler
// (CR-0067). Tests verify all three output tiers, default fallback values when
// ldflags are absent, auth backend reflection, no-Graph-call behaviour, and
// the under-5ms completion requirement (NFR-3).
package tools_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/desek/outlook-local-mcp/internal/tools"
	"github.com/mark3labs/mcp-go/mcp"
)

// callAbout invokes HandleAbout with the given build-info fields and optional
// output mode and returns the plain-text content string.
func callAbout(t *testing.T, version, commit, buildDate, output string) string {
	t.Helper()
	handler := tools.HandleAbout(version, commit, buildDate)
	req := mcp.CallToolRequest{}
	if output != "" {
		req.Params.Arguments = map[string]any{"output": output}
	}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("HandleAbout returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("HandleAbout returned tool error: %v", result.Content)
	}
	if len(result.Content) == 0 {
		t.Fatal("HandleAbout returned empty content")
	}
	tc, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	return tc.Text
}

// TestAbout_TextDefaultRendersAllFields verifies that the default text output
// contains all 12 labelled fields and stays at or under 24 lines (FR-9).
func TestAbout_TextDefaultRendersAllFields(t *testing.T) {
	text := callAbout(t, "v1.2.3", "abc1234", "2026-01-01T00:00:00Z", "")

	// Verify all 12 labelled fields are present.
	required := []string{
		"outlook-local-mcp",
		"commit:",
		"built:",
		"go:",
		"Host",
		"os/arch:",
		"runtime:",
		"distribution:",
		"auth backend:",
		"Links",
		"homepage:",
		"issues:",
		"docs:",
	}
	for _, want := range required {
		if !strings.Contains(text, want) {
			t.Errorf("text output missing field %q; got:\n%s", want, text)
		}
	}

	// Verify line count stays within FR-9 limit.
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) > 24 {
		t.Errorf("text output has %d lines, want ≤24; got:\n%s", len(lines), text)
	}
}

// TestAbout_SummaryReturnsCompactJSON verifies that output=summary returns
// compact JSON containing exactly the 12 summary fields defined in CR-0067.
func TestAbout_SummaryReturnsCompactJSON(t *testing.T) {
	text := callAbout(t, "v1.0.0", "def5678", "2026-02-01T00:00:00Z", "summary")

	var m map[string]string
	if err := json.Unmarshal([]byte(text), &m); err != nil {
		t.Fatalf("summary output is not valid JSON: %v\ngot:\n%s", err, text)
	}

	expectedFields := []string{
		"version", "commit", "buildDate", "goVersion",
		"os", "arch", "runtime", "distribution",
		"authBackend", "homepage", "issueTracker", "docsBase",
	}
	for _, field := range expectedFields {
		if _, ok := m[field]; !ok {
			t.Errorf("summary JSON missing field %q", field)
		}
	}
	if len(m) != len(expectedFields) {
		t.Errorf("summary JSON has %d fields, want %d", len(m), len(expectedFields))
	}
}

// TestAbout_RawReturnsFullInfoJSON verifies that output=raw returns the full
// buildinfo.Info JSON with all expected keys present.
func TestAbout_RawReturnsFullInfoJSON(t *testing.T) {
	text := callAbout(t, "v2.0.0", "ghi9012", "2026-03-01T00:00:00Z", "raw")

	var m map[string]any
	if err := json.Unmarshal([]byte(text), &m); err != nil {
		t.Fatalf("raw output is not valid JSON: %v\ngot:\n%s", err, text)
	}

	requiredKeys := []string{
		"version", "commit", "buildDate", "goVersion",
		"os", "arch", "runtime", "distribution",
		"authBackend", "homepage", "issueTracker", "docsBase",
	}
	for _, key := range requiredKeys {
		if _, ok := m[key]; !ok {
			t.Errorf("raw JSON missing key %q", key)
		}
	}
}

// TestAbout_DefaultsWhenLdflagsAbsent verifies that when the canonical
// ldflags-absent defaults ("dev", "unknown", "unknown") are passed, the text
// output contains those literal values per FR-3. In main.go the package-level
// vars default to these strings when no ldflags are provided at build time.
func TestAbout_DefaultsWhenLdflagsAbsent(t *testing.T) {
	// Pass the same default values that main.go uses when ldflags are absent.
	text := callAbout(t, "dev", "unknown", "unknown", "")

	for _, want := range []string{"dev", "unknown"} {
		if !strings.Contains(text, want) {
			t.Errorf("text output missing fallback value %q when ldflags absent; got:\n%s", want, text)
		}
	}
}

// TestAbout_AuthBackendReflectsPassedValue verifies that the auth backend
// reported by the about verb reflects the value returned by auth.ActiveBackend
// at request time (AC-3). Since tests run without a real keychain, the active
// backend defaults to "file"; we verify a non-empty string appears.
func TestAbout_AuthBackendReflectsPassedValue(t *testing.T) {
	text := callAbout(t, "v1.0.0", "abc0001", "2026-04-01T00:00:00Z", "summary")

	var m map[string]string
	if err := json.Unmarshal([]byte(text), &m); err != nil {
		t.Fatalf("summary output is not valid JSON: %v", err)
	}
	backend, ok := m["authBackend"]
	if !ok || backend == "" {
		t.Errorf("authBackend field missing or empty in summary; got %v", m)
	}
}

// TestAbout_NoGraphCall verifies that the about handler completes without any
// Microsoft Graph API call. It is invoked with a background context containing
// no authentication credentials; any Graph call would return an error (AC-5).
func TestAbout_NoGraphCall(t *testing.T) {
	// Call with a plain background context — no token, no account. If the
	// handler attempts a Graph call it will fail before returning a result.
	handler := tools.HandleAbout("v1.0.0", "abc0001", "2026-04-01T00:00:00Z")
	result, err := handler(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("HandleAbout returned unexpected error (possible Graph call attempted): %v", err)
	}
	if result.IsError {
		t.Fatalf("HandleAbout returned tool error (possible Graph call attempted): %v", result.Content)
	}
}

// TestAbout_CompletesUnder5ms verifies that the about handler returns within
// 5 milliseconds (NFR-3 — no I/O, no network, no external calls).
func TestAbout_CompletesUnder5ms(t *testing.T) {
	handler := tools.HandleAbout("v1.0.0", "abc0001", "2026-04-01T00:00:00Z")
	start := time.Now()
	_, err := handler(context.Background(), mcp.CallToolRequest{})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("HandleAbout returned error: %v", err)
	}
	if elapsed > 5*time.Millisecond {
		t.Errorf("HandleAbout took %v, want ≤5ms (NFR-3)", elapsed)
	}
}
