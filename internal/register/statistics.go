package register

// StatisticsGroups returns 4 ProbeGroup definitions for electricity statistics.
// Each group has 6 U32 metrics: generation, consumption, bought, sold, battery charge, battery discharge.
// The register layout interleaves daily/total pairs and monthly/yearly pairs at stride 4.
// From Sofar Modbus-G3 V1.38 section 5.1.9.
func StatisticsGroups() []ProbeGroup {
	metricNames := []string{
		"Power Generation",
		"Load Consumption",
		"Grid Bought",
		"Grid Sold",
		"Battery Charge",
		"Battery Discharge",
	}

	type groupDef struct {
		name   string
		base   uint16
		scale  float64
		stride uint16 // offset between consecutive metrics
	}

	defs := []groupDef{
		{name: "Today", base: 0x0684, scale: 0.01, stride: 4},
		{name: "Total", base: 0x0686, scale: 0.1, stride: 4},
		{name: "This Month", base: 0x069C, scale: 0.1, stride: 4},
		{name: "This Year", base: 0x069E, scale: 0.1, stride: 4},
	}

	groups := make([]ProbeGroup, 0, len(defs))
	for _, d := range defs {
		probes := make([]Probe, 0, len(metricNames))
		for i, name := range metricNames {
			probes = append(probes, Probe{
				Name:  name,
				Addr:  d.base + d.stride*uint16(i),
				Count: 2,
				U32:   true,
				Unit:  "kWh",
				Scale: d.scale,
			})
		}
		groups = append(groups, ProbeGroup{
			Name:   d.name,
			Layout: "column",
			Probes: probes,
		})
	}
	return groups
}
