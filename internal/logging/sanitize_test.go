package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
)

// TestMaskEmail_Standard verifies standard email masking preserves the first
// two characters of the local part and the full domain.
func TestMaskEmail_Standard(t *testing.T) {
	got := MaskEmail("alice@contoso.com")
	want := "al***@contoso.com"
	if got != want {
		t.Errorf("MaskEmail() = %q, want %q", got, want)
	}
}

// TestMaskEmail_ShortLocal verifies masking with a 1-character local part
// preserves the single character.
func TestMaskEmail_ShortLocal(t *testing.T) {
	got := MaskEmail("a@b.com")
	want := "a***@b.com"
	if got != want {
		t.Errorf("MaskEmail() = %q, want %q", got, want)
	}
}

// TestMaskEmail_TwoCharLocal verifies masking with exactly 2-character local
// part preserves both characters.
func TestMaskEmail_TwoCharLocal(t *testing.T) {
	got := MaskEmail("ab@c.com")
	want := "ab***@c.com"
	if got != want {
		t.Errorf("MaskEmail() = %q, want %q", got, want)
	}
}

// TestMaskEmail_NoAtSign verifies that non-email strings pass through unchanged.
func TestMaskEmail_NoAtSign(t *testing.T) {
	got := MaskEmail("not-an-email")
	want := "not-an-email"
	if got != want {
		t.Errorf("MaskEmail() = %q, want %q", got, want)
	}
}

// TestMaskEmail_EmptyString verifies that an empty string passes through.
func TestMaskEmail_EmptyString(t *testing.T) {
	got := MaskEmail("")
	if got != "" {
		t.Errorf("MaskEmail(\"\") = %q, want \"\"", got)
	}
}

// TestMaskEmail_LongLocal verifies masking with a long local part preserves
// only the first two characters.
func TestMaskEmail_LongLocal(t *testing.T) {
	got := MaskEmail("longusername@domain.org")
	want := "lo***@domain.org"
	if got != want {
		t.Errorf("MaskEmail() = %q, want %q", got, want)
	}
}

// TestSanitizeLogValue_BodyKey verifies that body content is always redacted.
func TestSanitizeLogValue_BodyKey(t *testing.T) {
	got := SanitizeLogValue("body", "<html>secret</html>")
	if got != "[body redacted]" {
		t.Errorf("SanitizeLogValue(body) = %q, want %q", got, "[body redacted]")
	}
}

// TestSanitizeLogValue_BodyPreviewKey verifies that bodyPreview triggers redaction.
func TestSanitizeLogValue_BodyPreviewKey(t *testing.T) {
	got := SanitizeLogValue("bodyPreview", "meeting notes...")
	if got != "[body redacted]" {
		t.Errorf("SanitizeLogValue(bodyPreview) = %q, want %q", got, "[body redacted]")
	}
}

// TestSanitizeLogValue_BodyContentKey verifies that body_content triggers redaction.
func TestSanitizeLogValue_BodyContentKey(t *testing.T) {
	got := SanitizeLogValue("body_content", "<p>text</p>")
	if got != "[body redacted]" {
		t.Errorf("SanitizeLogValue(body_content) = %q, want %q", got, "[body redacted]")
	}
}

// TestSanitizeLogValue_BodyPreviewSnakeCase verifies that body_preview key
// triggers redaction.
func TestSanitizeLogValue_BodyPreviewSnakeCase(t *testing.T) {
	got := SanitizeLogValue("body_preview", "preview text")
	if got != "[body redacted]" {
		t.Errorf("SanitizeLogValue(body_preview) = %q, want %q", got, "[body redacted]")
	}
}

// TestSanitizeLogValue_CredentialKeys verifies that credential values are
// fully redacted for all recognized credential key names.
func TestSanitizeLogValue_CredentialKeys(t *testing.T) {
	keys := []string{"authorization", "Authorization", "token", "access_token", "refresh_token", "password"}
	for _, key := range keys {
		got := SanitizeLogValue(key, "eyJhbGciOiJSUzI1NiJ9")
		if got != "[REDACTED]" {
			t.Errorf("SanitizeLogValue(%q) = %q, want %q", key, got, "[REDACTED]")
		}
	}
}

// TestSanitizeLogValue_EmailInValue verifies that emails in arbitrary values
// are masked.
func TestSanitizeLogValue_EmailInValue(t *testing.T) {
	got := SanitizeLogValue("error", "mailbox alice@contoso.com not found")
	want := "mailbox al***@contoso.com not found"
	if got != want {
		t.Errorf("SanitizeLogValue() = %q, want %q", got, want)
	}
}

// TestSanitizeLogValue_MultipleEmails verifies that multiple emails in one
// value are all masked.
func TestSanitizeLogValue_MultipleEmails(t *testing.T) {
	got := SanitizeLogValue("attendees", "alice@a.com, bob@b.com")
	want := "al***@a.com, bo***@b.com"
	if got != want {
		t.Errorf("SanitizeLogValue() = %q, want %q", got, want)
	}
}

// TestSanitizeLogValue_Truncation verifies that long values are truncated at
// 200 characters.
func TestSanitizeLogValue_Truncation(t *testing.T) {
	value := strings.Repeat("x", 250)
	got := SanitizeLogValue("description", value)
	if len(got) != 200+len("...[truncated]") {
		t.Errorf("truncated length = %d, want %d", len(got), 200+len("...[truncated]"))
	}
	if !strings.HasSuffix(got, "...[truncated]") {
		t.Errorf("expected ...[truncated] suffix, got %q", got[len(got)-20:])
	}
}

// TestSanitizeLogValue_NoTruncationUnder200 verifies that values under 200
// chars are not truncated.
func TestSanitizeLogValue_NoTruncationUnder200(t *testing.T) {
	got := SanitizeLogValue("subject", "Team meeting")
	if got != "Team meeting" {
		t.Errorf("SanitizeLogValue() = %q, want %q", got, "Team meeting")
	}
}

// TestSanitizeLogValue_ScriptTag verifies that script tag detection prepends
// a warning prefix.
func TestSanitizeLogValue_ScriptTag(t *testing.T) {
	got := SanitizeLogValue("content", "<script>alert(1)</script>")
	if !strings.HasPrefix(got, "[WARNING: script content detected] ") {
		t.Errorf("expected script warning prefix, got %q", got)
	}
}

// TestSanitizeLogValue_PlainValue verifies that normal values pass through
// unchanged.
func TestSanitizeLogValue_PlainValue(t *testing.T) {
	got := SanitizeLogValue("event_id", "AAMkAGI2TG93")
	if got != "AAMkAGI2TG93" {
		t.Errorf("SanitizeLogValue() = %q, want %q", got, "AAMkAGI2TG93")
	}
}

// TestSanitizingHandler_MasksEmail verifies that the SanitizingHandler masks
// email addresses in slog string attributes.
func TestSanitizingHandler_MasksEmail(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	handler := &SanitizingHandler{inner: inner}
	logger := slog.New(handler)

	logger.Info("test", "organizer", "alice@contoso.com")

	output := buf.String()
	if strings.Contains(output, "alice@contoso.com") {
		t.Errorf("log output should not contain raw email, got %s", output)
	}
	if !strings.Contains(output, "al***@contoso.com") {
		t.Errorf("expected masked email in output, got %s", output)
	}
}

// TestSanitizingHandler_RedactsBody verifies that the SanitizingHandler replaces
// body content with "[body redacted]".
func TestSanitizingHandler_RedactsBody(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	handler := &SanitizingHandler{inner: inner}
	logger := slog.New(handler)

	logger.Info("test", "body", "<html>secret content</html>")

	output := buf.String()
	if strings.Contains(output, "secret content") {
		t.Errorf("log output should not contain body content, got %s", output)
	}
	if !strings.Contains(output, "[body redacted]") {
		t.Errorf("expected [body redacted] in output, got %s", output)
	}
}

// TestSanitizingHandler_Enabled verifies that Enabled delegates to the inner
// handler.
func TestSanitizingHandler_Enabled(t *testing.T) {
	inner := slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelWarn})
	handler := &SanitizingHandler{inner: inner}

	if handler.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("expected info to be disabled at warn level")
	}
	if !handler.Enabled(context.Background(), slog.LevelWarn) {
		t.Error("expected warn to be enabled at warn level")
	}
}

// TestSanitizingHandler_WithAttrs verifies that WithAttrs returns a handler
// that still sanitizes.
func TestSanitizingHandler_WithAttrs(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	handler := &SanitizingHandler{inner: inner}
	childHandler := handler.WithAttrs([]slog.Attr{slog.String("user", "bob@example.com")})

	logger := slog.New(childHandler)
	logger.Info("test")

	output := buf.String()
	if strings.Contains(output, "bob@example.com") {
		t.Errorf("WithAttrs output should not contain raw email, got %s", output)
	}
}

// TestSanitizingHandler_WithGroup verifies that WithGroup returns a handler
// that still sanitizes.
func TestSanitizingHandler_WithGroup(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	handler := &SanitizingHandler{inner: inner}
	groupHandler := handler.WithGroup("ctx")

	logger := slog.New(groupHandler)
	logger.Info("test", "email", "test@example.com")

	output := buf.String()
	if strings.Contains(output, "test@example.com") {
		t.Errorf("WithGroup output should not contain raw email, got %s", output)
	}
}

// TestInitLogger_SanitizeEnabled verifies that InitLogger wraps the handler
// with SanitizingHandler when sanitize is true.
func TestInitLogger_SanitizeEnabled(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error: %v", err)
	}

	origStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = origStderr })

	InitLogger("info", "json", true, "")
	slog.Info("sanitize test", "organizer", "alice@contoso.com")

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("JSON parse error: %v\noutput: %s", err, buf.String())
	}

	organizer, _ := record["organizer"].(string)
	if strings.Contains(organizer, "alice@contoso.com") {
		t.Errorf("expected masked email with sanitize=true, got %q", organizer)
	}
}

// TestInitLogger_SanitizeDisabled verifies that InitLogger does not apply the
// SanitizingHandler when sanitize is false.
func TestInitLogger_SanitizeDisabled(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error: %v", err)
	}

	origStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = origStderr })

	InitLogger("info", "json", false, "")
	slog.Info("no sanitize test", "organizer", "alice@contoso.com")

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("JSON parse error: %v\noutput: %s", err, buf.String())
	}

	organizer, _ := record["organizer"].(string)
	if !strings.Contains(organizer, "alice@contoso.com") {
		t.Errorf("expected raw email with sanitize=false, got %q", organizer)
	}
}
