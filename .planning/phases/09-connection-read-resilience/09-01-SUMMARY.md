---
phase: 09-connection-read-resilience
plan: 01
subsystem: broker
tags: [modbus, tcp, disconnect, abort, mutex, concurrency]

# Dependency graph
requires:
  - phase: 02-websocket-hub-api-and-connection-ui
    provides: broker command channel architecture, hub section read management
  - phase: 08-refresh-architecture
    provides: browser-driven refresh, section cancelRead mechanism
provides:
  - abortRead mechanism for immediate disconnect during blocking TCP reads
  - connMu mutex protecting conn from concurrent abort/close races
  - aborting flag to suppress executeRead retry after abort
  - hub disconnect handler cancels section reads before broker disconnect
affects: [09-02, connection-management, read-resilience]

# Tech tracking
tech-stack:
  added: []
  patterns: [abort-via-deadline, mutex-protected-conn, atomic-abort-flag]

key-files:
  created: []
  modified:
    - internal/broker/broker.go
    - internal/hub/hub.go
    - internal/broker/broker_test.go

key-decisions:
  - "Use SetReadDeadline(time.Now()) to abort blocking TCP reads -- non-blocking, safe for concurrent call per Go net.Conn docs"
  - "Add aborting atomic.Bool flag to skip executeRead retry after abort -- prevents reconnect-and-block-again loop"
  - "Protect all conn-clearing paths with connMu to prevent race between abortRead and conn close/replace"

patterns-established:
  - "Abort-via-deadline: Set read deadline to now to unblock blocking TCP reads from outside the Run() goroutine"
  - "Atomic abort flag: Use atomic.Bool to signal cross-goroutine abort intent without channel coordination"

requirements-completed: [REL-01]

# Metrics
duration: 7min
completed: 2026-04-12
---

# Phase 09 Plan 01: Immediate Disconnect Abort Summary

**Broker abortRead mechanism with connMu mutex, aborting flag, and hub section cancel-before-disconnect for <1s disconnect during blocking TCP reads**

## Performance

- **Duration:** 7 min
- **Started:** 2026-04-12T15:18:28Z
- **Completed:** 2026-04-12T15:25:16Z
- **Tasks:** 3
- **Files modified:** 3

## Accomplishments
- Broker abortRead() method that sets conn.SetReadDeadline(time.Now()) to immediately unblock any pending TCP read
- connMu mutex protecting all conn-clearing paths (executeDisconnect, handleError, cleanup, executeReconfigure) from races with abortRead
- Hub MsgTypeDisconnect handler cancels all section reads before calling broker.Disconnect
- Two new tests proving <2s disconnect during blocking reads and no-panic on nil conn

## Task Commits

Each task was committed atomically:

1. **Task 1: Add abortRead mechanism and connMu mutex to broker** - `15edea3` (feat)
2. **Task 2: Update hub disconnect handler to cancel section reads first** - `ea60a8f` (feat)
3. **Task 3 deviation: Add aborting flag to prevent retry after abort** - `3567bd7` (fix)
4. **Task 3: Add disconnect abort tests to broker test suite** - `e096bf4` (test)

## Files Created/Modified
- `internal/broker/broker.go` - Added abortRead(), connMu mutex, aborting atomic.Bool flag, protected all conn-clearing paths
- `internal/hub/hub.go` - Added cancelRead() loop before broker.Disconnect in MsgTypeDisconnect handler
- `internal/broker/broker_test.go` - Added TestBrokerAbortRead, TestBrokerAbortReadNoConn, slowMockServer helper

## Decisions Made
- Used SetReadDeadline(time.Now()) rather than conn.Close() for abort -- non-destructive, lets the Run goroutine handle conn lifecycle
- Added atomic.Bool aborting flag to suppress executeRead retry after abort -- prevents the retry from reconnecting and blocking again on the same slow server
- Protect conn with sync.Mutex only for abort/close paths -- inside Run() goroutine, conn reads for data do NOT need the mutex since Run is the only reader

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Added aborting flag to prevent retry after abort**
- **Found during:** Task 3 (writing tests, RED phase)
- **Issue:** After abortRead() caused the first TCP read to fail via deadline, executeRead's retry logic would call handleError (closing conn), then ensureConnected (reconnecting to the same server), and block again on a new TCP read. The CmdDisconnect command could not be processed until executeRead completed, negating the abort mechanism.
- **Fix:** Added atomic.Bool `aborting` field to Broker. abortRead() sets it to true before setting the deadline. executeRead checks the flag after handleError and skips retry if set. executeDisconnect clears the flag.
- **Files modified:** internal/broker/broker.go
- **Verification:** TestBrokerAbortRead passes -- Disconnect completes in ~200ms instead of 10s
- **Committed in:** 3567bd7

---

**Total deviations:** 1 auto-fixed (1 bug fix)
**Impact on plan:** Essential for correctness -- without the aborting flag, the abort mechanism would not actually prevent the 10s blocking read during disconnect. No scope creep.

## Issues Encountered
None beyond the deviation documented above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Broker disconnect abort mechanism complete, tested, and integrated with hub
- Ready for 09-02 (register read retry / stale value persistence) which builds on the same broker infrastructure
- All existing tests pass (broker: 1.6s, hub: 55.9s, full suite green)

## Self-Check: PASSED

All 3 modified files verified present. All 4 commit hashes verified in git log.

---
*Phase: 09-connection-read-resilience*
*Completed: 2026-04-12*
