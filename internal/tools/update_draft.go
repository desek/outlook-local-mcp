// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the mail_update_draft MCP tool, which updates an
// existing draft message via PATCH /me/messages/{id} on the Microsoft Graph
// API. Only explicitly provided fields are set on the request body (PATCH
// semantics). The handler first GETs the message to verify isDraft=true and
// rejects updates on non-draft messages.
package tools

import (
	"context"
	"log/slog"
	"time"

	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/validate"
	"github.com/mark3labs/mcp-go/mcp"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/microsoftgraph/msgraph-sdk-go/users"
)

// NewUpdateDraftTool creates the MCP tool definition for mail_update_draft.
// It requires a message_id and accepts optional recipient, subject, body,
// content_type, and importance parameters. Only explicitly provided fields
// are updated.
//
// Returns the configured mcp.Tool ready for registration with server.AddTool.
func NewUpdateDraftTool() mcp.Tool {
	return mcp.NewTool("mail_update_draft",
		mcp.WithTitleAnnotation("Update Email Draft"),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithDescription(
			"Update an existing draft message in the Drafts folder. Only fields "+
				"explicitly provided are modified (PATCH semantics). The target "+
				"message MUST be a draft; updates on sent or received messages are "+
				"rejected. The draft is NOT sent automatically.",
		),
		mcp.WithString("message_id", mcp.Required(),
			mcp.Description("The unique identifier of the draft message to update."),
		),
		mcp.WithString("to_recipients",
			mcp.Description("Comma-separated list of To recipient email addresses (replaces existing)."),
		),
		mcp.WithString("cc_recipients",
			mcp.Description("Comma-separated list of Cc recipient email addresses (replaces existing)."),
		),
		mcp.WithString("bcc_recipients",
			mcp.Description("Comma-separated list of Bcc recipient email addresses (replaces existing)."),
		),
		mcp.WithString("subject",
			mcp.Description("New draft subject line."),
		),
		mcp.WithString("body",
			mcp.Description("New draft body content."),
		),
		mcp.WithString("content_type",
			mcp.Description("Body content type: 'text' or 'html'."),
			mcp.Enum("text", "html"),
		),
		mcp.WithString("importance",
			mcp.Description("New draft importance: low, normal, or high."),
		),
		mcp.WithString("account",
			mcp.Description(AccountParamDescription),
		),
	)
}

// NewHandleUpdateDraft creates the MCP tool handler for mail_update_draft. It
// first GETs the target message with a narrow $select to verify isDraft=true,
// then PATCHes the message with only explicitly provided fields.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for a single Graph API call.
//
// Returns a handler function compatible with the MCP server AddTool signature.
//
// Side effects: calls GET /me/messages/{id} then PATCH /me/messages/{id} on
// the Microsoft Graph API.
func NewHandleUpdateDraft(retryCfg graph.RetryConfig, timeout time.Duration) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := slog.With("tool", "mail_update_draft")

		client, err := GraphClient(ctx)
		if err != nil {
			return mcp.NewToolResultError("no account selected"), nil
		}

		messageID, err := request.RequireString("message_id")
		if err != nil {
			return mcp.NewToolResultError("missing required parameter: message_id"), nil
		}
		if err := validate.ValidateResourceID(messageID, "message_id"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Step 1: verify isDraft=true.
		if errResult := verifyIsDraft(ctx, client, retryCfg, timeout, messageID, logger); errResult != nil {
			return errResult, nil
		}

		// Step 2: build PATCH body from explicitly provided fields.
		args := request.GetArguments()
		patch := models.NewMessage()
		anyField := false

		for _, f := range []struct {
			key    string
			setter func([]models.Recipientable)
		}{
			{"to_recipients", patch.SetToRecipients},
			{"cc_recipients", patch.SetCcRecipients},
			{"bcc_recipients", patch.SetBccRecipients},
		} {
			raw, ok := args[f.key].(string)
			if !ok {
				continue
			}
			addrs, err := validate.ValidateRecipients(raw, f.key)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			f.setter(BuildRecipients(addrs))
			anyField = true
		}

		if subject, ok := args["subject"].(string); ok {
			if err := validate.ValidateStringLength(subject, "subject", validate.MaxSubjectLen); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			s := subject
			patch.SetSubject(&s)
			anyField = true
		}

		contentType, _ := args["content_type"].(string)
		if contentType != "" {
			if err := validate.ValidateContentType(contentType); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}
		if bodyStr, ok := args["body"].(string); ok {
			if err := validate.ValidateStringLength(bodyStr, "body", validate.MaxBodyLen); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			patch.SetBody(BuildDraftBody(bodyStr, contentType))
			anyField = true
		}

		if impStr, ok := args["importance"].(string); ok && impStr != "" {
			if err := validate.ValidateImportance(impStr); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			imp := graph.ParseImportance(impStr)
			patch.SetImportance(&imp)
			anyField = true
		}

		if !anyField {
			return mcp.NewToolResultError("no updatable fields provided"), nil
		}

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		var updated models.Messageable
		err = graph.RetryGraphCall(ctx, retryCfg, func() error {
			var gErr error
			updated, gErr = client.Me().Messages().ByMessageId(messageID).Patch(timeoutCtx, patch, nil)
			return gErr
		})
		if err != nil {
			if graph.IsTimeoutError(err) {
				logger.ErrorContext(ctx, "request timed out",
					"timeout_seconds", int(timeout.Seconds()),
					"error", err.Error())
				return mcp.NewToolResultError(graph.TimeoutErrorMessage(int(timeout.Seconds()))), nil
			}
			logger.ErrorContext(ctx, "update draft failed", "error", graph.FormatGraphError(err))
			return mcp.NewToolResultError(graph.RedactGraphError(err)), nil
		}

		draftID := graph.SafeStr(updated.GetId())
		draftSubject := graph.SafeStr(updated.GetSubject())
		logger.InfoContext(ctx, "draft updated", "draft_id", draftID)

		response := FormatDraftConfirmation("updated", draftSubject, draftID)
		if line := AccountInfoLine(ctx); line != "" {
			response += "\n" + line
		}
		return mcp.NewToolResultText(response), nil
	}
}

// verifyIsDraft fetches the target message with a narrow $select and returns
// an MCP error result if the message is not found, not a draft, or the GET
// call fails. Returns nil when the message is confirmed to be a draft.
//
// Parameters:
//   - ctx: the request context.
//   - client: the Graph service client.
//   - retryCfg: retry configuration.
//   - timeout: request timeout for the verification call.
//   - messageID: the message identifier to verify.
//   - logger: the handler-scoped logger.
//
// Returns an *mcp.CallToolResult when verification fails, or nil when the
// message is a draft.
//
// Side effects: calls GET /me/messages/{id} on the Microsoft Graph API.
func verifyIsDraft(ctx context.Context, client *msgraphsdk.GraphServiceClient, retryCfg graph.RetryConfig, timeout time.Duration, messageID string, logger *slog.Logger) *mcp.CallToolResult {
	timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
	defer cancel()

	cfg := &users.ItemMessagesMessageItemRequestBuilderGetRequestConfiguration{
		QueryParameters: &users.ItemMessagesMessageItemRequestBuilderGetQueryParameters{
			Select: []string{"id", "isDraft"},
		},
	}
	var msg models.Messageable
	err := graph.RetryGraphCall(ctx, retryCfg, func() error {
		var gErr error
		msg, gErr = client.Me().Messages().ByMessageId(messageID).Get(timeoutCtx, cfg)
		return gErr
	})
	if err != nil {
		if graph.IsTimeoutError(err) {
			logger.ErrorContext(ctx, "request timed out", "timeout_seconds", int(timeout.Seconds()))
			return mcp.NewToolResultError(graph.TimeoutErrorMessage(int(timeout.Seconds())))
		}
		logger.ErrorContext(ctx, "isDraft verification failed", "error", graph.FormatGraphError(err))
		return mcp.NewToolResultError(graph.RedactGraphError(err))
	}
	if !graph.SafeBool(msg.GetIsDraft()) {
		return mcp.NewToolResultError("message is not a draft: this tool only operates on messages with isDraft=true")
	}
	return nil
}
