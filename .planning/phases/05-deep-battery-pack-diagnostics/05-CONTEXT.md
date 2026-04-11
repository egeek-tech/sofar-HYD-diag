# Phase 5: Deep Battery Pack Diagnostics - Context

**Gathered:** 2026-04-11
**Status:** Ready for planning

<domain>
## Phase Boundary

Implement deep battery pack drill-down: hierarchical navigation (input → tower → pack), individual pack details (SN, voltage, SOC, current, capacity, cycles, cell count), 24 cell voltages with min/max/spread highlighting, pack temperatures (8 sensors + MOS + env), and alarm/protection/fault/balance bitmap decoding. This is the tool's killer feature — the primary reason users need this diagnostic tool.

Pack data retrieval requires writing 0x9020 (BMS_Inquire) via Modbus function 0x10 to select a specific pack, then reading 0x9044-0x907F for RT data. This write-read cycle was removed from the BMS overview in Phase 4 gap closure (caused timeouts) but is essential here for targeted pack inspection.

</domain>

<decisions>
## Implementation Decisions

### Pack Selection Mechanism
- **D-01:** Write 0x9020 via function 0x10 (NOT 0x06 — 0x06 times out on this inverter) with the pack's group byte encoding. Wait 1s settle time, then read 0x9044-0x907F RT data block. Retry once on timeout with 2s settle
- **D-02:** Pack selection is on-demand only — user clicks a specific pack, then the write-read cycle executes for that one pack. No automatic cycling through all packs (that would be too slow with 1s settle per pack)
- **D-03:** While pack data is loading (write + settle + read), show a loading indicator on the pack detail view. If write times out after retry, show error with "Pack may be offline — check BMS bitmap"

### Hierarchical Navigation UX
- **D-04:** Bitmap grid cells in BMS section become clickable in Phase 5 (per Phase 4 D-07). Clicking a cell navigates to that pack's detail view
- **D-05:** Breadcrumb navigation bar at top of pack detail view: "Battery > Input N > Tower M > Pack P" with each segment clickable to go back up the hierarchy
- **D-06:** Pack detail is a sub-view within the BMS section (not a separate sidebar section). User enters via bitmap click or dropdown selectors. Back button returns to BMS overview
- **D-07:** Dropdown selectors as alternative to bitmap click: [Input: 1-2] [Tower: 1-4] [Pack: 1-10] above the pack detail area. Constrained to configured topology values. Selecting a pack triggers the 0x9020 write-read cycle

### Cell Voltage Visualization
- **D-08:** 24 cell voltages displayed in a grid layout (6 columns x 4 rows). Each cell shows voltage value (e.g., "3.312V")
- **D-09:** Color-coded deviation from average: cells within 5mV of average are green, 5-20mV amber, >20mV red. This highlights imbalanced cells instantly
- **D-10:** Summary row above the grid showing: Min (cell#, value), Max (cell#, value), Spread (max-min), Average. Spread >30mV highlighted amber, >50mV red
- **D-11:** Min cell voltage from 0x906A and max from 0x9069 serve as verification against computed values

### Alarm/Protection/Fault/Balance State Decoding
- **D-12:** Protection info (0x9014-0x9015, 0x9122-0x9125) and fault info (0x9016-0x9017, 0x9126) displayed as decoded text list. Each bit maps to a specific alarm/protection/fault description
- **D-13:** Green "All Clear" indicator when all bitmap registers are zero. When bits are set: amber for warnings/alarms, red for protection trips and faults. Matches Phase 3 fault card visual pattern
- **D-14:** Balance state from 0x907A displayed as text: individual cell balance flags if available, or general "Balancing Active" / "Balanced" status
- **D-15:** Pack-level info block from 0x9104-0x9126 (BMS Pack Info area) included alongside RT data for complete diagnostic picture: design capacity, voltage, cycle count, etc.

### Temperature Display
- **D-16:** Pack temperatures shown as labeled rows: Temp 1-4 (0x906B-0x906E), Temp 5-8 (0x90BC-0x90BF), MOS (0x906F), Environment (0x9070)
- **D-17:** Color-coded ranges: green for normal operating range (15-45°C), amber for elevated (45-55°C), red for critical (>55°C or <0°C). Exact thresholds are Claude's discretion based on typical LiFePO4 limits
- **D-18:** Temperature values shown in °C with 0.1° resolution (raw value / 10, signed)

### Claude's Discretion
- Exact 0x9020 pack encoding: group byte layout (hi nibble = string index, lo nibble = pack index within string) — researcher verifies from protocol spec
- 0x9014-0x9017 protection/alarm bit-to-description mapping — researcher reads PDF section 5.10.1
- 0x9122-0x9126 pack-level protection/fault bit-to-description mapping — researcher reads PDF section 5.10.4
- Cell voltage grid responsive sizing (fixed vs fluid columns)
- Pack detail view layout arrangement (info block + cell grid + temps + alarms)
- Whether to include 0x9084-0x90FF fault data (historical fault snapshot) or only RT data — researcher assesses value

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Protocol Specification
- `Sofar_Inverter_MODBUS_V1.38_EN.pdf` — Key sections: 5.10.1 (BMS Info, 0x9020 query control), 5.10.2 (BMS Pack RT Data 0x9044-0x907F — cell voltages, temps, status), 5.10.3 (BMS Pack Fault Data 0x9084-0x90FF), 5.10.4 (BMS Pack Info 0x9104-0x9126 — design specs, protection bits)

### Memory Protocol Reference
- `.claude/projects/-data-git-private-modbus-reader/memory/reference_sofar_modbus_protocol.md` — Extracted register map. Section 5.10.2 has pack RT data layout (cell voltages at 0x9051-0x9068, temps at 0x906B-0x9070 + 0x90BC-0x90BF, min/max at 0x9069-0x906A)

### Existing Implementation (to extend)
- `internal/hub/hub.go` — triggerBMSRead (simplified in Phase 4 gap closure to remove 0x9020 writes). Pack drill-down needs a NEW triggerPackRead function that does the write-read cycle
- `internal/register/battery.go` — BMSInfoGroups, BMSProtectionProbes. Need new PackRTProbes, PackInfoProbes, PackAlarmProbes definitions
- `internal/hub/message.go` — GroupData, BitmapData. May need PackData struct or extend GroupData for cell grid rendering
- `internal/hub/section.go` — Section struct, flattenProbeGroups. Pack detail may need special handling (write before read)
- `web/static/app.js` — BMS section renderer, bitmap grid. Need click handler on grid cells, pack detail sub-view renderer, cell voltage grid renderer
- `internal/broker/broker.go` — WriteRegister (function 0x10). Verify it works; this is the critical path for pack selection

### Prior Phase Context
- `.planning/phases/04-battery-overview-and-statistics/04-CONTEXT.md` — D-07 (bitmap cells visual-only in P4, drill-down in P5), D-09 (read bitmap for all towers), D-13 (independent read cycles), D-14 (auto-detect topology from 0x900D)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `hub.triggerBMSRead` — Reference for how BMS read is structured; pack drill-down follows similar pattern but adds 0x9020 write step
- `hub.buildBMSGroupData` / `hub.buildProtectionGroup` — Pattern for building grouped data from probe results
- `register.BMSProtectionProbes` — Flat probe slice for bitmap decoding; extend pattern for pack-level protection/alarm/fault probes
- `broker.WriteRegister` — Already implemented for function 0x10. Used to exist in triggerBMSRead before Phase 4 gap closure. Re-enable for targeted pack selection
- Frontend bitmap grid renderer (`renderBitmapGroup` in app.js) — Add click handler to cells for drill-down navigation
- Frontend protection card renderer — Reuse pattern for pack alarm/fault display

### Established Patterns
- GroupData Type field for polymorphic rendering (bitmap, protection, standard) — extend with "pack_detail" or "cell_grid" type
- Configure message flow for topology dropdowns — pack selection can follow similar WS message pattern
- ProbeGroup-based section definitions with layout hints
- Server-side value formatting (backend formats all values to strings)
- Safe DOM rendering via createElement/textContent (no innerHTML)

### Integration Points
- `internal/hub/hub.go` — New triggerPackRead function, pack selection WS message handler
- `internal/register/battery.go` — PackRTProbes (0x9044-0x907F), PackInfoProbes (0x9104-0x9126), PackAlarmProbes
- `web/static/app.js` — Bitmap cell click handlers, pack detail sub-view, cell voltage grid, breadcrumb nav
- `web/static/style.css` — Cell voltage grid styling, color-coded deviation classes, breadcrumb nav CSS
- `web/static/index.html` — Minimal changes (pack detail is a sub-view within BMS section, not a new nav item)

</code_context>

<specifics>
## Specific Ideas

- Bitmap grid click-to-drill is the primary navigation — users see the topology overview, spot an interesting pack, click to inspect it
- Dropdown selectors provide keyboard-friendly alternative navigation for users who prefer precise pack selection
- Cell voltage color-coding based on deviation from average is the key diagnostic value — imbalanced cells are the #1 thing users look for
- 0x9020 write uses function 0x10 only (0x06 times out on this inverter, proven in original CLI tool testing)
- Pack detail is a sub-view within BMS, not a separate section — maintains the section navigation model established in earlier phases
- Phase 4 gap closure removed the 0x9020 write from BMS overview for reliability — Phase 5 re-introduces it only for targeted pack inspection (not cycling all packs)

</specifics>

<deferred>
## Deferred Ideas

- Cell voltage trend analysis over multiple reads (ADV-01 in v2 requirements)
- Battery cluster data from 0x9400+/0x9600+ direct read (EXT-04 in v2 requirements) — could supplement 0x9020-based pack data
- Historical fault snapshot display from 0x9084-0x90FF — researcher to assess value vs complexity

</deferred>

---

*Phase: 05-deep-battery-pack-diagnostics*
*Context gathered: 2026-04-11*
