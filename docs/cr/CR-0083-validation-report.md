# CR-0083 Validation Report

Validated at `9a9ff99` on branch `docs/cr-implementation-set-0079-0083`.
Diff range: `85c6450..HEAD` (the CR-0083 slice; `85c6450` is the last CR-0082 commit).
Merge base with `origin/main`: `2cce019`.

## Summary

Requirements: 30/34 | Acceptance Criteria: 11/14 | Tests: 29/36 | Gaps: 8

Requirements counted as 27 Functional plus 7 Non-Functional. Tests counted as the
26 rows of "Tests to Add" plus the 10 rows of "Tests to Modify".

**Check pipeline:** `make ci` exits **0**. The working tree is clean afterwards
(`git status --porcelain` empty), which is the surface-manifest drift clause of AC-8.
Package results: every package `ok`, no `FAIL` line. Coverage: `internal/server` 93.9%,
`internal/tools` 78.8%, `internal/surface` 98.4%, `internal/auth` 75.3%, `internal/config` 90.9%.

**Measured schema gate (AC-9, NFR-3):** `TestColdStartSchemaSize_Reduction` logs
`tool registration complete tools=6`, `post-CR schema: 27413 bytes (6 tools)`,
`reduction: 62% (required >= 60%)`. `minRequiredReductionPct` is unchanged at 60 and
`preCRBaselineBytes` unchanged at 74,000 (`internal/server/schema_size_test.go:26,30`).
`go.mod` and `go.sum` are unchanged in the diff range (0 files).

**Not run:** `make crud-test` (paid harness) and the `site/AGENTS.md` screenshot
comparison. Both are recorded as gaps below and deferred to the user.

## Requirement Verification

### Functional Requirements

| Req # | Description | Status | Evidence (file:line / test name) |
|---|---|---|---|
| FR-1 | Register a `teams` aggregate tool with exactly thirteen verbs | PASS | `internal/server/server.go:235-256`; `internal/server/teams_verbs.go:128-142`; tests `TestTeamsVerbsRegisterThirteen` (`internal/server/teams_verbs_test.go:77`), `TestRegisterTools_TeamsEnabled_RegistersSixthTool` — both pass |
| FR-2 | Registered only under `TeamsEnabled`, same condition in `BuildDomainVerbSets` | PASS | `internal/server/server.go:235`; `internal/server/introspect_verbs.go:36,138`; test `TestRegisterTools_TeamsDisabled_NoTeamsTool` passes |
| FR-3 | `TeamsEnabled` on `config.Config` from `OUTLOOK_MCP_TEAMS_ENABLED`, plus `EnvTeamsEnabled` const and inventory row | PASS | `internal/config/config.go:157-167,343`; `internal/config/inventory.go:45,95`; `site/src/generated/surface.json` config section carries `OUTLOOK_MCP_TEAMS_ENABLED`; test `TestLoadConfig_TeamsEnabledDefaultFalse` passes |
| FR-4 | Four delegated read scopes requested only when enabled | PASS | `internal/auth/auth.go:53,60,66,75,131`; test `TestScopes_TeamsEnabled` passes |
| FR-5 | No Teams send or write scope in any configuration | PASS | `internal/auth/auth.go:97-116` (doc comment states the property); test `TestScopes_NoTeamsSendEver` passes over every flag combination |
| FR-6 | All four hints declared explicitly with the matrix values | PARTIAL | Values are correct in source: `internal/server/teams_verbs.go:75-82` (twelve verbs, openWorld `true`), `internal/tools/help/verb.go:52-56` (`help`, openWorld `false`). Declaration of all four is graded by `TestEveryVerbHasClassification` (`internal/tools/verb_metadata_test.go:209` domain list includes `teams`). `readOnlyHint`/`destructiveHint` **values** graded by `TestTeamsExposesNoWriteVerb` (`internal/server/teams_verbs_test.go:156`). **No test asserts `idempotentHint` or `openWorldHint` values for any teams verb** — the specified `TestTeamsVerbAnnotations` does not exist. See GAP-2 |
| FR-7 | No verb writes/sends; `compose_reply` reads parent, returns text, posts nothing | PASS | `internal/tools/teams_compose_reply.go:108-149` (only a `Get`); description states the no-send property at `internal/server/teams_verbs.go:334`; tests `TestComposeReply_PostsNothing`, `TestComposeReply_ReadsTheParentItQuotes`, `TestComposeReply_StatesNotSent` pass |
| FR-8 | `search` posts `entityTypes: ["chatMessage"]`, requires non-empty query, hits carry identifiers | PASS | `internal/tools/teams_search.go:79-95,154`; tests `TestTeamsSearch_PostsChatMessageEntityType`, `TestTeamsSearch_RejectsEmptyQueryBeforeAnyRequest`, `TestTeamsSearch_LabelsHitsByCollection` pass |
| FR-9 | Chat reads via `/me/chats` builders with identifier validation; no single-chat read | PASS | `internal/tools/teams_list_chats.go`, `teams_list_chat_messages.go`, `teams_get_chat_message.go`, `teams_list_chat_message_replies.go`; tests `TestListChatMessages_RejectsMissingChatIDBeforeAnyRequest`, `TestGetChatMessage_RefusesEachIdentifierSeparately`, `TestTeamsRegistryExposesNoOutOfScopeVerb` (refuses `get_chat`) pass |
| FR-10 | Channel reads via `/teams/{id}/channels/{id}/messages` with validation | PASS | `internal/tools/teams_list_channel_messages.go`, `teams_get_channel_message.go`, `teams_list_channel_message_replies.go`; tests `TestGetChannelMessage_RefusesEachIdentifierSeparately`, `TestListChannelMessages_RefusesEachIdentifierSeparately`, `TestListChannelMessageReplies_RefusesEachIdentifierSeparately` pass |
| FR-11 | `get_online_meeting` requires exactly one identifier, resolves via `joinWebUrl` filter, no unfiltered list | PASS | `internal/tools/teams_get_online_meeting.go:182-209` (neither-or-both refused), `:230-233` (`Filter: joinWebUrl eq '...'`); tests `TestGetOnlineMeeting_ResolvesJoinURL`, `TestGetOnlineMeeting_RequiresOneIdentifier`, `TestGetOnlineMeeting_MissingIdentifierNamesTheCalendarVerb`, `TestGetOnlineMeeting_EmptyMatchIsStatedNotSilent` pass |
| FR-12 | `list_transcripts` by meeting id; `get_transcript` metadata plus preview, full WEBVTT only under raw | PASS | `internal/tools/teams_get_transcript.go:130` (`SerializeTranscript(..., outputMode == "raw")`); tests `TestGetTranscript_ContentOnlyUnderRaw`, `TestListTranscripts_RequiresMeetingID`, `TestListTranscripts_CarriesNoContentAtEitherTier` pass |
| FR-13 | Eleven reads declare `output`, checked by a registry-derived assertion; deliberate summary sets | PASS | `internal/server/teams_verbs.go:86-91` and per-verb `Schema`; `internal/tools/teams_serialize.go` (per-shape `SerializeSummary*`); test `TestTeamsReadVerbsDeclareOutput` (`internal/server/teams_verbs_test.go:193-209`, derives cases from the built slice and asserts the count is 11) passes. Test relocated from the specified `internal/tools/teams_output_test.go` and renamed |
| FR-14 | `get_chat_message`, `get_channel_message`, `get_transcript` preview by default and say `output=raw` | PASS | `internal/server/teams_verbs.go:216-217`, `:280-281`, `:400-401`; tests `TestGetChatMessage_DefaultPreviewsAndRawEscalates`, `TestGetChannelMessage_DefaultPreviewsAndRawEscalates`, `TestGetTranscript_ContentOnlyUnderRaw` pass |
| FR-15 | `compose_reply` declares no `output`, returns text unconditionally, description states not-sent | PASS | `internal/server/teams_verbs.go:334,342-349`; tests `TestComposeReply_DeclaresNoOutputParameter` (`internal/server/teams_verbs_test.go:180`), `TestComposeReply_StatesNotSent` pass |
| FR-16 | Every verb wrapped by auth, account resolution, observability, audit under `teams.<verb>` | PASS | `internal/server/teams_verbs.go:122-124`; test `TestTeamsVerbsCarryDomainQualifiedIdentity` (`internal/server/teams_verbs_test.go:227`) reads the written audit log and passes |
| FR-17 | Non-empty `Summary` ≤80 chars, `Description`, ≥1 `Examples`, ≥1 resolving `SeeDocs` | PASS | `internal/server/teams_verbs.go:150-416`; tests `TestEveryVerbHasSummary`, `TestEveryVerbHasDescription`, `TestSeeDocsAnchorsResolve`, `TestEveryVerbStatesRequiredParameters`, `TestEveryParameterHasDescription` all carry `teams` in their domain lists (`internal/tools/verb_metadata_test.go:102,209,235`; `internal/tools/description_quality_test.go:95,132,166`) and pass |
| FR-18 | Every error carries a fix on **both** the tool result and the log record; team/channel fix names `teams.search`; meeting fix names `get_online_meeting` | PARTIAL | Source satisfies both channels: e.g. `internal/tools/teams_get_channel_message.go:33-37` plus the handler's `logger.Error(..., "fix", ...)` and `mcp.NewToolResultError(fmt.Sprintf("%s: %s", ...))`; `internal/tools/teams_list_transcripts.go:32` and `teams_get_transcript.go:40` name `teams get_online_meeting`. **No test asserts the log-record half** — the specified `TestErrorFixReachesBothToolResultAndLog` does not exist and no teams test captures a `slog` record; the `*_GraphFailureCarriesFix` tests assert the tool-result half only. See GAP-1. Note the fix strings say "teams search" / "teams get_online_meeting" (the domain-plus-verb form used throughout this codebase) rather than the dotted `teams.search` |
| FR-19 | `teams` as the sixth `extension/manifest.json` entry; literal `5`→`6`; `maximalSurfaceConfig` gains the flag; `long_description` Teams line | PASS | `extension/manifest.json` `tools` array is `[calendar, mail, account, system, contacts, teams]`; `long_description` line 11 "Teams reading: search chat and channel messages…"; `internal/server/manifest_sync_test.go:73` (`TeamsEnabled: true`) and the count literal; test `TestManifestDescribesEveryRegisteredVerb` passes |
| FR-20 | Surface manifest regenerated; `build.go` gains `domainOrder`, both configs, gate probe | PASS | `internal/surface/build.go:31,44,77,84`; `site/src/generated/surface.json` records `teams` 13 full / 0 default with every verb gated on `OUTLOOK_MCP_TEAMS_ENABLED`; totals 70/38; test `TestCommittedManifestMatchesRecord` passes and `make ci` leaves the tree clean |
| FR-21 | `toolCount` incremented inside the gated branch; base literal untouched | PASS | `internal/server/server.go:205` (`toolCount := 4`), `:225`, `:255`; the schema-size run logs `tool registration complete tools=6` |
| FR-22 | Schema gate extended with `TeamsEnabled`, reduction ≥60%, constants unchanged | PASS | `internal/server/schema_size_test.go:67`; measured 27,413 bytes / 62%; `minRequiredReductionPct` still 60 (`:30`), `preCRBaselineBytes` still 74,000 (`:26`) |
| FR-23 | `docs/concepts.md` Teams gating section, **two `OUTLOOK_MCP_TEAMS_ENABLED` rows in the "OAuth scopes used per feature" table**, and no stale tool-set statement | FAIL | Gating section present and correct (`docs/concepts.md:80-91`); stale statements corrected (`:95`, `:189`, `:141`). **The "OAuth scopes used per feature" table (`docs/concepts.md:131-140`) has no `OUTLOOK_MCP_TEAMS_ENABLED` row.** The contacts precedent carries both of its rows there (`:137-138`); the Teams analogue is absent, so the canonical scopes table omits the four scopes the domain requests. See GAP-5 |
| FR-24 | CRUD prompt steps from Step 52 with a `config.features.teams_enabled` skip; `crud-test.sh` column/awk/jq; CSV header reset | PASS | `docs/prompts/mcp-tool-crud-test.md:790-858` (Steps 52-58, skip instruction at `:794`); `scripts/crud-test.sh:97` (`mcp_teams` column), `:118` (awk bucket), `:130,138` (jq argument); `docs/bench/crud-runs.csv` is header-only (1 line), historical rows reset |
| FR-25 | Every statement of the aggregate tool set updated | PASS | `teams` present in `AGENTS.md`, `README.md`, `docs/readme.md`, `docs/quickstart.md`, `docs/troubleshooting.md`, `docs/reference/architecture.md`, `extension/README.md`, `internal/docs/llmstxt.go`, `internal/tools/aggregate_annotations.go:2`, `site/src/surface.ts`; `site/src/surface.ts:82` `domainCount` still `surface.domains.length`, not a literal |
| FR-26 | No `Manage=3` capability introduced | PASS | Tests `TestTeamsRegistryExposesNoOutOfScopeVerb` (`internal/server/teams_verbs_test.go:121`, name and prefix exclusions) and `TestTeamsVerbsRegisterThirteen` pass |
| FR-27 | `system.status` reports `config.features.teams_enabled` | PARTIAL | Source present: `internal/tools/status.go:237-241` (field with the JSON tag) and `:341` (populated from `cfg.TeamsEnabled`). **No test**: `internal/tools/status_test.go` is unchanged in the diff range and `TestStatus_FeaturesGroup` (`:273-292`) asserts `read_only`, `mail_enabled`, `provenance_tag` only. The specified `TestStatus_ReportsTeamsEnabled` does not exist. See GAP-3 |

### Non-Functional Requirements

| Req # | Description | Status | Evidence |
|---|---|---|---|
| NFR-1 | One file per verb under `internal/tools/`; formatters appended to `text_format.go` | PASS | Twelve new handler files `internal/tools/teams_*.go` (largest 247 lines); `internal/tools/text_format.go` +390 lines; no `teams_text_format.go` exists in the diff |
| NFR-2 | Shared retry, timeout, and redaction helpers in every handler | PASS | All twelve handlers call `graph.RetryGraphCall`, `graph.WithTimeout`, and `graph.RedactGraphError` (`teams_get_transcript.go` calls retry twice, once per request) |
| NFR-3 | Max-configuration reduction ≥60% with constants unchanged | PASS | 27,413 bytes / 62% measured; both constants unchanged. No description shortening or budget amendment was required |
| NFR-4 | Composed tool description under 4,000 characters | PASS | `TestDescriptionLengthBounded` (`internal/tools/description_quality_test.go:153` domain list includes `teams`) passes |
| NFR-5 | No third-party dependency; `go.mod`/`go.sum` unchanged | PASS | `git diff 85c6450..HEAD --name-only -- go.mod go.sum` returns 0 files; new imports are `msgraph-sdk-go/search` and `msgraph-sdk-go/teams`, packages of the already-pinned module |
| NFR-6 | Minimum Graph requests per verb; `get_transcript` exactly two in every mode | PASS | `internal/tools/teams_get_transcript.go:105-124`; `TestGetTranscript_ContentOnlyUnderRaw` (`internal/tools/teams_get_transcript_test.go:98-108`) asserts `recorder.callCount() == 2` for both the `summary` and `raw` runs and checks the two paths in order. `TestListChats_IssuesOneRequest` covers the single-request case |
| NFR-7 | Deterministic and side-effect-free at the service | PASS | Every handler issues `GET`s except the `POST /search/query` retrieval; `TestComposeReply_PostsNothing` asserts no write reaches the test server; `TestTeamsExposesNoWriteVerb` asserts the classification |

## Acceptance Criteria Verification

| AC # | Description | Status | Evidence |
|---|---|---|---|
| AC-1 | Domain registers only when enabled; six tools when on, none when off | PASS | `TestRegisterTools_TeamsEnabled_RegistersSixthTool`, `TestRegisterTools_TeamsDisabled_NoTeamsTool` (`internal/server/server_test.go`, +86 lines) both pass |
| AC-2 | Search posts the chatMessage entity type; empty query refused pre-request | PASS | `TestTeamsSearch_PostsChatMessageEntityType`, `TestTeamsSearch_RejectsEmptyQueryBeforeAnyRequest`, `TestTeamsSearch_LabelsHitsByCollection` pass |
| AC-3 | Chat and channel reads with body escalation; channel read missing an identifier refused naming search | PASS | `TestGetChatMessage_DefaultPreviewsAndRawEscalates`, `TestGetChannelMessage_DefaultPreviewsAndRawEscalates`, `TestGetChannelMessage_RefusesEachIdentifierSeparately` pass; fix strings at `internal/tools/teams_get_channel_message.go:33-34` name the teams search hit |
| AC-4 | Draft-only reply sends nothing; text and description say so; no `output` parameter | PASS | All four clauses graded: `TestComposeReply_PostsNothing`, `TestComposeReply_StatesNotSent`, `TestComposeReply_DeclaresNoOutputParameter` pass |
| AC-5 | Transcript chain resolves by identifier; full content only under raw | PASS | `TestGetOnlineMeeting_ResolvesJoinURL`, `TestListTranscripts_ReadsTheNamedMeeting`, `TestGetTranscript_ContentOnlyUnderRaw` pass |
| AC-6 | Every verb read-only, non-destructive, idempotent; open-world except `help`; folded tool read-only; no send scope ever | PARTIAL | Scope clause PASS (`TestScopes_NoTeamsSendEver`). Read-only and non-destructive per verb PASS (`TestTeamsExposesNoWriteVerb`). **The idempotent and open-world clauses are unasserted, and the folded aggregate for `teams` is unasserted** — no `TestTeamsAggregateIsReadOnly` exists, unlike `TestContactsAggregateIsReadOnly` (`internal/tools/tool_annotations_test.go:303`). See GAP-2 |
| AC-7 | Consent surface changes only on opt-in | PASS | `TestScopes_TeamsEnabled`, `TestScopes_NoTeamsSendEver` pass; `internal/auth/auth.go:131` gates the four scopes |
| AC-8 | Tool count 6; `make ci` matches the committed surface manifest; manifest holds six entries; surface records 13 gated verbs | PASS | Schema-size run logs `tools=6`; `make ci` exit 0 with `git status --porcelain` empty; `extension/manifest.json` six entries; `site/src/generated/surface.json` teams 13/0, every verb gated on `OUTLOOK_MCP_TEAMS_ENABLED` |
| AC-9 | Schema gate passes at max configuration and the measurement is recorded | PASS | Measured 27,413 bytes / 62% ≥ 60%; recorded in `docs/backlog/cr-0078-0083.md:1290-1291`; `minRequiredReductionPct` unchanged; `go.mod`/`go.sum` unchanged |
| AC-10 | Registry metadata complete; eleven reads declare `output`, `compose_reply` and `help` do not | PASS | `TestEveryVerbHasSummary`, `TestEveryVerbHasDescription`, `TestSeeDocsAnchorsResolve`, `TestTeamsReadVerbsDeclareOutput`, `TestComposeReply_DeclaresNoOutputParameter` pass |
| AC-11 | Errors carry their correction on the tool result **and** the log record | PARTIAL | Tool-result half graded by the seven `*_GraphFailureCarriesFix` and `*_Refuses*` tests. **The log-record half is graded by nothing**: no teams test installs a capturing handler. Source does emit `"fix"` on the log record in every handler. See GAP-1 |
| AC-12 | No out-of-scope Teams capability; exactly thirteen verbs | PASS | `TestTeamsRegistryExposesNoOutOfScopeVerb`, `TestTeamsVerbsRegisterThirteen` pass |
| AC-13 | Every verb carries the `teams.<verb>` middleware identity | PASS | `TestTeamsVerbsCarryDomainQualifiedIdentity` derives its cases from the built slice, reads the audit log, and asserts `operation_type: read` — passes |
| AC-14 | Documentation, status, and harness surfaces state the domain that now exists | FAIL | Documentation clauses PASS: troubleshooting anchors at `docs/troubleshooting.md:253,270,285`; no file states a tool set omitting teams (FR-25 evidence); `site/src/surface.ts:82` `domainCount` still derived; CRUD prompt steps and skip instruction at `docs/prompts/mcp-tool-crud-test.md:790,794`; `scripts/crud-test.sh:97` header matches `docs/bench/crud-runs.csv:1`. **Two clauses fail:** the "two OAuth scope rows for `OUTLOOK_MCP_TEAMS_ENABLED`" are absent from the `docs/concepts.md` scopes table (GAP-5), and the `system.status` clause has no test (GAP-3) |

## Test Strategy Verification

### Tests to Add

| Test File | Test Name | Specified | Exists | Matches Spec |
|---|---|---|---|---|
| `internal/tools/teams_search_test.go` | `TestSearch_PostsChatMessageEntityType` | yes | yes, renamed `TestTeamsSearch_PostsChatMessageEntityType` (`:165`) | yes |
| `internal/tools/teams_search_test.go` | `TestSearch_RequiresQuery` | yes | yes, renamed `TestTeamsSearch_RejectsEmptyQueryBeforeAnyRequest` (`:206`) | yes |
| `internal/tools/teams_get_chat_message_test.go` | `TestGetChatMessage_BodyPreviewByDefault` | yes | yes, renamed `TestGetChatMessage_DefaultPreviewsAndRawEscalates` (`:106`) | yes |
| `internal/tools/teams_get_channel_message_test.go` | `TestGetChannelMessage_RequiresTeamAndChannel` | yes | yes, renamed `TestGetChannelMessage_RefusesEachIdentifierSeparately` (`:42`) | yes |
| `internal/tools/teams_compose_reply_test.go` | `TestComposeReply_PostsNothing` | yes | yes (`:41`) | yes |
| `internal/tools/teams_compose_reply_test.go` | `TestComposeReply_StatesNotSent` | yes | yes (`:99`) | yes |
| `internal/tools/teams_compose_reply_test.go` | `TestComposeReply_DeclaresNoOutputParameter` | yes | yes, relocated to `internal/server/teams_verbs_test.go:180` | yes |
| `internal/tools/teams_get_online_meeting_test.go` | `TestGetOnlineMeeting_ResolvesJoinURL` | yes | yes (`:142`) | yes |
| `internal/tools/teams_get_online_meeting_test.go` | `TestGetOnlineMeeting_RequiresOneIdentifier` | yes | yes (`:199`) | yes |
| `internal/tools/teams_get_transcript_test.go` | `TestGetTranscript_ContentOnlyUnderRaw` | yes | yes (`:77`) | yes |
| `internal/tools/teams_read_verbs_test.go` | `TestEveryReadVerbValidatesIdentifiersBeforeCall` | yes | **no — file absent** | GAP-1 |
| `internal/tools/teams_read_verbs_test.go` | `TestReadVerbsHonourTimeoutAndRedaction` | yes | **no — file absent** | GAP-1 |
| `internal/tools/teams_read_verbs_test.go` | `TestErrorFixReachesBothToolResultAndLog` | yes | **no — file absent** | GAP-1 |
| `internal/tools/tool_annotations_test.go` | `TestTeamsVerbAnnotations` | yes | **no** | GAP-2 |
| `internal/tools/tool_annotations_test.go` | `TestTeamsAggregateIsReadOnly` | yes | **no** | GAP-2 |
| `internal/server/teams_verbs_test.go` | `TestBuildTeamsVerbs_ThirteenVerbsInOrder` | yes | yes, renamed `TestTeamsVerbsRegisterThirteen` (`:77`) | partial — asserts the sorted verb **set**, not the inventory **order** the row specifies. GAP-6 |
| `internal/server/server_test.go` | `TestRegisterTools_TeamsEnabled_RegistersSixthTool` | yes | yes | yes |
| `internal/server/server_test.go` | `TestRegisterTools_TeamsDisabled_NoTeamsTool` | yes | yes | yes |
| `internal/tools/teams_output_test.go` | `TestEveryTeamsReadVerbDeclaresOutput` | yes | yes, relocated and renamed `TestTeamsReadVerbsDeclareOutput` (`internal/server/teams_verbs_test.go:193`) | yes — derives cases from the built slice and asserts the selection count is 11 |
| `internal/auth/auth_test.go` | `TestScopes_TeamsEnabled` | yes | yes | yes |
| `internal/auth/auth_test.go` | `TestScopes_NoTeamsSendEver` | yes | yes | yes |
| `internal/config/config_test.go` | `TestLoadConfig_TeamsEnabledDefaultFalse` | yes | yes | yes |
| `internal/surface/surface_test.go` | `TestTeamsDomainRecordedGatedAndFull` | yes | **no** — the file's only change makes `TestContactsDomainRecordedGatedAndFull` position-independent | GAP-4 |
| `internal/server/teams_verbs_test.go` | `TestTeamsVerbsCarryDomainQualifiedIdentity` | yes | yes (`:227`) | yes |
| `internal/tools/status_test.go` | `TestStatus_ReportsTeamsEnabled` | yes | **no** — file unchanged in the diff range | GAP-3 |
| `internal/server/teams_verbs_test.go` | `TestTeamsRegistryExposesNoOutOfScopeVerb` | yes | yes (`:121`) | yes |

### Tests to Modify

| Test File | Test Name | Specified | Exists | Matches Spec |
|---|---|---|---|---|
| `internal/tools/verb_metadata_test.go` | five metadata checks | yes | yes | yes — `TeamsEnabled` at `:59,160,379`; `"teams"` in the lists at `:102,164,209,235,384` |
| `internal/tools/description_quality_test.go` | four description checks | yes | yes | yes — `:42` config, `"teams"` at `:95,132,153,166` |
| `internal/tools/tool_description_test.go` | description-shape checks | yes | yes | yes — `:245,248,266,269` |
| `internal/tools/tool_annotations_test.go` | `TestPerVerbAnnotations_DocumentedInHelp` | yes | yes | yes — `:416` config, `:419` domain list |
| `internal/server/surface_export_test.go` | export shape check | yes | yes | yes — `:24,28` |
| `internal/server/manifest_sync_test.go` | `TestManifestDescribesEveryRegisteredVerb` | yes | yes | yes — `:73` config; count literal now 6; test passes |
| `internal/server/schema_size_test.go` | `TestColdStartSchemaSize_Reduction` | yes | yes | yes — `:67` `TeamsEnabled: true`; measures six tools at 27,413 bytes / 62% |
| `internal/tools/dispatch_registry_test.go` | `TestVerbInventoryUnchangedAfterUpgrade` | yes | yes | yes — 13 `teams.*` golden lines added (+14 lines); test passes in `make ci` |
| `docs/prompts/mcp-tool-crud-test.md` | Steps 52-58 with skip instruction | yes | yes | yes — `:790-858` |
| `scripts/crud-test.sh`, `docs/bench/crud-runs.csv` | per-domain accounting | yes | yes | yes — `:97,118,130,138`; CSV header-only |

### Test execution

Every test named above that exists was executed and passed. Full `make ci` exits 0 with no
`FAIL` line across 19 packages.

## Diff Coverage

| File | +/- | Mapped Requirements |
|---|---|---|
| `internal/config/config.go` | +16/-0 | FR-3 |
| `internal/config/inventory.go` | +2/-0 | FR-3 |
| `internal/config/config_test.go` | +60/-0 | FR-3 |
| `internal/auth/auth.go` | +53/-3 | FR-4, FR-5 |
| `internal/auth/auth_test.go` | +101/-0 | FR-4, FR-5, AC-7 |
| `internal/tools/teams_search.go` | +198/-0 | FR-8, NFR-1, NFR-2 |
| `internal/tools/teams_search_test.go` | +341/-0 | FR-8, AC-2 |
| `internal/tools/teams_list_chats.go` | +141/-0 | FR-9, NFR-1, NFR-6 |
| `internal/tools/teams_list_chats_test.go` | +127/-0 | FR-9, NFR-6 |
| `internal/tools/teams_list_chat_messages.go` | +152/-0 | FR-9, NFR-1 |
| `internal/tools/teams_list_chat_messages_test.go` | +155/-0 | FR-9 |
| `internal/tools/teams_get_chat_message.go` | +141/-0 | FR-9, FR-14 |
| `internal/tools/teams_get_chat_message_test.go` | +162/-0 | FR-14, AC-3 |
| `internal/tools/teams_list_chat_message_replies.go` | +161/-0 | FR-9, NFR-1 |
| `internal/tools/teams_list_chat_message_replies_test.go` | +155/-0 | FR-9 |
| `internal/tools/teams_list_channel_messages.go` | +164/-0 | FR-10, FR-18 |
| `internal/tools/teams_list_channel_messages_test.go` | +179/-0 | FR-10, AC-3 |
| `internal/tools/teams_get_channel_message.go` | +150/-0 | FR-10, FR-14, FR-18 |
| `internal/tools/teams_get_channel_message_test.go` | +153/-0 | FR-10, FR-14, AC-3 |
| `internal/tools/teams_list_channel_message_replies.go` | +147/-0 | FR-10, FR-18 |
| `internal/tools/teams_list_channel_message_replies_test.go` | +157/-0 | FR-10 |
| `internal/tools/teams_compose_reply.go` | +182/-0 | FR-7, FR-15, NFR-7 |
| `internal/tools/teams_compose_reply_test.go` | +171/-0 | FR-7, AC-4 |
| `internal/tools/teams_get_online_meeting.go` | +247/-0 | FR-11, FR-18 |
| `internal/tools/teams_get_online_meeting_test.go` | +277/-0 | FR-11, AC-5 |
| `internal/tools/teams_list_transcripts.go` | +156/-0 | FR-12, FR-18 |
| `internal/tools/teams_list_transcripts_test.go` | +135/-0 | FR-12, AC-5 |
| `internal/tools/teams_get_transcript.go` | +197/-0 | FR-12, FR-14, NFR-6 |
| `internal/tools/teams_get_transcript_test.go` | +173/-0 | FR-12, FR-14, NFR-6, AC-5 |
| `internal/tools/teams_serialize.go` | +629/-0 | FR-13 |
| `internal/tools/teams_serialize_test.go` | +424/-0 | FR-13 |
| `internal/tools/text_format.go` | +390/-0 | NFR-1 |
| `internal/tools/status.go` | +7/-0 | FR-27 |
| `internal/server/teams_verbs.go` | +416/-0 | FR-1, FR-6, FR-13, FR-14, FR-15, FR-16, FR-17 |
| `internal/server/teams_verbs_test.go` | +288/-0 | FR-1, FR-15, FR-16, FR-26, AC-4, AC-12, AC-13 |
| `internal/server/server.go` | +30/-0 | FR-1, FR-2, FR-21 |
| `internal/server/server_test.go` | +86/-0 | FR-1, FR-2, AC-1 |
| `internal/server/introspect_verbs.go` | +22/-8 | FR-2 |
| `internal/server/surface_export.go` | +2/-2 | FR-25 |
| `internal/server/surface_export_test.go` | +3/-3 | Tests to Modify |
| `internal/surface/build.go` | +10/-4 | FR-20 |
| `internal/surface/surface_test.go` | +21/-10 | FR-20 (refactor only; the specified teams test is absent — GAP-4) |
| `internal/server/manifest_sync_test.go` | +7/-6 | FR-19 |
| `internal/server/schema_size_test.go` | +6/-4 | FR-22, NFR-3, AC-9 |
| `internal/tools/dispatch_registry_test.go` | +14/-0 | Tests to Modify (verb-inventory golden) |
| `internal/tools/verb_metadata_test.go` | +8/-5 | FR-6, FR-17 |
| `internal/tools/description_quality_test.go` | +5/-4 | FR-17, NFR-4 |
| `internal/tools/tool_description_test.go` | +4/-2 | FR-17 |
| `internal/tools/tool_annotations_test.go` | +2/-1 | FR-6 (help-output clause only) |
| `internal/tools/aggregate_annotations.go` | +2/-2 | FR-25 |
| `internal/docs/llmstxt.go` | +2/-2 | FR-25 |
| `extension/manifest.json` | +5/-1 | FR-19 |
| `extension/README.md` | +1/-1 | FR-25 |
| `site/src/generated/surface.json` | +91/-1 | FR-20, AC-8 |
| `site/src/surface.ts` | +1/-1 | FR-25 |
| `scripts/crud-test.sh` | +6/-5 | FR-24 |
| `docs/bench/crud-runs.csv` | +1/-1 | FR-24 |
| `docs/prompts/mcp-tool-crud-test.md` | +71/-0 | FR-24 |
| `docs/concepts.md` | +18/-5 | FR-23 (gating section only; the OAuth table rows are missing — GAP-5) |
| `docs/troubleshooting.md` | +47/-1 | FR-17 (SeeDocs anchors), FR-25 |
| `docs/quickstart.md` | +38/-1 | FR-25 |
| `docs/readme.md` | +2/-1 | FR-25 |
| `docs/reference/architecture.md` | +2/-2 | FR-25 |
| `AGENTS.md` | +2/-1 | FR-25 |
| `README.md` | +2/-1 | FR-25 |
| `llms.txt` | +2/-2 | FR-25 (generated artifact — see below) |
| `docs/cr/CR-0083-teams-domain.md` | +967/-257 | The CR itself (review and finalization) |
| `docs/backlog/cr-0078-0083.md` | +281/-1 | AC-9 measurement record — see below |

### Unmapped changed files

Two files are outside the CR's Affected Components list. Both are justified:

* `llms.txt` (+2/-2). Not named in Affected Components, but it is the generated output of
  `internal/docs/llmstxt.go`, which **is** named (FR-25). The Makefile regenerates it and CI
  verifies it matches the catalog ("Verifying llms.txt matches catalog"), so leaving it stale
  would have failed `make ci`. Derived artifact, correctly regenerated.
* `docs/backlog/cr-0078-0083.md` (+281/-1). Not named in Affected Components, but the CR's own
  review summary designates it as the record for decisions taken without the author
  (`## CR-0083` section), and the CR's Quality Standards checklist requires the schema
  measurement to be "recorded in the implementation notes". That record lives at `:1290-1291`
  and is the evidence for AC-9's recording clause. Governance record, not source.

No stray source file was changed. Every one of the 66 non-governance changed files maps to at
least one requirement above.

## Gaps

### GAP-1 — The log-record half of the error contract is unasserted (FR-18, AC-11) — PARTIAL

The specified file `internal/tools/teams_read_verbs_test.go` does not exist, taking with it all
three of its tests: `TestEveryReadVerbValidatesIdentifiersBeforeCall`,
`TestReadVerbsHonourTimeoutAndRedaction`, and `TestErrorFixReachesBothToolResultAndLog`. The
first two are substantively covered by the per-verb `*_RefusesEachIdentifierSeparately` and
`*_GraphFailureCarriesFix` tests, though not derived from the registry as specified. The third
is not covered at all: FR-18 and AC-11 both require the fix to reach **both** the tool result
and the log record, and no teams test installs a capturing `slog` handler, so only the
tool-result half is graded. The handlers do emit `"fix"` on the log record
(`internal/tools/teams_get_channel_message.go`, `teams_get_transcript.go`, and the other ten),
so this is a verification gap rather than a behaviour gap.

**Minimal fix:** add `internal/tools/teams_read_verbs_test.go` with
`TestErrorFixReachesBothToolResultAndLog`: install a `slog` handler capturing records into a
buffer via `logging.WithLogger`, call `NewHandleGetChannelMessage` with `team_id` omitted and
`NewHandleGetTranscript` with `meeting_id` omitted, and assert the same fix substring appears
in both the tool result text and the captured record's `fix` attribute. Optionally add the two
registry-derived siblings.

### GAP-2 — Two of the four annotation hint values are unasserted, and the folded aggregate is unasserted (FR-6, AC-6) — PARTIAL

`TestTeamsVerbAnnotations` and `TestTeamsAggregateIsReadOnly` do not exist.
`TestTeamsExposesNoWriteVerb` (`internal/server/teams_verbs_test.go:156`) asserts `readOnlyHint`
is true and `destructiveHint` is not true; `TestEveryVerbHasClassification` asserts all four
hints are **declared**. Nothing asserts that `idempotentHint` is true or that `openWorldHint` is
true for the twelve Graph-calling verbs and false for `help`, and nothing asserts the folded
`teams` tool annotation, which `TestContactsAggregateIsReadOnly`
(`internal/tools/tool_annotations_test.go:303`) does for the contacts precedent. This also
misses the `AGENTS.md` rule that new verbs "MUST add a value assertion alongside the existing
annotation tests in `internal/tools/`".

**Minimal fix:** add `TestTeamsVerbAnnotations` to `internal/tools/tool_annotations_test.go`
using the existing `verbHints` helper (`:453`) over the registered teams verbs, asserting
`(true, false, true, true)` for every verb and `(true, false, true, false)` for `help`; and add
`TestTeamsAggregateIsReadOnly` mirroring `TestContactsAggregateIsReadOnly` at `:303` against
`getRegisteredTool(t, s, "teams")`.

### GAP-3 — `system.status` Teams flag has no test (FR-27, AC-14) — PARTIAL

`internal/tools/status.go:237-241,341` adds and populates the field, but
`internal/tools/status_test.go` is unchanged in the diff range and `TestStatus_FeaturesGroup`
(`:273-292`) asserts only `read_only`, `mail_enabled`, and `provenance_tag`. The specified
`TestStatus_ReportsTeamsEnabled` does not exist. FR-24's harness skip instruction keys on
`config.features.teams_enabled`, so a regression here silently turns the Teams harness steps
into permanent skips.

**Minimal fix:** add `TestStatus_ReportsTeamsEnabled` to `internal/tools/status_test.go`,
calling the status handler with `TeamsEnabled` true and then false and asserting
`config.features.teams_enabled` is present and follows the configured value, following the
shape of `TestStatus_FeaturesGroup`.

### GAP-4 — The teams surface-record gate is missing (FR-20, FR-3) — GAP

`TestTeamsDomainRecordedGatedAndFull` does not exist. The only change to
`internal/surface/surface_test.go` makes `TestContactsDomainRecordedGatedAndFull` select by
name rather than by last position. The teams record is graded indirectly by
`TestRecordCountsMatchBuiltVerbs`, `TestDefaultCountExcludesGatedVerbs`,
`TestEveryVerbCarriesSummaryAndGate` (which iterate `rec.Domains`), and
`TestCommittedManifestMatchesRecord`. What is not directly asserted is the specific contract
the CR names: `fullCount` 13, `defaultCount` 0, every verb attributed to
`OUTLOOK_MCP_TEAMS_ENABLED`, and the record's config section naming that variable (the FR-3
inventory-row half, folded into this test by review finding V6).

**Minimal fix:** add `TestTeamsDomainRecordedGatedAndFull` to
`internal/surface/surface_test.go` mirroring the now name-selecting
`TestContactsDomainRecordedGatedAndFull` at `:121`, asserting 13/0, the gate on every verb, and
the presence of `OUTLOOK_MCP_TEAMS_ENABLED` in `rec.Config`.

### GAP-5 — The `docs/concepts.md` OAuth scopes table has no Teams rows (FR-23, AC-14) — FAIL

FR-23 requires that the "OAuth scopes used per feature" table "**MUST** gain the two
`OUTLOOK_MCP_TEAMS_ENABLED` rows", and AC-14 grades it. The table at
`docs/concepts.md:131-140` carries both `OUTLOOK_MCP_CONTACTS_ENABLED` rows but no
`OUTLOOK_MCP_TEAMS_ENABLED` row of either polarity. The Teams gating section
(`docs/concepts.md:80-91`) does state the four scopes, but it is a different table under a
different heading, and a reader consulting the canonical per-feature scopes table sees nothing
about Teams beyond the trailing sentence at `:141`. This is the documented requirement's
literal subject and it is unmet.

**Minimal fix:** insert two rows into the table at `docs/concepts.md:138`, after the contacts
rows and before the `offline_access` row:

```
| `OUTLOOK_MCP_TEAMS_ENABLED=false` (default) | *(none)* |
| `OUTLOOK_MCP_TEAMS_ENABLED=true` | `Chat.Read`, `ChannelMessage.Read.All`, `OnlineMeetings.Read`, `OnlineMeetingTranscript.Read.All` |
```

### GAP-6 — The verb-set test asserts a set, not the inventory order — PARTIAL

The Test Strategy row for `TestBuildTeamsVerbs_ThirteenVerbsInOrder` specifies "Thirteen verbs,
named and ordered as the inventory states". The implemented
`TestTeamsVerbsRegisterThirteen` (`internal/server/teams_verbs_test.go:77-110`) sorts the built
names before comparing, so a builder that emitted the thirteen verbs in any order would pass.
Registration order is what the published operation enum and the help output present, so the
ordering clause is graded by nothing.

**Minimal fix:** in `TestTeamsVerbsRegisterThirteen`, drop the `sort.Strings(got)` call and
compare against the inventory order (`help`, `search`, `list_chats`, `list_chat_messages`,
`get_chat_message`, `list_chat_message_replies`, `list_channel_messages`,
`get_channel_message`, `list_channel_message_replies`, `compose_reply`, `get_online_meeting`,
`list_transcripts`, `get_transcript`), which is the order `buildTeamsVerbs` already produces at
`internal/server/teams_verbs.go:128-142`.

### GAP-7 — `make crud-test` not run — GAP (deferred to the user)

FR-24 adds Steps 52-58 to `docs/prompts/mcp-tool-crud-test.md` and the `mcp_teams` accounting to
`scripts/crud-test.sh`. The harness is a paid, live-tenant run and was excluded from this
validation by instruction. The static edits are verified (see FR-24), but the prompt has never
been executed against a live tenant with `OUTLOOK_MCP_TEAMS_ENABLED` set, so the Teams steps'
executability, the per-domain CSV accounting, and the `config.features.teams_enabled` skip path
are unproven end to end. `AGENTS.md` records prompt drift as an open class with no automated
check binding prompt prose to registry parameters, so this run is the only instrument that
would catch a drifted Teams step.

**Deferred to the user.** Rebuild the binary at the `.mcp.json` path first (`AGENTS.md`
records that the harness drives a built binary by path, not the working tree), confirm the
report's `Server version` line matches `git rev-parse --short HEAD`, then run `make crud-test`.

### GAP-8 — Site screenshot comparison not run — GAP (deferred to the user)

`site/AGENTS.md` requires that a change claiming to leave rendering untouched be verified by
screenshot comparison rather than assumed. This diff changes `site/src/generated/surface.json`
(+91/-1: a sixth domain, thirteen verbs, totals 57→70 full and a new config variable) and
`site/src/surface.ts` (+1/-1, doc comment only). The site derives every figure it states about
the tool surface from that manifest, so the rendered pages change by construction; the
comparison was not run and no before/after evidence exists for the affected pages.

**Deferred to the user.** Run the site's screenshot comparison against the pages that read
`domainCount`, `fullVerbCount`, `defaultVerbCount`, `configVarCount`, and `domainNames`, and
confirm the only deltas are the intended new figures.
