// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the handler for the contacts list_people verb, which reads
// the people Graph ranks as most relevant to the account. The ranking is the
// answer: the order Graph returns is preserved rather than re-sorted, because a
// list sorted by name would discard the only information the endpoint adds over
// the saved contacts collection.
//
// @agents-index: Handler constructor for the contacts.list_people verb, reading
// relevance-ranked people in Graph's own order and projecting them across the
// three output tiers.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/logging"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/microsoftgraph/msgraph-sdk-go/users"
)

// Corrections appended to this verb's refusals, authored at the verb because
// the shared helpers state a diagnosis without naming what to do next, and
// emitted to both the tool result and the log record.
const (
	listPeopleAccountFix = "no account is selected: run the account domain's list operation and retry naming one with the account parameter"
	listPeopleTimeoutFix = "retry, and if the deadline is hit again narrow the question with the contacts search operation instead of listing everyone"
	listPeopleGraphFix   = "check that consent was granted after OUTLOOK_MCP_CONTACTS_ENABLED was set, so People.Read was requested, then retry; see docs/troubleshooting.md#authentication-failures"
)

// Page bounds for the listing. The default matches the page Graph returns when
// no $top is sent, so an unparameterised call is unchanged; the ceiling stops a
// caller asking for an unbounded page in one round trip.
const (
	listPeopleDefaultLimit = 10
	listPeopleMaxLimit     = 100
)

// listPeopleMoreMarker is appended when Graph reports a further page, so a
// caller never reads the top of the ranking as everyone the account knows.
const listPeopleMoreMarker = "more results available: Graph returned a further page; retry with skip set to the current skip plus limit (or raise limit, maximum 100), then verify the next people appear"

// NewHandleListPeople creates the handler for the contacts list_people verb. It
// reads GET /me/people and returns the people in the relevance order Graph
// returned them, most relevant first.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for the Graph API call.
//
// Returns a tool handler function compatible with the MCP server's AddTool
// method.
//
// The handler issues exactly one Graph request carrying $top=limit and, when
// skip is positive, $skip=skip, and reads only that page; it does not follow the
// collection's next link, so the cost of the verb is bounded by one round trip.
// When the response carries @odata.nextLink a "more results available" marker
// is appended, naming the skip value that reads the next page.
func NewHandleListPeople(retryCfg graph.RetryConfig, timeout time.Duration) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := logging.Logger(ctx)
		start := time.Now()

		client, err := GraphClient(ctx)
		if err != nil {
			logger.Error("no account selected", "fix", listPeopleAccountFix)
			return mcp.NewToolResultError(listPeopleAccountFix), nil
		}

		outputMode, err := ValidateOutputMode(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		limit, skip := listPeoplePaging(request)
		query := &users.ItemPeopleRequestBuilderGetQueryParameters{}
		top := int32(limit)
		query.Top = &top
		if skip > 0 {
			skip32 := int32(skip)
			query.Skip = &skip32
		}

		logger.Debug("tool called", "limit", limit, "skip", skip)

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		logger.Debug("graph API request", "endpoint", "GET /me/people")

		var resp models.PersonCollectionResponseable
		graphErr := graph.RetryGraphCall(ctx, retryCfg, func() error {
			var callErr error
			resp, callErr = client.Me().People().Get(timeoutCtx, &users.ItemPeopleRequestBuilderGetRequestConfiguration{QueryParameters: query})
			return callErr
		})
		if graphErr != nil {
			if graph.IsTimeoutError(graphErr) {
				logger.ErrorContext(ctx, "request timed out",
					"timeout_seconds", int(timeout.Seconds()),
					"fix", listPeopleTimeoutFix,
					"error", graphErr.Error())
				return mcp.NewToolResultError(fmt.Sprintf("%s: %s",
					graph.TimeoutErrorMessage(int(timeout.Seconds())), listPeopleTimeoutFix)), nil
			}
			logger.Error("graph API call failed",
				"error", graph.FormatGraphError(graphErr),
				"fix", listPeopleGraphFix,
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s",
				graph.RedactGraphError(graphErr), listPeopleGraphFix)), nil
		}

		people := serializePeopleCollection(resp, outputMode)
		more := resp != nil && resp.GetOdataNextLink() != nil

		logger.Debug("graph API response",
			"endpoint", "GET /me/people",
			"count", len(people))

		if outputMode == "text" {
			logger.Info("tool completed",
				"duration", time.Since(start),
				"count", len(people))
			return listPeopleResult(FormatPeopleText(people), more), nil
		}

		jsonBytes, err := json.Marshal(people)
		if err != nil {
			logger.Error("json serialization failed",
				"error", err.Error(),
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("failed to serialize people: %s", err.Error())), nil
		}

		logger.Info("tool completed",
			"duration", time.Since(start),
			"count", len(people))
		return listPeopleResult(string(jsonBytes), more), nil
	}
}

// listPeoplePaging reads limit and skip, clamping limit to the page bounds
// (missing or non-positive falls back to the default) and skip to zero or
// more, so an out-of-range request still returns a bounded page.
func listPeoplePaging(request mcp.CallToolRequest) (int, int) {
	limit := int(request.GetFloat("limit", listPeopleDefaultLimit))
	if limit < 1 {
		limit = listPeopleDefaultLimit
	}
	if limit > listPeopleMaxLimit {
		limit = listPeopleMaxLimit
	}
	skip := int(request.GetFloat("skip", 0))
	if skip < 0 {
		skip = 0
	}
	return limit, skip
}

// listPeopleResult builds the tool result. The truncation marker travels as a
// second content block, so the first block keeps its documented shape (a JSON
// array in the summary and raw tiers) while the caller still sees it.
func listPeopleResult(body string, more bool) *mcp.CallToolResult {
	result := mcp.NewToolResultText(body)
	if more {
		result.Content = append(result.Content, mcp.NewTextContent(listPeopleMoreMarker))
	}
	return result
}

// serializePeopleCollection projects a people collection through the tier's
// serializer while preserving Graph's relevance order, which is the ranking the
// endpoint exists to express.
//
// Parameters:
//   - resp: the collection response, which may be nil when Graph returned no body.
//   - outputMode: the resolved output tier.
//
// Returns one serialized map per person, in the order received.
func serializePeopleCollection(resp models.PersonCollectionResponseable, outputMode string) []map[string]any {
	if resp == nil {
		return []map[string]any{}
	}
	people := make([]map[string]any, 0, len(resp.GetValue()))
	for _, person := range resp.GetValue() {
		if outputMode == "raw" {
			people = append(people, SerializePerson(person))
		} else {
			people = append(people, SerializeSummaryPerson(person))
		}
	}
	return people
}
