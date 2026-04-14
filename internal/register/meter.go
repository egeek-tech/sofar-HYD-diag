package register

// MeterGroups contains meter information register definitions.
// From Sofar Modbus-G3 V1.38 -- meter registers at 0x7080+.
var MeterGroups = []ProbeGroup{
	{Name: "Meter Info", Probes: []Probe{
		{Name: "Auto-ID status", Addr: 0x7080, Count: 1},
		{Name: "Connected meters", Addr: 0x7081, Count: 1},
	}},
}
