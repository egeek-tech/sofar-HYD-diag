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
	want := "AMASS"
	assert.Equal(t, want, got)
}

func TestFormatValueUnsignedScaled(t *testing.T) {
	p := Probe{Name: "Voltage", Scale: 0.1, Unit: "V"}
	// Encode 5288 as big-endian uint16
	data := make([]byte, 2)
	binary.BigEndian.PutUint16(data, 5288)
	got := FormatValue(p, data)
	want := "528.80 V"
	assert.Equal(t, want, got)
}

func TestFormatValueSignedNegative(t *testing.T) {
	p := Probe{Name: "Power", Signed: true, Scale: 0.01, Unit: "kW"}
	// Encode -83 as big-endian int16 (two's complement)
	data := make([]byte, 2)
	neg83 := int16(-83)
	binary.BigEndian.PutUint16(data, uint16(neg83))
	got := FormatValue(p, data)
	want := "-0.83 kW"
	assert.Equal(t, want, got)
}

func TestFormatValueNoScale(t *testing.T) {
	p := Probe{Name: "State"}
	// Encode 164 as big-endian uint16
	data := make([]byte, 2)
	binary.BigEndian.PutUint16(data, 164)
	got := FormatValue(p, data)
	want := "164 (0x00A4)"
	assert.Equal(t, want, got)
}

func TestFormatValueShortData(t *testing.T) {
	p := Probe{Name: "Short"}
	data := []byte{0x01}
	got := FormatValue(p, data)
	want := "<no data>"
	assert.Equal(t, want, got)
}

func TestFormatValueSignedNoUnit(t *testing.T) {
	p := Probe{Name: "Raw signed", Signed: true}
	data := make([]byte, 2)
	neg42 := int16(-42)
	binary.BigEndian.PutUint16(data, uint16(neg42))
	got := FormatValue(p, data)
	want := "-42"
	assert.Equal(t, want, got)
}

// === Edge case tests for FormatValue (D-06) ===

func TestFormatValueEdgeCases(t *testing.T) {
	t.Run("ZeroLengthData", func(t *testing.T) {
		p := Probe{Name: "Empty", Scale: 0.1, Unit: "V"}
		got := FormatValue(p, []byte{})
		assert.Equal(t, "<no data>", got, "zero-length data should return <no data>")
	})

	t.Run("MaxUint16Unsigned", func(t *testing.T) {
		p := Probe{Name: "MaxU16", Scale: 1, Unit: ""}
		data := make([]byte, 2)
		binary.BigEndian.PutUint16(data, 0xFFFF)
		got := FormatValue(p, data)
		assert.Equal(t, "65535.000", got, "max uint16 with scale=1, no unit uses 3 decimal places")
	})

	t.Run("MaxNegativeInt16Signed", func(t *testing.T) {
		p := Probe{Name: "MinS16", Signed: true, Scale: 1, Unit: "W"}
		data := make([]byte, 2)
		binary.BigEndian.PutUint16(data, 0x8000) // -32768
		got := FormatValue(p, data)
		assert.Equal(t, "-32768.00 W", got, "max negative int16 with scale=1")
	})

	t.Run("U32ShortData", func(t *testing.T) {
		p := Probe{Name: "ShortU32", U32: true, Count: 2}
		got := FormatValue(p, []byte{0x01, 0x02}) // only 2 bytes for U32
		assert.Equal(t, "<no data>", got, "U32 with only 2 bytes should return <no data>")
	})

	t.Run("ScaleZeroUnsigned", func(t *testing.T) {
		// Scale=0 means no scaling -- should fall through to hex format
		p := Probe{Name: "NoScale", Scale: 0, Unit: ""}
		data := make([]byte, 2)
		binary.BigEndian.PutUint16(data, 42)
		got := FormatValue(p, data)
		assert.Equal(t, "42 (0x002A)", got, "scale=0 should format as raw hex")
	})

	t.Run("ScaleZeroSigned", func(t *testing.T) {
		// Scale=0 with signed means no scaling -- should format as plain integer
		p := Probe{Name: "NoScaleSigned", Scale: 0, Signed: true}
		data := make([]byte, 2)
		neg1 := int16(-1)
		binary.BigEndian.PutUint16(data, uint16(neg1))
		got := FormatValue(p, data)
		assert.Equal(t, "-1", got, "signed scale=0 should format as plain integer")
	})

	t.Run("NilData", func(t *testing.T) {
		p := Probe{Name: "Nil"}
		got := FormatValue(p, nil)
		assert.Equal(t, "<no data>", got, "nil data should return <no data>")
	})
}

func TestFormatRawValueEdgeCases(t *testing.T) {
	t.Run("EmptyData", func(t *testing.T) {
		p := Probe{Name: "Empty"}
		got := FormatRawValue(p, []byte{})
		assert.Equal(t, "", got, "empty data should return empty string")
	})

	t.Run("SingleByte", func(t *testing.T) {
		p := Probe{Name: "Short"}
		got := FormatRawValue(p, []byte{0x01})
		assert.Equal(t, "", got, "single byte should return empty string")
	})

	t.Run("U32ShortData", func(t *testing.T) {
		p := Probe{Name: "ShortU32", U32: true}
		got := FormatRawValue(p, []byte{0x01, 0x02})
		// With only 2 bytes and U32, falls through to uint16 path
		assert.Equal(t, "258", got, "U32 with 2 bytes falls through to uint16")
	})

	t.Run("ASCIIProbe", func(t *testing.T) {
		p := Probe{Name: "SN", IsASCII: true}
		data := []byte("AB")
		got := FormatRawValue(p, data)
		assert.Equal(t, "4142", got, "ASCII probe raw value should be hex")
	})
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
		if !assert.True(t, ok) {
			continue
		}
		assert.Equal(t, tt.want, got)
	}
	// Verify all 12 entries exist (0-11)
	assert.Len(t, RunningStateEnum, 12)
}

func TestSystemGroups(t *testing.T) {
	require.Len(t, SystemGroups, 5)

	expectedNames := []string{"Identity", "Firmware", "Status", "Temperatures", "Protection"}
	for i, want := range expectedNames {
		assert.Equal(t, want, SystemGroups[i].Name)
	}

	// Identity: Inverter SN at 0x0445, Count 10, IsASCII
	identity := SystemGroups[0]
	require.GreaterOrEqual(t, len(identity.Probes), 1)
	assert.Equal(t, uint16(0x0445), identity.Probes[0].Addr)
	assert.Equal(t, uint16(10), identity.Probes[0].Count)
	assert.True(t, identity.Probes[0].IsASCII, "Identity SN IsASCII should be true")

	// Firmware: 5 probes
	firmware := SystemGroups[1]
	require.Len(t, firmware.Probes, 5)
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
		assert.Equal(t, fw.name, firmware.Probes[i].Name)
		assert.Equal(t, fw.addr, firmware.Probes[i].Addr)
		assert.Equal(t, fw.count, firmware.Probes[i].Count)
	}

	// Status: Running state with Enum at 0x0404, plus 6 time registers
	status := SystemGroups[2]
	assert.Equal(t, uint16(0x0404), status.Probes[0].Addr)
	assert.NotNil(t, status.Probes[0].Enum , "Status running state Enum should not be nil")
	assert.Len(t, status.Probes, 7)

	// Temperatures: 4 probes, all S16
	temps := SystemGroups[3]
	require.Len(t, temps.Probes, 4)
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
		assert.Equal(t, te.name, temps.Probes[i].Name)
		assert.Equal(t, te.addr, temps.Probes[i].Addr)
		assert.True(t, temps.Probes[i].Signed)
	}

	// Protection: Insulation impedance (0x042B) and Fan speed (0x043E)
	protection := SystemGroups[4]
	require.Len(t, protection.Probes, 2)
	assert.Equal(t, uint16(0x042B), protection.Probes[0].Addr)
	assert.Equal(t, uint16(0x043E), protection.Probes[1].Addr)
}

func TestGridGroups(t *testing.T) {
	require.Len(t, GridGroups, 7)

	expectedNames := []string{"General", "Phase R", "Phase S", "Phase T", "PCC Power", "Line Voltages", "Load"}
	for i, want := range expectedNames {
		assert.Equal(t, want, GridGroups[i].Name)
	}

	// Phase R: column layout, 5 probes
	phaseR := GridGroups[1]
	assert.Equal(t, "column", phaseR.Layout)
	require.Len(t, phaseR.Probes, 5)
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
		assert.Equal(t, re.name, phaseR.Probes[i].Name)
		assert.Equal(t, re.addr, phaseR.Probes[i].Addr)
		assert.Equal(t, re.scale, phaseR.Probes[i].Scale)
	}
	// Power factor has no Unit
	assert.Equal(t, "", phaseR.Probes[4].Unit)

	// Phase S: column layout, Voltage at 0x0498
	phaseS := GridGroups[2]
	assert.Equal(t, "column", phaseS.Layout)
	assert.Equal(t, uint16(0x0498), phaseS.Probes[0].Addr)
	assert.Equal(t, uint16(0x0499), phaseS.Probes[1].Addr)
	assert.Equal(t, uint16(0x049A), phaseS.Probes[2].Addr)
	assert.Equal(t, uint16(0x049B), phaseS.Probes[3].Addr)
	assert.Equal(t, uint16(0x049C), phaseS.Probes[4].Addr)

	// Phase T: Voltage at 0x04A3
	phaseT := GridGroups[3]
	assert.Equal(t, "column", phaseT.Layout)
	assert.Equal(t, uint16(0x04A3), phaseT.Probes[0].Addr)

	// Load: Total load power (0x04AF), Total power factor (0x04BD S16 Scale 0.001), Generation efficiency (0x04BF)
	load := GridGroups[6]
	require.Len(t, load.Probes, 3)
	assert.Equal(t, uint16(0x04AF), load.Probes[0].Addr)
	assert.Equal(t, uint16(0x04BD), load.Probes[1].Addr)
	assert.True(t, load.Probes[1].Signed, "Load total power factor should be signed")
	assert.Equal(t, 0.001, load.Probes[1].Scale)
	assert.Equal(t, uint16(0x04BF), load.Probes[2].Addr)
}

func TestEPSGroups(t *testing.T) {
	require.Len(t, EPSGroups, 5)

	expectedNames := []string{"General", "Phase R", "Phase S", "Phase T", "Emergency Load"}
	for i, want := range expectedNames {
		assert.Equal(t, want, EPSGroups[i].Name)
	}

	// General: 4 probes
	general := EPSGroups[0]
	require.Len(t, general.Probes, 4)
	assert.Equal(t, uint16(0x0504), general.Probes[0].Addr)
	assert.Equal(t, uint16(0x0505), general.Probes[1].Addr)
	assert.Equal(t, uint16(0x0506), general.Probes[2].Addr)
	assert.Equal(t, uint16(0x0507), general.Probes[3].Addr)

	// Phase R: column layout, 2 probes
	phaseR := EPSGroups[1]
	assert.Equal(t, "column", phaseR.Layout)
	assert.Equal(t, uint16(0x050A), phaseR.Probes[0].Addr)
	assert.Equal(t, uint16(0x050B), phaseR.Probes[1].Addr)
	assert.True(t, phaseR.Probes[1].Signed, "EPS Phase R load current should be signed")

	// Phase S: 0x0512/0x0513
	phaseS := EPSGroups[2]
	assert.Equal(t, "column", phaseS.Layout)
	assert.Equal(t, uint16(0x0512), phaseS.Probes[0].Addr)
	assert.Equal(t, uint16(0x0513), phaseS.Probes[1].Addr)

	// Phase T: 0x051A/0x051B
	phaseT := EPSGroups[3]
	assert.Equal(t, uint16(0x051A), phaseT.Probes[0].Addr)
	assert.Equal(t, uint16(0x051B), phaseT.Probes[1].Addr)

	// Emergency Load: voltages at 0x0510, 0x0518, 0x0520
	emerg := EPSGroups[4]
	require.Len(t, emerg.Probes, 3)
	assert.Equal(t, uint16(0x0510), emerg.Probes[0].Addr)
	assert.Equal(t, uint16(0x0518), emerg.Probes[1].Addr)
	assert.Equal(t, uint16(0x0520), emerg.Probes[2].Addr)
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
	assert.Equal(t, "Grid-connected", got)

	// Unknown value falls back to raw format
	binary.BigEndian.PutUint16(data, 99)
	got = FormatValue(p, data)
	want := "99 (0x0063)"
	assert.Equal(t, want, got)
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
	want := "2026-04-10 14:30:05"
	assert.Equal(t, want, got)

	got = ComposeSystemTime(0, 1, 1, 0, 0, 0)
	want = "2000-01-01 00:00:00"
	assert.Equal(t, want, got)
}

// === Task 2 TDD tests: Fault bitmap decoder ===

func TestFaultBitStruct(t *testing.T) {
	fb := FaultBit{Addr: 0x0405, Bit: 0, Desc: "Grid over-voltage"}
	assert.Equal(t, uint16(0x0405), fb.Addr)
	assert.Equal(t, uint8(0), fb.Bit)
	assert.Equal(t, "Grid over-voltage", fb.Desc)
}

func TestFaultTableSize(t *testing.T) {
	assert.GreaterOrEqual(t, len(FaultTable), 200)
}

func TestFaultTableFirstEntry(t *testing.T) {
	require.NotEmpty(t, FaultTable)
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
	assert.Equal(t, uint16(0x0405), FaultRegisters[0].Addr)
	assert.Equal(t, uint16(18), FaultRegisters[0].Count)
	assert.Equal(t, uint16(0x0432), FaultRegisters[1].Addr)
	assert.Equal(t, uint16(12), FaultRegisters[1].Count)
}

func TestDecodeFaultsEmpty(t *testing.T) {
	faultData := map[uint16]uint16{
		0x0405: 0x0000,
		0x0406: 0x0000,
	}
	faults := DecodeFaults(faultData)
	assert.Len(t, faults, 0)
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
	assert.Len(t, faults, 0)
}

// === Task 2 TDD tests: Dynamic PV group generator ===

func TestGeneratePVGroups2(t *testing.T) {
	groups := GeneratePVGroups(2)
	require.Len(t, groups, 3)

	// PV 1
	assert.Equal(t, "PV 1", groups[0].Name)
	assert.Equal(t, "column", groups[0].Layout)
	require.Len(t, groups[0].Probes, 3)
	assert.Equal(t, uint16(0x0584), groups[0].Probes[0].Addr)
	assert.Equal(t, 0.1, groups[0].Probes[0].Scale)
	assert.Equal(t, "V", groups[0].Probes[0].Unit)
	assert.Equal(t, uint16(0x0585), groups[0].Probes[1].Addr)
	assert.True(t, groups[0].Probes[1].Signed, "PV 1 current should be signed")
	assert.Equal(t, 0.01, groups[0].Probes[1].Scale)
	assert.Equal(t, "A", groups[0].Probes[1].Unit)
	assert.Equal(t, uint16(0x0586), groups[0].Probes[2].Addr)
	assert.Equal(t, 0.01, groups[0].Probes[2].Scale)
	assert.Equal(t, "kW", groups[0].Probes[2].Unit)

	// PV 2
	assert.Equal(t, "PV 2", groups[1].Name)
	assert.Equal(t, uint16(0x0587), groups[1].Probes[0].Addr)
	assert.Equal(t, uint16(0x0588), groups[1].Probes[1].Addr)
	assert.Equal(t, uint16(0x0589), groups[1].Probes[2].Addr)

	// Total PV Power
	assert.Equal(t, "Total PV Power", groups[2].Name)
	assert.Equal(t, "", groups[2].Layout)
}

func TestGeneratePVGroups16(t *testing.T) {
	groups := GeneratePVGroups(16)
	require.Len(t, groups, 17)
	// PV 16 voltage at 0x05B1
	pv16 := groups[15]
	assert.Equal(t, "PV 16", pv16.Name)
	assert.Equal(t, uint16(0x05B1), pv16.Probes[0].Addr)
}

func TestGeneratePVGroupsTotalPower(t *testing.T) {
	groups := GeneratePVGroups(4)
	total := groups[len(groups)-1]
	assert.Equal(t, "Total PV Power", total.Name)
	require.Len(t, total.Probes, 1)
	assert.Equal(t, uint16(0x05C4), total.Probes[0].Addr)
	assert.Equal(t, 0.1, total.Probes[0].Scale)
	assert.Equal(t, "kW", total.Probes[0].Unit)
}

func TestGeneratePVGroupsColumnLayout(t *testing.T) {
	groups := GeneratePVGroups(3)
	for i := 0; i < 3; i++ {
		assert.Equal(t, "column", groups[i].Layout)
	}
	assert.Equal(t, "", groups[3].Layout)
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
	assert.Equal(t, want, got)
}

func TestFormatValueU32Large(t *testing.T) {
	p := Probe{Name: "Energy", U32: true, Count: 2, Scale: 0.1, Unit: "kWh"}
	// Encode 1234567: hi_word=18 (1234567 >> 16 = 18), lo_word=54919 (1234567 & 0xFFFF = 54919)
	data := make([]byte, 4)
	binary.BigEndian.PutUint16(data[:2], 18)
	binary.BigEndian.PutUint16(data[2:4], 54919)
	got := FormatValue(p, data)
	want := "123456.70 kWh"
	assert.Equal(t, want, got)
}

func TestFormatValueU32NoScale(t *testing.T) {
	p := Probe{Name: "Raw", U32: true, Count: 2}
	// Encode 42: hi_word=0, lo_word=42
	data := make([]byte, 4)
	binary.BigEndian.PutUint16(data[:2], 0)
	binary.BigEndian.PutUint16(data[2:4], 42)
	got := FormatValue(p, data)
	want := "42 (0x0000002A)"
	assert.Equal(t, want, got)
}

func TestFormatValueU32ShortData(t *testing.T) {
	p := Probe{Name: "Short", U32: true}
	data := []byte{0x00, 0x01} // only 2 bytes, need 4
	got := FormatValue(p, data)
	want := "<no data>"
	assert.Equal(t, want, got)
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
		if !assert.True(t, ok) {
			continue
		}
		assert.Equal(t, v, got)
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
	require.Len(t, ch1.Probes, 10)
	// First 7 probes: pack info at base 0x0604
	assert.Equal(t, uint16(0x0604), ch1.Probes[0].Addr)

	// Channel 2
	ch2 := groups[1]
	assert.Equal(t, "Channel 2", ch2.Name)
	require.Len(t, ch2.Probes, 10)
	// Channel 2 base = 0x0604 + 7*(2-1) = 0x060B
	assert.Equal(t, uint16(0x060B), ch2.Probes[0].Addr)

	// Each channel has 10 probes (7 pack info + charge limit + discharge limit + state)
	// State probe should have BatteryStateEnum
	stateProbe := ch1.Probes[9]
	assert.NotNil(t, stateProbe.Enum , "Channel 1 state probe Enum should not be nil")

	// Global Stats
	global := groups[2]
	assert.Equal(t, "Global Stats", global.Name)
	assert.Equal(t, "", global.Layout)
	require.Len(t, global.Probes, 5)
	assert.Equal(t, uint16(0x0667), global.Probes[0].Addr)
	assert.Equal(t, uint16(0x066B), global.Probes[4].Addr)
}

func TestGenerateBatteryGroups1(t *testing.T) {
	groups := GenerateBatteryGroups(1)
	// 1 channel group + 1 global stats = 2
	require.Len(t, groups, 2)
	// State probe should have BatteryStateEnum
	ch1 := groups[0]
	stateProbe := ch1.Probes[9]
	assert.NotNil(t, stateProbe.Enum , "Channel 1 state probe Enum should not be nil")
	if _, ok := stateProbe.Enum[1]; !ok {
		assert.Fail(t, "State probe Enum missing key 1 (Charging)")
	}
}

func TestGenerateBatteryGroupsAddresses(t *testing.T) {
	groups := GenerateBatteryGroups(2)
	ch1 := groups[0]
	ch2 := groups[1]

	// Channel 1 pack info: voltage=0x0604
	assert.Equal(t, uint16(0x0604), ch1.Probes[0].Addr)
	// Channel 2 pack info: voltage=0x060B
	assert.Equal(t, uint16(0x060B), ch2.Probes[0].Addr)
	// Channel 1 charge limit: 0x0644
	assert.Equal(t, uint16(0x0644), ch1.Probes[7].Addr)
	// Channel 1 discharge limit: 0x0645
	assert.Equal(t, uint16(0x0645), ch1.Probes[8].Addr)
	// Channel 1 state: 0x0646
	assert.Equal(t, uint16(0x0646), ch1.Probes[9].Addr)
	// Channel 2 charge limit: 0x0648
	assert.Equal(t, uint16(0x0648), ch2.Probes[7].Addr)
	// Channel 2 state: 0x064A
	assert.Equal(t, uint16(0x064A), ch2.Probes[9].Addr)
}

// === Phase 04 Task 2 TDD tests: ProbeGroup Type, BMSInfoGroups, BMSProtectionProbes, StatisticsGroups, DecodeBMSClock, DecodeTopology ===

func TestProbeGroupType(t *testing.T) {
	pg := ProbeGroup{Name: "Protection", Type: "bitmap"}
	assert.Equal(t, "bitmap", pg.Type)
}

func TestBMSInfoGroups(t *testing.T) {
	groups := BMSInfoGroups()
	require.GreaterOrEqual(t, len(groups), 1)
	bmsInfo := groups[0]
	assert.Equal(t, "BMS Info", bmsInfo.Name)

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
	assert.True(t, foundClockHi, "BMSInfoGroups missing probe at 0x9004 (clock hi)")
	assert.True(t, foundSN, "BMSInfoGroups missing SN probe at 0x9024 (Count 10, IsASCII)")

	// Check key probes exist
	expectedAddrs := []uint16{0x9004, 0x9005, 0x9006, 0x9007, 0x900B, 0x900C, 0x900D, 0x900E, 0x900F, 0x9010, 0x9011, 0x9012, 0x9013, 0x9024, 0x9018, 0x9019, 0x901A, 0x901B}
	addrSet := make(map[uint16]bool)
	for _, p := range bmsInfo.Probes {
		addrSet[p.Addr] = true
	}
	for _, addr := range expectedAddrs {
		assert.True(t, addrSet[addr])
	}
}

func TestBMSProtectionProbes(t *testing.T) {
	probes := BMSProtectionProbes()
	require.Len(t, probes, 6)
	expectedAddrs := []uint16{0x9014, 0x9015, 0x9016, 0x9017, 0x901C, 0x901D}
	for i, addr := range expectedAddrs {
		assert.Equal(t, addr, probes[i].Addr)
		assert.Equal(t, uint16(1), probes[i].Count)
	}
}

func TestStatisticsGroups(t *testing.T) {
	groups := StatisticsGroups()
	require.Len(t, groups, 4)

	expectedNames := []string{"Today", "Total", "This Month", "This Year"}
	for i, want := range expectedNames {
		assert.Equal(t, want, groups[i].Name)
	}

	// Each group has 6 probes, all U32=true, Count=2
	for _, g := range groups {
		assert.Len(t, g.Probes, 6)
		for _, p := range g.Probes {
			assert.True(t, p.U32)
			assert.Equal(t, uint16(2), p.Count)
			assert.Equal(t, "kWh", p.Unit)
		}
	}

	// Today scale = 0.01
	for _, p := range groups[0].Probes {
		assert.Equal(t, 0.01, p.Scale)
	}

	// Total, Month, Year scale = 0.1
	for i := 1; i < 4; i++ {
		for _, p := range groups[i].Probes {
			assert.Equal(t, 0.1, p.Scale)
		}
	}
}

func TestStatisticsAddresses(t *testing.T) {
	groups := StatisticsGroups()

	// Today starts at 0x0684
	assert.Equal(t, uint16(0x0684), groups[0].Probes[0].Addr)
	// Total starts at 0x0686
	assert.Equal(t, uint16(0x0686), groups[1].Probes[0].Addr)
	// This Month starts at 0x069C
	assert.Equal(t, uint16(0x069C), groups[2].Probes[0].Addr)
	// This Year starts at 0x069E
	assert.Equal(t, uint16(0x069E), groups[3].Probes[0].Addr)

	// Stride 4 between metrics within each group
	// Today: gen=0x0684, consumption=0x0688, bought=0x068C, sold=0x0690, bat_charge=0x0694, bat_discharge=0x0698
	todayExpected := []uint16{0x0684, 0x0688, 0x068C, 0x0690, 0x0694, 0x0698}
	for i, addr := range todayExpected {
		assert.Equal(t, addr, groups[0].Probes[i].Addr)
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

func TestDecodeBMSClock(t *testing.T) {
	// Encode 2026-04-10 14:03:05
	var val uint32 = 0x6914E0C5
	got := DecodeBMSClock(val)
	want := "2026-04-10 14:03:05"
	assert.Equal(t, want, got)
}

func TestDecodeTopology(t *testing.T) {
	parallelStrings, packsPerString := DecodeTopology(0x020A)
	assert.Equal(t, 2, parallelStrings)
	assert.Equal(t, 10, packsPerString)
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
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPackRTProbes(t *testing.T) {
	probes := PackRTProbes()
	assert.GreaterOrEqual(t, len(probes), 30)

	// First probe should be Pack ID at 0x9044
	assert.Equal(t, "Pack ID", probes[0].Name)
	assert.Equal(t, uint16(0x9044), probes[0].Addr)

	// Build lookup by name for specific checks
	byName := make(map[string]Probe)
	for _, p := range probes {
		byName[p.Name] = p
	}

	// Serial Number: ASCII, 10 registers
	if p, ok := byName["Serial Number"]; !ok {
		assert.Fail(t, "missing Serial Number probe")
	} else {
		assert.Equal(t, uint16(0x9047), p.Addr)
		assert.Equal(t, uint16(10), p.Count)
		assert.True(t, p.IsASCII, "Serial Number should be ASCII")
	}

	// Total Voltage
	if p, ok := byName["Total Voltage"]; !ok {
		assert.Fail(t, "missing Total Voltage probe")
	} else {
		assert.Equal(t, uint16(0x9079), p.Addr)
		assert.Equal(t, 0.1, p.Scale)
		assert.Equal(t, "V", p.Unit)
	}

	// Check Cell 1
	if p, ok := byName["Cell 1"]; !ok {
		assert.Fail(t, "missing Cell 1 probe")
	} else {
		assert.Equal(t, uint16(0x9051), p.Addr)
		assert.Equal(t, 0.001, p.Scale)
		assert.Equal(t, "V", p.Unit)
	}
	// Check Cell 16 (last cell per D-05)
	if p, ok := byName["Cell 16"]; !ok {
		assert.Fail(t, "missing Cell 16 probe")
	} else {
		assert.Equal(t, uint16(0x9060), p.Addr)
		assert.Equal(t, 0.001, p.Scale)
	}

	// Cell 17 should NOT exist (only 16 cells per D-05)
	if _, ok := byName["Cell 17"]; ok {
		assert.Fail(t, "Cell 17 should not exist (only 16 cells per D-05)")
	}

	// Verify exactly 16 cell voltage probes (D-05)
	cellCount := 0
	for _, p := range probes {
		if strings.HasPrefix(p.Name, "Cell ") && p.Scale == 0.001 && p.Unit == "V" {
			cellCount++
		}
	}
	assert.Equal(t, 16, cellCount)

	// Current: signed, scale 0.1, unit A
	if p, ok := byName["Current"]; !ok {
		assert.Fail(t, "missing Current probe")
	} else {
		assert.Equal(t, uint16(0x9071), p.Addr)
		assert.True(t, p.Signed, "Current should be Signed")
		assert.Equal(t, 0.1, p.Scale)
		assert.Equal(t, "A", p.Unit)
	}

	// Temp 1-4 at 0x906B-0x906E, Signed, Scale 0.1, Unit C
	tempAddrs := map[string]uint16{"Temp 1": 0x906B, "Temp 2": 0x906C, "Temp 3": 0x906D, "Temp 4": 0x906E}
	for name, wantAddr := range tempAddrs {
		p, ok := byName[name]
		if !assert.True(t, ok) {
			continue
		}
		assert.Equal(t, wantAddr, p.Addr)
		assert.True(t, p.Signed)
		assert.Equal(t, 0.1, p.Scale)
	}

	// MOS Temp and Env Temp: signed, scale 0.1
	if p, ok := byName["MOS Temp"]; !ok {
		assert.Fail(t, "missing MOS Temp probe")
	} else {
		assert.Equal(t, uint16(0x906F), p.Addr)
		assert.True(t, p.Signed, "MOS Temp should be Signed")
		assert.Equal(t, 0.1, p.Scale)
	}
	if p, ok := byName["Env Temp"]; !ok {
		assert.Fail(t, "missing Env Temp probe")
	} else {
		assert.Equal(t, uint16(0x9070), p.Addr)
		assert.True(t, p.Signed, "Env Temp should be Signed")
		assert.Equal(t, 0.1, p.Scale)
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
		if !assert.True(t, ok) {
			continue
		}
		assert.Equal(t, wantAddr, p.Addr)
	}

	// Min/Max Cell Voltage
	if p, ok := byName["Min Cell Voltage"]; !ok {
		assert.Fail(t, "missing Min Cell Voltage probe")
	} else {
		assert.Equal(t, uint16(0x906A), p.Addr)
		assert.Equal(t, 0.001, p.Scale)
		assert.Equal(t, "V", p.Unit)
	}
	if p, ok := byName["Max Cell Voltage"]; !ok {
		assert.Fail(t, "missing Max Cell Voltage probe")
	} else {
		assert.Equal(t, uint16(0x9069), p.Addr)
		assert.Equal(t, 0.001, p.Scale)
	}
}

func TestPackInfoProbes(t *testing.T) {
	probes := PackInfoProbes()
	assert.GreaterOrEqual(t, len(probes), 6)

	byName := make(map[string]Probe)
	for _, p := range probes {
		byName[p.Name] = p
	}

	// SOH
	if p, ok := byName["SOH"]; !ok {
		assert.Fail(t, "missing SOH probe")
	} else {
		assert.Equal(t, uint16(0x910A), p.Addr)
		assert.Equal(t, 0.1, p.Scale)
		assert.Equal(t, "%", p.Unit)
	}

	// Rated Capacity
	if p, ok := byName["Rated Capacity"]; !ok {
		assert.Fail(t, "missing Rated Capacity probe")
	} else {
		assert.Equal(t, uint16(0x910B), p.Addr)
		assert.Equal(t, 0.1, p.Scale)
		assert.Equal(t, "Ah", p.Unit)
	}

	// Manufacturer
	if p, ok := byName["Manufacturer"]; !ok {
		assert.Fail(t, "missing Manufacturer probe")
	} else {
		assert.Equal(t, uint16(0x9106), p.Addr)
		assert.Equal(t, uint16(4), p.Count)
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
		if !assert.True(t, ok) {
			continue
		}
		assert.Equal(t, wantAddr, p.Addr)
	}
}

func TestPackTemps58Probes(t *testing.T) {
	probes := PackTemps58Probes()
	require.Len(t, probes, 4)

	wantAddrs := []uint16{0x90BC, 0x90BD, 0x90BE, 0x90BF}
	for i, p := range probes {
		wantName := "Temp " + string(rune('5'+i))
		assert.Equal(t, wantName, p.Name)
		assert.Equal(t, wantAddrs[i], p.Addr)
		assert.True(t, p.Signed)
		assert.Equal(t, 0.1, p.Scale)
		assert.Equal(t, "\u00b0C", p.Unit)
	}
}

func TestBMSAlarmTable(t *testing.T) {
	require.NotEmpty(t, BMSAlarmBits)

	// Check for cell OV alarm at bit 0
	found := false
	for _, fb := range BMSAlarmBits {
		if fb.Addr == 0x9076 && fb.Bit == 0 {
			assert.Contains(t, fb.Desc, "Cell")
			assert.Contains(t, fb.Desc, "OV")
			found = true
		}
	}
	assert.True(t, found, "missing BMSAlarmBits entry at Addr=0x9076 Bit=0")

	// Check for cell UV alarm at bit 1
	found = false
	for _, fb := range BMSAlarmBits {
		if fb.Addr == 0x9076 && fb.Bit == 1 {
			assert.Contains(t, fb.Desc, "Cell")
			assert.Contains(t, fb.Desc, "UV")
			found = true
		}
	}
	assert.True(t, found, "missing BMSAlarmBits entry at Addr=0x9076 Bit=1")
}

func TestBMSProtectionTable(t *testing.T) {
	require.NotEmpty(t, BMSProtectionBits)

	// Check for cell OV protection at bit 0
	found := false
	for _, fb := range BMSProtectionBits {
		if fb.Addr == 0x9077 && fb.Bit == 0 {
			assert.Contains(t, fb.Desc, "Cell")
			assert.Contains(t, fb.Desc, "OV")
			found = true
		}
	}
	assert.True(t, found, "missing BMSProtectionBits entry at Addr=0x9077 Bit=0")
}

func TestBMSFaultTable_Pack(t *testing.T) {
	require.NotEmpty(t, BMSFaultBits)

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
			assert.Equal(t, tt.exact, got)
		}
		for _, sub := range tt.contains {
			assert.Contains(t, got, sub)
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
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDecodeBMSBitmap(t *testing.T) {
	// Use BMSAlarmBits for testing bitmap decoding
	// Bit 0 and bit 1 set at address 0x9076
	result := DecodeBMSBitmap(0x0003, BMSAlarmBits, 0x9076)
	assert.Len(t, result, 2)

	// No bits set
	result = DecodeBMSBitmap(0x0000, BMSAlarmBits, 0x9076)
	assert.Len(t, result, 0)
}

func TestPackProbeGroupOrder(t *testing.T) {
	groups := PackProbeGroups()

	// Exactly 5 groups
	require.Len(t, groups, 5)

	// Group names in correct order (D-03)
	wantNames := []string{"Pack Info", "Cell Voltages", "Balance State", "Temperatures", "Pack Status"}
	for i, want := range wantNames {
		assert.Equal(t, want, groups[i].Name)
	}

	// Group types
	wantTypes := []string{"", "cell_grid", "balance", "", "pack_status"}
	for i, want := range wantTypes {
		assert.Equal(t, want, groups[i].Type)
	}

	// Cell Voltages group: 16 cell probes + Max Cell Voltage + Min Cell Voltage = 18 probes
	cellGroup := groups[1]
	require.Len(t, cellGroup.Probes, 18)
	for i := 0; i < 16; i++ {
		wantName := fmt.Sprintf("Cell %d", i+1)
		assert.Equal(t, wantName, cellGroup.Probes[i].Name)
	}

	// Balance State group: probe at 0x9075
	balanceGroup := groups[2]
	require.Len(t, balanceGroup.Probes, 1)
	assert.Equal(t, uint16(0x9075), balanceGroup.Probes[0].Addr)

	// Temperatures group: Temp 1-4 (0x906B-0x906E), MOS Temp (0x906F), Env Temp (0x9070), Temp 5-8 (0x90BC-0x90BF) = 10 probes
	tempGroup := groups[3]
	require.Len(t, tempGroup.Probes, 10)
	wantTempAddrs := []uint16{0x906B, 0x906C, 0x906D, 0x906E, 0x906F, 0x9070, 0x90BC, 0x90BD, 0x90BE, 0x90BF}
	for i, wantAddr := range wantTempAddrs {
		assert.Equal(t, wantAddr, tempGroup.Probes[i].Addr)
	}

	// Pack Status group: 6 probes at 0x9076, 0x9077, 0x9078, 0x9124, 0x9125, 0x9126
	statusGroup := groups[4]
	require.Len(t, statusGroup.Probes, 6)
	wantStatusAddrs := []uint16{0x9076, 0x9077, 0x9078, 0x9124, 0x9125, 0x9126}
	for i, wantAddr := range wantStatusAddrs {
		assert.Equal(t, wantAddr, statusGroup.Probes[i].Addr)
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
		assert.True(t, gotInfoAddrs[addr])
	}
	assert.Len(t, infoGroup.Probes, len(wantInfoAddrs))
}
