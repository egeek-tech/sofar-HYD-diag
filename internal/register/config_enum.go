package register

// Configuration register enum maps for Sofar Modbus-G3 V1.38 protocol.
// Each map translates raw register values to human-readable labels.

// BaudRateEnum maps RS485 baud rate selection (0x100C) values.
var BaudRateEnum = map[uint16]string{
	0:  "4800 bps",
	1:  "9600 bps",
	2:  "19200 bps",
	3:  "38400 bps",
	4:  "57600 bps",
	5:  "115200 bps",
	10: "1200 bps",
	11: "2400 bps",
}

// PVInputModeEnum maps PV input mode selection (0x1010) values.
var PVInputModeEnum = map[uint16]string{
	0: "Parallel",
	1: "Independent",
}

// AntiBackFlowEnum maps anti-back-flow enable control (0x1023) values.
var AntiBackFlowEnum = map[uint16]string{
	0: "Prohibit",
	1: "Default anti-back-flow",
	2: "Three phase average",
	3: "Phase power mode",
	4: "Master slave anti-back-flow",
}

// EPSControlEnum maps emergency power supply enable control (0x1029) values.
var EPSControlEnum = map[uint16]string{
	0: "Turn off EPS",
	1: "EPS enable, prohibit cold start",
	2: "EPS enable, allow cold start",
}

// LanguageEnum maps inverter menu language setting (0x1034) values.
var LanguageEnum = map[uint16]string{
	0:  "Chinese",
	1:  "English",
	2:  "Italian",
	3:  "German",
	4:  "French",
	5:  "Portuguese",
	6:  "Ukrainian",
	7:  "Slovak",
	8:  "Spanish",
	9:  "Finnish",
	10: "Polish",
	11: "Korean",
	12: "Japanese",
	13: "Russian",
	14: "Czech",
}

// ProhibitEnableEnum is a generic reusable enum for 0=Disabled, 1=Enabled registers.
var ProhibitEnableEnum = map[uint16]string{
	0: "Disabled",
	1: "Enabled",
}

// BatteryProtocolEnum maps communication protocol (0x1046) values.
var BatteryProtocolEnum = map[uint16]string{
	0:  "Sofar BMS/DEFAULT",
	1:  "Pie Energy/PYLON",
	2:  "Sofar/GENERAL",
	3:  "AMASS",
	4:  "LG",
	5:  "Alpha.ESS",
	6:  "CATL",
	7:  "Weco",
	8:  "Fronus",
	9:  "EMS",
	10: "Nilar",
	11: "BTS 5K",
	12: "Move for",
	13: "STRONG C",
	14: "Reserve",
	15: "NEOOM",
}

// CellTypeEnum maps cell type selection (0x1051) values.
var CellTypeEnum = map[uint16]string{
	0: "Lead acid (custom)",
	1: "Lithium iron phosphate",
	2: "Ternary",
	3: "Lithium titanate",
	4: "AGM",
	5: "Gel",
	6: "Flooded",
}

// BatteryUsageEnum maps battery usage configuration (0x1073) values.
var BatteryUsageEnum = map[uint16]string{
	0: "All charged/discharged",
	1: "Only charged",
	2: "Only discharged",
	3: "All stopped",
}

// EnergyStorageModeEnum maps energy storage operating mode (0x1110) values.
var EnergyStorageModeEnum = map[uint16]string{
	0: "Self-generation",
	1: "Time-sharing tariff",
	2: "Timed charge/discharge",
	3: "Passive",
	4: "Peak-shaving",
	5: "Off-grid",
	6: "Generator",
}

// DRMSEnum maps DRMs shutdown control (0x1071) values.
var DRMSEnum = map[uint16]string{
	0: "Disabled",
	1: "Enabled",
}

// ParallelModeEnum maps local parallel control register (0x1035) values.
var ParallelModeEnum = map[uint16]string{
	0: "Disable parallel",
	1: "Enable AC parallel",
	2: "Enable AC+BAT parallel",
}

// GridDetectionEnum maps grid detection and control (0x1067) values.
// Bit0: Phase loss detection enabled; other bits reserved.
var GridDetectionEnum = map[uint16]string{
	0: "Disabled",
	1: "Enabled",
}

// DryContactEnum maps dry contact control (0x1064) values.
var DryContactEnum = map[uint16]string{
	0: "Disabled",
	1: "Generator mode",
	2: "Relay mode 1 (off-grid output high, rest low)",
	3: "Relay mode 2 (off-grid output low, rest high)",
	4: "Intelligent load control",
}

// SafetyCountryEnum maps safety country code (0x1028) values.
// The high byte indicates country code, low byte indicates region code.
// Common country codes from V1.38 spec section 5.4.9.
var SafetyCountryEnum = map[uint16]string{
	0:  "Default",
	1:  "Germany",
	2:  "Italy",
	3:  "Spain",
	4:  "France",
	5:  "UK",
	6:  "Australia",
	7:  "Brazil",
	8:  "USA",
	9:  "Japan",
	10: "Korea",
	11: "Austria",
	12: "India",
	13: "Thailand",
	14: "Czech Republic",
	15: "Poland",
	16: "Netherlands",
	17: "Belgium",
	18: "Greece",
	19: "Portugal",
	20: "South Africa",
}

// RemoteOnOffEnum maps remote power on/off control (0x1104) values.
var RemoteOnOffEnum = map[uint16]string{
	0: "Remote shutdown",
	1: "Remote boot up",
}

// PowerControlEnum maps power control mode (0x1105) bitmask values.
// Bit0: Active power enable, Bit1: Reactive power enable, etc.
var PowerControlEnum = map[uint16]string{
	0: "Disabled",
	1: "Active power enabled",
	2: "Reactive power enabled",
	3: "Active + Reactive enabled",
}

// ChargeDischargeControlEnum maps timed charge/discharge enable control (0x1112) values.
// Bit0: Charge enable, Bit1: Discharge enable.
var ChargeDischargeControlEnum = map[uint16]string{
	0: "Disabled",
	1: "Charge enabled",
	2: "Discharge enabled",
	3: "Both enabled",
}

// ChargingSourceEnum maps off-grid charging source selection (0x1144) values.
var ChargingSourceEnum = map[uint16]string{
	0: "Grid",
	1: "Generator",
	2: "Reserved",
}

// ProtectionEnableEnum maps safety protection enable registers (bitmask bit0=enable).
var ProtectionEnableEnum = map[uint16]string{
	0: "Disabled",
	1: "Enabled",
}

// ReactiveControlModeEnum maps reactive power enable control (0x0980) bitmask values.
// Bit0: Reactive power enable bit, Bit1-3: Reactive mode.
var ReactiveControlModeEnum = map[uint16]string{
	0: "Off",
	1: "Reactive power enabled",
	2: "Static PF mode",
	3: "External PF mode",
}

// EnableStatusEnum maps the status result of *-enable registers (0x100A, 0x100F, etc.).
// These registers return the status of the last write operation.
var EnableStatusEnum = map[uint16]string{
	0:     "Success",
	1:     "In operation",
	65531: "Failed (controller refused)",
	65532: "Failed (controller not responding)",
	65533: "Failed (function disabled)",
	65534: "Failed (parameter storage failed)",
	65535: "Failed (input parameters incorrect)",
}

// InputChannelTypeEnum maps input channel type selection (0x1011-0x1020) values.
// Values 0=not in use, 1-127=PV panel, 128-255=battery, 256-383=fan input.
var InputChannelTypeEnum = map[uint16]string{
	0: "Not in use",
	1: "PV panel",
	2: "Battery",
	3: "Fan input",
}

// TimeShareControlModeEnum maps time sharing enable control (0x1121) values.
var TimeShareControlModeEnum = map[uint16]string{
	0: "Prohibit",
	1: "Enable",
}

// PeakShavingBuyEnum maps buying electricity to charge batteries (0x1134) values.
var PeakShavingBuyEnum = map[uint16]string{
	0: "Prohibited",
	1: "Enabled",
}

