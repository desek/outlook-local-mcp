// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the handler for the calendar list_event_attachments verb,
// which enumerates the attachment metadata clipped to a calendar event. It is
// the event-side mirror of the mail collection read: the navigation differs,
// the field selection, the serializers, and the output tiers do not, so the
// two domains describe an attachment the same way. No content bytes are
// downloaded here; the caller fetches those by identifier with
// get_event_attachment.
//
// @agents-index: Handler constructor for the calendar.list_event_attachments
// verb, enumerating an event's attachment metadata with the shared lightweight
// field selection and no content bytes.
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
	"github.com/microsoftgraph/msgraph-sdk-go/users"
)

// Corrections appended to the refusals this verb delegates to a shared helper.
// The identifier validator names the malformed value, the timeout helper names
// the deadline, and the redactor returns Graph's own message with addresses
// removed: each diagnoses and none states what to do next, because none knows
// which verb shaped the request. The correction is therefore authored at the
// verb and appended, so the diagnosis and the fix travel together to both the
// tool result and the log record.
const (
	listEventAttachmentsIDFix      = "supply event_id as an identifier returned by calendar list_events, search_events, or get_event"
	listEventAttachmentsTimeoutFix = "retry the call, and if it times out again confirm the event still exists with calendar get_event"
	listEventAttachmentsGraphFix   = "check that event_id names an event this account can read, re-resolving it with calendar search_events if unsure, then retry"
)

// NewHandleListEventAttachments creates a tool handler that enumerates a
// calendar event's attachment metadata by calling
// GET /me/events/{event_id}/attachments with a $select restricted to the
// lightweight fields shared with the mail collection read, so no attachment
// content is transferred.
//
// The handler issues exactly one Graph request on the success path and
// preserves the order Graph returns, so repeating the call against an
// unchanged event yields byte-identical output at every tier.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for the Graph API call.
//
// Returns a tool handler function compatible with the MCP server's AddTool
// method.
func NewHandleListEventAttachments(retryCfg graph.RetryConfig, timeout time.Duration) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
			msg := fmt.Sprintf("missing required parameter: event_id: %s", listEventAttachmentsIDFix)
			logger.Warn("parameter validation failed", "parameter", "event_id", "fix", listEventAttachmentsIDFix)
			return mcp.NewToolResultError(msg), nil
		}
		if err := validate.ValidateResourceID(eventID, "event_id"); err != nil {
			msg := fmt.Sprintf("%s: %s", err.Error(), listEventAttachmentsIDFix)
			logger.Warn("parameter validation failed", "parameter", "event_id", "fix", listEventAttachmentsIDFix, "error", err.Error())
			return mcp.NewToolResultError(msg), nil
		}

		logger.Debug("tool called", "event_id", eventID, "output", outputMode)

		// The field selection is the package-level one the mail collection read
		// already uses: one list, so the two domains cannot drift into
		// describing an attachment differently.
		qp := &users.ItemEventsItemAttachmentsRequestBuilderGetQueryParameters{
			Select: listAttachmentsSelectFields,
		}
		cfg := &users.ItemEventsItemAttachmentsRequestBuilderGetRequestConfiguration{QueryParameters: qp}

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		var resp models.AttachmentCollectionResponseable
		graphErr := graph.RetryGraphCall(ctx, retryCfg, func() error {
			var callErr error
			resp, callErr = client.Me().Events().ByEventId(eventID).Attachments().Get(timeoutCtx, cfg)
			return callErr
		})
		if graphErr != nil {
			if graph.IsTimeoutError(graphErr) {
				logger.ErrorContext(ctx, "request timed out",
					"timeout_seconds", int(timeout.Seconds()),
					"fix", listEventAttachmentsTimeoutFix,
					"error", graphErr.Error())
				return mcp.NewToolResultError(fmt.Sprintf("%s: %s",
					graph.TimeoutErrorMessage(int(timeout.Seconds())), listEventAttachmentsTimeoutFix)), nil
			}
			logger.Error("graph API call failed",
				"error", graph.FormatGraphError(graphErr),
				"fix", listEventAttachmentsGraphFix,
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s",
				graph.RedactGraphError(graphErr), listEventAttachmentsGraphFix)), nil
		}

		raw := resp.GetValue()
		items := make([]map[string]any, 0, len(raw))
		for _, att := range raw {
			if outputMode == "raw" {
				items = append(items, graph.SerializeAttachment(att))
			} else {
				items = append(items, graph.SerializeSummaryAttachment(att))
			}
		}

		if outputMode == "text" {
			logger.Info("tool completed", "duration", time.Since(start), "count", len(items))
			return mcp.NewToolResultText(FormatAttachmentsText(items)), nil
		}

		jsonBytes, jErr := json.Marshal(items)
		if jErr != nil {
			logger.Error("json serialization failed", "error", jErr.Error())
			return mcp.NewToolResultError(fmt.Sprintf("failed to serialize attachments: %s", jErr.Error())), nil
		}
		logger.Info("tool completed", "duration", time.Since(start), "count", len(items))
		return mcp.NewToolResultText(string(jsonBytes)), nil
	}
}
