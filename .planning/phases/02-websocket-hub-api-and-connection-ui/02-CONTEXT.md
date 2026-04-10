# Phase 2: WebSocket Hub, API, and Connection UI - Context

**Gathered:** 2026-04-10
**Status:** Ready for planning

<domain>
## Phase Boundary

Build the real-time communication backbone and connection management frontend. Users can configure inverter connection parameters (IP, port, slave ID) in the browser, connect/disconnect, and see connection status. The WebSocket hub infrastructure supports section-based subscriptions with auto-refresh, delivering register data as push messages. A demo "status" section validates the full pipeline end-to-end. All subsequent phases (3-5) build their sections on top of this infrastructure.

</domain>

<decisions>
## Implementation Decisions

### WebSocket Protocol
- **D-01:** Use `gorilla/websocket` library — battle-tested, widely used, clean chi integration
- **D-02:** JSON envelope message format: `{"type":"...","section":"...","data":{...},"timestamp":"..."}`
- **D-03:** Hub + per-client goroutine pattern — central hub tracks connected clients, each gets read/write goroutines, hub broadcasts section data to subscribed clients
- **D-04:** Connection state changes pushed to ALL connected clients automatically (not subscription-based)
- **D-05:** Full section snapshot on each refresh (no incremental diffs) — stateless, simple, Modbus timing is the bottleneck not payload size
- **D-06:** WebSocket endpoint at `/ws`
- **D-07:** Bidirectional WebSocket — client sends commands (subscribe, unsubscribe, connect, disconnect, refresh) via WS too. Single communication channel
- **D-08:** Server sends WS ping frames every 30s to detect stale browser connections (gorilla built-in ping/pong)
- **D-09:** Per-section error events when reads fail — each section gets its own error event, not just a global connection-lost event
- **D-10:** Each section_data message includes server-side timestamp of when the Modbus read completed

### Connection Lifecycle API
- **D-11:** Add `Broker.Reconfigure(addr, slaveID)` method that closes current connection, updates target, and reconnects. Single broker instance for app lifetime
- **D-12:** CLI flags (`-inverter-host`, `-inverter-port`, `-slave`) pre-populate browser form only — no Modbus traffic until user clicks Connect
- **D-13:** Disconnect immediately cancels pending/in-flight reads and closes TCP connection. Subscribed sections receive error events
- **D-14:** `GET /api/defaults` endpoint returns CLI default values as JSON `{"host":"...","port":...,"slaveId":...}` for browser form pre-population
- **D-15:** Broker starts in dormant state — no connection attempt on startup. Waits for explicit Connect from browser
- **D-16:** Modbus mode (TCP/RTU) fixed by CLI flag — not configurable at runtime. Deployment decision

### Section Subscription Model
- **D-17:** Explicit subscribe/unsubscribe commands via WS: `{"type":"subscribe","section":"system"}`, `{"type":"unsubscribe","section":"system"}`
- **D-18:** One section at a time per client — subscribing to a new section auto-unsubscribes the previous one
- **D-19:** Server-side timer per section (not per client) — 10 second auto-refresh interval
- **D-20:** Subscribe triggers immediate first read and push — user sees data as soon as they navigate to a section
- **D-21:** String-based section names: "status", "system", "grid", "eps", "pv", "battery", "stats" — Phase 2 defines mechanism, later phases register sections
- **D-22:** Shared read, broadcast to all subscribers — if two tabs subscribe to same section, one Modbus read cycle serves both
- **D-23:** Client can send `{"type":"refresh","section":"..."}` for one-shot manual refresh anytime, regardless of auto-refresh state
- **D-24:** Skip overlapping tick — if previous read is still in progress when timer fires, skip that tick
- **D-25:** Demo "status" section in Phase 2 reading Inverter SN (0x0445, ASCII), Running State (0x0404, enum), Internal Temp (0x0418, scaled) — validates full pipeline
- **D-26:** Section data as named key-value pairs: `{"type":"section_data","section":"status","data":{"inverter_sn":"HYD...","running_state":"Grid-connected","internal_temp":"42.3 °C"},"timestamp":"..."}`
- **D-27:** Server pre-formats all values using existing `internal/register.FormatValue` logic — frontend just displays strings
- **D-28:** Refresh timers pause on connection drop, auto-resume when broker reconnects
- **D-29:** Hub package at `internal/hub/` — manages clients, subscriptions, section timers, broadcasting

### Connection UI Design
- **D-30:** Collapsible sidebar layout — connection controls and section navigation in left sidebar, main content area for section data
- **D-31:** Colored dot + text for connection status: green=connected, red=disconnected, yellow=connecting/reconnecting. Dot pulses while connecting
- **D-32:** Section navigation links in sidebar below connection controls — clicking a section loads it in main content area. Collapsed sidebar shows section icons
- **D-33:** Background flash on section content area: light-green on successful refresh, light-red on failure. Fades back over ~1s
- **D-34:** Basic input validation before Connect: IP/hostname format, port 1-65535, slave ID 1-247. HTML5 pattern + JS check
- **D-35:** Per-section auto-refresh toggle button in main content area (matches RT-02 requirement)

### Claude's Discretion
- WebSocket message type constants and exact JSON field names
- Hub internal data structures (client maps, subscription tracking)
- CSS styling details (colors, spacing, animations)
- Broker.Reconfigure() internal implementation (context cancellation, goroutine coordination)
- Demo section probe mapping and formatting details

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Protocol Specification
- `Sofar_Inverter_MODBUS_V1.38_EN.pdf` — Register map for demo section registers (0x0445 SN, 0x0404 running state, 0x0418 temp). Data types and scaling factors

### Existing Implementation
- `main.go.bak` — Original CLI tool, reference for register reading patterns and value formatting
- `internal/broker/broker.go` — Current broker with Run(), ReadRegisters(), WriteRegister(), ReadBatch(), StateEvents(). Needs Reconfigure() and dormant-start modifications
- `internal/broker/state.go` — State enum (Disconnected, Connecting, Connected, Reconnecting) and StateEvent type
- `internal/register/format.go` — FormatValue logic for server-side value formatting
- `internal/register/system.go` — System register probe definitions (demo section will use these)
- `web/handler.go` — Current chi router setup, /api/status endpoint, embedded static file serving
- `cmd/server/main.go` — Server entry point with CLI flags, broker wiring, graceful shutdown

### Phase 1 Context
- `.planning/phases/01-foundation-and-modbus-service/01-CONTEXT.md` — Package layout decisions (D-01 through D-29), broker API design, chi router choice

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `broker.Broker` — Fully functional command-channel broker. Needs Reconfigure() method and dormant-start support, but core serialization and retry logic is ready
- `broker.StateEvents()` channel — Already emits state change events, ready to bridge to WebSocket hub
- `register.FormatValue()` — Existing value formatting logic for ASCII, signed, scaled, enum values. Will be used for server-side formatting
- `register.SystemProbes` (or equivalent) — System register probe definitions in `internal/register/system.go`. Demo section can reference these
- `web.SetupRoutes()` — Chi router setup with embedded static serving. Extend with /ws endpoint and /api/defaults
- `/api/status` endpoint — Already returns broker state as JSON. Pattern to follow for /api/defaults

### Established Patterns
- Command-channel serialization in broker — all Modbus ops go through single goroutine
- `slog` structured logging with component tags (`"component", "broker"`)
- Chi middleware chain (Recoverer, RealIP, debug logging)
- Go embed for static files in `web/` package

### Integration Points
- `internal/hub/` (new) integrates with `internal/broker/` for register reads and state events
- `web/handler.go` registers `/ws` upgrade endpoint and `/api/defaults` endpoint on chi router
- `cmd/server/main.go` wires hub to broker and passes to web.SetupRoutes()
- `web/static/` gets new JavaScript for WS client, connection form, section rendering

</code_context>

<specifics>
## Specific Ideas

- Collapsible sidebar chosen over centered card — user wants connection controls always accessible alongside section data
- 10 second refresh interval (not 5s) — user preference for conservative timing, aligns with Modbus read cycle duration for larger sections
- Per-section errors (not global) — user wants granular error reporting per section, even though root cause is often the same connection issue
- Demo "status" section reads SN + running state + temps to exercise ASCII, enum, and numeric data types in one demo

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 02-websocket-hub-api-and-connection-ui*
*Context gathered: 2026-04-10*
