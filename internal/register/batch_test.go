package register

import (
	"fmt"
	"testing"
)

func TestAnalyzeBatchPlan_Contiguous(t *testing.T) {
	groups := []ProbeGroup{{
		Name: "Grid General",
		Probes: []Probe{
			{Name: "Grid frequency", Addr: 0x0484, Count: 1},
			{Name: "Total active power", Addr: 0x0485, Count: 1},
		},
	}}
	plan := AnalyzeBatchPlan(groups)
	if len(plan.Spans) != 1 {
		t.Fatalf("len(Spans) = %d, want 1", len(plan.Spans))
	}
	span := plan.Spans[0]
	if span.StartAddr != 0x0484 {
		t.Errorf("StartAddr = 0x%04X, want 0x0484", span.StartAddr)
	}
	if span.TotalCount != 2 {
		t.Errorf("TotalCount = %d, want 2", span.TotalCount)
	}
	if len(span.Probes) != 2 {
		t.Fatalf("len(Probes) = %d, want 2", len(span.Probes))
	}
	if span.Probes[0].ByteOffset != 0 {
		t.Errorf("Probes[0].ByteOffset = %d, want 0", span.Probes[0].ByteOffset)
	}
	if span.Probes[1].ByteOffset != 2 {
		t.Errorf("Probes[1].ByteOffset = %d, want 2", span.Probes[1].ByteOffset)
	}
}

func TestAnalyzeBatchPlan_Gap(t *testing.T) {
	groups := []ProbeGroup{{
		Name: "Status",
		Probes: []Probe{
			{Name: "Running state", Addr: 0x0404, Count: 1},
			{Name: "Grid-connected wait time", Addr: 0x0417, Count: 1},
		},
	}}
	plan := AnalyzeBatchPlan(groups)
	if len(plan.Spans) != 2 {
		t.Fatalf("len(Spans) = %d, want 2", len(plan.Spans))
	}
	if plan.Spans[0].StartAddr != 0x0404 {
		t.Errorf("Spans[0].StartAddr = 0x%04X, want 0x0404", plan.Spans[0].StartAddr)
	}
	if plan.Spans[0].TotalCount != 1 {
		t.Errorf("Spans[0].TotalCount = %d, want 1", plan.Spans[0].TotalCount)
	}
	if plan.Spans[1].StartAddr != 0x0417 {
		t.Errorf("Spans[1].StartAddr = 0x%04X, want 0x0417", plan.Spans[1].StartAddr)
	}
	if plan.Spans[1].TotalCount != 1 {
		t.Errorf("Spans[1].TotalCount = %d, want 1", plan.Spans[1].TotalCount)
	}
}

func TestAnalyzeBatchPlan_CrossGroup(t *testing.T) {
	// D-01: Grid General (0x0484-0x0487) and PCC Power (0x0488-0x048B) merge across groups
	groups := []ProbeGroup{
		{Name: "General", Probes: []Probe{
			{Name: "Grid frequency", Addr: 0x0484, Count: 1},
			{Name: "Total active power", Addr: 0x0485, Count: 1},
			{Name: "Total reactive power", Addr: 0x0486, Count: 1},
			{Name: "Total apparent power", Addr: 0x0487, Count: 1},
		}},
		{Name: "PCC Power", Probes: []Probe{
			{Name: "PCC active power", Addr: 0x0488, Count: 1},
			{Name: "PCC reactive power", Addr: 0x0489, Count: 1},
			{Name: "PCC apparent power", Addr: 0x048A, Count: 1},
			{Name: "PCC active power 2", Addr: 0x048B, Count: 1},
		}},
	}
	plan := AnalyzeBatchPlan(groups)
	if len(plan.Spans) != 1 {
		t.Fatalf("len(Spans) = %d, want 1", len(plan.Spans))
	}
	span := plan.Spans[0]
	if span.StartAddr != 0x0484 {
		t.Errorf("StartAddr = 0x%04X, want 0x0484", span.StartAddr)
	}
	if span.TotalCount != 8 {
		t.Errorf("TotalCount = %d, want 8", span.TotalCount)
	}
	if len(span.Probes) != 8 {
		t.Fatalf("len(Probes) = %d, want 8", len(span.Probes))
	}
}

func TestAnalyzeBatchPlan_MultiRegister(t *testing.T) {
	// U32 probes with Count=2 each
	groups := []ProbeGroup{{
		Name: "Statistics",
		Probes: []Probe{
			{Name: "Power Generation", Addr: 0x0684, Count: 2, U32: true, Unit: "kWh", Scale: 0.01},
			{Name: "Load Consumption", Addr: 0x0686, Count: 2, U32: true, Unit: "kWh", Scale: 0.01},
		},
	}}
	plan := AnalyzeBatchPlan(groups)
	if len(plan.Spans) != 1 {
		t.Fatalf("len(Spans) = %d, want 1", len(plan.Spans))
	}
	span := plan.Spans[0]
	if span.TotalCount != 4 {
		t.Errorf("TotalCount = %d, want 4", span.TotalCount)
	}
	if len(span.Probes) != 2 {
		t.Fatalf("len(Probes) = %d, want 2", len(span.Probes))
	}
	// Second probe at 0x0686: ByteOffset = (0x0686-0x0684)*2 = 4
	if span.Probes[1].ByteOffset != 4 {
		t.Errorf("Probes[1].ByteOffset = %d, want 4", span.Probes[1].ByteOffset)
	}
	// Each U32 probe has ByteLength = Count*2 = 4
	if span.Probes[0].ByteLength != 4 {
		t.Errorf("Probes[0].ByteLength = %d, want 4", span.Probes[0].ByteLength)
	}
	if span.Probes[1].ByteLength != 4 {
		t.Errorf("Probes[1].ByteLength = %d, want 4", span.Probes[1].ByteLength)
	}
}

func TestAnalyzeBatchPlan_ASCII(t *testing.T) {
	groups := []ProbeGroup{{
		Name: "Mixed",
		Probes: []Probe{
			{Name: "Inverter SN", Addr: 0x0445, Count: 10, IsASCII: true},
			{Name: "Comm SW version", Addr: 0x044F, Count: 4, IsASCII: true},
		},
	}}
	plan := AnalyzeBatchPlan(groups)
	if len(plan.Spans) != 1 {
		t.Fatalf("len(Spans) = %d, want 1", len(plan.Spans))
	}
	span := plan.Spans[0]
	if span.TotalCount != 14 {
		t.Errorf("TotalCount = %d, want 14", span.TotalCount)
	}
	if len(span.Probes) != 2 {
		t.Fatalf("len(Probes) = %d, want 2", len(span.Probes))
	}
	// 0x044F: ByteOffset = (0x044F - 0x0445)*2 = 20
	if span.Probes[1].ByteOffset != 20 {
		t.Errorf("Probes[1].ByteOffset = %d, want 20", span.Probes[1].ByteOffset)
	}
}

func TestAnalyzeBatchPlan_MaxRegisters(t *testing.T) {
	// SAFE-02: 61 contiguous single-register probes should split into 60 + 1
	var probes []Probe
	for i := 0; i < 61; i++ {
		probes = append(probes, Probe{Name: fmt.Sprintf("Reg%d", i), Addr: 0x1000 + uint16(i), Count: 1})
	}
	groups := []ProbeGroup{{Name: "Test", Probes: probes}}
	plan := AnalyzeBatchPlan(groups)
	if len(plan.Spans) != 2 {
		t.Fatalf("len(Spans) = %d, want 2", len(plan.Spans))
	}
	if plan.Spans[0].TotalCount != 60 {
		t.Errorf("Spans[0].TotalCount = %d, want 60", plan.Spans[0].TotalCount)
	}
	if plan.Spans[1].TotalCount != 1 {
		t.Errorf("Spans[1].TotalCount = %d, want 1", plan.Spans[1].TotalCount)
	}
	if len(plan.Spans[0].Probes) != 60 {
		t.Errorf("Spans[0] probe count = %d, want 60", len(plan.Spans[0].Probes))
	}
	if len(plan.Spans[1].Probes) != 1 {
		t.Errorf("Spans[1] probe count = %d, want 1", len(plan.Spans[1].Probes))
	}
	if plan.Spans[1].StartAddr != 0x103C {
		t.Errorf("Spans[1].StartAddr = 0x%04X, want 0x103C", plan.Spans[1].StartAddr)
	}
}

func TestAnalyzeBatchPlan_SyntheticProbe(t *testing.T) {
	groups := []ProbeGroup{{
		Name: "Status",
		Probes: []Probe{
			{Name: "System time", Addr: 0x042C, Count: 0},
		},
	}}
	plan := AnalyzeBatchPlan(groups)
	if len(plan.Spans) != 0 {
		t.Errorf("len(Spans) = %d, want 0", len(plan.Spans))
	}
	if len(plan.Unbatchable) != 1 {
		t.Fatalf("len(Unbatchable) = %d, want 1", len(plan.Unbatchable))
	}
	if plan.Unbatchable[0].Probe.Addr != 0x042C {
		t.Errorf("Unbatchable[0].Probe.Addr = 0x%04X, want 0x042C", plan.Unbatchable[0].Probe.Addr)
	}
}

func TestAnalyzeBatchPlan_MixedSyntheticAndReal(t *testing.T) {
	groups := []ProbeGroup{{
		Name: "Mixed",
		Probes: []Probe{
			{Name: "System time", Addr: 0x042C, Count: 0},
			{Name: "Running state", Addr: 0x0404, Count: 1},
		},
	}}
	plan := AnalyzeBatchPlan(groups)
	if len(plan.Spans) != 1 {
		t.Fatalf("len(Spans) = %d, want 1", len(plan.Spans))
	}
	if plan.Spans[0].StartAddr != 0x0404 {
		t.Errorf("Spans[0].StartAddr = 0x%04X, want 0x0404", plan.Spans[0].StartAddr)
	}
	if len(plan.Unbatchable) != 1 {
		t.Fatalf("len(Unbatchable) = %d, want 1", len(plan.Unbatchable))
	}
	if plan.Unbatchable[0].Probe.Addr != 0x042C {
		t.Errorf("Unbatchable[0].Probe.Addr = 0x%04X, want 0x042C", plan.Unbatchable[0].Probe.Addr)
	}
}

func TestAnalyzeBatchPlan_EmptyGroups(t *testing.T) {
	plan := AnalyzeBatchPlan([]ProbeGroup{})
	if len(plan.Spans) != 0 {
		t.Errorf("len(Spans) = %d, want 0", len(plan.Spans))
	}
	if len(plan.Unbatchable) != 0 {
		t.Errorf("len(Unbatchable) = %d, want 0", len(plan.Unbatchable))
	}
}

func TestAnalyzeBatchPlan_RealSystemSection(t *testing.T) {
	// SystemGroups from system.go: 6 groups with real register addresses.
	// After D-03: System time is a real probe (Count: 6, Composite: "system_time").
	// It merges with insulation impedance (0x042B) into a 7-register span.
	// Span 0: 0x0404 (1 reg) -- Running state
	// Span 1: 0x0417 (4 regs) -- Grid wait + Temps (0x0417,0x0418,0x0419,0x041A)
	// Span 2: 0x0420 (1 reg) -- Module temp
	// Span 3: 0x0426 (1 reg) -- Power gen time
	// Span 4: 0x042B (7 regs) -- Insulation impedance + System time (0x042B-0x0431)
	// Span 5: 0x043E (1 reg) -- Fan speed
	// Span 6: 0x0445 (31 regs) -- Identity+Firmware+FirmwareExt (0x0445 to 0x0463)
	// Span 7: 0x0467 (6 regs) -- Safety package ver
	// Unbatchable: none (was 1 for synthetic system time)
	plan := AnalyzeBatchPlan(SystemGroups)
	if len(plan.Spans) != 8 {
		t.Fatalf("len(Spans) = %d, want 8", len(plan.Spans))
	}
	if len(plan.Unbatchable) != 0 {
		t.Fatalf("len(Unbatchable) = %d, want 0", len(plan.Unbatchable))
	}

	// Verify key spans
	if plan.Spans[0].StartAddr != 0x0404 {
		t.Errorf("Span 0 StartAddr = 0x%04X, want 0x0404", plan.Spans[0].StartAddr)
	}
	if plan.Spans[0].TotalCount != 1 {
		t.Errorf("Span 0 TotalCount = %d, want 1", plan.Spans[0].TotalCount)
	}

	// Span 1: 0x0417 with 4 regs (grid wait time + 3 temps)
	if plan.Spans[1].StartAddr != 0x0417 {
		t.Errorf("Span 1 StartAddr = 0x%04X, want 0x0417", plan.Spans[1].StartAddr)
	}
	if plan.Spans[1].TotalCount != 4 {
		t.Errorf("Span 1 TotalCount = %d, want 4", plan.Spans[1].TotalCount)
	}

	// Span 4: 0x042B with 7 regs (insulation impedance + system time merged)
	if plan.Spans[4].StartAddr != 0x042B {
		t.Errorf("Span 4 StartAddr = 0x%04X, want 0x042B", plan.Spans[4].StartAddr)
	}
	if plan.Spans[4].TotalCount != 7 {
		t.Errorf("Span 4 TotalCount = %d, want 7", plan.Spans[4].TotalCount)
	}
	if len(plan.Spans[4].Probes) != 2 {
		t.Errorf("Span 4 probe count = %d, want 2", len(plan.Spans[4].Probes))
	}

	// Span 6: large merged span starting at 0x0445
	if plan.Spans[6].StartAddr != 0x0445 {
		t.Errorf("Span 6 StartAddr = 0x%04X, want 0x0445", plan.Spans[6].StartAddr)
	}
	if plan.Spans[6].TotalCount != 31 {
		t.Errorf("Span 6 TotalCount = %d, want 31", plan.Spans[6].TotalCount)
	}

	// Span 7: safety package ver at 0x0467
	if plan.Spans[7].StartAddr != 0x0467 {
		t.Errorf("Span 7 StartAddr = 0x%04X, want 0x0467", plan.Spans[7].StartAddr)
	}
	if plan.Spans[7].TotalCount != 6 {
		t.Errorf("Span 7 TotalCount = %d, want 6", plan.Spans[7].TotalCount)
	}
}

func TestAnalyzeBatchPlan_ByteOffsets(t *testing.T) {
	// Span 0x0484-0x048B (8 regs), probe at 0x0488 has ByteOffset=8 and ByteLength=2
	groups := []ProbeGroup{
		{Name: "General", Probes: []Probe{
			{Name: "Grid frequency", Addr: 0x0484, Count: 1},
			{Name: "Total active power", Addr: 0x0485, Count: 1},
			{Name: "Total reactive power", Addr: 0x0486, Count: 1},
			{Name: "Total apparent power", Addr: 0x0487, Count: 1},
		}},
		{Name: "PCC Power", Probes: []Probe{
			{Name: "PCC active power", Addr: 0x0488, Count: 1},
			{Name: "PCC reactive power", Addr: 0x0489, Count: 1},
			{Name: "PCC apparent power", Addr: 0x048A, Count: 1},
			{Name: "PCC active power 2", Addr: 0x048B, Count: 1},
		}},
	}
	plan := AnalyzeBatchPlan(groups)
	if len(plan.Spans) != 1 {
		t.Fatalf("len(Spans) = %d, want 1", len(plan.Spans))
	}
	span := plan.Spans[0]
	// Find probe at 0x0488
	found := false
	for _, pm := range span.Probes {
		if pm.Probe.Addr == 0x0488 {
			found = true
			if pm.ByteOffset != 8 {
				t.Errorf("Probe 0x0488 ByteOffset = %d, want 8", pm.ByteOffset)
			}
			if pm.ByteLength != 2 {
				t.Errorf("Probe 0x0488 ByteLength = %d, want 2", pm.ByteLength)
			}
			break
		}
	}
	if !found {
		t.Error("Probe 0x0488 not found in span")
	}
}

func TestAnalyzeBatchPlan_GroupNamePreserved(t *testing.T) {
	// Two groups with contiguous probes -- should merge into one span
	groups := []ProbeGroup{
		{Name: "General", Probes: []Probe{
			{Name: "Grid frequency", Addr: 0x0484, Count: 1},
		}},
		{Name: "PCC Power", Probes: []Probe{
			{Name: "PCC active power", Addr: 0x0485, Count: 1},
		}},
	}
	plan := AnalyzeBatchPlan(groups)
	if len(plan.Spans) != 1 {
		t.Fatalf("len(Spans) = %d, want 1 (cross-group merge)", len(plan.Spans))
	}
	// Even though merged into one span, each probe retains its source group name
	span := plan.Spans[0]
	if len(span.Probes) != 2 {
		t.Fatalf("len(Probes) = %d, want 2", len(span.Probes))
	}
	if span.Probes[0].GroupName != "General" {
		t.Errorf("Probes[0].GroupName = %q, want %q", span.Probes[0].GroupName, "General")
	}
	if span.Probes[1].GroupName != "PCC Power" {
		t.Errorf("Probes[1].GroupName = %q, want %q", span.Probes[1].GroupName, "PCC Power")
	}
}
