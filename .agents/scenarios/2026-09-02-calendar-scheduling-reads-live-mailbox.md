---
date: 2026-09-02
source: cr (CR-0080)
surface: cli (MCP stdio client driving the built ./outlook-local-mcp binary)
outcome: not-run
runs: "0 of 0 attempted; derived from the CR, not yet run against a live mailbox"
coverage_gap: false
---

# Scheduling a meeting across two mailboxes

## Status

**Derived, not yet run.** This artifact is the executable form of the acceptance
requirement, written so the run can be performed without re-deriving it. No run has
happened: the scenario needs a live authenticated mailbox with a second readable mailbox
in the same tenant, and neither was available to the session that wrote it. The `outcome`
and `runs` frontmatter fields say so, and they MUST be rewritten by whoever performs the
first run rather than left as they are.

Nothing below has been observed. Every unit test covering these two verbs drives an
`httptest` server against canned response bodies, so **neither `findMeetingTimes` nor
`getSchedule` has been exercised against Microsoft Graph.** Four things are unconfirmed
against the live service and are what this scenario exists to settle: whether the twenty
mailbox ceiling this server enforces matches the one the service enforces; whether a
mailbox the account may not read returns a per-mailbox error inside a successful reply
rather than failing the whole call; whether `workingHours` is populated for an ordinary
tenant mailbox or omitted; and whether the availability-view interval the request names is
the one the reply is divided by.

## Goal

A person wants to find a time two colleagues could meet, then look at what those two
colleagues' calendars actually show across that day so they can sanity-check the
suggestion, and finally confirm their own calendar view is unchanged by any of it.

The surface is the MCP server as an assistant drives it, over stdio, against the built
binary, with the `calendar` aggregate tool and its `operation` verb dispatch. It is not
the handler constructors in `internal/tools/`.

## Preconditions

* An authenticated account is present in the token cache and resolves as the default
  account.
* Two mailbox SMTP addresses in the same tenant that this account may read availability
  for are known and recorded before the run, as `MBOX_A` and `MBOX_B`. Both should have at
  least one busy block on the chosen day, or the free/busy assertions grade nothing.
* One further address the account may **not** read availability for is known and recorded
  as `MBOX_DENIED`. Without it the per-mailbox error path is unreachable, which is the
  single most valuable thing this scenario settles.
* A calendar day is chosen and recorded as `DAY`, in `YYYY-MM-DD`, on which the signed-in
  account has at least one event of its own. The last assertion needs it.
* No configuration change is required: both verbs register unconditionally, in every
  configuration, and add no OAuth scope. `OUTLOOK_MCP_READ_ONLY` may be set either way,
  since both verbs are reads; running under `OUTLOOK_MCP_READ_ONLY=true` additionally
  confirms neither verb is behind the write guard.
* The binary under test is rebuilt at the path `.mcp.json` names before the run, and its
  reported commit matches `git rev-parse --short HEAD`. A run against a stale binary
  produces a report that looks authoritative and describes a different commit:

  ```bash
  go build -ldflags="-X main.commit=$(git rev-parse --short HEAD) -X main.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o ./outlook-local-mcp ./cmd/outlook-local-mcp
  ```

* On macOS, the binary is signed with a stable identity and the keychain prompt is
  answered interactively once before any headless run. An ad-hoc-signed binary blocks on
  an invisible prompt at the first authenticated call, which reads as a timeout.

## Steps at the user surface

Driven as MCP `tools/call` requests against the built server over stdio. Record the value
each step reports; several later steps consume them.

1. `calendar` with `operation="find_meeting_times"`, `attendees` set to a JSON array naming
   `MBOX_A` as `required` and `MBOX_B` as `optional`, `meeting_duration="PT30M"`,
   `start_datetime` the start of `DAY` and `end_datetime` its end. Record the first
   suggested slot's start and end as `SLOT_START` and `SLOT_END`, and its confidence.
2. Repeat step 1 with `output="raw"`. This is the tier comparison: the raw reply must carry
   at least one field the default text tier does not state.
3. `calendar` with `operation="find_meeting_times"` and `attendees` naming only `MBOX_A`,
   with no `start_datetime` or `end_datetime`. This grades the unbounded search and the
   `PT30M` default against the service rather than against a fixture.
4. `calendar` with `operation="get_schedule"`, `schedules="MBOX_A,MBOX_B"`, `date=DAY`,
   `availability_view_interval=30`.
5. `calendar` with `operation="get_schedule"`, `schedules="MBOX_A,MBOX_DENIED"`, `date=DAY`.
   This is the per-mailbox error path, which no fixture can settle.
6. `calendar` with `operation="get_schedule"`, `schedules="MBOX_A"`, and no `date`,
   `start_datetime`, or `end_datetime`.
7. `calendar` with `operation="get_schedule"`, `schedules` naming twenty-one addresses
   (`MBOX_A` repeated is sufficient), `date=DAY`.
8. `calendar` with `operation="get_schedule"`, `schedules="MBOX_A,not-an-email"`,
   `date=DAY`.
9. `calendar` with `operation="get_free_busy"`, `date=DAY`. This is the regression check
   that the two new verbs left the pre-existing availability read alone.

## Success condition

Graded on what the service returns for the calendars named, never on the wording of a
confirmation, and never on the step count. Both verbs are reads, so the end state graded
is the reply's agreement with the mailboxes rather than a mutation.

* Step 1 returns at least one suggested slot, bounded inside `DAY`, thirty minutes long,
  and stating a confidence. If it returns none, it states a reason for the emptiness rather
  than an empty list with no explanation, and the run records that reason as its finding.
* Step 2's raw reply carries a field absent from step 1's text reply. Identical content
  across the two tiers falsifies the tiering, whatever the payload sizes are.
* Step 3 returns suggestions with no window supplied, each thirty minutes long.
* Step 4 reports both `MBOX_A` and `MBOX_B`, each attributed to its own address, each with
  its free/busy blocks for `DAY`. Every block reported for a mailbox falls inside `DAY`.
  The mailboxes appear in the order the request named them. Whether working hours are
  reported for each mailbox is recorded either way: their absence is a finding about the
  tenant, not a failure of the verb, but it must be stated rather than assumed.
* Step 5 **succeeds**. Its reply names `MBOX_DENIED` with the service's own error code and
  message, and in the same reply still carries `MBOX_A`'s blocks. A call that fails
  outright, or that silently drops `MBOX_DENIED`, is the failure this verb exists to make
  impossible.
* Step 6 fails before any request reaches Graph, names the window parameters, and states
  what to supply.
* Step 7 fails naming the ceiling of twenty and stating that the list be split across
  several calls. If instead the service accepts twenty-one, that is a finding about the
  ceiling constant and is recorded rather than silently accepted.
* Step 8 fails naming the `schedules` parameter, naming `not-an-email`, and stating the
  correction to apply.
* Step 9 returns the signed-in account's own view for `DAY`, including event subjects,
  exactly as it did before this change. This is the assertion that the new verbs did not
  alter the one they sit beside.

## Restoration

None. Every step is a read; no step mutates a mailbox, and no configuration is changed for
the run. If step 7's twenty-first entry was constructed from a real address, nothing was
written to it.

## Run log

No runs. The first run appends its record here: the date, the driver invocation, the value
observed for each success-condition bullet, and how many attempts of how many passed.
