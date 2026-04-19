---
phase: 27-hub-test-optimization
plan: 03
subsystem: hub-tests
tags: [t-parallel, race-detection, test-optimization, data-race-fix]
dependency_graph:
  requires: [synctest-wrapped-hub-tests]
  provides: [parallel-hub-tests, race-free-hub]
  affects: [internal/hub/hub.go, internal/hub/hub_test.go]
tech_stack:
  added: []
  patterns: [t-parallel-before-synctest, done-channel-shutdown]
key_files:
  created: []
  modified:
    - internal/hub/hub.go
    - internal/hub/hub_test.go
decisions:
  - Fixed Hub.RunFunc/ClientCount data race by adding h.done channel instead of relying on h.ctx.Done() which raced with Run() writing h.ctx
  - All 72 tests marked parallel (race analysis confirmed zero shared mutable state between tests)
metrics:
  duration: 2m52s
  completed: 2026-04-19
  tasks: 1/1
  files: 2
---

# Phase 27 Plan 03: Race Analysis and t.Parallel() Summary

Race analysis with -race -shuffle=on found 5 data races in Hub.RunFunc/ClientCount (h.ctx read/write race), fixed via dedicated done channel; added t.Parallel() to all 72 hub tests with zero exclusions.

## Task Results

| Task | Name | Commit | Key Changes |
|------|------|--------|-------------|
| 1 | Race analysis and t.Parallel() addition | be6d30a | Fix RunFunc data race, add t.Parallel() to 72 tests |

## Changes Made

### Race Analysis (D-05)

Ran `go test -race -shuffle=on -count=1 -timeout 300s ./internal/hub/...` and found 5 failing tests:

- TestOtherSectionsUnaffectedByReadOnce
- TestGroupedSectionRegistered
- TestHubRegisterUnregister
- TestStatusSectionRemoved
- TestConfigurationSectionRegistered

All 5 had the same root cause: a data race between `Hub.RunFunc()` reading `h.ctx.Done()` (line 154) and `Hub.Run()` writing `h.ctx` (line 166). The `RunFunc` and `ClientCount` methods use `h.ctx.Done()` as a shutdown signal in select statements, but `Run()` replaces `h.ctx` with the caller-provided context on startup, creating a concurrent read/write.

### Data Race Fix (Rule 1 - Bug)

Added a `done chan struct{}` field to the Hub struct:
- Initialized in `NewHub()` via `make(chan struct{})`
- Closed in `shutdown()` as first operation
- `RunFunc()` and `ClientCount()` now select on `h.done` instead of `h.ctx.Done()`

This eliminates the race because `h.done` is created once and only closed once, while `h.ctx` is still used within the Run() goroutine (single-threaded, no race).

### t.Parallel() Addition (D-06)

Added `t.Parallel()` as the first line inside every `func Test*(t *testing.T)` function, before the `synctest.Test()` call. This is the verified-safe position (calling t.Parallel inside a synctest bubble panics with "deadlock: main bubble goroutine").

All 72 tests received t.Parallel() -- no exclusions needed because:
- No package-level mutable state (`grep '^var ' hub_test.go` returns only test helpers, no shared mutable vars)
- Each test creates its own mockBroker + Hub instance
- mockBroker uses sync.Mutex for thread safety
- Race analysis after the fix confirmed zero races

### Performance Results

| Metric | Plan 02 (sequential) | Plan 03 (parallel) | With -race |
|--------|----------------------|---------------------|------------|
| Wall-clock time | 0.04s | 0.033s | 1.063s |
| Test count | 72 | 72 | 72 |
| Races detected | N/A | 0 | 0 |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed data race in Hub.RunFunc and ClientCount**
- **Found during:** Task 1, Step 1 (race analysis)
- **Issue:** Hub.RunFunc() and ClientCount() read h.ctx.Done() in select statements. Hub.Run() writes h.ctx on startup. Concurrent access from test goroutines triggers data race.
- **Fix:** Added `done chan struct{}` field to Hub. RunFunc/ClientCount select on h.done instead of h.ctx.Done(). h.done is closed in shutdown().
- **Files modified:** internal/hub/hub.go
- **Commit:** be6d30a

## Known Stubs

None.

## Self-Check: PASSED

- [x] internal/hub/hub.go exists and is modified
- [x] internal/hub/hub_test.go exists and is modified
- [x] Commit be6d30a exists
- [x] `grep -c 't.Parallel()' hub_test.go` = 72
- [x] `go test -race -shuffle=on ./internal/hub/...` exits 0
- [x] `go test ./internal/hub/...` exits 0, duration 0.033s (under 60s)
- [x] `go test ./...` exits 0 (full project green)
- [x] `go vet ./...` exits 0
