# Phase 5: Deep Battery Pack Diagnostics - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-11
**Phase:** 05-deep-battery-pack-diagnostics
**Areas discussed:** Pack Selection, Navigation UX, Cell Voltage Viz, Bitmap Decoding, Temperature Display
**Mode:** --auto (all decisions auto-selected with recommended defaults)

---

## Pack Selection Mechanism

| Option | Description | Selected |
|--------|-------------|----------|
| Write 0x9020 via func 0x10 with retry | 1s settle, read 0x9044-0x907F, retry once with 2s on timeout | ✓ |
| Skip 0x9020, use cluster registers only | Read 0x9400+/0x9600+ directly (limited data) | |

**User's choice:** [auto] Write 0x9020 via func 0x10 with retry (recommended default)
**Notes:** On-demand only — user clicks specific pack, no automatic cycling. 0x06 times out, must use 0x10.

---

## Hierarchical Navigation UX

| Option | Description | Selected |
|--------|-------------|----------|
| Bitmap click + breadcrumb + dropdowns | Click grid cell or use dropdown selectors, breadcrumb nav bar | ✓ |
| Separate pack detail section | New sidebar section for pack view | |
| Tree view navigation | Expandable tree in sidebar | |

**User's choice:** [auto] Bitmap click + breadcrumb + dropdowns (recommended default)
**Notes:** Pack detail as sub-view within BMS section. Bitmap click is primary, dropdowns as alternative. Per Phase 4 D-07.

---

## Cell Voltage Visualization

| Option | Description | Selected |
|--------|-------------|----------|
| Color-coded 6x4 grid with deviation | Green/amber/red based on mV from average, summary row | ✓ |
| Simple table list | 24 rows, no color coding | |
| Bar chart visualization | Horizontal bars for each cell | |

**User's choice:** [auto] Color-coded 6x4 grid with deviation (recommended default)
**Notes:** 5mV green, 5-20mV amber, >20mV red. Min/max/spread summary row above grid.

---

## Bitmap State Decoding

| Option | Description | Selected |
|--------|-------------|----------|
| Decoded text list with severity colors | Green "All Clear" or amber/red per-bit labels, matching fault card | ✓ |
| Raw hex values only | Show register values without decoding | |
| Table with bit positions | Technical table showing each bit | |

**User's choice:** [auto] Decoded text list with severity colors (recommended default)
**Notes:** Follows Phase 3 fault card pattern. Protection trips red, alarms amber.

---

## Temperature Display

| Option | Description | Selected |
|--------|-------------|----------|
| Labeled rows with color ranges | 10 temps in rows, green/amber/red for range | ✓ |
| Compact inline display | All temps on one line | |
| Temperature gauge widgets | Visual gauge per sensor | |

**User's choice:** [auto] Labeled rows with color ranges (recommended default)
**Notes:** 8 cell temps + MOS + env. Green 15-45°C, amber 45-55°C, red >55°C or <0°C.

---

## Claude's Discretion

- Pack 0x9020 encoding byte layout
- Protection/alarm bit-to-description mappings
- Cell grid sizing and layout
- Pack detail view arrangement
- Whether to include fault snapshot data (0x9084-0x90FF)

## Deferred Ideas

- Cell voltage trend analysis (v2 ADV-01)
- Battery cluster direct read (v2 EXT-04)
- Historical fault snapshot (researcher to assess)
