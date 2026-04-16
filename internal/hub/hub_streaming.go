package hub

import (
	"context"
	"encoding/binary"
	"fmt"
	"strings"
	"time"

	"sofar-hyd-diag/internal/broker"
	"sofar-hyd-diag/internal/register"
)

// buildSectionSchema builds a SectionSchemaMessage from a section's Groups.
// Sent to clients on subscribe so the frontend can pre-render placeholder slots (STREAM-02, D-02).
func (h *Hub) buildSectionSchema(sectionName string, sec *Section) SectionSchemaMessage {
	var schemaGroups []SchemaGroup
	for _, g := range sec.Groups {
		sg := SchemaGroup{
			Name:   g.Name,
			Layout: g.Layout,
			Type:   g.Type,
		}
		for _, p := range g.Probes {
			sg.Registers = append(sg.Registers, p.Name)
		}
		schemaGroups = append(schemaGroups, sg)
	}
	return NewSectionSchema(sectionName, schemaGroups)
}

// readSpanIndividualFallback reads each probe in a span individually, emitting
// sectionResult per probe. Returns true if ALL individual reads failed (used by
// SpanTracker to decide individual-failure escalation). Returns early (with true)
// if readCtx is cancelled.
func (h *Hub) readSpanIndividualFallback(sectionName string, span register.BatchSpan, readCtx context.Context) bool {
	allFailed := true
	for _, pm := range span.Probes {
		if readCtx.Err() != nil {
			return true
		}
		indData, indErr := h.broker.ReadRegisters(readCtx, pm.Probe.Addr, pm.Probe.Count)
		var errStr, value, rawVal string
		if indErr != nil {
			errStr = indErr.Error()
		} else {
			allFailed = false
			value = FormatValue(pm.Probe, indData)
			rawVal = FormatRawValue(pm.Probe, indData)
		}
		if readCtx.Err() != nil {
			return true
		}
		h.results <- sectionResult{
			section: sectionName,
			msg:     NewRegisterValue(sectionName, pm.GroupName, pm.Probe.Name, value, errStr, pm.Probe.Addr, rawVal),
		}
	}
	return allFailed
}

// countBatteryChannels counts the number of battery channel groups by checking
// for the "Channel " name prefix. This is robust against additional non-channel
// groups (Global Stats, Internal Info) unlike the previous len(groups)-1 approach.
func countBatteryChannels(groups []register.ProbeGroup) int {
	count := 0
	for _, g := range groups {
		if strings.HasPrefix(g.Name, "Channel ") {
			count++
		}
	}
	return count
}

// streamStandardRead reads a standard section using batch spans from the pre-computed
// BatchPlan. Each span issues a single ReadRegisters call for the full contiguous range,
// then extracts individual probe values from the response. On span failure, falls back
// to individual probe reads (D-02). Section timing is logged at Info level (D-05, D-06).
func (h *Hub) streamStandardRead(sectionName string, sec *Section, readCtx context.Context) {
	isFault := sec.faultSection

	go func() {
		defer sec.reading.Store(false)

		start := time.Now()
		spanCount := len(sec.BatchPlan.Spans)
		var totalRegisters uint16

		// D-01 (Phase 22): Increment SpanTracker cycle counter for probe scheduling.
		if sec.SpanTracker != nil {
			sec.SpanTracker.Tick()
		}

		// D-01: Iterate batch spans instead of individual probes.
		// Every standard section gets batch reading automatically.
		for _, span := range sec.BatchPlan.Spans {
			if readCtx.Err() != nil {
				return
			}

			totalRegisters += span.TotalCount

			// Phase 22: Three-state degradation check.
			var state SpanState
			var shouldProbe bool
			if sec.SpanTracker != nil {
				state = sec.SpanTracker.State(span.StartAddr)
				shouldProbe = sec.SpanTracker.ShouldProbe(span.StartAddr)
			}

			// Fully skipped: no reads at all (D-03 state 3).
			// Cached values persist dimmed in the UI (D-06).
			if state == SpanSkipped && !shouldProbe {
				continue
			}

			// Degraded (not probing): skip batch, try individual reads only (D-03 state 2).
			if state == SpanDegraded && !shouldProbe {
				allIndivFailed := h.readSpanIndividualFallback(sectionName, span, readCtx)
				if readCtx.Err() != nil {
					return
				}
				// If ALL individual reads failed this cycle, record individual failure.
				// After 2 such cycles, span transitions to Skipped (D-03).
				if allIndivFailed && sec.SpanTracker != nil {
					sec.SpanTracker.RecordIndividualFailure(span.StartAddr)
				} else if !allIndivFailed && sec.SpanTracker != nil {
					// At least one individual read succeeded -- recover to Normal (D-08).
					sec.SpanTracker.RecordSuccess(span.StartAddr)
				}
				continue
			}

			// Normal OR probing: attempt batch read.
			data, err := h.broker.ReadRegisters(readCtx, span.StartAddr, span.TotalCount)
			if err != nil {
				if shouldProbe {
					// Probe failed -- stay in current state for degraded/skipped.
					// For normal spans that happen to be on a probe cycle, still record failure.
					if state == SpanNormal {
						if sec.SpanTracker != nil {
							sec.SpanTracker.RecordFailure(span.StartAddr)
						}
					}
					// WR-01 fix: Probe batch failed -- do individual fallback reads so
					// subscribers still get updated (or error) values this cycle,
					// regardless of whether the span is Normal, Degraded, or Skipped.
					h.logger.Warn("batch span failed, falling back to individual reads",
						"section", sectionName,
						"startAddr", fmt.Sprintf("0x%04X", span.StartAddr),
						"span_count", span.TotalCount,
						"error", err,
					)
					h.readSpanIndividualFallback(sectionName, span, readCtx)
					if readCtx.Err() != nil {
						return
					}
					continue
				}

				// Normal span batch failure (not a probe cycle): record failure, individual fallback.
				if sec.SpanTracker != nil {
					sec.SpanTracker.RecordFailure(span.StartAddr)
				}
				h.logger.Warn("batch span failed, falling back to individual reads",
					"section", sectionName,
					"startAddr", fmt.Sprintf("0x%04X", span.StartAddr),
					"span_count", span.TotalCount,
					"error", err,
				)
				h.readSpanIndividualFallback(sectionName, span, readCtx)
				if readCtx.Err() != nil {
					return
				}
				continue
			}

			// Batch read succeeded.
			if sec.SpanTracker != nil {
				sec.SpanTracker.RecordSuccess(span.StartAddr)
			}

			// WR-02: skip send if context cancelled after successful batch read
			if readCtx.Err() != nil {
				return
			}

			// Extract each probe's data from the batch response
			for _, pm := range span.Probes {
				end := pm.ByteOffset + pm.ByteLength
				if end > len(data) {
					h.results <- sectionResult{
						section: sectionName,
						msg:     NewRegisterValue(sectionName, pm.GroupName, pm.Probe.Name, "", "short response", pm.Probe.Addr, ""),
					}
					continue
				}
				probeData := data[pm.ByteOffset:end]
				value := FormatValue(pm.Probe, probeData)
				rawVal := FormatRawValue(pm.Probe, probeData)
				h.results <- sectionResult{
					section: sectionName,
					msg:     NewRegisterValue(sectionName, pm.GroupName, pm.Probe.Name, value, "", pm.Probe.Addr, rawVal),
				}
			}
		}

		// WR-02: bail out if context cancelled before fault/completion phase
		if readCtx.Err() != nil {
			return
		}

		// Read and decode faults for system section (streamed as a batch at the end)
		if isFault {
			var faultReads []broker.ReadRequest
			for _, fp := range register.FaultRegisters {
				faultReads = append(faultReads, broker.ReadRequest{Addr: fp.Addr, Count: fp.Count})
			}
			faultResults := h.broker.ReadBatch(readCtx, faultReads)
			faultData := make(map[uint16]uint16)
			for i, fp := range register.FaultRegisters {
				if i >= len(faultResults) || faultResults[i].Err != nil {
					continue
				}
				for reg := uint16(0); reg < fp.Count; reg++ {
					offset := int(reg) * 2
					if offset+2 <= len(faultResults[i].Data) {
						faultData[fp.Addr+reg] = binary.BigEndian.Uint16(faultResults[i].Data[offset : offset+2])
					}
				}
			}
			activeFaults := register.DecodeFaults(faultData)
			faultEntries := make([]FaultEntry, len(activeFaults))
			for i, desc := range activeFaults {
				faultEntries[i] = FaultEntry{Name: desc}
			}
			// Send fault data as a grouped section_data (only the faults card)
			h.results <- sectionResult{
				section: sectionName,
				msg: OutboundMessage{
					Type:    MsgTypeSectionData,
					Section: sectionName,
					Faults:  faultEntries,
				},
			}
		}

		// D-05, D-06: Log section read timing at Info level (always-on, no debug flag).
		h.logger.Info("section read complete",
			"section", sectionName,
			"duration_ms", time.Since(start).Milliseconds(),
			"spans", spanCount,
			"registers", totalRegisters,
		)

		// Signal completion
		h.results <- sectionResult{
			section: sectionName,
			msg:     NewSectionComplete(sectionName),
		}
	}()
}

// streamBatteryBatchRead reads the battery section using batch spans from the
// pre-computed BatchPlan. Before starting batch reads, it pre-reads register
// 0x066A individually to detect the active channel count. If the count changed,
// it rebuilds Groups/Probes/BatchPlan with InternalInfoGroups re-appended,
// resets SpanTracker, and re-triggers the read with the new plan (D-01 through D-05).
func (h *Hub) streamBatteryBatchRead(sec *Section, readCtx context.Context) {
	go func() {
		retrigger := false
		defer func() {
			if !retrigger {
				sec.reading.Store(false)
			}
		}()

		// Step 1: Pre-read 0x066A individually to detect channel count (D-03).
		// This is independent of SpanTracker -- always read even if the Global Stats
		// span is degraded/skipped. Do NOT emit sectionResult for this pre-read;
		// the batch span loop will emit the actual value (Pitfall 5).
		packCountData, err := h.broker.ReadRegisters(readCtx, 0x066A, 1)
		if err == nil && len(packCountData) >= 2 {
			detected := int(binary.BigEndian.Uint16(packCountData[:2]))
			if detected > 0 && detected <= 8 {
				currentChannels := countBatteryChannels(sec.Groups)
				if detected != currentChannels {
					// Channel count changed: rebuild with InternalInfoGroups (D-04, D-07).
					newGroups := append(register.GenerateBatteryGroups(detected), register.InternalInfoGroups()...)
					retrigger = true
					select {
					case h.funcs <- func() {
						sec.Groups = newGroups
						sec.Probes = flattenProbeGroups(newGroups)
						sec.BatchPlan = register.AnalyzeBatchPlan(newGroups)
						sec.SpanTracker.Reset() // D-05: reset since span addresses change
						h.logger.Info("battery section auto-detected channels", "channels", detected)
						h.triggerSectionRead("battery")
					}:
					case <-readCtx.Done():
						retrigger = false
					}
					return
				}
			}
		} else if err != nil {
			// 0x066A pre-read failed: log warning and continue with existing BatchPlan (Pitfall 6).
			h.logger.Warn("battery 0x066A pre-read failed, using existing plan", "error", err)
		}

		if readCtx.Err() != nil {
			return
		}

		// Step 2: SpanTracker tick (D-06).
		start := time.Now()
		spanCount := len(sec.BatchPlan.Spans)
		var totalRegisters uint16

		if sec.SpanTracker != nil {
			sec.SpanTracker.Tick()
		}

		// Step 3: Iterate batch spans with SpanTracker three-state degradation (D-02, D-06).
		for _, span := range sec.BatchPlan.Spans {
			if readCtx.Err() != nil {
				return
			}

			totalRegisters += span.TotalCount

			var state SpanState
			var shouldProbe bool
			if sec.SpanTracker != nil {
				state = sec.SpanTracker.State(span.StartAddr)
				shouldProbe = sec.SpanTracker.ShouldProbe(span.StartAddr)
			}

			// Fully skipped: no reads at all.
			if state == SpanSkipped && !shouldProbe {
				continue
			}

			// Degraded (not probing): individual reads only.
			if state == SpanDegraded && !shouldProbe {
				allIndivFailed := h.readSpanIndividualFallback("battery", span, readCtx)
				if readCtx.Err() != nil {
					return
				}
				if allIndivFailed && sec.SpanTracker != nil {
					sec.SpanTracker.RecordIndividualFailure(span.StartAddr)
				} else if !allIndivFailed && sec.SpanTracker != nil {
					sec.SpanTracker.RecordSuccess(span.StartAddr)
				}
				continue
			}

			// Normal or probing: attempt batch read.
			data, batchErr := h.broker.ReadRegisters(readCtx, span.StartAddr, span.TotalCount)
			if batchErr != nil {
				if shouldProbe {
					if state == SpanNormal {
						if sec.SpanTracker != nil {
							sec.SpanTracker.RecordFailure(span.StartAddr)
						}
					}
					h.logger.Warn("batch span failed, falling back to individual reads",
						"section", "battery",
						"startAddr", fmt.Sprintf("0x%04X", span.StartAddr),
						"span_count", span.TotalCount,
						"error", batchErr,
					)
					h.readSpanIndividualFallback("battery", span, readCtx)
					if readCtx.Err() != nil {
						return
					}
					continue
				}

				if sec.SpanTracker != nil {
					sec.SpanTracker.RecordFailure(span.StartAddr)
				}
				h.logger.Warn("batch span failed, falling back to individual reads",
					"section", "battery",
					"startAddr", fmt.Sprintf("0x%04X", span.StartAddr),
					"span_count", span.TotalCount,
					"error", batchErr,
				)
				h.readSpanIndividualFallback("battery", span, readCtx)
				if readCtx.Err() != nil {
					return
				}
				continue
			}

			// Batch read succeeded.
			if sec.SpanTracker != nil {
				sec.SpanTracker.RecordSuccess(span.StartAddr)
			}

			if readCtx.Err() != nil {
				return
			}

			// Extract each probe's data from the batch response.
			for _, pm := range span.Probes {
				end := pm.ByteOffset + pm.ByteLength
				if end > len(data) {
					h.results <- sectionResult{
						section: "battery",
						msg:     NewRegisterValue("battery", pm.GroupName, pm.Probe.Name, "", "short response", pm.Probe.Addr, ""),
					}
					continue
				}
				probeData := data[pm.ByteOffset:end]
				value := FormatValue(pm.Probe, probeData)
				rawVal := FormatRawValue(pm.Probe, probeData)
				h.results <- sectionResult{
					section: "battery",
					msg:     NewRegisterValue("battery", pm.GroupName, pm.Probe.Name, value, "", pm.Probe.Addr, rawVal),
				}
			}
		}

		if readCtx.Err() != nil {
			return
		}

		// Step 4: Log section read timing and emit section_complete (D-09).
		h.logger.Info("section read complete",
			"section", "battery",
			"duration_ms", time.Since(start).Milliseconds(),
			"spans", spanCount,
			"registers", totalRegisters,
		)

		h.results <- sectionResult{
			section: "battery",
			msg:     NewSectionComplete("battery"),
		}
	}()
}

// === Phase 11: Pack drill-down streaming ===

// buildPackSchema builds a SectionSchemaMessage for a pack drill-down with PackContext.
// The schema includes group metadata (types, cell counts) so the frontend can
// pre-render the correct skeleton layout before values stream in.
func (h *Hub) buildPackSchema(input, tower, pack int, groups []register.ProbeGroup) SectionSchemaMessage {
	var schemaGroups []SchemaGroup
	for _, g := range groups {
		sg := SchemaGroup{
			Name:   g.Name,
			Layout: g.Layout,
			Type:   g.Type,
		}
		if g.Type == "cell_grid" {
			// Count only actual cell probes (exclude MaxCell/MinCell summary probes)
			cellCount := 0
			for _, p := range g.Probes {
				if strings.HasPrefix(p.Name, "Cell ") {
					cellCount++
				}
			}
			sg.CellCount = cellCount
		}
		for _, p := range g.Probes {
			sg.Registers = append(sg.Registers, p.Name)
		}
		schemaGroups = append(schemaGroups, sg)
	}
	return SectionSchemaMessage{
		Type:        MsgTypeSectionSchema,
		Section:     "bms",
		Groups:      schemaGroups,
		PackContext: &PackSchemaContext{Input: input, Tower: tower, Pack: pack},
	}
}

// streamPackRead reads pack registers group-by-group, accumulating results per group
// and sending them as a batch when each group completes. This creates a per-group
// streaming effect where all values in a group appear together in the UI (D-05, D-06).
// Implements D-01 (streaming), D-04 (skip unsupported), D-05 (skip cleared in handleSelectPack).
func (h *Hub) streamPackRead(input, tower, pack int, client *Client, readCtx context.Context) {
	groups := register.PackProbeGroups()
	towersPerInput := TopoTowers
	queryWord := register.EncodePackQuery(input, tower, pack, towersPerInput)
	settleMs := h.packSettleMs
	// WR-03 fix: deep-copy the map so the goroutine writes to an independent copy,
	// avoiding a data race with handleSelectPack which replaces h.packSkipRegisters.
	skipRegs := make(map[uint16]bool, len(h.packSkipRegisters))
	for k, v := range h.packSkipRegisters {
		skipRegs[k] = v
	}

	// Capture BMS section on hub goroutine before launching reader goroutine
	// to avoid data race on h.sections map (CR-01).
	bmsSec := h.sections["bms"]
	if bmsSec == nil {
		return
	}

	go func() {
		defer bmsSec.reading.Store(false)

		// Step 1: Write 0x9020 to select pack
		err := h.broker.WriteRegister(readCtx, 0x9020, queryWord)
		if err != nil {
			h.logger.Warn("pack select write failed, retrying", "error", err)
			select {
			case <-time.After(time.Duration(settleMs*2) * time.Millisecond):
			case <-readCtx.Done():
				return
			}
			err = h.broker.WriteRegister(readCtx, 0x9020, queryWord)
			if err != nil {
				h.logger.Error("pack select write failed after retry", "error", err)
				h.sendPackError(client, input, tower, pack, "timeout writing pack selection after retry")
				return
			}
		}

		// Step 2: Wait for settle time (context-aware)
		select {
		case <-time.After(time.Duration(settleMs) * time.Millisecond):
		case <-readCtx.Done():
			return
		}

		// Step 3: Send pack schema
		schema := h.buildPackSchema(input, tower, pack, groups)
		h.results <- sectionResult{section: "bms", msg: schema}

		// Step 4: Read probes per group, accumulate results, send as batch (D-05)
		for _, g := range groups {
			var groupResults []sectionResult
			for _, p := range g.Probes {
				if readCtx.Err() != nil {
					return
				}
				if skipRegs[p.Addr] {
					continue
				}

				data, err := h.broker.ReadRegisters(readCtx, p.Addr, p.Count)
				if err != nil {
					// D-04: Track timeout/illegal address errors for skip
					errStr := err.Error()
					if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "0x02") || strings.Contains(errStr, "illegal") {
						skipRegs[p.Addr] = true
					}
					if readCtx.Err() != nil {
						return
					}
					groupResults = append(groupResults, sectionResult{
						section: "bms",
						msg:     NewRegisterValue("bms", g.Name, p.Name, "", errStr, p.Addr, ""),
					})
					continue
				}

				if readCtx.Err() != nil {
					return
				}
				groupResults = append(groupResults, sectionResult{
					section: "bms",
					msg:     NewRegisterValue("bms", g.Name, p.Name, FormatValue(p, data), "", p.Addr, FormatRawValue(p, data)),
				})
			}
			// D-05: Send all results for this group at once
			for _, r := range groupResults {
				h.results <- r
			}
		}

		if readCtx.Err() != nil {
			return
		}

		// Step 5: Send section_complete
		h.results <- sectionResult{
			section: "bms",
			msg:     NewSectionComplete("bms"),
		}
	}()
}

// streamBMSRead replaces triggerBMSRead with per-register streaming.
// Streams individual BMS info registers while collecting results for
// topology/bitmap/protection post-processing (Pitfall 6: "stream the reads, batch the computation").
func (h *Hub) streamBMSRead(sec *Section, readCtx context.Context) {
	groups := sec.Groups
	probes := sec.Probes

	// Build read list for protection registers (batch at end)
	protectionProbes := register.BMSProtectionProbes()

	go func() {
		defer sec.reading.Store(false)

		// Step 1: Stream each BMS info probe individually
		probeResults := make([]broker.Result, len(probes))
		probeIdx := 0

		for _, g := range groups {
			// Collect BMS clock registers for composition
			var clockHi, clockLo uint16
			var hasClockHi, hasClockLo bool
			var swChar string
			var swMajor, swNonStd, swMinor uint16
			var hasSWChar, hasSWMajor, hasSWNonStd, hasSWMinor bool

			for _, p := range g.Probes {
				if readCtx.Err() != nil {
					return
				}

				data, err := h.broker.ReadRegisters(readCtx, p.Addr, p.Count)
				probeResults[probeIdx] = broker.Result{Data: data, Err: err}
				probeIdx++

				if err != nil {
					// WR-02: skip send if context cancelled
					if readCtx.Err() != nil {
						return
					}
					h.results <- sectionResult{
						section: "bms",
						msg:     NewRegisterValue("bms", g.Name, p.Name, "", err.Error(), p.Addr, ""),
					}
					continue
				}

				// Collect composition registers while still streaming individual values
				switch p.Addr {
				case 0x9004:
					if len(data) >= 2 {
						clockHi = binary.BigEndian.Uint16(data[:2])
						hasClockHi = true
					}
				case 0x9005:
					if len(data) >= 2 {
						clockLo = binary.BigEndian.Uint16(data[:2])
						hasClockLo = true
					}
				case 0x9018:
					swChar = FormatValue(p, data)
					hasSWChar = true
				case 0x9019:
					if len(data) >= 2 {
						swMajor = binary.BigEndian.Uint16(data[:2])
						hasSWMajor = true
					}
				case 0x901A:
					if len(data) >= 2 {
						swNonStd = binary.BigEndian.Uint16(data[:2])
						hasSWNonStd = true
					}
				case 0x901B:
					if len(data) >= 2 {
						swMinor = binary.BigEndian.Uint16(data[:2])
						hasSWMinor = true
					}
				case 0x900D:
					if len(data) >= 2 {
						val := binary.BigEndian.Uint16(data[:2])
						pStr, pPack := register.DecodeTopology(val)
						value := fmt.Sprintf("%d strings x %d packs (0x%04X)", pStr, pPack, val)
						if readCtx.Err() != nil {
							return
						}
						h.results <- sectionResult{
							section: "bms",
							msg:     NewRegisterValue("bms", g.Name, p.Name, value, "", p.Addr, FormatRawValue(p, data)),
						}
						continue // topology already streamed with custom format
					}
				}

				// WR-02: skip send if context cancelled after ReadRegisters returned
				if readCtx.Err() != nil {
					return
				}
				// Stream register value (including composition source registers)
				h.results <- sectionResult{
					section: "bms",
					msg:     NewRegisterValue("bms", g.Name, p.Name, FormatValue(p, data), "", p.Addr, FormatRawValue(p, data)),
				}
			}

			// After group: send composed BMS clock
			if hasClockHi && hasClockLo && readCtx.Err() == nil {
				clockVal := uint32(clockHi)<<16 | uint32(clockLo)
				h.results <- sectionResult{
					section: "bms",
					msg:     NewRegisterValue("bms", g.Name, "System Clock", register.DecodeBMSClock(clockVal), "", 0, ""),
				}
			}

			// After group: send composed SW version
			if hasSWChar && hasSWMajor && hasSWNonStd && hasSWMinor && readCtx.Err() == nil {
				h.results <- sectionResult{
					section: "bms",
					msg:     NewRegisterValue("bms", g.Name, "SW Version", fmt.Sprintf("%s%d.%d.%d", swChar, swMajor, swNonStd, swMinor), "", 0, ""),
				}
			}
		}

		// WR-02: bail out if context cancelled before post-processing
		if readCtx.Err() != nil {
			return
		}

		// Step 2: Detect topology from 0x900D
		var detectedStr string
		var mismatch bool
		for i, p := range probes {
			if p.Addr == 0x900D && i < len(probeResults) && probeResults[i].Err == nil && len(probeResults[i].Data) >= 2 {
				val := binary.BigEndian.Uint16(probeResults[i].Data[:2])
				pStr, pPack := register.DecodeTopology(val)
				detectedStr = fmt.Sprintf("%d strings x %d packs", pStr, pPack)
				if pStr != TopoTowers || pPack != TopoPacksPerTower {
					mismatch = true
				}
				break
			}
		}

		// Step 3: Extract tower bitmap from 0x9022
		var towerBitmap uint16
		for i, p := range probes {
			if p.Addr == 0x9022 && i < len(probeResults) && probeResults[i].Err == nil && len(probeResults[i].Data) >= 2 {
				towerBitmap = binary.BigEndian.Uint16(probeResults[i].Data[:2])
				break
			}
		}

		onlineBitmaps := make([]uint16, TopoTowers)
		for t := 0; t < TopoTowers; t++ {
			if (towerBitmap>>uint(t))&1 == 1 {
				onlineBitmaps[t] = (1 << uint(TopoPacksPerTower)) - 1
			}
		}

		// Step 4: Send bitmap and protection as batched section_data (these are computed groups)
		bitmapGroup := GroupData{
			Name: "Battery Topology",
			Type: "bitmap",
			Bitmap: &BitmapData{
				Towers:           TopoTowers,
				PacksPerTower:    TopoPacksPerTower,
				Online:           onlineBitmaps,
				DetectedTopology: detectedStr,
				Mismatch:         mismatch,
			},
		}

		// Read protection registers in batch (small set, 6 registers)
		var protReads []broker.ReadRequest
		for _, pp := range protectionProbes {
			protReads = append(protReads, broker.ReadRequest{Addr: pp.Addr, Count: pp.Count})
		}
		protResults := h.broker.ReadBatch(readCtx, protReads)
		protGroup := h.buildProtectionGroup(protectionProbes, protResults)

		// Send computed groups as section_data for bitmap/protection widgets
		h.results <- sectionResult{
			section: "bms",
			msg:     NewGroupedSectionData("bms", []GroupData{bitmapGroup, protGroup}, nil),
		}

		// Signal completion
		h.results <- sectionResult{
			section: "bms",
			msg:     NewSectionComplete("bms"),
		}
	}()
}
