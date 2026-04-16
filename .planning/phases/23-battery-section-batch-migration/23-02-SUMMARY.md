---
phase: 23-battery-section-batch-migration
plan: 02
subsystem: hub
tags: [testing, battery, batch-read, integration]
dependency_graph:
  requires: [23-01]
  provides: [BATT-01-test, BATT-02-test, BATT-03-test]
  affects: [internal/hub/hub_test.go, internal/hub/export_test.go]
tech_stack:
  added: []
  patterns: [raw-json-drain-until-complete, exported-test-alias]
key_files:
  created: []
  modified:
    - internal/hub/hub_test.go
    - internal/hub/export_test.go
decisions:
  - Used raw JSON parsing for output equivalence test since OutboundMessage lacks Name/Value fields
  - Added drain-until-section_complete loop to avoid 10s timeout in OutputEquivalence test
  - Exported countBatteryChannels via export_test.go alias for external test package access
metrics:
  duration: 516s
  completed: "2026-04-16T19:58:10Z"
  tasks_completed: 2
  tasks_total: 2
  files_modified: 2
---

# Phase 23 Plan 02: Battery Batch Read Integration Tests Summary

Integration tests verifying battery batch span reads, channel auto-detection with reconfiguration, output equivalence to expected probe set, span fallback, and countBatteryChannels correctness.

## Completed Tasks

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Add battery batch read test helpers and setupBatterySpanTest | cfb0633 | internal/hub/hub_test.go, internal/hub/export_test.go |
| 2 | Integration tests for BATT-01, BATT-02, BATT-03 | 7e6ad89 | internal/hub/hub_test.go |

## Test Coverage

| Test | Requirement | What It Verifies |
|------|-------------|------------------|
| TestCountBatteryChannels | BATT-02 | Correct channel counting with 2-ch, 4-ch, InternalInfo, nil |
| TestBatteryBatchRead_SpanReads | BATT-01 | All 30 probes emit register_value via 7 batch spans |
| TestBatteryBatchRead_AutoDetect | BATT-02 | 0x066A=4 triggers reconfiguration to 4 channels; InternalInfo preserved |
| TestBatteryBatchRead_OutputEquivalence | BATT-03 | Exact register name match between expected groups and output messages; all values non-empty |
| TestBatteryBatchRead_SpanFallback | BATT-01 | Individual fallback produces all register values when first span fails |

## Decisions Made

1. **Raw JSON parsing for output equivalence**: OutboundMessage struct lacks Name/Value fields needed for BATT-03 verification. Used raw JSON unmarshalling into map[string]interface{} with a drain-until-section_complete loop to avoid the 10s timeout that drainRawMessages would incur.

2. **Export alias for countBatteryChannels**: Test file uses `package hub_test` (external test package), so added `var CountBatteryChannels = countBatteryChannels` in export_test.go.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed OutputEquivalence test timeout**
- **Found during:** Task 2 verification
- **Issue:** Using `drainRawMessages(send, 10*time.Second)` waited the full 10 seconds because it doesn't know when to stop. Test took 10.1s instead of 0.1s.
- **Fix:** Replaced with custom drain loop that breaks on `section_complete` message type, bringing test time to 0.1s.
- **Files modified:** internal/hub/hub_test.go
- **Commit:** 7e6ad89

## Pre-existing Test Failures

The following tests fail on the base commit and are NOT caused by this plan's changes:
- TestBMSTowerBitmapPartialOnline (timeout -- documented in plan)
- TestHubRegisterUnregister (data race -- documented in plan)
- TestGroupedSectionRegistered (data race -- documented in plan)
- TestStatusSectionRemoved (data race -- documented in plan)
- TestPackSkipUnsupported (pack skip not triggering -- discovered during verification)
- TestPackSkipResetOnSwitch (pack skip not triggering -- discovered during verification)

## Known Stubs

None. All tests exercise real code paths with deterministic mock data.

## Self-Check: PASSED
