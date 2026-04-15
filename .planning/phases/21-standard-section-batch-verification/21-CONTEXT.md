# Phase 21: Standard Section Batch Verification - Context

**Gathered:** 2026-04-15
**Status:** Ready for planning

<domain>
## Phase Boundary

All standard sections (Grid, EPS, PV, Meter, DCDC, PCU, BDU) confirmed working via batch spans on real hardware. Registers that fail are removed from source. The batch infrastructure already exists — this phase verifies it works and cleans up what doesn't.

</domain>

<decisions>
## Implementation Decisions

### Verification Method
- **D-01:** Standalone section-sweep tool (following the `tools/config-sweep/` pattern) that connects to hardware, reads every section's batch spans, and reports pass/fail per span. Reusable for future hardware changes.
- **D-02:** Single invocation sweeps all 7 sections (Grid, EPS, PV, Meter, DCDC, PCU, BDU). No per-section flag needed.
- **D-03:** Batch spans only — no individual-vs-batch comparison reads. Individual reads are already proven from v1.0-v1.3.

### Failure Handling
- **D-04:** Remove failing registers from source (same approach as Phase 20). Static removal — no runtime skip-list. Single-inverter diagnostic tool with fixed hardware.
- **D-05:** Remove entire ProbeGroup definitions when all their probes are unsupported, keeping sidebar clean (same as Phase 20).

### Pass Criteria
- **D-06:** Structured JSON results file (like `tools/config-sweep/results.json`) with per-section, per-span pass/fail data. Machine-readable, diffable against future sweeps.
- **D-07:** A section passes when zero individual register failures occur after fallback. Batch span failures are acceptable if fallback individual reads all succeed. This acknowledges hardware quirks that can break batch reads of specific address ranges.

### Claude's Discretion
- Sweep tool internal structure and naming (following config-sweep conventions)
- Whether to include timing data in the JSON output
- PV channel count to use during sweep (can use default 2 or detect from hardware)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Batch Reading Infrastructure
- `internal/register/batch.go` — BatchPlan/BatchSpan/ProbeMapping types and AnalyzeBatchPlan algorithm
- `internal/hub/hub_streaming.go:36` — streamStandardRead with batch span iteration and per-span fallback
- `internal/hub/hub.go:636-650` — registerBuiltinSections showing all section registrations

### Section Register Definitions
- `internal/register/system.go` — GridGroups, EPSGroups definitions
- `internal/register/pv.go` — GeneratePVGroups function
- `internal/register/meter.go` — MeterGroups definition
- `internal/register/dcdc.go` — DCDCGroups definition
- `internal/register/pcu.go` — PCUGroups definition
- `internal/register/bdu.go` — BDUGroups definition

### Prior Sweep Tool
- `tools/config-sweep/` — Established standalone tool pattern for hardware register verification (Phase 20)

### Protocol Specification
- `Sofar_Inverter_MODBUS_V1.38_EN.pdf` — Register addresses and data types for all sections

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `tools/config-sweep/` — Standalone sweep tool pattern; new tool can follow same structure
- `internal/register/batch.go:AnalyzeBatchPlan()` — Pure function to compute spans from probe groups; sweep tool can import and use directly
- `internal/broker/broker.go` — Modbus communication layer the sweep tool can reuse

### Established Patterns
- ProbeGroup/Probe struct pattern in `internal/register/` — removal is deleting struct literals from slices
- `RegisterGroupedSection()` in hub.go — no changes needed; removing probes automatically fixes batch plans
- `register_test.go` — Test assertions on group counts will need updating after probe removal

### Integration Points
- `hub.go:638-650` registers all standard sections — no code change needed (reads from exported group variables)
- Frontend sidebar renders from schema message — removing groups from source removes them from UI automatically
- Batch plan auto-computes from probe definitions — removing probes fixes spans automatically

</code_context>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

### Reviewed Todos (not folded)
- **Read delay burst on section switch** — hub timing issue, not batch verification (separate concern)
- **Skip unsupported PackInfoProbes (0x9104-0x9126)** — BMS-specific, covered by Phase 25
- **Stream pack drill-down values per-register** — pack drill-down specific, covered by Phase 25

</deferred>

---

*Phase: 21-standard-section-batch-verification*
*Context gathered: 2026-04-15*
