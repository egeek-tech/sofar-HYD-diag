# Feature Landscape

**Domain:** Inverter diagnostic/monitoring web tool (Sofar HYD hybrid inverter via TCP Modbus)
**Researched:** 2026-04-10
**Confidence:** HIGH (based on Sofar Modbus-G3 V1.38 protocol spec, existing CLI tool verified against hardware, and domain knowledge of inverter monitoring tools like SolarAssistant, SolarMan, Home Assistant solar integrations, and Fronius/SMA web portals)

## Table Stakes

Features users expect from any inverter diagnostic/monitoring tool. Missing any of these and the tool feels broken or incomplete.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **Connection management** | Users need to connect/disconnect and see connection status at a glance | Low | IP, port, slave ID fields + connect button + status indicator. Already proven in CLI. |
| **System identification** | First thing any tech checks: which inverter am I talking to? | Low | Inverter SN (0x0445), running state (0x0404), system time (0x042C-0x0431), firmware versions. One-shot read. |
| **Real-time power overview** | The core reason to use the tool: what is the inverter doing right now? | Medium | Grid power, PV power, battery power, load power. Requires reading multiple register blocks with 500ms delays. |
| **Grid parameters display** | Essential for grid-tied diagnostics: voltage, frequency, current per phase | Medium | Grid-connected (0x0484-0x04BC): frequency, V/I/P per phase R/S/T, line voltages, PCC power, load power, power factor. |
| **PV input display** | Users need to see what their panels are producing | Low | PV1/PV2+ voltage/current/power (0x0584+), total PV power (0x05C4). Configurable channel count (2-16). |
| **Battery status overview** | Hybrid inverter = battery is central. SOC/SOH/power are essential at a glance. | Low | Bat voltage/current/power/SOC/SOH/cycles/state per channel (0x0604-0x0646). |
| **Visual connection feedback** | Users must know if data is fresh or stale | Low | Green flash on successful read, red on failure. Stale data indicator if refresh stops. |
| **Auto-refresh toggle** | Diagnostic sessions need continuous monitoring, not manual clicking | Medium | WebSocket push with on/off toggle. Must respect 500ms Modbus timing. |
| **Sectioned navigation** | Too much data to show at once. Users need to navigate to what they care about. | Medium | Tabs or collapsible sections: System Info, Grid, EPS, PV, Battery, Statistics. |
| **Lazy loading by section** | Modbus is slow (500ms between reads). Loading everything kills responsiveness. | Medium | Only read registers for the currently visible section. Critical for usability given protocol timing constraints. |
| **Electricity statistics** | Day/total generation, consumption, bought, sold, battery charge/discharge | Low | U32 registers at 0x0684-0x06B3. Simple display, no charting needed for table stakes. |
| **Error/fault display** | If the inverter has a fault, the user MUST see it immediately | High | Fault registers 0x0405-0x0416 are bitmaps with 300+ possible fault codes. Need human-readable fault name lookup from Appendix 6.1. |
| **EPS/off-grid status** | Hybrid inverters have emergency power mode. Users need to see if EPS is active. | Low | Load active power (0x0504), output voltage frequency (0x0507). Small section. |

## Differentiators

Features that set this tool apart from Sofar's own cloud portal (SolarMan/SOFAR Cloud) and generic Modbus tools. These are not expected but provide significant diagnostic value.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Deep battery pack drill-down** | No cloud portal shows individual cell voltages. This is THE killer feature for battery diagnostics. | High | Requires BMS_Inquire write (0x9020), 1s settle time, then reading 0x9044+ for cell voltages (24 cells), temperatures (8 sensors), SN, capacity, cycles, fault/alarm/protect states. Must iterate per-pack with pack switching. |
| **Battery topology visualization** | Show the full hierarchy: inputs > towers > packs with online/offline status | Medium | Online bitmap (0x9022), configurable topology (inputs x towers x packs). Tree or grid navigation showing which packs are online, hibernating (0x9023), or faulted. |
| **Cell voltage spread analysis** | Highlight imbalanced cells within a pack -- the #1 early warning sign of battery degradation | Medium | Read 24 cell voltages per pack (0x9051-0x9068), compute min/max/spread, color-code cells that deviate from mean. No competitor shows this at the cell level. |
| **BMS protection/alarm/fault decoding** | Raw bitmap values are useless. Decode alarm (0x9076), protection (0x9077), fault (0x9078) into human-readable text. | Medium | Requires mapping bit positions to fault descriptions (similar to system faults but for BMS layer). Pack-level fault data at 0x9084-0x90B9 adds fault history per pack. |
| **Battery cluster overview** | Show all towers in a cluster with per-tower BMS data at a glance | Medium | Cluster 1 (0x9400+) and Cluster 2 (0x9600+) provide per-BMS voltage/current/power/SOC/state for up to 8 BMS units each. Quick health comparison across towers. |
| **Configurable topology** | Different HYD models have different PV counts and battery layouts. One tool fits all. | Low | Settings page: PV channels (2-16), battery inputs (1-2), towers per input (1-4), packs per tower (4-10). Persisted in browser localStorage. |
| **Historical event log** | Last 100 fault events with timestamps, readable from the inverter's internal log | Medium | 0x1480-0x160F: 100 events, each with fault ID + timestamp. Decode fault ID via Appendix 6.1 lookup table. Presented as a scrollable table. |
| **System fault register decoding** | Decode the 20 fault registers (0x0405-0x0416) into human-readable fault names with severity | High | 20 registers x 16 bits = 320 possible faults. Full lookup table from Appendix 6.1 must be embedded. Group by category (grid, PV, battery, thermal, communication). |
| **Internal diagnostics** | Leakage current, DC components, bus voltages -- data only a technician needs | Low | Internal info (0x06C4-0x06EF): leakage current, balance current, DC components per phase, bus voltages, capacitor voltages. Low complexity to display, high value for troubleshooting. |
| **Confluence/string current monitoring** | Per-string current data for diagnosing PV string issues | Low | 0x0704-0x0733: up to 16 groups with voltage + 2 string currents each. Useful for large PV arrays to spot underperforming strings. |

## Anti-Features

Features to explicitly NOT build. Each would add complexity without matching the tool's purpose as a local diagnostic utility.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| **Historical data storage / database** | Adds persistence layer, backup concerns, storage management. This is a real-time diagnostic tool, not a monitoring platform. | Show live data + the inverter's built-in 100-event history log. Users who need long-term history already use SolarMan cloud or Home Assistant. |
| **Charts and graphs** | Tempting but wrong. Without persistent data, charts show only the current session. Real-time numbers update faster than charts can convey meaning. | Use numeric displays with color-coded ranges. A cell voltage spread bar per pack is more useful than a time-series chart of one cell. |
| **Register writing / control commands** | Writing to inverter configuration registers (safety params, energy management, time periods) risks misconfiguration. One wrong write to a safety register can trip the inverter. | Read-only diagnostic tool. The ONE exception already exists: BMS_Inquire (0x9020) write to select battery packs. No other writes. |
| **Multi-inverter support** | Modbus is inherently single-connection serial. Supporting multiple inverters means connection multiplexing, UI for switching, confusion about which inverter is shown. | Single inverter, single connection. User disconnects and reconnects to a different IP if needed. |
| **User authentication** | This runs on a local network, accessed from a laptop sitting next to the inverter. Auth adds friction to a diagnostic workflow. | No auth. If network security is needed, the user's network handles it. |
| **Mobile responsive design** | Diagnostic data is dense: 24 cell voltages, 3-phase grid params, battery hierarchy. It does not compress to a phone screen. | Desktop-only layout using full page width. The tool is used on a laptop at the inverter site. |
| **Cloud connectivity / remote access** | Adds security concerns, hosting costs, complexity. Sofar already has SolarMan cloud for remote monitoring. | Local network only. Single binary, zero external dependencies. |
| **Firmware update capability** | The protocol supports remote firmware upgrade (0x2034-0x203F). Extremely dangerous to expose. A failed firmware update can brick the inverter. | Do not even read the upgrade registers. Show firmware version info (read-only) in system info section only. |
| **MQTT / Home Assistant integration** | Scope creep. Good integrations already exist (sofar2mqtt, ha-sofar). This tool solves a different problem: deep diagnostics. | Stay focused on the web UI diagnostic use case. Users who want HA integration use existing community tools. |
| **Parameter configuration UI** | Safety parameters (0x0800+), energy management modes (0x1000+), time periods -- writable but dangerous without deep understanding. | Display current configuration values as read-only if useful for diagnostics, but never provide write UI. |
| **PDF/CSV export** | Adds file generation, download handling, format decisions. Low value for a real-time tool. | Users can screenshot or copy values. If export becomes needed later, it is a simple addition (not architectural). |

## Feature Dependencies

```
Connection Management
  |
  +---> System Identification (needs active connection)
  |
  +---> Real-time Power Overview (needs active connection)
  |      |
  |      +---> Grid Parameters (subset of power overview registers)
  |      +---> PV Input Display (separate register block)
  |      +---> EPS Status (separate register block)
  |
  +---> Battery Status Overview (needs active connection)
  |      |
  |      +---> Battery Topology Visualization (needs online bitmap + config)
  |      |      |
  |      |      +---> Deep Battery Pack Drill-down (needs topology + BMS_Inquire write)
  |      |             |
  |      |             +---> Cell Voltage Spread Analysis (needs pack drill-down data)
  |      |             +---> BMS Protection/Alarm/Fault Decoding (needs pack drill-down data)
  |      |
  |      +---> Battery Cluster Overview (separate register block, independent of pack drill-down)
  |
  +---> Error/Fault Display (needs active connection)
  |      |
  |      +---> System Fault Register Decoding (needs fault lookup table)
  |      +---> Historical Event Log (needs fault lookup table + event registers)
  |
  +---> Electricity Statistics (needs active connection, independent section)
  |
  +---> Auto-refresh Toggle (needs WebSocket infrastructure)
         |
         +---> Lazy Loading by Section (determines WHICH registers auto-refresh reads)

Configurable Topology ---> Battery Topology Visualization (determines hierarchy)
                     ---> PV Input Display (determines channel count)
```

## MVP Recommendation

Build these first, in this order:

1. **Connection management** -- without this, nothing works
2. **System identification** -- immediate proof the connection works, shows inverter SN
3. **Sectioned navigation + lazy loading** -- architectural foundation; adding sections later is easy if the skeleton exists
4. **Real-time power overview** (grid + PV + battery + load) -- the core dashboard
5. **Auto-refresh via WebSocket** -- transforms from manual polling to live monitoring
6. **Battery status overview** -- table stakes for a hybrid inverter tool
7. **Error/fault display with decoding** -- safety-critical; users must see active faults
8. **Deep battery pack drill-down** -- THE differentiator. Cell voltages, temperatures, per-pack status.

**Defer to later phases:**
- **Electricity statistics**: useful but not diagnostic-critical. Simple to add once sections exist.
- **Historical event log**: valuable but requires building the full fault ID lookup table (300+ codes). Can share the table with fault display once both are built.
- **Battery cluster overview**: depends on BDU being online (failed in testing with current hardware). Implement when hardware is available for testing.
- **Internal diagnostics**: niche, only useful for advanced troubleshooting. Low effort to add but low priority.
- **Confluence/string current monitoring**: only relevant for large PV arrays. Low priority for typical 2-channel HYD setups.

## Complexity Budget

| Complexity | Feature Count | Key Risk |
|------------|--------------|----------|
| Low | 7 features | Straightforward register reads + display |
| Medium | 8 features | WebSocket timing, section architecture, bitmap decoding, topology navigation |
| High | 3 features | Pack drill-down (BMS_Inquire sequencing + pack switching delays), fault register decoding (300+ codes), system fault parsing (20 registers x 16 bits) |

The highest-risk feature is **deep battery pack drill-down** because it involves:
- Write operation (0x9020) to select pack
- 1-second settle time after pack switch
- Sequential reads of multiple register blocks per pack
- Iterating across all packs in a tower (up to 10)
- Handling offline/hibernating packs gracefully
- All while maintaining WebSocket responsiveness

This feature should be built carefully with explicit user-triggered pack selection (not auto-scan of all packs) to keep the UX responsive.

## Sources

- Sofar_Inverter_MODBUS_V1.38_EN.pdf (V1.38, January 2025) -- complete register map, fault tables, protocol constraints
- Existing main.go CLI tool (707 lines) -- verified register reads against real Sofar HYD hardware (2026-03-16)
- Domain knowledge: SolarAssistant, SolarMan/SOFAR Cloud portal, Fronius SolarWeb, SMA Sunny Portal, Home Assistant solar integrations, sofar2mqtt community tool
- Confidence: HIGH for all features (protocol-verified), MEDIUM for anti-features (opinion-based, informed by ecosystem patterns)
