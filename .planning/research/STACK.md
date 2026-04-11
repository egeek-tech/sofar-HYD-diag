# Technology Stack

**Project:** Sofar HYD Diagnostic Tool v1.2 -- Reliability & UX Refinements
**Researched:** 2026-04-11

## Existing Codebase Analysis

The v1.1 codebase (9,334 LOC) uses:
- **Go 1.26** with `chi/v5` router, `gorilla/websocket`, `fyne` (systray)
- **Broker pattern** (`internal/broker`): single-goroutine command loop serializing all Modbus ops via channel, with context support on all public methods
- **Hub pattern** (`internal/hub`): WebSocket client management, section subscriptions, streaming reads via per-section goroutines
- **Vanilla HTML/JS/CSS** frontend with streaming per-register updates

**Key principle: No new dependencies.** All v1.2 features are achievable with Go stdlib + existing deps. This is deliberate -- the project constraint is "single binary, minimal external dependencies."

## Recommended Stack

### Core Patterns (Go stdlib -- no new deps)

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| `context.WithCancel` | Go 1.7+ (stdlib) | Per-read-cycle cancellation | Already used in hub.go (`h.ctx`). Extend to child contexts per streaming read so disconnect aborts in-progress reads immediately. |
| `context.AfterFunc` | Go 1.21+ (stdlib) | Cancel blocked `net.Conn.Read` on context cancel | The canonical Go pattern for aborting a blocked TCP read. Calls `conn.SetReadDeadline(time.Now())` when context is cancelled, causing the read to return with timeout. Available since Go 1.21; project uses Go 1.26. |
| `context.Cause` | Go 1.20+ (stdlib) | Distinguish abort-vs-error in cancelled reads | When a read fails after cancellation, `context.Cause(ctx)` lets the broker distinguish "user disconnected" from "network error" -- different handling (no retry vs retry). |
| `sync/atomic` | Go 1.19+ (stdlib) | Track active read-cycle cancel functions | Already used for `Section.reading`. Extend pattern to store/swap cancel functions for active read cycles. |

### WebSocket Message Changes (No new deps)

| Change | Direction | Purpose | Why |
|--------|-----------|---------|-----|
| `register_value` enhanced | Server -> Client | Add `addr` and `raw` fields for tooltips | Reuse existing message type, add optional `omitempty` fields. Non-breaking -- frontend ignores unknown fields. |
| No new message types needed | -- | Disconnect already triggers context cancel | An explicit `"abort"` message is unnecessary initially. Disconnect cancels the child context which aborts reads. If "stop reading but stay connected" is needed later, add it then. |

### Frontend Patterns (Vanilla JS/CSS -- no libraries)

| Pattern | Purpose | Why |
|---------|---------|-----|
| CSS `::after` pseudo-element with `content: attr(data-tooltip)` | Pure CSS tooltips for register metadata | Zero JS for tooltip rendering. Tooltip text stored in `data-tooltip` attribute. No library needed, no event listeners to manage. |
| CSS `opacity: 0.45` + class toggle | Stale value visual dimming | Currently only color change (`--stale` class). Add opacity for stronger visual signal that previous value is stale. |
| DOM preservation on re-read | Keep last-known values during refresh | Instead of clearing body on `section_schema`, keep existing elements, mark stale, update as new values arrive. |

## Detailed Pattern Specifications

### 1. Context Cancellation for Abort-on-Disconnect

**Problem:** When user clicks Disconnect, streaming read goroutines in `hub_streaming.go` continue reading registers until their next `h.ctx.Err()` check. With 500ms inter-read delay and 10+ registers per section, this can take 5-10 seconds to stop. The user sees "Disconnected" in the UI but reads keep hitting the wire.

**Solution:** Per-read-cycle child contexts with cancellation.

**Architecture integration point -- Hub (hub.go):**

The hub already creates `h.ctx` via `context.WithCancel` in `Run()`. Currently, streaming goroutines pass `h.ctx` directly to broker calls. The fix:

```go
// Hub gains a cancel function for the active streaming read cycle
type Hub struct {
    // ... existing fields ...
    cancelRead context.CancelFunc  // cancels the active streaming read cycle
}

// triggerSectionRead creates a child context for each read cycle
func (h *Hub) triggerSectionRead(sectionName string) {
    // Cancel any in-progress read cycle first
    if h.cancelRead != nil {
        h.cancelRead()
    }
    
    readCtx, cancel := context.WithCancel(h.ctx)
    h.cancelRead = cancel
    
    // Pass readCtx (not h.ctx) to streaming functions
    sec.reading.Store(true)
    switch sectionName {
    case "bms":
        h.streamBMSRead(readCtx, sec)     // NEW: accepts context param
    case "battery":
        h.streamBatteryRead(readCtx, sec)  // NEW: accepts context param
    default:
        h.streamStandardRead(readCtx, sectionName, sec) // NEW: accepts context param
    }
}

// handleCommand for disconnect: cancel reads IMMEDIATELY
case MsgTypeDisconnect:
    if h.cancelRead != nil {
        h.cancelRead()
        h.cancelRead = nil
    }
    go func() {
        if err := h.broker.Disconnect(h.ctx); err != nil { ... }
    }()
```

**Why this works:** The broker's `ReadRegisters` already respects context cancellation in its double-select pattern (lines 144-164 of broker.go). When `cancelRead()` is called, every pending and future `ReadRegisters` call using that child context returns `ctx.Err()` immediately. The streaming goroutine's context check between reads also triggers.

**Confidence:** HIGH -- `context.WithCancel` is the standard Go cancellation pattern. The broker already has full context support on all methods. This requires only plumbing a child context through existing call chains.

### 2. Broker-Level TCP Read Cancellation via context.AfterFunc

**Problem:** Even after context cancellation, if the broker's `execute()` goroutine is blocked on `conn.Read()` inside `modbus.ReadHoldingRegistersTCP()` (specifically in `ReadFull()`), the goroutine won't return until the 10-second read deadline expires. Context cancellation only works at channel select points, not inside blocking syscalls.

**Solution:** Use `context.AfterFunc` to set an immediate deadline on the connection when context is cancelled.

**Architecture integration point -- Broker (broker.go, executeRead):**

```go
func (b *Broker) executeRead(ctx context.Context, req ReadRequest) Result {
    const maxAttempts = 2

    for attempt := 1; attempt <= maxAttempts; attempt++ {
        if err := b.ensureConnected(ctx); err != nil {
            return Result{Err: err}
        }

        b.enforceInterReadDelay()

        // Register context cancellation to interrupt blocked TCP read
        stop := context.AfterFunc(ctx, func() {
            if b.conn != nil {
                b.conn.SetReadDeadline(time.Now()) // unblocks ReadFull immediately
            }
        })

        var data []byte
        var err error
        if b.useRTU {
            data, err = modbus.ReadHoldingRegistersRTU(b.conn, b.logger, b.slaveID, req.Addr, req.Count)
        } else {
            data, err = modbus.ReadHoldingRegistersTCP(b.conn, b.logger, b.slaveID, req.Addr, req.Count)
        }

        b.lastReadTime = time.Now()
        
        // Stop the AfterFunc if it hasn't fired
        if !stop() {
            // AfterFunc fired -- context was cancelled during read.
            // The deadline is now in the past. Connection is still valid
            // but needs deadline cleared for next use.
            if b.conn != nil {
                b.conn.SetReadDeadline(time.Time{}) // clear deadline
            }
            // Don't treat this as a network error -- it's a cancellation
            return Result{Err: ctx.Err()}
        }

        // Normal path: AfterFunc did NOT fire
        if err == nil {
            return Result{Data: data}
        }

        b.handleError(err)
        if attempt == maxAttempts {
            return Result{Err: err}
        }
    }
    return Result{Err: fmt.Errorf("exhausted retry attempts")}
}
```

**Why `context.AfterFunc` over a goroutine:** `context.AfterFunc` (Go 1.21+) is the official stdlib mechanism for exactly this pattern. It is shown in the Go context package documentation as the canonical approach for interrupting `net.Conn` reads. It handles edge cases correctly: context already cancelled, stop cleanup, no goroutine leak. A hand-rolled goroutine watching `ctx.Done()` would need explicit cleanup to avoid leaks.

**Critical detail:** After AfterFunc fires, `b.conn.SetReadDeadline` is set to the past. All subsequent reads would fail until cleared. The pattern above handles this: if `stop()` returns false (AfterFunc fired), clear the deadline immediately. If the read error was from cancellation (not network failure), return `ctx.Err()` directly -- do NOT call `handleError` which would close the connection.

**Confidence:** HIGH -- This is the canonical Go pattern, documented in the official `context` package examples at pkg.go.dev. Go 1.26 includes `AfterFunc`.

### 3. Per-Register Retry Logic at Hub Level

**Problem:** Current broker retries at the transport level (2 attempts in `executeRead`). But the streaming read cycle treats any error as final for that register. A transient timeout on one register (e.g., BMS busy after pack switch) shouldn't block all subsequent registers, and should be retried.

**Solution:** Add retry logic at the hub streaming level, separate from broker retry.

**Rationale for hub-level vs broker-level:** The broker's 2-attempt retry handles *connection-level* failures (connection dropped, reconnect + retry). Hub-level retry handles *application-level* transience (BMS busy, register temporarily unavailable, timing-sensitive reads after pack switch). These are different failure modes requiring different retry strategies.

**Architecture integration point -- Hub (hub_streaming.go or new helper):**

```go
// readProbeWithRetry wraps broker.ReadRegisters with application-level retry.
// maxRetries is additional attempts beyond the first (so maxRetries=1 means 2 total).
func (h *Hub) readProbeWithRetry(ctx context.Context, p register.Probe, maxRetries int) ([]byte, error) {
    var lastErr error
    for attempt := 0; attempt <= maxRetries; attempt++ {
        if ctx.Err() != nil {
            return nil, ctx.Err()
        }
        data, err := h.broker.ReadRegisters(ctx, p.Addr, p.Count)
        if err == nil {
            return data, nil
        }
        lastErr = err
        // Don't retry on context cancellation
        if ctx.Err() != nil {
            return nil, ctx.Err()
        }
        h.logger.Debug("register read retry",
            "name", p.Name,
            "addr", fmt.Sprintf("0x%04X", p.Addr),
            "attempt", attempt+1,
            "maxRetries", maxRetries,
            "error", err)
    }
    return nil, lastErr
}
```

**Retry count recommendation:** `maxRetries=1` (2 total hub-level attempts). Each hub attempt triggers up to 2 broker-level attempts = 4 maximum wire-level attempts. This is conservative because each broker retry incurs the inter-read delay (500ms min) plus potential reconnection time. Total worst case for a single failing register: ~4 seconds. Acceptable for a diagnostic tool.

**Usage in streaming functions:** Replace `h.broker.ReadRegisters(ctx, p.Addr, p.Count)` calls in `streamStandardRead`, `streamBatteryRead`, and `streamBMSRead` with `h.readProbeWithRetry(ctx, p, 1)`.

**Confidence:** HIGH -- Straightforward wrapper around existing broker calls. No architectural changes needed.

### 4. Enhanced RegisterValueMessage for Tooltips

**Problem:** Frontend needs register address (hex) and raw value for diagnostic tooltips. Currently `RegisterValueMessage` only carries the formatted `Value` string.

**Solution:** Add optional fields to the existing message type.

**Architecture integration point -- Messages (message.go):**

```go
type RegisterValueMessage struct {
    Type    string `json:"type"`
    Section string `json:"section"`
    Group   string `json:"group"`
    Name    string `json:"name"`
    Value   string `json:"value,omitempty"`
    Error   string `json:"error,omitempty"`
    Addr    string `json:"addr,omitempty"`  // NEW: "0x0445" hex address
    Raw     string `json:"raw,omitempty"`   // NEW: raw hex bytes "ABCD" or "00120034"
}

// NewRegisterValue updated to accept probe metadata
func NewRegisterValue(section, group, name, value, errStr string, addr uint16, rawData []byte) RegisterValueMessage {
    msg := RegisterValueMessage{
        Type:    MsgTypeRegisterValue,
        Section: section,
        Group:   group,
        Name:    name,
        Value:   value,
        Error:   errStr,
        Addr:    fmt.Sprintf("0x%04X", addr),
    }
    if len(rawData) > 0 {
        msg.Raw = fmt.Sprintf("%X", rawData)
    }
    return msg
}
```

**Why `omitempty`:** Backward compatible. Existing frontend code ignores unknown JSON fields. New frontend code checks for presence.

**Confidence:** HIGH -- Adding optional JSON fields with `omitempty` is non-breaking by definition.

### 5. CSS Tooltip Pattern (Pure CSS, No JS)

**Problem:** Need tooltips showing register address and raw value on hover. Must be vanilla CSS/JS only per project constraints.

**Solution:** CSS `::after` pseudo-element with `content: attr(data-tooltip)`.

**Architecture integration -- Frontend (app.js + style.css):**

In `handleRegisterValue()`, set tooltip data attribute when message arrives:

```javascript
// handleRegisterValue -- set tooltip from addr/raw
if (msg.addr) {
    var tip = 'Addr: ' + msg.addr;
    if (msg.raw) tip += ' | Raw: 0x' + msg.raw;
    el.setAttribute('data-tooltip', tip);
}
```

CSS tooltip rule (added to style.css):

```css
.data-row-h__value[data-tooltip] {
    position: relative;
    cursor: help;
}

.data-row-h__value[data-tooltip]:hover::after {
    content: attr(data-tooltip);
    position: absolute;
    bottom: calc(100% + 4px);
    left: 50%;
    transform: translateX(-50%);
    padding: 4px 8px;
    background: var(--color-surface-secondary, #1e1e2e);
    color: var(--color-text-muted, #888);
    font-size: 11px;
    font-family: var(--font-mono, 'SF Mono', 'Consolas', monospace);
    white-space: nowrap;
    border-radius: 4px;
    border: 1px solid var(--color-border, #333);
    pointer-events: none;
    z-index: 100;
    box-shadow: 0 2px 4px rgba(0,0,0,0.3);
}
```

**Why CSS-only over JS tooltips:** Fewer moving parts. No event listeners to manage. No cleanup on DOM changes. The tooltip content changes via attribute, which CSS picks up automatically. The existing codebase already uses `data-register` attributes on value elements -- same pattern.

**Known limitation:** CSS tooltips can't reposition if they overflow the viewport. For this desktop-only diagnostic tool with fixed layout, this is acceptable. If edge overflow becomes an issue, a 4-line JS repositioner can be added later.

**Confidence:** HIGH -- Standard CSS pattern using `content: attr()`, supported in all modern browsers for over a decade.

### 6. Stale Value Dimming Pattern

**Problem:** When a register read fails on refresh, the previous value should remain visible but visually marked as stale. Currently implemented partially.

**Current state in codebase:**

`handleRegisterValue` (app.js line 914-922):
```javascript
if (msg.error) {
    el.classList.add('data-row-h__value--stale');
    el.classList.remove('data-row-h__value--pending');
} else {
    el.textContent = msg.value || '\u2014';
    el.classList.remove('data-row-h__value--pending', 'data-row-h__value--stale');
}
```

CSS (style.css lines 550-561):
```css
.data-row-h__value--stale {
    color: var(--color-text-muted, #888888);
}
.data-row-h__value--stale::after {
    content: " \26A0";
    color: var(--color-destructive, #d9534f);
    font-size: 12px;
}
```

**Enhancement needed:**

1. Add `opacity: 0.45` for stronger visual differentiation (color alone is subtle in dark theme)
2. Add `transition: opacity 0.3s ease` for smooth state changes
3. Preserve stale values across section schema re-renders (the big behavioral change)

Enhanced CSS:

```css
.data-row-h__value--stale {
    color: var(--color-text-muted, #888888);
    opacity: 0.45;
    transition: opacity 0.3s ease, color 0.3s ease;
}

.data-row-h__value--stale::after {
    content: " \26A0";
    color: var(--color-destructive, #d9534f);
    font-size: 12px;
    opacity: 1; /* warning icon stays fully visible */
}

/* Transition back to fresh state smoothly */
.data-row-h__value {
    transition: opacity 0.3s ease, color 0.3s ease;
}
```

**DOM preservation behavioral change:** Currently `handleSectionSchema` clears the body and renders fresh skeleton cards (app.js line 831: `body.textContent = ''`). For stale value persistence, the schema handler must:

1. On first render (no existing elements): render skeleton as today
2. On re-render (existing elements present): DO NOT clear body. Instead, add `--stale` class to all existing value elements. As new `register_value` messages arrive, they update values and remove `--stale`.

```javascript
function handleSectionSchema(msg) {
    if (msg.section !== App.activeSection) return;
    if (msg.section === 'bms' && packViewState.mode === 'pack_detail') return;

    var body = $('#content-body');
    var existingRegisters = body.querySelectorAll('[data-register]');
    
    if (existingRegisters.length > 0) {
        // Re-read: mark all existing values as stale, keep DOM
        existingRegisters.forEach(function(el) {
            el.classList.add('data-row-h__value--stale');
        });
        return; // keep existing DOM, values will be updated by register_value messages
    }
    
    // First render: build skeleton as before
    body.textContent = '';
    // ... existing skeleton rendering code ...
}
```

**Confidence:** HIGH -- CSS opacity is universal. DOM preservation is a modest refactoring of `handleSectionSchema` with clear before/after behavior.

## Alternatives Considered

| Category | Recommended | Alternative | Why Not |
|----------|-------------|-------------|---------|
| TCP read abort | `context.AfterFunc` + `SetReadDeadline` | Close connection on cancel | Closing is destructive; broker would need full reconnect cycle. `SetReadDeadline(time.Now())` is non-destructive -- connection stays open after deadline is cleared. |
| TCP read abort | `context.AfterFunc` | Goroutine watching `ctx.Done()` | `AfterFunc` is the official stdlib pattern (Go 1.21+). Handles cleanup, already-cancelled, stop-func correctly. Hand-rolled goroutine needs explicit cleanup to avoid leaks. |
| Tooltip impl | CSS `::after` + `data-tooltip` attr | JS `mouseenter`/`mouseleave` event listeners | More code, event listener cleanup needed on DOM changes, potential for memory leaks. CSS approach is zero-maintenance. |
| Tooltip impl | CSS `::after` + `data-tooltip` attr | `title` attribute (browser native) | Browser tooltips have 500ms+ delay, can't be styled, no monospace font. Custom CSS tooltip appears instantly with project styling. |
| Stale visual | CSS `opacity: 0.45` + class toggle | JS `setTimeout` fade after threshold | Timer-based approach is fragile (what if read takes longer than threshold?). Class-based is deterministic: stale on error, fresh on value received. |
| Retry location | Hub streaming layer | Broker `executeRead` | Broker retry handles transport failures (reconnect). Hub retry handles application transience (BMS busy). Different failure modes, different retry strategies. Mixing them in broker would conflate concerns. |
| Cancel scope | Per-read-cycle child context | Global `h.ctx` cancel | Cancelling `h.ctx` would shut down the entire hub. Child contexts cancel only the active read cycle, leaving hub operational for next user action. |
| Abort signal | Context cancel on disconnect (Go-side) | New `"abort"` WebSocket message | Disconnect already triggers the cancel path. A separate abort message adds complexity without clear use case. Add later if "stop reads but stay connected" is needed. |

## Integration Points with Existing Architecture

### Broker (internal/broker/broker.go)
- `executeRead` (line 369): Add `context.AfterFunc` for TCP read cancellation
- `executeWrite` (line 407): Same AfterFunc pattern for writes
- `executeBatch` (line 439): Already checks `ctx.Err()` between reads -- no change needed
- `ensureConnected` (line 503): Already respects context -- no change needed

### Hub (internal/hub/hub.go)
- Hub struct: Add `cancelRead context.CancelFunc` field
- `triggerSectionRead` (line 385): Create child context, store cancel
- `handleCommand` disconnect case (line 228): Call cancel before broker disconnect
- `subscribeClient` (line 295): Cancel previous read cycle when re-subscribing
- `handleTimerTick` (line 356): Create child context for timer-triggered reads

### Hub Streaming (internal/hub/hub_streaming.go)
- `streamStandardRead` (line 33): Accept `context.Context` parameter instead of using `h.ctx`
- `streamBatteryRead` (line 151): Same
- `streamBMSRead` (line 223): Same
- All streaming functions: Replace `h.broker.ReadRegisters(h.ctx, ...)` with `h.readProbeWithRetry(ctx, ...)`

### Messages (internal/hub/message.go)
- `RegisterValueMessage` (line 189): Add `Addr string` and `Raw string` fields
- `NewRegisterValue` (line 222): Add addr/rawData parameters

### Frontend (web/static/app.js)
- `handleSectionSchema` (line 825): DOM preservation logic for stale values
- `handleRegisterValue` (line 905): Set `data-tooltip` from `addr`/`raw`, handle stale->fresh transition
- `renderSkeletonCard` (line 866): No structural changes needed

### Frontend (web/static/style.css)
- `.data-row-h__value--stale` (line 554): Enhance with `opacity: 0.45` and transition
- New `.data-row-h__value[data-tooltip]:hover::after` rule for tooltips
- New `.data-row-h__value` transition rule for smooth state changes

## Installation

No changes to dependencies:

```bash
# No go.mod changes
# Verify build:
go build -o modbus_reader ./cmd/server
```

## Sources

- [Go context package -- AfterFunc](https://pkg.go.dev/context#AfterFunc) -- Canonical pattern for net.Conn read cancellation (Go 1.21+)
- [Go context package examples](https://go.dev/src/context/example_test.go) -- Official AfterFunc + SetReadDeadline example
- [Go net package -- Conn.SetReadDeadline](https://pkg.go.dev/net#Conn) -- Deadline mechanics for TCP reads
- [context.AfterFunc proposal](https://github.com/golang/go/issues/57928) -- Design rationale
- [CSS tooltip with only CSS](https://dev.to/kallmanation/building-a-tooltip-with-only-css-4k9) -- Pure CSS `content: attr()` pattern
- [Graceful shutdown of TCP server in Go](https://eli.thegreenplace.net/2020/graceful-shutdown-of-a-tcp-server-in-go/) -- Context + net.Conn patterns
- [Canceling a context -- Boldly Go](https://boldlygo.tech/archive/2025-04-28-canceling-a-context/) -- Modern context cancellation patterns
- [Go net.Conn deadline discussion](https://groups.google.com/g/golang-nuts/c/VPVWFrpIEyo) -- SetDeadline affecting in-flight reads
