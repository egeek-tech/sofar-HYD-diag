---
phase: 08-refresh-architecture
fixed_at: 2026-04-12T13:43:00Z
review_path: .planning/phases/08-refresh-architecture/08-REVIEW.md
iteration: 1
findings_in_scope: 6
fixed: 5
skipped: 1
status: partial
---

# Phase 8: Code Review Fix Report

**Fixed at:** 2026-04-12T13:43:00Z
**Source review:** .planning/phases/08-refresh-architecture/08-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 6
- Fixed: 5
- Skipped: 1

## Fixed Issues

### CR-01: Data Race on Hub Fields Accessed from Goroutines

**Files modified:** `internal/hub/hub.go`
**Commit:** a7d7d69
**Applied fix:** Captured `h.packSettleMs` into a local variable `settleMs` before launching the `triggerPackRead` goroutine. The goroutine now reads the snapshot instead of the mutable hub field, eliminating the data race between the event loop (which writes `packSettleMs` via `handleConfigure`) and the pack read goroutine.

### WR-01: CSS Selector Injection via Unescaped User-Controlled Data

**Files modified:** `web/static/app.js`
**Commit:** 5cac8c9
**Applied fix:** Replaced manual `key.replace(/"/g, '\\"')` with `CSS.escape(key)` in `handleRegisterValue`'s `querySelector` call. `CSS.escape` properly handles all special characters (backslashes, brackets, quotes) that could break out of the attribute selector.

### WR-02: Skipped Test Stubs Left in Test Suite

**Files modified:** `internal/hub/hub_test.go`
**Commit:** c7e4b67
**Applied fix:** Removed all four permanently skipped Wave 0 test stubs (`TestStreamingRead`, `TestSectionSchema`, `TestTimingConfigure`, `TestPackSettleConfigure`) along with their section header comment. These stubs were never implemented despite comments saying "Plan 02 will implement." The production behaviors they target are already covered by existing integration tests.

### WR-03: Dead Code -- `buildGroupedResult` Has Incorrect Error Semantics

**Files modified:** `internal/hub/hub.go`
**Commit:** fe84b6c
**Applied fix:** Removed the entire `buildGroupedResult` function (39 lines). Confirmed zero callers in the codebase via grep. The function was superseded by the streaming read path (`streamStandardRead`) which correctly handles per-register errors instead of discarding all data on any single probe error.

### WR-05: Timing Window in `streamBatteryRead` Auto-Detect Re-trigger

**Files modified:** `internal/hub/hub_streaming.go`
**Commit:** 78fa5b8
**Applied fix:** fixed: requires human verification. Introduced a `retrigger` boolean flag in the `streamBatteryRead` goroutine. When auto-detection dispatches a section re-read via `h.funcs`, `retrigger` is set to `true` before returning. The deferred cleanup function checks this flag and skips `sec.reading.Store(false)` when a retrigger is pending. This keeps `reading == true` through the handoff to the event loop's `triggerSectionRead`, closing the window where a concurrent `read_cycle` could start a duplicate read.

## Skipped Issues

### WR-04: Faults Field Always Serialized in Non-System Messages

**File:** `internal/hub/message.go:91`
**Reason:** The reviewer's suggested fix (add `omitempty` to the JSON tag and send `[]FaultEntry{}` for system section when no faults) does not work correctly in Go. Go's `encoding/json` with `omitempty` omits both nil slices AND empty slices (`len == 0`). This means the system section's "No active faults" card would stop rendering -- the frontend checks `Array.isArray(msg.faults)` which requires the field to be present in the JSON payload. Implementing a correct fix would require changing the field type to `*[]FaultEntry` (pointer to slice) with `omitempty`, which is a larger refactor with regression risk that exceeds the benefit of saving a few bytes per non-system message.
**Original issue:** The `Faults` field in `OutboundMessage` uses `json:"faults"` without `omitempty`, causing every non-system message to include `"faults":null` in the JSON payload.

---

_Fixed: 2026-04-12T13:43:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
