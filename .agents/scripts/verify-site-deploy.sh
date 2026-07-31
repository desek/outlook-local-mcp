#!/usr/bin/env bash
#
# verify-site-deploy.sh - post-deploy live check of the published website.
#
# The Lighthouse gate and the build checks all run against `dist/` on a local server.
# None of them can see what the host actually serves: whether the apex resolves, whether
# every asset path resolves at the root rather than under a project-page prefix, what
# Content-Type GitHub Pages attaches to `/index.md`, or whether the deployed artifact is
# the commit that was just merged. This script checks the live site, and is the only
# check in the project that does.
#
# It exists because the failure it guards against has happened: re-enabling the custom
# domain while `gh-pages` still held a build with `/outlook-local-mcp/...` asset paths
# left the apex serving a page whose every asset 404'd (CR-0070).
#
# Usage:
#   .agents/scripts/verify-site-deploy.sh [origin] [expected-commit]
#
# Parameters:
#   origin           Site origin to check. Defaults to https://outlook-local-mcp.com
#   expected-commit  Optional. Full or short SHA the deployed build should report in
#                    /build-info.json. Omit to report the deployed commit without
#                    asserting it.
#
# Exits 0 when every check passes, 1 otherwise. Read-only: issues GET and HEAD requests
# and writes nothing.
#
# @agents-index Post-deploy live check: status codes, asset resolution, crawler files, provenance, and the served Content-Type of /index.md.

# `pipefail` is deliberately NOT set, and the content checks below use bash pattern
# matching rather than `printf ... | grep -q`. Under `pipefail` that idiom is actively
# wrong: `grep -q` exits on its first match, `printf` then dies of SIGPIPE, and the
# pipeline reports 141 even though the pattern was found. The first version of this
# script did exactly that. It reported two false failures on a correct deployment, and
# far worse, it inverted its two negative checks — the wrong-base-path detector and the
# empty-root detector would have reported "ok" precisely when the defect was present.
set -u

ORIGIN="${1:-https://outlook-local-mcp.com}"
EXPECTED_COMMIT="${2:-}"

failures=0

# pass records a satisfied check.
pass() { printf 'ok   %s\n' "$1"; }

# fail records a violated check and marks the run failed.
fail() { printf 'FAIL %s\n' "$1"; failures=$((failures + 1)); }

# check_status asserts that a path returns the expected HTTP status.
#
# $1 path, $2 expected status code.
check_status() {
  local path="$1" want="$2" got
  got=$(curl -sS -o /dev/null -w '%{http_code}' -L "${ORIGIN}${path}")
  if [ "$got" = "$want" ]; then
    pass "${path} -> ${got}"
  else
    fail "${path} -> ${got}, expected ${want}"
  fi
}

printf '== pages ==\n'
for page in / /quickstart.html /concepts.html /troubleshooting.html; do
  check_status "$page" 200
done

printf '\n== crawler surface ==\n'
for file in /robots.txt /sitemap.xml /llms.txt /index.md /build-info.json; do
  check_status "$file" 200
done

# FR-34 left the served Content-Type of /index.md as an open question, answerable only
# against the host. Recorded rather than asserted, because the correct value is a
# decision this check cannot make.
md_type=$(curl -sS -o /dev/null -w '%{content_type}' -L "${ORIGIN}/index.md")
printf 'note /index.md Content-Type: %s\n' "${md_type:-<none>}"

printf '\n== asset resolution ==\n'
# The regression this guards: assets emitted under a project-page prefix 404 at the apex.
home=$(curl -sS -L "${ORIGIN}/")
if [[ "$home" == *'"/outlook-local-mcp/'* ]]; then
  fail 'home page references /outlook-local-mcp/ asset paths; the published build has the wrong base'
else
  pass 'no project-page asset prefix in the home page'
fi

asset_failures=0
asset_count=0
while read -r asset; do
  [ -z "$asset" ] && continue
  asset_count=$((asset_count + 1))
  got=$(curl -sS -o /dev/null -w '%{http_code}' -L "${ORIGIN}${asset}")
  if [ "$got" != "200" ]; then
    fail "asset ${asset} -> ${got}"
    asset_failures=$((asset_failures + 1))
  fi
done < <(printf '%s' "$home" | grep -oE '(href|src)="/assets/[^"]+"' | sed -E 's/.*="([^"]+)"/\1/' | sort -u)

if [ "$asset_count" -eq 0 ]; then
  fail 'no /assets/ references found in the home page; the published build changed shape'
elif [ "$asset_failures" -eq 0 ]; then
  pass "${asset_count} referenced asset(s) return 200"
fi

printf '\n== pre-render ==\n'
# The published document must carry its content without JavaScript (FR-8, FR-12).
if [[ "$home" =~ \<div\ id=\"root\"\>[[:space:]]*\</div\> ]]; then
  fail 'home page root is empty; the pre-render did not reach production'
else
  pass 'home page root carries pre-rendered markup'
fi

printf '\n== provenance ==\n'
build_info=$(curl -sS -L "${ORIGIN}/build-info.json")
deployed_commit=$(printf '%s' "$build_info" | sed -nE 's/.*"commit"[[:space:]]*:[[:space:]]*"([^"]*)".*/\1/p')
if [ -z "$deployed_commit" ]; then
  fail 'build-info.json carries no commit field'
else
  printf 'note deployed commit: %s\n' "$deployed_commit"
  if [ -n "$EXPECTED_COMMIT" ]; then
    case "$deployed_commit" in
      "$EXPECTED_COMMIT"*) pass "deployed commit matches ${EXPECTED_COMMIT}" ;;
      *) fail "deployed commit ${deployed_commit} does not match expected ${EXPECTED_COMMIT}" ;;
    esac
  fi
fi

if [[ "$home" == *'name="build:commit"'* ]]; then
  pass 'build provenance present in the home page head'
else
  fail 'build provenance meta missing from the home page head'
fi

printf '\n== canonical ==\n'
if [[ "$home" == *'<link rel="canonical" href="https://outlook-local-mcp.com/'* ]]; then
  pass 'canonical points at the apex'
else
  fail 'canonical missing or not pointing at the apex'
fi

printf '\n%s\n' "verify-site-deploy: ${failures} failure(s) against ${ORIGIN}"
[ "$failures" -eq 0 ]
