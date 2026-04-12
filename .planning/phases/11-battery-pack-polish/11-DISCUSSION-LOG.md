# Phase 11: Battery Pack Polish - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-12
**Phase:** 11-battery-pack-polish
**Areas discussed:** Cell grid visual details, Streaming transition UX, Temperature display, Balance/Status rendering

---

## Cell Grid Visual Details

### Cell voltage summary bar streaming behavior

| Option | Description | Selected |
|--------|-------------|----------|
| Progressive update (Recommended) | Summary updates with each arriving cell — Min/Max/Spread/Avg recalculate as cells stream in | |
| Show after all cells arrive | Summary bar shows em-dashes until all 16 cells are read, then computes once | |
| You decide | Claude picks the best approach | |

**User's choice:** Progressive update — "the layout stays as it is, all values like min, max, average, spread are calculated on the fly, you just changing reading this value on fly, not waiting until all are received"
**Notes:** User emphasized layout must not change. Attached screenshot of current cell voltage grid as reference.

### Cell deviation color updates

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, update progressively (Recommended) | Recompute avg and recolor ALL received cells as each arrives | ✓ |
| Lock colors after all cells arrive | Cells stream in with neutral styling, colored once all 16 present | |
| You decide | Claude picks whichever creates less visual noise | |

**User's choice:** Yes, update progressively
**Notes:** None

### Cell voltage tooltips

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, same behavior (Recommended) | Each cell gets tooltip with register address (hex) and raw value as it streams in | ✓ |
| Enhanced tooltip | Add cell deviation from average to the tooltip content | |

**User's choice:** Yes, same behavior
**Notes:** None

---

## Streaming Transition UX

### Pack skeleton on navigation

| Option | Description | Selected |
|--------|-------------|----------|
| Full layout with em-dashes (Recommended) | Render complete cell grid (16 cells), temp rows, balance, status — all with em-dash placeholders | ✓ |
| Groups appear as they stream | Start with empty content, groups appear as first register of each arrives | |
| You decide | Claude picks based on consistency | |

**User's choice:** Full layout with em-dashes
**Notes:** None

### Cell grid skeleton

| Option | Description | Selected |
|--------|-------------|----------|
| All 16 cells pre-rendered with em-dashes (Recommended) | 4x4 grid fully visible from start, values replace em-dashes as they stream in | ✓ |
| Cells appear as they arrive | Grid starts empty, cells pop in one by one | |

**User's choice:** All 16 cells pre-rendered with em-dashes
**Notes:** Stable layout, no jumping

### Cached pack view restore

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, restore full grid dimmed (Recommended) | Navigating back shows all 16 cells with last-known voltages dimmed at 50% | ✓ |
| Always start fresh with em-dashes | No cache restore for pack views | |

**User's choice:** Yes, restore full grid dimmed
**Notes:** Consistent with Phase 10 D-08/D-09

---

## Temperature Display

### Temperature layout style

| Option | Description | Selected |
|--------|-------------|----------|
| Keep data-row list (Recommended) | Temperatures stay as a standard labeled list, color coded per row | |
| Grid layout like cells | 4-column grid matching cell voltages, with summary bar | ✓ |
| You decide | Claude picks based on sensor count | |

**User's choice:** Grid layout like cells
**Notes:** User chose grid despite only 4-6 sensors. Different from recommendation.

### Temperature summary bar

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, with summary bar | Min temp, Max temp, Spread, Avg — same pattern as cell voltages | |
| Grid only, no summary | Just temperature boxes in a grid, no summary stats | |
| You decide | Claude picks based on diagnostic value | |

**User's choice:** With summary bar, but with specific rules:
- Only Temp 1 through Temp 8 included in grid and summary calculations
- Env Temp and MOS Temp excluded from summary but included in grid as visually distinct items
- Temp 8 hidden entirely if value is 0.00°C (not connected)
- Any temp sensor reading 0.00°C excluded from display and calculations
**Notes:** Detailed filtering rules provided by user — this is domain knowledge about which sensors are pack-specific vs environmental.

### Env Temp and MOS Temp placement

| Option | Description | Selected |
|--------|-------------|----------|
| Data rows below the grid | Standard labeled rows underneath the temperature grid | |
| Include in grid but visually distinct | In the grid but with different label style or separator | ✓ |
| Hide them entirely | Don't display Env Temp and MOS Temp | |

**User's choice:** Include in grid but visually distinct
**Notes:** None

### Temperature color thresholds

| Option | Description | Selected |
|--------|-------------|----------|
| Keep current thresholds (Recommended) | Normal <45°C, Elevated 45-55°C or <0°C, Critical >55°C or <-10°C | ✓ |
| Adjust thresholds | Different temperature thresholds | |

**User's choice:** Keep current thresholds
**Notes:** None

---

## Balance/Status Rendering

### Balance State rendering

| Option | Description | Selected |
|--------|-------------|----------|
| Keep as-is (Recommended) | "Balanced" green status or "Balancing Active" with cell pills | |
| Grid layout for balance cells | All 16 cells in a grid with balancing cells highlighted | |
| Compact inline | One-liner with balancing cell list | |

**User's choice:** Balance state should be part of Cell Voltage group, shown after the cell grid, similar to summary fields like min/max/spread/avg. Stay with colors, add tooltip (currently missing).
**Notes:** Major structural change — Balance is no longer a separate group card, it becomes an inline summary element within the Cell Voltages group.

### Balance placement confirmation

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, exactly | Summary-style row below the 4x4 cell grid, inside the Cell Voltages card | ✓ |
| Below grid but separate card | Balance stays as its own card, just positioned after Cell Voltages | |

**User's choice:** Yes, exactly — inside the Cell Voltages card
**Notes:** None

### Pack Status card

| Option | Description | Selected |
|--------|-------------|----------|
| Keep as-is (Recommended) | Green "All clear" card or red alarm/fault list | ✓ |
| Compact status row | Single row with expand-on-click for details | |
| You decide | Claude picks | |

**User's choice:** Keep as-is
**Notes:** None

---

## Claude's Discretion

- Internal data structures for progressive cell summary computation
- How to visually distinguish Env Temp and MOS Temp in the temperature grid
- How to render the balance summary row within the cell voltage card
- Temperature grid summary bar styling

## Deferred Ideas

None — discussion stayed within phase scope
