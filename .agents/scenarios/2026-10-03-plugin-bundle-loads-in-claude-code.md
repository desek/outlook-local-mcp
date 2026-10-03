---
date: 2026-10-03
source: docs/cr/CR-0084-directory-plugin-bundle.md
outcome: intended
runs: "1 of 1 succeeded"
surface: command line
---

## Goal

Use the server in Claude Code from the plugin bundle, with no manual binary handling by the user.

## Success condition

The plugin's MCP server is connected in the session and a call to the `system` tool returns the server's status, observed as the tool result, not as transcript text.

## Procedure

1. Build a server binary: `go build -o /tmp/olm ./cmd/outlook-local-mcp`.
2. `export OUTLOOK_MCP_PLUGIN_BIN=/tmp/olm` (no raw release asset exists for v1.0.0-rc.1 yet; the override exercises the exec path, the launcher tests in `internal/plugin` exercise the download and digest paths).
3. `claude -p "<prompt asking for system help and system status output=summary>" --plugin-dir ./plugin --model haiku --output-format json --dangerously-skip-permissions < /dev/null`
4. Grade: the JSON result names the tool `mcp__plugin_outlook-local-mcp_outlook__system`, lists help output, and carries the `version` field from status.

## Observation (run 1)

```
TOOL_NAME: mcp__plugin_outlook-local-mcp_outlook__system
HELP_DOMAINS: system
STATUS_VERSION: dev
is_error False, cost_usd 0.126
```

## Intent citation

Code: `plugin/.claude-plugin/plugin.json` (`mcpServers.outlook`), `plugin/scripts/outlook-local-mcp.sh` (override branch, `exec`). Tests: `internal/plugin/launcher_test.go` `TestLauncherHonoursBinaryOverride`, `TestLauncherColdStartFetchesVerifiesAndExecs`, `TestLauncherWritesNothingToStdoutBeforeExec`.

## Criteria proved

AC-3 (command and options resolve), AC-11 (override skips download and verification). AC-4 (cold start against a real release) is not provable until a release publishes raw binary assets; it is proved against a fake release host by `TestLauncherColdStartFetchesVerifiesAndExecs`.

## Not run

- Offline warm start against the real release (needs a published raw asset).
- Portal Validate (needs a logged-in paid account).
