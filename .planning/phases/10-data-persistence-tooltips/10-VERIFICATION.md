---
phase: 10-data-persistence-tooltips
verified: 2026-04-12T18:30:00Z
status: human_needed
score: 10/10 must-haves verified
overrides_applied: 0
human_verification:
  - test: "Observe values dim to 50% opacity when a read cycle starts and progressively restore"
    expected: "All values in section dim on read_cycle send; each register_value arriving snaps its value back to full opacity; green flash fires on section_complete"
    why_human: "Opacity transitions are visual CSS effects — grep confirms classes and logic are wired, but the actual render behavior requires a live browser"
  - test: "Navigate System -> Grid -> System and verify cached values appear dimmed immediately on return"
    expected: "Returning to System shows last-read values at 50% opacity (no em-dash skeletons) before the fresh read cycle completes"
    why_human: "Section navigation caching requires a live browser with actual WebSocket data flowing; DOM state across section transitions is not verifiable statically"
  - test: "Hover over any parameter value for 300ms and verify tooltip content"
    expected: "Dark tooltip appears above the value showing 'Register: 0xNNNN', 'Raw: NNNNN', 'Last read: HH:MM:SS'; composed values (System time, System Clock, SW Version) show only 'Last read: HH:MM:SS'"
    why_human: "Tooltip rendering requires hover interaction in a live browser; the 300ms delay, positioning logic, and content format cannot be verified statically"
  - test: "Click Disconnect while viewing a section — verify all values reset to em-dash skeletons"
    expected: "All value elements show em-dash, sectionCache is cleared, tooltip hidden, no stale values visible"
    why_human: "Disconnect lifecycle requires a live connection and DOM state transition that cannot be verified without running the server"
  - test: "Drill into a battery pack, hover over a cell voltage — verify tooltip shows cell register address"
    expected: "Tooltip shows 'Register: 0x9051' through '0x9060' for individual cell voltages; Pack Info values show their individual register addresses"
    why_human: "Pack drill-down view requires inverter connection to receive pack_data message; tooltip on cell values is a live hover interaction"
---

# Phase 10: Data Persistence & Tooltips Verification Report

**Phase Goal:** Users always see the most recent known values and can inspect register-level details on demand
**Verified:** 2026-04-12T18:30:00Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | When a new refresh cycle begins, previously read values remain visible (dimmed/faded) until replaced by fresh values | VERIFIED (code) | `applyRefreshDimming()` adds `.content__body--refreshing` at all 3 read_cycle trigger points (lines 492, 506, 1255); `.data-row-h__value--fresh` with `opacity: 1 !important` restores each value on arrival; `handleSectionComplete` removes `--refreshing` class |
| 2 | Navigating away from a section and back shows the last-read values (dimmed) immediately, without waiting for a new read cycle | VERIFIED (code) | `sectionCache` Map stores per-register entries; `restoreFromCache()` called in `handleSectionSchema` after skeleton DOM build adds `--refreshing` class on restore; `getCacheKey()` supports composite keys for pack drill-down |
| 3 | Hovering over any parameter value shows a tooltip displaying the Modbus register address (hex) and the raw register value | VERIFIED (code) | `initTooltip()` sets event delegation on `#content-body` with `useCapture=true`; `showTooltip()` reads `data-register-addr`/`data-register-raw`/`data-register-time` attributes; `data-register-addr` set in `handleRegisterValue`, `renderGroupCard`, `renderCellVoltageGrid`, `handlePackData`; composed values pass `addr=0` so tooltip omits Register/Raw lines |

**Score:** 10/10 must-haves verified (automated)

Note: The 3 roadmap success criteria are all code-verified. Visual/interactive behavior requires human testing (Step 8).

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/hub/message.go` | RegisterValueMessage with RegisterAddr + RawValue; PackItemMeta struct; PackGroup ItemMeta + CellAddrs fields | VERIFIED | Lines 199-207: `RegisterAddr uint16 json:"register_addr"`, `RawValue string json:"raw_value,omitempty"`; Lines 110-113: `PackItemMeta`; Lines 122-125: `ItemMeta` and `CellAddrs` on PackGroup |
| `internal/register/format.go` | FormatRawValue function | VERIFIED | Lines 77-90: `func FormatRawValue(p Probe, data []byte) string` — returns decimal for numeric, hex for ASCII, empty for insufficient data |
| `internal/hub/hub_streaming.go` | 8 call sites with 7-arg NewRegisterValue, including FormatRawValue | VERIFIED | All 8 call sites confirmed with `p.Addr` and `rawVal`/`FormatRawValue(p, data)` args; composed values pass `0, ""` |
| `internal/hub/hub.go` | buildPackDataMessage populates ItemMeta for 5 groups | VERIFIED | Lines 850, 863, 883, 912-925, 942, 954, 971, 1007-1025: all 5 group builders (PackInfo, CellVoltages, Temperatures, PackStatus, Balance) set ItemMeta |
| `internal/hub/section.go` | FormatRawValue alias | VERIFIED | Lines 19-20: `var FormatRawValue = register.FormatRawValue` |
| `internal/register/register_test.go` | TestFormatRawValue with 6 subtests | VERIFIED | Line 1491: `func TestFormatRawValue` — 6 subtests (Uint16, Uint32, ASCII, EmptyData, SingleByte, Signed) all PASS |
| `internal/hub/hub_test.go` | TestNewRegisterValueJSON, TestNewRegisterValueComposedJSON, TestPackDataMessageItemMeta | VERIFIED | Lines 2023, 2066, 2081 — all 3 test functions exist and PASS |
| `web/static/style.css` | Refresh dimming CSS + tooltip CSS + custom properties | VERIFIED | Lines 110-120: 9 custom properties for tooltip and dimming; Lines 605-613: `.content__body--refreshing` and `.data-row-h__value--fresh`; Lines 1171-1202: `.param-tooltip`, `.param-tooltip::after`, `.param-tooltip--below::after` |
| `web/static/app.js` | sectionCache, getCacheKey, updateCache, restoreFromCache, applyRefreshDimming, initTooltip, showTooltip, hideTooltip; disconnect cleanup | VERIFIED | All 8 functions present at lines 33-86 (cache), 78-86 (dimming), 1008-1089 (tooltip); `sectionCache.clear()` at line 623 in disconnect handler; `initTooltip()` called at line 223 on DOMContentLoaded |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `hub_streaming.go` | `message.go` | `NewRegisterValue(..., p.Addr, rawVal)` | WIRED | All 8 call sites pass addr and rawVal; grep confirmed `NewRegisterValue.*p\.Addr` pattern present |
| `hub_streaming.go` | `register/format.go` | `FormatRawValue(p, data)` | WIRED | `FormatRawValue` called at 5 call sites for non-error, non-composed values |
| `hub.go buildPackDataMessage` | `message.go PackItemMeta` | Populates `ItemMeta` map with `PackItemMeta{RegisterAddr, RawValue}` | WIRED | Lines 863, 883, 954, 971, 1007, 1025 in hub.go reference `PackItemMeta` |
| `app.js handleRegisterValue` | `app.js sectionCache` | `updateCache(key, {...})` on each value | WIRED | Line 1218: `updateCache(key, {...})` called in handleRegisterValue |
| `app.js handleSectionSchema` | `app.js sectionCache` | `restoreFromCache(cacheKey)` after DOM build | WIRED | Lines 1138-1139: `restoreFromCache(getCacheKey())` called after `body.appendChild(container)` |
| `app.js disconnect handler` | `app.js sectionCache` | `sectionCache.clear()` on disconnect | WIRED | Line 623: `sectionCache.clear()` in `case 'disconnected':` block |
| `app.js handleRegisterValue` | `web/static/style.css` | `classList.add('content__body--refreshing')` and `classList.add('data-row-h__value--fresh')` | WIRED | Lines 1204, 1209: fresh class added; `--refreshing` applied via `applyRefreshDimming()` |
| `app.js renderGroupCard` | `app.js tooltip system` | Sets `data-register-addr`, `data-register-raw` from `group.item_meta` | WIRED | Lines 791-795: meta lookup and attribute set on `valEl` |
| `app.js renderCellVoltageGrid` | `app.js tooltip system` | `voltSpan` gets `data-row-h__value` class and `data-register-addr` / `data-register-raw` | WIRED | Lines 1983-1990: class `'cell-value data-row-h__value'` and data attributes |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|-------------------|--------|
| `web/static/app.js handleRegisterValue` | `msg.register_addr`, `msg.raw_value` | WebSocket `register_value` message from `hub_streaming.go` | Yes — `p.Addr` is compile-time constant from probe definitions; `rawVal = FormatRawValue(p, data)` from actual Modbus read | FLOWING |
| `web/static/app.js renderGroupCard` | `group.item_meta[key].register_addr` | `PackDataMessage.Groups[].ItemMeta` populated in `buildPackDataMessage` from probe definitions and `FormatRawValue` | Yes — `p.Addr` and `FormatRawValue(p, data)` from actual pack Modbus reads | FLOWING |
| `web/static/app.js renderCellVoltageGrid` | `group.cell_addrs[c]` | `PackGroup.CellAddrs` — static array `0x9051..0x9060` in `buildPackDataMessage` | Yes — addresses are compile-time constants; cell values from actual Modbus reads | FLOWING |
| `web/static/app.js restoreFromCache` | `sectionCache.get(cacheKey)` | `sectionCache` populated by `updateCache` in `handleRegisterValue` and `handlePackData` | Yes — cache entries originate from real WebSocket messages | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Full test suite green | `go test ./... -count=1 -timeout 60s` | All 5 packages pass | PASS |
| FormatRawValue subtests | `go test ./internal/register/ -run TestFormatRawValue -v` | 6/6 subtests PASS | PASS |
| RegisterValueMessage JSON tests | `go test ./internal/hub/ -run TestNewRegisterValue` | 2/2 PASS (register_addr present, raw_value omitted for composed) | PASS |
| PackDataMessage ItemMeta test | `go test ./internal/hub/ -run TestPackDataMessageItemMeta` | PASS (item_meta, register_addr, cell_addrs in JSON) | PASS |
| Go vet clean | `go vet ./...` | No warnings | PASS |
| Binary compiles with embedded frontend | `go build ./...` | No errors | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| DISP-01 | 10-02-PLAN | Previously read values persist dimmed when refresh begins | SATISFIED | `applyRefreshDimming()` applies `--refreshing` at all 3 read_cycle points; `--fresh` class restores each value; `--refreshing` removed on `section_complete` |
| DISP-02 | 10-02-PLAN | Browser caches values per section; navigating back shows cached dimmed values | SATISFIED | `sectionCache` Map wired to `updateCache` (per-register) and `restoreFromCache` (on schema); composite keys support pack drill-down; cleared on disconnect |
| DISP-03 | 10-01-PLAN, 10-02-PLAN, 10-03-PLAN | Hover tooltip with register address and raw value | SATISFIED | Backend sends `register_addr`/`raw_value` on all 8 streaming call sites; `PackDataMessage` carries `ItemMeta` + `CellAddrs`; frontend tooltip system sets data attributes from both message types and renders tooltip on hover with 300ms delay |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `internal/hub/message.go` | 225 | Comment text "placeholder slots" | Info | Intentional UI terminology for skeleton loading state; not a stub |
| `internal/hub/hub_streaming.go` | 14 | Comment text "placeholder slots" | Info | Same as above — documentation comment |

No blockers or warnings found. The two "Info" items are documentation comments describing the intended skeleton loading UX pattern, not incomplete implementations.

### Human Verification Required

#### 1. Refresh Dimming (DISP-01)

**Test:** Connect to inverter, navigate to System section, enable Auto-refresh. Observe each read cycle.
**Expected:** All values dim to ~50% opacity when each read cycle starts. As individual register values arrive, each one snaps back to full opacity. Green flash still fires when the cycle completes.
**Why human:** Opacity transitions are visual CSS effects. Code correctly applies `.content__body--refreshing` and `.data-row-h__value--fresh` classes, but actual render behavior can only be confirmed in a live browser with data flowing.

#### 2. Section Caching (DISP-02)

**Test:** Connect to inverter. View System section, wait for one complete read. Navigate to Grid section, wait for read. Navigate back to System.
**Expected:** System section shows last-read values immediately at ~50% opacity (no em-dash skeletons). Values are not blank while waiting for the fresh cycle to complete. Disconnect clears all values back to em-dashes.
**Why human:** Section navigation state with in-memory cache requires live browser and actual WebSocket data. DOM state between section transitions is not statically verifiable.

#### 3. Parameter Tooltips on Main Sections (DISP-03)

**Test:** While values are displayed in any section, hover over a parameter value for ~300ms.
**Expected:** Dark tooltip appears above the value showing: "Register: 0xNNNN" (hex address), "Raw: NNNNN" (decimal), "Last read: HH:MM:SS". Composed values (System time, System Clock, SW Version) show only "Last read: HH:MM:SS".
**Why human:** Tooltip is triggered by mouse hover. The 300ms delay, viewport positioning, flip-below behavior, and content format require a live browser interaction.

#### 4. Pack Drill-Down Tooltips (DISP-03, D-15)

**Test:** Navigate to BMS, drill into a battery pack. Hover over a cell voltage value and a temperature value.
**Expected:** Cell voltage tooltip shows "Register: 0x9051" through "0x9060" for the individual cell. Temperature values show their individual register addresses. Pack Info values (SOC, Total Voltage, etc.) show their register addresses.
**Why human:** Pack drill-down requires inverter connection to receive `pack_data` message. Cell voltage tooltips (via `data-row-h__value` class on `voltSpan`) require hover interaction.

### Gaps Summary

No automated gaps found. All code artifacts exist, are substantive, are wired, and have data flowing through them. The full test suite passes with zero regressions.

The 5 human verification items cover the visual/interactive requirements (DISP-01 opacity animations, DISP-02 navigation caching UX, DISP-03 hover tooltip interactions) that cannot be verified without running the application with a live inverter connection.

---

_Verified: 2026-04-12T18:30:00Z_
_Verifier: Claude (gsd-verifier)_
