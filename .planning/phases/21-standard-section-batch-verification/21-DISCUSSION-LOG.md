# Phase 21: Standard Section Batch Verification - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-15
**Phase:** 21-standard-section-batch-verification
**Areas discussed:** Verification method, Failure handling, Pass criteria

---

## Verification Method

| Option | Description | Selected |
|--------|-------------|----------|
| Section sweep tool | Build standalone tool (like tools/config-sweep/) that connects to hardware, reads every section's batch spans, reports pass/fail per span | ✓ |
| Web UI + log inspection | Load each section in browser and check server logs for fallback warnings | |
| You decide | Claude picks the approach | |

**User's choice:** Section sweep tool
**Notes:** Follows established pattern from Phase 20

---

| Option | Description | Selected |
|--------|-------------|----------|
| All sections, one run | Single invocation sweeps all 7 sections | ✓ |
| Per-section flag | Run with -section flag to test one at a time | |

**User's choice:** All sections, one run

---

| Option | Description | Selected |
|--------|-------------|----------|
| Batch spans only | Just read each batch span and report success/failure | ✓ |
| Batch + individual comparison | Read span as batch, then re-read individually, compare values | |

**User's choice:** Batch spans only

---

## Failure Handling

| Option | Description | Selected |
|--------|-------------|----------|
| Remove failing registers | Same approach as Phase 20 — delete unsupported probes from source | ✓ |
| Keep and rely on fallback | Leave all registers, accept fallback handling and log noise | |
| Remove + document | Remove AND add source comments documenting removed V1.38 registers | |

**User's choice:** Remove failing registers

---

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, remove empty groups | Delete ProbeGroup definitions with zero working probes | ✓ |
| Keep empty groups | Leave group definition with no probes | |

**User's choice:** Yes, remove empty groups

---

## Pass Criteria

| Option | Description | Selected |
|--------|-------------|----------|
| JSON results file | Structured JSON with per-section, per-span pass/fail | ✓ |
| Console summary only | Print pass/fail to stdout with markers | |
| You decide | Claude picks output format | |

**User's choice:** JSON results file

---

| Option | Description | Selected |
|--------|-------------|----------|
| Zero span failures | Section passes only if every batch span reads with no fallback | |
| Zero register failures after fallback | Allow span failures as long as individual fallback reads succeed | ✓ |

**User's choice:** Zero register failures after fallback
**Notes:** More pragmatic — acknowledges hardware quirks that break batch reads of specific address ranges even when individual reads work fine.

---

## Claude's Discretion

- Sweep tool internal structure and naming
- Whether to include timing data in JSON output
- PV channel count for sweep

## Deferred Ideas

None — discussion stayed within phase scope.
