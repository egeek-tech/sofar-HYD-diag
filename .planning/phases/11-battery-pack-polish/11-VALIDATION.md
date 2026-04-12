---
phase: 11
slug: battery-pack-polish
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-12
---

# Phase 11 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing (stdlib), 1.26 |
| **Config file** | None -- standard `go test` |
| **Quick run command** | `go test ./internal/hub/ -run TestPack -v -count=1` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/hub/ -run TestPack -v -count=1`
- **After every plan wave:** Run `go test ./... -count=1`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 11-01-01 | 01 | 1 | BATT-01 | — | N/A | unit | `go test ./internal/register/ -run TestPackProbeGroupOrder -v -count=1` | ❌ W0 | ⬜ pending |
| 11-01-02 | 01 | 1 | BATT-01 | — | N/A | unit | `go test ./internal/hub/ -run TestPackSchemaGroupOrder -v -count=1` | ❌ W0 | ⬜ pending |
| 11-01-03 | 01 | 1 | BATT-02 | — | N/A | unit | `go test ./internal/hub/ -run TestPackStreamingMessages -v -count=1` | ❌ W0 | ⬜ pending |
| 11-01-04 | 01 | 1 | BATT-02 | — | N/A | unit | `go test ./internal/hub/ -run TestPackSchemaContext -v -count=1` | ❌ W0 | ⬜ pending |
| 11-01-05 | 01 | 1 | D-04 | — | N/A | unit | `go test ./internal/hub/ -run TestPackSkipUnsupported -v -count=1` | ❌ W0 | ⬜ pending |
| 11-01-06 | 01 | 1 | D-05 | — | N/A | unit | `go test ./internal/hub/ -run TestPackSkipResetOnSwitch -v -count=1` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/register/battery_test.go` -- test for `PackProbeGroups()` group ordering
- [ ] `internal/hub/hub_test.go` -- tests for pack streaming message flow, schema context, skip register logic

*Existing test infrastructure covers framework setup. Only test stubs needed.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Pack values stream visually per-register in browser | BATT-02 | Visual streaming behavior in real-time UI | Open browser, navigate to pack drill-down, observe values appearing one-by-one |
| Balance section appears above Temperature section | BATT-01 | Visual layout verification | Open pack drill-down, verify section order: Info, Cells, Balance, Temps, Status |
| Dimming, caching, tooltips work on pack views | D-01 | Cross-feature integration | Refresh pack view, verify dimming on refresh; navigate away and back, verify cache; hover values for tooltips |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
