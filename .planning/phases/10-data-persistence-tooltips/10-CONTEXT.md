# Phase 10: Data Persistence & Tooltips - Context

**Gathered:** 2026-04-12
**Status:** Ready for planning

<domain>
## Phase Boundary

Users always see the most recent known values and can inspect register-level details on demand. Three capabilities: (1) stale value persistence during refresh cycles — values dim when refreshing and snap back as fresh data arrives, (2) section-level value caching — navigating away and back shows cached values immediately (dimmed) while a fresh read starts, (3) parameter tooltips — hover over any value to see register address (hex), raw value, and last-read timestamp.

</domain>

<decisions>
## Implementation Decisions

### Stale value display during refresh
- **D-01:** When a new read cycle starts, all values in the section dim to 50% opacity immediately. No icon, no delay — just reduced opacity. This is triggered when the browser sends `read_cycle`.
- **D-02:** As each `register_value` arrives, that individual value snaps back to full opacity. The section progressively "fills in" as reads complete.
- **D-03:** The existing `--stale` CSS class (Phase 9, error case with ⚠ icon) remains distinct from the refresh dimming. Refresh dim = 50% opacity, no icon. Error stale = dimmed + ⚠ icon.
- **D-04:** Dimming is applied at the section container level (one class toggle), not per-value.
- **D-05:** Pack drill-down views use the same dimming behavior — consistent across all views.
- **D-06:** The green flash on `section_complete` is preserved alongside the dim/un-dim transitions. Both feedback layers coexist: per-register opacity restoration + section-level completion flash.
- **D-07:** Dim/un-dim transitions use a subtle CSS fade (~150-200ms) for smooth visual effect.

### Section value caching
- **D-08:** The browser maintains an in-memory cache of last-read values, keyed per-section and per-pack (for BMS drill-down pages). Navigating back to a previously viewed section shows cached values dimmed at 50% while a fresh read cycle starts automatically.
- **D-09:** Cache granularity: per main section (system, grid, eps, pv, battery_info, stats) AND per individual pack drill-down page. Navigating from Pack 3 to Pack 5 and back to Pack 3 shows Pack 3's cached cell voltages/temps.
- **D-10:** First visit to a section (no cache) shows em-dash skeleton placeholders — current behavior preserved. Mental model: em-dash = "never read", dimmed = "read before, refreshing".
- **D-11:** Cache is preserved when switching sections with auto-refresh active. Old section values stay cached until disconnect.

### Cache lifetime
- **D-12:** Cache is cleared on disconnect. All values reset to em-dash skeletons. Clean slate signals "no active connection, no data".
- **D-13:** No manual "clear cache" button — disconnect handles cache clearing. Page reload also clears (in-memory only, no localStorage persistence for cached values).
- **D-14:** On disconnect, the content area shows em-dash skeletons (not blank, not an overlay).

### Parameter tooltips
- **D-15:** Every parameter value in the app gets a hover tooltip — main sections AND pack drill-down cell voltage/temperature values.
- **D-16:** Tooltip content: register address in hex (e.g. `0x0484`), raw register value (before scaling/formatting), and last successful read timestamp in HH:MM:SS format.
- **D-17:** Tooltip is triggered by mouse hover only. No click-to-pin, no click-to-show.
- **D-18:** Styled custom tooltip (dark theme, monospace font for register/raw values). Positioned above the value element. Appears with slight hover delay (~300ms).
- **D-19:** For error-stale values (all retries failed), tooltip shows register address and last known raw value. If no value was ever read, just shows the address.

### Claude's Discretion
- How to send register address and raw value from backend to frontend (extend `register_value` message, or embed in schema, or via data attributes)
- Internal cache data structure in JavaScript (Map, plain object, etc.)
- How to implement the custom tooltip (CSS-only with `::before`/`::after`, or JS-managed tooltip element)
- How to store per-register timestamps in the cache
- Whether to add a CSS class like `--refreshing` distinct from `--stale`, or reuse opacity-based approach

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements
- `.planning/REQUIREMENTS.md` — DISP-01, DISP-02, DISP-03 requirement definitions
- `.planning/ROADMAP.md` — Phase 10 success criteria (3 criteria)

### Current implementation (to be extended)
- `web/static/app.js` — `handleRegisterValue()` (line ~994), `handleSectionComplete()` (line ~1014), `navigateToSection()` (line ~312), `refreshState` object
- `web/static/style.css` — `.data-row-h__value--stale` (line ~579), `.data-row-h__value--pending` (line ~575)
- `internal/hub/message.go` — `NewRegisterValue()` (line ~221), `RegisterValueMessage` struct — needs register address and raw value fields
- `internal/hub/hub_streaming.go` — `streamStandardRead()`, `streamBatteryRead()` — where register_value messages are constructed with probe metadata

### Prior phase context
- `.planning/phases/08-refresh-architecture/08-CONTEXT.md` — D-01 (section switch triggers read), D-04 (cycle delay), D-09 (auto button cycle count)
- `.planning/phases/09-connection-read-resilience/09-CONTEXT.md` — D-08/D-09 (stale display on error), D-02 (SetReadDeadline for disconnect)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `data-row-h__value--stale` CSS class: Already implements dimmed text + ⚠ icon for error cases. Phase 10 needs a new `--refreshing` class (dim only, no icon) or a container-level approach.
- `data-row-h__value--pending` CSS class: Used for em-dash skeleton state. Can be preserved for first-visit.
- `handleRegisterValue()`: Already manages `--pending` and `--stale` class toggling per register. Needs extension to restore opacity on value arrival.
- `section_complete` WebSocket message: Already used to trigger green flash and schedule next cycle. Natural integration point for removing section-level dim class.
- `localStorage` pattern: Used for connection params, PV channels, timing settings. Cache does NOT use localStorage (in-memory only per D-13).

### Established Patterns
- Section schema + streaming skeleton: `section_schema` message pre-renders placeholders with `data-register` attributes. Cache restoration can populate these same elements.
- Per-register streaming: Values arrive one at a time via `register_value` messages. Each value's opacity can be restored individually as it arrives.
- CSS custom properties: Theme uses `--color-text-muted` for dimmed states. New dimming should align with this.

### Integration Points
- `navigateToSection()` in app.js: Currently re-subscribes and re-renders from scratch. Needs cache-check logic — if cache exists, populate from cache + dim, then subscribe for fresh data.
- `RegisterValueMessage` in message.go: Needs new fields for register address (uint16) and raw value (string/int) to support tooltips.
- `handleRegisterValue()` in app.js: Needs to (a) update cache, (b) store tooltip data in data attributes, (c) restore opacity per-value.
- Disconnect handler in app.js: Needs to clear the value cache and reset all values to em-dashes.

</code_context>

<specifics>
## Specific Ideas

- Tooltip format: three lines — "Register: 0x0484", "Raw: 3852", "Last read: 16:03:42"
- Dark themed tooltip with monospace font, positioned above the value
- Subtle 150-200ms CSS transition on opacity changes for smooth dim/un-dim
- Section-level dim (class on container) for cycle start, per-register un-dim as values arrive
- Green flash on section_complete preserved alongside dimming
- Em-dash = never read, dimmed = previously read (refreshing or cached), full = fresh

</specifics>

<deferred>
## Deferred Ideas

### Reviewed Todos (not folded)
- **Read delay burst on section switch** — Already fixed in Phase 8 (D-03, REL-03). Todo can be closed.
- **Skip unsupported PackInfoProbes registers** — Already implemented in Phase 8 (skipPackInfo). Todo can be closed.
- **Stream pack drill-down values per-register** — Phase 11 concern (BATT-02). Not Phase 10 scope.

</deferred>

---

*Phase: 10-data-persistence-tooltips*
*Context gathered: 2026-04-12*
