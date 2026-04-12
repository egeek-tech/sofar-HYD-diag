---
title: Native Go UI Evaluation
created: 2026-04-10
context: Phase 4 UAT revealed systemic issues
---

# Native Go UI Evaluation

## Motivation

Two systemic issues with the current Go + embedded HTML/WebSocket architecture:

1. **State sync fragility** — Frontend frequently shows different state than backend (loading spinner when disconnected, auto-refresh toggle ignored, section state out of sync). Observed across ALL sections, not just Phase 4. Root cause: loose coupling between Go state and browser DOM via JSON-over-WebSocket.

2. **Batch update UX** — Hub reads all probes for a section in one batch (20+ registers at 500ms each = 10+ seconds), then sends one big JSON payload. User stares at "Loading..." until entire batch completes. Want per-parameter streaming updates — each value appears the moment its Modbus read returns.

## Decision Pending

Build a PoC with a native Go UI framework to evaluate whether it solves both issues. Compare UX against current HTML approach before committing to a direction.

## Framework Candidates

- **Fyne** — Most mature Go GUI. Cross-platform widgets. ~50MB binary increase.
- **Wails** — Go + web frontend with direct bindings (tighter than WebSocket).
- **Gio** — Immediate-mode GPU rendering. Lightweight but less widget ecosystem.
- **Bubble Tea** — Terminal TUI. Zero deps, instant. No graphics.

## Impact If Adopted

- All remaining phases (Phase 5 pack drill-down) would use native UI
- Current HTML/WebSocket frontend (web/ package, hub, client) would be replaced
- Single binary deployment preserved regardless of choice
- Modbus broker and register packages unaffected — only presentation layer changes
