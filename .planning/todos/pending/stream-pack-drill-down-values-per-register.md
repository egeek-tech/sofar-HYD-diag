---
created: 2026-04-11
title: Stream pack drill-down values per-register
area: bms
files:
  - internal/hub/hub.go:801-854
---

## Problem

Pack drill-down values appear all at once instead of streaming one-by-one like section data. `triggerPackRead` uses `ReadBatch` for 3 register blocks (RT 0x9044 60 regs, Info 0x9104 35 regs, Temps 0x90BC 4 regs) and sends a single `PackDataMessage` after all reads complete. This is inconsistent with the Phase 7 streaming display behavior where section values appear progressively.

Note: These are block reads (single Modbus requests returning multiple registers), so the data arrives in one shot from the inverter per block. Breaking into individual register reads would give per-value streaming but at the cost of 60+ round-trips vs 3.

## Solution

Option A: Break the 3 block reads into individual register reads using `ReadRegisters`, sending `register_value` messages after each. Slower (60+ round-trips) but consistent streaming UX.

Option B: Keep block reads but parse and stream individual values from the response data after each block completes. 3 round-trips, values appear in 3 bursts instead of 1.

Option C: Accept current behavior — pack reads are inherently batch due to the write-settle-read cycle and block read efficiency.
