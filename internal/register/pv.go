package register

import "fmt"

// GeneratePVGroups dynamically generates ProbeGroup definitions for N PV channels.
// Each channel has 3 registers (voltage, current, power) at base address 0x0584 + 3*(ch-1).
// An additional full-width "Total PV Power" group is appended at the end.
// From Sofar Modbus-G3 V1.38 sections 5.1.5-5.1.6.
func GeneratePVGroups(channels int) []ProbeGroup {
	groups := make([]ProbeGroup, 0, channels+1)
	for ch := 1; ch <= channels; ch++ {
		baseAddr := uint16(0x0584 + 3*(ch-1))
		groups = append(groups, ProbeGroup{
			Name:   fmt.Sprintf("PV %d", ch),
			Layout: "column",
			Probes: []Probe{
				{Name: "Voltage", Addr: baseAddr, Count: 1, Unit: "V", Scale: 0.1},
				{Name: "Current", Addr: baseAddr + 1, Count: 1, Signed: true, Unit: "A", Scale: 0.01},
				{Name: "Power", Addr: baseAddr + 2, Count: 1, Unit: "kW", Scale: 0.01},
			},
		})
	}
	groups = append(groups, ProbeGroup{
		Name: "Total PV Power",
		Probes: []Probe{
			{Name: "Total PV power", Addr: 0x05C4, Count: 1, Unit: "kW", Scale: 0.1},
		},
	})
	return groups
}
