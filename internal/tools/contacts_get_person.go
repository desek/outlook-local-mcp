// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the handler for the contacts get_person verb, which reads
// one relevance-ranked person by identifier. A person is inferred by Graph from
// whom the user corresponds with rather than saved by the user, so it is the
// other half of a resolution that a saved contact cannot answer.
//
// @agents-index: Handler constructor for the contacts.get_person verb, reading
// one relevance-ranked person by identifier and projecting it across the three
// output tiers.
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

// Corrections appended to this verb's refusals, authored at the verb because
// the shared helpers state a diagnosis without knowing which parameter shaped
// the request, and emitted to both the tool result and the log record.
const (
	getPersonAccountFix = "no account is selected: run the account domain's list operation and retry naming one with the account parameter"
	getPersonIDFix      = "supply person_id as the id field of a contacts search match or a list_people result"
	getPersonTimeoutFix = "retry, and if the deadline is hit again resolve the person with the contacts search operation instead"
	getPersonGraphFix   = "check that person_id came from a recent search or list_people result, since relevance identifiers are not durable, then retry; see docs/troubleshooting.md#authentication-failures"
)

// NewHandleGetPerson creates the handler for the contacts get_person verb. It
// reads GET /me/people/{id} and returns the person's display name and every
// scored address Graph holds for them.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for the Graph API call.
//
// Returns a tool handler function compatible with the MCP server's AddTool
// method.
//
// The handler validates person_id before issuing any request and issues exactly
// one request on the success path. A person carries no per-address name, so the
// rendered label for every address is the person's own display name.
func NewHandleGetPerson(retryCfg graph.RetryConfig, timeout time.Duration) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := logging.Logger(ctx)
		start := time.Now()

		client, err := GraphClient(ctx)
		if err != nil {
			logger.Error("no account selected", "fix", getPersonAccountFix)
			return mcp.NewToolResultError(getPersonAccountFix), nil
		}

		outputMode, err := ValidateOutputMode(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		personID := request.GetString("person_id", "")
		if err := validate.ValidateResourceID(personID, "person_id"); err != nil {
			logger.Error("person_id rejected", "error", err.Error(), "fix", getPersonIDFix)
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s", err.Error(), getPersonIDFix)), nil
		}

		logger.Debug("tool called", "person_id", personID)

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		logger.Debug("graph API request", "endpoint", "GET /me/people/{id}")

		var person models.Personable
		graphErr := graph.RetryGraphCall(ctx, retryCfg, func() error {
			var callErr error
			person, callErr = client.Me().People().ByPersonId(personID).Get(timeoutCtx, nil)
			return callErr
		})
		if graphErr != nil {
			if graph.IsTimeoutError(graphErr) {
				logger.ErrorContext(ctx, "request timed out",
					"timeout_seconds", int(timeout.Seconds()),
					"fix", getPersonTimeoutFix,
					"error", graphErr.Error())
				return mcp.NewToolResultError(fmt.Sprintf("%s: %s",
					graph.TimeoutErrorMessage(int(timeout.Seconds())), getPersonTimeoutFix)), nil
			}
			logger.Error("graph API call failed",
				"error", graph.FormatGraphError(graphErr),
				"fix", getPersonGraphFix,
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s",
				graph.RedactGraphError(graphErr), getPersonGraphFix)), nil
		}

		logger.Debug("graph API response", "endpoint", "GET /me/people/{id}", "status", "ok")

		result := SerializeSummaryPerson(person)
		if outputMode == "raw" {
			result = SerializePerson(person)
		}

		if outputMode == "text" {
			logger.Info("tool completed", "duration", time.Since(start))
			return mcp.NewToolResultText(FormatPersonDetailText(result)), nil
		}

		jsonBytes, err := json.Marshal(result)
		if err != nil {
			logger.Error("json serialization failed",
				"error", err.Error(),
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("failed to serialize person: %s", err.Error())), nil
		}

		logger.Info("tool completed", "duration", time.Since(start))
		return mcp.NewToolResultText(string(jsonBytes)), nil
	}
}
