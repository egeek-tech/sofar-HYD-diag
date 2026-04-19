<!-- GSD:project-start source:PROJECT.md -->
## Project

**Sofar HYD Diagnostic Tool**

A desktop-focused web application for monitoring and diagnosing Sofar HYD hybrid inverters via TCP Modbus. Built as a single Go binary with embedded HTML frontend, it reads real-time parameters from the inverter and presents them in a structured, easy-to-navigate interface. Based on an existing, proven CLI tool that already communicates correctly with the inverter.

**Core Value:** Provide clear, real-time visibility into all Sofar HYD inverter parameters — especially battery pack diagnostics — through a reliable web interface that reuses the proven Modbus communication layer.

### Constraints

- **Tech stack**: Go backend (reuse existing Modbus code), vanilla HTML/JS/CSS frontend (embedded via Go embed)
- **Protocol**: Sofar Modbus-G3 protocol V1.38 — register addresses and data types are fixed
- **Hardware timing**: 500ms minimum delay between Modbus reads; BMS pack switch needs ~1s settle time
- **Single connection**: Only one TCP connection to inverter at a time (Modbus is serial)
- **Deployment**: Single binary, no external dependencies
<!-- GSD:project-end -->

<!-- GSD:stack-start source:codebase/STACK.md -->
## Technology Stack

## Languages
- Go 1.26.1 - Single-file application for Modbus protocol communication
## Runtime
- Go runtime (compiled binary available)
- Go modules (go.mod)
- No external dependencies (stdlib only)
- No lockfile (go.sum not present - pure stdlib implementation)
## Frameworks
- Standard library `net` - TCP/IP socket communication and Modbus TCP transport
- Standard library `encoding/binary` - Binary data serialization (big-endian register parsing)
- Standard library `flag` - CLI argument parsing for host, port, slave ID, mode selection
- Standard library `fmt` - Output formatting and logging to stdout
- Standard library `time` - Connection timeouts, retry delays, scheduling
- Standard library `sync/atomic` - Thread-safe transaction ID counter for Modbus TCP
## Key Dependencies
- None - Pure Go standard library implementation
- Standard library `net` provides TCP/IP connectivity for Modbus TCP protocol
- Standard library `encoding/binary` handles big-endian multi-register data parsing
- CRC-16 implementation: custom function `crc16()` in `main.go` for RTU mode checksum validation
## Configuration
- Configured via CLI flags (no environment variables used)
- Connection parameters: `-host`, `-port`, `-slave`, `-mode`
- Query parameters: `-pack`, `-group`
- Standard Go build (no build configuration files present)
- Binary compiled: `modbus_reader` (executable at `/data/git/private/modbus_reader/modbus_reader`)
## Platform Requirements
- Go 1.26.1 or compatible (language features: generics via `range` over integers in 1.22+)
- Linux/amd64 (binary present)
- Network connectivity to Modbus TCP device on port 4192 (default)
- No external dependencies or runtime libraries required
## Protocol Support
- Modbus TCP (MBAP header + PDU, RFC-compliant with transaction ID matching)
- Modbus RTU (CRC-16 checksums, binary serial mode)
- Dual-mode support: selectable via `-mode tcp` or `-mode rtu`
- Function 0x03: Read Holding Registers
- Function 0x06: Write Single Register (RTU only)
- Function 0x10: Write Multiple Registers (TCP, used for 0x9020 BMS_Inquire)
- Sofar Inverter G3 (Modbus protocol specification: SOFAR_Modbus_Protocol_English_G3_V1.29)
- Register map: Inverter (0x04xx-0x05xx), Battery (0x06xx), BMS (0x90xx-0x90xx), Battery clusters (0x94xx)
- Supports battery pack topology: 4 strings × 10 packs per string (configurable via 0x900D)
<!-- GSD:stack-end -->

<!-- GSD:conventions-start source:CONVENTIONS.md -->
## Conventions

## Naming Patterns
- Single lowercase main entry point: `main.go`
- Standard Go package naming conventions
- PascalCase for exported functions: `readWithRetry()`, `readBatteryPack()`, `scanRegisters()`, `formatResult()`, `connect()`
- Lowercase for internal functions: `crc16()`, `readFull()`
- Descriptive verb-first naming: `read*`, `write*`, `format*`
- camelCase for local and package-level variables: `transactionID`, `conn`, `err`, `addr`, `regAddr`, `slaveID`
- Short abbreviations used in tight loops: `i`, `n`, `p` (for probe type)
- Descriptive names for configuration: `host`, `port`, `slaveID`, `packNum`, `groupNum`
- PascalCase for struct types: `probe` (lowercase because internal)
- Field names lowercase: `name`, `addr`, `count`, `isASCII`, `signed`, `unit`, `scale`
- UPPER_SNAKE_CASE for package-level constants: `maxRetries` (actually written as camelCase)
- Magic numbers avoided with named constants
## Code Style
- gofmt default formatting (implicit via standard Go tooling)
- Standard 1 tab indentation
- Line length follows Go conventions (typically 80-120 chars based on context)
- Standard Go tooling conventions
- No custom linters configured in this project
## Import Organization
- Not used in this codebase (single package)
## Error Handling
- Explicit error checking on every operation: `if err != nil { return nil, err }`
- Error wrapping with context using `fmt.Errorf()`: `return fmt.Errorf("write: %w", err)`
- Sentinel style for fatal errors with `log.Fatalf()`: `log.Fatalf("Failed to connect: %v", err)`
- Custom error messages provide operation context: "write:", "read MBAP:", "timeout waiting"
- Functions return error as last return value following Go conventions
- Connection management errors trigger reconnection attempts with retry logic
- Operation being performed: "write", "read", "timeout"
- Parameters when relevant: transaction ID, slave address, register address
- Underlying error wrapped for debugging
## Logging
- `log.Fatalf()` for unrecoverable initialization errors (connection failure, invalid args)
- `fmt.Printf()` for progress output and formatted results
- `fmt.Print()` for unformatted text (e.g., "Packs online: ")
- Status prefixes: `[OK]`, `[FAIL]`, `[WARN]`, `[ERROR]` with consistent spacing and alignment
- Informational output to stdout mixed with results (not separated to stderr)
## Comments
- Function behavior and return values explained: "readWithRetry reads holding registers with up to maxRetries attempts."
- Non-obvious Modbus protocol details: "Function 0x06 (Write Single Register) does NOT work for 0x9020 on this inverter - times out."
- Section markers for logical groupings: `// === Modbus Transport ===`
- Known working/failing registers documented with test dates: `// Known working registers (verified 2026-03-16):`
- Register selection logic and bitmap interpretation explained
- Bit manipulation operations explained when not immediately obvious
- Single-line comments with `//` (no block comments)
- Comment above the code it describes
- Additional context comments like `// Battery pack selection: write pack number to 0x9020...`
- Not applicable (Go project, not TypeScript/JavaScript)
## Function Design
- `formatResult()`: ~30 lines
- `readBatteryPack()`: ~160 lines (main orchestration)
- `readHoldingRegistersRTU()`: ~50 lines
- Small utility functions for Modbus protocol implementation
- Explicit parameters for connection management: `conn net.Conn` always passed through
- Slave/register addressing: `slaveID byte`, `regAddr uint16`
- Mixed parameters for control flow: `addr string`, `useRTU bool` for protocol selection
- Retry configuration via module constant (`maxRetries`)
- Error as last return value: `([]byte, error)` or `([]byte, net.Conn, int, error)`
- Multiple returns for connection state management (return new connection reference)
- Retry count returned from `readWithRetry()` for diagnostics
## Module Design
- Single entry point: `func main()`
- All helper functions exported (PascalCase): used by main program flow
- Constants and types at package level are internal or support main logic
- Not applicable (single file `main.go`, single package)
- Monolithic single-file design: all code in `main.go`
- No separate packages or internal packages
- Logical sections marked with comments: "=== Modbus Transport ===" separates protocol implementation from application logic
## Initialization & Configuration
- Standard `flag` package used for configuration
- Named parameters: `-host`, `-port`, `-slave`, `-mode`, `-pack`, `-group`
- Type conversions handled before function calls
- Input validation performed after flag parsing:
- Parse flags → validate → connect → execute (scan or read pack)
<!-- GSD:conventions-end -->

<!-- GSD:architecture-start source:ARCHITECTURE.md -->
## Architecture

## Pattern Overview
- Monolithic single-file structure (`main.go`)
- Protocol abstraction layer supporting both Modbus TCP and RTU variants
- Connection pooling with automatic retry and reconnect logic
- Register-based query model matching Sofar Modbus G3 inverter specification
- Batch and selective read modes (general scan vs. specific battery pack query)
## Layers
- Purpose: Command-line interface and user-facing functionality
- Location: `main()` function in `/data/git/private/modbus_reader/main.go` (lines 42-65)
- Contains: CLI flag parsing, entry point logic, mode selection
- Depends on: Transport layer (connection, retry logic)
- Used by: User invocation
- Purpose: Business logic for reading inverter state and battery data
- Location: `scanRegisters()` (lines 296-431) and `readBatteryPack()` (lines 116-294) in `/data/git/private/modbus_reader/main.go`
- Contains: Register probe definitions, data formatting, retry orchestration
- Depends on: Transport layer
- Used by: Application layer
- Purpose: Protocol-agnostic interface to Modbus communication
- Location: `readHoldingRegisters()` and `writeRegister()` wrapper functions (lines 464-476) in `/data/git/private/modbus_reader/main.go`
- Contains: Protocol dispatch logic
- Depends on: Protocol-specific implementations (TCP/RTU)
- Used by: Domain logic layer
- Purpose: Modbus TCP and RTU codec implementations
- Location:
- Contains: Frame construction, CRC calculation, MBAP header management
- Depends on: Utility functions (I/O, CRC)
- Used by: Transport adapter layer
- Purpose: Low-level I/O and checksum operations
- Location: `readFull()` (lines 681-691), `crc16()` (lines 693-705)
- Contains: Blocking socket reads, CRC16 MODBUS algorithm
- Depends on: Go standard library (net)
- Used by: Protocol implementation layer
## Data Flow
- **Connection State:** TCP connection object persisted across multiple register reads, closed on error and transparently reconnected via `readWithRetry()`
- **Transaction ID:** Global atomic counter incremented per TCP write to match responses (prevents stale response processing)
- **Register Read State:** Probe structures store metadata (name, address, count, data type, scaling) - stateless, reused across reads
- **Error Recovery:** Retry logic maintains reference to connection and address to enable reconnection
## Key Abstractions
- Purpose: Metadata for a single register read operation
- Examples: `{"Inverter SN", 0x0445, 10, true, false, "", 0}` in probes array (line 299)
- Pattern: Struct with fields for register address, quantity, ASCII flag, signed flag, unit, and scaling factor
- Reusability: Defines both what to read and how to format it
- Purpose: Organize registers by functional domain
- Examples:
- Pattern: Registers are grouped by address range, each range corresponds to a device subsystem
- Purpose: Support both TCP and RTU transports transparently
- Pattern: Transport layer functions dispatch to protocol-specific implementations based on `useRTU` boolean flag
- Mechanism: `readHoldingRegisters()` and `writeRegister()` accept `useRTU` parameter and route to correct implementation
- Purpose: Provide resilience against network timeouts
- Pattern: `readWithRetry()` handles connection lifecycle, retry logic, and reconnection
- Returns: Tuple of (data, connection, attempt_count, error) to support both success and error diagnostics
## Entry Points
- Location: `main()` function (lines 42-65) in `/data/git/private/modbus_reader/main.go`
- Triggers: Executable invocation with flags
- Responsibilities:
- Location: `/data/git/private/modbus_reader/modbus_reader`
- Invocation: `./modbus_reader -host <IP> -port <PORT> [-mode tcp|rtu] [-pack N] [-group G]`
## Error Handling
- Success: `[ OK ] Register_Name (0xADDR): value unit (retry N/M)` format
- Failure: `[FAIL] Register_Name (0xADDR): error (retried N/M)` format
- Warnings: `[WARN]` prefix for non-fatal issues (offline packs, requested pack unavailable)
## Cross-Cutting Concerns
- Approach: Direct `fmt.Printf()` to stdout; structured output with register name, address (hex), value, unit, and retry count
- No file logging or structured logging framework
- Approach: Input validation only for CLI flags (pack and group bounds); register addresses trusted from probe definitions
- Modbus-layer validation: slave ID acceptance, response format checking
- Approach: None - assumes network-isolated Modbus TCP/RTU gateway or trusted network
- Modbus protocol does not include authentication mechanism
- Write deadline: 3 seconds
- Read deadline: 10 seconds
- Connection dial timeout: 5 seconds
- Inter-register delay: 500ms (throttle to inverter)
- Pack-switch delay: 1 second (allow BMS time to respond)
<!-- GSD:architecture-end -->

<!-- GSD:skills-start source:skills/ -->
## Project Skills

No project skills found. Add skills to any of: `.claude/skills/`, `.agents/skills/`, `.cursor/skills/`, or `.github/skills/` with a `SKILL.md` index file.
<!-- GSD:skills-end -->

<!-- GSD:workflow-start source:GSD defaults -->
## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:
- `/gsd-quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd-debug` for investigation and bug fixing
- `/gsd-execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.
<!-- GSD:workflow-end -->



<!-- GSD:profile-start -->
## Developer Profile

> Profile not yet configured. Run `/gsd-profile-user` to generate your developer profile.
> This section is managed by `generate-claude-profile` -- do not edit manually.
<!-- GSD:profile-end -->
