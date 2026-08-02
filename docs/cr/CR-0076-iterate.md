---
name: iterate-ledger
description: Session ledger for a last-mile iteration session against an implemented Change Request. Records every attempt, its verification evidence, and its disposition, then distils the session into recommended patterns and anti-patterns at close.
cr: "CR-0076"
status: "closed"
opened: "2026-08-02"
closed: "2026-08-02"
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

* **Gap being closed:** unnamed at open, and settled at close as the outstanding acceptance
  item rather than a behaviour gap. The session was opened after the change was implemented,
  finalized, validated, and verified against a live mailbox, so nothing about the delivered
  behaviour had been judged wrong. What remained was the lifecycle harness run, GAP-2 of the
  validation report, which is what attempt 1 executed. This field is updated at close; it
  read "not yet named" while the session was open, because the change owner had not yet
  named one and the agent must not invent a gap on their behalf.

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

### Attempt 1 — run the lifecycle harness against the corrected search verb

* **State:** settled
* **Hypothesis:** the change request's last outstanding acceptance item, recorded as GAP-2
  in the validation report, is a `make crud-test` run exercising the two harness steps phase
  4 added: 30f, a property-restricted search, and 30g, a parenthesised phrase search paired
  with a two-nonsense-word control that must return zero rather than the newest mail. The
  hypothesis is that both pass against the corrected verb. The control step is the
  interesting one, because before this change it would have returned recent messages and
  looked like a pass to a careless reader.
* **Surface touched:** no source change. The harness drives the built binary named in
  `.mcp.json`, so the binary is rebuilt to that path first. This is a standing project rule:
  the harness tests whatever was last compiled there, not the working tree, and a run has
  previously reported findings against a commit days older than the tree before anyone
  noticed. The report's own `Server version` line is checked against `HEAD` before any row
  of it is trusted.
* **Verification evidence:** the binary was rebuilt to the configured path at `aee5254` and
  the report's own banner read `dev (commit aee5254)`, matching `HEAD`, so the report
  describes the code under test rather than an older build.

  Result: 53 PASS, 0 FAIL, run `2026-08-02T17-17-56`.

  Step 30f passed: the property-restricted `from:` query scoped correctly with no parse
  error and every result came from the named sender. Step 30g passed on both halves: the
  parenthesised subject query returned zero with no error, and the two-nonsense-word control
  returned zero rather than recent mail.

  The control is the load-bearing half. Before this change it would have returned the newest
  messages in the mailbox and a reader skimming for an error would have called it a pass.

  Four findings, none a defect in this change: `system.about` returns identical `raw` and
  `summary` payloads; a timezone rendering inconsistency between verbs with no functional
  mismatch on the test date; the step 30e baseline reflecting the `max_results` cap, which
  the prompt already documents as expected; and the server log spanning several sessions,
  which the driving agent handled by scoping its check to this run's window. The first two
  are pre-existing and unrelated to the search verb.

  Metrics against the prior three runs. Wall time 229.59 s against a prior mean of 229.84 s
  with a 38.5 s spread, so effectively unmoved. Cost $3.19 against a prior range of $2.63 to
  $3.03, therefore above the prior maximum. Turns 69 against 65 to 68 and mail calls 22
  against 19 to 21, both consistent with two steps having been added. No claim of "no cost
  regression" is recorded here: three prior runs do not characterise a noise band well
  enough to support one, and the honest reading is that cost rose by about what two extra
  steps would cost.

  This run covers GAP-2 of the validation report, which was the change request's last
  outstanding acceptance item.
* **Disposition:** kept

## Distillation

One attempt, settled as kept. The session was short because the change request had already
been validated and verified live before the session opened, so the only outstanding item was
the harness run.

### Recommended Patterns

* **Pair a positive assertion with a negative control whose expected result is empty, when
  the defect's failure mode is plausible wrong data rather than an error.** The search defect
  had two failure modes. One was loud, a parse error, and any positive test would have caught
  its repair. The other was silent: an unquoted multi-word query returned the newest messages
  in the mailbox, which looks exactly like a successful search. A positive assertion cannot
  distinguish a working search from that, because both return messages. Only a query whose
  correct answer is zero can, and it must use terms guaranteed absent from the data. This is
  what step 30g's two-nonsense-word control does, and it is why the harness run is evidence
  rather than ceremony.

* **Check a measuring instrument's identity against the thing it claims to measure, before
  reading a single result from it.** The report prints a version banner and this run's read
  `aee5254`, matching `HEAD`. That check costs one command and is the difference between a
  report about the current code and a report about whatever was last compiled to that path.
  It is already a standing project rule and it earned its place again here.

* **Refuse the tempting summary claim when the sample cannot support it.** Cost rose above
  the prior maximum while two steps were added to the harness. "No regression" was available
  and would have read well. Three prior runs do not characterise a noise band, so the entry
  records the numbers, the added steps, and the absence of a conclusion instead.

### Anti-Patterns

No attempt in this session was discarded, so this list is empty. It is recorded as empty
rather than left blank, because an empty anti-pattern list means one attempt was tried and
worked, not that the section was skipped. The two designs this change request falsified on
its way to the delivered behaviour are recorded in the change request's own Alternative
Approaches section, not here, because they were eliminated before implementation rather than
during this session.
