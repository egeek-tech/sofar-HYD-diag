# Roadmap: Sofar HYD Diagnostic Tool

## Milestones

- ✅ **v1.0** -- Phases 1-5 (shipped 2026-04-11) -- [Archive](milestones/v1.0-ROADMAP.md)
- ✅ **v1.1 UX Polish & Battery Pack Fix** -- Phases 6-7 (shipped 2026-04-11) -- [Archive](milestones/v1.1-ROADMAP.md)
- ✅ **v1.2 Reliability & UX Refinements** -- Phases 8-11 (shipped 2026-04-13) -- [Archive](milestones/v1.2-ROADMAP.md)
- 🚧 **v1.3 Data Cleanup & Configuration** -- Phases 12-17 (in progress)

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

### 🚧 v1.3 Data Cleanup & Configuration (In Progress)

**Milestone Goal:** Clean up data display issues, consolidate sections, add device configuration readout, and expand register coverage from XLSX register map.

- [x] **Phase 12: Dead Code Cleanup & Test Infrastructure** - Remove deprecated code and establish testify for new development (completed 2026-04-13)
- [x] **Phase 13: Statistics-to-System Merge** - Consolidate statistics into System section with additional daily metrics (completed 2026-04-13)
- [x] **Phase 14: System Time Fix** - Display system time as a single concatenated row (completed 2026-04-13)
- [x] **Phase 15: Configuration Section** - New read-only section displaying device configuration registers (completed 2026-04-13)
- [ ] **Phase 16: Frontend Polish** - Fix pack drill-down tooltips and hide disconnected temperature sensors
- [ ] **Phase 17: XLSX Register Discovery** - Offline tool to parse XLSX register map and integrate newly discovered registers

## Phase Details

### Phase 12: Dead Code Cleanup & Test Infrastructure
**Goal**: Codebase is clean of deprecated code and new Go packages use testify assertions
**Depends on**: Phase 11 (v1.2 complete)
**Requirements**: QUAL-01, QUAL-02
**Success Criteria** (what must be TRUE):
  1. The deprecated triggerPackRead function and all related dead code (~250 lines) no longer exist in the codebase
  2. Running `go build ./...` succeeds with no compilation errors after removal
  3. Running `go test ./...` passes with testify (assert/require) used in all test files for new or modified packages
**Plans:** 2/2 plans complete
Plans:
- [x] 12-01-PLAN.md -- Remove deprecated triggerPackRead chain and dead test helper from hub.go
- [x] 12-02-PLAN.md -- Convert all test files to testify assertions and strengthen with edge cases

### Phase 13: Statistics-to-System Merge
**Goal**: Users see all statistics data within the System section with no separate Statistics sidebar entry
**Depends on**: Phase 12
**Requirements**: SECT-01, CLEAN-01, SECT-03
**Success Criteria** (what must be TRUE):
  1. The Statistics sidebar button and separate Statistics section no longer exist in the UI
  2. Daily and total statistics (generation, consumption, bought, sold, battery charge/discharge) appear as groups within the System section
  3. Empty "This Month" and "This Year" groups do not appear anywhere in the UI
  4. Additional daily statistics (battery charge today, PV power today) appear alongside existing statistics in the System section
  5. Navigating to the System section streams all statistics registers as part of the normal System read cycle
**Plans:** 1/1 plans complete
Plans:
- [x] 13-01-PLAN.md -- Merge statistics groups into SystemGroups, remove stats section from hub and frontend

### Phase 14: System Time Fix
**Goal**: System time displays as a human-readable single value instead of raw register parts
**Depends on**: Phase 13 (both modify SystemGroups)
**Requirements**: CLEAN-02
**Success Criteria** (what must be TRUE):
  1. System time appears as a single concatenated row (e.g., "10:59:34 13-04-2026") in the System section
  2. The separate year/month/day/hour/minute/second register rows no longer appear individually
**Plans:** 1/1 plans complete
Plans:
- [x] 14-01-PLAN.md -- Consolidate 6 time registers into single composed row with batch read and range tooltip

### Phase 15: Configuration Section
**Goal**: Users can view all device configuration parameters in a new read-only Configuration section
**Depends on**: Phase 12 (clean codebase, testify available)
**Requirements**: SECT-02
**Success Criteria** (what must be TRUE):
  1. A "Configuration" entry appears in the sidebar navigation
  2. Clicking Configuration streams device config registers organized into logical groups (System, Battery, Function, Safety, Reactive Power, etc.)
  3. All configuration values are read-only with no write controls exposed
  4. Configuration registers display with proper units, scaling, and enum decoding matching the V1.38 protocol spec
**Plans:** 3/3 plans complete
Plans:
- [x] 15-01-PLAN.md -- Define configuration register groups and enum maps from V1.38 protocol spec
- [x] 15-02-PLAN.md -- Hub integration with read-once caching and section registration
- [x] 15-03-PLAN.md -- Frontend sidebar button, error suppression, group hiding, and visual verification
**UI hint**: yes

### Phase 16: Frontend Polish
**Goal**: Pack drill-down shows complete tooltip coverage and hides noise from disconnected sensors
**Depends on**: Phase 12 (clean codebase)
**Requirements**: TIP-01, TIP-02, CLEAN-03
**Success Criteria** (what must be TRUE):
  1. Hovering over any Balance State value in the pack drill-down shows a tooltip with register address (hex) and raw value
  2. Hovering over any Pack Status value in the pack drill-down shows a tooltip with register address (hex) and raw value
  3. Zero-value temperatures (0.0C) in the pack drill-down are hidden or visually dimmed as disconnected sensors
**Plans:** 2 plans
Plans:
- [ ] 16-01-PLAN.md -- Tooltip coverage for Balance State and Pack Status, zero-temp hiding, PackInfoProbes error suppression
- [ ] 16-02-PLAN.md -- Per-group batch streaming for pack drill-down register values
**UI hint**: yes

### Phase 17: XLSX Register Discovery
**Goal**: Offline CLI tool discovers registers from XLSX and valuable new registers are integrated into the web UI
**Depends on**: Phase 12 (clean codebase)
**Requirements**: REG-01, REG-02
**Success Criteria** (what must be TRUE):
  1. A standalone CLI tool (build-tagged out of the production binary) parses the XLSX register map and outputs a comparison against the PDF V1.38 register definitions
  2. The tool identifies registers present in XLSX but missing from the current probe definitions
  3. Meter registers (0x7080+) and any other valuable newly discovered registers are added to the appropriate sections in the web UI
  4. The production binary size is unchanged by the XLSX tool (excelize/v2 excluded via build tag)
**Plans**: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 12 -> 13 -> 14 -> 15 -> 16 -> 17
Note: Phases 15, 16, and 17 depend only on Phase 12, not on each other. They are ordered by size (largest first) but could be reordered after Phase 14 completes.

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. Foundation and Modbus Service | v1.0 | 3/3 | Done | 2026-04-10 |
| 2. WebSocket Hub, API, and Connection UI | v1.0 | 3/3 | Done | 2026-04-10 |
| 3. Core Monitoring Sections | v1.0 | 3/3 | Done | 2026-04-10 |
| 4. Battery Overview and Statistics | v1.0 | 4/4 | Done | 2026-04-11 |
| 5. Deep Battery Pack Diagnostics | v1.0 | 3/3 | Done | 2026-04-11 |
| 6. Battery Pack Access Fix | v1.1 | 3/3 | Done | 2026-04-11 |
| 7. Streaming Display and Configurable Timing | v1.1 | 3/3 | Done | 2026-04-11 |
| 8. Refresh Architecture | v1.2 | 2/2 | Done | 2026-04-12 |
| 9. Connection & Read Resilience | v1.2 | 2/2 | Done | 2026-04-12 |
| 10. Data Persistence & Tooltips | v1.2 | 3/3 | Done | 2026-04-13 |
| 11. Battery Pack Polish | v1.2 | 2/2 | Done | 2026-04-13 |
| 12. Dead Code Cleanup & Test Infrastructure | v1.3 | 2/2 | Complete    | 2026-04-13 |
| 13. Statistics-to-System Merge | v1.3 | 1/1 | Complete    | 2026-04-13 |
| 14. System Time Fix | v1.3 | 1/1 | Complete    | 2026-04-13 |
| 15. Configuration Section | v1.3 | 3/3 | Complete    | 2026-04-13 |
| 16. Frontend Polish | v1.3 | 0/2 | Planned     | - |
| 17. XLSX Register Discovery | v1.3 | 0/0 | Not started | - |
