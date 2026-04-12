package register

import "fmt"

// GenerateBatteryGroups dynamically generates ProbeGroup definitions for N battery channels.
// Each channel has 7 pack info probes (section 5.1.7) at base 0x0604 + 7*(ch-1)
// and 3 state probes (section 5.1.8) at state base 0x0644 + 4*(ch-1).
// A final full-width "Global Stats" group is appended with aggregate battery metrics.
// From Sofar Modbus-G3 V1.38 sections 5.1.7-5.1.8.
func GenerateBatteryGroups(channels int) []ProbeGroup {
	groups := make([]ProbeGroup, 0, channels+1)
	for ch := 1; ch <= channels; ch++ {
		baseAddr := uint16(0x0604 + 7*(ch-1))
		stateBase := uint16(0x0644 + 4*(ch-1))
		groups = append(groups, ProbeGroup{
			Name:   fmt.Sprintf("Channel %d", ch),
			Layout: "column",
			Probes: []Probe{
				// 7 pack info probes (section 5.1.7)
				{Name: "Voltage", Addr: baseAddr, Count: 1, Unit: "V", Scale: 0.1},
				{Name: "Current", Addr: baseAddr + 1, Count: 1, Signed: true, Unit: "A", Scale: 0.01},
				{Name: "Power", Addr: baseAddr + 2, Count: 1, Signed: true, Unit: "kW", Scale: 0.01},
				{Name: "Env Temp", Addr: baseAddr + 3, Count: 1, Signed: true, Unit: "\u00b0C", Scale: 1},
				{Name: "SOC", Addr: baseAddr + 4, Count: 1, Unit: "%", Scale: 1},
				{Name: "SOH", Addr: baseAddr + 5, Count: 1, Unit: "%", Scale: 1},
				{Name: "Cycles", Addr: baseAddr + 6, Count: 1, Unit: "cycles", Scale: 1},
				// 3 state probes (section 5.1.8)
				{Name: "Charge Limit", Addr: stateBase, Count: 1, Unit: "A", Scale: 0.01},
				{Name: "Discharge Limit", Addr: stateBase + 1, Count: 1, Unit: "A", Scale: 0.01},
				{Name: "State", Addr: stateBase + 2, Count: 1, Enum: BatteryStateEnum},
			},
		})
	}

	// Global Stats group (full-width)
	groups = append(groups, ProbeGroup{
		Name: "Global Stats",
		Probes: []Probe{
			{Name: "Total charge/discharge power", Addr: 0x0667, Count: 1, Signed: true, Unit: "kW", Scale: 0.1},
			{Name: "Average SOC", Addr: 0x0668, Count: 1, Unit: "%", Scale: 1},
			{Name: "Battery SOH", Addr: 0x0669, Count: 1, Unit: "%", Scale: 1},
			{Name: "Pack count", Addr: 0x066A, Count: 1},
			{Name: "Total capacity", Addr: 0x066B, Count: 1, Unit: "Ah", Scale: 1},
		},
	})
	return groups
}

// BMSInfoGroups returns ProbeGroup definitions for BMS global information.
// From Sofar Modbus-G3 V1.38 section 5.10.1.
func BMSInfoGroups() []ProbeGroup {
	return []ProbeGroup{
		{
			Name: "BMS Info",
			Probes: []Probe{
				{Name: "System Clock Hi", Addr: 0x9004, Count: 1},
				{Name: "System Clock Lo", Addr: 0x9005, Count: 1},
				{Name: "CAN Protocol Ver", Addr: 0x9006, Count: 1},
				{Name: "Manufacturer", Addr: 0x9007, Count: 4, IsASCII: true},
				{Name: "BMS Version", Addr: 0x900B, Count: 1},
				{Name: "Cell Type", Addr: 0x900C, Count: 1},
				{Name: "Topology Params", Addr: 0x900D, Count: 1},
				{Name: "Remaining Capacity", Addr: 0x900E, Count: 1, Unit: "%", Scale: 1},
				{Name: "Total Voltage", Addr: 0x900F, Count: 1, Unit: "V", Scale: 0.1},
				{Name: "Total Current", Addr: 0x9010, Count: 1, Signed: true, Unit: "A", Scale: 0.1},
				{Name: "Avg Cell Temp", Addr: 0x9011, Count: 1, Signed: true, Unit: "\u00b0C", Scale: 0.1},
				{Name: "SOC", Addr: 0x9012, Count: 1, Unit: "%", Scale: 1},
				{Name: "Health Level", Addr: 0x9013, Count: 1, Unit: "%", Scale: 1},
				{Name: "SW Version Char", Addr: 0x9018, Count: 1, IsASCII: true},
				{Name: "SW Major", Addr: 0x9019, Count: 1},
				{Name: "SW Non-standard", Addr: 0x901A, Count: 1},
				{Name: "SW Minor", Addr: 0x901B, Count: 1},
				{Name: "SN", Addr: 0x9024, Count: 10, IsASCII: true},
				{Name: "Online Bitmap", Addr: 0x9022, Count: 1},
			},
		},
	}
}

// BMSProtectionProbes returns a flat probe slice for the 6 BMS protection/alarm registers.
// From Sofar Modbus-G3 V1.38 section 5.10.1.
func BMSProtectionProbes() []Probe {
	return []Probe{
		{Name: "Protection 0", Addr: 0x9014, Count: 1},
		{Name: "Protection 1", Addr: 0x9015, Count: 1},
		{Name: "Alarm 0", Addr: 0x9016, Count: 1},
		{Name: "Alarm 1", Addr: 0x9017, Count: 1},
		{Name: "Protection 2", Addr: 0x901C, Count: 1},
		{Name: "Protection 3", Addr: 0x901D, Count: 1},
	}
}

// === Phase 05: Pack-level probe definitions, bitmap tables, encoding ===

// EncodePackQuery maps UI coordinates (1-indexed input, tower, pack) to the 0x9020
// register value for BMS pack selection. Protocol uses 0-based indexing for both
// group and pack: group = (input-1)*towersPerInput + (tower-1), packIdx = pack-1.
// UI shows packs 1-10, protocol sends 0-9. The packID returned at 0x9044 uses the
// BMS's own internal numbering (1-based).
func EncodePackQuery(input, tower, pack, towersPerInput int) uint16 {
	group := (input-1)*towersPerInput + (tower - 1)
	packIdx := pack - 1
	return uint16(packIdx&0xFF) | uint16((group&0x0F)<<8)
}

// PackRTProbes returns probe definitions for pack real-time data registers 0x9044-0x907C.
// From Sofar Modbus-G3 V1.38 section 5.10.2.
func PackRTProbes() []Probe {
	probes := []Probe{
		{Name: "Pack ID", Addr: 0x9044, Count: 1},
		{Name: "Timestamp Hi", Addr: 0x9045, Count: 1},
		{Name: "Timestamp Lo", Addr: 0x9046, Count: 1},
		{Name: "Serial Number", Addr: 0x9047, Count: 10, IsASCII: true},
	}

	// 16 cell voltages: 0x9051-0x9060, millivolt resolution (scale 0.001, per D-05)
	for i := 0; i < 16; i++ {
		probes = append(probes, Probe{
			Name:  fmt.Sprintf("Cell %d", i+1),
			Addr:  uint16(0x9051 + i),
			Count: 1,
			Unit:  "V",
			Scale: 0.001,
		})
	}

	probes = append(probes, []Probe{
		{Name: "Max Cell Voltage", Addr: 0x9069, Count: 1, Unit: "V", Scale: 0.001},
		{Name: "Min Cell Voltage", Addr: 0x906A, Count: 1, Unit: "V", Scale: 0.001},
		{Name: "Temp 1", Addr: 0x906B, Count: 1, Signed: true, Unit: "\u00b0C", Scale: 0.1},
		{Name: "Temp 2", Addr: 0x906C, Count: 1, Signed: true, Unit: "\u00b0C", Scale: 0.1},
		{Name: "Temp 3", Addr: 0x906D, Count: 1, Signed: true, Unit: "\u00b0C", Scale: 0.1},
		{Name: "Temp 4", Addr: 0x906E, Count: 1, Signed: true, Unit: "\u00b0C", Scale: 0.1},
		{Name: "MOS Temp", Addr: 0x906F, Count: 1, Signed: true, Unit: "\u00b0C", Scale: 0.1},
		{Name: "Env Temp", Addr: 0x9070, Count: 1, Signed: true, Unit: "\u00b0C", Scale: 0.1},
		{Name: "Current", Addr: 0x9071, Count: 1, Signed: true, Unit: "A", Scale: 0.1},
		{Name: "Remaining Capacity", Addr: 0x9072, Count: 1, Unit: "Ah", Scale: 0.1},
		{Name: "Full Charge Capacity", Addr: 0x9073, Count: 1, Unit: "Ah", Scale: 0.1},
		{Name: "Cycle Count", Addr: 0x9074, Count: 1, Unit: "cycles", Scale: 1},
		{Name: "Balance State", Addr: 0x9075, Count: 1},
		{Name: "Alarm Status", Addr: 0x9076, Count: 1},
		{Name: "Protection Status", Addr: 0x9077, Count: 1},
		{Name: "Fault Status", Addr: 0x9078, Count: 1},
		{Name: "Total Voltage", Addr: 0x9079, Count: 1, Unit: "V", Scale: 0.1},
		{Name: "SOC", Addr: 0x907A, Count: 1, Unit: "%", Scale: 1},
		{Name: "Total Packs", Addr: 0x907B, Count: 1},
		{Name: "Cell Count", Addr: 0x907C, Count: 1},
	}...)

	return probes
}

// PackInfoProbes returns probe definitions for pack info registers 0x9104-0x9126.
// From Sofar Modbus-G3 V1.38 section 5.10.3.
func PackInfoProbes() []Probe {
	return []Probe{
		{Name: "Balanced Bus Voltage", Addr: 0x9104, Count: 1, Unit: "V", Scale: 0.1},
		{Name: "Balanced Bus Current", Addr: 0x9105, Count: 1, Signed: true, Unit: "A", Scale: 0.1},
		{Name: "Manufacturer", Addr: 0x9106, Count: 4, IsASCII: true},
		{Name: "SOH", Addr: 0x910A, Count: 1, Unit: "%", Scale: 0.1},
		{Name: "Rated Capacity", Addr: 0x910B, Count: 1, Unit: "Ah", Scale: 0.1},
		{Name: "Alarm Status 2", Addr: 0x9124, Count: 1},
		{Name: "Protection Status 2", Addr: 0x9125, Count: 1},
		{Name: "Fault Status 2", Addr: 0x9126, Count: 1},
	}
}

// PackTemps58Probes returns probe definitions for pack temperature sensors 5-8
// at registers 0x90BC-0x90BF. From Sofar Modbus-G3 V1.38 section 5.10.2.
func PackTemps58Probes() []Probe {
	return []Probe{
		{Name: "Temp 5", Addr: 0x90BC, Count: 1, Signed: true, Unit: "\u00b0C", Scale: 0.1},
		{Name: "Temp 6", Addr: 0x90BD, Count: 1, Signed: true, Unit: "\u00b0C", Scale: 0.1},
		{Name: "Temp 7", Addr: 0x90BE, Count: 1, Signed: true, Unit: "\u00b0C", Scale: 0.1},
		{Name: "Temp 8", Addr: 0x90BF, Count: 1, Signed: true, Unit: "\u00b0C", Scale: 0.1},
	}
}

// PackProbeGroups returns pack drill-down probes organized into display groups
// in the correct order per D-03: Info, Cell Voltages, Balance State, Temperatures, Pack Status.
// Each group has a Type hint for frontend rendering dispatch.
func PackProbeGroups() []ProbeGroup {
	return []ProbeGroup{
		// Group 1: Pack Info (RT block info items + Info block items)
		{
			Name: "Pack Info",
			Probes: []Probe{
				{Name: "Pack ID", Addr: 0x9044, Count: 1},
				{Name: "Serial Number", Addr: 0x9047, Count: 10, IsASCII: true},
				{Name: "Total Voltage", Addr: 0x9079, Count: 1, Unit: "V", Scale: 0.1},
				{Name: "SOC", Addr: 0x907A, Count: 1, Unit: "%", Scale: 1},
				{Name: "Current", Addr: 0x9071, Count: 1, Signed: true, Unit: "A", Scale: 0.1},
				{Name: "Remaining Capacity", Addr: 0x9072, Count: 1, Unit: "Ah", Scale: 0.1},
				{Name: "Full Charge Capacity", Addr: 0x9073, Count: 1, Unit: "Ah", Scale: 0.1},
				{Name: "Cycle Count", Addr: 0x9074, Count: 1, Unit: "cycles", Scale: 1},
				{Name: "Total Packs", Addr: 0x907B, Count: 1},
				{Name: "Cell Count", Addr: 0x907C, Count: 1},
				// Info block items (0x9104-0x910B)
				{Name: "Balanced Bus Voltage", Addr: 0x9104, Count: 1, Unit: "V", Scale: 0.1},
				{Name: "Balanced Bus Current", Addr: 0x9105, Count: 1, Signed: true, Unit: "A", Scale: 0.1},
				{Name: "Manufacturer", Addr: 0x9106, Count: 4, IsASCII: true},
				{Name: "SOH", Addr: 0x910A, Count: 1, Unit: "%", Scale: 0.1},
				{Name: "Rated Capacity", Addr: 0x910B, Count: 1, Unit: "Ah", Scale: 0.1},
			},
		},
		// Group 2: Cell Voltages (16 cells + Max/Min for summary computation per D-07)
		{
			Name: "Cell Voltages",
			Type: "cell_grid",
			Probes: func() []Probe {
				probes := make([]Probe, 0, 18)
				for i := 0; i < 16; i++ {
					probes = append(probes, Probe{
						Name:  fmt.Sprintf("Cell %d", i+1),
						Addr:  uint16(0x9051 + i),
						Count: 1,
						Unit:  "V",
						Scale: 0.001,
					})
				}
				probes = append(probes,
					Probe{Name: "Max Cell Voltage", Addr: 0x9069, Count: 1, Unit: "V", Scale: 0.001},
					Probe{Name: "Min Cell Voltage", Addr: 0x906A, Count: 1, Unit: "V", Scale: 0.001},
				)
				return probes
			}(),
		},
		// Group 3: Balance State (single probe at 0x9075)
		{
			Name: "Balance State",
			Type: "balance",
			Probes: []Probe{
				{Name: "Balance State", Addr: 0x9075, Count: 1},
			},
		},
		// Group 4: Temperatures (Temp 1-4 from RT, MOS Temp, Env Temp, Temp 5-8 from temps58 block)
		{
			Name: "Temperatures",
			Probes: []Probe{
				{Name: "Temp 1", Addr: 0x906B, Count: 1, Signed: true, Unit: "\u00b0C", Scale: 0.1},
				{Name: "Temp 2", Addr: 0x906C, Count: 1, Signed: true, Unit: "\u00b0C", Scale: 0.1},
				{Name: "Temp 3", Addr: 0x906D, Count: 1, Signed: true, Unit: "\u00b0C", Scale: 0.1},
				{Name: "Temp 4", Addr: 0x906E, Count: 1, Signed: true, Unit: "\u00b0C", Scale: 0.1},
				{Name: "MOS Temp", Addr: 0x906F, Count: 1, Signed: true, Unit: "\u00b0C", Scale: 0.1},
				{Name: "Env Temp", Addr: 0x9070, Count: 1, Signed: true, Unit: "\u00b0C", Scale: 0.1},
				{Name: "Temp 5", Addr: 0x90BC, Count: 1, Signed: true, Unit: "\u00b0C", Scale: 0.1},
				{Name: "Temp 6", Addr: 0x90BD, Count: 1, Signed: true, Unit: "\u00b0C", Scale: 0.1},
				{Name: "Temp 7", Addr: 0x90BE, Count: 1, Signed: true, Unit: "\u00b0C", Scale: 0.1},
				{Name: "Temp 8", Addr: 0x90BF, Count: 1, Signed: true, Unit: "\u00b0C", Scale: 0.1},
			},
		},
		// Group 5: Pack Status (alarm/protection/fault from RT + Info blocks)
		{
			Name: "Pack Status",
			Type: "pack_status",
			Probes: []Probe{
				{Name: "Alarm Status", Addr: 0x9076, Count: 1},
				{Name: "Protection Status", Addr: 0x9077, Count: 1},
				{Name: "Fault Status", Addr: 0x9078, Count: 1},
				{Name: "Alarm Status 2", Addr: 0x9124, Count: 1},
				{Name: "Protection Status 2", Addr: 0x9125, Count: 1},
				{Name: "Fault Status 2", Addr: 0x9126, Count: 1},
			},
		},
	}
}

// === BMS Pack-level bitmap decode tables ===
// Follow the FaultBit pattern from fault.go.

// BMSAlarmBits maps bit positions in pack RT alarm register 0x9076 to descriptions.
var BMSAlarmBits = []FaultBit{
	{Addr: 0x9076, Bit: 0, Desc: "Cell OV alarm"},
	{Addr: 0x9076, Bit: 1, Desc: "Cell UV alarm"},
	{Addr: 0x9076, Bit: 2, Desc: "Pack OV alarm"},
	{Addr: 0x9076, Bit: 3, Desc: "Pack UV alarm"},
	{Addr: 0x9076, Bit: 4, Desc: "Charge over-temperature alarm"},
	{Addr: 0x9076, Bit: 5, Desc: "Charge under-temperature alarm"},
	{Addr: 0x9076, Bit: 6, Desc: "Discharge over-temperature alarm"},
	{Addr: 0x9076, Bit: 7, Desc: "Discharge under-temperature alarm"},
	{Addr: 0x9076, Bit: 8, Desc: "Charge overcurrent alarm"},
	{Addr: 0x9076, Bit: 9, Desc: "Discharge overcurrent alarm"},
}

// BMSProtectionBits maps bit positions in pack RT protection register 0x9077 to descriptions.
var BMSProtectionBits = []FaultBit{
	{Addr: 0x9077, Bit: 0, Desc: "Cell OV protection"},
	{Addr: 0x9077, Bit: 1, Desc: "Cell UV protection"},
	{Addr: 0x9077, Bit: 2, Desc: "Pack OV protection"},
	{Addr: 0x9077, Bit: 3, Desc: "Pack UV protection"},
	{Addr: 0x9077, Bit: 4, Desc: "Charge over-temperature protection"},
	{Addr: 0x9077, Bit: 5, Desc: "Charge under-temperature protection"},
	{Addr: 0x9077, Bit: 6, Desc: "Discharge over-temperature protection"},
	{Addr: 0x9077, Bit: 7, Desc: "Discharge under-temperature protection"},
	{Addr: 0x9077, Bit: 8, Desc: "Charge overcurrent protection"},
	{Addr: 0x9077, Bit: 9, Desc: "Discharge overcurrent protection"},
	{Addr: 0x9077, Bit: 10, Desc: "Short circuit protection"},
	{Addr: 0x9077, Bit: 11, Desc: "IC fault protection"},
	{Addr: 0x9077, Bit: 12, Desc: "MOS over-temperature protection"},
}

// BMSFaultBits maps bit positions in pack RT fault register 0x9078 to descriptions.
var BMSFaultBits = []FaultBit{
	{Addr: 0x9078, Bit: 0, Desc: "Cell voltage diff too large"},
	{Addr: 0x9078, Bit: 1, Desc: "Temperature diff too large"},
	{Addr: 0x9078, Bit: 2, Desc: "Charging lockout (cell OV)"},
	{Addr: 0x9078, Bit: 3, Desc: "Discharging lockout (cell UV)"},
}

// BMSAlarm2Bits maps bit positions in pack info alarm register 0x9124 to descriptions.
var BMSAlarm2Bits = []FaultBit{
	{Addr: 0x9124, Bit: 0, Desc: "Cell OV alarm 2"},
	{Addr: 0x9124, Bit: 1, Desc: "Cell UV alarm 2"},
	{Addr: 0x9124, Bit: 2, Desc: "Pack OV alarm 2"},
	{Addr: 0x9124, Bit: 3, Desc: "Pack UV alarm 2"},
}

// BMSProtection2Bits maps bit positions in pack info protection register 0x9125 to descriptions.
var BMSProtection2Bits = []FaultBit{
	{Addr: 0x9125, Bit: 0, Desc: "Cell OV protection 2"},
	{Addr: 0x9125, Bit: 1, Desc: "Cell UV protection 2"},
	{Addr: 0x9125, Bit: 2, Desc: "Pack OV protection 2"},
	{Addr: 0x9125, Bit: 3, Desc: "Pack UV protection 2"},
}

// BMSFault2Bits maps bit positions in pack info fault register 0x9126 to descriptions.
var BMSFault2Bits = []FaultBit{
	{Addr: 0x9126, Bit: 0, Desc: "Cell voltage diff too large 2"},
	{Addr: 0x9126, Bit: 1, Desc: "Temperature diff too large 2"},
	{Addr: 0x9126, Bit: 2, Desc: "Charging lockout (cell OV) 2"},
	{Addr: 0x9126, Bit: 3, Desc: "Discharging lockout (cell UV) 2"},
}

// DecodeBMSBitmap decodes a 16-bit register value against a FaultBit table,
// returning descriptions for all set bits matching the given address.
func DecodeBMSBitmap(value uint16, table []FaultBit, addr uint16) []string {
	var decoded []string
	for _, fb := range table {
		if fb.Addr == addr && value&(1<<fb.Bit) != 0 {
			decoded = append(decoded, fb.Desc)
		}
	}
	return decoded
}
