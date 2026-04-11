---
phase: 07-streaming-display-and-configurable-timing
reviewed: 2026-04-11T00:00:00Z
depth: standard
files_reviewed: 10
files_reviewed_list:
  - internal/broker/broker.go
  - internal/hub/broker_iface.go
  - internal/hub/export_test.go
  - internal/hub/hub.go
  - internal/hub/hub_streaming.go
  - internal/hub/hub_test.go
  - internal/hub/message.go
  - web/static/app.js
  - web/static/index.html
  - web/static/style.css
findings:
  critical: 0
  warning: 5
  info: 4
  total: 9
status: issues_found
---

# Phase 7: Code Review Report

**Reviewed:** 2026-04-11
**Depth:** standard
**Files Reviewed:** 10
**Status:** issues_found

## Summary

Phase 7 adds per-register streaming display (section_schema -> register_value* -> section_complete protocol), configurable timing controls (read delay, pack settle), and the associated frontend rendering pipeline. The overall design is sound — the hub event-loop single-writer pattern is correctly maintained, the message protocol is well-typed, and the frontend uses textContent/createElement throughout (no innerHTML). No critical security issues were found.

Five warnings cover: a potential goroutine leak when the hub shuts down while a streaming goroutine is blocked writing to the `results` channel; a `broker.Close()` panic-on-double-close risk; a missing context-cancellation guard on the `h.funcs` channel send inside `streamBatteryRead`; a test timing fragility pattern used throughout the test suite; and a timing default clamping discrepancy between frontend and backend. Four info items cover stale skipped test stubs, a variable re-declaration in strict-mode JS, a dead `Faults` field always emitted on `OutboundMessage`, and minor dead-code in `buildGroupedResult`.

---

## Warnings

### WR-01: Streaming goroutines can block forever after hub shutdown

**File:** `internal/hub/hub_streaming.go:87` (and lines 99, 131, 142, 181, 213, 255-323, 383, 389)

**Issue:** Every `h.results <- sectionResult{...}` send in `streamStandardRead`, `streamBatteryRead`, and `streamBMSRead` is a plain blocking channel send with no `select` / `ctx.Done()` guard. The `results` channel has a buffer of 32 (`hub.go:85`). If the hub's Run loop exits (context cancelled) while a streaming goroutine has more than 32 items to send — or is blocked waiting on a Modbus read that completes after shutdown — the goroutine will block indefinitely on `h.results <-`, leaking the goroutine. The same applies to the `h.funcs <- func()` send in `streamBatteryRead` at line 202.

**Fix:** Wrap every send to `h.results` and `h.funcs` inside a `select` with `h.ctx.Done()`:

```go
select {
case h.results <- sectionResult{section: sectionName, msg: msg}:
case <-h.ctx.Done():
    return
}
```

For the `h.funcs` send in `streamBatteryRead` (line 202):

```go
select {
case h.funcs <- func() { ... }:
case <-h.ctx.Done():
    return
}
```

This is already the pattern `RunFunc` uses internally (hub.go:163-165); the streaming goroutines should follow the same convention.

---

### WR-02: `broker.Close()` panics on double-close

**File:** `internal/broker/broker.go:245`

**Issue:** `Close()` calls `close(b.done)` with no protection against being called twice. Calling `Close()` a second time panics with "close of closed channel". The public API comment says "Safe to call from any goroutine" but does not mention it is not safe to call more than once. In practice this can be triggered if the caller wraps a deferred `b.Close()` and then calls it explicitly elsewhere, or if two goroutines race to shut down the broker.

```go
// Current:
func (b *Broker) Close() {
    close(b.done)  // panics on second call
}
```

**Fix:** Guard with `sync.Once`:

```go
type Broker struct {
    ...
    closeOnce sync.Once
}

func (b *Broker) Close() {
    b.closeOnce.Do(func() { close(b.done) })
}
```

---

### WR-03: `handleSelectPack` input clamping is contradictory (always forces input=1)

**File:** `internal/hub/hub.go:778-781`

**Issue:** The input validation block reads:

```go
if input < 1 {
    input = 1
}
if input > 1 {
    input = 1
}
```

The second guard `if input > 1 { input = 1 }` silently overrides any client-provided input value greater than 1 to 1. This means it is impossible to select input 2 even if the hardware supports it — the constraint is stricter than `TopoTowers` warrants and is inconsistent with the tower/pack clamping which uses the topology constants. This is either a logic error (the condition should be `input > MaxInputs`) or an intentional hardcode that is misleading.

**Fix:** If only one input is supported by design, replace both guards with a single assignment and add a comment:

```go
// Hardware has exactly one DC input; ignore client-provided value.
input = 1
```

If multiple inputs are planned, define a constant and use it:

```go
const TopoInputs = 1
if input < 1 { input = 1 }
if input > TopoInputs { input = TopoInputs }
```

---

### WR-04: Timing default clamping lower-bound differs between frontend and backend

**File:** `internal/hub/hub.go:735` vs `web/static/app.js:964`

**Issue:** The backend clamps `ReadDelayMs` with a lower bound of `10` ms (`hub.go:735-737`). The frontend `clamp()` function in `initTimingControls` also uses a lower bound of `10` ms (`app.js:964`). However, the HTML `<input>` element for read delay has `min="10"` (`index.html:100`), which is consistent. The `packSettleMs` lower bound is `500` ms in the backend (`hub.go:752-754`) and `500` in the frontend (`app.js:965`), also consistent. **However**, the stored timing is sent to the backend on WebSocket reconnect (`app.js:57-65`) but the `wsopen` handler sends the raw stored values (`storedTiming.readDelay || 500`) without re-clamping through `clamp()`. If a user manually edited localStorage with an out-of-range value, the unclamped value is sent on reconnect. The backend does clamp it, so the broker is safe, but the displayed input field would show an out-of-range value until the user interacts with it.

**Fix:** In the `ws.onopen` handler, apply the same clamping before sending:

```js
this.ws.onopen = () => {
    ...
    if (storedTiming) {
        var readDelay = Math.max(10, Math.min(5000, storedTiming.readDelay || 500));
        var packSettle = Math.max(500, Math.min(10000, storedTiming.packSettle || 1000));
        this.send({ type: 'configure', section: 'timing',
            timing_config: { read_delay_ms: readDelay, pack_settle_ms: packSettle }
        });
    }
    ...
};
```

---

### WR-05: `TestSystemSectionTimeComposition` does not actually assert the composed value

**File:** `internal/hub/hub_test.go:1093-1151`

**Issue:** The test at line 1136 sets `_ = foundComposed` (ignoring the boolean) and the loop that should check for the composed "System time" `register_value` message uses `continue` for every `MsgTypeRegisterValue` message without examining `rv.Name` (lines 1125-1135). The test falls through to only verifying `section_complete` arrives. As a result, if the composition logic in `streamStandardRead` is broken (e.g., the composed "System time" message is never sent), this test still passes. The test exists to verify STREAM-01 composition but provides no actual assertion.

**Fix:** Use the raw-JSON loop pattern already established in `TestSystemSectionEnumLabel` (lines 1172-1190):

```go
foundComposed := false
for _, raw := range allMsgs {
    var rv hub.RegisterValueMessage
    if err := json.Unmarshal(raw, &rv); err != nil {
        continue
    }
    if rv.Type == hub.MsgTypeRegisterValue && rv.Name == "System time" {
        if rv.Value != "2026-03-16 14:30:45" {
            t.Errorf("composed time = %q, want '2026-03-16 14:30:45'", rv.Value)
        }
        foundComposed = true
        break
    }
}
if !foundComposed {
    t.Error("expected composed 'System time' register_value message")
}
```

Note: `allMsgs` in this test uses `drainClientMessages` which unmarshals into `OutboundMessage`. Switch to `drainRawMessages` to capture `RegisterValueMessage` correctly, matching the pattern in `TestSystemSectionEnumLabel`.

---

## Info

### IN-01: Skipped test stubs for STREAM-01, STREAM-02, TIMING-01, TIMING-02 are now implemented but tests remain skipped

**File:** `internal/hub/hub_test.go:1998-2034`

**Issue:** Four test functions (`TestStreamingRead`, `TestSectionSchema`, `TestTimingConfigure`, `TestPackSettleConfigure`) call `t.Skip("Wave 0 stub: will be implemented when...")`. The implementation they describe now exists in the codebase (streaming reads in `hub_streaming.go`, schema on subscribe in `hub.go:314-326`, timing configure in `hub.go:726-763`). These stubs are dead weight; their test IDs show as skipped in CI but the actual behavior is being tested through other tests (e.g., `TestSubscribeTriggerImmediateRead`, `TestGridSectionGroupedLayout`).

**Fix:** Either remove the skipped stubs or implement them as proper tests to increase coverage of the specific spec references they name (STREAM-01, STREAM-02, TIMING-01, TIMING-02).

---

### IN-02: `var opt` re-declared within the same function scope in `populatePackSelectorOptions`

**File:** `web/static/app.js:1373` and `1379`

**Issue:** Under `'use strict'` mode, `var` is function-scoped and re-declaring the same variable name (`opt`) twice within the same function is permitted by the spec but is a code quality issue — strict linters (ESLint `no-redeclare`) flag this as an error. The two `var opt` declarations at lines 1373 and 1379 are inside different `for` blocks but share the same function scope.

```js
// Line 1373:
var opt = document.createElement('option'); // first decl
// ...
// Line 1379:
var opt = document.createElement('option'); // re-declaration
```

**Fix:** Rename the second variable or use `let` for block-scoping:

```js
for (var t = 1; t <= packViewState.topologyTowers; t++) {
    var towerOpt = document.createElement('option');
    ...
}
for (var p = 1; p <= packViewState.topologyPacks; p++) {
    var packOpt = document.createElement('option');
    ...
}
```

---

### IN-03: `OutboundMessage.Faults` is always serialized (no `omitempty`) causing unnecessary field in every message

**File:** `internal/hub/message.go:91`

**Issue:** The `Faults` field is declared as:

```go
Faults []FaultEntry `json:"faults"`
```

The comment notes "never omit so frontend always renders fault card". However, this means every `connection_state`, `section_schema`, `register_value`, and `section_complete` message that gets marshalled through `OutboundMessage` will carry a `"faults":null` JSON field even when it is not the system section. This is a minor serialization overhead on every message, and the frontend code in `handleRegisterValue` and `handleSectionComplete` ignores `faults` anyway.

**Fix:** If the intent is only to force `faults` on `section_data` messages for the system section, consider either adding `omitempty` and having the system section always produce a non-nil slice (even empty `[]FaultEntry{}`), or using a separate struct for section_data vs other outbound message types. The current comment is the design rationale so this is low priority.

---

### IN-04: `buildGroupedResult` error handling only retains the last error message

**File:** `internal/hub/hub.go:413-418`

**Issue:** The loop in `buildGroupedResult` iterates all results to detect errors but only stores the last one:

```go
for _, r := range results {
    if r.Err != nil {
        hasError = true
        errMsg = r.Err.Error()  // overwrites on each error
    }
}
```

If multiple probes fail (e.g., a network timeout mid-batch), only the last error message is reported in the `section_error` response. The function is still used by the non-streaming `buildBMSGroupData` path and `triggerBatteryRead`. In practice the streaming paths that replaced this function report individual errors inline, so this is low impact — but the function remains in the codebase.

**Fix:** Either collect all errors into a joined string, or report the first error:

```go
for _, r := range results {
    if r.Err != nil && !hasError {
        hasError = true
        errMsg = r.Err.Error()  // keep first error
        break
    }
}
```

---

_Reviewed: 2026-04-11_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
