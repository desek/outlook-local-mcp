---
date: 2026-09-01
branch: docs/cr-implementation-set-0079-0083
commit: 78a3bb3
---

## Question

Research the possibilities to provide passthrough/external authentication and support multi-tenant serving for the outlook-local-mcp server: MCP client sends a JWT to the MCP server for an OBO flow or token passthrough to the Graph API, and multi-tenant serving.

## Response

# Passthrough Authentication and Multi-Tenant Serving for outlook-local-mcp

*Decision brief for the maintainer. Every claim is grounded in the supplied architecture survey, research findings, and critic notes; file:line and spec citations are carried through. Tier-1 (installed-source) verification items the critic flagged are called out in §10.*

## 1. Question and short answer

**Q1 — OBO vs token passthrough:** If an MCP client sends a JWT to this server, should the server exchange it (On-Behalf-Of) or forward it unchanged to Microsoft Graph?

**Q2 — Multi-tenant:** What would it take to serve users from many Entra tenants?

**Verdict.** Both questions are premised on a transport this server does not have: it is stdio-only, and the MCP authorization spec explicitly places stdio *out of scope*, directing such servers to read credentials from the environment — which is exactly what today's device-code/keychain design does. Pure **token passthrough to Graph is forbidden** by the spec's unconditional MUST-NOT and is a confused-deputy vector; **OBO is the correct shape** *if* a token ever crosses a client-server boundary, but it forces a public-to-confidential-client conversion and a per-principal cache lifecycle the codebase has no analog for. **Multi-tenant OBO serving is a different product**, not an extension of this one — it collides head-on with the load-bearing `local_first` and single-purpose governance. The strongest recommendation the evidence actually supports is **stay stdio local-first**; if multi-user serving is genuinely required, prefer **per-user instances** or a **public-client remote resource server**, and gate any confidential-client OBO work behind a CR/ADR (§10).

## 2. Current state

Identity is entirely process-local; nothing carries a per-request caller token.

| Fact | Evidence |
|---|---|
| stdio is the sole transport | `cmd/outlook-local-mcp/main.go:212` (`server.ServeStdio(s)`); no `ServeHTTP`/`ListenAndServe`/SSE wiring in cmd or internal; no port/http/listen/addr config var |
| Banner + build hardcode `transport=stdio` | `main.go:74`, `main.go:176-182` (`NewMCPServer` + `WithElicitation`) |
| Graph client built from an `azcore.TokenCredential` | `main.go:106-109` (`NewGraphServiceClientWithCredentials(cred, scopes)`) |
| All three credentials are long-lived, cache-backed, interactive **public** clients; none accept an external token | `auth.go:195-209` (`SetupCredential` switch); `auth.go:256-302`; `authcode.go:101-172` |
| Scopes are static, from feature flags; `Mail.Send` never requested | `auth.go:22-65` |
| Accounts keyed by human label in an in-memory `sync.RWMutex` map; no request principal | `registry.go:22`, `:27-80`, `:84-96` |
| Per-call resolution picks a pre-authenticated client and injects it via `WithGraphClient` | `account_resolver.go:94-114`, `:230-270`; `context.go:33,44` |
| Tokens live in per-account OS-keychain / AES-256-GCM file caches under `~/.outlook-local-mcp/` | `active_backend.go:19-32`; `accounts.go:1-49` |
| `tenant_id` is per-account; process default `common` | `accounts.go:27-29`; `registry.go:38-41`; `config.go:223` |

**Why it matters.** Accepting an inbound token per request would require, all net-new: an HTTP transport, a request-scoped credential wrapping the bearer token (or an OBO exchange), and per-request Graph-client construction. The `azcore.TokenCredential` seam is clean, but everything upstream of it assumes server-side selected identity.

## 3. Precondition: transport

Passthrough or OBO both need a remote HTTP surface. mcp-go **v0.57.0** (installed) supplies the transport and header plumbing but deliberately none of the auth enforcement.

**What it gives:**
- StreamableHTTP + SSE servers: `NewStreamableHTTPServer`, `NewSSEServer` (`server/streamable_http.go:335`, `server/sse.go:407`).
- Per-request inbound headers on every request struct — `req.Header.Get("Authorization")` works directly (`server/mcp/tools.go:58`); also `WithHTTPContextFunc` (`server/http_transport_options.go:11`).
- Host-supplied auth via `WithToolHandlerMiddleware` / `ToolHandlerMiddleware` (`server/server.go:70,309`).
- RFC 9728 Protected Resource Metadata at `/.well-known/oauth-protected-resource` (`server/protected_resource.go:13,141`).
- Configurable sessions: stateful `Mcp-Session-Id`, `WithStateLess(true)`, or per-request ephemeral; `ClientSessionFromContext` (`server/streamable_http.go:47,85`; `server/session.go:141`).

**What it does NOT give (the host must build all of it):**
- No token validation, no `401`, no `WWW-Authenticate` — zero matches in non-test server source. *Note: this is sourced from a grep-absence + DeepWiki (tier 2); the critic flags it for tier-1 confirmation (§10).*
- OAuth 2.1 *client* flows (`OAuthHandler`, PKCE, `TokenStore`) live on the client side (`client/transport/oauth.go`) — relevant only if this repo acts as an MCP client.

The transport swap itself is small: replace `ServeStdio` with a StreamableHTTP server; `NewMCPServer`, `RegisterTools`, and shutdown are reused; the verb registry is transport-agnostic (operates on `mcp.CallToolRequest` + context). A listen/transport flag must be added to `internal/config`.

## 4. Option A — On-Behalf-Of (OBO)

**Mechanism.** Client sends a token whose `aud` is the **MCP server's own app**; the server (a confidential client) exchanges it via `grant_type=jwt-bearer, requested_token_use=on_behalf_of` for a separate Graph token, then calls Graph. This is the spec-sanctioned "separate upstream token" model.

**azidentity fit (tier-1, v1.14.0 pinned).**
- `OnBehalfOfCredential` with `NewOnBehalfOfCredentialWithSecret` / `WithCertificate` / `WithClientAssertions` (`on_behalf_of_credential.go:53,64,75`).
- Implements `azcore.TokenCredential.GetToken` (`:101-106`) → drops into `NewGraphServiceClientWithCredentials` **unchanged**, exactly where `SetupCredential` returns a `TokenCredential` today (`auth.go:195`).
- `userAssertion` is **fixed at construction**, routed through MSAL `AcquireTokenOnBehalfOf`, cache checked first (`confidential_client.go:78-99`).

**App-registration prerequisites (Microsoft Learn, v2-oauth2-on-behalf-of-flow):**
1. Confidential-client credential (secret / cert / federated).
2. An **exposed API scope** so inbound tokens carry `aud = server app ID URI` — today's direct-to-Graph tokens are **not** OBO-exchangeable; a Graph-audience token *must be rejected*.
3. Delegated Graph permissions on the middle-tier app.
4. Consent pre-arranged (admin consent / `knownClientApplications` + `.default` combined consent / `preAuthorizedApplications`) — the middle tier cannot prompt mid-flow.

| Pros | Cons |
|---|---|
| Spec-compliant; correct trust boundary | Public → **confidential client**: secret/cert lifecycle & rotation, new operational burden |
| Zero change to `GraphServiceClient` wiring | Must expose own API scope; re-point clients to request it, not Graph |
| Enforceable audience validation & least-privilege | Per-user credential/cache lifecycle: one credential per assertion, evict on assertion expiry (~1h, *policy-dependent, approximate* — CAE can extend/revoke) |
| Sanctioned upgrade path | Custom-signing-key middle tiers unsupported; downstream CA surfaces as `interaction_required` the server must relay |

*Federated credential / workload identity federation is the natural answer to the secret-lifecycle burden (a federated assertion instead of a managed secret) — referenced only glancingly in the source material and worth developing if OBO is pursued.*

## 5. Option B — Token passthrough to Graph

**Mechanism.** Forward the client's Graph-audience bearer token unchanged. It *works mechanically*: Graph validates only `aud == https://graph.microsoft.com` (+ slash/sovereign variants) and signature/lifetime, never the forwarder's network origin.

**Why the spec forbids it — normative and unconditional:**
> "MCP servers MUST NOT accept any tokens that were not explicitly issued for the MCP server." … "The MCP server MUST NOT pass through the token it received from the MCP client." *(security_best_practices §Token Passthrough; authorization §Access Token Privilege Restriction)*

Two *separate* MUSTs are in play (the critic notes the risk lists conflate them): (a) **audience validation** — the server is not the audience, so it *cannot* validate, already a violation before any forwarding; (b) **passthrough** — forwarding the unvalidated token makes the downstream a confused deputy. The four named risk classes: security-control circumvention, accountability/audit-trail breakage, trust-boundary collapse, future-compatibility. Passthrough is also an **architectural dead-end**: a Graph-audience token can never be OBO-exchanged, foreclosing the sanctioned upgrade.

**The narrow acceptable case — and its precise framing.** Passthrough is defensible *only outside the MCP HTTP trust model entirely*: a single-tenant, single-owner, local personal tool where client, server, and app registration are the same principal on one trust boundary — there is no second party to confuse. **Correction (critic):** do not frame this as an in-model exception. The spec's MUST-NOT is unconditional; the exemption is that **stdio is out of scope**, not that trust-domain collapse earns a carve-out. This is essentially the status quo of this project, which is neither a passthrough relay nor an OBO middle tier — it authenticates directly with its own cache.

## 6. Option C and others (surfaced by the critic)

The OBO-vs-passthrough framing omits viable alternatives:

- **C1 — Keep stdio, let the CLIENT do auth (do-nothing to the trust model).** The MCP host authenticates the user; the stdio server keeps reading credentials from the environment/keychain. This is *exactly* what the spec sanctions for stdio. Status-quo-preserving; no confidential registration; consent stays per-user delegated.
- **C2 — Per-user / per-tenant server instances.** One stdio (or isolated HTTP) process per user. Sidesteps *all* of multi-tenant state isolation, JWKS, and OBO caching, and **preserves local-first and single-purpose**. The critic calls this the reconciling answer the decision otherwise skips — the option most consistent with project governance.
- **C3 — Public-client remote resource server (PKCE).** A remote HTTP MCP server can be an OAuth 2.1 resource server delegating to Entra as a **public client with PKCE**, issuing its own audience-bound tokens **without a secret/cert**. "Remote" does *not* imply confidential-client OBO — transport and confidential-vs-public are independent axes. Shifts consent to per-user delegated consent, no admin/preauth machinery.
- **C4 — API keys / static credentials.** Named only to dismiss: no per-user identity, no Entra integration, incompatible with delegated Graph access.
- **C5 — Managed identity / workload identity federation.** Federated credential instead of a managed secret; the answer to the repeatedly-raised "secret lifecycle" risk under any confidential-client path.

## 7. Multi-tenant serving

Turns three single-tenant assumptions into per-request variables.

| Dimension | Requirement | Evidence / risk |
|---|---|---|
| **App registration** | `signInAudience=AzureADMultipleOrgs`; authority `organizations` (work/school) or `common` (also personal). SP provisioned per customer tenant only after that tenant's consent | Learn access-tokens "Multitenant applications"; per-tenant admin consent |
| **Issuer validation** | Read tenant-independent OIDC metadata → templated `https://login.microsoftonline.com/{tid}/v2.0`; substitute the token's `tid`; require **exact** `iss` match; confirm `tid` is a GUID; scope signing key to that issuer; check `aud == your AppId URI` | Trusting literal `common`/`organizations` admits **every** tenant incl. personal accounts — spoofing / confused-deputy hole |
| **OBO authority** | Exchange against the **user's own** `{tid}` authority, never `common`. In azidentity: per-tenant `OnBehalfOfCredential(tenantID=tid)`, or per-request `TokenRequestOptions.TenantID` on a credential with `AdditionallyAllowedTenants` | *Multi-tenant azidentity symbols cited from DeepWiki (tier 2) — critic flags for tier-1 confirmation (§10). `AdditionallyAllowedTenants="*"` + a lax issuer check removes the tenant boundary* |
| **State isolation** | Key **all** cache + user data by `(tid, user object id)`, backed by a **persistent, tenant-partitioned** store + JWKS cache with ~24h rotation | Today's process-local, in-memory, free-form-label `AccountRegistry` provides none of this; `sub` is unique only within a tenant; `GetByUPN` matches email only (`registry.go`) |

**Seams that exist:** `AccountEntry.TenantID` and per-account `CacheName`. **What must change:** label/UPN lookup replaced by identity *derived from the validated token*, not operator input. **Not addressed anywhere in the source:** revocation / Continuous Access Evaluation relay obligations (surface as `interaction_required` / `WWW-Authenticate` claim challenges the server must forward, not retry), and the interaction between session model choice and per-principal credential caching.

## 8. Migration cost and governance tension

**Change surface — two very different halves.**

| Small & well-isolated (additive, behind a flag) | Large & unbounded (no current analog) |
|---|---|
| Transport swap: ~20 lines in `main.go`, a config flag | Resource-server metadata endpoint wiring |
| Request-scoped credential (one `azcore.TokenCredential` impl) + one middleware reusing `WithGraphClient` | JWKS validation + `401`/`WWW-Authenticate` (mcp-go ships none) |
| Verb registry untouched (transport-agnostic) | Confidential-client OBO + managed secret/cert |
| Existing three credential builders untouched | Per-principal client construction, caching, eviction |
| | Per-request tenant/scope resolution; persistent tenant-partitioned store |

**Governance conflict (this is the decisive point):**
- **`local_first` (load-bearing:high).** The current design is explicitly local-first — stdio, OS-keychain, `~/.outlook-local-mcp/`. A remote multi-tenant OBO server is fully hosted. Per `local_first`, "hosted only when necessary… document why." This is a **governance-level scope change requiring a CR/ADR**, and the local-first exception must be *documented*, not merely CR-gated.
- **Single-purpose / minimize-LoC.** Multi-tenant OBO introduces JWKS validation, a token endpoint, secret rotation, and a persistent store — a **large new subsystem** sitting awkwardly against "many small single-purpose files." The honest framing: the recommended remote-multi-tenant path is a **different product alongside** the local server, not a refactor of it.
- **Discarded asset.** The OS-keychain / encrypted-file cache (and the CLAUDE.md code-signing keychain-ACL workaround) is a local-machine concern with no role in a hosted deployment.
- **Dead UX.** Elicitation-based account selection (`WithElicitation`) is meaningful only for interactive local clients; under a JWT identity model it becomes dead weight or must be per-transport disabled.
- **Ownership gap (unresolved in the source).** No section names *who* registers/owns the middle-tier confidential app for a local-first single-user tool, or who rotates its secret — a real obstacle for the OBO path.

## 9. Comparison table

| Option | Mechanism | Spec-compliant | App-reg burden | Security posture | Multi-tenant fit | Effort |
|---|---|---|---|---|---|---|
| **Status quo (stdio, env creds)** | Server authenticates directly; no inbound token | Yes — stdio is out of scope | None new (public client) | Best (no token crosses a boundary) | N/A (single user) | Zero |
| **C1 client-does-auth (stdio)** | Host authenticates; server reads env/keychain | Yes (sanctioned for stdio) | None new | High | Poor (still one user/process) | Minimal |
| **C2 per-user instances** | One isolated process per user | Yes (each is stdio/local) | None new | High; full isolation by construction | Good, by partition not by code | Low–moderate (orchestration) |
| **C3 public-client remote RS (PKCE)** | Remote RS issues own audience-bound tokens; delegates to Entra as public client | Yes | Moderate (expose API, **no secret**) | Good; audience-validated, no secret to leak | Moderate | Moderate |
| **A — OBO** | Validate `aud`=server; exchange for Graph token | Yes | High (confidential client, secret/cert, scope, consent) | Good; correct two-token boundary | Good with per-tenant authority | High |
| **A + multi-tenant OBO** | OBO per `{tid}` + per-tenant validation & store | Yes | Highest (multi-org, per-tenant consent) | Good *if* issuer/`tid` pinning correct; else cross-tenant bypass | Native | Highest (new subsystem) |
| **B — passthrough** | Forward client's Graph token unchanged | **No — unconditional MUST-NOT** | None (server has no identity) | Worst (confused deputy, full blast radius, broken audit) | Poor | Low but forbidden |
| **C4 API keys** | Static shared credential | No (no delegated identity) | Low | Poor | Poor | Low |

## 10. Recommendation

**Recommended path: stay stdio local-first.** The evidence supports "none of the above" as the correct answer for *this* project. It keeps the trust model the spec sanctions for stdio, preserves `local_first` and single-purpose governance, and requires zero new security-critical code. If a user needs to drive multiple accounts, the existing multi-account registry already covers that server-side.

**If multi-user *serving* is truly required, in ascending cost:**
1. **Per-user instances (C2)** — the governance-reconciling answer. One isolated process per user preserves local-first and sidesteps JWKS/OBO-caching/state-isolation entirely.
2. **Public-client remote resource server (C3)** — if a single shared remote endpoint is mandatory but a confidential registration owner is not available. No secret to manage; audience-bound tokens; PKCE.
3. **Confidential-client OBO (A)**, single-tenant first, then multi-tenant — only with a named app owner and secret-rotation home; treat as a **separate product**, document the `local_first` exception, and route through a CR/ADR *before any code*.

**Never adopt Option B** (passthrough) on any remote surface — forbidden and a confused-deputy vector.

**Concrete next steps:**
- **Author a CR/ADR** framing remote/multi-tenant serving as a new product mode, per `local_first` ("hosted only when necessary, document why") and the single-purpose governance.
- **Prototype an `OnBehalfOfCredential` behind the `TokenCredential` seam** (single-tenant, behind a transport flag) — the cheapest way to prove the Graph-client wiring is genuinely untouched (`auth.go:195` → `main.go:107-109`), without committing to the unbounded multi-tenant subsystem.
- **Design the `(tid, user object id)` isolation key** as the replacement for label/UPN lookup *before* any multi-tenant token store is built.

**Tier-1 verification still needed (critic — check installed source, not docs/DeepWiki):**
1. **mcp-go v0.57.0**: confirm the *absence* of any built-in bearer validation / `401` / `WWW-Authenticate` under any name (current basis is grep-absence + DeepWiki, tier 2) — this single claim carries the whole cost estimate. Also confirm `NewStreamableHTTPServer`, `CallToolRequest.Header`, `WithHTTPContextFunc`.
2. **azidentity v1.14.0**: confirm `AdditionallyAllowedTenants`, `TokenRequestOptions.TenantID`, `resolveTenant` semantics, and whether `OnBehalfOfCredential` reuses one MSAL confidential client per credential instance (the caching/eviction claim, currently medium-confidence and partly inferred; pin to MSAL source). The secret/cert/assertion constructors and `GetToken` interface satisfaction are already tier-1 and need no re-check.

**Approximate, not constant:** the "~1h inbound assertion" lifetime is Entra-policy-dependent and CAE-affected — treat as variable, and design assertion-expiry eviction around the token's actual `exp`, not a fixed hour.
