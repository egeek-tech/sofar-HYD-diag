---
phase: 20-configuration-register-cleanup
plan: 01
subsystem: tools
tags: [config-sweep, modbus, hardware-validation, standalone-tool]
dependency_graph:
  requires: [internal/register.ConfigurationGroups, internal/modbus.ReadHoldingRegistersTCP]
  provides: [tools/config-sweep standalone sweep tool, JSON pass/fail report format]
  affects: [20-02 probe removal decisions]
tech_stack:
  added: []
  patterns: [build-tag-isolated tool, JSON structured output, stderr progress / stdout data]
key_files:
  created:
    - tools/config-sweep/main.go
  modified: []
decisions:
  - Skipped synthetic probes (Count == 0) during sweep to avoid invalid Modbus reads
  - No retry on individual register failure -- record ERROR and continue to avoid infinite retry loops
  - Removed unused net import (Go compiler enforced; net.Conn type inferred from modbus.Connect return)
metrics:
  duration: 207s
  completed: "2026-04-15T11:21:31Z"
  tasks_completed: 1
  tasks_total: 1
  files_created: 1
  files_modified: 0
---

# Phase 20 Plan 01: Config Sweep Tool Summary

Standalone Go tool that sweeps every configuration register against real inverter hardware and produces a structured JSON pass/fail report, driving probe removal decisions in Plan 02.

## Task Results

### Task 1: Create config-sweep standalone tool
**Commit:** 88186a7
**Files created:** tools/config-sweep/main.go

Created `tools/config-sweep/main.go` with `//go:build config_sweep` build tag following the established xlsx-discover tool pattern. The tool:

- Parses CLI flags (`-host`, `-port`, `-slave`) with validation
- Connects to inverter via `modbus.Connect`
- Iterates all `register.ConfigurationGroups` probes individually
- Reads each register via `modbus.ReadHoldingRegistersTCP`
- Classifies responses: PASS (success), FAIL (0x02 illegal data address), ERROR (transient)
- Handles connection drops with automatic reconnect and continuation
- Enforces 500ms inter-read delay per hardware timing constraint
- Outputs structured JSON to stdout (redirectable to file)
- Prints progress and summary to stderr

## Verification Results

| Check | Result |
|-------|--------|
| `go build -tags config_sweep ./tools/config-sweep/` | PASS |
| `go build ./...` (main build unaffected) | PASS |
| `go test ./... -count=1` (no regressions) | PASS |
| File starts with `//go:build config_sweep` | PASS |
| Imports `sofar-hyd-diag/internal/modbus` | PASS |
| Imports `sofar-hyd-diag/internal/register` | PASS |
| Uses `register.ConfigurationGroups` | PASS |
| Uses `modbus.ReadHoldingRegistersTCP` | PASS |
| Checks `err=0x02` for illegal data address | PASS |
| Enforces `500 * time.Millisecond` delay | PASS |
| Uses `json.MarshalIndent` for output | PASS |
| CLI flags: `-host`, `-port`, `-slave` | PASS |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Removed unused `net` import**
- **Found during:** Task 1 build verification
- **Issue:** `net` was imported for a `var _ net.Conn` compile check that was unnecessary since `net.Conn` type is inferred from `modbus.Connect` return
- **Fix:** Removed the unused import
- **Files modified:** tools/config-sweep/main.go
- **Commit:** 88186a7

**2. [Rule 2 - Missing functionality] Skip synthetic probes with Count == 0**
- **Found during:** Task 1 implementation
- **Issue:** Configuration groups contain synthetic probes (Count: 0) used for composed display values -- reading these with Modbus would send an invalid request (quantity=0)
- **Fix:** Added `if p.Count == 0 { continue }` guard before each read
- **Files modified:** tools/config-sweep/main.go
- **Commit:** 88186a7
