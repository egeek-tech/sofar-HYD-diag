package register

// BDUGroups contains BDU (Battery Distribution Unit) register definitions organized into ProbeGroups.
// From Sofar Modbus-G3 V1.38 section 5.9 -- BDU data area 0x6084-0x60C7.
// Only read-only (R) registers included; RW/W control registers are excluded
// (read-only diagnostic tool).
var BDUGroups = []ProbeGroup{
	{Name: "Info", Probes: []Probe{
		{Name: "Total BDU online", Addr: 0x6084, Count: 1},
		{Name: "BDU Code", Addr: 0x6090, Count: 1},
		{Name: "BDU Serial number", Addr: 0x6091, Count: 10, IsASCII: true},
		{Name: "BDU Version", Addr: 0x609B, Count: 4, IsASCII: true},
		{Name: "BDU Fault info", Addr: 0x609F, Count: 1},
	}},
	{Name: "Topology", Probes: []Probe{
		{Name: "Cells in single cluster", Addr: 0x60A0, Count: 1},
		{Name: "Total strings", Addr: 0x60A1, Count: 1},
		{Name: "Total battery circuits", Addr: 0x60A2, Count: 1},
	}},
}
