# Privacy

Outlook Local MCP is a local program, not a service. Nobody operates it on your behalf,
there is no account to create, and there is no backend that could collect anything from you
even if it wanted to. This page states what the software actually does with your data, and
every statement below describes behaviour that is visible in the published source.

## The server runs on your machine

The server is a single binary you install and run yourself. It speaks the Model Context
Protocol over standard input and output to the assistant that launched it, so your calendar
and mail content travels between Microsoft, the server, and your assistant, and nowhere
else. No copy of your mailbox is kept: the server queries Microsoft Graph when asked and
returns the answer.

## Where your tokens are stored

Sign-in tokens are held by your operating system's own credential store: the Keychain on
macOS, libsecret on Linux, and DPAPI on Windows. Where the keychain is unavailable, the
server falls back to a local file encrypted with AES-256-GCM under your home directory,
which you can also select deliberately. Either way the tokens stay on the machine that
obtained them, and they are never transmitted anywhere except back to Microsoft when a
token is used or refreshed.

## Where the server connects

Outbound connections go to two Microsoft destinations and nowhere else: the Microsoft
identity platform, for sign-in and token refresh, and the Microsoft Graph API, for every
calendar and mail operation. There is no analytics endpoint, no update check, and no third
party in the path.

There is one exception, and it is off unless you turn it on: the server can export
OpenTelemetry metrics and traces to an OTLP endpoint you configure. Both the switch and the
endpoint default to off and empty, so nothing is exported until you set them, and the
destination is whichever collector you name, typically one running locally.

## The one inbound socket

Interactive browser sign-in briefly binds a loopback port on your own machine so the
browser can hand the authorisation result back to the server. It listens on the local
interface only, it exists solely for that exchange, and it is not a remote-control surface
of any kind.

## Logging

The server writes structured logs, and personal information is sanitised out of them by
default: subjects, attendee addresses, and message content are stripped. Logs stay on your
machine.

## This website

This site is static and collects nothing. It sets no cookies, runs no analytics, embeds no
trackers, and asks for no personal information. It is published from the same public
repository as the server itself.
