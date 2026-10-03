---
date: 2026-10-03
source: docs/cr/CR-0084-directory-plugin-bundle.md
outcome: intended
runs: "3 of 3 succeeded"
surface: command line
---

## Goal

Start the server from the plugin launcher against the real GitHub release, with no local binary and no override, on macOS arm64.

## Success condition

The launcher downloads the raw release binary for the plugin version, the digest check passes, the server answers MCP `initialize` with `serverInfo.version` equal to the plugin version, and a second start of the same version makes no network request.

## Procedure

1. `git checkout main` at or after the `v1.0.0-rc.2` release; `plugin.json` version is `1.0.0-rc.2`.
2. `export CLAUDE_PLUGIN_ROOT=$PWD/plugin CLAUDE_PLUGIN_DATA=$(mktemp -d)`; ensure `OUTLOOK_MCP_PLUGIN_BIN` and `OUTLOOK_MCP_PLUGIN_RELEASE_BASE` are unset.
3. Pipe one JSON-RPC `initialize` request into `plugin/scripts/outlook-local-mcp.sh`; read stdout and stderr separately.
4. Repeat with `OUTLOOK_MCP_PLUGIN_RELEASE_BASE=http://127.0.0.1:9/nope` and the same data dir.

## Observations

| Run | State | stdout | stderr |
|---|---|---|---|
| 1 | release live, digest-pin PR not yet merged | `initialize` result, `"version":"1.0.0-rc.2"` | one line: `the repository pin is absent for v1.0.0-rc.2; the release checksums are used` |
| 2 | digest-pin PR #66 merged, fresh data dir | `initialize` result, `"version":"1.0.0-rc.2"` | no launcher output |
| 3 | warm start, release host unreachable | `initialize` result, `"version":"1.0.0-rc.2"` | no launcher output |

Cached binary: `$CLAUDE_PLUGIN_DATA/1.0.0-rc.2/outlook-local-mcp`, 140 MB (the CGO desktop build is not stripped).

## Intent citation

Code: `plugin/scripts/outlook-local-mcp.sh` (download, digest lookup order, cache, exec); `.github/workflows/release.yml` steps "Create archives and checksums" (raw assets) and "Pin release digests" (PR #66). Tests: `internal/plugin/launcher_test.go` `TestLauncherColdStartFetchesVerifiesAndExecs`, `TestLauncherPrefersCommittedDigest`, `TestLauncherWarmStartMakesNoRequest`.

## Criteria proved

AC-4, AC-5, AC-12 against the real release and repository.
