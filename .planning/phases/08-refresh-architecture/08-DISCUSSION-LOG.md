# Phase 8: Refresh Architecture - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md -- this log preserves the alternatives considered.

**Date:** 2026-04-12
**Phase:** 08-refresh-architecture
**Areas discussed:** Section switch behavior, Refresh cycle delay, Auto-refresh button UX

---

## Folded Todos

| Todo | Decision | Notes |
|------|----------|-------|
| Read delay burst on section switch | Folded into Phase 8 | Directly addressed by REL-03 |
| Skip unsupported PackInfoProbes registers | Folded into Phase 8 | User selected both todos |

---

## Section Switch Behavior

| Option | Description | Selected |
|--------|-------------|----------|
| Immediate read on switch | Browser sends read request right away, then starts delay timer after completion | Yes |
| Wait for next cycle tick | Subscribe and wait for timer to fire | |
| Immediate read, cancel previous | Immediate read for new AND abort in-progress old section read | |

**User's choice:** Immediate read on switch (Recommended)

### Follow-up: Old section read handling

| Option | Description | Selected |
|--------|-------------|----------|
| Let it finish silently | Old read completes, results not broadcast | |
| Abort the old read | Cancel in-progress Modbus read for old section | Yes |

**User's choice:** Abort the old read

### Follow-up: Abort scope

| Option | Description | Selected |
|--------|-------------|----------|
| Cancel remaining probes | Let current register read finish, skip remaining probes | Yes |
| Hard abort mid-read | Cancel even current TCP read, close connection | |

**User's choice:** Cancel remaining probes (Recommended)

### Follow-up: Inter-read delay on switch

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, always enforce delay | Respect inter-read delay from last completed probe | Yes |
| No, immediate first read | Skip delay for first probe of new section | |

**User's choice:** Yes, always enforce delay (Recommended)

---

## Refresh Cycle Delay

| Option | Description | Selected |
|--------|-------------|----------|
| No extra delay | Browser requests next cycle immediately after section_complete | |
| Fixed 10s like current | Keep 10-second gap between cycles | |
| Configurable cycle delay | New setting for gap between cycles | Yes |

**User's choice:** Configurable cycle delay

### Follow-up: Configuration method

| Option | Description | Selected |
|--------|-------------|----------|
| New slider alongside existing | Second slider for cycle delay | |
| Replace fixed interval | Repurpose auto-refresh interval as slider | |
| Dropdown with presets | Preset options instead of free slider | Yes |

**User's choice:** Dropdown with presets

### Follow-up: Preset values

| Option | Description | Selected |
|--------|-------------|----------|
| 0s, 5s, 10s, 30s | Continuous, 5s, 10s (current), 30s | Yes |
| 0s, 2s, 5s, 10s | Faster options for diagnostic use | |
| You decide | Claude picks during implementation | |

**User's choice:** 0s, 5s, 10s, 30s (Recommended)

### Follow-up: Default value

| Option | Description | Selected |
|--------|-------------|----------|
| 0s (Continuous) | Next cycle right after previous finishes | Yes |
| 5s | Brief pause between cycles | |
| 10s (Current behavior) | Matches existing 10s ticker | |

**User's choice:** 0s (Continuous)

### Follow-up: Persistence

| Option | Description | Selected |
|--------|-------------|----------|
| Persist in localStorage | Remembered across reloads | Yes |
| Reset on reload | Always starts at default | |

**User's choice:** Persist in localStorage (Recommended)

### Follow-up: Placement

| Option | Description | Selected |
|--------|-------------|----------|
| Next to auto-refresh button | Inline in header area | Yes |
| In timing controls area | Alongside read delay slider | |

**User's choice:** Next to auto-refresh button (Recommended)

---

## Auto-Refresh Button UX

| Option | Description | Selected |
|--------|-------------|----------|
| Just 'Auto' on/off | Simple toggle, delay shown in dropdown | |
| 'Auto' with live cycle count | Show 'Auto (#3)' with incrementing count | Yes |
| 'Auto' with countdown | Show 'Auto (3s)' counting down | |

**User's choice:** 'Auto' with live cycle count

### Follow-up: Count reset behavior

| Option | Description | Selected |
|--------|-------------|----------|
| Reset on section switch | Counter starts at #1 for each new section | Yes |
| Running total | Global counter for session | |

**User's choice:** Reset on section switch (Recommended)

### Follow-up: Manual refresh button

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, show Refresh button | Manual trigger when auto is off | Yes |
| No, auto toggle only | Just on/off | |

**User's choice:** Yes, show Refresh button (Recommended)

---

## Claude's Discretion

- WebSocket message format for new "read now" / "cancel read" protocol
- Probe cancellation implementation (context, channel, or flag)
- Whether subscribe and read become separate messages
- Internal broker changes for cancellation support

## Deferred Ideas

None -- discussion stayed within phase scope.
