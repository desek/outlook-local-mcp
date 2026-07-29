# CR-0068 Validation Report

Validated by cr-validator on 2026-07-29 against branch `dev/cr-0066`.
CR: `docs/cr/CR-0068-tool-definition-quality-and-annotation-accuracy.md`

## Summary

Requirements: 13/13 | Non-Functional: 3/3 | Acceptance Criteria: 12/12 | Tests: 19/19 | Gaps: 0

FAIL: 0 | PARTIAL: 0 | GAP: 0

Scope: only the six CR-0068 commits were traced (`adbbd9b`, `364ecac`, `dbbdbcc`,
`8c55a96`, `5516c5c`, `0a35696`); CR-0066/CR-0067 commits on the same branch were
excluded. Evidence comes from three independent sources: (1) the branch diff
`git diff 364ecac^..0a35696`, (2) the project test runner
(`go test ./internal/tools/...`, `make build vet fmt-check tidy test lint`), and
(3) an end-to-end `tools/list` payload captured from the self-contained container
image `outlook-local-mcp:ci-standalone` (built via `docker build .`) under two
configurations: the default gated config and a full-surface config
(`OUTLOOK_MCP_MAIL_ENABLED=true`, `OUTLOOK_MCP_MAIL_MANAGE_ENABLED=true`,
`OUTLOOK_MCP_AUTH_METHOD=auth_code`).

## Pipeline result

| Check | Command | Result |
|-------|---------|--------|
| Build | `make build` | PASS |
| Vet | `make vet` | PASS |
| Format | `make fmt-check` | PASS |
| Tidy | `make tidy` | PASS |
| Test (`-race`/coverage) | `make test` | PASS (all packages ok; `internal/tools` 72.2%) |
| Lint | `make lint` | PASS (`0 issues.`) |
| Container smoke | `scripts/smoke-test-image.sh outlook-local-mcp:ci-standalone` | PASS (four aggregate tools advertised) |

## End-to-end annotation payload (captured from the built container)

| Tool | Config | readOnlyHint | destructiveHint | idempotentHint | openWorldHint | descLen |
|------|--------|:---:|:---:|:---:|:---:|---:|
| `mail` | gated (default) | true | false | true | true | 1268 |
| `mail` | MAIL_MANAGE_ENABLED | false | true | false | true | 2209 |
| `system` | browser (no `complete_auth`) | true | false | true | **false** | 1107 |
| `system` | auth_code (`complete_auth`) | false | false | false | **true** | 1217 |
| `calendar` | any | false | true | false | true | 2123 |
| `account` | any | false | true | false | true | 1018 |

This is the observable-behaviour ground truth for FR-1, FR-3..FR-8, AC-1..AC-4,
AC-6, AC-10, AC-12 and NFR-3.

## Requirement Verification

| Req # | Description | Status | Evidence (file:line / test / payload) |
|-------|-------------|--------|----------------------------------------|
| FR-1 | Compute the five annotations from the registered verb set | PASS | `aggregate_annotations.go:92` `AggregateAnnotations`; wired at `server.go:90,116,145,182`. Runtime proof: `mail`/`system` hints differ between gated and full configs (payload table). |
| FR-2 | Single shared helper used by all four domains | PASS | The four `*ToolAnnotations()` funcs deleted (diff of `account_verbs.go`, `calendar_verbs.go`, `mail_verbs.go`, `system_verbs.go`); all four call sites now call `tools.AggregateAnnotations(...)` (`server.go`). |
| FR-3 | readOnlyHint true iff every verb read-only | PASS | Fold `aggregate_annotations.go:105,111`; `TestAggregateAnnotationsAllReadOnly`, `TestAggregateAnnotationsOneDestructive`; payload: gated `mail` true, full `mail` false. |
| FR-4 | destructiveHint true if any destructive | PASS | Fold `aggregate_annotations.go:114`; `TestAggregateAnnotationsOneDestructive`; payload: `calendar` true. |
| FR-5 | idempotentHint false if any non-idempotent | PASS | Fold `aggregate_annotations.go:117`; `TestAggregateAnnotationsOneNonIdempotent`; payload: `calendar` false. |
| FR-6 | openWorldHint true if any Graph verb | PASS | Fold `aggregate_annotations.go:120`; payload: `system` auth_code true, `mail` true. |
| FR-7 | Gated `mail`: readOnly true, destructive false, openWorld true | PASS | Payload gated `mail` = (true, false, _, true). Depended on the help-verb classification fix at `help/verb.go:52-57` (see FR-13). |
| FR-8 | `system` without `complete_auth`: openWorld false | PASS | Payload gated `system` openWorldHint=**false**; `TestSystemAnnotationsClosedWorld`. |
| FR-9 | State each verb's required parameters, derived from schema | PASS | `dispatch_describe.go:108` `verbRequiredParams` reads `mcp.NewTool(..., v.Schema...).InputSchema.Required` — derived, cannot drift. Payload: every verb line ends `Requires: ...` or `No required parameters.` (e.g. `get_message` -> `Requires: message_id`). |
| FR-10 | Each verb inventory entry on its own line | PASS | `dispatch_describe.go:73-74` emits `"\n- \`"` per verb. Payload: calendar has 15 dedicated `- \`verb\`` lines. |
| FR-11 | `mail` description names gated write verbs + config keys | PASS | `server.go:174-181` intro; payload gated `mail` names `MailEnabled`, `MailManageEnabled`, and the five write verbs. |
| FR-12 | Every parameter description non-empty | PASS | `TestEveryParameterHasDescription`; payload scan: 62 params (gated) / 73 params (full), 0 empty. |
| FR-13 | Derive classification from registry `Annotations`; a verb cannot be registered without one | PASS | `aggregate_annotations.go:32-72` materialises hints; `TestEveryVerbHasClassification`. Mutation-verified non-vacuous (see below). |

## Non-Functional Requirement Verification

| Req # | Description | Status | Evidence |
|-------|-------------|--------|----------|
| NFR-1 | No verb name, parameter name, parameter type, or handler change | PASS | Diff touches only annotation wiring + description composition; no `Verb.Name`, `Schema`, or handler edits. `help/verb.go` change adds `Annotations` only. Payload confirms unchanged operation enums and parameter names/types across all four tools; four tools still present. |
| NFR-2 | Aggregate computation free of I/O | PASS | `AggregateAnnotations` and `materializeAnnotations` are pure functions over in-memory `mcp.ToolOption` closures; no I/O, no mutation of inputs (`aggregate_annotations.go`). |
| NFR-3 | Each description < 4000 characters | PASS | `TestDescriptionLengthBounded` (`< 4000`, PASS). Measured lengths: calendar 2123, mail 2209 (full), system 1217, account 1018 — max 2209. |

## Acceptance Criteria Verification

| AC # | Description | Status | Evidence |
|------|-------------|--------|----------|
| AC-1 | Gated mail: readOnly true, destructive false, openWorld true | PASS | Payload gated `mail`; `TestMailAnnotationsGatedReadOnly`. |
| AC-2 | MailManageEnabled: readOnly false, destructive true | PASS | Payload full `mail`; `TestMailAnnotationsManageEnabled`. |
| AC-3 | System closed-world without `complete_auth` | PASS | Payload gated `system` openWorldHint=false; `TestSystemAnnotationsClosedWorld`. |
| AC-4 | System open-world with `complete_auth` | PASS | Payload auth_code `system` openWorldHint=true; `TestSystemAnnotationsOpenWorldWithAuthCode`. |
| AC-5 | Single destructive verb dominates | PASS | `TestAggregateAnnotationsOneDestructive`; payload `calendar` destructive=true. |
| AC-6 | Line-structured inventories, no description > 4000 | PASS | Payload line structure + max 2209 chars; `TestDescriptionsListVerbsOnSeparateLines`, `TestDescriptionLengthBounded`. |
| AC-7 | Required parameters discoverable without a help round-trip | PASS | Payload: calendar description states `Requires:` for each of 15 verbs; `TestEveryVerbStatesRequiredParameters`. |
| AC-8 | Gated mail verbs disclosed | PASS | Payload gated `mail` names both gating keys; `TestMailDescriptionMentionsGatedVerbs`. |
| AC-9 | New verbs cannot omit a classification | PASS | `TestEveryVerbHasClassification`; **mutation-verified** (below). |
| AC-10 | No behavioural regression; four tools remain | PASS | Smoke test advertises `account`/`calendar`/`mail`/`system`; NFR-1; `TestAggregateAnnotations_FourToolsRegistered`, `TestAggregateAnnotations_NoOldToolNames`. |
| AC-11 | Single non-idempotent verb dominates | PASS | `TestAggregateAnnotationsOneNonIdempotent`; payload `calendar`/`account` idempotent=false. |
| AC-12 | Every parameter carries a non-empty description | PASS | Payload scan 0 empty (62/73 params); `TestEveryParameterHasDescription`. |

### AC-9 / FR-13 mutation verification (non-vacuous proof)

`TestEveryVerbHasClassification` was proven to fail when a real registered verb
omits a hint, using an isolated git worktree at CR-0068 tip `5516c5c`:

- Mutation: removed `mcp.WithOpenWorldHintAnnotation(true)` from `create_event`
  in `internal/server/calendar_verbs.go`.
- Before: `--- PASS: TestEveryVerbHasClassification`.
- After: `--- FAIL: TestEveryVerbHasClassification` with
  `verb_metadata_test.go:166: domain "calendar" verb "create_event": missing openWorldHint classification; every verb MUST declare all four hints (CR-0068 FR-13)`.

The guard names the exact verb and the exact missing hint. Worktree reverted;
no source in the working tree was modified.

## Test Strategy Verification

All tests specified in the CR exist and pass (`go test ./internal/tools/...`, all
PASS). Behavioural tests are backed by both the runner and the container payload.

| Test File | Test Name | Specified | Exists | Matches Spec |
|-----------|-----------|:---:|:---:|:---:|
| aggregate_annotations_test.go | TestAggregateAnnotationsAllReadOnly | yes | yes | yes |
| aggregate_annotations_test.go | TestAggregateAnnotationsOneDestructive | yes | yes | yes |
| aggregate_annotations_test.go | TestAggregateAnnotationsAllLocal | yes | yes | yes |
| aggregate_annotations_test.go | TestAggregateAnnotationsOneNonIdempotent | yes | yes | yes |
| aggregate_annotations_test.go | TestAggregateAnnotationsEmptyVerbSet | yes | yes | yes |
| tool_annotations_test.go | TestMailAnnotationsGatedReadOnly | yes | yes | yes |
| tool_annotations_test.go | TestMailAnnotationsManageEnabled | yes | yes | yes |
| tool_annotations_test.go | TestSystemAnnotationsClosedWorld | yes | yes | yes |
| tool_annotations_test.go | TestSystemAnnotationsOpenWorldWithAuthCode | yes | yes | yes |
| description_quality_test.go | TestDescriptionsListVerbsOnSeparateLines | yes | yes | yes |
| description_quality_test.go | TestMailDescriptionMentionsGatedVerbs | yes | yes | yes |
| description_quality_test.go | TestEveryVerbStatesRequiredParameters | yes | yes | yes |
| description_quality_test.go | TestDescriptionLengthBounded | yes | yes | yes |
| description_quality_test.go | TestEveryParameterHasDescription | yes | yes | yes |
| verb_metadata_test.go | TestEveryVerbHasClassification | yes | yes | yes (mutation-verified) |
| tool_annotations_test.go | TestAggregateAnnotations_Mail (modified) | yes | yes | yes |
| tool_annotations_test.go | TestAggregateAnnotations_System (modified) | yes | yes | yes (browser -> closed-world/read-only) |
| tool_annotations_test.go | TestAggregateAnnotations_Calendar (modified) | yes | yes | yes |
| tool_annotations_test.go | TestAggregateAnnotations_Account (modified) | yes | yes | yes |

## Diff Coverage

| File | +/- | Mapped Requirements |
|------|-----|---------------------|
| internal/tools/aggregate_annotations.go (new) | +132 | FR-1, FR-2, FR-3..FR-6, FR-13, NFR-2 |
| internal/tools/aggregate_annotations_test.go (new) | +123 | FR-3..FR-6 tests, AC-5, AC-11 |
| internal/tools/description_quality_test.go (new) | +185 | FR-9, FR-10, FR-11, FR-12, NFR-3, AC-6..AC-8, AC-12 |
| internal/tools/verb_metadata_test.go | +76 | FR-13, AC-9 |
| internal/tools/tool_annotations_test.go | +92/-… | FR-3..FR-8, AC-1..AC-4, AC-10 |
| internal/tools/dispatch_describe.go | +75/-… | FR-9, FR-10 (description composer; `verbRequiredParams`) |
| internal/server/server.go | +20/-… | FR-2 (helper wiring), FR-11 (mail intro) |
| internal/server/mail_verbs.go | -23 | FR-2 (removed hardcoded `mailToolAnnotations`) |
| internal/server/calendar_verbs.go | -20 | FR-2 |
| internal/server/account_verbs.go | -21 | FR-2 |
| internal/server/system_verbs.go | -23 | FR-2 |
| internal/server/introspect_verbs.go (new) | +114 | FR-13/AC-9 test support (`BuildDomainVerbSets`) |
| internal/tools/help/verb.go | +13 | FR-7, FR-13 (help-verb classification; the OBS-1 fix) |
| docs/concepts.md | +22 | Documentation of gating-dependent annotation semantics |
| docs/cr/CR-0068-...accuracy.md | +27/-… | CR review edits + finalization frontmatter |

### Unmapped changed files

Three files were changed that are not in the CR's "Affected Components" list.
Each is justified and consistent with the CR intent; none is stray:

- `internal/server/introspect_verbs.go` (new) — provides `BuildDomainVerbSets`,
  the per-verb introspection helper that `TestEveryVerbHasClassification` and
  `description_quality_test.go` require (RegisterTools discards per-verb detail).
  Necessary to implement AC-9/FR-13. Maintenance note: it deliberately mirrors
  the four build-config literals in `RegisterTools`; the two must be kept in sync
  when a domain gains a dependency (documented in the file header).
- `internal/tools/dispatch_describe.go` — the actual description composer; Phase 3
  restructured the verb inventory here (FR-9, FR-10) rather than in the per-domain
  verb files the CR named. Same net effect, better-localised.
- `internal/tools/help/verb.go` — adds the four-hint classification to the shared
  `help` verb. This is the load-bearing fix for FR-7/AC-1: before it, `help`
  declared no classification, folded as not-read-only, and dragged every
  aggregate's `readOnlyHint` to false. The task flagged this; it is verified in
  the payload (gated `mail`/`system` now read-only true).

## Gaps

None.

### Observations (non-blocking, no requirement or AC affected)

- `extension/manifest.json` is listed in the CR's Affected Components and the
  self-check "kept in sync" box is ticked, but the file was not modified in the
  CR-0068 range. This is defensible and not a gap: the AGENTS.md MCPB rule only
  mandates a manifest update when a tool is added or removed, and CR-0068 adds or
  removes no tool (all four aggregate tools remain, with unchanged names, enums,
  and parameter names/types — confirmed in the payload). The manifest's static
  tool descriptions do not mirror the runtime `tools/list` descriptions and are
  not required to. No Functional Requirement, NFR, or AC depends on the manifest,
  so this does not lower any item's status.
