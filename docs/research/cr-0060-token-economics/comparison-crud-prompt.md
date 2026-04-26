# CR-0060 Comparison CRUD Prompt

You are exercising an Outlook MCP server end-to-end to produce comparable runtime data across two server surfaces. Use only the Outlook MCP server tools that are already registered in your session. Do not name tools, verbs, or operations in your reasoning — figure out which tool to call from its description.

## Operating rules

- **Account.** Use the default account (do not pass any account selector). Never attempt interactive authentication. If a step would require interactive auth, mark it SKIP and continue.
- **Timezone.** Use `Europe/Amsterdam` for every date/time field that asks for a timezone.
- **Test date.** Pick a date exactly 7 days from today and use it for every event in this run.
- **Read output.** When reading data, prefer the default human-readable text mode. Only request a structured or full-fidelity output when a step explicitly asks you to verify a field that is not visible in the default text.
- **No discovery loops.** Do not browse the tool surface beyond what each step requires. If a step's intent is achievable with a single tool call, make a single tool call.
- **Reporting.** For every numbered step, emit one line: `STEP <n>: PASS|FAIL|SKIP — <one-sentence reason>`. After the last step, emit a final line: `SUMMARY: pass=<N> fail=<N> skip=<N>`.

## Steps

1. Check the server's health/status and confirm at least one account is connected. Record the server version and default timezone.
2. List the available accounts. Confirm at least one is authenticated.
3. List the calendars on the default account. Record the default calendar name.
4. Create a calendar event on the test date from 14:00 to 14:30, subject `MCP Compare Test - <epoch>` (substitute current epoch seconds), location `Test Room`, body `CR-0060 comparison run`, shown as `free`. Save the returned event ID.
5. Search for events on the test date matching the unique epoch substring from step 4. Confirm the new event appears.
6. Fetch the event by ID. Verify subject, location, start time, end time, and that the body preview includes `comparison`.
7. Update the event: append ` (updated)` to the subject, change location to `Updated Room`, extend end time to 15:00, set shown-as to `busy`, replace the body with the HTML `<h2>Agenda</h2><ol><li>Verify</li><li>Review</li></ol>`.
8. Fetch the event again in default text mode. Verify the updated subject, location, end time, and that the body preview contains `Agenda` as plain text (no HTML tags).
9. Fetch the event one more time in full-fidelity / raw mode. Verify the body content contains the literal `<h2>Agenda</h2>` HTML.
10. Retrieve the free/busy schedule for the test date. Verify a busy block overlaps 14:00–15:00.
11. Delete the event. Confirm a success message.
12. Re-fetch the event by ID and verify it is gone (a not-found error counts as PASS).
13. List the mail folders on the default account. Record the Inbox folder ID.
14. List the most recent messages in the Inbox (page size 5 is fine). Record the ID of the top message.
15. Search the Inbox for any keyword present in the top message's subject from step 14. Confirm the top message appears.
16. Fetch the top message by ID in default text mode. Verify subject, sender, and a body preview line are present.
17. Fetch the conversation that the top message belongs to. Verify at least one message is returned and the top message ID is among them. If the surface offers no conversation lookup, mark SKIP.
18. List the attachments on the top message. If there are none, mark PASS with reason "no attachments". If the surface offers no attachment listing, mark SKIP.
19. Create a new mail draft to yourself (the default account's address) with subject `MCP Compare Draft - <epoch>` and a plain-text body. Save the returned draft ID.
20. Update the draft: change the subject to append ` (edited)` and add a second body paragraph.
21. Delete the draft. Confirm a success message.
22. Create a reply draft to the top message from step 14 with body `Compare run reply`. Save the draft ID, then delete it. If the surface offers no reply-draft creation, mark SKIP.
23. Create a forward draft of the top message from step 14 to yourself with body `Compare run forward`. Save the draft ID, then delete it. If the surface offers no forward-draft creation, mark SKIP.
24. Final account list: confirm the default account is still authenticated.

End with the `SUMMARY:` line. Do not produce any other trailing prose.
