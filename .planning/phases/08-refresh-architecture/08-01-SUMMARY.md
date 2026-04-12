---
phase: 08-refresh-architecture
plan: 01
subsystem: hub
tags: [refactoring, websocket, context-cancellation, timer-removal]
dependency_graph:
  requires: []
  provides: [timer-free-hub, read-cycle-message, context-cancellable-reads, pack-info-skip]
  affects: [internal/hub/*, web/static/app.js]
tech_stack:
  added: []
  patterns: [context-cancellation, browser-driven-reads, atomic-bool-flags]
key_files:
  created: []
  modified:
    - internal/hub/section.go
    - internal/hub/hub.go
    - internal/hub/hub_streaming.go
    - internal/hub/message.go
    - internal/hub/export_test.go
    - internal/hub/hub_test.go
decisions:
  - Used atomic.Bool for skipPackInfo instead of channel-based mutation to avoid goroutine round-trip
  - Kept MsgTypeRefresh message type (manual refresh still useful alongside read_cycle)
  - Context cancellation on section switch cancels old section read before starting new one
metrics:
  duration: 581s
  completed: "2026-04-12T10:55:08Z"
  tasks: 2
  files_modified: 6
---

# Phase 08 Plan 01: Remove Backend Timer Infrastructure Summary

Timer-free hub architecture with browser-driven read cycles via context-cancellable streaming reads, PackInfoProbes session-level skip on illegal address error.

## Task Summary

| Task | Name | Commit | Status |
|------|------|--------|--------|
| 1 | Remove backend timer infrastructure and add context cancellation | 5d03cad | Done |
| 2 | Update hub tests for timer-free architecture | 4988a24 | Done |

## What Changed

### Task 1: Remove backend timer infrastructure and add context cancellation

**section.go:** Removed all timer fields (autoRefresh, ticker, stopCh, timerCh, interval) and methods (startTimer, stopTimer, pauseTimer, resumeTimer, SetInterval, defaultRefreshInterval). Added `readCancel context.CancelFunc` field and `cancelRead()` method for per-section read cancellation. Constructor signatures simplified (no timerCh parameter).

**message.go:** Removed `MsgTypeAutoRefresh` constant. Added `MsgTypeReadCycle = "read_cycle"` constant.

**hub.go:** Removed `timerCh chan string`, `refreshOverride time.Duration`, `SetRefreshOverride()`, `handleTimerTick()`, and `handleAutoRefreshToggle()`. Added `skipPackInfo atomic.Bool` for session-level PackInfoProbes skip. Added `handleReadCycle()` handler with connected/subscriber/reading guards. Updated `triggerSectionRead()` to cancel previous read and create per-section cancellable context via `context.WithCancel`. Updated `handleStateEvent()` to cancel all reads on disconnect instead of pausing timers. Updated `subscribeClient()` to cancel old section read on section switch. Updated `unsubscribeClient()` to cancel read when last subscriber leaves. Added PackInfoProbes skip logic in `triggerPackRead()` -- on illegal address error (0x02), sets `skipPackInfo` atomically and skips for remainder of session.

**hub_streaming.go:** All three streaming functions (`streamStandardRead`, `streamBatteryRead`, `streamBMSRead`) now accept `readCtx context.Context` parameter. All `h.ctx.Err()` checks replaced with `readCtx.Err()`. All broker read calls use `readCtx` instead of `h.ctx`, enabling per-section cancellation.

### Task 2: Update hub tests for timer-free architecture

Replaced 5 timer-based tests with 6 architecture tests:
- `TestReadCycleMessage` -- verifies read_cycle triggers a section read
- `TestSkipOverlappingReadCycle` -- verifies sec.reading guard skips overlapping read_cycles
- `TestReadsCancelledOnDisconnect` -- verifies reads stop and read_cycle rejected after disconnect
- `TestReadsWorkAfterReconnect` -- verifies read_cycle works after reconnect
- `TestNoBackendTimer` -- verifies no autonomous reads occur (2s wait, zero section_complete)
- `TestCancelReadOnSectionSwitch` -- verifies grid read cancelled when switching to system

Removed `NewTestHubWithInterval` from export_test.go. Added `SendReadCycle` test helper. Updated `setupConnectedHub` to remove interval dependency.

## Decisions Made

1. **atomic.Bool for skipPackInfo**: Used `sync/atomic.Bool` instead of routing through `h.funcs` channel since `skipPackInfo` is a simple boolean flag that only transitions false->true. Avoids goroutine round-trip overhead.
2. **Kept MsgTypeRefresh**: The existing manual refresh message type remains functional alongside the new read_cycle type. Manual refresh does not check `sec.reading` guard (allows force-refresh).
3. **readCtx vs h.ctx**: Per-section cancellable contexts prevent orphaned goroutines when switching sections or disconnecting, while h.ctx still controls overall hub lifecycle.

## Deviations from Plan

None -- plan executed exactly as written.

## Verification Results

1. `go build ./...` -- compiles without errors
2. `go test ./internal/hub/ -count=1 -timeout 120s` -- all 33 hub tests pass (4 skipped stubs)
3. `go test ./... -timeout 120s` -- full test suite passes (all packages)
4. Grep verification: section.go has no Ticker, hub.go has no timerCh, hub_streaming.go uses readCtx not h.ctx

## Requirements Addressed

- **REFR-01**: No autonomous backend refresh -- verified by TestNoBackendTimer (2s wait, zero reads)
- **REL-03**: Inter-read delay naturally enforced by broker.enforceInterReadDelay; context cancellation prevents burst on section switch

## Known Stubs

None. All functionality is fully wired.

## Self-Check: PASSED

All 6 modified files exist. Both task commits (5d03cad, 4988a24) verified in git log. SUMMARY.md created.
