// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the handler for the contacts search verb, the domain's
// entry point for turning a name into an address. Graph answers that question
// from two unrelated collections — the contacts the user saved and the people
// Graph ranks them as corresponding with — and neither subsumes the other, so
// the verb queries both with one normalised query and labels every match with
// the collection it came from.
//
// @agents-index: Handler constructor for the contacts.search verb, issuing one
// normalised query against saved contacts and ranked people and returning the
// merged, source-labelled matches across the three output tiers.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/logging"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/microsoftgraph/msgraph-sdk-go/users"
)

// Corrections appended to this verb's refusals. Each names what to supply or
// correct, and each reaches both the tool result and the log record, so a
// headless caller that sees no interactive surface still receives it.
const (
	contactsSearchAccountFix = "no account is selected: run the account domain's list operation and retry naming one with the account parameter"
	contactsSearchQueryFix   = "query is required: supply the name, address, or company to resolve, such as \"Alex\" or alex@example.com"
	contactsSearchTimeoutFix = "retry with a narrower query, since a short common term matches a large part of the mailbox"
	contactsSearchGraphFix   = "check that consent was granted after OUTLOOK_MCP_CONTACTS_ENABLED was set, so Contacts.Read and People.Read were requested, then retry; see docs/troubleshooting.md#authentication-failures"
)

// NewHandleContactsSearch creates the handler for the contacts search verb. It
// issues GET /me/contacts and GET /me/people, both carrying the identical
// normalised $search value, and returns the union of the two result sets with
// each match labelled by its source.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration covering both Graph API calls.
//
// Returns a tool handler function compatible with the MCP server's AddTool
// method.
//
// The query is normalised once, before the timeout context and before either
// request, so a query the normaliser refuses costs no Graph call and cannot
// diverge between the two collections. Exactly two requests are issued on the
// success path, and no per-match fetch follows them.
func NewHandleContactsSearch(retryCfg graph.RetryConfig, timeout time.Duration) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := logging.Logger(ctx)
		start := time.Now()

		client, err := GraphClient(ctx)
		if err != nil {
			logger.Error("no account selected", "fix", contactsSearchAccountFix)
			return mcp.NewToolResultError(contactsSearchAccountFix), nil
		}

		outputMode, err := ValidateOutputMode(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		query := request.GetString("query", "")
		if strings.TrimSpace(query) == "" {
			logger.Error("query rejected", "fix", contactsSearchQueryFix)
			return mcp.NewToolResultError(contactsSearchQueryFix), nil
		}

		// One normalisation, ahead of both requests, so the two collections are
		// asked the same question and a query that cannot be expressed as a
		// $search value is refused before any request is issued.
		normalised, err := NormaliseSearchQuery(query)
		if err != nil {
			logger.Error("search query rejected", "error", err.Error())
			return mcp.NewToolResultError(err.Error()), nil
		}

		logger.Debug("tool called", "query", query)

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		logger.Debug("graph API request", "endpoint", "GET /me/contacts", "search", normalised)

		var contactsResp models.ContactCollectionResponseable
		graphErr := graph.RetryGraphCall(ctx, retryCfg, func() error {
			var callErr error
			contactsResp, callErr = client.Me().Contacts().Get(timeoutCtx, &users.ItemContactsRequestBuilderGetRequestConfiguration{
				QueryParameters: &users.ItemContactsRequestBuilderGetQueryParameters{Search: &normalised},
			})
			return callErr
		})
		if graphErr != nil {
			return contactsSearchError(ctx, logger, graphErr, "GET /me/contacts", timeout, start), nil
		}

		logger.Debug("graph API request", "endpoint", "GET /me/people", "search", normalised)

		var peopleResp models.PersonCollectionResponseable
		graphErr = graph.RetryGraphCall(ctx, retryCfg, func() error {
			var callErr error
			peopleResp, callErr = client.Me().People().Get(timeoutCtx, &users.ItemPeopleRequestBuilderGetRequestConfiguration{
				QueryParameters: &users.ItemPeopleRequestBuilderGetQueryParameters{Search: &normalised},
			})
			return callErr
		})
		if graphErr != nil {
			return contactsSearchError(ctx, logger, graphErr, "GET /me/people", timeout, start), nil
		}

		matches := mergeContactMatches(contactsResp, peopleResp, outputMode)

		logger.Debug("graph API response",
			"endpoint", "GET /me/contacts and GET /me/people",
			"count", len(matches))

		if outputMode == "text" {
			logger.Info("tool completed",
				"duration", time.Since(start),
				"count", len(matches))
			return mcp.NewToolResultText(FormatContactMatchesText(matches)), nil
		}

		jsonBytes, err := json.Marshal(matches)
		if err != nil {
			logger.Error("json serialization failed",
				"error", err.Error(),
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("failed to serialize matches: %s", err.Error())), nil
		}

		logger.Info("tool completed",
			"duration", time.Since(start),
			"count", len(matches))
		return mcp.NewToolResultText(string(jsonBytes)), nil
	}
}

// mergeContactMatches projects both collections through the tier's serializers
// and returns them as one list, saved contacts first and ranked people after,
// each stamped with its source. Neither collection is re-sorted: the contacts
// arrive in Graph's own order and the people in relevance order, and merging
// them by any other key would discard that ranking.
//
// Parameters:
//   - contactsResp: the saved-contacts response, which may be nil.
//   - peopleResp: the ranked-people response, which may be nil.
//   - outputMode: the resolved output tier.
//
// Returns one serialized, source-labelled map per match.
func mergeContactMatches(contactsResp models.ContactCollectionResponseable, peopleResp models.PersonCollectionResponseable, outputMode string) []map[string]any {
	matches := []map[string]any{}

	if contactsResp != nil {
		for _, contact := range contactsResp.GetValue() {
			record := SerializeSummaryContact(contact)
			if outputMode == "raw" {
				record = SerializeContact(contact)
			}
			matches = append(matches, LabelContactMatch(record, contactSourceContact))
		}
	}

	if peopleResp != nil {
		for _, person := range peopleResp.GetValue() {
			record := SerializeSummaryPerson(person)
			if outputMode == "raw" {
				record = SerializePerson(person)
			}
			matches = append(matches, LabelContactMatch(record, contactSourcePerson))
		}
	}

	return matches
}

// contactsSearchError renders a failed half of the search as an error result.
// Either collection failing fails the verb, because a partial answer that does
// not say which half is missing would be read as a complete one, and a caller
// resolving a name would conclude the person does not exist.
//
// Parameters:
//   - ctx: the request context, used for the timeout log record.
//   - logger: the request logger.
//   - graphErr: the error the Graph call returned.
//   - endpoint: the endpoint that failed, named in the log record.
//   - timeout: the deadline that applied, named in the timeout message.
//   - start: when the handler began, for the duration field.
//
// Returns the tool result carrying the redacted diagnosis and its correction.
func contactsSearchError(ctx context.Context, logger *slog.Logger, graphErr error, endpoint string, timeout time.Duration, start time.Time) *mcp.CallToolResult {
	if graph.IsTimeoutError(graphErr) {
		logger.ErrorContext(ctx, "request timed out",
			"endpoint", endpoint,
			"timeout_seconds", int(timeout.Seconds()),
			"fix", contactsSearchTimeoutFix,
			"error", graphErr.Error())
		return mcp.NewToolResultError(fmt.Sprintf("%s: %s",
			graph.TimeoutErrorMessage(int(timeout.Seconds())), contactsSearchTimeoutFix))
	}
	logger.Error("graph API call failed",
		"endpoint", endpoint,
		"error", graph.FormatGraphError(graphErr),
		"fix", contactsSearchGraphFix,
		"duration", time.Since(start))
	return mcp.NewToolResultError(fmt.Sprintf("%s: %s",
		graph.RedactGraphError(graphErr), contactsSearchGraphFix))
}
