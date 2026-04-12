# Phase 10: Data Persistence & Tooltips - Research

**Researched:** 2026-04-12
**Domain:** Frontend state management (vanilla JS), CSS visual transitions, Go WebSocket message extension
**Confidence:** HIGH

## Summary

Phase 10 adds three frontend behaviors to the existing streaming read architecture: (1) refresh dimming -- when a new read cycle begins, previously-read values dim to 50% opacity and progressively restore as fresh values arrive; (2) section-level caching -- navigating away and back shows cached values dimmed immediately, avoiding blank screens; (3) parameter tooltips -- hover over any value to see its Modbus register address, raw value, and last-read timestamp.

The backend change is narrowly scoped: extend `RegisterValueMessage` with two new fields (`register_addr` and `raw_value`) so the frontend has the metadata needed for tooltips. All other changes are pure frontend: a new CSS class for container-level dimming, an in-memory JavaScript cache (Map), data attribute management on value elements, and a reusable JS-managed tooltip element. The UI-SPEC (approved) provides exact CSS properties, tooltip layout, and interaction flows.

**Primary recommendation:** Implement backend message extension first (smallest change, enables all three frontend features), then dimming/caching logic (most complex), then tooltip last (relies on data attributes populated by the cache logic).

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** When a new read cycle starts, all values in the section dim to 50% opacity immediately. No icon, no delay -- just reduced opacity. Triggered when the browser sends `read_cycle`.
- **D-02:** As each `register_value` arrives, that individual value snaps back to full opacity. Section progressively "fills in".
- **D-03:** The existing `--stale` CSS class (Phase 9, error case) remains distinct from refresh dimming. Refresh dim = 50% opacity, no icon. Error stale = dimmed + warning icon.
- **D-04:** Dimming is applied at the section container level (one class toggle), not per-value.
- **D-05:** Pack drill-down views use the same dimming behavior.
- **D-06:** Green flash on `section_complete` is preserved alongside dim/un-dim transitions.
- **D-07:** Dim/un-dim transitions use subtle CSS fade (~150-200ms).
- **D-08:** Browser maintains in-memory cache of last-read values, keyed per-section and per-pack.
- **D-09:** Cache granularity: per main section AND per individual pack drill-down page.
- **D-10:** First visit (no cache) shows em-dash skeleton placeholders (existing behavior). Em-dash = "never read", dimmed = "read before, refreshing".
- **D-11:** Cache preserved when switching sections with auto-refresh active.
- **D-12:** Cache cleared on disconnect. All values reset to em-dash skeletons.
- **D-13:** No manual "clear cache" button. No localStorage for cached values.
- **D-14:** On disconnect, content area shows em-dash skeletons (not blank).
- **D-15:** Every parameter value gets a hover tooltip (main sections AND pack drill-down).
- **D-16:** Tooltip content: register address hex, raw register value, last-read timestamp HH:MM:SS.
- **D-17:** Tooltip triggered by mouse hover only. No click-to-pin.
- **D-18:** Custom styled tooltip (dark theme, monospace). Positioned above value. 300ms hover delay.
- **D-19:** Error-stale values show register address and last known raw value in tooltip.

### Claude's Discretion
- How to send register address and raw value from backend to frontend (extend `register_value` message, or embed in schema, or via data attributes)
- Internal cache data structure in JavaScript (Map, plain object, etc.)
- How to implement the custom tooltip (CSS-only or JS-managed)
- How to store per-register timestamps in the cache
- Whether to add a CSS class like `--refreshing` distinct from `--stale`, or reuse opacity-based approach

### Deferred Ideas (OUT OF SCOPE)
- Read delay burst on section switch -- already fixed in Phase 8
- Skip unsupported PackInfoProbes registers -- already implemented in Phase 8
- Stream pack drill-down values per-register -- Phase 11 concern (BATT-02)
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| DISP-01 | Previously read parameter values persist on screen (dimmed) when a new refresh cycle begins, until replaced by fresh values | Container-level `.content__body--refreshing` class sets opacity:0.5 on all values; per-register restore via `.data-row-h__value--fresh` class or inline style as each `register_value` arrives |
| DISP-02 | Browser caches values per section and page -- navigating back to a previously viewed section shows cached values dimmed until refreshed | In-memory JS Map keyed by section name (and pack identifier for BMS drill-down); cache populated on each `register_value`; restored in `navigateToSection()` before subscribe |
| DISP-03 | User can hover over any parameter value to see a tooltip showing the register address and raw value | Backend adds `register_addr` (uint16) and `raw_value` (string) to `RegisterValueMessage`; frontend stores as `data-register-addr`, `data-register-raw`, `data-register-time` attributes; JS tooltip reads attributes on hover |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Vanilla JS | ES5/ES6 | All frontend logic | Project constraint: no frameworks [VERIFIED: CLAUDE.md] |
| Vanilla CSS | Custom properties | All styling | Project constraint: no preprocessors [VERIFIED: CLAUDE.md] |
| Go stdlib | 1.26.1 | Backend message serialization | Project constraint: stdlib only [VERIFIED: go.mod] |

### Supporting
No new libraries needed. This phase uses only existing project dependencies.

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| JS Map for cache | Plain object | Map has cleaner API for iteration and deletion; plain object works but Map is the better fit for key-value caches where keys are strings. Use Map. |
| JS-managed tooltip | CSS `::before`/`::after` | CSS-only tooltips cannot display dynamic content from data attributes (content property can read `attr()` but only as a string, and multi-line formatting is limited). JS-managed is required for this use case. |
| Inline `style="opacity:1"` | `.data-row-h__value--fresh` class | Inline style is simpler but harder to clean up (must remove inline style on next cycle). CSS class is cleaner: add on value arrival, remove all at cycle start. Use class approach per UI-SPEC. |

## Architecture Patterns

### Backend Change: RegisterValueMessage Extension

The `RegisterValueMessage` struct in `internal/hub/message.go` needs two new fields. [VERIFIED: current struct at message.go:189-196 has Type/Section/Group/Name/Value/Error only]

```go
// Source: internal/hub/message.go (current)
type RegisterValueMessage struct {
    Type    string `json:"type"`
    Section string `json:"section"`
    Group   string `json:"group"`
    Name    string `json:"name"`
    Value   string `json:"value,omitempty"`
    Error   string `json:"error,omitempty"`
    // Phase 10: tooltip metadata
    RegisterAddr uint16 `json:"register_addr"`
    RawValue     string `json:"raw_value,omitempty"`
}
```

**`NewRegisterValue` signature change:** Add `addr uint16` and `rawValue string` parameters. All 8 call sites in `hub_streaming.go` must be updated. [VERIFIED: 8 call sites found by grep]

**Raw value extraction:** A new `register.FormatRawValue(p Probe, data []byte) string` function should extract the raw numeric representation before scaling/formatting:
- For single-register probes (Count=1): `uint16` decimal (e.g., "3852")
- For U32 probes (Count=2): `uint32` decimal (e.g., "1234567")
- For ASCII probes: hex dump of raw bytes (e.g., "534F464152")
- For composed values (System time, BMS Clock, SW Version): use empty string (no single register address)

[VERIFIED: register/format.go shows data is always `[]byte` from broker, parsed as big-endian uint16 or uint32]

### Frontend Pattern 1: Container-Level Dimming (DISP-01)

**What:** Apply `.content__body--refreshing` class to `#content-body` when read cycle starts. Remove per-register freshness indicators from previous cycle. As each `register_value` arrives, add `.data-row-h__value--fresh` to that element (overrides container opacity). On `section_complete`, remove the container class as cleanup.

**Trigger points:** [VERIFIED: app.js]
1. When `App.ws.send({ type: 'read_cycle', ... })` is called (auto-refresh cycle start, line ~1039)
2. When `subscribe` is sent in `navigateToSection()` (line ~371) -- since subscribe triggers immediate read
3. When manual "Refresh" button triggers a read

**CSS (from UI-SPEC, approved):**
```css
.content__body--refreshing .data-row-h__value {
    opacity: var(--refresh-dim-opacity, 0.5);
    transition: opacity var(--refresh-dim-transition, 150ms) ease;
}

.data-row-h__value--fresh {
    opacity: 1 !important;
    transition: opacity var(--refresh-dim-transition, 150ms) ease;
}
```

**Cycle cleanup flow:**
1. Cycle start: add `.content__body--refreshing` to container, remove all `.data-row-h__value--fresh` from children
2. Each `register_value`: add `.data-row-h__value--fresh` to matching element
3. `section_complete`: remove `.content__body--refreshing` from container (all values now have `--fresh` class or `--stale`)

### Frontend Pattern 2: Section Value Cache (DISP-02)

**What:** In-memory `Map` storing last-known values per section. Updated on every successful `register_value`. Restored when navigating back to a section.

**Data structure recommendation:**
```javascript
// Top-level cache: Map<string, Map<string, CacheEntry>>
// Key: section name (e.g., "system") or pack key (e.g., "bms:pack:1:1:3")
// Value: Map of data-register key to cache entry
var sectionCache = new Map();

// CacheEntry shape:
// { value: string, registerAddr: string, rawValue: string, timestamp: string }
```

**Integration in `navigateToSection()`:** [VERIFIED: navigateToSection at app.js:~285]
1. After schema rendering via `handleSectionSchema`, check if cache exists for the section
2. If cache exists: iterate cached entries, populate matching `data-register` elements with cached values, set data attributes, add `.content__body--refreshing` class
3. If no cache: leave em-dash skeletons (existing behavior)
4. Then proceed with subscribe (triggers fresh read)

**Key consideration:** The `handleSectionSchema` message arrives asynchronously after `subscribe` is sent. The cache restoration must happen after the schema DOM is built, not before. Two approaches:
- Option A: Restore cache in `handleSectionSchema` after building the DOM (check cache, populate if exists)
- Option B: Store a pending flag and restore in a microtask after schema renders

Option A is simpler and recommended: extend `handleSectionSchema` to check the cache after building the skeleton DOM.

**Cache update point:** Inside `handleRegisterValue()`, after setting the value text, update the cache entry for the current section.

**Disconnect clearing:** In the disconnect handler (where `case 'disconnected':` is handled in `handleConnectionState`), call `sectionCache.clear()`.

### Frontend Pattern 3: JS-Managed Tooltip (DISP-03)

**What:** A single reusable tooltip `<div>` element, positioned absolutely via JS on mouseenter/mouseleave events. Content reads from `data-register-addr`, `data-register-raw`, `data-register-time` attributes.

**Implementation approach:**
1. Create tooltip element once at init (append to `<body>`)
2. Use event delegation on `#content-body` for `mouseenter`/`mouseleave` on `.data-row-h__value` elements
3. On mouseenter: start 300ms timeout. If mouse stays, position tooltip above element and show
4. On mouseleave: clear timeout, hide tooltip immediately
5. Content: read data attributes from the target element, format three-line tooltip

**Event delegation is essential** because value elements are dynamically created/destroyed during schema rendering and section navigation. Direct event binding would leak or miss elements.

**Positioning logic:**
```javascript
// Source: UI-SPEC approved positioning contract
var rect = targetEl.getBoundingClientRect();
tooltip.style.left = (rect.left + rect.width / 2 - tooltip.offsetWidth / 2) + 'px';
tooltip.style.top = (rect.top - tooltip.offsetHeight - 8) + 'px'; // 8px gap
// If tooltip goes above viewport, flip to below
if (parseFloat(tooltip.style.top) < 0) {
    tooltip.style.top = (rect.bottom + 8) + 'px';
}
```

### Anti-Patterns to Avoid
- **Per-element opacity toggling instead of container class:** D-04 explicitly requires container-level dimming. Do not loop through elements to set opacity individually at cycle start.
- **localStorage for cached values:** D-13 explicitly forbids localStorage for cached values. In-memory only.
- **Tooltip per element:** Do not create a tooltip element for each value. Use a single reusable tooltip repositioned on hover.
- **Cache clearing on section switch:** D-11 says cache is preserved when switching sections. Only clear on disconnect.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Tooltip positioning | Custom position calculation from scratch | `getBoundingClientRect()` + viewport bounds check | Browser API handles scroll offsets, transforms; hand-rolling misses edge cases |
| CSS transition timing | JavaScript animation loops | CSS `transition` property | Hardware-accelerated, simpler, 150ms is well within CSS transition capability |
| Debounced hover | Manual setTimeout management | Simple setTimeout with clearTimeout pattern | Standard pattern, no library needed, but must clear on mouseleave |

## Common Pitfalls

### Pitfall 1: Schema Arrives After Cache Restoration Attempt
**What goes wrong:** Cache restoration runs before `handleSectionSchema` builds the DOM, so `querySelector('[data-register="..."]')` finds nothing.
**Why it happens:** `subscribe` is sent in `navigateToSection()`, but the `section_schema` response arrives asynchronously. If cache restoration is triggered synchronously in `navigateToSection()`, the DOM elements don't exist yet.
**How to avoid:** Restore cache inside `handleSectionSchema` after building the skeleton DOM, not in `navigateToSection()`.
**Warning signs:** Cached values never appear; section always shows em-dashes until fresh values arrive.

### Pitfall 2: Stale Fresh Classes from Previous Cycle
**What goes wrong:** `.data-row-h__value--fresh` classes from the previous cycle are not removed when a new cycle starts, so new cycle's container-level dim has no visible effect (all elements still have `--fresh` overriding the container opacity).
**Why it happens:** The cleanup step at cycle start is forgotten.
**How to avoid:** At every cycle start (before adding `--refreshing` to container), remove all `--fresh` classes: `body.querySelectorAll('.data-row-h__value--fresh').forEach(el => el.classList.remove('data-row-h__value--fresh'))`.
**Warning signs:** After the first successful cycle, subsequent cycles show no dimming.

### Pitfall 3: Pack Drill-Down Cache Key Collision
**What goes wrong:** All pack drill-down pages share the same cache key, so navigating from Pack 3 to Pack 5 and back to Pack 3 shows Pack 5's values.
**Why it happens:** Cache key uses "bms" for all pack pages instead of including pack coordinates.
**How to avoid:** Use composite cache key: `"bms:pack:{input}:{tower}:{pack}"` per D-09.
**Warning signs:** Returning to a previously-viewed pack shows wrong cell voltages.

### Pitfall 4: Composed Values Missing Tooltip Data
**What goes wrong:** Composed values (System time, BMS Clock, SW Version) don't have a single register address, so tooltip shows empty register address.
**Why it happens:** These values are computed from multiple register reads and sent via `NewRegisterValue` with no single source address.
**How to avoid:** Send `register_addr: 0` and empty `raw_value` for composed values. Frontend tooltip logic: if `data-register-addr` is "0x0000" or empty, show only the value name (or omit register address line). Per D-19, "If no value was ever read, just shows the address" -- for composed values, the address is meaningless, so omit the Register line entirely.
**Warning signs:** Tooltip shows "Register: 0x0000" for composed values.

### Pitfall 5: Tooltip Element Orphaned on Section Switch
**What goes wrong:** User hovers a value, tooltip appears, then navigates to another section. The hovered element is destroyed but the tooltip stays visible pointing at nothing.
**Why it happens:** `mouseleave` never fires because the element was removed from DOM (not mouse-left).
**How to avoid:** In `navigateToSection()`, explicitly hide the tooltip (set `display: none`). Also consider hiding on any DOM mutation of `#content-body`.
**Warning signs:** Ghost tooltip visible after section navigation.

### Pitfall 6: NewRegisterValue Signature Change Breaks Tests
**What goes wrong:** Adding parameters to `NewRegisterValue()` breaks all test files that call it.
**Why it happens:** Tests use `NewRegisterValue` with the old 5-parameter signature.
**How to avoid:** Check all call sites including test files. Update test calls to include the new `addr` and `rawValue` parameters.
**Warning signs:** Compile errors in `hub_test.go`.

### Pitfall 7: Disconnect Handler Must Reset Visual State
**What goes wrong:** After disconnect, stale cached values remain visible with dimmed styling instead of showing em-dash skeletons.
**Why it happens:** Cache is cleared but DOM is not reset. Or cache is cleared but `--refreshing` class remains on the container.
**How to avoid:** Disconnect handler must: (1) clear `sectionCache`, (2) remove `--refreshing` class, (3) reset all value elements to em-dash with `--pending` class, (4) hide tooltip.
**Warning signs:** Disconnect shows stale numbers instead of clean em-dash skeletons.

## Code Examples

### Backend: Extended RegisterValueMessage
```go
// Source: internal/hub/message.go (to be modified)
type RegisterValueMessage struct {
    Type         string `json:"type"`
    Section      string `json:"section"`
    Group        string `json:"group"`
    Name         string `json:"name"`
    Value        string `json:"value,omitempty"`
    Error        string `json:"error,omitempty"`
    RegisterAddr uint16 `json:"register_addr"`
    RawValue     string `json:"raw_value,omitempty"`
}

func NewRegisterValue(section, group, name, value string, errStr string, addr uint16, rawValue string) RegisterValueMessage {
    return RegisterValueMessage{
        Type:         MsgTypeRegisterValue,
        Section:      section,
        Group:        group,
        Name:         name,
        Value:        value,
        Error:        errStr,
        RegisterAddr: addr,
        RawValue:     rawValue,
    }
}
```

### Backend: FormatRawValue Utility
```go
// Source: internal/register/format.go (new function)
// FormatRawValue extracts the raw numeric representation of register data
// before scaling or formatting. Returns a decimal string for numeric probes
// or hex string for ASCII probes.
func FormatRawValue(p Probe, data []byte) string {
    if len(data) < 2 {
        return ""
    }
    if p.IsASCII {
        return fmt.Sprintf("%X", data)
    }
    if p.U32 && len(data) >= 4 {
        val := uint32(binary.BigEndian.Uint16(data[:2]))<<16 | uint32(binary.BigEndian.Uint16(data[2:4]))
        return fmt.Sprintf("%d", val)
    }
    val := binary.BigEndian.Uint16(data[:2])
    return fmt.Sprintf("%d", val)
}
```

### Frontend: Cache Structure and Update
```javascript
// Source: web/static/app.js (new code)
var sectionCache = new Map();

function getCacheKey() {
    if (App.activeSection === 'bms' && packViewState.mode === 'pack_detail') {
        return 'bms:pack:' + packViewState.selectedInput + ':' +
               packViewState.selectedTower + ':' + packViewState.selectedPack;
    }
    return App.activeSection;
}

function updateCache(key, entry) {
    var cacheKey = getCacheKey();
    if (!sectionCache.has(cacheKey)) {
        sectionCache.set(cacheKey, new Map());
    }
    sectionCache.get(cacheKey).set(key, entry);
}
```

### Frontend: Tooltip Event Delegation
```javascript
// Source: web/static/app.js (new code)
var tooltipEl = null;
var tooltipTimer = null;

function initTooltip() {
    tooltipEl = document.createElement('div');
    tooltipEl.className = 'param-tooltip';
    tooltipEl.style.display = 'none';
    document.body.appendChild(tooltipEl);

    var body = $('#content-body');
    body.addEventListener('mouseenter', function(e) {
        var target = e.target.closest('.data-row-h__value');
        if (!target) return;
        tooltipTimer = setTimeout(function() {
            showTooltip(target);
        }, 300);
    }, true); // useCapture for delegation

    body.addEventListener('mouseleave', function(e) {
        var target = e.target.closest('.data-row-h__value');
        if (!target) return;
        clearTimeout(tooltipTimer);
        tooltipEl.style.display = 'none';
    }, true);
}

function showTooltip(el) {
    var addr = el.getAttribute('data-register-addr');
    var raw = el.getAttribute('data-register-raw');
    var time = el.getAttribute('data-register-time');
    
    var lines = [];
    if (addr && addr !== '0x0000') lines.push('Register: ' + addr);
    if (raw) lines.push('Raw: ' + raw);
    if (time) lines.push('Last read: ' + time);
    
    if (lines.length === 0) return;
    
    tooltipEl.textContent = '';
    lines.forEach(function(line) {
        var div = document.createElement('div');
        div.textContent = line;
        tooltipEl.appendChild(div);
    });
    
    tooltipEl.style.display = '';
    // Position above element
    var rect = el.getBoundingClientRect();
    tooltipEl.style.left = (rect.left + rect.width / 2 - tooltipEl.offsetWidth / 2) + 'px';
    tooltipEl.style.top = (rect.top + window.scrollY - tooltipEl.offsetHeight - 8) + 'px';
}
```

### Frontend: handleRegisterValue with Cache + Data Attributes
```javascript
// Source: web/static/app.js (modified handleRegisterValue)
function handleRegisterValue(msg) {
    if (msg.section !== App.activeSection) return;
    if (msg.section === 'bms' && packViewState.mode === 'pack_detail') return;

    var key = msg.group + '::' + msg.name;
    var el = document.querySelector('[data-register="' + CSS.escape(key) + '"]');
    if (!el) return;

    // Format timestamp
    var now = new Date();
    var timeStr = now.toTimeString().slice(0, 8); // "HH:MM:SS"

    // Format register address as hex
    var addrHex = '0x' + msg.register_addr.toString(16).toUpperCase().padStart(4, '0');

    if (msg.error) {
        el.classList.add('data-row-h__value--stale');
        el.classList.remove('data-row-h__value--pending');
    } else {
        el.textContent = msg.value || '\u2014';
        el.classList.remove('data-row-h__value--pending', 'data-row-h__value--stale');
        el.classList.add('data-row-h__value--fresh');
    }

    // Set tooltip data attributes (on both success and error)
    el.setAttribute('data-register-addr', addrHex);
    if (msg.raw_value) el.setAttribute('data-register-raw', msg.raw_value);
    el.setAttribute('data-register-time', timeStr);

    // Update cache
    updateCache(key, {
        value: msg.value || '',
        registerAddr: addrHex,
        rawValue: msg.raw_value || '',
        timestamp: timeStr,
        error: !!msg.error
    });
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Full section re-render on each read cycle | Per-register streaming (Phase 7) | Phase 7 | Enables per-register dimming/restoration |
| No stale value display | Error-stale with warning icon (Phase 9) | Phase 9 | Foundation for refresh dimming (distinct from error stale) |
| No value caching | In-memory section cache (Phase 10) | This phase | Enables instant navigation without re-read wait |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `mouseenter`/`mouseleave` with `useCapture: true` provides reliable event delegation for dynamically-created elements | Tooltip pattern | LOW -- if delegation fails, fallback is to bind events after each schema render |
| A2 | CSS `opacity` transitions on dynamically-created elements work smoothly at 150ms | Dimming pattern | LOW -- if janky, can adjust timing or remove transition |
| A3 | `getBoundingClientRect()` returns correct values for elements inside the scrollable content area | Tooltip positioning | LOW -- may need `window.scrollY` adjustment (included in example code) |

**If this table is empty:** N/A -- three low-risk assumptions identified above.

## Open Questions (RESOLVED)

1. **How to handle pack drill-down tooltips given pack_data is batch-rendered (not streamed)?** (RESOLVED)
   - What we know: Pack drill-down currently receives `pack_data` as a single batch message (Phase 11 will add streaming). Cell values, temperatures, and status are all rendered in `renderPackDetail()`.
   - What was unclear: The `pack_data` message uses `PackDataMessage` (not `RegisterValueMessage`), so it doesn't carry per-register addresses. Should this phase add address metadata to `PackGroup.Items`?
   - Resolution: Plan 03 extends `PackGroup` with a new `ItemMeta` field (`map[string]PackItemMeta`) carrying `RegisterAddr` and `RawValue` per item. The `buildPackDataMessage` function in `hub.go` populates this metadata from probe definitions. The frontend `renderGroupCard` and other pack renderers set `data-register-addr`, `data-register-raw`, and `data-register-time` on pack drill-down value elements. Cell voltages use the cell register address (0x9051+index). This delivers full D-15 compliance for pack drill-down views within Phase 10 without waiting for Phase 11 streaming.

2. **Composed value register address representation** (RESOLVED)
   - What we know: Composed values (System time, BMS Clock, SW Version) are synthesized from multiple register reads. They have no single register address.
   - What was unclear: What should the tooltip show for these values?
   - Resolution: Send `register_addr: 0` and empty `raw_value`. Frontend omits Register and Raw lines when addr is 0x0000 (implemented in Plan 02 Task 2 `showTooltip` function). Tooltip shows only "Last read: HH:MM:SS" for composed values.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (stdlib) |
| Config file | none (stdlib testing, no config needed) |
| Quick run command | `go test ./internal/hub/ -run TestRegisterValue -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| DISP-01 | Refresh dimming: container class + per-value restore | manual-only | N/A (CSS visual behavior, no JS test framework) | N/A |
| DISP-02 | Section cache: navigate away and back shows cached values | manual-only | N/A (browser DOM behavior, no JS test framework) | N/A |
| DISP-03 (backend) | RegisterValueMessage includes register_addr and raw_value | unit | `go test ./internal/hub/ -run TestRegisterValue -count=1` | Partial -- hub_test.go exists but needs new test for extended message |
| DISP-03 (backend) | FormatRawValue correctly extracts raw values | unit | `go test ./internal/register/ -run TestFormatRaw -count=1` | No -- new test needed |

### Sampling Rate
- **Per task commit:** `go test ./internal/hub/ ./internal/register/ -count=1`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green + manual browser verification of all 3 DISP requirements

### Wave 0 Gaps
- [ ] `internal/register/format_test.go` -- add `TestFormatRawValue` covering uint16, uint32, ASCII, empty data edge cases
- [ ] `internal/hub/hub_test.go` -- add test verifying `NewRegisterValue` includes `register_addr` and `raw_value` in JSON output
- [ ] No JS test framework exists -- all frontend validation is manual (acceptable per project scope: desktop diagnostic tool)

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | N/A (local diagnostic tool, no auth) |
| V3 Session Management | no | N/A |
| V4 Access Control | no | N/A |
| V5 Input Validation | yes (minimal) | Backend `register_addr` is uint16 from probe definition (trusted). `raw_value` is formatted from read data. Frontend reads data attributes (no injection risk -- `textContent` not `innerHTML`). |
| V6 Cryptography | no | N/A |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| XSS via tooltip content | Tampering | Use `textContent` (not `innerHTML`) for tooltip content. Data attributes are set from backend-controlled values. [VERIFIED: existing code uses textContent throughout] |
| DOM clobbering via data attributes | Tampering | Data attributes come from typed Go structs (uint16, string), not user input. No risk. |

## Project Constraints (from CLAUDE.md)

- **Tech stack:** Go backend, vanilla HTML/JS/CSS frontend. No frameworks, no build tools. [VERIFIED: CLAUDE.md]
- **Deployment:** Single binary, no external dependencies. [VERIFIED: CLAUDE.md]
- **Go stdlib only:** No external Go dependencies. [VERIFIED: go.mod]
- **Naming:** PascalCase exported, camelCase local, verb-first functions. [VERIFIED: CLAUDE.md conventions]
- **Error handling:** Explicit error checking, `fmt.Errorf` wrapping. [VERIFIED: CLAUDE.md]
- **gofmt:** Standard formatting. [VERIFIED: CLAUDE.md]

## Sources

### Primary (HIGH confidence)
- `internal/hub/message.go` -- Current RegisterValueMessage struct (lines 189-196), NewRegisterValue function (lines 222-231)
- `internal/hub/hub_streaming.go` -- All 8 NewRegisterValue call sites with probe metadata available at each
- `internal/register/format.go` -- FormatValue logic showing how raw data is interpreted (data parsing patterns)
- `internal/register/probe.go` -- Probe struct with Addr field (uint16)
- `web/static/app.js` -- handleRegisterValue (line ~994), handleSectionSchema (line ~914), navigateToSection (line ~285), disconnect handler (line ~529)
- `web/static/style.css` -- Existing --stale and --pending classes (lines ~575-588)
- `web/static/index.html` -- Content body element: `<div id="content-body" class="content__body">`
- `.planning/phases/10-data-persistence-tooltips/10-UI-SPEC.md` -- Approved UI design contract with exact CSS, tooltip spec, interaction flows
- `.planning/phases/10-data-persistence-tooltips/10-CONTEXT.md` -- All 19 locked decisions

### Secondary (MEDIUM confidence)
- `internal/hub/hub_test.go` -- Test patterns for hub, mockBroker with registerResults map
- `internal/hub/export_test.go` -- Test helpers (NewTestHub, NewTestClient, SendReadCycle)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- no new libraries, all patterns verified in existing codebase
- Architecture: HIGH -- all integration points verified by reading current source, UI-SPEC approved
- Pitfalls: HIGH -- identified from code analysis of actual call sites and DOM lifecycle
- Backend changes: HIGH -- exact struct and all call sites verified
- Frontend changes: MEDIUM -- patterns verified against existing code, but no JS test infrastructure to validate

**Research date:** 2026-04-12
**Valid until:** 2026-05-12 (stable -- vanilla JS/CSS patterns, Go stdlib)
