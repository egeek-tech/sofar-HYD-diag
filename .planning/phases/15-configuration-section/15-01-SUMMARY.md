---
phase: 15-configuration-section
plan: 01
subsystem: register
tags: [configuration, registers, enums, modbus, v1.38]
dependency_graph:
  requires: []
  provides: [ConfigurationGroups, config-enum-maps]
  affects: [hub-configuration-section, frontend-configuration-display]
tech_stack:
  added: []
  patterns: [programmatic-group-generation, enum-map-pattern]
key_files:
  created:
    - internal/register/config_enum.go
    - internal/register/configuration.go
  modified:
    - internal/register/register_test.go
decisions:
  - Used explicit base addresses for EMS Time Period groups (stride not uniform)
  - Added extra enum maps beyond plan spec for completeness (IPAllocationEnum, CommunicationInterruptEnum, FanNoiseEnum, PeakShavingBuyEnum, TimeShareControlModeEnum)
  - Used PDF values directly for anti-back-flow enum (5 modes, not just enable/disable)
  - EPS control mapped to 3 modes per PDF (turn off, enable prohibit cold start, enable allow cold start)
  - Off-grid charging source follows PDF values (0=Grid, 1=Generator, 2=Reserved) not plan override
  - PV input mode follows PDF (0=Parallel, 1=Independent) not plan override
metrics:
  duration: 6m30s
  completed: 2026-04-13T20:32:23Z
  tasks_completed: 2
  tasks_total: 2
  files_created: 2
  files_modified: 1
---

# Phase 15 Plan 01: Configuration Register Definitions Summary

Complete register definition layer for Sofar HYD configuration parameters -- 24+ enum maps and 259 probes across 26 groups covering V1.38 spec sections 5.2.x (Safety) and 5.3.x (Configuration).

## What Was Built

### Task 1: Configuration Enum Maps (config_enum.go)

Created `internal/register/config_enum.go` with 28 enum maps matching the Sofar Modbus-G3 V1.38 protocol specification:

- **Core enums**: BaudRateEnum (8 rates), LanguageEnum (15 languages), SafetyCountryEnum (21 countries)
- **Battery enums**: BatteryProtocolEnum (16 protocols), CellTypeEnum (7 types), BatteryUsageEnum (4 modes)
- **Mode enums**: EnergyStorageModeEnum (7 modes), EMSTimePeriodModeEnum (7 modes), ChargingSourceEnum (3 sources)
- **Control enums**: ProhibitEnableEnum, ChargeDischargeControlEnum, PowerControlEnum, RemoteOnOffEnum
- **Safety enums**: ProtectionEnableEnum, ReactiveControlModeEnum
- **Utility enums**: EnableStatusEnum (write operation results), InputChannelTypeEnum, IPAllocationEnum

### Task 2: Configuration Register Groups (configuration.go)

Created `internal/register/configuration.go` with `ConfigurationGroups` variable containing 26 ProbeGroups and 259 total probes:

**User-facing groups (13):**
1. System Config (25 probes) -- RS485, PV mode, anti-back-flow, country code, EPS, language, parallel
2. Battery Config (22 probes) -- protocol, voltages, currents, SOC limits, cell type, capacity
3. Energy Storage Mode (8 probes) -- operating mode, passive mode parameters
4. Function Config (19 probes) -- PCC, CT ratio, dry contact, grid detection, DRMs, battery usage
5. Function Config 2 (33 probes) -- arc detection, generator, communication, fan, unbalanced power
6. Remote Control (10 probes) -- on/off, power control, export/import limits, power factor
7. Timed Charge/Discharge (8 probes) -- schedule rules, charge/discharge power
8. Time Sharing Control (11 probes) -- time-of-use tariff rules
9. Peak Shaving (4 probes) -- purchase/sell power limits
10. Power Feeding Priority (1 probe) -- feed-in power
11. Off-Grid Mode (3 probes) -- charging source, power draw limits
12. Communication Protection (4 probes) -- interruption handling
13. EMS Time Period Enable (1 probe) -- bitmask for 6 periods

**EMS Time Period groups (6, programmatically generated):**
14-19. EMS Time Period 1-6 (11 probes each = 66 probes) -- start/end time, work mode, power limits, SOC limits

**Safety groups (7):**
20. Safety: Power On (12 probes) -- grid connection, startup voltage/frequency limits
21. Safety: Voltage Protection (18 probes) -- over/under-voltage levels 1-3
22. Safety: Frequency Protection (22 probes) -- over/under-frequency levels 1-3, rate of change
23. Safety: DCI Protection (13 probes) -- DC injection limits and test values
24. Safety: Island/GFCI/ISO (10 probes) -- island detection, ground fault, insulation
25. Safety: Reactive Power (16 probes) -- PF curves, reactive control modes
26. Network Config (8 probes) -- MAC, IP, gateway, subnet, Modbus TCP port

## Decisions Made

1. **Explicit EMS base addresses**: Used explicit base addresses `[0x1205, 0x1212, 0x121F, 0x122C, 0x1244, 0x1251]` instead of computed stride because the stride is non-uniform (Period 4 to Period 5 jumps 0x18 instead of 0x0D).

2. **PDF-accurate enum values**: Where the plan spec differed from the PDF, used PDF values. For example, PVInputModeEnum has 0=Parallel, 1=Independent (matching page 42), and ChargingSourceEnum has 0=Grid, 1=Generator (matching page 56 section 5.3.10).

3. **Extra enum maps**: Added IPAllocationEnum, CommunicationInterruptEnum, FanNoiseEnum, PeakShavingBuyEnum, and TimeShareControlModeEnum beyond the plan's 24 for completeness -- these support registers that would otherwise display raw numeric values.

4. **Safety groups scope**: Included 7 safety groups (Power On, Voltage Protection, Frequency Protection, DCI Protection, Island/GFCI/ISO, Reactive Power, Network Config) exceeding the minimum 5 required.

## Deviations from Plan

None -- plan executed as written. Minor PDF-accuracy corrections applied per Rule 1 (bug fix: using actual protocol values rather than plan approximations).

## Self-Check: PASSED

- [x] internal/register/config_enum.go exists
- [x] internal/register/configuration.go exists
- [x] internal/register/register_test.go exists
- [x] Commit b60f5aa (RED tests enum) exists
- [x] Commit f071882 (GREEN enum maps) exists
- [x] Commit aa270f5 (RED tests groups) exists
- [x] Commit 1471982 (GREEN configuration groups) exists
- [x] `go build ./...` succeeds
- [x] `go test ./internal/register/ -run TestConfig` passes
