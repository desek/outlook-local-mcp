---
date: 2026-07-30
branch: dev/site-v3
commit: 22c9bc2
---

## Question

Deep-research (fan-out web search, source fetch, 3-vote adversarial verification,
synthesis): the current state, standards, best practices, and frontier methods for
SEO and GEO (Generative Engine Optimization) as of mid-2026, for a small
open-source developer-tool project site served as static HTML on GitHub Pages,
as a complement to
`docs/cr/CR-0070-site-v3-adoption-with-seo-geo-optimization-and-ci-deployment.md`.

Sub-questions: (1) what actually causes an AI answer engine to cite a page;
(2) which AI crawlers execute JavaScript; (3) `llms.txt` and Markdown parallel
representations; (4) JSON-LD / schema.org value for GEO and the current status of
FAQPage, HowTo, and SoftwareApplication; (5) the 2026 AI crawler agent inventory
and robots.txt; (6) Core Web Vitals and Lighthouse; (7) frontier methods.
Explicit instruction to flag folklore and vendor marketing, and to call out both
gaps in CR-0070 and anything it mandates that the evidence says is low-value.

Run: 105 agents, 23 sources fetched, 114 claims extracted, 25 verified by 3-vote
adversarial verification (18 confirmed, 7 refuted).

## Response

### Headline

The evidence is **deflationary**. What makes an AI answer engine cite a page is
overwhelmingly (a) being retrievable at all, meaning indexed, crawlable, and
present in the initial HTML, and (b) topical relevance plus position within the
retrieved context. It is not any content-styling "GEO stack".

CR-0070's single highest-value requirement is **pre-rendering** (FR-8 to FR-12).
Its second-highest is a **robots.txt that names the retrieval-time agents**, which
the current FR-18 list gets partly wrong. Several of its structured-data and
machine-readable-file mandates are cheap but evidence-free.

### 1. GEO fundamentals: mostly folklore

**Confirmed (high confidence).** The published GEO evidence base does not support
the premise that on-page content tactics measurably increase retrieval or citation
in production engines.

* The foundational GEO paper (Aggarwal et al., KDD '24, arXiv 2311.09735) measured
  its famous gain (19.3 → 27.2 Position-Adjusted Word Count) **only inside a fixed
  five-document context window where the optimized page was already retrieved**.
  Retrieval was never a variable. Cite it as "the original GEO study found", never
  as a current measurement: it was evaluated against GPT-3.5/GPT-4-era engines.
* A July 2026 survey of 45 studies (arXiv 2607.14035) reports **no technique with a
  stable, longitudinal, cross-platform causal effect on organic discoverability**.
* C-SEO Bench (Puerto et al., NeurIPS 2025 D&B; 9 methods x 6 domains, 1,921
  queries, Wilcoxon signed-rank with Holm-Bonferroni) found **only 3 of 54
  method-domain combinations statistically significantly positive**. The
  "Statistics" method *decreased* rankings in 19 of 24 settings; 26 of 30
  product-recommendation cases were significantly negative on Haiku 3.5. It also
  finds traditional SEO, meaning improving the source's rank into the LLM context,
  substantially more effective than any content-level tactic.

**Partially supported (medium).** Keyword stuffing imported from classic SEO does
not work and may underperform baseline. Formatting-only changes have weak effects.
Embedding concrete extractable facts (explicit versions, dates, counts,
definitions) measurably helps in controlled trials, but as a second-order lever
behind relevance and context position, with gains that erode as adoption spreads.

**Refuted, 0-3 or 1-2 against. Do not reuse these numbers.** They are exactly what
circulates in vendor GEO marketing:

* "citations / quotes / statistics yield 30-40% improvement" (0-3)
* "fluency optimization gives a 15-30% boost" (0-3)
* "GEO disproportionately benefits low-ranked sites; Cite Sources +115.1% for
  rank-5" (0-3)
* "only 2.9% of AI citations point at the brand's own domain" (0-3)
* the brand-tier visibility ladder 72.9 / 44 / 11.4% (0-3)
* "listicles account for 35.7% of content citations" (1-2)

**Source-quality flag.** The July 2026 survey is a single-author, unreviewed
preprint with no stated affiliation, 15 days old at research time, and the author
discloses that "the original search did not retain database-specific hit counts or
a complete exclusion ledger". Attribute it explicitly, never as established fact,
and pair its null finding with its own positive finding that query-document
relevance and context position *are* reproducible factors.

### 2. JavaScript rendering: this is the load-bearing requirement

**Confirmed (high).** Pre-rendering is effectively required for the OpenAI,
Anthropic, Perplexity, Meta, and ByteDance direct-fetch path. These crawlers
download JavaScript files but show no evidence of executing them, so
client-side-rendered content is invisible to them.

* Vercel + MERJ instrumented ~569M GPTBot and ~370M ClaudeBot requests: GPTBot
  fetched JS in ~11.5% of requests, ClaudeBot ~23.84%, with zero evidence of
  execution, the fingerprint of a non-rendering fetcher.
* **Google and Apple are the exceptions.** Gemini and AI Overviews inherit
  Googlebot's Web Rendering Service; Applebot renders through a browser-based
  crawler ("Applebot may render the content of your website within a browser",
  Apple's own docs). Martin Splitt has confirmed Google-Extended renders like
  Googlebot, which is mechanically expected since Google-Extended is a robots.txt
  control token, not a separate fetcher.
* Independent mid-2026 re-tests (searchviu log analysis; Glenn Gabe's case study of
  CSR sites losing ChatGPT/Perplexity/Claude visibility) re-confirm non-rendering
  for GPTBot, OAI-SearchBot, ChatGPT-User, ClaudeBot, Claude-SearchBot,
  PerplexityBot, and Meta-ExternalAgent as of June/July 2026.

**Important honesty caveat (confirmed 3-0).** **No vendor primary documentation
confirms or denies JS execution.** OpenAI's crawler docs are entirely silent on
rendering. The requirement rests on third-party telemetry, principally a single
vendor's edge network, dated January 2025, from a company with a commercial
interest in SSR. CR-0070 should cite it as such rather than implying vendor
confirmation.

Scope limit worth recording: OpenAI's **agentic** surfaces (ChatGPT agent mode, the
Atlas browser) do drive a real Chromium-class browser and **do** execute JS. Do not
generalize "OpenAI never renders JS" from crawler silence.

### 3. robots.txt and the agent inventory: CR-0070's FR-18 list is wrong

**Confirmed (high, 3-0 on four separate claims).** Vendors separate **training**
tokens from **retrieval-for-citation** tokens, and robots.txt directly controls
citation-time retrieval.

| Vendor | Agent | Controls |
|---|---|---|
| OpenAI | `OAI-SearchBot` | surfacing sites in ChatGPT search |
| OpenAI | `ChatGPT-User` | user-initiated fetch |
| OpenAI | `GPTBot` | **training only** |
| OpenAI | `OAI-AdsBot` | ads |
| Anthropic | `Claude-User` | user-initiated retrieval; blocking "prevents our system from retrieving your content in response to a user query" |
| Anthropic | `Claude-SearchBot` | search indexing |
| Anthropic | `ClaudeBot` | **training only** |
| Perplexity | `PerplexityBot` | indexing |
| Perplexity | `Perplexity-User` | user-initiated; **generally ignores robots.txt** |

OpenAI's doc explicitly describes allowing `OAI-SearchBot` while disallowing
`GPTBot`. Anthropic's doc (updated 2026-04-07) documents three agents with three
distinct consequences.

**Direct implication for FR-18.** The CR currently names GPTBot, ClaudeBot,
PerplexityBot, Google-Extended, and OAI-SearchBot. Two of those five (**GPTBot**,
**ClaudeBot**) are *training* tokens, so listing them is a training-consent
decision, not a citation lever. The citation-relevant agents **missing** from the
list are **`ChatGPT-User`**, **`Claude-User`**, and **`Claude-SearchBot`**.
`Applebot-Extended` is also worth adding given Applebot renders.

Two further qualifications:

* **Permission is necessary but not sufficient.** ChatGPT Search draws heavily on
  the Bing index; a page absent from it is uncitable regardless of robots.txt. (The
  often-quoted Brave/Claude "86.7% overlap" figure is unverified vendor
  methodology; do not cite it.)
* **robots.txt is a preference signal, not access control.** Perplexity's own doc:
  "Since a user requested the fetch, this fetcher generally ignores robots.txt
  rules." Cloudflare's 2025-08-04 report documented Perplexity using undeclared
  crawlers with rotating user-agents/IPs/ASNs, and de-listed it from the
  verified-bot list. ClaudeBot has been publicly reported continuing to request
  globally-disallowed URLs. Immaterial for a project that *wants* citation, but the
  CR must not describe robots.txt as access control.
* robots.txt defaults to allow, so explicit `Allow` blocks are documentation rather
  than a technical requirement. That is a fine reason to keep them; it is not a
  functional one.

### 4. Structured data: correct the CR's claims

**Confirmed (high, 3-0).**

* **`SoftwareApplication` remains an actively supported Google rich result**
  (software-app page updated 2025-12-10, no deprecation banner; still listed in the
  structured-data gallery updated 2026-06-15). **But a self-published dev tool
  cannot qualify**: Google requires `name`, `offers.price` (0 for free), **and one
  of `aggregateRating` or `review`**, and self-publishing a rating on one's own
  site violates Google's self-serving review policy. Include
  `offers.priceCurrency`, a common validator warning even at price 0.
* **`FAQPage` is fully dead**, not merely restricted: deprecation notice
  2026-05-07; rich results stopped appearing; the appearance filter, rich result
  report, and Rich Results Test support were removed in June 2026.
* **`HowTo` was retired in 2023.**

**Confirmed (high, 3-0 and 2-1). Google explicitly denies that any AI-specific
optimization is required** for AI Overviews or AI Mode:

> "There are no additional requirements to appear in AI Overviews or AI Mode, nor
> other special optimizations necessary." … "You don't need to create new machine
> readable files, AI text files, or markup to appear in these features. There's
> also no special schema.org structured data that you need to add."
> — Google Search Central, AI features (updated 2025-12-10)

The newer dedicated guide (published 2026-05-15, updated 2026-07-10) strengthens
it: "Structured data isn't required for generative AI search"; "There's no
requirement to break your content into tiny pieces"; Google may crawl `llms.txt`
like any other page but "this doesn't mean that the file is treated in a special
way", and it "won't negatively or positively impact your visibility or rankings".

Bounding caveats: this is a statement about **eligibility**, not selection
probability (SE Ranking found only ~14% of AI Mode cited URLs rank in the organic
top 10, versus Ahrefs' 76% for AI Overviews across 1.9M citations); Google is an
interested party describing its own unaudited systems; and it says nothing about
ChatGPT, Perplexity, or Claude. Counter-claims of schema-driven lift ("2.5x higher
chance", "40% more AI Overview appearances") come from vendor blogs with no
disclosed methodology. **Flag as marketing folklore.**

**Recommendation for the CR.** Keep `SoftwareApplication` and `Organization` for
entity and knowledge-graph value and for LLM parsing. Keep `FAQPage` and `HowTo`
only on a speculative LLM-parsing rationale, and **say so honestly in the CR**
rather than implying rich-result eligibility, which FR-41 and FR-42 currently do by
omission. AC-8's Rich Results Test assertion is now partly meaningless for FAQPage
(tooling support removed) and will never pass for SoftwareApplication.

The `contributor` vs `sponsor` reasoning in FR-43 was not separately challenged and
stands.

### 5. llms.txt and Markdown representations: unjustified but not condemned

* Google's position is explicitly **neutral**: crawled like any other page, no
  special treatment, no ranking effect either way.
* SE Ranking's ~300k-domain study found **no clear effect on AI citations**.
* **No vendor other than Google was shown to have addressed `llms.txt` at all.** No
  statement either way from OpenAI, Anthropic, or Perplexity surfaced.
* **No evidence surfaced that AI crawlers prefer or benefit from Markdown parallel
  paths** (`/index.md`, `Vary: Accept`, `text/markdown`). Google explicitly says
  Markdown is not needed.

So FR-20 (`llms.txt`) and FR-31 to FR-35 (`/index.md`, SVG to Mermaid) are
**neither justified nor condemned by evidence**. They are cheap, low-risk, and
plausibly forward-looking, and the Mermaid conversion has a genuine
first-principles argument (a flattened SVG carries no meaning; a Mermaid fence
does). But the CR's Motivation section currently asserts a "structural advantage no
amount of metadata provides" that **the evidence does not support**. That sentence
should be softened to a bet rather than a finding.

### 6. Core Web Vitals and Lighthouse: no GEO justification found

**Zero claims survived verification in this area.** Nothing in the surviving
evidence connects performance to AI citation on any engine. Treat FR-36 to FR-38
and NFR-1 (the 95/95/95/100 gate) as **unvalidated by this research** and justified
on user-experience and classic-SEO grounds, which is a perfectly good
justification. The CR should just not imply a GEO rationale for it.

### 7. What the research could NOT establish

Recorded so these are not mistaken for settled:

* Core Web Vitals / Lighthouse in 2026 (INP thresholds, React+Vite practices).
* `llms.txt` effectiveness outside Google.
* Markdown parallel representations for any engine.
* Off-site brand mention and co-occurrence effects, AI referral tracking, and
  MCP-registry discovery surfaces. Claims here were **refuted** or failed
  verification, so nothing can be asserted.
* The IETF AI-preferences work / Cloudflare Content Signals Policy;
  `Applebot-Extended`, `Bytespider`, `CCBot`, `meta-externalagent`.
* Build-provenance meta tags; canonical / OG / Twitter card value for AI surfaces.
* Semantic chunking for RAG retrieval (Google denies it is needed; no non-Google
  evidence surfaced).

### Open questions worth carrying into the follow-up CR

1. Does ChatGPT Search's dependence on the Bing index mean the site must be
   separately verified in Bing Webmaster Tools or submitted via IndexNow to be
   citable at all? Robots.txt permission is necessary but demonstrably not
   sufficient. **This is arguably a missing requirement in CR-0070**, whose Phase 5
   covers Search Console and Bing registration but not IndexNow.
2. Is there any measurement that pre-rendering a previously-CSR site produces a
   measurable *increase* in citations, as opposed to the well-established negative
   fact that CSR content is not fetched? The causal direction is assumed, not shown.
3. What is the retrieval value of `llms.txt` and `.md` paths for the non-Google
   engines? No vendor statement either way exists. This is the single largest open
   item for FR-20 and FR-31 to FR-35.
4. Do Core Web Vitals influence AI citation on any engine? Nothing connects them.
5. For a niche open-source dev tool, does discovery actually flow through MCP-server
   registries and GitHub/Reddit/HN co-occurrence rather than the project site? The
   specific claims in this area were refuted, but the underlying question — where
   the citation supply for an unknown small project comes from — remains
   unanswered and **may dominate every on-page tactic in the CR**.

### Time sensitivity

Crawler behavior, agent-token inventories, and Google rich-result support all
changed materially inside the 2025-2026 window (FAQPage deprecated May 2026;
`OAI-AdsBot` added; Anthropic docs updated 2026-04-07). Re-verify against vendor
primaries before any of this hardens into a requirement.

### Primary sources

| Source | Quality |
|---|---|
| https://arxiv.org/pdf/2311.09735 (GEO, KDD '24) | primary |
| https://arxiv.org/html/2607.14035v1 (Jul 2026 survey, 45 studies) | primary, unreviewed preprint |
| https://arxiv.org/abs/2506.11097 (C-SEO Bench, NeurIPS 2025) | primary |
| https://vercel.com/blog/the-rise-of-the-ai-crawler | primary telemetry, single-vendor |
| https://developers.openai.com/api/docs/bots | primary |
| https://support.claude.com/en/articles/8896518-... | primary |
| https://docs.perplexity.ai/docs/resources/perplexity-crawlers | primary |
| https://support.apple.com/en-us/119829 (Applebot) | primary |
| https://developers.google.com/search/docs/appearance/ai-features | primary |
| https://developers.google.com/search/docs/fundamentals/ai-optimization-guide | primary |
| https://developers.google.com/search/docs/appearance/structured-data/software-app | primary |
| https://developers.google.com/search/docs/appearance/structured-data/search-gallery | primary |
| https://web.dev/articles/vitals, .../defining-core-web-vitals-thresholds | primary |
| https://registry.modelcontextprotocol.io/docs | primary |
| SE Ranking 300k-domain llms.txt study (via SEJ) | secondary |
| Ahrefs AI search overlap / AI Overview brand correlation | blog |
| https://dri.es/markdown-llms-txt-and-ai-crawlers | blog |
