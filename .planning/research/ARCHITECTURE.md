# Architecture Patterns: v1.2 Reliability & UX Refinements

**Domain:** Go single-binary diagnostic web tool with WebSocket + Modbus backend
**Researched:** 2026-04-11
**Focus:** How six targeted changes integrate with the existing broker/hub architecture

## Current Architecture Summary

The system has three layers, all in-process:

```
Browser (vanilla JS) --WebSocket--> Hub (event loop goroutine) --channel cmds--> Broker (single goroutine) --TCP--> Inverter
```

**Broker** (`internal/broker/broker.go`): Single goroutine consuming a `chan command` channel. Owns the TCP connection. All Modbus reads/writes serialize through `execute()`. Has exponential backoff reconnect, inter-read delay enforcement, and connection state machine (dormant/disconnected/connecting/connected/reconnecting).

**Hub** (`internal/hub/hub.go`): Single goroutine event loop with `select` over 7 channels: ctx.Done, register, unregister, commands, stateEvents, timerCh, results, funcs. Manages client subscriptions, section definitions, auto-refresh timers, and dispatches reads. Streaming reads spawn goroutines that call `broker.ReadRegisters()` and push results back through `h.results` channel.

**Frontend** (`web/static/app.js`): WSClient class with handler dispatch. Tracks `App.activeSection`, `App.autoRefresh`. Renders section schemas with em-dash placeholders, updates per `register_value` messages.

## Change 1: Remove Backend Auto-Refresh

### Problem

The `Section` type has `autoRefresh bool`, `ticker *time.Ticker`, and `timerCh` that push timer ticks into the hub event loop. The hub then calls `triggerSectionRead()` on tick. This creates two independent refresh triggers: the backend timer AND any browser-initiated refresh, leading to sync bugs and redundant reads on a single-connection device.

### Architecture Decision

**Remove all backend timer machinery from Section.** Refresh becomes browser-only: the frontend runs its own `setInterval` and sends `{ type: "refresh", section: "..." }` messages.

### Components Modified

| Component | Change | Scope |
|-----------|--------|-------|
| `internal/hub/section.go` | Remove `autoRefresh`, `ticker`, `stopCh`, `timerCh`, `startTimer()`, `stopTimer()`, `pauseTimer()`, `resumeTimer()`, `SetInterval()`, `interval` fields | **Delete ~80 lines** |
| `internal/hub/hub.go` | Remove `timerCh` field from Hub struct, remove `timerCh` case from `select` in `Run()`, remove `handleTimerTick()`, remove `handleAutoRefreshToggle()`, remove `MsgTypeAutoRefresh` handling from `handleCommand()`, remove `refreshOverride` field | **Delete ~50 lines** |
| `internal/hub/hub.go` | Remove timer resume/pause from `handleStateEvent()` (lines 263-272) | **Simplify** |
| `internal/hub/message.go` | Remove `MsgTypeAutoRefresh` constant, remove `Enabled` field from `InboundMessage` | **Minor** |
| `internal/hub/export_test.go` | Remove `NewTestHubWithInterval`, `SetRefreshOverride` | **Minor** |
| `web/static/app.js` | Replace server-side `auto_refresh` toggle with client-side `setInterval`/`clearInterval`. Button now controls local timer, sends `refresh` on tick. | **Rewrite auto-refresh section** |

### Data Flow Change

```
BEFORE: Section.ticker -> timerCh -> hub select -> triggerSectionRead
AFTER:  Browser setInterval -> ws.send({type:"refresh"}) -> hub select -> triggerSectionRead
```

### Build Dependency

None. This change is self-contained and can be built first. Other changes depend on it only in the sense that removing the timer simplifies the hub event loop for subsequent modifications.

### Key Constraint

The `subscribeClient()` method currently triggers an immediate read on subscribe (line 332: `h.triggerSectionRead(sectionName)`). **Keep this behavior.** When a user navigates to a section, they should see data immediately without waiting for the first timer tick. The frontend refresh timer starts *after* the initial subscribe response arrives.

---

## Change 2: Immediate Disconnect (Context Cancellation Through Broker)

### Problem

When the user clicks "Disconnect", the hub sends `CmdDisconnect` through the broker command channel. But if a streaming read goroutine is currently blocked on `h.broker.ReadRegisters()`, that call is sitting in the broker's `select` waiting for its response channel. The broker processes commands serially, so the disconnect command queues behind the in-progress read. The user waits for the current read (up to 10s TCP timeout) to finish before disconnect takes effect.

### Architecture Decision

**Use a per-section cancellable context with `context.AfterFunc` for TCP deadline manipulation.**

The Go standard library's `context.AfterFunc` (Go 1.21+) is the correct pattern for aborting blocked TCP reads. When the context is cancelled, `AfterFunc` calls `conn.SetReadDeadline(time.Now())` which immediately unblocks any in-progress `conn.Read()` call with an I/O timeout error.

However, the broker owns the connection and the streaming goroutines call broker methods (not raw conn). The solution layers context cancellation at two levels:

1. **Hub level:** Each section read gets a cancellable context. On disconnect, the hub cancels all active read contexts.
2. **Broker level:** `ReadRegisters` and `ReadBatch` already accept `ctx context.Context` and check `ctx.Err()` when sending commands and receiving responses. But the blocking point is inside `executeRead()` where `modbus.ReadHoldingRegistersTCP()` blocks on `conn.Read()`.

### Detailed Design

#### Step A: Hub per-section read context

```go
// In Hub struct, add:
type activeRead struct {
    cancel context.CancelFunc
    section string
}

// In Hub struct:
activeReads map[string]context.CancelFunc  // section name -> cancel func
```

When `triggerSectionRead` or `triggerPackRead` launches a goroutine, create a child context:

```go
func (h *Hub) triggerSectionRead(sectionName string) {
    // Cancel any in-progress read for this section
    if cancel, ok := h.activeReads[sectionName]; ok {
        cancel()
    }
    
    readCtx, readCancel := context.WithCancel(h.ctx)
    h.activeReads[sectionName] = readCancel
    
    sec.reading.Store(true)
    // Pass readCtx to streaming functions instead of h.ctx
    // ...
}
```

On disconnect, cancel all:

```go
// In handleCommand for MsgTypeDisconnect:
for name, cancel := range h.activeReads {
    cancel()
    delete(h.activeReads, name)
}
```

#### Step B: Broker propagates context to TCP operations

The broker's `executeRead` currently ignores the passed `ctx` during the actual `modbus.ReadHoldingRegistersTCP()` call. The TCP read blocks on `conn.Read()` inside `ReadFull()`.

**Option 1 (Recommended): Set conn deadline from context via AfterFunc**

Add a helper in the broker that hooks context cancellation to TCP deadlines:

```go
func (b *Broker) executeRead(ctx context.Context, req ReadRequest) Result {
    // ... ensureConnected, enforceInterReadDelay ...
    
    // Hook context cancellation to abort TCP read
    if b.conn != nil {
        stop := context.AfterFunc(ctx, func() {
            b.conn.SetReadDeadline(time.Now())
        })
        defer func() {
            if !stop() {
                // Context was cancelled, reset deadline
                if b.conn != nil {
                    b.conn.SetReadDeadline(time.Time{})
                }
            }
        }()
    }
    
    // ... existing modbus read logic ...
}
```

When the hub cancels the read context, `AfterFunc` fires, sets an immediate deadline on the TCP connection, and the in-progress `ReadFull` returns with a timeout error. The broker's existing `handleError()` closes and nils the connection.

**Option 2 (Simpler but less responsive): Close connection on disconnect**

Instead of deadline manipulation, `executeDisconnect()` could simply close the TCP connection (which it already does). The issue is that `executeDisconnect` is a command that waits in the channel queue behind the in-progress read. To fix this, make disconnect non-queued:

This option is inferior because it requires changing the broker's fundamental single-goroutine guarantee or adding a side-channel, which adds complexity for less benefit.

### Recommended: Option 1 with AfterFunc

**Why:** It respects the broker's single-goroutine ownership of the connection. The `AfterFunc` runs in its own goroutine, but `SetReadDeadline` is safe to call concurrently on `net.Conn` (documented in Go stdlib). The broker goroutine still processes the result and handles the error normally.

### Components Modified

| Component | Change | Scope |
|-----------|--------|-------|
| `internal/hub/hub.go` | Add `activeReads` map, cancel on disconnect and section switch, pass per-read context to streaming functions | **Moderate** |
| `internal/hub/hub_streaming.go` | Accept `context.Context` parameter in `streamStandardRead`, `streamBMSRead`, `streamBatteryRead`; replace `h.ctx` with passed context | **Moderate** |
| `internal/broker/broker.go` | Add `context.AfterFunc` TCP deadline hook in `executeRead` and `executeBatch` | **Small, surgical** |
| `internal/hub/hub.go` | `triggerPackRead` goroutine: use per-read context, register cancel in `activeReads["bms"]` | **Small** |

### Build Dependency

Requires Change 1 (remove auto-refresh) to be done first, because the timer-triggered reads also need to work with cancellable contexts, and removing the timer simplifies what needs cancellation.

---

## Change 3: Per-Register Retry

### Problem

Currently the broker's `executeRead` does 2 attempts (lines 372-404). If a register fails both attempts, the error propagates to the hub streaming goroutine, which sends `register_value` with an error string and moves on to the next probe. There is no hub-level retry.

The v1.2 requirement is: when a single register read fails, retry it N times before moving to the next register, so transient errors don't leave gaps in the display.

### Architecture Decision

**Retry lives in the broker, not the hub.** The broker already has retry logic in `executeRead`. Increase `maxAttempts` from 2 to 3 (configurable later).

The reason to keep retry in the broker rather than the hub: the broker owns connection lifecycle. When a read fails, the broker closes the connection and reconnects. Having the hub call `ReadRegisters` in a loop would just queue more commands through the channel, with reconnection happening inside the broker anyway. The broker's retry loop is the right place because it can reconnect and retry atomically within a single command execution.

### Components Modified

| Component | Change | Scope |
|-----------|--------|-------|
| `internal/broker/broker.go` | Change `maxAttempts` from 2 to 3 in `executeRead` | **One line** |
| `internal/broker/broker.go` | Add context check between retry attempts in `executeRead` to bail early on cancellation | **Small** |

### Design Detail

```go
func (b *Broker) executeRead(ctx context.Context, req ReadRequest) Result {
    const maxAttempts = 3  // was 2

    for attempt := 1; attempt <= maxAttempts; attempt++ {
        // Check context between attempts (supports Change 2: immediate disconnect)
        if ctx.Err() != nil {
            return Result{Err: ctx.Err()}
        }
        
        if err := b.ensureConnected(ctx); err != nil {
            return Result{Err: err}
        }
        b.enforceInterReadDelay()
        // ... read logic ...
    }
}
```

### Build Dependency

Can be done independently. Benefits from Change 2 (context check between retries).

---

## Change 4: Stale Value Display

### Problem

Currently, when a register read returns an error, the frontend adds the `data-row-h__value--stale` CSS class (line 917 of app.js), which dims the element. But it keeps the last known text value (or em-dash if no value was ever received). This is already partially implemented.

The gap: on a full section re-read (refresh), the frontend first resets all values to em-dash placeholders via `section_schema`, then streams new values. If a refresh starts and some registers fail, those slots show em-dash (lost the previous value) rather than a dimmed previous value.

### Architecture Decision

**Frontend-only change. Cache values in a JavaScript Map keyed by `group::name`. On schema render, pre-populate from cache. On error, show cached value dimmed.**

No backend changes needed. The stale display logic is purely a frontend concern.

### Components Modified

| Component | Change | Scope |
|-----------|--------|-------|
| `web/static/app.js` | Add `valueCache` Map. On `register_value` with value: cache it. On `section_schema` render: populate from cache with `--stale` class. On `register_value` with error: show cached value with `--stale` class. | **Moderate frontend** |

### Design Detail

```javascript
// Global cache
const valueCache = new Map();  // "section::group::name" -> "formatted value"

function handleRegisterValue(msg) {
    const cacheKey = msg.section + '::' + msg.group + '::' + msg.name;
    
    if (msg.error) {
        // Show cached value dimmed, or em-dash if never received
        el.textContent = valueCache.get(cacheKey) || '\u2014';
        el.classList.add('data-row-h__value--stale');
    } else {
        valueCache.set(cacheKey, msg.value);
        el.textContent = msg.value || '\u2014';
        el.classList.remove('data-row-h__value--stale', 'data-row-h__value--pending');
    }
}
```

On `section_schema`, when creating placeholder elements, check if there is a cached value and use it (dimmed) instead of em-dash.

### Build Dependency

Independent. Can be done at any point.

---

## Change 5: Pack Drill-Down Streaming

### Problem

`triggerPackRead()` (hub.go lines 803-854) reads pack data as a batch: it calls `broker.ReadBatch()` for the RT block (60 registers), then again for info and temps blocks. All results are assembled into a single `PackDataMessage` and sent to the client as one JSON blob. This means the user sees nothing until all 3 batch reads complete (3+ seconds at 500ms delay).

All other sections use per-register streaming (streamStandardRead, streamBMSRead, streamBatteryRead), where each value appears as soon as it is read.

### Architecture Decision

**Replace `triggerPackRead` with a streaming approach using the existing `register_value` message type.** However, pack data has specialized structure (cell grids, bitmaps, temperature arrays) that does not fit the simple key-value `register_value` format.

**Hybrid approach:** Stream the individual named values (pack info items, individual cell voltages, individual temperatures) as `register_value` messages, then send the computed groups (cell_grid, balance, pack_status) as a final `pack_data` message with the aggregated data.

Actually, re-examining the pack data message structure: the `PackDataMessage` has specialized group types (`cell_grid`, `pack_status`, `balance`) with fields like `Cells []int`, `BalanceBitmap`, `Decoded []string` that are computed from the raw register data. The frontend renders these with dedicated renderers (grid visualizations, bitmap displays).

**Better approach:** Keep the 3-phase read (write 0x9020, settle, read blocks), but send a `section_schema` for pack layout when the user selects a pack, then stream register values from the RT block as they are read individually (instead of batch), and finally send the computed groups at the end. The cell grid and status groups still need the full data block to compute indices and decode bitmaps, so those remain batched.

### Detailed Design

1. On `select_pack`: send pack schema (placeholder slots for pack info, cell voltages, temperatures)
2. Write 0x9020 + settle (unchanged)
3. Read RT block registers individually via `ReadRegisters` instead of `ReadBatch`, streaming each as `register_value` for pack info items (Pack ID, SN, Total Voltage, Current, SOC, etc.)
4. After all RT registers read: compute and send cell_grid, temperatures, balance, pack_status as grouped `pack_data` (these need the full data for cross-register computation)
5. Read info block and temps58 block, stream applicable values
6. Send `section_complete`

Alternatively: read the 60-register RT block as a single `ReadRegisters(0x9044, 60)` call (it is one Modbus read of 60 contiguous registers), then stream each extracted value from the response buffer. This avoids 60 individual Modbus reads (which would take 30+ seconds at 500ms delay). The batch is a single Modbus operation; the "streaming" is about sending extracted values to the frontend one at a time.

**This is the correct approach:** The RT block is a single 60-register Modbus read. The data arrives at once. The "streaming" for packs means: after the single read completes, parse and send each value individually rather than waiting for all 3 blocks.

### Components Modified

| Component | Change | Scope |
|-----------|--------|-------|
| `internal/hub/hub.go` | Refactor `triggerPackRead` to send schema first, then individual register_value messages extracted from each block as it is read, and computed groups at end | **Major refactor of one function** |
| `internal/hub/message.go` | Possibly add pack-specific schema message, or reuse `SectionSchemaMessage` with section="bms" and a pack identifier | **Small** |
| `web/static/app.js` | Handle `register_value` messages in pack detail mode (currently blocked by line 908 guard). Render pack schema placeholders. | **Moderate** |

### Build Dependency

Depends on Change 2 (context cancellation) so that pack reads can be aborted mid-stream on disconnect or section switch.

---

## Change 6: Timing Enforcement Fix (Inter-Read Delay Burst)

### Problem

`enforceInterReadDelay()` (broker.go line 493) checks `time.Since(b.lastReadTime)` and sleeps if needed. But `lastReadTime` is set after the read completes (line 387), not when the delay sleep ends. When a section switch happens, the first read of the new section sees a large elapsed time (because the last read may have been seconds ago when the previous section completed), so it skips the delay entirely. If the hub dispatches multiple read commands rapidly on section switch (e.g., schema + immediate first read), the first few reads fire without any inter-read delay.

### Architecture Decision

**Set `lastReadTime` at the END of `enforceInterReadDelay()`, not after the read.** This ensures the delay is measured from the last time a delay was enforced, not from the last time a read completed.

```go
func (b *Broker) enforceInterReadDelay() {
    elapsed := time.Since(b.lastReadTime)
    if elapsed < b.interReadDelay {
        time.Sleep(b.interReadDelay - elapsed)
    }
    b.lastReadTime = time.Now()  // Move here from executeRead
}
```

And remove `b.lastReadTime = time.Now()` from `executeRead` (line 387).

This way, even if a section switch causes a burst of commands, each one waits for the full delay from the previous delay enforcement. The first read after a gap still skips the delay (which is correct -- no point in sleeping 500ms if the last read was 10 seconds ago).

### Components Modified

| Component | Change | Scope |
|-----------|--------|-------|
| `internal/broker/broker.go` | Move `b.lastReadTime = time.Now()` from `executeRead` (line 387) to end of `enforceInterReadDelay()` | **Two lines** |

### Build Dependency

Independent. Can be done first. Simplest change of the six.

---

## Component Boundaries Summary

| Component | Responsibility | Changes in v1.2 |
|-----------|---------------|------------------|
| `internal/broker/broker.go` | Serial Modbus access, connection lifecycle, retry, delay enforcement | context.AfterFunc for TCP abort (Change 2), maxAttempts 2->3 (Change 3), delay timing fix (Change 6) |
| `internal/hub/hub.go` | Client lifecycle, section subscriptions, command dispatch, state events | Remove timer machinery (Change 1), add activeReads map for cancellation (Change 2), refactor triggerPackRead (Change 5) |
| `internal/hub/hub_streaming.go` | Goroutine-based streaming read logic | Accept per-read context (Change 2) |
| `internal/hub/section.go` | Section definition and subscriber tracking | Remove all timer fields and methods (Change 1) |
| `internal/hub/message.go` | WebSocket JSON protocol types | Remove auto_refresh types (Change 1) |
| `web/static/app.js` | UI rendering, WebSocket client, user interaction | Client-side refresh timer (Change 1), value cache (Change 4), pack streaming support (Change 5) |

## Data Flow: Before vs After

### Refresh Trigger (Change 1)

```
BEFORE:
  Section.ticker --tick--> timerCh --hub select--> triggerSectionRead
  Browser --refresh msg--> commands --hub select--> triggerSectionRead (separate path)

AFTER:
  Browser setInterval --refresh msg--> commands --hub select--> triggerSectionRead (only path)
```

### Disconnect Signal (Change 2)

```
BEFORE:
  Browser --disconnect--> hub commands --> broker.Disconnect (queued behind in-progress read)
  In-progress read blocks up to 10s until TCP timeout

AFTER:
  Browser --disconnect--> hub commands --> cancel all activeReads contexts
                                      --> broker.Disconnect (may still queue)
  context.AfterFunc fires --> conn.SetReadDeadline(time.Now())
  In-progress TCP read returns immediately with timeout error
  Broker handleError closes connection
  Disconnect command processes next (connection already nil, just sets state)
```

### Per-Register Read (Changes 2 + 3)

```
                              readCtx (cancellable)
                                    |
Hub streaming goroutine             v
  for each probe:        broker.ReadRegisters(readCtx, addr, count)
    <--register_value--      |
                             v
                        broker.execute()
                           |
                           v
                        executeRead(ctx):
                          ensureConnected
                          enforceInterReadDelay (delay measured from last enforcement)
                          context.AfterFunc(ctx, conn.SetReadDeadline(now))
                          modbus.ReadHoldingRegistersTCP  <-- abortable via deadline
                          if err && attempt < 3: retry
                          if err && attempt == 3: return error
```

### Stale Value Flow (Change 4)

```
Frontend valueCache: Map<"section::group::name", "formatted value">

On register_value(value):
  cache[key] = value
  el.textContent = value
  el.classList = normal

On register_value(error):
  el.textContent = cache[key] || em-dash
  el.classList = stale (dimmed)

On section_schema (new section render):
  for each register:
    el.textContent = cache[key] || em-dash
    el.classList = cache.has(key) ? stale : pending
```

## Patterns to Follow

### Pattern 1: Context Hierarchy for Read Cancellation

**What:** Create a context tree: `appCtx -> hubCtx -> readCtx`. Each streaming read gets its own `readCtx` derived from `hubCtx`. The hub stores cancel functions and calls them on disconnect or section switch.

**When:** Any time a streaming goroutine is launched from the hub.

**Why:** Enables immediate abort without violating the broker's single-goroutine guarantee. The broker still processes one command at a time; it just gets an I/O error sooner.

```go
// Hub creates per-read context
readCtx, readCancel := context.WithCancel(h.ctx)
h.activeReads[sectionName] = readCancel

// On disconnect
for _, cancel := range h.activeReads {
    cancel()
}
```

### Pattern 2: AfterFunc for TCP Deadline Manipulation

**What:** Use `context.AfterFunc` to set `conn.SetReadDeadline(time.Now())` when context is cancelled.

**When:** In broker's `executeRead`, before calling modbus read functions.

**Why:** `net.Conn.SetReadDeadline` is documented as safe for concurrent use. It immediately unblocks any blocked `Read()` call with a timeout error. AfterFunc runs in its own goroutine, so it does not require modifying the broker's single-goroutine loop.

```go
stop := context.AfterFunc(ctx, func() {
    b.conn.SetReadDeadline(time.Now())
})
defer func() {
    if !stop() {
        // Context was cancelled; AfterFunc ran or is running.
        // Reset deadline for future reads (if conn still alive).
        if b.conn != nil {
            b.conn.SetReadDeadline(time.Time{})
        }
    }
}()
```

### Pattern 3: Browser-Owned Refresh Timer

**What:** Move the periodic refresh timer to the frontend JavaScript. The browser runs `setInterval`, sends `{"type":"refresh","section":"..."}` to the server. Server has no timer state.

**When:** Always. Replace the existing server-side timer entirely.

**Why:** Eliminates sync bugs between server timer and client state. The server becomes stateless about refresh timing -- it only responds to explicit requests. Pause/resume is trivial (clearInterval/setInterval). Connection drops naturally pause refreshes because the WebSocket is closed.

## Anti-Patterns to Avoid

### Anti-Pattern 1: Side-Channel Disconnect

**What:** Adding a second channel or mutex to the broker to bypass the command queue for disconnect.

**Why bad:** Breaks the single-goroutine guarantee. The broker's entire safety model is that only one goroutine touches `b.conn`. If disconnect bypasses the command channel, you get concurrent access to the connection.

**Instead:** Use context cancellation + TCP deadline manipulation. The context fires an AfterFunc that calls `SetReadDeadline` (safe for concurrent use per Go docs), which causes the in-progress read to fail. The broker goroutine then handles the failure normally.

### Anti-Pattern 2: Hub-Level Retry Loops

**What:** Having the hub streaming goroutine call `broker.ReadRegisters()` in a retry loop when a read fails.

**Why bad:** Each call queues a new command through the broker channel. If 20 registers each retry 3 times, that is 60 commands queued, each with connection lifecycle overhead. The broker's command channel becomes a bottleneck.

**Instead:** Keep retry inside `executeRead()` where it can reconnect and retry within a single command execution without re-queuing.

### Anti-Pattern 3: Caching Values in Backend

**What:** Storing last-known register values in the hub and sending them as "stale" on errors.

**Why bad:** The hub processes many sections and packs. Caching all values in the hub adds memory, complexity, and another source of truth. The frontend already has the DOM as its state.

**Instead:** Cache in the browser's JavaScript. The frontend is already the consumer; let it own the cache. Zero backend changes needed.

## Scalability Considerations

Not applicable for this project (single-user diagnostic tool, 1-2 concurrent browser tabs). The architecture changes are about **responsiveness** (immediate disconnect, reduced stale display) and **reliability** (retry, timing enforcement), not throughput.

## Suggested Build Order

Based on dependency analysis and incremental testability:

| Order | Change | Rationale |
|-------|--------|-----------|
| 1 | **Timing fix** (Change 6) | Two lines. Standalone. Immediately testable. Eliminates burst on section switch. |
| 2 | **Remove auto-refresh** (Change 1) | Removes complexity from hub/section before adding new complexity. Simplifies event loop. |
| 3 | **Per-register retry** (Change 3) | One line in broker + context check. Benefits from simplified hub. |
| 4 | **Immediate disconnect** (Change 2) | Adds activeReads map + AfterFunc. Requires understanding new context flow. Most architecturally complex. |
| 5 | **Stale value display** (Change 4) | Frontend-only. Independent but benefits from testing with retry + disconnect working. |
| 6 | **Pack drill-down streaming** (Change 5) | Largest refactor. Benefits from all previous changes (context cancellation, retry, stale values). |

## Sources

- [Go context.AfterFunc documentation](https://pkg.go.dev/context#AfterFunc) -- standard pattern for aborting blocked I/O
- [Go net.Conn SetReadDeadline](https://pkg.go.dev/net#Conn) -- concurrent safety guarantee for deadline manipulation
- [Canceling in-progress operations (Go official docs)](https://go.dev/doc/database/cancel-operations) -- context cancellation patterns
- [Graceful shutdown of a TCP server in Go](https://eli.thegreenplace.net/2020/graceful-shutdown-of-a-tcp-server-in-go/) -- TCP connection lifecycle patterns
- [Go context.AfterFunc issue #57928](https://github.com/golang/go/issues/57928) -- design rationale for AfterFunc
