# Codebase Concerns

**Analysis Date:** 2026-04-10

## Tech Debt

**Monolithic Entry Point:**
- Issue: All functionality (general register scan, battery pack reading, Modbus transport, protocol handling) in single `main.go` file
- Files: `main.go` (707 lines)
- Impact: Difficult to test individual components, poor code organization, high maintenance burden
- Fix approach: Refactor into separate packages: `modbus/` (transport, protocol), `battery/` (pack reading), `inverter/` (register scanning), `cmd/` (CLI entry point)

**Hardcoded Configuration:**
- Issue: Network timeouts (5s, 10s), retry counts (3), sleep durations (500ms) hardcoded as constants
- Files: `main.go` lines 74, 97, 109, 158, 173, 192, 212, 496, 502, 504, 549, 554, 579, 585, 641, 646
- Impact: Cannot easily tune for different network conditions or batch operations
- Fix approach: Create configuration struct, allow CLI flags or config file for timeouts/retries

**String Manipulation for CRC:**
- Issue: Manual byte array construction for Modbus requests scattered throughout code (lines 485-494, 540-544, 570-577, 632-636)
- Files: `main.go` lines 481-629
- Impact: Error-prone, hard to maintain, easy to introduce off-by-one errors
- Fix approach: Create dedicated `ModbusFrame` builder class with helper methods

## Known Bugs

**Stale Response Handling Complexity:**
- Issue: In `readHoldingRegistersTCP()` and `writeMultipleRegistersTCP()`, code loops waiting for matching transaction ID, but no maximum packet count limit
- Files: `main.go` lines 586-606, 503-523
- Impact: Pathological case where many stale responses could cause memory exhaustion or long hangs before timeout
- Workaround: Device rarely sends stale responses in practice
- Fix: Add packet count limit (e.g., max 100 packets before giving up)

**CRC Validation Missing for Write Responses:**
- Issue: RTU write response in `writeSingleRegisterRTU()` reads 8 bytes but doesn't validate CRC of response
- Files: `main.go` lines 539-565
- Impact: Corrupted write responses not detected; silent failures possible
- Fix approach: Extract CRC validation to helper function and apply to both read and write responses

**Potential Buffer Overflow in RTU Response:**
- Issue: In `readHoldingRegistersRTU()`, byteCount read from register header (line 663) is not validated before allocating payload buffer
- Files: `main.go` lines 663-678
- Impact: Malformed response with huge byteCount could allocate massive buffer and crash
- Fix approach: Validate byteCount <= 240 (max register data per Modbus spec)

**Error Path in readFull() Doesn't Guarantee Full Read:**
- Issue: `readFull()` returns accumulated bytes + error; caller may process incomplete data if partial read + EOF
- Files: `main.go` lines 681-691
- Impact: If connection drops mid-read, data structures sized for full read may panic on iteration
- Example: Line 251 and 271 iterate `for i := 0; i < count && i*2+1 < len(data)` - index check mitigates but fragile
- Fix approach: Return error.Is(err, io.EOF) check; fail fast if not complete

## Security Considerations

**No Input Validation on Register Address:**
- Risk: User can specify any register address via command-line flag; no validation against hardware limits
- Files: `main.go` lines 42-48 (flag parsing), 301-381 (hardcoded register list)
- Current mitigation: Inverter/BMS will respond with exception if address invalid
- Recommendations: Whitelist allowed registers; reject pack numbers and group numbers with clearer validation

**No Authentication to Modbus Device:**
- Risk: TCP Modbus connection has no encryption or authentication; anyone on network can read/write device registers
- Files: `main.go` lines 66-72 (unencrypted TCP connect)
- Impact: Attacker on network can read battery SOC, write incorrect values
- Current mitigation: None built-in (security delegated to network isolation)
- Recommendations: Add option for Modbus/TCP with TLS wrapper; document network security requirements

**Hardcoded IP Address Default:**
- Risk: Default IP `10.5.99.29` is specific to user's network; in production could leak subnet topology
- Files: `main.go` line 42
- Current mitigation: Can be overridden with `-host` flag
- Recommendations: No default; require explicit IP or fail with helpful message

**CRC Calculated Incorrectly in One Path:**
- Issue: In RTU mode, CRC is little-endian (line 672: `uint16(fullResp[...]) | uint16(...) << 8`), but earlier uses may be inconsistent
- Files: `main.go` lines 546-547, 638-639 (append raw bytes), vs line 672 (reconstruct)
- Impact: Data integrity check may reject valid or accept invalid frames
- Fix approach: Centralize CRC validation, add unit tests

## Performance Bottlenecks

**Serial Registration Reads:**
- Problem: In `scanRegisters()`, each probe read waits 500ms, then makes another read (38 probes × 500ms = 19 seconds minimum)
- Files: `main.go` lines 391-424
- Cause: `time.Sleep(500ms)` after every single read (lines 212, 423)
- Improvement path: Batch reads into single Modbus request (read contiguous register ranges), eliminate artificial delays

**Connection Thrashing on Retry:**
- Problem: Every retry closes connection, reconnects, sleeps 500ms (line 97-98, 109-110)
- Files: `main.go` lines 89-113 (readWithRetry)
- Cause: Overly aggressive reconnect strategy
- Improvement path: Implement exponential backoff; reuse connection for multiple failed reads before closing

**Inefficient Cell Voltage Reading:**
- Problem: Reading 24 cell voltages one at a time in loop (lines 251-256), each cell voltage as separate register read
- Files: `main.go` lines 215-239 (readOne calls)
- Cause: Using `readOne()` wrapper instead of bulk reading
- Improvement path: Read 24 registers in single call (0x9051-0x9060), unpack locally

**Transaction ID Wrapping Risk:**
- Problem: Atomic transaction ID increments without reset; after 2^32 requests, wraps to 0
- Files: `main.go` line 13, 482, 568
- Impact: In long-running process, transaction ID collision possible with stale responses
- Improvement path: Add reset on wrap or use larger atomic counter

## Fragile Areas

**Battery Pack Selection via Side Effect:**
- Files: `main.go` lines 175-192
- Why fragile: Writing to 0x9020 via function 0x10 to select pack, then subsequent reads implicitly use that selection. If write fails partway or response corrupted, reads may read wrong pack without indication.
- Safe modification: Add sanity check: after writing 0x9020, immediately read it back to verify selection took effect before proceeding
- Test coverage: No unit tests for pack selection state management

**Retry Loop with Connection State:**
- Files: `main.go` lines 89-113
- Why fragile: Connection object passed by value in return tuple; if Close() fails silently but sets conn=nil, retry loop may continue with stale conn reference in other threads
- Safe modification: Always set conn=nil explicitly after Close(); use goroutine-safe mutex if concurrency added later
- Test coverage: No tests for concurrent read/retry scenarios

**Data Length Assumptions:**
- Files: `main.go` lines 251-256 (cell voltages), 271-274 (temps), 287-292 (temps 5-8)
- Why fragile: Code assumes data is at least `count*2` bytes (register count × 2 bytes per register), checks `i*2+1 < len(data)` but doesn't validate minimum length
- Safe modification: Pre-check `if len(data) < count*2 { return error }`
- Test coverage: No tests for malformed responses (too short, truncated)

## Scaling Limits

**Single Connection Per Command:**
- Current capacity: ~38 registers can be read in ~20 seconds (serial, 500ms sleeps)
- Limit: Cannot scale to monitoring hundreds of registers or continuous polling
- Scaling path: Implement connection pooling, batch reads into multi-register requests, remove artificial delays

**Register Probe Count:**
- Current capacity: 38 hardcoded probes in default scan (line 297-381)
- Limit: Adding more registers multiplies total scan time linearly
- Scaling path: Create modular register profiles (e.g., "basic" vs "full"), allow user to select

**Modbus RTU Bit Depth:**
- Current capacity: 16-bit transaction IDs, 16-bit register addresses
- Limit: Max 64K registers addressable (Modbus RTU hard limit)
- Scaling path: Not applicable; inherent limitation of Modbus protocol

## Dependencies at Risk

**No External Dependencies (Go Standard Library Only):**
- Risk: Minimal (single module: go.mod line 1)
- Impact: No supply chain risk
- Migration plan: Not applicable

**Go 1.26.1 Requirement:**
- Risk: Very new Go version (2026); may have undiscovered bugs or breaking changes in upcoming releases
- Impact: Build failures on different Go versions
- Migration plan: Pin to 1.25 (stable) or higher for compatibility; add CI/CD matrix testing

## Missing Critical Features

**No Persistent State/Logging:**
- Problem: All output to stdout; no way to record data over time
- Blocks: Cannot implement monitoring, alerting, or historical analysis
- Recommendation: Add optional JSON/CSV output mode; support writing to file or syslog

**No Concurrent Request Support:**
- Problem: Single-threaded serial execution
- Blocks: Cannot read multiple devices simultaneously or subscribe to register changes
- Recommendation: Implement worker pool pattern for concurrent device reads

**No Configuration File Support:**
- Problem: Must pass all parameters as command-line flags
- Blocks: Complex multi-device monitoring requires long command strings
- Recommendation: Add YAML/TOML config file support

**No Checksum/Validation for Multi-Register Data:**
- Problem: Cell voltages, temperatures stored in device but no way to verify consistency
- Blocks: Cannot detect partial/corrupted reads (e.g., 24 cell voltages where some are stale from previous pack)
- Recommendation: Read timestamp register alongside data; validate freshness

## Test Coverage Gaps

**No Unit Tests:**
- What's not tested: All public functions (readHoldingRegisters, writeRegister, CRC calculation, formatResult)
- Files: Entire `main.go`
- Risk: Regression in Modbus frame parsing, CRC calculation, retry logic
- Priority: High

**No Integration Tests:**
- What's not tested: Actual Modbus communication flow against real/simulated device
- Files: `main.go` (connect, readWithRetry, readBatteryPack, scanRegisters)
- Risk: Silent protocol violations, timing race conditions with device
- Priority: High

**No Malformed Response Tests:**
- What's not tested: Handling of truncated data, invalid CRC, exception responses, timeout scenarios
- Files: `main.go` (readHoldingRegistersTCP, readHoldingRegistersRTU, writeMultipleRegistersTCP)
- Risk: Panic on malformed data; silent acceptance of corrupted registers
- Priority: Medium

**No Bounds Testing:**
- What's not tested: Edge cases like packNum=15 (max), groupNum=3 (max), invalid addresses
- Files: `main.go` (readBatteryPack, scanRegisters)
- Risk: Index out of range, integer overflow in bit operations
- Priority: Medium

**No Retry/Failover Tests:**
- What's not tested: Behavior when device unreachable, partial responses, connection drops mid-read
- Files: `main.go` (readWithRetry, readFull)
- Risk: Unexplained failures; incorrect retry behavior with mixed failures
- Priority: High

---

*Concerns audit: 2026-04-10*
