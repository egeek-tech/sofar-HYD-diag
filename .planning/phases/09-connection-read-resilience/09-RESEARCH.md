# Phase 9: Connection & Read Resilience - Research

**Researched:** 2026-04-12
**Domain:** Go concurrency (context cancellation, net.Conn deadline manipulation), Modbus error classification, retry patterns
**Confidence:** HIGH

## Summary

Phase 9 adds two targeted resilience capabilities to the existing broker/hub/streaming architecture: (1) immediate disconnect response that aborts in-progress Modbus reads within 1 second, and (2) transparent per-register retry with error suppression so users never see transient errors.

The core challenge for disconnect is architectural: the broker's Run() loop processes commands sequentially through a single goroutine, so if `executeRead` is blocking on a 10-second TCP socket read, a disconnect command queued on the command channel cannot execute until that read completes or times out. The CONTEXT.md decision D-02 solves this by shortening `conn.SetReadDeadline(time.Now())` from outside the command loop to immediately unblock the pending read. This requires exposing the connection object (or an abort mechanism) to the disconnect path without violating the broker's single-goroutine ownership model.

The retry change (D-05) is simpler: change `maxAttempts` from 2 to 3 in `executeRead` and add a retryability check that skips retry for Modbus exception 0x02 (illegal address). The error suppression (D-08) requires the streaming goroutines to hold back error messages during retries, which is already naturally handled since `executeRead` only returns after all attempts are exhausted.

**Primary recommendation:** Add an `abortRead` method to the broker that sets `conn.SetReadDeadline(time.Now())` under a mutex, callable from `Disconnect()` before queuing the disconnect command. This unblocks the in-progress read, which fails fast, and the subsequent `executeDisconnect` command processes immediately.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Disconnect uses context cancellation to abort in-progress reads. The Hub cancels the readCtx, which propagates through broker operations and streaming goroutines.
- **D-02:** When disconnect is requested and a TCP read is blocking on the socket, set `conn.SetReadDeadline(time.Now())` to immediately unblock the pending read. The read returns a timeout error, context cancellation catches it, and disconnect proceeds. This guarantees <1s disconnect response.
- **D-03:** The UI waits for backend confirmation (state_change:disconnected) before transitioning to disconnected state -- no optimistic UI update. The context cancellation + deadline shortening path must be fast enough that this confirmation arrives within ~1s.
- **D-04:** On disconnect, all pending refresh timers in the browser are cleared (already implemented in Phase 8 D-13). No additional browser-side changes needed for the disconnect path.
- **D-05:** Register retries happen at the broker level. Change `executeRead`'s `maxAttempts` from 2 to 3. This keeps retry logic centralized in one place rather than splitting across broker and streaming layers.
- **D-06:** Retry all errors except Modbus exception 0x02 (illegal address). Timeout, connection reset, and other I/O errors are retried. Illegal address is permanent -- the register doesn't exist on this hardware. This aligns with Phase 8's PackInfoProbes skip logic.
- **D-07:** Between retry attempts, the existing reconnect-on-error behavior is preserved. If a read fails, the broker closes and re-establishes the TCP connection before retrying.
- **D-08:** During retries, suppress errors -- don't update the display until the final outcome is known. Keep the previous value (or em-dash skeleton if no previous value) visible while retrying. The user never sees transient errors.
- **D-09:** After all 3 retries fail: if a previous successful value exists, keep it displayed but add a small warning icon or dim it to indicate staleness. If no previous value exists, show em-dash. This preserves data continuity.
- **D-10:** Successful retries display the value normally -- the user never knows a retry happened. This directly addresses REL-02 success criterion 4.

### Claude's Discretion
- How to expose `conn.SetReadDeadline` to the disconnect path (mutex, method on broker, or separate abort channel)
- Whether to add a `retryable(err)` helper function or inline the illegal-address check
- How to track "previous successful value" in the frontend (per-register cache or DOM-based)
- Whether the warning icon for stale values should be a CSS class change, an SVG icon, or a text indicator

### Deferred Ideas (OUT OF SCOPE)
- Read delay burst on section switch -- already fixed in Phase 8 (D-03, REL-03)
- Skip unsupported PackInfoProbes registers -- already implemented in Phase 8 (skipPackInfo atomic.Bool)
- Stream pack drill-down values per-register -- Phase 11 concern (BATT-02)
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REL-01 | User can disconnect and the connection closes immediately, aborting any in-progress Modbus reads within 1 second | Disconnect abort pattern (D-01, D-02): abortRead method + context cancellation + SetReadDeadline shortening |
| REL-02 | Register reads that return errors are automatically retried (up to 3 total attempts) before showing an error | Retry increase (D-05): maxAttempts 2->3 + retryable error check (D-06) + error suppression in streaming (D-08) |
</phase_requirements>

## Standard Stack

No new dependencies. This phase uses only Go standard library features already in the project.

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `context` | stdlib | Cancellation propagation from hub through broker to streaming goroutines | Already used throughout; context.WithCancel creates readCtx for each section |
| `net` | stdlib | `net.Conn.SetReadDeadline(time.Now())` to unblock pending socket reads | Already used for all Modbus transport; deadline manipulation is the standard Go way to abort blocking reads |
| `sync` | stdlib | `sync.Mutex` to protect conn access from abort path | Standard concurrent access pattern for shared resources |
| `strings` | stdlib | Error string matching for Modbus exception classification | Already used in hub.go for PackInfoProbes skip logic |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| SetReadDeadline abort | Separate abort channel in broker | More complex; deadline approach is simpler and proven in Go networking |
| String-matching for exception 0x02 | Typed error values | Would require modbus package changes; string matching already used and proven in Phase 8 |
| sync.Mutex for conn | atomic.Pointer[net.Conn] | Mutex is clearer; atomic.Pointer requires Go 1.19+ (available) but less readable for this use case |

## Architecture Patterns

### Recommended Approach

The changes touch three layers with minimal cross-cutting impact:

```
Broker Layer (internal/broker/broker.go)
  - Add connMu sync.Mutex to protect conn field
  - Add abortRead() method: lock connMu, set deadline to now, unlock
  - Modify Disconnect() to call abortRead() BEFORE queuing command
  - Modify executeRead() to check for illegal address before retry
  - Change maxAttempts from 2 to 3

Hub Layer (internal/hub/hub.go)
  - handleCommand MsgTypeDisconnect: cancel all section readCtx THEN call broker.Disconnect()
  - No other changes needed

Frontend (web/static/app.js)
  - handleRegisterValue: already handles error display correctly (D-03 stale pattern)
  - No changes needed for disconnect (D-04: already implemented in Phase 8)
```

### Pattern 1: Abort-Then-Disconnect

**What:** Two-phase disconnect: first abort the blocking read, then queue the disconnect command.
**When to use:** Whenever a disconnect is requested while reads may be in progress.
**Why it works:** The broker's command channel is single-consumer. If `executeRead` is blocking on a TCP read, the `CmdDisconnect` cannot be dequeued. By shortening the read deadline first (from a different goroutine), the blocking read fails immediately, `executeRead` completes, and the `CmdDisconnect` command is processed next.

```go
// [VERIFIED: codebase analysis of broker.go command loop]

// abortRead forces any in-progress socket read to fail immediately
// by setting the read deadline to now. Safe to call from any goroutine.
func (b *Broker) abortRead() {
    b.connMu.Lock()
    defer b.connMu.Unlock()
    if b.conn != nil {
        b.conn.SetReadDeadline(time.Now())
    }
}

// Disconnect now calls abortRead before queuing the command.
// This ensures the command loop isn't blocked by a pending read.
func (b *Broker) Disconnect(ctx context.Context) error {
    b.abortRead() // unblock any pending read FIRST
    
    respCh := make(chan interface{}, 1)
    select {
    case b.commands <- command{
        cmdType:  CmdDisconnect,
        request:  nil,
        response: respCh,
    }:
    // ... rest same as current
    }
    // ...
}
```

**Critical detail:** The mutex (`connMu`) must also be held when the broker assigns/clears the conn in `executeReconfigure`, `executeDisconnect`, `handleError`, and `cleanup`. This prevents a race where `abortRead` sets the deadline on a connection that's already been closed and replaced.

### Pattern 2: Retryable Error Classification

**What:** A helper function that determines whether a Modbus error should be retried.
**When to use:** In `executeRead` before deciding whether to retry.

```go
// [VERIFIED: modbus/tcp.go:61, modbus/rtu.go:42 error format]

// isRetryable returns false for permanent Modbus errors (illegal address 0x02).
// All other errors (timeout, connection reset, other exceptions) are retryable.
func isRetryable(err error) bool {
    if err == nil {
        return false
    }
    s := err.Error()
    // Modbus exception 0x02 = illegal data address -- register doesn't exist
    return !strings.Contains(s, "err=0x02")
}
```

**Why a helper function:** The same check is used in Phase 8's `skipPackInfo` logic (`strings.Contains(errStr, "0x02")`). A named helper makes the intent clearer and can be reused if needed.

### Pattern 3: Mutex-Protected Connection Access

**What:** All reads and writes to `b.conn` go through `b.connMu` when accessed from outside the Run goroutine.
**When to use:** Only `abortRead()` needs the mutex from outside. Inside the Run goroutine (where `executeRead`, `executeDisconnect`, etc. run), the single-goroutine model already prevents races on `b.conn`.

```go
// [VERIFIED: broker.go Run() loop is single-goroutine]

type Broker struct {
    // ... existing fields ...
    connMu sync.Mutex // protects conn for abortRead() from outside Run()
}

// Inside Run() goroutine: no mutex needed (single writer)
// BUT abortRead() is called from Disconnect() which runs on caller goroutine.
// So executeRead must also hold connMu when doing the actual TCP read,
// OR we accept the race window is benign (setting deadline on a conn that's
// about to be replaced is harmless -- the new conn gets a fresh deadline).
```

**Important consideration:** The benign-race approach is simpler: `abortRead` sets deadline on whatever conn exists. If the conn was just replaced (unlikely in the disconnect path since we're disconnecting), the worst case is the deadline is set on a stale conn object that's about to be closed anyway. For disconnect specifically, the conn won't be replaced because we're disconnecting, not reconnecting. The mutex approach is strictly correct but the connMu only needs to protect the read in `abortRead` and the clear in `executeDisconnect`/`handleError`/`cleanup`.

**Recommendation:** Use the simple mutex approach. Lock `connMu` in `abortRead()` and in `executeDisconnect()` when clearing `b.conn`. The `executeRead` path does NOT need the mutex because it runs on the same goroutine as `executeDisconnect` (the Run loop), so they can never execute concurrently.

### Anti-Patterns to Avoid

- **Optimistic UI disconnect:** D-03 explicitly says NO optimistic UI update. The frontend must wait for the `state_change:disconnected` WebSocket message. The abort-then-disconnect pattern ensures this arrives within ~1s.
- **Retry at the streaming layer:** D-05 says retries happen at the broker level in `executeRead`. Do NOT add retry logic in `streamStandardRead`, `streamBatteryRead`, or `streamBMSRead`.
- **Closing conn from Disconnect() directly:** The broker owns the conn. `abortRead()` only sets the deadline -- it does NOT close the connection. Closing happens in `executeDisconnect()` which runs on the broker's goroutine.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Aborting blocking TCP reads | Custom signal channel or separate read goroutine | `conn.SetReadDeadline(time.Now())` | Go standard pattern; deadline manipulation is the idiomatic way to interrupt blocking net.Conn operations |
| Error classification | Parsing error structs or error types | String matching on `"err=0x02"` | Modbus errors are already formatted as strings; the format is stable and consistent (verified in tcp.go:61, rtu.go:42) |
| Stale value indicator | Custom DOM manipulation | Existing CSS class `data-row-h__value--stale` + `::after` pseudo-element | Already implemented in Phase 7 with warning triangle (U+26A0); D-09 behavior is already present in the frontend |

**Key insight:** Most of D-08/D-09/D-10 (error display behavior) is already implemented. The `handleRegisterValue` function in app.js already: (1) keeps previous value and adds stale class on error (line 1006), (2) displays value normally on success removing stale/pending classes (line 1010). The only thing missing is that the broker currently surfaces errors after 2 attempts -- changing to 3 attempts means the user sees fewer transient errors naturally.

## Common Pitfalls

### Pitfall 1: Mutex Deadlock Between abortRead and Run Loop

**What goes wrong:** If `abortRead()` acquires `connMu` and the Run loop also needs `connMu`, a deadlock can occur if the Run loop is blocked waiting for a command response that's blocked on the mutex.
**Why it happens:** The broker's single-goroutine model means `executeRead`, `executeDisconnect`, etc. all run on the same goroutine. If `abortRead()` is called from `Disconnect()` which blocks waiting for the command response, and the Run loop needs `connMu` to proceed, deadlock.
**How to avoid:** `abortRead()` must be NON-BLOCKING. It acquires `connMu`, sets deadline, releases mutex, and returns. The `Disconnect()` method calls `abortRead()` before sending the command, not as part of the command execution. The Run loop only needs `connMu` in `executeDisconnect` (to clear conn), which runs AFTER the read has already failed.
**Warning signs:** Disconnect hangs indefinitely instead of completing within 1s.

### Pitfall 2: Race Between abortRead and Connection Replacement

**What goes wrong:** `abortRead()` sets deadline on conn, but the Run loop has already closed that conn and opened a new one (during reconnect). The deadline is set on a closed/new conn.
**Why it happens:** The `ensureConnected` -> reconnect path replaces `b.conn`. If `abortRead()` runs during a reconnect cycle, it may target the wrong conn.
**How to avoid:** For the disconnect path specifically, this race is benign: we're disconnecting, so `ensureConnected` won't be called again (the broker goes dormant). The `abortRead` + `executeDisconnect` sequence is: (1) abort any current read, (2) executeDisconnect closes conn and sets dormant. No reconnect happens between these steps.
**Warning signs:** Disconnect appears to work but a new connection spuriously appears.

### Pitfall 3: enforceInterReadDelay Blocking After Abort

**What goes wrong:** After `abortRead()` forces a read to fail, the retry in `executeRead` calls `enforceInterReadDelay()` which sleeps up to 500ms, adding unnecessary latency to the disconnect path.
**Why it happens:** `enforceInterReadDelay()` always sleeps if less than 500ms has passed since the last read. After an aborted read, the retry will wait 500ms before attempting the next read, which then gets aborted again by context cancellation.
**How to avoid:** The context cancellation from the hub (D-01) propagates to `ensureConnected` via `ctx.Done()`. Once the hub cancels all section readCtx values, the broker's `ReadRegisters` callers (streaming goroutines) return `ctx.Err()` immediately. The `executeRead` won't even reach `enforceInterReadDelay` for retry because the context is cancelled. However, there's still a window: if the read was already dispatched and is blocking on TCP, `abortRead` unblocks it, and then the retry loop checks `ensureConnected(ctx)` which checks `ctx.Done()` from the hub's cancellation. This works correctly.
**Warning signs:** Disconnect takes 500ms+ instead of near-instant.

### Pitfall 4: Streaming Goroutines Continuing After Disconnect

**What goes wrong:** After disconnect, streaming goroutines that were in-progress continue sending results to `h.results`, causing nil pointer or send-on-closed-channel panics.
**Why it happens:** The streaming goroutines (e.g., `streamStandardRead`) run in separate goroutines and check `readCtx.Err()` between probes. If a probe is currently executing (blocking on broker.ReadRegisters), the goroutine won't check the context until the read returns.
**How to avoid:** This is already handled: (1) Hub's `handleStateEvent` cancels all section reads on disconnect (line 249-253), (2) `ReadRegisters` returns `ctx.Err()` when context is cancelled, (3) streaming goroutines check `readCtx.Err()` at the top of each probe loop iteration. With `abortRead()` forcing the TCP read to fail fast, the `ReadRegisters` call returns quickly, the streaming goroutine checks context and exits. The `h.results` channel is buffered and not closed on disconnect, so late sends are safe.
**Warning signs:** Panic on send to closed channel, or stale results appearing after disconnect.

### Pitfall 5: Illegal Address Check Too Broad

**What goes wrong:** The `isRetryable` check matches `"err=0x02"` in the error string, but other Modbus exceptions (0x01 illegal function, 0x03 illegal data value, 0x04 slave device failure) are also formatted as `"err=0xNN"`. Only 0x02 should be non-retryable.
**Why it happens:** Overly broad string matching.
**How to avoid:** Match specifically `"err=0x02"` not just `"0x02"`. The error format from modbus/tcp.go and modbus/rtu.go is always `"exception: func=0xNN err=0xNN"`, so checking for `"err=0x02"` is specific enough. Other exception codes like 0x04 (slave device failure) SHOULD be retried since they can be transient.
**Warning signs:** Registers that fail with transient exceptions (0x04) are not retried.

## Code Examples

### Disconnect Flow (End-to-End)

```
// Source: Codebase analysis of hub.go, broker.go, hub_streaming.go
// Sequence diagram for disconnect while read is in progress:

1. User clicks Disconnect button
2. Browser sends { type: "disconnect" } via WebSocket
3. Hub.handleCommand receives MsgTypeDisconnect
4. Hub cancels ALL section readCtx (sec.cancelRead() for each section)
   - This sets readCtx to cancelled for all streaming goroutines
5. Hub calls broker.Disconnect(h.ctx) in goroutine
6. broker.Disconnect() calls b.abortRead()
   - abortRead: connMu.Lock() -> conn.SetReadDeadline(time.Now()) -> connMu.Unlock()
   - This unblocks any pending ReadFull() in modbus/tcp.go or modbus/rtu.go
7. broker.Disconnect() queues CmdDisconnect on command channel
8. Meanwhile, the in-progress executeRead:
   - ReadFull returns timeout error (due to deadline)
   - handleError closes conn, sets reconnecting state
   - BUT dormant was already set by executeDisconnect? No -- executeDisconnect hasn't run yet
   - Actually: executeRead returns error result to streaming goroutine
   - Streaming goroutine checks readCtx.Err() -> context cancelled -> returns
9. executeDisconnect runs:
   - Closes conn, sets dormant=true, emits StateDisconnected
10. Hub receives StateDisconnected event
11. Hub broadcasts state_change:disconnected to all clients
12. Browser receives state_change:disconnected, transitions to disconnected UI
```

**Timing concern addressed:** Between step 6 (abortRead) and step 9 (executeDisconnect), there's a window where the broker is NOT dormant but the read has failed. In `handleError`, since `dormant` is false, the state goes to `StateReconnecting`. But then `executeDisconnect` immediately sets `dormant=true` and `StateDisconnected`. The client may see a brief `reconnecting` flash before `disconnected`. 

**Fix:** Set dormant=true in `Disconnect()` (via an atomic bool or by reordering) BEFORE the abort, or accept the brief intermediate state since it resolves within milliseconds. The simpler fix: in `handleCommand` for `MsgTypeDisconnect`, call `b.abortRead()` but also have the hub's disconnect code path set a flag so the `handleStateEvent` ignores the brief reconnecting state. OR even simpler: the `handleStateEvent` already cancels all reads on `StateReconnecting` (line 249-253), so the intermediate reconnecting state is harmless -- it just means reads get cancelled twice (once from hub, once from state event), which is idempotent.

### executeRead with 3 Attempts and Retryability Check

```go
// Source: broker.go:369 (current), modified per D-05, D-06

func (b *Broker) executeRead(ctx context.Context, req ReadRequest) Result {
    const maxAttempts = 3  // Changed from 2 (D-05)

    for attempt := 1; attempt <= maxAttempts; attempt++ {
        if err := b.ensureConnected(ctx); err != nil {
            return Result{Err: err}
        }

        b.enforceInterReadDelay()

        var data []byte
        var err error
        if b.useRTU {
            data, err = modbus.ReadHoldingRegistersRTU(b.conn, b.logger, b.slaveID, req.Addr, req.Count)
        } else {
            data, err = modbus.ReadHoldingRegistersTCP(b.conn, b.logger, b.slaveID, req.Addr, req.Count)
        }

        b.lastReadTime = time.Now()

        if err == nil {
            return Result{Data: data}
        }

        // Don't retry non-retryable errors (D-06: illegal address 0x02)
        if !isRetryable(err) {
            return Result{Err: err}
        }

        b.handleError(err)

        if attempt == maxAttempts {
            return Result{Err: err}
        }

        b.logger.Debug("retrying read after error",
            "addr", fmt.Sprintf("0x%04X", req.Addr),
            "attempt", attempt,
            "maxAttempts", maxAttempts)
    }

    return Result{Err: fmt.Errorf("exhausted retry attempts")}
}
```

### Hub Disconnect Handler (Enhanced)

```go
// Source: hub.go:213 (current), modified per D-01

case MsgTypeDisconnect:
    // D-01: Cancel all section reads FIRST so streaming goroutines exit
    for _, sec := range h.sections {
        sec.cancelRead()
    }
    go func() {
        if err := h.broker.Disconnect(h.ctx); err != nil {
            h.logger.Error("disconnect failed", "error", err)
        }
    }()
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| 2 retry attempts (maxAttempts=2) | 3 retry attempts (maxAttempts=3) | Phase 9 | Users see fewer transient errors |
| No abort on disconnect | SetReadDeadline abort on disconnect | Phase 9 | Disconnect responds within 1s even during reads |
| All errors retried equally | Illegal address (0x02) skips retry | Phase 9 | Faster failure for permanently invalid registers |

**Already implemented (no changes needed):**
- Frontend stale value display with warning icon: `data-row-h__value--stale` + `::after` content (Phase 7)
- Section read context cancellation: `sec.cancelRead()` (Phase 8)
- Disconnect clears browser refresh timers: already in `handleStateEvent` disconnect handler (Phase 8)
- Frontend `handleRegisterValue` already preserves previous values on error and removes stale class on success

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Setting `conn.SetReadDeadline(time.Now())` from a different goroutine while `ReadFull` is blocking on the same conn is safe in Go | Architecture Patterns | HIGH -- if this races, the entire abort pattern fails. Go docs confirm: "A deadline is an absolute time after which I/O operations fail" and "can be changed even if there is a pending I/O operation" [VERIFIED: Go net.Conn interface docs state deadlines can be set concurrently] |

**Note:** All other claims in this research were verified against the codebase. The only assumption is A1 which is verified against Go standard library documentation.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (stdlib) |
| Config file | none (stdlib test runner) |
| Quick run command | `go test ./internal/broker/ -run TestBroker -count=1 -v` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REL-01 | Disconnect aborts in-progress reads within 1s | integration | `go test ./internal/broker/ -run TestBrokerAbortRead -count=1 -v -timeout=10s` | Wave 0 |
| REL-01 | Hub cancels all section reads on disconnect | unit | `go test ./internal/hub/ -run TestHubDisconnectCancelsReads -count=1 -v` | Wave 0 |
| REL-02 | executeRead retries 3 times before returning error | unit | `go test ./internal/broker/ -run TestBrokerRetryThreeAttempts -count=1 -v` | Wave 0 |
| REL-02 | Illegal address (0x02) is not retried | unit | `go test ./internal/broker/ -run TestBrokerNoRetryIllegalAddress -count=1 -v` | Wave 0 |
| REL-02 | Successful retry returns value normally | unit | `go test ./internal/broker/ -run TestBrokerRetrySuccess -count=1 -v` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/broker/ ./internal/hub/ -count=1 -v -timeout=60s`
- **Per wave merge:** `go test ./... -count=1 -timeout=120s`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/broker/broker_test.go::TestBrokerAbortRead` -- covers REL-01 (disconnect aborts blocking read within 1s)
- [ ] `internal/broker/broker_test.go::TestBrokerRetryThreeAttempts` -- covers REL-02 (3 retry attempts)
- [ ] `internal/broker/broker_test.go::TestBrokerNoRetryIllegalAddress` -- covers REL-02 (exception 0x02 skip)
- [ ] `internal/broker/broker_test.go::TestBrokerRetrySuccess` -- covers REL-02 (successful retry returns data)

Existing test infrastructure: `mockModbusServer`, `buildReadResponse`, mock broker, `discardLogger` are all available and reusable. The mock server needs extension to support: (1) delayed responses (for abort testing), (2) error responses followed by success (for retry testing), (3) Modbus exception responses (for illegal address testing).

## Security Domain

Not applicable for this phase. Changes are confined to error handling and connection lifecycle management. No new inputs are accepted, no new data is exposed, and the Modbus protocol has no authentication mechanism (noted in CLAUDE.md cross-cutting concerns).

## Open Questions (RESOLVED)

1. **Brief StateReconnecting flash during disconnect** — RESOLVED: Accept the brief intermediate state per plan 09-01.
   - What we know: Between `abortRead()` and `executeDisconnect()`, the failed read triggers `handleError` which sets `StateReconnecting` (because `dormant` is still false). Then `executeDisconnect` immediately sets `StateDisconnected`.
   - What's unclear: Whether the UI will flash "Reconnecting..." briefly before showing "Disconnected".
   - Recommendation: Accept the brief intermediate state. The `handleStateEvent` in the hub already cancels all reads on `StateReconnecting`, so the behavior is correct. The flash is milliseconds at most (command channel has capacity 32, `executeDisconnect` runs immediately after `executeRead` returns). If testing reveals a visible flash, add an `aborting` flag to suppress the intermediate state event.

2. **connMu scope: how much to protect** — RESOLVED: Minimal mutex scope per plan 09-01 (abortRead, executeDisconnect, handleError, cleanup).
   - What we know: Only `abortRead()` accesses `b.conn` from outside the Run goroutine. Inside Run, all access is single-threaded.
   - What's unclear: Whether the mutex should wrap ALL conn access or just `abortRead()` and `executeDisconnect`.
   - Recommendation: Minimal mutex: only in `abortRead()` (read conn, set deadline) and `executeDisconnect()` (set conn to nil). This prevents the race where `abortRead` acts on a conn that's being closed. Inside Run, no mutex needed since it's single-goroutine.

## Sources

### Primary (HIGH confidence)
- `internal/broker/broker.go` -- executeRead (line 369), executeDisconnect (line 482), command loop architecture
- `internal/hub/hub.go` -- handleCommand disconnect path (line 213), handleStateEvent (line 237), triggerSectionRead (line 357)
- `internal/hub/hub_streaming.go` -- streamStandardRead, streamBatteryRead, streamBMSRead -- readCtx check pattern
- `internal/modbus/tcp.go` -- ReadHoldingRegistersTCP blocking read pattern, SetReadDeadline usage (line 35)
- `internal/modbus/rtu.go` -- ReadHoldingRegistersRTU blocking read pattern (line 30)
- `internal/modbus/common.go` -- ReadFull blocking loop, Connect timeout
- `internal/broker/broker_test.go` -- existing test patterns, mockModbusServer
- `web/static/app.js` -- handleRegisterValue (line 994), handleSectionComplete (line 1014), disconnect state handler (line 529)
- `web/static/style.css` -- data-row-h__value--stale class with warning icon (line 579)
- Go `net.Conn` interface documentation: deadlines can be set concurrently with pending I/O operations [VERIFIED: Go stdlib docs]

### Secondary (MEDIUM confidence)
- `.planning/phases/09-connection-read-resilience/09-CONTEXT.md` -- locked decisions D-01 through D-10
- `.planning/phases/08-refresh-architecture/08-CONTEXT.md` -- Phase 8 decisions referenced by Phase 9

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- pure Go stdlib, no new dependencies, all patterns verified in codebase
- Architecture: HIGH -- changes are well-scoped to broker.go (retry + abort) and hub.go (disconnect handler), with verified integration points
- Pitfalls: HIGH -- identified from direct codebase analysis of the command loop architecture and concurrent access patterns

**Research date:** 2026-04-12
**Valid until:** 2026-05-12 (stable -- Go stdlib patterns, no external dependency version drift)
