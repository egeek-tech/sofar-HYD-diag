---
phase: 10
slug: data-persistence-tooltips
status: verified
threats_open: 0
asvs_level: 1
created: 2026-04-12
---

# Phase 10 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Backend -> Frontend (WebSocket) | RegisterValueMessage carries register_addr and raw_value | uint16 register address (from probe definition), string raw value (from Modbus read) |
| Backend -> Frontend (WebSocket) | PackDataMessage carries ItemMeta and CellAddrs | Per-item register metadata from buildPackDataMessage |
| User mouse events -> Tooltip display | Mouse hover triggers tooltip with content from data attributes | Data attributes populated from WebSocket messages |

---

## Threat Register

| Threat ID | Category | Component | Disposition | Mitigation | Status |
|-----------|----------|-----------|-------------|------------|--------|
| T-10-01 | Tampering | RegisterValueMessage raw_value | accept | raw_value derived from Modbus read data via FormatRawValue, not user input. Probe addresses are compile-time constants. No injection vector. | closed |
| T-10-02 | Information Disclosure | register_addr exposed to frontend | accept | Register addresses defined in public Modbus specification. Diagnostic tool for device owner. Not sensitive. | closed |
| T-10-03 | Tampering | Tooltip content via data attributes | mitigate | Tooltip content set via `textContent` (not `innerHTML`). Data attributes from typed Go struct fields, not user input. No XSS vector. Verified: `tooltipEl.textContent = ''` at app.js:1050, no innerHTML usage in tooltip code. | closed |
| T-10-04 | Tampering | Section cache data | accept | Cache is in-memory JavaScript Map, populated only from WebSocket messages originating from Go backend. No user-supplied data enters cache. | closed |
| T-10-05 | Denial of Service | Cache memory growth | accept | Cache stores one entry per register per section (bounded by register count ~200 total). Cleared on disconnect via `sectionCache.clear()`. No unbounded growth. | closed |
| T-10-06 | Tampering | PackItemMeta raw_value in pack_data JSON | accept | raw_value derived from Modbus read data via FormatRawValue. Probe addresses are compile-time constants. Frontend uses textContent for tooltip display. | closed |
| T-10-07 | Information Disclosure | Cell register addresses exposed to frontend | accept | Register addresses defined in public Modbus specification. Diagnostic tool for device owner. Not sensitive. | closed |

*Status: open / closed*
*Disposition: mitigate (implementation required) / accept (documented risk) / transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-10-01 | T-10-01 | Raw register values are read from hardware, not user-controlled. No injection path. | gsd-security-audit | 2026-04-12 |
| AR-10-02 | T-10-02 | Modbus register addresses are public specification data, not secrets. Tool is for device owner. | gsd-security-audit | 2026-04-12 |
| AR-10-04 | T-10-04 | Cache populated exclusively from trusted backend WebSocket messages. No external input. | gsd-security-audit | 2026-04-12 |
| AR-10-05 | T-10-05 | Register count is hardware-bounded (~200). Cache cleared on disconnect. | gsd-security-audit | 2026-04-12 |
| AR-10-06 | T-10-06 | Same rationale as T-10-01. Pack metadata follows same trusted data path. | gsd-security-audit | 2026-04-12 |
| AR-10-07 | T-10-07 | Same rationale as T-10-02. Cell register addresses are public specification. | gsd-security-audit | 2026-04-12 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-04-12 | 7 | 7 | 0 | gsd-secure-phase orchestrator |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-04-12
