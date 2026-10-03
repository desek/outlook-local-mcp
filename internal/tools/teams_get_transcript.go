// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the handler for the teams get_transcript verb, which reads
// one meeting transcript: its metadata and its WEBVTT text.
//
// Graph publishes no preview field on a callTranscript, so a preview can only be
// a truncation of text that was already fetched. The verb therefore issues the
// same two requests in every mode, the metadata read and the content read, and
// what the output tier decides is how much of the fetched text is returned, not
// how many requests were made. The alternative, returning metadata alone by
// default, is cheaper but leaves the model nothing to judge the escalation from,
// which is the purpose the body-escalation rule states.
//
// @agents-index: Handler constructor for the teams.get_transcript verb, reading
// one transcript's metadata and content with the full WEBVTT text gated behind
// the raw output tier.
package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/logging"
	"github.com/desek/outlook-local-mcp/internal/validate"
	"github.com/mark3labs/mcp-go/mcp"
	abstractions "github.com/microsoft/kiota-abstractions-go"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/microsoftgraph/msgraph-sdk-go/models/odataerrors"
	"github.com/microsoftgraph/msgraph-sdk-go/users"
)

// Corrections appended to this verb's refusals. Both identifiers are named
// separately, and each names the verb that produces it, because a caller holding
// one of the two cannot derive the other.
const (
	getTranscriptAccountFix        = "no account is selected: run the account domain's list operation and retry naming one with the account parameter"
	getTranscriptMeetingIDFix      = "supply meeting_id as the id returned by the teams get_online_meeting operation, which resolves the join URL a calendar get_event result carries"
	getTranscriptTranscriptIDFix   = "supply transcript_id as the id field of a teams list_transcripts entry for this meeting"
	getTranscriptTimeoutFix        = "retry, and if the deadline is hit again confirm both identifiers with the teams list_transcripts operation"
	getTranscriptAccessDisabledFix = transcriptAccessDisabledFix
	getTranscriptGraphFix          = "check that meeting_id and transcript_id name a transcript of a meeting this account can read and that consent was granted after OUTLOOK_MCP_TEAMS_ENABLED was set, so OnlineMeetingTranscript.Read.All was requested, then retry; see docs/troubleshooting.md#authentication-failures"
)

// transcriptAccessDisabledFix is the correction for a tenant that has turned
// off Graph API access to meeting transcripts. The cause is a tenant policy, not
// this account's consent or the request, so the text says a retry cannot work
// and names the administrator action instead. It is shared with the transcript
// listing, which receives the same refusal for the same reason.
const transcriptAccessDisabledFix = "the tenant administrator has turned off Graph API access to meeting transcripts, and no retry, consent, or request change can work around it; ask a Teams administrator to re-enable transcript API access in the Teams admin center or with Set-CsTeamsMeetingConfiguration, then verify by running the teams list_transcripts operation for a meeting that has a transcript"

// Inner-error codes Graph returns on a 403 from the transcript endpoints. The
// handlers branch on these codes, not the message text, because the message is
// not a stable contract.
const (
	innerCodeSpeakerAttributionNotAllowed     = "SpeakerAttributionNotAllowed"
	innerCodeGraphAccessToTranscriptsDisabled = "GraphAccessToTranscriptsDisabled"
)

// The transcript content formats. WEBVTT carries speaker names and is the
// documented default; the plain transcript format is the unattributed fallback a
// tenant that disables speaker attribution still serves.
const (
	transcriptFormatVTT          = "text/vtt"
	transcriptFormatUnattributed = "application/vnd.microsoft.graph.transcript+text"
)

// The two endpoints this verb reads, stated once each so a failure names the
// request that actually failed rather than the pair.
const (
	getTranscriptMetadataEndpoint = "GET /me/onlineMeetings/{meeting-id}/transcripts/{transcript-id}"
	getTranscriptContentEndpoint  = "GET /me/onlineMeetings/{meeting-id}/transcripts/{transcript-id}/content"
)

// NewHandleGetTranscript creates the handler for the teams get_transcript verb.
// It reads the transcript's metadata and then its content, returning a truncated
// preview of the text by default and the whole WEBVTT document under output=raw.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for the Graph API calls, applied to the pair
//     so the verb's total deadline is the one the caller configured.
//
// Returns a tool handler function compatible with the MCP server's AddTool
// method.
//
// Both identifiers are validated before either request is issued, so a malformed
// one costs no Graph call.
func NewHandleGetTranscript(retryCfg graph.RetryConfig, timeout time.Duration) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := logging.Logger(ctx)
		start := time.Now()

		client, err := GraphClient(ctx)
		if err != nil {
			logger.Error("no account selected", "fix", getTranscriptAccountFix)
			return mcp.NewToolResultError(getTranscriptAccountFix), nil
		}

		outputMode, err := ValidateOutputMode(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		meetingID := request.GetString("meeting_id", "")
		if err := validate.ValidateResourceID(meetingID, "meeting_id"); err != nil {
			logger.Error("meeting_id rejected", "error", err.Error(), "fix", getTranscriptMeetingIDFix)
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s", err.Error(), getTranscriptMeetingIDFix)), nil
		}

		transcriptID := request.GetString("transcript_id", "")
		if err := validate.ValidateResourceID(transcriptID, "transcript_id"); err != nil {
			logger.Error("transcript_id rejected", "error", err.Error(), "fix", getTranscriptTranscriptIDFix)
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s", err.Error(), getTranscriptTranscriptIDFix)), nil
		}

		logger.Debug("tool called", "meeting_id", meetingID, "transcript_id", transcriptID)

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		builder := client.Me().OnlineMeetings().ByOnlineMeetingId(meetingID).Transcripts().ByCallTranscriptId(transcriptID)

		logger.Debug("graph API request", "endpoint", getTranscriptMetadataEndpoint)

		var transcript models.CallTranscriptable
		graphErr := graph.RetryGraphCall(ctx, retryCfg, func() error {
			var callErr error
			transcript, callErr = builder.Get(timeoutCtx, nil)
			return callErr
		})
		if graphErr != nil {
			return transcriptGraphFailure(ctx, logger, getTranscriptMetadataEndpoint, timeout, start, graphErr), nil
		}

		logger.Debug("graph API request", "endpoint", getTranscriptContentEndpoint)

		var content []byte
		format := transcriptFormatVTT
		readContent := func() error {
			return graph.RetryGraphCall(ctx, retryCfg, func() error {
				var callErr error
				content, callErr = transcriptContent(timeoutCtx, builder, format)
				return callErr
			})
		}
		graphErr = readContent()
		// A tenant that disables speaker attribution refuses the attributed
		// format; the documented recovery is one retry in the unattributed one.
		if graphErr != nil && transcriptInnerErrorCode(graphErr) == innerCodeSpeakerAttributionNotAllowed {
			logger.Warn("speaker attribution not allowed, retrying unattributed",
				"endpoint", getTranscriptContentEndpoint, "format", transcriptFormatUnattributed)
			format = transcriptFormatUnattributed
			graphErr = readContent()
		}
		if graphErr != nil {
			return transcriptGraphFailure(ctx, logger, getTranscriptContentEndpoint, timeout, start, graphErr), nil
		}

		logger.Debug("graph API response",
			"endpoint", getTranscriptContentEndpoint,
			"content_bytes", len(content))

		result := SerializeTranscript(transcript, content, outputMode == "raw")
		if len(result) > 0 {
			// The format tells a reader whether speaker names can be present.
			result["contentFormat"] = format
		}

		if outputMode == "text" {
			logger.Info("tool completed", "duration", time.Since(start))
			return mcp.NewToolResultText(FormatTranscriptDetailText(result)), nil
		}

		jsonBytes, err := json.Marshal(result)
		if err != nil {
			logger.Error("json serialization failed",
				"error", err.Error(),
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("failed to serialize transcript: %s", err.Error())), nil
		}

		logger.Info("tool completed", "duration", time.Since(start))
		return mcp.NewToolResultText(string(jsonBytes)), nil
	}
}

// transcriptContent reads the transcript's WEBVTT bytes. It is separated from
// the handler so the content read and the metadata read differ only in the call
// they make, and share one failure path.
//
// Parameters:
//   - ctx: the request context, already carrying the call's deadline.
//   - builder: the request builder addressing the transcript.
//   - format: the media type sent as Accept, which selects the transcript
//     format; without it the SDK sends a generic default that does not name one.
//
// Returns the content bytes, which are empty when the transcript holds none.
func transcriptContent(ctx context.Context, builder *users.ItemOnlineMeetingsItemTranscriptsCallTranscriptItemRequestBuilder, format string) ([]byte, error) {
	headers := abstractions.NewRequestHeaders()
	headers.Add("Accept", format)
	return builder.Content().Get(ctx, &users.ItemOnlineMeetingsItemTranscriptsItemContentRequestBuilderGetRequestConfiguration{Headers: headers})
}

// transcriptInnerErrorCode returns the innerError.code of a Graph OData error,
// or "" when there is none. The SDK models innerError without a code field, so
// the code arrives in the inner error's additional data.
func transcriptInnerErrorCode(err error) string {
	var odataErr *odataerrors.ODataError
	if !errors.As(err, &odataErr) || odataErr.GetErrorEscaped() == nil || odataErr.GetErrorEscaped().GetInnerError() == nil {
		return ""
	}
	switch code := odataErr.GetErrorEscaped().GetInnerError().GetAdditionalData()["code"].(type) {
	case *string:
		if code != nil {
			return *code
		}
	case string:
		return code
	}
	return ""
}

// transcriptGraphFailure renders a failed Graph call as a tool error carrying
// the fix, and records the same fix on the log record. It takes the endpoint so
// a reader learns which of the verb's two requests failed, which is the
// difference between a transcript this account cannot address and one whose text
// the service declined to return.
//
// Parameters:
//   - ctx: the request context, for the timeout log record.
//   - logger: the request logger.
//   - endpoint: the endpoint whose call failed.
//   - timeout: the configured deadline, reported in the timeout message.
//   - start: when the handler began, for the recorded duration.
//   - graphErr: the failure.
//
// Returns the tool result to hand back to the caller.
//
// Side effects: writes one error log record.
func transcriptGraphFailure(ctx context.Context, logger *slog.Logger, endpoint string, timeout time.Duration, start time.Time, graphErr error) *mcp.CallToolResult {
	if graph.IsTimeoutError(graphErr) {
		logger.ErrorContext(ctx, "request timed out",
			"endpoint", endpoint,
			"timeout_seconds", int(timeout.Seconds()),
			"fix", getTranscriptTimeoutFix,
			"error", graphErr.Error())
		return mcp.NewToolResultError(fmt.Sprintf("%s: %s",
			graph.TimeoutErrorMessage(int(timeout.Seconds())), getTranscriptTimeoutFix))
	}
	fix := getTranscriptGraphFix
	if transcriptInnerErrorCode(graphErr) == innerCodeGraphAccessToTranscriptsDisabled {
		fix = getTranscriptAccessDisabledFix
	}
	logger.Error("graph API call failed",
		"endpoint", endpoint,
		"error", graph.FormatGraphError(graphErr),
		"fix", fix,
		"duration", time.Since(start))
	return mcp.NewToolResultError(fmt.Sprintf("%s: %s",
		graph.RedactGraphError(graphErr), fix))
}
