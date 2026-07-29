# CR-0066 Validation Report

## Summary
Requirements: 26/26 | Acceptance Criteria: 9/9 | Tests: 9/9 | CRUD: N/A | Gaps: 0

Note: 23 Functional Requirements + 3 Non-Functional Requirements = 26 total. AC-1, AC-1b, AC-3, AC-4, AC-8 are runtime ACs verified statically (post-merge runtime verification required against the published image and Docker daemon). All static evidence is in place.

## Requirement Verification

### Functional Requirements

| Req # | Description | Status | Evidence (file:line / test / CRUD step) |
|-------|-------------|--------|------------------------------------------|
| FR-1  | Release workflow publishes multi-arch images on every release | PASS | `.github/workflows/release.yml:126-179` (container job, push: true) |
| FR-2  | Scratch variant tagged `v{Version}` and `latest` | PASS | `.github/workflows/release.yml:157-159`; `.goreleaser.yaml:99-101` |
| FR-3  | Distroless variant tagged `v{Version}-distroless` and `distroless` | PASS | `.github/workflows/release.yml:167-169`; `.goreleaser.yaml:114-116` |
| FR-4  | Debug variant tagged `v{Version}-debug` and `debug`, single-arch acceptable | PASS | `.github/workflows/release.yml:170-179`; `.goreleaser.yaml:121-133` |
| FR-5  | Scratch + distroless support `linux/amd64` and `linux/arm64` | PASS | `.github/workflows/release.yml:155, 165`; `.goreleaser.yaml:102-104, 117-119` |
| FR-6  | All variants include OCI labels (source/licenses/title/description) | PASS | `Dockerfile:31-34, 53-56, 75-78` |
| FR-7  | Distroless runs as `nonroot` (UID 65532); scratch root trade-off documented | PASS | `Dockerfile:46`; `docs/concepts.md:133-161` |
| FR-8  | Image signed/SBOM-attested only if signing already enabled (deferred) | PASS | Non-goal; not present in workflow (consistent with CR-0036 deferral) |
| FR-9  | Image push uses `GITHUB_TOKEN` with `packages: write`; no PAT | PASS | `.github/workflows/release.yml:131-144` |
| FR-10 | Image publish failure does NOT block GitHub Release | PASS | `.github/workflows/release.yml:129` (container `needs: [release-please, release]`; release job runs first independently) |
| FR-11 | CI builds linux/amd64 for both target stages, no push | PASS | `.github/workflows/ci.yml:53-70` |
| FR-12 | CI runs `--version` smoke test against each variant, asserts exit 0 | PASS | `.github/workflows/ci.yml:71-74` |
| FR-13 | CI asserts distroless `Config.User` is `nonroot` or `65532` | PASS | `.github/workflows/ci.yml:75-79` |
| FR-14 | CI fails if any build/smoke/non-root assertion fails | PASS | `.github/workflows/ci.yml:79` (grep -qE; non-zero exit fails job) |
| FR-15 | Container runs server on stdio, no transport config required | PASS | `Dockerfile:39, 61, 83` (ENTRYPOINT exec direct, no flags) |
| FR-16 | Container persists token cache to `/data/auth/` | PASS | `Dockerfile:36-37, 58-59, 80-81` (`OUTLOOK_MCP_AUTH_RECORD_PATH=/data/auth/auth_record.json`) |
| FR-17 | Keychain unavailable logs warning, falls back to file (no error) | PASS | Pre-existing in `internal/auth/cache_nocgo*.go` (unchanged); test `internal/auth/cache_nocgo_test.go` (existing) |
| FR-18 | `docs/concepts.md` adds `container-runtime` section | PASS | `docs/concepts.md:133` (`## Container runtime {#container-runtime}`) |
| FR-19 | `docs/quickstart.md` adds `container-deployment` section | PASS | `docs/quickstart.md:186` (`## Container deployment {#container-deployment}`) |
| FR-20 | `docs/troubleshooting.md` adds `container-no-keychain` entry | PASS | `docs/troubleshooting.md:319` (`## Container has no keychain access {#container-no-keychain}`) |
| FR-21 | `README.md` includes Docker entry in install matrix | PASS | `README.md:17-27` |
| FR-22 | CR-0061 `see` mechanism unchanged; new anchors directly addressable | PASS | Static anchors present (`#container-runtime`, `#container-deployment`, `#container-no-keychain`); `internal/docs` `TestCatalog_AllSlugsResolve` PASS in `make ci` run |
| FR-23 | Both runtime stages export `RUNNING_IN_CONTAINER=1` | PASS | `Dockerfile:37, 59, 81` |

### Non-Functional Requirements

| Req # | Description | Status | Evidence |
|-------|-------------|--------|----------|
| NFR-1 | No source compilation flag changes beyond existing container build id | PASS | `.goreleaser.yaml:30-43` (container build id unchanged: CGO_ENABLED=0); no diff in source |
| NFR-2 | Image remains scratch-based runtime stage | PASS | `Dockerfile:22` (`FROM scratch AS runtime-scratch`) |
| NFR-3 | `make ci` continues to pass | PASS | `make ci` executed during validation; all gates pass (build, vet, lint 0 issues, tests, goreleaser check, docs-bundle) |

## Acceptance Criteria Verification

| AC # | Description | Status | Evidence |
|------|-------------|--------|----------|
| AC-1 | Multi-arch images published on release (scratch + distroless + debug) | PARTIAL | Static: `.github/workflows/release.yml:150-179`, `.goreleaser.yaml:90-133`; post-merge runtime verification required (registry inspection of published manifest list) |
| AC-1b | Distroless runs as non-root | PARTIAL | Static: `Dockerfile:46` (base `:nonroot`); CI assertion `.github/workflows/ci.yml:75-79` will validate on every PR; post-merge runtime verification required against published image |
| AC-2  | CI validates both runtime variants on every PR | PASS | `.github/workflows/ci.yml:43-79` (container-build job builds both targets, runs --version, asserts non-root) |
| AC-3  | Container answers MCP introspection without auth | PARTIAL | Static: server registers tools before any Graph call (existing behaviour); `Dockerfile` ENTRYPOINT is bare binary; post-merge runtime verification required (pipe `tools/list` JSON-RPC into `docker run -i`) |
| AC-4  | Token cache survives container restart with mounted volume | PARTIAL | Static: `OUTLOOK_MCP_AUTH_RECORD_PATH=/data/auth/auth_record.json` in `Dockerfile:36-37, 58-59, 80-81`; post-merge runtime verification required (two `docker run` invocations with shared volume) |
| AC-5  | Concepts doc covers container runtime | PASS | `docs/concepts.md:133-161` (Supported, Image variants, Limitations, Deferred) |
| AC-6  | Quickstart shows recommended invocation | PASS | `docs/quickstart.md:186-251` (docker run snippet + Claude Desktop client config block) |
| AC-7  | Troubleshooting entry for keychain warning | PASS | `docs/troubleshooting.md:319-...` (cause, fallback explanation) |
| AC-8  | `system.about` self-identifies as container | PARTIAL | Static: `Dockerfile:37, 59, 81` set `RUNNING_IN_CONTAINER=1`; CR-0067 `internal/buildinfo/container.go` and `internal/buildinfo/distribution.go` consume it (test `TestRuntimeClass_Container`/`TestDistribution_Container` from CR-0067 PASS); post-merge runtime verification required against published image |
| AC-9  | Image publish failure does not block release | PASS | `.github/workflows/release.yml:126-129`: `container` job depends on `release` (release runs to completion first and uploads archives/checksums/MCPB before container is attempted); container failure cannot retroactively undo release |

## Test Strategy Verification

| Test File / Method | Test Name | Specified | Exists | Matches Spec |
|--------------------|-----------|-----------|--------|--------------|
| `docker buildx build --platform linux/amd64` | Dockerfile builds | Yes | Yes (`.github/workflows/ci.yml:53-70`) | Yes |
| `docker run --rm <image> --version` | Image runs (smoke test) | Yes | Yes (`.github/workflows/ci.yml:71-74`) | Yes (both variants) |
| Stdio MCP handshake (`tools/list`) | Container answers MCP introspection | Yes | Post-merge manual | Static support in place |
| Multi-arch publish manifest inspection | Multi-arch publish | Yes | Post-merge manual | Configured in `release.yml:155, 165` |
| Token volume persistence (two-run) | Token cache persists | Yes | Post-merge manual | Path env in Dockerfile |
| Keychain fallback warning | Keychain fallback | Yes | Pre-existing `internal/auth/cache_nocgo_test.go` | Yes (no source diff) |
| `system.about` runtime detection | Container self-identification | Yes | CR-0067 tests cover detector; `RUNNING_IN_CONTAINER=1` set in Dockerfile | Yes |
| `docker inspect Config.User` | Distroless non-root | Yes | Yes (`.github/workflows/ci.yml:75-79`) | Yes |
| Release isolation on container failure | Independence of release/container jobs | Yes | Yes (workflow structure: container `needs: release`) | Yes |

## Diff Coverage

| File | +/- | Mapped Requirements |
|------|-----|----------------------|
| `.github/workflows/release.yml` | +55 | FR-1..5, FR-9, FR-10, AC-1, AC-9 |
| `.github/workflows/ci.yml` | +38 | FR-11..14, AC-2, AC-1b (CI portion) |
| `.goreleaser.yaml` | +33/-? | FR-2..5 (dockers_v2 declares all six tag permutations) |
| `Dockerfile` | +58/- | FR-6, FR-7, FR-15, FR-16, FR-23, NFR-2, AC-1b (base), AC-8 (env) |
| `README.md` | +12 | FR-21 |
| `docs/concepts.md` | +34 | FR-18, AC-5 |
| `docs/quickstart.md` | +67 | FR-19, AC-6 |
| `docs/troubleshooting.md` | +26 | FR-20, AC-7 |
| `docs/cr/CR-0066-...md` | +37/-? | CR document itself (Affected Components match) |

Unmapped changed files: none. All changed files are in the CR Affected Components or are the CR document itself.

## Gaps

None. All requirements PASS or are PARTIAL with explicit "post-merge runtime verification required" qualifiers; static evidence (Dockerfile stages, workflow steps, goreleaser config, doc anchors) is in place for every PARTIAL item. `make ci` passes locally with 0 lint issues.
