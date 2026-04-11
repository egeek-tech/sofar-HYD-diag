# Phase 5: Deep Battery Pack Diagnostics - Research

**Researched:** 2026-04-11
**Domain:** Modbus register reading with write-read cycle, cell-level battery diagnostics UI
**Confidence:** HIGH

## Summary

Phase 5 implements the tool's primary differentiator: hierarchical drill-down into individual battery packs showing cell voltages, temperatures, and fault/alarm states. The core technical challenge is the write-read cycle -- writing to 0x9020 (BMS_Inquire) via Modbus function 0x10 to select a specific pack, waiting for BMS settle time, then reading the pack's RT data from 0x9044-0x907F and additional info from 0x9104-0x9126.

The existing codebase provides all necessary infrastructure: the `broker.WriteRegister` function already implements function 0x10 writes through the command channel, the hub has established patterns for custom section read cycles (`triggerBMSRead`, `triggerBatteryRead`), and the frontend has a type-based widget dispatch system for rendering different group types (standard, bitmap, protection). The original CLI tool (`main.go.bak`) contains a proven, working implementation of the pack read cycle including the exact 0x9020 encoding and all register addresses, which serves as the authoritative reference for the web implementation.

The main implementation areas are: (1) a new `triggerPackRead` hub function implementing the write-settle-read cycle, (2) new probe definitions for pack RT data, pack info, and pack alarm registers, (3) a new WebSocket message type for pack selection, (4) frontend sub-view within the BMS section for pack detail rendering, and (5) cell voltage grid with deviation-based color coding.

**Primary recommendation:** Model the pack read as a new message type `select_pack` that triggers a dedicated `triggerPackRead` function in the hub. Use the proven 0x9020 encoding from `main.go.bak` (bits 0-7 = pack number, bits 8-11 = group/string number). Add new GroupData types `pack_detail`, `cell_grid`, and `pack_temps` for frontend widget dispatch.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Write 0x9020 via function 0x10 (NOT 0x06) with the pack's group byte encoding. Wait 1s settle time, then read 0x9044-0x907F RT data block. Retry once on timeout with 2s settle
- **D-02:** Pack selection is on-demand only -- user clicks a specific pack, then the write-read cycle executes for that one pack. No automatic cycling through all packs
- **D-03:** While pack data is loading, show a loading indicator on the pack detail view. If write times out after retry, show error with "Pack may be offline -- check BMS bitmap"
- **D-04:** Bitmap grid cells in BMS section become clickable in Phase 5. Clicking a cell navigates to that pack's detail view
- **D-05:** Breadcrumb navigation bar at top of pack detail view: "Battery > Input N > Tower M > Pack P" with each segment clickable to go back up the hierarchy
- **D-06:** Pack detail is a sub-view within the BMS section (not a separate sidebar section). User enters via bitmap click or dropdown selectors. Back button returns to BMS overview
- **D-07:** Dropdown selectors as alternative to bitmap click: [Input: 1-2] [Tower: 1-4] [Pack: 1-10] above the pack detail area. Constrained to configured topology values. Selecting a pack triggers the 0x9020 write-read cycle
- **D-08:** 24 cell voltages displayed in a grid layout (6 columns x 4 rows). Each cell shows voltage value (e.g., "3.312V")
- **D-09:** Color-coded deviation from average: cells within 5mV of average are green, 5-20mV amber, >20mV red
- **D-10:** Summary row above the grid showing: Min (cell#, value), Max (cell#, value), Spread (max-min), Average. Spread >30mV highlighted amber, >50mV red
- **D-11:** Min cell voltage from 0x906A and max from 0x9069 serve as verification against computed values
- **D-12:** Protection info (0x9014-0x9015, 0x9122-0x9125) and fault info (0x9016-0x9017, 0x9126) displayed as decoded text list. Each bit maps to a specific alarm/protection/fault description
- **D-13:** Green "All Clear" indicator when all bitmap registers are zero. When bits are set: amber for warnings/alarms, red for protection trips and faults. Matches Phase 3 fault card visual pattern
- **D-14:** Balance state from 0x907A displayed as text: individual cell balance flags if available, or general "Balancing Active" / "Balanced" status
- **D-15:** Pack-level info block from 0x9104-0x9126 (BMS Pack Info area) included alongside RT data for complete diagnostic picture
- **D-16:** Pack temperatures shown as labeled rows: Temp 1-4 (0x906B-0x906E), Temp 5-8 (0x90BC-0x90BF), MOS (0x906F), Environment (0x9070)
- **D-17:** Color-coded ranges: green for normal (15-45C), amber for elevated (45-55C), red for critical (>55C or <0C)
- **D-18:** Temperature values shown in C with 0.1 resolution (raw value / 10, signed)

### Claude's Discretion
- Exact 0x9020 pack encoding: group byte layout (hi nibble = string index, lo nibble = pack index within string)
- 0x9014-0x9017 protection/alarm bit-to-description mapping
- 0x9122-0x9126 pack-level protection/fault bit-to-description mapping
- Cell voltage grid responsive sizing (fixed vs fluid columns)
- Pack detail view layout arrangement (info block + cell grid + temps + alarms)
- Whether to include 0x9084-0x90FF fault data (historical fault snapshot) or only RT data

### Deferred Ideas (OUT OF SCOPE)
- Cell voltage trend analysis over multiple reads (ADV-01 in v2 requirements)
- Battery cluster data from 0x9400+/0x9600+ direct read (EXT-04 in v2 requirements)
- Historical fault snapshot display from 0x9084-0x90FF (researcher to assess value vs complexity)
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| BAT-07 | User can navigate hierarchically: select input > select tower > select pack to view details | D-04 bitmap click, D-05 breadcrumb, D-06 sub-view, D-07 dropdown selectors. WebSocket `select_pack` message type. Frontend pack detail sub-view architecture. |
| BAT-08 | User can view individual pack details: SN, total voltage, SOC, current, remaining/full capacity, cycles, cell count | Pack RT probes 0x9044-0x907F (ID, SN, voltage, SOC, current, caps, cycles). Pack Info probes 0x9104-0x9126 (SOH, design cap). 0x9020 write-read cycle. |
| BAT-09 | User can view 24 cell voltages per pack with min/max/spread highlighting | Cell voltages at 0x9051-0x9068 (24 x U16, scale 0.001V). Min/Max at 0x906A/0x9069. Frontend cell grid with deviation color coding (D-08 through D-11). |
| BAT-10 | User can view pack temperatures (up to 8 sensors + MOS temp + env temp) | Temps 1-4 at 0x906B-0x906E, Temps 5-8 at 0x90BC-0x90BF, MOS at 0x906F, Env at 0x9070. All S16 scale 0.1C. Color-coded ranges (D-16 through D-18). |
| BAT-11 | User can view pack alarm, protection, fault, and balance states with decoded bitmaps | RT alarm/protection/fault at 0x9076-0x9078, balance at 0x9075. Pack-level protection at 0x9124-0x9126. Bitmap decoding tables (D-12 through D-14). |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

- **Tech stack**: Go backend, vanilla HTML/JS/CSS frontend (embedded via Go embed). No frameworks.
- **Protocol**: Sofar Modbus-G3 protocol V1.38 -- register addresses and data types are fixed.
- **Hardware timing**: 500ms minimum delay between Modbus reads; BMS pack switch needs ~1s settle time.
- **Single connection**: Only one TCP connection to inverter at a time (Modbus is serial).
- **Deployment**: Single binary, no external dependencies.
- **DOM safety**: All dynamic rendering uses createElement/textContent, zero innerHTML (XSS prevention).
- **Error handling**: Explicit error checking on every operation with context wrapping.
- **Logging**: slog structured logging with component context.

## Standard Stack

This phase uses no new dependencies. Everything is built on the existing Go stdlib + project architecture.

### Core (existing, no additions)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `net` | Go 1.26.2 | TCP Modbus transport | Already in use, proven |
| Go stdlib `encoding/binary` | Go 1.26.2 | Register data parsing | Big-endian U16/S16 parsing |
| Go stdlib `log/slog` | Go 1.26.2 | Structured logging | Project convention |
| Vanilla JS/CSS | N/A | Frontend rendering | Project constraint: no frameworks |

[VERIFIED: go version command returned go1.26.2]

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Custom cell grid | CSS Grid | CSS Grid is the right tool -- `grid-template-columns: repeat(6, 1fr)` for the 6x4 layout |
| Separate pack detail route | Sub-view within BMS section | Decision D-06 locks this: pack detail is a sub-view, not a new section |

## Architecture Patterns

### Recommended Project Structure (new/modified files)
```
internal/
  register/
    battery.go        # ADD: PackRTProbes(), PackInfoProbes(), PackAlarmBitmap tables
  hub/
    hub.go            # ADD: triggerPackRead(), handleSelectPack(), pack read orchestration
    message.go        # ADD: MsgTypeSelectPack, PackSelectPayload, PackData struct, CellVoltageData
web/
  static/
    app.js            # ADD: pack detail sub-view renderer, cell grid renderer, breadcrumb nav, bitmap click handlers
    style.css         # ADD: cell voltage grid, deviation colors, breadcrumb nav, temp range colors, pack detail layout
```

### Pattern 1: Write-Read Cycle for Pack Selection
**What:** A new `triggerPackRead` function that writes 0x9020 to select a pack, waits for settle, then reads RT data and pack info.
**When to use:** Every time a user clicks a bitmap cell or selects a pack from dropdowns.
**How it differs from triggerBMSRead:** BMS read was simplified in Phase 4 gap closure to remove 0x9020 writes. Pack read re-introduces the write step, but only for the single requested pack.

```go
// Source: main.go.bak line 178 (proven working)
// 0x9020 encoding: bit0-7 = pack number (0-15), bit8-11 = group/string (0-15), bit12-15 = fault serial (0)
queryWord := uint16(packNum&0xFF) | uint16((groupNum&0x0F)<<8)
```
[VERIFIED: main.go.bak line 178 confirms this encoding]

**Sequence:**
1. Hub receives `select_pack` message with input, tower, pack
2. Convert topology coordinates to 0x9020 encoding (group = (input-1)*towers + (tower-1), pack = pack-1)
3. Call `broker.WriteRegister(ctx, 0x9020, queryWord)`
4. Wait 1s settle (D-01)
5. Call `broker.ReadBatch(ctx, packRTReads)` for 0x9044-0x907F
6. Call `broker.ReadBatch(ctx, packInfoReads)` for 0x9104-0x9126 and 0x90BC-0x90BF
7. Build pack detail GroupData and broadcast to subscriber
8. On timeout: retry once with 2s settle (D-01), then send error message (D-03)

### Pattern 2: Sub-View Navigation within BMS Section
**What:** The BMS section gets a "view mode" state: either "overview" (existing bitmap/info) or "pack_detail" (new drill-down).
**When to use:** When user clicks a bitmap cell or selects a pack from dropdowns.

The frontend manages this state client-side:
- `App.bmsViewMode = "overview" | "pack_detail"`
- `App.selectedPack = { input: N, tower: M, pack: P }`
- When `select_pack` response arrives, switch to pack_detail view
- When breadcrumb "BMS" is clicked, switch back to overview and re-subscribe to normal BMS data

### Pattern 3: Pack Detail GroupData Types
**What:** New GroupData types for polymorphic frontend rendering, following the established pattern.
**When to use:** When broadcasting pack read results.

| Type | Purpose | Frontend Widget |
|------|---------|-----------------|
| `pack_info` | SN, voltage, SOC, current, capacity, cycles | Standard key-value card |
| `cell_grid` | 24 cell voltages + min/max/spread | 6x4 colored grid with summary row |
| `pack_temps` | 10 temperature readings | Labeled rows with color coding |
| `pack_alarms` | Alarm/protection/fault/balance bitmaps | Decoded text list with severity colors |

### Pattern 4: New WebSocket Message Flow
**What:** A new inbound message type for pack selection, and corresponding outbound pack data.
**Inbound:**
```json
{
  "type": "select_pack",
  "section": "bms",
  "config": { "bat_input": 1, "bat_tower": 2, "bat_pack": 3 }
}
```
**Outbound:** Standard `section_data` message with `section: "bms"` and pack-specific groups (the frontend uses its view mode state to know whether to render overview or pack detail).

Alternative: Use a different section name like `"bms_pack"` to avoid overwriting the overview data. This is cleaner because auto-refresh of BMS overview won't clobber the pack detail view. **Recommendation: use `"bms_pack"` as a virtual section that shares the BMS subscription but has its own data path.** This avoids the complexity of the frontend distinguishing between two different data shapes on the same section name.

### Anti-Patterns to Avoid
- **Cycling all packs automatically:** D-02 explicitly forbids this. With 1s settle per pack and 20+ packs, a full cycle takes 20+ seconds. On-demand only.
- **Using function 0x06 for 0x9020 write:** This times out on the Sofar inverter. Always use function 0x10. [VERIFIED: main.go.bak line 479-480 documents this]
- **Blocking the hub event loop:** The write-settle-read must run in a goroutine, same as all other read functions. The settle delay (1-2s) is longer than normal inter-read delays.
- **innerHTML for cell grid rendering:** All DOM construction must use createElement/textContent per project convention.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Cell voltage statistics | Manual min/max/avg over 24 values | Server-side computation before sending to frontend | Backend already formats all values; keep logic server-side for consistency |
| Alarm bitmap decoding | Inline bit checks in handler | Data-driven bitmap table (like FaultTable in register/fault.go) | Maintainable, testable, follows existing pattern |
| Temperature color ranges | Hardcoded CSS per value | CSS classes applied based on computed range | Matches existing pattern (online/offline bitmap cells) |
| Pack coordinate encoding | Ad-hoc bit manipulation | Named function `EncodePackQuery(input, tower, pack, topology)` | Encoding logic is non-obvious; a named function is self-documenting and testable |

## 0x9020 Pack Query Encoding (Claude's Discretion Resolution)

From the protocol spec at 0x9020 [VERIFIED: memory reference section 5.10.1]:
```
bit0-7:  pack serial number (0-15)
bit8-11: battery group/string number (0-15)  
bit12-15: fault serial (0-5), use 0 for RT data
```

From the proven CLI tool [VERIFIED: main.go.bak line 178]:
```go
queryWord := uint16(packNum & 0xFF) | uint16((groupNum & 0x0F) << 8)
```

**Topology coordinate mapping:**
- The user selects: Input N (1-2), Tower M (1-4), Pack P (1-10)
- The group/string number in 0x9020 = (input-1) * towersPerInput + (tower-1)
  - For 1 input, 2 towers: Tower 1 = group 0, Tower 2 = group 1
  - For 2 inputs, 2 towers each: Input 1 Tower 1 = group 0, Input 1 Tower 2 = group 1, Input 2 Tower 1 = group 2, etc.
- The pack number = pack - 1 (0-indexed in protocol, 1-indexed in UI)

Example: Input 1, Tower 2, Pack 5 with 2 towers/input:
- group = (1-1)*2 + (2-1) = 1
- pack = 5-1 = 4
- queryWord = 4 | (1 << 8) = 0x0104

[VERIFIED: Protocol reference says bit0-7=pack(0-15), bit8-11=group(0-15)]

## Protection/Alarm/Fault Bitmap Decoding (Claude's Discretion Resolution)

### BMS Global Level (0x9014-0x9017, already read in BMS overview)

Based on Sofar Modbus-G3 V1.38 protocol, BMS protection and alarm registers follow standard Sofar BMS bitmap conventions. The exact bit-to-description mapping is not fully enumerated in the memory reference but follows this general structure from the protocol PDF sections:

**0x9014 (Protection Info 0):** [ASSUMED]
- Bit 0: Cell OV Protection
- Bit 1: Cell UV Protection
- Bit 2: Pack OV Protection
- Bit 3: Pack UV Protection
- Bit 4: Charge OT Protection
- Bit 5: Charge UT Protection
- Bit 6: Discharge OT Protection
- Bit 7: Discharge UT Protection
- Bit 8: Charge OC Protection
- Bit 9: Discharge OC Protection
- Bit 10: Short Circuit Protection
- Bit 11: IC Fault Protection
- Bit 12: MOS OT Protection
- Bit 13-15: Reserved

**0x9015 (Protection Info 1):** [ASSUMED]
- Bit 0: Cell voltage diff too large
- Bit 1: Temperature diff too large
- Bit 2: Charging lockout (cell OV)
- Bit 3: Discharging lockout (cell UV)
- Bits 4-15: Reserved/model-specific

**0x9016 (Alarm Info 0):** [ASSUMED]
- Bit 0: Cell OV Alarm
- Bit 1: Cell UV Alarm
- Bit 2: Pack OV Alarm
- Bit 3: Pack UV Alarm
- Bit 4: Charge OT Alarm
- Bit 5: Charge UT Alarm
- Bit 6: Discharge OT Alarm
- Bit 7: Discharge UT Alarm
- Bit 8: Charge OC Alarm
- Bit 9: Discharge OC Alarm
- Bits 10-15: Reserved

**0x9017 (Alarm Info 1):** [ASSUMED]
- Bit 0: Voltage diff alarm
- Bit 1: Temperature diff alarm
- Bits 2-15: Reserved/model-specific

### Pack RT Level (0x9076-0x9078, from 0x9044-0x907F block)

These have the same bitmap layout as the global BMS registers but apply to the individual pack:
- 0x9076: Pack Alarm Status (same layout as 0x9016)
- 0x9077: Pack Protection Status (same layout as 0x9014)
- 0x9078: Pack Fault Status (general fault bits)

### Pack Info Level (0x9124-0x9126, from 0x9104-0x9126 block)

- 0x9124: Alarm Status 2 (extended alarms)
- 0x9125: Protection Status 2 (extended protections)
- 0x9126: Fault Status 2 (extended faults)

**Implementation recommendation:** Create a data-driven `BMSAlarmTable` and `BMSProtectionTable` mapping bit positions to descriptions, following the pattern of `register.FaultTable` from fault.go. Since the exact bit definitions are ASSUMED, make the tables easy to update. Display as hex value alongside decoded text so the user can verify raw values even if some bits are undocumented.

### Balance State (0x9075)

From protocol spec [VERIFIED: memory reference 0x9075]:
```
0x9075: Equilibrium state (16 cells) - bit N=1 means cell N is balancing
```

Display: Check each bit 0-15. If any bit is set, show "Balancing: Cell N, Cell M, ...". If all zero, show "Balanced".

## Historical Fault Data Assessment (Claude's Discretion Resolution)

**Decision: Exclude 0x9084-0x90FF from Phase 5.** Rationale:
1. The historical fault snapshot (0x9084-0x90FF) captures the state at the time of the last fault, not current state
2. It duplicates much of the RT data structure (same register layout shifted by 0x40)
3. Including it doubles the read time after pack selection (already 10+ seconds for RT + Pack Info)
4. The user deferred this explicitly in the CONTEXT.md deferred section
5. RT alarm/protection/fault registers (0x9076-0x9078) already show current fault state

If users need fault history, it can be added later as a "Fault Snapshot" tab within the pack detail view.

## Register Read Strategy

### Pack RT Data Block (0x9044-0x907F = 60 registers)
This is a contiguous block within the Modbus 60-register per-read limit. Read in one batch:

```
ReadRequest{Addr: 0x9044, Count: 60}  // 0x9044 through 0x907F
```

This single read captures: Pack ID, timestamp, SN, all 24 cell voltages, max/min cell voltage, temps 1-4, MOS temp, env temp, current, remaining/full capacity, cycles, balance state, alarm/protection/fault status, total voltage, SOC, total packs, cell strings.

[VERIFIED: Protocol spec section 5.10.2 confirms 0x9044-0x907F is the Pack RT Data block, protocol max 60 regs per read]

### Pack Info Block (0x9104-0x9126 = 35 registers)
Second read for extended pack information:

```
ReadRequest{Addr: 0x9104, Count: 35}  // 0x9104 through 0x9126
```

This captures: balanced bus voltage/current, manufacturer, SOH, rated capacity, cell temps 1-16, lifetime discharge/charge Ah/Wh, alarm2/protection2/fault2.

### Temps 5-8 (0x90BC-0x90BF = 4 registers)
These are in the "fault data" address range but are actually RT temps 5-8 [VERIFIED: memory reference shows "0x90BC-0x90BF | RT temps 5-8 | S16 | 0.1C"]. Third read:

```
ReadRequest{Addr: 0x90BC, Count: 4}  // Temps 5-8
```

### Total Reads After Pack Selection
1. Write 0x9020 (1 operation)
2. Wait 1s settle
3. Read 0x9044-0x907F (1 batch read, 60 registers)
4. Read 0x9104-0x9126 (1 batch read, 35 registers)
5. Read 0x90BC-0x90BF (1 batch read, 4 registers)

With 500ms inter-read delay: ~1s (settle) + ~2s (3 reads + delays) = ~3s total. Acceptable for on-demand user action.

## Common Pitfalls

### Pitfall 1: 0x9020 Write Using Wrong Function Code
**What goes wrong:** Writing 0x9020 using function 0x06 (Write Single Register) times out on the Sofar inverter.
**Why it happens:** The protocol spec says 0x9020 is RW, but 0x06 does not work for this register on this inverter model.
**How to avoid:** Always use function 0x10 (Write Multiple Registers). The existing `broker.WriteRegister` already routes to `modbus.WriteMultipleRegistersTCP` for TCP mode.
**Warning signs:** Timeout errors on pack selection write.
[VERIFIED: main.go.bak line 479-480 documents this explicitly]

### Pitfall 2: BMS Settle Time Race
**What goes wrong:** Reading pack data immediately after 0x9020 write returns stale data from the previously selected pack.
**Why it happens:** The BMS needs time to switch internal addressing to the new pack. 1s is the minimum.
**How to avoid:** Use `time.Sleep(1 * time.Second)` between write and first read. On timeout, retry with 2s settle per D-01.
**Warning signs:** Pack RT ID (0x9044) returns a different pack number than requested.

### Pitfall 3: Pack Coordinate Encoding Off-by-One
**What goes wrong:** Requesting pack 1 in the UI reads pack 0 in the protocol, but the offset is applied incorrectly.
**Why it happens:** UI uses 1-indexed (Pack 1-10, Tower 1-4, Input 1-2) but protocol uses 0-indexed (pack 0-9, group 0-7).
**How to avoid:** Single named function `EncodePackQuery(input, tower, pack, towersPerInput)` that encapsulates all offset logic. Unit test with known values.
**Warning signs:** Wrong pack data returned; pack ID at 0x9044 doesn't match expected encoding.

### Pitfall 4: Section Data Clobbering
**What goes wrong:** Auto-refresh of BMS overview data overwrites the pack detail view.
**Why it happens:** Both BMS overview and pack detail send `section_data` for section "bms". If the user is viewing pack detail and an auto-refresh fires, the overview data replaces the pack detail.
**How to avoid:** Either (a) use a distinct virtual section name like `"bms_pack"` for pack data, or (b) pause BMS auto-refresh while viewing pack detail. Option (a) is cleaner.
**Warning signs:** Pack detail view suddenly replaced by BMS overview grid.

### Pitfall 5: Cell Voltage Scale Confusion
**What goes wrong:** Cell voltages display incorrectly (e.g., 3312mV instead of 3.312V).
**Why it happens:** Cell voltages at 0x9051-0x9068 use scale 0.001V (millivolt resolution), different from pack total voltage at 0x9079 which uses 0.1V.
**How to avoid:** Probe definitions must use `Scale: 0.001` for cell voltages. The server-side formatting handles the rest via `FormatValue`.
**Warning signs:** Cell voltages showing 3312 instead of 3.312.
[VERIFIED: Protocol spec section 5.10.2 confirms 0x9051-0x9068 are U16 with 0.001V]

### Pitfall 6: Signed vs Unsigned Temperature Values
**What goes wrong:** Temperatures below zero display as large positive numbers.
**Why it happens:** Temperature registers are S16 (signed) but read as U16 by default.
**How to avoid:** All temperature probes must have `Signed: true` in their definitions.
**Warning signs:** Temperatures showing 65526 instead of -10.
[VERIFIED: Protocol spec section 5.10.2 confirms temps are S16 0.1C]

## Code Examples

### Pack RT Probe Definitions
```go
// Source: Sofar Modbus-G3 V1.38 section 5.10.2 (0x9044-0x907F)
// [VERIFIED: memory reference + main.go.bak lines 215-232]

func PackRTProbes() []Probe {
    return []Probe{
        {Name: "Pack ID", Addr: 0x9044, Count: 1},
        {Name: "Serial Number", Addr: 0x9047, Count: 10, IsASCII: true},
        {Name: "Total Voltage", Addr: 0x9079, Count: 1, Unit: "V", Scale: 0.1},
        {Name: "SOC", Addr: 0x907A, Count: 1, Unit: "%", Scale: 1},
        {Name: "Current", Addr: 0x9071, Count: 1, Signed: true, Unit: "A", Scale: 0.1},
        {Name: "Remaining Capacity", Addr: 0x9072, Count: 1, Unit: "Ah", Scale: 0.1},
        {Name: "Full Charge Capacity", Addr: 0x9073, Count: 1, Unit: "Ah", Scale: 0.1},
        {Name: "Cycle Count", Addr: 0x9074, Count: 1, Unit: "cycles", Scale: 1},
        {Name: "Cell Count", Addr: 0x907C, Count: 1},
        {Name: "Total Packs", Addr: 0x907B, Count: 1},
    }
}
```

### Cell Voltage Batch Read
```go
// Source: Sofar Modbus-G3 V1.38 section 5.10.2
// [VERIFIED: 0x9051-0x9068 = 24 registers, each U16, scale 0.001V]

// Read all 24 cell voltages in a single batch (contiguous registers)
cellRead := broker.ReadRequest{Addr: 0x9051, Count: 24}
// Parse: for i := 0; i < 24; i++ { voltage := float64(binary.BigEndian.Uint16(data[i*2:i*2+2])) * 0.001 }
```

### 0x9020 Encoding Function
```go
// Source: main.go.bak line 178 + protocol spec
// [VERIFIED: proven working in CLI tool]

// EncodePackQuery converts UI topology coordinates to the 0x9020 register value.
// input: 1-based battery input (1-2)
// tower: 1-based tower within input (1-4)
// pack: 1-based pack within tower (1-10)
// towersPerInput: configured towers per input (1-4)
func EncodePackQuery(input, tower, pack, towersPerInput int) uint16 {
    group := (input-1)*towersPerInput + (tower - 1) // 0-indexed group/string
    packIdx := pack - 1                              // 0-indexed pack
    return uint16(packIdx&0xFF) | uint16((group&0x0F)<<8)
}
```

### Frontend Cell Voltage Grid (createElement pattern)
```javascript
// Source: project convention (createElement/textContent, no innerHTML)
// [VERIFIED: app.js bitmap grid uses same pattern]

function renderCellGrid(cells, minV, maxV) {
    var grid = document.createElement('div');
    grid.className = 'cell-grid';
    grid.style.gridTemplateColumns = 'repeat(6, 1fr)';
    
    var avg = cells.reduce(function(s, c) { return s + c.voltage; }, 0) / cells.length;
    
    for (var i = 0; i < cells.length; i++) {
        var cell = document.createElement('div');
        var dev = Math.abs(cells[i].voltage - avg) * 1000; // deviation in mV
        var cls = 'cell-voltage';
        if (dev <= 5) cls += ' cell-voltage--good';
        else if (dev <= 20) cls += ' cell-voltage--warn';
        else cls += ' cell-voltage--bad';
        cell.className = cls;
        cell.textContent = cells[i].voltage.toFixed(3) + 'V';
        grid.appendChild(cell);
    }
    return grid;
}
```

### Temperature Color Coding
```css
/* Source: D-17 temperature thresholds (Claude's discretion, LiFePO4 typical ranges) */
/* [ASSUMED: thresholds based on typical LiFePO4 battery safe operating ranges] */

.temp-value--normal { color: var(--color-success-text); }   /* 15-45C */
.temp-value--elevated { color: var(--color-warning-text); }  /* 45-55C or 0-15C */
.temp-value--critical { color: var(--color-error-text); }    /* >55C or <0C */
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Full pack cycle in BMS read | On-demand single pack read | Phase 4 gap closure | Removed timeout-prone cycling; Phase 5 adds targeted read |
| Flat probe slices | ProbeGroup-based sections | Phase 3 | Pack detail uses grouped layout with type dispatch |
| innerHTML rendering | createElement/textContent | Phase 2 (T-02-11) | All new widgets must follow this pattern |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | 0x9014-0x9017 protection/alarm bit definitions follow standard Sofar BMS bitmap layout | Protection/Alarm Bitmap Decoding | Low -- display hex alongside decoded text, user can verify. Easy to update mapping table |
| A2 | 0x9124-0x9126 pack-level protection/alarm/fault bitmaps exist and follow similar layout | Protection/Alarm Bitmap Decoding | Medium -- registers may return zero or different layout. Graceful fallback: show hex value |
| A3 | LiFePO4 safe temperature thresholds: 15-45C normal, 45-55C elevated, >55C/<0C critical | Temperature Color Coding | Low -- thresholds are configurable CSS classes, easily adjusted |
| A4 | 0x9104-0x9126 Pack Info block is populated after 0x9020 write (same as RT block) | Register Read Strategy | Medium -- if Pack Info requires a different query, reads may return stale/zero data. Verify on first integration test |

## Open Questions

1. **Pack Info block availability**
   - What we know: The protocol spec lists 0x9104-0x9126 as "BMS Pack Info" with design capacity, SOH, cell temps 1-16, lifetime stats
   - What's unclear: Whether this block is populated by the same 0x9020 write that populates the RT block (0x9044-0x907F)
   - Recommendation: Read it after the RT block in the same cycle. If data returns zero for all registers, it may need a separate query or may not be supported by the BMS firmware. Handle gracefully by hiding empty groups.

2. **Exact protection/alarm bit definitions**
   - What we know: Standard Sofar BMS uses typical protection bitmaps (OV, UV, OT, UT, OC, SC)
   - What's unclear: Exact bit-to-description mapping for 0x9014-0x9017 and 0x9124-0x9126 without reading the PDF section 5.10.1 directly
   - Recommendation: Implement with assumed standard mappings, display hex value alongside decoded text so users can cross-reference with protocol documentation. The tables are trivially updatable.

3. **Cell count variability**
   - What we know: Protocol supports up to 24 cells per pack (registers 0x9051-0x9068). Register 0x907C reports actual cell string count.
   - What's unclear: Whether packs with fewer than 24 cells return zero for unused cell registers or return different data
   - Recommendation: Check 0x907C first. Only display cells up to the reported count. Skip zero-value cells at the end of the 24-register block.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (stdlib) |
| Config file | None (stdlib) |
| Quick run command | `go test ./internal/register/... -run Pack -count=1 -v` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| BAT-07 | Pack selection encoding, topology coordinate conversion | unit | `go test ./internal/register/... -run TestEncodePackQuery -v` | No -- Wave 0 |
| BAT-08 | Pack RT probe definitions cover all required registers | unit | `go test ./internal/register/... -run TestPackRTProbes -v` | No -- Wave 0 |
| BAT-09 | Cell voltage statistics computation (min/max/spread/avg) | unit | `go test ./internal/hub/... -run TestCellVoltageStats -v` | No -- Wave 0 |
| BAT-10 | Temperature probe definitions, signed value formatting | unit | `go test ./internal/register/... -run TestPackTempProbes -v` | No -- Wave 0 |
| BAT-11 | Alarm/protection bitmap decoding with known values | unit | `go test ./internal/register/... -run TestBMSAlarmDecode -v` | No -- Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/register/... ./internal/hub/... -count=1`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/register/battery_test.go` -- test EncodePackQuery, PackRTProbes coverage, alarm decode tables
- [ ] `internal/hub/hub_test.go` additions -- test triggerPackRead mock scenarios (write success, write timeout, retry)
- [ ] `internal/register/format_test.go` additions -- test cell voltage formatting at 0.001 scale

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | No | Local network tool, no auth |
| V3 Session Management | No | Stateless WebSocket |
| V4 Access Control | No | Single-user tool |
| V5 Input Validation | Yes | Validate pack selection coordinates (input/tower/pack bounds) server-side before 0x9020 write |
| V6 Cryptography | No | No crypto needed |

### Known Threat Patterns for This Phase

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Malformed pack selection coordinates | Tampering | Server-side clamping of input/tower/pack to valid ranges before encoding 0x9020 value (same pattern as PV channel clamping) |
| XSS via cell voltage values | Tampering | createElement/textContent only, no innerHTML (existing project convention) |
| Denial of service via rapid pack selection | Denial of Service | Debounce/ignore pack selection while a read is in progress (use `reading` atomic flag from Section struct) |

## Sources

### Primary (HIGH confidence)
- Memory protocol reference (`reference_sofar_modbus_protocol.md`) -- full register map for 0x9020, 0x9044-0x907F, 0x9104-0x9126, 0x90BC-0x90BF
- `main.go.bak` lines 116-294 -- proven working CLI implementation of pack read cycle
- `internal/hub/hub.go` -- existing triggerBMSRead, triggerBatteryRead, buildProtectionGroup patterns
- `internal/register/battery.go` -- existing BMSInfoGroups, BMSProtectionProbes patterns
- `internal/hub/message.go` -- existing GroupData, BitmapData, OutboundMessage structures
- `internal/broker/broker.go` -- existing WriteRegister implementation
- `web/static/app.js` -- existing bitmap grid renderer, protection group renderer, section data handler

### Secondary (MEDIUM confidence)
- 0x9020 encoding format cross-verified between protocol spec and working CLI tool

### Tertiary (LOW confidence)
- 0x9014-0x9017 and 0x9124-0x9126 bit-to-description mappings (assumed standard Sofar BMS layout, not verified against PDF section 5.10.1)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- no new dependencies, all existing infrastructure
- Architecture: HIGH -- follows established hub/section/probe/GroupData patterns with clear extension points
- Register addresses: HIGH -- verified against protocol spec and working CLI tool
- 0x9020 encoding: HIGH -- verified from working CLI code and protocol spec
- Protection bitmap decoding: LOW -- assumed standard layout, needs PDF verification or runtime validation
- Pack Info block availability: MEDIUM -- spec says it exists, unclear if same 0x9020 write populates it

**Research date:** 2026-04-11
**Valid until:** 2026-05-11 (stable protocol, no moving parts)
