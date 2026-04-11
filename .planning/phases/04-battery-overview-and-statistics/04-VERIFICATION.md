---
phase: 04-battery-overview-and-statistics
verified: 2026-04-11T10:10:00Z
status: human_needed
score: 4/4 must-haves verified
overrides_applied: 0
human_verification:
  - test: "Navigate to Battery section with inverter connected, verify per-channel voltage/current/power/SOC/SOH/cycles display and battery state shows human-readable label (Charging/Discharging/Sleeping/Fault/Loss reduction)"
    expected: "Each battery channel shows 10 data rows with correct units; State row shows text label not raw number"
    why_human: "Requires live Modbus TCP connection to inverter hardware and browser rendering"
  - test: "Navigate to BMS section, verify BMS Info group shows manufacturer, protocol version, cell type, total voltage, current, avg temp, SOC, SOH; verify bitmap grid shows colored cells for online packs"
    expected: "BMS Info card with labelled rows; Battery Topology card showing grid of green (online) and gray (offline) cells with tower labels, detected topology string, and color legend"
    why_human: "Requires live inverter connection and visual bitmap grid rendering verification"
  - test: "In BMS section, change topology dropdowns (Inputs/Towers/Packs) and reload page, verify values persist via localStorage"
    expected: "Dropdown values survive page refresh; configure message sends to server and BMS re-reads with new topology"
    why_human: "Requires browser interaction and localStorage state testing"
  - test: "Navigate to Statistics section, verify Today/Total/Month/Year groups each show Power Generation, Load Consumption, Grid Bought, Grid Sold, Battery Charge, Battery Discharge with kWh values"
    expected: "4 groups visible, each with 6 U32 energy metric rows, values formatted to 2 decimal places with 'kWh' unit"
    why_human: "Requires live inverter connection to confirm U32 register reads and scaling"
  - test: "With auto-refresh toggled OFF, navigate between sections; verify toggle state persists across navigation"
    expected: "Auto-refresh button remains inactive after navigating away and back; no spurious refresh messages sent"
    why_human: "Requires browser interaction to verify auto-refresh sync behavior added in plan 04"
---

# Phase 4: Battery Overview and Statistics Verification Report

**Phase Goal:** Users can view global battery status per channel, BMS summary info, online battery bitmap, configurable topology, and electricity generation/consumption statistics
**Verified:** 2026-04-11T10:10:00Z
**Status:** human_needed
**Re-verification:** No - initial verification

## Goal Achievement

### Observable Truths (Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | User can view per-channel battery voltage, current, power, SOC, SOH, cycles, and charge/discharge state with human-readable labels | VERIFIED | `GenerateBatteryGroups` generates 10 probes per channel (Voltage/Current/Power/EnvTemp/SOC/SOH/Cycles + Charge Limit/Discharge Limit/State); `BatteryStateEnum` maps values 1-5 to Charging/Discharging/Sleeping/Fault/Loss reduction; battery section registered in hub; frontend renders via `renderGroupedData` |
| 2 | User can view BMS global info (manufacturer, protocol, cell type, total voltage, current, SOC, SOH) and online battery bitmap showing which packs are online | VERIFIED | `BMSInfoGroups` has 19 probes including Manufacturer(ASCII), CAN Protocol Ver, Cell Type, Total Voltage, Total Current, SOC, SOH; Online Bitmap probe at 0x9022; `triggerBMSRead` builds `BitmapData` struct with per-tower online arrays; `renderBitmapGroup` renders colored 28px cells grid with tower labels and legend |
| 3 | User can configure battery topology (inputs 1-2, towers per input 1-4, packs per tower 4-10) with sensible defaults (1/2/10) | VERIFIED | CLI flags `-bat-inputs`/`-bat-towers`/`-bat-packs` with validation and defaults 1/2/10; `handleConfigure` clamps inputs(1-2)/towers(1-4)/packs(4-10); `initTopologyDropdowns` in frontend with localStorage persistence and `/api/defaults` fallback; configure message sent on change and section navigate |
| 4 | User can view daily and total electricity statistics: generation, consumption, bought, sold, battery charge, battery discharge | VERIFIED | `StatisticsGroups` returns 4 ProbeGroup definitions (Today/Total/Month/Year), each with 6 U32 probes (Power Generation, Load Consumption, Grid Bought, Grid Sold, Battery Charge, Battery Discharge) at correct Sofar register addresses; registered as "stats" section in hub; U32 FormatValue handles 32-bit big-endian Sofar word order with kWh scaling |

**Score:** 4/4 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/register/probe.go` | U32 bool field on Probe struct | VERIFIED | `U32 bool` field present, documented with constraint (Count must be 2) |
| `internal/register/probe_group.go` | Type string field on ProbeGroup | VERIFIED | `Type string` field present with "" / "bitmap" / "protection" dispatch documentation |
| `internal/register/format.go` | U32 FormatValue handling, DecodeBMSClock, DecodeTopology | VERIFIED | U32 path reads 4 bytes with Sofar word order (hi at low addr); DecodeBMSClock decodes packed bitfields; DecodeTopology returns parallelStrings/packsPerString |
| `internal/register/enum.go` | BatteryStateEnum with 5 entries | VERIFIED | 5 entries: Charging(1), Discharging(2), Sleeping(3), Fault(4), Loss reduction(5) |
| `internal/register/battery.go` | GenerateBatteryGroups, BMSInfoGroups, BMSProtectionProbes | VERIFIED | All three functions present; GenerateBatteryGroups has 10 probes/channel + Global Stats; BMSInfoGroups has 19 probes including 0x9022 Online Bitmap; BMSProtectionProbes has 6 protection/alarm registers |
| `internal/register/statistics.go` | StatisticsGroups with 4 groups x 6 U32 metrics | VERIFIED | New file with 4 groups (Today/Total/Month/Year) x 6 U32 metrics each at correct Sofar register addresses with correct scales |
| `internal/hub/message.go` | GroupData with Type/BitmapData, ConfigPayload with BatInputs/BatTowers/BatPacks | VERIFIED | `GroupData` has `Type string` and `Bitmap *BitmapData` fields; `BitmapData` struct with Towers/PacksPerTower/Online/DetectedTopology/Mismatch; `ConfigPayload` has BatInputs/BatTowers/BatPacks |
| `internal/hub/broker_iface.go` | WriteRegister on BrokerInterface | VERIFIED | `WriteRegister(ctx context.Context, addr uint16, value uint16) error` present |
| `internal/hub/hub.go` | Battery/BMS/stats section registration, triggerBMSRead, triggerBatteryRead, handleConfigure | VERIFIED | `registerBuiltinSections` registers battery/bms/stats; `triggerBatteryRead` with auto-detect from 0x066A; `triggerBMSRead` with standard 0x9022 read + topology detection + bitmap building; `handleConfigure` handles "bms" case with clamping |
| `cmd/server/main.go` | -bat-inputs/-bat-towers/-bat-packs CLI flags with validation | VERIFIED | All 3 flags present with default 1/2/10; validated against ranges before use; passed to `NewHub` and `DefaultsConfig` |
| `web/handler.go` | DefaultsConfig with BatInputs/BatTowers/BatPacks | VERIFIED | `DefaultsConfig` struct has `BatInputs int`, `BatTowers int`, `BatPacks int` JSON fields; served at `/api/defaults` |
| `web/static/index.html` | Battery/BMS/Statistics nav items, topology dropdowns | VERIFIED | Battery/BMS/Statistics nav buttons enabled; topology controls div with bat-inputs-select/bat-towers-select/bat-packs-select dropdowns |
| `web/static/app.js` | renderBitmapGroup, renderProtectionGroup, initTopologyDropdowns, sendTopologyConfigure | VERIFIED | All 4 functions present and substantive; type-based dispatch in renderGroupedData; auto-refresh sync after subscribe added in plan 04 |
| `web/static/style.css` | Bitmap grid CSS, topology controls CSS, custom properties | VERIFIED | --bitmap-online/--bitmap-offline/--bitmap-cell-border CSS custom properties; .bitmap-group/.bitmap-row/.bitmap-grid/.bitmap-cell/.bitmap-cell--online/.bitmap-cell--offline/.bitmap-legend CSS; .topology-controls/.topology-label/.topology-select CSS |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `statistics.go` | `hub.go` (stats section) | `register.StatisticsGroups()` in `registerBuiltinSections` | WIRED | `h.RegisterGroupedSection("stats", register.StatisticsGroups())` at line 934 |
| `battery.go` | `hub.go` (battery section) | `register.GenerateBatteryGroups(2)` in `registerBuiltinSections` | WIRED | `h.RegisterGroupedSection("battery", register.GenerateBatteryGroups(2))` at line 932 |
| `battery.go` | `hub.go` (bms section) | `register.BMSInfoGroups()` in `registerBMSSection` | WIRED | `groups := register.BMSInfoGroups()` at line 940 |
| `hub.go` BMS read | `broker.ReadBatch` | `triggerBMSRead` goroutine | WIRED | `results := h.broker.ReadBatch(h.ctx, reads)` with bitmap extraction from 0x9022 |
| `hub.go` battery read | `broker.ReadBatch` | `triggerBatteryRead` goroutine | WIRED | `results := h.broker.ReadBatch(h.ctx, reads)` with channel auto-detect from 0x066A |
| `hub.go` GroupData | frontend | WebSocket `section_data` with `groups` array | WIRED | `NewGroupedSectionData` returns message with `Groups []GroupData`; frontend `renderGroupedData` dispatches on `group.type` |
| `app.js` topology dropdowns | `hub.go` handleConfigure | WebSocket configure message type="bms" | WIRED | `App.ws.send({type: 'configure', section: 'bms', config: {...}})` in `sendTopologyConfigure`; `handleConfigure` handles "bms" case |
| `/api/defaults` | `app.js` initTopologyDropdowns | `fetch('/api/defaults')` | WIRED | `fetch('/api/defaults').then(data => { inputsSel.value = ... })` in `initTopologyDropdowns` |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|-------------------|--------|
| Battery section (app.js renderGroupedData) | `groups[]` from WebSocket message | `triggerBatteryRead` -> `broker.ReadBatch` -> `buildGroupedResult` -> `FormatValue` | Yes: broker.ReadBatch performs Modbus TCP reads; FormatValue applies scaling/units | FLOWING |
| BMS section bitmap (renderBitmapGroup) | `group.bitmap.online[]` | `triggerBMSRead` -> 0x9022 probe read -> `bitmapVal` extraction | Yes: reads 0x9022 register via standard probe batch | FLOWING |
| Statistics section (renderGroupedData) | `groups[]` U32 kWh values | `triggerSectionRead("stats")` -> `broker.ReadBatch` -> `buildGroupedResult` -> `FormatValue` U32 path | Yes: U32 FormatValue path reads 4 bytes from 2-register batch, applies scale | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| All 19 register tests for phase 04 pass | `go test ./internal/register/... -run "Battery\|BMS\|Statistic\|Decode\|FormatValueU32"` | 19 tests PASS | PASS |
| Hub disconnected subscribe error test | `go test ./internal/hub/... -run TestSubscribeWhileDisconnectedSendsError` | PASS (0.14s) | PASS |
| Hub auto-refresh toggle stops timer test | `go test ./internal/hub/... -run TestAutoRefreshToggleStopsTimer` | PASS (1.12s) | PASS |
| Full test suite (all packages) | `go test ./... -count=1` | All packages PASS; 0 failures | PASS |
| Binary builds successfully | `go build ./...` | BUILD OK | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| BAT-01 | 04-01, 04-02, 04-03 | View global battery info per channel: voltage, current, power, env temp, SOC, SOH, cycles | SATISFIED | GenerateBatteryGroups probes at 0x0604+7*(ch-1); rendered via grouped section data |
| BAT-02 | 04-01, 04-02, 04-03 | View battery state per channel with human-readable labels | SATISFIED | BatteryStateEnum maps 1-5; State probe at stateBase+2 with Enum field |
| BAT-03 | 04-01, 04-02, 04-03 | View charge/discharge limits, total charge/discharge power, average SOC, total capacity | SATISFIED | Global Stats group with 5 probes: 0x0667/0x0668/0x0669/0x066A/0x066B |
| BAT-04 | 04-01, 04-02, 04-03, 04-04 | View BMS global info: manufacturer, protocol version, cell type, total voltage, current, avg temp, SOC, SOH | SATISFIED | BMSInfoGroups: 0x9007(Manufacturer ASCII), 0x9006(CAN Protocol), 0x900C(Cell Type), 0x900F(Total Voltage), 0x9010(Current), 0x9011(Avg Cell Temp), 0x9012(SOC), 0x9013(SOH) |
| BAT-05 | 04-01, 04-02, 04-03, 04-04 | View online battery bitmap showing which packs are online | SATISFIED | Online Bitmap probe at 0x9022 in BMSInfoGroups; triggerBMSRead extracts and builds BitmapData; renderBitmapGroup renders per-pack cells with bitwise online check `(online[tower] >> pack) & 1` |
| BAT-06 | 04-02, 04-03, 04-04 | Configure battery topology: inputs (1-2), towers (1-4), packs (4-10), defaults 1/2/10 | SATISFIED | CLI flags with defaults; handleConfigure with clamping; topology dropdowns with localStorage + /api/defaults fallback |
| STAT-01 | 04-01, 04-02, 04-03 | View daily and total: power generation, load consumption | SATISFIED | StatisticsGroups: "Power Generation" and "Load Consumption" U32 probes in Today and Total groups |
| STAT-02 | 04-01, 04-02, 04-03 | View daily and total: power bought from grid, power sold to grid | SATISFIED | StatisticsGroups: "Grid Bought" and "Grid Sold" U32 probes in Today and Total groups |
| STAT-03 | 04-01, 04-02, 04-03 | View daily and total: battery charge, battery discharge | SATISFIED | StatisticsGroups: "Battery Charge" and "Battery Discharge" U32 probes in Today and Total groups |

No orphaned requirements: all 9 requirement IDs declared by phase plans (BAT-01 through BAT-06, STAT-01 through STAT-03) are accounted for and satisfied. REQUIREMENTS.md traceability table marks all 9 as Complete in Phase 4.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `internal/hub/hub.go` | 580 | `h.funcs <- func()` in goroutine without ctx guard (WR-01) | Warning | Potential goroutine leak on hub shutdown if battery auto-detect fires during shutdown; does not affect normal operation |
| `internal/hub/hub.go` | 140-173 | `h.ctx` used before `Run()` called (WR-02) | Warning | Nil pointer panic if ClientCount/RunFunc called before Run; unlikely in current usage but a robustness gap |
| `internal/hub/hub.go` | 708-710 | Silent `break` on results count mismatch (WR-03) | Warning | Could silently render empty groups on concurrent battery section rebuild; auto-detect path has a window for this |
| `web/static/app.js` | 806-816 | `parseInt` without NaN check in sendTopologyConfigure (WR-04) | Warning | Server-side clamping prevents harm; NaN serializes to null in JSON; silent substitution with minimum values |

No STUB or MISSING anti-patterns found. No `TODO`/`FIXME`/`placeholder` comments in phase 04 files. No empty implementations or hardcoded static returns in data paths.

### Human Verification Required

#### 1. Battery Section Live Rendering

**Test:** Navigate to Battery section with inverter connected; observe each channel card
**Expected:** Per-channel cards show Voltage (V), Current (A), Power (kW), Env Temp (°C), SOC (%), SOH (%), Cycles; State row shows text label (e.g. "Charging") not raw number; Global Stats shows charge/discharge power, average SOC, total capacity
**Why human:** Requires live Modbus TCP connection to inverter hardware and visual verification of grouped card rendering

#### 2. BMS Section Bitmap Grid

**Test:** Navigate to BMS section; inspect BMS Info card and Battery Topology card
**Expected:** BMS Info card shows rows for Manufacturer (ASCII text), CAN Protocol Ver, Cell Type, Total Voltage, Total Current, Avg Cell Temp, SOC, SOH, SW Version (composed string); Battery Topology card shows grid of colored 28px cells with green=online, gray=offline, tower row labels, detected topology string, optional mismatch warning, color legend
**Why human:** Requires live inverter connection and visual inspection of bitmap grid widget

#### 3. Topology Configuration Persistence

**Test:** In BMS section, change Inputs/Towers/Packs dropdowns; navigate away and back; reload page
**Expected:** Dropdown values persist across section navigation (localStorage); after page reload, values are restored from localStorage; configure message sent to server updates BMS read behavior
**Why human:** Requires browser interaction and localStorage state testing; configure effect requires hardware verification

#### 4. Statistics Section U32 Values

**Test:** Navigate to Statistics section; verify 4 groups visible with 6 rows each
**Expected:** Today/Total/Month/Year groups each show 6 rows: Power Generation, Load Consumption, Grid Bought, Grid Sold, Battery Charge, Battery Discharge with kWh values formatted to 2 decimal places
**Why human:** Requires live inverter connection to confirm U32 register reads and proper scaling from 32-bit Sofar word-order registers

#### 5. Auto-Refresh Toggle Cross-Section Persistence

**Test:** Toggle auto-refresh OFF; navigate from Battery to BMS to Statistics and back; verify toggle remains OFF
**Expected:** Auto-refresh button remains inactive across all navigations; no auto-refresh data arrives while OFF
**Why human:** Requires browser interaction to verify the auto-refresh sync fix added in plan 04 (f668ed7)

### Gaps Summary

No gaps found. All 4 success criteria are implemented with substantive, wired, and data-flowing code across all layers (register definitions, hub integration, frontend rendering). All 9 requirement IDs are satisfied. All 8 phase commits are present in git history. All tests pass (19 register tests + 3 hub tests for phase 04 functionality). The build succeeds.

The code review (04-REVIEW.md) identified 4 warnings (goroutine leak, nil-ctx risk, silent data loss, unvalidated parseInt) and 3 info items. None prevent the phase goal from being achieved. These are quality/resilience improvements for future attention.

Human verification is required to confirm live rendering with actual inverter hardware, bitmap grid visual correctness, localStorage persistence, and auto-refresh toggle behavior across sections.

---

_Verified: 2026-04-11T10:10:00Z_
_Verifier: Claude (gsd-verifier)_
