#!/usr/bin/env bash
# bench.sh — paired CRUD bench for CR-0060 token-economics research.
#
# Builds a pre-CR-0060 binary and a post-CR-0060 binary, then runs the natural-
# language CRUD prompt N times against each surface, swapping the binary that
# .mcp.json references between runs. Appends one row per run to a dedicated
# CSV with a `surface` column.
#
# Usage:
#   docs/research/cr-0060-token-economics/bench.sh <surface> [runs]
#     surface = pre | post | both     (default: both)
#     runs    = integer               (default: 5)
#
# Env overrides: MODEL (default claude-sonnet-4-6), THINKING (default low),
#                ACCOUNT (default: first connected), PRE_REF (default v0.3.0).
set -euo pipefail

SURFACE_ARG="${1:-both}"
RUNS="${2:-5}"
MODEL="${MODEL:-claude-sonnet-4-6}"
THINKING="${THINKING:-low}"
PRE_REF="${PRE_REF:-v0.3.0}"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
cd "$REPO_ROOT"

BIN_LIVE="${REPO_ROOT}/outlook-local-mcp"
BIN_DIR="${REPO_ROOT}/bin"
BIN_PRE="${BIN_DIR}/outlook-local-mcp-pre"
BIN_POST="${BIN_DIR}/outlook-local-mcp-post"
WORKTREE_PRE="${REPO_ROOT}/.worktrees/cr-0060-pre"

PROMPT_FILE="${SCRIPT_DIR}/comparison-crud-prompt.md"
CSV="${SCRIPT_DIR}/comparison.csv"
RUN_ROOT="${SCRIPT_DIR}/runs"
mkdir -p "$BIN_DIR" "$(dirname "$CSV")" "$RUN_ROOT"

# ----- account selection (same single connected account on both surfaces) -----
ACCOUNTS_DIR="${HOME}/.outlook-local-mcp"
ACCOUNTS_JSON="${ACCOUNTS_DIR}/accounts.json"
LEGACY_RECORD="${ACCOUNTS_DIR}/auth_record.json"

if [[ -s "$ACCOUNTS_JSON" ]]; then
  mapfile -t CONNECTED < <(jq -r '.accounts[].label' "$ACCOUNTS_JSON" \
    | while read -r l; do [[ -n "$l" && -s "${ACCOUNTS_DIR}/${l}_auth_record.json" ]] && echo "$l"; done)
  [[ ${#CONNECTED[@]} -gt 0 ]] || { echo "ERROR: no connected account in ${ACCOUNTS_JSON}" >&2; exit 2; }
  ACCOUNT="${ACCOUNT:-${CONNECTED[0]}}"
  printf '%s\n' "${CONNECTED[@]}" | grep -qx "$ACCOUNT" || ACCOUNT="${CONNECTED[0]}"
elif [[ -s "$LEGACY_RECORD" ]]; then
  # Legacy single-account state: server uses its implicit default. Pass the
  # label the server reports to the prompt so the agent doesn't guess.
  ACCOUNT="${ACCOUNT:-default}"
else
  echo "ERROR: no auth state in ${ACCOUNTS_DIR}" >&2; exit 2
fi
echo "==> using account: ${ACCOUNT}"

# ----- build binaries -----
build_post() {
  echo "==> building post-CR binary (HEAD)"
  go build -o "$BIN_POST" ./cmd/outlook-local-mcp/
}

build_pre() {
  echo "==> building pre-CR binary (${PRE_REF})"
  if [[ ! -d "$WORKTREE_PRE" ]]; then
    git worktree add --detach "$WORKTREE_PRE" "$PRE_REF"
  fi
  ( cd "$WORKTREE_PRE" && go build -o "$BIN_PRE" ./cmd/outlook-local-mcp/ )
}

# ----- single run -----
run_once() {
  local surface="$1" idx="$2"
  local run_ts run_dir stream timefile bin
  run_ts="$(date +%Y-%m-%dT%H-%M-%S)"
  run_dir="${RUN_ROOT}/${surface}-${run_ts}"
  mkdir -p "$run_dir"
  stream="${run_dir}/stream.jsonl"
  timefile="${run_dir}/time.txt"

  case "$surface" in
    pre)  bin="$BIN_PRE"  ;;
    post) bin="$BIN_POST" ;;
    *) echo "bad surface: $surface" >&2; return 2 ;;
  esac
  # Atomic replace: cp -f over an executable on macOS can produce a binary
  # that silently fails to start (codesign cache / exec image staleness).
  # Stage to a sibling temp path, then rename atomically.
  cp -f "$bin" "${BIN_LIVE}.new"
  mv -f "${BIN_LIVE}.new" "$BIN_LIVE"

  local prompt
  prompt="Read and execute every step in ${PROMPT_FILE} against the connected Outlook MCP server using the default account (label '${ACCOUNT}'). You are running non-interactively: never invoke any account login or interactive auth tool. Do not invoke any slash command, skill, make target, or this script — execute the prompt steps directly via the registered MCP tools. Emit only the per-step PASS/FAIL/SKIP lines and the final SUMMARY line as instructed by the prompt."

  echo "==> [${surface} ${idx}/${RUNS}] ${run_ts}  bin=$(basename "$bin")"
  /usr/bin/time -p claude --dangerously-skip-permissions \
    --model "$MODEL" --effort "$THINKING" \
    --disable-slash-commands \
    --output-format stream-json --verbose \
    -p "$prompt" \
    > "$stream" 2> "$timefile" || true

  local wall_s sha
  wall_s="$(awk '/^real/ {print $2}' "$timefile")"
  sha="$(git -C "$REPO_ROOT" rev-parse --short HEAD)"
  if [[ "$surface" == "pre" ]]; then
    sha="$(git -C "$WORKTREE_PRE" rev-parse --short HEAD)"
  fi

  # tool-call distribution
  read -r MCP_CAL MCP_MAIL MCP_ACC MCP_SYS MCP_LEGACY T_OTHER < <(
    jq -r 'select(.type=="assistant") | .message.content[]? | select(.type=="tool_use") | .name' "$stream" \
    | awk '
      /^mcp__outlook-local-mcp__calendar$/  { cal++;    next }
      /^mcp__outlook-local-mcp__mail$/      { mail++;   next }
      /^mcp__outlook-local-mcp__account$/   { acc++;    next }
      /^mcp__outlook-local-mcp__system$/    { sys++;    next }
      /^mcp__outlook-local-mcp__/           { legacy++; next }
      { other++ }
      END { printf "%d %d %d %d %d %d\n", cal+0, mail+0, acc+0, sys+0, legacy+0, other+0 }'
  )

  jq -r --arg surface "$surface" --arg ts "$run_ts" --arg sha "$sha" --arg wall "$wall_s" \
    --arg cal "$MCP_CAL" --arg mail "$MCP_MAIL" --arg acc "$MCP_ACC" \
    --arg sys "$MCP_SYS" --arg legacy "$MCP_LEGACY" --arg other "$T_OTHER" '
    select(.type=="result") |
    [$surface, $ts, $sha, $wall,
     .duration_ms, .duration_api_ms, .num_turns, .total_cost_usd,
     .usage.input_tokens, .usage.output_tokens,
     .usage.cache_creation_input_tokens, .usage.cache_read_input_tokens,
     .is_error,
     ($cal|tonumber), ($mail|tonumber), ($acc|tonumber), ($sys|tonumber),
     ($legacy|tonumber), ($other|tonumber)] | @csv
  ' "$stream" >> "$CSV"
}

# ----- CSV header -----
if [[ ! -s "$CSV" ]]; then
  echo "surface,run_ts,sha,wall_s,duration_ms,duration_api_ms,num_turns,total_cost_usd,input_tokens,output_tokens,cache_creation_input_tokens,cache_read_input_tokens,is_error,mcp_calendar,mcp_mail,mcp_account,mcp_system,mcp_legacy,tool_other" > "$CSV"
fi

# ----- orchestrate -----
case "$SURFACE_ARG" in
  pre)  build_pre  ;;
  post) build_post ;;
  both) build_post; build_pre ;;
  *) echo "usage: $0 [pre|post|both] [runs]" >&2; exit 2 ;;
esac

run_surface() {
  local s="$1"
  for i in $(seq 1 "$RUNS"); do run_once "$s" "$i"; done
}

case "$SURFACE_ARG" in
  pre)  run_surface pre  ;;
  post) run_surface post ;;
  both) run_surface post; run_surface pre ;;
esac

# Restore the live binary to post-CR (HEAD) so subsequent normal use is unchanged.
if [[ -x "$BIN_POST" ]]; then
  cp -f "$BIN_POST" "${BIN_LIVE}.new"
  mv -f "${BIN_LIVE}.new" "$BIN_LIVE"
fi

echo "==> done. CSV: ${CSV}"
column -s, -t "$CSV" | tail -n +1
