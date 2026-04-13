package register

// SystemGroups returns system information register definitions organized into ProbeGroups,
// including daily and total electricity statistics appended after Protection.
// Covers sections 5.1.1, 5.1.2, and 5.1.9 of the Sofar Modbus-G3 V1.38 protocol.
func SystemGroups() []ProbeGroup {
	base := []ProbeGroup{
	{Name: "Identity", Probes: []Probe{
		{Name: "Inverter SN", Addr: 0x0445, Count: 10, IsASCII: true},
	}},
	{Name: "Firmware", Probes: []Probe{
		{Name: "HW version", Addr: 0x044D, Count: 2, IsASCII: true},
		{Name: "Comm SW version", Addr: 0x044F, Count: 4, IsASCII: true},
		{Name: "Master DSP version", Addr: 0x0453, Count: 4, IsASCII: true},
		{Name: "Slave DSP version", Addr: 0x0457, Count: 4, IsASCII: true},
		{Name: "Safety cert version", Addr: 0x045B, Count: 2, IsASCII: true},
	}},
	{Name: "Status", Probes: []Probe{
		{Name: "Running state", Addr: 0x0404, Count: 1, Enum: RunningStateEnum},
		{Name: "System time (Year)", Addr: 0x042C, Count: 1},
		{Name: "System time (Month)", Addr: 0x042D, Count: 1},
		{Name: "System time (Day)", Addr: 0x042E, Count: 1},
		{Name: "System time (Hour)", Addr: 0x042F, Count: 1},
		{Name: "System time (Min)", Addr: 0x0430, Count: 1},
		{Name: "System time (Sec)", Addr: 0x0431, Count: 1},
	}},
	{Name: "Temperatures", Probes: []Probe{
		{Name: "Ambient temp 1", Addr: 0x0418, Count: 1, Signed: true, Unit: "\u00b0C", Scale: 1},
		{Name: "Ambient temp 2", Addr: 0x0419, Count: 1, Signed: true, Unit: "\u00b0C", Scale: 1},
		{Name: "Radiator temp", Addr: 0x041A, Count: 1, Signed: true, Unit: "\u00b0C", Scale: 1},
		{Name: "Module temp", Addr: 0x0420, Count: 1, Signed: true, Unit: "\u00b0C", Scale: 1},
	}},
	{Name: "Protection", Probes: []Probe{
		{Name: "Insulation impedance", Addr: 0x042B, Count: 1, Unit: "k\u03a9", Scale: 1},
		{Name: "Fan speed", Addr: 0x043E, Count: 1, Unit: "r/min", Scale: 1},
	}},
	}
	return append(base, StatisticsGroups()...)
}

// GridGroups contains grid-connected output register definitions organized into ProbeGroups.
// Covers section 5.1.3 of the Sofar Modbus-G3 V1.38 protocol.
var GridGroups = []ProbeGroup{
	{Name: "General", Probes: []Probe{
		{Name: "Grid frequency", Addr: 0x0484, Count: 1, Unit: "Hz", Scale: 0.01},
		{Name: "Total active power", Addr: 0x0485, Count: 1, Signed: true, Unit: "kW", Scale: 0.01},
		{Name: "Total reactive power", Addr: 0x0486, Count: 1, Signed: true, Unit: "kVar", Scale: 0.01},
		{Name: "Total apparent power", Addr: 0x0487, Count: 1, Signed: true, Unit: "kVA", Scale: 0.01},
	}},
	{Name: "Phase R", Layout: "column", Probes: []Probe{
		{Name: "Voltage", Addr: 0x048D, Count: 1, Unit: "V", Scale: 0.1},
		{Name: "Current", Addr: 0x048E, Count: 1, Unit: "A", Scale: 0.01},
		{Name: "Active power", Addr: 0x048F, Count: 1, Signed: true, Unit: "kW", Scale: 0.01},
		{Name: "Reactive power", Addr: 0x0490, Count: 1, Signed: true, Unit: "kVar", Scale: 0.01},
		{Name: "Power factor", Addr: 0x0491, Count: 1, Signed: true, Scale: 0.001},
	}},
	{Name: "Phase S", Layout: "column", Probes: []Probe{
		{Name: "Voltage", Addr: 0x0498, Count: 1, Unit: "V", Scale: 0.1},
		{Name: "Current", Addr: 0x0499, Count: 1, Unit: "A", Scale: 0.01},
		{Name: "Active power", Addr: 0x049A, Count: 1, Signed: true, Unit: "kW", Scale: 0.01},
		{Name: "Reactive power", Addr: 0x049B, Count: 1, Signed: true, Unit: "kVar", Scale: 0.01},
		{Name: "Power factor", Addr: 0x049C, Count: 1, Signed: true, Scale: 0.001},
	}},
	{Name: "Phase T", Layout: "column", Probes: []Probe{
		{Name: "Voltage", Addr: 0x04A3, Count: 1, Unit: "V", Scale: 0.1},
		{Name: "Current", Addr: 0x04A4, Count: 1, Unit: "A", Scale: 0.01},
		{Name: "Active power", Addr: 0x04A5, Count: 1, Signed: true, Unit: "kW", Scale: 0.01},
		{Name: "Reactive power", Addr: 0x04A6, Count: 1, Signed: true, Unit: "kVar", Scale: 0.01},
		{Name: "Power factor", Addr: 0x04A7, Count: 1, Signed: true, Scale: 0.001},
	}},
	{Name: "PCC Power", Probes: []Probe{
		{Name: "PCC active power", Addr: 0x0488, Count: 1, Signed: true, Unit: "kW", Scale: 0.01},
		{Name: "PCC reactive power", Addr: 0x0489, Count: 1, Signed: true, Unit: "kVar", Scale: 0.01},
	}},
	{Name: "Line Voltages", Probes: []Probe{
		{Name: "L1 (R/S)", Addr: 0x04BA, Count: 1, Unit: "V", Scale: 0.1},
		{Name: "L2 (S/T)", Addr: 0x04BB, Count: 1, Unit: "V", Scale: 0.1},
		{Name: "L3 (T/R)", Addr: 0x04BC, Count: 1, Unit: "V", Scale: 0.1},
	}},
	{Name: "Load", Probes: []Probe{
		{Name: "Total load power", Addr: 0x04AF, Count: 1, Unit: "kW", Scale: 0.01},
		{Name: "Total power factor", Addr: 0x04BD, Count: 1, Signed: true, Scale: 0.001},
		{Name: "Generation efficiency", Addr: 0x04BF, Count: 1, Unit: "%", Scale: 0.01},
	}},
}

// EPSGroups contains grid-disconnected (EPS) output register definitions organized into ProbeGroups.
// Covers section 5.1.4 of the Sofar Modbus-G3 V1.38 protocol.
var EPSGroups = []ProbeGroup{
	{Name: "General", Probes: []Probe{
		{Name: "Load active power", Addr: 0x0504, Count: 1, Signed: true, Unit: "kW", Scale: 0.01},
		{Name: "Load reactive power", Addr: 0x0505, Count: 1, Signed: true, Unit: "kVar", Scale: 0.01},
		{Name: "Load apparent power", Addr: 0x0506, Count: 1, Signed: true, Unit: "kVA", Scale: 0.01},
		{Name: "Output voltage frequency", Addr: 0x0507, Count: 1, Unit: "Hz", Scale: 0.01},
	}},
	{Name: "Phase R", Layout: "column", Probes: []Probe{
		{Name: "Inverter output voltage", Addr: 0x050A, Count: 1, Unit: "V", Scale: 0.1},
		{Name: "Load current", Addr: 0x050B, Count: 1, Signed: true, Unit: "A", Scale: 0.01},
	}},
	{Name: "Phase S", Layout: "column", Probes: []Probe{
		{Name: "Inverter output voltage", Addr: 0x0512, Count: 1, Unit: "V", Scale: 0.1},
		{Name: "Load current", Addr: 0x0513, Count: 1, Signed: true, Unit: "A", Scale: 0.01},
	}},
	{Name: "Phase T", Layout: "column", Probes: []Probe{
		{Name: "Inverter output voltage", Addr: 0x051A, Count: 1, Unit: "V", Scale: 0.1},
		{Name: "Load current", Addr: 0x051B, Count: 1, Signed: true, Unit: "A", Scale: 0.01},
	}},
	{Name: "Emergency Load", Probes: []Probe{
		{Name: "Emergency load voltage R", Addr: 0x0510, Count: 1, Unit: "V", Scale: 0.1},
		{Name: "Emergency load voltage S", Addr: 0x0518, Count: 1, Unit: "V", Scale: 0.1},
		{Name: "Emergency load voltage T", Addr: 0x0520, Count: 1, Unit: "V", Scale: 0.1},
	}},
}
