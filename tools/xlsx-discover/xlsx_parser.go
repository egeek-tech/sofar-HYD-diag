//go:build xlsx_discover

package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// parseXLSX reads the XLSX register map and returns a map of address -> RegisterInfo.
func parseXLSX(path string) (map[uint16]*RegisterInfo, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("open XLSX: %w", err)
	}
	defer f.Close()

	rows, err := f.GetRows("Address Description")
	if err != nil {
		return nil, fmt.Errorf("get rows from 'Address Description': %w", err)
	}

	result := make(map[uint16]*RegisterInfo)

	for i, row := range rows {
		if i == 0 {
			continue // skip header
		}
		if len(row) < 2 {
			continue
		}

		addrStr := strings.TrimSpace(row[1])
		if addrStr == "" {
			continue
		}
		// Skip range entries like "0445 -- 044C"
		if strings.Contains(addrStr, "--") || strings.Contains(addrStr, " ") {
			continue
		}
		// Must be exactly 4 hex chars
		if len(addrStr) != 4 {
			continue
		}

		addr, err := strconv.ParseUint(addrStr, 16, 16)
		if err != nil {
			continue
		}

		// Column 2: field name
		name := ""
		if len(row) > 2 {
			name = strings.TrimSpace(row[2])
		}
		if name == "" {
			continue
		}

		// Column 4: data type
		dataType := "U16"
		if len(row) > 4 {
			dt := strings.TrimSpace(row[4])
			if dt != "" {
				// Map I16 -> S16 for consistency (Research Pitfall 5)
				dt = strings.ToUpper(dt)
				if dt == "I16" {
					dt = "S16"
				}
				dataType = dt
			}
		}

		// Column 5: scale/precision (take first line if multi-line)
		var scale float64
		if len(row) > 5 {
			scale = parseScale(row[5])
		}

		// Column 6: unit
		unit := ""
		if len(row) > 6 {
			unit = cleanUnit(strings.TrimSpace(row[6]))
		}

		// Column 9: R/RW/W
		rw := "R"
		if len(row) > 9 {
			rwVal := strings.TrimSpace(row[9])
			if rwVal != "" {
				rw = strings.ToUpper(rwVal)
			}
		}

		result[uint16(addr)] = &RegisterInfo{
			Addr:   uint16(addr),
			Name:   name,
			Type:   dataType,
			Scale:  scale,
			Unit:   unit,
			RW:     rw,
			Source: "xlsx",
		}
	}

	return result, nil
}

// parseScale extracts a numeric scale from a potentially multi-line string.
func parseScale(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	// Take first line only
	lines := strings.Split(s, "\n")
	s = strings.TrimSpace(lines[0])
	if s == "" {
		return 0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

// cleanUnit maps known CJK/Unicode unit strings to proper abbreviations.
func cleanUnit(s string) string {
	s = strings.TrimSpace(s)
	switch s {
	case "\u2103", "\u00b0C":
		return "\u00b0C"
	case "kvar", "Kvar":
		return "kVar"
	case "kva", "Kva":
		return "kVA"
	case "kw", "Kw":
		return "kW"
	case "kwh", "Kwh", "KWh":
		return "kWh"
	case "k\u03a9", "kOhm":
		return "k\u03a9"
	}
	return s
}
