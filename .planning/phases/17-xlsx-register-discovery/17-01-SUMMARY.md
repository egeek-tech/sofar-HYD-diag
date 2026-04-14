---
phase: 17-xlsx-register-discovery
plan: 01
status: complete
started: "2026-04-14"
completed: "2026-04-14"
---

## Summary

Built the offline CLI discovery tool that performs a three-way comparison of registers: XLSX V1.29 (925 registers parsed), hardcoded V1.38 PDF map (269 registers), and current Go probe definitions (640 registers). All files use `//go:build xlsx_discover` tag to isolate excelize from the production binary.

## What Was Built

### Discovery Tool (`tools/xlsx-discover/`)
- **v138_registers.go** — Hardcoded V1.38 PDF register map covering all protocol sections: System Info 1/2, Grid, EPS, PV, Battery, Statistics, Internal Info, DCDC, PCU, BDU, Meter, BMS
- **xlsx_parser.go** — XLSX parser using excelize/v2, reads "Address Description" sheet, handles multi-line scales, CJK units, I16->S16 mapping
- **comparison.go** — Three-way comparison engine grouping by address range (14 groups), tabwriter-aligned output with ADDR/NAME/XLSX/V1.38/PROBES/STATUS columns, plus currentProbeAddrs() collecting all register exports
- **main.go** — CLI entry point with `-xlsx` flag
- **main_test.go** — 8 tests including real XLSX parsing and full comparison

### Build Infrastructure
- **Makefile** — `make server` (production binary), `make discover` (tool with tag), `make test`, `make test-discover`, `make check-size`, `make clean`

## Verification
- `make discover` — builds tool binary without errors
- `make server` — builds production binary without errors (no excelize leakage)
- `go test -tags xlsx_discover ./tools/xlsx-discover/ -v -count=1` — all 8 tests pass
- `go test ./...` — all existing tests pass (no regression)
- Tool parses real XLSX: 925 registers found
- Output shows grouped comparison with proper alignment

## Commits
1. `db83ea4` — feat(17-01): create V1.38 register map, XLSX parser, and Makefile
2. `25ec951` — feat(17-01): add three-way comparison logic, CLI entry point, and tests
