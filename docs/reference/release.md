# Release and Build

Reference documentation for the GoReleaser pipeline, snapshot builds, MCPB extension packaging, and release artifacts.

---

## Overview

The release pipeline produces three artifact types:

1. **Go binary releases** via GoReleaser — platform-native binaries uploaded to GitHub Releases and installable via `go install`.
2. **MCPB extension package** — a `.mcpb` archive containing the binary and `extension/manifest.json`, installable in Claude Desktop with one click.
3. **Container images** — multi-arch OCI images published to `ghcr.io/desek/outlook-local-mcp` on every tagged release (see [Container images](#container-images) below).

All three are produced by CI from a Git tag. One step is **not** automated: the [Glama directory listing](#directory-listings) publishes its own release from a pinned commit, and a tagged release is not complete until that release is published too.

One gate is **not** automated either: [`make crud-test`](#required-manual-gate-make-crud-test) exercises the live tool surface against a real mailbox and MUST be run by hand before a release is cut.

One surface is easy to forget: the [published website](#the-published-site-is-a-release-surface) states the tool surface to every reader and every generative engine that retrieves it, so a release that changes a verb or a domain is not complete until the site is rebuilt from the regenerated manifest.

---

## Required manual gate: `make crud-test`

`make crud-test` is a **required manual gate before a release is cut**. It drives the full MCP tool surface through a create-read-update-delete lifecycle (`docs/prompts/mcp-tool-crud-test.md`) and is the only check that exercises the tools end to end against a real tenant. It has caught defects no unit test could, including the `get_docs` explicit-anchor failure recorded in CR-0074, which every automated check reported as passing.

### Why it cannot run in CI

The harness needs live Microsoft 365 credentials and performs real create, update, and delete operations against a real mailbox and calendar. CI has neither the credentials nor a mailbox it may mutate, so the gate cannot be wired into a workflow without a dedicated test tenant, which is out of scope for CR-0074 and named there as the real long-term fix.

### Honest limitation

A documented manual gate is weaker than an automated one, and this document says so rather than implying otherwise. Its value is that its absence becomes a visible omission a maintainer can be held to, not a silent one. It does not run unless a human remembers to run it, so a release checklist MUST treat a missing `make crud-test` run as a blocking omission. The durable fix is a dedicated test tenant that lets the gate run in CI; until that exists, the run is manual and its evidence (the `docs/bench/crud-runs.csv` row for the run) is the record that it happened.

---

## The published site is a release surface

The website at `outlook-local-mcp.com` is not marketing separate from the release. It is the artifact a generative engine retrieves and quotes verbatim, so whatever it says about the tool surface is a claim the project is held to. A release that adds, renames, or removes a verb or a domain therefore **MUST NOT** be considered complete until the site has been rebuilt from the regenerated surface manifest (`site/src/generated/surface.json`), because until then the live site describes the interface the previous release exposed.

Since CR-0073 the site states no figure of its own: every tool count, verb name, domain name, and configuration variable it displays is derived from that manifest, which `cmd/gen-surface` generates from the live verb registry. The drift check in `make ci` and in both workflows fails a stale manifest before merge, and `deploy-site.yml` republishes the site on push to `main`, so in the ordinary flow the regeneration and the redeploy happen without a manual step.

### Honest limitation

That automation covers the ordinary flow, not every flow, and this document names the gap rather than implying the machine closes it. The redeploy is triggered by a push to `main`, so a tool-surface change that reaches a release tag by any path that did not run `deploy-site.yml` against the release commit leaves the live site describing the prior surface, silently and for as long as nobody looks. The value of naming it here is the same as for `make crud-test`: a release checklist MUST treat "the live site's tool count matches the release" as a verifiable line a maintainer can be held to, confirmed by reading the live page rather than assumed. The durable coupling is the drift check that already prevents a stale manifest from merging; the residual manual step is confirming the deploy that publishes it actually ran for the release being cut.

---

## GoReleaser

The project uses [GoReleaser](https://goreleaser.com/) to build and publish cross-platform binaries. The configuration lives in `.goreleaser.yaml` at the repository root.

### Snapshot build (local, no publish)

```bash
make snapshot
# equivalent: goreleaser release --snapshot --clean
```

Produces platform binaries under `dist/`. Use this to verify the build matrix without publishing.

### Configuration validation

```bash
make goreleaser-check
# equivalent: goreleaser check
```

Validates `.goreleaser.yaml` against the installed GoReleaser version without building anything. This target is part of `make ci`.

### Release artifacts

After a tagged release, GoReleaser produces:

| Artifact | Description |
|----------|-------------|
| `outlook-local-mcp-darwin-arm64` | macOS Apple Silicon binary |
| `outlook-local-mcp-darwin-amd64` | macOS Intel binary |
| `outlook-local-mcp-linux-amd64` | Linux x86-64 binary |
| `outlook-local-mcp-windows-amd64.exe` | Windows x86-64 binary |
| `outlook-local-mcp_<version>_checksums.txt` | SHA-256 checksums for all artifacts |
| SBOM files (CycloneDX, SPDX) | Generated by `make sbom` |

---

## MCPB Extension Packaging

The MCPB format is a ZIP archive recognised by Claude Desktop for one-click extension installation. The extension manifest at `extension/manifest.json` declares the tool surface and platform binary paths.

### Targets

| Target | Description |
|--------|-------------|
| `make mcpb-validate` | Validates `extension/manifest.json` against the MCPB schema (`mcpb validate`). Part of `make ci`. |
| `make mcpb-pack` | Builds cross-platform binaries via `make snapshot`, then packs `outlook-local-mcp.mcpb`. |
| `make mcpb-local` | Builds a local Darwin arm64 binary and packs a local `.mcpb`. Useful for testing the extension on a development machine without a full cross-compile. |
| `make mcpb-clean` | Removes `extension/bin/` and all `.mcpb` files. |

### Packaging workflow

```bash
# Full release packaging (cross-platform)
make mcpb-pack

# Local development packaging (darwin arm64 only)
make mcpb-local
```

`mcpb-pack` depends on `make snapshot` (GoReleaser cross-compile) and `make mcpb-validate`. It copies the platform binaries from `dist/` into `extension/bin/` before packing.

### Manifest

`extension/manifest.json` contains the four aggregate domain tools (`calendar`, `mail`, `account`, `system`) with their annotations. When a new verb is added or a tool annotation changes, the manifest **MUST** be updated to match. The manifest is validated by `make mcpb-validate`, which is wired into `make ci`.

---

## Container images

As of CR-0066, every tagged release publishes multi-arch OCI images to `ghcr.io/desek/outlook-local-mcp` via a dedicated `container` job in `.github/workflows/release.yml`. The job runs after the `release` job and uses `docker buildx` with QEMU emulation to produce `linux/amd64` and `linux/arm64` layers.

### Image variants

| Tag | Base | UID | Size | Use case |
|-----|------|-----|------|----------|
| `latest`, `v<version>` | `scratch` | root (0) | ~15 MB | Default — minimum attack surface |
| `distroless`, `v<version>-distroless` | `gcr.io/distroless/static-debian12:nonroot` | nonroot (65532) | ~17 MB | Non-root enforcement (Kubernetes PSA, OpenShift) |
| `debug`, `v<version>-debug` | `gcr.io/distroless/static-debian12:debug` | nonroot (65532) | larger | Incident response only; not for production |

The scratch and distroless variants are built for both `linux/amd64` and `linux/arm64`. The debug variant is `linux/amd64` only.

### CI validation

Every PR runs two container jobs:

* `container-build` builds the `runtime-scratch` and `runtime-distroless` target stages for `linux/amd64` (without pushing) and asserts the distroless image's configured user is `nonroot` or `65532`.
* `container-build-standalone` builds the default target from a clean checkout with a bare `docker build .`, which is the path third-party build services and `docker-compose` use.

Both jobs smoke-test via `scripts/smoke-test-image.sh`, which drives a real `initialize` plus `tools/list` handshake over stdio and asserts all four aggregate tools are advertised. It deliberately does not use `--version`: the binary takes no arguments, so `docker run <image> --version` starts the server, reads EOF and exits 0 even when no tools registered, proving only that the file is executable.

### Build mechanics

The release stages do not compile. They expect a prebuilt static binary at `${TARGETOS}/${TARGETARCH}/outlook-local-mcp` relative to the build context, because buildx substitutes those variables per target platform.

`scripts/stage-container-binaries.sh` produces that layout. GoReleaser does not emit it directly: `goreleaser build --id container` writes to `dist/container_linux_amd64_v1/` and `dist/container_linux_arm64_v8.0/`, where the trailing microarchitecture suffix corresponds to no `TARGETARCH` value. The script bridges the two and fails loudly if either architecture is missing.

Note that `goreleaser release` has no `--id` flag; only `goreleaser build` does. The release workflow passes `--release` so the binaries carry the real tag rather than a `0.0.0-SNAPSHOT` version, which is what `system.about` reports from a published image.

Reusing the cross-compiled binary rather than recompiling keeps release time overhead to roughly two minutes.

### Runtime notes

Both image variants export `RUNNING_IN_CONTAINER=1` (CR-0067) so `system.about` reports `runtime=container` even from a `scratch` image where filesystem markers (`/.dockerenv`, `/proc/1/cgroup`) may be absent. The OS keychain is not available inside the container; the server automatically falls back to file-backed token storage at `/data/auth/`. See [Container runtime](../concepts.md#container-runtime) for the full narrative and [Container deployment](../quickstart.md#container-deployment) for the recommended invocation pattern.

---

## Directory listings

The server is listed in the [Glama MCP registry](https://glama.ai/mcp/servers/desek/outlook-local-mcp). Glama does not track Git tags: it publishes its own releases, built from a pinned commit via a build spec configured in its admin UI. **A GitHub release is therefore not complete until the corresponding Glama release is published**, or the directory silently continues serving the previous build.

### Why it matters

Glama's quality score is not computed until a release exists. Until then the listing shows `quality - not tested` and the per-tool rows sit at `pending`, regardless of how good the tool definitions are. The score grades tool definition quality (70%) and server coherence (30%), so it reflects whatever verb registry and descriptions were in the pinned commit.

### Publishing a Glama release

1. Claim the server, if not already claimed. One-time.
2. Open the Dockerfile admin page, configure the build spec, and click **Deploy**.
3. Once the build test succeeds, click **Make Release**, enter the version, and publish.

### Version consistency

The Glama release version **MUST** equal the Git tag, and the pinned commit SHA **MUST** be the commit that tag points at. Glama's version field is free text and is not validated against the repository, so nothing prevents the two diverging.

This has already happened once: release `0.4.0` was published from commit `326f3d74`, which is one commit ahead of the `v0.4.0` tag (`5b22fb5`). That commit is the squash-merge carrying CR-0066, CR-0067 and CR-0068, so the image published as "0.4.0" contains container distribution, `system.about`, and registry-computed annotations that the real `v0.4.0` does not. Because the quality score grades the pinned commit, the published score described code the named version did not contain.

Cut the Git tag first, then publish Glama against that tag's commit.

### Build spec values

Glama generates its own Dockerfile from form fields rather than using the repository's. The repository `Dockerfile` builds a Go binary; Glama's template assumes Node and Python, so the Go toolchain is installed via build steps. Values verified against a local build of the generated file:

* **Base image**: `debian:trixie-slim`. Node.js and Python version fields are unused by a Go build; leave the defaults.
* **Build steps**: install `ca-certificates`, `curl`, then the Go toolchain tarball, then `CGO_ENABLED=0 go build -trimpath -o /usr/local/bin/outlook-local-mcp ./cmd/outlook-local-mcp`. `CGO_ENABLED=0` avoids needing a C toolchain in the slim image and selects the file-backed token cache, matching the release containers.
* **CMD**: the binary path. Glama wraps it in `mcp-proxy`, which serves streamable HTTP on `/mcp` (not `/sse`) and requires a full `initialize` plus `notifications/initialized` handshake before `tools/list` will answer.
* **Environment variables JSON schema**: declare no `required` entries. Every variable has a working default, and the server starts and answers introspection with none set. Marking any as required forces placeholder credentials that the checks do not need.
* **Placeholder parameters**: `{}`, which is valid once the schema has no required entries.
* **Pinned commit SHA**: the tagged release commit, per the version consistency rule above.

The Go tarball URL is architecture-specific. If Glama's builders change architecture, that build step needs the matching `linux-<arch>` tarball.

---

## SBOM and Security Scanning

### SBOM generation

```bash
make sbom
```

Generates two SBOM formats from the compiled binary:

- `outlook-local-mcp.cdx.json` (CycloneDX)
- `outlook-local-mcp.spdx.json` (SPDX)

### Vulnerability scan

```bash
make vuln-scan
```

Scans the CycloneDX SBOM with `grype`. Fails the build if any **high**-severity vulnerability is found.

### License compliance

```bash
make license-check
```

Scans Go module dependencies with `syft` and checks licenses with `grant`. Used to ensure all dependency licenses are compatible with the project's MIT licence.

For what each instrument (`govulncheck`, Dependabot, `grype`) measures, why their finding counts differ, how to triage a new alert, and the standing published ceiling, see [`docs/reference/security.md`](security.md). Dependency currency is maintained by `.github/dependabot.yml`. See CR-0071.

---

## Version injection

GoReleaser injects the version string at build time via `ldflags`. The version is available at runtime as the `version` field in `system.status` output and in the `docs.version` field reported by the docs surface.

See `.goreleaser.yaml` for the exact `ldflags` configuration and build matrix.
