# Technology Stack

**Analysis Date:** 2026-04-10

## Languages

**Primary:**
- Go 1.26.1 - Single-file application for Modbus protocol communication

## Runtime

**Environment:**
- Go runtime (compiled binary available)

**Package Manager:**
- Go modules (go.mod)
- No external dependencies (stdlib only)
- No lockfile (go.sum not present - pure stdlib implementation)

## Frameworks

**Core:**
- Standard library `net` - TCP/IP socket communication and Modbus TCP transport
- Standard library `encoding/binary` - Binary data serialization (big-endian register parsing)
- Standard library `flag` - CLI argument parsing for host, port, slave ID, mode selection
- Standard library `fmt` - Output formatting and logging to stdout

**Utilities:**
- Standard library `time` - Connection timeouts, retry delays, scheduling
- Standard library `sync/atomic` - Thread-safe transaction ID counter for Modbus TCP

## Key Dependencies

**Critical:**
- None - Pure Go standard library implementation

**Infrastructure:**
- Standard library `net` provides TCP/IP connectivity for Modbus TCP protocol
- Standard library `encoding/binary` handles big-endian multi-register data parsing
- CRC-16 implementation: custom function `crc16()` in `main.go` for RTU mode checksum validation

## Configuration

**Environment:**
- Configured via CLI flags (no environment variables used)
- Connection parameters: `-host`, `-port`, `-slave`, `-mode`
- Query parameters: `-pack`, `-group`

**Build:**
- Standard Go build (no build configuration files present)
- Binary compiled: `modbus_reader` (executable at `/data/git/private/modbus_reader/modbus_reader`)

## Platform Requirements

**Development:**
- Go 1.26.1 or compatible (language features: generics via `range` over integers in 1.22+)

**Production:**
- Linux/amd64 (binary present)
- Network connectivity to Modbus TCP device on port 4192 (default)
- No external dependencies or runtime libraries required

## Protocol Support

**Modbus Transport:**
- Modbus TCP (MBAP header + PDU, RFC-compliant with transaction ID matching)
- Modbus RTU (CRC-16 checksums, binary serial mode)
- Dual-mode support: selectable via `-mode tcp` or `-mode rtu`

**Modbus Functions:**
- Function 0x03: Read Holding Registers
- Function 0x06: Write Single Register (RTU only)
- Function 0x10: Write Multiple Registers (TCP, used for 0x9020 BMS_Inquire)

**Device Target:**
- Sofar Inverter G3 (Modbus protocol specification: SOFAR_Modbus_Protocol_English_G3_V1.29)
- Register map: Inverter (0x04xx-0x05xx), Battery (0x06xx), BMS (0x90xx-0x90xx), Battery clusters (0x94xx)
- Supports battery pack topology: 4 strings × 10 packs per string (configurable via 0x900D)

---

*Stack analysis: 2026-04-10*
