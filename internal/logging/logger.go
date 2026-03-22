package logging

import (
	"log/slog"
	"os"
	"strings"
)

// logFile holds the open file handle for file logging, or nil when file logging
// is not active. It is set by InitLogger and cleared by CloseLogFile.
var logFile *os.File

// newHandler creates an slog.Handler for the given writer using the specified
// format and handler options.
//
// Parameters:
//   - w: the output writer (e.g., os.Stderr or a file).
//   - format: "text" for text format, anything else for JSON.
//   - opts: the slog handler options.
//
// Returns a configured slog.Handler.
func newHandler(w *os.File, format string, opts *slog.HandlerOptions) slog.Handler {
	if strings.ToLower(format) == "text" {
		return slog.NewTextHandler(w, opts)
	}
	return slog.NewJSONHandler(w, opts)
}

// InitLogger initializes the process-wide default slog logger with the given
// log level, output format, optional sanitization wrapper, and optional file
// output. It must be called as the first operation in main() after loading
// configuration, before any code that might produce log output.
//
// Parameters:
//   - levelStr: the minimum log level as a string. Accepts "debug", "info",
//     "warn", or "error" (case-insensitive). Any unrecognized value defaults
//     to "warn".
//   - format: the structured log output format. Accepts "json" or "text".
//     An empty string defaults to JSON format.
//   - sanitize: when true, wraps the underlying handler with SanitizingHandler
//     to mask PII in all log output.
//   - filePath: optional filesystem path for log file output. When non-empty,
//     log records are written to both os.Stderr and the specified file via a
//     MultiHandler. When empty, only stderr logging is active.
//
// Side effects:
//   - Calls slog.SetDefault to set the process-wide default logger.
//   - All log output is written to os.Stderr; os.Stdout is never used.
//   - When filePath is non-empty, opens the file in append mode (O_APPEND|
//     O_CREATE|O_WRONLY) with permissions 0600 and stores the handle for
//     later cleanup via CloseLogFile.
//   - When the file cannot be opened, logs an error to stderr and continues
//     with stderr-only logging (graceful degradation).
func InitLogger(levelStr, format string, sanitize bool, filePath string) {
	level := ParseLogLevel(levelStr)

	opts := &slog.HandlerOptions{
		AddSource: true,
		Level:     level,
	}

	stderrHandler := newHandler(os.Stderr, format, opts)

	handler := stderrHandler

	if filePath != "" {
		f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			// Log via the stderr handler we just created (not slog.Default, which
			// may still target a previous handler from before os.Stderr was set).
			slog.New(stderrHandler).Error("log file open failed", "path", filePath, "error", err)
		} else {
			logFile = f
			fileHandler := newHandler(f, format, opts)
			handler = NewMultiHandler(stderrHandler, fileHandler)
		}
	}

	if sanitize {
		handler = &SanitizingHandler{inner: handler}
	}

	slog.SetDefault(slog.New(handler))
}

// CloseLogFile flushes (Sync) and closes the log file handle opened by
// InitLogger. It is a no-op when file logging is not active (logFile is nil).
// Safe to call multiple times; subsequent calls after the first are no-ops.
//
// Side effects: syncs and closes the log file handle, then sets the module-level
// logFile variable to nil.
func CloseLogFile() {
	if logFile == nil {
		return
	}
	_ = logFile.Sync()
	_ = logFile.Close()
	logFile = nil
}

// ParseLogLevel converts a log level string to the corresponding slog.Level.
// The parsing is case-insensitive. Unrecognized values default to slog.LevelWarn.
//
// Parameters:
//   - s: the log level string to parse.
//
// Returns the corresponding slog.Level value.
func ParseLogLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelWarn
	}
}
