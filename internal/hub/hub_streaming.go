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

// readSpanIndividualFallbackAccum reads each probe in a span individually,
// emitting sectionResult per probe AND accumulating results into probeResults.
// Returns true if ALL individual reads failed. Returns early on context cancel.
func (h *Hub) readSpanIndividualFallbackAccum(readCtx context.Context, sectionName string, span register.BatchSpan, probeResults []broker.Result, addrToIdx map[uint16]int) bool {
	allFailed := true
	for _, pm := range span.Probes {
		if readCtx.Err() != nil {
			return true
		}
		indData, indErr := h.broker.ReadRegisters(readCtx, pm.Probe.Addr, pm.Probe.Count)
		var errStr, value, rawVal string
		if indErr != nil {
			errStr = indErr.Error()
			// Record error in probeResults so post-processing (e.g. topology
			// detection, tower bitmap) can distinguish a failed read from an
			// untouched zero-value Result.
			if idx, ok := addrToIdx[pm.Probe.Addr]; ok {
				probeResults[idx] = broker.Result{Err: indErr}
			}
		} else {
			allFailed = false
			value = FormatValue(pm.Probe, indData)
			rawVal = FormatRawValue(pm.Probe, indData)
			// Accumulate successful individual read result.
			if idx, ok := addrToIdx[pm.Probe.Addr]; ok {
				probeResults[idx] = broker.Result{Data: make([]byte, len(indData))}
				copy(probeResults[idx].Data, indData)
			}
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

// readBatchSpans iterates a section's BatchPlan spans using SpanTracker
// three-state degradation. For each probe, it emits a sectionResult to the UI
// (streaming) and accumulates the broker.Result in probeResults[probeIndex].
// Returns probeResults aligned with sec.Probes indexing (nil on context cancel).
func (h *Hub) readBatchSpans(readCtx context.Context, sectionName string, sec *Section) []broker.Result {
	start := time.Now()
	spanCount := len(sec.BatchPlan.Spans)
	var totalRegisters uint16

	probeResults := make([]broker.Result, len(sec.Probes))

	// Build address-to-probe-index map for result accumulation.
	addrToIdx := make(map[uint16]int, len(sec.Probes))
	for i, p := range sec.Probes {
		addrToIdx[p.Addr] = i
	}

	if sec.SpanTracker != nil {
		sec.SpanTracker.Tick()
	}

	for _, span := range sec.BatchPlan.Spans {
		if readCtx.Err() != nil {
			return nil
		}

		totalRegisters += span.TotalCount

		// Three-state degradation check (Phase 22).
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
			allIndivFailed := h.readSpanIndividualFallbackAccum(readCtx, sectionName, span, probeResults, addrToIdx)
			if readCtx.Err() != nil {
				return nil
			}
			if allIndivFailed && sec.SpanTracker != nil {
				sec.SpanTracker.RecordIndividualFailure(span.StartAddr)
			} else if !allIndivFailed && sec.SpanTracker != nil {
				sec.SpanTracker.RecordSuccess(span.StartAddr)
			}
			continue
		}

		// Normal or probing: attempt batch read.
		data, err := h.broker.ReadRegisters(readCtx, span.StartAddr, span.TotalCount)
		if err != nil {
			// Record failure for all states (normal, degraded probe, skipped probe).
			if sec.SpanTracker != nil {
				sec.SpanTracker.RecordFailure(span.StartAddr)
			}
			h.logger.Warn("batch span failed, falling back to individual reads",
				"section", sectionName,
				"startAddr", fmt.Sprintf("0x%04X", span.StartAddr),
				"span_count", span.TotalCount,
				"error", err,
			)
			h.readSpanIndividualFallbackAccum(readCtx, sectionName, span, probeResults, addrToIdx)
			if readCtx.Err() != nil {
				return nil
			}
			continue
		}

		// Batch read succeeded.
		if sec.SpanTracker != nil {
			sec.SpanTracker.RecordSuccess(span.StartAddr)
		}

		if readCtx.Err() != nil {
			return nil
		}

		// Extract each probe's data from the batch response.
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
			// Accumulate result for post-processing.
			if idx, ok := addrToIdx[pm.Probe.Addr]; ok {
				probeResults[idx] = broker.Result{Data: make([]byte, len(probeData))}
				copy(probeResults[idx].Data, probeData)
			}
		}
	}

	h.logger.Info("section read complete",
		"section", sectionName,
		"duration_ms", time.Since(start).Milliseconds(),
		"spans", spanCount,
		"registers", totalRegisters,
	)

	return probeResults
}

// streamStandardRead reads a standard section using batch spans from the pre-computed
// BatchPlan. Uses the shared readBatchSpans helper for span iteration (D-07).
// Section timing is logged by readBatchSpans. Fault reading and section_complete
// are handled after spans complete.
func (h *Hub) streamStandardRead(readCtx context.Context, sectionName string, sec *Section) {
	isFault := sec.faultSection

	go func() {
		defer sec.reading.Store(false)

		// Use shared helper for batch span reads (D-07).
		_ = h.readBatchSpans(readCtx, sectionName, sec)
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
				for reg := range fp.Count {
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
func (h *Hub) streamBatteryBatchRead(readCtx context.Context, sec *Section) {
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
					// Allocate a fresh slice to avoid mutating backing arrays of
					// GenerateBatteryGroups or InternalInfoGroups (latent shared-array hazard).
					battGroups := register.GenerateBatteryGroups(detected)
					infoGroups := register.InternalInfoGroups()
					newGroups := make([]register.ProbeGroup, 0, len(battGroups)+len(infoGroups))
					newGroups = append(newGroups, battGroups...)
					newGroups = append(newGroups, infoGroups...)
					retrigger = true
					select {
					case h.funcs <- func() {
						sec.Groups = newGroups
						sec.Probes = flattenProbeGroups(newGroups)
						sec.BatchPlan = register.AnalyzeBatchPlan(newGroups)
						sec.SpanTracker.Reset() // D-05: reset since span addresses change
						// Broadcast updated schema to all battery subscribers (bug fix:
						// frontend was keeping stale N-channel skeleton after auto-detection).
						schema := h.buildSectionSchema("battery", sec)
						h.broadcastResultToSection("battery", schema)
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

		// Step 2: Read all spans via shared helper (D-07).
		_ = h.readBatchSpans(readCtx, "battery", sec)
		if readCtx.Err() != nil {
			return
		}

		h.results <- sectionResult{
			section: "battery",
			msg:     NewSectionComplete("battery"),
		}
	}()
}

// streamBMSBatchRead reads the BMS section using the shared readBatchSpans helper,
// then performs BMS-specific post-processing: topology detection, tower bitmap widget,
// and protection decoding. Per D-06, keeps BMS post-processing isolated from standard path.
func (h *Hub) streamBMSBatchRead(readCtx context.Context, sec *Section) {
	go func() {
		defer sec.reading.Store(false)

		// Step 1: Read all spans via shared helper (D-07).
		probeResults := h.readBatchSpans(readCtx, "bms", sec)
		if probeResults == nil || readCtx.Err() != nil {
			return
		}

		probes := sec.Probes

		// Step 2: Detect topology from 0x900D (same logic as old streamBMSRead Step 2).
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

		// Step 3: Extract tower bitmap from 0x9022 (D-03, same logic as old streamBMSRead Step 3).
		var towerBitmap uint16
		for i, p := range probes {
			if p.Addr == 0x9022 && i < len(probeResults) && probeResults[i].Err == nil && len(probeResults[i].Data) >= 2 {
				towerBitmap = binary.BigEndian.Uint16(probeResults[i].Data[:2])
				break
			}
		}

		onlineBitmaps := make([]uint16, TopoTowers)
		for t := range TopoTowers {
			if (towerBitmap>>uint(t))&1 == 1 {
				onlineBitmaps[t] = (1 << uint(TopoPacksPerTower)) - 1
			}
		}

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

		// Step 4: Collect protection probe results for buildProtectionGroup (D-05).
		// Protection probes are identified by address match against the known set.
		isProtectionAddr := func(addr uint16) bool {
			switch addr {
			case 0x9014, 0x9015, 0x9016, 0x9017, 0x901C, 0x901D:
				return true
			}
			return false
		}
		var protProbes []register.Probe
		var protResults []broker.Result
		for i, p := range probes {
			if isProtectionAddr(p.Addr) {
				protProbes = append(protProbes, p)
				protResults = append(protResults, probeResults[i])
			}
		}
		protGroup := h.buildProtectionGroup(protProbes, protResults)

		// Step 5: Send bitmap and protection as batched section_data.
		h.results <- sectionResult{
			section: "bms",
			msg:     NewGroupedSectionData("bms", []GroupData{bitmapGroup, protGroup}, nil),
		}

		// Step 6: section_complete.
		h.results <- sectionResult{
			section: "bms",
			msg:     NewSectionComplete("bms"),
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

// streamPackBatchRead reads pack registers using batch spans via the shared
// readBatchSpans helper. Preserves the 0x9020 write + settle delay from the
// legacy streamPackRead. Per D-01, pack-specific logic stays isolated.
func (h *Hub) streamPackBatchRead(readCtx context.Context, input, tower, pack int, client *Client) {
	groups := register.PackProbeGroups()
	towersPerInput := TopoTowers
	queryWord := register.EncodePackQuery(input, tower, pack, towersPerInput)
	settleMs := h.packSettleMs

	// Capture SpanTracker reference on hub goroutine before launching reader
	// goroutine to prevent data race with handleSelectPack (Pitfall 2).
	packTracker := h.packSpanTracker

	bmsSec := h.sections["bms"]
	if bmsSec == nil {
		return
	}

	go func() {
		defer bmsSec.reading.Store(false)

		// Step 1: Write 0x9020 to select pack (same as legacy streamPackRead).
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

		// Step 2: Wait for settle time (context-aware).
		select {
		case <-time.After(time.Duration(settleMs) * time.Millisecond):
		case <-readCtx.Done():
			return
		}

		// Step 3: Send pack schema.
		schema := h.buildPackSchema(input, tower, pack, groups)
		h.results <- sectionResult{section: "bms", msg: schema}

		// Step 4: Build temporary Section and call readBatchSpans (D-02, D-03).
		plan := register.AnalyzeBatchPlan(groups)
		probes := flattenProbeGroups(groups)
		tempSec := &Section{
			BatchPlan:   plan,
			SpanTracker: packTracker,
			Probes:      probes,
		}
		_ = h.readBatchSpans(readCtx, "bms", tempSec)
		if readCtx.Err() != nil {
			return
		}

		// Step 5: section_complete.
		h.results <- sectionResult{
			section: "bms",
			msg:     NewSectionComplete("bms"),
		}
	}()
}
