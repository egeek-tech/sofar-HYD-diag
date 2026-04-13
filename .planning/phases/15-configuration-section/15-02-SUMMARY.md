---
phase: 15-configuration-section
plan: 02
subsystem: hub
tags: [go, websocket, caching, modbus, configuration]

# Dependency graph
requires:
  - phase: 15-configuration-section/15-01
    provides: "register.ConfigurationGroups with 26 ProbeGroups and 259 probes"
provides:
  - "Configuration section registered in hub with readOnce caching"
  - "readOnce/hasReadOnce fields on Section struct for skip-on-reread behavior"
  - "D-09 read-once pattern: read_cycle skipped after initial read, refresh resets cache"
affects: [15-configuration-section/15-03, frontend-sidebar]

# Tech tracking
tech-stack:
  added: []
  patterns: ["readOnce caching pattern on Section struct for static device settings"]

key-files:
  created: []
  modified:
    - internal/hub/section.go
    - internal/hub/hub.go
    - internal/hub/hub_test.go
    - internal/hub/export_test.go

key-decisions:
  - "readOnce guard placed in handleReadCycle only -- subscribe and refresh paths bypass it intentionally"
  - "hasReadOnce set via SectionCompleteMessage type switch in event loop result handler"
  - "No hasReadOnce reset on unsubscribe -- configuration is static while connected to same inverter"

patterns-established:
  - "readOnce caching: Section.readOnce + hasReadOnce fields control auto-refresh skip for static sections"

requirements-completed: [SECT-02]

# Metrics
duration: 7min
completed: 2026-04-13
---

# Phase 15 Plan 02: Hub Configuration Section with Read-Once Caching Summary

**Configuration section registered in hub with D-09 read-once caching -- skips auto-refresh re-reads, explicit refresh resets cache**

## Performance

- **Duration:** 7 min
- **Started:** 2026-04-13T20:35:53Z
- **Completed:** 2026-04-13T20:43:03Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- Added readOnce and hasReadOnce fields to Section struct for skip-on-reread behavior
- Registered configuration section in hub between system and grid with readOnce=true
- Added readOnce guard in handleReadCycle to skip re-reads when cache is warm
- Intercept SectionCompleteMessage in event loop to set hasReadOnce=true
- Reset hasReadOnce on explicit MsgTypeRefresh before triggering re-read
- 5 comprehensive tests covering registration, caching, refresh reset, and non-regression

## Task Commits

Each task was committed atomically:

1. **Task 1 (RED): Failing tests for readOnce caching** - `8689640` (test)
2. **Task 1 (GREEN): Implement readOnce caching for configuration section** - `29b6107` (feat)

_Note: Task 2 tests were written as part of Task 1's TDD RED phase since both tasks cover the same test behaviors._

## Files Created/Modified
- `internal/hub/section.go` - Added readOnce and hasReadOnce fields to Section struct
- `internal/hub/hub.go` - Configuration section registration, readOnce guard in handleReadCycle, hasReadOnce set on section_complete, refresh cache reset
- `internal/hub/hub_test.go` - 5 new tests: registration, first read, skip second read_cycle, refresh resets cache, other sections unaffected
- `internal/hub/export_test.go` - Test helpers: GetSectionReadOnce, GetSectionHasReadOnce

## Decisions Made
- readOnce guard placed only in handleReadCycle -- subscribe and refresh paths intentionally bypass it so new subscribers always get data and explicit refresh always works
- hasReadOnce set by type-switching SectionCompleteMessage in the event loop result handler (line 196 area) rather than in streamStandardRead, keeping the streaming handler unmodified
- No hasReadOnce reset on unsubscribe or disconnect -- configuration register values are static while connected to the same inverter, so the cache remains valid

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Configuration section is registered and functional with read-once caching
- Ready for Plan 03 (frontend sidebar integration) to add the configuration section to the UI navigation
- The section uses streamStandardRead, so no frontend changes needed for data streaming

## Self-Check: PASSED

All 4 files verified present. All 2 commit hashes verified in git log.

---
*Phase: 15-configuration-section*
*Completed: 2026-04-13*
