package register

// SystemProbes contains system info register definitions (section 5.1.1).
// Extracted from main.go.bak scanRegisters() lines 298-309.
var SystemProbes = []Probe{
	{"Inverter SN", 0x0445, 10, true, false, "", 0},
	{"System running state", 0x0404, 1, false, false, "", 0},
	{"System time (Year+2000)", 0x042C, 1, false, false, "", 0},
	{"System time (Month)", 0x042D, 1, false, false, "", 0},
	{"System time (Day)", 0x042E, 1, false, false, "", 0},
	{"System time (Hour)", 0x042F, 1, false, false, "", 0},
	{"System time (Min)", 0x0430, 1, false, false, "", 0},
	{"System time (Sec)", 0x0431, 1, false, false, "", 0},
	{"Ambient temp 1", 0x0418, 1, false, true, "\u00b0C", 1},
	{"Ambient temp 2", 0x0419, 1, false, true, "\u00b0C", 1},
	{"Insulation impedance", 0x042B, 1, false, false, "k\u03a9", 1},
}

// GridProbes contains grid-connected output register definitions (section 5.1.3).
// Extracted from main.go.bak scanRegisters() lines 312-328.
var GridProbes = []Probe{
	{"Grid frequency", 0x0484, 1, false, false, "Hz", 0.01},
	{"Total active power", 0x0485, 1, false, true, "kW", 0.01},
	{"Total reactive power", 0x0486, 1, false, true, "kVar", 0.01},
	{"Total apparent power", 0x0487, 1, false, true, "kVA", 0.01},
	{"Total PCC active power", 0x0488, 1, false, true, "kW", 0.01},
	{"Grid voltage R", 0x048D, 1, false, false, "V", 0.1},
	{"Output current R", 0x048E, 1, false, false, "A", 0.01},
	{"Output active power R", 0x048F, 1, false, true, "kW", 0.01},
	{"Grid voltage S", 0x0498, 1, false, false, "V", 0.1},
	{"Output current S", 0x0499, 1, false, false, "A", 0.01},
	{"Grid voltage T", 0x04A3, 1, false, false, "V", 0.1},
	{"Output current T", 0x04A4, 1, false, false, "A", 0.01},
	{"Line voltage L1 (R/S)", 0x04BA, 1, false, false, "V", 0.1},
	{"Line voltage L2 (S/T)", 0x04BB, 1, false, false, "V", 0.1},
	{"Line voltage L3 (T/R)", 0x04BC, 1, false, false, "V", 0.1},
	{"Total load power", 0x04AF, 1, false, false, "kW", 0.01},
	{"Power gen efficiency", 0x04BF, 1, false, false, "%", 0.01},
}

// EPSProbes contains grid-disconnected output register definitions (section 5.1.4).
// Extracted from main.go.bak scanRegisters() lines 331-332.
var EPSProbes = []Probe{
	{"Load active power", 0x0504, 1, false, true, "kW", 0.01},
	{"Output voltage freq", 0x0507, 1, false, false, "Hz", 0.01},
}

// PVProbes contains PV input register definitions (section 5.1.5-5.1.6).
// Extracted from main.go.bak scanRegisters() lines 335-343.
var PVProbes = []Probe{
	{"PV1 Voltage", 0x0584, 1, false, false, "V", 0.1},
	{"PV1 Current", 0x0585, 1, false, true, "A", 0.01},
	{"PV1 Power", 0x0586, 1, false, false, "kW", 0.01},
	{"PV2 Voltage", 0x0587, 1, false, false, "V", 0.1},
	{"PV2 Current", 0x0588, 1, false, true, "A", 0.01},
	{"PV2 Power", 0x0589, 1, false, false, "kW", 0.01},
	{"Total PV power", 0x05C4, 1, false, false, "kW", 0.1},
}
