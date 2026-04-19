<!-- generated-by: gsd-doc-writer -->
# Sofar HYD Diagnostic Tool

A desktop-focused web application for monitoring and diagnosing Sofar HYD hybrid inverters via TCP Modbus, presenting real-time register data through a structured browser interface.

[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)

## Installation

### Prerequisites

- **Go 1.26** or later
- Network connectivity to a Sofar HYD inverter on its Modbus TCP port (default 4192)

### Build from source

```bash
git clone git@github.com:richie-tt/sofar-HYD-diag.git
cd sofar-HYD-diag
make server
```

This produces a single `server` binary with the HTML/JS/CSS frontend embedded inside it. No external runtime dependencies are required.

## Quick Start

1. Build the server binary:
   ```bash
   make server
   ```

2. Run it, pointing at your inverter's IP address:
   ```bash
   ./server -inverter-host 192.168.1.100
   ```

3. Open your browser to `http://localhost:8080` and use the sidebar to connect and subscribe to data sections.

## Usage

### Command-line flags

| Flag | Default | Description |
|------|---------|-------------|
| `-listen` | `:8080` | HTTP listen address (host:port) |
| `-inverter-host` | `10.5.99.29` | Inverter IP address (pre-populates browser UI) |
| `-inverter-port` | `4192` | Inverter TCP port |
| `-slave` | `1` | Modbus slave ID (1--247) |
| `-modbus-mode` | `tcp` | Modbus protocol mode: `tcp` or `rtu` |
| `-pv-channels` | `2` | Default number of PV channels (2--16, pre-populates browser dropdown) |
| `-log-level` | `info` | Log level: `debug`, `info`, `warn`, `error` |

### Example: custom listen port and RTU mode

```bash
./server -listen :9090 -inverter-host 10.0.0.50 -modbus-mode rtu
```

### Data sections

The web UI organizes inverter data into subscribable sections, each backed by register groups from the Sofar Modbus-G3 V1.38 protocol:

- **System** -- Identity, firmware versions, running state, temperatures, and system time
- **PV** -- Photovoltaic input power and voltage per channel
- **Battery** -- Per-channel voltage, current, power, SOC, SOH, and cycle counts
- **BMS** -- Battery Management System pack topology with online/offline bitmap visualization
- **PCU** -- Power Conversion Unit operating parameters
- **DCDC** -- DC-DC converter metrics
- **Meter** -- Grid meter readings
- **Statistics** -- Cumulative energy generation and consumption totals
- **Configuration** -- Inverter configuration registers
- **Faults** -- Active fault and protection alarm status

### WebSocket protocol

The browser communicates with the server over a single WebSocket connection at `/ws`. Clients send JSON messages to subscribe to sections, trigger reads, connect/disconnect to the inverter, and select battery packs for BMS inspection. The server streams register values back as they are read, providing per-register real-time updates.

### REST API

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/status` | Server uptime, connection state, and inverter address |
| `GET` | `/api/defaults` | CLI flag defaults for pre-populating the browser UI |

## Architecture Overview

The application is structured as four layers:

- **`cmd/server`** -- Entry point: CLI flag parsing, signal handling, HTTP server lifecycle
- **`internal/broker`** -- Serialized Modbus connection manager with command queue, retry logic, and automatic reconnection
- **`internal/hub`** -- WebSocket hub managing client subscriptions, section-based read scheduling, and batch read orchestration
- **`internal/modbus`** -- Protocol implementations for Modbus TCP (MBAP framing) and Modbus RTU (CRC-16 checksums)
- **`internal/register`** -- Register map definitions (probe/probe-group structs), data formatting, and batch planning
- **`web`** -- Embedded static frontend (HTML/JS/CSS) and HTTP route setup via chi router

## Additional tools

The `tools/` directory contains supplementary utilities:

- **`tools/xlsx-discover`** -- Parses Sofar XLSX protocol specification files to discover register definitions
- **`tools/config-sweep`** -- Scans configuration register ranges
- **`tools/section-sweep`** -- Sweeps all section registers for diagnostic purposes

Build the XLSX discovery tool separately:

```bash
make discover
```

## Running tests

```bash
make test
```

This runs `go test ./...` across all packages. To run tests for the XLSX discovery tool (requires the `xlsx_discover` build tag):

```bash
make test-discover
```

## License

This project is licensed under the GNU General Public License v3.0. See [LICENSE](LICENSE) for details.
