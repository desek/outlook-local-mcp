---
name: outlook
description: Use when the user reads or triages Outlook mail, books a meeting or finds a free slot, attaches files to a draft, resolves a person to an email address, or reads Microsoft Teams messages or a meeting transcript, through the outlook-local-mcp server.
---

# Outlook

The outlook-local-mcp server exposes six domain tools. Each tool takes a required `operation` verb.

- `calendar`: events, availability, and schedules.
- `mail`: messages, folders, drafts, and attachments.
- `account`: the signed-in Microsoft accounts.
- `system`: server status and documentation.
- `contacts`: contacts and people. This domain is opt-in through `OUTLOOK_MCP_CONTACTS_ENABLED`.
- `teams`: Teams messages and meeting transcripts. This domain is opt-in through `OUTLOOK_MCP_TEAMS_ENABLED`.

Mail write verbs need `OUTLOOK_MCP_MAIL_MANAGE_ENABLED`.

## Rules

1. Call `operation="help"` on a domain before you guess a verb or a parameter.
2. Read the resource id from each write confirmation. Use that id in the next call.
3. `move_message` makes a new message id. Use the new id after a move.
4. `teams.compose_reply` returns text only. It sends nothing.
5. If `OUTLOOK_MCP_READ_ONLY` is set, the server refuses all write verbs. Tell the user.
