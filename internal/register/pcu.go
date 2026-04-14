package register

// PCUGroups contains PCU (Power Conversion Unit) register definitions organized into ProbeGroups.
// From Sofar Modbus-G3 V1.38 section 5.8 -- PCU data area 0x6004-0x604E.
// Only read-only (R) registers included; RW/W control registers are excluded
// (read-only diagnostic tool).
var PCUGroups = []ProbeGroup{
	{Name: "Info", Probes: []Probe{
		{Name: "Total PCU online", Addr: 0x6004, Count: 1},
		{Name: "PCU Code", Addr: 0x6010, Count: 1},
		{Name: "Module state", Addr: 0x6020, Count: 1, Enum: PCUModuleStateEnum},
	}},
	{Name: "Temperatures", Layout: "column", Probes: []Probe{
		{Name: "Internal temp", Addr: 0x6021, Count: 1, Unit: "\u00b0C", Scale: 1},
		{Name: "Radiator temp 1", Addr: 0x6022, Count: 1, Signed: true, Unit: "\u00b0C", Scale: 1},
		{Name: "Radiator temp 2", Addr: 0x6023, Count: 1, Signed: true, Unit: "\u00b0C", Scale: 1},
	}},
	{Name: "Alarms", Probes: []Probe{
		{Name: "Alarm 1", Addr: 0x6024, Count: 1},
		{Name: "Alarm 2", Addr: 0x6025, Count: 1},
		{Name: "Alarm 3", Addr: 0x6026, Count: 1},
		{Name: "Alarm 4", Addr: 0x6027, Count: 1},
	}},
	{Name: "Faults", Probes: []Probe{
		{Name: "Fault 1", Addr: 0x6028, Count: 1},
		{Name: "Fault 2", Addr: 0x6029, Count: 1},
		{Name: "Fault 3", Addr: 0x602A, Count: 1},
		{Name: "Fault 4", Addr: 0x602B, Count: 1},
	}},
	{Name: "DC Measurements", Probes: []Probe{
		{Name: "DC HV side voltage", Addr: 0x602C, Count: 1, Unit: "V", Scale: 0.1},
		{Name: "DC LV side voltage", Addr: 0x602D, Count: 1, Unit: "V", Scale: 0.1},
		{Name: "DC LV side current", Addr: 0x602E, Count: 1, Signed: true, Unit: "A", Scale: 0.1},
		{Name: "DC LV side power", Addr: 0x602F, Count: 1, Signed: true, Unit: "kW", Scale: 0.01},
	}},
}
