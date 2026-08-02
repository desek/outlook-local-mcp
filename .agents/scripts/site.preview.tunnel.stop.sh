#!/usr/bin/env bash
#
# site.preview.tunnel.stop.sh — take the preview site off the public hostname.
#
# @agents-index Stops the Cloudflare tunnel and the local static server that
# site.preview.tunnel.sh started, so the local build stops being published.
#
# Purpose: the preview publishes a working build to a public URL. Leaving it up
# after the work is done keeps serving whatever the tree happens to contain.
#
# Usage:
#   .agents/scripts/site.preview.tunnel.stop.sh
#
# Side effects: kills the cloudflared tunnel process and the server on port 8099.
# After it runs, https://preview.outlook-local-mcp.com returns 530 rather than the
# site, which is the expected state when no tunnel is connected.
set -euo pipefail

PORT=8099

if pkill -f "cloudflared tunnel run" 2> /dev/null; then
  echo "tunnel stopped"
else
  echo "no tunnel was running"
fi

if lsof -ti:"$PORT" > /dev/null 2>&1; then
  lsof -ti:"$PORT" | xargs kill
  echo "server on port $PORT stopped"
else
  echo "no server was holding port $PORT"
fi
