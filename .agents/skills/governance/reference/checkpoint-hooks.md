---
name: checkpoint-hooks
description: Guide for automating checkpoint creation using Claude Code hooks.
metadata:
  copyright: Copyright Daniel Grenemark 2026
  version: "0.0.1"
---

# Checkpoint Hooks Integration

Automate checkpoint creation using Claude Code prompt-based hooks. This guide covers hook configuration for triggering checkpoints at key lifecycle events.

## Overview

Claude Code prompt-based hooks (`type: "prompt"`) can automatically evaluate whether to trigger checkpoint creation. When the hook returns `{"ok": false, "reason": "..."}`, Claude follows the instruction in the reason field.

Recommended hook event:

- **Stop**: When Claude finishes responding - evaluates if checkpoint is needed

## Hook Configuration

Add hook configuration to your `.claude/settings.json`:

```json
{
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {
            "type": "prompt",
            "prompt": "You are evaluating whether Claude should create a checkpoint before stopping. Context: $ARGUMENTS\n\nCheck if:\n1. There are uncommitted changes in the repository\n2. The stop_hook_active field is false (to prevent infinite loops)\n\nIf stop_hook_active is true, respond with {\"ok\": true}.\nIf there are uncommitted changes and stop_hook_active is false, respond with {\"ok\": false, \"reason\": \"/checkpoint-commit\"}.\nIf there are no uncommitted changes, respond with {\"ok\": true}.",
            "timeout": 30
          }
        ]
      }
    ]
  }
}
```

## How Prompt-Based Hooks Work

1. The Stop event fires when Claude finishes responding
2. Claude Code sends the hook input (including `$ARGUMENTS`) to a fast Claude model
3. The model evaluates the prompt and returns a JSON decision
4. If `{"ok": false, "reason": "..."}`, Claude continues working with the reason as instruction
5. If `{"ok": true}`, Claude stops normally

## JSON Input Fields

The `$ARGUMENTS` placeholder is replaced with the hook's JSON input:

```json
{
  "session_id": "abc123",
  "stop_hook_active": false,
  "transcript_path": "/path/to/transcript.json"
}
```

**Important:** The `stop_hook_active` field prevents infinite loops. When `true`, the hook was triggered by a previous Stop hook response.

## Response Schema

The prompt hook must respond with JSON:

```json
{
  "ok": true | false,
  "reason": "Explanation (required when ok is false)"
}
```

| Field | Description |
|-------|-------------|
| `ok` | `true` allows Claude to stop, `false` continues working |
| `reason` | Required when `ok` is `false`. Instruction shown to Claude |

## Best Practices

### Infinite Loop Prevention

Always check `stop_hook_active` in your prompt to prevent infinite loops:

```
If stop_hook_active is true, respond with {"ok": true}.
```

This ensures the hook allows stopping when triggered by a previous checkpoint action.

### Timeout Configuration

Set appropriate timeouts (in seconds for prompt hooks):

```json
{
  "type": "prompt",
  "prompt": "...",
  "timeout": 30
}
```

Default timeout is 30 seconds for prompt hooks.

### Seamless Handover Protocol

The prompt's reason field should return a checkpoint instruction directly:

```
"reason": "/checkpoint-commit"
```

This seamless handover ensures Claude executes the checkpoint workflow directly, including `.gitignore` maintenance.

## See Also

- [Checkpoint Instruction](checkpoint.md) - Checkpoint workflow reference
- [Claude Code Hooks Reference](https://docs.anthropic.com/en/docs/claude-code/hooks) - Official documentation
