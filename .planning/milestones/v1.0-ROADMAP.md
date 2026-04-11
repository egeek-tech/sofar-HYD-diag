# Roadmap: Sofar HYD Diagnostic Tool

## Overview

Transform a proven CLI Modbus tool into a single-binary web application for real-time Sofar HYD inverter diagnostics. The journey moves from extracting the existing Modbus layer into a concurrency-safe service, through building the WebSocket-based real-time backbone and connection UI, into progressive frontend sections (System, Grid, EPS, PV), then battery overview and statistics, and finally the deep battery pack drill-down that is the tool's killer feature.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 1: Foundation and Modbus Service** - Extract proven Modbus code into concurrency-safe packages with project scaffold
- [x] **Phase 2: WebSocket Hub, API, and Connection UI** - Build real-time communication backbone with connection management frontend
- [x] **Phase 3: Core Monitoring Sections** - System Info, Grid, EPS, PV sections with tab navigation and desktop layout
- [x] **Phase 4: Battery Overview and Statistics** - Global battery info, topology config, electricity statistics, and fault display
- [x] **Phase 5: Deep Battery Pack Diagnostics** - Hierarchical pack navigation with cell voltages, temperatures, and fault decoding

## Phase Details

### Phase 1: Foundation and Modbus Service
**Goal**: A buildable Go project with the proven Modbus transport extracted into reusable packages, serialized through a concurrency-safe broker, with structured logging and single-binary embedding ready
**Depends on**: Nothing (first phase)
**Requirements**: CONN-04, CONN-05, INFRA-01, INFRA-02, INFRA-03
**Success Criteria** (what must be TRUE):
  1. Running `go build` produces a single binary that starts an HTTP server on a configurable port
  2. The Modbus broker connects to the inverter and reads registers identically to the original CLI tool (verified against hardware)
  3. All Modbus operations are serialized through a single-goroutine command channel -- concurrent callers never corrupt the TCP connection
  4. Backend logs are structured (JSON via slog) with configurable log level, and Modbus request/response details appear in debug logs
  5. Auto-reconnect recovers from connection drops with exponential backoff without manual intervention
**Plans:** 3 plans

Plans:
- [x] 01-01-PLAN.md -- Project scaffold and Modbus TCP/RTU protocol extraction
- [x] 01-02-PLAN.md -- Register definitions and concurrency-safe broker with auto-reconnect
- [x] 01-03-PLAN.md -- Web package, HTTP server entry point, and integration wiring

### Phase 2: WebSocket Hub, API, and Connection UI
**Goal**: Users can configure and manage the inverter connection through the browser, and the real-time WebSocket infrastructure is operational for all subsequent sections
**Depends on**: Phase 1
**Requirements**: CONN-01, CONN-02, CONN-03, RT-01, RT-02, RT-03, RT-04, RT-05
**Success Criteria** (what must be TRUE):
  1. User can enter IP address, port, and slave ID on a connection config page and click Connect/Disconnect
  2. Connection status is visually indicated and persists across browser page refreshes (localStorage)
  3. WebSocket connection is established between browser and server, delivering section data as push messages (not polling)
  4. Navigating to a section triggers register reads only for that section (lazy loading); inactive sections do not generate Modbus traffic
  5. User can toggle auto-refresh on/off per section; successful refreshes flash green, failures flash red
**Plans:** 3 plans
**UI hint**: yes

Plans:
- [x] 02-01-PLAN.md -- Broker dormant start, Reconfigure, Disconnect + hub message types and broker interface
- [x] 02-02-PLAN.md -- Hub event loop, client read/write pumps, section registry, timer management, demo section
- [x] 02-03-PLAN.md -- API endpoints, server wiring, and complete frontend rewrite with sidebar layout

### Phase 3: Core Monitoring Sections
**Goal**: Users can monitor all non-battery inverter parameters -- system identity, grid status, EPS status, and PV production -- through a tabbed desktop interface
**Depends on**: Phase 2
**Requirements**: SYS-01, SYS-02, SYS-03, SYS-04, SYS-05, GRID-01, GRID-02, GRID-03, GRID-04, EPS-01, EPS-02, EPS-03, EPS-04, PV-01, PV-02, PV-03, INFRA-04
**Success Criteria** (what must be TRUE):
  1. User can view device serial number, firmware versions, running state, temperatures, insulation impedance, and fan speed in the System section
  2. User can view active faults with human-readable descriptions decoded from fault registers
  3. User can view grid frequency, per-phase voltage/current/power, PCC power, line voltages, load power, and power factor in the Grid section
  4. User can view EPS load power, output voltage/frequency, per-phase inverter output, and emergency load voltages in the EPS section
  5. User can view per-channel PV voltage/current/power and total PV power, with a dropdown to configure 2-16 channels
**Plans:** 3 plans
**UI hint**: yes

Plans:
- [x] 03-01-PLAN.md -- ProbeGroup struct, enum tables, full register definitions, fault decoder, PV generator
- [x] 03-02-PLAN.md -- Hub grouped section data, fault reading, system time composition, configure message, CLI flag
- [x] 03-03-PLAN.md -- Frontend grouped renderer, fault card, PV dropdown, multi-column layout, nav changes

### Phase 4: Battery Overview and Statistics
**Goal**: Users can view global battery status per channel, BMS summary info, online battery bitmap, configurable topology, and electricity generation/consumption statistics
**Depends on**: Phase 3
**Requirements**: BAT-01, BAT-02, BAT-03, BAT-04, BAT-05, BAT-06, STAT-01, STAT-02, STAT-03
**Success Criteria** (what must be TRUE):
  1. User can view per-channel battery voltage, current, power, SOC, SOH, cycles, and charge/discharge state with human-readable labels
  2. User can view BMS global info (manufacturer, protocol, cell type, total voltage, current, SOC, SOH) and the online battery bitmap showing which packs are online
  3. User can configure battery topology (inputs 1-2, towers per input 1-4, packs per tower 4-10) with sensible defaults (1/2/10)
  4. User can view daily and total electricity statistics: generation, consumption, bought, sold, battery charge, battery discharge
**Plans:** 3 plans
**UI hint**: yes

Plans:
- [x] 04-01-PLAN.md -- U32 register support, battery state enum, register definitions for Battery/BMS/Statistics sections
- [x] 04-02-PLAN.md -- Hub section registration, BMS write-read cycle, topology configure, CLI flags, API defaults
- [x] 04-03-PLAN.md -- Frontend bitmap grid widget, protection card, topology dropdowns, nav enablement, CSS

### Phase 5: Deep Battery Pack Diagnostics
**Goal**: Users can drill into individual battery packs to inspect cell-level voltages, temperatures, and fault states -- the tool's primary differentiator
**Depends on**: Phase 4
**Requirements**: BAT-07, BAT-08, BAT-09, BAT-10, BAT-11
**Success Criteria** (what must be TRUE):
  1. User can navigate hierarchically: select input, then tower, then pack -- reflecting the configured topology
  2. User can view individual pack details: serial number, total voltage, SOC, current, remaining/full capacity, cycles, cell count
  3. User can view all 24 cell voltages per pack with min/max/spread highlighted (color-coded deviation from average)
  4. User can view pack temperatures (up to 8 sensors plus MOS and environment) and pack alarm/protection/fault/balance states decoded from bitmaps into readable text
**Plans:** 3 plans
**UI hint**: yes

Plans:
- [x] 05-01-PLAN.md -- Pack RT/Info probe definitions, alarm/protection bitmap tables, EncodePackQuery, DecodeBalanceState
- [x] 05-02-PLAN.md -- Hub triggerPackRead write-settle-read cycle, select_pack handler, pack data/error messages
- [x] 05-03-PLAN.md -- Frontend bitmap click, pack detail sub-view, cell voltage grid, temp display, alarm card, balance pills

## Progress

**Execution Order:**
Phases execute in numeric order: 1 -> 2 -> 3 -> 4 -> 5

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Foundation and Modbus Service | 3/3 | Done | - |
| 2. WebSocket Hub, API, and Connection UI | 3/3 | Done | 2026-04-10 |
| 3. Core Monitoring Sections | 3/3 | Done | 2026-04-10 |
| 4. Battery Overview and Statistics | 4/4 | Done | 2026-04-11 |
| 5. Deep Battery Pack Diagnostics | 3/3 | Done | 2026-04-11 |
