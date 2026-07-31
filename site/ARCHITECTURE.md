# ARCHITECTURE.md — Outlook Local MCP Product Page
# Design Language: Terminal Industries
# Generated: 2026-04-06

---

## 2a. Section Architecture (Intent-Driven Map)

### Overview — Page Narrative Arc

The page follows a five-beat developer conversion funnel:
1. **Hook** — What this is and why "no Entra ID app registration" is genuinely rare
2. **Capability proof** — 23 tools in 5 clusters, scannable and trustworthy
3. **Trust signals** — Privacy model, OS-native storage, enterprise internals
4. **Getting started** — Install command → paste config → done
5. **Reference** — Full tool and config tables, collapsed by default

**10-second test:** A developer who lands on this page and spends only 10 seconds must leave knowing: (a) this is a local MCP server for Outlook calendar/mail, (b) it requires ZERO ENTRA ID setup, (c) they can install it with one `go install` command. These three facts must be visible above the fold without scrolling.

---

### Section 0 — Navigation / Header

**Purpose:** Persistent orientation and one-tap CTA access.

**Editorial framing:** Adapts Terminal's frosted-glass pill nav. Desktop shows wordmark + nav links + solid CTA button. Mobile shows wordmark + hamburger.

**Structural layout:**
- `position: fixed; top: 0; left: 0; right: 0; z-index: 50`
- Desktop: centered pill, max-width 960px, height 78px, padding 18px 24px
- Mobile: centered pill, width calc(100% - 38px), height 64px, padding 20px 26px
- Pill: `background: rgba(0,0,0,0.3); backdrop-filter: blur(30px); border-radius: 8px`

**Desktop nav items (left to right):**
- Wordmark: `outlook-local-mcp` in Inter, white
- Text links: "Features", "Docs", "GitHub"
- CTA button: "INSTALL NOW" — `background: #ffffff; color: #052424; border-radius: 8px; padding: 12px 32px; font-size: 11px; font-weight: 600; letter-spacing: 1.5px`

**Mobile nav:** Wordmark + hamburger (44×44px touch target — fix Terminal's 24px failure). Drawer opens full-screen `rgba(0,0,0,0.95)` with: Features, Docs, GitHub links stacked, plus a full-width "INSTALL NOW" button at the bottom.

**Interactive elements:**
- Hover on text links: text color → `#abff02`, lime dot (4px) appears below label
- Focus outline: `2px solid #abff02`
- ARIA: `aria-expanded` on hamburger

**Responsive behavior:**
- Desktop ≥1024px: full pill with all nav links
- Mobile <1024px: compact pill, hamburger drawer

**Text contrast:** White text on `rgba(0,0,0,0.3)` blur achieves effective ~6:1 contrast against the underlying content.

---

### Section 1 — Hero (HERO tier)

**Purpose:** Hook. Answer "what is this, why does it matter" in under 5 seconds. Deliver the install command above the fold on desktop.

**Editorial framing:** No Entra ID app registration is the headline differentiator — it removes a multi-hour blocker most developers assume they'll face. Lead with that. "100% local" is the trust signal. "23 tools" is the breadth signal. Together these three facts form the hero's payload.

**Headline (≤60 chars):** `Your Outlook calendar, inside your AI.`
_(38 chars — passes hero brevity rule)_

**Subheadline:** `No Entra ID setup. No cloud middleman. Zero-config auth.`

**Body (below fold, visible on scroll-in):**
```
A Model Context Protocol server that connects Claude — or any MCP client — directly to Microsoft Calendar and Mail via the Graph API. All data stays on your machine. OAuth tokens live in your OS keychain. The server process never leaves localhost.
```

**Three stat callouts (visual badges):**
- `100% LOCAL` — no intermediate servers
- `ZERO ENTRA ID` — pre-authorized Microsoft first-party client ID
- `23 MCP TOOLS` — calendar, mail, multi-account, diagnostics

**Primary CTA:** Copy-able install command block:
```
go install github.com/desek/outlook-local-mcp/cmd/outlook-local-mcp@latest
```
Rendered as a dark `#0a1010` pill with monospace font, a copy icon (right edge), and a subtle `#abff02` left-border accent.

**Secondary CTA:** Text link below: `Download Claude Desktop extension (.mcpb) ↗`

**Visual background — adapted from Terminal's cinematic truck photography:**
- Background: A terminal window / dark code editor at golden-hour ambient light — deep blacks with warm amber light leaking in from the left edge, suggesting a developer at night. The "window" framing replaces the truck-on-highway framing but maintains Terminal's cinematic atmosphere.
- Upper 55% of viewport: atmospheric dark gradient, warm amber accent light from left, `#0d1a0d` → `#050d05` → `#000000`
- Lower 45% fades to pure black `#000000`
- At scroll=0, only the background is visible — headline text enters as user begins scrolling (Terminal's pattern)

**Scroll-to-explore indicator:**
- Text: `SCROLL TO EXPLORE` — Geist Mono, 11px, weight 600, white, positioned at bottom of hero viewport

**Structural layout:**
- Full-bleed section: `height: 100vh; overflow: hidden; position: relative`
- Headline text: centered, Inter weight 400, white, `font-size: clamp(2.5rem, 5.09vw, 73.33px)`
- Stat badges: horizontal row centered below subheadline, gap 32px
- Install command block: max-width 640px, centered, positioned below stat badges

**Interactive elements:**
- Copy button on install command block: click copies to clipboard, icon flips to checkmark for 2s
- Secondary CTA link: hover → `color: #abff02`

**Responsive behavior:**
- Desktop: full-bleed background, two-column stat badges, install block centered at max-width 640px
- Mobile: stacked layout, stat badges wrap to single column, install block full-width with horizontal scroll disabled

**Text-over-visual contrast strategy:**
- Headline and subheadline rendered over the dark (bottom 50%) portion of the background
- Fallback: `text-shadow: 0 2px 20px rgba(0,0,0,0.8)` on headline
- Install block uses an opaque `rgba(5,36,36,0.85)` background pill — not transparent

**Scroll journey:**
- At Y=0: background only, "SCROLL TO EXPLORE" indicator
- Y=0→300: headline + subheadline fade in (scroll-bound opacity via GSAP ScrollTrigger scrub)
- Y=300→600: stat badges stagger in
- Y=600→900: install command block + CTAs enter
- Total hero scroll travel: ~1400px desktop, ~1800px mobile

**Section transition:** Notch-corner SVG mask at bottom of hero — convex protrusions cutting into the white introduction section below.

---

### Section 2 — Introduction: "What this actually is" (HERO tier, secondary)

**Purpose:** Bridge from cinematic hook to technical credibility. One punchy statement followed by the "why it's different" payload.

**Editorial framing:** Equivalent to Terminal's "Imagine the yard as an intelligent bridge…" section. Replaces physical metaphor with technical metaphor. Uses a large, confident display heading with a scroll-triggered typed animation on the key differentiator word.

**Heading (display, weight 400):**
```
Connect Claude to your calendar.
No servers. No registration.
Just your data.
```

**Animated word:** "directly" (or "locally") types in character by character — lime `#abff02` during typing, then transitions to `#052424` on completion. Same pattern as Terminal's "intelligent" animation.

**Supporting detail panel (right side, desktop):**
- Three-column fact grid:
  - `OAuth 2.0` / Device code, browser, or PKCE
  - `OS Keychain` / macOS, Linux, Windows native storage
  - `Graph API v1.0` / Calendar + Mail scopes only

**Structural layout:**
- Background: white `#ffffff`
- Content: 45% left text / 55% right detail panel split at desktop
- Mobile: stacked single column, text above, fact grid below
- Notch-corner SVG masks: upper corners of this section (same as Terminal's intro-to-white transition)

**Interactive elements:** None — purely editorial.

**Responsive behavior:**
- Desktop ≥1024px: two-column split; fact grid on right with notch-corner mask
- Mobile <1024px: stacked, full-width fact grid as 1-column cards

---

### Section 3 — Showcase: Core Capabilities (SHOWCASE tier — Scroll-Pinned)

**Purpose:** Prove the 23-tool breadth without overwhelming. Each capability cluster is a distinct selling point with an atmospheric visual.

**Editorial framing:** Adapts Terminal's scroll-pinned "features-steps" section. Left panel: five numbered capability clusters. Right panel: atmospheric terminal/calendar visual that changes per active cluster. The numbered list replaces "Autonomous agentic workflows gate to dock" with developer-relevant capabilities.

**Five capability clusters:**

```
01  CALENDAR MANAGEMENT
    14 tools — read, search, create, update, delete, free/busy.
    Extra safety: meeting tools include attendee confirmation warnings.

02  MULTI-ACCOUNT
    Add, list, remove accounts at runtime.
    Lazy auth — no credentials required at startup.

03  MAIL ACCESS (opt-in)
    4 read-only tools. KQL full-text search.
    Enabled with one env var: OUTLOOK_MCP_MAIL_ENABLED=true

04  LOCAL PRIVACY & SECURITY
    OS keychain token storage. AES-256-GCM file fallback.
    OData injection protection. PII-sanitized logs.

05  ZERO-CONFIG AUTH
    Three auth methods. ZERO ENTRA ID registration.
    Token expiry ~90 days. Silent refresh automatic.
```

**Right-panel visuals (one per cluster — see Section 2d for asset details):**
- 01: Terminal window showing calendar events in JSON output from Claude
- 02: Multi-account switcher UI diagram, matrix/grid aesthetic
- 03: Mail search query UI in dark terminal, KQL syntax highlighted in lime
- 04: Architectural diagram — local machine boundary, keychain storage, Graph API outbound arrow only
- 05: Auth flow diagram — device code URL display, browser OAuth dance, PKCE sequence

**Structural layout:**
- Desktop: pinned left panel (45% width) / right media panel (55%, bleeds to viewport edge)
- Left panel: numbered list, active item at `30.67px` Inter weight 450 `#052424`, inactive dimmed
- Right panel: notch-corner SVG mask on left edge (~80px radius)
- Mobile: NOT pinned — horizontal tab strip (01–05) at top, media above text, natural scroll

**Interactive elements:**
- Desktop: scroll-driven active state — each cluster activates at intervals of ~380px scroll travel
- Mobile: tap tab number → switches cluster, text + visual update with cross-fade ~0.3s

**Responsive behavior:**
- Desktop ≥1024px: GSAP ScrollTrigger pin, `height: 100vh; overflow: hidden` on trigger wrapper, separate desktop/mobile ref arrays
- Mobile <1024px: tab strip, stacked layout, `column-reverse` flex, media above text

**Section transition:** Bottom of right panel has notch-corner SVG mask, leading into the dark "Brand Reveal" section.

---

### Section 4 — Brand Reveal: "Outlook Local MCP" (SHOWCASE tier — Scroll-Pinned)

**Purpose:** The brand moment. Equivalent to Terminal's "Yard Operating System." → "YOS™" morph. Creates the cinematic pause that earns the product name its weight.

**Editorial framing:** "That's" → full product name → abbreviation morph. Replaces "Yard Operating System." with the brand statement for this product.

**Text sequence (scroll-driven):**
1. Pre-label fades in: `"That's"` — Inter 18px, weight 400, `rgba(255,255,255,0.5)`, centered
2. Full name appears: `"Outlook Local MCP."` — large display text, white, weight 400
3. Morphs to: `"OL-MCP"` or stays as `"Outlook Local MCP"` — the morph compresses text until it fills the viewport width as a single typographic statement
4. Background transitions: dark green `#052424` → off-white `#f0f0f0` (scroll-driven)
5. Text transitions: white → dark `#030000` simultaneously with background

**Background:** Dark forest green `#052424` with CSS dot-grid overlay:
- Grid cells: ~80px squares
- Grid lines: `rgba(171,255,2,0.08)` (extremely faint)
- Dot intersections: `#abff02` at 3×3px square dots

**Heading sizes:**
- Desktop: `110px`, Inter, weight 400, white
- Mobile: `41px`, Inter, weight 400, white

**Structural layout:**
- Desktop: pinned section, `height: 100vh; overflow: hidden` on trigger, ~1100px total scroll travel
- Mobile: static (not pinned), natural scroll ~1600px height, animation via IntersectionObserver

**Interactive elements:** None — purely cinematic.

**Responsive behavior:**
- Desktop ≥1024px: GSAP ScrollTrigger pin + scrub, background tween + text morph
- Mobile <1024px: IntersectionObserver triggers animation, static layout

---

### Section 5 — Showcase: Getting Started (SHOWCASE tier — Full)

**Purpose:** Actionable zero-friction path from page to running tool. The developer should be able to copy-paste their way to a working install.

**Editorial framing:** Three steps: Install → Configure → First Run. Each step is minimal. Config snippets are the primary value. Presented as a numbered step flow, not prose.

**Step layout:**

**STEP 1: INSTALL**
Four tabs (Go / Docker / Claude Desktop / Build):
```bash
# Go (recommended)
go install github.com/desek/outlook-local-mcp/cmd/outlook-local-mcp@latest

# Docker
docker pull ghcr.io/desek/outlook-local-mcp:latest
```
Claude Desktop: "Download `.mcpb` from GitHub Releases" → button link
Build from source: collapsed by default (expandable "Build from source" accordion)

**STEP 2: CONFIGURE**
Two sub-tabs (Claude Desktop / Claude Code):
```json
{
  "mcpServers": {
    "outlook-local": {
      "command": "outlook-local-mcp",
      "env": { "OUTLOOK_MCP_DEFAULT_TIMEZONE": "America/New_York" }
    }
  }
}
```

**STEP 3: FIRST RUN**
Prose: "No credential setup before first use. On first tool call, a device code URL displays. Complete auth once in a browser. Tokens cached in your OS keychain for ~90 days."
Visual: Small animated terminal mockup showing the device code URL output.

**Structural layout:**
- Background: white `#ffffff`
- Steps: numbered 01/02/03 vertical flow, generous spacing
- Code blocks: dark `#0a1010` background, Geist Mono, syntax highlighting — comment lines in `rgba(171,255,2,0.6)`, strings white, keywords `#abff02`
- Step numbers: Geist Mono 11px, `#abff02`

**Interactive elements:**
- Copy button on each code block
- Install method tabs (Go / Docker / Claude Desktop / Build) — active tab border-bottom `#abff02`
- Config sub-tabs (Claude Desktop / Claude Code)
- "Build from source" accordion (collapsed by default)

**Responsive behavior:**
- Desktop: two-column for step number + content; code block max-width 640px centered
- Mobile: single column, full-width code blocks with `overflow-x: auto`

---

### Section 6 — Showcase: Platform Support (SHOWCASE tier — Compact)

**Purpose:** Reassure developers their platform is supported. Compact, scannable.

**Editorial framing:** Presented as a platform badge row with a compact platform support grid. Pairs with the "Install" section visually.

**Content:**
```
macOS Apple Silicon   ✓ arm64 binary  ✓ Docker linux/arm64
Linux x86_64          ✓ amd64 binary  ✓ Docker linux/amd64
Linux ARM             ✓ arm64 binary  ✓ Docker linux/arm64
Windows               ✓ amd64 binary  —
```
Docker image: `ghcr.io/desek/outlook-local-mcp:latest` — scratch-based, < 20 MB

**Structural layout:**
- Background: dark `#052424` with faint dot-grid (matches Brand Reveal section aesthetic — visual bookending)
- Platform grid: 3-column table (Platform / Binary / Docker), Inter 14px
- Monospace values in Geist Mono `#abff02`
- Section height: ~300px desktop, ~400px mobile

**Interactive elements:** None.

**Responsive behavior:**
- Desktop: horizontal 3-column table
- Mobile: collapsed card per platform, accordion expand

---

### Section 7 — Trust: Local Privacy & Security (SHOWCASE tier)

**Purpose:** Allow a developer to explain this tool to their security team. Surface verifiable, auditable facts.

**Editorial framing:** Adapts Terminal's social proof / "Built by" section. Replaces partner logos with a technical trust architecture diagram showing: local machine → OS keychain storage → outbound to Graph API only. No inbound servers, no third parties.

**Content clusters:**
- Token storage: OS Keychain (macOS / Linux / Windows) + AES-256-GCM file fallback
- Network: only two outbound endpoints (Graph API + Microsoft Identity Platform)
- Logging: PII sanitization enabled by default, structured JSON, optional audit log per tool invocation
- Safety: OData injection protection, optional read-only mode (`OUTLOOK_MCP_READ_ONLY=true`)
- Observability: OpenTelemetry (OTLP gRPC) optional — zero overhead when disabled

**Visual:** Architecture diagram (Canvas 2D asset — see Section 2d): local machine box with labeled internal components (MCP Client → MCP Server → OS Keychain, Token Cache), single outbound arrow to "Microsoft Graph API". No inbound arrows. Clear boundary box around local machine.

**Structural layout:**
- Background: white `#ffffff`
- Left panel (45%): headline + bullet list with lime `#abff02` left-border accents
- Right panel (55%): architecture diagram visual, notch-corner mask on left edge
- Mobile: stacked, diagram below text

**Interactive elements:**
- Bullet items on hover: left-border brightens, subtle background `rgba(171,255,2,0.05)`

**Responsive behavior:**
- Desktop: 45/55 split
- Mobile: stacked single column

---

### Section 8 — Showcase: Tech Stack (REFERENCE tier — Compact Inline)

**Purpose:** Satisfy developers who evaluate tools by inspecting internals. One compact metadata block, not a full section.

**Editorial framing:** Presented as a horizontal metadata bar or grid of property:value pairs. Compact — occupies ~200px vertical space on desktop.

**Content:**
```
Language       Go 1.24+
Protocol       MCP (JSON-RPC over stdio)
API            Microsoft Graph API v1.0
Auth           OAuth 2.0 — device code, browser, PKCE
Token Storage  OS keychain + AES-256-GCM file fallback
Logging        Structured JSON, per-tool audit log, OpenTelemetry (OTLP gRPC)
License        MIT
```

**Structural layout:**
- Background: `#f0f0f0` (off-white, Terminal's "dirty white")
- Three-column grid of property cards
- Property label: Geist Mono 11px, `#abff02`, letter-spacing 1.98px
- Value: Inter 14px, `#052424`
- Mobile: two-column grid

---

### Section 9 — Reference: Full Tool List (REFERENCE tier — Collapsed)

**Purpose:** Allow developers to audit the full 23-tool surface area before adopting.

**Editorial framing:** Collapsed accordion by default. Eyebrow says "23 TOOLS". Expand reveals grouped tables by category. This content is for the developer doing due diligence — not the first-pass evaluation.

**Structural layout:**
- Background: white `#ffffff`
- Accordion container: border-bottom `1px solid rgba(5,36,36,0.1)`
- Accordion trigger: "23 MCP TOOLS — VIEW FULL REFERENCE" — Geist Mono 11px, `#052424`, letter-spacing 1.98px, with `+` icon right edge
- Expanded state: four grouped tables (Account, Diagnostics, Calendar, Mail)
- Table header: Geist Mono 11px, `#abff02`
- Table rows: Inter 14px, alternating background `rgba(5,36,36,0.02)`

**Interactive elements:**
- Accordion open/close: GSAP `.to({ height: 'auto' })` via Flip or GSAP height tween
- Active accordion indicator: `#abff02` left border on trigger when open

---

### Section 10 — Reference: Configuration Variables (REFERENCE tier — Collapsed)

**Purpose:** Allow developers to customize before deploying. Searchable/scrollable table.

**Editorial framing:** Collapsed by default. Exposes the 15 most commonly needed variables. Note: `DEFAULT_TIMEZONE`, `AUTH_METHOD`, `READ_ONLY`, `MAIL_ENABLED` are surfaced inline as "quick config" callouts in Section 5 (Getting Started); the full table here is for deep customization.

**Structural layout:**
- Same accordion pattern as Section 9
- Trigger: "15 CONFIGURATION VARIABLES — VIEW FULL REFERENCE"
- Expanded: searchable filter input (client-side, no backend), table with Variable / Description columns
- Variable names: Geist Mono `#abff02`
- All variables prefixed `OUTLOOK_MCP_`

**Interactive elements:**
- Search input filters table rows by variable name or description substring (case-insensitive)
- Copy-to-clipboard icon on each variable name

---

### Section 11 — Getting Started CTA / Pre-Footer (SHOWCASE tier)

**Purpose:** The closing brand moment before footer. Equivalent to Terminal's "The yard of the future starts today." + frosted-glass CTA button.

**Editorial framing:** A decisive, confident statement. The tone is developer-confidence, not hype.

**Headline (display, weight 400):**
```
Your AI assistant just got a upgraded.
```

**CTA button:** Frosted-glass: `background: rgba(255,255,255,0.15); backdrop-filter: blur(27px); border-radius: 8px`
- Button text: `INSTALL NOW` — Geist Mono 11px, weight 600, letter-spacing 1.98px, white
- Desktop padding: `25px 50px`
- Mobile: full-width within content column (~335px)

**Secondary link:** `View on GitHub ↗` — Geist Mono 11px, `#abff02`, letter-spacing 1.98px

**Background:** Dark forest green `#052424` with dot-grid overlay (same as Brand Reveal section — completes the visual bookend). Notch-corner SVG masks at top edge.

**Structural layout:**
- `height: 100vh` with centered content column
- Heading: `clamp(1.8rem, 4.86vw, 70px)`, Inter weight 400, white
- CTA centered below heading with `margin-top: 40px`
- Secondary link below CTA with `margin-top: 20px`

---

### Section 12 — Footer

**Purpose:** Navigation, license, social, attribution.

**Editorial framing:** Adapted from Terminal's footer. No "Made by rejouice" — replaced with "MIT License" and GitHub link.

**Content:**
- Column 1 (left): `outlook-local-mcp` wordmark + subtitle "A Model Context Protocol server for Microsoft Calendar & Mail" + MIT badge
- Column 2: PRODUCT links — Features, Getting Started, Docs, GitHub Releases
- Column 3: DEVELOPER links — GitHub, Report an Issue, Changelog
- Column 4 / REACH US: "Built for Claude, works with any MCP client." + GitHub link button + small social icons (GitHub, Twitter/X)
- Bottom bar: "MIT License — outlook-local-mcp" + link to GitHub

**Structural layout:**
- Background: `#052424` (solid, no photograph, no gradient)
- Desktop: 4-column layout
- Mobile: 2-column grid (wordmark + links) + stacked REACH US row below
- Column heading typography: 11px, Inter, weight 600, letter-spacing 1.98px, white, all-caps

**Interactive elements:**
- Link hover: text color → `#abff02`, transition 0.2s, no underline

---

## 2b. Design System Translation

### Color Token Translation

Terminal's `#052424` (dark forest green) is retained as the brand anchor — it reads as "developer tool" more than "industrial software" when paired with terminal window visuals. The lime `#abff02` accent is kept exactly.

```css
/* src/index.css — @theme block */
@theme {
  /* Brand colors */
  --color-brand-dark:     #052424;   /* Primary text, nav pill, footer BG */
  --color-brand-lime:     #abff02;   /* ONLY accent — active states, CTAs, accents */
  --color-brand-black:    #000000;   /* Deep space backgrounds only */
  --color-brand-white:    #ffffff;   /* Text on dark, button fills */
  --color-brand-off-white: #f0f0f0;  /* Section backgrounds, card fills */

  /* Functional grays */
  --color-gray-900:       #0a1010;   /* Code block backgrounds */
  --color-gray-800:       #1a2a2a;   /* Secondary dark surfaces */
  --color-gray-600:       #454742;   /* Mid-gray text */
  --color-gray-400:       #7f7f7f;   /* Body text on white */
  --color-gray-200:       #c2c2c2;   /* Eyebrow labels, muted text */
  --color-gray-100:       #e8e8e8;   /* Divider lines */

  /* Accent variants for light-mode WCAG compliance */
  --color-lime-dark:      #5a8600;   /* Lime darkened for text on white (4.8:1 ratio vs white) */
  --color-lime-mid:       #7ab800;   /* Lime for borders/icons on white (3.5:1 — AA large only) */

  /* Typography */
  --font-sans:    'Inter', system-ui, -apple-system, sans-serif;
  --font-mono:    'Geist Mono', 'Fira Code', 'Cascadia Code', monospace;

  /* Type scale */
  --text-hero:              clamp(2.5rem, 5.09vw, 4.58rem);  /* 73.33px @ 1440px */
  --text-display:           clamp(2.4rem, 4.86vw, 4.375rem); /* 70px @ 1440px */
  --text-brand-reveal:      clamp(2.56rem, 7.64vw, 6.875rem); /* 110px @ 1440px */
  --text-section-heading:   clamp(1.5rem, 3.33vw, 2.5rem);
  --text-capability:        clamp(1.25rem, 2.13vw, 1.917rem); /* 30.67px @ 1440px */
  --text-body:              1rem;      /* 16px */
  --text-small:             0.875rem;  /* 14px */
  --text-label:             0.6875rem; /* 11px */
  --text-mono-label:        0.6875rem; /* 11px Geist Mono */

  /* Font weights */
  --font-weight-display:    400;  /* All large headings — confident restraint */
  --font-weight-body:       400;
  --font-weight-medium:     450;  /* Active nav items, capability text */
  --font-weight-semibold:   500;  /* Mobile drawer nav */
  --font-weight-bold:       600;  /* CTA labels only */

  /* Letter-spacing */
  --tracking-hero:          -0.02em;
  --tracking-label:         0.18em;   /* 1.98px at 11px */
  --tracking-nav:           0.03em;   /* 0.42px at 14px */
  --tracking-cta:           0.136em;  /* 1.5px at 11px */

  /* Border radius */
  --radius-pill:    8px;   /* Nav pill, buttons, cards */
  --radius-notch:   80px;  /* Notch-corner SVG mask radius */
  --radius-sm:      4px;
  --radius-full:    9999px;

  /* Spacing tokens (NOT --spacing — reserved by Tailwind 4) */
  --section-gap:          80px;    /* Between major sections */
  --section-gap-sm:       48px;
  --container-padding-x:  46.67px; /* Desktop: 5.128vw */
  --container-padding-mobile: 20px;
  --container-max-width:  1280px;
}
```

### Theme Plan

**Default theme: Dark (matches Terminal Industries primary aesthetic)**
The page opens in dark mode by default — the hero is dark, the nav pill is dark, the Brand Reveal section is dark. Light sections (Introduction, Capabilities list, Getting Started, Tech Stack) are white `#ffffff` islands within the dark journey.

**Light sections (white `#ffffff` background):**
- Section 2 — Introduction
- Left panel of Section 3 — Capabilities
- Section 5 — Getting Started
- Section 8 — Tech Stack
- Section 9, 10 — Reference accordions

**Dark sections (`#052424` background):**
- Section 0 — Nav pill (always)
- Section 1 — Hero (atmospheric dark)
- Section 4 — Brand Reveal
- Section 6 — Platform Support
- Section 11 — Pre-footer CTA
- Section 12 — Footer

**Off-white sections (`#f0f0f0` background):**
- Section 8 — Tech Stack metadata bar
- End-state of Brand Reveal (scroll-driven transition from `#052424` → `#f0f0f0`)

**Light mode accent handling (WCAG AA):**
- Lime `#abff02` on white background: contrast ratio 1.3:1 — FAILS AA. Never use lime text directly on white.
- Use `--color-lime-dark: #5a8600` for any lime-colored text on white backgrounds (contrast 4.8:1 — passes AA for normal text)
- Lime `#abff02` is safe on `#052424` (8.2:1) and on `#000000` (10.5:1)
- Code block accent (lime on `#0a1010`): contrast ~9:1 — passes AAA

---

## 2c. Motion Logic & Easing Variables

### Global Animation Variables

```css
:root {
  --ease-out:        cubic-bezier(0, 0, 0.58, 1);   /* Terminal's global easing */
  --ease-in-out:     cubic-bezier(0.42, 0, 0.58, 1);
  --ease-spring:     cubic-bezier(0.34, 1.56, 0.64, 1); /* Slight overshoot for pop-in */
  --duration-fast:   0.2s;
  --duration-base:   0.4s;
  --duration-slow:   0.6s;
  --duration-crawl:  1.0s;
}
```

### GSAP Easing Curves

```tsx
// Register once in src/main.tsx
import { gsap } from 'gsap'
import { ScrollTrigger } from 'gsap/ScrollTrigger'
import { useGSAP } from '@gsap/react'
gsap.registerPlugin(ScrollTrigger)

// Easing presets
const EASE_OUT     = 'cubic-bezier(0, 0, 0.58, 1)'   // entrance animations
const EASE_SCRUB   = 'none'                             // scroll-bound: no easing
const EASE_FADEOUT = 'power2.inOut'                    // crossfade transitions
```

### Section Entrance Animations (shared pattern)

All section headings share this pattern:
```tsx
gsap.from(headingRef.current, {
  opacity: 0,
  y: 30,
  duration: 0.6,
  ease: EASE_OUT,
  scrollTrigger: { trigger: headingRef.current, start: 'top 85%' }
})
// Eyebrow label staggered 0.15s after heading
gsap.from(eyebrowRef.current, {
  opacity: 0,
  y: 20,
  duration: 0.5,
  ease: EASE_OUT,
  delay: 0.15,
  scrollTrigger: { trigger: headingRef.current, start: 'top 85%' }
})
```

### Hero Scroll Journey

**Pattern:** Single GSAP timeline with `animation` prop on ScrollTrigger (NOT CSS transitions + React state — per CLAUDE.md):

```tsx
// Hero scroll journey — scroll-position-bound opacity
const HOLD = 0.3  // fraction of timeline = "hold" phase
const FADE = 0.2  // fraction for crossfade

const tl = gsap.timeline()
// Phase 1: background only → headline enters
tl.to(headlineRef.current, { opacity: 1, duration: FADE, ease: EASE_FADEOUT })
tl.to({}, { duration: HOLD })
// Phase 2: headline holds → stat badges stagger in
tl.to(badgeRefs.current, { opacity: 1, stagger: 0.1, duration: FADE })
tl.to({}, { duration: HOLD })
// Phase 3: install block enters
tl.to(installRef.current, { opacity: 1, y: 0, duration: FADE })
tl.to({}, { duration: HOLD * 2 })

ScrollTrigger.create({
  trigger: heroWrapperRef.current,
  start: 'top top',
  end: 'bottom top',
  scrub: 1.5,
  animation: tl,
})
// Progress dots / indicators: direct DOM manipulation in onUpdate (no React state re-renders)
```

### Scroll-Pinning Plan

**Section 3 — Core Capabilities (pinned, desktop only)**

```
Pin trigger:  height: 100vh; overflow: hidden (explicit — not minHeight)
Total scroll: ~1900px (5 clusters × ~380px each)
Scrub:        1.5
Breakpoint:   gsap.matchMedia() — pin only at (min-width: 1024px)
Refs:         Separate desktopPanelRefs / mobilePanelRefs arrays
```

```tsx
// src/components/Capabilities.tsx
const desktopPanelRefs = useRef<(HTMLDivElement | null)[]>([])
const mobilePanelRefs  = useRef<(HTMLDivElement | null)[]>([])
const wrapperRef       = useRef<HTMLDivElement>(null)

useGSAP(() => {
  const mm = gsap.matchMedia()
  mm.add('(min-width: 1024px)', () => {
    const tl = gsap.timeline()
    const n = desktopPanelRefs.current.length
    const HOLD = 1 / n
    const FADE = 0.08

    tl.to({}, { duration: HOLD }) // first panel hold
    for (let i = 1; i < n; i++) {
      const label = `fade-${i}`
      tl.addLabel(label)
      tl.to(desktopPanelRefs.current[i - 1], { opacity: 0, duration: FADE, ease: 'power2.inOut' }, label)
      tl.to(desktopPanelRefs.current[i],     { opacity: 1, duration: FADE, ease: 'power2.inOut' }, label)
      tl.to({}, { duration: HOLD })
    }

    ScrollTrigger.create({
      trigger: wrapperRef.current,
      pin: true,
      scrub: 1.5,
      start: 'top top',
      end: `+=${n * 380}`,
      animation: tl,
    })
  })
  mm.add('(max-width: 1023px)', () => { /* no pin — tab strip handles switching */ })
}, { scope: wrapperRef })
```

**Section 4 — Brand Reveal (pinned, desktop only)**

```
Pin trigger:  height: 100vh; overflow: hidden
Total scroll: ~1100px desktop
Scrub:        1.0
Animation:    Background color tween (#052424 → #f0f0f0) + text color tween (white → #030000) + text scale
```

```tsx
const tl = gsap.timeline()
tl.to(sectionRef.current, { backgroundColor: '#f0f0f0', duration: 0.5 })
tl.to(headingRef.current, { color: '#030000', duration: 0.5 }, '<')
tl.to(headingRef.current, { scale: 0.7, duration: 0.3 }, '<+=0.2')

// Single ScrollTrigger on parent timeline
tl.scrollTrigger = {  // attached via scrollTrigger prop on timeline constructor
  trigger: brandWrapperRef.current,
  pin: true,
  scrub: 1,
  start: 'top top',
  end: '+=1100',
}
```

### Stagger Timings

| Context | Stagger | Duration | Ease |
|---|---|---|---|
| Section heading entrance | — | 0.6s | ease-out |
| Eyebrow label (after heading) | +0.15s delay | 0.5s | ease-out |
| Stat badges row | 0.08s per badge | 0.4s | ease-out |
| Logo/platform grid fade-in | 0.1s per item | 0.4s | ease-out |
| Capability cluster tab items | 0.05s per item | 0.3s | ease-out |
| Code syntax highlight sweep | 0.02s per char | 0.3s | linear |

### Typed Animation (Introduction section)

```tsx
// Character-by-character type-in for the key differentiator word
// Using GSAP SplitText or manual character splitting
const chars = textRef.current.querySelectorAll('.char')
gsap.fromTo(chars,
  { opacity: 0.2, color: '#052424' },
  {
    opacity: 1,
    color: '#abff02',  // lime during typing
    stagger: 0.06,
    duration: 0.1,
    ease: 'none',
    scrollTrigger: { trigger: textRef.current, start: 'top 70%' },
    onComplete: () => {
      gsap.to(chars, { color: '#052424', duration: 0.4, ease: EASE_OUT })
    }
  }
)
```

### Nav Entrance (preloader exit equivalent)

```tsx
// Nav pill animates in from above after page load
gsap.from(navRef.current, {
  y: -20,
  opacity: 0,
  duration: 0.3,
  ease: EASE_OUT,
  delay: 0.1  // slight delay after DOMContentLoaded
})
```

### `prefers-reduced-motion` Guard

```tsx
// Wrap all GSAP init in matchMedia check
const mm = gsap.matchMedia()
mm.add('(prefers-reduced-motion: no-preference)', () => {
  // all animations here
})
mm.add('(prefers-reduced-motion: reduce)', () => {
  // show final states immediately — no transitions
  gsap.set([headingRef.current, badgeRefs.current], { opacity: 1, y: 0 })
})
```

---

## 2d. Visual Asset Inventory

Each asset below is a dedicated file for one asset agent. One agent = one file.

---

### Asset 1 — Hero Background

**Asset name:** `HeroBackground`
**Target file path:** `src/assets/HeroBackground.tsx`
**Visual tier:** Canvas 2D (or CSS gradient + WebGL light rays)
**What it depicts:** Atmospheric dark environment simulating a developer's desk at night — deep blacks (`#000000` → `#020d06`), warm amber light leak from the left edge (simulating a monitor or lamp). Subtle terminal-window glow suggestion. No literal objects — purely atmospheric light and dark. The "cinematic warmth" of Terminal's truck photography, adapted for a code context.
**Implementation notes:**
- Full-bleed canvas, `position: absolute; inset: 0; z-index: 0; pointer-events: none`
- Upper 55% of canvas: atmospheric dark with amber gradient from left edge (`#1a0800` → transparent)
- Lower 45%: pure black `#000000`
- Optional: slow-moving particle field (very subtle, white dots 0.5px at 3% opacity) suggesting a terminal output stream
- Fallback: CSS gradient `background: radial-gradient(ellipse 60% 40% at 15% 50%, #1a0800 0%, #000000 60%)`
**Parent component:** `src/sections/Hero.tsx`

---

### Asset 2 — Hero Install Command Block

**Asset name:** `InstallCommandBlock`
**Target file path:** `src/components/InstallCommandBlock.tsx`
**Visual tier:** SVG (interactive component, not a canvas asset)
**What it depicts:** A dark pill-shaped command block with syntax highlighting. Left accent: 2px `#abff02` vertical bar. Main content: `go install github.com/desek/...` in Geist Mono. Right side: copy icon (clipboard SVG). On copy: icon transitions to checkmark, border flashes `#abff02` for 0.5s.
**Implementation notes:**
- `background: #0a1010; border-radius: 8px; border-left: 2px solid #abff02`
- Copy button: `aria-label="Copy install command"`, 44×44px touch target
- Clipboard / checkmark icons: inline SVG, color `#abff02`
**Parent component:** `src/sections/Hero.tsx`

---

### Asset 3 — Capability Visual: Calendar Management

**Asset name:** `CapabilityCalendar`
**Target file path:** `src/assets/CapabilityCalendar.tsx`
**Visual tier:** Canvas 2D
**What it depicts:** A dark terminal window showing Claude's response to a calendar query. Terminal window chrome at top (three dots in gray, title bar `#1a2a2a`). Inside: JSON-formatted calendar event output with lime `#abff02` syntax highlighting on keys, white strings, muted gray comments. Subtle cursor blink. Atmosphere: a real tool response, not a mockup diagram.
**Implementation notes:**
- Canvas or `<pre>` block styled to look like a terminal; if Canvas — static render (no animation), if DOM — Geist Mono, 12px, line-height 1.6
- Window shadow: `box-shadow: 0 24px 80px rgba(0,0,0,0.6)`
- Terminal dots: `#ef5350` / `#ffb300` / `#66bb6a` at 12px diameter
- Notch-corner SVG mask applied on left edge (80px radius concave corners)
**Parent component:** `src/sections/Capabilities.tsx` (panel for cluster 01)

---

### Asset 4 — Capability Visual: Multi-Account Grid

**Asset name:** `CapabilityMultiAccount`
**Target file path:** `src/assets/CapabilityMultiAccount.tsx`
**Visual tier:** SVG
**What it depicts:** A schematic diagram showing three account "nodes" (circles labeled "Account 1", "Account 2", "Account 3") connected by lines to a central "MCP Server" node. Each account node has a small keychain icon inside. Aesthetic: blueprint/schematic, dark background `#0a1010`, white/gray line art, `#abff02` highlights on active connections. Clean geometric layout.
**Implementation notes:**
- Fully SVG, `viewBox="0 0 800 600"`, responsive via `width: 100%; height: auto`
- Node circles: `stroke: rgba(171,255,2,0.4); fill: #0f1f1f`
- Active node: `stroke: #abff02; fill: rgba(171,255,2,0.05)`
- Line connections: `stroke: rgba(171,255,2,0.3); stroke-width: 1.5px; stroke-dasharray: 4 4`
- `pointer-events: none`
**Parent component:** `src/sections/Capabilities.tsx` (panel for cluster 02)

---

### Asset 5 — Capability Visual: Mail Search Terminal

**Asset name:** `CapabilityMailSearch`
**Target file path:** `src/assets/CapabilityMailSearch.tsx`
**Visual tier:** Canvas 2D (or styled DOM)
**What it depicts:** Terminal window showing a KQL mail search query and result output. Input line: `> mail_search_messages({ query: "project proposal" })`. Output: two message summaries with subject lines in white, sender in gray, timestamp in lime. Conveys: powerful search, readable output, developer-friendly interface.
**Implementation notes:**
- Same terminal window chrome as Asset 3 (consistent aesthetic across capability panels)
- KQL query keyword highlighted in `#abff02`
- Notch-corner SVG mask on left edge
**Parent component:** `src/sections/Capabilities.tsx` (panel for cluster 03)

---

### Asset 6 — Capability Visual: Privacy Architecture Diagram

**Asset name:** `CapabilityPrivacyDiagram`
**Target file path:** `src/assets/CapabilityPrivacyDiagram.tsx`
**Visual tier:** SVG
**What it depicts:** A layered containment diagram. Outer box: "Your Machine" (dashed border, `rgba(171,255,2,0.3)`). Inside: three inner boxes — "MCP Client (Claude)", "MCP Server (outlook-local-mcp)", "OS Keychain". Arrows: MCP Client → MCP Server (bidirectional), MCP Server → OS Keychain (read/write), MCP Server → "Microsoft Graph API" (outbound only, exits the outer box). No inbound arrows cross the outer boundary. Clear visual proof that data stays local.
**Implementation notes:**
- SVG, `viewBox="0 0 800 500"`, fully responsive
- Outer dashed boundary: `stroke: rgba(171,255,2,0.4); stroke-dasharray: 8 4; fill: none`
- Internal boxes: `fill: #0f1f1f; stroke: rgba(255,255,255,0.15)`
- Arrows: `stroke: rgba(171,255,2,0.6)` for internal; `stroke: rgba(255,255,255,0.3)` for outbound
- Labels: Geist Mono 11px, white
- "Microsoft Graph API" label outside boundary: `fill: rgba(255,255,255,0.5)`
- `pointer-events: none`
**Parent component:** `src/sections/Capabilities.tsx` (panel for cluster 04) and `src/sections/TrustSection.tsx`

---

### Asset 7 — Capability Visual: Auth Flow Diagram

**Asset name:** `CapabilityAuthFlow`
**Target file path:** `src/assets/CapabilityAuthFlow.tsx`
**Visual tier:** SVG
**What it depicts:** A three-step auth flow diagram. Left: "Device Code" box with a URL display mock (`aka.ms/devicelogin → XXXXX`). Center: browser icon with checkmark. Right: keychain icon with "~90 days" label. Connected by right-facing arrows. Sub-label below: "No Entra ID app registration required." The visual directly proves the zero-config claim.
**Implementation notes:**
- SVG, responsive
- Step boxes: same dark card style as Asset 4
- Device code URL text: Geist Mono 10px, `rgba(171,255,2,0.8)`
- "No Entra ID" label: Geist Mono 11px, `#abff02`, centered below diagram
- `pointer-events: none`
**Parent component:** `src/sections/Capabilities.tsx` (panel for cluster 05)

---

### Asset 8 — Dot-Grid Background Overlay

**Asset name:** `DotGridOverlay`
**Target file path:** `src/assets/DotGridOverlay.tsx`
**Visual tier:** SVG (tiled pattern)
**What it depicts:** The recurring dot-grid motif used in the Brand Reveal section, Platform Support section, and Pre-footer CTA section. Square grid of ~80px cells with tiny lime-green (`#abff02`) 3×3px square dots at each intersection. Grid lines are extremely faint `rgba(171,255,2,0.08)`.
**Implementation notes:**
- SVG `<pattern>` element with `patternUnits="userSpaceOnUse"` at 80px repeat
- Grid lines: `stroke: rgba(171,255,2,0.08); stroke-width: 1`
- Dots: `<rect x="-1.5" y="-1.5" width="3" height="3" fill="#abff02" opacity="0.6" />`
- Applied via `position: absolute; inset: 0; z-index: 0; pointer-events: none` as a background layer
- Full-bleed: `width: 100%; height: 100%`
**Parent component:** Used in `BrandReveal.tsx`, `PlatformSupport.tsx`, `PreFooterCTA.tsx`

---

### Asset 9 — Notch-Corner SVG Mask

**Asset name:** `NotchCornerMask`
**Target file path:** `src/components/NotchCornerMask.tsx`
**Visual tier:** SVG
**What it depicts:** The section transition ornament. Two concave corner cutouts at the upper-left and upper-right corners of a section (or lower corners), creating the illusion that the section above has convex protrusions extending into this one. This is the single most distinctive design language element from Terminal Industries.
**Implementation notes:**
- Reusable component accepting props: `position: 'top' | 'bottom'`, `color: string` (background color of the section below/above), `radius: number` (default 80)
- Implementation: `position: absolute; top: 0; left: 0; right: 0; pointer-events: none; z-index: 1`
- SVG `<path>` using cubic bezier curves to cut concave arcs from corners
- The `color` prop fills the path with the adjacent section's background color to achieve the cut-out effect
- Used on: bottom of Hero, top/bottom of Introduction, sides of Capabilities media panel, top of Brand Reveal, top of Pre-footer CTA
**Parent component:** Multiple sections

---

### Asset 10 — Brand Reveal Background (Three.js Particle Field)

**Asset name:** `BrandRevealParticles`
**Target file path:** `src/assets/BrandRevealParticles.tsx`
**Visual tier:** Three.js
**What it depicts:** Extremely subtle star/particle field visible in the background of the Brand Reveal section's dark-green phase. White dots at <1% opacity, very slow drift. Aesthetic equivalent to Terminal's hero video state 2 (wireframe truck with particle stars). Conveys: "code runs everywhere, invisibly, like stars." Disappears as background transitions to `#f0f0f0`.
**Implementation notes:**
- Three.js `<Canvas>` from `@react-three/fiber`
- `position: absolute; inset: 0; z-index: 0; pointer-events: none` (CRITICAL — must not block interaction)
- ~300 particle points, random positions in a 20×20×5 unit space
- Camera: orthographic, looking down Z axis
- Particle material: `PointsMaterial`, color white, size 0.02, opacity 0.15, transparent
- Slow drift: `useFrame` animates position by `+0.0001` per tick on Z axis, reset on loop
- Opacity tied to GSAP scroll progress — fade out as background lightens
- Fallback: if Three.js fails to initialize, show nothing (empty canvas)
**Parent component:** `src/sections/BrandReveal.tsx`

---

### Asset Summary Table

| # | Asset Name | File | Tier | Parent Section |
|---|---|---|---|---|
| 1 | HeroBackground | `src/assets/HeroBackground.tsx` | Canvas 2D | Hero |
| 2 | InstallCommandBlock | `src/components/InstallCommandBlock.tsx` | SVG component | Hero |
| 3 | CapabilityCalendar | `src/assets/CapabilityCalendar.tsx` | Canvas 2D | Capabilities (01) |
| 4 | CapabilityMultiAccount | `src/assets/CapabilityMultiAccount.tsx` | SVG | Capabilities (02) |
| 5 | CapabilityMailSearch | `src/assets/CapabilityMailSearch.tsx` | Canvas 2D | Capabilities (03) |
| 6 | CapabilityPrivacyDiagram | `src/assets/CapabilityPrivacyDiagram.tsx` | SVG | Capabilities (04) + Trust |
| 7 | CapabilityAuthFlow | `src/assets/CapabilityAuthFlow.tsx` | SVG | Capabilities (05) |
| 8 | DotGridOverlay | `src/assets/DotGridOverlay.tsx` | SVG | BrandReveal, PlatformSupport, PreFooterCTA |
| 9 | NotchCornerMask | `src/components/NotchCornerMask.tsx` | SVG | Multiple sections |
| 10 | BrandRevealParticles | `src/assets/BrandRevealParticles.tsx` | Three.js | BrandReveal |

---

## Non-Negotiable Design Rules (Adapted for Developer Tool Context)

Carried directly from Terminal Industries' Feature 16:

1. **Lime `#abff02` is the ONLY accent color.** No blue, no orange, no red. Reserved for: active nav items, lime dot indicator, capability step numbers, code syntax highlights, CTA link text, diagram line accents, typed animation, copy button flash. Used at most 4 times per viewport.

2. **The frosted-glass nav pill must not change shape or disappear.** At all scroll positions, all section backgrounds: `background: rgba(0,0,0,0.3); backdrop-filter: blur(30px); border-radius: 8px`. Desktop: ~960px wide × 78px. Mobile: calc(100% - 38px) × 64px. Never full-width, never opaque, never transparent.

3. **Notch-corner SVG masks define section transitions.** Flat rectangular edges between contrasting background sections are NEVER acceptable. Every dark-to-white or white-to-dark transition uses the concave/convex notch-corner pattern.

4. **Typography weight 400 for all display text ≥30px.** Headings are Inter weight 400. The deliberate lightness at large size is the tone — confident restraint, not aggression. Only CTAs (weight 600) and nav column headers (weight 600) use bold.

5. **Canvas / Three.js backgrounds must be `pointer-events: none`.** `HeroBackground.tsx`, `BrandRevealParticles.tsx`, and `DotGridOverlay.tsx` all require this — they must not block nav buttons, CTA clicks, or any interactive element above them in the stacking order.

6. **Dark green `#052424` is the brand anchor.** All body text, form text, dark backgrounds, footer. Never use `#000000` as a brand color — pure black is reserved for deep space/atmospheric hero backgrounds only.

7. **Scroll-driven animations cannot use CSS `transition` + React state** for scrub-bound sequences. All pinned-section crossfades must use GSAP timeline with `animation` prop on ScrollTrigger. Progress indicators in `onUpdate` must use direct DOM manipulation, not `setState`.

8. **Desktop pinned sections require `height: 100vh; overflow: hidden`** on the trigger element — never `minHeight`. Separate `desktopPanelRefs` and `mobilePanelRefs` arrays required when both DOM branches render simultaneously.

9. **`gsap.matchMedia()` for all breakpoint-conditional animations.** Never `window.innerWidth` checks inside `useGSAP`. Align breakpoint with Tailwind's `lg:` at 1024px.

10. **All GSAP animations must use `useGSAP` hook** — never raw `useEffect`. GSAP plugins registered in `src/main.tsx`.
