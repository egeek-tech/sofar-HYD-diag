---
phase: 26-milestone-cleanup
plan: 01
subsystem: hub, register, ui
tags: [batch-plan, dead-code, enum-cleanup, pv-configure, span-tracker]

# Dependency graph
requires:
  - phase: 21-standard-section-batch-verification
    provides: BatchPlan and SpanTracker infrastructure
  - phase: 20-configuration-register-cleanup
    provides: Clean configuration register set
provides:
  - PV handleConfigure recomputes BatchPlan and resets SpanTracker after channel change
  - Dead readSpanIndividualFallback removed from streaming code
  - DCDC nav button removed from sidebar
  - Four orphaned enum maps removed from config_enum.go
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "All handleConfigure section mutations must recompute BatchPlan and reset SpanTracker"

key-files:
  created: []
  modified:
    - internal/hub/hub.go
    - internal/hub/hub_test.go
    - internal/hub/hub_streaming.go
    - internal/register/config_enum.go
    - internal/register/register_test.go
    - web/static/index.html

key-decisions:
  - "Test asserts total register count increase (not span count) since contiguous PV channels merge into one span"

patterns-established:
  - "handleConfigure pattern: after changing Groups/Probes, always recompute BatchPlan and reset SpanTracker"

requirements-completed: [CLEAN-01, CLEAN-02, CLEAN-03, CLEAN-04]

# Metrics
duration: 11min
completed: 2026-04-18
---

# Phase 26 Plan 01: Milestone Cleanup Summary

**PV BatchPlan staleness fix with SpanTracker reset, dead code removal (readSpanIndividualFallback, DCDC button, 4 orphaned enum maps)**

## Performance

- **Duration:** 11 min
- **Started:** 2026-04-17T21:56:12Z
- **Completed:** 2026-04-18T00:07:37Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments
- Fixed PV section BatchPlan staleness: handleConfigure now recomputes sec.BatchPlan and resets SpanTracker after channel count change, matching the battery section pattern
- Added TestConfigurePVBatchPlan verifying register count increases from 2 to 4 channels
- Removed dead readSpanIndividualFallback (non-Accum variant with zero callers), preserving the active Accum variant
- Removed orphaned DCDC nav button from sidebar
- Removed 4 orphaned enum maps (EMSTimePeriodModeEnum, IPAllocationEnum, CommunicationInterruptEnum, FanNoiseEnum) and corresponding test reference

## Task Commits

Each task was committed atomically:

1. **Task 1: Fix PV BatchPlan staleness and add test (CLEAN-01)** - `d56f197` (fix)
2. **Task 2: Remove dead code and orphaned artifacts (CLEAN-02, CLEAN-03, CLEAN-04)** - `19bdb78` (chore)

## Files Created/Modified
- `internal/hub/hub.go` - Added BatchPlan recomputation and SpanTracker reset in PV handleConfigure
- `internal/hub/hub_test.go` - Added TestConfigurePVBatchPlan verifying batch plan after channel change
- `internal/hub/hub_streaming.go` - Removed dead readSpanIndividualFallback function (29 lines)
- `internal/register/config_enum.go` - Removed 4 orphaned enum map declarations (28 lines)
- `internal/register/register_test.go` - Removed EMSTimePeriodModeEnum from TestConfigEnumMaps
- `web/static/index.html` - Removed DCDC nav button (4 lines)

## Decisions Made
- Test asserts total register count increase rather than span count increase, because contiguous PV channel registers merge into a single batch span regardless of channel count

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed BatchSpan field name in test**
- **Found during:** Task 1 (TestConfigurePVBatchPlan)
- **Issue:** Plan used `span.Count` but actual struct field is `span.TotalCount`
- **Fix:** Changed `span.Count` to `span.TotalCount` in test assertion
- **Files modified:** internal/hub/hub_test.go
- **Verification:** Test compiles and passes
- **Committed in:** d56f197 (Task 1 commit)

**2. [Rule 1 - Bug] Fixed test assertion from span count to register count**
- **Found during:** Task 1 (TestConfigurePVBatchPlan)
- **Issue:** Plan asserted span count increases when going from 2 to 4 channels, but PV registers are contiguous so both produce 2 spans (one merged channel span + one Total PV Power span). The difference is in TotalCount, not span count.
- **Fix:** Changed test to assert total register count increases instead of span count
- **Files modified:** internal/hub/hub_test.go
- **Verification:** Test passes: 2 channels = 7 regs, 4 channels = 13 regs
- **Committed in:** d56f197 (Task 1 commit)

---

**Total deviations:** 2 auto-fixed (2 bugs in plan's test template)
**Impact on plan:** Both auto-fixes were necessary for test correctness. The fix logic in hub.go was exactly as planned. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- v1.5 milestone cleanup complete: no stale batch plans, no dead functions, no orphaned UI elements, no unreferenced enum maps
- Codebase ready for milestone completion / tagging

## Self-Check: PASSED

All 6 modified files verified present. Both task commits (d56f197, 19bdb78) verified in git log.

---
*Phase: 26-milestone-cleanup*
*Completed: 2026-04-18*
