# CR-0074 Validation Report

Validated: 2026-08-01. Branch `docs/cr-0073` (historical name; CR renumbered to
CR-0074 mid-flight). Finalization commit `cda9337`. Merge-base with `origin/main`:
`dcfbadd`. Documentation-only audit; no source was modified.

## Summary
Requirements: 20/20 | Acceptance Criteria: 13/14 | Tests: 4/5 | Gaps: 2

Legend: PASS, PARTIAL, GAP, FAIL. Requirement count folds the 15 Functional and
5 Non-Functional requirements. One AC is PARTIAL (AC-9). One Test-Strategy row
is GAP (a named test not implemented; coverage subsumed elsewhere). One advisory
GAP is recorded for residual prompt drift outside the CR's enumerated scope.

## Requirement Verification

### Functional Requirements

| Req # | Description | Status | Evidence (file:line / test name) |
|-------|-------------|--------|----------------------------------|
| FR-1 | Explicit `{#id}` tag used verbatim, heading text ignored | PASS | `internal/tools/get_docs.go:126-128` (regexp submatch returns `m[1]`); `TestHeadingToAnchor` explicit cases PASS |
| FR-2 | Derive from heading text when no tag | PASS | `internal/tools/get_docs.go:130-140`; `TestHeadingToAnchor` text-derived cases PASS |
| FR-3 | Explicit-anchor heading MUST NOT also resolve under derived form | PASS | Exclusive branch `get_docs.go:126-129`; `TestHeadingToAnchor` negative assertion (`get_docs_test.go:326-331`); live negative case in `.agents/logs/CR-0074-phase6-verification.md` (derived `container-has-no-keychain-access` → "section not found") |
| FR-4 | `extractSection` heading text strips the `{#...}` tag | PASS | `get_docs.go:110-124` (`headingAnchorTagRe.ReplaceAllString` on heading line); phase6 heading-tag check: zero returned heading lines contain `{#` or `}` |
| FR-5 | All five sections resolve via `get_docs` | PASS | `TestGetDocs_EveryHeadingReachable` PASS; phase6 per-section live table (all `isError=false`, non-empty) |
| FR-6 | Corpus test walks every H2 in every embedded doc | PASS | `TestGetDocs_EveryHeadingReachable` (`get_docs_test.go:58-107`) PASS |
| FR-7 | Cross-link test covers intra- and inter-document forms | PASS | `TestGetDocs_CrossLinksResolve` (`get_docs_test.go:128-206`) PASS; logs "checked 6 embedded section cross-links"; resolves through production `HandleGetDocs` |
| FR-8 | `TestHeadingToAnchor` includes explicit-differs-from-derived case | PASS | `get_docs_test.go:311` (`Container has no keychain access {#container-no-keychain}` → `container-no-keychain`) |
| FR-9 | `site.yml` runs a `.agents/scripts` harness | PASS | `.github/workflows/site.yml:100-104` "Exercise the site content-check harness" step runs `node .agents/scripts/site.content.check.mjs` |
| FR-10 | `release.md` states crud-test manual gate and why not in CI | PASS | `docs/reference/release.md` "Required manual gate: `make crud-test`" section (+16 lines) |
| FR-11 | `security.md` records unexercised-path rule + three incidents | PASS | `docs/reference/security.md` "The unexercised-path rule" with container job, PR #34 harnesses, `TestHeadingToAnchor` worked examples |
| FR-12 | `security.md` records grype symbol-stripping caveat + ceiling | PASS | `docs/reference/security.md` "The `grype` symbol-stripping caveat" section, incl. what restores reachability-precision |
| FR-13 | Prompt uses registry param names | PASS | `docs/prompts/mcp-tool-crud-test.md`: `message_id` (Steps 32,34,36), `conversation_id` (Step 35), `folder_id` (Steps 30b-30e,33,35,36). Verified against `get_message.go:67`, `get_conversation.go:51`, `list_messages.go:76` |
| FR-14 | Remove "match by event ID in text" at Steps 5,6 | PASS | Prompt diff Steps 5/6 now "match by subject; ... no event ID" |
| FR-15 | Record `max_results`-capped mail list as expected | PASS | Prompt Step 30 "Expected scale artifact (not a finding)" note (27,214 unread vs default cap 25) |

### Non-Functional Requirements

| Req # | Description | Status | Evidence |
|-------|-------------|--------|----------|
| NFR-1 | No verb added/removed/renamed/re-signatured | PASS | `TestVerbInventoryUnchangedAfterUpgrade` PASS; annotation golden tests PASS; `extension/manifest.json` absent from branch diff |
| NFR-2 | Embedded bundle remains exactly four files | PASS | `internal/docs/embed.go` and `docs/{readme,quickstart,concepts,troubleshooting}.md` absent from branch diff; `internal/docs` tests PASS |
| NFR-3 | `make ci` passes including `-race` | PASS | `.agents/logs/CR-0074-phase6-verification.md` records `make ci`=0; independently confirmed `make build`=0, `go vet ./internal/tools/...`=0, `go test -race ./internal/tools/`=ok |
| NFR-4 | site.yml harness adds no live creds/network beyond existing | PASS | `site.yml:100-104` reuses `site/dist` build + `puppeteer-core` devDependency; only adds `CHROME_PATH` env pointing at runner's pre-installed Chrome |
| NFR-5 | Corpus test fails for a new unreachable heading, unedited | PASS | Test derives cases from `docs.MustCatalog()`/`ReadSlug` (`get_docs_test.go:71-73`), not a hardcoded list; mechanism proven by the AC-5 red run |

## Acceptance Criteria Verification

| AC # | Description | Status | Evidence |
|------|-------------|--------|----------|
| AC-1 | Explicit anchors resolve, no cross-section bleed | PASS | phase6 live case 3 (`container-no-keychain`, 1377 bytes, own heading only); "No cross-section bleed" section |
| AC-2 | All five affected sections resolve | PASS | phase6 per-section table (5/5 `isError=false`); `TestGetDocs_EveryHeadingReachable` |
| AC-3 | Derived form does not also resolve | PASS | phase6 FR-3 negative case ("section not found"); `TestHeadingToAnchor` negative assertion |
| AC-4 | Anchor tag never reaches caller | PASS | phase6 heading-tag check: zero heading lines contain `{#`/`}` (braces appear in body content only) |
| AC-5 | Corpus test fails before the fix, naming each section | PASS | `.agents/logs/CR-0074-phase1-corpus-reachability-failure.log` — `--- FAIL` naming all five sections with document and anchor |
| AC-6 | Future unreachable heading fails build, unedited | PASS | Corpus-derived cases (NFR-5); mechanism demonstrated red in AC-5 |
| AC-7 | Broken cross-links fail build | PASS | `TestGetDocs_CrossLinksResolve` resolves through production path, both link forms, with a vacuous-pass guard (`checked==0` → `t.Fatal`, `get_docs_test.go:199-202`) |
| AC-8 | CI exercises an agent harness | PASS | `site.yml` step invokes the harness against `site/dist`; runtime confirmation occurs on the PR run (not observable within this repo diff) |
| AC-9 | Harness job actually detects a broken import | PARTIAL | Substantiated only by the phase-4 checkpoint claim (`acb35a3`: import pointed at a bad path → exit 1 `ERR_MODULE_NOT_FOUND`, reverted); no committed red-run log or CI PR run artifact. CHROME_PATH macOS default confirmed intact (`site.content.check.mjs:38`) |
| AC-10 | crud-test is a visible manual gate | PASS | `docs/reference/release.md` "Required manual gate" + "Why it cannot run in CI" + "Honest limitation" |
| AC-11 | Unexercised-path rule recorded with three incidents | PASS | `docs/reference/security.md` three worked examples |
| AC-12 | grype caveat published with its ceiling | PASS | `docs/reference/security.md` grype caveat + "What would restore reachability-precision" |
| AC-13 | Harness prompt matches the registry | PASS | Prompt param corrections verified against live registry (see FR-13); event-ID matching removed; scale artifact recorded |
| AC-14 | Tool surface unchanged | PASS | `TestVerbInventoryUnchangedAfterUpgrade` PASS; `extension/manifest.json` unchanged |

## Test Strategy Verification

| Test File | Test Name | Specified | Exists | Matches Spec |
|-----------|-----------|-----------|--------|--------------|
| `internal/tools/get_docs_test.go` | `TestGetDocs_EveryHeadingReachable` | yes | yes | PASS — walks every H2 via corpus, asserts non-empty; PASS on fixed tree, red pre-fix |
| `internal/tools/get_docs_test.go` | `TestGetDocs_CrossLinksResolve` | yes | yes | PASS — both intra/inter forms, vacuous-pass guard, resolves through production path |
| `internal/tools/get_docs_test.go` | `TestGetDocs_ExplicitAnchorSections` | yes | no | GAP — not implemented under this name; the five-section assertion is subsumed by `TestGetDocs_EveryHeadingReachable` and the phase6 live verification |
| `internal/tools/get_docs_test.go` | `TestHeadingToAnchor` (modify) | yes | yes | PASS — explicit-anchor cases added incl. differs-from-derived; negative assertion added |
| `internal/tools/verb_metadata_test.go` | `buildHeadingIndex` (modify) | yes | yes | PASS — now registers only the single production-reachable anchor; duplicate `headingToAnchor` reconciled to production regexp; `TestSeeDocsAnchorsResolve` PASS |

## Diff Coverage

| File | +/- | Mapped Requirements |
|------|-----|---------------------|
| `internal/tools/get_docs.go` | +40/-9 | FR-1, FR-2, FR-3, FR-4 |
| `internal/tools/get_docs_test.go` | +214 | FR-5, FR-6, FR-7, FR-8; Test Strategy |
| `internal/tools/verb_metadata_test.go` | +37/-... | FR-3 (buildHeadingIndex reconciliation) |
| `.github/workflows/site.yml` | +26 | FR-9, NFR-4, AC-8, AC-9 |
| `docs/reference/release.md` | +16 | FR-10, AC-10 |
| `docs/reference/security.md` | +84 | FR-11, FR-12, AC-11, AC-12 |
| `docs/prompts/mcp-tool-crud-test.md` | +14/-13 | FR-13, FR-14, FR-15, AC-13 |
| `docs/cr/CR-0074-...paths.md` | +950 | The CR itself (authoring/review) |

### Unmapped changed files
Not listed in the CR's Affected Components, each justified:

- `.agents/scripts/site.content.check.mjs` (+6/-1): the `CHROME_PATH` env override that lets the CI harness step (FR-9) run the same script on ubuntu-latest while preserving the macOS default for local use. Necessary for FR-9/AC-8/AC-9 to function; a legitimate omission from Affected Components rather than a stray change.
- `docs/bench/crud-runs.csv` (+1): the `2026-08-01T11-55-17` run row for the 0.5.1 crud-test that surfaced this CR (cited in Related Items and referenced by `release.md` as the evidence record). Consistent with the AGENTS.md harness-maintenance rule.
- `.agents/logs/CR-0074-phase1-corpus-reachability-failure.log`, `.agents/logs/CR-0074-phase6-verification.md`: phase evidence artifacts, expected.

The untracked `docs/cr/CR-0073-site-content-correction-...md` is an unrelated draft, correctly excluded from all commits. Not flagged.

## Gaps

1. **GAP — `TestGetDocs_ExplicitAnchorSections` not implemented under its Test-Strategy name.** The row in Test Strategy specifies a dedicated test asserting the five documented anchors resolve with no cross-section bleed. It does not exist as a named function. Coverage is subsumed by `TestGetDocs_EveryHeadingReachable` (which exercises all five as part of the corpus walk) and by the phase6 live per-section verification (which additionally asserts no cross-section bleed). Suggested minimal fix: either add the named table-driven test for the five anchors (low cost, matches the spec literally) or amend the CR's Test Strategy to record that `TestGetDocs_EveryHeadingReachable` supersedes it. Functionally covered; the gap is nominal.

2. **GAP (advisory, outside CR's enumerated scope) — residual prompt drift left in `docs/prompts/mcp-tool-crud-test.md`.** Two further parameter mismatches remain, both verified against the live registry (`list_messages.go`): Step 30d passes `provenance: "created_by_mcp"` but the registry declares `provenance` as a boolean (`list_messages.go:108`, `WithBoolean`), so the correct value is `provenance: true`; Steps 35 and 36 pass `top: 1` but the registry parameter is `max_results` (`list_messages.go:111`), so the correct value is `max_results: 1`. Phase 5 flagged both and did not fix them. No CR requirement (FR-13/14/15) covers these, so leaving them is defensible scoping, but they are genuine unfixed defects in a document the CR edited. Suggested minimal fix: correct the two values, or record them explicitly as a follow-up in the CR. (Phase 5 also corrected Step 36's `get_message` `id`→`message_id` beyond FR-13's enumerated Steps 32/34 — a consistent, in-scope-spirit improvement, not a gap.)

3. **PARTIAL — AC-9 fail-detection proof is uncaptured.** The requirement that the new `site.yml` harness job be proven to fail is substantiated only by the phase-4 checkpoint commit message (`acb35a3`), which reports a local reproduction (`ERR_MODULE_NOT_FOUND`, exit 1, reverted). There is no committed red-run log (as exists for Phase 1 and Phase 6) nor a CI PR run recorded. Given this CR's own thesis — "a gate not proven to fail is not proven to be a gate" — the strongest evidence would be a captured artifact or the PR CI run. The CHROME_PATH macOS-default fallback is independently confirmed intact, so local use is not broken. Suggested minimal fix: attach the red-run output as a `.agents/logs/CR-0074-phase4-*.log` artifact, or reference the PR CI run once available.

No FAIL rows.

## Scrutiny items addressed
- **FR-3 in both parsers:** production `get_docs.go` and test-side `verb_metadata_test.go` `headingToAnchor`/`buildHeadingIndex` both now register only the single production-reachable anchor; `TestSeeDocsAnchorsResolve` PASS. Confirmed.
- **AC-5 genuinely red before fix:** phase1 log shows `--- FAIL` naming all five sections. Confirmed.
- **AC-9 CHROME_PATH default:** macOS bundle preserved as the `??` fallback. Confirmed; the red-run itself is only a commit-message claim (PARTIAL above).
- **AC-6/AC-7 durability:** both tests derive cases from the corpus, not hardcoded lists; cross-link test covers inter-document links (the primary Change Driver) and has a vacuous-pass guard. Confirmed.
- **`make crud-test` not falsely claimed:** the CR checklist line for crud-test is unchecked with an explicit "deliberately not run" note; phase6 log explicitly states it was not run. No false "verified in CI" claim recurred. Confirmed.
- **NFR-2:** `docs/embed.go` and the four embedded files untouched. Confirmed.
