package register

import (
	"encoding/binary"
	"testing"
)

func TestFormatValueASCII(t *testing.T) {
	p := Probe{Name: "Test SN", Addr: 0x0445, Count: 10, IsASCII: true}
	data := []byte("AMASS\x00\x00\x00")
	got := FormatValue(p, data)
	want := `"AMASS"`
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
