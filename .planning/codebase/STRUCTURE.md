# Codebase Structure

**Analysis Date:** 2026-04-10

## Directory Layout

```
modbus_reader/
├── main.go                                    # Single-file CLI application
├── go.mod                                     # Go module definition
├── modbus_reader                              # Compiled binary (executable)
├── .git/                                      # Git repository metadata
├── .claude/                                   # Claude project metadata
├── .planning/                                 # GSD planning documents (generated)
│   └── codebase/                             # Codebase analysis output
├── image.png                                  # Technical diagram/reference (24402 bytes)
├── Sofar_Inverter_MODBUS_V1.38_EN.pdf        # Sofar inverter Modbus protocol reference
└── SOFAR_Modbus_Protocol_English_G3_V1.29(only 3PH).xlsx  # G3 three-phase protocol spec
```

## Directory Purposes

**Project Root (`/data/git/private/modbus_reader/`):**
- Purpose: Single-file Go project for reading Sofar inverter Modbus registers
- Contains: Application source, build output, technical documentation
- Key files: `main.go`, `modbus_reader` (binary)

**.git/:**
- Purpose: Git version control metadata
- Committed: Yes

**.claude/:**
- Purpose: Claude AI project configuration and memory
- Committed: Yes
- Contents: Project context and memory files

**.planning/codebase/:**
- Purpose: GSD-generated codebase analysis documents
- Generated: Yes
- Committed: No (build artifact)

## Key File Locations

**Entry Points:**
- `main.go`: Single application entry point (line 42)

**Configuration:**
- `go.mod`: Go module definition at `/data/git/private/modbus_reader/go.mod`
  - Module: `modbus_reader`
  - Go version: 1.26.1 (Note: modern version, likely 1.26 when created)

**Core Logic:**
- All application logic in `/data/git/private/modbus_reader/main.go`:
  - CLI parsing: `main()` (lines 42-65)
  - General system scan: `scanRegisters()` (lines 296-431)
  - Battery pack query: `readBatteryPack()` (lines 116-294)
  - Result formatting: `formatResult()` (lines 433-460)

**Protocol Transport:**
- Modbus TCP reads: `readHoldingRegistersTCP()` (lines 567-629)
- Modbus TCP writes: `writeMultipleRegistersTCP()` (lines 481-537)
- Modbus RTU reads: `readHoldingRegistersRTU()` (lines 631-679)
- Modbus RTU writes: `writeSingleRegisterRTU()` (lines 539-565)
- Protocol dispatch: `readHoldingRegisters()`, `writeRegister()` (lines 464-476)

**Utilities:**
- Blocking socket read: `readFull()` (lines 681-691)
- CRC16 checksum: `crc16()` (lines 693-705)
- Connection establishment: `connect()` (lines 67-75)
- Retry wrapper: `readWithRetry()` (lines 90-115)

**Testing:**
- Not detected (no `*_test.go` files)

## Naming Conventions

**Files:**
- Single file: `main.go` (Go convention)
- Binary output: `modbus_reader` (snake_case executable)

**Functions:**
- camelCase, action-verb first: `readHoldingRegisters()`, `formatResult()`, `scanRegisters()`
- Protocol-specific suffix: `readHoldingRegistersTCP()`, `readHoldingRegistersRTU()`, `writeSingleRegisterRTU()`, `writeMultipleRegistersTCP()`
- Utility functions: `readFull()`, `crc16()`, `connect()`, `readWithRetry()`

**Variables:**
- camelCase: `transactionID`, `conn`, `slaveID`, `useRTU`, `regAddr`, `maxRetries`
- Loop counters: `i`, `n` (short names acceptable in tight loops)
- Error: `err` (Go convention)

**Types:**
- Struct: `probe` (camelCase, lowercase single name) - defined at line 82
- Constants: `maxRetries` (camelCase value)

**Constants:**
- Timeout durations: `5*time.Second`, `3*time.Second`, `10*time.Second`
- Register addresses: Hexadecimal literals (e.g., `0x0445`, `0x9022`)
- Modbus function codes: Hex literals (e.g., `0x03` read holding, `0x10` write multiple)
- Default values: `host := flag.String("host", "10.5.99.29", ...)`

**Registers:**
- Address ranges organized by subsystem: 0x0404-0x0431 (system), 0x0484-0x04BC (grid), 0x0604-0x066B (battery), 0x9006-0x9022 (BMS info), 0x9044-0x90BC (pack RT data)

## Where to Add New Code

**New Feature (e.g., read a new register subsystem):**
- Primary code: Add function to `/data/git/private/modbus_reader/main.go` following pattern of `scanRegisters()` or `readBatteryPack()`
- Register definitions: Add probe structs to existing probes array in `scanRegisters()` or define new array for new feature
- Dispatch: Add flag parsing in `main()` to trigger new function

**New Protocol Support (e.g., Modbus ASCII):**
- Implementation: Add `readHoldingRegistersASCII()` function in `/data/git/private/modbus_reader/main.go`
- Dispatch: Update `readHoldingRegisters()` and `writeRegister()` wrapper functions to check new protocol type
- CLI: Add mode to `-mode` flag options in `main()` (currently accepts "tcp" or "rtu")

**New Utilities:**
- Location: Add function directly to `/data/git/private/modbus_reader/main.go` (no separate utility files)
- Pattern: Follows existing utility naming (action verb first, lowercase)

**Refactoring for Multi-file:**
- Future refactoring could split into:
  - `transport.go` - Protocol implementations
  - `domain.go` - Register scanning logic
  - `cli.go` - CLI and main entry point
  - `modbus.go` - Modbus codec utilities
  - `main.go` - App bootstrap only

## Special Directories

**`.planning/codebase/`:**
- Purpose: GSD codebase analysis documents (ARCHITECTURE.md, STRUCTURE.md, etc.)
- Generated: Yes
- Committed: No

## Global State and Synchronization

**Transaction ID:**
- Variable: `transactionID` (line 11)
- Type: `atomic.Uint32`
- Purpose: Ensures unique transaction IDs for TCP Modbus requests to match responses
- Lifecycle: Initialized to 0, incremented atomically in `writeMultipleRegistersTCP()` (line 568) and `readHoldingRegistersTCP()` (line 568)
- Thread-safety: Atomic operations prevent concurrent corruption

**Connection Lifecycle:**
- Passed by reference through call chain: `main()` → `scanRegisters()/readBatteryPack()` → `readWithRetry()` → protocol functions
- Explicit close: `defer conn.Close()` in `readBatteryPack()` (line 120), explicit close in `scanRegisters()` (line 429)
- Reconnection: `readWithRetry()` handles null connection by reconnecting via `connect()`

## Code Organization Principle

**Single Responsibility, Organized by Technical Layer:**
- Each function handles one specific responsibility (read TCP, read RTU, format output, calculate CRC)
- Functions organized bottom-up: utilities → protocol → transport → domain → CLI
- No horizontal or feature-based layering (no separate battery vs. grid packages)

---

*Structure analysis: 2026-04-10*
