---
phase: 21-standard-section-batch-verification
plan: 01
subsystem: tools
tags: [modbus, batch-verification, sweep-tool, cli]
dependency_graph:
  requires: [internal/register/batch.go, internal/modbus/tcp.go, internal/register/system.go, internal/register/pv.go, internal/register/meter.go, internal/register/dcdc.go, internal/register/pcu.go, internal/register/bdu.go]
  provides: [tools/section-sweep/main.go]
  affects: []
tech_stack:
  added: []
  patterns: [batch-span-fallback, 3-way-error-classification, structured-json-output]
key_files:
  created: [tools/section-sweep/main.go]
  modified: []
decisions:
  - "Followed config-sweep pattern for CLI flags, connection management, and error classification"
  - "Connection variable reassigned on reconnect rather than using defer conn.Close() at top level"
  - "Reconnect on batch failure only for non-0x02 errors (0x02 is data address issue, not connection)"
metrics:
  duration: "177s"
  completed: "2026-04-15T21:05:40Z"
  tasks_completed: 1
  tasks_total: 1
  files_created: 1
  files_modified: 0
---

# Phase 21 Plan 01: Section Sweep Tool Summary

Standalone CLI tool that connects to inverter hardware and verifies every batch span for all 7 standard sections (Grid, EPS, PV, Meter, DCDC, PCU, BDU), outputting structured JSON with per-span pass/fail and per-probe fallback results.

## Task Execution

### Task 1: Build section-sweep standalone tool
- **Status:** Complete
- **Commit:** `f415646`
- **Files created:** `tools/section-sweep/main.go` (254 lines)
- **Verification:** `go build -tags section_sweep` compiles clean, `go vet` reports no issues, full test suite passes

## Deviations from Plan

None - plan executed exactly as written.

## Key Implementation Details

- Build tag `//go:build section_sweep` isolates the tool from normal builds
- All 7 sections defined: grid, eps, pv, meter, dcdc, pcu, bdu
- Batch span reads via `register.AnalyzeBatchPlan()` with per-span fallback to individual probes
- 3-way error classification matches config-sweep: PASS (nil), FAIL (0x02 illegal data address), ERROR (transient, reconnect)
- 500ms delay after every Modbus read (batch and individual)
- JSON output to stdout, progress to stderr (follows config-sweep convention)
- Connection reconnection on transient errors with fatal exit if reconnect fails
- Explicit `conn.Close()` at end instead of defer (conn reassigned on reconnects)

## Verification Results

| Check | Result |
|-------|--------|
| `go build -tags section_sweep` | PASS |
| `go vet -tags section_sweep` | PASS |
| `go test ./... -count=1` | PASS (all packages) |
| Build tag present | PASS |
| All 7 sections referenced | PASS |
| Batch read with fallback | PASS |
| 500ms delay enforcement | PASS |
| JSON structured output | PASS |

## Known Stubs

None - tool is complete and ready for hardware execution.

## Self-Check: PASSED
