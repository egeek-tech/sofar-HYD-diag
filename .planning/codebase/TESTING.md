# Testing Patterns

**Analysis Date:** 2026-04-10

## Test Framework

**Runner:**
- Go built-in testing framework (`testing` package)
- No custom test runner configured

**Assertion Library:**
- No external assertion library (not applicable - no tests exist)

**Run Commands:**
```bash
go test ./...              # Run all tests
go test -v ./...           # Run with verbose output
go test -cover ./...       # Show coverage
go test -race ./...        # Run with race detector
```

## Test File Organization

**Location:**
- No test files present in repository
- Standard Go convention: `*_test.go` files in same package

**Naming:**
- Expected pattern (not currently used): `main_test.go` for tests of `main.go`
- Test functions follow Go convention: `TestFunctionName(t *testing.T)`

**Structure:**
```
modbus_reader/
├── main.go              # All source code
└── (no test files)
```

## Test Coverage

**Status:** No test coverage - no test files exist in repository

**Required Tests (not implemented):**
- Connection management: retry logic, reconnection behavior
- Modbus protocol: TCP and RTU frame construction
- CRC calculation: `crc16()` function
- Register parsing: ASCII, signed, unsigned, scaled values
- Battery pack selection: bit manipulation for group/pack encoding
- Error handling: timeout, connection failure, malformed responses

**Test Gaps:**
- No unit tests for utility functions (`crc16()`, `readFull()`, `formatResult()`)
- No integration tests for Modbus communication
- No mocking of network I/O
- No fixture data for register values

## Mocking

**Status:** Not applicable - no tests exist

**Would require mocking:**
- `net.Conn` for TCP/RTU connection I/O
- Hardware Modbus server responses (TCP MBAP headers, PDU payloads, CRC)
- Timeout scenarios

**Recommended approach (if implemented):**
```go
// Mock net.Conn to simulate Modbus device responses
type mockConn struct {
    io.ReadWriter
    responses [][]byte
}

// Mock responses by controlling Read() behavior
```

## Manual Testing Evidence

**Tested functionality (documented in comments):**

From `main.go` lines 16-40, register verification performed on actual hardware (2026-03-16):
```
// Known working registers (verified 2026-03-16):
// [ OK ] Inverter SN                    (0x0445): "SP2ES108N6R462"
// [ OK ] System running state           (0x0404): 0 (Waiting)
// [ OK ] Bat voltage ch1                (0x0604): 528.8V
// [ OK ] Grid frequency                 (0x0484): 50.01 Hz
// [FAIL] Battery cluster 1 (0x9400+)    - timeout (BDU not online)
```

**Test execution:**
- Manual invocation against Sofar inverter at `10.5.99.29:4192`
- Specific pack reading tested: `-pack` and `-group` flags verified
- Function 0x06 vs 0x10 write behavior tested: found 0x06 times out, 0x10 works

**Known issues identified through testing:**
- BMS_Inquire (0x9020) requires function 0x10, not 0x06 (documented in line 479-480)
- Battery cluster 1 (0x9400+) times out when BDU offline

## Error Testing

**Current approach:** No automated error testing

**Error conditions handled manually:**
- Connection timeout (500ms retry, up to 3 attempts)
- Read timeout (10 second deadline)
- Write timeout (3 second deadline)
- Invalid slave ID validation
- Invalid group/pack number validation
- CRC mismatch detection in RTU mode
- MBAP length validation
- Stale response skipping (transaction ID matching)

**Error scenario examples from code:**

Timeout with retry in `readWithRetry()`:
```go
if err != nil {
    lastErr = err
    conn.Close()
    conn = nil
    if attempt < maxRetries {
        time.Sleep(500 * time.Millisecond)
    }
    continue
}
```

Validation before execution:
```go
if *group < 0 || *group > 15 {
    log.Fatalf("Invalid group %d: must be 0-15", *group)
}
```

Response validation:
```go
respLen := int(binary.BigEndian.Uint16(mbap[4:6]))
if respLen < 1 || respLen > 260 {
    return fmt.Errorf("invalid MBAP length: %d", respLen)
}
```

## Test Type Classification

**Unit Tests:**
- Not implemented
- Would test: `crc16()`, `readFull()`, `formatResult()`, bit manipulation

**Integration Tests:**
- Not automated
- Manual testing performed against real Sofar inverter hardware
- Modbus TCP and RTU communication verified end-to-end

**E2E Tests:**
- Not automated
- Device communication verified manually via command-line flags

## Async Testing

**Applicable but not automated:**
- Connection establishment is synchronous with timeout
- Read/write operations use `SetReadDeadline()` and `SetWriteDeadline()`
- Retry logic handles asynchronous network delays

**Example from code:**
```go
conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
if _, err := conn.Write(req); err != nil {
    return fmt.Errorf("write: %w", err)
}
```

## Code Examples for Testing

**Current CRC implementation (would need testing):**
```go
func crc16(data []byte) uint16 {
    crc := uint16(0xFFFF)
    for _, b := range data {
        crc ^= uint16(b)
        for range 8 {
            if crc&0x0001 != 0 {
                crc = (crc >> 1) ^ 0xA001
            } else {
                crc >>= 1
            }
        }
    }
    return crc
}
```

**Modbus frame construction (would need verification):**
```go
func readHoldingRegistersTCP(conn net.Conn, slaveID byte, startAddr uint16, quantity uint16) ([]byte, error) {
    txID := uint16(transactionID.Add(1))
    
    req := make([]byte, 12)
    binary.BigEndian.PutUint16(req[0:2], txID)
    binary.BigEndian.PutUint16(req[2:4], 0)
    binary.BigEndian.PutUint16(req[4:6], 6)
    req[6] = slaveID
    req[7] = 0x03
    binary.BigEndian.PutUint16(req[8:10], startAddr)
    binary.BigEndian.PutUint16(req[10:12], quantity)
    // ...
}
```

**Response parsing (requires validation):**
```go
if respTxID != txID {
    continue // skip stale response from previous request
}

if pdu[0]&0x80 != 0 {
    errCode := byte(0)
    if len(pdu) > 1 {
        errCode = pdu[1]
    }
    return nil, fmt.Errorf("exception: func=0x%02X err=0x%02X", pdu[0], errCode)
}
```

## Recommendations for Testing

**High priority (core functionality):**
1. Unit tests for `crc16()` with known CRC values
2. Modbus frame construction tests (TCP and RTU)
3. Response parsing with mock data (exception handling, stale response filtering)

**Medium priority:**
4. Integration tests with mock Modbus server
5. Retry logic verification (timeout, reconnection)
6. Battery pack selection encoding (bit manipulation for group/pack)

**Low priority:**
7. CLI flag validation tests
8. Output formatting tests

**Suggested test framework enhancement:**
- Add `*_test.go` files alongside functions under test
- Use table-driven tests for Modbus frame variations
- Mock `net.Conn` for deterministic testing
- Define fixtures for known register values

---

*Testing analysis: 2026-04-10*
