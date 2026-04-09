---
id: "CR-0057"
status: "proposed"
date: 2026-04-09
requestor: desek
stakeholders:
  - desek
priority: "medium"
target-version: "0.3.0"
source-branch: main
source-commit: 87060b5
---

# Homebrew Tap and Scoop Bucket Distribution

## Change Summary

Add native package manager distribution for macOS/Linux (Homebrew) and Windows (Scoop) by creating a Homebrew tap repository (`desek/homebrew-tap`), a Scoop bucket repository (`desek/scoop-bucket`), and configuring GoReleaser's `brews` and `scoops` sections to automatically generate and publish formula/manifest files on each release. After implementation, users can install with `brew install desek/tap/outlook-local-mcp` or `scoop bucket add desek https://github.com/desek/scoop-bucket && scoop install outlook-local-mcp`.

## Motivation and Background

The project currently distributes binaries through three channels:

1. **GitHub Releases** — compressed archives (`.tar.gz`/`.zip`) with checksums, requiring manual download, extraction, and PATH configuration.
2. **MCPB extension bundle** — `.mcpb` file for Claude Desktop's built-in extension system, limited to Claude Desktop users.
3. **`go install`** — requires a Go toolchain, builds from source, and does not include version injection.

None of these provide the frictionless install-and-update experience that package managers offer. Homebrew is the dominant package manager on macOS (the primary deployment target per the MCPB extension's platform support) and is widely used on Linux. Scoop is the leading non-admin package manager on Windows. Together, they cover all three desktop platforms with a single-command install and upgrade path.

CR-0036 (GoReleaser Integration) explicitly deferred both Homebrew tap formula generation and Scoop manifest for Windows as out-of-scope items. This CR fulfills that deferred work.

## Change Drivers

* **Installation friction**: Manual archive download, extraction, and PATH configuration is the #1 barrier to adoption for CLI tools.
* **Upgrade path**: Package managers provide `brew upgrade` and `scoop update` for zero-friction version updates.
* **Discoverability**: Homebrew taps and Scoop buckets are searchable, improving project visibility.
* **CR-0036 deferred items**: Both Homebrew and Scoop were explicitly marked for a future CR.

## Current State

### Release Pipeline

The release pipeline (`.github/workflows/release.yml`) operates in three stages:

1. **release-please** creates a GitHub Release with changelog.
2. **build-desktop** builds native binaries on macOS, Linux, and Windows via `goreleaser build --single-target --id desktop`.
3. **release** combines artifacts into compressed archives, creates checksums, packages MCPB bundle, and uploads all assets to the GitHub Release.

### GoReleaser Configuration

`.goreleaser.yaml` defines:
- Two build IDs: `desktop` (CGO_ENABLED=1, 4 targets) and `container` (CGO_ENABLED=0, linux only).
- Archives: `tar.gz` for Unix, `zip` for Windows, named `outlook-local-mcp-{os}-{arch}.{ext}`.
- Checksums: SHA-256 in `checksums.txt`.
- SBOMs: CycloneDX and SPDX JSON.
- Docker: `ghcr.io/desek/outlook-local-mcp` (configured but not pushed in current workflow).
- **No `brews` or `scoops` sections exist.**

### Release Workflow Architecture

The current release workflow does **not** invoke `goreleaser release --clean`. Instead, it uses `goreleaser build --single-target` per platform in a matrix, then manually creates archives and checksums. This means GoReleaser's built-in `brews` and `scoops` publishers **cannot run automatically** within the current pipeline — they require either `goreleaser release` or a separate post-release step.

### Platform Build Matrix

| OS | Arch | Archive | Status |
|----|------|---------|--------|
| linux | amd64 | `outlook-local-mcp-linux-amd64.tar.gz` | Built |
| linux | arm64 | N/A (container only) | Built for Docker, not in desktop release |
| darwin | arm64 | `outlook-local-mcp-darwin-arm64.tar.gz` | Built |
| windows | amd64 | `outlook-local-mcp-windows-amd64.zip` | Built |

### Current State Diagram

```mermaid
flowchart LR
    RP[release-please] --> BD[build-desktop\n3 platform matrix]
    BD --> R[release job\narchive + checksum\n+ MCPB pack]
    R --> GH[GitHub Release\n.tar.gz / .zip\nchecksums.txt\n.mcpb]

    style GH fill:#9f9,stroke:#333
```

## Proposed Change

### 1. Create Homebrew Tap Repository

Create a new GitHub repository `desek/homebrew-tap` with:
- A `Formula/` directory (Homebrew convention).
- A minimal `README.md` explaining usage.
- GoReleaser will auto-commit the formula file to this repo on each release.

### 2. Create Scoop Bucket Repository

Create a new GitHub repository `desek/scoop-bucket` with:
- A `bucket/` directory (Scoop convention).
- A minimal `README.md` explaining usage.
- GoReleaser will auto-commit the manifest file to this repo on each release.

### 3. Add `brews` Section to `.goreleaser.yaml`

```yaml
brews:
  - ids:
      - binaries
    repository:
      owner: desek
      name: homebrew-tap
      token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"
    directory: Formula
    homepage: https://github.com/desek/outlook-local-mcp
    description: "MCP server for Microsoft Outlook — calendar, mail, and account management via Microsoft Graph API"
    license: MIT
    install: |
      bin.install "outlook-local-mcp"
    test: |
      assert_match version.to_s, shell_output("#{bin}/outlook-local-mcp --version 2>&1", 1)
    commit_author:
      name: "github-actions[bot]"
      email: "github-actions[bot]@users.noreply.github.com"
```

### 4. Add `scoops` Section to `.goreleaser.yaml`

```yaml
scoops:
  - ids:
      - binaries
    repository:
      owner: desek
      name: scoop-bucket
      token: "{{ .Env.SCOOP_BUCKET_TOKEN }}"
    directory: bucket
    homepage: https://github.com/desek/outlook-local-mcp
    description: "MCP server for Microsoft Outlook — calendar, mail, and account management via Microsoft Graph API"
    license: MIT
    commit_author:
      name: "github-actions[bot]"
      email: "github-actions[bot]@users.noreply.github.com"
```

### 5. Add Post-Release Publish Step to Release Workflow

Since the current release workflow uses manual archive creation (not `goreleaser release`), the `brews` and `scoops` publishers cannot run as part of the standard GoReleaser execution. Instead, add a **post-release publish step** that invokes GoReleaser in publish-only mode after the archives and checksums are uploaded to the GitHub Release:

```yaml
- name: Publish Homebrew formula and Scoop manifest
  env:
    GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
    HOMEBREW_TAP_TOKEN: ${{ secrets.HOMEBREW_TAP_TOKEN }}
    SCOOP_BUCKET_TOKEN: ${{ secrets.SCOOP_BUCKET_TOKEN }}
  run: |
    goreleaser publish --clean
```

If `goreleaser publish` is not viable with the current hybrid workflow, the alternative is a standalone script that:
1. Downloads the release checksums from the GitHub Release.
2. Renders the Homebrew formula and Scoop manifest from templates.
3. Commits them to the respective tap/bucket repositories via `gh api` or `git push`.

### 6. Create GitHub Personal Access Token (PAT) Secrets

GoReleaser needs write access to the tap and bucket repositories to commit the formula/manifest files. Two repository secrets are required:

- `HOMEBREW_TAP_TOKEN` — a GitHub PAT with `repo` scope for `desek/homebrew-tap`.
- `SCOOP_BUCKET_TOKEN` — a GitHub PAT with `repo` scope for `desek/scoop-bucket`.

Alternatively, a single fine-grained PAT with write access to both repositories can be used, referenced by both environment variables.

### Proposed State Diagram

```mermaid
flowchart LR
    RP[release-please] --> BD[build-desktop\n3 platform matrix]
    BD --> R[release job\narchive + checksum\n+ MCPB pack]
    R --> GH[GitHub Release\n.tar.gz / .zip\nchecksums.txt\n.mcpb]
    R --> PUB[Publish step]
    PUB --> HB["desek/homebrew-tap\nFormula/outlook-local-mcp.rb"]
    PUB --> SC["desek/scoop-bucket\nbucket/outlook-local-mcp.json"]

    style GH fill:#9f9,stroke:#333
    style HB fill:#9cf,stroke:#333
    style SC fill:#9cf,stroke:#333
```

## Requirements

### Functional Requirements

#### Homebrew Tap

1. A GitHub repository `desek/homebrew-tap` **MUST** exist with a `Formula/` directory.
2. The `.goreleaser.yaml` `brews` section **MUST** generate a formula file `Formula/outlook-local-mcp.rb` targeting `desek/homebrew-tap`.
3. The formula **MUST** reference the `darwin-arm64` and `linux-amd64` archives from the GitHub Release.
4. The formula **MUST** include SHA-256 checksums for each archive (auto-generated by GoReleaser from `checksums.txt`).
5. The formula **MUST** include an `install` block that installs the binary to `bin/`.
6. The formula **MUST** include a `test` block that verifies the binary runs.
7. The formula **MUST** set `homepage`, `description`, and `license` fields.
8. The formula commit **MUST** use `github-actions[bot]` as the commit author.
9. `brew install desek/tap/outlook-local-mcp` **MUST** succeed on macOS (arm64) and Linux (amd64).
10. `brew upgrade outlook-local-mcp` **MUST** pick up new versions after a release.

#### Scoop Bucket

11. A GitHub repository `desek/scoop-bucket` **MUST** exist with a `bucket/` directory.
12. The `.goreleaser.yaml` `scoops` section **MUST** generate a manifest file `bucket/outlook-local-mcp.json` targeting `desek/scoop-bucket`.
13. The manifest **MUST** reference the `windows-amd64` zip archive from the GitHub Release.
14. The manifest **MUST** include the SHA-256 checksum for the archive.
15. The manifest **MUST** set `homepage`, `description`, and `license` fields.
16. The manifest **MUST** include a `bin` entry pointing to `outlook-local-mcp.exe`.
17. The manifest commit **MUST** use `github-actions[bot]` as the commit author.
18. `scoop install outlook-local-mcp` **MUST** succeed on Windows (amd64) after adding the bucket.

#### Release Workflow

19. The release workflow **MUST** publish the Homebrew formula and Scoop manifest as part of the release process.
20. The publish step **MUST** use repository secrets for cross-repo write access (not the default `GITHUB_TOKEN`, which is scoped to the source repo only).
21. The publish step **MUST** only execute when `release_created == true`.
22. Formula/manifest publication failure **MUST NOT** fail the overall release — assets are already uploaded to GitHub Releases at this point.

#### Documentation

23. The project `README.md` **MUST** be updated with Homebrew and Scoop installation instructions.
24. The `desek/homebrew-tap` repository **MUST** include a `README.md` with usage instructions.
25. The `desek/scoop-bucket` repository **MUST** include a `README.md` with usage instructions.

### Non-Functional Requirements

1. No changes to the existing archive naming convention, checksum format, or SBOM generation.
2. No changes to the existing MCPB extension packaging.
3. The GoReleaser configuration **MUST** continue to pass `goreleaser check`.
4. The `make ci` target **MUST** continue to pass (includes `goreleaser-check`).
5. Formula and manifest files are generated by GoReleaser, not hand-maintained — no manual updates are required after initial setup.

## Affected Components

| Component | Change |
|-----------|--------|
| `.goreleaser.yaml` | Add `brews` and `scoops` sections |
| `.github/workflows/release.yml` | Add post-release publish step for formula/manifest |
| `README.md` | Add Homebrew and Scoop installation instructions |
| `desek/homebrew-tap` (new repo) | New repository with `Formula/` directory and `README.md` |
| `desek/scoop-bucket` (new repo) | New repository with `bucket/` directory and `README.md` |
| GitHub repository secrets | Add `HOMEBREW_TAP_TOKEN` and `SCOOP_BUCKET_TOKEN` |

## Scope Boundaries

### In Scope

* Creating `desek/homebrew-tap` and `desek/scoop-bucket` repositories.
* Adding `brews` and `scoops` sections to `.goreleaser.yaml`.
* Adding a post-release publish step to the release workflow.
* Setting up GitHub PAT secrets for cross-repo write access.
* Updating `README.md` with installation instructions.
* Verifying end-to-end install on macOS (Homebrew) and Windows (Scoop).

### Out of Scope ("Here, But Not Further")

* **Homebrew core submission** — submitting to the official `homebrew/homebrew-core` repository requires meeting Homebrew's notability and CI requirements. The tap approach is sufficient for the project's current adoption level. Core submission is a future consideration.
* **Scoop extras submission** — similar to Homebrew core, submitting to `ScoopInstaller/Extras` is deferred.
* **Chocolatey package** — Windows also has Chocolatey, but Scoop is lower friction (no admin rights) and aligns with the project's developer-focused audience.
* **Linux package managers (apt, dnf, pacman)** — deferred. Homebrew on Linux covers the primary use case. Native `.deb`/`.rpm` packaging is a future CR.
* **Artifact signing (cosign/GPG)** — deferred per CR-0036. Homebrew and Scoop do not require signed archives; checksums are sufficient.
* **Shell completions** — GoReleaser can generate and distribute shell completions via the formula's `extra_install`. Deferred to a future enhancement.
* **darwin/amd64 build target** — Intel Mac support is not currently built (ignored in `.goreleaser.yaml`). Homebrew formulas support architecture-specific URLs, so when/if darwin/amd64 is added, the formula picks it up automatically.
* **linux/arm64 desktop build** — currently only built for containers (`container` build ID, CGO_ENABLED=0). Adding a desktop `linux/arm64` binary is a separate concern.
* **Refactoring the release workflow to use `goreleaser release`** — the current hybrid workflow (matrix build + manual archive) works but prevents GoReleaser's `brews`/`scoops` publishers from running natively. A full refactor to invoke `goreleaser release --clean` as a single step would simplify everything but is a larger change deferred to a separate CR.

## Alternative Approaches Considered

* **Invoke `goreleaser release --clean` in the release workflow.** This would let GoReleaser handle archives, checksums, formula, and manifest generation in one step. Rejected for this CR because the current workflow is a 3-job matrix build (native CGO on each platform) that cannot be replaced by a single GoReleaser invocation without solving cross-compilation with CGO or switching to a self-hosted runner matrix. The current hybrid approach works; adding a publish step is simpler.

* **Template-based formula/manifest generation without GoReleaser.** Write a shell script that renders `outlook-local-mcp.rb` and `outlook-local-mcp.json` from templates using `sed`/`envsubst` and commits them via `gh api`. Rejected because GoReleaser already has first-class `brews` and `scoops` support, and maintaining templates manually is error-prone when archive naming or checksums change.

* **Single repository for both tap and bucket.** Combine Homebrew formula and Scoop manifest in one repo (e.g., `desek/packages`). Rejected because Homebrew requires the repository name to match `homebrew-*` for `brew tap` to work, and Scoop buckets have their own naming convention. Separate repos follow ecosystem conventions.

## Impact Assessment

### User Impact

After this change, users gain two new installation methods:

**macOS / Linux:**
```bash
brew install desek/tap/outlook-local-mcp
```

**Windows:**
```powershell
scoop bucket add desek https://github.com/desek/scoop-bucket
scoop install outlook-local-mcp
```

Both methods handle download, checksum verification, binary placement, and PATH configuration automatically. Upgrades are a single command (`brew upgrade` / `scoop update`).

### Technical Impact

* **GoReleaser config**: Two new sections (`brews`, `scoops`). No changes to existing build, archive, or checksum configuration.
* **Release workflow**: One additional step after the existing `Upload release assets` step. Failure is non-blocking.
* **Repository secrets**: Two new secrets. PATs must be rotated periodically (GitHub fine-grained PATs support expiration).
* **External repositories**: Two new public repositories under the `desek` GitHub account.

### Business Impact

Lowers the barrier to adoption by providing the install experience users expect from CLI tools. Homebrew and Scoop are the standard distribution channels for developer tools.

## Implementation Approach

### Phase 1: Create External Repositories

Create two new GitHub repositories:

**`desek/homebrew-tap`:**
```
homebrew-tap/
  Formula/
    .gitkeep
  README.md
```

README contents:
```markdown
# Homebrew Tap for desek

## Install

brew install desek/tap/outlook-local-mcp

## Update

brew upgrade outlook-local-mcp
```

**`desek/scoop-bucket`:**
```
scoop-bucket/
  bucket/
    .gitkeep
  README.md
```

README contents:
```markdown
# Scoop Bucket for desek

## Install

scoop bucket add desek https://github.com/desek/scoop-bucket
scoop install outlook-local-mcp

## Update

scoop update outlook-local-mcp
```

### Phase 2: Configure Repository Secrets

Create a GitHub fine-grained PAT with:
- **Resource owner**: `desek`
- **Repository access**: `desek/homebrew-tap` and `desek/scoop-bucket`
- **Permissions**: Contents (read and write)
- **Expiration**: 90 days (set a calendar reminder to rotate)

Add as repository secrets on `desek/outlook-local-mcp`:
- `HOMEBREW_TAP_TOKEN` → the PAT value
- `SCOOP_BUCKET_TOKEN` → the PAT value (can be the same PAT)

### Phase 3: Add `brews` and `scoops` to `.goreleaser.yaml`

Append the following sections to `.goreleaser.yaml`:

```yaml
brews:
  - ids:
      - binaries
    repository:
      owner: desek
      name: homebrew-tap
      token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"
    directory: Formula
    homepage: https://github.com/desek/outlook-local-mcp
    description: "MCP server for Microsoft Outlook — calendar, mail, and account management via Microsoft Graph API"
    license: MIT
    install: |
      bin.install "outlook-local-mcp"
    test: |
      assert_match "outlook-local-mcp", shell_output("#{bin}/outlook-local-mcp --version 2>&1", 1)
    commit_author:
      name: "github-actions[bot]"
      email: "github-actions[bot]@users.noreply.github.com"

scoops:
  - ids:
      - binaries
    repository:
      owner: desek
      name: scoop-bucket
      token: "{{ .Env.SCOOP_BUCKET_TOKEN }}"
    directory: bucket
    homepage: https://github.com/desek/outlook-local-mcp
    description: "MCP server for Microsoft Outlook — calendar, mail, and account management via Microsoft Graph API"
    license: MIT
    commit_author:
      name: "github-actions[bot]"
      email: "github-actions[bot]@users.noreply.github.com"
```

Run `goreleaser check` to validate.

### Phase 4: Add Publish Step to Release Workflow

Since the current workflow does not invoke `goreleaser release`, the `brews` and `scoops` publishers will not run automatically. Add a dedicated publish step that uses the GoReleaser-generated template approach or a script that:

1. Reads the tag name and version from the release-please output.
2. Downloads `checksums.txt` from the GitHub Release.
3. Parses SHA-256 hashes for each archive.
4. Renders the Homebrew formula from a template.
5. Renders the Scoop manifest from a template.
6. Commits the formula to `desek/homebrew-tap` and the manifest to `desek/scoop-bucket`.

Add to `.github/workflows/release.yml` after the `Upload release assets` step:

```yaml
- name: Publish Homebrew formula
  env:
    GH_TOKEN: ${{ secrets.HOMEBREW_TAP_TOKEN }}
  run: |
    VERSION="${{ needs.release-please.outputs.tag_name }}"
    VERSION="${VERSION#v}"

    # Download checksums from the release
    gh release download "${{ needs.release-please.outputs.tag_name }}" \
      --pattern checksums.txt --dir /tmp/release

    # Extract SHA256 for each platform
    DARWIN_ARM64_SHA=$(grep "darwin-arm64.tar.gz" /tmp/release/checksums.txt | awk '{print $1}')
    LINUX_AMD64_SHA=$(grep "linux-amd64.tar.gz" /tmp/release/checksums.txt | awk '{print $1}')

    # Generate formula
    cat > /tmp/outlook-local-mcp.rb << FORMULA
    class OutlookLocalMcp < Formula
      desc "MCP server for Microsoft Outlook — calendar, mail, and account management via Microsoft Graph API"
      homepage "https://github.com/desek/outlook-local-mcp"
      version "${VERSION}"
      license "MIT"

      on_macos do
        on_arm do
          url "https://github.com/desek/outlook-local-mcp/releases/download/v${VERSION}/outlook-local-mcp-darwin-arm64.tar.gz"
          sha256 "${DARWIN_ARM64_SHA}"
        end
      end

      on_linux do
        on_intel do
          url "https://github.com/desek/outlook-local-mcp/releases/download/v${VERSION}/outlook-local-mcp-linux-amd64.tar.gz"
          sha256 "${LINUX_AMD64_SHA}"
        end
      end

      def install
        bin.install "outlook-local-mcp"
      end

      test do
        assert_match "outlook-local-mcp", shell_output("#{bin}/outlook-local-mcp --version 2>&1", 1)
      end
    end
    FORMULA

    # Clone tap, commit, push
    git clone "https://x-access-token:${GH_TOKEN}@github.com/desek/homebrew-tap.git" /tmp/homebrew-tap
    cp /tmp/outlook-local-mcp.rb /tmp/homebrew-tap/Formula/outlook-local-mcp.rb
    cd /tmp/homebrew-tap
    git config user.name "github-actions[bot]"
    git config user.email "github-actions[bot]@users.noreply.github.com"
    git add Formula/outlook-local-mcp.rb
    git commit -m "outlook-local-mcp ${VERSION}"
    git push

- name: Publish Scoop manifest
  env:
    GH_TOKEN: ${{ secrets.SCOOP_BUCKET_TOKEN }}
  run: |
    VERSION="${{ needs.release-please.outputs.tag_name }}"
    VERSION="${VERSION#v}"

    # Download checksums from the release
    gh release download "${{ needs.release-please.outputs.tag_name }}" \
      --pattern checksums.txt --dir /tmp/release

    # Extract SHA256 for Windows
    WINDOWS_AMD64_SHA=$(grep "windows-amd64.zip" /tmp/release/checksums.txt | awk '{print $1}')

    # Generate Scoop manifest
    cat > /tmp/outlook-local-mcp.json << MANIFEST
    {
      "version": "${VERSION}",
      "description": "MCP server for Microsoft Outlook — calendar, mail, and account management via Microsoft Graph API",
      "homepage": "https://github.com/desek/outlook-local-mcp",
      "license": "MIT",
      "architecture": {
        "64bit": {
          "url": "https://github.com/desek/outlook-local-mcp/releases/download/v${VERSION}/outlook-local-mcp-windows-amd64.zip",
          "hash": "${WINDOWS_AMD64_SHA}"
        }
      },
      "bin": "outlook-local-mcp.exe",
      "checkver": {
        "github": "https://github.com/desek/outlook-local-mcp"
      },
      "autoupdate": {
        "architecture": {
          "64bit": {
            "url": "https://github.com/desek/outlook-local-mcp/releases/download/v\$version/outlook-local-mcp-windows-amd64.zip"
          }
        }
      }
    }
    MANIFEST

    # Clone bucket, commit, push
    git clone "https://x-access-token:${GH_TOKEN}@github.com/desek/scoop-bucket.git" /tmp/scoop-bucket
    cp /tmp/outlook-local-mcp.json /tmp/scoop-bucket/bucket/outlook-local-mcp.json
    cd /tmp/scoop-bucket
    git config user.name "github-actions[bot]"
    git config user.email "github-actions[bot]@users.noreply.github.com"
    git add bucket/outlook-local-mcp.json
    git commit -m "outlook-local-mcp ${VERSION}"
    git push
```

Both steps use `continue-on-error: true` semantics — if formula/manifest publish fails, the release itself (archives + MCPB) is already complete.

### Phase 5: Update README

Add installation instructions to `README.md`:

```markdown
## Installation

### Homebrew (macOS / Linux)

```bash
brew install desek/tap/outlook-local-mcp
```

### Scoop (Windows)

```powershell
scoop bucket add desek https://github.com/desek/scoop-bucket
scoop install outlook-local-mcp
```

### Go Install

```bash
go install github.com/desek/outlook-local-mcp/cmd/outlook-local-mcp@latest
```

### Manual Download

Download the latest release from [GitHub Releases](https://github.com/desek/outlook-local-mcp/releases).
```

### Phase 6: Validate

1. Run `goreleaser check` to validate the updated `.goreleaser.yaml`.
2. Run `make ci` to verify all quality checks pass.
3. Run `make snapshot` to verify the formula and manifest are generated correctly in the snapshot (they will reference local paths, but the structure is validated).
4. After merging, trigger a release and verify:
   - Formula appears in `desek/homebrew-tap/Formula/outlook-local-mcp.rb`.
   - Manifest appears in `desek/scoop-bucket/bucket/outlook-local-mcp.json`.
   - `brew install desek/tap/outlook-local-mcp` succeeds on macOS.
   - `scoop install outlook-local-mcp` succeeds on Windows (after adding bucket).

### Implementation Flow

```mermaid
flowchart LR
    subgraph Phase1["Phase 1: Repositories"]
        A1["desek/homebrew-tap"] --> A2["desek/scoop-bucket"]
    end
    subgraph Phase2["Phase 2: Secrets"]
        B1["HOMEBREW_TAP_TOKEN"] --> B2["SCOOP_BUCKET_TOKEN"]
    end
    subgraph Phase3["Phase 3: GoReleaser"]
        C1["brews section"] --> C2["scoops section"]
    end
    subgraph Phase4["Phase 4: Workflow"]
        D1["Publish Homebrew\nformula step"] --> D2["Publish Scoop\nmanifest step"]
    end
    subgraph Phase5["Phase 5: README"]
        E1["Installation\ninstructions"]
    end
    subgraph Phase6["Phase 6: Validate"]
        F1["goreleaser check\nmake ci\nend-to-end install"]
    end
    Phase1 --> Phase2 --> Phase3 --> Phase4 --> Phase5 --> Phase6
```

## Test Strategy

### Tests to Add

This CR does not introduce Go code changes, so no unit tests are added. Validation is performed through integration testing:

| Test | Method | Inputs | Expected Output |
|------|--------|--------|-----------------|
| GoReleaser config validation | `goreleaser check` | Updated `.goreleaser.yaml` | Exit code 0, no errors |
| CI pipeline validation | `make ci` | Full codebase | All checks pass including `goreleaser-check` |
| Snapshot formula generation | `make snapshot` | Local build | Formula file generated in `dist/` |
| Snapshot manifest generation | `make snapshot` | Local build | Manifest file generated in `dist/` |
| Homebrew install (macOS) | `brew install desek/tap/outlook-local-mcp` | Post-release | Binary installed to PATH, `outlook-local-mcp --version` works |
| Scoop install (Windows) | `scoop install outlook-local-mcp` | Post-release, bucket added | Binary installed to PATH, `outlook-local-mcp --version` works |
| Homebrew upgrade | `brew upgrade outlook-local-mcp` | After a second release | New version installed |
| Scoop update | `scoop update outlook-local-mcp` | After a second release | New version installed |

### Tests to Modify

Not applicable — no existing tests are affected.

### Tests to Remove

Not applicable.

## Acceptance Criteria

### AC-1: Homebrew formula published on release

```gherkin
Given a new release is created via release-please
When the release workflow completes
Then desek/homebrew-tap contains Formula/outlook-local-mcp.rb
  And the formula version matches the release tag
  And the formula contains SHA-256 checksums for darwin-arm64 and linux-amd64 archives
```

### AC-2: Homebrew install succeeds on macOS

```gherkin
Given the formula is published in desek/homebrew-tap
When a user runs "brew install desek/tap/outlook-local-mcp" on macOS (arm64)
Then the binary is installed to the Homebrew bin directory
  And "outlook-local-mcp --version" outputs the release version
```

### AC-3: Scoop manifest published on release

```gherkin
Given a new release is created via release-please
When the release workflow completes
Then desek/scoop-bucket contains bucket/outlook-local-mcp.json
  And the manifest version matches the release tag
  And the manifest contains the SHA-256 checksum for the windows-amd64 archive
```

### AC-4: Scoop install succeeds on Windows

```gherkin
Given the manifest is published in desek/scoop-bucket
  And a user has added the bucket: "scoop bucket add desek https://github.com/desek/scoop-bucket"
When the user runs "scoop install outlook-local-mcp"
Then the binary is installed to the Scoop shims directory
  And "outlook-local-mcp --version" outputs the release version
```

### AC-5: GoReleaser config passes validation

```gherkin
Given the updated .goreleaser.yaml with brews and scoops sections
When "goreleaser check" is executed
Then the exit code is 0
  And no validation errors are reported
```

### AC-6: CI pipeline passes

```gherkin
Given the updated .goreleaser.yaml
When "make ci" is executed
Then all checks pass including goreleaser-check
```

### AC-7: README includes installation instructions

```gherkin
Given the updated README.md
When a user reads the Installation section
Then it includes Homebrew, Scoop, go install, and manual download instructions
```

### AC-8: Formula/manifest publish failure does not block release

```gherkin
Given the release workflow is running
  And the Homebrew or Scoop publish step fails (e.g., PAT expired)
When the workflow completes
Then the GitHub Release assets (archives, checksums, MCPB) are still published
  And the overall workflow does not report failure
```

## Quality Standards Compliance

### Build & Compilation

- [ ] `goreleaser check` passes with updated config
- [ ] `make ci` passes

### Linting & Code Style

- [ ] No Go code changes; linting not applicable

### Test Execution

- [ ] End-to-end Homebrew install verified on macOS
- [ ] End-to-end Scoop install verified on Windows
- [ ] `make snapshot` generates formula and manifest

### Documentation

- [ ] README.md updated with installation instructions
- [ ] Tap repository README created
- [ ] Bucket repository README created

### Code Review

- [ ] Changes submitted via pull request
- [ ] PR title follows Conventional Commits format
- [ ] Code review completed and approved
- [ ] Changes squash-merged to maintain linear history

### Verification Commands

```bash
# GoReleaser validation
goreleaser check

# Full CI pipeline
make ci

# Local snapshot (generates formula/manifest in dist/)
make snapshot
```

## Risks and Mitigation

### Risk 1: GoReleaser `brews`/`scoops` sections ignored in hybrid workflow

**Likelihood:** high
**Impact:** medium
**Mitigation:** The current release workflow does not invoke `goreleaser release`, so the `brews` and `scoops` publishers will not run automatically. Phase 4 addresses this with explicit publish steps that generate and commit the formula/manifest independently. The `brews` and `scoops` sections in `.goreleaser.yaml` serve as documentation and are validated by `goreleaser check`. If the workflow is later refactored to use `goreleaser release`, the sections become active automatically.

### Risk 2: PAT expiration breaks formula/manifest publishing

**Likelihood:** high (PATs expire)
**Impact:** low
**Mitigation:** Formula/manifest publishing is non-blocking — the release itself (archives + MCPB) succeeds regardless. The PAT expiration is detected in the workflow logs. A calendar reminder to rotate PATs mitigates this. Fine-grained PATs support up to 1-year expiration.

### Risk 3: Homebrew formula fails `brew audit`

**Likelihood:** medium
**Impact:** low
**Mitigation:** `brew audit --new-formula Formula/outlook-local-mcp.rb` can be run locally before the first release. Common issues (missing `test` block, incorrect `license` identifier, missing `desc`) are addressed in the formula template. This audit is for Homebrew core submission readiness; tap formulas have looser requirements.

### Risk 4: Archive naming changes break formula/manifest

**Likelihood:** low
**Impact:** medium
**Mitigation:** The formula and manifest reference archive names that match the GoReleaser `name_template` (`outlook-local-mcp-{os}-{arch}.{ext}`). If the naming convention changes, the publish step's `grep` patterns must be updated. This is documented in the implementation approach.

### Risk 5: Scoop manifest `checkver` fails for pre-release versions

**Likelihood:** low
**Impact:** low
**Mitigation:** The `checkver.github` property uses the GitHub API to detect the latest release. Pre-release versions (0.x) are valid release versions in GitHub's model. Scoop's `autoupdate` mechanism handles version string substitution automatically.

## Dependencies

* **External repositories**: `desek/homebrew-tap` and `desek/scoop-bucket` must be created before the first release that uses this configuration.
* **GitHub PATs**: Must be created and stored as repository secrets before the first release.
* **GoReleaser 2.x**: Already installed via mise (version 2, per `.mise.toml`). The `brews` and `scoops` sections are available in the free edition.
* **No Go dependency changes**: `go.mod` is unchanged.

## Estimated Effort

4–6 person-hours, distributed as:

| Phase | Effort |
|-------|--------|
| Phase 1: Create repositories | 30 minutes |
| Phase 2: Configure secrets | 15 minutes |
| Phase 3: GoReleaser config | 30 minutes |
| Phase 4: Release workflow | 1–2 hours |
| Phase 5: README update | 15 minutes |
| Phase 6: Validate (end-to-end) | 1–2 hours |

## Decision Outcome

Chosen approach: "GoReleaser `brews`/`scoops` config with explicit publish steps in the release workflow", because it uses GoReleaser's first-class package manager support for configuration and validation while working within the constraints of the current hybrid release workflow. The explicit publish steps generate formula/manifest files using release checksums and commit them to separate tap/bucket repositories, providing the standard install experience for both macOS/Linux (Homebrew) and Windows (Scoop) users.

## Related Items

* CR-0036: GoReleaser Integration — established the release automation infrastructure and explicitly deferred Homebrew tap and Scoop manifest.
* CR-0028: Release Tooling and Quality Automation — established release-please, Makefile, and CI pipeline.
* CR-0027: Open Source Scaffolding — established the MIT license referenced in formula/manifest.
