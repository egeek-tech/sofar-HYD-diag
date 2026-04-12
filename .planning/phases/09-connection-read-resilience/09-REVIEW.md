---
phase: 09-connection-read-resilience
reviewed: 2026-04-12T00:00:00Z
depth: standard
files_reviewed: 3
files_reviewed_list:
  - internal/broker/broker.go
  - internal/broker/broker_test.go
  - internal/hub/hub.go
findings:
  critical: 1
  warning: 3
  info: 2
  total: 6
status: issues_found
---

# Phase 09: Code Review Report

**Reviewed:** 2026-04-12
**Depth:** standard
**Files Reviewed:** 3
**Status:** issues_found

## Summary

The broker serialisation and retry logic is well-structured. The `abortRead` / `connMu` interlock for fast disconnect is correct and the exponential backoff is clean. The main concern is a potential panic in the hub when a pack-read goroutine outlives its client, and a silent API contract gap where `Reconfigure()` never propagates connection errors to callers. Two secondary goroutine-hygiene warnings relate to blocking sends on hub-owned channels from goroutines that lack a shutdown escape hatch.

---

## Critical Issues

### CR-01: Panic — send on closed channel in triggerPackRead goroutine

**File:** `internal/hub/hub.go:1014` and `internal/hub/hub.go:1027`

**Issue:** `triggerPackRead` spawns a goroutine that captures a raw `*Client` pointer. The goroutine calls `h.sendPackError` and `h.sendPackDataToClient`, both of which do `case client.send <- data:` inside a `select`. If the client disconnects while the pack-read goroutine is in-flight — which is likely for a multi-second operation (settle delay + three ReadBatch calls) — `removeClient` will have called `close(c.send)` by the time the goroutine tries to send. Sending to a closed channel panics unconditionally even inside a `select`.

```go
// hub.go:1013-1016 (sendPackError) — panics if client.send is closed
select {
case client.send <- data:   // PANIC if channel was closed by removeClient
default:
}
```

**Fix:** Gate sends on whether the client is still registered. Route pack results through `h.results` (already guarded) so delivery happens on the hub event loop where `removeClient` state is consistent, or check channel health with a recover:

```go
// Option A: route through h.results with a dedicated client target
// Option B: recover the panic (simple but masks the root cause)
// Option C: guard with a closed-channel check via an atomic flag on Client

// Recommended: add a `closed atomic.Bool` to Client, set it in removeClient,
// check before sending in sendPackError / sendPackDataToClient:

func (h *Hub) sendPackError(client *Client, ...) {
    if client.closed.Load() {
        return
    }
    ...
    select {
    case client.send <- data:
    default:
    }
}
```

---

## Warnings

### WR-01: Reconfigure() always returns nil — connection errors are silently dropped

**File:** `internal/broker/broker.go:372` and `internal/broker/broker.go:266-287`

**Issue:** `execute()` responds to `CmdReconfigure` with `cmd.response <- Result{}` regardless of whether the dial succeeded or failed. `Reconfigure()` then checks `if err, ok := resp.(error); ok { return err }` — `Result{}` is never an `error`, so `ok` is always `false` and `nil` is always returned. Every call-site that checks `if err := b.Reconfigure(...); err != nil` will never observe a connection failure.

In `hub.go:209` the hub logs `h.logger.Error("connect failed", ...)` if `Reconfigure` returns an error, but this log path is dead code.

```go
// execute (line 369-372):
case CmdReconfigure:
    req := cmd.request.(ReconfigureRequest)
    b.executeReconfigure(ctx, req)
    cmd.response <- Result{}   // always Result{}, never the dial error
```

**Fix:** Either propagate the error through `Result.Err`, or document clearly that `Reconfigure` is fire-and-forget and remove the `error` return type. If keeping the error return, send the error:

```go
case CmdReconfigure:
    req := cmd.request.(ReconfigureRequest)
    dialErr := b.executeReconfigure(ctx, req) // change return type to error
    cmd.response <- Result{Err: dialErr}
```

And in `Reconfigure()`:
```go
case resp := <-respCh:
    r := resp.(Result)
    return r.Err
```

### WR-02: Streaming goroutines send blocking on h.results without context guard — cancelled reads deliver stale data

**File:** `internal/hub/hub_streaming.go:88`, `100`, `143`, `190`, `225`, `267-338`, `395-401`

**Issue:** All three streaming read goroutines (`streamStandardRead`, `streamBatteryRead`, `streamBMSRead`) check `readCtx.Err()` only at the top of the probe-read loop — before calling `ReadRegisters`. After a successful register read, the result is sent unconditionally to `h.results` without a context check:

```go
// hub_streaming.go:86-91 — no context guard before send
data, err := h.broker.ReadRegisters(readCtx, p.Addr, p.Count)
// ... format value ...
h.results <- sectionResult{   // always sends, even if readCtx was cancelled
    section: sectionName,
    msg:     NewRegisterValue(...),
}
```

When `cancelRead()` fires (e.g., on disconnect or section switch), the goroutine's current in-flight `ReadRegisters` unblocks with an error. The goroutine then exits the loop at the next iteration's context check. But it has already sent one extra result to `h.results` after the cancellation. The hub will deliver this stale result to any remaining subscribers, which may confuse the UI.

**Fix:** Add a context check immediately before each `h.results <-` send:

```go
if readCtx.Err() != nil {
    return
}
h.results <- sectionResult{...}
```

### WR-03: Blocking send on h.funcs in streamBatteryRead goroutine — goroutine leak on hub shutdown

**File:** `internal/hub/hub_streaming.go:214`

**Issue:** When battery channel auto-detection triggers a section re-read, the goroutine does a plain blocking send on `h.funcs`:

```go
h.funcs <- func() {       // blocking — no context guard, no select with done
    sec.Groups = newGroups
    ...
    h.triggerSectionRead("battery")
}
```

`h.funcs` has capacity 8. If the hub has shut down (or `h.ctx` is done) and the channel is full or unread, this goroutine blocks forever — a goroutine leak.

**Fix:** Use a select with the hub context:

```go
select {
case h.funcs <- func() {
    sec.Groups = newGroups
    sec.Probes = flattenProbeGroups(newGroups)
    h.logger.Info("battery section auto-detected channels", "channels", detected)
    h.triggerSectionRead("battery")
}:
case <-readCtx.Done():
    return
}
```

---

## Info

### IN-01: handleSelectPack input clamping uses magic number 1 instead of a named constant

**File:** `internal/hub/hub.go:718-720`

**Issue:** The `input` field is clamped using `if input > 1 { input = 1 }` — a hardcoded `1` rather than a named constant. The pattern for the other fields uses named constants (`TopoTowers`, `TopoPacksPerTower`), making the `input` clamping look like a copy-paste error at first glance:

```go
if input > 1 {   // magic number — why 1? should be a constant
    input = 1
}
```

**Fix:** Add a constant and use it consistently:

```go
const TopoInputs = 1  // hardware has one DC input

if input > TopoInputs {
    input = TopoInputs
}
```

### IN-02: Broker.Close() is not idempotent — double-close panics

**File:** `internal/broker/broker.go:248-250`

**Issue:** `Close()` calls `close(b.done)` with no protection. If called more than once (e.g., a deferred Close in a caller plus an explicit Close), the second call panics with "close of closed channel":

```go
func (b *Broker) Close() {
    close(b.done)   // panics on second call
}
```

The test suite uses `defer b.Close()` in every test, but test teardown relies on correct single-close discipline.

**Fix:** Use a `sync.Once`:

```go
var closeOnce sync.Once

func (b *Broker) Close() {
    b.closeOnce.Do(func() { close(b.done) })
}
```

---

_Reviewed: 2026-04-12_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
