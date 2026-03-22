package logging

import (
	"context"
	"log/slog"
	"regexp"
	"strings"
)

// emailRegex matches standard email address patterns in arbitrary strings.
// It is compiled once at package initialization time to avoid per-call
// compilation overhead. The pattern covers common Graph API email formats
// (user@domain.tld) without attempting full RFC 5322 compliance.
var emailRegex = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)

// credentialKeys is the set of log attribute key names whose values contain
// credential material and must be fully redacted. The check is
// case-insensitive via strings.ToLower on the key.
var credentialKeys = map[string]bool{
	"authorization": true,
	"token":         true,
	"access_token":  true,
	"refresh_token": true,
	"password":      true,
}

// bodyKeys is the set of log attribute key names whose values contain event
// body content that must never appear in logs. The check is exact match.
var bodyKeys = map[string]bool{
	"body":         true,
	"bodyPreview":  true,
	"body_content": true,
	"body_preview": true,
}

// maxLogValueLen is the maximum number of characters allowed in a sanitized
// log value before truncation is applied.
const maxLogValueLen = 200

// MaskEmail transforms an email address by preserving the first 2 characters
// of the local part and replacing the remainder with "***". The full domain
// is preserved. If the input does not contain an "@" character, it is returned
// unchanged.
//
// Parameters:
//   - email: the email address string to mask.
//
// Returns the masked email string, or the original string if it contains no "@".
//
// MaskEmail is a pure function safe for concurrent use.
func MaskEmail(email string) string {
	at := strings.Index(email, "@")
	if at < 0 {
		return email
	}

	local := email[:at]
	domain := email[at+1:]

	preserved := local
	if len(local) > 2 {
		preserved = local[:2]
	}

	return preserved + "***@" + domain
}

// SanitizeLogValue applies field-aware sanitization to a log attribute value
// based on the attribute key name. The sanitization rules are applied in order:
//
//  1. Credential keys (authorization, token, access_token, refresh_token,
//     password) -> "[REDACTED]"
//  2. Body keys (body, bodyPreview, body_content, body_preview) ->
//     "[body redacted]"
//  3. Email addresses in the value are masked via MaskEmail.
//  4. HTML script tags trigger a "[WARNING: script content detected] " prefix.
//  5. Values exceeding 200 characters are truncated with "...[truncated]".
//
// Parameters:
//   - key: the log attribute key name used for field-aware rule selection.
//   - value: the string value to sanitize.
//
// Returns the sanitized string.
//
// SanitizeLogValue is a pure function safe for concurrent use.
func SanitizeLogValue(key, value string) string {
	lower := strings.ToLower(key)

	if credentialKeys[lower] {
		return "[REDACTED]"
	}

	if bodyKeys[key] {
		return "[body redacted]"
	}

	result := emailRegex.ReplaceAllStringFunc(value, MaskEmail)

	if strings.Contains(strings.ToLower(result), "<script") {
		result = "[WARNING: script content detected] " + result
	}

	if len(result) > maxLogValueLen {
		result = result[:maxLogValueLen] + "...[truncated]"
	}

	return result
}

// SanitizingHandler wraps an slog.Handler to automatically sanitize all string
// attribute values in log records before they reach the underlying handler.
// Email addresses are masked, credential values are redacted, and body content
// is replaced. The wrapper is transparent: non-string attributes pass through
// unchanged.
//
// SanitizingHandler is safe for concurrent use because it delegates all state
// management to the underlying handler and uses only pure sanitization functions.
type SanitizingHandler struct {
	inner slog.Handler
}

// Enabled reports whether the underlying handler is enabled for the given level.
// It delegates directly to the inner handler.
//
// Parameters:
//   - ctx: the context for the log record.
//   - level: the log level to check.
//
// Returns true if the inner handler is enabled for the given level.
func (h *SanitizingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// Handle sanitizes all string attributes in the log record and delegates to
// the inner handler. A new record is created with the same time, level, message,
// and PC but with sanitized attributes. Non-string attributes pass through
// unchanged.
//
// Parameters:
//   - ctx: the context for the log record.
//   - r: the log record to sanitize and forward.
//
// Returns any error from the inner handler's Handle method.
func (h *SanitizingHandler) Handle(ctx context.Context, r slog.Record) error {
	newRecord := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		if a.Value.Kind() == slog.KindString {
			a.Value = slog.StringValue(SanitizeLogValue(a.Key, a.Value.String()))
		}
		newRecord.AddAttrs(a)
		return true
	})
	return h.inner.Handle(ctx, newRecord)
}

// WithAttrs returns a new SanitizingHandler wrapping the inner handler's
// WithAttrs result. Pre-attached attributes are sanitized here because they
// bypass the Handle method and would otherwise reach the underlying handler
// unsanitized.
//
// Parameters:
//   - attrs: the attributes to attach to the new handler.
//
// Returns a new SanitizingHandler wrapping the inner handler with the given attrs.
func (h *SanitizingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	sanitized := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		if a.Value.Kind() == slog.KindString {
			a.Value = slog.StringValue(SanitizeLogValue(a.Key, a.Value.String()))
		}
		sanitized[i] = a
	}
	return &SanitizingHandler{inner: h.inner.WithAttrs(sanitized)}
}

// WithGroup returns a new SanitizingHandler wrapping the inner handler's
// WithGroup result. This preserves sanitization for loggers created with
// log groups.
//
// Parameters:
//   - name: the group name.
//
// Returns a new SanitizingHandler wrapping the inner handler with the given group.
func (h *SanitizingHandler) WithGroup(name string) slog.Handler {
	return &SanitizingHandler{inner: h.inner.WithGroup(name)}
}
