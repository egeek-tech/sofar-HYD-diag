package register

import (
	"encoding/binary"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatValueASCII(t *testing.T) {
	p := Probe{Name: "Test SN", Addr: 0x0445, Count: 10, IsASCII: true}
	data := []byte("AMASS\x00\x00\x00")
	got := FormatValue(p, data)
	assert.Equal(t, "AMASS", got)
}

func TestFormatValueUnsignedScaled(t *testing.T) {
	p := Probe{Name: "Voltage", Scale: 0.1, Unit: "V"}
	// Encode 5288 as big-endian uint16
	data := make([]byte, 2)
	binary.BigEndian.PutUint16(data, 5288)
	got := FormatValue(p, data)
	assert.Equal(t, "528.80 V", got)
}

func TestFormatValueSignedNegative(t *testing.T) {
	p := Probe{Name: "Power", Signed: true, Scale: 0.01, Unit: "kW"}
	// Encode -83 as big-endian int16 (two's complement)
	data := make([]byte, 2)
	neg83 := int16(-83)
	binary.BigEndian.PutUint16(data, uint16(neg83))
	got := FormatValue(p, data)
	assert.Equal(t, "-0.83 kW", got)
}

func TestFormatValueNoScale(t *testing.T) {
	p := Probe{Name: "State"}
	// Encode 164 as big-endian uint16
	data := make([]byte, 2)
	binary.BigEndian.PutUint16(data, 164)
	got := FormatValue(p, data)
	assert.Equal(t, "164 (0x00A4)", got)
}

func TestFormatValueShortData(t *testing.T) {
	p := Probe{Name: "Short"}
	data := []byte{0x01}
	got := FormatValue(p, data)
	assert.Equal(t, "<no data>", got)
}

func TestFormatValueSignedNoUnit(t *testing.T) {
	p := Probe{Name: "Raw signed", Signed: true}
	data := make([]byte, 2)
	neg42 := int16(-42)
	binary.BigEndian.PutUint16(data, uint16(neg42))
	got := FormatValue(p, data)
	assert.Equal(t, "-42", got)
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
	assert.Equal(t, "Test Group", pg.Name)
	assert.Len(t, pg.Probes, 1)
	assert.Equal(t, "column", pg.Layout)
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
		if !assert.True(t, ok, "RunningStateEnum[%d] not found", tt.val) {
			continue
		}
		assert.Equal(t, tt.want, got, "RunningStateEnum[%d]", tt.val)
	}
	// Verify all 12 entries exist (0-11)
	assert.Len(t, RunningStateEnum, 12)
}

func TestSystemGroups(t *testing.T) {
	require.Len(t, SystemGroups, 6)

	expectedNames := []string{"Identity", "Firmware", "Status", "Firmware (Extended)", "Temperatures", "Protection"}
	for i, want := range expectedNames {
		assert.Equal(t, want, SystemGroups[i].Name, "SystemGroups[%d].Name", i)
	}

	// Identity: Inverter SN at 0x0445, Count 10, IsASCII
	identity := SystemGroups[0]
	require.NotEmpty(t, identity.Probes, "Identity group has no probes")
	assert.Equal(t, uint16(0x0445), identity.Probes[0].Addr, "Identity SN addr")
	assert.Equal(t, uint16(10), identity.Probes[0].Count, "Identity SN count")
	assert.True(t, identity.Probes[0].IsASCII, "Identity SN IsASCII should be true")

	// Firmware: 5 probes
	firmware := SystemGroups[1]
	require.Len(t, firmware.Probes, 5, "Firmware group probes")
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
		assert.Equal(t, fw.name, firmware.Probes[i].Name, "Firmware[%d].Name", i)
		assert.Equal(t, fw.addr, firmware.Probes[i].Addr, "Firmware[%d].Addr", i)
		assert.Equal(t, fw.count, firmware.Probes[i].Count, "Firmware[%d].Count", i)
	}

	// Status: Running state with Enum at 0x0404, grid wait time, power gen time, plus synthetic System time
	status := SystemGroups[2]
	assert.Equal(t, uint16(0x0404), status.Probes[0].Addr, "Status running state addr")
	assert.NotNil(t, status.Probes[0].Enum, "Status running state Enum should not be nil")
	assert.Len(t, status.Probes, 4, "Status probes (running state + wait time + gen time + System time)")
	assert.Equal(t, "System time", status.Probes[3].Name, "Status[3].Name")
	assert.Equal(t, uint16(0x042C), status.Probes[3].Addr, "Status[3].Addr")
	assert.Equal(t, uint16(6), status.Probes[3].Count, "Status[3].Count (Composite system_time probe)")
	assert.Equal(t, "system_time", status.Probes[3].Composite, "Status[3].Composite")

	// Firmware (Extended): BOOT versions + safety versions
	fwExt := SystemGroups[3]
	assert.Equal(t, "Firmware (Extended)", fwExt.Name)

	// Temperatures: 4 probes, all S16
	temps := SystemGroups[4]
	require.Len(t, temps.Probes, 4, "Temperatures probes")
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
		assert.Equal(t, te.name, temps.Probes[i].Name, "Temps[%d].Name", i)
		assert.Equal(t, te.addr, temps.Probes[i].Addr, "Temps[%d].Addr", i)
		assert.True(t, temps.Probes[i].Signed, "Temps[%d].Signed should be true", i)
	}

	// Protection: Insulation impedance (0x042B) and Fan speed (0x043E)
	protection := SystemGroups[5]
	require.Len(t, protection.Probes, 2, "Protection probes")
	assert.Equal(t, uint16(0x042B), protection.Probes[0].Addr, "Protection[0].Addr")
	assert.Equal(t, uint16(0x043E), protection.Probes[1].Addr, "Protection[1].Addr")
}

func TestGridGroups(t *testing.T) {
	require.Len(t, GridGroups, 7)

	expectedNames := []string{"General", "Phase R", "Phase S", "Phase T", "PCC Power", "Line Voltages", "Load"}
	for i, want := range expectedNames {
		assert.Equal(t, want, GridGroups[i].Name, "GridGroups[%d].Name", i)
	}

	// Phase R: column layout, 5 probes
	phaseR := GridGroups[1]
	assert.Equal(t, "column", phaseR.Layout, "Phase R layout")
	require.Len(t, phaseR.Probes, 5, "Phase R probes")
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
		assert.Equal(t, re.name, phaseR.Probes[i].Name, "Phase R[%d].Name", i)
		assert.Equal(t, re.addr, phaseR.Probes[i].Addr, "Phase R[%d].Addr", i)
		assert.Equal(t, re.scale, phaseR.Probes[i].Scale, "Phase R[%d].Scale", i)
	}
	// Power factor has no Unit
	assert.Empty(t, phaseR.Probes[4].Unit, "Phase R power factor Unit")

	// Phase S: column layout, Voltage at 0x0498
	phaseS := GridGroups[2]
	assert.Equal(t, "column", phaseS.Layout, "Phase S layout")
	assert.Equal(t, uint16(0x0498), phaseS.Probes[0].Addr, "Phase S voltage addr")
	assert.Equal(t, uint16(0x0499), phaseS.Probes[1].Addr, "Phase S current addr")
	assert.Equal(t, uint16(0x049A), phaseS.Probes[2].Addr, "Phase S active power addr")
	assert.Equal(t, uint16(0x049B), phaseS.Probes[3].Addr, "Phase S reactive power addr")
	assert.Equal(t, uint16(0x049C), phaseS.Probes[4].Addr, "Phase S power factor addr")

	// Phase T: Voltage at 0x04A3
	phaseT := GridGroups[3]
	assert.Equal(t, "column", phaseT.Layout, "Phase T layout")
	assert.Equal(t, uint16(0x04A3), phaseT.Probes[0].Addr, "Phase T voltage addr")

	// Load: External power gen (0x04AE), Total load power (0x04AF), Total power factor (0x04BD S16 Scale 0.001), Generation efficiency (0x04BF)
	load := GridGroups[6]
	require.Len(t, load.Probes, 4, "Load probes")
	assert.Equal(t, uint16(0x04AE), load.Probes[0].Addr, "Load external power gen addr")
	assert.Equal(t, uint16(0x04AF), load.Probes[1].Addr, "Load total load power addr")
	assert.Equal(t, uint16(0x04BD), load.Probes[2].Addr, "Load total power factor addr")
	assert.True(t, load.Probes[2].Signed, "Load total power factor should be signed")
	assert.Equal(t, 0.001, load.Probes[2].Scale, "Load total power factor scale")
	assert.Equal(t, uint16(0x04BF), load.Probes[3].Addr, "Load generation efficiency addr")
}

func TestEPSGroups(t *testing.T) {
	require.Len(t, EPSGroups, 5)

	expectedNames := []string{"General", "Phase R", "Phase S", "Phase T", "Emergency Load"}
	for i, want := range expectedNames {
		assert.Equal(t, want, EPSGroups[i].Name, "EPSGroups[%d].Name", i)
	}

	// General: 4 probes
	general := EPSGroups[0]
	require.Len(t, general.Probes, 4, "EPS General probes")
	assert.Equal(t, uint16(0x0504), general.Probes[0].Addr, "EPS load active power addr")
	assert.Equal(t, uint16(0x0505), general.Probes[1].Addr, "EPS load reactive power addr")
	assert.Equal(t, uint16(0x0506), general.Probes[2].Addr, "EPS load apparent power addr")
	assert.Equal(t, uint16(0x0507), general.Probes[3].Addr, "EPS output freq addr")

	// Phase R: column layout, 2 probes
	phaseR := EPSGroups[1]
	assert.Equal(t, "column", phaseR.Layout, "EPS Phase R layout")
	assert.Equal(t, uint16(0x050A), phaseR.Probes[0].Addr, "EPS Phase R output voltage addr")
	assert.Equal(t, uint16(0x050B), phaseR.Probes[1].Addr, "EPS Phase R load current addr")
	assert.True(t, phaseR.Probes[1].Signed, "EPS Phase R load current should be signed")

	// Phase S: 0x0512/0x0513
	phaseS := EPSGroups[2]
	assert.Equal(t, "column", phaseS.Layout, "EPS Phase S layout")
	assert.Equal(t, uint16(0x0512), phaseS.Probes[0].Addr, "EPS Phase S output voltage addr")
	assert.Equal(t, uint16(0x0513), phaseS.Probes[1].Addr, "EPS Phase S load current addr")

	// Phase T: 0x051A/0x051B
	phaseT := EPSGroups[3]
	assert.Equal(t, uint16(0x051A), phaseT.Probes[0].Addr, "EPS Phase T output voltage addr")
	assert.Equal(t, uint16(0x051B), phaseT.Probes[1].Addr, "EPS Phase T load current addr")

	// Emergency Load: voltages at 0x0510, 0x0518, 0x0520
	emerg := EPSGroups[4]
	require.Len(t, emerg.Probes, 3, "Emergency Load probes")
	assert.Equal(t, uint16(0x0510), emerg.Probes[0].Addr, "Emergency Load R addr")
	assert.Equal(t, uint16(0x0518), emerg.Probes[1].Addr, "Emergency Load S addr")
	assert.Equal(t, uint16(0x0520), emerg.Probes[2].Addr, "Emergency Load T addr")
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
	assert.Equal(t, "Grid-connected", got, "FormatValue enum value 2")

	// Unknown value falls back to raw format
	binary.BigEndian.PutUint16(data, 99)
	got = FormatValue(p, data)
	assert.Equal(t, "99 (0x0063)", got, "FormatValue enum unknown value 99")
}

func TestFormatValueScaleNoUnit(t *testing.T) {
	// Power factor: Scale 0.001, no unit, value 990 -> "0.990"
	p := Probe{Name: "Power factor", Signed: true, Scale: 0.001}
	data := make([]byte, 2)
	binary.BigEndian.PutUint16(data, uint16(int16(990)))
	got := FormatValue(p, data)
	assert.Equal(t, "0.990", got)
}

func TestComposeSystemTime(t *testing.T) {
	got := ComposeSystemTime(26, 4, 10, 14, 30, 5)
	assert.Equal(t, "14:30:05 10-04-2026", got)

	got = ComposeSystemTime(0, 1, 1, 0, 0, 0)
	assert.Equal(t, "00:00:00 01-01-2000", got)
}

func TestFormatValueComposite(t *testing.T) {
	p := Probe{Name: "System time", Addr: 0x042C, Count: 6, Composite: "system_time"}
	data := make([]byte, 12)
	binary.BigEndian.PutUint16(data[0:2], 26)  // year
	binary.BigEndian.PutUint16(data[2:4], 4)   // month
	binary.BigEndian.PutUint16(data[4:6], 14)  // day
	binary.BigEndian.PutUint16(data[6:8], 10)  // hour
	binary.BigEndian.PutUint16(data[8:10], 30) // min
	binary.BigEndian.PutUint16(data[10:12], 45) // sec
	got := FormatValue(p, data)
	assert.Equal(t, "10:30:45 14-04-2026", got)
}

func TestFormatValueCompositeNoData(t *testing.T) {
	p := Probe{Name: "System time", Addr: 0x042C, Count: 6, Composite: "system_time"}
	got := FormatValue(p, []byte{0x00, 0x01})
	assert.Equal(t, "<no data>", got)
}

func TestFormatRawValueComposite(t *testing.T) {
	p := Probe{Name: "System time", Addr: 0x042C, Count: 6, Composite: "system_time"}
	data := make([]byte, 12)
	binary.BigEndian.PutUint16(data[0:2], 26)
	binary.BigEndian.PutUint16(data[2:4], 4)
	binary.BigEndian.PutUint16(data[4:6], 14)
	binary.BigEndian.PutUint16(data[6:8], 10)
	binary.BigEndian.PutUint16(data[8:10], 30)
	binary.BigEndian.PutUint16(data[10:12], 45)
	got := FormatRawValue(p, data)
	assert.Equal(t, "0x042C-0x0431 | 26, 4, 14, 10, 30, 45", got)
}

func TestFormatValueBMSClock(t *testing.T) {
	p := Probe{Name: "System Clock", Addr: 0x9004, Count: 2, Composite: "bms_clock"}
	data := make([]byte, 4)
	binary.BigEndian.PutUint16(data[0:2], 0x6914)
	binary.BigEndian.PutUint16(data[2:4], 0xE0C5)
	got := FormatValue(p, data)
	assert.Equal(t, "2026-04-10 14:03:05", got)
}

func TestFormatValueBMSClockNoData(t *testing.T) {
	p := Probe{Name: "System Clock", Addr: 0x9004, Count: 2, Composite: "bms_clock"}
	got := FormatValue(p, []byte{0x00, 0x01})
	assert.Equal(t, "<no data>", got)
}

func TestFormatValueBMSSWVersion(t *testing.T) {
	p := Probe{Name: "SW Version", Addr: 0x9018, Count: 4, Composite: "bms_sw_version"}
	data := []byte{0x00, 0x56, 0x00, 0x01, 0x00, 0x02, 0x00, 0x03}
	got := FormatValue(p, data)
	assert.Equal(t, "V1.2.3", got)
}

func TestFormatValueBMSSWVersionNoData(t *testing.T) {
	p := Probe{Name: "SW Version", Addr: 0x9018, Count: 4, Composite: "bms_sw_version"}
	got := FormatValue(p, []byte{0x00, 0x56, 0x00, 0x01})
	assert.Equal(t, "<no data>", got)
}

func TestFormatRawValueBMSClock(t *testing.T) {
	p := Probe{Name: "System Clock", Addr: 0x9004, Count: 2, Composite: "bms_clock"}
	data := make([]byte, 4)
	binary.BigEndian.PutUint16(data[0:2], 0x6914)
	binary.BigEndian.PutUint16(data[2:4], 0xE0C5)
	got := FormatRawValue(p, data)
	assert.Equal(t, "0x9004-0x9005 | 26900, 57541", got)
}

func TestFormatRawValueBMSSWVersion(t *testing.T) {
	p := Probe{Name: "SW Version", Addr: 0x9018, Count: 4, Composite: "bms_sw_version"}
	data := []byte{0x00, 0x56, 0x00, 0x01, 0x00, 0x02, 0x00, 0x03}
	got := FormatRawValue(p, data)
	assert.Equal(t, "0x9018-0x901B | 86, 1, 2, 3", got)
}

// === Task 2 TDD tests: Fault bitmap decoder ===

func TestFaultBitStruct(t *testing.T) {
	fb := FaultBit{Addr: 0x0405, Bit: 0, Desc: "Grid over-voltage"}
	assert.Equal(t, uint16(0x0405), fb.Addr)
	assert.Equal(t, uint8(0), fb.Bit)
	assert.Equal(t, "Grid over-voltage", fb.Desc)
}

func TestFaultTableSize(t *testing.T) {
	assert.Greater(t, len(FaultTable), 200, "FaultTable len")
}

func TestFaultTableFirstEntry(t *testing.T) {
	require.NotEmpty(t, FaultTable, "FaultTable is empty")
	first := FaultTable[0]
	assert.Equal(t, uint16(0x0405), first.Addr)
	assert.Equal(t, uint8(0), first.Bit)
	assert.Equal(t, "Grid over-voltage", first.Desc)
}

func TestFaultTableLeakageCurrent(t *testing.T) {
	found := false
	for _, fb := range FaultTable {
		if fb.Addr == 0x0405 && fb.Bit == 4 && fb.Desc == "Leakage current faults" {
			found = true
			break
		}
	}
	assert.True(t, found, "FaultTable missing entry for 0x0405 bit 4 'Leakage current faults'")
}

func TestFaultRegisters(t *testing.T) {
	require.Len(t, FaultRegisters, 2)
	assert.Equal(t, uint16(0x0405), FaultRegisters[0].Addr, "FaultRegisters[0].Addr")
	assert.Equal(t, uint16(18), FaultRegisters[0].Count, "FaultRegisters[0].Count")
	assert.Equal(t, uint16(0x0432), FaultRegisters[1].Addr, "FaultRegisters[1].Addr")
	assert.Equal(t, uint16(12), FaultRegisters[1].Count, "FaultRegisters[1].Count")
}

func TestDecodeFaultsEmpty(t *testing.T) {
	faultData := map[uint16]uint16{
		0x0405: 0x0000,
		0x0406: 0x0000,
	}
	faults := DecodeFaults(faultData)
	assert.Empty(t, faults, "DecodeFaults all zeros")
}

func TestDecodeFaultsSingleBit(t *testing.T) {
	faultData := map[uint16]uint16{
		0x0405: 0x0001, // bit 0 set
	}
	faults := DecodeFaults(faultData)
	require.Len(t, faults, 1)
	assert.Equal(t, "Grid over-voltage", faults[0])
}

func TestDecodeFaultsMultipleBits(t *testing.T) {
	faultData := map[uint16]uint16{
		0x0405: 0x0003, // bits 0+1 set
	}
	faults := DecodeFaults(faultData)
	require.Len(t, faults, 2)
	assert.Equal(t, "Grid over-voltage", faults[0])
	assert.Equal(t, "Grid under-voltage", faults[1])
}

func TestDecodeFaultsUnknownRegister(t *testing.T) {
	faultData := map[uint16]uint16{
		0xFFFF: 0xFFFF, // register not in table
	}
	faults := DecodeFaults(faultData)
	assert.Empty(t, faults, "DecodeFaults unknown register")
}

// === Task 2 TDD tests: Dynamic PV group generator ===

func TestGeneratePVGroups2(t *testing.T) {
	groups := GeneratePVGroups(2)
	require.Len(t, groups, 3)

	// PV 1
	assert.Equal(t, "PV 1", groups[0].Name)
	assert.Equal(t, "column", groups[0].Layout)
	require.Len(t, groups[0].Probes, 3, "PV 1 probes")
	assert.Equal(t, uint16(0x0584), groups[0].Probes[0].Addr, "PV 1 voltage addr")
	assert.Equal(t, 0.1, groups[0].Probes[0].Scale, "PV 1 voltage scale")
	assert.Equal(t, "V", groups[0].Probes[0].Unit, "PV 1 voltage unit")
	assert.Equal(t, uint16(0x0585), groups[0].Probes[1].Addr, "PV 1 current addr")
	assert.True(t, groups[0].Probes[1].Signed, "PV 1 current should be signed")
	assert.Equal(t, 0.01, groups[0].Probes[1].Scale, "PV 1 current scale")
	assert.Equal(t, "A", groups[0].Probes[1].Unit, "PV 1 current unit")
	assert.Equal(t, uint16(0x0586), groups[0].Probes[2].Addr, "PV 1 power addr")
	assert.Equal(t, 0.01, groups[0].Probes[2].Scale, "PV 1 power scale")
	assert.Equal(t, "kW", groups[0].Probes[2].Unit, "PV 1 power unit")

	// PV 2
	assert.Equal(t, "PV 2", groups[1].Name)
	assert.Equal(t, uint16(0x0587), groups[1].Probes[0].Addr, "PV 2 voltage addr")
	assert.Equal(t, uint16(0x0588), groups[1].Probes[1].Addr, "PV 2 current addr")
	assert.Equal(t, uint16(0x0589), groups[1].Probes[2].Addr, "PV 2 power addr")

	// Total PV Power
	assert.Equal(t, "Total PV Power", groups[2].Name)
	assert.Empty(t, groups[2].Layout, "Total PV Power layout")
}

func TestGeneratePVGroups16(t *testing.T) {
	groups := GeneratePVGroups(16)
	require.Len(t, groups, 17)
	// PV 16 voltage at 0x05B1
	pv16 := groups[15]
	assert.Equal(t, "PV 16", pv16.Name)
	assert.Equal(t, uint16(0x05B1), pv16.Probes[0].Addr, "PV 16 voltage addr")
}

func TestGeneratePVGroupsTotalPower(t *testing.T) {
	groups := GeneratePVGroups(4)
	total := groups[len(groups)-1]
	assert.Equal(t, "Total PV Power", total.Name)
	require.Len(t, total.Probes, 1, "Total PV Power probes")
	assert.Equal(t, uint16(0x05C4), total.Probes[0].Addr, "Total PV Power addr")
	assert.Equal(t, 0.1, total.Probes[0].Scale, "Total PV Power scale")
	assert.Equal(t, "kW", total.Probes[0].Unit, "Total PV Power unit")
}

func TestGeneratePVGroupsColumnLayout(t *testing.T) {
	groups := GeneratePVGroups(3)
	for i := 0; i < 3; i++ {
		assert.Equal(t, "column", groups[i].Layout, "PV %d layout", i+1)
	}
	assert.Empty(t, groups[3].Layout, "Total PV Power layout")
}

// === Phase 04 Task 1 TDD tests: U32, BatteryStateEnum, GenerateBatteryGroups ===

func TestFormatValueU32(t *testing.T) {
	p := Probe{Name: "Energy", U32: true, Count: 2, Scale: 0.01, Unit: "kWh"}
	// Encode 23900: hi_word=0, lo_word=23900
	data := make([]byte, 4)
	binary.BigEndian.PutUint16(data[:2], 0)     // high word
	binary.BigEndian.PutUint16(data[2:4], 23900) // low word
	got := FormatValue(p, data)
	assert.Equal(t, "239.00 kWh", got)
}

func TestFormatValueU32Large(t *testing.T) {
	p := Probe{Name: "Energy", U32: true, Count: 2, Scale: 0.1, Unit: "kWh"}
	// Encode 1234567: hi_word=18 (1234567 >> 16 = 18), lo_word=54919 (1234567 & 0xFFFF = 54919)
	data := make([]byte, 4)
	binary.BigEndian.PutUint16(data[:2], 18)
	binary.BigEndian.PutUint16(data[2:4], 54919)
	got := FormatValue(p, data)
	assert.Equal(t, "123456.70 kWh", got)
}

func TestFormatValueU32NoScale(t *testing.T) {
	p := Probe{Name: "Raw", U32: true, Count: 2}
	// Encode 42: hi_word=0, lo_word=42
	data := make([]byte, 4)
	binary.BigEndian.PutUint16(data[:2], 0)
	binary.BigEndian.PutUint16(data[2:4], 42)
	got := FormatValue(p, data)
	assert.Equal(t, "42 (0x0000002A)", got)
}

func TestFormatValueU32Signed(t *testing.T) {
	p := Probe{Name: "Signed32", U32: true, Signed: true, Scale: 0.01, Unit: "W"}
	// Encode -100 as uint32: 0xFFFFFF9C
	data := make([]byte, 4)
	binary.BigEndian.PutUint16(data[:2], 0xFFFF)
	binary.BigEndian.PutUint16(data[2:4], 0xFF9C)
	got := FormatValue(p, data)
	assert.Equal(t, "-1.00 W", got)
}

func TestFormatValueU32ShortData(t *testing.T) {
	p := Probe{Name: "Short", U32: true}
	data := []byte{0x00, 0x01} // only 2 bytes, need 4
	got := FormatValue(p, data)
	assert.Equal(t, "<no data>", got)
}

func TestBatteryStateEnum(t *testing.T) {
	expected := map[uint16]string{
		1: "Charging",
		2: "Discharging",
		3: "Sleeping",
		4: "Fault",
		5: "Loss reduction",
	}
	assert.Len(t, BatteryStateEnum, 5)
	for k, v := range expected {
		got, ok := BatteryStateEnum[k]
		if !assert.True(t, ok, "BatteryStateEnum[%d] not found", k) {
			continue
		}
		assert.Equal(t, v, got, "BatteryStateEnum[%d]", k)
	}
}

func TestGenerateBatteryGroups2(t *testing.T) {
	groups := GenerateBatteryGroups(2)
	// 2 channel groups + 1 global stats = 3
	require.Len(t, groups, 3)

	// Channel 1
	ch1 := groups[0]
	assert.Equal(t, "Channel 1", ch1.Name)
	assert.Equal(t, "column", ch1.Layout)
	require.Len(t, ch1.Probes, 10, "Channel 1 probes")
	// First 7 probes: pack info at base 0x0604
	assert.Equal(t, uint16(0x0604), ch1.Probes[0].Addr, "Ch1 voltage addr")

	// Channel 2
	ch2 := groups[1]
	assert.Equal(t, "Channel 2", ch2.Name)
	require.Len(t, ch2.Probes, 10, "Channel 2 probes")
	// Channel 2 base = 0x0604 + 7*(2-1) = 0x060B
	assert.Equal(t, uint16(0x060B), ch2.Probes[0].Addr, "Ch2 voltage addr")

	// Each channel has 10 probes (7 pack info + charge limit + discharge limit + state)
	// State probe should have BatteryStateEnum
	stateProbe := ch1.Probes[9]
	assert.NotNil(t, stateProbe.Enum, "Channel 1 state probe Enum should not be nil")

	// Global Stats
	global := groups[2]
	assert.Equal(t, "Global Stats", global.Name)
	assert.Empty(t, global.Layout, "Global Stats layout")
	require.Len(t, global.Probes, 5, "Global Stats probes")
	assert.Equal(t, uint16(0x0667), global.Probes[0].Addr, "Global Stats charge/discharge addr")
	assert.Equal(t, uint16(0x066B), global.Probes[4].Addr, "Global Stats total capacity addr")
}

func TestGenerateBatteryGroups1(t *testing.T) {
	groups := GenerateBatteryGroups(1)
	// 1 channel group + 1 global stats = 2
	require.Len(t, groups, 2)
	// State probe should have BatteryStateEnum
	ch1 := groups[0]
	stateProbe := ch1.Probes[9]
	assert.NotNil(t, stateProbe.Enum, "Channel 1 state probe Enum should not be nil")
	_, ok := stateProbe.Enum[1]
	assert.True(t, ok, "State probe Enum missing key 1 (Charging)")
}

func TestGenerateBatteryGroupsAddresses(t *testing.T) {
	groups := GenerateBatteryGroups(2)
	ch1 := groups[0]
	ch2 := groups[1]

	// Channel 1 pack info: voltage=0x0604
	assert.Equal(t, uint16(0x0604), ch1.Probes[0].Addr, "Ch1 voltage")
	// Channel 2 pack info: voltage=0x060B
	assert.Equal(t, uint16(0x060B), ch2.Probes[0].Addr, "Ch2 voltage")
	// Channel 1 charge limit: 0x0644
	assert.Equal(t, uint16(0x0644), ch1.Probes[7].Addr, "Ch1 charge limit")
	// Channel 1 discharge limit: 0x0645
	assert.Equal(t, uint16(0x0645), ch1.Probes[8].Addr, "Ch1 discharge limit")
	// Channel 1 state: 0x0646
	assert.Equal(t, uint16(0x0646), ch1.Probes[9].Addr, "Ch1 state")
	// Channel 2 charge limit: 0x0648
	assert.Equal(t, uint16(0x0648), ch2.Probes[7].Addr, "Ch2 charge limit")
	// Channel 2 state: 0x064A
	assert.Equal(t, uint16(0x064A), ch2.Probes[9].Addr, "Ch2 state")
}

// === Phase 04 Task 2 TDD tests: ProbeGroup Type, BMSInfoGroups, BMSProtectionProbes, StatisticsGroups, DecodeBMSClock, DecodeTopology ===

func TestProbeGroupType(t *testing.T) {
	pg := ProbeGroup{Name: "Protection", Type: "bitmap"}
	assert.Equal(t, "bitmap", pg.Type)
}

func TestBMSInfoGroups(t *testing.T) {
	groups := BMSInfoGroups()
	require.Len(t, groups, 2)

	// Group 0: BMS Info with 18 probes
	bmsInfo := groups[0]
	assert.Equal(t, "BMS Info", bmsInfo.Name)
	assert.Len(t, bmsInfo.Probes, 18, "BMSInfoGroups[0] probes")

	// Check that 0x9004 is a bms_clock Composite probe
	foundClock := false
	for _, p := range bmsInfo.Probes {
		if p.Addr == 0x9004 {
			foundClock = true
			assert.Equal(t, "bms_clock", p.Composite, "probe 0x9004 Composite")
			assert.Equal(t, uint16(2), p.Count, "probe 0x9004 Count")
		}
	}
	assert.True(t, foundClock, "BMSInfoGroups missing bms_clock probe at 0x9004")

	// Check that 0x9018 is a bms_sw_version Composite probe
	foundSW := false
	for _, p := range bmsInfo.Probes {
		if p.Addr == 0x9018 {
			foundSW = true
			assert.Equal(t, "bms_sw_version", p.Composite, "probe 0x9018 Composite")
			assert.Equal(t, uint16(4), p.Count, "probe 0x9018 Count")
		}
	}
	assert.True(t, foundSW, "BMSInfoGroups missing bms_sw_version probe at 0x9018")

	// Check SN probe
	foundSN := false
	for _, p := range bmsInfo.Probes {
		if p.Addr == 0x9024 && p.Count == 10 && p.IsASCII {
			foundSN = true
		}
	}
	assert.True(t, foundSN, "BMSInfoGroups missing SN probe at 0x9024 (Count 10, IsASCII)")

	// Check BMS Info expected addresses (no more 0x9005, 0x9019, 0x901A, 0x901B)
	expectedAddrs := []uint16{0x9004, 0x9006, 0x9007, 0x900B, 0x900C, 0x900D, 0x900E, 0x900F, 0x9010, 0x9011, 0x9012, 0x9013, 0x9018, 0x9024, 0x9022, 0x9023, 0x902F, 0x9030}
	addrSet := make(map[uint16]bool)
	for _, p := range bmsInfo.Probes {
		addrSet[p.Addr] = true
	}
	for _, addr := range expectedAddrs {
		assert.True(t, addrSet[addr], "BMSInfoGroups BMS Info missing probe at 0x%04X", addr)
	}

	// Group 1: Protection with 6 probes
	prot := groups[1]
	assert.Equal(t, "Protection", prot.Name)
	assert.Equal(t, "protection", prot.Type)
	assert.Len(t, prot.Probes, 6, "BMSInfoGroups[1] probes")
	protExpected := []uint16{0x9014, 0x9015, 0x9016, 0x9017, 0x901C, 0x901D}
	for i, addr := range protExpected {
		if i < len(prot.Probes) {
			assert.Equal(t, addr, prot.Probes[i].Addr, "Protection[%d].Addr", i)
		}
	}
}

func TestBMSProtectionInGroups(t *testing.T) {
	groups := BMSInfoGroups()
	require.True(t, len(groups) >= 2, "BMSInfoGroups returned fewer than 2 groups")
	prot := groups[1]
	assert.Equal(t, "Protection", prot.Name)
	assert.Equal(t, "protection", prot.Type)
	require.Len(t, prot.Probes, 6, "Protection probes")
	expectedAddrs := []uint16{0x9014, 0x9015, 0x9016, 0x9017, 0x901C, 0x901D}
	for i, addr := range expectedAddrs {
		assert.Equal(t, addr, prot.Probes[i].Addr, "Protection[%d].Addr", i)
		assert.Equal(t, uint16(1), prot.Probes[i].Count, "Protection[%d].Count", i)
	}
}

func TestStatisticsGroups(t *testing.T) {
	groups := StatisticsGroups()
	require.Len(t, groups, 2)

	expectedNames := []string{"Today", "Total"}
	for i, want := range expectedNames {
		assert.Equal(t, want, groups[i].Name, "StatisticsGroups[%d].Name", i)
	}

	// Each group has 6 probes, all U32=true, Count=2
	for i, g := range groups {
		if !assert.Len(t, g.Probes, 6, "StatisticsGroups[%d] probes", i) {
			continue
		}
		for j, p := range g.Probes {
			assert.True(t, p.U32, "StatisticsGroups[%d].Probes[%d].U32", i, j)
			assert.Equal(t, uint16(2), p.Count, "StatisticsGroups[%d].Probes[%d].Count", i, j)
			assert.Equal(t, "kWh", p.Unit, "StatisticsGroups[%d].Probes[%d].Unit", i, j)
		}
	}

	// Today scale = 0.01
	for j, p := range groups[0].Probes {
		assert.Equal(t, 0.01, p.Scale, "Today.Probes[%d].Scale", j)
	}

	// Total scale = 0.1
	for j, p := range groups[1].Probes {
		assert.Equal(t, 0.1, p.Scale, "Total.Probes[%d].Scale", j)
	}
}

func TestStatisticsAddresses(t *testing.T) {
	groups := StatisticsGroups()

	// Today starts at 0x0684
	assert.Equal(t, uint16(0x0684), groups[0].Probes[0].Addr, "Today gen addr")
	// Total starts at 0x0686
	assert.Equal(t, uint16(0x0686), groups[1].Probes[0].Addr, "Total gen addr")

	// Stride 4 between metrics within each group
	// Today: gen=0x0684, consumption=0x0688, bought=0x068C, sold=0x0690, bat_charge=0x0694, bat_discharge=0x0698
	todayExpected := []uint16{0x0684, 0x0688, 0x068C, 0x0690, 0x0694, 0x0698}
	for i, addr := range todayExpected {
		assert.Equal(t, addr, groups[0].Probes[i].Addr, "Today.Probes[%d].Addr", i)
	}
}

func TestBMSInfoGroupsIncludesOnlineBitmap(t *testing.T) {
	groups := BMSInfoGroups()
	found := false
	for _, g := range groups {
		for _, p := range g.Probes {
			if p.Addr == 0x9022 {
				found = true
				assert.Equal(t, "Online Bitmap", p.Name)
			}
		}
	}
	assert.True(t, found, "BMSInfoGroups missing probe at 0x9022")
}

func TestBMSInfoGroupsBatchPlan(t *testing.T) {
	groups := BMSInfoGroups()
	plan := AnalyzeBatchPlan(groups)
	require.Len(t, plan.Spans, 3)
	// Span 0: 0x9004..0x901D (BMS Info + Protection contiguous block)
	assert.Equal(t, uint16(0x9004), plan.Spans[0].StartAddr, "Span[0].StartAddr")
	assert.Equal(t, uint16(26), plan.Spans[0].TotalCount, "Span[0].TotalCount")
	// Span 1: 0x9022..0x902D (Online Bitmap, Hibernation, SN block)
	assert.Equal(t, uint16(0x9022), plan.Spans[1].StartAddr, "Span[1].StartAddr")
	assert.Equal(t, uint16(12), plan.Spans[1].TotalCount, "Span[1].TotalCount")
	// Span 2: 0x902F..0x9030 (Max Discharge/Charge Current)
	assert.Equal(t, uint16(0x902F), plan.Spans[2].StartAddr, "Span[2].StartAddr")
	assert.Equal(t, uint16(2), plan.Spans[2].TotalCount, "Span[2].TotalCount")
}

func TestDecodeBMSClock(t *testing.T) {
	// Encode 2026-04-10 14:03:05
	var val uint32 = 0x6914E0C5
	got := DecodeBMSClock(val)
	assert.Equal(t, "2026-04-10 14:03:05", got)
}

func TestDecodeTopology(t *testing.T) {
	parallelStrings, packsPerString := DecodeTopology(0x020A)
	assert.Equal(t, 2, parallelStrings, "DecodeTopology parallelStrings")
	assert.Equal(t, 10, packsPerString, "DecodeTopology packsPerString")
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
			assert.Equal(t, tt.want, got,
				"EncodePackQuery(%d,%d,%d,%d)", tt.input, tt.tower, tt.pack, tt.towersPerInput)
		})
	}
}

func TestPackRTProbes(t *testing.T) {
	probes := PackRTProbes()
	assert.GreaterOrEqual(t, len(probes), 30, "PackRTProbes count")

	// First probe should be Pack ID at 0x9044
	assert.Equal(t, "Pack ID", probes[0].Name, "first probe name")
	assert.Equal(t, uint16(0x9044), probes[0].Addr, "first probe addr")

	// Build lookup by name for specific checks
	byName := make(map[string]Probe)
	for _, p := range probes {
		byName[p.Name] = p
	}

	// Serial Number: ASCII, 10 registers
	if p, ok := byName["Serial Number"]; assert.True(t, ok, "missing Serial Number probe") {
		assert.Equal(t, uint16(0x9047), p.Addr, "Serial Number Addr")
		assert.Equal(t, uint16(10), p.Count, "Serial Number Count")
		assert.True(t, p.IsASCII, "Serial Number should be ASCII")
	}

	// Total Voltage
	if p, ok := byName["Total Voltage"]; assert.True(t, ok, "missing Total Voltage probe") {
		assert.Equal(t, uint16(0x9079), p.Addr, "Total Voltage Addr")
		assert.Equal(t, 0.1, p.Scale, "Total Voltage Scale")
		assert.Equal(t, "V", p.Unit, "Total Voltage Unit")
	}

	// Check Cell 1
	if p, ok := byName["Cell 1"]; assert.True(t, ok, "missing Cell 1 probe") {
		assert.Equal(t, uint16(0x9051), p.Addr, "Cell 1 Addr")
		assert.Equal(t, 0.001, p.Scale, "Cell 1 Scale")
		assert.Equal(t, "V", p.Unit, "Cell 1 Unit")
	}
	// Check Cell 16 (last cell per D-05)
	if p, ok := byName["Cell 16"]; assert.True(t, ok, "missing Cell 16 probe") {
		assert.Equal(t, uint16(0x9060), p.Addr, "Cell 16 Addr")
		assert.Equal(t, 0.001, p.Scale, "Cell 16 Scale")
	}

	// Cell 17 should NOT exist (only 16 cells per D-05)
	_, ok := byName["Cell 17"]
	assert.False(t, ok, "Cell 17 should not exist (only 16 cells per D-05)")

	// Verify exactly 16 cell voltage probes (D-05)
	cellCount := 0
	for _, p := range probes {
		if strings.HasPrefix(p.Name, "Cell ") && p.Scale == 0.001 && p.Unit == "V" {
			cellCount++
		}
	}
	assert.Equal(t, 16, cellCount, "cell voltage probes count")

	// Current: signed, scale 0.1, unit A
	if p, ok := byName["Current"]; assert.True(t, ok, "missing Current probe") {
		assert.Equal(t, uint16(0x9071), p.Addr, "Current Addr")
		assert.True(t, p.Signed, "Current should be Signed")
		assert.Equal(t, 0.1, p.Scale, "Current Scale")
		assert.Equal(t, "A", p.Unit, "Current Unit")
	}

	// Temp 1-4 at 0x906B-0x906E, Signed, Scale 0.1, Unit C
	tempAddrs := map[string]uint16{"Temp 1": 0x906B, "Temp 2": 0x906C, "Temp 3": 0x906D, "Temp 4": 0x906E}
	for name, wantAddr := range tempAddrs {
		p, ok := byName[name]
		if !assert.True(t, ok, "missing %s probe", name) {
			continue
		}
		assert.Equal(t, wantAddr, p.Addr, "%s Addr", name)
		assert.True(t, p.Signed, "%s should be Signed", name)
		assert.Equal(t, 0.1, p.Scale, "%s Scale", name)
	}

	// MOS Temp and Env Temp: signed, scale 0.1
	if p, ok := byName["MOS Temp"]; assert.True(t, ok, "missing MOS Temp probe") {
		assert.Equal(t, uint16(0x906F), p.Addr, "MOS Temp Addr")
		assert.True(t, p.Signed, "MOS Temp should be Signed")
		assert.Equal(t, 0.1, p.Scale, "MOS Temp Scale")
	}
	if p, ok := byName["Env Temp"]; assert.True(t, ok, "missing Env Temp probe") {
		assert.Equal(t, uint16(0x9070), p.Addr, "Env Temp Addr")
		assert.True(t, p.Signed, "Env Temp should be Signed")
		assert.Equal(t, 0.1, p.Scale, "Env Temp Scale")
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
		if !assert.True(t, ok, "missing %s probe", name) {
			continue
		}
		assert.Equal(t, wantAddr, p.Addr, "%s Addr", name)
	}

	// Min/Max Cell Voltage
	if p, ok := byName["Min Cell Voltage"]; assert.True(t, ok, "missing Min Cell Voltage probe") {
		assert.Equal(t, uint16(0x906A), p.Addr, "Min Cell Voltage Addr")
		assert.Equal(t, 0.001, p.Scale, "Min Cell Voltage Scale")
		assert.Equal(t, "V", p.Unit, "Min Cell Voltage Unit")
	}
	if p, ok := byName["Max Cell Voltage"]; assert.True(t, ok, "missing Max Cell Voltage probe") {
		assert.Equal(t, uint16(0x9069), p.Addr, "Max Cell Voltage Addr")
		assert.Equal(t, 0.001, p.Scale, "Max Cell Voltage Scale")
	}
}

func TestPackInfoProbes(t *testing.T) {
	probes := PackInfoProbes()
	assert.GreaterOrEqual(t, len(probes), 6, "PackInfoProbes count")

	byName := make(map[string]Probe)
	for _, p := range probes {
		byName[p.Name] = p
	}

	// SOH
	if p, ok := byName["SOH"]; assert.True(t, ok, "missing SOH probe") {
		assert.Equal(t, uint16(0x910A), p.Addr, "SOH Addr")
		assert.Equal(t, 0.1, p.Scale, "SOH Scale")
		assert.Equal(t, "%", p.Unit, "SOH Unit")
	}

	// Rated Capacity
	if p, ok := byName["Rated Capacity"]; assert.True(t, ok, "missing Rated Capacity probe") {
		assert.Equal(t, uint16(0x910B), p.Addr, "Rated Capacity Addr")
		assert.Equal(t, 0.1, p.Scale, "Rated Capacity Scale")
		assert.Equal(t, "Ah", p.Unit, "Rated Capacity Unit")
	}

	// Manufacturer
	if p, ok := byName["Manufacturer"]; assert.True(t, ok, "missing Manufacturer probe") {
		assert.Equal(t, uint16(0x9106), p.Addr, "Manufacturer Addr")
		assert.Equal(t, uint16(4), p.Count, "Manufacturer Count")
		assert.True(t, p.IsASCII, "Manufacturer should be ASCII")
	}

	// Alarm 2, Protection 2, Fault 2
	extProbes := map[string]uint16{
		"Alarm Status 2":      0x9124,
		"Protection Status 2": 0x9125,
		"Fault Status 2":      0x9126,
	}
	for name, wantAddr := range extProbes {
		p, ok := byName[name]
		if !assert.True(t, ok, "missing %s probe", name) {
			continue
		}
		assert.Equal(t, wantAddr, p.Addr, "%s Addr", name)
	}
}

func TestPackTemps58Probes(t *testing.T) {
	probes := PackTemps58Probes()
	require.Len(t, probes, 4)

	wantAddrs := []uint16{0x90BC, 0x90BD, 0x90BE, 0x90BF}
	for i, p := range probes {
		wantName := "Temp " + string(rune('5'+i))
		assert.Equal(t, wantAddrs[i], p.Addr, "probe %d Addr", i)
		assert.True(t, p.Signed, "%s should be Signed", wantName)
		assert.Equal(t, 0.1, p.Scale, "%s Scale", wantName)
		assert.Equal(t, "\u00b0C", p.Unit, "%s Unit", wantName)
	}
}

func TestBMSAlarmTable(t *testing.T) {
	require.NotEmpty(t, BMSAlarmBits, "BMSAlarmBits is empty")

	// Check for cell OV alarm at bit 0
	found := false
	for _, fb := range BMSAlarmBits {
		if fb.Addr == 0x9076 && fb.Bit == 0 {
			assert.Contains(t, fb.Desc, "Cell", "bit 0 Desc should contain Cell")
			assert.Contains(t, fb.Desc, "OV", "bit 0 Desc should contain OV")
			found = true
		}
	}
	assert.True(t, found, "missing BMSAlarmBits entry at Addr=0x9076 Bit=0")

	// Check for cell UV alarm at bit 1
	found = false
	for _, fb := range BMSAlarmBits {
		if fb.Addr == 0x9076 && fb.Bit == 1 {
			assert.Contains(t, fb.Desc, "Cell", "bit 1 Desc should contain Cell")
			assert.Contains(t, fb.Desc, "UV", "bit 1 Desc should contain UV")
			found = true
		}
	}
	assert.True(t, found, "missing BMSAlarmBits entry at Addr=0x9076 Bit=1")
}

func TestBMSProtectionTable(t *testing.T) {
	require.NotEmpty(t, BMSProtectionBits, "BMSProtectionBits is empty")

	// Check for cell OV protection at bit 0
	found := false
	for _, fb := range BMSProtectionBits {
		if fb.Addr == 0x9077 && fb.Bit == 0 {
			assert.Contains(t, fb.Desc, "Cell", "bit 0 Desc should contain Cell")
			assert.Contains(t, fb.Desc, "OV", "bit 0 Desc should contain OV")
			found = true
		}
	}
	assert.True(t, found, "missing BMSProtectionBits entry at Addr=0x9077 Bit=0")
}

func TestBMSFaultTable_Pack(t *testing.T) {
	require.NotEmpty(t, BMSFaultBits, "BMSFaultBits is empty")

	// Check entries exist for 0x9078
	found := false
	for _, fb := range BMSFaultBits {
		if fb.Addr == 0x9078 {
			found = true
			break
		}
	}
	assert.True(t, found, "missing BMSFaultBits entries for Addr=0x9078")
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
			assert.Equal(t, tt.exact, got, "DecodeBalanceState(0x%04X)", tt.val)
		}
		for _, sub := range tt.contains {
			assert.Contains(t, got, sub, "DecodeBalanceState(0x%04X) missing %q", tt.val, sub)
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
		{
			name: "Composite_system_time",
			p:    Probe{Addr: 0x042C, Count: 6, Composite: "system_time"},
			data: func() []byte {
				d := make([]byte, 12)
				for i := 0; i < 6; i++ {
					binary.BigEndian.PutUint16(d[i*2:i*2+2], uint16(i+1))
				}
				return d
			}(),
			want: "0x042C-0x0431 | 1, 2, 3, 4, 5, 6",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatRawValue(tt.p, tt.data)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDecodeBMSBitmap(t *testing.T) {
	// Use BMSAlarmBits for testing bitmap decoding
	// Bit 0 and bit 1 set at address 0x9076
	result := DecodeBMSBitmap(0x0003, BMSAlarmBits, 0x9076)
	assert.Len(t, result, 2, "DecodeBMSBitmap(0x0003)")

	// No bits set
	result = DecodeBMSBitmap(0x0000, BMSAlarmBits, 0x9076)
	assert.Empty(t, result, "DecodeBMSBitmap(0x0000)")
}

func TestPackProbeGroupOrder(t *testing.T) {
	groups := PackProbeGroups()

	// Exactly 5 groups
	require.Len(t, groups, 5)

	// Group names in correct order (D-03)
	wantNames := []string{"Pack Info", "Cell Voltages", "Balance State", "Temperatures", "Pack Status"}
	for i, want := range wantNames {
		assert.Equal(t, want, groups[i].Name, "group[%d].Name", i)
	}

	// Group types
	wantTypes := []string{"", "cell_grid", "balance", "", "pack_status"}
	for i, want := range wantTypes {
		assert.Equal(t, want, groups[i].Type, "group[%d].Type", i)
	}

	// Cell Voltages group: 16 cell probes + Max Cell Voltage + Min Cell Voltage = 18 probes
	cellGroup := groups[1]
	require.Len(t, cellGroup.Probes, 18, "Cell Voltages group probes")
	for i := 0; i < 16; i++ {
		wantName := fmt.Sprintf("Cell %d", i+1)
		assert.Equal(t, wantName, cellGroup.Probes[i].Name, "Cell Voltages probe[%d].Name", i)
	}

	// Balance State group: probe at 0x9075
	balanceGroup := groups[2]
	require.Len(t, balanceGroup.Probes, 1, "Balance State group probes")
	assert.Equal(t, uint16(0x9075), balanceGroup.Probes[0].Addr, "Balance State probe[0].Addr")

	// Temperatures group: Temp 1-4 (0x906B-0x906E), MOS Temp (0x906F), Env Temp (0x9070), Temp 5-8 (0x90BC-0x90BF) = 10 probes
	tempGroup := groups[3]
	require.Len(t, tempGroup.Probes, 10, "Temperatures group probes")
	wantTempAddrs := []uint16{0x906B, 0x906C, 0x906D, 0x906E, 0x906F, 0x9070, 0x90BC, 0x90BD, 0x90BE, 0x90BF}
	for i, wantAddr := range wantTempAddrs {
		assert.Equal(t, wantAddr, tempGroup.Probes[i].Addr, "Temperatures probe[%d].Addr", i)
	}

	// Pack Status group: 6 probes at 0x9076, 0x9077, 0x9078, 0x9124, 0x9125, 0x9126
	statusGroup := groups[4]
	require.Len(t, statusGroup.Probes, 6, "Pack Status group probes")
	wantStatusAddrs := []uint16{0x9076, 0x9077, 0x9078, 0x9124, 0x9125, 0x9126}
	for i, wantAddr := range wantStatusAddrs {
		assert.Equal(t, wantAddr, statusGroup.Probes[i].Addr, "Pack Status probe[%d].Addr", i)
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
		assert.True(t, gotInfoAddrs[addr], "Pack Info group missing probe at addr 0x%04X", addr)
	}
	assert.Len(t, infoGroup.Probes, len(wantInfoAddrs), "Pack Info group probe count")
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
		"ChargeDischargeControlEnum": ChargeDischargeControlEnum,
		"ChargingSourceEnum":        ChargingSourceEnum,
		"ProtectionEnableEnum":      ProtectionEnableEnum,
		"ReactiveControlModeEnum":   ReactiveControlModeEnum,
		"EnableStatusEnum":          EnableStatusEnum,
		"InputChannelTypeEnum":      InputChannelTypeEnum,
	}

	for name, enum := range allEnums {
		if !assert.NotNil(t, enum, "%s is nil", name) {
			continue
		}
		assert.GreaterOrEqual(t, len(enum), 2, "%s has too few entries", name)
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
		if !assert.True(t, ok, "%s[%d] not found", sc.name, sc.key) {
			continue
		}
		assert.Equal(t, sc.want, got, "%s[%d]", sc.name, sc.key)
	}
}

// === Configuration Groups Tests ===

func TestConfigurationGroups(t *testing.T) {
	groups := ConfigurationGroups

	// Must be non-nil with >= 15 groups
	require.NotNil(t, groups, "ConfigurationGroups is nil")
	assert.GreaterOrEqual(t, len(groups), 18, "ConfigurationGroups count")

	// First group should be "System Config"
	assert.Equal(t, "System Config", groups[0].Name, "First group name")
	// First probe should be in 0x1000 range
	if len(groups[0].Probes) > 0 {
		assert.GreaterOrEqual(t, groups[0].Probes[0].Addr, uint16(0x1000), "First group first probe addr")
	}

	// Find specific groups by name
	groupMap := make(map[string]*ProbeGroup)
	for i := range groups {
		groupMap[groups[i].Name] = &groups[i]
	}

	// "Battery Config" group with probe at 0x1046 referencing BatteryProtocolEnum
	battConfig, ok := groupMap["Battery Config"]
	if assert.True(t, ok, "Missing 'Battery Config' group") {
		found := false
		for _, p := range battConfig.Probes {
			if p.Addr == 0x1046 {
				found = true
				assert.NotNil(t, p.Enum, "Battery Config probe at 0x1046 has nil Enum")
				break
			}
		}
		assert.True(t, found, "Battery Config missing probe at 0x1046")
	}

	// "Energy Storage Mode" group with probe at 0x1110
	esMode, ok := groupMap["Energy Storage Mode"]
	if assert.True(t, ok, "Missing 'Energy Storage Mode' group") {
		found := false
		for _, p := range esMode.Probes {
			if p.Addr == 0x1110 {
				found = true
				assert.NotNil(t, p.Enum, "Energy Storage Mode probe at 0x1110 has nil Enum")
				break
			}
		}
		assert.True(t, found, "Energy Storage Mode missing probe at 0x1110")
	}

	// No duplicate addresses across all groups
	addrSet := make(map[uint16]string)
	totalProbes := 0
	for _, g := range groups {
		for _, p := range g.Probes {
			totalProbes++
			if prev, exists := addrSet[p.Addr]; exists {
				assert.Fail(t, "Duplicate address",
					"Duplicate address 0x%04X in group %q (first seen in %q)", p.Addr, g.Name, prev)
			}
			addrSet[p.Addr] = g.Name
		}
	}

	// All probes have Count >= 1
	for _, g := range groups {
		for _, p := range g.Probes {
			assert.GreaterOrEqual(t, p.Count, uint16(1),
				"Probe %q in group %q has Count < 1", p.Name, g.Name)
		}
	}

	// All U32 probes have Count == 2
	for _, g := range groups {
		for _, p := range g.Probes {
			if p.U32 {
				assert.Equal(t, uint16(2), p.Count,
					"U32 probe %q in group %q should have Count 2", p.Name, g.Name)
			}
		}
	}

	// Total probe count >= 224 (post hardware sweep cleanup)
	assert.GreaterOrEqual(t, totalProbes, 224, "Total probe count")

	// At least 5 safety groups exist ("Safety:" prefix)
	safetyCount := 0
	for _, g := range groups {
		if strings.HasPrefix(g.Name, "Safety:") {
			safetyCount++
		}
	}
	assert.GreaterOrEqual(t, safetyCount, 5, "Safety group count")
}

// TestInternalInfoGroups verifies InternalInfoGroups returns the expected structure:
// exactly 1 group named "Internal Info" with 5 probes at known addresses.
func TestInternalInfoGroups(t *testing.T) {
	groups := InternalInfoGroups()
	require.Len(t, groups, 1)

	g := groups[0]
	assert.Equal(t, "Internal Info", g.Name)
	require.Len(t, g.Probes, 5, "probe count")

	// First probe: "Total BUS voltage" at 0x06CC
	assert.Equal(t, "Total BUS voltage", g.Probes[0].Name, "first probe name")
	assert.Equal(t, uint16(0x06CC), g.Probes[0].Addr, "first probe addr")

	// Last probe: "Rated power" at 0x06ED
	last := g.Probes[len(g.Probes)-1]
	assert.Equal(t, "Rated power", last.Name, "last probe name")
	assert.Equal(t, uint16(0x06ED), last.Addr, "last probe addr")
}
