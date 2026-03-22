// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the respond_event MCP tool, which responds to a meeting
// invitation on the authenticated user's calendar via the Microsoft Graph API.
// The tool supports three response types: accept, tentatively accept, and
// decline. An optional comment parameter allows the attendee to include a
// message to the organizer.
package tools

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/validate"
	"github.com/mark3labs/mcp-go/mcp"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
	graphusers "github.com/microsoftgraph/msgraph-sdk-go/users"
)

// NewRespondEventTool creates the MCP tool definition for respond_event. The
// tool accepts a required event_id, a required response type (accept,
// tentative, or decline), an optional comment string, an optional send_response
// boolean (defaults to true), and an optional account selector.
//
// Returns the configured mcp.Tool ready for registration with server.AddTool.
func NewRespondEventTool() mcp.Tool {
	return mcp.NewTool("calendar_respond_event",
		mcp.WithDescription(
			"Respond to a meeting invitation: accept, tentatively accept, or decline. "+
				"Sends a response to the organizer. Only applicable to events where you "+
				"are an attendee, not the organizer.",
		),
		mcp.WithString("event_id", mcp.Required(),
			mcp.Description("The unique identifier of the event to respond to."),
		),
		mcp.WithString("response", mcp.Required(),
			mcp.Description("Response type: 'accept', 'tentative', or 'decline'."),
		),
		mcp.WithString("comment",
			mcp.Description("Optional message to the organizer explaining your response."),
		),
		mcp.WithBoolean("send_response",
			mcp.Description("Whether to send the response to the organizer. Defaults to true."),
		),
		mcp.WithString("account",
			mcp.Description("Account label to use. If omitted, the default account is used. Use account_list to see available accounts."),
		),
	)
}

// HandleRespondEvent is the MCP tool handler for respond_event. It extracts the
// event_id, response type, optional comment, and optional send_response from
// the request arguments, then routes to the correct Graph API endpoint based on
// the response value:
//   - "accept"    -> POST /me/events/{id}/accept
//   - "tentative" -> POST /me/events/{id}/tentativelyAccept
//   - "decline"   -> POST /me/events/{id}/decline
//
// Each endpoint accepts a request body with Comment (string) and SendResponse
// (boolean). When send_response is omitted, it defaults to true.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for the Graph API call.
//
// Returns a closure matching the MCP tool handler function signature. The
// closure returns an *mcp.CallToolResult with either a plain text confirmation
// containing the response action and event ID, or an error result when
// validation fails or the Graph API call fails.
//
// Side effects: calls POST /me/events/{id}/{accept|tentativelyAccept|decline}
// on the Microsoft Graph API. Logs at debug level on entry, error level on
// failure, and info level on success.
func HandleRespondEvent(retryCfg graph.RetryConfig, timeout time.Duration) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := slog.With("tool", "calendar_respond_event")

		client, err := GraphClient(ctx)
		if err != nil {
			return mcp.NewToolResultError("no account selected"), nil
		}

		eventID, err := request.RequireString("event_id")
		if err != nil {
			return mcp.NewToolResultError("missing required parameter: event_id. Tip: Use calendar_list_events or calendar_search_events to find the event ID."), nil
		}
		if err := validate.ValidateResourceID(eventID, "event_id"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		response, err := request.RequireString("response")
		if err != nil {
			return mcp.NewToolResultError("missing required parameter: response. Valid values: 'accept', 'tentative', 'decline'."), nil
		}
		if response != "accept" && response != "tentative" && response != "decline" {
			return mcp.NewToolResultError(fmt.Sprintf("invalid response value: %q. Valid values: 'accept', 'tentative', 'decline'.", response)), nil
		}

		comment := request.GetString("comment", "")
		if comment != "" {
			if err := validate.ValidateStringLength(comment, "comment", validate.MaxCommentLen); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}

		sendResponse := true
		args := request.GetArguments()
		if sr, ok := args["send_response"].(bool); ok {
			sendResponse = sr
		}

		logger.DebugContext(ctx, "responding to event",
			"event_id", eventID,
			"response", response,
			"send_response", sendResponse,
		)

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		if timeoutCtx.Err() != nil {
			logger.ErrorContext(ctx, "request timed out",
				"timeout_seconds", int(timeout.Seconds()),
				"error", timeoutCtx.Err().Error())
			return mcp.NewToolResultError(graph.TimeoutErrorMessage(int(timeout.Seconds()))), nil
		}

		err = graph.RetryGraphCall(ctx, retryCfg, func() error {
			return postEventResponse(timeoutCtx, client, eventID, response, comment, sendResponse)
		})
		if err != nil {
			if graph.IsTimeoutError(err) {
				logger.ErrorContext(ctx, "request timed out",
					"timeout_seconds", int(timeout.Seconds()),
					"error", err.Error())
				return mcp.NewToolResultError(graph.TimeoutErrorMessage(int(timeout.Seconds()))), nil
			}
			logger.ErrorContext(ctx, "respond to event failed",
				"event_id", eventID,
				"response", response,
				"error", graph.FormatGraphError(err))
			return mcp.NewToolResultError(graph.RedactGraphError(err)), nil
		}

		logger.InfoContext(ctx, "event response sent",
			"event_id", eventID,
			"response", response,
		)

		// Map response values to human-readable labels.
		responseLabel := map[string]string{
			"accept":    "accepted",
			"tentative": "tentatively accepted",
			"decline":   "declined",
		}[response]

		result := fmt.Sprintf("Event %s: %s\nResponse sent to organizer.", responseLabel, eventID)
		return mcp.NewToolResultText(result), nil
	}
}

// postEventResponse routes the RSVP action to the correct Graph API endpoint
// based on the response type. It builds the appropriate SDK request body and
// calls the corresponding POST method.
//
// Parameters:
//   - ctx: the request context with timeout applied.
//   - client: the Graph service client for the authenticated user.
//   - eventID: the unique identifier of the event.
//   - response: one of "accept", "tentative", or "decline".
//   - comment: optional message to the organizer (empty string if not provided).
//   - sendResponse: whether to send the response notification to the organizer.
//
// Returns an error if the Graph API call fails.
func postEventResponse(ctx context.Context, client *msgraphsdk.GraphServiceClient, eventID, response, comment string, sendResponse bool) error {
	switch response {
	case "accept":
		body := graphusers.NewItemEventsItemAcceptPostRequestBody()
		if comment != "" {
			body.SetComment(&comment)
		}
		body.SetSendResponse(&sendResponse)
		return client.Me().Events().ByEventId(eventID).Accept().Post(ctx, body, nil)
	case "tentative":
		body := graphusers.NewItemEventsItemTentativelyAcceptPostRequestBody()
		if comment != "" {
			body.SetComment(&comment)
		}
		body.SetSendResponse(&sendResponse)
		return client.Me().Events().ByEventId(eventID).TentativelyAccept().Post(ctx, body, nil)
	case "decline":
		body := graphusers.NewItemEventsItemDeclinePostRequestBody()
		if comment != "" {
			body.SetComment(&comment)
		}
		body.SetSendResponse(&sendResponse)
		return client.Me().Events().ByEventId(eventID).Decline().Post(ctx, body, nil)
	default:
		return fmt.Errorf("invalid response type: %s", response)
	}
}
