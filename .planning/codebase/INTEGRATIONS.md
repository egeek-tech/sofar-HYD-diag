# External Integrations

**Analysis Date:** 2026-04-10

## APIs & External Services

**Hardware Device Integration:**
- Sofar Inverter G3 with Modbus TCP/RTU interface
  - Protocol: Sofar Modbus G3 (version 1.29, register-based)
  - Connection: TCP (default) or RTU serial mode
  - Default endpoint: `10.5.99.29:4192`
  - Communication: Polling-only (read and write to device registers)

## Data Storage

**Databases:**
- Not applicable - no persistence layer

**File Storage:**
- Local filesystem only (reference documentation)
  - Device specification: `Sofar_Inverter_MODBUS_V1.38_EN.pdf`
  - Protocol register map: `SOFAR_Modbus_Protocol_English_G3_V1.29(only 3PH).xlsx`

**Caching:**
- Not used - all reads are live queries to device

## Authentication & Identity

**Auth Provider:**
- Not applicable - direct TCP connection to device

**Access Control:**
- Modbus slave ID selection (1-247, default: 1)
- No encryption or authentication in Modbus protocol

## Monitoring & Observability

**Error Tracking:**
- Not externally reported - errors logged to stdout only

**Logs:**
- Approach: Console output via `fmt.Printf` and `log.Fatalf`
- Log locations: stdout only
- Retry logging: Prints retry attempts and timeout details inline with results
- Sample registers logged on startup: known working registers list at lines 16-40 in `main.go`

## CI/CD & Deployment

**Hosting:**
- Standalone binary (Linux/amd64)
- No cloud deployment framework

**CI Pipeline:**
- Not detected - no CI configuration present

**Build Artifacts:**
- Compiled Go binary: `modbus_reader`

## Environment Configuration

**Required environment variables:**
- None - all configuration via CLI flags

**CLI Parameters:**
- `-host` (default: `10.5.99.29`) - IP address of Modbus TCP device
- `-port` (default: `4192`) - TCP port
- `-slave` (default: `1`) - Modbus slave ID
- `-mode` (default: `tcp`) - Protocol: `tcp` or `rtu`
- `-pack` (default: `-1`) - Specific battery pack number (0-15) for detailed read
- `-group` (default: `0`) - Battery string/group number (0-3)

**Secrets location:**
- No secrets required - TCP connection is unauthenticated

## Webhooks & Callbacks

**Incoming:**
- None - read-only polling client

**Outgoing:**
- None - no external notifications

## Device Register Mapping

**System Information (0x04xx range):**
- Inverter serial number (0x0445)
- System running state (0x0404)
- System time (0x042C-0x0431)
- Ambient temperatures (0x0418-0x0419)
- Insulation impedance (0x042B)

**Grid Output (0x048x range):**
- Grid frequency, voltage, current, power (0x0484-0x04BC)
- Phase-specific measurements (R/S/T)

**Battery Info (0x06xx range):**
- Voltage, current, power, SOC, SOH (0x0604-0x0609)
- Charge/discharge limits and state (0x0644-0x0646)
- Pack health and cycle count (0x0669-0x066B)

**BMS Data (0x90xx range):**
- Manufacturer, version, cell type (0x9006-0x900C)
- Voltage, current, temperature (0x900E-0x9011)
- SOC, health, online bitmap (0x9012-0x9022)
- **Pack selection**: Write pack number to 0x9020 (BMS_Inquire)
- **Pack data**: Read 0x9044+ for cell voltages, temperatures, state
- Fault data (0x9084-0x90B9)
- Pack cycles, balancing, alarms (0x9074-0x9078)

**Battery Cluster Info (0x94xx range):**
- Not accessible in testing (BDU timeout noted in code)

**Connection Retry Strategy:**
- Max 3 retries with 500ms delays
- Reconnects on timeout
- Transaction ID matching prevents stale responses (TCP mode)

---

*Integration audit: 2026-04-10*
