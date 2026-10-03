// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains the handler tests for teams search, and the recording
// endpoint the domain's other handler tests share. The requests are graded as
// well as the responses: the entity type this verb scopes its query to is the
// property that keeps the search inside the consent the domain asked for, and it
// is only observable in the outgoing body.
//
// @agents-index: Handler tests for teams.search covering the posted entity type,
// query refusal, source labelling, and output tiering, plus the shared Teams
// recording endpoint.
package tools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/mark3labs/mcp-go/mcp"
)

// teamsRecorder answers any Teams endpoint with one canned body and records what
// each request carried. It records the method and the request body as well as
// the path, because the domain's entry point is a POST whose only distinguishing
// content is its body.
type teamsRecorder struct {
	mu sync.Mutex
	// paths holds the request path of every call, in order.
	paths []string
	// methods holds the HTTP method of every call, in order.
	methods []string
	// bodies holds the request body of every call, in order, empty for a GET.
	bodies []string
	// response is served for every request when status is zero.
	response string
	// status, when non-zero, is written instead of the body, so a test can drive
	// the Graph failure path.
	status int
}

// ServeHTTP records the request and answers with the canned response.
func (r *teamsRecorder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	body, _ := io.ReadAll(req.Body)

	r.mu.Lock()
	r.paths = append(r.paths, req.URL.Path)
	r.methods = append(r.methods, req.Method)
	r.bodies = append(r.bodies, string(body))
	r.mu.Unlock()

	if r.status != 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(r.status)
		//nolint:errcheck // test helper
		w.Write([]byte(`{"error":{"code":"ErrorAccessDenied","message":"Access is denied."}}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	//nolint:errcheck // test helper
	w.Write([]byte(r.response))
}

// callCount returns how many requests reached the recorder.
func (r *teamsRecorder) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.paths)
}

// runTeamsHandler invokes a Teams handler against a recording endpoint and
// returns the result together with the recorder. Every Teams handler has the
// same constructor shape, so one runner serves the whole domain.
func runTeamsHandler(
	t *testing.T,
	recorder *teamsRecorder,
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

// teamsSearchResponseJSON is a canned search response carrying one chat hit and
// one channel hit, which is the mix the verb exists to disambiguate.
const teamsSearchResponseJSON = `{
	"value": [
		{
			"hitsContainers": [
				{
					"total": 2,
					"moreResultsAvailable": true,
					"hits": [
						{
							"hitId": "hit-1",
							"rank": 1,
							"summary": "the release checklist is ready",
							"resource": {
								"@odata.type": "#microsoft.graph.chatMessage",
								"id": "msg-chat-1",
								"chatId": "19:chat-1",
								"createdDateTime": "2026-01-02T03:04:05Z",
								"from": {"user": {"displayName": "Alex Stone"}}
							}
						},
						{
							"hitId": "hit-2",
							"rank": 2,
							"summary": "release notes drafted",
							"resource": {
								"@odata.type": "#microsoft.graph.chatMessage",
								"id": "msg-chan-1",
								"subject": "Release",
								"channelIdentity": {"teamId": "team-1", "channelId": "chan-1"},
								"createdDateTime": "2026-01-03T03:04:05Z",
								"from": {"emailAddress": {"name": "Bo Lee", "address": "bo@contoso.com"}}
							}
						}
					]
				}
			]
		}
	]
}`

// newTeamsSearchRecorder returns a recorder answering with the canned hits.
func newTeamsSearchRecorder() *teamsRecorder {
	return &teamsRecorder{response: teamsSearchResponseJSON}
}

// decodeTeamsRecords decodes a JSON tool result into the records it carries.
func decodeTeamsRecords(t *testing.T, result *mcp.CallToolResult) []map[string]any {
	t.Helper()
	var records []map[string]any
	if err := json.Unmarshal([]byte(resultText(t, result)), &records); err != nil {
		t.Fatalf("result is not a JSON list: %v", err)
	}
	return records
}

// decodeTeamsSearchHits decodes the search verb's JSON envelope into its hits
// and the more-results flag.
func decodeTeamsSearchHits(t *testing.T, result *mcp.CallToolResult) ([]map[string]any, bool) {
	t.Helper()
	var envelope struct {
		Hits                 []map[string]any `json:"hits"`
		MoreResultsAvailable bool             `json:"moreResultsAvailable"`
	}
	if err := json.Unmarshal([]byte(resultText(t, result)), &envelope); err != nil {
		t.Fatalf("result is not the search envelope: %v", err)
	}
	return envelope.Hits, envelope.MoreResultsAvailable
}

// TestTeamsSearch_PostsChatMessageEntityType validates the outgoing request:
// the verb must reach the search endpoint by POST and scope the query to the
// chatMessage entity type. An unscoped search would reach collections this
// domain never requested consent for.
func TestTeamsSearch_PostsChatMessageEntityType(t *testing.T) {
	recorder := newTeamsSearchRecorder()
	result := runTeamsHandler(t, recorder, NewHandleTeamsSearch, map[string]any{"query": "release checklist"})

	if result.IsError {
		t.Fatalf("expected success, got %q", resultText(t, result))
	}
	if got := recorder.callCount(); got != 1 {
		t.Fatalf("graph request count = %d, want 1", got)
	}
	if recorder.methods[0] != http.MethodPost {
		t.Errorf("method = %q, want POST", recorder.methods[0])
	}
	if !strings.HasSuffix(recorder.paths[0], "/search/query") {
		t.Errorf("path = %q, want the search query endpoint", recorder.paths[0])
	}

	var body struct {
		Requests []struct {
			EntityTypes []string `json:"entityTypes"`
			Size        int      `json:"size"`
			From        int      `json:"from"`
			Query       struct {
				QueryString string `json:"queryString"`
			} `json:"query"`
		} `json:"requests"`
	}
	if err := json.Unmarshal([]byte(recorder.bodies[0]), &body); err != nil {
		t.Fatalf("request body is not JSON: %v (%q)", err, recorder.bodies[0])
	}
	if len(body.Requests) != 1 {
		t.Fatalf("posted %d search requests, want 1", len(body.Requests))
	}
	if len(body.Requests[0].EntityTypes) != 1 || body.Requests[0].EntityTypes[0] != "chatMessage" {
		t.Errorf("entityTypes = %v, want [chatMessage]", body.Requests[0].EntityTypes)
	}
	if body.Requests[0].Query.QueryString != "release checklist" {
		t.Errorf("queryString = %q, want the caller's query", body.Requests[0].Query.QueryString)
	}
	if body.Requests[0].Size != 25 || body.Requests[0].From != 0 {
		t.Errorf("size/from = %d/%d, want the default page 25/0", body.Requests[0].Size, body.Requests[0].From)
	}
}

// TestTeamsSearch_PostsCallerPaging validates that the page size and offset
// the caller names reach the posted body, since without them only the first
// page of hits is ever reachable.
func TestTeamsSearch_PostsCallerPaging(t *testing.T) {
	recorder := newTeamsSearchRecorder()
	runTeamsHandler(t, recorder, NewHandleTeamsSearch, map[string]any{"query": "release", "max_results": float64(50), "from": float64(25)})

	var body struct {
		Requests []struct {
			Size int `json:"size"`
			From int `json:"from"`
		} `json:"requests"`
	}
	if err := json.Unmarshal([]byte(recorder.bodies[0]), &body); err != nil {
		t.Fatalf("request body is not JSON: %v", err)
	}
	if body.Requests[0].Size != 50 || body.Requests[0].From != 25 {
		t.Errorf("size/from = %d/%d, want 50/25", body.Requests[0].Size, body.Requests[0].From)
	}
}

// TestTeamsSearch_RejectsOutOfBoundsPaging validates that a page size or an
// offset outside the service bounds is refused with its fix before any request.
func TestTeamsSearch_RejectsOutOfBoundsPaging(t *testing.T) {
	cases := []map[string]any{
		{"query": "release", "max_results": float64(0)},
		{"query": "release", "max_results": float64(501)},
		{"query": "release", "max_results": float64(2.5)},
		{"query": "release", "from": float64(-1)},
	}
	for _, args := range cases {
		recorder := newTeamsSearchRecorder()
		result := runTeamsHandler(t, recorder, NewHandleTeamsSearch, args)
		if !result.IsError {
			t.Errorf("args %v: expected refusal, got %q", args, resultText(t, result))
		}
		if recorder.callCount() != 0 {
			t.Errorf("args %v: graph request count = %d, want 0", args, recorder.callCount())
		}
		if !strings.Contains(resultText(t, result), "verify") {
			t.Errorf("args %v: refusal carries no fix: %q", args, resultText(t, result))
		}
	}
}

// TestTeamsSearch_SurfacesMoreResults validates that the service's
// more-results flag reaches both the JSON tiers and the text tier.
func TestTeamsSearch_SurfacesMoreResults(t *testing.T) {
	_, more := decodeTeamsSearchHits(t, runTeamsHandler(t, newTeamsSearchRecorder(), NewHandleTeamsSearch, map[string]any{
		"query": "release", "output": "summary",
	}))
	if !more {
		t.Error("moreResultsAvailable = false, want true from the service flag")
	}
	text := resultText(t, runTeamsHandler(t, newTeamsSearchRecorder(), NewHandleTeamsSearch, map[string]any{
		"query": "release", "output": "text", "from": float64(10), "max_results": float64(5),
	}))
	if !strings.Contains(text, "More results available: retry with from=15.") {
		t.Errorf("text tier does not state more results:\n%s", text)
	}
}

// TestTeamsSearch_SenderFromEmailAddress validates that a hit whose sender is
// named only by the email address pair the search service returns still
// carries the sender's name, and its address at the raw tier.
func TestTeamsSearch_SenderFromEmailAddress(t *testing.T) {
	summary, _ := decodeTeamsSearchHits(t, runTeamsHandler(t, newTeamsSearchRecorder(), NewHandleTeamsSearch, map[string]any{
		"query": "release", "output": "summary",
	}))
	if summary[1]["from"] != "Bo Lee" {
		t.Errorf("sender = %v, want the email address name", summary[1]["from"])
	}
	raw, _ := decodeTeamsSearchHits(t, runTeamsHandler(t, newTeamsSearchRecorder(), NewHandleTeamsSearch, map[string]any{
		"query": "release", "output": "raw",
	}))
	if raw[1]["fromAddress"] != "bo@contoso.com" {
		t.Errorf("raw sender address = %v, want bo@contoso.com", raw[1]["fromAddress"])
	}
}

// TestTeamsSearch_RejectsEmptyQueryBeforeAnyRequest validates that a query of
// only whitespace is refused with its correction and costs no Graph call.
func TestTeamsSearch_RejectsEmptyQueryBeforeAnyRequest(t *testing.T) {
	recorder := newTeamsSearchRecorder()
	result := runTeamsHandler(t, recorder, NewHandleTeamsSearch, map[string]any{"query": "   "})

	if !result.IsError {
		t.Fatalf("expected refusal, got %q", resultText(t, result))
	}
	if got := recorder.callCount(); got != 0 {
		t.Errorf("graph request count = %d, want 0", got)
	}
	if !strings.Contains(resultText(t, result), "query is required") {
		t.Errorf("refusal carries no correction: %q", resultText(t, result))
	}
}

// TestTeamsSearch_LabelsHitsByCollection validates that each hit states which
// collection it came from and carries the identifiers that collection's read
// verbs take, since a chat hit and a channel hit are read back differently.
func TestTeamsSearch_LabelsHitsByCollection(t *testing.T) {
	result := runTeamsHandler(t, newTeamsSearchRecorder(), NewHandleTeamsSearch, map[string]any{
		"query":  "release",
		"output": "summary",
	})

	hits, _ := decodeTeamsSearchHits(t, result)
	if len(hits) != 2 {
		t.Fatalf("hit count = %d, want 2", len(hits))
	}

	if hits[0]["source"] != teamsHitSourceChat {
		t.Errorf("first hit source = %v, want %q", hits[0]["source"], teamsHitSourceChat)
	}
	if hits[0]["chatId"] != "19:chat-1" {
		t.Errorf("first hit chatId = %v, want the chat identifier", hits[0]["chatId"])
	}
	if hits[0]["bodyPreview"] != "the release checklist is ready" {
		t.Errorf("first hit preview = %v, want the service snippet", hits[0]["bodyPreview"])
	}

	if hits[1]["source"] != teamsHitSourceChannel {
		t.Errorf("second hit source = %v, want %q", hits[1]["source"], teamsHitSourceChannel)
	}
	if hits[1]["teamId"] != "team-1" || hits[1]["channelId"] != "chan-1" {
		t.Errorf("second hit channel coordinates = %v/%v, want team-1/chan-1", hits[1]["teamId"], hits[1]["channelId"])
	}
	if _, present := hits[1]["chatId"]; present {
		t.Error("channel hit carries a chatId, which no channel read verb takes")
	}
}

// TestTeamsSearch_PreservesRelevanceOrder validates that the ranking Graph
// returned survives the flattening of its nested containers, since the ranking
// is the only thing the search adds over an enumeration.
func TestTeamsSearch_PreservesRelevanceOrder(t *testing.T) {
	result := runTeamsHandler(t, newTeamsSearchRecorder(), NewHandleTeamsSearch, map[string]any{
		"query":  "release",
		"output": "summary",
	})

	hits, _ := decodeTeamsSearchHits(t, result)
	if len(hits) != 2 {
		t.Fatalf("hit count = %d, want 2", len(hits))
	}
	if hits[0]["rank"] != float64(1) || hits[1]["rank"] != float64(2) {
		t.Errorf("ranks = %v then %v, want 1 then 2", hits[0]["rank"], hits[1]["rank"])
	}
}

// TestTeamsSearch_ProjectsOnlyRetrievableFields validates that a hit carries
// the service snippet as its preview and none of the message fields the search
// service never returns, which would otherwise always be empty.
func TestTeamsSearch_ProjectsOnlyRetrievableFields(t *testing.T) {
	for _, tier := range []string{"summary", "raw"} {
		hits, _ := decodeTeamsSearchHits(t, runTeamsHandler(t, newTeamsSearchRecorder(), NewHandleTeamsSearch, map[string]any{
			"query":  "release",
			"output": tier,
		}))
		if hits[0]["bodyPreview"] != "the release checklist is ready" {
			t.Errorf("%s preview = %v, want the hit summary", tier, hits[0]["bodyPreview"])
		}
		for _, absent := range []string{"body", "attachments", "attachmentCount", "mentions", "messageType", "lastEditedDateTime", "replyToId", "locale"} {
			if _, present := hits[0][absent]; present {
				t.Errorf("%s hit carries %q, which search never returns", tier, absent)
			}
		}
	}
}

// TestTeamsSearch_TextTierStatesSourceAndTotal validates that the text tier
// renders each hit under its collection label and closes with a total.
func TestTeamsSearch_TextTierStatesSourceAndTotal(t *testing.T) {
	result := runTeamsHandler(t, newTeamsSearchRecorder(), NewHandleTeamsSearch, map[string]any{
		"query":  "release",
		"output": "text",
	})

	text := resultText(t, result)
	for _, want := range []string{"[chat]", "[channel]", "Chat ID: 19:chat-1", "Team ID: team-1", "2 match(es) total."} {
		if !strings.Contains(text, want) {
			t.Errorf("text output missing %q:\n%s", want, text)
		}
	}
}

// TestTeamsSearch_GraphFailureCarriesFix validates that a refused search reaches
// the caller with the correction naming the consent to check, not only a
// redacted diagnosis.
func TestTeamsSearch_GraphFailureCarriesFix(t *testing.T) {
	recorder := &teamsRecorder{response: teamsSearchResponseJSON, status: http.StatusForbidden}
	result := runTeamsHandler(t, recorder, NewHandleTeamsSearch, map[string]any{"query": "release"})

	if !result.IsError {
		t.Fatalf("expected failure, got %q", resultText(t, result))
	}
	if !strings.Contains(resultText(t, result), "OUTLOOK_MCP_TEAMS_ENABLED") {
		t.Errorf("failure carries no correction: %q", resultText(t, result))
	}
}

// TestTeamsSearch_EmptyResponseIsAnEmptyList validates that a search matching
// nothing returns a stated empty result rather than a null, so a caller reading
// the JSON tier does not have to guard against one.
func TestTeamsSearch_EmptyResponseIsAnEmptyList(t *testing.T) {
	result := runTeamsHandler(t, &teamsRecorder{response: `{"value":[]}`}, NewHandleTeamsSearch, map[string]any{
		"query":  "release",
		"output": "summary",
	})

	if got := resultText(t, result); got != `{"hits":[],"moreResultsAvailable":false}` {
		t.Errorf("empty search result = %q, want an empty hits list", got)
	}
}
