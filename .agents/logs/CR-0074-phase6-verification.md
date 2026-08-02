# CR-0074 Phase 6 — Verification Evidence

- **Date (UTC):** 2026-08-01T10:50:44Z
- **Branch:** docs/cr-0073 (branch name historical; CR is CR-0074)
- **Tree at start of phase:** commit `1035575` (`checkpoint(CR-0074): phase 5: Documentation`)
- **Binary under test:** `./outlook-local-mcp`, built via `make build` from `1035575`
- **Scope:** no source changes; this phase produces verification evidence only.

Raw command output is kept next to this file as `*.raw.log` (gitignored via
`*.log`). This markdown is the committed record and quotes the load-bearing
lines verbatim.

## Gate results

| Gate | Command | Exit code |
|------|---------|-----------|
| Full CI pipeline | `make ci` | **0** |
| Security pipeline | `make security` | **0** |

`make ci` runs `docs-bundle build vet fmt-check tidy lint test goreleaser-check
mcpb-validate` (test includes `-race` and coverage).

`make security` runs the build, `syft` SBOM generation, `grype --fail-on high`,
and `grant` license compliance. It exited 0. `grype` reported one advisory,
`GO-2026-5932` against `golang.org/x/crypto v0.54.0`, at severity **Unknown** —
below the `--fail-on high` gate, so it does not fail the run. License check:
`185 allowed, No denied packages found`.

Note the `grype` instrument caveat recorded in Phase 5 (`docs/reference/security.md`):
the release build strips symbols (`-s -w`), so this scan (run against the
locally-built, symbol-bearing binary via `syft`/SBOM) matches at module
granularity in the same way. The verdict is coarser than a reachability-precise
scan; this is documented, not a new defect.

## Live `get_docs` verification through the real server

The five previously-unreachable sections were exercised **through the real
server**, not through a unit test. The binary was driven over its MCP stdio
transport with a JSON-RPC `initialize` + `notifications/initialized` +
`tools/call` sequence (tool `system`, `operation="get_docs"`, `output="raw"`
so the exact returned bytes could be inspected). This satisfies FR-4, AC-1,
AC-2, and AC-4, and the FR-3 negative case (AC-3).

Each content case asserts three things: the call is not an error, the returned
text is non-empty, and **no returned heading line** (`#`-prefixed) contains
`{#` or `}` (FR-4/AC-4 concern the heading text; body content such as JSON and
tool-call code snippets legitimately contains braces and is not in scope).

### Per-section results (all PASS)

| # | Documented anchor | slug | isError | bytes | anchor tag in any heading | returned heading |
|---|-------------------|------|---------|-------|---------------------------|------------------|
| 1 | `before-you-file-an-issue` | troubleshooting | false | 635 | no | `## Before you file an issue` |
| 2 | `auto-default-account` | troubleshooting | false | 2489 | no | `## Auto-default account` |
| 3 | `container-no-keychain` | troubleshooting | false | 1377 | no | `## Container has no keychain access` |
| 4 | `container-deployment` | quickstart | false | 1766 | no | `## Container deployment` |
| 5 | `container-runtime` | concepts | false | 2624 | no | `## Container runtime` |

Opening lines returned (evidence each section returns *its own* content):

1. `## Before you file an issue\n\nCall \`system.about\` before opening a GitHub issue. It captures build identity and host envi…`
2. `## Auto-default account\n\nThe implicit \`default\` account registration is conditional on \`accounts.json\` contents, and \`ac…`
3. `## Container has no keychain access\n\n**Symptom:** The server logs a warning such as \`keychain unavailable, falling back …`
4. `## Container deployment\n\nThe server is available as an OCI image at \`ghcr.io/desek/outlook-local-mcp\`. No Go toolchain i…`
5. `## Container runtime\n\nThe server ships as an OCI container image at \`ghcr.io/desek/outlook-local-mcp\`. The image uses st…`

### No cross-section bleed (AC-1)

Each of the five returns exactly one H2 heading, its own, and its body begins
with content specific to that section (see opening lines above). The returned
headings are five distinct strings, one per requested anchor, with no adjacent
section's heading appearing in any result.

### Heading-tag check detail (FR-4 / AC-4)

Three of the five sections do contain `}` **in body content only** — verified
by classifying every brace-bearing line:

- `troubleshooting/before-you-file-an-issue`: `[body] {tool: "system", args: {operation: "about", output: "summary"}}`
- `troubleshooting/container-no-keychain`: `[body] {tool: "system", args: {operation: "about", output: "summary"}}`
- `quickstart/container-deployment`: `[body]` several JSON closing-brace lines (`}`, `  }`, …)

**Zero heading lines** in any of the five sections contain `{#` or `}`. The
`{#custom-id}` anchor tags are stripped from the H2 heading before it reaches
the caller, as FR-4 requires.

## FR-3 negative case (AC-3) — PASS

A heading carrying an explicit anchor must **not** resolve under its
text-derived form. The heading `## Container has no keychain access {#container-no-keychain}`
was requested by its text-derived anchor `container-has-no-keychain-access`:

```
slug=troubleshooting section=container-has-no-keychain-access
isError=True
text: get_docs: section "container-has-no-keychain-access" not found in "troubleshooting"
```

Only the explicit anchor `container-no-keychain` resolves (case 3 above); the
derived form returns "section not found", as FR-3 mandates.

## `make crud-test` — deliberately not run

Per the phase instructions and CR-0074 Scope Boundaries, `make crud-test` was
**not** run in this phase. It mutates the user's real mailbox, needs live
Microsoft 365 credentials, and was already run against 0.5.1 earlier in the
session (bench row `2026-08-01T11-55-17`, the run that surfaced this CR). It is
documented as a required **manual** release gate (`docs/reference/release.md`),
not a per-phase check. The live-server verification above substitutes the
"five sections resolve through the real server" evidence that the Phase 6
description attached to a crud-test run, without the destructive side effects.

## Summary

All Phase 6 gates pass:

- `make ci` = 0, `make security` = 0.
- All five explicit-anchor sections resolve live through the real server with
  their own content and no cross-section bleed (FR-5, AC-1, AC-2).
- No returned heading contains `{#` or `}` (FR-4, AC-4).
- The text-derived form of an explicit-anchor heading returns "section not
  found" (FR-3, AC-3).
