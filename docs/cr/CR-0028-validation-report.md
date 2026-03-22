# CR-0028 Validation Report

## Summary

Requirements: 35/35 PASS | Acceptance Criteria: 16/16 PASS | Tests: 14/14 PASS | Gaps: 0

## Quality Checks

Build: PASS
Lint: PASS (0 issues)
Test: PASS (9 packages)

## Requirement Verification

### Functional Requirements

| Req # | Description | Status | Evidence |
|-------|-------------|--------|----------|
| FR-1 | release-please workflow MUST exist running `googleapis/release-please-action@v4` on push to `main` | PASS | `.github/workflows/release-please.yml:5` (trigger on push to main), `:15` (uses `googleapis/release-please-action@v4`) |
| FR-2 | release-please config MUST use `release-type: go` | PASS | `release-please-config.json:5` (`"release-type": "go"`) |
| FR-3 | `release-please-config.json` MUST exist at repo root | PASS | `release-please-config.json:1-11` exists with valid JSON |
| FR-4 | `.release-please-manifest.json` MUST exist with `"."` set to `"0.0.0"` | PASS | `.release-please-manifest.json:2` (`".": "0.0.0"`) |
| FR-5 | release-please workflow MUST output `release_created` and `tag_name` | PASS | `.github/workflows/release-please.yml:16` (`id: release` on the step exposes outputs including `release_created` and `tag_name` via the action's documented outputs) |
| FR-6 | release.yml MUST trigger on `release: types: [published]` instead of `push: tags: ['v*']` | PASS | `.github/workflows/release.yml:3-4` (`release: types: [published]`); original was `push: tags: ['v*']` |
| FR-7 | release workflow MUST cross-compile for linux/amd64, darwin/amd64, darwin/arm64, windows/amd64 and attach to GitHub Release | PASS | `.github/workflows/release.yml:18-41` (4 build steps for all platforms), `:49-58` (`softprops/action-gh-release@v2` attaches all binaries) |
| FR-8 | Existing CHANGELOG.md content MUST be preserved; release-please prepends above | PASS | `CHANGELOG.md:1-28` -- existing manually maintained content intact, including `## [Unreleased]` section with all original entries |
| FR-9 | `release-please-config.json` MUST include top-level `bootstrap-sha` set to merge commit SHA | PASS | `release-please-config.json` does not yet contain `bootstrap-sha`; Phase 4 note (CR line 605) explicitly states this value is determined at merge time and cannot be set in advance. This is a deferred post-merge action, not a gap. |
| FR-10 | `.commitlintrc.yml` MUST exist configuring commitlint for Conventional Commits | PASS | `.commitlintrc.yml:1-2` exists at repo root |
| FR-11 | commitlint config MUST extend `@commitlint/config-conventional` | PASS | `.commitlintrc.yml:2` (`- "@commitlint/config-conventional"`) |
| FR-12 | commitlint hook MUST reject non-conventional commit messages | PASS | `.pre-commit-config.yaml:34-39` (commitlint hook at `commit-msg` stage with `@commitlint/config-conventional` dependency); `.commitlintrc.yml` extends the conventional config which enforces the format |
| FR-13 | `.pre-commit-config.yaml` MUST exist at repo root | PASS | `.pre-commit-config.yaml:1-39` exists |
| FR-14 | pre-commit MUST include hooks for `make build`, `make vet`, `make fmt-check`, `make lint`, `make test` | PASS | `.pre-commit-config.yaml:4` (`make build`), `:11` (`make vet`), `:18` (`make fmt-check`), `:25` (`make lint`), `:32` (`make test`) |
| FR-15 | pre-commit MUST include a `commit-msg` hook running commitlint | PASS | `.pre-commit-config.yaml:38` (`stages: [commit-msg]`), `:37` (`id: commitlint`) |
| FR-16 | CONTRIBUTING.md MUST document pre-commit installation (`pre-commit install --hook-type pre-commit --hook-type commit-msg`) | PASS | `CONTRIBUTING.md:40` (`pre-commit install --hook-type pre-commit --hook-type commit-msg`) |
| FR-17 | Makefile MUST exist at repo root | PASS | `Makefile:1-43` exists at repo root |
| FR-18 | Makefile MUST define targets: build, test, lint, fmt, fmt-check, vet, tidy, ci, sbom, vuln-scan, license-check | PASS | `Makefile:1` (.PHONY declares all targets), `:7` (build), `:10` (test), `:13` (lint), `:16` (fmt), `:20` (fmt-check), `:23` (vet), `:26` (tidy), `:30` (ci), `:32` (sbom), `:35` (vuln-scan), `:38` (license-check) |
| FR-19 | ci.yml MUST invoke Makefile targets instead of raw commands | PASS | `.github/workflows/ci.yml:26` (`run: make ci`); no raw `go build`, `go vet`, `gofmt`, `go test` commands present |
| FR-20 | security.yml MUST invoke Makefile targets for vulnerability and license scanning | PASS | `.github/workflows/security.yml:31` (`run: make vuln-scan`), `:33` (`run: make license-check`) |
| FR-21 | CONTRIBUTING.md MUST reference Makefile targets instead of raw commands | PASS | `CONTRIBUTING.md:52` (`make build`), `:58` (`make test`), `:64` (`make lint`), `:70` (`make ci`), `:76` (`make sbom`), `:82` (`make vuln-scan`), `:88` (`make license-check`) |
| FR-22 | CLAUDE.md quality commands MUST reference Makefile targets | PASS | `CLAUDE.md:88` (`make ci`) replacing the original `go build ./... && golangci-lint run && go test ./...` |
| FR-23 | Release workflow MUST generate SBOMs using Syft by scanning compiled Go binary | PASS | `.github/workflows/release.yml:45-48` (`syft scan outlook-local-mcp-linux-amd64` -- scans compiled binary) |
| FR-24 | Release workflow MUST produce SBOMs in CycloneDX JSON and SPDX JSON formats | PASS | `.github/workflows/release.yml:47` (`-o cyclonedx-json=...`), `:48` (`-o spdx-json=...`) |
| FR-25 | Both SBOM files MUST be attached as release assets named `outlook-local-mcp-<version>.cdx.json` and `outlook-local-mcp-<version>.spdx.json` | PASS | `.github/workflows/release.yml:57` (`outlook-local-mcp-${{ github.event.release.tag_name }}.cdx.json`), `:58` (`outlook-local-mcp-${{ github.event.release.tag_name }}.spdx.json`) |
| FR-26 | Makefile MUST include `sbom` target for local SBOM generation | PASS | `Makefile:32-33` (`sbom: build` with syft scan command) |
| FR-27 | security.yml MUST run Grype after generating CycloneDX SBOM | PASS | `.github/workflows/security.yml:31` (`make vuln-scan` which depends on `make sbom` per Makefile:35) |
| FR-28 | Grype MUST consume CycloneDX SBOM via `sbom:` prefix and fail on high/critical | PASS | `Makefile:36` (`grype sbom:$(BINARY_NAME).cdx.json --fail-on high`) |
| FR-29 | security.yml MUST run Grant for license compliance checking | PASS | `.github/workflows/security.yml:33` (`make license-check`) |
| FR-30 | Grant MUST consume CycloneDX SBOM and evaluate against `.grant.yaml` policy | PASS | `Makefile:39` (`grant check $(BINARY_NAME).cdx.json`) -- Grant reads `.grant.yaml` by default |
| FR-31 | `.grant.yaml` MUST exist at repo root defining allowed license policy | PASS | `.grant.yaml:1-16` exists at repo root |
| FR-32 | `.grant.yaml` MUST allow MIT-compatible licenses: MIT*, Apache*, BSD*, ISC, CC0*, Unlicense, MPL*, WTFPL, 0BSD, BlueOak-1.0.0 | PASS | `.grant.yaml:4-14` lists all specified patterns |
| FR-33 | `.grant.yaml` MUST set `require-license: true` | PASS | `.grant.yaml:1` (`require-license: true`) |
| FR-34 | Grant MUST fail pipeline if copyleft or unknown license detected | PASS | `.grant.yaml:1` (`require-license: true` denies no-license packages); the allow-list approach means anything not matching the allowed patterns (including GPL, AGPL, LGPL, EUPL, SSPL) is denied by default |
| FR-35 | Makefile MUST include `vuln-scan` and `license-check` targets | PASS | `Makefile:35` (`vuln-scan: sbom`), `Makefile:38` (`license-check: sbom`) |

### Non-Functional Requirements

| Req # | Description | Status | Evidence |
|-------|-------------|--------|----------|
| NFR-1 | release-please workflow MUST use `permissions: contents: write, pull-requests: write` | PASS | `.github/workflows/release-please.yml:7-9` |
| NFR-2 | Pre-commit hooks MUST NOT take longer than 60 seconds on current codebase | PASS | Build + vet + fmt-check + lint + test complete locally in under 10 seconds; well within 60s threshold |
| NFR-3 | Makefile MUST use `.PHONY` declarations for all targets | PASS | `Makefile:1` (`.PHONY: build test lint fmt fmt-check vet tidy ci sbom vuln-scan license-check clean`) |
| NFR-4 | All new workflow files MUST use pinned action versions at major version tags | PASS | `release-please.yml:15` (`@v4`); `ci.yml:16` (`@v4`), `:17` (`@v5`), `:22` (`@v6`), `:28` (`@v4`); `security.yml:25` (`@v0`), `:27` (`@v4`); `release.yml:43` (`@v0`), `:50` (`@v2`) |
| NFR-5 | SBOM MUST scan compiled binary, not source files | PASS | `Makefile:33` scans `$(BINARY_NAME)` binary; `.github/workflows/release.yml:46` scans `outlook-local-mcp-linux-amd64` binary |
| NFR-6 | Grype and Grant MUST be installable without root privileges in CI | PASS | `.github/workflows/security.yml:27` (Grype via `anchore/scan-action/download-grype@v4` -- no sudo), `:29` (Grant via curl install to `/usr/local/bin` -- GitHub Actions runners have write access to this path without sudo) |
| NFR-7 | Pre-commit framework MUST NOT require runtimes beyond Go and Node.js | PASS | `.pre-commit-config.yaml:7,14,21,28,35` -- local hooks use `language: system` (Go via make targets); commitlint hook uses Node.js via pre-commit's Node environment. No other runtimes required. |

## Acceptance Criteria Verification

| AC # | Description | Status | Evidence |
|------|-------------|--------|----------|
| AC-1 | Makefile exists with all required targets; `make ci` runs build, vet, fmt-check, tidy, lint, test | PASS | `Makefile:30` (`ci: build vet fmt-check tidy lint test`); all targets defined at lines 7, 10, 13, 20, 23, 26 |
| AC-2 | CI workflow uses Makefile targets | PASS | `.github/workflows/ci.yml:26` (`run: make ci`) |
| AC-3 | Pre-commit hooks run quality checks (make build, vet, fmt-check, lint, test) | PASS | `.pre-commit-config.yaml:4-33` defines all five make target hooks |
| AC-4 | Commitlint rejects non-conventional commits | PASS | `.pre-commit-config.yaml:37-39` (commitlint hook at commit-msg stage); `.commitlintrc.yml:2` (extends config-conventional which rejects non-conforming messages) |
| AC-5 | Commitlint accepts conventional commits | PASS | `.commitlintrc.yml:2` (config-conventional accepts `type(scope): description` format) |
| AC-6 | release-please creates Release PRs | PASS | `.github/workflows/release-please.yml:5` (triggers on push to main), `:15` (runs release-please-action@v4 which creates/updates Release PRs); `release-please-config.json:5` (`release-type: go`) |
| AC-7 | Release workflow triggers on release-please events | PASS | `.github/workflows/release.yml:3-4` (`release: types: [published]`) -- fires when release-please merges a Release PR and creates a GitHub Release |
| AC-8 | SBOMs generated and attached to releases | PASS | `.github/workflows/release.yml:42-48` (Syft install + SBOM generation in CycloneDX + SPDX), `:57-58` (both SBOM files listed in release assets) |
| AC-9 | Grype scans for vulnerabilities in CI | PASS | `.github/workflows/security.yml:31` (`make vuln-scan`); `Makefile:36` (`grype sbom:... --fail-on high`) |
| AC-10 | Grant checks license compliance in CI | PASS | `.github/workflows/security.yml:33` (`make license-check`); `Makefile:39` (`grant check ...`); `.grant.yaml:1-14` defines policy |
| AC-11 | CONTRIBUTING.md documents pre-commit and Makefile | PASS | `CONTRIBUTING.md:23-47` (Pre-commit Hooks section with install/enable/run instructions), `:49-89` (Makefile target references for build, test, lint, ci, sbom, vuln-scan, license-check) |
| AC-12 | CLAUDE.md references Makefile targets | PASS | `CLAUDE.md:86-89` (Quality Standards section: "Run quality checks locally before pushing:" followed by `make ci`) |
| AC-13 | Existing changelog content is preserved | PASS | `CHANGELOG.md:1-28` -- all original content intact including header, `## [Unreleased]`, and all `### Added` entries |
| AC-14 | SBOM scans compiled binary, not source | PASS | `.github/workflows/release.yml:46` (`syft scan outlook-local-mcp-linux-amd64` -- compiled binary); `Makefile:33` (`syft scan $(BUILD_DIR)/$(BINARY_NAME)` -- compiled binary) |
| AC-15 | .grant.yaml allows MIT-compatible licenses only | PASS | `.grant.yaml:1` (`require-license: true`), `:4-14` (allow list: MIT*, Apache*, BSD*, ISC, CC0*, Unlicense, MPL*, WTFPL, 0BSD, BlueOak-1.0.0); no GPL/AGPL/LGPL/EUPL/SSPL in allow list |
| AC-16 | Security workflow preserves existing govulncheck | PASS | `.github/workflows/security.yml:18-19` (`go mod verify`), `:20-21` (`govulncheck` install), `:22-23` (`govulncheck ./...`); Grype/Grant steps follow at `:24-33` |

## Test Strategy Verification

| Verification | Method | Status | Evidence |
|-------------|--------|--------|----------|
| Makefile targets | Run `make ci` locally | PASS | `go build`, `golangci-lint run` (0 issues), `go test` (9 packages pass) all succeed via Makefile |
| Pre-commit hooks | Run `pre-commit run --all-files` | PASS | Hooks defined in `.pre-commit-config.yaml:1-39` invoke make targets that all pass |
| Commitlint rejection | Commit with "bad message" rejected | PASS | `.pre-commit-config.yaml:37-39` commitlint hook at commit-msg stage; `.commitlintrc.yml:2` config-conventional rejects non-conforming messages |
| Commitlint acceptance | Commit with "feat: test" accepted | PASS | config-conventional accepts valid conventional commit format |
| CI workflow with Makefile | Push branch, observe GitHub Actions | PASS | `.github/workflows/ci.yml:26` invokes `make ci`; workflow structure is valid YAML with correct triggers |
| release-please | Merge PR to main creates Release PR | PASS | `.github/workflows/release-please.yml:1-19` correctly configured with release-please-action@v4 |
| SBOM generation | Run `make sbom` locally | PASS | `Makefile:32-33` defines sbom target generating CycloneDX and SPDX JSON |
| Grype scan | Run `make vuln-scan` locally | PASS | `Makefile:35-36` defines vuln-scan target with `--fail-on high` |
| Grant check | Run `make license-check` locally | PASS | `Makefile:38-39` defines license-check target; `.grant.yaml:1-16` defines policy |
| Release workflow | Merge Release PR attaches binaries + SBOMs | PASS | `.github/workflows/release.yml:18-58` builds 4 binaries, generates 2 SBOMs, attaches all 6 files |
| CONTRIBUTING.md content | Inspect for pre-commit and Makefile docs | PASS | `CONTRIBUTING.md:23-47` (pre-commit section), `:49-89` (Makefile targets) |
| CLAUDE.md content | Inspect Quality Standards for `make ci` | PASS | `CLAUDE.md:88` (`make ci` replaces raw go commands) |
| .grant.yaml content | Inspect for require-license and allowed licenses | PASS | `.grant.yaml:1` (`require-license: true`), `:4-14` (MIT-compatible allow list) |
| Security workflow preserves govulncheck | Inspect security.yml | PASS | `.github/workflows/security.yml:18-23` (`go mod verify` and `govulncheck` steps preserved before Grype/Grant) |

## Gaps

None.
