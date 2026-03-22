package logging

import (
	"context"
	"log/slog"
)

// MultiHandler is an slog.Handler that fans out each log record to two inner
// handlers: a primary handler (typically stderr) and a secondary handler
// (typically a log file). Both handlers receive identical records with the same
// attributes and groups.
//
// If the secondary handler returns an error, the primary handler still processes
// the record and a best-effort warning is emitted to the primary handler. The
// primary handler's error is returned to the caller.
//
// MultiHandler is safe for concurrent use from multiple goroutines, as required
// by the slog.Handler interface contract. It delegates all synchronization to
// the inner handlers and holds no mutable state of its own.
type MultiHandler struct {
	// primary is the main output handler (e.g., stderr). Its error is returned
	// from Handle and it receives best-effort warnings about secondary failures.
	primary slog.Handler

	// secondary is the auxiliary output handler (e.g., log file). Errors from
	// this handler are reported to primary but do not prevent primary processing.
	secondary slog.Handler
}

// NewMultiHandler creates a MultiHandler that fans out log records to both the
// primary and secondary handlers.
//
// Parameters:
//   - primary: the main handler whose error is returned from Handle.
//   - secondary: the auxiliary handler; its errors are reported but non-fatal.
//
// Returns a new MultiHandler ready for use as an slog.Handler.
func NewMultiHandler(primary, secondary slog.Handler) *MultiHandler {
	return &MultiHandler{
		primary:   primary,
		secondary: secondary,
	}
}

// Enabled reports whether either inner handler is enabled for the given level.
// This ensures log records are processed if at least one handler wants them.
//
// Parameters:
//   - ctx: the context for the log check.
//   - level: the log level to check.
//
// Returns true if either the primary or secondary handler is enabled.
func (h *MultiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.primary.Enabled(ctx, level) || h.secondary.Enabled(ctx, level)
}

// Handle sends the log record to both inner handlers. The primary handler is
// always called. If the secondary handler returns an error, a best-effort
// warning is emitted to the primary handler. The primary handler's error is
// returned to the caller.
//
// Parameters:
//   - ctx: the context for the log record.
//   - r: the log record to fan out to both handlers.
//
// Returns the primary handler's error, or nil if primary succeeds.
//
// Side effects: writes the record to both inner handlers. On secondary failure,
// emits a warning record to the primary handler.
func (h *MultiHandler) Handle(ctx context.Context, r slog.Record) error {
	if err := h.secondary.Handle(ctx, r); err != nil {
		// Best-effort warning to primary about secondary failure.
		warnRecord := slog.NewRecord(r.Time, slog.LevelWarn, "secondary log handler failed", 0)
		warnRecord.AddAttrs(slog.String("error", err.Error()))
		_ = h.primary.Handle(ctx, warnRecord)
	}

	return h.primary.Handle(ctx, r)
}

// WithAttrs returns a new MultiHandler with the given attributes applied to
// both inner handlers. This ensures persistent attributes propagate to all
// output destinations.
//
// Parameters:
//   - attrs: the attributes to attach to both inner handlers.
//
// Returns a new MultiHandler wrapping the attributed inner handlers.
func (h *MultiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &MultiHandler{
		primary:   h.primary.WithAttrs(attrs),
		secondary: h.secondary.WithAttrs(attrs),
	}
}

// WithGroup returns a new MultiHandler with the given group name applied to
// both inner handlers. This ensures log groups propagate to all output
// destinations.
//
// Parameters:
//   - name: the group name to apply to both inner handlers.
//
// Returns a new MultiHandler wrapping the grouped inner handlers.
func (h *MultiHandler) WithGroup(name string) slog.Handler {
	return &MultiHandler{
		primary:   h.primary.WithGroup(name),
		secondary: h.secondary.WithGroup(name),
	}
}
