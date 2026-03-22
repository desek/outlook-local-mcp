package tools

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/mark3labs/mcp-go/mcp"
)

// mockRescheduleEventJSON is a minimal Graph API event response returned by the
// GET endpoint, with a 1-hour event from 14:00 to 15:00.
const mockRescheduleEventJSON = `{
	"id":"AAMkAGTest123",
	"subject":"1:1 Meeting",
	"start":{"dateTime":"2026-03-19T14:00:00.0000000","timeZone":"Europe/Stockholm"},
	"end":{"dateTime":"2026-03-19T15:00:00.0000000","timeZone":"Europe/Stockholm"},
	"isAllDay":false,
	"isCancelled":false,
	"isOnlineMeeting":false,
	"webLink":"",
	"categories":[]
}`

// mockReschedulePatchResponseJSON is the response returned by the PATCH endpoint
// after rescheduling.
const mockReschedulePatchResponseJSON = `{
	"id":"AAMkAGTest123",
	"subject":"1:1 Meeting",
	"start":{"dateTime":"2026-03-20T16:00:00.0000000","timeZone":"Europe/Stockholm"},
	"end":{"dateTime":"2026-03-20T17:00:00.0000000","timeZone":"Europe/Stockholm"},
	"isAllDay":false,
	"isCancelled":false,
	"isOnlineMeeting":false,
	"webLink":"",
	"categories":[]
}`

// TestRescheduleEvent_PreservesDuration validates that a 1-hour event moved to
// a new start time preserves the 1-hour duration. The handler should GET the
// event, compute the duration, and PATCH with new_start + 1 hour as end.
func TestRescheduleEvent_PreservesDuration(t *testing.T) {
	var patchBody string
	requestCount := 0
	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			//nolint:errcheck // test helper
			w.Write([]byte(mockRescheduleEventJSON))
			return
		}
		if r.Method == http.MethodPatch {
			body, _ := io.ReadAll(r.Body)
			patchBody = string(body)
			//nolint:errcheck // test helper
			w.Write([]byte(mockReschedulePatchResponseJSON))
			return
		}
	}))
	defer srv.Close()

	handler := HandleRescheduleEvent(graph.RetryConfig{}, 30*time.Second, "Europe/Stockholm")

	req := mcp.CallToolRequest{}
	req.Params.Name = "calendar_reschedule_event"
	req.Params.Arguments = map[string]any{
		"event_id":           "AAMkAGTest123",
		"new_start_datetime": "2026-03-20T16:00:00",
	}

	ctx := auth.WithGraphClient(context.Background(), client)
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		t.Fatalf("expected success, got error: %s", text)
	}

	// Verify the PATCH body contains the new end time (start + 1 hour).
	if !strings.Contains(patchBody, "2026-03-20T17:00:00") {
		t.Errorf("PATCH body should contain computed end time 2026-03-20T17:00:00, got %q", patchBody)
	}

	// Verify exactly 2 Graph API calls (GET + PATCH) per NFR-3.
	if requestCount != 2 {
		t.Errorf("expected 2 Graph API calls, got %d", requestCount)
	}

	// Verify response is text confirmation with event data.
	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "Event rescheduled:") {
		t.Errorf("expected text confirmation, got: %q", text)
	}
	if !strings.Contains(text, "AAMkAGTest123") {
		t.Errorf("expected event ID in response, got: %q", text)
	}
}

// TestRescheduleEvent_DefaultTimezone validates that when new_start_timezone is
// omitted, the handler uses the configured default timezone.
func TestRescheduleEvent_DefaultTimezone(t *testing.T) {
	var patchBody string
	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			//nolint:errcheck // test helper
			w.Write([]byte(mockRescheduleEventJSON))
			return
		}
		if r.Method == http.MethodPatch {
			body, _ := io.ReadAll(r.Body)
			patchBody = string(body)
			//nolint:errcheck // test helper
			w.Write([]byte(mockReschedulePatchResponseJSON))
			return
		}
	}))
	defer srv.Close()

	handler := HandleRescheduleEvent(graph.RetryConfig{}, 30*time.Second, "America/New_York")

	req := mcp.CallToolRequest{}
	req.Params.Name = "calendar_reschedule_event"
	req.Params.Arguments = map[string]any{
		"event_id":           "AAMkAGTest123",
		"new_start_datetime": "2026-03-20T16:00:00",
		// new_start_timezone intentionally omitted — should default to America/New_York
	}

	ctx := auth.WithGraphClient(context.Background(), client)
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		t.Fatalf("expected success, got error: %s", text)
	}

	// The PATCH body should use the default timezone "America/New_York".
	if !strings.Contains(patchBody, "America/New_York") {
		t.Errorf("PATCH body should contain default timezone America/New_York, got %q", patchBody)
	}
}

// TestRescheduleEvent_EventNotFound validates that a 404 from the GET call
// returns an error with a hint to verify the event ID.
func TestRescheduleEvent_EventNotFound(t *testing.T) {
	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		//nolint:errcheck // test helper
		w.Write([]byte(`{"error":{"code":"ErrorItemNotFound","message":"The specified object was not found in the store."}}`))
	}))
	defer srv.Close()

	handler := HandleRescheduleEvent(graph.RetryConfig{}, 30*time.Second, "Europe/Stockholm")

	req := mcp.CallToolRequest{}
	req.Params.Name = "calendar_reschedule_event"
	req.Params.Arguments = map[string]any{
		"event_id":           "AAMkAGInvalid",
		"new_start_datetime": "2026-03-20T16:00:00",
	}

	ctx := auth.WithGraphClient(context.Background(), client)
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for 404")
	}

	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "calendar_list_events") && !strings.Contains(text, "calendar_search_events") {
		t.Errorf("error should contain hint about calendar_list_events or calendar_search_events, got %q", text)
	}
}
