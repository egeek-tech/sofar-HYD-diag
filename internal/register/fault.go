package register

// FaultBit maps a register address and bit position to a fault description.
// From Sofar Modbus-G3 V1.38 appendix 6.1 (pages 99-109).
type FaultBit struct {
	Addr uint16 // Fault register address (e.g., 0x0405)
	Bit  uint8  // Bit position 0-15 within the register
	Desc string // Human-readable fault description
}

// FaultTable contains all non-reserved fault definitions from the V1.38 appendix 6.1.
// Each fault register is a 16-bit bitmap. byte1 = bits 0-7 (low byte), byte2 = bits 8-15 (high byte).
// Registers 0x0414-0x0416 (Fault 16-18), 0x0432 (Fault 19), 0x0435-0x0436 (Fault 22-23) are all N/A.
var FaultTable = []FaultBit{
	// Fault 1 (0x0405) - Fault IDs 1-16
	{Addr: 0x0405, Bit: 0, Desc: "Grid over-voltage"},
	{Addr: 0x0405, Bit: 1, Desc: "Grid under-voltage"},
	{Addr: 0x0405, Bit: 2, Desc: "Grid over-frequency"},
	{Addr: 0x0405, Bit: 3, Desc: "Grid under-frequency"},
	{Addr: 0x0405, Bit: 4, Desc: "Leakage current faults"},
	{Addr: 0x0405, Bit: 5, Desc: "High penetration error"},
	{Addr: 0x0405, Bit: 6, Desc: "Low penetration error"},
	{Addr: 0x0405, Bit: 7, Desc: "Island error"},
	{Addr: 0x0405, Bit: 8, Desc: "Grid instantaneous value over-voltage 1"},
	{Addr: 0x0405, Bit: 9, Desc: "Grid instantaneous value over-voltage2"},
	{Addr: 0x0405, Bit: 10, Desc: "Grid line voltage error"},
	{Addr: 0x0405, Bit: 11, Desc: "Inverter voltage error"},
	{Addr: 0x0405, Bit: 12, Desc: "Anti-backflow overload"},
	{Addr: 0x0405, Bit: 13, Desc: "Grid voltage unbalance"},
	{Addr: 0x0405, Bit: 14, Desc: "Inverter transient over-voltage"},
	{Addr: 0x0405, Bit: 15, Desc: "Sudden grid phase change"},

	// Fault 2 (0x0406) - Fault IDs 17-32
	{Addr: 0x0406, Bit: 0, Desc: "Grid current sampling error"},
	{Addr: 0x0406, Bit: 1, Desc: "DCI sampling error(AC)"},
	{Addr: 0x0406, Bit: 2, Desc: "Network voltage sampling error(DC)"},
	{Addr: 0x0406, Bit: 3, Desc: "Network voltage sampling error(AC)"},
	{Addr: 0x0406, Bit: 4, Desc: "GFCI sampling error(DC)"},
	{Addr: 0x0406, Bit: 5, Desc: "GFCI sampling error(AC)"},
	{Addr: 0x0406, Bit: 6, Desc: "DCV sampling error"},
	{Addr: 0x0406, Bit: 7, Desc: "Input current sampling error"},
	{Addr: 0x0406, Bit: 8, Desc: "DCI sampling error(DC)"},
	{Addr: 0x0406, Bit: 9, Desc: "Branch current sampling error"},
	{Addr: 0x0406, Bit: 10, Desc: "PV low impedance to ground"},
	{Addr: 0x0406, Bit: 11, Desc: "PID abnormal output"},
	{Addr: 0x0406, Bit: 12, Desc: "Leakage current consistency error"},
	{Addr: 0x0406, Bit: 13, Desc: "Network voltage consistency error"},
	{Addr: 0x0406, Bit: 14, Desc: "DCI consistency error"},
	{Addr: 0x0406, Bit: 15, Desc: "Neutral ground fault"},

	// Fault 3 (0x0407) - Fault IDs 33-48
	{Addr: 0x0407, Bit: 0, Desc: "SPI communication error(DC)"},
	{Addr: 0x0407, Bit: 1, Desc: "SPI communication error(AC)"},
	{Addr: 0x0407, Bit: 2, Desc: "Chip error(DC)"},
	{Addr: 0x0407, Bit: 3, Desc: "Chip error(AC)"},
	{Addr: 0x0407, Bit: 4, Desc: "Auxiliary power error"},
	{Addr: 0x0407, Bit: 5, Desc: "Inverter soft start failure"},
	{Addr: 0x0407, Bit: 6, Desc: "Arc shutdown protection"},
	{Addr: 0x0407, Bit: 7, Desc: "Weak light detection failure"},
	{Addr: 0x0407, Bit: 8, Desc: "Relay detection failure"},
	{Addr: 0x0407, Bit: 9, Desc: "Low insulation impedance"},
	{Addr: 0x0407, Bit: 10, Desc: "Grounding error"},
	{Addr: 0x0407, Bit: 11, Desc: "Wrong input mode setting"},
	{Addr: 0x0407, Bit: 12, Desc: "CT error"},
	{Addr: 0x0407, Bit: 13, Desc: "Input reversal error"},
	{Addr: 0x0407, Bit: 14, Desc: "Parallel error"},
	{Addr: 0x0407, Bit: 15, Desc: "Serial number error"},

	// Fault 4 (0x0408) - Fault IDs 49-64
	{Addr: 0x0408, Bit: 0, Desc: "Battery temperature protection"},
	{Addr: 0x0408, Bit: 1, Desc: "Radiator 1 temperature protection"},
	{Addr: 0x0408, Bit: 2, Desc: "Radiator 2 temperature protection"},
	{Addr: 0x0408, Bit: 3, Desc: "Radiator 3 temperature protection"},
	{Addr: 0x0408, Bit: 4, Desc: "Radiator 4 temperature protection"},
	{Addr: 0x0408, Bit: 5, Desc: "Radiator 5 temperature protection"},
	{Addr: 0x0408, Bit: 6, Desc: "Radiator 6 temperature protection"},
	{Addr: 0x0408, Bit: 7, Desc: "NTC error"},
	{Addr: 0x0408, Bit: 8, Desc: "Ambient temperature 1 protection"},
	{Addr: 0x0408, Bit: 9, Desc: "Ambient temperature 2 protection"},
	{Addr: 0x0408, Bit: 10, Desc: "Module 1 temperature protection"},
	{Addr: 0x0408, Bit: 11, Desc: "Module 2 temperature protection"},
	{Addr: 0x0408, Bit: 12, Desc: "Module 3 temperature protection"},
	{Addr: 0x0408, Bit: 13, Desc: "Module temperature difference too large"},
	// Bits 14-15 are N/A

	// Fault 5 (0x0409) - Fault IDs 65-80
	{Addr: 0x0409, Bit: 0, Desc: "Bus-bar valid value unbalanced"},
	{Addr: 0x0409, Bit: 1, Desc: "Bus-bar instantaneous value unbalance"},
	{Addr: 0x0409, Bit: 2, Desc: "Bus-bar under-voltage during grid connection"},
	{Addr: 0x0409, Bit: 3, Desc: "Bus-bar low voltage"},
	{Addr: 0x0409, Bit: 4, Desc: "PV over-voltage"},
	{Addr: 0x0409, Bit: 5, Desc: "Battery over-voltage"},
	{Addr: 0x0409, Bit: 6, Desc: "LLC-Bus over-voltage protection"},
	{Addr: 0x0409, Bit: 7, Desc: "Bus-bar Rms software over-voltage"},
	{Addr: 0x0409, Bit: 8, Desc: "Bus-bar transient software over-voltage"},
	{Addr: 0x0409, Bit: 9, Desc: "Fly-span capacitor over-voltage protection"},
	{Addr: 0x0409, Bit: 10, Desc: "Fly-span capacitor under-voltage protection"},
	{Addr: 0x0409, Bit: 11, Desc: "PV under-voltage"},
	// Bits 12-15 are N/A

	// Fault 6 (0x040A) - Fault IDs 81-96
	{Addr: 0x040A, Bit: 0, Desc: "Battery over-current software protection"},
	{Addr: 0x040A, Bit: 1, Desc: "DCI over-current protection"},
	{Addr: 0x040A, Bit: 2, Desc: "Output transient current protection"},
	{Addr: 0x040A, Bit: 3, Desc: "BckBst software over-current"},
	{Addr: 0x040A, Bit: 4, Desc: "Inverter Rms current protection"},
	{Addr: 0x040A, Bit: 5, Desc: "PV transient software over-current"},
	{Addr: 0x040A, Bit: 6, Desc: "PV parallel uneven current"},
	{Addr: 0x040A, Bit: 7, Desc: "Output current unbalance"},
	{Addr: 0x040A, Bit: 8, Desc: "PV software over-current protection"},
	{Addr: 0x040A, Bit: 9, Desc: "Balanced circuit over-current protection"},
	{Addr: 0x040A, Bit: 10, Desc: "Resonance protection"},
	{Addr: 0x040A, Bit: 11, Desc: "Software wave-by-wave current limit protection"},
	{Addr: 0x040A, Bit: 12, Desc: "PV branch software over-current 1(enabled by default)"},
	{Addr: 0x040A, Bit: 13, Desc: "Primary DC charge and discharge over current protection"},
	{Addr: 0x040A, Bit: 14, Desc: "Software BUS over-current protection"},
	{Addr: 0x040A, Bit: 15, Desc: "EPS short circuit protection"},

	// Fault 7 (0x040B) - Fault IDs 97-112
	{Addr: 0x040B, Bit: 0, Desc: "LLC bus-bar hardware over-voltage"},
	{Addr: 0x040B, Bit: 1, Desc: "Inverter bus hardware over-voltage"},
	{Addr: 0x040B, Bit: 2, Desc: "BckBst hardware over-current"},
	{Addr: 0x040B, Bit: 3, Desc: "Battery hardware over-current"},
	{Addr: 0x040B, Bit: 4, Desc: "Primary DC hardware GaN tube error"},
	{Addr: 0x040B, Bit: 5, Desc: "PV hardware over-current"},
	{Addr: 0x040B, Bit: 6, Desc: "AC output hardware over-current"},
	{Addr: 0x040B, Bit: 7, Desc: "Hardware differential over-current"},
	{Addr: 0x040B, Bit: 8, Desc: "Meter communication error"},
	{Addr: 0x040B, Bit: 9, Desc: "Serial number model error"},
	{Addr: 0x040B, Bit: 10, Desc: "Hardware version mismatch"},
	{Addr: 0x040B, Bit: 11, Desc: "Generator startup failure"},
	{Addr: 0x040B, Bit: 12, Desc: "Generator overload protection"},
	{Addr: 0x040B, Bit: 13, Desc: "Overload protection 1"},
	{Addr: 0x040B, Bit: 14, Desc: "Overload protection 2"},
	{Addr: 0x040B, Bit: 15, Desc: "Overload protection 3"},

	// Fault 8 (0x040C) - Fault IDs 113-128
	{Addr: 0x040C, Bit: 0, Desc: "Over-temperature load-down"},
	{Addr: 0x040C, Bit: 1, Desc: "Frequency load-down"},
	{Addr: 0x040C, Bit: 2, Desc: "Frequency loading"},
	{Addr: 0x040C, Bit: 3, Desc: "Voltage load-down"},
	{Addr: 0x040C, Bit: 4, Desc: "Voltage loading"},
	{Addr: 0x040C, Bit: 5, Desc: "Low temperature load-down"},
	// Bits 6-7 are N/A
	{Addr: 0x040C, Bit: 8, Desc: "Lightning Protection Fault(DC)"},
	{Addr: 0x040C, Bit: 9, Desc: "Lightning Protection Fault(AC)"},
	{Addr: 0x040C, Bit: 10, Desc: "Output short circuit error"},
	{Addr: 0x040C, Bit: 11, Desc: "Low battery power"},
	{Addr: 0x040C, Bit: 12, Desc: "Battery prohibit discharge alarm"},
	// Bits 13-14 are N/A
	{Addr: 0x040C, Bit: 15, Desc: "Battery input reverse protection"},

	// Fault 9 (0x040D) - Fault IDs 129-144
	{Addr: 0x040D, Bit: 0, Desc: "AC hardware over-current fault"},
	{Addr: 0x040D, Bit: 1, Desc: "Bus over-voltage fault"},
	{Addr: 0x040D, Bit: 2, Desc: "Bus hardware over-voltage fault"},
	{Addr: 0x040D, Bit: 3, Desc: "PV uneven flow fault"},
	{Addr: 0x040D, Bit: 4, Desc: "EPS battery over-current fault"},
	{Addr: 0x040D, Bit: 5, Desc: "Output transient over-current fault"},
	{Addr: 0x040D, Bit: 6, Desc: "AC current unbalance fault"},
	{Addr: 0x040D, Bit: 7, Desc: "Inverter soft start failure fault"},
	{Addr: 0x040D, Bit: 8, Desc: "Input mode setting fault"},
	{Addr: 0x040D, Bit: 9, Desc: "Input over-current fault"},
	{Addr: 0x040D, Bit: 10, Desc: "Input hardware over-current fault"},
	{Addr: 0x040D, Bit: 11, Desc: "Relay permanent failure"},
	{Addr: 0x040D, Bit: 12, Desc: "Bus unbalance fault"},
	{Addr: 0x040D, Bit: 13, Desc: "Lightning Protection Fault(DC)"},
	{Addr: 0x040D, Bit: 14, Desc: "Lightning Protection Fault(AC)"},
	{Addr: 0x040D, Bit: 15, Desc: "Grid relay fault"},

	// Fault 10 (0x040E) - Fault IDs 145-160
	{Addr: 0x040E, Bit: 0, Desc: "USB fault"},
	{Addr: 0x040E, Bit: 1, Desc: "WIFI fault"},
	{Addr: 0x040E, Bit: 2, Desc: "Bluetooth fault"},
	{Addr: 0x040E, Bit: 3, Desc: "RTC clock fault"},
	{Addr: 0x040E, Bit: 4, Desc: "Communication board EEPROM error"},
	{Addr: 0x040E, Bit: 5, Desc: "Communication board FLASH error"},
	{Addr: 0x040E, Bit: 6, Desc: "The battery is partially disconnected"},
	{Addr: 0x040E, Bit: 7, Desc: "Safety version error"},
	{Addr: 0x040E, Bit: 8, Desc: "SCI error(DC)"},
	{Addr: 0x040E, Bit: 9, Desc: "SCI error(AC)"},
	{Addr: 0x040E, Bit: 10, Desc: "SCI error(Fuse)"},
	{Addr: 0x040E, Bit: 11, Desc: "Software version inconsistency"},
	{Addr: 0x040E, Bit: 12, Desc: "Lithium battery 1 communication failure"},
	{Addr: 0x040E, Bit: 13, Desc: "Lithium battery 2 communication failure"},
	{Addr: 0x040E, Bit: 14, Desc: "Lithium battery 3 communication failure"},
	{Addr: 0x040E, Bit: 15, Desc: "Lithium battery 4 communication failure"},

	// Fault 11 (0x040F) - Fault IDs 161-176
	{Addr: 0x040F, Bit: 0, Desc: "Forced shutdown"},
	{Addr: 0x040F, Bit: 1, Desc: "Remote shutdown"},
	{Addr: 0x040F, Bit: 2, Desc: "Drms0 shutdown"},
	{Addr: 0x040F, Bit: 3, Desc: "Power station communication failure shutdown"},
	// Bits 4-6 are N/A
	{Addr: 0x040F, Bit: 7, Desc: "AFCI force shutdown"},
	{Addr: 0x040F, Bit: 8, Desc: "Fan 1 fault"},
	{Addr: 0x040F, Bit: 9, Desc: "Fan 2 fault"},
	{Addr: 0x040F, Bit: 10, Desc: "Fan 3 fault"},
	{Addr: 0x040F, Bit: 11, Desc: "Fan 4 fault"},
	{Addr: 0x040F, Bit: 12, Desc: "Fan 5 fault"},
	{Addr: 0x040F, Bit: 13, Desc: "Fan 6 fault"},
	{Addr: 0x040F, Bit: 14, Desc: "Fan 7 fault"},
	{Addr: 0x040F, Bit: 15, Desc: "Heating film fault"},

	// Fault 12 (0x0410) - Fault IDs 177-192
	{Addr: 0x0410, Bit: 0, Desc: "BMS over-voltage protection"},
	{Addr: 0x0410, Bit: 1, Desc: "BMS under-voltage protection"},
	{Addr: 0x0410, Bit: 2, Desc: "BMS high temperature protection"},
	{Addr: 0x0410, Bit: 3, Desc: "BMS low temperature protection"},
	{Addr: 0x0410, Bit: 4, Desc: "BMS over-current protection"},
	{Addr: 0x0410, Bit: 5, Desc: "BMS short-circuit protection"},
	{Addr: 0x0410, Bit: 6, Desc: "BMS version inconsistency"},
	{Addr: 0x0410, Bit: 7, Desc: "BMS CAN version inconsistency"},
	{Addr: 0x0410, Bit: 8, Desc: "BMS CAN version too low"},
	{Addr: 0x0410, Bit: 9, Desc: "Battery discharge over-temperature protection"},
	{Addr: 0x0410, Bit: 10, Desc: "Battery discharge low temperature protection"},
	{Addr: 0x0410, Bit: 11, Desc: "Battery charging over-temperature protection"},
	{Addr: 0x0410, Bit: 12, Desc: "Arc device communication failure"},
	{Addr: 0x0410, Bit: 13, Desc: "Battery charging low temperature protection"},
	{Addr: 0x0410, Bit: 14, Desc: "PID repair failure"},
	{Addr: 0x0410, Bit: 15, Desc: "PLC module heartbeat lost"},

	// Fault 13 (0x0411) - Fault IDs 193-208
	{Addr: 0x0411, Bit: 0, Desc: "Group string insurance open circuit 1-1"},
	{Addr: 0x0411, Bit: 1, Desc: "Group string insurance open circuit 1-2"},
	{Addr: 0x0411, Bit: 2, Desc: "Group string insurance open circuit 2-1"},
	{Addr: 0x0411, Bit: 3, Desc: "Group string insurance open circuit 2-2"},
	{Addr: 0x0411, Bit: 4, Desc: "Group string insurance open circuit 3-1"},
	{Addr: 0x0411, Bit: 5, Desc: "Group string insurance open circuit 3-2"},
	{Addr: 0x0411, Bit: 6, Desc: "Group string insurance open circuit 4-1"},
	{Addr: 0x0411, Bit: 7, Desc: "Group string insurance open circuit 4-2"},
	{Addr: 0x0411, Bit: 8, Desc: "Group string insurance open circuit 5-1"},
	{Addr: 0x0411, Bit: 9, Desc: "Group string insurance open circuit 5-2"},
	{Addr: 0x0411, Bit: 10, Desc: "Group string insurance open circuit 6-1"},
	{Addr: 0x0411, Bit: 11, Desc: "Group string insurance open circuit 6-2"},
	{Addr: 0x0411, Bit: 12, Desc: "Group string insurance open circuit 7-1"},
	{Addr: 0x0411, Bit: 13, Desc: "Group string insurance open circuit 7-2"},
	{Addr: 0x0411, Bit: 14, Desc: "Group string insurance open circuit 8-1"},
	{Addr: 0x0411, Bit: 15, Desc: "Group string insurance open circuit 8-2"},

	// Fault 14 (0x0412) - Fault IDs 209-224
	{Addr: 0x0412, Bit: 0, Desc: "Group string insurance open circuit 9-1"},
	{Addr: 0x0412, Bit: 1, Desc: "Group string insurance open circuit 9-2"},
	{Addr: 0x0412, Bit: 2, Desc: "Group string insurance open circuit 10-1"},
	{Addr: 0x0412, Bit: 3, Desc: "Group string insurance open circuit 10-2"},
	{Addr: 0x0412, Bit: 4, Desc: "Group string insurance open circuit 11-1"},
	{Addr: 0x0412, Bit: 5, Desc: "Group string insurance open circuit 11-2"},
	{Addr: 0x0412, Bit: 6, Desc: "Group string insurance open circuit 12-1"},
	{Addr: 0x0412, Bit: 7, Desc: "Group string insurance open circuit 12-2"},
	{Addr: 0x0412, Bit: 8, Desc: "Group string insurance open circuit 13-1"},
	{Addr: 0x0412, Bit: 9, Desc: "Group string insurance open circuit 13-2"},
	{Addr: 0x0412, Bit: 10, Desc: "Group string insurance open circuit 14-1"},
	{Addr: 0x0412, Bit: 11, Desc: "Group string insurance open circuit 14-2"},
	{Addr: 0x0412, Bit: 12, Desc: "Group string insurance open circuit 15-1"},
	{Addr: 0x0412, Bit: 13, Desc: "Group string insurance open circuit 15-2"},
	{Addr: 0x0412, Bit: 14, Desc: "Group string insurance open circuit 16-1"},
	{Addr: 0x0412, Bit: 15, Desc: "Group string insurance open circuit 16-2"},

	// Fault 15 (0x0413) - Fault IDs 225-240
	{Addr: 0x0413, Bit: 0, Desc: "Input insurance open circuit 0"},
	{Addr: 0x0413, Bit: 1, Desc: "Input insurance open circuit 1"},
	{Addr: 0x0413, Bit: 2, Desc: "Input insurance open circuit 2"},
	{Addr: 0x0413, Bit: 3, Desc: "Input insurance open circuit 3"},
	{Addr: 0x0413, Bit: 4, Desc: "Input insurance open circuit 4"},
	{Addr: 0x0413, Bit: 5, Desc: "Input insurance open circuit 5"},
	{Addr: 0x0413, Bit: 6, Desc: "Input insurance open circuit 6"},
	{Addr: 0x0413, Bit: 7, Desc: "Input insurance open circuit 7"},
	{Addr: 0x0413, Bit: 8, Desc: "Input insurance open circuit 8"},
	{Addr: 0x0413, Bit: 9, Desc: "Input insurance open circuit 9"},
	{Addr: 0x0413, Bit: 10, Desc: "Input insurance open circuit 10"},
	{Addr: 0x0413, Bit: 11, Desc: "Input insurance open circuit 11"},
	{Addr: 0x0413, Bit: 12, Desc: "Input insurance open circuit 12"},
	{Addr: 0x0413, Bit: 13, Desc: "Input insurance open circuit 13"},
	{Addr: 0x0413, Bit: 14, Desc: "Input insurance open circuit 14"},
	{Addr: 0x0413, Bit: 15, Desc: "Input insurance open circuit 15"},

	// Fault 16 (0x0414) - All N/A (reserved)
	// Fault 17 (0x0415) - All N/A (reserved)
	// Fault 18 (0x0416) - All N/A (reserved)
	// Fault 19 (0x0432) - All N/A (reserved)

	// Fault 20 (0x0433) - Fault IDs 305-320
	{Addr: 0x0433, Bit: 0, Desc: "Control unit over temperature warning"},
	{Addr: 0x0433, Bit: 1, Desc: "Control unit over temperature protection"},
	{Addr: 0x0433, Bit: 2, Desc: "Control unit low temperature warning"},
	{Addr: 0x0433, Bit: 3, Desc: "Temperature sensor fault"},
	{Addr: 0x0433, Bit: 4, Desc: "Abnormal DC port temperature (including temperature and NTC abnormalities)"},
	{Addr: 0x0433, Bit: 5, Desc: "Abnormal AC port temperature (including temperature and NTC abnormalities)"},
	// Bits 6-7 are N/A
	{Addr: 0x0433, Bit: 8, Desc: "Module 4 temperature protection"},
	{Addr: 0x0433, Bit: 9, Desc: "Module 5 temperature protection"},
	{Addr: 0x0433, Bit: 10, Desc: "Module 6 temperature protection"},
	// Bits 11-15 are N/A

	// Fault 21 (0x0434) - Fault IDs 321-336
	{Addr: 0x0434, Bit: 0, Desc: "Grid phase sequence error"},
	{Addr: 0x0434, Bit: 1, Desc: "Phase line to ground failure"},
	{Addr: 0x0434, Bit: 2, Desc: "Anti-short-circuit protection"},
	{Addr: 0x0434, Bit: 3, Desc: "Anti-short-circuit device fault"},
	{Addr: 0x0434, Bit: 4, Desc: "DC relay fault"},
	{Addr: 0x0434, Bit: 5, Desc: "Anti-short-circuit temperature error"},
	{Addr: 0x0434, Bit: 6, Desc: "Module temperature low protection"},
	{Addr: 0x0434, Bit: 7, Desc: "AC start timeout"},
	{Addr: 0x0434, Bit: 8, Desc: "Over modulation"},
	{Addr: 0x0434, Bit: 9, Desc: "Device ID mismatch"},
	{Addr: 0x0434, Bit: 10, Desc: "Internal communicate fault"},
	{Addr: 0x0434, Bit: 11, Desc: "Carrier synchronization signal fault"},
	{Addr: 0x0434, Bit: 12, Desc: "Bus short circuit protection"},
	{Addr: 0x0434, Bit: 13, Desc: "Boost short circuit fault"},
	{Addr: 0x0434, Bit: 14, Desc: "PV+ ground short circuit fault"},
	// Bit 15 is N/A

	// Fault 22 (0x0435) - All N/A (reserved)
	// Fault 23 (0x0436) - All N/A (reserved)

	// Fault 24 (0x0437) - Fault IDs 369-384
	// Bits 0-7 are N/A
	{Addr: 0x0437, Bit: 8, Desc: "Inverter hardware MOS tube fault"},
	{Addr: 0x0437, Bit: 9, Desc: "EPS hardware output over-current protection"},
	{Addr: 0x0437, Bit: 10, Desc: "AFCI self-checking error"},
	{Addr: 0x0437, Bit: 11, Desc: "Load short circuit protection"},
	{Addr: 0x0437, Bit: 12, Desc: "DC switch 1 trip"},
	{Addr: 0x0437, Bit: 13, Desc: "DC switch 2 trip"},
	{Addr: 0x0437, Bit: 14, Desc: "DC switch 3 trip"},
	{Addr: 0x0437, Bit: 15, Desc: "DC switch 4 trip"},

	// Fault 25 (0x0438) - Fault IDs 385-400
	{Addr: 0x0438, Bit: 0, Desc: "Hardware AD sampling bias error"},
	{Addr: 0x0438, Bit: 1, Desc: "Fan 8 fault"},
	// Bits 2-15 are N/A

	// Fault 26 (0x0439) - Fault IDs 401-416
	{Addr: 0x0439, Bit: 0, Desc: "Arc failure 0"},
	{Addr: 0x0439, Bit: 1, Desc: "Arc failure 1"},
	{Addr: 0x0439, Bit: 2, Desc: "Arc failure 2"},
	{Addr: 0x0439, Bit: 3, Desc: "Arc failure 3"},
	{Addr: 0x0439, Bit: 4, Desc: "Arc failure 4"},
	{Addr: 0x0439, Bit: 5, Desc: "Arc failure 5"},
	{Addr: 0x0439, Bit: 6, Desc: "Arc failure 6"},
	{Addr: 0x0439, Bit: 7, Desc: "Arc failure 7"},
	{Addr: 0x0439, Bit: 8, Desc: "Arc failure 8"},
	{Addr: 0x0439, Bit: 9, Desc: "Arc failure 9"},
	{Addr: 0x0439, Bit: 10, Desc: "Arc failure 10"},
	{Addr: 0x0439, Bit: 11, Desc: "Arc failure 11"},
	{Addr: 0x0439, Bit: 12, Desc: "Arc failure 12"},
	{Addr: 0x0439, Bit: 13, Desc: "Arc failure 13"},
	{Addr: 0x0439, Bit: 14, Desc: "Arc failure 14"},
	{Addr: 0x0439, Bit: 15, Desc: "Arc failure 15"},

	// Fault 27 (0x043A) - Fault IDs 417-432
	{Addr: 0x043A, Bit: 0, Desc: "Arc failure 16"},
	{Addr: 0x043A, Bit: 1, Desc: "Arc failure 17"},
	{Addr: 0x043A, Bit: 2, Desc: "Arc failure 18"},
	{Addr: 0x043A, Bit: 3, Desc: "Arc failure 19"},
	{Addr: 0x043A, Bit: 4, Desc: "Arc failure 20"},
	{Addr: 0x043A, Bit: 5, Desc: "Arc failure 21"},
	{Addr: 0x043A, Bit: 6, Desc: "Arc failure 22"},
	{Addr: 0x043A, Bit: 7, Desc: "Arc failure 23"},
	{Addr: 0x043A, Bit: 8, Desc: "Arc failure 24"},
	{Addr: 0x043A, Bit: 9, Desc: "Arc failure 25"},
	{Addr: 0x043A, Bit: 10, Desc: "Arc failure 26"},
	{Addr: 0x043A, Bit: 11, Desc: "Arc failure 27"},
	{Addr: 0x043A, Bit: 12, Desc: "Arc failure 28"},
	{Addr: 0x043A, Bit: 13, Desc: "Arc failure 29"},
	{Addr: 0x043A, Bit: 14, Desc: "Arc failure 30"},
	{Addr: 0x043A, Bit: 15, Desc: "Arc failure 31"},

	// Fault 28 (0x043B) - Fault IDs 433-448
	{Addr: 0x043B, Bit: 0, Desc: "AFCI 1 communication fault"},
	{Addr: 0x043B, Bit: 1, Desc: "AFCI 2 communication fault"},
	// Bits 2-7 are N/A
	{Addr: 0x043B, Bit: 8, Desc: "Modify over voltage protection"},
	{Addr: 0x043B, Bit: 9, Desc: "ARM_DSP protocol version inconsistent"},
	{Addr: 0x043B, Bit: 10, Desc: "ARM_AFCI protocol version inconsistent"},
	{Addr: 0x043B, Bit: 11, Desc: "ARM_DCDC protocol version inconsistent"},
	// Bits 12-15 are N/A

	// Fault 29 (0x043C) - Fault IDs 449-464
	{Addr: 0x043C, Bit: 0, Desc: "The ground zero voltage is too high"},
	{Addr: 0x043C, Bit: 1, Desc: "PV branch software over-current 2(disabled by default)"},
	{Addr: 0x043C, Bit: 2, Desc: "N-line open circuit fault"},
	{Addr: 0x043C, Bit: 3, Desc: "Grid frequency change protection"},
	{Addr: 0x043C, Bit: 4, Desc: "Phase-loss failure"},
	{Addr: 0x043C, Bit: 5, Desc: "PV branch software current regurgitation"},
	{Addr: 0x043C, Bit: 6, Desc: "Inverter voltage sampling error"},
	{Addr: 0x043C, Bit: 7, Desc: "Inverter EPS sampling failure fault"},
	{Addr: 0x043C, Bit: 8, Desc: "Bus voltage consistency error"},
	{Addr: 0x043C, Bit: 9, Desc: "Bus current sampling error"},
	{Addr: 0x043C, Bit: 10, Desc: "Parallel off grid start synchronization fault"},
	{Addr: 0x043C, Bit: 11, Desc: "Primary hardware over-current protection"},
	{Addr: 0x043C, Bit: 12, Desc: "Off-grid prohibited discharge failure"},
	{Addr: 0x043C, Bit: 13, Desc: "Live wire and ground wire short circuit fault"},
	{Addr: 0x043C, Bit: 14, Desc: "Permanent fault of live wire and ground wire short circuit"},
	// Bit 15 is N/A

	// Fault 30 (0x043D) - Fault IDs 465-480
	{Addr: 0x043D, Bit: 0, Desc: "DCDC Fault"},
	{Addr: 0x043D, Bit: 1, Desc: "BCU Fault"},
	{Addr: 0x043D, Bit: 2, Desc: "BMU Fault"},
	{Addr: 0x043D, Bit: 3, Desc: "PID Fault"},
	// Bits 4-7 are N/A
	{Addr: 0x043D, Bit: 8, Desc: "Discharge Mos fault"},
	{Addr: 0x043D, Bit: 9, Desc: "Charge Mos fault"},
	{Addr: 0x043D, Bit: 10, Desc: "Battery NTC fault"},
	{Addr: 0x043D, Bit: 11, Desc: "Cell fault"},
	{Addr: 0x043D, Bit: 12, Desc: "Cell voltage sampling fault"},
	{Addr: 0x043D, Bit: 13, Desc: "Storage fault"},
	// Bits 14-15 are N/A
}

// FaultRegisters defines the Modbus read requests needed to fetch all fault data.
// Two separate ranges due to non-contiguous fault register addresses.
var FaultRegisters = []Probe{
	{Name: "Fault batch 1", Addr: 0x0405, Count: 18}, // 0x0405-0x0416
	{Name: "Fault batch 2", Addr: 0x0432, Count: 12}, // 0x0432-0x043D
}

// DecodeFaults examines fault register bitmap data and returns a list of active fault descriptions.
// The faultData map keys are register addresses, values are the 16-bit register contents.
func DecodeFaults(faultData map[uint16]uint16) []string {
	var active []string
	for _, fb := range FaultTable {
		val, ok := faultData[fb.Addr]
		if !ok {
			continue
		}
		if val&(1<<fb.Bit) != 0 {
			active = append(active, fb.Desc)
		}
	}
	return active
}
