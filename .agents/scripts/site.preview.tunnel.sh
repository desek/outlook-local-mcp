#!/usr/bin/env bash
#
# site.preview.tunnel.sh — publish the locally built site at the preview hostname.
#
# @agents-index Serves site/dist on localhost:8099 and runs the named Cloudflare
# tunnel that fronts it at preview.outlook-local-mcp.com, so a local build can be
# seen and measured through a public URL.
#
# Purpose: the tunnel and its credentials existed only in a CR ledger, so no agent
# could find them. This script is that knowledge in runnable form.
#
# Usage:
#   .agents/scripts/site.preview.tunnel.sh [--no-build]
#
# Parameters:
#   --no-build  serve site/dist as it stands instead of rebuilding it first.
#
# Requires site/.env (gitignored) with CLOUDFLARE_API_TOKEN, CLOUDFLARE_ACCOUNT_ID,
# CLOUDFLARE_TUNNEL_ID, and CLOUDFLARE_TUNNEL_NAME. The token is scoped to this zone
# only. The tunnel identity lives there rather than in this script, so the script
# carries no account-specific value and a different tunnel needs no edit here.
#
# Side effects: publishes the local build to a PUBLIC hostname, replaces any
# process already holding port 8099, and starts two background processes. Stop
# them with .agents/scripts/site.preview.tunnel.stop.sh.
#
# What the preview does NOT tell you, because the differences have already misled
# a measurement here:
#   * Content-Type, compression, and 404 handling are this script's, not GitHub
#     Pages'.
#   * Performance is pessimistic: traffic routes edge to laptop, not edge to CDN.
#   * Cloudflare injects email-decode.min.js, which Lighthouse charges to Best
#     Practices and which does not exist on the real host.
#   * Canonical, og:url, and JSON-LD url all point at the apex, so the preview
#     page disclaims itself for anything indexing-related.
set -euo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
HOSTNAME_PUBLIC="preview.outlook-local-mcp.com"
PORT=8099
LOG_DIR="${TMPDIR:-/tmp}/outlook-site-preview"

REQUIRED_VARS=(
  CLOUDFLARE_API_TOKEN
  CLOUDFLARE_ACCOUNT_ID
  CLOUDFLARE_TUNNEL_ID
  CLOUDFLARE_TUNNEL_NAME
)

if [[ ! -f "$REPO/site/.env" ]]; then
  echo "error: $REPO/site/.env is missing." >&2
  echo "It must define: ${REQUIRED_VARS[*]}" >&2
  exit 66
fi

set -a
# shellcheck disable=SC1091
source "$REPO/site/.env"
set +a

for var in "${REQUIRED_VARS[@]}"; do
  if [[ -z "${!var:-}" ]]; then
    echo "error: $var is not set in $REPO/site/.env." >&2
    echo "The tunnel identity and its credentials live there, not in this script." >&2
    exit 66
  fi
done

mkdir -p "$LOG_DIR"

if [[ "${1:-}" != "--no-build" ]]; then
  echo "building site/dist ..."
  pnpm --dir "$REPO/site" run build > "$LOG_DIR/build.log" 2>&1 ||
    { echo "error: the site build failed. See $LOG_DIR/build.log" >&2; exit 1; }
fi

if [[ ! -f "$REPO/site/dist/index.html" ]]; then
  echo "error: $REPO/site/dist/index.html is missing. Build first." >&2
  exit 66
fi

# Replace whatever holds the port. A stale server from an earlier session serves
# the directory it was started in, which is not necessarily this build.
if lsof -ti:"$PORT" > /dev/null 2>&1; then
  echo "port $PORT is held, replacing that process"
  lsof -ti:"$PORT" | xargs kill
  sleep 1
fi

python3 -m http.server "$PORT" --directory "$REPO/site/dist" \
  > "$LOG_DIR/http.log" 2>&1 &
echo "serving site/dist on http://localhost:$PORT (pid $!)"

TOKEN=$(curl -sf \
  -H "Authorization: Bearer $CLOUDFLARE_API_TOKEN" \
  "https://api.cloudflare.com/client/v4/accounts/$CLOUDFLARE_ACCOUNT_ID/cfd_tunnel/$CLOUDFLARE_TUNNEL_ID/token" |
  python3 -c 'import json,sys; print(json.load(sys.stdin)["result"])')

if [[ -z "$TOKEN" ]]; then
  echo "error: the Cloudflare API returned no tunnel token." >&2
  echo "Check CLOUDFLARE_API_TOKEN (needs Tunnel:Edit) and CLOUDFLARE_ACCOUNT_ID." >&2
  exit 1
fi

pkill -f "cloudflared tunnel run" 2> /dev/null || true
cloudflared tunnel run --token "$TOKEN" > "$LOG_DIR/tunnel.log" 2>&1 &
echo "tunnel $CLOUDFLARE_TUNNEL_NAME starting (pid $!)"

for _ in $(seq 1 30); do
  code=$(curl -s -o /dev/null -w "%{http_code}" --max-time 10 "https://$HOSTNAME_PUBLIC/" || true)
  if [[ "$code" == "200" ]]; then
    echo "live: https://$HOSTNAME_PUBLIC/"
    echo "logs: $LOG_DIR"
    exit 0
  fi
  sleep 2
done

echo "error: https://$HOSTNAME_PUBLIC/ did not return 200. See $LOG_DIR/tunnel.log" >&2
exit 1
