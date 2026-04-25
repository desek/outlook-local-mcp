#!/usr/bin/env bash
# crud-test.sh — primary CRUD test runner for outlook-local-mcp.
#
# Runs docs/prompts/mcp-tool-crud-test.md headlessly via `claude -p` against
# the locally configured outlook-local-mcp server, captures both human-readable
# results and machine-readable metrics, and appends one row to
# docs/bench/crud-runs.csv for trend tracking.
#
# Usage:
#   scripts/crud-test.sh [account_label]
#
# Defaults: account_label=default, model=claude-sonnet-4-6, thinking effort=low.
# Override via env: ACCOUNT, MODEL, THINKING.
set -euo pipefail

ACCOUNT="${ACCOUNT:-${1:-default}}"
MODEL="${MODEL:-claude-sonnet-4-6}"
THINKING="${THINKING:-low}"

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"

RUN_TS="$(date +%Y-%m-%dT%H-%M-%S)"
BENCH_DIR="docs/bench"
RUN_DIR="${BENCH_DIR}/runs/${RUN_TS}"
CSV="${BENCH_DIR}/crud-runs.csv"
mkdir -p "$RUN_DIR"

STREAM="${RUN_DIR}/stream.jsonl"
TIMEFILE="${RUN_DIR}/time.txt"

PROMPT="Read and execute every step in docs/prompts/mcp-tool-crud-test.md against the outlook-local-mcp MCP server using account label '${ACCOUNT}'. You are running in non-interactive mode: NEVER call account.login under any circumstance; if any step would require interactive auth, mark it SKIP and continue. After completing the test run, write the full report (results table, environment table, findings, any notes) to a timestamped file at the repository root named TEST-REPORT-${RUN_TS}.md. Do not overwrite previous reports. Report pass/fail per step and a final summary table."

echo "==> CRUD test run ${RUN_TS} (account=${ACCOUNT}, model=${MODEL})"
echo "    stream: ${STREAM}"

/usr/bin/time -p claude --dangerously-skip-permissions \
  --model "$MODEL" \
  --effort "$THINKING" \
  --output-format stream-json --verbose \
  -p "$PROMPT" \
  > "$STREAM" 2> "$TIMEFILE"

BRANCH="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo unknown)"
SHA="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
WALL_S="$(awk '/^real/ {print $2}' "$TIMEFILE")"

if [[ ! -s "$CSV" ]]; then
  echo "run_ts,branch,sha,wall_s,duration_ms,duration_api_ms,num_turns,total_cost_usd,input_tokens,output_tokens,cache_creation_input_tokens,cache_read_input_tokens,is_error" > "$CSV"
fi

jq -r --arg ts "$RUN_TS" --arg branch "$BRANCH" --arg sha "$SHA" --arg wall "$WALL_S" '
  select(.type=="result") |
  [$ts, $branch, $sha, $wall,
   .duration_ms, .duration_api_ms, .num_turns, .total_cost_usd,
   .usage.input_tokens, .usage.output_tokens,
   .usage.cache_creation_input_tokens, .usage.cache_read_input_tokens,
   .is_error] | @csv
' "$STREAM" >> "$CSV"

echo "==> Summary"
jq -r 'select(.type=="result") | "  duration_ms : \(.duration_ms)
  num_turns   : \(.num_turns)
  cost_usd    : \(.total_cost_usd)
  in_tokens   : \(.usage.input_tokens)
  out_tokens  : \(.usage.output_tokens)
  cache_create: \(.usage.cache_creation_input_tokens)
  cache_read  : \(.usage.cache_read_input_tokens)
  is_error    : \(.is_error)"' "$STREAM"

echo "==> Tool-call distribution"
jq -r 'select(.type=="assistant") | .message.content[]? | select(.type=="tool_use") | .name' "$STREAM" \
  | sort | uniq -c | sort -rn | sed 's/^/  /'

echo "==> Appended row to ${CSV}"
echo "==> Report: TEST-REPORT-${RUN_TS}.md"
