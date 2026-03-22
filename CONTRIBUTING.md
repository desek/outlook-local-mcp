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
- Node.js (required by commitlint)
- pre-commit

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
