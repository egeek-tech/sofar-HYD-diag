---
phase: 24-bms-batch-migration
plan: 01
subsystem: register
tags: [composite, bms, batch-migration, format-dispatch]
dependency_graph:
  requires: []
  provides: [bms_clock_composite, bms_sw_version_composite, bms_info_groups_v2]
  affects: [hub_streaming_bms, hub_routing]
tech_stack:
  added: []
  patterns: [composite_probe_dispatch, multi_register_composition]
key_files:
  created: []
  modified:
    - internal/register/format.go
    - internal/register/battery.go
    - internal/register/register_test.go
decisions:
  - "bms_clock uses 2-register packed uint32 via DecodeBMSClock, consistent with existing pattern"
  - "bms_sw_version extracts ASCII char from low byte of first register (Modbus big-endian convention)"
  - "BMSProtectionProbes removed; protection probes folded into BMSInfoGroups as second ProbeGroup"
  - "BMSInfoGroups returns 2 ProbeGroups: BMS Info (18 probes) and Protection (6 probes with Type: protection)"
metrics:
  duration: 4m
  completed: "2026-04-17T09:38:00Z"
  tasks_completed: 2
  tasks_total: 2
  files_modified: 3
---

# Phase 24 Plan 01: BMS Register Layer Composite Probes Summary

BMS clock and SW version Composite probe dispatch with Protection group restructure for batch-eligible BMSInfoGroups

## What Was Done

### Task 1: Add bms_clock and bms_sw_version Composite dispatch (cce4c89)

Added two new Composite probe types to the FormatValue and FormatRawValue dispatch chains in format.go:

- **bms_clock**: Combines 2 registers (hi/lo uint16) into packed uint32, dispatches to existing DecodeBMSClock for "YYYY-MM-DD HH:MM:SS" formatting. FormatRawValue returns address-range-prefixed decimal values.
- **bms_sw_version**: Combines 4 registers into "{char}{major}.{nonstd}.{minor}" version string (e.g., "V1.2.3"). ASCII character extracted from low byte of first register per Modbus big-endian convention.

Both handlers include len(data) guards (T-24-01, T-24-02 mitigations) returning `"<no data>"` on short input.

Added 6 unit tests: TestFormatValueBMSClock, TestFormatValueBMSClockNoData, TestFormatValueBMSSWVersion, TestFormatValueBMSSWVersionNoData, TestFormatRawValueBMSClock, TestFormatRawValueBMSSWVersion.

### Task 2: Update BMSInfoGroups with Composites and Protection group (7e193db)

Restructured BMSInfoGroups to return 2 ProbeGroups instead of 1:

- **BMS Info** (18 probes): Merged System Clock Hi/Lo into single `bms_clock` Composite (Addr: 0x9004, Count: 2). Merged SW Version Char/Major/NonStd/Minor into single `bms_sw_version` Composite (Addr: 0x9018, Count: 4). All other probes unchanged.
- **Protection** (6 probes, Type: "protection"): Folded former BMSProtectionProbes (0x9014-0x9017, 0x901C-0x901D) into BMSInfoGroups as a second group.

Removed BMSProtectionProbes function entirely (hub references will be updated in Plan 24-02).

Updated TestBMSInfoGroups to verify 2 groups, 18+6 probe counts, Composite probe attributes, and updated address lists. Replaced TestBMSProtectionProbes with TestBMSProtectionInGroups. Added TestBMSInfoGroupsBatchPlan verifying AnalyzeBatchPlan produces 3 spans: {0x9004, 26}, {0x9022, 12}, {0x902F, 2}.

## Deviations from Plan

None - plan executed exactly as written.

## Decisions Made

1. **bms_clock dispatch reuses DecodeBMSClock**: The existing bit-packed decoder handles the uint32 composition from two registers, keeping the Composite handler minimal.
2. **bms_sw_version ASCII from low byte**: Modbus big-endian stores the ASCII character in data[1] (low byte of first register), consistent with how the inverter encodes single-character fields.
3. **BMSProtectionProbes removal**: Function removed from register package. The hub_streaming.go caller (line 596) will be updated in Plan 24-02 which depends on this plan.

## Verification

- `go test ./internal/register/ -count=1` -- all tests pass (existing + 9 new)
- `go vet ./internal/register/` -- clean, no issues
- BMSProtectionProbes grep returns no matches in battery.go
- FormatValue bms_clock returns "2026-04-10 14:03:05" for test input 0x6914E0C5
- FormatValue bms_sw_version returns "V1.2.3" for test input [0x00,0x56,0x00,0x01,0x00,0x02,0x00,0x03]
- AnalyzeBatchPlan(BMSInfoGroups()) produces exactly 3 spans with correct start addresses and counts

## Self-Check: PASSED

- All 3 modified files exist on disk
- Both task commits (cce4c89, 7e193db) found in git log
- bms_clock and bms_sw_version Composite dispatch present in format.go
- bms_clock and bms_sw_version Composite probes present in battery.go BMSInfoGroups
- Protection ProbeGroup with Type: "protection" present in battery.go
- BMSProtectionProbes function confirmed removed from battery.go
