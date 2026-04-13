package register

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// FormatValue interprets raw register bytes according to the probe's type configuration
// and returns a human-readable string. Returns the formatted value only (no prefix/address).
// Extracted from main.go.bak formatResult() lines 433-460.
func FormatValue(p Probe, data []byte) string {
	if p.IsASCII {
		s := string(data)
		if i := strings.IndexByte(s, 0); i >= 0 {
			s = s[:i]
		}
		return strings.TrimRight(s, " ")
	}

	// U32: 32-bit unsigned from two consecutive registers.
	// Sofar word order: low address = high word, high address = low word.
	if p.U32 {
		if len(data) < 4 {
			return "<no data>"
		}
		val := uint32(binary.BigEndian.Uint16(data[:2]))<<16 | uint32(binary.BigEndian.Uint16(data[2:4]))
		if p.Scale > 0 {
			scaled := float64(val) * p.Scale
			if p.Unit != "" {
				return fmt.Sprintf("%.2f %s", scaled, p.Unit)
			}
			return fmt.Sprintf("%.3f", scaled)
		}
		return fmt.Sprintf("%d (0x%08X)", val, val)
	}

	if len(data) < 2 {
		return "<no data>"
	}

	// Enum lookup: if the probe has an enum map, try to resolve the value to a label.
	if p.Enum != nil {
		val := binary.BigEndian.Uint16(data[:2])
		if label, ok := p.Enum[val]; ok {
			return label
		}
		// Fall through to default formatting if enum value unknown
	}

	if p.Signed {
		val := int16(binary.BigEndian.Uint16(data[:2]))
		if p.Scale > 0 {
			scaled := float64(val) * p.Scale
			if p.Unit != "" {
				return fmt.Sprintf("%.2f %s", scaled, p.Unit)
			}
			return fmt.Sprintf("%.3f", scaled)
		}
		return fmt.Sprintf("%d", val)
	}

	val := binary.BigEndian.Uint16(data[:2])
	if p.Scale > 0 {
		scaled := float64(val) * p.Scale
		if p.Unit != "" {
			return fmt.Sprintf("%.2f %s", scaled, p.Unit)
		}
		return fmt.Sprintf("%.3f", scaled)
	}
	return fmt.Sprintf("%d (0x%04X)", val, val)
}

// FormatRawValue extracts the raw numeric representation of register data
// before scaling or formatting. Returns a decimal string for numeric probes
// or hex string for ASCII probes. Returns empty string for insufficient data.
func FormatRawValue(p Probe, data []byte) string {
	if len(data) < 2 {
		return ""
	}
	if p.IsASCII {
		return fmt.Sprintf("%X", data)
	}
	if p.U32 && len(data) >= 4 {
		val := uint32(binary.BigEndian.Uint16(data[:2]))<<16 | uint32(binary.BigEndian.Uint16(data[2:4]))
		return fmt.Sprintf("%d", val)
	}
	val := binary.BigEndian.Uint16(data[:2])
	return fmt.Sprintf("%d", val)
}

// ComposeSystemTime takes the 6 time register values and returns a formatted datetime string.
// The year value is offset by 2000 (e.g., 26 -> 2026).
func ComposeSystemTime(year, month, day, hour, min, sec uint16) string {
	return fmt.Sprintf("%02d:%02d:%02d %02d-%02d-%04d", hour, min, sec, day, month, year+2000)
}

// DecodeBMSClock unpacks a 32-bit BMS system clock value into a formatted datetime string.
// The packed format stores fields as bit ranges within a single uint32:
// bits [5:0]=seconds, [11:6]=minutes, [16:12]=hours, [21:17]=day, [25:22]=month, [31:26]=year(offset 2000).
func DecodeBMSClock(val uint32) string {
	sec := val & 0x3F
	min := (val >> 6) & 0x3F
	hr := (val >> 12) & 0x1F
	day := (val >> 17) & 0x1F
	mon := (val >> 22) & 0x0F
	yr := (val >> 26) & 0x3F
	return fmt.Sprintf("%04d-%02d-%02d %02d:%02d:%02d", 2000+yr, mon, day, hr, min, sec)
}

// DecodeTopology unpacks a 16-bit topology parameter register (0x900D) into
// the number of parallel strings (high byte) and packs per string (low byte).
func DecodeTopology(val uint16) (parallelStrings int, packsPerString int) {
	parallelStrings = int(val >> 8)
	packsPerString = int(val & 0xFF)
	return
}

// DecodeBalanceState interprets a 16-bit balance state register (0x9075).
// Each bit corresponds to a cell (bit 0 = Cell 1, etc.). If zero, all cells
// are balanced. Otherwise returns a comma-separated list of balancing cells.
func DecodeBalanceState(val uint16) string {
	if val == 0 {
		return "Balanced"
	}
	var cells []string
	for i := 0; i < 16; i++ {
		if val&(1<<uint(i)) != 0 {
			cells = append(cells, fmt.Sprintf("Cell %d", i+1))
		}
	}
	return "Balancing: " + strings.Join(cells, ", ")
}
