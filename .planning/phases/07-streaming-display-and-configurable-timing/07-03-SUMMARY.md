---
phase: "07"
plan: "03"
subsystem: frontend-streaming
tags: [streaming, timing-controls, skeleton-rendering, websocket]
dependency_graph:
  requires: [07-01, 07-02]
  provides: [streaming-display, timing-ui, skeleton-rendering]
  affects: [web/static/app.js, web/static/index.html, web/static/style.css]
tech_stack:
  added: []
  patterns: [data-register-attribute, skeleton-card, em-dash-placeholder, streaming-value-update]
key_files:
  created: []
  modified:
    - web/static/index.html
    - web/static/style.css
    - web/static/app.js
decisions:
  - "Used data-register attribute keyed as GroupName::RegisterName for targeted DOM updates"
  - "Streaming skeleton coexists with legacy batch mode via hasStreamingSkeleton check"
  - "Timing config sent on WebSocket connect to restore user preferences on reconnect"
metrics:
  duration: "4 minutes"
  completed: "2026-04-11"
  tasks_completed: 2
  tasks_total: 3
---

# Phase 7 Plan 3: Frontend Streaming Display and Timing Controls Summary

Frontend streaming display with skeleton rendering, per-register value updates, and configurable timing inputs in the header bar.

## What Was Done

### Task 1: Timing Controls HTML and Streaming/Timing CSS (b251e57)

- Added timing control inputs (Read Delay, Pack Settle) to header bar in `index.html`
- Positioned between PV channel select and auto-refresh button
- Added CSS custom properties: `--timing-input-bg`, `--timing-input-border`, `--timing-input-width`, `--timing-input-height`
- Added timing control styles: `.timing-controls`, `.timing-control`, `.timing-control__label`, `.timing-control__input`, `.timing-control__unit`
- Added streaming value state styles: `.data-row-h__value--pending` (dimmed), `.data-row-h__value--stale` (dimmed with warning icon via ::after pseudo-element)
- Hidden by default (`style="display:none;"`) -- JS shows when connected

### Task 2: Streaming Message Handlers, Skeleton Rendering, and Timing JS (97e18a6)

- Added `handleSectionSchema`: renders skeleton DOM with em-dash placeholders on subscribe
- Added `renderSkeletonCard`: creates group cards with `data-register` attributes for targeted updates
- Added `handleRegisterValue`: updates single values in-place by finding `[data-register="GroupName::RegisterName"]`
- Added `handleSectionComplete`: updates timestamp and triggers green success flash
- Updated `handleSectionData`: detects streaming skeleton presence and only renders computed groups (bitmap, protection, faults) without clearing skeleton
- Added `initTimingControls`: loads values from localStorage, clamps input, sends configure message on change/Enter
- Added timing controls show/hide logic in `handleConnectionState`
- Sends stored timing config on WebSocket connect via onopen handler
- Added constants: `TIMING_STORAGE_KEY`, `TIMING_DEFAULTS`

### Task 3: Human Verification (checkpoint)

Awaiting user verification of streaming display and timing controls in browser.

## Deviations from Plan

None -- plan executed exactly as written.

## Key Technical Details

- **Skeleton rendering**: `section_schema` message triggers skeleton with em-dash placeholders. Each value element gets `data-register="GroupName::RegisterName"` attribute for O(1) lookup.
- **Streaming coexistence**: `handleSectionData` checks for `[data-register]` elements to determine streaming vs legacy batch mode. In streaming mode, only computed groups (bitmap, protection) and faults are rendered into placeholder divs.
- **Timing persistence**: Values saved to `localStorage` under `sofar_timing` key. Restored on page load and sent to backend on WebSocket connect.
- **Input clamping**: Read Delay clamped to 100-5000ms, Pack Settle clamped to 500-10000ms. Client-side clamping mirrors server-side authoritative bounds.

## Verification Results

- Build: `go build ./...` -- PASS
- Tests: `go test ./... -count=1` -- PASS (all packages)
- All acceptance criteria verified via grep checks

## Self-Check: PENDING

Awaiting checkpoint completion for final self-check.
