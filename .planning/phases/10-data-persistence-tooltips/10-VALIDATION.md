---
phase: 10
slug: data-persistence-tooltips
status: draft
nyquist_compliant: true
wave_0_complete: true
created: 2026-04-12
---

# Phase 10 -- Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (existing) |
| **Config file** | none -- standard Go test infrastructure |
| **Quick run command** | `go test ./internal/hub/ -run Test -count=1 -timeout 30s` |
| **Full suite command** | `go test ./... -count=1 -timeout 60s` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/hub/ ./internal/register/ -run Test -count=1 -timeout 30s`
- **After every plan wave:** Run `go test ./... -count=1 -timeout 60s`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 10-01-T1 | 01 | 1 | DISP-03 | -- | N/A | unit | `go test ./internal/register/ -run TestFormatRawValue -count=1` | No (W0) | pending |
| 10-01-T2 | 01 | 1 | DISP-03 | -- | N/A | unit | `go test ./internal/hub/ -run TestNewRegisterValueJSON -count=1` | No (W0) | pending |
| 10-02-T1 | 02 | 2 | DISP-01 | -- | N/A | manual | Browser: verify dimming on read cycle | N/A | pending |
| 10-02-T2 | 02 | 2 | DISP-01, DISP-02, DISP-03 | T-10-03 | textContent not innerHTML | manual+grep | `grep -c "sectionCache" web/static/app.js && grep -c "data-register-addr" web/static/app.js` | N/A | pending |
| 10-02-T3 | 02 | 2 | DISP-01, DISP-02, DISP-03 | -- | N/A | manual | Browser: full UAT of dimming, caching, tooltips | N/A | pending |
| 10-03-T1 | 03 | 2 | DISP-03 | -- | N/A | unit | `go test ./internal/hub/ -run TestPackDataMessageItemMeta -count=1` | No (W0) | pending |
| 10-03-T2 | 03 | 2 | DISP-03 | T-10-06 | textContent not innerHTML | manual+grep | `grep -c "item_meta" web/static/app.js && grep -c "cell_addrs" web/static/app.js` | N/A | pending |
| 10-03-T3 | 03 | 2 | DISP-03 | -- | N/A | manual | Browser: verify pack drill-down tooltips | N/A | pending |

*Status: pending / green / red / flaky*

---

## Wave 0 Requirements

- [x] `internal/register/register_test.go` -- Plan 01 Task 1 creates `TestFormatRawValue` (TDD task, tests written first)
- [x] `internal/hub/hub_test.go` -- Plan 01 Task 2 creates `TestNewRegisterValueJSON` and `TestNewRegisterValueComposedJSON`
- [x] `internal/hub/hub_test.go` -- Plan 03 Task 1 creates `TestPackDataMessageItemMeta`
- [x] Verify existing test suite passes with new message fields

*All Wave 0 tests are created within their respective plan tasks (TDD and unit tests). No separate Wave 0 plan needed.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Values dim to 50% opacity on read cycle start | DISP-01 | Visual CSS behavior | Connect, start auto-refresh, observe dimming on each cycle |
| Cached values shown dimmed on section return | DISP-02 | Browser navigation state | Visit System, switch to Grid, switch back to System -- verify cached values appear dimmed |
| Tooltip shows register address + raw + timestamp | DISP-03 | Hover interaction | Hover over any parameter value, verify tooltip content format |
| Cache cleared on disconnect | DISP-02 | State lifecycle | Disconnect while viewing section, verify em-dash reset |
| Pack drill-down tooltips show register addresses | DISP-03 (D-15) | Hover interaction on pack view | Drill into a pack, hover over cell voltage and temperature values |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 5s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
