# Domain Pitfalls: v1.2 Reliability & UX Refinements

**Domain:** Adding cancellation, retry, stale-value, and streaming fixes to existing Go WebSocket + Modbus TCP diagnostic tool
**Researched:** 2026-04-11
**Confidence:** HIGH (based on direct analysis of 9,334 LOC codebase, existing architecture, and known issues)

**Context:** This is a SUBSEQUENT milestone pitfalls analysis. The v1.0/v1.1 PITFALLS.md covered greenfield architecture risks (request serialization, pack atomicity, connection lifecycle). This document covers pitfalls specific to MODIFYING the existing broker/hub/streaming architecture for v1.2 features.

---

## Critical Pitfalls

Mistakes that cause deadlocks, data corruption, or require architectural rework.

### Pitfall 1: Context Cancellation Deadlock in Broker Command Channel

**What goes wrong:** Adding `context.WithCancel` to abort in-progress section reads creates a deadlock when the cancelled goroutine is blocked sending to `h.results` (capacity 32) while the hub event loop is blocked waiting for the cancellation to complete. The streaming goroutine holds a reference to `h.results`, cancellation fires, but the goroutine is mid-send on the channel. If the hub is processing the cancel signal and not draining `h.results`, deadlock.

**Why it happens in THIS codebase:** The hub's `Run()` loop (hub.go:183-213) uses a single `select` over 7 channels. The `h.results` channel (capacity 32) is drained in the same select. If cancellation is triggered from `handleCommand` (e.g., disconnect or section switch), and the hub loop is executing that handler, it is NOT simultaneously draining `h.results`. The streaming goroutines (`streamStandardRead`, `streamBMSRead`, `streamBatteryRead` in hub_streaming.go) send to `h.results` with no select/context guard -- they just do `h.results <- sectionResult{...}` as a bare send. If 32+ results are buffered and the hub is busy processing disconnect, the goroutine blocks on send and never reaches the `h.ctx.Err()` check that would cause it to exit.

**Specific code path:**
1. `streamStandardRead` goroutine is streaming 30+ registers for "system" section
2. User clicks Disconnect, hub enters `handleCommand` -> broker.Disconnect
3. Hub cancels context, but streaming goroutine is blocked on `h.results <- sectionResult{...}` (line 87-89 of hub_streaming.go)
4. Hub loop returns to select, but the goroutine's defer `sec.reading.Store(false)` never fires
5. `sec.reading` stays true forever, blocking all future reads for that section

**Consequences:** Section permanently shows "loading" after reconnect. The `reading` atomic flag (section.go:32) never clears. Goroutine leak. Must restart the binary.

**Prevention:**
- Every send to `h.results` must be wrapped in a select with `h.ctx.Done()`:
  ```go
  select {
  case h.results <- sectionResult{section: sectionName, msg: msg}:
  case <-h.ctx.Done():
      return
  }
  ```
- Alternatively, introduce a per-section cancellation context (separate from `h.ctx`) that is cancelled on section switch or disconnect. This avoids killing all streaming goroutines when only one section needs to stop.
- The `defer sec.reading.Store(false)` in each streaming goroutine is correctly placed, but only fires if the goroutine actually exits. The bare channel send prevents exit.

**Detection:** Disconnect while a large section (system with faults, BMS with protection) is mid-stream. Check if `sec.reading.Load()` is stuck true. Monitor goroutine count with `runtime.NumGoroutine()`.

**Phase:** Must be the FIRST change in v1.2. Every subsequent feature (retry, abort, stale values) depends on goroutines exiting cleanly on cancel.

---

### Pitfall 2: Stale Data Showing for Wrong Section After Navigation

**What goes wrong:** User navigates from "system" to "grid". The system section's streaming goroutine is still running (it does not get cancelled -- there is no per-section cancel today). It continues sending `register_value` messages for section "system" to `h.results`. The hub broadcasts these to section subscribers. But the client has now subscribed to "grid". The `broadcastResultToSection` function (hub.go:625-642) sends to `sec.subscribers` for "system", which is now empty. So far so good -- the messages are harmlessly dropped.

BUT with the v1.2 stale-value feature: if the frontend caches last-known values in DOM elements keyed by `data-register="groupName::registerName"`, and two different sections have a register with the same group+name key (unlikely but possible if groups are reused), a late-arriving system section value overwrites a grid section value in the DOM.

More likely scenario with stale values: The frontend's `handleRegisterValue` (app.js:905-923) guards with `if (msg.section !== App.activeSection) return;`. This is correct. But if the user switches sections and the new section schema re-renders the DOM, any "stale" values from the previous section that were dimmed are replaced by skeleton em-dashes. Then a late-arriving value from the OLD section (still in flight on the WebSocket) could match a `data-register` selector in the NEW section's DOM if keys collide. The `msg.section` guard prevents this today, but only if the section field is checked.

**Why it matters for v1.2:** The stale-value feature requires keeping old values visible (dimmed) across refresh cycles. If the DOM retains stale values and a section switch does not fully clear them, the user sees wrong-section data dimmed as if it were "stale but valid."

**Consequences:** Confusing UI -- dimmed values that belong to a different section. User thinks a grid parameter is stale when it is actually a system parameter that leaked through.

**Prevention:**
- The existing `handleRegisterValue` guard `if (msg.section !== App.activeSection) return;` is the primary defense. Keep it.
- On section switch, `navigateToSection` already calls `showLoading()` which clears the content body (`body.textContent = ''`). This destroys all stale DOM elements. Ensure this behavior is preserved -- do NOT change section switch to "keep old values and dim them." Only dim values within the SAME section on refresh.
- Stale-value persistence must be scoped: values are stale within one section's refresh cycle, not across section switches. Section switch = fresh start.
- Backend: cancel the old section's streaming goroutine on unsubscribe. Today the goroutine runs to completion even with zero subscribers. The results are dropped by `broadcastResultToSection` (subscriber map is empty), but the goroutine wastes broker reads.

**Detection:** Navigate rapidly between sections while connected. Check if any displayed value belongs to the wrong section by adding a `data-section` attribute alongside `data-register` in the DOM.

**Phase:** Must be designed carefully when implementing stale-value caching. The scope rule (stale = within section, switch = fresh) must be explicit in the implementation.

---

### Pitfall 3: Retry Logic Violating 500ms Modbus Inter-Read Delay

**What goes wrong:** Adding retry logic for failed register reads causes rapid-fire reads that violate the 500ms minimum inter-read delay. The broker's `executeRead` (broker.go:369-404) already has a 2-attempt retry with `enforceInterReadDelay()` called before each attempt. But the retry happens within `executeRead`, and `enforceInterReadDelay` uses `time.Since(b.lastReadTime)`. After a failed read, `b.lastReadTime` was already set (line 387: `b.lastReadTime = time.Now()`). On retry, the elapsed time is near-zero (just the error handling time), so the full delay is enforced. This is correct.

BUT: The v1.2 plan calls for "register read retry" at a HIGHER level -- retrying individual probes that returned errors during a streaming read cycle. If the hub's streaming goroutine (`streamStandardRead`) encounters an error on probe 5 of 30, and the plan is to retry probe 5 later (after completing probes 6-30), the retry read for probe 5 happens naturally after the delay from probe 30. This is fine.

The dangerous pattern: retrying probe 5 IMMEDIATELY after the error, before moving to probe 6. This inserts an extra read into the sequence. The broker's `enforceInterReadDelay` handles the timing correctly (it sleeps if needed), so no timing violation occurs. BUT the total section read time increases because each retry adds at least 500ms. With 5 errors out of 30 probes, the read cycle extends from 15s to 17.5s, potentially overlapping with the next auto-refresh timer tick.

**What actually breaks:** The `sec.reading` atomic flag (section.go:32) prevents overlapping reads (hub.go:366-368). If retries extend the read cycle beyond the 10-second auto-refresh interval, the next timer tick is skipped. This is correct behavior. But the user sees a section that takes 17+ seconds to refresh, with the progress bar stuck.

**Real timing danger:** If retry logic is implemented as a SEPARATE broker call (not within `executeRead`), and the hub streaming goroutine sends a second `ReadRegisters` call for the same probe while the broker is processing the original, the broker's command channel (capacity 32) gets an extra command. Since broker serializes all commands, this is safe, but it burns 500ms+ of bus time on a read that was already attempted.

**Consequences:** Over-throttling of the Modbus bus. Sections take longer to refresh. User perceives the tool as slow.

**Prevention:**
- Retry at the broker level (`executeRead`'s existing 2-attempt pattern) is sufficient. Do NOT add a second retry layer in the hub streaming goroutines.
- If hub-level retry is desired (e.g., retry all failed probes after the main pass), collect failures and retry them in a second pass AFTER the main streaming loop completes. This avoids interleaving retries with normal reads.
- The v1.2 "register read retry" feature should be an enhancement to `executeRead`'s `maxAttempts` constant (currently 2), potentially configurable. Not a new retry loop in the hub.
- Display retry count to the user so they understand why a read took longer.

**Detection:** Set read delay to 500ms, connect, navigate to system section. Count actual reads in broker logs. If retries cause > 2x the expected read count, the retry logic is too aggressive.

**Phase:** Retry logic must be designed as a broker-layer enhancement, not a hub-layer addition. Phase that modifies `executeRead` should document the timing impact.

---

### Pitfall 4: Aborting Mid-Stream Pack Drill-Down Leaves Hub in Inconsistent State

**What goes wrong:** User selects Pack 3 (triggers write 0x9020 + settle + 3 block reads). Mid-way through the settle sleep or the block reads, user clicks "Back to BMS" or switches sections. The pack read goroutine (hub.go:812-853) is running in the background. It writes 0x9020 (changing the BMS query target), then the goroutine receives a cancellation signal. But 0x9020 has ALREADY been written -- the BMS is now pointed at Pack 3. If the BMS overview section immediately starts reading BMS info registers, those registers still reflect Pack 3's state until the BMS naturally resets.

**Why it happens in THIS codebase:** The `triggerPackRead` goroutine (hub.go:812-853) does not check context between steps. The sequence is:
1. `h.broker.WriteRegister(h.ctx, 0x9020, queryWord)` -- writes pack selection
2. `time.Sleep(packSettleMs)` -- waits for BMS (NOT cancellable)
3. `h.broker.ReadBatch(h.ctx, rtReads)` -- reads RT data
4. `h.broker.ReadBatch(h.ctx, infoReads)` -- reads info data (may fail, 0x9104 illegal address)
5. `h.broker.ReadBatch(h.ctx, temps58Reads)` -- reads temps
6. `h.sendPackDataToClient(client, msg)` -- sends to client

If context is cancelled after step 1, step 2 (`time.Sleep`) is NOT interrupted because `time.Sleep` does not respect context. The goroutine sleeps for the full settle time, then step 3's `ReadBatch` will return `ctx.Err()`. But the BMS is now pointed at Pack 3.

**Hub state issue:** The `h.selectedPack` field (hub.go:57) is set in `handleSelectPack` (hub.go:796) BEFORE the goroutine starts. If the user navigates away, `subscribeClient` sets `h.selectedPack = nil` (hub.go:312). But the goroutine is still running. When it completes (or fails), it calls `h.sendPackDataToClient` which sends a `PackDataMessage`. The client may have already switched sections, so the message arrives at the wrong time. The `handlePackData` handler in JS checks nothing about current view state -- it always sets `packViewState.mode = 'pack_detail'` (app.js:1180).

**Consequences:** Late-arriving pack data overwrites the BMS overview that the user navigated to. Pack detail view flashes on screen unexpectedly. BMS query target left pointing at a pack instead of the default.

**Prevention:**
- Replace `time.Sleep(packSettleMs)` with a context-aware wait:
  ```go
  select {
  case <-time.After(time.Duration(h.packSettleMs) * time.Millisecond):
  case <-h.ctx.Done():
      return
  }
  ```
- Add context checks between each step of the pack read sequence.
- Gate `sendPackDataToClient` with a check: only send if `h.selectedPack` still matches the goroutine's input/tower/pack. If it does not match (user navigated away), discard the result.
- On the frontend: `handlePackData` should check if `packViewState.mode === 'overview'` and if so, ignore the incoming pack data message.
- Consider writing a "reset" value to 0x9020 (e.g., 0x0000) when cancelling a pack read, to return the BMS to its default state. Verify this does not cause BMS errors.

**Detection:** Select a pack, immediately click "Back to BMS." Check if pack data appears briefly after the BMS overview loads.

**Phase:** This should be addressed in the pack drill-down streaming phase. The abort path must be designed alongside the streaming implementation.

---

### Pitfall 5: Auto-Refresh State Desync Between Frontend Toggle and Backend Timer

**What goes wrong:** The v1.2 plan removes backend auto-refresh and makes it browser-only. But the existing architecture has auto-refresh state in TWO places:
1. Frontend: `App.autoRefresh` boolean (app.js:119)
2. Backend: `sec.autoRefresh` boolean per section (section.go:28) + per-section ticker goroutines

Today, navigating to a section sends TWO messages: `subscribe` (app.js:341) and `auto_refresh` with the current toggle state (app.js:343-348). These are SEPARATE WebSocket messages. Race condition: if the subscribe triggers an immediate read (hub.go:332: `h.triggerSectionRead(sectionName)`), and the auto_refresh message arrives AFTER the subscribe, the timer may start before the auto_refresh "enabled: false" message arrives. Result: a timer tick fires between subscribe and the auto_refresh toggle.

With v1.2 "browser-only refresh trigger": if the backend timer is removed entirely and the browser sends explicit `refresh` messages on a `setInterval`, a different race appears. If the browser sends `refresh` while the previous read is still in progress (`sec.reading` is true), the refresh is silently dropped (hub.go:367: `sec.reading.Load()` check). The browser's interval does not know this happened. The user sees a missed refresh cycle with no feedback.

**Why it happens in THIS codebase:** The auto-refresh system was designed as backend-driven with frontend toggle. Switching to frontend-driven requires changing the timing authority. The `handleTimerTick` function (hub.go:356-380) has sophisticated skip-if-reading logic that a browser-side `setInterval` cannot replicate because it does not know when reads complete.

**Consequences:** After reconnection, auto-refresh may not restart. Timer fires when user has toggled off. Missed refresh cycles confuse the user.

**Prevention:**
- Keep the refresh timer on the backend (section.go timers) but make it respond to an explicit frontend trigger. The backend timer should NOT auto-start on subscribe. Instead:
  1. Client subscribes -- backend sends schema + triggers ONE immediate read
  2. Client sends `auto_refresh` with `enabled: true` -- backend starts timer
  3. Client sends `auto_refresh` with `enabled: false` -- backend stops timer
- If moving to browser-driven refresh: the browser must receive `section_complete` before scheduling the next refresh. Use `handleSectionComplete` to set a timeout for the next refresh, NOT a fixed interval:
  ```javascript
  function handleSectionComplete(msg) {
      // ... existing code ...
      if (App.autoRefresh) {
          setTimeout(() => {
              App.ws.send({ type: 'refresh', section: App.activeSection });
          }, 10000); // 10s after COMPLETION, not fixed interval
      }
  }
  ```
- The backend should acknowledge `refresh` commands even when `sec.reading` is true, sending a "refresh_skipped" message so the browser knows to retry.
- On reconnect (WSClient.onopen, app.js:50-71): re-send both subscribe AND auto_refresh state. The existing code does `navigateToSection(App.activeSection)` which sends both. Verify this ordering is preserved.

**Detection:** Connect, navigate to system, toggle auto-refresh off, disconnect, reconnect. Check if auto-refresh is still off. Toggle on, switch sections rapidly. Check if timer is running for the correct section.

**Phase:** This is the foundational change for v1.2. The auto-refresh architecture decision (backend timer vs browser interval) propagates to every other feature.

---

## Moderate Pitfalls

### Pitfall 6: Stale Value Cache Showing Wrong Pack's Data

**What goes wrong:** User views Pack 3 drill-down, sees cell voltages. Navigates to Pack 7. During Pack 7's write-settle-read cycle (3+ seconds), the UI shows dimmed "stale" values from Pack 3. The user sees Pack 3's cell voltages under the "Pack 7" heading, dimmed but readable. They may interpret this as Pack 7's previous values.

**Why it happens:** The stale-value feature preserves last-known values across refresh cycles. If the pack drill-down view reuses the same DOM structure (same `data-register` keys for cell voltages regardless of which pack), previous pack's values persist in the DOM until the new pack's values arrive.

**Prevention:**
- On pack switch, clear ALL cached values and show skeleton loaders (em-dashes), not dimmed old values. Stale values are only valid within the same pack's refresh cycle.
- The scope rule: stale values persist across REFRESH of the same view. Stale values do NOT persist across VIEW CHANGES (section switch or pack switch).
- In the frontend, `showPackLoading()` (app.js:1276-1299) already clears `body.textContent = ''`. Ensure this is called before any stale-value caching logic runs.

**Detection:** Navigate from Pack 3 to Pack 7. During the loading period, check if any Pack 3 values are visible.

**Phase:** Stale-value implementation phase. The scope rule must be enforced in the frontend.

---

### Pitfall 7: enforceInterReadDelay Burst on Section Switch (Existing Bug)

**What goes wrong:** This is a documented known issue (todo: `read-delay-burst-on-section-switch.md`). When switching sections, `enforceInterReadDelay` (broker.go:493-498) checks `time.Since(b.lastReadTime)`. If the user spent 5 seconds navigating, the elapsed time exceeds the delay, so the first N reads fire with no delay between them. The reads are correct (broker serializes them), but the visual effect is a burst of rapid values followed by the normal 500ms cadence.

**Why it matters for v1.2:** The timing enforcement fix is a v1.2 target feature. The todo proposes three options. The wrong fix can cause over-throttling.

**Dangerous fix:** Resetting `lastReadTime = time.Now()` at the start of each streaming goroutine (todo Option A). This forces a full delay before the FIRST read, even though no Modbus operation has happened recently. If the delay is 1000ms, the user waits 1 second after clicking a section before seeing any data. This feels sluggish.

**Correct fix:** The burst is actually correct behavior -- if the bus has been idle for 5 seconds, there is no need to wait 500ms before the first read. The "inconsistent" feeling is cosmetic. Option C (accept as cosmetic) is the safest choice.

**If a fix is required:** Reset `lastReadTime` only if the PREVIOUS streaming goroutine for ANY section was cancelled mid-stream (meaning the bus was recently active). Do NOT reset on section switch where the bus has been idle.

**Consequences of wrong fix:** Over-throttling adds 500ms-1000ms latency to every section navigation. Users notice immediate degradation.

**Prevention:**
- Do not reset `lastReadTime` unconditionally at goroutine start.
- If fixing the burst, use a separate "busIdleSince" timestamp that is set when a streaming goroutine completes. Only skip the delay if `busIdleSince` is far enough in the past.
- The `SetDelayRuntime` race (mentioned in the todo) is real: the `go func()` wrapper around `h.broker.SetDelayRuntime` (hub.go:743-746) can race with the immediate `triggerSectionRead`. Fix: call `SetDelayRuntime` synchronously in `handleConfigure` (it blocks until the broker processes the command), or queue the section read after the delay update completes.

**Detection:** Set read delay to 1000ms. Navigate between sections. Time the first 3 reads. If all 3 fire within 100ms, the burst exists. If the first read takes 1000ms, over-throttling exists.

**Phase:** Timing enforcement fix phase. This is a low-risk cosmetic issue -- do not break timing correctness trying to fix appearance.

---

### Pitfall 8: Tooltip Positioning in Dense Data Grid Overflows Viewport

**What goes wrong:** Adding tooltips (hover to see register address and raw value) to a dense data grid causes tooltips to overflow the viewport edge. In the cell voltage grid (16 cells in a 4x4 grid, app.js:1586-1609), cells near the right edge of the screen render tooltips that extend beyond the viewport. The tooltip is clipped or causes horizontal scroll.

**Why it matters:** The cell voltage grid has small cells (each showing "Cell N" + voltage). Adding a tooltip with "0x9051 (raw: 3285)" requires ~150px width. If the grid is right-aligned or the viewport is narrow (diagnostic tool on a laptop), the rightmost column's tooltips overflow.

**Prevention:**
- Use dynamic positioning: check if the tooltip would overflow the viewport right/bottom edge, and flip it to the left/top if needed. CSS `position: fixed` with JavaScript bounds checking.
- Alternatively, use a single floating tooltip element that is positioned by mousemove events, rather than per-element tooltips. This avoids DOM bloat (16 cells x 1 tooltip = 16 extra elements) and allows centralized positioning logic.
- Do NOT use the `title` attribute -- it is unstyled, has a delay, and cannot show formatted content (register address in hex).
- For the data rows in standard sections (`data-row-h` elements), tooltips are simpler because rows span the full width. Position tooltip below the row, left-aligned with the value.

**Detection:** Hover over Cell 16 in the cell voltage grid with the browser window at 1024px width. Check if the tooltip is fully visible.

**Phase:** Tooltip implementation phase. Build a reusable tooltip component with viewport-aware positioning before applying it to any section.

---

### Pitfall 9: Pack Drill-Down Streaming Creates Inconsistent Mixed State

**What goes wrong:** Converting pack drill-down from batch to streaming (todo: `stream-pack-drill-down-values-per-register.md`) creates a period where the UI shows PARTIAL pack data. The pack detail view has 5 groups: Pack Info, Cell Voltages, Temperatures, Pack Status, Balance State. If streaming values arrive group-by-group, the user sees cell voltages before temperatures. This is fine visually but creates a problem with the cell voltage statistics: the "Min/Max/Spread" summary (app.js:1562-1582) requires ALL 16 cell values. If only 8 have arrived, the summary shows wrong min/max/spread.

**Additional complication:** The pack read uses block reads (0x9044 count=60 returns all 60 registers in one Modbus response). The todo's Option B (parse and stream individual values from block response) means the data arrives in one Modbus round-trip but is parsed and sent as individual `register_value` messages. This is fast (no extra Modbus delay) but requires the frontend to handle partial cell voltage data correctly.

**The write-settle-read constraint:** Unlike standard sections where streaming means individual `ReadRegisters` calls, pack data MUST use block reads because:
1. The write-settle-read cycle (0x9020 write + 1s settle) is per-pack, not per-register
2. Individual reads would require the BMS to stay on the selected pack for 60+ reads
3. The BMS may time out the pack selection during extended individual-read sequences

**Prevention:**
- Option B from the todo is the correct approach: keep 3 block reads, parse and stream values from each response.
- Frontend must defer summary computation (Min/Max/Spread for cells) until ALL 16 cell values have been received. Use a counter or flag that triggers summary render only when all cells are present.
- Send pack schema BEFORE the write-settle-read cycle so the frontend can render skeletons immediately. The user sees the loading state before the 1-3 second settle wait.
- Send `section_complete` (or a new `pack_complete`) message after all 3 blocks are parsed and streamed.

**Detection:** Select a pack, observe if the cell voltage summary shows incorrect values during the loading period.

**Phase:** Pack drill-down streaming phase. Must handle the partial-data rendering problem explicitly.

---

### Pitfall 10: Disconnect During Broker Reconnection Loop Causes Goroutine Leak

**What goes wrong:** The broker's `ensureConnected` function (broker.go:503-536) enters an infinite reconnection loop with exponential backoff when the connection drops. If the user clicks Disconnect during this loop, the `executeDisconnect` function (broker.go:482-490) runs on the broker's command goroutine. But `ensureConnected` is called from within `executeRead` (broker.go:373), which is called from the hub's streaming goroutine, which sends commands via the broker's command channel. The broker processes one command at a time in `Run()`.

**The sequence:**
1. Streaming goroutine calls `h.broker.ReadRegisters()` (broker.go:144-164)
2. This sends a command to `b.commands` channel and blocks waiting for response
3. Broker's `Run()` receives the command, calls `executeRead` -> `ensureConnected`
4. `ensureConnected` is looping in backoff, sleeping in `time.After(delay)`
5. User clicks Disconnect, hub sends `broker.Disconnect()` which sends to `b.commands`
6. BUT the broker's `Run()` loop is BLOCKED inside `ensureConnected` -- it cannot receive the Disconnect command until reconnection succeeds or ctx is cancelled

**The disconnect command sits in the channel until reconnection succeeds.** The broker is busy-looping on reconnect, not processing new commands. The user sees "Connecting..." indefinitely.

**Prevention:**
- The broker's `ensureConnected` already checks `ctx.Done()` in its select (broker.go:528-530). If the hub cancels the context on disconnect, this breaks the reconnection loop. Verify that `handleCommand` for disconnect (hub.go:229-233) cancels the hub context or the broker context.
- Currently, disconnect sends to the broker (hub.go:230: `h.broker.Disconnect(h.ctx)`), but the broker cannot process it because it is stuck in `ensureConnected`. The fix is to check `b.done` in the `ensureConnected` loop's select:
  ```go
  select {
  case <-ctx.Done():
      // cancelled
  case <-b.done:
      // broker closed
  case <-time.After(delay):
      // try again
  }
  ```
  The existing code already does this -- `b.done` is NOT in the ensureConnected select. Adding it would allow `Close()` to break the loop.
- Alternative: use a separate context for each read operation, cancellable from the hub, rather than relying on the hub-wide `h.ctx`.

**Detection:** Connect to a non-existent IP, let it fail and enter reconnection. Click Disconnect. Measure how long until the UI shows "Disconnected."

**Phase:** Disconnect abort phase. Must verify the broker's reconnection loop is interruptible.

---

## Minor Pitfalls

### Pitfall 11: Section Schema Sent on Every Subscribe Even When Section Data is Cached

**What goes wrong:** Each `navigateToSection` call sends `subscribe` which triggers `subscribeClient` (hub.go:295-336). This sends the section schema (hub.go:316-326) and triggers a full read. If the user toggles rapidly between two sections, each navigation sends schema + full read. The Modbus bus is kept busy reading sections that the user has already seen.

**Prevention:**
- Frontend: cache section data in a JavaScript object keyed by section name. On navigation, show cached data immediately (with stale indicator) and send subscribe for fresh data in the background.
- Backend: if section data was read recently (within auto-refresh interval), send cached results immediately on subscribe instead of triggering a new read.
- This optimization is not critical for v1.2 but should be considered to avoid the "loading spinner on every section switch" UX.

**Detection:** Navigate to system, then grid, then back to system. The system section shows loading spinner instead of cached data.

**Phase:** Stale-value implementation phase. The caching infrastructure could support this optimization.

---

### Pitfall 12: Frontend handleRegisterValue Missing Error-then-Success Transition

**What goes wrong:** The current `handleRegisterValue` (app.js:905-923) handles two cases: error (adds `--stale` class) and success (sets value, removes `--pending` and `--stale`). But with retry logic, the sequence is: first attempt fails (shows stale), retry succeeds (should show fresh value). The code handles this correctly -- the success path removes `--stale`. No bug here.

**However:** If the v1.2 stale-value feature changes the error path to PRESERVE the old value (instead of just adding a class), the success-after-error path must also handle the "old value was from a different register" edge case. Currently, on error, the value text is not changed (line 918: only class is added). On success, value is overwritten (line 920: `el.textContent = msg.value`). This is correct.

**The real danger:** If stale-value implementation stores the LAST GOOD VALUE in a data attribute (e.g., `data-last-value`) and displays it dimmed when the current read fails, the code must clear `data-last-value` on section switch. Otherwise, after reconnect, stale values from the pre-disconnect session appear dimmed.

**Prevention:**
- Use a simple rule: stale values are ONLY the values currently visible in DOM text content. No separate storage. On error, add `--stale` class (dims current value). On success, update text and remove `--stale`.
- On section switch, `showLoading()` clears the DOM. No stale cleanup needed.
- On disconnect, add `--stale` to ALL visible values (mark everything as potentially outdated).

**Detection:** Read a section, disconnect, reconnect, read the same section. Check that stale indicators clear as fresh values arrive.

**Phase:** Stale-value implementation phase.

---

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Severity | Mitigation |
|-------------|---------------|----------|------------|
| Remove backend auto-refresh | Desync between toggle and timer (Pitfall 5) | Critical | Keep backend timer, drive from frontend trigger. Complete-then-schedule, not fixed interval |
| Add context cancellation | Deadlock on h.results channel send (Pitfall 1) | Critical | Wrap every h.results send in select with ctx.Done() |
| Add context cancellation | Broker reconnect loop not interruptible (Pitfall 10) | Moderate | Add b.done to ensureConnected select |
| Abort mid-stream reads | Pack write-then-cancel leaves BMS pointed at pack (Pitfall 4) | Critical | Context-aware sleep, gate sendPackData on selectedPack match |
| Add retry logic | Retry doubling read time, triggering overlap skip (Pitfall 3) | Moderate | Keep retry in broker executeRead, not hub streaming |
| Stale value caching | Wrong section/pack data shown dimmed (Pitfalls 2, 6) | Critical | Scope rule: stale = same view, switch = fresh start |
| Timing enforcement fix | Over-throttling from lastReadTime reset (Pitfall 7) | Moderate | Accept burst as cosmetic, or use busIdleSince |
| Pack drill-down streaming | Partial cell data produces wrong statistics (Pitfall 9) | Moderate | Defer summary until all 16 cells received |
| Tooltip positioning | Overflow in dense cell grid (Pitfall 8) | Minor | Viewport-aware floating tooltip component |

## Recommended Phase Ordering (Based on Pitfall Dependencies)

1. **Context cancellation and abort** (Pitfalls 1, 4, 10) -- foundation for everything else
2. **Auto-refresh architecture** (Pitfall 5) -- changes how all sections are triggered
3. **Timing enforcement fix** (Pitfall 7) -- quick fix, low risk if Option C chosen
4. **Stale value caching** (Pitfalls 2, 6, 12) -- depends on cancel working correctly
5. **Retry logic** (Pitfall 3) -- enhance broker, not hub
6. **Pack drill-down streaming** (Pitfalls 4, 9) -- requires abort + stale scoping
7. **Tooltips** (Pitfall 8) -- independent, low risk

## Sources

- Direct analysis of codebase: `internal/hub/hub.go` (1055 lines), `internal/hub/hub_streaming.go` (395 lines), `internal/broker/broker.go` (563 lines), `web/static/app.js` (1752 lines)
- Existing known issues: `.planning/todos/pending/read-delay-burst-on-section-switch.md`, `.planning/todos/pending/stream-pack-drill-down-values-per-register.md`
- Go concurrency patterns: [Preventing Goroutine Leaks](https://oneuptime.com/blog/post/2026-01-07-go-goroutine-leaks/view), [Channel Deadlock Patterns](https://medium.com/@gerahitesh13/the-one-channel-mistake-that-instantly-deadlocks-your-go-concurrency-and-how-to-fix-it-8b17da6dbdfd)
- Go WebSocket hub pattern: [Gorilla WebSocket Server Guide](https://oneuptime.com/blog/post/2026-02-01-go-websocket-gorilla/view)
- Go context cancellation: [Go Context in Depth](https://backendbytes.com/articles/go-context-resilient-microservices/)
