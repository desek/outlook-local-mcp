#!/usr/bin/env bash
#
# @agents-index Stages GoReleaser `container` binaries into the linux/<arch>/ layout the Dockerfile release stages COPY from.
#
# Purpose:
#   The runtime-scratch / runtime-distroless / runtime-debug stages in the
#   Dockerfile do not compile anything. They expect a prebuilt static binary at
#   `${TARGETOS}/${TARGETARCH}/outlook-local-mcp` relative to the Docker build
#   context (the repository root), because buildx substitutes those variables
#   per target platform during a multi-arch build.
#
#   GoReleaser does not emit that layout. `goreleaser build --id container`
#   writes to dist/container_linux_amd64_v1/ and dist/container_linux_arm64_v8.0/,
#   where the trailing microarchitecture suffix (_v1, _v8.0) does not correspond
#   to any TARGETARCH value. This script bridges the two.
#
#   Note: `goreleaser release` has no --id flag; only `goreleaser build` does.
#
# Usage:
#   scripts/stage-container-binaries.sh [--release]
#
# Parameters:
#   --release  Omit --snapshot so GoReleaser derives the version from the
#              checked-out Git tag. Without it the binaries are stamped
#              0.0.0-SNAPSHOT-<sha>, which would then be what `system.about`
#              reports from a published release image. Use this only where the
#              checkout is at a real tag.
#
# Side effects:
#   - Runs `goreleaser build --clean --id container`, which rewrites dist/.
#   - Creates linux/amd64/ and linux/arm64/ at the repository root (gitignored).
#
# Exits non-zero if GoReleaser fails or if an expected architecture is missing.

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${repo_root}"

snapshot_flag="--snapshot"
case "${1:-}" in
  --release) snapshot_flag="" ;;
  "") ;;
  *) echo "usage: $0 [--release]" >&2; exit 2 ;;
esac

echo "==> Building container binaries with GoReleaser"
# shellcheck disable=SC2086 # snapshot_flag is intentionally word-split or empty.
goreleaser build --clean ${snapshot_flag} --id container

echo "==> Staging binaries into linux/<arch>/"
rm -rf linux

staged=0
for src in dist/container_linux_*/outlook-local-mcp; do
  [ -f "${src}" ] || continue

  # dist/container_linux_amd64_v1 -> amd64 ; dist/container_linux_arm64_v8.0 -> arm64
  dir="$(basename "$(dirname "${src}")")"
  arch="${dir#container_linux_}"
  arch="${arch%%_*}"

  mkdir -p "linux/${arch}"
  cp "${src}" "linux/${arch}/outlook-local-mcp"
  chmod +x "linux/${arch}/outlook-local-mcp"
  echo "    ${src} -> linux/${arch}/outlook-local-mcp"
  staged=$((staged + 1))
done

# Both architectures are required: the release workflow builds the scratch and
# distroless images for linux/amd64,linux/arm64, and a missing binary would
# surface as an opaque COPY failure inside buildx.
for arch in amd64 arm64; do
  if [ ! -f "linux/${arch}/outlook-local-mcp" ]; then
    echo "ERROR: expected linux/${arch}/outlook-local-mcp was not staged" >&2
    exit 1
  fi
done

echo "==> Staged ${staged} binaries"
