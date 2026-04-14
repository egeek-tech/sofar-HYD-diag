//go:build xlsx_discover

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

func main() {
	xlsxPath := flag.String("xlsx", "", "Path to the Sofar XLSX register map file (required)")
	flag.Parse()

	if *xlsxPath == "" {
		fmt.Fprintf(os.Stderr, "Usage: xlsx-discover -xlsx <path-to-xlsx>\n")
		os.Exit(1)
	}

	// Parse XLSX
	xlsx, err := parseXLSX(*xlsxPath)
	if err != nil {
		log.Fatalf("Failed to parse XLSX: %v", err)
	}
	fmt.Printf("Parsed %d registers from XLSX\n", len(xlsx))

	// Load current probe definitions
	probes := currentProbeAddrs()
	fmt.Printf("Found %d registers in current probes\n", len(probes))

	// V1.38 hardcoded map
	v138 := V138Registers
	fmt.Printf("V1.38 reference has %d registers\n", len(v138))

	// Three-way comparison
	entries := compare(xlsx, v138, probes)
	printComparison(entries)

	fmt.Printf("\nDone. Use this report to identify registers for integration.\n")
}
