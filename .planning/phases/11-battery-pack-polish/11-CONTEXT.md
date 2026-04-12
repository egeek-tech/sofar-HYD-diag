# Phase 11: Battery Pack Polish — Context

**Gathered:** 2026-04-12 (updated)
**Status:** Ready for planning

<domain>
## Phase Boundary

Pack drill-down displays data consistently with other sections and presents information in logical order. This phase makes pack views stream per-register (like System, Grid, etc.), reorders groups for logical presentation, and refines cell voltage/temperature grid layouts.

</domain>

<decisions>
## Implementation Decisions

### Pack Streaming Architecture
- **D-01:** Reuse the existing streaming pattern (section_schema + register_value + section_complete) for pack drill-down views. [locked]
- **D-02:** Send pack section_schema before every read cycle. Consistent with main sections. [locked]

### Group Display Order
- **D-03:** Info, Cells (with Balance inline), Temps, Status. [locked, updated]
  - **Why:** Balance State is now part of the Cell Voltages group (see D-07), so it no longer appears as a standalone group. Status at the end as least-frequently-checked diagnostic data.
  - **Current order:** Info, Cells, Temps, Status, Balance (in `buildPackDataMessage` append order)
  - **Required change:** Remove Balance as separate group. Reorder to: Info, Cells+Balance, Temps, Status.

### Unsupported Register Handling
- **D-04:** Probe once per pack, skip on timeout for subsequent reads. Self-adapting to inverter model. [locked]
- **D-05:** Reset unsupported register list on pack switch. Different packs may have different BMS firmware/capabilities. [locked]

### Read Delay Enforcement
- **D-06:** Enforce 500ms minimum delay globally in the broker between any two Modbus reads. [locked]

### Cell Voltage Grid Layout [locked]
- **D-07:** Preserve the current 4-column cell voltage grid layout exactly as-is. The layout includes:
  - Summary bar at top: Min (with cell index), Max (with cell index), Spread (mV, color-coded), Avg
  - 4x4 grid of cell boxes, each showing "Cell N" label and voltage value
  - Deviation color coding: green (within 5mV of avg), warn (5-20mV), danger (>20mV)
  - **Why:** User explicitly confirmed the current layout is correct and should not change.

### Cell Grid Streaming Behavior [locked]
- **D-08:** Summary bar (Min/Max/Spread/Avg) recalculates progressively as each cell value streams in. No waiting for all 16 cells.
- **D-09:** Deviation colors update progressively — as each cell arrives, recompute average and recolor ALL received cells.
- **D-10:** Cell tooltips identical to current behavior — register address (hex) and raw value per cell. No enhancement needed.

### Pack Skeleton and Caching [locked]
- **D-11:** When navigating to a pack drill-down, render the full layout with em-dash placeholders (all 16 cells, temp grid, status). Values fill in per-register. Matches main section streaming pattern.
- **D-12:** All 16 cell boxes are pre-rendered with "Cell N" label and em-dash value. Stable layout, no jumping as data arrives.
- **D-13:** Cached pack views restore the full cell grid with last-known values dimmed at 50%. Fresh read overwrites per-cell. Consistent with Phase 10 D-08/D-09.

### Temperature Grid Layout [locked]
- **D-14:** Temperatures use a 4-column grid layout matching cell voltages (not data-row list).
- **D-15:** Summary bar (Min/Max/Spread) calculated from Temp 1-8 only, excluding sensors reading 0.00°C.
  - Temp 8: hidden entirely if value is 0.00°C (not connected)
  - Env Temp and MOS Temp: displayed in the grid but visually distinct (different label style or separator). Excluded from Min/Max/Spread calculation.
- **D-16:** Temperature color thresholds preserved: normal (<45°C, green), elevated (45-55°C or <0°C, amber), critical (>55°C or <-10°C, red).

### Balance State Integration [locked]
- **D-17:** Balance State is part of the Cell Voltages group card, displayed as a summary-style row below the 4x4 cell grid (after the cell grid, before the group card ends).
  - Balance is no longer a separate group card.
  - Shows "Balanced" (green) when bitmap=0, or "Balancing: Cell N, Cell M..." with pills/badges when active.
  - Colors preserved (green for balanced, active color for balancing cells).
  - Tooltips added to balance data (register address for the balance bitmap).

### Pack Status Card [locked]
- **D-18:** Pack Status card rendering stays as-is. Green "All clear" card with checkmark when no faults. Red card with decoded alarm/protection/fault list when active.

### Claude's Discretion
- Internal data structures for progressive cell summary computation
- How to visually distinguish Env Temp and MOS Temp in the temperature grid (different background, label prefix, separator line, etc.)
- How to render the balance summary row within the cell voltage card (pill badges, inline text, etc.)
- Temperature grid summary bar styling (reuse cell voltage summary CSS or create temp-specific variant)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements
- `.planning/REQUIREMENTS.md` — BATT-01, BATT-02
- `.planning/ROADMAP.md` — Phase 11 success criteria

### Current implementation (to be modified)
- `internal/hub/hub.go` — `buildPackDataMessage()` (current batch builder, lines ~817-1029)
- `internal/hub/hub_streaming.go` — Per-register streaming pattern to reuse
- `web/static/app.js` — `renderPackDetail()` (line ~1869), `renderCellVoltageGrid()` (line ~1905), `renderPackTemperatures()` (line ~2004), `renderBalanceState()` (line ~2100), `renderPackStatusCard()` (line ~2028), `handlePackData()` (line ~1529)
- `web/static/style.css` — `.cell-grid` (line ~1045), `.cell-voltage` (line ~1051), `.cell-summary` (line ~1012), `.balance-status` / `.balance-pills`, `.fault-card`
- `internal/hub/message.go` — PackDataMessage, PackGroup structs
- `internal/register/probe.go` — Probe struct, pack probe definitions

### Prior phase context
- `.planning/phases/10-data-persistence-tooltips/10-CONTEXT.md` — D-05 (pack dimming), D-08/D-09 (section caching with per-pack keys), D-15 (tooltips on all values including pack cells)
- `.planning/phases/08-refresh-architecture/08-CONTEXT.md` — D-01 (section switch triggers read), D-03 (read delay enforcement)
- `.planning/phases/09-connection-read-resilience/09-CONTEXT.md` — D-08/D-09 (stale display on error)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `renderCellVoltageGrid()`: Current batch renderer for cell voltages — layout and styling to be preserved, but needs conversion to schema-driven skeleton + streaming fill
- `cell-grid` CSS class: 4-column grid with `repeat(4, 1fr)` — reuse for temperature grid
- `cell-summary` CSS class: Summary bar layout — reuse for temperature summary
- Phase 10 dimming/caching/tooltip infrastructure: All must work identically on pack views
- `handleRegisterValue()`: Per-register streaming handler — needs to handle pack register values

### Established Patterns
- Section schema + register_value streaming: Used by all main sections. Pack views adopt this pattern.
- Em-dash skeleton → dimmed cached → full value: Three visual states from Phase 10
- Progressive summary computation: New pattern for cell voltages — summary recalculates as each value arrives

### Integration Points
- `buildPackDataMessage()` in hub.go: Replace with per-register streaming (send schema, then stream individual register_value messages)
- `renderPackDetail()` in app.js: Replace batch renderer with schema-driven skeleton renderer
- Pack section_schema message: New message type to define the pack layout (groups, register positions)
- Balance State: Moves from standalone group to inline element within cell voltage group schema

</code_context>

<specifics>
## Specific Ideas

- Cell voltage grid layout is explicitly approved by user (screenshot reference) — do not modify the 4-column layout, summary bar, or color coding approach
- Temperature grid should mirror the cell voltage grid aesthetic but with sensor-specific rules (exclude zeros, separate Env/MOS)
- Balance State as an inline summary within the cell voltage card — should feel like it belongs there, not like a separate section jammed in
- Progressive summary updates give a "live calculation" feel as data streams in
- Pack skeleton must feel identical to main section skeletons (em-dashes in the right positions)

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

### Folded Todos
- **Stream pack drill-down values per-register** — Core of BATT-02
- **Skip unsupported PackInfoProbes registers (0x9104-0x9126)** — Improve pack read reliability
- **Read delay burst on section switch** — Enforce consistent inter-read timing globally

</deferred>

---

*Phase: 11-battery-pack-polish*
*Context gathered: 2026-04-12 (updated)*
