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
