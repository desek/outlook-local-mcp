# CR-0035 Validation Report

**Validator**: Validation Agent
**Date**: 2026-03-16
**Branch**: dev/cc-swarm
**Source commit**: 576e35c (Phase 3 checkpoint)

## Summary

Requirements: 7/7 | Acceptance Criteria: 7/7 | Tests: 11/11 | Gaps: 0

**Build**: PASS (0 errors)
**Lint**: PASS (0 issues)
**Tests**: PASS (all packages)

## Requirement Verification

| Req # | Description | Status | Evidence |
|---|---|---|---|
| FR-1 | Auth goroutine MUST continue polling when elicitation fails | PASS | `add_account.go:591-594`: when `presentDeviceCodeElicitation` returns error, `storePending` is called instead of `cancel()`. The goroutine started at line 581-584 continues running on `authCtx` (context.Background with 300s timeout). No cancel is invoked on the pending path. |
| FR-2 | Credential/authenticator MUST be stored in pending map keyed by label | PASS | `add_account.go:570-580`: `pendingAccount` struct is created with `cred`, `authenticator`, `authRecordPath`, `cacheName`, `clientID`, `tenantID`, `authMethod`. `storePending` at line 593 stores it in `s.pending[label]` (line 715). |
| FR-3 | Second add_account MUST check pending state first (success/error/running) | PASS | `add_account.go:211-241`: `checkPending` is called before `registry.Get`. Three branches: success returns `p` with nil result (line 692), error returns nil with error result (line 688-690), in-progress returns nil with text result (line 696-699). |
| FR-4 | cancel() at ~line 485 MUST be removed for pending path | PASS | `add_account.go:586-610`: in the `case msg := <-deviceCodeCh` branch, when `elicitErr != nil` (line 590), only `storePending` is called (line 593). `cancel()` is only called on the elicitation-succeeded path (line 598), the auth-completed-before-prompt path (line 603), and the timeout path (line 607). No cancel on the pending path. |
| FR-5 | defer cancel() MUST be conditional (not cancel when stored as pending) | PASS | `add_account.go:561`: `cancel` is no longer deferred. Instead, `cancel` is stored in the `pendingAccount` struct (line 578) and called explicitly on non-pending paths (lines 598, 603, 607) or during cleanup in `checkPending` (line 684). |
| FR-6 | Pending accounts MUST have bounded lifetime via 300-second timeout | PASS | `add_account.go:561`: `context.WithTimeout(context.Background(), 300*time.Second)` creates the auth context. The goroutine at line 581-584 runs on this context. When the 300s timeout fires, `ctx.Done()` closes and `authenticate` returns the timeout error to `p.err`. |
| FR-7 | Auth record MUST be persisted when goroutine completes successfully | PASS | `add_account.go:583`: `s.authenticate(authCtx, authenticator, authRecordPath)` is called inside the goroutine. The `authenticate` field defaults to `auth.Authenticate` (line 143), which persists the auth record as part of its normal flow. No change needed -- persistence happens inside the existing `auth.Authenticate` function. |

## Acceptance Criteria Verification

| AC # | Description | Status | Evidence |
|---|---|---|---|
| AC-1 | Second add_account call completes registration | PASS | `add_account.go:211-241`: when `checkPending` returns `(p, nil)`, the handler creates a Graph client (line 215), calls `registry.Add` (line 225), persists config (line 229), and returns success JSON with `"added": true` (line 234). Test: `TestDeviceCode_PendingAuth_CompletedSuccessfully` (line 983) confirms this end-to-end. |
| AC-2 | Auth goroutine survives between calls | PASS | `add_account.go:593`: `storePending` stores the pending account without cancelling context. The goroutine (line 581-584) keeps running. Test: `TestDeviceCode_PendingAuth_GoroutineNotCancelled` (line 1246) verifies context is NOT cancelled after handler returns. |
| AC-3 | In-progress message for premature second call | PASS | `add_account.go:694-699`: `checkPending` default case returns "still in progress" text result. Test: `TestDeviceCode_PendingAuth_StillInProgress` (line 1094) confirms the message. |
| AC-4 | Failed pending auth allows retry | PASS | `add_account.go:682-690`: `checkPending` deletes the failed entry and returns error with failure reason and "try add_account again" message. Test: `TestDeviceCode_PendingAuth_Failed` (line 1153) and `TestDeviceCode_PendingAuth_ThirdCallAfterFailure` (line 1293) confirm cleanup and fresh retry. |
| AC-5 | No regression for elicitation-supporting clients | PASS | `add_account.go:595-599`: when elicitation succeeds (`elicitErr == nil`), the handler waits for auth inline (`<-p.done`) then cancels and returns. No `storePending` is called. Test: `TestDeviceCode_ElicitationSupported_NoPendingState` (line 1406) confirms no pending entries and inline registration. |
| AC-6 | No regression for browser and auth_code methods | PASS | `add_account.go:364-369`: `authenticateInline` routes browser to `authenticateBrowser` and auth_code to `authenticateAuthCode`, neither of which uses the pending mechanism. Tests: `TestBrowserAuth_NoPendingState` (line 1475) and `TestAuthCodeAuth_NoPendingState` (line 1535) confirm no pending entries for these methods. |
| AC-7 | Bounded goroutine lifetime | PASS | `add_account.go:561`: 300-second `context.WithTimeout`. When timeout fires, goroutine writes timeout error to `p.err` and closes `p.done`. Next `checkPending` call cleans up the entry and calls `p.cancel()` (line 684). Test: `TestDeviceCode_PendingAuth_Timeout` (line 1577) uses a 50ms timeout to simulate this, confirms error result and cleanup. |

## Test Strategy Verification

### New Tests (9)

| Test File | Test Name | Specified | Exists | Matches Spec |
|---|---|---|---|---|
| add_account_test.go | `TestDeviceCode_PendingAuth_CompletedSuccessfully` | Yes | Yes (line 983) | Yes -- 1st call stores pending, signals auth success, 2nd call returns `"added": true`, account in registry, pending map empty |
| add_account_test.go | `TestDeviceCode_PendingAuth_StillInProgress` | Yes | Yes (line 1094) | Yes -- 1st call stores pending, 2nd call returns "still in progress" text (non-error) |
| add_account_test.go | `TestDeviceCode_PendingAuth_Failed` | Yes | Yes (line 1153) | Yes -- 1st call stores pending, auth fails, 2nd call returns error with "user denied" and retry instructions, pending map empty |
| add_account_test.go | `TestDeviceCode_PendingAuth_GoroutineNotCancelled` | Yes | Yes (line 1246) | Yes -- uses `atomic.Bool` to verify context is NOT cancelled after elicitation failure |
| add_account_test.go | `TestDeviceCode_PendingAuth_ThirdCallAfterFailure` | Yes | Yes (line 1293) | Yes -- 1st call pending, 2nd call picks up failure, 3rd call starts fresh (new device code CODE2), authenticate called at least twice |
| add_account_test.go | `TestDeviceCode_ElicitationSupported_NoPendingState` | Yes | Yes (line 1406) | Yes -- elicitation succeeds, account registered inline, pending map empty |
| add_account_test.go | `TestBrowserAuth_NoPendingState` | Yes | Yes (line 1475) | Yes -- browser auth succeeds, account registered, pending map empty |
| add_account_test.go | `TestAuthCodeAuth_NoPendingState` | Yes | Yes (line 1535) | Yes -- auth_code via `authenticateInline`, pending map empty |
| add_account_test.go | `TestDeviceCode_PendingAuth_Timeout` | Yes | Yes (line 1577) | Yes -- short context timeout, goroutine exits, next call returns timeout error, pending entry cleaned up |

### Modified Tests (2)

| Test File | Test Name | Specified | Exists | Matches Spec |
|---|---|---|---|---|
| add_account_test.go | `TestAuthenticateDeviceCode_ElicitationError_ReturnsDeviceCode` | Yes | Yes (line 687) | Yes -- verifies `DeviceCodeFallbackError` returned AND pending state stored (lines 744-756: checks `state.pending["dc-fallback"]` exists, done channel still open) |
| add_account_test.go | `TestAuthenticateDeviceCode_ElicitationError_DoesNotBlock` | Yes | Yes (line 762) | Yes -- verifies prompt return (< 5s) AND pending state exists (lines 805-810: checks `state.pending["block-test"]` exists) |

## Gaps

None.
