---
id: "CR-0079"
name: mail-draft-attachments
description: Add one mail domain write verb that attaches a file to an existing draft, choosing a direct upload for small files and a chunked upload session for large ones.
status: "completed"
date: 2026-09-01
completed-date: 2026-09-02
requestor: desek
stakeholders:
  - desek
priority: "medium"
target-version: "0.11.0"
source-branch: docs/cr-implementation-set-0079-0083
source-commit: 5f5c4ca
---

# Mail Draft Attachments

## Change Summary

The `mail` domain can create, update, and delete a draft, and it can read the
attachments already on a message. It cannot add an attachment. A caller that has
assembled a draft through `create_draft` or `create_reply_draft` has no way to
put a file on it, so any draft that needs a document must be finished by hand in
Outlook, which is the same dead end for composition that CR-0078 closed for
triage.

This change adds exactly one write verb to the existing `mail` domain registry:
`add_attachment`. It takes a draft `message_id`, the file bytes as base64, a
`name`, and an optional MIME type, and it internally chooses the transfer path:
a direct `POST /me/messages/{id}/attachments` for a small file, and a chunked
`createUploadSession` upload for a file above the Graph inline limit. No new
top-level MCP tool is added; the aggregate tool count stays at four.

**CR-0078 has landed.** It is complete on this branch at `ee23a77`, and this
document has been reconciled against that tree rather than against the state it
was authored on. It uses the mail surface CR-0078 left behind: the same
`MailManageEnabled` gate, the same write middleware chain, and the same
`verifyIsDraft` draft guard the draft verbs apply. The measured starting surface
is **mail 17 verbs full and 5 default, totals 46 full and 33 default**, read from
`site/src/generated/surface.json`. Where this document states a verb count or a
totals figure it is stated against that baseline, and the generated surface
manifest remains the authority rather than any number written here.

## Motivation and Background

The feature-gap matrix places two mail rows in scope for this cycle and assigns
both to a Change Request:

> * **add attachment** — create_draft/update_draft take no attachment parameter
>   (SDK attachments POST confirmed); drafts cannot carry files. `4`.
> * **large attachment upload session** — no chunked upload-session verb (SDK
>   item_messages_item_attachments_create_upload_session confirmed); large
>   attachments unsupported (moot while attach is absent). `4`.

The second row is explicitly gated on the first: a chunked upload session is
"moot while attach is absent". The two are therefore one verb with two internal
paths, not two verbs. A caller does not choose the transfer mechanism; the size
of the file chooses it. Splitting the rows into two verbs would put the choice
of a byte threshold in the hands of the LLM, which is exactly the decision the
verb should make for it.

The OAuth cost of closing both rows is zero. `MAIL_MANAGE_ENABLED` already
requests `Mail.ReadWrite`, which is the scope both the direct POST and the upload
session need. No new consent prompt, no new scope, no new dependency.

The blast radius is small and draft-scoped by construction. The verb attaches to
a **draft** and refuses anything else, mirroring `update_draft` and
`delete_draft`, which fetch the target and reject a non-draft message. Attaching
to received mail is out of scope in the matrix, and the draft guard is what keeps
this change inside that boundary. Nothing sends: `Mail.Send` stays unrequested,
preserving the drafts-only safety property.

## Change Drivers

* The matrix marks "add attachment" in scope and assigns it to a CR; the draft
  composition path is a dead end without it.
* The "large attachment upload session" row is in scope but only as the large-file
  path of the same capability, so it belongs inside the one verb rather than in a
  verb of its own.
* Microsoft Graph splits attachment upload at a size threshold: a file below the
  inline limit is a single `POST`, and a file at or above it requires a
  `createUploadSession` followed by chunked transfer. This is a property of the
  service the verb must absorb, not surface.
* The attachment id the caller needs to later remove or reference the file is
  minted by the service and is not otherwise observable, so it is a confirmation
  requirement rather than a nicety.
* The four aggregate domain tools are a fixed surface, so growth belongs in a
  domain's verb registry rather than in a fifth tool.

## Current State

After CR-0078 the mail domain registers nine write verbs in the
`MailManageEnabled` tier of `buildMailVerbs` (`internal/server/mail_verbs.go:131`):
`create_draft`, `create_reply_draft`, `create_forward_draft`, `update_draft`,
`delete_draft`, `move_message`, `set_flag`, `set_categories`, and `mark_read`.
Every one of them operates on a message the caller already has, and the two
draft-mutating verbs refuse a non-draft: `verifyIsDraft`
(`internal/tools/update_draft.go:218`) fetches the message with a narrow
`$select` of `id` and `isDraft` and returns `"message is not a draft: this tool
only operates on messages with isDraft=true"` when `isDraft` is false. It is
called from `update_draft.go:105` and `delete_draft.go:76`, and it returns only
`*mcp.CallToolResult`, discarding the message it fetched.

The read side already reaches attachments. `list_attachments`
(`internal/tools/list_attachments.go`) enumerates attachment metadata via
`GET /me/messages/{id}/attachments`, and `get_attachment`
(`internal/tools/get_attachment.go`) downloads one attachment's bytes and is
bounded by `MaxAttachmentSizeBytes` to protect server memory. The server can
therefore list and read attachments, and cannot create one: a draft the model
composed cannot be given the document it was composed to send.

Measured facts about the surface, read from the repository rather than assumed:

* The published surface at `ee23a77` is mail 17 verbs full and 5 default, totals
  46 full and 33 default (`site/src/generated/surface.json`).
* The mail write tier is gated by `MailManageEnabled`, which requests
  `Mail.ReadWrite` (`docs/concepts.md:112`). `Mail.Send` is not requested in any
  configuration.
* Draft mutations already pay one `GET` for the `verifyIsDraft` guard before their
  write, so a guarded draft write issuing more than one Graph request is the
  established shape, not a new cost this verb introduces.
* `get_attachment` already carries a size ceiling and an actionable oversize error
  naming the environment variable that raises it
  (`internal/tools/get_attachment.go:128`), so a bounded-size write error has a
  pattern to follow.
* **`get_attachment` does not stream.** It reads the whole attachment into memory
  through the SDK and returns it base64-encoded in the tool result
  (`internal/tools/get_attachment.go:112` and `:132`), so `MaxAttachmentSizeBytes`
  is already a memory bound on a fully buffered payload, not a bound on a stream.
  This is the measurement that settles Open Question 1 below: the download and
  upload directions have the same memory shape, so they take the same bound.
* CR-0078 introduced `FormatMailWriteConfirmation`
  (`internal/tools/mail_write_confirmation.go:39`), the mail write confirmation
  shape: `Message <action>: "<subject>"`, then `ID: <id>`, then one
  `<field>: <value>` line, then an optional consequence line. It carries exactly
  one field-and-value pair.
* CR-0078 also introduced `TestSharedParametersNameTheirWriteVerbs`
  (`internal/server/mail_verbs_test.go:161`), which derives its cases from the
  registry, and `TestManifestDescribesEveryRegisteredVerb`
  (`internal/server/manifest_sync_test.go:84`), which asserts each domain's
  `extension/manifest.json` description names every verb registered for it.

### Current State Diagram

```mermaid
flowchart TD
    A["Caller composes a draft"] --> B["create_draft or create_reply_draft"]
    B --> C["Draft exists in Drafts folder"]
    C --> D{"Needs a file attached?"}
    D -->|"Read the files it has"| E["list_attachments, get_attachment"]
    D -->|"Add a file"| F["No verb exists"]
    F --> G["User attaches the file by hand in Outlook"]
```

## Proposed Change

One verb, `add_attachment`, is added to the `MailManageEnabled` tier of
`buildMailVerbs`. It requires a draft `message_id`, a `name`, and the file bytes
as base64 `content_bytes`, accepts an optional `mime_type`, verifies the target
is a draft, chooses the transfer path from the decoded size, and returns an
unconditional text confirmation naming the new attachment id.

### Verb inventory

| Verb | Graph call (small path) | Graph call (large path) | Required parameters | Confirmation carries |
|---|---|---|---|---|
| `add_attachment` | `POST /me/messages/{id}/attachments` with a `fileAttachment` | `POST /me/messages/{id}/attachments/createUploadSession`, then chunked `PUT` to the returned upload URL | `message_id`, `name`, `content_bytes` | draft subject, message id, attachment name, size in bytes, **attachment id**, and which transfer path was used |

The Graph SDK surfaces are confirmed against the pinned `msgraph-sdk-go v1.100.0`
in the module cache, not from documentation. Every line reference below was
re-read from the module cache at review time and still resolves. Both endpoints
live under the `users/` package of the v1.0 GA module (`module
github.com/microsoftgraph/msgraph-sdk-go`), not the beta SDK, so both are v1.0 GA:

* **Small path.** `client.Me().Messages().ByMessageId(id).Attachments().Post(ctx,
  Attachmentable, cfg)` returns `models.Attachmentable`
  (`users/item_messages_item_attachments_request_builder.go:110`). The body is a
  `models.FileAttachment` built by `models.NewFileAttachment()`, which stamps the
  `@odata.type` of `#microsoft.graph.fileAttachment`
  (`models/file_attachment.go:14`). It carries `SetContentBytes([]byte)`
  (`models/file_attachment.go:126`) and inherits `SetName(*string)`,
  `SetContentType(*string)`, and `SetSize(*int32)` from the embedded `Attachment`
  base (`models/attachment.go:224`, `:203`, `:231`). The returned attachment
  carries the service-assigned id.
* **Large path.**
  `client.Me().Messages().ByMessageId(id).Attachments().CreateUploadSession().Post(ctx,
  body, cfg)` returns `models.UploadSessionable`
  (`users/item_messages_item_attachments_create_upload_session_request_builder.go:43`;
  accessor at `users/item_messages_item_attachments_request_builder.go:84`). The
  body is `users.NewItemMessagesItemAttachmentsCreateUploadSessionPostRequestBody()`
  with `SetAttachmentItem(AttachmentItemable)`
  (`users/item_messages_item_attachments_create_upload_session_post_request_body.go:99`).
  The `AttachmentItem` (`models.NewAttachmentItem()`) carries
  `SetAttachmentType(*AttachmentType)` set to `models.FILE_ATTACHMENTTYPE`
  (`models/attachment_type.go:8`), `SetName`, `SetContentType`, and
  `SetSize(*int64)` (`models/attachment_item.go:267`, `:299`, `:285`, `:313`). The
  returned `UploadSession` exposes `GetUploadUrl()`, `GetExpirationDateTime()`,
  and `GetNextExpectedRanges()` (`models/upload_session.go:137`, `:49`, `:113`).

The SDK returns the upload session but does not transfer the chunks: the file
bytes are written by `PUT` requests to the pre-authenticated upload URL, which is
raw HTTP outside the request-builder chain. That boundary is a deliberate design
point, addressed in the implementation approach and in the non-functional
requirements below.

### The inline threshold

Microsoft Graph accepts a single `POST` of a `fileAttachment` below its inline
limit and requires an upload session at or above it. The threshold is
`3 * 1000 * 1000 = 3000000` bytes, and the routing is **strictly below the
threshold** for the direct `POST`, **at or above it** for the upload session.

Two properties decide the exact value and the strictness, and both point the same
way. Graph states the direct-`POST` limit in decimal megabytes, not mebibytes, so
3 MiB (`3145728`) is already over it. And the direct `POST` carries the bytes
base64-encoded inside a JSON request body, which expands them by four thirds:
`3000000` decoded bytes become `4000000` encoded characters, at Graph's request
body ceiling. A file at the boundary therefore routes to the upload session, which
handles every size the upper cap admits. Misrouting downward fails the request;
misrouting upward only costs an extra round trip, so the boundary resolves toward
the session path.

The verb decodes `content_bytes`, measures the decoded byte length, and routes on
it. The threshold is an internal constant, not a parameter, so the caller never
chooses the mechanism.

### Annotation matrix

The verb declares all four hints explicitly, per the project rule that a verb
cannot be registered without its own classification and that the aggregate is
computed from the registry.

| Verb | `readOnlyHint` | `destructiveHint` | `idempotentHint` | `openWorldHint` |
|---|---|---|---|---|
| `add_attachment` | `false` | `false` | `false` | `true` |

Justification, hint by hint, because an unjustified hint is the defect the
computed-annotation rule exists to prevent:

* **`readOnlyHint: false`.** The verb writes mailbox state: it adds an attachment
  to a draft.
* **`destructiveHint: false`.** The operation is purely additive. It removes
  nothing and it makes nothing unaddressable: the draft and its existing
  attachments are untouched, and a new attachment appears alongside them. This is
  the same shape as `create_draft`, classified `false` today.
* **`idempotentHint: false`.** A second call with identical arguments adds a
  second identical attachment with a distinct id, rather than leaving the same
  end state. The operation mints a new attachment id on every successful call, so
  it cannot be idempotent. This matches `create_draft`, also `false`.
* **`openWorldHint: true`.** The verb calls Microsoft Graph.

The aggregate fold is unchanged by these values. Under `MailManageEnabled` the
`mail` tool already publishes `destructiveHint: true` (forced by `delete_draft`)
and `idempotentHint: false` (forced by `create_draft`), so the published
configuration-dependent annotation table in
`docs/concepts.md#tool-annotation-semantics` stays correct as written. Under the
default and `MailEnabled`-only configurations the verb is not registered, so
`readOnlyHint: true` is preserved there.

### The flattened aggregate schema, and why the MIME parameter is not `content_type`

`aggregateSchemaOptions` (`internal/tools/dispatch_aggregate_schema.go:25`)
publishes the union of every registered verb's parameters on the domain tool and
merges duplicate names with first-occurrence-wins for type, description, and enum.
The mail domain already declares a `content_type` parameter on `create_draft`
(`internal/server/mail_verbs.go:458`) and `update_draft`
(`internal/server/mail_verbs.go:570`), and in both it is a **body** content type
restricted by an enum to `text` and `html`. An attachment's content type is a MIME
type such as `application/pdf`, which that enum would reject and that description
would misdescribe.

Reusing `content_type` for the attachment MIME type is therefore not honest:
first-occurrence-wins keeps the draft verbs' `text | html` enum, so the flattened
schema would tell an LLM that the attachment's content type must be `text` or
`html`. This CR names the parameter `mime_type` instead, one concept to one name,
following the precedent CR-0078 set with `destination_folder_id` rather than
`folder_id`: two genuinely different concepts in one domain get two names, and the
distinct name is what keeps the merged schema correct.

`name`, `content_bytes`, and `mime_type` are new to the mail domain and collide
with nothing: the domain's declared parameter names at `ee23a77` are `account`,
`attachment_id`, `bcc_recipients`, `body`, `categories`, `cc_recipients`,
`comment`, `content_type`, `conversation_id`, `destination_folder_id`,
`end_datetime`, `flag_status`, `folder_id`, `from`, `has_attachments`,
`importance`, `is_draft`, `is_read`, `max_results`, `message_id`, `output`,
`provenance`, `query`, `reply_all`, `start_datetime`, `subject`, `timezone`, and
`to_recipients`. `message_id` and `account` are already published domain-wide, and
`message_id`'s description was already rewritten by CR-0078 to name its write
verbs. This CR introduces **no** filter-versus-write dual-sense parameter of the
kind that required the shared-parameter description rewrite in CR-0078:
`TestSharedParametersNameTheirWriteVerbs` selects only names declared by both a
read verb and a write verb, and the three new names are declared by one write verb
only, so that check continues to pass unchanged.

### Proposed State Diagram

```mermaid
flowchart TD
    A["Caller composes a draft, then calls add_attachment"] --> B{"MailManageEnabled?"}
    B -->|"No"| C["Verb not registered, mail write stays draft-body only"]
    B -->|"Yes"| D["Validate message_id, name, content_bytes"]
    D --> E["verifyIsDraft: reject a non-draft message, return its subject"]
    E --> F["Decode base64, measure size, reject over the upper cap"]
    F --> G{"Decoded size below 3000000 bytes?"}
    G -->|"Yes"| H["POST a fileAttachment"]
    G -->|"No"| I["createUploadSession, then chunked PUT to the upload URL"]
    H --> J["Confirmation names the new attachment id and the direct path"]
    I --> K["Confirmation names the new attachment id and the session path"]
```

## Requirements

### Functional Requirements

1. The system **MUST** register exactly one new verb in the `mail` domain verb
   registry, named `add_attachment`, and **MUST NOT** register any new top-level
   MCP tool; the registered tool count **MUST** remain four.
2. The verb **MUST** be registered only when `MailManageEnabled` is configured, and
   **MUST NOT** appear in the operation enum under the default or
   `MailEnabled`-only configurations.
3. The verb **MUST** require `message_id`, `name`, and `content_bytes`, and **MUST**
   accept an optional `mime_type`.
4. The verb **MUST** validate `message_id` with `validate.ValidateResourceID`,
   **MUST** reject an empty `name` and validate its length with
   `validate.ValidateStringLength` against an attachment-name bound, and **MUST**
   reject a `content_bytes` value that is empty or is not valid base64, in every
   case before any Graph request is issued.
5. The verb **MUST** verify the target is a draft with the same guard the draft
   verbs apply (the `verifyIsDraft` fetch), and **MUST** reject a non-draft message
   with the established not-a-draft refusal, because attaching to received mail is
   out of scope. Because the confirmation must name the draft subject (FR-10) and
   the guard currently discards the message it fetched, `verifyIsDraft` **MUST** be
   widened rather than duplicated: its `$select` **MUST** add `subject`, and it
   **MUST** return the fetched `models.Messageable` alongside its refusal result.
   Its two existing callers (`internal/tools/update_draft.go:105` and
   `internal/tools/delete_draft.go:76`) **MUST** be updated to the new signature
   and **MUST NOT** change behaviour, and both **MUST** still refuse a non-draft
   with the same message.
6. The verb **MUST** decode `content_bytes` from base64, measure the decoded byte
   length, and choose the transfer path from that length: **strictly below** the
   inline threshold of `3 * 1000 * 1000` bytes it **MUST** issue a single
   `POST /me/messages/{id}/attachments` carrying a `fileAttachment`; **at or above**
   the threshold it **MUST** create an upload session and transfer the bytes in
   chunks. The threshold **MUST NOT** be exposed as a parameter.
7. On the small path the `fileAttachment` **MUST** carry the decoded content bytes,
   the supplied `name`, and the content type from `mime_type` defaulting to
   `application/octet-stream` when omitted, with the `@odata.type` of
   `#microsoft.graph.fileAttachment` that `models.NewFileAttachment` stamps.
8. On the large path the verb **MUST** create the upload session with an
   `attachmentItem` whose attachment type is `file` and whose `name`,
   `contentType`, and `size` match the request, and **MUST** transfer the bytes by
   `PUT` to the upload URL the session returns, in chunks whose size is a multiple
   of 320 KiB except the final chunk, each carrying a
   `Content-Range: bytes {start}-{end}/{total}` header. The chunk size **MUST** be
   a named constant (NFR-8), not an inline literal.
9. The verb **MUST** name its attachment MIME-type parameter `mime_type` and
   **MUST NOT** reuse `content_type`, whose mail-domain declaration is a body
   content type restricted to `text` and `html`.
10. The confirmation **MUST** name the draft subject, the message id, the
    attachment name, the attachment size in bytes, the attachment id the service
    returns, and which transfer path was used, and **MUST** construct the
    attachment id from the Graph response rather than from the request arguments.
    On the small path the id **MUST** be read from the `Attachmentable` the `POST`
    returns. On the large path it **MUST** be read from the completion response the
    service returns on the final chunk, taking the identifier from that response's
    `Location` header, and falling back to the response body only where the service
    returns one. A missing draft subject **MUST** render as `(No subject)`, matching
    `FormatMailWriteConfirmation`.
11. The verb **MUST** return a text confirmation unconditionally and **MUST NOT**
    declare an `output` parameter, per the project's write-verb tiering rule.
12. The verb **MUST** declare all four annotation hints explicitly, with the values
    given in the annotation matrix above.
13. The verb **MUST** be wrapped by the write middleware chain (authentication,
    account resolution, observability, read-only guard, audit) under the identity
    `mail.add_attachment` and the audit operation `write`, so the audit record and
    the OpenTelemetry attributes carry the same `{domain}.{operation}` identity as
    every other verb.
14. The verb **MUST** enforce an upper bound on the decoded attachment size before
    any upload begins, and **MUST** reject an oversize attachment with an actionable
    error that names the measured size, the bound, and the environment variable that
    raises it, mirroring the oversize error `get_attachment` already emits on the
    read side. The bound **MUST** be the existing `config.MaxAttachmentSizeBytes`
    (`OUTLOOK_MCP_MAX_ATTACHMENT_SIZE_BYTES`, default `10485760`); this change
    **MUST NOT** introduce a second attachment size environment variable.
15. Every error the verb raises **MUST** carry a fix instruction naming what to
    supply or correct in its tool result. Where the verb emits a log record for a
    failure, that record **MUST** carry the same fix instruction, so a headless
    caller reading a persisted log receives the correction on the only channel it
    has. Following the convention every sibling mail write verb already applies, a
    refusal decided from the request arguments alone need not emit a log record:
    the caller holds the arguments, the operator's log is reserved for failures
    the caller cannot reconstruct, and a per-verb departure from that convention
    would make this verb inconsistent with its nine siblings for no gain. An error
    arising from a chunk `PUT` **MUST** be redacted so the pre-authenticated upload
    URL, which carries an access token, never appears in the result or the log.
16. The verb **MUST** carry a non-empty `Summary` of at most eighty characters, a
    non-empty `Description` stating its parameters, its gating requirement, and its
    annotation semantics, at least one `Examples` entry, and at least one `SeeDocs`
    reference that resolves to an existing heading in the embedded documentation
    bundle.
17. The `mail` domain introduction registered in `internal/server/server.go`
    **MUST** name `add_attachment` in its enumeration of `MailManageEnabled`-gated
    write verbs.
18. The `mail` entry of `extension/manifest.json` **MUST** enumerate
    `add_attachment`, and the manifest's `tools` array **MUST** remain exactly four
    entries, because no new top-level tool is added.
19. The generated surface manifest `site/src/generated/surface.json` **MUST** be
    regenerated with `make surface-manifest` and committed in the same change, so
    the published website states the surface that exists.
20. `docs/prompts/mcp-tool-crud-test.md` **MUST** gain one lifecycle step exercising
    `add_attachment`, appended as **Step 41** without renumbering any existing step,
    and the gated skip instruction at its Step 31 heading **MUST** read
    "Steps 31 through 35 and Steps 37 through 41", so a run with mail management
    disabled records the new step as skipped rather than failing. The step **MUST**
    attach to a draft the harness creates and deletes, never to a received message,
    because the verb refuses a non-draft and a received message cannot be restored
    to its unattached state.
21. The committed verb inventory golden in
    `internal/tools/dispatch_registry_test.go` **MUST** be regenerated to include
    the identity `mail.add_attachment ro=false de=false id=false ow=true`.
22. The change **MUST NOT** alter the OAuth scope set requested in any
    configuration: `Mail.ReadWrite` already covers the verb and `Mail.Send` stays
    unrequested.
23. The change **MUST NOT** alter the default verb surface: the mail domain's
    `defaultCount` in the surface manifest **MUST** remain 5, and its `fullCount`
    **MUST** rise from 17 to 18 while the totals rise from 46 to 47 full with 33
    default unchanged.
24. The embedded documentation bundle **MUST** be updated so a caller reading it
    mid-session can find the verb and recover from its two failure modes:
    - the `MAIL_MANAGE_ENABLED` row of the mail gating table in `docs/concepts.md`
      **MUST** name draft attachments alongside the draft management and
      received-message management it already names, and **MUST** keep naming
      `move_message`, `set_flag`, `set_categories`, and `mark_read`, which
      `TestMailGatingRowNamesMessageManagement` asserts;
    - the symptom list of the existing `Mail management disabled` entry in
      `docs/troubleshooting.md` **MUST** name `add_attachment`;
    - `docs/troubleshooting.md` **MUST** gain two entries with stable anchors, one
      for an attempt to attach to a non-draft message and one for an upload-session
      transfer failure or expiry.
25. The `MAIL_MANAGE_ENABLED` description in `internal/config/inventory.go` and the
    `MAX_ATTACHMENT_SIZE_BYTES` description alongside it **MUST** be widened to
    cover draft attachments and the upload direction respectively, because both are
    published verbatim into `site/src/generated/surface.json` and would otherwise
    describe a narrower server than the one that ships.

### Non-Functional Requirements

1. The verb's handler **MUST** live in its own file under `internal/tools/`, named
   for the verb, and the chunked upload-session transfer **MUST** live in its own
   file, so neither file carries the other's concern and each stays small.
2. The composed `mail` tool description **MUST** stay below 4 000 characters, and
   the cold-start schema reduction **MUST** stay at or above 60% against the
   documented baseline. Both bounds are asserted by existing gates
   (`TestDescriptionLengthBounded` and `TestColdStartSchemaSize_Reduction`) rather
   than by this document. The measured margins after CR-0078 were 2 737 of 4 000
   characters and 75% against a 60% floor, so one verb with three new parameters is
   admitted by both with room to spare; the gates, not this note, decide.
3. The change **MUST NOT** add a third-party dependency. The chunked `PUT` transfer
   **MUST** use the standard library HTTP client, not a new upload library.
4. The verb's Graph SDK calls (the `verifyIsDraft` `GET`, the small-path `POST`,
   and the `createUploadSession` `POST`) **MUST** route through
   `graph.RetryGraphCall` inside `graph.WithTimeout`, and **MUST** redact Graph
   errors with the existing helpers, so retry, timeout, and redaction behaviour is
   identical to the verbs already registered.
5. The chunked `PUT` transfer **MUST** be bounded by a per-chunk timeout and
   **MUST** surface a transfer failure as an actionable, redacted error rather than
   a partial success. A partially uploaded attachment that the service discards on
   session expiry **MUST NOT** be reported as attached.
6. The whole attachment is delivered to the verb as base64 in one argument and is
   therefore held in memory in full; the chunked path bounds the Graph transfer,
   not the memory footprint. The upper size bound of FR-14 is what keeps the memory
   footprint bounded, and it **MUST** be the existing `MaxAttachmentSizeBytes`
   (default `10485760`), which is already a memory bound on a fully buffered
   payload: `get_attachment` buffers the whole attachment too, so the two
   directions have the same shape and take the same bound. The default **MUST NOT**
   be raised toward the service's 150 MB ceiling in this change.
7. The small path **MUST** issue exactly the `verifyIsDraft` `GET` and one `POST`
   on its success path. The large path **MUST** issue the `verifyIsDraft` `GET`,
   one `createUploadSession` `POST`, and the minimum number of chunk `PUT`s the
   file size requires, and **MUST NOT** re-read the draft after the upload.
8. The attachment-name bound, the inline threshold, and the chunk size **MUST** each
   be defined once as a named constant, and the upper size bound **MUST** come from
   configuration, not from an inline literal at the call site, so each has a single
   source of truth.

## Affected Components

* `internal/tools/add_attachment.go` (new): the `add_attachment` handler
  constructor `NewHandleAddAttachment`. Validates parameters, runs the draft guard,
  decodes and measures the bytes, dispatches to the small or large path, and formats
  the confirmation.
* `internal/tools/attachment_upload_session.go` (new): the chunked upload-session
  transfer. Creates the session via the SDK, then `PUT`s the bytes to the upload URL
  in 320 KiB-multiple chunks with `Content-Range`, and returns the created
  attachment id. Isolated from the handler so the raw-HTTP transfer concern does not
  bloat the handler file.
* `internal/tools/attachment_confirmation.go` (new): the confirmation formatter for
  an added attachment. Two existing formatters were considered and neither fits.
  `FormatDraftConfirmation` (`internal/tools/draft_helpers.go:84`) closes with the
  draft-specific Drafts-folder sentence and names a draft, not an attachment.
  `FormatMailWriteConfirmation` (`internal/tools/mail_write_confirmation.go:39`),
  which CR-0078 introduced as the mail write confirmation shape, carries exactly one
  field-and-value pair, and this confirmation reports four (attachment name, size,
  attachment id, transfer path). The CR-0078 review recorded that a verb needing two
  changed fields extends that signature rather than getting its own formatter; four
  is past where a positional signature stays readable, so a dedicated formatter is
  the smaller change. It **MUST** follow the established line order:
  `Attachment added: "<name>"`, then `Message: "<draft subject>"`, then
  `Message ID`, `Attachment ID`, `Size`, and `Transfer`.
* `internal/tools/update_draft.go`: `verifyIsDraft` gains `subject` in its `$select`
  and returns the fetched `models.Messageable` alongside its refusal result (FR-5),
  so the confirmation can name the draft without a second `GET`. Its call site at
  line 105 is updated to the new signature with no behaviour change.
* `internal/tools/delete_draft.go`: the second `verifyIsDraft` call site (line 76),
  updated to the new signature with no behaviour change.
* `internal/validate/validate.go`: a `ValidateBase64` helper and an attachment-name
  length bound alongside the existing bounds, returning errors that name the
  parameter and the correction (NFR-8). The existing `ValidateContentType`
  (line 218) validates the **body** content type against `text` and `html` and
  **MUST NOT** be applied to `mime_type`.
* `internal/config/inventory.go`: the `OUTLOOK_MCP_MAX_ATTACHMENT_SIZE_BYTES`
  description (line 92), widened from "returned by `get_attachment`" to cover both
  directions, and the `OUTLOOK_MCP_MAIL_MANAGE_ENABLED` description (line 91),
  widened to name draft attachments (FR-25). **No new environment variable**, so
  `TestEveryEnvLiteralIsEnumerated` and `TestInventoryEntriesAreComplete` are
  unaffected and `internal/config/config.go` needs no change (FR-14, NFR-6).
* `internal/server/mail_verbs.go`: a new `buildAddAttachmentVerb` constructor
  appended to the `MailManageEnabled` block of `buildMailVerbs` (line 131), carrying
  `Summary`, `Description`, `Examples`, `SeeDocs`, `Annotations`, and `Schema` per
  FR-12 and FR-16.
* `internal/server/server.go`: the `mail` domain `Intro` string, which enumerates
  the `MailManageEnabled` write verbs by name (FR-17).
* `extension/manifest.json`: the `mail` tool description. The `tools` array itself is
  **unchanged at four entries** (FR-18). `TestManifestDescribesEveryRegisteredVerb`
  (`internal/server/manifest_sync_test.go:84`) derives its cases from the registry
  and fails until this edit lands, so the edit is gated rather than remembered.
* `site/src/generated/surface.json`: regenerated, not hand-edited. The mail domain's
  `fullCount` rises 17 to 18 and its `defaultCount` stays 5; the totals rise 46 to 47
  full with 33 default unchanged. The regeneration also picks up the two widened
  configuration descriptions of FR-25, since the manifest's `config` array is built
  from `config.Inventory()` (`internal/surface/build.go:167`). Gate attribution is
  derived by probe, so no mapping needs maintaining (FR-19).
* `docs/concepts.md`: the mail gating table row for `MAIL_MANAGE_ENABLED` (line 65).
  CR-0078 already widened this row from its former "including draft management"
  wording; it now names both draft management and received-message management with
  the verbs of each. This change adds draft attachments to it and **MUST** preserve
  the strings `draft management`, `received-message management`, `move_message`,
  `set_flag`, `set_categories`, and `mark_read`, which
  `TestMailGatingRowNamesMessageManagement` asserts against the embedded bundle. The
  OAuth scopes table and the annotation-semantics table are **unchanged**, and this
  CR asserts that rather than leaving it to inference.
* `docs/troubleshooting.md`: two new entries with stable anchors, one for an attempt
  to attach to a non-draft message and one for an upload-session transfer failure or
  expiry, which are the two failure modes a caller cannot diagnose from the Graph
  error alone; plus `add_attachment` added to the symptom list of the existing
  `Mail management disabled` entry (line 206), which already enumerates the nine
  gated write verbs and would otherwise read as exhaustive (FR-24).
* `internal/docs/catalog_test.go`: `TestMailGatingRowNamesMessageManagement`, extended
  to assert the gating row also names `add_attachment`. CR-0078 established that the
  concepts gating row gets its gate in this file, asserted against the embedded
  bundle rather than the file on disk, because the bundle is what a running server
  serves to an LLM mid-session.
* `docs/prompts/mcp-tool-crud-test.md`: one new lifecycle step and the widened skip
  range (FR-20).
* `internal/tools/dispatch_registry_test.go`: the `verbInventoryGolden` list (FR-21).
* `scripts/crud-test.sh`: **no change required.** Its per-domain accounting keys on
  the MCP tool name `mcp__outlook-local-mcp__mail`, not on the operation verb, so an
  additional mail verb raises the existing `mcp_mail` counter without a script edit.
* `docs/bench/crud-runs.csv`: **no column change.** The header is per-domain, not
  per-verb; new runs record a higher `mcp_mail` value and more turns.

## Scope Boundaries

### In Scope

* Exactly one new mail verb: `add_attachment`, covering both the direct-POST small
  path and the chunked upload-session large path.
* Its handler, the chunked-transfer helper, the confirmation formatter, the registry
  entry, annotations, schema, and unit tests.
* The base64 and attachment-name validation helpers, the inline threshold and chunk
  size constants, and the reuse of the existing `MaxAttachmentSizeBytes` as the
  upper cap.
* Widening `verifyIsDraft` to `$select` the subject and return the fetched message,
  and updating its two existing call sites to the new signature without changing
  their behaviour.
* The domain introduction, the extension manifest mail description, the two widened
  configuration inventory descriptions, and the regenerated surface manifest.
* The concepts gating-row wording and its gate in `internal/docs/catalog_test.go`,
  two new troubleshooting entries plus the widened `Mail management disabled`
  symptom list, and the lifecycle harness step.

### Out of Scope ("Here, But Not Further")

* **Attaching to a received message.** The verb attaches to a **draft** and refuses
  anything else, mirroring the draft guard the draft verbs apply. The matrix places
  received-mail writes outside this cycle's scope, and the draft guard is the
  boundary.
* **Removing an attachment.** `DELETE /me/messages/{id}/attachments/{id}` is the
  matrix's "delete attachment" row, marked destructive and out of scope (Manage=3).
  It belongs in its own change request. This verb only adds.
* **Sending, replying, or forwarding.** `Mail.Send` stays unrequested. The
  drafts-only safety property is preserved without qualification.
* **Item and reference attachments.** This verb creates a `fileAttachment` (file
  bytes). Attaching an Outlook item (`itemAttachment`) or a link to a cloud file
  (`referenceAttachment`) is a different body shape and is not added here.
* **Inline attachments.** Setting `isInline` and a `contentId` to embed an image in
  an HTML body is a separate compositional concern and is not exposed; every
  attachment this verb adds is a regular file attachment.
* **Streaming from disk or a URL.** The bytes arrive as base64 in the tool argument.
  The verb does not read a local path or fetch a remote URL, which would be a
  separate ingestion design.
* **Attaching to calendar events.** The matrix's calendar attachment rows
  (add, get, list, upload session) are a separate domain and a separate CR. This
  change touches the mail domain only.
* **Resuming an interrupted upload session.** The upload session's
  `nextExpectedRanges` supports resumption; this verb performs a single forward
  transfer and reports a failure rather than resuming a prior session. Resumption is
  a larger reliability feature deferred to its own change.

## Alternative Approaches Considered

* **Two verbs, one for small files and one for large.** Rejected: the matrix marks
  the large-upload row "moot while attach is absent", so it is the same capability
  at a different size. Two verbs would force the LLM to choose a byte threshold it
  cannot see, which is precisely the decision the verb should make internally.
* **A single `content_type` parameter shared with the draft body content type.**
  Rejected: the mail domain's `content_type` is a body type restricted to `text` and
  `html`, and first-occurrence-wins merging would publish that enum on the attachment
  parameter, misdescribing a MIME type. `mime_type` keeps one concept to one name.
* **Adding an optional attachment parameter to `create_draft` and `update_draft`
  instead of a new verb.** Rejected: it would spread one honest annotation
  classification across verbs whose other operations are pure body edits, and it
  would put file bytes into every draft-edit schema. A dedicated verb keeps the
  attachment concern, and its non-idempotent additive semantics, in one place.
* **Using a large-file upload task abstraction from a third-party or kiota upload
  library.** Rejected on the no-new-dependency rule: the standard library HTTP client
  performs the chunked `PUT` against the pre-authenticated upload URL, which is all
  the large path needs.
* **Reporting a partially uploaded attachment as success.** Rejected: a transfer that
  fails partway leaves a session the service discards on expiry, so reporting success
  would name an attachment id that does not resolve. A partial transfer is a failure,
  redacted and actionable.
* **A distinct `OUTLOOK_MCP_MAX_ATTACHMENT_UPLOAD_BYTES` bound.** Rejected on a
  measurement rather than on taste. The case for a distinct bound was that the
  download side streams while the upload side buffers, and reading the code shows it
  does not: `get_attachment` deserialises the whole attachment through the SDK and
  returns it base64-encoded in the tool result, so both directions hold the full
  payload in memory. With the asymmetry gone, a second variable would add an
  inventory row, a config field, a surface-manifest entry, and a second number for a
  user to reconcile, in exchange for nothing. `MaxAttachmentSizeBytes` is reused.
* **Duplicating the draft guard rather than widening it.** Rejected: a second copy of
  the `isDraft` fetch would put the refusal string in two places, and the CR-0078
  review already recorded that correcting named instances is the remedy this project
  treats as failing to close a class. `verifyIsDraft` is widened once and its two
  call sites follow.

## Impact Assessment

### User Impact

A user who has enabled `MAIL_MANAGE_ENABLED` gains the ability to have the model
attach a file to a draft it composed, closing the composition path that otherwise
ends in Outlook. Nothing changes for a user who has not enabled it, and nothing
changes about sending. The one behaviour a user must understand is that attaching
the same file twice produces two attachments, which the non-idempotent
classification and the description both disclose.

### Technical Impact

No breaking change, no schema migration, no new dependency, and no new OAuth scope.
The default configuration's published tool surface is byte-identical, because the
verb is gated. Under `MailManageEnabled` the published `mail` tool grows by one
operation-enum entry, one inventory line, and three new flattened parameters
(`name`, `content_bytes`, `mime_type`); `message_id` and `account` are already
published. The large path introduces the one architectural novelty in this change,
a raw-HTTP chunked `PUT` outside the SDK request-builder chain, which is isolated to
its own file and bounded by a per-chunk timeout and the upper size cap.

### Business Impact

Low cost against a matrix row assigned to a CR. The work is one handler plus one
transfer helper, and the risk concentrates in the large path, whose contract this
document pins: chunk sizing, error redaction of the upload URL, and no partial
success.

## Implementation Approach

### Implementation Flow

```mermaid
flowchart LR
    subgraph P1["Phase 1: Validation and shared pieces"]
        A1["ValidateBase64 and name bound"] --> A2["Constants: inline threshold and chunk size"]
        A2 --> A3["Attachment confirmation formatter"]
        A3 --> A4["Widen verifyIsDraft, update its two call sites"]
    end
    subgraph P2["Phase 2: The small path"]
        B1["Handler: validate, draft guard, decode, measure"] --> B2["Build fileAttachment and POST"]
    end
    subgraph P3["Phase 3: The large path"]
        C1["createUploadSession via the SDK"] --> C2["Chunked PUT to the upload URL"]
        C2 --> C3["Redact the upload URL in every error"]
    end
    subgraph P4["Phase 4: Registry"]
        D1["buildAddAttachmentVerb with annotations and schema"] --> D2["mime_type, not content_type"]
    end
    subgraph P5["Phase 5: Surface, docs, harness"]
        E1["Domain intro, extension manifest, config inventory descriptions"] --> E2["Regenerate surface manifest"]
        E2 --> E3["Concepts row and its gate, troubleshooting, lifecycle step 41"]
        E3 --> E4["Regenerate the verb inventory golden"]
    end
    P1 --> P2 --> P3 --> P4 --> P5
```

### Phase 1: Validation and shared pieces

**Affected components:** `internal/validate/validate.go`,
`internal/validate/validate_test.go`, `internal/tools/attachment_confirmation.go`
(new), `internal/tools/attachment_confirmation_test.go` (new),
`internal/tools/update_draft.go`, `internal/tools/delete_draft.go`.

1. Add `validate.ValidateBase64(value, paramName string) ([]byte, error)` to
   `internal/validate/validate.go`, decoding the standard base64 encoding and
   returning an error naming the parameter when the value is empty or malformed.
   Add an attachment-name length bound alongside the existing bounds. Do **not**
   route `mime_type` through the existing `ValidateContentType`, which validates the
   body content type against `text` and `html`.
2. Define the inline threshold (`3 * 1000 * 1000`) and the chunk size (a 320 KiB
   multiple) as named constants (NFR-8). The upper cap is not a new constant: it is
   the existing `config.MaxAttachmentSizeBytes`, threaded to the handler as a
   parameter the way `NewHandleGetAttachment` already takes `maxSize` (FR-14, NFR-6).
3. Add `internal/tools/attachment_confirmation.go` with a formatter that renders the
   attachment name, the draft subject (falling back to `(No subject)`), the message
   id, the attachment id, the size in bytes, and which transfer path was used. It
   follows the line shape of `FormatMailWriteConfirmation` rather than
   `FormatDraftConfirmation`, which closes with a Drafts-folder sentence that does
   not apply to an attachment.
4. Widen `verifyIsDraft` in `internal/tools/update_draft.go`: add `subject` to its
   `$select` and return the fetched `models.Messageable` alongside the refusal
   result, so the confirmation can name the draft without a second `GET` (FR-5).
   Update both call sites, `update_draft.go:105` and `delete_draft.go:76`, to the new
   signature, discarding the message. Their behaviour must not change, and the
   existing draft-guard tests for both verbs must pass unmodified.

### Phase 2: The small path

**Affected components:** `internal/tools/add_attachment.go` (new),
`internal/tools/add_attachment_test.go` (new).

1. `internal/tools/add_attachment.go`: `NewHandleAddAttachment(retryCfg, timeout,
   maxAttachmentBytes)`. Resolve the Graph client, require and validate
   `message_id`, `name`, and `content_bytes`, decode the bytes, and reject an
   oversize attachment against `MaxAttachmentSizeBytes` before any Graph request,
   with an error naming the measured size, the bound, and
   `OUTLOOK_MCP_MAX_ATTACHMENT_SIZE_BYTES` (FR-4, FR-14).
2. Run the draft guard through the widened `verifyIsDraft`, taking both the refusal
   result and the fetched message, and read the subject from it (FR-5, FR-10). A
   non-draft returns the established refusal before any attachment request.
3. Below the inline threshold, build a `models.FileAttachment` with the name, the
   MIME type (`mime_type` defaulting to `application/octet-stream`), and the decoded
   bytes, and `POST` via
   `client.Me().Messages().ByMessageId(id).Attachments().Post(...)` through
   `graph.RetryGraphCall` inside `graph.WithTimeout` (FR-6, FR-7, NFR-4).
4. Read the attachment id from the returned `Attachmentable` and format the
   confirmation naming the direct path. A boundary test pins the routing: a payload
   of exactly `3000000` decoded bytes takes the session path, not the `POST`.

### Phase 3: The large path

**Affected components:** `internal/tools/attachment_upload_session.go` (new),
`internal/tools/attachment_upload_session_test.go` (new),
`internal/tools/add_attachment.go` (the large-path branch).

1. `internal/tools/attachment_upload_session.go`: create the session with
   `users.NewItemMessagesItemAttachmentsCreateUploadSessionPostRequestBody()`, an
   `attachmentItem` of type `file` carrying the name, MIME type, and size, and `POST`
   it through the retry and timeout wrappers (FR-8, NFR-4).
2. Transfer the decoded bytes by `PUT` to `UploadSession.GetUploadUrl()` in chunks
   whose size is a multiple of 320 KiB except the last, each with a
   `Content-Range: bytes {start}-{end}/{total}` header, using the standard library
   HTTP client with a per-chunk timeout (FR-8, NFR-3, NFR-5).
3. Read the created attachment id from the completion response the service returns
   on the final chunk, taking it from that response's `Location` header and falling
   back to the response body only where the service returns one (FR-10). Redact the
   upload URL in every error so its access token never reaches the result or the log
   (FR-15). A transfer that does not complete, including one whose final response
   carries no identifier, is a failure rather than a partial success (NFR-5).

### Phase 4: Registry entry

**Affected components:** `internal/server/mail_verbs.go`,
`internal/server/mail_verbs_test.go`, `internal/tools/tool_annotations_test.go`.

1. Add `buildAddAttachmentVerb` to `internal/server/mail_verbs.go` and append it to
   the `MailManageEnabled` block of `buildMailVerbs` (line 131), wrapped with
   `wrapWrite` under the identity `mail.add_attachment` and the audit operation
   `write` (FR-13). It becomes the tenth verb of that block.
2. Populate `Summary`, `Description`, `Examples`, and `SeeDocs`. The description
   states the parameters, the gating requirement, the annotation semantics, and that
   the file bytes are supplied as base64 in `content_bytes`. `SeeDocs` points at
   `concepts#mail-gating`.
3. Declare all four annotation hints explicitly per the matrix (FR-12), and declare
   the schema with `message_id`, `name`, and `content_bytes` required, `mime_type`
   and `account` optional, and **no** `output` parameter (FR-9, FR-11).

### Phase 5: Surface, documentation, and harness

**Affected components:** `internal/server/server.go`, `extension/manifest.json`,
`internal/config/inventory.go`, `site/src/generated/surface.json`,
`docs/concepts.md`, `docs/troubleshooting.md`, `internal/docs/catalog_test.go`,
`docs/prompts/mcp-tool-crud-test.md`, `internal/tools/dispatch_registry_test.go`,
`internal/server/server_test.go`, `internal/server/readonly_test.go`.

1. Extend the `mail` domain `Intro` in `internal/server/server.go` (line 174) to name
   `add_attachment` among the `MailManageEnabled`-gated writes (FR-17). The string is
   exhaustive today and must stay exhaustive.
2. Extend the `mail` tool description in `extension/manifest.json` with the verb,
   leaving the `tools` array at four entries (FR-18).
   `TestManifestDescribesEveryRegisteredVerb` fails until this lands.
3. Widen the `OUTLOOK_MCP_MAIL_MANAGE_ENABLED` and
   `OUTLOOK_MCP_MAX_ATTACHMENT_SIZE_BYTES` descriptions in
   `internal/config/inventory.go` (lines 91 and 92) per FR-25.
4. Run `make surface-manifest` and commit `site/src/generated/surface.json`
   (FR-19, FR-23). `make ci` fails on a stale manifest, so this step is not optional.
   Confirm the regenerated figures are mail 18 full and 5 default, totals 47 full and
   33 default, and that the two widened configuration descriptions appear in the
   manifest's `config` array.
5. Update the `MAIL_MANAGE_ENABLED` row of the mail gating table in
   `docs/concepts.md` (line 65) to name draft attachments, preserving every string
   `TestMailGatingRowNamesMessageManagement` asserts; extend that test to also assert
   `add_attachment`; add `add_attachment` to the symptom list of the existing
   `Mail management disabled` troubleshooting entry; and add the two new
   troubleshooting entries with stable anchors (FR-24).
6. Append the lifecycle step to `docs/prompts/mcp-tool-crud-test.md` as Step 41,
   attaching to a draft the harness creates and deletes, and change the skip
   instruction under the Step 31 heading to "Steps 31 through 35 and Steps 37
   through 41" (FR-20). Do not renumber an existing step: the historical rows in
   `docs/bench/crud-runs.csv` reference the current numbering.
7. Regenerate `verbInventoryGolden` in `internal/tools/dispatch_registry_test.go`
   from the failing test's own output and review the delta, which must be exactly one
   added line, `mail.add_attachment ro=false de=false id=false ow=true` (FR-21).

## Test Strategy

Handler tests follow the established mail write-verb pattern: an `httptest` server
returning canned Graph JSON behind a real SDK client built by `newTestGraphClient`,
the client injected with `auth.WithGraphClient`, the handler constructor called
directly, and assertions made as substring checks on the returned text and on
`result.IsError`. For the large path, the test server also serves the upload URL so
the chunk `PUT`s are observed without leaving the process.

### Tests to Add

| Test File | Test Name | Description | Inputs | Expected Output |
|-----------|-----------|-------------|--------|-----------------|
| `internal/tools/add_attachment_test.go` | `TestAddAttachment_SmallFileUsesDirectPost` | A sub-threshold file takes the single POST | `message_id`, `name`, small `content_bytes` | One POST to /attachments observed; no upload session created; confirmation names the direct path |
| `internal/tools/add_attachment_test.go` | `TestAddAttachment_ThresholdBoundaryTakesSessionPath` | The routing boundary is strictly below the threshold | Decoded payload of exactly 3000000 bytes, then one of 2999999 | The first creates an upload session; the second issues the direct POST |
| `internal/tools/add_attachment_test.go` | `TestAddAttachment_ConfirmationNamesAttachmentID` | The new attachment id from the response is surfaced | Canned POST response carrying an attachment id | Confirmation contains the attachment id, the name, and the size, read from the response |
| `internal/tools/add_attachment_test.go` | `TestAddAttachment_RejectsNonDraft` | The draft guard refuses received mail | Canned message with `isDraft` false | The not-a-draft refusal is returned; no attachment request is issued |
| `internal/tools/add_attachment_test.go` | `TestAddAttachment_RequiresNameAndBytes` | Required parameters are enforced before the call | `message_id` only, then missing `content_bytes` | Error naming the missing parameter; no request issued |
| `internal/tools/add_attachment_test.go` | `TestAddAttachment_RejectsInvalidBase64` | Malformed content bytes are refused before the call | `content_bytes` that is not valid base64 | Error naming `content_bytes`; no request issued |
| `internal/tools/add_attachment_test.go` | `TestAddAttachment_InvalidMessageIDRejectedBeforeCall` | Identifier validation precedes Graph | Malformed `message_id` | Error returned; no request issued |
| `internal/tools/add_attachment_test.go` | `TestAddAttachment_DefaultsMimeType` | An omitted MIME type defaults | Small file, no `mime_type` | The fileAttachment content type is `application/octet-stream` |
| `internal/tools/add_attachment_test.go` | `TestAddAttachment_OversizeRejectedBeforeUpload` | The upper cap is enforced before any transfer | Decoded size above the cap | Error naming the bound and how to raise it; no session created |
| `internal/tools/add_attachment_test.go` | `TestAddAttachment_NoOutputParameter` | The write verb declares no output tier | The registered verb schema | No `output` parameter is present |
| `internal/tools/attachment_upload_session_test.go` | `TestUploadSession_LargeFileChunks` | A file at or above the threshold takes the session path | `content_bytes` decoding to more than 3000000 bytes | A createUploadSession POST, then chunk PUTs whose sizes are 320 KiB multiples except the last, with correct Content-Range |
| `internal/tools/attachment_upload_session_test.go` | `TestUploadSession_AttachmentIDFromLocationHeader` | The large-path id comes from the completion response | Final chunk answered with a Location header naming the attachment | The confirmation reports that identifier; a final response carrying none is an error, not a success |
| `internal/tools/attachment_upload_session_test.go` | `TestUploadSession_TransferFailureIsNotPartialSuccess` | A failed chunk is a failure | A test upload URL that rejects a chunk | Error returned; the confirmation is not produced |
| `internal/tools/attachment_upload_session_test.go` | `TestUploadSession_ErrorRedactsUploadURL` | The token-bearing URL is never leaked | A chunk PUT that returns an error carrying the URL | The error text does not contain the upload URL |
| `internal/tools/attachment_confirmation_test.go` | `TestAttachmentConfirmationUsesResponseNotArguments` | The confirmation echoes the service | Canned response whose id differs from any argument | The confirmation reports the response id |
| `internal/tools/attachment_confirmation_test.go` | `TestAttachmentConfirmationNoSubjectPlaceholder` | A draft with no subject still reads cleanly | Empty subject | The confirmation renders `(No subject)`, matching the mail write confirmation shape |
| `internal/tools/update_draft_test.go` | `TestVerifyIsDraftReturnsSubject` | The widened guard supplies the subject in one GET | Canned draft carrying a subject | The guard returns the message with its subject populated; exactly one GET is issued |
| `internal/tools/add_attachment_test.go` | `TestAddAttachment_UsesMaxAttachmentSizeBytes` | The cap is the existing configured bound, not a new one | Handler built with a 1024-byte bound, payload above it | Error names the measured size, the bound, and `OUTLOOK_MCP_MAX_ATTACHMENT_SIZE_BYTES` |
| `internal/docs/catalog_test.go` | `TestMailGatingRowNamesDraftAttachments` | The embedded gating row names the new capability | The embedded `concepts` bundle | The `MAIL_MANAGE_ENABLED` row names `add_attachment` and still names draft management, received-message management, and the four CR-0078 verbs |
| `internal/tools/tool_annotations_test.go` | `TestAddAttachmentAnnotations` | The four hints match the matrix | The registry under `MailManageEnabled` | `add_attachment` is not read-only, not destructive, not idempotent, and open-world |
| `internal/server/mail_verbs_test.go` | `TestMimeTypeIsNotBodyContentType` | The MIME parameter is distinct from the body content type | The mail aggregate schema | `content_type` keeps its text/html enum; `mime_type` is a separate free-string parameter |
| `internal/server/server_test.go` | `TestRegisterTools_MailManage_RegistersAddAttachment` | The verb registers only when gated on | `MailManageEnabled` true | `add_attachment` present in the operation enum; tool count 4 |
| `internal/server/server_test.go` | `TestMailIntroNamesEveryGatedWriteVerb` | The domain introduction is exhaustive, and stays so | The mail `Intro` string and the registry under `MailManageEnabled` | Every `MailManageEnabled`-gated verb name appears in the `Intro`. Cases derived from the registry rather than listed, so a future gated verb added without an `Intro` edit fails the build instead of shipping an introduction that describes a smaller server than the one registered |
| `internal/server/readonly_test.go` | `TestReadOnlyBlocksAddAttachment` | Read-only mode blocks the verb | Read-only server, verb invoked | The read-only refusal naming `mail.add_attachment` |

### Tests to Modify

| Test File | Test Name | Current Behavior | New Behavior | Reason for Change |
|-----------|-----------|------------------|--------------|-------------------|
| `internal/tools/dispatch_registry_test.go` | `TestVerbInventoryUnchangedAfterUpgrade` | Golden holds the post-CR-0078 identities | Golden adds `mail.add_attachment ro=false de=false id=false ow=true` | The verb surface changes intentionally; the golden records that intent |
| `internal/server/server_test.go` | `TestRegisterTools_MailEnabled` | Asserts the `MailEnabled` verb set | Additionally asserts `add_attachment` is absent | The default-surface guarantee of FR-2 and FR-23 needs a negative assertion, not only a positive one |
| `internal/tools/tool_annotations_test.go` | `TestMailAnnotationsManageEnabled` | Asserts the folded mail annotations under `MailManageEnabled` | Unchanged expectations, re-asserted against the larger verb set | The fold must not shift; `delete_draft` and `create_draft` already force destructive and non-idempotent |
| `internal/tools/update_draft_test.go`, `internal/tools/delete_draft_test.go` | The existing draft-guard cases | Call `verifyIsDraft` through the old signature | Compile against the new two-value signature, assertions unchanged | The guard is widened, not re-specified; a changed assertion here would mean behaviour drifted |
| `docs/prompts/mcp-tool-crud-test.md` | Lifecycle prompt steps | Steps end at 40; the skip instruction reads "Steps 31 through 35 and Steps 37 through 40" | Step 41 exercises `add_attachment` on a harness-created draft; the skip instruction reads "Steps 31 through 35 and Steps 37 through 41" | The harness must exercise the verb it now has, and skip it coherently when the gate is off |

### Tests to Remove

Not applicable. No functionality is removed or superseded, so no existing test
becomes obsolete. The draft verbs keep their `isDraft` guard and their tests, and
`add_attachment` reuses that guard rather than replacing it.

### Existing Tests That Gate This Change Without Modification

These already derive their cases from the registry or the configuration, so they
grade several acceptance criteria without being touched. They are listed because a
criterion whose gate is invisible reads as ungraded.

| Test File | Test Name | Criterion it grades |
|-----------|-----------|---------------------|
| `internal/tools/verb_metadata_test.go` | `TestEveryVerbHasDescription`, `TestEveryVerbHasSummary`, `TestEveryVerbHasClassification`, `TestSeeDocsAnchorsResolve` | AC-9: registry metadata completeness and anchor resolution for the new verb |
| `internal/tools/verb_metadata_test.go` | `TestWriteVerbsDeclareNoOutputParameter` | AC-6: the write verb declares no output tier, derived across every domain |
| `internal/tools/description_quality_test.go` | `TestDescriptionLengthBounded`, `TestMailDescriptionMentionsGatedVerbs`, `TestEveryVerbStatesRequiredParameters` | AC-10 and AC-12: the composed mail description stays under 4 000 characters and states the new verb's required parameters |
| `internal/server/schema_size_test.go` | `TestColdStartSchemaSize_Reduction` | AC-12: the cold-start schema reduction stays at or above 60% |
| `internal/surface/manifest_test.go` | `TestCommittedManifestMatchesRecord` | AC-11: the committed surface manifest matches the registry |
| `internal/server/manifest_sync_test.go` | `TestManifestDescribesEveryRegisteredVerb` | AC-11: the `mail` entry of `extension/manifest.json` names every registered mail verb, and the `tools` array holds four entries. Derived from the registry, so it fails until the manifest edit lands |
| `internal/server/mail_verbs_test.go` | `TestSharedParametersNameTheirWriteVerbs` | AC-14: the three new parameters are declared by one write verb only, so no shared-sense description rewrite is owed |
| `internal/config/inventory_test.go` | `TestEveryEnvLiteralIsEnumerated`, `TestInventoryEntriesAreComplete` | FR-14: no second attachment size environment variable is introduced |
| `internal/auth/auth_test.go` | `TestScopes_MailManage`, `TestScopes_NoMailSend` | AC-13: the requested scope set is unchanged and `Mail.Send` stays unrequested |

## Acceptance Criteria

### AC-1: The verb registers, and only behind the manage gate

```gherkin
Given a server configured with mail management enabled
When the registered tools are listed
Then the mail tool's operation enum contains add_attachment
  And exactly four top-level tools are registered
Given instead a server configured with mail read enabled but not mail management
When the registered tools are listed
Then add_attachment does not appear in the operation enum
```

### AC-2: A small file is attached with a single direct POST

```gherkin
Given a draft message and a file whose decoded size is strictly below three million bytes
When add_attachment is called with the message id, a name, and the base64 bytes
Then a single POST to the message's attachments collection is issued carrying a file attachment
  And no upload session is created
  And the confirmation names the draft subject, the message id, the attachment name, the size, the new attachment id, and the direct path
```

### AC-3: A large file is attached through a chunked upload session

```gherkin
Given a draft message and a file whose decoded size is at or above three million bytes
When add_attachment is called
Then an upload session is created for the attachment
  And the bytes are transferred by PUT to the session's upload URL in chunks that are multiples of 320 kibibytes except the last, each with a correct Content-Range
  And the attachment id is read from the completion response the service returns on the final chunk
  And the confirmation names the new attachment id and the session path
Given instead a file whose decoded size is exactly three million bytes
When add_attachment is called
Then it takes the upload session path, not the direct POST
```

### AC-4: The attachment id comes from the service, not the request

```gherkin
Given a Graph response whose attachment id was assigned by the service
When add_attachment formats its confirmation
Then the confirmation reports the id from the response
  And it does not echo a request argument as the id
```

### AC-5: A non-draft message is refused

```gherkin
Given a message whose draft state is false
When add_attachment is called on it
Then the not-a-draft refusal used by the draft verbs is returned
  And no attachment request is sent to Microsoft Graph
```

### AC-6: Required and well-formed inputs are enforced before any call

```gherkin
Given add_attachment called without a name, or without content bytes, or with content bytes that are not valid base64, or with a malformed message id
When the call is made
Then it is rejected before any request is sent
  And the error names the parameter to supply or correct
Given the registered verb schema
When it is inspected
Then it declares no output parameter
```

### AC-7: An oversize attachment is refused before any upload

```gherkin
Given a file whose decoded size exceeds the configured maximum attachment size
When add_attachment is called
Then the call is rejected before any session is created or any byte is transferred
  And the error names the measured size, the bound, and the OUTLOOK_MCP_MAX_ATTACHMENT_SIZE_BYTES variable that raises it
Given the implemented change
When the configuration inventory is enumerated
Then no second attachment size environment variable has been introduced
```

### AC-8: A transfer failure is a failure, and never leaks the upload URL

```gherkin
Given an upload session whose chunk transfer fails or whose session expires
When add_attachment reports the outcome
Then it returns an error rather than a success confirmation
  And neither the tool result nor the log record contains the pre-authenticated upload URL
```

### AC-9: Registry metadata is complete for the new verb

```gherkin
Given the add_attachment verb
When its registry entry is inspected
Then it carries a non-empty summary of at most eighty characters
  And a non-empty description stating its parameters, its gating requirement, and its annotation semantics
  And at least one example
  And at least one documentation reference resolving to an existing heading in the embedded bundle
```

### AC-10: The confirmation and the description name the base64 contract

```gherkin
Given the add_attachment description and a successful call
When each is read
Then the description states that the file bytes are supplied as base64 in content_bytes
  And the confirmation names the attachment name, the size in bytes, and which transfer path was used
```

### AC-11: Every generated and hand-written description of the surface names the new verb

```gherkin
Given the implemented change
When the mail domain introduction is read
Then it names add_attachment among the MailManageEnabled-gated write verbs
  And it names every other verb registered behind that gate
Given the committed verb inventory golden
When it is compared against the live registry
Then it differs by exactly one added line, mail.add_attachment ro=false de=false id=false ow=true
Given the implemented change
When make ci is run
Then the surface drift check passes without modifying the working tree
  And the mail domain's full verb count is eighteen while its default count is five
  And the totals are forty-seven full and thirty-three default
  And every verb registered for the mail domain appears in the mail entry of the extension manifest
  And the manifest tools array still holds exactly four entries
  And the manifest's configuration array carries the widened mail management and maximum attachment size descriptions
```

### AC-12: The measured surface gates keep their margins

```gherkin
Given the implemented change
When the composed mail description and the cold-start schema are measured
Then the description is below four thousand characters
  And the cold-start schema reduction is at least sixty percent against the documented baseline
  And no third-party dependency has been added
```

### AC-13: The consent surface is unchanged

```gherkin
Given the implemented change
When the requested OAuth scope set is computed for every supported configuration
Then it is identical to the set requested before this change
  And Mail.Send is not requested in any configuration
```

### AC-14: The MIME parameter is distinct from the body content type

```gherkin
Given the mail aggregate tool schema
When its parameters are read
Then content_type keeps its text and html enum for the draft body
  And mime_type is published as a separate parameter for the attachment content type
```

### AC-15: The annotation hints are declared and match the matrix

```gherkin
Given the mail registry under mail management enabled
When add_attachment's four annotation hints are read
Then it is not read-only, is not destructive, is not idempotent, and is open-world
  And the folded mail tool annotations are unchanged from their values before this change
```

### AC-16: Read-only mode blocks the verb, and the identity is consistent

```gherkin
Given a server started in read-only mode with mail management enabled
When add_attachment is invoked
Then it returns the read-only refusal naming its mail dot verb identity
  And that same identity is the one recorded in the audit record and the telemetry attributes
```

### AC-17: The implementation follows the project's file and helper conventions

```gherkin
Given the implemented change
When a Graph SDK call from the verb times out or returns an error
Then the timeout message names the configured timeout in seconds
  And the error text is redacted by the shared helper and carries a fix instruction
Given the same change
When the source tree is reviewed
Then the handler and the chunked-transfer helper live in separate files under the tools package, each named for its concern
  And the inline threshold, the chunk size, and the attachment name bound are each a named constant rather than an inline literal
```

### AC-18: The draft guard is widened once, and its existing callers are unchanged in behaviour

```gherkin
Given the widened draft guard
When it verifies a draft
Then it issues exactly one GET selecting the identifier, the draft state, and the subject
  And it returns the fetched message so the confirmation can name the draft without a second GET
Given update_draft and delete_draft
When each is called on a message whose draft state is false
Then each still returns the same not-a-draft refusal it returned before this change
```

### AC-19: The embedded documentation describes the verb it now has

```gherkin
Given the embedded documentation bundle a running server serves
When the mail gating row of the concepts document is read
Then it names draft attachments alongside draft management and received-message management
  And it still names move_message, set_flag, set_categories, and mark_read
Given the troubleshooting document
When it is read
Then the mail management disabled entry's symptom list names add_attachment
  And an entry with a stable anchor covers attaching to a non-draft message
  And an entry with a stable anchor covers an upload-session transfer failure or expiry
Given the lifecycle harness prompt
When its mail management section is read
Then Step 41 exercises add_attachment against a draft the harness creates and deletes
  And no existing step has been renumbered
  And the skip instruction reads Steps 31 through 35 and Steps 37 through 41
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
- [x] All new tests pass, including under the race detector
- [x] Test coverage meets project requirements for changed code

### Documentation

- [x] Every new file carries a package-consistent doc comment and a single index annotation
- [x] The new verb's parameters and annotation semantics are documented in the registry, not in markdown
- [x] Both troubleshooting entries carry stable anchors
- [x] No governance identifier appears in source, test names, or user-facing documentation

### Code Review

- [ ] Changes submitted via pull request
- [x] PR title follows Conventional Commits format
- [ ] Code review completed and approved
- [ ] Changes squash-merged to maintain linear history

### Verification Commands

```bash
# Full pipeline, including the surface drift check and the extension manifest validation
make ci

# The new handler, the transfer helper, the widened draft guard, and the registry and
# documentation checks, under the race detector
go test -race ./internal/tools/ ./internal/server/ ./internal/validate/ \
  ./internal/docs/ ./internal/config/ ./internal/surface/ \
  -run 'AddAttachment|UploadSession|AttachmentConfirmation|MimeType|VerbInventory|VerifyIsDraft|MailGatingRow|ManifestDescribes|SharedParameters|EnvLiteral|CommittedManifest'

# Regenerate the published surface manifest after the registry changes
make surface-manifest

# Rebuild the binary the harness drives, then run the lifecycle harness
go build -ldflags="-X main.commit=$(git rev-parse --short HEAD) -X main.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -o ./outlook-local-mcp ./cmd/outlook-local-mcp
make crud-test
```

The unit suite issues no Graph call and serves the upload URL from an in-process
`httptest` server, so it cannot confirm live chunked transfer. Acceptance additionally
requires a user scenario driving the built server against a live mailbox: create a draft,
attach a small file through the direct path and a larger file through the session path,
then confirm through `list_attachments` that both attachment ids resolve on the draft.
Persist the scenario under `.agents/scenarios/` per the project's scenario rule, and read
the harness report's own server version line against `git rev-parse --short HEAD` before
trusting any row of it.

## Risks and Mitigation

### Risk 1: A partial chunked upload is reported as a success

**Likelihood:** medium
**Impact:** high
**Mitigation:** This is the failure the large path is specified to prevent. A transfer
that fails partway leaves an upload session the service discards on expiry, so a success
confirmation would name an attachment id that never resolves, which is exactly the
handle-to-nothing failure the move contract in CR-0078 was written to avoid. NFR-5 forbids
reporting a partial transfer as attached; the handler reads the created attachment id only
from a completed final chunk, and a transfer that does not complete returns an actionable,
redacted error rather than a confirmation. AC-8 and
`TestUploadSession_TransferFailureIsNotPartialSuccess` grade it.

### Risk 2: The token-bearing upload URL leaks into an error or a log

**Likelihood:** high without the mitigation
**Impact:** high
**Mitigation:** The `createUploadSession` response returns a pre-authenticated upload URL
that carries an access token, and the chunk `PUT`s are raw HTTP outside the SDK
request-builder chain, so the standard Graph error redaction does not cover them by
default. FR-15 requires every error arising from a chunk `PUT` to be redacted so the
upload URL never appears in the tool result or the log record, on either channel. The
transfer helper is isolated in its own file precisely so this redaction has a single
enforced site rather than being re-implemented per call. AC-8 and
`TestUploadSession_ErrorRedactsUploadURL` grade it.

### Risk 3: The full base64 payload sits in memory and exhausts it

**Likelihood:** medium
**Impact:** medium
**Mitigation:** The attachment arrives as base64 in one tool argument and is decoded in
full, so the whole file, and transiently its base64 encoding, is resident in memory; the
chunked path bounds the Graph transfer, not the footprint. NFR-6 makes the upper size
bound of FR-14 the control that keeps the footprint bounded, and fixes it at the existing
`MaxAttachmentSizeBytes` default of 10 MB rather than anywhere near the service's 150 MB
ceiling. That bound is not a new invention: `get_attachment` buffers the whole attachment
too, so the same variable already governs a fully resident payload on the read side. The
oversize check runs before any session is created or any byte is transferred, so an
over-cap request is refused rather than buffered. AC-7,
`TestAddAttachment_OversizeRejectedBeforeUpload`, and
`TestAddAttachment_UsesMaxAttachmentSizeBytes` grade it.

### Risk 4: The attachment MIME type is misdescribed by the flattened schema

**Likelihood:** high without the mitigation
**Impact:** medium
**Mitigation:** The mail domain already declares `content_type` as a body content type
restricted by an enum to `text` and `html`, and `aggregateSchemaOptions` merges duplicate
parameter names with first-occurrence-wins, so reusing that name for the attachment MIME
type would publish the `text | html` enum on it and tell an LLM an attachment must be
`text` or `html`. The verb names its parameter `mime_type` instead, one concept to one
name, following the `destination_folder_id` precedent CR-0078 set. AC-14 and
`TestMimeTypeIsNotBodyContentType` grade it.

### Risk 5: A caller reads `add_attachment` as idempotent and retries it

**Likelihood:** medium
**Impact:** low
**Mitigation:** A second call with identical arguments mints a second identical
attachment with a distinct id rather than leaving the same end state, so the verb is
declared `idempotentHint: false`. The classification, the description, and the user-impact
note all disclose that attaching the same file twice produces two attachments, so a client
that gates retries on the hint does not silently duplicate. AC-15 grades the hint.

### Risk 6: The lifecycle prompt drifts from the new parameter names

**Likelihood:** medium
**Impact:** low
**Mitigation:** This is the open class the project already documents: the harness prompt
names parameters in prose and nothing binds the two together, and instance-level
correction has not closed it. This change does not claim to close it. It adds one step
written against the registry as implemented, and records that a clean harness run
afterwards is evidence about that step and not about the class.

## Dependencies

* No new third-party dependency. The small path uses surfaces already present in the
  pinned `github.com/microsoftgraph/msgraph-sdk-go v1.100.0`, confirmed by reading the
  module cache rather than the vendor's documentation, and the large path's chunk `PUT`s
  use the standard library HTTP client rather than an upload library (NFR-3).
* No new OAuth scope. `Mail.ReadWrite` is already requested whenever `MailManageEnabled`
  is set, and it covers both the direct `POST` and the `createUploadSession` upload.
  `Mail.Send` stays unrequested.
* Depends on the mail write surface CR-0078 left behind: the `MailManageEnabled` gate,
  the write middleware chain, the `verifyIsDraft` draft guard, `FormatMailWriteConfirmation`,
  and the two derived checks `TestSharedParametersNameTheirWriteVerbs` and
  `TestManifestDescribesEveryRegisteredVerb`. **CR-0078 is complete** at `ee23a77` on
  branch `docs/cr-implementation-set-0079-0083`, and this document is reconciled against
  that tree.
* Depends on the verb registry and conservative annotation fold of CR-0060 and CR-0068,
  and the generated surface manifest and its drift check of CR-0073, all completed.
* Picks up two feature-gap-matrix rows assigned to a CR: "add attachment" and "large
  attachment upload session", the second gated on the first.

## Estimated Effort

| Phase | Effort |
|---|---|
| Phase 1, validation helper, size constants, confirmation formatter, widened draft guard | 3 hours |
| Phase 2, the small path handler and its tests | 3 hours |
| Phase 3, the chunked upload session, redaction, and its tests | 4 to 5 hours |
| Phase 4, registry entry, annotations, schema | 2 hours |
| Phase 5, surface, documentation, harness, golden regeneration | 3 hours |
| Total | 15 to 17 hours |

The effort concentrates in Phase 3, the one architectural novelty in this change: a
raw-HTTP chunked `PUT` outside the SDK request-builder chain, with the redaction and the
no-partial-success contract both carried in that phase.

## Decision Outcome

Chosen approach: "one `add_attachment` verb with two internal transfer paths selected by
the decoded byte size, returning an unconditional confirmation read from the response",
because the feature-gap matrix marks the large-upload row "moot while attach is absent",
which makes it the same capability at a different size rather than a second verb.

The alternative that looks tidier, two verbs split at the size boundary, was rejected on
that ground rather than on taste. It would force the LLM to choose a byte threshold it
cannot see, which is precisely the decision the verb should make internally from the
decoded size.

The large path is the part specified most carefully. It is raw HTTP outside the SDK, it
carries a token-bearing URL that must never leak, and a transfer that fails partway must
be a failure rather than a success naming an id that does not resolve. Those three
properties, chunk sizing, redaction, and no partial success, are the contract this change
request pins.

## Open Questions

Each item below records an assumption made so the change request could be written without
blocking. Each is the smallest reasonable choice, and each is stated here so a reviewer can
overturn it rather than discover it in the diff. Items 1, 2, and 4 were **settled during
review**; the decisions and their grounds are recorded in
`docs/backlog/cr-0078-0083.md` under `## CR-0079`.

1. **A distinct upload bound versus reuse of `MaxAttachmentSizeBytes`. Settled: reuse.**
   The case for a distinct `OUTLOOK_MCP_MAX_ATTACHMENT_UPLOAD_BYTES` rested on a memory
   asymmetry, that the download side streams while the upload side buffers. Reading
   `internal/tools/get_attachment.go` shows the download side does not stream: it
   deserialises the whole attachment through the SDK and returns it base64-encoded in the
   tool result. With the asymmetry gone, the existing `MaxAttachmentSizeBytes` is the
   bound in both directions (FR-14, NFR-6), and no second environment variable is added.
2. **The upload cap value. Settled: the existing default of `10485760` (10 MB).**
   It follows from item 1. It is well below the service's 150 MB ceiling, which is what
   NFR-6 requires, and a caller who needs more raises the one variable that already
   controls attachment size in both directions.
3. **`mime_type` defaulting to `application/octet-stream`.** Assumed when the caller omits
   it (FR-7), which is the safe generic type Graph accepts for arbitrary bytes. The
   alternative, requiring `mime_type`, was rejected as friction for the common case where
   the caller does not know or care about the precise type.
4. **The inline threshold. Settled: strictly below `3 * 1000 * 1000` bytes.**
   The draft assumed 3 MiB (`3145728`) inclusive. Graph states the single-`POST` limit in
   decimal megabytes, and the direct `POST` carries the bytes base64-encoded in a JSON
   body, expanding them by four thirds against Graph's request body ceiling, so `3145728`
   inclusive is over the limit on both counts. Misrouting downward fails the request while
   misrouting upward costs one extra round trip, so the boundary resolves toward the
   session path. It remains an internal constant, so a change to the service limit is a
   one-line edit rather than a contract change.
5. **Audit operation string `write`.** Assumed, matching the draft write verbs. The verb
   only adds, so no `attach` audit category was introduced for one verb.
6. **Target version 0.11.0.** Assumed from CR-0078's target of 0.10.0, since this CR
   follows it in sequence. The latest release tag is `v0.6.0`, so the target is a
   placeholder to be reconciled at release-planning time rather than a commitment. This
   matches how CR-0078 carries its own target.

## Related Items

* Follows CR-0078, which is complete, and is written against the mail write surface it
  left behind: the same `MailManageEnabled` gate, write middleware chain, and
  `verifyIsDraft` draft guard, which this change widens rather than duplicates.
* Closes two feature-gap-matrix rows assigned to a CR: "add attachment" and "large
  attachment upload session", the second gated on the first.
* Builds on the verb registry and dispatch model of CR-0060, the registry-owned
  documentation rule of CR-0065, and the computed annotation fold of CR-0068.
* Uses the generated surface manifest and its drift check from CR-0073.
* Follows the annotation matrix presentation established by CR-0052, and the
  distinct-parameter-name precedent (`destination_folder_id`, here `mime_type`) set by
  CR-0078.
* Deliberately deferred to their own change requests: removing an attachment
  (`DELETE /me/messages/{id}/attachments/{id}`, the matrix's "delete attachment" row),
  attaching to a received message, item and reference attachments, inline attachments,
  resuming an interrupted upload session, and the calendar attachment rows.

## More Information

The capability this change closes is one the read side already reaches. `list_attachments`
enumerates attachment metadata via `GET /me/messages/{id}/attachments` and `get_attachment`
downloads one attachment's bytes bounded by `MaxAttachmentSizeBytes`, so the server can
list and read attachments on a message it has, and cannot create one. A draft the model
composed to carry a document has, until this change, no way to be given that document
except by hand in Outlook.

The large path is the part worth reading twice. Microsoft Graph does not transfer the file
through the SDK for a file above its inline limit: `createUploadSession` returns a
pre-authenticated upload URL, and the bytes are written by `PUT` requests to that URL
outside the request-builder chain. The pinned SDK's own signatures confirm this, where
`Attachments().CreateUploadSession().Post` returns a `models.UploadSessionable` exposing
`GetUploadUrl()` and `GetNextExpectedRanges()` rather than a created attachment. That URL
carries an access token, which is why its redaction in every error is a requirement rather
than a nicety, and why the transfer is isolated in its own file so the redaction has one
enforced site.

<!-- review-summary -->
## Review summary

Reviewed against the working tree at `ee23a77` on branch
`docs/cr-implementation-set-0079-0083`, after CR-0078 landed. The CR was authored at
`78a3bb3`, before that work, so the drift check ran first and its findings are counted
separately below.

**Findings: 28. Drift 10, contradiction 3, ambiguity 7, coverage 4, scope 4. Fixes
applied: 28. Unresolved: 0.**

### Drift (10)

1. `internal/server/mail_verbs.go` line 130 is now line 131, and the `MailManageEnabled`
   block holds nine verbs rather than the five draft verbs the CR described. Current State
   and Phase 4 updated; the nine are named.
2. `internal/server/mail_verbs.go:452` no longer carries the `content_type` declaration.
   It is now `:458` on `create_draft` and `:570` on `update_draft`. Both cited.
3. The `MAIL_MANAGE_ENABLED` gating row in `docs/concepts.md` no longer reads "including
   draft management"; CR-0078 widened it to name draft management and received-message
   management with the verbs of each. The Affected Components entry described a row that
   no longer exists and has been rewritten, including the strings
   `TestMailGatingRowNamesMessageManagement` now requires it to preserve.
4. `FormatMailWriteConfirmation` (`internal/tools/mail_write_confirmation.go:39`) landed
   with CR-0078 and is now the mail write confirmation shape. The CR justified its new
   formatter only against `FormatDraftConfirmation`. The justification is rewritten against
   both, and the new formatter is required to follow the established line order.
5. `TestManifestDescribesEveryRegisteredVerb` (`internal/server/manifest_sync_test.go:84`)
   did not exist when the CR was authored. It derives its cases from the registry and fails
   until FR-18 lands, so it is recorded as the gate on that requirement.
6. `TestSharedParametersNameTheirWriteVerbs` (`internal/server/mail_verbs_test.go:161`)
   likewise post-dates the CR. The claim that it "continues to pass unchanged" is correct
   and is now substantiated: the check selects only names declared by both a read verb and
   a write verb, and the domain's 28 declared parameter names were enumerated to confirm
   `name`, `content_bytes`, and `mime_type` collide with none of them.
7. The `Mail management disabled` entry in `docs/troubleshooting.md` now enumerates nine
   gated write verbs and reads as exhaustive. Widening its symptom list was not in scope
   and is now required by FR-24.
8. `docs/prompts/mcp-tool-crud-test.md` now runs to Step 40 and its skip instruction
   already covers two ranges. FR-20 was written against the pre-CR-0078 single range and is
   rewritten with the exact step number and the exact instruction text.
9. `internal/config/inventory.go` was named only as `internal/config`. It is the source of
   the surface manifest's `config` array, and its `MAIL_MANAGE_ENABLED` description was
   widened by CR-0078. Both descriptions it owns are now named with their line numbers and
   required edits (FR-25).
10. Frontmatter `source-branch: main` and `source-commit: 78a3bb3` predate the branch this
    CR is implemented on. Updated to the reviewed baseline.

Verified unchanged and left as written: `internal/tools/update_draft.go:218`,
`internal/tools/draft_helpers.go:84`, `internal/tools/get_attachment.go:128`,
`docs/concepts.md:112`, all fourteen `msgraph-sdk-go@v1.100.0` module-cache line citations,
the starting surface figures (mail 17 full and 5 default, totals 46 and 33), and all
sixteen cited test file paths and function names.

### Contradictions resolved (3)

1. **The draft guard cannot supply the subject.** Phase 2 required the confirmation to name
   the draft subject from the `verifyIsDraft` fetch, but that helper returns only
   `*mcp.CallToolResult` and discards the message, and neither of its two call sites was in
   Affected Components. Resolved in favour of widening the helper once rather than
   duplicating the guard: FR-5 now requires the `$select` to add `subject`, the helper to
   return the fetched message, and both call sites to be updated with no behaviour change.
   `internal/tools/update_draft.go` and `internal/tools/delete_draft.go` are added to
   Affected Components and to Phase 1, and AC-18 grades it.
2. **The routing boundary sent a boundary file into a request Graph rejects.** FR-6 routed
   a file at exactly 3 MiB into the direct `POST`. Graph states that limit in decimal
   megabytes, and the direct `POST` carries the bytes base64-encoded in a JSON body,
   expanding them by four thirds. Resolved toward the path that cannot fail on size:
   strictly below `3 * 1000 * 1000` bytes takes the `POST`, at or above it takes the
   session. FR-6, AC-2, AC-3, the threshold section, the proposed-state diagram, and the
   Change Drivers bullet all follow, and a boundary test pins it.
3. **The upload bound was justified on a property the code does not have.** NFR-6 and Open
   Question 1 rested on `get_attachment` streaming while the upload buffers. It does not
   stream: it deserialises the whole attachment through the SDK and returns it
   base64-encoded in the tool result. Resolved by reusing `MaxAttachmentSizeBytes` rather
   than adding a second variable.

### Ambiguity fixed (7)

1. NFR-2's "leaves the margins substantially intact" replaced by the measured post-CR-0078
   values (2 737 of 4 000 characters, 75% against a 60% floor) and a statement that the
   gates decide, not the document.
2. NFR-6's "set with that in mind rather than at the service's 150 MB ceiling" replaced by
   the named default `10485760`.
3. FR-20's "widened to cover the new step" replaced by the exact step number (41) and the
   exact instruction text.
4. Phase 3's "read the created attachment id from the final chunk's response" now names the
   channel: the `Location` header, with the body as a fallback only where the service
   returns one, and a final response carrying no identifier is a failure.
5. FR-10 now states the id source per path and requires `(No subject)` for a missing
   subject, matching the established formatter.
6. NFR-8 now covers the chunk size alongside the name bound and the inline threshold, and
   distinguishes the constants from the configured bound.
7. FR-14's "names the bound and how to raise it" now names the measured size, the bound,
   and `OUTLOOK_MCP_MAX_ATTACHMENT_SIZE_BYTES`.

### Coverage gaps closed (4)

1. FR-17 (the domain introduction) had neither an acceptance criterion nor a test. AC-11
   now grades it, and `TestMailIntroNamesEveryGatedWriteVerb` derives its cases from the
   registry so a future gated verb cannot ship an introduction that omits it.
2. FR-20 (the harness prompt) had no acceptance criterion. AC-19 now grades it.
3. FR-21 (the verb inventory golden) had no acceptance criterion. AC-11 now grades it.
4. `docs/concepts.md` and `docs/troubleshooting.md` were in Affected Components with no
   requirement and no criterion. FR-24 and AC-19 close that, and
   `TestMailGatingRowNamesDraftAttachments` gates the concepts edit against the embedded
   bundle, following the precedent CR-0078 set.

### Scope consistency (4)

1. No phase listed its affected components, which is the exact defect the CR-0078 review
   recorded against its own Phase 5. Every phase now carries an explicit list.
2. `internal/tools/update_draft.go` and `internal/tools/delete_draft.go` added.
3. `internal/docs/catalog_test.go` added, as the gate on the concepts row edit.
4. `internal/config` narrowed to `internal/config/inventory.go` with its two line-numbered
   descriptions, and `internal/config/config.go` explicitly stated as needing no change.

### Open questions settled

Items 1, 2, and 4 were settled during review and are recorded with their grounds under
`## CR-0079` in `docs/backlog/cr-0078-0083.md`. Items 3, 5, and 6 were already recorded
assumptions rather than open choices and are left as written.

### Requiring human decision

None.
<!-- /review-summary -->
