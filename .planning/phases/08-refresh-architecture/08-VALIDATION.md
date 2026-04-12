---
phase: 8
slug: refresh-architecture
status: draft
nyquist_compliant: true
wave_0_complete: true
created: 2026-04-12
---

# Phase 8 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing (stdlib) |
| **Config file** | None needed (Go conventions) |
| **Quick run command** | `go test ./internal/hub/ -run TestAutoRefresh -count=1 -timeout 120s` |
| **Full suite command** | `go test ./... -timeout 120s` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/hub/ -count=1 -timeout 120s`
- **After every plan wave:** Run `go test ./... -timeout 120s`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 08-01-01 | 01 | 1 | REL-03 | — | N/A | integration | `go test ./internal/hub/ -run TestInterReadDelayAcrossSectionSwitch -count=1` | ❌ W0 | ⬜ pending |
| 08-01-02 | 01 | 1 | REFR-01 | — | N/A | unit | `go test ./internal/hub/ -run TestNoBackendTimer -count=1` | ❌ W0 | ⬜ pending |
| 08-01-03 | 01 | 1 | D-02 | — | N/A | integration | `go test ./internal/hub/ -run TestCancelReadOnSectionSwitch -count=1` | ❌ W0 | ⬜ pending |
| 08-01-04 | 01 | 1 | D-13 | — | N/A | integration | `go test ./internal/hub/ -run TestStopAutoRefreshStopsReads -count=1` | ❌ W0 | ⬜ pending |
| 08-02-01 | 02 | 2 | REFR-02 | — | N/A | manual | Manual browser test: verify setTimeout fires after section_complete | manual-only | ⬜ pending |
| 08-02-02 | 02 | 2 | D-09 | — | N/A | manual | Manual browser test: check button text updates | manual-only | ⬜ pending |
| 08-02-03 | 02 | 2 | D-11 | — | N/A | manual | Manual browser test: verify button swap | manual-only | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/hub/hub_test.go` — add `TestInterReadDelayAcrossSectionSwitch` (REL-03)
- [ ] `internal/hub/hub_test.go` — add `TestNoBackendTimer` (REFR-01)
- [ ] `internal/hub/hub_test.go` — add `TestCancelReadOnSectionSwitch` (D-02)
- [ ] `internal/hub/hub_test.go` — add `TestStopAutoRefreshStopsReads` (D-13)
- [ ] `internal/hub/hub_test.go` — add `TestReadCycleMessage` for new message type
- [ ] `internal/hub/hub_test.go` — update/remove existing timer tests (`TestAutoRefreshTimer`, `TestSkipOverlappingTick`, `TestTimerPausesOnDisconnect`, `TestTimerResumesOnReconnect`, `TestAutoRefreshToggleStopsTimer`)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Browser cycle delay timer fires after section_complete | REFR-02 | JS frontend, no test framework | 1. Open app, connect. 2. Enable auto-refresh. 3. Observe cycle delay between refreshes matches dropdown setting. |
| Auto button shows "Auto (#N)" | D-09 | UI text rendering | 1. Enable auto-refresh. 2. Verify button text shows "Auto (#1)", increments after each cycle. |
| Manual Refresh button shown when auto off | D-11 | UI visibility | 1. Disable auto-refresh. 2. Verify "Refresh" button appears. 3. Click it, verify single read cycle triggers. |
| Cycle delay dropdown persists in localStorage | D-07 | Browser persistence | 1. Set cycle delay to 10s. 2. Reload page. 3. Verify dropdown still shows 10s. |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
