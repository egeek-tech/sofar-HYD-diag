# SECURITY.md — Phase 07: Streaming Display and Configurable Timing

**ASVS Level:** 1
**Audit Date:** 2026-04-11
**Auditor:** gsd-security-auditor

---

## Threat Verification Summary

**Closed:** 7/7 | **Open:** 0/7

---

## Closed Threats

| Threat ID | Category | Disposition | Evidence |
|-----------|----------|-------------|----------|
| T-07-03 | Information Disclosure | accept | Register values are inverter telemetry; no PII or secrets. Local diagnostic tool on trusted network. Logged below. |
| T-07-04 | Denial of Service | accept | 500ms+ inter-read delay enforced by broker; broadcastResultToSection drops slow clients. Local diagnostic tool. Logged below. |
| T-07-06 | Tampering (XSS) | mitigate | web/static/app.js:920 — `el.textContent = msg.value || '\u2014'`. No innerHTML usage in streaming handlers. All streaming value rendering uses textContent exclusively. |
| T-07-07 | Information Disclosure | accept | localStorage stores only integer timing values (readDelay, packSettle). No PII, no credentials, no secrets. Local diagnostic tool. Logged below. |
| T-07-05 | Tampering | mitigate | web/static/app.js:964 — `clamp(readDelayInput.value, 50, 5000)` and `:965` — `clamp(packSettleInput.value, 500, 10000)`. Client-side clamping present. Server-side authoritative clamping at 50ms also present. |
| T-07-01 | Denial of Service | mitigate | hub.go:735 — `if delay < 50 { delay = 50 }`. Floor raised to 50ms, above inverter timeout threshold. All layers aligned (server, JS, HTML). |
| T-07-02 | Tampering | mitigate | hub.go:735 + app.js:964 + index.html min=50. Three-layer validation ensures read_delay_ms cannot go below 50ms. |

---

## Open Threats

None — all threats closed.

### Previously Open (resolved 2026-04-11)

| Threat ID | Category | Resolution |
|-----------|----------|------------|
| T-07-01 | Denial of Service | CLOSED: Floor raised to 50ms (hub.go:735 `if delay < 50`). Compromise between plan's 100ms and user's 10ms testing request. 50ms is above inverter timeout threshold. |
| T-07-02 | Tampering | CLOSED: All three layers aligned to 50ms floor — server (hub.go), JS (app.js clamp), HTML (input min=50). |

---

## Accepted Risks Log

| Threat ID | Risk | Rationale | Owner |
|-----------|------|-----------|-------|
| T-07-03 | Information Disclosure — register_value messages expose inverter telemetry over WebSocket | Values are Modbus diagnostic data; no PII or secrets. Tool is deployed on a local network with direct physical access to the inverter. No authentication layer is in scope for this tool (accepted at architecture level). | Robert Tkocz |
| T-07-04 | Denial of Service — per-register streaming generates high WebSocket message volume | Inter-read delay of 500ms+ enforced by broker limits to ~2 messages/sec. Slow clients are dropped by broadcastResultToSection's non-blocking send. Local single-user diagnostic tool; no availability SLA. | Robert Tkocz |
| T-07-07 | Information Disclosure — timing values persisted in localStorage | localStorage stores only integer read_delay_ms and pack_settle_ms values. No credentials, no PII, no sensitive inverter data. Browser localStorage is accessible to the page origin only. | Robert Tkocz |

---

## Unregistered Flags

None. No `## Threat Flags` section was present in any 07-0x-SUMMARY.md file.
