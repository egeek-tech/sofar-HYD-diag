---
phase: 18-batch-read-infrastructure
plan: 02
subsystem: hub
tags: [batch-read, degradation, span-tracker, section]
dependency_graph:
  requires: [register.BatchPlan from 18-01]
  provides: [SpanTracker, Section.BatchPlan, GetSectionBatchPlan]
  affects: [internal/hub, internal/register]
tech_stack:
  added: []
  patterns: [state-machine, structured-logging, hub-event-loop]
key_files:
  created:
    - internal/hub/batch.go
    - internal/hub/batch_test.go
    - internal/register/batch.go
  modified:
    - internal/hub/section.go
    - internal/hub/hub_streaming.go
    - internal/hub/export_test.go
decisions:
  - SpanTracker is not goroutine-safe by design, matching Section's single-goroutine access pattern
  - Created register.BatchPlan stub for parallel wave compilation; 18-01 provides full implementation
metrics:
  duration: 4m 43s
  completed: "2026-04-14T20:44:40Z"
  tasks: 2
  files: 6
---

# Phase 18 Plan 02: SpanTracker and Section BatchPlan Summary

SpanTracker state machine for batch span degradation/recovery plus BatchPlan field on Section struct for pre-computed batch read plans.

## What Was Done

### Task 1: SpanTracker Implementation and Tests

Created `internal/hub/batch.go` with the `SpanTracker` type that tracks consecutive batch failures per span start address. After `DefaultDegradationThreshold` (3) consecutive failures, a span is marked degraded and should fall back to individual register reads. A single successful read resets the failure counter and clears degradation. Structured slog logging emits WARN on degradation and INFO on recovery, both with `0x%04X` formatted span addresses.

Created `internal/hub/batch_test.go` with 8 comprehensive test cases covering:
- New tracker returns non-nil, unknown spans not degraded
- Single failure does not trigger degradation
- Degradation at exact threshold (3 failures)
- Recovery on success resets counter
- Success on fresh tracker is safe (no panic)
- Independent spans do not interfere
- Additional failures after degradation keep span degraded
- Re-degradation after recovery requires 3 new failures

**Commit:** `1f7f1f4`

### Task 2: BatchPlan Field on Section

Added `BatchPlan register.BatchPlan` field to the `Section` struct in `section.go`. The field is populated at construction time via `register.AnalyzeBatchPlan(groups)` in `newGroupedSection`. Non-grouped sections (created via `newSection`) correctly have a zero-value BatchPlan since they have no Groups.

Modified `hub_streaming.go` battery auto-detect section to recompute `sec.BatchPlan = register.AnalyzeBatchPlan(newGroups)` alongside the existing `sec.Groups` and `sec.Probes` reassignment.

Added `GetSectionBatchPlan` test helper in `export_test.go` following the established pattern (routes through hub event loop for thread safety).

Created `internal/register/batch.go` with stub types (`BatchPlan`, `BatchSpan`, `ProbeMapping`, `MaxBatchRegisters`) and a working `AnalyzeBatchPlan` implementation to enable compilation in this parallel worktree. Plan 18-01 provides the authoritative implementation; the orchestrator merge will resolve the overlap.

**Commit:** `25a45d3`

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Created register.BatchPlan stub for parallel compilation**
- **Found during:** Task 2
- **Issue:** Plan 18-02 references `register.BatchPlan` and `register.AnalyzeBatchPlan` which are created by plan 18-01, running in parallel (same wave). Without these types, Task 2 code would not compile.
- **Fix:** Created `internal/register/batch.go` with the required types and a working AnalyzeBatchPlan function. The orchestrator merge will resolve overlap with 18-01's authoritative implementation.
- **Files created:** internal/register/batch.go
- **Commit:** 25a45d3

## Verification Results

- `go test ./internal/hub/... -run TestSpanTracker -count=1`: 8/8 PASS
- `go test ./... -count=1`: All packages PASS (no regressions)
- `go vet ./internal/hub/...`: Clean

## Decisions Made

1. **SpanTracker single-goroutine design**: Matches Section's existing access pattern (no mutex, accessed only from hub event loop goroutine). Consistent with the codebase convention.
2. **Stub types for parallel wave**: Created working register.BatchPlan stub rather than leaving code uncompilable. The orchestrator merge handles file conflicts between worktrees.

## Self-Check: PASSED

All 6 files verified present. Both task commits (1f7f1f4, 25a45d3) found in git log. SUMMARY.md exists.
