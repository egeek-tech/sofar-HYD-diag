# Coding Conventions

**Analysis Date:** 2026-04-10

## Naming Patterns

**Files:**
- Single lowercase main entry point: `main.go`
- Standard Go package naming conventions

**Functions:**
- PascalCase for exported functions: `readWithRetry()`, `readBatteryPack()`, `scanRegisters()`, `formatResult()`, `connect()`
- Lowercase for internal functions: `crc16()`, `readFull()`
- Descriptive verb-first naming: `read*`, `write*`, `format*`

**Variables:**
- camelCase for local and package-level variables: `transactionID`, `conn`, `err`, `addr`, `regAddr`, `slaveID`
- Short abbreviations used in tight loops: `i`, `n`, `p` (for probe type)
- Descriptive names for configuration: `host`, `port`, `slaveID`, `packNum`, `groupNum`

**Types:**
- PascalCase for struct types: `probe` (lowercase because internal)
- Field names lowercase: `name`, `addr`, `count`, `isASCII`, `signed`, `unit`, `scale`

**Constants:**
- UPPER_SNAKE_CASE for package-level constants: `maxRetries` (actually written as camelCase)
- Magic numbers avoided with named constants

## Code Style

**Formatting:**
- gofmt default formatting (implicit via standard Go tooling)
- Standard 1 tab indentation
- Line length follows Go conventions (typically 80-120 chars based on context)

**Linting:**
- Standard Go tooling conventions
- No custom linters configured in this project

## Import Organization

**Order:**
1. Standard library: `encoding/binary`, `flag`, `fmt`, `log`, `net`, `strings`, `sync/atomic`, `time`
2. Blank line separating standard library from external packages
3. No external dependencies in current codebase

**Path Aliases:**
- Not used in this codebase (single package)

## Error Handling

**Patterns:**
- Explicit error checking on every operation: `if err != nil { return nil, err }`
- Error wrapping with context using `fmt.Errorf()`: `return fmt.Errorf("write: %w", err)`
- Sentinel style for fatal errors with `log.Fatalf()`: `log.Fatalf("Failed to connect: %v", err)`
- Custom error messages provide operation context: "write:", "read MBAP:", "timeout waiting"
- Functions return error as last return value following Go conventions
- Connection management errors trigger reconnection attempts with retry logic

**Error context includes:**
- Operation being performed: "write", "read", "timeout"
- Parameters when relevant: transaction ID, slave address, register address
- Underlying error wrapped for debugging

Example from `readHoldingRegistersRTU()`:
```go
if err != nil {
    return nil, fmt.Errorf("read header (got %d): %w", n, err)
}
```

## Logging

**Framework:** Standard Go `log` package

**Patterns:**
- `log.Fatalf()` for unrecoverable initialization errors (connection failure, invalid args)
- `fmt.Printf()` for progress output and formatted results
- `fmt.Print()` for unformatted text (e.g., "Packs online: ")
- Status prefixes: `[OK]`, `[FAIL]`, `[WARN]`, `[ERROR]` with consistent spacing and alignment
- Informational output to stdout mixed with results (not separated to stderr)

Example output patterns:
```
  [ OK ] %-28s (0x%04X): %.2f %s
  [FAIL] %-28s (0x%04X): %v (retried %d/%d)
  [WARN] Pack %d appears offline in bitmap!
```

## Comments

**When to Comment:**
- Function behavior and return values explained: "readWithRetry reads holding registers with up to maxRetries attempts."
- Non-obvious Modbus protocol details: "Function 0x06 (Write Single Register) does NOT work for 0x9020 on this inverter - times out."
- Section markers for logical groupings: `// === Modbus Transport ===`
- Known working/failing registers documented with test dates: `// Known working registers (verified 2026-03-16):`
- Register selection logic and bitmap interpretation explained
- Bit manipulation operations explained when not immediately obvious

**Comment style:**
- Single-line comments with `//` (no block comments)
- Comment above the code it describes
- Additional context comments like `// Battery pack selection: write pack number to 0x9020...`

Example from code:
```go
// readWithRetry reads holding registers with up to maxRetries attempts.
// On timeout, it closes the connection, reconnects, and retries.
// Returns data, connection (possibly new), retry count, error.
func readWithRetry(conn net.Conn, addr string, slaveID byte, regAddr uint16, count uint16, useRTU bool) ([]byte, net.Conn, int, error) {
```

**JSDoc/TSDoc:**
- Not applicable (Go project, not TypeScript/JavaScript)

## Function Design

**Size:** Functions range from single-line helpers to ~80-line orchestration functions
- `formatResult()`: ~30 lines
- `readBatteryPack()`: ~160 lines (main orchestration)
- `readHoldingRegistersRTU()`: ~50 lines
- Small utility functions for Modbus protocol implementation

**Parameters:**
- Explicit parameters for connection management: `conn net.Conn` always passed through
- Slave/register addressing: `slaveID byte`, `regAddr uint16`
- Mixed parameters for control flow: `addr string`, `useRTU bool` for protocol selection
- Retry configuration via module constant (`maxRetries`)

**Return Values:**
- Error as last return value: `([]byte, error)` or `([]byte, net.Conn, int, error)`
- Multiple returns for connection state management (return new connection reference)
- Retry count returned from `readWithRetry()` for diagnostics

Example:
```go
func readWithRetry(conn net.Conn, addr string, slaveID byte, regAddr uint16, count uint16, useRTU bool) ([]byte, net.Conn, int, error)
```

## Module Design

**Exports:**
- Single entry point: `func main()`
- All helper functions exported (PascalCase): used by main program flow
- Constants and types at package level are internal or support main logic

**Barrel Files:**
- Not applicable (single file `main.go`, single package)

**Package structure:**
- Monolithic single-file design: all code in `main.go`
- No separate packages or internal packages
- Logical sections marked with comments: "=== Modbus Transport ===" separates protocol implementation from application logic

## Initialization & Configuration

**Command-line flags:**
- Standard `flag` package used for configuration
- Named parameters: `-host`, `-port`, `-slave`, `-mode`, `-pack`, `-group`
- Type conversions handled before function calls
- Input validation performed after flag parsing:
  ```go
  if *group < 0 || *group > 15 {
      log.Fatalf("Invalid group %d: must be 0-15", *group)
  }
  ```

**Flow:**
- Parse flags → validate → connect → execute (scan or read pack)

---

*Convention analysis: 2026-04-10*
