// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains the assertions shared by the two scheduling read verbs:
// timeout reporting, Graph error redaction, the fix instruction every refusal
// carries, and formatter determinism. They live here rather than being repeated
// per handler so a verb that improvises its own error or ordering fails against
// the family, not only against itself.
//
// @agents-index: Cross-verb tests for the calendar scheduling reads, covering
// timeout reporting, redaction, refusal fix instructions, and determinism.
package tools

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/mark3labs/mcp-go/mcp"
)

// schedulingReadVerb names one of the two scheduling reads together with
// everything a shared assertion needs to drive it: its handler constructor, a
// valid argument set, and the canned Graph response its success path expects.
type schedulingReadVerb struct {
	name       string
	newHandler func(graph.RetryConfig, time.Duration, string) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
	args       map[string]any
	response   string
	// timeoutFix and graphFix are the corrections the verb appends to the
	// shared helpers' diagnoses. They are named per verb because the
	// correction refers to that verb's own parameters.
	timeoutFix string
	graphFix   string
}

// schedulingReadVerbs returns the family under test. A verb added to the
// scheduling domain belongs here, so the shared behaviour is asserted for it
// without a new test being written.
func schedulingReadVerbs() []schedulingReadVerb {
	return []schedulingReadVerb{
		{
			name:       "find_meeting_times",
			newHandler: NewHandleFindMeetingTimes,
			args:       map[string]any{"attendees": `[{"email":"a@example.com"}]`},
			response:   findMeetingTimesResponseJSON,
			timeoutFix: findMeetingTimesTimeoutFix,
			graphFix:   findMeetingTimesGraphFix,
		},
		{
			name:       "get_schedule",
			newHandler: NewHandleGetSchedule,
			args:       withScheduleWindow(map[string]any{"schedules": "a@example.com,b@example.com"}),
			response:   getScheduleResponseJSON,
			timeoutFix: getScheduleTimeoutFix,
			graphFix:   getScheduleGraphFix,
		},
	}
}

// requestFor builds a tool request from the verb's valid arguments plus the
// supplied overrides, leaving the verb's own arguments intact.
func (v schedulingReadVerb) requestFor(overrides map[string]any) mcp.CallToolRequest {
	args := map[string]any{}
	for k, val := range v.args {
		args[k] = val
	}
	for k, val := range overrides {
		args[k] = val
	}
	req := mcp.CallToolRequest{}
	req.Params.Arguments = args
	return req
}

// captureLogs redirects the default logger into a buffer for the duration of the
// test, so an assertion can read the record a failure path emitted. The default
// logger is process-wide, so it is restored on cleanup.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return &buf
}

// TestSchedulingReadsHonourTimeoutAndRedaction verifies that both verbs route
// their Graph call through the shared timeout and redaction helpers rather than
// improvising per handler. A configured deadline other than the common thirty
// seconds is used so the assertion fails on a hardcoded message rather than
// passing by coincidence, and the canned Graph error carries an address, which
// is exactly what the redactor removes.
func TestSchedulingReadsHonourTimeoutAndRedaction(t *testing.T) {
	const configured = 7 * time.Second
	const leaked = "mailbox-owner@contoso.com"

	for _, v := range schedulingReadVerbs() {
		t.Run(v.name+" timeout", func(t *testing.T) {
			logs := captureLogs(t)

			// A server that never replies is what makes the deadline, rather
			// than the transport, decide the outcome.
			hang := make(chan struct{})
			client, srv := newTestGraphClient(t, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				select {
				case <-hang:
				case <-r.Context().Done():
				}
			}))
			defer srv.Close()
			// Released before the server is closed, because Close waits for
			// its outstanding handlers and a cancelled client request does not
			// reliably cancel the server side of a kept-alive connection.
			defer close(hang)

			handler := v.newHandler(graph.RetryConfig{}, 50*time.Millisecond, "UTC")
			result, err := handler(auth.WithGraphClient(context.Background(), client), v.requestFor(nil))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !result.IsError {
				t.Fatal("expected an error result for a timed-out request")
			}
			if !strings.Contains(resultText(t, result), "timed out") {
				t.Errorf("the refusal does not report a timeout: %q", resultText(t, result))
			}
			if !strings.Contains(logs.String(), "timeout_seconds") {
				t.Errorf("the failure path emitted no record naming the configured deadline: %s", logs.String())
			}
			// Both channels: the refusal the caller reads and the record the
			// operator reads must each state the correction, because the
			// shared timeout helper states only the deadline.
			if !strings.Contains(resultText(t, result), v.timeoutFix) {
				t.Errorf("the timeout refusal states no correction to apply: %q", resultText(t, result))
			}
			if !strings.Contains(logs.String(), v.timeoutFix) {
				t.Errorf("the timeout log record states no correction to apply: %s", logs.String())
			}
		})

		t.Run(v.name+" names the configured deadline", func(t *testing.T) {
			// An already-expired context reaches the same refusal without
			// waiting, which is where the configured value can be asserted
			// exactly rather than approximately.
			expired, cancel := context.WithTimeout(context.Background(), 0)
			defer cancel()

			recorder := &findMeetingTimesRecorder{response: v.response}
			client, srv := newTestGraphClient(t, recorder)
			defer srv.Close()

			handler := v.newHandler(graph.RetryConfig{}, configured, "UTC")
			result, err := handler(auth.WithGraphClient(expired, client), v.requestFor(nil))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !result.IsError {
				t.Fatal("expected an error result for an expired context")
			}
			if !strings.Contains(resultText(t, result), "7s") {
				t.Errorf("the refusal does not name the configured deadline: %q", resultText(t, result))
			}
		})

		t.Run(v.name+" redaction", func(t *testing.T) {
			logs := captureLogs(t)
			client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"error":{"code":"ErrorAccessDenied","message":"Access denied for ` + leaked + ` on this mailbox."}}`))
			}))
			defer srv.Close()

			handler := v.newHandler(graph.RetryConfig{}, 30*time.Second, "UTC")
			result, err := handler(auth.WithGraphClient(context.Background(), client), v.requestFor(nil))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !result.IsError {
				t.Fatal("expected an error result for a failed Graph call")
			}
			text := resultText(t, result)
			if strings.Contains(text, leaked) {
				t.Errorf("the Graph error reached the caller unredacted: %q", text)
			}
			if !strings.Contains(text, "[email redacted]") {
				t.Errorf("the redaction placeholder is absent from the refusal: %q", text)
			}
			if !strings.Contains(text, "ErrorAccessDenied") {
				t.Errorf("the refusal drops the Graph code the caller would act on: %q", text)
			}
			// The redactor returns Graph's diagnosis and nothing else, so the
			// correction is asserted on both channels here as well.
			if !strings.Contains(text, v.graphFix) {
				t.Errorf("the redacted refusal states no correction to apply: %q", text)
			}
			if !strings.Contains(logs.String(), v.graphFix) {
				t.Errorf("the Graph failure log record states no correction to apply: %s", logs.String())
			}
		})
	}
}

// TestSchedulingReadsRefusalsCarryFixInstruction verifies that the refusal each
// verb authors itself names what to supply, rather than only reporting that
// something was wrong. The missing required parameter is used because it is the
// one refusal both verbs share.
func TestSchedulingReadsRefusalsCarryFixInstruction(t *testing.T) {
	cases := []struct {
		verb      string
		handler   func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
		parameter string
	}{
		{
			verb:      "find_meeting_times",
			handler:   NewHandleFindMeetingTimes(graph.RetryConfig{}, 30*time.Second, "UTC"),
			parameter: "attendees",
		},
		{
			verb:      "get_schedule",
			handler:   NewHandleGetSchedule(graph.RetryConfig{}, 30*time.Second, "UTC"),
			parameter: "schedules",
		},
	}

	for _, tc := range cases {
		t.Run(tc.verb, func(t *testing.T) {
			recorder := &findMeetingTimesRecorder{response: "{}"}
			client, srv := newTestGraphClient(t, recorder)
			defer srv.Close()

			req := mcp.CallToolRequest{}
			req.Params.Arguments = map[string]any{}

			result, err := tc.handler(auth.WithGraphClient(context.Background(), client), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !result.IsError {
				t.Fatal("expected an error result for a missing required parameter")
			}
			text := resultText(t, result)
			if !strings.Contains(text, tc.parameter) {
				t.Errorf("the refusal does not name the parameter: %q", text)
			}
			if !strings.Contains(text, "supply") {
				t.Errorf("the refusal states no correction to apply: %q", text)
			}
			if got := recorder.calls.Load(); got != 0 {
				t.Errorf("graph request count = %d, want 0", got)
			}
		})
	}
}

// TestSchedulingFormattersDeterministic verifies that identical Graph responses
// project identically in every tier, including the ordering of mailboxes and of
// suggestions. Ordering that varies between calls would make the text tier
// unreadable across two runs and the JSON tiers uncacheable.
func TestSchedulingFormattersDeterministic(t *testing.T) {
	for _, v := range schedulingReadVerbs() {
		for _, mode := range []string{"text", "summary", "raw"} {
			t.Run(v.name+" "+mode, func(t *testing.T) {
				first := runSchedulingRead(t, v, mode)
				second := runSchedulingRead(t, v, mode)
				if first != second {
					t.Errorf("output mode %q is not deterministic across identical responses:\nfirst:  %s\nsecond: %s", mode, first, second)
				}
			})
		}
	}
}

// runSchedulingRead drives one verb once in one output mode against its canned
// response and returns the text of the result, so a determinism assertion
// compares two whole projections rather than sampled fields.
func runSchedulingRead(t *testing.T, v schedulingReadVerb, mode string) string {
	t.Helper()

	recorder := &findMeetingTimesRecorder{response: v.response}
	client, srv := newTestGraphClient(t, recorder)
	defer srv.Close()

	handler := v.newHandler(graph.RetryConfig{}, 30*time.Second, "UTC")
	result, err := handler(auth.WithGraphClient(context.Background(), client), v.requestFor(map[string]any{"output": mode}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got %q", resultText(t, result))
	}
	return resultText(t, result)
}
