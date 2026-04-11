# Milestones

## v1.1 UX Polish & Battery Pack Fix (Shipped: 2026-04-11)

**Phases completed:** 2 phases, 6 plans, 11 tasks

**Key accomplishments:**

- Hardcoded battery topology constants (2 towers, 10 packs, 16 cells) replacing configurable UI, fixing BMS pack access to match proven CLI behavior
- Corrected 0x9022 tower bitmap interpretation — single read shows all towers online status without cycling
- Per-register streaming display — each parameter value appears in the UI the moment it is read, with em-dash skeleton loading
- Configurable Modbus timing — Read Delay (50-5000ms) and Pack Settle (500-10000ms) controls in header bar with localStorage persistence
- Updated 15 hub tests for streaming model, added Wave 0 test stubs for all phase requirements

---

## v1.0 Sofar HYD Diagnostic Tool (Shipped: 2026-04-11)

**Phases completed:** 5 phases, 16 plans, 33 tasks

**Key accomplishments:**

- Modbus TCP/RTU protocol codecs extracted verbatim from proven CLI tool into internal/modbus package with slog debug logging and net.Pipe round-trip tests
- Channel-based Modbus broker serializing all operations through single goroutine with exponential backoff reconnection, plus centralized register probe definitions with FormatValue formatting
- Single-binary HTTP server with embedded static frontend, /api/status JSON endpoint, slog structured logging, and SIGINT/SIGTERM graceful shutdown wiring all Phase 1 packages together
- Dormant-start broker with runtime Reconfigure/Disconnect commands, hub WebSocket message types, and BrokerInterface abstraction
- Hub event loop with client lifecycle, section subscription triggering immediate Modbus reads, auto-refresh timers with pause/resume on connection state, and demo "status" section reading 3 inverter registers
- WebSocket /ws and /api/defaults endpoints wired to hub, complete sidebar connection UI with form validation, localStorage persistence, section navigation, and real-time data card rendering via safe DOM API
- ProbeGroup-based register definitions for all 4 monitoring sections (System/Grid/EPS/PV) with enum tables, 240-entry fault decoder, and dynamic PV channel generator
- ProbeGroup-based hub sections with grouped WebSocket output, fault decoding, system time composition, PV channel configuration, and -pv-channels CLI flag
- Grouped data renderer with multi-column CSS grid layout, fault card (amber/green persistent styling), PV channel dropdown (2-16 with localStorage), and auto-navigate to System on connect
- U32 register support, multi-channel battery groups, BMS info/protection probes, and 4-period statistics groups with 24 U32 energy metrics
- Full backend integration of battery, BMS, and statistics sections with BMS write-read bitmap cycle, battery auto-detect, topology configure, and GroupData type extension for bitmap/protection widgets
- Bitmap grid widget, topology dropdowns, protection card, and statistics display completing Phase 4 browser experience
- Simplified BMS read to standard probe batch (removed 0x9020 write timeout), added disconnected subscribe error, synced auto-refresh toggle on navigate
- RED:
- Complete pack detail drill-down UI with bitmap click navigation, 6x4 cell voltage grid with deviation color coding, temperature thermal ranges, alarm/protection/fault decoding, and balance state cell pills

---
