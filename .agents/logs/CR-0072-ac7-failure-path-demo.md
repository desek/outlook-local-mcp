# CR-0072 AC-7 Failure-Path Demonstration

/ Purpose: exercise the NEGATIVE behaviour of the production audit gate
/ (FR-10 / AC-6 / AC-7): confirm `pnpm --dir site audit --prod
/ --audit-level=moderate` exits non-zero when a moderate-or-higher
/ vulnerable package is present in `site/package.json` `dependencies`.
/ Performed non-destructively and fully reverted; nothing committed.

Date: 2026-07-31
Branch: dev/cr-0071-0072-dependency-currency
Operator: CR-0072 gap-fixer

## Why this exists

The validator recorded AC-7 as PARTIAL: the gate mechanism was present and
correct (audit `--prod`, moderate floor, before build, no `continue-on-error`)
but its failure path was asserted by construction and never exercised. This log
closes that gap behaviourally rather than by weakening the AC.

## Method (non-destructive)

1. Backed up `site/package.json` and `site/pnpm-lock.yaml` to the session
   scratchpad, recording their SHA256.
2. Injected a known-vulnerable runtime dependency into `dependencies`:
   `"lodash": "4.17.11"` (multiple advisories: critical/high/moderate).
3. `pnpm --dir site install --lockfile-only` so the lockfile resolves the
   injected package (node_modules untouched; no network fetch of the package
   itself was needed to audit).
4. Ran the EXACT gate command from `site.yml`:
   `pnpm --dir site audit --prod --audit-level=moderate`.
5. Restored both files from backup, verified SHA256 identity and a clean
   `git status`, and re-ran the gate to confirm it returns to green.

## Result — the gate fails as required

Command: `pnpm --dir site audit --prod --audit-level=moderate`
Injected: `lodash@4.17.11` in `dependencies`.

```
7 vulnerabilities found
Severity: 3 moderate | 3 high | 1 critical
EXIT=1
```

Advisories surfaced against `lodash` (path `.>lodash`, i.e. a direct production
dependency): GHSA-jf85-cpcp-j695 (critical, prototype pollution),
GHSA-35jh-r3h4-6jhm (high, command injection), GHSA-p6mc-m468-83gw (high),
GHSA-r5fr-rjxr-66jc (high), GHSA-29mw-wpgm-hmr9 (moderate, ReDoS),
GHSA-f23m-r3pf-42rh (moderate), GHSA-xxjr-mmjv-4gpg (moderate).

A non-zero exit at this step fails the `site` workflow job (the step carries no
`continue-on-error`), and the step is ordered before the build (FR-11), so the
build never runs. This is exactly AC-7's required behaviour: a runtime
dependency becoming vulnerable fails the job at the audit step, before the
build.

## Control — no false positive

Before injection, the same command on the unmodified tree reported
`No known vulnerabilities found` / `EXIT=0`. After full revert it again reported
`No known vulnerabilities found` / `EXIT=0`. The gate is green on the shipped
tree and only red because of the injected package.

## Revert integrity

- `site/package.json` restored, SHA256 unchanged:
  `59031bec3f5ff503cdf7a85dfd76bf34aa1602ad9b69c1da76111467ef892fc8`
- `site/pnpm-lock.yaml` restored, SHA256 unchanged:
  `6c07903205fb1375c680d07d5a1ed7dba838e47829fb6946270198422e6247ca`
- `git status --porcelain` empty after revert.
- The residual `lodash` strings in `site/pnpm-lock.yaml` are the pre-existing
  transitive `lodash@4.18.1` / `lodash-es` under the `@lhci/cli` dev tree, which
  `--prod` does not audit; the identical lockfile hash confirms no injected
  residue remains.

No vulnerable dependency was committed at any point.
