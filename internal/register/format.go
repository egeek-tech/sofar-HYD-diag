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

// ComposeSystemTime takes the 6 time register values and returns a formatted datetime string.
// The year value is offset by 2000 (e.g., 26 -> 2026).
func ComposeSystemTime(year, month, day, hour, min, sec uint16) string {
	return fmt.Sprintf("%04d-%02d-%02d %02d:%02d:%02d", year+2000, month, day, hour, min, sec)
}
