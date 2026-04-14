package register

// ProbeMapping maps a single Probe to its position within a BatchSpan.
// ByteOffset is (probe.Addr - span.StartAddr) * 2, ByteLength is probe.Count * 2.
type ProbeMapping struct {
	Probe      Probe
	GroupName  string
	ByteOffset int
	ByteLength int
}

// BatchSpan represents a contiguous range of registers that can be read in a
// single Modbus request. Probes contains the individual probe mappings within
// the span, with byte offsets for extracting each probe's data from the response.
type BatchSpan struct {
	StartAddr  uint16
	TotalCount uint16
	Probes     []ProbeMapping
}

// MaxBatchRegisters is the maximum number of registers in a single Modbus read (protocol limit).
const MaxBatchRegisters = 60

// BatchPlan is the pre-computed batch read strategy for a section.
// Spans are contiguous register ranges; Unbatchable holds synthetic probes (Count==0).
type BatchPlan struct {
	Spans       []BatchSpan
	Unbatchable []ProbeMapping
}

// AnalyzeBatchPlan analyzes ProbeGroups to produce a BatchPlan that merges
// contiguous register addresses into BatchSpans for efficient batch reading.
// Spans never exceed MaxBatchRegisters. Synthetic probes (Count==0) are placed
// in Unbatchable.
func AnalyzeBatchPlan(groups []ProbeGroup) BatchPlan {
	if len(groups) == 0 {
		return BatchPlan{}
	}

	// Collect all real probes sorted by address, track synthetics separately
	type taggedProbe struct {
		probe     Probe
		groupName string
	}
	var real []taggedProbe
	var synthetic []ProbeMapping

	for _, g := range groups {
		for _, p := range g.Probes {
			if p.Count == 0 {
				synthetic = append(synthetic, ProbeMapping{
					Probe:     p,
					GroupName: g.Name,
				})
				continue
			}
			real = append(real, taggedProbe{probe: p, groupName: g.Name})
		}
	}

	if len(real) == 0 {
		return BatchPlan{Unbatchable: synthetic}
	}

	// Sort by address
	for i := 1; i < len(real); i++ {
		for j := i; j > 0 && real[j].probe.Addr < real[j-1].probe.Addr; j-- {
			real[j], real[j-1] = real[j-1], real[j]
		}
	}

	// Build spans from sorted probes
	var spans []BatchSpan
	current := BatchSpan{
		StartAddr:  real[0].probe.Addr,
		TotalCount: real[0].probe.Count,
		Probes: []ProbeMapping{{
			Probe:      real[0].probe,
			GroupName:  real[0].groupName,
			ByteOffset: 0,
			ByteLength: int(real[0].probe.Count) * 2,
		}},
	}

	for i := 1; i < len(real); i++ {
		p := real[i]
		spanEnd := current.StartAddr + current.TotalCount
		nextAddr := p.probe.Addr

		// Check if probe is contiguous or overlapping with current span
		if nextAddr <= spanEnd {
			// Contiguous or overlapping
			probeEnd := nextAddr + p.probe.Count
			newTotal := current.TotalCount
			if probeEnd > spanEnd {
				newTotal = probeEnd - current.StartAddr
			}

			// Check if adding would exceed max
			if newTotal > MaxBatchRegisters {
				spans = append(spans, current)
				current = BatchSpan{
					StartAddr:  p.probe.Addr,
					TotalCount: p.probe.Count,
					Probes: []ProbeMapping{{
						Probe:      p.probe,
						GroupName:  p.groupName,
						ByteOffset: 0,
						ByteLength: int(p.probe.Count) * 2,
					}},
				}
				continue
			}

			current.TotalCount = newTotal
			current.Probes = append(current.Probes, ProbeMapping{
				Probe:      p.probe,
				GroupName:  p.groupName,
				ByteOffset: int(nextAddr-current.StartAddr) * 2,
				ByteLength: int(p.probe.Count) * 2,
			})
		} else {
			// Gap -- start new span
			spans = append(spans, current)
			current = BatchSpan{
				StartAddr:  p.probe.Addr,
				TotalCount: p.probe.Count,
				Probes: []ProbeMapping{{
					Probe:      p.probe,
					GroupName:  p.groupName,
					ByteOffset: 0,
					ByteLength: int(p.probe.Count) * 2,
				}},
			}
		}
	}
	spans = append(spans, current)

	return BatchPlan{
		Spans:       spans,
		Unbatchable: synthetic,
	}
}
