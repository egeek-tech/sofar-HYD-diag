//go:build xlsx_discover

package main

import (
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"sofar-hyd-diag/internal/register"
)

// ComparisonEntry holds the three-way comparison for a single register address.
type ComparisonEntry struct {
	Addr   uint16
	XLSX   *RegisterInfo // nil if not in XLSX
	V138   *RegisterInfo // nil if not in V1.38
	Probes *RegisterInfo // nil if not in current probes
}

// compare merges three register maps into a sorted slice of ComparisonEntry.
func compare(xlsx, v138, probes map[uint16]*RegisterInfo) []ComparisonEntry {
	addrs := make(map[uint16]struct{})
	for a := range xlsx {
		addrs[a] = struct{}{}
	}
	for a := range v138 {
		addrs[a] = struct{}{}
	}
	for a := range probes {
		addrs[a] = struct{}{}
	}

	entries := make([]ComparisonEntry, 0, len(addrs))
	for a := range addrs {
		entries = append(entries, ComparisonEntry{
			Addr:   a,
			XLSX:   xlsx[a],
			V138:   v138[a],
			Probes: probes[a],
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Addr < entries[j].Addr
	})

	return entries
}

// statusLabel returns a human-readable status for a ComparisonEntry.
func statusLabel(e ComparisonEntry) string {
	hasXLSX := e.XLSX != nil
	hasV138 := e.V138 != nil
	hasProbes := e.Probes != nil

	switch {
	case hasXLSX && hasV138 && hasProbes:
		return "all three"
	case hasXLSX && hasV138 && !hasProbes:
		return "missing from probes"
	case hasXLSX && !hasV138 && hasProbes:
		return "XLSX+probes"
	case hasXLSX && !hasV138 && !hasProbes:
		return "XLSX only"
	case !hasXLSX && hasV138 && hasProbes:
		return "V1.38+probes"
	case !hasXLSX && hasV138 && !hasProbes:
		return "V1.38 only"
	case !hasXLSX && !hasV138 && hasProbes:
		return "probes only"
	default:
		return "unknown"
	}
}

// addressGroup defines a named address range for grouping output.
type addressGroup struct {
	name    string
	minAddr uint16
	maxAddr uint16
}

var groups = []addressGroup{
	{"General (0x00xx)", 0x0000, 0x00FF},
	{"System (0x04xx)", 0x0400, 0x04FF},
	{"EPS/PV (0x05xx)", 0x0500, 0x05FF},
	{"Battery (0x06xx)", 0x0600, 0x06FF},
	{"Arc (0x07xx)", 0x0700, 0x707F},
	{"Meter (0x70xx)", 0x7080, 0x70FF},
	{"Safety (0x08xx-0x0Axx)", 0x0800, 0x0AFF},
	{"Config (0x10xx-0x13xx)", 0x1000, 0x13FF},
	{"Network/Control (0x20xx)", 0x2000, 0x20FF},
	{"Compatible (0x4Exx)", 0x4E00, 0x4EFF},
	{"DCDC (0x50xx-0x54xx)", 0x5000, 0x54FF},
	{"PCU (0x60xx)", 0x6000, 0x607F},
	{"BDU (0x60xx)", 0x6080, 0x60FF},
	{"BMS (0x90xx-0x91xx)", 0x9000, 0x91FF},
}

func groupName(addr uint16) string {
	for _, g := range groups {
		if addr >= g.minAddr && addr <= g.maxAddr {
			return g.name
		}
	}
	return fmt.Sprintf("Other (0x%04X)", addr&0xFF00)
}

// entryName returns the name from the first non-nil source, truncated to maxLen.
func entryName(e ComparisonEntry, maxLen int) string {
	var name string
	switch {
	case e.V138 != nil:
		name = e.V138.Name
	case e.XLSX != nil:
		name = e.XLSX.Name
	case e.Probes != nil:
		name = e.Probes.Name
	}
	if len(name) > maxLen {
		name = name[:maxLen-3] + "..."
	}
	return name
}

func yesOrDash(v *RegisterInfo) string {
	if v != nil {
		return "yes"
	}
	return "-"
}

// printComparison outputs the grouped three-way comparison table.
func printComparison(entries []ComparisonEntry) {
	// Group entries
	grouped := make(map[string][]ComparisonEntry)
	for _, e := range entries {
		g := groupName(e.Addr)
		grouped[g] = append(grouped[g], e)
	}

	// Print in group order
	for _, g := range groups {
		ents, ok := grouped[g.name]
		if !ok || len(ents) == 0 {
			continue
		}
		fmt.Printf("\n=== %s ===\n", g.name)
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ADDR\tNAME\tXLSX\tV1.38\tPROBES\tSTATUS")
		fmt.Fprintln(w, "----\t----\t----\t-----\t------\t------")
		for _, e := range ents {
			fmt.Fprintf(w, "0x%04X\t%s\t%s\t%s\t%s\t%s\n",
				e.Addr,
				entryName(e, 30),
				yesOrDash(e.XLSX),
				yesOrDash(e.V138),
				yesOrDash(e.Probes),
				statusLabel(e),
			)
		}
		w.Flush()
	}

	// Print "Other" groups not in the predefined list
	for gName, ents := range grouped {
		found := false
		for _, g := range groups {
			if g.name == gName {
				found = true
				break
			}
		}
		if found || len(ents) == 0 {
			continue
		}
		fmt.Printf("\n=== %s ===\n", gName)
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ADDR\tNAME\tXLSX\tV1.38\tPROBES\tSTATUS")
		fmt.Fprintln(w, "----\t----\t----\t-----\t------\t------")
		for _, e := range ents {
			fmt.Fprintf(w, "0x%04X\t%s\t%s\t%s\t%s\t%s\n",
				e.Addr,
				entryName(e, 30),
				yesOrDash(e.XLSX),
				yesOrDash(e.V138),
				yesOrDash(e.Probes),
				statusLabel(e),
			)
		}
		w.Flush()
	}

	// Summary
	var xlsxCount, v138Count, probeCount, uniqueCount, missingCount int
	for _, e := range entries {
		uniqueCount++
		if e.XLSX != nil {
			xlsxCount++
		}
		if e.V138 != nil {
			v138Count++
		}
		if e.Probes != nil {
			probeCount++
		}
		if statusLabel(e) == "missing from probes" {
			missingCount++
		}
	}

	fmt.Printf("\n--- Summary ---\n")
	fmt.Printf("XLSX registers:       %d\n", xlsxCount)
	fmt.Printf("V1.38 registers:      %d\n", v138Count)
	fmt.Printf("Probe registers:      %d\n", probeCount)
	fmt.Printf("Total unique:         %d\n", uniqueCount)
	fmt.Printf("Missing from probes:  %d\n", missingCount)
}

// currentProbeAddrs collects all register addresses from the current codebase probe definitions.
func currentProbeAddrs() map[uint16]*RegisterInfo {
	result := make(map[uint16]*RegisterInfo)

	addProbe := func(p register.Probe) {
		dataType := "U16"
		switch {
		case p.IsASCII:
			dataType = "ASCII"
		case p.U32:
			dataType = "U32"
		case p.Signed:
			dataType = "S16"
		}
		result[p.Addr] = &RegisterInfo{
			Addr:   p.Addr,
			Name:   p.Name,
			Type:   dataType,
			Scale:  p.Scale,
			Unit:   p.Unit,
			RW:     "R",
			Source: "probes",
		}
	}

	addGroups := func(gs []register.ProbeGroup) {
		for _, g := range gs {
			for _, p := range g.Probes {
				addProbe(p)
			}
		}
	}

	// All exported group variables
	addGroups(register.SystemGroups)
	addGroups(register.GridGroups)
	addGroups(register.EPSGroups)
	addGroups(register.ConfigurationGroups)
	addGroups(register.GeneratePVGroups(16))
	addGroups(register.GenerateBatteryGroups(8))
	addGroups(register.StatisticsGroups())
	addGroups(register.BMSInfoGroups())
	addGroups(register.MeterGroups)
	addGroups(register.DCDCGroups)
	addGroups(register.PCUGroups)
	addGroups(register.BDUGroups)
	addGroups(register.InternalInfoGroups())

	// Flat probe functions
	for _, p := range register.PackRTProbes() {
		addProbe(p)
	}
	for _, p := range register.PackInfoProbes() {
		addProbe(p)
	}
	for _, p := range register.PackTemps58Probes() {
		addProbe(p)
	}
	addGroups(register.PackProbeGroups())

	// Fault registers
	for _, p := range register.FaultRegisters {
		addProbe(p)
	}

	return result
}
