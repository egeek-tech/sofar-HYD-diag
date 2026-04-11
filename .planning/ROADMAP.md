# Roadmap: Sofar HYD Diagnostic Tool

## Milestones

- ✅ **v1.0** -- Phases 1-5 (shipped 2026-04-11) -- [Archive](milestones/v1.0-ROADMAP.md)
- ✅ **v1.1 UX Polish & Battery Pack Fix** -- Phases 6-7 (shipped 2026-04-11) -- [Archive](milestones/v1.1-ROADMAP.md)
- 🚧 **v1.2 Reliability & UX Refinements** -- Phases 8-11 (in progress)

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

### 🚧 v1.2 Reliability & UX Refinements (In Progress)

**Milestone Goal:** Fix auto-refresh architecture, improve read reliability, and polish the diagnostic UI with better feedback and control.

- [ ] **Phase 8: Refresh Architecture** - Move auto-refresh to browser-only trigger with consistent timing enforcement
- [ ] **Phase 9: Connection & Read Resilience** - Immediate disconnect and automatic register read retry
- [ ] **Phase 10: Data Persistence & Tooltips** - Stale value display, section caching, and parameter tooltips
- [ ] **Phase 11: Battery Pack Polish** - Pack drill-down streaming and section reorder

## Phase Details

### Phase 8: Refresh Architecture
**Goal**: Auto-refresh is driven entirely by the browser with consistent, predictable Modbus timing
**Depends on**: Phase 7
**Requirements**: REL-03, REFR-01, REFR-02
**Success Criteria** (what must be TRUE):
  1. Switching between sections does not cause a burst of rapid Modbus reads -- the inter-read delay is consistent regardless of navigation
  2. The backend performs no autonomous refresh cycles -- all reads are initiated by the browser
  3. After a read cycle completes, the browser waits for the configured delay before starting the next cycle (no fixed-interval timer)
  4. Stopping auto-refresh in the browser immediately stops all Modbus reads (no orphaned backend timer continues reading)
**Plans**: TBD

### Phase 9: Connection & Read Resilience
**Goal**: Users experience immediate disconnect response and transparent error recovery during reads
**Depends on**: Phase 8
**Requirements**: REL-01, REL-02
**Success Criteria** (what must be TRUE):
  1. Clicking disconnect while a read cycle is in progress aborts the current Modbus read and closes the connection within 1 second
  2. The UI transitions to disconnected state immediately after clicking disconnect, even if reads were in progress
  3. A single register that returns an error is automatically retried up to 3 times before the error is shown to the user
  4. Registers that succeed on retry display their value normally -- the user never sees the transient error
**Plans**: TBD

### Phase 10: Data Persistence & Tooltips
**Goal**: Users always see the most recent known values and can inspect register-level details on demand
**Depends on**: Phase 9
**Requirements**: DISP-01, DISP-02, DISP-03
**Success Criteria** (what must be TRUE):
  1. When a new refresh cycle begins, previously read values remain visible (dimmed/faded) until replaced by fresh values
  2. Navigating away from a section and back shows the last-read values (dimmed) immediately, without waiting for a new read cycle
  3. Hovering over any parameter value shows a tooltip displaying the Modbus register address (hex) and the raw register value
**Plans**: TBD
**UI hint**: yes

### Phase 11: Battery Pack Polish
**Goal**: Pack drill-down displays data consistently with other sections and presents information in logical order
**Depends on**: Phase 8
**Requirements**: BATT-01, BATT-02
**Success Criteria** (what must be TRUE):
  1. In the pack drill-down view, the balance state section appears before the temperature section
  2. Pack drill-down values stream per-register as they are read, with each value appearing in the UI as soon as it arrives (consistent with System, Grid, and other sections)
**Plans**: TBD
**UI hint**: yes

## Progress

**Execution Order:**
Phases execute in numeric order: 8 -> 9 -> 10 -> 11

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. Foundation and Modbus Service | v1.0 | 3/3 | Done | 2026-04-10 |
| 2. WebSocket Hub, API, and Connection UI | v1.0 | 3/3 | Done | 2026-04-10 |
| 3. Core Monitoring Sections | v1.0 | 3/3 | Done | 2026-04-10 |
| 4. Battery Overview and Statistics | v1.0 | 4/4 | Done | 2026-04-11 |
| 5. Deep Battery Pack Diagnostics | v1.0 | 3/3 | Done | 2026-04-11 |
| 6. Battery Pack Access Fix | v1.1 | 3/3 | Done | 2026-04-11 |
| 7. Streaming Display and Configurable Timing | v1.1 | 3/3 | Done | 2026-04-11 |
| 8. Refresh Architecture | v1.2 | 0/0 | Not started | - |
| 9. Connection & Read Resilience | v1.2 | 0/0 | Not started | - |
| 10. Data Persistence & Tooltips | v1.2 | 0/0 | Not started | - |
| 11. Battery Pack Polish | v1.2 | 0/0 | Not started | - |
