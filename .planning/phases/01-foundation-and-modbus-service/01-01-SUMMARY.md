---
phase: 01-foundation-and-modbus-service
plan: 01
subsystem: modbus
tags: [modbus-tcp, modbus-rtu, crc16, go-modules, chi, slog]

# Dependency graph
requires: []
provides:
  - "internal/modbus package with TCP and RTU codecs"
  - "Project scaffold with module sofar-hyd-diag"
  - "Original main.go preserved as main.go.bak"
  - "chi/v5 dependency in go.mod"
affects: [01-02-broker, 01-03-web-server]

# Tech tracking
tech-stack:
  added: [github.com/go-chi/chi/v5@v5.2.5, log/slog]
  patterns: [internal package layout, slog logger injection, net.Pipe testing]

key-files:
  created:
    - internal/modbus/common.go
    - internal/modbus/tcp.go
    - internal/modbus/rtu.go
    - internal/modbus/modbus_test.go
    - main.go.bak
    - go.sum
  modified:
    - go.mod

key-decisions:
  - "CRC16 test vector uses uint16 native byte order (0x0A84) not wire order"
  - "transactionID kept as unexported package-level var (not exported as stated in plan) since tests are same-package"
  - "Added .gitkeep files to preserve empty directory structure in git"

patterns-established:
  - "Logger injection: all modbus functions accept *slog.Logger as parameter after conn"
  - "Same-package tests: modbus_test.go uses package modbus for internal access"
  - "net.Pipe mock testing: goroutine runs mock server, main thread runs client"

requirements-completed: [CONN-05]

# Metrics
duration: 5min
completed: 2026-04-10
---

# Phase 01 Plan 01: Project Scaffold and Modbus Codec Extraction Summary

**Modbus TCP/RTU protocol codecs extracted verbatim from proven CLI tool into internal/modbus package with slog debug logging and net.Pipe round-trip tests**

## Performance

- **Duration:** 5 min
- **Started:** 2026-04-10T10:06:45Z
- **Completed:** 2026-04-10T10:11:53Z
- **Tasks:** 3
- **Files modified:** 8

## Accomplishments
- Extracted all six Modbus protocol functions (TCP read/write, RTU read/write, ReadFull, CRC16) from monolithic main.go into structured internal/modbus package
- Established project scaffold with Go module renamed to sofar-hyd-diag, chi dependency, and full directory layout
- Seven protocol tests pass with race detector: CRC16 vector, ReadFull partial assembly, TCP read/write round-trips, TCP exception handling, RTU read with CRC validation, RTU CRC mismatch detection

## Task Commits

Each task was committed atomically:

1. **Task 1: Create project scaffold, rename module, backup original** - `2e0a854` (chore)
2. **Task 2: Extract Modbus TCP and RTU codecs into internal/modbus package** - `63b7e3c` (feat)
3. **Task 3: Write Modbus protocol tests using net.Pipe** - `a1203b4` (test)

## Files Created/Modified
- `go.mod` - Module renamed to sofar-hyd-diag, chi/v5 dependency added
- `go.sum` - Chi dependency checksum
- `main.go.bak` - Original 707-line CLI tool preserved for extraction verification
- `internal/modbus/common.go` - ReadFull, CRC16, Connect, DiscardLogger, transactionID counter
- `internal/modbus/tcp.go` - ReadHoldingRegistersTCP, WriteMultipleRegistersTCP with MBAP framing
- `internal/modbus/rtu.go` - ReadHoldingRegistersRTU, WriteSingleRegisterRTU with CRC16 validation
- `internal/modbus/modbus_test.go` - 7 tests covering all codecs via net.Pipe mock connections
- `cmd/server/.gitkeep`, `internal/broker/.gitkeep`, `internal/register/.gitkeep`, `web/static/.gitkeep` - Directory structure placeholders

## Decisions Made
- CRC16 test vector corrected to 0x0A84 (native uint16 byte order) rather than 0x840A (wire little-endian order) -- the CRC16 function returns a uint16, not bytes
- transactionID kept as unexported package-level variable since test file uses same-package access (package modbus, not modbus_test)
- Added .gitkeep files to track empty directories (cmd/server, internal/broker, internal/register, web/static) that will be populated by Plans 02 and 03

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Corrected CRC16 test vector byte order**
- **Found during:** Task 3 (TDD test execution)
- **Issue:** Plan specified CRC16 expected value as 0x840A but the CRC16 function returns uint16 in native order (0x0A84). The 0x840A value represents the wire-format little-endian byte sequence, not the uint16 return value.
- **Fix:** Changed test expectation from 0x840A to 0x0A84
- **Files modified:** internal/modbus/modbus_test.go
- **Verification:** TestCRC16 passes
- **Committed in:** a1203b4 (Task 3 commit)

---

**Total deviations:** 1 auto-fixed (1 bug in test vector)
**Impact on plan:** Minor test vector correction. No scope creep. Protocol logic unchanged.

## Issues Encountered
None -- all protocol extraction was straightforward since functions were well-isolated in the original main.go.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- internal/modbus package is ready for Plan 02 (broker layer) to wrap with connection management, retry logic, and scheduling
- All protocol functions have consistent signatures: `(conn net.Conn, logger *slog.Logger, slaveID byte, ...)` ready for broker abstraction
- chi/v5 dependency is ready for Plan 03 (web server)

---
*Phase: 01-foundation-and-modbus-service*
*Completed: 2026-04-10*
