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
- ✓ WebSocket-based auto-refresh with toggle button — v1.0
- ✓ Visual feedback: green/red background flash — v1.0
- ✓ Structured backend logging — v1.0
- ✓ Desktop-optimized layout — v1.0

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

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Single Go binary with embedded frontend | Simple deployment, no CORS, one file to copy | — Pending |
| WebSocket for live updates | Real-time push, no polling overhead, instant feedback | — Pending |
| Reuse existing main.go Modbus layer | Proven working code, verified against real hardware | — Pending |
| Configurable PV channels (2-16) | Different HYD models have different PV input counts | — Pending |
| Configurable battery topology | Different setups: 1-2 inputs, 1-4 towers, 4-10 packs | ⚠️ Revisit — v1.1 hardcoded to 2/10/16, configurable removed |
| Per-register streaming | Stream each value as read instead of batch-then-send | ✓ Good — v1.1 |
| Hardcoded topology constants | User's actual setup: 2 towers, 10 packs, 16 cells | ✓ Good — v1.1, simpler than configurable |
| 0x9022 is tower bitmap | "Battery" = tower in Sofar protocol, not individual pack | ✓ Good — v1.1, corrected from v1.0 misinterpretation |
| Desktop-only layout | Diagnostic tool used at inverter location on laptop/desktop | — Pending |

## Evolution

This document evolves at phase transitions and milestone boundaries.

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

## Current Milestone: v1.2 Reliability & UX Refinements

**Goal:** Fix auto-refresh architecture, improve read reliability, and polish the diagnostic UI with better feedback and control.

**Target features:**
- Backend auto-refresh removed — browser-only refresh trigger, fixing sync state bugs
- Immediate disconnect — abort in-progress Modbus reads on disconnect
- Register read retry — automatically retry values that return errors
- Stale value persistence — show previous values dimmed until refreshed
- Parameter tooltips — hover to see register address and raw value
- Battery pack UI reorder — balance state moved before temperature
- Pack drill-down streaming — fix batch display, stream per-register like other sections
- Timing enforcement — fix read delay burst on section switch

## Current State

Shipped v1.1 with 9,334 LOC Go + vanilla HTML/JS/CSS.
All 20 battery packs accessible, per-register streaming display, configurable timing controls.
Two milestones complete (v1.0 MVP + v1.1 UX Polish & Battery Pack Fix).

**Known issues (from todos):**
- PackInfoProbes (0x9104-0x9126) returns illegal address on this BMS hardware
- Pack drill-down values appear as batch instead of streaming
- Read delay shows burst on section switch due to enforceInterReadDelay timing

---
*Last updated: 2026-04-12 after v1.2 milestone start*
