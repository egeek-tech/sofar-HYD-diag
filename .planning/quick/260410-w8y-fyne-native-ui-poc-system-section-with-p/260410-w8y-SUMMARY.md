---
phase: quick
plan: 01
subsystem: ui
tags: [fyne, desktop, native-gui, modbus, streaming]

# Dependency graph
requires:
  - phase: 01-foundation
    provides: broker and register packages for Modbus communication
provides:
  - Fyne v2 native desktop PoC with per-parameter streaming UI
  - Validation of native desktop approach as alternative to web UI
affects: []

# Tech tracking
tech-stack:
  added: [fyne.io/fyne/v2 v2.7.3]
  patterns: [per-parameter streaming via individual broker.ReadRegisters calls, composite time register accumulation]

key-files:
  created: [cmd/fyne-poc/main.go]
  modified: [go.mod, go.sum, .gitignore]

key-decisions:
  - "Used layout.NewFormLayout for name-value pairs instead of widget.Form for simpler label-only rows"
  - "Composite system time: all 6 time registers map to shared label, updated only when all 6 accumulated"
  - "canvas.Circle for status indicator with color-coded states (green/yellow/red/gray)"

patterns-established:
  - "Per-parameter streaming: iterate probes sequentially, update each label after individual ReadRegisters call"
  - "Time register accumulation: read individually but compose into single display row"

requirements-completed: []

# Metrics
duration: 4min
completed: 2026-04-10
---

# Quick Task 260410-w8y: Fyne Native UI PoC Summary

**Fyne v2 native desktop PoC streaming System section parameters one-by-one from Sofar HYD inverter via broker.ReadRegisters**

## Performance

- **Duration:** 4 min
- **Started:** 2026-04-10T21:15:37Z
- **Completed:** 2026-04-10T21:20:16Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- Native Fyne v2 desktop window with scrollable System section (Identity, Firmware, Status, Temperatures, Protection)
- Per-parameter streaming: each register value updates individually with ~500ms cadence, visible one-by-one
- Connect/Disconnect toggle with color-coded status indicator reflecting all 5 broker states
- System time composes 6 individual register reads into single datetime string
- Running state shows enum label (e.g. "Grid-connected") not raw number, via register.FormatValue with Enum map

## Task Commits

Each task was committed atomically:

1. **Task 1: Add Fyne v2 dependency** - `51bdd07` (chore)
2. **Task 2: Create Fyne PoC with per-parameter streaming UI** - `869d85e` (feat)

## Files Created/Modified
- `cmd/fyne-poc/main.go` - Standalone Fyne v2 native desktop PoC (403 lines)
- `go.mod` - Added fyne.io/fyne/v2 v2.7.3 and transitive dependencies
- `go.sum` - Updated with all Fyne dependency checksums
- `.gitignore` - Added /fyne-poc binary pattern

## Decisions Made
- Used `layout.NewFormLayout()` for name-value pairs -- provides clean two-column alignment without needing widget.Form submit handling
- Composite system time: all 6 time registers (0x042C-0x0431) share a single label, updated only after all 6 values accumulated
- `canvas.Circle` for status indicator with `layout.NewCustomPaddedLayout` for consistent sizing
- Polling goroutine uses context cancellation for clean stop on disconnect/shutdown

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Fixed missing go.sum entries for Fyne transitive dependencies**
- **Found during:** Task 2 (build verification)
- **Issue:** `go get fyne.io/fyne/v2@latest` added the direct dependency but not all transitive go.sum entries
- **Fix:** Ran `go mod tidy` to resolve all transitive dependencies
- **Files modified:** go.mod, go.sum
- **Verification:** `go build ./cmd/fyne-poc/` succeeds
- **Committed in:** 869d85e (Task 2 commit)

**2. [Rule 1 - Bug] Fixed canvas.Circle.SetMinSize compile error**
- **Found during:** Task 2 (build verification)
- **Issue:** `canvas.Circle` in Fyne v2.7.3 does not have `SetMinSize` method
- **Fix:** Removed `SetMinSize` call, used `layout.NewCustomPaddedLayout` wrapper for sizing
- **Files modified:** cmd/fyne-poc/main.go
- **Verification:** `go build ./cmd/fyne-poc/` and `go vet ./cmd/fyne-poc/` pass
- **Committed in:** 869d85e (Task 2 commit)

**3. [Rule 1 - Bug] Fixed .gitignore pattern matching cmd/fyne-poc/ directory**
- **Found during:** Task 2 (git commit)
- **Issue:** `fyne-poc` gitignore pattern also matched `cmd/fyne-poc/` directory, preventing git add
- **Fix:** Changed to `/fyne-poc` (root-only anchor) to only match the built binary
- **Files modified:** .gitignore
- **Verification:** `git add cmd/fyne-poc/main.go` succeeds
- **Committed in:** 869d85e (Task 2 commit)

---

**Total deviations:** 3 auto-fixed (2 bug fixes, 1 blocking)
**Impact on plan:** All auto-fixes necessary for successful compilation and commit. No scope creep.

## Issues Encountered
None beyond the auto-fixed items above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Fyne v2 validated as viable native desktop UI framework
- Per-parameter streaming UX confirmed (~500ms per register visible update cadence)
- Can extend to other sections (Grid, EPS, PV, Battery) by adding more ProbeGroup iterations
- May want to add configurable inverter address fields in the UI (currently CLI-only)

## Self-Check: PASSED

- cmd/fyne-poc/main.go: FOUND (403 lines, min 200 required)
- Commit 51bdd07 (Task 1): FOUND
- Commit 869d85e (Task 2): FOUND
- go build ./cmd/fyne-poc/: PASSED
- go vet ./cmd/fyne-poc/: PASSED

---
*Quick task: 260410-w8y*
*Completed: 2026-04-10*
