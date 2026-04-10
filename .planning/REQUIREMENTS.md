# Requirements: Sofar HYD Diagnostic Tool

**Defined:** 2026-04-10
**Core Value:** Clear, real-time visibility into all Sofar HYD inverter parameters -- especially battery pack diagnostics -- through a reliable web interface

## v1 Requirements

Requirements for initial release. Each maps to roadmap phases.

### Connection

- [x] **CONN-01**: User can configure IP address, port, and slave ID for inverter connection
- [x] **CONN-02**: User can connect/disconnect with a button showing connection status
- [x] **CONN-03**: Connection state persists across page refreshes (saved to browser localStorage)
- [x] **CONN-04**: Backend auto-reconnects on connection loss with exponential backoff
- [x] **CONN-05**: Modbus operations serialized through single-goroutine command channel (concurrency safety)

### System Information

- [x] **SYS-01**: User can view device serial number and firmware versions (HW, comm SW, master/slave DSP, safety cert)
- [x] **SYS-02**: User can view system running state with human-readable label (Wait/Detect/Grid/EPS/Fault/etc.)
- [x] **SYS-03**: User can view system time, ambient temperatures, radiator/module temperatures
- [x] **SYS-04**: User can view insulation impedance and fan speed
- [x] **SYS-05**: User can view active faults with human-readable descriptions (decoded from fault registers 0x0405-0x043D)

### Grid Connected

- [x] **GRID-01**: User can view grid frequency and total active/reactive/apparent power
- [x] **GRID-02**: User can view per-phase data: voltage, current, active power, reactive power, power factor (R/S/T)
- [x] **GRID-03**: User can view PCC active/reactive power
- [x] **GRID-04**: User can view line voltages (L1/L2/L3), total load power, total power factor, generation efficiency

### EPS / Grid Disconnected

- [x] **EPS-01**: User can view EPS load active/reactive/apparent power
- [x] **EPS-02**: User can view output voltage frequency
- [x] **EPS-03**: User can view per-phase inverter output voltage and load current (R/S/T)
- [x] **EPS-04**: User can view emergency load voltage per phase

### PV Input

- [x] **PV-01**: User can view per-channel voltage, current, and power for each PV input
- [x] **PV-02**: User can configure number of PV channels (2-16, default 2) via dropdown
- [x] **PV-03**: User can view total PV power

### Battery

- [x] **BAT-01**: User can view global battery info per channel: voltage, current, power, env temp, SOC, SOH, cycles
- [x] **BAT-02**: User can view battery state per channel (charge/discharge/sleep/fault/loss) with human-readable labels
- [x] **BAT-03**: User can view charge/discharge limits, total charge/discharge power, average SOC, total capacity
- [x] **BAT-04**: User can view BMS global info: manufacturer, protocol version, cell type, total voltage, current, avg temp, SOC, SOH
- [x] **BAT-05**: User can view online battery bitmap showing which packs are online
- [ ] **BAT-06**: User can configure battery topology: number of inputs (1-2), towers per input (1-4), packs per tower (4-10), with defaults 1/2/10
- [ ] **BAT-07**: User can navigate hierarchically: select input → select tower → select pack to view details
- [ ] **BAT-08**: User can view individual pack details: SN, total voltage, SOC, current, remaining/full capacity, cycles, cell count
- [ ] **BAT-09**: User can view 24 cell voltages per pack with min/max/spread highlighting
- [ ] **BAT-10**: User can view pack temperatures (up to 8 sensors + MOS temp + env temp)
- [ ] **BAT-11**: User can view pack alarm, protection, fault, and balance states with decoded bitmaps

### Electricity Statistics

- [x] **STAT-01**: User can view daily and total: power generation, load consumption
- [x] **STAT-02**: User can view daily and total: power bought from grid, power sold to grid
- [x] **STAT-03**: User can view daily and total: battery charge, battery discharge

### Real-time Updates

- [x] **RT-01**: Parameters load lazily -- only when user navigates to a section
- [x] **RT-02**: User can toggle auto-refresh per section via button
- [x] **RT-03**: Successfully refreshed parameters show light-green background flash
- [x] **RT-04**: Failed parameter reads show light-red background
- [x] **RT-05**: Real-time updates delivered via WebSocket (not polling)

### Infrastructure

- [x] **INFRA-01**: Application builds as single Go binary with embedded HTML/JS/CSS
- [x] **INFRA-02**: Backend uses structured logging (slog) with configurable log level
- [x] **INFRA-03**: Modbus request/response details logged for troubleshooting
- [x] **INFRA-04**: Desktop-optimized layout using full page width for parameter display

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Advanced Diagnostics

- **ADV-01**: Cell voltage spread analysis with trend indicators across multiple reads
- **ADV-02**: Connection health dashboard (latency histogram, error rate over time)
- **ADV-03**: Fault history log with timestamps

### Extended Data

- **EXT-01**: Internal diagnostics (BUS voltages, buck-boost current, fly-across cap voltages)
- **EXT-02**: String current monitoring (confluence info)
- **EXT-03**: Monthly/yearly electricity statistics breakdown
- **EXT-04**: Battery cluster data (0x9400+/0x9600+) if BDU is available

## Out of Scope

| Feature | Reason |
|---------|--------|
| Mobile-responsive design | Desktop diagnostic tool used on laptop at inverter location |
| User authentication | Local network tool, no public access |
| Database / history storage | Real-time diagnostic tool, not a monitoring platform |
| Register writing (control commands) | Read-only diagnostic tool (except BMS_Inquire for pack selection) |
| Cloud connectivity | Local network only |
| Multi-inverter support | Single connection at a time |
| Charts / graphs | v1 focuses on real-time values, not trends |
| Data export (CSV/JSON) | Not needed for diagnostic use |
| MQTT integration | Not a monitoring bridge |
| Firmware updates | Out of scope for diagnostic tool |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| CONN-01 | Phase 2 | Complete |
| CONN-02 | Phase 2 | Complete |
| CONN-03 | Phase 2 | Complete |
| CONN-04 | Phase 1 | Complete |
| CONN-05 | Phase 1 | Complete |
| SYS-01 | Phase 3 | Complete |
| SYS-02 | Phase 3 | Complete |
| SYS-03 | Phase 3 | Complete |
| SYS-04 | Phase 3 | Complete |
| SYS-05 | Phase 3 | Complete |
| GRID-01 | Phase 3 | Complete |
| GRID-02 | Phase 3 | Complete |
| GRID-03 | Phase 3 | Complete |
| GRID-04 | Phase 3 | Complete |
| EPS-01 | Phase 3 | Complete |
| EPS-02 | Phase 3 | Complete |
| EPS-03 | Phase 3 | Complete |
| EPS-04 | Phase 3 | Complete |
| PV-01 | Phase 3 | Complete |
| PV-02 | Phase 3 | Complete |
| PV-03 | Phase 3 | Complete |
| BAT-01 | Phase 4 | Complete |
| BAT-02 | Phase 4 | Complete |
| BAT-03 | Phase 4 | Complete |
| BAT-04 | Phase 4 | Complete |
| BAT-05 | Phase 4 | Complete |
| BAT-06 | Phase 4 | Pending |
| BAT-07 | Phase 5 | Pending |
| BAT-08 | Phase 5 | Pending |
| BAT-09 | Phase 5 | Pending |
| BAT-10 | Phase 5 | Pending |
| BAT-11 | Phase 5 | Pending |
| STAT-01 | Phase 4 | Complete |
| STAT-02 | Phase 4 | Complete |
| STAT-03 | Phase 4 | Complete |
| RT-01 | Phase 2 | Complete |
| RT-02 | Phase 2 | Complete |
| RT-03 | Phase 2 | Complete |
| RT-04 | Phase 2 | Complete |
| RT-05 | Phase 2 | Complete |
| INFRA-01 | Phase 1 | Complete |
| INFRA-02 | Phase 1 | Complete |
| INFRA-03 | Phase 1 | Complete |
| INFRA-04 | Phase 3 | Complete |

**Coverage:**
- v1 requirements: 44 total
- Mapped to phases: 44
- Unmapped: 0

---
*Requirements defined: 2026-04-10*
*Last updated: 2026-04-10 after roadmap creation*
