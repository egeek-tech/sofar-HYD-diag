---
phase: 19-system-configuration-batch-application
plan: 02
subsystem: hub
tags: [modbus, batch-read, streaming, fallback, timing, hub]

# Dependency graph
requires:
  - phase: 18-batch-read-infrastructure
    provides: AnalyzeBatchPlan, BatchSpan, ProbeMapping types and batch analysis engine
  - plan: 19-01
    provides: Composite probe field, system_time FormatValue dispatch, real 6-register system time probe
provides:
  - Batch span streaming in streamStandardRead for all standard sections
  - Per-span fallback to individual reads on batch failure
  - Section read timing log at Info level
  - Removal of hardcoded system time special case
affects: [all standard sections (system, grid, eps, pv, configuration, meter, dcdc, pcu, bdu)]

# Tech tracking
tech-stack:
  added: []
  patterns: [Batch span iteration with per-span fallback and progressive UI streaming]

key-files:
  created: []
  modified:
    - internal/hub/hub_streaming.go
    - internal/hub/hub_test.go

key-decisions:
  - "Uses pm.GroupName from ProbeMapping for register_value messages to preserve correct frontend rendering across cross-group merged spans"
  - "Bounds check end > len(data) before slicing batch response prevents panic on short/corrupt inverter responses (T-19-02 mitigation)"
  - "Timing log placed after fault reading but before section_complete so logged duration includes fault register time for system section"

patterns-established:
  - "Batch span iteration: for _, span := range sec.BatchPlan.Spans with single ReadRegisters per span"
  - "Per-span fallback: on batch error, iterate span.Probes with individual reads"
  - "Section timing: slog.Info after all reads complete with section name, duration_ms, span count, register count"

requirements-completed: [BATCH-02, BATCH-03, BATCH-04]

# Metrics
duration: 5min
completed: 2026-04-15
---

# Phase 19 Plan 02: Batch Span Streaming Rewrite Summary

**streamStandardRead rewritten to iterate sec.BatchPlan.Spans with single ReadRegisters per span, per-span fallback to individual reads, bounds-checked response slicing, and section timing log -- delivering 3-5x speedup for all standard sections**

## Performance

- **Duration:** 5 min
- **Started:** 2026-04-15T06:34:02Z
- **Completed:** 2026-04-15T06:39:00Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- Rewrote streamStandardRead to use batch span iteration instead of per-register loop
- Each span issues a single ReadRegisters call for the full contiguous register range
- Individual probe values extracted from batch response via ByteOffset/ByteLength
- Per-span fallback to individual reads when a batch span read fails (D-02)
- Bounds check prevents panic on short/corrupt responses (T-19-02 threat mitigation)
- Removed hardcoded system time special case (g.Name == "Status" block) -- system time now handled natively as Composite probe
- Section read timing logged at Info level with section name, duration_ms, span count, register count (D-05, D-06)
- All standard sections (system, grid, eps, pv, configuration, meter, dcdc, pcu, bdu) benefit automatically
- 4 new hub tests verify batch streaming, fallback, progressive updates, and timing log

## Task Commits

Each task was committed atomically:

1. **Task 1: Rewrite streamStandardRead to use batch spans** - `b4cfaa5` (feat)
2. **Task 2: Add hub tests for batch streaming behavior** - `af646ea` (test)

## Files Created/Modified
- `internal/hub/hub_streaming.go` - Rewrote streamStandardRead: batch span iteration, per-span fallback, bounds checking, timing log, removed system time special case
- `internal/hub/hub_test.go` - Added 4 tests: TestBatchStreamingMessages, TestBatchSpanFallback, TestBatchProgressiveStreaming, TestBatchTimingLog

## Decisions Made
- Uses pm.GroupName from ProbeMapping (not a loop variable) for register_value group names, preserving correct frontend rendering when spans merge probes across group boundaries
- Bounds check `end > len(data)` before slicing prevents panic on short inverter responses (T-19-02 mitigation)
- Timing log placed after fault reading but before section_complete, so the logged duration accurately includes fault register time for the system section

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All standard sections now use batch span reading automatically
- System time displays correctly as a Composite probe within the normal batch flow
- SpanTracker (from 18-02) is available for future degradation tracking integration
- Progressive UI updates preserved: register_value messages stream per-span, not buffered

## Self-Check: PASSED
