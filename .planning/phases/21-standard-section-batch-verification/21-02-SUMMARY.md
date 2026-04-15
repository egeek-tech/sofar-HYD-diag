---
phase: 21-standard-section-batch-verification
plan: 02
subsystem: register
tags: [modbus, batch-verification, hardware-sweep, register-cleanup, dcdc]
dependency_graph:
  requires: [tools/section-sweep/main.go, internal/register/dcdc.go, internal/hub/hub.go]
  provides: [tools/section-sweep/results.json, tools/section-sweep/results1.json]
  affects: [internal/register/dcdc.go, internal/hub/hub.go]
tech_stack:
  added: []
  patterns: [hardware-verified-register-removal, empty-section-deregistration]
key_files:
  created: [tools/section-sweep/results.json, tools/section-sweep/results1.json]
  modified: [internal/register/dcdc.go, internal/hub/hub.go]
decisions:
  - "Entire DCDC section emptied: all 25 registers (4 groups) failed on real hardware"
  - "DCDCGroups kept as empty slice rather than deleted, since other tools reference the variable"
  - "Hub registration for DCDC removed to avoid registering an empty section"
  - "DCDCRunningStateEnum left in dcdc_enum.go as dead code (file also contains PCUModuleStateEnum)"
  - "Frontend DCDC nav button left in place -- out of scope for this plan, noted as deferred"
metrics:
  duration: "337s"
  completed: "2026-04-15T21:48:00Z"
  tasks_completed: 1
  tasks_total: 1
  files_created: 2
  files_modified: 2
---

# Phase 21 Plan 02: Hardware Sweep Results and Register Cleanup Summary

Committed hardware sweep results from real inverter and removed all failing DCDC registers (25/25 probes failed); Grid, EPS, PV, Meter, PCU, BDU sections passed 100% with no changes needed.

## Task Execution

### Task 1: Run section sweep on real hardware (checkpoint:human-action)
- **Status:** Completed by user prior to executor start
- **Files created:** `tools/section-sweep/results.json`, `tools/section-sweep/results1.json`
- **Results:** 82 passed, 15 failed (illegal data address 0x02), 10 errors (timeout) -- all failures in DCDC section

### Task 2: Remove failing registers and update tests
- **Status:** Complete
- **Commit:** `ef6a7dc`
- **Files modified:** `internal/register/dcdc.go`, `internal/hub/hub.go`
- **Files committed:** `tools/section-sweep/results.json`, `tools/section-sweep/results1.json`

**Actions taken:**
1. Parsed results.json: confirmed all 25 DCDC probes failed (15 FAIL + 10 ERROR)
2. Confirmed Grid (33/33), EPS (14/14), PV (7/7), Meter (2/2), PCU (18/18), BDU (8/8) all PASS -- no changes needed
3. Emptied DCDCGroups in dcdc.go (all 4 groups removed: System Info, Real-Time Data, Faults, Capacity)
4. Added hardware verification comment to dcdc.go
5. Removed DCDC section registration from hub.go (empty section)
6. Verified DCDCRunningStateEnum is now dead code but compiles fine (shares file with PCUModuleStateEnum)
7. Confirmed no DCDC tests exist in register_test.go -- no test updates needed
8. All tests pass: `go test ./internal/register/` and `go test ./...`

## Section Results Summary

| Section | Probes | Passed | Failed | Errors | Source Changes |
|---------|--------|--------|--------|--------|----------------|
| Grid    | 33     | 33     | 0      | 0      | None           |
| EPS     | 14     | 14     | 0      | 0      | None           |
| PV      | 7      | 7      | 0      | 0      | None           |
| Meter   | 2      | 2      | 0      | 0      | None           |
| DCDC    | 25     | 0      | 15     | 10     | All probes removed, section deregistered |
| PCU     | 18     | 18     | 0      | 0      | None           |
| BDU     | 8      | 8      | 0      | 0      | None           |

## DCDC Probes Removed

| Group | Probe | Addr | Status | Error |
|-------|-------|------|--------|-------|
| System Info | DCDC SN | 0x5029 | ERROR | timeout |
| Real-Time Data | Parallel DCDC count | 0x5044 | ERROR | timeout |
| Real-Time Data | System running state | 0x5046 | ERROR | timeout |
| Real-Time Data | Power limiting state | 0x5047 | FAIL | illegal data address |
| Real-Time Data | SOC balanced power | 0x5048 | FAIL | illegal data address |
| Real-Time Data | LV side bus voltage | 0x504E | ERROR | timeout |
| Real-Time Data | LV side current | 0x5050 | ERROR | timeout |
| Real-Time Data | LV side power | 0x5051 | FAIL | illegal data address |
| Real-Time Data | HV bus voltage | 0x505C | ERROR | timeout |
| Real-Time Data | Insulation impedance | 0x505E | ERROR | timeout |
| Real-Time Data | Internal env temp | 0x5060 | ERROR | timeout |
| Real-Time Data | Radiator temp 1 | 0x5061 | FAIL | illegal data address |
| Real-Time Data | Radiator temp 2 | 0x5062 | FAIL | illegal data address |
| Faults | Fault 1 | 0x5065 | ERROR | timeout |
| Faults | Fault 2 | 0x5066 | FAIL | illegal data address |
| Faults | Fault 3 | 0x5067 | FAIL | illegal data address |
| Faults | Fault 4 | 0x5068 | FAIL | illegal data address |
| Faults | Fault 5 | 0x5069 | FAIL | illegal data address |
| Faults | Fault 6 | 0x506A | FAIL | illegal data address |
| Faults | Fault 7 | 0x506B | FAIL | illegal data address |
| Faults | Fault 8 | 0x506C | FAIL | illegal data address |
| Faults | Fault 9 | 0x506D | FAIL | illegal data address |
| Faults | Fault 10 | 0x506E | FAIL | illegal data address |
| Capacity | Total charge capacity | 0x5074 | ERROR | timeout |
| Capacity | Total discharge capacity | 0x5076 | FAIL | illegal data address |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Removed DCDC hub registration**
- **Found during:** Task 2
- **Issue:** With all DCDC groups removed, registering an empty section in hub.go would create a non-functional section entry
- **Fix:** Removed `h.RegisterGroupedSection("dcdc", register.DCDCGroups)` from hub.go, added comment explaining why
- **Files modified:** internal/hub/hub.go
- **Commit:** ef6a7dc

## Deferred Items

1. **Frontend DCDC navigation button** -- `web/static/index.html` line 87 still has a hardcoded DCDC nav button. With the section deregistered, clicking it will not load any data. A future plan should either remove the button or make section nav dynamic based on registered sections.

2. **DCDCRunningStateEnum dead code** -- `internal/register/dcdc_enum.go` still contains the `DCDCRunningStateEnum` map which is no longer referenced. It compiles fine (package-level var) and shares the file with `PCUModuleStateEnum`. Can be cleaned up in a future pass.

## Verification Results

| Check | Result |
|-------|--------|
| results.json valid JSON | PASS |
| All FAIL/ERROR probes removed from dcdc.go | PASS (0 of 25 addresses found) |
| Empty groups removed | PASS (all 4 groups removed) |
| Hub registration removed | PASS |
| `go build ./...` | PASS |
| `go test ./internal/register/ -count=1` | PASS |
| `go test ./... -count=1` | PASS |
| No unexpected file deletions | PASS |

## Known Stubs

None.

## Self-Check: PASSED
