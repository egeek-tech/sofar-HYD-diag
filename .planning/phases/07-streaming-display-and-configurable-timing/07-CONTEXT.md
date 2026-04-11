# Phase 7: Streaming Display and Configurable Timing - Context

**Gathered:** 2026-04-11
**Status:** Ready for planning

<domain>
## Phase Boundary

Stream each parameter value to the UI immediately as it is read from the inverter (not after batch completes), and let users configure Modbus timing (inter-read delay and pack settle time) via the web UI.

</domain>

<decisions>
## Implementation Decisions

### Streaming Granularity
- **D-01:** Per-register streaming. Each individual register value appears in the UI the moment it is read from the inverter. The hub sends many small WebSocket messages (one per probe result) rather than one large batch message.

### Loading State
- **D-02:** Unloaded values show an em dash (—) in a dimmed/muted style. Values fill in as they arrive during a read cycle. No spinners or skeleton animations.
- **D-03:** When a register read fails mid-stream, show the last known value in a dimmed style with a small error icon. Other values continue loading normally. This keeps the display stable during intermittent errors.

### Timing Controls
- **D-04:** Two labeled number inputs in milliseconds for timing control: "Read Delay: [500] ms" and "Pack Settle: [1000] ms". Direct numeric entry, no sliders or presets.
- **D-05:** Timing controls live in the header bar, next to the existing PV channel selector and auto-refresh button. Always visible, no settings panel needed.
- **D-06:** Changed timing settings take effect on the next read cycle without requiring reconnection. The hub picks up new values from the WebSocket configure message.

### Claude's Discretion
- WebSocket message format for streaming updates (new message type vs extending section_data)
- Whether to modify broker.ReadBatch to support streaming callbacks or create a new streaming read method
- How to handle the transition from "loading" to "loaded" state in the frontend DOM
- Whether timing values should persist in localStorage between sessions

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Current Implementation
- `internal/broker/broker.go` lines 107, 185, 331, 403 — Inter-read delay enforcement, ReadBatch, executeRead
- `internal/hub/hub.go` lines 400-525 — Section read cycle (triggerSectionRead), ReadBatch usage, sectionResult channel
- `internal/hub/hub.go` lines 603-690 — triggerBMSRead with batch results
- `internal/hub/message.go` — OutboundMessage, sectionResult, GroupData structures
- `web/static/app.js` — Section data rendering, WebSocket message handling

### Prior Phase Context
- `.planning/phases/06-battery-pack-access-fix/06-CONTEXT.md` — Topology constants, bitmap interpretation

</canonical_refs>

<code_context>
## Existing Code Insights

### Key Architecture Constraints
- `broker.ReadBatch()` reads all registers sequentially with 500ms inter-read delay, returns all results at once
- Hub sends results via `sectionResult` channel → event loop → client broadcast
- Frontend receives one `section_data` message per section with all groups/values
- Broker already has `SetInterReadDelay()` for configuring the delay

### Integration Points
- `broker.ReadBatch()` needs to be modified or supplemented with a streaming variant
- `sectionResult` channel carries complete results — needs per-register streaming path
- Frontend `handleSectionData()` replaces entire section content — needs incremental update support
- `configure` WebSocket message already exists for PV channels — extend for timing settings

</code_context>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 07-streaming-display-and-configurable-timing*
*Context gathered: 2026-04-11*
