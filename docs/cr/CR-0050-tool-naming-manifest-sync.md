---
id: "CR-0050"
status: "completed"
date: 2026-03-22
completed-date: 2026-03-22
requestor: Daniel Grenemark
stakeholders:
  - Daniel Grenemark
priority: "high"
target-version: "2.0.0"
source-branch: dev/cc-swarm
source-commit: bb453b6
---

# Domain-Prefixed Tool Names and Manifest Synchronization

## Change Summary

Rename all MCP tools to use a domain prefix (`calendar_`, `mail_`, `account_`) followed by the operation, add 6 missing tools to `extension/manifest.json`, and update all internal references, cross-references in tool descriptions, and test documentation. This is a **breaking change** to tool names, targeting the 2.0.0 major release.

## Motivation and Background

Three problems motivate this change:

1. **Tool discoverability** -- With 20 tools spanning three domains (calendar, mail, account management), flat names like `list_events`, `list_messages`, and `list_accounts` force the LLM to infer context from descriptions alone. Domain prefixes create a natural namespace that groups related tools and reduces selection ambiguity.

2. **Manifest drift** -- Six tools registered in `server.go` are missing from `extension/manifest.json`: `respond_event`, `reschedule_event`, `list_mail_folders`, `list_messages`, `search_messages`, and `get_message`. Claude Desktop cannot discover these tools, meaning users of the MCPB extension package are missing functionality.

3. **Public release readiness** -- Before the initial public release, tool names should follow a consistent, self-documenting convention. Renaming after release would be a breaking change for existing users; doing it now (pre-release) has zero migration cost.

## Current State

### Manifest vs Server Registration Gap

| Tool | In `server.go` | In `manifest.json` | Status |
|------|:-:|:-:|--------|
| `list_calendars` | Yes | Yes | OK |
| `list_events` | Yes | Yes | OK |
| `get_event` | Yes | Yes | OK |
| `search_events` | Yes | Yes | OK |
| `get_free_busy` | Yes | Yes | OK |
| `create_event` | Yes | Yes | OK |
| `update_event` | Yes | Yes | OK |
| `delete_event` | Yes | Yes | OK |
| `cancel_event` | Yes | Yes | OK |
| `respond_event` | Yes | **No** | **MISSING** |
| `reschedule_event` | Yes | **No** | **MISSING** |
| `add_account` | Yes | Yes | OK |
| `list_accounts` | Yes | Yes | OK |
| `remove_account` | Yes | Yes | OK |
| `status` | Yes | Yes | OK |
| `list_mail_folders` | Yes (conditional) | **No** | **MISSING** |
| `list_messages` | Yes (conditional) | **No** | **MISSING** |
| `search_messages` | Yes (conditional) | **No** | **MISSING** |
| `get_message` | Yes (conditional) | **No** | **MISSING** |
| `complete_auth` | Yes (conditional) | Yes | OK |

### Current Naming Convention

Tool names use `verb_noun` without domain context. When the server had only calendar tools this was unambiguous, but the addition of mail tools (CR-0043) and account tools (CR-0025) created naming collisions in semantics (e.g., `list_events` vs `list_messages` vs `list_accounts` -- all "list" operations on different domains).

## Proposed Change

### Tool Name Mapping

#### Calendar Domain (11 tools)

| Current Name | New Name | Registration |
|---|---|---|
| `list_calendars` | `calendar_list` | Always |
| `list_events` | `calendar_list_events` | Always |
| `get_event` | `calendar_get_event` | Always |
| `search_events` | `calendar_search_events` | Always |
| `get_free_busy` | `calendar_get_free_busy` | Always |
| `create_event` | `calendar_create_event` | Always |
| `update_event` | `calendar_update_event` | Always |
| `delete_event` | `calendar_delete_event` | Always |
| `cancel_event` | `calendar_cancel_event` | Always |
| `respond_event` | `calendar_respond_event` | Always |
| `reschedule_event` | `calendar_reschedule_event` | Always |

#### Mail Domain (4 tools)

| Current Name | New Name | Registration |
|---|---|---|
| `list_mail_folders` | `mail_list_folders` | Conditional (`cfg.MailEnabled`) |
| `list_messages` | `mail_list_messages` | Conditional (`cfg.MailEnabled`) |
| `search_messages` | `mail_search_messages` | Conditional (`cfg.MailEnabled`) |
| `get_message` | `mail_get_message` | Conditional (`cfg.MailEnabled`) |

#### Account Domain (3 tools)

| Current Name | New Name | Registration |
|---|---|---|
| `add_account` | `account_add` | Always |
| `list_accounts` | `account_list` | Always |
| `remove_account` | `account_remove` | Always |

#### System Tools (2 tools -- unchanged)

| Current Name | New Name | Rationale |
|---|---|---|
| `status` | `status` | Domain-agnostic diagnostic; no prefix needed |
| `complete_auth` | `complete_auth` | Conditional auth utility; no domain affinity |

### Naming Convention

```
{domain}_{operation}[_{resource}]
```

- **domain**: `calendar`, `mail`, or `account`
- **operation**: verb (`list`, `get`, `search`, `create`, `update`, `delete`, `cancel`, `respond`, `reschedule`, `add`, `remove`)
- **resource** (optional): noun when the domain has multiple resource types (`events`, `folders`, `messages`); omitted when unambiguous (`calendar_list` lists calendars, `account_add` adds an account)

### Tool Description Cross-References

Several tool descriptions reference other tools by name. These must be updated:

| Tool | Description Fragment | Old Reference | New Reference |
|---|---|---|---|
| `calendar_delete_event` | "use cancel_event instead" | `cancel_event` | `calendar_cancel_event` |
| `calendar_cancel_event` | "use delete_event instead" | `delete_event` | `calendar_delete_event` |
| `mail_list_messages` | "Use list_mail_folders to discover folder IDs" | `list_mail_folders` | `mail_list_folders` |
| `mail_list_messages` | "use search_messages instead" | `search_messages` | `mail_search_messages` |
| `mail_search_messages` | "use list_messages instead" | `list_messages` | `mail_list_messages` |

### Manifest Description Updates

The manifest descriptions for existing tools are short summaries of the full tool descriptions, which is appropriate. For the 6 newly added tools, use the following concise descriptions:

| Tool | Manifest Description |
|---|---|
| `calendar_respond_event` | "Respond to a meeting invitation: accept, tentatively accept, or decline. Sends a response to the organizer." |
| `calendar_reschedule_event` | "Move an event to a new time, preserving its original duration. Sends update notifications to attendees if applicable." |
| `mail_list_folders` | "List the user's mail folders (Inbox, Sent Items, Drafts, etc.) with display name, unread count, and total count." |
| `mail_list_messages` | "List email messages in a mail folder or across all folders. Supports filtering by date range, sender, and conversation ID." |
| `mail_search_messages` | "Full-text search across email messages using Microsoft Graph KQL syntax. Returns messages ranked by relevance." |
| `mail_get_message` | "Get full details of a single email message by its ID. Returns body content, all recipients, headers, and attachment metadata." |

### Default Client ID

The `user_config.client_id.default` in `manifest.json` **MUST** remain `"outlook-desktop"`. Verified: it is currently set correctly. No change needed.

## Requirements

### Functional Requirements

1. All calendar tools **MUST** be registered with the `calendar_` prefix in `mcp.NewTool()`, `server.go` name strings (`wrap`/`wrapWrite`, `WithObservability`, `AuditWrap`), and `manifest.json`.
2. All mail tools **MUST** be registered with the `mail_` prefix in `mcp.NewTool()`, `server.go` name strings (`wrap`/`wrapWrite`, `WithObservability`, `AuditWrap`), and `manifest.json`.
3. All account tools **MUST** be registered with the `account_` prefix in `mcp.NewTool()`, `server.go` name strings (`WithObservability`, `AuditWrap`), and `manifest.json`.
4. The `status` and `complete_auth` tools **MUST** retain their current names (no prefix).
5. All 20 tools registered in `server.go` **MUST** have a corresponding entry in `extension/manifest.json`.
6. Tool descriptions that cross-reference other tools by name **MUST** use the new prefixed names.
7. The `user_config.client_id.default` in `manifest.json` **MUST** be `"outlook-desktop"`.
8. The tool name string in `mcp.NewTool()` **MUST** match the name in all `server.go` middleware calls for that tool (`wrap()`/`wrapWrite()` for calendar/mail tools, or direct `observability.WithObservability()` and `audit.AuditWrap()` calls for account/system tools).
9. The `docs/prompts/mcp-tool-crud-test.md` **MUST** be updated to use the new tool names in all step instructions.
10. The `AGENTS.md` tool count **MUST** be updated to reflect the actual number of tools (20 total: 11 calendar + 4 mail + 3 account + 2 system).
11. `AGENTS.md` **MUST** include a **Tool Naming Convention** section documenting the `{domain}_{operation}[_{resource}]` pattern, the recognized domain prefixes (`calendar_`, `mail_`, `account_`), and the rule that new tools must follow this convention.

### Non-Functional Requirements

1. File names in `internal/tools/` **MUST NOT** be renamed. The Go package already provides namespace context; file renames would add churn without value.
2. All existing tests **MUST** pass after the rename, with tool name assertions updated to match the new names.
3. The manifest tool ordering **MUST** group tools by domain: calendar, mail, account, system.

## Affected Components

| Component | Change |
|---|---|
| `internal/tools/list_calendars.go` | Rename `"list_calendars"` to `"calendar_list"` in `mcp.NewTool()` |
| `internal/tools/list_events.go` | Rename `"list_events"` to `"calendar_list_events"` in `mcp.NewTool()` |
| `internal/tools/get_event.go` | Rename `"get_event"` to `"calendar_get_event"` in `mcp.NewTool()` |
| `internal/tools/search_events.go` | Rename `"search_events"` to `"calendar_search_events"` in `mcp.NewTool()` |
| `internal/tools/get_free_busy.go` | Rename `"get_free_busy"` to `"calendar_get_free_busy"` in `mcp.NewTool()` |
| `internal/tools/create_event.go` | Rename `"create_event"` to `"calendar_create_event"` in `mcp.NewTool()` |
| `internal/tools/update_event.go` | Rename `"update_event"` to `"calendar_update_event"` in `mcp.NewTool()` |
| `internal/tools/delete_event.go` | Rename `"delete_event"` to `"calendar_delete_event"`, update description cross-ref |
| `internal/tools/cancel_event.go` | Rename `"cancel_event"` to `"calendar_cancel_event"`, update description cross-ref |
| `internal/tools/respond_event.go` | Rename `"respond_event"` to `"calendar_respond_event"` in `mcp.NewTool()` |
| `internal/tools/reschedule_event.go` | Rename `"reschedule_event"` to `"calendar_reschedule_event"` in `mcp.NewTool()` |
| `internal/tools/list_mail_folders.go` | Rename `"list_mail_folders"` to `"mail_list_folders"` in `mcp.NewTool()` |
| `internal/tools/list_messages.go` | Rename `"list_messages"` to `"mail_list_messages"`, update description cross-refs (`list_mail_folders` and `search_messages`) |
| `internal/tools/search_messages.go` | Rename `"search_messages"` to `"mail_search_messages"`, update description cross-ref |
| `internal/tools/get_message.go` | Rename `"get_message"` to `"mail_get_message"` in `mcp.NewTool()` |
| `internal/tools/add_account.go` | Rename `"add_account"` to `"account_add"` in `mcp.NewTool()` |
| `internal/tools/list_accounts.go` | Rename `"list_accounts"` to `"account_list"` in `mcp.NewTool()` |
| `internal/tools/remove_account.go` | Rename `"remove_account"` to `"account_remove"` in `mcp.NewTool()` |
| `internal/tools/*.go` (all 18 domain tools) | Additionally update `slog.With("tool", ...)` string in each handler |
| `internal/server/server.go` | Update all `wrap()`/`wrapWrite()`/`WithObservability()`/`AuditWrap()` name strings and `toolCount` comment |
| `extension/manifest.json` | Rename all tools, add 6 missing tools, reorder by domain |
| `docs/prompts/mcp-tool-crud-test.md` | Update all `mcp__outlookCalendar__*` tool references |
| `AGENTS.md` | Update tool count from "12 MCP tool handlers (9 calendar + 3 account management)" to "20 MCP tool handlers (11 calendar + 4 mail + 3 account + 2 system)"; add Tool Naming Convention section |
| `internal/tools/*_test.go` | Update tool name assertions in tests that verify tool names |
| `internal/tools/tool_description_test.go` | Update expected tool names |

## Scope Boundaries

### In Scope

- Renaming all 18 domain tools with `calendar_`, `mail_`, `account_` prefixes
- Adding 6 missing tools to `extension/manifest.json`
- Updating all internal name strings (middleware, observability, audit, slog)
- Updating tool description cross-references
- Updating `mcp-tool-crud-test.md` with new tool names
- Updating `AGENTS.md` tool count and adding Tool Naming Convention section
- Reordering manifest tools by domain

### Out of Scope

- Renaming Go source files (file names stay as-is per NFR-1)
- Changing tool behavior, parameters, or response shapes
- Changing the `user_config` section of the manifest
- Adding new tools beyond the 20 already registered in `server.go`
- Backward-compatibility aliases for old tool names (clean break for 2.0.0)

## Impact Assessment

### User Impact

- **Breaking change**: All tool names change. Any saved Claude Desktop conversations referencing old tool names will not resolve. This is acceptable pre-release.
- **Improved discoverability**: LLMs can now group and reason about tools by domain prefix.

### Technical Impact

- **String-only change**: No struct, interface, or logic changes. Every modification is a string literal rename.
- **Test updates**: All tests asserting tool names must be updated. No logic changes in tests.
- **Observability continuity**: Metrics, traces, and audit logs will use the new tool names. Historical data will show old names. No migration needed since this is pre-release.

### Business Impact

- Establishes the public API tool naming convention before release. Prevents a breaking rename post-release.

## Implementation Approach

### Phase 1: Rename Tool Names in Source

For each of the 18 domain tools:
1. Update the `mcp.NewTool("old_name", ...)` call to use the new name.
2. Update the corresponding name string in `server.go`: `wrap()`/`wrapWrite()` for calendar and mail tools, or `WithObservability()`/`AuditWrap()` for account tools (which do not use `wrap`/`wrapWrite`).
3. Update the `slog.With("tool", "old_name")` call in each tool handler to use the new name.
4. Update cross-references in tool descriptions where applicable.

### Phase 2: Synchronize Manifest

1. Rename all existing tool entries in `extension/manifest.json`.
2. Add the 6 missing tools with concise descriptions.
3. Reorder tools by domain: calendar (11), mail (4), account (3), system (2).
4. Verify `user_config.client_id.default` is `"outlook-desktop"`.

### Phase 3: Update Documentation and Tests

1. Update all tool name references in `docs/prompts/mcp-tool-crud-test.md`.
2. Update `AGENTS.md` tool count.
3. Add a **Tool Naming Convention** section to `AGENTS.md` documenting:
   - The naming pattern: `{domain}_{operation}[_{resource}]`
   - Recognized domain prefixes: `calendar_`, `mail_`, `account_`
   - System tools (`status`, `complete_auth`) are exempt from prefixing
   - New tools **MUST** follow this convention
4. Update tool name assertions in `*_test.go` files.
5. Run `make ci` to verify all checks pass.

## Test Strategy

### Tests to Modify

| Test File | Change |
|---|---|
| `internal/tools/tool_description_test.go` | Update expected tool names to new prefixed names |
| All `internal/tools/*_test.go` | Update any assertions on tool name strings and `req.Params.Name` values |
| `internal/server/server.go` | Update `toolCount` if the comment references it |

### Tests to Add

None. The rename is mechanical; existing tests cover the tool registration and behavior.

### Tests to Remove

None.

### AC Verification Method

| AC | Verification | Method |
|---|---|---|
| AC-1 | Tool name assertions in `*_test.go` files | Automated (`make test`) |
| AC-2 | Count manifest `tools` array entries vs `server.go` registrations | Manual inspection |
| AC-3 | Grep tool descriptions for old unprefixed cross-references | Manual inspection + `make test` (description tests) |
| AC-4 | Compare `mcp.NewTool()` names with `server.go` middleware name strings | Manual inspection |
| AC-5 | Inspect `user_config.client_id.default` in `manifest.json` | Manual inspection |
| AC-6 | Grep `mcp-tool-crud-test.md` for old tool names | Manual inspection |
| AC-7 | Inspect AGENTS.md for tool count and naming convention section | Manual inspection |
| AC-8 | Inspect `manifest.json` tool ordering | Manual inspection |

## Acceptance Criteria

### AC-1: All tools use domain-prefixed names

```gherkin
Given the MCP server is running
When the LLM discovers available tools
Then all calendar tools have names starting with "calendar_"
  And all mail tools have names starting with "mail_"
  And all account tools have names starting with "account_"
  And "status" and "complete_auth" retain their unprefixed names
```

### AC-2: Manifest contains all 20 tools

```gherkin
Given the extension/manifest.json file
When the tools array is inspected
Then it contains exactly 20 tool entries
  And each tool registered in server.go has a corresponding manifest entry
  And no manifest entry exists without a server.go registration
```

### AC-3: Tool description cross-references use new names

```gherkin
Given the tool descriptions for calendar_delete_event, calendar_cancel_event, mail_list_messages, and mail_search_messages
When the descriptions are inspected
Then calendar_delete_event references "calendar_cancel_event" (not "cancel_event")
  And calendar_cancel_event references "calendar_delete_event" (not "delete_event")
  And mail_list_messages references "mail_list_folders" (not "list_mail_folders")
  And mail_list_messages references "mail_search_messages" (not "search_messages")
  And mail_search_messages references "mail_list_messages" (not "list_messages")
```

### AC-4: Middleware name strings match tool names

```gherkin
Given each tool registration in server.go
When the wrap/wrapWrite, WithObservability, and AuditWrap name arguments are inspected
Then each name argument matches the tool name in the corresponding mcp.NewTool() call
```

### AC-5: Manifest default client_id is "outlook-desktop"

```gherkin
Given the extension/manifest.json file
When user_config.client_id.default is inspected
Then it equals "outlook-desktop"
```

### AC-6: CRUD test document uses new names

```gherkin
Given docs/prompts/mcp-tool-crud-test.md
When all mcp__outlookCalendar__ tool references are inspected
Then every tool reference uses the new domain-prefixed name
  And no references to old unprefixed tool names remain
```

### AC-7: AGENTS.md documents tool count and naming convention

```gherkin
Given AGENTS.md
When the project structure and Tool Naming Convention sections are inspected
Then the tools/ line states "20 MCP tool handlers (11 calendar + 4 mail + 3 account + 2 system)"
  And the Tool Naming Convention section documents the pattern "{domain}_{operation}[_{resource}]"
  And it lists the recognized domain prefixes: "calendar_", "mail_", "account_"
  And it states that system tools ("status", "complete_auth") are exempt
  And it states that new tools MUST follow this convention
```

### AC-8: Manifest tools ordered by domain

```gherkin
Given the extension/manifest.json tools array
When the ordering is inspected
Then calendar tools appear first (11 entries)
  And mail tools appear second (4 entries)
  And account tools appear third (3 entries)
  And system tools appear last (2 entries)
```

## Quality Standards Compliance

### Build & Compilation

- [x] Code compiles/builds without errors
- [x] No new compiler warnings introduced

### Linting & Code Style

- [x] All linter checks pass with zero warnings/errors
- [x] Code follows project coding conventions and style guides
- [x] Any linter exceptions are documented with justification

### Test Execution

- [x] All existing tests pass after implementation
- [x] All new tests pass
- [x] Test coverage meets project requirements for changed code

### Documentation

- [x] Inline code documentation updated where applicable
- [x] Tool descriptions updated for new/modified tools
- [x] User-facing documentation updated if behavior changes

### Code Review

- [ ] Changes submitted via pull request
- [ ] PR title follows Conventional Commits format
- [ ] Code review completed and approved
- [ ] Changes squash-merged to maintain linear history

### Verification Commands

```bash
# Build verification
make build

# Lint verification
make lint

# Test execution
make test

# Full CI check
make ci
```

## Risks and Mitigation

### Risk 1: Missed tool name reference

**Likelihood:** medium
**Impact:** medium -- a stale name in middleware causes observability/audit to log the wrong tool name.
**Mitigation:** Grep the entire codebase for each old tool name after implementation. The `tool_description_test.go` test validates all registered tool names. Phase 3 explicitly includes a `make ci` gate.

### Risk 2: CRUD test protocol becomes stale

**Likelihood:** low
**Impact:** low -- the test protocol is a prompt document, not automated CI.
**Mitigation:** Phase 3 explicitly updates `mcp-tool-crud-test.md`. The new AGENTS.md rule (added earlier this session) requires future tool changes to update this document.

## Estimated Effort

| Phase | Description | Estimate |
|---|---|---|
| Phase 1 | Rename tool names in 18 tool files + server.go | 1-2 hours |
| Phase 2 | Synchronize manifest (rename + add 6 + reorder) | 0.5 hours |
| Phase 3 | Update docs, tests, AGENTS.md, verify with `make ci` | 1-2 hours |
| **Total** | | **2.5-4.5 hours** |

## Decision Outcome

Chosen approach: "Domain-prefixed tool names with manifest sync", because it establishes a consistent, discoverable naming convention before public release when the migration cost is zero.

Alternatives considered:
- **Prefix only calendar tools**: Would leave mail and account tools inconsistent. Rejected because the same discoverability argument applies to all domains.
- **Use slash-separated namespaces (e.g., `calendar/list_events`)**: MCP tool names are typically flat identifiers; slashes may cause issues with some clients. Rejected for compatibility.
- **Keep current names and add missing tools only**: Defers the naming debt to post-release where it becomes a breaking change with real migration cost. Rejected.
- **Add backward-compatible aliases**: Doubles the tool count visible to the LLM, degrading discoverability. Rejected since this is pre-release.

## Related Items

- CR-0006 -- Read-Only Tools (introduced `list_calendars`, `list_events`, `get_event`)
- CR-0007 -- Search & Free/Busy (introduced `search_events`, `get_free_busy`)
- CR-0008 -- Create & Update Tools (introduced `create_event`, `update_event`)
- CR-0009 -- Delete & Cancel Tools (introduced `delete_event`, `cancel_event`)
- CR-0025 -- Multi-Account Elicitation (introduced `add_account`, `list_accounts`, `remove_account`)
- CR-0029 -- MCPB Extension Packaging (created `extension/manifest.json`)
- CR-0030 -- Manual Auth Code Flow (introduced `complete_auth`)
- CR-0042 -- UX Polish: Tool Ergonomics (introduced `respond_event`, `reschedule_event`)
- CR-0043 -- Mail Read & Event Correlation (introduced mail tools)
- `extension/manifest.json` -- Extension manifest to update
- `internal/server/server.go` -- Tool registration hub
- `docs/prompts/mcp-tool-crud-test.md` -- CRUD test protocol to update

<!--
## CR Review Summary (2026-03-22)

**Findings: 6 | Fixes applied: 6 | Unresolvable: 0**

1. **Contradiction (FR-1/2/3, FR-8, Phase 1)**: Account tools do not use `wrap()`/`wrapWrite()`
   in server.go — they use direct `WithObservability()`/`AuditWrap()` calls. FRs and Phase 1
   were updated to accurately describe both middleware patterns.

2. **Missing cross-reference**: `list_messages` description references `list_mail_folders`
   (will become `mail_list_folders`), but this cross-reference was absent from the
   Tool Description Cross-References table. Added to table and AC-3.

3. **Incomplete AC-3**: Only covered calendar tool cross-references (`delete_event`/`cancel_event`).
   Added mail tool cross-references (`list_messages`/`search_messages`/`list_mail_folders`).

4. **Missing FR-10 coverage in AC-7**: AC-7 covered the naming convention section but not the
   AGENTS.md tool count update (FR-10). Extended AC-7 to verify the tool count string.

5. **Missing slog strings**: All 18 domain tool handlers contain `slog.With("tool", "old_name")`
   literals that must be renamed. Added to Phase 1 steps, Affected Components, and In Scope.

6. **Missing AC-Test mapping**: No explicit mapping between ACs and verification methods.
   Added "AC Verification Method" table to Test Strategy section.
-->
