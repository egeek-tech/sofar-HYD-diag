<!-- generated-by: gsd-doc-writer -->
# Configuration

Sofar HYD Diagnostic Tool is configured entirely via CLI flags passed to the server binary. There are no environment variables, config files, or `.env` files. All settings have sensible defaults and can be overridden at startup.

## CLI Flags

The server binary (`./server`) accepts the following flags:

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `-listen` | `string` | `:8080` | No | HTTP listen address in `host:port` format. Use `:8080` for all interfaces or `127.0.0.1:8080` for localhost only. |
| `-inverter-host` | `string` | `10.5.99.29` | No | Inverter IP address. Pre-populates the connection form in the browser UI; the actual connection is initiated from the frontend. |
| `-inverter-port` | `int` | `4192` | No | Inverter Modbus TCP port. |
| `-slave` | `int` | `1` | No | Modbus slave ID. Must be 1-247. |
| `-modbus-mode` | `string` | `tcp` | No | Modbus protocol mode: `tcp` or `rtu`. |
| `-pv-channels` | `int` | `2` | No | Default number of PV channels. Must be 2-16. Pre-populates the PV channel dropdown in the browser UI. |
| `-log-level` | `string` | `info` | No | Log level for structured logging output. Accepts: `debug`, `info`, `warn`, `error` (case-insensitive). |

**Example:**

```bash
./server -listen :9090 -inverter-host 192.168.1.100 -inverter-port 4192 -slave 1 -log-level debug
```

## Input Validation

The server validates flag values at startup and exits with an error message if any are out of range:

- **Slave ID** must be 1-247. Values outside this range cause an immediate exit with: `error: slave ID must be 1-247, got <value>`.
- **Modbus mode** must be exactly `tcp` or `rtu`. Any other value causes: `error: modbus-mode must be 'tcp' or 'rtu', got "<value>"`.
- **PV channels** must be 2-16. Values outside this range cause: `error: pv-channels must be 2-16, got <value>`.

No other flags are validated at startup. The listen address and inverter host/port are passed through to the HTTP server and Modbus broker respectively, where connection errors are reported at runtime.

## Runtime Configuration via WebSocket

Several parameters can be changed at runtime through the browser UI without restarting the server. These are sent as WebSocket messages from the frontend.

### Connection Parameters

The browser sends a `connect` message with `host`, `port`, and `slave_id` fields to initiate or change the inverter connection. This triggers a broker reconfiguration with a single 5-second dial attempt.

A `disconnect` message closes the current connection and puts the broker into a dormant state until the next `connect`.

### Timing Parameters

A `configure` message with `section: "timing"` and a `timing_config` payload adjusts read timing:

| Parameter | Field | Default | Range | Description |
|-----------|-------|---------|-------|-------------|
| Inter-read delay | `read_delay_ms` | 500 ms | 50-5000 ms | Minimum delay between consecutive Modbus register reads. Lower values increase read speed but may overwhelm the inverter. |
| Pack settle time | `pack_settle_ms` | 1000 ms | 500-10000 ms | Delay after writing a pack selection register (0x9020) before reading pack data. Allows the BMS time to respond. |

Values outside the allowed range are clamped server-side to the nearest boundary. The inter-read delay update is propagated to the Modbus broker at runtime without reconnection.

### PV Channel Count

A `configure` message with `section: "pv"` and a `config` payload containing `channels` (2-16) rebuilds the PV section's register map and triggers an immediate re-read if subscribers are connected.

## Hardcoded Constants

The following values are compiled into the binary and cannot be changed without rebuilding:

### Network Timeouts

| Constant | Value | Location | Description |
|----------|-------|----------|-------------|
| Connection dial timeout | 5 s | `internal/modbus/common.go` | TCP dial timeout when connecting to the inverter. |
| Write deadline | 3 s | `internal/modbus/tcp.go` | Socket write timeout for Modbus requests. |
| Read deadline | 10 s | `internal/modbus/tcp.go` | Socket read timeout for Modbus responses. |
| Graceful shutdown timeout | 5 s | `cmd/server/main.go` | Maximum time to wait for HTTP server shutdown on SIGINT/SIGTERM. |

### WebSocket Parameters

| Constant | Value | Location | Description |
|----------|-------|----------|-------------|
| Write wait | 10 s | `internal/hub/client.go` | Timeout for writing a WebSocket message to a client. |
| Pong wait | 60 s | `internal/hub/client.go` | Maximum time to wait for a pong response before considering the client dead. |
| Ping period | 30 s | `internal/hub/client.go` | Interval between WebSocket ping frames (must be less than pong wait). |
| Max message size | 4096 bytes | `internal/hub/client.go` | Maximum inbound WebSocket message size. |
| Send buffer size | 256 messages | `internal/hub/client.go` | Per-client outbound message channel buffer. Slow clients whose buffer fills are disconnected. |

### Broker and Retry Parameters

| Constant | Value | Location | Description |
|----------|-------|----------|-------------|
| Read retry attempts | 3 | `internal/broker/broker.go` | Maximum attempts per register read before returning an error. |
| Write retry attempts | 2 | `internal/broker/broker.go` | Maximum attempts per register write before returning an error. |
| Command channel buffer | 32 | `internal/broker/broker.go` | Broker command queue depth. |
| Backoff base | 1 s | `internal/broker/broker.go` | Initial reconnection delay (exponential backoff). |
| Backoff max | 30 s | `internal/broker/broker.go` | Maximum reconnection delay cap. |
| Degradation threshold | 3 failures | `internal/hub/batch.go` | Consecutive batch failures before a register span degrades to individual reads. |
| Probe interval | 10 cycles | `internal/hub/batch.go` | Read cycles between recovery probe attempts for degraded/skipped spans. |

### Battery Topology

| Constant | Value | Location | Description |
|----------|-------|----------|-------------|
| Battery inputs | 1 | `internal/hub/hub.go` | Number of battery inputs supported. |
| Towers per input | 2 | `internal/hub/hub.go` | Number of battery towers (strings) per input. |
| Packs per tower | 10 | `internal/hub/hub.go` | Number of battery packs (modules) per tower. |
| Cells per pack | 16 | `internal/hub/hub.go` | Number of cells per battery pack. |

## Defaults Endpoint

The server exposes `GET /api/defaults` which returns the CLI flag defaults as JSON. The browser uses this to pre-populate the connection form on page load:

```json
{
  "host": "10.5.99.29",
  "port": 4192,
  "slave_id": 1,
  "pv_channels": 2
}
```

The values in this response reflect whatever was passed via CLI flags at startup.

## Log Output

Structured logging uses Go's `slog` package with text format to stdout. The log level is set once at startup via `-log-level` and cannot be changed at runtime. Components tag their log entries with a `component` field (`broker`, `hub`, `ws-client`).

Example output at `info` level:

```
time=2026-04-18T10:00:00.000Z level=INFO msg="server started" addr=:8080 inverter=10.5.99.29:4192 mode=tcp
time=2026-04-18T10:00:05.000Z level=INFO msg="connection state changed" component=broker state=connected
```

At `debug` level, individual Modbus read/write operations and HTTP requests are also logged.
