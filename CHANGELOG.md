# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.3.0](https://github.com/desek/outlook-local-mcp/compare/v0.2.1...v0.3.0) (2026-04-20)


### Features

* **auth:** UPN-based account identity and lifecycle management (CR-0056) ([#16](https://github.com/desek/outlook-local-mcp/issues/16)) ([962540a](https://github.com/desek/outlook-local-mcp/commit/962540afb36601fd57345f6403927da6c8dfce32))
* **mail:** complete mail management with reliability fixes (CR-0058) ([#19](https://github.com/desek/outlook-local-mcp/issues/19)) ([e17e86a](https://github.com/desek/outlook-local-mcp/commit/e17e86a8266680bc47bfeb6898a38403ca6aade8))

## [0.2.1](https://github.com/desek/outlook-local-mcp/compare/v0.2.0...v0.2.1) (2026-04-06)


### Bug Fixes

* rename azure ad to entra id ([#15](https://github.com/desek/outlook-local-mcp/issues/15)) ([dd7e517](https://github.com/desek/outlook-local-mcp/commit/dd7e517b6d3ce87615666136fb48998f5b4144d5))
* user confirmation prompts ([#12](https://github.com/desek/outlook-local-mcp/issues/12)) ([ba851db](https://github.com/desek/outlook-local-mcp/commit/ba851dbc2615a754687c0cdb4fc182a832cf6345))

## [0.2.0](https://github.com/desek/outlook-local-mcp/compare/v0.1.0...v0.2.0) (2026-03-22)


### Features

* **tools:** add MCP tool annotations for directory compliance ([#8](https://github.com/desek/outlook-local-mcp/issues/8)) ([049bebb](https://github.com/desek/outlook-local-mcp/commit/049bebb415e41df6dbe9c5f6359edf2705144d52))

## [0.1.0](https://github.com/desek/outlook-local-mcp/compare/v0.0.1...v0.1.0) (2026-03-22)


### Features

* initial release ([dbf0375](https://github.com/desek/outlook-local-mcp/commit/dbf0375856a841e1c1ff91ec58cc4ab01bfaec3e))

## [Unreleased]

### Added

- MCP server communicating over stdio (JSON-RPC) backed by Microsoft Graph API v1.0
- 9 calendar tools: list_calendars, list_events, get_event, search_events, get_free_busy, create_event, update_event, delete_event, cancel_meeting
- 3 account management tools: add_account, list_accounts, remove_account
- Multi-account support with per-account token isolation
- Lazy authentication via OAuth 2.0 authorization code flow (browser) and device code flow
- Persistent token cache using OS-native secure storage (macOS Keychain, Linux libsecret, Windows DPAPI)
- Read-only mode to disable write operations
- Structured logging with JSON/text format, configurable levels, and PII sanitization
- File logging output with append mode
- Audit logging for every tool invocation
- OpenTelemetry metrics and tracing via OTLP gRPC
- Automatic retry with exponential backoff for transient Graph API errors (429, 503, 504)
- Configurable per-request and graceful shutdown timeouts
- Input validation with OData injection protection
- Graceful shutdown handling for SIGINT/SIGTERM
- MCP Elicitation API support for interactive account selection and authentication prompts
- Graceful elicitation fallback for MCP clients without elicitation support (e.g., Claude Code): all auth feedback is delivered via tool result text (CR-0031)
- Persistent per-account identity configuration via `accounts.json`: accounts added with `add_account` survive server restarts and are restored automatically with silent token acquisition (CR-0032)
- `OUTLOOK_MCP_ACCOUNTS_PATH` environment variable to override the default accounts file location (CR-0032)
