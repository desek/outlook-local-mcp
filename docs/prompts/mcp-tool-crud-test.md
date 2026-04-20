# MCP Tool CRUD Lifecycle Test

Step-by-step instruction for Claude Code to exercise the calendar MCP tools through a complete create-read-update-delete cycle with verification at each stage.

## Prerequisites

- The MCP server `outlookCalendar` is running and connected.
- At least one account is authenticated (verify with `account_list`).
- The server **must** be configured with `LOG_LEVEL=debug` and file logging enabled (`LOG_FILE` set). Both are verified in Step 0.

## Instructions

Follow every step sequentially. Use the **default account** (omit `account` param) unless the user specifies otherwise. Omit the `output` parameter for all read operations (the default is `text`) unless a step specifies otherwise.

Pick a test date **7 days from today** to avoid conflicts with real events. Use the timezone `Europe/Amsterdam` for all operations.

### Step 0 -- Verify connectivity and server config

**0a.** Call `mcp__outlookCalendar__status` with `output: "summary"` (the full JSON config is needed for this verification step).

- **Verify:** At least one account is listed with an authenticated status.
- **Fail:** Stop and report the authentication issue.

**0b.** Record the top-level status fields and the `config` object from the Step 0a JSON response.

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

**0c.** Call `mcp__outlookCalendar__status` without the `output` parameter (default `text` mode).

- **Verify:** The response is plain text (not JSON).
- **Verify:** The text includes: server version, timezone, uptime, account list with auth state, and feature flags.
- **Verify:** The full configuration details (logging paths, Graph API settings, identity config) are NOT present in the text output.
- **Fail:** If the default response is JSON or if essential health fields are missing from the text.

### Step 1 -- List accounts

Call `mcp__outlookCalendar__account_list`.

- **Verify:** The response is plain text (not JSON) listing accounts with labels and authentication state.
- **Verify:** At least one account shows an authenticated status.
- **Record:** The number of accounts and their labels for the environment report.
- **Record:** If **two or more** accounts show authenticated status, set **multi-account mode** to `true`. Record the first authenticated account that is NOT the default as the **attendee account label**.
- If only one account is authenticated, set **multi-account mode** to `false`.
- **Fail:** If no accounts are returned or none are authenticated.

### Step 2 -- List calendars

Call `mcp__outlookCalendar__calendar_list`.

- **Verify:** The response is plain text listing calendars.
- **Verify:** At least one calendar is present (the default calendar).
- **Record:** The default calendar name and ID.
- **Fail:** If no calendars are returned.

**If multi-account mode:** Also call `mcp__outlookCalendar__calendar_list` with `account: <attendee account label>`.

- **Verify:** The response is plain text listing the attendee's calendars.
- **Record:** The `owner` email address from the attendee's default calendar as the **attendee email**. If the email cannot be determined from the text response, call again with `output: "summary"` to extract the email, or ask the user for the attendee's email address.
- **Fail:** If the attendee account's calendars cannot be listed (the account may not be properly authenticated).

### Step 3 -- Baseline list

Call `mcp__outlookCalendar__calendar_list_events` for the test date (use the `date` param with the ISO 8601 date, e.g. `2026-03-29`).

- **Verify:** The response is plain text (not JSON).
- Record the number of events from the total count line. This is the **baseline count**.
- Note any existing event subjects to avoid collisions.

### Step 4 -- Create event

Call `mcp__outlookCalendar__calendar_create_event` with:

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

Call `mcp__outlookCalendar__calendar_search_events` with:

| Parameter        | Value                              |
|------------------|------------------------------------|
| `created_by_mcp` | `true`                            |
| `query`          | The unique timestamp portion of the subject from Step 4 |
| `date`           | Test date (ISO 8601)              |

- **Verify:** The text results contain the event created in Step 4 (match by event ID in the text).
- **Verify:** The `created_by_mcp` filter correctly narrows results to MCP-created events only.
- **Fail:** If the event is missing, the provenance tag was not stamped during creation.

### Step 6 -- Search with "next_week" date shorthand

Call `mcp__outlookCalendar__calendar_search_events` with:

| Parameter | Value                              |
|-----------|------------------------------------|
| `query`   | The unique timestamp portion of the subject from Step 4 |
| `date`    | `next_week`                       |

- **Verify:** The text results contain the event created in Step 4 (match by event ID in the text).
- **Fail:** If the event is not found, the `next_week` date shorthand is not resolving correctly.

### Step 7 -- Search with "this_week" date shorthand

Call `mcp__outlookCalendar__calendar_search_events` with:

| Parameter | Value                              |
|-----------|------------------------------------|
| `query`   | The unique timestamp portion of the subject from Step 4 |
| `date`    | `this_week`                       |

- **Verify:** The results do NOT contain the event created in Step 4 (the test date is 7 days from today, outside the current week).
- **Fail:** If the event appears, the `this_week` date boundary is incorrect.

### Step 8 -- Get created event

Call `mcp__outlookCalendar__calendar_get_event` with the saved event ID.

- **Verify:** The response is plain text (not JSON).
- **Verify:**
  - Subject matches what was sent in Step 4.
  - Location shows `Test Room`.
  - Start time corresponds to test date `14:00` in `Europe/Amsterdam`.
  - Show As is `free`.
  - A body preview line is present (containing text from the `body` parameter in Step 4).
- **Fail:** Report any mismatched field.

### Step 9 -- Update event

Call `mcp__outlookCalendar__calendar_update_event` with:

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

**10a.** Call `mcp__outlookCalendar__calendar_get_event` with the saved event ID (default `text` output).

- **Verify:** The response is plain text (not JSON).
- **Verify:**
  - Subject ends with `(updated)`.
  - Location shows `Updated Room`.
  - End time corresponds to test date `15:00` in `Europe/Amsterdam`.
  - Show As is `busy`.
  - Start time is **unchanged** (still `14:00`).
  - A body preview line is present containing `Agenda` and the agenda items text (plain-text snippet, not HTML).
- **Fail:** Report any mismatched field.

**10b.** Call `mcp__outlookCalendar__calendar_get_event` with the saved event ID and `output: "raw"`.

- **Verify:** The response is JSON (not plain text).
- **Verify:** The `body.content` field contains the full HTML body set in Step 9 (including the `<h2>Agenda</h2>` and `<ol>` tags).
- **Verify:** The `bodyPreview` field is also present as a plain-text snippet.
- **Purpose:** This confirms the body escalation pattern -- `bodyPreview` in default text mode is sufficient to determine whether the full HTML body retrieval via `output=raw` is needed.
- **Fail:** If the full HTML body is not present in raw mode, or if the text default in Step 10a leaked HTML tags.

### Step 11 -- Get free/busy

Call `mcp__outlookCalendar__calendar_get_free_busy` with:

| Parameter | Value                 |
|-----------|-----------------------|
| `date`    | Test date (ISO 8601)  |

- **Verify:** The response is plain text showing schedule availability.
- **Verify:** The text contains a busy period that overlaps with the test event's time range (14:00–15:00 Europe/Amsterdam).
- **Verify:** The busy period's status is `busy`.
- **Fail:** If no busy period is found covering the test event time, or the status does not match.

### Step 12 -- Reschedule event

Call `mcp__outlookCalendar__calendar_reschedule_event` with:

| Parameter            | Value                              |
|----------------------|------------------------------------|
| `event_id`           | Saved event ID                     |
| `new_start_datetime` | Test date at `17:00:00`            |
| `new_start_timezone` | `Europe/Amsterdam`                 |

- **Pass:** Response is a plain text confirmation containing `Event rescheduled:` and the event subject.

### Step 13 -- Get rescheduled event

Call `mcp__outlookCalendar__calendar_get_event` with the saved event ID.

- **Verify:** The response is plain text.
- **Verify:**
  - Start time corresponds to test date `17:00` in `Europe/Amsterdam`.
  - End time corresponds to test date `18:00` in `Europe/Amsterdam` (original 1-hour duration preserved from Step 9's update).
  - Subject is **unchanged** (still ends with `(updated)`).
  - Location is **unchanged** (still `Updated Room`).
- **Fail:** Report any mismatched field or if duration was not preserved.

### Step 14 -- Delete event

Call `mcp__outlookCalendar__calendar_delete_event` with the saved event ID.

- **Pass:** Response is plain text containing `Event deleted:` and the event ID.

### Step 15 -- Get deleted event (expect failure)

Call `mcp__outlookCalendar__calendar_get_event` with the saved event ID.

- **Pass:** The call returns an error or "not found" response.
- **Fail:** If the event is still returned, report that deletion did not take effect.

### Step 16 -- Provenance search (after deletion)

Call `mcp__outlookCalendar__calendar_search_events` with:

| Parameter        | Value                              |
|------------------|------------------------------------|
| `created_by_mcp` | `true`                            |
| `query`          | The unique timestamp portion of the subject from Step 4 |
| `date`           | Test date (ISO 8601)              |

- **Verify:** The deleted event does NOT appear in the results.
- **Fail:** If the event still appears in provenance search after deletion.

### Step 17 -- Verify list after deletion

Call `mcp__outlookCalendar__calendar_list_events` for the same test date as Step 3.

- **Verify:** The response is plain text.
- **Verify:** The test event subject does NOT appear in the results.
- **Verify:** The event count from the total count line is equal to the **baseline count** from Step 3.
- **Fail:** Report if the deleted event still appears.

### Step 18 -- Create Teams meeting

> **Note:** In multi-account mode, this step uses `calendar_create_meeting` (the meeting variant) because attendees are involved. The LLM should present a draft summary (subject, date/time, attendee list, location, body preview) and wait for user confirmation before calling the tool. If any attendee email domain differs from the user's own domain, the LLM should also display an external attendee warning. Confirm when prompted.

**If multi-account mode**, call `mcp__outlookCalendar__calendar_create_meeting` with:

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

**If single-account mode**, call `mcp__outlookCalendar__calendar_create_event` with the same parameters **without** the `attendees` field (single-account mode uses the event variant since no attendees are involved).

- **Pass:** Response is a plain text confirmation containing the event subject and an `ID:` line.
- Save the returned **Teams event ID** from the `ID:` line.
- Report the created event subject and ID.

### Step 19 -- Verify Teams meeting details

Call `mcp__outlookCalendar__calendar_get_event` with the saved Teams event ID and `output: "raw"`.

- **Verify:**
  - `isOnlineMeeting` is `true`.
  - `onlineMeeting.joinUrl` is a non-empty string.
  - `body.content` contains a Teams join link (look for `teams.microsoft.com` or the `joinUrl` value).
- **If multi-account mode, also verify:**
  - `attendees` array contains at least one entry with the attendee email.
- **Fail:** Report any missing Teams meeting information.

### Step 20 -- Verify invitation on attendee calendar

> **Multi-account only.** If single-account mode, mark this step **SKIP**.

Call `mcp__outlookCalendar__calendar_search_events` with:

| Parameter | Value                              |
|-----------|------------------------------------|
| `account` | Attendee account label             |
| `query`   | The unique timestamp portion of the subject from Step 18 |
| `date`    | Test date (ISO 8601)               |

- **Verify:** The text results contain the Teams meeting created in Step 18 (match by subject).
- **Record:** The **attendee event ID** from the text result. If the ID is not visible in the text, call again with `output: "summary"` to extract it (it may differ from the organizer's event ID).
- **Fail:** If the meeting does not appear on the attendee's calendar, the invitation was not delivered.

### Step 21 -- Respond from attendee

> **Multi-account only.** If single-account mode, mark this step **SKIP**.

Call `mcp__outlookCalendar__calendar_respond_event` with:

| Parameter       | Value                              |
|-----------------|------------------------------------|
| `account`       | Attendee account label             |
| `event_id`      | Attendee event ID from Step 20     |
| `response`      | `tentative`                        |
| `comment`       | `CRUD test -- tentative response`  |
| `send_response` | `true`                             |

- **Pass:** Response is plain text containing `Event tentatively accepted:` and the event ID.
- **Fail:** If the call returns an error.

### Step 22 -- Verify attendee response from organizer

> **Multi-account only.** If single-account mode, mark this step **SKIP**.

Call `mcp__outlookCalendar__calendar_get_event` with the saved Teams event ID (organizer's ID) and `output: "raw"`.

- **Verify:** The `attendees` array contains an entry for the attendee email with `status.response` equal to `tentativelyAccepted`.
- **Fail:** If the attendee's response status has not updated.

### Step 22a -- Update meeting (meeting tool)

> **Multi-account only.** If single-account mode, mark this step **SKIP**.

> **Note:** This step uses `calendar_update_meeting` (the meeting variant) because the event has attendees. The LLM should present a draft summary of the changes and affected attendees, then wait for user confirmation before calling the tool. Confirm when prompted.

Call `mcp__outlookCalendar__calendar_update_meeting` with:

| Parameter  | Value                                                    |
|------------|----------------------------------------------------------|
| `event_id` | Saved Teams event ID                                     |
| `subject`  | Append ` (meeting updated)` to original subject from Step 18 |
| `body`     | `Updated meeting agenda -- CRUD test`                    |

- **Pass:** Response is a plain text confirmation containing `Event updated:` and the event subject.

### Step 22b -- Verify meeting update

> **Multi-account only.** If single-account mode, mark this step **SKIP**.

Call `mcp__outlookCalendar__calendar_get_event` with the saved Teams event ID.

- **Verify:** Subject ends with `(meeting updated)`.
- **Verify:** Body preview contains `Updated meeting agenda`.
- **Verify:** Start time and end time are unchanged from Step 18.
- **Fail:** Report any mismatched field.

### Step 22c -- Reschedule meeting (meeting tool)

> **Multi-account only.** If single-account mode, mark this step **SKIP**.

> **Note:** This step uses `calendar_reschedule_meeting` (the meeting variant) because the event has attendees. The LLM should present a summary showing the event subject, current time, proposed new time, and attendee list, then wait for user confirmation. Confirm when prompted.

Call `mcp__outlookCalendar__calendar_reschedule_meeting` with:

| Parameter            | Value                              |
|----------------------|------------------------------------|
| `event_id`           | Saved Teams event ID               |
| `new_start_datetime` | Test date at `17:30:00`            |
| `new_start_timezone` | `Europe/Amsterdam`                 |

- **Pass:** Response is a plain text confirmation containing `Event rescheduled:` and the event subject.

### Step 22d -- Verify meeting reschedule

> **Multi-account only.** If single-account mode, mark this step **SKIP**.

Call `mcp__outlookCalendar__calendar_get_event` with the saved Teams event ID.

- **Verify:** Start time corresponds to test date `17:30` in `Europe/Amsterdam`.
- **Verify:** End time corresponds to test date `18:00` in `Europe/Amsterdam` (original 30-minute duration preserved from Step 18).
- **Verify:** Subject is unchanged (still ends with `(meeting updated)`).
- **Fail:** Report any mismatched field or if duration was not preserved.

### Step 23 -- Respond to own meeting (expect failure)

Call `mcp__outlookCalendar__calendar_respond_event` with:

| Parameter   | Value                                    |
|-------------|------------------------------------------|
| `event_id`  | Saved Teams event ID                     |
| `response`  | `accept`                                 |
| `comment`   | `CRUD test -- organizer self-response`   |

- **Pass:** The call returns an error (the authenticated user is the organizer, not an attendee; responding to your own meeting is not permitted).
- **Fail:** If the call succeeds, the server is not enforcing the organizer/attendee distinction.

### Step 24 -- Cancel Teams meeting

> **Note:** This event has attendees (in multi-account mode). The LLM should present a summary (subject, time, attendee list) and wait for user confirmation before calling the tool. If any attendee is external, the LLM should also display an external attendee warning. Confirm when prompted.

Call `mcp__outlookCalendar__calendar_cancel_meeting` with:

| Parameter  | Value                              |
|------------|------------------------------------|
| `event_id` | Saved Teams event ID               |
| `comment`  | `Automated CRUD test cancellation` |

- **Pass:** Response is plain text containing `Event cancelled:` and the event ID.

### Step 25 -- Verify cancellation

Call `mcp__outlookCalendar__calendar_get_event` with the saved Teams event ID.

- **Pass:** The call returns an error or "not found" response (cancelled meetings are removed from the calendar).
- **Fail:** If the event is still returned as a non-cancelled event.

**If multi-account mode**, also call `mcp__outlookCalendar__calendar_get_event` with `account: <attendee account label>` and the **attendee event ID** from Step 20.

- **Verify:** The call returns an error/"not found", or the event shows `isCancelled: true`.
- **Fail:** If the event is still active on the attendee's calendar.

### Step 26 -- Verify server logs

Read the **log file path** recorded in Step 0. Inspect the log entries emitted during the test (from Step 1 onward).

- **Verify:** Every tool call has a `DEBUG`-level "tool called" entry and an `INFO`-level (or `ERROR` for Steps 15, 23, 25) "tool completed" entry.
- **Verify:** The `calendar_create_event` log includes the event ID.
- **Verify:** The `calendar_delete_event` log includes the event ID and confirms deletion.
- **Verify:** The `calendar_reschedule_event` log includes the event ID.
- **Verify:** The `calendar_cancel_meeting` log includes the event ID.
- **If multi-account mode:** Verify the `calendar_create_meeting` log (Step 18) includes the event ID.
- **If multi-account mode:** Verify the `calendar_update_meeting` log (Step 22a) includes the event ID.
- **If multi-account mode:** Verify the `calendar_reschedule_meeting` log (Step 22c) includes the event ID.
- **Verify:** The `calendar_get_event` call after deletion (Step 15) is logged at `ERROR` level with `ErrorItemNotFound`.
- **Verify:** The `calendar_respond_event` call (Step 23) is logged at `ERROR` level.
- **If multi-account mode:** Verify the `calendar_respond_event` call (Step 21) is logged at `INFO` level (success).
- **Verify:** No unexpected `ERROR` or `WARN` entries appear (the Step 15, 23, and 25 errors are expected; Step 20 attendee-side errors in multi-account mode from Step 25 are also expected).
- **Fail:** Report any missing log entries or unexpected errors.

### Step 27 -- Force refresh authenticated account token

Call `mcp__outlookCalendar__account_refresh` with:

| Parameter | Value                                        |
|-----------|----------------------------------------------|
| `label`   | The default account label from Step 1        |

- **Pass:** Response is plain text confirming the refresh and including a new token expiry timestamp.
- **Verify:** The response references the account's label and/or UPN.
- **Fail:** If the call errors or the expiry time is absent from the response.

### Step 28 -- Log out of account

> **Note:** This test requires at least one non-default account in addition to the default account, or `account_login` in Step 29 must be used to restore access before further tests. If only one account is registered, mark Steps 28 and 29 **SKIP** to avoid leaving the test environment unauthenticated.

Pick a **non-default authenticated account** from Step 1's list (the **attendee account label** in multi-account mode). Call `mcp__outlookCalendar__account_logout` with:

| Parameter | Value                                     |
|-----------|-------------------------------------------|
| `label`   | Non-default authenticated account label   |

- **Pass:** Response is plain text confirming the logout.
- **Verify:** A subsequent `mcp__outlookCalendar__account_list` call shows the account as `disconnected` while still listing the entry (not removed).
- **Verify:** Calling any calendar tool with `account: <logged-out label>` returns an actionable error mentioning `disconnected` and `account_login`.
- **Fail:** If the account is removed, still shown as authenticated, or if the disconnected-account error is missing.

### Step 29 -- Log back in to account

Call `mcp__outlookCalendar__account_login` with:

| Parameter | Value                               |
|-----------|-------------------------------------|
| `label`   | The label from Step 28              |

Complete the authentication flow interactively when prompted (browser, device code, or auth code, per the account's persisted auth method).

- **Pass:** Response is plain text confirming re-authentication, including the account's UPN.
- **Verify:** A subsequent `account_list` call shows the account back as `authenticated`.
- **Verify:** Calling `account_login` again on the same (now connected) account returns an error stating the account is already connected.
- **Fail:** If the account does not return to the authenticated state or the already-connected guard does not trigger.

### Step 30 -- Mail list filter matrix (skip if mail disabled)

If `config.features.mail_enabled` from Step 0b is `false`, **skip** Steps 30 through 36 and record them as SKIP.

Call `mcp__outlookCalendar__mail_list_messages` four times with the following filter combinations and record whether each call returns plain text, a sensible total count, and the expected filtering behavior:

| Call | Parameters                              | Expected                                                         |
|------|-----------------------------------------|------------------------------------------------------------------|
| 30a  | `folder: "Inbox", is_read: false`       | Only unread messages are listed; count matches folder unread     |
| 30b  | `folder: "Inbox", flag_status: "flagged"` | Only flagged messages are listed                                |
| 30c  | `folder: "Inbox", provenance: "created_by_mcp"` | Only MCP-tagged messages (may be empty if none created yet)  |
| 30d  | `folder: "Inbox"` (no filters, baseline) | Baseline message count recorded for comparison                  |

- **Verify:** All calls return plain text. The filtered counts are less than or equal to the baseline.
- **Fail:** If any call returns an error or ignores the filter.

### Step 31 -- Create draft (skip if mail management disabled)

If `config.features.mail_manage_enabled` from Step 0b is `false`, **skip** Steps 31 through 35 and record them as SKIP.

Call `mcp__outlookCalendar__mail_create_draft` with:

| Parameter   | Value                                                   |
|-------------|---------------------------------------------------------|
| `to`        | The default account's own UPN (self-send for test)     |
| `subject`   | `"CRUD test draft"`                                     |
| `body`      | `"Created by MCP CRUD lifecycle test."`                |
| `importance`| `"normal"`                                              |

- **Verify:** Response is a plain text confirmation including the draft's message ID.
- **Record:** The draft's message ID as **draft ID**.
- **Fail:** If the tool errors or the message ID is not returned.

### Step 32 -- Update draft

Call `mcp__outlookCalendar__mail_update_draft` with:

| Parameter  | Value                     |
|------------|---------------------------|
| `id`       | The **draft ID**          |
| `subject`  | `"CRUD test draft (updated)"` |

- **Verify:** Response is plain text confirming the update.
- **Verify:** A subsequent `mail_get_message` call for **draft ID** shows the updated subject.
- **Fail:** If the update is not reflected.

### Step 33 -- Create reply draft

Call `mcp__outlookCalendar__mail_create_reply_draft` with:

| Parameter  | Value                                         |
|------------|-----------------------------------------------|
| `id`       | The **draft ID** (reply to the draft itself) |
| `comment`  | `"Replying to my own draft."`                |

If the server rejects replying to a draft, instead pick the most recent message from `mail_list_messages` on `Inbox` and reply to it. Record the reply draft ID as **reply draft ID**.

- **Verify:** Response is plain text confirming the reply draft creation with a new message ID.
- **Fail:** If no reply draft is created.

### Step 34 -- Delete reply draft and original draft

Call `mcp__outlookCalendar__mail_delete_draft` twice:

1. `id: <reply draft ID>` (if created)
2. `id: <draft ID>`

- **Verify:** Both calls return plain text delete confirmations.
- **Verify:** A subsequent `mail_get_message` for each ID returns an error (message no longer exists).
- **Fail:** If any draft remains retrievable.

### Step 35 -- Get conversation

Call `mcp__outlookCalendar__mail_list_messages` with `folder: "Inbox", top: 1` and record the first message's `conversationId` as **conversation ID**. If Inbox is empty, skip Step 35.

Call `mcp__outlookCalendar__mail_get_conversation` with `id: <conversation ID>`.

- **Verify:** Response is plain text listing one or more messages in chronological order.
- **Fail:** If the call errors for a valid conversation ID.

### Step 36 -- Get attachment

Using `mail_list_messages` with `folder: "Inbox", has_attachments: true, top: 1` pick a message that has attachments. If none found, skip Step 36.

Call `mcp__outlookCalendar__mail_get_message` on the chosen message with `output: "summary"` to enumerate its attachment IDs. Then call `mcp__outlookCalendar__mail_get_attachment` with:

| Parameter       | Value                                       |
|-----------------|---------------------------------------------|
| `message_id`    | The chosen message ID                       |
| `attachment_id` | The first attachment's ID                   |

- **Verify:** Response is plain text with attachment metadata (name, size, content type).
- **Verify:** If the attachment is within the configured size limit, content is returned (base64); otherwise an explanatory message is returned.
- **Fail:** If the attachment cannot be retrieved for a valid ID.

## Reporting

After all steps, print a summary table. Every row **MUST** include a short `Comment` (under ~120 characters) explaining the result — for PASS rows, a brief confirmation of what was verified; for FAIL rows, the failure cause (tool name, error, mismatch); for SKIP rows, the reason (e.g., "single-account mode"). Do not leave the `Comment` column blank.

```
| Step | Action                            | Result         | Comment                                                  |
|------|-----------------------------------|----------------|----------------------------------------------------------|
| 0a   | Verify connectivity (summary)     | PASS/FAIL      | e.g., "default account authenticated; DEBUG logging on"  |
| 0b   | Record config                     | PASS/FAIL      | e.g., "log_file present; timezone=Europe/Stockholm"      |
| 0c   | Verify text default for status    | PASS/FAIL      | e.g., "plain text, no config leak"                       |
| 1    | List accounts (text)              | PASS/FAIL      | e.g., "1 authenticated, 1 disconnected"                  |
| 2    | List calendars (text)             | PASS/FAIL      | e.g., "default + Birthdays"                              |
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
| 19   | Verify Teams meeting details      | PASS/FAIL      | e.g., "isOnlineMeeting=true, joinUrl present"            |
| 20   | Verify invitation (attendee)      | PASS/FAIL/SKIP | e.g., "single-account mode" or "invitation visible"      |
| 21   | Respond from attendee (text)      | PASS/FAIL/SKIP | e.g., "single-account mode" or "tentative response sent" |
| 22   | Verify attendee response          | PASS/FAIL/SKIP | e.g., "single-account mode" or "status=tentative"        |
| 22a  | Update meeting (meeting tool)     | PASS/FAIL/SKIP | e.g., "single-account mode" or "meeting updated"         |
| 22b  | Verify meeting update             | PASS/FAIL/SKIP | e.g., "single-account mode" or "fields updated"          |
| 22c  | Reschedule meeting (meeting tool) | PASS/FAIL/SKIP | e.g., "single-account mode" or "rescheduled 17:30"       |
| 22d  | Verify meeting reschedule         | PASS/FAIL/SKIP | e.g., "single-account mode" or "duration preserved"      |
| 23   | Respond to own meeting (err)      | PASS/FAIL      | e.g., "organizer self-response rejected"                 |
| 24   | Cancel Teams meeting (text)       | PASS/FAIL      | e.g., "cancellation confirmation with event ID"          |
| 25   | Verify cancellation               | PASS/FAIL      | e.g., "ErrorItemNotFound as expected"                    |
| 26   | Verify server logs                | PASS/FAIL      | e.g., "all expected levels/IDs present"                  |
| 27   | Force refresh token (text)        | PASS/FAIL      | e.g., "label + expiry in plain text"                     |
| 28   | Log out non-default account       | PASS/FAIL/SKIP | e.g., "only default authenticated" or "logged out"       |
| 29   | Log back in non-default account   | PASS/FAIL/SKIP | e.g., "only default authenticated" or "re-authenticated" |
| 30   | Mail list filter matrix           | PASS/FAIL/SKIP | e.g., "is_read/flag_status/provenance filters honored"   |
| 31   | Create mail draft                 | PASS/FAIL/SKIP | e.g., "draft id returned"                                |
| 32   | Update mail draft                 | PASS/FAIL/SKIP | e.g., "subject updated"                                  |
| 33   | Create reply draft                | PASS/FAIL/SKIP | e.g., "reply draft created"                              |
| 34   | Delete drafts                     | PASS/FAIL/SKIP | e.g., "both drafts deleted, 404 on re-fetch"             |
| 35   | Get conversation                  | PASS/FAIL/SKIP | e.g., "thread returned in chronological order"           |
| 36   | Get attachment                    | PASS/FAIL/SKIP | e.g., "metadata + base64 under size limit"               |
```

Then print the **environment** section using all values recorded in Steps 0b and 1:

```
| Property            | Value                                  |
|---------------------|----------------------------------------|
| Server version      | <version from Step 0b>                 |
| Default timezone    | <timezone from Step 0b>                |
| Uptime (seconds)    | <server_uptime_seconds from Step 0b>   |
| Auth method         | <auth_method> (<auth_method_source>)   |
| Client ID           | <client_id from Step 0b>               |
| Tenant ID           | <tenant_id from Step 0b>               |
| Token cache backend | keychain / file                        |
| Read-only mode      | true / false                           |
| Provenance tag      | <provenance_tag from Step 0b>          |
| Log level           | debug                                  |
| Log format          | <log_format from Step 0b>              |
| Log file            | <path from Step 0b>                    |
| PII sanitization    | true / false                           |
| Audit logging       | true / false                           |
| Max retries         | <max_retries from Step 0b>             |
| Request timeout (s) | <request_timeout_seconds from Step 0b> |
| Multi-account mode  | true / false                           |
| Attendee account    | <label from Step 1> / N/A             |
| Attendee email      | <email from Step 2> / N/A             |
```

If any step FAILs, stop execution and report the failure details including the full tool response.
