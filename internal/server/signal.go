package server

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// AwaitShutdownSignal installs an OS signal handler that listens for SIGINT
// and SIGTERM. On the first signal it cancels the root context via cancel,
// waits for in-flight requests to drain (or a configurable timeout to expire),
// then calls shutdownOTEL to flush pending telemetry before exiting.
//
// The function returns immediately (non-blocking). All shutdown logic runs in
// a background goroutine.
//
// Parameters:
//   - cancel: the CancelFunc for the root context, called on the first signal.
//   - timeout: maximum duration to wait for in-flight requests after cancel.
//   - done: channel that is closed when ServeStdio returns, signaling drain.
//   - shutdownOTEL: function that flushes and shuts down OTEL providers. Called
//     with a 5-second timeout context before os.Exit. May be nil (noop path).
//
// Exit behavior:
//   - Exit code 0 on timeout expiration or drain completion.
//   - Exit code 1 on a second signal during the drain period.
//
// Side effects: registers signal notifications via os/signal.Notify, spawns a
// goroutine that calls shutdownOTEL and os.Exit when shutdown completes.
func AwaitShutdownSignal(cancel context.CancelFunc, timeout time.Duration, done <-chan struct{}, shutdownOTEL func(context.Context) error) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		// Wait for first signal.
		sig := <-sigCh
		slog.Info("shutdown initiated", "signal", sig, "timeout_seconds", timeout.Seconds())
		cancel()
		slog.Info("waiting for in-flight requests", "timeout_seconds", timeout.Seconds())

		exitCode := 0
		// Wait for drain, timeout, or second signal.
		select {
		case <-time.After(timeout):
			slog.Info("shutdown complete", "reason", "timeout_expired")
		case <-done:
			slog.Info("shutdown complete", "reason", "drain_complete")
		case sig = <-sigCh:
			slog.Warn("forced shutdown on second signal", "signal", sig)
			exitCode = 1
		}

		// Flush pending OTEL telemetry before exit.
		if shutdownOTEL != nil {
			ctx, otelCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer otelCancel()
			if err := shutdownOTEL(ctx); err != nil {
				slog.Error("otel shutdown failed", "error", err)
			}
		}

		os.Exit(exitCode)
	}()
}
