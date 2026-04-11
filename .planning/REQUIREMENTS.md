# Requirements: Sofar HYD Diagnostic Tool — v1.1

**Defined:** 2026-04-11
**Core Value:** Clear, real-time visibility into all Sofar HYD inverter parameters -- especially battery pack diagnostics -- through a reliable web interface

## v1.1 Requirements

### Modbus Timing

- [ ] **TIMING-01**: User can adjust the default Modbus read delay via UI slider/input (default 500ms)
- [ ] **TIMING-02**: Battery pack reads use a separate, longer settle time after 0x9020 write (configurable, default 1-2s)

### Streaming Display

- [ ] **STREAM-01**: Each parameter appears in the UI immediately as it is read, not after the entire batch completes
- [ ] **STREAM-02**: Loading state shows partial data with remaining parameters still loading

### Battery Pack Access

- [ ] **PACK-01**: All 20 battery packs (2 towers x 10 packs) are accessible for drill-down, matching old CLI tool behavior
- [ ] **PACK-02**: Pack selection correctly encodes tower/pack index in 0x9020 write matching proven CLI encoding
- [ ] **PACK-03**: Topology hardcoded to actual setup: 16 cells/pack, 10 packs/tower, 2 towers

## Out of Scope

| Feature | Reason |
|---------|--------|
| Charts / trend graphs | v2 feature — v1.1 focuses on correctness and UX |
| Data export (CSV/JSON) | Not needed for diagnostic use |
| Mobile layout | Desktop diagnostic tool |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| TIMING-01 | Phase 7 | Pending |
| TIMING-02 | Phase 7 | Pending |
| STREAM-01 | Phase 7 | Pending |
| STREAM-02 | Phase 7 | Pending |
| PACK-01 | Phase 6 | Pending |
| PACK-02 | Phase 6 | Pending |
| PACK-03 | Phase 6 | Pending |

**Coverage:**
- v1.1 requirements: 7 total
- Mapped to phases: 7
- Unmapped: 0

---
*Requirements defined: 2026-04-11*
