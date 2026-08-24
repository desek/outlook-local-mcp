# CR-0077 Validation Report

## Summary
Requirements: 11/11 | Acceptance Criteria: 14/14 | Tests: 10/10 | Gaps: 0
(all four gaps FIXED; see the gap-fix pass recorded below)

The original audit graded **AC-1** FAIL and downgraded three Test Strategy rows.
All four findings were harness work, and all four are now closed:

* `.agents/scripts/site.serve.mjs` resolves an extensionless path to the
  directory index, so `/about`, `/contact`, and `/privacy` return `200` with the
  correct document. Trailing-slash, `.html`, and 404 behaviour are unchanged.
* `.agents/scripts/site.content.check.mjs` now carries three standing assertions
  that did not exist: the extensionless paths resolve over HTTP (AC-1), the
  footer crawl path is intact by `href` in both directions (FR-6a, FR-6b, AC-3),
  and each page's canonical names its own URL while it carries exactly the
  schema.org entities its registry key declares and none of the landing page's
  (AC-2a).

No page content, no requirement, and no acceptance criterion was changed. Each
new assertion derives its cases from `site/build/seo.pages.ts` and
`site/build/seo.jsonld.ts` rather than a hand-kept list, so a fourth trust anchor
page is covered without editing the script.

Scope: branch `dev/is-agentic-site`, merge-base `origin/main` at `5c2037b`, HEAD
`e310b8a`. Group A pre-CR work (`da19458`, `ba97e6f`) is excluded from
attribution throughout: `site/public/404.html`, root `llms.txt`,
`internal/docs/llmstxt.go`, and the `url` / `offers` / `contactPoint` portions of
`site/build/seo.jsonld.ts` belong to that work, not to CR-0077. Commit
attribution is clean, with no phase commit touching a Group A file and no Group A
commit touching a CR-0077 file.

### Measurements taken during this audit

Every figure below was produced in this session against a clean build
(`rm -rf site/dist site/dist-ssr && pnpm --dir site run build`), not read from the
CR.

| Gate | Command | Result |
|---|---|---|
| Site build | `pnpm --dir site run build` | exit 0, `prerender: optimised 6 doc-page documents` |
| Content check | `node .agents/scripts/site.content.check.mjs` | exit 0, all assertions hold |
| W3C validity | `node .agents/scripts/site.validate.mjs` | 0 real errors on all 7 pages |
| Contrast audit | `node .agents/scripts/site.contrast.audit.mjs` | 0 failing text elements, 7 pages x 4 widths |
| Lighthouse round 1 | `pnpm --dir site run lighthouse` | exit 0, 7 URLs / 21 runs |
| Lighthouse round 2 | `pnpm --dir site run lighthouse` (same build) | exit 0, 7 URLs / 21 runs |
| Go pipeline | `make ci` | exit 0 (build, vet, fmt, tidy, lint 0 issues, test, goreleaser, mcpb) |

Instrument validation, per the project's measurement rules. The content check was
run twice over the unchanged build and returned identical character counts for all
seven pages, so its noise floor is 0 characters. Lighthouse was run twice over the
same unchanged build; Cumulative Layout Shift measured `0.0000` on all six runs of
each trust anchor page in both rounds, so the noise floor on that metric is 0.0000
and the 0.01 threshold is cleared with the whole of the margin, not by an
aggregation artefact. This is the property the CR's own Phase 6 amendment warned
was absent pre-fix, where `privacy` passed on one lucky run of six.

Measured Lighthouse figures, two rounds, three runs per URL, mobile:

| Page | Round | CLS | LCP (ms) | TBT (ms) | Perf / A11y / BP / SEO |
|---|---|---|---|---|---|
| `/about/index.html` | 1 | 0.0000, 0.0000, 0.0000 | 1505, 1503, 1502 | 0 | 1 / 1 / 1 / 1 |
| `/about/index.html` | 2 | 0.0000, 0.0000, 0.0000 | 1503, 1503, 1502 | 0 | 1 / 1 / 1 / 1 |
| `/contact/index.html` | 1 | 0.0000, 0.0000, 0.0000 | 1501, 1502, 1503 | 0 | 1 / 1 / 1 / 1 |
| `/contact/index.html` | 2 | 0.0000, 0.0000, 0.0000 | 1503, 1502, 1510 | 0 | 1 / 1 / 1 / 1 |
| `/privacy/index.html` | 1 | 0.0000, 0.0000, 0.0000 | 1504, 1502, 1503 | 0 | 1 / 1 / 1 / 1 |
| `/privacy/index.html` | 2 | 0.0000, 0.0000, 0.0000 | 1503, 1502, 1503 | 0 | 1 / 1 / 1 / 1 |

Against the CR's Phase 6 pre-fix baseline (about 0.0260, contact 0.0119, privacy
0.0311) this reproduces Phase 8's recorded post-fix result exactly. The CLS defect
is closed and the closure is confirmed independently of the commit that claimed it.

## Requirement Verification

| Req # | Description | Status | Evidence (file:line / test name) |
|---|---|---|---|
| FR-1 | About page at `/about`: what the project is, open source under stated licence, Graph relationship, GigWhere acknowledgement, >= 500 chars prose | PASS | `site/content/about.md:1-33` — MIT Licence at `:12`, Graph relationship section `:18-29`, GigWhere at `:33`; registry entry `site/build/seo.pages.ts:96-103`; emitted `site/dist/about/index.html`; measured 1,845 chars no-JS by `site.content.check.mjs` (floor 1,845) |
| FR-2 | Contact page at `/contact`: GitHub repository and issue tracker, >= 500 chars prose | PASS | `site/content/contact.md:8-20` (repository `:11`, issue tracker `:19`); registry `site/build/seo.pages.ts:104-111`; measured 1,719 chars no-JS (floor 1,719) |
| FR-3 | Privacy page at `/privacy`: local execution, OS keychain tokens, outbound to Graph and identity platform plus optional OTel, inbound loopback socket, site collects nothing; >= 500 chars; every claim true | PASS | `site/content/privacy.md` — local `:8-14`, keychain `:16-23`, outbound `:25-35`, **loopback `:37-42`**, logging `:44-48`, website `:50-54`; measured 2,729 chars no-JS (floor 2,729); claim-to-code tracing under AC-5 below |
| FR-4 | Each page: canonical URL, Open Graph, Twitter card, >= 1 valid schema.org entity; `AboutPage` / `ContactPage` / `WebPage` respectively | PASS | Builders `site/build/seo.jsonld.ts:234-242` (`aboutPage`), `:255-268` (`contactPage`), `:215-222` (`webPage`); dispatch `:276-289`; built output verified: about → `"@type": "AboutPage"`, contact → `"ContactPage"` + `"ContactPoint"`, privacy → `"WebPage"`, each with own canonical, `og:title`/`og:type`/`og:url`, `twitter:card` |
| FR-5 | Each page in `sitemap.xml` with extensionless canonical URL, emitted from the same registry entry as its canonical tag | PASS | Single source `site/build/seo.pages.ts:63-120` (`path`) consumed by `canonicalUrl` `:128-130` and by `build/sitemap.ts` (unmodified); built `sitemap.xml` carries `/about`, `/contact`, `/privacy` and each page's canonical names the identical URL |
| FR-6 | Full prose renders without JavaScript | PASS | `site.content.check.mjs:161-175` drives each page with `setJavaScriptEnabled(false)`; all three clear their floors; pages are static HTML from `pageTemplate` `site/build/doc.pages.ts:74-97` with no React entry |
| FR-6a | Landing-page footer links all three pages | PASS | `site/src/components/Footer.tsx:22-26` (`DOCUMENT_LINKS`, no `#` handler, no `target="_blank"`), rendered `:168-179`; pre-rendered `site/dist/index.html` carries `href="/about"`, `href="/contact"`, `href="/privacy"` (1 each) |
| FR-6b | Generated-page footer template links all three plus `/` | PASS | `site/build/doc.pages.ts:90-93`; all six generated pages (`about`, `contact`, `privacy`, `concepts`, `quickstart`, `troubleshooting`) carry `href="/"`, `href="/about"`, `href="/contact"`, `href="/privacy"` |
| FR-7 | Pages state no tool-surface figure of their own | PASS | `site.content.check.mjs` reports `ok claims: no bare tool-surface figure in 3 source files under site/content`; no digit-bearing surface claim in the three Markdown sources |
| FR-7a | FR-7 enforced by an executable rule reaching Markdown sources | PASS | `walkSources` extension filter extended to `.md` at `site.content.check.mjs:353`; second scan invocation at `:229` targeting `site/content`; negative test fired this session (see AC-7) |
| FR-8 | `softwareVersion` sourced from the release record at build time; no version literal in `seo.jsonld.ts` | PASS | `site/build/release.version.ts:32-51` (new file); call site `site/build/seo.jsonld.ts:100`; `grep -nE "[0-9]+\.[0-9]+\.[0-9]+" site/build/seo.jsonld.ts` exits 1 (no match); built `dist/index.html` carries `"softwareVersion": "0.5.1"` matching `.release-please-manifest.json` |

## Acceptance Criteria Verification

| AC # | Description | Status | Evidence |
|---|---|---|---|
| AC-1 | Extensionless `/about`, `/contact`, `/privacy` each return HTML >= 500 chars no-JS with exactly one `<h1>`; **`site.serve.mjs` MUST resolve the extensionless path to the directory index** | **FIXED** | `.agents/scripts/site.serve.mjs:57` — an extensionless path with no matching file now falls back to `<path>/index.html`, mirroring GitHub Pages. Re-probed against a clean `site/dist`: `/about` → **200**, 44,625 bytes, `<h1>About Outlook Local MCP</h1>`; `/contact` → **200**, 44,754 bytes, `<h1>Contact</h1>`; `/privacy` → **200**, 45,411 bytes, `<h1>Privacy</h1>`. Existing behaviour unchanged in the same probe: `/about/` and `/about/index.html` 200 and byte-identical, `/concepts.html` 200, and `/nope`, `/nope/`, `/assets` each still **404** so a missing artefact stays a measurable failure. Now guarded by a standing assertion, `assertExtensionlessPaths` in `site.content.check.mjs`, which drives every directory-index registry entry over HTTP: `ok extensionless paths: /about, /contact, /privacy each resolve to their directory index`. The content and `<h1>` sub-clauses continue to hold (1,845 / 1,719 / 2,729 chars, 1 `<h1>` each) |
| AC-2 | Each page in `sitemap.xml` with extensionless URL, own canonical naming that URL, own OG and Twitter card, the FR-4 entity; `site.validate.mjs` zero real errors | PASS | `sitemap.xml` lists `https://outlook-local-mcp.com/{about,contact,privacy}`; each built page carries `<link rel="canonical" href="https://outlook-local-mcp.com/about">` (etc.), `og:title` / `og:type` / `og:url`, `twitter:card="summary_large_image"`, and its assigned entity. `node .agents/scripts/site.validate.mjs` this session: `0 real error(s)` on all seven pages |
| AC-2a | No new page carries the landing canonical or its `SoftwareApplication` / `FAQPage` / `Organization` JSON-LD | PASS (now guarded) | Standing assertion added: `assertPageIdentity` in `site.content.check.mjs` requires, per registry entry, that the canonical equals `${SITE_ORIGIN}${page.path}` and that the page's top-level JSON-LD entities are exactly the set its key declares, with no landing entity on any subpage — `ok page identity: 7 pages, each with its own canonical and exactly its assigned entities`. Falsified by substitution against a mutated copy of `dist`: retyping about's entity and repointing privacy's canonical at `/` both fail, naming the page. Original evidence: per-page grep on built output: `SoftwareApplication\|FAQPage\|"Organization"` → **0** on each of the three; landing canonical `href="https://outlook-local-mcp.com/"` → **0** on each. Mechanism: path-keyed match `site/build/seo.pages.ts:144-147`, `site/build/seo.plugin.ts:65-71`, distinct Rollup keys `site/vite.config.ts:24-27`; `aboutPage` deliberately embeds no Organization node (`seo.jsonld.ts:226-233` docstring) |
| AC-3 | Landing HTML links all three (FR-6a); each generated page footer links the other two and `/` (FR-6b); both without JavaScript | PASS (now guarded) | Verified by grep on the pre-rendered, JS-free artefacts — see FR-6a and FR-6b rows. Now also guarded by `assertFooterLinks` in `site.content.check.mjs`, which asserts the `href` targets themselves on every registry page: `ok footer links: /about, /contact, /privacy linked from every page, root linked back from each generated page`. Falsified by substitution: retargeting the landing page's `/about` link to an external URL fails the check by page name |
| AC-4 | Content check passes with a **measured** floor per new page, not the 500-char minimum | PASS | `site.content.check.mjs:69-77` records `about/index.html: 1845`, `contact/index.html: 1719`, `privacy/index.html: 2729` — all measured values, none is 500; rationale and instrument validation at `:121-135`; check exits 0 with each page at exactly its floor, reproducing the recorded measurement |
| AC-4a | Lighthouse passes with the three pages in `collect.url` and matched by an `assertMatrix` entry of their own; root pattern anchored | PASS | `site/lighthouserc.json:10-12` (three URLs), `:27` root pattern anchored to `^https?://[^/]+/index\.html$`, `:48-59` dedicated trust anchor entry. Both rounds: `Checking assertions against 7 URL(s), 21 total run(s)`, exit 0. The report demonstrably contains the three pages — per-URL metrics extracted from `.lighthouseci/lhr-*.json` for `/about/index.html`, `/contact/index.html`, `/privacy/index.html` |
| AC-5 | Every privacy claim traceable to the code producing it | PASS | Token storage → `internal/auth/cache_backend_cgo.go:11-37` (keychain probe), `internal/auth/active_backend.go:22` and `cache_cgo.go:19,65` (file AES-256-GCM fallback); optional OTel → `internal/config/config.go:89-95` and `:285-286` (`GetEnv(EnvOTELEnabled, "false")`, `GetEnv(EnvOTELEndpoint, "")` — off and empty by default); identity platform → `internal/auth/authcode.go:152` (`https://login.microsoftonline.com/`); loopback sign-in socket → `internal/auth/auth.go:262` (`RedirectURL: "http://localhost"`); Graph service root → no `SetBaseUrl` or `graph.microsoft.com` in any non-test Go file, so the `msgraph-sdk-go` default stands, as AC-5 states. Log sanitisation claim → `internal/config/config.go:260` (`GetEnv(EnvLogSanitize, "true") != "false"`, on by default) |
| AC-6 | Built `dist/index.html` `softwareVersion` equals `.release-please-manifest.json` `"."`, compared as strings | PASS | Executed the CR's own assertion: manifest `0.5.1`, built HTML contains `"softwareVersion": "0.5.1"`, string comparison `true` |
| AC-6a | No semantic-version literal anywhere in `seo.jsonld.ts` | PASS | `grep -nE "[0-9]+\.[0-9]+\.[0-9]+" site/build/seo.jsonld.ts` → exit 1, no output. The literal is deleted, not retained as fallback or comment |
| AC-6b | Build fails loudly naming the file when the manifest or its `"."` key is absent; no default fallback | PASS | Both throw paths exercised against a byte-identical copy of `release.version.ts` in a scratch tree: manifest absent → `.release-please-manifest.json is missing (expected at the repository root, resolved to …); the site reads the published software version from it and cannot build without it`; manifest present without `"."` → `… has no "." key holding the released version; …`; valid manifest → returns the value. `release.version.ts:38-49`. No default is returned on any path |
| AC-7 | The extended scan demonstrably reaches the new Markdown: an introduced bare figure MUST fail the check naming file and line, and pass once removed | PASS | Negative test executed this session against a mirrored content tree (real source untouched): injected `The server exposes 23 tools across its surface.` → `FAIL bare tool-surface claim "23 tools" at site/content/about.md:35 (derive it from src/generated/surface.json)`, `content-check: 1 failing`. Against the real tree the same scan reports `ok claims: no bare tool-surface figure in 3 source files under site/content`. Both halves of the criterion satisfied |
| AC-8 | Each page carries the four `DOC_FONTS` preloads, an inlined `<style>`, and no local `rel="stylesheet"`, matching `concepts.html` | PASS | Built output: `about` / `contact` / `privacy` each `preload=4 style=1 stylesheet=0`; `concepts.html` `preload=4 style=1 stylesheet=0` — identical. Mechanism `site/build/prerender.mjs:251-259` with `DOC_FONTS` `:199-204` |
| AC-8a | Trust anchor `assertMatrix` asserts CLS at severity `error`, threshold unchanged at 0.01, passing across two independent rounds on the same build | PASS | `site/lighthouserc.json:56` — `"cumulative-layout-shift": ["error", { "maxNumericValue": 0.01 }]`, severity `error`, threshold 0.01 unchanged, and the Phase 6 `_comment` recording the pending fix is gone. Two rounds run this session on one unchanged build: CLS `0.0000` on all six runs of each page, both rounds exit 0. No ceiling needed; the threshold was met, not relaxed |
| AC-8b | The class is closed: no `pageTemplate` page can reach `dist` unoptimised; demonstrated with an added doc-template page | PASS | Set derived from the artefact, not a list: `site/build/prerender.mjs:216-259` scans `dist` recursively for `class="doc-page"`. Demonstrated this session — staged two extra marked pages into `dist` between `vite build` and `prerender.mjs` (`fourth-doc-page.html` and a deep `deep/nested/arbitrary-name.html`); both were optimised automatically with **no list edit**, output `prerender: optimised 8 doc-page documents`, each staged file ending at `preload=4 style=1 stylesheet=0`. The inverse guard also fires: with the marker removed from all six pages, `prerender.mjs` exits **1** with `no page carrying class="doc-page" found under dist; the doc-page template changed shape` (`:263-265`) |

## Test Strategy Verification

| Test File | Test Name | Specified | Exists | Matches Spec |
|---|---|---|---|---|
| `.agents/scripts/site.content.check.mjs` | Each new page exists at its directory-index path, clears its recorded floor, exactly one `<h1>`, JS disabled (FR-1, FR-2, FR-3, AC-1) | Yes | Yes | **PASS (was PARTIAL, now FIXED)** — floor, `<h1>`, and JS-disabled assertions all present and passing, measured at the directory-index path where the floors were baselined. The extensionless path AC-1 is about is now exercised too, by `assertExtensionlessPaths`, which requires HTTP 200 and a non-empty document for every directory-index registry entry through the same served origin the rest of the check measures against |
| `.agents/scripts/site.validate.mjs` | Each new page posted to the W3C Nu checker, zero real errors (FR-4, FR-5, AC-2) | Yes | Yes | PASS — executed: `validate about/index.html: 0 real error(s)`, likewise contact and privacy; 7/7 pages clean |
| `.agents/scripts/site.content.check.mjs` | A dedicated assertion confirms each new page's canonical names its own extensionless URL and its head carries none of the landing page's three entities (AC-2a) | Yes | **Yes (added)** | **PASS (was PARTIAL, now FIXED)** — `assertPageIdentity` asserts both halves per registry entry, deriving the expected canonical from `seo.pages.ts` and the expected entity set from `entitiesFor` in `seo.jsonld.ts`, so the cases come from the build rather than a list. Landing entities are read from the `index` key, not restated. Negative test executed against a mutated copy of `dist`: a retyped entity, a landing entity on a subpage, and a collided canonical each fail, naming the page |
| `.agents/scripts/site.content.check.mjs` | "The content check asserts the three links are present in the pre-rendered landing HTML and in each generated page's footer" (FR-6a, FR-6b, AC-3) | Yes | **Yes (added)** | **PASS (was GAP, now FIXED)** — `assertFooterLinks` reads every registry page's built HTML and requires `href="/about"`, `href="/contact"`, and `href="/privacy"` (the directory-index entries, derived from the registry), plus `href="/"` on every page other than `index.html`. The indirect `TEXT_FLOOR` coverage still catches deletion; this catches retargeting, which is the failure the labels cannot show |
| `.agents/scripts/site.content.check.mjs` | Scan MUST fail on a deliberately introduced bare figure in a new Markdown source, then pass once removed (FR-7, FR-7a) | Yes | Yes | PASS — both halves executed this session; failure names `site/content/about.md:35` |
| `site/lighthouserc.json` | Lighthouse passes and the report MUST be confirmed to contain three new page entries (AC-4a) | Yes | Yes | PASS — two rounds, exit 0, 7 URLs asserted; the three entries confirmed present in `.lighthouseci/lhr-*.json`, not merely assumed from a passing pattern |
| — | Each privacy claim traced by hand to the symbol AC-5 names, tracing recorded (AC-5) | Yes | Yes | PASS — full tracing performed and recorded in the AC-5 row above. Note the CR says "recorded in the pull request"; no PR exists for this branch yet, so the tracing is recorded here and in `69447da` |
| `.agents/scripts/site.contrast.audit.mjs` | Zero failures for the three new pages at all four widths | Yes | Yes | PASS — `contrast-audit: 0 failing text element(s) across 7 pages x 4 widths` |
| `site/build/prerender.mjs`, `site/lighthouserc.json` | Preload/style/stylesheet compared against `concepts.html`; CLS re-measured over two agreeing rounds on one clean build; class closure exercised with a fourth doc-template page (AC-8, AC-8a, AC-8b) | Yes | Yes | PASS — all three exercised independently this session; see AC-8, AC-8a, AC-8b |
| `site/build/release.version.ts`, `site/build/seo.jsonld.ts` | Built version compared against the manifest; grep finds no literal; a build against a missing manifest fails naming the file rather than defaulting (FR-8, AC-6, AC-6a, AC-6b) | Yes | Yes | PASS — all three exercised; the missing-manifest case run against a byte-identical scratch copy so the repository manifest was never renamed |

## Diff Coverage

Merge-base `5c2037b`, HEAD `e310b8a`.

| File | +/- | Mapped Requirements |
|---|---|---|
| `site/build/seo.pages.ts` | +54 / -8 | FR-4, FR-5, AC-2, AC-2a (Phase 1) |
| `site/build/seo.plugin.ts` | +28 / -2 | AC-2a (Phase 1) |
| `site/vite.config.ts` | +18 / -4 | AC-2a (Phase 1) |
| `site/build/seo.jsonld.ts` | +65 / -5 (CR-0077 portion) | FR-4, FR-8, AC-6, AC-6a (Phases 2, 7) |
| `site/build/release.version.ts` | +51 / -0 (new) | FR-8, AC-6, AC-6b (Phase 7) |
| `site/build/doc.pages.ts` | +57 / -24 | FR-1, FR-2, FR-3, FR-6, FR-6b, AC-3 (Phases 3, 4) |
| `site/content/about.md` | +33 / -0 (new) | FR-1, FR-7, AC-4 |
| `site/content/contact.md` | +34 / -0 (new) | FR-2, FR-7, AC-4 |
| `site/content/privacy.md` | +54 / -0 (new) | FR-3, FR-7, AC-4, AC-5 |
| `site/src/components/Footer.tsx` | +25 / -0 | FR-6a, AC-3 (Phase 4) |
| `.agents/scripts/site.pages.mjs` | +17 / -4 | AC-2, AC-4, contrast-audit entry (Phase 5) |
| `.agents/scripts/site.content.check.mjs` | +42 / -4 (Phases 5, 6) plus +201 / -1 (gap fix) | FR-7a, AC-4, AC-7; and AC-1, AC-2a, AC-3 from the gap fix |
| `.agents/scripts/site.serve.mjs` | +10 / -3 (gap fix) | AC-1 |
| `site/lighthouserc.json` | +17 / -2 | AC-4a, AC-8a (Phases 5, 6, 8) |
| `site/build/prerender.mjs` | +51 / -8 | AC-8, AC-8a, AC-8b (Phase 8) |
| `.gitignore` | +11 / -0 | Phase 3 step 4 (8 of 11 lines); see below |
| `docs/cr/CR-0077-agent-trust-anchor-pages.md` | +886 / -0 | The CR itself (authoring, review, Phase 6 amendment, finalization) |

Every Functional Requirement and every Acceptance Criterion now maps to at least
one changed file with a specific hunk. AC-1's serving clause, which mapped to no
changed hunk at the time of the audit, maps to the `site.serve.mjs` resolution
rule added by the gap fix.

### Unmapped changed files

Three files in the branch diff belong to pre-CR Group A work and are correctly
**not** attributed to CR-0077. Verified by commit, not by assumption:

| File | +/- | Commit | Justification |
|---|---|---|---|
| `site/public/404.html` | +63 / -0 | `da19458` | Group A crawler-surface work, predates Phase 1 (`4b6782c`). The CR's Current State section names it explicitly as pre-existing |
| `llms.txt` | +8 / -0 | `da19458` | Group A |
| `internal/docs/llmstxt.go` | +11 / -0 | `ba97e6f` | Group A follow-up (`fix(docs): generate the llms.txt when-to-use section from cmd/gen-llms`). The only Go file on the branch; CR-0077 correctly states it modifies no Go package |

Partially unmapped hunks within mapped files:

| File | Hunk | Justification |
|---|---|---|
| `site/build/seo.jsonld.ts` | `url`, `offers`, `contactPoint` additions | Group A (`da19458`). CR-0077 owns only the three entity builders, the `entitiesFor` branches, and the `softwareVersion` sourcing |
| `.gitignore` | `/resume.sh` entry (3 of 11 lines) | Added by `58041a8`, a CR-0077 checkpoint commit, but not described by Phase 3, whose scope is the three generated output directories. Incidental repository hygiene for a machine-local session script; harmless, but out of the CR's stated `.gitignore` scope |

No stray changed file falls outside either CR-0077's Affected Components or the
identified Group A set.

## Gaps

All four gaps below were closed in a gap-fix pass after the audit. Each retains
the original finding, followed by what was changed and the evidence it now holds.

**Gap 1 — AC-1 (FAIL → FIXED). `site.serve.mjs` did not resolve extensionless paths.**

AC-1 states that serving `site/dist` with `.agents/scripts/site.serve.mjs` MUST
resolve `/about`, `/contact`, and `/privacy` to their directory indexes, matching
GitHub Pages. It returns 404 for all three. The file was never modified on the
branch and never appears in the CR's Affected Components table, so the gap is an
omission carried from planning into implementation rather than a regression.

Production impact is limited: GitHub Pages performs the redirect itself, so the
live site is expected to serve these paths. But nothing in this repository proves
that, and AC-1 was written precisely so the claim would be testable locally.

Minimal fix, in `.agents/scripts/site.serve.mjs` around the resolution at `:52-54`:
after the existing trailing-slash rule, when the requested path has no extension,
retry `join(root, path, 'index.html')` before returning 404. Roughly:

```js
if (path.endsWith('/')) path += 'index.html'
else if (!extname(path)) path += '/index.html'
```

`extname` is already imported at `:22`. Keep the 404 for anything that still does
not resolve, so a missing artefact stays a measurable failure rather than falling
back to a shell.

*Resolution.* Applied exactly as written, at `.agents/scripts/site.serve.mjs:57`,
with the module docstring updated to state the rule and why it exists. Probe over
a clean `site/dist`:

```
/about              200 text/html 44625  <h1>About Outlook Local MCP</h1>
/contact            200 text/html 44754  <h1>Contact</h1>
/privacy            200 text/html 45411  <h1>Privacy</h1>
/about/             200 text/html 44625  (unchanged)
/about/index.html   200 text/html 44625  (unchanged)
/concepts.html      200 text/html 64456  (unchanged)
/nope  /nope/  /assets   404            (unchanged)
```

The 404 path is retained for anything that still does not resolve, including a
bare directory with no index, so the fix widens resolution without softening the
failure signal.

**Gap 2 — Test Strategy row 4 (GAP → FIXED). No footer-link assertion existed.**

The Test Strategy asserts that "the content check asserts the three links are
present in the pre-rendered landing HTML and in each generated page's footer".
No such assertion exists in `site.content.check.mjs`. The links are present today
(verified), and the landing page's `TEXT_FLOOR` rise of 19 characters gives
indirect coverage against deletion, but an `href` retargeted or made external
would pass every current check. This matters more than usual here because the
links are the sole crawl path from `/` to the trust anchor pages, which is the
whole point of FR-6a and FR-6b.

Minimal fix: add one assertion to `site.content.check.mjs` that, for each entry in
`PAGES`, reads the built HTML and requires `href="/about"`, `href="/contact"`, and
`href="/privacy"` (plus `href="/"` for the generated pages, excluding
`index.html`), failing by page name. Derive the expected set from
`site/build/seo.pages.ts` paths rather than a literal list, so a fourth trust
anchor page is covered without editing the script.

*Resolution.* `assertFooterLinks` added to `site.content.check.mjs`. The expected
link set is derived from the registry (`readPageRegistry` parses
`site/build/seo.pages.ts`), so the directory-index entries define the anchors and
a fourth trust anchor page is covered with no script edit. Passing output:
`ok footer links: /about, /contact, /privacy linked from every page, root linked
back from each generated page`. Instrument falsified by substitution against a
mutated copy of `dist`: rewriting the landing page's `href="/about"` to
`href="https://example.com/about"` produces
`FAIL footer links: index.html carries no href="/about" (the footer is the only crawl path to that page)`.

**Gap 3 — Test Strategy row 1 (PARTIAL → FIXED). The content check never drove the
extensionless path.**

`PAGES` names `about/index.html`, so the check exercises the directory-index path
and not the URL AC-1 is written about. Same root cause as Gap 1.

Minimal fix, after Gap 1: add a small assertion driving `/about`, `/contact`, and
`/privacy` through the served origin and requiring HTTP 200 plus a non-empty body.
Keep the existing `PAGES` loop as is; the floors and `<h1>` counts are correct at
the directory-index path and should stay measured there.

*Resolution.* `assertExtensionlessPaths` added, driving every directory-index
registry entry over the served origin and requiring HTTP 200 with a non-empty
body. The `PAGES` loop is untouched, so the floors stay measured where they were
baselined. Passing output: `ok extensionless paths: /about, /contact, /privacy
each resolve to their directory index`.

**Gap 4 — Test Strategy row 3 (PARTIAL → FIXED). AC-2a had no standing assertion.**

The canonical-and-entity separation was verified by hand in Phase 3 and again in
this audit, but nothing guards it. AC-2a's own text explains why that is
uncomfortable: the `basename` collision it was written against "fails silently:
the pages build, render, and validate while every one of them claims to be `/`".
A future change to `pageForFile`, `sitePathForTransform`, or the Rollup input keys
could reintroduce exactly that failure and every current gate would stay green.

Minimal fix: extend `site.content.check.mjs` with an assertion that, for each
registry entry, the built HTML contains `rel="canonical" href="${SITE_ORIGIN}${page.path}"`
and that no page other than `index.html` contains `"SoftwareApplication"`,
`"FAQPage"`, or `"Organization"`. Both facts are already available from
`site/build/seo.pages.ts`, so the check derives its cases from the registry rather
than from a list.

*Resolution.* `assertPageIdentity` added, going one step further than the sketch
above: as well as the canonical and the absence of the landing page's entities, it
asserts that each page's top-level JSON-LD entities are **exactly** the set its
registry key declares. The expected set is read from `entitiesFor` in
`site/build/seo.jsonld.ts` and the landing entities from the `index` key, so no
entity name is transcribed into the check. Passing output: `ok page identity: 7
pages, each with its own canonical and exactly its assigned entities`. Falsified
by substitution against a mutated copy of `dist`, which produced all three failure
classes the criterion is about:

```
FAIL page identity: about/index.html declares entities [FAQPage], expected exactly [AboutPage] from its registry key "about"
FAIL page identity: about/index.html asserts the landing page's "FAQPage" entity, which only / may state
FAIL page identity: privacy/index.html canonical is "https://outlook-local-mcp.com/", expected "https://outlook-local-mcp.com/privacy"
```

---

None of the four gaps required a change to the trust anchor pages themselves. All
four were harness work: one behaviour fix in `site.serve.mjs` and three assertions
in `site.content.check.mjs`, all now applied and verified.

### Gap-fix verification

| Gate | Command | Result |
|---|---|---|
| Site build | `rm -rf site/dist site/dist-ssr && pnpm --dir site run build` | exit 0, `prerender: optimised 6 doc-page documents` |
| Content check | `node .agents/scripts/site.content.check.mjs` | exit 0, `content-check: all assertions hold`, including the three new assertions |
| Negative test | same check against a mutated copy of `dist` | `content-check: 4 failing`, each naming its page — the three new assertions all fire |
| Serve probe | direct HTTP probe through `site.serve.mjs` | `/about`, `/contact`, `/privacy` → 200; trailing-slash, `.html`, and 404 behaviour unchanged |

The instrument was validated before its results were trusted: the content check
was run twice over the unchanged build and returned identical character counts for
all seven pages, and each new assertion was falsified by substitution rather than
accepted on a passing run alone.

### Left as noted

The `/resume.sh` entry in `.gitignore`, flagged above as outside the CR's stated
`.gitignore` scope, is left in place. `resume.sh` is a machine-local session
script that exists in the working tree; removing the ignore line would make it
show as untracked and invite an accidental commit, which is a worse outcome than
three lines of incidental hygiene. No CR-0077 requirement depends on it either
way.
