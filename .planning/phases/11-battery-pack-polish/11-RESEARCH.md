# Phase 11: Battery Pack Polish - Research

**Researched:** 2026-04-12
**Domain:** Per-register streaming for pack drill-down views, group reordering, unsupported register handling
**Confidence:** HIGH

## Summary

Phase 11 converts the pack drill-down view from batch rendering to per-register streaming, matching the pattern already established for all main sections (System, Grid, EPS, PV, Battery, BMS) in Phase 7. The existing streaming infrastructure (section_schema + register_value + section_complete message types, skeleton rendering, dimming, caching, tooltips) provides all the building blocks. The core work is: (1) a new `streamPackRead` backend function that replaces the batch `triggerPackRead` + `buildPackDataMessage` flow, (2) pack-specific skeleton renderers on the frontend that handle cell_grid, balance, and pack_status group types, (3) group reordering to place Balance before Temperatures (D-03), and (4) unsupported register tracking to skip timed-out registers on subsequent reads (D-04/D-05).

The broker already enforces inter-read delay globally (D-06 is satisfied by the existing `enforceInterReadDelay()` in `broker.go:532`). The 500ms delay is applied per `ReadRegisters` call in the broker's command loop, so switching to per-register streaming for packs automatically inherits this timing. No broker changes are needed.

**Primary recommendation:** Build `streamPackRead` as a new function in `hub_streaming.go` following the exact pattern of `streamBMSRead` (stream individual registers, collect results for post-processing of cell statistics, balance bitmap, and status bitmap decoding). Frontend adds pack-aware skeleton renderers and a `handlePackRegisterValue` dispatcher that routes pack register values to cell/balance/status updaters.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- D-01: Pack Streaming Architecture -- Reuse existing streaming pattern (section_schema + register_value + section_complete) for pack drill-down views
- D-02: Pack Schema Frequency -- Send pack section_schema before every read cycle
- D-03: Group Display Order -- Info, Cells, Balance, Temps, Status
- D-04: Unsupported Register Handling -- Probe once per pack, skip on timeout for subsequent reads
- D-05: Unsupported Register Memory Scope -- Reset unsupported register list on pack switch
- D-06: Read Delay Enforcement -- Enforce 500ms minimum delay globally in the broker between any two Modbus reads

### Claude's Discretion
None specified -- all decisions are locked.

### Deferred Ideas (OUT OF SCOPE)
None -- all 3 matching todos were folded into scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| BATT-01 | Balance state section appears before temperature section in pack drill-down view | D-03 locks the group order: Info, Cells, Balance, Temps, Status. Backend reorders group appends in the new streaming function and schema builder. Frontend renders groups in arrival order. |
| BATT-02 | Pack drill-down values stream per-register as they are read, consistent with other sections | D-01 locks the architecture. Backend reads each register individually via `broker.ReadRegisters()` and sends `register_value` messages. Frontend builds skeleton from schema, fills values progressively. |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

- **Tech stack:** Go backend (stdlib only), vanilla HTML/JS/CSS frontend (embedded via Go embed)
- **No external dependencies:** Pure Go standard library
- **Hardware timing:** 500ms minimum inter-read delay, 1s BMS pack settle time
- **Single Modbus connection:** Serial access through broker queue
- **Code conventions:** PascalCase exported functions, camelCase locals, `// === Section ===` markers, explicit error handling with `fmt.Errorf`, error as last return value
- **gofmt formatting:** Standard Go formatting, no custom linters
- **GSD Workflow:** All changes through GSD commands

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib | 1.26 | Backend (net, encoding/binary, sync/atomic, context, time) | Project constraint: no external deps [VERIFIED: go.mod] |
| Vanilla JS | ES6 | Frontend DOM manipulation, WebSocket client | Project constraint: no frameworks [VERIFIED: app.js] |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| log/slog | Go 1.26 stdlib | Structured logging | All hub logging follows slog pattern [VERIFIED: hub.go imports] |
| testing | Go 1.26 stdlib | Unit tests | Hub tests use standard `testing` package [VERIFIED: hub_test.go] |
| encoding/json | Go 1.26 stdlib | WebSocket message marshaling | All message types marshal via json.Marshal [VERIFIED: message.go] |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Per-register ReadRegisters loop | ReadBatch for pack blocks | ReadBatch gives all-at-once data but no per-register streaming. Decision D-01 locks per-register approach. |

## Architecture Patterns

### Recommended Project Structure
```
internal/hub/
  hub.go             # handleSelectPack, triggerPackRead (modified)
  hub_streaming.go   # streamPackRead (new function, alongside streamBMSRead)
  message.go         # New pack schema message fields (pack_context)
  section.go         # No changes
  export_test.go     # Test helpers (may need additions)
internal/register/
  battery.go         # PackProbeGroups function (new, groups probes by display group)
  probe_group.go     # No changes
web/static/
  app.js             # Pack skeleton renderers, pack register_value handler
  style.css          # No CSS changes needed (confirmed by UI-SPEC)
```

### Pattern 1: Streaming Pack Read (mirrors streamBMSRead)
**What:** New `streamPackRead` function in `hub_streaming.go` that reads pack registers one-by-one, sending `register_value` messages for each, then sends `section_complete`.
**When to use:** All pack drill-down reads (both initial select_pack and auto-refresh read_cycle).
**Key difference from streamBMSRead:** Pack reads require write-settle-read cycle (write 0x9020, wait settle, then read). The read phase streams per-register. Post-processing computes cell statistics, balance bitmap decode, and status bitmap decode from accumulated results.

```go
// Source: Pattern derived from streamBMSRead in hub_streaming.go [VERIFIED: hub_streaming.go:257-444]
func (h *Hub) streamPackRead(input, tower, pack int, client *Client, readCtx context.Context) {
    // 1. Write 0x9020 to select pack (same as current triggerPackRead)
    // 2. Wait settle time
    // 3. Send pack section_schema with pack_context
    // 4. Read each register individually via h.broker.ReadRegisters()
    //    - Stream register_value for each
    //    - Track unsupported registers (timeout -> skip)
    // 5. Post-process: cell statistics, balance bitmap, status bitmaps
    // 6. Send section_complete
}
```

### Pattern 2: Pack Probe Groups (new function for schema + ordering)
**What:** New `PackProbeGroups()` function in `register/battery.go` that organizes existing pack probes into `ProbeGroup` structs in the correct display order (Info, Cells, Balance, Temps, Status).
**Why:** Current code has 3 flat probe lists (`PackRTProbes`, `PackInfoProbes`, `PackTemps58Probes`). Streaming needs grouped probes for schema generation and per-group iteration, in the D-03 display order.

```go
// Source: Derived from buildPackDataMessage group construction [VERIFIED: hub.go:847-1029]
func PackProbeGroups() []ProbeGroup {
    return []ProbeGroup{
        {Name: "Pack Info", Probes: [...]},      // RT block info + Info block items
        {Name: "Cell Voltages", Type: "cell_grid", Probes: [...]},  // 16 cells from RT block
        {Name: "Balance State", Type: "balance", Probes: [...]},    // 0x9075 from RT block
        {Name: "Temperatures", Probes: [...]},    // Temp 1-4 from RT, Temp 5-8 from 0x90BC block
        {Name: "Pack Status", Type: "pack_status", Probes: [...]},  // Alarm/Protection/Fault from RT + Info
    }
}
```

### Pattern 3: Unsupported Register Tracking (D-04, D-05)
**What:** Per-pack-read skip list that tracks registers returning timeout errors. Cleared on pack switch (D-05).
**Implementation:** A `map[uint16]bool` (or `sync.Map`) on the Hub, keyed by register address. On first read cycle for a pack, all registers are attempted. If a register times out, it is added to the skip map. On subsequent reads of the same pack, skipped registers are not read (and no register_value is sent for them). On pack switch (new select_pack), the map is cleared.

```go
// Source: D-04 and D-05 decisions [VERIFIED: 11-CONTEXT.md]
// Hub struct addition:
type Hub struct {
    // ...
    packSkipRegisters map[uint16]bool // registers to skip for current pack
}

// In streamPackRead, before each ReadRegisters:
if h.packSkipRegisters[probe.Addr] {
    continue // skip previously timed-out register
}
data, err := h.broker.ReadRegisters(readCtx, probe.Addr, probe.Count)
if err != nil && isTimeout(err) {
    h.packSkipRegisters[probe.Addr] = true
}

// In handleSelectPack, on pack change:
h.packSkipRegisters = make(map[uint16]bool)
```

### Pattern 4: Pack Schema with Context (frontend routing)
**What:** Extend `SectionSchemaMessage` with optional `PackContext` field so the frontend can distinguish pack schemas from BMS overview schemas.
**Implementation:** Add `PackContext *PackSchemaContext` to `SectionSchemaMessage`. When present, frontend builds pack skeleton instead of standard section skeleton.

```go
// Source: UI-SPEC section 2a [VERIFIED: 11-UI-SPEC.md:108-123]
type PackSchemaContext struct {
    Input int `json:"input"`
    Tower int `json:"tower"`
    Pack  int `json:"pack"`
}

type SectionSchemaMessage struct {
    // existing fields...
    PackContext *PackSchemaContext `json:"pack_context,omitempty"`
}
```

### Pattern 5: Frontend Pack Register Value Routing
**What:** When `packViewState.mode === 'pack_detail'` and a `register_value` message arrives for section `bms`, route to pack-specific updaters instead of standard handler.
**Key insight:** The current code guards against pack mode in `handleRegisterValue` (`if (msg.section === 'bms' && packViewState.mode === 'pack_detail') return;`). This guard must be replaced with routing logic that dispatches to pack-specific updaters.

### Anti-Patterns to Avoid
- **Sending PackDataMessage for streaming:** The existing `PackDataMessage` type carries all groups at once. Do NOT use it for the streaming path. Use the standard `RegisterValueMessage` and `SectionSchemaMessage` types.
- **Per-register delay in streaming function:** Do NOT add `time.Sleep(500ms)` in `streamPackRead`. The broker's `enforceInterReadDelay()` handles this automatically for every `ReadRegisters` call. [VERIFIED: broker.go:393]
- **Rebuilding cell statistics on every cell value:** Cell statistics (min/max/spread/avg) should only update after all 16 cell values arrive (using a counter), not incrementally per cell. Incremental updates cause visual jitter on summary values.
- **Clearing DOM on each register_value:** The skeleton must be built once (from schema), then only values are updated. Never clear `content-body` on register_value arrival.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Inter-read delay | Manual sleep in streaming function | `broker.ReadRegisters()` | Broker enforces 500ms delay globally [VERIFIED: broker.go:532-538] |
| Register value formatting | Custom formatting in streaming function | `register.FormatValue(p, data)` | Existing function handles all types: ASCII, signed, unsigned, scaled, enum [VERIFIED: section.go:17] |
| Raw value formatting | Custom hex/raw formatter | `register.FormatRawValue(p, data)` | Existing function for tooltip raw values [VERIFIED: section.go:19] |
| BMS bitmap decoding | Custom bitmap decode | `register.DecodeBMSBitmap()` | Existing function for alarm/protection/fault bits [VERIFIED: battery.go:247-255] |
| Pack query encoding | Manual bit-shifting | `register.EncodePackQuery()` | Existing function for 0x9020 write value [VERIFIED: battery.go:100-104] |
| JSON message construction | Manual map building | `NewRegisterValue()`, `NewSectionSchema()`, `NewSectionComplete()` | Existing constructor functions [VERIFIED: message.go:233-262] |
| Cache key construction | Hardcoded strings | `getCacheKey()` | Already handles pack_detail mode [VERIFIED: app.js:35-41] |

## Common Pitfalls

### Pitfall 1: Pack Registers Span Multiple Modbus Read Blocks
**What goes wrong:** Pack data comes from 3 separate register blocks (RT: 0x9044-0x907C, Info: 0x9104-0x9126, Temps58: 0x90BC-0x90BF). The current batch approach reads each block as one request. Switching to per-register means individual `ReadRegisters` calls for each probe, which cross block boundaries.
**Why it happens:** The probe list is organized by display group (Info, Cells, Balance, Temps, Status), not by register block. An Info group probe may need data from both the RT block (e.g., Pack ID at 0x9044) and the Info block (e.g., SOH at 0x910A).
**How to avoid:** Organize the streaming read order by register block, not display group. Read all RT block registers first (0x9044-0x907C), then Info block (0x9104-0x9126), then Temps58 block (0x90BC-0x90BF). Map each probe to its display group name for the `register_value` message's `group` field regardless of read order.
**Warning signs:** If you see the schema group order not matching the read order, that is expected and correct. Schema groups determine display order; read order determines Modbus efficiency.

### Pitfall 2: BMS Pack Switch Write Must Precede All Reads
**What goes wrong:** The 0x9020 write to select a pack must happen before any pack register reads. If streaming is implemented naively, the write might be skipped or delayed.
**Why it happens:** The current `triggerPackRead` handles write-settle-read as a single goroutine. The new streaming function must preserve this exact sequence.
**How to avoid:** `streamPackRead` starts with the write-settle sequence (identical to current `triggerPackRead` lines 760-774), then enters the per-register streaming loop. The schema message is sent after the settle wait but before the first register read.
**Warning signs:** If pack data shows values from the previously selected pack.

### Pitfall 3: Info Block Registers May Be Unsupported (D-04)
**What goes wrong:** Registers 0x9104-0x9126 return "illegal address" (Modbus exception 0x02) on some Sofar HYD BMS hardware. Currently handled by `h.skipPackInfo` session-level flag. D-04 requires per-register, per-pack tracking.
**Why it happens:** Different BMS firmware versions expose different register sets.
**How to avoid:** Track unsupported registers in `packSkipRegisters` map. On timeout or illegal address error, add register to skip list. On pack switch, clear the skip list (D-05). For skipped registers, do not send a register_value message -- the frontend skeleton shows the em-dash placeholder and the register remains pending.
**Warning signs:** If all Info block values show as pending permanently on a BMS that supports them, the skip list may be clearing incorrectly.

### Pitfall 4: Cell Statistics Need All 16 Values Before Accurate Rendering
**What goes wrong:** Cell deviation colors and summary statistics (min/max/spread/avg) are meaningless until all 16 cells have reported values.
**Why it happens:** Cells arrive one-by-one. Computing min/max from partial data gives incorrect results.
**How to avoid:** Frontend tracks a cell arrival counter per read cycle. Summary row and deviation colors are recalculated only when all 16 cells have arrived (or on section_complete as a fallback). Individual cell values are displayed as they arrive but without deviation coloring until the full set is available.
**Warning signs:** Cell colors flickering between good/warn/danger as values stream in.

### Pitfall 5: Pack Schema Must Differentiate from BMS Overview Schema
**What goes wrong:** Frontend receives `section_schema` with `section: "bms"` and cannot tell if it is for BMS overview or pack detail.
**Why it happens:** Both pack and BMS overview use section "bms". Without a distinguishing field, the schema handler applies the wrong rendering.
**How to avoid:** Add `pack_context` field to `SectionSchemaMessage` (as specified in UI-SPEC). Frontend checks for `pack_context` presence to decide rendering path. When `pack_context` is present: build pack skeleton with breadcrumb. When absent: build standard BMS overview skeleton.
**Warning signs:** Pack detail view showing BMS overview layout, or BMS overview showing pack breadcrumb.

### Pitfall 6: Register Value Handler Guard Inversion
**What goes wrong:** The current `handleRegisterValue` has a guard that returns early when in pack_detail mode (`if (msg.section === 'bms' && packViewState.mode === 'pack_detail') return;`). This guard must be changed, not just removed.
**Why it happens:** Without the guard, BMS overview register values would incorrectly update pack detail DOM elements. With the guard removed entirely, values flow through but may not find the right DOM elements.
**How to avoid:** Replace the guard with routing logic: if in pack_detail mode, dispatch to `handlePackRegisterValue(msg)` which uses pack-specific DOM selectors. If in overview mode, use standard `handleRegisterValue` logic. The `data-register` attribute format is the same (`group::name`), so the DOM lookup works if the skeleton was built with matching attributes.
**Warning signs:** Values updating wrong elements or not updating at all.

### Pitfall 7: Concurrent Pack Read and BMS Overview Read
**What goes wrong:** If a user switches from pack detail back to BMS overview while a pack read is in progress, stale pack register_value messages may arrive after the overview schema is rendered.
**Why it happens:** The pack read goroutine continues after the user navigates away.
**How to avoid:** The existing `readCancel` mechanism on the Section cancels in-progress reads when a new read starts. When the user navigates back to overview, a new subscribe triggers a new BMS overview read, which cancels the pack read via `sec.cancelRead()`. Additionally, the frontend should check `packViewState.mode` before applying register_value updates.
**Warning signs:** BMS overview values appearing in wrong groups or vice versa.

## Code Examples

### Backend: Pack Schema Builder
```go
// Source: Derived from buildSectionSchema pattern [VERIFIED: hub_streaming.go:15-29]
func (h *Hub) buildPackSchema(input, tower, pack int, groups []register.ProbeGroup) SectionSchemaMessage {
    var schemaGroups []SchemaGroup
    for _, g := range groups {
        sg := SchemaGroup{
            Name:   g.Name,
            Layout: g.Layout,
            Type:   g.Type,
        }
        for _, p := range g.Probes {
            sg.Registers = append(sg.Registers, p.Name)
        }
        schemaGroups = append(schemaGroups, sg)
    }
    return SectionSchemaMessage{
        Type:        MsgTypeSectionSchema,
        Section:     "bms",
        Groups:      schemaGroups,
        PackContext: &PackSchemaContext{Input: input, Tower: tower, Pack: pack},
    }
}
```

### Backend: Streaming Pack Read Core Loop
```go
// Source: Pattern from streamBMSRead [VERIFIED: hub_streaming.go:257-373]
// For each probe in the ordered probe groups:
for _, g := range groups {
    for _, p := range g.Probes {
        if readCtx.Err() != nil {
            return
        }
        if skipRegisters[p.Addr] {
            continue
        }
        data, err := h.broker.ReadRegisters(readCtx, p.Addr, p.Count)
        if err != nil {
            if isTimeoutOrIllegal(err) {
                skipRegisters[p.Addr] = true
            }
            if readCtx.Err() != nil {
                return
            }
            h.results <- sectionResult{
                section: "bms",
                msg:     NewRegisterValue("bms", g.Name, p.Name, "", err.Error(), p.Addr, ""),
            }
            continue
        }
        if readCtx.Err() != nil {
            return
        }
        h.results <- sectionResult{
            section: "bms",
            msg:     NewRegisterValue("bms", g.Name, p.Name, FormatValue(p, data), "", p.Addr, FormatRawValue(p, data)),
        }
    }
}
```

### Frontend: Pack Schema Detection in handleSectionSchema
```javascript
// Source: UI-SPEC section 2a-2b [VERIFIED: 11-UI-SPEC.md:94-137]
function handleSectionSchema(msg) {
    if (msg.section !== App.activeSection) return;

    // Pack schema detection: pack_context present means pack drill-down
    if (msg.pack_context) {
        packViewState.mode = 'pack_detail';
        packViewState.selectedInput = msg.pack_context.input;
        packViewState.selectedTower = msg.pack_context.tower;
        packViewState.selectedPack = msg.pack_context.pack;
        renderPackSkeleton(msg);
        return;
    }

    // BMS overview guard (existing)
    if (msg.section === 'bms' && packViewState.mode === 'pack_detail') return;

    // ... existing standard schema handling
}
```

### Frontend: Register Value Routing for Pack Mode
```javascript
// Source: UI-SPEC section 2c [VERIFIED: 11-UI-SPEC.md:139-159]
function handleRegisterValue(msg) {
    if (msg.section !== App.activeSection) return;

    // Route to pack handler when in pack_detail mode
    if (msg.section === 'bms' && packViewState.mode === 'pack_detail') {
        handlePackRegisterValue(msg);
        return;
    }

    // ... existing standard register value handling
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Batch pack read (ReadBatch for entire block) | Per-register streaming (ReadRegisters per probe) | Phase 11 | Matches main section pattern, enables progressive rendering |
| Session-level skipPackInfo flag | Per-register, per-pack skip tracking | Phase 11 | Better adaptation to mixed hardware, fresh probe on pack switch |
| Group order: Info, Cells, Temps, Status, Balance | Group order: Info, Cells, Balance, Temps, Status | Phase 11 | User-preferred logical presentation (BATT-01) |

## Assumptions Log

> List all claims tagged [ASSUMED] in this research.

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Cell statistics should only be calculated when all 16 cells have values (not incrementally) | Pitfall 4 | Minor -- incremental would cause visual jitter but not data corruption. Could be acceptable UX tradeoff. |
| A2 | Streaming reads should be ordered by register block (RT, Info, Temps58) not by display group for Modbus efficiency | Pitfall 1 | Low -- individual ReadRegisters calls go through broker queue regardless of order. Block ordering may reduce connection churn but has no timing impact since broker enforces delay anyway. |

**Both assumptions are implementation details within Claude's discretion and have low risk if wrong.**

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (stdlib), 1.26 |
| Config file | None -- standard `go test` |
| Quick run command | `go test ./internal/hub/ -run TestPack -v -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| BATT-01 | Balance group appears before Temperatures in pack probe groups | unit | `go test ./internal/register/ -run TestPackProbeGroupOrder -v -count=1` | Wave 0 |
| BATT-01 | Pack schema message groups are in order: Info, Cells, Balance, Temps, Status | unit | `go test ./internal/hub/ -run TestPackSchemaGroupOrder -v -count=1` | Wave 0 |
| BATT-02 | streamPackRead sends register_value messages (not pack_data) | unit | `go test ./internal/hub/ -run TestPackStreamingMessages -v -count=1` | Wave 0 |
| BATT-02 | Pack schema includes pack_context with correct input/tower/pack | unit | `go test ./internal/hub/ -run TestPackSchemaContext -v -count=1` | Wave 0 |
| D-04 | Unsupported registers are skipped on subsequent reads | unit | `go test ./internal/hub/ -run TestPackSkipUnsupported -v -count=1` | Wave 0 |
| D-05 | Skip list clears on pack switch | unit | `go test ./internal/hub/ -run TestPackSkipResetOnSwitch -v -count=1` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/hub/ -run TestPack -v -count=1`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/register/battery_test.go` -- test for `PackProbeGroups()` group ordering
- [ ] `internal/hub/hub_test.go` -- tests for pack streaming message flow, schema context, skip register logic

## Open Questions

1. **Cell statistics rendering strategy**
   - What we know: All 16 cell values arrive sequentially. Summary (min/max/spread/avg) needs all values.
   - What's unclear: Should individual cells show with pending deviation colors until all arrive, or should they show without color classes?
   - Recommendation: Show cell values as they arrive (text updates), apply deviation colors only when all 16 are present. The UI-SPEC does not specify timing of color application, so this is implementation detail.

2. **Pack status progressive rendering**
   - What we know: Status registers (alarm, protection, fault) arrive individually. Card must switch between clear and active states.
   - What's unclear: Should the card start as "clear" and switch to "active" on first non-zero register, or should it remain in pending state until all status registers arrive?
   - Recommendation: Start in pending state. Switch to clear/active after all status registers arrive (or on section_complete). This avoids a brief "clear" flash before an alarm arrives.

## Sources

### Primary (HIGH confidence)
- `internal/hub/hub_streaming.go` -- Full streaming pattern for standard, battery, and BMS reads [VERIFIED: codebase read]
- `internal/hub/hub.go:709-814` -- Current triggerPackRead and buildPackDataMessage implementation [VERIFIED: codebase read]
- `internal/hub/message.go` -- All WebSocket message types and constructors [VERIFIED: codebase read]
- `internal/register/battery.go` -- Pack probe definitions, bitmap tables, encode functions [VERIFIED: codebase read]
- `web/static/app.js` -- Frontend handlers: handlePackData, handleSectionSchema, handleRegisterValue, renderPackDetail [VERIFIED: codebase read]
- `internal/broker/broker.go:382-410` -- enforceInterReadDelay and executeRead with retry [VERIFIED: codebase read]
- `.planning/phases/11-battery-pack-polish/11-CONTEXT.md` -- All 6 locked decisions [VERIFIED: file read]
- `.planning/phases/11-battery-pack-polish/11-UI-SPEC.md` -- Full UI design contract [VERIFIED: file read]
- `.planning/REQUIREMENTS.md` -- BATT-01 and BATT-02 requirement definitions [VERIFIED: file read]

### Secondary (MEDIUM confidence)
- None -- all findings verified from codebase.

### Tertiary (LOW confidence)
- None.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- Pure Go stdlib project, no dependency questions
- Architecture: HIGH -- All patterns derived from existing streaming code in same codebase
- Pitfalls: HIGH -- Based on actual code analysis of current pack implementation and streaming infrastructure

**Research date:** 2026-04-12
**Valid until:** 2026-05-12 (stable codebase, no external dependencies)
