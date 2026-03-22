# Privacy Policy

## What Data Is Accessed

This extension accesses the following data through the Microsoft Graph API:

- **Calendar events**: Event details including subject, time, location, attendees, body content, and recurrence patterns.
- **Free/busy status**: Availability information for specified time ranges.
- **Account metadata**: Display name and email address of authenticated Microsoft accounts.

## How Data Is Processed

All data processing occurs locally on your machine. The extension runs as a local binary that communicates directly with the Microsoft Graph API. No data is routed through, processed by, or stored on any intermediate server.

## Credentials Storage

- **OAuth tokens** are stored in your operating system's native keychain (macOS Keychain, Windows Credential Manager).
- **Authentication records** are stored on disk in your user profile directory to enable silent token refresh.
- No credentials are transmitted to any server other than Microsoft's authentication endpoints.

## Third-Party Services

The extension contacts **Microsoft Graph API** (`graph.microsoft.com`) and **Microsoft Identity Platform** (`login.microsoftonline.com`) exclusively. No other third-party services are contacted.

## No Data Collection

The extension author does not collect, transmit, store, or have access to any of your data. All communication occurs directly between your machine and Microsoft's services using your own credentials.
