---
phase: 22-spantracker-integration
plan: 02
subsystem: hub
tags: [resilience, degradation, batch-reading, spantracker]
dependency_graph:
  requires:
    - 22-01 (SpanTracker three-state extension)
  provides:
    - SpanTracker wired into streamStandardRead span loop
    - Section.SpanTracker field on grouped sections
    - SpanTracker reset on reconnect
    - Thread-safe GetSpanState test accessor
  affects:
    - internal/hub/section.go
    - internal/hub/hub.go
    - internal/hub/hub_streaming.go
    - internal/hub/export_test.go
    - internal/hub/hub_test.go
tech_stack:
  added: []
  patterns:
    - Three-state span degradation in streaming read loop
    - spanFailAddrs mock for count-aware batch vs individual failure injection
    - GetSpanState thread-safe accessor via RunFunc for race-free test assertions
key_files:
  created: []
  modified:
    - internal/hub/section.go
    - internal/hub/hub.go
    - internal/hub/hub_streaming.go
    - internal/hub/export_test.go
    - internal/hub/hub_test.go
decisions:
  - SpanTracker initialized only for grouped sections (newGroupedSection), not newSection
  - Nil check on SpanTracker in all call sites for safety with non-grouped sections
  - GetSpanState accessor routes state reads through hub event loop to avoid data races
  - spanFailAddrs mock field distinguishes batch (count>1) from individual (count=1) reads
metrics:
  duration: 13m
  completed: 2026-04-16
  tasks_completed: 2
  tasks_total: 2
  files_modified: 5
---

# Phase 22 Plan 02: SpanTracker Integration Wiring Summary

Three-state SpanTracker wired into streamStandardRead with degradation lifecycle: Normal (batch) -> Degraded (individual-only) -> Skipped (no reads), periodic probe recovery every 10 cycles, and reconnect reset.

## Changes Made

### Task 1: Add SpanTracker to Section and wire reconnect reset (e8369cf)

**section.go:**
- Added `SpanTracker *SpanTracker` field to Section struct after BatchPlan
- Initialized SpanTracker in `newGroupedSection` with `DefaultDegradationThreshold` (3) and section-scoped logger
- Not added to `newSection` (non-batch sections like battery/bms)

**hub.go:**
- Added SpanTracker reset loop in `handleStateEvent` `case broker.StateConnected` with nil guard
- All sections' SpanTrackers are reset on reconnect (D-02)

**export_test.go:**
- Added `GetSectionSpanTracker` accessor following existing pattern
- Added `GetSpanState` thread-safe accessor that reads SpanTracker state on the hub event loop to avoid data races in test assertions

### Task 2: Wire SpanTracker into streamStandardRead with integration tests (eaa59ec)

**hub_streaming.go:**
- Added `sec.SpanTracker.Tick()` call before span loop to advance cycle counter
- Three-state span handling in span loop:
  - SpanSkipped + not probe: skip entirely (no reads)
  - SpanDegraded + not probe: individual reads only, RecordIndividualFailure if all fail, RecordSuccess if any succeed
  - SpanNormal or probe: batch read attempt with full fallback logic
- Batch success calls `RecordSuccess` to recover from any degraded state
- Batch failure calls `RecordFailure` for normal spans, no-op for failed probes on degraded/skipped

**hub_test.go:**
- Added `spanFailAddrs` field to mockBroker for count-aware failure injection (batch reads count>1 fail, individual reads count=1 succeed)
- Added `setRegisterResult`, `resetBatchCallCount`, `setSpanFail`, `clearSpanFail` mock helpers
- Added `setupGridSpanTest` helper for grid span test setup
- Added `triggerGridReadCycle` helper for read cycle triggering
- 4 integration tests:
  1. `TestStreamStandardRead_SpanTrackerDegradation`: 3 batch failures degrade span, individual fallback works
  2. `TestStreamStandardRead_SpanTrackerSkipped`: Degraded + 2 individual all-fail cycles -> Skipped, no reads on skipped span
  3. `TestStreamStandardRead_SpanTrackerProbeRecovery`: Skipped span recovers on 10th cycle probe success
  4. `TestStreamStandardRead_SpanTrackerResetOnReconnect`: Degraded span resets to Normal on reconnect

## Verification Results

- `go build ./...` exits 0
- 74 tests pass without race detector (all existing + 4 new)
- 4 SpanTracker integration tests pass with race detector enabled
- All 17 SpanTracker unit tests (from Plan 01) continue to pass
- TestBatchSpanFallback and TestBatchProgressiveStreaming continue to pass
- Pre-existing race in TestHubRegisterUnregister/TestGroupedSectionRegistered/TestStatusSectionRemoved unchanged (Hub.ClientCount/RunFunc vs Hub.Run race, not caused by this plan)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Mock broker registerResults key collision**
- **Found during:** Task 2 test development
- **Issue:** `registerResults[addr]` keyed by address only; span batch read (addr=0x0484, count=8) and first individual probe (addr=0x0484, count=1) shared the same key, causing the individual probe setup to overwrite the span failure
- **Fix:** Added `spanFailAddrs map[uint16]error` to mock broker that only triggers for count>1 reads, checked before registerResults in ReadRegisters
- **Files modified:** internal/hub/hub_test.go
- **Commit:** eaa59ec

**2. [Rule 1 - Bug] Data race in SpanTracker state access from test goroutine**
- **Found during:** Task 2 race detector run
- **Issue:** Test assertions called `tracker.State(failAddr)` directly from test goroutine while hub goroutine could be calling `SpanTracker.Reset()` concurrently (SpanTracker is intentionally not goroutine-safe)
- **Fix:** Added `GetSpanState(section, startAddr)` accessor in export_test.go that reads state through `RunFunc` on the hub event loop; replaced all direct `tracker.State()` calls in tests
- **Files modified:** internal/hub/export_test.go, internal/hub/hub_test.go
- **Commit:** eaa59ec

## Self-Check: PASSED

All 5 modified files exist. Both task commits (e8369cf, eaa59ec) verified in git log.
