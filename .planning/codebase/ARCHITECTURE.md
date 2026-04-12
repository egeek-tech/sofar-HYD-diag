# Architecture

**Analysis Date:** 2026-04-10

## Pattern Overview

**Overall:** Single-file CLI application using a protocol adapter pattern for Modbus communication

**Key Characteristics:**
- Monolithic single-file structure (`main.go`)
- Protocol abstraction layer supporting both Modbus TCP and RTU variants
- Connection pooling with automatic retry and reconnect logic
- Register-based query model matching Sofar Modbus G3 inverter specification
- Batch and selective read modes (general scan vs. specific battery pack query)

## Layers

**Application Layer:**
- Purpose: Command-line interface and user-facing functionality
- Location: `main()` function in `/data/git/private/modbus_reader/main.go` (lines 42-65)
- Contains: CLI flag parsing, entry point logic, mode selection
- Depends on: Transport layer (connection, retry logic)
- Used by: User invocation

**Domain Logic Layer:**
- Purpose: Business logic for reading inverter state and battery data
- Location: `scanRegisters()` (lines 296-431) and `readBatteryPack()` (lines 116-294) in `/data/git/private/modbus_reader/main.go`
- Contains: Register probe definitions, data formatting, retry orchestration
- Depends on: Transport layer
- Used by: Application layer

**Transport Adapter Layer:**
- Purpose: Protocol-agnostic interface to Modbus communication
- Location: `readHoldingRegisters()` and `writeRegister()` wrapper functions (lines 464-476) in `/data/git/private/modbus_reader/main.go`
- Contains: Protocol dispatch logic
- Depends on: Protocol-specific implementations (TCP/RTU)
- Used by: Domain logic layer

**Protocol Implementation Layer:**
- Purpose: Modbus TCP and RTU codec implementations
- Location: 
  - TCP reads: `readHoldingRegistersTCP()` (lines 567-629)
  - TCP writes: `writeMultipleRegistersTCP()` (lines 481-537)
  - RTU reads: `readHoldingRegistersRTU()` (lines 631-679)
  - RTU writes: `writeSingleRegisterRTU()` (lines 539-565)
- Contains: Frame construction, CRC calculation, MBAP header management
- Depends on: Utility functions (I/O, CRC)
- Used by: Transport adapter layer

**Utility Layer:**
- Purpose: Low-level I/O and checksum operations
- Location: `readFull()` (lines 681-691), `crc16()` (lines 693-705)
- Contains: Blocking socket reads, CRC16 MODBUS algorithm
- Depends on: Go standard library (net)
- Used by: Protocol implementation layer

## Data Flow

**General System Scan Flow:**

1. User invokes with default mode (no `-pack` flag)
2. `main()` calls `scanRegisters()` with inverter address, slave ID
3. `scanRegisters()` connects to inverter via `connect()` (line 67)
4. For each probe in probes array (lines 297-381):
   - `readWithRetry()` invokes read operation up to 3 attempts
   - On failure, closes connection, waits 500ms, reconnects
   - On success, formats and prints result
   - Waits 500ms between reads to avoid overwhelming inverter
5. Connection closes at end of scan

**Battery Pack Query Flow:**

1. User invokes with `-pack N -group G` flags
2. `main()` calls `readBatteryPack()` with pack number and group number
3. Reads online bitmap (0x9022) to determine which packs are available
4. Reads pack parameters (0x900D) to verify pack count
5. Writes query word to 0x9020 (BMS_Inquire) using function 0x10
6. Waits 1 second for BMS to switch pack
7. Reads pack RT data in sections:
   - Pack info (ID, SN, voltage, SOC, current, capacity, cycles)
   - Battery health metrics (SOH, health level)
   - Cell voltage range (max/min)
   - Fault data
   - 24 cell voltages (0x9051-0x9060)
   - 6 temperatures + MOS + environment (0x906B-0x9070)
   - 4 additional temperatures (0x90BC)
8. Connection closes

**State Management:**

- **Connection State:** TCP connection object persisted across multiple register reads, closed on error and transparently reconnected via `readWithRetry()`
- **Transaction ID:** Global atomic counter incremented per TCP write to match responses (prevents stale response processing)
- **Register Read State:** Probe structures store metadata (name, address, count, data type, scaling) - stateless, reused across reads
- **Error Recovery:** Retry logic maintains reference to connection and address to enable reconnection

## Key Abstractions

**Probe:**
- Purpose: Metadata for a single register read operation
- Examples: `{"Inverter SN", 0x0445, 10, true, false, "", 0}` in probes array (line 299)
- Pattern: Struct with fields for register address, quantity, ASCII flag, signed flag, unit, and scaling factor
- Reusability: Defines both what to read and how to format it

**Register Address Namespace:**
- Purpose: Organize registers by functional domain
- Examples:
  - System info: 0x0404-0x0431
  - Grid output: 0x0484-0x04BC
  - Battery pack: 0x0604-0x066B
  - BMS data: 0x9006-0x9022
  - Battery pack RT (real-time): 0x9044-0x90BC
- Pattern: Registers are grouped by address range, each range corresponds to a device subsystem

**Protocol Abstraction:**
- Purpose: Support both TCP and RTU transports transparently
- Pattern: Transport layer functions dispatch to protocol-specific implementations based on `useRTU` boolean flag
- Mechanism: `readHoldingRegisters()` and `writeRegister()` accept `useRTU` parameter and route to correct implementation

**Retry/Reconnect Wrapper:**
- Purpose: Provide resilience against network timeouts
- Pattern: `readWithRetry()` handles connection lifecycle, retry logic, and reconnection
- Returns: Tuple of (data, connection, attempt_count, error) to support both success and error diagnostics

## Entry Points

**Main CLI:**
- Location: `main()` function (lines 42-65) in `/data/git/private/modbus_reader/main.go`
- Triggers: Executable invocation with flags
- Responsibilities: 
  - Parse command-line flags (host, port, slave ID, protocol mode, pack number, group number)
  - Validate pack and group numbers (bounds checking)
  - Dispatch to either `scanRegisters()` or `readBatteryPack()` based on pack flag presence

**Compiled Binary:**
- Location: `/data/git/private/modbus_reader/modbus_reader`
- Invocation: `./modbus_reader -host <IP> -port <PORT> [-mode tcp|rtu] [-pack N] [-group G]`

## Error Handling

**Strategy:** Fail-fast with retry for transient network errors; graceful degradation for protocol-level failures

**Patterns:**

1. **Connection Errors:** `readWithRetry()` catches all errors, closes connection, waits 500ms, reconnects up to 3 times
2. **Timeout Handling:** All socket operations use deadlines (3s write, 10s read); timeout is treated as connection error triggering retry
3. **Modbus Exception Responses:** Protocol layer checks for exception bit (0x80) in function code; returns formatted error with function code and exception code
4. **CRC Mismatch (RTU):** Detected after full frame receipt; reported as error without retry
5. **Invalid MBAP Length (TCP):** Checked against range [1, 260]; invalid length reported as error
6. **Stale Response Handling:** TCP implementation loops until matching transaction ID received; prevents processing old responses

**User Feedback:**

- Success: `[ OK ] Register_Name (0xADDR): value unit (retry N/M)` format
- Failure: `[FAIL] Register_Name (0xADDR): error (retried N/M)` format
- Warnings: `[WARN]` prefix for non-fatal issues (offline packs, requested pack unavailable)

## Cross-Cutting Concerns

**Logging:** 
- Approach: Direct `fmt.Printf()` to stdout; structured output with register name, address (hex), value, unit, and retry count
- No file logging or structured logging framework

**Validation:** 
- Approach: Input validation only for CLI flags (pack and group bounds); register addresses trusted from probe definitions
- Modbus-layer validation: slave ID acceptance, response format checking

**Authentication:** 
- Approach: None - assumes network-isolated Modbus TCP/RTU gateway or trusted network
- Modbus protocol does not include authentication mechanism

**Timeouts:**
- Write deadline: 3 seconds
- Read deadline: 10 seconds
- Connection dial timeout: 5 seconds
- Inter-register delay: 500ms (throttle to inverter)
- Pack-switch delay: 1 second (allow BMS time to respond)

---

*Architecture analysis: 2026-04-10*
