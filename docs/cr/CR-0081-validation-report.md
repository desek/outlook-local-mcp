# CR-0081 Validation Report

Validated at `a88c3d1` on branch `docs/cr-implementation-set-0079-0083`.
The CR-0081 diff is `a690a94...HEAD` (implementation commits `6d59026`, `38e148e`,
`7344b0d`, `f7b67d0`; CR frontmatter closed at `a88c3d1`).

Gap-fix pass applied after `ae1bfeb`: the three naming-only PARTIAL rows are resolved by
amending the CR's Test Strategy to the test names and split that exist, and GAP-2's scenario
artefact is written. GAP-1 stays open by design, and both live runs are pending for the user.

## Summary

Requirements: 33/33 | Acceptance Criteria: 11/11 | Tests: 24/24 | Gaps: 1 open (GAP-1)

Functional requirements 26/26, non-functional requirements 7/7. Every requirement and
every acceptance criterion maps to at least one changed file with a specific hunk, and
every observable behaviour maps to a named test that was executed and passed. `make ci`
exits 0 and leaves the working tree clean. Both gaps raised were un-executed live-mailbox
verification the CR's own Test Strategy requires for acceptance, deferred to the user.
GAP-2's artefact has since been written and the scenario persisted, unrun; GAP-1, the paid
harness, remains open.

### Check pipeline

| Command | Result | Note |
|---|---|---|
| `make ci` | PASS (exit 0) | build, vet, tidy, `golangci-lint` 0 issues, full test suite, docs bundle, surface manifest regeneration, `goreleaser check`, `mcpb validate` |
| `git status --porcelain` after `make ci` | clean | the surface drift check regenerated `site/src/generated/surface.json` and `llms.txt` without moving the tree (AC-7) |
| CR Verification Commands, targeted `-race` run | PASS, 24/24 | see Test Strategy Verification |
| Gating suite (`verb_metadata`, `description_quality`, `schema_size`, auth scopes) | PASS, 15/15 | see Test Strategy Verification |
| `make crud-test` | NOT RUN | paid harness, excluded by the validation brief; recorded as GAP-1 |

### Measured instrument readings

Taken rather than assumed, because NFR-2 and NFR-3 are budget requirements and a budget
without a reading is not verified.

| Measurement | Reading | Bound | Source |
|---|---|---|---|
| Composed `calendar` tool description | 2736 characters | < 4000 | `tools/list` against the built binary; `TestDescriptionLengthBounded` passes |
| Cold-start schema reduction | 71% | >= 60% | `schema_size_test.go:84` log line |
| Registered top-level tools | 4 | exactly 4 | `tools/list` against the built binary; `manifest_sync_test.go:87` |
| Folded `calendar` annotation | `readOnlyHint=false, destructiveHint=true, idempotentHint=false, openWorldHint=true` | unchanged | `tools/list`; `TestAggregateAnnotations_Calendar` passes |
| `calendar` operation enum | 20 verbs, ending `list_event_attachments, get_event_attachment, add_event_attachment` | 17 + 3 | `tools/list` |
| Surface manifest calendar counts | `fullCount` 20, `defaultCount` 20; totals 52 / 38 | 17 -> 20; 49/35 -> 52/38 | `site/src/generated/surface.json:127-128,345-346` |

## Requirement Verification

### Functional Requirements

| Req # | Description | Status | Evidence (file:line / test name) |
|---|---|---|---|
| FR-1 | Exactly three new calendar verbs; no new top-level tool; count stays four | PASS | `internal/server/calendar_verbs.go:125-127`; `tools/list` returns 4 tools and a 20-entry calendar enum containing all three; `TestCalendarRegistersAttachmentVerbs`, `TestManifestDescribesEveryRegisteredVerb` |
| FR-2 | Registered in every configuration; reads on `wrap`, write on `wrapWrite` behind `ReadOnlyGuard` | PASS | `internal/server/calendar_verbs.go:125-127`, `:998` (`wrap(...,"read",...)`), `:1044` (`wrapWrite(...,"write",...)`); `TestCalendarRegistersAttachmentVerbs` (built with no mail configuration), `TestReadOnlyBlocksAddEventAttachment` |
| FR-3 | `list_event_attachments` requires `event_id`, `$select` from the existing `listAttachmentsSelectFields`, no content bytes | PASS | `internal/tools/list_event_attachments.go:75-85`, `:92-95` (`Select: listAttachmentsSelectFields`); `TestListEventAttachments_NoContentBytesFetched` asserts every shared field is on the wire and `contentBytes` is not |
| FR-4 | `get_event_attachment` requires both identifiers, issues the item GET, returns metadata plus base64 content | PASS | `internal/tools/get_event_attachment.go:72-90`, `:100`; `TestGetEventAttachment_ReturnsContent` (asserts `contentBytes == "aGVsbG8="`) |
| FR-5 | `get_event_attachment` enforces `MaxAttachmentSizeBytes`, refusal names the limit and `OUTLOOK_MCP_MAX_ATTACHMENT_SIZE_BYTES` | PASS | `internal/tools/get_event_attachment.go:123-130`; ceiling plumbed at `internal/server/server.go:85` and `internal/server/introspect_verbs.go:73`; `TestGetEventAttachment_RefusesOverSizeCeiling` |
| FR-6 | `add_event_attachment` requires `event_id`, `name`, `content_bytes`; decodes with `validate.ValidateBase64` before any Graph request | PASS | `internal/tools/add_event_attachment.go:85-114`; `TestAddEventAttachment_RejectsBadInputBeforeAnyRequest/invalid_base64` (asserts 0 GET, 0 POST) |
| FR-7 | Direct POST strictly below `inlineAttachmentThresholdBytes`, upload session at or above; no second constant, no path parameter | PASS | `internal/tools/add_event_attachment.go:137` (`size >= inlineAttachmentThresholdBytes`), `:176-189` (direct `FileAttachment` POST), `:140-143` (session); the constant is defined once, at `internal/tools/add_attachment.go:38`; the schema publishes no path parameter (`internal/server/calendar_verbs.go:1052-1073`); `TestAddEventAttachment_DirectUploadPath`, `TestAddEventAttachment_UploadSessionPath` |
| FR-8 | Decoded-size ceiling equal to `MaxAttachmentSizeBytes`, refused naming the limit and the variable, before any Graph request | PASS | `internal/tools/add_event_attachment.go:119-125`; `TestAddEventAttachment_RefusesOverSizeCeiling` (asserts the text names 16, 8 and the variable, and that 0 GET / 0 POST were issued) |
| FR-9 | Subject fetch with `$select=subject` before any bytes move; no further metadata request on the success path | PASS | `internal/tools/add_event_attachment.go:132-135` and `fetchEventSubject` `:232-272` with `Select: []string{"subject"}` at `:245`; `TestAddEventAttachment_SubjectFetchPrecedesTransfer` (404 event -> 0 POST, 0 sessions, 0 chunks), `TestAddEventAttachment_DirectUploadPath` (`gets == 1`), `TestAddEventAttachment_UploadSessionPath` (`gets == 1`) |
| FR-10 | Confirmation names subject, event id, attachment name, decoded size, response attachment id, and transfer path; id never echoed | PASS | `internal/tools/attachment_confirmation.go:96-109`; called at `internal/tools/add_event_attachment.go:169` and `:214`; id sourced from `created.GetId()` at `:207` and from the session Location at `:143`; `TestAddEventAttachment_ConfirmationUsesGraphResponse`, `TestFormatEventAttachmentConfirmation_NamesSubjectNameSizeIDAndPath` |
| FR-11 | All three validate `event_id` with `ValidateResourceID`; `get_event_attachment` also validates `attachment_id` | PASS | `internal/tools/list_event_attachments.go:81`, `internal/tools/get_event_attachment.go:77` and `:87`, `internal/tools/add_event_attachment.go:90`; `TestListEventAttachments_InvalidEventIDRejectedBeforeCall`, `TestGetEventAttachment_RejectsInvalidEventIDBeforeCall`, `TestGetEventAttachment_RequiresAttachmentID` |
| FR-12 | `name` validated against `validate.MaxAttachmentNameLen`; no new bound | PASS | `internal/tools/add_event_attachment.go:100` (the only length call, referencing the shared constant); `TestAddEventAttachment_RejectsBadInputBeforeAnyRequest/over-length_name` asserts the text names `name` and `255` |
| FR-13 | Reads implement three tiers via `output` and the shared serialisers; write returns text and declares no `output` | PASS | reads: `internal/tools/list_event_attachments.go:70`, `:126-135`; `internal/tools/get_event_attachment.go:67`, `:132-141`. Write schema has no `output` (`internal/server/calendar_verbs.go:1052-1073`); `TestWriteVerbsDeclareNoOutputParameter`, `TestEventAttachmentReadsDeterministic` (exercises all three tiers on both reads) |
| FR-14 | Reads expose reads only; the item builder's `Delete` is not wired; GET is the only method | PASS | no `Delete` call exists on the event attachment path (repo-wide grep of `Attachments().ByAttachmentId` finds no delete); `TestEventAttachmentReadsIssueOnlyGet` (`internal/tools/event_attachment_readonly_test.go:28`, fails on any non-GET), `TestEventAttachmentReadSchemasDeclareNoMutation` (allowlists exactly `event_id`, `attachment_id`, `account`, `output`) |
| FR-15 | All four hints declared explicitly per the matrix | PASS | `internal/server/calendar_verbs.go:957-962`, `:1000-1005`, `:1046-1051`; `TestCalendarAttachmentVerbAnnotations` (`internal/tools/tool_annotations_test.go:646`); golden lines `internal/tools/dispatch_registry_test.go:47,54,59` |
| FR-16 | Wrapped under `calendar.<verb>`; reads audit `read`, write audits `write` | PASS | `internal/server/calendar_verbs.go:998`, `:1044`, and the list verb's `wrap("calendar.list_event_attachments","read",...)`; `TestAttachmentVerbsCarryDotIdentity` reads the emitted audit log and asserts both the identity and the `operation_type` |
| FR-17 | `RetryGraphCall` inside `WithTimeout`; `IsTimeoutError` / `TimeoutErrorMessage` / `RedactGraphError` / `FormatGraphError` | PASS | `internal/tools/list_event_attachments.go:97-121`; `internal/tools/get_event_attachment.go:94-118`; `internal/tools/add_event_attachment.go:181-205`, `:240-270`; `internal/tools/attachment_upload_session.go:180-192`; `TestEventAttachmentVerbsHonourTimeoutAndRedaction` (all three handlers, both a stall and a token-bearing Graph failure) |
| FR-18 | Every error carries a fix instruction reaching both the tool result and the log record | PASS | fix constants `internal/tools/list_event_attachments.go:38-42`, `internal/tools/get_event_attachment.go:33-38`, `internal/tools/add_event_attachment.go:45-52`, appended at every refusal site; `TestEventAttachmentErrorsReachBothChannels` asserts the same fix string in both channels for all three verbs |
| FR-19 | The pre-authenticated upload URL reaches neither channel; `redactUploadURL` reused | PASS | reuses `transferAttachmentChunks` (`internal/tools/attachment_upload_session.go:210`) and `redactUploadURL` (`:323`) unchanged; `internal/tools/add_event_attachment.go:143`; `TestAddEventAttachment_UploadURLNeverEscapes` checks both the result and the captured log for the URL and its `upload-secret` query token |
| FR-20 | Summary <= 80 chars, description with `Requires:` and annotation semantics, >= 1 example, >= 1 resolving `SeeDocs` (reads `concepts#output-tiers`, write `concepts#read-only-mode`) | PASS | `internal/server/calendar_verbs.go:951-956`, `:990-997`, `:1036-1043`; `TestEveryVerbHasSummary`, `TestEveryVerbHasDescription`, `TestEveryVerbHasClassification`, `TestSeeDocsAnchorsResolve`, `TestEveryVerbStatesRequiredParameters`, `TestEveryParameterHasDescription`, `TestDescriptionsListVerbsOnSeparateLines` |
| FR-21 | `get_event_attachment` description states the ceiling and the refusal above it | PASS | `internal/server/calendar_verbs.go:992` ("The full content is returned only within the server's attachment size limit (OUTLOOK_MCP_MAX_ATTACHMENT_SIZE_BYTES, 10 MB by default); a larger attachment is refused ... no partial content is returned") |
| FR-22 | Manifest calendar description names all three literally; `tools` array stays at four | PASS | `extension/manifest.json:57`; `TestManifestDescribesEveryRegisteredVerb` (word-boundary match per verb, plus `manifest_sync_test.go:87` asserting exactly 4 tools); `mcpb validate` passes in `make ci` |
| FR-23 | `site/src/generated/surface.json` regenerated with `make surface-manifest` and committed | PASS | `site/src/generated/surface.json:109-128,345-346`; `TestCommittedManifestMatchesRecord`; `make ci` regenerated the file and left `git status` clean |
| FR-24 | Harness prompt gains lifecycle steps from Step 44 covering add, list, and get | PASS (structural) | `docs/prompts/mcp-tool-crud-test.md:715` (Step 44, add), `:727` (Step 45, list), `:736` (Step 46, get plus cleanup delete). The steps exist and are well-formed; they have not been executed — see GAP-1 |
| FR-25 | Golden regenerated; delta exactly three added lines | PASS | `internal/tools/dispatch_registry_test.go:47,54,59`, inserted in sort order; the file's diff is `3 insertions, 0 deletions`, so no existing line changed; `TestVerbInventoryUnchangedAfterUpgrade` |
| FR-26 | OAuth scope set unchanged | PASS | no file under `internal/auth/` appears in the CR diff; `TestScopes_CalendarOnly`, `TestScopes_WithMail`, `TestScopes_MailManage`, `TestScopes_MailManageImpliesRead`, `TestScopes_NoMailSend` all pass |

### Non-Functional Requirements

| Req # | Description | Status | Evidence (file:line / test name) |
|---|---|---|---|
| NFR-1 | One handler file per verb under `internal/tools/`, each with exactly one `@agents-index` | PASS | `internal/tools/list_event_attachments.go:12`, `internal/tools/get_event_attachment.go:10`, `internal/tools/add_event_attachment.go:19`; `grep -c "@agents-index"` returns 1 for each new file and for both modified shared files |
| NFR-2 | Composed `calendar` description stays below 4000 characters | PASS | measured 2736 characters from `tools/list` against the built binary (2348 at `a690a94`, so the three inventory lines cost 388 and leave 1264 of headroom); `TestDescriptionLengthBounded` passes |
| NFR-3 | Cold-start schema reduction stays at or above 60% | PASS | measured 71% (21301 bytes over 4 tools), logged at `internal/server/schema_size_test.go:84`; `TestColdStartSchemaSize_Reduction` passes |
| NFR-4 | The two reads are deterministic at every tier | PASS | `TestEventAttachmentReadsDeterministic` invokes each read twice at `text`, `summary`, and `raw` and compares byte for byte; `TestListEventAttachments_Deterministic`, `TestGetEventAttachment_Deterministic` |
| NFR-5 | Exact request budget per path | PASS | reads: `TestListEventAttachments_Success` (`len(methods) != 1` fails), `TestGetEventAttachment_ReturnsContent` (`len(methods) != 1 \|\| methods[0] != GET` fails). Write direct: `TestAddEventAttachment_DirectUploadPath` asserts `gets == 1 && posts == 1`. Write chunked: `TestAddEventAttachment_UploadSessionPath` asserts `gets == 1`, `posts == 0`, `sessions == 1`, and that the chunks carried exactly the payload |
| NFR-6 | No third-party dependency added; `transferAttachmentChunks` reused; no second chunk transfer | PASS | `go.mod` and `go.sum` do not appear in the CR diff (0 files); `internal/tools/add_event_attachment.go:143` calls the shipped `transferAttachmentChunks` (`internal/tools/attachment_upload_session.go:210`), which the diff leaves untouched; the only addition to that file is `createEventAttachmentUploadSession` at `:162` |
| NFR-7 | No existing calendar or mail verb's behaviour, schema, annotations, or published text changed; folded calendar annotation unchanged | PASS | the `internal/server/calendar_verbs.go` diff is additive only (a doc-comment line, one config field at `:61-65`, three slice entries at `:125-127`, three constructors from `:944`); no mail source file appears in the CR diff; the live fold reads `readOnlyHint=false, destructiveHint=true, idempotentHint=false, openWorldHint=true`, identical to before; `TestAggregateAnnotations_Calendar` passes |

## Acceptance Criteria Verification

| AC # | Description | Status | Evidence |
|---|---|---|---|
| AC-1 | Three verbs register unconditionally; four tools; `calendar.<verb>` identity with the right audit operation; read-only blocks only the write | PASS | `TestCalendarRegistersAttachmentVerbs` (built with no mail configuration), `TestAttachmentVerbsCarryDotIdentity` (audit log carries `calendar.list_event_attachments`/`get_event_attachment` as `read` and `calendar.add_event_attachment` as `write`), `TestReadOnlyBlocksAddEventAttachment` (write refused with the identity in the text; both reads not blocked); four tools confirmed live via `tools/list` and by `manifest_sync_test.go:87` |
| AC-2 | Collection read enumerates metadata without downloading content; deterministic; malformed id refused with no request | PASS | `TestListEventAttachments_Success`, `TestListEventAttachments_NoContentBytesFetched`, `TestListEventAttachments_Deterministic`, `TestEventAttachmentReadsDeterministic/list_event_attachments`, `TestListEventAttachments_InvalidEventIDRejectedBeforeCall`, `TestListEventAttachments_MissingEventIDRejected`. Live-mailbox leg of this criterion is unexecuted; see GAP-2 |
| AC-3 | Item read returns content, honours the ceiling, refuses a missing or malformed `attachment_id` with no request | PASS | `TestGetEventAttachment_ReturnsContent`, `TestGetEventAttachment_RefusesOverSizeCeiling`, `TestGetEventAttachment_RequiresAttachmentID`, `TestGetEventAttachment_RejectsInvalidEventIDBeforeCall`. Live-mailbox leg unexecuted; see GAP-2 |
| AC-4 | Small file: one subject GET then one POST, no further metadata, response id in the confirmation, full confirmation contract; over-length name refused with no request | PASS | `TestAddEventAttachment_DirectUploadPath` (asserts `gets==1`, `posts==1`, all six confirmation lines, `@odata.type` is `#microsoft.graph.fileAttachment`, and that `isInline` was not sent), `TestAddEventAttachment_ConfirmationUsesGraphResponse`, `TestAddEventAttachment_RejectsBadInputBeforeAnyRequest/over-length_name`. Live-mailbox leg unexecuted; see GAP-2 |
| AC-5 | Large file: one subject GET, one `createUploadSession`, then chunk PUTs; no further metadata; upload-session path named; a failing chunk leaks the URL to neither channel | PASS | `TestAddEventAttachment_UploadSessionPath` (asserts `gets==1`, `posts==0`, `sessions==1`, exact byte count, `Content-Range` on every chunk, and `Transfer: chunked upload session`), `TestAddEventAttachment_UploadURLNeverEscapes`. Live-mailbox leg unexecuted; see GAP-2 |
| AC-6 | Registry metadata complete for every new verb; reads declare `output`, the add declares none; the get description states the ceiling | PASS | `TestEveryVerbHasSummary`, `TestEveryVerbHasDescription`, `TestEveryVerbHasClassification`, `TestSeeDocsAnchorsResolve`, `TestEveryVerbStatesRequiredParameters`, `TestEveryParameterHasDescription`, `TestWriteVerbsDeclareNoOutputParameter`, `TestCalendarAttachmentVerbAnnotations`; `internal/server/calendar_verbs.go:992` for the ceiling sentence |
| AC-7 | Manifest, generated surface, golden, and harness all name the new verbs; surface drift check leaves the tree unmodified; calendar records 20/20; golden delta exactly three lines | PASS | `extension/manifest.json:57`; `site/src/generated/surface.json:109-128` and `:345-346`; `internal/tools/dispatch_registry_test.go:47,54,59` (3 insertions, 0 deletions); `docs/prompts/mcp-tool-crud-test.md:715,727,736`; `make ci` regenerated the manifest and `git status --porcelain` returned empty; `TestCommittedManifestMatchesRecord`, `TestManifestDescribesEveryRegisteredVerb`, `TestVerbInventoryUnchangedAfterUpgrade` |
| AC-8 | Consent surface unchanged | PASS | no `internal/auth/` file in the CR diff; `TestScopes_CalendarOnly`, `TestScopes_WithMail`, `TestScopes_MailManage`, `TestScopes_MailManageImpliesRead`, `TestScopes_NoMailSend` |
| AC-9 | No read verb exposes a mutation; the item delete builder is unwired; GET only | PASS | `TestEventAttachmentReadSchemasDeclareNoMutation` (schema half, allowlist of four parameters), `TestEventAttachmentReadsIssueOnlyGet` (handler half, fails on any non-GET); repo-wide grep confirms no delete is wired on the event attachment path |
| AC-10 | Errors carry their correction on both channels; bad input sends no request | PASS | `TestEventAttachmentErrorsReachBothChannels` (all three verbs, result and captured log), `TestAddEventAttachment_RejectsBadInputBeforeAnyRequest` (7 cases, each asserting `gets==0 && posts==0`), `TestAddEventAttachment_RefusesOverSizeCeiling`, `TestAddEventAttachment_UploadURLNeverEscapes` (asserts the fix instruction is present alongside the redaction) |
| AC-11 | Timeout names the bound; errors redacted and carry a fix; one file per handler with one index line; shipped chunk transfer reused; no new dependency; no existing verb changed | PASS | `TestEventAttachmentVerbsHonourTimeoutAndRedaction`; `@agents-index` count of 1 in each new and modified file; `internal/tools/attachment_upload_session.go:210` unchanged by the diff and called at `internal/tools/add_event_attachment.go:143`; `go.mod`/`go.sum` absent from the diff; `TestAggregateAnnotations_Calendar` and the live fold reading |

## Test Strategy Verification

All 24 CR-specified tests were executed under `-race`. All passed.

Three were originally raised as PARTIAL because they exist under a different symbol than the
CR named. The divergence was in naming and placement, never in coverage, and in both shapes
the shipped form is the better one: `TestReadVerbsWireNoDeleteOrMutation`'s schema half
cannot live in `internal/tools` without an import cycle, and the two bad-input cases grade
one invariant across a table of arguments. The CR's Test Strategy has been amended to the
names and split that exist, carrying the split reason, rather than shipped tests renamed to
match a document. The three rows are graded PASS against that amended text; the amendment is
recorded under `## CR-0081` in `docs/backlog/cr-0078-0083.md`.

| Test File | Test Name | Specified | Exists | Matches Spec |
|---|---|---|---|---|
| `internal/tools/list_event_attachments_test.go` | `TestListEventAttachments_Success` | yes | yes | PASS |
| `internal/tools/list_event_attachments_test.go` | `TestListEventAttachments_NoContentBytesFetched` | yes | yes | PASS |
| `internal/tools/list_event_attachments_test.go` | `TestListEventAttachments_InvalidEventIDRejectedBeforeCall` | yes | yes | PASS |
| `internal/tools/list_event_attachments_test.go` | `TestListEventAttachments_MissingEventIDRejected` | no (added) | yes | PASS (extra coverage of FR-3's required parameter) |
| `internal/tools/list_event_attachments_test.go` | `TestListEventAttachments_Deterministic` | no (added) | yes | PASS (extra coverage of NFR-4) |
| `internal/tools/get_event_attachment_test.go` | `TestGetEventAttachment_ReturnsContent` | yes | yes | PASS |
| `internal/tools/get_event_attachment_test.go` | `TestGetEventAttachment_RefusesOverSizeCeiling` | yes | yes | PASS |
| `internal/tools/get_event_attachment_test.go` | `TestGetEventAttachment_RequiresAttachmentID` | yes | yes | PASS |
| `internal/tools/get_event_attachment_test.go` | `TestGetEventAttachment_RejectsInvalidEventIDBeforeCall` | no (added) | yes | PASS (extra coverage of FR-11) |
| `internal/tools/get_event_attachment_test.go` | `TestGetEventAttachment_Deterministic` | no (added) | yes | PASS (extra coverage of NFR-4) |
| `internal/tools/event_attachment_readonly_test.go` | `TestEventAttachmentReadsIssueOnlyGet` | yes (as amended) | yes | PASS — the behavioural half. The Test Strategy specified one test named `TestReadVerbsWireNoDeleteOrMutation` covering both halves; it was amended to the two that exist, carrying the split reason, because the schema half cannot live in `internal/tools` without an import cycle. `internal/tools/event_attachment_readonly_test.go:28` |
| `internal/tools/add_event_attachment_test.go` | `TestAddEventAttachment_DirectUploadPath` | yes | yes | PASS |
| `internal/tools/add_event_attachment_test.go` | `TestAddEventAttachment_UploadSessionPath` | yes | yes | PASS |
| `internal/tools/add_event_attachment_test.go` | `TestAddEventAttachment_SubjectFetchPrecedesTransfer` | yes | yes | PASS |
| `internal/tools/add_event_attachment_test.go` | `TestAddEventAttachment_RejectsBadInputBeforeAnyRequest/invalid_base64` | yes (as amended) | yes | PASS — `internal/tools/add_event_attachment_test.go:254`, asserting the error names `content_bytes` and that 0 GET / 0 POST were issued. The Test Strategy named the case as a standalone `TestAddEventAttachment_RejectsInvalidBase64`; it was amended to the shared bad-input table this case is a subtest of. FR-6 is graded and passes |
| `internal/tools/add_event_attachment_test.go` | `TestAddEventAttachment_RefusesOverSizeCeiling` | yes | yes | PASS |
| `internal/tools/add_event_attachment_test.go` | `TestAddEventAttachment_RejectsBadInputBeforeAnyRequest/over-length_name` | yes (as amended) | yes | PASS — `:252`, asserting the error names `name` and `255` and that no request was issued. The Test Strategy named the case as a standalone `TestAddEventAttachment_RejectsOverLengthName`; it was amended to the shared bad-input table. FR-12 is graded and passes |
| `internal/tools/add_event_attachment_test.go` | `TestAddEventAttachment_ConfirmationUsesGraphResponse` | yes | yes | PASS |
| `internal/tools/add_event_attachment_test.go` | `TestAddEventAttachment_UploadURLNeverEscapes` | yes | yes | PASS |
| `internal/tools/attachment_confirmation_test.go` | `TestFormatEventAttachmentConfirmation_NamesSubjectNameSizeIDAndPath` | yes | yes | PASS (added beside the mail cases, as specified) |
| `internal/tools/event_attachment_verbs_test.go` | `TestEventAttachmentVerbsHonourTimeoutAndRedaction` | yes | yes | PASS |
| `internal/tools/event_attachment_verbs_test.go` | `TestEventAttachmentErrorsReachBothChannels` | yes | yes | PASS |
| `internal/tools/event_attachment_verbs_test.go` | `TestEventAttachmentReadsDeterministic` | yes | yes | PASS |
| `internal/tools/tool_annotations_test.go` | `TestCalendarAttachmentVerbAnnotations` | yes | yes | PASS |
| `internal/server/calendar_verbs_test.go` | `TestCalendarRegistersAttachmentVerbs` | yes | yes | PASS |
| `internal/server/calendar_verbs_test.go` | `TestReadOnlyBlocksAddEventAttachment` | yes | yes | PASS |
| `internal/server/calendar_verbs_test.go` | `TestAttachmentVerbsCarryDotIdentity` | yes | yes | PASS |
| `internal/server/calendar_verbs_test.go` | `TestEventAttachmentReadSchemasDeclareNoMutation` | yes (as amended) | yes | PASS — the schema half of the split noted above, placed where the registry is in scope |

### Tests to Modify

| Test File | Test Name | Specified change | Applied | Matches Spec |
|---|---|---|---|---|
| `internal/tools/dispatch_registry_test.go` | `TestVerbInventoryUnchangedAfterUpgrade` via `verbInventoryGolden` | gain exactly three lines in sort order | yes (`:47`, `:54`, `:59`; 3 insertions, 0 deletions) | PASS |
| `internal/tools/tool_annotations_test.go` | `TestAggregateAnnotations_Calendar` | expectations unchanged, re-asserted | yes (untouched by the diff, passes against the 20-verb set) | PASS |
| `internal/server/calendar_verbs_test.go` | scheduling-read cases | joined by three attachment cases, otherwise untouched | yes (diff is additive from `:176`) | PASS |
| `docs/prompts/mcp-tool-crud-test.md` | lifecycle steps from Step 44 | yes | yes (`:715`, `:727`, `:736`) | PASS (unexecuted; GAP-1) |

### Gating tests that grade this change without modification

All executed, all pass: `TestEveryVerbHasDescription`, `TestEveryVerbHasSummary`,
`TestEveryVerbHasClassification`, `TestSeeDocsAnchorsResolve`,
`TestWriteVerbsDeclareNoOutputParameter`, `TestDescriptionLengthBounded`,
`TestEveryVerbStatesRequiredParameters`, `TestEveryParameterHasDescription`,
`TestDescriptionsListVerbsOnSeparateLines`, `TestColdStartSchemaSize_Reduction`,
`TestCommittedManifestMatchesRecord`, `TestManifestDescribesEveryRegisteredVerb`, and the
five `internal/auth` scope tests.

## Diff Coverage

Scope: `git diff a690a94...HEAD` (24 files, +3217 / -352).

| File | +/- | Mapped Requirements |
|---|---|---|
| `internal/tools/list_event_attachments.go` | +146 | FR-3, FR-11, FR-13, FR-17, FR-18, NFR-1, NFR-4, NFR-5 |
| `internal/tools/get_event_attachment.go` | +151 | FR-4, FR-5, FR-11, FR-13, FR-17, FR-18, FR-21, NFR-1, NFR-4, NFR-5 |
| `internal/tools/add_event_attachment.go` | +272 | FR-6, FR-7, FR-8, FR-9, FR-10, FR-11, FR-12, FR-17, FR-18, FR-19, NFR-1, NFR-5, NFR-6 |
| `internal/tools/attachment_confirmation.go` | +50/-5 | FR-10, NFR-1, NFR-7 (function appended; the mail formatter and both transfer constants are unchanged) |
| `internal/tools/attachment_upload_session.go` | +57/-5 | FR-7, FR-19, NFR-1, NFR-6 (only `createEventAttachmentUploadSession` added; `transferAttachmentChunks` and below unchanged) |
| `internal/server/calendar_verbs.go` | +142/-4 | FR-1, FR-2, FR-13, FR-15, FR-16, FR-20, FR-21, NFR-2, NFR-3, NFR-7 |
| `extension/manifest.json` | +1/-1 | FR-22 |
| `site/src/generated/surface.json` | +26/-6 | FR-23 |
| `docs/prompts/mcp-tool-crud-test.md` | +35 | FR-24 |
| `internal/tools/dispatch_registry_test.go` | +3 | FR-25 |
| `internal/tools/list_event_attachments_test.go` | +174 | FR-3, FR-11, NFR-4, NFR-5, AC-2 |
| `internal/tools/get_event_attachment_test.go` | +191 | FR-4, FR-5, FR-11, NFR-4, NFR-5, AC-3 |
| `internal/tools/add_event_attachment_test.go` | +355 | FR-6 to FR-12, FR-19, NFR-5, AC-4, AC-5, AC-10 |
| `internal/tools/event_attachment_readonly_test.go` | +80 | FR-14, AC-9 |
| `internal/tools/event_attachment_verbs_test.go` | +227 | FR-17, FR-18, NFR-4, AC-10, AC-11 |
| `internal/tools/attachment_confirmation_test.go` | +72/-2 | FR-10, AC-4 |
| `internal/tools/tool_annotations_test.go` | +66 | FR-15, AC-6 |
| `internal/server/calendar_verbs_test.go` | +208/-4 | FR-1, FR-2, FR-14, FR-16, AC-1, AC-9 |
| `docs/cr/CR-0081-calendar-event-attachments.md` | +1155/-330 | the CR itself (review reconciliation and finalization) |

### Unmapped changed files

Five files changed outside the CR's Affected Components. Each is justified below; none is
unaccounted for.

| File | +/- | Why it changed, and whether it is justified |
|---|---|---|
| `internal/server/server.go` | +1 | Justified. `maxAttachmentSize: cfg.MaxAttachmentSizeBytes` at `:85`. FR-5 and FR-8 require the ceiling in two calendar handlers, and `calendarVerbsConfig` had no field for it, so the production wiring had to pass it. Without this line both requirements would be unreachable. The CR's Affected Components names `internal/server/calendar_verbs.go` but omits its two callers |
| `internal/server/introspect_verbs.go` | +1 | Justified, same reason: `:73` supplies the field on the introspection path, so `system.help` and the surface manifest generator build the calendar verb set identically to the server |
| `docs/concepts.md` | +1/-1 | Justified. The read-only mode section enumerates every write verb; FR-2 makes `calendar.add_event_attachment` one, and the project's documentation governance names `docs/concepts.md` as the single owner of that list. Leaving it out would have shipped a documented write-verb inventory that omits a write verb |
| `docs/readme.md` | +2/-1 | Justified. An embedded-bundle file: adds the event-attachments capability bullet and widens the `MAX_ATTACHMENT_SIZE_BYTES` row to name all four verbs the one knob now governs, which is the consequence of the CR's own decision to reuse that limit (Open Question 2) |
| `docs/backlog/cr-0078-0083.md` | +148/-2 | Justified. The per-phase decision ledger the CR review established (`## CR-0081`), recording implementation decisions on the record. Governance artefact, no runtime effect |

No stray source change exists: every non-test `.go` file in the diff is either named in
Affected Components or is one of the two single-line plumbing lines above.

## Gaps

Two gaps were raised, both the same shape: verification the CR's own Test Strategy makes
part of acceptance, which has not been executed. Neither is a defect in the implementation,
and neither is fixable by a source edit. GAP-2's artefact has since been written and is
recorded below; both runs are pending for the user.

**GAP-1 — the lifecycle harness has not been run against the new steps. Open, pending for
the user.**
Requirement ref: FR-24, AC-7 (harness clause), and the CR's Verification Commands
(`make crud-test`). What is missing: Steps 44 to 46 of `docs/prompts/mcp-tool-crud-test.md`
exist and are well-formed, but no harness run has exercised them, so the prompt's claims
about `add_event_attachment`, `list_event_attachments`, and `get_event_attachment` are
unverified against a real server. This is precisely the prompt-drift class `CLAUDE.md`
records as still open: a parameter named in prose but absent from the registry would pass
every check run here.
Suggested minimal fix: none in source. Deferred to the user. Rebuild the binary the harness
drives first (`go build -ldflags="-X main.commit=$(git rev-parse --short HEAD) ..." -o
./outlook-local-mcp ./cmd/outlook-local-mcp`), then `make crud-test`, then confirm the
report's own `Server version` line matches `git rev-parse --short HEAD` before trusting any
row. Excluded from this validation because it is a paid harness.

**GAP-2 — FIXED (artefact persisted; the run itself is pending for the user).**
`.agents/scenarios/2026-09-02-event-attachment-two-transfer-paths.md` now holds the derived
scenario, in the same format as its three siblings, marked `outcome: not-run` and
`runs: "0 of 0 attempted"`. It covers the four steps the CR enumerates plus a `tools/list`
check and two refusal steps, grades on the calendar state read back rather than on the write
confirmations, names the two files' recorded byte sizes as the thing that proves every chunk
arrived, and states its own restoration (manual, because no event-attachment delete verb
exists). What remains is the run, which needs a live authenticated mailbox and an
interactively granted keychain entry and so cannot be performed from a headless session; it
is recorded as pending for the user under `## CR-0081` in `docs/backlog/cr-0078-0083.md`.
Until it runs, the chunked upload path on the event navigation has still never moved a byte
against real Graph. The original finding follows.

**GAP-2 (as originally raised) — no user scenario is persisted for CR-0081.**
Requirement ref: the CR's Test Strategy closing paragraph, which states "Acceptance
additionally requires a user scenario driving the built server against a live mailbox: list
the attachments on a real invitation, download one, add a small file and confirm it appears
in a re-list, then add a file above the threshold and confirm the upload-session path
succeeds. Persist the scenario under `.agents/scenarios/`."
What is missing: `.agents/scenarios/` holds three scenarios
(`2026-09-01-received-message-management-lifecycle.md`,
`2026-09-02-calendar-scheduling-reads-live-mailbox.md`,
`2026-09-02-draft-attachment-two-transfer-paths.md`) and none of them covers event
attachments. Sibling CR-0079 and CR-0080 each persisted one; CR-0081 did not. The
consequence is that the entire chunked-upload path, which Risk 1 of the CR names as the
under-exercised one, has never moved a byte against real Graph, and the direct path has
never been round-tripped through a real mailbox.
Suggested minimal fix: none in source. Deferred to the user. Run the `user-scenario` skill
against the four steps the CR enumerates and persist the artefact under
`.agents/scenarios/`. It requires a live mailbox and a signed binary with an existing
keychain grant, so it cannot be executed from a headless validation session.
