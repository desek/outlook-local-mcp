---
name: iterate-ledger
description: Session ledger for a last-mile iteration session against an implemented Change Request. Records every attempt, its verification evidence, and its disposition, then distils the session into recommended patterns and anti-patterns at close.
cr: "CR-0073"
status: "closed"
opened: "2026-08-02"
closed: "2026-08-02"
source-branch: "feat/cr-0073-surface-manifest"
source-commit: "64ef7e1"
worktree: "/Users/desek/Repo/desek/outlook-local-mcp"
---

# Iteration Session Ledger: the site describes the interface, not the outcome

## Session Context

* **Governing Change Request:** CR-0073 — the site's figures are generated from a
  surface manifest, so the published page can no longer state a tool count the code
  has moved past.
* **Gap being closed:** the correction made every figure true, and left the page
  speaking in the vocabulary of the implementation. A visitor reads "4 MCP tools, 42
  verbs, 33 by default" and learns the shape of the interface rather than what they
  can now do. The counts were the defect the CR fixed; they are not the value the
  page exists to communicate.
* **Starting point:** branch `feat/cr-0073-surface-manifest` at commit `64ef7e1`,
  working tree `/Users/desek/Repo/desek/outlook-local-mcp`

## Attempt Ledger

### Attempt 1 — outcome-first copy on the surfaces that lead with a count

* **State:** settled
* **Hypothesis:** the surfaces a visitor meets first should answer "what can I do
  with this" rather than "how big is it". Replacing the count-led wording in the
  hero stat, the capabilities lead, the tools-reference entry point, the meta
  description, and the JSON-LD feature list with outcome-led wording makes the page
  communicate value, while the counts stay available where a reader has already
  decided to look at the reference. Nothing regresses to a typed figure: any figure
  that survives is still read from the manifest.
* **Surface touched:** `site/src/components/HeroSection.tsx`,
  `site/src/components/CapabilitiesSection.tsx`,
  `site/src/components/ToolsReferenceSection.tsx`, `site/build/seo.pages.ts`,
  `site/build/seo.jsonld.ts`, and the landing-page text floor in
  `.agents/scripts/site.content.check.mjs` if the length moves.
* **Verification evidence:** the site build exits 0, and the content check holds every
  assertion, including the claims assertion, so no typed figure returned. The landing
  page measured 12,002 characters against the 11,942 floor, which is 60 more prose than
  the figures it replaced, so the floor was raised to 12,002 with its attribution. The
  public preview at `preview.outlook-local-mcp.com` serves `CALENDAR + MAIL`, "Ask for
  your week", and "Everything It Can Do", and no longer serves "42 verbs" in the landing
  copy. Two builds failed first on unused imports, once in the components and once in
  `seo.jsonld.ts`, which is the compiler reporting exactly the thing this attempt set out
  to do: the entry surfaces no longer consume the counts. The imports were narrowed to
  what each file still uses. `site/build/` is outside `tsconfig.app.json`'s `include`, so
  its unused import was found by review rather than by the typecheck, which is worth
  knowing before trusting a green build there.
* **Disposition:** kept

## Distillation

### Recommended Patterns

* **A correction that makes every figure true has not made the page useful.** The
  governing change removed a class of defect: the site could no longer state a count
  the code had moved past. It left the page speaking in the vocabulary of the
  implementation, which is a different failure that the first one hides, because
  every sentence is now defensible. When a change replaces wrong facts with right
  ones, ask separately whether the right ones are the facts a reader came for.

* **Put the figure where the reader has asked for it.** Counts were not deleted, they
  moved behind the expanded reference. A visitor meeting the page is told what they
  can ask for; a reader who opened the reference is told how large it is. Both
  audiences are served by the same generated source, and neither is served by leading
  with the other's answer.

* **An unused import is a design signal, not a build error to clear.** Both build
  failures in this session named the imports the entry surfaces had stopped using.
  That is the type system reporting that consumption moved exactly where the change
  intended it to move. Read the list before deleting it: had a file still needed a
  count that the copy no longer showed, the compiler would have stayed silent and the
  inconsistency would have shipped.

* **Know which directories the typecheck actually covers.** `site/build/` sits outside
  `tsconfig.app.json`'s `include`, so its unused import was found by reading rather
  than by `tsc`. A green build is evidence about the directories the configuration
  names, and about no others. Confirm the coverage before treating a passing build as
  a statement about a file.

### Anti-Patterns

* **No attempt in this session was discarded**, so this list carries no rejected
  approach. Recording that plainly is more honest than promoting a hazard the session
  merely noticed into a failure it never had. The one near-miss is written above as a
  pattern rather than invented as an anti-pattern: trusting the site build to police
  `site/build/` would have shipped an unused import, but that trust was never acted
  on, so it did not fail here.
