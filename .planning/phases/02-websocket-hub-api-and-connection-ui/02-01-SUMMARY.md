---
phase: 02-websocket-hub-api-and-connection-ui
plan: 01
subsystem: api
tags: [websocket, modbus, broker, gorilla-websocket, hub]

# Dependency graph
requires:
  - phase: 01-foundation-and-modbus-service
    provides: broker with command-channel serialization, state machine, and TCP reconnect
provides:
  - Broker dormant start with Reconfigure() and Disconnect() commands
  - Hub message types defining the JSON WebSocket protocol contract
  - BrokerInterface for hub testability without real TCP connections
  - gorilla/websocket dependency
affects: [02-02-PLAN, 02-03-PLAN, 03-section-reader-and-data-pipeline]

# Tech tracking
tech-stack:
  added: [gorilla/websocket v1.5.3]
  patterns: [dormant-start broker, command-channel reconfigure/disconnect, interface-based broker abstraction]

key-files:
  created:
    - internal/hub/message.go
    - internal/hub/broker_iface.go
    - internal/hub/hub_test.go
  modified:
    - internal/broker/broker.go
    - internal/broker/state.go
    - internal/broker/broker_test.go
    - web/web_test.go
    - go.mod
    - go.sum

key-decisions:
  - "StateDormant = -1 with explicit constant values to avoid breaking existing iota chain"
  - "Reconfigure and Disconnect serialized through command channel like all other broker operations"
  - "dormant bool field gates ensureConnected and handleError to prevent unwanted reconnection"
  - "BrokerInterface compile-time check in test file to avoid production code coupling"

patterns-established:
  - "Dormant start pattern: broker starts inactive, requires explicit Reconfigure() before any connection"
  - "Command extension pattern: new CmdType + executeX handler for extending broker operations"
  - "Hub message envelope: type + section + data/state/error + timestamp"

requirements-completed: [CONN-02, RT-05]

# Metrics
duration: 5min
completed: 2026-04-10
---

# Phase 2 Plan 1: Broker Evolution + Hub Foundation Summary

**Dormant-start broker with runtime Reconfigure/Disconnect commands, hub WebSocket message types, and BrokerInterface abstraction**

## Performance

- **Duration:** 5 min
- **Started:** 2026-04-10T12:27:34Z
- **Completed:** 2026-04-10T12:33:02Z
- **Tasks:** 2
- **Files modified:** 9

## Accomplishments
- Broker starts in StateDormant -- no TCP connection on startup until explicit Reconfigure()
- Reconfigure(ctx, addr, slaveID) connects to new address; Disconnect(ctx) closes without auto-reconnect
- Hub package created with InboundMessage/OutboundMessage structs and 9 message type constants
- BrokerInterface enables hub unit testing without real TCP connections
- gorilla/websocket v1.5.3 added to go.mod for upcoming WebSocket hub

## Task Commits

Each task was committed atomically:

1. **Task 1: Broker dormant start, Reconfigure, and Disconnect commands** - `05cf773` (feat)
2. **Task 2: Add gorilla/websocket dependency and create hub message types + broker interface** - `249dd1f` (feat)

## Files Created/Modified
- `internal/broker/state.go` - Added StateDormant = -1, dormant case in String()
- `internal/broker/broker.go` - Added CmdReconfigure/CmdDisconnect, Reconfigure(), Disconnect(), dormant field, dormant guards
- `internal/broker/broker_test.go` - 4 new tests (DormantStart, Reconfigure, Disconnect, ReconfigureWhileConnected) + updated 3 existing tests
- `internal/hub/message.go` - InboundMessage, OutboundMessage structs with JSON tags, message type constants, constructor helpers
- `internal/hub/broker_iface.go` - BrokerInterface with Reconfigure, Disconnect, ReadBatch, CurrentState, StateEvents
- `internal/hub/hub_test.go` - Compile-time check that broker.Broker satisfies BrokerInterface
- `go.mod` - Added gorilla/websocket v1.5.3
- `go.sum` - Updated checksums
- `web/web_test.go` - Updated TestStatusEndpoint to expect dormant state

## Decisions Made
- StateDormant = -1 with explicit integer values for all states to avoid breaking the existing iota sequence
- Reconfigure and Disconnect flow through the same command channel as reads/writes, maintaining the single-goroutine serialization invariant
- dormant bool field on Broker controls whether ensureConnected attempts a connection and whether handleError triggers reconnection
- BrokerInterface compile-time satisfaction check placed in hub_test.go (not production code) to keep hub package clean

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Updated web_test.go for dormant state expectation**
- **Found during:** Task 2 (full project test verification)
- **Issue:** TestStatusEndpoint expected connection_state "disconnected" but broker now starts in "dormant"
- **Fix:** Changed expected value from "disconnected" to "dormant"
- **Files modified:** web/web_test.go
- **Verification:** go test ./web/ -v passes
- **Committed in:** 249dd1f (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 bug fix)
**Impact on plan:** Necessary fix for correctness after dormant start change. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Broker now supports runtime connection management needed by WebSocket hub
- Hub message types define the JSON protocol contract for Plans 02-02 (hub core) and 02-03 (connection UI)
- BrokerInterface ready for hub unit testing with mocks
- gorilla/websocket available for WebSocket handler implementation

---
## Self-Check: PASSED

All 7 files verified present. Both task commits (05cf773, 249dd1f) verified in git log.

---
*Phase: 02-websocket-hub-api-and-connection-ui*
*Completed: 2026-04-10*
