// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the handler for the calendar get_event_attachment verb,
// which downloads one attachment clipped to a calendar event, returning its
// metadata and, for a file attachment, its base64-encoded content. It is the
// event-side mirror of the mail item read, including the configurable size
// ceiling that refuses an oversized attachment rather than allocating it.
//
// @agents-index: Handler constructor for the calendar.get_event_attachment
// verb, downloading one event attachment's metadata and base64 content under
// the configured maximum attachment size.
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

// Corrections appended to the refusals this verb delegates to a shared helper,
// and the one it raises itself. The identifier validator and the redactor each
// state a diagnosis without a next step, and the size refusal must name the
// knob that raises the ceiling, because a caller cannot otherwise tell a
// refusal from a failure.
const (
	getEventAttachmentEventIDFix      = "supply event_id as an identifier returned by calendar list_events, search_events, or get_event"
	getEventAttachmentAttachmentIDFix = "supply attachment_id as an id returned by calendar list_event_attachments for this event"
	getEventAttachmentTimeoutFix      = "retry the call, and if it times out again list the event's attachments first to confirm the attachment is still present"
	getEventAttachmentGraphFix        = "check that event_id and attachment_id name an event and attachment this account can read, re-resolving them with calendar list_event_attachments, then retry"
)

// NewHandleGetEventAttachment creates a tool handler that retrieves one
// calendar event attachment's metadata and base64-encoded content by calling
// GET /me/events/{event_id}/attachments/{attachment_id}.
//
// The maxSize parameter bounds the attachment's reported size: an attachment
// above it is refused with an error naming the limit and the environment
// variable that raises it, so a single tool call cannot allocate unbounded
// memory. The handler issues exactly one Graph request on the success path.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for the Graph API call.
//   - maxSize: the maximum allowed attachment size in bytes. Values <=0 are
//     treated as unlimited.
//
// Returns a tool handler function compatible with the MCP server's AddTool
// method.
func NewHandleGetEventAttachment(retryCfg graph.RetryConfig, timeout time.Duration, maxSize int64) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := logging.Logger(ctx)
		start := time.Now()

		client, err := GraphClient(ctx)
		if err != nil {
			return mcp.NewToolResultError("no account selected"), nil
		}

		outputMode, err := ValidateOutputMode(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		eventID, err := request.RequireString("event_id")
		if err != nil || eventID == "" {
			logger.Warn("parameter validation failed", "parameter", "event_id", "fix", getEventAttachmentEventIDFix)
			return mcp.NewToolResultError(fmt.Sprintf("missing required parameter: event_id: %s", getEventAttachmentEventIDFix)), nil
		}
		if err := validate.ValidateResourceID(eventID, "event_id"); err != nil {
			logger.Warn("parameter validation failed", "parameter", "event_id", "fix", getEventAttachmentEventIDFix, "error", err.Error())
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s", err.Error(), getEventAttachmentEventIDFix)), nil
		}

		attachmentID, err := request.RequireString("attachment_id")
		if err != nil || attachmentID == "" {
			logger.Warn("parameter validation failed", "parameter", "attachment_id", "fix", getEventAttachmentAttachmentIDFix)
			return mcp.NewToolResultError(fmt.Sprintf("missing required parameter: attachment_id: %s", getEventAttachmentAttachmentIDFix)), nil
		}
		if err := validate.ValidateResourceID(attachmentID, "attachment_id"); err != nil {
			logger.Warn("parameter validation failed", "parameter", "attachment_id", "fix", getEventAttachmentAttachmentIDFix, "error", err.Error())
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s", err.Error(), getEventAttachmentAttachmentIDFix)), nil
		}

		logger.Debug("tool called", "event_id", eventID, "attachment_id", attachmentID, "output", outputMode)

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		var att models.Attachmentable
		graphErr := graph.RetryGraphCall(ctx, retryCfg, func() error {
			var callErr error
			att, callErr = client.Me().Events().ByEventId(eventID).Attachments().ByAttachmentId(attachmentID).Get(timeoutCtx, nil)
			return callErr
		})
		if graphErr != nil {
			if graph.IsTimeoutError(graphErr) {
				logger.ErrorContext(ctx, "request timed out",
					"timeout_seconds", int(timeout.Seconds()),
					"fix", getEventAttachmentTimeoutFix,
					"error", graphErr.Error())
				return mcp.NewToolResultError(fmt.Sprintf("%s: %s",
					graph.TimeoutErrorMessage(int(timeout.Seconds())), getEventAttachmentTimeoutFix)), nil
			}
			logger.Error("graph API call failed",
				"error", graph.FormatGraphError(graphErr),
				"fix", getEventAttachmentGraphFix,
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s",
				graph.RedactGraphError(graphErr), getEventAttachmentGraphFix)), nil
		}

		// The ceiling is checked against the size Graph reports, before any
		// serialization, so an oversized attachment is refused rather than
		// copied into the result.
		if maxSize > 0 {
			if sz := att.GetSize(); sz != nil && int64(*sz) > maxSize {
				fix := fmt.Sprintf("raise OUTLOOK_MCP_MAX_ATTACHMENT_SIZE_BYTES above %d bytes and restart the server to download this attachment", int64(*sz))
				logger.Warn("attachment exceeds maximum size", "size", int64(*sz), "max", maxSize, "fix", fix)
				return mcp.NewToolResultError(fmt.Sprintf(
					"attachment size %d bytes exceeds maximum allowed %d bytes; %s", int64(*sz), maxSize, fix)), nil
			}
		}

		result := graph.SerializeAttachment(att)

		if outputMode == "text" {
			logger.Info("tool completed", "duration", time.Since(start), "attachment_id", attachmentID)
			return mcp.NewToolResultText(FormatAttachmentText(result)), nil
		}

		if outputMode == "summary" {
			result = graph.SerializeSummaryAttachment(att)
		}

		jsonBytes, jErr := json.Marshal(result)
		if jErr != nil {
			logger.Error("json serialization failed", "error", jErr.Error())
			return mcp.NewToolResultError(fmt.Sprintf("failed to serialize attachment: %s", jErr.Error())), nil
		}
		logger.Info("tool completed", "duration", time.Since(start), "attachment_id", attachmentID)
		return mcp.NewToolResultText(string(jsonBytes)), nil
	}
}
