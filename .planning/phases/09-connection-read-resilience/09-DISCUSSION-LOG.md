# Phase 9: Connection & Read Resilience - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-12
**Phase:** 09-connection-read-resilience
**Areas discussed:** Disconnect abort behavior, Register retry strategy, Error display behavior

---

## Disconnect abort behavior

### Q1: How should disconnect abort an in-progress read?

| Option | Description | Selected |
|--------|-------------|----------|
| Close TCP socket immediately | Close the raw TCP connection from outside the command loop. Fastest path to 1-second guarantee. | |
| Cancel via context propagation | Hub's context cancellation propagates to broker's readCtx. Cleaner separation of concerns. | ✓ |
| Both: context + socket close | Cancel context AND close socket. Belt-and-suspenders approach. | |

**User's choice:** Cancel via context propagation
**Notes:** None

### Q2: Should the UI transition to disconnected state immediately or wait for backend confirmation?

| Option | Description | Selected |
|--------|-------------|----------|
| Immediate optimistic transition | UI shows disconnected as soon as user clicks. Backend confirms asynchronously. | |
| Wait for backend confirmation | UI stays connected until backend sends state_change:disconnected. More accurate. | ✓ |

**User's choice:** Wait for backend confirmation
**Notes:** This means context cancellation + deadline shortening must be fast enough for <1s confirmation.

### Q3: Should we shorten read deadline on disconnect to unblock TCP reads?

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, shorten read deadline on disconnect | Set conn.SetReadDeadline(time.Now()) to immediately unblock pending reads. | ✓ |
| Accept possible delay | Rely purely on context cancellation. May take up to 10s read deadline to unblock. | |

**User's choice:** Yes, shorten read deadline on disconnect
**Notes:** Combined with context cancellation, this guarantees <1s disconnect response.

---

## Register retry strategy

### Q1: Where should the additional retry logic live?

| Option | Description | Selected |
|--------|-------------|----------|
| Broker level — increase maxAttempts to 3 | Change executeRead's maxAttempts from 2 to 3. Simplest, keeps retry logic centralized. | ✓ |
| Streaming level — retry wrapper | Keep broker at 2, add retry in hub_streaming.go. More flexible but split logic. | |
| Both levels | Broker retries 2x, streaming adds one more. Total 3 but split across layers. | |

**User's choice:** Broker level — increase maxAttempts to 3
**Notes:** None

### Q2: Which errors should be retried vs treated as permanent?

| Option | Description | Selected |
|--------|-------------|----------|
| Retry all errors except illegal address | Retry timeout/connection errors. Don't retry Modbus 0x02 (illegal address). | ✓ |
| Retry only connection/timeout errors | Don't retry any Modbus-level exceptions. | |
| Retry everything | Retry all errors uniformly. | |

**User's choice:** Retry all errors except illegal address
**Notes:** Aligns with Phase 8's PackInfoProbes skip logic.

---

## Error display behavior

### Q1: What should the user see while a register is being retried?

| Option | Description | Selected |
|--------|-------------|----------|
| Show nothing — suppress until final result | Keep previous value or skeleton visible during retry. User never sees transient errors. | ✓ |
| Show subtle retry indicator | Small spinner on the specific register being retried. | |
| Show error immediately, replace on success | Display error as it occurs, replace with value on retry success. | |

**User's choice:** Show nothing — suppress until final result
**Notes:** None

### Q2: After all 3 retries fail, how should the error be displayed?

| Option | Description | Selected |
|--------|-------------|----------|
| Red text with em-dash replacing value | Replace value with red em-dash. Consistent with skeleton loading. | |
| Red background flash + error text | Flash row red, show 'Error' as value. More prominent. | |
| Keep previous value with error indicator | If previous value exists, keep it with warning icon/dimming. Else show em-dash. | ✓ |

**User's choice:** Keep previous value with error indicator
**Notes:** Preserves data continuity — user always sees last known good value.

---

## Claude's Discretion

- How to expose conn.SetReadDeadline to the disconnect path
- Whether to add a retryable(err) helper or inline the illegal-address check
- How to track previous successful values in the frontend
- Warning icon style for stale values

## Deferred Ideas

- Stream pack drill-down values per-register (Phase 11 scope)
