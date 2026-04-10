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
}
