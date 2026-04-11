# Roadmap: Sofar HYD Diagnostic Tool

## Milestones

- ✅ **v1.0** — Phases 1-5 (shipped 2026-04-11) — [Archive](milestones/v1.0-ROADMAP.md)
- 🚧 **v1.1 UX Polish & Battery Pack Fix** — Phases 6-7 (in progress)

## Phases

<details>
<summary>✅ v1.0 (Phases 1-5) — SHIPPED 2026-04-11</summary>

- [x] Phase 1: Foundation and Modbus Service (3/3 plans)
- [x] Phase 2: WebSocket Hub, API, and Connection UI (3/3 plans)
- [x] Phase 3: Core Monitoring Sections (3/3 plans)
- [x] Phase 4: Battery Overview and Statistics (4/4 plans)
- [x] Phase 5: Deep Battery Pack Diagnostics (3/3 plans)

</details>

### 🚧 v1.1 UX Polish & Battery Pack Fix (In Progress)

**Milestone Goal:** Fix battery pack access so all 20 packs are reachable (matching proven CLI tool), stream parameters in real-time as they arrive, and give the user control over Modbus timing.

- [ ] **Phase 6: Battery Pack Access Fix** - Fix pack encoding and topology so all 20 packs are accessible
- [ ] **Phase 7: Streaming Display and Configurable Timing** - Stream each parameter as it arrives and let user control Modbus read delays

## Phase Details

### Phase 6: Battery Pack Access Fix
**Goal**: All 20 battery packs are accessible for drill-down, matching the proven CLI tool behavior
**Depends on**: Phase 5
**Requirements**: PACK-01, PACK-02, PACK-03
**Success Criteria** (what must be TRUE):
  1. User can navigate to any of the 20 battery packs (2 towers x 10 packs) and see cell-level data
  2. Pack selection writes the correct tower/pack encoding to 0x9020, matching the proven CLI tool
  3. Topology is fixed at 16 cells/pack, 10 packs/tower, 2 towers -- no configuration dropdowns for these values
  4. Online bitmap correctly reflects all packs that the inverter reports as available
**Plans:** 3 plans

Plans:
- [x] 06-01-PLAN.md — Hardcode topology constants, simplify Hub/NewHub, reduce cell probes to 16
- [x] 06-02-PLAN.md — Implement per-tower bitmap cycling in triggerBMSRead
- [x] 06-03-PLAN.md — Remove frontend topology dropdowns, hardcode JS constants, simplify bitmap click

### Phase 7: Streaming Display and Configurable Timing
**Goal**: Users see each parameter value appear immediately as it is read, and can tune Modbus timing to match their hardware
**Depends on**: Phase 6
**Requirements**: STREAM-01, STREAM-02, TIMING-01, TIMING-02
**Success Criteria** (what must be TRUE):
  1. Each parameter value appears in the UI as soon as it is read from the inverter, not after the entire section batch completes
  2. While a section is loading, already-read parameters show their values and remaining parameters show a loading indicator
  3. User can adjust the default Modbus inter-read delay via a UI control (default 500ms)
  4. Battery pack reads use a separate, longer settle delay after the 0x9020 write (configurable, default 1-2s)
  5. Changed timing settings take effect on the next read cycle without requiring reconnection
**Plans:** 3 plans

Plans:
- [ ] 07-01-PLAN.md — Add streaming message types, broker CmdSetDelay, extend BrokerInterface
- [ ] 07-02-PLAN.md — Replace batch reads with per-register streaming, add timing config handler
- [ ] 07-03-PLAN.md — Frontend skeleton rendering, streaming handlers, timing controls UI

## Progress

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. Foundation and Modbus Service | v1.0 | 3/3 | Done | 2026-04-10 |
| 2. WebSocket Hub, API, and Connection UI | v1.0 | 3/3 | Done | 2026-04-10 |
| 3. Core Monitoring Sections | v1.0 | 3/3 | Done | 2026-04-10 |
| 4. Battery Overview and Statistics | v1.0 | 4/4 | Done | 2026-04-11 |
| 5. Deep Battery Pack Diagnostics | v1.0 | 3/3 | Done | 2026-04-11 |
| 6. Battery Pack Access Fix | v1.1 | 0/3 | Planning | - |
| 7. Streaming Display and Configurable Timing | v1.1 | 0/3 | Planning | - |
