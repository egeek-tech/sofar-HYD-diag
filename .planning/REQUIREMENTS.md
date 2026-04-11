# Requirements: Sofar HYD Diagnostic Tool — v1.2

**Defined:** 2026-04-12
**Core Value:** Clear, real-time visibility into all Sofar HYD inverter parameters — especially battery pack diagnostics — through a reliable web interface

## v1.2 Requirements

### Reliability

- [ ] **REL-01**: User can disconnect and the connection closes immediately, aborting any in-progress Modbus reads within 1 second
- [ ] **REL-02**: Register reads that return errors are automatically retried (up to 3 total attempts) before showing an error
- [ ] **REL-03**: Inter-read delay is consistently enforced between all Modbus reads, with no burst of rapid reads on section switch

### Auto-Refresh

- [ ] **REFR-01**: Auto-refresh is triggered only by the browser — backend performs no autonomous refresh cycles
- [ ] **REFR-02**: Auto-refresh timer restarts after each read cycle completes (not on a fixed interval)

### Data Display

- [ ] **DISP-01**: Previously read parameter values persist on screen (dimmed) when a new refresh cycle begins, until replaced by fresh values
- [ ] **DISP-02**: Browser caches values per section and page — navigating back to a previously viewed section shows cached values dimmed until refreshed
- [ ] **DISP-03**: User can hover over any parameter value to see a tooltip showing the register address and raw value

### Battery Pack

- [ ] **BATT-01**: Balance state section appears before temperature section in pack drill-down view
- [ ] **BATT-02**: Pack drill-down values stream per-register as they are read, consistent with other sections

## Future Requirements

### Deferred from v1.2

- **DISP-04**: Per-register retry configuration UI
- **DISP-05**: Register value history in tooltips
- **REFR-03**: Auto-refresh interval configuration slider

## Out of Scope

| Feature | Reason |
|---------|--------|
| Charts / trend graphs | v2+ feature — v1.2 focuses on reliability |
| Data export (CSV/JSON) | Not needed for diagnostic use |
| Mobile layout | Desktop diagnostic tool |
| SSE migration | WebSocket working well, no need to change transport |
| Multi-client timer coordination | 1-2 tabs typical for diagnostic tool |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| REL-01 | Phase 9 | Pending |
| REL-02 | Phase 9 | Pending |
| REL-03 | Phase 8 | Pending |
| REFR-01 | Phase 8 | Pending |
| REFR-02 | Phase 8 | Pending |
| DISP-01 | Phase 10 | Pending |
| DISP-02 | Phase 10 | Pending |
| DISP-03 | Phase 10 | Pending |
| BATT-01 | Phase 11 | Pending |
| BATT-02 | Phase 11 | Pending |

**Coverage:**
- v1.2 requirements: 10 total
- Mapped to phases: 10
- Unmapped: 0

---
*Requirements defined: 2026-04-12*
*Last updated: 2026-04-12 after roadmap creation*
