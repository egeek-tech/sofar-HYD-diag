---
phase: 25-pack-drill-down-batch-migration
plan: 01
subsystem: hub/streaming, frontend
tags: [batch-read, pack-drill-down, span-tracker, frontend-fix]
dependency_graph:
  requires: [Phase 23 readBatchSpans, Phase 24 BMS batch]
  provides: [streamPackBatchRead, packSpanTracker, D-07 fix]
  affects: [internal/hub/hub.go, internal/hub/hub_streaming.go, internal/hub/export_test.go, internal/hub/hub_test.go, web/static/app.js]
tech_stack:
  added: []
  patterns: [temp Section for readBatchSpans reuse, packSpanTracker on Hub struct]
key_files:
  created: []
  modified:
    - internal/hub/hub.go
    - internal/hub/hub_streaming.go
    - internal/hub/export_test.go
    - internal/hub/hub_test.go
    - web/static/app.js
decisions:
  - "Use temp Section with packSpanTracker for readBatchSpans reuse (avoids adding pack logic to Section struct)"
  - "select_pack resets packSpanTracker rather than clearing a skip map (degradation-based approach)"
metrics:
  duration: 33m
  completed: "2026-04-17T20:21:02Z"
  tasks_completed: 2
  tasks_total: 2
  files_modified: 5
---

# Phase 25 Plan 01: Pack Drill-Down Batch Migration Summary

Pack drill-down migrated from per-register individual reads to BatchPlan span reads via the shared readBatchSpans helper, with a dedicated packSpanTracker for degradation tracking and frontend D-07 fix for BMS widget duplication.

## What Changed

### Task 1: Create streamPackBatchRead and update Hub struct + routing (246ef83)

**Hub struct changes (hub.go):**
- Replaced `packSkipRegisters map[uint16]bool` with `packSpanTracker *SpanTracker` on Hub struct
- NewHub initializes packSpanTracker with DefaultDegradationThreshold
- handleSelectPack calls `packSpanTracker.Reset()` on pack switch (replaces map recreation)
- handleStateEvent reconnect handler resets packSpanTracker alongside section SpanTrackers
- Both handleSelectPack and handleReadCycle route to `streamPackBatchRead` (replaces `streamPackRead`)

**hub_streaming.go changes:**
- Added `streamPackBatchRead` following the streamBMSBatchRead hybrid pattern:
  - Preserves 0x9020 write + settle delay (hardware requirement)
  - Builds a temporary Section with BatchPlan from PackProbeGroups
  - Captures packSpanTracker reference before goroutine launch (race safety)
  - Delegates register reading to `readBatchSpans` for span-based batch reads
- Removed dead `streamPackRead` function (107 lines of per-register code)

**Batch plan structure (6 spans from PackProbeGroups):**
- Span 0: 0x9044 (1 reg) - Pack ID
- Span 1: 0x9047 (26 regs) - Serial Number + 16 cell voltages
- Span 2: 0x9069 (20 regs) - Max/Min cell, temps, current, capacity, status
- Span 3: 0x90BC (4 regs) - Temp 5-8
- Span 4: 0x9104 (8 regs) - Info block (known to fail on some BMS hardware)
- Span 5: 0x9124 (3 regs) - Status 2 registers

### Task 2: Update tests, export helpers, and frontend D-07 fix (7748983)

**export_test.go:**
- Removed `GetPackSkipRegisters()` (references deleted field)
- Added `GetPackSpanTracker()` and `GetPackSpanState(startAddr)` following GetSectionSpanTracker pattern

**hub_test.go:**
- Rewrote `TestPackSkipUnsupported` -> `TestPackSpanDegradation`: verifies span degradation after 3 batch failures using spanFailAddrs mock, confirms individual fallback still produces register_value messages
- Rewrote `TestPackSkipResetOnSwitch` -> `TestPackSpanResetOnSwitch`: verifies packSpanTracker.Reset() on pack switch returns degraded spans to SpanNormal
- Rewrote `TestStreamPackReadGroupBatch` -> `TestStreamPackBatchReadAllProbes`: verifies all 50 pack probes produce register_value messages via batch spans, all 5 groups represented
- Updated `TestHandleSelectPack` assertion from 10+ individual reads to 6+ batch span reads

**app.js (D-07 fix):**
- Added `data-computed-group` attribute in `renderBitmapGroup` (Battery Topology widget)
- Added `data-computed-group` attribute in `renderProtectionGroup` (Protection & Alarms widget)
- These attributes enable handleSectionData to find and replace widgets on refresh without duplication

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] TestHandleSelectPack assertion updated**
- **Found during:** Task 2
- **Issue:** TestHandleSelectPack expected 10+ individual ReadRegisters calls but batch reads produce only 6 calls
- **Fix:** Updated assertion threshold from 10 to 6 (one per batch span)
- **Files modified:** internal/hub/hub_test.go
- **Commit:** 7748983

**2. [Rule 1 - Bug] Pack span test race with reading flag**
- **Found during:** Task 2 verification
- **Issue:** read_cycle commands were silently skipped when sec.reading was still true from previous goroutine
- **Fix:** Added 100ms sleep after collectRawMessages to allow goroutine defer to clear reading flag
- **Files modified:** internal/hub/hub_test.go
- **Commit:** 7748983

**3. [Rule 1 - Bug] Mock registerResults overwrite for 0x9104 span**
- **Found during:** Task 2 verification
- **Issue:** Individual probe setup for 0x9104 (2 bytes) overwrote batch span data (16 bytes), causing batch reads to return truncated data
- **Fix:** Restored full span data in registerResults after removing spanFailAddrs entry
- **Files modified:** internal/hub/hub_test.go
- **Commit:** 7748983

## Verification Results

1. `go build ./...` -- exits 0
2. `go test ./internal/hub/ -run "TestPack" -count=1 -timeout 60s` -- all pack tests pass
3. `go test ./... -count=1 -timeout 300s` -- full test suite passes (5 packages, 0 failures)
4. `grep -r "packSkipRegisters" internal/hub/` -- zero matches (dead code removed)
5. `grep -r "func.*streamPackRead[^B]" internal/hub/` -- zero matches (old function removed)
6. `grep "data-computed-group" web/static/app.js` -- matches in renderBitmapGroup and renderProtectionGroup

## Self-Check: PASSED

All 5 modified files exist. Both task commits (246ef83, 7748983) verified in git log.
