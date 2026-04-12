# Phase 8: Refresh Architecture - Research

**Researched:** 2026-04-12
**Domain:** WebSocket protocol redesign, Go concurrency (context cancellation), browser-driven refresh timing
**Confidence:** HIGH

## Summary

Phase 8 moves auto-refresh control from the Go backend (time.Ticker in section.go) to the browser. Currently, each Section owns a `time.Ticker` goroutine that fires every 10s, sending the section name on `timerCh` which triggers `handleTimerTick` in the hub event loop. The refactoring eliminates all backend timers and makes the hub purely reactive: the browser sends a "read now" message, the hub executes the read cycle, sends `section_complete`, and the browser decides when to request the next cycle.

The key challenge is probe-level cancellation on section switch. Currently, streaming reads (`streamStandardRead`, `streamBatteryRead`, `streamBMSRead`) run in goroutines that iterate through probes calling `broker.ReadRegisters` one at a time. There is no mechanism to abort mid-cycle. Adding `context.Context` cancellation to these goroutines is the cleanest approach -- each streaming goroutine gets a cancellable context that is cancelled when the section changes.

Additionally, the inter-read delay burst bug (REL-03) occurs because `enforceInterReadDelay` in the broker checks `time.Since(lastReadTime)`, and after navigation time elapses, the first several reads in the new section skip the delay. This is fixed by ensuring the broker's `lastReadTime` is reset when a new read cycle starts, or by the natural enforcement that now each read cycle respects the delay since the browser controls timing.

**Primary recommendation:** Replace section.go timer machinery with a per-section cancellable context for streaming reads; add new WebSocket message types `read_cycle` and `cancel_read`; move auto-refresh state machine entirely to app.js using `section_complete` as the trigger for `setTimeout`.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** When navigating to a new section, the browser immediately requests a read for the new section (preserves D-20 responsiveness).
- **D-02:** Any in-progress read cycle for the old section is aborted by cancelling remaining probes. The current individual register read is allowed to finish, but subsequent probes in the cycle are skipped.
- **D-03:** Inter-read delay is always enforced, even across section switches. If a probe from the old section just completed, the delay must elapse before the first probe of the new section starts. This directly addresses REL-03.
- **D-04:** After a read cycle completes (`section_complete`), the browser waits a configurable delay before requesting the next cycle.
- **D-05:** Cycle delay is configured via a dropdown with presets: 0s (Continuous), 5s, 10s, 30s.
- **D-06:** Default cycle delay is 0s (Continuous) -- next cycle starts immediately after the previous one finishes. Inter-read delay between individual probes is still enforced by the broker.
- **D-07:** Cycle delay selection persists in localStorage across page reloads.
- **D-08:** Cycle delay dropdown is placed inline next to the auto-refresh button in the header area.
- **D-09:** Auto-refresh button shows "Auto (#N)" when active, where N is a live cycle count that increments after each completed cycle.
- **D-10:** Cycle count resets to #1 when switching sections.
- **D-11:** When auto-refresh is off, a manual "Refresh" button is shown to trigger a single read cycle on demand.
- **D-12:** All backend `time.Ticker` timers in section.go are removed. The backend performs no autonomous refresh cycles (REFR-01). All reads are initiated by browser WebSocket messages.
- **D-13:** Stopping auto-refresh in the browser immediately stops all Modbus reads -- no orphaned backend timer continues reading (success criterion 4).

### Claude's Discretion
- WebSocket message format for the new "read now" / "cancel read" protocol
- How to implement probe cancellation (context, channel signal, or flag check between probes)
- Whether `subscribe` and `read` become separate messages or subscribe implies first read
- Internal broker changes for cancellation support

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope.

### Folded Todos (IN SCOPE)
- Read delay burst on section switch (REL-03) -- addressed by D-03
- Skip unsupported PackInfoProbes registers (0x9104-0x9126) -- skip on illegal address error
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REL-03 | Inter-read delay is consistently enforced between all Modbus reads, with no burst of rapid reads on section switch | Cancellation architecture ensures old cycle finishes cleanly; `lastReadTime` in broker preserves timing across cycles; D-03 requires delay enforcement even across section boundaries |
| REFR-01 | Auto-refresh is triggered only by the browser -- backend performs no autonomous refresh cycles | Section timer removal pattern; new `read_cycle` message type replaces timer-driven reads |
| REFR-02 | Auto-refresh timer restarts after each read cycle completes (not on a fixed interval) | Browser-side `setTimeout` triggered by `section_complete` message; no fixed `setInterval` |
</phase_requirements>

## Standard Stack

No new libraries needed. This phase refactors existing Go stdlib and vanilla JS code.

### Core (unchanged)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `context` | Go 1.26.1 | Cancellable goroutine contexts for streaming reads | [VERIFIED: go.mod] Already used throughout broker; standard Go cancellation pattern |
| Go stdlib `sync/atomic` | Go 1.26.1 | Atomic flags for read-in-progress tracking | [VERIFIED: section.go] Already in use (`reading atomic.Bool`) |
| Go stdlib `time` | Go 1.26.1 | `time.Since` for inter-read delay enforcement | [VERIFIED: broker.go] Already in use; timers being REMOVED from section.go |
| Vanilla JS `setTimeout` | Browser | Client-side cycle delay between read completions | [VERIFIED: app.js] Already used for WS reconnect; no framework dependencies |
| Vanilla JS `localStorage` | Browser | Persist cycle delay preference | [VERIFIED: app.js] Already used for connection settings, PV channels, timing config |

## Architecture Patterns

### Current Architecture (being replaced)

```
Browser                     Hub (Go)                    Broker (Go)
  |                           |                           |
  |-- subscribe ------------->|                           |
  |                           |-- startTimer() --------->.|
  |                           |   (time.Ticker 10s)       |
  |                           |                           |
  |                           |<-- timerCh tick ----------|
  |                           |-- triggerSectionRead()--->|
  |                           |   go streamStandardRead() |
  |                           |         ReadRegisters()-->|
  |<-- register_value --------|<-- result ----------------|
  |<-- section_complete ------|                           |
  |                           |   ...wait 10s...          |
  |                           |<-- timerCh tick ----------|
```

### Target Architecture (browser-driven)

```
Browser                     Hub (Go)                    Broker (Go)
  |                           |                           |
  |-- subscribe ------------->| (no timer started)        |
  |-- read_cycle ------------>|                           |
  |                           |-- triggerSectionRead()--->|
  |                           |   go streamRead(ctx)      |
  |                           |         ReadRegisters()-->|
  |<-- register_value --------|<-- result ----------------|
  |<-- section_complete ------|                           |
  |                           |                           |
  | (setTimeout cycleDelay)   |                           |
  |-- read_cycle ------------>|                           |
  |                           |   ...next cycle...        |
```

### Pattern 1: Per-Section Cancellable Context [RECOMMENDED]

**What:** Each section maintains a `context.CancelFunc` that is called when the section's read cycle should be aborted (on section switch or auto-refresh stop). The streaming goroutine checks `ctx.Err()` between each probe read.

**When to use:** Always -- this is the recommended approach for D-02 (cancel remaining probes).

**Why context over channel/flag:**
- The streaming goroutines already check `h.ctx.Err()` between probes [VERIFIED: hub_streaming.go lines 47, 164, 248]
- `broker.ReadRegisters` already accepts a `context.Context` and respects cancellation [VERIFIED: broker.go lines 144-164]
- A per-section child context automatically cascades to the broker call, so a read-in-progress also respects the timeout (though D-02 says let current read finish -- see implementation note below)
- No new primitives needed; standard Go pattern

**Implementation approach:**
```go
// In Section struct, replace timer fields with:
type Section struct {
    // ... existing fields (Name, Probes, Groups, subscribers, etc.) ...
    readCancel context.CancelFunc  // cancels the current streaming read goroutine
    reading    atomic.Bool          // true while a read is in progress (keep existing)
    // REMOVE: ticker, stopCh, timerCh, interval, autoRefresh
}

// When starting a read cycle:
func (h *Hub) triggerSectionRead(sectionName string) {
    sec := h.sections[sectionName]
    
    // Cancel any in-progress read for this section
    if sec.readCancel != nil {
        sec.readCancel()
        sec.readCancel = nil
    }
    
    sec.reading.Store(true)
    readCtx, cancel := context.WithCancel(h.ctx)
    sec.readCancel = cancel
    
    // Pass readCtx to streaming goroutine
    h.streamStandardRead(sectionName, sec, readCtx)
}
```

**D-02 compliance (let current read finish):** The `broker.ReadRegisters` call is a synchronous channel send into the broker's command queue. Once a read command enters the broker's `Run()` loop, it executes to completion (the broker does not check context mid-read). The context cancellation is checked BETWEEN probe reads in the streaming goroutine's loop, which means the current individual register read finishes but subsequent probes are skipped. This naturally satisfies D-02 without any broker changes. [VERIFIED: broker.go `execute()` method processes one command at a time]

### Pattern 2: Subscribe Implies First Read

**What:** `subscribe` message continues to trigger an immediate read (D-01/D-20 compatibility), AND the browser sends `read_cycle` for subsequent cycles. No need to split into two separate "subscribe" and "first read" messages.

**When to use:** Always -- preserves backward compatibility with existing subscribe flow.

**Implementation:**
- `subscribe` handler calls `triggerSectionRead()` as it does now (D-20)
- New `read_cycle` handler also calls `triggerSectionRead()`
- Browser's auto-refresh loop: on `section_complete`, wait `cycleDelay`, then send `read_cycle`

### Pattern 3: Browser-Side Auto-Refresh State Machine

**What:** The browser maintains cycle count, delay timer, and auto-refresh toggle. The `section_complete` message is the only trigger for scheduling the next cycle.

**State transitions:**
```
IDLE ----[user clicks Auto]----> WAITING_FOR_COMPLETE
WAITING_FOR_COMPLETE ----[section_complete received]----> DELAY_WAIT
DELAY_WAIT ----[setTimeout fires]----> SEND_READ_CYCLE --> WAITING_FOR_COMPLETE
WAITING_FOR_COMPLETE ----[user clicks Auto off]----> IDLE
WAITING_FOR_COMPLETE ----[section switch]----> cancel + reset counter + SEND_READ_CYCLE (for new section)
```

**Implementation sketch:**
```javascript
var CYCLE_DELAY_KEY = 'sofar_cycle_delay';
var CYCLE_DELAY_PRESETS = [0, 5000, 10000, 30000];

var refreshState = {
    active: false,
    cycleCount: 0,
    delayTimer: null,
    cycleDelay: 0  // default: Continuous (D-06)
};

function handleSectionComplete(msg) {
    if (msg.section !== App.activeSection) return;
    
    // Update timestamp display (existing)
    // ...
    
    refreshState.cycleCount++;
    updateAutoRefreshButton();
    
    if (refreshState.active) {
        // Schedule next cycle after configured delay
        refreshState.delayTimer = setTimeout(function() {
            App.ws.send({ type: 'read_cycle', section: App.activeSection });
        }, refreshState.cycleDelay);
    }
}

function stopAutoRefresh() {
    refreshState.active = false;
    if (refreshState.delayTimer) {
        clearTimeout(refreshState.delayTimer);
        refreshState.delayTimer = null;
    }
    // D-13: No need to tell backend -- backend has no timer to stop
}
```

### Anti-Patterns to Avoid

- **Do NOT use `setInterval`:** The requirement (REFR-02) explicitly says timer restarts AFTER cycle completes. `setInterval` would create fixed-interval timing that doesn't account for read cycle duration.
- **Do NOT keep backend auto_refresh message type for timer control:** The `auto_refresh` message type currently toggles `sec.autoRefresh` and starts/stops timers. With timers removed, this message type becomes unnecessary. Remove it or repurpose it for informational logging only.
- **Do NOT cancel the broker-level context:** D-02 says let the current individual read finish. Cancel the streaming goroutine's context, not the broker's context. The broker processes one command at a time and should not be interrupted mid-read.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Goroutine cancellation | Custom done channel with select | `context.WithCancel` | Standard Go pattern; already used by broker; cascades through call chain |
| Timer scheduling in browser | Custom Date.now() polling loop | `setTimeout` / `clearTimeout` | Native browser API, exact semantics needed (fire once after delay) |
| State persistence in browser | Cookie-based storage | `localStorage.getItem/setItem` | Already used for 3 other settings in this codebase |

**Key insight:** The entire timer infrastructure in section.go (ticker goroutine, stopCh channel, timerCh forwarding) is being DELETED, not refactored. The replacement is simpler: a single `context.CancelFunc` per section and `setTimeout` in the browser.

## Common Pitfalls

### Pitfall 1: Orphaned Goroutine on Rapid Section Switching
**What goes wrong:** User clicks 3 sections quickly. If the cancellation doesn't propagate correctly, 3 streaming goroutines run concurrently, all writing to the results channel.
**Why it happens:** `triggerSectionRead` spawns a goroutine. If the previous one isn't cancelled before starting a new one, they overlap.
**How to avoid:** Always cancel the previous section's `readCancel` before spawning a new read goroutine. The `sec.reading.Store(true)` / `defer sec.reading.Store(false)` pattern already exists but is per-section; with the new subscribe-unsubscribe flow (D-18: one section at a time), the old section's context gets cancelled when the client unsubscribes.
**Warning signs:** Multiple `section_complete` messages arriving for the same section, or results from wrong section appearing.

### Pitfall 2: Inter-Read Delay Not Enforced Across Section Switch (REL-03)
**What goes wrong:** Old section's last probe finishes, browser immediately sends `read_cycle` for new section, and the new section's first probe fires without waiting for inter-read delay.
**Why it happens:** The broker's `lastReadTime` is set when a read completes. If the browser sends `read_cycle` immediately and the hub dispatches a new streaming goroutine, the first `ReadRegisters` call checks `enforceInterReadDelay` which looks at `lastReadTime` -- this correctly enforces the delay as long as `lastReadTime` was set by the previous read.
**How to avoid:** Verify that `lastReadTime` persists across section switches (it does -- it's on the broker, not the section). The existing `enforceInterReadDelay()` in broker.go naturally handles this. [VERIFIED: broker.go line 387 sets `lastReadTime` after every read, line 493 checks it before every read]
**Warning signs:** Reads appearing faster than 500ms apart in broker logs right after section switch.

### Pitfall 3: SetDelayRuntime Race with New Section Read
**What goes wrong:** The `handleConfigure` for timing sends `SetDelayRuntime` in a `go func()` goroutine. If a section read starts before this command is processed by the broker, the old delay is used.
**Why it happens:** The goroutine dispatch means the `SetDelayRuntime` command and the first `ReadRegisters` command race into the broker's command channel.
**How to avoid:** This is the timing bug mentioned in the todo. Either make `SetDelayRuntime` synchronous (block until broker acknowledges) or accept that the first read may use the old delay. Since the user's CONTEXT.md doesn't specifically address this, and the inter-read delay is already correct within a cycle, this is a cosmetic issue. However, the todo explicitly identifies this as a fix target, and it's folded into scope.
**Warning signs:** First read after changing delay uses old delay value.

### Pitfall 4: Browser Timer Leak on Section Switch
**What goes wrong:** Browser has a pending `setTimeout` for the next read cycle. User switches sections. The old timer fires and sends `read_cycle` for the wrong section (or the old section).
**Why it happens:** `setTimeout` wasn't cleared on section switch.
**How to avoid:** `navigateToSection` must call `clearTimeout(refreshState.delayTimer)` before any other action. Reset `cycleCount` to 0 (D-10 says reset to #1, meaning first completed cycle shows #1).
**Warning signs:** Unexpected `read_cycle` messages arriving for non-active sections.

### Pitfall 5: PackInfoProbes Error Spam
**What goes wrong:** Every pack drill-down cycle reads 0x9104-0x9126, gets illegal address error (0x02), logs ERROR, wastes ~2s on retry.
**Why it happens:** BMS hardware doesn't support these extended registers. The code reads them unconditionally.
**How to avoid:** Track a session-level flag (on the Hub or a package-level atomic bool). On first 0x02 error for 0x9104, set flag to skip PackInfoProbes for the rest of the session. Or simply skip them proactively since the todo confirms they don't work on this hardware.
**Warning signs:** Repeated `exception: func=0x83 err=0x02` in logs during pack reads.

### Pitfall 6: section_complete Not Sent on Cancelled Read
**What goes wrong:** Browser is waiting for `section_complete` to schedule the next cycle. The old section's read was cancelled, so `section_complete` is never sent. The browser's auto-refresh stalls.
**Why it happens:** The `defer` in the streaming goroutine only sends `section_complete` at the end of the normal flow. If the context is cancelled and the goroutine returns early, it may skip the completion message.
**How to avoid:** The NEW section's read cycle will eventually send its own `section_complete`. The browser should reset its waiting state on section switch (clear any pending timer, reset cycle count). Alternatively, send a cancellation-aware `section_complete` (or just don't -- the new section's subscribe triggers a new read which will eventually complete).
**Warning signs:** Auto-refresh stops working after switching sections.

## Code Examples

### Example 1: Section Struct After Refactoring

```go
// Source: derived from section.go [VERIFIED: current codebase]
type Section struct {
    Name         string
    Probes       []register.Probe
    Groups       []register.ProbeGroup
    faultSection bool
    subscribers  map[*Client]bool
    readCancel   context.CancelFunc  // NEW: cancels current read goroutine
    reading      atomic.Bool          // KEEP: true while read in progress
    logger       *slog.Logger
    // REMOVED: autoRefresh, ticker, stopCh, timerCh, interval
}
```

### Example 2: New WebSocket Message Types

```go
// Source: derived from message.go patterns [VERIFIED: current codebase]
const (
    MsgTypeReadCycle  = "read_cycle"   // browser -> server: start a read cycle
    MsgTypeCancelRead = "cancel_read"  // browser -> server: abort in-progress read
    // MsgTypeAutoRefresh is REMOVED (or kept for backward compat but ignored)
)
```

### Example 3: Hub Run() Loop Changes

```go
// Source: derived from hub.go Run() [VERIFIED: current codebase]
// In the Run() select loop:
// REMOVE: case sectionName := <-h.timerCh: h.handleTimerTick(sectionName)
// The timerCh field and handleTimerTick method are deleted entirely.
// New command handler added to handleCommand:
case MsgTypeReadCycle:
    h.triggerSectionRead(msg.Section)
```

### Example 4: Cancellation-Aware Streaming Read

```go
// Source: derived from hub_streaming.go streamStandardRead [VERIFIED: current codebase]
func (h *Hub) streamStandardRead(sectionName string, sec *Section, readCtx context.Context) {
    go func() {
        defer sec.reading.Store(false)
        
        for _, g := range groups {
            for _, p := range g.Probes {
                // Check cancellation BEFORE each probe read (D-02)
                if readCtx.Err() != nil {
                    return  // cancelled -- exit without sending section_complete
                }
                
                // Uses readCtx (not h.ctx) so cancellation is per-section
                data, err := h.broker.ReadRegisters(readCtx, p.Addr, p.Count)
                // ... process and send register_value ...
            }
        }
        
        // Only send section_complete for non-cancelled reads
        h.results <- sectionResult{
            section: sectionName,
            msg:     NewSectionComplete(sectionName),
        }
    }()
}
```

### Example 5: Browser Cycle Delay Dropdown (HTML)

```html
<!-- Source: derived from index.html pattern for pv-channel-select [VERIFIED: current codebase] -->
<select id="cycle-delay-select" class="cycle-delay-select" style="display:none;">
    <option value="0" selected>Continuous</option>
    <option value="5000">5s</option>
    <option value="10000">10s</option>
    <option value="30000">30s</option>
</select>
<button id="btn-auto-refresh" class="btn-auto-refresh" style="display:none;">Auto</button>
<button id="btn-refresh" class="btn-refresh" style="display:none;">Refresh</button>
```

### Example 6: Skip PackInfoProbes on Illegal Address

```go
// Source: design pattern for folded todo [ASSUMED]
// In triggerPackRead, after reading RT data successfully:
infoResults := h.broker.ReadBatch(h.ctx, infoReads)
if len(infoResults) > 0 && infoResults[0].Err != nil {
    errStr := infoResults[0].Err.Error()
    if strings.Contains(errStr, "0x02") || strings.Contains(errStr, "illegal") {
        h.logger.Debug("PackInfoProbes not supported, skipping", "error", errStr)
        h.skipPackInfo = true  // session-level flag
        // Continue building response without info data
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Backend `time.Ticker` drives refresh | Browser `setTimeout` after `section_complete` | Phase 8 (this phase) | Backend becomes purely reactive; timing controlled by client |
| `auto_refresh` WS message toggles backend timer | Auto-refresh is pure client state | Phase 8 (this phase) | No backend auto_refresh concept; simpler hub code |
| No read cancellation | `context.WithCancel` per-section read | Phase 8 (this phase) | Section switch stops old reads; consistent timing |

**Deprecated/outdated after this phase:**
- `Section.ticker`, `Section.stopCh`, `Section.timerCh`, `Section.interval` -- all deleted
- `Section.autoRefresh` -- moved to browser-only state
- `Section.startTimer()`, `Section.stopTimer()`, `Section.pauseTimer()`, `Section.resumeTimer()` -- all deleted
- `Section.SetInterval()` -- deleted (no timer to configure)
- `Hub.timerCh` channel -- deleted (no timers sending on it)
- `Hub.handleTimerTick()` -- deleted
- `Hub.handleAutoRefreshToggle()` -- deleted or simplified
- `Hub.refreshOverride` -- deleted (was for testing timer interval)
- `defaultRefreshInterval` constant -- deleted
- `MsgTypeAutoRefresh` -- can be removed (no backend handler needed)

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `broker.ReadRegisters` will return promptly on context cancellation because the cancel propagates to the `select` on the command channel | Architecture Patterns | If the broker is mid-TCP-read and ignores context, cancellation could be delayed by up to 10s (read deadline). LOW risk -- D-02 says let current read finish anyway. |
| A2 | Skip PackInfoProbes via session-level flag based on 0x02 error detection | Code Examples | If error message format changes between broker versions, the string match might miss it. LOW risk -- can use error type instead. |
| A3 | The `handleTimerTick` for BMS selectedPack re-triggering needs equivalent in new architecture | Architecture Patterns | If missed, pack drill-down auto-refresh breaks. MEDIUM risk -- must ensure `read_cycle` for BMS with `selectedPack` still triggers pack read. |

## Open Questions (RESOLVED)

1. **Should `auto_refresh` WS message type be removed or kept?**
   - What we know: With timers removed from backend, `handleAutoRefreshToggle` has nothing to toggle. Browser handles all auto-refresh state.
   - What's unclear: Whether to keep the message type for logging/observability, or remove it entirely to simplify.
   - Recommendation: Remove it. Less code, less confusion. The `read_cycle` / `cancel_read` messages provide clear observability.

2. **How should pack drill-down auto-refresh work in the new model?**
   - What we know: Currently, `handleTimerTick` checks `selectedPack` and reroutes to `triggerPackRead` instead of `triggerSectionRead`. The browser doesn't know about this.
   - What's unclear: Should the browser send `read_cycle` with a pack selection, or should the hub remember the selected pack and auto-route?
   - Recommendation: Hub remembers `selectedPack` (as it does now). When browser sends `read_cycle` for section "bms" and `selectedPack` is set, hub calls `triggerPackRead`. Same logic, just triggered by browser message instead of timer tick.

3. **Should `cancel_read` be a distinct message or implicit in `subscribe`?**
   - What we know: `subscribe` already unsubscribes from previous section (D-18). The unsubscribe path could cancel the read context.
   - What's unclear: Whether a separate `cancel_read` message adds value for explicit "stop reading but stay subscribed."
   - Recommendation: Make `cancel_read` implicit in `subscribe` (unsubscribing cancels the read). A separate `cancel_read` is only needed if the user wants to stop auto-refresh without switching sections -- but since auto-refresh is browser-side, simply not sending `read_cycle` achieves this. D-13 is satisfied because no backend timer exists.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (stdlib) |
| Config file | None needed (Go conventions) |
| Quick run command | `go test ./internal/hub/ -run TestAutoRefresh -count=1 -timeout 120s` |
| Full suite command | `go test ./... -timeout 120s` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REL-03 | Inter-read delay enforced across section switch | integration | `go test ./internal/hub/ -run TestInterReadDelayAcrossSectionSwitch -count=1` | Wave 0 |
| REFR-01 | No backend autonomous refresh (timers removed) | unit | `go test ./internal/hub/ -run TestNoBackendTimer -count=1` | Wave 0 |
| REFR-02 | Refresh restarts after cycle complete (browser-side) | manual | Manual browser test: verify setTimeout fires after section_complete | manual-only (JS, no test framework) |
| D-02 | Cancel remaining probes on section switch | integration | `go test ./internal/hub/ -run TestCancelReadOnSectionSwitch -count=1` | Wave 0 |
| D-09 | Auto button shows "Auto (#N)" | manual | Manual browser test: check button text updates | manual-only |
| D-11 | Manual Refresh button when auto off | manual | Manual browser test: verify button swap | manual-only |
| D-13 | Stopping auto-refresh stops reads immediately | integration | `go test ./internal/hub/ -run TestStopAutoRefreshStopsReads -count=1` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/hub/ -count=1 -timeout 120s`
- **Per wave merge:** `go test ./... -timeout 120s`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/hub/hub_test.go` -- add `TestInterReadDelayAcrossSectionSwitch` (REL-03)
- [ ] `internal/hub/hub_test.go` -- add `TestNoBackendTimer` (REFR-01): verify Section has no ticker after refactoring
- [ ] `internal/hub/hub_test.go` -- add `TestCancelReadOnSectionSwitch` (D-02): mock broker with delay, subscribe to section A, subscribe to section B, verify section A's read was cancelled
- [ ] `internal/hub/hub_test.go` -- add `TestStopAutoRefreshStopsReads` (D-13): verify no reads after cancel
- [ ] `internal/hub/hub_test.go` -- update existing `TestAutoRefreshTimer`, `TestAutoRefreshToggleStopsTimer`, `TestTimerPausesOnDisconnect`, `TestTimerResumesOnReconnect`, `TestSkipOverlappingTick` to reflect new timer-less architecture (most will be replaced/removed)
- [ ] `internal/hub/hub_test.go` -- add `TestReadCycleMessage` to verify new message type handling

### Existing Tests That Must Be Updated
| Test | Current Behavior | New Behavior |
|------|-----------------|--------------|
| `TestAutoRefreshTimer` | Verifies timer fires and triggers reads | Must verify that without timer, `read_cycle` message triggers reads |
| `TestSkipOverlappingTick` | Verifies overlapping timer ticks are skipped | May be deleted or replaced with overlapping `read_cycle` test |
| `TestTimerPausesOnDisconnect` | Verifies timer stops on disconnect | Simplified: no timer to pause; reads just fail if disconnected |
| `TestTimerResumesOnReconnect` | Verifies timer restarts on reconnect | Simplified: no timer to resume; browser re-sends `read_cycle` |
| `TestAutoRefreshToggleStopsTimer` | Verifies toggle stops timer | Replaced: no toggle needed on backend |
| `TestManualRefresh` | Verifies `refresh` message triggers read | May be kept; `refresh` and `read_cycle` serve similar purpose |

## Security Domain

Not applicable for this phase. No new inputs from external users, no authentication changes, no new data paths. The WebSocket protocol changes are internal between the embedded frontend and the Go backend running on localhost. The Modbus connection assumes a trusted network (documented in CLAUDE.md cross-cutting concerns).

## Sources

### Primary (HIGH confidence)
- `internal/hub/section.go` -- Current timer implementation (startTimer, stopTimer, ticker, autoRefresh)
- `internal/hub/hub.go` -- Hub event loop (timerCh, handleTimerTick, handleAutoRefreshToggle)
- `internal/hub/hub_streaming.go` -- Streaming read goroutines (streamStandardRead, streamBatteryRead, streamBMSRead)
- `internal/broker/broker.go` -- Inter-read delay enforcement (enforceInterReadDelay, lastReadTime), command serialization
- `internal/hub/message.go` -- WebSocket message type definitions
- `internal/hub/broker_iface.go` -- BrokerInterface contract
- `web/static/app.js` -- Current browser auto-refresh state, section_complete handler, navigateToSection
- `web/static/index.html` -- Current header controls layout
- `.planning/todos/pending/read-delay-burst-on-section-switch.md` -- REL-03 root cause analysis
- `.planning/todos/pending/skip-unsupported-pack-info-registers.md` -- PackInfoProbes skip requirement

### Secondary (MEDIUM confidence)
- Go `context` package documentation -- cancellation propagation semantics [CITED: pkg.go.dev/context]

### Tertiary (LOW confidence)
- None

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- no new dependencies; pure refactoring of existing Go stdlib and vanilla JS
- Architecture: HIGH -- all patterns derived from verified codebase analysis; context cancellation is standard Go
- Pitfalls: HIGH -- pitfalls identified from direct code reading and the project's own todo documents

**Research date:** 2026-04-12
**Valid until:** 2026-05-12 (stable; Go stdlib and vanilla JS don't change)
