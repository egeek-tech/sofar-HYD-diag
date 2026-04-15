//go:build section_sweep

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"sofar-hyd-diag/internal/modbus"
	"sofar-hyd-diag/internal/register"
)

// SweepResult is the top-level JSON output of a section sweep.
type SweepResult struct {
	Timestamp  string          `json:"timestamp"`
	Host       string          `json:"host"`
	Port       int             `json:"port"`
	SlaveID    int             `json:"slave_id"`
	PVChannels int             `json:"pv_channels"`
	Sections   []SectionResult `json:"sections"`
	Summary    SummaryResult   `json:"summary"`
}

// SummaryResult aggregates pass/fail/error counts across all sections.
type SummaryResult struct {
	TotalSections int `json:"total_sections"`
	TotalProbes   int `json:"total_probes"`
	Passed        int `json:"passed"`
	Failed        int `json:"failed"`
	Errors        int `json:"errors"`
}

// SectionResult holds the sweep results for a single section (e.g. Grid, EPS).
type SectionResult struct {
	Name        string       `json:"name"`
	TotalProbes int          `json:"total_probes"`
	Passed      int          `json:"passed"`
	Failed      int          `json:"failed"`
	Errors      int          `json:"errors"`
	Spans       []SpanResult `json:"spans"`
}

// SpanResult holds the sweep results for a single batch span.
type SpanResult struct {
	StartAddr   string        `json:"start_addr"`
	TotalCount  uint16        `json:"total_count"`
	BatchStatus string        `json:"batch_status"`
	Probes      []ProbeResult `json:"probes"`
}

// ProbeResult holds the sweep result for a single probe/register.
type ProbeResult struct {
	Name   string `json:"name"`
	Group  string `json:"group"`
	Addr   string `json:"addr"`
	Count  uint16 `json:"count"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// sectionDef pairs a section name with its register groups.
type sectionDef struct {
	name   string
	groups []register.ProbeGroup
}

func main() {
	host := flag.String("host", "", "Inverter IP address (required)")
	port := flag.Int("port", 4192, "Modbus TCP port")
	slaveID := flag.Int("slave", 1, "Modbus slave ID")
	pvChannels := flag.Int("pv", 2, "Number of PV channels")
	flag.Parse()

	if *host == "" {
		fmt.Fprintf(os.Stderr, "Usage: section-sweep -host <inverter-ip> [-port 4192] [-slave 1] [-pv 2]\n")
		os.Exit(1)
	}

	addr := fmt.Sprintf("%s:%d", *host, *port)
	conn, err := modbus.Connect(addr)
	if err != nil {
		log.Fatalf("Failed to connect to %s: %v", addr, err)
	}

	logger := modbus.DiscardLogger()

	// Define all 7 standard sections.
	sections := []sectionDef{
		{"grid", register.GridGroups},
		{"eps", register.EPSGroups},
		{"pv", register.GeneratePVGroups(*pvChannels)},
		{"meter", register.MeterGroups},
		{"dcdc", register.DCDCGroups},
		{"pcu", register.PCUGroups},
		{"bdu", register.BDUGroups},
	}

	// Count total probes across all sections for progress reporting.
	totalProbes := 0
	for _, sec := range sections {
		for _, g := range sec.groups {
			for _, p := range g.Probes {
				if p.Count > 0 {
					totalProbes++
				}
			}
		}
	}
	fmt.Fprintf(os.Stderr, "Sweeping %d probes across %d sections...\n", totalProbes, len(sections))

	var sectionResults []SectionResult
	var summaryPassed, summaryFailed, summaryErrors int

	for _, sec := range sections {
		plan := register.AnalyzeBatchPlan(sec.groups)

		secResult := SectionResult{Name: sec.name}

		for _, span := range plan.Spans {
			spanResult := SpanResult{
				StartAddr:  fmt.Sprintf("0x%04X", span.StartAddr),
				TotalCount: span.TotalCount,
			}

			// Attempt batch read for the entire span.
			_, batchErr := modbus.ReadHoldingRegistersTCP(conn, logger, byte(*slaveID), span.StartAddr, span.TotalCount)

			if batchErr == nil {
				// Batch read succeeded -- all probes in this span pass.
				spanResult.BatchStatus = "PASS"
				for _, pm := range span.Probes {
					pr := ProbeResult{
						Name:   pm.Probe.Name,
						Group:  pm.GroupName,
						Addr:   fmt.Sprintf("0x%04X", pm.Probe.Addr),
						Count:  pm.Probe.Count,
						Status: "PASS",
					}
					spanResult.Probes = append(spanResult.Probes, pr)
					secResult.Passed++
					summaryPassed++
					fmt.Fprintf(os.Stderr, "[PASS] %s / %s (0x%04X)\n", sec.name, pm.Probe.Name, pm.Probe.Addr)
				}
			} else {
				// Batch read failed -- fall back to individual probe reads.
				spanResult.BatchStatus = "FAIL"
				fmt.Fprintf(os.Stderr, "[BATCH FAIL] %s span 0x%04X (%d regs): %v\n", sec.name, span.StartAddr, span.TotalCount, batchErr)

				// Handle reconnection if the batch error was a transient/connection error.
				if !strings.Contains(batchErr.Error(), "err=0x02") {
					conn.Close()
					conn, err = modbus.Connect(addr)
					if err != nil {
						log.Fatalf("Failed to reconnect to %s: %v", addr, err)
					}
					// Delay after reconnection before individual reads.
					time.Sleep(500 * time.Millisecond)
				}

				for _, pm := range span.Probes {
					_, readErr := modbus.ReadHoldingRegistersTCP(conn, logger, byte(*slaveID), pm.Probe.Addr, pm.Probe.Count)

					pr := ProbeResult{
						Name:  pm.Probe.Name,
						Group: pm.GroupName,
						Addr:  fmt.Sprintf("0x%04X", pm.Probe.Addr),
						Count: pm.Probe.Count,
					}

					switch {
					case readErr == nil:
						pr.Status = "PASS"
						secResult.Passed++
						summaryPassed++
					case strings.Contains(readErr.Error(), "err=0x02"):
						pr.Status = "FAIL"
						pr.Error = "illegal data address (0x02)"
						secResult.Failed++
						summaryFailed++
					default:
						pr.Status = "ERROR"
						pr.Error = readErr.Error()
						secResult.Errors++
						summaryErrors++

						// Attempt reconnection on transient errors.
						conn.Close()
						conn, err = modbus.Connect(addr)
						if err != nil {
							log.Fatalf("Failed to reconnect to %s: %v", addr, err)
						}
					}

					fmt.Fprintf(os.Stderr, "[%s] %s / %s (0x%04X)\n", pr.Status, sec.name, pm.Probe.Name, pm.Probe.Addr)
					spanResult.Probes = append(spanResult.Probes, pr)

					// Enforce 500ms inter-read delay (hardware timing constraint).
					time.Sleep(500 * time.Millisecond)
				}
			}

			secResult.Spans = append(secResult.Spans, spanResult)

			// Enforce 500ms inter-read delay after each batch read.
			time.Sleep(500 * time.Millisecond)
		}

		// Count total probes for this section (non-synthetic only).
		for _, g := range sec.groups {
			for _, p := range g.Probes {
				if p.Count > 0 {
					secResult.TotalProbes++
				}
			}
		}

		sectionResults = append(sectionResults, secResult)
	}

	// Close connection explicitly (conn may have been reassigned during reconnects).
	conn.Close()

	// Build final result.
	result := SweepResult{
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Host:       *host,
		Port:       *port,
		SlaveID:    *slaveID,
		PVChannels: *pvChannels,
		Sections:   sectionResults,
		Summary: SummaryResult{
			TotalSections: len(sections),
			TotalProbes:   totalProbes,
			Passed:        summaryPassed,
			Failed:        summaryFailed,
			Errors:        summaryErrors,
		},
	}

	// Marshal and write JSON to stdout.
	out, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal JSON: %v", err)
	}
	fmt.Println(string(out))

	fmt.Fprintf(os.Stderr, "\nSweep complete: %d sections, %d passed, %d failed, %d errors\n",
		len(sections), summaryPassed, summaryFailed, summaryErrors)
}
