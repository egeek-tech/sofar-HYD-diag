---
phase: 04-battery-overview-and-statistics
plan: 01
subsystem: register
tags: [modbus, u32, battery, bms, statistics, probe, register-map]

# Dependency graph
requires:
  - phase: 03-core-monitoring-sections
    provides: Probe/ProbeGroup structs, FormatValue, enum pattern, GeneratePVGroups pattern
provides:
  - U32 bool field on Probe struct for 32-bit unsigned register support
  - FormatValue U32 handling with Sofar big-endian word order
  - BatteryStateEnum mapping values 1-5
  - GenerateBatteryGroups(channels) dynamic multi-channel battery ProbeGroup generator
  - BMSInfoGroups() with 18 BMS identity/status probes
  - BMSProtectionProbes() for 6 protection/alarm registers
  - StatisticsGroups() with 4 time-period groups of 6 U32 energy metrics each
  - DecodeBMSClock for packed BMS timestamp decoding
  - DecodeTopology for battery topology parameter decoding
  - ProbeGroup Type field for widget dispatch (bitmap, protection, standard)
affects: [04-02-hub-battery-statistics, 04-03-frontend-battery-statistics]

# Tech tracking
tech-stack:
  added: []
  patterns: [U32 dual-register probe pattern, dynamic ProbeGroup generator with stride-based addressing, packed bitfield decode helpers]

key-files:
  created:
    - internal/register/statistics.go
  modified:
    - internal/register/probe.go
    - internal/register/probe_group.go
    - internal/register/format.go
    - internal/register/enum.go
    - internal/register/battery.go
    - internal/register/register_test.go

key-decisions:
  - "U32 probes use Sofar word order: high word at low address, low word at high address"
  - "GenerateBatteryGroups follows same dynamic generator pattern as GeneratePVGroups"
  - "Statistics addresses use stride-4 interleaved layout matching register map (today/total pairs, month/year pairs)"
  - "BMSProtectionProbes returns flat Probe slice (not ProbeGroup) since hub decodes bitmaps differently"

patterns-established:
  - "U32 Probe pattern: U32=true, Count=2, FormatValue reads 4 bytes as 32-bit unsigned with scale"
  - "ProbeGroup Type field: empty=standard rows, 'bitmap'=bitmap widget, 'protection'=protection card"
  - "Packed bitfield decode helpers: DecodeBMSClock and DecodeTopology for register value interpretation"

requirements-completed: [BAT-01, BAT-02, BAT-03, BAT-04, BAT-05, STAT-01, STAT-02, STAT-03]

# Metrics
duration: 4min
completed: 2026-04-10
---

# Phase 04 Plan 01: Register Definitions Summary

**U32 register support, multi-channel battery groups, BMS info/protection probes, and 4-period statistics groups with 24 U32 energy metrics**

## Performance

- **Duration:** 4 min
- **Started:** 2026-04-10T18:59:28Z
- **Completed:** 2026-04-10T19:03:58Z
- **Tasks:** 2
- **Files modified:** 7

## Accomplishments
- Extended Probe struct with U32 field and FormatValue with 32-bit unsigned handling (bounds-checked per T-04-01)
- Replaced flat BatteryProbes/BatteryStateProbes/BMSProbes/BDUProbes with dynamic GenerateBatteryGroups generator supporting N channels
- Created comprehensive BMS info (18 probes) and protection (6 registers) definitions
- Built StatisticsGroups returning 4 time-period groups (Today/Total/Month/Year) with 6 U32 energy metrics each
- Added DecodeBMSClock and DecodeTopology helpers for packed register value interpretation
- Added ProbeGroup Type field for widget dispatch
- All 40 register tests pass with zero regressions

## Task Commits

Each task was committed atomically:

1. **Task 1: U32 Probe extension, FormatValue U32 handling, BatteryStateEnum, GenerateBatteryGroups** - `4b4c661` (feat)
2. **Task 2: ProbeGroup Type field, BMSInfoGroups, BMSProtectionProbes, StatisticsGroups, DecodeBMSClock, DecodeTopology** - `73ecf4c` (feat)

_TDD: Both tasks followed RED-GREEN flow with tests written first._

## Files Created/Modified
- `internal/register/probe.go` - Added U32 bool field to Probe struct
- `internal/register/probe_group.go` - Added Type string field to ProbeGroup struct
- `internal/register/format.go` - Added U32 handling in FormatValue, DecodeBMSClock, DecodeTopology
- `internal/register/enum.go` - Added BatteryStateEnum (5 entries)
- `internal/register/battery.go` - Rewrote with GenerateBatteryGroups, BMSInfoGroups, BMSProtectionProbes
- `internal/register/statistics.go` - New file with StatisticsGroups (4 groups x 6 U32 metrics)
- `internal/register/register_test.go` - Added 19 new tests for all Task 1 and Task 2 functionality

## Decisions Made
- U32 probes use Sofar word order: high word at low address, low word at high address
- GenerateBatteryGroups follows same dynamic generator pattern as GeneratePVGroups (stride-based addressing)
- Statistics addresses use stride-4 interleaved layout matching register map
- BMSProtectionProbes returns flat Probe slice since hub will decode bitmaps differently than standard ProbeGroup rendering

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed incorrect test data for U32 large value encoding**
- **Found during:** Task 1 (TDD GREEN phase)
- **Issue:** Plan specified lo_word=54519 for encoding 1234567, but correct value is 54919 (1234567 & 0xFFFF = 54919)
- **Fix:** Corrected test data from 54519 to 54919
- **Files modified:** internal/register/register_test.go
- **Verification:** TestFormatValueU32Large passes with correct encoding
- **Committed in:** 4b4c661 (Task 1 commit)

**2. [Rule 1 - Bug] Used valid BMS clock test value instead of plan's invalid one**
- **Found during:** Task 2 (TDD RED phase)
- **Issue:** Plan's test value 0x68A50F45 decodes to min=61 which is invalid. Computed correct value 0x6914E0C5 for "2026-04-10 14:03:05"
- **Fix:** Used 0x6914E0C5 as test value instead of 0x68A50F45
- **Files modified:** internal/register/register_test.go
- **Verification:** TestDecodeBMSClock passes with valid timestamp
- **Committed in:** 73ecf4c (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (2 bug fixes in test data)
**Impact on plan:** Both fixes corrected arithmetic errors in test expectations. No scope change.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All register definitions ready for hub consumption (Plan 02)
- ProbeGroup Type field available for frontend widget dispatch (Plan 03)
- U32 support enables Statistics section with 32-bit energy counters
- DecodeBMSClock and DecodeTopology ready for hub-level composition

## Self-Check: PASSED

All 8 files verified present. Both commit hashes (4b4c661, 73ecf4c) found in git log.

---
*Phase: 04-battery-overview-and-statistics*
*Completed: 2026-04-10*
