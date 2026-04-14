//go:build xlsx_discover

package main

import (
	"os"
	"testing"
)

func TestV138RegistersNotEmpty(t *testing.T) {
	if len(V138Registers) < 100 {
		t.Errorf("expected V138Registers to have >100 entries, got %d", len(V138Registers))
	}
}

func TestV138RegistersHasKeyAddresses(t *testing.T) {
	keys := []uint16{
		0x0404, // system running state
		0x0484, // grid frequency
		0x5044, // DCDC parallel count
		0x6004, // PCU online
		0x6084, // BDU online
		0x7080, // meter auto-ID
		0x9012, // BMS SOC
	}
	for _, k := range keys {
		if _, ok := V138Registers[k]; !ok {
			t.Errorf("V138Registers missing key address 0x%04X", k)
		}
	}
}

func TestCurrentProbeAddrsNotEmpty(t *testing.T) {
	probes := currentProbeAddrs()
	if len(probes) < 50 {
		t.Errorf("expected >50 probe addresses, got %d", len(probes))
	}
}

func TestCurrentProbeAddrsHasSystemRegisters(t *testing.T) {
	probes := currentProbeAddrs()
	keys := []uint16{0x0404, 0x0484}
	for _, k := range keys {
		if _, ok := probes[k]; !ok {
			t.Errorf("probes missing system register 0x%04X", k)
		}
	}
}

func TestCompareAllThree(t *testing.T) {
	xlsx := map[uint16]*RegisterInfo{
		0x0001: {Addr: 0x0001, Name: "Reg A", Source: "xlsx"},
		0x0002: {Addr: 0x0002, Name: "Reg B", Source: "xlsx"},
		0x0003: {Addr: 0x0003, Name: "Reg C", Source: "xlsx"},
	}
	v138 := map[uint16]*RegisterInfo{
		0x0002: {Addr: 0x0002, Name: "Reg B", Source: "v138"},
		0x0003: {Addr: 0x0003, Name: "Reg C", Source: "v138"},
		0x0004: {Addr: 0x0004, Name: "Reg D", Source: "v138"},
	}
	probes := map[uint16]*RegisterInfo{
		0x0003: {Addr: 0x0003, Name: "Reg C", Source: "probes"},
		0x0004: {Addr: 0x0004, Name: "Reg D", Source: "probes"},
		0x0005: {Addr: 0x0005, Name: "Reg E", Source: "probes"},
	}

	entries := compare(xlsx, v138, probes)

	if len(entries) != 5 {
		t.Fatalf("expected 5 entries, got %d", len(entries))
	}
	// Should be sorted by address
	if entries[0].Addr != 0x0001 {
		t.Errorf("first entry should be 0x0001, got 0x%04X", entries[0].Addr)
	}
	if entries[4].Addr != 0x0005 {
		t.Errorf("last entry should be 0x0005, got 0x%04X", entries[4].Addr)
	}
	// Check 0x0003 has all three
	e3 := entries[2]
	if e3.XLSX == nil || e3.V138 == nil || e3.Probes == nil {
		t.Errorf("0x0003 should have all three sources")
	}
	// Check 0x0001 is XLSX only
	if entries[0].XLSX == nil || entries[0].V138 != nil || entries[0].Probes != nil {
		t.Errorf("0x0001 should be XLSX only")
	}
}

func TestStatusLabel(t *testing.T) {
	tests := []struct {
		name   string
		entry  ComparisonEntry
		expect string
	}{
		{"all three", ComparisonEntry{XLSX: &RegisterInfo{}, V138: &RegisterInfo{}, Probes: &RegisterInfo{}}, "all three"},
		{"missing from probes", ComparisonEntry{XLSX: &RegisterInfo{}, V138: &RegisterInfo{}}, "missing from probes"},
		{"XLSX only", ComparisonEntry{XLSX: &RegisterInfo{}}, "XLSX only"},
		{"V1.38 only", ComparisonEntry{V138: &RegisterInfo{}}, "V1.38 only"},
		{"probes only", ComparisonEntry{Probes: &RegisterInfo{}}, "probes only"},
		{"XLSX+probes", ComparisonEntry{XLSX: &RegisterInfo{}, Probes: &RegisterInfo{}}, "XLSX+probes"},
		{"V1.38+probes", ComparisonEntry{V138: &RegisterInfo{}, Probes: &RegisterInfo{}}, "V1.38+probes"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := statusLabel(tt.entry)
			if got != tt.expect {
				t.Errorf("statusLabel() = %q, want %q", got, tt.expect)
			}
		})
	}
}

func TestParseXLSXRealFile(t *testing.T) {
	xlsxPath := "../../SOFAR_Modbus_Protocol_English_G3_V1.29(only 3PH).xlsx"
	if _, err := os.Stat(xlsxPath); os.IsNotExist(err) {
		t.Skip("XLSX file not found at project root, skipping real file test")
	}
	result, err := parseXLSX(xlsxPath)
	if err != nil {
		t.Fatalf("parseXLSX failed: %v", err)
	}
	if len(result) < 500 {
		t.Errorf("expected >500 registers from XLSX, got %d", len(result))
	}
}

func TestCompareRealData(t *testing.T) {
	xlsxPath := "../../SOFAR_Modbus_Protocol_English_G3_V1.29(only 3PH).xlsx"
	if _, err := os.Stat(xlsxPath); os.IsNotExist(err) {
		t.Skip("XLSX file not found at project root, skipping real data test")
	}
	xlsx, err := parseXLSX(xlsxPath)
	if err != nil {
		t.Fatalf("parseXLSX failed: %v", err)
	}
	probes := currentProbeAddrs()
	entries := compare(xlsx, V138Registers, probes)
	if len(entries) == 0 {
		t.Error("expected non-empty comparison entries")
	}
}
