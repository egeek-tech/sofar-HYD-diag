# Phase 10: Data Persistence & Tooltips - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-12
**Phase:** 10-data-persistence-tooltips
**Areas discussed:** Stale vs refreshing visuals, Section cache behavior, Tooltip content & trigger, Cache lifetime & clearing, Disconnect display state, Dim/un-dim animation, Tooltip visual style

---

## Stale vs Refreshing Visuals

| Option | Description | Selected |
|--------|-------------|----------|
| Dim text only | Reduce opacity 50%, no icon. Distinct from error-stale ⚠ | ✓ |
| No visual change | Values stay full opacity, replaced in-place | |
| Subtle background pulse | CSS animation on section while reading | |

**User's choice:** Dim text only
**Notes:** Recommended option. Clear distinction: refreshing = dim only, error = dim + ⚠ icon.

| Option | Description | Selected |
|--------|-------------|----------|
| Immediately on cycle start | Dim as soon as browser sends read_cycle | ✓ |
| Only after a short delay | Wait ~1-2s before dimming | |

**User's choice:** Immediately on cycle start
**Notes:** Instant visual feedback that refresh is happening.

| Option | Description | Selected |
|--------|-------------|----------|
| Entire section at once | One class on section container | ✓ |
| Per-value progressive | Each value individually dimmed/restored | |

**User's choice:** Entire section at once
**Notes:** Simpler implementation, consistent visual.

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, same behavior | Pack drill-down uses same dimming | ✓ |
| No, packs are different | Main sections only | |

**User's choice:** Yes, same behavior
**Notes:** Consistent across all views. Aligns with Phase 11 streaming.

| Option | Description | Selected |
|--------|-------------|----------|
| Keep both | Per-register restoration + section completion flash | ✓ |
| Replace flash with un-dim | Drop green flash, un-dim is the feedback | |
| You decide | Claude's discretion | |

**User's choice:** Keep both
**Notes:** Two layers of feedback coexist.

---

## Section Cache Behavior

| Option | Description | Selected |
|--------|-------------|----------|
| Cached values, dimmed | Last-known values at 50% opacity, auto-refresh starts | ✓ |
| Cached values, full opacity | Full opacity, replaced as fresh data arrives | |
| Em-dash skeleton (current) | Current behavior, clean slate | |

**User's choice:** Cached values, dimmed
**Notes:** See data immediately, know it's old, watch it update.

| Option | Description | Selected |
|--------|-------------|----------|
| Per-section + per-pack | Cache each main section AND each pack drill-down | ✓ |
| Per-section only | Main sections only, packs always fresh | |
| You decide | Claude's discretion | |

**User's choice:** Per-section + per-pack
**Notes:** Most valuable for battery diagnostics — comparing across packs.

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, em-dashes for first visit | Current behavior preserved | ✓ |
| Show 'Loading...' message | New UI state | |

**User's choice:** Yes, em-dashes for first visit
**Notes:** Em-dash = never read, dimmed = previously read. Clear mental model.

---

## Tooltip Content & Trigger

| Option | Description | Selected |
|--------|-------------|----------|
| Address + raw value | Register address hex + raw value. Matches DISP-03 | ✓ |
| Address + raw + data type | Also include U16/S16/ASCII and scale factor | |
| Address + raw + register count | Also include how many registers the parameter spans | |

**User's choice:** Address + raw value
**Notes:** Concise and useful for diagnostics. User later added: also include last successful read timestamp (HH:MM:SS).

| Option | Description | Selected |
|--------|-------------|----------|
| Hover only | CSS/JS tooltip on mouse hover | ✓ |
| Click to show/dismiss | Click to toggle | |
| Hover with click-to-pin | Hover shows, click pins | |

**User's choice:** Hover only
**Notes:** Zero friction, standard desktop pattern.

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, all values | Every parameter in the app gets a tooltip | ✓ |
| Main sections only | Not on dense pack drill-down grids | |
| You decide | Claude's discretion | |

**User's choice:** Yes, all values
**Notes:** Consistent behavior everywhere including cell voltages/temps.

| Option | Description | Selected |
|--------|-------------|----------|
| Show last known address + raw | Register address + last successful raw value | ✓ |
| Show address + error info | Address + error message | |
| You decide | Claude's discretion | |

**User's choice:** Show last known address + raw
**Notes:** See WHICH register is failing. If no value ever read, just show address.

### Tooltip timestamp (user-initiated addition)

| Option | Description | Selected |
|--------|-------------|----------|
| Time only — HH:MM:SS | Just time of day, compact | ✓ |
| Relative time — '5s ago' | How long ago, requires timer | |
| Both time + relative | Most verbose | |

**User's choice:** Time only — HH:MM:SS
**Notes:** User specifically requested adding last successful read timestamp to tooltip.

---

## Cache Lifetime & Clearing

| Option | Description | Selected |
|--------|-------------|----------|
| On disconnect | Cache cleared on disconnect, fresh start on reconnect | ✓ |
| On page reload only | Cache persists across disconnect/reconnect | |
| Never (localStorage) | Persist to localStorage | |

**User's choice:** On disconnect
**Notes:** Prevents stale data from a previous inverter connection.

| Option | Description | Selected |
|--------|-------------|----------|
| No manual button | Disconnect handles clearing | ✓ |
| Add a clear button | Manual cache wipe UI | |

**User's choice:** No manual button
**Notes:** Keep interface clean.

| Option | Description | Selected |
|--------|-------------|----------|
| Preserve cached values | Old section values stay cached until disconnect | ✓ |
| Mark as extra-stale | Additional visual hint for old cached values | |

**User's choice:** Preserve cached values
**Notes:** Core DISP-02 behavior.

---

## Disconnect Display State

| Option | Description | Selected |
|--------|-------------|----------|
| Clear to em-dashes | All values reset to em-dash skeletons | ✓ |
| Keep last values dimmed | Show last values dimmed after disconnect | |
| Show 'Disconnected' overlay | Overlay message on top of section | |

**User's choice:** Clear to em-dashes
**Notes:** Clean slate signals "no active connection, no data".

---

## Dim/Un-dim Animation

| Option | Description | Selected |
|--------|-------------|----------|
| Subtle fade | CSS transition ~150-200ms | ✓ |
| Instant toggle | No transition, snap between states | |
| You decide | Claude's discretion | |

**User's choice:** Subtle fade
**Notes:** Smooth visual effect for dim/un-dim transitions.

---

## Tooltip Visual Style

| Option | Description | Selected |
|--------|-------------|----------|
| Styled custom tooltip | Dark theme, monospace, positioned above value | ✓ |
| Browser native title | HTML title attribute | |
| You decide | Claude's discretion | |

**User's choice:** Styled custom tooltip
**Notes:** Matches app's dark theme, monospace for register data.

## Claude's Discretion

- How to send register address and raw value from backend to frontend
- Internal cache data structure in JavaScript
- Custom tooltip implementation (CSS-only vs JS-managed)
- Per-register timestamp storage in cache
- CSS class naming for refreshing state vs error-stale state

## Deferred Ideas

None — all discussion stayed within phase scope.
