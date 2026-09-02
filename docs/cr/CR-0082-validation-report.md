# CR-0082 Validation Report

Validated at `43b39b8` on branch `docs/cr-implementation-set-0079-0083`.
Diff base `3ae0e2c` (last CR-0081 commit); implementation commits `012f02b`, `9a3a493`,
`27e0473`, `8cf26e1`.

## Summary

Requirements: 37/39 | Acceptance Criteria: 14/17 | Tests: 61/61 run passing (30 specified rows: 21 satisfied under a renamed equivalent, 4 GAP) | Gaps: 9

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
| FR-16 | `ContactsEnabled` field, `LoadConfig` read, inventory const and row | PASS | `internal/config/config.go:148-154,325-327`; `internal/config/inventory.go:44,93`; `TestLoadConfig_ContactsEnabled`; `site/src/generated/surface.json` config section names `OUTLOOK_MCP_CONTACTS_ENABLED`, held by `TestCommittedManifestMatchesRecord`. (The CR-specified direct assertion on `config.Inventory()` is absent — GAP-2.) |
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
| FR-29 | Fourteen hard-coded domain lists extended with configs; four-tool count assertions unchanged | PARTIAL | 13 of 14 sites edited: `verb_metadata_test.go:98,150,162,204,233,376,381` (5 lists + configs), `description_quality_test.go:41,94,131,152,165` (4 lists + config), `tool_description_test.go:244,247,264,267` (2), `tool_annotations_test.go:387,390` (1), `surface_export_test.go:24,28` (1). The fourteenth (`tool_annotations_test.go:328` pre-change) sits **inside** `TestAggregateAnnotations_FourToolsRegistered`, which the same FR says MUST NOT be changed — see GAP-6. Count assertions confirmed intact at `server_test.go:784,925,954,1051` and `tool_annotations_test.go:313` |
| FR-30 | Registry-derived domain list explicitly not attempted | PASS | No shared/derived domain-list helper introduced; every site remains a literal `[]string{...}` extended by hand |

### Non-Functional Requirements

| Req # | Description | Status | Evidence |
|---|---|---|---|
| NFR-1 | One handler file per verb, named for the verb | PASS | `internal/tools/contacts_{search,get_contact,list_people,get_person}.go` |
| NFR-2 | ≥ 60% reduction with contacts enabled; test config and comments updated to five | PASS | `internal/server/schema_size_test.go:33,36,39-40,65`; measured 23 461 bytes / 68% |
| NFR-3 | Instrument validated twice on unchanged input; figures recorded | PASS | Two `-count=1` runs, both `post-CR schema: 23461 bytes (5 tools)` / `reduction: 68%`; recorded in Summary |
| NFR-4 | File-local fix constants reaching both the tool result and the log record | PARTIAL | Constants declared file-local: `contacts_search.go:34-39`, `contacts_get_contact.go:31-36`, `contacts_get_person.go:30-35`, `contacts_list_people.go:30-34`, each emitted to `logger.*(… "fix", …)` and to `mcp.NewToolResultError`. **No test asserts the log channel**; the CR-specified two-channel test was implemented as a result-only assertion — GAP-3 |
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
| AC-9 | Read verbs implement all three output tiers | PARTIAL | Implemented for all four verbs (`contacts_search.go:127-145`, `contacts_get_contact.go:106-125`, `contacts_list_people.go:96-120`, `contacts_get_person.go:105-124`) and `text` is the default. Tested tiers: `search` text+summary (raw **untested**), `get_contact` all three, `list_people` all three, `get_person` text+raw (summary **untested**) — GAP-5. Relevance-order clause held by `TestListPeople_PreservesRelevanceOrder` and `TestContactsSearch_PreservesRelevanceOrder`; distinct-serializer clause by `TestSerializeSummary_EmptyRecordsProjectStably` |
| AC-10 | Cold-start schema gate re-established over five tools | PASS | 23 461 bytes / 68% reduction, identical across two runs on unchanged input; `TestColdStartSchemaSize_Reduction` |
| AC-11 | The four-tool rule and documents are amended coherently | PARTIAL | Every FR-22-enumerated site amended; `AGENTS.md` edited once with `CLAUDE.md` untouched; manifest holds exactly 5 entries; `## Contacts gating` H2 present; scopes table names both scopes. **The repository-wide clause fails**: `docs/reference/release.md:113` still asserts the manifest "contains the four aggregate domain tools (`calendar`, `mail`, `account`, `system`)", which the five-entry manifest now contradicts — GAP-1 |
| AC-12 | CRUD harness lifecycles the new domain | PASS | `docs/prompts/mcp-tool-crud-test.md:745-790` (Steps 47-51, all five verbs, each "skip if contacts disabled"); all three `scripts/crud-test.sh` edits present; `docs/bench/crud-runs.csv` header matches the script's emitted schema and carries no short historical row (rows reset). Harness **not executed** — GAP-7 |
| AC-13 | Each verb issues the intended number of Graph requests | PASS | `callCount()` assertions: 2 for `search`, 1 for `get_contact`, `get_person`, `list_people`; no per-result fetch in `mergeContactMatches` (`contacts_search.go:161-185`) |
| AC-14 | Failures carry a correction on both channels | PARTIAL | Tool-result channel tested: `TestContactsSearch_GraphFailureCarriesFix`, `TestContactsSearch_NoAccountCarriesFix`, `TestListPeople_NoAccountCarriesFix`, `TestGetPerson_RejectsEmptyIdentifier`. Log channel has code evidence only (`contacts_search.go:211-215`, `contacts_get_contact.go:96-99`, `contacts_get_person.go:95-98`, `contacts_list_people.go:88-91`) and **no test captures a log handler** — GAP-3 |
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
| `contacts_search_test.go` | `TestContactsSearch_TierSummarySelectsFields` | yes | covered by `TestContactsSearch_LabelsEveryMatch` / `_PreservesRelevanceOrder` (both `output=summary`) | partial — summary tier exercised; no assertion that the field set is the serializer's rather than a filter |
| `contacts_search_test.go` | `TestContactsSearch_ErrorCarriesFixOnBothChannels` | yes | renamed → `TestContactsSearch_GraphFailureCarriesFix` | **no** — asserts the tool result only; no captured log handler (GAP-3) |
| `contacts_get_contact_test.go` | `TestGetContact_Success` | yes | renamed → `TestGetContact_ReturnsEveryAddress` | yes |
| `contacts_get_contact_test.go` | `TestGetContact_InvalidIDRejectedBeforeCall` | yes | split → `TestGetContact_RejectsEmptyIdentifier`, `TestGetContact_RejectsOverlongIdentifier` | yes — over-length case present, as FR-8 requires |
| `contacts_get_contact_test.go` | `TestGetContact_AllThreeTiers` | yes | split → `_ReturnsEveryAddress` (text), `_SummaryTierCarriesResolutionFields`, `_RawTierCarriesDetail` | yes — all three tiers covered |
| `contacts_get_contact_test.go` | `TestGetContact_RendersEveryEmailAddress` | yes | merged into `TestGetContact_ReturnsEveryAddress` | yes — every address rendered, `callCount()==1` |
| `contacts_list_people_test.go` | `TestListPeople_PreservesRelevanceOrder` | yes | yes | yes |
| `contacts_list_people_test.go` | `TestListPeople_AllThreeTiers` | yes | split → `_PreservesRelevanceOrder` (summary), `_TextStatesTotalAndOrder`, `_RawTierCarriesDetail` | yes — all three tiers covered |
| `contacts_get_person_test.go` | `TestGetPerson_Success` | yes | renamed → `TestGetPerson_LabelsAddressesWithDisplayName` | yes — 1 request |
| `contacts_get_person_test.go` | `TestGetPerson_InvalidIDRejectedBeforeCall` | yes | split → `_RejectsEmptyIdentifier`, `_RejectsOverlongIdentifier` | yes |
| `contacts_get_person_test.go` | `TestGetPerson_RendersScoredEmailAddresses` | yes | covered by `_LabelsAddressesWithDisplayName` + `_RawTierCarriesScoredAddresses` | yes — both addresses rendered, labelled with the person's display name |
| `contacts_get_person_test.go` | `TestGetPerson_AllThreeTiers` | yes | text + raw only | **no** — summary tier untested (GAP-5) |
| `contacts_serialize_test.go` | `TestSummarySerializersAreDistinctAndDeliberate` | yes | split → `TestSerializeSummaryContact_CarriesResolutionFields`, `_KeepsEveryAddress`, `TestSerializeSummaryPerson_CarriesRelevanceScore`, `TestSerializeSummary_EmptyRecordsProjectStably` | yes — the empty-record case is the discriminator the spec asked for |
| `tool_annotations_test.go` | `TestContactsVerbAnnotations` | yes | **absent** from `tool_annotations_test.go` | partial — the exact per-verb hint values are asserted by `verbInventoryGolden` (`dispatch_registry_test.go:67-71`), same package, different file (GAP-4) |
| `tool_annotations_test.go` | `TestContactsAggregateIsReadOnly` | yes | **absent** | **no** — no automated assertion on the folded `contacts` tool annotation; verified manually via `tools/list` (GAP-4) |
| `contacts_verbs_test.go` | `TestContactsVerbsRegisterFive` | yes | yes | yes |
| `contacts_verbs_test.go` | `TestContactsExposesNoWriteVerb` | yes | yes | yes — asserts `readOnlyHint` true and `destructiveHint` false on every verb |
| `contacts_verbs_test.go` | `TestContactsVerbsWrappedUnderDomainIdentity` | yes | yes | yes — audit records carry `contacts.<verb>` |
| `server_test.go` | `TestRegisterTools_ContactsEnabled_RegistersFifthTool` | yes | yes | yes |
| `server_test.go` | `TestRegisterTools_ContactsDisabled_StaysFourTools` | yes | yes | yes — graded by name, not only count |
| `introspect_verbs_test.go` | `TestBuildDomainVerbSets_ContactsFollowsFlag` | yes | yes | yes |
| `surface_test.go` | `TestContactsDomainRecordedGatedAndFull` | yes | yes | yes |
| `config_test.go` | `TestInventoryNamesContactsFlag` | yes | **absent** | **no** — `TestLoadConfig_ContactsEnabled` asserts the flag binding but never touches `config.Inventory()` (GAP-2) |
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
| `tool_annotations_test.go` | 2 loops (lines 328, 389) | partial — line 389 done; line 328 is inside `TestAggregateAnnotations_FourToolsRegistered`, which the same FR forbids changing (GAP-6) |
| `surface_export_test.go` | `TestBuildVerbsRequiresNoCredentials` | yes |

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

**GAP-1 — AC-11's repository-wide clause fails on `docs/reference/release.md`.**
AC-11 asserts "a repository-wide search for the phrase four aggregate returns no statement
that contradicts this". `docs/reference/release.md:113` still reads: "`extension/manifest.json`
contains the four aggregate domain tools (`calendar`, `mail`, `account`, `system`) with their
annotations." The manifest now holds five entries, so this is a direct contradiction of a file
a contributor consults before editing that manifest. FR-22 did not enumerate the file, which is
how it survived; AC-11 grades the property regardless of the enumeration.
*Minimal fix:* amend `docs/reference/release.md:113` to name the four default tools plus the
opt-in `contacts` entry. Also consider `docs/reference/architecture.md:5` ("As of v0.6.0 the
server exposes four aggregate domain tools"), which is a status note about a superseded
document and is weaker but now imprecise. `docs/reference/release.md:138` ("asserts all four
aggregate tools are advertised") is **correct as written** — the smoke test runs the default
configuration — and must not be changed.

**GAP-2 — `TestInventoryNamesContactsFlag` was not written.**
FR-16's inventory clause has diff evidence (`internal/config/inventory.go:44,93`) and derived
evidence (the generated manifest's `config` section names `OUTLOOK_MCP_CONTACTS_ENABLED`, held
by `TestCommittedManifestMatchesRecord`), but the specified direct assertion on
`config.Inventory()` — a row named `OUTLOOK_MCP_CONTACTS_ENABLED` with default `false` — does
not exist. `TestLoadConfig_ContactsEnabled` never calls `Inventory()`.
*Minimal fix:* add `TestInventoryNamesContactsFlag` to `internal/config/config_test.go`
asserting the row's name and its `"false"` default.

**GAP-3 — the fix instruction's log channel is untested (NFR-4, AC-14).**
The CR specified `TestContactsSearch_ErrorCarriesFixOnBothChannels` with "a captured log
handler … Both the result text and the log record carry the same verb-local fix constant". The
implemented `TestContactsSearch_GraphFailureCarriesFix` asserts the tool result only. The log
emission is present in code at `contacts_search.go:211-215` and in the three sibling handlers,
but nothing holds it: a refactor that dropped the `"fix"` attribute would leave every test
green, which is exactly the drift NFR-4 exists to prevent. This is the requirement whose stated
purpose is "so a headless caller still receives the correction", and the headless channel is
the untested one.
*Minimal fix:* install a capturing `slog.Handler` in the search failure test and assert
`contactsSearchGraphFix` appears in the record's `fix` attribute as well as in the result text.

**GAP-4 — no annotation-value assertions in `internal/tools/tool_annotations_test.go`.**
Neither `TestContactsVerbAnnotations` nor `TestContactsAggregateIsReadOnly` exists. Partial
substitutes: `verbInventoryGolden` (`internal/tools/dispatch_registry_test.go:67-71`) asserts
each contacts verb's exact `ro/de/id/ow` values, which covers the per-verb matrix; but **no
test asserts the folded `contacts` tool annotation**, which is the load-bearing safety property
AC-2 names ("a client that gates writes behind confirmation never prompts for a contacts
call"). I verified it manually via `tools/list` on the built binary — `readOnlyHint true,
destructiveHint false, idempotentHint true, openWorldHint true` — so the behaviour is correct
today and unguarded tomorrow. `AGENTS.md` also requires new verbs to "add a value assertion
alongside the existing annotation tests in `internal/tools/`", and `tool_annotations_test.go`
received only a domain-list entry.
*Minimal fix:* add `TestContactsAggregateIsReadOnly` beside the existing
`TestAggregateAnnotations_*` tests, asserting the four folded values on the registered
`contacts` tool.

**GAP-5 — two output tiers are implemented but unexercised (AC-9).**
`search` with `output=raw` and `get_person` with `output=summary` have no test. Both code paths
exist (`contacts_search.go:167-169`, `contacts_get_person.go:105`), and the sibling verbs cover
all three tiers, so this is a coverage hole rather than a behaviour defect — but AC-9 says
"each tier renders its expected shape" for all four read verbs.
*Minimal fix:* add a raw-tier case to `contacts_search_test.go` and a summary-tier case to
`contacts_get_person_test.go`, each unmarshalling the result and asserting the tier's field set.

**GAP-6 — FR-29's site list is self-contradictory; the fourteenth site was correctly not edited.**
FR-29 requires `internal/tools/tool_annotations_test.go:328` to gain `"contacts"`, while the
same requirement's closing sentence says `TestAggregateAnnotations_FourToolsRegistered`
(`:313`) MUST NOT be changed. At `3ae0e2c`, line 328 is the `[]string{"calendar","mail",
"account","system"}` loop **inside** that very test. The Tests-to-Modify table attributes line
328 to `TestAggregateAnnotations_NoOldToolNames`, which has no hard-coded domain list at all.
The implementation followed the explicit MUST NOT and left the test alone, which is the correct
resolution — the default-surface guarantee of FR-25 depends on it. This is a CR authoring
defect, not an implementation defect.
*Minimal fix:* amend FR-29's site list in the CR to drop `tool_annotations_test.go:328` and
state that only line 389 moves. Do **not** touch the test.

**GAP-7 — the CRUD harness was not run (deferred to the user).**
`make crud-test` is a paid, live-mailbox harness and was deliberately not executed. AC-12 grades
the prompt, script, and CSV by reading them, which passed; but nothing has yet exercised
Steps 47-51 against a real mailbox, so the new `mcp_contacts` accounting column and the five
new prompt steps are unproven end to end. `AGENTS.md` also warns that the harness drives the
binary named in `.mcp.json` rather than the working tree, so the rebuild step must precede any
run.
*Deferred to the user:* rebuild to the configured path, then run `make crud-test` with
`OUTLOOK_MCP_CONTACTS_ENABLED=true` and confirm the emitted CSV row has the full column count.

**GAP-8 — the site screenshot comparison was not run (deferred to the user).**
`site/src/generated/surface.json` gained a fifth domain and `site/src/surface.ts` changed, so
the published site now renders an additional domain and different derived counts. `site/AGENTS.md`
requires a change claiming to leave rendering untouched to be verified by screenshot comparison
rather than assumed; this change does not claim that, but the rendering delta is unverified.
*Deferred to the user:* run the site's screenshot comparison against `main` and confirm the
only visual delta is the added contacts domain and the moved totals.

**GAP-9 — `$search` on `/me/contacts` is proved only at the SDK layer (Open Question 6).**
The tests assert the outgoing request, exactly as the shipped `search_messages` verb is graded.
Kiota generates the `Search` field uniformly, so its presence is not evidence Graph v1.0 honours
`$search` for the personal-contacts collection, and no `ConsistencyLevel: eventual` header is
set on this path. The CR explicitly routes a live rejection to a follow-on change rather than an
in-flight redesign, so this is recorded, not raised as a defect.
*Deferred to the user:* one live `contacts.search` call against a real mailbox with contacts
enabled. If Graph rejects `$search` on `/me/contacts`, open a follow-on CR; do not patch here.

---

No FAIL. The five PARTIAL rows (FR-29, NFR-4, AC-9, AC-11, AC-14) are each traceable to one of
GAP-1 through GAP-6; GAP-7 through GAP-9 are deferred verifications, not implementation defects.
