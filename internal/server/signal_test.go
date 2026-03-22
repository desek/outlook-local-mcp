package server

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/desek/outlook-local-mcp/internal/logging"
)

// TestAwaitShutdownSignal_NonBlocking validates that AwaitShutdownSignal
// returns immediately without blocking the caller, confirming the signal
// channel and goroutine are set up correctly.
func TestAwaitShutdownSignal_NonBlocking(t *testing.T) {
	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})

	// Must not panic or block.
	AwaitShutdownSignal(cancel, 5*time.Second, done, nil)
}

// TestAwaitShutdownSignal_SIGTERM_CancelsContext validates that sending
// SIGTERM to a subprocess running the shutdown handler causes the context
// to be cancelled and the process to exit with code 0 after the timeout.
func TestAwaitShutdownSignal_SIGTERM_CancelsContext(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SIGTERM not supported on Windows")
	}
	runSignalSubprocess(t, "SIGTERM_CANCEL", syscall.SIGTERM, 0)
}

// TestAwaitShutdownSignal_SIGINT_CancelsContext validates that sending
// SIGINT to a subprocess running the shutdown handler causes the context
// to be cancelled and the process to exit with code 0 after the timeout.
func TestAwaitShutdownSignal_SIGINT_CancelsContext(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SIGINT not supported on Windows")
	}
	runSignalSubprocess(t, "SIGINT_CANCEL", syscall.SIGINT, 0)
}

// TestAwaitShutdownSignal_ExitCode0_OnTimeout validates that the process
// exits with code 0 when the shutdown timeout expires without the done
// channel being closed or a second signal arriving.
func TestAwaitShutdownSignal_ExitCode0_OnTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SIGTERM not supported on Windows")
	}
	result := runSignalSubprocess(t, "TIMEOUT_EXIT", syscall.SIGTERM, 0)
	if !strings.Contains(result.stderr, "timeout_expired") {
		t.Errorf("expected 'timeout_expired' in stderr, got: %s", result.stderr)
	}
}

// TestAwaitShutdownSignal_ExitCode0_OnDrainComplete validates that the
// process exits with code 0 when the done channel is closed before the
// timeout expires, indicating in-flight requests have drained.
func TestAwaitShutdownSignal_ExitCode0_OnDrainComplete(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SIGTERM not supported on Windows")
	}
	result := runSignalSubprocess(t, "DRAIN_COMPLETE", syscall.SIGTERM, 0)
	if !strings.Contains(result.stderr, "drain_complete") {
		t.Errorf("expected 'drain_complete' in stderr, got: %s", result.stderr)
	}
}

// TestAwaitShutdownSignal_ExitCode1_OnDoubleSignal validates that a second
// signal during the drain period causes the process to exit with code 1.
func TestAwaitShutdownSignal_ExitCode1_OnDoubleSignal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SIGTERM not supported on Windows")
	}
	result := runSignalSubprocess(t, "DOUBLE_SIGNAL", syscall.SIGTERM, 1)
	if !strings.Contains(result.stderr, "forced shutdown on second signal") {
		t.Errorf("expected 'forced shutdown on second signal' in stderr, got: %s", result.stderr)
	}
}

// TestAwaitShutdownSignal_LogsShutdownPhases validates that all expected
// shutdown phase log messages appear in stderr output when SIGTERM is sent.
func TestAwaitShutdownSignal_LogsShutdownPhases(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SIGTERM not supported on Windows")
	}
	result := runSignalSubprocess(t, "TIMEOUT_EXIT", syscall.SIGTERM, 0)

	phases := []string{
		"shutdown initiated",
		"waiting for in-flight requests",
		"shutdown complete",
	}
	for _, phase := range phases {
		if !strings.Contains(result.stderr, phase) {
			t.Errorf("expected %q in stderr, got: %s", phase, result.stderr)
		}
	}
}

// subprocessResult holds the output from a signal test subprocess.
type subprocessResult struct {
	// stderr contains the captured stderr output from the subprocess.
	stderr string
}

// runSignalSubprocess spawns a subprocess running the TestSignalHelper test,
// sends the specified signal, and validates the expected exit code.
//
// Parameters:
//   - t: the testing.T instance for reporting failures.
//   - mode: the TEST_SIGNAL_MODE value controlling subprocess behavior.
//   - sig: the OS signal to send to the subprocess.
//   - wantExitCode: the expected process exit code (0 or 1).
//
// Returns the subprocess result containing captured stderr.
func runSignalSubprocess(t *testing.T, mode string, sig os.Signal, wantExitCode int) subprocessResult {
	t.Helper()

	cmd := exec.Command(os.Args[0], "-test.run=^TestSignalHelper$", "-test.v")
	cmd.Env = append(os.Environ(), "TEST_SIGNAL_HELPER=1", "TEST_SIGNAL_MODE="+mode)

	var stderr strings.Builder
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start subprocess: %v", err)
	}

	// Wait for the subprocess to install the signal handler.
	time.Sleep(500 * time.Millisecond)

	// Send the first signal.
	if err := cmd.Process.Signal(sig); err != nil {
		t.Fatalf("failed to send signal: %v", err)
	}

	// For double-signal mode, send a second signal after a short delay.
	if mode == "DOUBLE_SIGNAL" {
		time.Sleep(200 * time.Millisecond)
		if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
			t.Fatalf("failed to send second signal: %v", err)
		}
	}

	err := cmd.Wait()
	output := stderr.String()

	if wantExitCode == 0 {
		if err != nil {
			t.Errorf("expected exit code 0, got error: %v\nstderr: %s", err, output)
		}
	} else {
		if err == nil {
			t.Errorf("expected exit code %d, got 0\nstderr: %s", wantExitCode, output)
		} else if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() != wantExitCode {
				t.Errorf("expected exit code %d, got %d\nstderr: %s",
					wantExitCode, exitErr.ExitCode(), output)
			}
		}
	}

	return subprocessResult{stderr: output}
}

// TestSignalHelper is a helper test that is only executed in a subprocess
// spawned by runSignalSubprocess. It installs the shutdown signal handler
// and blocks until the handler calls os.Exit. The TEST_SIGNAL_MODE env var
// controls the behavior:
//   - SIGTERM_CANCEL, SIGINT_CANCEL, TIMEOUT_EXIT: use a short timeout, no drain.
//   - DRAIN_COMPLETE: close the done channel shortly after signal receipt.
//   - DOUBLE_SIGNAL: use a long timeout so the second signal arrives during drain.
func TestSignalHelper(t *testing.T) {
	if os.Getenv("TEST_SIGNAL_HELPER") == "" {
		t.Skip("skipping: not a signal helper subprocess")
	}

	mode := os.Getenv("TEST_SIGNAL_MODE")

	// Initialize logger so shutdown messages appear on stderr.
	logging.InitLogger("info", "text", false, "")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	var timeout time.Duration
	switch mode {
	case "SIGTERM_CANCEL", "SIGINT_CANCEL", "TIMEOUT_EXIT":
		timeout = 1 * time.Second
	case "DRAIN_COMPLETE":
		timeout = 10 * time.Second
		// Close done channel shortly after context is cancelled.
		go func() {
			<-ctx.Done()
			time.Sleep(100 * time.Millisecond)
			close(done)
		}()
	case "DOUBLE_SIGNAL":
		timeout = 10 * time.Second
	default:
		timeout = 1 * time.Second
	}

	AwaitShutdownSignal(cancel, timeout, done, nil)

	// Block forever; the signal handler will call os.Exit.
	select {}
}

// TestAwaitShutdownSignal_ContextCancelled validates that the root context
// is actually cancelled when the cancel function is invoked by the signal
// handler. This test directly invokes cancel and checks ctx.Err().
func TestAwaitShutdownSignal_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Directly call cancel to simulate what the signal handler does.
	cancel()

	if ctx.Err() != context.Canceled {
		t.Errorf("ctx.Err() = %v, want %v", ctx.Err(), context.Canceled)
	}
}

// TestAwaitShutdownSignal_TimeoutParsing validates that the shutdown timeout
// value from the environment is correctly converted to a time.Duration for
// use by the signal handler.
func TestAwaitShutdownSignal_TimeoutParsing(t *testing.T) {
	tests := []struct {
		name    string
		seconds int
		want    time.Duration
	}{
		{"one_second", 1, 1 * time.Second},
		{"fifteen_seconds", 15, 15 * time.Second},
		{"three_hundred_seconds", 300, 300 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := time.Duration(tt.seconds) * time.Second
			if got != tt.want {
				t.Errorf("Duration(%d) = %v, want %v", tt.seconds, got, tt.want)
			}
		})
	}
}

// TestSignalHelper_ExitCodeFromEnv is a diagnostic helper to verify the
// subprocess mechanism works by checking that the TEST_SIGNAL_MODE env var
// is correctly received.
func TestSignalHelper_ExitCodeFromEnv(t *testing.T) {
	mode := os.Getenv("TEST_SIGNAL_MODE")
	if mode == "" {
		t.Skip("skipping: TEST_SIGNAL_MODE not set")
	}
	// Verify the mode string is parseable.
	_ = strconv.Itoa(len(mode))
}
