---
phase: 08-refresh-architecture
verified: 2026-04-12T12:00:00Z
status: human_needed
score: 7/8
overrides_applied: 0
human_verification:
  - test: "Verify no burst reads on section switch (REL-03, SC-1)"
    expected: "Switching between sections rapidly (System -> Grid -> PV -> Battery) does not produce a burst of rapid Modbus reads. Server logs show consistent spacing (~500ms) between reads regardless of navigation."
    why_human: "Context cancellation code is wired correctly, but actual burst-prevention behaviour requires a live inverter connection and server log inspection to confirm timing in practice."
  - test: "Verify no autonomous reads when auto-refresh is off (REFR-01, SC-2)"
    expected: "After disabling auto-refresh (clicking Auto button), waiting 30 seconds produces zero new data updates and zero section_complete messages."
    why_human: "TestNoBackendTimer covers 2-second automated check. Real-world confirmation with an actual WebSocket session over 30s is not automatable without running the server."
  - test: "Verify browser-controlled cycle delay (REFR-02, SC-3)"
    expected: "After a read cycle completes, the browser waits the configured delay (5s, 10s, or 30s) before sending the next read_cycle. Continuous mode (0s) sends read_cycle immediately after section_complete."
    why_human: "setTimeout chaining is implemented correctly in code. Actual timing confirmation (that 5s really means ~5s gap, not <1s or >10s) requires browser observation."
  - test: "Verify stopping auto-refresh stops all Modbus reads immediately (SC-4)"
    expected: "Clicking Auto (off) immediately stops further data updates. No further section_complete messages arrive after the button is toggled off."
    why_human: "clearTimeout(refreshState.delayTimer) is coded correctly but the stop-immediately guarantee for in-progress reads (backend context cancellation) is best confirmed by observing server logs."
---

# Phase 8: Refresh Architecture — Verification Report

**Phase Goal:** Auto-refresh is driven entirely by the browser with consistent, predictable Modbus timing
**Verified:** 2026-04-12T12:00:00Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

Roadmap Success Criteria (SC) and Plan must-haves combined:

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| SC-1 | Switching sections does not cause burst reads — inter-read delay consistent regardless of navigation | ? HUMAN NEEDED | Context cancellation wired: subscribeClient calls oldSec.cancelRead() (hub.go:264), navigateToSection clears delayTimer (app.js:294-296). Needs live confirmation |
| SC-2 | Backend performs no autonomous refresh cycles — all reads initiated by browser | VERIFIED | Section struct has no ticker/stopCh/timerCh/interval. Hub has no timerCh or handleTimerTick. TestNoBackendTimer test passes (hub_test.go:853). MsgTypeAutoRefresh removed from message.go |
| SC-3 | After read cycle completes, browser waits configured delay before next cycle | ? HUMAN NEEDED | handleSectionComplete uses setTimeout after section_complete (app.js:1035-1041) with refreshState.cycleDelay. Code is correct but real timing needs browser confirmation |
| SC-4 | Stopping auto-refresh immediately stops all Modbus reads | ? HUMAN NEEDED | clearTimeout(refreshState.delayTimer) called on toggle off (app.js:418-421). Backend context cancellation wired on disconnect. Needs live verification |
| T-01 (Plan 01) | Section struct has no ticker, stopCh, timerCh, interval, or autoRefresh fields | VERIFIED | section.go contains only: Name, Probes, Groups, faultSection, subscribers, readCancel, reading, logger. No timer fields present |
| T-02 (Plan 01) | Hub has no timerCh, handleTimerTick, or handleAutoRefreshToggle | VERIFIED | grep confirms zero occurrences of timerCh field, handleTimerTick, handleAutoRefreshToggle in hub.go |
| T-03 (Plan 01) | Streaming read goroutines accept cancellable context and check ctx.Err() between probes | VERIFIED | hub_streaming.go: streamStandardRead(readCtx), streamBatteryRead(readCtx), streamBMSRead(readCtx) all check readCtx.Err() between probe reads |
| T-04 (Plan 01) | Subscribing to a new section cancels previous section's in-progress read | VERIFIED | subscribeClient (hub.go:263-264) calls oldSec.cancelRead() before unsubscribing; triggerSectionRead (hub.go:364) calls sec.cancelRead() before new read |
| T-05 (Plan 01) | read_cycle WebSocket message triggers a section read without backend timer | VERIFIED | message.go has MsgTypeReadCycle="read_cycle"; hub.go:225-226 routes to handleReadCycle; handleReadCycle calls triggerSectionRead |
| T-06 (Plan 01) | PackInfoProbes registers skipped when they return illegal address error | VERIFIED | hub.go:819-831 uses skipPackInfo atomic.Bool, checks for "0x02"/"illegal" in error string, skips on subsequent calls |
| T-07 (Plan 01) | All existing tests pass or are replaced with new architecture tests | VERIFIED | 6 new tests confirmed in hub_test.go: TestReadCycleMessage, TestSkipOverlappingReadCycle, TestReadsCancelledOnDisconnect, TestReadsWorkAfterReconnect, TestNoBackendTimer, TestCancelReadOnSectionSwitch. Full test suite passes: ok sofar-hyd-diag/internal/hub 55.929s |
| T-08 (Plan 02) | Auto-refresh controlled entirely by browser — no backend timer | VERIFIED | App.autoRefresh removed, refreshState object controls all refresh state (app.js:21-27). No auto_refresh WebSocket message sent |
| T-09 (Plan 02) | Cycle delay dropdown offers Continuous/5s/10s/30s presets and persists in localStorage | VERIFIED | index.html:109-114 has all 4 options. initCycleDelayDropdown (app.js:1109) reads/writes CYCLE_DELAY_KEY='sofar_cycle_delay' in localStorage |
| T-10 (Plan 02) | Auto button shows 'Auto (#N)' and N increments after each completed cycle | VERIFIED | updateAutoRefreshButton (app.js:450-451): 'Auto (#' + refreshState.cycleCount + ')'. cycleCount++ in handleSectionComplete (app.js:1031) |
| T-11 (Plan 02) | Cycle count resets when switching sections | VERIFIED | navigateToSection (app.js:298): refreshState.cycleCount = 0 |
| T-12 (Plan 02) | Manual Refresh button appears when auto-refresh is off, triggers single read cycle | VERIFIED | updateRefreshButtonVisibility (app.js:466-470): hides when refreshState.active is true. btn-refresh click handler (app.js:437-444) sends read_cycle |
| T-13 (Plan 02) | Stopping auto-refresh clears pending delay timers | VERIFIED | setupAutoRefreshToggle (app.js:418-421): clearTimeout(refreshState.delayTimer) when !refreshState.active |

**Score:** 7/8 must-haves fully automated-verified (SC-1, SC-3, SC-4 need human testing; SC-2 and all plan truths verified)

Note: 4 roadmap SCs evaluated — 1 VERIFIED, 3 HUMAN NEEDED. Plan truths 13 evaluated — all VERIFIED.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/hub/section.go` | Timer-free Section struct with readCancel context.CancelFunc | VERIFIED | readCancel at line 27; no ticker/stopCh/timerCh/interval/autoRefresh fields |
| `internal/hub/hub.go` | Hub with read_cycle handler, no timerCh | VERIFIED | handleReadCycle at line 329; MsgTypeReadCycle at line 225; no timerCh field |
| `internal/hub/hub_streaming.go` | Streaming reads with per-section cancellable context | VERIFIED | readCtx parameter on all 3 streaming functions (lines 34, 152, 224) |
| `internal/hub/message.go` | New MsgTypeReadCycle constant | VERIFIED | MsgTypeReadCycle = "read_cycle" at line 15; MsgTypeAutoRefresh absent |
| `internal/hub/hub_test.go` | Tests for read_cycle, cancellation, no-backend-timer | VERIFIED | 6 tests confirmed; full suite passes |
| `web/static/index.html` | Cycle delay dropdown + manual Refresh button in header controls | VERIFIED | cycle-delay-select (line 109) with 4 options; btn-refresh (line 116); correct source order |
| `web/static/style.css` | CSS for .btn-refresh class | VERIFIED | .btn-refresh at line 497 with height:32px, border-radius:16px, background-color:#e0e0e0 |
| `web/static/app.js` | Browser refresh state machine with read_cycle WebSocket messages | VERIFIED | refreshState object (line 21); CYCLE_DELAY_KEY (line 18); 3 locations sending read_cycle |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `internal/hub/hub.go` | `internal/hub/hub_streaming.go` | triggerSectionRead passes readCtx to streaming goroutines | VERIFIED | hub.go:381 calls h.streamStandardRead(sectionName, sec, readCtx) |
| `internal/hub/hub.go` | `internal/hub/section.go` | readCancel called before starting new read | VERIFIED | hub.go:364 sec.cancelRead() in triggerSectionRead; hub.go:264 oldSec.cancelRead() in subscribeClient |
| `web/static/app.js` | `internal/hub/hub.go` | WebSocket read_cycle message triggers handleReadCycle | VERIFIED | app.js sends {type:'read_cycle'}; hub.go:225 routes MsgTypeReadCycle to handleReadCycle |
| `web/static/app.js` | `web/static/app.js` | handleSectionComplete triggers setTimeout for next read_cycle | VERIFIED | handleSectionComplete (app.js:1035-1041): setTimeout -> App.ws.send({type:'read_cycle'}) |

### Data-Flow Trace (Level 4)

Not applicable — this phase produces a backend refactoring (no new data rendering components). Data continues to flow through the existing broker -> hub -> WebSocket -> frontend pipeline, unchanged in structure.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Build compiles (all embedded files valid) | `go build ./...` | Exit 0, no output | PASS |
| Full test suite passes | `go test ./... -timeout 120s` | All packages: ok | PASS |
| Hub test suite passes (includes 6 new architecture tests) | `go test ./internal/hub/ -count=1 -timeout 120s` | ok 55.929s | PASS |
| section.go has no timer fields | `grep -n "ticker\|stopCh\|timerCh\|interval\|autoRefresh" internal/hub/section.go` | No matches | PASS |
| app.js has no App.autoRefresh | `grep "App\.autoRefresh" web/static/app.js` | No matches | PASS |
| app.js has no auto_refresh WebSocket send | `grep "type.*auto_refresh" web/static/app.js` | No matches | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| REL-03 | 08-01-PLAN.md | Inter-read delay consistently enforced, no burst on section switch | ? HUMAN NEEDED | Context cancellation prevents orphaned reads (code verified). Burst-prevention under real navigation needs human confirmation |
| REFR-01 | 08-01-PLAN.md | No autonomous backend refresh cycles — all reads browser-initiated | VERIFIED | Section struct timer-free. TestNoBackendTimer confirms 0 autonomous reads after 2s. MsgTypeAutoRefresh removed |
| REFR-02 | 08-02-PLAN.md | Auto-refresh timer restarts after each read cycle completes (not fixed interval) | ? HUMAN NEEDED | setTimeout chaining after section_complete is correctly implemented. Actual timer restart behaviour needs browser observation |

All 3 phase requirements (REL-03, REFR-01, REFR-02) are claimed by plans. REFR-01 is fully automated-verified. REL-03 and REFR-02 are code-verified but require human testing for behavioural confirmation.

No orphaned requirements — all 3 IDs from REQUIREMENTS.md Phase 8 row are covered by plan frontmatter.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| hub_streaming.go | 14 | "placeholder" in comment | Info | Legitimate use — describes schema pre-rendering slots, not implementation stubs |
| app.js | 932-935 | "placeholder" variable name | Info | Legitimate use — DOM element for computed group containers, not a stub |

No blockers or implementation stubs found.

### Human Verification Required

#### 1. No burst reads on section switch (SC-1, REL-03)

**Test:** Connect to inverter. Navigate rapidly between sections (System -> Grid -> PV -> Battery). Check server debug logs.
**Expected:** Each Modbus read is spaced by the configured inter-read delay (~500ms default). No burst of 3-4 reads in quick succession after switching sections.
**Why human:** The context cancellation code is wired correctly at both the browser (clearTimeout in navigateToSection) and backend (oldSec.cancelRead() in subscribeClient). Confirming actual timing behaviour requires a live connection and log inspection.

#### 2. Backend performs no autonomous reads when auto-refresh is off (SC-2, REFR-01 — partial)

**Test:** Connect to inverter. Turn auto-refresh OFF. Wait 30 seconds.
**Expected:** Zero new data updates appear. No section_complete messages in browser DevTools WebSocket inspector.
**Why human:** TestNoBackendTimer covers a 2-second automated window. Extended real-world confirmation (30s) is not automatable without running the server.

#### 3. Browser-controlled cycle delay timing (SC-3, REFR-02)

**Test:** Enable auto-refresh. Set cycle delay to 5s. Watch the Auto (#N) counter and timestamps.
**Expected:** Counter increments approximately every 5 seconds (last_updated timestamp + ~5s cycle time). Switching to Continuous produces back-to-back cycles with minimal gap.
**Why human:** setTimeout(fn, refreshState.cycleDelay) is correctly wired. Actual elapsed time between read_cycle messages needs browser observation to confirm.

#### 4. Stopping auto-refresh stops reads immediately (SC-4)

**Test:** Enable auto-refresh with Continuous delay. Click Auto to disable. Observe server logs.
**Expected:** No further Modbus reads after the button is toggled off. The in-progress read (if any) completes but no new read_cycle is sent.
**Why human:** clearTimeout is correctly called in the toggle handler. The in-progress read abort (backend context) is confirmed by TestReadsCancelledOnDisconnect but the specific stop-auto-refresh path needs live observation.

### Gaps Summary

No automated-verifiable gaps found. All code artifacts exist, are substantive, and are correctly wired.

The phase goal is achieved at the code level. Three of the four roadmap success criteria require live-environment confirmation because they describe timing and absence-of-behaviour properties that are difficult to verify with static analysis alone. Human verification of the 4 test scenarios above is required before marking the phase as fully passed.

---

_Verified: 2026-04-12T12:00:00Z_
_Verifier: Claude (gsd-verifier)_
