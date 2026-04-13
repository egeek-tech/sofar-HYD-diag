---
phase: 14-system-time-fix
verified: 2026-04-13T18:00:00Z
status: human_needed
score: 6/6 must-haves verified
overrides_applied: 0
human_verification:
  - test: "Start server and navigate to System section in browser"
    expected: "System section Status group shows exactly 2 rows: 'Running state' and 'System time'; System time value is in HH:MM:SS DD-MM-YYYY format (e.g. '10:59:34 13-04-2026'); no individual Year/Month/Day/Hour/Min/Sec rows appear"
    why_human: "Visual layout and live WebSocket streaming cannot be verified without a running server and inverter (or mock)"
  - test: "Hover over the System time row in the System section"
    expected: "Tooltip shows 'Registers: 0x042C-0x0431' on first line and 'Raw: <values>' on second line; 'Last read:' timestamp on third line; no single 'Register: 0x042C' line"
    why_human: "Tooltip rendering requires live browser interaction; JavaScript pipe-delimiter logic is present in code but visual output requires manual confirmation"
---

# Phase 14: System Time Fix Verification Report

**Phase Goal:** System time displays as a human-readable single value instead of raw register parts
**Verified:** 2026-04-13T18:00:00Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | System time appears as a single composed row in the System section Status group | VERIFIED | `system.go` Status group has exactly 2 probes: "Running state" and synthetic "System time" (Count:0); `hub_streaming.go` sends `NewRegisterValue(..., "System time", composed, ...)` after batch read |
| 2 | The composed time uses HH:MM:SS DD-MM-YYYY format (e.g. '10:59:34 13-04-2026') | VERIFIED | `format.go` line 95: `Sprintf("%02d:%02d:%02d %02d-%02d-%04d", hour, min, sec, day, month, year+2000)`; `TestComposeSystemTime` asserts `"14:30:05 10-04-2026"` and `"00:00:00 01-01-2000"` — both pass |
| 3 | The 6 individual time register rows (Year, Month, Day, Hour, Min, Sec) no longer appear | VERIFIED | `system.go` contains no "System time (Year/Month/Day/Hour/Min/Sec)" strings; `hub_streaming.go` contains no `timeVals`, `timeCount`, `strings.HasPrefix(p.Name, "System time (")`, or `if timeCount == 6` |
| 4 | The batch read is a single ReadRegisters(0x042C, 6) call, not 6 individual reads | VERIFIED | `hub_streaming.go` line 78: `h.broker.ReadRegisters(readCtx, 0x042C, 6)` inside `if g.Name == "Status"` block after probe loop |
| 5 | On batch read failure, the skeleton em-dash persists with no error text | VERIFIED | `hub_streaming.go`: on error the entire Status block is silently skipped — comment "D-06: On error, silently skip — skeleton dash persists until next successful cycle"; no error message is sent |
| 6 | Hovering the System time row shows tooltip with register range 0x042C-0x0431 and raw register values | VERIFIED (code) | `app.js` showTooltip: `raw.indexOf(' | ') !== -1` → splits on ` | ` → pushes `'Registers: ' + parts[0]` and `'Raw: ' + parts[1]`; `rawStr` in hub_streaming.go formatted as `"0x042C-0x0431 | %d, %d, %d, %d, %d, %d"` — visual confirmation needs human |

**Score:** 6/6 truths verified (automated code checks)

### Roadmap Success Criteria

| # | Success Criterion | Status | Evidence |
|---|------------------|--------|----------|
| SC-1 | System time appears as a single concatenated row (e.g., "10:59:34 13-04-2026") in the System section | VERIFIED | Synthetic probe + batch read + ComposeSystemTime produces the format; test assertions confirm |
| SC-2 | The separate year/month/day/hour/minute/second register rows no longer appear individually | VERIFIED | Old probe definitions and time-collection logic fully removed; grep confirms absence |

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/register/format.go` | ComposeSystemTime with HH:MM:SS DD-MM-YYYY format | VERIFIED | Line 95: `Sprintf("%02d:%02d:%02d %02d-%02d-%04d", hour, min, sec, day, month, year+2000)` |
| `internal/register/system.go` | Status group with 2 entries (Running state + synthetic System time) | VERIFIED | Lines 16-19: exactly 2 probes, "System time" has `Addr: 0x042C, Count: 0` |
| `internal/hub/hub_streaming.go` | Batch read of 0x042C-0x0431 after Status group probes | VERIFIED | Lines 77-96: `if g.Name == "Status"` block with `ReadRegisters(readCtx, 0x042C, 6)` |
| `web/static/app.js` | Tooltip parsing for pipe-delimited register range | VERIFIED | `raw.indexOf(' | ') !== -1` → `raw.split(' | ')` → `lines.push('Registers: ' + parts[0])` |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `internal/register/system.go` | `internal/hub/hub_streaming.go` | Status group probe list feeds schema; Count: 0 probes skipped during read loop | WIRED | `if p.Count == 0 { continue }` present at line 45-47 in hub_streaming.go |
| `internal/hub/hub_streaming.go` | `web/static/app.js` | register_value message with raw_value containing pipe delimiter | WIRED | rawStr formatted as `"0x042C-0x0431 | %d, ..."` at line 85-86; app.js parses `' | '` delimiter |
| `internal/register/system.go` | `internal/hub/hub_streaming.go` | Schema sends 'System time' register name; streaming sends matching register_value name | WIRED | `NewRegisterValue(sectionName, g.Name, "System time", ...)` at line 91; schema built from `p.Name` in buildSectionSchema |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|--------------|--------|--------------------|--------|
| `hub_streaming.go` batch read | `composed` string | `h.broker.ReadRegisters(readCtx, 0x042C, 6)` — live Modbus read of 6 registers | Yes — reads from hardware broker (not static) | FLOWING |
| `format.go` ComposeSystemTime | formatted string | uint16 values from Modbus register bytes | Yes — arithmetic on hardware register values | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| TestComposeSystemTime passes | `go test ./internal/register/ -run TestComposeSystemTime -count=1` | PASS | PASS |
| TestSystemGroups passes (2 probes) | `go test ./internal/register/ -run TestSystemGroups -count=1` | PASS | PASS |
| TestNewRegisterValueComposedJSON passes | `go test ./internal/hub/ -run TestNewRegisterValueComposedJSON -count=1` | PASS | PASS |
| Full test suite green | `go test ./... -count=1` | All packages PASS | PASS |
| Build compiles | `go build ./...` | Exit 0 | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| CLEAN-02 | 14-01-PLAN.md | System time displays as a single concatenated row instead of separate rows per register | SATISFIED | System section Status group has synthetic "System time" probe; batch read composes HH:MM:SS DD-MM-YYYY format; 6 individual time register rows removed |

No orphaned requirements: CLEAN-02 is the only requirement mapped to Phase 14 in REQUIREMENTS.md traceability table, and it is accounted for in 14-01-PLAN.md.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| None found | — | — | — | No stubs, placeholders, or incomplete implementations detected in modified files |

The `strings` import in `hub_streaming.go` is still used by the `streamPackRead` and `buildPackSchema` functions (lines 257, 338) — correctly retained.

### Human Verification Required

#### 1. System time single row display

**Test:** Start the server and navigate to the System section in a browser.
**Expected:** Status group shows exactly 2 rows — "Running state" and "System time". The System time value is formatted as HH:MM:SS DD-MM-YYYY (e.g., "10:59:34 13-04-2026"). No individual Year/Month/Day/Hour/Min/Sec rows are visible.
**Why human:** Live WebSocket streaming and frontend rendering cannot be verified without a running server. The register schema and streaming logic are wired correctly in code, but visual confirmation requires a browser.

#### 2. System time tooltip with register range

**Test:** In the System section, hover over the "System time" row value.
**Expected:** Tooltip shows three lines: "Registers: 0x042C-0x0431", "Raw: 26, 4, 13, 10, 59, 34" (actual values will vary), and "Last read: <timestamp>". The single "Register: 0x042C" line should NOT appear.
**Why human:** JavaScript tooltip rendering is conditional on the pipe-delimiter in raw_value. The logic is correct in app.js but the actual rendered tooltip requires browser interaction to confirm.

### Gaps Summary

No gaps found. All 6 must-have truths are verified in the codebase, all artifacts are substantive and wired, data flows from Modbus hardware reads through ComposeSystemTime to the frontend. Two human verification items remain for visual/UX confirmation but do not indicate implementation deficiencies.

---

_Verified: 2026-04-13T18:00:00Z_
_Verifier: Claude (gsd-verifier)_
