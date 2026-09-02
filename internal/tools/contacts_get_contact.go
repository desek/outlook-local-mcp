// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the handler for the contacts get_contact verb, which reads
// one saved contact by identifier. It is the escalation target of a search
// match: search returns the identifier, and this verb returns the whole record
// behind it.
//
// @agents-index: Handler constructor for the contacts.get_contact verb, reading
// one saved contact by identifier and projecting it across the three output
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

// Corrections appended to this verb's refusals. The shared helpers state a
// diagnosis and none of them knows which parameter shaped the request, so the
// correction is authored here and emitted to both the tool result and the log
// record, keeping the two channels from drifting apart.
const (
	getContactAccountFix = "no account is selected: run the account domain's list operation and retry naming one with the account parameter"
	getContactIDFix      = "supply contact_id as the id field of a contacts search match or a list result"
	getContactTimeoutFix = "retry, and if the deadline is hit again resolve the contact with the contacts search operation instead"
	getContactGraphFix   = "check that contact_id names a contact in this mailbox and that OUTLOOK_MCP_CONTACTS_ENABLED was set before consent was granted, then retry; see docs/troubleshooting.md#authentication-failures"
)

// NewHandleGetContact creates the handler for the contacts get_contact verb. It
// reads GET /me/contacts/{id} and returns the contact's display name, every
// address it holds, and, at the raw tier, its phones and postal addresses.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for the Graph API call.
//
// Returns a tool handler function compatible with the MCP server's AddTool
// method.
//
// The handler validates contact_id before issuing any request, so a malformed
// identifier costs no Graph call, and issues exactly one request on the success
// path.
func NewHandleGetContact(retryCfg graph.RetryConfig, timeout time.Duration) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := logging.Logger(ctx)
		start := time.Now()

		client, err := GraphClient(ctx)
		if err != nil {
			logger.Error("no account selected", "fix", getContactAccountFix)
			return mcp.NewToolResultError(getContactAccountFix), nil
		}

		outputMode, err := ValidateOutputMode(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		contactID := request.GetString("contact_id", "")
		if err := validate.ValidateResourceID(contactID, "contact_id"); err != nil {
			logger.Error("contact_id rejected", "error", err.Error(), "fix", getContactIDFix)
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s", err.Error(), getContactIDFix)), nil
		}

		logger.Debug("tool called", "contact_id", contactID)

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		logger.Debug("graph API request", "endpoint", "GET /me/contacts/{id}")

		var contact models.Contactable
		graphErr := graph.RetryGraphCall(ctx, retryCfg, func() error {
			var callErr error
			contact, callErr = client.Me().Contacts().ByContactId(contactID).Get(timeoutCtx, nil)
			return callErr
		})
		if graphErr != nil {
			if graph.IsTimeoutError(graphErr) {
				logger.ErrorContext(ctx, "request timed out",
					"timeout_seconds", int(timeout.Seconds()),
					"fix", getContactTimeoutFix,
					"error", graphErr.Error())
				return mcp.NewToolResultError(fmt.Sprintf("%s: %s",
					graph.TimeoutErrorMessage(int(timeout.Seconds())), getContactTimeoutFix)), nil
			}
			logger.Error("graph API call failed",
				"error", graph.FormatGraphError(graphErr),
				"fix", getContactGraphFix,
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s",
				graph.RedactGraphError(graphErr), getContactGraphFix)), nil
		}

		logger.Debug("graph API response", "endpoint", "GET /me/contacts/{id}", "status", "ok")

		result := SerializeSummaryContact(contact)
		if outputMode == "raw" {
			result = SerializeContact(contact)
		}

		if outputMode == "text" {
			logger.Info("tool completed", "duration", time.Since(start))
			return mcp.NewToolResultText(FormatContactDetailText(result)), nil
		}

		jsonBytes, err := json.Marshal(result)
		if err != nil {
			logger.Error("json serialization failed",
				"error", err.Error(),
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("failed to serialize contact: %s", err.Error())), nil
		}

		logger.Info("tool completed", "duration", time.Since(start))
		return mcp.NewToolResultText(string(jsonBytes)), nil
	}
}
