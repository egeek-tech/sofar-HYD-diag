---
phase: 18-batch-read-infrastructure
plan: "01"
subsystem: register
tags: [batch-read, contiguity-analysis, pure-function, tdd]
dependency_graph:
  requires: []
  provides: [BatchPlan, BatchSpan, ProbeMapping, MaxBatchRegisters, AnalyzeBatchPlan]
  affects: [internal/register]
tech_stack:
  added: []
  patterns: [contiguity-analysis, overlap-handling, address-sorted-merge]
key_files:
  created:
    - internal/register/batch.go
    - internal/register/batch_test.go
  modified: []
decisions:
  - "Overlap handling: probes starting inside an existing span are absorbed without extending TotalCount unless they extend beyond span end"
  - "Capacity hint: pre-count total probes for allocation-efficient flattening"
metrics:
  duration: "5m"
  completed: "2026-04-14T20:44:00Z"
  tasks_completed: 3
  tasks_total: 3
---

# Phase 18 Plan 01: Batch Read Analysis Engine Summary

Batch contiguity analysis engine as a pure function in the register package -- merges contiguous probes across group boundaries into BatchSpans with pre-computed byte offsets, enforcing the 60-register Modbus limit.

## Tasks Completed

| # | Task | Commit | Key Changes |
|---|------|--------|-------------|
| 1 | RED: Write failing tests for AnalyzeBatchPlan | 33f0ca4 | 12 tests covering contiguous, gap, cross-group, multi-register, ASCII, max-regs, synthetic, overlap, real section data |
| 2 | GREEN: Implement BatchPlan types and AnalyzeBatchPlan | 036a4b6 | ProbeMapping, BatchSpan, BatchPlan types; AnalyzeBatchPlan pure function with sort, merge, split |
| 3 | REFACTOR: Capacity hint and full suite verification | 694c719 | Pre-calculated allocation capacity for flattening slice |

## Implementation Details

**Types exported:**
- `ProbeMapping` -- maps probe to byte position within batch response (ByteOffset, ByteLength, GroupName)
- `BatchSpan` -- contiguous register range readable in one Modbus request (StartAddr, TotalCount, Probes)
- `BatchPlan` -- complete batch read strategy (Spans, Unbatchable)
- `MaxBatchRegisters = 60` -- Sofar protocol limit constant

**Algorithm:** Flatten all probes from all groups, filter synthetic (Count=0) to Unbatchable, sort by address, walk building spans with three cases:
1. **Contiguous** (probe.Addr == span end): extend span, check 60-reg limit
2. **Gap** (probe.Addr > span end): close span, start new
3. **Overlap** (probe.Addr < span end): absorb probe, extend only if it reaches beyond current end

**Real data verification:** SystemGroups produces exactly 8 spans + 1 unbatchable. The large merged span (0x0445, 31 regs) correctly handles the HW version probe (0x044D) overlapping within the Inverter SN range (0x0445-0x044E).

## Test Coverage

12 tests covering:
- Contiguous merging (basic 2-probe case)
- Gap detection (non-adjacent addresses)
- Cross-group batching (D-01: General + PCC Power merge)
- Multi-register U32 probes (Count=2, ByteLength=4)
- ASCII multi-register probes (Count=10, ByteOffset=20 for next probe)
- 60-register limit splitting (SAFE-02: 61 probes -> 60+1 spans)
- Synthetic probe filtering (Count=0 -> Unbatchable)
- Mixed synthetic and real probes
- Empty input edge case
- Real SystemGroups data (8 spans, 1 unbatchable)
- ByteOffset/ByteLength computation (D-08)
- GroupName preservation across cross-group merges

## TDD Gate Compliance

- RED gate: `test(18-01)` commit 33f0ca4 -- all tests fail with compilation error (batch.go absent)
- GREEN gate: `feat(18-01)` commit 036a4b6 -- all 12 tests pass
- REFACTOR gate: `refactor(18-01)` commit 694c719 -- capacity hint, all tests still pass

## Decisions Made

1. **Overlap handling strategy:** When a sorted probe's address falls inside the current span (e.g., HW version 0x044D inside Inverter SN 0x0445-0x044E), absorb it with correct ByteOffset. Only extend TotalCount if the overlapping probe extends beyond the current span end. This correctly handles the real SystemGroups layout.

2. **No fmt/log imports:** The register package remains a pure data package with no I/O dependencies, consistent with the codebase convention.

## Deviations from Plan

None -- plan executed exactly as written.

## Self-Check: PASSED

All files exist. All commits verified in git log.
