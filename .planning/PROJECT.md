# Sofar HYD Diagnostic Tool

## What This Is

A desktop-focused web application for monitoring and diagnosing Sofar HYD hybrid inverters via TCP Modbus. Built as a single Go binary with embedded HTML frontend, it reads real-time parameters from the inverter and presents them in a structured, easy-to-navigate interface. Based on an existing, proven CLI tool that already communicates correctly with the inverter.

## Core Value

Provide clear, real-time visibility into all Sofar HYD inverter parameters — especially battery pack diagnostics — through a reliable web interface that reuses the proven Modbus communication layer.

## Requirements

### Validated

- ✓ TCP Modbus communication with Sofar HYD inverter (func 0x03 read, 0x10 write) — existing
- ✓ Modbus TCP and RTU protocol support — existing
- ✓ Connection retry logic with reconnect — existing
- ✓ Battery pack selection via BMS_Inquire (0x9020) write — existing
- ✓ Battery online bitmap detection (0x9022) — existing
- ✓ CRC16 calculation for RTU mode — existing
- ✓ Register reading with proper data type handling (U16/S16/ASCII/scale) — existing
- ✓ Battery Information section: global battery info per channel, BMS global info, configurable topology, online bitmap — Validated in Phase 4
- ✓ Electricity Statistics section (daily/total/monthly/yearly: generation, consumption, bought, sold, battery charge/discharge) — Validated in Phase 4

### Validated (v1.4)

- ✓ Register groups with contiguous addresses batched into single Modbus requests (up to 60 regs) — Validated in Phase 18
- ✓ Batch reads respect 60-register limit and protocol block boundaries — Validated in Phase 18
- ✓ Per-span fallback to individual reads on batch failure — Validated in Phase 18, 19
- ✓ Composite probe type with formatting dispatch (system_time composition) — Validated in Phase 19
- ✓ System section ~3-5x faster with batch reading (confirmed by human testing) — Validated in Phase 19
- ✓ Configuration section dramatically faster with batch reading — Validated in Phase 19
- ✓ Progressive UI rendering preserved — values appear in span-groups — Validated in Phase 19

### Validated (v1.3)

- ✓ System time as single composed row (HH:MM:SS DD-MM-YYYY) — batch read replaces 6 individual register reads — Validated in Phase 14
- ✓ Read-only Configuration section with all V1.38 config registers, enum decoding, read-once caching, per-group streaming — Validated in Phase 15
- ✓ Complete tooltip coverage in pack drill-down (Balance State, Pack Status) — Validated in Phase 16
- ✓ Zero-temperature hiding for disconnected sensors, PackInfoProbes error suppression — Validated in Phase 16
- ✓ Per-group batch streaming in pack drill-down (groups fill at once instead of individual values) — Validated in Phase 16
- ✓ XLSX register discovery tool with three-way comparison (XLSX V1.29 / PDF V1.38 / current probes) — Validated in Phase 17
- ✓ Meter, DCDC, PCU, BDU sections added to sidebar with full register coverage — Validated in Phase 17
- ✓ 22 gap-filled registers across existing sections from XLSX discovery — Validated in Phase 17

### Validated (v1.2)

- ✓ Browser-driven auto-refresh — backend performs no autonomous cycles, browser triggers all reads — v1.2
- ✓ Consistent inter-read delay — no burst of rapid reads on section switch — v1.2
- ✓ Immediate disconnect — abort in-progress Modbus reads, connection closes within 1 second — v1.2
- ✓ Per-register retry — automatic retry up to 3 times, transparent error recovery — v1.2
- ✓ Refresh dimming — previously read values persist on screen (dimmed) until replaced by fresh values — v1.2
- ✓ Section value caching — navigating back shows cached values dimmed immediately — v1.2
- ✓ Parameter tooltips — hover shows register address (hex) and raw value — v1.2
- ✓ Pack drill-down streaming — per-register streaming consistent with other sections — v1.2
- ✓ Pack section reorder — balance state before temperatures, logical group ordering — v1.2
- ✓ BMS bitmap decode — human-readable alarm/protection/fault display in pack status — v1.2

### Validated (v1.1)

- ✓ Configurable Modbus read delay via UI (50-5000ms, separate pack settle 500-10000ms) — v1.1
- ✓ Streaming parameter display — each value appears as it is read with em-dash skeleton loading — v1.1
- ✓ Battery pack access fix — all 20 packs accessible, 0x9022 is tower bitmap (not pack bitmap) — v1.1
- ✓ Hardcoded actual topology: 16 cells/pack, 10 packs/tower, 2 towers — v1.1

### Validated (v1.0)

- ✓ Single Go binary with embedded HTML/JS/CSS frontend — v1.0
- ✓ Connection config page (IP address, port, slave ID, connect button) — v1.0
- ✓ System Information section — v1.0
- ✓ Grid Connected section — v1.0
- ✓ Grid Disconnected / EPS section — v1.0
- ✓ PV Input section (configurable 2-16 channels) — v1.0
- ✓ Battery pack drill-down: hierarchical navigation, cell voltages, temps, faults — v1.0
- ✓ Lazy loading — parameters load only when navigating to a section — v1.0
- ✓ Browser-driven auto-refresh with configurable cycle delay, no backend timer — Validated in Phase 8 (replaced v1.0 timer-based auto-refresh)
- ✓ Visual feedback: green/red background flash — v1.0
- ✓ Structured backend logging — v1.0
- ✓ Desktop-optimized layout — v1.0

### Validated (v1.6)

- ✓ Hub test suite optimized from 160s to 0.03s via synctest migration, idle-timeout helpers, and t.Parallel() — v1.6 Phase 27
- ✓ Data race in Hub.RunFunc/ClientCount fixed (done channel synchronization) — v1.6 Phase 27
- ✓ enforceInterReadDelay burst bug fixed (IsZero guard) — v1.6 Phase 27

### Validated (v1.5)

- ✓ Configuration section excludes unsupported registers (90 removed via hardware sweep) — v1.5 Phase 20
- ✓ All 7 standard sections (Grid, EPS, PV, Meter, PCU, BDU + DCDC removed) verified on real hardware — v1.5 Phase 21
- ✓ Three-state SpanTracker (Normal → Degraded → Skipped) with 10th-cycle probe recovery — v1.5 Phase 22
- ✓ Battery section reads via BatchPlan spans with channel auto-detection preserved — v1.5 Phase 23
- ✓ BMS section reads via BatchPlan spans with Composite probes for clock/version composition — v1.5 Phase 24
- ✓ Pack drill-down reads via BatchPlan spans with write+settle preserved — v1.5 Phase 25
- ✓ PV BatchPlan staleness fix, dead code cleanup — v1.5 Phase 26
- ✓ Battery channel config resets on reconnect (schema broadcast to subscribers) — v1.5 Phase 22 + quick fix

### Out of Scope

- Mobile-responsive design — desktop only
- User authentication — local diagnostic tool
- Database/history storage — real-time only
- Register writing (control commands) — read-only diagnostic tool
- Cloud connectivity — local network only
- Multi-inverter support — single inverter connection

## Context

- **Existing codebase:** `main.go` (707 lines) — fully working CLI tool with TCP Modbus communication to Sofar HYD inverter, verified against real hardware (2026-03-16)
- **Protocol reference:** Sofar_Inverter_MODBUS_V1.38_EN.pdf (V1.38, Jan 2025, 121 pages) — complete register map stored in memory
- **Hardware:** Sofar HYD hybrid inverter with AMASS batteries, accessed via TCP-to-RTU converter at configurable IP:port (default 10.5.99.29:4192)
- **Battery topology:** 1 input, 2 towers, 10 packs/tower = 20 packs total. Always 16 cells/pack. Pack selection via write to 0x9020, data read from 0x9044+
- **Key protocol constraints:** Max 60 registers per read, no cross-block reads, 500ms delay between reads recommended
- **PV channels:** Up to 16, registers at 0x0584 + 3*N (voltage, current, power per channel)
- **Frontend approach:** Use frontend-design skill for polished desktop UI

## Constraints

- **Tech stack**: Go backend (reuse existing Modbus code), vanilla HTML/JS/CSS frontend (embedded via Go embed)
- **Protocol**: Sofar Modbus-G3 protocol V1.38 — register addresses and data types are fixed
- **Hardware timing**: 500ms minimum delay between Modbus reads; BMS pack switch needs ~1s settle time
- **Single connection**: Only one TCP connection to inverter at a time (Modbus is serial)
- **Deployment**: Single binary, no external dependencies

## Current Milestone: v1.6 CI/CD, Docker & Test Performance

**Goal:** Production-ready CI/CD pipeline with Docker packaging, automated releases via conventional commits, and fast test suite.

**Target features:**
- Dockerfile with minimal image (scratch/distroless) receiving pre-built binary from CI
- GitHub Actions PR workflow (build + test + lint, skip on docs-only changes)
- GitHub Actions release workflow (auto-release on master merge, conventional commit semver, Docker push to ghcr.io)
- Dependabot for Go dependency updates
- Hub test suite optimization (currently 160s)

## Completed Milestone: v1.5 Full Batch Reading & Configuration Cleanup (shipped 2026-04-18)

Extended batch reading to every section. 90 unsupported config registers removed. SpanTracker auto-skips persistently-failing spans. Battery, BMS, and pack drill-down all migrated to BatchPlan spans. Shared `readBatchSpans` helper eliminated 3-way span loop duplication. 8 days, 7 phases, 14 plans, +8,449/-1,430 lines.

## Completed Milestone: v1.4 Batch Register Reading (shipped 2026-04-15)

Proved that batching contiguous register reads dramatically reduces section load times. System section 3-5x faster, Configuration section ~6x faster. Batch infrastructure ready to extend to all remaining sections.

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Single Go binary with embedded frontend | Simple deployment, no CORS, one file to copy | — Pending |
| WebSocket for live updates | Real-time push, no polling overhead, instant feedback | — Pending |
| Reuse existing main.go Modbus layer | Proven working code, verified against real hardware | — Pending |
| Configurable PV channels (2-16) | Different HYD models have different PV input counts | — Pending |
| Configurable battery topology | Different setups: 1-2 inputs, 1-4 towers, 4-10 packs | ⚠️ Revisit — v1.1 hardcoded to 2/10/16, configurable removed |
| Per-register streaming | Stream each value as read instead of batch-then-send | ✓ Good — v1.1, refined in v1.3 Phase 16 with per-group batching |
| Hardcoded topology constants | User's actual setup: 2 towers, 10 packs, 16 cells | ✓ Good — v1.1, simpler than configurable |
| 0x9022 is tower bitmap | "Battery" = tower in Sofar protocol, not individual pack | ✓ Good — v1.1, corrected from v1.0 misinterpretation |
| Desktop-only layout | Diagnostic tool used at inverter location on laptop/desktop | — Pending |
| Browser-driven refresh | Remove backend timer, browser triggers all reads | ✓ Good — v1.2, eliminates timer sync bugs |
| Abort-on-disconnect | Close TCP mid-read via goroutine abort mechanism | ✓ Good — v1.2, <1s disconnect |
| Per-register retry | Retry failed registers transparently up to 3 times | ✓ Good — v1.2, no user-visible transient errors |
| Refresh dimming | Show stale values dimmed until fresh data arrives | ✓ Good — v1.2, no blank screens during refresh |
| Pack streaming rewrite | Replace batch pack reads with per-register streaming | ✓ Good — v1.2, consistent with all other sections |
| Synthetic probe convention | Count: 0 marks schema-only probes skipped during read | ✓ Good — v1.3, enables composed values without extra message fields |
| Batch time read | Single ReadRegisters(0x042C, 6) replaces 6 individual reads | ✓ Good — v1.3, saves 2.5s per System refresh cycle |
| Composite probe type | Probe.Composite field with format dispatch instead of special-case code | ✓ Good — v1.4, system time is a real batchable probe |
| Batch span streaming | Iterate BatchPlan.Spans with single read per span, fallback per span | ✓ Good — v1.4, 3-5x speedup confirmed by human testing |
| Synthetic → Composite migration | Convert Count:0 probes to real Composite probes for batch eligibility | ✓ Good — v1.4, eliminates unbatchable probes in System section |
| Hardware sweep for register pruning | Build standalone tool, test on hardware, remove failing probes | ✓ Good — v1.5, 90 registers removed, zero fallback warnings |
| DCDC section removal | All registers returned illegal data address on this hardware | ✓ Good — v1.5, section deregistered and nav button removed |
| Three-state SpanTracker | Normal → Degraded → Skipped with 10th-cycle probe recovery | ✓ Good — v1.5, resilient to intermittent hardware failures |
| Shared readBatchSpans helper | Single function for all section batch reads, eliminates duplication | ✓ Good — v1.5, used by standard, battery, BMS, and pack sections |
| Battery reconnect reset | Reset Groups/Probes/BatchPlan to 2-channel default on StateConnected | ✓ Good — v1.5, eliminates stale channel config after reconnect |
| Schema broadcast on reconfiguration | Push updated section_schema to subscribers when Groups change | ✓ Good — v1.5, UI reflects channel changes immediately |

## Evolution

This document evolves at phase transitions and milestone boundaries.

Last updated: 2026-04-19 — v1.6 milestone started

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd-complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

## Current State

Shipped v1.5 with ~13,000 LOC Go + ~2,900 LOC JS/HTML/CSS.
Six milestones: v1.0 MVP, v1.1 UX Polish & Battery Pack Fix, v1.2 Reliability & UX Refinements, v1.3 Data Cleanup & Configuration, v1.4 Batch Register Reading, v1.5 Full Batch Reading & Configuration Cleanup.
All sections now use BatchPlan batch spans. SpanTracker auto-skips failing spans. Battery channel auto-detection with schema broadcast on reconnect. 90 unsupported config registers removed. Shared readBatchSpans helper eliminates span loop duplication.

**Known issues (from todos):**
- PackInfoProbes (0x9104-0x9126) returns illegal address on this BMS hardware — skip unsupported registers
- Read delay burst on section switch — enforceInterReadDelay timing edge case
- Stream pack drill-down values per-register — currently waits for full batch before sending

---
*Last updated: 2026-04-18 after v1.5 milestone*
