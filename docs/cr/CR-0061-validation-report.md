---
cr: CR-0061
validator: N+2 Validation Agent
branch: dev/cr-0061
finalized-commit: ee67ca1
validation-date: 2026-04-25
---

# CR-0061 Validation Report — In-Server Documentation Access for LLM Self-Troubleshooting

## Summary

| Category | Pass | Fail | Partial | Deferred | Total |
|----------|------|------|---------|----------|-------|
| Functional Requirements | 15 | 0 | 0 | 0 | 15 |
| Acceptance Criteria | 9 | 0 | 0 | 0 | 9 |
| Tests (unit) | 15 | 0 | 0 | 0 | 15 |
| CRUD Test Steps | 0 | 0 | 0 | 5 | 5 |
| Gaps | None | | | | |

**Overall: PASS. All functional requirements and acceptance criteria are satisfied. `make ci` is clean.**

`make ci` ran clean: build, vet, fmt-check, tidy, lint, all unit tests, goreleaser check, mcpb validate — all pass.

---

## Requirement Verification

| Req | Description | Status | Evidence |
|-----|-------------|--------|----------|
| FR-1 | Embed curated bundle via `embed.FS` | PASS | `internal/docs/embed.go:11-14` — `//go:embed files/readme.md files/quickstart.md files/troubleshooting.md` explicit allowlist |
| FR-2 | Register each doc as MCP resource with `doc://outlook-local-mcp/{slug}`, MIME `text/markdown` | PASS | `internal/server/server.go:185-220` registers resources in loop over catalog |
| FR-3 | Support `resources/list` and `resources/read` for all bundled docs | PASS | `internal/server/server_test.go:638 TestResourcesList_IncludesBundledDocs` passes |
| FR-4 | `system.list_docs` verb returns catalog (slug, title, summary, tags, size) | PASS | `internal/tools/list_docs.go`; `TestSystemListDocs_Text` passes |
| FR-5 | `system.search_docs` case-insensitive search with ranked snippets ±2 lines and 1-based line numbers | PASS | `internal/docs/search.go`; `TestSearch_RanksExactMatchesFirst`, `TestSearch_ReturnsSnippetWithLineNumbers` pass |
| FR-6 | `system.get_docs` accepts `slug`, optional `section`, optional `output` (`text`/`raw`) | PASS | `internal/tools/get_docs.go`; `TestSystemGetDocs_Section`, `TestSystemGetDocs_Raw` pass |
| FR-7 | New verbs conform to CR-0060/CLAUDE.md: system domain registry, text-tier default, help output, no new manifest entry | PASS | Verbs registered via `internal/server/system_verbs.go` in `VerbRegistry`; help enumerated in `TestSystemHelp_ListsDocsVerbs`; `extension/manifest.json` unchanged; note: CR listed `dispatch_registry.go` as location but `system_verbs.go` is the correct implementation file per the server architecture |
| FR-8 | `docs/troubleshooting.md` authored covering all required topics | PASS | `docs/troubleshooting.md` 273 lines; 15 `##` sections including all mandated topics (auth failures, token refresh, device code, browser auth, auth code, keychain, multi-account, 429 throttling, inefficient filter, mail disabled, mail management disabled, read-only, log location, account lifecycle, in-server docs) |
| FR-9 | Error payloads include `see` field for all error classes with troubleshooting sections; build-time test fails on missing/unresolved mapping | PASS | `internal/graph/errors.go` defines `errorSeeTable` with 9 entries; `TestErrorSeeTable_AnchorsCoverEmbeddedHeadings` (`internal/graph/errors_test.go:201`) iterates every unique anchor, normalises each `##` heading to a slug, and fails the build if any anchor is unresolved. All 6 unique anchors pass. Per-class tests `TestErrorSeeHint_InefficientFilter`, `TestErrorSeeHint_Throttling`, `TestErrorSeeHint_SentinelString` provide additional coverage. Gap closed by N+3 fix. |
| FR-10 | Bundle limited to `readme`, `quickstart`, `troubleshooting`; engineering docs excluded; test fails if disallowed path added | PASS | `internal/docs/bundle_allowlist_test.go:TestBundle_OnlyAllowedSlugsPresent` and `TestBundle_AllAllowedSlugsPresent` enforce explicit allowlist; `embed.go` uses explicit file list, not globs |
| FR-11 | `system.status` includes `docs` section with base URI and troubleshooting slug | PASS | `internal/tools/status.go:68-86` defines `statusDocs` with `BaseURI`, `TroubleshootingSlug`, `Version`; `TestStatus_Text` asserts presence |
| FR-12 | Bundle regenerated at build time via `make docs-bundle`; verifies every slug resolves | PASS | `Makefile:54-74` `docs-bundle` target copies source files, runs `TestCatalog_AllSlugsResolve`, `TestBundleSizeUnder2MiB`, secret-pattern scan, regenerates `llms.txt`, runs `TestLLMsTxt_MatchesCatalog`; wired into `ci` target at `Makefile:30` |
| FR-13 | `README.md` refactored for human audience with required sections; deep content excluded | PASS | `README.md` reduced by 1075 lines to ~100 lines; contains project pitch, install pointer, feature list, link to `QUICKSTART.md`, link to `llms.txt`, contributing/license; deep config and troubleshooting content moved out |
| FR-14 | `llms.txt` at repo root per AnswerDotAI standard: H1, blockquote, H2 sections (`Docs`, `Tools`, `Change Requests`, `Optional`), absolute GitHub URLs | PASS | `llms.txt` present; `TestLLMsTxt_StructureCompliesWithStandard` and `TestLLMsTxt_LinksAreAbsolute` pass; file starts with `# outlook-local-mcp`, blockquote, four required H2 sections |
| FR-15 | `llms.txt` regenerated by `make docs-bundle` from same catalog; CI fails if stale | PASS | `cmd/gen-llms/main.go` generates from same catalog; `TestLLMsTxt_MatchesCatalog` fails on drift; wired into `docs-bundle` target |

---

## Acceptance Criteria Verification

| AC | Description | Status | Evidence |
|----|-------------|--------|----------|
| AC-1 | `resources/list` returns one resource per doc with `doc://outlook-local-mcp/` prefix and `text/markdown` MIME | PASS | `TestResourcesList_IncludesBundledDocs` (`internal/server/server_test.go:638`) |
| AC-2 | `search_docs` with `query="InefficientFilter"` ranks troubleshooting first with ±2-line snippets and 1-based line numbers | PASS | `TestSearch_RanksExactMatchesFirst` + `TestSearch_ReturnsSnippetWithLineNumbers` (`internal/docs/search_test.go:13,33`) |
| AC-3 | `get_docs` with `slug=troubleshooting section=token-refresh` returns only that section; `output=raw` returns unmodified markdown | PASS | `TestSystemGetDocs_Section` + `TestSystemGetDocs_Raw` (`internal/tools/get_docs_test.go:11,78`) |
| AC-4 | `InefficientFilter` error envelope includes `see="doc://outlook-local-mcp/troubleshooting#inefficient-filter"` | PASS | `TestErrorSeeHint_InefficientFilter` (`internal/graph/errors_test.go:115`) |
| AC-5 | `system.status` response includes `docs` section with `base_uri`, `troubleshooting_slug`, `docs.version` | PASS | `TestStatus_Text` (`internal/tools/status_test.go:453`); `statusDocs` struct at `internal/tools/status.go:72-87` |
| AC-6 | `make docs-bundle` completes: slugs resolve, size under 2 MiB, no secrets | PASS | Observed clean run: bundle size 23,712 bytes (limit 2,097,152); all three verification steps pass |
| AC-7 | Bundle contains only `readme`, `quickstart`, `troubleshooting`; engineering docs excluded; build fails on violation | PASS | `TestBundle_OnlyAllowedSlugsPresent` (`internal/docs/bundle_allowlist_test.go:36`) enforces exact set |
| AC-8 | `README.md` is human-facing front page; deep content in dedicated docs; links to `llms.txt` | PASS | `README.md:68` links to `llms.txt`; content reduced from 1,075 to ~99 lines |
| AC-9 | `llms.txt` starts with H1, blockquote, H2 sections (`Docs`, `Tools`, `Change Requests`, `Optional`), absolute URLs; regenerated by `make docs-bundle` idempotently | PASS | `TestLLMsTxt_StructureCompliesWithStandard` + `TestLLMsTxt_LinksAreAbsolute` + `TestLLMsTxt_MatchesCatalog` (`internal/docs/llmstxt_test.go:31,78,14`) |

---

## Test Strategy Verification

| Test File | Test Name | Status | Notes |
|-----------|-----------|--------|-------|
| `internal/docs/catalog_test.go` | `TestCatalog_AllSlugsResolve` | PASS | Runs as part of `make docs-bundle` and `make test` |
| `internal/docs/search_test.go` | `TestSearch_RanksExactMatchesFirst` | PASS | Confirmed in test suite output |
| `internal/docs/search_test.go` | `TestSearch_ReturnsSnippetWithLineNumbers` | PASS | Confirmed in test suite output |
| `internal/tools/list_docs_test.go` | `TestSystemListDocs_Text` | PASS | Confirmed in test suite output |
| `internal/tools/get_docs_test.go` | `TestSystemGetDocs_Section` | PASS | Confirmed in test suite output |
| `internal/tools/search_docs_test.go` | `TestSystemSearchDocs_NoResults` | PASS | Confirmed in test suite output |
| `internal/server/server_test.go` | `TestSystemHelp_ListsDocsVerbs` | PASS | Confirmed in test suite output |
| `internal/graph/errors_test.go` | `TestErrorSeeHint_InefficientFilter` | PASS | Confirmed in test suite output |
| `internal/server/server_test.go` | `TestResourcesList_IncludesBundledDocs` | PASS | Confirmed in test suite output |
| `internal/docs/bundle_size_test.go` | `TestBundleSizeUnder2MiB` | PASS | 23,712 bytes (limit 2,097,152) |
| `internal/docs/bundle_secrets_test.go` | `TestBundleContainsNoSecrets` | PASS | Confirmed in test suite output |
| `internal/docs/llmstxt_test.go` | `TestLLMsTxt_MatchesCatalog` | PASS | Runs in `make docs-bundle` and `make test` |
| `internal/docs/llmstxt_test.go` | `TestLLMsTxt_StructureCompliesWithStandard` | PASS | Confirmed in test suite output |
| `internal/docs/llmstxt_test.go` | `TestLLMsTxt_LinksAreAbsolute` | PASS | Confirmed in test suite output |
| `internal/tools/status_test.go` | `TestStatus_Text` (modified) | PASS | Asserts `docs` section present |
| `internal/tools/tool_annotations_test.go` | `TestAggregateAnnotations_System` (modified) | PASS | Conservative aggregation on `system` verified after adding read-only docs verbs |
| `internal/graph/errors_test.go` | `TestErrorSeeTable_AnchorsCoverEmbeddedHeadings` | PASS | Table-driven; iterates all unique anchors in `errorSeeTable`; verifies each resolves to a `##` heading in embedded troubleshooting.md; added by N+3 gap fix |

---

## CRUD Test Verification

All CR-0061 CRUD steps require a live Outlook/Microsoft account. DEFERRED per validation agent instructions.

| Step | Description | Status |
|------|-------------|--------|
| 0a | `system.help` lists `list_docs`, `search_docs`, `get_docs` verbs | DEFERRED (live server) |
| 0a2 | `list_docs` returns 3 slugs: readme, quickstart, troubleshooting | DEFERRED (live server) |
| 0a3 | `search_docs` with `query="token refresh"` ranks troubleshooting slug | DEFERRED (live server) |
| 0a4 | `get_docs` with `slug=troubleshooting section=token-refresh` returns section content | DEFERRED (live server) |
| 0a5 | `get_docs` with `slug=troubleshooting output=raw` returns raw markdown | DEFERRED (live server) |

Steps are documented in `docs/prompts/mcp-tool-crud-test.md` at steps 0a through 0a5 with expected outputs, result table rows, and CR-0061 AC references.

---

## Diff Coverage

| File | +/- Lines | Mapped Requirements | Flag |
|------|-----------|---------------------|------|
| `internal/docs/embed.go` | +14 | FR-1 | — |
| `internal/docs/catalog.go` | +109 | FR-1, FR-4, FR-10 | — |
| `internal/docs/search.go` | +153 | FR-5 | — |
| `internal/docs/llmstxt.go` | +128 | FR-14, FR-15 | — |
| `internal/docs/doc.go` | +21 | FR-1 | — |
| `internal/docs/files/readme.md` | +99 | FR-1, FR-13 (copy of README) | — |
| `internal/docs/files/quickstart.md` | +155 | FR-1 | — |
| `internal/docs/files/troubleshooting.md` | +273 | FR-1, FR-8 (copy of docs/troubleshooting.md) | — |
| `internal/docs/catalog_test.go` | +45 | FR-1, FR-10 | — |
| `internal/docs/bundle_allowlist_test.go` | +99 | FR-10 | — |
| `internal/docs/bundle_secrets_test.go` | +49 | NFR-4 | — |
| `internal/docs/bundle_size_test.go` | +43 | NFR-1 | — |
| `internal/docs/llmstxt_test.go` | +98 | FR-14, FR-15 | — |
| `internal/docs/search_test.go` | +72 | FR-5 | — |
| `internal/tools/list_docs.go` | +97 | FR-4 | — |
| `internal/tools/search_docs.go` | +89 | FR-5 | — |
| `internal/tools/get_docs.go` | +127 | FR-6 | — |
| `internal/tools/list_docs_test.go` | +63 | FR-4 | — |
| `internal/tools/search_docs_test.go` | +74 | FR-5 | — |
| `internal/tools/get_docs_test.go` | +118 | FR-6 | — |
| `internal/tools/status.go` | +28 | FR-11 | — |
| `internal/tools/status_test.go` | +68 | FR-11 | — |
| `internal/graph/errors.go` | +73 | FR-9 | — |
| `internal/graph/errors_test.go` | +70 | FR-9 | — |
| `internal/server/server.go` | +50 | FR-2, FR-3 | — |
| `internal/server/server_test.go` | +106 | FR-2, FR-3, FR-7 | — |
| `internal/server/system_verbs.go` | +83 | FR-7 | — |
| `cmd/gen-llms/main.go` | +17 | FR-14, FR-15 | — |
| `cmd/outlook-local-mcp/main.go` | +2 | FR-1 (wires docs package) | — |
| `Makefile` | +26 | FR-12 | — |
| `README.md` | -1075/+? | FR-13 | — |
| `llms.txt` | +25 | FR-14 | — |
| `docs/troubleshooting.md` | +273 | FR-8 | — |
| `docs/prompts/mcp-tool-crud-test.md` | +40 | CLAUDE.md CRUD test requirement | — |
| `CHANGELOG.md` | +14 | Quality standards (CHANGELOG entry) | — |
| `docs/cr/CR-0061-in-server-documentation-access-for-llm-self-troubleshooting.md` | +190 | CR revisions | — |

No stray changed files detected outside Affected Components defined in the CR.

---

## Gaps

*All requirements, acceptance criteria, and test strategy entries are PASS. `make ci` is clean. CRUD live tests are DEFERRED.*
