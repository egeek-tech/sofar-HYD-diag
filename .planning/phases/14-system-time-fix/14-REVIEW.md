---
phase: 14-system-time-fix
reviewed: 2026-04-13T00:00:00Z
depth: standard
files_reviewed: 6
files_reviewed_list:
  - internal/register/format.go
  - internal/register/system.go
  - internal/register/register_test.go
  - internal/hub/hub_streaming.go
  - internal/hub/hub_test.go
  - web/static/app.js
findings:
  critical: 0
  warning: 3
  info: 3
  total: 6
status: issues_found
---

# Phase 14: Code Review Report

**Reviewed:** 2026-04-13
**Depth:** standard
**Files Reviewed:** 6
**Status:** issues_found

## Summary

This phase introduces system time composition for the streaming path: `ComposeSystemTime` in `format.go`, a synthetic `Count: 0` probe in `system.go`, and a batch read of 6 time registers after the "Status" group in `hub_streaming.go`. The core logic is sound and the Sofar register layout (0x042C=year, 0x042D=month ... 0x0431=sec) is correctly mapped to the `ComposeSystemTime(year, month, day, hour, min, sec)` call.

Three warnings were found: a group-name-only guard that could misfire on future section additions, a test that asserts `section_complete` was received but does not actually verify the composed time value or output format, and a duplicate `var` declaration in `renderProtectionGroup` that shadows a loop variable under strict mode linting. Three info items cover minor style/clarity opportunities.

---

## Warnings

### WR-01: Group-name guard for time read is not section-scoped

**File:** `internal/hub/hub_streaming.go:77`
**Issue:** The batch read of 6 time registers is triggered solely by `g.Name == "Status"`. If any future section (e.g., a combined status page) were to include a group named "Status", the streaming loop would silently attempt to read 0x042C-0x0431 for that section too, potentially returning garbage or timing out. The check is inside `streamStandardRead`, but that function is called for all standard sections.
**Fix:** Guard with both the group name and the section name:
```go
if g.Name == "Status" && sectionName == "system" && readCtx.Err() == nil {
```

---

### WR-02: `TestSystemSectionTimeComposition` sets up incorrect mock data and does not verify the composed value

**File:** `internal/hub/hub_test.go:1126-1184`
**Issue:** The test populates `batchResults[7]` through `batchResults[12]` intending them to represent the 6 time registers. However, the streaming code reads time registers via `ReadRegisters(ctx, 0x042C, 6)`, which routes through `ReadBatch(ctx, [{Addr: 0x042C, Count: 6}])` — a single batch call returning one 12-byte blob, not 6 separate 2-byte results. The time registers in `batchResults` will never be the indexed calls `[7]` through `[12]`; the batch call would get whatever is at `batchResults[batchCallCount]` at that point in the call sequence.

As a result, the composed value is never tested — the test only asserts that `section_complete` arrives, which passes trivially even if the time read returns zeros. The `foundComposed` variable is set and then immediately discarded (`_ = foundComposed`), confirming the composition assertion was abandoned.
**Fix:** Use `registerResults` (per-address mock) and then drain raw JSON to assert the "System time" register value appears with the expected formatted string:
```go
mb.mu.Lock()
mb.registerResults = map[uint16]broker.Result{
    // Time registers as a single 12-byte read from 0x042C
    // ReadRegisters routes through ReadBatch as one call
}
mb.mu.Unlock()
// The correct approach: set mb.batchResults with the 12-byte time blob
// at the right position, OR set registerResults with a 12-byte result
// keyed to 0x042C.
//
// Then assert:
// found register_value with name=="System time" && value=="14:30:45 16-03-2026"
```
A minimal correct fix is to replace `mb.batchResults` with a properly sized 12-byte result for the time read, and then scan raw JSON messages for a `register_value` with `"name":"System time"` and the expected formatted value.

---

### WR-03: Duplicate `var heading` declarations in `renderProtectionGroup` cause implicit variable shadowing

**File:** `web/static/app.js:2276,2294`
**Issue:** `renderProtectionGroup` uses `var heading` in both the `if (hasActive)` and `else` branches. Because `var` is function-scoped in JavaScript, this is a duplicate declaration in the same function scope. In `'use strict'` mode this is still legal but triggers warnings from all major linters (ESLint `no-redeclare`). If the code is ever transpiled or moved to `let`/`const`, this would become an error. The same pattern exists in `renderFaultCard` (lines 821, 841) and `renderPackStatusFromState` (lines 1890, 1901).
**Fix:** Declare `heading` once at the top of the function and assign it in each branch:
```javascript
function renderProtectionGroup(group) {
    var card = document.createElement('div');
    var items = group.items || {};
    var keys = Object.keys(items);
    var heading; // single declaration

    // ...
    if (hasActive) {
        heading = document.createElement('h3');
        // ...
    } else {
        heading = document.createElement('h3');
        // ...
    }
}
```

---

## Info

### IN-01: `ComposeSystemTime` output format is not separately verified against edge cases

**File:** `internal/register/register_test.go:459-471`
**Issue:** `TestComposeSystemTime` tests two cases: a normal date and the zero epoch. It does not test month/day boundary values (e.g., December=12, Day=31) or the year rollover (e.g., year=99 should render as 2099). These are low-risk since the logic is a single `fmt.Sprintf`, but the test coverage could be more complete.
**Fix:** Add a third case: `ComposeSystemTime(99, 12, 31, 23, 59, 59)` should return `"23:59:59 31-12-2099"`.

---

### IN-02: Synthetic probe `Count: 0` guard in streaming loop is correct but undocumented at the call site

**File:** `internal/hub/hub_streaming.go:45-47`
**Issue:** The `if p.Count == 0 { continue }` guard skips synthetic probes (the "System time" placeholder). This is the right behavior, but the comment says "synthetic probe: schema placeholder only (D-07)" without linking to which probe in the schema is synthetic. A future reader editing `system.go` might add another `Count: 0` probe for a different reason and be surprised it is silently skipped in the read loop.
**Fix:** Cross-reference the comment with the probe address. For example:
```go
if p.Count == 0 {
    continue // synthetic probe (e.g., "System time" at 0x042C): schema-only, read separately below
}
```

---

### IN-03: `packCellState.totalCells` is initialized to 16 but never consulted

**File:** `web/static/app.js:1219`
**Issue:** `packCellState.totalCells = 16` is declared but `computeCellSummary` and `applyCellDeviationColors` iterate over `Object.keys(packCellState.values)` (i.e., whatever cells have arrived), not up to `totalCells`. The field is dead code.
**Fix:** Either use `totalCells` in the summary computation to distinguish "no data yet" from "partial data arrived", or remove the field entirely.

---

_Reviewed: 2026-04-13_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
