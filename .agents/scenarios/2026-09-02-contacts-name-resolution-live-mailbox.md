---
date: 2026-09-02
source: cr (CR-0082)
surface: cli (MCP stdio client driving the built ./outlook-local-mcp binary)
outcome: not-run
runs: "0 of 0 attempted; derived from the CR, not yet run against a live mailbox"
coverage_gap: false
---

# Resolving a name to an address across saved contacts and ranked people

## Status

**Derived, not yet run.** This artifact is the executable form of the acceptance
requirement, written so the run can be performed without re-deriving it. No run has
happened: the scenario needs a live authenticated mailbox with saved contacts and an
interactively granted keychain entry, and neither was available to the session that wrote
it. The `outcome` and `runs` frontmatter fields say so, and they MUST be rewritten by
whoever performs the first run rather than left as they are.

Nothing below has been observed. Every unit test covering these five verbs drives an
`httptest` server and grades the outgoing request, so **no call has ever reached Microsoft
Graph on either contacts collection.** The load-bearing unconfirmed fact is one the change
request named and deliberately routed to a follow-on change rather than an in-flight
redesign:

**`$search` on `/me/contacts` is proved only at the SDK layer.** The generated Kiota query
struct exposes a `Search` field on every collection uniformly, so its presence is not
evidence that Graph v1.0 honours `$search` for the personal-contacts collection, and this
path sets no `ConsistencyLevel: eventual` header. If the service rejects the parameter, the
saved-contacts half of `search` fails while the ranked-people half would have succeeded,
which under the verb's all-or-nothing failure rule fails the whole call. That is the single
outcome this scenario exists to settle. **If it reproduces, open a follow-on change request;
do not patch the verb from this run.**

Two further things are unconfirmed: whether the relevance order `/me/people` returns is the
order a caller actually sees end to end, and whether a `person` identifier returned by
`list_people` resolves through `get_person` unchanged.

## Goal

A person remembers a colleague only by name and wants the address to write to. They ask for
the name, weigh a saved contact against a person Graph merely ranks as relevant, then open
the one record they chose to read every address it holds.

The surface is the MCP server as an assistant drives it, over stdio, against the built
binary, with the `contacts` aggregate tool and its `operation` verb dispatch. It is not the
handler constructors in `internal/tools/`.

## Preconditions

* An authenticated account is present in the token cache and resolves as the default
  account. Every verb here is a read; nothing in this scenario mutates the mailbox.
* `OUTLOOK_MCP_CONTACTS_ENABLED=true` is set for the server under test. With it unset the
  `contacts` tool is not registered at all, which is a separate assertion graded at step 1.
* Consent was granted **after** the flag was set, so `Contacts.Read` and `People.Read` are
  on the token. A token minted before the flag was set carries neither scope, and every verb
  fails with an authorisation error that looks like a service fault and is not one.
* The mailbox holds at least two saved contacts whose display names share a leading
  substring, at least one of them carrying two or more email addresses, and the person has
  corresponded with at least two people who are not saved contacts. Record the shared
  substring as `QUERY`; a single-word term of four or more characters is intended, since a
  short common term matches a large part of the mailbox and the verb's own timeout
  correction says so.
* The binary under test is rebuilt at the path `.mcp.json` names before the run, and its
  reported commit matches `git rev-parse --short HEAD`. A run against a stale binary
  produces a report that looks authoritative and describes a different commit:

  ```bash
  go build -ldflags="-X main.commit=$(git rev-parse --short HEAD) -X main.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o ./outlook-local-mcp ./cmd/outlook-local-mcp
  ```

* On macOS, the binary is signed with a stable identity and the keychain prompt is answered
  interactively once before any headless run. An ad-hoc-signed binary blocks on an invisible
  prompt at the first authenticated call, which reads as a timeout.

## Steps at the user surface

Driven as MCP `tools/call` requests against the built server over stdio. Record the value
each step reports; several later steps consume them.

1. `tools/list` against a server started with the contacts flag **unset**. Then restart with
   `OUTLOOK_MCP_CONTACTS_ENABLED=true` and repeat. Every step below runs against the second
   server.
2. `contacts` with `operation="help"`. Confirm the five verbs and their read-only semantics
   are documented where the registry says they are.
3. `contacts` with `operation="search"`, `query=QUERY`. **This is the step the scenario
   exists for**: it is the only one that puts `$search` on `/me/contacts` in front of the
   live service. Record the identifier of one saved-contact match as `CONTACT` and one
   ranked-person match as `PERSON`.
4. `contacts` with `operation="search"`, `query=QUERY`, `output="raw"`.
5. `contacts` with `operation="get_contact"`, `contact_id=CONTACT`.
6. `contacts` with `operation="get_contact"`, `contact_id=CONTACT`, `output="raw"`.
7. `contacts` with `operation="list_people"`.
8. `contacts` with `operation="get_person"`, `person_id=PERSON`.
9. `contacts` with `operation="search"` and no `query` argument.
10. `contacts` with `operation="get_contact"`, `contact_id="not-a-real-identifier"`.

## Success condition

Graded on what the live service returns, never on the wording of a verb description or on
what the unit tests already assert against a canned response.

* Step 1 lists four tools with the flag unset and no `contacts` entry, and five with it set.
  A `contacts` tool present in the first listing falsifies the whole gating guarantee.
* Step 2 names exactly `help`, `search`, `get_contact`, `list_people`, `get_person`, and no
  verb that writes a contact, folder, or photo.
* **Step 3 returns matches rather than an error.** A Graph error naming `$search`, the query
  parameter, or a required `ConsistencyLevel` header is the reproduction this scenario is
  looking for: record the verbatim error, mark the outcome `reproduced`, and open a follow-on
  change request. If it succeeds, the matches include at least one labelled as a saved
  contact and at least one labelled as a ranked person, and the saved contacts precede the
  ranked people.
* Step 4 returns the same match count as step 3, with each saved contact carrying the
  detail the summary omits and each ranked person carrying its scored addresses. A raw tier
  identical to the summary tier means the escalation buys nothing and is a finding.
* Step 5 resolves `CONTACT` in one call and renders every address the contact holds, not
  only the first. An address visible in Outlook but absent here is the failure the verb's
  every-address requirement exists to prevent.
* Step 6 returns the phone numbers and postal addresses the raw tier promises.
* Step 7 returns the ranked people in Graph's own relevance order, and that order is
  observably not alphabetical and not the order of step 3's people half if the two differ.
  A re-sorted list discards the ranking, which is the whole value of the resource.
* Step 8 resolves `PERSON` unchanged from the identifier step 3 reported, and labels each
  address with the person's own display name.
* Step 9 fails naming `query`, states the correction, and issues no request. Latency
  indistinguishable from step 3's is evidence it did call the service.
* Step 10 fails naming the identifier and stating where one comes from.

Re-run steps 3 and 7 a second time and record whether both attempts agree. Ranked people is
a relevance-scored resource and may legitimately move between calls; a saved-contacts result
that moves is a finding.

## Restoration

None. Every verb in this scenario is a read and the domain registers no write verb, so the
mailbox is in the state it started in. Unset `OUTLOOK_MCP_CONTACTS_ENABLED` afterwards if
the machine's default server should stay on the four-tool surface. The consent granted to
`Contacts.Read` and `People.Read` persists on the token and is revoked through the account
domain's logout verb, not by unsetting the flag.

## Relationship to the paid CRUD harness

`make crud-test` Steps 47 to 51 drive these same five verbs and are also unrun. They are a
separate, paid instrument with a separate purpose: they grade the harness's own accounting,
including the `mcp_contacts` column added to `docs/bench/crud-runs.csv`, and they skip
themselves when contacts are disabled. Run this scenario first. It costs one authenticated
call to settle the `$search` question, and a harness run that fails on that question spends
a great deal more to report the same thing.

Whoever runs the harness must rebuild to the path `.mcp.json` names first, per the
precondition above, and must read the report's own `Server version` line before trusting a
row of it.

## Run log

No runs. The first run appends its record here: the date, the driver invocation, the value
observed for each success-condition bullet, and how many attempts of how many passed.
