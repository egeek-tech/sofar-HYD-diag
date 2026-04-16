---
phase: 23-battery-section-batch-migration
plan: 01
subsystem: hub-streaming
tags: [batch-reading, battery, refactor, span-tracker]
dependency_graph:
  requires: []
  provides: [streamBatteryBatchRead, readSpanIndividualFallback, countBatteryChannels]
  affects: [hub.go-routing, hub_streaming.go]
tech_stack:
  added: []
  patterns: [hybrid-batch-read-with-pre-read-probe, shared-individual-fallback-helper]
key_files:
  created: []
  modified:
    - internal/hub/hub_streaming.go
    - internal/hub/hub.go
decisions:
  - "Extracted readSpanIndividualFallback as shared helper used by both streamStandardRead and streamBatteryBatchRead"
  - "countBatteryChannels uses string prefix matching instead of fragile len-1 arithmetic"
  - "Reconfiguration path re-appends InternalInfoGroups and resets SpanTracker (fixes 2 existing bugs)"
metrics:
  duration_seconds: 308
  completed: "2026-04-16T19:45:38Z"
  tasks_completed: 2
  tasks_total: 2
  files_modified: 2
---

# Phase 23 Plan 01: Battery Section Batch Migration Summary

Migrated battery section from 30 individual Modbus reads to ~8 batch span transactions via streamBatteryBatchRead hybrid function with 0x066A pre-read auto-detection and SpanTracker three-state degradation.

## Completed Tasks

| Task | Name | Commit | Key Changes |
|------|------|--------|-------------|
| 1 | Extract readSpanIndividualFallback helper and add countBatteryChannels | 6dd89d2 | DRY refactor: 3 duplicated inline loops in streamStandardRead replaced with helper calls; countBatteryChannels added |
| 2 | Create streamBatteryBatchRead and update routing | affe914 | New hybrid batch function with pre-read 0x066A, SpanTracker degradation, reconfiguration; routing updated; old streamBatteryRead removed |

## What Was Built

### readSpanIndividualFallback (Task 1)
Shared helper method that reads each probe in a span individually, emitting sectionResult per probe. Returns true if all reads failed (for SpanTracker escalation). Replaced 3 identical 15-line blocks in streamStandardRead with single-line calls.

### countBatteryChannels (Task 1)
Package-private function counting groups with "Channel " name prefix. Replaces fragile `len(groups) - 1` arithmetic that was off-by-one after Phase 17-03 added InternalInfoGroups.

### streamBatteryBatchRead (Task 2)
Hybrid batch reading function for battery section:
- **Pre-read 0x066A** individually before batch spans for channel auto-detection
- **Channel change handling**: rebuilds Groups/Probes/BatchPlan with InternalInfoGroups re-appended, resets SpanTracker, re-triggers read
- **Batch span loop**: identical three-state SpanTracker degradation pattern as streamStandardRead
- **Pitfall handling**: pre-read failure is non-fatal (log + continue), pre-read result not emitted as sectionResult (batch loop emits it)

### Routing Update (Task 2)
hub.go switch case changed from `h.streamBatteryRead(sec, readCtx)` to `h.streamBatteryBatchRead(sec, readCtx)`.

### Old Function Removal (Task 2)
`streamBatteryRead` completely removed. No references remain in codebase.

## Deviations from Plan

None - plan executed exactly as written.

## Bug Fixes (Existing Issues Resolved)

**1. InternalInfoGroups dropped on reconfiguration (Pitfall 1)**
- Old code: `newGroups := register.GenerateBatteryGroups(detected)` (missing InternalInfoGroups)
- New code: `newGroups := append(register.GenerateBatteryGroups(detected), register.InternalInfoGroups()...)`
- Impact: Internal Info registers (0x06CC-0x06ED) now survive channel count reconfiguration

**2. Channel count off-by-one with InternalInfoGroups (Pitfall 2)**
- Old code: `currentChannels := len(groups) - 1` (returned 3 instead of 2 with InternalInfo)
- New code: `currentChannels := countBatteryChannels(sec.Groups)` (explicit prefix matching)
- Impact: Battery section no longer reconfigures unnecessarily every read cycle

## Verification Results

- `go build ./...` -- PASS
- `go vet ./...` -- PASS
- `go test ./internal/register/` -- PASS (all register tests)
- `go test ./internal/hub/` -- PASS (all tests except pre-existing known failures: TestBMSTowerBitmapPartialOnline timeout, data races in TestHubRegisterUnregister/TestGroupedSectionRegistered/TestStatusSectionRemoved)
- SpanTracker integration tests (Degradation, Skipped, ProbeRecovery, ResetOnReconnect) -- all PASS
- TestBatchSpanFallback, TestBatchProgressiveStreaming -- PASS
- No references to old `streamBatteryRead` remain in codebase

## Self-Check: PASSED

All files exist, all commits verified:
- 6dd89d2: refactor(23-01): extract readSpanIndividualFallback helper and add countBatteryChannels
- affe914: feat(23-01): create streamBatteryBatchRead and update routing
