# CR-0079 Validation Report

Validated at `b270559` on branch `docs/cr-implementation-set-0079-0083`.

The CR-scoped diff is `ee23a77..HEAD` (CR-0078 landed at `ee23a77`), covering the review
commit `0bcf168` and the six implementation commits `7b28959`, `a170a01`, `1fc6f57`,
`4b255f8`, `ae75576`, `5f5c4ca`, plus the finalization commit `b270559`. The branch-level
diff against `merge-base origin/main HEAD` (`2cce019`) additionally carries CR-0078 and the
CR-0080 through CR-0083 documents, which are out of this report's scope.

## Summary

Requirements: 31/33 | Acceptance Criteria: 15/19 | Tests: 27/29 | Gaps: 7

**FAIL: 0 | PARTIAL: 6 | GAP: 7**

Every requirement and criterion maps to at least one changed file with a specific hunk.
No stray changed files. The four PARTIAL criteria and two PARTIAL requirements are all the
same shape: the behaviour is implemented and structurally sound, but the specific assertion
the CR names is absent, so the evidence is source-reading rather than a passing test.

### Check pipeline

| Command | Result |
|---|---|
| `make ci` | **exit 0**. Includes build, vet, tidy, `golangci-lint` (0 issues), full `go test`, docs-bundle regeneration, surface-manifest regeneration, `goreleaser check`, `mcpb validate`. |
| `git status --porcelain` after `make ci` | **empty**. The surface drift check regenerated `site/src/generated/surface.json` and the working tree did not move, which is the AC-11 clause. |
| CR Test Strategy race command (`go test -race ./internal/tools/ ./internal/server/ ./internal/validate/ ./internal/docs/ ./internal/config/ ./internal/surface/ -run '…'`) | **all pass**, 45 top-level tests, 6 packages ok. No race reports. |
| `make crud-test` | **not run** (paid harness, excluded by instruction). Recorded as GAP-2. |

## Requirement Verification

### Functional Requirements

| Req # | Description | Status | Evidence (file:line / test name) |
|---|---|---|---|
| FR-1 | Exactly one new verb `add_attachment`; no new top-level tool; tool count stays 4 | PASS | `internal/server/mail_verbs.go:143`; `extension/manifest.json` `tools` array unchanged at 4 entries; `TestRegisterTools_MailManage_RegistersAddAttachment` (asserts `len(s.ListTools()) == 4`); `mcpb validate` passes in `make ci` |
| FR-2 | Registered only under `MailManageEnabled`; absent from default and `MailEnabled`-only enums | PASS | `internal/server/mail_verbs.go:140-144` (inside `if c.cfg.MailManageEnabled`); `TestRegisterTools_MailEnabled` negative assertion at `internal/server/server_test.go:799-805`; `TestRegisterTools_MailManage_RegistersAddAttachment`; `TestRegisterTools_MailDisabled` |
| FR-3 | Requires `message_id`, `name`, `content_bytes`; accepts optional `mime_type` | PASS | `internal/server/mail_verbs.go:775-796` (three `mcp.Required()`, `mime_type` optional); `TestAddAttachment_RejectsBadInputBeforeAnyRequest` |
| FR-4 | Validate `message_id`, bound `name`, reject empty/malformed base64 — all before any Graph request | PASS | `internal/tools/add_attachment.go:85-111`; `internal/validate/validate.go:250-258`; `TestAddAttachment_RejectsBadInputBeforeAnyRequest` (8 subcases, each asserting `rec.gets == 0 && rec.posts == 0`); `TestValidateBase64_Invalid` |
| FR-5 | Reuse the draft guard; widen `verifyIsDraft` to `$select` subject and return the message; update both call sites without behaviour change | PASS | `internal/tools/update_draft.go:224` (new two-value signature), `:230` (`Select: []string{"id","isDraft","subject"}`), `:250`; call sites `internal/tools/update_draft.go:105` and `internal/tools/delete_draft.go:76`; `TestAddAttachment_NonDraftRefused`, `TestUpdateDraft_NotDraft`, `TestDeleteDraft_NotDraft`, `TestUpdateDraft_Success`, `TestDeleteDraft_Success` |
| FR-6 | Route on decoded size; strictly below `3*1000*1000` direct POST, at or above upload session; threshold not a parameter | PASS | `internal/tools/add_attachment.go:38` (constant), `:113`, `:132` (`size >= inlineAttachmentThresholdBytes`); `TestAddAttachment_RoutingBoundary` (both boundary cases) |
| FR-7 | `fileAttachment` carries bytes, name, MIME defaulting to `application/octet-stream`, with the SDK-stamped `@odata.type` | PASS | `internal/tools/add_attachment.go:121-124`, `:166-169`; `TestAddAttachment_DefaultsMimeType` (asserts `contentType`, `@odata.type`, `name` on the wire) |
| FR-8 | Upload session with a `file` `attachmentItem`; chunked PUT in 320 KiB multiples with `Content-Range`; chunk size a named constant | PASS | `internal/tools/attachment_upload_session.go:47` (`5 * 320 * 1024`), `:119-127`, `:164-176`, `:210`; `TestUploadSession_LargeFileChunks` (asserts three exact `Content-Range` headers and total bytes) |
| FR-9 | MIME parameter named `mime_type`, not `content_type` | PASS | `internal/server/mail_verbs.go:792`; `TestMimeTypeIsNotBodyContentType` |
| FR-10 | Confirmation names subject, message id, attachment name, size, attachment id, transfer path; id from the response; `(No subject)` fallback | PASS | `internal/tools/attachment_confirmation.go:56-68`; small-path id `internal/tools/add_attachment.go:195`; large-path id `internal/tools/attachment_upload_session.go:236-249` (Location, then body); `TestAddAttachment_DirectPath` (all six lines), `TestAddAttachment_IdentifierComesFromResponse`, `TestUploadSession_AttachmentIDFromLocationHeader`, `TestAttachmentConfirmationNoSubjectPlaceholder`, `TestAttachmentConfirmationLineOrder` |
| FR-11 | Unconditional text confirmation; no `output` parameter | PASS | `internal/server/mail_verbs.go:775-796` (no `output`); `internal/tools/add_attachment.go:163`, `:206`; `TestWriteVerbsDeclareNoOutputParameter` (registry-derived) |
| FR-12 | All four hints declared explicitly, matching the matrix | PASS | `internal/server/mail_verbs.go:773-778`; `TestAddAttachmentAnnotations` |
| FR-13 | Wrapped by the write middleware chain under `mail.add_attachment` / audit `write` | PASS | `internal/server/mail_verbs.go:772` (`wrapWrite("mail.add_attachment", "write", …)`); the single `name` argument feeds observability, `ReadOnlyGuard`, and `AuditWrap` at `internal/server/mail_verbs.go:107`; `TestReadOnlyBlocksAddAttachment` proves the string reaches the chain |
| FR-14 | Upper bound enforced before upload; actionable error naming size, bound, env var; reuse `config.MaxAttachmentSizeBytes`; no second variable | PASS | `internal/tools/add_attachment.go:114-119`; bound threaded at `internal/server/mail_verbs.go:772` (`c.cfg.MaxAttachmentSizeBytes`); `TestAddAttachment_OversizeRefusedBeforeUpload` (asserts `16 bytes`, `8 bytes`, `OUTLOOK_MCP_MAX_ATTACHMENT_SIZE_BYTES`, and zero Graph requests); `TestEveryEnvLiteralIsEnumerated`, `TestInventoryEntriesAreComplete` |
| FR-15 | Every error carries a fix instruction reaching both the tool result and the log record; chunk-PUT errors redacted of the upload URL | **PARTIAL** | Redaction: `internal/tools/attachment_upload_session.go:171`, `:272-282`; `TestUploadSession_ErrorRedactsUploadURL`. Graph-path fix instruction on both channels: `internal/tools/add_attachment.go:143-150`, `:187-192` (`"fix", attachmentFixInstruction`). **But** the validation errors at `internal/tools/add_attachment.go:87-111` emit no log record at all, and the oversize log record at `:115` omits the environment variable its tool result names. This matches the established pattern in `internal/tools/move_message.go:65-77`, but not the literal "MUST reach both" wording. See GAP-6. |
| FR-16 | Non-empty `Summary` ≤ 80 chars, `Description`, ≥1 `Examples`, ≥1 resolving `SeeDocs` | PASS | `internal/server/mail_verbs.go:765` (Summary, 74 chars), `:766` (Description), `:767-770` (two examples), `:771` (`concepts#mail-gating`); `TestEveryVerbHasSummary`, `TestEveryVerbHasDescription`, `TestEveryVerbHasClassification`, `TestSeeDocsAnchorsResolve`, `TestEveryVerbStatesRequiredParameters` |
| FR-17 | Mail domain `Intro` names `add_attachment` | PASS | `internal/server/server.go:179`; `TestMailIntroNamesEveryGatedWriteVerb` (cases derived by diffing the gated against the ungated registry) |
| FR-18 | `extension/manifest.json` mail entry enumerates `add_attachment`; `tools` array stays at four | PASS | `extension/manifest.json` mail description (single-line hunk, +1/-1); `TestManifestDescribesEveryRegisteredVerb`; `mcpb validate extension/manifest.json` passes in `make ci` |
| FR-19 | `site/src/generated/surface.json` regenerated by `make surface-manifest` and committed | PASS | `site/src/generated/surface.json:204-210`, `:212`, `:315`, `:439-448`; `make ci` regenerated the manifest and left `git status --porcelain` empty; `TestCommittedManifestMatchesRecord` |
| FR-20 | One appended Step 41 without renumbering; skip instruction reads "Steps 31 through 35 and Steps 37 through 41"; attaches to a harness-created draft | PASS | `docs/prompts/mcp-tool-crud-test.md:583` (exact instruction text), `:681-691` (Step 41 creates and deletes its own draft, and asserts the non-draft refusal on a Step 30 received message), `:768` (summary row). Diff is pure addition at the tail; no existing step number changed |
| FR-21 | Verb inventory golden regenerated with `mail.add_attachment ro=false de=false id=false ow=true` | PASS | `internal/tools/dispatch_registry_test.go:62` (exactly one added line); `TestVerbInventoryUnchangedAfterUpgrade` |
| FR-22 | No change to the OAuth scope set; `Mail.Send` stays unrequested | PASS (negative) | No hunk touches `internal/auth/` anywhere in `ee23a77..HEAD`; `TestScopes_MailManage` and `TestScopes_NoMailSend` pass in `make ci`. A MUST-NOT requirement's evidence is the absence of a hunk in the named package plus the derived tests that would fail on a change; stated explicitly rather than claimed as diff evidence |
| FR-23 | Default surface unchanged: mail `defaultCount` 5, `fullCount` 17→18, totals 46→47 full and 33 default | PASS | `site/src/generated/surface.json:212` (`"fullCount": 18`), `:213` (`"defaultCount": 5`), `:315` (`"fullCount": 47`), `:316` (`"defaultCount": 33`); `TestCommittedManifestMatchesRecord`; `TestRegisterTools_MailEnabled` negative assertion |
| FR-24 | Embedded bundle updated: gating row, symptom list, two anchored troubleshooting entries | PASS | `docs/concepts.md:65`; `docs/troubleshooting.md:206` (symptom list), `:208` (cause), `:220` (`{#attachment-target-not-a-draft}`), `:238` (`{#attachment-upload-did-not-complete}`); `TestMailGatingRowNamesDraftAttachments`, `TestMailGatingRowNamesMessageManagement`, `TestCatalog_AllSlugsResolve`, `TestBundleSizeUnder2MiB` |
| FR-25 | Both configuration inventory descriptions widened | PASS | `internal/config/inventory.go:91-92`; propagated to `site/src/generated/surface.json:442`, `:447` |

### Non-Functional Requirements

| Req # | Description | Status | Evidence (file:line / test name) |
|---|---|---|---|
| NFR-1 | Handler and chunked transfer in their own files under `internal/tools/` | PASS | `internal/tools/add_attachment.go` (208 lines), `internal/tools/attachment_upload_session.go` (282 lines), `internal/tools/attachment_confirmation.go` (69 lines); each carries a single `@agents-index` line at `:15`, `:18`, `:13` respectively |
| NFR-2 | Mail description < 4 000 chars; cold-start schema reduction ≥ 60% | PASS | `TestDescriptionLengthBounded`, `TestColdStartSchemaSize_Reduction` — both pass under `-race` |
| NFR-3 | No third-party dependency; standard library HTTP for the chunked PUT | PASS (negative) | `go.mod` and `go.sum` absent from the entire `ee23a77..HEAD` diff; `internal/tools/attachment_upload_session.go:30` imports `net/http` and `:23-33` imports nothing outside stdlib plus the already-pinned SDK |
| NFR-4 | Graph SDK calls route through `RetryGraphCall` inside `WithTimeout` and redact with the shared helpers | PASS | `internal/tools/add_attachment.go:171-179` (small path), `internal/tools/attachment_upload_session.go:129-138` (session creation), `internal/tools/update_draft.go:225-238` (guard); redaction at `internal/tools/add_attachment.go:148`, `:190` |
| NFR-5 | Per-chunk timeout; transfer failure surfaces as an actionable redacted error, never a partial success | **PARTIAL** | No-partial-success is fully covered: `internal/tools/attachment_upload_session.go:170-181`; `TestUploadSession_TransferFailureIsNotPartialSuccess`, `TestUploadSession_AttachmentIDFromLocationHeader/a_completion_naming_no_attachment_is_an_error`. **But** the per-chunk timeout bound at `internal/tools/attachment_upload_session.go:201` (`graph.WithTimeout(ctx, timeout)` inside `putAttachmentChunk`) has no test that exercises a stalled chunk, so that clause is source-reading only. See GAP-5 |
| NFR-6 | Upper bound is the existing `MaxAttachmentSizeBytes`; default not raised | PASS | `internal/tools/add_attachment.go:114-119` takes `maxSize` as a parameter; `internal/server/mail_verbs.go:772` supplies `c.cfg.MaxAttachmentSizeBytes`; `internal/config/inventory.go:92` default `10485760` unchanged; `TestAddAttachment_OversizeRefusedBeforeUpload` |
| NFR-7 | Small path: exactly one GET and one POST. Large path: one GET, one createUploadSession, minimum chunk PUTs, no re-read | PASS | `TestAddAttachment_DirectPath` asserts `rec.gets == 1 && rec.posts == 1`; `TestUploadSession_LargeFileChunks` asserts `up.sessions == 1`, `rec.posts == 0`, exactly 3 chunks for `2*chunk+12345` bytes, and `up.received == size` |
| NFR-8 | Name bound, inline threshold, chunk size each a named constant; upper bound from configuration | PASS | `internal/validate/validate.go:40` (`MaxAttachmentNameLen`), `internal/tools/add_attachment.go:38` (`inlineAttachmentThresholdBytes`), `internal/tools/attachment_upload_session.go:47` (`uploadChunkSizeBytes`); bound from config, not a literal, at `internal/server/mail_verbs.go:772`; `TestMaxAttachmentNameLenBoundsAName` |

## Acceptance Criteria Verification

| AC # | Description | Status | Evidence |
|---|---|---|---|
| AC-1 | Registers, and only behind the manage gate | PASS | `TestRegisterTools_MailManage_RegistersAddAttachment` (enum contains it, four tools); `TestRegisterTools_MailEnabled` (absent under the read gate, `internal/server/server_test.go:799-805`); `TestRegisterTools_MailDisabled` |
| AC-2 | Small file attached with a single direct POST | PASS | `TestAddAttachment_DirectPath` — 1 GET, 1 POST, no session, and all six confirmation fields asserted verbatim |
| AC-3 | Large file attached through a chunked upload session; the boundary takes the session | PASS | `TestUploadSession_LargeFileChunks` (session count, zero direct POSTs, three exact `Content-Range` headers, total bytes, session label); `TestUploadSession_AttachmentIDFromLocationHeader`; `TestAddAttachment_RoutingBoundary` (exactly-`3000000` case asserts zero direct POSTs) |
| AC-4 | Attachment id comes from the service, not the request | PASS | `TestAddAttachment_IdentifierComesFromResponse` (positive and negative assertion); `TestAttachmentConfirmationUsesResponseNotArguments` |
| AC-5 | Non-draft message refused, no attachment request sent | PASS | `TestAddAttachment_NonDraftRefused` (asserts the established refusal string and `rec.posts == 0`) |
| AC-6 | Required and well-formed inputs enforced before any call; no `output` parameter | PASS | `TestAddAttachment_RejectsBadInputBeforeAnyRequest` (8 subcases, each asserting zero GETs and zero POSTs and that the error names the parameter); `TestWriteVerbsDeclareNoOutputParameter` |
| AC-7 | Oversize refused before any upload; no second env variable | PASS | `TestAddAttachment_OversizeRefusedBeforeUpload`; `TestEveryEnvLiteralIsEnumerated`, `TestInventoryEntriesAreComplete` |
| AC-8 | Transfer failure is a failure and never leaks the upload URL, on either channel | **PARTIAL** | Tool-result half fully covered: `TestUploadSession_TransferFailureIsNotPartialSuccess`, `TestUploadSession_ErrorRedactsUploadURL` (asserts the URL, the `upload-secret` query value, and the presence of the redaction marker). Log-record half is safe by construction — the error is redacted at `internal/tools/attachment_upload_session.go:171` *before* it reaches `logger.ErrorContext` at `internal/tools/add_attachment.go:143-147` — but **no test reads the log record**. See GAP-3 |
| AC-9 | Registry metadata complete | PASS | `TestEveryVerbHasSummary`, `TestEveryVerbHasDescription`, `TestEveryVerbHasClassification`, `TestSeeDocsAnchorsResolve` (all registry-derived, all pass under `-race`) |
| AC-10 | Description and confirmation name the base64 contract | PASS | `internal/server/mail_verbs.go:766` ("content_bytes carries the file content as a standard base64-encoded string"); `TestEveryVerbStatesRequiredParameters`; confirmation half by `TestAttachmentConfirmationLineOrder` and `TestAddAttachment_DirectPath` (name, `Size: 5 bytes`, `Transfer: direct upload`) |
| AC-11 | Every generated and hand-written description of the surface names the new verb | PASS | `TestMailIntroNamesEveryGatedWriteVerb`; `TestVerbInventoryUnchangedAfterUpgrade` (golden delta is exactly one line, `internal/tools/dispatch_registry_test.go:62`); `TestCommittedManifestMatchesRecord`; `TestManifestDescribesEveryRegisteredVerb`; `make ci` exit 0 with `git status --porcelain` empty after the surface regeneration; figures 18/5 and 47/33 at `site/src/generated/surface.json:212-213`, `:315-316`; widened config descriptions at `:442`, `:447` |
| AC-12 | Measured surface gates keep their margins; no new dependency | PASS | `TestDescriptionLengthBounded`, `TestColdStartSchemaSize_Reduction`; `go.mod`/`go.sum` absent from the CR diff |
| AC-13 | Consent surface unchanged | PASS | `TestScopes_MailManage`, `TestScopes_NoMailSend` pass in `make ci`; no hunk in `internal/auth/` |
| AC-14 | MIME parameter distinct from the body content type | PASS | `TestMimeTypeIsNotBodyContentType` (asserts `content_type` keeps exactly the `text`/`html` enum and that `mime_type` is a separate free string with no enum) |
| AC-15 | Annotation hints declared and matching the matrix; fold unchanged | PASS | `TestAddAttachmentAnnotations`; `TestMailAnnotationsManageEnabled`, `TestMailAnnotationsGatedReadOnly` (both unchanged and passing) |
| AC-16 | Read-only mode blocks the verb; the same identity reaches the audit record and telemetry | **PARTIAL** | Read-only half fully covered: `TestReadOnlyBlocksAddAttachment` (`internal/server/readonly_test.go:250-260`) asserts the refusal names `mail.add_attachment`. Identity is single-source by construction — `wrapWrite` at `internal/server/mail_verbs.go:107` passes one `name` to `WithObservability`, `ReadOnlyGuard`, and `AuditWrap`. **But** `TestMailManagementVerbsCarryDotIdentity` enumerates its verbs by hand at `internal/server/mail_verbs_test.go:378` (`{"move_message","set_flag","set_categories","mark_read"}`) and does not include `add_attachment`, so no test reads the audit record for this verb. See GAP-4 |
| AC-17 | Implementation follows the file and helper conventions; timeout and error text carry the fix instruction | **PARTIAL** | Second clause PASS from source: three separate files (`add_attachment.go`, `attachment_upload_session.go`, `attachment_confirmation.go`), and three named constants (`internal/tools/add_attachment.go:38`, `internal/tools/attachment_upload_session.go:47`, `internal/validate/validate.go:40`). **First clause is source-reading only**: `internal/tools/add_attachment.go:137-141` and `:181-186` build the timeout message from `graph.TimeoutErrorMessage(int(timeout.Seconds()))` and `:148`/`:190` redact via `graph.RedactGraphError`, but no test exercises a timeout or a Graph error on this verb. See GAP-5 |
| AC-18 | Draft guard widened once; existing callers unchanged in behaviour | **PARTIAL** | Second clause fully covered: `TestUpdateDraft_NotDraft`, `TestDeleteDraft_NotDraft` pass unmodified (neither test file was touched, because neither called `verifyIsDraft` directly). One-GET-and-subject outcome covered indirectly by `TestAddAttachment_DirectPath` (`rec.gets == 1` and `Message: "Quarterly report"` in the confirmation). **But** the `$select` naming `subject` is evidenced only at `internal/tools/update_draft.go:230`, and the Test Strategy's `TestVerifyIsDraftReturnsSubject` was never written. See GAP-1 |
| AC-19 | Embedded documentation and the lifecycle harness describe the verb that now exists | PASS | `TestMailGatingRowNamesDraftAttachments`, `TestMailGatingRowNamesMessageManagement` (both against the embedded bundle, `internal/docs/catalog_test.go:81`, `:56`); `docs/troubleshooting.md:206`, `:220`, `:238`; `docs/prompts/mcp-tool-crud-test.md:583`, `:681-691`, `:768`; `TestCatalog_AllSlugsResolve`, `TestSeeDocsAnchorsResolve` |

## Test Strategy Verification

### Tests to Add

| Test File | Test Name | Specified | Exists | Matches Spec |
|---|---|---|---|---|
| `internal/tools/add_attachment_test.go` | `TestAddAttachment_SmallFileUsesDirectPost` | yes | yes, as `TestAddAttachment_DirectPath` | yes — asserts 1 POST, no session, direct-path label |
| `internal/tools/add_attachment_test.go` | `TestAddAttachment_ThresholdBoundaryTakesSessionPath` | yes | yes, as `TestAddAttachment_RoutingBoundary` | yes — both `3000000` and `2999999` cases |
| `internal/tools/add_attachment_test.go` | `TestAddAttachment_ConfirmationNamesAttachmentID` | yes | yes, as `TestAddAttachment_IdentifierComesFromResponse` | yes |
| `internal/tools/add_attachment_test.go` | `TestAddAttachment_RejectsNonDraft` | yes | yes, as `TestAddAttachment_NonDraftRefused` | yes |
| `internal/tools/add_attachment_test.go` | `TestAddAttachment_RequiresNameAndBytes` | yes | folded into `TestAddAttachment_RejectsBadInputBeforeAnyRequest` | yes — subcases "missing name", "empty name", "missing content bytes" |
| `internal/tools/add_attachment_test.go` | `TestAddAttachment_RejectsInvalidBase64` | yes | folded into `TestAddAttachment_RejectsBadInputBeforeAnyRequest` | yes — subcase "malformed content bytes" |
| `internal/tools/add_attachment_test.go` | `TestAddAttachment_InvalidMessageIDRejectedBeforeCall` | yes | folded into `TestAddAttachment_RejectsBadInputBeforeAnyRequest` | yes — subcases "missing message id", "oversize message id" |
| `internal/tools/add_attachment_test.go` | `TestAddAttachment_DefaultsMimeType` | yes | yes | yes |
| `internal/tools/add_attachment_test.go` | `TestAddAttachment_OversizeRejectedBeforeUpload` | yes | yes, as `TestAddAttachment_OversizeRefusedBeforeUpload` | yes |
| `internal/tools/add_attachment_test.go` | `TestAddAttachment_NoOutputParameter` | yes | **no** | superseded by `TestWriteVerbsDeclareNoOutputParameter`, which the CR itself names as the AC-6 gate and which is strictly stronger (registry-derived across every domain). Not counted as a gap |
| `internal/tools/attachment_upload_session_test.go` | `TestUploadSession_LargeFileChunks` | yes | yes | yes — chunk sizes, `Content-Range`, total bytes |
| `internal/tools/attachment_upload_session_test.go` | `TestUploadSession_AttachmentIDFromLocationHeader` | yes | yes | yes — including the percent-decoding case and the no-identifier-is-an-error subtest |
| `internal/tools/attachment_upload_session_test.go` | `TestUploadSession_TransferFailureIsNotPartialSuccess` | yes | yes | yes |
| `internal/tools/attachment_upload_session_test.go` | `TestUploadSession_ErrorRedactsUploadURL` | yes | yes | yes — asserts the URL, the query-string credential, and the redaction marker |
| `internal/tools/attachment_confirmation_test.go` | `TestAttachmentConfirmationUsesResponseNotArguments` | yes | yes | yes |
| `internal/tools/attachment_confirmation_test.go` | `TestAttachmentConfirmationNoSubjectPlaceholder` | yes | yes | yes |
| `internal/tools/update_draft_test.go` | `TestVerifyIsDraftReturnsSubject` | yes | **no** | **GAP-1** — `internal/tools/update_draft_test.go` was not touched by this CR |
| `internal/tools/add_attachment_test.go` | `TestAddAttachment_UsesMaxAttachmentSizeBytes` | yes | folded into `TestAddAttachment_OversizeRefusedBeforeUpload` | yes — handler built with an 8-byte bound; error names size, bound, and the env var |
| `internal/docs/catalog_test.go` | `TestMailGatingRowNamesDraftAttachments` | yes | yes | yes |
| `internal/tools/tool_annotations_test.go` | `TestAddAttachmentAnnotations` | yes | yes | yes |
| `internal/server/mail_verbs_test.go` | `TestMimeTypeIsNotBodyContentType` | yes | yes | yes |
| `internal/server/server_test.go` | `TestRegisterTools_MailManage_RegistersAddAttachment` | yes | yes | yes |
| `internal/server/server_test.go` | `TestMailIntroNamesEveryGatedWriteVerb` | yes | yes | yes — cases derived from the registry diff, as specified |
| `internal/server/readonly_test.go` | `TestReadOnlyBlocksAddAttachment` | yes | yes | yes |

Unspecified extras added (not defects, recorded for completeness):
`TestNewHandleAddAttachment_ReturnsHandler`, `TestAttachmentConfirmationLineOrder`,
`TestValidateBase64_Valid`, `TestValidateBase64_Invalid`, `TestMaxAttachmentNameLenBoundsAName`.

### Tests to Modify

| Test File | Test Name | Specified | Exists | Matches Spec |
|---|---|---|---|---|
| `internal/tools/dispatch_registry_test.go` | `TestVerbInventoryUnchangedAfterUpgrade` | yes | yes | yes — golden gains exactly one line at `:62` |
| `internal/server/server_test.go` | `TestRegisterTools_MailEnabled` | yes | yes | yes — negative assertion added at `:799-805` |
| `internal/tools/tool_annotations_test.go` | `TestMailAnnotationsManageEnabled` | yes | yes | yes — unchanged and re-asserted against the larger verb set; passes |
| `internal/tools/update_draft_test.go`, `internal/tools/delete_draft_test.go` | existing draft-guard cases | yes | yes, **unmodified** | yes — neither file calls `verifyIsDraft` directly, so the widened signature required no edit; all four tests pass |
| `docs/prompts/mcp-tool-crud-test.md` | lifecycle prompt steps | yes | yes | yes — Step 41 added at `:681`, skip instruction at `:583`, summary row at `:768`, no renumbering |

### Existing gates that graded this change without modification

All pass under `-race`: `TestEveryVerbHasDescription`, `TestEveryVerbHasSummary`,
`TestEveryVerbHasClassification`, `TestSeeDocsAnchorsResolve`,
`TestWriteVerbsDeclareNoOutputParameter`, `TestDescriptionLengthBounded`,
`TestColdStartSchemaSize_Reduction`, `TestCommittedManifestMatchesRecord`,
`TestManifestDescribesEveryRegisteredVerb`, `TestSharedParametersNameTheirWriteVerbs`,
`TestEveryEnvLiteralIsEnumerated`, `TestInventoryEntriesAreComplete`,
`TestScopes_MailManage`, `TestScopes_NoMailSend`.

## Diff Coverage

Range `ee23a77..HEAD`. 26 files, +2545 / -221.

| File | +/- | Mapped Requirements |
|---|---|---|
| `internal/tools/add_attachment.go` | +208 / -0 | FR-4, FR-6, FR-7, FR-10, FR-11, FR-14, FR-15, NFR-1, NFR-4, NFR-6, NFR-7, NFR-8 |
| `internal/tools/add_attachment_test.go` | +304 / -0 | AC-2, AC-4, AC-5, AC-6, AC-7, AC-10, AC-18 (indirect) |
| `internal/tools/attachment_upload_session.go` | +282 / -0 | FR-8, FR-10, FR-15, NFR-1, NFR-3, NFR-4, NFR-5, NFR-8 |
| `internal/tools/attachment_upload_session_test.go` | +246 / -0 | AC-3, AC-8 |
| `internal/tools/attachment_confirmation.go` | +69 / -0 | FR-10, NFR-1 |
| `internal/tools/attachment_confirmation_test.go` | +80 / -0 | AC-4, AC-10 |
| `internal/tools/update_draft.go` | +16 / -10 | FR-5, AC-18 |
| `internal/tools/delete_draft.go` | +1 / -1 | FR-5, AC-18 |
| `internal/validate/validate.go` | +34 / -0 | FR-4, NFR-8 |
| `internal/validate/validate_test.go` | +61 / -0 | AC-6 |
| `internal/server/mail_verbs.go` | +54 / -3 | FR-1, FR-2, FR-3, FR-9, FR-11, FR-12, FR-13, FR-14, FR-16, NFR-6, NFR-8 |
| `internal/server/mail_verbs_test.go` | +94 / -0 | AC-14 |
| `internal/server/server.go` | +1 / -1 | FR-17 |
| `internal/server/server_test.go` | +85 / -0 | AC-1, AC-11 |
| `internal/server/readonly_test.go` | +61 / -0 | AC-16 |
| `internal/tools/tool_annotations_test.go` | +62 / -0 | AC-15 |
| `internal/tools/dispatch_registry_test.go` | +1 / -0 | FR-21, AC-11 |
| `internal/docs/catalog_test.go` | +43 / -15 | FR-24, AC-19 |
| `internal/config/inventory.go` | +2 / -2 | FR-25 |
| `extension/manifest.json` | +1 / -1 | FR-1, FR-18 |
| `site/src/generated/surface.json` | +10 / -4 | FR-19, FR-23, FR-25, AC-11 |
| `docs/concepts.md` | +1 / -1 | FR-24, AC-19 |
| `docs/troubleshooting.md` | +37 / -2 | FR-24, AC-19 |
| `docs/prompts/mcp-tool-crud-test.md` | +14 / -1 | FR-20, AC-19 |
| `docs/cr/CR-0079-mail-draft-attachments.md` | +581 / -178 | Governance document itself (review commit `0bcf168` and finalization `b270559`) |
| `docs/backlog/cr-0078-0083.md` | +197 / -2 | Open Questions 1, 2, 4 — the CR names this file at `docs/cr/CR-0079-mail-draft-attachments.md:1327` as where the settled decisions are recorded |

### Unmapped changed files

None. Two files are outside the CR's Affected Components list and both are justified:

* `docs/cr/CR-0079-mail-draft-attachments.md` — the governance document under validation.
* `docs/backlog/cr-0078-0083.md` — explicitly named by the CR's Open Questions section as
  the record for the settled decisions, and by the per-phase implementation ledger. Touched
  by the review commit and by every implementation phase commit.

Two files the CR explicitly predicted would need **no** change, and which correctly show no
hunk: `scripts/crud-test.sh` (per-domain accounting keys on the tool name, not the verb) and
`docs/bench/crud-runs.csv` (header is per-domain). Likewise `internal/config/config.go`,
`go.mod`, `go.sum`, and everything under `internal/auth/`.

## Gaps

**GAP-1 — `TestVerifyIsDraftReturnsSubject` was never written.**
*Requirement ref:* Test Strategy "Tests to Add"; AC-18 first clause; FR-5.
*What's missing:* The CR specifies a test in `internal/tools/update_draft_test.go` asserting
that the widened guard returns the fetched message with its subject populated and issues
exactly one GET. That file was not touched by this CR. The `$select` widening at
`internal/tools/update_draft.go:230` is evidenced only by reading the source. The outcome is
covered indirectly by `TestAddAttachment_DirectPath` (`rec.gets == 1`, subject in the
confirmation), but that fixture returns `subject` regardless of the `$select`, so it cannot
detect a regression that drops `subject` from the projection — which would silently make the
confirmation read `(No subject)` against a real mailbox.
*Suggested minimal fix:* Add `TestVerifyIsDraftReturnsSubject` to
`internal/tools/update_draft_test.go`, calling `verifyIsDraft` directly against an `httptest`
fixture that records `req.URL.Query().Get("$select")`, asserting the projection contains
`subject`, that exactly one GET is issued, and that the returned `models.Messageable` carries
the subject.

**GAP-2 — Live-mailbox verification deferred.**
*Requirement ref:* CR "Verification Commands" narrative — `make crud-test` after rebuilding
the binary, plus a user scenario under `.agents/scenarios/` creating a draft, attaching a
small file through the direct path and a larger file through the session path, and confirming
through `list_attachments` that both ids resolve.
*What's missing:* Neither was performed. `make crud-test` is a paid harness and was excluded
by instruction. `.agents/scenarios/` contains only
`2026-09-01-received-message-management-lifecycle.md` (CR-0078); no CR-0079 scenario exists.
The unit suite serves the upload URL from an in-process `httptest` server, so **the chunked
transfer has never been exercised against Microsoft Graph** — the `Content-Range` contract,
the `Location` header shape on the completing 201, and the real service's inline threshold
are all unconfirmed against the live service.
*Suggested minimal fix:* Deferred to the user. Rebuild to the path in `.mcp.json` with the
`-ldflags` commit stamp, confirm the report's `Server version` line against
`git rev-parse --short HEAD`, run `make crud-test`, then derive and persist the two-path
attachment scenario under `.agents/scenarios/` per the `user-scenario` skill.

**GAP-3 — No test asserts the upload URL is absent from the log record.**
*Requirement ref:* FR-15; AC-8 second clause ("neither the tool result **nor the log record**
contains the pre-authenticated upload URL").
*What's missing:* `TestUploadSession_ErrorRedactsUploadURL` reads only the tool result. The
log record is safe by construction — `transferAttachmentChunks` redacts at
`internal/tools/attachment_upload_session.go:171` before the error reaches
`logger.ErrorContext` at `internal/tools/add_attachment.go:143-147` — but that ordering is a
one-line invariant with no guard. Moving the redaction, or adding a second log call on the
raw error, would leak a credential and pass every existing test. This is the highest-impact
gap in the report: it guards Risk 2, rated "likelihood high without the mitigation, impact
high".
*Suggested minimal fix:* Extend `TestUploadSession_ErrorRedactsUploadURL` to install a
capturing `slog.Handler` on the context via `logging`, and assert that neither `up.url` nor
`upload-secret` appears in any emitted record's rendered attributes.

**GAP-4 — The audit record is not asserted for `mail.add_attachment`.**
*Requirement ref:* AC-16 second clause; FR-13.
*What's missing:* `TestMailManagementVerbsCarryDotIdentity`
(`internal/server/mail_verbs_test.go:353`) enumerates its verbs by hand at `:378` and covers
only the four CR-0078 verbs. The identity is single-source through `wrapWrite`
(`internal/server/mail_verbs.go:107`) and `TestReadOnlyBlocksAddAttachment` proves the string
reaches the chain, so the risk is low; but the CR's own criterion names the audit record, and
the hand-written verb list is the same list-of-known-instances shape the project documents as
failing to close a class.
*Suggested minimal fix:* Add `"add_attachment"` to the `want` slice at
`internal/server/mail_verbs_test.go:378` (`AuditWrap` emits regardless of the handler's
outcome, so the existing `message_id`-only argument map is sufficient), or better, derive the
slice from the registry so a future gated verb cannot be omitted.

**GAP-5 — The verb's timeout and Graph-error paths are untested.**
*Requirement ref:* AC-17 first clause; NFR-5 per-chunk timeout clause.
*What's missing:* No test exercises `graph.IsTimeoutError` on this verb, so
`internal/tools/add_attachment.go:137-141` (session path) and `:181-186` (direct path) and
the per-chunk `graph.WithTimeout` at `internal/tools/attachment_upload_session.go:201` are
source-reading only. The CR's Test Strategy does not specify one either, so this is a
coverage gap in the CR as much as in the diff.
*Suggested minimal fix:* One test with a fixture that blocks past a 1 ms handler timeout,
asserting the result carries the seconds figure from `graph.TimeoutErrorMessage`. A second
subtest with a stalling chunk handler covers the NFR-5 per-chunk bound.

**GAP-6 — Validation errors carry the fix instruction on one channel only.**
*Requirement ref:* FR-15 ("Every error the verb raises **MUST** carry a fix instruction …
and **MUST** reach both the tool result and the log record").
*What's missing:* The eight validation refusals at `internal/tools/add_attachment.go:87-111`
return a fix-bearing tool result but emit no log record at all, and the oversize log record
at `:115` carries `"size"` and `"max"` but not the `OUTLOOK_MCP_MAX_ATTACHMENT_SIZE_BYTES`
name its tool result carries. This matches the established pattern across the mail write
verbs (`internal/tools/move_message.go:65-77` is identical), so the honest reading is that
FR-15's literal wording is stricter than the convention the codebase follows.
*Suggested minimal fix:* Either add a `logger.WarnContext` with a `"fix"` attribute to the
validation branches of `add_attachment.go` — which would make this verb inconsistent with its
nine siblings — or amend FR-15 to scope the log-record clause to errors that already produce
one. The second is the smaller and more truthful change; it is a CR amendment, not a code fix.

**GAP-7 — `TestAddAttachment_NoOutputParameter` was not written.**
*Requirement ref:* Test Strategy "Tests to Add"; AC-6 second clause.
*What's missing:* The named bespoke test does not exist.
*Suggested minimal fix:* None required. The CR's own "Existing Tests That Gate This Change
Without Modification" table names `TestWriteVerbsDeclareNoOutputParameter` as the AC-6 gate,
and that check is registry-derived across every domain, so it is strictly stronger than the
bespoke test it displaces. Recorded for completeness; the criterion is graded.
