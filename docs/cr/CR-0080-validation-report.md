# CR-0080 Validation Report

Validated at `a53786c` on branch `docs/cr-implementation-set-0079-0083`.
Diff base: `caebec3` (last CR-0079 commit), per the implementation sequence the CR records.
Implementation commits: `8b9b481` (review), `e7e4337`, `54610eb`, `ea2ef12`, `b9690f2`, `a53786c` (finalize).
Re-validated after the gap-fix pass, which closed GAP-1 and GAP-2. Line references in the
GAP-1 evidence are against the post-fix tree; the rows the fix did not touch keep the
references established at `a53786c`.

## Summary

Requirements: 32/32 | Acceptance Criteria: 18/18 | Tests: 30/30 | Gaps: 0

Functional Requirements 25/25 PASS. Non-Functional Requirements 7/7 PASS.
Acceptance Criteria 18/18 PASS. Every Test-Strategy test exists, matches its
specified behaviour, and passes under `-race`. `make ci` exits 0 with a clean working tree.

**Gap fixing applied after the first pass.** Both gaps are closed. GAP-1 is FIXED: each
verb now appends its own correction to the two shared-helper refusals and to the bare
malformed-address refusal, on the tool result and on the log record alike, with assertions
on both channels. GAP-2 is RESOLVED as far as this session can take it: the live-mailbox
scenario is derived and persisted under `.agents/scenarios/`, marked `outcome: not-run`,
with the run itself pending for the user and recorded as such in the backlog.

## Check Pipeline

| Command | Result | Evidence |
|---|---|---|
| `make ci` (docs-bundle, surface-check, build, vet, fmt-check, tidy, lint, test, goreleaser-check, mcpb-validate) | PASS (exit 0) | `golangci-lint run` → `0 issues.`; all 16 packages `ok`; `mcpb validate` → schema passes |
| `git status --porcelain` after `make ci` | PASS (clean) | `surface-check` regenerated `site/src/generated/surface.json` without moving the working tree, so AC-11's drift gate holds |
| `go test -race ./internal/{tools,server,graph,surface}/ -run '<CR verification set>'` | PASS | 55 named tests, 0 failures (transcript in Test Strategy Verification below) |
| `go test ./internal/auth/ -run 'TestScopes_'` | PASS | 5 scope tests pass, gating AC-9 |
| `make crud-test` | NOT RUN | Paid harness; excluded by the orchestrator, and by the gap-fix run. Pending for the user; see GAP-2 |

Measured gates:

* `TestColdStartSchemaSize_Reduction`: post-change schema **20 152 bytes**, **72 % reduction**
  against the documented 74 000-byte baseline, floor 60 %. (`internal/server/schema_size_test.go:82-84`)
* `TestDescriptionLengthBounded`: PASS against `const maxLen = 4000`
  (`internal/tools/description_quality_test.go:147`). The test logs no figure; per NFR-3's
  own wording, the test decides.

## Requirement Verification

### Functional Requirements

| Req # | Description | Status | Evidence (file:line / test name) |
|---|---|---|---|
| FR-1 | Register exactly two calendar verbs; no new top-level tool; tool count stays 4 | PASS | `internal/server/calendar_verbs.go:116-117` (appended to `buildCalendarVerbs` slice); `internal/server/calendar_verbs.go:824`, `:886`; `TestRegisterTools_CalendarSchedulingReads` asserts `len(s.ListTools()) == 4` (`internal/server/server_test.go:1052-1055`) |
| FR-2 | Registered unconditionally; present in the operation enum in every configuration | PASS | `internal/server/calendar_verbs.go:116-117` (no gate branch); `TestRegisterTools_CalendarSchedulingReads` reads the enum under `default` and `maximal` (`internal/server/server_test.go:1039-1050`) |
| FR-3 | `find_meeting_times` requires `attendees`, parses to `[]models.AttendeeBaseable`, issues exactly one `POST /me/findMeetingTimes` | PASS | `internal/tools/find_meeting_times.go:91-98`, `:252-288` (`parseAttendeeBases`), `:187` (`client.Me().FindMeetingTimes().Post`); `TestFindMeetingTimes_Success` asserts `graph request count = 1` and both attendees on the wire (`internal/tools/find_meeting_times_test.go:109-114`); `TestFindMeetingTimes_RequiresAttendees` |
| FR-4 | Validate every attendee email with `ValidateEmail` and type with `ValidateAttendeeType`; reject before any Graph request | PASS | `internal/tools/find_meeting_times.go:270`, `:274`; `TestFindMeetingTimes_RejectsInvalidAttendeeEmail` asserts `calls == 0` |
| FR-5 | Optional `meeting_duration` ISO 8601; default `PT30M`; reject unparseable before any Graph request | PASS | `internal/tools/find_meeting_times.go:33` (const), `:100-104` (`serialization.ParseISODuration`); `TestFindMeetingTimes_RejectsUnparseableDuration`; `TestFindMeetingTimes_AppliesDurationAndCandidateDefaults` |
| FR-6 | Map both bounds to a `TimeConstraint`, validate each, omit when both absent, refuse when exactly one supplied | PASS | `internal/tools/find_meeting_times.go:120-134`, `:149-151`, `:293-301` (`buildTimeConstraint`); `TestFindMeetingTimes_RejectsHalfOpenWindow`, `TestFindMeetingTimes_RejectsInvalidDatetime`, `TestFindMeetingTimes_OmitsTimeConstraintWhenNoBounds` |
| FR-7 | `get_schedule` requires `schedules`, validates each address, refuses >20 naming the ceiling, issues one `POST /me/calendar/getSchedule` | PASS | `internal/tools/get_schedule.go:33` (`maxScheduleMailboxes = 20`), `:211-236`, `:138`; `TestGetSchedule_RequiresSchedules`, `TestGetSchedule_RejectsInvalidAddress`, `TestGetSchedule_EnforcesMailboxCeiling`, `TestGetSchedule_Success` (`count = 1`) |
| FR-8 | Require a resolved window; accept explicit datetimes or the `date` shorthand via `expandDateParam`; explicit wins; reject when nothing resolves | PASS | `internal/tools/get_schedule.go:252-281` (`resolveScheduleWindow`, calls `expandDateParam` at `:258`); `TestGetSchedule_RequiresWindow`, `TestGetSchedule_DateShorthandResolvesWindow`, `TestGetSchedule_ExplicitDatetimesBeatDateShorthand` |
| FR-9 | Return per-mailbox free/busy blocks and working hours, attributed to the mailbox | PASS | `internal/tools/get_schedule.go:164-174`; `internal/graph/scheduling_serialize.go:213-251` (`scheduleId`, `scheduleItems`, `workingHours`); `internal/tools/text_format.go` `FormatScheduleText` labels each section via `scheduleMailboxLabel`; `TestGetSchedule_Success`, `TestGetSchedule_ReturnsWorkingHours` |
| FR-10 | Surface a per-mailbox Graph error as a stated error, not a dropped mailbox or a failed call | PASS | `internal/graph/scheduling_serialize.go:246-248`, `:286-296` (`serializeFreeBusyError`); `internal/tools/text_format.go` `scheduleErrorLine`; `TestGetSchedule_PerMailboxErrorSurfaced` asserts the call succeeds, names the failing mailbox with message and response code, and keeps the other mailbox's blocks |
| FR-11 | Both verbs read-only: no write, create/update, send/forward, delete, and no parameter that does any | PASS | Published schemas are `attendees`, `meeting_duration`, `start_datetime`, `end_datetime`, `max_candidates`, `minimum_attendee_percentage`, `is_organizer_optional`, `timezone`, `account`, `output` (`internal/server/calendar_verbs.go:839-877`) and `schedules`, `date`, `start_datetime`, `end_datetime`, `availability_view_interval`, `timezone`, `account`, `output` (`:901-930`); handlers issue only `Post` reads (`find_meeting_times.go:187`, `get_schedule.go:138`); `TestSchedulingReadsRegisteredAndReadOnly` |
| FR-12 | Three tiers via `output`; no write confirmation; `raw` full Graph serialisation, `summary` a dedicated serializer, following `list_events` not `get_free_busy` | PASS | `internal/tools/find_meeting_times.go:86-89`, `:208-212`, `:217-222`; `internal/tools/get_schedule.go:85-88`, `:169-173`, `:177-182`; four serializers at `internal/graph/scheduling_serialize.go:29`, `:104`, `:164`, `:213`; `TestFindMeetingTimes_SummaryAndRawTiersDiffer`, `TestGetSchedule_SummaryAndRawTiersDiffer`, `TestSuggestionTiersDifferInFieldSet`, `TestSerializeScheduleTiersDifferInFieldSet` |
| FR-13 | Four annotation hints declared explicitly per the matrix | PASS | `internal/server/calendar_verbs.go:833-838`, `:895-900`; `TestSchedulingReadVerbAnnotations` asserts `ro=true de=false id=true ow=true` for both |
| FR-14 | Wrapped by the read chain under `calendar.<verb>`; not by the read-only guard | PASS | `internal/server/calendar_verbs.go:832`, `:894` (`wrap(...)`, audit op `"read"`, not `wrapWrite`); `TestSchedulingReadsCarryDotIdentity` reads the audit log and asserts `tool_name == calendar.find_meeting_times` / `calendar.get_schedule` with `operation_type == read`; `TestSchedulingReadsRegisteredAndReadOnly` builds with `readOnly: true` and asserts no refusal |
| FR-15 | Route through `RetryGraphCall` and `WithTimeout`; report timeouts with `TimeoutErrorMessage`; redact with the existing helpers | PASS | `internal/tools/find_meeting_times.go:179-201`; `internal/tools/get_schedule.go:130-152`; `TestSchedulingReadsHonourTimeoutAndRedaction` (three sub-cases per verb: hang, expired context naming `7s`, redacted `ErrorAccessDenied`) |
| FR-16 | Do not alter, deprecate, or remove `get_free_busy` | PASS | `internal/tools/get_free_busy.go` and `internal/tools/get_free_busy_test.go` absent from `git diff caebec3...HEAD --name-only`; the 12 `TestGetFreeBusy_*` tests pass unmodified |
| FR-17 | Each `Description` states when to prefer it over the other availability reads | PASS | `internal/server/calendar_verbs.go:826` ("Prefer this when the question is \"when could these people meet?\"; prefer get_schedule when… and get_free_busy when…"); `:888` ("Prefer this over get_free_busy when…; prefer get_free_busy for…; prefer find_meeting_times when…") |
| FR-18 | Non-empty `Summary` ≤80 chars, `Description`, ≥1 `Examples`, ≥1 resolving `SeeDocs` | PASS | `internal/server/calendar_verbs.go:825-831` (Summary 71 chars, 2 examples, `SeeDocs: concepts#output-tiers`), `:887-893` (Summary 64 chars, 2 examples); `TestEveryVerbHasSummary`, `TestEveryVerbHasDescription`, `TestSeeDocsAnchorsResolve` |
| FR-19 | Calendar domain `Intro` remains accurate and unchanged | PASS | `internal/server/server.go:88` reads `"Calendar operations for Microsoft Outlook via Microsoft Graph."` and enumerates no verb; `internal/server/server.go` is absent from the CR-0080 diff (`git log -1 -- internal/server/server.go` → `5f5c4ca`, a CR-0079 commit) |
| FR-20 | Extension manifest enumerates both verbs; `tools` array stays at four | PASS | `extension/manifest.json:57` (single description line gains `find_meeting_times (...)` and `get_schedule (...)`); `TestManifestDescribesEveryRegisteredVerb` PASS; `TestRegisterTools_CalendarSchedulingReads` asserts tool count 4 |
| FR-21 | `site/src/generated/surface.json` regenerated and committed | PASS | `site/src/generated/surface.json` +16/−4: calendar `fullCount` 15→17, `defaultCount` 15→17, totals 47→49 / 33→35; `make ci` `surface-check` left the tree clean; `TestCommittedManifestMatchesRecord` PASS |
| FR-22 | Steps 42 and 43 appended after Step 41; no renumbering; no skip range | PASS | `docs/prompts/mcp-tool-crud-test.md` +22/−0 (pure insertion, so nothing renumbered); Step 42 at the `find_meeting_times` block, Step 43 at the `get_schedule` block; neither carries skip language, and the file's skip ranges (lines 25, 550, 583) end at Step 41 |
| FR-23 | `verbInventoryGolden` regenerated with the two identities, sorted | PASS | `internal/tools/dispatch_registry_test.go` +2/−0: `calendar.find_meeting_times ro=true de=false id=true ow=true` and `calendar.get_schedule ro=true de=false id=true ow=true`, inserted in sort position; `TestVerbInventoryUnchangedAfterUpgrade` PASS |
| FR-24 | No change to the OAuth scope set in any configuration | PASS | No file under `internal/auth/` in the diff; `TestScopes_CalendarOnly`, `TestScopes_WithMail`, `TestScopes_MailManage`, `TestScopes_MailManageImpliesRead`, `TestScopes_NoMailSend` all PASS |
| FR-25 | Every numeric parameter declares its bound in the schema and rejects out-of-bound before any Graph request; no-default parameters omitted from the body | PASS | Schema bounds: `internal/server/calendar_verbs.go:855-856` (1–100), `:860-861` (0–100), `:917-918` (5–1440). Handler refusals before the call: `internal/tools/find_meeting_times.go:138-143`, `:156-162`; `internal/tools/get_schedule.go:111-116`. Omission: `find_meeting_times.go:156-166` gates `minimum_attendee_percentage` and `is_organizer_optional` on presence. `TestFindMeetingTimes_RejectsOutOfBoundNumerics`, `TestFindMeetingTimes_OmitsUnsuppliedOptionalsFromBody`, `TestFindMeetingTimes_SuppliedOptionalsReachBody`, `TestGetSchedule_RejectsOutOfBoundInterval`, `TestGetSchedule_AppliesIntervalDefault` |

### Non-Functional Requirements

| Req # | Description | Status | Evidence |
|---|---|---|---|
| NFR-1 | Handler per file named for the verb; formatters in `text_format.go`; serializers in a new `internal/graph/scheduling_serialize.go` | PASS | `internal/tools/find_meeting_times.go` (new, 312), `internal/tools/get_schedule.go` (new, 291); `internal/tools/text_format.go` +178 (`FormatMeetingTimeSuggestionsText`, `FormatScheduleText` appended after `FormatFreeBusyText`); `internal/graph/scheduling_serialize.go` (new, 323); no `scheduling_text_format.go` created |
| NFR-2 | Exactly one Graph request on the success path; no read-modify-write, no fan-out | PASS | `internal/tools/find_meeting_times.go:185-189`; `internal/tools/get_schedule.go:136-140` (single `SetSchedules` list, no loop); `TestFindMeetingTimes_Success` and `TestGetSchedule_Success` both assert `graph request count = 1` |
| NFR-3 | Composed calendar description below 4 000 characters | PASS | `TestDescriptionLengthBounded` PASS (`internal/tools/description_quality_test.go:147`, `maxLen = 4000`) |
| NFR-4 | Cold-start schema reduction ≥ 60 % | PASS | `TestColdStartSchemaSize_Reduction` PASS: 20 152 bytes, **72 %** against a 60 % floor (`internal/server/schema_size_test.go:82-84`) |
| NFR-5 | Every error carries a fix instruction naming what to supply or correct, and reaches the tool result; a failure path that logs carries the same report | **PASS** (was PARTIAL, fixed) | Handler-authored refusals carry corrections and are graded by `TestSchedulingReadsRefusalsCarryFixInstruction`. The three delegated paths GAP-1 named are now wrapped verb-locally: the timeout refusal appends `findMeetingTimesTimeoutFix` / `getScheduleTimeoutFix` to `graph.TimeoutErrorMessage` and emits the same text on the log record's `fix` attribute (`internal/tools/find_meeting_times.go:204-209`, `internal/tools/get_schedule.go:156-161`); the Graph-failure refusal appends `findMeetingTimesGraphFix` / `getScheduleGraphFix` to `graph.RedactGraphError` and to the log record (`find_meeting_times.go:211-216`, `get_schedule.go:163-168`); the bare `validate.ValidateEmail` refusal is wrapped as `schedules: %w: <correction>` (`get_schedule.go:249`). Assertions: `TestSchedulingReadsHonourTimeoutAndRedaction` now checks the correction on the tool result **and** in the captured log record for both verbs on both paths; `TestGetSchedule_RejectsInvalidAddress` checks the parameter name and the correction. The shared helpers themselves are unchanged, by design — see the resolution note under GAP-1 |
| NFR-6 | No third-party dependency added | PASS | `go.mod` and `go.sum` absent from `git diff caebec3...HEAD --name-only`; `make tidy` clean inside `make ci` |
| NFR-7 | Deterministic projection in all three tiers, including ordering | PASS | `internal/tools/get_schedule.go:156-174` (preserves the requested order, no sort); `internal/tools/find_meeting_times.go:205-215` (preserves Graph's rank order); `TestSchedulingFormattersDeterministic` (2 verbs × 3 modes = 6 sub-tests, byte-comparison), `TestFindMeetingTimes_FormatterDeterministic` |

## Acceptance Criteria Verification

| AC # | Description | Status | Evidence |
|---|---|---|---|
| AC-1 | Both verbs register in every configuration; four tools; intro unchanged | PASS | `TestRegisterTools_CalendarSchedulingReads` (default + maximal, enum + tool count); `internal/server/server.go:88` unchanged and absent from the diff |
| AC-2 | Suggests slots from attendee constraints; missing attendees refused pre-call | PASS | `TestFindMeetingTimes_Success` (one POST, both attendees on the wire, `Confidence 100%` in text); `TestFindMeetingTimes_RequiresAttendees` (`calls == 0`, error names `attendees`) |
| AC-3 | Inputs validated before calling Graph (email, duration, half-open window, malformed bound, numeric bounds) | PASS | `TestFindMeetingTimes_RejectsInvalidAttendeeEmail`, `_RejectsUnparseableDuration`, `_RejectsHalfOpenWindow`, `_RejectsInvalidDatetime`, `_RejectsOutOfBoundNumerics` — each asserts `graph request count = 0` |
| AC-4 | Cross-mailbox free/busy and working hours in one request | PASS | `TestGetSchedule_Success` (one POST carrying both addresses and `2026-03-12T00:00:00Z`; text attributes blocks to each mailbox); `TestGetSchedule_ReturnsWorkingHours` (`Working hours:`, `08:00:00`, `monday`) |
| AC-5 | Per-mailbox error reported, other mailbox still reported | PASS | `TestGetSchedule_PerMailboxErrorSurfaced` (call succeeds; `Access is denied.`, `ErrorAccessDenied`, `denied@example.com` present; the succeeding mailbox keeps `busy`) |
| AC-6 | Mailboxes required, within the ceiling, and a resolvable window; interval bounds | PASS | `TestGetSchedule_RequiresSchedules`, `_RequiresWindow`, `_EnforcesMailboxCeiling` (21 addresses), `_RejectsOutOfBoundInterval` (4 and 1441) — each asserts `calls == 0` |
| AC-7 | Read hints, no write/send/delete parameter, not behind the read-only guard, `calendar.<verb>` identity | PASS | `TestSchedulingReadVerbAnnotations`; schema inspection at `internal/server/calendar_verbs.go:839-877`, `:901-930`; `TestSchedulingReadsRegisteredAndReadOnly` (built with `readOnly: true`, no refusal); `TestSchedulingReadsCarryDotIdentity` (audit log carries both identities, `operation_type read`) |
| AC-8 | `get_free_busy` unchanged | PASS | Neither `internal/tools/get_free_busy.go` nor its test file appears in the diff; 12 `TestGetFreeBusy_*` tests PASS unmodified; `TestAggregateAnnotations_Calendar` PASS |
| AC-9 | Consent surface unchanged; no send scope | PASS | No `internal/auth/` file in the diff; `TestScopes_CalendarOnly`, `_WithMail`, `_MailManage`, `_MailManageImpliesRead`, `_NoMailSend` PASS |
| AC-10 | Three tiers honoured; summary from a dedicated serializer; raw carries a field summary omits; payloads differ; no write confirmation; determinism | PASS | `TestFindMeetingTimes_SummaryAndRawTiersDiffer`, `TestGetSchedule_SummaryAndRawTiersDiffer`, `TestSuggestionTiersDifferInFieldSet`, `TestSerializeScheduleTiersDifferInFieldSet`, `TestSerializeSummaryScheduleInformationOmitsAbsentWorkingHours`, `TestSerializeSummaryMeetingTimeSuggestionOmitsAbsentReason`, `TestSchedulingFormattersDeterministic`, `TestWriteVerbsDeclareNoOutputParameter` |
| AC-11 | Surface manifest regenerated; calendar 17/17; totals 49/35 | PASS | `site/src/generated/surface.json` diff shows exactly those figures; `make ci` `surface-check` left the tree clean (`git status --porcelain` empty); `TestCommittedManifestMatchesRecord`, `TestRecordCountsMatchBuiltVerbs`, `TestDefaultCountExcludesGatedVerbs`, `TestEveryVerbCarriesSummaryAndGate` PASS |
| AC-12 | Verb inventory golden delta is exactly two added lines | PASS | `internal/tools/dispatch_registry_test.go` numstat `2 0` — two insertions, zero deletions; `TestVerbInventoryUnchangedAfterUpgrade` PASS |
| AC-13 | Extension manifest names both verbs; four tools | PASS | `extension/manifest.json` numstat `1 1` (description line only); `TestManifestDescribesEveryRegisteredVerb` PASS (derives its cases from the registry) |
| AC-14 | Timeout named on both channels; Graph error redacted **and carrying a fix instruction** | **PASS** (was PARTIAL, fixed) | Timeout half as before, plus the correction on both channels: `TestSchedulingReadsHonourTimeoutAndRedaction` asserts `"timed out"`, `timeout_seconds`, the literal `7s`, and now the verb's timeout correction on the tool result and in the captured log record. Redaction half: the same test asserts `[email redacted]`, absence of the leaked address, retention of `ErrorAccessDenied`, and now the verb's Graph correction on the tool result and in the log record. The clause is implemented (`find_meeting_times.go:211-216`, `get_schedule.go:163-168`) and asserted for both verbs |
| AC-15 | File and helper conventions; one request per verb; no new dependency | PASS | See NFR-1, NFR-2, NFR-6 |
| AC-16 | Registry metadata complete per verb | PASS | `TestEveryVerbHasSummary` (80-char bound), `TestEveryVerbHasDescription`, `TestEveryVerbHasClassification`, `TestSeeDocsAnchorsResolve`, `TestEveryVerbStatesRequiredParameters`, `TestEveryParameterHasDescription`, `TestDescriptionsListVerbsOnSeparateLines` all PASS; preference guidance at `internal/server/calendar_verbs.go:826`, `:888` |
| AC-17 | Measured surface gates keep their margins | PASS | `TestDescriptionLengthBounded` PASS; `TestColdStartSchemaSize_Reduction` PASS at 72 % ≥ 60 % |
| AC-18 | Steps 42 and 43 exist as reads, no cleanup, no skip range, nothing renumbered | PASS | `docs/prompts/mcp-tool-crud-test.md` numstat `22 0` — a pure insertion, so no pre-existing step moved; Step 42 (`find_meeting_times`) and Step 43 (`get_schedule`) each carry Verify/Fail bullets and no Cleanup bullet; neither appears in the file's skip ranges |

## Test Strategy Verification

All 30 tests the CR specifies exist and match their specified behaviour. Runner:
`go test -race ./internal/tools/ ./internal/server/ ./internal/graph/ ./internal/surface/`,
all PASS.

| Test File | Test Name | Specified | Exists | Matches Spec |
|---|---|---|---|---|
| `internal/tools/find_meeting_times_test.go` | `TestFindMeetingTimes_Success` | yes | yes (`:102`) | yes — PASS |
| `internal/tools/find_meeting_times_test.go` | `TestFindMeetingTimes_RequiresAttendees` | yes | yes (`:128`) | yes — PASS |
| `internal/tools/find_meeting_times_test.go` | `TestFindMeetingTimes_RejectsInvalidAttendeeEmail` | yes | yes (`:144`) | yes — PASS |
| `internal/tools/find_meeting_times_test.go` | `TestFindMeetingTimes_RejectsUnparseableDuration` | yes | yes (`:163`) | yes — PASS |
| `internal/tools/find_meeting_times_test.go` | `TestFindMeetingTimes_RejectsInvalidDatetime` | yes | yes (`:183`) | yes — PASS |
| `internal/tools/find_meeting_times_test.go` | `TestFindMeetingTimes_RejectsHalfOpenWindow` | yes | yes (`:203`) | yes — PASS |
| `internal/tools/find_meeting_times_test.go` | `TestFindMeetingTimes_OmitsTimeConstraintWhenNoBounds` | yes | yes (`:223`) | yes — PASS |
| `internal/tools/find_meeting_times_test.go` | `TestFindMeetingTimes_AppliesDurationAndCandidateDefaults` | yes | yes (`:238`) | yes — PASS |
| `internal/tools/find_meeting_times_test.go` | `TestFindMeetingTimes_RejectsOutOfBoundNumerics` | yes | yes (`:288`) | yes — PASS |
| `internal/tools/find_meeting_times_test.go` | `TestFindMeetingTimes_EmptySuggestionsReasonSurfaced` | yes | yes (`:324`) | yes — PASS |
| `internal/tools/find_meeting_times_test.go` | `TestFindMeetingTimes_SummaryAndRawTiersDiffer` | yes | yes (`:341`) | yes — PASS |
| `internal/tools/find_meeting_times_test.go` | `TestFindMeetingTimes_OmitsUnsuppliedOptionalsFromBody` | no (added) | yes (`:259`) | additional FR-25 coverage — PASS |
| `internal/tools/find_meeting_times_test.go` | `TestFindMeetingTimes_SuppliedOptionalsReachBody` | no (added) | yes (`:271`) | additional FR-25 coverage — PASS |
| `internal/tools/find_meeting_times_test.go` | `TestFindMeetingTimes_NoClientInContext` | no (added) | yes (`:390`) | additional — PASS |
| `internal/tools/find_meeting_times_test.go` | `TestFindMeetingTimes_FormatterDeterministic` | no (added) | yes (`:406`) | additional NFR-7 coverage — PASS |
| `internal/tools/get_schedule_test.go` | `TestGetSchedule_Success` | yes | yes (`:133`) | yes — PASS |
| `internal/tools/get_schedule_test.go` | `TestGetSchedule_ReturnsWorkingHours` | yes | yes (`:165`) | yes — PASS |
| `internal/tools/get_schedule_test.go` | `TestGetSchedule_RequiresSchedules` | yes | yes (`:181`) | yes — PASS |
| `internal/tools/get_schedule_test.go` | `TestGetSchedule_RejectsInvalidAddress` | yes | yes (`:197`) | yes — PASS |
| `internal/tools/get_schedule_test.go` | `TestGetSchedule_RequiresWindow` | yes | yes (`:215`) | yes — PASS |
| `internal/tools/get_schedule_test.go` | `TestGetSchedule_EnforcesMailboxCeiling` | yes | yes (`:234`) | yes — PASS |
| `internal/tools/get_schedule_test.go` | `TestGetSchedule_RejectsOutOfBoundInterval` | yes | yes (`:257`) | yes — PASS |
| `internal/tools/get_schedule_test.go` | `TestGetSchedule_AppliesIntervalDefault` | yes | yes (`:289`) | yes — PASS |
| `internal/tools/get_schedule_test.go` | `TestGetSchedule_PerMailboxErrorSurfaced` | yes | yes (`:306`) | yes — PASS |
| `internal/tools/get_schedule_test.go` | `TestGetSchedule_DateShorthandResolvesWindow` | yes | yes (`:329`) | yes — PASS |
| `internal/tools/get_schedule_test.go` | `TestGetSchedule_ExplicitDatetimesBeatDateShorthand` | yes | yes (`:353`) | yes — PASS |
| `internal/tools/get_schedule_test.go` | `TestGetSchedule_SummaryAndRawTiersDiffer` | yes | yes (`:386`) | yes — PASS |
| `internal/tools/get_schedule_test.go` | `TestGetSchedule_RejectsInvalidDatetime` | no (added) | yes (`:366`) | additional FR-8 coverage — PASS |
| `internal/tools/get_schedule_test.go` | `TestGetSchedule_NoClientInContext` | no (added) | yes (`:425`) | additional — PASS |
| `internal/graph/scheduling_serialize_test.go` | `TestSerializeScheduleTiersDifferInFieldSet` | yes | yes (`:234`) | yes — PASS |
| `internal/graph/scheduling_serialize_test.go` | `TestSerializeMeetingTimeSuggestionRawKeepsEveryKey` | no (added) | yes (`:66`) | additional FR-12 raw coverage — PASS |
| `internal/graph/scheduling_serialize_test.go` | `TestSuggestionTiersDifferInFieldSet` | no (added; the CR folded suggestions into the schedule row) | yes (`:90`) | covers the suggestion half of the specified row — PASS |
| `internal/graph/scheduling_serialize_test.go` | `TestSerializeSummaryMeetingTimeSuggestionFlattensSlot` | no (added) | yes (`:122`) | additional — PASS |
| `internal/graph/scheduling_serialize_test.go` | `TestSerializeSummaryMeetingTimeSuggestionOmitsAbsentReason` | no (added) | yes (`:140`) | additional — PASS |
| `internal/graph/scheduling_serialize_test.go` | `TestSerializeScheduleInformationRawKeepsEveryKey` | no (added) | yes (`:211`) | additional — PASS |
| `internal/graph/scheduling_serialize_test.go` | `TestSerializeSummaryScheduleInformationSurfacesError` | no (added) | yes (`:275`) | additional FR-10 coverage — PASS |
| `internal/graph/scheduling_serialize_test.go` | `TestSerializeSummaryScheduleInformationFlattensItems` | no (added) | yes (`:293`) | additional — PASS |
| `internal/graph/scheduling_serialize_test.go` | `TestSerializeSummaryScheduleInformationOmitsAbsentWorkingHours` | no (added) | yes (`:323`) | additional — PASS |
| `internal/tools/scheduling_read_verbs_test.go` | `TestSchedulingReadsHonourTimeoutAndRedaction` | yes | yes (`:100`) | yes — PASS. Timeout and redaction each asserted on the tool result and in the captured log record, and the "carries a fix instruction" clause now asserted on both channels for both verbs (GAP-1 fix) |
| `internal/tools/scheduling_read_verbs_test.go` | `TestSchedulingFormattersDeterministic` | yes | yes (`:267`) | yes — PASS |
| `internal/tools/scheduling_read_verbs_test.go` | `TestSchedulingReadsRefusalsCarryFixInstruction` | no (added) | yes (`:215`) | NFR-5 coverage for the missing-required-parameter refusal; the delegated refusals are covered by the row above — PASS |
| `internal/tools/tool_annotations_test.go` | `TestSchedulingReadVerbAnnotations` | yes | yes (`:581`) | yes — PASS |
| `internal/server/calendar_verbs_test.go` | `TestSchedulingReadsRegisteredAndReadOnly` | yes | yes (`:85`) | yes — PASS |
| `internal/server/calendar_verbs_test.go` | `TestSchedulingReadsCarryDotIdentity` | yes | yes (`:124`) | yes — PASS |
| `internal/server/server_test.go` | `TestRegisterTools_CalendarSchedulingReads` | yes | yes (`:1019`) | yes — PASS |

Tests to Modify:

| Test File | Test Name | Specified Change | Applied | Result |
|---|---|---|---|---|
| `internal/tools/dispatch_registry_test.go` | `TestVerbInventoryUnchangedAfterUpgrade` over `verbInventoryGolden` | +2 identities | yes (`+2/−0`) | PASS |
| `internal/tools/tool_annotations_test.go` | `TestAggregateAnnotations_Calendar` | unchanged expectations, re-asserted | yes (untouched) | PASS |
| `docs/prompts/mcp-tool-crud-test.md` | lifecycle steps | +Steps 42, 43 | yes (`+22/−0`) | n/a (prompt) |

## Diff Coverage

| File | +/- | Mapped Requirements |
|---|---|---|
| `internal/tools/find_meeting_times.go` | +312/−0 | FR-3, FR-4, FR-5, FR-6, FR-11, FR-12, FR-15, FR-25, NFR-1, NFR-2, NFR-5, NFR-7 |
| `internal/tools/get_schedule.go` | +291/−0 | FR-7, FR-8, FR-9, FR-10, FR-11, FR-12, FR-15, FR-25, NFR-1, NFR-2, NFR-5, NFR-7 |
| `internal/graph/scheduling_serialize.go` | +323/−0 | FR-9, FR-10, FR-12, NFR-1, NFR-7 |
| `internal/tools/text_format.go` | +178/−0 | FR-9, FR-10, FR-12, NFR-1, NFR-7 |
| `internal/server/calendar_verbs.go` | +123/−3 | FR-1, FR-2, FR-13, FR-14, FR-17, FR-18, FR-25 |
| `extension/manifest.json` | +1/−1 | FR-20, AC-13 |
| `site/src/generated/surface.json` | +16/−4 | FR-21, AC-11 |
| `docs/troubleshooting.md` | +18/−0 | FR-10 disclosure (Affected Components: one troubleshooting entry with a stable anchor `{#schedule-mailbox-error}`) |
| `docs/prompts/mcp-tool-crud-test.md` | +22/−0 | FR-22, AC-18 |
| `internal/tools/dispatch_registry_test.go` | +2/−0 | FR-23, AC-12 |
| `internal/tools/find_meeting_times_test.go` | +415/−0 | Test Strategy (find_meeting_times rows) |
| `internal/tools/get_schedule_test.go` | +437/−0 | Test Strategy (get_schedule rows) |
| `internal/graph/scheduling_serialize_test.go` | +329/−0 | Test Strategy (serializer rows) |
| `internal/tools/scheduling_read_verbs_test.go` | +273/−0 | Test Strategy (cross-verb rows), AC-14, NFR-5, NFR-7 |
| `internal/server/calendar_verbs_test.go` | +177/−0 | AC-7, FR-14 |
| `internal/server/server_test.go` | +49/−0 | AC-1, FR-1, FR-2 |
| `internal/tools/tool_annotations_test.go` | +65/−0 | FR-13, AC-7 |
| `docs/cr/CR-0080-calendar-scheduling-reads.md` | +633/−203 | The CR itself (review edits at `8b9b481`, finalization frontmatter at `a53786c`) |
| `docs/backlog/cr-0078-0083.md` | +201/−2 | Governance decision ledger for CR-0080 |
| `.agents/scenarios/2026-09-02-calendar-scheduling-reads-live-mailbox.md` | new | GAP-2 fix: the derived, not-yet-run live-mailbox scenario the CR's Verification Commands require |

### Unmapped changed files

* `docs/backlog/cr-0078-0083.md` — **justified.** Not listed in Affected Components, but the
  CR's Open Questions and Review Summary both cite it by name as the home for the settled
  decisions ("recorded with their grounds under `## CR-0080` in
  `docs/backlog/cr-0078-0083.md`"), and the change is confined to the `## CR-0080` section
  (`+201/−2`, replacing the placeholder "Not yet reviewed"). Governance record, no source
  impact.
* `.agents/scenarios/2026-09-02-calendar-scheduling-reads-live-mailbox.md` — **justified.**
  Added by the gap-fix pass. Not in Affected Components, but the CR's Verification Commands
  paragraph names `.agents/scenarios/` as the required home for it. Scenario artifact, no
  source impact.
* `docs/cr/CR-0080-calendar-scheduling-reads.md` — **justified.** The governed document
  itself, edited by the review commit and the finalization commit.

No stray source file changed. Every file the CR named as requiring no change is untouched:
`internal/validate/validate.go`, `internal/server/server.go`, `docs/concepts.md`,
`scripts/crud-test.sh`, `docs/bench/crud-runs.csv`, `README.md`, `docs/readme.md`,
`llms.txt`, `internal/docs/llmstxt.go`, `internal/config/inventory.go`,
`internal/tools/get_free_busy.go`, `go.mod`, `go.sum`.

## Gaps

Both gaps are closed. The original findings are kept below, each followed by its
resolution, so the record states what was wrong as well as what was done.

### GAP-1 — FIXED — NFR-5 and AC-14: the shared error helpers carry no fix instruction

**Requirement ref:** NFR-5; AC-14, second `Given` ("the returned error text is redacted by
the shared helper **and carries a fix instruction**").

**What was missing.** NFR-5 requires *every* error either verb raises to name what to
supply or correct. Three refusal paths did not:

1. `graph.TimeoutErrorMessage` (`internal/graph/timeout.go:55`) returns
   `"request timed out after 7s"` — the deadline and no correction. Reached from both
   handlers.
2. `graph.RedactGraphError` (`internal/graph/errors.go:131-134`) returns Graph's own
   diagnosis, redacted, with no correction appended. This is exactly the text AC-14's
   second `Given` grades.
3. `validate.ValidateEmail` (`internal/validate/validate.go:95-101`) returns
   `invalid email address: "not-an-email"`. On the `get_schedule` path it was returned
   bare, so the refusal named neither the `schedules` parameter nor a correction.

No test asserted a fix instruction on any of the three.

**Resolution — the narrowest fix, applied verb-locally.** The shared helpers in
`internal/graph` are **unchanged**, deliberately: every verb in the repository routes
through them, so changing their text rewrites the whole error surface and moves assertions
in tests this CR does not own. Instead each handler appends its own correction to the
helper's output and emits the same correction on its log record, so the deadline, the Graph
code, and the offending address all survive alongside an instruction to act on.

| Path | Correction source | Result channel | Log channel |
|---|---|---|---|
| timeout, `find_meeting_times` | `findMeetingTimesTimeoutFix` (`internal/tools/find_meeting_times.go:43`) | `find_meeting_times.go:208-209` | `:204-207` (`fix` attribute) |
| timeout, `get_schedule` | `getScheduleTimeoutFix` (`internal/tools/get_schedule.go:43`) | `get_schedule.go:160-161` | `:156-159` (`fix` attribute) |
| redacted Graph error, `find_meeting_times` | `findMeetingTimesGraphFix` (`find_meeting_times.go:44`) | `find_meeting_times.go:215-216` | `:211-214` (`fix` attribute) |
| redacted Graph error, `get_schedule` | `getScheduleGraphFix` (`get_schedule.go:44`) | `get_schedule.go:167-168` | `:163-166` (`fix` attribute) |
| invalid `schedules` address | `getScheduleAddressFix` (`get_schedule.go:45`) | `get_schedule.go:249` (`schedules: %w: <correction>`) | no log on this path; the refusal is raised before any request, so there is no second channel to carry |

**Evidence.** `TestSchedulingReadsHonourTimeoutAndRedaction`
(`internal/tools/scheduling_read_verbs_test.go`) carries the correction per verb on the
`schedulingReadVerb` fixture and asserts it on the tool result *and* in the captured log
record, in both the timeout sub-case and the redaction sub-case, for both verbs — four
result assertions and four log assertions added. `TestGetSchedule_RejectsInvalidAddress`
asserts the refusal names `schedules` and states the correction. All PASS under `-race`.

**Recorded, not done.** That `graph.TimeoutErrorMessage` and `graph.RedactGraphError` never
carry a fix instruction, and that lifting this pattern into them is a separate change, is
written under `## CR-0080` in `docs/backlog/cr-0078-0083.md` with its grounds.

### GAP-2 — RESOLVED (scenario persisted; run pending for the user)

**Requirement ref:** CR "Verification Commands" closing paragraph, and the project's
scenario rule.

**What was missing.** No scenario for this change existed under `.agents/scenarios/`.

**Resolution.** `.agents/scenarios/2026-09-02-calendar-scheduling-reads-live-mailbox.md` is
written in the format the two existing scenario files use, derived from the CR's
Verification Commands paragraph: nine steps at the `calendar` aggregate tool over stdio
against the built binary, covering `find_meeting_times` bounded and unbounded, the raw-tier
comparison, `get_schedule` across two mailboxes, the per-mailbox error path against a
mailbox the account may not read, the window, ceiling, and malformed-address refusals, and
the `get_free_busy` regression check. It carries `outcome: not-run` and
`runs: "0 of 0 attempted"`, and states plainly that nothing in it has been observed.

**Still pending for the user, and not performable here.** The run itself needs a live
authenticated mailbox, a second readable mailbox in the same tenant, and one address the
account may not read. `make crud-test` (Steps 42 and 43) is a paid harness and was excluded
from this run by the orchestrator. Both are recorded as pending under `## CR-0080` in
`docs/backlog/cr-0078-0083.md`. Before running the harness, rebuild the binary it drives and
confirm the report's own `Server version` line matches `git rev-parse --short HEAD`, per the
project rule.
