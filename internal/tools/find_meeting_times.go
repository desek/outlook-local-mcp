// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the handler for the calendar find_meeting_times verb,
// which asks Microsoft Graph to suggest candidate meeting slots from attendee
// constraints. It is a pure read: the request body carries the query, and the
// response is projected through the scheduling serializers.
//
// @agents-index: Handler constructor for the calendar.find_meeting_times verb,
// posting attendee and time constraints to findMeetingTimes and projecting the
// ranked suggestions across the three output tiers.
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
	"github.com/microsoft/kiota-abstractions-go/serialization"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/microsoftgraph/msgraph-sdk-go/users"
)

// defaultMeetingDurationISO is the meeting length applied when the caller
// supplies no meeting_duration. The request body always carries a duration so
// that the suggestions returned are for a stated length rather than one Graph
// picks silently.
const defaultMeetingDurationISO = "PT30M"

// Corrections appended to the two refusals this verb delegates to the shared
// Graph helpers. Those helpers state a diagnosis and stop there: the timeout
// message names the deadline, and the redactor returns Graph's own message with
// addresses removed. Neither knows which parameters shaped the request, so the
// correction is authored here, at the verb, where they are known. Appending
// rather than replacing keeps the deadline and the Graph code the caller acts
// on.
const (
	findMeetingTimesTimeoutFix = "narrow the search window with start_datetime and end_datetime, lower max_candidates, or shorten the attendees list, then retry"
	findMeetingTimesGraphFix   = "check that every address in attendees is a mailbox this account may read availability for, and that the account is a work or school account (personal Microsoft accounts are not supported), then retry"
)

// Candidate-count bounds for find_meeting_times. The default mirrors the
// max_results shape the calendar domain already publishes on its listings, and
// the ceiling keeps one call from returning a result set no reader will use.
const (
	defaultMaxCandidates = 20
	minMaxCandidates     = 1
	maxMaxCandidates     = 100
)

// FindMeetingTimesResponse is the JSON envelope the summary and raw output
// tiers return. The suggestions are carried as already-serialized maps so the
// tier decides the field set, and EmptySuggestionsReason explains a suggestion
// list Graph returned empty.
type FindMeetingTimesResponse struct {
	// Suggestions holds the serialized meeting-time suggestions in the rank
	// order Graph returned them.
	Suggestions []map[string]any `json:"suggestions"`
	// EmptySuggestionsReason is Graph's own explanation of why no slot was
	// offered. Omitted when Graph offered at least one suggestion or stated no
	// reason.
	EmptySuggestionsReason string `json:"emptySuggestionsReason,omitempty"`
}

// NewHandleFindMeetingTimes creates the handler for the find_meeting_times
// verb. It posts the caller's attendee list and optional constraints to
// POST /me/findMeetingTimes and returns the ranked suggestions.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for the Graph API call.
//   - defaultTimezone: the IANA timezone applied to the search window bounds
//     when the caller names no timezone, so the posted constraint is never
//     zone-ambiguous.
//
// Returns a tool handler function compatible with the MCP server's AddTool
// method.
//
// The handler refuses every invalid input before issuing any Graph request: a
// missing or malformed attendee list, an unparseable meeting_duration, a
// malformed window bound, a half-open window, and any numeric parameter
// outside its published bound.
func NewHandleFindMeetingTimes(retryCfg graph.RetryConfig, timeout time.Duration, defaultTimezone string) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

		attendeesJSON := request.GetString("attendees", "")
		if attendeesJSON == "" {
			return mcp.NewToolResultError("attendees is required: supply a JSON array of {\"email\":\"...\",\"type\":\"required|optional|resource\"} objects"), nil
		}
		attendees, err := parseAttendeeBases(attendeesJSON)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		durationParam := request.GetString("meeting_duration", defaultMeetingDurationISO)
		duration, err := serialization.ParseISODuration(durationParam)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("meeting_duration %q is not an ISO 8601 duration: supply a value such as PT30M or PT1H30M", durationParam)), nil
		}
		// A zero duration parses but serializes as the bare token "P", which is
		// not a valid Edm.Duration, so it is refused here rather than sent.
		if d, convErr := duration.ToDuration(); convErr != nil || d <= 0 {
			return mcp.NewToolResultError(fmt.Sprintf("meeting_duration %q must be longer than zero: supply a positive ISO 8601 duration such as PT30M, then verify the call returns suggestions", durationParam)), nil
		}

		timezone := request.GetString("timezone", "")
		if timezone != "" {
			if err := validate.ValidateTimezone(timezone, "timezone"); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}
		windowTimezone := timezone
		if windowTimezone == "" {
			windowTimezone = defaultTimezone
		}

		// The search window is both bounds or neither. A half-open window has
		// no honest reading: neither treating the missing bound as unbounded
		// nor inventing a length for it is what the caller asked for.
		startDatetime := request.GetString("start_datetime", "")
		endDatetime := request.GetString("end_datetime", "")
		if (startDatetime == "") != (endDatetime == "") {
			return mcp.NewToolResultError("start_datetime and end_datetime must be supplied together: provide both to bound the search window, or neither to let Graph apply its own"), nil
		}
		var timeConstraint models.TimeConstraintable
		if startDatetime != "" {
			if err := validate.ValidateDatetime(startDatetime, "start_datetime"); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if err := validate.ValidateDatetime(endDatetime, "end_datetime"); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			timeConstraint = buildTimeConstraint(startDatetime, endDatetime, windowTimezone)
		}

		args := request.GetArguments()

		maxCandidates := int32(request.GetFloat("max_candidates", defaultMaxCandidates))
		if _, supplied := args["max_candidates"]; supplied {
			if maxCandidates < minMaxCandidates || maxCandidates > maxMaxCandidates {
				return mcp.NewToolResultError(fmt.Sprintf("max_candidates must be between %d and %d, got %d", minMaxCandidates, maxMaxCandidates, maxCandidates)), nil
			}
		}

		body := users.NewItemFindMeetingTimesPostRequestBody()
		body.SetAttendees(attendees)
		body.SetMeetingDuration(duration)
		body.SetMaxCandidates(&maxCandidates)
		if timeConstraint != nil {
			body.SetTimeConstraint(timeConstraint)
		}

		// A parameter with no default is omitted from the body entirely, so
		// Graph's own default stays in force rather than being shadowed by one
		// this server would then have to keep correct.
		if _, supplied := args["minimum_attendee_percentage"]; supplied {
			pct := request.GetFloat("minimum_attendee_percentage", 0)
			if pct < 0 || pct > 100 {
				return mcp.NewToolResultError(fmt.Sprintf("minimum_attendee_percentage must be between 0 and 100, got %v", pct)), nil
			}
			body.SetMinimumAttendeePercentage(&pct)
		}
		if _, supplied := args["is_organizer_optional"]; supplied {
			organizerOptional := request.GetBool("is_organizer_optional", false)
			body.SetIsOrganizerOptional(&organizerOptional)
		}

		// Reasons are always requested: the summary tier publishes the reason
		// a slot was suggested, and without this flag Graph never returns one.
		returnReasons := true
		body.SetReturnSuggestionReasons(&returnReasons)

		logger.Debug("tool called",
			"attendees", len(attendees),
			"meeting_duration", durationParam,
			"max_candidates", maxCandidates,
			"bounded_window", timeConstraint != nil)

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		logger.Debug("graph API request", "endpoint", "POST /me/findMeetingTimes")

		var resp models.MeetingTimeSuggestionsResultable
		graphErr := graph.RetryGraphCall(ctx, retryCfg, func() error {
			var callErr error
			resp, callErr = client.Me().FindMeetingTimes().Post(timeoutCtx, body, nil)
			return callErr
		})
		if graphErr != nil {
			if graph.IsTimeoutError(graphErr) {
				logger.ErrorContext(ctx, "request timed out",
					"timeout_seconds", int(timeout.Seconds()),
					"fix", findMeetingTimesTimeoutFix,
					"error", graphErr.Error())
				return mcp.NewToolResultError(fmt.Sprintf("%s: %s",
					graph.TimeoutErrorMessage(int(timeout.Seconds())), findMeetingTimesTimeoutFix)), nil
			}
			logger.Error("graph API call failed",
				"error", graph.FormatGraphError(graphErr),
				"fix", findMeetingTimesGraphFix,
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s",
				graph.RedactGraphError(graphErr), findMeetingTimesGraphFix)), nil
		}

		logger.Debug("graph API response", "endpoint", "POST /me/findMeetingTimes", "status", "ok")

		result := FindMeetingTimesResponse{Suggestions: []map[string]any{}}
		if resp != nil {
			for _, suggestion := range resp.GetMeetingTimeSuggestions() {
				if outputMode == "raw" {
					result.Suggestions = append(result.Suggestions, graph.SerializeMeetingTimeSuggestion(suggestion))
				} else {
					result.Suggestions = append(result.Suggestions, graph.SerializeSummaryMeetingTimeSuggestion(suggestion))
				}
			}
			result.EmptySuggestionsReason = graph.SafeStr(resp.GetEmptySuggestionsReason())
		}

		if outputMode == "text" {
			logger.Info("tool completed",
				"duration", time.Since(start),
				"suggestions", len(result.Suggestions))
			return mcp.NewToolResultText(FormatMeetingTimeSuggestionsText(result)), nil
		}

		jsonBytes, err := json.Marshal(result)
		if err != nil {
			logger.Error("json serialization failed",
				"error", err.Error(),
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("failed to serialize response: %s", err.Error())), nil
		}

		logger.Info("tool completed",
			"duration", time.Since(start),
			"suggestions", len(result.Suggestions))
		return mcp.NewToolResultText(string(jsonBytes)), nil
	}
}

// parseAttendeeBases parses the same attendee JSON shape create_meeting
// publishes into the AttendeeBase slice findMeetingTimes takes. The "name"
// field is accepted and ignored, so one attendee list can be passed to both
// verbs unchanged.
//
// Parameters:
//   - jsonStr: a JSON array of attendee objects.
//
// Returns the attendee slice, or an error naming the offending entry when the
// JSON is malformed, an email fails validation, a type is not a recognised
// attendee type, or the list exceeds maxAttendees.
//
// Side effects: none.
func parseAttendeeBases(jsonStr string) ([]models.AttendeeBaseable, error) {
	var attendeeList []struct {
		Email string `json:"email"`
		Name  string `json:"name"`
		Type  string `json:"type"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &attendeeList); err != nil {
		return nil, fmt.Errorf("invalid attendees JSON: %w: supply a JSON array such as [{\"email\":\"a@example.com\",\"type\":\"required\"}]", err)
	}
	if len(attendeeList) == 0 {
		return nil, fmt.Errorf("attendees must name at least one attendee")
	}
	if len(attendeeList) > maxAttendees {
		return nil, fmt.Errorf("attendee count %d exceeds maximum of %d", len(attendeeList), maxAttendees)
	}

	result := make([]models.AttendeeBaseable, len(attendeeList))
	for i, a := range attendeeList {
		if err := validate.ValidateEmail(a.Email); err != nil {
			return nil, fmt.Errorf("attendee %d: %w", i, err)
		}
		if a.Type != "" {
			if err := validate.ValidateAttendeeType(a.Type); err != nil {
				return nil, fmt.Errorf("attendee %d: %w", i, err)
			}
		}
		att := models.NewAttendeeBase()
		email := models.NewEmailAddress()
		addr := a.Email
		email.SetAddress(&addr)
		att.SetEmailAddress(email)
		attType := graph.ParseAttendeeType(a.Type)
		att.SetTypeEscaped(&attType)
		result[i] = att
	}
	return result, nil
}

// buildTimeConstraint wraps a validated start and end into the single-slot
// TimeConstraint findMeetingTimes takes, stamping both bounds with the
// timezone the caller named or the server's configured one.
func buildTimeConstraint(startDatetime, endDatetime, timezone string) models.TimeConstraintable {
	slot := models.NewTimeSlot()
	slot.SetStart(newDateTimeTimeZone(startDatetime, timezone))
	slot.SetEnd(newDateTimeTimeZone(endDatetime, timezone))

	constraint := models.NewTimeConstraint()
	constraint.SetTimeSlots([]models.TimeSlotable{slot})
	return constraint
}

// newDateTimeTimeZone builds the Graph DateTimeTimeZone pair from an ISO 8601
// datetime string and an IANA timezone name.
func newDateTimeTimeZone(datetime, timezone string) models.DateTimeTimeZoneable {
	dtz := models.NewDateTimeTimeZone()
	dt := datetime
	tz := timezone
	dtz.SetDateTime(&dt)
	dtz.SetTimeZone(&tz)
	return dtz
}
