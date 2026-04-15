---
phase: 20-configuration-register-cleanup
plan: 02
subsystem: register
tags: [modbus, configuration, hardware-sweep, probe-cleanup]

# Dependency graph
requires:
  - phase: 20-configuration-register-cleanup/20-01
    provides: config-sweep tool and results.json from real hardware
provides:
  - Cleaned configuration.go with only hardware-verified registers
  - Updated test assertions matching post-cleanup probe/group counts
affects: [hub, batch-read, frontend-configuration-section]

# Tech tracking
tech-stack:
  added: []
  patterns: [hardware-verified register definitions, sweep-based static removal]

key-files:
  created: []
  modified:
    - internal/register/configuration.go
    - internal/register/register_test.go

key-decisions:
  - "Removed all 90 FAIL probes statically rather than runtime filtering -- per D-03 decision"
  - "Removed emsTimePeriodGroups() and networkConfigGroup() functions entirely since all probes in those groups failed"
  - "Left orphaned enum maps (CommunicationInterruptEnum, FanNoiseEnum, EMSTimePeriodModeEnum, IPAllocationEnum) in config_enum.go untouched -- Go allows unused package-level vars and they may be referenced elsewhere"

patterns-established:
  - "Hardware sweep verification: run tools/config-sweep against real hardware to validate register support before inclusion"

requirements-completed: [CONF-01, CONF-02, CONF-03]

# Metrics
duration: 46min
completed: 2026-04-15
---

# Phase 20 Plan 02: Configuration Register Cleanup Summary

**Removed 90 unsupported Modbus registers from configuration probe definitions based on real hardware sweep, eliminating batch read fallback warnings**

## Performance

- **Duration:** 46 min
- **Started:** 2026-04-15T16:17:20Z
- **Completed:** 2026-04-15T17:03:25Z
- **Tasks:** 3 (1 checkpoint + 2 auto)
- **Files modified:** 2

## Accomplishments
- Removed 90 registers returning Modbus exception 0x02 (illegal data address) from configuration.go
- Eliminated 8 entire groups: EMS Time Period Enable, EMS Time Periods 1-6, and Network Config
- Removed 15 individual FAIL probes from the Function Config 2 group
- Preserved all 224 hardware-verified PASS probes unchanged across 18 remaining groups
- Updated test assertions to reflect exact post-cleanup counts (18 groups, 224 probes)
- Full test suite passes with zero regressions

## Task Commits

Each task was committed atomically:

1. **Task 1: Run config sweep on real hardware** - (checkpoint:human-action, completed by user)
2. **Task 2: Remove failing probes and empty groups** - `0744aba` (feat)
3. **Task 3: Update TestConfigurationGroups assertions** - `bcb7fcd` (test)

## Files Created/Modified
- `internal/register/configuration.go` - Removed 90 FAIL probes, 8 empty groups, 2 generator functions; added sweep verification comment
- `internal/register/register_test.go` - Updated minimum group count (15->18), probe count (150->224), removed EMS count assertion

## Decisions Made
- Removed `emsTimePeriodGroups()` function entirely since all 6 generated EMS Time Period groups had every probe fail -- no partial removal needed
- Removed `networkConfigGroup()` function entirely since all 8 Network Config probes failed
- Left 4 orphaned enum maps in config_enum.go (CommunicationInterruptEnum, FanNoiseEnum, EMSTimePeriodModeEnum, IPAllocationEnum) -- Go does not error on unused package-level variables, and removing them is outside this plan's scope

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Removal Summary

### Groups Removed Entirely (all probes FAIL)
| Group | Probes Removed | Address Range |
|-------|---------------|---------------|
| EMS Time Period Enable | 1 | 0x1204 |
| EMS Time Period 1 | 11 | 0x1205-0x120F |
| EMS Time Period 2 | 11 | 0x1212-0x121C |
| EMS Time Period 3 | 11 | 0x121F-0x1229 |
| EMS Time Period 4 | 11 | 0x122C-0x1236 |
| EMS Time Period 5 | 11 | 0x1244-0x124E |
| EMS Time Period 6 | 11 | 0x1251-0x125B |
| Network Config | 8 | 0x2504-0x2512 |

### Individual Probes Removed from Function Config 2
| Probe | Address | Error |
|-------|---------|-------|
| Comm interruption control | 0x10C0 | illegal data address (0x02) |
| Comm interruption timeout | 0x10C1 | illegal data address (0x02) |
| Comm interruption preset | 0x10C2 | illegal data address (0x02) |
| Power enable control | 0x10CC | illegal data address (0x02) |
| Active power output | 0x10CD | illegal data address (0x02) |
| Reactive power output | 0x10CF | illegal data address (0x02) |
| Fan self-test control | 0x10E0 | illegal data address (0x02) |
| Fan noise mode | 0x10E1 | illegal data address (0x02) |
| Fan speed control | 0x10E4 | illegal data address (0x02) |
| Unbalanced power control | 0x10EB | illegal data address (0x02) |
| Unbalanced R-phase | 0x10EC | illegal data address (0x02) |
| Unbalanced S-phase | 0x10ED | illegal data address (0x02) |
| Unbalanced T-phase | 0x10EE | illegal data address (0x02) |
| Relay configuration | 0x10F0 | illegal data address (0x02) |
| Standby monitoring | 0x10F1 | illegal data address (0x02) |

### Functions Removed
- `emsTimePeriodGroups()` - generated 6 EMS Time Period groups (66 probes total)
- `networkConfigGroup()` - returned Network Config group (8 probes)

## Orphaned Enum Maps
The following enum maps in `config_enum.go` are no longer referenced by any probe in `configuration.go` but are left in place (Go allows unused package-level variables):
- `CommunicationInterruptEnum` (was used by 0x10C0)
- `FanNoiseEnum` (was used by 0x10E1)
- `EMSTimePeriodModeEnum` (was used by EMS Time Period groups; still referenced in register_test.go enum coverage test)
- `IPAllocationEnum` (was used by 0x2507)

## Next Phase Readiness
- Configuration section now contains only hardware-verified registers
- Batch read spans will auto-recompute from surviving probes (no changes to batch.go needed)
- Configuration section should load on real hardware without fallback warnings (CONF-03)
- Hardware verification recommended: run server against inverter and check logs for absence of "batch span failed" messages

---
*Phase: 20-configuration-register-cleanup*
*Completed: 2026-04-15*
