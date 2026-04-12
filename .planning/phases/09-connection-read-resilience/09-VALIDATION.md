---
phase: 9
slug: connection-read-resilience
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-12
---

# Phase 9 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — stdlib testing |
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
| 09-01-01 | 01 | 1 | REL-01 | — | N/A | unit | `go test ./internal/broker/...` | ❌ W0 | ⬜ pending |
| 09-01-02 | 01 | 1 | REL-01 | — | N/A | unit | `go test ./internal/hub/...` | ❌ W0 | ⬜ pending |
| 09-02-01 | 02 | 1 | REL-02 | — | N/A | unit | `go test ./internal/broker/...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/broker/broker_test.go` — test stubs for disconnect abort and retry logic
- [ ] `internal/hub/hub_test.go` — test stubs for disconnect handler cancellation

*Existing infrastructure covers test framework — Go stdlib testing already available.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Disconnect aborts within 1s | REL-01 | Requires live Modbus TCP connection with blocking read | Connect to inverter, start read cycle, click disconnect, measure time to disconnected state |
| Retry suppresses transient UI errors | REL-02 | Requires live Modbus TCP with intermittent errors | Induce error conditions, observe UI shows value without transient error flash |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 5s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
