# Phase 9: Connection & Read Resilience - Context

**Gathered:** 2026-04-12
**Status:** Ready for planning

<domain>
## Phase Boundary

Immediate disconnect response (abort in-progress reads within 1 second) and transparent per-register error recovery (retry up to 3 times before showing error). The backend's reactive architecture from Phase 8 is preserved — this phase adds resilience to the read and disconnect paths without changing the browser-driven refresh model.

</domain>

<decisions>
## Implementation Decisions

### Disconnect abort behavior
- **D-01:** Disconnect uses context cancellation to abort in-progress reads. The Hub cancels the readCtx, which propagates through broker operations and streaming goroutines.
- **D-02:** When disconnect is requested and a TCP read is blocking on the socket, set `conn.SetReadDeadline(time.Now())` to immediately unblock the pending read. The read returns a timeout error, context cancellation catches it, and disconnect proceeds. This guarantees <1s disconnect response.
- **D-03:** The UI waits for backend confirmation (state_change:disconnected) before transitioning to disconnected state — no optimistic UI update. The context cancellation + deadline shortening path must be fast enough that this confirmation arrives within ~1s.
- **D-04:** On disconnect, all pending refresh timers in the browser are cleared (already implemented in Phase 8 D-13). No additional browser-side changes needed for the disconnect path.

### Register retry strategy
- **D-05:** Register retries happen at the broker level. Change `executeRead`'s `maxAttempts` from 2 to 3. This keeps retry logic centralized in one place rather than splitting across broker and streaming layers.
- **D-06:** Retry all errors except Modbus exception 0x02 (illegal address). Timeout, connection reset, and other I/O errors are retried. Illegal address is permanent — the register doesn't exist on this hardware. This aligns with Phase 8's PackInfoProbes skip logic.
- **D-07:** Between retry attempts, the existing reconnect-on-error behavior is preserved. If a read fails, the broker closes and re-establishes the TCP connection before retrying.

### Error display behavior
- **D-08:** During retries, suppress errors — don't update the display until the final outcome is known. Keep the previous value (or em-dash skeleton if no previous value) visible while retrying. The user never sees transient errors.
- **D-09:** After all 3 retries fail: if a previous successful value exists, keep it displayed but add a small warning icon or dim it to indicate staleness. If no previous value exists, show em-dash. This preserves data continuity.
- **D-10:** Successful retries display the value normally — the user never knows a retry happened. This directly addresses REL-02 success criterion 4.

### Claude's Discretion
- How to expose `conn.SetReadDeadline` to the disconnect path (mutex, method on broker, or separate abort channel)
- Whether to add a `retryable(err)` helper function or inline the illegal-address check
- How to track "previous successful value" in the frontend (per-register cache or DOM-based)
- Whether the warning icon for stale values should be a CSS class change, an SVG icon, or a text indicator

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Broker implementation
- `internal/broker/broker.go` — executeRead (line ~369), executeDisconnect (line ~482), enforceInterReadDelay, ensureConnected
- `internal/broker/broker.go` — handleError pattern (line ~541): dormant flag prevents auto-reconnect after disconnect

### Hub and streaming
- `internal/hub/hub.go` — handleDisconnect (line ~213), triggerSectionRead with readCtx (line ~364), cancelRead on disconnect (line ~251)
- `internal/hub/hub_streaming.go` — streamStandardRead, streamBatteryRead, streamBMSRead — all accept readCtx and check ctx.Err() between probes
- `internal/hub/section.go` — cancelRead() method, readCancel context.CancelFunc field

### Modbus transport
- `internal/modbus/` — ReadHoldingRegistersTCP, ReadHoldingRegistersRTU — where the blocking TCP reads happen

### Phase 8 context (prior decisions)
- `.planning/phases/08-refresh-architecture/08-CONTEXT.md` — D-02 (section switch cancellation), D-12 (no backend timers), D-13 (stop auto-refresh stops reads)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `Section.cancelRead()` — context cancellation pattern already working for section switches; extend for disconnect
- `readCtx context.Context` parameter — already threaded through all streaming functions
- `Broker.handleError()` — already closes connection and sets reconnecting state; retry logic builds on this
- Phase 8's `skipPackInfo` atomic.Bool — pattern for session-level error suppression

### Established Patterns
- Command-channel architecture in broker — all operations go through `b.commands` channel
- Context cancellation checked between probes in streaming reads (`readCtx.Err()`)
- `reading` atomic.Bool flag prevents overlapping reads per section
- Hub event loop is single-threaded — state mutations are safe without locks

### Integration Points
- `Broker.Disconnect()` → command channel → `executeDisconnect()` — needs access to conn for deadline shortening
- `hub_streaming.go` probe loops — where retry suppression and error display decisions manifest
- `web/static/app.js` handleRegisterValue — where stale value display and warning indicators would be added

</code_context>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches for context cancellation and retry patterns.

</specifics>

<deferred>
## Deferred Ideas

### Reviewed Todos (not folded)
- **Read delay burst on section switch** — Already fixed in Phase 8 (D-03, REL-03). Todo can be closed.
- **Skip unsupported PackInfoProbes registers** — Already implemented in Phase 8 (skipPackInfo atomic.Bool). Todo can be closed.
- **Stream pack drill-down values per-register** — Phase 11 concern (Battery Pack Polish), not Phase 9 scope.

</deferred>

---

*Phase: 09-connection-read-resilience*
*Context gathered: 2026-04-12*
