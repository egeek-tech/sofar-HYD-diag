<!-- generated-by: gsd-doc-writer -->
# API

Sofar HYD Diagnostic Tool exposes two HTTP endpoints and one WebSocket endpoint. The HTTP endpoints provide server status and configuration defaults. The WebSocket endpoint is the primary communication channel, carrying all real-time inverter data and control commands as JSON messages.

## Authentication

No authentication is required. The tool is designed for local network use on a trusted network segment where the Modbus inverter resides. The WebSocket upgrader accepts all origins (`CheckOrigin` returns `true`).

## Endpoints overview

| Method | Path | Description | Auth Required |
|--------|------|-------------|---------------|
| GET | `/api/status` | Server uptime, connection state, and inverter address | No |
| GET | `/api/defaults` | CLI flag defaults for the browser connection form | No |
| GET | `/ws` | WebSocket upgrade for real-time data streaming | No |
| GET | `/*` | Embedded static files (HTML, CSS, JS frontend) | No |

## HTTP endpoints

### GET /api/status

Returns the current server status as JSON.

**Response:**

```json
{
  "uptime": "1h23m45s",
  "connection_state": "connected",
  "inverter_addr": "10.5.99.29:4192"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `uptime` | string | Time since server start, rounded to seconds (Go `time.Duration` format) |
| `connection_state` | string | One of: `dormant`, `disconnected`, `connecting`, `connected`, `reconnecting` |
| `inverter_addr` | string | Configured inverter address in `host:port` format |

### GET /api/defaults

Returns the CLI flag default values as JSON. The browser uses these to pre-populate the connection form on page load.

**Response:**

```json
{
  "host": "10.5.99.29",
  "port": 4192,
  "slave_id": 1,
  "pv_channels": 2
}
```

| Field | Type | Description |
|-------|------|-------------|
| `host` | string | Inverter IP address from `-inverter-host` flag |
| `port` | int | Inverter TCP port from `-inverter-port` flag |
| `slave_id` | int | Modbus slave ID from `-slave` flag (1-247) |
| `pv_channels` | int | Default PV channel count from `-pv-channels` flag (2-16) |

## WebSocket protocol

The `/ws` endpoint upgrades HTTP to WebSocket. All messages are JSON text frames. The protocol is asymmetric: inbound messages (browser to server) are commands; outbound messages (server to browser) carry data and state.

### Connection lifecycle

1. Client opens a WebSocket connection to `/ws`.
2. Server immediately sends a `connection_state` message with the current broker state.
3. Client sends a `connect` message with inverter address and slave ID.
4. Server begins connecting and streams `connection_state` updates.
5. Once connected, the client sends `subscribe` to select a data section.
6. Server streams `section_schema`, then `register_value` messages, then `section_complete`.
7. Client sends `read_cycle` to trigger subsequent data refreshes.

### Connection parameters

- **Ping interval:** 30 seconds (server to client)
- **Pong timeout:** 60 seconds (client must respond within this window)
- **Write timeout:** 10 seconds per message
- **Max inbound message size:** 4096 bytes
- **Client send buffer:** 256 messages (slow clients are disconnected)

### Inbound message types (browser to server)

All inbound messages share this base shape:

```json
{
  "type": "<message_type>",
  "section": "<section_name>"
}
```

Additional fields depend on the message type:

#### connect

Initiates a connection to the inverter. The broker performs a single dial attempt with a 5-second timeout.

```json
{
  "type": "connect",
  "host": "10.5.99.29",
  "port": 4192,
  "slave_id": 1
}
```

#### disconnect

Closes the inverter connection. Cancels all in-progress section reads. The broker enters a dormant state and will not auto-reconnect until a new `connect` is sent.

```json
{
  "type": "disconnect"
}
```

#### subscribe

Subscribes to a data section. Only one section subscription is active per client at a time; subscribing to a new section automatically unsubscribes from the previous one. Triggers an immediate data read if the broker is connected.

```json
{
  "type": "subscribe",
  "section": "system"
}
```

Available sections: `system`, `configuration`, `grid`, `eps`, `pv`, `battery`, `bms`, `meter`, `pcu`, `bdu`.

#### unsubscribe

Unsubscribes from a data section. Cancels any in-progress read if no subscribers remain.

```json
{
  "type": "unsubscribe",
  "section": "system"
}
```

#### refresh

Forces a re-read of the specified section. For `configuration` (a read-once section), this resets the cache so the next read fetches fresh data from the inverter.

```json
{
  "type": "refresh",
  "section": "configuration"
}
```

#### read_cycle

Triggers a section read cycle. Used by the browser to drive periodic refresh. Skipped if a read is already in progress or if the broker is disconnected. For cached read-once sections (`configuration`), the read is skipped unless the cache has been reset via `refresh`.

```json
{
  "type": "read_cycle",
  "section": "system"
}
```

#### configure

Reconfigures a section at runtime.

**PV channel count:**

```json
{
  "type": "configure",
  "section": "pv",
  "config": {
    "channels": 4
  }
}
```

The `channels` value is clamped to the range 2-16. Rebuilds the PV section probe groups and triggers an immediate re-read.

**Timing parameters:**

```json
{
  "type": "configure",
  "section": "timing",
  "timing_config": {
    "read_delay_ms": 500,
    "pack_settle_ms": 1000
  }
}
```

| Field | Range | Default | Description |
|-------|-------|---------|-------------|
| `read_delay_ms` | 50-5000 | 500 | Minimum delay between consecutive Modbus reads (ms) |
| `pack_settle_ms` | 500-10000 | 1000 | Delay after BMS pack selection write before reading (ms) |

#### select_pack

Selects a specific battery pack for BMS drill-down. Writes the pack query word to register 0x9020, waits for the settle time, then reads pack-level registers (cell voltages, temperatures, status).

```json
{
  "type": "select_pack",
  "input": 1,
  "tower": 1,
  "pack": 1
}
```

| Field | Range | Description |
|-------|-------|-------------|
| `input` | 1 | Battery input (clamped to 1; single input supported) |
| `tower` | 1-2 | Tower/string index |
| `pack` | 1-10 | Pack/module index within the tower |

### Outbound message types (server to browser)

#### connection_state

Broadcast to all clients when the broker connection state changes.

```json
{
  "type": "connection_state",
  "state": "connected",
  "error": ""
}
```

States: `dormant`, `disconnected`, `connecting`, `connected`, `reconnecting`. The `error` field is non-empty when the state change was caused by an error (e.g., connection timeout, network reset).

#### section_schema

Sent to a client immediately after `subscribe` (or after pack selection). Describes the section layout so the frontend can pre-render placeholder slots before data streams in.

```json
{
  "type": "section_schema",
  "section": "system",
  "groups": [
    {
      "name": "Identity",
      "registers": ["Inverter SN"]
    },
    {
      "name": "Status",
      "layout": "column",
      "registers": ["Running state", "Grid-connected wait time", "Power gen time today", "System time"]
    }
  ]
}
```

For pack drill-down, the message includes a `pack_context` identifying the selected pack:

```json
{
  "type": "section_schema",
  "section": "bms",
  "groups": [...],
  "pack_context": {
    "input": 1,
    "tower": 1,
    "pack": 3
  }
}
```

Group types that may appear: `""` (standard data rows), `"bitmap"` (BMS topology widget), `"protection"` (protection/alarm card), `"cell_grid"` (cell voltage grid with `cell_count` field).

#### register_value

Streamed one per register as each Modbus read completes. Allows the frontend to display values progressively rather than waiting for an entire section.

```json
{
  "type": "register_value",
  "section": "system",
  "group": "Status",
  "name": "Running state",
  "value": "Normal",
  "register_addr": 1028,
  "raw_value": "0x0002"
}
```

On read error, `value` is empty and `error` contains the error string:

```json
{
  "type": "register_value",
  "section": "system",
  "group": "Temperatures",
  "name": "Ambient temp 1",
  "value": "",
  "error": "exception: func=0x83 err=0x02",
  "register_addr": 1048,
  "raw_value": ""
}
```

#### section_complete

Signals that all registers in a section have been read for this cycle. The frontend can use this as a "read finished" marker.

```json
{
  "type": "section_complete",
  "section": "system",
  "timestamp": "2026-04-18T10:30:00Z"
}
```

#### section_data

Used for grouped data payloads (BMS topology bitmap, protection/alarm cards, and fault data). Carries pre-formatted groups rather than individual register values.

```json
{
  "type": "section_data",
  "section": "bms",
  "groups": [
    {
      "name": "Battery Topology",
      "type": "bitmap",
      "bitmap": {
        "towers": 2,
        "packs_per_tower": 10,
        "online": [1023, 1023],
        "detected_topology": "2 strings x 10 packs",
        "mismatch": false
      }
    },
    {
      "name": "Protection & Alarms",
      "type": "protection",
      "items": {
        "BMS Protection 1": "0x0000",
        "BMS Protection 2": "0x0000"
      }
    }
  ],
  "faults": [],
  "timestamp": "2026-04-18T10:30:00Z"
}
```

The `faults` array is always present (never omitted) so the frontend can always render the fault card. Each entry has a `name` string describing the active fault.

#### section_error

Sent when a section read cannot proceed (e.g., not connected to inverter).

```json
{
  "type": "section_error",
  "section": "system",
  "error": "not connected to inverter",
  "timestamp": "2026-04-18T10:30:00Z"
}
```

#### pack_data

Response to `select_pack` with pack-level register data. Contains groups for cell voltages, temperatures, pack status, and balance bitmaps.

```json
{
  "type": "pack_data",
  "section": "bms",
  "input": 1,
  "tower": 1,
  "pack": 3,
  "groups": [
    {
      "name": "Pack Summary",
      "items": {
        "Pack Voltage": "51.2 V",
        "Pack Current": "12.50 A"
      },
      "item_meta": {
        "Pack Voltage": {"register_addr": 36961, "raw_value": "0x0200"}
      }
    },
    {
      "name": "Cell Voltages",
      "type": "cell_grid",
      "cells": [3312, 3311, 3310, 3315, 3312, 3313, 3311, 3314, 3310, 3312, 3313, 3311, 3310, 3315, 3312, 3311],
      "cell_addrs": [36929, 36930, 36931, 36932, 36933, 36934, 36935, 36936, 36937, 36938, 36939, 36940, 36941, 36942, 36943, 36944],
      "max_cell": 3315,
      "min_cell": 3310,
      "max_cell_index": 4,
      "min_cell_index": 3
    },
    {
      "name": "Pack Status",
      "type": "pack_status",
      "alarm": 0,
      "protection": 0,
      "fault": 0,
      "decoded": []
    }
  ],
  "timestamp": "2026-04-18T10:30:00Z"
}
```

#### pack_error

Sent when a pack selection or read fails.

```json
{
  "type": "pack_error",
  "section": "bms",
  "input": 1,
  "tower": 1,
  "pack": 3,
  "error": "timeout writing pack selection after retry"
}
```

## Data sections

The following data sections are registered on startup. Each section maps to a set of Modbus register groups read from the inverter.

| Section | Description | Read behavior |
|---------|-------------|---------------|
| `system` | Identity, firmware versions, running state, temperatures, fault registers | Standard streaming read with fault decoding |
| `configuration` | Inverter configuration settings (work mode, grid standard, limits) | Read-once with cache; re-read on explicit `refresh` |
| `grid` | Grid voltage, current, frequency, and power for all phases | Standard streaming read |
| `eps` | Emergency power supply (off-grid) voltage, current, frequency, power | Standard streaming read |
| `pv` | PV channel voltage, current, power (dynamically sized via `configure`) | Standard streaming read; 2-16 channels |
| `battery` | Per-channel battery voltage, current, SOC, SOH, cycles, charge/discharge limits | Auto-detects channel count from register 0x066A on each read |
| `bms` | BMS overview: topology bitmap, protection/alarm registers; supports pack drill-down | Custom read cycle with topology detection and pack selection |
| `meter` | External meter measurements | Standard streaming read |
| `pcu` | Power conversion unit registers | Standard streaming read |
| `bdu` | Battery distribution unit registers | Standard streaming read |

## Error codes

All error responses use the same JSON envelope with a `type` indicating the error category and an `error` field containing a human-readable description.

| Error type | Context | Description |
|------------|---------|-------------|
| `section_error` | Section read failure | Broker is disconnected, section is unknown, or read failed |
| `pack_error` | Pack drill-down failure | Pack selection write timed out or pack registers unreadable |

Modbus-level errors surface as `register_value` messages with an empty `value` and populated `error` field. Common Modbus errors:

| Error pattern | Meaning |
|---------------|---------|
| `exception: func=0x83 err=0x02` | Illegal data address -- register does not exist on this hardware |
| `exception: func=0x83 err=0x04` | Slave device failure -- transient hardware error |
| `timeout waiting for response` | Inverter did not respond within the read deadline (10 seconds) |
| `broker is dormant` | No connection has been initiated; send a `connect` message first |

## Rate limits

No application-level rate limiting is implemented. The Modbus protocol layer enforces a configurable inter-read delay (default 500ms, range 50-5000ms) between consecutive register reads to respect the inverter's hardware timing constraints. Only one TCP connection to the inverter exists at a time, and all reads are serialized through the broker, which acts as a natural throughput limiter.
