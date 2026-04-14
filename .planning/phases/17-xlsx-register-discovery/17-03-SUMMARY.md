---
phase: 17-xlsx-register-discovery
plan: 03
status: complete
started: "2026-04-14"
completed: "2026-04-14"
---

## Summary

Ran the discovery tool against the real XLSX, identified 30 registers missing from probes, appended 22 diagnostically valuable read-only registers to existing sections, and verified build tag isolation. Remaining 8 gaps are correctly excluded (5 system time sub-registers already covered by synthetic probe, 3 RW/W control registers outside read-only scope).

## What Was Built

### Gap Closure — 22 registers added across 4 files
- **internal/register/system.go** — Added to Status group: grid-connected wait time (0x0417), power gen time today (0x0426). New "Firmware (Extended)" group: ARM/DSP BOOT versions (0x045D-0x045F), safety cert/package versions (0x0460, 0x0467). Added to PCC Power group: PCC apparent power (0x048A), PCC active power 2 (0x048B), PCC per-phase current/power R (0x0492-0x0494). Added to Load group: external power gen (0x04AE). Added to EPS Phase R: load active power (0x050C).
- **internal/register/battery.go** — Added to BMS Info: hibernation state (0x9023), max discharge/charge current limits (0x902F-0x9030). New InternalInfoGroups(): BUS voltages (0x06CC-0x06CE), buck boost current (0x06D0), rated power (0x06ED).
- **internal/hub/hub.go** — Battery section now includes InternalInfoGroups.
- **tools/xlsx-discover/comparison.go** — Updated currentProbeAddrs() to include InternalInfoGroups.

### Verification Results
- Server binary size: 11,501,880 bytes (~11 MB) — no excelize leakage
- `go test ./...` — all pass
- `go test -tags xlsx_discover ./tools/xlsx-discover/` — all 8 tests pass
- "Missing from probes" reduced from 30 to 8 (all correctly excluded)
- User approved checkpoint verification

## Commits
1. `3b161ad` — feat(17-03): append missing registers from XLSX discovery to existing sections
