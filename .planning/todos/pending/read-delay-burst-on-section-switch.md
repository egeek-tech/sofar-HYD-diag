---
created: 2026-04-11
title: Read delay burst on section switch
area: hub
files:
  - internal/hub/hub_streaming.go
  - internal/broker/broker.go:492-496
---

## Problem

When switching sections, the first several register reads appear much faster than the configured delay (e.g., set to 1000ms but reads at ~100ms pace). This is because `enforceInterReadDelay` checks `time.Since(lastReadTime)` — navigation time counts toward the delay, so no sleep is needed for initial reads after a section switch. Technically correct but feels inconsistent.

Additionally, `SetDelayRuntime` is called in a `go func()` goroutine which may race with the new section read starting immediately via `triggerSectionRead`.

## Solution

Option A: Reset `lastReadTime` to `time.Now()` at the start of each streaming read goroutine, forcing the first read to also respect the delay.

Option B: Move the `SetDelayRuntime` call out of the goroutine so it blocks until the broker acknowledges the delay change before any new reads start.

Option C: Accept as cosmetic — the delay works correctly within a read cycle.
