---
id: "CR-0026"
status: "completed"
date: 2026-03-15
completed-date: 2026-03-15
requestor: "Project Lead"
stakeholders:
  - "Development Team"
  - "Project Lead"
priority: "high"
target-version: "0.7.0"
source-branch: dev/cc-swarm
source-commit: 1e7c28e
---

# Project Rename: outlook-calendar-mcp to outlook-local-mcp

## Change Summary

Rename the project from `outlook-calendar-mcp` to `outlook-local-mcp` across all code, configuration, documentation, and CI/CD artifacts. The new name reflects the project's planned expansion beyond calendar into mail, contacts, and other Microsoft 365 capabilities, while emphasizing the local-first, no-app-registration-required design. The MCP server name changes from `outlook-calendar` to `outlook-local`.

## Motivation and Background

The current name `outlook-calendar-mcp` is scoped to calendar functionality only. The project roadmap includes expanding to additional Microsoft 365 workloads (mail, contacts, etc.). Renaming now -- before publishing to a remote or package registry -- avoids breaking changes for downstream consumers later.

The name `outlook-local-mcp` was chosen because:

- **Broader scope**: Does not constrain the project to calendar-only functionality.
- **Differentiator**: "local" communicates the key value proposition -- no remote MCP server, no Azure AD app registration, runs entirely on the user's machine.
- **Convention-compliant**: Follows the dominant `<name>-mcp` naming pattern (40% of MCP servers per ecosystem analysis).

The repository is local-only with no remote configured and has never been published, making this a zero-impact rename.

## Change Drivers

* The project will expand beyond calendar to mail, contacts, and other Microsoft 365 workloads.
* The current name creates a false ceiling that would require a breaking rename later if done post-publish.
* The `<name>-mcp` naming convention is the ecosystem standard.
* The repository has no remote or downstream consumers, so this is the optimal time to rename.

## Scope of Changes

### Phase 1: Go Module and Binary

| Item | Current | New |
|---|---|---|
| Go module path | `github.com/desek/outlook-calendar-mcp` | `github.com/desek/outlook-local-mcp` |
| Binary directory | `cmd/outlook-calendar-mcp/` | `cmd/outlook-local-mcp/` |
| MCP server name | `outlook-calendar` | `outlook-local` |
| Default auth record path | `~/.outlook-calendar-mcp/auth_record.json` | `~/.outlook-local-mcp/auth_record.json` |
| Default cache name | `outlook-calendar-mcp` | `outlook-local-mcp` |

**Files affected:**

- `go.mod` -- module declaration
- `cmd/outlook-calendar-mcp/main.go` -- package location and `NewMCPServer` name argument
- `internal/config/config.go` -- default values for `AuthRecordPath` and `CacheName`
- All `internal/**/*.go` files -- import paths (76 occurrences across 37 source files)
- All `internal/**/*_test.go` files -- import paths and test server names

### Phase 2: Build and Deployment

| Item | Current | New |
|---|---|---|
| Docker image name | `outlook-calendar-mcp` | `outlook-local-mcp` |
| Docker binary path | `/usr/local/bin/outlook-calendar-mcp` | `/usr/local/bin/outlook-local-mcp` |
| Docker Compose service | `outlook-calendar-mcp` | `outlook-local-mcp` |
| CI release binaries | `outlook-calendar-mcp-{os}-{arch}` | `outlook-local-mcp-{os}-{arch}` |

**Files affected:**

- `Dockerfile` -- build output name, COPY target, ENTRYPOINT, LABEL, comments
- `docker-compose.yml` -- service name, comments
- `.github/workflows/release.yml` -- binary output names, release asset names
- `.dockerignore` -- binary name pattern
- `.gitignore` -- binary name pattern

### Phase 3: Kubernetes Manifests

**Files affected:**

- `docs/deploy/deployment.yaml` -- image name, container name, labels
- `docs/deploy/configmap.yaml` -- metadata name, labels, `OUTLOOK_MCP_CACHE_NAME` default value
- `docs/deploy/secret.yaml` -- metadata name, labels

### Phase 4: Documentation

**Files affected:**

- `README.md` -- title, project structure tree, build/install commands, all references
- `QUICKSTART.md` -- clone URL, build commands, binary paths, MCP client config examples
- `AGENTS.md` -- project structure tree, build/install commands
- `CLAUDE.md` -- project structure tree, build/install commands
- `docs/reference/outlook-calendar-mcp-spec.md` -- filename rename to `outlook-local-mcp-spec.md`, all internal references
- `docs/research/authentication-channels.md` -- reference

### Phase 5: Historical Documentation (CRs)

All prior CR documents contain references to the old name. These are updated for consistency:

- `docs/cr/CR-0001-project-foundation.md`
- `docs/cr/CR-0002-structured-logging.md`
- `docs/cr/CR-0003-authentication.md`
- `docs/cr/CR-0004-server-bootstrap.md`
- `docs/cr/CR-0005-error-handling.md`
- `docs/cr/CR-0006-read-only-tools.md`
- `docs/cr/CR-0007-search-free-busy.md`
- `docs/cr/CR-0008-create-update-tools.md`
- `docs/cr/CR-0009-delete-cancel-tools.md`
- `docs/cr/CR-0016-ci-cd-pipeline.md`
- `docs/cr/CR-0017-container-support.md`
- `docs/cr/CR-0018-observability-metrics.md`
- `docs/cr/CR-0019-security-hardening.md`
- `docs/cr/CR-0021-go-standard-project-layout.md`
- `docs/cr/CR-0022-improved-authentication-flow.md`
- `docs/cr/CR-0022-validation-report.md`
- `docs/cr/CR-0023-file-logging.md`
- `docs/cr/CR-0023-validation-report.md`
- `docs/cr/CR-0024-interactive-browser-auth-default.md`
- `docs/cr/CR-0024-validation-report.md`
- `docs/cr/CR-0025-multi-account-elicitation.md`

### Phase 6: Repository Directory (Manual)

The filesystem directory `outlook-mcp/` MUST be renamed to `outlook-local-mcp/`. This is a manual operation performed outside of git and MUST be done last, after all in-repo changes are committed.

## Environment Variables

Environment variable **names** retain the `OUTLOOK_MCP_` prefix. This prefix is intentionally **not** renamed because:

- It is already generic (not calendar-specific).
- Changing env var names is a breaking change for any existing user configuration.
- The prefix accurately describes the project scope (Outlook via MCP).

However, **default values** that embed the old project name MUST be updated. Specifically, `OUTLOOK_MCP_CACHE_NAME` defaults to `"outlook-calendar-mcp"` and `OUTLOOK_MCP_AUTH_RECORD_PATH` defaults to `"~/.outlook-calendar-mcp/auth_record.json"`. Both default values MUST change to use `outlook-local-mcp`.

## Validation Criteria

1. `go build ./cmd/outlook-local-mcp/` compiles successfully.
2. `go test ./...` passes with no failures.
3. `golangci-lint run` passes with no new warnings.
4. No remaining references to `outlook-calendar-mcp` in any `.go` file.
5. No remaining references to `outlook-calendar-mcp` in `Dockerfile`, `docker-compose.yml`, or `.github/workflows/release.yml`.
6. `grep -r "outlook-calendar-mcp" --include="*.go" --include="*.yml" --include="*.yaml" --include="*.mod"` returns zero results.
7. The MCP server identifies as `outlook-local` in the `initialize` response.
8. Docker builds successfully: `docker build -t outlook-local-mcp .`

## Risks and Mitigations

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Missed reference causes build failure | Low | High | Automated grep validation in step 6 above |
| Import path mismatch after partial rename | Medium | High | Single-pass `sed` replacement of module path, followed by `go build` |
| `go.sum` becomes stale after module rename | Low | Low | Run `go mod tidy` after module path change |
| Test hardcoded server name mismatch | Low | Medium | Grep for old server name in test files |

## Implementation Notes

The recommended execution order within each phase is:

1. Rename `go.mod` module path.
2. Rename `cmd/outlook-calendar-mcp/` directory to `cmd/outlook-local-mcp/`.
3. Find-and-replace `github.com/desek/outlook-calendar-mcp` with `github.com/desek/outlook-local-mcp` across all `.go` files.
4. Update `NewMCPServer` name argument from `"outlook-calendar"` to `"outlook-local"`.
5. Update default values in `internal/config/config.go`: `AuthRecordPath` from `~/.outlook-calendar-mcp/` to `~/.outlook-local-mcp/` and `CacheName` from `outlook-calendar-mcp` to `outlook-local-mcp`.
6. Run `go mod tidy && go build ./cmd/outlook-local-mcp/ && go test ./...` to validate.
7. Proceed with Phases 2-5 (non-Go files).
8. Run full validation criteria.
9. Commit all changes.
10. Rename filesystem directory (Phase 6) as a separate manual step.

<!--
## CR Review Summary (CR-0026)

**Reviewer:** Agent 2 (CR Reviewer)
**Date:** 2026-03-15

### Findings: 7 total, 7 fixes applied, 0 unresolvable

1. **[Fixed] Incorrect occurrence count (Phase 1):** CR stated "262 occurrences across ~50 source files" but actual count is 76 occurrences across 37 files. Corrected.

2. **[Fixed] Missing default value renames (Phase 1):** `internal/config/config.go` contains default values `AuthRecordPath` (`~/.outlook-calendar-mcp/auth_record.json`) and `CacheName` (`"outlook-calendar-mcp"`) that MUST be renamed. Added to Phase 1 scope table, affected files list, and Implementation Notes step 5.

3. **[Fixed] Missing `CLAUDE.md` from Phase 4:** `CLAUDE.md` contains 3 references to `outlook-calendar-mcp` (project structure tree, build/install commands) but was not listed. Added.

4. **[Fixed] Non-existent file in Phase 4:** `docs/git-author-rewrite.md` does not exist in the repository. Removed. (Applied by linter hook.)

5. **[Fixed] False positive in Phase 5:** `docs/cr/CR-0015-audit-logging.md` contains zero references to `outlook-calendar-mcp` or `outlook-calendar`. Removed from the list.

6. **[Fixed] Ambiguous language in Phase 6:** Changed "should be renamed" and "must be done" to "MUST be renamed" and "MUST be done" for precise, testable language.

7. **[Fixed] Missing `configmap.yaml` default value note (Phase 3):** `docs/deploy/configmap.yaml` line 17 sets `OUTLOOK_MCP_CACHE_NAME: "outlook-calendar-mcp"` which is a default value, not just a label/name. Added note to Phase 3. Also added clarification to Environment Variables section distinguishing env var names (unchanged) from default values (MUST change).
-->
