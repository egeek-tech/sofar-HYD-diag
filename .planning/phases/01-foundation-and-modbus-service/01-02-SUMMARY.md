---
phase: 01-foundation-and-modbus-service
plan: 02
subsystem: modbus
tags: [broker, channel-concurrency, backoff, register-probes, formatting]

# Dependency graph
requires:
  - "01-01: internal/modbus package with TCP/RTU codecs and Connect function"
provides:
  - "internal/register package with Probe type, FormatValue, and all section definitions"
  - "internal/broker package with channel-based command queue and auto-reconnect"
  - "Backoff utility with exponential escalation and cap"
  - "Connection state events via StateEvents channel"
affects: [01-03-web-server, 02-websocket, 05-battery-pack]

# Tech tracking
tech-stack:
  added: []
  patterns: [channel-based command queue, exponential backoff reconnection, external package tests with net.Listen mock, retry-once on error]

key-files:
  created:
    - internal/register/probe.go
    - internal/register/format.go
    - internal/register/system.go
    - internal/register/battery.go
    - internal/register/register_test.go
    - internal/broker/broker.go
    - internal/broker/backoff.go
    - internal/broker/state.go
    - internal/broker/broker_test.go
  modified: []

key-decisions:
  - "Broker retries once on communication error (matching original readWithRetry pattern) before returning failure to caller"
  - "SetInterReadDelay and SetBackoff methods exposed for test configuration without affecting production defaults"
  - "CurrentState() method instead of State() to avoid collision with State type name"
  - "BDUProbes added to battery.go (not in plan but present in main.go.bak probe definitions)"

patterns-established:
  - "External package tests: broker_test uses package broker_test with net.Listen mock server"
  - "Mock Modbus server: goroutine accepts connection, reads MBAP requests, sends canned responses"
  - "Probe organization: one file per register section (system, battery), exported slice variables"
  - "FormatValue returns string, caller decides display -- separation of formatting from presentation"

requirements-completed: [CONN-04, CONN-05, INFRA-03]

# Metrics
duration: 4min
completed: 2026-04-10
---

# Phase 01 Plan 02: Register Definitions and Broker Summary

**Channel-based Modbus broker serializing all operations through single goroutine with exponential backoff reconnection, plus centralized register probe definitions with FormatValue formatting**

## Performance

- **Duration:** 4 min
- **Started:** 2026-04-10T10:13:42Z
- **Completed:** 2026-04-10T10:18:33Z
- **Tasks:** 2
- **Files modified:** 9

## Accomplishments
- Extracted all register probe definitions from main.go.bak into organized internal/register package: SystemProbes, GridProbes, EPSProbes, PVProbes, BatteryProbes, BatteryStateProbes, BMSProbes, BDUProbes
- FormatValue function produces identical output to original formatResult for all data types (ASCII, signed, unsigned, scaled, raw)
- Concurrency-safe broker with buffered command channel (32) ensures single-goroutine TCP connection ownership -- no mutex, no race conditions
- Auto-reconnect with exponential backoff (1s base, 30s cap) and retry-once on communication errors
- State events emitted on buffered channel for WebSocket push in Phase 2
- 11 tests passing with race detector across both packages

## Task Commits

Each task was committed atomically:

1. **Task 1: Extract register definitions and formatting into internal/register** - `2e9e536` (feat)
2. **Task 2: Create concurrency-safe broker with command channel and auto-reconnect** - `d5bb425` (feat)

## Files Created/Modified
- `internal/register/probe.go` - Probe struct type with exported PascalCase fields
- `internal/register/format.go` - FormatValue function (ASCII, signed, unsigned, scaled formatting)
- `internal/register/system.go` - SystemProbes, GridProbes, EPSProbes, PVProbes register definitions
- `internal/register/battery.go` - BatteryProbes, BatteryStateProbes, BMSProbes, BDUProbes register definitions
- `internal/register/register_test.go` - 6 tests covering all FormatValue data type branches
- `internal/broker/broker.go` - Broker type with Run, ReadRegisters, WriteRegister, ReadBatch, StateEvents
- `internal/broker/backoff.go` - Exponential backoff with base/max/reset
- `internal/broker/state.go` - State enum (Disconnected, Connecting, Connected, Reconnecting) and StateEvent
- `internal/broker/broker_test.go` - 5 tests: serialization, reconnect, backoff, context cancellation, batch timing

## Decisions Made
- Broker retries once on communication error before returning failure -- this matches the original readWithRetry pattern from main.go.bak where the CLI tool had maxRetries=3 but the broker delegates retrying to callers for higher-level retry policies
- SetInterReadDelay and SetBackoff methods exposed publicly for test configuration (production code uses defaults from constructor)
- Named CurrentState() instead of State() to avoid conflict with the State type in the same package
- BDUProbes included in battery.go even though the plan's code example for battery.go already included them -- they were present in main.go.bak probes and belong in the register package

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed int16 constant overflow in register test**
- **Found during:** Task 1 (register test compilation)
- **Issue:** Go does not allow negative int16 constants directly in uint16() cast: `uint16(int16(-83))` fails at compile time
- **Fix:** Used intermediate int16 variable: `neg83 := int16(-83); binary.BigEndian.PutUint16(data, uint16(neg83))`
- **Files modified:** internal/register/register_test.go
- **Verification:** All 6 register tests pass
- **Committed in:** 2e9e536 (Task 1 commit)

**2. [Rule 2 - Missing Critical] Added retry-once on broker read/write errors**
- **Found during:** Task 2 (TestBrokerReconnect failure)
- **Issue:** Broker returned error immediately on communication failure without attempting reconnect and retry. The original readWithRetry had retry logic that the plan's broker design omitted.
- **Fix:** Added retry loop (maxAttempts=2) in executeRead and executeWrite -- on error, handleError closes connection, then ensureConnected reconnects before second attempt
- **Files modified:** internal/broker/broker.go
- **Verification:** TestBrokerReconnect passes (reconnects to new server and reads successfully)
- **Committed in:** d5bb425 (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (1 bug, 1 missing critical)
**Impact on plan:** Both fixes necessary for correctness. No scope creep.

## Issues Encountered
None -- all issues were resolved via auto-fix deviation rules.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- internal/broker package ready for Plan 03 web server to wire into HTTP handlers
- Broker.ReadRegisters and ReadBatch ready for API endpoints to call
- StateEvents channel ready for Phase 2 WebSocket connection state push
- Register probe definitions ready for section-based reading in all phases
- FormatValue ready for formatting register values in API responses

## Self-Check: PASSED

All 9 files verified present. Both commit hashes (2e9e536, d5bb425) verified in git log.

---
*Phase: 01-foundation-and-modbus-service*
*Completed: 2026-04-10*
