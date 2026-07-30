# Contributing to outlook-local-mcp

## Reporting Bugs

Use GitHub Issues to report bugs. Please include:

- Steps to reproduce the issue
- Expected behavior
- Actual behavior

## Suggesting Features

Use GitHub Issues with a feature request description.

## Development Setup

### Prerequisites

- Go 1.24+
- golangci-lint
- Node.js 22+ and pnpm 11.18.0 (required for the website in `site/`; see below)
- pre-commit

The Go and Node toolchains are independent. Contributors touching only Go do not
need Node; contributors touching `site/` need Node and pnpm. Tool versions are
pinned in `.mise.toml` (`pnpm = "11.18.0"`), so `mise install` provisions the whole
toolchain. `site/package.json` also carries a `packageManager` pin that agrees with
it, which is what corepack and CI resolve pnpm from.

### Pre-commit Hooks

This project uses [pre-commit](https://pre-commit.com/) to run quality checks before each commit.

#### Install pre-commit

```bash
# macOS
brew install pre-commit

# pip
pip install pre-commit
```

#### Enable hooks

```bash
pre-commit install --hook-type pre-commit --hook-type commit-msg
```

#### Run hooks manually

```bash
pre-commit run --all-files
```

### Build

```bash
make build
```

### Test

```bash
make test
```

### Lint

```bash
make lint
```

### Full Quality Check

```bash
make ci
```

### Validate GoReleaser Configuration

```bash
make goreleaser-check
```

### Local Release Snapshot

Build cross-compiled binaries locally without publishing (outputs to `dist/`):

```bash
make snapshot
```

### Vulnerability Scan

```bash
make vuln-scan
```

### License Check

```bash
make license-check
```

### MCPB Extension Packaging

```bash
make build-mcpb-binaries   # Run snapshot build and copy binaries for MCPB platforms
make mcpb-pack              # Build binaries, validate manifest, and pack .mcpb bundle
make mcpb-clean             # Remove extension/bin/ and *.mcpb artifacts
```

## Website (`site/`)

The project website lives in `site/`, a pnpm-managed React and Vite project. Use
pnpm, not npm.

```bash
pnpm --dir site install --frozen-lockfile   # install dependencies
pnpm --dir site run dev                      # local dev server
pnpm --dir site run build                    # typecheck (tsc -b) and build to site/dist
pnpm --dir site run preview                  # serve the built site locally
```

`pnpm --dir site run build` runs `tsc -b` before `vite build`, so a type error fails
the build. This is the same gate CI enforces before publishing.

### Deployment is CI-managed

The site is deployed by the `Deploy site` GitHub Actions workflow
(`.github/workflows/deploy-site.yml`), which builds `site/` and publishes the output
to the `gh-pages` branch on every push to `main` that touches `site/` or `docs/`. It
uses only the default `GITHUB_TOKEN`.

**`gh-pages` is a CI-managed build artifact and MUST NOT be hand-edited.** Any manual
commit to that branch is reverted by the next deploy. `CNAME`, `robots.txt`,
`sitemap.xml`, and `llms.txt` are build outputs inside `dist/`, not files placed on
the branch by hand, so they cannot be lost to a rebuild. To change what the site
serves, change the source under `site/` (or `docs/`) and merge to `main`.

## Code Standards

- New code MUST be placed in the appropriate `internal/` package
- All exported symbols MUST have Go doc comments
- Follow SOLID design principles
- See the project's CLAUDE.md for detailed conventions

## Submitting Changes

### Workflow

1. Fork the repository
2. Create a feature branch from `main`
3. Make your changes
4. Run the full quality check: `make ci`
5. Commit using Conventional Commits format
6. Open a pull request

### Commit Messages

This project uses [Conventional Commits](https://www.conventionalcommits.org/). Format:

```
type(scope): description
```

Common types: feat, fix, docs, style, refactor, test, chore

### Pull Requests

- PR titles MUST follow Conventional Commits format
- All PRs use squash merge only
- All quality checks MUST pass before merging

### Branch Protection

- Force pushes are blocked on protected branches
- Direct commits to `main` are prohibited
- All changes MUST go through a pull request
