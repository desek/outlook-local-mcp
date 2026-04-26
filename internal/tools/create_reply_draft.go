// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the mail_create_reply_draft MCP tool, which creates a
// reply draft (with correct threading headers) via POST
// /me/messages/{id}/createReply or /createReplyAll on the Microsoft Graph
// API. The draft is not sent: it is left in the user's Drafts folder for
// review and manual send.
package tools

import (
	"context"
	"strings"
	"time"

	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/logging"
	"github.com/desek/outlook-local-mcp/internal/validate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/microsoftgraph/msgraph-sdk-go/users"
)

// invalidReferenceReplyMessage is returned when Graph rejects createReply with
// ErrorInvalidReferenceItem — typically because message_id refers to a draft
// or another item type that cannot be replied to. The default Graph message
// ("The reference item does not support the requested operation") is opaque;
// this replacement gives the caller an actionable next step.
const invalidReferenceReplyMessage = "Cannot create a reply to this message. " +
	"The message_id may refer to a draft or an item that does not support replies. " +
	"Use mail_update_draft to edit a draft, or supply the ID of a sent or received message."

// NewCreateReplyDraftTool creates the MCP tool definition for
// mail_create_reply_draft. It requires a message_id identifying the source
// message, and accepts an optional comment (prepended to the quoted original)
// and an optional reply_all boolean (default false).
//
// Returns the configured mcp.Tool ready for registration with server.AddTool.
func NewCreateReplyDraftTool() mcp.Tool {
	return mcp.NewTool("mail_create_reply_draft",
		mcp.WithTitleAnnotation("Create Email Reply Draft"),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithDescription(
			"Create a reply draft to an existing message, preserving threading "+
				"headers. The draft is NOT sent automatically: it appears in the "+
				"user's Outlook Drafts folder for review, edit, and manual send. "+
				"message_id must reference a sent or received message; replying "+
				"to a draft is not supported by Graph and will error — use "+
				"mail_update_draft to edit a draft instead.",
		),
		mcp.WithString("message_id", mcp.Required(),
			mcp.Description("The unique identifier of the source message to reply to."),
		),
		mcp.WithString("comment",
			mcp.Description("Optional reply body text prepended to the quoted original message."),
		),
		mcp.WithBoolean("reply_all",
			mcp.Description("When true, reply to all original recipients (To + Cc). Default false."),
		),
		mcp.WithString("account",
			mcp.Description(AccountParamDescription),
		),
	)
}

// NewHandleCreateReplyDraft creates the MCP tool handler for
// mail_create_reply_draft. It routes to createReply or createReplyAll based
// on the reply_all flag. When provenancePropertyID is non-empty, a follow-up
// PATCH stamps the provenance extended property on the returned draft
// (Graph API does not support extended properties on the createReply request
// body directly).
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for a single Graph API call.
//   - provenancePropertyID: the full MAPI property ID for provenance tagging.
//     Empty string disables the follow-up PATCH.
//
// Returns a handler function compatible with the MCP server AddTool signature.
//
// Side effects: calls POST /me/messages/{id}/createReply or /createReplyAll,
// and optionally PATCH /me/messages/{draftID} for provenance.
func NewHandleCreateReplyDraft(retryCfg graph.RetryConfig, timeout time.Duration, provenancePropertyID string) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := logging.Logger(ctx)

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

		args := request.GetArguments()
		comment, _ := args["comment"].(string)
		replyAll, _ := args["reply_all"].(bool)

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		var created models.Messageable
		if replyAll {
			body := users.NewItemMessagesItemCreateReplyAllPostRequestBody()
			if comment != "" {
				c := comment
				body.SetComment(&c)
			}
			err = graph.RetryGraphCall(ctx, retryCfg, func() error {
				var gErr error
				created, gErr = client.Me().Messages().ByMessageId(messageID).CreateReplyAll().Post(timeoutCtx, body, nil)
				return gErr
			})
		} else {
			body := users.NewItemMessagesItemCreateReplyPostRequestBody()
			if comment != "" {
				c := comment
				body.SetComment(&c)
			}
			err = graph.RetryGraphCall(ctx, retryCfg, func() error {
				var gErr error
				created, gErr = client.Me().Messages().ByMessageId(messageID).CreateReply().Post(timeoutCtx, body, nil)
				return gErr
			})
		}
		if err != nil {
			if graph.IsTimeoutError(err) {
				logger.ErrorContext(ctx, "request timed out",
					"timeout_seconds", int(timeout.Seconds()),
					"error", err.Error())
				return mcp.NewToolResultError(graph.TimeoutErrorMessage(int(timeout.Seconds()))), nil
			}
			logger.ErrorContext(ctx, "create reply draft failed", "error", graph.FormatGraphError(err))
			if strings.Contains(graph.FormatGraphError(err), "ErrorInvalidReferenceItem") {
				return mcp.NewToolResultError(invalidReferenceReplyMessage), nil
			}
			return mcp.NewToolResultError(graph.RedactGraphError(err)), nil
		}

		draftID := graph.SafeStr(created.GetId())
		draftSubject := graph.SafeStr(created.GetSubject())

		// Follow-up PATCH to stamp provenance on the created draft. Failures here
		// are logged but do not mask the successful draft creation.
		if provenancePropertyID != "" && draftID != "" {
			patch := models.NewMessage()
			MaybeSetMailProvenance(patch, provenancePropertyID)
			patchCtx, patchCancel := graph.WithTimeout(ctx, timeout)
			pErr := graph.RetryGraphCall(ctx, retryCfg, func() error {
				_, e := client.Me().Messages().ByMessageId(draftID).Patch(patchCtx, patch, nil)
				return e
			})
			patchCancel()
			if pErr != nil {
				logger.WarnContext(ctx, "provenance patch failed", "draft_id", draftID, "error", graph.FormatGraphError(pErr))
			}
		}

		logger.InfoContext(ctx, "reply draft created", "draft_id", draftID, "reply_all", replyAll)

		response := FormatDraftConfirmation("created", draftSubject, draftID)
		if line := AccountInfoLine(ctx); line != "" {
			response += "\n" + line
		}
		return mcp.NewToolResultText(response), nil
	}
}
