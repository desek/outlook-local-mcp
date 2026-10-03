# CR-0083 Validation Report

Validated at `9a9ff99` on branch `docs/cr-implementation-set-0079-0083`.
Gaps closed at `c9e048c`; this revision records the fixed state.
Diff range: `85c6450..HEAD` (the CR-0083 slice; `85c6450` is the last CR-0082 commit).
Merge base with `origin/main`: `2cce019`.

## Summary

Requirements: 34/34 | Acceptance Criteria: 14/14 | Tests: 36/36 | Gaps: 8 (6 FIXED, 2 not-run)

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
comparison. Both are recorded below as `not-run`, deferred to the user, and listed as pending
bullets under `## CR-0083` in `docs/backlog/cr-0078-0083.md`. A derived, not-yet-run user
scenario stands in for the live-tenant Teams check at
`.agents/scenarios/2026-09-02-teams-conversation-and-transcript-reads.md`.

## Requirement Verification

### Functional Requirements

| Req # | Description | Status | Evidence (file:line / test name) |
|---|---|---|---|
| FR-1 | Register a `teams` aggregate tool with exactly thirteen verbs | PASS | `internal/server/server.go:235-256`; `internal/server/teams_verbs.go:128-142`; tests `TestTeamsVerbsRegisterThirteen` (`internal/server/teams_verbs_test.go:77`), `TestRegisterTools_TeamsEnabled_RegistersSixthTool` — both pass |
| FR-2 | Registered only under `TeamsEnabled`, same condition in `BuildDomainVerbSets` | PASS | `internal/server/server.go:235`; `internal/server/introspect_verbs.go:36,138`; test `TestRegisterTools_TeamsDisabled_NoTeamsTool` passes |
| FR-3 | `TeamsEnabled` on `config.Config` from `OUTLOOK_MCP_TEAMS_ENABLED`, plus `EnvTeamsEnabled` const and inventory row | PASS | `internal/config/config.go:157-167,343`; `internal/config/inventory.go:45,95`; `site/src/generated/surface.json` config section carries `OUTLOOK_MCP_TEAMS_ENABLED`; test `TestLoadConfig_TeamsEnabledDefaultFalse` passes |
| FR-4 | Four delegated read scopes requested only when enabled | PASS | `internal/auth/auth.go:53,60,66,75,131`; test `TestScopes_TeamsEnabled` passes |
| FR-5 | No Teams send or write scope in any configuration | PASS | `internal/auth/auth.go:97-116` (doc comment states the property); test `TestScopes_NoTeamsSendEver` passes over every flag combination |
| FR-6 | All four hints declared explicitly with the matrix values | PASS | Values are correct in source: `internal/server/teams_verbs.go:75-82` (twelve verbs, openWorld `true`), `internal/tools/help/verb.go:52-56` (`help`, openWorld `false`). Declaration of all four is graded by `TestEveryVerbHasClassification` (`internal/tools/verb_metadata_test.go:209` domain list includes `teams`). `readOnlyHint`/`destructiveHint` **values** graded by `TestTeamsExposesNoWriteVerb` (`internal/server/teams_verbs_test.go:156`). FIXED: `TestTeamsVerbAnnotations` (`internal/tools/tool_annotations_test.go:360`) derives its cases from `BuildDomainVerbSets(cfg)["teams"]` and asserts all four values per verb, `(true, false, true, true)` for the twelve Graph-calling verbs and `(true, false, true, false)` for `help` — passes. Status raised to PASS |
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
| FR-18 | Every error carries a fix on **both** the tool result and the log record; team/channel fix names `teams.search`; meeting fix names `get_online_meeting` | PASS | Source satisfies both channels: e.g. `internal/tools/teams_get_channel_message.go:33-37` plus the handler's `logger.Error(..., "fix", ...)` and `mcp.NewToolResultError(fmt.Sprintf("%s: %s", ...))`; `internal/tools/teams_list_transcripts.go:32` and `teams_get_transcript.go:40` name `teams get_online_meeting`. FIXED: `internal/tools/teams_read_verbs_test.go` now exists and installs a buffer-backed `slog` handler. `TestErrorFixReachesBothToolResultAndLog` (`:269`) asserts the same correction substring on both channels for a pre-request refusal and a service failure; `TestReadVerbsHonourTimeoutAndRedaction` (`:213`) asserts a correction reaches the log record on both the timeout and the service-failure branch of all twelve Graph-calling verbs — all pass. Status raised to PASS. Note the fix strings say "teams search" / "teams get_online_meeting" (the domain-plus-verb form used throughout this codebase) rather than the dotted `teams.search` |
| FR-19 | `teams` as the sixth `extension/manifest.json` entry; literal `5`→`6`; `maximalSurfaceConfig` gains the flag; `long_description` Teams line | PASS | `extension/manifest.json` `tools` array is `[calendar, mail, account, system, contacts, teams]`; `long_description` line 11 "Teams reading: search chat and channel messages…"; `internal/server/manifest_sync_test.go:73` (`TeamsEnabled: true`) and the count literal; test `TestManifestDescribesEveryRegisteredVerb` passes |
| FR-20 | Surface manifest regenerated; `build.go` gains `domainOrder`, both configs, gate probe | PASS | `internal/surface/build.go:31,44,77,84`; `site/src/generated/surface.json` records `teams` 13 full / 0 default with every verb gated on `OUTLOOK_MCP_TEAMS_ENABLED`; totals 70/38; test `TestCommittedManifestMatchesRecord` passes and `make ci` leaves the tree clean |
| FR-21 | `toolCount` incremented inside the gated branch; base literal untouched | PASS | `internal/server/server.go:205` (`toolCount := 4`), `:225`, `:255`; the schema-size run logs `tool registration complete tools=6` |
| FR-22 | Schema gate extended with `TeamsEnabled`, reduction ≥60%, constants unchanged | PASS | `internal/server/schema_size_test.go:67`; measured 27,413 bytes / 62%; `minRequiredReductionPct` still 60 (`:30`), `preCRBaselineBytes` still 74,000 (`:26`) |
| FR-23 | `docs/concepts.md` Teams gating section, **two `OUTLOOK_MCP_TEAMS_ENABLED` rows in the "OAuth scopes used per feature" table**, and no stale tool-set statement | FIXED | Gating section present and correct (`docs/concepts.md:80-91`); stale statements corrected (`:95`, `:191`, `:143`). Both scope rows now present, following the contacts rows: `docs/concepts.md:139-140`. Gated by `TestTeamsScopeRowsNameEveryRequestedScope` (`internal/docs/catalog_test.go:104`), which reads the **embedded bundle** rather than the file on disk, as the contacts-era mail precedent in the same file does, and asserts the enabled row names all four scopes — passes |
| FR-24 | CRUD prompt steps from Step 52 with a `config.features.teams_enabled` skip; `crud-test.sh` column/awk/jq; CSV header reset | PASS | `docs/prompts/mcp-tool-crud-test.md:790-858` (Steps 52-58, skip instruction at `:794`); `scripts/crud-test.sh:97` (`mcp_teams` column), `:118` (awk bucket), `:130,138` (jq argument); `docs/bench/crud-runs.csv` is header-only (1 line), historical rows reset |
| FR-25 | Every statement of the aggregate tool set updated | PASS | `teams` present in `AGENTS.md`, `README.md`, `docs/readme.md`, `docs/quickstart.md`, `docs/troubleshooting.md`, `docs/reference/architecture.md`, `extension/README.md`, `internal/docs/llmstxt.go`, `internal/tools/aggregate_annotations.go:2`, `site/src/surface.ts`; `site/src/surface.ts:82` `domainCount` still `surface.domains.length`, not a literal |
| FR-26 | No `Manage=3` capability introduced | PASS | Tests `TestTeamsRegistryExposesNoOutOfScopeVerb` (`internal/server/teams_verbs_test.go:121`, name and prefix exclusions) and `TestTeamsVerbsRegisterThirteen` pass |
| FR-27 | `system.status` reports `config.features.teams_enabled` | FIXED | Source present: `internal/tools/status.go:237-241` (field with the JSON tag) and `:341` (populated from `cfg.TeamsEnabled`). `TestStatus_ReportsTeamsEnabled` (`internal/tools/status_test.go:305`) now calls the status handler under both polarities and asserts the field is present and follows the configured value — passes. Both polarities are asserted so a field pinned to a constant cannot pass |

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
| AC-6 | Every verb read-only, non-destructive, idempotent; open-world except `help`; folded tool read-only; no send scope ever | PASS | Scope clause PASS (`TestScopes_NoTeamsSendEver`). Read-only and non-destructive per verb PASS (`TestTeamsExposesNoWriteVerb`). FIXED: the idempotent and open-world clauses are graded per verb by `TestTeamsVerbAnnotations` (`internal/tools/tool_annotations_test.go:360`), including `help`'s `openWorldHint: false`, and the folded aggregate by `TestTeamsAggregateIsReadOnly` (`:330`), which mirrors `TestContactsAggregateIsReadOnly` against `getRegisteredTool(t, s, "teams")` — both pass. Status raised to PASS |
| AC-7 | Consent surface changes only on opt-in | PASS | `TestScopes_TeamsEnabled`, `TestScopes_NoTeamsSendEver` pass; `internal/auth/auth.go:131` gates the four scopes |
| AC-8 | Tool count 6; `make ci` matches the committed surface manifest; manifest holds six entries; surface records 13 gated verbs | PASS | Schema-size run logs `tools=6`; `make ci` exit 0 with `git status --porcelain` empty; `extension/manifest.json` six entries; `site/src/generated/surface.json` teams 13/0, every verb gated on `OUTLOOK_MCP_TEAMS_ENABLED` |
| AC-9 | Schema gate passes at max configuration and the measurement is recorded | PASS | Measured 27,413 bytes / 62% ≥ 60%; recorded in `docs/backlog/cr-0078-0083.md:1290-1291`; `minRequiredReductionPct` unchanged; `go.mod`/`go.sum` unchanged |
| AC-10 | Registry metadata complete; eleven reads declare `output`, `compose_reply` and `help` do not | PASS | `TestEveryVerbHasSummary`, `TestEveryVerbHasDescription`, `TestSeeDocsAnchorsResolve`, `TestTeamsReadVerbsDeclareOutput`, `TestComposeReply_DeclaresNoOutputParameter` pass |
| AC-11 | Errors carry their correction on the tool result **and** the log record | PASS | Tool-result half graded by the seven `*_GraphFailureCarriesFix` and `*_Refuses*` tests. FIXED: the log-record half is graded by `TestErrorFixReachesBothToolResultAndLog` and `TestReadVerbsHonourTimeoutAndRedaction` (`internal/tools/teams_read_verbs_test.go:269,213`), both installing a buffer-backed `slog` handler and reading the emitted record — pass. Status raised to PASS |
| AC-12 | No out-of-scope Teams capability; exactly thirteen verbs | PASS | `TestTeamsRegistryExposesNoOutOfScopeVerb`, `TestTeamsVerbsRegisterThirteen` pass |
| AC-13 | Every verb carries the `teams.<verb>` middleware identity | PASS | `TestTeamsVerbsCarryDomainQualifiedIdentity` derives its cases from the built slice, reads the audit log, and asserts `operation_type: read` — passes |
| AC-14 | Documentation, status, and harness surfaces state the domain that now exists | FIXED | Documentation clauses PASS: troubleshooting anchors at `docs/troubleshooting.md:253,270,285`; no file states a tool set omitting teams (FR-25 evidence); `site/src/surface.ts:82` `domainCount` still derived; CRUD prompt steps and skip instruction at `docs/prompts/mcp-tool-crud-test.md:790,794`; `scripts/crud-test.sh:97` header matches `docs/bench/crud-runs.csv:1`. Both previously failing clauses are FIXED: the two OAuth scope rows are present at `docs/concepts.md:139-140` and gated by `TestTeamsScopeRowsNameEveryRequestedScope`, and the `system.status` clause is graded by `TestStatus_ReportsTeamsEnabled`. Status raised to FIXED |

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
| `internal/tools/teams_read_verbs_test.go` | `TestEveryReadVerbValidatesIdentifiersBeforeCall` | yes | yes (`:183`) | yes — table-driven over the eleven verbs with a required argument, asserting refusal and a zero request count. `list_chats` is excluded and the exclusion is stated: it takes no required argument |
| `internal/tools/teams_read_verbs_test.go` | `TestReadVerbsHonourTimeoutAndRedaction` | yes | yes (`:213`) | yes — both halves over all twelve Graph-calling verbs, with a buffer-backed `slog` handler asserting the log record on each |
| `internal/tools/teams_read_verbs_test.go` | `TestErrorFixReachesBothToolResultAndLog` | yes | yes (`:269`) | yes — a pre-request refusal and a service failure, each asserted on both channels |
| `internal/tools/teams_read_verbs_test.go` | `TestEveryTeamsGraphVerbIsCovered` | no — added | yes (`:169`) | not specified; guards the hand-written table above against a fourteenth verb escaping all three cross-verb properties |
| `internal/tools/tool_annotations_test.go` | `TestTeamsVerbAnnotations` | yes | yes (`:360`) | yes — all four values per verb, derived from the registered verb set |
| `internal/tools/tool_annotations_test.go` | `TestTeamsAggregateIsReadOnly` | yes | yes (`:330`) | yes — mirrors `TestContactsAggregateIsReadOnly` |
| `internal/server/teams_verbs_test.go` | `TestBuildTeamsVerbs_ThirteenVerbsInOrder` | yes | yes, renamed `TestTeamsVerbsRegisterThirteen` (`:84`) | yes — the `sort.Strings` call is removed and the comparison is now against the inventory order `buildTeamsVerbs` produces |
| `internal/server/server_test.go` | `TestRegisterTools_TeamsEnabled_RegistersSixthTool` | yes | yes | yes |
| `internal/server/server_test.go` | `TestRegisterTools_TeamsDisabled_NoTeamsTool` | yes | yes | yes |
| `internal/tools/teams_output_test.go` | `TestEveryTeamsReadVerbDeclaresOutput` | yes | yes, relocated and renamed `TestTeamsReadVerbsDeclareOutput` (`internal/server/teams_verbs_test.go:193`) | yes — derives cases from the built slice and asserts the selection count is 11 |
| `internal/auth/auth_test.go` | `TestScopes_TeamsEnabled` | yes | yes | yes |
| `internal/auth/auth_test.go` | `TestScopes_NoTeamsSendEver` | yes | yes | yes |
| `internal/config/config_test.go` | `TestLoadConfig_TeamsEnabledDefaultFalse` | yes | yes | yes |
| `internal/surface/surface_test.go` | `TestTeamsDomainRecordedGatedAndFull` | yes | yes (`:175`) | yes — asserts 13 full / 0 default, thirteen recorded verbs, the gate on every one, and `EnvTeamsEnabled` enumerated in `rec.Config` |
| `internal/server/teams_verbs_test.go` | `TestTeamsVerbsCarryDomainQualifiedIdentity` | yes | yes (`:227`) | yes |
| `internal/tools/status_test.go` | `TestStatus_ReportsTeamsEnabled` | yes | yes (`:305`) | yes — both polarities |
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

| `internal/docs/catalog_test.go` | Teams OAuth scope row gate | no — added by gap fix | yes, `TestTeamsScopeRowsNameEveryRequestedScope` (`:104`) | not specified; the contacts-era precedent in the same file gates its documentation row the same way, reading the embedded bundle rather than the file on disk |

### Test execution

Every test named above was executed and passed. Full `make ci` exits 0 with no
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
| `internal/tools/status_test.go` | gap fix | FR-27, AC-14 (`TestStatus_ReportsTeamsEnabled`) |
| `internal/tools/teams_read_verbs_test.go` | gap fix, new file | FR-18, AC-11, NFR-2 |
| `internal/docs/catalog_test.go` | gap fix | FR-23, AC-14 (embedded-bundle gate on the scope rows) |
| `.agents/scenarios/2026-09-02-teams-conversation-and-transcript-reads.md` | gap fix, new file | Deferred live-tenant verification (GAP-7, GAP-8) |
| `internal/server/teams_verbs.go` | +416/-0 | FR-1, FR-6, FR-13, FR-14, FR-15, FR-16, FR-17 |
| `internal/server/teams_verbs_test.go` | +288/-0 | FR-1, FR-15, FR-16, FR-26, AC-4, AC-12, AC-13 |
| `internal/server/server.go` | +30/-0 | FR-1, FR-2, FR-21 |
| `internal/server/server_test.go` | +86/-0 | FR-1, FR-2, AC-1 |
| `internal/server/introspect_verbs.go` | +22/-8 | FR-2 |
| `internal/server/surface_export.go` | +2/-2 | FR-25 |
| `internal/server/surface_export_test.go` | +3/-3 | Tests to Modify |
| `internal/surface/build.go` | +10/-4 | FR-20 |
| `internal/surface/surface_test.go` | +21/-10, plus the gap fix | FR-20 (`TestTeamsDomainRecordedGatedAndFull` added by the gap fix) |
| `internal/server/manifest_sync_test.go` | +7/-6 | FR-19 |
| `internal/server/schema_size_test.go` | +6/-4 | FR-22, NFR-3, AC-9 |
| `internal/tools/dispatch_registry_test.go` | +14/-0 | Tests to Modify (verb-inventory golden) |
| `internal/tools/verb_metadata_test.go` | +8/-5 | FR-6, FR-17 |
| `internal/tools/description_quality_test.go` | +5/-4 | FR-17, NFR-4 |
| `internal/tools/tool_description_test.go` | +4/-2 | FR-17 |
| `internal/tools/tool_annotations_test.go` | +2/-1, plus the gap fix | FR-6 (help-output clause, plus `TestTeamsVerbAnnotations` and `TestTeamsAggregateIsReadOnly`) |
| `internal/tools/aggregate_annotations.go` | +2/-2 | FR-25 |
| `internal/docs/llmstxt.go` | +2/-2 | FR-25 |
| `extension/manifest.json` | +5/-1 | FR-19 |
| `extension/README.md` | +1/-1 | FR-25 |
| `site/src/generated/surface.json` | +91/-1 | FR-20, AC-8 |
| `site/src/surface.ts` | +1/-1 | FR-25 |
| `scripts/crud-test.sh` | +6/-5 | FR-24 |
| `docs/bench/crud-runs.csv` | +1/-1 | FR-24 |
| `docs/prompts/mcp-tool-crud-test.md` | +71/-0 | FR-24 |
| `docs/concepts.md` | +18/-5, plus the two scope rows | FR-23 |
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

Six gaps are closed. The two remaining are the deferred live-instrument runs, recorded here
as `not-run` and listed for the user in `docs/backlog/cr-0078-0083.md` under `## CR-0083`.

### GAP-1 — The log-record half of the error contract is unasserted (FR-18, AC-11) — FIXED

The specified file `internal/tools/teams_read_verbs_test.go` now exists and carries all three
of its tests plus one guard.

`TestErrorFixReachesBothToolResultAndLog` (`:269`) installs a buffer-backed `slog` handler via
`slog.SetDefault`, which is the channel `logging.Logger(ctx)` derives from, and asserts the
same correction substring appears in the tool result and in the emitted record. Two
structurally different failures are driven, a pre-request validation refusal on
`NewHandleGetChannelMessage` and a service failure on `NewHandleGetTranscript`, because the
two are written on different branches of every handler and a fix present on one says nothing
about the other.

`TestReadVerbsHonourTimeoutAndRedaction` (`:213`) drives all twelve Graph-calling verbs
against two endpoints: one that withholds its answer past a millisecond deadline, and one
that answers with a service error carrying an email address. It asserts the timeout is
reported rather than blocked on, the address does not survive into the tool result while
`[email redacted]` does, and a correction reaches the log record on both branches.

`TestEveryReadVerbValidatesIdentifiersBeforeCall` (`:183`) asserts a missing required
argument refuses with a zero request count across the eleven verbs that take one.

`TestEveryTeamsGraphVerbIsCovered` (`:169`) was added beyond the specification. The three
tests above drive bare constructors against a recording endpoint, so their tables cannot be
derived from the registry the way the server-package tests are; without this guard a
fourteenth verb would be added to the domain and silently escape all three cross-verb
properties. It fails the build when the table stops covering twelve.

One implementation note worth carrying: the slow endpoint bounds its own wait rather than
blocking on `req.Context().Done()`. An aborted client is not reliably observed as a cancelled
server-side context, and the POST-bodied `search` verb hung the whole package for the full
test timeout while `httptest.Server.Close` waited on the outstanding handler.

### GAP-2 — Two of the four annotation hint values are unasserted, and the folded aggregate is unasserted (FR-6, AC-6) — FIXED

`TestTeamsVerbAnnotations` (`internal/tools/tool_annotations_test.go:360`) reads
`BuildDomainVerbSets(cfg)["teams"]` under `TeamsEnabled: true` and asserts all four hint
values for every registered verb through the existing `verbHints` helper: readOnly true,
destructive false, idempotent true for all thirteen, and openWorld true for the twelve that
reach Graph and false for `help`. The cases are derived from the registry rather than listed,
so a verb added later is asserted without anyone extending a list.

`TestTeamsAggregateIsReadOnly` (`:330`) mirrors `TestContactsAggregateIsReadOnly` against
`getRegisteredTool(t, s, "teams")`, asserting the folded `(Teams, true, false, true, true)`.

This also satisfies the `AGENTS.md` rule that a new verb must add a value assertion alongside
the existing annotation tests in `internal/tools/`.

### GAP-3 — `system.status` Teams flag has no test (FR-27, AC-14) — FIXED

`TestStatus_ReportsTeamsEnabled` (`internal/tools/status_test.go:305`) calls the status
handler with `TeamsEnabled` false and then true and asserts `config.features.teams_enabled` is
present and follows the configured value. Both polarities are asserted deliberately: a field
pinned to a constant would pass a single-value check, and the harness keys its Teams skip on
this field, so that regression would turn the Teams steps into permanent silent skips.

### GAP-4 — The teams surface-record gate is missing (FR-20, FR-3) — FIXED

`TestTeamsDomainRecordedGatedAndFull` (`internal/surface/surface_test.go:175`) selects the
domain by name, as the contacts test now does, and asserts `FullCount` 13, `DefaultCount` 0,
thirteen recorded verbs, `config.EnvTeamsEnabled` as the gate on every one of them, and the
presence of that variable in the record's own `Config` inventory, which is the FR-3
inventory-row half folded in by review finding V6. A verb attributed to a variable the
inventory does not enumerate would render on the site as a gate a reader cannot look up,
which is why the last clause is in the same test rather than a separate one.

### GAP-5 — The `docs/concepts.md` OAuth scopes table has no Teams rows (FR-23, AC-14) — FIXED

Both rows are inserted into the "OAuth scopes used per feature" table at
`docs/concepts.md:139-140`, after the contacts rows and before the `offline_access` row, in
the polarity pair the contacts precedent uses.

The row is gated by `TestTeamsScopeRowsNameEveryRequestedScope`
(`internal/docs/catalog_test.go:104`), following the precedent already in that file: the
assertion reads the **embedded bundle** rather than the file on disk, because the bundle is
what a running server serves to an LLM mid-session, and a documentation row that is correct
on disk but absent from the bundle is absent where it is consulted. It asserts the disabled
row states that no scope is requested and the enabled row names all four scopes. The rows are
told apart by the `=false` marker rather than by position, and the `=` form is what selects
the canonical scopes table over the gating section, which states the same variable in a
Variable/Value column pair.

### GAP-6 — The verb-set test asserts a set, not the inventory order — FIXED

`TestTeamsVerbsRegisterThirteen` (`internal/server/teams_verbs_test.go:84`) no longer sorts.
The `sort.Strings` call and the now-unused `sort` import are removed, and the comparison is
against the inventory order `buildTeamsVerbs` already produces at
`internal/server/teams_verbs.go:128-142`: `help`, `search`, `list_chats`,
`list_chat_messages`, `get_chat_message`, `list_chat_message_replies`,
`list_channel_messages`, `get_channel_message`, `list_channel_message_replies`,
`compose_reply`, `get_online_meeting`, `list_transcripts`, `get_transcript`. The Test Strategy
row is met as written; no amendment was needed. Registration order is what the published
operation enum and the help output present, and the inventory groups the verbs by the chain a
caller walks, which a sorted comparison would have accepted any permutation of.

### GAP-7 — `make crud-test` not run — not-run (deferred to the user)

Unchanged from the original finding: the harness is a paid, live-tenant run and was excluded
by instruction. The static edits are verified (see FR-24); the prompt has never been executed
against a live tenant with `OUTLOOK_MCP_TEAMS_ENABLED` set, so the Teams steps'
executability, the per-domain CSV accounting, and the `config.features.teams_enabled` skip
path are unproven end to end. `AGENTS.md` records prompt drift as an open class with no
automated check binding prompt prose to registry parameters, so this run is the only
instrument that would catch a drifted Teams step.

**Deferred to the user**, and recorded as a pending bullet under `## CR-0083` in
`docs/backlog/cr-0078-0083.md`. Rebuild the binary at the `.mcp.json` path first, confirm the
report's `Server version` line matches `git rev-parse --short HEAD`, then run `make crud-test`.

### GAP-8 — Site screenshot comparison not run — not-run (deferred to the user)

Unchanged from the original finding. `site/AGENTS.md` requires that a change claiming to
leave rendering untouched be verified by screenshot comparison rather than assumed. This diff
changes `site/src/generated/surface.json` (a sixth domain, thirteen verbs, totals 57 to 70
full, and a new config variable) and `site/src/surface.ts` (doc comment only). The site
derives every figure it states about the tool surface from that manifest, so the rendered
pages change by construction; the comparison was not run and no before/after evidence exists.

**Deferred to the user**, and recorded as a pending bullet under `## CR-0083` in
`docs/backlog/cr-0078-0083.md`. Compare the pages that read `domainCount`, `fullVerbCount`,
`defaultVerbCount`, `configVarCount`, and `domainNames`, and confirm the only deltas are the
intended new figures.

## Deferred live-tenant verification

The two `not-run` gaps above are both instruments, not observations. Neither of them, and no
test in this repository, has ever put a Teams call on the wire: every Teams unit test drives
an `httptest` server and grades the outgoing request against a canned response.

A user scenario is derived and written to
`.agents/scenarios/2026-09-02-teams-conversation-and-transcript-reads.md`, marked
`outcome: not-run` in its frontmatter. It exists to settle the two facts a canned endpoint
cannot reach and on which the domain's usability rests:

* **Whether a live search hit from a channel carries the team and channel identifiers.** The
  domain deliberately offers no team or channel enumeration, so a channel is reachable only
  through a search hit's `channelIdentity`. If the live service omits either field, the
  channel half of the domain is unreachable by the only route it offers.
* **Whether `joinWebUrl eq` resolves an online meeting the account attended rather than
  organised.** The whole transcript chain depends on it, and it is the single most likely
  failure.

The scenario states that either reproduction routes to a follow-on change request rather than
a patch, and that it should be run before the paid harness: it costs a handful of
authenticated calls to answer what a harness run would spend a great deal more to report.
