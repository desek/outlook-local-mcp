# CR-0078 Validation Report

Validated 2026-09-01 against `HEAD` `c8544d1` on branch `docs/cr-implementation-set-0079-0083`.
Branch base (`git merge-base origin/main HEAD`): `2cce019`. Implementation commits:
`f3357a3`, `86fc06e`, `5f5b387`, `cdc05e6`, `cfb9aa3`, `0d768df`, plus the finalization
checkpoint `c8544d1`.

## Summary

Requirements: 34/34 | Acceptance Criteria: 25/25 | Tests: 36/36 | Gaps: 0 open, 1 deferred

**Gap fix pass, 2026-09-01.** Every gap this report opened has been closed by adding the
test the CR's Test Strategy named, except G9, which needs a live mailbox and a paid harness
run and is therefore deferred to the user on the record. No production code changed in the
fix pass: each of the eight closed gaps was a missing assertion over behaviour that was
already implemented, which is why the pass adds tests only. The rows below carry their
post-fix status; the Gaps section records what each fix was.

No FAIL, at the audit or after the fix pass. Every functional and non-functional
requirement is implemented and maps to a changed file with a specific hunk.

The audit downgraded five requirements and five acceptance criteria to PARTIAL: the
behaviour they assert was implemented but had no test grading it, and in every one of those
ten cases the CR's Test Strategy named a test that was not written. All ten now carry the
named test and read PASS. The one thing the suite still cannot show is that any of the four
verbs has issued a real Microsoft Graph call, which is G9 and is deferred to the user.

### Check pipeline

| Command | Result |
|---|---|
| `make ci` | **pass**, exit 0 (build, vet, tidy, `golangci-lint` 0 issues, `go test ./...`, docs-bundle, surface drift check, `goreleaser check`, `mcpb validate`) |
| `make ci` surface regeneration | working tree unchanged afterwards (`git status --porcelain` empty), so the committed `site/src/generated/surface.json` matches the live registry |
| `go test -race ./internal/tools/ ./internal/server/ ./internal/graph/ ./internal/validate/ -run 'MoveMessage\|SetFlag\|SetCategories\|MarkRead\|Confirmation\|SharedParameters\|Manifest\|VerbInventory'` (CR Verification Commands) | **pass**, 47 tests, 0 failures |
| `go test -race ./internal/docs/ ./internal/surface/ -run 'MailGatingRow\|CommittedManifest\|RecordCounts\|DefaultCount\|EveryVerbCarries'` | **pass**, 4 tests |
| `go test -race ./internal/tools/ ./internal/server/ -run 'MailAnnotations\|AggregateAnnotations\|EveryVerbHas\|SeeDocs\|DescriptionLengthBounded\|ColdStart\|RegisterTools_Mail\|ReadOnlyGuard'` | **pass**, 30 tests |
| `make crud-test` | **not run**, in the audit or in the gap-fix pass (drives a live mailbox and costs a paid agent run). Recorded as pending for the user; see G9. |
| `make ci`, gap-fix pass | **pass**, exit 0; working tree clean after the surface manifest regenerated |
| `go test -race ./internal/tools/ ./internal/server/`, gap-fix pass | **pass** |

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
| FR-10 | All four validate `message_id` with `ValidateResourceID` before any Graph request | PASS | Implemented in all four: `move_message.go:67`, `set_flag.go:53`, `set_categories.go:56`, `mark_read.go:53`. Graded for all four by `TestMailWriteVerbs_MalformedMessageIDRejectedByEveryVerb` (`mail_write_verbs_test.go:162`), which drives an over-length identifier at each row of `mailWriteVerbs()` and asserts the refusal with `len(rec.methods) == 0`; plus the two per-handler tests `TestMoveMessage_InvalidIdentifiersRejectedBeforeCall` (`:146`) and `TestSetFlag_InvalidMessageIDRejectedBeforeCall` (`set_flag_test.go:180`). G1 fixed. |
| FR-11 | None of the four applies the `isDraft` guard | PASS | No `verifyIsDraft` call in any of the four handlers; `TestMoveMessage_AcceptsNonDraftMessage` (`:180`), `TestSetFlag_AcceptsNonDraftMessage` (`set_flag_test.go:148`), `TestSetCategories_AcceptsNonDraftMessage` (`set_categories_test.go:161`), `TestMarkRead_AcceptsNonDraftMessage` (`mark_read_test.go:130`); `TestMailWriteVerbs_SingleGraphRequestOnSuccess` (`mail_write_verbs_test.go:97`) proves no preceding GET |
| FR-12 | Unconditional text confirmation; no `output` parameter declared | PASS | No `output` in any of the four `Schema` blocks (`mail_verbs.go:632-646`, `:667-681`, `:701-715`, `:735-749`); every handler returns `mcp.NewToolResultText` on success. The derived cross-domain check is now present: `TestWriteVerbsDeclareNoOutputParameter` (`verb_metadata_test.go:354`) iterates every domain's verbs under the maximal configuration and fails when a verb declaring `readOnlyHint: false` publishes an `output` property. G5 fixed. |
| FR-13 | Confirmations built from the Graph response, not the request arguments | PASS | Implemented in all four (`move_message.go:107-121`, `set_flag.go:102-109` via `responseFlagStatus:128`, `set_categories.go:105-113` via `responseCategories:133`, `mark_read.go:94-101` via `responseReadState:119`). Falsifiable evidence now exists for all four: `TestSetCategories_ConfirmationListsResult`, `TestMoveMessage_ConfirmationNamesAllThreeIdentifiers`, and the two added disagreeing-response cases `TestSetFlag_ConfirmationStatesResponseNotRequest` (`set_flag_test.go:209`, request `flagged` / response `complete`) and `TestMarkRead_ConfirmationStatesResponseNotRequest` (`mark_read_test.go:159`, request `is_read: true` / response `isRead: false`). G4 fixed. |
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
| NFR-4 | Every error carries a fix instruction reaching both the tool result and the log record | PASS | Both channels implemented: `move_message.go:99-104` (result text plus `"fix", moveFixInstruction` on the log record), `set_flag.go:91-96`, `set_categories.go:94-99`, `mark_read.go:83-88`. Both channels graded: the tool result by `TestMailWriteVerbs_GraphFailureCarriesFix` (`mail_write_verbs_test.go:317`), and the log record by `TestMoveMessage_UnresolvableDestinationCarriesFix` (`move_message_test.go:214`), which now swaps `slog.Default()` for a buffer-backed handler and asserts the captured output carries `moveFixInstruction`. G3 fixed. |
| NFR-5 | No third-party dependency added | PASS | `git diff 2cce019..HEAD -- go.mod go.sum` is empty; `go mod tidy` in `make ci` leaves the tree clean |
| NFR-6 | Exactly one Graph request per success path; no read-modify-write | PASS | `TestMailWriteVerbs_SingleGraphRequestOnSuccess` (`mail_write_verbs_test.go:97`) asserts exactly one request of the expected method for each of the four |
| NFR-7 | Route through `RetryGraphCall` and `WithTimeout`; redact Graph errors with the existing helpers | PASS | Wiring present and identical across the four: `move_message.go:83-105`, `set_flag.go:75-97`, `set_categories.go:78-100`, `mark_read.go:67-89` each call `graph.WithTimeout`, `graph.RetryGraphCall`, `graph.IsTimeoutError`, `graph.TimeoutErrorMessage`, `graph.RedactGraphError`. Both paths now graded across all four: `TestMailWriteVerbs_TimeoutNamesConfiguredSeconds` (`mail_write_verbs_test.go:194`) configures a 7s deadline, so the assertion fails on a hardcoded 30s message rather than passing by coincidence; `TestMailWriteVerbs_GraphErrorIsRedacted` (`:224`) serves a Graph error carrying an address and asserts it is absent from the result and replaced by the redaction placeholder. G2 fixed. |
| NFR-8 | The three property writes are deterministic and idempotent | PASS | `TestMailWriteVerbs_PropertyWritesAreIdempotent` (`mail_write_verbs_test.go:253`) asserts identical request bodies and identical confirmation text on repeat, for the three, excluding `move_message` |

## Acceptance Criteria Verification

| AC # | Description | Status | Evidence |
|---|---|---|---|
| AC-1 | The four register, and only behind the manage gate | PASS | Live `tools/list`: under `MAIL_MANAGE_ENABLED` the enum holds all four and exactly 4 tools are registered; under `MAIL_ENABLED`-only none is present. `TestEveryVerbCarriesSummaryAndGate`, `TestDefaultCountExcludesGatedVerbs`, `TestManifestDescribesEveryRegisteredVerb`, `TestAggregateAnnotations_FourToolsRegistered`. Both CR-named tests now exist: `TestRegisterTools_MailManage_RegistersManagementVerbs` (`internal/server/server_test.go:890`) asserts all four are in the published operation enum with the tool count still 4, and `TestRegisterTools_MailEnabled` (`:793`) carries the complementary negative. G7 fixed. |
| AC-2 | A move surfaces the identifier the caller cannot otherwise see | PASS | `TestMoveMessage_Success` (`move_message_test.go:33`), `TestMoveMessage_ConfirmationNamesAllThreeIdentifiers` (`:63`); `move_message.go:113-121` renders the new ID, the destination, and the sentence that the original no longer resolves |
| AC-3 | The follow-up flag is written, and only with an accepted status | PASS | `TestSetFlag_Success` (`set_flag_test.go:55`) asserts `"flagStatus":"flagged"` in the PATCH body and the status in the confirmation; `TestSetFlag_RejectsUnknownStatus` (`:90`) asserts refusal naming the three values with no request issued; `TestValidateFlagStatus` (`validate_test.go:438`), `TestParseFlagStatus` (`enums_test.go:105`) |
| AC-4 | Categories are replaced as a set, and the result is stated | PASS | `TestSetCategories_ReplacesFullSet` (`set_categories_test.go:18`), `TestSetCategories_ConfirmationListsResult` (`:47`), `TestSetCategories_OverLengthRejected` (`:105`) |
| AC-5 | An empty categories value clears every category | PASS | `TestSetCategories_EmptyValueClears` (`set_categories_test.go:76`) |
| AC-6 | The read state is written in both directions | PASS | `TestMarkRead_SetsRead` (`mark_read_test.go:16`), `TestMarkRead_SetsUnread` (`:49`), `TestMarkRead_RequiresIsRead` (`:78`) |
| AC-7 | An invalid message identifier never reaches Microsoft Graph | PASS | `TestMailWriteVerbs_MalformedMessageIDRejectedByEveryVerb` (`mail_write_verbs_test.go:162`) drives a *malformed* identifier at all four, which is what reaches `ValidateResourceID`; `TestMailWriteVerbs_RequireMessageID` (`:212`) covers the *missing* case separately. G1 fixed. |
| AC-8 | A received message is not refused for being a non-draft | PASS | Four per-handler tests: `TestMoveMessage_AcceptsNonDraftMessage`, `TestSetFlag_AcceptsNonDraftMessage`, `TestSetCategories_AcceptsNonDraftMessage`, `TestMarkRead_AcceptsNonDraftMessage`; plus `TestMailWriteVerbs_SingleGraphRequestOnSuccess` proving no draft-check GET precedes the write |
| AC-9 | Each verb is a write verb by the project's tiering rule | PASS | The text-confirmation half is graded for all four by `TestMailWriteVerbs_ConfirmationNamesSubjectAndIdentifier` (`mail_write_verbs_test.go:269`). The derived half is graded by `TestWriteVerbsDeclareNoOutputParameter` (`verb_metadata_test.go:354`), whose cases come from the live verb sets rather than a list, so a write verb added later is covered without anyone extending it. Instrument validated by inversion: run against read-only verbs instead, it reports 18 verbs declaring `output`, so it detects the property it looks for. G5 fixed. |
| AC-10 | Confirmations report the service, not the request | PASS | Proven with a differing response for all four: `TestSetCategories_ConfirmationListsResult`, `TestMoveMessage_ConfirmationNamesAllThreeIdentifiers`, `TestSetFlag_ConfirmationStatesResponseNotRequest` (`set_flag_test.go:209`), `TestMarkRead_ConfirmationStatesResponseNotRequest` (`mark_read_test.go:159`). The two added cases each assert both halves: the response value is stated and the requested value is absent. G4 fixed. |
| AC-11 | Every hint is declared, and matches the matrix | PASS | `verbInventoryGolden` (`dispatch_registry_test.go:73-77`) asserts all four hints per verb against the matrix; live folded `mail` annotations are `false/true/false/true` under manage and `true/false/true/true` under read-only, unchanged from before this change (`TestMailAnnotationsManageEnabled`, `TestMailAnnotationsGatedReadOnly`). The CR-named `TestMailManagementVerbAnnotations` now exists (`internal/tools/tool_annotations_test.go:450`) and asserts the four per-verb hint values directly, which the aggregate fold cannot: three of the four declare `destructiveHint: false`, invisible in an aggregate folding to true. G6 fixed. |
| AC-12 | Read-only mode blocks all four, and the identity is consistent | PASS | Second clause graded by `TestMailManagementVerbsCarryDotIdentity` (`mail_verbs_test.go:259`), which reads the emitted audit record and asserts `tool_name` is `mail.<verb>` with `operation_type` `write`. First clause verified behaviourally: driving the built binary with `OUTLOOK_MCP_READ_ONLY=true` returns `operation blocked: mail.<verb> is not allowed in read-only mode` for each of the four. That observation is now reproducible in CI: `TestReadOnlyBlocksMailManagementVerbs` (`internal/server/readonly_test.go:151`) builds the mail verbs with `readOnly: true` and invokes each of the four through its own middleware chain, asserting the refusal names both read-only mode and the `mail.<verb>` identity. G8 fixed. |
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
| AC-24 | A failure carries its correction on both channels | PASS | Both channels graded by `TestMoveMessage_UnresolvableDestinationCarriesFix` (`move_message_test.go:214`): the tool result names `list_folders`, and the test now installs a buffer-backed `slog` default for the call and asserts the captured record carries `moveFixInstruction`. The log channel is the one that matters here, because it is the only channel a headless caller reading a persisted log has. G3 fixed. |
| AC-25 | The implementation follows the project's file and helper conventions | PASS | The file-layout clause: four handlers, four files, named for the verbs. The error-handling clause is now graded by `TestMailWriteVerbs_TimeoutNamesConfiguredSeconds` (`mail_write_verbs_test.go:194`), which asserts the refusal names the configured 7s rather than a hardcoded value, and `TestMailWriteVerbs_GraphErrorIsRedacted` (`:224`), which asserts an address in the Graph error is replaced by the redaction placeholder. Both run over every row of `mailWriteVerbs()`. G2 fixed. |

## Test Strategy Verification

### Tests to Add

| Test File | Test Name | Specified | Exists | Matches Spec |
|---|---|---|---|---|
| `internal/tools/move_message_test.go` | `TestMoveMessage_Success` | yes | yes (`:33`) | yes |
| `internal/tools/move_message_test.go` | `TestMoveMessage_ConfirmationNamesNewID` | yes | renamed to `TestMoveMessage_ConfirmationNamesAllThreeIdentifiers` (`:63`) | yes |
| `internal/tools/move_message_test.go` | `TestMoveMessage_RequiresDestination` | yes | renamed to `TestMoveMessage_RequiresDestinationFolderID` (`:89`) | yes |
| `internal/tools/move_message_test.go` | `TestMoveMessage_InvalidMessageIDRejectedBeforeCall` | yes | renamed to `TestMoveMessage_InvalidIdentifiersRejectedBeforeCall` (`:146`) | yes, and broader (covers both identifiers) |
| `internal/tools/mail_write_verbs_test.go` | `TestInvalidMessageIDRejectedByEveryWriteVerb` | yes | added as `TestMailWriteVerbs_MalformedMessageIDRejectedByEveryVerb` (`:162`) | yes — drives an over-length identifier at every row of `mailWriteVerbs()`; `TestMailWriteVerbs_RequireMessageID` (`:212`) keeps the missing-identifier case (G1 fixed) |
| `internal/tools/mail_write_verbs_test.go` | `TestNoDraftGuardOnReceivedMessageWrites` | yes | as four per-handler `*_AcceptsNonDraftMessage` tests | yes |
| `internal/tools/mail_write_verbs_test.go` | `TestWriteVerbsHonourTimeoutAndRedaction` | yes | added as two tests, `TestMailWriteVerbs_TimeoutNamesConfiguredSeconds` (`:194`) and `TestMailWriteVerbs_GraphErrorIsRedacted` (`:224`) | yes — split so a timeout failure and a redaction failure are distinguishable rather than reported as one red test (G2 fixed) |
| `internal/tools/move_message_test.go` | `TestMoveMessage_UnresolvableDestinationCarriesFix` | yes | yes (`:214`) | yes — now installs a buffer-backed `slog` default for the call and asserts the captured record carries `moveFixInstruction`, alongside the tool-result assertion (G3 fixed) |
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
| `internal/tools/mail_write_verbs_test.go` | `TestConfirmationsUseGraphResponseNotArguments` | yes | added per handler: `TestSetFlag_ConfirmationStatesResponseNotRequest` (`set_flag_test.go:209`) and `TestMarkRead_ConfirmationStatesResponseNotRequest` (`mark_read_test.go:159`), joining the two existing cases | yes — placed per handler rather than in the cross-verb table because the disagreeing response differs per verb and the shared table carries one canned response per row (G4 fixed) |
| `internal/tools/mail_write_verbs_test.go` | `TestStateSetVerbsAreIdempotent` | yes | renamed to `TestMailWriteVerbs_PropertyWritesAreIdempotent` (`:253`) | yes |
| `internal/tools/mail_write_verbs_test.go` | `TestSingleGraphRequestPerWrite` | yes | renamed to `TestMailWriteVerbs_SingleGraphRequestOnSuccess` (`:97`) | yes |
| `internal/tools/tool_annotations_test.go` | `TestMailManagementVerbAnnotations` | yes | yes (`:450`) | yes — asserts all four hints per verb against the matrix, alongside the aggregate fold tests (G6 fixed) |
| `internal/tools/verb_metadata_test.go` | `TestWriteVerbsDeclareNoOutputParameter` | yes | yes (`:354`) | yes — cases derived from the live verb sets under the maximal configuration, with a guard failing when the selection is empty (G5 fixed) |
| `internal/server/mail_verbs_test.go` | `TestSharedParametersNameTheirWriteVerbs` | yes | yes (`:161`) | yes, cases derived from the registry as specified |
| `internal/server/mail_verbs_test.go` | `TestDestinationFolderIDIsNotFolderID` | yes | yes (`:224`) | yes |
| `internal/server/manifest_sync_test.go` | `TestManifestDescribesEveryRegisteredVerb` | yes | yes (`:84`) | yes |
| `internal/server/server_test.go` | `TestRegisterTools_MailManage_RegistersManagementVerbs` | yes | yes (`:890`) | yes — reads the published operation enum via the added `registeredOperations` helper and re-asserts the tool count stays 4 (G7 fixed) |
| `internal/server/readonly_test.go` | `TestReadOnlyBlocksMailManagementVerbs` | yes | yes (`:151`) | yes — invokes each verb through its own middleware chain with `readOnly: true` and asserts the refusal names read-only mode and the `mail.<verb>` identity (G8 fixed) |
| `internal/server/mail_verbs_test.go` | `TestMailManagementVerbsCarryDotIdentity` | yes | yes (`:259`) | yes |
| `internal/docs/catalog_test.go` | `TestMailGatingRowNamesMessageManagement` | yes | yes (`:56`) | yes |

### Tests to Modify

| Test File | Test Name | Specified | Exists | Matches Spec |
|---|---|---|---|---|
| `internal/tools/dispatch_registry_test.go` | `TestVerbInventoryUnchangedAfterUpgrade` | golden 42 -> 46, four added lines | yes (`:73-77`) | yes, exactly the four specified strings |
| `internal/server/server_test.go` | `TestRegisterTools_MailEnabled` | add a negative assertion that none of the four is present | modified (`:793`) | yes — the negative reads the same `mailManagementVerbs()` list as the positive test, so the pair cannot drift apart (G7 fixed) |
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

Added in the gap-fix pass, tests only:

| File | Mapped Gap |
|---|---|
| `internal/tools/mail_write_verbs_test.go` | G1, G2 |
| `internal/tools/move_message_test.go` | G3 |
| `internal/tools/set_flag_test.go`, `internal/tools/mark_read_test.go` | G4 |
| `internal/tools/verb_metadata_test.go` | G5 |
| `internal/tools/tool_annotations_test.go` | G6 |
| `internal/server/server_test.go` | G7 |
| `internal/server/readonly_test.go` | G8 |
| `.agents/scenarios/2026-09-01-received-message-management-lifecycle.md`, `docs/backlog/cr-0078-0083.md` | G9 (deferred; the artefact and the pending-run record) |

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

Nine gaps were opened by the audit. Eight are now **FIXED**; one, G9, is **DEFERRED** to
the user because it needs a live mailbox and a paid harness run. None was ever a FAIL:
every requirement was implemented and mapped to a hunk, and eight of the nine were missing
assertions over behaviour that already worked. The fix pass therefore changed no production
code, only tests, plus the persisted scenario and the backlog entry for G9.

The fix pass re-ran `make ci` (exit 0, `golangci-lint` 0 issues, surface manifest
regenerated with the working tree left clean) and `go test -race ./internal/tools/
./internal/server/` (pass).

**G1 — FIXED. FR-10 / AC-7: malformed `message_id` was untested for `set_categories` and `mark_read`.**
The written substitute deleted the argument instead of malforming it, so
`validate.ValidateResourceID` was never reached.
*Fix:* `TestMailWriteVerbs_MalformedMessageIDRejectedByEveryVerb`
(`internal/tools/mail_write_verbs_test.go:162`) overrides `message_id` with
`strings.Repeat("a", validate.MaxResourceIDLen+1)` for every row of `mailWriteVerbs()` and
asserts the refusal names the parameter with `len(rec.methods) == 0`. Over-length is the
malformation used because emptiness and length are what `ValidateResourceID` rejects; it
does not bound the character set. The missing-argument test is retained beside it, since
the two failures take different paths.

**G2 — FIXED. NFR-7 / AC-25: the timeout and redaction paths were ungraded for all four verbs.**
*Fix:* two table-driven tests over `mailWriteVerbs()`, split rather than combined so a
timeout regression and a redaction regression are distinguishable.
`TestMailWriteVerbs_TimeoutNamesConfiguredSeconds` (`:194`) configures a **7s** deadline,
not the 30s every other test uses, so the assertion fails on a hardcoded message rather
than passing by coincidence. `TestMailWriteVerbs_GraphErrorIsRedacted` (`:224`) serves a
Graph error carrying an address and asserts it is absent from the result and replaced by
`[email redacted]`. The redaction assertion is written against an address rather than a
bearer token because `graph.RedactGraphError` (`internal/graph/errors.go:131`) redacts
addresses only; a token assertion would have failed while reporting nothing about the
helper the requirement names.

**G3 — FIXED. NFR-4 / AC-24: the log channel of the move fix instruction was not asserted.**
*Fix:* `TestMoveMessage_UnresolvableDestinationCarriesFix` (`internal/tools/move_message_test.go:214`)
now installs a `slog` handler writing to a `bytes.Buffer` as the default for the duration
of the call and asserts the captured output contains `moveFixInstruction`, alongside the
existing tool-result assertion. The handler reads its logger through `logging.Logger(ctx)`,
which derives from `slog.Default()`; there is no `WithLogger` injection point, so swapping
the default and restoring it is the available capture. No test in `internal/tools` calls
`t.Parallel`, and the package passes under `-race`.

**G4 — FIXED. FR-13 / AC-10: `set_flag` and `mark_read` could not fail the "response, not arguments" criterion.**
*Fix:* a disagreeing-response case per handler.
`TestSetFlag_ConfirmationStatesResponseNotRequest` (`set_flag_test.go:209`) requests
`flagged` against a response reporting `complete`;
`TestMarkRead_ConfirmationStatesResponseNotRequest` (`mark_read_test.go:159`) requests
`is_read: true` against a response reporting `isRead: false`. Each asserts both halves: the
response value is stated and the requested value is absent. They sit per handler rather
than in `mail_write_verbs_test.go` because the shared table carries one canned response per
row, and the disagreement has to differ per verb.

**G5 — FIXED. FR-12 / AC-9: no derived check that write verbs declare no `output` parameter.**
*Fix:* `TestWriteVerbsDeclareNoOutputParameter` (`internal/tools/verb_metadata_test.go:354`)
iterates every domain's verbs from `server.BuildDomainVerbSets` under the maximal
configuration, materialises each verb's own `Schema` onto a throwaway `mcp.NewTool`, and
fails when a verb declaring `readOnlyHint: false` publishes an `output` property. A guard
fails the test when the selection is empty, so a broken derivation cannot pass as a clean
run. This is the derived-check shape the project's instructions prefer over instance lists.
*Instrument validated:* inverted to select read-only verbs instead, it reports **18**
verbs declaring `output`, so it detects the property it looks for rather than passing
because it finds nothing.

**G6 — FIXED. FR-14 / AC-11: no per-verb annotation assertion alongside the existing annotation tests.**
*Fix:* `TestMailManagementVerbAnnotations` (`internal/tools/tool_annotations_test.go:450`)
asserts all four hints for each of the four verbs against the matrix. It earns its place
beside the aggregate fold tests rather than duplicating `verbInventoryGolden`, because a
fold is lossy: three of these four declare `destructiveHint: false`, and that value is
invisible in an aggregate folding to true because of `move_message`.

**G7 — FIXED. FR-1 / FR-2 / AC-1: neither the positive nor the negative registration test was written.**
*Fix:* `TestRegisterTools_MailManage_RegistersManagementVerbs`
(`internal/server/server_test.go:890`) asserts all four appear in the mail tool's published
operation enum and that the tool count is still 4, so the surface grows by verbs and not by
tools. `TestRegisterTools_MailEnabled` (`:793`) gains the complementary negative. Both read
the verb names from one `mailManagementVerbs()` helper and the enum through one
`registeredOperations()` helper, so the positive and negative halves of the gate cannot
drift apart.

**G8 — FIXED. AC-12: no test invoked the four under read-only mode.**
*Fix:* `TestReadOnlyBlocksMailManagementVerbs` (`internal/server/readonly_test.go:151`)
builds the mail verbs with `readOnly: true` and invokes each of the four through its own
registered middleware chain, asserting the refusal names read-only mode and the
`mail.<verb>` identity. Asserting at the chain rather than at `ReadOnlyGuard` alone is the
point: a write verb wired through the read wrapper would pass every existing
`ReadOnlyGuard` unit test in that file while executing in read-only mode.

**G9 — DEFERRED, and named as pending for the user. CR acceptance: no live-mailbox run.**
The CR requires a user scenario driving the built server against a live mailbox, plus a
`make crud-test` run. Both need an authenticated mailbox and a paid harness run, so neither
was performed and neither may be reported as performed.

What was done instead, so the run costs only its own price when it happens:

* The scenario is derived and persisted at
  `.agents/scenarios/2026-09-01-received-message-management-lifecycle.md`, carrying the
  goal, preconditions, the ten steps at the MCP surface, the success condition, and the
  restoration procedure. Its frontmatter reads `outcome: not-run` and
  `runs: "0 of 0 attempted"`, and a Status section states plainly that nothing in it has
  been observed. Whoever performs the first run rewrites those fields and appends the run
  log.
* The pending run is recorded as a bullet under the CR-0078 heading in
  `docs/backlog/cr-0078-0083.md`, with the harness caveats it has to be run under: rebuild
  the binary at the path `.mcp.json` names, and check the report's own server version line
  against `git rev-parse --short HEAD` before trusting a row of it.

The standing consequence, stated so it is not lost: **no evidence yet exists that any of
the four verbs has issued a real Microsoft Graph call.** Every passing test in this report
drives an `httptest` server with canned JSON. That is a limit on what this report proves,
not a defect in the implementation.

### Not gaps, recorded so they are not rediscovered

* The `mail` aggregate tool does publish an `output` property in its flattened schema. That
  is the union of the read verbs' declarations, not a violation of FR-12, which constrains
  the four write verbs' own `Schema` blocks. Verified: none of the four declares it.
* `internal/tools/tool_annotations_test.go`, `internal/tools/verb_metadata_test.go`,
  `internal/server/server_test.go`, and `internal/server/readonly_test.go` all had zero diff
  when the audit ran, and four of the nine gaps were exactly that fact seen from four
  different criteria. All four now carry the tests the CR named. Recorded because the shape
  is worth recognising next time: four separate criteria reading as ungraded is one missing
  edit, not four.
* The CR's stale-`long_description` note in `extension/manifest.json:7` (naming `Mail.Read`
  where `Mail.ReadWrite` is requested) is explicitly out of scope and is correctly still
  present. Not a gap against this CR.
