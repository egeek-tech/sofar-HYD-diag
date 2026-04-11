---
status: complete
phase: 07-streaming-display-and-configurable-timing
source: [07-01-SUMMARY.md, 07-02-SUMMARY.md, 07-03-SUMMARY.md]
started: 2026-04-11T22:30:00Z
updated: 2026-04-11T22:45:00Z
---

## Current Test

[testing complete]

## Tests

### 1. Per-register streaming display
expected: Navigate to System section while connected. Values appear one-by-one as each register is read (approximately one every 500ms at default delay). Em-dash placeholders show for unloaded values. No batch appearance — values fill in progressively.
result: passed

### 2. Skeleton loading state
expected: Switch between sections (e.g., System → Grid → System). On re-entering a section, all register names appear immediately as a skeleton with em-dash (—) placeholders in muted style, then values stream in progressively.
result: passed

### 3. Timing controls visible and functional
expected: When connected, "Read Delay:" and "Pack Settle:" number inputs are visible in the header bar with dark text on white background. Default values are 500ms and 1000ms. Labels, values, and "ms" units are all clearly readable.
result: passed

### 4. Read delay change takes effect
expected: Change Read Delay to 50ms. On the next read cycle, values stream in noticeably faster (roughly 10x faster than default 500ms). No reconnection needed — change applies immediately on next cycle.
result: issue
reported: "if I set 1000ms, sometimes it read parameter per 1s and if I change to other page then it read much faster like with 100ms"
severity: major

### 5. Timing persistence across refresh
expected: Set Read Delay to 200ms. Refresh the page (F5). After reconnecting, the Read Delay input still shows 200ms (restored from localStorage). The backend uses 200ms for the next read cycle.
result: skipped
reason: Marked as unstable — needs further validation after Test 4 delay issue is resolved

### 6. BMS section streaming with computed groups
expected: Navigate to BMS section. Individual register values (CAN Protocol Ver, Manufacturer, BMS Version, etc.) stream in one-by-one. After all reads complete, the Battery Topology bitmap and Protection card appear as batch-rendered groups.
result: passed

## Summary

total: 6
passed: 4
issues: 1
pending: 0
skipped: 1
blocked: 0

## Gaps

- truth: "Read delay change takes effect on next read cycle consistently"
  status: failed
  reason: "User reported: setting 1000ms works sometimes but switching sections resets to faster speed (~100ms). Delay not consistently applied across section transitions."
  severity: major
  test: 4
  artifacts:
    - internal/hub/hub.go
    - internal/hub/hub_streaming.go
    - internal/broker/broker.go
  missing: []
