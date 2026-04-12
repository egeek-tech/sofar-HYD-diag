---
phase: 10-data-persistence-tooltips
reviewed: 2026-04-12T12:00:00Z
depth: standard
files_reviewed: 9
files_reviewed_list:
  - internal/hub/hub.go
  - internal/hub/hub_streaming.go
  - internal/hub/hub_test.go
  - internal/hub/message.go
  - internal/hub/section.go
  - internal/register/format.go
  - internal/register/register_test.go
  - web/static/app.js
  - web/static/style.css
findings:
  critical: 0
  warning: 2
  info: 4
  total: 6
status: issues_found
---

# Phase 10: Code Review Report

**Reviewed:** 2026-04-12T12:00:00Z
**Depth:** standard
**Files Reviewed:** 9
**Status:** issues_found

## Summary

Phase 10 adds data persistence (section value caching), refresh dimming during read cycles, and parameter tooltips showing register address, raw value, and timestamp on hover. The implementation spans the Go hub layer (new `RegisterAddr` and `RawValue` fields on streaming messages and pack data), formatting utilities (`FormatRawValue`), and the JS frontend (tooltip DOM, cache restore, refresh dimming CSS).

Overall the code is well-structured and follows project conventions. All DOM construction uses `textContent` (no `innerHTML`), all JS comparisons use strict equality, and context cancellation guards are consistently applied. Two warnings were identified: a potential data race on `h.readDelayMs` accessed from a goroutine, and a minor logic bug in the min-cell index computation. Four informational items note dead code, a missing cache invalidation edge case, and minor style observations.

## Warnings

### WR-01: Potential data race on h.readDelayMs in SetDelayRuntime goroutine

**File:** `internal/hub/hub.go:682-688`
**Issue:** The `handleConfigure` method reads `h.readDelayMs` on the hub event loop (line 682) and updates it there, which is correct. However, at line 685 a goroutine is launched that reads the `delay` local variable (safe) to call `SetDelayRuntime`. The concern is that `h.packSettleMs` was explicitly snapshot-captured before its goroutine (line 756, comment "Capture packSettleMs before goroutine to avoid data race (CR-01)"), but `readDelayMs` at line 682 is written and the goroutine at 685 only captures the `delay` local, so this specific instance is actually safe. However, the pattern is fragile -- if future code adds reads of `h.readDelayMs` inside the goroutine (e.g., for logging), it would race. Additionally, `h.packSettleMs` (line 701) is written on the hub event loop but read from the `triggerPackRead` goroutine at line 774 only via the `settleMs` snapshot. This pattern is correct but undocumented for `readDelayMs`.

**Note:** On closer inspection, the current code is safe because the goroutine only captures the `delay` local. Downgrading severity -- but flagging the asymmetric documentation as a maintenance risk.

**Fix:** Add a comment analogous to line 753 for the readDelay goroutine to make the safety reasoning explicit:
```go
// delay is a local copy; safe to use from goroutine (no data race).
go func() {
    if err := h.broker.SetDelayRuntime(h.ctx, time.Duration(delay)*time.Millisecond); err != nil {
```

### WR-02: Min cell index logic may select wrong cell when cells[0] is zero

**File:** `internal/hub/hub.go:928-931`
**Issue:** The min-cell index computation includes a special case `(minVal == 0 && v > 0)` at line 929. This condition means: if the current minimum is 0 and we see a non-zero value, treat the non-zero value as the new minimum. The intent is to skip zero-initialized cells. However, this logic is incorrect when cell 0 genuinely reads 0mV (dead cell) -- it would skip it and report a non-zero cell as the minimum, which is misleading. More importantly, after the first non-zero cell sets `minVal`, any subsequent cell with value 0 would NOT update `minVal` (since `v < minVal` is `0 < non-zero` which is true, so it would actually be caught). Wait -- `0 < non-zero` is true, so cell value 0 would set a new minimum. The special case only matters for the initial value. If `cells[0]` is 0 and `cells[1]` is 3200, then `minVal` stays 0 and `minIdx` stays 1. So the special case triggers for `cells[1]`: `minVal == 0 && v > 0` sets `minVal = 3200, minIdx = 2`. Then if `cells[2]` is 0, `0 < 3200` is true, so `minVal = 0, minIdx = 3`. The logic is inconsistent.

The real issue: if a pack has some cells reading 0mV (no data or dead), the min/max computation produces unreliable results. The special case attempts to handle this but does so incompletely.

**Fix:** Filter out zero-valued cells entirely from min/max computation, or remove the special case and let 0 be a valid minimum (it signals a dead cell, which is useful diagnostic information):
```go
// Simple approach: let 0 be valid (dead cell is genuinely minimum)
maxIdx, minIdx := 1, 1
maxVal, minVal := cells[0], cells[0]
for i, v := range cells {
    if v > maxVal {
        maxVal = v
        maxIdx = i + 1
    }
    if v < minVal {
        minVal = v
        minIdx = i + 1
    }
}
```

## Info

### IN-01: Unused variable foundComposed in test

**File:** `internal/hub/hub_test.go:1156-1167`
**Issue:** In `TestSystemSectionTimeComposition`, the variable `foundComposed` is declared at line 1156, set to `false`, but never set to `true`. The loop at lines 1158-1166 continues on every `MsgTypeRegisterValue` without ever setting `foundComposed`. At line 1167, `_ = foundComposed` silences the compiler warning but the test does not actually verify the composed time value -- it only checks that `section_complete` arrives.
**Fix:** Either implement the raw JSON check for the composed "System time" register value (similar to `TestSystemSectionEnumLabel` which uses `drainRawMessages`), or remove the dead code and add a comment explaining why the composed value is not verified at this level.

### IN-02: Section cache not invalidated on PV channel reconfiguration

**File:** `web/static/app.js:978-986`
**Issue:** When the user changes PV channel count via the dropdown, a `configure` message is sent to the backend which rebuilds the PV groups with different register sets. However, the `sectionCache` for the "pv" key is not cleared. If the user previously viewed PV with 4 channels and switches to 2 channels, `restoreFromCache` (line 1139) would try to apply stale cached values from the 4-channel layout onto the 2-channel skeleton. The `data-register` attribute keys would not match (different group names), so the stale entries would be harmlessly ignored. This is not a bug but could lead to confusing behavior if group names overlap between configurations.
**Fix:** Clear the section-specific cache entry when PV channels change:
```javascript
select.addEventListener('change', function() {
    var channels = parseInt(select.value, 10);
    savePVChannels(channels);
    sectionCache.delete('pv');  // Clear stale PV cache
    App.ws.send({ ... });
});
```

### IN-03: Tooltip max-width may truncate long register addresses or raw values

**File:** `web/static/style.css:1185`
**Issue:** The `.param-tooltip` has `max-width: 200px` and `white-space: nowrap`. If register metadata produces lines longer than 200px (unlikely for standard registers but possible for ASCII raw values like serial numbers rendered as hex), content would be clipped. The `white-space: nowrap` combined with a constrained `max-width` could cause overflow.
**Fix:** Either remove `white-space: nowrap` (allow wrapping within max-width) or increase max-width to accommodate longer hex strings:
```css
.param-tooltip {
    white-space: normal;  /* allow wrapping */
    max-width: 280px;
}
```

### IN-04: Faults JSON field uses non-omitempty serialization

**File:** `internal/hub/message.go:91`
**Issue:** The `Faults` field on `OutboundMessage` has the JSON tag `json:"faults"` without `omitempty`. The comment explains this is intentional ("never omit so frontend always renders fault card"). This is correct for the system section but means every `OutboundMessage` (including non-fault messages like `connection_state`, `section_error`, etc.) will include `"faults":null` in the JSON payload. This adds a small amount of unnecessary bytes to every WebSocket message.
**Fix:** This is by design per the comment. No change needed, but noting it for awareness. If bandwidth becomes a concern, the fault card rendering could be triggered by a dedicated message type instead.

---

_Reviewed: 2026-04-12T12:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
