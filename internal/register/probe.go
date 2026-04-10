package register

// Probe defines metadata for reading and formatting a single register group.
// Extracted from main.go.bak lines 77-85 with PascalCase exported field names.
type Probe struct {
	Name    string
	Addr    uint16
	Count   uint16
	IsASCII bool
	Signed  bool
	Unit    string
	Scale   float64
	Enum    map[uint16]string // optional: value -> human-readable label (D-04)
	U32     bool              // when true, Count must be 2; FormatValue reads 4 bytes as 32-bit unsigned
}
