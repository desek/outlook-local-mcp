package tools

import "strings"

// buildAdvisory evaluates whether an event with attendees is missing recommended
// fields (body and/or location) and returns a plain-text advisory string for LLM
// consumption. The advisory names the missing field(s) and instructs the LLM to
// offer the user the option to add them.
//
// Parameters:
//   - hasAttendees: whether the event has at least one attendee.
//   - hasBody: whether the event has a non-empty body/description.
//   - hasLocation: whether the event has a non-empty location.
//   - isOnlineMeeting: whether the event has is_online_meeting set to true,
//     which makes a missing location acceptable (Teams provides the location).
//
// Returns an advisory string describing what is missing, or an empty string if
// nothing is missing or the event has no attendees. The advisory is only produced
// when hasAttendees is true and at least one field is missing.
func buildAdvisory(hasAttendees, hasBody, hasLocation, isOnlineMeeting bool) string {
	if !hasAttendees {
		return ""
	}

	var missing []string
	if !hasBody {
		missing = append(missing, "description")
	}
	if !hasLocation && !isOnlineMeeting {
		missing = append(missing, "location")
	}

	if len(missing) == 0 {
		return ""
	}

	return "This event has attendees but is missing " +
		strings.Join(missing, " and ") +
		". Offer the user the option to update the event with the missing information."
}
