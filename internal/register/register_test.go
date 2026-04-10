package register

import (
	"encoding/binary"
	"testing"
)

func TestFormatValueASCII(t *testing.T) {
	p := Probe{Name: "Test SN", Addr: 0x0445, Count: 10, IsASCII: true}
	data := []byte("AMASS\x00\x00\x00")
	got := FormatValue(p, data)
	want := "AMASS"
	if got != want {
		t.Errorf("FormatValue ASCII = %q, want %q", got, want)
	}
}

func TestFormatValueUnsignedScaled(t *testing.T) {
	p := Probe{Name: "Voltage", Scale: 0.1, Unit: "V"}
	// Encode 5288 as big-endian uint16
	data := make([]byte, 2)
	binary.BigEndian.PutUint16(data, 5288)
	got := FormatValue(p, data)
	want := "528.80 V"
	if got != want {
		t.Errorf("FormatValue unsigned scaled = %q, want %q", got, want)
	}
}

func TestFormatValueSignedNegative(t *testing.T) {
	p := Probe{Name: "Power", Signed: true, Scale: 0.01, Unit: "kW"}
	// Encode -83 as big-endian int16 (two's complement)
	data := make([]byte, 2)
	neg83 := int16(-83)
	binary.BigEndian.PutUint16(data, uint16(neg83))
	got := FormatValue(p, data)
	want := "-0.83 kW"
	if got != want {
		t.Errorf("FormatValue signed negative = %q, want %q", got, want)
	}
}

func TestFormatValueNoScale(t *testing.T) {
	p := Probe{Name: "State"}
	// Encode 164 as big-endian uint16
	data := make([]byte, 2)
	binary.BigEndian.PutUint16(data, 164)
	got := FormatValue(p, data)
	want := "164 (0x00A4)"
	if got != want {
		t.Errorf("FormatValue no scale = %q, want %q", got, want)
	}
}

func TestFormatValueShortData(t *testing.T) {
	p := Probe{Name: "Short"}
	data := []byte{0x01}
	got := FormatValue(p, data)
	want := "<no data>"
	if got != want {
		t.Errorf("FormatValue short data = %q, want %q", got, want)
	}
}

func TestFormatValueSignedNoUnit(t *testing.T) {
	p := Probe{Name: "Raw signed", Signed: true}
	data := make([]byte, 2)
	neg42 := int16(-42)
	binary.BigEndian.PutUint16(data, uint16(neg42))
	got := FormatValue(p, data)
	want := "-42"
	if got != want {
		t.Errorf("FormatValue signed no unit = %q, want %q", got, want)
	}
}

// === Task 1 TDD RED tests ===

func TestProbeGroupStruct(t *testing.T) {
	pg := ProbeGroup{
		Name: "Test Group",
		Probes: []Probe{
			{Name: "Test Probe", Addr: 0x0001, Count: 1},
		},
		Layout: "column",
	}
	if pg.Name != "Test Group" {
		t.Errorf("ProbeGroup.Name = %q, want %q", pg.Name, "Test Group")
	}
	if len(pg.Probes) != 1 {
		t.Errorf("ProbeGroup.Probes len = %d, want 1", len(pg.Probes))
	}
	if pg.Layout != "column" {
		t.Errorf("ProbeGroup.Layout = %q, want %q", pg.Layout, "column")
	}
}

func TestRunningStateEnum(t *testing.T) {
	tests := []struct {
		val  uint16
		want string
	}{
		{0, "Waiting"},
		{2, "Grid-connected"},
		{4, "Recoverable fault"},
		{5, "Permanent fault"},
		{7, "Self-charging"},
		{11, "Standby monitoring"},
	}
	for _, tt := range tests {
		got, ok := RunningStateEnum[tt.val]
		if !ok {
			t.Errorf("RunningStateEnum[%d] not found", tt.val)
			continue
		}
		if got != tt.want {
			t.Errorf("RunningStateEnum[%d] = %q, want %q", tt.val, got, tt.want)
		}
	}
	// Verify all 12 entries exist (0-11)
	if len(RunningStateEnum) != 12 {
		t.Errorf("RunningStateEnum len = %d, want 12", len(RunningStateEnum))
	}
}

func TestSystemGroups(t *testing.T) {
	if len(SystemGroups) != 5 {
		t.Fatalf("SystemGroups len = %d, want 5", len(SystemGroups))
	}

	expectedNames := []string{"Identity", "Firmware", "Status", "Temperatures", "Protection"}
	for i, want := range expectedNames {
		if SystemGroups[i].Name != want {
			t.Errorf("SystemGroups[%d].Name = %q, want %q", i, SystemGroups[i].Name, want)
		}
	}

	// Identity: Inverter SN at 0x0445, Count 10, IsASCII
	identity := SystemGroups[0]
	if len(identity.Probes) < 1 {
		t.Fatal("Identity group has no probes")
	}
	if identity.Probes[0].Addr != 0x0445 {
		t.Errorf("Identity SN addr = 0x%04X, want 0x0445", identity.Probes[0].Addr)
	}
	if identity.Probes[0].Count != 10 {
		t.Errorf("Identity SN count = %d, want 10", identity.Probes[0].Count)
	}
	if !identity.Probes[0].IsASCII {
		t.Error("Identity SN IsASCII should be true")
	}

	// Firmware: 5 probes
	firmware := SystemGroups[1]
	if len(firmware.Probes) != 5 {
		t.Fatalf("Firmware group probes = %d, want 5", len(firmware.Probes))
	}
	fwExpected := []struct {
		name  string
		addr  uint16
		count uint16
	}{
		{"HW version", 0x044D, 2},
		{"Comm SW version", 0x044F, 4},
		{"Master DSP version", 0x0453, 4},
		{"Slave DSP version", 0x0457, 4},
		{"Safety cert version", 0x045B, 2},
	}
	for i, fw := range fwExpected {
		if firmware.Probes[i].Name != fw.name {
			t.Errorf("Firmware[%d].Name = %q, want %q", i, firmware.Probes[i].Name, fw.name)
		}
		if firmware.Probes[i].Addr != fw.addr {
			t.Errorf("Firmware[%d].Addr = 0x%04X, want 0x%04X", i, firmware.Probes[i].Addr, fw.addr)
		}
		if firmware.Probes[i].Count != fw.count {
			t.Errorf("Firmware[%d].Count = %d, want %d", i, firmware.Probes[i].Count, fw.count)
		}
	}

	// Status: Running state with Enum at 0x0404, plus 6 time registers
	status := SystemGroups[2]
	if status.Probes[0].Addr != 0x0404 {
		t.Errorf("Status running state addr = 0x%04X, want 0x0404", status.Probes[0].Addr)
	}
	if status.Probes[0].Enum == nil {
		t.Error("Status running state Enum should not be nil")
	}
	if len(status.Probes) != 7 {
		t.Errorf("Status probes = %d, want 7 (running state + 6 time)", len(status.Probes))
	}

	// Temperatures: 4 probes, all S16
	temps := SystemGroups[3]
	if len(temps.Probes) != 4 {
		t.Fatalf("Temperatures probes = %d, want 4", len(temps.Probes))
	}
	tempExpected := []struct {
		name string
		addr uint16
	}{
		{"Ambient temp 1", 0x0418},
		{"Ambient temp 2", 0x0419},
		{"Radiator temp", 0x041A},
		{"Module temp", 0x0420},
	}
	for i, te := range tempExpected {
		if temps.Probes[i].Name != te.name {
			t.Errorf("Temps[%d].Name = %q, want %q", i, temps.Probes[i].Name, te.name)
		}
		if temps.Probes[i].Addr != te.addr {
			t.Errorf("Temps[%d].Addr = 0x%04X, want 0x%04X", i, temps.Probes[i].Addr, te.addr)
		}
		if !temps.Probes[i].Signed {
			t.Errorf("Temps[%d].Signed should be true", i)
		}
	}

	// Protection: Insulation impedance (0x042B) and Fan speed (0x043E)
	protection := SystemGroups[4]
	if len(protection.Probes) != 2 {
		t.Fatalf("Protection probes = %d, want 2", len(protection.Probes))
	}
	if protection.Probes[0].Addr != 0x042B {
		t.Errorf("Protection[0].Addr = 0x%04X, want 0x042B", protection.Probes[0].Addr)
	}
	if protection.Probes[1].Addr != 0x043E {
		t.Errorf("Protection[1].Addr = 0x%04X, want 0x043E", protection.Probes[1].Addr)
	}
}

func TestGridGroups(t *testing.T) {
	if len(GridGroups) != 7 {
		t.Fatalf("GridGroups len = %d, want 7", len(GridGroups))
	}

	expectedNames := []string{"General", "Phase R", "Phase S", "Phase T", "PCC Power", "Line Voltages", "Load"}
	for i, want := range expectedNames {
		if GridGroups[i].Name != want {
			t.Errorf("GridGroups[%d].Name = %q, want %q", i, GridGroups[i].Name, want)
		}
	}

	// Phase R: column layout, 5 probes
	phaseR := GridGroups[1]
	if phaseR.Layout != "column" {
		t.Errorf("Phase R layout = %q, want %q", phaseR.Layout, "column")
	}
	if len(phaseR.Probes) != 5 {
		t.Fatalf("Phase R probes = %d, want 5", len(phaseR.Probes))
	}
	rExpected := []struct {
		name  string
		addr  uint16
		scale float64
	}{
		{"Voltage", 0x048D, 0.1},
		{"Current", 0x048E, 0.01},
		{"Active power", 0x048F, 0.01},
		{"Reactive power", 0x0490, 0.01},
		{"Power factor", 0x0491, 0.001},
	}
	for i, re := range rExpected {
		if phaseR.Probes[i].Name != re.name {
			t.Errorf("Phase R[%d].Name = %q, want %q", i, phaseR.Probes[i].Name, re.name)
		}
		if phaseR.Probes[i].Addr != re.addr {
			t.Errorf("Phase R[%d].Addr = 0x%04X, want 0x%04X", i, phaseR.Probes[i].Addr, re.addr)
		}
		if phaseR.Probes[i].Scale != re.scale {
			t.Errorf("Phase R[%d].Scale = %f, want %f", i, phaseR.Probes[i].Scale, re.scale)
		}
	}
	// Power factor has no Unit
	if phaseR.Probes[4].Unit != "" {
		t.Errorf("Phase R power factor Unit = %q, want empty", phaseR.Probes[4].Unit)
	}

	// Phase S: column layout, Voltage at 0x0498
	phaseS := GridGroups[2]
	if phaseS.Layout != "column" {
		t.Errorf("Phase S layout = %q, want %q", phaseS.Layout, "column")
	}
	if phaseS.Probes[0].Addr != 0x0498 {
		t.Errorf("Phase S voltage addr = 0x%04X, want 0x0498", phaseS.Probes[0].Addr)
	}
	if phaseS.Probes[1].Addr != 0x0499 {
		t.Errorf("Phase S current addr = 0x%04X, want 0x0499", phaseS.Probes[1].Addr)
	}
	if phaseS.Probes[2].Addr != 0x049A {
		t.Errorf("Phase S active power addr = 0x%04X, want 0x049A", phaseS.Probes[2].Addr)
	}
	if phaseS.Probes[3].Addr != 0x049B {
		t.Errorf("Phase S reactive power addr = 0x%04X, want 0x049B", phaseS.Probes[3].Addr)
	}
	if phaseS.Probes[4].Addr != 0x049C {
		t.Errorf("Phase S power factor addr = 0x%04X, want 0x049C", phaseS.Probes[4].Addr)
	}

	// Phase T: Voltage at 0x04A3
	phaseT := GridGroups[3]
	if phaseT.Layout != "column" {
		t.Errorf("Phase T layout = %q, want %q", phaseT.Layout, "column")
	}
	if phaseT.Probes[0].Addr != 0x04A3 {
		t.Errorf("Phase T voltage addr = 0x%04X, want 0x04A3", phaseT.Probes[0].Addr)
	}

	// Load: Total load power (0x04AF), Total power factor (0x04BD S16 Scale 0.001), Generation efficiency (0x04BF)
	load := GridGroups[6]
	if len(load.Probes) != 3 {
		t.Fatalf("Load probes = %d, want 3", len(load.Probes))
	}
	if load.Probes[0].Addr != 0x04AF {
		t.Errorf("Load total load power addr = 0x%04X, want 0x04AF", load.Probes[0].Addr)
	}
	if load.Probes[1].Addr != 0x04BD {
		t.Errorf("Load total power factor addr = 0x%04X, want 0x04BD", load.Probes[1].Addr)
	}
	if !load.Probes[1].Signed {
		t.Error("Load total power factor should be signed")
	}
	if load.Probes[1].Scale != 0.001 {
		t.Errorf("Load total power factor scale = %f, want 0.001", load.Probes[1].Scale)
	}
	if load.Probes[2].Addr != 0x04BF {
		t.Errorf("Load generation efficiency addr = 0x%04X, want 0x04BF", load.Probes[2].Addr)
	}
}

func TestEPSGroups(t *testing.T) {
	if len(EPSGroups) != 5 {
		t.Fatalf("EPSGroups len = %d, want 5", len(EPSGroups))
	}

	expectedNames := []string{"General", "Phase R", "Phase S", "Phase T", "Emergency Load"}
	for i, want := range expectedNames {
		if EPSGroups[i].Name != want {
			t.Errorf("EPSGroups[%d].Name = %q, want %q", i, EPSGroups[i].Name, want)
		}
	}

	// General: 4 probes
	general := EPSGroups[0]
	if len(general.Probes) != 4 {
		t.Fatalf("EPS General probes = %d, want 4", len(general.Probes))
	}
	if general.Probes[0].Addr != 0x0504 {
		t.Errorf("EPS load active power addr = 0x%04X, want 0x0504", general.Probes[0].Addr)
	}
	if general.Probes[1].Addr != 0x0505 {
		t.Errorf("EPS load reactive power addr = 0x%04X, want 0x0505", general.Probes[1].Addr)
	}
	if general.Probes[2].Addr != 0x0506 {
		t.Errorf("EPS load apparent power addr = 0x%04X, want 0x0506", general.Probes[2].Addr)
	}
	if general.Probes[3].Addr != 0x0507 {
		t.Errorf("EPS output freq addr = 0x%04X, want 0x0507", general.Probes[3].Addr)
	}

	// Phase R: column layout, 2 probes
	phaseR := EPSGroups[1]
	if phaseR.Layout != "column" {
		t.Errorf("EPS Phase R layout = %q, want %q", phaseR.Layout, "column")
	}
	if phaseR.Probes[0].Addr != 0x050A {
		t.Errorf("EPS Phase R output voltage addr = 0x%04X, want 0x050A", phaseR.Probes[0].Addr)
	}
	if phaseR.Probes[1].Addr != 0x050B {
		t.Errorf("EPS Phase R load current addr = 0x%04X, want 0x050B", phaseR.Probes[1].Addr)
	}
	if !phaseR.Probes[1].Signed {
		t.Error("EPS Phase R load current should be signed")
	}

	// Phase S: 0x0512/0x0513
	phaseS := EPSGroups[2]
	if phaseS.Layout != "column" {
		t.Errorf("EPS Phase S layout = %q, want %q", phaseS.Layout, "column")
	}
	if phaseS.Probes[0].Addr != 0x0512 {
		t.Errorf("EPS Phase S output voltage addr = 0x%04X, want 0x0512", phaseS.Probes[0].Addr)
	}
	if phaseS.Probes[1].Addr != 0x0513 {
		t.Errorf("EPS Phase S load current addr = 0x%04X, want 0x0513", phaseS.Probes[1].Addr)
	}

	// Phase T: 0x051A/0x051B
	phaseT := EPSGroups[3]
	if phaseT.Probes[0].Addr != 0x051A {
		t.Errorf("EPS Phase T output voltage addr = 0x%04X, want 0x051A", phaseT.Probes[0].Addr)
	}
	if phaseT.Probes[1].Addr != 0x051B {
		t.Errorf("EPS Phase T load current addr = 0x%04X, want 0x051B", phaseT.Probes[1].Addr)
	}

	// Emergency Load: voltages at 0x0510, 0x0518, 0x0520
	emerg := EPSGroups[4]
	if len(emerg.Probes) != 3 {
		t.Fatalf("Emergency Load probes = %d, want 3", len(emerg.Probes))
	}
	if emerg.Probes[0].Addr != 0x0510 {
		t.Errorf("Emergency Load R addr = 0x%04X, want 0x0510", emerg.Probes[0].Addr)
	}
	if emerg.Probes[1].Addr != 0x0518 {
		t.Errorf("Emergency Load S addr = 0x%04X, want 0x0518", emerg.Probes[1].Addr)
	}
	if emerg.Probes[2].Addr != 0x0520 {
		t.Errorf("Emergency Load T addr = 0x%04X, want 0x0520", emerg.Probes[2].Addr)
	}
}

func TestFormatValueEnum(t *testing.T) {
	p := Probe{
		Name: "Running state",
		Addr: 0x0404,
		Count: 1,
		Enum: RunningStateEnum,
	}
	// Value 2 should return "Grid-connected"
	data := make([]byte, 2)
	binary.BigEndian.PutUint16(data, 2)
	got := FormatValue(p, data)
	if got != "Grid-connected" {
		t.Errorf("FormatValue enum value 2 = %q, want %q", got, "Grid-connected")
	}

	// Unknown value falls back to raw format
	binary.BigEndian.PutUint16(data, 99)
	got = FormatValue(p, data)
	want := "99 (0x0063)"
	if got != want {
		t.Errorf("FormatValue enum unknown value 99 = %q, want %q", got, want)
	}
}

func TestFormatValueScaleNoUnit(t *testing.T) {
	// Power factor: Scale 0.001, no unit, value 990 -> "0.990"
	p := Probe{Name: "Power factor", Signed: true, Scale: 0.001}
	data := make([]byte, 2)
	binary.BigEndian.PutUint16(data, uint16(int16(990)))
	got := FormatValue(p, data)
	if got != "0.990" {
		t.Errorf("FormatValue scale no unit = %q, want %q", got, "0.990")
	}
}

func TestComposeSystemTime(t *testing.T) {
	got := ComposeSystemTime(26, 4, 10, 14, 30, 5)
	want := "2026-04-10 14:30:05"
	if got != want {
		t.Errorf("ComposeSystemTime = %q, want %q", got, want)
	}

	got = ComposeSystemTime(0, 1, 1, 0, 0, 0)
	want = "2000-01-01 00:00:00"
	if got != want {
		t.Errorf("ComposeSystemTime zero = %q, want %q", got, want)
	}
}

// === Task 2 TDD tests: Fault bitmap decoder ===

func TestFaultBitStruct(t *testing.T) {
	fb := FaultBit{Addr: 0x0405, Bit: 0, Desc: "Grid over-voltage"}
	if fb.Addr != 0x0405 {
		t.Errorf("FaultBit.Addr = 0x%04X, want 0x0405", fb.Addr)
	}
	if fb.Bit != 0 {
		t.Errorf("FaultBit.Bit = %d, want 0", fb.Bit)
	}
	if fb.Desc != "Grid over-voltage" {
		t.Errorf("FaultBit.Desc = %q, want %q", fb.Desc, "Grid over-voltage")
	}
}

func TestFaultTableSize(t *testing.T) {
	if len(FaultTable) < 200 {
		t.Errorf("FaultTable len = %d, want > 200", len(FaultTable))
	}
}

func TestFaultTableFirstEntry(t *testing.T) {
	if len(FaultTable) == 0 {
		t.Fatal("FaultTable is empty")
	}
	first := FaultTable[0]
	if first.Addr != 0x0405 {
		t.Errorf("FaultTable[0].Addr = 0x%04X, want 0x0405", first.Addr)
	}
	if first.Bit != 0 {
		t.Errorf("FaultTable[0].Bit = %d, want 0", first.Bit)
	}
	if first.Desc != "Grid over-voltage" {
		t.Errorf("FaultTable[0].Desc = %q, want %q", first.Desc, "Grid over-voltage")
	}
}

func TestFaultTableLeakageCurrent(t *testing.T) {
	found := false
	for _, fb := range FaultTable {
		if fb.Addr == 0x0405 && fb.Bit == 4 && fb.Desc == "Leakage current faults" {
			found = true
			break
		}
	}
	if !found {
		t.Error("FaultTable missing entry for 0x0405 bit 4 'Leakage current faults'")
	}
}

func TestFaultRegisters(t *testing.T) {
	if len(FaultRegisters) != 2 {
		t.Fatalf("FaultRegisters len = %d, want 2", len(FaultRegisters))
	}
	if FaultRegisters[0].Addr != 0x0405 {
		t.Errorf("FaultRegisters[0].Addr = 0x%04X, want 0x0405", FaultRegisters[0].Addr)
	}
	if FaultRegisters[0].Count != 18 {
		t.Errorf("FaultRegisters[0].Count = %d, want 18", FaultRegisters[0].Count)
	}
	if FaultRegisters[1].Addr != 0x0432 {
		t.Errorf("FaultRegisters[1].Addr = 0x%04X, want 0x0432", FaultRegisters[1].Addr)
	}
	if FaultRegisters[1].Count != 12 {
		t.Errorf("FaultRegisters[1].Count = %d, want 12", FaultRegisters[1].Count)
	}
}

func TestDecodeFaultsEmpty(t *testing.T) {
	faultData := map[uint16]uint16{
		0x0405: 0x0000,
		0x0406: 0x0000,
	}
	faults := DecodeFaults(faultData)
	if len(faults) != 0 {
		t.Errorf("DecodeFaults all zeros returned %d faults, want 0", len(faults))
	}
}

func TestDecodeFaultsSingleBit(t *testing.T) {
	faultData := map[uint16]uint16{
		0x0405: 0x0001, // bit 0 set
	}
	faults := DecodeFaults(faultData)
	if len(faults) != 1 {
		t.Fatalf("DecodeFaults single bit returned %d faults, want 1", len(faults))
	}
	if faults[0] != "Grid over-voltage" {
		t.Errorf("DecodeFaults single bit = %q, want %q", faults[0], "Grid over-voltage")
	}
}

func TestDecodeFaultsMultipleBits(t *testing.T) {
	faultData := map[uint16]uint16{
		0x0405: 0x0003, // bits 0+1 set
	}
	faults := DecodeFaults(faultData)
	if len(faults) != 2 {
		t.Fatalf("DecodeFaults two bits returned %d faults, want 2", len(faults))
	}
	if faults[0] != "Grid over-voltage" {
		t.Errorf("DecodeFaults[0] = %q, want %q", faults[0], "Grid over-voltage")
	}
	if faults[1] != "Grid under-voltage" {
		t.Errorf("DecodeFaults[1] = %q, want %q", faults[1], "Grid under-voltage")
	}
}

func TestDecodeFaultsUnknownRegister(t *testing.T) {
	faultData := map[uint16]uint16{
		0xFFFF: 0xFFFF, // register not in table
	}
	faults := DecodeFaults(faultData)
	if len(faults) != 0 {
		t.Errorf("DecodeFaults unknown register returned %d faults, want 0", len(faults))
	}
}

// === Task 2 TDD tests: Dynamic PV group generator ===

func TestGeneratePVGroups2(t *testing.T) {
	groups := GeneratePVGroups(2)
	if len(groups) != 3 {
		t.Fatalf("GeneratePVGroups(2) len = %d, want 3", len(groups))
	}

	// PV 1
	if groups[0].Name != "PV 1" {
		t.Errorf("groups[0].Name = %q, want %q", groups[0].Name, "PV 1")
	}
	if groups[0].Layout != "column" {
		t.Errorf("groups[0].Layout = %q, want %q", groups[0].Layout, "column")
	}
	if len(groups[0].Probes) != 3 {
		t.Fatalf("PV 1 probes = %d, want 3", len(groups[0].Probes))
	}
	if groups[0].Probes[0].Addr != 0x0584 {
		t.Errorf("PV 1 voltage addr = 0x%04X, want 0x0584", groups[0].Probes[0].Addr)
	}
	if groups[0].Probes[0].Scale != 0.1 {
		t.Errorf("PV 1 voltage scale = %f, want 0.1", groups[0].Probes[0].Scale)
	}
	if groups[0].Probes[0].Unit != "V" {
		t.Errorf("PV 1 voltage unit = %q, want %q", groups[0].Probes[0].Unit, "V")
	}
	if groups[0].Probes[1].Addr != 0x0585 {
		t.Errorf("PV 1 current addr = 0x%04X, want 0x0585", groups[0].Probes[1].Addr)
	}
	if !groups[0].Probes[1].Signed {
		t.Error("PV 1 current should be signed")
	}
	if groups[0].Probes[1].Scale != 0.01 {
		t.Errorf("PV 1 current scale = %f, want 0.01", groups[0].Probes[1].Scale)
	}
	if groups[0].Probes[1].Unit != "A" {
		t.Errorf("PV 1 current unit = %q, want %q", groups[0].Probes[1].Unit, "A")
	}
	if groups[0].Probes[2].Addr != 0x0586 {
		t.Errorf("PV 1 power addr = 0x%04X, want 0x0586", groups[0].Probes[2].Addr)
	}
	if groups[0].Probes[2].Scale != 0.01 {
		t.Errorf("PV 1 power scale = %f, want 0.01", groups[0].Probes[2].Scale)
	}
	if groups[0].Probes[2].Unit != "kW" {
		t.Errorf("PV 1 power unit = %q, want %q", groups[0].Probes[2].Unit, "kW")
	}

	// PV 2
	if groups[1].Name != "PV 2" {
		t.Errorf("groups[1].Name = %q, want %q", groups[1].Name, "PV 2")
	}
	if groups[1].Probes[0].Addr != 0x0587 {
		t.Errorf("PV 2 voltage addr = 0x%04X, want 0x0587", groups[1].Probes[0].Addr)
	}
	if groups[1].Probes[1].Addr != 0x0588 {
		t.Errorf("PV 2 current addr = 0x%04X, want 0x0588", groups[1].Probes[1].Addr)
	}
	if groups[1].Probes[2].Addr != 0x0589 {
		t.Errorf("PV 2 power addr = 0x%04X, want 0x0589", groups[1].Probes[2].Addr)
	}

	// Total PV Power
	if groups[2].Name != "Total PV Power" {
		t.Errorf("groups[2].Name = %q, want %q", groups[2].Name, "Total PV Power")
	}
	if groups[2].Layout != "" {
		t.Errorf("Total PV Power layout = %q, want empty", groups[2].Layout)
	}
}

func TestGeneratePVGroups16(t *testing.T) {
	groups := GeneratePVGroups(16)
	if len(groups) != 17 {
		t.Fatalf("GeneratePVGroups(16) len = %d, want 17", len(groups))
	}
	// PV 16 voltage at 0x05B1
	pv16 := groups[15]
	if pv16.Name != "PV 16" {
		t.Errorf("groups[15].Name = %q, want %q", pv16.Name, "PV 16")
	}
	if pv16.Probes[0].Addr != 0x05B1 {
		t.Errorf("PV 16 voltage addr = 0x%04X, want 0x05B1", pv16.Probes[0].Addr)
	}
}

func TestGeneratePVGroupsTotalPower(t *testing.T) {
	groups := GeneratePVGroups(4)
	total := groups[len(groups)-1]
	if total.Name != "Total PV Power" {
		t.Errorf("Total group name = %q, want %q", total.Name, "Total PV Power")
	}
	if len(total.Probes) != 1 {
		t.Fatalf("Total PV Power probes = %d, want 1", len(total.Probes))
	}
	if total.Probes[0].Addr != 0x05C4 {
		t.Errorf("Total PV Power addr = 0x%04X, want 0x05C4", total.Probes[0].Addr)
	}
	if total.Probes[0].Scale != 0.1 {
		t.Errorf("Total PV Power scale = %f, want 0.1", total.Probes[0].Scale)
	}
	if total.Probes[0].Unit != "kW" {
		t.Errorf("Total PV Power unit = %q, want %q", total.Probes[0].Unit, "kW")
	}
}

func TestGeneratePVGroupsColumnLayout(t *testing.T) {
	groups := GeneratePVGroups(3)
	for i := 0; i < 3; i++ {
		if groups[i].Layout != "column" {
			t.Errorf("PV %d layout = %q, want %q", i+1, groups[i].Layout, "column")
		}
	}
	if groups[3].Layout != "" {
		t.Errorf("Total PV Power layout = %q, want empty", groups[3].Layout)
	}
}

// === Phase 04 Task 1 TDD tests: U32, BatteryStateEnum, GenerateBatteryGroups ===

func TestFormatValueU32(t *testing.T) {
	p := Probe{Name: "Energy", U32: true, Count: 2, Scale: 0.01, Unit: "kWh"}
	// Encode 23900: hi_word=0, lo_word=23900
	data := make([]byte, 4)
	binary.BigEndian.PutUint16(data[:2], 0)     // high word
	binary.BigEndian.PutUint16(data[2:4], 23900) // low word
	got := FormatValue(p, data)
	want := "239.00 kWh"
	if got != want {
		t.Errorf("FormatValue U32 = %q, want %q", got, want)
	}
}

func TestFormatValueU32Large(t *testing.T) {
	p := Probe{Name: "Energy", U32: true, Count: 2, Scale: 0.1, Unit: "kWh"}
	// Encode 1234567: hi_word=18 (1234567 >> 16 = 18), lo_word=54919 (1234567 & 0xFFFF = 54919)
	data := make([]byte, 4)
	binary.BigEndian.PutUint16(data[:2], 18)
	binary.BigEndian.PutUint16(data[2:4], 54919)
	got := FormatValue(p, data)
	want := "123456.70 kWh"
	if got != want {
		t.Errorf("FormatValue U32 large = %q, want %q", got, want)
	}
}

func TestFormatValueU32NoScale(t *testing.T) {
	p := Probe{Name: "Raw", U32: true, Count: 2}
	// Encode 42: hi_word=0, lo_word=42
	data := make([]byte, 4)
	binary.BigEndian.PutUint16(data[:2], 0)
	binary.BigEndian.PutUint16(data[2:4], 42)
	got := FormatValue(p, data)
	want := "42 (0x0000002A)"
	if got != want {
		t.Errorf("FormatValue U32 no scale = %q, want %q", got, want)
	}
}

func TestFormatValueU32ShortData(t *testing.T) {
	p := Probe{Name: "Short", U32: true}
	data := []byte{0x00, 0x01} // only 2 bytes, need 4
	got := FormatValue(p, data)
	want := "<no data>"
	if got != want {
		t.Errorf("FormatValue U32 short data = %q, want %q", got, want)
	}
}

func TestBatteryStateEnum(t *testing.T) {
	expected := map[uint16]string{
		1: "Charging",
		2: "Discharging",
		3: "Sleeping",
		4: "Fault",
		5: "Loss reduction",
	}
	if len(BatteryStateEnum) != 5 {
		t.Errorf("BatteryStateEnum len = %d, want 5", len(BatteryStateEnum))
	}
	for k, v := range expected {
		got, ok := BatteryStateEnum[k]
		if !ok {
			t.Errorf("BatteryStateEnum[%d] not found", k)
			continue
		}
		if got != v {
			t.Errorf("BatteryStateEnum[%d] = %q, want %q", k, got, v)
		}
	}
}

func TestGenerateBatteryGroups2(t *testing.T) {
	groups := GenerateBatteryGroups(2)
	// 2 channel groups + 1 global stats = 3
	if len(groups) != 3 {
		t.Fatalf("GenerateBatteryGroups(2) len = %d, want 3", len(groups))
	}

	// Channel 1
	ch1 := groups[0]
	if ch1.Name != "Channel 1" {
		t.Errorf("groups[0].Name = %q, want %q", ch1.Name, "Channel 1")
	}
	if ch1.Layout != "column" {
		t.Errorf("groups[0].Layout = %q, want %q", ch1.Layout, "column")
	}
	if len(ch1.Probes) != 10 {
		t.Fatalf("Channel 1 probes = %d, want 10", len(ch1.Probes))
	}
	// First 7 probes: pack info at base 0x0604
	if ch1.Probes[0].Addr != 0x0604 {
		t.Errorf("Ch1 voltage addr = 0x%04X, want 0x0604", ch1.Probes[0].Addr)
	}

	// Channel 2
	ch2 := groups[1]
	if ch2.Name != "Channel 2" {
		t.Errorf("groups[1].Name = %q, want %q", ch2.Name, "Channel 2")
	}
	if len(ch2.Probes) != 10 {
		t.Fatalf("Channel 2 probes = %d, want 10", len(ch2.Probes))
	}
	// Channel 2 base = 0x0604 + 7*(2-1) = 0x060B
	if ch2.Probes[0].Addr != 0x060B {
		t.Errorf("Ch2 voltage addr = 0x%04X, want 0x060B", ch2.Probes[0].Addr)
	}

	// Each channel has 10 probes (7 pack info + charge limit + discharge limit + state)
	// State probe should have BatteryStateEnum
	stateProbe := ch1.Probes[9]
	if stateProbe.Enum == nil {
		t.Error("Channel 1 state probe Enum should not be nil")
	}

	// Global Stats
	global := groups[2]
	if global.Name != "Global Stats" {
		t.Errorf("groups[2].Name = %q, want %q", global.Name, "Global Stats")
	}
	if global.Layout != "" {
		t.Errorf("Global Stats layout = %q, want empty", global.Layout)
	}
	if len(global.Probes) != 5 {
		t.Fatalf("Global Stats probes = %d, want 5", len(global.Probes))
	}
	if global.Probes[0].Addr != 0x0667 {
		t.Errorf("Global Stats charge/discharge addr = 0x%04X, want 0x0667", global.Probes[0].Addr)
	}
	if global.Probes[4].Addr != 0x066B {
		t.Errorf("Global Stats total capacity addr = 0x%04X, want 0x066B", global.Probes[4].Addr)
	}
}

func TestGenerateBatteryGroups1(t *testing.T) {
	groups := GenerateBatteryGroups(1)
	// 1 channel group + 1 global stats = 2
	if len(groups) != 2 {
		t.Fatalf("GenerateBatteryGroups(1) len = %d, want 2", len(groups))
	}
	// State probe should have BatteryStateEnum
	ch1 := groups[0]
	stateProbe := ch1.Probes[9]
	if stateProbe.Enum == nil {
		t.Error("Channel 1 state probe Enum should not be nil")
	}
	if _, ok := stateProbe.Enum[1]; !ok {
		t.Error("State probe Enum missing key 1 (Charging)")
	}
}

func TestGenerateBatteryGroupsAddresses(t *testing.T) {
	groups := GenerateBatteryGroups(2)
	ch1 := groups[0]
	ch2 := groups[1]

	// Channel 1 pack info: voltage=0x0604
	if ch1.Probes[0].Addr != 0x0604 {
		t.Errorf("Ch1 voltage = 0x%04X, want 0x0604", ch1.Probes[0].Addr)
	}
	// Channel 2 pack info: voltage=0x060B
	if ch2.Probes[0].Addr != 0x060B {
		t.Errorf("Ch2 voltage = 0x%04X, want 0x060B", ch2.Probes[0].Addr)
	}
	// Channel 1 charge limit: 0x0644
	if ch1.Probes[7].Addr != 0x0644 {
		t.Errorf("Ch1 charge limit = 0x%04X, want 0x0644", ch1.Probes[7].Addr)
	}
	// Channel 1 discharge limit: 0x0645
	if ch1.Probes[8].Addr != 0x0645 {
		t.Errorf("Ch1 discharge limit = 0x%04X, want 0x0645", ch1.Probes[8].Addr)
	}
	// Channel 1 state: 0x0646
	if ch1.Probes[9].Addr != 0x0646 {
		t.Errorf("Ch1 state = 0x%04X, want 0x0646", ch1.Probes[9].Addr)
	}
	// Channel 2 charge limit: 0x0648
	if ch2.Probes[7].Addr != 0x0648 {
		t.Errorf("Ch2 charge limit = 0x%04X, want 0x0648", ch2.Probes[7].Addr)
	}
	// Channel 2 state: 0x064A
	if ch2.Probes[9].Addr != 0x064A {
		t.Errorf("Ch2 state = 0x%04X, want 0x064A", ch2.Probes[9].Addr)
	}
}
