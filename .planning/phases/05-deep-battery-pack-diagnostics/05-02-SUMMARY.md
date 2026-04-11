---
phase: 05-deep-battery-pack-diagnostics
plan: 02
subsystem: hub-pack-selection
tags: [websocket, pack-data, bms, write-settle-read, modbus]
dependency_graph:
  requires: [05-01]
  provides: [select_pack handler, pack_data message, pack_error message, triggerPackRead, auto-refresh pack]
  affects: [internal/hub/hub.go, internal/hub/message.go]
tech_stack:
  added: []
  patterns: [write-settle-read cycle, per-client message routing, TDD red-green]
key_files:
  created: []
  modified:
    - internal/hub/hub.go
    - internal/hub/message.go
    - internal/hub/hub_test.go
decisions:
  - PackDataMessage is a separate struct from OutboundMessage to carry the richer pack payload
  - Cell voltages sent as raw millivolt integers (not formatted strings) for frontend computation
  - Pack selection cleared on BMS subscribe/unsubscribe to reset to overview view
  - sendPackDataToClient and sendPackError send directly to requesting client, not broadcast
metrics:
  duration: 7min
  completed: 2026-04-11T12:26:30Z
  tasks: 1
  files: 3
---

# Phase 05 Plan 02: Hub Pack Selection and Data Retrieval Summary

Hub-level select_pack handler with write-settle-read cycle: writes 0x9020 via EncodePackQuery, 1s settle, reads 3 register blocks (RT/Info/Temps58), builds structured PackDataMessage with 5 groups including raw millivolt cell array and bitmap-decoded alarm/protection/fault status.

## What Was Done

### Task 1: Pack message types and select_pack handler with triggerPackRead (TDD)

**RED:** Added 4 failing tests (TestHandleSelectPack, TestPackDataMessageShape, TestPackErrorOnWriteTimeout, TestEncodePackQueryInHandler) with enhanced mock broker supporting WriteRegister call tracking, error injection, and per-call ReadBatch result queues.

**GREEN:** Implemented the full pack selection pipeline:

- **message.go**: Added `MsgTypeSelectPack`, `MsgTypePackData`, `MsgTypePackError` constants. Added `Input`, `Tower`, `Pack` fields to `InboundMessage`. Created `PackDataMessage`, `PackGroup`, and `PackErrorMessage` structs with full JSON contract matching UI-SPEC.

- **hub.go**: Added `packSelection` struct and `selectedPack` field to Hub. Added `MsgTypeSelectPack` case to `handleCommand`. Implemented:
  - `handleSelectPack`: Validates/clamps input/tower/pack to topology bounds (T-05-02 mitigation)
  - `triggerPackRead`: Async goroutine performing write-settle-read cycle (0x9020 write, 1s settle, retry with 2s on failure, 3 ReadBatch calls)
  - `buildPackDataMessage`: Assembles 5 groups from 3 register block results
  - `sendPackError`/`sendPackDataToClient`: Direct-to-client message routing
  - `extractU16`/`extractS16`: Register data extraction helpers
  - Auto-refresh re-triggers pack read when `selectedPack` is set
  - `selectedPack` cleared on BMS subscribe/unsubscribe

## Commits

| Commit | Type | Description |
|--------|------|-------------|
| 1462082 | test | Add failing tests for pack selection and data retrieval |
| 494f2b9 | feat | Implement pack selection handler and write-settle-read cycle |

## Deviations from Plan

None - plan executed exactly as written.

## Verification

```
go test ./internal/hub/... -run "TestHandleSelectPack|TestPackData|TestPackError|TestEncodePackQueryInHandler" -v -count=1 -- PASS (4/4)
go test ./... -count=1 -- ALL PASS
go build ./... -- SUCCESS
```

All acceptance criteria verified:
- MsgTypeSelectPack/PackData/PackError constants in message.go
- PackDataMessage, PackGroup, PackErrorMessage structs in message.go
- triggerPackRead, handleSelectPack, buildPackDataMessage functions in hub.go
- 0x9020 write register call in hub.go
- selectedPack auto-refresh state tracking in hub.go
- MsgTypeSelectPack case in handleCommand

## Self-Check: PASSED

All files exist, all commits verified.
