// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains the handler tests for teams get_online_meeting, and the
// query-recording endpoint the meeting and transcript tests share. The outgoing
// query is graded as well as the answer: resolving a join URL is a filtered
// collection read, and whether the filter was actually sent is only observable
// on the request.
//
// @agents-index: Handler tests for teams.get_online_meeting covering join-URL
// resolution, the exactly-one-identifier rule, and the empty-match refusal, plus
// the shared query-recording Teams endpoint.
package tools

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/mark3labs/mcp-go/mcp"
)

// teamsPathRecorder answers each request according to its path and records the
// request line. It records the raw query, which the domain's other recorder does
// not, because the meeting and transcript verbs are distinguished by what they
// filter on and by which of two endpoints they reached, neither of which is
// visible in a path-agnostic canned answer.
type teamsPathRecorder struct {
	mu sync.Mutex
	// paths holds the request path of every call, in order.
	paths []string
	// queries holds the raw query string of every call, in order.
	queries []string
	// methods holds the HTTP method of every call, in order.
	methods []string
	// respond returns the body and content type served for a request path.
	respond func(path string) (body string, contentType string)
	// status, when non-zero, is written instead of the body.
	status int
	// failSuffix, when non-empty, fails only the requests whose path ends with
	// it, so a verb issuing more than one request can be driven to fail on a
	// chosen one of them.
	failSuffix string
}

// ServeHTTP records the request and answers with the body its path selects.
func (r *teamsPathRecorder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mu.Lock()
	r.paths = append(r.paths, req.URL.Path)
	r.queries = append(r.queries, req.URL.RawQuery)
	r.methods = append(r.methods, req.Method)
	r.mu.Unlock()

	if r.status != 0 || (r.failSuffix != "" && strings.HasSuffix(req.URL.Path, r.failSuffix)) {
		w.Header().Set("Content-Type", "application/json")
		status := r.status
		if status == 0 {
			status = http.StatusForbidden
		}
		w.WriteHeader(status)
		//nolint:errcheck // test helper
		w.Write([]byte(`{"error":{"code":"ErrorAccessDenied","message":"Access is denied."}}`))
		return
	}

	body, contentType := r.respond(req.URL.Path)
	w.Header().Set("Content-Type", contentType)
	//nolint:errcheck // test helper
	w.Write([]byte(body))
}

// callCount returns how many requests reached the recorder.
func (r *teamsPathRecorder) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.paths)
}

// runTeamsPathHandler invokes a Teams handler against a path-recording endpoint
// and returns the result. It mirrors the domain's other runner, differing only
// in the recorder it drives.
func runTeamsPathHandler(
	t *testing.T,
	recorder *teamsPathRecorder,
	newHandler func(graph.RetryConfig, time.Duration) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error),
	args map[string]any,
) *mcp.CallToolResult {
	t.Helper()

	client, srv := newTestGraphClient(t, recorder)
	defer srv.Close()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	result, err := newHandler(graph.RetryConfig{}, 30*time.Second)(auth.WithGraphClient(context.Background(), client), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return result
}

// teamsJoinWebURL is the join link a calendar event would carry, which is the
// input this verb exists to resolve.
const teamsJoinWebURL = "https://teams.microsoft.com/l/meetup-join/19%3ameeting_abc%40thread.v2/0"

// teamsOnlineMeetingJSON is a canned single online meeting.
const teamsOnlineMeetingJSON = `{
	"id": "meeting-1",
	"subject": "Release review",
	"startDateTime": "2026-01-02T09:00:00Z",
	"endDateTime": "2026-01-02T10:00:00Z",
	"joinWebUrl": "` + teamsJoinWebURL + `",
	"allowTranscription": true,
	"participants": {"organizer": {"identity": {"user": {"displayName": "Alex Stone"}}}}
}`

// teamsOnlineMeetingCollectionJSON is the filtered collection carrying that one
// meeting, which is how a join URL resolves.
const teamsOnlineMeetingCollectionJSON = `{"value": [` + teamsOnlineMeetingJSON + `]}`

// newTeamsMeetingRecorder returns a recorder answering the collection read with
// the supplied body and the item read with the single meeting.
func newTeamsMeetingRecorder(collection string) *teamsPathRecorder {
	return &teamsPathRecorder{respond: func(path string) (string, string) {
		if strings.HasSuffix(path, "/onlineMeetings") {
			return collection, "application/json"
		}
		return teamsOnlineMeetingJSON, "application/json"
	}}
}

// TestGetOnlineMeeting_ResolvesJoinURL validates the resolution the transcript
// chain depends on: a join URL is sent as a joinWebUrl filter on the collection,
// and the meeting-scoped id comes back.
func TestGetOnlineMeeting_ResolvesJoinURL(t *testing.T) {
	recorder := newTeamsMeetingRecorder(teamsOnlineMeetingCollectionJSON)
	result := runTeamsPathHandler(t, recorder, NewHandleGetOnlineMeeting, map[string]any{
		"join_web_url": teamsJoinWebURL,
		"output":       "summary",
	})

	if result.IsError {
		t.Fatalf("expected success, got %q", resultText(t, result))
	}
	if got := recorder.callCount(); got != 1 {
		t.Fatalf("graph request count = %d, want 1", got)
	}
	if !strings.HasSuffix(recorder.paths[0], "/onlineMeetings") {
		t.Errorf("path = %q, want the online meetings collection", recorder.paths[0])
	}

	query, err := url.ParseQuery(recorder.queries[0])
	if err != nil {
		t.Fatalf("query is not parseable: %v (%q)", err, recorder.queries[0])
	}
	want := "joinWebUrl eq '" + teamsJoinWebURL + "'"
	if got := query.Get("$filter"); got != want {
		t.Errorf("$filter = %q, want %q", got, want)
	}

	if got := decodeTeamsRecord(t, result)["id"]; got != "meeting-1" {
		t.Errorf("id = %v, want meeting-1", got)
	}
}

// TestGetOnlineMeeting_ReadsTheNamedMeetingUnfiltered validates that a supplied
// meeting id addresses the item directly, with no filter, so the resolution cost
// is not paid by a caller that already holds the identifier.
func TestGetOnlineMeeting_ReadsTheNamedMeetingUnfiltered(t *testing.T) {
	recorder := newTeamsMeetingRecorder(teamsOnlineMeetingCollectionJSON)
	result := runTeamsPathHandler(t, recorder, NewHandleGetOnlineMeeting, map[string]any{
		"meeting_id": "meeting-1",
	})

	if result.IsError {
		t.Fatalf("expected success, got %q", resultText(t, result))
	}
	if got := recorder.callCount(); got != 1 {
		t.Fatalf("graph request count = %d, want 1", got)
	}
	if !strings.HasSuffix(recorder.paths[0], "/onlineMeetings/meeting-1") {
		t.Errorf("path = %q, want the addressed meeting", recorder.paths[0])
	}
	if recorder.queries[0] != "" {
		t.Errorf("query = %q, want none", recorder.queries[0])
	}
}

// TestGetOnlineMeeting_RequiresOneIdentifier validates that neither an unnamed
// meeting nor a doubly named one reaches Graph, and that the refusal names both
// parameters and where a join URL comes from.
func TestGetOnlineMeeting_RequiresOneIdentifier(t *testing.T) {
	for _, tc := range []struct {
		name string
		args map[string]any
	}{
		{"neither", map[string]any{}},
		{"both", map[string]any{"meeting_id": "meeting-1", "join_web_url": teamsJoinWebURL}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := newTeamsMeetingRecorder(teamsOnlineMeetingCollectionJSON)
			result := runTeamsPathHandler(t, recorder, NewHandleGetOnlineMeeting, tc.args)

			if !result.IsError {
				t.Fatalf("expected refusal, got %q", resultText(t, result))
			}
			if got := recorder.callCount(); got != 0 {
				t.Errorf("graph request count = %d, want 0", got)
			}
			text := resultText(t, result)
			for _, want := range []string{"meeting_id", "join_web_url"} {
				if !strings.Contains(text, want) {
					t.Errorf("refusal = %q, want it to name %q", text, want)
				}
			}
		})
	}
}

// TestGetOnlineMeeting_MissingIdentifierNamesTheCalendarVerb validates that a
// caller holding an event rather than a meeting is told where the join URL comes
// from, which is the only route into this domain's transcript chain.
func TestGetOnlineMeeting_MissingIdentifierNamesTheCalendarVerb(t *testing.T) {
	recorder := newTeamsMeetingRecorder(teamsOnlineMeetingCollectionJSON)
	result := runTeamsPathHandler(t, recorder, NewHandleGetOnlineMeeting, map[string]any{})

	if !result.IsError {
		t.Fatalf("expected refusal, got %q", resultText(t, result))
	}
	if !strings.Contains(resultText(t, result), "get_event") {
		t.Errorf("refusal = %q, want it to name the calendar get_event operation", resultText(t, result))
	}
}

// TestGetOnlineMeeting_RejectsAJoinURLThatIsNotAnHTTPSLink validates that a
// value that is not a link the service issued is refused before it can be
// interpolated into a filter expression.
func TestGetOnlineMeeting_RejectsAJoinURLThatIsNotAnHTTPSLink(t *testing.T) {
	recorder := newTeamsMeetingRecorder(teamsOnlineMeetingCollectionJSON)
	result := runTeamsPathHandler(t, recorder, NewHandleGetOnlineMeeting, map[string]any{
		"join_web_url": "meetup-join/19:meeting_abc",
	})

	if !result.IsError {
		t.Fatalf("expected refusal, got %q", resultText(t, result))
	}
	if got := recorder.callCount(); got != 0 {
		t.Errorf("graph request count = %d, want 0", got)
	}
	if !strings.Contains(resultText(t, result), "https") {
		t.Errorf("refusal = %q, want it to state the expected form", resultText(t, result))
	}
}

// TestGetOnlineMeeting_EmptyMatchIsStatedNotSilent validates that a filter
// matching nothing is reported as an answer with a correction rather than as an
// empty success a caller would carry forward as a meeting id.
func TestGetOnlineMeeting_EmptyMatchIsStatedNotSilent(t *testing.T) {
	recorder := newTeamsMeetingRecorder(`{"value": []}`)
	result := runTeamsPathHandler(t, recorder, NewHandleGetOnlineMeeting, map[string]any{
		"join_web_url": teamsJoinWebURL,
	})

	if !result.IsError {
		t.Fatalf("expected a stated absence, got %q", resultText(t, result))
	}
	if !strings.Contains(resultText(t, result), "no online meeting matched") {
		t.Errorf("result = %q, want it to state that nothing matched", resultText(t, result))
	}
}
