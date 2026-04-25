---
cr: CR-0064
title: Validation Report — Conditional Implicit Default Account Registration
date: 2026-04-25
branch: dev/cr-0061
validator: Agent N+2 (Validation)
status: PASS
---

# CR-0064 Validation Report

## Summary

CR-0064 implements (1) conditional implicit "default" account registration in `cmd/outlook-local-mcp/main.go` gated on `accounts.json` contents, (2) a `FindByIdentity` helper in `internal/auth/accounts.go`, (3) persistent `account.remove` via atomic `accounts.json` rewrite, and (4) troubleshooting documentation under the `auto-default-account` anchor. All Functional Requirements, Non-Functional Requirements, and Acceptance Criteria are mapped to changed files with specific hunks and to passing tests in `go test -v`. `make ci` passes cleanly. CRUD test prompt updated with Steps 29a/29b. No stray changes outside Affected Components.

Counts:

- Functional Requirements: 5/5 PASS
- Non-Functional Requirements: 2/2 PASS
- Acceptance Criteria: 6/6 PASS (AC-5 manual via Step 29b; behavior covered transitively by AC-3+AC-4 evidence)
- Test Strategy rows added: 10/10 PASS
- Gaps: 0

## Diff Coverage

Base: `git merge-base origin/main HEAD` → CR-0064 commits beginning at `0cfaffa..ac0dc4e` (4 commits). Diff against the CR-0064 baseline (`52835b1`, draft commit) restricted to CR-0064-only changes:

```
cmd/outlook-local-mcp/main.go                                                  |  84 ++++++++++---
cmd/outlook-local-mcp/main_test.go                                             |  48 +++++++
docs/cr/CR-0064-conditional-implicit-default-account-registration.md           |  44 +++---
docs/prompts/mcp-tool-crud-test.md                                             |  30 +++++
docs/troubleshooting.md                                                        |  29 +++++
internal/auth/accounts.go                                                      |  25 ++++
internal/auth/accounts_test.go                                                 |  63 +++++++++
internal/docs/files/troubleshooting.md                                         |  29 +++++
internal/docs/search_test.go                                                   |  27 ++++
internal/tools/remove_account_test.go                                          | 129 +++++++++++++++++
```

Every changed file maps to a CR-0064 Affected Component or to the Test Strategy table. No stray changes detected.

## Requirement Verification

| ID | Requirement | Evidence (file:line) | Test Evidence | Status |
|----|-------------|----------------------|---------------|--------|
| FR-1 | Implicit "default" registered only when accounts.json has no "default" label AND no identity match | `cmd/outlook-local-mcp/main.go:104-126` (gated `registry.Add`); `cmd/outlook-local-mcp/main.go:213-256` (`shouldAddImplicitDefault`) | `TestStartup_SkipsImplicitDefault_WhenAccountsJsonCoversCfg`, `TestStartup_SkipsImplicitDefault_WhenDefaultLabelInAccountsJson` PASS | PASS |
| FR-2 | When either condition false, MUST NOT register implicit default | `cmd/outlook-local-mcp/main.go:240-254` (early-return false branches) | Same two tests above PASS | PASS |
| FR-3 | `FindByIdentity(accounts, clientID, tenantID)` exported; empty args return `(zero, false)` | `internal/auth/accounts.go:180-202` | `TestFindByIdentity_Match`, `TestFindByIdentity_NoMatch`, `TestFindByIdentity_EmptyArgs` PASS | PASS |
| FR-4 | `HandleRemoveAccount` rewrites accounts.json; missing file or no match still succeeds and does not create empty file | `internal/tools/remove_account.go:66,92-95` (calls `RemoveAccountConfig`); `internal/auth/accounts.go:151-178` (filter+save logic) | `TestRemoveAccount_RewritesAccountsJson`, `TestRemoveAccount_ImplicitDefault_NoFileWrite` PASS | PASS |
| FR-5 | `docs/troubleshooting.md` documents ghost-default + persistent removal under anchor `auto-default-account` | `docs/troubleshooting.md:263-289`; mirrored at `internal/docs/files/troubleshooting.md` | `TestSearchDocs_AutoDefaultAnchor` PASS | PASS |
| NFR-1 | Atomic accounts.json rewrites (temp+rename) | `internal/auth/accounts.go:88-122` (`os.CreateTemp` + `os.Rename`) | `TestRemoveAccount_AtomicWrite_NoPartialFileOnError` PASS (original preserved on write failure) | PASS |
| NFR-2 | `FindByIdentity` O(n) over <100 accounts | `internal/auth/accounts.go:194-200` (single-pass for-range) | Code inspection; complexity bound trivially satisfied | PASS |

## Acceptance Criteria Verification

| AC | Statement | Evidence | Test/Step | Status |
|----|-----------|----------|-----------|--------|
| AC-1 | Implicit default skipped when accounts.json covers cfg identity | `cmd/outlook-local-mcp/main.go:243-247` | `TestStartup_SkipsImplicitDefault_WhenAccountsJsonCoversCfg` PASS | PASS |
| AC-2 | Implicit default skipped when accounts.json has explicit "default" label | `cmd/outlook-local-mcp/main.go:249-254` | `TestStartup_SkipsImplicitDefault_WhenDefaultLabelInAccountsJson` PASS | PASS |
| AC-3 | Implicit default added when accounts.json empty/missing | `cmd/outlook-local-mcp/main.go:256` (default-true return); `main.go:104-122` | `TestStartup_AddsImplicitDefault_WhenAccountsJsonEmpty` PASS | PASS |
| AC-4 | account_remove rewrites accounts.json (not re-added on restart) | `internal/tools/remove_account.go:92-95` | `TestRemoveAccount_RewritesAccountsJson` PASS; CRUD Step 29a | PASS |
| AC-5 | Removing a named cfg-identity-covering entry reinstates default at restart | `cmd/outlook-local-mcp/main.go:243-247` (gating signal removal); `docs/troubleshooting.md:286-289` | CRUD Step 29b (manual; informational); transitively covered by AC-3+AC-4 unit tests | PASS |
| AC-6 | system.search_docs returns heading with anchor `auto-default-account` | `docs/troubleshooting.md:263`; `internal/docs/files/troubleshooting.md` (regenerated bundle) | `TestSearchDocs_AutoDefaultAnchor` PASS | PASS |

## Test Strategy Verification

All 10 Test Strategy rows mapped to passing tests in `go test -v`:

| Test File | Test Name | go test Result |
|-----------|-----------|---------------|
| `internal/auth/accounts_test.go` | `TestFindByIdentity_Match` | PASS |
| `internal/auth/accounts_test.go` | `TestFindByIdentity_NoMatch` | PASS |
| `internal/auth/accounts_test.go` | `TestFindByIdentity_EmptyArgs` (3 subtests) | PASS |
| `cmd/outlook-local-mcp/main_test.go` | `TestStartup_SkipsImplicitDefault_WhenAccountsJsonCoversCfg` | PASS |
| `cmd/outlook-local-mcp/main_test.go` | `TestStartup_SkipsImplicitDefault_WhenDefaultLabelInAccountsJson` | PASS |
| `cmd/outlook-local-mcp/main_test.go` | `TestStartup_AddsImplicitDefault_WhenAccountsJsonEmpty` | PASS |
| `internal/tools/remove_account_test.go` | `TestRemoveAccount_RewritesAccountsJson` | PASS |
| `internal/tools/remove_account_test.go` | `TestRemoveAccount_ImplicitDefault_NoFileWrite` | PASS |
| `internal/tools/remove_account_test.go` | `TestRemoveAccount_AtomicWrite_NoPartialFileOnError` | PASS |
| `internal/docs/search_test.go` | `TestSearchDocs_AutoDefaultAnchor` | PASS |

`make ci` (build, vet, fmt-check, tidy, lint, test, docs-bundle, goreleaser check, mcpb validate) completes with `0 issues` from golangci-lint and all test packages green.

## CRUD Test Verification

CRUD live execution skipped per orchestration template (interactive Microsoft auth required). Per-tool spec coverage inspected in `docs/prompts/mcp-tool-crud-test.md`:

- **Step 29a (Durable account removal):** Multi-account-only step. Calls `account.list`, `account.remove <target>`, restarts the server, re-asserts `account.list` does not include `<target>`, and restores via `account.add`. Covers AC-4 in live testing. Includes pass/fail/skip states and a summary-table row (`29a`).
- **Step 29b (Default reappearance, informational):** Manual procedure for AC-5. Marked SKIP for automated suites; provides explicit reproduction steps to verify env-only fallback semantics when `accounts.json` loses its only cfg-identity-covering entry.

CRUD prompt accurately reflects the new behavior of the `account.remove` verb. No other verbs changed; no other CRUD steps require updates.

## Gaps

None. All FRs, NFRs, and ACs satisfied with both file-hunk evidence and passing-test evidence (or documented manual CRUD steps for AC-5). Quality Standards Compliance checkboxes in the CR are accurate against the implementation. `extension/manifest.json` correctly unchanged (no new MCP tools).
