# Phase 7: Streaming Display and Configurable Timing - Research

**Researched:** 2026-04-11
**Domain:** WebSocket streaming, DOM incremental updates, Go callback patterns
**Confidence:** HIGH

## Summary

Phase 7 transforms the current batch-then-render data flow into a per-register streaming pipeline where each Modbus register value appears in the UI the instant it is read. The current architecture reads all registers for a section via `broker.ReadBatch()`, collects all results, builds a complete `section_data` message, and broadcasts it. The frontend then replaces the entire content area. Streaming requires changes at three layers: (1) the broker/hub must emit per-register results as they are read, (2) a new WebSocket message type carries individual register updates, and (3) the frontend must update DOM elements in-place rather than replacing the entire section.

The secondary goal -- configurable timing -- is straightforward: the broker already has `SetInterReadDelay()` and the pack settle time is a hardcoded `time.Sleep(1 * time.Second)` in `triggerPackRead()`. Both need to become runtime-configurable via the existing `configure` WebSocket message pattern.

**Primary recommendation:** Add a streaming callback to the hub's read goroutines (not the broker itself) so that each register result is immediately sent as a lightweight `register_value` WebSocket message. The frontend pre-renders all register slots with em-dash placeholders on subscribe, then fills them in via targeted DOM updates keyed by register name. Timing controls reuse the existing `configure` message pattern with a new `timing` config payload.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Per-register streaming. Each individual register value appears in the UI the moment it is read from the inverter. The hub sends many small WebSocket messages (one per probe result) rather than one large batch message.
- **D-02:** Unloaded values show an em dash (---) in a dimmed/muted style. Values fill in as they arrive during a read cycle. No spinners or skeleton animations.
- **D-03:** When a register read fails mid-stream, show the last known value in a dimmed style with a small error icon. Other values continue loading normally. This keeps the display stable during intermittent errors.
- **D-04:** Two labeled number inputs in milliseconds for timing control: "Read Delay: [500] ms" and "Pack Settle: [1000] ms". Direct numeric entry, no sliders or presets.
- **D-05:** Timing controls live in the header bar, next to the existing PV channel selector and auto-refresh button. Always visible, no settings panel needed.
- **D-06:** Changed timing settings take effect on the next read cycle without requiring reconnection. The hub picks up new values from the WebSocket configure message.

### Claude's Discretion
- WebSocket message format for streaming updates (new message type vs extending section_data)
- Whether to modify broker.ReadBatch to support streaming callbacks or create a new streaming read method
- How to handle the transition from "loading" to "loaded" state in the frontend DOM
- Whether timing values should persist in localStorage between sessions

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| STREAM-01 | Each parameter appears in the UI immediately as it is read, not after the entire batch completes | Streaming callback in hub read goroutines + new `register_value` message type + DOM in-place updates |
| STREAM-02 | Loading state shows partial data with remaining parameters still loading | Pre-render all register slots with em-dash placeholders; fill in as values arrive |
| TIMING-01 | User can adjust the default Modbus read delay via UI input (default 500ms) | Extend `configure` message with timing payload; broker already has `SetInterReadDelay()` but needs runtime update path |
| TIMING-02 | Battery pack reads use a separate, longer settle time after 0x9020 write (configurable, default 1-2s) | Extract hardcoded `time.Sleep(1 * time.Second)` in `triggerPackRead()` to a hub-level field |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib | 1.26.x | All backend logic | Project constraint: no external deps [VERIFIED: go.mod] |
| Vanilla JS | ES5+ | All frontend logic | Project constraint: no frameworks [VERIFIED: CLAUDE.md] |

### Supporting
No additional libraries needed. All functionality is achievable with existing stdlib and vanilla JS.

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Per-register WS messages | Server-Sent Events (SSE) | SSE is unidirectional; project already uses bidirectional WebSocket -- adding SSE would mean two transport layers |
| DOM in-place updates | Virtual DOM / morphdom | Project constraint prohibits JS frameworks; manual targeted DOM updates are simple enough for this use case |

## Architecture Patterns

### Current Data Flow (What Must Change)

```
Current (batch):
  hub goroutine -> broker.ReadBatch(all probes) -> wait for ALL -> build GroupData -> sectionResult channel -> broadcast section_data

Target (streaming):
  hub goroutine -> for each probe: broker.ReadRegisters(1 probe) -> send register_value immediately -> after all: send section_complete
```

### Pattern 1: Streaming Read in Hub Goroutine

**What:** Instead of calling `broker.ReadBatch()` (which blocks until all reads complete), the hub's read goroutine calls `broker.ReadRegisters()` for each probe individually, sending a `register_value` message after each successful read. After all probes are read, it sends a `section_complete` message. [VERIFIED: broker.go executeRead and ReadRegisters methods exist]

**When to use:** All standard section reads (system, grid, eps, pv, battery, stats).

**Why not modify the broker:** The broker's `ReadBatch` uses the command channel pattern for serialization. Adding streaming callbacks inside the broker would complicate the clean command/response model. Instead, the hub goroutine simply loops over individual reads, which still serialize through the broker's command channel. The inter-read delay is already enforced per-read in `executeRead()`. [VERIFIED: broker.go lines 331-368]

**Key insight:** `broker.ReadRegisters()` already exists and handles retry + inter-read delay. `ReadBatch` is just a loop over `executeRead` internally. The hub goroutine can replicate this loop while sending streaming updates between iterations.

```go
// Source: hub.go triggerStandardRead — proposed streaming replacement
go func() {
    defer sec.reading.Store(false)

    for i, p := range probes {
        if h.ctx.Err() != nil {
            break
        }
        result, err := h.broker.ReadRegisters(h.ctx, p.Addr, p.Count)
        // Send per-register update immediately
        h.results <- sectionResult{
            section: sectionName,
            msg: NewRegisterValue(sectionName, groupIdx, p.Name, result, err),
        }
    }
    // Signal read cycle complete
    h.results <- sectionResult{
        section: sectionName,
        msg: NewSectionComplete(sectionName),
    }
}()
```

### Pattern 2: New WebSocket Message Types

**What:** Two new outbound message types to replace the monolithic `section_data` for streaming.

**Message type: `register_value`**
```json
{
    "type": "register_value",
    "section": "system",
    "group": "System Info",
    "name": "Inverter SN",
    "value": "SA3200...",
    "error": ""
}
```

**Message type: `section_complete`**
```json
{
    "type": "section_complete",
    "section": "system",
    "timestamp": "2026-04-11T12:00:00Z"
}
```

**Why new types instead of extending `section_data`:** The frontend currently replaces the entire content body when it receives `section_data`. A new type avoids breaking the existing handler. The `section_data` type can be kept for backward compatibility or removed later. [VERIFIED: app.js handleSectionData lines 448-472 calls body.textContent = '']

### Pattern 3: Pre-rendered DOM Skeleton

**What:** When a section is subscribed to, the frontend pre-renders all group cards with all register name rows, each showing an em-dash in dimmed style. As `register_value` messages arrive, the corresponding DOM element is updated in place.

**Key implementation detail:** Each value `<span>` needs a stable identifier so that incoming messages can target it. Use `data-register="GroupName::RegisterName"` attributes on the value elements.

```javascript
// On subscribe: pre-render skeleton with em-dash placeholders
function renderSectionSkeleton(sectionName, groups) {
    var body = $('#content-body');
    body.textContent = '';
    for (var i = 0; i < groups.length; i++) {
        var card = renderGroupCardSkeleton(groups[i]);
        body.appendChild(card);
    }
}

// On register_value message: update single value in place
function handleRegisterValue(msg) {
    if (msg.section !== App.activeSection) return;
    var key = msg.group + '::' + msg.name;
    var el = document.querySelector('[data-register="' + CSS.escape(key) + '"]');
    if (!el) return;
    el.textContent = msg.value || '\u2014'; // em-dash fallback
    el.classList.remove('value--loading');
    if (msg.error) {
        el.classList.add('value--error');
    }
}
```

### Pattern 4: Timing Configuration via Configure Message

**What:** Extend the existing `configure` WebSocket message pattern (currently used for PV channels) to carry timing settings.

**Inbound message:**
```json
{
    "type": "configure",
    "section": "timing",
    "config": {
        "read_delay_ms": 500,
        "pack_settle_ms": 1000
    }
}
```

**Hub handler:** The hub stores `readDelayMs` and `packSettleMs` as fields. On receiving this configure message, update the broker's inter-read delay and the hub's pack settle time. These take effect on the next read cycle. [VERIFIED: broker.SetInterReadDelay exists at broker.go line 107; pack settle is hardcoded at hub.go line 1055]

**Runtime broker delay update:** The broker's `interReadDelay` field is only read inside the `Run()` goroutine (in `enforceInterReadDelay`). Since the hub sends commands through the broker's command channel, we need either: (a) a new command type `CmdSetDelay`, or (b) send the delay update through the broker's command channel as a reconfigure-like operation. Option (a) is cleaner -- add a `SetInterReadDelayRuntime()` method that sends a command. [VERIFIED: broker.go interReadDelay field at line 84, enforceInterReadDelay at line 458]

### Pattern 5: Section Metadata for Skeleton Pre-rendering

**What:** The frontend needs to know the group/register structure of a section BEFORE data arrives, so it can render the skeleton. Two approaches:

**Option A (recommended):** Send a `section_schema` message when the client subscribes, containing the group names and register names. This is a one-time message per subscription.

**Option B:** Hardcode the section schemas in the frontend JavaScript. This is fragile -- if the backend adds/removes registers, the frontend gets out of sync.

```json
{
    "type": "section_schema",
    "section": "system",
    "groups": [
        {
            "name": "System Info",
            "layout": "column",
            "registers": ["Inverter SN", "DSP Version", "..."]
        }
    ]
}
```

The hub already has `sec.Groups` and `sec.Probes` -- generating this schema message is trivial.

### Anti-Patterns to Avoid

- **Don't stream at the broker level:** The broker's clean command/response model should not be complicated with streaming callbacks. Keep streaming logic in the hub goroutine.
- **Don't use innerHTML for updates:** Always use `textContent` for values to prevent XSS. The project already follows this pattern. [VERIFIED: app.js uses textContent throughout]
- **Don't break the existing `section_data` path immediately:** BMS and battery sections have custom read logic. Convert standard sections to streaming first, then adapt BMS/battery in subsequent updates within this phase.
- **Don't use Web Workers or complex state management:** The streaming updates are small and infrequent (one every 500ms+). Direct DOM manipulation is sufficient.

### Recommended Project Structure Changes

```
internal/
  broker/
    broker.go            # Add CmdSetDelay command type + SetDelayRuntime() method
  hub/
    hub.go               # Add readDelayMs/packSettleMs fields; streaming read goroutines
    message.go           # Add MsgTypeRegisterValue, MsgTypeSectionComplete, MsgTypeSectionSchema
    hub_streaming.go     # (NEW) Streaming read methods extracted for clarity
web/static/
    app.js               # Add skeleton rendering, register_value handler, timing controls
    index.html           # Add timing input controls in content__header-controls
    style.css            # Add .value--loading, .value--error styles
```

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| CSS escape for data attributes | Custom escape function | `CSS.escape()` built-in | Handles all edge cases for register names with special chars [ASSUMED] |
| Number input validation | Custom parsing | HTML `<input type="number" min="100" max="5000">` | Browser handles validation, step constraints |
| WebSocket message routing | Custom router | Existing `WSClient.on()` handler pattern | Already established in app.js [VERIFIED: app.js lines 92-94] |

## Common Pitfalls

### Pitfall 1: Race Between Skeleton Render and First Register Value
**What goes wrong:** The schema message and first register_value message arrive before the frontend has rendered the skeleton, causing values to be dropped.
**Why it happens:** WebSocket messages are asynchronous; the hub goroutine starts reading immediately after sending the schema.
**How to avoid:** The hub should send the `section_schema` message synchronously through the results channel BEFORE starting the streaming read goroutine. The frontend renders the skeleton in the `section_schema` handler, which runs before any `register_value` handlers.
**Warning signs:** First few register values missing on initial load.

### Pitfall 2: Stale DOM Elements After Section Switch
**What goes wrong:** User switches sections while a streaming read is in progress. Register values from the old section arrive and try to update DOM elements that no longer exist.
**Why it happens:** The read goroutine runs asynchronously and doesn't know about section switches.
**How to avoid:** The `handleRegisterValue` handler already checks `msg.section !== App.activeSection` and returns early. This is sufficient. Also, `querySelector` will return null for elements that have been removed.
**Warning signs:** Console errors about null elements.

### Pitfall 3: Broker InterReadDelay Not Thread-Safe
**What goes wrong:** Setting the broker's `interReadDelay` from the hub goroutine while the broker's `Run()` goroutine reads it causes a data race.
**Why it happens:** `interReadDelay` is a plain `time.Duration` field, not protected by a mutex or atomic.
**How to avoid:** Use the broker's command channel pattern -- add a `CmdSetDelay` command that updates the field inside the `Run()` goroutine, where it is the only reader/writer. [VERIFIED: broker.go Run() at line 122 is the only goroutine that reads interReadDelay]
**Warning signs:** Race detector failures in tests.

### Pitfall 4: Too Many Small WebSocket Messages
**What goes wrong:** System section has ~20 probes, each producing a separate WebSocket message. If the browser processes these slowly, the send buffer fills up.
**Why it happens:** The client send channel is buffered but finite (gorilla/websocket chat pattern).
**How to avoid:** The 500ms inter-read delay means messages arrive slowly enough (~2/second). The client send channel has adequate buffer. Also, `broadcastToSection` already handles slow clients by closing them. [VERIFIED: hub.go broadcastToSection at line 888 has slow-client handling]
**Warning signs:** Clients being disconnected during streaming.

### Pitfall 5: Pack Settle Time Overridden During Active Pack Read
**What goes wrong:** User changes pack settle time while a pack read is in progress. The currently sleeping goroutine uses the old value.
**Why it happens:** `time.Sleep(settleTime)` captures the value at call time.
**How to avoid:** Per D-06, changes take effect on the NEXT read cycle. The current goroutine uses whatever value was set when it started. This is correct behavior -- no mitigation needed.
**Warning signs:** None -- this is expected.

### Pitfall 6: BMS/Battery Custom Read Paths Need Streaming Too
**What goes wrong:** Standard sections stream but BMS and battery sections still batch, creating an inconsistent UX.
**Why it happens:** `triggerBMSRead()` and `triggerBatteryRead()` have complex post-processing (topology detection, fault decoding, auto-channel detection) that depend on having all results available.
**How to avoid:** Stream individual register values as they arrive (for immediate UI feedback), BUT also collect all results to run the post-processing logic. Send computed groups (bitmap, faults, protection) as final `register_value` messages after all reads complete. This is a "stream the reads, batch the computation" pattern.
**Warning signs:** BMS section appears blank until all reads finish.

## Code Examples

### Example 1: New Message Types (message.go)

```go
// Source: Extending current message.go pattern [VERIFIED: message.go]
const (
    MsgTypeRegisterValue   = "register_value"
    MsgTypeSectionComplete = "section_complete"
    MsgTypeSectionSchema   = "section_schema"
    MsgTypeTimingConfig    = "timing_config" // optional: echo current timing to client
)

// RegisterValueMessage carries a single register's value.
type RegisterValueMessage struct {
    Type    string `json:"type"`
    Section string `json:"section"`
    Group   string `json:"group"`
    Name    string `json:"name"`
    Value   string `json:"value,omitempty"`
    Error   string `json:"error,omitempty"`
}

// SectionSchemaMessage describes the structure of a section for skeleton rendering.
type SectionSchemaMessage struct {
    Type    string              `json:"type"`
    Section string              `json:"section"`
    Groups  []SchemaGroup       `json:"groups"`
}

type SchemaGroup struct {
    Name      string   `json:"name"`
    Layout    string   `json:"layout,omitempty"`
    Type      string   `json:"type,omitempty"`
    Registers []string `json:"registers"`
}
```

### Example 2: Broker Runtime Delay Update (broker.go)

```go
// Source: Extending current broker command pattern [VERIFIED: broker.go]
const CmdSetDelay CmdType = 5 // add to CmdType iota block

type SetDelayRequest struct {
    InterReadDelay time.Duration
}

func (b *Broker) SetDelayRuntime(ctx context.Context, d time.Duration) error {
    respCh := make(chan interface{}, 1)
    select {
    case b.commands <- command{
        cmdType:  CmdSetDelay,
        request:  SetDelayRequest{InterReadDelay: d},
        response: respCh,
    }:
    case <-b.done:
        return ErrBrokerClosed
    case <-ctx.Done():
        return ctx.Err()
    }
    select {
    case <-respCh:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}

// In execute():
case CmdSetDelay:
    req := cmd.request.(SetDelayRequest)
    b.interReadDelay = req.InterReadDelay
    b.logger.Info("inter-read delay updated", "delay", req.InterReadDelay)
    cmd.response <- Result{}
```

### Example 3: Timing Controls HTML (index.html)

```html
<!-- Source: Extending existing content__header-controls pattern [VERIFIED: index.html line 95-98] -->
<div class="content__header-controls">
    <label class="timing-label">Read Delay:
        <input type="number" id="input-read-delay" class="timing-input" value="500" min="100" max="5000" step="100">
        <span class="timing-unit">ms</span>
    </label>
    <label class="timing-label">Pack Settle:
        <input type="number" id="input-pack-settle" class="timing-input" value="1000" min="500" max="5000" step="100">
        <span class="timing-unit">ms</span>
    </label>
    <select id="pv-channel-select" class="pv-channel-select" style="display:none;"></select>
    <button id="btn-auto-refresh" class="btn-auto-refresh" style="display:none;">Auto</button>
</div>
```

### Example 4: Frontend Streaming Handler (app.js)

```javascript
// Source: Extending existing WSClient.on() pattern [VERIFIED: app.js]

// Register new handlers in DOMContentLoaded:
App.ws.on('section_schema', handleSectionSchema);
App.ws.on('register_value', handleRegisterValue);
App.ws.on('section_complete', handleSectionComplete);

function handleSectionSchema(msg) {
    if (msg.section !== App.activeSection) return;
    renderSectionSkeleton(msg);
}

function handleRegisterValue(msg) {
    if (msg.section !== App.activeSection) return;
    var key = msg.group + '::' + msg.name;
    var el = document.querySelector('[data-register="' + key.replace(/"/g, '\\"') + '"]');
    if (!el) return;
    if (msg.error) {
        // D-03: show last known value dimmed with error icon
        el.classList.add('value--error');
        el.classList.remove('value--loading');
    } else {
        el.textContent = msg.value;
        el.classList.remove('value--loading', 'value--error');
    }
}

function handleSectionComplete(msg) {
    if (msg.section !== App.activeSection) return;
    // Update timestamp
    if (msg.timestamp) {
        var ts = $('#content-timestamp');
        var d = new Date(msg.timestamp);
        ts.textContent = 'Last updated: ' + d.toLocaleTimeString();
        ts.style.display = '';
    }
    triggerFlash('success');
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Batch-then-render | Per-register streaming | This phase | Values appear ~20x faster (500ms per vs 10s+ for full batch) |
| Hardcoded timing | User-configurable delays | This phase | Adapts to different inverter models/network conditions |

**No deprecations:** This phase extends existing patterns, no breaking changes to existing message types needed. The `section_data` message type should be preserved for pack data and any remaining batch paths.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `CSS.escape()` is available in all target browsers | Don't Hand-Roll | Low -- fallback to manual escaping; tool is desktop Chrome/Firefox only |
| A2 | 100ms minimum for read delay is a safe lower bound | Code Examples | Medium -- if set too low, inverter may reject reads. Clamping at 200ms may be safer |
| A3 | BMS/battery sections can stream individual register values while still collecting for post-processing | Pitfall 6 | Low -- the pattern is straightforward: stream reads, batch computation |

## Open Questions (RESOLVED)

1. RESOLVED: **Should timing values persist in localStorage?**
   - What we know: PV channels already persist via `localStorage` (PV_STORAGE_KEY pattern) [VERIFIED: app.js lines 737-748]
   - What's unclear: Whether users want timing to persist or reset to defaults each session
   - Recommendation: Persist in localStorage following the PV channels pattern. Sane defaults (500ms read, 1000ms settle) are safe fallbacks.

2. RESOLVED: **Should the section_schema be sent on every read cycle or only on subscribe?**
   - What we know: The schema only changes when PV channels are reconfigured (rare)
   - What's unclear: Whether re-sending schema on each cycle simplifies state management
   - Recommendation: Send schema only on subscribe (and on configure). The skeleton stays rendered between read cycles; only values update.

3. RESOLVED: **Should BMS bitmap/protection groups stream or remain batched?**
   - What we know: Bitmap depends on reading 0x9022 first; protection has 6 registers with special decoding
   - What's unclear: Whether streaming individual BMS info registers before bitmap is useful
   - Recommendation: Stream individual BMS info values as they arrive. Bitmap and protection groups are sent as final computed messages after all reads complete. The schema includes them as placeholders.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (stdlib) |
| Config file | None -- standard `go test` |
| Quick run command | `go test ./internal/hub/ -run TestStream -count=1 -v` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| STREAM-01 | Per-register values sent as individual WS messages | unit | `go test ./internal/hub/ -run TestStreamingRead -count=1 -v` | Wave 0 |
| STREAM-02 | Schema message sent on subscribe; skeleton rendering | unit | `go test ./internal/hub/ -run TestSectionSchema -count=1 -v` | Wave 0 |
| TIMING-01 | Configure message updates broker inter-read delay | unit | `go test ./internal/hub/ -run TestTimingConfigure -count=1 -v` | Wave 0 |
| TIMING-02 | Pack settle time configurable via configure message | unit | `go test ./internal/hub/ -run TestPackSettleConfigure -count=1 -v` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/hub/ -count=1 -v`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] Streaming read tests (STREAM-01): `TestStreamingRead` -- verify mockBroker receives individual ReadRegisters calls and client receives per-register messages
- [ ] Schema message tests (STREAM-02): `TestSectionSchema` -- verify schema sent on subscribe
- [ ] Timing configure tests (TIMING-01): `TestTimingConfigure` -- verify broker delay updated via configure message
- [ ] Pack settle configure tests (TIMING-02): `TestPackSettleConfigure` -- verify hub packSettleMs field updated

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | N/A -- local diagnostic tool, no auth |
| V3 Session Management | no | N/A |
| V4 Access Control | no | N/A |
| V5 Input Validation | yes | Clamp timing values to safe range server-side (min 100ms read, min 500ms settle) |
| V6 Cryptography | no | N/A |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Timing value set to 0ms causing inverter flood | Denial of Service | Server-side clamping to minimum values [VERIFIED: existing PV channels clamp pattern at hub.go lines 966-972] |
| XSS via register value display | Tampering | Use `textContent` exclusively, never `innerHTML` [VERIFIED: app.js pattern] |
| WebSocket message injection | Tampering | Server validates all inbound message fields; timing values clamped |

## Sources

### Primary (HIGH confidence)
- `internal/broker/broker.go` -- ReadBatch, executeRead, executeRead, SetInterReadDelay, interReadDelay, enforceInterReadDelay [VERIFIED: direct code review]
- `internal/hub/hub.go` -- triggerStandardRead, triggerBMSRead, triggerBatteryRead, triggerPackRead, handleConfigure, broadcastToSection [VERIFIED: direct code review]
- `internal/hub/message.go` -- OutboundMessage, GroupData, InboundMessage, ConfigPayload [VERIFIED: direct code review]
- `internal/hub/section.go` -- Section struct, timer management [VERIFIED: direct code review]
- `internal/hub/broker_iface.go` -- BrokerInterface definition [VERIFIED: direct code review]
- `web/static/app.js` -- handleSectionData, renderGroupedData, WSClient, navigateToSection, initPVDropdown [VERIFIED: direct code review]
- `web/static/index.html` -- content__header-controls layout [VERIFIED: direct code review]
- `web/static/style.css` -- existing header control styles [VERIFIED: direct code review]
- `internal/register/probe.go` -- Probe struct [VERIFIED: direct code review]
- `internal/register/probe_group.go` -- ProbeGroup struct [VERIFIED: direct code review]

### Secondary (MEDIUM confidence)
None -- all findings from direct codebase analysis.

### Tertiary (LOW confidence)
None.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- pure Go stdlib + vanilla JS, no new dependencies
- Architecture: HIGH -- all integration points verified in codebase, patterns follow existing conventions
- Pitfalls: HIGH -- identified from actual code structure and data flow analysis

**Research date:** 2026-04-11
**Valid until:** 2026-05-11 (stable stack, no external dependency changes)
