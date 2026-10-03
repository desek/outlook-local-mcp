# Graph specification control sweep

Contributor-facing. Not embedded in the binary and not published to the site.

Commit under review: `bba461c`. Sweep date: 2026-10-03.

## Scope and method

The sweep compares every Graph call the server issues against the Microsoft Graph v1.0 contract. No live tenant was available, so the specifications are the authority and no finding below was observed against the real service.

Sources, ranked:

* Tier 1, code: the pinned SDK `github.com/microsoftgraph/msgraph-sdk-go` v1.100.0 (with `kiota-abstractions-go` v1.9.4 and `kiota-http-go` v1.5.6) read from the module cache, and the handlers under `internal/` at `bba461c`.
* Tier 3, documentation: Microsoft Graph v1.0 reference and concept pages on `learn.microsoft.com`, and their source in `microsoftgraph/microsoft-graph-docs-contrib`. Each page's `ms.date` or last-updated date was read before it was trusted.

Verification was adversarial. Each candidate finding was handed to three independent refuters, each with one lens:

1. Spec lens: does the cited document say what the finding claims?
2. Code lens: does the code and the pinned SDK do what the finding claims?
3. Impact lens: would the real service reject or mis-serve the call, or is the gap harmless?

A finding is confirmed when it survives the vote; a finding that all three lenses refute is listed under Refuted. A single impact-lens refutation is recorded with the finding and lowers the confidence, not the status. Severity: `blocker` means the verb cannot deliver its contract on the real service; `defect` means a documented contract is violated or a stated promise is false; `risk` means the behaviour rests on undocumented service behaviour.

## Confirmed findings

Each entry gives the claim, the specification evidence, the code evidence, the fix, and the refuter votes.

### Blockers

#### B1. `internal/tools/attachment_upload_session.go:306` and `:312`, chunked upload reports a malformed attachment id

* Claim: `attachmentIDFromLocation` returns the literal OData key segment `Attachments('<id>')` instead of the attachment id, so the chunked-path confirmation and the audit attribute carry an identifier that does not resolve. The same function serves the message path and the event path.
* Spec evidence: `outlook-large-attachments`, Step 3, final PUT response: `201 Created`, `Location: https://outlook.office.com/api/v2.0/Users('...')/Messages('AAMk...')/Attachments('AAMkADI5MAAIT3drCAAABEgAQANAqbAe7qaROhYdTnUQwXm0=')`, `Content-Length: 0`. The event variant is `.../Events('AAMk...')/Attachments('AAMkADU5CCmSAAANZAlYPeyQByv7Y=')`. The doc says the attachment id is the value inside the parentheses.
* Code evidence: the function splits `parsed.Path` on `/`, takes the last segment and only percent-decodes it. A probe with the documented URL returned `Attachments('AAMkADI5MAAIT3drCAAABEgAQANAqbAe7qaROhYdTnUQwXm0=')`. The JSON body fallback in `completedAttachmentID` cannot help because the documented 201 has no body. The test fixture at `attachment_upload_session_test.go:97-98` emits `/me/messages/draft-1/attachments/<id>`, a shape the doc does not show, so the passing test proves nothing about the real service. The value flows into `FormatAttachmentConfirmation` and `FormatEventAttachmentConfirmation` as `Attachment ID` and into the `attachment_id` audit attribute.
* Fix: if the last segment matches `^Attachments\('(.+)'\)$` (case-insensitive) return the capture group, then percent-decode; keep the plain `/attachments/<id>` form as a fallback. Change the fixture `Location` to the documented `.../Messages('draft-1')/Attachments('<id>')` and `.../Events('evt-1')/Attachments('<id>')` forms, and add a case for a key that contains `=` and `%3D`.
* Votes: 3 of 3 confirm (both reports, six votes in total).

#### B2. `internal/tools/teams_list_chats.go:75`, `list_chats` never asks for the fields its summary shows

* Claim: `GET /me/chats` is issued with no `$expand`, so `lastMessagePreview` and `members` are never returned; the summary tier's headline field `lastMessagePreview` is always empty on the real service and the raw tier's `members` is always `[]`.
* Spec evidence: `chat-list` v1.0: "$expand: Currently supports members and lastMessagePreview properties." Example 1 (no `$expand`) returns neither field; Examples 2 and 4 return them only under `$expand=members` and `$expand=lastMessagePreview`. The `chat` resource lists both under Relationships, not Properties. Known limit: `$expand=members` caps the response at 25 chats regardless of `$top`.
* Code evidence: `client.Me().Chats().Get(timeoutCtx, nil)` passes a nil request configuration. `teams_serialize.go` reads `chat.GetLastMessagePreview()` and `chat.GetMembers()`. The fixture at `teams_list_chats_test.go:28-34` inlines both fields, so the unit tests cannot observe the missing expand. The SDK exposes `Expand []string` on `ItemChatsRequestBuilderGetQueryParameters` (`users/item_chats_request_builder.go:21`).
* Fix: pass a request configuration with `Expand: []string{"lastMessagePreview"}` on the default path and add `members` only for `output=raw` because of the 25-item cap. Add a test that asserts the recorded request URL carries `$expand`.
* Votes: 3 of 3 confirm.

#### B3. `internal/tools/teams_list_chat_message_replies.go:91`, a verb on a path the contract does not define

* Claim: the verb reads `GET /me/chats/{chat-id}/messages/{message-id}/replies`, a path the v1.0 contract documents only for channel messages. Chat threads are flat, so on the real service the call fails or returns an empty collection, and the verb can never deliver what its header docstring promises ("the answers to a question are only reachable through this verb").
* Spec evidence: `chatmessage-list-replies` v1.0 is titled "List channel message replies", lists only `GET /teams/{team-id}/channels/{channel-id}/messages/{message-id}/replies`, and its permissions table names only `ChannelMessage.Read.All`. The `chatMessage` resource says `replyToId` "(Only applies to chat messages in channels, not chats.)", the `replies` relationship "Supports $expand for channel messages", and the Methods table has no replies method under "Chat messages".
* Code evidence: `client.Me().Chats().ByChatId(chatID).Messages().ByChatMessageId(messageID).Replies().Get(timeoutCtx, nil)`. The SDK builder `users/item_chats_item_messages_item_replies_request_builder.go` exists because it is generated from the OpenAPI metadata, which is not evidence that the service implements it. The verb is advertised in `extension/manifest.json`, `docs/concepts.md`, `docs/prompts/mcp-tool-crud-test.md`, and `site/src/generated/surface.json`.
* Fix: remove the verb (registry entry, manifest entry, help text, harness steps, surface manifest) or re-scope it to channel replies, which `list_channel_message_replies` already covers. `list_chat_messages` already returns every message in a chat. Update the governing change request and the verb inventory.
* Votes: 3 of 3 confirm.

#### B4. `internal/auth/auth.go:121`, `find_meeting_times` runs without a scope the contract lists

* Claim: `POST /me/findMeetingTimes` is documented for delegated work accounts under `Calendars.Read.Shared` or `Calendars.ReadWrite.Shared` only, but `Scopes()` requests `Calendars.ReadWrite` and no `.Shared` scope under any flag, so the spec predicts `403 ErrorAccessDenied`. A companion risk entry (R6) records the same gap at the constant.
* Spec evidence: `user-findmeetingtimes` v1.0 (updated 2026-06-19) Permissions table: Delegated (work or school account): least privileged `Calendars.Read.Shared`, higher privileged `Calendars.ReadWrite.Shared`. Personal account and Application: Not supported. Neither `Calendars.Read` nor `Calendars.ReadWrite` appears.
* Code evidence: `auth.go:22` `const calendarScope = "Calendars.ReadWrite"`; `auth.go:121` `scopes := []string{calendarScope}` with mail, contacts, people and Teams scopes appended only under their flags. `grep -rn Shared internal/` matches no scope. `find_meeting_times.go:199` issues `client.Me().FindMeetingTimes().Post(...)`. `docs/concepts.md:133` lists only `Calendars.ReadWrite` for the calendar domain.
* Fix: add a `calendarSharedScope` constant (`Calendars.ReadWrite.Shared`, or `Calendars.Read.Shared` for a narrower consent) and append it in `Scopes()` whenever `find_meeting_times` is registered. Document it in the OAuth scopes summary and add a `Scopes()` test. Confirm against a live tenant at the first opportunity; the impact-lens refuter of R6 found that the permissions include listed `Calendars.Read` and `Calendars.ReadWrite` for this endpoint in an October 2023 build, and that the switch to `.Shared` was a catalog regeneration, so the call may still succeed.
* Votes: 3 of 3 confirm at `auth.go:121`. The companion entry at `auth.go:22` (R6) drew 2 confirm, 1 impact-lens refute.

### Defects

#### D1. `internal/tools/add_attachment.go:114` and `internal/tools/add_event_attachment.go:120`, no 150 MB ceiling

* Claim: the only size cap is the operator-configurable `OUTLOOK_MCP_MAX_ATTACHMENT_SIZE_BYTES`; a raised limit sends a `createUploadSession` the service documents as out of range.
* Spec evidence: `attachment-createuploadsession`: "Use this approach to attach a file if the file size is between 3 MB and 150 MB." `outlook-large-attachments`: "you can attach files up to 150 MB". Tip: the default Exchange Online message size limit is 35 MB.
* Code evidence: `if maxSize > 0 && int64(size) > maxSize` is the only guard in both handlers. `config.go:345-352` parses the variable with no ceiling and treats values `<= 0` as unlimited. No constant for 150 MB exists under `internal/`.
* Fix: add a constant for the documented 150 MB ceiling and refuse above `min(maxSize, 150 MB)` before any Graph call, with an error that names the documented limit and the tenant message-size caveat.
* Votes: 2 confirm, 1 impact-lens refute on each file. The refuter notes the default is 10 MB, the service rejects the session creation before any chunk moves, and the tenant limit of 35 MB cannot be known client-side, so the fix only improves the error text on a misconfigured path.

#### D2. `internal/server/calendar_verbs.go:834`, `find_meeting_times` offered to personal accounts

* Claim: the verb is offered to every account, but the doc states the endpoint is not supported for personal Microsoft accounts; the project supports personal accounts and neither the Description nor the fix text names the case.
* Spec evidence: `user-findmeetingtimes` Permissions table: Delegated (personal Microsoft account): Not supported.
* Code evidence: `docs/quickstart.md:8` lists "A Microsoft account (personal, work, or school)" and line 255 documents `TENANT_ID=consumers`. `findMeetingTimesGraphFix` (`find_meeting_times.go:44`) tells the caller to check attendee mailboxes. `create_event` at `calendar_verbs.go:323` already says "(work/school accounts)" for Teams meetings, so the pattern exists.
* Fix: state in the verb Description and in `docs/troubleshooting.md` that `find_meeting_times` requires a work or school account, and extend `findMeetingTimesGraphFix` to name the personal-account case.
* Votes: 3 of 3 confirm.

#### D3. `internal/tools/find_meeting_times.go:113`, a zero duration serializes as `P`

* Claim: `PT0M`, `PT0S` or `P0D` passes `ParseISODuration` and is serialized as the malformed token `P`, an invalid `Edm.Duration`.
* Spec evidence: `user-findmeetingtimes`: `meetingDuration` is `Edm.Duration` in ISO 8601 format, for example `PT1H` or `PT2H30M`. `P` with no component is not a valid ISO 8601 duration.
* Code evidence: `kiota-abstractions-go` v1.9.4 `serialization/duration.go` `string()` writes `P` and then only components `> 0`; `hasTimePart()` is false when all time fields are zero. A probe printed `PT0M -> "P"`, `PT0S -> "P"`, `P0D -> "P"`, `PT30M -> "PT30M"`. The handler has no minimum check between line 113 and `body.SetMeetingDuration` at line 159.
* Fix: after parsing, convert with `ToDuration()` and refuse a value `<= 0` with an error that names `meeting_duration` and the `PT30M` example.
* Votes: 3 of 3 confirm.

#### D4. `internal/tools/get_schedule.go:151`, the `timezone` parameter promises what the request does not ask for

* Claim: the handler sends no `Prefer: outlook.timezone` header, so every returned `scheduleItem` start and end is UTC, while the parameter description says "IANA timezone name for the queried window and the returned times".
* Spec evidence: `calendar-getschedule` v1.0 request headers: "Prefer: outlook.timezone ... Use this to specify the time zone for start and end times in the response. If not specified, those time values are returned in UTC." `outlook-get-free-busy-schedule`: "By default, the start and end times of the returned schedule items are represented in UTC."
* Code evidence: `PostAsGetSchedulePostResponse(timeoutCtx, body, nil)`; no `Prefer` string in the file. `internal/server/calendar_verbs.go:931` carries the promise. The sibling `get_free_busy.go:206-213` adds the header for the same purpose. The raw tier carries each item's `timeZone` field, so raw output is self-labelled; the summary and text tiers are not.
* Fix: build an `ItemCalendarGetScheduleRequestBuilderPostRequestConfiguration` with `Headers` carrying `Prefer: outlook.timezone="<windowTimezone>"` (copy the `get_free_busy.go` pattern), pass it instead of nil, and add a test that asserts the recorder saw the header. Or correct the description to say the response times are UTC.
* Votes: 3 of 3 confirm.

#### D5. `internal/tools/get_schedule.go:269`, the 62-day window ceiling is not enforced

* Claim: `resolveScheduleWindow` validates format only; it never checks `end > start` or `end - start < 62 days`, so an oversize window reaches Graph and the handler returns an error that does not name the limit, against the file's own stated design.
* Spec evidence: `outlook-get-free-busy-schedule`, Limits and error conditions: "The time period to look up must be less than 62 days."
* Code evidence: lines 269-300 call only `validate.ValidateDatetime` on each bound. The same file enforces the adjacent limit from the same doc section (`maxScheduleMailboxes = 20`, lines 29-33) with the rationale that the caller is told the limit rather than shown an unnamed Graph error. No test in `get_schedule_test.go` covers a long or inverted window; `grep 62` over the file, its test and `docs/` returns nothing.
* Fix: after both bounds validate, parse them and refuse when `end <= start` or `end - start >= 62 * 24h` with a message that names the ceiling and the two parameters; add a test. The validate package already parses `datetimeFormats`, so expose or reuse that parse.
* Votes: 3 of 3 confirm.

#### D6. `internal/tools/get_event_attachment.go:118`, the size ceiling runs after the download

* Claim: the ceiling is checked after Graph has returned, and the SDK has parsed, the full `contentBytes` payload, so the refusal does not prevent the allocation the docstring says it prevents.
* Spec evidence: `attachment-get` v1.0 Example 1: a plain GET on a `fileAttachment` returns `contentBytes` inline. The item request builder supports `$select` (template `...attachments/{attachment%2Did}{?%24expand,%24select}`); `/$value` is a separate call.
* Code evidence: `.Get(timeoutCtx, nil)` with no `$select`, then `if sz := att.GetSize(); sz != nil && int64(*sz) > maxSize`. `models/file_attachment.go` registers a `contentBytes` deserializer, so the base64 payload is decoded into memory before the check. The docstring says "a single tool call cannot allocate unbounded memory" and "refused rather than copied into the result". `list_event_attachments.go:93` already uses `listAttachmentsSelectFields` for a metadata-only read.
* Fix: issue a first GET with `Select: listAttachmentsSelectFields`, check `size` against `maxSize`, and only then issue the content GET. Or correct the docstring and the governing change request to say the ceiling bounds the tool result, not the download.
* Votes: 3 of 3 confirm; the impact-lens refuter grades it a false docstring claim rather than a runtime defect.

#### D7. `internal/tools/contacts_search.go:191`, the contacts half is silently one page

* Claim: the contacts request sets no `$top` and `@odata.nextLink` is never read, so a mailbox whose matches exceed the server page returns a truncated match list with no marker.
* Spec evidence: `user-list-contacts` returns "a collection of contact objects"; Graph collections page via `@odata.nextLink` (`paging` concept). `search-query-parameter.md` gives a 250-result default only for `/me/people`, nothing for contacts.
* Code evidence: lines 98-99 send only `Search`; `mergeContactMatches` (lines 161-185) iterates `contactsResp.GetValue()` once. The governing change request bounds the verb to two Graph requests, so following the link is out of scope, but neither the handler doc comment nor the verb Description states the single-page bound.
* Fix: set an explicit `$top` and append a "more results available" marker when `GetOdataNextLink()` is non-nil; state the bound in the Description.
* Votes: 3 of 3 confirm.

#### D8. `internal/tools/contacts_list_people.go:76`, `list_people` caps at 10 with no disclosure

* Claim: the verb returns only the default page of 10 people, exposes no `limit` or `skip`, never reads `@odata.nextLink`, and its Description does not state the cap.
* Spec evidence: `people-insights-overview`: "By default, each response returns 10 records, but you can change this by using the $top query parameter" and "you can make a second request using $top and $skip to request additional pages". `user-list-people` lists `$top` and `$skip`.
* Code evidence: `client.Me().People().Get(timeoutCtx, nil)`; `serializePeopleCollection` reads `resp.GetValue()` only; `internal/server/contacts_verbs.go:175-200` declares only `account` and `output`. The SDK exposes `Top` and `Skip` on `ItemPeopleRequestBuilderGetQueryParameters`.
* Fix: add an optional `limit` (mapped to `$top`) and optionally `skip`, and state in the Description and the text footer that at most N (default 10) people are returned. Add a test that asserts `$top` is sent.
* Votes: 2 confirm, 1 impact-lens refute: the top 10 by relevance is the valid documented response and the single-page read is a stated design choice, so the residual gap is the undisclosed cap.

#### D9. `internal/tools/teams_serialize.go:146`, `teams.search` projects fields the Search API never returns

* Claim: the summary and raw tiers of `teams.search` emit `bodyPreview`, `body`, `attachments`, `mentions`, `messageType`, `lastEditedDateTime`, `replyToId` and `locale`, none of which the Search API returns for `chatMessage` hits, so they are always empty on the real service while the one preview the service does return (`hit.summary`) is not what the text formatter prints.
* Spec evidence: `search-concept-chat-messages`, Known limitations: "The search Teams API doesn't return all properties defined in chatMessage." Its JSON representation of retrievable properties lists only `channelIdentity`, `chatId`, `createdDateTime`, `etag`, `from`, `id`, `importance`, `lastModifiedDateTime`, `subject`, `webUrl`. No example response carries a body.
* Code evidence: `SerializeTeamsSearchHit` (line 428) routes to `SerializeSummaryChatMessage` / `SerializeSummaryChannelMessage`, which read `msg.GetBody()` (lines 146, 189); `teamsMessageRawCommon` (line 495) emits body, attachments, mentions. `teams_search_test.go:105` includes a `body` on each hit, so `TestTeamsSearch_RawTierCarriesFullBody` passes against a shape the doc rules out. `FormatTeamsSearchHitsText` (`text_format.go:1213`) prints `bodyPreview`.
* Fix: build a dedicated search-hit field set from the documented properties, using `hit.summary` as the preview; drop body, attachments, mentions and messageType from this verb; state in the Description that the full body comes from the follow-up read verbs; remove `body` and `messageType` from the fixture.
* Votes: 3 of 3 confirm.

#### D10. `internal/tools/teams_serialize.go:526`, the sender is blank on every search hit

* Claim: `teamsSenderName` reads `from.user` and `from.application`, but the documented search response carries the sender as `from.emailAddress.{name,address}`, which the SDK drops into `additionalData`.
* Spec evidence: `search-concept-chat-messages` Examples 1, 2 and 3: `"from": { "emailAddress": { "name": "Goncalo Torres", "address": "gtorres@contoso.com" } }` with no `user` or `application` member.
* Code evidence: lines 526-538 return `GetUser().GetDisplayName()` or `GetApplication().GetDisplayName()`, else `""`. SDK `models/identity_set.go` registers deserializers only for `application`, `device`, `@odata.type`, `user`; `chat_message_from_identity_set.go` adds none. The fixtures at `teams_search_test.go:121,135` use `from.user.displayName`.
* Fix: after the user and application branches, read `from.GetAdditionalData()["emailAddress"]` as `map[string]any` and return its `name` (or `address`). Add a fixture whose `from` carries `emailAddress` only.
* Votes: 3 of 3 confirm.

#### D11. `internal/tools/teams_search.go:149`, no paging and no `moreResultsAvailable`

* Claim: the request sets no `size` or `from` and the handler ignores `moreResultsAvailable`, so the caller gets the service default of 25 hits with no way to page or to learn more exist. This verb is the domain's only identifier resolver.
* Spec evidence: `search-api-overview`, Page search results: "from ... default value is 0. size ... The default is 25 results. The maximum is 1000 results." (`searchrequest` says max 500; the two pages conflict.) `searchHitsContainer` carries `moreResultsAvailable`, shown `true` in `search-query` Example 1.
* Code evidence: `buildTeamsSearchBody` (lines 149-161) sets only `entityTypes` and `query`; `collectTeamsSearchHits` (line 175) reads `GetHits()` only. The SDK exposes `SetFrom`, `SetSize` and `GetMoreResultsAvailable`. `internal/server/teams_verbs.go` declares only `query`, `account`, `output`.
* Fix: add optional `limit` (default 25, validated against the conservative 500) and `from`, and surface `moreResultsAvailable` as a trailing JSON field and a "More results available" text line.
* Votes: 3 of 3 confirm.

#### D12. `internal/tools/teams_list_chats.go:75`, the ordering claim has no request behind it

* Claim: the serializer docstring says Graph's order "puts the most recently active conversations first", but no `$orderby` is sent and the doc guarantees that order only under `$orderby=lastMessagePreview/createdDateTime desc`.
* Spec evidence: `chat-list` v1.0: "$orderby: Currently supports lastMessagePreview/createdDateTime in descending order." Example 3 requires it; no default order is documented.
* Code evidence: `teams_list_chats.go:120-121` docstring on `serializeChatCollection`; the call at line 75 sends no `Orderby`. The user-facing Description and the exported handler docstring do not make the claim.
* Fix: set `Orderby = []string{"lastMessagePreview/createdDateTime desc"}`, or remove the ordering clause from the docstring.
* Votes: 2 confirm, 1 impact-lens refute: the claim lives only in an internal comment, so the fix is a one-line docstring correction.

#### D13. `internal/tools/teams_list_chats.go:75`, `list_chats` is one page with no marker

* Claim: the handler reads the first page only, sets no `$top`, never reads `@odata.nextLink`, and the Description does not say so, so a user with more chats than one page (about 20 by the doc's own `skiptoken`) gets a truncated list presented as complete.
* Spec evidence: `chat-list` v1.0 Note: "If the result set for all chats spans multiple pages, the response object includes an @odata.nextLink property ... continue making additional requests ... until all the results are returned." `$top` max is 50.
* Code evidence: docstring at lines 46-48 states the single request; `serializeChatCollection` reads `resp.GetValue()` only. `internal/server/teams_verbs.go:177` promises to list "the signed-in user's Teams conversations" and says this is how a chat id is resolved. `list_events.go:279`, `list_messages.go:373`, `get_conversation.go:216` already use `msgraphcore.NewPageIterator`.
* Fix: set `$top=50` and follow the link with a page iterator up to a `max_results` cap, or at minimum surface a truncation field when `GetOdataNextLink()` is non-nil and state the bound in the Description.
* Votes: 3 of 3 confirm.

#### D14. `internal/tools/teams_list_chat_messages.go:85`, one page in `lastModifiedDateTime` order

* Claim: the handler reads one page without `$top` or `$orderby` and ignores `@odata.nextLink`, so a long conversation is truncated and the order is the undocumented default, which the doc says is `lastModifiedDateTime` and which changes when a reaction is added to an old message.
* Spec evidence: `chat-list-messages` v1.0: "$top ... Maximum allowed $top value is 50." "$orderby: Currently supports the lastModifiedDateTime (default) and createdDateTime properties in descending order." Every example response carries `@odata.nextLink`. `chatmessage` resource: `lastModifiedDateTime` changes "including when a reaction is added or removed".
* Code evidence: `Messages().Get(timeoutCtx, nil)`; docstring lines 49-51. The SDK exposes `Orderby` and `Top` on `ItemChatsItemMessagesRequestBuilderGetQueryParameters`.
* Fix: set `Top` to 50 and expose whether a next link was present (or iterate with a `max_results` cap as `list_events.go` does); consider `Orderby = []string{"createdDateTime desc"}`.
* Votes: 3 of 3 confirm.

#### D15. `internal/tools/teams_serialize.go:136`, system events render as empty-sender messages

* Claim: without `Prefer: include-unknown-enum-members`, system event records arrive as `messageType: unknownFutureValue` with `from: null` and body `<systemEventMessage/>`, and the summary serializer interleaves them with real messages as blank-sender rows.
* Spec evidence: `chat-list-messages` v1.0 Example 1 (no Prefer header) shows exactly that record with an `eventDetail`; Example 2 shows it as `systemEventMessage` only under the header. `messageType` is an evolvable enum.
* Code evidence: `SerializeSummaryChatMessage` (lines 136-150) emits `messageType` via `teamsEnumStr` and `from` via `teamsSenderName` (returns `""` for nil). No Teams list handler sets `Headers`; no code under `internal/` reads `eventDetail`. `TeamsPreview` does no tag stripping, so `bodyPreview` is `<systemEventMessage/>`.
* Fix: send `Prefer: include-unknown-enum-members` on `list_chat_messages` and `get_chat_message`, and either filter `messageType != "message"` out of the summary and text tiers or render `eventDetail`'s `@odata.type` so a system event is labelled.
* Votes: 3 of 3 confirm.

#### D16. `internal/tools/teams_list_channel_messages.go:97`, 20 posts presented as the channel total

* Claim: the listing reads one page at the service default of 20, sends no `$top`, does not follow `@odata.nextLink`, and the text footer prints "%d message(s) total.", so a channel with more threads is mis-served as complete.
* Spec evidence: `channel-list-messages` v1.0 (ms.date 2024-10-02): "The default page size is 20 messages. You can extend up to 50 channel messages per page." Example 1 carries `@odata.nextLink`. Messages are "sorted by the last modified date of the entire reply chain".
* Code evidence: `.Messages().Get(timeoutCtx, nil)`; doc comment lines 57-58; `internal/server/teams_verbs.go:258` Description states no bound; `text_format.go:1299` prints the total. The SDK exposes `Top` on `ItemChannelsItemMessagesRequestBuilderGetQueryParameters`.
* Fix: pass `Top` capped at 50 or expose a `limit` (1 to 50), and state in the Description and text footer that the result is the newest-modified page and older threads are reached through `teams.search`. Add a test for the `Top` value and the cap.
* Votes: 3 of 3 confirm.

#### D17. `internal/tools/teams_list_channel_message_replies.go:103`, a long thread is cut silently

* Claim: the replies listing sends no `$top` and does not follow `@odata.nextLink`, so a thread longer than the service page is truncated and the caller is not told.
* Spec evidence: `chatmessage-list-replies` v1.0: "You can use the $top query parameter to control the number of items per response. Maximum allowed $top value is 50. The other OData query parameters aren't currently supported." No default page size is stated.
* Code evidence: `Replies().Get(timeoutCtx, nil)`; doc comment line 57; `internal/server/teams_verbs.go:303` Description states no bound. The SDK exposes `Top` on `ItemChannelsItemMessagesItemRepliesRequestBuilderGetQueryParameters`.
* Fix: pass `Top` at most 50 and state in the Description that at most one page is returned, or follow the link up to a stated cap. Add a test for the `Top` value.
* Votes: 3 of 3 confirm.

#### D18. `internal/tools/teams_get_transcript.go:183`, no transcript format negotiation

* Claim: the content read sends the SDK default `Accept: application/octet-stream, application/json`, and the handler never branches on the `SpeakerAttributionNotAllowed` inner-error code to retry with the unattributed format. In a tenant that disables speaker attribution every content read fails with a misleading fix.
* Spec evidence: `calltranscript-get` (updated 2026-10-01): select the format with `Accept` (`text/vtt` or `application/vnd.microsoft.graph.transcript+text`, optional, `text/vtt` by default); when attribution is disabled, requesting `text/vtt` "returns 403 Forbidden with the SpeakerAttributionNotAllowed inner-error code. Retry with the unattributed format"; "Branch on the innerError.code value, not the message text." The controls took effect at the end of July 2026.
* Code evidence: `transcriptContent` calls `builder.Content().Get(ctx, nil)`; SDK `ToGetRequestInformation` adds the default `Accept`. `transcriptGraphFailure` appends the generic `getTranscriptGraphFix` and never inspects `innerError.code`. The serializer labels content `WEBVTT` unconditionally. `grep SpeakerAttributionNotAllowed` over `internal/` and `docs/` returns nothing.
* Fix: pass a request configuration with `Accept: text/vtt`; on a 403 whose `innerError.code` is `SpeakerAttributionNotAllowed`, retry with `application/vnd.microsoft.graph.transcript+text` and mark the result speaker-unattributed via a `contentFormat` field. Add a troubleshooting anchor for both inner-error codes.
* Votes: 3 of 3 confirm; the impact-lens refuter notes it is harmless for default tenants and a hard failure for attribution-disabled tenants.

#### D19. `internal/tools/teams_get_transcript.go:145` and `internal/tools/teams_list_transcripts.go:34`, the fix text names a remedy that cannot work

* Claim: a `GraphAccessToTranscriptsDisabled` 403 is reported with a fix that names consent and scopes and says "then retry", but the documented cause is a tenant admin setting with no request-side workaround.
* Spec evidence: `onlinemeeting-list-transcripts` and `calltranscript-get`: "If a tenant administrator has turned off Graph API access to transcripts for the tenant, this request returns 403 Forbidden with the GraphAccessToTranscriptsDisabled inner-error code. There is no request-side workaround; the app receives this response until an administrator re-enables access." The setting lives in the Teams Admin Center or `Set-CsTeamsMeetingConfiguration`.
* Code evidence: `getTranscriptGraphFix` (line 43) and `listTranscriptsGraphFix` (line 34) are fixed strings attached to every non-timeout Graph error. `internal/graph/errors.go` reads only the top-level ODataError code, never `innerError`. `docs/troubleshooting.md` has no entry for the code.
* Fix: inspect `innerError.code` in both handlers; when it is `GraphAccessToTranscriptsDisabled`, emit a fix that names the tenant admin action and states that a retry cannot help. Add a troubleshooting anchor.
* Votes: 3 of 3 confirm.

#### D20. `internal/auth/email_resolver.go:37`, `GET /me` without `User.Read`

* Claim: `EnsureEmail` calls `GET /me`, documented under delegated `User.Read`, but `Scopes()` never requests a `User.*` scope; MSAL adds only `openid`, `profile`, `offline_access`. On failure the resolver logs a warning and leaves `AccountEntry.Email` empty.
* Spec evidence: `user-get` v1.0 Permissions: Delegated (work or school account) least privileged `User.Read`. `scopes-oidc`: `profile` gives id_token claims and the UserInfo endpoint, not Graph `/me`.
* Code evidence: `entry.Client.Me().Get(ctx, nil)`; `auth.go:120` `Scopes()` has no `User.*` scope; MSAL v1.8.0 `accesstokens.go:485` `defaultScopes = []string{"openid", "offline_access", "profile"}`. `extension/manifest.json:7` and `infra/app-registration.json:45` document `User.Read` as required, so the project documentation and the code disagree.
* Fix: request `User.Read` in `Scopes()`, or read the username from the azidentity `AuthenticationRecord` or the id_token `preferred_username` claim so no undocumented scope dependency remains; make the failure visible.
* Votes: 2 confirm, 1 impact-lens refute: the default first-party Office client id is pre-authorized for `User.Read.All`, and the `scopes-oidc` page says pre-authorized permissions always appear in the token, so the call succeeds under the shipped client id and fails only for a custom client id without a `User.*` grant, where every consumer tolerates an empty Email.

### Risks

#### R1. `internal/tools/set_flag.go:74`, status-only flag write

* Claim: setting `flagStatus` sends no `startDateTime`, `dueDateTime` or `completedDateTime`; the docs state no rule for a status-only write.
* Spec evidence: `followupflag` resource: "To set the due date, you must also specify the startDateTime; otherwise, you get a 400 Bad Request response." `message-update`: omitted properties "maintain their previous values or be recalculated".
* Code evidence: `models.NewFollowupFlag()`; only `SetFlagStatus`; nil date fields are skipped by the SDK serializer.
* Fix: keep as is; optionally record in the Description that dates are not set.
* Votes: 2 confirm, 1 impact-lens refute (the general PATCH contract documents the omitted-property behaviour).

#### R2. `internal/tools/set_categories.go:101` and `internal/tools/mark_read.go:90`, unguarded PATCH result

* Claim: both handlers call `updated.GetId()` on the PATCH result with no nil guard, while the pinned SDK returns `(nil, nil)` on an empty body.
* Spec evidence: `message-update`: "returns a 200 OK response code and updated message object in the response body." No 204 is documented.
* Code evidence: `users/item_messages_message_item_request_builder.go:158-160` `if res == nil { return nil, nil }`; `kiota-http-go` v1.5.6 `shouldReturnNil` returns nil on 204. Neither handler sends `Prefer: return=minimal`. `main.go:179` registers `server.WithRecovery()`.
* Fix: add `if updated == nil` before reading fields and degrade to the existing "unknown (not reported by Graph)" wording.
* Votes: 2 confirm, 1 impact-lens refute on each (the 204 path is not reachable without the Prefer header, and the panic is recovered).

#### R3. `internal/tools/move_message.go:87`, a retried move reports failure for a move that succeeded

* Claim: the non-idempotent move runs inside `RetryGraphCall`, which retries 429, 503 and 504; a retry after the service applied the move resends the original id, gets 404, and the handler reports failure.
* Spec evidence: `message-move`: the move "creates a new copy of the message in the destination folder and removes the original message"; nothing documents idempotency or that a 5xx implies no state change.
* Code evidence: `internal/graph/retry.go:122-123`; `move_message.go:86-90`; `mail_verbs.go:630` annotates the verb non-idempotent. The Kiota `RetryHandler` (`kiota-http-go` v1.5.6 `retry_handler.go:171-179`) already retries POST on the same codes, so removing 503/504 from the outer loop does not remove the exposure.
* Fix: on a 404 after a prior 5xx, report the ambiguity and tell the caller to re-list the destination folder.
* Votes: 3 of 3 confirm.

#### R4. `internal/tools/add_attachment.go:38` and `internal/tools/add_event_attachment.go:137`, the 3 MB boundary is a choice, not a documented figure

* Claim: `inlineAttachmentThresholdBytes = 3 * 1000 * 1000` is commented as "the figure the service documents", but no doc defines 3 MB in bytes; a payload of 3,000,000 to 3,145,727 bytes goes to `createUploadSession` and is rejected if the service minimum is 3 MiB. No fallback exists.
* Spec evidence: `message-post-attachments` and `event-post-attachments`: "under 3 MB". `attachment-createuploadsession`: "between 3 MB and 150 MB". `outlook-large-attachments` Errors: `ErrorAttachmentSizeShouldNotBeLessThanMinimumSize` for "a file smaller than 3 MB". No byte value anywhere; the doc example size 3483322 is above both candidates.
* Code evidence: `if size >= inlineAttachmentThresholdBytes` routes to the session path; `grep ErrorAttachmentSizeShouldNotBeLessThanMinimumSize internal/` returns nothing. The project backlog records the decimal boundary as the implementer's chosen reading, unverified against a live mailbox.
* Fix: correct the comment; on the minimum-size error from `createUploadSession` fall back to the direct POST (or the reverse on a direct-POST size rejection); record the observed boundary in `docs/troubleshooting.md` once live access confirms it.
* Votes: 3 of 3 confirm on both files.

#### R5. `internal/tools/attachment_upload_session.go:211`, fail-whole chunk transfer

* Claim: chunk PUTs have no retry, no resume from `nextExpectedRanges`, and no DELETE of the session on failure, so one transient failure discards the whole transfer and leaves the session open until expiry.
* Spec evidence: `outlook-large-attachments` Step 2 and 3: `nextExpectedRanges` is "useful if you need to resume a transfer that was interrupted"; "Cancel the upload session" via DELETE on the same URL returns 204.
* Code evidence: `transferAttachmentChunks` loops sequentially with a bare `&http.Client{}`, returns on the first error, never reads `nextExpectedRanges` on 200, never issues DELETE; `RetryGraphCall` wraps only the session-creation POST.
* Fix: optional hardening: re-PUT the same range once or twice on 5xx or transport error; best-effort DELETE on final failure. Otherwise document the fail-whole behaviour.
* Votes: 3 of 3 confirm (bounded impact: no data corruption, the attachment exists only after the final 201).

#### R6. `internal/auth/auth.go:22`, `Calendars.ReadWrite` is not a scope the `findMeetingTimes` doc lists

* Companion of B4. Claim, evidence and fix are the same.
* Votes: 2 confirm, 1 impact-lens refute: the permissions include listed `Calendars.Read` / `Calendars.ReadWrite` for this endpoint in October 2023, the `.Shared` switch was a catalog regeneration, and nothing documents rejection of non-Shared scopes.

#### R7. `internal/tools/find_meeting_times.go:139` and `internal/tools/get_schedule.go:296`, an offset inside `dateTime` next to a separate `timeZone`

* Claim: `start_datetime` and `end_datetime` accept RFC 3339 values with `Z` or a numeric offset and post them verbatim inside a `dateTimeTimeZone` that also carries an IANA `timeZone`, a combination the resource doc does not define; the schema text "ISO 8601 without offset" is not enforced.
* Spec evidence: `datetimetimezone` v1.0: `dateTime` is "{date}T{time}; for example, 2017-08-29T04:00:00.0000000"; the zone is carried by `timeZone`. The `getSchedule` example uses "2019-03-15T09:00:00" with a separate `timeZone`.
* Code evidence: `internal/validate/validate.go:46-52` accepts `...Z07:00`, `time.RFC3339` and `...Z` layouts; `buildTimeConstraint` and `get_schedule.go:135-136` pass the raw string into `newDateTimeTimeZone`. `get_schedule_test.go:147` asserts the body carries "2026-03-12T00:00:00Z", so the tests pin this shape. `internal/server/calendar_verbs.go:858-861` says "ISO 8601 without offset".
* Fix: reject an offset or `Z` for these verbs, or convert the instant into the chosen zone and format as `2006-01-02T15:04:05` before building the `DateTimeTimeZone`; change the fixtures to offset-free values.
* Votes: 3 of 3 confirm at `find_meeting_times.go:139`; 2 confirm, 1 impact-lens refute at `get_schedule.go:296` (no doc states rejection; the fixtures pair `Z` with `timeZone` UTC, which is consistent).

#### R8. `internal/tools/get_schedule.go:126` (current tree line 115), any host IANA zone is accepted

* Claim: `validate.ValidateTimezone` is `time.LoadLocation`, so any IANA zone the host knows passes, but the doc lists Windows zones plus a finite set of IANA names; `Europe/Stockholm` is not in that list.
* Spec evidence: `datetimetimezone` v1.0 (ms.date 2024-08-08): `timeZone` "can be set to any of the time zones currently supported by Windows, as well as the other time zones supported by the calendar API", followed by a finite "Additional time zones" list.
* Code evidence: `validate.go:80-86`; the same validator serves `find_meeting_times.go` and `get_free_busy.go`.
* Fix: document the constraint, or map the IANA zone to a Windows zone name before posting, once in a shared helper. One `getSchedule` call with `timezone=Europe/Stockholm` settles it when access returns.
* Votes: 2 confirm, 1 impact-lens refute (the list reads as the aliases from `supportedTimeZones(Iana)`, not an accept-list; Graph resolves IANA names through the CLDR mapping).

#### R9. `internal/tools/list_event_attachments.go:117`, no `@odata.nextLink` handling

* Claim: the collection read iterates `resp.GetValue()` once; a paged attachments collection is truncated to the first page.
* Spec evidence: `event-list-attachments` v1.0: "collection of Attachment objects", supports OData query parameters, no page size stated. `paging` concept: "Not all resources or relationships support paging."
* Code evidence: no call to `GetOdataNextLink` anywhere under `internal/`; the mail sibling `list_attachments.go:122` has the same shape.
* Fix: check `GetOdataNextLink()` and report truncation, or follow it with a page iterator.
* Votes: 2 confirm, 1 impact-lens refute (paging of this collection is undocumented either way; probe with more than 10 attachments when access returns).

#### R10. `internal/tools/contacts_search.go:106` and `:99`, `$search` on `/me/contacts` is undocumented

* Claim: the handler sends `$search` to `GET /me/contacts`, but the v1.0 List contacts page documents only `$filter` on `emailAddresses/any(a:a/address eq ...)`; `$search` is documented for messages, people and directory objects only, and unsupported query parameters "might fail silently". If Graph rejects the parameter the whole verb fails under its all-or-nothing rule; if Graph ignores it the verb returns every contact as a match because `mergeContactMatches` does no client-side filtering.
* Spec evidence: `user-list-contacts` Optional query parameters; `search-query-parameter.md` ("Support for this query parameter varies by entity"); `query-parameters.md:829` on silent failure.
* Code evidence: `ItemContactsRequestBuilderGetQueryParameters{Search: &normalised}`. The SDK `Search` field is generated for every collection and is not evidence of support. `.agents/scenarios/2026-09-02-contacts-name-resolution-live-mailbox.md` records the dependency as never run.
* Fix: run the recorded live probe scenario, or replace the contacts half with the documented `$filter` when the query is an address and a client-side `displayName` match over a paged `GET /me/contacts` otherwise.
* Votes: 3 of 3 confirm on both entries.

#### R11. `internal/tools/contacts_get_person.go:83`, `GET /me/people/{id}` is not a documented operation

* Claim: no v1.0 reference page documents a single-person GET; the verb depends on metadata-only behaviour and asserts "relevance identifiers are not durable" without a source.
* Spec evidence: `person-get` returns HTTP 404 for v1.0 and beta; the `person` resource page lists one method, "List people"; `people-insights-overview` documents only collection reads.
* Code evidence: `client.Me().People().ByPersonId(personID).Get(timeoutCtx, nil)` via SDK template `/users/{user-id}/people/{person-id}`; `getPersonGraphFix` at line 34 carries the durability claim.
* Fix: document in the Description and `docs/concepts.md` that the single-person read is undocumented and may 404, or re-implement as `GET /me/people?$filter=id eq '{id}'` with the item builder as fallback; soften the durability claim.
* Votes: 3 of 3 confirm.

#### R12. `internal/tools/contacts_list_people.go:76`, maintenance mode and mailbox-only scope undisclosed

* Claim: the People API is in maintenance mode and serves mailbox-only results by default; the verb discloses neither, so an LLM can read an absent person as unknown to the tenant.
* Spec evidence: `user-list-people`: "The People API is in maintenance mode"; recommends `POST /search/query` with `entityTypes: ["person"]`; not available in the China (21Vianet) cloud. `people-insights-overview`: "By default, Microsoft Graph serves mailbox-only results ... specify an HTTP header X-PeopleQuery-QuerySources: Mailbox,Directory".
* Code evidence: `Get(timeoutCtx, nil)`; `internal/server/contacts_verbs.go:176-177` says "inferred correspondents rather than saved contacts" and nothing more; `grep -i "maintenance\|QuerySources" docs/ internal/` returns nothing.
* Fix: record both caveats in `docs/concepts.md` and the Description; optionally expose a directory flag that sets the header via `Headers`.
* Votes: 3 of 3 confirm.

#### R13. `internal/tools/teams_serialize.go:432`, an undecorated discriminator drops every hit

* Claim: if the service emits `"@odata.type": "microsoft.graph.chatMessage"` without the leading `#`, as the Teams search doc shows, the SDK deserializes a bare `Entity`, the `ChatMessageable` assertion fails, and every hit is dropped silently.
* Spec evidence: `search-concept-chat-messages` examples use `microsoft.graph.chatMessage`; `search-query` Example 2 uses `#microsoft.graph.listItem`. The docs are inconsistent.
* Code evidence: SDK `models/search_hit.go:112` and `models/entity.go:344` match only `#microsoft.graph.chatMessage`; line 432 returns nil on a failed assertion; the fixture uses the `#` form only.
* Fix: fall back to re-parsing from `Entity.GetAdditionalData`, or log a warning that names the discriminator received; add a test with the undecorated form; confirm the live wire form.
* Votes: 2 confirm, 1 impact-lens refute (every Kiota SDK in every language keys on the `#` form; a bare form would be a widely reported break).

#### R14. `internal/tools/teams_serialize.go:149`, `webUrl` may be empty on search hits

* Claim: the documented example carries the link as `webLink` while the JSON representation lists `webUrl`; the SDK model reads only `webUrl`.
* Spec evidence: `search-concept-chat-messages` Examples 1 to 3 (`webLink`) against its JSON representation (`webUrl`).
* Code evidence: lines 149, 191 and 507 emit `graph.SafeStr(msg.GetWebUrl())`; `models/chat_message.go` has no `webLink` field.
* Fix: fall back to `msg.GetAdditionalData()["webLink"]` when `GetWebUrl()` is nil, or drop the field from search hits until the wire name is confirmed.
* Votes: 3 of 3 confirm (low impact).

#### R15. `internal/tools/teams_get_channel_message.go:105`, a reply id on the root-message path

* Claim: a `message_id` that names a reply (from `list_channel_message_replies` or a search hit with `replyToId`) is read through `/messages/{id}`, while the doc gives `/messages/{id}/replies/{reply-id}` as a distinct path. Secondary sources report the service rejects a reply id on the root path.
* Spec evidence: `chatmessage-get` v1.0 (ms.date 2024-10-30) lists both paths; Example 3 reads a reply only through `/replies/{reply-id}`.
* Code evidence: `Messages().ByChatMessageId(messageID).Get(timeoutCtx, nil)` for every id; the same at `teams_compose_reply.go:116`; the fix strings direct callers to take `message_id` from a search hit. The SDK exposes `Replies().ByChatMessageId1(...)`.
* Fix: accept an optional `reply_to_id` (or read `replyToId` from the hit) and address the reply path when present; until then state that `message_id` must be a root post id.
* Votes: 3 of 3 confirm.

#### R16. `internal/tools/teams_list_transcripts.go:99`, first page only

* Claim: the transcripts collection supports `$top` and may page; the handler reads one page and ignores `@odata.nextLink` without telling the caller.
* Spec evidence: `onlinemeeting-list-transcripts`: "This method supports the $select, $filter, and $top OData query parameters." No page size; the sample response has no `@odata.nextLink`.
* Code evidence: `Transcripts().Get(timeoutCtx, nil)`; `serializeTranscriptCollection` iterates `resp.GetValue()` only; the SDK response embeds `BaseCollectionPaginationCountResponse`.
* Fix: read `GetOdataNextLink()` and flag truncation.
* Votes: 2 confirm, 1 impact-lens refute (a meeting holds few transcripts; no source shows this collection pages).

## Refuted findings

| File:line | Claim | Refutation |
|---|---|---|
| `internal/server/calendar_verbs.go:992` | `get_event_attachment` never states that item and reference attachments return no content. | `FormatAttachmentText` (`text_format.go:719-723`) already prints "This attachment has no downloadable file content (not a file attachment)." and `odataType` is emitted on every attachment; the handler uses the plain GET, so the documented 405 for `/$value` on a reference attachment cannot occur. Residue is a wording imprecision in the Description only. |
| `internal/tools/attachment_upload_session.go:274` | Intermediate `nextExpectedRanges` are discarded, so a partially accepted chunk silently corrupts the upload. | The doc says a 200 means the chunk was accepted and bytes must be uploaded in order; `nextExpectedRanges` is for resuming "a transfer that was interrupted and your client is unsure of the state". The client uploads strictly sequential ranges and aborts on any non-2xx, so after every 200 the only possible next range is the fixed next offset. No documented path yields a 200 for a partial chunk. Resume-on-failure is covered by R5. |
| `internal/tools/teams_get_online_meeting.go:270` | The `joinWebUrl` filter value is not URL-encoded and relies on the SDK to encode `&`, `=`, `%` and `#`. | A probe through the real handler and recorder showed the SDK template expansion (`std-uritemplate` via `kiota-abstractions-go` v1.9.4) percent-encodes every reserved character exactly once, and `url.ParseQuery` decodes `$filter` back to the exact OData expression. The existing test already covers `%3a` and `%40`. Pre-encoding would double-encode and break the match. |

## Verified OK

Grouped by verb. Each item was checked against the pinned SDK and the v1.0 page and matched.

### mail.set_flag

* PATCH `/me/messages/{id}` with `Accept` and `Content-Type: application/json`.
* Body `{"@odata.type":"#microsoft.graph.message","flag":{"flagStatus":"<value>"}}`; keys match the `message-update` and `followupFlag` docs.
* Enum values `notFlagged`, `complete`, `flagged` match the doc; the validator accepts only these three.
* Nil date fields are skipped, so no empty `dateTimeTimeZone` triggers the documented 400.
* `Mail.ReadWrite` is requested under `MailManageEnabled`, the documented least privilege.
* Response read through nil-guarded accessors; "unknown (not reported by Graph)" when absent.
* `ValidateFlagStatus` precedes `ParseFlagStatus`, so an unrecognised input cannot clear a flag.
* Exactly one PATCH; no read-modify-write; no query options; `idempotent=true` is consistent.

### mail.set_categories

* PATCH `/me/messages/{id}`; `categories` serialized as a string collection, matching the doc.
* `categories` is updatable on any message, so no `isDraft` guard is correct.
* A non-nil empty slice serializes as `[]` (clear semantics), pinned by `TestSetCategories_EmptyValueClears`.
* Minimal body; `Mail.ReadWrite` requested; response read with nil and empty guards in `responseCategories`.
* No query options, no pagination; tests cover single PATCH, body, confirmation, clear, over-length rejection, missing parameter.

### mail.mark_read

* PATCH `/me/messages/{id}`; only `isRead` serialized as a JSON bool; no `isDraft` restriction in the doc.
* Minimal body; SDK sets `Accept` and `Content-Type`; no query options.
* `Mail.ReadWrite` gated behind `MAIL_MANAGE_ENABLED`.
* `id`, `subject`, `isRead` read nil-safe; `TestMarkRead_ConfirmationStatesResponseNotRequest` pins the service-stored value.
* Annotations `readOnly=false`, `destructive=false`, `idempotent=true`, `openWorld=true` consistent; `is_read` required as a boolean; `message_id` validated first.
* Tests run the real Kiota adapter against `httptest` and assert the raw body casing.

### mail.move_message

* POST `/me/messages/{id}/move` with the single documented parameter `destinationId`.
* `Mail.ReadWrite` requested and the verb registered only under that gate.
* 201 with a message resource parsed into `Messageable`; `id` and `subject` nil-guarded.
* The confirmation reports the new id and states the original no longer resolves, matching the doc.
* No query options or pagination; both identifiers validated before any request; redacted error plus fix on both channels.

### mail.add_attachment

* Direct path: POST `/me/messages/{id}/attachments`; `FileAttachment` sets `@odata.type` `#microsoft.graph.fileAttachment`; keys `name`, `contentType`, `contentBytes` (base64) match the doc; 201 read nil-guarded.
* `Mail.ReadWrite` requested, the least privilege for both documented calls.
* Session path: POST `.../attachments/createUploadSession`; body key `AttachmentItem` with `attachmentType: file`, `name`, `size` (int64), `contentType`.
* `uploadUrl` read with nil and empty guards; `expirationDateTime` and `nextExpectedRanges` present in the model.
* Chunk PUT: `Content-Type: application/octet-stream`, `Content-Length`, `Content-Range: bytes {start}-{end}/{total}` 0-based inclusive, no `Authorization`, URL used verbatim, sequential order, 1,638,400 bytes per chunk (under the 4 MB bound).
* Intermediate 200 accepted; final 201 is completion; completion without an identifier is a failure.
* The upload URL (which carries `authtoken`) is redacted from result and log; tests cover both.
* Size checks are on decoded bytes; per-call timeout on the draft GET, session POST and each PUT; retry on the SDK calls.

### calendar.find_meeting_times

* POST `/me/findMeetingTimes` with `Accept` and `Content-Type: application/json`.
* Body keys `attendees`, `isOrganizerOptional`, `maxCandidates`, `meetingDuration`, `minimumAttendeePercentage`, `returnSuggestionReasons`, `timeConstraint.timeSlots[].start/end.dateTime/timeZone` match the API example.
* `attendeeBase` shape `emailAddress.address` plus `type` with `required`, `optional`, `resource`.
* `meetingDuration` as ISO 8601 via `WriteISODurationValue`; default `PT30M` matches the doc default.
* `isOrganizerOptional` false by default; `returnSuggestionReasons` true; `minimumAttendeePercentage` bounded 0 to 100 and sent only when supplied; `timeConstraint` omitted when no window; `activityDomain` omitted.
* IANA zone names on `timeZone` are documented as accepted.
* Response fields `emptySuggestionsReason`, `confidence`, `order`, `organizerAvailability`, `suggestionReason`, `meetingTimeSlot`, `attendeeAvailability`, `locations` nil-guarded.
* Not a paged collection; timeout and retry applied; tests assert body fields, half-open window refusal, empty-suggestion reason.

### calendar.get_schedule

* POST `/me/calendar/getSchedule`; `Calendars.ReadWrite` is a documented higher-privileged permission.
* Body fields `Schedules`, `StartTime`, `EndTime`, `AvailabilityViewInterval` in the PascalCase the doc's own example uses.
* `availabilityViewInterval` default 30, min 5, max 1440 match the doc.
* Response fields `scheduleId`, `availabilityView`, `scheduleItems` (`start`, `end`, `status`, `subject`, `location`, `isPrivate`), `workingHours`, `error` match the resource keys; every pointer getter nil-guarded; `resp == nil` guarded.
* Per-mailbox error surfaced rather than failing the call, matching the doc; `FreeBusyStatus` enum matches; single collection, no `nextLink`; request order preserved.

### calendar.list_event_attachments and calendar.get_event_attachment

* GET `/me/events/{id}/attachments` and `/me/events/{id}/attachments/{id}`; no body.
* `$select` wire form joined with commas; fields `id`, `name`, `contentType`, `size`, `isInline`, `lastModifiedDateTime` all exist on `attachment`; the test asserts `contentBytes` is excluded from the list call.
* `Calendars.ReadWrite` is a superset of the documented `Calendars.Read`.
* `contentType` null for an `itemAttachment` is guarded; `size` is `*int32` widened to int64; polymorphic dispatch covers the three documented subtypes.
* A non-transient Graph error is returned once without retry and redacted; no `$filter`, `$search`, `$expand`, `$orderby` sent.

### calendar.add_event_attachment

* Direct path: POST `/me/events/{id}/attachments`; body `@odata.type`, `name`, `contentType`, `contentBytes`; `id` read nil-guarded.
* Session path: POST `/me/events/{id}/attachments/createUploadSession`; `AttachmentItem` with `attachmentType: file`, `name`, `size`, `contentType`.
* `uploadUrl` guarded; chunk PUT headers and order as for mail; 201 with `Location` is read (the id extraction is B1); non-2xx is an error; incomplete transfer is a failure.
* Upload URL redacted; `Calendars.ReadWrite` is the documented least privilege for both calls.
* Pre-flight GET `/me/events/{id}?$select=subject` is a supported select; failure refuses before any bytes move.
* Annotations non-read-only, non-destructive, non-idempotent, open-world; no pagination.

### contacts.search

* GET `/me/contacts` and GET `/me/people`; `Contacts.Read` and `People.Read` are the documented least privileges.
* `$search` enclosed in exactly one pair of double quotes, matching the people form; both requests carry the identical normalised value.
* Only `Accept: application/json`; neither doc requires `ConsistencyLevel` or `X-PeopleQuery-QuerySources` for mailbox-scoped search.
* Response field names (`displayName`, `emailAddresses[]`, phones, addresses, `scoredEmailAddresses[]`, `personType`, `isFavorite`, `userPrincipalName`) match the resource tables; every pointer nil-guarded.
* People relevance order preserved; empty query rejected before any call; timeout and Graph errors return no partial results.

### contacts.get_contact, contacts.get_person, contacts.list_people

* `get_contact`: GET `/me/contacts/{id}`; `Contacts.Read`; only `$expand` and `$select` offered.
* `list_people`: GET `/me/people`; `People.Read`; no body.
* Wire field names across `contact` and `person` match the documented casing; every documented-nullable string passes through `graph.SafeStr`; nested objects nil-guarded; a nil collection yields an empty list.
* Relevance order preserved; identifiers validated before any call; phone type rendered from the SDK enum.

### teams.search

* POST `/search/query`; body keys `requests`, `entityTypes`, `query`, `queryString` match the doc; `EntityType` serializes to `chatMessage`.
* `queryString` always set; empty query refused before any call; exactly one `searchRequest`; `entityTypes` is `chatMessage` alone, the only permitted combination.
* No `sortProperties`, `aggregations`, `collapseProperties`, `xrank`, `contentSources`, `enableTopResults` (message-only).
* `Chat.Read` and `ChannelMessage.Read.All` requested, as `search-api-overview` lists for `chatMessage`.
* Chat vs channel classification on `channelIdentity`; `resp`, each `searchResponse`, `hitsContainer` and `hit` nil-checked; nesting `value[].hitsContainers[].hits[]` matches the SDK deserializers; empty response yields `[]`.
* Graph errors routed through redaction with a consent fix; timeouts separated.

### teams.list_chats, teams.list_chat_messages, teams.get_chat_message

* GET `/me/chats`, `/me/chats/{id}/messages`, `/me/chats/{id}/messages/{id}` match the docs; `Chat.Read` accepted or least privileged for each.
* `get_chat_message` sends no OData parameters, as the doc requires, and issues one request for every tier because the full body is always returned.
* `chatType` enum values match; every `chatMessage` key read by the serializer exists with matching casing; every nullable field nil-guarded.
* `chat_id` and `message_id` validated before any request; timeout and redaction uniform.

### teams.list_channel_messages, teams.get_channel_message, teams.list_channel_message_replies, teams.compose_reply

* Paths match `channel-list-messages`, `chatmessage-get` (no OData parameters sent), `chatmessage-list-replies`.
* `compose_reply` chat path GET `/me/chats/{id}/messages/{id}` is documented and needs `Chat.Read`, which is requested.
* `ChannelMessage.Read.All` is the documented least privilege and `auth.go:60` requests exactly it; fix strings name it correctly.
* No `$filter`, `$search`, `$orderby`, `$select`, `$expand` or body on any of the four calls.
* `channelIdentity`, `from`, `body`, `subject`, `webUrl`, `replyToId`, `lastEditedDateTime`, `deletedDateTime` nil-guarded; wire keys read through typed getters.
* `compose_reply` issues one GET and no write; identifiers validated before the call.

### teams.get_online_meeting, teams.list_transcripts, teams.get_transcript

* GET `/me/onlineMeetings/{id}` and the filtered form `?$filter=joinWebUrl eq '...'` match the doc; the handler reads the first value and treats an empty collection as a stated absence.
* `OnlineMeetings.Read` and `OnlineMeetingTranscript.Read.All` are the documented least privileges.
* GET `.../transcripts`, `.../transcripts/{id}` and `/content` match the doc; `Content().Get` returns `[]byte`.
* `callTranscript` keys and `onlineMeeting` keys match the wire; `meetingOrganizer.user.id` nil-guarded on both levels.
* Single quote in the join URL doubled per OData rules; no `$search`, `$expand`, `$orderby`, `$count`.
* Exactly one of `meeting_id` and `join_web_url` enforced; `join_web_url` must be absolute https and at most 2048 bytes.
* `get_transcript` states the full content requires `output=raw`; preview truncates on rune boundaries.
* Timeouts and transient retry applied uniformly.

### Scope set (`internal/auth/auth.go`)

* `Mail.Read` is sufficient for every mail read; no write verb is registered without `MailManageEnabled`.
* `Mail.ReadWrite` is the documented least privilege for `createReply`, `createReplyAll`, `createForward`, message POST/PATCH/DELETE, move and attachment POST.
* `Mail.Send` is never requested and no send endpoint is called.
* `Calendars.ReadWrite` is the documented least privilege for event accept/decline/tentativelyAccept/cancel, event POST/PATCH/DELETE and event attachment POST, and covers `getSchedule`.
* `Contacts.Read`, `People.Read`, `Chat.Read`, `ChannelMessage.Read.All`, `OnlineMeetings.Read`, `OnlineMeetingTranscript.Read.All` match the documented least privileges for their calls; `$search` is documented on `/me/people`.
* `Scopes()` excludes `ChatMessage.Send`, `Chat.ReadWrite`, `ChannelMessage.Send`, `Group.ReadWrite` and `Contacts.ReadWrite` as its docstring claims; gated scopes are appended only under their flags.
* MSAL appends `openid`, `offline_access` and `profile`, so a refresh token is obtained without listing `offline_access`.

## Unreachable docs

* Graph known issue 13644 (large attachments in shared or delegated mailboxes), referenced by the large-attachments pages at `developer.microsoft.com/en-us/graph/known-issues`, was not fetched; its effect on the attachment handlers is unknown.
* `attachment-createuploadsession` API reference was not fetched by one verifier; the how-to page and the SDK doc comment were used for the body contract instead. Another verifier fetched it and found the "between 3 MB and 150 MB" text.
* `https://learn.microsoft.com/en-us/graph/people-example` returned HTTP 404; the fuzzy-search examples were taken from `people-insights-overview` and `search-query-parameter`.
* `https://learn.microsoft.com/en-us/graph/api/person-get?view=graph-rest-1.0` returned HTTP 404 (also at `view=graph-rest-beta`). The single-person GET contract for `/me/people/{id}` could not be verified against any reference page (see R11).
* `https://learn.microsoft.com/en-us/graph/api/resources/searchhitscontainer?view=graph-rest-1.0` was not fetched; `moreResultsAvailable` and `total` were read from `search-concept-chat-messages` and `search-query` instead.
