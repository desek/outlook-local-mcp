// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the handler for the teams list_transcripts verb, which
// reads the transcripts a meeting holds. It carries metadata only: a meeting can
// hold several transcripts, each of them the full text of a call, so the listing
// exists to choose which one to fetch rather than to deliver any of them.
//
// @agents-index: Handler constructor for the teams.list_transcripts verb,
// reading one online meeting's transcript metadata across the three output
// tiers.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/logging"
	"github.com/desek/outlook-local-mcp/internal/validate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

// Corrections appended to this verb's refusals. The identifier correction names
// the verb that produces a meeting id, because a caller holding a calendar event
// has a join URL and not the meeting-scoped identifier this verb takes.
const (
	listTranscriptsAccountFix   = "no account is selected: run the account domain's list operation and retry naming one with the account parameter"
	listTranscriptsMeetingIDFix = "supply meeting_id as the id returned by the teams get_online_meeting operation, which resolves the join URL a calendar get_event result carries"
	listTranscriptsTimeoutFix   = "retry, and if the deadline is hit again confirm the meeting id with the teams get_online_meeting operation"
	listTranscriptsGraphFix     = "check that meeting_id names a meeting this account can read and that consent was granted after OUTLOOK_MCP_TEAMS_ENABLED was set, so OnlineMeetingTranscript.Read.All was requested, then retry; see docs/troubleshooting.md#authentication-failures"
)

// The endpoint this verb reads, stated once so the log records and the failure
// paths cannot drift from each other.
const listTranscriptsEndpoint = "GET /me/onlineMeetings/{meeting-id}/transcripts"

// NewHandleListTranscripts creates the handler for the teams list_transcripts
// verb. It reads GET /me/onlineMeetings/{meeting-id}/transcripts and returns the
// transcript metadata in the order Graph returned it.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for the Graph API call.
//
// Returns a tool handler function compatible with the MCP server's AddTool
// method.
//
// The meeting identifier is validated before any request is issued, and exactly
// one request is issued on the success path; the collection's next link is not
// followed, so the cost of the verb is bounded by one round trip.
func NewHandleListTranscripts(retryCfg graph.RetryConfig, timeout time.Duration) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := logging.Logger(ctx)
		start := time.Now()

		client, err := GraphClient(ctx)
		if err != nil {
			logger.Error("no account selected", "fix", listTranscriptsAccountFix)
			return mcp.NewToolResultError(listTranscriptsAccountFix), nil
		}

		outputMode, err := ValidateOutputMode(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		meetingID := request.GetString("meeting_id", "")
		if err := validate.ValidateResourceID(meetingID, "meeting_id"); err != nil {
			logger.Error("meeting_id rejected", "error", err.Error(), "fix", listTranscriptsMeetingIDFix)
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s", err.Error(), listTranscriptsMeetingIDFix)), nil
		}

		logger.Debug("tool called", "meeting_id", meetingID)

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		logger.Debug("graph API request", "endpoint", listTranscriptsEndpoint)

		var resp models.CallTranscriptCollectionResponseable
		graphErr := graph.RetryGraphCall(ctx, retryCfg, func() error {
			var callErr error
			resp, callErr = client.Me().OnlineMeetings().ByOnlineMeetingId(meetingID).Transcripts().Get(timeoutCtx, nil)
			return callErr
		})
		if graphErr != nil {
			if graph.IsTimeoutError(graphErr) {
				logger.ErrorContext(ctx, "request timed out",
					"endpoint", listTranscriptsEndpoint,
					"timeout_seconds", int(timeout.Seconds()),
					"fix", listTranscriptsTimeoutFix,
					"error", graphErr.Error())
				return mcp.NewToolResultError(fmt.Sprintf("%s: %s",
					graph.TimeoutErrorMessage(int(timeout.Seconds())), listTranscriptsTimeoutFix)), nil
			}
			// A tenant that turned off transcript API access cannot be
			// recovered by consent or retry, so it gets its own correction.
			fix := listTranscriptsGraphFix
			if transcriptInnerErrorCode(graphErr) == innerCodeGraphAccessToTranscriptsDisabled {
				fix = transcriptAccessDisabledFix
			}
			logger.Error("graph API call failed",
				"endpoint", listTranscriptsEndpoint,
				"error", graph.FormatGraphError(graphErr),
				"fix", fix,
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s",
				graph.RedactGraphError(graphErr), fix)), nil
		}

		transcripts := serializeTranscriptCollection(resp, outputMode)

		logger.Debug("graph API response",
			"endpoint", listTranscriptsEndpoint,
			"count", len(transcripts))

		if outputMode == "text" {
			logger.Info("tool completed", "duration", time.Since(start), "count", len(transcripts))
			return mcp.NewToolResultText(FormatTranscriptsText(transcripts)), nil
		}

		jsonBytes, err := json.Marshal(transcripts)
		if err != nil {
			logger.Error("json serialization failed",
				"error", err.Error(),
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("failed to serialize transcripts: %s", err.Error())), nil
		}

		logger.Info("tool completed", "duration", time.Since(start), "count", len(transcripts))
		return mcp.NewToolResultText(string(jsonBytes)), nil
	}
}

// serializeTranscriptCollection projects a transcript collection through the
// tier's serializer. Neither tier inlines transcript text: the raw tier of a
// listing is a fuller description of each record, not the content of every one
// of them, which is what the single-transcript read is for.
//
// Parameters:
//   - resp: the collection response, which may be nil when Graph returned no body.
//   - outputMode: the resolved output tier.
//
// Returns one serialized map per transcript, in the order received.
func serializeTranscriptCollection(resp models.CallTranscriptCollectionResponseable, outputMode string) []map[string]any {
	if resp == nil {
		return []map[string]any{}
	}
	transcripts := make([]map[string]any, 0, len(resp.GetValue()))
	for _, transcript := range resp.GetValue() {
		if outputMode == "raw" {
			transcripts = append(transcripts, SerializeTranscriptMetadata(transcript))
		} else {
			transcripts = append(transcripts, SerializeSummaryTranscript(transcript))
		}
	}
	return transcripts
}
