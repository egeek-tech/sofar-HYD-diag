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

// streamStandardRead replaces triggerStandardRead with per-register streaming.
// Instead of calling ReadBatch and waiting for all results, it reads each register
// individually and sends a register_value message immediately after each read (STREAM-01, D-01).
func (h *Hub) streamStandardRead(sectionName string, sec *Section, readCtx context.Context) {
	groups := sec.Groups
	isFault := sec.faultSection

	go func() {
		defer sec.reading.Store(false)

		// Stream each probe individually using ReadRegisters
		for _, g := range groups {
			for _, p := range g.Probes {
				if p.Count == 0 {
					continue // synthetic probe: schema placeholder only (D-07)
				}
				if readCtx.Err() != nil {
					return
				}

				data, err := h.broker.ReadRegisters(readCtx, p.Addr, p.Count)

				var errStr string
				var value string

				var rawVal string
				if err != nil {
					errStr = err.Error()
				} else {
					value = FormatValue(p, data)
					rawVal = FormatRawValue(p, data)
				}

				// WR-02: skip send if context cancelled after ReadRegisters returned
				if readCtx.Err() != nil {
					return
				}
				// Send register_value immediately
				h.results <- sectionResult{
					section: sectionName,
					msg:     NewRegisterValue(sectionName, g.Name, p.Name, value, errStr, p.Addr, rawVal),
				}
			}

			// D-04, D-05: Batch read time registers after Status group probes
			if g.Name == "Status" && readCtx.Err() == nil {
				data, err := h.broker.ReadRegisters(readCtx, 0x042C, 6)
				if err == nil && len(data) >= 12 {
					var vals [6]uint16
					for i := 0; i < 6; i++ {
						vals[i] = binary.BigEndian.Uint16(data[i*2 : i*2+2])
					}
					composed := register.ComposeSystemTime(vals[0], vals[1], vals[2], vals[3], vals[4], vals[5])
					rawStr := fmt.Sprintf("0x042C-0x0431 | %d, %d, %d, %d, %d, %d",
						vals[0], vals[1], vals[2], vals[3], vals[4], vals[5])
					// D-06: Only send on success; on failure skeleton em-dash persists
					if readCtx.Err() == nil {
						h.results <- sectionResult{
							section: sectionName,
							msg:     NewRegisterValue(sectionName, g.Name, "System time", composed, "", 0x042C, rawStr),
						}
					}
				}
				// D-06: On error, silently skip — skeleton dash persists until next successful cycle
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

		// Signal completion
		h.results <- sectionResult{
			section: sectionName,
			msg:     NewSectionComplete(sectionName),
		}
	}()
}

// streamBatteryRead replaces triggerBatteryRead with per-register streaming.
// Streams individual registers while collecting results for auto-detection logic.
func (h *Hub) streamBatteryRead(sec *Section, readCtx context.Context) {
	groups := sec.Groups
	probes := sec.Probes

	go func() {
		// retrigger tracks whether a section re-read was dispatched.
		// When true, the defer skips clearing sec.reading so it stays true
		// through the handoff to the event loop's triggerSectionRead (WR-05).
		retrigger := false
		defer func() {
			if !retrigger {
				sec.reading.Store(false)
			}
		}()

		// Read all probes individually, streaming each value
		allResults := make([]broker.Result, len(probes))
		probeIdx := 0

		for _, g := range groups {
			for _, p := range g.Probes {
				if readCtx.Err() != nil {
					return
				}

				data, err := h.broker.ReadRegisters(readCtx, p.Addr, p.Count)
				result := broker.Result{Data: data, Err: err}
				allResults[probeIdx] = result
				probeIdx++

				var errStr string
				var value string
				var rawVal string
				if err != nil {
					errStr = err.Error()
				} else {
					value = FormatValue(p, data)
					rawVal = FormatRawValue(p, data)
				}

				// WR-02: skip send if context cancelled after ReadRegisters returned
				if readCtx.Err() != nil {
					return
				}
				h.results <- sectionResult{
					section: "battery",
					msg:     NewRegisterValue("battery", g.Name, p.Name, value, errStr, p.Addr, rawVal),
				}
			}
		}

		// Auto-detect channel count from 0x066A (same logic as triggerBatteryRead)
		packCountIdx := -1
		for i, p := range probes {
			if p.Addr == 0x066A {
				packCountIdx = i
				break
			}
		}
		if packCountIdx >= 0 && allResults[packCountIdx].Err == nil && len(allResults[packCountIdx].Data) >= 2 {
			detected := int(binary.BigEndian.Uint16(allResults[packCountIdx].Data[:2]))
			if detected > 0 && detected <= 8 {
				currentChannels := len(groups) - 1
				if detected != currentChannels {
					newGroups := register.GenerateBatteryGroups(detected)
					// Keep reading=true through the handoff to prevent
					// duplicate reads from read_cycle during the window
					retrigger = true
					// WR-03: use select with context to avoid goroutine leak on hub shutdown
					select {
					case h.funcs <- func() {
						sec.Groups = newGroups
						sec.Probes = flattenProbeGroups(newGroups)
						h.logger.Info("battery section auto-detected channels", "channels", detected)
						h.triggerSectionRead("battery")
					}:
					case <-readCtx.Done():
						retrigger = false // context cancelled; let defer clear reading flag
					}
					return
				}
			}
		}

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

// streamPackRead reads pack registers one-by-one, streaming register_value messages
// for each probe, then sends section_complete. Follows the same pattern as streamBMSRead.
// Implements D-01 (streaming), D-04 (skip unsupported), D-05 (skip cleared in handleSelectPack).
func (h *Hub) streamPackRead(input, tower, pack int, client *Client, readCtx context.Context) {
	groups := register.PackProbeGroups()
	towersPerInput := TopoTowers
	queryWord := register.EncodePackQuery(input, tower, pack, towersPerInput)
	settleMs := h.packSettleMs
	skipRegs := h.packSkipRegisters // capture reference for goroutine

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

		// Step 4: Read each probe individually, streaming register_value messages
		for _, g := range groups {
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
					h.results <- sectionResult{
						section: "bms",
						msg:     NewRegisterValue("bms", g.Name, p.Name, "", errStr, p.Addr, ""),
					}
					continue
				}

				if readCtx.Err() != nil {
					return
				}
				h.results <- sectionResult{
					section: "bms",
					msg:     NewRegisterValue("bms", g.Name, p.Name, FormatValue(p, data), "", p.Addr, FormatRawValue(p, data)),
				}
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
