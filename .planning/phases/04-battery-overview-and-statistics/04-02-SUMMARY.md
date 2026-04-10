---
phase: 04-battery-overview-and-statistics
plan: 02
subsystem: api
tags: [modbus, websocket, bms, battery, topology, bitmap, statistics]

# Dependency graph
requires:
  - phase: 04-01
    provides: "Register definitions for battery, BMS, and statistics probe groups"
  - phase: 02
    provides: "Hub event loop, BrokerInterface, section registration, WebSocket message types"
provides:
  - "Battery section with auto-detect channel count from 0x066A"
  - "BMS section with write-0x9020/read-0x9022 bitmap cycle per tower"
  - "Statistics section with U32 register formatting"
  - "BMS topology configure handler with clamping"
  - "GroupData Type and BitmapData for bitmap widget rendering"
  - "WriteRegister on BrokerInterface"
  - "CLI flags -bat-inputs/-bat-towers/-bat-packs with /api/defaults serving topology"
affects: [04-03, 05]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Custom section read dispatching via switch in triggerSectionRead"
    - "BMS write-read cycle pattern: WriteRegister + Sleep + ReadBatch per tower"
    - "Battery auto-detect channel count from register value with section rebuild"
    - "GroupData Type field for polymorphic group rendering (bitmap, protection, standard)"

key-files:
  modified:
    - "internal/hub/hub.go"
    - "internal/hub/message.go"
    - "internal/hub/broker_iface.go"
    - "cmd/server/main.go"
    - "web/handler.go"
    - "internal/hub/export_test.go"
    - "internal/hub/hub_test.go"
    - "web/web_test.go"

key-decisions:
  - "Dispatch custom read handlers for bms/battery in triggerSectionRead switch, standard path for all others"
  - "BMS clock composed from 0x9004+0x9005 via DecodeBMSClock; SW version composed from 0x9018-0x901B"
  - "Topology auto-detect from 0x900D with mismatch flag sent to frontend in BitmapData"
  - "Protection registers formatted as hex (0x%04X) for bitmap inspection by user"

patterns-established:
  - "Custom section read: sections with non-standard read cycles get their own triggerXxxRead method"
  - "BitmapData struct: carries tower bitmap state, topology detection, and mismatch flag for frontend widget"
  - "handleConfigure switch: section-specific configure handling with input clamping per threat model"

requirements-completed: [BAT-01, BAT-02, BAT-03, BAT-04, BAT-05, BAT-06, STAT-01, STAT-02, STAT-03]

# Metrics
duration: 5min
completed: 2026-04-10
---

# Phase 04 Plan 02: Battery/BMS/Stats Hub Integration Summary

**Full backend integration of battery, BMS, and statistics sections with BMS write-read bitmap cycle, battery auto-detect, topology configure, and GroupData type extension for bitmap/protection widgets**

## Performance

- **Duration:** 5 min
- **Started:** 2026-04-10T19:06:14Z
- **Completed:** 2026-04-10T19:11:36Z
- **Tasks:** 2
- **Files modified:** 8

## Accomplishments
- Extended GroupData with Type and BitmapData fields for bitmap/protection widget rendering over WebSocket
- Added WriteRegister to BrokerInterface enabling BMS 0x9020 write-read cycle for pack online bitmap
- Registered battery (auto-detect channels from 0x066A), BMS (custom write-read cycle), and stats sections in hub
- BMS topology configure handler with input clamping (T-04-03) and immediate re-read on reconfigure
- CLI flags (-bat-inputs, -bat-towers, -bat-packs) with validation and /api/defaults serving topology

## Task Commits

Each task was committed atomically:

1. **Task 1: GroupData extension, BrokerInterface WriteRegister, ConfigPayload topology, DefaultsConfig, CLI flags, Hub constructor** - `f4243d4` (feat)
2. **Task 2: Register battery/bms/stats sections, BMS write-read cycle, topology configure handler, battery auto-detect** - `49ba52d` (feat)

## Files Created/Modified
- `internal/hub/message.go` - Added Type, BitmapData to GroupData; BatInputs/BatTowers/BatPacks to ConfigPayload
- `internal/hub/broker_iface.go` - Added WriteRegister method to BrokerInterface
- `internal/hub/hub.go` - Battery/BMS/stats section registration, triggerBMSRead, triggerBatteryRead, buildBMSGroupData, buildProtectionGroup, BMS configure handler, refactored triggerSectionRead dispatch
- `internal/hub/hub_test.go` - Added WriteRegister to mockBroker
- `internal/hub/export_test.go` - Updated test hub constructors for new NewHub signature
- `web/handler.go` - Added BatInputs/BatTowers/BatPacks to DefaultsConfig
- `web/web_test.go` - Updated NewHub calls for new signature
- `cmd/server/main.go` - Added -bat-inputs/-bat-towers/-bat-packs CLI flags with validation, updated NewHub and DefaultsConfig construction

## Decisions Made
- Dispatch custom read handlers for bms/battery via switch in triggerSectionRead; standard path handles system/grid/eps/pv/stats
- BMS clock composed from two 16-bit registers (0x9004+0x9005) using DecodeBMSClock; SW version composed from 0x9018-0x901B
- Topology auto-detected from 0x900D with mismatch flag propagated to frontend in BitmapData struct
- Protection registers formatted as hex (0x%04X) for bitmap inspection

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Fixed web_test.go NewHub calls for updated signature**
- **Found during:** Task 2 (test verification)
- **Issue:** web/web_test.go had two NewHub calls with old 3-parameter signature, failing to compile after NewHub was extended to accept topology parameters
- **Fix:** Updated both NewHub calls to pass default topology values (1, 2, 10)
- **Files modified:** web/web_test.go
- **Verification:** `go test ./... -count=1` passes
- **Committed in:** 49ba52d (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Necessary fix for test compilation after API change. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All three backend sections (battery, bms, stats) are registered and functional
- Frontend (Plan 03) can now receive grouped data including bitmap groups, protection groups, and statistics groups over WebSocket
- BMS configure message accepted and processed with topology clamping
- /api/defaults serves topology defaults for browser pre-population

## Self-Check: PASSED

All 8 modified files verified present. Both task commits (f4243d4, 49ba52d) verified in git log.

---
*Phase: 04-battery-overview-and-statistics*
*Completed: 2026-04-10*
