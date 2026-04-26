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

# Precondition: a connected account must exist. An account is "connected" when
# it is registered in accounts.json AND has a corresponding {label}_auth_record.json.
# If the requested ACCOUNT label is not connected, fall back to the first
# connected account. If none are connected, abort and instruct the user.
ACCOUNTS_DIR="${HOME}/.outlook-local-mcp"
ACCOUNTS_JSON="${ACCOUNTS_DIR}/accounts.json"
if [[ ! -s "$ACCOUNTS_JSON" ]]; then
  echo "ERROR: no accounts registered at ${ACCOUNTS_JSON}." >&2
  echo "Run the server interactively and add an account via the 'account.login' verb before retrying." >&2
  exit 2
fi

mapfile -t CONNECTED < <(jq -r '.accounts[].label' "$ACCOUNTS_JSON" 2>/dev/null \
  | while read -r label; do
      [[ -n "$label" && -s "${ACCOUNTS_DIR}/${label}_auth_record.json" ]] && echo "$label"
    done)

if [[ ${#CONNECTED[@]} -eq 0 ]]; then
  echo "ERROR: no connected accounts found (no {label}_auth_record.json in ${ACCOUNTS_DIR})." >&2
  echo "Run the server interactively and authenticate an account via 'account.login' before retrying." >&2
  exit 2
fi

if ! printf '%s\n' "${CONNECTED[@]}" | grep -qx "$ACCOUNT"; then
  echo "==> Requested account '${ACCOUNT}' is not connected; using '${CONNECTED[0]}' instead."
  ACCOUNT="${CONNECTED[0]}"
fi

RUN_TS="$(date +%Y-%m-%dT%H-%M-%S)"
BENCH_DIR="docs/bench"
RUN_DIR="${BENCH_DIR}/runs/${RUN_TS}"
CSV="${BENCH_DIR}/crud-runs.csv"
mkdir -p "$RUN_DIR"

STREAM="${RUN_DIR}/stream.jsonl"
TIMEFILE="${RUN_DIR}/time.txt"

PROMPT="Read and execute every step in docs/prompts/mcp-tool-crud-test.md against the outlook-local-mcp MCP server using account label '${ACCOUNT}'. You are running in non-interactive mode: NEVER call account.login under any circumstance; if any step would require interactive auth, mark it SKIP and continue. DO NOT invoke the 'outlook-llm-tests' skill, the '/outlook-llm-tests' slash command, 'make crud-test', or 'scripts/crud-test.sh' — you ARE the test runner; execute the prompt steps directly via the mcp__outlook-local-mcp__* tools. After completing the test run, write the full report (results table, environment table, findings, any notes) to a timestamped file at the repository root named TEST-REPORT-${RUN_TS}.md. Do not overwrite previous reports. Report pass/fail per step and a final summary table."

echo "==> CRUD test run ${RUN_TS} (account=${ACCOUNT}, model=${MODEL})"
echo "    stream: ${STREAM}"

# --disable-slash-commands prevents the headless agent from invoking the
# outlook-llm-tests skill (which would shell back into `make crud-test` and
# recurse). The agent must execute the test steps directly.
/usr/bin/time -p claude --dangerously-skip-permissions \
  --model "$MODEL" \
  --effort "$THINKING" \
  --disable-slash-commands \
  --output-format stream-json --verbose \
  -p "$PROMPT" \
  > "$STREAM" 2> "$TIMEFILE"

BRANCH="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo unknown)"
SHA="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
WALL_S="$(awk '/^real/ {print $2}' "$TIMEFILE")"

if [[ ! -s "$CSV" ]]; then
  echo "run_ts,branch,sha,wall_s,duration_ms,duration_api_ms,num_turns,total_cost_usd,input_tokens,output_tokens,cache_creation_input_tokens,cache_read_input_tokens,is_error,mcp_calendar,mcp_mail,mcp_account,mcp_system,bash,read,write,tool_other" > "$CSV"
fi

# Tool-call counts per bucket (mcp__outlook-local-mcp__{domain} + common built-ins).
read -r MCP_CAL MCP_MAIL MCP_ACC MCP_SYS T_BASH T_READ T_WRITE T_OTHER < <(
  jq -r 'select(.type=="assistant") | .message.content[]? | select(.type=="tool_use") | .name' "$STREAM" \
  | awk '
    { total++ }
    /^mcp__outlook-local-mcp__calendar$/ { cal++; next }
    /^mcp__outlook-local-mcp__mail$/     { mail++; next }
    /^mcp__outlook-local-mcp__account$/  { acc++;  next }
    /^mcp__outlook-local-mcp__system$/   { sys++;  next }
    /^Bash$/  { bash++;  next }
    /^Read$/  { read_++; next }
    /^Write$/ { write++; next }
    { other++ }
    END { printf "%d %d %d %d %d %d %d %d\n", cal+0, mail+0, acc+0, sys+0, bash+0, read_+0, write+0, other+0 }
  '
)

jq -r --arg ts "$RUN_TS" --arg branch "$BRANCH" --arg sha "$SHA" --arg wall "$WALL_S" \
  --arg cal "$MCP_CAL" --arg mail "$MCP_MAIL" --arg acc "$MCP_ACC" --arg sys "$MCP_SYS" \
  --arg bash "$T_BASH" --arg read "$T_READ" --arg write "$T_WRITE" --arg other "$T_OTHER" '
  select(.type=="result") |
  [$ts, $branch, $sha, $wall,
   .duration_ms, .duration_api_ms, .num_turns, .total_cost_usd,
   .usage.input_tokens, .usage.output_tokens,
   .usage.cache_creation_input_tokens, .usage.cache_read_input_tokens,
   .is_error,
   ($cal|tonumber), ($mail|tonumber), ($acc|tonumber), ($sys|tonumber),
   ($bash|tonumber), ($read|tonumber), ($write|tonumber), ($other|tonumber)] | @csv
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
