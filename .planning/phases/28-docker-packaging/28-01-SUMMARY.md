---
phase: 28-docker-packaging
plan: 01
subsystem: infra
tags: [docker, health-endpoints, env-vars, version-injection, ldflags]

# Dependency graph
requires:
  - phase: 26-milestone-cleanup
    provides: stable v1.5 server with chi router and broker state machine
provides:
  - version variable injectable via -ldflags at build time
  - environment variable overrides for all 7 server flags
  - /healthz liveness probe endpoint (bare 200 OK)
  - /readyz readiness probe endpoint (200 when connected, 503 otherwise)
  - /status operational monitoring endpoint (JSON with status, version, uptime, broker)
affects: [28-02-docker-packaging]

# Tech tracking
tech-stack:
  added: []
  patterns: [flag.Visit for explicit-flag detection, env var override with precedence]

key-files:
  created: []
  modified: [cmd/server/main.go, web/handler.go, web/web_test.go]

key-decisions:
  - "StatusInfo struct separate from existing StatusResponse to avoid breaking /api/status frontend contract"
  - "flag.Visit used for precedence detection -- only overrides flags not explicitly set on CLI"
  - "No prefix on env var names (LISTEN_ADDR not APP_LISTEN_ADDR) per D-06"

patterns-established:
  - "Env var override pattern: flag.Visit to detect explicit flags, os.LookupEnv for env, flag.Set to apply"
  - "Health endpoint pattern: /healthz bare 200, /readyz broker-state-gated, /status rich JSON"

requirements-completed: [DOCK-04, DOCK-05]

# Metrics
duration: 2min
completed: 2026-04-19
---

# Phase 28 Plan 01: Docker Runtime Prerequisites Summary

**Version injection via ldflags, 7 env var overrides with flag>env>default precedence, and /healthz /readyz /status health endpoints**

## Performance

- **Duration:** 2 min
- **Started:** 2026-04-19T14:31:15Z
- **Completed:** 2026-04-19T14:33:33Z
- **Tasks:** 3
- **Files modified:** 3

## Accomplishments
- Added `var version = "dev"` injectable via `go build -ldflags "-X main.version=..."` for Docker build-time stamping
- Implemented `applyEnvOverrides()` mapping all 7 server flags to environment variables with correct precedence (explicit flag > env var > compiled default)
- Added three container orchestration endpoints: `/healthz` (liveness), `/readyz` (readiness gated on broker.StateConnected), `/status` (JSON operational monitoring)
- 3 new tests covering all endpoints, all 10 web tests passing

## Task Commits

Each task was committed atomically:

1. **Task 1: Add version variable and env var overrides** - `a3bc573` (feat)
2. **Task 2: Add /healthz, /readyz, /status endpoints** - `5b8e860` (feat)
3. **Task 3: Add tests for healthz, readyz, and status endpoints** - `46265e3` (test)

## Files Created/Modified
- `cmd/server/main.go` - Added version var, applyEnvOverrides() function, version passed to SetupRoutes
- `web/handler.go` - Added StatusInfo struct, /healthz /readyz /status endpoints, version parameter to SetupRoutes
- `web/web_test.go` - Added TestHealthzEndpoint, TestReadyzEndpointDormant, TestStatusInfoEndpoint; updated SetupRoutes calls with version

## Decisions Made
- Kept StatusInfo as a separate struct from existing StatusResponse -- /api/status serves the browser frontend with different fields, /status serves container orchestration
- Used stdlib test style (t.Fatalf/t.Errorf) in web_test.go per existing conventions, not testify

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Version injection, env var overrides, and health endpoints ready for Dockerfile consumption in plan 28-02
- `go build -ldflags="-X main.version=..."` verified working
- All tests pass across full project (`go test ./...`)

## Self-Check: PASSED

All 4 files found. All 3 commit hashes verified in git log.

---
*Phase: 28-docker-packaging*
*Completed: 2026-04-19*
