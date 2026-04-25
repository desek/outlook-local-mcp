# MCP Tool CRUD Lifecycle Test

Step-by-step instruction for Claude Code to exercise the MCP tools through a complete create-read-update-delete cycle with verification at each stage.

**Invocation shape:** After CR-0060, all tools use the aggregate domain tool with an `operation` verb. The shape is `{tool: "<domain>", args: {operation: "<verb>", ...}}`. Examples:

- `{tool: "calendar", args: {operation: "list_calendars"}}`
- `{tool: "mail", args: {operation: "help"}}`
- `{tool: "account", args: {operation: "list"}}`
- `{tool: "system", args: {operation: "status", output: "summary"}}`

## Prerequisites

- The MCP server `outlookCalendar` is running and connected.
- At least one account is authenticated (verify with `{tool: "account", args: {operation: "list"}}`).
- The server **must** be configured with `LOG_LEVEL=debug` and file logging enabled (`LOG_FILE` set). Both are verified in Step 0.

## Instructions

Follow every step sequentially. Use the **default account** (omit `account` param) unless the user specifies otherwise. Omit the `output` parameter for all read operations (the default is `text`) unless a step specifies otherwise.

**Always call `help` first.** Before invoking any verb in a domain you have not yet exercised, call `{tool: "<domain>", args: {operation: "help"}}` to enumerate the available verbs **and their parameters**. The help output lists every parameter's exact name, type, required/optional status, and (where applicable) accepted enum values. Use those exact parameter names — do not guess (`id` vs `message_id` vs `event_id` differ between verbs and inventing names will surface as `missing required parameter` errors at call time). When a step's parameter spec disagrees with `help`, trust `help` and report the discrepancy in the findings section of the report.

Pick a test date **7 days from today** to avoid conflicts with real events. Use the timezone `Europe/Amsterdam` for all operations.

### Step 0 -- Discover and verify connectivity

**0a.** Call `{tool: "system", args: {operation: "help"}}`.

- **Verify:** The response is plain text listing the available system verbs (at least `help`, `status`, `list_docs`, `search_docs`, `get_docs`).
- **Purpose:** Exercises the help verb and confirms the server is responding and that the docs verbs are registered (CR-0061 AC-1).
- **Fail:** Stop and report if the help verb errors or if any docs verb is missing.

**0a2.** Call `{tool: "system", args: {operation: "list_docs"}}`.

- **Verify:** The response is plain text listing at least three slugs: `readme`, `quickstart`, and `troubleshooting`.
- **Purpose:** Exercises the `list_docs` verb (CR-0061 AC-1).
- **Fail:** Report if the verb errors or the expected slugs are absent.

**0a3.** Call `{tool: "system", args: {operation: "search_docs", query: "token refresh"}}`.

- **Verify:** The response includes at least one result with a snippet containing "token" or "refresh" and references the `troubleshooting` slug.
- **Purpose:** Exercises the `search_docs` verb with a known query (CR-0061 AC-2).
- **Fail:** Report if the verb errors. Zero results for this query is a failure.

**0a4.** Call `{tool: "system", args: {operation: "get_docs", slug: "troubleshooting", section: "token-refresh"}}`.

- **Verify:** The response is plain text containing the content of the `## Token refresh` section from the troubleshooting guide.
- **Verify:** The response does NOT include sections from other parts of the document (e.g., `## Graph 429 throttling`).
- **Purpose:** Exercises the `get_docs` verb with section slicing (CR-0061 AC-3).
- **Fail:** Report if the verb errors or if the section content is missing or incorrect.

**0a5.** Call `{tool: "system", args: {operation: "get_docs", slug: "troubleshooting", output: "raw"}}`.

- **Verify:** The response is raw Markdown text (starts with `# Troubleshooting`).
- **Purpose:** Exercises the `raw` output mode of `get_docs` (CR-0061 AC-3).
- **Fail:** Report if the verb errors.

**0b.** Call `{tool: "system", args: {operation: "status", output: "summary"}}` (the full JSON config is needed for this verification step).

- **Verify:** At least one account is listed with an authenticated status.
- **Verify:** The response contains a `docs` object with `base_uri="doc://outlook-local-mcp/"`, `troubleshooting_slug="troubleshooting"`, and a `version` field (CR-0061 AC-5).
- **Fail:** Stop and report the authentication issue or if the `docs` section is absent.

**0c.** Record the top-level status fields and the `config` object from the Step 0b JSON response.

- **Record:** `version` as the **server version**.
- **Record:** `timezone` as the **default timezone**.
- **Record:** `server_uptime_seconds` as the **uptime**.
- **Verify:** `config.logging.log_file` is a non-empty string. Record this as the **log file path** for Step 26.
- **Verify:** `config.logging.log_level` is `"debug"`. If not, stop and ask the user to set `LOG_LEVEL=debug`.
- **Record:** `config.logging.log_format` as the **log format**.
- **Record:** `config.logging.log_sanitize` as the **PII sanitization** setting.
- **Record:** `config.logging.audit_log_enabled` as the **audit logging** setting.
- **Record:** `config.identity.auth_method` and `config.identity.auth_method_source` as the **auth method** and its **source**.
- **Record:** `config.identity.client_id` and `config.identity.tenant_id` as the **identity config**.
- **Record:** `config.storage.token_cache_backend` (either `"keychain"` or `"file"`) as the **auth cache type**.
- **Record:** `config.features.read_only` as the **read-only mode** setting.
- **Record:** `config.features.provenance_tag` as the **provenance tag**.
- **Record:** `config.graph_api.max_retries` and `config.graph_api.request_timeout_seconds` as the **Graph API settings**.
- **Fail:** Stop and report if `config.logging.log_file` is empty or `config.logging.log_level` is not `"debug"`.

**0d.** Call `{tool: "system", args: {operation: "status"}}` (default `text` mode, no `output` param).

- **Verify:** The response is plain text (not JSON).
- **Verify:** The text includes: server version, timezone, uptime, account list with auth state, and feature flags.
- **Verify:** The full configuration details (logging paths, Graph API settings, identity config) are NOT present in the text output.
- **Fail:** If the default response is JSON or if essential health fields are missing from the text.

### Step 1 -- List accounts

Call `{tool: "account", args: {operation: "list"}}`.

- **Verify:** The response is plain text (not JSON) listing accounts with labels and authentication state.
- **Verify:** At least one account shows an authenticated status.
- **Record:** The number of accounts and their labels for the environment report.
- **Record:** If **two or more** accounts show authenticated status, set **multi-account mode** to `true`. Record the first authenticated account that is NOT the default as the **attendee account label**.
- If only one account is authenticated, set **multi-account mode** to `false`.
- **Fail:** If no accounts are returned or none are authenticated.

### Step 2 -- List calendars

Call `{tool: "calendar", args: {operation: "list_calendars"}}`.

- **Verify:** The response is plain text listing calendars.
- **Verify:** At least one calendar is present (the default calendar).
- **Record:** The default calendar name and ID.
- **Fail:** If no calendars are returned.

**If multi-account mode:** Also call `{tool: "calendar", args: {operation: "list_calendars", account: "<attendee account label>"}}`.

- **Verify:** The response is plain text listing the attendee's calendars.
- **Record:** The `owner` email address from the attendee's default calendar as the **attendee email**. If the email cannot be determined from the text response, call again with `output: "summary"` to extract the email, or ask the user for the attendee's email address.
- **Fail:** If the attendee account's calendars cannot be listed (the account may not be properly authenticated).

### Step 2a -- Discover calendar operations via help

Call `{tool: "calendar", args: {operation: "help"}}`.

- **Verify:** The response is plain text listing all registered calendar verbs (at least `help`, `list_calendars`, `list_events`, `get_event`, `search_events`, `create_event`, `update_event`, `delete_event`, `respond_event`, `reschedule_event`, `create_meeting`, `update_meeting`, `cancel_meeting`, `reschedule_meeting`, `get_free_busy`).
- **Purpose:** Exercises the help verb for the calendar domain (AC-2 / FR-4 / FR-15).
- **Fail:** If any of the listed verbs is absent from the help output.

### Step 3 -- Baseline list

Call `{tool: "calendar", args: {operation: "list_events", date: "<test date ISO 8601>"}}`.

- **Verify:** The response is plain text (not JSON).
- Record the number of events from the total count line. This is the **baseline count**.
- Note any existing event subjects to avoid collisions.

### Step 4 -- Create event

Call `{tool: "calendar", args: {operation: "create_event", ...}}` with:

| Parameter        | Value                                           |
|------------------|--------------------------------------------------|
| `subject`        | `MCP CRUD Test -- <timestamp>` (use current epoch seconds for uniqueness) |
| `start_datetime` | Test date at `14:00:00`                          |
| `end_datetime`   | Test date at `14:30:00`                          |
| `start_timezone` | `Europe/Amsterdam`                               |
| `end_timezone`   | `Europe/Amsterdam`                               |
| `location`       | `Test Room`                                      |
| `body`           | `Automated CRUD lifecycle test`                  |
| `show_as`        | `free`                                           |

- **Pass:** Response is a plain text confirmation containing the event subject and an `ID:` line.
- Save the returned **event ID** from the `ID:` line -- all subsequent steps depend on it.
- Report the created event subject and ID.

### Step 5 -- Provenance search (created event)

Call `{tool: "calendar", args: {operation: "search_events", created_by_mcp: true, query: "<timestamp portion>", date: "<test date>"}}`.

- **Verify:** The text results contain the event created in Step 4 (match by event ID in the text).
- **Verify:** The `created_by_mcp` filter correctly narrows results to MCP-created events only.
- **Fail:** If the event is missing, the provenance tag was not stamped during creation.

### Step 6 -- Search with "next_week" date shorthand

Call `{tool: "calendar", args: {operation: "search_events", query: "<timestamp portion>", date: "next_week"}}`.

- **Verify:** The text results contain the event created in Step 4 (match by event ID in the text).
- **Fail:** If the event is not found, the `next_week` date shorthand is not resolving correctly.

### Step 7 -- Search with "this_week" date shorthand

Call `{tool: "calendar", args: {operation: "search_events", query: "<timestamp portion>", date: "this_week"}}`.

- **Verify:** The results do NOT contain the event created in Step 4 (the test date is 7 days from today, outside the current week).
- **Fail:** If the event appears, the `this_week` date boundary is incorrect.

### Step 8 -- Get created event

Call `{tool: "calendar", args: {operation: "get_event", event_id: "<saved event ID>"}}`.

- **Verify:** The response is plain text (not JSON).
- **Verify:**
  - Subject matches what was sent in Step 4.
  - Location shows `Test Room`.
  - Start time corresponds to test date `14:00` in `Europe/Amsterdam`.
  - Show As is `free`.
  - A body preview line is present (containing text from the `body` parameter in Step 4).
- **Fail:** Report any mismatched field.

### Step 9 -- Update event

Call `{tool: "calendar", args: {operation: "update_event", ...}}` with:

| Parameter        | Value                              |
|------------------|------------------------------------|
| `event_id`       | Saved event ID                     |
| `subject`        | Append ` (updated)` to original subject |
| `location`       | `Updated Room`                     |
| `end_datetime`   | Test date at `15:00:00`            |
| `end_timezone`   | `Europe/Amsterdam`                 |
| `show_as`        | `busy`                             |
| `body`           | `<h2>Agenda</h2><ol><li>Verify CRUD operations</li><li>Review test results</li></ol>` |

- **Pass:** Response is a plain text confirmation containing `Event updated:` and the event subject.

### Step 10 -- Get updated event and verify body escalation

**10a.** Call `{tool: "calendar", args: {operation: "get_event", event_id: "<saved event ID>"}}` (default `text` output).

- **Verify:** The response is plain text (not JSON).
- **Verify:**
  - Subject ends with `(updated)`.
  - Location shows `Updated Room`.
  - End time corresponds to test date `15:00` in `Europe/Amsterdam`.
  - Show As is `busy`.
  - Start time is **unchanged** (still `14:00`).
  - A body preview line is present containing `Agenda` and the agenda items text (plain-text snippet, not HTML).
- **Fail:** Report any mismatched field.

**10b.** Call `{tool: "calendar", args: {operation: "get_event", event_id: "<saved event ID>", output: "raw"}}`.

- **Verify:** The response is JSON (not plain text).
- **Verify:** The `body.content` field contains the full HTML body set in Step 9 (including the `<h2>Agenda</h2>` and `<ol>` tags).
- **Verify:** The `bodyPreview` field is also present as a plain-text snippet.
- **Purpose:** This confirms the body escalation pattern — `bodyPreview` in default text mode is sufficient to determine whether the full HTML body retrieval via `output=raw` is needed.
- **Fail:** If the full HTML body is not present in raw mode, or if the text default in Step 10a leaked HTML tags.

### Step 11 -- Get free/busy

Call `{tool: "calendar", args: {operation: "get_free_busy", date: "<test date>"}}`.

- **Verify:** The response is plain text showing schedule availability.
- **Verify:** The text contains a busy period that overlaps with the test event's time range (14:00–15:00 Europe/Amsterdam).
- **Verify:** The busy period's status is `busy`.
- **Fail:** If no busy period is found covering the test event time, or the status does not match.

### Step 12 -- Reschedule event

Call `{tool: "calendar", args: {operation: "reschedule_event", event_id: "<saved event ID>", new_start_datetime: "<test date>T17:00:00", new_start_timezone: "Europe/Amsterdam"}}`.

- **Pass:** Response is a plain text confirmation containing `Event rescheduled:` and the event subject.

### Step 13 -- Get rescheduled event

Call `{tool: "calendar", args: {operation: "get_event", event_id: "<saved event ID>"}}`.

- **Verify:** The response is plain text.
- **Verify:**
  - Start time corresponds to test date `17:00` in `Europe/Amsterdam`.
  - End time corresponds to test date `18:00` in `Europe/Amsterdam` (original 1-hour duration preserved from Step 9's update).
  - Subject is **unchanged** (still ends with `(updated)`).
  - Location is **unchanged** (still `Updated Room`).
- **Fail:** Report any mismatched field or if duration was not preserved.

### Step 14 -- Delete event

Call `{tool: "calendar", args: {operation: "delete_event", event_id: "<saved event ID>"}}`.

- **Pass:** Response is plain text containing `Event deleted:` and the event ID.

### Step 15 -- Get deleted event (expect failure)

Call `{tool: "calendar", args: {operation: "get_event", event_id: "<saved event ID>"}}`.

- **Pass:** The call returns an error or "not found" response.
- **Fail:** If the event is still returned, report that deletion did not take effect.

### Step 16 -- Provenance search (after deletion)

Call `{tool: "calendar", args: {operation: "search_events", created_by_mcp: true, query: "<timestamp portion>", date: "<test date>"}}`.

- **Verify:** The deleted event does NOT appear in the results.
- **Fail:** If the event still appears in provenance search after deletion.

### Step 17 -- Verify list after deletion

Call `{tool: "calendar", args: {operation: "list_events", date: "<test date>"}}`.

- **Verify:** The response is plain text.
- **Verify:** The test event subject does NOT appear in the results.
- **Verify:** The event count from the total count line is equal to the **baseline count** from Step 3.
- **Fail:** Report if the deleted event still appears.

### Step 18 -- Create Teams meeting

> **Note:** In multi-account mode, this step uses `create_meeting` (the meeting variant) because attendees are involved. The LLM should present a draft summary (subject, date/time, attendee list, location, body preview) and wait for user confirmation before calling the tool. If any attendee email domain differs from the user's own domain, the LLM should also display an external attendee warning. Confirm when prompted.

**If multi-account mode**, call `{tool: "calendar", args: {operation: "create_meeting", ...}}` with:

| Parameter           | Value                                           |
|---------------------|--------------------------------------------------|
| `subject`           | `MCP Teams Test -- <timestamp>` (use current epoch seconds for uniqueness) |
| `start_datetime`    | Test date at `16:00:00`                          |
| `end_datetime`      | Test date at `16:30:00`                          |
| `start_timezone`    | `Europe/Amsterdam`                               |
| `end_timezone`      | `Europe/Amsterdam`                               |
| `is_online_meeting` | `true`                                           |
| `body`              | `Automated Teams meeting test`                   |
| `show_as`           | `free`                                           |
| `attendees`         | `[{"email":"<attendee email>","name":"Attendee","type":"required"}]` |

**If single-account mode**, call `{tool: "calendar", args: {operation: "create_meeting", ...}}` with the same parameters but set `attendees` to a single self-addressed entry using the authenticated account's own UPN: `[{"email":"<self UPN>","name":"Self","type":"required"}]`. A self-attendee is required because Microsoft Graph only provisions a Teams `onlineMeeting` resource (and populates `joinUrl`) when the event is created via `create_meeting` with at least one attendee. The LLM must still present a confirmation summary before invoking the tool, and the resulting self-invitation email is expected test artifact noise.

- **Pass:** Response is a plain text confirmation containing the event subject and an `ID:` line.
- Save the returned **Teams event ID** from the `ID:` line.
- Report the created event subject and ID.

### Step 19 -- Verify Teams meeting details

Call `{tool: "calendar", args: {operation: "get_event", event_id: "<saved Teams event ID>"}}` (default `text` output).

- **Primary evidence (Pass requires all):**
  - The text output indicates the event is an online meeting (e.g., a Teams join URL line, an `Online meeting:` field, or a Teams link in the body preview). This is the observable proof Graph provisioned a Teams meeting.
  - The body preview or an explicit join URL field references `teams.microsoft.com` or a Teams join link.
- **Single-account mode, also verify:**
  - The attendee section lists exactly one attendee whose email matches the authenticated account's own UPN.
- **Multi-account mode, also verify:**
  - The attendee section lists at least one entry with the external attendee email.
- **Escalate only if needed:** If text output does not surface a join URL or attendee detail, re-call with `output: "raw"` to inspect `onlineMeeting.joinUrl` and `attendees` directly. Note the escalation in the report.
- **Fail:** If no Teams join URL is observable in text or raw output, Teams promotion did not happen; report the failure (commonly caused by calling `create_event` instead of `create_meeting`, or omitting attendees).

### Step 20 -- Verify invitation on attendee calendar

> **Multi-account only.** If single-account mode, mark this step **SKIP**.

Call `{tool: "calendar", args: {operation: "search_events", account: "<attendee account label>", query: "<timestamp portion>", date: "<test date>"}}`.

- **Verify:** The text results contain the Teams meeting created in Step 18 (match by subject).
- **Record:** The **attendee event ID** from the text result. If the ID is not visible in the text, call again with `output: "summary"` to extract it (it may differ from the organizer's event ID).
- **Fail:** If the meeting does not appear on the attendee's calendar, the invitation was not delivered.

### Step 21 -- Respond from attendee

> **Multi-account only.** If single-account mode, mark this step **SKIP**.

Call `{tool: "calendar", args: {operation: "respond_event", account: "<attendee account label>", event_id: "<attendee event ID>", response: "tentative", comment: "CRUD test -- tentative response", send_response: true}}`.

- **Pass:** Response is plain text containing `Event tentatively accepted:` and the event ID.
- **Fail:** If the call returns an error.

### Step 22 -- Verify attendee response from organizer

> **Multi-account only.** If single-account mode, mark this step **SKIP**.

Call `{tool: "calendar", args: {operation: "get_event", event_id: "<saved Teams event ID>"}}` (default `text` output).

- **Verify:** The attendee section shows the attendee email with a tentative response status.
- **Escalate only if needed:** If the response status is not visible in text, re-call with `output: "raw"` and check `attendees[].status.response == "tentativelyAccepted"`. Note the escalation in the report.
- **Fail:** If the attendee's response status has not updated.

### Step 22a -- Update meeting (meeting verb)

> **Multi-account only.** If single-account mode, mark this step **SKIP**.

> **Note:** This step uses `update_meeting` (the meeting variant) because the event has attendees. The LLM should present a draft summary of the changes and affected attendees, then wait for user confirmation before calling the tool. Confirm when prompted.

Call `{tool: "calendar", args: {operation: "update_meeting", event_id: "<saved Teams event ID>", subject: "<original subject> (meeting updated)", body: "Updated meeting agenda -- CRUD test"}}`.

- **Pass:** Response is a plain text confirmation containing `Event updated:` and the event subject.

### Step 22b -- Verify meeting update

> **Multi-account only.** If single-account mode, mark this step **SKIP**.

Call `{tool: "calendar", args: {operation: "get_event", event_id: "<saved Teams event ID>"}}`.

- **Verify:** Subject ends with `(meeting updated)`.
- **Verify:** Body preview contains `Updated meeting agenda`.
- **Verify:** Start time and end time are unchanged from Step 18.
- **Fail:** Report any mismatched field.

### Step 22c -- Reschedule meeting (meeting verb)

> **Multi-account only.** If single-account mode, mark this step **SKIP**.

> **Note:** This step uses `reschedule_meeting` (the meeting variant) because the event has attendees. The LLM should present a summary showing the event subject, current time, proposed new time, and attendee list, then wait for user confirmation. Confirm when prompted.

Call `{tool: "calendar", args: {operation: "reschedule_meeting", event_id: "<saved Teams event ID>", new_start_datetime: "<test date>T17:30:00", new_start_timezone: "Europe/Amsterdam"}}`.

- **Pass:** Response is a plain text confirmation containing `Event rescheduled:` and the event subject.

### Step 22d -- Verify meeting reschedule

> **Multi-account only.** If single-account mode, mark this step **SKIP**.

Call `{tool: "calendar", args: {operation: "get_event", event_id: "<saved Teams event ID>"}}`.

- **Verify:** Start time corresponds to test date `17:30` in `Europe/Amsterdam`.
- **Verify:** End time corresponds to test date `18:00` in `Europe/Amsterdam` (original 30-minute duration preserved from Step 18).
- **Verify:** Subject is unchanged (still ends with `(meeting updated)`).
- **Fail:** Report any mismatched field or if duration was not preserved.

### Step 23 -- Respond to own meeting (expect failure)

Call `{tool: "calendar", args: {operation: "respond_event", event_id: "<saved Teams event ID>", response: "accept", comment: "CRUD test -- organizer self-response"}}`.

- **Pass:** The call returns an error (the authenticated user is the organizer, not an attendee; responding to your own meeting is not permitted).
- **Fail:** If the call succeeds, the server is not enforcing the organizer/attendee distinction.

### Step 24 -- Cancel Teams meeting

> **Note:** This event has attendees (in multi-account mode). The LLM should present a summary (subject, time, attendee list) and wait for user confirmation before calling the tool. If any attendee is external, the LLM should also display an external attendee warning. Confirm when prompted.

Call `{tool: "calendar", args: {operation: "cancel_meeting", event_id: "<saved Teams event ID>", comment: "Automated CRUD test cancellation"}}`.

- **Pass:** Response is plain text containing `Event cancelled:` and the event ID.

### Step 25 -- Verify cancellation

Call `{tool: "calendar", args: {operation: "get_event", event_id: "<saved Teams event ID>"}}`.

- **Pass:** The call returns an error or "not found" response (cancelled meetings are removed from the calendar).
- **Fail:** If the event is still returned as a non-cancelled event.

**If multi-account mode**, also call `{tool: "calendar", args: {operation: "get_event", account: "<attendee account label>", event_id: "<attendee event ID>"}}`.

- **Verify:** The call returns an error/"not found", or the event shows `isCancelled: true`.
- **Fail:** If the event is still active on the attendee's calendar.

### Step 26 -- Verify server logs

Read the **log file path** recorded in Step 0c. Inspect the log entries emitted during the test (from Step 1 onward).

- **Verify:** Every tool call has a `DEBUG`-level "tool called" entry and an `INFO`-level (or `ERROR` for Steps 15, 23, 25) "tool completed" entry.
- **Verify:** The `calendar.create_event` audit entry includes the event ID.
- **Verify:** The `calendar.delete_event` audit entry includes the event ID and confirms deletion.
- **Verify:** The `calendar.reschedule_event` audit entry includes the event ID.
- **Verify:** The `calendar.cancel_meeting` audit entry includes the event ID.
- **If multi-account mode:** Verify the `calendar.create_meeting` audit entry (Step 18) includes the event ID.
- **If multi-account mode:** Verify the `calendar.update_meeting` audit entry (Step 22a) includes the event ID.
- **If multi-account mode:** Verify the `calendar.reschedule_meeting` audit entry (Step 22c) includes the event ID.
- **Verify:** Audit entries use the fully-qualified `{domain}.{operation}` format (e.g., `calendar.delete_event`), not the old `calendar_delete_event` style.
- **Verify:** The `calendar.get_event` call after deletion (Step 15) is logged at `ERROR` level with `ErrorItemNotFound`.
- **Verify:** The `calendar.respond_event` call (Step 23) is logged at `ERROR` level.
- **If multi-account mode:** Verify the `calendar.respond_event` call (Step 21) is logged at `INFO` level (success).
- **Verify:** No unexpected `ERROR` or `WARN` entries appear (the Step 15, 23, and 25 errors are expected; Step 20 attendee-side errors in multi-account mode from Step 25 are also expected).
- **Fail:** Report any missing log entries or unexpected errors.

### Step 27 -- Force refresh authenticated account token

Call `{tool: "account", args: {operation: "refresh", label: "<default account label>"}}`.

- **Pass:** Response is plain text confirming the refresh and including a new token expiry timestamp.
- **Verify:** The response references the account's label and/or UPN.
- **Fail:** If the call errors or the expiry time is absent from the response.

### Step 28 -- Log out of account

> **Note:** This test requires at least one non-default account in addition to the default account, or `account login` in Step 29 must be used to restore access before further tests. If only one account is registered, mark Steps 28 and 29 **SKIP** to avoid leaving the test environment unauthenticated.

Pick a **non-default authenticated account** from Step 1's list (the **attendee account label** in multi-account mode). Call `{tool: "account", args: {operation: "logout", label: "<non-default account label>"}}`.

- **Pass:** Response is plain text confirming the logout.
- **Verify:** A subsequent `{tool: "account", args: {operation: "list"}}` call shows the account as `disconnected` while still listing the entry (not removed).
- **Verify:** Calling any calendar tool with `account: <logged-out label>` returns an actionable error mentioning `disconnected` and `login`.
- **Fail:** If the account is removed, still shown as authenticated, or if the disconnected-account error is missing.

### Step 29 -- Log back in to account

Call `{tool: "account", args: {operation: "login", label: "<label from Step 28>"}}`.

Complete the authentication flow interactively when prompted (browser, device code, or auth code, per the account's persisted auth method).

- **Pass:** Response is plain text confirming re-authentication, including the account's UPN.
- **Verify:** A subsequent `{tool: "account", args: {operation: "list"}}` call shows the account back as `authenticated`.
- **Verify:** Calling `{tool: "account", args: {operation: "login", label: "<label>"}}` again on the same (now connected) account returns an error stating the account is already connected.
- **Fail:** If the account does not return to the authenticated state or the already-connected guard does not trigger.

### Step 29a -- Durable account removal (skip if single-account mode)

> **Multi-account only.** If single-account mode, mark this step **SKIP**.

This step verifies that `account.remove` is durable across server restart when `accounts.json` contains an entry for the removed label (CR-0064 AC-4).

1. Call `{tool: "account", args: {operation: "list"}}` and record the full set of registered account labels.
2. Pick any non-default account that has a persisted `accounts.json` entry (for example the attendee account from Step 1). Record its label as `<remove-target>`.
3. Call `{tool: "account", args: {operation: "remove", label: "<remove-target>"}}`.
   - **Pass:** Response is plain text confirming removal including the label and "Token cache cleared."
   - **Verify:** A subsequent `{tool: "account", args: {operation: "list"}}` does not include `<remove-target>`.
4. **Restart the server** (stop and start the `outlook-local-mcp` process).
5. After restart, call `{tool: "account", args: {operation: "list"}}` again.
   - **Pass:** `<remove-target>` is absent from the account list.
   - **Fail:** If `<remove-target>` reappears, `accounts.json` was not rewritten correctly.
6. **Restore:** Call `{tool: "account", args: {operation: "add", label: "<remove-target>", ...}}` with the original `client_id`, `tenant_id`, and `auth_method` to restore the attendee account for subsequent steps. Complete the authentication flow when prompted.

### Step 29b -- Default reappearance when accounts.json loses cfg coverage (informational)

> **Informational only.** Do not run this step in automated test suites; it requires a server restart and leaves the default account in a potentially unauthenticated state. Record as **SKIP** unless specifically testing CR-0064 AC-5.

When `accounts.json` contains the only entry whose `client_id` and `tenant_id` match the env config (`OUTLOOK_MCP_CLIENT_ID`, `OUTLOOK_MCP_TENANT_ID`), removing that entry removes the gating signal. The implicit "default" reappears at the next server start. This is expected behavior: the env-only single-account UX is preserved.

To verify AC-5 manually:
1. Ensure `accounts.json` contains exactly one entry whose identity matches the env config.
2. Run `{tool: "account", args: {operation: "remove", label: "<that entry>"}}`.
3. Restart the server.
4. Call `{tool: "account", args: {operation: "list"}}` and verify "default" is present.

### Step 30 -- Mail operations (skip if mail disabled)

If `config.features.mail_enabled` from Step 0c is `false`, **skip** Steps 30 through 36 and record them as SKIP.

**30a.** Call `{tool: "mail", args: {operation: "help"}}` to discover available mail verbs.

- **Verify:** The response is plain text listing at minimum `help`, `list_folders`, `list_messages`, `get_message`, `search_messages`.
- **Purpose:** Exercises the help verb for the mail domain (AC-2 / FR-15).

Call `{tool: "mail", args: {operation: "list_messages", ...}}` four times with the following filter combinations and record whether each call returns plain text, a sensible total count, and the expected filtering behavior:

| Call | Parameters                                                | Expected                                                         |
|------|-----------------------------------------------------------|------------------------------------------------------------------|
| 30b  | `folder: "Inbox", is_read: false`                         | Only unread messages are listed; count matches folder unread     |
| 30c  | `folder: "Inbox", flag_status: "flagged"`                 | Only flagged messages are listed                                |
| 30d  | `folder: "Inbox", provenance: "created_by_mcp"`           | Only MCP-tagged messages (may be empty if none created yet)     |
| 30e  | `folder: "Inbox"` (no filters, baseline)                  | Baseline message count recorded for comparison                  |

- **Verify:** All calls return plain text. The filtered counts are less than or equal to the baseline.
- **Fail:** If any call returns an error or ignores the filter.

### Step 31 -- Create draft (skip if mail management disabled)

If `config.features.mail_manage_enabled` from Step 0c is `false`, **skip** Steps 31 through 35 and record them as SKIP.

Call `{tool: "mail", args: {operation: "create_draft", to: "<own UPN>", subject: "CRUD test draft", body: "Created by MCP CRUD lifecycle test.", importance: "normal"}}`.

- **Verify:** Response is a plain text confirmation including the draft's message ID.
- **Record:** The draft's message ID as **draft ID**.
- **Fail:** If the tool errors or the message ID is not returned.

### Step 32 -- Update draft

Call `{tool: "mail", args: {operation: "update_draft", id: "<draft ID>", subject: "CRUD test draft (updated)"}}`.

- **Verify:** Response is plain text confirming the update.
- **Verify:** A subsequent `{tool: "mail", args: {operation: "get_message", id: "<draft ID>"}}` call shows the updated subject.
- **Fail:** If the update is not reflected.

### Step 33 -- Create reply draft

Call `{tool: "mail", args: {operation: "create_reply_draft", id: "<draft ID>", comment: "Replying to my own draft."}}`.

If the server rejects replying to a draft, instead pick the most recent message from `{tool: "mail", args: {operation: "list_messages", folder: "Inbox"}}` and reply to it. Record the reply draft ID as **reply draft ID**.

- **Verify:** Response is plain text confirming the reply draft creation with a new message ID.
- **Fail:** If no reply draft is created.

### Step 34 -- Delete reply draft and original draft

Call `{tool: "mail", args: {operation: "delete_draft", id: "<reply draft ID>"}}` (if created).

Then call `{tool: "mail", args: {operation: "delete_draft", id: "<draft ID>"}}`.

- **Verify:** Both calls return plain text delete confirmations.
- **Verify:** A subsequent `{tool: "mail", args: {operation: "get_message", id: "<draft ID>"}}` returns an error (message no longer exists).
- **Fail:** If any draft remains retrievable.

### Step 35 -- Get conversation

Call `{tool: "mail", args: {operation: "list_messages", folder: "Inbox", top: 1}}` and record the first message's `conversationId` as **conversation ID**. If Inbox is empty, skip Step 35.

Call `{tool: "mail", args: {operation: "get_conversation", id: "<conversation ID>"}}`.

- **Verify:** Response is plain text listing one or more messages in chronological order.
- **Fail:** If the call errors for a valid conversation ID.

### Step 36 -- Get attachment

Using `{tool: "mail", args: {operation: "list_messages", folder: "Inbox", has_attachments: true, top: 1}}` pick a message that has attachments. If none found, skip Step 36.

Call `{tool: "mail", args: {operation: "get_message", id: "<message ID>", output: "summary"}}` to enumerate its attachment IDs. Then call:

`{tool: "mail", args: {operation: "get_attachment", message_id: "<message ID>", attachment_id: "<first attachment ID>"}}`.

- **Verify:** Response is plain text with attachment metadata (name, size, content type).
- **Verify:** If the attachment is within the configured size limit, content is returned (base64); otherwise an explanatory message is returned.
- **Fail:** If the attachment cannot be retrieved for a valid ID.

## Reporting

After all steps, print a summary table. Every row **MUST** include a short `Comment` (under ~120 characters) explaining the result — for PASS rows, a brief confirmation of what was verified; for FAIL rows, the failure cause (tool name, error, mismatch); for SKIP rows, the reason (e.g., "single-account mode"). Do not leave the `Comment` column blank.

```
| Step | Action                            | Result         | Comment                                                  |
|------|-----------------------------------|----------------|----------------------------------------------------------|
| 0a   | Discover system verbs (help)      | PASS/FAIL      | e.g., "help verb lists status and other verbs"           |
| 0b   | Verify connectivity (summary)     | PASS/FAIL      | e.g., "default account authenticated; DEBUG logging on"  |
| 0c   | Record config                     | PASS/FAIL      | e.g., "log_file present; timezone=Europe/Stockholm"      |
| 0d   | Verify text default for status    | PASS/FAIL      | e.g., "plain text, no config leak"                       |
| 0a   | system help (docs verbs listed)   | PASS/FAIL      | e.g., "list_docs, search_docs, get_docs present"         |
| 0a2  | list_docs (text)                  | PASS/FAIL      | e.g., "3 slugs: readme, quickstart, troubleshooting"     |
| 0a3  | search_docs (token refresh)       | PASS/FAIL      | e.g., "troubleshooting slug ranked in results"           |
| 0a4  | get_docs section (token-refresh)  | PASS/FAIL      | e.g., "section content returned, no cross-section bleed" |
| 0a5  | get_docs raw (troubleshooting)    | PASS/FAIL      | e.g., "raw markdown starts with # Troubleshooting"       |
| 0b   | status docs section present       | PASS/FAIL      | e.g., "base_uri + troubleshooting_slug + version"        |
| 1    | List accounts (text)              | PASS/FAIL      | e.g., "1 authenticated, 1 disconnected"                  |
| 2    | List calendars (text)             | PASS/FAIL      | e.g., "default + Birthdays"                              |
| 2a   | Discover calendar verbs (help)    | PASS/FAIL      | e.g., "all 14 verbs listed in help output"               |
| 3    | Baseline list (text)              | PASS/FAIL      | e.g., "baseline count = 0"                               |
| 4    | Create event (text confirmation)  | PASS/FAIL      | e.g., "event created at 14:00 Amsterdam"                 |
| 5    | Provenance search (text)          | PASS/FAIL      | e.g., "created_by_mcp filter returned the event"         |
| 6    | Search next_week (text)           | PASS/FAIL      | e.g., "next_week shorthand resolved correctly"           |
| 7    | Search this_week (negative)       | PASS/FAIL      | e.g., "this_week correctly excluded the event"           |
| 8    | Get created event (text)          | PASS/FAIL      | e.g., "all fields match"                                 |
| 9    | Update event (text confirmation)  | PASS/FAIL      | e.g., "subject/location/end/show_as updated"             |
| 10a  | Get updated event (text)          | PASS/FAIL      | e.g., "bodyPreview plain text, start unchanged"          |
| 10b  | Body escalation (raw HTML body)   | PASS/FAIL      | e.g., "raw mode returns full HTML body"                  |
| 11   | Get free/busy (text)              | PASS/FAIL      | e.g., "busy block 14:00-15:00"                           |
| 12   | Reschedule event (text confirm)   | PASS/FAIL      | e.g., "rescheduled to 17:00"                             |
| 13   | Get rescheduled event (text)      | PASS/FAIL      | e.g., "duration preserved"                               |
| 14   | Delete event (text confirmation)  | PASS/FAIL      | e.g., "delete confirmation includes event ID"            |
| 15   | Get deleted (404)                 | PASS/FAIL      | e.g., "ErrorItemNotFound as expected"                    |
| 16   | Provenance search (deleted)       | PASS/FAIL      | e.g., "event absent after deletion"                      |
| 17   | List after delete (text)          | PASS/FAIL      | e.g., "count back to baseline"                           |
| 18   | Create Teams meeting (text)       | PASS/FAIL      | e.g., "online meeting created"                           |
| 19   | Verify Teams meeting details      | PASS/FAIL      | e.g., "onlineMeeting.joinUrl present, isOnlineMeeting=true, self-attendee echoed (single-account)" |
| 20   | Verify invitation (attendee)      | PASS/FAIL/SKIP | e.g., "single-account mode" or "invitation visible"      |
| 21   | Respond from attendee (text)      | PASS/FAIL/SKIP | e.g., "single-account mode" or "tentative response sent" |
| 22   | Verify attendee response          | PASS/FAIL/SKIP | e.g., "single-account mode" or "status=tentative"        |
| 22a  | Update meeting (meeting verb)     | PASS/FAIL/SKIP | e.g., "single-account mode" or "meeting updated"         |
| 22b  | Verify meeting update             | PASS/FAIL/SKIP | e.g., "single-account mode" or "fields updated"          |
| 22c  | Reschedule meeting (meeting verb) | PASS/FAIL/SKIP | e.g., "single-account mode" or "rescheduled 17:30"       |
| 22d  | Verify meeting reschedule         | PASS/FAIL/SKIP | e.g., "single-account mode" or "duration preserved"      |
| 23   | Respond to own meeting (err)      | PASS/FAIL      | e.g., "organizer self-response rejected"                 |
| 24   | Cancel Teams meeting (text)       | PASS/FAIL      | e.g., "cancellation confirmation with event ID"          |
| 25   | Verify cancellation               | PASS/FAIL      | e.g., "ErrorItemNotFound as expected"                    |
| 26   | Verify server logs (FQN audit)    | PASS/FAIL      | e.g., "calendar.delete_event in audit log"               |
| 27   | Force refresh token (text)        | PASS/FAIL      | e.g., "label + expiry in plain text"                     |
| 28   | Log out non-default account       | PASS/FAIL/SKIP | e.g., "only default authenticated" or "logged out"       |
| 29   | Log back in non-default account   | PASS/FAIL/SKIP | e.g., "only default authenticated" or "re-authenticated" |
| 29a  | Durable account removal           | PASS/FAIL/SKIP | e.g., "single-account mode" or "label absent after restart" |
| 30a  | Discover mail verbs (help)        | PASS/FAIL/SKIP | e.g., "mail disabled" or "all verbs listed"              |
| 30b  | Mail list is_read filter          | PASS/FAIL/SKIP | e.g., "unread filter honored"                            |
| 30c  | Mail list flag_status filter      | PASS/FAIL/SKIP | e.g., "flagged filter honored"                           |
| 30d  | Mail list provenance filter       | PASS/FAIL/SKIP | e.g., "provenance filter returned 0 MCP messages"        |
| 30e  | Mail list baseline                | PASS/FAIL/SKIP | e.g., "baseline count recorded"                          |
| 31   | Create mail draft                 | PASS/FAIL/SKIP | e.g., "draft id returned"                                |
| 32   | Update mail draft                 | PASS/FAIL/SKIP | e.g., "subject updated"                                  |
| 33   | Create reply draft                | PASS/FAIL/SKIP | e.g., "reply draft created"                              |
| 34   | Delete drafts                     | PASS/FAIL/SKIP | e.g., "both drafts deleted, 404 on re-fetch"             |
| 35   | Get conversation                  | PASS/FAIL/SKIP | e.g., "thread returned in chronological order"           |
| 36   | Get attachment                    | PASS/FAIL/SKIP | e.g., "metadata + base64 under size limit"               |
```

Then print the **environment** section using all values recorded in Steps 0c and 1:

```
| Property            | Value                                  |
|---------------------|----------------------------------------|
| Server version      | <version from Step 0c>                 |
| Default timezone    | <timezone from Step 0c>                |
| Uptime (seconds)    | <server_uptime_seconds from Step 0c>   |
| Auth method         | <auth_method> (<auth_method_source>)   |
| Client ID           | <client_id from Step 0c>               |
| Tenant ID           | <tenant_id from Step 0c>               |
| Token cache backend | keychain / file                        |
| Read-only mode      | true / false                           |
| Provenance tag      | <provenance_tag from Step 0c>          |
| Log level           | debug                                  |
| Log format          | <log_format from Step 0c>              |
| Log file            | <path from Step 0c>                    |
| PII sanitization    | true / false                           |
| Audit logging       | true / false                           |
| Max retries         | <max_retries from Step 0c>             |
| Request timeout (s) | <request_timeout_seconds from Step 0c> |
| Multi-account mode  | true / false                           |
| Attendee account    | <label from Step 1> / N/A             |
| Attendee email      | <email from Step 2> / N/A             |
```

If any step FAILs, stop execution and report the failure details including the full tool response.
