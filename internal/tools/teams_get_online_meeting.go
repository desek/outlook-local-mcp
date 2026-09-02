// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the handler for the teams get_online_meeting verb, which is
// the entry point of the transcript chain. A calendar event carries a join URL,
// not the meeting-scoped identifier the transcript reads are keyed by, so this
// verb resolves one to the other.
//
// The verb offers no unfiltered listing of online meetings, and that is a
// service contract rather than an omission: Graph serves the collection only
// when it is filtered on joinWebUrl or videoTeleconferenceId. A caller therefore
// arrives here holding one of the two identifiers this verb accepts, and a call
// naming neither cannot be answered by falling back to a listing.
//
// @agents-index: Handler constructor for the teams.get_online_meeting verb,
// resolving a meeting identifier or a join URL to the online meeting the
// transcript verbs are keyed by.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/logging"
	"github.com/desek/outlook-local-mcp/internal/validate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/microsoftgraph/msgraph-sdk-go/users"
)

// Corrections appended to this verb's refusals. The identifier correction names
// the calendar verb that supplies a join URL, because that is where a caller
// asking for a meeting recap actually starts: it holds an event, not a
// meeting-scoped identifier.
const (
	getOnlineMeetingAccountFix    = "no account is selected: run the account domain's list operation and retry naming one with the account parameter"
	getOnlineMeetingIdentifierFix = "supply exactly one of meeting_id or join_web_url: take join_web_url from the onlineMeeting join URL of a calendar get_event result, or meeting_id from an earlier get_online_meeting result"
	getOnlineMeetingURLFix        = "supply join_web_url as the complete https join link the calendar get_event result carries, not a fragment of it"
	getOnlineMeetingNoMatchFix    = "confirm the join URL is the one calendar get_event returned for this account's event; a meeting organised on another tenant is not resolvable here"
	getOnlineMeetingTimeoutFix    = "retry, and if the deadline is hit again confirm the join URL with the calendar get_event operation"
	getOnlineMeetingGraphFix      = "check that the meeting belongs to this account and that consent was granted after OUTLOOK_MCP_TEAMS_ENABLED was set, so OnlineMeetings.Read was requested, then retry; see docs/troubleshooting.md#authentication-failures"
)

// The two endpoints this verb reads, stated once each so the log records and the
// failure paths cannot drift from the request that was actually issued.
const (
	getOnlineMeetingByIDEndpoint     = "GET /me/onlineMeetings/{meeting-id}"
	getOnlineMeetingByFilterEndpoint = "GET /me/onlineMeetings?$filter=joinWebUrl"
)

// maxJoinWebURLLen bounds an accepted join URL. A Teams join link is long
// because it embeds an encoded context object, but it is not unbounded, and a
// bound keeps an oversized argument from being interpolated into a filter.
const maxJoinWebURLLen = 2048

// NewHandleGetOnlineMeeting creates the handler for the teams
// get_online_meeting verb. Given a meeting_id it reads the meeting directly;
// given a join_web_url it resolves the meeting through the joinWebUrl filter on
// the online-meetings collection and returns the first match.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for the Graph API call.
//
// Returns a tool handler function compatible with the MCP server's AddTool
// method.
//
// Exactly one identifier is required, and a call supplying neither or both is
// refused before any request is issued: a call supplying both has named two
// meetings and no reading of it is safe, and a call supplying neither cannot be
// answered by a listing, because the service does not serve one.
func NewHandleGetOnlineMeeting(retryCfg graph.RetryConfig, timeout time.Duration) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := logging.Logger(ctx)
		start := time.Now()

		client, err := GraphClient(ctx)
		if err != nil {
			logger.Error("no account selected", "fix", getOnlineMeetingAccountFix)
			return mcp.NewToolResultError(getOnlineMeetingAccountFix), nil
		}

		outputMode, err := ValidateOutputMode(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		meetingID := request.GetString("meeting_id", "")
		joinWebURL := request.GetString("join_web_url", "")

		if err := validateMeetingIdentifiers(meetingID, joinWebURL); err != nil {
			fix := getOnlineMeetingIdentifierFix
			if joinWebURL != "" {
				fix = getOnlineMeetingURLFix
			}
			logger.Error("meeting identifier rejected", "error", err.Error(), "fix", fix)
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s", err.Error(), fix)), nil
		}

		logger.Debug("tool called", "by_meeting_id", meetingID != "")

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		endpoint := getOnlineMeetingByIDEndpoint
		if meetingID == "" {
			endpoint = getOnlineMeetingByFilterEndpoint
		}
		logger.Debug("graph API request", "endpoint", endpoint)

		var meeting models.OnlineMeetingable
		graphErr := graph.RetryGraphCall(ctx, retryCfg, func() error {
			var callErr error
			if meetingID != "" {
				meeting, callErr = client.Me().OnlineMeetings().ByOnlineMeetingId(meetingID).Get(timeoutCtx, nil)
				return callErr
			}
			meeting, callErr = resolveMeetingByJoinURL(timeoutCtx, client.Me().OnlineMeetings(), joinWebURL)
			return callErr
		})
		if graphErr != nil {
			if graph.IsTimeoutError(graphErr) {
				logger.ErrorContext(ctx, "request timed out",
					"endpoint", endpoint,
					"timeout_seconds", int(timeout.Seconds()),
					"fix", getOnlineMeetingTimeoutFix,
					"error", graphErr.Error())
				return mcp.NewToolResultError(fmt.Sprintf("%s: %s",
					graph.TimeoutErrorMessage(int(timeout.Seconds())), getOnlineMeetingTimeoutFix)), nil
			}
			logger.Error("graph API call failed",
				"endpoint", endpoint,
				"error", graph.FormatGraphError(graphErr),
				"fix", getOnlineMeetingGraphFix,
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s",
				graph.RedactGraphError(graphErr), getOnlineMeetingGraphFix)), nil
		}

		// A filtered collection that matched nothing is a successful request with
		// an empty answer, so it is reported as a stated absence naming what to
		// check rather than as a service failure.
		if meeting == nil {
			logger.Error("no meeting matched the join URL",
				"endpoint", endpoint,
				"fix", getOnlineMeetingNoMatchFix,
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("no online meeting matched the supplied join_web_url: %s",
				getOnlineMeetingNoMatchFix)), nil
		}

		logger.Debug("graph API response", "endpoint", endpoint, "status", "ok")

		result := SerializeSummaryOnlineMeeting(meeting)
		if outputMode == "raw" {
			result = SerializeOnlineMeeting(meeting)
		}

		if outputMode == "text" {
			logger.Info("tool completed", "duration", time.Since(start))
			return mcp.NewToolResultText(FormatOnlineMeetingText(result)), nil
		}

		jsonBytes, err := json.Marshal(result)
		if err != nil {
			logger.Error("json serialization failed",
				"error", err.Error(),
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("failed to serialize meeting: %s", err.Error())), nil
		}

		logger.Info("tool completed", "duration", time.Since(start))
		return mcp.NewToolResultText(string(jsonBytes)), nil
	}
}

// validateMeetingIdentifiers checks that the meeting is addressed by exactly one
// of its two accepted identifiers.
//
// Parameters:
//   - meetingID: the meeting-scoped identifier, empty when a URL was supplied.
//   - joinWebURL: the join link, empty when an identifier was supplied.
//
// Returns nil when exactly one is supplied and well formed, or an error stating
// which reading of the request failed. A join URL must be an absolute https URL:
// anything else is not a link the service issued, and it would otherwise be sent
// to Graph inside a filter expression.
func validateMeetingIdentifiers(meetingID, joinWebURL string) error {
	switch {
	case meetingID != "" && joinWebURL != "":
		return fmt.Errorf("meeting_id must not be combined with join_web_url")
	case meetingID != "":
		return validate.ValidateResourceID(meetingID, "meeting_id")
	case joinWebURL != "":
		if err := validate.ValidateStringLength(joinWebURL, "join_web_url", maxJoinWebURLLen); err != nil {
			return err
		}
		parsed, err := url.Parse(joinWebURL)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			return fmt.Errorf("join_web_url must be an absolute https URL")
		}
		return nil
	default:
		return fmt.Errorf("one of meeting_id or join_web_url must be given")
	}
}

// resolveMeetingByJoinURL reads the online-meetings collection filtered on the
// join URL and returns the first meeting it matched, or nil when it matched
// none. The collection is requested filtered because the service serves it no
// other way, and only the first entry is read: a join URL addresses one meeting,
// so a second entry would be a service response this verb has no reading of.
//
// Parameters:
//   - ctx: the request context, already carrying the call's deadline.
//   - meetings: the online-meetings request builder for the signed-in user.
//   - joinWebURL: the validated join link to filter on.
//
// Returns the matched meeting, nil when the filter matched nothing, or the
// service error.
func resolveMeetingByJoinURL(ctx context.Context, meetings *users.ItemOnlineMeetingsRequestBuilder, joinWebURL string) (models.OnlineMeetingable, error) {
	// A single quote is the OData string delimiter and is escaped by doubling it.
	// A join link does not normally carry one, but the escape is applied rather
	// than assumed, so a URL that does cannot terminate the filter expression.
	filter := fmt.Sprintf("joinWebUrl eq '%s'", strings.ReplaceAll(joinWebURL, "'", "''"))

	resp, err := meetings.Get(ctx, &users.ItemOnlineMeetingsRequestBuilderGetRequestConfiguration{
		QueryParameters: &users.ItemOnlineMeetingsRequestBuilderGetQueryParameters{Filter: &filter},
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil
	}
	for _, meeting := range resp.GetValue() {
		if meeting != nil {
			return meeting, nil
		}
	}
	return nil, nil
}
