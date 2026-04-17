---
phase: 25-pack-drill-down-batch-migration
plan: 02
subsystem: hub/test, register/test
tags: [batch-read, pack-drill-down, integration-tests, span-tracker]
dependency_graph:
  requires: [25-01 streamPackBatchRead, 25-01 packSpanTracker, 25-01 export_test helpers]
  provides: [pack batch integration test suite, pack batch plan span unit test]
  affects: [internal/hub/hub_test.go, internal/register/batch_test.go]
tech_stack:
  added: []
  patterns: [setupPackBatchSpanTest shared helper, waitForMessageType for async message collection]
key_files:
  created: []
  modified:
    - internal/hub/hub_test.go
    - internal/register/batch_test.go
decisions:
  - "Used existing waitForMessageType helper instead of creating collectMessagesUntilComplete (avoids duplicate helper)"
  - "Added BMS subscription before select_pack in all tests (required for hub message routing to client)"
  - "Used collectRawMessages + read_cycle loop pattern from TestPackSpanDegradation for degradation test"
metrics:
  duration: 10m
  completed: "2026-04-17T20:41:27Z"
  tasks_completed: 2
  tasks_total: 2
  files_modified: 2
---

# Phase 25 Plan 02: Pack Batch Integration Tests Summary

Integration tests verifying pack batch migration correctness: span reads produce register_value messages (PACK-01), 0x9020 write with correct query word encoding (PACK-02), and SpanTracker degrades 0x9104+ spans after repeated failures (PACK-03), plus a unit test confirming AnalyzeBatchPlan produces the expected span structure for PackProbeGroups.

## What Changed

### Task 1: Add setupPackBatchSpanTest helper and pack batch integration tests (6eb27a8)

**hub_test.go additions (164 lines):**

- `setupPackBatchSpanTest` helper: creates mockBroker configured for pack batch span reads with all spans returning known value 100. Follows setupBMSBatchSpanTest pattern but simplified (no special-case probes like bitmap/topology).

- `TestPackBatchRead_SpanReads`: subscribes to BMS, triggers select_pack(1,1,1), collects messages until section_complete, verifies register_value count is at least the number of probes in the first two supported spans.

- `TestPackBatchRead_WriteAndSettle`: subscribes to BMS, triggers select_pack(1,2,3), waits for section_complete, asserts first WriteRegister call targets 0x9020 with the correct EncodePackQuery(1,2,3,TopoTowers) value.

- `TestPackBatchRead_SpanDegradation`: subscribes to BMS, finds the span starting at 0x9104+, configures it to fail at both batch and individual levels, runs select_pack + 2 read_cycles (3 total failures), verifies GetPackSpanState reports non-Normal state.

### Task 2: Add TestPackBatchPlanSpans unit test (ba41bd2)

**batch_test.go additions (65 lines):**

- `TestPackBatchPlanSpans`: calls AnalyzeBatchPlan(PackProbeGroups()) and verifies:
  - First span starts at 0x9044
  - A span covers 0x90BC (Temps 5-8)
  - A span covers 0x9104 (Pack Info block 2)
  - A span covers 0x9124 (Pack Status 2)
  - Total probe count across all spans matches PackProbeGroups total

- Added testify `assert` and `require` imports to batch_test.go (previously stdlib-only)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Missing BMS subscription before select_pack**
- **Found during:** Task 1 initial test run
- **Issue:** Plan's test code omitted BMS subscription; hub only routes messages to subscribed clients for a section, so select_pack messages were processed but results never reached the test client channel
- **Fix:** Added `h.Command(c, hub.InboundMessage{Type: hub.MsgTypeSubscribe, Section: "bms"})` and `drainRawMessages(send, 2*time.Second)` before each select_pack call in all 3 tests
- **Files modified:** internal/hub/hub_test.go
- **Commit:** 6eb27a8

**2. [Rule 1 - Bug] SpanDegradation test used waitForMessageType instead of collectRawMessages**
- **Found during:** Task 1 adaptation
- **Issue:** The first select_pack call needed collectRawMessages (timeout-based) rather than waitForMessageType for the read_cycle loop pattern to work correctly with the 100ms sleep between cycles
- **Fix:** Used collectRawMessages for the initial select_pack and read_cycle iterations, matching the pattern from existing TestPackSpanDegradation
- **Files modified:** internal/hub/hub_test.go
- **Commit:** 6eb27a8

## Verification Results

1. `go test ./internal/hub/ -run "TestPackBatchRead_SpanReads" -count=1 -timeout 30s` -- PASS (3.2s)
2. `go test ./internal/hub/ -run "TestPackBatchRead_WriteAndSettle" -count=1 -timeout 30s` -- PASS (3.2s)
3. `go test ./internal/hub/ -run "TestPackBatchRead_SpanDegradation" -count=1 -timeout 60s` -- PASS (17.5s)
4. `go test ./internal/register/ -run "TestPackBatchPlanSpans" -count=1 -timeout 30s` -- PASS (0.002s)
5. `go test ./... -count=1 -timeout 300s` -- all 5 packages pass (hub: 164s, note: default 120s timeout insufficient for full hub suite due to cumulative pack test settle delays)

## Self-Check: PASSED
