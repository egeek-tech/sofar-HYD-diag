---
phase: 06-battery-pack-access-fix
plan: 01
subsystem: hub
tags: [modbus, topology, constants, battery, bms]

# Dependency graph
requires: []
provides:
  - "Topology constants (TopoTowers=2, TopoPacksPerTower=10, TopoCellsPerPack=16)"
  - "Simplified NewHub(broker, logger, pvChannels) signature"
  - "16-cell PackRTProbes (0x9051-0x9060)"
affects: [06-02, 06-03]

# Tech tracking
tech-stack:
  added: []
  patterns: ["Package-level topology constants replace configurable struct fields"]

key-files:
  created: []
  modified:
    - internal/hub/hub.go
    - internal/hub/message.go
    - internal/hub/export_test.go
    - internal/hub/hub_test.go
    - internal/register/battery.go
    - internal/register/register_test.go
    - cmd/server/main.go
    - web/handler.go
    - web/web_test.go

key-decisions:
  - "Hardcode input=1 in handleSelectPack (single battery input per hardware)"
  - "Use constants in all topology references rather than passing values through"

patterns-established:
  - "TopoTowers/TopoPacksPerTower/TopoCellsPerPack as exported package constants for test access"

requirements-completed: [PACK-02, PACK-03]

# Metrics
duration: 6min
completed: 2026-04-11
---

# Phase 06 Plan 01: Hardcode Topology Constants Summary

**Replaced configurable battery topology with compile-time constants (2 towers, 10 packs, 16 cells) and reduced cell probes from 24 to 16**

## Performance

- **Duration:** 6 min
- **Started:** 2026-04-11T17:08:02Z
- **Completed:** 2026-04-11T17:14:14Z
- **Tasks:** 2
- **Files modified:** 9

## Accomplishments
- Hub struct simplified: removed defaultBatInputs/defaultBatTowers/defaultBatPacks fields, replaced with TopoTowers/TopoPacksPerTower/TopoCellsPerPack constants
- NewHub signature reduced from 6 params to 3 (broker, logger, pvChannels)
- PackRTProbes now emits exactly 16 cell voltage probes (0x9051-0x9060) matching actual hardware
- Removed BMS topology configuration from handleConfigure (no longer user-configurable)
- Removed bat-inputs/bat-towers/bat-packs CLI flags and their validation
- All 5 test packages pass with updated signatures and expectations
- New TestTopologyConstants test validates constant values

## Task Commits

Each task was committed atomically:

1. **Task 1: Replace topology fields with constants and update all signatures** - `73ec8c4` (feat)
2. **Task 2: Reduce cell probes to 16 and update all tests** - `0f7413a` (feat)

## Files Created/Modified
- `internal/hub/hub.go` - Added topology constants, simplified Hub struct and NewHub, updated triggerBMSRead/handleSelectPack/triggerPackRead/buildPackDataMessage to use constants, removed BMS case from handleConfigure
- `internal/hub/message.go` - Removed BatInputs/BatTowers/BatPacks from ConfigPayload, updated Cells comment for 16 cells
- `internal/hub/export_test.go` - Updated NewTestHub/NewTestHubWithInterval/NewTestHubWithPVChannels to 3-param NewHub
- `internal/hub/hub_test.go` - Updated cell grid expectations to 16 cells, updated makePackRTData, added TestTopologyConstants
- `internal/register/battery.go` - Changed cell voltage loop from 24 to 16 (0x9051-0x9060)
- `internal/register/register_test.go` - Updated TestPackRTProbes: check Cell 16, verify Cell 17 absent, expect 16 cell probes
- `cmd/server/main.go` - Removed bat-inputs/bat-towers/bat-packs flags and validation, simplified NewHub and DefaultsConfig calls
- `web/handler.go` - Removed BatInputs/BatTowers/BatPacks from DefaultsConfig struct
- `web/web_test.go` - Updated hub.NewHub calls to 3-param signature

## Decisions Made
- Hardcoded input=1 in handleSelectPack since hardware has a single battery input
- Used package-level exported constants for topology so tests can assert against them directly
- Updated makePackRTData test helper to generate only 16 cell values and matching max/min register values

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Updated cell grid in buildPackDataMessage**
- **Found during:** Task 1
- **Issue:** buildPackDataMessage hardcoded 24 cells in the cell grid construction; with 16-cell probes this would read stale/zero data for cells 17-24
- **Fix:** Changed cell grid loop to use TopoCellsPerPack constant
- **Files modified:** internal/hub/hub.go
- **Verification:** go build ./... succeeds, tests pass
- **Committed in:** 73ec8c4 (Task 1 commit)

**2. [Rule 1 - Bug] Updated makePackRTData test helper and cell grid assertions**
- **Found during:** Task 2
- **Issue:** Test data generated 24 cell values and assertions expected 24 cells; needed to match new 16-cell reality
- **Fix:** Updated makePackRTData to 16 cells, max cell register to 3215, and test expectations to 16 cells
- **Files modified:** internal/hub/hub_test.go
- **Verification:** go test ./... passes
- **Committed in:** 0f7413a (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (2 bugs)
**Impact on plan:** Both auto-fixes were necessary correctness changes directly caused by the cell count reduction. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Topology constants are in place for Plan 02 (bitmap cycling) to reference
- NewHub simplified signature ready for all callers
- 16-cell probe list matches actual hardware for accurate pack data display

## Self-Check: PASSED

All 10 files verified present. Both task commits (73ec8c4, 0f7413a) found in git log.

---
*Phase: 06-battery-pack-access-fix*
*Completed: 2026-04-11*
