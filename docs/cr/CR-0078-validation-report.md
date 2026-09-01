# CR-0078 Validation Report

Validated 2026-09-01 against `HEAD` `c8544d1` on branch `docs/cr-implementation-set-0079-0083`.
Branch base (`git merge-base origin/main HEAD`): `2cce019`. Implementation commits:
`f3357a3`, `86fc06e`, `5f5b387`, `cdc05e6`, `cfb9aa3`, `0d768df`, plus the finalization
checkpoint `c8544d1`.

## Summary

Requirements: 29/34 | Acceptance Criteria: 20/25 | Tests: 27/36 | Gaps: 9

No FAIL. Every functional and non-functional requirement is implemented and maps to a
changed file with a specific hunk. Five requirements and five acceptance criteria are
downgraded to PARTIAL because the behaviour they assert is implemented but has no test
grading it, and in every one of those five cases the CR's Test Strategy named a test that
was not written.

### Check pipeline

| Command | Result |
|---|---|
| `make ci` | **pass**, exit 0 (build, vet, tidy, `golangci-lint` 0 issues, `go test ./...`, docs-bundle, surface drift check, `goreleaser check`, `mcpb validate`) |
| `make ci` surface regeneration | working tree unchanged afterwards (`git status --porcelain` empty), so the committed `site/src/generated/surface.json` matches the live registry |
| `go test -race ./internal/tools/ ./internal/server/ ./internal/graph/ ./internal/validate/ -run 'MoveMessage\|SetFlag\|SetCategories\|MarkRead\|Confirmation\|SharedParameters\|Manifest\|VerbInventory'` (CR Verification Commands) | **pass**, 47 tests, 0 failures |
| `go test -race ./internal/docs/ ./internal/surface/ -run 'MailGatingRow\|CommittedManifest\|RecordCounts\|DefaultCount\|EveryVerbCarries'` | **pass**, 4 tests |
| `go test -race ./internal/tools/ ./internal/server/ -run 'MailAnnotations\|AggregateAnnotations\|EveryVerbHas\|SeeDocs\|DescriptionLengthBounded\|ColdStart\|RegisterTools_Mail\|ReadOnlyGuard'` | **pass**, 30 tests |
| `make crud-test` | **not run** (drives a live mailbox and costs a paid agent run; out of scope for a documentation-only audit) |

### Measured gates

Measured rather than asserted, by driving the binary `make ci` built and by reading the
instruments' own log lines.

| Gate | Measured | Bound | Source |
|---|---|---|---|
| Composed `mail` tool description | **2 737 characters** | < 4 000 | `tools/list` over stdio against `./outlook-local-mcp` with `OUTLOOK_MCP_MAIL_MANAGE_ENABLED=true` |
| Cold-start schema, four tools | **18 151 bytes, 75 % reduction** | >= 60 % | `schema_size_test.go:82-84` log output |
| Third-party dependencies | `go.mod` / `go.sum` diff is **empty** over `2cce019..HEAD` | none added | `git diff 2cce019..HEAD -- go.mod go.sum` |
| Verb inventory golden delta | **exactly 4 added lines, 0 removed** | exactly 4 | `internal/tools/dispatch_registry_test.go:73-77` |

### Behavioural verification performed

The unit suite issues no Graph call, so five criteria were additionally verified by driving
the built binary over stdio as an MCP client. These are recorded because they are stronger
than registry-level evidence and because they cover three criteria whose CR-named tests are
absent.

| Observation | Result |
|---|---|
| `tools/list` under `MAIL_MANAGE_ENABLED=true` | 4 tools (`account`, `calendar`, `mail`, `system`); `mail` operation enum holds 17 verbs including `move_message`, `set_flag`, `set_categories`, `mark_read` |
| `tools/list` under `MAIL_ENABLED=true, MAIL_MANAGE_ENABLED=false` | `mail` operation enum holds 8 verbs; none of the four is present; folded annotations `readOnlyHint: true, destructiveHint: false, idempotentHint: true` |
| Published shared-parameter descriptions | `flag_status`, `is_read`, `importance`, `message_id` each name a declaring write verb; `folder_id` and `destination_folder_id` publish distinct text |
| Folded `mail` annotations under manage | `readOnlyHint: false, destructiveHint: true, idempotentHint: false, openWorldHint: true`, unchanged from before this change |
| `tools/call mail` under `READ_ONLY=true` for each of the four | `operation blocked: mail.move_message is not allowed in read-only mode`, and identically for `mail.set_flag`, `mail.set_categories`, `mail.mark_read` |

## Requirement Verification

### Functional Requirements

| Req # | Description | Status | Evidence (file:line / test name) |
|---|---|---|---|
| FR-1 | Register exactly four new mail verbs; no new top-level tool; tool count stays 4 | PASS | `internal/server/mail_verbs.go:139-142`; live `tools/list` returns 4 tools and a 17-verb mail enum; `TestManifestDescribesEveryRegisteredVerb` (`internal/server/manifest_sync_test.go:84`) asserts the manifest `tools` array holds exactly 4; `TestAggregateAnnotations_FourToolsRegistered` |
| FR-2 | Registered only under `MailManageEnabled`; absent under default and `MailEnabled`-only | PASS | `internal/server/mail_verbs.go:130-143` (the `MailManageEnabled` block); live `tools/list` under `MAIL_ENABLED`-only shows none of the four; gate attribution derived by probe (`internal/surface/build.go:57-73`) records `OUTLOOK_MCP_MAIL_MANAGE_ENABLED` for all four in `site/src/generated/surface.json:181-205`, graded by `TestEveryVerbCarriesSummaryAndGate` and `TestDefaultCountExcludesGatedVerbs` |
| FR-3 | `move_message` requires both IDs and POSTs the move with `destinationId` | PASS | `internal/tools/move_message.go:63-91`; `TestMoveMessage_Success` (`move_message_test.go:33`), `TestMoveMessage_RequiresDestinationFolderID` (`:89`) |
| FR-4 | Move confirmation names the new ID, the original ID, and the destination | PASS | `internal/tools/move_message.go:107-121`; `TestMoveMessage_ConfirmationNamesAllThreeIdentifiers` (`move_message_test.go:63`) |
| FR-5 | `set_flag` requires `message_id` and `flag_status`, restricted to the three values, written by PATCH | PASS | `internal/tools/set_flag.go:57-81`; enum on `internal/server/mail_verbs.go:672-676`; `TestSetFlag_Success` (`set_flag_test.go:55`), `TestSetFlag_RejectsUnknownStatus` (`:90`) |
| FR-6 | `set_categories` replaces the full array and states the resulting list | PASS | `internal/tools/set_categories.go:70-84`, `:133-138`; `TestSetCategories_ReplacesFullSet` (`set_categories_test.go:18`), `TestSetCategories_ConfirmationListsResult` (`:47`) |
| FR-7 | Validate with `ValidateStringLength` against `MaxCategoriesLen`; split on commas, trim, drop empties | PASS | `internal/tools/set_categories.go:66-71`; `TestSetCategories_OverLengthRejected` (`set_categories_test.go:105`), `TestSetCategories_ReplacesFullSet` |
| FR-8 | Empty or whitespace-only `categories` clears every category, confirmation says so | PASS | `internal/tools/set_categories.go:71-76`, `:134-136`; `TestSetCategories_EmptyValueClears` (`set_categories_test.go:76`) asserts `"categories":[]` in the PATCH body and the no-categories sentence |
| FR-9 | `mark_read` requires `message_id` and a boolean `is_read`, written by PATCH | PASS | `internal/tools/mark_read.go:57-73`; `TestMarkRead_SetsRead` (`mark_read_test.go:16`), `TestMarkRead_SetsUnread` (`:49`), `TestMarkRead_RequiresIsRead` (`:78`), `TestMarkRead_RejectsNonBooleanIsRead` (`:104`) |
| FR-10 | All four validate `message_id` with `ValidateResourceID` before any Graph request | PARTIAL | Implemented in all four: `move_message.go:67`, `set_flag.go:53`, `set_categories.go:56`, `mark_read.go:53`. Graded for two only: `TestMoveMessage_InvalidIdentifiersRejectedBeforeCall` (`move_message_test.go:146`), `TestSetFlag_InvalidMessageIDRejectedBeforeCall` (`set_flag_test.go:180`). The cross-verb test `TestMailWriteVerbs_RequireMessageID` (`mail_write_verbs_test.go:124`) omits `message_id` rather than malforming it, so `set_categories` and `mark_read` have no malformed-identifier evidence. See G1. |
| FR-11 | None of the four applies the `isDraft` guard | PASS | No `verifyIsDraft` call in any of the four handlers; `TestMoveMessage_AcceptsNonDraftMessage` (`:180`), `TestSetFlag_AcceptsNonDraftMessage` (`set_flag_test.go:148`), `TestSetCategories_AcceptsNonDraftMessage` (`set_categories_test.go:161`), `TestMarkRead_AcceptsNonDraftMessage` (`mark_read_test.go:130`); `TestMailWriteVerbs_SingleGraphRequestOnSuccess` (`mail_write_verbs_test.go:97`) proves no preceding GET |
| FR-12 | Unconditional text confirmation; no `output` parameter declared | PARTIAL | No `output` in any of the four `Schema` blocks (`mail_verbs.go:632-646`, `:667-681`, `:701-715`, `:735-749`); every handler returns `mcp.NewToolResultText` on success. The derived cross-domain check the CR specified, `TestWriteVerbsDeclareNoOutputParameter`, is absent from `internal/tools/verb_metadata_test.go`. See G5. |
| FR-13 | Confirmations built from the Graph response, not the request arguments | PARTIAL | Implemented in all four (`move_message.go:107-121`, `set_flag.go:102-109` via `responseFlagStatus:128`, `set_categories.go:105-113` via `responseCategories:133`, `mark_read.go:94-101` via `responseReadState:119`). Falsifiable evidence exists for two: `TestSetCategories_ConfirmationListsResult` asserts the request value `"Ignored"` is absent from the output; `TestMoveMessage_ConfirmationNamesAllThreeIdentifiers` uses a response ID differing from the request. `set_flag` and `mark_read` canned responses echo the request, so the "not the arguments" half is untestable there. See G4. |
| FR-14 | All four annotation hints declared explicitly, per the matrix | PASS | `mail_verbs.go:626-631` (`move_message`: false/true/false/true), `:660-665`, `:694-699`, `:728-733` (the other three: false/false/true/true); `verbInventoryGolden` (`dispatch_registry_test.go:73-77`) asserts the values, graded by `TestVerbInventoryUnchangedAfterUpgrade`; `TestEveryVerbHasClassification` |
| FR-15 | Wrapped by the write middleware chain under the `mail.<verb>` identity | PASS | `mail_verbs.go:139-142` via `wrapWrite` (`:107-108`, chaining auth, account resolution, observability, `ReadOnlyGuard`, `AuditWrap`); `TestMailManagementVerbsCarryDotIdentity` (`mail_verbs_test.go:259`) asserts the audit record's `tool_name` reads `mail.<verb>` with `operation_type` `write`; live read-only refusal names `mail.move_message` and the other three |
| FR-16 | Shared read/write parameter descriptions name a declaring write verb (four rewrites) | PASS | `mail_verbs.go:214` (`is_read`), `:223` (`importance`), `:227` (`flag_status`), `:269` (`message_id`); `TestSharedParametersNameTheirWriteVerbs` (`mail_verbs_test.go:161`) derives its cases from the registry and exempts `account` inline; live `tools/list` confirms the published text |
| FR-17 | `destination_folder_id` is its own parameter, not `folder_id` | PASS | `mail_verbs.go:637-640`; `TestDestinationFolderIDIsNotFolderID` (`mail_verbs_test.go:224`); live schema publishes both with distinct descriptions |
| FR-18 | Non-empty `Summary` <= 80 chars, `Description`, >= 1 `Examples`, >= 1 resolving `SeeDocs` | PASS | `mail_verbs.go:619-625`, `:652-658`, `:687-692`, `:721-726`; summaries measure 65, 64, 68, 60 characters; `TestEveryVerbHasSummary` (bound at `verb_metadata_test.go:217`), `TestEveryVerbHasDescription`, `TestSeeDocsAnchorsResolve`, `TestEveryVerbStatesRequiredParameters` |
| FR-19 | The `mail` domain `Intro` names the four new write verbs | PASS | `internal/server/server.go:178-179`; live `tools/list` description reads "...update_draft, delete_draft, move_message, set_flag, set_categories, mark_read) are registered when MailManageEnabled is configured" |
| FR-20 | `extension/manifest.json` mail entry enumerates the four; `tools` array stays 4 | PASS | `extension/manifest.json:61`; `TestManifestDescribesEveryRegisteredVerb` (`manifest_sync_test.go:84`); `mcpb validate` passes in `make ci` |
| FR-21 | `site/src/generated/surface.json` regenerated and committed | PASS | `site/src/generated/surface.json:181-205`, `fullCount` 13 -> 17, totals 42 -> 46; `make ci` regenerated it and left the working tree clean; `TestCommittedManifestMatchesRecord` |
| FR-22 | Lifecycle prompt gains a step per verb; gated skip instruction widened | PASS | `docs/prompts/mcp-tool-crud-test.md:583` (skip now covers "Steps 31 through 35 and Steps 37 through 40"), new Steps 37-40 at `:639-678`, report rows at `:752-755` |
| FR-23 | Verb inventory golden regenerated with the four new identities | PASS | `internal/tools/dispatch_registry_test.go:73-77`, exactly four added lines; `TestVerbInventoryUnchangedAfterUpgrade` passes |
| FR-24 | The `MAIL_MANAGE_ENABLED` gating row names received-message management | PASS | `docs/concepts.md:65`; `TestMailGatingRowNamesMessageManagement` (`internal/docs/catalog_test.go:56`) |
| FR-25 | No change to the requested OAuth scope set; `Mail.Send` stays unrequested | PASS | `internal/auth/**` has zero diff over `2cce019..HEAD`; `TestScopes_CalendarOnly`, `TestScopes_WithMail`, `TestScopes_MailManage`, `TestScopes_MailManageImpliesRead`, `TestScopes_NoMailSend` pass unchanged in `make ci` |
| FR-26 | The mail domain's `defaultCount` stays 5 | PASS | `site/src/generated/surface.json:207` unchanged at 5 (only `fullCount` moved in the diff); `TestDefaultCountExcludesGatedVerbs`, `TestRecordCountsMatchBuiltVerbs` |

### Non-Functional Requirements

| Req # | Description | Status | Evidence (file:line / test name) |
|---|---|---|---|
| NFR-1 | Each handler in its own file under `internal/tools/`, named for the verb | PASS | `internal/tools/move_message.go`, `set_flag.go`, `set_categories.go`, `mark_read.go`, each 127-138 lines, one constructor apiece |
| NFR-2 | Composed `mail` description stays below 4 000 characters | PASS | Measured 2 737 characters from the live `tools/list`; `TestDescriptionLengthBounded` (`description_quality_test.go:147`) passes |
| NFR-3 | Cold-start schema reduction stays >= 60 % | PASS | 18 151 bytes, 75 % reduction, logged by `schema_size_test.go:82-84`; `TestColdStartSchemaSize_Reduction` passes |
| NFR-4 | Every error carries a fix instruction reaching both the tool result and the log record | PARTIAL | Both channels implemented: `move_message.go:99-104` (result text plus `"fix", moveFixInstruction` on the log record), `set_flag.go:91-96`, `set_categories.go:94-99`, `mark_read.go:83-88`. Only the tool-result channel is graded: `TestMailWriteVerbs_GraphFailureCarriesFix` (`mail_write_verbs_test.go:224`), `TestMoveMessage_UnresolvableDestinationCarriesFix` (`move_message_test.go:206`). No test binds a logger to a capture buffer. See G3. |
| NFR-5 | No third-party dependency added | PASS | `git diff 2cce019..HEAD -- go.mod go.sum` is empty; `go mod tidy` in `make ci` leaves the tree clean |
| NFR-6 | Exactly one Graph request per success path; no read-modify-write | PASS | `TestMailWriteVerbs_SingleGraphRequestOnSuccess` (`mail_write_verbs_test.go:97`) asserts exactly one request of the expected method for each of the four |
| NFR-7 | Route through `RetryGraphCall` and `WithTimeout`; redact Graph errors with the existing helpers | PARTIAL | Wiring present and identical across the four: `move_message.go:83-105`, `set_flag.go:75-97`, `set_categories.go:78-100`, `mark_read.go:67-89` each call `graph.WithTimeout`, `graph.RetryGraphCall`, `graph.IsTimeoutError`, `graph.TimeoutErrorMessage`, `graph.RedactGraphError`. No test exercises the timeout path or asserts redaction of a token-like string for any of the four. See G2. |
| NFR-8 | The three property writes are deterministic and idempotent | PASS | `TestMailWriteVerbs_PropertyWritesAreIdempotent` (`mail_write_verbs_test.go:253`) asserts identical request bodies and identical confirmation text on repeat, for the three, excluding `move_message` |

## Acceptance Criteria Verification

| AC # | Description | Status | Evidence |
|---|---|---|---|
| AC-1 | The four register, and only behind the manage gate | PASS | Live `tools/list`: under `MAIL_MANAGE_ENABLED` the enum holds all four and exactly 4 tools are registered; under `MAIL_ENABLED`-only none is present. `TestEveryVerbCarriesSummaryAndGate`, `TestDefaultCountExcludesGatedVerbs`, `TestManifestDescribesEveryRegisteredVerb`, `TestAggregateAnnotations_FourToolsRegistered`. The two CR-named tests for this criterion are absent; see G7. |
| AC-2 | A move surfaces the identifier the caller cannot otherwise see | PASS | `TestMoveMessage_Success` (`move_message_test.go:33`), `TestMoveMessage_ConfirmationNamesAllThreeIdentifiers` (`:63`); `move_message.go:113-121` renders the new ID, the destination, and the sentence that the original no longer resolves |
| AC-3 | The follow-up flag is written, and only with an accepted status | PASS | `TestSetFlag_Success` (`set_flag_test.go:55`) asserts `"flagStatus":"flagged"` in the PATCH body and the status in the confirmation; `TestSetFlag_RejectsUnknownStatus` (`:90`) asserts refusal naming the three values with no request issued; `TestValidateFlagStatus` (`validate_test.go:438`), `TestParseFlagStatus` (`enums_test.go:105`) |
| AC-4 | Categories are replaced as a set, and the result is stated | PASS | `TestSetCategories_ReplacesFullSet` (`set_categories_test.go:18`), `TestSetCategories_ConfirmationListsResult` (`:47`), `TestSetCategories_OverLengthRejected` (`:105`) |
| AC-5 | An empty categories value clears every category | PASS | `TestSetCategories_EmptyValueClears` (`set_categories_test.go:76`) |
| AC-6 | The read state is written in both directions | PASS | `TestMarkRead_SetsRead` (`mark_read_test.go:16`), `TestMarkRead_SetsUnread` (`:49`), `TestMarkRead_RequiresIsRead` (`:78`) |
| AC-7 | An invalid message identifier never reaches Microsoft Graph | PARTIAL | Graded for `move_message` and `set_flag` only. `TestMailWriteVerbs_RequireMessageID` covers all four but tests a *missing* identifier, not a *malformed* one, so it does not exercise `ValidateResourceID`. `set_categories` and `mark_read` have no malformed-identifier test. See G1. |
| AC-8 | A received message is not refused for being a non-draft | PASS | Four per-handler tests: `TestMoveMessage_AcceptsNonDraftMessage`, `TestSetFlag_AcceptsNonDraftMessage`, `TestSetCategories_AcceptsNonDraftMessage`, `TestMarkRead_AcceptsNonDraftMessage`; plus `TestMailWriteVerbs_SingleGraphRequestOnSuccess` proving no draft-check GET precedes the write |
| AC-9 | Each verb is a write verb by the project's tiering rule | PARTIAL | The text-confirmation half is graded for all four by `TestMailWriteVerbs_ConfirmationNamesSubjectAndIdentifier` (`mail_write_verbs_test.go:176`). The derived half, "every verb whose read-only hint is false declares no output parameter", has no test: `TestWriteVerbsDeclareNoOutputParameter` was never written. Registry evidence only. See G5. |
| AC-10 | Confirmations report the service, not the request | PARTIAL | Proven with a differing response for two of four: `TestSetCategories_ConfirmationListsResult` (asserts the request value `"Ignored"` is absent) and `TestMoveMessage_ConfirmationNamesAllThreeIdentifiers`. For `set_flag` and `mark_read` every canned response echoes the request, so the criterion cannot fail there. See G4. |
| AC-11 | Every hint is declared, and matches the matrix | PASS | `verbInventoryGolden` (`dispatch_registry_test.go:73-77`) asserts all four hints per verb against the matrix; live folded `mail` annotations are `false/true/false/true` under manage and `true/false/true/true` under read-only, unchanged from before this change (`TestMailAnnotationsManageEnabled`, `TestMailAnnotationsGatedReadOnly`). The CR-named `TestMailManagementVerbAnnotations` is absent; see G6. |
| AC-12 | Read-only mode blocks all four, and the identity is consistent | PASS | Second clause graded by `TestMailManagementVerbsCarryDotIdentity` (`mail_verbs_test.go:259`), which reads the emitted audit record and asserts `tool_name` is `mail.<verb>` with `operation_type` `write`. First clause verified behaviourally: driving the built binary with `OUTLOOK_MCP_READ_ONLY=true` returns `operation blocked: mail.<verb> is not allowed in read-only mode` for each of the four. The CR-named `TestReadOnlyBlocksMailManagementVerbs` is absent; see G8. |
| AC-13 | A shared parameter's published description covers its write sense | PASS | `TestSharedParametersNameTheirWriteVerbs` (`mail_verbs_test.go:161`) derives its cases from the registry, exempts `account` inline, and fails when the selection is empty; live `tools/list` confirms all four published strings name a write verb |
| AC-14 | Scoping and destination stay distinct | PASS | `TestDestinationFolderIDIsNotFolderID` (`mail_verbs_test.go:224`); live schema publishes `folder_id` ("Mail folder ID to list messages from") and `destination_folder_id` ("...This names where the message is moved to; folder_id scopes a read instead") separately |
| AC-15 | Registry metadata is complete for every new verb | PASS | `TestEveryVerbHasSummary`, `TestEveryVerbHasDescription`, `TestEveryVerbHasClassification`, `TestSeeDocsAnchorsResolve`, `TestEveryVerbStatesRequiredParameters`, all passing over the maximal configuration |
| AC-16 | The domain introduction and the extension manifest name the new verbs | PASS | `server.go:178-179` and `extension/manifest.json:61`; `TestManifestDescribesEveryRegisteredVerb` asserts every registered verb of every domain appears in its manifest description and the `tools` array holds exactly four |
| AC-17 | The generated surface manifest is regenerated and committed | PASS | `make ci` ran `surface-manifest` and left the tree clean; `TestCommittedManifestMatchesRecord`; `surface.json` records mail 17 full / 5 default and totals 46 full / 33 default, exactly the CR's figures |
| AC-18 | The lifecycle harness exercises and gates the new verbs | PASS | Steps 37-40 at `docs/prompts/mcp-tool-crud-test.md:639-678`, one per verb; skip instruction widened at `:583`; `scripts/crud-test.sh` unchanged and correct, its accounting keying on `mcp__outlook-local-mcp__mail` (`:115`) with no per-verb or per-step logic; `docs/bench/crud-runs.csv` header still 23 columns |
| AC-19 | The verb inventory golden records exactly the intended delta | PASS | `git diff` on `dispatch_registry_test.go` is `+4/-0`, one line per new verb; `TestVerbInventoryUnchangedAfterUpgrade` passes |
| AC-20 | The gating documentation stops reading as exhaustive | PASS | `docs/concepts.md:65`; `TestMailGatingRowNamesMessageManagement` (`internal/docs/catalog_test.go:56`) |
| AC-21 | The consent surface is unchanged | PASS | `internal/auth/**` zero diff; the five scope tests pass unchanged in `make ci` |
| AC-22 | The measured surface gates keep their margins | PASS | Description 2 737 < 4 000; cold-start 18 151 bytes at 75 % >= 60 %; `go.mod`/`go.sum` diff empty; `TestMailWriteVerbs_SingleGraphRequestOnSuccess` proves one Graph request per success path |
| AC-23 | The three property writes are idempotent | PASS | `TestMailWriteVerbs_PropertyWritesAreIdempotent` (`mail_write_verbs_test.go:253`) asserts identical request body and identical confirmation on the second call |
| AC-24 | A failure carries its correction on both channels | PARTIAL | Tool-result channel graded: `TestMoveMessage_UnresolvableDestinationCarriesFix` (`move_message_test.go:206`) asserts the result names `list_folders`. Log channel implemented (`move_message.go:101-103`) but **not** graded: the test does not bind the handler's logger to a capture buffer, which the CR's Test Strategy row for this criterion explicitly required. See G3. |
| AC-25 | The implementation follows the project's file and helper conventions | PARTIAL | The file-layout clause PASSES: four handlers, four files, named for the verbs. The error-handling clause is ungraded: no test causes a timeout to assert the message names the configured seconds, and no test asserts `RedactGraphError` removed a token-like string. Code evidence only. See G2. |

## Test Strategy Verification

### Tests to Add

| Test File | Test Name | Specified | Exists | Matches Spec |
|---|---|---|---|---|
| `internal/tools/move_message_test.go` | `TestMoveMessage_Success` | yes | yes (`:33`) | yes |
| `internal/tools/move_message_test.go` | `TestMoveMessage_ConfirmationNamesNewID` | yes | renamed to `TestMoveMessage_ConfirmationNamesAllThreeIdentifiers` (`:63`) | yes |
| `internal/tools/move_message_test.go` | `TestMoveMessage_RequiresDestination` | yes | renamed to `TestMoveMessage_RequiresDestinationFolderID` (`:89`) | yes |
| `internal/tools/move_message_test.go` | `TestMoveMessage_InvalidMessageIDRejectedBeforeCall` | yes | renamed to `TestMoveMessage_InvalidIdentifiersRejectedBeforeCall` (`:146`) | yes, and broader (covers both identifiers) |
| `internal/tools/mail_write_verbs_test.go` | `TestInvalidMessageIDRejectedByEveryWriteVerb` | yes | **no** | **no** — `TestMailWriteVerbs_RequireMessageID` (`:124`) omits the identifier rather than malforming it, so `ValidateResourceID` is never reached for `set_categories` or `mark_read` (G1) |
| `internal/tools/mail_write_verbs_test.go` | `TestNoDraftGuardOnReceivedMessageWrites` | yes | as four per-handler `*_AcceptsNonDraftMessage` tests | yes |
| `internal/tools/mail_write_verbs_test.go` | `TestWriteVerbsHonourTimeoutAndRedaction` | yes | **no** | **no** — no equivalent anywhere; the timeout and redaction paths are ungraded for all four (G2) |
| `internal/tools/move_message_test.go` | `TestMoveMessage_UnresolvableDestinationCarriesFix` | yes | yes (`:206`) | **no** — asserts the tool result only; the specified logger capture buffer and log-record assertion are absent (G3) |
| `internal/tools/set_flag_test.go` | `TestSetFlag_Success` | yes | yes (`:55`) | yes |
| `internal/tools/set_flag_test.go` | `TestSetFlag_RejectsUnknownStatus` | yes | yes (`:90`) | yes |
| `internal/tools/set_flag_test.go` | `TestSetFlag_AcceptsNonDraftMessage` | yes | yes (`:148`) | yes |
| `internal/tools/set_categories_test.go` | `TestSetCategories_ReplacesFullSet` | yes | yes (`:18`) | yes |
| `internal/tools/set_categories_test.go` | `TestSetCategories_ConfirmationListsResult` | yes | yes (`:47`) | yes |
| `internal/tools/set_categories_test.go` | `TestSetCategories_EmptyValueClears` | yes | yes (`:76`) | yes |
| `internal/tools/set_categories_test.go` | `TestSetCategories_OverLengthRejected` | yes | yes (`:105`) | yes |
| `internal/tools/mark_read_test.go` | `TestMarkRead_SetsRead` | yes | yes (`:16`) | yes |
| `internal/tools/mark_read_test.go` | `TestMarkRead_SetsUnread` | yes | yes (`:49`) | yes |
| `internal/tools/mark_read_test.go` | `TestMarkRead_RequiresIsRead` | yes | yes (`:78`) | yes |
| `internal/tools/mail_write_confirmation_test.go` | `TestFormatMailWriteConfirmationFields` | yes | yes (`:16`) | yes |
| `internal/tools/mail_write_verbs_test.go` | `TestConfirmationsUseGraphResponseNotArguments` | yes | **no** | **partial** — `TestSetCategories_ConfirmationListsResult` and `TestMoveMessage_ConfirmationNamesAllThreeIdentifiers` cover two of four; `set_flag` and `mark_read` have no differing-value case (G4) |
| `internal/tools/mail_write_verbs_test.go` | `TestStateSetVerbsAreIdempotent` | yes | renamed to `TestMailWriteVerbs_PropertyWritesAreIdempotent` (`:253`) | yes |
| `internal/tools/mail_write_verbs_test.go` | `TestSingleGraphRequestPerWrite` | yes | renamed to `TestMailWriteVerbs_SingleGraphRequestOnSuccess` (`:97`) | yes |
| `internal/tools/tool_annotations_test.go` | `TestMailManagementVerbAnnotations` | yes | **no** — `tool_annotations_test.go` has zero diff | **partial** — the values are asserted by `verbInventoryGolden` instead (G6) |
| `internal/tools/verb_metadata_test.go` | `TestWriteVerbsDeclareNoOutputParameter` | yes | **no** — `verb_metadata_test.go` has zero diff | **no** (G5) |
| `internal/server/mail_verbs_test.go` | `TestSharedParametersNameTheirWriteVerbs` | yes | yes (`:161`) | yes, cases derived from the registry as specified |
| `internal/server/mail_verbs_test.go` | `TestDestinationFolderIDIsNotFolderID` | yes | yes (`:224`) | yes |
| `internal/server/manifest_sync_test.go` | `TestManifestDescribesEveryRegisteredVerb` | yes | yes (`:84`) | yes |
| `internal/server/server_test.go` | `TestRegisterTools_MailManage_RegistersManagementVerbs` | yes | **no** — `server_test.go` has zero diff | **no** (G7) |
| `internal/server/readonly_test.go` | `TestReadOnlyBlocksMailManagementVerbs` | yes | **no** — `readonly_test.go` has zero diff | **no** (G8) |
| `internal/server/mail_verbs_test.go` | `TestMailManagementVerbsCarryDotIdentity` | yes | yes (`:259`) | yes |
| `internal/docs/catalog_test.go` | `TestMailGatingRowNamesMessageManagement` | yes | yes (`:56`) | yes |

### Tests to Modify

| Test File | Test Name | Specified | Exists | Matches Spec |
|---|---|---|---|---|
| `internal/tools/dispatch_registry_test.go` | `TestVerbInventoryUnchangedAfterUpgrade` | golden 42 -> 46, four added lines | yes (`:73-77`) | yes, exactly the four specified strings |
| `internal/server/server_test.go` | `TestRegisterTools_MailEnabled` | add a negative assertion that none of the four is present | **not modified** (`:756`, zero diff) | **no** (G7) |
| `internal/tools/tool_annotations_test.go` | `TestMailAnnotationsManageEnabled` | unchanged expectations re-asserted against the larger verb set | yes (`:239`), unchanged and passing | yes |
| `internal/tools/tool_annotations_test.go` | `TestMailAnnotationsGatedReadOnly` | unchanged expectation re-asserted | yes (`:220`), unchanged and passing | yes |
| `docs/prompts/mcp-tool-crud-test.md` | Lifecycle prompt steps | four new steps and a widened skip range | yes (`:583`, `:639-678`, `:752-755`) | yes |

### Existing gate tests that grade this change without modification

All verified passing at `HEAD`: `TestEveryVerbHasDescription`, `TestEveryVerbHasSummary`,
`TestEveryVerbHasClassification`, `TestSeeDocsAnchorsResolve`,
`TestDescriptionsListVerbsOnSeparateLines`, `TestMailDescriptionMentionsGatedVerbs`,
`TestEveryVerbStatesRequiredParameters`, `TestEveryParameterHasDescription`,
`TestDescriptionLengthBounded`, `TestColdStartSchemaSize_Reduction`,
`TestCommittedManifestMatchesRecord`, `TestRecordCountsMatchBuiltVerbs`,
`TestDefaultCountExcludesGatedVerbs`, `TestEveryVerbCarriesSummaryAndGate`, and the five
`TestScopes_*` tests.

## Diff Coverage

Diff computed as `git diff $(git merge-base origin/main HEAD)...HEAD`, base `2cce019`.

| File | +/- | Mapped Requirements |
|---|---|---|
| `internal/tools/move_message.go` | +127/-0 | FR-3, FR-4, FR-10, FR-11, FR-12, FR-13, NFR-1, NFR-4, NFR-6, NFR-7 |
| `internal/tools/set_flag.go` | +137/-0 | FR-5, FR-10, FR-11, FR-12, FR-13, NFR-1, NFR-4, NFR-6, NFR-7, NFR-8 |
| `internal/tools/set_categories.go` | +138/-0 | FR-6, FR-7, FR-8, FR-10, FR-11, FR-12, FR-13, NFR-1, NFR-4, NFR-6, NFR-7, NFR-8 |
| `internal/tools/mark_read.go` | +127/-0 | FR-9, FR-10, FR-11, FR-12, FR-13, NFR-1, NFR-4, NFR-6, NFR-7, NFR-8 |
| `internal/tools/mail_write_confirmation.go` | +52/-0 | FR-4, FR-6, FR-8, FR-12, FR-13 |
| `internal/graph/enums.go` | +28/-0 | FR-5 (Phase 1 `ParseFlagStatus`) |
| `internal/validate/validate.go` | +20/-0 | FR-5, NFR-4 (Phase 1 `ValidateFlagStatus`) |
| `internal/server/mail_verbs.go` | +154/-7 | FR-1, FR-2, FR-5, FR-9, FR-12, FR-14, FR-15, FR-16, FR-17, FR-18 |
| `internal/server/server.go` | +2/-1 | FR-19 |
| `extension/manifest.json` | +1/-1 | FR-20 |
| `site/src/generated/surface.json` | +26/-2 | FR-21, FR-26 |
| `docs/concepts.md` | +1/-1 | FR-24 |
| `docs/troubleshooting.md` | +19/-2 | NFR-4 (the move-destination failure mode, anchor `#move-destination-not-found`) |
| `docs/prompts/mcp-tool-crud-test.md` | +47/-1 | FR-22 |
| `internal/tools/dispatch_registry_test.go` | +4/-0 | FR-14, FR-23 |
| `internal/tools/move_message_test.go` | +226/-0 | AC-2, AC-7, AC-8, AC-24 |
| `internal/tools/set_flag_test.go` | +202/-0 | AC-3, AC-7, AC-8 |
| `internal/tools/set_categories_test.go` | +183/-0 | AC-4, AC-5, AC-8, AC-10 |
| `internal/tools/mark_read_test.go` | +152/-0 | AC-6, AC-8 |
| `internal/tools/mail_write_verbs_test.go` | +282/-0 | AC-8, AC-9, AC-22, AC-23, NFR-4, NFR-6 |
| `internal/tools/mail_write_confirmation_test.go` | +92/-0 | FR-4, FR-8, FR-13 |
| `internal/server/mail_verbs_test.go` | +248/-7 | FR-15, FR-16, FR-17, AC-12, AC-13, AC-14 |
| `internal/server/manifest_sync_test.go` | +123/-0 | FR-20, AC-16 |
| `internal/graph/enums_test.go` | +27/-0 | FR-5 |
| `internal/validate/validate_test.go` | +24/-0 | FR-5 |
| `internal/docs/catalog_test.go` | +41/-0 | FR-24, AC-20 |

### Unmapped changed files

Seven files in the branch diff carry no CR-0078 requirement. Six are the pre-existing
unrelated documentation commits the orchestrator identified (`cce4291`, `4367f74`), which
sit between the branch base and the CR-0078 work; none touches Go source, the extension
manifest, or the generated surface.

| File | +/- | Justification |
|---|---|---|
| `.agents/explore/2026-09-01-passthrough-auth-multi-tenant-serving.md` | +170/-0 | Commit `cce4291`, explore-cache entry unrelated to CR-0078 |
| `docs/cr/CR-0079-mail-draft-attachments.md` | +1119/-0 | Commit `4367f74`, sibling CR authored on the same branch, still `draft` |
| `docs/cr/CR-0080-calendar-scheduling-reads.md` | +1051/-0 | as above |
| `docs/cr/CR-0081-calendar-event-attachments.md` | +964/-0 | as above |
| `docs/cr/CR-0082-contacts-domain.md` | +1092/-0 | as above |
| `docs/cr/CR-0083-teams-domain.md` | +1105/-0 | as above |
| `docs/backlog/cr-0078-0083.md` | +159/-0 | Backlog record the CR's own review summary cites for the deferred FR-16 scoping decisions; governance artefact, not implementation |
| `docs/cr/CR-0078-email-management-write-verbs.md` | +1445/-0 | The CR itself |

One changed file is in scope but absent from the CR's Affected Components list:

| File | +/- | Justification |
|---|---|---|
| `internal/docs/catalog_test.go` | +41/-0 | Named in the CR's Test Strategy as the home of `TestMailGatingRowNamesMessageManagement` (grading AC-20) but omitted from Affected Components. A CR bookkeeping omission, not a scope violation. |

No source file outside the CR's Affected Components was changed. `scripts/crud-test.sh` and
`docs/bench/crud-runs.csv` are unchanged, which is what the CR predicted and which the
script's own accounting (`:115`, keying on the tool name, not the verb) and the CSV's
23-column per-domain header confirm.

## Gaps

Nine gaps. None is a FAIL: every requirement is implemented and mapped to a hunk. Eight are
missing or under-specified tests where the CR's Test Strategy named a test that was not
written, which leaves the corresponding behaviour ungraded; the ninth is a missing
acceptance artefact the CR requires by name.

**G1 — FR-10 / AC-7: malformed `message_id` is untested for `set_categories` and `mark_read`.**
The CR specified `TestInvalidMessageIDRejectedByEveryWriteVerb` in
`internal/tools/mail_write_verbs_test.go`, driving a *malformed* identifier at each of the
four. The written substitute, `TestMailWriteVerbs_RequireMessageID` (`:124`), deletes
`message_id` instead, so `validate.ValidateResourceID` is never reached; only
`TestMoveMessage_InvalidIdentifiersRejectedBeforeCall` and
`TestSetFlag_InvalidMessageIDRejectedBeforeCall` exercise it.
*Minimal fix:* add a table-driven test in `mail_write_verbs_test.go` that overrides
`message_id` with `strings.Repeat("a", validate.MaxResourceIDLen+1)` for each row of
`mailWriteVerbs()` and asserts `result.IsError` with `len(rec.methods) == 0`. The existing
`mailWriteVerb.requestFor(overrides)` helper already supports this in about fifteen lines.

**G2 — NFR-7 / AC-25: the timeout and redaction paths are ungraded for all four verbs.**
`TestWriteVerbsHonourTimeoutAndRedaction` was specified and does not exist. Nothing asserts
that the timeout message names the configured seconds, and nothing asserts
`RedactGraphError` strips a token-like string from any of the four. AC-25's first Given is
therefore ungraded, though the wiring is present and identical in all four handlers.
*Minimal fix:* add two table-driven tests over `mailWriteVerbs()`: one against an
`httptest` server that blocks past a short configured timeout, asserting the result contains
the seconds value; one against a server returning a Graph error carrying a bearer-token-like
string, asserting the token is absent from the result text. `delete_event_test.go:135` and
`cancel_meeting_test.go:197` already carry the timeout pattern to copy.

**G3 — NFR-4 / AC-24: the log channel of the move fix instruction is not asserted.**
`TestMoveMessage_UnresolvableDestinationCarriesFix` (`move_message_test.go:206`) checks only
the tool result. The CR's Test Strategy row for this test explicitly required "the handler's
logger bound to a capture buffer" and "the captured log record carries the same correction",
and the CR's review summary recorded this as contradiction 7, fixed in the document. The
implementation logs the fix (`move_message.go:101-103`), so only the assertion is missing.
This is the criterion whose entire purpose is that a headless caller cannot read the
interactive surface, so the untested channel is the one that matters.
*Minimal fix:* bind a `slog` handler writing to a `bytes.Buffer` into the context with
`logging.WithLogger` before invoking the handler, and assert the captured output contains
`moveFixInstruction`.

**G4 — FR-13 / AC-10: `set_flag` and `mark_read` cannot fail the "response, not arguments" criterion.**
`TestConfirmationsUseGraphResponseNotArguments` was specified and does not exist. The two
tests that do carry differing-value evidence cover `set_categories` and `move_message`. The
canned responses for `set_flag` and `mark_read` echo the request values, so those two
confirmations would pass whether they read the response or the arguments.
*Minimal fix:* in `set_flag_test.go` and `mark_read_test.go`, add a case whose canned
response reports a value the request did not ask for (request `flagged`, response
`complete`; request `is_read: true`, response `"isRead": false`) and assert the confirmation
states the response value.

**G5 — FR-12 / AC-9: no derived check that write verbs declare no `output` parameter.**
`TestWriteVerbsDeclareNoOutputParameter` was specified for
`internal/tools/verb_metadata_test.go`, which has zero diff. The CR's own review summary
listed this test under "VERIFIED, NO FIX REQUIRED" as passing on arrival, which was a claim
about a test that does not exist. AC-9's first clause therefore rests on reading the four
`Schema` blocks. This is the derived-check shape the project's own instructions prefer over
instance lists, and it is the one the CR asked for and did not get.
*Minimal fix:* add to `verb_metadata_test.go` a test iterating every domain's verbs under
the maximal configuration, materialising each verb's `Schema` into a throwaway
`mcp.NewTool`, and failing when a verb with `readOnlyHint: false` publishes an `output`
property. `mailVerbParams`/`mailVerbIsReadOnly` in `mail_verbs_test.go:117-141` are the
pattern.

**G6 — FR-14 / AC-11: no per-verb annotation assertion alongside the existing annotation tests.**
`TestMailManagementVerbAnnotations` was specified for
`internal/tools/tool_annotations_test.go`, which has zero diff. The project's standing rule
in `CLAUDE.md` reads "New verbs **MUST** add a value assertion alongside the existing
annotation tests in `internal/tools/`". The four hint values *are* asserted, by
`verbInventoryGolden` in `internal/tools/dispatch_registry_test.go:73-77`, which is in the
same package, so the rule is arguably met and AC-11 is graded. The gap is that the CR named
a specific test in a specific file and it is absent.
*Minimal fix:* either add the four-verb value assertion to `tool_annotations_test.go`, or
amend the CR's Test Strategy to record the golden as the grading instrument. The first is
about twenty lines; the second is a documentation change.

**G7 — FR-1 / FR-2 / AC-1: neither the positive nor the negative registration test was written.**
`TestRegisterTools_MailManage_RegistersManagementVerbs` was specified as a new test in
`internal/server/server_test.go`, and `TestRegisterTools_MailEnabled` was specified for
modification to add the negative assertion. `server_test.go` has zero diff, so neither
happened. AC-1 is nonetheless graded, by the derived gate probe in `internal/surface/`
(which attributes all four to `OUTLOOK_MCP_MAIL_MANAGE_ENABLED` only because they are absent
under the `MailEnabled` probe) and by this report's live `tools/list` observation under both
configurations.
*Minimal fix:* add `TestRegisterTools_MailManage_RegistersManagementVerbs` asserting the
four appear in the mail tool's operation enum with `len(registered) == 4`, and extend
`TestRegisterTools_MailEnabled` (`:756`) with the complementary negative. The existing
`getRegisteredTool` helper supplies the enum.

**G8 — AC-12: no test invokes the four under read-only mode.**
`TestReadOnlyBlocksMailManagementVerbs` was specified for `internal/server/readonly_test.go`,
which has zero diff. The behaviour is correct and was verified for this report by driving
the built binary with `OUTLOOK_MCP_READ_ONLY=true`, which returned
`operation blocked: mail.<verb> is not allowed in read-only mode` for each of the four. That
observation is not reproducible in CI.
*Minimal fix:* add a test to `readonly_test.go` building the mail verbs with
`readOnly: true` and asserting each of the four handlers returns the refusal naming its
`mail.<verb>` identity. `TestMailManagementVerbsCarryDotIdentity` (`mail_verbs_test.go:259`)
already builds the verbs this way and is nine lines from being the test.

**G9 — CR acceptance: no user scenario is persisted.**
The CR's Verification Commands section states that "Acceptance additionally requires a user
scenario driving the built server against a live mailbox: flag a message, label it, mark it
read, move it, then confirm through `get_message` that the new identifier resolves and the
original does not. Persist the scenario under `.agents/scenarios/`". The directory
`.agents/scenarios/` does not exist in the repository. `make crud-test` was also not run for
this change, so no evidence exists that any of the four verbs has issued a real Graph call.
Every passing test in this report uses an `httptest` server and canned JSON.
*Minimal fix:* run the `user-scenario` skill against the four verbs per the CR's wording and
persist the artefact under `.agents/scenarios/`, or amend the CR to withdraw the requirement
on the record. Note the harness caveat in `CLAUDE.md`: rebuild the binary at the path
`.mcp.json` names and check the report's own `Server version` line against
`git rev-parse --short HEAD` before trusting any row of it.

### Not gaps, recorded so they are not rediscovered

* The `mail` aggregate tool does publish an `output` property in its flattened schema. That
  is the union of the read verbs' declarations, not a violation of FR-12, which constrains
  the four write verbs' own `Schema` blocks. Verified: none of the four declares it.
* `internal/tools/tool_annotations_test.go`, `internal/tools/verb_metadata_test.go`,
  `internal/server/server_test.go`, and `internal/server/readonly_test.go` all have zero diff
  on this branch. Four of the nine gaps above are exactly that fact, seen from four
  different criteria.
* The CR's stale-`long_description` note in `extension/manifest.json:7` (naming `Mail.Read`
  where `Mail.ReadWrite` is requested) is explicitly out of scope and is correctly still
  present. Not a gap against this CR.
