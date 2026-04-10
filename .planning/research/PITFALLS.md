# Domain Pitfalls

**Domain:** Modbus TCP diagnostic web tool (Sofar HYD inverter)
**Researched:** 2026-04-10
**Confidence:** HIGH (based on direct code analysis of proven CLI tool and established Modbus protocol constraints)

## Critical Pitfalls

Mistakes that cause rewrites, data corruption, or inverter communication failures.

### Pitfall 1: Concurrent Modbus Access Without Request Serialization

**What goes wrong:** Multiple WebSocket clients (or auto-refresh timers from multiple browser tabs) fire simultaneous Modbus read requests. Two goroutines write to the same TCP connection at the same time, interleaving bytes on the wire. The inverter receives garbled frames, responds with exceptions or goes silent. Even if writes don't interleave, responses get routed to the wrong goroutine.

**Why it happens:** The existing CLI code (`main.go`) uses a single `net.Conn` in a purely sequential loop -- there is zero concurrency. Developers refactoring this into a web server naturally reach for goroutines per HTTP/WebSocket handler, forgetting that Modbus TCP is inherently serial: one request, one response, then the next.

**Consequences:** Corrupted Modbus frames, inverter timeout cascade, TCP-to-RTU converter lockup requiring power cycle, phantom data (response from request A delivered to handler B).

**Prevention:**
- Wrap the Modbus connection in a struct with a `sync.Mutex` that serializes all read/write operations. Every `readWithRetry` and `writeRegister` call must hold the lock for the entire request-response cycle.
- Better: use a single-goroutine command channel pattern. One dedicated goroutine owns the `net.Conn`, reads commands from a channel, executes them sequentially, and sends results back via per-request response channels.
- Never expose `net.Conn` directly to HTTP/WebSocket handlers.

**Detection:** Multiple browser tabs open simultaneously cause timeouts or wrong data. Intermittent "invalid MBAP length" or "wrong slave" errors under concurrent load.

**Phase:** Must be solved in the first phase when extracting the Modbus layer from CLI into a service. This is the architectural foundation -- everything else depends on it.

---

### Pitfall 2: Battery Pack Selection Race Condition (Write-Wait-Read Atomicity)

**What goes wrong:** The BMS pack query sequence is: (1) write pack number to 0x9020, (2) wait ~1 second for BMS to switch, (3) read pack data from 0x9044+. If another request writes a different pack number to 0x9020 during the wait or read phase, the data returned belongs to the wrong pack. The UI shows Pack 3's cell voltages under Pack 7's label.

**Why it happens:** The current code (`readBatteryPack`, line 116) treats the entire write-wait-read as one atomic operation because CLI is single-threaded. In a web server with multiple clients, request B's pack selection can stomp on request A's in-progress read.

**Consequences:** Silently wrong data displayed to user. This is worse than an error -- the user sees plausible-looking cell voltages and temperatures, but for the wrong pack. Misdiagnosis of battery issues.

**Prevention:**
- The Modbus serialization mutex (Pitfall 1) must hold the lock for the ENTIRE pack query sequence: write 0x9020 + 1s wait + all subsequent reads for that pack. Not per-request, but per-sequence.
- Implement a "pack query" as a single atomic operation in the command channel. The command struct should contain the full sequence of operations.
- Consider a pack data cache: once a pack's data is read, serve it from cache for subsequent requests within a short TTL (e.g., 5-10 seconds). This reduces the frequency of the expensive write-wait-read cycle.

**Detection:** Open two browser tabs, request different packs simultaneously. If the data matches when it shouldn't, you have a race.

**Phase:** Must be addressed in the same phase as Pitfall 1 (Modbus service extraction). The command channel design must account for multi-step atomic operations from day one.

---

### Pitfall 3: Blocking the Modbus Bus During Pack Enumeration

**What goes wrong:** User navigates to battery section. The UI tries to enumerate all packs for all towers (e.g., 2 towers x 10 packs = 20 pack queries). Each pack query takes ~3-5 seconds (write + 1s settle + ~15 register reads at 500ms each). Total: 60-100 seconds where the Modbus bus is completely locked. During this time, no other section can refresh -- the entire UI appears frozen.

**Why it happens:** Eagerly loading all pack data when the battery section opens. The CLI tool only reads one pack per invocation (`-pack 0 -group 1`), so this was never a problem.

**Consequences:** UI freeze for minutes. Users think the tool is broken. Other sections (grid, PV, system info) cannot update. If a WebSocket has a heartbeat timeout, the connection drops.

**Prevention:**
- Lazy-load individual packs on demand (already in PROJECT.md requirements). Only query a specific pack when the user clicks on it.
- Show the online bitmap (0x9022) immediately -- this is a single fast read. Use it to gray out offline packs.
- Implement request priority: system info and grid data reads should be able to preempt queued pack queries. Use a priority queue for the command channel, not a simple FIFO.
- Show progress indicator during pack query: "Selecting pack 3... Reading cell voltages..."

**Detection:** Navigate to battery section with many packs online. Measure time until any other section can refresh.

**Phase:** UI/UX design phase must define the navigation flow. Backend command queue architecture must support priority/cancellation by the Modbus service phase.

---

### Pitfall 4: Connection State Mismatch Between Server and Hardware

**What goes wrong:** The TCP connection to the Modbus converter drops (network blip, converter reboot, inverter power cycle), but the server's connection state still shows "connected." Subsequent Modbus requests fail with write errors, but the server keeps retrying on a dead socket instead of reconnecting. Or: the TCP socket appears alive (TCP keepalive hasn't detected the drop yet), but the RTU converter behind it has reset its state.

**Why it happens:** The current CLI code reconnects on failure (`readWithRetry` closes and reopens the connection, lines 90-114). But in a long-running web server, the connection lifetime is hours/days, not seconds. Network partitions, TCP half-open states, and converter resets become inevitable. TCP keepalive defaults are often 2+ hours.

**Consequences:** Extended period of all-red "read failed" indicators across the UI. Server logs fill with retry errors. User has to manually disconnect/reconnect.

**Prevention:**
- Implement connection health monitoring: periodic heartbeat read (e.g., read system running state 0x0404 every 30 seconds) that doubles as connection validation.
- On any read/write error, immediately mark connection as "degraded" in the UI. After 2-3 consecutive failures, auto-reconnect.
- Set TCP keepalive to a short interval (e.g., 10 seconds) via `net.TCPConn.SetKeepAlive` and `SetKeepAlivePeriod`.
- Expose connection state to the frontend over WebSocket: connected / reconnecting / disconnected. The current CLI just exits on fatal errors -- the web server must recover.
- Track the connection's `net.Conn` with a generation counter. When a reconnection happens, all in-flight requests on the old connection should be cancelled and retried on the new one.

**Detection:** Pull the network cable for 5 seconds, reconnect. Observe how long until the UI recovers.

**Phase:** Modbus service extraction phase. The reconnection logic from `readWithRetry` must be elevated to a connection manager that runs independently of individual requests.

---

### Pitfall 5: Breaking the Proven Modbus Protocol Implementation During Refactoring

**What goes wrong:** While refactoring `main.go` into packages/modules for the web server, developers inadvertently change the Modbus frame construction, timing, or error handling. Subtle changes like reordering `SetWriteDeadline`/`SetReadDeadline` calls, changing the transaction ID atomics, or modifying the stale-response-skipping loop (lines 501-536) break communication with the hardware.

**Why it happens:** The current code has several hard-won implementation details that look like they could be "improved":
- Function 0x10 (Write Multiple) is used instead of 0x06 (Write Single) for 0x9020 because 0x06 times out on this specific inverter (line 478-479 comment).
- Transaction ID matching loop skips stale responses (lines 501-537, 585-628) -- this handles real-world situations where the converter sends delayed responses.
- The 500ms delay between reads (line 59) is a protocol requirement, not a conservative default.
- CRC16 for RTU mode uses a specific polynomial and byte order.

**Consequences:** Tool stops working with the actual hardware. Since the inverter is a physical device that may not be available during development, these regressions may not be caught until deployment.

**Prevention:**
- Extract the Modbus transport layer (`readHoldingRegisters`, `writeRegister`, `readWithRetry`, `readFull`, `crc16`) into a separate package WITH ZERO FUNCTIONAL CHANGES initially. Copy-paste, then verify it compiles and the CLI still works.
- Write integration tests against the real hardware before refactoring. Record known-good request/response pairs.
- Preserve every comment about protocol quirks (e.g., the 0x10 vs 0x06 comment). These are battle-tested findings.
- Gate refactoring with a "smoke test" flag: `go run . -host 10.5.99.29 -port 4192` must produce identical output before and after each refactoring step.

**Detection:** After any refactoring, run the original CLI scan against the real inverter. Diff the output against a known-good baseline.

**Phase:** Very first refactoring step. The Modbus transport extraction should be a separate, minimal change with hardware verification before any web server code is added.

---

## Moderate Pitfalls

### Pitfall 6: WebSocket Lifecycle Mismanagement

**What goes wrong:** Browser tab closes but the server doesn't detect it. The server keeps queuing Modbus reads for a dead WebSocket, wasting bus time. Or: server detects close but doesn't clean up the subscription, causing goroutine leaks. Or: user opens 10 tabs, each starts its own auto-refresh loop, and the Modbus bus is overwhelmed with 10x the expected request volume.

**Prevention:**
- Implement a subscription model: WebSocket clients subscribe to data sections (system, grid, battery, etc.). The server reads each section once per cycle and broadcasts to all subscribers. Multiple tabs do NOT multiply Modbus traffic.
- Use WebSocket ping/pong frames for liveness detection. Go's `gorilla/websocket` (or `nhooyr.io/websocket`) supports `SetPongHandler` and `SetPingHandler`. Set a read deadline and close dead connections after missed pongs.
- Track active WebSocket connections with a connection manager. Log connection/disconnection events.
- Implement a "data hub" pattern: one goroutine reads data at a fixed interval, stores latest values, and notifies all connected WebSocket clients. Clients that connect late immediately get the cached latest data.

**Detection:** Open and close browser tabs rapidly. Monitor goroutine count with `runtime.NumGoroutine()` or pprof. It should stabilize, not grow.

**Phase:** WebSocket implementation phase. Design the pub/sub model before writing WebSocket handlers.

---

### Pitfall 7: Frontend Auto-Refresh Overwhelming the Modbus Bus

**What goes wrong:** Auto-refresh is implemented as a tight loop or short interval (e.g., every 1 second). Each refresh reads 20+ registers, each taking 500ms+ with the mandatory inter-read delay. The refresh cycle takes 10+ seconds, but the timer fires every 1 second, creating a backlog of pending requests that never drains.

**Prevention:**
- Auto-refresh should be "read, then wait, then read again" -- NOT timer-based. The next cycle starts only after the previous one completes plus a configurable pause.
- Display the actual refresh rate in the UI so users see "Last updated: 3s ago" rather than expecting instant updates.
- Section-aware refresh: only refresh the currently visible section. If the user is viewing "Grid Connected," don't read PV or battery registers.
- The toggle button should immediately cancel any in-progress refresh cycle when turned off.

**Detection:** Enable auto-refresh, watch server logs. If you see "request queued" messages growing without corresponding completions, the refresh rate exceeds bus capacity.

**Phase:** Frontend/WebSocket design phase. The refresh model must be pull-based (server pushes when data is ready) not push-based (client demands on a timer).

---

### Pitfall 8: `go:embed` Build and Development Workflow Friction

**What goes wrong:** During development, every frontend change requires a full `go build` to re-embed the assets. Developer makes a CSS change, waits for compilation, refreshes browser, sees old version because browser cached it. Development cycle becomes painfully slow.

**Prevention:**
- Use a build tag or environment variable to serve from filesystem during development (`//go:build !embed` with `os.DirFS("./frontend")` fallback). Only use `embed.FS` for production builds.
- Set proper `Cache-Control: no-cache` headers in development mode.
- Alternatively, serve frontend via a separate HTTP server during development (e.g., just `python -m http.server` in the frontend directory) and proxy WebSocket connections. But this adds CORS complexity -- the build-tag approach is simpler.
- Include a `Makefile` or build script that does `go build -tags embed` for the final binary.

**Detection:** Change a CSS color, rebuild, refresh. If you don't see the change, you have a caching or embed-staleness issue.

**Phase:** Project scaffolding / first phase. Set up the development workflow before writing frontend code.

---

### Pitfall 9: Not Handling Modbus Exception Responses in the UI

**What goes wrong:** The inverter returns Modbus exception codes (illegal function, illegal address, slave busy, etc.) but the UI shows a generic "Error" or blank field. The user has no idea what went wrong or whether to retry.

**Why it happens:** The current CLI code does decode exceptions (line 525-529, 608-613) and prints them, but web UIs often lose this detail in the HTTP/WebSocket abstraction layer. Exception `0x06` (Slave Device Busy) is particularly important -- it means "try again shortly" not "this register doesn't exist."

**Prevention:**
- Propagate Modbus exception codes through the WebSocket protocol. Include error type: timeout, exception (with code), connection lost.
- Display meaningful messages: "Slave Busy -- retrying" vs "Register not supported" vs "Connection lost."
- For exception 0x06 (Busy), auto-retry after a delay. For 0x02 (Illegal Data Address), mark the field as "not available on this model" and stop retrying.
- Color-code: green for fresh data, amber for stale (last read succeeded but current one failed), red for persistent failure.

**Detection:** Read a register that doesn't exist on your inverter model. The UI should show a specific error, not just go blank.

**Phase:** Backend API design phase (define error response format) and frontend implementation phase (render error states).

---

### Pitfall 10: Transaction ID Exhaustion or Collision in Long-Running Server

**What goes wrong:** The current code uses `atomic.Uint32` for transaction IDs (line 14) but Modbus TCP transaction IDs are 16-bit (uint16). The atomic counter wraps at 2^32, but only the lower 16 bits are used in the frame (line 571: `txID := uint16(transactionID.Add(1))`). This is actually fine for collision avoidance in a sequential CLI run, but in a long-running server processing thousands of requests per day, combined with the stale-response-skipping loop, there's a subtle risk: if the counter wraps and a very old stale response arrives with a matching 16-bit transaction ID, it could be accepted as the current response.

**Why it happens:** The CLI runs for seconds. The web server runs for weeks. The stale response window from the TCP-to-RTU converter can be unpredictable.

**Prevention:**
- This is actually a low-risk issue given the current skip-stale-responses loop design, but be aware of it.
- Add a response validation: check that the response register address matches the request (not just the transaction ID). The current code only validates transaction ID.
- Set shorter read deadlines in the server context (the current 10-second deadline is very generous; 3-5 seconds is sufficient for single reads).
- Log stale response events for debugging. If they occur frequently, the TCP-to-RTU converter may need attention.

**Detection:** Monitor for "skipping stale response" log events. If frequent, investigate converter health.

**Phase:** Modbus service extraction. Add response validation fields when wrapping the transport.

---

## Minor Pitfalls

### Pitfall 11: Hardcoded Register Addresses Scattered Across Code

**What goes wrong:** Register addresses (0x9020, 0x9022, 0x0604, etc.) are duplicated between the Go backend and JavaScript frontend. A register address changes or gets added, and only one side is updated.

**Prevention:**
- Define all register addresses and metadata (name, address, count, type, unit, scale) in a single Go data structure (similar to the existing `probes` slice on line 297). Generate or serve this as a JSON endpoint that the frontend consumes.
- The frontend should never hardcode register addresses. It should receive a data structure from the backend that includes labels, units, and current values.

**Detection:** Search for hex literals (0x0..., 0x9...) in both Go and JS code. If the same address appears in both, you have a duplication risk.

**Phase:** Backend API design phase.

---

### Pitfall 12: Ignoring the 60-Register-Per-Read Limit

**What goes wrong:** Developer optimizes by reading large register ranges in a single Modbus request (e.g., reading 0x0404 through 0x0460 as one request for 92 registers). The inverter silently returns only the first 60 registers, or returns an exception.

**Prevention:**
- Enforce max 60 registers per read in the Modbus transport layer. The `readHoldingRegisters` function should reject or split requests exceeding this limit.
- Document this constraint prominently. The existing code already respects it (largest read is 24 registers for cell voltages), but future developers may try to optimize.

**Detection:** Any `readHoldingRegisters` call with `count > 60` will fail or return truncated data.

**Phase:** Modbus service extraction. Add validation in the transport layer.

---

### Pitfall 13: Losing the 500ms Inter-Read Timing Constraint

**What goes wrong:** The 500ms delay between reads (from protocol spec and hardcoded in current CLI via `time.Sleep(500 * time.Millisecond)`) is removed during refactoring because it looks like a conservative sleep. Without it, rapid-fire reads overwhelm the TCP-to-RTU converter or the inverter's Modbus slave, causing timeouts and eventually connection drops.

**Prevention:**
- Build the inter-read delay into the Modbus command executor, not the caller. The single-goroutine command channel should enforce minimum 500ms between any two Modbus operations automatically.
- Make the delay configurable (for different hardware) but default to 500ms.
- Log actual read timing so you can verify the constraint is being respected.

**Detection:** Remove the delay, run a rapid scan. Count timeout errors vs. baseline with delay.

**Phase:** Modbus service extraction. The command executor must own timing.

---

### Pitfall 14: Frontend State Inconsistency During Connection Changes

**What goes wrong:** User is viewing battery data when the Modbus connection drops. The UI still shows the last-known values without any indication they are stale. User reconnects, but only the currently-visible section refreshes. Other sections show a mix of old and new data.

**Prevention:**
- Timestamp every data point. Display "Last updated: Xs ago" per section.
- On connection loss, immediately mark ALL displayed data as stale (amber/gray overlay).
- On reconnection, the currently-visible section refreshes first (priority), but queue background refreshes for other sections.
- Never silently show stale data as if it were current.

**Detection:** Disconnect network, wait 30 seconds, reconnect. Check if all sections show clear stale indicators.

**Phase:** Frontend implementation phase.

---

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|-------------|---------------|------------|
| Modbus service extraction | Breaking proven protocol code (Pitfall 5) | Copy-paste first, verify against hardware, then refactor |
| Modbus service extraction | No request serialization (Pitfall 1) | Single-goroutine command channel from day one |
| Modbus service extraction | Pack query race condition (Pitfall 2) | Atomic multi-step commands in the channel |
| Modbus service extraction | Losing inter-read delay (Pitfall 13) | Delay lives in executor, not caller |
| Connection management | State mismatch (Pitfall 4) | Heartbeat reads, auto-reconnect, generation counter |
| WebSocket implementation | Lifecycle leaks (Pitfall 6) | Pub/sub with connection manager, ping/pong |
| WebSocket implementation | Bus overload from refresh (Pitfall 7) | Completion-triggered refresh, section-aware |
| Backend API design | Scattered register addresses (Pitfall 11) | Single source of truth, JSON endpoint |
| Backend API design | Poor error propagation (Pitfall 9) | Typed errors with Modbus exception codes |
| Frontend implementation | Stale data display (Pitfall 14) | Timestamps, stale indicators, prioritized refresh |
| Frontend implementation | Bus blocking during pack enumeration (Pitfall 3) | Lazy load, online bitmap first, priority queue |
| Build/scaffold | Embed workflow friction (Pitfall 8) | Build tags for dev/prod asset serving |

## Sources

- Direct analysis of `/data/git/private/modbus_reader/main.go` (707 lines, verified working CLI tool)
- Sofar Modbus-G3 Protocol V1.38 constraints (max 60 registers, 500ms delay, BMS query sequence)
- `.planning/PROJECT.md` project requirements and constraints
- Modbus TCP protocol specification (MBAP header, transaction ID matching, function codes 0x03/0x10)
- Established patterns for serial protocol web gateways (Modbus, SCADA, industrial IoT)
