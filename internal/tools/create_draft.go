// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the mail_create_draft MCP tool, which creates a new
// message in the authenticated user's Drafts folder via POST /me/messages on
// the Microsoft Graph API. The draft is not sent: it is left in Drafts for
// the user to review, edit, and send manually from Outlook.
package tools

import (
	"context"
	"time"

	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/logging"
	"github.com/desek/outlook-local-mcp/internal/validate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

// NewCreateDraftTool creates the MCP tool definition for mail_create_draft.
// All parameters are optional; when all are omitted, an empty draft is
// created. The draft appears in the user's Outlook Drafts folder and is not
// sent automatically.
//
// Returns the configured mcp.Tool ready for registration with server.AddTool.
func NewCreateDraftTool() mcp.Tool {
	return mcp.NewTool("mail_create_draft",
		mcp.WithTitleAnnotation("Create Email Draft"),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithDescription(
			"Create a new email draft in the user's Drafts folder. The draft is "+
				"NOT sent automatically: it appears in the user's Outlook Drafts "+
				"folder for review, edit, and manual send.",
		),
		mcp.WithString("to_recipients",
			mcp.Description("Comma-separated list of To recipient email addresses."),
		),
		mcp.WithString("cc_recipients",
			mcp.Description("Comma-separated list of Cc recipient email addresses."),
		),
		mcp.WithString("bcc_recipients",
			mcp.Description("Comma-separated list of Bcc recipient email addresses."),
		),
		mcp.WithString("subject",
			mcp.Description("Draft subject line."),
		),
		mcp.WithString("body",
			mcp.Description("Draft body content. Plain text unless content_type is set to 'html'."),
		),
		mcp.WithString("content_type",
			mcp.Description("Body content type: 'text' (default) or 'html'."),
			mcp.Enum("text", "html"),
		),
		mcp.WithString("importance",
			mcp.Description("Draft importance: low, normal, or high."),
		),
		mcp.WithString("account",
			mcp.Description(AccountParamDescription),
		),
	)
}

// NewHandleCreateDraft creates the MCP tool handler for mail_create_draft.
// It validates optional recipient/body/importance parameters, constructs a
// models.Message, optionally stamps the MCP provenance extended property when
// provenancePropertyID is non-empty, and POSTs the message to /me/messages.
// The Graph API assigns a draft ID and stores the message in the Drafts
// folder without sending it.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for the Graph API call.
//   - provenancePropertyID: the full MAPI property ID for provenance tagging
//     (built via graph.BuildProvenancePropertyID). Empty string disables
//     provenance.
//
// Returns a handler function compatible with the MCP server AddTool signature.
//
// Side effects: calls POST /me/messages on the Microsoft Graph API. Logs at
// debug level on entry, error level on failure, and info level on success.
func NewHandleCreateDraft(retryCfg graph.RetryConfig, timeout time.Duration, provenancePropertyID string) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := logging.Logger(ctx)
		args := request.GetArguments()
		logger.DebugContext(ctx, "tool called")

		client, err := GraphClient(ctx)
		if err != nil {
			return mcp.NewToolResultError("no account selected"), nil
		}

		msg := models.NewMessage()

		// Recipients.
		for _, f := range []struct {
			key    string
			setter func([]models.Recipientable)
		}{
			{"to_recipients", msg.SetToRecipients},
			{"cc_recipients", msg.SetCcRecipients},
			{"bcc_recipients", msg.SetBccRecipients},
		} {
			raw, _ := args[f.key].(string)
			addrs, err := validate.ValidateRecipients(raw, f.key)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if len(addrs) > 0 {
				f.setter(BuildRecipients(addrs))
			}
		}

		// Subject.
		if subject, ok := args["subject"].(string); ok && subject != "" {
			if err := validate.ValidateStringLength(subject, "subject", validate.MaxSubjectLen); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			s := subject
			msg.SetSubject(&s)
		}

		// Body + content type.
		contentType, _ := args["content_type"].(string)
		if contentType != "" {
			if err := validate.ValidateContentType(contentType); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}
		if bodyStr, ok := args["body"].(string); ok && bodyStr != "" {
			if err := validate.ValidateStringLength(bodyStr, "body", validate.MaxBodyLen); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			msg.SetBody(BuildDraftBody(bodyStr, contentType))
		}

		// Importance.
		if impStr, ok := args["importance"].(string); ok && impStr != "" {
			if err := validate.ValidateImportance(impStr); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			imp := graph.ParseImportance(impStr)
			msg.SetImportance(&imp)
		}

		// Provenance tagging (opt-in via ProvenanceTag config).
		MaybeSetMailProvenance(msg, provenancePropertyID)

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		var created models.Messageable
		err = graph.RetryGraphCall(ctx, retryCfg, func() error {
			var gErr error
			created, gErr = client.Me().Messages().Post(timeoutCtx, msg, nil)
			return gErr
		})
		if err != nil {
			if graph.IsTimeoutError(err) {
				logger.ErrorContext(ctx, "request timed out",
					"timeout_seconds", int(timeout.Seconds()),
					"error", err.Error())
				return mcp.NewToolResultError(graph.TimeoutErrorMessage(int(timeout.Seconds()))), nil
			}
			logger.ErrorContext(ctx, "create draft failed", "error", graph.FormatGraphError(err))
			return mcp.NewToolResultError(graph.RedactGraphError(err)), nil
		}

		draftID := graph.SafeStr(created.GetId())
		draftSubject := graph.SafeStr(created.GetSubject())
		logger.InfoContext(ctx, "draft created", "draft_id", draftID)

		response := FormatDraftConfirmation("created", draftSubject, draftID)
		if line := AccountInfoLine(ctx); line != "" {
			response += "\n" + line
		}
		return mcp.NewToolResultText(response), nil
	}
}
