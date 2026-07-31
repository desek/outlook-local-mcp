# CR-0071 Phase 8: Merge-readiness record ("Close the loop")

Prepared 2026-07-31 on branch `dev/cr-0071-0072-dependency-currency` at
checkpoint `1f4dfe6` (Phase 7 complete). Stages 0-3 all landed; the `mcp-go`
Stage 3 bump was **not** dropped (FR-5 exit not invoked).

This record is PREPARATION ONLY. Phase 8's terminal actions (squash-merge,
alert closure, PR #23 resolution) are irreversible human decisions and are
**not** executed here. The branch is not yet pushed, no pull request exists for
it yet, and CR-0072's commits are still to come on this same branch. This file
records the current state and the exact actions left for the human so the loop
can be closed correctly once the branch is pushed and CR-0072 is implemented.

## 1. Implemented resolved versions (live `go.mod` at `1f4dfe6`)

| Module | Alert-required floor | Resolved in `go.mod` | Clears range? |
|--------|----------------------|----------------------|---------------|
| `github.com/microsoft/kiota-http-go` | 1.5.5 | v1.5.6 (indirect) | Yes |
| `golang.org/x/net` | 0.55.0 | v0.57.0 (indirect) | Yes |
| `golang.org/x/crypto` | 0.52.0 | v0.54.0 (indirect) | Yes |
| `google.golang.org/grpc` | 1.82.1 | v1.83.0 (direct) | Yes |
| `golang.org/x/text` | 0.39.0 (govulncheck, no Dependabot alert) | v0.40.0 (indirect) | Yes |

Toolchain consistent across all three files at **go 1.25.12** (`.mise.toml`,
`go.mod` `go` directive, `Dockerfile.Glama` tarball URL).

## 2. Live Dependabot alert state (read-only, `gh api`, 2026-07-31)

### go.mod alerts (CR-0071 scope) — `dependency.manifest_path == go.mod`

Sixteen open, one already fixed. Every open one is cleared by the resolved
version above, so **all sixteen should auto-close** once this change reaches the
default branch (`main`). Dependabot re-evaluates `go.mod`/`go.sum` on push to the
default branch and transitions alerts whose vulnerable range no longer matches
to `fixed`. **None requires a written dismissal** — there is no unreachable
alert being carried, so FR-16's dismissal clause does not apply to any go.mod
alert.

| # | State now | GHSA | Sev | Package | Floor | Expected after merge |
|---|-----------|------|-----|---------|-------|----------------------|
| 1 | fixed | (otel/sdk) | high | go.opentelemetry.io/otel/sdk | 1.43.0 | already fixed |
| 2 | open | GHSA-7j59-v9qr-6fq9 | high | kiota-http-go | 1.5.5 | auto-close |
| 3 | open | GHSA-5cv4-jp36-h3mw | medium | golang.org/x/net | 0.55.0 | auto-close |
| 4 | open | GHSA-w879-237q-wc7r | high | golang.org/x/crypto | 0.52.0 | auto-close |
| 5 | open | GHSA-x527-x647-q7gg | critical | golang.org/x/crypto | 0.52.0 | auto-close |
| 6 | open | GHSA-45gg-vh54-h5m9 | medium | golang.org/x/crypto | 0.52.0 | auto-close |
| 7 | open | GHSA-f5wc-c3c7-36mc | critical | golang.org/x/crypto | 0.52.0 | auto-close |
| 8 | open | GHSA-9m57-25v3-79x9 | medium | golang.org/x/crypto | 0.52.0 | auto-close |
| 9 | open | GHSA-vgwf-h737-ff37 | critical | golang.org/x/crypto | 0.52.0 | auto-close |
| 10 | open | GHSA-q4h4-gmj2-qvw2 | high | golang.org/x/crypto | 0.52.0 | auto-close |
| 11 | open | GHSA-rm3j-f69w-wqmq | critical | golang.org/x/crypto | 0.52.0 | auto-close |
| 12 | open | GHSA-89gr-r52h-f8rx | critical | golang.org/x/crypto | 0.52.0 | auto-close |
| 13 | open | GHSA-5cgq-3rg8-m6cv | critical | golang.org/x/crypto | 0.52.0 | auto-close |
| 14 | open | GHSA-jppx-rxg9-jmrx | critical | golang.org/x/crypto | 0.52.0 | auto-close |
| 15 | open | GHSA-78mq-xcr3-xm33 | medium | golang.org/x/crypto | 0.52.0 | auto-close |
| 16 | open | GHSA-qpw4-5x99-6vjp | medium | golang.org/x/crypto | 0.52.0 | auto-close |
| 17 | open | GHSA-hrxh-6v49-42gf | high | google.golang.org/grpc | 1.82.1 | auto-close |

Counts match the CR: 16 open go.mod alerts (#2-#17), of which 13 are
`golang.org/x/crypto` (7 critical, 2 high, 4 medium), plus one each for
`kiota-http-go`, `golang.org/x/net`, and `grpc`. AC-9 is satisfied by
auto-close; no dismissal reason referencing `docs/reference/security.md` is
needed for any go.mod alert.

### npm alerts (NOT CR-0071 scope) — `dependency.manifest_path == site/pnpm-lock.yaml`

Out of scope for this phase. Recorded only so they are not mistaken for go.mod
alerts when confirming AC-9 after merge. These belong to CR-0072.

| # | State now | Package | Ecosystem |
|---|-----------|---------|-----------|
| 18 | open | tmp | npm |
| 19 | open | uuid | npm |
| 20 | open | tmp | npm |
| 21 | auto_dismissed | brace-expansion | npm |

## 3. PR #23 (the deadlocked Dependabot PR)

Live state: **OPEN**, `MERGEABLE`, base `main`, head
`dependabot/go_modules/go_modules-0ba9e44b80`, title
`chore(deps): bump github.com/microsoft/kiota-http-go from 1.5.4 to 1.5.5 in the go_modules group across 1 directory`.

**Superseded by this CR.** This branch already carries `kiota-http-go` at
**v1.5.6** (one patch beyond PR #23's target of 1.5.5), so merging PR #23 would
add nothing and, being a `chore(deps):` title, would produce no release.

Recommended human action per AC-9: **close PR #23 as superseded** (do not merge
it). A close comment should point at this CR's PR. Dependabot will not reopen it
because alert #2 (the alert it was filed against) will already be `fixed`.

## 4. PR #28 (pending release) and sequencing

Live state: **OPEN**, title `chore(main): release 0.5.0`.

Per the CR's Dependencies section, **#28 should merge first**. Landing this CR
before #28 would fold a security patch into a release that is mostly the site
work. With #28 merged first, this CR lands cleanly as **0.5.1** — a release
whose entire content is "take this".

## 5. CR-0072 shares this branch and PR

CR-0072's commits are **still to come on this same branch** and share this CR's
single pull request. CR-0072 adds the `npm` ecosystem entry to the
`.github/dependabot.yml` created here (Stage 4) and remediates the site/npm
alerts (#18-#20). The combined change is governed by this CR's `fix(deps):`
squash-merge title (FR-17). Do not open the PR or squash-merge until CR-0072's
commits are also on the branch.

## 6. Exact actions left for the human (in order)

1. **Merge release PR #28** (`chore(main): release 0.5.0`) to `main` first, so
   this work lands as a clean 0.5.1.
2. **Implement CR-0072** on this same branch (`dev/cr-0071-0072-dependency-currency`),
   committing its stages after CR-0071's.
3. **Push the branch** and open a single pull request for the combined
   CR-0071 + CR-0072 change against `main`.
4. **Squash-merge with a `fix(deps):` title** — NOT `chore(deps):`.
   `release-please` is `release-type: go`, `bump-minor-pre-major: true`,
   `bump-patch-for-minor-pre-major: false`; a `chore` produces **no release**,
   a `fix` produces a **patch** bump. Expected result: **0.5.0 -> 0.5.1**.
   Suggested title:
   `fix(deps): bring the Go module graph and toolchain current (CR-0071, CR-0072)`
5. **Confirm all sixteen go.mod alerts (#2-#17) transition to `fixed`** after
   the merge reaches `main`:
   ```
   gh api repos/desek/outlook-local-mcp/dependabot/alerts --paginate \
     -q '.[] | select(.dependency.manifest_path=="go.mod" and .state=="open") | .number'
   ```
   Expect empty output. If any go.mod alert remains open, investigate before
   dismissing; no go.mod alert is expected to need a written dismissal.
6. **Close PR #23 as superseded** with a comment linking this CR's PR. Do not
   merge it.
7. **Verify the release**: `release-please` should open/update a release PR that
   bumps to 0.5.1; merging that produces the patch release users should take.

## 7. Constraints honoured by this phase

No branch push, no PR create/merge/close, no alert dismissal, no merge to
`main` was performed. All GitHub reads were via `gh api` / `gh pr view`. The
items above are recorded for the human to execute.
