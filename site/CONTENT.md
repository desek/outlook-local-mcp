# Content Manifest — Outlook Local MCP Product Page

---

## Intent Brief

**Product:** Outlook Local MCP
**Target audience:** Developers evaluating calendar and mail management tools for AI assistants (Claude Desktop, Claude Code, or any MCP-compatible client).
**Page type:** Marketing product page — not documentation. The goal is to persuade a developer to install and adopt the tool, not to be a reference manual.
**Design language:** Terminal Industries (applied by downstream agents).
**Tone:** Technically credible, direct, minimal hype. Developers trust precision over adjectives — specificity is the persuasion mechanism. Confidence comes from exact numbers, exact algorithm names, exact API versions.
**Core conversion action:** Copy an install command or download a release. Everything on the page should reduce friction to that moment.
**Narrative arc:**
1. Hook — what this is and why it's different from alternatives (local, zero-config)
2. Capability proof — show the breadth of 23 tools without overwhelming
3. Trust signals — privacy, security, OS-native patterns, enterprise-grade internals
4. Getting started — minimal friction path to first run
5. Reference — full tool and config list for developers who want to go deep

---

## Section 1 — Hero

**Tier: HERO**

**Headline (primary):** Outlook Local MCP
**Subheadline (working copy):** Your Microsoft calendar, inside your AI — no cloud middleman, no app registration.
**Body (working copy):** A Model Context Protocol server that connects Claude (or any MCP client) directly to Microsoft Calendar and Mail via the Graph API. All data stays on your machine. OAuth tokens live in your OS keychain. The server process never leaves localhost.

**Key differentiators (3 punchy statements — use as visual callouts or stat bars):**
- 100% local — no intermediate servers, no data exfiltration surface
- ZERO ENTRA ID setup — uses Microsoft's own first-party client ID, pre-authorized for Calendar scopes
- 23 MCP tools — calendar, mail, multi-account, diagnostics

**Primary CTA:** Install with Go (copy-able command)
```
go install github.com/desek/outlook-local-mcp/cmd/outlook-local-mcp@latest
```

**Secondary CTA:** Download Claude Desktop extension (.mcpb release)

**Editorial note:** The "no Entra ID app registration" point is genuinely rare and removes a common multi-hour blocker for developers. It should be surfaced as a headline differentiator, not buried in a feature list. The "100% local" angle is also the primary trust signal — lead with it before anything else.

---

## Section 2 — Showcase: Core Capabilities

**Tier: SHOWCASE**

Present as a scannable feature grid or tab group. Each capability cluster is a distinct selling point.

### 2a — Calendar Management (14 tools)
**Label:** Calendar
**Summary:** Read, search, create, update, and delete calendar events and meetings. Check free/busy availability across accounts.
**Tool list (for tooltip or expanded view):** calendar_list, calendar_list_events, calendar_get_event, calendar_search_events, calendar_get_free_busy, calendar_create_event, calendar_create_meeting, calendar_update_event, calendar_update_meeting, calendar_delete_event, calendar_cancel_meeting, calendar_respond_event, calendar_reschedule_event, calendar_reschedule_meeting
**Key detail:** Meeting tools include attendee confirmation guidance and extra warnings for external attendees — the LLM won't accidentally spam a meeting invite without a check.

### 2b — Multi-Account Support (3 tools)
**Label:** Multi-Account
**Summary:** Add, list, and remove Microsoft accounts at runtime. Each account gets isolated token storage. Accounts persist across restarts.
**Tool list:** account_add, account_list, account_remove
**Key detail:** Lazy auth — no credentials required at startup. Authentication triggers on first tool call per account.

### 2c — Mail Access (4 tools, opt-in)
**Label:** Mail (opt-in)
**Summary:** Read-only access to mailbox folders, messages, and full-text search via KQL. Disabled by default; enabled with one env var.
**Tool list:** mail_list_folders, mail_list_messages, mail_search_messages, mail_get_message
**Key detail:** Opt-in only. Set `OUTLOOK_MCP_MAIL_ENABLED=true`. Never writes to mail.

### 2d — Local Privacy & Security
**Label:** Privacy
**Summary line:** No data routing through third parties. Every credential and token stays on your machine.
**Bullet points:**
- OAuth tokens stored in macOS Keychain / Linux libsecret / Windows DPAPI
- AES-256-GCM encrypted file fallback when OS keychain is unavailable
- Only outbound connections: Microsoft Graph API + Microsoft Identity Platform
- PII sanitization built into structured logging
- OData injection protection on all inputs
- Read-only mode: single env var disables all write operations

**Editorial note:** This cluster is not just a security section — it's a trust section. Developers who are privacy-conscious will evaluate this tool partly on whether they can explain it to their security team. Frame the storage and network behavior as verifiable, auditable facts.

### 2e — Zero-Config Auth
**Label:** Authentication
**Summary:** Three auth methods, all requiring ZERO ENTRA ID app registration.
**Auth methods:**
- **Device code (default):** Displays URL + code. Works in headless and remote environments. Pre-authorized Microsoft first-party client ID means no admin consent.
- **Interactive browser:** Opens system browser, listens on localhost.
- **Authorization code (PKCE):** For fully headless/remote setups via the complete_auth tool.
**Key detail:** Token expiry is ~90 days. Silent refresh is automatic. First-time auth is a one-time browser action.

---

## Section 3 — Showcase: Getting Started

**Tier: SHOWCASE**

This section must be actionable — a developer should be able to go from this page to running the tool in under 5 minutes. Present as a minimal steps list, with copy-able code blocks.

### Install

**Option A — Go install (recommended for Go developers):**
```bash
go install github.com/desek/outlook-local-mcp/cmd/outlook-local-mcp@latest
```

**Option B — Docker (no Go required, < 20 MB image):**
```bash
docker pull ghcr.io/desek/outlook-local-mcp:latest
```

**Option C — Claude Desktop extension:**
Download `.mcpb` from GitHub Releases. No terminal required.

**Option D — Build from source:**
```bash
git clone https://github.com/desek/outlook-local-mcp
cd outlook-local-mcp
go build ./cmd/outlook-local-mcp
```

### Configure Claude Desktop

```json
{
  "mcpServers": {
    "outlook-local": {
      "command": "outlook-local-mcp",
      "env": {
        "OUTLOOK_MCP_DEFAULT_TIMEZONE": "America/New_York"
      }
    }
  }
}
```

### Configure Claude Code

```json
{
  "mcpServers": {
    "outlook-local": {
      "command": "/path/to/outlook-local-mcp",
      "env": {
        "OUTLOOK_MCP_DEFAULT_TIMEZONE": "America/New_York"
      }
    }
  }
}
```

### First run
The server authenticates lazily — no credential setup before first use. On first tool call, it displays a device code URL. Complete auth once in a browser. Tokens are cached in your OS keychain for ~90 days.

**Editorial note:** Keep this section minimal. The target developer has seen a hundred README quickstarts. Show the minimum viable path — install, paste config, done. The config snippets are the highest-value content here because they can be copied verbatim.

---

## Section 4 — Showcase: Platform Support

**Tier: SHOWCASE (compact)**

Present as a platform badge row or small table.

| Platform | Binary | Docker |
|---|---|---|
| macOS (Apple Silicon) | arm64 | linux/arm64 |
| macOS (Intel) | — | — |
| Linux (x86_64) | amd64 | linux/amd64 |
| Linux (ARM) | arm64 | linux/arm64 |
| Windows | amd64 | — |

**Docker image:** `ghcr.io/desek/outlook-local-mcp:latest` — scratch-based, < 20 MB
**Claude Desktop:** Native extension via `.mcpb` format (GitHub Releases)

---

## Section 5 — Reference: Full Tool List

**Tier: REFERENCE**

Present in a collapsed/expandable table or accordion. This content is for developers who want to audit the full surface area before adopting. It should not be prominent on first load.

### Account Management
| Tool | Description |
|---|---|
| account_add | Add a Microsoft account; triggers lazy auth on first use |
| account_list | List all configured accounts and their auth status |
| account_remove | Remove an account and delete its cached token |

### Diagnostics
| Tool | Description |
|---|---|
| status | Server health, configured accounts, enabled features |
| complete_auth | Complete authorization code flow for headless/remote setups |

### Calendar — Read
| Tool | Description |
|---|---|
| calendar_list | List available calendars |
| calendar_list_events | List events in a date range |
| calendar_get_event | Get a single event by ID |

### Calendar — Search
| Tool | Description |
|---|---|
| calendar_search_events | Full-text and OData-filtered event search |
| calendar_get_free_busy | Check free/busy slots for one or more users |

### Calendar — Write
| Tool | Description |
|---|---|
| calendar_create_event | Create a calendar event |
| calendar_create_meeting | Create a meeting with attendees (includes confirmation guidance) |
| calendar_update_event | Update an existing event |
| calendar_update_meeting | Update an existing meeting (includes confirmation guidance) |
| calendar_delete_event | Delete an event |
| calendar_cancel_meeting | Cancel a meeting and notify attendees |
| calendar_respond_event | Accept, decline, or tentatively accept a meeting |
| calendar_reschedule_event | Reschedule an event to a new time |
| calendar_reschedule_meeting | Reschedule a meeting and notify attendees |

### Mail — Read (opt-in)
| Tool | Description |
|---|---|
| mail_list_folders | List mailbox folders |
| mail_list_messages | List messages in a folder with OData filtering |
| mail_search_messages | Full-text KQL search across mailbox |
| mail_get_message | Get a single message by ID |

---

## Section 6 — Reference: Configuration Variables

**Tier: REFERENCE**

Collapsed by default. Present as a searchable or scrollable table. Target audience: developers who want to customize behavior before deploying.

**All variables prefixed:** `OUTLOOK_MCP_`

| Variable | Description |
|---|---|
| CLIENT_ID | Override the default Microsoft first-party client ID |
| TENANT_ID | Restrict auth to a specific Entra ID tenant |
| AUTH_METHOD | `device` (default), `browser`, `authcode` |
| DEFAULT_TIMEZONE | IANA timezone string, e.g. `America/New_York` |
| LOG_LEVEL | `debug`, `info`, `warn`, `error` |
| LOG_FORMAT | `json` (default) or `text` |
| LOG_SANITIZE | `true` (default) — strips PII from logs |
| LOG_FILE | Path to write log output |
| MAX_RETRIES | Retry count for transient Graph API errors |
| REQUEST_TIMEOUT_SECONDS | HTTP timeout per Graph API request |
| READ_ONLY | `true` — disables all write operations |
| MAIL_ENABLED | `true` — enables 4 opt-in mail tools |
| OTEL_ENABLED | `true` — enables OpenTelemetry metrics/tracing |
| TOKEN_STORAGE | `keychain` (default) or `file` |
| PROVENANCE_TAG | Custom tag written to MCP-created events |

**Editorial note:** Do not show all 22 variables on first load. Surface the 5–6 most commonly needed ones (DEFAULT_TIMEZONE, AUTH_METHOD, READ_ONLY, MAIL_ENABLED) inline in the getting-started section. The full table belongs in a collapsed reference panel.

---

## Section 7 — Reference: Tech Stack & Internals

**Tier: REFERENCE (brief inline callout, not a full section)**

For developers who evaluate tools by inspecting internals. Present as a compact metadata block, not prose.

- **Language:** Go 1.24+
- **API:** Microsoft Graph API v1.0
- **Protocol:** MCP (JSON-RPC over stdio)
- **Auth:** OAuth 2.0 — device code, browser, PKCE auth code
- **Token storage:** OS-native keychain + AES-256-GCM file fallback
- **Observability:** Structured JSON logging, audit log per tool invocation, optional OpenTelemetry (OTLP gRPC)
- **Resilience:** Exponential backoff retry, graceful SIGINT/SIGTERM shutdown
- **License:** MIT
- **Distribution:** GitHub Releases (cross-compiled binaries + SBOMs), Docker (ghcr.io), Claude Desktop extension

---

## Omitted Content

**Tier: OMIT**

The following source content is omitted from the product page:

- **Full 22-variable config table as primary content** — Too dense for a product page. Condensed to the 15 most relevant variables in a collapsed reference section.
- **Build-from-source instructions as a primary install path** — Relegated to install option D; not featured prominently.
- **Raw MCP protocol details (JSON-RPC over stdio)** — Implementation detail, not a selling point for the target audience.
- **Specific OTEL/OTLP gRPC configuration** — Too niche. Mentioned as a capability; not elaborated.
- **Well-known client ID friendly names list** — Internal configuration detail. Omit from the product page entirely.
- **Response filtering modes (text/summary/raw)** — Useful for power users but adds noise to the hero/showcase sections. Could appear in a deep FAQ or reference section if space allows.
- **SBOM distribution mention** — Relevant to enterprise procurement, not typical developer evaluation.
- **Author byline ("desek")** — Not needed on a product page; open source + MIT license is sufficient social proof signal.
- **Event provenance tagging internal detail** — Hidden property implementation detail. Worth one sentence in a security/trust section at most; not a headline feature.
- **MCP tool annotations / Anthropic Software Directory compliance** — Platform compliance detail. Omit from page copy; it is implied by the tool working with Claude.
