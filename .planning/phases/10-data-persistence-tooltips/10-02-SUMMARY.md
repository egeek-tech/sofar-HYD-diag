---
phase: 10-data-persistence-tooltips
plan: 02
subsystem: frontend
tags: [css, javascript, cache, dimming, tooltip, refresh, disconnect]

# Dependency graph
requires:
  - phase: 10-data-persistence-tooltips
    plan: 01
    provides: RegisterValueMessage with register_addr and raw_value JSON fields
provides:
  - CSS custom properties for tooltip and refresh dimming
  - Refresh dimming classes (.content__body--refreshing, .data-row-h__value--fresh)
  - Tooltip CSS (.param-tooltip with arrow and --below modifier)
  - Section cache system (sectionCache Map, getCacheKey, updateCache, restoreFromCache)
  - Tooltip system (initTooltip, showTooltip, hideTooltip with event delegation)
  - applyRefreshDimming helper for container-level dimming
  - Disconnect cleanup (cache clear, DOM reset, tooltip hide)
affects: [10-03-PLAN, browser-ux]

# Tech tracking
tech-stack:
  added: []
  patterns: [container-level-dimming, per-register-fresh-restore, js-managed-tooltip, section-cache-map]

key-files:
  created: []
  modified:
    - web/static/style.css
    - web/static/app.js

key-decisions:
  - "Container-level dimming via .content__body--refreshing class, per-register restore via .data-row-h__value--fresh with !important override"
  - "Section cache uses Map<string, Map<string, CacheEntry>> with composite keys for pack drill-down (bms:pack:input:tower:pack)"
  - "Tooltip uses position:fixed with getBoundingClientRect for viewport-relative positioning, flips below if insufficient space above"
  - "Event delegation with useCapture on mouseenter/mouseleave for dynamically-created value elements"
  - "Composed values (addr=0x0000) omit Register and Raw lines in tooltip, showing only Last read timestamp"

patterns-established:
  - "applyRefreshDimming removes all --fresh classes before adding --refreshing container class (Pitfall 2 prevention)"
  - "Cache restored in handleSectionSchema after DOM build, not in navigateToSection (Pitfall 1 prevention)"
  - "Tooltip hidden on section navigation and disconnect (Pitfall 5 prevention)"

requirements-completed: [DISP-01, DISP-02, DISP-03]

# Metrics
duration: 3min
completed: 2026-04-12
---

# Phase 10 Plan 02: Frontend Dimming, Caching, and Tooltips Summary

**Implemented CSS refresh dimming, section value cache, parameter tooltip system, and disconnect cleanup for all three DISP requirements**

## Performance

- **Duration:** 3 min
- **Started:** 2026-04-12T17:42:33Z
- **Completed:** 2026-04-12T17:45:40Z
- **Tasks:** 2 of 3 (Task 3 is human-verify checkpoint)
- **Files modified:** 2

## Accomplishments

- Added 11 CSS custom properties to :root for tooltip styling and refresh dimming configuration
- Added .content__body--refreshing container-level dimming class (opacity 0.5 with 150ms transition)
- Added .data-row-h__value--fresh per-register opacity restore class (!important override)
- Added .param-tooltip CSS with fixed positioning, dark theme, arrow indicator, and --below modifier
- Implemented sectionCache Map with getCacheKey (supports per-pack composite keys), updateCache, and restoreFromCache
- Implemented applyRefreshDimming helper that clears previous --fresh classes and applies container dim
- Implemented tooltip system with initTooltip (event delegation via useCapture), showTooltip (viewport clamping, flip-below), hideTooltip
- Updated handleRegisterValue with: hex addr formatting, timestamp, fresh class, tooltip data attrs, cache update
- Updated handleSectionComplete to remove --refreshing class on cycle end
- Updated handleSectionSchema to restore from cache after skeleton DOM build
- Added applyRefreshDimming calls at all three read_cycle send points (auto toggle, manual refresh, auto-refresh timer)
- Updated disconnect handler to clear cache, reset all values to em-dash pending, remove tooltip data attrs, hide tooltip
- Added tooltip hide on section navigation (Pitfall 5 prevention)
- Preserved existing 10-03 pack drill-down tooltip data attribute code (renderGroupCard, renderCellVoltageGrid)
- Go build passes with embedded frontend

## Task Commits

Each task was committed atomically:

1. **Task 1: CSS custom properties, dimming classes, tooltip styles** - `f35d4b1` (feat)
2. **Task 2: Section cache, refresh dimming, tooltip system in app.js** - `f49baac` (feat)

## Files Created/Modified

- `web/static/style.css` - Added Phase 10 custom properties, refresh dimming classes, tooltip CSS (61 lines added)
- `web/static/app.js` - Added cache system, tooltip system, dimming logic, disconnect cleanup (212 lines added)

## Decisions Made

- Container-level dimming with per-register fresh restore matches D-01/D-02/D-04 exactly
- Cache key uses section name for main sections, composite "bms:pack:input:tower:pack" for pack drill-down per D-09
- Tooltip uses position:fixed (not absolute) since it is appended to body and positioned via getBoundingClientRect
- Event delegation with useCapture on content-body handles dynamically-created elements without re-binding

## Deviations from Plan

None - plan executed exactly as written. All 8 modifications applied as specified.

## Issues Encountered

None

## User Setup Required

None

## Awaiting Verification

Task 3 is a human-verify checkpoint requiring browser testing of all three DISP requirements (dimming, caching, tooltips).

---
*Phase: 10-data-persistence-tooltips*
*Completed: 2026-04-12 (Tasks 1-2; Task 3 pending human verification)*
