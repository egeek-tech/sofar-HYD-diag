<!-- generated-by: gsd-doc-writer -->
# Testing

This document describes the testing framework, test structure, and how to run and write tests for the Sofar HYD Diagnostic Tool.

## Test framework and setup

The project uses Go's built-in `testing` package as the primary test framework (Go 1.26). In addition, several packages use the [testify](https://github.com/stretchr/testify) assertion library (v1.11.1) for more expressive assertions via `assert` and `require`.

No global test setup is required beyond having Go installed. The `xlsx-discover` tool tests require the `xlsx_discover` build tag and the presence of the XLSX protocol file at the project root; those tests are gated with `t.Skip()` when the file is absent.

**Dependencies for testing:**

| Dependency | Version | Purpose |
|---|---|---|
| `testing` (stdlib) | Go 1.26 | Test runner, subtests, benchmarks |
| `github.com/stretchr/testify` | v1.11.1 | `assert` and `require` helpers |
| `net/http/httptest` (stdlib) | Go 1.26 | HTTP handler testing |
| `github.com/gorilla/websocket` | v1.5.3 | WebSocket upgrade testing |

## Running tests

**Run the full test suite:**

```bash
make test
```

This executes `go test ./...`, which discovers and runs all `*_test.go` files across all packages. Note that the `tools/xlsx-discover` tests are excluded by default because they require the `xlsx_discover` build tag.

**Run tests for the xlsx-discover tool:**

```bash
make test-discover
```

This executes `go test -tags xlsx_discover ./tools/xlsx-discover/ -v -count=1`.

**Run tests for a specific package:**

```bash
go test ./internal/modbus/ -v -count=1
go test ./internal/broker/ -v -count=1
go test ./internal/register/ -v -count=1
go test ./internal/hub/ -v -count=1
go test ./web/ -v -count=1
```

**Run a single test by name:**

```bash
go test ./internal/modbus/ -v -run TestCRC16
go test ./internal/hub/ -v -run TestPackSpanDegradation
```

**Typical test durations:**

| Package | Approximate Duration |
|---|---|
| `internal/modbus` | < 1s |
| `internal/register` | < 1s |
| `web` | < 1s |
| `internal/broker` | ~2s |
| `internal/hub` | ~2-3 min (many time-sensitive integration tests) |

The `internal/hub` package contains the largest test suite with time-dependent behavior (read cycles, reconnection, span degradation) which accounts for its longer runtime.

## Test file structure

Tests follow Go conventions: test files are co-located with the source files they test, using the `_test.go` suffix.

```
internal/
  modbus/
    modbus_test.go          # 7 tests: CRC16, ReadFull, TCP/RTU read/write, exceptions
  broker/
    broker_test.go          # 14 tests: connection lifecycle, reconnect, batch reads, serialization
  hub/
    hub_test.go             # 72 tests: hub orchestration, section management, batch streaming
    batch_test.go           # 17 tests: SpanTracker state machine (3-state degradation)
    export_test.go          # Test helpers: exports unexported Hub methods for external tests
  register/
    register_test.go        # 67 tests: value formatting, section definitions, probe groups
    batch_test.go           # 13 tests: batch plan analysis, span merging, max register limits
web/
  web_test.go               # 7 tests: HTTP endpoints, WebSocket upgrade, static file serving
tools/
  xlsx-discover/
    main_test.go            # 8 tests: XLSX parsing, register comparison (build-tag gated)
```

**Total: ~205 test functions** across 9 test files.

## Writing new tests

### File naming

Place test files alongside the source code they test, using the `_test.go` suffix:

- `internal/modbus/modbus_test.go` tests `internal/modbus/common.go`, `tcp.go`, `rtu.go`
- `internal/register/batch_test.go` tests `internal/register/batch.go`

### Package naming

Tests use two patterns depending on what they need to access:

- **White-box tests** use the same package name (e.g., `package modbus` in `modbus_test.go`) to access unexported functions directly.
- **Black-box tests** use the `_test` suffix (e.g., `package hub_test` in `hub_test.go`) to test only the public API.

The `internal/hub/export_test.go` file bridges this gap by exporting unexported methods (e.g., `GetSectionProbes`, `GetSectionBatchPlan`) for use in black-box tests.

### Test helpers

| Helper | Location | Purpose |
|---|---|---|
| `discardLogger()` | `internal/modbus/modbus_test.go` | Returns an `*slog.Logger` that discards all output |
| `mockModbusServer()` | `internal/broker/broker_test.go` | TCP listener that responds to Modbus TCP requests with configurable register values |
| `slowMockServer()` | `internal/broker/broker_test.go` | TCP listener that holds connections open to simulate unresponsive devices |
| `buildReadResponse()` | `internal/broker/broker_test.go` | Constructs valid Modbus TCP read responses |
| `buildWriteResponse()` | `internal/broker/broker_test.go` | Constructs valid Modbus TCP write responses |
| `buildExceptionResponse()` | `internal/broker/broker_test.go` | Constructs Modbus TCP exception responses |
| `newMockBroker()` | `internal/hub/hub_test.go` | Mock implementation of `hub.BrokerInterface` with configurable batch results, write errors, and per-address register data |
| `hub.NewTestHub()` | `internal/hub/export_test.go` | Creates a Hub with default settings for testing |
| `hub.NewTestClient()` | `internal/hub/export_test.go` | Creates a Client with a provided send channel (no WebSocket needed) |
| `hub.SendReadCycle()` | `internal/hub/export_test.go` | Sends a `read_cycle` command to the hub |
| `newTestRouter()` | `web/web_test.go` | Creates a chi router wired to a disconnected broker and hub |

### Mock patterns

The project uses hand-written mocks rather than a mocking framework:

- **Modbus protocol mocks** (`broker_test.go`): Use `net.Pipe()` or `net.Listen("tcp", "127.0.0.1:0")` to create real TCP connections with mock servers that respond with valid Modbus TCP/RTU frames.
- **Broker mock** (`hub_test.go`): A `mockBroker` struct implements `hub.BrokerInterface` with fields for controlling batch results, write errors, per-address register data, and span failure addresses. Supports per-call result queuing via `batchResultQueue`.
- **HTTP mocks** (`web_test.go`): Uses `net/http/httptest` for request/response testing with `httptest.NewRecorder()` and `httptest.NewServer()` for WebSocket upgrade tests.

### Test naming conventions

Tests follow the `Test<Function>` or `Test<Type>_<Behavior>` pattern:

```go
func TestCRC16(t *testing.T) { ... }
func TestReadHoldingRegistersTCP_Exception(t *testing.T) { ... }
func TestSpanTracker_DegradeAtThreshold(t *testing.T) { ... }
func TestBrokerReconfigureWhileConnected(t *testing.T) { ... }
```

Table-driven tests use `t.Run()` for subtests:

```go
func TestStatusLabel(t *testing.T) {
    tests := []struct {
        name   string
        entry  ComparisonEntry
        expect string
    }{
        {"all three", ComparisonEntry{...}, "all three"},
        {"XLSX only", ComparisonEntry{...}, "XLSX only"},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // ...
        })
    }
}
```

### Assertion style

Most tests use standard `t.Errorf()` / `t.Fatalf()` assertions. The `register/batch_test.go` and parts of `hub/hub_test.go` use testify:

```go
require.NotEmpty(t, plan.Spans, "pack batch plan must have spans")
assert.Equal(t, uint16(0x9044), plan.Spans[0].StartAddr, "first span should start at 0x9044")
```

Both styles are acceptable in the project.

## Coverage requirements

No coverage threshold is configured. Coverage can be collected manually:

```bash
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out
go tool cover -html=coverage.out
```

## CI integration

No CI/CD pipeline is detected. Tests are run locally via `make test` or `go test ./...`. There are no GitHub Actions workflows configured for automated testing.

To ensure test reliability before committing:

```bash
make test
```

For the full suite including the xlsx-discover tool:

```bash
make test && make test-discover
```
