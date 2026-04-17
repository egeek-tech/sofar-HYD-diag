---
phase: 24-bms-batch-migration
plan: 02
subsystem: hub
tags: [modbus, batch-read, bms, span-tracker, streaming, go]

# Dependency graph
requires:
  - phase: 24-01
    provides: "Composite probe definitions (bms_clock, bms_sw_version) in BMSInfoGroups"
  - phase: 22-spantracker-integration
    provides: "SpanTracker three-state degradation for batch span reads"
  - phase: 23-battery-section-batch-migration
    provides: "streamBatteryBatchRead pattern and setupBatterySpanTest test helper"
provides:
  - "readBatchSpans shared helper eliminating span loop duplication across standard, battery, BMS"
  - "readSpanIndividualFallbackAccum helper for individual fallback with result accumulation"
  - "streamBMSBatchRead hybrid function with BMS-specific post-processing"
  - "BMS batch read integration tests covering span reads, Composite formatting, protection decoding"
affects: [bms-pack-drilldown, future-batch-sections]

# Tech tracking
tech-stack:
  added: []
  patterns: ["readBatchSpans shared helper pattern for all batch section reads", "setupBMSBatchSpanTest with span-level mock responses"]

key-files:
  created: []
  modified:
    - "internal/hub/hub_streaming.go"
    - "internal/hub/hub.go"
    - "internal/hub/hub_test.go"

key-decisions:
  - "Extracted readBatchSpans as shared helper returning []broker.Result for post-processing"
  - "Created readSpanIndividualFallbackAccum to accumulate results during individual fallback reads"
  - "BMS protection probes identified by address match (isProtectionAddr) rather than separate function"
  - "Used raw JSON unmarshalling in CompositeValues test to access RegisterValueMessage Name field"

patterns-established:
  - "readBatchSpans: shared span iteration helper used by all three section readers (standard, battery, BMS)"
  - "setupBMSBatchSpanTest: test helper creating span-level mock data with known Composite values"

requirements-completed: [BMS-01, BMS-04]

# Metrics
duration: 7min
completed: 2026-04-17
---

# Phase 24 Plan 02: BMS Hub Streaming Batch Migration Summary

**readBatchSpans shared helper eliminates three-way span loop duplication; streamBMSBatchRead hybrid function reads BMS via 3 batch spans with bitmap/topology/protection post-processing**

## Performance

- **Duration:** 7 min
- **Started:** 2026-04-17T09:45:00Z
- **Completed:** 2026-04-17T09:52:00Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- Extracted readBatchSpans shared helper that replaces duplicated span iteration loops in streamStandardRead (reduced from ~130 lines to 3 lines), streamBatteryBatchRead (reduced from ~110 lines to 3 lines), and new streamBMSBatchRead
- Created streamBMSBatchRead hybrid function with BMS-specific post-processing: topology detection from 0x900D, tower bitmap widget from 0x9022, and protection group decoding from 6 protection registers
- Removed 277 net lines of dead code: streamBMSRead (190 lines) and buildBMSGroupData (98 lines)
- Added 5 BMS integration tests: bitmap (both online, partial online), span reads, Composite values, protection decoding

## Task Commits

Each task was committed atomically:

1. **Task 1: Extract readBatchSpans shared helper and create streamBMSBatchRead** - `7c77b01` (feat)
2. **Task 2: Add BMS batch read integration tests and update bitmap tests** - `0427c4d` (test)

## Files Created/Modified
- `internal/hub/hub_streaming.go` - Added readBatchSpans, readSpanIndividualFallbackAccum, streamBMSBatchRead; refactored streamStandardRead and streamBatteryBatchRead to use shared helper; removed streamBMSRead
- `internal/hub/hub.go` - Updated BMS routing to streamBMSBatchRead; removed buildBMSGroupData
- `internal/hub/hub_test.go` - Added setupBMSBatchSpanTest helper; updated TestBMSTowerBitmap/PartialOnline for batch spans; added TestBMSBatchRead_SpanReads, TestBMSBatchRead_CompositeValues, TestBMSBatchRead_ProtectionDecoding; removed dead makeBMSInfoResults/makeBMSInfoResultsWithBitmap

## Decisions Made
- Used address-based identification for protection probes (isProtectionAddr closure) rather than calling a separate function, keeping the logic self-contained in streamBMSBatchRead
- readBatchSpans returns []broker.Result aligned with sec.Probes indexing, enabling post-processing functions to correlate results with probes by index
- Used raw JSON unmarshalling in TestBMSBatchRead_CompositeValues to access the Name field from RegisterValueMessage, since OutboundMessage does not expose Name

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- BMS section fully migrated to batch span reads (3 Modbus transactions per cycle vs previous per-register reads)
- readBatchSpans shared helper available for any future section that needs batch reads with result accumulation
- All existing tests pass with no regressions across standard, battery, BMS, and SpanTracker tests

## Self-Check: PASSED

All files exist, all commits verified, all key functions present, all dead code removed.

---
*Phase: 24-bms-batch-migration*
*Completed: 2026-04-17*
