Run `make crud-test` from the repository root via the Bash tool. This is the **primary, canonical** way to execute the MCP CRUD lifecycle test — do not reinterpret the prompt at `docs/prompts/mcp-tool-crud-test.md` directly. The target wraps `scripts/crud-test.sh`.

The script:
- Spawns a headless `claude -p` subprocess against the locally configured outlook-local-mcp server.
- Captures `stream-json` events to `docs/bench/runs/{timestamp}/stream.jsonl` and wall-clock to `time.txt`.
- Appends one row of metrics (duration, turns, cost, token usage, cache hits) to `docs/bench/crud-runs.csv`.
- Writes the human-readable report to `TEST-REPORT-{timestamp}.md` at the repo root.
- Prints a summary plus tool-call distribution.

Defaults: account `default`, model `claude-sonnet-4-6`, thinking effort `low`. Override via env vars: `ACCOUNT=foo MODEL=claude-opus-4-7 THINKING=medium make crud-test`. `THINKING` must be one of `low|medium|high|xhigh|max`.

After the script finishes, summarize the run: pass/fail counts, anomalies, and the appended CSV row.
