---
phase: 22-spantracker-integration
plan: 01
subsystem: hub/batch
tags: [spantracker, degradation, three-state, tdd]
dependency_graph:
  requires: []
  provides: [SpanState-enum, three-state-SpanTracker, probe-scheduling]
  affects: [internal/hub/batch.go, internal/hub/batch_test.go]
tech_stack:
  added: []
  patterns: [three-state-machine, cycle-based-probe-scheduling, map-clear-builtin]
key_files:
  created: []
  modified:
    - internal/hub/batch.go
    - internal/hub/batch_test.go
decisions:
  - "IsDegraded returns true for both SpanDegraded and SpanSkipped (backward compatible with existing callers)"
  - "RecordSuccess resets any state to SpanNormal (D-08: full reset, not back one level)"
  - "Additional batch failures beyond threshold stay at SpanDegraded (do not advance to SpanSkipped)"
metrics:
  duration: 165s
  completed: "2026-04-16T10:10:24Z"
  tasks: 1
  files: 2
---

# Phase 22 Plan 01: Three-State SpanTracker Extension Summary

Extended two-state SpanTracker (Normal/Degraded) to three-state model (Normal/Degraded/Skipped) with cycle-based probe recovery scheduling via TDD.

## Task Results

| Task | Name | Commit(s) | Files | Status |
|------|------|-----------|-------|--------|
| 1 | Extend SpanTracker to three-state model with TDD | 7d225e1 (RED), 07ee79b (GREEN) | internal/hub/batch.go, internal/hub/batch_test.go | Done |

## What Was Done

### Task 1: Three-State SpanTracker (TDD)

**RED phase (7d225e1):** Added 9 new failing tests covering all three-state transitions, probe scheduling, reset, and IsSkipped. Updated 3 existing tests to also verify State() method.

**GREEN phase (07ee79b):** Replaced the two-state SpanTracker implementation with the three-state model:

- Added `SpanState` enum (`SpanNormal`, `SpanDegraded`, `SpanSkipped`) with `String()` method
- Added `DefaultProbeInterval = 10` constant
- Replaced `degraded map[uint16]bool` with `states map[uint16]SpanState`
- Added `indivFails map[uint16]int`, `cycle int`, `probeInterval int` fields
- Implemented 6 new methods: `State()`, `IsSkipped()`, `RecordIndividualFailure()`, `Tick()`, `ShouldProbe()`, `Reset()`
- `RecordSuccess` resets any state to `SpanNormal` with full counter reset (D-08)
- `RecordFailure` transitions `Normal->Degraded` at threshold (backward compatible)
- `RecordIndividualFailure` transitions `Degraded->Skipped` after 2 failures
- `IsDegraded` returns true for both `Degraded` and `Skipped` (backward compatible)

**REFACTOR phase:** No changes needed -- implementation is clean and well-structured.

## Verification

```
go test ./internal/hub/ -run TestSpanTracker -count=1 -v        -> 17/17 PASS
go test ./internal/hub/ -run TestSpanTracker -count=1 -race      -> PASS (clean)
go build ./...                                                    -> PASS (no compile errors)
```

All 8 existing tests pass unchanged (backward compatibility confirmed). 9 new tests cover three-state transitions, probe scheduling, and reset.

## Deviations from Plan

None - plan executed exactly as written.

## TDD Gate Compliance

- RED gate: `test(22-01)` commit 7d225e1 -- tests fail to compile (methods undefined)
- GREEN gate: `feat(22-01)` commit 07ee79b -- all 17 tests pass
- REFACTOR gate: Skipped (no cleanup needed)

## Self-Check: PASSED

- [x] internal/hub/batch.go exists and contains SpanState enum, all new methods
- [x] internal/hub/batch_test.go exists and contains 17 test functions
- [x] Commit 7d225e1 exists (RED)
- [x] Commit 07ee79b exists (GREEN)
- [x] All tests pass with race detector
