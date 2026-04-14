# Requirements: Sofar HYD Diagnostic Tool -- v1.4

**Defined:** 2026-04-14
**Core Value:** Clear, real-time visibility into all Sofar HYD inverter parameters -- especially battery pack diagnostics -- through a reliable web interface

## v1.4 Requirements

### Batch Reading

- [ ] **BATCH-01**: Register groups with contiguous address ranges are read in a single Modbus request instead of per-register reads
- [ ] **BATCH-02**: System section load time is measurably faster with batch reading vs individual reads (target: 3-5x improvement)
- [ ] **BATCH-03**: Configuration section load time benefits from batch reading on its single read-once cycle
- [ ] **BATCH-04**: Existing per-register streaming behavior is preserved -- values still appear progressively in the UI as each batch completes

### Safety

- [ ] **SAFE-02**: Batch reads respect the Modbus 60-register-per-request limit and do not cross protocol block boundaries
- [ ] **SAFE-03**: Registers that fail in a batch fall back to individual reads (no data loss from one bad register killing a batch)

## Future Requirements

### Deferred from v1.4

- **BATCH-EXT**: Extend batch reading to all remaining sections (Grid, EPS, PV, Battery, Meter, DCDC, PCU, BDU)
- **SAFE-01**: Safety parameter monitoring section
- **DIAG-01**: Internal diagnostics register display

## Out of Scope

| Feature | Reason |
|---------|--------|
| Batch reading for BMS pack data | Pack reads use write-then-read protocol (0x9020), different pattern |
| Configurable batch size | Fixed at protocol max (60 regs), no user knob needed |
| Parallel Modbus requests | Single TCP connection, serial protocol |
| Configuration writes | Read-only diagnostic tool -- accidental writes could damage inverter settings |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| BATCH-01 | Phase 18 | Pending |
| BATCH-02 | Phase 19 | Pending |
| BATCH-03 | Phase 19 | Pending |
| BATCH-04 | Phase 19 | Pending |
| SAFE-02 | Phase 18 | Pending |
| SAFE-03 | Phase 18 | Pending |

**Coverage:**
- v1.4 requirements: 6 total
- Mapped to phases: 6
- Unmapped: 0

---
*Requirements defined: 2026-04-14*
