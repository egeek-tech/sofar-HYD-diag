---
phase: 09-connection-read-resilience
plan: 02
subsystem: broker
tags: [reliability, retry, modbus, error-handling]
dependency_graph:
  requires: [09-01]
  provides: [per-register-retry, error-classification]
  affects: [internal/broker]
tech_stack:
  added: []
  patterns: [isRetryable-error-classification, 3-attempt-retry-loop]
key_files:
  created: []
  modified:
    - internal/broker/broker.go
    - internal/broker/broker_test.go
decisions:
  - "maxAttempts=3 (D-05): increased from 2 to recover from more transient errors"
  - "isRetryable skips retry for Modbus exception 0x02 illegal address (D-06): no point retrying a register that does not exist on hardware"
  - "Non-retryable errors skip handleError to keep connection open (D-07): avoids pointless reconnect cycle"
metrics:
  duration: "2m 53s"
  completed: "2026-04-12T15:31:46Z"
  tasks_completed: 2
  tasks_total: 2
---

# Phase 09 Plan 02: Per-Register Retry with Error Classification Summary

Transparent per-register error recovery with 3 retry attempts and intelligent Modbus exception classification that skips retry for illegal address (0x02) errors.

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 | 250c537 | feat(09-02): add isRetryable helper and increase executeRead to 3 attempts |
| 2 | ba19b9e | test(09-02): add comprehensive retry behavior tests |

## Changes Made

### Task 1: isRetryable helper and executeRead update

**internal/broker/broker.go:**
- Added `"strings"` import
- Changed `const maxAttempts = 2` to `const maxAttempts = 3` in executeRead (D-05)
- Added `isRetryable(err error) bool` package-level function that returns false for errors containing `"err=0x02"` (Modbus illegal data address exception)
- Added early return in executeRead: `if !isRetryable(err) { return Result{Err: err} }` placed BEFORE `handleError(err)` -- this ensures the connection is not closed for non-retryable errors
- Added `"maxAttempts", maxAttempts` to retry debug log for clarity

### Task 2: Comprehensive retry behavior tests

**internal/broker/broker_test.go:**
- `buildExceptionResponse` helper: constructs Modbus TCP exception response with configurable exception code
- `TestBrokerRetryThreeAttempts`: mock server that closes connections, verifies broker makes 3 connection attempts before returning error
- `TestBrokerNoRetryIllegalAddress`: mock server returns exception 0x02, verifies only 1 request made (no retry) and broker stays in StateConnected (handleError not called)
- `TestBrokerRetrySuccess`: mock server fails first read then succeeds on retry, verifies transparent recovery returning valid data (0xBEEF)

## Deviations from Plan

None -- plan executed exactly as written.

## Verification

1. `go build ./...` -- compiles without errors
2. `go test ./internal/broker/ -run "TestBrokerRetry|TestBrokerNoRetry" -count=1 -v -timeout=30s` -- all 3 tests pass
3. `go test ./... -count=1 -timeout=120s` -- full test suite green (broker, hub, modbus, register, web)
4. broker.go contains `const maxAttempts = 3` at line 388
5. broker.go contains `func isRetryable(err error) bool` at line 613
6. broker.go executeRead contains `if !isRetryable(err)` at line 414, before `b.handleError(err)` at line 418

## Self-Check: PASSED

All files exist, all commits verified.
