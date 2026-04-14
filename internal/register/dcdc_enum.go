package register

// DCDCRunningStateEnum maps DCDC system running state (0x5046).
var DCDCRunningStateEnum = map[uint16]string{
	0: "Standby",
	1: "Discharge",
	2: "Charge",
}

// PCUModuleStateEnum maps PCU module state (0x6020).
var PCUModuleStateEnum = map[uint16]string{
	0: "Update",
	1: "Check",
	2: "Normal",
	3: "Active",
	4: "Fault",
	5: "Permanent fault",
}
