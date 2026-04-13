package register

import (
	"encoding/binary"
	"fmt"
	"strings"
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

	// Status: Running state with Enum at 0x0404, plus synthetic System time
	status := SystemGroups[2]
	if status.Probes[0].Addr != 0x0404 {
		t.Errorf("Status running state addr = 0x%04X, want 0x0404", status.Probes[0].Addr)
	}
	if status.Probes[0].Enum == nil {
		t.Error("Status running state Enum should not be nil")
	}
	if len(status.Probes) != 2 {
		t.Errorf("Status probes = %d, want 2 (running state + synthetic System time)", len(status.Probes))
	}
	if status.Probes[1].Name != "System time" {
		t.Errorf("Status[1].Name = %q, want %q", status.Probes[1].Name, "System time")
	}
	if status.Probes[1].Addr != 0x042C {
		t.Errorf("Status[1].Addr = 0x%04X, want 0x042C", status.Probes[1].Addr)
	}
	if status.Probes[1].Count != 0 {
		t.Errorf("Status[1].Count = %d, want 0 (synthetic probe)", status.Probes[1].Count)
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
	want := "14:30:05 10-04-2026"
	if got != want {
		t.Errorf("ComposeSystemTime = %q, want %q", got, want)
	}

	got = ComposeSystemTime(0, 1, 1, 0, 0, 0)
	want = "00:00:00 01-01-2000"
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

func TestFormatValueU32Signed(t *testing.T) {
	p := Probe{Name: "Signed32", U32: true, Signed: true, Scale: 0.01, Unit: "W"}
	// Encode -100 as uint32: 0xFFFFFF9C
	data := make([]byte, 4)
	binary.BigEndian.PutUint16(data[:2], 0xFFFF)
	binary.BigEndian.PutUint16(data[2:4], 0xFF9C)
	got := FormatValue(p, data)
	want := "-1.00 W"
	if got != want {
		t.Errorf("FormatValue U32 signed = %q, want %q", got, want)
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

// === Phase 04 Task 2 TDD tests: ProbeGroup Type, BMSInfoGroups, BMSProtectionProbes, StatisticsGroups, DecodeBMSClock, DecodeTopology ===

func TestProbeGroupType(t *testing.T) {
	pg := ProbeGroup{Name: "Protection", Type: "bitmap"}
	if pg.Type != "bitmap" {
		t.Errorf("ProbeGroup.Type = %q, want %q", pg.Type, "bitmap")
	}
}

func TestBMSInfoGroups(t *testing.T) {
	groups := BMSInfoGroups()
	if len(groups) < 1 {
		t.Fatal("BMSInfoGroups returned empty slice")
	}
	bmsInfo := groups[0]
	if bmsInfo.Name != "BMS Info" {
		t.Errorf("BMSInfoGroups[0].Name = %q, want %q", bmsInfo.Name, "BMS Info")
	}

	// Check for system clock hi at 0x9004
	foundClockHi := false
	foundSN := false
	for _, p := range bmsInfo.Probes {
		if p.Addr == 0x9004 {
			foundClockHi = true
		}
		if p.Addr == 0x9024 && p.Count == 10 && p.IsASCII {
			foundSN = true
		}
	}
	if !foundClockHi {
		t.Error("BMSInfoGroups missing probe at 0x9004 (clock hi)")
	}
	if !foundSN {
		t.Error("BMSInfoGroups missing SN probe at 0x9024 (Count 10, IsASCII)")
	}

	// Check key probes exist
	expectedAddrs := []uint16{0x9004, 0x9005, 0x9006, 0x9007, 0x900B, 0x900C, 0x900D, 0x900E, 0x900F, 0x9010, 0x9011, 0x9012, 0x9013, 0x9024, 0x9018, 0x9019, 0x901A, 0x901B}
	addrSet := make(map[uint16]bool)
	for _, p := range bmsInfo.Probes {
		addrSet[p.Addr] = true
	}
	for _, addr := range expectedAddrs {
		if !addrSet[addr] {
			t.Errorf("BMSInfoGroups missing probe at 0x%04X", addr)
		}
	}
}

func TestBMSProtectionProbes(t *testing.T) {
	probes := BMSProtectionProbes()
	if len(probes) != 6 {
		t.Fatalf("BMSProtectionProbes len = %d, want 6", len(probes))
	}
	expectedAddrs := []uint16{0x9014, 0x9015, 0x9016, 0x9017, 0x901C, 0x901D}
	for i, addr := range expectedAddrs {
		if probes[i].Addr != addr {
			t.Errorf("BMSProtectionProbes[%d].Addr = 0x%04X, want 0x%04X", i, probes[i].Addr, addr)
		}
		if probes[i].Count != 1 {
			t.Errorf("BMSProtectionProbes[%d].Count = %d, want 1", i, probes[i].Count)
		}
	}
}

func TestStatisticsGroups(t *testing.T) {
	groups := StatisticsGroups()
	if len(groups) != 2 {
		t.Fatalf("StatisticsGroups len = %d, want 2", len(groups))
	}

	expectedNames := []string{"Today", "Total"}
	for i, want := range expectedNames {
		if groups[i].Name != want {
			t.Errorf("StatisticsGroups[%d].Name = %q, want %q", i, groups[i].Name, want)
		}
	}

	// Each group has 6 probes, all U32=true, Count=2
	for i, g := range groups {
		if len(g.Probes) != 6 {
			t.Errorf("StatisticsGroups[%d] probes = %d, want 6", i, len(g.Probes))
			continue
		}
		for j, p := range g.Probes {
			if !p.U32 {
				t.Errorf("StatisticsGroups[%d].Probes[%d].U32 = false, want true", i, j)
			}
			if p.Count != 2 {
				t.Errorf("StatisticsGroups[%d].Probes[%d].Count = %d, want 2", i, j, p.Count)
			}
			if p.Unit != "kWh" {
				t.Errorf("StatisticsGroups[%d].Probes[%d].Unit = %q, want %q", i, j, p.Unit, "kWh")
			}
		}
	}

	// Today scale = 0.01
	for j, p := range groups[0].Probes {
		if p.Scale != 0.01 {
			t.Errorf("Today.Probes[%d].Scale = %f, want 0.01", j, p.Scale)
		}
	}

	// Total scale = 0.1
	for j, p := range groups[1].Probes {
		if p.Scale != 0.1 {
			t.Errorf("Total.Probes[%d].Scale = %f, want 0.1", j, p.Scale)
		}
	}
}

func TestStatisticsAddresses(t *testing.T) {
	groups := StatisticsGroups()

	// Today starts at 0x0684
	if groups[0].Probes[0].Addr != 0x0684 {
		t.Errorf("Today gen addr = 0x%04X, want 0x0684", groups[0].Probes[0].Addr)
	}
	// Total starts at 0x0686
	if groups[1].Probes[0].Addr != 0x0686 {
		t.Errorf("Total gen addr = 0x%04X, want 0x0686", groups[1].Probes[0].Addr)
	}

	// Stride 4 between metrics within each group
	// Today: gen=0x0684, consumption=0x0688, bought=0x068C, sold=0x0690, bat_charge=0x0694, bat_discharge=0x0698
	todayExpected := []uint16{0x0684, 0x0688, 0x068C, 0x0690, 0x0694, 0x0698}
	for i, addr := range todayExpected {
		if groups[0].Probes[i].Addr != addr {
			t.Errorf("Today.Probes[%d].Addr = 0x%04X, want 0x%04X", i, groups[0].Probes[i].Addr, addr)
		}
	}
}

func TestBMSInfoGroupsIncludesOnlineBitmap(t *testing.T) {
	groups := BMSInfoGroups()
	found := false
	for _, g := range groups {
		for _, p := range g.Probes {
			if p.Addr == 0x9022 {
				found = true
				if p.Name != "Online Bitmap" {
					t.Errorf("expected name 'Online Bitmap', got %q", p.Name)
				}
			}
		}
	}
	if !found {
		t.Error("BMSInfoGroups missing probe at 0x9022")
	}
}

func TestDecodeBMSClock(t *testing.T) {
	// Encode 2026-04-10 14:03:05
	var val uint32 = 0x6914E0C5
	got := DecodeBMSClock(val)
	want := "2026-04-10 14:03:05"
	if got != want {
		t.Errorf("DecodeBMSClock(0x%08X) = %q, want %q", val, got, want)
	}
}

func TestDecodeTopology(t *testing.T) {
	parallelStrings, packsPerString := DecodeTopology(0x020A)
	if parallelStrings != 2 {
		t.Errorf("DecodeTopology parallelStrings = %d, want 2", parallelStrings)
	}
	if packsPerString != 10 {
		t.Errorf("DecodeTopology packsPerString = %d, want 10", packsPerString)
	}
}

// === Phase 05 Plan 01: Pack probe definitions, bitmap tables, EncodePackQuery, DecodeBalanceState ===

func TestEncodePackQuery(t *testing.T) {
	tests := []struct {
		name                              string
		input, tower, pack, towersPerInput int
		want                              uint16
	}{
		// 0-based encoding: group = (input-1)*tpi + (tower-1), packIdx = pack-1
		{"input1 tower2 pack5 tpi2", 1, 2, 5, 2, 0x0104},  // group=1, pack=4
		{"input2 tower1 pack1 tpi2", 2, 1, 1, 2, 0x0200},  // group=2, pack=0
		{"input1 tower1 pack1 tpi1", 1, 1, 1, 1, 0x0000},  // group=0, pack=0
		{"input1 tower1 pack10 tpi2", 1, 1, 10, 2, 0x0009}, // group=0, pack=9
		{"input1 tower1 pack6 tpi2", 1, 1, 6, 2, 0x0005},  // group=0, pack=5 (UI "Pack 6")
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EncodePackQuery(tt.input, tt.tower, tt.pack, tt.towersPerInput)
			if got != tt.want {
				t.Errorf("EncodePackQuery(%d,%d,%d,%d) = 0x%04X, want 0x%04X",
					tt.input, tt.tower, tt.pack, tt.towersPerInput, got, tt.want)
			}
		})
	}
}

func TestPackRTProbes(t *testing.T) {
	probes := PackRTProbes()
	if len(probes) < 30 {
		t.Errorf("PackRTProbes returned %d probes, want >= 30", len(probes))
	}

	// First probe should be Pack ID at 0x9044
	if probes[0].Name != "Pack ID" || probes[0].Addr != 0x9044 {
		t.Errorf("first probe = {%q, 0x%04X}, want {\"Pack ID\", 0x9044}", probes[0].Name, probes[0].Addr)
	}

	// Build lookup by name for specific checks
	byName := make(map[string]Probe)
	for _, p := range probes {
		byName[p.Name] = p
	}

	// Serial Number: ASCII, 10 registers
	if p, ok := byName["Serial Number"]; !ok {
		t.Error("missing Serial Number probe")
	} else {
		if p.Addr != 0x9047 {
			t.Errorf("Serial Number Addr = 0x%04X, want 0x9047", p.Addr)
		}
		if p.Count != 10 {
			t.Errorf("Serial Number Count = %d, want 10", p.Count)
		}
		if !p.IsASCII {
			t.Error("Serial Number should be ASCII")
		}
	}

	// Total Voltage
	if p, ok := byName["Total Voltage"]; !ok {
		t.Error("missing Total Voltage probe")
	} else {
		if p.Addr != 0x9079 {
			t.Errorf("Total Voltage Addr = 0x%04X, want 0x9079", p.Addr)
		}
		if p.Scale != 0.1 {
			t.Errorf("Total Voltage Scale = %f, want 0.1", p.Scale)
		}
		if p.Unit != "V" {
			t.Errorf("Total Voltage Unit = %q, want \"V\"", p.Unit)
		}
	}

	// Check Cell 1
	if p, ok := byName["Cell 1"]; !ok {
		t.Error("missing Cell 1 probe")
	} else {
		if p.Addr != 0x9051 {
			t.Errorf("Cell 1 Addr = 0x%04X, want 0x9051", p.Addr)
		}
		if p.Scale != 0.001 {
			t.Errorf("Cell 1 Scale = %f, want 0.001", p.Scale)
		}
		if p.Unit != "V" {
			t.Errorf("Cell 1 Unit = %q, want \"V\"", p.Unit)
		}
	}
	// Check Cell 16 (last cell per D-05)
	if p, ok := byName["Cell 16"]; !ok {
		t.Error("missing Cell 16 probe")
	} else {
		if p.Addr != 0x9060 {
			t.Errorf("Cell 16 Addr = 0x%04X, want 0x9060", p.Addr)
		}
		if p.Scale != 0.001 {
			t.Errorf("Cell 16 Scale = %f, want 0.001", p.Scale)
		}
	}

	// Cell 17 should NOT exist (only 16 cells per D-05)
	if _, ok := byName["Cell 17"]; ok {
		t.Error("Cell 17 should not exist (only 16 cells per D-05)")
	}

	// Verify exactly 16 cell voltage probes (D-05)
	cellCount := 0
	for _, p := range probes {
		if strings.HasPrefix(p.Name, "Cell ") && p.Scale == 0.001 && p.Unit == "V" {
			cellCount++
		}
	}
	if cellCount != 16 {
		t.Errorf("found %d cell voltage probes, want 16", cellCount)
	}

	// Current: signed, scale 0.1, unit A
	if p, ok := byName["Current"]; !ok {
		t.Error("missing Current probe")
	} else {
		if p.Addr != 0x9071 {
			t.Errorf("Current Addr = 0x%04X, want 0x9071", p.Addr)
		}
		if !p.Signed {
			t.Error("Current should be Signed")
		}
		if p.Scale != 0.1 {
			t.Errorf("Current Scale = %f, want 0.1", p.Scale)
		}
		if p.Unit != "A" {
			t.Errorf("Current Unit = %q, want \"A\"", p.Unit)
		}
	}

	// Temp 1-4 at 0x906B-0x906E, Signed, Scale 0.1, Unit C
	tempAddrs := map[string]uint16{"Temp 1": 0x906B, "Temp 2": 0x906C, "Temp 3": 0x906D, "Temp 4": 0x906E}
	for name, wantAddr := range tempAddrs {
		p, ok := byName[name]
		if !ok {
			t.Errorf("missing %s probe", name)
			continue
		}
		if p.Addr != wantAddr {
			t.Errorf("%s Addr = 0x%04X, want 0x%04X", name, p.Addr, wantAddr)
		}
		if !p.Signed {
			t.Errorf("%s should be Signed", name)
		}
		if p.Scale != 0.1 {
			t.Errorf("%s Scale = %f, want 0.1", name, p.Scale)
		}
	}

	// MOS Temp and Env Temp: signed, scale 0.1
	if p, ok := byName["MOS Temp"]; !ok {
		t.Error("missing MOS Temp probe")
	} else {
		if p.Addr != 0x906F {
			t.Errorf("MOS Temp Addr = 0x%04X, want 0x906F", p.Addr)
		}
		if !p.Signed {
			t.Error("MOS Temp should be Signed")
		}
		if p.Scale != 0.1 {
			t.Errorf("MOS Temp Scale = %f, want 0.1", p.Scale)
		}
	}
	if p, ok := byName["Env Temp"]; !ok {
		t.Error("missing Env Temp probe")
	} else {
		if p.Addr != 0x9070 {
			t.Errorf("Env Temp Addr = 0x%04X, want 0x9070", p.Addr)
		}
		if !p.Signed {
			t.Error("Env Temp should be Signed")
		}
		if p.Scale != 0.1 {
			t.Errorf("Env Temp Scale = %f, want 0.1", p.Scale)
		}
	}

	// Balance State, Alarm Status, Protection Status, Fault Status
	statusProbes := map[string]uint16{
		"Balance State":     0x9075,
		"Alarm Status":      0x9076,
		"Protection Status": 0x9077,
		"Fault Status":      0x9078,
	}
	for name, wantAddr := range statusProbes {
		p, ok := byName[name]
		if !ok {
			t.Errorf("missing %s probe", name)
			continue
		}
		if p.Addr != wantAddr {
			t.Errorf("%s Addr = 0x%04X, want 0x%04X", name, p.Addr, wantAddr)
		}
	}

	// Min/Max Cell Voltage
	if p, ok := byName["Min Cell Voltage"]; !ok {
		t.Error("missing Min Cell Voltage probe")
	} else {
		if p.Addr != 0x906A {
			t.Errorf("Min Cell Voltage Addr = 0x%04X, want 0x906A", p.Addr)
		}
		if p.Scale != 0.001 {
			t.Errorf("Min Cell Voltage Scale = %f, want 0.001", p.Scale)
		}
		if p.Unit != "V" {
			t.Errorf("Min Cell Voltage Unit = %q, want \"V\"", p.Unit)
		}
	}
	if p, ok := byName["Max Cell Voltage"]; !ok {
		t.Error("missing Max Cell Voltage probe")
	} else {
		if p.Addr != 0x9069 {
			t.Errorf("Max Cell Voltage Addr = 0x%04X, want 0x9069", p.Addr)
		}
		if p.Scale != 0.001 {
			t.Errorf("Max Cell Voltage Scale = %f, want 0.001", p.Scale)
		}
	}
}

func TestPackInfoProbes(t *testing.T) {
	probes := PackInfoProbes()
	if len(probes) < 6 {
		t.Errorf("PackInfoProbes returned %d probes, want >= 6", len(probes))
	}

	byName := make(map[string]Probe)
	for _, p := range probes {
		byName[p.Name] = p
	}

	// SOH
	if p, ok := byName["SOH"]; !ok {
		t.Error("missing SOH probe")
	} else {
		if p.Addr != 0x910A {
			t.Errorf("SOH Addr = 0x%04X, want 0x910A", p.Addr)
		}
		if p.Scale != 0.1 {
			t.Errorf("SOH Scale = %f, want 0.1", p.Scale)
		}
		if p.Unit != "%" {
			t.Errorf("SOH Unit = %q, want \"%%\"", p.Unit)
		}
	}

	// Rated Capacity
	if p, ok := byName["Rated Capacity"]; !ok {
		t.Error("missing Rated Capacity probe")
	} else {
		if p.Addr != 0x910B {
			t.Errorf("Rated Capacity Addr = 0x%04X, want 0x910B", p.Addr)
		}
		if p.Scale != 0.1 {
			t.Errorf("Rated Capacity Scale = %f, want 0.1", p.Scale)
		}
		if p.Unit != "Ah" {
			t.Errorf("Rated Capacity Unit = %q, want \"Ah\"", p.Unit)
		}
	}

	// Manufacturer
	if p, ok := byName["Manufacturer"]; !ok {
		t.Error("missing Manufacturer probe")
	} else {
		if p.Addr != 0x9106 {
			t.Errorf("Manufacturer Addr = 0x%04X, want 0x9106", p.Addr)
		}
		if p.Count != 4 {
			t.Errorf("Manufacturer Count = %d, want 4", p.Count)
		}
		if !p.IsASCII {
			t.Error("Manufacturer should be ASCII")
		}
	}

	// Alarm 2, Protection 2, Fault 2
	extProbes := map[string]uint16{
		"Alarm Status 2":      0x9124,
		"Protection Status 2": 0x9125,
		"Fault Status 2":      0x9126,
	}
	for name, wantAddr := range extProbes {
		p, ok := byName[name]
		if !ok {
			t.Errorf("missing %s probe", name)
			continue
		}
		if p.Addr != wantAddr {
			t.Errorf("%s Addr = 0x%04X, want 0x%04X", name, p.Addr, wantAddr)
		}
	}
}

func TestPackTemps58Probes(t *testing.T) {
	probes := PackTemps58Probes()
	if len(probes) != 4 {
		t.Fatalf("PackTemps58Probes returned %d probes, want 4", len(probes))
	}

	wantAddrs := []uint16{0x90BC, 0x90BD, 0x90BE, 0x90BF}
	for i, p := range probes {
		wantName := "Temp " + string(rune('5'+i))
		if p.Addr != wantAddrs[i] {
			t.Errorf("probe %d Addr = 0x%04X, want 0x%04X", i, p.Addr, wantAddrs[i])
		}
		if !p.Signed {
			t.Errorf("%s should be Signed", wantName)
		}
		if p.Scale != 0.1 {
			t.Errorf("%s Scale = %f, want 0.1", wantName, p.Scale)
		}
		if p.Unit != "\u00b0C" {
			t.Errorf("%s Unit = %q, want \"°C\"", wantName, p.Unit)
		}
	}
}

func TestBMSAlarmTable(t *testing.T) {
	if len(BMSAlarmBits) == 0 {
		t.Fatal("BMSAlarmBits is empty")
	}

	// Check for cell OV alarm at bit 0
	found := false
	for _, fb := range BMSAlarmBits {
		if fb.Addr == 0x9076 && fb.Bit == 0 {
			if !strings.Contains(fb.Desc, "Cell") || !strings.Contains(fb.Desc, "OV") {
				t.Errorf("bit 0 Desc = %q, want containing Cell and OV", fb.Desc)
			}
			found = true
		}
	}
	if !found {
		t.Error("missing BMSAlarmBits entry at Addr=0x9076 Bit=0")
	}

	// Check for cell UV alarm at bit 1
	found = false
	for _, fb := range BMSAlarmBits {
		if fb.Addr == 0x9076 && fb.Bit == 1 {
			if !strings.Contains(fb.Desc, "Cell") || !strings.Contains(fb.Desc, "UV") {
				t.Errorf("bit 1 Desc = %q, want containing Cell and UV", fb.Desc)
			}
			found = true
		}
	}
	if !found {
		t.Error("missing BMSAlarmBits entry at Addr=0x9076 Bit=1")
	}
}

func TestBMSProtectionTable(t *testing.T) {
	if len(BMSProtectionBits) == 0 {
		t.Fatal("BMSProtectionBits is empty")
	}

	// Check for cell OV protection at bit 0
	found := false
	for _, fb := range BMSProtectionBits {
		if fb.Addr == 0x9077 && fb.Bit == 0 {
			if !strings.Contains(fb.Desc, "Cell") || !strings.Contains(fb.Desc, "OV") {
				t.Errorf("bit 0 Desc = %q, want containing Cell and OV", fb.Desc)
			}
			found = true
		}
	}
	if !found {
		t.Error("missing BMSProtectionBits entry at Addr=0x9077 Bit=0")
	}
}

func TestBMSFaultTable_Pack(t *testing.T) {
	if len(BMSFaultBits) == 0 {
		t.Fatal("BMSFaultBits is empty")
	}

	// Check entries exist for 0x9078
	found := false
	for _, fb := range BMSFaultBits {
		if fb.Addr == 0x9078 {
			found = true
			break
		}
	}
	if !found {
		t.Error("missing BMSFaultBits entries for Addr=0x9078")
	}
}

func TestDecodeBalanceState(t *testing.T) {
	tests := []struct {
		val      uint16
		contains []string
		exact    string
	}{
		{0x0000, nil, "Balanced"},
		{0x0001, []string{"Cell 1"}, ""},
		{0x0005, []string{"Cell 1", "Cell 3"}, ""},
		{0xFFFF, []string{"Cell 1", "Cell 16"}, ""},
	}
	for _, tt := range tests {
		got := DecodeBalanceState(tt.val)
		if tt.exact != "" {
			if got != tt.exact {
				t.Errorf("DecodeBalanceState(0x%04X) = %q, want %q", tt.val, got, tt.exact)
			}
		}
		for _, sub := range tt.contains {
			if !strings.Contains(got, sub) {
				t.Errorf("DecodeBalanceState(0x%04X) = %q, missing %q", tt.val, got, sub)
			}
		}
	}
}

func TestFormatRawValue(t *testing.T) {
	tests := []struct {
		name string
		p    Probe
		data []byte
		want string
	}{
		{
			name: "Uint16",
			p:    Probe{Count: 1},
			data: []byte{0x0F, 0x0C},
			want: "3852",
		},
		{
			name: "Uint32",
			p:    Probe{U32: true, Count: 2},
			data: []byte{0x00, 0x01, 0x00, 0x00},
			want: "65536",
		},
		{
			name: "ASCII",
			p:    Probe{IsASCII: true},
			data: []byte{0x53, 0x4F},
			want: "534F",
		},
		{
			name: "EmptyData",
			p:    Probe{},
			data: []byte{},
			want: "",
		},
		{
			name: "SingleByte",
			p:    Probe{},
			data: []byte{0x01},
			want: "",
		},
		{
			name: "Signed",
			p:    Probe{Signed: true, Count: 1},
			data: []byte{0xFF, 0xFE},
			want: "65534",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatRawValue(tt.p, tt.data)
			if got != tt.want {
				t.Errorf("FormatRawValue(%s) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestDecodeBMSBitmap(t *testing.T) {
	// Use BMSAlarmBits for testing bitmap decoding
	// Bit 0 and bit 1 set at address 0x9076
	result := DecodeBMSBitmap(0x0003, BMSAlarmBits, 0x9076)
	if len(result) != 2 {
		t.Errorf("DecodeBMSBitmap(0x0003) returned %d entries, want 2", len(result))
	}

	// No bits set
	result = DecodeBMSBitmap(0x0000, BMSAlarmBits, 0x9076)
	if len(result) != 0 {
		t.Errorf("DecodeBMSBitmap(0x0000) returned %d entries, want 0", len(result))
	}
}

func TestPackProbeGroupOrder(t *testing.T) {
	groups := PackProbeGroups()

	// Exactly 5 groups
	if len(groups) != 5 {
		t.Fatalf("PackProbeGroups returned %d groups, want 5", len(groups))
	}

	// Group names in correct order (D-03)
	wantNames := []string{"Pack Info", "Cell Voltages", "Balance State", "Temperatures", "Pack Status"}
	for i, want := range wantNames {
		if groups[i].Name != want {
			t.Errorf("group[%d].Name = %q, want %q", i, groups[i].Name, want)
		}
	}

	// Group types
	wantTypes := []string{"", "cell_grid", "balance", "", "pack_status"}
	for i, want := range wantTypes {
		if groups[i].Type != want {
			t.Errorf("group[%d].Type = %q, want %q", i, groups[i].Type, want)
		}
	}

	// Cell Voltages group: 16 cell probes + Max Cell Voltage + Min Cell Voltage = 18 probes
	cellGroup := groups[1]
	if len(cellGroup.Probes) != 18 {
		t.Fatalf("Cell Voltages group has %d probes, want 18", len(cellGroup.Probes))
	}
	for i := 0; i < 16; i++ {
		wantName := fmt.Sprintf("Cell %d", i+1)
		if cellGroup.Probes[i].Name != wantName {
			t.Errorf("Cell Voltages probe[%d].Name = %q, want %q", i, cellGroup.Probes[i].Name, wantName)
		}
	}

	// Balance State group: probe at 0x9075
	balanceGroup := groups[2]
	if len(balanceGroup.Probes) != 1 {
		t.Fatalf("Balance State group has %d probes, want 1", len(balanceGroup.Probes))
	}
	if balanceGroup.Probes[0].Addr != 0x9075 {
		t.Errorf("Balance State probe[0].Addr = 0x%04X, want 0x9075", balanceGroup.Probes[0].Addr)
	}

	// Temperatures group: Temp 1-4 (0x906B-0x906E), MOS Temp (0x906F), Env Temp (0x9070), Temp 5-8 (0x90BC-0x90BF) = 10 probes
	tempGroup := groups[3]
	if len(tempGroup.Probes) != 10 {
		t.Fatalf("Temperatures group has %d probes, want 10", len(tempGroup.Probes))
	}
	wantTempAddrs := []uint16{0x906B, 0x906C, 0x906D, 0x906E, 0x906F, 0x9070, 0x90BC, 0x90BD, 0x90BE, 0x90BF}
	for i, wantAddr := range wantTempAddrs {
		if tempGroup.Probes[i].Addr != wantAddr {
			t.Errorf("Temperatures probe[%d].Addr = 0x%04X, want 0x%04X", i, tempGroup.Probes[i].Addr, wantAddr)
		}
	}

	// Pack Status group: 6 probes at 0x9076, 0x9077, 0x9078, 0x9124, 0x9125, 0x9126
	statusGroup := groups[4]
	if len(statusGroup.Probes) != 6 {
		t.Fatalf("Pack Status group has %d probes, want 6", len(statusGroup.Probes))
	}
	wantStatusAddrs := []uint16{0x9076, 0x9077, 0x9078, 0x9124, 0x9125, 0x9126}
	for i, wantAddr := range wantStatusAddrs {
		if statusGroup.Probes[i].Addr != wantAddr {
			t.Errorf("Pack Status probe[%d].Addr = 0x%04X, want 0x%04X", i, statusGroup.Probes[i].Addr, wantAddr)
		}
	}

	// Pack Info group: contains probes from both RT and Info blocks
	infoGroup := groups[0]
	wantInfoAddrs := map[uint16]bool{
		0x9044: true, // Pack ID
		0x9047: true, // Serial Number
		0x9079: true, // Total Voltage
		0x907A: true, // SOC
		0x9071: true, // Current
		0x9072: true, // Remaining Capacity
		0x9073: true, // Full Charge Capacity
		0x9074: true, // Cycle Count
		0x907B: true, // Total Packs
		0x907C: true, // Cell Count
		0x9104: true, // Balanced Bus Voltage
		0x9105: true, // Balanced Bus Current
		0x9106: true, // Manufacturer
		0x910A: true, // SOH
		0x910B: true, // Rated Capacity
	}
	gotInfoAddrs := make(map[uint16]bool)
	for _, p := range infoGroup.Probes {
		gotInfoAddrs[p.Addr] = true
	}
	for addr := range wantInfoAddrs {
		if !gotInfoAddrs[addr] {
			t.Errorf("Pack Info group missing probe at addr 0x%04X", addr)
		}
	}
	if len(infoGroup.Probes) != len(wantInfoAddrs) {
		t.Errorf("Pack Info group has %d probes, want %d", len(infoGroup.Probes), len(wantInfoAddrs))
	}
}

// === Configuration Enum Maps Tests ===

func TestConfigEnumMaps(t *testing.T) {
	// All enum maps must be non-nil and have at least 2 entries
	allEnums := map[string]map[uint16]string{
		"BaudRateEnum":              BaudRateEnum,
		"PVInputModeEnum":           PVInputModeEnum,
		"AntiBackFlowEnum":          AntiBackFlowEnum,
		"EPSControlEnum":            EPSControlEnum,
		"LanguageEnum":              LanguageEnum,
		"ProhibitEnableEnum":        ProhibitEnableEnum,
		"BatteryProtocolEnum":       BatteryProtocolEnum,
		"CellTypeEnum":              CellTypeEnum,
		"BatteryUsageEnum":          BatteryUsageEnum,
		"EnergyStorageModeEnum":     EnergyStorageModeEnum,
		"DRMSEnum":                  DRMSEnum,
		"ParallelModeEnum":          ParallelModeEnum,
		"GridDetectionEnum":         GridDetectionEnum,
		"DryContactEnum":            DryContactEnum,
		"SafetyCountryEnum":         SafetyCountryEnum,
		"RemoteOnOffEnum":           RemoteOnOffEnum,
		"PowerControlEnum":          PowerControlEnum,
		"EMSTimePeriodModeEnum":     EMSTimePeriodModeEnum,
		"ChargeDischargeControlEnum": ChargeDischargeControlEnum,
		"ChargingSourceEnum":        ChargingSourceEnum,
		"ProtectionEnableEnum":      ProtectionEnableEnum,
		"ReactiveControlModeEnum":   ReactiveControlModeEnum,
		"EnableStatusEnum":          EnableStatusEnum,
		"InputChannelTypeEnum":      InputChannelTypeEnum,
	}

	for name, enum := range allEnums {
		if enum == nil {
			t.Errorf("%s is nil", name)
			continue
		}
		if len(enum) < 2 {
			t.Errorf("%s has %d entries, want >= 2", name, len(enum))
		}
	}

	// Spot-check specific key-value pairs
	spotChecks := []struct {
		name string
		enum map[uint16]string
		key  uint16
		want string
	}{
		{"BaudRateEnum", BaudRateEnum, 1, "9600 bps"},
		{"BaudRateEnum", BaudRateEnum, 5, "115200 bps"},
		{"CellTypeEnum", CellTypeEnum, 1, "Lithium iron phosphate"},
		{"BatteryProtocolEnum", BatteryProtocolEnum, 0, "Sofar BMS/DEFAULT"},
		{"EnergyStorageModeEnum", EnergyStorageModeEnum, 0, "Self-generation"},
		{"EnergyStorageModeEnum", EnergyStorageModeEnum, 5, "Off-grid"},
		{"LanguageEnum", LanguageEnum, 1, "English"},
		{"ProhibitEnableEnum", ProhibitEnableEnum, 0, "Disabled"},
		{"ProhibitEnableEnum", ProhibitEnableEnum, 1, "Enabled"},
	}

	for _, sc := range spotChecks {
		got, ok := sc.enum[sc.key]
		if !ok {
			t.Errorf("%s[%d] not found", sc.name, sc.key)
			continue
		}
		if got != sc.want {
			t.Errorf("%s[%d] = %q, want %q", sc.name, sc.key, got, sc.want)
		}
	}
}

// === Configuration Groups Tests ===

func TestConfigurationGroups(t *testing.T) {
	groups := ConfigurationGroups

	// Must be non-nil with >= 15 groups
	if groups == nil {
		t.Fatal("ConfigurationGroups is nil")
	}
	if len(groups) < 15 {
		t.Errorf("ConfigurationGroups has %d groups, want >= 15", len(groups))
	}

	// First group should be "System Config"
	if groups[0].Name != "System Config" {
		t.Errorf("First group name = %q, want %q", groups[0].Name, "System Config")
	}
	// First probe should be in 0x1000 range
	if len(groups[0].Probes) > 0 && groups[0].Probes[0].Addr < 0x1000 {
		t.Errorf("First group first probe addr = 0x%04X, want >= 0x1000", groups[0].Probes[0].Addr)
	}

	// Find specific groups by name
	groupMap := make(map[string]*ProbeGroup)
	for i := range groups {
		groupMap[groups[i].Name] = &groups[i]
	}

	// "Battery Config" group with probe at 0x1046 referencing BatteryProtocolEnum
	battConfig, ok := groupMap["Battery Config"]
	if !ok {
		t.Error("Missing 'Battery Config' group")
	} else {
		found := false
		for _, p := range battConfig.Probes {
			if p.Addr == 0x1046 {
				found = true
				if p.Enum == nil {
					t.Error("Battery Config probe at 0x1046 has nil Enum")
				}
				break
			}
		}
		if !found {
			t.Error("Battery Config missing probe at 0x1046")
		}
	}

	// "Energy Storage Mode" group with probe at 0x1110
	esMode, ok := groupMap["Energy Storage Mode"]
	if !ok {
		t.Error("Missing 'Energy Storage Mode' group")
	} else {
		found := false
		for _, p := range esMode.Probes {
			if p.Addr == 0x1110 {
				found = true
				if p.Enum == nil {
					t.Error("Energy Storage Mode probe at 0x1110 has nil Enum")
				}
				break
			}
		}
		if !found {
			t.Error("Energy Storage Mode missing probe at 0x1110")
		}
	}

	// No duplicate addresses across all groups
	addrSet := make(map[uint16]string)
	totalProbes := 0
	for _, g := range groups {
		for _, p := range g.Probes {
			totalProbes++
			if prev, exists := addrSet[p.Addr]; exists {
				t.Errorf("Duplicate address 0x%04X in group %q (first seen in %q)", p.Addr, g.Name, prev)
			}
			addrSet[p.Addr] = g.Name
		}
	}

	// All probes have Count >= 1
	for _, g := range groups {
		for _, p := range g.Probes {
			if p.Count < 1 {
				t.Errorf("Probe %q in group %q has Count %d, want >= 1", p.Name, g.Name, p.Count)
			}
		}
	}

	// All U32 probes have Count == 2
	for _, g := range groups {
		for _, p := range g.Probes {
			if p.U32 && p.Count != 2 {
				t.Errorf("U32 probe %q in group %q has Count %d, want 2", p.Name, g.Name, p.Count)
			}
		}
	}

	// Total probe count >= 150
	if totalProbes < 150 {
		t.Errorf("Total probe count = %d, want >= 150", totalProbes)
	}

	// 6 EMS Time Period groups exist
	emsCount := 0
	for _, g := range groups {
		if strings.Contains(g.Name, "EMS Time Period") && !strings.Contains(g.Name, "Enable") {
			emsCount++
		}
	}
	if emsCount != 6 {
		t.Errorf("EMS Time Period group count = %d, want 6", emsCount)
	}

	// At least 5 safety groups exist ("Safety:" prefix)
	safetyCount := 0
	for _, g := range groups {
		if strings.HasPrefix(g.Name, "Safety:") {
			safetyCount++
		}
	}
	if safetyCount < 5 {
		t.Errorf("Safety group count = %d, want >= 5", safetyCount)
	}
}
