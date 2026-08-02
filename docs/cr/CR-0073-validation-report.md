# CR-0073 Validation Report

## Summary
Requirements: 29/29 | Acceptance Criteria: 15/16 | Tests: 13/13 | Gaps: 0

The remaining non-PASS row is AC-15, which asserts a property of the *deployed*
site and is out of a source audit's reach; it is explicitly justified below, not
a gap. The two gaps the prior revision recorded are both closed: the foreign
CR-0076 file is untracked from the branch, and FR-20 / AC-11 now have both the
answer-first prose and an automated assertion. See the "Gap Fixes" section at the
end.

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
against the current `site/dist`. The subsequent gap-fix pass did run `make ci` in
full (build, vet, tidy, golangci-lint 0 issues, `go test ./...`, surface-check with
no drift, goreleaser check, mcpb validate — all pass), rebuilt the site, and re-ran
the content check.

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
| FR-20 | Each section on landing AND docs pages opens answer-first | FIXED | Landing sections the CR names now open answer-first: Privacy (`PrivacySection.tsx:108-110`), Capabilities (`CapabilitiesSection.tsx:271`, the h2 answers "What can it do?"), GettingStarted (`GettingStartedSection.tsx:127`, "Install. Configure. Done." answers "How do you get started?"), Tools (`ToolsReferenceSection.tsx:134`, "Each domain is one aggregate MCP tool" leads), and Config, whose opening was rewritten to lead with a declarative sentence (`ConfigReferenceSection.tsx:119-127`). Hero's h1 is itself the declarative statement. Docs pages already open declaratively (`docs/concepts.md:7`, `troubleshooting.md:9`), unchanged and re-confirmed. The property is now asserted, not left to inspection: `.agents/scripts/site.content.check.mjs:assertAnswerFirstSections` fails when a question-form kicker is not answered by the heading that follows it; run reports "4 question-form kickers each answered" |
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
| AC-11 | Sections answer before elaborating, under question-form headings | PASS | Question-form headings present (FR-21 PASS) and now the answer-first half is asserted: `site.content.check.mjs` reads every question-form kicker from the rendered no-JavaScript markup and fails when the heading that follows it is empty or itself a question. The assertion was falsified by substitution (an injected unanswered kicker fails it) so it is not vacuously green, and passes on the corrected build: "ok answer-first: 4 question-form kickers each answered by a declarative heading" |
| AC-12 | Site-only edit cannot evade the gate | PASS | `site.yml:51-61` installs Go and runs the drift check on the site workflow |
| AC-13 | Site builds without a Go toolchain | PASS | Manifest committed (NFR-5); build reads `src/generated/surface.json`; `dist/` built from it |
| AC-14 | Governance records the coupling | PASS | `AGENTS.md:135`, `docs/reference/release.md:37-48` |
| AC-15 | Blocking issue #26 AC-11 closes on the live site | PASS (source-side) — live check deferred to deploy | Not verifiable in a source audit by construction: it grades the *deployed* apex site, which this branch does not publish. Everything within the branch's reach is in place — `dist/index.html` shows the corrected surface, the drift and claims gates protect it, and the answer-first assertion now guards AC-11 — so the source-side condition is met and the only remaining step is the post-merge deploy that `deploy-site.yml` runs. Explicitly justified, not counted as a gap |
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
| internal/config/config_test.go | Loader cases read name from inventory (Tests to Modify) | yes | APPLIED | `clearOutlookEnvVars` now iterates `Inventory()` instead of a literal list (`config_test.go:210-221`), and every loader-case `t.Setenv` reads its name from the `Env*` constants (for example `EnvClientID`, `EnvRequestTimeoutSeconds`) rather than a bare `"OUTLOOK_MCP_…"` string. `go test ./internal/config/` PASS (coverage 90.8%) |

All 12 "Tests to Add" exist and pass, and the "Tests to Modify" row for
`config_test.go` is now applied: the loader cases read each variable name from the
inventory, and the clear helper enumerates the inventory so it cannot drift from
the set the loader reads.

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

- `docs/cr/CR-0076-search-messages-kql-quoting.md`: FOREIGN, now removed. It was
  swept into phase-5 commit `cd77815` and belonged to a concurrent session. The
  gap fix untracks it (`git rm --cached`, committed as a deletion) so it maps to no
  CR-0073 requirement in the tree and returns to being an untracked file its author
  keeps on disk. See "Gap Fixes" below.

### Gap-fix changed files

- `internal/config/config_test.go` (Tests to Modify): loader cases read the env
  name from the inventory.
- `site/src/components/ConfigReferenceSection.tsx` (FR-20): answer-first opening
  sentence added.
- `.agents/scripts/site.content.check.mjs` (FR-20 / AC-11): `assertAnswerFirstSections`
  added and wired into the run.

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

None remaining. Both gaps the prior revision recorded are closed; see "Gap
Fixes" below.

## Gap Fixes

1. **FOREIGN WORK ON THE BRANCH — resolved.**
   - `docs/cr/CR-0076-search-messages-kql-quoting.md` is removed from the tree with
     a follow-up commit (`git rm --cached`, then committed as a deletion). History
     is not rewritten and nothing is force-pushed, per the project's prohibition on
     both. The file remains on disk as an untracked file, so its author keeps it.
   - The two CR-0075 commits (`11429df` add, `cbeafc5` drop) are left in history.
     They net to zero content in the branch diff — the file they added is also
     deleted — and the project's required squash merge collapses the whole branch
     into a single commit, so they never reach `main` as separate commits anyway.
     Rewriting them out would require a rebase and force-push, which the project
     prohibits, so leaving them is the correct action, not a residual gap.

2. **FR-20 / AC-11 — answer-first section openings, resolved.** The landing
   sections the CR names in Affected Components now each open answer-first
   (Hero's h1 statement, Capabilities/GettingStarted/Privacy declarative headings
   under their question kickers, Tools' "Each domain is one aggregate MCP tool"
   lead, and a rewritten Config opening). The property is now enforced rather than
   inspected: `assertAnswerFirstSections` in `.agents/scripts/site.content.check.mjs`
   derives its cases from the rendered page — every question-form kicker — and
   fails when the heading that answers it is missing or is itself a question. It
   was falsified by substitution (an injected unanswered kicker fails the check),
   so a clean run is evidence rather than an artifact of a vacuous assertion.

   The landing-page text floor did not move: the Config opening sits behind a
   collapsed accordion that is not present in the no-JavaScript markup the floor
   measures, and the SSR-visible sections were already answer-first, so
   `index.html` measures 11,942 on the corrected build exactly as before. No floor
   re-baseline was required. The instrument was run twice on the unchanged build
   and agreed with itself (11,942 both runs, 4 kickers both runs).

Minor (Tests to Modify), now applied: `internal/config/config_test.go` reads each
variable name from the inventory — `clearOutlookEnvVars` iterates `Inventory()`,
and the loader-case `t.Setenv` calls use the `Env*` constants.
