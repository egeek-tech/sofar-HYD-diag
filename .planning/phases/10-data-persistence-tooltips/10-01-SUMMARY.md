---
phase: 10-data-persistence-tooltips
plan: 01
subsystem: api
tags: [modbus, websocket, register-metadata, tooltip-backend]

# Dependency graph
requires:
  - phase: 07-streaming-display-and-configurable-timing
    provides: RegisterValueMessage struct, NewRegisterValue function, streaming call sites
provides:
  - RegisterValueMessage with register_addr and raw_value JSON fields
  - FormatRawValue utility for raw register value extraction
  - Updated call sites passing register metadata through WebSocket
affects: [10-02-PLAN, 10-03-PLAN, frontend-tooltip-rendering]

# Tech tracking
tech-stack:
  added: []
  patterns: [composed-values-pass-zero-addr, raw-value-always-unsigned]

key-files:
  created: []
  modified:
    - internal/hub/message.go
    - internal/hub/hub_streaming.go
    - internal/register/register_test.go
    - internal/hub/hub_test.go

key-decisions:
  - "Composed values (System time, System Clock, SW Version) pass addr=0 and empty raw_value since they span multiple registers"
  - "raw_value is always unsigned decimal for numeric probes, hex for ASCII probes"
  - "register_addr always present in JSON (no omitempty) so frontend can rely on the field"

patterns-established:
  - "Composed register values use addr=0 to signal multi-register origin"
  - "FormatRawValue extracts pre-scaling unsigned value for tooltip display"

requirements-completed: [DISP-03]

# Metrics
duration: 3min
completed: 2026-04-12
---

# Phase 10 Plan 01: Register Metadata for Tooltips Summary

**Extended RegisterValueMessage with register_addr and raw_value fields, updated all 8 streaming call sites, added FormatRawValue tests and JSON verification tests**

## Performance

- **Duration:** 3 min
- **Started:** 2026-04-12T17:34:50Z
- **Completed:** 2026-04-12T17:37:57Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- Added RegisterAddr (uint16) and RawValue (string) fields to RegisterValueMessage for frontend tooltip consumption
- Updated NewRegisterValue signature and all 8 call sites in hub_streaming.go to pass register address and raw value
- Added 6 table-driven TestFormatRawValue subtests covering uint16, uint32, ASCII, empty data, single byte, signed
- Added TestNewRegisterValueJSON and TestNewRegisterValueComposedJSON verifying JSON output shape
- Full test suite passes (go test ./... -count=1) and go vet clean

## Task Commits

Each task was committed atomically:

1. **Task 1: Add FormatRawValue tests** - `c234ee3` (test)
2. **Task 2: Extend RegisterValueMessage and update all call sites** - `c28414a` (feat)

## Files Created/Modified
- `internal/hub/message.go` - Extended RegisterValueMessage struct, updated NewRegisterValue signature
- `internal/hub/hub_streaming.go` - Updated all 8 NewRegisterValue call sites with addr and rawValue params
- `internal/register/register_test.go` - Added TestFormatRawValue table-driven tests (6 subtests)
- `internal/hub/hub_test.go` - Added TestNewRegisterValueJSON and TestNewRegisterValueComposedJSON

## Decisions Made
- Composed values (System time, System Clock, SW Version) pass addr=0 and empty raw_value since they aggregate multiple registers and have no single address
- RegisterAddr field uses no omitempty so it always appears in JSON (frontend can rely on field presence)
- raw_value uses omitempty so composed/error cases produce cleaner JSON

## Deviations from Plan

None - plan executed exactly as written. FormatRawValue and its section.go alias were already created by a previous agent; this plan added only the tests, message struct extension, and call site updates.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- RegisterValueMessage now carries register_addr and raw_value for all streaming sections (system, battery, bms)
- Frontend tooltip rendering (Plan 10-02/10-03) can consume these fields from the WebSocket JSON protocol
- Composed values identifiable by addr=0

---
*Phase: 10-data-persistence-tooltips*
*Completed: 2026-04-12*
