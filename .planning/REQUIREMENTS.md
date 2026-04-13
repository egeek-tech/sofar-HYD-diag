# Requirements: Sofar HYD Diagnostic Tool -- v1.3

**Defined:** 2026-04-13
**Core Value:** Clear, real-time visibility into all Sofar HYD inverter parameters -- especially battery pack diagnostics -- through a reliable web interface

## v1.3 Requirements

### Data Cleanup

- [ ] **CLEAN-01**: Empty "This Month" and "This Year" statistics groups are removed from the statistics display
- [ ] **CLEAN-02**: System time displays as a single concatenated row (e.g., "10:59:34 13-04-2026") instead of separate rows per register
- [ ] **CLEAN-03**: Zero-value temperatures (0.0C) in pack drill-down are hidden or dimmed as disconnected sensors

### Section Reorganization

- [ ] **SECT-01**: Statistics data (daily/total) is merged into the System section -- separate Statistics section and sidebar button removed
- [ ] **SECT-02**: New Configuration section displays device config registers (System, Battery, Function, Safety, Reactive Power, etc.) read-only
- [ ] **SECT-03**: Additional daily statistics (battery charge today, PV power today, etc.) appear alongside existing stats in System section

### Tooltip Fixes

- [ ] **TIP-01**: Balance State values in pack drill-down show tooltips with register address and raw value on hover
- [ ] **TIP-02**: Pack Status values in pack drill-down show tooltips with register address and raw value on hover

### Register Discovery

- [ ] **REG-01**: Offline CLI tool parses XLSX register map and compares against PDF V1.38 register definitions
- [ ] **REG-02**: Meter registers (0x7080+) and any newly discovered registers from XLSX are integrated where valuable

### Code Quality

- [ ] **QUAL-01**: Deprecated triggerPackRead and related dead code (~250 lines) removed
- [ ] **QUAL-02**: Go test suite uses testify (assert/require) with coverage for new and modified packages

## Future Requirements

### Deferred from v1.3

- **SAFE-01**: Safety parameter monitoring section
- **DIAG-01**: Internal diagnostics register display
- **CONF-WR**: Configuration register write support

## Out of Scope

| Feature | Reason |
|---------|--------|
| Configuration writes | Read-only diagnostic tool -- accidental writes could damage inverter settings |
| Safety parameter monitoring | Large scope -- defer to v1.4 |
| Internal diagnostics | Large scope -- defer to v1.4 |
| Charts / trend graphs | v2+ feature |
| Data export (CSV/JSON) | Not needed for diagnostic use |
| Mobile layout | Desktop diagnostic tool |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| CLEAN-01 | Phase 13 | Pending |
| CLEAN-02 | Phase 14 | Pending |
| CLEAN-03 | Phase 16 | Pending |
| SECT-01 | Phase 13 | Pending |
| SECT-02 | Phase 15 | Pending |
| SECT-03 | Phase 13 | Pending |
| TIP-01 | Phase 16 | Pending |
| TIP-02 | Phase 16 | Pending |
| REG-01 | Phase 17 | Pending |
| REG-02 | Phase 17 | Pending |
| QUAL-01 | Phase 12 | Pending |
| QUAL-02 | Phase 12 | Pending |

**Coverage:**
- v1.3 requirements: 12 total
- Mapped to phases: 12
- Unmapped: 0

---
*Requirements defined: 2026-04-13*
*Last updated: 2026-04-13 after roadmap creation*
