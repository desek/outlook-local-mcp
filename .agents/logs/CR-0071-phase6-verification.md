<!--
@agents-index: CR-0071 Phase 6 verification evidence — the end-to-end security
gate result, each of the four `make security` steps run individually, the grype
module-level ceiling (GO-2026-5932, no fix), `make ci`, both container builds,
and per-stage bisectability against the Phase 1 baseline.
-->

# CR-0071 Phase 6 — Verify the gate end to end

Verification captured on branch `dev/cr-0071-0072-dependency-currency` at HEAD
`3e57c45` (Stage 3, mcp-go v0.57.0), after Stages 0–3 landed. Compared against
the Phase 1 baseline in `.agents/logs/CR-0071-phase1-baseline.md`.

Raw evidence (gitignored, `*.log`) under `.agents/logs/`:
`CR-0071-phase6-{verify,govulncheck,vuln-scan,vuln-scan-runA,vuln-scan-runB,license-check,security-after,ci,docker-main,docker-glama}.log`
and `CR-0071-phase6-ci-{984978a,1353ddb,b2925d8}.log`.

## Toolchain consistency (AC-4)

All three files state the same Go version, and it is ≥ 1.25.12:

| Location | Value |
|----------|-------|
| `go version` (built toolchain) | go1.25.12 |
| `go.mod` `go` directive | 1.25.12 |
| `.mise.toml` | 1.25.12 |
| `Dockerfile.Glama` tarball | go1.25.12 |

## The four `make security` steps, each run individually (FR-9, AC-2)

`make security: verify govulncheck vuln-scan license-check`. At baseline
`govulncheck` failed first and masked the three steps behind it. Each is run
alone here and each exits 0 on its own.

| # | Step | Command | Exit | Result |
|---|------|---------|------|--------|
| 1 | verify | `go mod verify` | 0 | `all modules verified` |
| 2 | govulncheck | `govulncheck ./...` | 0 | `No vulnerabilities found` — 0 reachable |
| 3 | vuln-scan | `make vuln-scan` (syft + `grype --fail-on high`) | 0 | 1 module finding, Unknown severity, does not trip `--fail-on high` |
| 4 | license-check | `make license-check` (syft + `grant`) | 0 | `No denied packages found` (185 allowed) |

Aggregate `make security` exits **0** (`CR-0071-phase6-security-after.log`).

## Instrument validation for grype (Measurement & Verification standard)

The first `vuln-scan` run emitted `WARN current database is invalid ... built 18
weeks ago (max allowed age is 5 days)`. That warning came from a stale cached
grype DB which grype auto-refreshed mid-scan. After the refresh, `grype db
status` reports the DB built `2026-07-31T07:09:24Z` (system clock
`2026-07-31 12:06 UTC`, i.e. ~5 h old) with `Status: valid`, and `grype db
update` reports `No vulnerability database update available`. With the DB
confirmed fresh, `vuln-scan` was run twice: the grype output is **byte-identical
across both runs** (`runA` vs `runB` diff empty) and both exit 0. The instrument
agrees with itself on the unchanged tree and is therefore trustworthy.

## Risk 3 materialized: a grype/govulncheck module-level ceiling

Fixing `govulncheck` exposed exactly the module-level finding Risk 3 predicted.
It is documented here rather than silenced; `--fail-on high` was **not** relaxed.

* **Finding:** `GO-2026-5932` in `golang.org/x/crypto` v0.54.0.
* **What it is:** the `golang.org/x/crypto/openpgp` package is unmaintained,
  unsafe by design, and has known security issues.
* **Reachability:** not reachable. `govulncheck` reports it under "modules you
  require, but your code doesn't appear to call" — 0 reachable findings. This
  server imports no `openpgp` code.
* **grype disposition:** reported at module level (grype matches by module, not
  reachability) with severity **Unknown**, so `grype --fail-on high` exits 0.
* **Ceiling — Fixed in: N/A.** We are already on the latest `golang.org/x/crypto`
  (`go list -m -u golang.org/x/crypto` shows no available update). The advisory
  has no fixed version and never will: the package is intentionally deprecated
  upstream.
* **What would move it:** only upstream shipping a fixed `x/crypto` release
  (not expected, the package is deprecated by design) or the transitive
  requirement on `x/crypto` dropping the `openpgp` subtree. Neither is
  actionable within this CR.

This ceiling belongs in `docs/reference/security.md` (Phase 7) per Risk 3; it is
recorded here as the Phase 6 discovery.

## `make ci` (NFR-2)

`make ci` exits **0** (`CR-0071-phase6-ci.log`): docs-bundle, build, vet,
fmt-check, tidy, lint (`0 issues`), test (all packages ok), goreleaser-check,
mcpb-validate all pass.

## Container builds (NFR-4, AC-14)

Both images build against a live Docker daemon (Docker Desktop 4.80.0):

| Image | Command | Exit | Embedded Go version |
|-------|---------|------|---------------------|
| `outlook-local-mcp:cr0071` | `docker build -t outlook-local-mcp:cr0071 .` | 0 | go1.25.12 |
| `outlook-local-mcp:cr0071-glama` | `docker build -f Dockerfile.Glama -t outlook-local-mcp:cr0071-glama .` | 0 | go1.25.12 |

Go version confirmed with `go version <binary>` on the binary extracted from
each image (`/usr/local/bin/outlook-local-mcp`), satisfying AC-14's "1.25.12 or
later" requirement.

## Bisectability — each stage passes `make ci` alone (NFR-5, AC-7)

Each of the four stage commits was checked out and `make ci` run on it
independently (Stages 0–2 via a detached worktree, Stage 3 as HEAD):

| Stage | Commit | `make ci` exit |
|-------|--------|----------------|
| Stage 0, toolchain | `984978a` | 0 |
| Stage 1, low-risk currency | `1353ddb` | 0 |
| Stage 2, Graph SDK family | `b2925d8` | 0 |
| Stage 3, mcp-go | `3e57c45` (HEAD) | 0 |

(`733bd55`, the phase-5 test-add commit, sits between Stage 2 and Stage 3 and is
not itself a stage.)

## Before / after finding counts (vs Phase 1 baseline)

| Instrument | Baseline (Phase 1) | After (Phase 6) |
|------------|--------------------|-----------------|
| `make security` exit | 2 (failed at govulncheck) | **0** |
| `govulncheck` reachable findings | 13 | **0** |
| `go mod verify` | passed | passed |
| `grype` (via vuln-scan) | did not run (masked) | **exit 0**, 1 Unknown-severity module finding (GO-2026-5932, no fix, unreachable) |
| `grant` (via license-check) | did not run (masked) | **exit 0**, no denied packages |
| `make ci` | (n/a) | **0** |
| Container builds | (n/a) | both **succeed**, both go1.25.12 |

The 13 reachable `govulncheck` findings are all cleared; the only remaining
finding is the unreachable, no-fix `x/crypto/openpgp` module-level ceiling
documented above. No result was fabricated and no gate was relaxed.

## Method

```bash
go mod verify                    # exit 0
govulncheck ./...                # exit 0, 0 reachable
grype db update && grype db status   # DB valid, built today
make vuln-scan                   # exit 0, run twice, identical output
make license-check               # exit 0
make security                    # exit 0
make ci                          # exit 0
docker build -t outlook-local-mcp:cr0071 .                          # exit 0
docker build -f Dockerfile.Glama -t outlook-local-mcp:cr0071-glama . # exit 0
# per-stage make ci in a detached worktree for 984978a, 1353ddb, b2925d8
```
