---
phase: 06-battery-pack-access-fix
plan: 03
subsystem: frontend
tags: [topology, ui-cleanup, hardcode, bitmap]
dependency_graph:
  requires: []
  provides: [hardcoded-topology-frontend, simplified-bitmap-click, compact-cell-grid]
  affects: [web/static/app.js, web/static/index.html, web/static/style.css]
tech_stack:
  added: []
  patterns: [hardcoded-constants, direct-tower-mapping]
key_files:
  created: []
  modified:
    - web/static/index.html
    - web/static/style.css
    - web/static/app.js
decisions:
  - "Replaced topology-label/topology-select CSS classes with pack-selector-label and pv-channel-select (reusing existing select styling)"
  - "Renamed showTopologyDropdowns to hidePackSelectors for clarity since topology controls no longer exist"
metrics:
  duration: 4m
  completed: "2026-04-11T17:12:39Z"
---

# Phase 06 Plan 03: Remove Topology UI and Hardcode Constants Summary

**One-liner:** Removed topology dropdown controls, hardcoded TOPO_TOWERS=2 / TOPO_PACKS=10, simplified bitmap click to input=1/tower=towerIndex+1, and compacted cell grid to 4 columns for 16-cell packs.

## What Was Done

### Task 1: Remove topology controls from HTML and CSS (eff3e35)

- Removed entire `#topology-controls` div from `index.html` (Inputs/Towers/Packs dropdown selectors)
- Removed `.topology-controls`, `.topology-label`, `.topology-select`, `.topology-select:focus` CSS rules from `style.css`
- Added `.pack-selector-controls` and `.pack-selector-label` CSS rules for the pack detail selector dropdowns
- Changed `.cell-grid` from `repeat(6, 1fr)` to `repeat(4, 1fr)` matching 16-cell topology

### Task 2: Hardcode topology in app.js and simplify bitmap click handler (d31c238)

- Replaced `BAT_INPUTS_KEY`, `BAT_TOWERS_KEY`, `BAT_PACKS_KEY`, `BAT_DEFAULT_INPUTS`, `BAT_DEFAULT_TOWERS`, `BAT_DEFAULT_PACKS` with two constants: `TOPO_TOWERS = 2`, `TOPO_PACKS = 10`
- Removed `topologyInputs` from `packViewState` (input is always 1)
- Updated `packViewState` initialization to use `TOPO_TOWERS` and `TOPO_PACKS`
- Removed `initTopologyDropdowns()` call from DOMContentLoaded initialization
- Removed BMS topology sync block from `navigateToSection` (no more `loadTopologyValue` calls)
- Removed BMS configure message send (`type: 'configure', section: 'bms'`) from section navigation
- Deleted four functions: `initTopologyDropdowns()`, `sendTopologyConfigure()`, `loadTopologyValue()`, `saveTopologyValue()`
- Simplified `handleBitmapCellClick`: `input = 1`, `tower = towerIndex + 1` (per D-06)
- Simplified bitmap grid selected cell detection: `cellInput = 1`, `cellTower = t + 1`
- Updated `initPackSelectors` to use `pack-selector-controls` class and `pv-channel-select` for select elements
- Replaced input dropdown loop in `populatePackSelectorOptions` with single option (value 1)
- Removed all `$('#topology-controls')` references from `showPackSelectors` and renamed `showTopologyDropdowns` to `hidePackSelectors`

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing CSS] Added pack-selector-label class to style.css**
- **Found during:** Task 2, Step 8
- **Issue:** Plan mentioned updating the container className to `pack-selector-controls` but the label elements also referenced `topology-label` which was deleted. Needed a replacement CSS class for label styling in the pack selector.
- **Fix:** Added `.pack-selector-label` CSS rule and reused existing `.pv-channel-select` class for select elements
- **Files modified:** web/static/style.css, web/static/app.js

**2. [Rule 1 - Bug] Renamed showTopologyDropdowns to hidePackSelectors**
- **Found during:** Task 2, Step 9
- **Issue:** `showTopologyDropdowns()` referenced `$('#topology-controls')` which no longer exists in DOM. Function was called from `returnToBMSOverview()`.
- **Fix:** Renamed to `hidePackSelectors()` which only hides the pack selector controls. Updated call site in `returnToBMSOverview()`.
- **Files modified:** web/static/app.js

## Decisions Made

1. **Pack selector label styling:** Used dedicated `.pack-selector-label` CSS class rather than inline styles, keeping consistent with the project's class-based styling approach.
2. **Select element reuse:** Pack selector dropdowns reuse `.pv-channel-select` class for consistent select element appearance across the app.
3. **Function rename:** `showTopologyDropdowns` renamed to `hidePackSelectors` since the topology concept no longer exists in the frontend.

## Verification Results

- `grep -c "topology-controls" web/static/index.html` returns 0
- `grep -c "initTopologyDropdowns" web/static/app.js` returns 0
- `grep -c "BAT_INPUTS_KEY" web/static/app.js` returns 0
- `grep "TOPO_TOWERS = 2" web/static/app.js` finds the constant
- `grep "var input = 1;" web/static/app.js` finds hardcoded input in handleBitmapCellClick
- `grep "repeat(4, 1fr)" web/static/style.css` finds updated cell grid columns
- `go build ./...` succeeds (embedded files compile)

## Commits

| Task | Name | Commit | Key Files |
|------|------|--------|-----------|
| 1 | Remove topology controls from HTML and CSS | eff3e35 | web/static/index.html, web/static/style.css |
| 2 | Hardcode topology in app.js and simplify bitmap click handler | d31c238 | web/static/app.js, web/static/style.css |

## Self-Check: PASSED

All files exist. All commits verified.
