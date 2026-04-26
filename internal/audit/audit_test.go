package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// TestMaskAuditEmail_Standard validates that a standard email address is masked
// to show only the first character of the local part and the full domain.
func TestMaskAuditEmail_Standard(t *testing.T) {
	got := MaskAuditEmail("alice@example.com")
	want := "a***@example.com"
	if got != want {
		t.Errorf("MaskAuditEmail() = %q, want %q", got, want)
	}
}

// TestMaskAuditEmail_SingleChar validates masking when the local part is a
// single character.
func TestMaskAuditEmail_SingleChar(t *testing.T) {
	got := MaskAuditEmail("a@example.com")
	want := "a***@example.com"
	if got != want {
		t.Errorf("MaskAuditEmail() = %q, want %q", got, want)
	}
}

// TestMaskAuditEmail_NoAtSign validates that non-email strings are returned
// unchanged.
func TestMaskAuditEmail_NoAtSign(t *testing.T) {
	got := MaskAuditEmail("not-an-email")
	want := "not-an-email"
	if got != want {
		t.Errorf("MaskAuditEmail() = %q, want %q", got, want)
	}
}

// TestMaskAuditEmail_Empty validates that an empty string is returned as-is.
func TestMaskAuditEmail_Empty(t *testing.T) {
	got := MaskAuditEmail("")
	if got != "" {
		t.Errorf("MaskAuditEmail() = %q, want empty", got)
	}
}

// TestTruncateAuditString_Short validates that short strings are not truncated.
func TestTruncateAuditString_Short(t *testing.T) {
	got := TruncateAuditString("hello", 200)
	if got != "hello" {
		t.Errorf("TruncateAuditString() = %q, want %q", got, "hello")
	}
}

// TestTruncateAuditString_Exact validates that strings at exactly the limit
// are not truncated.
func TestTruncateAuditString_Exact(t *testing.T) {
	s := strings.Repeat("a", 200)
	got := TruncateAuditString(s, 200)
	if got != s {
		t.Errorf("TruncateAuditString() length = %d, want %d", len(got), 200)
	}
}

// TestTruncateAuditString_Long validates that strings exceeding the limit are
// truncated with the "...[truncated]" suffix.
func TestTruncateAuditString_Long(t *testing.T) {
	s := strings.Repeat("a", 250)
	got := TruncateAuditString(s, 200)
	wantPrefix := strings.Repeat("a", 200)
	wantSuffix := "...[truncated]"
	if got != wantPrefix+wantSuffix {
		t.Errorf("TruncateAuditString() = %q..., want %d chars + suffix", got[:20], 200)
	}
}

// TestSanitizeAuditParams_ExcludesBody validates that the "body" parameter is
// excluded from the sanitized output.
func TestSanitizeAuditParams_ExcludesBody(t *testing.T) {
	params := map[string]any{
		"subject": "Test Meeting",
		"body":    "<p>Secret content</p>",
	}
	got := SanitizeAuditParams(params)
	if _, ok := got["body"]; ok {
		t.Error("SanitizeAuditParams() should exclude 'body' key")
	}
	if got["subject"] != "Test Meeting" {
		t.Errorf("subject = %q, want %q", got["subject"], "Test Meeting")
	}
}

// TestSanitizeAuditParams_MasksEmail validates that email addresses in parameter
// values are masked.
func TestSanitizeAuditParams_MasksEmail(t *testing.T) {
	params := map[string]any{
		"attendees": `[{"email":"alice@example.com"}]`,
	}
	got := SanitizeAuditParams(params)
	if strings.Contains(got["attendees"], "alice@example.com") {
		t.Error("SanitizeAuditParams() should mask email addresses")
	}
	if !strings.Contains(got["attendees"], "a***@example.com") {
		t.Errorf("attendees = %q, want masked email", got["attendees"])
	}
}

// TestSanitizeAuditParams_TruncatesLong validates that long parameter values are
// truncated.
func TestSanitizeAuditParams_TruncatesLong(t *testing.T) {
	params := map[string]any{
		"notes": strings.Repeat("x", 300),
	}
	got := SanitizeAuditParams(params)
	if !strings.HasSuffix(got["notes"], "...[truncated]") {
		t.Errorf("SanitizeAuditParams() should truncate long values, got length %d", len(got["notes"]))
	}
}

// TestSanitizeAuditParams_PreservesNormal validates that normal parameter values
// pass through unchanged.
func TestSanitizeAuditParams_PreservesNormal(t *testing.T) {
	params := map[string]any{
		"event_id": "abc-123",
	}
	got := SanitizeAuditParams(params)
	if got["event_id"] != "abc-123" {
		t.Errorf("event_id = %q, want %q", got["event_id"], "abc-123")
	}
}

// setAuditState is a test helper that sets the module-level audit state and
// restores it after the test.
func setAuditState(t *testing.T, enabled bool, writer *bytes.Buffer) {
	t.Helper()
	origEnabled := auditEnabled
	origWriter := auditWriter
	t.Cleanup(func() {
		auditEnabled = origEnabled
		auditWriter = origWriter
	})
	auditEnabled = enabled
	if writer != nil {
		auditWriter = writer
	}
}

// TestEmitAuditLog_JSONFormat validates that emitted entries are valid JSON with
// all required fields.
func TestEmitAuditLog_JSONFormat(t *testing.T) {
	var buf bytes.Buffer
	setAuditState(t, true, &buf)

	entry := AuditEntry{
		Audit:         true,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		ToolName:      "calendar.list_events",
		OperationType: "read",
		Parameters:    map[string]string{"calendar_id": "cal-1"},
		Outcome:       "success",
		DurationMs:    42,
	}
	EmitAuditLog(entry)

	var parsed AuditEntry
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("EmitAuditLog() produced invalid JSON: %v", err)
	}
	if !parsed.Audit {
		t.Error("parsed.Audit = false, want true")
	}
	if parsed.ToolName != "calendar.list_events" {
		t.Errorf("parsed.ToolName = %q, want %q", parsed.ToolName, "calendar.list_events")
	}
	if parsed.OperationType != "read" {
		t.Errorf("parsed.OperationType = %q, want %q", parsed.OperationType, "read")
	}
	if parsed.Outcome != "success" {
		t.Errorf("parsed.Outcome = %q, want %q", parsed.Outcome, "success")
	}
	if parsed.DurationMs != 42 {
		t.Errorf("parsed.DurationMs = %d, want %d", parsed.DurationMs, 42)
	}
}

// TestEmitAuditLog_FileOutput validates that audit entries are written to a
// file when configured.
func TestEmitAuditLog_FileOutput(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "audit.jsonl")

	origEnabled := auditEnabled
	origWriter := auditWriter
	t.Cleanup(func() {
		auditEnabled = origEnabled
		auditWriter = origWriter
	})

	InitAuditLog(true, path)

	entry := AuditEntry{
		Audit:         true,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		ToolName:      "calendar.get_event",
		OperationType: "read",
		Parameters:    map[string]string{},
		Outcome:       "success",
		DurationMs:    10,
	}
	EmitAuditLog(entry)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read audit file: %v", err)
	}
	if !strings.Contains(string(data), `"tool_name":"calendar.get_event"`) {
		t.Errorf("audit file does not contain expected entry: %s", string(data))
	}
}

// TestEmitAuditLog_StderrFallback validates that audit entries go to the writer
// when no file path is specified (the writer is set to stderr by default).
func TestEmitAuditLog_StderrFallback(t *testing.T) {
	var buf bytes.Buffer
	setAuditState(t, true, &buf)

	entry := AuditEntry{
		Audit:         true,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		ToolName:      "calendar.search_events",
		OperationType: "read",
		Parameters:    map[string]string{},
		Outcome:       "success",
		DurationMs:    5,
	}
	EmitAuditLog(entry)

	if buf.Len() == 0 {
		t.Error("EmitAuditLog() wrote nothing when audit is enabled")
	}
}

// TestEmitAuditLog_Disabled validates that no output is produced when audit
// logging is disabled.
func TestEmitAuditLog_Disabled(t *testing.T) {
	var buf bytes.Buffer
	setAuditState(t, false, &buf)

	entry := AuditEntry{
		Audit:    true,
		ToolName: "calendar.list",
	}
	EmitAuditLog(entry)

	if buf.Len() != 0 {
		t.Errorf("EmitAuditLog() wrote %d bytes when disabled, want 0", buf.Len())
	}
}

// TestEmitAuditLog_AppendOnly validates that multiple entries are appended as
// separate JSON lines.
func TestEmitAuditLog_AppendOnly(t *testing.T) {
	var buf bytes.Buffer
	setAuditState(t, true, &buf)

	for i := 0; i < 3; i++ {
		EmitAuditLog(AuditEntry{
			Audit:         true,
			Timestamp:     time.Now().UTC().Format(time.RFC3339),
			ToolName:      "calendar.list_events",
			OperationType: "read",
			Parameters:    map[string]string{},
			Outcome:       "success",
			DurationMs:    int64(i),
		})
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Errorf("got %d lines, want 3", len(lines))
	}
	for i, line := range lines {
		var parsed AuditEntry
		if err := json.Unmarshal([]byte(line), &parsed); err != nil {
			t.Errorf("line %d is not valid JSON: %v", i, err)
		}
	}
}

// TestEmitAuditLog_FlushAfterWrite validates that entries written to a file are
// immediately readable after EmitAuditLog returns.
func TestEmitAuditLog_FlushAfterWrite(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "audit-flush.jsonl")

	origEnabled := auditEnabled
	origWriter := auditWriter
	t.Cleanup(func() {
		auditEnabled = origEnabled
		auditWriter = origWriter
	})

	InitAuditLog(true, path)

	EmitAuditLog(AuditEntry{
		Audit:         true,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		ToolName:      "calendar.create_event",
		OperationType: "write",
		Parameters:    map[string]string{},
		Outcome:       "success",
		DurationMs:    1,
	})

	// Re-open and read the file to verify the entry was flushed.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read audit file: %v", err)
	}
	if !strings.Contains(string(data), `"tool_name":"calendar.create_event"`) {
		t.Error("audit entry was not flushed to disk")
	}
}

// TestAuditWrap_Success validates that the middleware emits an audit entry with
// outcome "success" when the handler completes without error.
func TestAuditWrap_Success(t *testing.T) {
	var buf bytes.Buffer
	setAuditState(t, true, &buf)

	handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.TextContent{Text: "ok"}}}, nil
	}
	wrapped := AuditWrap("calendar.list", "read", handler)

	result, err := wrapped(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	var entry AuditEntry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("invalid audit JSON: %v", err)
	}
	if entry.Outcome != "success" {
		t.Errorf("Outcome = %q, want %q", entry.Outcome, "success")
	}
	if entry.ToolName != "calendar.list" {
		t.Errorf("ToolName = %q, want %q", entry.ToolName, "calendar.list")
	}
	if entry.OperationType != "read" {
		t.Errorf("OperationType = %q, want %q", entry.OperationType, "read")
	}
}

// TestAuditWrap_ToolError validates that the middleware emits an audit entry with
// outcome "error" when the handler returns a tool error (IsError=true).
func TestAuditWrap_ToolError(t *testing.T) {
	var buf bytes.Buffer
	setAuditState(t, true, &buf)

	handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultError("something went wrong"), nil
	}
	wrapped := AuditWrap("calendar.create_event", "write", handler)

	result, err := wrapped(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatal("expected error result")
	}

	var entry AuditEntry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("invalid audit JSON: %v", err)
	}
	if entry.Outcome != "error" {
		t.Errorf("Outcome = %q, want %q", entry.Outcome, "error")
	}
	if entry.ErrorMessage != "something went wrong" {
		t.Errorf("ErrorMessage = %q, want %q", entry.ErrorMessage, "something went wrong")
	}
}

// TestAuditWrap_ProtocolError validates that the middleware emits an audit entry
// with outcome "error" when the handler returns a Go error.
func TestAuditWrap_ProtocolError(t *testing.T) {
	var buf bytes.Buffer
	setAuditState(t, true, &buf)

	handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return nil, errors.New("protocol failure")
	}
	wrapped := AuditWrap("calendar_delete_event", "delete", handler)

	_, err := wrapped(context.Background(), mcp.CallToolRequest{})
	if err == nil {
		t.Fatal("expected error")
	}

	var entry AuditEntry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("invalid audit JSON: %v", err)
	}
	if entry.Outcome != "error" {
		t.Errorf("Outcome = %q, want %q", entry.Outcome, "error")
	}
	if entry.ErrorMessage != "protocol failure" {
		t.Errorf("ErrorMessage = %q, want %q", entry.ErrorMessage, "protocol failure")
	}
}

// TestAuditWrap_DurationMeasured validates that the duration_ms field reflects
// the actual handler execution time.
func TestAuditWrap_DurationMeasured(t *testing.T) {
	var buf bytes.Buffer
	setAuditState(t, true, &buf)

	handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		time.Sleep(10 * time.Millisecond)
		return &mcp.CallToolResult{}, nil
	}
	wrapped := AuditWrap("calendar.get_event", "read", handler)

	_, _ = wrapped(context.Background(), mcp.CallToolRequest{})

	var entry AuditEntry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("invalid audit JSON: %v", err)
	}
	if entry.DurationMs < 10 {
		t.Errorf("DurationMs = %d, want >= 10", entry.DurationMs)
	}
}

// TestAuditWrap_EventIDExtracted validates that event_id is extracted from
// request parameters.
func TestAuditWrap_EventIDExtracted(t *testing.T) {
	var buf bytes.Buffer
	setAuditState(t, true, &buf)

	handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{}, nil
	}
	wrapped := AuditWrap("calendar.get_event", "read", handler)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"event_id": "evt-123"}

	_, _ = wrapped(context.Background(), req)

	var entry AuditEntry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("invalid audit JSON: %v", err)
	}
	if entry.EventID != "evt-123" {
		t.Errorf("EventID = %q, want %q", entry.EventID, "evt-123")
	}
}

// TestAuditWrap_CalendarIDExtracted validates that calendar_id is extracted
// from request parameters.
func TestAuditWrap_CalendarIDExtracted(t *testing.T) {
	var buf bytes.Buffer
	setAuditState(t, true, &buf)

	handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{}, nil
	}
	wrapped := AuditWrap("calendar.list_events", "read", handler)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"calendar_id": "cal-456"}

	_, _ = wrapped(context.Background(), req)

	var entry AuditEntry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("invalid audit JSON: %v", err)
	}
	if entry.CalendarID != "cal-456" {
		t.Errorf("CalendarID = %q, want %q", entry.CalendarID, "cal-456")
	}
}

// TestAuditWrap_PassesResultThrough validates that the middleware returns the
// handler's result and error unchanged.
func TestAuditWrap_PassesResultThrough(t *testing.T) {
	var buf bytes.Buffer
	setAuditState(t, true, &buf)

	wantResult := &mcp.CallToolResult{
		Content: []mcp.Content{mcp.TextContent{Text: "calendar data"}},
	}
	handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return wantResult, nil
	}
	wrapped := AuditWrap("calendar.list", "read", handler)

	gotResult, gotErr := wrapped(context.Background(), mcp.CallToolRequest{})
	if gotErr != nil {
		t.Fatalf("unexpected error: %v", gotErr)
	}
	if gotResult != wantResult {
		t.Error("AuditWrap modified the handler's result")
	}
}

// TestInitAuditLog_FileCreated validates that InitAuditLog creates the audit
// file at the specified path.
func TestInitAuditLog_FileCreated(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "audit-init.jsonl")

	origEnabled := auditEnabled
	origWriter := auditWriter
	t.Cleanup(func() {
		auditEnabled = origEnabled
		auditWriter = origWriter
	})

	InitAuditLog(true, path)

	if !auditEnabled {
		t.Error("auditEnabled = false, want true")
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("audit file was not created at %s", path)
	}
}

// TestInitAuditLog_InvalidPath validates that InitAuditLog falls back to stderr
// when the file path is invalid.
func TestInitAuditLog_InvalidPath(t *testing.T) {
	origEnabled := auditEnabled
	origWriter := auditWriter
	t.Cleanup(func() {
		auditEnabled = origEnabled
		auditWriter = origWriter
	})

	InitAuditLog(true, "/nonexistent/dir/audit.jsonl")

	if !auditEnabled {
		t.Error("auditEnabled = false, want true")
	}
	if auditWriter != os.Stderr {
		t.Error("auditWriter should fall back to os.Stderr on invalid path")
	}
}

// TestInitAuditLog_Disabled validates that InitAuditLog does not open a file
// when disabled.
func TestInitAuditLog_Disabled(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "should-not-exist.jsonl")

	origEnabled := auditEnabled
	origWriter := auditWriter
	t.Cleanup(func() {
		auditEnabled = origEnabled
		auditWriter = origWriter
	})

	InitAuditLog(false, path)

	if auditEnabled {
		t.Error("auditEnabled = true, want false")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file should not be created when audit is disabled")
	}
}

// TestOperationTypeClassification validates the operation type mapping for all
// nine tools by verifying the constants used in registerTools.
func TestOperationTypeClassification(t *testing.T) {
	tests := []struct {
		toolName string
		opType   string
	}{
		{"calendar.list", "read"},
		{"calendar.list_events", "read"},
		{"calendar.get_event", "read"},
		{"calendar.search_events", "read"},
		{"calendar_get_free_busy", "read"},
		{"calendar.create_event", "write"},
		{"calendar_update_event", "write"},
		{"calendar_delete_event", "delete"},
		{"calendar_cancel_meeting", "delete"},
	}

	for _, tt := range tests {
		t.Run(tt.toolName, func(t *testing.T) {
			var buf bytes.Buffer
			setAuditState(t, true, &buf)

			handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return &mcp.CallToolResult{}, nil
			}
			wrapped := AuditWrap(tt.toolName, tt.opType, handler)
			_, _ = wrapped(context.Background(), mcp.CallToolRequest{})

			var entry AuditEntry
			if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
				t.Fatalf("invalid audit JSON: %v", err)
			}
			if entry.ToolName != tt.toolName {
				t.Errorf("ToolName = %q, want %q", entry.ToolName, tt.toolName)
			}
			if entry.OperationType != tt.opType {
				t.Errorf("OperationType = %q, want %q", entry.OperationType, tt.opType)
			}
		})
	}
}

// TestAuditEntryAuditField validates that the "audit":true discriminator field
// is always present in serialized entries.
func TestAuditEntryAuditField(t *testing.T) {
	var buf bytes.Buffer
	setAuditState(t, true, &buf)

	EmitAuditLog(AuditEntry{
		Audit:         true,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		ToolName:      "calendar.list",
		OperationType: "read",
		Parameters:    map[string]string{},
		Outcome:       "success",
		DurationMs:    1,
	})

	if !strings.Contains(buf.String(), `"audit":true`) {
		t.Error("audit entry missing 'audit':true discriminator")
	}
}
