# Phase 8: Refresh Architecture - Context

**Gathered:** 2026-04-12
**Status:** Ready for planning

<domain>
## Phase Boundary

Move auto-refresh from backend timer (`time.Ticker` in section.go) to browser-driven request cycle. The browser controls when read cycles happen; the backend becomes purely reactive. Inter-read delay enforcement remains in the broker but section-level timing moves to JavaScript. Also fix read delay burst on section switch (REL-03) and skip unsupported PackInfoProbes registers (0x9104-0x9126).

</domain>

<decisions>
## Implementation Decisions

### Section switch behavior
- **D-01:** When navigating to a new section, the browser immediately requests a read for the new section (preserves D-20 responsiveness).
- **D-02:** Any in-progress read cycle for the old section is aborted by cancelling remaining probes. The current individual register read is allowed to finish, but subsequent probes in the cycle are skipped.
- **D-03:** Inter-read delay is always enforced, even across section switches. If a probe from the old section just completed, the delay must elapse before the first probe of the new section starts. This directly addresses REL-03.

### Refresh cycle delay
- **D-04:** After a read cycle completes (`section_complete`), the browser waits a configurable delay before requesting the next cycle.
- **D-05:** Cycle delay is configured via a dropdown with presets: 0s (Continuous), 5s, 10s, 30s.
- **D-06:** Default cycle delay is 0s (Continuous) -- next cycle starts immediately after the previous one finishes. Inter-read delay between individual probes is still enforced by the broker.
- **D-07:** Cycle delay selection persists in localStorage across page reloads.
- **D-08:** Cycle delay dropdown is placed inline next to the auto-refresh button in the header area.

### Auto-refresh button UX
- **D-09:** Auto-refresh button shows "Auto (#N)" when active, where N is a live cycle count that increments after each completed cycle.
- **D-10:** Cycle count resets to #1 when switching sections.
- **D-11:** When auto-refresh is off, a manual "Refresh" button is shown to trigger a single read cycle on demand.

### Backend timer removal
- **D-12:** All backend `time.Ticker` timers in section.go are removed. The backend performs no autonomous refresh cycles (REFR-01). All reads are initiated by browser WebSocket messages.
- **D-13:** Stopping auto-refresh in the browser immediately stops all Modbus reads -- no orphaned backend timer continues reading (success criterion 4).

### Claude's Discretion
- WebSocket message format for the new "read now" / "cancel read" protocol
- How to implement probe cancellation (context, channel signal, or flag check between probes)
- Whether `subscribe` and `read` become separate messages or subscribe implies first read
- Internal broker changes for cancellation support

### Folded Todos
- **Read delay burst on section switch** -- The enforceInterReadDelay timing bug that causes rapid reads when switching sections. Directly addressed by D-03 and REL-03.
- **Skip unsupported PackInfoProbes registers (0x9104-0x9126)** -- These registers return illegal address on this BMS hardware. Should be skipped during pack read cycles to avoid unnecessary errors and delays.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

No external specs -- requirements fully captured in decisions above and the following project files:

### Requirements and roadmap
- `.planning/REQUIREMENTS.md` -- REL-03, REFR-01, REFR-02 requirement definitions
- `.planning/ROADMAP.md` -- Phase 8 success criteria (4 criteria)

### Current implementation (to be refactored)
- `internal/hub/section.go` -- Current backend timer implementation (time.Ticker, startTimer, stopTimer)
- `internal/hub/hub.go` -- handleTimerTick, handleAutoRefreshToggle, timerCh channel
- `internal/broker/broker.go` -- enforceInterReadDelay, inter-read delay enforcement
- `web/static/app.js` -- Current subscribe/auto_refresh WebSocket protocol, App.autoRefresh state

### Todos (folded into scope)
- `.planning/todos/pending/read-delay-burst-on-section-switch.md` -- Details on the timing burst bug
- `.planning/todos/pending/skip-unsupported-pack-info-registers.md` -- PackInfoProbes register skip details

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `section_complete` WebSocket message: Already sent by backend after each read cycle -- browser can use this as the trigger to start its delay timer
- `enforceInterReadDelay()` in broker.go: Per-register delay enforcement stays as-is, already tracks lastReadTime
- `App.autoRefresh` in app.js: Boolean state for toggle, already wired to the button

### Established Patterns
- WebSocket message types defined in `message.go` (MsgTypeSubscribe, MsgTypeAutoRefresh, etc.) -- new message types follow same pattern
- Section subscriber model (addSubscriber/removeSubscriber) -- subscription stays, timer logic removed
- localStorage persistence for PV channels and read delay -- cycle delay follows same pattern

### Integration Points
- `hub.go` Run() select loop: Remove timerCh case, add new "read request" message handling
- `section.go`: Remove ticker/timer fields and methods, keep subscriber tracking
- `broker.go`: Add cancellation support for in-progress read cycles (cancel remaining probes)
- `app.js`: Replace server-driven auto-refresh with client-side cycle management (setTimeout after section_complete)
- `index.html`: Add cycle delay dropdown next to auto-refresh button

</code_context>

<specifics>
## Specific Ideas

- Auto button shows "Auto (#3)" with live cycle count -- visual heartbeat that refresh is working
- Manual "Refresh" button when auto is off for on-demand single reads
- Cycle delay dropdown with presets (0s, 5s, 10s, 30s) next to the auto button -- not a slider
- Default 0s (Continuous) because this is a diagnostic tool where you typically want fresh data
- Abort old section reads on switch by cancelling remaining probes (not hard TCP abort)

</specifics>

<deferred>
## Deferred Ideas

None -- discussion stayed within phase scope.

</deferred>

---

*Phase: 08-refresh-architecture*
*Context gathered: 2026-04-12*
