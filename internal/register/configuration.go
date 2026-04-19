package register

// ConfigurationGroups contains all configuration register definitions organized into ProbeGroups.
// Covers sections 5.2.x (Safety Parameter Area) and 5.3.x (Parameter Configuration Area)
// of the Sofar Modbus-G3 V1.38 protocol.
//
// Registers verified against real hardware on 2026-04-15 using tools/config-sweep.
// Probes returning Modbus exception 0x02 (illegal data address) have been removed.
// See tools/config-sweep/results.json for the full sweep report.
var ConfigurationGroups = buildConfigurationGroups()

func buildConfigurationGroups() []ProbeGroup {
	groups := []ProbeGroup{
		// === HIGH PRIORITY (most user-relevant) ===

		// 5.3.1 System parameter configuration (0x1004-0x103F)
		{Name: "System Config", Probes: []Probe{
			{Name: "RS485 address", Addr: 0x100B, Count: 1, Scale: 1},
			{Name: "RS485 baud rate", Addr: 0x100C, Count: 1, Enum: BaudRateEnum},
			{Name: "RS485 stop bit", Addr: 0x100D, Count: 1},
			{Name: "RS485 parity", Addr: 0x100E, Count: 1},
			{Name: "PV input mode", Addr: 0x1010, Count: 1, Enum: PVInputModeEnum},
			{Name: "Input channel 1 type", Addr: 0x1011, Count: 1, Enum: InputChannelTypeEnum},
			{Name: "Input channel 2 type", Addr: 0x1012, Count: 1, Enum: InputChannelTypeEnum},
			{Name: "Anti-back-flow control", Addr: 0x1023, Count: 1, Enum: AntiBackFlowEnum},
			{Name: "Anti-back-flow power", Addr: 0x1024, Count: 1, Signed: true, Unit: "W", Scale: 100},
			{Name: "IV curve scanning", Addr: 0x1025, Count: 1, Enum: ProhibitEnableEnum},
			{Name: "Safety country code", Addr: 0x1028, Count: 1, Enum: SafetyCountryEnum},
			{Name: "Emergency power supply", Addr: 0x1029, Count: 1, Enum: EPSControlEnum},
			{Name: "EPS wait time", Addr: 0x102A, Count: 1, Unit: "s", Scale: 1},
			{Name: "Battery auto activation", Addr: 0x102B, Count: 1, Enum: ProhibitEnableEnum},
			{Name: "Heating film enable", Addr: 0x102F, Count: 1, Enum: ProhibitEnableEnum},
			{Name: "Language", Addr: 0x1034, Count: 1, Enum: LanguageEnum},
			{Name: "Parallel mode", Addr: 0x1035, Count: 1, Enum: ParallelModeEnum},
			{Name: "Parallel control mode", Addr: 0x1036, Count: 1},
			{Name: "Parallel address", Addr: 0x1037, Count: 1},
			{Name: "Imbalance support", Addr: 0x1038, Count: 1, Enum: ProhibitEnableEnum},
			{Name: "Power generation multiplier", Addr: 0x1039, Count: 1, Scale: 0.001},
			{Name: "Buy power multiplier", Addr: 0x103A, Count: 1, Scale: 0.001},
			{Name: "Sell power multiplier", Addr: 0x103B, Count: 1, Scale: 0.001},
			{Name: "Charge amount multiplier", Addr: 0x103C, Count: 1, Scale: 0.001},
			{Name: "Discharge volume multiplier", Addr: 0x103D, Count: 1, Scale: 0.001},
		}},

		// 5.3.2 Battery parameter configuration (0x1044-0x105B)
		{Name: "Battery Config", Probes: []Probe{
			{Name: "Battery serial number", Addr: 0x1044, Count: 1},
			{Name: "Battery address", Addr: 0x1045, Count: 1},
			{Name: "Communication protocol", Addr: 0x1046, Count: 1, Enum: BatteryProtocolEnum},
			{Name: "Over-voltage protection", Addr: 0x1047, Count: 1, Unit: "V", Scale: 0.1},
			{Name: "Charging voltage", Addr: 0x1048, Count: 1, Unit: "V", Scale: 0.1},
			{Name: "Under-voltage protection", Addr: 0x1049, Count: 1, Unit: "V", Scale: 0.1},
			{Name: "Min discharge voltage", Addr: 0x104A, Count: 1, Unit: "V", Scale: 0.1},
			{Name: "Max charge current", Addr: 0x104B, Count: 1, Unit: "A", Scale: 0.01},
			{Name: "Max discharge current", Addr: 0x104C, Count: 1, Unit: "A", Scale: 0.01},
			{Name: "DOD grid-connected", Addr: 0x104D, Count: 1, Unit: "%", Scale: 1},
			{Name: "EOD off-grid", Addr: 0x104E, Count: 1, Unit: "%", Scale: 1},
			{Name: "Battery capacity", Addr: 0x104F, Count: 1, Unit: "Ah", Scale: 1},
			{Name: "Rated voltage", Addr: 0x1050, Count: 1, Unit: "V", Scale: 0.1},
			{Name: "Cell type", Addr: 0x1051, Count: 1, Enum: CellTypeEnum},
			{Name: "Off-grid recovery hysteresis", Addr: 0x1052, Count: 1, Unit: "%", Scale: 1},
			{Name: "Battery address 2", Addr: 0x1054, Count: 1},
			{Name: "Battery address 3", Addr: 0x1055, Count: 1},
			{Name: "Battery address 4", Addr: 0x1056, Count: 1},
			{Name: "Lead acid temp coefficient", Addr: 0x1057, Count: 1, Signed: true, Unit: "mV/Cell", Scale: 0.1},
			{Name: "Lead acid recovery voltage", Addr: 0x1058, Count: 1, Unit: "V", Scale: 0.1},
			{Name: "Lead acid float voltage", Addr: 0x105A, Count: 1, Unit: "V", Scale: 0.1},
			{Name: "Maximum charging SOC", Addr: 0x105B, Count: 1, Unit: "%", Scale: 1},
		}},

		// 5.3.11 Energy storage mode (0x1110, 0x1184-0x118F)
		{Name: "Energy Storage Mode", Probes: []Probe{
			{Name: "Operating mode", Addr: 0x1110, Count: 1, Enum: EnergyStorageModeEnum},
			{Name: "Passive mode timeout", Addr: 0x1184, Count: 1, Unit: "s", Scale: 1},
			{Name: "Passive mode timeout action", Addr: 0x1185, Count: 1},
			{Name: "Manual mode desired grid power", Addr: 0x1187, Count: 2, U32: true, Signed: true, Unit: "W", Scale: 1},
			{Name: "Manual mode min battery power", Addr: 0x1189, Count: 2, U32: true, Signed: true, Unit: "W", Scale: 1},
			{Name: "Manual mode max battery power", Addr: 0x118B, Count: 2, U32: true, Signed: true, Unit: "W", Scale: 1},
			{Name: "Manual mode permissible sell", Addr: 0x118D, Count: 2, U32: true, Signed: true, Unit: "W", Scale: 1},
			{Name: "Manual mode purchase of power", Addr: 0x118F, Count: 2, U32: true, Signed: true, Unit: "W", Scale: 1},
		}},

		// 5.3.3 Function parameter configuration 1 (0x1060-0x1076)
		{Name: "Function Config", Probes: []Probe{
			{Name: "PCC device config", Addr: 0x1060, Count: 1},
			{Name: "Resonance detection", Addr: 0x1061, Count: 1},
			{Name: "CT ratio", Addr: 0x1062, Count: 1},
			{Name: "Hard anti-back-flow", Addr: 0x1063, Count: 1, Enum: ProhibitEnableEnum},
			{Name: "Dry contact control", Addr: 0x1064, Count: 1, Enum: DryContactEnum},
			{Name: "Wet contact control", Addr: 0x1065, Count: 1, Enum: ProhibitEnableEnum},
			{Name: "PV branch over-current", Addr: 0x1066, Count: 1, Unit: "A", Scale: 0.1},
			{Name: "Grid detection", Addr: 0x1067, Count: 1, Enum: GridDetectionEnum},
			{Name: "Missing phase domain", Addr: 0x1068, Count: 1, Unit: "%", Scale: 0.1},
			{Name: "PCC bias power", Addr: 0x1069, Count: 1, Signed: true, Unit: "W", Scale: 1},
			{Name: "Purchase power limit enable", Addr: 0x106A, Count: 1, Enum: ProhibitEnableEnum},
			{Name: "Purchase power limit", Addr: 0x106B, Count: 1, Unit: "kW", Scale: 0.1},
			{Name: "Intelligent load mode", Addr: 0x106C, Count: 1},
			{Name: "Intelligent load power", Addr: 0x106F, Count: 1, Unit: "kW", Scale: 0.1},
			{Name: "Parallel module number", Addr: 0x1070, Count: 1},
			{Name: "DRMs control", Addr: 0x1071, Count: 1, Enum: DRMSEnum},
			{Name: "Battery usage config", Addr: 0x1073, Count: 1, Enum: BatteryUsageEnum},
			{Name: "Grid side port type", Addr: 0x1075, Count: 1},
			{Name: "Generator side port type", Addr: 0x1076, Count: 1},
		}},

		// 5.3.4 Function parameter configuration 2 (0x1084-0x10F1)
		{Name: "Function Config 2", Probes: []Probe{
			{Name: "Arc detection enable", Addr: 0x1084, Count: 1, Enum: ProhibitEnableEnum},
			{Name: "Peak to peak value", Addr: 0x1085, Count: 1},
			{Name: "Variance setting", Addr: 0x1086, Count: 1},
			{Name: "Harmonic energy setting", Addr: 0x1087, Count: 1},
			{Name: "Amplitude variance", Addr: 0x1088, Count: 1},
			{Name: "Time domain weight", Addr: 0x1089, Count: 1},
			{Name: "Frequency domain weight", Addr: 0x108A, Count: 1},
			{Name: "Arc detection sensitivity", Addr: 0x108B, Count: 1},
			{Name: "Arc alarm threshold", Addr: 0x108C, Count: 1},
			{Name: "PLC operation control", Addr: 0x10A0, Count: 1},
			{Name: "BLE Bluetooth running", Addr: 0x10A5, Count: 1},
			{Name: "PID control word", Addr: 0x10AF, Count: 1, Enum: ProhibitEnableEnum},
			{Name: "PID start auto-run", Addr: 0x10B0, Count: 1, Enum: ProhibitEnableEnum},
			{Name: "Generator control", Addr: 0x10B9, Count: 1},
			{Name: "Generator power", Addr: 0x10BA, Count: 1, Unit: "kW", Scale: 0.1},
			{Name: "Generator startup voltage", Addr: 0x10BB, Count: 1, Unit: "V", Scale: 0.1},
			{Name: "Generator stop voltage", Addr: 0x10BC, Count: 1, Unit: "V", Scale: 0.1},
			{Name: "Generator SOC", Addr: 0x10BD, Count: 1, Unit: "%", Scale: 1},
		}},

		// 5.3.5 Remote control parameter (0x1104-0x110D)
		{Name: "Remote Control", Probes: []Probe{
			{Name: "Remote on/off", Addr: 0x1104, Count: 1, Enum: RemoteOnOffEnum},
			{Name: "Power control mode", Addr: 0x1105, Count: 1, Enum: PowerControlEnum},
			{Name: "Export power limit", Addr: 0x1106, Count: 1, Unit: "%", Scale: 0.1},
			{Name: "Import power limit", Addr: 0x1107, Count: 1, Unit: "%", Scale: 0.1},
			{Name: "Reactive power percent", Addr: 0x1108, Count: 1, Signed: true, Unit: "%", Scale: 0.1},
			{Name: "Power factor", Addr: 0x1109, Count: 1, Signed: true, Scale: 0.01},
			{Name: "Active power change rate", Addr: 0x110A, Count: 1, Unit: "%Pn/min", Scale: 1},
			{Name: "Reactive response time", Addr: 0x110B, Count: 1, Unit: "s", Scale: 0.1},
			{Name: "Fixed reactive power (SVG)", Addr: 0x110C, Count: 1, Signed: true, Unit: "kVar", Scale: 0.1},
			{Name: "SVG PF compensation", Addr: 0x110D, Count: 1, Signed: true, Unit: "kVar", Scale: 0.1},
		}},

		// 5.3.6 Timed charging and discharging (0x1111-0x111F)
		{Name: "Timed Charge/Discharge", Probes: []Probe{
			{Name: "Rule serial number", Addr: 0x1111, Count: 1},
			{Name: "Charge/discharge enable", Addr: 0x1112, Count: 1, Enum: ChargeDischargeControlEnum},
			{Name: "Charge start time", Addr: 0x1113, Count: 1},
			{Name: "Charge end time", Addr: 0x1114, Count: 1},
			{Name: "Discharge start time", Addr: 0x1115, Count: 1},
			{Name: "Discharge end time", Addr: 0x1116, Count: 1},
			{Name: "Charging power", Addr: 0x1117, Count: 2, U32: true, Unit: "W", Scale: 1},
			{Name: "Discharge power", Addr: 0x1119, Count: 2, U32: true, Unit: "W", Scale: 1},
		}},

		// 5.3.7 Time sharing control (0x1120-0x112F)
		{Name: "Time Sharing Control", Probes: []Probe{
			{Name: "Rule serial number", Addr: 0x1120, Count: 1},
			{Name: "Enable control", Addr: 0x1121, Count: 1, Enum: ProhibitEnableEnum},
			{Name: "Start time", Addr: 0x1122, Count: 1},
			{Name: "End time", Addr: 0x1123, Count: 1},
			{Name: "Target SOC", Addr: 0x1124, Count: 1, Unit: "%", Scale: 1},
			{Name: "Target power", Addr: 0x1125, Count: 2, U32: true, Unit: "W", Scale: 1},
			{Name: "Effective date", Addr: 0x1127, Count: 1},
			{Name: "Expiration date", Addr: 0x1128, Count: 1},
			{Name: "Effective week", Addr: 0x1129, Count: 1},
			{Name: "Control mode", Addr: 0x112A, Count: 1},
			{Name: "Additional feature", Addr: 0x112B, Count: 1},
		}},

		// 5.3.8 Peak shaving mode (0x1130-0x1135)
		{Name: "Peak Shaving", Probes: []Probe{
			{Name: "Upper limit purchase power", Addr: 0x1130, Count: 2, U32: true, Unit: "W", Scale: 1},
			{Name: "Upper limit sell power", Addr: 0x1132, Count: 2, U32: true, Unit: "W", Scale: 1},
			{Name: "Buy to charge batteries", Addr: 0x1134, Count: 1, Enum: ProhibitEnableEnum},
			{Name: "Battery backup SOC", Addr: 0x1135, Count: 1, Unit: "%", Scale: 1},
		}},

		// 5.3.9 Power feeding priority (0x113A-0x113B)
		{Name: "Power Feeding Priority", Probes: []Probe{
			{Name: "Feed-in power", Addr: 0x113A, Count: 2, U32: true, Unit: "W", Scale: 1},
		}},

		// 5.3.10 Off-grid mode (0x1144-0x1146)
		{Name: "Off-Grid Mode", Probes: []Probe{
			{Name: "Charging source", Addr: 0x1144, Count: 1, Enum: ChargingSourceEnum},
			{Name: "Grid draw power", Addr: 0x1145, Count: 1, Unit: "kW", Scale: 0.01},
			{Name: "Max generator power draw", Addr: 0x1146, Count: 1, Unit: "kW", Scale: 0.01},
		}},

		// 5.3.12 Power station communication (0x11C4-0x11C8)
		{Name: "Communication Protection", Probes: []Probe{
			{Name: "Comm interruption protection", Addr: 0x11C4, Count: 1, Enum: ProhibitEnableEnum},
			{Name: "Comm interruption recovery", Addr: 0x11C5, Count: 1},
			{Name: "Comm interruption threshold", Addr: 0x11C6, Count: 1, Unit: "min", Scale: 1},
			{Name: "Comm interruption data source", Addr: 0x11C7, Count: 1},
		}},
	}

	// Append Safety groups
	groups = append(groups, safetyGroups()...)

	return groups
}

// safetyGroups returns ProbeGroups for the Safety Parameter Area sections 5.2.x.
func safetyGroups() []ProbeGroup {
	return []ProbeGroup{
		// 5.2.1 Safety: Power On (0x0800-0x080B)
		{Name: "Safety: Power On", Probes: []Probe{
			{Name: "Grid connection wait time", Addr: 0x0800, Count: 1, Unit: "s", Scale: 1},
			{Name: "Rise rate", Addr: 0x0801, Count: 1, Unit: "%Pn/min", Scale: 1},
			{Name: "Reconnect wait time", Addr: 0x0802, Count: 1, Unit: "s", Scale: 1},
			{Name: "Rise speed after recovery", Addr: 0x0803, Count: 1, Unit: "%Pn/min", Scale: 1},
			{Name: "Start-up voltage high", Addr: 0x0804, Count: 1, Unit: "V", Scale: 0.1},
			{Name: "Start-up voltage low", Addr: 0x0805, Count: 1, Unit: "V", Scale: 0.1},
			{Name: "Start-up frequency high", Addr: 0x0806, Count: 1, Unit: "Hz", Scale: 0.01},
			{Name: "Start-up frequency low", Addr: 0x0807, Count: 1, Unit: "Hz", Scale: 0.01},
			{Name: "Reconnect voltage high", Addr: 0x0808, Count: 1, Unit: "V", Scale: 0.1},
			{Name: "Reconnect voltage low", Addr: 0x0809, Count: 1, Unit: "V", Scale: 0.1},
			{Name: "Reconnect frequency high", Addr: 0x080A, Count: 1, Unit: "Hz", Scale: 0.01},
			{Name: "Reconnect frequency low", Addr: 0x080B, Count: 1, Unit: "Hz", Scale: 0.01},
		}},

		// 5.2.2 Safety: Voltage Protection (0x0840-0x0851)
		{Name: "Safety: Voltage Protection", Probes: []Probe{
			{Name: "Protection enable", Addr: 0x0840, Count: 1, Enum: ProtectionEnableEnum},
			{Name: "Rated grid voltage", Addr: 0x0841, Count: 1, Unit: "V", Scale: 0.1},
			{Name: "Over-voltage 1 value", Addr: 0x0842, Count: 1, Unit: "V", Scale: 0.1},
			{Name: "Over-voltage 1 time", Addr: 0x0843, Count: 1, Unit: "ms", Scale: 10},
			{Name: "Over-voltage 2 value", Addr: 0x0844, Count: 1, Unit: "V", Scale: 0.1},
			{Name: "Over-voltage 2 time", Addr: 0x0845, Count: 1, Unit: "ms", Scale: 10},
			{Name: "Over-voltage 3 value", Addr: 0x0846, Count: 1, Unit: "V", Scale: 0.1},
			{Name: "Over-voltage 3 time", Addr: 0x0847, Count: 1, Unit: "ms", Scale: 10},
			{Name: "Under-voltage 1 value", Addr: 0x0848, Count: 1, Unit: "V", Scale: 0.1},
			{Name: "Under-voltage 1 time", Addr: 0x0849, Count: 1, Unit: "ms", Scale: 10},
			{Name: "Under-voltage 2 value", Addr: 0x084A, Count: 1, Unit: "V", Scale: 0.1},
			{Name: "Under-voltage 2 time", Addr: 0x084B, Count: 1, Unit: "ms", Scale: 10},
			{Name: "Under-voltage 3 value", Addr: 0x084C, Count: 1, Unit: "V", Scale: 0.1},
			{Name: "Under-voltage 3 time", Addr: 0x084D, Count: 1, Unit: "ms", Scale: 10},
			{Name: "10-min over-voltage value", Addr: 0x084E, Count: 1, Unit: "V", Scale: 0.1},
			{Name: "OV1 time high", Addr: 0x084F, Count: 1},
			{Name: "OV2 time high", Addr: 0x0850, Count: 1},
			{Name: "OV3 time high", Addr: 0x0851, Count: 1},
		}},

		// 5.2.3 Safety: Frequency Protection (0x0880-0x0895)
		{Name: "Safety: Frequency Protection", Probes: []Probe{
			{Name: "Protection enable", Addr: 0x0880, Count: 1, Enum: ProtectionEnableEnum},
			{Name: "Rated frequency", Addr: 0x0881, Count: 1, Unit: "Hz", Scale: 0.01},
			{Name: "Over-frequency 1 value", Addr: 0x0882, Count: 1, Unit: "Hz", Scale: 0.01},
			{Name: "Over-frequency 1 time", Addr: 0x0883, Count: 1, Unit: "ms", Scale: 10},
			{Name: "Over-frequency 2 value", Addr: 0x0884, Count: 1, Unit: "Hz", Scale: 0.01},
			{Name: "Over-frequency 2 time", Addr: 0x0885, Count: 1, Unit: "ms", Scale: 10},
			{Name: "Over-frequency 3 value", Addr: 0x0886, Count: 1, Unit: "Hz", Scale: 0.01},
			{Name: "Over-frequency 3 time", Addr: 0x0887, Count: 1, Unit: "ms", Scale: 10},
			{Name: "Under-frequency 1 value", Addr: 0x0888, Count: 1, Unit: "Hz", Scale: 0.01},
			{Name: "Under-frequency 1 time", Addr: 0x0889, Count: 1, Unit: "ms", Scale: 10},
			{Name: "Under-frequency 2 value", Addr: 0x088A, Count: 1, Unit: "Hz", Scale: 0.01},
			{Name: "Under-frequency 2 time", Addr: 0x088B, Count: 1, Unit: "ms", Scale: 10},
			{Name: "Under-frequency 3 value", Addr: 0x088C, Count: 1, Unit: "Hz", Scale: 0.01},
			{Name: "Under-frequency 3 time", Addr: 0x088D, Count: 1, Unit: "ms", Scale: 10},
			{Name: "OF1 time high", Addr: 0x088E, Count: 1},
			{Name: "OF2 time high", Addr: 0x088F, Count: 1},
			{Name: "OF3 time high", Addr: 0x0890, Count: 1},
			{Name: "UF1 time high", Addr: 0x0891, Count: 1},
			{Name: "UF2 time high", Addr: 0x0892, Count: 1},
			{Name: "UF3 time high", Addr: 0x0893, Count: 1},
			{Name: "Rate of change value", Addr: 0x0894, Count: 1, Unit: "Hz/s", Scale: 0.01},
			{Name: "Rate of change time", Addr: 0x0895, Count: 1, Unit: "ms", Scale: 10},
		}},

		// 5.2.4 Safety: DCI Protection (0x08C0-0x08CC)
		{Name: "Safety: DCI Protection", Probes: []Probe{
			{Name: "DCI enable", Addr: 0x08C0, Count: 1, Enum: ProtectionEnableEnum},
			{Name: "DCI level 1 value", Addr: 0x08C1, Count: 1, Unit: "mA", Scale: 1},
			{Name: "DCI level 1 time", Addr: 0x08C2, Count: 1, Unit: "ms", Scale: 10},
			{Name: "DCI level 2 value", Addr: 0x08C3, Count: 1, Unit: "mA", Scale: 1},
			{Name: "DCI level 2 time", Addr: 0x08C4, Count: 1, Unit: "ms", Scale: 10},
			{Name: "DCI level 3 value", Addr: 0x08C5, Count: 1, Unit: "mA", Scale: 1},
			{Name: "DCI level 3 time", Addr: 0x08C6, Count: 1, Unit: "ms", Scale: 10},
			{Name: "DCI R-phase test", Addr: 0x08C7, Count: 1, Signed: true, Unit: "mA", Scale: 1},
			{Name: "DCI S-phase test", Addr: 0x08C8, Count: 1, Signed: true, Unit: "mA", Scale: 1},
			{Name: "DCI T-phase test", Addr: 0x08C9, Count: 1, Signed: true, Unit: "mA", Scale: 1},
			{Name: "DCI level 1 ratio", Addr: 0x08CA, Count: 1, Unit: "%", Scale: 0.01},
			{Name: "DCI level 2 ratio", Addr: 0x08CB, Count: 1, Unit: "%", Scale: 0.01},
			{Name: "DCI level 3 ratio", Addr: 0x08CC, Count: 1, Unit: "%", Scale: 0.01},
		}},

		// 5.2.9 Safety: Island/GFCI/ISO (0x0A00-0x0A09)
		{Name: "Safety: Island/GFCI/ISO", Probes: []Probe{
			{Name: "Island detection enable", Addr: 0x0A00, Count: 1, Enum: ProtectionEnableEnum},
			{Name: "GFCI enable", Addr: 0x0A01, Count: 1, Enum: ProtectionEnableEnum},
			{Name: "ISO enable", Addr: 0x0A02, Count: 1, Enum: ProtectionEnableEnum},
			{Name: "Insulation impedance", Addr: 0x0A03, Count: 1, Unit: "k\u03a9", Scale: 1},
			{Name: "Leakage current limit", Addr: 0x0A04, Count: 1, Unit: "mA", Scale: 1},
			{Name: "Leakage current value", Addr: 0x0A05, Count: 1, Unit: "mA/kVA", Scale: 1},
			{Name: "PE and N control", Addr: 0x0A06, Count: 1},
			{Name: "Island detection sensitivity", Addr: 0x0A07, Count: 1},
			{Name: "Ground fault threshold", Addr: 0x0A08, Count: 1, Unit: "k\u03a9", Scale: 1},
			{Name: "Power calculation control", Addr: 0x0A09, Count: 1},
		}},

		// 5.2.7 Safety: Reactive Power (0x0980-0x099A) -- Key registers
		{Name: "Safety: Reactive Power", Probes: []Probe{
			{Name: "Reactive control enable", Addr: 0x0980, Count: 1, Enum: ReactiveControlModeEnum},
			{Name: "Power factor", Addr: 0x0981, Count: 1, Signed: true, Scale: 0.0001},
			{Name: "Fixed reactive percent", Addr: 0x0982, Count: 1, Signed: true, Unit: "%", Scale: 0.01},
			{Name: "First PF value", Addr: 0x0983, Count: 1, Signed: true, Scale: 0.0001},
			{Name: "First power percent", Addr: 0x0984, Count: 1, Signed: true, Unit: "%", Scale: 1},
			{Name: "Second PF value", Addr: 0x0985, Count: 1, Signed: true, Scale: 0.0001},
			{Name: "Second power percent", Addr: 0x0986, Count: 1, Signed: true, Unit: "%", Scale: 1},
			{Name: "Third PF value", Addr: 0x0987, Count: 1, Signed: true, Scale: 0.0001},
			{Name: "Third power percent", Addr: 0x0988, Count: 1, Signed: true, Unit: "%", Scale: 1},
			{Name: "Fourth PF value", Addr: 0x0989, Count: 1, Signed: true, Scale: 0.0001},
			{Name: "Fourth power percent", Addr: 0x098A, Count: 1, Signed: true, Unit: "%", Scale: 1},
			{Name: "LockinV voltage percent", Addr: 0x098B, Count: 1, Unit: "%", Scale: 1},
			{Name: "LockoutV voltage percent", Addr: 0x098C, Count: 1, Unit: "%", Scale: 1},
			{Name: "Phase type", Addr: 0x099F, Count: 1},
			{Name: "Regulation duration", Addr: 0x09A0, Count: 1, Unit: "s", Scale: 1},
			{Name: "Max lag reactive percent", Addr: 0x09A1, Count: 1, Unit: "%Pn", Scale: 0.01},
		}},
	}
}
