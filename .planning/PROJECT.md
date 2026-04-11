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

### Active (v1.1)

- [ ] Configurable Modbus read delay via UI (default 500ms, separate pack settle time)
- [ ] Streaming parameter display — show each value as it arrives, not batch
- [ ] Battery pack access fix — investigate and fix why only 2 of 20 packs show as available (old CLI accesses all 20)
- [ ] Hardcode actual topology: 16 cells/pack, 10 packs/tower, 2 towers

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
| Configurable battery topology | Different setups: 1-2 inputs, 1-4 towers, 4-10 packs | — Pending |
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

## Current Milestone: v1.1 UX Polish & Battery Pack Fix

**Goal:** Fix battery pack access (all 20 packs), stream parameters in real-time, and add configurable Modbus timing.

**Target features:**
- Configurable Modbus read delay (UI control, separate pack settle time)
- Streaming parameter display (show each value as it arrives, not batch)
- Battery pack access fix (investigate why only 2 of 20 packs show)
- Hardcode actual topology: 16 cells/pack, 10 packs/tower, 2 towers

---
*Last updated: 2026-04-11 after v1.0 milestone, starting v1.1*
