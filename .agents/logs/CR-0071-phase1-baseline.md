<!--
@agents-index: CR-0071 Phase 1 baseline evidence — the two `make security` runs
on the unchanged tree, their finding counts, and the determination that the
instrument agrees with itself on a null change.
-->

# CR-0071 Phase 1 — Baseline the instruments

Baseline captured on the unchanged tree (branch
`dev/cr-0071-0072-dependency-currency`) so that any later stage's effect on
`make security` is measured against a validated instrument. A gate that
disagrees with itself on a null change cannot judge a fix, so `make security`
was run twice with nothing changed between runs.

Raw evidence (gitignored, `*.log`): `.agents/logs/CR-0071-security-before-run1.log`
and `.agents/logs/CR-0071-security-before-run2.log`.

## Instrument validation: the two runs agree

The two runs are **identical**. Byte-for-byte identical normalized output, the
same set of 13 `GO-*` finding IDs, the same summary line, and the same exit
status. The instrument is deterministic on a null change and is therefore
usable to judge the staged bumps.

| Property | Run 1 | Run 2 | Agree |
|----------|-------|-------|-------|
| Reachable findings (`govulncheck`) | 13 | 13 | yes |
| Distinct `GO-*` IDs | 13 | 13 | yes (identical set) |
| Normalized full output | — | — | yes (diff empty) |
| `make security` exit | 2 | 2 | yes |
| Failing step | `govulncheck` (Error 3) | `govulncheck` (Error 3) | yes |

## Exit-code capture

Exit codes were captured without a pipe masking them (output redirected to a
file, `${PIPESTATUS[0]:-$?}` read immediately). `make security` exits **2**
(GNU make's own error exit); the underlying `govulncheck` recipe fails with
`Error 3`, which is `govulncheck`'s exit code for "vulnerabilities found in
called code". This failure is **by design at baseline** — it is the state
CR-0071 exists to fix and was captured faithfully, not repaired here.

## Finding counts

### `make security` step outcomes

`security: verify govulncheck vuln-scan license-check`. Because make runs
prerequisites in order and halts on the first failure:

* `verify` (`go mod verify`) — **passed**.
* `govulncheck ./...` — **failed** (Error 3), 13 reachable findings.
* `vuln-scan` (`grype`) — **did not run** (make halted at `govulncheck`).
* `license-check` (`grant`) — **did not run** (make halted at `govulncheck`).

`grype` and `grant` being unrun at baseline is exactly the masking the CR
predicts (Impact Assessment, Risk 3): they will first be exercisable once
`govulncheck` passes, in Phase 6.

### `govulncheck` findings

* **13 reachable** (called-code) vulnerabilities, from 4 modules and the Go
  standard library.
* Also reported but **not called** (not counted as reachable, not gating):
  5 vulnerabilities in imported packages, 21 in required modules.

Reachable finding attribution (verified from the run output, not from CR
prose):

| # | GO ID | Attributed to | Fixed in |
|---|-------|---------------|----------|
| 1 | GO-2026-6061 | `google.golang.org/grpc` | grpc |
| 2 | GO-2026-5970 | `golang.org/x/text` | x/text |
| 3 | GO-2026-5856 | stdlib `crypto/tls` | go1.25.12 |
| 4 | GO-2026-5224 | `github.com/microsoft/kiota-http-go` | kiota-http-go |
| 5 | GO-2026-5039 | stdlib `net/textproto` | go1.25.11 |
| 6 | GO-2026-5037 | stdlib `crypto/x509` | go1.25.11 |
| 7 | GO-2026-5026 | `golang.org/x/net` | x/net |
| 8 | GO-2026-4986 | stdlib `net/mail` | go1.25.10 |
| 9 | GO-2026-4982 | stdlib `html/template` | go1.25.10 |
| 10 | GO-2026-4980 | stdlib `html/template` | go1.25.10 |
| 11 | GO-2026-4977 | stdlib `net/mail` | go1.25.10 |
| 12 | GO-2026-4971 | stdlib `net` | go1.25.10 |
| 13 | GO-2026-4918 | `golang.org/x/net` **and** stdlib `net/http` | x/net v0.53.0 / go1.25.10 |

## Confirmation of the CR reviewer's corrected numbers

The review summary flagged that earlier prose miscounted; the corrected numbers
were re-verified here against live output and **all match**. No discrepancy
found:

* **13 reachable findings** — confirmed.
* **Nine involve the standard library** — confirmed (9 findings reference a
  standard-library package: #3, #5, #6, #8, #9, #10, #11, #12, and #13).
* **Eight fixed by the toolchain alone** — confirmed. Of the nine
  stdlib-referencing findings, eight are pure-stdlib (#3, #5, #6, #8–#12) and
  clear with the toolchain floor; the ninth, **GO-2026-4918** (#13), also
  carries a `golang.org/x/net` call path and so clears in Stage 1, not Stage 0.
* **Toolchain floor `go1.25.12`** — confirmed as the highest required stdlib
  fix (`crypto/tls`), matching Stage 0's "1.25.12 or later".
* **Four non-stdlib modules** — confirmed: `google.golang.org/grpc`,
  `golang.org/x/text`, `golang.org/x/net`, `github.com/microsoft/kiota-http-go`.
  `golang.org/x/text` (GO-2026-5970) is reachable but is not a Dependabot
  alert, as the CR notes.

## Method

```bash
make security > .agents/logs/CR-0071-security-before-run1.log 2>&1
echo "RUN1_EXIT=${PIPESTATUS[0]:-$?}"   # 2
make security > .agents/logs/CR-0071-security-before-run2.log 2>&1
echo "RUN2_EXIT=${PIPESTATUS[0]:-$?}"   # 2
# GO-ID sets and normalized outputs diffed: both empty (identical).
```
