# Roadmap: Sofar HYD Diagnostic Tool

## Milestones

- ✅ **v1.0** -- Phases 1-5 (shipped 2026-04-11) -- [Archive](milestones/v1.0-ROADMAP.md)
- ✅ **v1.1 UX Polish & Battery Pack Fix** -- Phases 6-7 (shipped 2026-04-11) -- [Archive](milestones/v1.1-ROADMAP.md)
- ✅ **v1.2 Reliability & UX Refinements** -- Phases 8-11 (shipped 2026-04-13) -- [Archive](milestones/v1.2-ROADMAP.md)
- ✅ **v1.3 Data Cleanup & Configuration** -- Phases 12-17 (shipped 2026-04-14) -- [Archive](milestones/v1.3-ROADMAP.md)
- ✅ **v1.4 Batch Register Reading** -- Phases 18-19 (shipped 2026-04-15) -- [Archive](milestones/v1.4-ROADMAP.md)
- 🚧 **v1.5 Full Batch Reading & Configuration Cleanup** -- Phases 20-25 (in progress)

## Phases

<details>
<summary>✅ v1.0 (Phases 1-5) -- SHIPPED 2026-04-11</summary>

- [x] Phase 1: Foundation and Modbus Service (3/3 plans)
- [x] Phase 2: WebSocket Hub, API, and Connection UI (3/3 plans)
- [x] Phase 3: Core Monitoring Sections (3/3 plans)
- [x] Phase 4: Battery Overview and Statistics (4/4 plans)
- [x] Phase 5: Deep Battery Pack Diagnostics (3/3 plans)

</details>

<details>
<summary>✅ v1.1 UX Polish & Battery Pack Fix (Phases 6-7) -- SHIPPED 2026-04-11</summary>

- [x] Phase 6: Battery Pack Access Fix (3/3 plans)
- [x] Phase 7: Streaming Display and Configurable Timing (3/3 plans)

</details>

<details>
<summary>✅ v1.2 Reliability & UX Refinements (Phases 8-11) -- SHIPPED 2026-04-13</summary>

- [x] Phase 8: Refresh Architecture (2/2 plans) -- completed 2026-04-12
- [x] Phase 9: Connection & Read Resilience (2/2 plans) -- completed 2026-04-12
- [x] Phase 10: Data Persistence & Tooltips (3/3 plans) -- completed 2026-04-13
- [x] Phase 11: Battery Pack Polish (2/2 plans) -- completed 2026-04-13

</details>

<details>
<summary>✅ v1.3 Data Cleanup & Configuration (Phases 12-17) -- SHIPPED 2026-04-14</summary>

- [x] Phase 12: Dead Code Cleanup & Test Infrastructure (2/2 plans)
- [x] Phase 13: Statistics-to-System Merge (1/1 plans)
- [x] Phase 14: System Time Fix (1/1 plans)
- [x] Phase 15: Configuration Section (3/3 plans)
- [x] Phase 16: Frontend Polish (2/2 plans)
- [x] Phase 17: XLSX Register Discovery (3/3 plans)

</details>

<details>
<summary>✅ v1.4 Batch Register Reading (Phases 18-19) -- SHIPPED 2026-04-15</summary>

- [x] Phase 18: Batch Read Infrastructure (2/2 plans) -- completed 2026-04-14
- [x] Phase 19: System & Configuration Batch Application (2/2 plans) -- completed 2026-04-15

</details>

### v1.5 Full Batch Reading & Configuration Cleanup (In Progress)

**Milestone Goal:** Extend batch reading to every section and clean up non-functional configuration registers for a fully optimized, error-free UI.

- [x] **Phase 20: Configuration Register Cleanup** - Remove unsupported config registers and empty groups (completed 2026-04-15)
- [x] **Phase 21: Standard Section Batch Verification** - Confirm existing batch behavior across all standard sections on real hardware (completed 2026-04-17)
- [x] **Phase 22: SpanTracker Integration** - Track and auto-skip persistently-failing spans (completed 2026-04-16)
- [x] **Phase 23: Battery Section Batch Migration** - Migrate battery section to BatchPlan span reading (completed 2026-04-16)
- [x] **Phase 24: BMS Batch Migration** - Migrate BMS section to BatchPlan with Composite probes for composed values (completed 2026-04-17)
- [x] **Phase 25: Pack Drill-Down Batch Migration** - Migrate pack drill-down to BatchPlan span reading (completed 2026-04-17)
- [x] **Phase 26: Milestone Cleanup** - Fix PV BatchPlan staleness, remove dead code, clean up orphaned UI/enum artifacts (completed 2026-04-17)

## Phase Details

### Phase 20: Configuration Register Cleanup
**Goal**: Configuration section loads cleanly without errors from unsupported registers
**Depends on**: Nothing (first phase in v1.5)
**Requirements**: CONF-01, CONF-02, CONF-03
**Success Criteria** (what must be TRUE):
  1. Configuration section loads on real hardware with zero fallback warnings in server logs
  2. Registers that return illegal data address (0x83/0x02) are excluded from configuration probe definitions
  3. Configuration groups that contained only unsupported registers are no longer visible in the sidebar or UI
**Plans**: 2 plans
Plans:
- [x] 20-01-PLAN.md -- Build config-sweep standalone tool for hardware register testing
- [x] 20-02-PLAN.md -- Run sweep on hardware, remove failing probes and empty groups, update tests

### Phase 21: Standard Section Batch Verification
**Goal**: All standard sections confirmed working via batch spans on real hardware
**Depends on**: Phase 20
**Requirements**: BVER-01, BVER-02, BVER-03, BVER-04, BVER-05, BVER-06, BVER-07
**Success Criteria** (what must be TRUE):
  1. Grid section loads via batch spans on real hardware and displays all register values without errors
  2. EPS section loads via batch spans on real hardware and displays all register values without errors
  3. PV, Meter, DCDC, PCU, and BDU sections each load via batch spans on real hardware without errors
  4. No section shows fallback-to-individual-read behavior in server logs during normal operation
  5. All section values match expected ranges for the connected inverter
**Plans**: 2 plans
Plans:
- [x] 21-01-PLAN.md -- Build section-sweep standalone tool for batch span verification across all 7 sections
- [x] 21-02-PLAN.md -- Run sweep on hardware, remove failing registers and empty groups, update tests

### Phase 22: SpanTracker Integration
**Goal**: Persistently-failing spans are automatically detected and skipped on subsequent reads
**Depends on**: Phase 21
**Requirements**: RESIL-01, RESIL-02
**Success Criteria** (what must be TRUE):
  1. SpanTracker is wired into streamStandardRead and records which spans fail
  2. A span that fails on consecutive reads is automatically skipped on the next read cycle
  3. Skipped spans do not cause UI errors or blank values -- previously cached values persist (dimmed)
**Plans**: 2 plans
Plans:
- [x] 22-01-PLAN.md -- Extend SpanTracker to three-state degradation model with unit tests
- [x] 22-02-PLAN.md -- Wire SpanTracker into streamStandardRead, Section, reconnect handler with integration tests

### Phase 23: Battery Section Batch Migration
**Goal**: Battery section reads all registers via batch spans instead of individual reads
**Depends on**: Phase 22
**Requirements**: BATT-01, BATT-02, BATT-03
**Success Criteria** (what must be TRUE):
  1. Battery section uses BatchPlan spans for all register reads instead of per-register individual reads
  2. Battery channel auto-detection (0x066A read to determine active channels) still works correctly after migration
  3. Battery section UI displays identical information to pre-migration behavior (same values, same groups, same formatting)
**Plans**: 2 plans
Plans:
- [x] 23-01-PLAN.md -- Extract shared fallback helper, create streamBatteryBatchRead with pre-read + batch spans, update routing
- [x] 23-02-PLAN.md -- Integration tests for battery batch read: span reads, auto-detection, output equivalence, fallback

### Phase 24: BMS Batch Migration
**Goal**: BMS section reads all registers via batch spans with Composite probes for composed values
**Depends on**: Phase 23
**Requirements**: BMS-01, BMS-02, BMS-03, BMS-04
**Success Criteria** (what must be TRUE):
  1. BMS section uses BatchPlan spans for all register reads instead of per-register individual reads
  2. BMS clock composition (0x9004 + 0x9005 merged into HH:MM:SS DD-MM-YYYY) renders correctly
  3. BMS SW version composition (registers 0x9018-0x901B merged into version string) renders correctly
  4. BMS bitmap decoding and protection post-processing produce identical output to pre-migration behavior
**Plans**: 2 plans
Plans:
- [x] 24-01-PLAN.md -- Add bms_clock and bms_sw_version Composite probes, update BMSInfoGroups with protection, register tests
- [x] 24-02-PLAN.md -- Extract readBatchSpans shared helper, create streamBMSBatchRead, remove dead code, integration tests

### Phase 25: Pack Drill-Down Batch Migration
**Goal**: Pack drill-down reads registers via batch spans for dramatic speedup
**Depends on**: Phase 24
**Requirements**: PACK-01, PACK-02, PACK-03, PACK-04
**Success Criteria** (what must be TRUE):
  1. Pack drill-down uses BatchPlan spans for all register reads instead of per-register individual reads
  2. Pack selection write (0x9020) and 1-second settle delay are preserved before any batch read begins
  3. Known unsupported PackInfo registers (0x9104+) are excluded from batch spans and do not cause errors
  4. Pack drill-down UI displays identical information to pre-migration behavior (cell voltages, temperatures, balance state, fault/alarm/protection decoding)
**Plans**: 2 plans
Plans:
- [x] 25-01-PLAN.md -- Create streamPackBatchRead hybrid function, update Hub struct and routing, remove dead code, fix frontend D-07 duplication bug
- [x] 25-02-PLAN.md -- Add pack batch integration tests (span reads, write+settle, degradation) and batch plan unit test

### Phase 26: Milestone Cleanup
**Goal**: Fix pre-existing PV BatchPlan staleness bug, remove dead code and orphaned artifacts
**Depends on**: Phase 25
**Requirements**: CLEAN-01, CLEAN-02, CLEAN-03, CLEAN-04
**Success Criteria** (what must be TRUE):
  1. PV section `handleConfigure` recomputes `sec.BatchPlan` after channel change — batch spans match new channel count
  2. Dead `readSpanIndividualFallback` function (non-Accum variant) removed from hub_streaming.go
  3. DCDC nav button removed from index.html (section was deregistered in Phase 21)
  4. Orphaned config enum maps (CommunicationInterruptEnum, FanNoiseEnum, IPAllocationEnum, EMSTimePeriodModeEnum) removed from config_enum.go
**Plans**: 1 plan
Plans:
- [x] 26-01-PLAN.md -- Fix PV BatchPlan staleness bug, remove dead function/nav button/orphaned enums

## Progress

**Execution Order:**
Phases execute in numeric order: 20 -> 21 -> 22 -> 23 -> 24 -> 25 -> 26

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 20. Configuration Register Cleanup | v1.5 | 2/2 | Complete    | 2026-04-15 |
| 21. Standard Section Batch Verification | v1.5 | 2/2 | Complete    | 2026-04-17 |
| 22. SpanTracker Integration | v1.5 | 2/2 | Complete    | 2026-04-16 |
| 23. Battery Section Batch Migration | v1.5 | 2/2 | Complete    | 2026-04-16 |
| 24. BMS Batch Migration | v1.5 | 2/2 | Complete    | 2026-04-17 |
| 25. Pack Drill-Down Batch Migration | v1.5 | 2/2 | Complete    | 2026-04-17 |
| 26. Milestone Cleanup | v1.5 | 1/1 | Complete   | 2026-04-17 |
