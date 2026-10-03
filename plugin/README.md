# Outlook Local MCP plugin

This plugin connects Claude to Microsoft Outlook calendar, mail, contacts, and Teams through Microsoft Graph. The server runs on your machine. Tokens stay in the keychain of your operating system.

## Surfaces and platforms

- Surfaces: Claude Code, and Cowork desktop sessions. The plugin does not work in claude.ai chat.
- Platforms: macOS arm64 and Linux amd64.
- Windows: install the `outlook-local-mcp.mcpb` extension in Claude Desktop, or use the release zip from https://github.com/desek/outlook-local-mcp/releases.

## What the launcher does

The launcher `scripts/outlook-local-mcp.sh` runs on the first start of each plugin version. It does these steps:

1. It downloads the raw binary of the pinned plugin version from the GitHub release. The download uses github.com and objects.githubusercontent.com.
2. It verifies the SHA-256 digest against `checksums.txt` in this folder. If this folder has no digest for the version, it uses the `checksums.txt` of the release.
3. It deletes a binary with a wrong digest and stops.
4. It keeps a verified binary in the plugin data folder and runs it.

The launcher sends no other data. The server itself talks only to Microsoft identity and Microsoft Graph.

## Example prompts

Each prompt works once the server is signed in. The first needs no opt-in flag; the second needs `mail_manage_enabled`; the third needs `contacts_enabled` and `teams_enabled`.

1. "Find a 30-minute slot next week that works for alex@contoso.com and me, then book it as 'Roadmap sync'."
2. "Move the newest message from Finance in my Inbox to the Receipts folder, flag it for follow-up, and mark it read."
3. "Look up Priya in my contacts, then summarise the last Teams chat I had with her and draft a reply I can paste."

## Local binary override

Set `OUTLOOK_MCP_PLUGIN_BIN` to the path of an outlook-local-mcp binary to run that binary. The override bypasses the download and the digest verification. Use it only with a binary that you built or verified.

## Privacy Policy

The [privacy policy](../PRIVACY.md) describes the data that the server reads and where it goes. The server keeps your data on your machine and sends nothing to the author.
