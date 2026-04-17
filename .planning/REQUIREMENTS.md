# Requirements: Sofar HYD Diagnostic Tool

**Defined:** 2026-04-15
**Core Value:** Provide clear, real-time visibility into all Sofar HYD inverter parameters through a reliable web interface

## v1.5 Requirements

Requirements for v1.5 Full Batch Reading & Configuration Cleanup. Each maps to roadmap phases.

### Configuration Cleanup

- [ ] **CONF-01**: Configuration section excludes registers that return illegal data address on this hardware
- [ ] **CONF-02**: Configuration groups with no working registers are hidden from the UI
- [ ] **CONF-03**: Configuration section loads without batch span fallback warnings in server logs

### Batch Verification

- [ ] **BVER-01**: Grid section loads via batch spans on real hardware without errors
- [ ] **BVER-02**: EPS section loads via batch spans on real hardware without errors
- [ ] **BVER-03**: PV section loads via batch spans on real hardware without errors
- [ ] **BVER-04**: Meter section loads via batch spans on real hardware without errors
- [ ] **BVER-05**: DCDC section loads via batch spans on real hardware without errors
- [ ] **BVER-06**: PCU section loads via batch spans on real hardware without errors
- [ ] **BVER-07**: BDU section loads via batch spans on real hardware without errors

### Battery Migration

- [ ] **BATT-01**: Battery section reads registers via BatchPlan spans instead of individual reads
- [ ] **BATT-02**: Battery channel auto-detection (0x066A) still works correctly after migration
- [ ] **BATT-03**: Battery section UI renders identically to pre-migration behavior

### Pack Drill-Down Migration

- [ ] **PACK-01**: Pack drill-down reads registers via BatchPlan spans instead of individual reads
- [ ] **PACK-02**: Pack selection write (0x9020) and 1s settle delay preserved before reading
- [ ] **PACK-03**: Known unsupported PackInfo registers (0x9104+) are skipped without errors
- [ ] **PACK-04**: Pack drill-down UI renders identically to pre-migration behavior

### Resilience

- [ ] **RESIL-01**: SpanTracker wired into streamStandardRead to track persistently-failing spans
- [ ] **RESIL-02**: Spans that fail consistently are automatically skipped on subsequent reads

### BMS Migration

- [ ] **BMS-01**: BMS section reads registers via BatchPlan spans instead of individual reads
- [ ] **BMS-02**: BMS clock composition (0x9004+0x9005) renders correctly after migration
- [ ] **BMS-03**: BMS SW version composition (0x9018-0x901B) renders correctly after migration
- [ ] **BMS-04**: BMS bitmap and protection post-processing preserved after migration

### Milestone Cleanup

- [x] **CLEAN-01**: PV handleConfigure recomputes sec.BatchPlan after channel change
- [x] **CLEAN-02**: Dead readSpanIndividualFallback (non-Accum) removed from hub_streaming.go
- [x] **CLEAN-03**: DCDC nav button removed from index.html
- [x] **CLEAN-04**: Orphaned config enum maps removed from config_enum.go

## Future Requirements

Deferred to future release. Tracked but not in current roadmap.

### Batch Diagnostics

- **BDIAG-01**: API endpoint exposing batch plan span statistics per section
- **BDIAG-02**: Gap-filling reads for sparse register ranges

### Dynamic Discovery

- **DISC-01**: Runtime register auto-discovery for hardware-specific register support

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Cross-section batch merging | Sections are independent streaming units; merging adds complexity for marginal gain |
| Dynamic register auto-discovery | Register map is fixed per hardware model; static removal is correct and simpler |
| Batch plan UI visualization | Developer-facing diagnostic, not user-facing value |
| Write register batching | Read-only diagnostic tool; no write batching needed |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| CONF-01 | Phase 20 | Pending |
| CONF-02 | Phase 20 | Pending |
| CONF-03 | Phase 20 | Pending |
| BVER-01 | Phase 21 | Pending |
| BVER-02 | Phase 21 | Pending |
| BVER-03 | Phase 21 | Pending |
| BVER-04 | Phase 21 | Pending |
| BVER-05 | Phase 21 | Pending |
| BVER-06 | Phase 21 | Pending |
| BVER-07 | Phase 21 | Pending |
| RESIL-01 | Phase 22 | Pending |
| RESIL-02 | Phase 22 | Pending |
| BATT-01 | Phase 23 | Pending |
| BATT-02 | Phase 23 | Pending |
| BATT-03 | Phase 23 | Pending |
| BMS-01 | Phase 24 | Pending |
| BMS-02 | Phase 24 | Pending |
| BMS-03 | Phase 24 | Pending |
| BMS-04 | Phase 24 | Pending |
| PACK-01 | Phase 25 | Pending |
| PACK-02 | Phase 25 | Pending |
| PACK-03 | Phase 25 | Pending |
| PACK-04 | Phase 25 | Pending |
| CLEAN-01 | Phase 26 | Complete |
| CLEAN-02 | Phase 26 | Complete |
| CLEAN-03 | Phase 26 | Complete |
| CLEAN-04 | Phase 26 | Complete |

**Coverage:**
- v1.5 requirements: 27 total
- Mapped to phases: 27
- Unmapped: 0

---
*Requirements defined: 2026-04-15*
*Last updated: 2026-04-15 after roadmap creation*
