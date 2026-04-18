---
phase: 22-spantracker-integration
plan: 03
subsystem: hub
tags: [modbus, battery, reconnect, spantracker, batch-read]

# Dependency graph
requires:
  - phase: 22-02
    provides: Battery batch read with auto-detection via 0x066A
provides:
  - Battery section reset to 2-channel default on inverter reconnect
  - Fresh channel auto-detection after reconnect without app restart
affects: [battery, hub, reconnect-resilience]

# Tech tracking
tech-stack:
  added: []
  patterns: [reconnect-reset-to-defaults, fresh-allocation-on-rebuild]

key-files:
  created: []
  modified:
    - internal/hub/hub.go
    - internal/hub/hub_test.go

key-decisions:
  - "Reset battery to 2-channel default (not 0-channel or last-known) on reconnect, matching registerBuiltinSections initial state"
  - "Fresh slice allocation in handleStateEvent to avoid shared-array hazard with GenerateBatteryGroups/InternalInfoGroups"

patterns-established:
  - "Section-specific reconnect reset: sections with auto-detection should reset to safe defaults on StateConnected, letting next read cycle re-detect"

requirements-completed: [RESIL-01, RESIL-02]

# Metrics
duration: 5min
completed: 2026-04-18
---

# Phase 22 Plan 03: Battery Reconnect Channel Reset Summary

**Battery section resets to 2-channel default on disconnect/reconnect, re-detecting actual channel count via 0x066A on next read cycle**

## Performance

- **Duration:** 5 min
- **Started:** 2026-04-18T06:47:53Z
- **Completed:** 2026-04-18T06:52:53Z
- **Tasks:** 1 (TDD: RED + GREEN)
- **Files modified:** 2

## Accomplishments
- Battery Groups/Probes/BatchPlan reset to 2-channel default in handleStateEvent(StateConnected)
- Next streamBatteryBatchRead after reconnect re-detects actual channels via 0x066A
- No application restart required to recover correct channel configuration after disconnect
- Integration test proves 4-channel -> reconnect -> 2-channel reset -> 1-channel re-detection cycle

## Task Commits

Each task was committed atomically:

1. **Task 1 (RED): Failing test for battery reconnect reset** - `b35bc01` (test)
2. **Task 1 (GREEN): Reset battery section on reconnect** - `88f630d` (feat)

## Files Created/Modified
- `internal/hub/hub.go` - Added battery section reset in handleStateEvent(StateConnected) after SpanTracker reset loop
- `internal/hub/hub_test.go` - Added TestBatteryBatchRead_ReconnectResetsChannels integration test

## Decisions Made
- Reset to 2-channel default (matching registerBuiltinSections line 551) rather than 0 channels or attempting to cache last-known count. The 2-channel default is safe because streamBatteryBatchRead always pre-reads 0x066A to detect actual count.
- Used fresh slice allocation (`make([]register.ProbeGroup, 0, cap)`) to avoid the shared-array hazard documented in streamBatteryBatchRead, consistent with the existing rebuild pattern at lines 290-294.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## TDD Gate Compliance

- RED gate: `b35bc01` (test commit - TestBatteryBatchRead_ReconnectResetsChannels fails with channels=4 instead of expected 2)
- GREEN gate: `88f630d` (feat commit - test passes after adding battery reset in handleStateEvent)
- REFACTOR gate: Not needed - implementation is minimal (14 lines) and follows existing patterns

## Next Phase Readiness
- Battery reconnect resilience complete
- All existing battery tests pass (SpanReads, AutoDetect, OutputEquivalence, SpanFallback)
- All existing reconnect tests pass (SpanTrackerResetOnReconnect)

## Self-Check: PASSED

- internal/hub/hub.go: FOUND
- internal/hub/hub_test.go: FOUND
- 22-03-SUMMARY.md: FOUND
- Commit b35bc01: FOUND
- Commit 88f630d: FOUND

---
*Phase: 22-spantracker-integration*
*Completed: 2026-04-18*
