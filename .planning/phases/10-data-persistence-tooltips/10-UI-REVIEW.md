# Phase 10 — UI Review

**Audited:** 2026-04-12
**Baseline:** 10-UI-SPEC.md (approved 2026-04-12)
**Screenshots:** Not captured (Playwright CLI unavailable; dev server detected on port 8080)

---

## Pillar Scores

| Pillar | Score | Key Finding |
|--------|-------|-------------|
| 1. Copywriting | 4/4 | All tooltip strings match spec exactly; empty/error states unchanged and correct |
| 2. Visuals | 3/4 | Sidebar toggle lacks accessible name; tooltip position:fixed vs spec position:absolute is a justified deviation |
| 3. Color | 3/4 | Two hardcoded colors outside :root in auto-refresh and manual refresh button (#e0e0e0, #666666); all Phase 10 tokens correct |
| 4. Typography | 4/4 | All four font sizes (12/14/16/18px) within declared scale; tooltip 12px/400/1.4 matches spec exactly |
| 5. Spacing | 4/4 | All Phase 10 new spacing (8px 16px tooltip padding) on declared scale; off-scale values pre-date Phase 10 |
| 6. Experience Design | 4/4 | Dimming, caching, tooltips, disconnect cleanup all fully implemented; loading, error, empty states covered |

**Overall: 22/24**

---

## Top 3 Priority Fixes

1. **Sidebar toggle has no accessible name** — Screen reader users hear "button" with no label; visually shows `«` chevron only — Add `aria-label="Collapse sidebar"` to `<button id="sidebar-toggle">` in `web/static/index.html` line 87, and update the JS toggle handler to set it to `"Expand sidebar"` when collapsed.

2. **Two hardcoded non-token colors in button styles** — `#e0e0e0` and `#666666` on `.btn-auto-refresh` and `.btn-refresh` (lines 500-501, 519-520 of style.css) are not CSS custom properties; a future theme or dark-mode change would miss them — Extract to `:root` as `--btn-neutral-bg: #e0e0e0` and `--btn-neutral-text: #666666`, then reference those tokens in both button classes.

3. **UI-SPEC declares `position: absolute` for `.param-tooltip` but implementation uses `position: fixed`** — The spec's CSS block is technically incorrect (the approach rationale in both the plan and implementation notes is sound: tooltip is appended to `<body>` and uses `getBoundingClientRect` for viewport-relative coordinates, requiring `fixed`) — Update the UI-SPEC's `.param-tooltip` code block to reflect `position: fixed` so the spec matches implementation, preventing future confusion.

---

## Detailed Findings

### Pillar 1: Copywriting (4/4)

All Phase 10 copywriting obligations met or exceeded.

**Tooltip content strings** match the spec contract exactly (10-UI-SPEC.md Copywriting Contract section):
- `web/static/app.js:1043` — `'Register: ' + addr` matches spec `Register: 0x0484`
- `web/static/app.js:1044` — `'Raw: ' + raw` matches spec `Raw: 3852`
- `web/static/app.js:1045` — `'Last read: ' + time` matches spec `Last read: 16:03:42`

**Composed value suppression** correct: lines 1042-1043 omit the Register line when `addr === '0x0000'`, matching CONTEXT.md D-19 and UI-SPEC "Tooltip for special states" table.

**Empty state copy** unchanged: `web/static/index.html:120` — "Enter your inverter connection details in the sidebar and click Connect." matches spec.

**Disconnected state** label: `index.html:94` — "Not Connected" matches spec.

No generic labels found ("Submit", "Click Here", "OK", "Cancel", "Save" — grep returned zero matches).

---

### Pillar 2: Visuals (3/4)

**Pass: Tooltip visual hierarchy.** The tooltip uses `position: fixed`, `z-index: 100`, dark background `#1a1a2e` (matching sidebar for visual coherence), monospace font — creates clear differentiation from body content. Arrow indicator direction (above/below) implemented correctly with `--below` modifier class.

**Pass: Refresh dimming feedback.** Container-level dim (`opacity: 0.5`) with per-register progressive restore (`opacity: 1 !important`) creates a clear "stale" vs "fresh" visual signal. Coexistence with green flash on `section_complete` preserved.

**Pass: Section navigation icons.** All 7 nav items use Unicode emoji as icon + visible text label. No icon-only navigation items.

**Finding: Sidebar toggle button has no accessible name.**
`web/static/index.html:87-89` — The collapse/expand button contains only `«`/`»` HTML entity characters with no `aria-label`, `title`, or screen-reader text. This is a pre-existing issue carried through all phases. For a desktop diagnostic tool the practical impact is low, but it is a clear omission given the spec's Accessibility Notes section.

**Finding: UI-SPEC vs implementation divergence on tooltip position.**
The `10-UI-SPEC.md` `.param-tooltip` CSS code block (lines 271-287) specifies `position: absolute`, but `web/static/style.css:1172` correctly implements `position: fixed`. The plan (10-02-PLAN.md) documents the rationale: tooltip is appended to `<body>` and uses `getBoundingClientRect()` for viewport coordinates, requiring `fixed`. The implementation is correct; the spec contains an error that should be updated for documentation accuracy.

---

### Pillar 3: Color (3/4)

**Phase 10 tokens: correct.** All 9 new custom properties declared in `:root` as specified:
- `--tooltip-bg: #1a1a2e` reuses `--color-sidebar` value (intentional, per spec note)
- `--tooltip-text: #f0f0f0`
- `--refresh-dim-opacity: 0.5`
- `--refresh-dim-transition: 150ms`
- Five additional tooltip tokens all present

**60/30/10 split: preserved.** Phase 10 adds no new surface colors. Tooltip background reuses the existing dark sidebar palette. Accent `#4a90d9` used on 10 elements in style.css (lines 305, 344, 389, 425, 505, 577, 665, 833, 979, 1000) — all declared uses per spec (active nav border, auto-refresh pill, focus rings, breadcrumb links, spinner).

**Finding: Two hardcoded neutral colors outside `:root`.**
`web/static/style.css:500-501` — `.btn-auto-refresh` uses `background-color: #e0e0e0` and `color: #666666` directly (not via custom properties).
`web/static/style.css:519-520` — `.btn-refresh` repeats the same hardcoded values.
Line 398 also has `background-color: #d0d0d0` for the hover state.

These pre-date Phase 10 but are within the audited files. They are not new regressions but remain a theming liability. The Phase 10 CSS additions correctly use custom properties throughout.

**No hardcoded colors in Phase 10 additions.** The `.param-tooltip`, `.content__body--refreshing`, and `.data-row-h__value--fresh` blocks all use `var(--...)` references with appropriate fallbacks.

---

### Pillar 4: Typography (4/4)

**Four distinct font sizes in use:** 12px, 14px, 16px, 18px.
- The spec declares: body 14px, label 12px, heading 20px, tooltip 12px.
- 16px maps to `--font-size-data-large` (cell voltages, from Phase 5).
- 18px is the sidebar toggle icon size only (one instance, line 417 of style.css).
- 20px heading is only declared as a custom property and used via `var(--font-size-heading)`.

These are all within 4 distinct sizes and consistent with prior phase declarations.

**Tooltip typography:** `web/static/style.css:1177-1179` — font-size 12px, font-weight 400, line-height 1.4 match spec exactly. Monospace font-family via `var(--tooltip-font)` correct.

**Font weights in use:** 400 (regular) and 600 (semibold) — exactly two, matching the declared design system.

---

### Pillar 5: Spacing (4/4)

**Phase 10 new spacing:** Tooltip padding `8px 16px` = `--space-sm` / `--space-md` — on the declared scale. The 8px gap between tooltip and target element (`top = rect.top - tipHeight - 8`) is also `--space-sm`.

**Off-scale values** exist in the codebase (e.g., `gap: 4px`, `gap: 24px`, `padding: 4px 8px` in Phase 4-5 components) but these pre-date Phase 10 and are outside this phase's scope.

No arbitrary `[Npx]` Tailwind-style values (this is a vanilla CSS project, none applicable).

All Phase 10 CSS additions use either custom property references or spacing values that align with the declared scale (`4px = xs`, `8px = sm`, `16px = md`).

---

### Pillar 6: Experience Design (4/4)

**DISP-01 Refresh Dimming:** Fully implemented.
- `applyRefreshDimming()` (app.js:78) removes `--fresh` classes then adds `--refreshing` to container.
- Called at all 3 read-cycle dispatch points: auto-refresh start (line 492), manual refresh (line 506), auto-refresh next cycle (line 1255).
- `handleSectionComplete` (line 1240) removes `--refreshing` as cleanup.
- Per-register restore via `--fresh` class with `!important` override (style.css:610-613).

**DISP-02 Section Caching:** Fully implemented.
- `sectionCache` Map with composite pack keys (app.js:33).
- `restoreFromCache` called from `handleSectionSchema` after DOM build (line 1138-1139, correctly avoiding Pitfall 1).
- Pack drill-down cache populated in `handlePackData` (lines 1550-1567).
- `sectionCache.clear()` on disconnect (line 623).
- Values reset to em-dash skeleton on disconnect (lines 627-634).

**DISP-03 Parameter Tooltips:** Fully implemented.
- `initTooltip()` called in `DOMContentLoaded` (line 223).
- Event delegation with `useCapture` on `#content-body` (lines 1019-1033).
- 300ms delay before show, immediate hide on mouseleave.
- Viewport clamping and flip-below logic correct.
- `aria-describedby` set/removed on show/hide.
- Pack drill-down: cell voltage elements get `data-row-h__value` class to participate in delegation (app.js:1983). `renderGroupCard` sets `data-register-addr` and `data-register-raw` from `item_meta` (lines 790-796). `handlePackData` sets `data-register-time` after render (line 1546).

**Tooltip hide on navigation:** `navigateToSection()` calls `hideTooltip()` and `clearTimeout(tooltipTimer)` (lines 412-413) before navigating.

**Tooltip hide on disconnect:** handled at lines 637-638.

**Minor observation: Pack drill-down cache restoration gap.**
`restoreFromCache` is only called from `handleSectionSchema` (main sections). When a user navigates to BMS overview and then back to a pack drill-down they previously visited, the pack cache exists but there is no path to restore it before the fresh `pack_data` arrives. This is not a bug — the pack data arrives quickly via the existing pack subscribe mechanism — and it is consistent with the declared behavior in CONTEXT.md D-09. No score impact.

---

## Files Audited

- `web/static/style.css` (1204 lines)
- `web/static/app.js` (lines 1-100, 400-640, 775-810, 1005-1262, 1529-1568, 1975-2015)
- `web/static/index.html`
- `.planning/phases/10-data-persistence-tooltips/10-UI-SPEC.md`
- `.planning/phases/10-data-persistence-tooltips/10-CONTEXT.md`
- `.planning/phases/10-data-persistence-tooltips/10-01-PLAN.md`
- `.planning/phases/10-data-persistence-tooltips/10-02-PLAN.md`
- `.planning/phases/10-data-persistence-tooltips/10-03-PLAN.md`
- `.planning/phases/10-data-persistence-tooltips/10-01-SUMMARY.md`
- `.planning/phases/10-data-persistence-tooltips/10-02-SUMMARY.md`
- `.planning/phases/10-data-persistence-tooltips/10-03-SUMMARY.md`
- `internal/hub/hub_streaming.go` (call sites for composed value addr=0 verification)
