---
phase: 14-system-time-fix
plan: 01
subsystem: ui
tags: [modbus, time-format, batch-read, tooltip, streaming]

# Dependency graph
requires:
  - phase: 13-stats-merge
    provides: SystemGroups definition with Status group containing time registers
provides:
  - Composed "System time" row in HH:MM:SS DD-MM-YYYY format
  - Batch read of 6 time registers in single Modbus call (saves 2.5s per cycle)
  - Synthetic probe convention (Count: 0) for schema-only entries
  - Pipe-delimited raw value format for register range tooltips
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Synthetic probe (Count: 0) for schema-only entries skipped during read loop"
    - "Pipe-delimited raw value for multi-register composed values"

key-files:
  created: []
  modified:
    - internal/register/format.go
    - internal/register/system.go
    - internal/register/register_test.go
    - internal/hub/hub_streaming.go
    - internal/hub/hub_test.go
    - web/static/app.js

key-decisions:
  - "Count: 0 convention marks synthetic schema-only probes that are skipped during read iteration"
  - "Batch read after Status group rather than inline collection eliminates 5 individual Modbus reads"
  - "Pipe delimiter in raw value enables register range tooltip without new message fields"

patterns-established:
  - "Synthetic probe pattern: Count: 0 probes appear in schema for frontend skeleton but are skipped in read loop"
  - "Pipe-delimited raw value: 'addr_range | values' format parsed by frontend for multi-register tooltips"

requirements-completed: [CLEAN-02]

# Metrics
duration: 5min
completed: 2026-04-13
---

# Phase 14 Plan 01: System Time Fix Summary

**Consolidated 6 individual time register rows into single composed "System time" row with HH:MM:SS DD-MM-YYYY format, batch Modbus read, and register range tooltip**

## Performance

- **Duration:** 5 min
- **Started:** 2026-04-13T17:32:39Z
- **Completed:** 2026-04-13T17:37:31Z
- **Tasks:** 3
- **Files modified:** 6

## Accomplishments
- ComposeSystemTime now produces HH:MM:SS DD-MM-YYYY format (e.g., "14:30:05 10-04-2026")
- Status group reduced from 7 probes to 2 (Running state + synthetic System time), saving 5 Modbus reads per refresh cycle
- Single batch ReadRegisters(0x042C, 6) replaces 6 individual reads, saving ~2.5 seconds per cycle
- Frontend tooltip parses pipe-delimited raw value to show "Registers: 0x042C-0x0431" and "Raw: values"

## Task Commits

Each task was committed atomically:

1. **Task 1: Update register definitions and ComposeSystemTime format** - `b28bd82` (test, RED) + `f659538` (feat, GREEN)
2. **Task 2: Replace time-collection logic with batch read** - `2b3d95e` (feat)
3. **Task 3: Update frontend tooltip for register range format** - `4f162e2` (feat)

_Note: Task 1 followed TDD with RED (failing tests) then GREEN (implementation) commits._

## Files Created/Modified
- `internal/register/format.go` - Changed ComposeSystemTime to HH:MM:SS DD-MM-YYYY format
- `internal/register/system.go` - Replaced 7 Status probes with 2 (Running state + synthetic System time)
- `internal/register/register_test.go` - Updated assertions for new time format and 2-probe Status group
- `internal/hub/hub_streaming.go` - Removed time-collection logic, added Count==0 skip, added batch read after Status group
- `internal/hub/hub_test.go` - Updated TestNewRegisterValueComposedJSON for new format, addr, and pipe-delimited raw value
- `web/static/app.js` - Added pipe-delimiter detection in showTooltip for register range display

## Decisions Made
- Used Count: 0 convention to mark synthetic probes (schema-only, skipped during read iteration) rather than a separate boolean field
- Batch read positioned after probe loop (not inline) to keep streaming logic clean and avoid special-casing within the probe iteration
- Pipe delimiter ` | ` in raw value string enables frontend tooltip parsing without adding new fields to RegisterValueMessage

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- System time display consolidated and optimized
- Full test suite green (register, hub, all packages)
- Build compiles successfully with embedded frontend

## Self-Check: PASSED

All 6 modified files verified present. All 4 task commits verified in git log.

---
*Phase: 14-system-time-fix*
*Completed: 2026-04-13*
