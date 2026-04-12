---
phase: 10-data-persistence-tooltips
plan: 03
subsystem: api, frontend
tags: [pack-drilldown, tooltip, register-metadata, cell-voltage]

# Dependency graph
requires:
  - phase: 10-data-persistence-tooltips
    plan: 01
    provides: FormatRawValue utility, RegisterValueMessage with register_addr
provides:
  - PackGroup.ItemMeta for per-item register metadata in pack drill-down
  - PackGroup.CellAddrs for per-cell register addresses
  - Frontend pack renderers set tooltip data attributes
affects: [tooltip-system, pack-detail-views]

# Tech tracking
tech-stack:
  added: []
  patterns: [item-meta-per-group, cell-addrs-array]

key-files:
  created: []
  modified:
    - internal/hub/message.go
    - internal/hub/hub.go
    - internal/hub/section.go
    - internal/hub/hub_test.go
    - internal/register/format.go
    - web/static/app.js

key-decisions:
  - "PackItemMeta carries per-item register_addr and raw_value for Pack Info, Temperature, Status, and Balance groups"
  - "CellAddrs is a separate uint16 array on PackGroup since cells use int array (not Items map)"
  - "Pack Status and Balance decoded items skip per-item tooltips since they represent bitmap bits, not individual registers"
  - "sectionCache usage guarded with typeof check since Plan 02 creates the cache infrastructure"

patterns-established:
  - "Pack drill-down renderers set data-register-addr, data-register-raw, data-register-time on value elements"
  - "handlePackData sets timestamps on all tooltip-enabled elements after render"

requirements-completed: [DISP-03]

# Metrics
duration: 7min
completed: 2026-04-12
---

# Phase 10 Plan 03: Pack Drill-Down Tooltip Metadata Summary

**Extended PackDataMessage with per-item register metadata and updated pack drill-down renderers to set tooltip data attributes for full D-15 compliance**

## Performance

- **Duration:** 7 min
- **Started:** 2026-04-12
- **Completed:** 2026-04-12
- **Tasks:** 3 (2 auto + 1 human-verify)
- **Files modified:** 6

## Accomplishments
- Added PackItemMeta struct (RegisterAddr uint16, RawValue string) to PackGroup for per-item tooltip metadata
- Added CellAddrs []uint16 field to PackGroup for per-cell register addresses (0x9051-0x9060)
- Populated ItemMeta in buildPackDataMessage for all 5 groups: Pack Info, Cell Voltages, Temperatures, Pack Status, Balance
- Updated renderGroupCard to set data-register-addr and data-register-raw on Pack Info and Temperature value elements
- Updated renderCellVoltageGrid to add data-row-h__value class and tooltip data attributes on cell voltage spans
- Added handlePackData timestamp setting and cache population for pack drill-down views
- Added TestPackDataMessageItemMeta verifying JSON serialization
- Browser verification confirmed tooltips work on Pack Info, cell voltages, and temperatures

## Task Commits

Each task was committed atomically:

1. **Task 1: Add PackItemMeta and populate in buildPackDataMessage** - `9f168d0` (feat)
2. **Task 2: Update frontend pack renderers** - `01f5202` (feat)
3. **Task 3: Browser verification** - approved by user

## Files Created/Modified
- `internal/hub/message.go` - Added PackItemMeta struct, ItemMeta and CellAddrs fields to PackGroup
- `internal/hub/hub.go` - Populated ItemMeta in buildPackDataMessage for all 5 pack groups
- `internal/hub/section.go` - Added FormatRawValue alias
- `internal/hub/hub_test.go` - Added TestPackDataMessageItemMeta
- `internal/register/format.go` - FormatRawValue function (created as dependency)
- `web/static/app.js` - Updated renderGroupCard, renderCellVoltageGrid, handlePackData with tooltip data attributes

## Decisions Made
- Pack Status decoded items skip individual tooltips since they represent bitmap bits from aggregated registers, not individual register reads
- Balance bitmap visualization skips per-cell tooltips since all cells come from a single register (0x9075)
- ItemMeta still sent for Status and Balance groups in JSON for potential future use

## Deviations from Plan
- FormatRawValue and section.go alias were created as part of this plan execution since Plan 01 had not been executed yet at the time. Plan 01 later added the tests.
- Added typeof sectionCache guard in handlePackData since Plan 02 creates the cache infrastructure

## Issues Encountered
None

## Next Phase Readiness
- Full D-15 compliance: every parameter value in the app gets a hover tooltip (main sections AND pack drill-down)
- Pack cache populated from pack_data messages for navigation persistence

---
*Phase: 10-data-persistence-tooltips*
*Completed: 2026-04-12*
