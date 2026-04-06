---
name: checkpoint
description: Reference guide for creating iterative Git checkpoints to preserve work-in-progress state during development.
metadata:
  copyright: Copyright Daniel Grenemark 2026
  version: "0.0.1"
---

# Checkpoint Instruction

Create iterative Git checkpoints to preserve work-in-progress state on active branches. Checkpoints serve as short-term memory during development and can be compressed into merge commits for long-term memory.

## Checkpoint Workflow

### Step 1: Analyze Changes

Examine all changes in the repository:

```bash
# Staged changes
git diff --staged

# Unstaged changes
git diff

# Untracked files
git ls-files --others --exclude-standard
```

### Step 2: Update .gitignore

Before staging, review and update `.gitignore` to exclude:

- Temporary files (`.tmp`, `.bak`, `*.log`)
- Build artifacts (`dist/`, `build/`, `node_modules/`)
- Generated content (compiled files, cache directories)
- IDE-specific files not already covered

**This step is mandatory** to prevent repository bloat.

### Step 3: Write Summary

Create a commit message with:

- **Subject line**: One-sentence summary of changes
- **Body**: Detailed description with multiple sentences or bullet points explaining what changed and why

### Step 4: Stage All Changes

```bash
git add -A
```

### Step 5: Create Checkpoint Commit

```bash
git commit -m "checkpoint(CR-xxxx): {summary}

{detailed body with bullet points or sentences}"
```

Replace `CR-xxxx` with the relevant CR number for the current work.

## Commit Message Format

```
checkpoint(CR-xxxx): {one-sentence summary}

- {change 1}
- {change 2}
- {change 3}
```

## Squash Merge Output

When ready to merge the branch, generate a long-term memory message by semantically compressing all checkpoint commits:

### Step 1: List Checkpoint Commits

```bash
git log --oneline --grep="^checkpoint" main..HEAD
```

### Step 2: Generate Compressed Summary

Combine all checkpoint commits into a single coherent message:

- Extract key changes from each checkpoint
- Remove redundant or superseded changes
- Organize by theme or component
- Write a clear summary suitable for PR description or squash merge commit

### Output Format

```
{type}(scope): {overall summary}

{Coherent description combining all checkpoint changes}

- {Major change 1}
- {Major change 2}
- {Major change 3}
```

This compressed message serves as long-term project memory and can be copied directly into PR descriptions or used for local squash merge commits.

## Safety Rules

- **DO NOT** perform destructive Git operations (reset, rebase, amend, force push)
- **DO** preserve all work without data loss
- **DO** ensure idempotent operations safe to run multiple times
