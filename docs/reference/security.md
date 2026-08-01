# Security: dependency vulnerability instruments and triage

Contributor-facing. Not embedded in the binary and not published to the site.

This repository is a credential-holding MCP server, so its dependency posture is
the surface a prospective user actually inspects. Three instruments report on that
posture, they disagree with each other on the same tree by design, and the failure
mode CR-0071 corrected was not having too few instruments but having three and
reading none. This document records what each measures, why their counts differ,
how to triage a new alert, and the two rules and one ceiling learned the expensive
way while restoring the gate.

## The three instruments and why their counts differ

The three instruments answer three different questions, so a finding count from
one is not comparable to a finding count from another.

* **`govulncheck`** (run by `make security`, `govulncheck ./...`) answers *"can an
  attacker reach this from our code?"* It performs reachability analysis: it
  reports a vulnerability only when there is a call path from this module's code
  into the vulnerable symbol. It is the right gate for deciding whether to ship,
  and a `govulncheck` failure is always worth acting on. It says nothing about a
  vulnerable module that sits in `go.sum` but is never called, and its verdict is
  invisible to anyone browsing the repository. On the CR-0071 tree it reported 13
  reachable findings before remediation and 0 after.

* **Dependabot** (GitHub, no local command) answers *"is anything in the module
  graph known-vulnerable?"* It matches advisories against the versions in
  `go.mod`/`go.sum` with no reachability analysis, so it over-reports by design —
  thirteen of its sixteen CR-0071 alerts were `golang.org/x/crypto/ssh`
  advisories this server cannot reach. Its value is that it is continuous,
  external, and visible on the security tab. It also *under*-reports relative to
  `govulncheck`: it never filed an alert for `golang.org/x/text`, which
  `govulncheck` flagged as reachable. An alert-driven remediation would have
  missed it and left the gate red.

* **`grype`** (run by `make vuln-scan` as `grype --fail-on high` against the
  built SBOM) answers the same question as Dependabot — *"is anything
  known-vulnerable?"* — but against the built SBOM rather than the module graph,
  so it catches what actually got linked rather than what got required, and like
  Dependabot it matches at module level with no reachability analysis.

The counts differ because reachability (govulncheck) is a strict subset of
module-presence (Dependabot, grype), the two module-presence tools scan different
inputs (graph vs built SBOM), and Dependabot's advisory feed is neither a superset
nor a subset of the Go vulnerability database `govulncheck` uses. Keep all three:
each covers a gap the other two leave open.

## Triaging a new alert

When a new Dependabot alert or a red `Security` check appears, in order:

1. **Reproduce locally first.** Run `make security` and read which of its four
   steps failed (`go mod verify`, `govulncheck ./...`, `make vuln-scan`,
   `make license-check`). A finding you cannot reproduce with `make security` is
   not yet understood — do not act on the GitHub summary alone.
2. **Remediate by version bump, by default.** The default remediation is to raise
   the dependency to a fixed version and let the graph go current, not to argue
   the finding away. A current graph has no open advisories by construction, which
   is cheaper over time than re-litigating reachability on every future advisory.
   Read the fixed version using Rule 1 below.
3. **Dismiss only with a written reachability argument.** An alert may be
   dismissed instead of fixed only when there is a written argument, recorded here
   or in the PR, that the vulnerable code is unreachable, and that argument must
   name the command that establishes it (see the `x/crypto/ssh` assessment below)
   rather than asserting unreachability. A dismissal with no reproducible command
   behind it is folklore, not an assessment.

## Standing assessment: `golang.org/x/crypto/ssh` advisories are unreachable

All thirteen `golang.org/x/crypto` Dependabot alerts on the CR-0071 tree
(GHSA-hrxh-6v49-42gf, GHSA-jppx-rxg9-jmrx, GHSA-5cgq-3rg8-m6cv,
GHSA-89gr-r52h-f8rx, GHSA-rm3j-f69w-wqmq, GHSA-vgwf-h737-ff37,
GHSA-f5wc-c3c7-36mc, GHSA-x527-x647-q7gg, GHSA-w879-237q-wc7r,
GHSA-q4h4-gmj2-qvw2, GHSA-9m57-25v3-79x9, GHSA-45gg-vh54-h5m9,
GHSA-qpw4-5x99-6vjp) concern the `golang.org/x/crypto/ssh` package tree: agent
key constraints, host-key callbacks, channel handling, FIDO/U2F presence checks.
This server opens no SSH connection, so none is reachable.

The command that establishes it, reproducibly:

```bash
govulncheck ./...
```

`govulncheck` reports 0 reachable findings on the remediated tree; anything in the
`x/crypto/ssh` tree that were reachable would appear in its output with a call
path. To confirm no code imports the package at all:

```bash
go list -deps ./... | grep 'golang.org/x/crypto/ssh'   # no output: not in the build
```

These advisories were nonetheless remediated by bumping `golang.org/x/crypto`
rather than dismissed, because the bump was free and the alternative was
re-arguing reachability on every future advisory in that package. Dismissal with
the argument above is the fallback only if a future bump proves troublesome.

## Rule 1: read the patched version from the entry matching the ecosystem

An advisory can list several ecosystems, and their patched versions are unrelated
numbers. Read the patched version from the entry whose ecosystem is **`go`**, and
check **every** affected range in that entry, not just the first one — a single
advisory often patches different major/minor streams at different versions.

Concrete case from CR-0071: **GHSA-7j59-v9qr-6fq9** lists Maven first, patched at
`1.9.1`, while its `go` entry patches `github.com/microsoft/kiota-http-go` at
`1.5.5`. `kiota-http-go` has no `1.9.1` release and never will; remediating
against the headline Maven number produces an unreachable target and an alert that
never closes. Always locate the `go` entry, then check each of its ranges.

## Rule 2: a user-facing dependency PR MUST be `fix(deps):`, squash-merged

`release-please` is configured `release-type: go` with
`bump-minor-pre-major: true` and `bump-patch-for-minor-pre-major: false`. Under
that config a `chore(deps):` commit — which is exactly what Dependabot generates
by default — produces **no release at all**. The fix lands on `main` and reaches
no user of a released binary.

Therefore a dependency pull request that should reach users **MUST** be
squash-merged with a `fix(deps):` title so `release-please` opens a patch release.
This is not hypothetical: PR #23 was titled `chore(deps): bump ... kiota-http-go
...` and, had it merged, would have shipped its security fix to nobody. Retitle
Dependabot's PRs to `fix(deps):` at merge time whenever the change is one users
should take.

## Ceiling: GO-2026-5932, `x/crypto/openpgp`, no fix available

`make vuln-scan` reports one finding that has no remediation. Per the project's
Measurement and Verification standard, the ceiling is published here with the
measurement chain that established it and what would have to change to move it,
rather than silenced by relaxing `--fail-on`.

* **Finding:** `GO-2026-5932` in `golang.org/x/crypto` v0.54.0 — the
  `golang.org/x/crypto/openpgp` package is unmaintained, unsafe by design, and
  has known security issues.
* **Reachability:** not reachable. `govulncheck ./...` reports it under "modules
  you require, but your code doesn't appear to call" and 0 reachable findings;
  this server imports no `openpgp` code.
* **grype disposition:** reported at module level (grype matches by module, not
  reachability) at severity **Unknown**, so `grype --fail-on high` exits 0 and the
  gate is not relaxed to accommodate it.
* **Fixed in: N/A.** The tree is already on the latest `golang.org/x/crypto`
  (`go list -m -u golang.org/x/crypto` shows no available update). The advisory
  has no fixed version and never will: the package is intentionally deprecated
  upstream.
* **What would move it:** either upstream shipping a fixed `x/crypto` release (not
  expected — the package is deprecated by design) or the transitive requirement on
  `x/crypto` dropping the `openpgp` subtree from the build graph. Neither is
  actionable within a dependency-currency pass.

Measurement chain: the finding, both `vuln-scan` runs (byte-identical on the
unchanged tree, confirming the instrument agrees with itself), the grype DB
freshness check, and the per-stage bisectability are recorded in
`.agents/logs/CR-0071-phase6-verification.md`.
