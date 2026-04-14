package register

import "sort"

// MaxBatchRegisters is the maximum number of registers in a single Modbus read request.
// Sofar protocol limit: 60 registers per function 0x03 call.
const MaxBatchRegisters = 60

// ProbeMapping maps a single probe to its byte position within a batch response.
type ProbeMapping struct {
	Probe      Probe
	GroupName  string
	ByteOffset int // (probe.Addr - span.StartAddr) * 2
	ByteLength int // probe.Count * 2
}

// BatchSpan represents a contiguous range of registers readable in one Modbus request.
type BatchSpan struct {
	StartAddr  uint16
	TotalCount uint16
	Probes     []ProbeMapping
}

// BatchPlan is the complete batch read strategy for a section.
type BatchPlan struct {
	Spans       []BatchSpan
	Unbatchable []ProbeMapping
}

// annotatedProbe pairs a probe with its source group name for flattening.
type annotatedProbe struct {
	probe     Probe
	groupName string
}

// AnalyzeBatchPlan analyzes probe groups and produces a batch read plan.
// It merges contiguous register ranges across group boundaries into BatchSpans,
// enforces the MaxBatchRegisters limit per span, and separates synthetic probes
// (Count == 0) into the Unbatchable list.
//
// This is a pure function with no I/O or side effects.
func AnalyzeBatchPlan(groups []ProbeGroup) BatchPlan {
	// Flatten all probes from all groups.
	var real []annotatedProbe
	var unbatchable []ProbeMapping

	for _, g := range groups {
		for _, p := range g.Probes {
			if p.Count == 0 {
				unbatchable = append(unbatchable, ProbeMapping{
					Probe:     p,
					GroupName: g.Name,
				})
				continue
			}
			real = append(real, annotatedProbe{probe: p, groupName: g.Name})
		}
	}

	if len(real) == 0 {
		return BatchPlan{Unbatchable: unbatchable}
	}

	// Sort by address ascending.
	sort.Slice(real, func(i, j int) bool {
		return real[i].probe.Addr < real[j].probe.Addr
	})

	// Walk sorted probes building spans.
	var spans []BatchSpan
	cur := BatchSpan{
		StartAddr:  real[0].probe.Addr,
		TotalCount: real[0].probe.Count,
		Probes: []ProbeMapping{{
			Probe:      real[0].probe,
			GroupName:  real[0].groupName,
			ByteOffset: 0,
			ByteLength: int(real[0].probe.Count) * 2,
		}},
	}

	for _, ap := range real[1:] {
		spanEnd := cur.StartAddr + cur.TotalCount // next expected address after current span

		if ap.probe.Addr < spanEnd {
			// OVERLAP: probe starts inside the current span's range.
			// Add the mapping with correct byte offset.
			pm := ProbeMapping{
				Probe:      ap.probe,
				GroupName:  ap.groupName,
				ByteOffset: int(ap.probe.Addr-cur.StartAddr) * 2,
				ByteLength: int(ap.probe.Count) * 2,
			}
			cur.Probes = append(cur.Probes, pm)
			// Extend TotalCount only if the overlapping probe extends beyond current end.
			probeEnd := ap.probe.Addr + ap.probe.Count
			if probeEnd > spanEnd {
				cur.TotalCount = probeEnd - cur.StartAddr
			}
		} else if ap.probe.Addr == spanEnd {
			// CONTIGUOUS: check if extending would exceed limit.
			newTotal := cur.TotalCount + ap.probe.Count
			if newTotal > MaxBatchRegisters {
				// Close current span and start a new one.
				spans = append(spans, cur)
				cur = BatchSpan{
					StartAddr:  ap.probe.Addr,
					TotalCount: ap.probe.Count,
					Probes: []ProbeMapping{{
						Probe:      ap.probe,
						GroupName:  ap.groupName,
						ByteOffset: 0,
						ByteLength: int(ap.probe.Count) * 2,
					}},
				}
			} else {
				// Extend the current span.
				pm := ProbeMapping{
					Probe:      ap.probe,
					GroupName:  ap.groupName,
					ByteOffset: int(ap.probe.Addr-cur.StartAddr) * 2,
					ByteLength: int(ap.probe.Count) * 2,
				}
				cur.Probes = append(cur.Probes, pm)
				cur.TotalCount = newTotal
			}
		} else {
			// GAP: close current span and start a new one.
			spans = append(spans, cur)
			cur = BatchSpan{
				StartAddr:  ap.probe.Addr,
				TotalCount: ap.probe.Count,
				Probes: []ProbeMapping{{
					Probe:      ap.probe,
					GroupName:  ap.groupName,
					ByteOffset: 0,
					ByteLength: int(ap.probe.Count) * 2,
				}},
			}
		}
	}

	// Close the final span.
	spans = append(spans, cur)

	return BatchPlan{
		Spans:       spans,
		Unbatchable: unbatchable,
	}
}
