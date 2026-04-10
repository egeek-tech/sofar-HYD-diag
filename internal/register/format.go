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
		return fmt.Sprintf("%q", strings.TrimRight(string(data), "\x00 "))
	}

	if len(data) < 2 {
		return "<no data>"
	}

	if p.Signed {
		val := int16(binary.BigEndian.Uint16(data[:2]))
		if p.Scale > 0 && p.Unit != "" {
			return fmt.Sprintf("%.2f %s", float64(val)*p.Scale, p.Unit)
		}
		return fmt.Sprintf("%d", val)
	}

	val := binary.BigEndian.Uint16(data[:2])
	if p.Scale > 0 && p.Unit != "" {
		return fmt.Sprintf("%.2f %s", float64(val)*p.Scale, p.Unit)
	}
	return fmt.Sprintf("%d (0x%04X)", val, val)
}
