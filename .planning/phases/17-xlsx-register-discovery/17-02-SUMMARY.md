---
phase: 17-xlsx-register-discovery
plan: 02
status: complete
started: "2026-04-14"
completed: "2026-04-14"
---

## Summary

Created four new register definition files (Meter, DCDC, PCU, BDU) covering previously undiscovered protocol sections from the Sofar Modbus-G3 V1.38 specification, wired them into the hub as standard grouped sections, and added sidebar navigation buttons with icons matching the UI-SPEC.

## What Was Built

### New Register Files
- **internal/register/meter.go** — MeterGroups: 2 registers (0x7080 Auto-ID status, 0x7081 Connected meters)
- **internal/register/dcdc.go** — DCDCGroups: 4 groups (System Info, Real-Time Data, Faults, Capacity) covering DCDC converter monitoring at 0x5000-0x5077
- **internal/register/pcu.go** — PCUGroups: 5 groups (Info, Temperatures, Alarms, Faults, DC Measurements) covering PCU at 0x6004-0x602F
- **internal/register/bdu.go** — BDUGroups: 2 groups (Info, Topology) covering BDU at 0x6084-0x60A2
- **internal/register/dcdc_enum.go** — DCDCRunningStateEnum and PCUModuleStateEnum enum maps

### Hub Wiring
- Added 4 `RegisterGroupedSection()` calls in `internal/hub/hub.go` for meter, dcdc, pcu, bdu
- Placed after BMS section registration, before read-once configuration block

### Sidebar Navigation
- Added 4 buttons to `web/static/index.html` sidebar after BMS button
- Icons: Meter (chart), DCDC (arrows), PCU (alembic), BDU (shield)

## Verification
- `go build ./internal/register/` — passed
- `go test ./internal/register/` — passed
- `go build -o /tmp/test-server ./cmd/server` — passed
- `go test ./internal/hub/` — passed

## Commits
1. `229e8f9` — feat(17-02): add register definitions for Meter, DCDC, PCU, and BDU sections
2. `3198a18` — feat(17-02): register Meter/DCDC/PCU/BDU sections in hub and add sidebar buttons
