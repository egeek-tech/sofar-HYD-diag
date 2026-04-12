---
status: complete
phase: 10-data-persistence-tooltips
source: [10-01-SUMMARY.md, 10-02-SUMMARY.md, 10-03-SUMMARY.md]
started: 2026-04-12T18:30:00Z
updated: 2026-04-12T18:45:00Z
---

## Current Test

[testing complete]

## Tests

### 1. Refresh Dimming on Auto-Refresh
expected: With auto-refresh enabled, when a new read cycle starts all parameter values dim to ~50% opacity. As each register value arrives, that value snaps back to full opacity. Green flash still fires on section_complete.
result: pass

### 2. Refresh Dimming on Manual Refresh
expected: Clicking the manual Refresh button causes the same dimming-to-restore behavior as auto-refresh. Values dim, then snap back one by one as they arrive.
result: pass

### 3. Section Cache on Navigation
expected: After reading System section values, navigate to Grid, then back to System. System values appear immediately (dimmed at 50%) from cache, then progressively refresh to full opacity as new values stream in.
result: pass

### 4. First Visit Shows Skeletons
expected: Navigate to a section you haven't visited yet in this session. It shows em-dash placeholder skeletons, not dimmed cached values. No dimming effect on first visit.
result: pass

### 5. Disconnect Clears Cache and Resets
expected: While viewing a section with values, click Disconnect. All values reset to em-dash skeletons immediately. Reconnect and navigate back -- no cached values from the previous connection, fresh skeletons shown.
result: pass

### 6. Tooltip on Main Section Value
expected: While connected with values displayed, hover over any parameter value (e.g., Inverter SN, Grid Voltage) for about 300ms. A dark tooltip appears above the value showing: "Register: 0xNNNN", "Raw: NNNN", "Last read: HH:MM:SS". Moving the mouse away hides it immediately.
result: pass

### 7. Tooltip on Composed Value
expected: Hover over a composed value like "System time". The tooltip shows only "Last read: HH:MM:SS" with no Register or Raw lines (since composed values span multiple registers).
result: pass

### 8. Tooltip Delay and Cancel
expected: Quickly move the mouse over a value and away in under 300ms. No tooltip appears. The tooltip only shows after holding the mouse still for ~300ms.
result: pass

### 9. Pack Drill-Down Tooltips
expected: Navigate to BMS, drill into a battery pack. Hover over a Pack Info value (e.g., SOC, Total Voltage). Tooltip shows the register address and raw value. Hover over a cell voltage in the grid -- tooltip shows the individual cell register (0x9051-0x9060) and raw millivolt value.
result: pass

### 10. Tooltip Dismissed on Section Navigation
expected: While a tooltip is visible, click a different section in the navigation. The tooltip disappears immediately without lingering.
result: pass

## Summary

total: 10
passed: 10
issues: 0
pending: 0
skipped: 0

## Gaps

[none]
