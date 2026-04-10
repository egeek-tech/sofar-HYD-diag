# Phase 2: WebSocket Hub, API, and Connection UI - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-10
**Phase:** 02-websocket-hub-api-and-connection-ui
**Areas discussed:** WebSocket protocol, Connection lifecycle API, Section subscription model, Connection UI design

---

## WebSocket Protocol

| Option | Description | Selected |
|--------|-------------|----------|
| gorilla/websocket | Battle-tested, widely used, clean chi integration | ✓ |
| nhooyr/websocket | Modern, context-aware, smaller footprint | |
| stdlib only | No built-in WS in Go — impractical | |

**User's choice:** gorilla/websocket
**Notes:** Adds second external dependency (after chi). Selected for maturity and community support.

| Option | Description | Selected |
|--------|-------------|----------|
| JSON envelope | Self-describing, debuggable in devtools | ✓ |
| Binary/protobuf | Smaller payloads, overkill for local tool | |
| JSON-RPC style | Structured but heavy for mostly server-push | |

**User's choice:** JSON envelope
**Notes:** None

| Option | Description | Selected |
|--------|-------------|----------|
| Hub + per-client goroutine | Classic Go pattern, central hub broadcasts | ✓ |
| Single client only | Simpler but prevents multiple tabs | |
| You decide | | |

**User's choice:** Hub + per-client goroutine
**Notes:** None

| Option | Description | Selected |
|--------|-------------|----------|
| Push to all always | Universal, everyone needs connection status | ✓ |
| Subscribe to status | Opt-in, cleaner but unnecessary friction | |

**User's choice:** Push to all always
**Notes:** None

| Option | Description | Selected |
|--------|-------------|----------|
| Full snapshot | Stateless, simple, Modbus timing is bottleneck | ✓ |
| Incremental diffs | Saves bandwidth but adds complexity | |
| You decide | | |

**User's choice:** Full snapshot
**Notes:** None

| Option | Description | Selected |
|--------|-------------|----------|
| /ws | Short, conventional | ✓ |
| /api/ws | Groups with API endpoints | |

**User's choice:** /ws
**Notes:** None

| Option | Description | Selected |
|--------|-------------|----------|
| Bidirectional WS | Single channel for commands and data | ✓ |
| WS push + REST actions | Two communication channels | |
| You decide | | |

**User's choice:** Bidirectional WS
**Notes:** None

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, server pings 30s | Detects stale browsers, gorilla built-in | ✓ |
| No heartbeat | Rely on TCP keepalive | |
| You decide | | |

**User's choice:** Yes, server pings every 30s
**Notes:** None

| Option | Description | Selected |
|--------|-------------|----------|
| Global connection event | One event, frontend marks all stale | |
| Per-section errors | Each section gets own error event | ✓ |
| Both | Global + per-section | |

**User's choice:** Per-section errors
**Notes:** Non-default choice. User wants granular error reporting per section.

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, include read timestamp | Helps user know data freshness | ✓ |
| No timestamps | Client uses receive time | |

**User's choice:** Yes, include read timestamp
**Notes:** None

---

## Connection Lifecycle API

| Option | Description | Selected |
|--------|-------------|----------|
| Reconfigure method | Add Broker.Reconfigure(), single instance lifetime | ✓ |
| Recreate broker | Stop and destroy, create new. Risk goroutine leaks | |
| You decide | | |

**User's choice:** Reconfigure method
**Notes:** None

| Option | Description | Selected |
|--------|-------------|----------|
| Pre-populate only | No Modbus traffic until user clicks Connect | ✓ |
| Auto-connect at startup | Broker connects immediately on start | |
| Configurable | Add -auto-connect flag | |

**User's choice:** Pre-populate only
**Notes:** Safer for diagnostic tool — no unwanted traffic.

| Option | Description | Selected |
|--------|-------------|----------|
| Close + cancel in-flight | Immediate cancel, clean slate | ✓ |
| Close after current read | Wait for current op to finish | |
| You decide | | |

**User's choice:** Close + cancel in-flight
**Notes:** None

| Option | Description | Selected |
|--------|-------------|----------|
| REST /api/defaults | Clean separation, browser fetches on load | ✓ |
| Embed in HTML template | Requires template engine | |
| WS initial message | WS might not be established yet | |

**User's choice:** REST endpoint GET /api/defaults
**Notes:** None

| Option | Description | Selected |
|--------|-------------|----------|
| Dormant until Connect | No connection attempt on startup | ✓ |
| Start with CLI defaults | Auto-connect with flag values | |

**User's choice:** Dormant until Connect
**Notes:** Matches pre-populate-only decision.

| Option | Description | Selected |
|--------|-------------|----------|
| Fixed by CLI flag | Deployment decision, keeps form simpler | ✓ |
| Configurable in browser | Adds complexity for rare change | |

**User's choice:** Fixed by CLI flag
**Notes:** None

---

## Section Subscription Model

| Option | Description | Selected |
|--------|-------------|----------|
| Explicit subscribe/unsubscribe | Client sends commands when navigating | ✓ |
| Navigate-based implicit | Server infers, only one section | |
| You decide | | |

**User's choice:** Explicit subscribe/unsubscribe
**Notes:** None

| Option | Description | Selected |
|--------|-------------|----------|
| One section at a time | Auto-unsubscribe previous, minimizes traffic | ✓ |
| Multiple sections | More flexible but slow with 500ms delays | |

**User's choice:** One section at a time
**Notes:** None

| Option | Description | Selected |
|--------|-------------|----------|
| Server-side timer | Server runs ticker per section | ✓ |
| Client-driven refresh | Client sends refresh on interval | |
| You decide | | |

**User's choice:** Server-side timer per subscription
**Notes:** None

| Option | Description | Selected |
|--------|-------------|----------|
| 5 seconds | Balanced | |
| 3 seconds | Fast, may overlap | |
| 10 seconds | Conservative | ✓ |

**User's choice:** 10 seconds
**Notes:** Non-default choice. User prefers conservative timing.

| Option | Description | Selected |
|--------|-------------|----------|
| Immediate first read | Data appears on navigate | ✓ |
| Wait for manual refresh | Requires user action | |

**User's choice:** Immediate first read
**Notes:** None

| Option | Description | Selected |
|--------|-------------|----------|
| String-based names | "system", "grid", etc. | ✓ |
| Enum/const IDs | Typed constants | |
| You decide | | |

**User's choice:** String-based section names
**Notes:** None

| Option | Description | Selected |
|--------|-------------|----------|
| Shared read broadcast | One read serves all subscribers | ✓ |
| Independent per-client | Duplicate reads per client | |

**User's choice:** Shared read, broadcast to subscribers
**Notes:** None

| Option | Description | Selected |
|--------|-------------|----------|
| Yes manual refresh | Client sends refresh command anytime | ✓ |
| No, toggle only | Only auto-refresh toggle | |

**User's choice:** Yes, manual refresh command
**Notes:** None

| Option | Description | Selected |
|--------|-------------|----------|
| Skip overlapping tick | Safe, prevents buildup | ✓ |
| Queue next read | Fastest data but risk storms | |
| You decide | | |

**User's choice:** Skip overlapping tick
**Notes:** None

| Option | Description | Selected |
|--------|-------------|----------|
| SN + running state + temps | Tests ASCII, enum, numeric | ✓ |
| Minimal SN only | Proves pipeline but less coverage | |
| You decide | | |

**User's choice:** SN + running state + temps
**Notes:** Exercises three different data type formatters.

| Option | Description | Selected |
|--------|-------------|----------|
| Named key-value pairs | Human-readable keys, pre-formatted | ✓ |
| Array of register objects | Matches probe model | |
| You decide | | |

**User's choice:** Named key-value pairs
**Notes:** None

| Option | Description | Selected |
|--------|-------------|----------|
| Server pre-formats | Uses existing FormatValue logic | ✓ |
| Raw values + metadata | Frontend formats | |
| Both | Formatted + raw | |

**User's choice:** Server pre-formats
**Notes:** Single source of truth for formatting.

| Option | Description | Selected |
|--------|-------------|----------|
| Pause and auto-resume | Seamless recovery | ✓ |
| Stop, user re-enables | Explicit control | |
| You decide | | |

**User's choice:** Pause and auto-resume
**Notes:** None

| Option | Description | Selected |
|--------|-------------|----------|
| internal/hub/ | Describes the role | ✓ |
| internal/ws/ | Describes the transport | |

**User's choice:** internal/hub/
**Notes:** None

---

## Connection UI Design

| Option | Description | Selected |
|--------|-------------|----------|
| Centered card | Clean, fills viewport when disconnected | |
| Top bar always visible | Compact but busier | |
| Sidebar panel | Collapsible left sidebar | ✓ |

**User's choice:** Sidebar panel
**Notes:** Non-default choice. User wants connection controls alongside section data.

| Option | Description | Selected |
|--------|-------------|----------|
| Colored dot + text | Green/red/yellow dot with label | ✓ |
| Banner/toast | Full-width colored banner | |
| Button state only | Minimal | |

**User's choice:** Colored dot + text
**Notes:** None

| Option | Description | Selected |
|--------|-------------|----------|
| Always visible | No need to hide on desktop | |
| Collapsible | Maximizes content area | ✓ |
| Auto-collapse after connect | Full when disconnected, collapsed when connected | |

**User's choice:** Collapsible
**Notes:** Non-default choice.

| Option | Description | Selected |
|--------|-------------|----------|
| Background flash on data area | Subtle, whole section flashes | ✓ |
| Border flash | Less intrusive, might be too subtle | |
| Per-value flash | Granular but noisy | |

**User's choice:** Background flash on data area
**Notes:** None

| Option | Description | Selected |
|--------|-------------|----------|
| Sidebar section nav | Below connection controls | ✓ |
| Horizontal tabs in content | Tab bar at top of content | |
| You decide | | |

**User's choice:** Sidebar section nav
**Notes:** Natural fit with sidebar layout.

| Option | Description | Selected |
|--------|-------------|----------|
| Basic validation | IP, port 1-65535, slave 1-247 | ✓ |
| No validation | Server rejects invalid | |
| You decide | | |

**User's choice:** Basic validation
**Notes:** None

| Option | Description | Selected |
|--------|-------------|----------|
| Per-section toggle | Each section has own auto-refresh | ✓ |
| Global toggle | One toggle for all | |

**User's choice:** Per-section toggle
**Notes:** Matches RT-02 requirement.

---

## Claude's Discretion

- WebSocket message type constants and exact JSON field names
- Hub internal data structures (client maps, subscription tracking)
- CSS styling details (colors, spacing, animations)
- Broker.Reconfigure() internal implementation
- Demo section probe mapping and formatting details

## Deferred Ideas

None — discussion stayed within phase scope.
