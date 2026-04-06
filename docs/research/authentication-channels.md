# Research: Alternative Authentication Channels for Outlook Calendar MCP Server

**Date**: 2026-03-14
**Status**: Research Complete
**Current Implementation**: `azidentity.DeviceCodeCredential` with persistent OS-native token cache and `AuthenticationRecord` persistence

## Executive Summary

This document evaluates all viable authentication channels for the Outlook Calendar MCP server beyond the current device code flow. The evaluation covers 16 `azidentity` credential types, macOS Keychain token reuse, Azure CLI authentication, broker-based auth, and patterns used by other Microsoft Graph MCP servers.

**Conclusion**: The current `DeviceCodeCredential` implementation is the optimal choice for a stdio-based MCP server requiring delegated `Calendars.ReadWrite` permissions. The only viable alternative is `InteractiveBrowserCredential`, which trades headless compatibility for a slightly more seamless desktop UX. All other approaches are blocked by fundamental technical or security constraints.

---

## Table of Contents

1. [Current Architecture](#1-current-architecture)
2. [Viable Alternatives](#2-viable-alternatives)
   - [InteractiveBrowserCredential](#21-interactivebrowsercredential)
3. [Investigated but Not Viable](#3-investigated-but-not-viable)
   - [Azure CLI Credential](#31-azure-cli-credential)
   - [macOS Keychain Token Reuse](#32-macos-keychain-token-reuse)
   - [Broker-Based Auth (WAM / Enterprise SSO)](#33-broker-based-auth-wam--enterprise-sso)
   - [MSAL Shared Token Cache](#34-msal-shared-token-cache)
   - [Client Certificate Auth](#35-client-certificate-authentication)
   - [ROPC (Username/Password)](#36-ropc-resource-owner-password-credentials)
   - [Application-Only Credentials](#37-application-only-credentials)
   - [Infrastructure Credentials](#38-infrastructure-credentials)
4. [MCP Ecosystem Comparison](#4-mcp-ecosystem-comparison)
5. [Complete azidentity Credential Matrix](#5-complete-azidentity-credential-matrix)
6. [Recommendation](#6-recommendation)
7. [Sources](#7-sources)

---

## 1. Current Architecture

The server uses `azidentity.DeviceCodeCredential` (v1.13.1) configured with:

- **Scope**: `Calendars.ReadWrite` (delegated)
- **Client ID**: `d3590ed6-52b3-4102-aeff-aad2292ab01c` (Microsoft Office first-party)
- **Token cache**: OS-native persistent cache via `azidentity/cache` (macOS Keychain)
- **Auth record**: JSON file at `~/.outlook-local-mcp/auth_record.json` (0600 permissions)
- **Lazy auth**: Authentication deferred to first tool call, not startup
- **Re-auth**: Automatic via `AuthMiddleware` when token errors are detected
- **CAE**: Continuous Access Evaluation enabled
- **UX integration**: Device code message forwarded via MCP `LoggingMessageNotification`

This approach provides full control over requested scopes, works without a local browser, integrates with the MCP notification protocol, and supports silent token refresh across server restarts.

---

## 2. Viable Alternatives

### 2.1 InteractiveBrowserCredential

| Attribute | Detail |
|---|---|
| **Delegated permissions** | Yes |
| **Flow** | OAuth 2.0 authorization code with PKCE. Opens system browser to Microsoft login page; localhost HTTP server receives the redirect callback. |
| **User interaction** | Yes -- browser-based login UI |
| **Persistent cache** | Yes -- supports `Cache` field + `AuthenticationRecord` (same pattern as current impl) |
| **Scope control** | Full -- can request `Calendars.ReadWrite` specifically |
| **Prerequisites** | App registration with redirect URI `http://localhost`. Public client setting enabled. |

**Advantages over DeviceCodeCredential**:
- More seamless desktop UX -- user sees the familiar Microsoft login page directly (one click vs. copy-paste a code)
- Authorization code flow is considered more secure than device code flow

**Disadvantages**:
- Requires a local GUI browser -- fails in headless environments (SSH, containers)
- Browser popup is opaque to the MCP agent -- cannot relay the login URL through MCP notifications the way device code messages can
- Starts a temporary localhost HTTP server for the OAuth redirect, which may conflict with firewalls or security policies
- The UX is jarring in an agent-driven MCP context (a browser window appears unprompted)

**Implementation effort**: Low. The `InteractiveBrowserCredential` supports the same `Cache` + `AuthenticationRecord` pattern already used. The main change would be replacing the `UserPrompt` callback with redirect URL configuration.

**Verdict**: The only true alternative. Could be offered as a user-configurable option alongside device code for desktop-only users who prefer browser-based login.

---

## 3. Investigated but Not Viable

### 3.1 Azure CLI Credential

| Attribute | Detail |
|---|---|
| **Type** | `azidentity.AzureCLICredential` |
| **How it works** | Shells out to `az account get-access-token --resource <resource>` |
| **Blocker** | **Cannot obtain `Calendars.ReadWrite` tokens** |

The Azure CLI uses a first-party app registration (client ID `04b07795-8ddb-461a-bbee-02f9e1bf7b46`) with a fixed set of pre-authorized Microsoft Graph scopes:

- `AuditLog.Read.All`
- `Directory.AccessAsUser.All`
- `Group.ReadWrite.All`
- `User.ReadWrite.All`

**`Calendars.ReadWrite` is not in this set.** Attempting to request it via `az login --scope https://graph.microsoft.com/Calendars.ReadWrite` fails with `AADSTS65002`:

> "Consent between first party application and first party resource must be configured via preauthorization -- applications owned and operated by Microsoft must get approval from the API owner before requesting tokens for that API."

This is a hard limitation of the CLI's app registration that users cannot modify. Confirmed by multiple open GitHub issues ([#12986](https://github.com/Azure/azure-cli/issues/12986), [#30149](https://github.com/Azure/azure-cli/issues/30149)). A feature request for custom client ID support ([#22775](https://github.com/Azure/azure-cli/issues/22775)) remains unimplemented.

Additional limitations:
- No persistent caching in the credential itself (spawns a subprocess per `GetToken` call)
- No `AuthenticationRecord` support
- Requires Azure CLI installed as an external dependency
- Scope conversion strips specific scopes -- only supports `.default` pattern

**Verdict**: Blocked. Cannot obtain calendar-scoped tokens.

### 3.2 macOS Keychain Token Reuse

Investigated whether tokens from existing Microsoft applications (Outlook, Edge, Teams) in the macOS Keychain could be reused.

**Approach 1: Outlook/Office Keychain tokens**

Microsoft apps cache tokens under the Keychain access group `com.microsoft.identity.universalstorage` (Team ID `UBF8T346G9`). Two independent barriers block access:

1. **Apple Team ID enforcement**: Keychain access groups are prefixed with the developer's Team ID, validated against the app's code signature. A third-party app cannot access items in `UBF8T346G9.com.microsoft.identity.universalstorage`.
2. **Client ID binding**: Tokens are issued for specific client IDs. Microsoft's token endpoint rejects refresh token redemption from a different client ID.

**Approach 2: Microsoft FOCI (Family of Client IDs)**

Microsoft maintains an undocumented feature where certain first-party apps share "family refresh tokens." However, FOCI membership is restricted to Microsoft's own client IDs -- third-party app registrations cannot join the family. FOCI is also monitored as an attack vector.

**Approach 3: Azure CLI's file-based token cache**

Azure CLI stores its MSAL cache at `~/.azure/msal_token_cache.json`. While readable by any same-user process, the tokens are bound to the CLI's client ID and do not include calendar scopes (see section 3.1).

**Approach 4: Edge browser tokens**

While extraction of Edge's cached refresh tokens is technically possible (documented by security researchers), this constitutes token theft and is detected by Microsoft's anomalous token risk detections.

**Verdict**: Blocked by Apple's Keychain Team ID enforcement, Microsoft's client ID binding, and the absence of calendar scopes. Token extraction from other apps is unsupported and constitutes token theft.

### 3.3 Broker-Based Auth (WAM / Enterprise SSO)

On macOS, Microsoft's authentication broker is the **Enterprise SSO plug-in** hosted in Company Portal (replacing Windows-specific WAM). Three barriers block usage:

1. **No Go support**: `azidentity` for Go does not have a broker plugin. There is no `azure-identity-broker` equivalent package for Go. Only .NET, Python, and ObjC/Swift SDKs support the macOS broker.
2. **CLI executables blocked**: The macOS broker blocks requests from signed executables that are not bundled `.app` applications. A Go binary is rejected.
3. **Apple networking frameworks required**: The non-MSAL SSO allowlist path requires apps to use `WKWebView` or `NSURLSession` for auth. Go's `net/http` stack does not qualify.
4. **MDM enrollment required**: The Enterprise SSO plug-in requires Intune-enrolled devices.

**Verdict**: Blocked. No Go SDK support, CLI binaries rejected by broker, MDM required.

### 3.4 MSAL Shared Token Cache

| Cache Location | Encryption | Accessible from Go? |
|---|---|---|
| Keychain (`com.microsoft.identity.universalstorage`) | Keychain | No (Team ID) |
| Azure CLI (`~/.azure/msal_token_cache.json`) | Optional | Yes, but wrong scopes |
| MSAL Go (in-memory by default) | N/A | N/A |

No legitimate shared cache is accessible to a third-party Go app with the correct scopes.

**Verdict**: Blocked. Same barriers as Keychain token reuse.

### 3.5 Client Certificate Authentication

Certificate-based auth in Microsoft Entra ID is tied to the **client credentials flow**, which yields **application permissions** only. There is no way to use certificate auth to obtain delegated user tokens through any Microsoft-supported flow.

Application permissions for `Calendars.ReadWrite` would grant access to ALL users' calendars in the tenant -- inappropriate for a personal CLI tool.

**Verdict**: Not applicable. Certificate auth does not support delegated permissions.

### 3.6 ROPC (Resource Owner Password Credentials)

`UsernamePasswordCredential` is **deprecated** in `azidentity` (underlying MSAL method deprecated in v1.6.0).

- Incompatible with MFA (Microsoft is mandating MFA across all Entra ID tenants)
- Only works with work/school accounts, not personal Microsoft accounts
- Requires storing user passwords -- a security anti-pattern
- Being removed from OAuth 2.1 (which MCP has adopted)
- Exchange Online removing ROPC support after June 2026

**Verdict**: Deprecated and actively being phased out. Not recommended.

### 3.7 Application-Only Credentials

The following credentials authenticate as an application, not a user:

| Credential | Flow |
|---|---|
| `ClientSecretCredential` | Client credentials with secret |
| `ClientCertificateCredential` | Client credentials with certificate |
| `ClientAssertionCredential` | Client credentials with federated assertion |

All yield application-level permissions. `Calendars.ReadWrite` as an application permission grants access to every user's calendar in the tenant with admin consent -- not appropriate for a personal tool.

**Verdict**: Wrong permission model for this use case.

### 3.8 Infrastructure Credentials

The following are designed for specific Azure hosting environments:

| Credential | Environment | Delegated? |
|---|---|---|
| `ManagedIdentityCredential` | Azure VMs, App Service, Functions, AKS | No |
| `WorkloadIdentityCredential` | Kubernetes with federation | No |
| `AzurePipelinesCredential` | Azure Pipelines CI/CD | No |

None support delegated permissions or run on local machines.

**Verdict**: Not relevant for a local MCP server.

---

## 4. MCP Ecosystem Comparison

How other Microsoft Graph MCP servers handle authentication:

| Project | Language | Auth Method | Token Storage | Delegated? |
|---|---|---|---|---|
| **microsoft/EnterpriseMCP** | -- | OAuth 2.1 delegated | -- | Yes |
| **merill/lokka** | Node.js | Browser, certificate, client secret, client-provided token | Config-based | Mixed |
| **Softeria/ms-365-mcp-server** | -- | Device code (stdio), auth code (HTTP), BYO token | OS credential store + file fallback | Yes |
| **elyxlz/microsoft-mcp** | Python | Device code | Plaintext JSON file | Yes |
| **kgatilin/outlookmcp** | Go | Auth code with localhost callback | In-memory only | Yes |
| **This project** | Go | Device code + persistent OS cache + auth record + MCP notifications + CAE | macOS Keychain + auth record file | Yes |

**Key observations**:
- Device code flow is the dominant pattern for stdio-based MCP servers
- This project's implementation is the most mature: OS-native cache, auth record persistence, MCP notification integration, CAE support, and automatic re-authentication middleware
- No other Go-based MCP server uses `AzureCLICredential` or broker auth for Graph calendar access

The MCP specification (2025-11-25) explicitly states that stdio-based servers should use "environment-based credentials or credentials provided by third-party libraries embedded directly in the MCP server" rather than the full OAuth 2.1 resource server pattern designed for HTTP transports.

---

## 5. Complete azidentity Credential Matrix

All 16 credential types in `azidentity` v1.13.1 evaluated:

| Credential | Delegated? | Interactive? | Cache? | Specific Scopes? | Suitability |
|---|---|---|---|---|---|
| `DeviceCodeCredential` | Yes | Yes | Yes | Yes | **Current -- optimal** |
| `InteractiveBrowserCredential` | Yes | Yes (browser) | Yes | Yes | **Viable alternative** |
| `AzureCLICredential` | Yes | No | No | No (`.default`) | Blocked (no calendar scopes) |
| `AzureDeveloperCLICredential` | Yes | No | No | No (`.default`) | Same limitation as CLI |
| `AzurePowerShellCredential` | Yes | No | No | No (`.default`) | Same limitation as CLI |
| `UsernamePasswordCredential` | Yes | No | No | Yes | **Deprecated** |
| `OnBehalfOfCredential` | Yes (OBO) | No | No | Yes | Wrong architecture |
| `DefaultAzureCredential` | Varies | No | No | No | Not suitable |
| `ChainedTokenCredential` | Varies | Varies | No | Varies | Architectural only |
| `ManagedIdentityCredential` | No | No | No | N/A | Azure infra only |
| `ClientSecretCredential` | No | No | Yes | N/A | App-only |
| `ClientCertificateCredential` | No | No | Yes | N/A | App-only |
| `ClientAssertionCredential` | No | No | Yes | N/A | App-only |
| `EnvironmentCredential` | Deprecated path | No | No | No | Not suitable |
| `WorkloadIdentityCredential` | No | No | Yes | N/A | Kubernetes only |
| `AzurePipelinesCredential` | No | No | Yes | N/A | CI/CD only |

---

## 6. Recommendation

**Keep `DeviceCodeCredential` as the primary (and default) authentication method.** It is the best fit for a stdio-based MCP server requiring delegated calendar permissions.

**Optional enhancement**: Offer `InteractiveBrowserCredential` as a user-configurable alternative (e.g., via `OUTLOOK_MCP_AUTH_METHOD=browser`) for desktop users who prefer browser-based login. Both credentials support the same `Cache` + `AuthenticationRecord` pattern, so the implementation would be a straightforward extension of the existing auth setup.

No other authentication channel is viable for this use case due to:
- Azure CLI's inability to obtain `Calendars.ReadWrite` tokens
- Apple's Keychain Team ID enforcement blocking token reuse from Microsoft apps
- Absence of broker/WAM support in the Go `azidentity` SDK
- Deprecation of ROPC
- Application-only credentials granting overly broad tenant-wide access

---

## 7. Sources

### Azure SDK and Identity
- [azidentity package - Go Packages](https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/azidentity)
- [Azure SDK for Go azidentity README](https://github.com/Azure/azure-sdk-for-go/blob/main/sdk/azidentity/README.md)
- [Credential chains in Azure Identity for Go](https://learn.microsoft.com/en-us/azure/developer/go/sdk/authentication/credential-chains)
- [Additional authentication methods - Go on Azure](https://learn.microsoft.com/en-us/azure/developer/go/sdk/authentication/authentication-additional-methods)
- [Azure SDK for Go token caching](https://github.com/Azure/azure-sdk-for-go/blob/main/sdk/azidentity/TOKEN_CACHING.MD)

### Azure CLI Limitations
- [Request ms-graph token with scope "calendars.read" - Azure CLI #12986](https://github.com/Azure/azure-cli/issues/12986)
- [Not able to create a valid ms-graph token - Azure CLI #30149](https://github.com/Azure/azure-cli/issues/30149)
- [Feature Request: Support custom client ID - Azure CLI #22775](https://github.com/Azure/azure-cli/issues/22775)
- [azure_cli_credential.go source](https://github.com/Azure/azure-sdk-for-go/blob/main/sdk/azidentity/azure_cli_credential.go)

### macOS Token Security
- [Microsoft Enterprise SSO plug-in for Apple devices](https://learn.microsoft.com/en-us/entra/identity-platform/apple-sso-plugin)
- [Configure keychain - MSAL for iOS/macOS](https://learn.microsoft.com/en-us/entra/msal/objc/howto-v2-keychain-objc)
- [Using MSAL.NET with the macOS broker](https://learn.microsoft.com/en-us/entra/msal/dotnet/acquiring-tokens/desktop-mobile/macos-broker-dotnet-sdk)
- [Apple Keychain Access Groups Entitlement](https://developer.apple.com/documentation/bundleresources/entitlements/keychain-access-groups)
- [Abuse and replay of Entra ID token from Edge in macOS Keychain](https://www.cloud-architekt.net/abuse-and-replay-azuread-token-macos/)
- [Family of Client IDs Research - Secureworks](https://github.com/secureworks/family-of-client-ids-research)
- [macOS Platform SSO overview](https://learn.microsoft.com/en-us/entra/identity/devices/macos-psso)

### Broker Authentication
- [WAM Broker Support - Azure SDK Blog](https://devblogs.microsoft.com/azure-sdk/wham-authentication-broker-support-lands-in-the-azure-identity-libraries/)
- [Using MSAL Python with macOS broker](https://learn.microsoft.com/en-us/entra/msal/python/advanced/macos-broker)
- [MSAL for Go documentation](https://learn.microsoft.com/en-us/entra/msal/go/)

### MCP Authentication Standards
- [MCP Authorization Specification](https://modelcontextprotocol.io/docs/tutorials/security/authorization)
- [MCP Spec Updates - Auth0](https://auth0.com/blog/mcp-specs-update-all-about-auth/)
- [OAuth for MCP Enterprise Patterns - GitGuardian](https://blog.gitguardian.com/oauth-for-mcp-emerging-enterprise-patterns-for-agent-authorization/)
- [Authentication and Authorization in MCP - Stack Overflow](https://stackoverflow.blog/2026/01/21/is-that-allowed-authentication-and-authorization-in-model-context-protocol/)

### Competing MCP Servers
- [microsoft/EnterpriseMCP](https://github.com/microsoft/EnterpriseMCP)
- [merill/lokka](https://github.com/merill/lokka)
- [Softeria/ms-365-mcp-server](https://github.com/Softeria/ms-365-mcp-server)
- [elyxlz/microsoft-mcp](https://github.com/elyxlz/microsoft-mcp)
- [kgatilin/outlookmcp](https://github.com/kgatilin/outlookmcp)

### Microsoft Identity Platform
- [Choose a Microsoft Graph authentication provider](https://learn.microsoft.com/en-us/graph/sdks/choose-authentication-providers)
- [ROPC Documentation](https://learn.microsoft.com/en-us/entra/identity-platform/v2-oauth-ropc)
- [Mandatory MFA for Microsoft Entra](https://learn.microsoft.com/en-us/entra/identity/authentication/concept-mandatory-multifactor-authentication)
- [Configurable Token Lifetimes](https://learn.microsoft.com/en-us/entra/identity-platform/configurable-token-lifetimes)
- [Protecting Tokens in Microsoft Entra ID](https://learn.microsoft.com/en-us/entra/identity/devices/protecting-tokens-microsoft-entra-id)
