---
status: partial
phase: 04-battery-overview-and-statistics
source: [04-VERIFICATION.md]
started: 2026-04-11T10:15:00Z
updated: 2026-04-11T10:15:00Z
---

## Current Test

[awaiting human testing]

## Tests

### 1. Battery Section Live Rendering
expected: Navigate to Battery section with inverter connected; each battery channel shows 10 data rows (voltage, current, power, SOC, SOH, cycles, etc.) with correct units; State row shows human-readable label (Charging/Discharging/Sleeping/Fault/Loss reduction), not raw number
result: [pending]

### 2. BMS Bitmap Grid and Info
expected: Navigate to BMS section; BMS Info card shows manufacturer, protocol version, cell type, total voltage, current, avg temp, SOC, SOH; Battery Topology card renders colored pack grid with tower labels, detected topology string, green=online/gray=offline legend
result: [pending]

### 3. Topology Configuration Persistence
expected: Change Inputs/Towers/Packs dropdowns in BMS section; navigate away and back; reload page; dropdown values persist via localStorage; configure message sent to server triggers BMS re-read with new topology
result: [pending]

### 4. Statistics U32 Values
expected: Navigate to Statistics section; 4 groups visible (Today, Total, This Month, This Year); each shows 6 energy metrics (Power Generation, Load Consumption, Grid Bought, Grid Sold, Battery Charge, Battery Discharge) with kWh values formatted to 2 decimal places from U32 registers
result: [pending]

### 5. Auto-Refresh Toggle Cross-Section
expected: Toggle auto-refresh OFF; navigate between Battery/BMS/Statistics sections; toggle state remains OFF; no spurious refresh messages sent to server
result: [pending]

## Summary

total: 5
passed: 0
issues: 0
pending: 5
skipped: 0
blocked: 0

## Gaps
