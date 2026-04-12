---
phase: 11-battery-pack-polish
plan: 01
subsystem: backend/hub
tags: [streaming, pack-drilldown, modbus, websocket]
dependency_graph:
  requires: []
  provides: [PackProbeGroups, streamPackRead, buildPackSchema, PackSchemaContext, packSkipRegisters]
  affects: [internal/hub, internal/register]
tech_stack:
  added: []
  patterns: [per-register-streaming, skip-list-tracking, pack-schema-context]
key_files:
  created: []
  modified:
    - internal/register/battery.go
    - internal/register/register_test.go
    - internal/hub/message.go
    - internal/hub/hub_streaming.go
    - internal/hub/hub.go
    - internal/hub/export_test.go
    - internal/hub/hub_test.go
decisions:
  - "Pack probe groups ordered Info/Cells/Balance/Temps/Status per D-03"
  - "Balance State kept as separate schema group (frontend renders it inline in cell card per D-17)"
  - "streamPackRead replaces triggerPackRead; old batch functions marked DEPRECATED"
  - "packSkipRegisters map tracks timed-out registers per D-04, clears on pack switch per D-05"
metrics:
  duration: 13m
  completed: "2026-04-12T21:51:00Z"
  tasks_completed: 2
  tasks_total: 2
  files_modified: 7
---

# Phase 11 Plan 01: Pack Streaming Backend Summary

Per-register streaming for pack drill-down with D-03 group ordering and unsupported register tracking via PackProbeGroups, streamPackRead, and PackSchemaContext.

## What Was Done

### Task 1: PackProbeGroups function and group order tests (TDD)

Created `PackProbeGroups()` in `internal/register/battery.go` returning 5 ordered `ProbeGroup` structs:

1. **Pack Info** (15 probes) -- RT block info items (Pack ID, Serial Number, Total Voltage, SOC, Current, capacities, counts) + Info block items (Balanced Bus Voltage/Current, Manufacturer, SOH, Rated Capacity)
2. **Cell Voltages** (18 probes, Type="cell_grid") -- 16 individual cell probes at 0x9051-0x9060, plus Max/Min Cell Voltage for summary computation per D-07
3. **Balance State** (1 probe, Type="balance") -- Balance bitmap at 0x9075
4. **Temperatures** (10 probes) -- Temp 1-4 from RT block, MOS Temp, Env Temp, Temp 5-8 from temps58 block
5. **Pack Status** (6 probes, Type="pack_status") -- Alarm/Protection/Fault from RT block + Alarm2/Protection2/Fault2 from Info block

Added `TestPackProbeGroupOrder` verifying group count, names, types, probe counts, and specific register addresses.

### Task 2: streamPackRead, pack schema, skip tracking, and hub wiring (TDD)

**message.go extensions:**
- Added `PackSchemaContext` struct with Input/Tower/Pack fields
- Added `PackContext *PackSchemaContext` field to `SectionSchemaMessage` (omitempty)
- Added `CellCount int` field to `SchemaGroup` for cell_grid groups

**hub.go changes:**
- Added `packSkipRegisters map[uint16]bool` to Hub struct, initialized in `NewHub`
- Updated `handleSelectPack`: clears skip list (D-05), sets up BMS section read context, calls `streamPackRead`
- Updated `handleReadCycle`: routes BMS pack reads to `streamPackRead` with proper section read context
- Marked `triggerPackRead` and `buildPackDataMessage` as DEPRECATED

**hub_streaming.go additions:**
- `buildPackSchema`: generates SectionSchemaMessage with pack_context and CellCount for cell_grid groups
- `streamPackRead`: full write-settle-read streaming cycle following streamBMSRead pattern
  - Writes 0x9020 to select pack (with retry on failure)
  - Waits configurable settle time
  - Sends section_schema with pack_context
  - Reads each probe individually via `ReadRegisters`, streams `register_value` messages
  - Tracks timeout/illegal address errors in skip list (D-04)
  - Sends `section_complete` at end
  - Checks `readCtx.Err()` after every operation for cancellation safety

**Tests added:**
- `TestPackSchemaContext`: Verifies schema JSON contains pack_context with correct coordinates
- `TestPackSchemaGroupOrder`: Verifies 5 groups in D-03 order
- `TestPackStreamingMessages`: End-to-end test verifying section_schema, register_value, section_complete flow (no pack_data)
- `TestPackSkipUnsupported`: Verifies timeout errors add register to skip list, second read skips it
- `TestPackSkipResetOnSwitch`: Verifies skip list clears when selecting a different pack

**Updated existing tests:**
- `TestPackDataMessageShape`: Converted from pack_data batch assertion to streaming message assertion
- `TestHandleSelectPack`: Updated ReadBatch count assertion to ReadRegisters count assertion

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Updated TestPackDataMessageShape for streaming path**
- **Found during:** Task 2 GREEN phase
- **Issue:** Existing test expected `pack_data` batch message which is no longer sent
- **Fix:** Rewrote test to verify streaming message flow (section_schema + register_value + section_complete)
- **Files modified:** internal/hub/hub_test.go
- **Commit:** 014864c

**2. [Rule 1 - Bug] Updated TestHandleSelectPack ReadBatch assertion**
- **Found during:** Task 2 GREEN phase
- **Issue:** Test expected 3 ReadBatch calls, but streamPackRead uses individual ReadRegisters
- **Fix:** Updated assertion to check for 10+ ReadRegisters calls (one per pack probe)
- **Files modified:** internal/hub/hub_test.go
- **Commit:** 014864c

**3. [Rule 1 - Bug] Added BMS subscribe before select_pack in TestPackDataMessageShape**
- **Found during:** Task 2 GREEN phase
- **Issue:** Streaming messages broadcast to section subscribers; test client was not subscribed to BMS
- **Fix:** Added subscribe command and drain of initial BMS overview messages before select_pack
- **Files modified:** internal/hub/hub_test.go
- **Commit:** 014864c

## Verification

```
go test ./internal/register/ -run TestPackProbeGroupOrder -v -count=1  -- PASS
go test ./internal/hub/ -run "TestPackSchema|TestPackStreaming|TestPackSkip" -v -count=1  -- PASS (5/5)
go test ./... -count=1  -- ALL PASS
go vet ./...  -- CLEAN
```

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 | 7e648ff | feat(11-01): add PackProbeGroups function with D-03 group ordering |
| 2 | 014864c | feat(11-01): add streamPackRead with pack schema, skip tracking, and hub wiring |

## Self-Check: PASSED

All 7 files found. Both commits (7e648ff, 014864c) verified. All key functions (PackProbeGroups, streamPackRead, buildPackSchema, PackSchemaContext, packSkipRegisters) confirmed present in correct files.
