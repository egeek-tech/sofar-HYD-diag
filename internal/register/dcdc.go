package register

// DCDCGroups contains DCDC converter register definitions organized into ProbeGroups.
// From Sofar Modbus-G3 V1.38 section 5.7 -- DCDC data area 0x5000-0x530F.
// Only read-only (R) registers included; RW/W control registers are excluded
// (read-only diagnostic tool).
var DCDCGroups = []ProbeGroup{
	{Name: "System Info", Probes: []Probe{
		{Name: "DCDC SN", Addr: 0x5029, Count: 10, IsASCII: true},
	}},
	{Name: "Real-Time Data", Probes: []Probe{
		{Name: "Parallel DCDC count", Addr: 0x5044, Count: 1},
		{Name: "System running state", Addr: 0x5046, Count: 1, Enum: DCDCRunningStateEnum},
		{Name: "Power limiting state", Addr: 0x5047, Count: 1},
		{Name: "SOC balanced power", Addr: 0x5048, Count: 1, Signed: true, Unit: "%", Scale: 0.1},
		{Name: "LV side bus voltage", Addr: 0x504E, Count: 1, Signed: true, Unit: "V", Scale: 0.1},
		{Name: "LV side current", Addr: 0x5050, Count: 1, Signed: true, Unit: "A", Scale: 0.1},
		{Name: "LV side power", Addr: 0x5051, Count: 1, Signed: true, Unit: "kW", Scale: 0.01},
		{Name: "HV bus voltage", Addr: 0x505C, Count: 1, Signed: true, Unit: "V", Scale: 0.1},
		{Name: "Insulation impedance", Addr: 0x505E, Count: 1, Unit: "k\u03a9", Scale: 1},
		{Name: "Internal env temp", Addr: 0x5060, Count: 1, Signed: true, Unit: "\u00b0C", Scale: 1},
		{Name: "Radiator temp 1", Addr: 0x5061, Count: 1, Signed: true, Unit: "\u00b0C", Scale: 1},
		{Name: "Radiator temp 2", Addr: 0x5062, Count: 1, Signed: true, Unit: "\u00b0C", Scale: 1},
	}},
	{Name: "Faults", Probes: []Probe{
		{Name: "Fault 1", Addr: 0x5065, Count: 1},
		{Name: "Fault 2", Addr: 0x5066, Count: 1},
		{Name: "Fault 3", Addr: 0x5067, Count: 1},
		{Name: "Fault 4", Addr: 0x5068, Count: 1},
		{Name: "Fault 5", Addr: 0x5069, Count: 1},
		{Name: "Fault 6", Addr: 0x506A, Count: 1},
		{Name: "Fault 7", Addr: 0x506B, Count: 1},
		{Name: "Fault 8", Addr: 0x506C, Count: 1},
		{Name: "Fault 9", Addr: 0x506D, Count: 1},
		{Name: "Fault 10", Addr: 0x506E, Count: 1},
	}},
	{Name: "Capacity", Probes: []Probe{
		{Name: "Total charge capacity", Addr: 0x5074, Count: 2, U32: true, Unit: "Wh", Scale: 1},
		{Name: "Total discharge capacity", Addr: 0x5076, Count: 2, U32: true, Unit: "Wh", Scale: 1},
	}},
}
