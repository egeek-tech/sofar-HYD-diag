# Security Audit — Phase 09: Connection Read Resilience

**Phase:** 09 — connection-read-resilience
**ASVS Level:** 1
**Audited:** 2026-04-12
**Auditor:** gsd-security-auditor

---

## Threat Verification

| Threat ID | Category | Disposition | Status | Evidence |
|-----------|----------|-------------|--------|----------|
| T-09-01 | Tampering | mitigate | CLOSED | broker.go:89 `connMu sync.Mutex`; abortRead() locks at :257; executeDisconnect() at :521; handleError() at :583; cleanup() at :599; executeReconfigure() at :491,:511 |
| T-09-02 | Denial of Service (deadlock) | mitigate | CLOSED | abortRead() (broker.go:255-262) performs only Store+Lock+SetReadDeadline+Unlock — no channel sends or waits. Disconnect() calls abortRead() at :293 before command send at :297. aborting flag (broker.go:90,:421) prevents retry-after-abort loop |
| T-09-03 | Tampering | accept | CLOSED | See Accepted Risks below |
| T-09-04 | Denial of Service (resource) | accept | CLOSED | See Accepted Risks below |

---

## Accepted Risks

### T-09-03: isRetryable error string matching

**Component:** `isRetryable()` in `internal/broker/broker.go`
**Risk:** String-based error classification (`strings.Contains(err.Error(), "err=0x02")`) could misclassify if error format changes.
**Acceptance Rationale:** Error strings are generated exclusively by `internal/modbus/tcp.go` and `internal/modbus/rtu.go` within the same codebase — they are not sourced from external input. The format `"exception: func=0xNN err=0xNN"` is stable and tested. Matching `"err=0x02"` is specific enough to avoid false positives on other hex values in the string.
**Owner:** internal — format change in modbus package requires updating isRetryable
**Residual Risk:** Low. Any format change in modbus error strings would surface as a test failure in `TestBrokerNoRetryIllegalAddress`.

### T-09-04: Retry loop consuming resources

**Component:** `executeRead()` in `internal/broker/broker.go`
**Risk:** Retry loop could consume resources (connections, CPU, time) during degraded inverter conditions.
**Acceptance Rationale:** `maxAttempts=3` is a hard bound. Each attempt includes the inter-read delay (500ms default, 10ms in tests) and reconnect backoff. Context cancellation provides an unconditional escape hatch via `ensureConnected`. Worst-case per-register time is approximately 3 seconds, which is acceptable for a diagnostic tool. The `aborting` flag (added in 09-01 deviation) further bounds resource use by cutting retries on explicit disconnect.
**Owner:** internal — acceptable for the application's use case and user population (single local user)
**Residual Risk:** Low. Bounded by design; escape hatch always available.

---

## Threat Flags (from SUMMARY.md)

| Flag | Source Plan | Maps To | Classification |
|------|-------------|---------|----------------|
| aborting atomic.Bool added to prevent retry-after-abort loop | 09-01-SUMMARY deviation | T-09-02 | Informational — strengthens existing mitigation, not a new threat surface |

No unregistered flags.

---

## Files Audited

- `internal/broker/broker.go`
- `internal/hub/hub.go`
- `.planning/phases/09-connection-read-resilience/09-01-PLAN.md`
- `.planning/phases/09-connection-read-resilience/09-02-PLAN.md`
- `.planning/phases/09-connection-read-resilience/09-01-SUMMARY.md`
- `.planning/phases/09-connection-read-resilience/09-02-SUMMARY.md`
