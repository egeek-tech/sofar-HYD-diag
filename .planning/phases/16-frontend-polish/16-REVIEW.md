---
phase: 16-frontend-polish
reviewed: 2026-04-14T12:00:00Z
depth: standard
files_reviewed: 4
files_reviewed_list:
  - internal/hub/hub_streaming.go
  - internal/hub/hub_test.go
  - web/static/app.js
  - web/static/style.css
findings:
  critical: 0
  warning: 3
  info: 5
  total: 8
status: issues_found
---

# Phase 16: Code Review Report

**Reviewed:** 2026-04-14T12:00:00Z
**Depth:** standard
**Files Reviewed:** 4
**Status:** issues_found

## Summary

Reviewed four files spanning Go backend streaming logic, test suite, JavaScript frontend, and CSS styles. The codebase is well-structured with consistent patterns, thorough test coverage, and good security practices (textContent used instead of innerHTML, CSS.escape for dynamic selectors, context-aware cancellation in goroutines).

Three warnings were found: a CSS property conflict that silently clips tooltip content, a JavaScript variable shadowing issue under `var` hoisting, and `console.warn` calls remaining in production code. Five informational items note minor code quality improvements. No critical or security issues were identified.

## Warnings

### WR-01: CSS `white-space: nowrap` conflicts with `max-width` causing tooltip text clipping

**File:** `web/static/style.css:1185-1186`
**Issue:** The `.param-tooltip` class has both `white-space: nowrap` and `max-width: 200px`. When tooltip content exceeds 200px (common for multiline pack status tooltips built in `showTooltip()` at app.js:1070-1101 which create multiple `<div>` children), `nowrap` prevents wrapping and `max-width` causes the container to clip via hidden overflow. The tooltip positioning in JS reads `tooltipEl.offsetWidth` after setting `display: ''`, but the rendered width is capped at 200px even though content is wider, leading to misaligned or truncated tooltips.
**Fix:** The tooltip content uses child `<div>` elements (block-level), so `white-space: nowrap` is needed only for inline content within each line. Change to:
```css
.param-tooltip {
    /* ... */
    white-space: normal;
    max-width: 320px;
}
```
Or remove `max-width` entirely since block-level children handle line breaks and the tooltip has `pointer-events: none` so it does not need tight bounds.

### WR-02: `var` hoisting causes `rawVal` variable shadowing in `updateTemperatureValue`

**File:** `web/static/app.js:1815-1856`
**Issue:** The `updateTemperatureValue` function declares `var rawVal` at line 1815 (inside the `rawVal === 0` early-return branch) and again at line 1856 (in the non-zero temperature tracking branch). While JavaScript `var` hoisting means both refer to the same function-scoped variable and the early return at line 1837 prevents actual conflict, this pattern obscures intent and is fragile: if the early return were ever removed or the control flow changed, the second `rawVal` assignment would silently overwrite the first. The same pattern exists with `var key` (line 1818) and `var key` (line 1862), and `var el` / `var el2` use different names presumably to avoid this.
**Fix:** Either use distinct variable names in each branch (as already done with `key2`/`el2`) or restructure into an early return guard:
```javascript
function updateTemperatureValue(msg) {
    if (!msg.error && msg.raw_value) {
        var rawVal = parseInt(msg.raw_value, 10);
        if (rawVal === 0) {
            // ... hide logic, return
            return;
        }
        // ... non-zero temperature logic uses same rawVal
    }
    // ...
}
```
This avoids re-declaring `rawVal` and makes the single-scope lifetime explicit.

### WR-03: `console.warn` calls in production frontend code

**File:** `web/static/app.js:1581,2104`
**Issue:** Two `console.warn` calls are left in production code paths -- one at line 1581 (pack info register suppression) and one at line 2104 (configuration register suppression). While these are intentional diagnostic aids, they will fire on every refresh cycle for every unsupported register, creating noise in the browser console for end users. In a desktop-focused tool this is low-severity but worth noting.
**Fix:** Either gate behind a debug flag or remove:
```javascript
// Option A: debug flag
if (window.SOFAR_DEBUG) {
    console.warn('[Config] Register unavailable:', msg.name, ...);
}
// Option B: remove entirely (errors are already handled by hiding the row)
```

## Info

### IN-01: Variable `h` shadows loop counter convention in hex fallback rendering

**File:** `web/static/app.js:2076,3039`
**Issue:** In `renderPackStatusFromState` (line 2076) and `renderPackStatusCard` (line 3039), the loop variable `var h` is used to iterate hex items. While `h` is a common short variable name, in this codebase `h` is also used for the hub variable in test code and could confuse readers. Using `var hi` or `var idx` would be clearer.
**Fix:** Rename the loop variable to something less ambiguous, e.g., `var hi = 0`.

### IN-02: Duplicated pack status rendering logic between streaming and batch paths

**File:** `web/static/app.js:2002-2085,2982-3050`
**Issue:** `renderPackStatusFromState` (streaming path, line 2002) and `renderPackStatusCard` (batch path, line 2982) contain nearly identical DOM construction logic for the all-clear and active-faults cases. If the status card UI changes, both functions must be updated in lockstep.
**Fix:** Extract a shared helper function `buildPackStatusDOM(card, alarm, protection, fault, alarm2, protection2, fault2, decoded)` and call it from both paths.

### IN-03: `renderFaultCard` uses `var heading` in both if/else branches

**File:** `web/static/app.js:848,868`
**Issue:** The `renderFaultCard` function declares `var heading` in both the `if` and `else` branches. Due to `var` hoisting these are the same variable, which works correctly but reads as if two separate variables are intended.
**Fix:** Declare `var heading` once before the `if` block, or switch to block-scoped `let` if the project adopts ES6 variable declarations.

### IN-04: Test helper `TestSystemSectionTimeComposition` has dead code

**File:** `internal/hub/hub_test.go:1158-1169`
**Issue:** The variable `foundComposed` is declared (line 1158) and assigned inside a `continue` branch (line 1167) but never meaningfully used -- it is underscore-consumed at line 1169 (`_ = foundComposed`). The test comment at lines 1171-1174 acknowledges that `OutboundMessage` does not capture `RegisterValueMessage` fields, making this assertion path dead code. The test effectively only checks that `section_complete` arrives.
**Fix:** Either remove the dead code block (lines 1156-1169) and update the test name to reflect what it actually verifies, or add raw JSON assertion logic to properly verify the composed time value.

### IN-05: Test file uses `time.Sleep` for synchronization throughout

**File:** `internal/hub/hub_test.go` (multiple locations, e.g., lines 356, 364, 400, 431)
**Issue:** The test suite relies heavily on `time.Sleep` for synchronization (20-100ms sleeps between operations). While this is a common pattern for concurrent Go tests and works reliably with the chosen timeouts, it makes the test suite slower than necessary and creates potential flakiness on slow CI machines.
**Fix:** This is not actionable in the short term -- the hub's channel-based architecture makes event-driven synchronization in tests non-trivial. Consider adding synchronization hooks (e.g., a test-only "drain funcs channel" helper) in a future refactoring phase.

---

_Reviewed: 2026-04-14T12:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
