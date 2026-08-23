# About Outlook Local MCP

Outlook Local MCP is a Model Context Protocol server that gives an AI assistant a native
interface to your Microsoft Outlook calendar and mail. It runs as a single local binary on
your own machine and speaks MCP over stdio, so an assistant such as Claude Desktop or
Claude Code can read your week, book a meeting, find a thread, or draft a reply by calling
the server directly rather than by scraping a screen or relaying your data through a hosted
intermediary.

## Open source

The project is open source and published under the MIT Licence. The full source, the
licence text, the change history, and the governance records that explain why the software
is shaped the way it is are all in the public repository. Anyone may read it, build it,
audit what it sends and where, fork it, or adapt it. There is no closed component, no
hosted service you must sign up for, and no telemetry you have not switched on yourself.

## Relationship to the Microsoft Graph API

The server is a client of the Microsoft Graph API. Every calendar and mail operation it
performs is a Graph call made with your own credentials, under the OAuth scopes you consent
to during sign-in, against your own mailbox. Authentication goes to the Microsoft identity
platform; data goes to and from Microsoft Graph. The server holds no mirror of your mailbox
and adds no store of its own between you and Microsoft.

This project is an independent piece of software. It is not affiliated with, endorsed by,
or sponsored by Microsoft, and Microsoft, Outlook, and Microsoft Graph are trademarks of
Microsoft Corporation. Access to Graph, and the behaviour of the API itself, remains
governed by Microsoft's own terms.

## Acknowledgement

Support and testing by [GigWhere](https://gigwhere.com) ❤️
