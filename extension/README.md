# Outlook Calendar for Claude

Manage Microsoft Outlook calendars and events directly from Claude. This extension connects to the Microsoft Graph API to list calendars, create and search events, check availability, and manage multiple Microsoft accounts -- all through natural language.

## Requirements

This extension requires the **Calendars.ReadWrite** Microsoft Graph API scope. On first use, you will be prompted to authenticate with your Microsoft account and grant this permission. When mail access is enabled (`OUTLOOK_MCP_MAIL_ENABLED=true`), the **Mail.Read** scope is additionally requested for read-only email access (see CR-0043).

## Configuration

| Field | Description |
|-------|-------------|
| **Client ID** | OAuth 2.0 client ID for Microsoft Graph API access. Leave empty to use the default Microsoft Office first-party client ID. |
| **Tenant ID** | Entra ID tenant identifier. Use `common` for any account, `organizations` for work/school only, `consumers` for personal only, or a specific tenant GUID. Defaults to `common`. |

Both fields are optional. The extension works out of the box with default values for most users.

## Usage Examples

**View upcoming events:**

> Show me my calendar events for next week

Uses `list_events` to retrieve events within a time range and display them with details like subject, time, location, and attendees.

**Schedule a meeting:**

> Create a meeting with alice@example.com tomorrow at 2pm for 30 minutes about Q2 planning

Uses `create_event` to create a new calendar event with attendees, time, duration, and subject.

**Check availability:**

> Check if I'm free on Friday afternoon

Uses `get_free_busy` to query your calendar for busy periods and report available time slots.

## Privacy

All data processing occurs locally on your machine. This extension communicates directly with the Microsoft Graph API using your own credentials. No calendar data, account information, or credentials are sent to any third-party server. See the [Privacy Policy](https://github.com/desek/outlook-local-mcp/blob/main/PRIVACY.md) for full details.
