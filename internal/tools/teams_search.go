// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the handler for the teams search verb, the domain's entry
// point. Every other Teams read is keyed by an identifier the caller has to have
// obtained from somewhere, and enumerating teams and channels is deliberately
// not offered, so this verb is where those identifiers come from: a ranked hit
// carries the chat, team, and channel coordinates the follow-up verbs take.
//
// Unlike every other read in the domain this is a POST, because the Graph search
// endpoint takes its query in a request body rather than in a query string. It
// still reads and mutates nothing, which is why the verb is classified read-only
// despite the method.
//
// @agents-index: Handler constructor for the teams.search verb, posting a
// chatMessage-scoped Graph search query and returning ranked, source-labelled
// hits across the three output tiers.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/logging"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/microsoftgraph/msgraph-sdk-go/search"
)

// Corrections appended to this verb's refusals. Each names what to supply or
// correct, and each reaches both the tool result and the log record, so a
// headless caller that sees no interactive surface still receives it.
const (
	teamsSearchAccountFix = "no account is selected: run the account domain's list operation and retry naming one with the account parameter"
	teamsSearchQueryFix   = "query is required: supply the words to search Teams messages for, such as \"release checklist\""
	teamsSearchTimeoutFix = "retry with a narrower query, since a short common term matches a large part of the message store"
	teamsSearchSizeFix    = "max_results must be a whole number from 1 to 500, the page size the search service accepts; omit it for the default of 25, then verify the reply lists at most that many hits"
	teamsSearchFromFix    = "from must be a whole number of 0 or more, the count of hits to skip; pass the previous from plus the previous max_results to read the next page, then verify the hits differ from the previous page"
	teamsSearchGraphFix   = "check that consent was granted after OUTLOOK_MCP_TEAMS_ENABLED was set, so Chat.Read and ChannelMessage.Read.All were requested, then retry; see docs/troubleshooting.md#authentication-failures"
)

// Paging bounds for one search request. The service skips from hits and
// returns at most size. The default page is the service default. The ceiling
// is the smaller of the two documented maxima (the search-query page states
// 500, the overview page 1000), so a request the stricter page allows is
// never refused by the service.
const (
	teamsSearchDefaultSize = 25
	teamsSearchMaxSize     = 500
)

// teamsSearchEndpoint names the endpoint in log records and error messages. It
// is stated once because three separate paths report it.
const teamsSearchEndpoint = "POST /search/query"

// NewHandleTeamsSearch creates the handler for the teams search verb. It posts
// one search request scoped to the chatMessage entity type and returns the
// ranked hits, each labelled with the collection its message came from and
// carrying the identifiers that collection's read verbs are keyed by.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for the Graph API call.
//
// Returns a tool handler function compatible with the MCP server's AddTool
// method.
//
// The query is rejected before the request is built when it is empty, so a
// question with no terms costs no Graph call, and exactly one request is issued
// on the success path.
func NewHandleTeamsSearch(retryCfg graph.RetryConfig, timeout time.Duration) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := logging.Logger(ctx)
		start := time.Now()

		client, err := GraphClient(ctx)
		if err != nil {
			logger.Error("no account selected", "fix", teamsSearchAccountFix)
			return mcp.NewToolResultError(teamsSearchAccountFix), nil
		}

		outputMode, err := ValidateOutputMode(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		query := strings.TrimSpace(request.GetString("query", ""))
		if query == "" {
			logger.Error("query rejected", "fix", teamsSearchQueryFix)
			return mcp.NewToolResultError(teamsSearchQueryFix), nil
		}

		size, from, pageFix := teamsSearchPaging(request)
		if pageFix != "" {
			logger.Error("paging rejected", "fix", pageFix)
			return mcp.NewToolResultError(pageFix), nil
		}

		logger.Debug("tool called", "query", query, "size", size, "from", from)

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		logger.Debug("graph API request", "endpoint", teamsSearchEndpoint, "entity_type", models.CHATMESSAGE_ENTITYTYPE.String())

		var resp search.QueryPostResponseable
		graphErr := graph.RetryGraphCall(ctx, retryCfg, func() error {
			var callErr error
			resp, callErr = client.Search().Query().PostAsQueryPostResponse(timeoutCtx, buildTeamsSearchBody(query, size, from), nil)
			return callErr
		})
		if graphErr != nil {
			if graph.IsTimeoutError(graphErr) {
				logger.ErrorContext(ctx, "request timed out",
					"endpoint", teamsSearchEndpoint,
					"timeout_seconds", int(timeout.Seconds()),
					"fix", teamsSearchTimeoutFix,
					"error", graphErr.Error())
				return mcp.NewToolResultError(fmt.Sprintf("%s: %s",
					graph.TimeoutErrorMessage(int(timeout.Seconds())), teamsSearchTimeoutFix)), nil
			}
			logger.Error("graph API call failed",
				"endpoint", teamsSearchEndpoint,
				"error", graph.FormatGraphError(graphErr),
				"fix", teamsSearchGraphFix,
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s",
				graph.RedactGraphError(graphErr), teamsSearchGraphFix)), nil
		}

		hits, more := collectTeamsSearchHits(resp, outputMode)

		logger.Debug("graph API response", "endpoint", teamsSearchEndpoint, "count", len(hits))

		if outputMode == "text" {
			logger.Info("tool completed", "duration", time.Since(start), "count", len(hits))
			return mcp.NewToolResultText(teamsSearchText(hits, more, from, size)), nil
		}

		jsonBytes, err := json.Marshal(map[string]any{"hits": hits, "moreResultsAvailable": more})
		if err != nil {
			logger.Error("json serialization failed",
				"error", err.Error(),
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("failed to serialize search hits: %s", err.Error())), nil
		}

		logger.Info("tool completed", "duration", time.Since(start), "count", len(hits))
		return mcp.NewToolResultText(string(jsonBytes)), nil
	}
}

// buildTeamsSearchBody builds the single-request search body this verb posts.
// The entity type is fixed to chatMessage and is not caller-controlled: the
// scopes this domain requests cover Teams messages and nothing else, so a
// caller-supplied entity type would name a collection consent was never granted
// for and turn a search into a permission error.
//
// Parameters:
//   - query: the already-trimmed, non-empty query string.
//   - size: the page size, already validated against the service bounds.
//   - from: the zero-based count of hits to skip.
//
// Returns the request body carrying exactly one search request.
func buildTeamsSearchBody(query string, size, from int32) search.QueryPostRequestBodyable {
	searchQuery := models.NewSearchQuery()
	searchQuery.SetQueryString(&query)

	searchRequest := models.NewSearchRequest()
	searchRequest.SetEntityTypes([]models.EntityType{models.CHATMESSAGE_ENTITYTYPE})
	searchRequest.SetQuery(searchQuery)
	searchRequest.SetSize(&size)
	searchRequest.SetFrom(&from)

	body := search.NewQueryPostRequestBody()
	body.SetRequests([]models.SearchRequestable{searchRequest})
	return body
}

// collectTeamsSearchHits flattens the search response onto one ranked list.
// Graph nests hits under a response per search request and a container per
// result set, and this verb posts one request, so the nesting carries no
// information a caller needs; the relevance order inside it does, and is
// preserved.
//
// Parameters:
//   - resp: the search response, which may be nil when Graph returned no body.
//   - outputMode: the resolved output tier.
//
// Returns one serialized hit per addressable result, in the order received,
// and whether any container reported more results beyond this page. A hit
// whose resource is not a Teams message is skipped rather than returned as a
// record no follow-up verb can act on.
func collectTeamsSearchHits(resp search.QueryPostResponseable, outputMode string) ([]map[string]any, bool) {
	hits := []map[string]any{}
	more := false
	if resp == nil {
		return hits, more
	}

	for _, response := range resp.GetValue() {
		if response == nil {
			continue
		}
		for _, container := range response.GetHitsContainers() {
			if container == nil {
				continue
			}
			if graph.SafeBool(container.GetMoreResultsAvailable()) {
				more = true
			}
			for _, hit := range container.GetHits() {
				if record := SerializeTeamsSearchHit(hit, outputMode == "raw"); record != nil {
					hits = append(hits, record)
				}
			}
		}
	}

	return hits, more
}

// teamsSearchPaging reads and bounds the page size and offset. Arguments arrive
// as JSON numbers, so a fractional value is refused rather than truncated into
// a page the caller did not ask for.
//
// Returns the size, the offset, and an empty string, or a fix instruction when
// either argument is out of bounds.
func teamsSearchPaging(request mcp.CallToolRequest) (int32, int32, string) {
	size := request.GetFloat("max_results", teamsSearchDefaultSize)
	if size < 1 || size > teamsSearchMaxSize || size != float64(int32(size)) {
		return 0, 0, teamsSearchSizeFix
	}
	from := request.GetFloat("from", 0)
	if from < 0 || from > math.MaxInt32 || from != float64(int32(from)) {
		return 0, 0, teamsSearchFromFix
	}
	return int32(size), int32(from), ""
}

// teamsSearchText renders the text tier and, when the service reported more
// hits beyond this page, closes with the offset that reads the next one, so a
// caller is not left believing a truncated page is the whole result.
func teamsSearchText(hits []map[string]any, more bool, from, size int32) string {
	text := FormatTeamsSearchHitsText(hits)
	if more {
		text += fmt.Sprintf("\nMore results available: retry with from=%d.", from+size)
	}
	return text
}
