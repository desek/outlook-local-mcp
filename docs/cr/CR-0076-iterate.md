---
name: iterate-ledger
description: Session ledger for a last-mile iteration session against an implemented Change Request. Records every attempt, its verification evidence, and its disposition, then distils the session into recommended patterns and anti-patterns at close.
cr: "CR-0076"
status: "open"
opened: "2026-08-02"
closed: ""
source-branch: "fix/cr-0076-search-kql-quoting"
source-commit: "4c93ef1"
worktree: "/Users/desek/Repo/desek/outlook-local-mcp"
---

# Iteration Session Ledger: closing the last mile on KQL query normalisation

## Session Context

* **Governing Change Request:** CR-0076, which corrected the quoting of
  `mail.search_messages`. Before it, the caller's query reached Microsoft Graph without the
  enclosing double quotes Graph requires, so every documented property keyword failed and an
  unquoted multi-word query silently returned the newest messages instead of matches. The
  change added a pure normalisation function, wired it into both request paths, translated
  the documented double-quoted phrase form into the parenthesised form Graph accepts,
  corrected the live verb description, deleted a dead constructor whose lying description
  had misdirected the change request's own first draft, and bound the documented examples to
  the behaviour with a registry-derived test.

* **Gap being closed:** not yet named. The session was opened after the change was
  implemented, finalized, validated, and verified against a live mailbox, so no last-mile
  gap has been identified by the change owner at the time of opening. The change owner names
  the first attempt.

* **Starting point:** branch `fix/cr-0076-search-kql-quoting` at commit `4c93ef1`, working
  tree `/Users/desek/Repo/desek/outlook-local-mcp`.

### State at open

Recorded so a later agent recovering this session does not re-derive it.

**Verified.** The measured rows were re-run against a restarted server carrying the fix and
all pass. `subject:Contoso` returns matches where it previously failed at position 7. An
unquoted multi-word query returns zero rather than the newest mail. The documented phrase
form is translated and scopes correctly: the two-token subject query returns three messages
while the variant whose second token appears only in message bodies returns none, which is
what separates a translation that scopes to the property from one that leaks. A stray quote
is refused before any Graph call, with an error naming the parenthesised form.

**Known outstanding at open**, none of which the change owner has yet called a gap:

1. The lifecycle harness run, `make crud-test`, exercising the new steps 30f and 30g. This
   is the one remaining acceptance item from the change request and is recorded as GAP-2 in
   the validation report. It is a paid run.
2. `site/src/components/svg/CapabilityMailSearch.tsx` renders a query in the old
   double-quoted property form. It is not broken for a caller, because the normaliser
   rewrites that form, but the site teaches the syntax this change removed. Its text is a
   rendered SVG label, so the site's own conventions require screenshot comparison for any
   edit.
3. Two local files carry real correspondent names from the evidence-gathering: the server
   log and four harness transcripts under `docs/bench/runs/`. Both are gitignored and were
   never committed. The repository history was scrubbed and verified clean.

**Topology corrected at open.** The eleven implementation commits were found on local
`main`, which the project prohibits. They were moved to the feature branch named above and
`main` was returned to `origin/main`. No commit was lost. The pre-scrub state remains at
`backup/pre-scrub-rewrite`.

**Checks for this session** are the project's own: `make ci`, and `go test -race` on the
touched packages. The skill's generic `bats -r tests/` does not apply to this repository.

## Attempt Ledger

No attempts recorded yet. The change owner names the first one.

## Distillation

### Recommended Patterns

<!-- Populated at close from the settled entries. -->

### Anti-Patterns

<!-- Populated at close from the discarded entries, with the reason each failed. -->
