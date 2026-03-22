package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestInitLoggerDefaultLevel verifies that an unrecognized level string results
// in the default warn level.
func TestInitLoggerDefaultLevel(t *testing.T) {
	InitLogger("bogus", "json", false, "")

	if !slog.Default().Enabled(context.TODO(), slog.LevelWarn) {
		t.Error("expected warn level to be enabled for bogus input")
	}
	if slog.Default().Enabled(context.TODO(), slog.LevelInfo) {
		t.Error("expected info level to be disabled for bogus input")
	}
}

// TestInitLoggerDebugLevel verifies that "debug" correctly sets the debug level.
func TestInitLoggerDebugLevel(t *testing.T) {
	InitLogger("debug", "json", false, "")

	if !slog.Default().Enabled(context.TODO(), slog.LevelDebug) {
		t.Error("expected debug level to be enabled")
	}
}

// TestInitLoggerInfoLevel verifies that "info" correctly sets the info level.
func TestInitLoggerInfoLevel(t *testing.T) {
	InitLogger("info", "json", false, "")

	if !slog.Default().Enabled(context.TODO(), slog.LevelInfo) {
		t.Error("expected info level to be enabled")
	}
	if slog.Default().Enabled(context.TODO(), slog.LevelDebug) {
		t.Error("expected debug level to be disabled at info level")
	}
}

// TestInitLoggerWarnLevel verifies that "warn" correctly sets the warn level.
func TestInitLoggerWarnLevel(t *testing.T) {
	InitLogger("warn", "json", false, "")

	if !slog.Default().Enabled(context.TODO(), slog.LevelWarn) {
		t.Error("expected warn level to be enabled")
	}
	if slog.Default().Enabled(context.TODO(), slog.LevelInfo) {
		t.Error("expected info level to be disabled at warn level")
	}
}

// TestInitLoggerErrorLevel verifies that "error" correctly sets the error level.
func TestInitLoggerErrorLevel(t *testing.T) {
	InitLogger("error", "json", false, "")

	if !slog.Default().Enabled(context.TODO(), slog.LevelError) {
		t.Error("expected error level to be enabled")
	}
	if slog.Default().Enabled(context.TODO(), slog.LevelWarn) {
		t.Error("expected warn level to be disabled at error level")
	}
}

// TestInitLoggerCaseInsensitive verifies that level parsing is case-insensitive,
// for example "DEBUG" is treated the same as "debug".
func TestInitLoggerCaseInsensitive(t *testing.T) {
	InitLogger("DEBUG", "json", false, "")

	if !slog.Default().Enabled(context.TODO(), slog.LevelDebug) {
		t.Error("expected debug level to be enabled for uppercase DEBUG")
	}
}

// TestInitLoggerJSONFormat verifies that the JSON handler produces valid JSON
// output containing the required time, level, and msg fields.
func TestInitLoggerJSONFormat(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error: %v", err)
	}

	origStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = origStderr })

	InitLogger("info", "json", false, "")
	slog.Info("test message")

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}

	for _, field := range []string{"time", "level", "msg"} {
		if _, ok := record[field]; !ok {
			t.Errorf("JSON output missing %q field", field)
		}
	}
}

// TestInitLoggerTextFormat verifies that the text handler produces key=value
// formatted output.
func TestInitLoggerTextFormat(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error: %v", err)
	}

	origStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = origStderr })

	InitLogger("info", "text", false, "")
	slog.Info("test message")

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	output := buf.String()
	if !strings.Contains(output, "level=") {
		t.Errorf("expected key=value format with level= in output, got: %s", output)
	}
	if !strings.Contains(output, "msg=") {
		t.Errorf("expected key=value format with msg= in output, got: %s", output)
	}
}

// TestInitLoggerDefaultFormat verifies that an empty format string defaults to
// JSON output.
func TestInitLoggerDefaultFormat(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error: %v", err)
	}

	origStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = origStderr })

	InitLogger("info", "", false, "")
	slog.Info("test message")

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("empty format should produce JSON, got parse error: %v\noutput: %s", err, buf.String())
	}
}

// TestInitLoggerStderrOnly verifies that log output goes to stderr and nothing
// is written to stdout.
func TestInitLoggerStderrOnly(t *testing.T) {
	// Capture stdout.
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() for stdout error: %v", err)
	}
	origStdout := os.Stdout
	os.Stdout = stdoutW

	// Capture stderr.
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() for stderr error: %v", err)
	}
	origStderr := os.Stderr
	os.Stderr = stderrW

	t.Cleanup(func() {
		os.Stdout = origStdout
		os.Stderr = origStderr
	})

	InitLogger("info", "json", false, "")
	slog.Info("stderr test")

	_ = stdoutW.Close()
	_ = stderrW.Close()

	var stdoutBuf, stderrBuf bytes.Buffer
	_, _ = io.Copy(&stdoutBuf, stdoutR)
	_, _ = io.Copy(&stderrBuf, stderrR)

	if stdoutBuf.Len() != 0 {
		t.Errorf("expected no stdout output, got: %s", stdoutBuf.String())
	}
	if stderrBuf.Len() == 0 {
		t.Error("expected stderr output, got nothing")
	}
}

// TestInitLoggerAddSource verifies that AddSource: true causes the JSON output
// to include a "source" field with "file" and "line" subfields.
func TestInitLoggerAddSource(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error: %v", err)
	}

	origStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = origStderr })

	InitLogger("info", "json", false, "")
	slog.Info("source test")

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("JSON parse error: %v", err)
	}

	source, ok := record["source"]
	if !ok {
		t.Fatal("JSON output missing \"source\" field")
	}

	sourceMap, ok := source.(map[string]any)
	if !ok {
		t.Fatalf("source field is not an object: %T", source)
	}

	if _, ok := sourceMap["file"]; !ok {
		t.Error("source missing \"file\" subfield")
	}
	if _, ok := sourceMap["line"]; !ok {
		t.Error("source missing \"line\" subfield")
	}
}

// TestInitLoggerSetsDefault verifies that InitLogger calls slog.SetDefault so
// that slog.Default() returns the configured logger. After initialization at
// info level, slog.Default() must be enabled for info but not for debug.
func TestInitLoggerSetsDefault(t *testing.T) {
	InitLogger("info", "json", false, "")

	logger := slog.Default()
	if !logger.Enabled(context.TODO(), slog.LevelInfo) {
		t.Error("slog.Default() should be enabled at info level after InitLogger")
	}
	if logger.Enabled(context.TODO(), slog.LevelDebug) {
		t.Error("slog.Default() should not be enabled at debug level when configured for info")
	}
}

// TestLoggerWithContext verifies that slog.With adds persistent attributes to
// all log records emitted by the child logger.
func TestLoggerWithContext(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error: %v", err)
	}

	origStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = origStderr })

	InitLogger("info", "json", false, "")
	child := slog.With("tool", "list_events")
	child.Info("handler invoked")

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("JSON parse error: %v", err)
	}

	tool, ok := record["tool"]
	if !ok {
		t.Fatal("expected \"tool\" attribute in log record from slog.With child logger")
	}
	if tool != "list_events" {
		t.Errorf("tool = %v, want \"list_events\"", tool)
	}
}

// TestLogLevelGating verifies that messages below the configured threshold are
// suppressed. With level set to warn, debug messages must not appear in output.
func TestLogLevelGating(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error: %v", err)
	}

	origStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = origStderr })

	InitLogger("warn", "json", false, "")
	slog.Debug("should be suppressed")
	slog.Info("also suppressed")

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	if buf.Len() != 0 {
		t.Errorf("expected no output for debug/info at warn level, got: %s", buf.String())
	}
}

// TestInitLogger_FileLogging_WritesToFile verifies that log records appear in
// both stderr and the log file when a file path is provided.
func TestInitLogger_FileLogging_WritesToFile(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.log")

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error: %v", err)
	}
	origStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() {
		os.Stderr = origStderr
		CloseLogFile()
	})

	InitLogger("info", "json", false, tmpFile)
	slog.Info("file test message")

	_ = w.Close()
	var stderrBuf bytes.Buffer
	_, _ = io.Copy(&stderrBuf, r)

	CloseLogFile()

	fileContent, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	if !strings.Contains(stderrBuf.String(), "file test message") {
		t.Errorf("stderr missing message, got: %s", stderrBuf.String())
	}
	if !strings.Contains(string(fileContent), "file test message") {
		t.Errorf("file missing message, got: %s", string(fileContent))
	}
}

// TestInitLogger_FileLogging_JSONFormat verifies that file output is valid JSON
// when the format is "json".
func TestInitLogger_FileLogging_JSONFormat(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.log")

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error: %v", err)
	}
	origStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() {
		os.Stderr = origStderr
		CloseLogFile()
	})

	InitLogger("info", "json", false, tmpFile)
	slog.Info("json format test")

	_ = w.Close()
	_, _ = io.Copy(io.Discard, r)
	CloseLogFile()

	fileContent, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	var record map[string]any
	if err := json.Unmarshal(fileContent, &record); err != nil {
		t.Fatalf("file output is not valid JSON: %v\noutput: %s", err, string(fileContent))
	}
}

// TestInitLogger_FileLogging_TextFormat verifies that file output is key=value
// format when the format is "text".
func TestInitLogger_FileLogging_TextFormat(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.log")

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error: %v", err)
	}
	origStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() {
		os.Stderr = origStderr
		CloseLogFile()
	})

	InitLogger("info", "text", false, tmpFile)
	slog.Info("text format test")

	_ = w.Close()
	_, _ = io.Copy(io.Discard, r)
	CloseLogFile()

	fileContent, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	output := string(fileContent)
	if !strings.Contains(output, "level=") {
		t.Errorf("expected key=value format with level= in file output, got: %s", output)
	}
	if !strings.Contains(output, "msg=") {
		t.Errorf("expected key=value format with msg= in file output, got: %s", output)
	}
}

// TestInitLogger_FileLogging_InvalidPath verifies that an invalid file path
// results in graceful degradation: an error is logged to stderr and the logger
// continues with stderr-only output.
func TestInitLogger_FileLogging_InvalidPath(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error: %v", err)
	}
	origStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() {
		os.Stderr = origStderr
		CloseLogFile()
	})

	InitLogger("info", "json", false, "/nonexistent/dir/test.log")
	slog.Info("fallback test")

	_ = w.Close()
	var stderrBuf bytes.Buffer
	_, _ = io.Copy(&stderrBuf, r)

	output := stderrBuf.String()
	if !strings.Contains(output, "log file open failed") {
		t.Errorf("expected error about log file open failure, got: %s", output)
	}
	if !strings.Contains(output, "fallback test") {
		t.Errorf("expected fallback stderr logging to work, got: %s", output)
	}

	if logFile != nil {
		t.Error("logFile should be nil after failed open")
	}
}

// TestInitLogger_FileLogging_EmptyPath verifies that an empty file path results
// in no file being opened, identical to the current behavior.
func TestInitLogger_FileLogging_EmptyPath(t *testing.T) {
	t.Cleanup(func() { CloseLogFile() })

	InitLogger("info", "json", false, "")

	if logFile != nil {
		t.Error("logFile should be nil when filePath is empty")
	}
}

// TestInitLogger_FileLogging_SanitizationApplied verifies that PII is masked
// in file output when sanitize is true.
func TestInitLogger_FileLogging_SanitizationApplied(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.log")

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error: %v", err)
	}
	origStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() {
		os.Stderr = origStderr
		CloseLogFile()
	})

	InitLogger("info", "json", true, tmpFile)
	slog.Info("sanitize test", "email", "user@example.com")

	_ = w.Close()
	_, _ = io.Copy(io.Discard, r)
	CloseLogFile()

	fileContent, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	output := string(fileContent)
	if strings.Contains(output, "user@example.com") {
		t.Errorf("file output should have masked email, got: %s", output)
	}
	if !strings.Contains(output, "us***@example.com") {
		t.Errorf("file output should contain masked email us***@example.com, got: %s", output)
	}
}

// TestCloseLogFile_NoFile verifies that CloseLogFile is a no-op when no file
// logging is configured.
func TestCloseLogFile_NoFile(t *testing.T) {
	logFile = nil
	CloseLogFile() // Should not panic or error.
}

// TestCloseLogFile_FlushesAndCloses verifies that CloseLogFile syncs and
// closes the file, and sets logFile to nil.
func TestCloseLogFile_FlushesAndCloses(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "close-test.log")

	f, err := os.OpenFile(tmpFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	logFile = f
	CloseLogFile()

	if logFile != nil {
		t.Error("logFile should be nil after CloseLogFile")
	}

	// Verify the file is closed by attempting to write; should fail.
	_, err = f.WriteString("after close")
	if err == nil {
		t.Error("expected error writing to closed file")
	}
}

// TestInitLogger_FileLogging_FilePermissions verifies that the log file
// created by InitLogger has permissions 0600.
func TestInitLogger_FileLogging_FilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "perms-test.log")

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error: %v", err)
	}
	origStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() {
		os.Stderr = origStderr
		CloseLogFile()
	})

	InitLogger("info", "json", false, tmpFile)
	slog.Info("permissions test")

	_ = w.Close()
	_, _ = io.Copy(io.Discard, r)
	CloseLogFile()

	info, err := os.Stat(tmpFile)
	if err != nil {
		t.Fatalf("failed to stat log file: %v", err)
	}

	mode := info.Mode().Perm()
	if mode != 0600 {
		t.Errorf("expected file permissions 0600, got %04o", mode)
	}
}

// TestInitLogger_FileLogging_StartupLogField verifies that a log entry with
// a "log_file" field can be emitted and appears in the file output.
func TestInitLogger_FileLogging_StartupLogField(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "startup-test.log")

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error: %v", err)
	}
	origStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() {
		os.Stderr = origStderr
		CloseLogFile()
	})

	InitLogger("info", "json", false, tmpFile)
	slog.Info("server starting", "log_file", tmpFile)

	_ = w.Close()
	_, _ = io.Copy(io.Discard, r)
	CloseLogFile()

	fileContent, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	output := string(fileContent)
	if !strings.Contains(output, "log_file") {
		t.Errorf("file output should contain 'log_file' key, got: %s", output)
	}
	if !strings.Contains(output, tmpFile) {
		t.Errorf("file output should contain the log file path, got: %s", output)
	}
}
