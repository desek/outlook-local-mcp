// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides shared test helpers for the tools package tests, including
// pointer helper functions and a test Graph client constructor.
package tools

import (
	"net/http"
	"net/http/httptest"
	"testing"

	kiotaauth "github.com/microsoft/kiota-abstractions-go/authentication"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
)

// ptr returns a pointer to the given string value.
func ptr(s string) *string {
	return &s
}

// boolPtr returns a pointer to the given bool value.
func boolPtr(b bool) *bool {
	return &b
}

// newTestGraphClient creates a GraphServiceClient backed by the given test
// HTTP server. It uses an anonymous authentication provider so no real
// credentials are required. The caller must close the returned httptest.Server.
func newTestGraphClient(t *testing.T, handler http.Handler) (*msgraphsdk.GraphServiceClient, *httptest.Server) {
	t.Helper()

	srv := httptest.NewServer(handler)

	httpClient := &http.Client{
		Transport: &testTransport{baseURL: srv.URL},
	}

	auth := &kiotaauth.AnonymousAuthenticationProvider{}
	adapter, err := msgraphsdk.NewGraphRequestAdapterWithParseNodeFactoryAndSerializationWriterFactoryAndHttpClient(
		auth, nil, nil, httpClient,
	)
	if err != nil {
		t.Fatalf("create test adapter: %v", err)
	}

	client := msgraphsdk.NewGraphServiceClient(adapter)
	return client, srv
}

// testTransport is an http.RoundTripper that rewrites requests from the Graph
// API base URL (https://graph.microsoft.com) to the local test server URL.
type testTransport struct {
	baseURL string
}

// RoundTrip rewrites the request URL to target the test server and delegates
// to the default transport.
func (t *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "http"
	req.URL.Host = t.baseURL[len("http://"):]
	return http.DefaultTransport.RoundTrip(req)
}
