---
phase: 05-deep-battery-pack-diagnostics
plan: 01
subsystem: register
tags: [battery, pack, probes, bitmap, encoding]
dependency_graph:
  requires: [internal/register/probe.go, internal/register/fault.go]
  provides: [PackRTProbes, PackInfoProbes, PackTemps58Probes, EncodePackQuery, DecodeBMSBitmap, DecodeBalanceState, BMSAlarmBits, BMSProtectionBits, BMSFaultBits]
  affects: [internal/register/battery.go, internal/register/format.go]
tech_stack:
  added: []
  patterns: [FaultBit bitmap tables, probe function generators, coordinate-to-register encoding]
key_files:
  created: []
  modified: [internal/register/battery.go, internal/register/format.go, internal/register/register_test.go]
decisions:
  - "Cell voltage probes use loop generation (24 cells) rather than hand-written entries"
  - "Temperature Unit uses Unicode degree symbol for consistency with existing battery.go probes"
  - "BMS Alarm2/Protection2/Fault2 tables each have 4 entries matching the primary fault structure"
metrics:
  duration: 4min
  completed: 2026-04-11T12:16:37Z
  tasks_completed: 1
  tasks_total: 1
  files_modified: 3
---

# Phase 05 Plan 01: Pack Register Definitions Summary

Complete pack-level register probe definitions, BMS alarm/protection/fault bitmap decode tables, EncodePackQuery coordinate encoding, and DecodeBalanceState helper -- pure data foundation for hub and frontend plans.

## Task Results

| Task | Name | Commit(s) | Files | Status |
|------|------|-----------|-------|--------|
| 1 | Pack probe definitions, bitmap tables, EncodePackQuery, DecodeBalanceState | 29761f9 (RED), 227bd39 (GREEN) | battery.go, format.go, register_test.go | PASS |

## What Was Built

### EncodePackQuery (battery.go)
Maps 1-indexed UI coordinates (input, tower, pack, towersPerInput) to the 16-bit 0x9020 register value for BMS pack selection. Encoding matches the proven main.go.bak implementation: bits 0-7 = pack index (0-based), bits 8-11 = group index (0-based).

### PackRTProbes (battery.go)
44 probe definitions covering registers 0x9044-0x907C:
- Pack ID, timestamps, serial number (ASCII, 10 registers)
- 24 cell voltages (0x9051-0x9068) at millivolt resolution (Scale 0.001)
- Max/Min cell voltage (0x9069-0x906A)
- 4 temperature sensors + MOS + Env (0x906B-0x9070, signed, Scale 0.1)
- Current (0x9071, signed), capacities, cycle count
- Balance/alarm/protection/fault status registers
- Total voltage, SOC, total packs, cell count

### PackInfoProbes (battery.go)
8 probe definitions for 0x9104-0x9126: balanced bus voltage/current, manufacturer (ASCII), SOH, rated capacity, alarm2/protection2/fault2 status registers.

### PackTemps58Probes (battery.go)
4 probe definitions for temperature sensors 5-8 at 0x90BC-0x90BF.

### BMS Bitmap Tables (battery.go)
6 bitmap decode tables following the FaultBit pattern:
- BMSAlarmBits (0x9076): 10 alarm conditions
- BMSProtectionBits (0x9077): 13 protection conditions
- BMSFaultBits (0x9078): 4 fault conditions
- BMSAlarm2Bits (0x9124): 4 extended alarm conditions
- BMSProtection2Bits (0x9125): 4 extended protection conditions
- BMSFault2Bits (0x9126): 4 extended fault conditions

### DecodeBMSBitmap (battery.go)
Helper function that decodes a 16-bit register value against a FaultBit table, returning descriptions for all set bits matching a given address.

### DecodeBalanceState (format.go)
Interprets the 16-bit balance state register (0x9075): returns "Balanced" when zero, or "Balancing: Cell N, Cell M, ..." listing all cells with active balancing.

## Test Coverage

All tests in TDD RED-GREEN cycle:
- TestEncodePackQuery: 4 encoding cases covering all coordinate permutations
- TestPackRTProbes: Validates 38+ probes with specific address, type, scale, and unit checks for all critical probes
- TestPackInfoProbes: SOH, rated capacity, manufacturer, alarm2/protection2/fault2
- TestPackTemps58Probes: 4 probes at correct addresses with signed/scale/unit
- TestBMSAlarmTable: Cell OV/UV alarm entries at correct bits
- TestBMSProtectionTable: Cell OV protection entry
- TestBMSFaultTable_Pack: Entries exist for 0x9078
- TestDecodeBalanceState: Balanced, single cell, multi-cell, all 16 cells
- TestDecodeBMSBitmap: Multi-bit decode and zero-value handling

## Deviations from Plan

None -- plan executed exactly as written.

## Verification

```
go test ./internal/register/... -count=1 -v  -> PASS (0 failures)
go build ./...                                -> SUCCESS
All acceptance criteria grep checks           -> PASS
```

## Self-Check: PASSED

All files exist, all commits verified in git log.
