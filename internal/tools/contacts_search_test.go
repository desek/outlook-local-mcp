// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains the handler tests for contacts search: that both
// collections are asked the identical normalised query, that exactly two
// requests are issued, that every match states its source, and that a refused
// query costs no request at all.
//
// @agents-index: Handler tests for contacts.search covering the two-collection
// fan-out, query normalisation, source labelling, and output tiering.
package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/mark3labs/mcp-go/mcp"
)

// contactsCollectionJSON is a canned saved-contacts response carrying one
// contact with two addresses.
const contactsCollectionJSON = `{
	"value": [
		{
			"id": "AAMkContact1",
			"displayName": "Alex Stone",
			"companyName": "Northwind",
			"emailAddresses": [
				{"name": "Alex Stone", "address": "alex@example.com"},
				{"name": "Alex home", "address": "alex@home.example"}
			],
			"businessPhones": ["+1 555 0100"],
			"mobilePhone": "+1 555 0111"
		}
	]
}`

// peopleCollectionJSON is a canned ranked-people response in the relevance
// order Graph returns, most relevant first.
const peopleCollectionJSON = `{
	"value": [
		{
			"id": "person-1",
			"displayName": "Alex Ranked",
			"scoredEmailAddresses": [{"address": "alex.ranked@example.com", "relevanceScore": 12.5}]
		},
		{
			"id": "person-2",
			"displayName": "Alexandra Second",
			"scoredEmailAddresses": [{"address": "alexandra@example.com", "relevanceScore": 3.0}]
		}
	]
}`

// contactsRecorder answers both collections a contacts call may reach and
// records what each request carried, so a test can assert on the requests as
// well as the response. The contacts domain is the one place two endpoints are
// hit in a single verb, so the request log is per-path rather than a single
// last-request field.
type contactsRecorder struct {
	mu sync.Mutex
	// paths holds the request path of every call, in order.
	paths []string
	// queries holds the raw query string of every call, in order.
	queries []string
	// contactsResponse is served for any path naming the contacts collection.
	contactsResponse string
	// peopleResponse is served for any path naming the people collection.
	peopleResponse string
	// status, when non-zero, is written instead of a body, so a test can drive
	// the Graph failure path.
	status int
}

// ServeHTTP records the request and answers from the canned response matching
// the collection the path names.
func (r *contactsRecorder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mu.Lock()
	r.paths = append(r.paths, req.URL.Path)
	r.queries = append(r.queries, req.URL.RawQuery)
	r.mu.Unlock()

	if r.status != 0 {
		w.WriteHeader(r.status)
		//nolint:errcheck // test helper
		w.Write([]byte(`{"error":{"code":"ErrorAccessDenied","message":"Access is denied."}}`))
		return
	}

	body := r.peopleResponse
	if strings.Contains(req.URL.Path, "/contacts") {
		body = r.contactsResponse
	}
	w.Header().Set("Content-Type", "application/json")
	//nolint:errcheck // test helper
	w.Write([]byte(body))
}

// callCount returns how many requests reached the recorder.
func (r *contactsRecorder) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.paths)
}

// runContactsSearch invokes the search handler against a recording endpoint and
// returns the result together with the recorder.
func runContactsSearch(t *testing.T, recorder *contactsRecorder, args map[string]any) (*mcp.CallToolResult, *contactsRecorder) {
	t.Helper()

	client, srv := newTestGraphClient(t, recorder)
	defer srv.Close()

	handler := NewHandleContactsSearch(graph.RetryConfig{}, 30*time.Second)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	result, err := handler(auth.WithGraphClient(context.Background(), client), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return result, recorder
}

// newContactsSearchRecorder returns a recorder answering both collections with
// the canned success responses.
func newContactsSearchRecorder() *contactsRecorder {
	return &contactsRecorder{
		contactsResponse: contactsCollectionJSON,
		peopleResponse:   peopleCollectionJSON,
	}
}

// TestContactsSearch_QueriesBothCollections validates that one call reaches the
// saved contacts and the ranked people, and only those two.
func TestContactsSearch_QueriesBothCollections(t *testing.T) {
	result, recorder := runContactsSearch(t, newContactsSearchRecorder(), map[string]any{"query": "Alex"})

	if result.IsError {
		t.Fatalf("expected success, got %q", resultText(t, result))
	}
	if got := recorder.callCount(); got != 2 {
		t.Fatalf("graph request count = %d, want 2", got)
	}
	if !strings.Contains(recorder.paths[0], "/contacts") {
		t.Errorf("first request path = %q, want the contacts collection", recorder.paths[0])
	}
	if !strings.Contains(recorder.paths[1], "/people") {
		t.Errorf("second request path = %q, want the people collection", recorder.paths[1])
	}
}

// TestContactsSearch_SendsIdenticalNormalisedQuery validates that both
// collections are asked the same question, since a divergence between them
// would return two answer sets to different questions under one result.
func TestContactsSearch_SendsIdenticalNormalisedQuery(t *testing.T) {
	_, recorder := runContactsSearch(t, newContactsSearchRecorder(), map[string]any{"query": "Alex Stone"})

	if len(recorder.queries) != 2 {
		t.Fatalf("recorded %d query strings, want 2", len(recorder.queries))
	}
	if recorder.queries[0] != recorder.queries[1] {
		t.Errorf("query strings diverge: %q and %q", recorder.queries[0], recorder.queries[1])
	}
	if !strings.Contains(recorder.queries[0], "search=") {
		t.Errorf("query string carries no search parameter: %q", recorder.queries[0])
	}
	if !strings.Contains(recorder.queries[0], "%22") {
		t.Errorf("query string carries no normalised quoting: %q", recorder.queries[0])
	}
}

// TestContactsSearch_LabelsEveryMatch validates that each match states which
// collection it came from, which is what lets a caller weigh a saved address
// against an inferred one.
func TestContactsSearch_LabelsEveryMatch(t *testing.T) {
	result, _ := runContactsSearch(t, newContactsSearchRecorder(), map[string]any{
		"query":  "Alex",
		"output": "summary",
	})

	var matches []map[string]any
	if err := json.Unmarshal([]byte(resultText(t, result)), &matches); err != nil {
		t.Fatalf("summary output is not JSON: %v", err)
	}
	if len(matches) != 3 {
		t.Fatalf("match count = %d, want 3", len(matches))
	}
	if matches[0]["source"] != contactSourceContact {
		t.Errorf("first match source = %v, want %q", matches[0]["source"], contactSourceContact)
	}
	if matches[1]["source"] != contactSourcePerson {
		t.Errorf("second match source = %v, want %q", matches[1]["source"], contactSourcePerson)
	}
}

// TestContactsSearch_PreservesRelevanceOrder validates that the ranked half
// arrives in the order Graph returned it, since re-sorting would discard the
// ranking.
func TestContactsSearch_PreservesRelevanceOrder(t *testing.T) {
	result, _ := runContactsSearch(t, newContactsSearchRecorder(), map[string]any{
		"query":  "Alex",
		"output": "summary",
	})

	var matches []map[string]any
	if err := json.Unmarshal([]byte(resultText(t, result)), &matches); err != nil {
		t.Fatalf("summary output is not JSON: %v", err)
	}
	if matches[1]["displayName"] != "Alex Ranked" || matches[2]["displayName"] != "Alexandra Second" {
		t.Errorf("ranked matches out of relevance order: %v then %v", matches[1]["displayName"], matches[2]["displayName"])
	}
}

// TestContactsSearch_TextStatesSourceAndTotal validates that the default tier
// renders a numbered listing carrying the source label and a total count.
func TestContactsSearch_TextStatesSourceAndTotal(t *testing.T) {
	result, _ := runContactsSearch(t, newContactsSearchRecorder(), map[string]any{"query": "Alex"})

	text := resultText(t, result)
	if !strings.Contains(text, "1. Alex Stone ["+contactSourceContact+"]") {
		t.Errorf("text output does not label the saved contact: %s", text)
	}
	if !strings.Contains(text, "alex@example.com") {
		t.Errorf("text output states no address: %s", text)
	}
	if !strings.Contains(text, "3 match(es) total.") {
		t.Errorf("text output states no total: %s", text)
	}
}

// TestContactsSearch_RawTierCarriesDetail validates that the raw tier projects
// each half through its own full serializer, so a caller that escalated sees
// the detail the summary deliberately omits rather than the same field set
// under a different name.
func TestContactsSearch_RawTierCarriesDetail(t *testing.T) {
	result, _ := runContactsSearch(t, newContactsSearchRecorder(), map[string]any{
		"query":  "Alex",
		"output": "raw",
	})

	var matches []map[string]any
	if err := json.Unmarshal([]byte(resultText(t, result)), &matches); err != nil {
		t.Fatalf("raw output is not JSON: %v", err)
	}
	if len(matches) != 3 {
		t.Fatalf("match count = %d, want 3", len(matches))
	}
	if matches[0]["companyName"] != "Northwind" {
		t.Errorf("saved contact carries no raw-tier detail: %v", matches[0])
	}
	if matches[0]["mobilePhone"] != "+1 555 0111" {
		t.Errorf("saved contact carries no phone detail: %v", matches[0])
	}
	if _, ok := matches[1]["scoredEmailAddresses"]; !ok {
		t.Errorf("ranked person carries no scored addresses: %v", matches[1])
	}
	if matches[1]["source"] != contactSourcePerson {
		t.Errorf("raw tier drops the source label: %v", matches[1]["source"])
	}
}

// TestContactsSearch_RequiresQuery validates that a call naming no query is
// refused before any request, with an error naming the parameter.
func TestContactsSearch_RequiresQuery(t *testing.T) {
	result, recorder := runContactsSearch(t, newContactsSearchRecorder(), map[string]any{"query": "   "})

	if !result.IsError {
		t.Fatal("expected an error result when query is whitespace only")
	}
	if !strings.Contains(resultText(t, result), "query") {
		t.Errorf("error does not name query: %s", resultText(t, result))
	}
	if got := recorder.callCount(); got != 0 {
		t.Errorf("graph request count = %d, want 0", got)
	}
}

// TestContactsSearch_RejectsUnconvertibleQuery validates that a query the
// shared normaliser refuses issues no request, so neither collection is asked a
// question Graph would reject.
func TestContactsSearch_RejectsUnconvertibleQuery(t *testing.T) {
	result, recorder := runContactsSearch(t, newContactsSearchRecorder(), map[string]any{"query": `Alex "Stone`})

	if !result.IsError {
		t.Fatal("expected an error result for an unconvertible query")
	}
	if got := recorder.callCount(); got != 0 {
		t.Errorf("graph request count = %d, want 0", got)
	}
}

// TestContactsSearch_GraphFailureCarriesFix validates that a failed collection
// fails the verb with a correction, rather than returning the other half as if
// it were the whole answer, and that the same correction reaches the log record.
//
// Both channels are asserted because the log record is the only channel a
// headless caller reading a persisted log has. Grading the tool result alone
// would leave that channel unheld: a refactor dropping the "fix" attribute from
// the record would keep every test green.
func TestContactsSearch_GraphFailureCarriesFix(t *testing.T) {
	var logged bytes.Buffer
	restore := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, nil)))
	defer slog.SetDefault(restore)

	recorder := newContactsSearchRecorder()
	recorder.status = http.StatusForbidden

	result, _ := runContactsSearch(t, recorder, map[string]any{"query": "Alex"})

	if !result.IsError {
		t.Fatal("expected an error result when a collection fails")
	}
	if !strings.Contains(resultText(t, result), contactsSearchGraphFix) {
		t.Errorf("error carries no fix instruction: %s", resultText(t, result))
	}
	if !strings.Contains(logged.String(), contactsSearchGraphFix) {
		t.Errorf("log record carries no fix instruction: %q", logged.String())
	}
}

// TestContactsSearch_NoAccountCarriesFix validates that a call with no resolved
// account is told how to select one.
func TestContactsSearch_NoAccountCarriesFix(t *testing.T) {
	handler := NewHandleContactsSearch(graph.RetryConfig{}, 30*time.Second)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"query": "Alex"}

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError || resultText(t, result) != contactsSearchAccountFix {
		t.Errorf("result = %q, want the account correction", resultText(t, result))
	}
}
