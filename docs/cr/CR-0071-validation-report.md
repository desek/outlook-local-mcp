# CR-0071 Validation Report

Validated 2026-07-31 by cr-validator against branch
`dev/cr-0071-0072-dependency-currency` at finalization commit `b7e6d3f`.
Diff base: `git merge-base origin/main HEAD` = `ea10208`.

Evidence was produced by re-running the gates on the working tree (not by
trusting prior agents): `make ci` (exit 0), `make security` (exit 0),
`go mod verify`, `govulncheck ./...`, the two added tests, `go mod tidy` (clean),
and `go list -m -u all`. Merge-time and container-build claims are traced to the
Phase 6 and Phase 8 evidence logs under `.agents/logs/` and marked accordingly.

## Summary

Requirements: 15/17 PASS (2 deferred-to-merge) | Non-Functional: 5/5 PASS |
Acceptance Criteria: 12/15 PASS, 1 N/A, 2 deferred-to-merge |
Tests: 2/2 added tests PASS, `make crud-test` NOT run (GAP) | Gaps: 1

Counts by status: **PASS 34 · PARTIAL 0 · GAP 1 · FAIL 0 · DEFERRED 4 · N/A 1**

DEFERRED rows (FR-16, FR-17, AC-9, AC-10) are merge-time requirements. The
branch is unpushed and unmerged by design (Phase 8 is preparation-only), so they
are **not failures** but are **unverified as of this report**.

## Requirement Verification

| Req # | Description | Status | Evidence (file:line / test name) |
|-------|-------------|--------|----------------------------------|
| FR-1 | `.mise.toml` pins Go ≥ 1.25.12 | PASS | `.mise.toml:4` `go = "1.25.12"` |
| FR-2 | `go.mod` `go` directive and `Dockerfile.Glama` URL match `.mise.toml` | PASS | `go.mod:3` `go 1.25.12`; `Dockerfile.Glama:9` `go1.25.12.linux-amd64.tar.gz` |
| FR-3 | Every direct dep in the currency table raised (except mcp-go per FR-5) | PASS | `go.mod` diff: azcore v1.22.0, azidentity v1.14.0, MSAL v1.8.0, msgraph-sdk-go v1.100.0, -core v1.4.1, grpc v1.83.0, otel×7 v1.44.0; `go list -m -u all` reports no direct update available |
| FR-4 | Indirect graph tidied, `go.sum` consistent | PASS | `go mod tidy` on HEAD leaves `go.mod`/`go.sum` byte-unchanged; `go.sum` diff present |
| FR-5 | mcp-go raised to v0.57.0 OR drop declared with follow-up CR | PASS | `go.mod` `github.com/mark3labs/mcp-go v0.57.0`; not dropped (Phase 8 log line 5: "FR-5 exit not invoked") |
| FR-6 | `govulncheck` reports no reachable vulnerability | PASS | `govulncheck ./...` re-run: "0 vulnerabilities … your code affected by 0" |
| FR-7 | `grype --fail-on high` exits 0 | PASS | `make security` re-run: grype reports only GO-2026-5932 at severity Unknown; exit 0 |
| FR-8 | Licence check exits 0 | PASS | `make security` re-run: grant "No denied packages found" (185 allowed) |
| FR-9 | `make security` exits 0, each of 4 steps shown to pass | PASS | `make security` exit 0 (re-run); four steps individually recorded in `.agents/logs/CR-0071-phase6-verification.md:29-42` |
| FR-10 | `.github/dependabot.yml` declares `gomod` on `/` and `github-actions` on `/` | PASS | `.github/dependabot.yml:26-27,39-40` |
| FR-11 | Version updates grouped per ecosystem; security updates not grouped | PASS | `.github/dependabot.yml:30-35,43-47` (`applies-to: version-updates` groups only; no security grouping) |
| FR-12 | `security.md` states what each instrument measures, why counts differ, triage procedure | PASS | `docs/reference/security.md:13-67` |
| FR-13 | `security.md` records `x/crypto/ssh` reachability naming the establishing command | PASS | `docs/reference/security.md:69-97` names `govulncheck ./...` and `go list -deps ./... \| grep golang.org/x/crypto/ssh` |
| FR-14 | `security.md` states advisory-ecosystem rule and `fix(deps):` merge rule | PASS | `docs/reference/security.md:99-125` (Rule 1 GHSA-7j59-v9qr-6fq9; Rule 2 fix(deps)) |
| FR-15 | CLAUDE.md lists `security` in `docs/reference/` enum and points to it from Quality Standards | PASS | `AGENTS.md:146` (enum) and `AGENTS.md:242-247` (Quality Standards pointer); `CLAUDE.md` is a symlink to `AGENTS.md` (`readlink CLAUDE.md` = `AGENTS.md`) |
| FR-16 | No open Dependabot alert against `go.mod` after merge | DEFERRED | Merge-time. Branch unpushed. Phase 8 log lines 27-63 project all 16 alerts (#2-#17) auto-close on push to `main`; none needs dismissal. UNVERIFIED as of this report |
| FR-17 | PR squash-merged with `fix(deps):` title | DEFERRED | Merge-time. No PR exists yet (Phase 8 log lines 9-12, 109-122). UNVERIFIED |

## Non-Functional Requirement Verification

| NFR # | Description | Status | Evidence |
|-------|-------------|--------|----------|
| NFR-1 | No observable MCP tool surface change | PASS | `TestVerbInventoryUnchangedAfterUpgrade` passes (golden over verb ids + 4 hints; the CR's designated NFR-1 check, `dispatch_registry_test.go:137-175`); `extension/manifest.json` unchanged (absent from diff); no non-test source file under `internal/` changed |
| NFR-2 | `make ci` passes incl `-race` | PASS | `make ci` re-run: exit 0; test target runs `-race` with coverage; all packages ok |
| NFR-3 | `extension/manifest.json` unchanged | PASS | Not present in `git diff ea10208...HEAD --name-only` |
| NFR-4 | Both Dockerfiles build | PASS | `.agents/logs/CR-0071-phase6-verification.md:87-98` (both images build, both go1.25.12). Not independently re-run in this audit; traced to Phase 6 evidence |
| NFR-5 | Each stage a separate, independently-CI-passing commit | PASS | Separate commits confirmed: Stage 0 `984978a`, Stage 1 `1353ddb`, Stage 2 `b2925d8`, Stage 3 `3e57c45`; per-stage `make ci` in `.agents/logs/CR-0071-phase6-verification.md:100-113` |

## Acceptance Criteria Verification

| AC # | Description | Status | Evidence |
|------|-------------|--------|----------|
| AC-1 | Security gate passes end to end | PASS | `make security` re-run exit 0 (verify + govulncheck + grype + grant all pass) |
| AC-2 | Each gate step verified independently | PASS | `.agents/logs/CR-0071-phase6-verification.md:29-42`; `go mod verify` and `govulncheck ./...` also re-run standalone here |
| AC-3 | Every direct dependency current | PASS | `go list -m -u all` (direct filter) returns no available update |
| AC-4 | Toolchain pin patched (≥1.25.12) and consistent across 3 files | PASS | `.mise.toml:4`, `go.mod:3`, `Dockerfile.Glama:9` all `1.25.12` |
| AC-5 | Drifting toolchain pin fails the test suite | PASS | `TestGoVersionMatchesToolchainPin` passes; failure branch `snapshot_test.go:110-115` errors naming both pinned and built versions |
| AC-6 | MCP surface provably unchanged via golden test | PASS | `TestVerbInventoryUnchangedAfterUpgrade` passes; verified as a genuine bidirectional set assertion over `{domain}.{operation}` + all four annotation hints (`dispatch_registry_test.go:39-82,143-166`), NOT a count — a drop-and-add would fail both loops |
| AC-7 | Staging is bisectable (4 separate commits, each passes `make ci`) | PASS | Commits `984978a`/`1353ddb`/`b2925d8`/`3e57c45`; per-stage `make ci` logged in Phase 6 |
| AC-8 | A dropped mcp-go stage is declared, not silent | N/A | Precondition not met: Stage 3 (mcp-go v0.57.0) IS present in the diff (`go.mod`), so the FR-5 drop path was not taken. AC vacuously satisfied |
| AC-9 | Alert backlog and PR #23 resolved on merge | DEFERRED | Merge-time. Phase 8 log lines 27-89 record plan (auto-close 16 alerts; close PR #23 as superseded). UNVERIFIED as of this report |
| AC-10 | The fix reaches a release (`fix(deps):`, patch bump) | DEFERRED | Merge-time. No squash-merge performed. UNVERIFIED |
| AC-11 | Dependabot maintains currency going forward | PASS | `.github/dependabot.yml:26-47` (both ecosystems on `/`, version-updates grouped, security ungrouped) |
| AC-12 | Instrument disagreement documented | PASS | `docs/reference/security.md:13-67` (what each measures, why counts differ, triage) plus Rule 1/Rule 2 at :99-125 |
| AC-13 | Unreachable-advisory assessment names its evidence | PASS | `docs/reference/security.md:80-92` names `govulncheck ./...` and the `go list -deps` import check, reproducible |
| AC-14 | Both container builds succeed; binary reports Go ≥1.25.12 via `system.about` | PASS | `.agents/logs/CR-0071-phase6-verification.md:87-98`: both images build, go1.25.12 confirmed via `go version <binary>`. Note: confirmed through `go version` on the extracted binary, not by invoking `system.about` at runtime; the value is identical (`buildinfo.Snapshot.GoVersion = runtime.Version()`, `snapshot_test.go:134`). Not re-run in this audit |
| AC-15 | CLAUDE.md points to the new reference in both required places | PASS | `AGENTS.md:146` (Documentation Governance enum) and `AGENTS.md:242-247` (Quality Standards pointer); CLAUDE.md symlinks AGENTS.md |

## Test Strategy Verification

| Test File | Test Name | Specified | Exists | Matches Spec |
|-----------|-----------|-----------|--------|--------------|
| `internal/buildinfo/snapshot_test.go` | `TestGoVersionMatchesToolchainPin` | Yes | Yes (`:90-116`) | Yes — asserts built toolchain ≥ `.mise.toml` pin, skips when `.mise.toml` absent, message names both versions. Re-run: PASS |
| `internal/tools/dispatch_registry_test.go` | `TestVerbInventoryUnchangedAfterUpgrade` | Yes | Yes (`:143-175`) | Yes — golden over verb id set AND four annotation hints, bidirectional membership check (not a count). Re-run: PASS |
| (harness) | `make crud-test` (Quality Standards checklist; Phases 5, 6; Verification Commands; Risk 1) | Yes | N/A | **GAP** — did not run. See Gaps |

## Diff Coverage

| File | +/- | Mapped Requirements |
|------|-----|---------------------|
| `.mise.toml` | +1/-1 | FR-1, AC-4 |
| `go.mod` | +/- | FR-3, FR-4, FR-5, AC-3 |
| `go.sum` | +/- | FR-4 |
| `Dockerfile.Glama` | +1/-1 | FR-2, AC-4 |
| `.github/dependabot.yml` | +47 | FR-10, FR-11, AC-11 |
| `docs/reference/security.md` | +155 | FR-12, FR-13, FR-14, AC-12, AC-13 |
| `AGENTS.md` (== `CLAUDE.md` symlink) | +9/-2 | FR-15, AC-15 |
| `internal/buildinfo/snapshot_test.go` | +110 | AC-5, Test Strategy |
| `internal/tools/dispatch_registry_test.go` | +175 | NFR-1, AC-6, Test Strategy |
| `docs/cr/CR-0071-...md` | +1126 | The CR itself (finalizer frontmatter update, review summary) |
| `.agents/logs/CR-0071-phase1-baseline.md` | +112 | Phase 1 evidence (baseline) |
| `.agents/logs/CR-0071-phase6-verification.md` | +144 | FR-9, NFR-4, NFR-5, AC-2, AC-7, AC-14 evidence |
| `.agents/logs/CR-0071-phase8-merge-readiness.md` | +140 | FR-16, FR-17, AC-9, AC-10 (merge-time plan) |

### Unmapped changed files

* `docs/cr/CR-0072-site-dependency-currency-and-vulnerability-remediation.md`
  (+922) — outside CR-0071's Affected Components, but **justified**: CR-0071's
  Dependencies section (lines 957-961) and the Phase 8 log (lines 100-107)
  state CR-0072 shares this branch and PR. It is an authored CR document, not
  source, and does not touch CR-0071's implementation surface.

No non-test source file under `internal/` was changed. This corroborates NFR-1
and the CR's expectation that the mcp-go and Graph SDK bumps required no source
adaptation; there is no silent source edit hiding a surface change.

## Gaps

1. **`make crud-test` did not run, and the Quality Standards checkbox is
   inaccurately marked.** The CR marks `- [x] make crud-test passes after Stage 3`
   with the note "verified in CI" (CR line 799). This is not accurate:
   - No workflow under `.github/workflows/` references `crud-test` (grep of all
     six workflows returns nothing), so CI does not run it.
   - No crud-test evidence exists anywhere (`.agents/logs/` contains no crud-run
     log; there is no Phase 5 evidence markdown).
   - Phase 5 of the Implementation Approach (CR line 556), Phase 6 (line 974),
     the Verification Commands (line 838), and Risk 1's mitigation (line 861)
     all call for `make crud-test` after the mcp-go bump, precisely because it
     is the runtime behavioral check that a framework migration can break
     observable tool behavior without changing any file under `internal/tools/`.

   **Assessment:** `make crud-test` legitimately cannot run in this environment
   (it needs live Microsoft 365 credentials and an authenticated `claude` CLI,
   and would mutate real user data), so its absence is defensible. What is a
   defect is the *claim* that it was "verified in CI" when no CI path exists for
   it. The static substitute — `TestVerbInventoryUnchangedAfterUpgrade` plus the
   unchanged `manifest.json` — is strong and covers verb identity and annotation
   drift, but it does not exercise runtime request/response behavior through the
   upgraded mcp-go v0.57.0, which is exactly the AC-6 / Risk 1 scenario.

   **Suggested minimal fix (documentation-only):** correct the Quality Standards
   checkbox note from "verified in CI" to an honest statement, e.g. "not run:
   requires live M365 credentials and an authenticated claude CLI; must be
   executed manually before merge per Phase 6. NFR-1 surface stability is
   covered statically by TestVerbInventoryUnchangedAfterUpgrade." Either run
   `make crud-test` against a test tenant and record the result under
   `.agents/logs/`, or downgrade the checkbox to unchecked with the manual
   follow-up noted. Do not leave a false "verified in CI" attribution.

### Deferred-to-merge (not gaps, but unverified as of this report)

FR-16, FR-17, AC-9, AC-10 are merge-time actions. The branch is intentionally
unpushed and no pull request exists (Phase 8 is preparation-only, CR-0072 has
yet to land on the branch). Their evidence — alert auto-closure, the `fix(deps):`
squash title, PR #23 resolution, and the release-please patch bump — cannot
exist until the human executes the Phase 8 steps. They are correctly staged and
documented; they are simply not verifiable now.
