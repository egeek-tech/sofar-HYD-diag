---
phase: 08-refresh-architecture
reviewed: 2026-04-12T13:45:00Z
depth: standard
files_reviewed: 9
files_reviewed_list:
  - internal/hub/export_test.go
  - internal/hub/hub.go
  - internal/hub/hub_streaming.go
  - internal/hub/hub_test.go
  - internal/hub/message.go
  - internal/hub/section.go
  - web/static/app.js
  - web/static/index.html
  - web/static/style.css
findings:
  critical: 1
  warning: 5
  info: 4
  total: 10
status: issues_found
---

# Phase 8: Code Review Report

**Reviewed:** 2026-04-12T13:45:00Z
**Depth:** standard
**Files Reviewed:** 9
**Status:** issues_found

## Summary

Reviewed the Phase 8 refresh architecture implementation across Go hub package (hub, streaming, section, message, client, tests) and the vanilla JS/HTML/CSS frontend. The codebase is well-structured with a clear single-goroutine event loop pattern in the hub, proper context-based cancellation, and safe DOM manipulation in the frontend (no innerHTML usage). Key concerns: a data race accessing `h.packSettleMs` from goroutines without synchronization, a CSS selector injection vector in the streaming JS handler, and several test stubs that are permanently skipped.

## Critical Issues

### CR-01: Data Race on Hub Fields Accessed from Goroutines

**File:** `internal/hub/hub.go:792`
**Issue:** `triggerPackRead` launches a goroutine that reads `h.packSettleMs` (lines 792, 802) without synchronization. The `packSettleMs` field is written on the hub event loop goroutine (line 733 in `handleConfigure`) but read from a separate goroutine spawned by `triggerPackRead`. This is a data race detectable by `go test -race`. The `h.ctx` access is safe because it is set once in `Run()` before any goroutines are spawned. However, `h.packSettleMs` can be mutated at any time via a `configure` message while a pack read goroutine is running.
**Fix:** Capture `h.packSettleMs` into a local variable before launching the goroutine, or use `atomic.Int32` for the field:
```go
// In triggerPackRead, before the go func():
settleMs := h.packSettleMs

go func() {
    // ... use settleMs instead of h.packSettleMs ...
    time.Sleep(time.Duration(settleMs*2) * time.Millisecond)
    // ...
    time.Sleep(time.Duration(settleMs) * time.Millisecond)
}()
```

## Warnings

### WR-01: CSS Selector Injection via Unescaped User-Controlled Data

**File:** `web/static/app.js:1000`
**Issue:** `handleRegisterValue` builds a CSS attribute selector from `msg.group` and `msg.name` received over WebSocket: `document.querySelector('[data-register="' + key.replace(/"/g, '\\"') + '"]')`. While double-quotes are escaped, the replacement does not escape backslashes in the input, which could allow a crafted register name containing `\"` to break out of the attribute selector. Additionally, characters like `]` are not escaped and could terminate the attribute selector. Since the WebSocket data originates from the trusted Go backend (not arbitrary user input), this is low-risk in practice, but it violates defense-in-depth.
**Fix:** Use a more robust lookup, e.g., store references in a Map keyed by group+name, or use `CSS.escape()` for the selector:
```javascript
var el = document.querySelector('[data-register="' + CSS.escape(key) + '"]');
```

### WR-02: Skipped Test Stubs Left in Test Suite

**File:** `internal/hub/hub_test.go:2028-2064`
**Issue:** Four test functions (`TestStreamingRead`, `TestSectionSchema`, `TestTimingConfigure`, `TestPackSettleConfigure`) contain `t.Skip("Wave 0 stub...")` and are never implemented despite comments saying "Plan 02 will remove Skip and implement the test bodies." These stubs cover important behavioral expectations (streaming reads, schema-on-subscribe, timing configuration) that are now implemented in production code but lack their dedicated test coverage. The existing integration tests partially cover these behaviors, but the explicit stubs indicate planned tests that were never completed.
**Fix:** Either implement the test bodies (removing `t.Skip`) to provide the documented coverage, or remove the stubs entirely if the existing integration tests are deemed sufficient. Leaving them creates the false impression that testing is incomplete.

### WR-03: Dead Code -- `buildGroupedResult` Has Incorrect Error Semantics

**File:** `internal/hub/hub.go:386-423`
**Issue:** `buildGroupedResult` is dead code (confirmed: no callers exist in the codebase after the streaming migration). Beyond being dead code, it has a design flaw: if any single probe in the results slice has an error, the function returns a `NewSectionError` for the entire section (line 420-421), discarding all successfully read data. If this function were ever re-used, one failed register read out of potentially dozens would cause the entire section to show an error. The streaming path (`streamStandardRead`) correctly handles per-register errors.
**Fix:** Remove `buildGroupedResult` as dead code. It is superseded by the streaming read path.

### WR-04: Faults Field Always Serialized in Non-System Messages

**File:** `internal/hub/message.go:91`
**Issue:** The `Faults` field in `OutboundMessage` uses the JSON tag `json:"faults"` without `omitempty`. The comment says "never omit so frontend always renders fault card" -- this is intentional for the system section. However, for every other section (grid, eps, pv, battery, stats), every `OutboundMessage` includes `"faults":null` in the JSON payload. The streaming path sends `section_data` with `Faults: nil` for BMS bitmap/protection groups (line 387 in hub_streaming.go), which serializes as `"faults":null`. The frontend guards this with `Array.isArray(msg.faults)` so it is not a bug, but it adds unnecessary bytes to every non-system message.
**Fix:** Consider using `omitempty` and explicitly sending an empty slice `[]FaultEntry{}` for system section faults when there are none (so the frontend still renders "No active faults").

### WR-05: Timing Window in `streamBatteryRead` Auto-Detect Re-trigger

**File:** `internal/hub/hub_streaming.go:202-209`
**Issue:** In `streamBatteryRead`, when auto-detection triggers a section re-read, the flow is: (1) send re-trigger via `h.funcs` channel, (2) return from goroutine, (3) deferred `sec.reading.Store(false)` executes, (4) hub event loop processes the funcs message and calls `triggerSectionRead` which sets `sec.reading.Store(true)`. Between steps 3 and 4, there is a window where `reading` is false. During this window, a concurrent `read_cycle` message could pass the `sec.reading.Load()` guard in `handleReadCycle` and trigger a duplicate read.
**Fix:** Capture the intent to re-trigger within the goroutine by not resetting `reading` to false before the re-trigger:
```go
if detected != currentChannels {
    newGroups := register.GenerateBatteryGroups(detected)
    // Don't let defer reset reading -- the retrigger will set it again
    h.funcs <- func() {
        sec.Groups = newGroups
        sec.Probes = flattenProbeGroups(newGroups)
        h.logger.Info("battery section auto-detected channels", "channels", detected)
        h.triggerSectionRead("battery")
    }
    // Prevent defer from clearing reading flag before retrigger
    // (reading will be set to true again by triggerSectionRead)
    return
}
```
The fix requires restructuring so `reading` stays true through the handoff. One approach: use a sentinel in the deferred function that skips the `Store(false)` when a retrigger is pending.

## Info

### IN-01: Unused Function `toSnakeCase`

**File:** `internal/hub/section.go:90-100`
**Issue:** The `toSnakeCase` function is defined but never called anywhere in the codebase (confirmed via grep). It appears to be leftover from a previous implementation.
**Fix:** Remove the function to reduce dead code.

### IN-02: Variable `select` Shadows Reserved-like Identifier in JavaScript

**File:** `web/static/app.js:857`
**Issue:** `var select = $('#pv-channel-select');` uses the identifier `select` which, while not technically a JavaScript reserved word, is an HTML element type name and is flagged by many linters. The same pattern appears in `initCycleDelayDropdown` (line 1110). It works in practice but is confusing.
**Fix:** Rename to `selectEl` or `pvSelect` for clarity.

### IN-03: Dead Code -- `buildBMSGroupData` Has No Callers

**File:** `internal/hub/hub.go:427-523`
**Issue:** `buildBMSGroupData` is confirmed dead code (grep shows only its definition and no call sites). The BMS read path now uses `streamBMSRead` which processes groups inline. This is ~100 lines of unreachable code.
**Fix:** Remove the function.

### IN-04: Hardcoded Development IP Fallback in Frontend

**File:** `web/static/app.js:187`
**Issue:** The connection form falls back to a hardcoded IP address `10.5.99.29` when both localStorage and `/api/defaults` are unavailable. This is a development-specific IP that leaks internal network details.
**Fix:** Either remove the hardcoded fallback and leave fields empty, or use empty strings as the fallback:
```javascript
if (!$('#input-host').value) $('#input-host').value = '';
```

---

_Reviewed: 2026-04-12T13:45:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
