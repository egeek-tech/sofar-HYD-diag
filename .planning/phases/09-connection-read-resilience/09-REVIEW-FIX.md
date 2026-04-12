---
phase: 09-connection-read-resilience
fixed_at: 2026-04-12T00:00:00Z
review_path: .planning/phases/09-connection-read-resilience/09-REVIEW.md
iteration: 1
findings_in_scope: 4
fixed: 4
skipped: 0
status: all_fixed
---

# Phase 09: Code Review Fix Report

**Fixed at:** 2026-04-12
**Source review:** .planning/phases/09-connection-read-resilience/09-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 4
- Fixed: 4
- Skipped: 0

## Fixed Issues

### CR-01: Panic -- send on closed channel in triggerPackRead goroutine

**Files modified:** `internal/hub/client.go`, `internal/hub/hub.go`
**Commit:** 8e6ce29
**Applied fix:** Added `closed atomic.Bool` field to `Client` struct. Set it to `true` in `removeClient()` and `shutdown()` before calling `close(c.send)`. Added early-return guard in `sendPackError()` and `sendPackDataToClient()` that checks `client.closed.Load()` before attempting to send on the channel. This prevents the panic when a pack-read goroutine outlives its client.

### WR-01: Reconfigure() always returns nil -- connection errors are silently dropped

**Files modified:** `internal/broker/broker.go`
**Commit:** 71f1a36
**Applied fix:** Changed `executeReconfigure` return type from `void` to `error`, returning the dial error on failure and `nil` on success. Updated the `CmdReconfigure` case in `execute()` to send `Result{Err: dialErr}` instead of empty `Result{}`. Updated `Reconfigure()` to extract `Result.Err` via type assertion instead of checking if the response is an `error` interface (which was always false for `Result{}`). Hub's error logging path in `handleConnect` is now reachable.

### WR-02: Streaming goroutines send blocking on h.results without context guard -- cancelled reads deliver stale data

**Files modified:** `internal/hub/hub_streaming.go`
**Commit:** 814ffd0
**Applied fix:** Added `readCtx.Err()` checks before every `h.results <-` send in all three streaming functions (`streamStandardRead`, `streamBatteryRead`, `streamBMSRead`). This covers: individual register value sends, composed system time sends, composed BMS clock/SW version sends, topology sends, fault data sends, and the post-processing bitmap/protection batch. When the context is cancelled mid-read, goroutines now return immediately instead of delivering stale results.

### WR-03: Blocking send on h.funcs in streamBatteryRead goroutine -- goroutine leak on hub shutdown

**Files modified:** `internal/hub/hub_streaming.go`
**Commit:** 1ce03b8
**Applied fix:** Replaced the plain blocking `h.funcs <- func(){...}` with a `select` statement that also checks `<-readCtx.Done()`. On context cancellation, `retrigger` is set back to `false` so the deferred cleanup correctly clears `sec.reading`. This prevents the goroutine from blocking forever if the hub shuts down while the `h.funcs` channel is full.

---

_Fixed: 2026-04-12_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
