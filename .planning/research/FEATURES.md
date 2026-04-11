# Feature Landscape: v1.2 Reliability & UX Refinements

**Domain:** Inverter diagnostic/monitoring web tool (Sofar HYD hybrid inverter via TCP Modbus)
**Milestone:** v1.2 - Reliability & UX Refinements
**Researched:** 2026-04-11
**Confidence:** HIGH (grounded in SCADA/HMI conventions, existing codebase analysis, and Modbus protocol best practices)

## Table Stakes

Features that users of any industrial monitoring/diagnostic tool expect when the features they depend on are already present. Missing these makes the existing v1.0/v1.1 features feel broken or unreliable.

| Feature | Why Expected | Complexity | Dependencies | Notes |
|---------|--------------|------------|--------------|-------|
| **Stale value persistence** | SCADA convention: never show blank when you have a last-known-good value. Ignition SCADA overlays "Uncertain" quality on stale data rather than blanking. Grafana shows last known value with staleness markers. Users expect data continuity during refresh cycles. | Low | Frontend-only. Uses existing `data-row-h__value--stale` CSS class already partially implemented in `handleRegisterValue`. | Extend to all sections. Keep previous text, dim it (reduced opacity), show timestamp of last good read. ISA-101 pattern: neutral gray for uncertain state. |
| **Register read retry** | Industrial Modbus best practice: 1-2 retries per register. Broker already has `maxAttempts = 2` in `executeRead`. The gap is at the streaming level -- if a single register in a section fails, the streaming hub currently marks it as error and moves on. Users expect transient errors to be retried transparently. | Low | Backend (hub streaming layer). Broker retry logic exists. Need streaming-level retry for individual registers that fail within a section read cycle. | FlowFuse best practice: "The right retry count for most industrial installations is 1, sometimes 2." Current broker does 2 attempts. Add 1 more at the streaming layer for individual register failures only. |
| **Timing enforcement** | Known bug: read delay shows burst on section switch due to `enforceInterReadDelay` timing. When switching sections, the first read fires immediately without respecting the inter-read delay from the previous section's last read. | Low | Backend (`broker.go` `enforceInterReadDelay`). The `lastReadTime` is tracked but section switches create a gap where the delay is not enforced because the new streaming goroutine starts fresh. | Fix: the broker already tracks `lastReadTime` globally. The issue is likely that `enforceInterReadDelay` is called after `ensureConnected` but the timing gap occurs between the old section's last read and the new section's first read. Straightforward fix in broker. |
| **Immediate disconnect** | When a user clicks Disconnect, they expect the UI to respond immediately. Current flow: `Disconnect` sends a command through the broker channel, but any in-progress streaming read holds the broker's command channel busy reading registers sequentially. The disconnect waits behind all queued reads. | Medium | Backend (broker + hub). Requires context cancellation propagation. The broker uses `context.Context` in `ReadRegisters`/`ReadBatch` but streaming goroutines in `hub_streaming.go` check `h.ctx.Err()` only between registers, not during the blocking `ReadRegisters` call itself. | Go pattern: `conn.SetReadDeadline(time.Now())` on context cancellation to abort a blocking TCP read. Broker needs a cancellable connection wrapper. |

## Differentiators

Features that go beyond what users expect from a v1.2 polish release. These make the tool feel noticeably more professional and diagnostic-capable.

| Feature | Value Proposition | Complexity | Dependencies | Notes |
|---------|-------------------|------------|--------------|-------|
| **Parameter tooltips** | Professional SCADA tools (Ignition, WinCC) show register metadata on hover: address, raw hex value, data type, scale factor. No inverter monitoring tool in the solar ecosystem does this -- SolarAssistant, SolarMan, Fronius SolarWeb all show only formatted values. This turns the diagnostic tool into a protocol-level debugger. | Low | Frontend-only. Register address and raw value are available in the backend (`register.Probe` has `Addr`, `Count`). Need to add `addr` and `raw` fields to `RegisterValueMessage`. | Add `data-tooltip` or `title` attribute on each register value cell. Show: register name, hex address (e.g., `0x0484`), raw hex bytes, data type, scale. CSS-only tooltip or lightweight JS hover. |
| **Battery pack UI reorder** | Move balance state (cell balancing bitmap) before temperature in pack detail view. Balance state is the most diagnostic-critical data: it tells you whether the BMS is actively balancing cells, which is the #1 indicator of pack health during charging. Current order buries it after temperatures. | Low | Frontend-only. Reorder `PackGroup` rendering in `buildPackDataMessage` or reorder groups in the frontend rendering. | No competitor (SolarAssistant, SolarMan) shows balance state at all. Placing it prominently is a differentiator for the diagnostic use case. Ordering: Pack Info > Cell Voltages > Balance State > Temperatures > Pack Status. |
| **Pack drill-down streaming** | Fix the batch display issue where pack values appear all-at-once instead of streaming per-register like other sections. Currently `triggerPackRead` reads all pack registers via `ReadBatch` and sends a single `PackDataMessage`. Convert to per-register streaming with skeleton loading, consistent with the v1.1 streaming pattern used in all other sections. | Medium | Backend (hub) + Frontend. Requires adding schema/skeleton support for pack detail view and converting `triggerPackRead` from batch to streaming pattern (similar to `streamStandardRead`). | The challenge is that pack reads require a write-settle-read sequence (0x9020 write, 1s settle, then reads). The streaming must happen within the read phase after settle completes. Write+settle stays batch; individual register reads within the data phase stream. |
| **Browser-only auto-refresh** | Remove backend auto-refresh timer, move refresh trigger to browser `setInterval`. Fixes sync state bugs where backend timer and frontend toggle get out of sync (especially on WebSocket reconnect, section switch, or multiple clients). The browser sends `refresh` messages on its own timer; backend just responds. | Medium | Backend (hub: remove section timers, `timerCh`, `handleTimerTick`) + Frontend (add `setInterval` timer management). This is an architectural change to the refresh flow. | Current architecture: backend `Section.startTimer`/`stopTimer` manages a `time.Ticker` that fires `timerCh`. Frontend just toggles. Problem: reconnect/section-switch creates timer desync. Browser-side timer is simpler: browser owns the interval, sends `refresh` messages, backend is stateless about refresh timing. Eliminates an entire class of sync bugs. |

## Anti-Features

Features to explicitly NOT build as part of v1.2, even if they seem related.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| **Automatic retry loop for persistently failing registers** | Some registers (e.g., PackInfoProbes 0x9104-0x9126) consistently return "illegal address" on certain BMS hardware. Retrying these every cycle wastes Modbus bandwidth and adds latency to every refresh. | Retry once per register per read cycle (total 3 attempts with broker's built-in 2). After that, mark as error and show stale value. Do not auto-retry across refresh cycles -- the next refresh cycle will try again naturally. |
| **Per-register retry configuration** | Adding UI to configure retry count per register or per section is over-engineering for a diagnostic tool. | Use a fixed retry policy: broker does 2 attempts, streaming layer adds 1 more for failed registers. Consistent, predictable, no configuration needed. |
| **Server-Sent Events (SSE) migration** | Some monitoring tools use SSE for server-push. WebSocket is already working, bidirectional (needed for commands), and the architecture is built around it. | Keep WebSocket. Browser-only auto-refresh still uses WebSocket for the `refresh` command and receiving streamed values. |
| **Tooltip library / rich tooltip framework** | Adding a JS tooltip library (Tippy.js, Popper.js) for parameter metadata hover. Adds external dependency to a vanilla JS/CSS frontend. | Use CSS-only tooltips with `::after` pseudo-element on `[data-tooltip]` attributes, or native `title` attribute for simplicity. Native `title` is sufficient for diagnostic metadata. |
| **Register value history within tooltips** | Showing last N values or sparkline in tooltip. Adds state management complexity with no persistent storage. | Show only current formatted value + raw hex + address in tooltip. Users who need history use external tools. |
| **Auto-refresh interval configuration** | Letting users set the refresh interval (currently hardcoded 10s). Adds UI complexity and edge cases (too-fast intervals overwhelming Modbus). | Keep 10s interval. The interval is bounded by Modbus read time anyway (a full section read takes 3-15s depending on register count). Faster intervals just queue up reads. |
| **Disconnect confirmation dialog** | Prompting "Are you sure?" before disconnecting. Adds friction to a diagnostic workflow where connect/disconnect cycles are frequent. | Disconnect immediately on button click. The "immediate disconnect" feature already makes this feel responsive and intentional. |

## Feature Dependencies

```
Timing enforcement (fix read delay burst)
  |
  +---> Register read retry (retry logic depends on correct inter-read timing)
  |
  +---> Browser-only auto-refresh (refresh trigger must respect timing)
         |
         +---> Stale value persistence (values must persist across refresh cycles)

Immediate disconnect (context cancellation)
  |
  +---> Browser-only auto-refresh (browser must detect disconnect and stop timer)
  |
  +---> Pack drill-down streaming (streaming reads must be cancellable)

Parameter tooltips (standalone, no dependencies on other v1.2 features)

Battery pack UI reorder (standalone, no dependencies on other v1.2 features)

Pack drill-down streaming
  |
  +---> Battery pack UI reorder (reorder applies to the new streaming layout)
  |
  +---> Stale value persistence (pack values need stale handling like other sections)
```

### Critical Path

```
1. Timing enforcement (unblocks correct retry behavior)
2. Immediate disconnect (architectural: context cancellation plumbing)
3. Register read retry (builds on timing + cancellation)
4. Browser-only auto-refresh (removes backend timers, simplifies architecture)
5. Stale value persistence (builds on streaming, works with new refresh model)
6. Pack drill-down streaming (converts pack reads to streaming pattern)
7. Battery pack UI reorder (trivial once pack streaming is in place)
8. Parameter tooltips (independent, can go anywhere in the sequence)
```

## Expected UX Behaviors (SCADA/Monitoring Conventions)

### Stale Data Display

**Industry convention (ISA-101, Ignition SCADA, Grafana):**
- Never blank a value that was previously good. The last known value is always more useful than an em-dash.
- Ignition uses three quality overlay types: Pending (write in progress), Unknown/Uncertain (stale), and Error (bad). Each has small and large visual variants.
- Stale values should be visually distinct but readable. Reduced opacity (0.5-0.6) is the standard approach -- the value is visible but clearly "aged."
- A timestamp of the last successful read should be available (already partially implemented via `section_complete` timestamp).
- FlowFuse warns: "your polling stack fills in the gap by returning the last successfully read value, making the problem invisible until someone notices the data has stopped changing." The dimmed visual is critical to prevent this invisibility.

**Recommended implementation:**
- On error: keep previous text, add `--stale` class (opacity: 0.5, italic or lighter font-weight).
- On success: replace text, remove `--stale` class, flash green briefly.
- On section load (no prior value): show em-dash skeleton with `--pending` class.
- Include last-good-read timestamp in tooltip or section footer.

### Abort/Disconnect Semantics

**Industry convention:**
- Disconnect must be immediate (sub-second UI response). Operators in industrial settings expect control actions to take effect within 1-2 seconds.
- In-progress operations should be cancelled, not completed-then-disconnected.
- Go pattern: `context.WithCancel` + `conn.SetReadDeadline(time.Now())` to interrupt blocking TCP reads.
- After disconnect: all values become stale (dimmed). Connection status indicator changes immediately. No auto-reconnect unless explicitly triggered.

**Recommended implementation:**
- Hub creates a per-connection context (child of hub context) that is cancelled on disconnect.
- Broker's `executeDisconnect` closes the TCP connection (already done), which causes any blocking `conn.Read` to return immediately with an error.
- Streaming goroutines check `h.ctx.Err()` between registers (already done) AND the broker's read returns error when connection is closed mid-read.
- Frontend: on `connection_state: disconnected`, dim all displayed values, stop browser-side auto-refresh timer, update button to "Connect".

### Retry Strategy

**Industry convention (FlowFuse Modbus best practices, SCADA field guide):**
- 1-2 retries per register is the standard. "Three retries at a 250ms timeout means a single unresponsive device costs you 1 full second per poll cycle."
- Retries should be transparent to the user -- no retry indicator unless all attempts fail.
- Failed registers should not block the rest of the section read. "Report the error for that specific tag while continuing to deliver good data for the others."
- Distinguish between transient errors (timeout, connection drop) and permanent errors (illegal address). Transient errors warrant retry; permanent errors should not be retried aggressively.

**Recommended implementation:**
- Broker: 2 attempts (already implemented in `executeRead`).
- Streaming layer: on error from `ReadRegisters`, retry once more (3 total). If still failing, send `register_value` with error and move to next register.
- Do not retry registers that return Modbus exception code 0x02 (illegal data address) -- these will never succeed. The existing `PackInfoProbes` (0x9104-0x9126) failure is this type.
- Track retry count per register for logging but do not expose to UI. Show only success or final failure.

### Register Metadata Tooltips

**Industry convention (OPC UA, Ignition, Modbus diagnostic tools):**
- OPC UA standardizes metadata: every data point has a structured address, data type, quality code, and timestamp.
- Professional Modbus tools (Modbus Poll, ModRSsim2) always show register address in hex, raw value, and data type.
- Diagnostic users expect to see "behind the formatted value" -- what register was read, what raw bytes came back, and how they were interpreted.

**Recommended implementation:**
- Extend `RegisterValueMessage` with: `addr` (hex string, e.g., "0x0484"), `raw` (hex bytes, e.g., "138A"), `count` (register count).
- Frontend: add `title` attribute or `data-tooltip` with content: `"Register 0x0484 | Raw: 0x138A | U16 x0.01"`.
- CSS tooltip positioning: above the value, appears on hover after 300ms delay, stays visible while hovering.
- For multi-register values (ASCII, U32), show all raw register values: `"0x0445-0x044E | Raw: 534F 4641 5220..."`.

## Complexity Budget

| Feature | Complexity | Effort Est. | Risk |
|---------|------------|-------------|------|
| Timing enforcement | Low | 1-2 hours | Low -- straightforward broker fix |
| Register read retry | Low | 2-3 hours | Low -- adds retry loop in streaming functions |
| Stale value persistence | Low | 2-3 hours | Low -- mostly CSS + frontend state management |
| Parameter tooltips | Low | 3-4 hours | Low -- backend message extension + CSS tooltip |
| Battery pack UI reorder | Low | 1 hour | Low -- reorder groups in build function |
| Immediate disconnect | Medium | 4-6 hours | Medium -- context cancellation plumbing, must not break reconnect logic |
| Browser-only auto-refresh | Medium | 6-8 hours | Medium -- removes backend timer system, introduces new frontend timer, must handle edge cases (section switch, reconnect, multiple tabs) |
| Pack drill-down streaming | Medium | 6-8 hours | Medium -- write-settle-read sequence complicates streaming; must handle the batch pack_data -> streaming register_value message type change |

**Total estimated effort:** 25-35 hours across 8 features.

**Highest risk:** Browser-only auto-refresh, because it replaces a working (if buggy) backend timer system with a fundamentally different architecture. Must handle: tab visibility (pause when hidden), WebSocket reconnect (restart timer), section switch (reset timer), disconnect (stop timer).

**Lowest risk:** Battery pack UI reorder and timing enforcement -- both are targeted, isolated changes.

## Sources

- [Ignition SCADA Quality Codes and Overlays](https://www.docs.inductiveautomation.com/docs/8.1/platform/tags/quality-codes-and-overlays) -- Authoritative reference for stale/uncertain/bad quality visual indicators (HIGH confidence)
- [FlowFuse: Most Modbus Polling Setups Are Wrong](https://flowfuse.com/blog/2026/04/modbus-polling-best-practices/) -- Retry count recommendations, stale data masking, timeout configuration (HIGH confidence)
- [Modbus Troubleshooting in SCADA](https://scadaprotocols.com/modbus-troubleshooting-scada-guide/) -- Error categorization, retry logic, exception code handling (HIGH confidence)
- [HMI Design Best Practices 2025](https://plcprogramming.io/blog/hmi-design-best-practices-complete-guide) -- ISA-101 high-performance HMI patterns, color conventions (MEDIUM confidence)
- [Go Context Cancellation](https://www.willem.dev/articles/context-cancellation-explained/) -- Pattern for aborting in-progress TCP reads via context + SetReadDeadline (HIGH confidence)
- [SolarAssistant Battery View](https://solar-assistant.io/help/dashboard/battery) -- Competitor reference for battery monitoring layout (MEDIUM confidence)
- Existing codebase analysis: `internal/hub/hub_streaming.go`, `internal/broker/broker.go`, `web/static/app.js` -- Current implementation patterns (HIGH confidence)
