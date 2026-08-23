---
id: "CR-0077"
status: "draft"
date: 2026-08-22
requestor: desek
stakeholders:
  - desek
priority: "medium"
target-version: "0.9.0"
source-branch: dev/is-agentic-site
---

# Agent Trust Anchor Pages: About, Contact, and Privacy

## Change Summary

The website publishes a landing page and three documentation pages, but no
about, contact, or privacy page. Agents that assess a project for legitimacy read
these three pages first, so their absence lowers the project's machine-readable
trust signal even though the project is real, open source, and honest about its
data handling.

This change adds three trust anchor pages: `/about`, `/contact`, and `/privacy`.
Each page states real content, carries its own canonical URL and schema.org
structured data, and appears in the sitemap. The privacy page is the most
load-bearing: this server reads a user's mail and calendar, so a clear statement
of where data goes is a real user need, not only a scanner signal.

## Motivation and Background

An automated agent-readiness scan of `outlook-local-mcp.com` (the `is-agentic`
checker) reported that no about, contact, or privacy page carries enough content,
and that the site states no machine-readable contact channel. Those are the pages
an AI agent checks to verify a project is legitimate before it recommends it.

The scan also reported a larger group of failures that do **not** apply to this
product and are deliberately out of scope (see Non-Goals). This change request
takes only the honest, applicable subset: real pages with real content, for a real
project.

The trust question is sharper here than for a typical tool, because the server
touches mail and calendar. A user, or an agent acting for a user, is right to ask
where that data goes before connecting an account. The answer is already good, the
server runs locally and its only outbound calls are to Microsoft, but the answer is
not published anywhere a reader can find it. This change publishes it.

## Change Drivers

* The `is-agentic` scan (2026-08-22) marks the absence of about, contact, and
  privacy pages as a trust-anchor gap.
* The server reads mail and calendar, so a published privacy statement is a genuine
  user need, independent of any scanner.
* The site already has the SEO, canonical, sitemap, and JSON-LD pipeline (CR-0070),
  so adding pages that reuse it is low cost and consistent.

## Current State

The site publishes four registered pages: the landing page and `concepts.html`,
`quickstart.html`, `troubleshooting.html`, plus an unregistered `site/public/404.html`
copied verbatim by Vite (commit `da19458`). The page set is the registry in
`site/build/seo.pages.ts`, consumed by the SEO plugin, the sitemap generator, and
the JSON-LD builder. There is no about, contact, or privacy page, and the
Organization JSON-LD carries a `contactPoint` (`site/build/seo.jsonld.ts`, the
`organization` function, added by `da19458`) but no page a human or agent can open
for contact detail.

Three properties of the current build shape the work and are stated here because the
implementation must change each of them:

* Every registered page is emitted flat at the site root, so every canonical path
  except `/` carries a `.html` suffix. No page is currently emitted as a directory
  index, and no directory-index page has ever been built here.
* `site/build/seo.plugin.ts` resolves the page for a build artefact with
  `basename(ctx.path || ctx.filename || '')` and `pageForFile`, which matches
  `PageSeo.file`. The match key is therefore a bare filename, not a path.
* `site/vite.config.ts` derives each Rollup input key with `basename(p, '.html')`,
  so the input key is also a bare filename.

## Proposed Change

Add three pages to the published site, each reachable at an extensionless path so a
plain probe of `/about`, `/contact`, or `/privacy` resolves.

### Functional Requirements

* **FR-1** The site MUST publish an about page reachable at `/about`, stating what
  the project is, that it is open source under its stated licence, its relationship
  to the Microsoft Graph API, and the GigWhere acknowledgement. At least 500
  characters of prose.
* **FR-2** The site MUST publish a contact page reachable at `/contact`, naming the
  public contact channels: the GitHub repository and its issue tracker. At least 500
  characters of prose.
* **FR-3** The site MUST publish a privacy page reachable at `/privacy`, stating: the
  server runs locally on the user's machine; tokens are stored in the operating
  system keychain; the server's only outbound connections are to Microsoft Graph and
  the Microsoft identity platform, plus an optional OpenTelemetry export the user
  enables; that the one inbound socket is a temporary loopback port bound only for
  interactive browser sign-in; and what the website itself collects (nothing, no
  analytics, no cookies). At least 500 characters of prose. Every claim MUST be true
  against the server's actual behaviour.

  The loopback clause is not optional. `site/src/components/PrivacySection.tsx`
  already discloses it on the landing page, so a privacy page that omits it would be
  less complete than the section it exists to expand, which is the failure this
  requirement exists to prevent.
* **FR-4** Each new page MUST carry a canonical URL, an Open Graph and Twitter card,
  and at least one valid schema.org entity in its pre-rendered head, via the existing
  SEO pipeline. The about page MUST use `AboutPage`, the contact page MUST use
  `ContactPage`, and the privacy page MUST use `WebPage`.

  `WebPage` for the privacy page is settled, not a fallback. schema.org defines no
  `PrivacyPolicy` type, so there is no richer type to prefer and no condition under
  which the implementor should substitute one.
* **FR-5** Each new page MUST appear in `sitemap.xml` with its extensionless canonical
  URL, emitted from the same registry entry that produces its canonical tag, so the
  two can never disagree.
* **FR-6** Each new page MUST render its full prose without JavaScript, consistent
  with the site's pre-render invariant.
* **FR-6a** The landing-page footer (`site/src/components/Footer.tsx`) MUST link to
  all three new pages, so a crawler reaches each one from `/`.
* **FR-6b** The shared generated-page footer template (the `pageTemplate` function in
  `site/build/doc.pages.ts`) MUST link to all three new pages, so each new page links
  to the other two and back to `/` rather than being a crawl dead end. This footer is
  shared with `concepts`, `quickstart`, and `troubleshooting`, so those three pages
  gain the same links; that is intended, not incidental.
* **FR-7** The pages MUST state no tool-surface figure of their own; any such figure
  comes from the generated surface manifest, per the site's standing rule.
* **FR-7a** FR-7 MUST be enforced by an executable rule, not by prose alone.
  `assertNoBareClaims` in `.agents/scripts/site.content.check.mjs` currently walks only
  `.ts` and `.tsx` files (the `walkSources` extension filter), so Markdown page sources
  are unscanned and FR-7 would otherwise have no check behind it. The scan MUST be
  extended to cover the new pages' Markdown sources.
* **FR-8** The `softwareVersion` property of the landing page's `SoftwareApplication`
  entity MUST be sourced from the project's authoritative release record at build time.
  It MUST NOT be a transcribed literal, and no version literal may remain in
  `site/build/seo.jsonld.ts`.

  This is the same hand-typed-literal class CR-0073 removed from the tool-surface
  figures, and it has already failed in the same way. The literal `'0.8.0'` was written
  once, in commit `09397dd`, and has never been touched since. The latest release tag is
  `v0.5.1` and `.release-please-manifest.json` reads `0.5.1`, so the published structured
  data currently advertises a version that has never existed. A generative engine reading
  the site is being told a falsehood today, not at some future release.

  The authoritative record is `.release-please-manifest.json` (the `"."` key). It is
  tracked, it is written by release-please on every release, it matches the git tag, and
  its value is already in the bare `0.5.1` form schema.org expects, so it is used
  verbatim with no `v` prefix added and no transformation applied.

### Non-Goals

The `is-agentic` scan reported these failures. They are explicitly **out of scope**,
because the product is a local, single-binary MCP server over stdio, not a hosted
web API, and satisfying them would require publishing metadata for a service that
does not exist:

* An OpenAPI or Swagger specification, scoped OAuth scopes in an OpenAPI security
  scheme, JSON error responses, and function-calling endpoint schemas. There is no
  HTTP API to describe.
* An OAuth 2.0 authorization server and `/.well-known/oauth-authorization-server`
  metadata. The server is an OAuth client of Microsoft; it is not an authorization
  server.
* A live MCP handshake at `/.well-known/mcp`. The server speaks MCP over stdio, not
  Streamable HTTP; there is no hosted endpoint to hand back a handshake.
* A developer portal with API keys and a sandbox. There are no API keys; the product
  is a local binary.
* A `Vary: Accept` header on the Markdown-negotiated response. GitHub Pages serves
  static files and cannot set per-response headers; `/index.md` is a distinct file,
  not an `Accept`-negotiated variant.

Publishing false machine-readable metadata to raise a score is a regression in
honesty, not an improvement, and this change request does not do it.

### Acceptance Criteria

* **AC-1** A request to the extensionless paths `/about`, `/contact`, and `/privacy`
  against the built `site/dist` each returns an HTML page whose no-JavaScript
  `document.body.textContent.length` is at least 500 characters, and each page
  contains exactly one `<h1>`. Serving `site/dist` with
  `.agents/scripts/site.serve.mjs` MUST resolve the extensionless path to the
  directory index, matching GitHub Pages behaviour.
* **AC-2** Each of the three pages appears in `sitemap.xml` with its extensionless
  canonical URL, and each carries its own canonical tag naming that same URL, its own
  Open Graph and Twitter card, and the schema.org entity FR-4 assigns it. Verified by
  `.agents/scripts/site.validate.mjs` with zero real errors.
* **AC-2a** No new page carries the landing page's canonical URL or the landing page's
  `SoftwareApplication`, `FAQPage`, or `Organization` JSON-LD. This is asserted
  explicitly because the pre-change `basename` match key makes exactly that the
  default failure, and it fails silently: the pages build, render, and validate while
  every one of them claims to be `/`.
* **AC-3** The pre-rendered landing HTML links to all three new pages (FR-6a), and each
  generated page's footer links to the other two and to `/` (FR-6b), both verified
  without JavaScript.
* **AC-4** `node .agents/scripts/site.content.check.mjs` passes on the build that adds
  the pages, with a measured text floor recorded in `TEXT_FLOOR` for each new page. The
  recorded floor is the value measured on that build, which is a different and larger
  number than the 500-character minimum FR-1 to FR-3 set; the 500-character figure MUST
  NOT be used as the recorded floor.
* **AC-4a** `pnpm --dir site run lighthouse` passes with the three new pages present in
  `collect.url` in `site/lighthouserc.json` and matched by an `assertMatrix` entry that
  is theirs. The existing `.*/index\.html$` pattern matches `/about/index.html` and
  every other directory index, so leaving the matrix unchanged would grade the new
  pages against the landing page's thresholds; the entry MUST be anchored so it matches
  only the site root.
* **AC-5** Every privacy claim is traceable to the code that produces the behaviour:
  token storage to `internal/auth` (`cache_cgo.go`, `cache_backend_cgo.go`,
  `active_backend.go`); the optional OpenTelemetry export to `internal/config`
  (`OTELEnabled` and `OTELEndpoint` in `config.go`, both defaulting to off and empty);
  the Microsoft identity platform endpoint to `internal/auth/authcode.go` (the
  `authority` value); the loopback sign-in socket to the redirect URL in
  `internal/auth/auth.go`; and the Microsoft Graph service root to the
  `msgraph-sdk-go` client default, since no package in this repository sets it.

  The earlier wording cited "the Graph client" for outbound destinations. No such
  component owns that fact here: `internal/graph` holds errors, retry, serialization,
  and enum helpers, and the `GraphServiceClient` is constructed in `internal/tools`
  against the SDK's own default service root. The citation is corrected so the criterion
  can actually be traced.
* **AC-6** The `softwareVersion` in the built `site/dist/index.html` JSON-LD equals the
  `"."` value of `.release-please-manifest.json` on that same build, compared as strings.
  Asserting the property merely exists is not sufficient, because the pre-change literal
  also exists and is wrong.
* **AC-6a** No semantic-version literal remains anywhere in `site/build/seo.jsonld.ts`,
  confirmed by grep. This is asserted separately from AC-6 because a build that reads the
  manifest while leaving a stale literal behind as a fallback or a comment satisfies AC-6
  and still leaves the defect in place for the next reader to copy.
* **AC-6b** The site build fails loudly, naming the file, when
  `.release-please-manifest.json` is missing or its `"."` key is absent. The build MUST
  NOT fall back to a default version string, because a fabricated version is the exact
  failure FR-8 exists to remove. This mirrors the existing `repoRootLlmsTxt` behaviour in
  `site/build/seo.plugin.ts`.
* **AC-7** None of the three new pages states a tool-surface figure of its own, and the
  extended `assertNoBareClaims` scan demonstrably reaches their Markdown sources: a bare
  tool-surface figure introduced into one of them MUST fail
  `.agents/scripts/site.content.check.mjs`, naming the file and line, and the check MUST
  pass once it is removed. A passing check alone does not satisfy this criterion, because
  a scan that never opens the new files also passes.

## Affected Components

Every file the phases below touch, verified to exist at these paths on
`dev/is-agentic-site` as of 2026-08-23. No Go package is modified.

| Path | Change | Phase |
|---|---|---|
| `site/build/seo.pages.ts` | Extend `PageKey`, add three `PageSeo` entries, match by path | 1 |
| `site/build/seo.plugin.ts` | Stop reducing the match key with `basename` | 1 |
| `site/vite.config.ts` | Derive Rollup input keys from the slug | 1 |
| `site/build/seo.jsonld.ts` | `AboutPage`, `ContactPage`, privacy `WebPage` entities | 2 |
| `site/build/doc.pages.ts` | Per-page output path, recursive mkdir, footer links | 3, 4 |
| `site/content/about.md` | New | 3 |
| `site/content/contact.md` | New | 3 |
| `site/content/privacy.md` | New | 3 |
| `.gitignore` | Ignore the three generated output directories | 3 |
| `site/src/components/Footer.tsx` | Three internal links | 4 |
| `.agents/scripts/site.pages.mjs` | Add three pages to `PAGES` | 5 |
| `.agents/scripts/site.content.check.mjs` | Markdown claim scan, text floors | 5, 6 |
| `site/lighthouserc.json` | Collect URLs, anchor the root pattern, new matrix entry | 5, 6 |
| `site/build/seo.jsonld.ts` | Source `softwareVersion` from the release manifest | 7 |
| `site/build/release.version.ts` | New: reads the release manifest at build time | 7 |

Not modified, but affected in behaviour and therefore verified: `site/build/sitemap.ts`
picks up the new pages from the registry with no edit, and `site.validate.mjs`,
`site.contrast.audit.mjs`, and `site.screenshot.mjs` each pick them up from
`site.pages.mjs` with no edit.

## Implementation Approach

The chosen approach is **site-local Markdown reusing the doc-page generator**. The
three pages are authored as Markdown under a site-local content directory (not the
embedded `docs/**` bundle, which `docs/embed.go` and its allowlist test fix at four
files), and `site/build/doc.pages.ts` is extended to emit them. They inherit the
existing generated-page template, so they are static HTML whose prose survives with
JavaScript disabled, satisfying the site's pre-render invariant without adding a React
entry, a router, or a hydration path.

A React-page alternative was considered and rejected: the trust anchor pages are
static prose with no interactive element, so a React entry would add a bundle, a
hydration step, and a second pre-render path to maintain for content that never
changes at runtime, against the site's standing preference for content that exists in
the markup.

Each page is emitted as a directory index (`about/index.html`) because GitHub Pages
resolves an extensionless request path only to a directory index. That decision has
two consequences the current build does not survive unchanged, and Phase 1 exists to
absorb them before any page is authored:

* `pageForFile` matches on a bare filename. Three pages named `index.html` in
  subdirectories collide with each other and with the landing page, so all three would
  silently receive the landing page's canonical URL and JSON-LD.
* The Rollup input key is also a bare filename, so the same three pages collide on the
  key `index` and `Object.fromEntries` would keep only the last.

Phases are sequential. Each is verified before the next begins.

**Structured data is added before the pages are emitted, and the order is load-bearing.**
`seo.plugin.ts` runs `transformIndexHtml` for every emitted HTML file, and for any file
`pageForFile` resolves it calls `renderJsonLd`, which reaches `entitiesFor` and then
`byKey[page.key]()`. A page that is registered in `seo.pages.ts` but has no branch in
`byKey` therefore looks up `undefined` and calls it, throwing at build time the moment
that page is first emitted.

That failure is not caught by the typecheck. `byKey` is typed `Record<PageKey, ...>`, so
a missing branch would ordinarily be a compile error, but `site/tsconfig.json` references
only `tsconfig.app.json`, whose `include` is `["src"]`. Nothing under `site/build/` is
typechecked by `tsc -b`, and Vite transpiles without checking types, so the
non-exhaustive record compiles clean and fails only at runtime. The ordering below is the
control, not the type system, and the `Record<PageKey, ...>` type MUST NOT be weakened to
an index signature regardless, so the guard is restored if `site/build/` is ever brought
into a typechecked project.

### Phase 1: Make the pipeline addressable by path, not by filename

Files: `site/build/seo.pages.ts`, `site/build/seo.plugin.ts`, `site/vite.config.ts`

1. Extend `PageKey` with `'about' | 'contact' | 'privacy'`.
2. Add the three `PageSeo` entries with `file` set to the dist-relative output path
   (`about/index.html`) and `path` set to the extensionless canonical path (`/about`),
   each with its own title and description.
3. Change `pageForFile` to match on the dist-relative path rather than the basename,
   normalising any leading slash so `/about/index.html` and `about/index.html` both
   resolve. Change `seo.plugin.ts` to stop reducing `ctx.path`/`ctx.filename` with
   `basename` and pass the site-relative path instead.
4. Change the Rollup input keys in `vite.config.ts` to derive from the page slug rather
   than `basename(p, '.html')`, so no two entries collide on `index`.

This phase registers the three pages without emitting them. No HTML file resolves to the
new keys yet, so `renderJsonLd` is never called for them and the absent `byKey` branches
cannot throw. `sitemap.xml` does list the three new URLs from this phase onward, ahead of
the pages existing; that is transient and closes in Phase 3, and the tree MUST NOT be
deployed between phases.

Verify: `pnpm --dir site run build` succeeds and `site/dist/index.html` still carries
the landing page's own canonical tag and `SoftwareApplication` JSON-LD, unchanged. This
phase must be provably inert for the existing four pages before any page is added.

### Phase 2: Structured data

Files: `site/build/seo.jsonld.ts`

1. Add an `aboutPage`, a `contactPage`, and a privacy `webPage` entity builder. The
   contact entity states the GitHub issue tracker as its contact channel and reuses the
   existing `REPO` constant, publishing no email address and no postal address.
2. Add all three branches to `entitiesFor`. All three are added in this phase, not one
   per page later, because a single missing branch crashes the build for every page once
   emission begins.

Verify: `pnpm --dir site run build` succeeds and the existing four pages' JSON-LD is
byte-identical to Phase 1's output. The new entities cannot be observed in built HTML
yet, because no page carrying them is emitted until Phase 3; assert instead that
`entitiesFor` returns the entity FR-4 assigns for each of the three new keys, by calling
it directly against each new `PageSeo` entry. Confirming exhaustiveness here is the whole
point of the phase, and it MUST NOT be deferred to the typecheck, which does not read
this file.

### Phase 3: Author the Markdown and emit the directory-index pages

Files: `site/content/about.md`, `site/content/contact.md`, `site/content/privacy.md`
(new), `site/build/doc.pages.ts`, `.gitignore`

1. Extend the `DocPage` interface with an explicit output path so a page can be emitted
   at `about/index.html` rather than at `<slug>.html`, keeping the existing three
   entries on their current flat output.
2. Create the output directory before writing (`mkdirSync` with `recursive`), since
   `generateDocPages` currently only ever writes into an existing `site/`.
3. Author the three Markdown sources against FR-1, FR-2, and FR-3. Each begins with a
   single level-1 heading, which the template renders as the page's only `<h1>`.
4. Add `/site/about/`, `/site/contact/`, and `/site/privacy/` to `.gitignore`, matching
   the existing per-page entries for the generated `concepts.html`, `quickstart.html`,
   and `troubleshooting.html`, so generated output stays untracked.

This is the first phase in which the three pages are emitted, so it is the first in which
the Phase 1 path matching and the Phase 2 JSON-LD branches are exercised end to end.

Verify: `pnpm --dir site run build`, then confirm `site/dist/about/index.html` and its
two siblings exist, each carries its own canonical tag naming the extensionless URL, each
carries exactly the entity FR-4 assigns it, and none carries any of the landing page's
`SoftwareApplication`, `FAQPage`, or `Organization` entities (AC-2a).

### Phase 4: Footer links

Files: `site/src/components/Footer.tsx`, `site/build/doc.pages.ts`

1. Add the three links to the landing-page footer (FR-6a). They are internal document
   links, so they MUST NOT use the `#`-anchor smooth-scroll handler that the existing
   product links use, and MUST NOT carry `target="_blank"`.
2. Add the three links plus a link to `/` to the generated-page footer template
   (FR-6b).

Verify: `node .agents/scripts/site.content.check.mjs` still passes, and the landing
page's recorded text floor is re-measured in Phase 6 because added footer text raises
it.

### Phase 5: Enrol the new pages in the harness

Files: `.agents/scripts/site.pages.mjs`, `.agents/scripts/site.content.check.mjs`,
`site/lighthouserc.json`

1. Add `about/index.html`, `contact/index.html`, and `privacy/index.html` to `PAGES` in
   `site.pages.mjs`. This list is imported by `site.content.check.mjs`,
   `site.validate.mjs`, `site.contrast.audit.mjs`, and `site.screenshot.mjs`, so the
   three pages are enrolled in the content check, W3C validation, the contrast audit,
   and screenshot capture in one edit. That fan-out is intended: the contrast audit
   must report zero failures for the new pages as it does for the existing four.
2. Extend `assertNoBareClaims` to scan the new pages' Markdown sources, satisfying
   FR-7a. Extending the `walkSources` extension filter to include `.md` and pointing
   the scan at the site content directory is sufficient; do not add a denylist of known
   figures.
3. Add the three page URLs to `collect.url` in `site/lighthouserc.json`.
4. Anchor the existing `.*/index\.html$` assertion pattern so it matches only the site
   root, and add a third `assertMatrix` entry for the three new pages. Without step 4
   the new pages are graded against the landing page's thresholds, which assert no
   performance score at all.

The text floors for the three new pages are deliberately not set here; Phase 6 measures
and records them. Until it does, `TEXT_FLOOR[page]` is `undefined` for each new page and
`text < undefined` evaluates to `false`, so the content check reports `ok` for them and
enforces nothing. A green run at this phase is therefore not evidence the floors hold. Do
not read it as one, and do not skip Phase 6 on the strength of it.

Verify: `node .agents/scripts/site.content.check.mjs`,
`node .agents/scripts/site.validate.mjs`, and
`node .agents/scripts/site.contrast.audit.mjs` all pass.

### Phase 6: Measure and record the gates

Files: `.agents/scripts/site.content.check.mjs`, `site/lighthouserc.json`

1. Measure each new page's no-JavaScript `document.body.textContent.length` on the
   green build and record it in `TEXT_FLOOR`, with the same per-floor rationale comment
   the existing entries carry.
2. Re-measure and update the landing-page floor, which Phase 4 raises by adding footer
   text. Account for the increase in the comment rather than silently editing the
   number.
3. Set the new `assertMatrix` entry's thresholds to the doc-page bar (performance,
   accessibility, best-practices and SEO at 1, LCP at 1700, CLS at 0.01, TBT at 50),
   since the new pages are static HTML of the same shape as the documentation pages.
   If a measurement misses one of those, amend this change request with the measured
   ceiling and the chain that established it. A threshold MUST NOT be lowered quietly
   to make the gate pass.

Verify: `pnpm --dir site run lighthouse` passes.

### Phase 7: Source the published software version instead of transcribing it

Files: `site/build/release.version.ts` (new), `site/build/seo.jsonld.ts`

This phase is independent of Phases 1 to 6 and touches no trust anchor page. It is
carried here because it is the same defect class in the same file, and because the
landing page's structured data is currently publishing a version that has never been
released.

**Source selection.** The release version is not currently exposed to the site build,
and the minimal wiring is to read the record that already holds it rather than to
introduce a second one. The candidates and why one wins:

* `.release-please-manifest.json`, the `"."` key. **Chosen.** Tracked in the repository,
  written by release-please on every release, currently `0.5.1` and matching the `v0.5.1`
  tag, readable by Node at site build time with no new build step, and already in the
  bare form schema.org wants.
* The provenance pipeline (`site/build/provenance.ts`, `build-info.json`). **Rejected.**
  It deliberately carries commit, build time, run, and environment, and no version. Its
  governing rule is that provenance must never fabricate identity, and it collapses to a
  `local` marker off CI. A release version is a different fact from which build produced
  the artefact, and a site built locally must still publish the real released version.
* `internal/buildinfo` and `main.version`. **Rejected.** Both are populated at Go link
  time from goreleaser `ldflags`. No Go binary is built or executed during the site
  build, so neither value is reachable from Vite.
* `site/package.json` `version`. **Rejected.** It reads `0.0.0` and is not maintained as
  the release version.

1. Add `site/build/release.version.ts` exporting a single function that reads
   `.release-please-manifest.json` from the repository root and returns the `"."` value.
   Resolve the path from the module's own location, matching how `repoRootLlmsTxt` in
   `site/build/seo.plugin.ts` reaches the repository root. Throw an error naming the file
   when it is missing or the key is absent (AC-6b). Do not supply a default.
2. In `site/build/seo.jsonld.ts`, replace `softwareVersion: '0.8.0'` with a call to that
   function. Delete the literal outright rather than leaving it as a fallback or a
   comment (AC-6a). Update the `softwareApplication` docstring to state that the version
   is read from the release manifest, matching how it already documents `featureList` as
   manifest-derived.

Verify:

```bash
pnpm --dir site run build
grep -nE "[0-9]+\.[0-9]+\.[0-9]+" site/build/seo.jsonld.ts   # must find no version literal
node -e "const m=require('./.release-please-manifest.json')['.'];const h=require('fs').readFileSync('site/dist/index.html','utf8');process.exit(h.includes('\"softwareVersion\": \"'+m+'\"')?0:1)"
```

The second command asserts AC-6 directly: the built page must carry the manifest's
version, not merely some version.

## Verification

This change touches `site/**` and `.agents/scripts/**` only. It adds no Go code and
edits nothing under `docs/**`, so the Go pipeline is unaffected and `site.yml` is the
governing workflow. The full gate for the change is:

```bash
pnpm --dir site install --frozen-lockfile
pnpm --dir site run build
node .agents/scripts/site.content.check.mjs
node .agents/scripts/site.validate.mjs
node .agents/scripts/site.contrast.audit.mjs
pnpm --dir site run lighthouse
```

`make ci` is not a gate for this change and MUST NOT be treated as one: its targets
(`docs-bundle`, `surface-check`, `build`, `vet`, `fmt-check`, `tidy`, `lint`, `test`,
`goreleaser-check`, `mcpb-validate`) cover the Go module and the generated surface
manifest, neither of which this change modifies. Run it once at the end to confirm
nothing outside the site moved.

## Test Strategy

* **FR-1, FR-2, FR-3, AC-1** `.agents/scripts/site.content.check.mjs` asserts each new
  page exists at its directory-index path, clears its recorded text floor, and carries
  exactly one `<h1>`, all with JavaScript disabled.
* **FR-4, FR-5, AC-2** `.agents/scripts/site.validate.mjs` posts each new page to the
  W3C Nu checker and MUST report zero real errors, confirming the JSON-LD and markup
  are valid.
* **AC-2a** A dedicated assertion confirms each new page's canonical tag names its own
  extensionless URL and that its head carries none of the landing page's three
  entities. A build-passes check is not sufficient evidence here, because the
  `basename` collision this change removes fails silently.
* **FR-6a, FR-6b, AC-3** The content check asserts the three links are present in the
  pre-rendered landing HTML and in each generated page's footer.
* **FR-7, FR-7a** The extended `assertNoBareClaims` scan MUST fail on a deliberately
  introduced bare tool-surface figure in one of the new Markdown sources, then pass
  once it is removed. Asserting only that the check passes does not demonstrate the
  scan reaches the new files at all.
* **AC-4a** `pnpm --dir site run lighthouse` passes, and the report MUST be confirmed
  to contain three new page entries. A pattern that matches nothing also passes.
* **AC-5** Each privacy claim is traced by hand to the symbol named in AC-5 and the
  tracing recorded in the pull request.
* The contrast audit (`site.contrast.audit.mjs`) MUST report zero failures for the
  three new pages at all four widths.
* **FR-8, AC-6, AC-6a, AC-6b** The built landing page's `softwareVersion` is compared
  against `.release-please-manifest.json` on the same build and MUST match; a grep of
  `site/build/seo.jsonld.ts` MUST find no version literal; and a build run against a
  temporarily renamed manifest MUST fail naming the file rather than emitting a default.
  The last case is exercised deliberately, because a fallback path that is never taken
  looks identical to one that does not exist.

## Settled Decisions

The requestor settled both privacy questions on 2026-08-23:

* **No contact email.** The GitHub issue tracker is the only published contact channel.
  No email address is published on the contact page or in the Organization
  `contactPoint`. This is deliberate, not a gap for the implementor to fill.
* **No postal address.** The Organization schema publishes no `PostalAddress`, because
  the project has none. It stays omitted. A fabricated address is not acceptable.

Both decisions are settled. The implementor MUST NOT introduce an email address or a
postal address on any of the three new pages or in any structured-data entity.

The requestor settled a third question on 2026-08-23, in response to this review:

* **The hardcoded `softwareVersion` is absorbed into this change request** as Phase 7,
  rather than deferred to a separate one. It is sourced from
  `.release-please-manifest.json` at build time and the literal is deleted. Scope is the
  version property only; no other provenance or release wiring is in scope here.

<!-- review-summary -->
Reviewed 2026-08-23 against `dev/is-agentic-site` at `5c2037b` plus the uncommitted
`da19458` site trust-signal work.

FINDINGS: 19 (drift 6, contradiction 4, ambiguity 5, coverage 2, ordering 2)
FIXES APPLIED: 19
UNRESOLVED: 0
PHASES: 7

DRIFT (6), reconciled against the current codebase:
0. `site/build/seo.jsonld.ts` publishes `softwareVersion: '0.8.0'`, a literal written
   once in `09397dd` and never updated. The latest tag is `v0.5.1` and
   `.release-please-manifest.json` reads `0.5.1`, so the live site advertises a version
   that has never been released. Raised in the first review pass as the sole UNRESOLVED
   item; the requestor chose to absorb it, and it is now FR-8, AC-6, AC-6a, AC-6b and
   Phase 7.
1. AC-5 cited "the Graph client" as the owner of outbound destinations. No such
   component exists here: `internal/graph` holds errors, retry, serialization and enum
   helpers, and `GraphServiceClient` is constructed in `internal/tools` against the
   `msgraph-sdk-go` default service root. AC-5 rewritten with traceable citations
   (`internal/auth/authcode.go` for the identity endpoint, `internal/auth/auth.go` for
   the loopback redirect, `internal/config/config.go` `OTELEnabled`/`OTELEndpoint`).
2. FR-3 omitted the inbound loopback socket, which `site/src/components/PrivacySection.tsx`
   already discloses on the landing page. Added, with the reason it is not optional.
3. Current State omitted `site/public/404.html`, added by `da19458` and served as a page.
   Added, and the `contactPoint` claim confirmed present in `seo.jsonld.ts`.
4. `.gitignore` ignores each generated page individually (lines 79 to 81). The three new
   generated output directories need entries and were absent from scope. Added to Phase 3.
5. Scope omitted the harness fan-out: `site.pages.mjs` `PAGES` is imported by
   `site.validate.mjs`, `site.contrast.audit.mjs` and `site.screenshot.mjs` as well as the
   content check. Stated in Phase 5 and in Affected Components.

CONTRADICTION (4):
1. FR-4 offered `PrivacyPolicy` "where the pipeline supports it". schema.org defines no
   such type, so the option was unsatisfiable. Settled on `WebPage`.
2. FR-6 required footer links while AC-3 verified only the landing HTML, and two distinct
   footers exist (`Footer.tsx` and the `pageTemplate` footer in `doc.pages.ts`). As written
   the three new pages would be crawl dead ends. Split into FR-6a and FR-6b; AC-3 updated.
3. AC-4 asserted the Lighthouse gate passes, but `lighthouserc.json` collects only four
   URLs, so the criterion was vacuously true. Split out as AC-4a requiring the new URLs in
   `collect.url` and confirmation that the report contains them.
4. FR-1 to FR-3 set a 500-character minimum while AC-4 required a recorded text floor.
   These are different numbers and were conflated. Separated explicitly.

AMBIGUITY (5):
1. Implementation Approach presented two mutually exclusive approaches as numbered items 1
   and 2. The downstream orchestrator enumerates numbered phases and implements each in
   order, so this would have been executed as "build it twice". Restructured into six
   sequential phases, each naming its affected files and its verification command.
2. "Lowest-cost path" and "more control over layout, more code" were evaluative and not
   testable. Removed with the alternatives framing; the rejection of the React path is now
   stated as a reason, not a cost comparison.
3. Test Strategy said "record any new-page thresholds it holds". Replaced with MUST-form
   per-criterion entries mapped to FR and AC identifiers.
4. "Whichever path, emit each page as a directory index" was correct but named no file and
   accounted for neither collision it causes. Now Phase 1.
5. No phase stated its verification command. Each now does, plus a Verification section
   naming the governing workflow.

COVERAGE (2):
1. FR-7 (no tool-surface figure) had no executable rule behind it. `assertNoBareClaims`
   walks only `.ts`/`.tsx` via the `walkSources` extension filter, so Markdown sources are
   unscanned. The repository standard requires prose and executable rule together. Added
   FR-7a, Phase 5 step 2, and a Test Strategy entry requiring the scan be proven to reach
   the new files by failing on a deliberately introduced figure.
2. FR-7 had no acceptance criterion exercising it, so it was the one requirement with no
   AC. Added AC-7, which asserts the scan reaches the new Markdown sources rather than
   merely that the check passes.

ORDERING (2), both defects introduced by this review's own first restructure and caught
before implementation began:
1. The phase order was build-breaking. The first restructure put page emission before the
   JSON-LD branches. `seo.plugin.ts` calls `renderJsonLd` for every emitted file that
   `pageForFile` resolves, reaching `byKey[page.key]()`; with the pages registered in
   Phase 1 and no branch until the structured-data phase, the emit phase would look up
   `undefined` and call it, throwing at build time. That phase's own verify step, which
   asked to confirm the emitted pages carry their canonical tags, was therefore
   unsatisfiable. Structured data now runs as Phase 2 and emission as Phase 3, with the
   dependency stated above the phase list rather than left implicit in the numbering.
2. Phase 1 claimed the `Record<PageKey, ...>` type makes a missing JSON-LD branch "a
   compile error rather than a runtime gap". It does not. `site/tsconfig.json` references
   only `tsconfig.app.json`, whose `include` is `["src"]`, so nothing under `site/build/`
   is typechecked by `tsc -b`, and Vite transpiles without checking types. The record is
   non-exhaustive at runtime and compiles clean. The claim is corrected, the ordering is
   named as the actual control, and the `Record<PageKey, ...>` type is still required to
   stay as it is so the guard returns if `site/build/` is ever typechecked.

   This is worth recording rather than quietly fixing: the false claim is what made the
   bad order look safe. Having written that a missing branch could not reach a build, the
   restructure had no reason to order the phases against it.

BLOCKER FOUND AND ABSORBED (the substantive result of this review):
Emitting the pages as directory indexes, which GitHub Pages requires for extensionless
paths, breaks two build assumptions the CR did not account for. `seo.plugin.ts` resolves
pages with `basename(ctx.path || ctx.filename || '')`, so `about/index.html`,
`contact/index.html`, `privacy/index.html` and the landing page all reduce to the key
`index.html`; `pageForFile` would return the landing page for all four, giving every trust
anchor page the landing page's canonical URL and its `SoftwareApplication`, `FAQPage` and
`Organization` JSON-LD. Independently, `vite.config.ts` derives Rollup input keys with
`basename(p, '.html')`, so the same four collide on the key `index` and
`Object.fromEntries` keeps only the last. Both failures are silent: the build succeeds and
the pages validate. A third, milder instance of the same shape: the Lighthouse
`.*/index\.html$` pattern matches every directory index, so the new pages would be graded
against the landing page's thresholds, which assert no performance score. Phase 1 removes
the first two before any page is authored; Phase 5 step 4 removes the third. AC-2a asserts
the outcome directly rather than relying on the build passing.

UNRESOLVED: none. The one item raised in the first review pass, the hardcoded
`softwareVersion`, was returned by the requestor with a decision to absorb it into this
change request. It is now FR-8, AC-6, AC-6a, AC-6b, and Phase 7, and the phase count is
seven.

Version source, recorded because the phase turns on it: `.release-please-manifest.json`
(the `"."` key) is authoritative and readable at site build time, and the release version
was not otherwise exposed to the site build, so Phase 7 adds the minimal wiring
(`site/build/release.version.ts`) rather than a second literal. The provenance pipeline
was considered and rejected: it deliberately carries no version, its governing rule is that
it must never fabricate identity, and it collapses to a `local` marker off CI, whereas a
locally built site must still publish the real released version. `internal/buildinfo` and
`main.version` were rejected because both are populated at Go link time and no Go binary
is built or run during the site build. `site/package.json` reads `0.0.0` and is not
maintained as the release version.

The severity of this item rose on investigation rather than falling. It was raised as a
future-drift risk on the assumption the literal tracked some earlier release. It does not:
`0.8.0` has never been a release of this project, so the correction is to a claim that is
false today, which is why AC-6 compares the built value against the manifest rather than
merely asserting the property is present.
<!-- /review-summary -->
