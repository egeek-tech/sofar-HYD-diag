# Project Research Summary

**Project:** Sofar HYD Diagnostic Tool
**Domain:** Industrial protocol (Modbus TCP) web gateway -- single-device diagnostic tool
**Researched:** 2026-04-10
**Confidence:** HIGH

## Executive Summary

This project wraps a proven CLI Modbus tool (707 lines of Go, verified against real Sofar HYD hardware) in a single-binary web server with an embedded vanilla HTML/JS/CSS frontend. The domain is well-understood: the Modbus protocol is fixed (V1.38 spec), the register map is complete, and the existing transport code already handles every protocol quirk (RTU framing, stale response skipping, function 0x10 workaround for BMS writes, 500ms inter-read timing). The challenge is not "can we talk to the inverter" -- that is solved -- but "how do we multiplex a serial protocol across concurrent web clients without breaking what already works."

The recommended approach is a hub-and-spoke architecture with a single-goroutine Modbus broker at the center. The broker serializes all hardware access through a command channel, enforces timing constraints, and handles atomic multi-step operations (pack selection write-wait-read). A WebSocket hub fans out results to subscribed browser clients, with section-based lazy loading to avoid overwhelming the slow Modbus bus. The entire stack adds exactly one external dependency (`github.com/coder/websocket` v1.8.14, zero transitive deps) to the existing zero-dependency Go codebase. Everything else -- HTTP routing, static file embedding, structured logging -- uses the Go standard library.

The primary risk is breaking the proven Modbus transport during refactoring. The existing code contains hard-won protocol workarounds that look like they could be "cleaned up" but are actually critical for hardware compatibility. The mitigation is a copy-paste-first extraction strategy: lift the transport layer into a package with zero functional changes, verify against hardware, then build the web layer on top. The secondary risk is the battery pack drill-down feature, which requires atomic write-wait-read sequences that must not be interleaved with other Modbus traffic. This must be designed into the broker's command channel from day one, not bolted on later.

## Key Findings

### Recommended Stack

The stack is almost entirely Go standard library, reflecting the existing codebase's zero-dependency philosophy. Go 1.26.x provides enhanced HTTP routing (method matching, path parameters since 1.22+), `embed` for single-binary deployment, and `log/slog` for structured logging. The only external dependency is `github.com/coder/websocket` v1.8.14 for WebSocket support -- chosen for its zero transitive dependencies, context-native API, and active maintenance. The frontend is vanilla HTML/JS/CSS with no build step, no Node.js tooling, and no framework.

**Core technologies:**
- **Go 1.26.x + net/http ServeMux**: HTTP server and REST API -- Go 1.22+ routing eliminates need for chi/echo/gin
- **github.com/coder/websocket v1.8.14**: Real-time push to browsers -- zero deps, context-native, concurrent-write-safe
- **embed + http.FileServerFS**: Single binary with bundled frontend -- no external files at runtime
- **log/slog**: Structured logging with runtime level control -- standard library, perfect for diagnostic tools
- **Vanilla HTML/JS/CSS**: Desktop-only diagnostic UI -- `<details>`, `<template>`, CSS custom properties, native WebSocket API

### Expected Features

**Must have (table stakes):**
- Connection management (IP/port/slave config, connect/disconnect, status indicator)
- System identification (SN, running state, firmware, temperatures)
- Real-time power overview (grid, PV, battery, load power)
- Grid parameters (voltage/current/power per phase, frequency, power factor)
- PV input display (configurable 2-16 channels)
- Battery status overview (SOC, SOH, voltage, current, power, cycles)
- Auto-refresh via WebSocket with on/off toggle
- Sectioned navigation with lazy loading (only read registers for visible section)
- Error/fault display with human-readable decoding
- Visual feedback (green flash on success, red on failure, stale data indicators)
- EPS/off-grid status
- Electricity statistics (daily/total generation, consumption, bought, sold)

**Should have (differentiators):**
- Deep battery pack drill-down (individual cell voltages, temperatures, per-pack fault data) -- THE killer feature
- Battery topology visualization (inputs > towers > packs with online/offline status)
- Cell voltage spread analysis (min/max/deviation color-coding per pack)
- BMS protection/alarm/fault decoding (bitmap-to-text)
- Configurable topology (PV channels, battery inputs/towers/packs)
- System fault register decoding (20 registers x 16 bits, 320 possible faults)

**Defer (v2+):**
- Historical event log (100 events from inverter memory -- requires full 300+ fault code lookup table)
- Battery cluster overview (depends on BDU hardware availability for testing)
- Internal diagnostics (leakage current, bus voltages -- niche use case)
- String current monitoring (only relevant for large PV arrays)

### Architecture Approach

Hub-and-spoke with a single-goroutine Modbus broker. The broker owns the TCP connection and enforces serial access, timing delays, and atomic multi-step operations. A WebSocket hub manages client subscriptions per section, broadcasts data only to interested clients, and dispatches read requests to the broker via channels (never blocking the hub goroutine). The frontend is a single-page app with tab navigation across six sections (System, Grid, EPS, PV, Battery, Statistics), where only the active tab triggers Modbus reads.

**Major components:**
1. **Modbus Transport** (`modbus/transport.go`) -- TCP/RTU framing, CRC16, raw register read/write. Extracted from main.go with zero functional changes.
2. **Modbus Broker** (`modbus/broker.go`) -- Connection lifecycle, command channel, request serialization, timing enforcement, retry logic, atomic pack queries.
3. **Register Definitions** (`register/`) -- Declarative register map: address, name, type, scale, unit, section. Single source of truth for both backend and frontend.
4. **WebSocket Hub** (`hub/`) -- Client registry, section-based subscriptions, fan-out broadcast, completion-triggered refresh (not timer-based).
5. **API Server** (`api/`) -- HTTP routes, WebSocket upgrade, embedded static file serving.
6. **Frontend** (`web/`) -- Vanilla JS single-page app, tab navigation, WebSocket client, DOM rendering with visual feedback.

### Critical Pitfalls

1. **Concurrent Modbus access** -- Multiple clients or refresh timers writing to the same TCP socket simultaneously corrupts frames and crashes the converter. Prevention: single-goroutine command channel owns all Modbus I/O. Never expose `net.Conn` to handlers.

2. **Pack selection race condition** -- BMS write-wait-read sequence (write 0x9020, wait 1s, read 0x9044+) must be atomic. If interleaved, the UI silently shows data for the wrong pack. Prevention: the command channel treats the entire sequence as one atomic operation. The mutex/lock must span the full write-wait-read, not individual operations.

3. **Bus blocking during pack enumeration** -- Eagerly scanning all packs (20 packs x 3-5s each = 100s) freezes the entire UI. Prevention: lazy-load individual packs on user click. Show online bitmap (0x9022) first. Priority queue allows system/grid reads to preempt queued pack queries.

4. **Breaking proven protocol code during refactoring** -- Subtle changes to frame construction, timing, or stale-response handling break hardware communication. Prevention: copy-paste extraction, hardware verification before and after each refactoring step, preserve all protocol-quirk comments.

5. **Connection state mismatch** -- TCP connection drops (network blip, converter reboot) but server still thinks it is connected. Prevention: periodic heartbeat reads (0x0404 every 30s), short TCP keepalive (10s), auto-reconnect after 2-3 consecutive failures, connection generation counter.

## Implications for Roadmap

Based on research, suggested phase structure:

### Phase 1: Foundation -- Modbus Service Extraction and Project Scaffold

**Rationale:** Everything depends on a correctly functioning, concurrency-safe Modbus layer. The existing main.go must be refactored into packages before any web code is written. This is the highest-risk phase because protocol regressions are invisible until hardware testing.
**Delivers:** `modbus/` package (transport + broker), `register/` package (definitions + sections), project scaffold with build tags for dev/prod asset serving, working CLI mode that proves the extracted code is identical to the original.
**Addresses:** Connection management foundation, register definition system.
**Avoids:** Pitfall 5 (breaking proven code), Pitfall 1 (concurrent access), Pitfall 2 (pack selection race), Pitfall 13 (losing 500ms timing), Pitfall 12 (60-register limit validation).

### Phase 2: WebSocket Hub and Core API

**Rationale:** The hub is the backbone of the real-time architecture. It must be built before any frontend work because the frontend's entire interaction model depends on the WebSocket message protocol. Building hub + API together allows end-to-end testing with a simple HTML test page.
**Delivers:** WebSocket hub with section-based subscriptions, connection/disconnection message flow, HTTP server with embedded static files, REST endpoint for connection status, completion-triggered refresh model.
**Addresses:** Auto-refresh toggle, lazy loading infrastructure, WebSocket message protocol.
**Avoids:** Pitfall 6 (WebSocket lifecycle leaks), Pitfall 7 (refresh overwhelming bus), Pitfall 8 (embed workflow friction).

### Phase 3: Frontend Shell and Core Sections

**Rationale:** With the backend API stable, the frontend can be built section by section. Core sections (System, Grid, PV, EPS) are straightforward register reads with no special sequencing -- low risk, high visible progress.
**Delivers:** Tab navigation shell, connection config page, System Info section, Grid Connected section, PV Input section, EPS section, visual feedback (green/red flash, stale indicators).
**Addresses:** System identification, real-time power overview, grid parameters, PV input display, EPS status, sectioned navigation, visual connection feedback.
**Avoids:** Pitfall 14 (stale data display), Pitfall 9 (poor error propagation to UI), Pitfall 11 (hardcoded register addresses in frontend).

### Phase 4: Battery Overview and Fault Display

**Rationale:** Battery status and fault display are table stakes but depend on the section infrastructure from Phase 3. Fault decoding requires building the lookup table from Appendix 6.1, which is a significant data entry effort. Battery overview is the simpler battery section (global SOC/SOH/power), not the deep pack drill-down.
**Delivers:** Battery status overview section (global battery info per channel), error/fault display with human-readable decoding, electricity statistics section, configurable topology settings (PV channels, battery layout).
**Addresses:** Battery status overview, error/fault display, electricity statistics, configurable topology.
**Avoids:** Pitfall 3 (does not attempt pack enumeration yet).

### Phase 5: Deep Battery Diagnostics (Differentiator)

**Rationale:** This is the killer feature but also the most complex. It requires the atomic write-wait-read sequence for pack selection, careful UI design for the pack sub-navigation, and handling of offline/hibernating packs. Building it last means the full infrastructure (broker, hub, frontend shell) is stable.
**Delivers:** Battery topology visualization (inputs > towers > packs tree), individual pack drill-down (24 cell voltages, 8 temperatures, SN, capacity, cycles, fault/alarm/protect states), cell voltage spread analysis with color-coding, BMS protection/alarm/fault decoding, battery cluster overview (if hardware available).
**Addresses:** Deep battery pack drill-down, battery topology visualization, cell voltage spread analysis, BMS fault decoding.
**Avoids:** Pitfall 2 (pack selection race -- broker already handles atomic sequences from Phase 1), Pitfall 3 (lazy load on click, not eager enumeration).

### Phase Ordering Rationale

- **Phases 1-2 are strictly dependency-ordered:** The broker must exist before the hub, and the hub must exist before the frontend. No parallelism is possible here.
- **Phase 3 is the largest and most parallelizable:** Each section (System, Grid, PV, EPS) is independent and can be built and tested in isolation.
- **Phase 4 groups features by shared infrastructure:** Fault decoding and battery overview both need the fault lookup table. Statistics is trivially simple once the section infrastructure exists.
- **Phase 5 is isolated to contain risk:** The pack drill-down's complexity (BMS write, settle time, multi-register reads, offline detection) is contained in a single phase. If it slips, the tool is still fully functional for all non-pack diagnostics.

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 1:** Needs `/gsd-research-phase` -- the Modbus transport extraction is the highest-risk refactoring. Must analyze main.go line by line to identify every protocol quirk that must be preserved. The command channel vs. mutex design decision needs concrete benchmarking against the real timing constraints.
- **Phase 5:** Needs `/gsd-research-phase` -- battery pack drill-down involves complex sequencing (write, wait, multi-read), offline/hibernating pack handling, and the BMS fault bitmap decoding. The cluster overview registers (0x9400+/0x9600+) have not been tested against hardware.

Phases with standard patterns (skip research-phase):
- **Phase 2:** WebSocket hub is a well-documented Go pattern (gorilla/coder hub example). Message protocol is fully specified in ARCHITECTURE.md.
- **Phase 3:** Straightforward register reads + DOM rendering. No novel patterns.
- **Phase 4:** Register reads + lookup table. The complexity is data entry (fault codes), not architecture.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | All technologies verified on pkg.go.dev. Single external dependency with zero transitive deps. Go stdlib covers everything else. |
| Features | HIGH | Feature set derived from Sofar Modbus V1.38 spec and verified against real hardware. Clear table stakes vs. differentiator separation. |
| Architecture | HIGH | Hub-and-spoke with serialized broker is the established pattern for serial-protocol web gateways. Patterns are well-documented in Go ecosystem. |
| Pitfalls | HIGH | Pitfalls derived from direct analysis of working CLI code and known Modbus protocol constraints. Hardware-verified timing requirements. |

**Overall confidence:** HIGH

### Gaps to Address

- **WebSocket library discrepancy:** STACK.md recommends `coder/websocket`, ARCHITECTURE.md examples use `gorilla/websocket` API patterns (ReadMessage/WriteMessage, SetPongHandler). Decision: use `coder/websocket` as STACK.md recommends -- it is actively maintained, has zero deps, and context-native. The hub patterns are the same; only the API surface differs slightly. Adapt gorilla hub pattern examples to coder/websocket API during Phase 2.
- **Fault code lookup table:** 320 possible faults from 20 registers x 16 bits, plus BMS-level faults. The full table from Appendix 6.1 of the protocol spec must be transcribed. This is a significant data-entry task that should be budgeted into Phase 4, not treated as trivial.
- **Battery cluster registers untested:** Cluster 1 (0x9400+) and Cluster 2 (0x9600+) registers have not been verified against the actual hardware (FEATURES.md notes BDU testing failed). Phase 5 should treat cluster overview as best-effort, with graceful degradation if registers are unavailable.
- **Dev/prod asset serving:** The build-tag approach for development workflow (serve from filesystem in dev, embed in prod) needs concrete implementation during Phase 2 scaffolding. Not architecturally complex, but must be set up early to avoid frontend development friction.

## Sources

### Primary (HIGH confidence)
- Sofar_Inverter_MODBUS_V1.38_EN.pdf (V1.38, January 2025) -- complete register map, protocol constraints, fault tables
- Existing `main.go` (707 lines) -- verified working CLI tool against real Sofar HYD hardware (2026-03-16)
- Go standard library documentation (pkg.go.dev) -- net/http routing, embed, log/slog
- `github.com/coder/websocket` v1.8.14 (pkg.go.dev) -- verified zero transitive dependencies

### Secondary (MEDIUM confidence)
- Domain knowledge from comparable tools: SolarAssistant, SolarMan/SOFAR Cloud, Fronius SolarWeb, SMA Sunny Portal, Home Assistant solar integrations, sofar2mqtt
- gorilla/websocket hub pattern documentation -- architecture pattern applicable regardless of WebSocket library choice

---
*Research completed: 2026-04-10*
*Ready for roadmap: yes*
