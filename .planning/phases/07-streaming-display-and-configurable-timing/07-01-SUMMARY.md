---
phase: 07-streaming-display-and-configurable-timing
plan: 01
subsystem: hub, broker
tags: [streaming, message-types, timing, broker-command, test-stubs]
dependency_graph:
  requires: []
  provides:
    - "RegisterValueMessage, SectionCompleteMessage, SectionSchemaMessage types"
    - "TimingConfigPayload struct and InboundMessage.TimingConfig field"
    - "CmdSetDelay broker command with SetDelayRuntime method"
    - "BrokerInterface extended with ReadRegisters and SetDelayRuntime"
    - "Wave 0 test stubs for STREAM-01, STREAM-02, TIMING-01, TIMING-02"
  affects:
    - "07-02 (streaming read implementation uses these types)"
    - "07-03 (frontend uses these message types)"
tech_stack:
  added: []
  patterns:
    - "Command-channel pattern for runtime delay update (CmdSetDelay)"
    - "Constructor functions for streaming message types"
key_files:
  created: []
  modified:
    - internal/hub/message.go
    - internal/broker/broker.go
    - internal/hub/broker_iface.go
    - internal/hub/hub_test.go
decisions:
  - "ReadRegisters mock delegates to ReadBatch with single read for simplicity"
  - "SetDelayRuntime uses command channel pattern matching existing broker commands"
metrics:
  duration: "3m 46s"
  completed: "2026-04-11"
  tasks: 4
  files: 4
---

# Phase 07 Plan 01: Streaming Types, Broker Commands, and Test Stubs Summary

**One-liner:** WebSocket message contracts for per-register streaming (register_value, section_complete, section_schema), runtime delay command via broker command channel, and Wave 0 test stubs for all phase requirements.

## Task Results

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Add streaming message types and timing config payload | aa63f5e | internal/hub/message.go |
| 2 | Add CmdSetDelay to broker and extend BrokerInterface | dc5fe13 | internal/broker/broker.go, internal/hub/broker_iface.go |
| 3 | Update mock broker with ReadRegisters and SetDelayRuntime | 4cdf4ce | internal/hub/hub_test.go |
| 4 | Create Wave 0 test stubs for phase requirements | e2326cc | internal/hub/hub_test.go |

## What Was Built

### Message Types (Task 1)
- Three new message type constants: `MsgTypeRegisterValue`, `MsgTypeSectionComplete`, `MsgTypeSectionSchema`
- `RegisterValueMessage` struct: carries per-register results during streaming (section, group, name, value, error)
- `SectionCompleteMessage` struct: signals all registers in a section have been read (with UTC timestamp)
- `SectionSchemaMessage` and `SchemaGroup` structs: describe section layout for frontend pre-rendering
- `TimingConfigPayload` struct with `ReadDelayMs` and `PackSettleMs` fields
- `InboundMessage.TimingConfig` field for timing configuration messages
- Constructor functions: `NewRegisterValue`, `NewSectionComplete`, `NewSectionSchema`

### Broker Command (Task 2)
- `CmdSetDelay` command type added to broker's command type enum
- `SetDelayRequest` struct carries the new delay duration
- `SetDelayRuntime` method on `*Broker` sends delay update through command channel (avoids data races)
- `CmdSetDelay` case in `execute()` dispatcher updates `interReadDelay` field safely
- `BrokerInterface` extended with `ReadRegisters(ctx, addr, count)` and `SetDelayRuntime(ctx, d)`

### Mock and Tests (Tasks 3-4)
- `mockBroker` updated with `lastDelay` field and both new interface methods
- `ReadRegisters` mock delegates to `ReadBatch` with single-element read request
- Four Wave 0 test stubs: `TestStreamingRead`, `TestSectionSchema`, `TestTimingConfigure`, `TestPackSettleConfigure`
- All stubs use `t.Skip` and include comments describing expected behavior for Plan 02

## Deviations from Plan

None -- plan executed exactly as written.

## Verification

- `go build ./...` exits 0 (all packages compile)
- `go test ./... -count=1` exits 0 (all tests pass)
- `TestBrokerSatisfiesInterface` passes (confirms `*broker.Broker` satisfies extended `BrokerInterface`)
- All four Wave 0 stubs show SKIP status in test output

## Self-Check: PASSED

All 5 files found. All 4 task commits found.
