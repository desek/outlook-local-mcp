# Privacy Policy

## What Data Is Accessed

This extension accesses the following data through the Microsoft Graph API. Each domain is available only when its gate is on.

Calendar, always on:

- **Calendar events**: Event details including subject, time, location, attendees, body content, and recurrence patterns.
- **Event attachments**: Attachment metadata and content on calendar events.
- **Free/busy and schedules**: Availability, schedule blocks, and working hours for specified mailboxes and time ranges.
- **Account metadata**: Display name and email address of authenticated Microsoft accounts.

Mail, under `OUTLOOK_MCP_MAIL_ENABLED`:

- **Mail messages**: Message details, conversations, and body content.
- **Mail folders**: Folder names and unread counts.
- **Mail attachments**: Attachment metadata and content on received messages.

Mail management, under `OUTLOOK_MCP_MAIL_MANAGE_ENABLED`:

- **Drafts**: Drafts that you create, reply, forward, update, or delete.
- **Received-message writes**: Folder moves, follow-up flags, categories, and read state on received messages.
- **Draft attachments**: Files that you attach to a draft.

Contacts, under `OUTLOOK_MCP_CONTACTS_ENABLED`:

- **Contacts and people**: Saved personal contacts and relevance-ranked people with their email addresses.

Teams, under `OUTLOOK_MCP_TEAMS_ENABLED`:

- **Teams chats**: Chats of the signed-in user and their messages.
- **Channel messages**: Channel posts and their replies.
- **Online meetings**: Meeting details resolved from a meeting ID or join URL.
- **Transcripts**: Meeting transcript metadata and text.

## How Data Is Processed

All data processing occurs locally on your machine. The extension runs as a local binary that communicates directly with the Microsoft Graph API. No data is routed through, processed by, or stored on any intermediate server.

## Credentials Storage

- **OAuth tokens** are stored in your operating system's native keychain (macOS Keychain, Windows Credential Manager).
- **Authentication records** are stored on disk in your user profile directory to enable silent token refresh.
- No credentials are transmitted to any server other than Microsoft's authentication endpoints.

## Third-Party Services

The extension contacts **Microsoft Graph API** (`graph.microsoft.com`) and **Microsoft Identity Platform** (`login.microsoftonline.com`) exclusively. When you install the extension as a Claude plugin, the plugin launcher also contacts **GitHub** (`github.com` and `objects.githubusercontent.com`) once per version to download the release binary. No other third-party services are contacted.

## Data Retention

- **OAuth tokens and authentication records** stay until you sign out or remove the account with `account.remove`.
- **The optional log file** stays until you delete it. The server removes personal data from it by default.
- **The plugin launcher** keeps the downloaded binary under `${CLAUDE_PLUGIN_DATA}` until you uninstall the plugin.
- **Message, event, and contact content** is never written to disk by the server.

## No Data Collection

The extension author does not collect, transmit, store, or have access to any of your data. All communication occurs directly between your machine and Microsoft's services using your own credentials.

## Contact

- **Questions and issues**: [GitHub issues](https://github.com/desek/outlook-local-mcp/issues).
- **Security reports**: [GitHub private vulnerability reporting](https://github.com/desek/outlook-local-mcp/security/advisories/new).
