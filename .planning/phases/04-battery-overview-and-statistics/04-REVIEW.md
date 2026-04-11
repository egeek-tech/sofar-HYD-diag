---
phase: 04-battery-overview-and-statistics
reviewed: 2026-04-11T08:04:55Z
depth: standard
files_reviewed: 10
files_reviewed_list:
  - internal/hub/hub.go
  - internal/hub/hub_test.go
  - internal/register/battery.go
  - internal/register/enum.go
  - internal/register/format.go
  - internal/register/probe.go
  - internal/register/probe_group.go
  - internal/register/register_test.go
  - internal/register/statistics.go
  - web/static/app.js
findings:
  critical: 0
  warning: 4
  info: 3
  total: 7
status: issues_found
---

# Phase 04: Code Review Report

**Reviewed:** 2026-04-11T08:04:55Z
**Depth:** standard
**Files Reviewed:** 10
**Status:** issues_found

## Summary

This phase adds battery channel overview (GenerateBatteryGroups), BMS info/protection groups, electricity statistics (StatisticsGroups), and corresponding frontend rendering. The register definitions and format logic are clean and well-tested. The primary concerns are: a potential hub goroutine deadlock in the battery auto-detect path, a missing nil-guard on the `h.ctx` field before it is used, silent data loss when results count does not match probe count in grouped rendering, and an unvalidated integer parse in the topology dropdown.

---

## Warnings

### WR-01: Battery auto-detect sends to `h.funcs` from a goroutine without a context guard — potential deadlock on hub shutdown

**File:** `internal/hub/hub.go:580`
**Issue:** Inside the goroutine launched by `triggerBatteryRead`, when a channel-count mismatch is detected the code sends directly to `h.funcs` with no `select` fallback and no context cancellation check:

```go
h.funcs <- func() {  // blocks forever if hub event loop has exited
```

If the hub context is cancelled while the Modbus read is in progress (e.g. the user closes the app), `ReadBatch` will return but the goroutine then blocks trying to send to `h.funcs`. The hub's `Run` loop has already exited, so nothing drains `h.funcs` (capacity 8). The goroutine leaks until the process exits.

The same pattern is intentionally used with a `select + ctx.Done()` guard in `RunFunc` and `ClientCount`. This case is missing the guard.

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
    // hub shutting down; discard the rebuild
}
return
```

---

### WR-02: `h.ctx` used before `Run()` is called — nil pointer dereference if `ClientCount`/`RunFunc` is called before `Run()`

**File:** `internal/hub/hub.go:140-173`
**Issue:** `ClientCount()` and `RunFunc()` both access `h.ctx` (which is the context stored in the Hub struct). This field is set inside `Run()` via `h.ctx, h.cancel = context.WithCancel(ctx)`. If either method is called before `Run()` starts, `h.ctx` is `nil` and the `<-h.ctx.Done()` receive will panic.

`NewHub` does not initialise `h.ctx`, so there is a window between construction and `Run` start where these methods are unsafe.

**Fix:** Initialise a background context as a sentinel in `NewHub` so the `Done()` channel is never nil:

```go
// In NewHub, after building the struct:
h.ctx, h.cancel = context.WithCancel(context.Background())
return h
```

Then in `Run()`, cancel the old context and create a fresh one so cancellation semantics are preserved:
```go
func (h *Hub) Run(ctx context.Context) {
    h.cancel() // cancel sentinel context
    h.ctx, h.cancel = context.WithCancel(ctx)
    ...
}
```

---

### WR-03: Silent data loss when `probeIdx` overruns `results` in `buildGroupedResult`

**File:** `internal/hub/hub.go:708-710`
**Issue:** `buildGroupedResult` contains:
```go
if probeIdx >= len(results) {
    break
}
```
When `probeIdx` reaches the end of results mid-group, the `break` exits the inner probe loop but the outer group loop continues — producing subsequent empty `GroupData` entries without any indication of the truncation. This can silently render groups with missing values rather than surfacing an error.

The same `break` pattern exists in `triggerStandardRead` at line ~436 but there `probeIdx` is derived from groups, so overflow is less likely. In `buildGroupedResult` it is the code path used for battery and stats sections, and since `flattenProbeGroups` sets `sec.Probes` from `groups`, the counts should match — but a concurrent section rebuild (battery auto-detect) could create a temporary mismatch.

**Fix:** Log a warning (or return an error message) instead of silently breaking:
```go
if probeIdx >= len(results) {
    h.logger.Warn("result count shorter than probe count",
        "section", section, "probeIdx", probeIdx, "results", len(results))
    break
}
```

---

### WR-04: `parseInt` result not range-validated for topology dropdowns in JS

**File:** `web/static/app.js:806-816`
**Issue:** `sendTopologyConfigure` reads the three `<select>` element values and calls `parseInt` on them, but does not check for `NaN` or whether the value is within the expected ranges before sending over WebSocket. If a DOM manipulation or an unexpected `select.value` (e.g. `""`) causes `parseInt` to return `NaN`, the message `{ bat_inputs: NaN, bat_towers: NaN, bat_packs: NaN }` is serialised to JSON as `null` values, and the server-side clamp logic would then replace them with the minimum values (1/1/4) silently.

While the server-side clamping does prevent harm, the silent substitution could confuse users who intentionally selected 0 or an out-of-range value via JS console manipulation.

**Fix:**
```js
function sendTopologyConfigure() {
    var inputs = parseInt($('#bat-inputs-select').value, 10);
    var towers = parseInt($('#bat-towers-select').value, 10);
    var packs  = parseInt($('#bat-packs-select').value, 10);
    if (isNaN(inputs) || isNaN(towers) || isNaN(packs)) return;
    ...
}
```

---

## Info

### IN-01: `GenerateBatteryGroups(0)` produces a degenerate "Global Stats"-only result

**File:** `internal/register/battery.go:10-47`
**Issue:** Calling `GenerateBatteryGroups(0)` returns a single-element slice containing only the Global Stats group (the channel loop body never executes). The public function has no guard against `channels <= 0`. While the production call site defaults to `2` and the auto-detect path checks `detected > 0`, the function itself is silent about invalid input, which could mislead future callers.

**Fix:** Add a guard at the top:
```go
func GenerateBatteryGroups(channels int) []ProbeGroup {
    if channels < 1 {
        channels = 1
    }
    ...
}
```

---

### IN-02: Magic constant `8` in battery auto-detect upper bound with no explanation

**File:** `internal/hub/hub.go:574`
**Issue:** The condition `detected > 0 && detected <= 8` silently clamps auto-detected channel count to a maximum of 8. The Sofar protocol supports 2 battery inputs × 4 strings = up to 8 channels, so this is correct — but the constant is unexplained. A future maintainer might not understand why 8 was chosen over some other limit.

**Fix:** Replace the magic number with a named constant or comment:
```go
const maxBatteryChannels = 8 // 2 inputs × 4 strings per Sofar topology spec
if detected > 0 && detected <= maxBatteryChannels {
```

---

### IN-03: `var` declarations mixed with `const` for storage keys in app.js

**File:** `web/static/app.js:5-13`
**Issue:** `STORAGE_KEY` is declared `const` while `PV_STORAGE_KEY`, `BAT_INPUTS_KEY`, etc. are declared `var`. They are all read-only module-level constants; using `var` is inconsistent with the `const` pattern established on line 4 and makes them mutable (they are never reassigned, but the inconsistency is misleading).

**Fix:** Declare all storage-key identifiers as `const`:
```js
const PV_STORAGE_KEY = 'sofar_pv_channels';
const BAT_INPUTS_KEY = 'sofar_bat_inputs';
const BAT_TOWERS_KEY = 'sofar_bat_towers';
const BAT_PACKS_KEY  = 'sofar_bat_packs';
```

---

_Reviewed: 2026-04-11T08:04:55Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
