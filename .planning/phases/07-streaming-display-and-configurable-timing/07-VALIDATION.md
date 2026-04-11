---
phase: 7
slug: streaming-display-and-configurable-timing
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-11
---

# Phase 7 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — standard Go test tooling |
| **Quick run command** | `go test ./internal/...` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/...`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD | TBD | TBD | STREAM-01 | — | N/A | unit | `go test ./internal/hub/...` | TBD | ⬜ pending |
| TBD | TBD | TBD | STREAM-02 | — | N/A | unit | `go test ./internal/hub/...` | TBD | ⬜ pending |
| TBD | TBD | TBD | TIMING-01 | — | N/A | unit | `go test ./internal/broker/...` | TBD | ⬜ pending |
| TBD | TBD | TBD | TIMING-02 | — | N/A | unit | `go test ./internal/hub/...` | TBD | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

*Existing infrastructure covers all phase requirements.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Values stream visually in real-time | STREAM-01 | Visual UX behavior | Connect to inverter, navigate to section, observe values filling in one-by-one |
| Timing controls visible in header | TIMING-01 | Visual layout check | Verify inputs appear in header bar with correct labels |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 5s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
