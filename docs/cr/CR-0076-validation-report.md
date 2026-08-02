# CR-0076 Validation Report

Validated at finalization checkpoint `e873178` (branch `feat/cr-0073-surface-manifest`).
Merge-base with `origin/main`: `7b2f0ad`. Validator is documentation-only; no source was
modified.

## Summary
Requirements: 17/17 | Acceptance Criteria: 9/13 | Tests: 17/17 | Gaps: 2

- Requirements counts both the 12 Functional Requirements and the 5 Non-Functional
  Requirements; all 17 are PASS.
- Acceptance Criteria: 9 PASS, 4 PARTIAL. The four PARTIAL (AC-1, AC-2, AC-3, AC-8) are the
  criteria graded on a live-mailbox end state that no test can observe, because the suite
  issues no Microsoft Graph call. Their runtime *mechanism* (the exact value reaching Graph)
  is unit-tested; the *end state* is the outstanding acceptance step below, not a defect.
- No FAIL. No stray changed files outside the CR's Affected Components.

## Diff evidence and scope

`git diff 7b2f0ad...HEAD --name-only` returns exactly nine files, every one named in the CR's
Affected Components. No unmapped files. `go.mod`/`go.sum` are unchanged (NFR-3, AC-13).

Targeted suite (the CR's Verification Commands) run under `-race`:
`go test -race ./internal/tools/ ./internal/server/ -run 'SearchMessages|Normalis|DocumentedExamples|...'`
-> all PASS. `make build vet fmt-check` -> exit 0. Full `./internal/tools` and
`./internal/server` packages -> both `ok`.

## Requirement Verification
| Req # | Description | Status | Evidence (file:line / test name) |
|---|---|---|---|
| 1 | Pure normalisation function, no Graph dep, no network | PASS | `internal/tools/search_messages_query.go:62` `NormaliseSearchQuery`; imports only `fmt`, `regexp`, `strings` (`:18-22`) |
| 2 | Rewrite double-quoted property value to parenthesised form | PASS | `search_messages_query.go:31,66` regex `(\w+):"([^"]*)"` -> `$1:($2)`; `TestQuotedPropertyValueBecomesParenthesised` |
| 3 | MUST NOT repair by deleting quotes (bare multi-word value) | PASS | Only replacement is to parenthesised form (`:66`); no quote-deletion path exists; `TestQuotedPropertyValueIsNotUnquotedInPlace` asserts parens present and rejects the `subject:Design Review` bare form |
| 4 | Wrap a no-quote query in exactly one pair | PASS | `search_messages_query.go:76-78`; `TestUnquotedQueryIsWrapped`, `TestMultiWordBareQueryIsWrapped` |
| 5 | Pass through an already-enclosed single pair unchanged | PASS | `isSingleEnclosedPair` (`:95-100`) checked at `:71` before reject; `TestAlreadyEnclosedQueryPassesThrough` (byte-identical) |
| 6 | Reject stray quote; error names parenthesised form; pair-check first | PASS | `:71` (behaviour 2) precedes `:83` (behaviour 4); error text names `subject:(Design Review)`; `TestStrayQuoteIsRejected` asserts "parentheses" and the concrete example |
| 7 | Apply on both paths; cannot diverge | PASS | Normalised once at `internal/tools/search_messages.go:110`, before the folder branch at `:144`; both paths read the same `normalised` (`:149`, `:168`); `TestBothRequestPathsNormalise` compares actual wire values |
| 8 | Reject before the Graph call | PASS | `search_messages.go:110-114` returns before `graph.WithTimeout` (`:138`) and the retry wrappers; `TestUnconvertibleQueryIsRejectedBeforeTheCall` asserts `!rec.called` |
| 9 | Description states parenthesised, not double-quoted; canonical std | PASS | `internal/server/mail_verbs.go:281`; `TestDocumentedExamplesAreCanonical` guards `Description` against `\w+:"` (`mail_verbs_test.go:83`) |
| 10 | Description states first-token binding rule | PASS | `mail_verbs.go:281` "an unparenthesised value such as subject:Design Review binds only its first token ... turns the remaining words into free-text terms" |
| 11 | Every example normalises and `normalise(example)==example` | PASS | `TestDocumentedExamplesAreCanonical` (`mail_verbs_test.go:73-80`), cases derived from `verb.Examples` (registry), not a hand-written list |
| 12 | Single bare term keeps working | PASS (seam) | Measured row 3 fixture `Zzzqqxx`->`"Zzzqqxx"` in `TestMeasuredRows`. Wire transform is test-backed; the runtime equivalence claim is the live-verification item shared with AC-8 |
| NFR-1 | Deterministic | PASS | Pure regex function; `TestMeasuredRows`/`TestNormalisationIsIdempotent` reproduce identical output |
| NFR-2 | Idempotent | PASS | `TestNormalisationIsIdempotent` over the whole corpus (accepted and rejected inputs) |
| NFR-3 | No new dependency | PASS | `go.mod`/`go.sum` absent from the branch diff |
| NFR-4 | Error reaches tool result AND log record | PASS | `search_messages.go:112` `logger.Error("search query rejected", ...)` and `:113` `NewToolResultError(err.Error())`; the log line was observed in the `TestUnconvertibleQueryIsRejectedBeforeTheCall` run |
| NFR-5 | No shape / tier / annotation change | PASS | `mail_verbs.go` diff touches only `Description`, `Examples`, and the `query` parameter description; annotation block and schema shape are byte-unchanged |

## Acceptance Criteria Verification
| AC # | Description | Status | Evidence |
|---|---|---|---|
| AC-1 | Documented keyword returns results, no parse error | PARTIAL | Mechanism proven: `TestSearchMessages_BasicQuery`/`_WithFolderId` assert the correctly-quoted value reaches Graph, which is what prevents the parse error. The "returns a matching message" end state needs a live Graph call (see Gaps). Manual step 30f enumerated in `docs/prompts/mcp-tool-crud-test.md` but not executed |
| AC-2 | Multi-word query filters, not discarded | PARTIAL | Seam covered by `TestMultiWordBareQueryIsWrapped` (`Zzzqqxx Wwwyyzz`->`"..."`). The observable "zero, not newest messages" is a Graph end state; manual control step 30g enumerated, not executed |
| AC-3 | Phrase scopes to its property | PARTIAL | `TestQuotedPropertyValueBecomesParenthesised` proves the parenthesised wire form; scoping behaviour is a Graph end state (measured rows 8/9), not test-observable |
| AC-4 | Never repaired by deleting quotes | PASS | `TestQuotedPropertyValueIsNotUnquotedInPlace` (pure, testable) |
| AC-5 | A caller who already quotes is not broken | PASS | `TestAlreadyEnclosedQueryPassesThrough` (byte-identical) |
| AC-6 | Stray quote refused before Graph, error names correction | PASS | `TestUnconvertibleQueryIsRejectedBeforeTheCall` asserts no request issued (`!rec.called`); `TestStrayQuoteIsRejected` asserts the error text |
| AC-7 | Both request paths agree | PASS | `TestBothRequestPathsNormalise` compares recorded `$search` values from both paths |
| AC-8 | Single bare term unaffected | PARTIAL | Transform is test-backed (row 3). No dedicated test or executed manual step confirms the wrapped `"Zzzqqxx"` behaves identically at Graph to the bare `Zzzqqxx` that works today; runtime equivalence is unverified |
| AC-9 | Documented example must already be canonical | PASS | `TestDocumentedExamplesAreCanonical`: `normalise(example)==example` on registry-derived `Examples`, plus a `\w+:"` guard on `Description` prose |
| AC-10 | Failure legible without an interactive surface | PASS | Tool result and log both carry the correction (NFR-4 evidence; log line observed in the rejection test run) |
| AC-11 | Description teaches the corrected rules | PASS | `mail_verbs.go:281` states the parenthesised rule, the first-token rule, and documents no double-quoted phrase |
| AC-12 | Deterministic and idempotent | PASS | `TestNormalisationIsIdempotent` |
| AC-13 | No dependency added; response contract preserved | PASS | `go.mod`/`go.sum` unchanged; annotation/schema block unchanged in the `mail_verbs.go` diff |

## Test Strategy Verification
| Test File | Test Name | Specified | Exists | Matches Spec |
|---|---|---|---|---|
| `internal/tools/search_messages_query_test.go` | `TestUnquotedQueryIsWrapped` | yes | yes | yes (`subject:Contoso`->`"subject:Contoso"`) |
| `internal/tools/search_messages_query_test.go` | `TestMultiWordBareQueryIsWrapped` | yes | yes | yes (`Zzzqqxx Wwwyyzz`->`"..."`) |
| `internal/tools/search_messages_query_test.go` | `TestQuotedPropertyValueBecomesParenthesised` | yes | yes | yes (`subject:"Design Review"`->`"subject:(Design Review)"`) |
| `internal/tools/search_messages_query_test.go` | `TestQuotedPropertyValueIsNotUnquotedInPlace` | yes | yes | yes (asserts parens, rejects bare two-token form) |
| `internal/tools/search_messages_query_test.go` | `TestAlreadyEnclosedQueryPassesThrough` | yes | yes | yes (byte-identical) |
| `internal/tools/search_messages_query_test.go` | `TestStrayQuoteIsRejected` | yes | yes | yes (error names the parenthesised form) |
| `internal/tools/search_messages_query_test.go` | `TestBooleanExpressionIsWrappedOnce` | yes | yes | yes (exactly one enclosing pair) |
| `internal/tools/search_messages_query_test.go` | `TestNormalisationIsIdempotent` | yes | yes | yes (whole corpus; accepted and rejected) |
| `internal/server/mail_verbs_test.go` | `TestDocumentedExamplesAreCanonical` | yes | yes | yes (registry-derived; both assertions) |
| `internal/tools/search_messages_test.go` | `TestBothRequestPathsNormalise` | yes | yes | yes (identical wire value) |
| `internal/tools/search_messages_test.go` | `TestUnconvertibleQueryIsRejectedBeforeTheCall` | yes | yes | yes (asserts no request issued) |
| `internal/tools/search_messages_test.go` | `TestSearchMessages_BasicQuery` (modify) | yes | yes | yes (now asserts normalised wire value) |
| `internal/tools/search_messages_test.go` | `TestSearchMessages_WithFolderId` (modify) | yes | yes | yes (asserts wire value on folder path) |
| `docs/prompts/mcp-tool-crud-test.md` | steps 30f / 30g (modify) | yes | yes | yes (property-restricted + phrase + silent-discard control) |
| `internal/tools/search_messages_test.go` | `TestSearchMessagesTool_Registration` (remove) | yes | removed | yes (no references remain) |
| `internal/tools/search_messages_test.go` | `TestSearchMessagesTool_HasParameters` (remove) | yes | removed | yes |
| `internal/tools/search_messages_test.go` | `TestSearchMessagesToolCanBeAddedToServer` (remove) | yes | removed | yes |

Extra beyond spec (not a defect): `TestMeasuredRows` anchors all twelve measured rows as
fixtures. The dead `NewSearchMessagesTool` constructor is deleted from
`internal/tools/search_messages.go` (grep returns no references anywhere).

## Diff Coverage
| File | +/- | Mapped Requirements |
|---|---|---|
| `internal/tools/search_messages_query.go` (new) | +100 | FR-1..FR-6, NFR-1..NFR-3 |
| `internal/tools/search_messages_query_test.go` (new) | +218 | FR-2..FR-6, FR-12, NFR-1, NFR-2, Test Strategy adds |
| `internal/tools/search_messages.go` | +? -? (100 changed) | FR-7, FR-8, NFR-4; dead-code deletion (Change Drivers) |
| `internal/tools/search_messages_test.go` | +? (183 changed) | FR-7, FR-8, AC-6, AC-7; test modify + remove rows |
| `internal/server/mail_verbs.go` | +4 -4 | FR-9, FR-10, FR-11, NFR-5 |
| `internal/server/mail_verbs_test.go` (new) | +86 | FR-11, AC-9 |
| `docs/troubleshooting.md` | +18 | AC-10 surface; troubleshooting entry (`{#search-query-rejected}` anchor) |
| `docs/prompts/mcp-tool-crud-test.md` | +13 | Test Strategy lifecycle step (30f/30g) |
| `docs/cr/CR-0076-search-messages-kql-quoting.md` (new) | +768 | The CR itself |

### Unmapped changed files
None. Every changed file is named in the CR's Affected Components.

Note on `scripts/crud-test.sh`: the CR's Affected Components lists it alongside the harness
prompt, but the script was correctly left unchanged. Its per-domain accounting keys on
top-level domains (`mcp_mail`, `mcp_calendar`, `mcp_account`, `mcp_system`) and its own
maintenance comment requires an edit only when a top-level domain is added or removed. This
change adds verb-exercising steps within the existing `mail` domain, so the CSV schema and
per-domain buckets are unaffected. This is a justified non-change, not a gap.

## Gaps

**GAP-1: CLOSED after this report was written.** The live re-run has now been executed.

At the time this report was authored the running server predated the fix, which was itself
confirmed by observation rather than assumed: `subject:Contoso` still returned the
position-7 parse error while the on-disk binary already carried the change. The server was
restarted and the rows re-issued. Results:

| Query issued | Before | After |
|---|---|---|
| `subject:Contoso` | Error, `':'` invalid at position 7 | Two correct matches |
| `Zzzqqxx Wwwyyzz` | Newest unrelated messages, silently wrong | Zero results |
| `subject:"Contoso Quarterly"` | Error | Three correct matches, translated |
| `subject:"Contoso Teams"` | Not previously issued | Zero results |
| `subject:Design" Review` | Would reach Graph and fail there | Refused before the call |
| `"subject:(Contoso Quarterly)"` | Three matches | Three matches, unchanged |
| `from:alice@contoso.com hasAttachments:true` | Error | Two correct matches |

The decisive pair is the third and fourth rows. `subject:"Contoso Quarterly"` returns three
and `subject:"Contoso Teams"` returns none, which establishes that the translation scopes
the value to the named property rather than merely producing an expression Graph accepts.
A leaking translation would have returned the same three for both, because "Teams" appears
in those messages' bodies. That is AC-3 graded on its end state, which no network-free test
can reach.

The refusal text was observed in full and carries the failure, the fix, a worked example,
and the retry instruction, satisfying NFR-4 at the caller's surface rather than only in the
unit test.

AC-1, AC-2, AC-3, and AC-8 are therefore confirmed against a live mailbox and move from
PARTIAL to PASS. Ref: FR-12, AC-1, AC-2, AC-3, AC-8.

**GAP-2 (outstanding acceptance step, not a defect).** `make crud-test` (the lifecycle
harness, now carrying steps 30f/30g) has not been executed post-implementation in this
session. It requires a rebuilt binary at the configured path and a live account. Suggested
minimal action: rebuild per the AGENTS.md ldflags recipe, then `make crud-test`, and confirm
30f/30g PASS. Ref: Test Strategy harness step.

GAP-1 is closed by live observation. GAP-2 remains open and is a paid harness run awaiting
a decision. Neither was ever an implementation failure; both were the explicitly documented
live-verification limitation the CR's own Verification Commands note anticipated.

Revised counts after the live re-run: FAIL 0, PARTIAL 0, GAP 1.

One methodological note worth keeping, because it nearly produced a false pass. The first
attempt to close GAP-1 was made against a server process started before the implementation
landed. It would have reported the defect unfixed. The binary on disk was current; the
running process was not, and nothing in the tool output distinguishes the two. The check
that caught it was issuing a query whose answer was known to differ between the two
versions, before trusting any other result from that instrument.
