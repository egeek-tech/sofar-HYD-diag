//go:build config_sweep

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

// SweepResult is the top-level JSON output of a configuration register sweep.
type SweepResult struct {
	Timestamp      string        `json:"timestamp"`
	Host           string        `json:"host"`
	Port           int           `json:"port"`
	SlaveID        int           `json:"slave_id"`
	TotalRegisters int           `json:"total_registers"`
	Passed         int           `json:"passed"`
	Failed         int           `json:"failed"`
	Errors         int           `json:"errors"`
	Groups         []GroupResult `json:"groups"`
}

// GroupResult holds the sweep results for a single ProbeGroup.
type GroupResult struct {
	Name   string        `json:"name"`
	Probes []ProbeResult `json:"probes"`
}

// ProbeResult holds the sweep result for a single probe/register.
type ProbeResult struct {
	Name   string `json:"name"`
	Addr   string `json:"addr"`
	Count  uint16 `json:"count"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

func main() {
	host := flag.String("host", "", "Inverter IP address (required)")
	port := flag.Int("port", 4192, "Modbus TCP port")
	slaveID := flag.Int("slave", 1, "Modbus slave ID")
	flag.Parse()

	if *host == "" {
		fmt.Fprintf(os.Stderr, "Usage: config-sweep -host <inverter-ip> [-port 4192] [-slave 1]\n")
		os.Exit(1)
	}

	addr := fmt.Sprintf("%s:%d", *host, *port)
	conn, err := modbus.Connect(addr)
	if err != nil {
		log.Fatalf("Failed to connect to %s: %v", addr, err)
	}
	defer conn.Close()

	logger := modbus.DiscardLogger()

	// Count total probes across all configuration groups
	groups := register.ConfigurationGroups
	totalProbes := 0
	for _, g := range groups {
		totalProbes += len(g.Probes)
	}
	fmt.Fprintf(os.Stderr, "Sweeping %d registers across %d groups...\n", totalProbes, len(groups))

	var passed, failed, errors int
	var groupResults []GroupResult

	for _, g := range groups {
		gr := GroupResult{Name: g.Name}

		for _, p := range g.Probes {
			// Skip synthetic probes (Count == 0)
			if p.Count == 0 {
				continue
			}

			_, readErr := modbus.ReadHoldingRegistersTCP(conn, logger, byte(*slaveID), p.Addr, p.Count)

			pr := ProbeResult{
				Name:  p.Name,
				Addr:  fmt.Sprintf("0x%04X", p.Addr),
				Count: p.Count,
			}

			switch {
			case readErr == nil:
				pr.Status = "PASS"
				passed++
			case strings.Contains(readErr.Error(), "err=0x02"):
				pr.Status = "FAIL"
				pr.Error = "illegal data address (0x02)"
				failed++
			default:
				pr.Status = "ERROR"
				pr.Error = readErr.Error()
				errors++

				// Attempt reconnection on transient errors
				conn.Close()
				conn, err = modbus.Connect(addr)
				if err != nil {
					log.Fatalf("Failed to reconnect to %s: %v", addr, err)
				}
			}

			fmt.Fprintf(os.Stderr, "[%s] %s / %s (0x%04X)\n", pr.Status, g.Name, p.Name, p.Addr)

			gr.Probes = append(gr.Probes, pr)

			// Enforce 500ms inter-read delay (hardware timing constraint)
			time.Sleep(500 * time.Millisecond)
		}

		groupResults = append(groupResults, gr)
	}

	// Build final result
	result := SweepResult{
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
		Host:           *host,
		Port:           *port,
		SlaveID:        *slaveID,
		TotalRegisters: totalProbes,
		Passed:         passed,
		Failed:         failed,
		Errors:         errors,
		Groups:         groupResults,
	}

	// Marshal and write JSON to stdout
	out, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal JSON: %v", err)
	}
	fmt.Println(string(out))

	fmt.Fprintf(os.Stderr, "\nSweep complete: %d passed, %d failed, %d errors\n", passed, failed, errors)
}
