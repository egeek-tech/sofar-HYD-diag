---
phase: 27-hub-test-optimization
plan: 01
subsystem: testing
tags: [broker, modbus, inter-read-delay, timing, regression-test]

# Dependency graph
requires: []
provides:
  - "Fixed enforceInterReadDelay that handles zero/stale lastReadTime (D-07)"
  - "TestInterReadDelayBurst regression test for burst prevention"
affects: [27-02, 27-03]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "IsZero guard pattern for time.Time zero-value edge cases"

key-files:
  created: []
  modified:
    - "internal/broker/broker.go"
    - "internal/broker/broker_test.go"

key-decisions:
  - "IsZero guard returns early and stamps lastReadTime for burst prevention, rather than enforcing full delay on first read"

patterns-established:
  - "Zero-value time guard: check IsZero() before time.Since() to handle uninitialized timestamps"

requirements-completed: [TEST-03]

# Metrics
duration: 2min
completed: 2026-04-19
---

# Phase 27 Plan 01: Fix Inter-Read Delay Burst Bug Summary

**Fixed enforceInterReadDelay zero-value edge case (D-07) with IsZero guard and added TestInterReadDelayBurst regression test proving delay enforcement after idle periods**

## Performance

- **Duration:** 2 min
- **Started:** 2026-04-19T08:07:13Z
- **Completed:** 2026-04-19T08:09:48Z
- **Tasks:** 1
- **Files modified:** 2

## Accomplishments
- Fixed the D-07 production bug: enforceInterReadDelay now handles zero-value lastReadTime by stamping the current time, ensuring subsequent reads within the same batch are properly throttled
- Added TestInterReadDelayBurst regression test that simulates section switch (2s idle gap) and verifies inter-read delay enforcement across two batches of 3 reads each
- All 15 existing broker tests continue to pass; go vet clean

## Task Commits

Each task was committed atomically:

1. **Task 1: Fix enforceInterReadDelay burst bug and add regression test** - `6b453e5` (fix)

## Files Created/Modified
- `internal/broker/broker.go` - Added lastReadTime.IsZero() guard in enforceInterReadDelay (lines 532-552)
- `internal/broker/broker_test.go` - Added TestInterReadDelayBurst function (lines 657-705)

## Decisions Made
- The IsZero guard stamps lastReadTime and returns without delay on first read. This is correct because the first read has no prior read to enforce spacing from. The key fix is that lastReadTime is now set during enforceInterReadDelay (not just after read completion), so the second read in a batch immediately sees a recent timestamp and enforces the 500ms delay.
- After investigation, confirmed the burst scenario only affects the zero-value case (very first read ever). The stale-lastReadTime case (section switch after idle) correctly skips the delay for the first read (the idle time already satisfies the gap) and properly enforces delay on subsequent reads since lastReadTime is updated after each read completes.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Broker inter-read delay is now robust against zero-value edge case
- TestInterReadDelayBurst provides regression coverage for future changes to enforceInterReadDelay
- Ready for plans 27-02 and 27-03 (hub test helper rewrite and synctest migration)

## Self-Check: PASSED

- [x] internal/broker/broker.go exists with lastReadTime.IsZero() guard
- [x] internal/broker/broker_test.go exists with TestInterReadDelayBurst
- [x] 27-01-SUMMARY.md created
- [x] Commit 6b453e5 exists in git log
- [x] All broker tests pass (15/15)
- [x] go vet clean

---
*Phase: 27-hub-test-optimization*
*Completed: 2026-04-19*
