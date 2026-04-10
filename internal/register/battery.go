package register

// BatteryProbes contains battery pack channel 1 register definitions (section 5.1.7).
// Extracted from main.go.bak scanRegisters() lines 346-352.
var BatteryProbes = []Probe{
	{"Bat voltage ch1", 0x0604, 1, false, false, "V", 0.1},
	{"Bat current ch1", 0x0605, 1, false, true, "A", 0.01},
	{"Bat power ch1", 0x0606, 1, false, true, "kW", 0.01},
	{"Bat env temp ch1", 0x0607, 1, false, true, "\u00b0C", 1},
	{"Bat SOC ch1", 0x0608, 1, false, false, "%", 1},
	{"Bat SOH ch1", 0x0609, 1, false, false, "%", 1},
	{"Bat cycles ch1", 0x060A, 1, false, false, "cycles", 1},
}

// BatteryStateProbes contains battery state register definitions (section 5.1.8).
// Extracted from main.go.bak scanRegisters() lines 355-362.
var BatteryStateProbes = []Probe{
	{"Bat charge limit ch1", 0x0644, 1, false, false, "A", 0.01},
	{"Bat discharge limit ch1", 0x0645, 1, false, false, "A", 0.01},
	{"Bat state ch1", 0x0646, 1, false, false, "", 0},
	{"Bat ch/disch power", 0x0667, 1, false, true, "kW", 0.1},
	{"Avg electricity level", 0x0668, 1, false, false, "%", 1},
	{"Battery pack SOH", 0x0669, 1, false, false, "%", 1},
	{"RT battery pack count", 0x066A, 1, false, false, "", 0},
	{"Total battery capacity", 0x066B, 1, false, false, "Ah", 1},
}

// BMSProbes contains BMS info register definitions (section 5.10.1).
// Extracted from main.go.bak scanRegisters() lines 369-380.
var BMSProbes = []Probe{
	{"BMS CAN protocol ver", 0x9006, 1, false, false, "", 0},
	{"BMS Manufacturer", 0x9007, 4, true, false, "", 0},
	{"BMS Version number", 0x900B, 1, false, false, "", 0},
	{"BMS Cell type", 0x900C, 1, false, false, "", 0},
	{"BMS Remaining capacity", 0x900E, 1, false, false, "%", 1},
	{"BMS Total voltage", 0x900F, 1, false, false, "V", 0.1},
	{"BMS Total current", 0x9010, 1, false, true, "A", 0.1},
	{"BMS Avg cell temp", 0x9011, 1, false, true, "\u00b0C", 0.1},
	{"BMS SOC", 0x9012, 1, false, false, "%", 1},
	{"BMS Health level", 0x9013, 1, false, false, "%", 1},
	{"BMS Online bitmap", 0x9022, 1, false, false, "", 0},
	{"BMS Battery pack params", 0x900D, 1, false, false, "", 0},
}

// BDUProbes contains BDU area register definitions (section 5.9).
// Extracted from main.go.bak scanRegisters() lines 365-366.
var BDUProbes = []Probe{
	{"BDU total online", 0x6084, 1, false, false, "", 0},
	{"BDU Code", 0x6090, 1, false, false, "", 0},
}
