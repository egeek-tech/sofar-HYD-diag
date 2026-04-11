---
phase: 05-deep-battery-pack-diagnostics
reviewed: 2026-04-11T14:14:31Z
depth: standard
files_reviewed: 8
files_reviewed_list:
  - internal/register/battery.go
  - internal/register/format.go
  - internal/register/register_test.go
  - internal/hub/hub.go
  - internal/hub/message.go
  - internal/hub/hub_test.go
  - web/static/app.js
  - web/static/style.css
findings:
  critical: 0
  warning: 3
  info: 4
  total: 7
status: issues_found
---

# Phase 05: Code Review Report

**Reviewed:** 2026-04-11T14:14:31Z
**Depth:** standard
**Files Reviewed:** 8
**Status:** issues_found

## Summary

This phase implements deep battery pack diagnostics: `EncodePackQuery`, pack-level probe definitions, BMS bitmap decode tables, pack RT/info read cycle (`triggerPackRead`), cell voltage grid, temperature colour-coding, and pack status rendering. The Go backend code is well-structured and the register address tables look correct against the Sofar Modbus G3 spec. The JavaScript frontend uses `textContent` consistently (no XSS vectors found). No security vulnerabilities were identified.

Three warnings were found: a double-send bug in the legacy flat section path, a goroutine that can block indefinitely when the `funcs` channel is full, and a misleading mock counter in tests that can mask retry-success scenarios. Four info-level findings cover minor dead code and style issues.

## Warnings

### WR-01: Legacy flat-section path sends both error and data when partial reads succeed

**File:** `internal/hub/hub.go:561-566`
**Issue:** In `triggerStandardRead`, the grouped path returns early after sending an error (line 538-539). The legacy flat path (fallback) does not return early — when `hasError` is true and at least some probes succeeded, it sends a `section_error` message followed by a `section_data` message to `h.results`. Both get broadcast to subscribers, causing the client to first receive an error banner and immediately overwrite it with partial data (or vice versa, depending on channel ordering). The grouped path already handles this correctly with `return` after the error send.
**Fix:**
```go
// Legacy flat section path (fallback)
if hasError {
    h.results <- sectionResult{section: sectionName, msg: NewSectionError(sectionName, errMsg)}
    return  // ADD THIS: do not also send partial data
}
if len(data) > 0 {
    h.results <- sectionResult{section: sectionName, msg: NewSectionData(sectionName, data)}
}
```

### WR-02: Battery auto-detect goroutine sends to `h.funcs` without context cancellation check

**File:** `internal/hub/hub.go:602`
**Issue:** Inside the `triggerBatteryRead` goroutine, when a channel count mismatch is detected, the code does `h.funcs <- func() { ... }` (an unbuffered blocking send). If the hub's `funcs` channel (capacity 8) is momentarily full — e.g., during rapid topology reconfiguration — this goroutine blocks indefinitely. Because `defer sec.reading.Store(false)` still fires when the goroutine eventually unblocks, the risk is only a temporary read stall rather than a permanent lock, but it could cause an observed "battery section frozen" symptom under load. The `RunFunc` helper at line 166 already shows the correct pattern with a context-aware select.
**Fix:**
```go
select {
case h.funcs <- func() {
    sec.Groups = newGroups
    sec.Probes = flattenProbeGroups(newGroups)
    h.logger.Info("battery section auto-detected channels", "channels", detected)
    h.triggerSectionRead("battery")
}:
case <-h.ctx.Done():
    // Hub shutting down; read.Store(false) will run via defer
}
```

### WR-03: `mockBroker.writeErrCount` counter logic never clears the error, making retry-success tests impossible

**File:** `internal/hub/hub_test.go:113-121`
**Issue:** The `writeErrCount` field is documented as "number of times to return writeErr (0=forever)" but the decrement path still returns `m.writeErr` after decrementing, and the `<= 0` branch also returns `m.writeErr`. Once `writeErr` is set, `WriteRegister` always returns an error regardless of `writeErrCount`. This makes it impossible to write a test that verifies the retry-once success path in `triggerPackRead` (first write fails, second succeeds). The test `TestPackErrorOnWriteTimeout` likely only tests the "always fail" path. If a test ever sets `writeErrCount = 1` intending "fail once then succeed", it will instead always fail, silently passing a test for the wrong reason.
**Fix:**
```go
func (m *mockBroker) WriteRegister(ctx context.Context, addr uint16, value uint16) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.writeCalls = append(m.writeCalls, writeCall{Addr: addr, Value: value})
    if m.writeErr != nil {
        if m.writeErrCount == 0 {
            // 0 means error forever
            return m.writeErr
        }
        // Positive count: return error and decrement; when count reaches 0 after
        // decrement, subsequent calls succeed
        m.writeErrCount--
        return m.writeErr
    }
    return nil
}
```
With this fix, a test can set `writeErrCount = 1` to get "fail once, then succeed" behaviour.

## Info

### IN-01: `PackInfoProbes` address gap — registers 0x910C-0x9123 are skipped without comment

**File:** `internal/register/battery.go:154-165`
**Issue:** `PackInfoProbes` jumps from 0x910B directly to 0x9124, skipping registers 0x910C-0x9123. This may be intentional (those registers are undocumented or reserved in V1.38), but there is no comment explaining the gap. A future maintainer may add registers incorrectly here.
**Fix:** Add a comment above the Alarm Status 2 probe:
```go
// Registers 0x910C-0x9123 are reserved/undocumented in V1.38; skip directly to alarm/protection/fault.
{Name: "Alarm Status 2", Addr: 0x9124, Count: 1},
```

### IN-02: Magic thresholds in cell voltage deviation colouring are undocumented

**File:** `web/static/app.js:1463-1465`
**Issue:** The cell deviation thresholds `5` (good) and `20` (warn) millivolts are magic numbers with no comment explaining their source. The Sofar BMS spec or battery datasheet presumably defines acceptable spread limits; without attribution, these numbers may silently diverge from the actual spec.
**Fix:** Add inline comments:
```js
if (dev <= 5) cls += ' cell--good';       // <= 5mV deviation from avg: within spec
else if (dev <= 20) cls += ' cell--warn'; // 5-20mV: approaching limit
else cls += ' cell--danger';              // > 20mV: exceeds recommended spread
```

### IN-03: Unused variable shadowing in `initTopologyDropdowns` (non-strict `var` re-declaration)

**File:** `web/static/app.js:783-797`
**Issue:** The variable `opt` is declared with `var` inside three separate `for` loops that are all in the same function scope. In non-strict `var` semantics this is legal but the three `i`, `i`, `i` counters and three `opt` variables refer to the same binding, which can be confusing and is a JavaScript anti-pattern. With `'use strict'` at the top of the file this is not an error, but it is a code quality issue. Each loop also redeclares `var opt` identically.
**Fix:** Use `let` for loop variables and inner declarations:
```js
for (let i = 1; i <= 2; i++) {
    const opt = document.createElement('option');
    // ...
}
```
Or at minimum add a comment acknowledging the `var` hoisting here is intentional.

### IN-04: `sendPackError` silently drops the message when the client send buffer is full

**File:** `internal/hub/hub.go:1332-1335`
**Issue:** `sendPackError` uses a non-blocking select with an empty `default:` — if the client's send buffer is full, the error message is silently dropped. `sendToClient` (used for other messages) at least logs a warning and removes the client. The asymmetry means a slow client browsing the BMS pack view will receive no feedback when a pack read fails, and the loading spinner will spin indefinitely.
**Fix:** Either log the dropped error or reuse `sendToClient` for consistency:
```go
select {
case client.send <- data:
default:
    h.logger.Warn("pack error message dropped, client buffer full")
    h.removeClient(client)  // matches sendToClient pattern
}
```
(Same applies symmetrically to `sendPackDataToClient` at line 1346.)

---

_Reviewed: 2026-04-11T14:14:31Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
