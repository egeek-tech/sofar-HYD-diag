---
phase: quick
plan: 260418-m4i
subsystem: testing
tags: [coverage, unit-tests, register, hub, modbus, broker]
dependency_graph:
  requires: []
  provides: [test-coverage-improvement]
  affects: [internal/register, internal/hub, internal/modbus, internal/broker]
tech_stack:
  added: []
  patterns: [table-driven-tests, net.Pipe-mock, mock-tcp-server]
key_files:
  created:
    - internal/hub/section_test.go
  modified:
    - internal/register/register_test.go
    - internal/modbus/modbus_test.go
    - internal/broker/broker_test.go
decisions:
  - Used package-internal test file for hub/section_test.go to access unexported toSnakeCase and newSection
  - Created dedicated mockWriteServer for broker write test since mockModbusServer reads only 12 bytes (read request size) but write requests are 15 bytes
  - Adapted OutboundMessage test to use actual struct fields (Type, Data) instead of plan's incorrect Payload field
metrics:
  duration: ~8m
  completed: 2026-04-18T14:11:00Z
  tasks_completed: 2
  tasks_total: 2
  files_changed: 4
---

# Quick Task 260418-m4i: Add Tests for Uncovered Functions Summary

Test coverage for 15 uncovered functions across 4 packages (register, hub, modbus, broker), raising overall coverage from ~65.7% to 82.7%.

## Task Results

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | register + hub package tests | 22bc68d | internal/register/register_test.go, internal/hub/section_test.go |
| 2 | modbus + broker package tests | 05fbe9b | internal/modbus/modbus_test.go, internal/broker/broker_test.go |

## Tests Added

### register package (1 test)
- **TestInternalInfoGroups**: Validates return structure -- 1 group named "Internal Info", 5 probes, correct first/last addresses (0x06CC, 0x06ED)

### hub package (4 tests)
- **TestToSnakeCase**: Table-driven with 5 cases including spaces, parentheses, already-snake, and empty string
- **TestNewSection**: Verifies constructor initializes Name, Probes, subscribers map, zero-value BatchPlan, nil SpanTracker
- **TestRegisterSection**: Confirms section added to hub's internal map with correct properties
- **TestBroadcastToSection**: Verifies JSON message delivery to subscribers via send channel; confirms no-panic on nonexistent section

### modbus package (4 tests)
- **TestConnect**: Success against real TCP listener; failure against port 1 (connection refused)
- **TestDiscardLogger**: Returns non-nil functional logger that does not panic on use
- **TestWriteSingleRegisterRTU**: Full RTU write cycle -- verifies 8-byte request frame (slaveID, func 0x06, regAddr, value, CRC), echo response parsing
- **TestWriteSingleRegisterRTU_Exception**: Verifies exception response (func|0x80) returns error containing "exception"

### broker package (3 tests)
- **TestBrokerAddress**: Simple getter returns constructor value without requiring Run()
- **TestBrokerSetDelayRuntime**: Updates delay to 200ms via command channel; verifies timing between 2 consecutive reads
- **TestBrokerWriteRegister**: Full write cycle through broker command channel using dedicated mockWriteServer; exercises both WriteRegister (public) and executeWrite (internal)

## Coverage Results

| Package | Before | After |
|---------|--------|-------|
| internal/register | ~90% | 96.2% |
| internal/hub | ~75% | 80.3% |
| internal/modbus | ~65% | 81.2% |
| internal/broker | ~65% | 77.8% |
| **Total** | **~65.7%** | **82.7%** |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed BatchPlan.Groups reference**
- **Found during:** Task 1
- **Issue:** Plan referenced `BatchPlan.Groups` but actual struct field is `BatchPlan.Spans`
- **Fix:** Changed assertion to use `sec.BatchPlan.Spans`
- **Files modified:** internal/hub/section_test.go

**2. [Rule 1 - Bug] Fixed OutboundMessage Payload field**
- **Found during:** Task 1
- **Issue:** Plan referenced `Payload: json.RawMessage(...)` but OutboundMessage has no Payload field
- **Fix:** Used actual struct fields (Type, Section, Data) for test message construction
- **Files modified:** internal/hub/section_test.go

**3. [Rule 1 - Bug] Fixed undefined err variable**
- **Found during:** Task 2
- **Issue:** Used `err =` instead of `err :=` in TestWriteSingleRegisterRTU (new scope)
- **Fix:** Changed to `:=` declaration
- **Files modified:** internal/modbus/modbus_test.go

## Known Stubs

None -- all tests exercise real function behavior with no placeholder data.
