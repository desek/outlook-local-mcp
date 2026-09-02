# CR-0082 Validation Report

Validated at `43b39b8` on branch `docs/cr-implementation-set-0079-0083`.
Diff base `3ae0e2c` (last CR-0081 commit); implementation commits `012f02b`, `9a3a493`,
`27e0473`, `8cf26e1`.

**Gap-fix round applied on top of `31919de`.** GAP-1 through GAP-6 are closed and their
rows re-traced below; GAP-7, GAP-8 and GAP-9 are deferred verifications requiring a paid
harness run, a site screenshot comparison, and a live mailbox respectively, and are recorded
as `not-run` rather than closed. Each is named as pending for the user under `## CR-0082` in
`docs/backlog/cr-0078-0083.md`, and GAP-9 has a derived, unrun scenario at
`.agents/scenarios/2026-09-02-contacts-name-resolution-live-mailbox.md`.

## Summary

Requirements: 39/39 | Acceptance Criteria: 17/17 | Tests: 66/66 run passing (30 specified rows: all satisfied directly or under a renamed equivalent) | Gaps: 6 fixed, 3 not-run

`make ci` exits **0**. The working tree is clean after the run, so the surface-drift gate and
the `llms.txt` regeneration both agree with what is committed.

Two measured figures the CR required this report to record rather than estimate:

| Measurement | Requirement | Run 1 | Run 2 | Verdict |
|---|---|---|---|---|
| Cold-start schema, five tools (`TestColdStartSchemaSize_Reduction`) | ≥ 60% reduction vs. 74 000-byte baseline (NFR-2, NFR-3, AC-10) | 23 461 bytes, 68% | 23 461 bytes, 68% | PASS; instrument agrees with itself exactly on unchanged input |
| `contacts` composed tool description | < 4 000 characters (NFR-8, AC-8) | 1 044 chars | — | PASS |

The CR's authoring-time estimate ("a maximum well under 20 000 bytes … low-to-mid seventies
of a percent") is **not** what the instrument reports. The true five-tool maximum is 23 461
bytes at 68%, still 8 points above the gate. Open Question 2 anticipated this and bound the
requirement to the measurement, so this is a corrected estimate rather than a failure.

## Requirement Verification

### Functional Requirements

| Req # | Description | Status | Evidence (file:line / test name) |
|---|---|---|---|
| FR-1 | `contacts` registered only when `ContactsEnabled`; four tools when unset | PASS | `internal/server/server.go:190` (`if cfg.ContactsEnabled`); `TestRegisterTools_ContactsDisabled_StaysFourTools`; manual `tools/list` on default config returns 4 (`account`, `calendar`, `mail`, `system`) |
| FR-2 | Five tools when enabled; fifth named `contacts` | PASS | `internal/server/server.go:191-226`; `TestRegisterTools_ContactsEnabled_RegistersFifthTool`; manual `tools/list` with `OUTLOOK_MCP_CONTACTS_ENABLED=true` returns 5 including `contacts` |
| FR-3 | Exactly five verbs; no write/folder/photo/directory/delta verb | PASS | `internal/server/contacts_verbs.go:97-103`; `TestContactsVerbsRegisterFive`, `TestContactsExposesNoWriteVerb`; manual operation enum = `["help","search","get_contact","list_people","get_person"]` |
| FR-4 | `search` requires `query`, issues both collection GETs with the identical normalised `Search` | PASS | `internal/tools/contacts_search.go:73-119`; `TestContactsSearch_QueriesBothCollections` (paths `/contacts` then `/people`, count 2), `TestContactsSearch_SendsIdenticalNormalisedQuery`, `TestContactsSearch_LabelsEveryMatch` |
| FR-5 | `get_contact` by ID; `displayName` + every `emailAddresses` entry; no follow-up request | PASS | `internal/tools/contacts_get_contact.go:69,84,106-109`; `internal/tools/contacts_serialize.go:50-56` (`emailAddresses` carried at every tier); `TestGetContact_ReturnsEveryAddress` (asserts `callCount()==1`) |
| FR-6 | `list_people` preserves Graph's relevance order, no re-sort | PASS | `internal/tools/contacts_list_people.go:133-146`; `TestListPeople_PreservesRelevanceOrder` |
| FR-7 | `get_person` by ID; `scoredEmailAddresses`; label is the person's own display name | PASS | `internal/tools/contacts_get_person.go:68,83,105`; `internal/tools/contacts_serialize.go:130-148`; `TestGetPerson_LabelsAddressesWithDisplayName`, `TestGetPerson_RawTierCarriesScoredAddresses` |
| FR-8 | `ValidateResourceID` before any Graph request | PASS | `internal/tools/contacts_get_contact.go:69`, `internal/tools/contacts_get_person.go:68`; `TestGetContact_RejectsOverlongIdentifier`, `TestGetPerson_RejectsOverlongIdentifier` (both assert `callCount()==0`) |
| FR-9 | Reuse `NormaliseSearchQuery`, once, before the timeout context; reject empty `query` first | PASS | `internal/tools/contacts_search.go:74-86` (empty check at :74, normalise at :82, `graph.WithTimeout` at :90); `TestContactsSearch_RequiresQuery`, `TestContactsSearch_RejectsUnconvertibleQuery` (both `callCount()==0`) |
| FR-10 | Three tiers via `output`, `text` default; `help` declares none | PASS | `internal/server/contacts_verbs.go:133-136,164-167,192-195,223-226` (enum on all four reads); `help.NewHelpVerb` declares no `output`; manual schema shows `output` on the aggregate and `text` documented as default |
| FR-11 | Summary tier from a dedicated serializer, not an empty-filter | PASS | `internal/tools/contacts_serialize.go:45-57,109-122`; `TestSerializeSummary_EmptyRecordsProjectStably` (empty declared fields present, which an empty-filter cannot produce) |
| FR-12 | All four hints declared explicitly, matching the matrix | PASS | `internal/server/contacts_verbs.go:121-126,152-157,184-189,211-216`; `internal/tools/dispatch_registry_test.go:67-71` golden asserts exact `ro/de/id/ow` per verb; `TestEveryVerbHasClassification` |
| FR-13 | Read middleware chain under `contacts.<verb>`, audit op `read` | PASS | `internal/server/contacts_verbs.go:91-93,120,151,183,210`; `TestContactsVerbsWrappedUnderDomainIdentity` |
| FR-14 | `Scopes` appends both scopes on opt-in; unchanged when off | PASS | `internal/auth/auth.go:86-88`; `TestScopes_Contacts`, `TestScopes_NoContactsByDefault`; manual startup log `"graph client initialized","scopes":["Calendars.ReadWrite","Contacts.Read","People.Read"]` vs. `["Calendars.ReadWrite"]` by default |
| FR-15 | No contact write scope in any configuration | PASS | `internal/auth/auth.go:83-89`; `TestScopes_NoContactsWriteEver` |
| FR-16 | `ContactsEnabled` field, `LoadConfig` read, inventory const and row | PASS | `internal/config/config.go:148-154,325-327`; `internal/config/inventory.go:44,93`; `TestLoadConfig_ContactsEnabled`; `site/src/generated/surface.json` config section names `OUTLOOK_MCP_CONTACTS_ENABLED`, held by `TestCommittedManifestMatchesRecord`; the direct assertion on `config.Inventory()` is `TestInventoryNamesContactsFlag` (`internal/config/config_test.go:982`), asserting the row's name, its `"false"` default, and a non-empty description (GAP-2 FIXED) |
| FR-17 | Summary ≤ 80 chars, Description with parameters and annotation semantics, Examples and SeeDocs on non-help, anchors resolve to H2 | PASS | `internal/server/contacts_verbs.go:113-119,145-150,176-182,204-209`; `TestEveryVerbHasSummary`, `TestEveryVerbHasDescription`, `TestEveryVerbStatesRequiredParameters`, `TestSeeDocsAnchorsResolve` (resolves `concepts#contacts-gating`) |
| FR-18 | `operation="help"` verb | PASS | `internal/server/contacts_verbs.go:98`; manual enum contains `help`; `TestTopLevelDescription_HelpVerbPresent` |
| FR-19 | Manifest fifth entry naming all five verbs as whole words; five entries; scopes in `long_description` | PASS | `extension/manifest.json:7` (scopes clause), `:70-73` (entry); `TestManifestDescribesEveryRegisteredVerb` (`len(doc.Tools) != 5` fatal, word-boundary verb match) |
| FR-20 | Regenerated manifest: contacts 5 full / 0 default, gate on each, totals 57/38 | PASS | `site/src/generated/surface.json` measured: `totals {fullCount: 57, defaultCount: 38}`, contacts `fullCount 5`, `defaultCount 0`, all five verbs `gate: OUTLOOK_MCP_CONTACTS_ENABLED`; `TestCommittedManifestMatchesRecord`, `TestContactsDomainRecordedGatedAndFull`; `make ci` left the tree clean |
| FR-21 | Computed `toolCount`; completion log reports the actual number | PASS | `internal/server/server.go:188-227`; manual stderr `"msg":"tool registration complete","tools":5` (enabled) and `"tools":4` (default) |
| FR-22 | Every enumerated four-tool statement amended; `CLAUDE.md` untouched; `domainCount` left derived | PASS | `AGENTS.md:30,82,90`; `README.md:35,42`; `docs/readme.md:43,50`; `docs/concepts.md:69-77,124-125,176`; `internal/docs/llmstxt.go:69,153`; `internal/tools/aggregate_annotations.go:6-8`; `internal/server/surface_export.go:25-26`; `site/src/surface.ts:37,63,81`. `CLAUDE.md` absent from the diff (symlink). `domainCount` still `surface.domains.length` |
| FR-23 | `## Contacts gating` H2 plus two OAuth-scope rows | PASS | `docs/concepts.md:69-77` (H2 + table), `:124-125` (two rows), `:127` (`Contacts.ReadWrite` never requested); anchor resolution held by `TestSeeDocsAnchorsResolve` |
| FR-24 | Prompt steps from 47, three `crud-test.sh` edits, CSV header and reset rows | PASS | `docs/prompts/mcp-tool-crud-test.md:745,754,764,772,782` (Steps 47-51, each "skip if contacts disabled"); `scripts/crud-test.sh:97` (header), `:117`+`:124` (awk pattern and `END` printf), `:109`+`:137` (`read -r` list and `jq` arg/array); `docs/bench/crud-runs.csv:1` header with `mcp_contacts`, historical rows reset |
| FR-25 | Default surface unchanged by the three observable properties | PASS | Tool-name set: `TestRegisterTools_ContactsDisabled_StaysFourTools` + manual default `tools/list`. Manifest: `defaultCount` 38; the only removed line in the `surface.json` diff is `"fullCount": 52`, so no default entry changed. Scopes: manual default log `["Calendars.ReadWrite"]` |
| FR-26 | `BuildDomainVerbSets` gated on the same flag; no `"contacts"` key when off | PASS | `internal/server/introspect_verbs.go:113-133`; `TestBuildDomainVerbSets_ContactsFollowsFlag` |
| FR-27 | `domainOrder`, `fullConfig()`, `gateProbes()` all extended | PASS | `internal/surface/build.go:22` (contacts last), `:30`, `:71-78`; `TestEveryVerbCarriesSummaryAndGate` |
| FR-28 | `manifest_sync_test.go` modified: config, `!= 5`, both prose sites | PASS | `internal/server/manifest_sync_test.go:1-8,38-40,72,80-81,90-91`; no second test asserts the property |
| FR-29 | Thirteen hard-coded domain lists extended with configs; four-tool count assertions unchanged | PASS | All enumerated sites edited: `verb_metadata_test.go:98,150,162,204,233,376,381` (5 lists + configs), `description_quality_test.go:41,94,131,152,165` (4 lists + config), `tool_description_test.go:244,247,264,267` (2), `tool_annotations_test.go:387,390` (1), `surface_export_test.go:24,28` (1) — thirteen of thirteen against the amended FR-29 text. The list that sits **inside** `TestAggregateAnnotations_FourToolsRegistered` is now explicitly excluded by FR-29's own text rather than enumerated by it (GAP-6 FIXED, by CR amendment); the test is unchanged. Count assertions confirmed intact at `server_test.go:784,925,954,1051` and `tool_annotations_test.go:313` |
| FR-30 | Registry-derived domain list explicitly not attempted | PASS | No shared/derived domain-list helper introduced; every site remains a literal `[]string{...}` extended by hand |

### Non-Functional Requirements

| Req # | Description | Status | Evidence |
|---|---|---|---|
| NFR-1 | One handler file per verb, named for the verb | PASS | `internal/tools/contacts_{search,get_contact,list_people,get_person}.go` |
| NFR-2 | ≥ 60% reduction with contacts enabled; test config and comments updated to five | PASS | `internal/server/schema_size_test.go:33,36,39-40,65`; measured 23 461 bytes / 68% |
| NFR-3 | Instrument validated twice on unchanged input; figures recorded | PASS | Two `-count=1` runs, both `post-CR schema: 23461 bytes (5 tools)` / `reduction: 68%`; recorded in Summary |
| NFR-4 | File-local fix constants reaching both the tool result and the log record | PASS | Constants declared file-local: `contacts_search.go:34-39`, `contacts_get_contact.go:31-36`, `contacts_get_person.go:30-35`, `contacts_list_people.go:30-34`, each emitted to `logger.*(… "fix", …)` and to `mcp.NewToolResultError`. Both channels are now asserted: `TestContactsSearch_GraphFailureCarriesFix` (`internal/tools/contacts_search_test.go:310`) installs a buffer-backed `slog.TextHandler` as the default logger and requires `contactsSearchGraphFix` in the emitted record as well as in the result text (GAP-3 FIXED) |
| NFR-5 | No third-party dependency added | PASS | `go.mod` and `go.sum` absent from the branch diff |
| NFR-6 | One request for the three by-ID/list reads; exactly two for `search` | PASS | `TestContactsSearch_QueriesBothCollections` (`==2`), `TestGetContact_ReturnsEveryAddress` (`==1`), `TestGetPerson_LabelsAddressesWithDisplayName` (`==1`), `TestListPeople_PreservesRelevanceOrder` (`==1`) |
| NFR-7 | `RetryGraphCall` inside `WithTimeout`, redaction via shared helpers | PASS | `contacts_search.go:90-119,201-218`, `contacts_get_contact.go:76-102`, `contacts_get_person.go:75-101`, `contacts_list_people.go:68-94` |
| NFR-8 | Composed description below 4 000 characters; measured value recorded | PASS | Measured 1 044 characters via `tools/list` on the built binary; `TestDescriptionLengthBounded` |
| NFR-9 | Two distinct serializers, not one parameterised function | PASS | `internal/tools/contacts_serialize.go:45` / `:109` (summary) and `:65` / `:130` (raw); `TestSerializeSummaryContact_*`, `TestSerializeSummaryPerson_*` |

## Acceptance Criteria Verification

| AC # | Description | Status | Evidence |
|---|---|---|---|
| AC-1 | The fifth tool appears only when opted in | PASS | Manual `tools/list`: default 4 (`account,calendar,mail,system`), enabled 5 including `contacts`. Manual stderr: `"tool registration complete","tools":4` and `"tools":5`. `TestRegisterTools_ContactsEnabled_RegistersFifthTool`, `TestRegisterTools_ContactsDisabled_StaysFourTools`, `TestLoadConfig_ContactsEnabled`. Inventory clause held by the generated manifest's config section |
| AC-2 | Read-only domain with exactly five verbs; folded annotation | PASS | Manual `tools/list` operation enum = the five verbs; manual folded annotation on `contacts` = `readOnlyHint true, destructiveHint false, idempotentHint true, openWorldHint true`. `TestContactsVerbsRegisterFive`, `TestContactsExposesNoWriteVerb`, `TestContactsVerbsWrappedUnderDomainIdentity`, `dispatch_registry_test.go:67-71` golden (per-verb four hints, `ow=false` only for `help`) |
| AC-3 | Search resolves a name across both resources | PASS | `TestContactsSearch_QueriesBothCollections`, `TestContactsSearch_SendsIdenticalNormalisedQuery`, `TestContactsSearch_LabelsEveryMatch` (3 matches, sources `personal contact` then `ranked person`) |
| AC-4 | Search refuses an empty query, and an unconvertible one, before any call | PASS | `TestContactsSearch_RequiresQuery` (error names `query`, 0 requests), `TestContactsSearch_RejectsUnconvertibleQuery` (0 requests, normaliser's own error surfaced at `contacts_search.go:84-85`) |
| AC-5 | Fetch by identifier; invalid one never reaches Graph | PASS | `TestGetContact_ReturnsEveryAddress`, `TestGetPerson_LabelsAddressesWithDisplayName` (1 request each), `TestGetContact_RejectsEmptyIdentifier`, `TestGetContact_RejectsOverlongIdentifier`, `TestGetPerson_RejectsEmptyIdentifier`, `TestGetPerson_RejectsOverlongIdentifier` (0 requests each) |
| AC-6 | Surface manifest records the gated domain and the new totals | PASS | `make ci` exit 0 with a clean tree after `surface-check`; measured `totals 57/38`, contacts `5/0`, gate on each verb, config section names the variable; `TestCommittedManifestMatchesRecord`, `TestContactsDomainRecordedGatedAndFull` |
| AC-7 | Consent surface changes only on opt-in, never to a write scope | PASS | Manual startup log: `["Calendars.ReadWrite"]` default vs. `["Calendars.ReadWrite","Contacts.Read","People.Read"]` enabled; `TestScopes_Contacts`, `TestScopes_NoContactsByDefault`, `TestScopes_NoContactsWriteEver` |
| AC-8 | Registry metadata complete for every new verb | PASS | `TestEveryVerbHasSummary`, `TestEveryVerbHasDescription`, `TestEveryVerbHasClassification`, `TestEveryVerbStatesRequiredParameters`, `TestSeeDocsAnchorsResolve`, `TestEveryParameterHasDescription`, `TestDescriptionLengthBounded` — all now iterate `contacts`. Measured composed description 1 044 chars |
| AC-9 | Read verbs implement all three output tiers | PASS | Implemented for all four verbs (`contacts_search.go:127-145`, `contacts_get_contact.go:106-125`, `contacts_list_people.go:96-120`, `contacts_get_person.go:105-124`) and `text` is the default. All four verbs now cover all three tiers: `search` text, summary, and raw (`TestContactsSearch_RawTierCarriesDetail`, `contacts_search_test.go:245`, asserting the raw-only `companyName`, `mobilePhone` and `scoredEmailAddresses` fields and the retained source label); `get_contact` all three; `list_people` all three; `get_person` text, raw, and summary (`TestGetPerson_SummaryTierCarriesResolutionFields`, `contacts_get_person_test.go:104`, asserting the resolution field set, the relevance score, and the absence of raw-only detail) — GAP-5 FIXED. Relevance-order clause held by `TestListPeople_PreservesRelevanceOrder` and `TestContactsSearch_PreservesRelevanceOrder`; distinct-serializer clause by `TestSerializeSummary_EmptyRecordsProjectStably` |
| AC-10 | Cold-start schema gate re-established over five tools | PASS | 23 461 bytes / 68% reduction, identical across two runs on unchanged input; `TestColdStartSchemaSize_Reduction` |
| AC-11 | The four-tool rule and documents are amended coherently | PASS | Every FR-22-enumerated site amended; `AGENTS.md` edited once with `CLAUDE.md` untouched; manifest holds exactly 5 entries; `## Contacts gating` H2 present; scopes table names both scopes. The repository-wide clause now holds: `docs/reference/release.md:113` names the four default tools plus the opt-in `contacts` entry and states why five are published; `docs/reference/architecture.md:5` names the opt-in fifth. A fresh repository-wide grep for "four aggregate", "four domain", "four tools" and "four MCP" outside the governance and backlog files returns only statements that are correct as written — the default-configuration smoke test (`scripts/smoke-test-image.sh:40`, `docs/reference/release.md:138`), the default-surface prose in `docs/concepts.md:71,75`, the "four MCP annotation hints" statements, which are about hints and not tools, and a historical run record under `.agents/scenarios/` (GAP-1 FIXED) |
| AC-12 | CRUD harness lifecycles the new domain | PASS | `docs/prompts/mcp-tool-crud-test.md:745-790` (Steps 47-51, all five verbs, each "skip if contacts disabled"); all three `scripts/crud-test.sh` edits present; `docs/bench/crud-runs.csv` header matches the script's emitted schema and carries no short historical row (rows reset). Harness **not executed** — GAP-7, recorded `not-run` and pending for the user |
| AC-13 | Each verb issues the intended number of Graph requests | PASS | `callCount()` assertions: 2 for `search`, 1 for `get_contact`, `get_person`, `list_people`; no per-result fetch in `mergeContactMatches` (`contacts_search.go:161-185`) |
| AC-14 | Failures carry a correction on both channels | PASS | Tool-result channel tested: `TestContactsSearch_GraphFailureCarriesFix`, `TestContactsSearch_NoAccountCarriesFix`, `TestListPeople_NoAccountCarriesFix`, `TestGetPerson_RejectsEmptyIdentifier`. Log channel now held by a captured handler: `TestContactsSearch_GraphFailureCarriesFix` swaps `slog.Default()` for a buffer-backed text handler and asserts the record carries the same constant as the result (`contacts_search_test.go:310`), following the established `TestMoveMessage_UnresolvableDestinationCarriesFix` pattern. Emission sites: `contacts_search.go:211-215`, `contacts_get_contact.go:96-99`, `contacts_get_person.go:95-98`, `contacts_list_people.go:88-91` (GAP-3 FIXED) |
| AC-15 | File and helper conventions followed | PASS | Per-verb files under `internal/tools/`; fix instructions as file-local constants; `contactsVerbsConfig` carries `retryCfg`, `timeout`, `m`, `tracer`, `authMW`, `accountResolverMW` and no `readOnly` (`contacts_verbs.go:34-53`), following `mailVerbsConfig`; timeout message names the seconds via `graph.TimeoutErrorMessage(int(timeout.Seconds()))`; no dependency added |
| AC-16 | The domain reaches both builders and the surface generator | PASS | `TestBuildDomainVerbSets_ContactsFollowsFlag` (five verbs when on, no key when off); `TestBuildVerbsRequiresNoCredentials` now includes `contacts`; `internal/surface/build.go:22` places contacts after `system`; `TestEveryVerbCarriesSummaryAndGate` and the measured manifest attribute every contacts verb to `OUTLOOK_MCP_CONTACTS_ENABLED` |
| AC-17 | Registry-derived checks extended rather than bypassed | PASS | `TestManifestDescribesEveryRegisteredVerb` builds from the registry under `maximalSurfaceConfig()` with `ContactsEnabled: true` and asserts exactly 5 manifest tools; it is the only test asserting that property. Registry-metadata, description-quality and annotation lists all name `contacts`; the four-tool count assertions at `server_test.go:784,925,954,1051` and `tool_annotations_test.go:313` are unchanged and pass |

## Test Strategy Verification

`make ci` ran the whole suite: **exit 0**, all packages `ok`. The 61 CR-relevant tests
re-run individually all pass (contacts handlers and serializers: 31; server/surface/auth/config
registration and gating: 10; registry-metadata, description-quality, annotation and golden:
20). No test failed, skipped, or was quarantined.

### Tests to Add

| Test File | Test Name (as specified) | Specified | Exists | Matches Spec |
|---|---|---|---|---|
| `contacts_search_test.go` | `TestContactsSearch_QueriesBothResources` | yes | renamed → `TestContactsSearch_QueriesBothCollections` | yes — asserts 2 requests, `/contacts` then `/people` |
| `contacts_search_test.go` | `TestContactsSearch_LabelsSource` | yes | renamed → `TestContactsSearch_LabelsEveryMatch` | yes |
| `contacts_search_test.go` | `TestContactsSearch_RejectsEmptyQuery` | yes | renamed → `TestContactsSearch_RequiresQuery` | yes — `"   "`, error names `query`, 0 requests |
| `contacts_search_test.go` | `TestContactsSearch_SendsIdenticalNormalisedValueToBoth` | yes | renamed → `TestContactsSearch_SendsIdenticalNormalisedQuery` | partial — asserts the two `$search` values are equal and normalised-quoted, but does not compare against `NormaliseSearchQuery("Alex Smith")` directly |
| `contacts_search_test.go` | `TestContactsSearch_RejectsUnconvertibleQueryBeforeCall` | yes | renamed → `TestContactsSearch_RejectsUnconvertibleQuery` | yes — 0 requests |
| `contacts_search_test.go` | `TestContactsSearch_TierSummarySelectsFields` | yes | covered by `TestContactsSearch_LabelsEveryMatch` / `_PreservesRelevanceOrder` (`output=summary`) and `TestContactsSearch_RawTierCarriesDetail` | yes — the raw case asserts fields the summary omits, which is what distinguishes a deliberate field set from a filtered one |
| `contacts_search_test.go` | `TestContactsSearch_ErrorCarriesFixOnBothChannels` | yes | renamed → `TestContactsSearch_GraphFailureCarriesFix` | yes — a buffer-backed `slog.TextHandler` is installed as the default logger and both the result text and the log record are asserted to carry `contactsSearchGraphFix` |
| `contacts_get_contact_test.go` | `TestGetContact_Success` | yes | renamed → `TestGetContact_ReturnsEveryAddress` | yes |
| `contacts_get_contact_test.go` | `TestGetContact_InvalidIDRejectedBeforeCall` | yes | split → `TestGetContact_RejectsEmptyIdentifier`, `TestGetContact_RejectsOverlongIdentifier` | yes — over-length case present, as FR-8 requires |
| `contacts_get_contact_test.go` | `TestGetContact_AllThreeTiers` | yes | split → `_ReturnsEveryAddress` (text), `_SummaryTierCarriesResolutionFields`, `_RawTierCarriesDetail` | yes — all three tiers covered |
| `contacts_get_contact_test.go` | `TestGetContact_RendersEveryEmailAddress` | yes | merged into `TestGetContact_ReturnsEveryAddress` | yes — every address rendered, `callCount()==1` |
| `contacts_list_people_test.go` | `TestListPeople_PreservesRelevanceOrder` | yes | yes | yes |
| `contacts_list_people_test.go` | `TestListPeople_AllThreeTiers` | yes | split → `_PreservesRelevanceOrder` (summary), `_TextStatesTotalAndOrder`, `_RawTierCarriesDetail` | yes — all three tiers covered |
| `contacts_get_person_test.go` | `TestGetPerson_Success` | yes | renamed → `TestGetPerson_LabelsAddressesWithDisplayName` | yes — 1 request |
| `contacts_get_person_test.go` | `TestGetPerson_InvalidIDRejectedBeforeCall` | yes | split → `_RejectsEmptyIdentifier`, `_RejectsOverlongIdentifier` | yes |
| `contacts_get_person_test.go` | `TestGetPerson_RendersScoredEmailAddresses` | yes | covered by `_LabelsAddressesWithDisplayName` + `_RawTierCarriesScoredAddresses` | yes — both addresses rendered, labelled with the person's display name |
| `contacts_get_person_test.go` | `TestGetPerson_AllThreeTiers` | yes | split → `_LabelsAddressesWithDisplayName` (text), `_SummaryTierCarriesResolutionFields`, `_RawTierCarriesScoredAddresses` | yes — all three tiers covered |
| `contacts_serialize_test.go` | `TestSummarySerializersAreDistinctAndDeliberate` | yes | split → `TestSerializeSummaryContact_CarriesResolutionFields`, `_KeepsEveryAddress`, `TestSerializeSummaryPerson_CarriesRelevanceScore`, `TestSerializeSummary_EmptyRecordsProjectStably` | yes — the empty-record case is the discriminator the spec asked for |
| `tool_annotations_test.go` | `TestContactsVerbAnnotations` | yes | satisfied by `verbInventoryGolden` (`dispatch_registry_test.go:67-71`) | yes — the exact per-verb `ro/de/id/ow` values are asserted, in the same package under a different file; no second test restates the property |
| `tool_annotations_test.go` | `TestContactsAggregateIsReadOnly` | yes | yes (`internal/tools/tool_annotations_test.go:303`) | yes — asserts the folded values on the registered `contacts` tool under `ContactsEnabled: true`: title `Contacts`, `readOnly` true, `destructive` false, `idempotent` true, `openWorld` true, matching the manually observed `tools/list` |
| `contacts_verbs_test.go` | `TestContactsVerbsRegisterFive` | yes | yes | yes |
| `contacts_verbs_test.go` | `TestContactsExposesNoWriteVerb` | yes | yes | yes — asserts `readOnlyHint` true and `destructiveHint` false on every verb |
| `contacts_verbs_test.go` | `TestContactsVerbsWrappedUnderDomainIdentity` | yes | yes | yes — audit records carry `contacts.<verb>` |
| `server_test.go` | `TestRegisterTools_ContactsEnabled_RegistersFifthTool` | yes | yes | yes |
| `server_test.go` | `TestRegisterTools_ContactsDisabled_StaysFourTools` | yes | yes | yes — graded by name, not only count |
| `introspect_verbs_test.go` | `TestBuildDomainVerbSets_ContactsFollowsFlag` | yes | yes | yes |
| `surface_test.go` | `TestContactsDomainRecordedGatedAndFull` | yes | yes | yes |
| `config_test.go` | `TestInventoryNamesContactsFlag` | yes | yes (`internal/config/config_test.go:982`) | yes — asserts the `OUTLOOK_MCP_CONTACTS_ENABLED` row exists in `config.Inventory()` with default `"false"` and a non-empty description |
| `auth_test.go` | `TestScopes_Contacts` | yes | yes | yes |
| `auth_test.go` | `TestScopes_NoContactsByDefault` | yes | yes | yes |
| `auth_test.go` | `TestScopes_NoContactsWriteEver` | yes | yes | yes |
| `config_test.go` | `TestLoadConfig_ContactsEnabled` | yes | yes | yes — five sub-cases, plus a no-implied-flag assertion |

### Tests to Modify

| Test File | Test Name | Modified as specified |
|---|---|---|
| `schema_size_test.go` | `TestColdStartSchemaSize_Reduction` | yes — `ContactsEnabled: true` at `:65`, comments at `:33,36,39-40` updated to five |
| `dispatch_registry_test.go` | `TestVerbInventoryUnchangedAfterUpgrade` | yes — exactly five contacts identities added (`:67-71`), config gains the gate (`:126`) |
| `server_test.go` | `TestRegisterTools_MailEnabled` | yes — unchanged and re-confirmed passing |
| `manifest_sync_test.go` | `TestManifestDescribesEveryRegisteredVerb` | yes — config, `!= 5`, and both prose sites |
| `verb_metadata_test.go` | 5 tests | yes — all five lists plus both server configs |
| `description_quality_test.go` | 4 tests | yes — all four lists plus `fullSurfaceConfig()` |
| `tool_description_test.go` | 2 loops | yes |
| `tool_annotations_test.go` | 1 loop (line 389) | yes — the amended FR-29 enumerates line 389 alone and states why the list inside `TestAggregateAnnotations_FourToolsRegistered` is excluded; that test is unchanged |
| `surface_export_test.go` | `TestBuildVerbsRequiresNoCredentials` | yes |

### Tests Added in the Gap-Fix Round

| Test | File | Closes | Evidence |
|---|---|---|---|
| `TestInventoryNamesContactsFlag` | `internal/config/config_test.go:982` | GAP-2 | Row name, `"false"` default, non-empty description asserted against `config.Inventory()` |
| `TestContactsAggregateIsReadOnly` | `internal/tools/tool_annotations_test.go:303` | GAP-4 | Folded `contacts` annotation: `Contacts` / true / false / true / true |
| `TestContactsSearch_RawTierCarriesDetail` | `internal/tools/contacts_search_test.go:245` | GAP-5 | Raw tier carries `companyName`, `mobilePhone`, `scoredEmailAddresses`, and retains the source label |
| `TestGetPerson_SummaryTierCarriesResolutionFields` | `internal/tools/contacts_get_person_test.go:104` | GAP-5 | Summary tier carries `displayName`, `emailAddress`, `relevanceScore` and omits `jobTitle` |
| `TestContactsSearch_GraphFailureCarriesFix` (extended) | `internal/tools/contacts_search_test.go:310` | GAP-3 | Captured `slog` buffer asserts the fix constant on the log record as well as the result |

All five run and pass under `make ci`, and each was confirmed to execute rather than be
skipped by a `-run`-filtered verbose run.

### Existing Tests That Gate Without Modification

All pass: `TestCommittedManifestMatchesRecord`, `TestRecordCountsMatchBuiltVerbs`,
`TestDefaultCountExcludesGatedVerbs`, `TestEveryVerbCarriesSummaryAndGate`,
`TestVerbInventoryUnchangedAfterUpgrade`, `TestLLMsTxt_MatchesCatalog`,
`TestLLMsTxt_StructureCompliesWithStandard`, `TestLLMsTxt_LinksAreAbsolute`.

## Diff Coverage

`git diff 3ae0e2c...HEAD --stat`: 48 files, +4141 / −317.

| File | +/- | Mapped Requirements |
|---|---|---|
| `internal/server/contacts_verbs.go` | +229 | FR-3, FR-10, FR-12, FR-13, FR-17, FR-18, AC-2, AC-15 |
| `internal/tools/contacts_search.go` | +218 | FR-4, FR-9, NFR-4, NFR-6, NFR-7, AC-3, AC-4, AC-13, AC-14 |
| `internal/tools/contacts_get_contact.go` | +127 | FR-5, FR-8, NFR-1, NFR-6, NFR-7, AC-5 |
| `internal/tools/contacts_get_person.go` | +126 | FR-7, FR-8, NFR-1, NFR-6, NFR-7, AC-5 |
| `internal/tools/contacts_list_people.go` | +146 | FR-6, NFR-1, NFR-6, NFR-7 |
| `internal/tools/contacts_serialize.go` | +267 | FR-5, FR-7, FR-11, NFR-9, AC-9 |
| `internal/tools/text_format.go` | +145 | FR-10, AC-9 (four contacts formatters) |
| `internal/server/server.go` | +40/−4 | FR-1, FR-2, FR-21, FR-22, AC-1 |
| `internal/server/introspect_verbs.go` | +34/−8 | FR-26, AC-16 |
| `internal/surface/build.go` | +13/−4 | FR-27, FR-20, AC-16 |
| `internal/config/config.go` | +13 | FR-16, AC-1 |
| `internal/config/inventory.go` | +2 | FR-16, AC-6 |
| `internal/auth/auth.go` | +31/−4 | FR-14, FR-15, AC-7 |
| `internal/server/surface_export.go` | +3/−2 | FR-22 |
| `internal/tools/aggregate_annotations.go` | +5/−3 | FR-22 |
| `internal/docs/llmstxt.go` | +4/−2 | FR-22 |
| `extension/manifest.json` | +6/−2 | FR-19, AC-11 |
| `site/src/generated/surface.json` | +44/−1 | FR-20, AC-6 |
| `site/src/surface.ts` | +6/−3 | FR-22 |
| `AGENTS.md`, `README.md`, `docs/readme.md`, `docs/concepts.md` | +5/−2, +3/−1, +3/−1, +19/−3 | FR-22, FR-23, AC-11 |
| `docs/prompts/mcp-tool-crud-test.md`, `scripts/crud-test.sh`, `docs/bench/crud-runs.csv` | +50, +10/−5, +1/−6 | FR-24, AC-12 |
| `internal/server/{contacts_verbs,introspect_verbs}_test.go` | +175, +49 | FR-3, FR-13, FR-26, AC-2, AC-16 |
| `internal/tools/contacts_*_test.go` (5 files) | +913 | FR-4..FR-11, NFR-6, AC-3..AC-5, AC-9, AC-13 |
| `internal/server/{server,manifest_sync,schema_size,surface_export}_test.go` | +79, +17/−8, +12/−5, +17/−7 | FR-1, FR-2, FR-25, FR-28, FR-29, NFR-2, AC-1, AC-10, AC-17 |
| `internal/surface/surface_test.go` | +36 | FR-20, AC-6 |
| `internal/{auth,config}/*_test.go` | +79, +37 | FR-14..FR-16, AC-7 |
| `internal/tools/{verb_metadata,description_quality,tool_description,tool_annotations,dispatch_registry}_test.go` | +16/−7, +9/−4, +6/−3, +3/−2, +11/−4 | FR-12, FR-29, AC-8, AC-17 |
| `docs/cr/CR-0082-contacts-domain.md` | +1225/−~300 | The CR itself (review and finalize commits) |

### Unmapped changed files

Two files outside the CR's Affected Components. Both are justified:

* `llms.txt` (+4/−2) — generated from `internal/docs/llmstxt.go` (an enumerated FR-22 site) by
  `make docs-bundle` inside `make ci`. Committing it is required: `TestLLMsTxt_MatchesCatalog`
  fails otherwise. A derivative of a mapped change, not an independent edit.
* `docs/backlog/cr-0078-0083.md` (+214) — the CR's own review summary states the thirteen
  author decisions taken at review are "recorded under `## CR-0082` in
  docs/backlog/cr-0078-0083.md". Confirmed present at `docs/backlog/cr-0078-0083.md:800`.

No stray source file. No file outside the governance, documentation, harness, and
`internal/{server,tools,surface,config,auth,docs}` scope the CR declares.

## Gaps

Six of the nine are fixed in this round. Three are deferred verifications that need an
instrument this session must not run: a paid harness, a site screenshot comparison, and a
live mailbox. Those three are marked `not-run`, not closed, and each is recorded as pending
for the user under `## CR-0082` in `docs/backlog/cr-0078-0083.md`.

**GAP-1 — FIXED. AC-11's repository-wide clause now holds.**
`docs/reference/release.md:113` named four manifest entries where five are published. It now
names the four default tools plus the opt-in `contacts` entry, states the total, and says why
the gated domain is published anyway: the manifest describes the surface a user may opt into,
not the default one. `docs/reference/architecture.md:5` gained the same correction to its
status note. A fresh repository-wide grep for "four aggregate", "four domain", "four tools"
and "four MCP" outside `docs/cr/`, `docs/adr/`, `docs/backlog/` and this report returns only
statements that are correct as written: the default-configuration smoke test
(`scripts/smoke-test-image.sh:40`, `docs/reference/release.md:138`, both left untouched as
the gap note directed), the default-surface prose in `docs/concepts.md:71,75`, the "four MCP
annotation hints" statements, which are about hints and not tools, and one historical run
record under `.agents/scenarios/`. Six stale test-helper and package comments that described
their own builders as registering "all four domain tools" were corrected in the same pass,
along with `internal/server/surface_export.go:2` and `.agents/scripts/site.content.check.mjs:251`.
`docs/embed_test.go`'s manually maintained `knownVerbNames` gained the four contacts verbs,
since the comment above it claims to span every domain registry and the heading check it
feeds was otherwise blind to them.

**GAP-2 — FIXED. `TestInventoryNamesContactsFlag` exists.**
`internal/config/config_test.go:982` asserts `config.Inventory()` carries a row named
`OUTLOOK_MCP_CONTACTS_ENABLED` with default `"false"` and a non-empty description, and fails
when no such row is found. The inventory is what the published configuration surface is
generated from, so this is the binding a `LoadConfig` test cannot hold.

**GAP-3 — FIXED. The fix instruction's log channel is now asserted.**
`TestContactsSearch_GraphFailureCarriesFix` (`internal/tools/contacts_search_test.go:310`)
installs a buffer-backed `slog.NewTextHandler` as the default logger for the duration of the
test, restores the prior default afterwards, and requires `contactsSearchGraphFix` to appear
both in the result text and in the emitted record. This is the pattern
`TestMoveMessage_UnresolvableDestinationCarriesFix` established
(`internal/tools/move_message_test.go:214`), reused rather than reinvented. A refactor
dropping the `"fix"` attribute now fails a test, which is the drift NFR-4 exists to prevent.

**GAP-4 — FIXED. The folded `contacts` annotation is asserted.**
`TestContactsAggregateIsReadOnly` (`internal/tools/tool_annotations_test.go:303`) builds the
server under `ContactsEnabled: true` and asserts the registered `contacts` tool's four folded
hints through the existing `assertAggregateAnnotations` helper: title `Contacts`,
`readOnlyHint` true, `destructiveHint` false, `idempotentHint` true, `openWorldHint` true.
These are the values previously observed only by hand via `tools/list`. The per-verb matrix
remains held by `verbInventoryGolden`, so the two together cover both granularities and
`AGENTS.md`'s requirement that a new verb add a value assertion beside the existing
annotation tests is satisfied.

**GAP-5 — FIXED. Both unexercised tiers now have tests.**
`TestContactsSearch_RawTierCarriesDetail` (`contacts_search_test.go:245`) asserts the raw
tier carries `companyName` and `mobilePhone` on the saved half and `scoredEmailAddresses` on
the ranked half, and that the source label survives the escalation. Asserting fields the
summary deliberately omits is also what distinguishes a dedicated serializer from a filter,
so it strengthens the FR-11 evidence.
`TestGetPerson_SummaryTierCarriesResolutionFields` (`contacts_get_person_test.go:104`)
asserts the summary tier's `displayName`, leading `emailAddress` and `relevanceScore`, and
the absence of the raw-only `jobTitle`.

**GAP-6 — FIXED by CR amendment; the test remains unchanged.**
FR-29 enumerated a domain list inside `TestAggregateAnnotations_FourToolsRegistered` while
its own closing sentence forbade changing that test. The requirement now enumerates thirteen
sites rather than fourteen, drops that line from the list, and states why it is excluded:
the list is that test's own default-surface expectation, which is the guarantee FR-25 rests
on, so extending it would contradict the same requirement. The Current State bullet, the
Tests-to-Modify row, FR-30's "fourteen sites", the risk mitigation, the effort table and the
dependency list were all moved with it. The amendment is recorded under `## CR-0082` in
`docs/backlog/cr-0078-0083.md`. **The test was not touched**, and the FR-29 row is graded
PASS against the amended text.

**GAP-7 — NOT RUN. The CRUD harness is a paid, live-mailbox instrument.**
`make crud-test` was deliberately not executed here. AC-12 grades the prompt, the script and
the CSV header by reading them, which passes; nothing has yet exercised Steps 47-51 against a
real mailbox, so the `mcp_contacts` accounting column and the five new prompt steps remain
unproven end to end.
*Pending for the user:* rebuild to the path `.mcp.json` names, then run `make crud-test` with
`OUTLOOK_MCP_CONTACTS_ENABLED=true` and confirm the emitted CSV row carries the full column
count. Read the report's own `Server version` line before trusting a row of it. Recorded in
the backlog, and cross-referenced from the GAP-9 scenario, which is the cheaper instrument
for the question most likely to fail.

**GAP-8 — NOT RUN. The site screenshot comparison needs a rendered before-and-after.**
`site/src/generated/surface.json` gained a fifth domain and `site/src/surface.ts` changed, so
the published site renders an additional domain and different derived counts. That is an
intended content change rather than a claim that rendering is untouched, but the delta is
unverified.
*Pending for the user:* run the site's screenshot comparison against a build of `main` and
confirm the only visual delta is the added contacts domain and the moved totals. Recorded in
the backlog.

**GAP-9 — NOT RUN. `$search` on `/me/contacts` is still proved only at the SDK layer.**
Kiota generates the `Search` field uniformly across collections, so its presence is not
evidence Graph v1.0 honours `$search` for the personal-contacts collection, and no
`ConsistencyLevel: eventual` header is set on this path. A derived, not-yet-run scenario is
written at `.agents/scenarios/2026-09-02-contacts-name-resolution-live-mailbox.md`, in the
format of the existing scenarios there and marked `outcome: not-run`. It drives the five
verbs at the MCP surface against a live mailbox, and its step 3 is the single call that
settles the question.
*Pending for the user:* run that scenario. If Graph rejects `$search` on `/me/contacts`,
record the verbatim error, mark the scenario `reproduced`, and open a follow-on change
request; do not patch the verb from that run, as the CR directs. Recorded in the backlog.

---

No FAIL, no GAP, no unresolved PARTIAL. GAP-1 through GAP-6 are fixed with the evidence
traced above; GAP-7 through GAP-9 are deferred verifications rather than implementation
defects, carried as `not-run` with a named owner action each. `make ci` exits **0** and the
working tree is clean after the checkpoint, so the surface-drift gate and the `llms.txt`
regeneration both still agree with what is committed.
