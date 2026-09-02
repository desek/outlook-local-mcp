---
date: 2026-09-02
source: cr (CR-0083)
surface: cli (MCP stdio client driving the built ./outlook-local-mcp binary)
outcome: not-run
runs: "0 of 0 attempted; derived from the CR, not yet run against a live tenant"
coverage_gap: false
---

# Finding what was said in Teams and reading the transcript of the meeting it came from

## Status

**Derived, not yet run.** This artifact is the executable form of the acceptance
requirement, written so the run can be performed without re-deriving it. No run has
happened: the scenario needs a live authenticated tenant with Teams conversations, a
recorded meeting with a transcript, and an interactively granted keychain entry, and none
was available to the session that wrote it. The `outcome` and `runs` frontmatter fields say
so, and they MUST be rewritten by whoever performs the first run rather than left as they
are.

Nothing below has been observed. Every unit test covering these thirteen verbs drives an
`httptest` server and grades the outgoing request, so **no call has ever reached Microsoft
Graph on any Teams collection.** Four things are unconfirmed, and each of them is a way the
domain could be correct in tests and useless against the service:

**Search hits are assumed to carry the identifiers the rest of the domain spends.** The
domain deliberately offers no team or channel enumeration: a channel is reached only by
searching for a message in it, and every channel read is addressed by a team identifier and
a channel identifier together. Both are read out of a search hit's `channelIdentity`. If the
live service omits either field on a chat-message hit from a channel, the channel half of
the domain is unreachable by the only route it offers, and that is a design question routed
to a follow-on change request rather than a patch.

**The transcript chain is assumed to resolve.** A calendar event carries a join URL, not the
meeting-scoped identifier a transcript is keyed by, so `get_online_meeting` filters
`/me/onlineMeetings` by `joinWebUrl eq '...'`. Whether the service accepts that filter for a
meeting the account merely attended, rather than organised, is unconfirmed and is the
single most likely failure.

**`OnlineMeetingTranscript.Read.All` is assumed sufficient as a delegated scope.** It is
requested delegated, and transcript access is tenant-policy dependent in ways a scope grant
does not by itself settle.

**The consent prompt is assumed to name four scopes and no send scope.** The server requests
no Teams write scope in any configuration, and the prompt is where a user can observe that.

## Goal

A person half-remembers something said in Teams, wants the thread it came from, and then
wants the transcript of the meeting that thread was about. They search, walk from a hit to
its thread and its replies, draft an answer they will post themselves, resolve the meeting
behind the conversation, and read its transcript.

The surface is the MCP server as an assistant drives it, over stdio, against the built
binary, with the `teams` aggregate tool and its `operation` verb dispatch. It is not the
handler constructors in `internal/tools/`.

## Preconditions

* An authenticated account is present in the token cache and resolves as the default
  account. Every verb here is a read; nothing in this scenario changes anything in Teams.
* `OUTLOOK_MCP_TEAMS_ENABLED=true` is set for the server under test. With it unset the
  `teams` tool is not registered at all, which is a separate assertion graded at step 1.
* Consent was granted **after** the flag was set, so `Chat.Read`, `ChannelMessage.Read.All`,
  `OnlineMeetings.Read`, and `OnlineMeetingTranscript.Read.All` are on the token. A token
  minted before the flag was set carries none of them, and every verb fails with an
  authorisation error that looks like a service fault and is not one.
* The tenant holds at least one one-to-one or group chat with several messages, at least one
  of them carrying replies; at least one channel post the account can read, discoverable by
  search; and at least one past meeting the account can reach that has a transcript.
* The binary under test is the one `.mcp.json` names, rebuilt from the current tree, and it
  has been driven interactively once so the keychain grant has been answered. A locally
  driven server that hangs on the first authenticated call after a clean `initialize` is the
  ad-hoc code-signing symptom, not a network fault.

## Steps at the user surface

Capitalised names are values carried from an earlier step, not literals.

1. List the tools with `OUTLOOK_MCP_TEAMS_ENABLED` unset, then again with it set.
2. `teams` with `operation="help"`.
3. `teams` with `operation="search"` and a `query` matching something known to have been
   said, at the default output tier. Take CHATID and CHATMSG from a chat hit, and TEAMID,
   CHANNELID, and POSTID from a channel hit.
4. `teams` with `operation="list_chats"`.
5. `teams` with `operation="list_chat_messages"`, `chat_id=CHATID`.
6. `teams` with `operation="get_chat_message"`, `chat_id=CHATID`, `message_id=CHATMSG`, at
   the default tier, then again with `output="raw"`.
7. `teams` with `operation="list_chat_message_replies"`, `chat_id=CHATID`,
   `message_id=CHATMSG`.
8. `teams` with `operation="list_channel_messages"`, `team_id=TEAMID`,
   `channel_id=CHANNELID`.
9. `teams` with `operation="get_channel_message"`, `team_id=TEAMID`, `channel_id=CHANNELID`,
   `message_id=POSTID`, at the default tier, then again with `output="raw"`.
10. `teams` with `operation="list_channel_message_replies"`, the same three identifiers.
11. `teams` with `operation="compose_reply"`, `chat_id=CHATID`, `message_id=CHATMSG`, and a
    short `body`. Then open Teams and confirm the chat is unchanged.
12. `teams` with `operation="get_online_meeting"`, `join_web_url` taken from a calendar event
    for the meeting. Take MEETINGID from the result.
13. `teams` with `operation="list_transcripts"`, `meeting_id=MEETINGID`. Take TRANSCRIPTID.
14. `teams` with `operation="get_transcript"`, `meeting_id=MEETINGID`,
    `transcript_id=TRANSCRIPTID`, at the default tier, then again with `output="raw"`.
15. `teams` with `operation="get_online_meeting"` and neither identifier, then with both.
16. `teams` with `operation="get_channel_message"` and `team_id` omitted.

## Success condition

Graded on what the live service returns, never on the wording of a verb description or on
what the unit tests already assert against a canned response.

* Step 1 lists no `teams` entry with the flag unset and one with it set. A `teams` tool
  present in the first listing falsifies the gating guarantee the domain rests on.
* Step 2 names exactly the thirteen verbs and no verb that sends, posts, edits, or deletes.
* **Step 3 returns hits that carry the identifiers the later steps spend.** A chat hit
  carries a chat identifier and a message identifier; a channel hit carries a team
  identifier, a channel identifier, and a message identifier. A hit missing any of them is
  the reproduction this scenario is looking for: record which field was absent on which hit
  shape, mark the outcome `reproduced`, and open a follow-on change request. Do not patch the
  verb from this run.
* Steps 4, 5, 7, 8, and 10 return the conversations, messages, and replies visible in Teams
  for the same identifiers. A thread whose replies are visible in Teams and empty here is a
  finding.
* Steps 6 and 9 return a preview at the default tier and the whole body under `output="raw"`,
  and the two differ for a message long enough to be truncated. A raw tier identical to the
  default means the escalation the description promises buys nothing.
* **Step 11 returns prepared text and changes nothing.** The text quotes the parent and states
  that it was not sent. Teams shows no new message in the chat. A message appearing in Teams
  is the most serious possible failure of this domain and stops the run.
* **Step 12 resolves the join URL to a meeting identifier.** An empty match, or a service
  error naming the `joinWebUrl` filter, is the second reproduction this scenario is looking
  for. Record whether the account organised or merely attended the meeting, because that
  distinction is the likely cause and is not recoverable afterwards.
* Step 13 lists the transcripts of that meeting and no other. Step 14 returns metadata with
  no transcript content at the default tier and the WEBVTT content under `output="raw"`.
  Content present at the default tier is a finding: the tier boundary is what keeps a
  transcript out of a context window that did not ask for it.
* Step 15 fails both times, naming the two identifiers and stating that exactly one is
  required, and the neither-identifier refusal names the calendar verb that supplies a join
  URL. Step 16 fails naming `team_id` and stating that a search hit supplies it. All three
  refusals issue no request; latency indistinguishable from a successful call is evidence
  one was issued.

Re-run steps 3 and 12 a second time and record whether both attempts agree. Search is a
relevance-scored resource and may legitimately return a different order; a meeting
resolution that moves is a finding.

## Restoration

None. Every verb in this scenario is a read, the domain registers no write verb, and no
Teams send or write scope is on the token, so the tenant is in the state it started in.
Unset `OUTLOOK_MCP_TEAMS_ENABLED` afterwards if the machine's default server should stay on
its default surface. The consent granted to the four read scopes persists on the token and
is revoked through the account domain's logout verb, not by unsetting the flag.

## Relationship to the paid CRUD harness

`make crud-test` Steps 52 to 58 drive these same thirteen verbs and are also unrun. They are
a separate, paid instrument with a separate purpose: they grade the harness's own
accounting, including the `mcp_teams` column added to `docs/bench/crud-runs.csv`, and they
skip themselves when `config.features.teams_enabled` reads false in the status output. Run
this scenario first. It costs a handful of authenticated calls to settle the two identifier
questions above, and a harness run that fails on either spends a great deal more to report
the same thing.

Whoever runs the harness must rebuild to the path `.mcp.json` names first, per the
precondition above, and must read the report's own `Server version` line before trusting a
row of it.

## Run log

No runs. The first run appends its record here: the date, the driver invocation, the value
observed for each success-condition bullet, and how many attempts of how many passed.
