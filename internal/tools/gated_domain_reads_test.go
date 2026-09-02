// Package tools contains the standing assertion that the two opt-in domains,
// contacts and teams, issue no mutating request.
//
// Both domains register only verbs that declare readOnlyHint true, and the
// registration tests grade that declaration. This file grades the behaviour:
// every Graph-calling handler in the two domains is driven against a server
// that records the HTTP method of each request, and any method other than GET
// fails the test. The one exception is the Teams search verb, which reaches the
// search API through a POST to the query endpoint; that POST is a read by
// contract and is permitted on that path only.
//
// @agents-index: Tests asserting every contacts and Teams handler issues only
// GET requests, apart from the Teams search POST to the query endpoint.
package tools

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/mark3labs/mcp-go/mcp"
)

// gatedReadCase names one handler of an opt-in domain with arguments that reach
// its request path, and the one non-GET request it is permitted to issue.
type gatedReadCase struct {
	// name attributes a failure to a verb rather than a table position.
	name string
	// ctor builds the handler under test.
	ctor teamsHandlerCtor
	// args pass every validation the verb performs.
	args map[string]any
	// allowedPostSuffix, when non-empty, is the only path a POST may target.
	allowedPostSuffix string
}

// contactsGraphCallingVerbs enumerates the four contacts handlers with
// arguments that reach the request path. The list is hand-written for the same
// reason teamsGraphCallingVerbs is: these tests drive bare constructors.
func contactsGraphCallingVerbs() []gatedReadCase {
	return []gatedReadCase{
		{name: "search", ctor: NewHandleContactsSearch, args: map[string]any{"query": "alex"}},
		{name: "get_contact", ctor: NewHandleGetContact, args: map[string]any{"contact_id": "contact-1"}},
		{name: "list_people", ctor: NewHandleListPeople, args: map[string]any{}},
		{name: "get_person", ctor: NewHandleGetPerson, args: map[string]any{"person_id": "person-1"}},
	}
}

// TestGatedDomainReadsIssueNoMutatingRequest drives every contacts and Teams
// Graph-calling handler against a method-recording server and fails on any
// request that is not a GET, apart from the Teams search POST to the query
// endpoint. The server answers with an empty object so each handler issues its
// request whether or not it can render the reply; the method is graded, not the
// result.
func TestGatedDomainReadsIssueNoMutatingRequest(t *testing.T) {
	var cases []gatedReadCase
	for _, c := range contactsGraphCallingVerbs() {
		c.name = "contacts." + c.name
		cases = append(cases, c)
	}
	for _, c := range teamsGraphCallingVerbs() {
		gc := gatedReadCase{name: "teams." + c.name, ctor: c.ctor, args: c.args}
		if c.name == "search" {
			gc.allowedPostSuffix = "/search/query"
		}
		cases = append(cases, gc)
	}
	if len(cases) != len(contactsGraphCallingVerbs())+len(teamsGraphCallingVerbs()) {
		t.Fatal("case assembly dropped a verb")
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var methods, urls []string
			client, srv := newTestGraphClient(t, recordingHandler("{}", &methods, &urls))
			defer srv.Close()

			req := mcp.CallToolRequest{}
			req.Params.Arguments = tc.args
			ctx := auth.WithGraphClient(context.Background(), client)
			if _, err := tc.ctor(graph.RetryConfig{}, 5*time.Second)(ctx, req); err != nil {
				t.Fatalf("unexpected transport error: %v", err)
			}

			if len(methods) == 0 {
				t.Fatal("expected at least one Graph request; the arguments did not reach the request path")
			}
			for i, m := range methods {
				if m == http.MethodGet {
					continue
				}
				if m == http.MethodPost && tc.allowedPostSuffix != "" && strings.HasSuffix(strings.SplitN(urls[i], "?", 2)[0], tc.allowedPostSuffix) {
					continue
				}
				t.Errorf("request %d used %s against %s; a read-only domain must not issue a mutating request", i, m, urls[i])
			}
		})
	}
}
