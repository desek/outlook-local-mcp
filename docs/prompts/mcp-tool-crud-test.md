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

## Non-interactive mode (CR-0064 Phase 3)

When this test runs under `claude -p` or any other non-interactive caller, the following rules apply:

- **MUST NOT call `account.login` for any reason.** The device-code and browser flows require a human at the keyboard and will hang a non-interactive runner indefinitely.
- **Cached tokens are a hard precondition.** Before starting, ensure all accounts whose steps you intend to execute have a valid file-cache token (run the server interactively at least once to warm the cache).
- **If an account shows `disconnected` in Step 1:** attempt one benign read with that account (e.g., `{tool: "mail", args: {operation: "list_folders", account: "<label>"}}`). If the read succeeds the account was silently reconnected via the Phase 3 path. If the read returns an auth error, mark all steps that depend on that account **SKIP** and continue with any remaining authenticated accounts.
- **Steps 28 and 29 (logout/login round-trip) are unconditionally SKIP in non-interactive mode.** See those steps for the explicit note.

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

**0a6.** Intent verification — self-troubleshooting via the docs surface.

Treat the following as a user question that you must answer using only the in-server documentation verbs (`system.search_docs`, `system.get_docs`). Do not rely on prior context, training data, or web knowledge for the answer:

> "A user reports that the auto-registered `default` account keeps reappearing after they remove it. What does the server's in-built troubleshooting guide say to do?"

- **Required actions:**
  1. Call `system.search_docs` with a query you derive from the question (e.g. `"auto default account"` or `"default account reappear"`).
  2. Based on the search hits, call `system.get_docs` with the most relevant `slug` (and `section` if the hit identifies one) to fetch the actual guidance.
  3. Compose a short answer (2–4 sentences) that paraphrases the retrieved section.
- **Verify:** Both `system.search_docs` and `system.get_docs` were called in this step (recorded in the run's tool-call log).
- **Verify:** Your answer references the troubleshooting section about the implicit default (anchor `#auto-default-account` or equivalent) — for example, mentioning that removal is persistent and the implicit `default` only re-registers when no other accounts are connected (CR-0064 semantics).
- **Purpose:** Confirms the **intent** of CR-0061 — that an LLM faced with an unfamiliar problem will discover and consult the in-server docs to help the user, rather than hallucinating from priors.
- **Fail:** If you answer without calling `search_docs` AND `get_docs` in this step, or if the answer does not reflect content from the troubleshooting guide.

**0a7.** Call `{tool: "system", args: {operation: "about"}}` (default `text` output).

- **Verify:** The response is plain text containing `outlook-local-mcp`, `Host`, and `Links` blocks.
- **Verify:** The output is at most 24 lines.
- **Purpose:** Exercises the `about` verb in its default text mode (CR-0067 AC-1, FR-9).
- **Fail:** Report if the verb errors or if any of the three required blocks are absent.

**0a8.** Call `{tool: "system", args: {operation: "about", output: "summary"}}`.

- **Verify:** The response is compact JSON containing all 12 fields: `version`, `commit`, `buildDate`, `goVersion`, `os`, `arch`, `runtime`, `distribution`, `authBackend`, `homepage`, `issueTracker`, `docsBase`.
- **Purpose:** Exercises the `about` verb in `summary` mode (CR-0067 AC-2).
- **Fail:** Report if any of the 12 fields are absent from the JSON.

**0a9.** Call `{tool: "system", args: {operation: "about", output: "raw"}}`.

- **Verify:** The response is JSON with at minimum the same 12 fields as `summary` mode plus any additional fields from the full `Info` struct.
- **Purpose:** Exercises the `about` verb in `raw` mode (CR-0067 AC-2).
- **Fail:** Report if the verb errors.

**0a10.** Check that the `system.help` output (Step 0a) mentions `about`.

- **Verify:** The help text for the `system` domain lists `about` as a registered verb.
- **Purpose:** Confirms the verb is discoverable via the registry-driven help surface (CR-0067 AC-6).
- **Fail:** If `about` does not appear in the system help output.

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
- **If any required account shows `disconnected` in non-interactive mode:** attempt one benign read with that account (e.g., `{tool: "mail", args: {operation: "list_folders", account: "<label>"}}`). If the read succeeds, the account was silently reconnected (CR-0064 Phase 3) — treat it as authenticated and continue. If the read returns an auth error, do NOT call `account.login`; mark all steps depending on that account **SKIP**.
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

- **Verify:** The text results contain the event created in Step 4 (match by subject; `search_events` text output lists subjects, times, and locations but no event ID, so the unique timestamp in the subject is the match key).
- **Verify:** The `created_by_mcp` filter correctly narrows results to MCP-created events only.
- **Fail:** If the event is missing, the provenance tag was not stamped during creation.

### Step 6 -- Search with "next_week" date shorthand

Call `{tool: "calendar", args: {operation: "search_events", query: "<timestamp portion>", date: "next_week"}}`.

- **Verify:** The text results contain the event created in Step 4 (match by subject; `search_events` text output lists subjects, times, and locations but no event ID, so the unique timestamp in the subject is the match key).
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

- **Verify:** Every tool call has a `DEBUG`-level entry at the start of the operation and an `INFO`-level (or `ERROR` for Steps 15, 23, 25) "tool completed" entry. The DEBUG entry may be the generic `"tool called"` (read verbs and `create_event`, `create_meeting`, `update_event`) or a domain-specific message (`"rescheduling event"`, `"deleting event"`, `"responding to event"`, `"cancelling event"`); both forms satisfy this check.
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

> **Non-interactive mode:** Mark Steps 28 and 29 unconditionally **SKIP**. The login step requires interactive user input (browser or device code) and will hang a non-interactive runner. Do not attempt even if cached tokens appear valid.

> **Note:** This test requires at least one non-default account in addition to the default account, or `account login` in Step 29 must be used to restore access before further tests. If only one account is registered, mark Steps 28 and 29 **SKIP** to avoid leaving the test environment unauthenticated.

Pick a **non-default authenticated account** from Step 1's list (the **attendee account label** in multi-account mode). Call `{tool: "account", args: {operation: "logout", label: "<non-default account label>"}}`.

- **Pass:** Response is plain text confirming the logout.
- **Verify:** A subsequent `{tool: "account", args: {operation: "list"}}` call shows the account as `disconnected` while still listing the entry (not removed).
- **Verify:** Calling any calendar tool with `account: <logged-out label>` returns an actionable error mentioning `disconnected` and `login`.
- **Fail:** If the account is removed, still shown as authenticated, or if the disconnected-account error is missing.

### Step 29 -- Log back in to account

> **Non-interactive mode:** Mark this step unconditionally **SKIP** (see Step 28 note above).

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
| 30b  | `folder_id: "Inbox", is_read: false`                      | Only unread messages are listed; count matches folder unread     |
| 30c  | `folder_id: "Inbox", flag_status: "flagged"`              | Only flagged messages are listed                                |
| 30d  | `folder_id: "Inbox", provenance: true`                    | Only MCP-tagged messages (may be empty if none created yet)     |
| 30e  | `folder_id: "Inbox"` (no filters, baseline)               | Baseline message count recorded for comparison                  |

- **Verify:** All calls return plain text. The filtered counts are less than or equal to the baseline.
- **Fail:** If any call returns an error or ignores the filter.
- **Expected scale artifact (not a finding):** On a large mailbox a list is capped at `max_results` (default 25, max 100), so a folder unread total far larger than the returned list is expected. A prior run recorded 27,214 unread against a default cap of 25; this is the cap working as designed, not a defect. Read the folder's own unread total from the count line, not from the number of rows returned, and do not raise it as a finding.

**30f -- Property-restricted search.** Call `{tool: "mail", args: {operation: "search_messages", query: "from:<own UPN>", max_results: 5}}`, substituting your own UPN for `<own UPN>`.

- **Verify:** The call returns plain text (ranked results, not chronological) and does not raise a parse error naming a character position. Any returned message is from the named sender.
- **Fail:** If the call returns a Graph parse error such as `character ':' is not valid at position N`. The server must supply the enclosing quotes a `$search` value requires, so a `property:value` query must not reach Graph unquoted.

**30g -- Parenthesised phrase search, with a silent-discard control.** Call `{tool: "mail", args: {operation: "search_messages", query: "subject:(CRUD test)", max_results: 5}}`, then call `{tool: "mail", args: {operation: "search_messages", query: "Zzzqqxx Wwwyyzz", max_results: 5}}`.

- **Verify:** The parenthesised call returns plain text and does not raise a parse error. The parenthesised form matches all of its tokens in any order, not as an adjacent phrase, so a subject holding both words in either order is a match.
- **Verify (silent-discard control):** The two-nonsense-word call returns **zero** results, not the newest messages in the mailbox. A multi-word query that matches nothing must filter to an empty result, not fall through to recent unrelated mail.
- **Fail:** If the parenthesised call errors, or if the nonsense multi-word query returns recent messages instead of zero results.

### Step 31 -- Create draft (skip if mail management disabled)

If `config.features.mail_manage_enabled` from Step 0c is `false`, **skip** Steps 31 through 35 and Steps 37 through 41 (received-message management) and record them all as SKIP.

Call `{tool: "mail", args: {operation: "create_draft", to_recipients: "<own UPN>", subject: "CRUD test draft", body: "Created by MCP CRUD lifecycle test.", importance: "normal"}}`.

- **Verify:** Response is a plain text confirmation including the draft's message ID.
- **Record:** The draft's message ID as **draft ID**.
- **Fail:** If the tool errors or the message ID is not returned.

### Step 32 -- Update draft

Call `{tool: "mail", args: {operation: "update_draft", message_id: "<draft ID>", subject: "CRUD test draft (updated)"}}`.

- **Verify:** Response is plain text confirming the update.
- **Verify:** A subsequent `{tool: "mail", args: {operation: "get_message", message_id: "<draft ID>"}}` call shows the updated subject.
- **Fail:** If the update is not reflected.

### Step 33 -- Create reply draft

Call `{tool: "mail", args: {operation: "create_reply_draft", message_id: "<draft ID>", comment: "Replying to my own draft."}}`. **Note:** `create_reply_draft` cannot reply to a draft message (Microsoft Graph constraint); the call is expected to return an error indicating the source must be a received or sent message. Fall back to using a recent Inbox message ID for this step.

If the server rejects replying to a draft, instead pick the most recent message from `{tool: "mail", args: {operation: "list_messages", folder_id: "Inbox"}}` and reply to it. Record the reply draft ID as **reply draft ID**.

- **Verify:** Response is plain text confirming the reply draft creation with a new message ID.
- **Fail:** If no reply draft is created.

### Step 34 -- Delete reply draft and original draft

Call `{tool: "mail", args: {operation: "delete_draft", message_id: "<reply draft ID>"}}` (if created).

Then call `{tool: "mail", args: {operation: "delete_draft", message_id: "<draft ID>"}}`.

- **Verify:** Both calls return plain text delete confirmations.
- **Verify:** A subsequent `{tool: "mail", args: {operation: "get_message", message_id: "<draft ID>"}}` returns an error (message no longer exists).
- **Fail:** If any draft remains retrievable.

### Step 35 -- Get conversation

Call `{tool: "mail", args: {operation: "list_messages", folder_id: "Inbox", max_results: 1}}` and record the first message's `conversationId` as **conversation ID**. If Inbox is empty, skip Step 35.

Call `{tool: "mail", args: {operation: "get_conversation", conversation_id: "<conversation ID>"}}`.

- **Verify:** Response is plain text listing one or more messages in chronological order.
- **Fail:** If the call errors for a valid conversation ID.

### Step 36 -- Get attachment

Using `{tool: "mail", args: {operation: "list_messages", folder_id: "Inbox", has_attachments: true, max_results: 1}}` pick a message that has attachments. If none found, skip Step 36.

Call `{tool: "mail", args: {operation: "list_attachments", message_id: "<message ID>"}}` to enumerate its attachment IDs. Note that `get_message` does not return them at any output tier: its summary carries `hasAttachments` only, so `list_attachments` is the verb that yields an attachment ID. Then call:

`{tool: "mail", args: {operation: "get_attachment", message_id: "<message ID>", attachment_id: "<first attachment ID>"}}`.

- **Verify:** Response is plain text with attachment metadata (name, size, content type).
- **Verify:** If the attachment is within the configured size limit, content is returned (base64); otherwise an explanatory message is returned.
- **Fail:** If the attachment cannot be retrieved for a valid ID.

### Step 37 -- Mark read and restore (skip if mail management disabled)

Steps 37 through 39 write properties on a real received message and **must restore the original value**. Call `{tool: "mail", args: {operation: "list_messages", folder_id: "Inbox", max_results: 1}}` and record the first message's ID as **triage message ID**. Then call `{tool: "mail", args: {operation: "get_message", message_id: "<triage message ID>", output: "raw"}}` and record its current `isRead`, `flag`, and `categories` values as the **restore values**. If the Inbox is empty, skip Steps 37 through 39.

Call `{tool: "mail", args: {operation: "mark_read", message_id: "<triage message ID>", is_read: <the inverse of the recorded isRead>}}`, then call it a second time with the identical arguments.

- **Verify:** Both calls return a plain text confirmation naming the subject, the message ID, and the resulting read state.
- **Verify:** The two confirmations are identical, and a subsequent `get_message` shows the written state. The verb is declared idempotent, so a repeat must not change the outcome.
- **Restore:** Call `mark_read` once more with the recorded `isRead` value.
- **Fail:** If either call errors, if the state is not reflected, or if the repeat produces a different confirmation.

### Step 38 -- Set follow-up flag and restore

Call `{tool: "mail", args: {operation: "set_flag", message_id: "<triage message ID>", flag_status: "flagged"}}`, then call `{tool: "mail", args: {operation: "set_flag", message_id: "<triage message ID>", flag_status: "urgent"}}`.

- **Verify:** The first call returns a plain text confirmation stating the resulting status is `flagged`, and `list_messages` with `flag_status: "flagged"` now includes the message.
- **Verify:** The second call is **refused** with an error naming the three accepted values `notFlagged`, `flagged`, and `complete`. An unrecognised status must not silently clear the flag.
- **Restore:** Call `set_flag` with the recorded original status (`notFlagged` if the message was unflagged).
- **Fail:** If the invalid status is accepted, or if the confirmation does not state the resulting status.

### Step 39 -- Set categories, clear, and restore

Call `{tool: "mail", args: {operation: "set_categories", message_id: "<triage message ID>", categories: "MCP CRUD test"}}`, then call `{tool: "mail", args: {operation: "set_categories", message_id: "<triage message ID>", categories: "   "}}`.

- **Verify:** The first confirmation lists the resulting category set, read back from the service rather than echoing the request.
- **Verify:** The second call clears every category and its confirmation states that the message now carries no categories, rather than printing an empty list.
- **Note:** Graph applies a category on a message whether or not it exists in the mailbox's master category list, so a category set here may render without a colour in Outlook. That is expected; these verbs do not create master categories.
- **Restore:** Call `set_categories` with the recorded original categories as a comma-separated string, or with an empty string if there were none.
- **Fail:** If the second call leaves categories in place, or if either confirmation reports the request rather than the response.

### Step 40 -- Move a message and follow the new identifier

Create a disposable subject rather than moving the user's mail: call `{tool: "mail", args: {operation: "create_draft", to_recipients: "<own UPN>", subject: "CRUD test move", body: "Created by MCP CRUD lifecycle test."}}` and record the ID as **move source ID**.

Call `{tool: "mail", args: {operation: "list_folders"}}` and record the ID of the `Deleted Items` folder as **destination folder ID**. Then call `{tool: "mail", args: {operation: "move_message", message_id: "<move source ID>", destination_folder_id: "<destination folder ID>"}}`.

- **Verify:** The confirmation names the destination folder, the original identifier, and a **new** message identifier, and states that the original identifier no longer resolves. Record the new ID as **moved message ID**.
- **Verify:** `{tool: "mail", args: {operation: "get_message", message_id: "<move source ID>"}}` now errors, and `get_message` with the **moved message ID** succeeds.
- **Verify (unresolvable destination):** Call `move_message` again with `destination_folder_id: "Archive"` (a folder *name*, not an identifier). The call must fail with an error naming `list_folders` as the way to obtain a destination identifier.
- **Cleanup:** Call `{tool: "mail", args: {operation: "delete_draft", message_id: "<moved message ID>"}}`.
- **Fail:** If the confirmation omits the new identifier, if the original identifier still resolves, or if the folder-name destination is accepted.

### Step 41 -- Attach a file to a draft

Create the target rather than using an existing message: call `{tool: "mail", args: {operation: "create_draft", to_recipients: "<own UPN>", subject: "CRUD test attachment", body: "Created by MCP CRUD lifecycle test."}}` and record the ID as **attachment draft ID**.

Call `{tool: "mail", args: {operation: "add_attachment", message_id: "<attachment draft ID>", name: "crud-test.txt", mime_type: "text/plain", content_bytes: "Q1JVRCB0ZXN0IGF0dGFjaG1lbnQu"}}` (the base64 of a short ASCII sentence).

- **Verify:** Response is a plain text confirmation naming the attachment name, the draft subject, the message ID, a new attachment ID, the size in bytes, and which transfer path was used.
- **Verify:** `{tool: "mail", args: {operation: "list_attachments", message_id: "<attachment draft ID>"}}` lists `crud-test.txt` with the reported attachment ID.
- **Verify (non-draft refused):** Call `add_attachment` again with the `message_id` of any received message from Step 30. The call must fail with an error stating the message is not a draft, and must not attach anything.
- **Cleanup:** Call `{tool: "mail", args: {operation: "delete_draft", message_id: "<attachment draft ID>"}}`.
- **Fail:** If the confirmation omits the attachment ID or the size, if the attachment is absent from `list_attachments`, or if the non-draft target is accepted.

### Step 42 -- Propose meeting slots for a set of attendees

Call `{tool: "calendar", args: {operation: "find_meeting_times", attendees: "[{\"email\":\"<self UPN>\",\"type\":\"required\"}]", meeting_duration: "PT30M", start_datetime: "<test date>T09:00:00", end_datetime: "<test date>T18:00:00", timezone: "Europe/Amsterdam", max_candidates: 5}}`.

- **Verify:** The response is plain text, is a numbered list of at most five candidate slots, and ends with a total count.
- **Verify:** Each candidate names a start and an end inside the requested window and carries a confidence value.
- **Verify:** If no slot is offered, the response states the reason Graph gave rather than returning an empty list with no explanation.
- **Verify (both-or-neither window):** Call again with `start_datetime` supplied and `end_datetime` omitted. The call must fail with an error stating that the two bounds are supplied together or not at all, and must not reach Graph.
- **Verify (bounds):** Call again with `max_candidates: 0`. The call must fail with an error naming the accepted range.
- **Fail:** If the default response is not plain text, if a candidate lacks its times or confidence, if the one-sided window is accepted, or if the out-of-range candidate count is accepted.

### Step 43 -- Read free/busy blocks and working hours per mailbox

Call `{tool: "calendar", args: {operation: "get_schedule", schedules: "<self UPN>", date: "<test date>", timezone: "Europe/Amsterdam"}}`.

- **Verify:** The response is plain text with one labeled section for the queried mailbox and a total mailbox count at the end.
- **Verify:** The section states either the mailbox's busy periods with their times and status, or that it has no busy periods. Both are valid results here: every event this run created on the test date was deleted or cancelled by Step 25, so an empty diary is the expected state and must read as a stated finding rather than a missing section.
- **Verify:** Where Graph supplies them, the section states the mailbox's working hours, which `get_free_busy` does not report. Record their absence as an observation rather than a failure; Graph omits them for some mailbox types.
- **Verify (per-mailbox error):** Call again with `schedules: "<self UPN>,definitely-not-a-mailbox@<own domain>"`. The call must succeed, the real mailbox must still return its section, and the unknown address must carry an `Error:` line naming what Graph reported rather than being omitted from the output.
- **Verify (summary tier):** Call again with `output: "summary"`. The response is structured, attributes every block to its mailbox, and preserves the order the mailboxes were named in.
- **Fail:** If a per-mailbox failure fails the whole call or silently drops that mailbox, if blocks are not attributable to a mailbox, or if the mailbox order is not the requested order.

### Step 44 -- Attach a file to an event

Create the target rather than reusing an earlier one: every event created before this point was deleted by Step 14 or cancelled by Step 24. Call `{tool: "calendar", args: {operation: "create_event", subject: "MCP CRUD attachment -- <timestamp>", start_datetime: "<test date>T11:00:00", end_datetime: "<test date>T11:30:00", start_timezone: "Europe/Amsterdam", end_timezone: "Europe/Amsterdam", body: "Automated CRUD lifecycle test.", show_as: "free"}}` and record the ID as **attachment event ID**.

Call `{tool: "calendar", args: {operation: "add_event_attachment", event_id: "<attachment event ID>", name: "crud-test.txt", mime_type: "text/plain", content_bytes: "Q1JVRCB0ZXN0IGF0dGFjaG1lbnQu"}}` (the base64 of a short ASCII sentence).

- **Verify:** Response is a plain text confirmation naming the attachment name, the event subject, the event ID, a new attachment ID, the size in bytes, and which transfer path was used. Record the attachment ID as **event attachment ID**.
- **Verify:** The verb takes no `output` parameter; it is a write and confirms in text unconditionally.
- **Verify (invalid content):** Call `add_event_attachment` again with `content_bytes: "not base64!!"`. The call must fail with an error stating the content is not standard base64 and saying to re-encode it, and must attach nothing.
- **Verify (unknown event):** Call `add_event_attachment` with `event_id: "AAAAAAAAAAAAAAAAAAAAAA=="` and otherwise valid arguments. The call must fail naming the event rather than reporting a successful attach.
- **Fail:** If the confirmation omits the attachment ID or the size, if the malformed content is accepted, or if the unknown event is accepted.

### Step 45 -- List the attachments of an event

Call `{tool: "calendar", args: {operation: "list_event_attachments", event_id: "<attachment event ID>"}}`.

- **Verify:** The default response is plain text, a numbered list naming `crud-test.txt` with the **event attachment ID**, its content type, and its size, and ends with a total count.
- **Verify:** No content bytes appear in the response at the default tier; this verb returns metadata only.
- **Verify (summary tier):** Call again with `output: "summary"`. The response is structured and still carries the attachment ID, name, content type, and size.
- **Fail:** If the attachment added in Step 44 is absent, if the listed ID does not match the one confirmed there, or if content bytes are returned.

### Step 46 -- Download an event attachment

Call `{tool: "calendar", args: {operation: "get_event_attachment", event_id: "<attachment event ID>", attachment_id: "<event attachment ID>"}}`.

- **Verify:** Response is plain text with the attachment metadata (name, content type, size) and the content as base64. Decoding the content yields the sentence sent in Step 44, so the round trip is graded on the bytes rather than on the metadata alone.
- **Verify (unknown attachment):** Call again with `attachment_id: "AAAAAAAAAAAAAAAAAAAAAA=="`. The call must fail naming the attachment rather than returning empty content.
- **Cleanup:** Call `{tool: "calendar", args: {operation: "delete_event", event_id: "<attachment event ID>"}}`. Deleting the event removes its attachments with it; there is no separate attachment removal verb.
- **Fail:** If the returned content does not decode to the bytes sent in Step 44, if the unknown attachment ID is accepted, or if the cleanup delete fails.

### Step 47 -- Contacts domain help (skip if contacts disabled)

Call `{tool: "contacts", args: {operation: "help"}}`.

- **Skip:** If no `contacts` tool is registered, mark Steps 47-51 SKIP with the reason "contacts disabled" and continue. The domain is opt-in and is absent unless `OUTLOOK_MCP_CONTACTS_ENABLED` is set.
- **Verify:** The response names all five verbs: `help`, `search`, `get_contact`, `list_people`, `get_person`.
- **Verify:** Every verb is documented as read-only and non-destructive; no write, create, update, delete, folder, photo, directory, or sync verb is listed.
- **Fail:** If any of the five verbs is missing, or if any verb that writes a contact is offered.

### Step 48 -- Search contacts and people (skip if contacts disabled)

Call `{tool: "contacts", args: {operation: "search", query: "<first name of the attendee from Step 2, or any common name>"}}`.

- **Verify:** The default response is plain text and every match is labelled with the source it came from, so a saved personal contact is distinguishable from a relevance-ranked person.
- **Verify (summary tier):** Call again with `output: "summary"`. The response is structured and still carries the display name and the email address of each match.
- **Verify (empty query rejected):** Call again with `query: "   "`. The call must fail naming the `query` parameter and stating what to supply, and no result set is returned.
- **Record:** A `contact_id` and a `person_id` from the results, if any, for Steps 49 and 51.
- **Fail:** If a match carries no source label, if the whitespace-only query is accepted, or if the summary tier omits the address.

### Step 49 -- Get one saved contact (skip if contacts disabled)

Call `{tool: "contacts", args: {operation: "get_contact", contact_id: "<contact_id from Step 48>"}}`. If Step 48 returned no personal contact, mark this step SKIP with the reason "no personal contact in this mailbox".

- **Verify:** Response is plain text naming the contact's display name and every one of its email addresses.
- **Verify (invalid identifier):** Call again with `contact_id: ""`. The call must fail naming the `contact_id` parameter, before any Microsoft Graph request is issued.
- **Fail:** If the addresses are missing, or if the empty identifier is accepted.

### Step 50 -- List relevance-ranked people (skip if contacts disabled)

Call `{tool: "contacts", args: {operation: "list_people"}}`.

- **Verify:** The default response is plain text, a numbered list of people ending with a total count.
- **Verify:** The order is the relevance order Microsoft Graph returned, most relevant first; it is not alphabetical unless Graph returned it that way.
- **Verify (raw tier):** Call again with `output: "raw"`. The response carries the full payload for each person.
- **Record:** A `person_id` from the results for Step 51 if Step 48 supplied none.
- **Fail:** If the response is re-sorted, or if the total count is absent.

### Step 51 -- Get one relevance-ranked person (skip if contacts disabled)

Call `{tool: "contacts", args: {operation: "get_person", person_id: "<person_id from Step 48 or Step 50>"}}`.

- **Verify:** Response is plain text naming the person's display name and every one of its scored email addresses, each address labelled with that same display name.
- **Verify (invalid identifier):** Call again with `person_id: ""`. The call must fail naming the `person_id` parameter, before any Microsoft Graph request is issued.
- **Fail:** If any scored address is omitted, or if the empty identifier is accepted.

### Step 52 -- Teams domain help (skip if teams disabled)

Call `{tool: "teams", args: {operation: "help"}}`.

- **Skip:** If `config.features.teams_enabled` from the Step 0b status output is false, or if no `teams` tool is registered, mark Steps 52-58 SKIP with the reason "teams disabled" and continue. The domain is opt-in and is absent unless `OUTLOOK_MCP_TEAMS_ENABLED` is set.
- **Verify:** The response names all thirteen verbs: `help`, `search`, `list_chats`, `list_chat_messages`, `get_chat_message`, `list_chat_message_replies`, `list_channel_messages`, `get_channel_message`, `list_channel_message_replies`, `compose_reply`, `get_online_meeting`, `list_transcripts`, `get_transcript`.
- **Verify:** Every verb is documented as read-only and non-destructive; no send, post, create, update, delete, presence, recording, or attendance verb is listed, and no verb enumerates joined teams or channels.
- **Fail:** If any of the thirteen verbs is missing, or if any verb that writes to Teams is offered.

### Step 53 -- Search Teams messages (skip if teams disabled)

Call `{tool: "teams", args: {operation: "search", query: "the"}}`.

- **Verify:** The default response is plain text and each hit is labelled with the chat, or the team and channel, it came from, so the identifiers the other verbs require are obtainable here.
- **Verify (summary tier):** Call again with `output: "summary"`. The response is structured and still carries the identifiers of each hit.
- **Verify (empty query rejected):** Call again with `query: "   "`. The call must fail naming the `query` parameter and stating what to supply, and no result set is returned.
- **Record:** From the hits, a `chat_id` and message id for a chat message, and a team id, channel id, and message id for a channel message, for Steps 54 to 56.
- **Fail:** If a hit carries no chat or channel label, or if the whitespace-only query is accepted.

### Step 54 -- Read a chat thread (skip if teams disabled)

Call `{tool: "teams", args: {operation: "list_chats"}}`, then `{tool: "teams", args: {operation: "list_chat_messages", chat_id: "<chat_id from Step 53 or from list_chats>"}}`, then `{tool: "teams", args: {operation: "get_chat_message", chat_id: "<same chat_id>", message_id: "<message id from the listing>"}}`, then `{tool: "teams", args: {operation: "list_chat_message_replies", chat_id: "<same chat_id>", message_id: "<same message id>"}}`.

- **Skip:** If the account is a member of no chat, mark this step SKIP with the reason "no Teams chat in this account".
- **Verify:** `list_chats` returns a numbered list ending with a total count, each entry carrying enough to tell one chat from another.
- **Verify:** `list_chat_messages` returns a body preview per message rather than every full body.
- **Verify (body escalation):** `get_chat_message` returns a preview by default and states that the full body requires `output=raw`; calling it again with `output: "raw"` returns the full body.
- **Verify (invalid identifier):** Call `get_chat_message` again with `chat_id: ""`. The call must fail naming the `chat_id` parameter, before any Microsoft Graph request is issued.
- **Fail:** If the default tier returns full bodies, if the raw tier does not, or if the empty identifier is accepted.

### Step 55 -- Read a channel thread (skip if teams disabled)

Call `{tool: "teams", args: {operation: "list_channel_messages", team_id: "<team id from Step 53>", channel_id: "<channel id from Step 53>"}}`, then `{tool: "teams", args: {operation: "get_channel_message", team_id: "<same team id>", channel_id: "<same channel id>", message_id: "<message id from the listing>"}}`, then `{tool: "teams", args: {operation: "list_channel_message_replies", team_id: "<same team id>", channel_id: "<same channel id>", message_id: "<same message id>"}}`.

- **Skip:** If Step 53 returned no channel hit, mark this step SKIP with the reason "no Teams channel message reachable from search".
- **Verify:** `list_channel_messages` returns the thread openers, and the replies under one of them come back only from `list_channel_message_replies`.
- **Verify (body escalation):** `get_channel_message` returns a preview by default and the full body under `output: "raw"`.
- **Verify (missing identifier):** Call `list_channel_messages` again with `team_id: ""`. The call must fail naming the `team_id` parameter and pointing at the `search` operation as the way to obtain it, before any Microsoft Graph request is issued.
- **Fail:** If a channel read succeeds without both identifiers, or if the refusal does not say where the identifier comes from.

### Step 56 -- Prepare a reply without sending it (skip if teams disabled)

Call `{tool: "teams", args: {operation: "compose_reply", chat_id: "<chat_id from Step 54>", message_id: "<message id from Step 54>", body: "Acknowledged, thank you."}}`.

- **Verify:** The response is the prepared reply text, quoting the message being answered, and it states plainly that nothing was sent or posted.
- **Verify:** Nothing new appears in the chat. Call `list_chat_messages` for the same `chat_id` again; the message list is unchanged.
- **Verify (no output parameter):** The `help` output for `compose_reply` declares no `output` parameter; it returns its prepared text unconditionally.
- **Verify (mixed identifiers refused):** Call again supplying `chat_id`, `team_id`, and `channel_id` together. The call must fail naming which shape to supply.
- **Fail:** If a message is posted to Teams, if the response does not say the reply is unsent, or if the mixed identifier shape is accepted.

### Step 57 -- Resolve an online meeting (skip if teams disabled)

Call `{tool: "calendar", args: {operation: "list_events", date: "week"}}` and find an event carrying a Teams join URL, then call `{tool: "teams", args: {operation: "get_online_meeting", join_web_url: "<the join URL>"}}`.

- **Skip:** If no event in the range carries a join URL, mark Steps 57 and 58 SKIP with the reason "no Teams meeting in range".
- **Verify:** The response carries the meeting-scoped identifier the transcript verbs are keyed by.
- **Verify (both identifiers refused):** Call again supplying both `meeting_id` and `join_web_url`. The call must fail naming both parameters, before any Microsoft Graph request is issued.
- **Verify (neither identifier refused):** Call again supplying neither. The call must fail naming both parameters and pointing at the calendar `get_event` operation as the source of the join URL.
- **Record:** The meeting identifier for Step 58.
- **Fail:** If either malformed call is accepted, or if no meeting identifier is returned.

### Step 58 -- Read a meeting transcript (skip if teams disabled)

Call `{tool: "teams", args: {operation: "list_transcripts", meeting_id: "<meeting id from Step 57>"}}`, then `{tool: "teams", args: {operation: "get_transcript", meeting_id: "<same meeting id>", transcript_id: "<transcript id from the listing>"}}`.

- **Skip:** If the meeting holds no transcript, mark this step SKIP with the reason "meeting holds no transcript".
- **Verify:** `list_transcripts` returns metadata only; no transcript text is delivered by it.
- **Verify (content escalation):** `get_transcript` returns metadata and a short preview by default and states that the full text requires `output=raw`; calling it again with `output: "raw"` returns the full WEBVTT text.
- **Verify (invalid identifier):** Call `get_transcript` again with `transcript_id: ""`. The call must fail naming the `transcript_id` parameter, before any Microsoft Graph request is issued.
- **Fail:** If the listing delivers transcript content, if the default tier returns the full text, or if the empty identifier is accepted.

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
| 0a6  | docs intent (self-troubleshoot)   | PASS/FAIL      | e.g., "search_docs + get_docs called; answer cites #auto-default-account" |
| 0a7  | about text (all blocks present)   | PASS/FAIL      | e.g., "outlook-local-mcp, Host, Links blocks ≤24 lines"  |
| 0a8  | about summary (12 JSON fields)    | PASS/FAIL      | e.g., "all 12 fields present in compact JSON"            |
| 0a9  | about raw (full Info JSON)        | PASS/FAIL      | e.g., "raw JSON includes version, authBackend, docsBase"  |
| 0a10 | about listed in system help       | PASS/FAIL      | e.g., "about verb present in system help output"         |
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
| 30f  | Mail search (property-restricted) | PASS/FAIL/SKIP | e.g., "from: query scoped, no parse error"               |
| 30g  | Mail search (phrase + control)    | PASS/FAIL/SKIP | e.g., "parenthesised matched; nonsense query returned 0" |
| 31   | Create mail draft                 | PASS/FAIL/SKIP | e.g., "draft id returned"                                |
| 32   | Update mail draft                 | PASS/FAIL/SKIP | e.g., "subject updated"                                  |
| 33   | Create reply draft                | PASS/FAIL/SKIP | e.g., "reply draft created"                              |
| 34   | Delete drafts                     | PASS/FAIL/SKIP | e.g., "both drafts deleted, 404 on re-fetch"             |
| 35   | Get conversation                  | PASS/FAIL/SKIP | e.g., "thread returned in chronological order"           |
| 36   | Get attachment                    | PASS/FAIL/SKIP | e.g., "metadata + base64 under size limit"               |
| 37   | Mark read (idempotent + restore)  | PASS/FAIL/SKIP | e.g., "state written, repeat identical, original restored" |
| 38   | Set flag (+ invalid refused)      | PASS/FAIL/SKIP | e.g., "flagged written; 'urgent' refused naming 3 values" |
| 39   | Set categories (+ clear)          | PASS/FAIL/SKIP | e.g., "set from response; empty value cleared all"       |
| 40   | Move message (new ID follows)     | PASS/FAIL/SKIP | e.g., "new id returned; original 404s; name destination refused" |
| 41   | Add attachment to a draft         | PASS/FAIL/SKIP | e.g., "attachment id and size confirmed; non-draft refused" |
| 42   | Find meeting times (candidates)   | PASS/FAIL      | e.g., "5 slots with confidence; one-sided window refused" |
| 43   | Get schedule (per-mailbox)        | PASS/FAIL      | e.g., "self section returned; unknown mailbox carried Error:" |
| 44   | Add attachment to an event        | PASS/FAIL      | e.g., "attachment id and size confirmed; bad base64 refused" |
| 45   | List event attachments            | PASS/FAIL      | e.g., "crud-test.txt listed, metadata only, no content"  |
| 46   | Get event attachment (round trip) | PASS/FAIL      | e.g., "base64 decodes to the bytes sent; event deleted"  |
| 47   | Contacts help (five verbs)        | PASS/FAIL/SKIP | e.g., "help, search, get_contact, list_people, get_person" |
| 48   | Search contacts and people        | PASS/FAIL/SKIP | e.g., "matches labelled by source; blank query refused"  |
| 49   | Get one saved contact             | PASS/FAIL/SKIP | e.g., "all email addresses returned; empty id refused"   |
| 50   | List relevance-ranked people      | PASS/FAIL/SKIP | e.g., "relevance order preserved; total count present"   |
| 51   | Get one relevance-ranked person   | PASS/FAIL/SKIP | e.g., "scored addresses labelled with display name"      |
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
