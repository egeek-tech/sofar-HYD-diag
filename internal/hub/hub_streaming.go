package hub

import (
	"encoding/binary"
	"fmt"
	"strings"

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
func (h *Hub) streamStandardRead(sectionName string, sec *Section) {
	groups := sec.Groups
	isFault := sec.faultSection

	go func() {
		defer sec.reading.Store(false)

		// Stream each probe individually using ReadRegisters
		for _, g := range groups {
			// Collect system time register values for composition (system section only)
			var timeVals [6]uint16
			timeCount := 0

			for _, p := range g.Probes {
				if h.ctx.Err() != nil {
					return
				}

				data, err := h.broker.ReadRegisters(h.ctx, p.Addr, p.Count)

				var errStr string
				var value string

				if err != nil {
					errStr = err.Error()
				} else {
					// Handle system time composition: collect but don't stream individual time registers
					if strings.HasPrefix(p.Name, "System time (") && len(data) >= 2 {
						val := binary.BigEndian.Uint16(data[:2])
						switch p.Name {
						case "System time (Year)":
							timeVals[0] = val
							timeCount++
						case "System time (Month)":
							timeVals[1] = val
							timeCount++
						case "System time (Day)":
							timeVals[2] = val
							timeCount++
						case "System time (Hour)":
							timeVals[3] = val
							timeCount++
						case "System time (Min)":
							timeVals[4] = val
							timeCount++
						case "System time (Sec)":
							timeVals[5] = val
							timeCount++
						}
						continue // don't stream individual time registers
					}
					value = FormatValue(p, data)
				}

				// Send register_value immediately
				h.results <- sectionResult{
					section: sectionName,
					msg:     NewRegisterValue(sectionName, g.Name, p.Name, value, errStr),
				}
			}

			// After all probes in this group: send composed system time if collected
			if timeCount == 6 {
				composed := register.ComposeSystemTime(
					timeVals[0], timeVals[1], timeVals[2],
					timeVals[3], timeVals[4], timeVals[5],
				)
				h.results <- sectionResult{
					section: sectionName,
					msg:     NewRegisterValue(sectionName, g.Name, "System time", composed, ""),
				}
			}
		}

		// Read and decode faults for system section (streamed as a batch at the end)
		if isFault {
			var faultReads []broker.ReadRequest
			for _, fp := range register.FaultRegisters {
				faultReads = append(faultReads, broker.ReadRequest{Addr: fp.Addr, Count: fp.Count})
			}
			faultResults := h.broker.ReadBatch(h.ctx, faultReads)
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
func (h *Hub) streamBatteryRead(sec *Section) {
	groups := sec.Groups
	probes := sec.Probes

	go func() {
		defer sec.reading.Store(false)

		// Read all probes individually, streaming each value
		allResults := make([]broker.Result, len(probes))
		probeIdx := 0

		for _, g := range groups {
			for _, p := range g.Probes {
				if h.ctx.Err() != nil {
					return
				}

				data, err := h.broker.ReadRegisters(h.ctx, p.Addr, p.Count)
				result := broker.Result{Data: data, Err: err}
				allResults[probeIdx] = result
				probeIdx++

				var errStr string
				var value string
				if err != nil {
					errStr = err.Error()
				} else {
					value = FormatValue(p, data)
				}

				h.results <- sectionResult{
					section: "battery",
					msg:     NewRegisterValue("battery", g.Name, p.Name, value, errStr),
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
					h.funcs <- func() {
						sec.Groups = newGroups
						sec.Probes = flattenProbeGroups(newGroups)
						h.logger.Info("battery section auto-detected channels", "channels", detected)
						h.triggerSectionRead("battery")
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

// streamBMSRead replaces triggerBMSRead with per-register streaming.
// Streams individual BMS info registers while collecting results for
// topology/bitmap/protection post-processing (Pitfall 6: "stream the reads, batch the computation").
func (h *Hub) streamBMSRead(sec *Section) {
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
				if h.ctx.Err() != nil {
					return
				}

				data, err := h.broker.ReadRegisters(h.ctx, p.Addr, p.Count)
				probeResults[probeIdx] = broker.Result{Data: data, Err: err}
				probeIdx++

				if err != nil {
					h.results <- sectionResult{
						section: "bms",
						msg:     NewRegisterValue("bms", g.Name, p.Name, "", err.Error()),
					}
					continue
				}

				// Handle special composition registers: collect but stream composed value later
				switch p.Addr {
				case 0x9004:
					if len(data) >= 2 {
						clockHi = binary.BigEndian.Uint16(data[:2])
						hasClockHi = true
					}
					continue
				case 0x9005:
					if len(data) >= 2 {
						clockLo = binary.BigEndian.Uint16(data[:2])
						hasClockLo = true
					}
					continue
				case 0x9018:
					swChar = FormatValue(p, data)
					hasSWChar = true
					continue
				case 0x9019:
					if len(data) >= 2 {
						swMajor = binary.BigEndian.Uint16(data[:2])
						hasSWMajor = true
					}
					continue
				case 0x901A:
					if len(data) >= 2 {
						swNonStd = binary.BigEndian.Uint16(data[:2])
						hasSWNonStd = true
					}
					continue
				case 0x901B:
					if len(data) >= 2 {
						swMinor = binary.BigEndian.Uint16(data[:2])
						hasSWMinor = true
					}
					continue
				case 0x900D:
					if len(data) >= 2 {
						val := binary.BigEndian.Uint16(data[:2])
						pStr, pPack := register.DecodeTopology(val)
						value := fmt.Sprintf("%d strings x %d packs (0x%04X)", pStr, pPack, val)
						h.results <- sectionResult{
							section: "bms",
							msg:     NewRegisterValue("bms", g.Name, p.Name, value, ""),
						}
					}
					continue
				}

				// Stream standard register value
				h.results <- sectionResult{
					section: "bms",
					msg:     NewRegisterValue("bms", g.Name, p.Name, FormatValue(p, data), ""),
				}
			}

			// After group: send composed BMS clock
			if hasClockHi && hasClockLo {
				clockVal := uint32(clockHi)<<16 | uint32(clockLo)
				h.results <- sectionResult{
					section: "bms",
					msg:     NewRegisterValue("bms", g.Name, "System Clock", register.DecodeBMSClock(clockVal), ""),
				}
			}

			// After group: send composed SW version
			if hasSWChar && hasSWMajor && hasSWNonStd && hasSWMinor {
				h.results <- sectionResult{
					section: "bms",
					msg:     NewRegisterValue("bms", g.Name, "SW Version", fmt.Sprintf("%s%d.%d.%d", swChar, swMajor, swNonStd, swMinor), ""),
				}
			}
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
		protResults := h.broker.ReadBatch(h.ctx, protReads)
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
