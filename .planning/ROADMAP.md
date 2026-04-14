# Roadmap: Sofar HYD Diagnostic Tool

## Milestones

- ✅ **v1.0** -- Phases 1-5 (shipped 2026-04-11) -- [Archive](milestones/v1.0-ROADMAP.md)
- ✅ **v1.1 UX Polish & Battery Pack Fix** -- Phases 6-7 (shipped 2026-04-11) -- [Archive](milestones/v1.1-ROADMAP.md)
- ✅ **v1.2 Reliability & UX Refinements** -- Phases 8-11 (shipped 2026-04-13) -- [Archive](milestones/v1.2-ROADMAP.md)
- ✅ **v1.3 Data Cleanup & Configuration** -- Phases 12-17 (shipped 2026-04-14) -- [Archive](milestones/v1.3-ROADMAP.md)
- 🚧 **v1.4 Batch Register Reading** -- Phases 18-19 (in progress)

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

### v1.4 Batch Register Reading

- [ ] **Phase 18: Batch Read Infrastructure** - Analyze probe groups for contiguous ranges and build batch read engine with safety constraints
- [ ] **Phase 19: System & Configuration Batch Application** - Apply batch reading to System and Configuration sections, measure speedup, verify streaming preserved

## Phase Details

### Phase 18: Batch Read Infrastructure
**Goal**: The broker/hub layer can batch contiguous registers into single Modbus requests with automatic fallback on failure
**Depends on**: Phase 17 (v1.3 complete)
**Requirements**: BATCH-01, SAFE-02, SAFE-03
**Success Criteria** (what must be TRUE):
  1. ProbeGroups are analyzed to identify contiguous address ranges that can be batched into single Modbus requests
  2. A batch read of up to 60 contiguous registers returns the same data as individual register reads
  3. Batch reads do not cross protocol block boundaries (e.g., 0x04xx vs 0x05xx register areas)
  4. When a batch read fails, the system falls back to reading each register individually and no data is lost
**Plans**: TBD

### Phase 19: System & Configuration Batch Application
**Goal**: System and Configuration sections load measurably faster with batch reading while preserving progressive UI updates
**Depends on**: Phase 18
**Requirements**: BATCH-02, BATCH-03, BATCH-04
**Success Criteria** (what must be TRUE):
  1. System section completes a full refresh cycle in measurably less time than per-register reads (target 3-5x improvement)
  2. Configuration section's one-time load completes faster with batch reading
  3. Values still appear progressively in the UI as each batch completes -- not all-at-once after a long wait
  4. All register values displayed match what individual reads would return (no data corruption from batching)
**Plans**: TBD

## Progress

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 18. Batch Read Infrastructure | 0/TBD | Not started | - |
| 19. System & Configuration Batch Application | 0/TBD | Not started | - |
