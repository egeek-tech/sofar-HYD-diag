---
phase: 06-battery-pack-access-fix
plan: 02
subsystem: hub
tags: [modbus, bitmap, bms, battery, topology, cycling]

# Dependency graph
requires: [06-01]
provides:
  - "Per-tower bitmap cycling in triggerBMSRead via 0x9020 write + 0x9022 read"
  - "Distinct online/offline bitmaps per tower in BMS overview"
affects: [06-03]

# Tech tracking
tech-stack:
  added: []
  patterns: ["Write-cycle-read pattern for BMS register queries with 500ms settle time"]

key-files:
  created: []
  modified:
    - internal/hub/hub.go
    - internal/hub/hub_test.go

key-decisions:
  - "Keep 0x9022 in initial ReadBatch (stale value ignored) to avoid index-shifting complexity"
  - "Use writeErrQueue in mockBroker for per-call write error control in tests"

patterns-established:
  - "Per-tower bitmap cycling: write group index to 0x9020, sleep 500ms, read 0x9022"

requirements-completed: [PACK-01]

# Metrics
duration: 4min
completed: 2026-04-11
---

# Phase 06 Plan 02: Per-Tower Bitmap Cycling Summary

**Replaced single-bitmap-copied-to-all-towers with write-cycle-read loop that queries each tower individually via 0x9020, producing distinct online/offline bitmaps per tower**

## Performance

- **Duration:** 4 min
- **Started:** 2026-04-11T17:18:53Z
- **Completed:** 2026-04-11T17:22:39Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- triggerBMSRead Step 3 rewritten: writes group index to 0x9020 per tower, waits 500ms settle, then reads 0x9022 for that tower's true bitmap
- Write failure logs warning and leaves tower bitmap as 0 (all packs offline) -- graceful degradation
- BitmapData construction uses TopoTowers and TopoPacksPerTower constants directly
- Removed unused local `towers`/`packs` variables from triggerBMSRead
- TestBMSBitmapCycling verifies 2 write calls (0x0000 for group 0, 0x0100 for group 1), 3 ReadBatch calls, and distinct bitmap values (0x03FF vs 0x001F)
- TestBMSBitmapCyclingWriteFailure verifies tower 0 bitmap is 0 when write fails, tower 1 still gets correct bitmap
- Added writeErrQueue to mockBroker for per-call error control
- Added makeBMSInfoResults helper for BMS batch mock data
- Full test suite green (all 5 packages pass)

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement per-tower bitmap cycling in triggerBMSRead** - `5882384` (feat)
2. **Task 2: Add TestBMSBitmapCycling test** - `35b2e3e` (test)

## Files Created/Modified
- `internal/hub/hub.go` - Replaced Step 3 single-bitmap extraction with per-tower write-cycle-read loop; updated BitmapData to use topology constants; removed unused local variables
- `internal/hub/hub_test.go` - Added TestBMSBitmapCycling, TestBMSBitmapCyclingWriteFailure, makeBMSInfoResults helper, writeErrQueue field on mockBroker

## Decisions Made
- Kept 0x9022 in the initial ReadBatch to avoid index-shifting the results array; the stale bitmap value is simply ignored since bitmap cycling reads it separately per tower
- Added writeErrQueue (per-call error list) to mockBroker to support selective write failure testing without changing existing writeErr/writeErrCount behavior

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed mockBroker writeErrCount semantics for selective failure**
- **Found during:** Task 2
- **Issue:** The existing `writeErrCount` field treated count=0 as "error forever", so setting count=1 would fail the first call then also fail all subsequent calls (falling through to forever mode)
- **Fix:** Added `writeErrQueue []error` field to mockBroker that pops per-call errors; falls back to existing writeErr behavior when queue is empty
- **Files modified:** internal/hub/hub_test.go
- **Commit:** 35b2e3e

**Total deviations:** 1 auto-fixed (1 test infrastructure bug)
**Impact on plan:** Minimal -- mock enhancement needed for the planned test scenario. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Per-tower bitmap cycling is in place; each tower now shows its true online/offline status
- Frontend (Plan 03) can rely on distinct `Online[0]` and `Online[1]` values in the bitmap group

## Self-Check: PASSED

All 3 files verified present. Both task commits (5882384, 35b2e3e) found in git log.

---
*Phase: 06-battery-pack-access-fix*
*Completed: 2026-04-11*
