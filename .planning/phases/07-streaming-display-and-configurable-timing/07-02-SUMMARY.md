---
phase: 07-streaming-display-and-configurable-timing
plan: 02
subsystem: hub
tags: [streaming, timing, websocket, modbus]
dependency_graph:
  requires: [07-01]
  provides: [streaming-reads, timing-config, section-schema]
  affects: [frontend-display, pack-reads]
tech_stack:
  added: []
  patterns: [per-register-streaming, stream-reads-batch-computation, schema-on-subscribe]
key_files:
  created:
    - internal/hub/hub_streaming.go
  modified:
    - internal/hub/hub.go
    - internal/hub/export_test.go
    - internal/hub/hub_test.go
decisions:
  - "sectionResult.msg changed to interface{} for polymorphic streaming messages (RegisterValueMessage, SectionCompleteMessage, OutboundMessage)"
  - "broadcastResultToSection added alongside existing broadcastToSection for backward compatibility with pack data"
  - "BMS protection registers still read via ReadBatch (small batch, 6 registers) rather than individual streaming"
  - "System fault registers still read via ReadBatch then sent as section_data with Faults field"
  - "Mock broker extended with registerResults map for per-address streaming test control"
metrics:
  duration: "20 minutes"
  completed: "2026-04-11T20:41:00Z"
  tasks: 2
  files: 4
---

# Phase 07 Plan 02: Streaming Display and Configurable Timing Summary

Per-register streaming reads with ReadRegisters, runtime timing configuration, and section schema on subscribe.

## What Was Done

### Task 1: Timing Config and Hub Structural Changes

Added `readDelayMs` (default 500, clamped 100-5000) and `packSettleMs` (default 1000, clamped 500-10000) fields to the Hub struct. Extended `handleConfigure` with a `"timing"` case that validates, clamps, and applies timing changes. The broker's inter-read delay is updated via `SetDelayRuntime` through a goroutine (thread-safe command channel pattern). The hardcoded `time.Sleep(1 * time.Second)` and `time.Sleep(2 * time.Second)` in `triggerPackRead` were replaced with `time.Duration(h.packSettleMs)` and `time.Duration(h.packSettleMs*2)` respectively.

The `sectionResult.msg` field was changed from `OutboundMessage` to `interface{}` to support polymorphic streaming messages. A new `broadcastResultToSection` method handles JSON marshalling of any message type. The `subscribeClient` method now sends a `section_schema` message before triggering the first read, enabling frontend skeleton pre-rendering.

The `triggerSectionRead` dispatch was updated to call `streamStandardRead`, `streamBMSRead`, and `streamBatteryRead` instead of the old batch methods.

### Task 2: Streaming Read Methods and Schema Builder

Created `internal/hub/hub_streaming.go` with four methods:

- **`buildSectionSchema`**: Builds a `SectionSchemaMessage` from a section's ProbeGroups, listing all register names per group with layout hints.

- **`streamStandardRead`**: Replaces `triggerStandardRead`. Reads each probe individually via `ReadRegisters` and sends a `register_value` message immediately. System time registers (6 components) are collected and composed into a single "System time" value. Fault registers for the system section are still batch-read and sent as a `section_data` message with the Faults field.

- **`streamBatteryRead`**: Replaces `triggerBatteryRead`. Streams individual register values while collecting results for the auto-detect channel count logic (0x066A). Preserves the section rebuild-and-re-read path when channel count changes.

- **`streamBMSRead`**: Replaces `triggerBMSRead`. Streams individual BMS info registers while collecting results for post-processing (Pitfall 6: "stream the reads, batch the computation"). Composition registers (BMS clock 0x9004+0x9005, SW version 0x9018-0x901B, topology 0x900D) are collected and composed after the group. Bitmap and protection data are sent as a batched `section_data` message at the end.

The old `triggerStandardRead`, `triggerBMSRead`, and `triggerBatteryRead` methods were removed from `hub.go`. The `buildBMSGroupData`, `buildGroupedResult`, and `buildProtectionGroup` helper methods were retained for use by remaining batch paths (pack data, protection).

Updated 15 existing hub tests to work with the streaming model: tests now check for `section_schema` as the first subscribe message, look for `register_value` messages instead of grouped `section_data`, and use `section_complete` to verify read cycle completion.

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1+2 | 16ae31a | feat(07-02): add timing config, streaming read methods, and schema-on-subscribe |
| 2 | ad58276 | test(07-02): update hub tests for streaming read model |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Updated 15 existing hub tests for streaming model**
- **Found during:** Task 2 verification
- **Issue:** Existing tests expected `section_data` as first message after subscribe, but streaming model sends `section_schema` first, then `register_value` messages instead of batched `section_data`
- **Fix:** Updated all affected tests to drain streaming messages, find correct message types, and use new test helpers (`drainRawMessages`, `waitForMessageType`, `drainUntilComplete`). Added `registerResults` map to mock broker for per-address streaming result control.
- **Files modified:** internal/hub/hub_test.go
- **Commit:** ad58276

## Verification

```
go build ./... -- exits 0
go test ./... -count=1 -- all packages pass
```

All 35 hub tests pass (4 Wave 0 stubs skipped as expected).

## Self-Check: PASSED
