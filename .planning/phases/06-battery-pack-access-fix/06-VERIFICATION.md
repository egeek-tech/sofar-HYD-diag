---
phase: 06-battery-pack-access-fix
verified: 2026-04-11T19:35:00Z
status: human_needed
score: 4/4 must-haves verified
overrides_applied: 0
human_verification:
  - test: "Navigate to BMS overview with live inverter connected, verify both tower rows show distinct online/offline bitmaps"
    expected: "Tower 1 and Tower 2 rows display different pack-online patterns (or same if both have identical online packs), NOT both showing identical stale data"
    why_human: "Bitmap cycling correctness against real hardware requires a live Modbus TCP connection to verify 0x9020 group-write settle behavior at 500ms. Cannot verify with unit tests alone."
  - test: "Click any pack cell in the bitmap grid (both towers, multiple packs) and verify pack detail view loads with 16 cell voltages"
    expected: "Pack detail view shows Cell 1 through Cell 16 (not 17-24). Each shows a voltage reading. Tower 1 and Tower 2 packs both navigable (10 each = 20 total)."
    why_human: "UI rendering and live data retrieval for all 20 packs requires hardware interaction. Automated tests mock the broker but cannot verify actual 0x9020 encoding behavior against physical inverter."
---

# Phase 6: Battery Pack Access Fix Verification Report

**Phase Goal:** All 20 battery packs are accessible for drill-down, matching the proven CLI tool behavior
**Verified:** 2026-04-11T19:35:00Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | User can navigate to any of the 20 battery packs (2 towers x 10 packs) and see cell-level data | ✓ VERIFIED | `handleSelectPack` clamps input=1, tower 1-2, pack 1-10 using `TopoTowers`/`TopoPacksPerTower`. `triggerPackRead` uses `EncodePackQuery` with constant `TopoTowers`. Frontend `handleBitmapCellClick` hardcodes `input=1`, `tower=towerIndex+1`. All 20 pack coordinates reachable. `TestHandleSelectPack` passes. |
| 2 | Pack selection writes the correct tower/pack encoding to 0x9020, matching the proven CLI tool | ✓ VERIFIED | `EncodePackQuery(input, tower, pack, towersPerInput)` implements `packIdx | (group << 8)` matching CLI `main.go.bak` line 178. `triggerPackRead` calls it with `towersPerInput = TopoTowers`. `TestEncodePackQuery` and `TestEncodePackQueryInHandler` pass. |
| 3 | Topology is fixed at 16 cells/pack, 10 packs/tower, 2 towers — no configuration dropdowns for these values | ✓ VERIFIED | Constants `TopoTowers=2`, `TopoPacksPerTower=10`, `TopoCellsPerPack=16` in `hub.go`. No `defaultBatInputs`/`defaultBatTowers`/`defaultBatPacks` fields remain. `NewHub` takes 3 params (no bat params). `PackRTProbes` loops `for i := 0; i < 16`. HTML has no `#topology-controls`. JS has `TOPO_TOWERS=2`/`TOPO_PACKS=10`, no `BAT_INPUTS_KEY`. CSS `.cell-grid` uses `repeat(4, 1fr)`. `TestTopologyConstants` passes. |
| 4 | Online bitmap correctly reflects all packs that the inverter reports as available | ✓ VERIFIED | `triggerBMSRead` Step 3 cycles per tower: writes `uint16(t) << 8` to `0x9020`, sleeps 500ms, reads `0x9022` for that tower's true bitmap. `BitmapData.Online` is populated with distinct per-tower values. `TestBMSBitmapCycling` verifies writes `0x0000`/`0x0100`, reads distinct `0x03FF`/`0x001F`. `TestBMSBitmapCyclingWriteFailure` verifies graceful degradation. |

**Score:** 4/4 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/hub/hub.go` | Topology constants and simplified Hub struct | ✓ VERIFIED | Contains `TopoTowers=2`, `TopoPacksPerTower=10`, `TopoCellsPerPack=16`. `NewHub` signature is `func NewHub(b BrokerInterface, logger *slog.Logger, pvChannels int) *Hub`. No `defaultBat*` fields. `handleConfigure` has no `case "bms"`. |
| `internal/register/battery.go` | Reduced cell probe list | ✓ VERIFIED | `for i := 0; i < 16; i++` with comment "16 cell voltages: 0x9051-0x9060". |
| `cmd/server/main.go` | CLI without topology flags | ✓ VERIFIED | No `bat-inputs`/`bat-towers`/`bat-packs` flags. `NewHub` call is `hub.NewHub(b, logger.With(...), *pvChannels)`. |
| `internal/hub/hub_test.go` | TestBMSBitmapCycling and TestTopologyConstants tests | ✓ VERIFIED | Both functions exist and pass. `TestBMSBitmapCyclingWriteFailure` also present. |
| `web/static/app.js` | Hardcoded topology constants, no dropdown functions | ✓ VERIFIED | Contains `var TOPO_TOWERS = 2` and `var TOPO_PACKS = 10`. No `initTopologyDropdowns`, `sendTopologyConfigure`, `loadTopologyValue`, `saveTopologyValue`. No `BAT_INPUTS_KEY`/`BAT_TOWERS_KEY`/`BAT_PACKS_KEY`. `packViewState` uses `TOPO_TOWERS`/`TOPO_PACKS`. No `topologyInputs` field. |
| `web/static/index.html` | HTML without topology controls div | ✓ VERIFIED | No `topology-controls`, `bat-inputs-select`, `bat-towers-select`, `bat-packs-select`. PV channel select and auto-refresh button present. |
| `web/static/style.css` | CSS without topology-controls rules, cell-grid with 4 columns | ✓ VERIFIED | No `.topology-controls`, `.topology-label`, `.topology-select`. `.cell-grid` uses `repeat(4, 1fr)`. `.pack-selector-controls` added. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `cmd/server/main.go` | `internal/hub/hub.go` | `NewHub(b, logger, pvChannels)` | ✓ WIRED | Line 81: `wsHub := hub.NewHub(b, logger.With("component", "hub"), *pvChannels)` |
| `internal/hub/hub.go triggerBMSRead` | `broker.WriteRegister(0x9020)` | bitmap cycling loop | ✓ WIRED | Line 650: `h.broker.WriteRegister(h.ctx, 0x9020, queryWord)` with `queryWord := uint16(t) << 8` |
| `internal/hub/hub.go triggerBMSRead` | `broker.ReadBatch(0x9022)` | bitmap read after write | ✓ WIRED | Line 658: `h.broker.ReadBatch(h.ctx, []broker.ReadRequest{{Addr: 0x9022, Count: 1}})` |
| `web/static/app.js handleBitmapCellClick` | `sendSelectPack` | hardcoded input=1, tower=towerIndex+1 | ✓ WIRED | Lines 977-978: `var input = 1; var tower = towerIndex + 1;` |
| `web/static/app.js packViewState` | `TOPO_TOWERS` | constant assignment | ✓ WIRED | Line 20: `topologyTowers: TOPO_TOWERS` |
| `internal/hub/hub.go triggerPackRead` | `register.EncodePackQuery` | `EncodePackQuery(input, tower, pack, TopoTowers)` | ✓ WIRED | Line 1033-1034: `towersPerInput := TopoTowers; queryWord := register.EncodePackQuery(input, tower, pack, towersPerInput)` |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|--------------|--------|-------------------|--------|
| `hub.go triggerBMSRead` | `onlineBitmaps[t]` | `broker.ReadBatch(0x9022)` after per-tower `WriteRegister(0x9020)` | Yes — hardware register read, not static | ✓ FLOWING |
| `hub.go triggerPackRead` | `queryWord` | `register.EncodePackQuery(input, tower, pack, TopoTowers)` | Yes — computed from UI input | ✓ FLOWING |
| `app.js handleBitmapCellClick` | `input`, `tower`, `pack` | Hardcoded `input=1`, click coordinates `towerIndex`/`packIndex` | Yes — derived from bitmap grid click | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Go project compiles cleanly | `go build ./...` | exit 0, no output | ✓ PASS |
| All test packages pass | `go test ./... -count=1 -timeout 120s` | 5 packages pass (broker, hub, modbus, register, web) | ✓ PASS |
| TestBMSBitmapCycling verifies per-tower bitmap cycling | `go test ./internal/hub/ -run TestBMSBitmapCycling -v` | PASS — asserts 2 writes (0x0000/0x0100) and distinct Online[0]=0x03FF, Online[1]=0x001F | ✓ PASS |
| TestTopologyConstants verifies constant values | `go test ./internal/hub/ -run TestTopologyConstants -v` | PASS — TopoTowers=2, TopoPacksPerTower=10, TopoCellsPerPack=16 | ✓ PASS |
| TestHandleSelectPack verifies pack clamping | `go test ./internal/hub/ -run TestHandleSelectPack -v` | PASS | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| PACK-01 | 06-02, 06-03 | All 20 battery packs accessible for drill-down | ✓ SATISFIED | Per-tower bitmap cycling (hub.go:643-664) ensures correct online status per tower. Frontend `handleBitmapCellClick` uses `input=1, tower=towerIndex+1` enabling click on any of 20 cells. `TestBMSBitmapCycling` passes. |
| PACK-02 | 06-01 | Pack selection correctly encodes tower/pack index in 0x9020 write | ✓ SATISFIED | `EncodePackQuery` implements `packIdx | (group << 8)` matching CLI. `triggerPackRead` uses `towersPerInput := TopoTowers` constant. `TestEncodePackQuery` and `TestEncodePackQueryInHandler` pass. |
| PACK-03 | 06-01, 06-03 | Topology hardcoded: 16 cells/pack, 10 packs/tower, 2 towers | ✓ SATISFIED | Constants in hub.go, JS constants in app.js, 16-cell loop in battery.go, `.cell-grid` 4-column CSS. `TestTopologyConstants` passes. `TestPackRTProbes` verifies exactly 16 cell probes. |

### Anti-Patterns Found

No blockers or warnings found in modified files. No TODO/FIXME/placeholder comments in new code. No empty implementations. The old "Apply same bitmap to all towers" bug comment is absent from hub.go.

### Human Verification Required

#### 1. Live Hardware: Per-Tower Bitmap Accuracy

**Test:** Connect to live Sofar HYD inverter, navigate to BMS section in web app, observe bitmap grid for Tower 1 and Tower 2 rows.
**Expected:** Each tower row shows its true online/offline pack pattern. The two rows may differ. Neither row should show stale/identical data copied from a single 0x9022 read. Check that the ~2-3s additional load time occurs (indicating per-tower cycling actually ran).
**Why human:** Bitmap cycling correctness depends on the 500ms settle time being sufficient for the actual hardware after writing to 0x9020. Unit tests use a mock broker that responds instantly. Only live hardware can confirm the settle timing is adequate. Also verifies the correct group index encoding `uint16(t) << 8` produces the right inverter response.

#### 2. Live Hardware: All 20 Pack Cells Clickable

**Test:** In BMS overview, click on packs from Tower 1 (positions 1-10) and Tower 2 (positions 1-10). Verify each navigation leads to a pack detail view showing 16 cell voltages with real data.
**Expected:** All 20 cells produce a pack detail view. Cell voltage grid shows 4 columns × 4 rows (16 cells). Values are non-zero for online packs. Previously, only packs from one tower were accessible.
**Why human:** The full end-to-end flow (bitmap click → select_pack WebSocket → 0x9020 write → settle → 0x9044+ read → WebSocket push → UI render) can only be validated against live hardware. The unit tests verify each component but not the integrated path with real Modbus timing.

### Gaps Summary

No automated gaps. All 4 must-haves verified. PACK-01, PACK-02, and PACK-03 requirements are satisfied by the implementation. Two human verification items are outstanding, both requiring live inverter hardware.

---

_Verified: 2026-04-11T19:35:00Z_
_Verifier: Claude (gsd-verifier)_
