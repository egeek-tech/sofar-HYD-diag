---
phase: 02-websocket-hub-api-and-connection-ui
plan: 02
subsystem: api
tags: [websocket, hub, goroutine, timer, modbus, section, auto-refresh]

# Dependency graph
requires:
  - phase: 02-websocket-hub-api-and-connection-ui
    provides: BrokerInterface, InboundMessage/OutboundMessage, message type constants, gorilla/websocket
provides:
  - Hub event loop with client register/unregister, command dispatch, state broadcast
  - Client read/write pumps with 30s ping / 60s pong keep-alive
  - Section registry with subscriber management and auto-refresh timers
  - Demo "status" section reading Inverter SN, Running State, Ambient Temp
  - Timer pause on disconnect, resume on reconnect
  - Skip-overlapping-read guard via atomic bool
affects: [02-03-PLAN, 03-section-reader-and-data-pipeline]

# Tech tracking
tech-stack:
  added: []
  patterns: [hub-goroutine event loop, per-client read/write pump, channel-based thread-safe queries, sectionResult channel for race-free broadcast]

key-files:
  created:
    - internal/hub/hub.go
    - internal/hub/client.go
    - internal/hub/section.go
    - internal/hub/export_test.go
  modified:
    - internal/hub/hub_test.go
    - go.mod
    - go.sum

key-decisions:
  - "All hub state mutations happen in single Run() goroutine; external queries routed via funcs channel"
  - "Section read results routed back to hub via sectionResult channel to avoid race on subscriber maps"
  - "Timer goroutine captures ticker.C locally to avoid race with stopTimer setting ticker = nil"
  - "Probe type alias (Probe = register.Probe) keeps hub package API clean while using register types"
  - "refreshOverride field on Hub enables short timer intervals in tests without modifying production defaults"

patterns-established:
  - "Hub event loop pattern: single goroutine owns all mutable state, channels for all external communication"
  - "RunFunc pattern: execute arbitrary function on hub goroutine for thread-safe state access"
  - "sectionResult pattern: async read goroutine sends results back to event loop for broadcasting"
  - "Timer capture pattern: local variable captures for goroutine to avoid struct field races"

requirements-completed: [RT-01, RT-02, RT-03, RT-04, RT-05]

# Metrics
duration: 9min
completed: 2026-04-10
---

# Phase 2 Plan 2: Hub Core, Sections, and Timers Summary

**Hub event loop with client lifecycle, section subscription triggering immediate Modbus reads, auto-refresh timers with pause/resume on connection state, and demo "status" section reading 3 inverter registers**

## Performance

- **Duration:** 9 min
- **Started:** 2026-04-10T12:36:04Z
- **Completed:** 2026-04-10T12:45:00Z
- **Tasks:** 2
- **Files created/modified:** 7

## Accomplishments

- Hub event loop processes register/unregister/command/stateEvents/timerCh/results/funcs in a single select
- Client type with ReadPump (JSON decode + forward to hub) and WritePump (JSON write + 30s ping, D-08)
- Section registry with subscriber tracking, auto-refresh timer per section (10s default, D-19)
- Subscribe triggers immediate Modbus read and data push to client (D-20, RT-01)
- One section per client: subscribing to new auto-unsubscribes previous (D-18)
- Auto-refresh timer skips overlapping reads via atomic bool guard (D-24)
- Timers pause on broker disconnect, resume on reconnect (D-28)
- Per-section error events when ReadBatch returns errors (D-09, RT-04)
- Connection state changes broadcast to ALL clients (D-04)
- Demo "status" section reads Inverter SN (0x0445), Running State (0x0404), Ambient Temp 1 (0x0418) per D-25
- Values pre-formatted server-side via register.FormatValue (D-27)
- Manual refresh command triggers immediate read regardless of timer state (D-23)
- 15 tests passing with race detector: interface check, register/unregister, connect/disconnect commands, state broadcast, subscribe/immediate-read, single-section, auto-refresh, skip-overlapping, pause/resume, errors, manual refresh, demo probes

## Task Commits

Each task was committed atomically:

1. **Task 1: Hub event loop and Client read/write pumps** - `12fa933` (feat)
2. **Task 2: Section registry, timer management, and demo status section** - `7f4dc62` (feat)

## Files Created/Modified

- `internal/hub/hub.go` (412 lines) - Hub type with Run() event loop, client management, command dispatch, section subscription, timer handling, broadcast methods
- `internal/hub/client.go` (95 lines) - Client type with ReadPump/WritePump goroutines, ping/pong keep-alive constants
- `internal/hub/section.go` (128 lines) - Section type with subscriber management, auto-refresh timer, atomic reading guard, toSnakeCase helper
- `internal/hub/export_test.go` (42 lines) - Test helpers: NewTestHub, NewTestHubWithInterval, NewTestClient, GetSectionProbes
- `internal/hub/hub_test.go` (709 lines) - 15 tests with mockBroker, message collection helpers
- `go.mod` - gorilla/websocket v1.5.3 dependency restored (was missing from prior commit)
- `go.sum` - Updated checksums

## Decisions Made

- All hub state mutations happen in a single Run() goroutine; external queries are routed through a `funcs` channel that executes closures on the hub goroutine
- Section read results are routed back to the hub event loop via a `sectionResult` channel, avoiding data races when broadcasting to subscriber maps
- Timer goroutines capture the ticker channel and section name locally to avoid struct field races when stopTimer sets ticker = nil
- `Probe = register.Probe` type alias keeps the hub package API clean while sharing types with the register package
- `refreshOverride` field on Hub enables short timer intervals in tests without modifying production defaults (50ms vs 10s)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed data race in timer goroutine**
- **Found during:** Task 2 (race detector flagged startTimer goroutine)
- **Issue:** Timer goroutine's `for range s.ticker.C` referenced struct field while stopTimer set `s.ticker = nil` from hub goroutine
- **Fix:** Captured `s.ticker.C`, `s.Name`, and `s.timerCh` as local variables before launching goroutine
- **Files modified:** internal/hub/section.go
- **Committed in:** 7f4dc62

**2. [Rule 1 - Bug] Fixed data race on hub.clients map access**
- **Found during:** Task 1 (race detector flagged ClientCount reading map from test goroutine)
- **Issue:** `ClientCount()` directly read `h.clients` map while Run() goroutine modified it
- **Fix:** Introduced `funcs` channel; ClientCount sends closure to hub goroutine for execution, receives result via reply channel
- **Files modified:** internal/hub/hub.go
- **Committed in:** 12fa933

**3. [Rule 1 - Bug] Fixed data race on broadcastToSection from read goroutine**
- **Found during:** Task 2 (read goroutine accessed hub.sections and sec.subscribers)
- **Issue:** `triggerSectionRead` goroutine called `broadcastToSection` which accessed maps owned by Run() goroutine
- **Fix:** Introduced `sectionResult` channel; read goroutine sends results back to event loop which performs the broadcast
- **Files modified:** internal/hub/hub.go
- **Committed in:** 7f4dc62

**4. [Rule 3 - Blocking] Restored gorilla/websocket dependency**
- **Found during:** Task 1 (go.mod missing gorilla/websocket despite Plan 02-01 claiming it was added)
- **Issue:** go.mod only had chi dependency; gorilla/websocket was listed in 02-01-SUMMARY but not present
- **Fix:** Ran `go get github.com/gorilla/websocket@v1.5.3`
- **Files modified:** go.mod, go.sum
- **Committed in:** 12fa933

---

**Total deviations:** 4 auto-fixed (3 race conditions, 1 missing dependency)
**Impact on plan:** All fixes were necessary for correctness under race detector. No scope creep.

## Issues Encountered
None beyond the auto-fixed deviations above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Hub event loop ready for WebSocket upgrade handler (Plan 02-03)
- Client ReadPump/WritePump ready to be wired to gorilla/websocket Upgrader
- Section subscription pipeline validated end-to-end (subscribe -> ReadBatch -> FormatValue -> broadcast)
- "status" section serves as demo for connection UI testing

---
## Self-Check: PASSED

All files verified present. Both task commits (12fa933, 7f4dc62) verified in git log.

---
*Phase: 02-websocket-hub-api-and-connection-ui*
*Completed: 2026-04-10*
