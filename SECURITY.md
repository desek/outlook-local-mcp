# Security Policy

## Supported Versions

Only the latest release of outlook-local-mcp is supported with security updates.

| Version | Supported |
| ------- | --------- |
| Latest  | Yes       |
| Older   | No        |

## Reporting a Vulnerability

If you discover a security vulnerability in outlook-local-mcp, please report it responsibly.

**Preferred method:** Use [GitHub's private vulnerability reporting](https://github.com/desek/outlook-local-mcp/security/advisories/new) to submit your report. This keeps the details confidential until a fix is available.

**Fallback method:** If private vulnerability reporting is unavailable, contact the maintainer through their [GitHub profile](https://github.com/desek).

**Response target:** You can expect an acknowledgment within 7 calendar days of your report.

**Disclosure policy:** Please do not disclose the vulnerability publicly until a fix has been released. We will coordinate with you on an appropriate disclosure timeline.

## Scope

### In Scope

- outlook-local-mcp server code and its direct dependencies

### Out of Scope

- Microsoft Graph API
- Azure Identity SDK
- OS keychain implementations (e.g., macOS Keychain, Windows Credential Manager, Linux secret service)
