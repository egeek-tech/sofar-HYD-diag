package register

// RunningStateEnum maps system running state register values (0x0404) to human-readable labels.
// From Sofar Modbus-G3 V1.38 section 5.1.1.
var RunningStateEnum = map[uint16]string{
	0:  "Waiting",
	1:  "Detecting",
	2:  "Grid-connected",
	3:  "EPS mode",
	4:  "Recoverable fault",
	5:  "Permanent fault",
	6:  "Upgrading",
	7:  "Self-charging",
	8:  "SVG status",
	9:  "PID status",
	10: "Limit the load",
	11: "Standby monitoring",
}

// BatteryStateEnum maps battery state register values to human-readable labels.
// From Sofar Modbus-G3 V1.38 section 5.1.8.
var BatteryStateEnum = map[uint16]string{
	1: "Charging",
	2: "Discharging",
	3: "Sleeping",
	4: "Fault",
	5: "Loss reduction",
}
