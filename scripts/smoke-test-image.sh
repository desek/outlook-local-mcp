#!/usr/bin/env bash
#
# @agents-index Smoke-tests a built container image by driving a real MCP initialize plus tools/list handshake over stdio.
#
# Purpose:
#   Verifies that an image actually serves the MCP protocol, not merely that its
#   binary is executable. The server takes no command-line arguments, so the
#   obvious `docker run <image> --version` check is vacuous: the argument is
#   ignored, the server starts, reads EOF on stdin and exits 0 even if no tools
#   were registered.
#
#   This is also the check third-party directories (Glama) perform before
#   listing a server, so keeping it in CI means a listing regression is caught
#   here rather than on submission.
#
# Usage:
#   scripts/smoke-test-image.sh <image-tag> [docker-run-arg ...]
#
# Parameters:
#   image-tag        Required. A locally loaded image, e.g. outlook-local-mcp:ci-scratch.
#   docker-run-arg   Optional extra arguments passed through to `docker run`
#                    (for example --platform linux/amd64).
#
# Side effects:
#   Runs a throwaway container (docker run --rm). No files are written outside
#   the process's temporary directory. Makes no network calls: the handshake is
#   answered locally, with no Microsoft Graph credentials required.
#
# Exits non-zero if the handshake fails or any expected aggregate tool is absent.

set -euo pipefail

image="${1:-}"
if [ -z "${image}" ]; then
  echo "usage: $0 <image-tag> [docker-run-arg ...]" >&2
  exit 2
fi
shift

# The four aggregate domain tools the server must advertise (see AGENTS.md,
# "Tool Naming Convention"). A missing entry means tool registration regressed.
expected_tools=(account calendar mail system)

out="$(mktemp)"
trap 'rm -f "${out}"' EXIT

printf '%s\n%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"smoke-test","version":"1"}}}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' \
  | docker run --rm -i "$@" "${image}" > "${out}"

echo "==> Response from ${image}:"
cat "${out}"

if ! grep -q '"serverInfo"' "${out}"; then
  echo "ERROR: no initialize result returned by ${image}" >&2
  exit 1
fi

status=0
for tool in "${expected_tools[@]}"; do
  if grep -q "\"name\":\"${tool}\"" "${out}"; then
    echo "    ok: ${tool}"
  else
    echo "    MISSING: ${tool}" >&2
    status=1
  fi
done

if [ "${status}" -ne 0 ]; then
  echo "ERROR: ${image} did not advertise all expected tools" >&2
  exit 1
fi

echo "==> ${image} passed MCP introspection"
