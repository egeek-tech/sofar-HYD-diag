---
phase: 02-websocket-hub-api-and-connection-ui
plan: 03
subsystem: api, ui
tags: [websocket, gorilla, chi, vanilla-js, css-custom-properties, modbus, real-time]

# Dependency graph
requires:
  - phase: 02-websocket-hub-api-and-connection-ui (plan 01)
    provides: Broker dormant start, Reconfigure/Disconnect, BrokerInterface, gorilla/websocket dep
  - phase: 02-websocket-hub-api-and-connection-ui (plan 02)
    provides: Hub event loop, Client read/write pumps, Section subscriptions, auto-refresh timers
provides:
  - GET /api/defaults endpoint returning CLI flag defaults as JSON
  - GET /ws WebSocket upgrade endpoint registering clients with hub
  - Server wiring of hub to broker in cmd/server/main.go
  - Complete sidebar layout with connection status, form, section navigation
  - WSClient class with exponential backoff reconnect and jitter
  - Connection form with IP/port/slaveID validation and localStorage persistence
  - Section navigation with lazy subscribe and data card rendering
  - Flash feedback (green success, red error) on content area
  - Auto-refresh toggle pill button
  - Full CSS design system with custom properties
affects: [phase-03, phase-04, phase-05]

# Tech tracking
tech-stack:
  added: [gorilla/websocket (now direct dep)]
  patterns: [SetupRoutes accepts hub+defaults+logger, WebSocket upgrade with client lifecycle, safe DOM rendering via textContent/createElement]

key-files:
  created: []
  modified:
    - web/handler.go
    - web/web_test.go
    - cmd/server/main.go
    - web/static/index.html
    - web/static/style.css
    - web/static/app.js
    - go.mod

key-decisions:
  - "CheckOrigin returns true for WebSocket upgrader -- local network diagnostic tool (T-02-10)"
  - "All dynamic data rendering uses textContent/createElement, zero innerHTML -- XSS prevention (T-02-11)"
  - "Sidebar toggle uses Unicode characters via textContent instead of innerHTML for chevron icons"

patterns-established:
  - "SetupRoutes(r, broker, hub, defaults, startTime, logger) -- extended signature pattern for web package"
  - "DefaultsConfig struct -- JSON-tagged struct for CLI-to-browser default propagation"
  - "Safe DOM rendering -- all WebSocket data rendered via createElement + textContent, never innerHTML"
  - "CSS custom properties on :root -- all design tokens centralized for consistency"
  - "WSClient class -- singleton WebSocket manager with exponential backoff reconnect"

requirements-completed: [CONN-01, CONN-02, CONN-03, RT-01, RT-02, RT-03, RT-04, RT-05]

# Metrics
duration: 5min
completed: 2026-04-10
---

# Phase 2 Plan 3: API Endpoints, Server Wiring, and Frontend UI Summary

**WebSocket /ws and /api/defaults endpoints wired to hub, complete sidebar connection UI with form validation, localStorage persistence, section navigation, and real-time data card rendering via safe DOM API**

## Performance

- **Duration:** 5 min
- **Started:** 2026-04-10T12:48:54Z
- **Completed:** 2026-04-10T12:54:30Z
- **Tasks:** 2 completed, 1 checkpoint (auto-approved)
- **Files modified:** 7

## Accomplishments
- /api/defaults returns CLI flag defaults as JSON for browser form pre-population (D-14)
- /ws upgrades to WebSocket, creates Client, registers with Hub, launches ReadPump/WritePump goroutines (D-06)
- Hub created and wired to broker in cmd/server/main.go with Run() goroutine (D-03, D-29)
- Complete sidebar layout: connection status dot with pulse animation, connection form with validation, section navigation with 7 sections (1 enabled, 6 disabled)
- WSClient class with exponential backoff (1s-30s) and 30% random jitter for reconnection (T-02-14)
- Connection form validates IP/hostname, port 1-65535, slave 1-247 with per-field error messages (D-34)
- localStorage persistence: save on connect, restore on page load, fallback chain localStorage > /api/defaults > hardcoded (CONN-03)
- Flash feedback: green on section_data, red on section_error with CSS transition fade-out (RT-03, RT-04)
- Auto-refresh toggle pill button sends auto_refresh message to server (RT-02, D-35)
- Zero innerHTML usage in app.js -- all data rendering via textContent/createElement (T-02-11)

## Task Commits

Each task was committed atomically:

1. **Task 1: API endpoints, server wiring, integration tests** - `c2b650e` (test: RED), `2a884f1` (feat: GREEN)
2. **Task 2: Complete frontend rewrite** - `1dc8633` (feat)
3. **Task 3: Visual verification** - auto-approved (checkpoint:human-verify)

## Files Created/Modified
- `web/handler.go` - DefaultsConfig struct, /api/defaults endpoint, /ws WebSocket upgrade endpoint, updated SetupRoutes signature
- `web/web_test.go` - TestDefaultsEndpoint, TestWSUpgrade, TestWSUpgradeWithoutHeaders, updated newTestRouter for new signature
- `cmd/server/main.go` - Hub creation, Run() goroutine, DefaultsConfig wiring, updated SetupRoutes call
- `web/static/index.html` - Complete sidebar layout with connection form, status dot, section nav, content area
- `web/static/style.css` - Full CSS design system with custom properties, sidebar, flash, status dot animations, data card
- `web/static/app.js` - WSClient class, connection form handler, validation, section navigation, flash feedback, localStorage, auto-refresh toggle
- `go.mod` - gorilla/websocket promoted from indirect to direct dependency

## Decisions Made
- CheckOrigin returns true for WebSocket upgrader -- acceptable for local network diagnostic tool with no public internet exposure (T-02-10)
- All dynamic data rendering uses textContent/createElement, zero innerHTML -- prevents XSS from register values flowing through WebSocket (T-02-11)
- Sidebar toggle uses Unicode characters (\u00AB / \u00BB) via textContent instead of HTML entities via innerHTML for security consistency

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Phase 2 complete: broker, hub, API, and connection UI all wired end-to-end
- Ready for Phase 3: section-specific register reading and display
- Pattern established for adding new sections: register probes in hub, add nav button, section data renders automatically via data card
- All 37 tests pass across all packages with race detector

## Self-Check: PASSED

All 6 source files exist. All 3 task commits verified. SUMMARY exists.

---
*Phase: 02-websocket-hub-api-and-connection-ui*
*Completed: 2026-04-10*
