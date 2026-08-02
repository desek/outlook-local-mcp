# CR-0073 Validation Report

## Summary
Requirements: 28/29 | Acceptance Criteria: 14/16 | Tests: 12/12 | Gaps: 2

Scope: branch `feat/cr-0073-surface-manifest`, merge-base `origin/main` at
`8371f74`, HEAD `afd44f2`. Grading traces every requirement to a CHANGED hunk in
the branch diff and, where behaviour is observable, to an executed test or the
CR's own Test Strategy harness. Full Go suite `go test ./...` passes; the site
content-check harness `node .agents/scripts/site.content.check.mjs site/dist`
exits 0 with every assertion holding; the manifest generator is byte-identical
across two runs.

Method note: `make ci` was not run in full (lint, goreleaser, mcpb are outside
the audit's cost budget and the finalizer already ran it). Instead the
load-bearing gates were exercised directly: `go build ./...` (exit 0), the named
Go tests, the twice-run determinism gate, and the JS content-check harness
against the current `site/dist`.

## Requirement Verification

| Req # | Description | Status | Evidence (file:line / test name) |
|---|---|---|---|
| FR-1 | `internal/surface` builds record from live builders + config inventory, no network/credential | PASS | `internal/surface/build.go:105-163` (`BuildRecord`), `server.BuildVerbsForInspection`; `TestBuildVerbsRequiresNoCredentials` PASS |
| FR-2 | Per-domain name, ordered verbs, per-verb name/summary/readOnly/gate-or-null | PASS | `internal/surface/record.go:12-43`; `build.go:134-149`; `TestEveryVerbCarriesSummaryAndGate` PASS |
| FR-3 | Full and default counts, per-domain and total, derived by counting | PASS | `build.go:151-160` (`len(fullVerbs)`, `len(def[d])`); `TestRecordCountsMatchBuiltVerbs`, `TestDefaultCountExcludesGatedVerbs` PASS; manifest totals 42/33 |
| FR-4 | Every config var: name, default, description | PASS | `internal/config/inventory.go:66-93`; `build.go:167-178`; manifest carries 26 vars |
| FR-5 | Byte-identical serialization, stable order, no timestamp/commit/env | PASS | `internal/surface/serialize.go:27-36`; `TestSerializationIsDeterministic`, `TestSerializationCarriesNoEnvironmentValue` PASS; twice-run gate no diff |
| FR-6 | `cmd/gen-surface` writes `site/src/generated/surface.json` | PASS | `cmd/gen-surface/main.go:27-43`; file present (420 lines) |
| FR-7 | `make surface-manifest` runs the generator | PASS | `Makefile:37-40` |
| FR-8 | `internal/config` declarative inventory (name/default/description) | PASS | `internal/config/inventory.go:66-104`; `TestInventoryEntriesAreComplete` PASS |
| FR-9 | Loader reads env names from the inventory constants | PASS | `internal/config/config.go:222-327` (all literals → `Env*` consts); `validate.go:163`; `TestEveryEnvLiteralIsEnumerated` PASS |
| FR-10 | ToolsReferenceSection renders from manifest, no verb literal | PASS | `site/src/components/ToolsReferenceSection.tsx:4,33-39` (imports `domains`, maps them); flat literal arrays removed |
| FR-11 | Verbs presented as `operation` values, not flat tool names | PASS | `ToolsReferenceSection.tsx:127-139,209` ("Operation" column, `{category.label} · {verb.name}`); harness "no flat tool name" PASS |
| FR-12 | ConfigReferenceSection renders from manifest, no literal name/default/count | PASS | `ConfigReferenceSection.tsx:4,27-33,104` (`configVars`, `configVarCount`) |
| FR-13 | Capabilities and GettingStarted compose counts from manifest | PASS | `CapabilitiesSection.tsx:10,271,396`; `GettingStartedSection.tsx:4,311` (`domainCount`) |
| FR-14 | seo.pages.ts meta description composed, no transcribed figure | PASS | `site/build/seo.pages.ts:16,56` (`${domainCount}`/`${fullVerbCount}`); `site/index.html:9` figure removed |
| FR-15 | JSON-LD feature count/list composed from manifest | PASS | `site/build/seo.jsonld.ts:22,77` (`featureList` from `domainCount`/`domainNames`/counts) |
| FR-16 | Site states which figure is full vs default wherever a count shows | PASS | `ToolsReferenceSection.tsx:131-140`; `CapabilitiesSection.tsx:271`; `HeroSection.tsx:15` |
| FR-17 | No "Diagnostics" domain; names calendar/mail/account/system | PASS | `ToolsReferenceSection.tsx` (hardcoded `diagnostics` category removed); harness "no Diagnostics" + "4 domains named" PASS |
| FR-18 | No "opens no listening port" claim (loopback port acknowledged) | PASS | `site/src/components/PrivacySection.tsx:19` (restated, names loopback sign-in port) |
| FR-19 | Outbound restated: Microsoft endpoints + optional telemetry, no 3rd-party relay | PASS | `PrivacySection.tsx:19,108-110` |
| FR-20 | Each section on landing AND docs pages opens answer-first | PARTIAL | Landing: only Privacy opening rewritten (`PrivacySection.tsx:108-110`), 3 kickers made question-form. Docs pages already open declaratively (`docs/concepts.md:7`, `troubleshooting.md:9`) but were NOT touched this CR; no diff demonstrates coverage of every landing section (Intro/Hero/Tools/Config openings unchanged) |
| FR-21 | Landing headings question-form where they answer a user question | PASS | `CapabilitiesSection.tsx:267,300,432` ("What can it do?"), `GettingStartedSection.tsx:121` ("How do you get started?"), `PrivacySection.tsx:99` ("Is your data private?"). Tools/Config kept statement-form (defensible under the "where it answers a question" qualifier) |
| FR-22 | CI fails when regenerating changes the tree | PASS | `Makefile:47-49` (`surface-check`); `.github/workflows/site.yml:58-61` |
| FR-23 | Drift check on both Go and site workflows; site workflow installs Go | PASS (accepted deviation) | Go side: `ci.yml` runs `make ci` which now includes `surface-check` (`Makefile:30`), so `ci.yml` needed no edit though Affected Components named it. Site side: `site.yml:51-61` installs Go and runs the check |
| FR-24 | Content-check fails on bare numeric tool/verb/domain/variable claim, names file:line | PASS | `.agents/scripts/site.content.check.mjs:216-233` (`assertNoBareClaims`); harness "no bare tool-surface figure" PASS |
| FR-25 | `make ci` includes the drift check | PASS | `Makefile:30` (`ci: docs-bundle surface-check ...`) |
| FR-26 | Text floors re-measured on corrected build, script records the change | PASS | `site.content.check.mjs:67-97` (index floor 11942, re-baseline narrative 11853→11714→11942 recorded); harness floor assertions PASS |
| FR-27 | AGENTS.md harness rules list the surface manifest | PASS | `AGENTS.md:135,140` |
| FR-28 | site/AGENTS.md states the site holds no figure; enforced by harness | PASS | `site/AGENTS.md:44-66` |
| FR-29 | release.md states the site is a release surface, in the crud-test pattern | PASS | `docs/reference/release.md:19,37-48` |

Non-functional: NFR-1 (determinism) PASS (twice-run gate). NFR-2 (no
credentials/network) PASS (`TestBuildVerbsRequiresNoCredentials`). NFR-5
(manifest committed) PASS (`site/src/generated/surface.json` in tree). NFR-3
(payload <20 KiB compressed) and NFR-4 (Lighthouse budget) NOT MEASURED here
(lighthouse not run); manifest JSON is ~9 KB uncompressed so NFR-3 is very likely
met, but the budget instrument was not exercised in this audit.

## Acceptance Criteria Verification

| AC # | Description | Status | Evidence |
|---|---|---|---|
| AC-1 | `make surface-manifest` writes file, tree unchanged | PASS | Determinism gate: generator run, `git diff --exit-code` clean |
| AC-2 | Generator twice → byte-identical | PASS | Two successive runs, both no diff |
| AC-3 | Added verb fails drift until site follows; message names target | PASS | `Makefile:47-49` diff gate + error text naming `make surface-manifest`; `site.yml:59-61` |
| AC-4 | Site states four tools; count = manifest; JSON-LD count = manifest; no "23 MCP tools" | PASS | `dist/index.html` grep: no "23 MCP tools", says "aggregate MCP tools"; `seo.jsonld.ts:77`; harness PASS |
| AC-5 | No flat tool name (`calendar_list_events`) in served HTML | PASS | harness `assertToolSurfaceShape` "no flat tool name" PASS; SVG literals converted to `domain · verb` form |
| AC-6 | Categories exactly the four domains, no Diagnostics | PASS | harness "no Diagnostics" + "4 domains named (calendar, mail, account, system)" |
| AC-7 | Config reference complete, count = enumerated vars | PASS | `ConfigReferenceSection.tsx:104` `configVarCount`; manifest 26 vars; `TestEveryEnvLiteralIsEnumerated` |
| AC-8 | A count is never ambiguous (full vs default stated) | PASS | `ToolsReferenceSection.tsx:131-140`, `CapabilitiesSection.tsx:271`, `HeroSection.tsx:15` |
| AC-9 | Reintroduced literal rejected, names file+line | PASS | `site.content.check.mjs:227-229` (`fail(... at ${file}:${i+1})`); assertion runs and passes on clean tree |
| AC-10 | Security claims survive checking (no "no port"; Microsoft + telemetry) | PASS | `PrivacySection.tsx:19,108-110` |
| AC-11 | Sections answer before elaborating, under question-form headings | PARTIAL | Question-form headings present for 3 landing sections (FR-21 PASS). Answer-first-opening half unproven for every section: only Privacy's opening was rewritten; no test asserts AC-11 and docs-page openings were not reviewed in this change |
| AC-12 | Site-only edit cannot evade the gate | PASS | `site.yml:51-61` installs Go and runs the drift check on the site workflow |
| AC-13 | Site builds without a Go toolchain | PASS | Manifest committed (NFR-5); build reads `src/generated/surface.json`; `dist/` built from it |
| AC-14 | Governance records the coupling | PASS | `AGENTS.md:135`, `docs/reference/release.md:37-48` |
| AC-15 | Blocking issue #26 AC-11 closes on the live site | PARTIAL | Not verifiable in a source audit; requires the deployed site. Source-side correction is in place (`dist/index.html` shows correct surface) |
| AC-16 | Text floor re-baselined, not weakened; reduction accounted for | PASS | `site.content.check.mjs:74-97` records 11853→11714 (−139, obsolete inventory) then →11942 (phase 4 prose); harness floor PASS at exactly 11942 |

## Test Strategy Verification

| Test File | Test Name | Specified | Exists | Matches Spec |
|---|---|---|---|---|
| internal/surface/surface_test.go | TestRecordCountsMatchBuiltVerbs | yes | yes | PASS |
| internal/surface/surface_test.go | TestDefaultCountExcludesGatedVerbs | yes | yes | PASS |
| internal/surface/surface_test.go | TestEveryVerbCarriesSummaryAndGate | yes | yes | PASS |
| internal/surface/serialize_test.go | TestSerializationIsDeterministic | yes | yes | PASS |
| internal/surface/serialize_test.go | TestSerializationCarriesNoEnvironmentValue | yes | yes | PASS |
| internal/surface/manifest_test.go | TestCommittedManifestMatchesRecord | yes | yes | PASS |
| internal/config/inventory_test.go | TestEveryEnvLiteralIsEnumerated | yes | yes | PASS |
| internal/config/inventory_test.go | TestInventoryEntriesAreComplete | yes | yes | PASS |
| internal/server/surface_export_test.go | TestBuildVerbsRequiresNoCredentials | yes | yes | PASS |
| site.content.check.mjs | claims assertion | yes | yes | PASS (executed, exit 0) |
| site.content.check.mjs | text floor assertion | yes | yes | PASS (index 11942 == floor) |
| site.content.check.mjs | tool-surface shape assertion | yes | yes | PASS (no flat name, no Diagnostics, 4 domains) |
| internal/config/config_test.go | Loader cases read name from inventory (Tests to Modify) | yes | NOT APPLIED | `config_test.go` is absent from the branch diff; loader tests still assert literal env names. Existing tests pass, so no regression, but the specified modification was not made |

All 12 "Tests to Add" exist and pass. The single "Tests to Modify" row for
`config_test.go` was not applied (minor: the tests pass unchanged and no AC
depends on it).

## Diff Coverage

| File | +/- | Mapped Requirements |
|---|---|---|
| internal/surface/record.go | +79 | FR-2, FR-4 |
| internal/surface/build.go | +178 | FR-1, FR-2, FR-3 |
| internal/surface/serialize.go | +36 | FR-5 |
| internal/surface/doc.go | +13 | FR-1 (package doc) |
| internal/surface/surface_test.go | +112 | FR-2, FR-3 tests |
| internal/surface/serialize_test.go | +52 | FR-5 tests |
| internal/surface/manifest_test.go | +35 | FR-6 test |
| cmd/gen-surface/main.go | +43 | FR-6 |
| internal/config/inventory.go | +104 | FR-8 |
| internal/config/inventory_test.go | +84 | FR-8, FR-9 tests |
| internal/config/config.go | +28/-28 | FR-9 |
| internal/config/validate.go | +1/-1 | FR-9 |
| internal/server/surface_export.go | +44 | FR-1, NFR-2 |
| internal/server/surface_export_test.go | +33 | NFR-2 test |
| Makefile | +23/-2 | FR-7, FR-22, FR-25 |
| .github/workflows/site.yml | +18 | FR-22, FR-23, AC-12 |
| .agents/scripts/site.content.check.mjs | +141 | FR-24, FR-26, AC-5, AC-6, AC-9, AC-16 |
| site/src/surface.ts | +104 | FR-10..16 (typed manifest accessor) |
| site/src/generated/surface.json | +420 | FR-6 (generated artifact) |
| site/tsconfig.app.json | +1 | enables JSON import for surface.ts (supports FR-10..16) |
| site/src/components/ToolsReferenceSection.tsx | +/-137 | FR-10, FR-11, FR-16, FR-17 |
| site/src/components/ConfigReferenceSection.tsx | +/-73 | FR-12 |
| site/src/components/CapabilitiesSection.tsx | +/-80 | FR-13, FR-16, FR-17, FR-21 |
| site/src/components/GettingStartedSection.tsx | +/-5 | FR-13, FR-21 |
| site/src/components/HeroSection.tsx | +/-5 | FR-13, FR-16 |
| site/src/components/PrivacySection.tsx | +/-9 | FR-18, FR-19, FR-20, FR-21 |
| site/src/components/svg/CapabilityCalendar.tsx | +/-12 | AC-5 (removes flat tool names from illustration) |
| site/src/components/svg/CapabilityMailSearch.tsx | +/-4 | AC-5 |
| site/src/components/svg/CapabilityMultiAccount.tsx | +/-2 | AC-5 |
| site/build/seo.pages.ts | +/-9 | FR-14 |
| site/build/seo.jsonld.ts | +/-12 | FR-15 |
| site/index.html | +/-2 | FR-14 (static fallback figure removed) |
| AGENTS.md | +3 | FR-27 |
| site/AGENTS.md | +23 | FR-28 |
| docs/reference/release.md | +14 | FR-29 |
| docs/cr/CR-0073-...manifest.md | +903 | the CR itself |

### Unmapped changed files

- `docs/cr/CR-0076-search-messages-kql-quoting.md` (+532): FOREIGN. Swept into
  CR-0073 phase 5 commit `cd77815`; concerns a different, unrelated change. It
  maps to no CR-0073 requirement and must be removed from this branch before
  merge.

Deviations verified and accepted (not defects):
- `.github/workflows/ci.yml` unchanged despite being named in Affected
  Components: acceptable because `ci.yml` runs `make ci`, which now includes
  `surface-check` (`Makefile:30`), so the Go-workflow half of FR-23 is satisfied
  without editing the workflow file.
- Phase-3 additions not enumerated in Affected Components (`site/src/surface.ts`,
  the `resolveJsonModule` line in `site/tsconfig.app.json`, and the three
  `Capability*` SVGs): all necessary. surface.ts is the typed accessor the
  components import; resolveJsonModule lets TypeScript import the JSON; the SVGs
  held flat tool names (`calendar_create_event`) that AC-5 requires removed from
  served HTML.

## Gaps

1. **FOREIGN WORK ON THE BRANCH (blocks merge).** Two commits carry work
   unrelated to CR-0073, and one leaves a stray file in the diff:
   - `11429df checkpoint(CR-0075): ...` added
     `docs/cr/CR-0075-draft-editable-region-revision.md`, and `cbeafc5 docs(cr):
     drop CR-0075 ...` deleted it. The two net to zero in the branch diff but
     pollute the history of a squash/rebase and are not CR-0073.
   - `docs/cr/CR-0076-search-messages-kql-quoting.md` was added inside the
     CR-0073 phase-5 commit `cd77815` and REMAINS in the branch diff (+532
     lines). It is foreign to CR-0073.
   - Suggested minimal fix: rebase the branch to drop `11429df` and `cbeafc5`
     entirely, and remove `docs/cr/CR-0076-...md` from `cd77815` (or delete it in
     a follow-up commit) so the branch carries only CR-0073 work.

2. **FR-20 / AC-11 — answer-first section openings only partially demonstrated.**
   The requirement covers "each content section on the landing AND documentation
   pages". Phase 4 rewrote only the Privacy section's opening sentence and
   converted three kickers to question form; the documentation pages already open
   declaratively (pre-existing, e.g. `docs/concepts.md:7`) but were not reviewed
   in this change, and the openings of the other landing sections (Intro, Hero,
   Tools, Config) were not modified. There is no automated assertion for AC-11,
   so the property is unverified for the sections the diff did not touch.
   - Suggested minimal fix: either confirm and record (in the CR or the harness)
     that every landing section already opens answer-first, or rewrite the
     openings that do not, so the "each section" claim has evidence.

Minor (not counted as a gap): the "Tests to Modify" entry for
`internal/config/config_test.go` was not applied; the loader tests still name
literal env strings. No AC depends on it and the tests pass, so it is a
documentation-vs-implementation mismatch rather than a functional defect.
