---
phase: 06-battery-pack-access-fix
reviewed: 2026-04-11T19:30:00Z
depth: standard
files_reviewed: 12
files_reviewed_list:
  - cmd/server/main.go
  - internal/hub/export_test.go
  - internal/hub/hub.go
  - internal/hub/hub_test.go
  - internal/hub/message.go
  - internal/register/battery.go
  - internal/register/register_test.go
  - web/handler.go
  - web/static/app.js
  - web/static/index.html
  - web/static/style.css
  - web/web_test.go
findings:
  critical: 0
  warning: 3
  info: 4
  total: 7
status: issues_found
---

# Phase 06: Code Review Report

**Reviewed:** 2026-04-11T19:30:00Z
**Depth:** standard
**Files Reviewed:** 12
**Status:** issues_found

## Summary

Reviewed all 12 source files for the battery pack access fix (Phase 06). The codebase is well-structured with clean separation between the hub event loop, register definitions, message types, web handler, and frontend. The Go code follows project conventions, uses proper error handling, and the test suite is thorough with good coverage of pack selection, bitmap cycling, and edge cases.

Three warnings were identified: a bug in the min-cell-index computation when zero-valued (offline) cells are interspersed with real values, a context leak in a test helper, and the BMS bitmap cycling performing blocking sleeps inside a goroutine without context cancellation checks. Four informational items relate to variable shadowing in the JavaScript, a redundant input clamping pattern, and a minor test resource concern.

## Warnings

### WR-01: Min cell index computation incorrect when zero-valued cells are interspersed

**File:** `internal/hub/hub.go:1166-1177`
**Issue:** The min-cell-index scanning logic tries to skip zero-valued cells (offline/no-data) with the condition `v < minVal || (minVal == 0 && v > 0)`. This breaks when zero-valued cells appear after non-zero cells because `v < minVal` (where v=0) is true, resetting `minVal` to 0. On the next non-zero cell, the `minVal == 0 && v > 0` branch fires, replacing the true minimum with a later (possibly higher) value. For example, with cells `[3200, 0, 3150, 0, 3180]`, the algorithm returns minIdx=5 (3180mV) instead of the correct minIdx=3 (3150mV).
**Fix:**
```go
maxIdx, minIdx := 1, 1
maxVal := cells[0]
minVal := -1 // sentinel: no valid min found yet
for i, v := range cells {
    if v > maxVal {
        maxVal = v
        maxIdx = i + 1
    }
    if v > 0 && (minVal < 0 || v < minVal) {
        minVal = v
        minIdx = i + 1
    }
}
```

### WR-02: BMS bitmap cycling blocks goroutine without context cancellation check

**File:** `internal/hub/hub.go:646-664`
**Issue:** The `triggerBMSRead` goroutine performs a blocking loop of `WriteRegister` + `time.Sleep(500ms)` + `ReadBatch` for each tower (currently 2 towers, ~2-3s total). During this loop, the hub context (`h.ctx`) is not checked between iterations. If the hub shuts down or the user disconnects during the bitmap cycling, the goroutine continues sleeping and issuing Modbus writes/reads until the loop completes. This delays graceful shutdown by up to 2-3 seconds and issues unnecessary Modbus commands after the user has disconnected.
**Fix:** Add a context check between loop iterations:
```go
for t := 0; t < TopoTowers; t++ {
    select {
    case <-h.ctx.Done():
        return
    default:
    }
    queryWord := uint16(t) << 8
    err := h.broker.WriteRegister(h.ctx, 0x9020, queryWord)
    // ... rest of loop
}
```

### WR-03: Context cancel function leaked in test helper newTestRouter

**File:** `web/web_test.go:26-28`
**Issue:** The `newTestRouter()` helper creates a `context.WithCancel` but assigns the cancel function to `_ = cancel` with a comment "cleanup happens when test ends (short-lived)". The cancel function is never called, meaning the hub goroutine and its associated context leak until the process exits. While harmless in short tests, it causes the `go vet` copylocks/context leak detector to flag this pattern, and the goroutine is technically orphaned.
**Fix:** Return the cancel function or defer it:
```go
func newTestRouter(t *testing.T) *chi.Mux {
    t.Helper()
    // ...
    ctx, cancel := context.WithCancel(context.Background())
    t.Cleanup(cancel)
    go h.Run(ctx)
    // ...
}
```

## Info

### IN-01: Redundant input clamping logic in handleSelectPack

**File:** `internal/hub/hub.go:1005-1009`
**Issue:** The input validation clamps input to exactly 1 with two separate checks (`if input < 1 { input = 1 }` and `if input > 1 { input = 1 }`). This is correct for the current single-input hardware but reads oddly. A single assignment `input = 1` would be clearer for the current hardcoded topology.
**Fix:**
```go
// Single input supported in current hardware topology
input = 1
```

### IN-02: Variable shadowing of `h` in renderPackStatusCard

**File:** `web/static/app.js:1451`
**Issue:** The loop variable `var h = 0` in the hex fallback rendering path shadows the outer scope. In JavaScript with `var` (function-scoped, not block-scoped), this does not cause a bug because the function is self-contained, but it is a code quality concern that could confuse readers since `h` typically refers to the hub or a DOM element in other functions.
**Fix:** Rename the loop variable to `var idx` or `var k` to avoid confusion.

### IN-03: Variable `select` used as identifier in initPVDropdown

**File:** `web/static/app.js:697`
**Issue:** `var select = $('#pv-channel-select')` uses `select` as a variable name. While not a reserved word in non-strict contexts in all browsers, `select` is an HTML element name and could be confusing. In strict mode (`'use strict'` is enabled at line 3), this is technically valid JavaScript but could trip up linters.
**Fix:** Rename to `var pvSelect` for clarity.

### IN-04: Faults field in OutboundMessage never omitted for system section

**File:** `internal/hub/message.go:78`
**Issue:** The `Faults` field uses `json:"faults"` without `omitempty`, meaning it serializes as `"faults":null` for non-system sections. The comment on the same line explains this is intentional ("never omit so frontend always renders fault card"). This is noted as an architectural decision, not a bug -- the frontend correctly checks `Array.isArray(msg.faults)` at `app.js:505`.
**Fix:** No action needed. Documenting that this is by design.

---

_Reviewed: 2026-04-11T19:30:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
