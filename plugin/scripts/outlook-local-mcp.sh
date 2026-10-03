#!/bin/sh
# @agents-index Plugin launcher: downloads the pinned outlook-local-mcp release binary, verifies its SHA-256, and execs it.
#
# The launcher writes nothing to stdout, because stdout carries the MCP stdio
# protocol of the server that it execs. Every diagnostic goes to stderr and
# names what failed, the fixes, and what to verify.
set -eu

REPO_URL="https://github.com/desek/outlook-local-mcp"

# fail prints an actionable error to stderr and exits non-zero.
# $1 what failed, $2 the labeled fixes, $3 what to verify afterwards.
fail() {
	printf 'outlook-local-mcp launcher: %s\n  fix: %s\n  verify: %s\n' "$1" "$2" "$3" >&2
	exit 1
}

# Override: run a local binary and skip download and verification.
if [ -n "${OUTLOOK_MCP_PLUGIN_BIN:-}" ]; then
	[ -x "$OUTLOOK_MCP_PLUGIN_BIN" ] || fail "OUTLOOK_MCP_PLUGIN_BIN=$OUTLOOK_MCP_PLUGIN_BIN is not an executable file" \
		"(a) point OUTLOOK_MCP_PLUGIN_BIN at a built outlook-local-mcp binary; (b) unset it to use the verified release binary" \
		"the path exists and 'ls -l' shows the execute bit"
	exec "$OUTLOOK_MCP_PLUGIN_BIN"
fi

ROOT=${CLAUDE_PLUGIN_ROOT:?CLAUDE_PLUGIN_ROOT is not set; start this launcher through Claude Code}
DATA=${CLAUDE_PLUGIN_DATA:?CLAUDE_PLUGIN_DATA is not set; start this launcher through Claude Code}

# The version lives in a one-line text file (first token; the trailing comment is the
# release-please annotation that bumps it). The launcher deliberately reads nothing
# else in the plugin, so the directory's scanner follows only this file and
# checksums.txt from here.
version=$(sed -n '1s/^[[:space:]]*\([^[:space:]#]*\).*/\1/p' "$ROOT/VERSION")
[ -n "$version" ] || fail "no version found in $ROOT/VERSION" \
	"(a) reinstall the plugin; (b) restore $ROOT/VERSION to one line: <x.y.z> # x-release-please-version" \
	"$ROOT/VERSION starts with the plugin version"

case "$(uname -s)-$(uname -m)" in
	Darwin-arm64) platform=darwin-arm64 ;;
	Linux-x86_64 | Linux-amd64) platform=linux-amd64 ;;
	*) fail "no plugin binary for platform $(uname -s)-$(uname -m); the plugin supports macOS arm64 and Linux amd64" \
		"(a) on Claude Desktop install the outlook-local-mcp.mcpb extension; (b) download the release archive for your platform from $REPO_URL/releases and set OUTLOOK_MCP_PLUGIN_BIN to it" \
		"the server starts and system operation=\"help\" answers" ;;
esac

BIN="$DATA/$version/outlook-local-mcp"
[ -x "$BIN" ] && exec "$BIN"

BASE=${OUTLOOK_MCP_PLUGIN_RELEASE_BASE:-$REPO_URL/releases/download}
ASSET="outlook-local-mcp-$platform"
mkdir -p "$DATA/$version"

# fetch downloads $1 to the file $2, never to stdout.
fetch() {
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL -o "$2" "$1"
	elif command -v wget >/dev/null 2>&1; then
		wget -q -O "$2" "$1"
	else
		fail "neither curl nor wget is installed" "(a) install curl; (b) install wget" "'command -v curl' prints a path"
	fi
}

fetch "$BASE/v$version/$ASSET" "$BIN.tmp" || {
	rm -f "$BIN.tmp"
	fail "download of $BASE/v$version/$ASSET failed" \
		"(a) check the network access to github.com and objects.githubusercontent.com; (b) confirm that release v$version has the asset $ASSET at $REPO_URL/releases" \
		"the URL downloads with 'curl -fsSLI'"
}

expected=$(awk -v v="# v$version" -v a="$ASSET" '
	/^#/ { inblock = ($0 == v); next }
	inblock && $2 == a { print $1; exit }' "$ROOT/checksums.txt" 2>/dev/null || true)
if [ -z "$expected" ]; then
	printf 'outlook-local-mcp launcher: the repository pin is absent for v%s; the release checksums are used\n' "$version" >&2
	sums="$BIN.sums.tmp"
	fetch "$BASE/v$version/checksums.txt" "$sums" || {
		rm -f "$sums" "$BIN.tmp"
		fail "download of $BASE/v$version/checksums.txt failed" \
			"(a) check the network access to github.com; (b) confirm that release v$version has checksums.txt" \
			"the URL downloads with 'curl -fsSLI'"
	}
	expected=$(awk -v a="$ASSET" '$2 == a { print $1; exit }' "$sums")
	rm -f "$sums"
fi
[ -n "$expected" ] || {
	rm -f "$BIN.tmp"
	fail "no SHA-256 digest for $ASSET in v$version" \
		"(a) wait for the release pin of v$version; (b) set OUTLOOK_MCP_PLUGIN_BIN to a binary that you verified" \
		"checksums.txt of the release lists $ASSET"
}

if command -v shasum >/dev/null 2>&1; then
	actual=$(shasum -a 256 "$BIN.tmp" | awk '{ print $1 }')
else
	actual=$(sha256sum "$BIN.tmp" | awk '{ print $1 }')
fi
if [ "$actual" != "$expected" ]; then
	rm -f "$BIN.tmp"
	fail "SHA-256 mismatch for $ASSET v$version: expected $expected, actual $actual; the download was deleted" \
		"(a) restart to download again; (b) report the mismatch at $REPO_URL/issues" \
		"the digest of the release asset equals the line for $ASSET in checksums.txt"
fi

mv "$BIN.tmp" "$BIN"
chmod +x "$BIN"
exec "$BIN"
