// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// @agents-index: Metadata-only size pre-check for calendar.get_event_attachment, refusing an attachment above the ceiling before its content is downloaded.
package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/logging"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/microsoftgraph/msgraph-sdk-go/users"
)

// checkEventAttachmentSize reads the attachment's metadata without its content
// and refuses it when the reported size exceeds maxSize.
//
// The request selects the lightweight fields only, so Graph does not return
// contentBytes and the ceiling bounds the download, not just the tool result.
// It returns nil when the content read may proceed, or the error result to
// return to the caller: the size refusal, a timeout, or a Graph failure.
func checkEventAttachmentSize(ctx, timeoutCtx context.Context, retryCfg graph.RetryConfig, rb *users.ItemEventsItemAttachmentsAttachmentItemRequestBuilder, maxSize int64, timeout time.Duration, start time.Time) *mcp.CallToolResult {
	logger := logging.Logger(ctx)
	cfg := &users.ItemEventsItemAttachmentsAttachmentItemRequestBuilderGetRequestConfiguration{
		QueryParameters: &users.ItemEventsItemAttachmentsAttachmentItemRequestBuilderGetQueryParameters{
			Select: listAttachmentsSelectFields,
		},
	}
	var meta models.Attachmentable
	err := graph.RetryGraphCall(ctx, retryCfg, func() error {
		var callErr error
		meta, callErr = rb.Get(timeoutCtx, cfg)
		return callErr
	})
	if err != nil {
		if graph.IsTimeoutError(err) {
			logger.ErrorContext(ctx, "request timed out", "timeout_seconds", int(timeout.Seconds()), "fix", getEventAttachmentTimeoutFix, "error", err.Error())
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s", graph.TimeoutErrorMessage(int(timeout.Seconds())), getEventAttachmentTimeoutFix))
		}
		logger.Error("graph API call failed", "error", graph.FormatGraphError(err), "fix", getEventAttachmentGraphFix, "duration", time.Since(start))
		return mcp.NewToolResultError(fmt.Sprintf("%s: %s", graph.RedactGraphError(err), getEventAttachmentGraphFix))
	}
	sz := meta.GetSize()
	if sz == nil || int64(*sz) <= maxSize {
		return nil
	}
	fix := fmt.Sprintf("raise OUTLOOK_MCP_MAX_ATTACHMENT_SIZE_BYTES above %d bytes and restart the server to download this attachment, then call get_event_attachment again and confirm the content is returned", int64(*sz))
	logger.Warn("attachment exceeds maximum size", "size", int64(*sz), "max", maxSize, "fix", fix)
	return mcp.NewToolResultError(fmt.Sprintf("attachment size %d bytes exceeds maximum allowed %d bytes; %s", int64(*sz), maxSize, fix))
}
