package hub

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"sofar-hyd-diag/internal/broker"
	"sofar-hyd-diag/internal/register"
)

// ClientCommand wraps an inbound message with the originating client.
type ClientCommand struct {
	Client  *Client
	Message InboundMessage
}

// sectionResult carries ReadBatch results back to the hub event loop for broadcasting.
type sectionResult struct {
	section string
	msg     OutboundMessage
}

// Hub manages WebSocket clients, section subscriptions, and broker integration.
// All state mutations happen in the Run() goroutine (D-03 hub + per-client goroutine pattern).
type Hub struct {
	broker     BrokerInterface
	logger     *slog.Logger
	clients    map[*Client]bool
	sections   map[string]*Section
	register   chan *Client
	unregister chan *Client
	commands   chan ClientCommand
	funcs      chan func()         // thread-safe arbitrary function execution on hub goroutine
	results    chan sectionResult  // read results routed back to event loop
	timerCh    chan string
	ctx        context.Context
	cancel     context.CancelFunc
	connected          bool          // tracks whether broker is connected for timer pause/resume (D-28)
	refreshOverride    time.Duration // if non-zero, overrides defaultRefreshInterval for all sections (testing)
	defaultPVChannels  int           // default PV channel count for PV section (D-16)
	defaultBatInputs   int           // default battery inputs (1-2)
	defaultBatTowers   int           // default towers per input (1-4)
	defaultBatPacks    int           // default packs per tower (4-10)
	selectedPack       *packSelection // currently selected pack for auto-refresh (nil = no pack selected)
}

// packSelection tracks the currently selected pack for auto-refresh re-triggering.
type packSelection struct {
	input, tower, pack int
	client             *Client
}

// NewHub creates a new Hub. Call Run() in a goroutine to start processing.
// pvChannels sets the default number of PV channels (clamped to 2-16).
// batInputs/batTowers/batPacks set the default battery topology.
func NewHub(b BrokerInterface, logger *slog.Logger, pvChannels int, batInputs, batTowers, batPacks int) *Hub {
	if pvChannels < 2 {
		pvChannels = 2
	}
	if pvChannels > 16 {
		pvChannels = 16
	}
	if batInputs < 1 {
		batInputs = 1
	}
	if batInputs > 2 {
		batInputs = 2
	}
	if batTowers < 1 {
		batTowers = 1
	}
	if batTowers > 4 {
		batTowers = 4
	}
	if batPacks < 4 {
		batPacks = 4
	}
	if batPacks > 10 {
		batPacks = 10
	}
	return &Hub{
		broker:            b,
		logger:            logger.With("component", "hub"),
		clients:           make(map[*Client]bool),
		sections:          make(map[string]*Section),
		register:          make(chan *Client, 16),
		unregister:        make(chan *Client, 16),
		commands:          make(chan ClientCommand, 64),
		funcs:             make(chan func(), 8),
		results:           make(chan sectionResult, 32),
		timerCh:           make(chan string, 16),
		defaultPVChannels: pvChannels,
		defaultBatInputs:  batInputs,
		defaultBatTowers:  batTowers,
		defaultBatPacks:   batPacks,
	}
}

// Register queues a client for registration with the hub.
func (h *Hub) Register(c *Client) {
	h.register <- c
}

// Unregister queues a client for removal from the hub.
func (h *Hub) Unregister(c *Client) {
	h.unregister <- c
}

// Command queues a client command for processing by the hub.
func (h *Hub) Command(c *Client, msg InboundMessage) {
	h.commands <- ClientCommand{Client: c, Message: msg}
}

// RegisterSection creates a Section and adds it to the hub's section map.
// Must be called before Run() or from within the Run goroutine.
func (h *Hub) RegisterSection(name string, probes []Probe) {
	sec := newSection(name, probes, h.timerCh, h.logger)
	if h.refreshOverride > 0 {
		sec.SetInterval(h.refreshOverride)
	}
	h.sections[name] = sec
}

// RegisterGroupedSection creates a ProbeGroup-based Section and adds it to the hub.
// Must be called before Run() or from within the Run goroutine.
func (h *Hub) RegisterGroupedSection(name string, groups []register.ProbeGroup) {
	sec := newGroupedSection(name, groups, h.timerCh, h.logger)
	if h.refreshOverride > 0 {
		sec.SetInterval(h.refreshOverride)
	}
	h.sections[name] = sec
}

// SetRefreshOverride sets a custom refresh interval applied to all sections.
// Must be called before Run(). Used for testing with shorter intervals.
func (h *Hub) SetRefreshOverride(d time.Duration) {
	h.refreshOverride = d
}

// ClientCount returns the number of registered clients.
// Thread-safe: routes the query through the hub event loop.
// Returns 0 if the hub has shut down.
func (h *Hub) ClientCount() int {
	reply := make(chan int, 1)
	select {
	case h.funcs <- func() { reply <- len(h.clients) }:
	case <-h.ctx.Done():
		return 0
	}
	select {
	case n := <-reply:
		return n
	case <-h.ctx.Done():
		return 0
	}
}

// RunFunc executes a function on the hub's event loop goroutine.
// Blocks until the function completes. Thread-safe.
// Returns without executing fn if the hub has shut down.
func (h *Hub) RunFunc(fn func()) {
	done := make(chan struct{})
	wrapper := func() {
		fn()
		close(done)
	}
	select {
	case h.funcs <- wrapper:
	case <-h.ctx.Done():
		return // hub shut down; fn will not execute
	}
	select {
	case <-done:
	case <-h.ctx.Done():
	}
}

// Run starts the hub event loop. Blocks until ctx is cancelled.
func (h *Hub) Run(ctx context.Context) {
	h.ctx, h.cancel = context.WithCancel(ctx)
	defer h.cancel()

	stateEvents := h.broker.StateEvents()

	// Register built-in sections
	h.registerBuiltinSections()

	for {
		select {
		case <-h.ctx.Done():
			h.shutdown()
			return
		case client := <-h.register:
			h.clients[client] = true
			// Send current connection state to new client (D-04)
			state := h.broker.CurrentState()
			h.sendToClient(client, NewStateMessage(state.String(), ""))
			h.logger.Debug("client registered", "clients", len(h.clients))
		case client := <-h.unregister:
			h.removeClient(client)
		case cmd := <-h.commands:
			h.handleCommand(cmd)
		case evt, ok := <-stateEvents:
			if !ok {
				h.logger.Info("broker state channel closed, shutting down hub")
				h.shutdown()
				return
			}
			h.handleStateEvent(evt)
		case sectionName := <-h.timerCh:
			h.handleTimerTick(sectionName)
		case res := <-h.results:
			h.broadcastToSection(res.section, res.msg)
		case fn := <-h.funcs:
			fn()
		}
	}
}

// handleCommand dispatches based on message type.
func (h *Hub) handleCommand(cmd ClientCommand) {
	msg := cmd.Message
	switch msg.Type {
	case MsgTypeConnect:
		addr := fmt.Sprintf("%s:%d", msg.Host, msg.Port)
		// Run async so the hub event loop can still process state events and disconnect commands
		// while the broker's single dial attempt (5s timeout) is in progress.
		go func() {
			if err := h.broker.Reconfigure(h.ctx, addr, byte(msg.SlaveID)); err != nil {
				h.logger.Error("connect failed", "addr", addr, "error", err)
			}
		}()
	case MsgTypeDisconnect:
		go func() {
			if err := h.broker.Disconnect(h.ctx); err != nil {
				h.logger.Error("disconnect failed", "error", err)
			}
		}()
	case MsgTypeSubscribe:
		h.subscribeClient(cmd.Client, msg.Section)
	case MsgTypeUnsubscribe:
		h.unsubscribeClient(cmd.Client, msg.Section)
	case MsgTypeRefresh:
		h.triggerSectionRead(msg.Section)
	case MsgTypeAutoRefresh:
		h.handleAutoRefreshToggle(cmd.Client, msg)
	case MsgTypeConfigure:
		h.handleConfigure(cmd)
	case MsgTypeSelectPack:
		h.handleSelectPack(cmd)
	default:
		h.logger.Debug("unknown message type", "type", msg.Type)
	}
}

// handleStateEvent broadcasts state changes to all clients and manages timer pause/resume.
func (h *Hub) handleStateEvent(evt broker.StateEvent) {
	var errMsg string
	if evt.Err != nil {
		errMsg = evt.Err.Error()
	}
	h.broadcastAll(NewStateMessage(evt.State.String(), errMsg))

	switch evt.State {
	case broker.StateConnected:
		h.connected = true
		// Resume all section timers that have subscribers (D-28)
		for _, sec := range h.sections {
			sec.resumeTimer()
		}
	case broker.StateDisconnected, broker.StateReconnecting:
		h.connected = false
		// Pause all section timers (D-28)
		for _, sec := range h.sections {
			sec.pauseTimer()
		}
	}
}

// handleAutoRefreshToggle toggles auto-refresh for a section.
func (h *Hub) handleAutoRefreshToggle(c *Client, msg InboundMessage) {
	sec, ok := h.sections[msg.Section]
	if !ok {
		h.sendToClient(c, NewSectionError(msg.Section, "unknown section"))
		return
	}
	if msg.Enabled != nil {
		sec.autoRefresh = *msg.Enabled
		if *msg.Enabled && sec.SubscriberCount() > 0 {
			sec.startTimer()
		} else if !*msg.Enabled {
			sec.stopTimer()
		}
	}
}

// subscribeClient subscribes a client to a section.
// Unsubscribes from any previous section first (D-18: one section at a time).
// Triggers an immediate read (D-20).
func (h *Hub) subscribeClient(c *Client, sectionName string) {
	// Unsubscribe from previous section if any (D-18)
	if c.section != "" && c.section != sectionName {
		h.unsubscribeClient(c, c.section)
	}

	sec, ok := h.sections[sectionName]
	if !ok {
		h.sendToClient(c, NewSectionError(sectionName, "unknown section"))
		return
	}

	sec.addSubscriber(c)
	c.section = sectionName
	// Clear pack selection when (re-)subscribing to BMS overview
	if sectionName == "bms" {
		h.selectedPack = nil
	}
	h.logger.Debug("client subscribed", "section", sectionName, "subscribers", sec.SubscriberCount())

	// Trigger immediate read (D-20) or send error if disconnected
	if h.connected {
		h.triggerSectionRead(sectionName)
	} else {
		h.sendToClient(c, NewSectionError(sectionName, "not connected to inverter"))
	}
}

// unsubscribeClient removes a client from a section.
func (h *Hub) unsubscribeClient(c *Client, sectionName string) {
	sec, ok := h.sections[sectionName]
	if !ok {
		return
	}
	sec.removeSubscriber(c)
	if c.section == sectionName {
		c.section = ""
	}
	// Clear pack selection when leaving BMS
	if sectionName == "bms" {
		h.selectedPack = nil
	}
	h.logger.Debug("client unsubscribed", "section", sectionName, "subscribers", sec.SubscriberCount())
}

// handleTimerTick processes a timer tick for a section.
func (h *Hub) handleTimerTick(sectionName string) {
	sec, ok := h.sections[sectionName]
	if !ok {
		return
	}
	// Skip if no subscribers
	if sec.SubscriberCount() == 0 {
		return
	}
	// Skip overlapping reads (D-24)
	if sec.reading.Load() {
		h.logger.Debug("skipping overlapping read", "section", sectionName)
		return
	}
	// Skip if not connected (D-28)
	if !h.connected {
		return
	}
	// If BMS section has a selected pack, re-trigger pack read instead of BMS overview
	if sectionName == "bms" && h.selectedPack != nil {
		h.triggerPackRead(h.selectedPack.input, h.selectedPack.tower, h.selectedPack.pack, h.selectedPack.client)
		return
	}
	h.triggerSectionRead(sectionName)
}

// triggerSectionRead initiates a Modbus read for all probes in a section.
// The read runs in a goroutine; results are broadcast to section subscribers.
// BMS and battery sections have custom read cycles; all others use the standard path.
func (h *Hub) triggerSectionRead(sectionName string) {
	sec, ok := h.sections[sectionName]
	if !ok {
		return
	}

	sec.reading.Store(true)

	// Dispatch to custom read handlers for special sections
	switch sectionName {
	case "bms":
		h.triggerBMSRead(sec)
		return
	case "battery":
		h.triggerBatteryRead(sec)
		return
	}

	// Standard read path for system, grid, eps, pv, stats
	h.triggerStandardRead(sectionName, sec)
}

// triggerStandardRead performs a standard grouped read for a section.
// Handles system time composition and fault decoding for the system section.
func (h *Hub) triggerStandardRead(sectionName string, sec *Section) {
	// Build read requests from probes
	reads := make([]broker.ReadRequest, len(sec.Probes))
	for i, p := range sec.Probes {
		reads[i] = broker.ReadRequest{Addr: p.Addr, Count: p.Count}
	}

	// For system section, append fault register read requests
	var faultReads []broker.ReadRequest
	if sec.faultSection {
		for _, fp := range register.FaultRegisters {
			faultReads = append(faultReads, broker.ReadRequest{Addr: fp.Addr, Count: fp.Count})
		}
		reads = append(reads, faultReads...)
	}

	// Copy groups and probes for safe goroutine access
	groups := sec.Groups
	probes := sec.Probes
	isFault := sec.faultSection

	go func() {
		defer sec.reading.Store(false)

		results := h.broker.ReadBatch(h.ctx, reads)

		// Grouped section path
		if groups != nil {
			var hasError bool
			var errMsg string

			// Check for errors in probe results
			probeResults := results[:len(probes)]
			for _, r := range probeResults {
				if r.Err != nil {
					hasError = true
					errMsg = r.Err.Error()
				}
			}

			// Build GroupData by iterating groups and matching results by probe index
			groupDataSlice := make([]GroupData, 0, len(groups))
			probeIdx := 0
			for _, g := range groups {
				gd := GroupData{
					Name:   g.Name,
					Layout: g.Layout,
					Items:  make(map[string]string),
				}

				// Collect system time register values for composition
				var timeVals [6]uint16
				timeCount := 0

				for _, p := range g.Probes {
					r := probeResults[probeIdx]
					probeIdx++

					if r.Err != nil {
						continue
					}

					// Detect system time registers for composition
					if strings.HasPrefix(p.Name, "System time (") && len(r.Data) >= 2 {
						val := binary.BigEndian.Uint16(r.Data[:2])
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
						continue // don't add individual time registers
					}

					gd.Items[p.Name] = FormatValue(p, r.Data)
				}

				// Compose system time if all 6 components found
				if timeCount == 6 {
					gd.Items["System time"] = register.ComposeSystemTime(
						timeVals[0], timeVals[1], timeVals[2],
						timeVals[3], timeVals[4], timeVals[5],
					)
				}

				groupDataSlice = append(groupDataSlice, gd)
			}

			// Decode faults for system section
			var faultEntries []FaultEntry
			if isFault {
				faultResults := results[len(probes):]
				faultData := make(map[uint16]uint16)
				faultIdx := 0
				for _, fp := range register.FaultRegisters {
					if faultIdx >= len(faultResults) {
						break
					}
					r := faultResults[faultIdx]
					faultIdx++
					if r.Err != nil {
						continue
					}
					// Each fault probe reads multiple contiguous registers
					// Result data is count*2 bytes, parse each register
					for reg := uint16(0); reg < fp.Count; reg++ {
						offset := int(reg) * 2
						if offset+2 <= len(r.Data) {
							addr := fp.Addr + reg
							faultData[addr] = binary.BigEndian.Uint16(r.Data[offset : offset+2])
						}
					}
				}
				activeFaults := register.DecodeFaults(faultData)
				faultEntries = make([]FaultEntry, len(activeFaults))
				for i, desc := range activeFaults {
					faultEntries[i] = FaultEntry{Name: desc}
				}
			}

			if hasError {
				h.results <- sectionResult{section: sectionName, msg: NewSectionError(sectionName, errMsg)}
				return
			}
			h.results <- sectionResult{section: sectionName, msg: NewGroupedSectionData(sectionName, groupDataSlice, faultEntries)}
			return
		}

		// Legacy flat section path (fallback)
		data := make(map[string]string)
		var hasError bool
		var errMsg string

		for i, r := range results {
			if r.Err != nil {
				hasError = true
				errMsg = r.Err.Error()
				continue
			}
			key := toSnakeCase(probes[i].Name)
			data[key] = FormatValue(probes[i], r.Data)
		}

		if hasError {
			h.results <- sectionResult{section: sectionName, msg: NewSectionError(sectionName, errMsg)}
		}
		if len(data) > 0 {
			h.results <- sectionResult{section: sectionName, msg: NewSectionData(sectionName, data)}
		}
	}()
}

// triggerBatteryRead performs a battery section read with auto-detection of channel count.
// If the pack count register (0x066A) indicates a different channel count than currently
// configured, the section is rebuilt and re-read with the correct number of channels.
func (h *Hub) triggerBatteryRead(sec *Section) {
	groups := sec.Groups
	probes := sec.Probes

	reads := make([]broker.ReadRequest, len(probes))
	for i, p := range probes {
		reads[i] = broker.ReadRequest{Addr: p.Addr, Count: p.Count}
	}

	go func() {
		defer sec.reading.Store(false)
		results := h.broker.ReadBatch(h.ctx, reads)

		// Find pack count value from 0x066A result to auto-detect channel count
		packCountIdx := -1
		for i, p := range probes {
			if p.Addr == 0x066A {
				packCountIdx = i
				break
			}
		}
		if packCountIdx >= 0 && results[packCountIdx].Err == nil && len(results[packCountIdx].Data) >= 2 {
			detected := int(binary.BigEndian.Uint16(results[packCountIdx].Data[:2]))
			if detected > 0 && detected <= 8 {
				// Subtract Global Stats group to get current channel count
				currentChannels := len(groups) - 1
				if detected != currentChannels {
					// Rebuild battery section with detected channel count
					newGroups := register.GenerateBatteryGroups(detected)
					h.funcs <- func() {
						sec.Groups = newGroups
						sec.Probes = flattenProbeGroups(newGroups)
						h.logger.Info("battery section auto-detected channels", "channels", detected)
						// Re-trigger read with new probes
						h.triggerSectionRead("battery")
					}
					return
				}
			}
		}

		// Build grouped data using standard group building logic
		msg := h.buildGroupedResult("battery", groups, probes, results)
		h.results <- sectionResult{section: "battery", msg: msg}
	}()
}

// triggerBMSRead performs the BMS section's custom read cycle (D-09, D-14).
// It reads BMS info registers (including 0x9022 bitmap), detects topology from 0x900D,
// extracts bitmap from standard read results, and composes the BMS clock from 0x9004+0x9005.
func (h *Hub) triggerBMSRead(sec *Section) {
	groups := sec.Groups
	probes := sec.Probes
	inputs := h.defaultBatInputs
	towers := h.defaultBatTowers
	packs := h.defaultBatPacks

	// Build standard read requests for BMS info probes
	reads := make([]broker.ReadRequest, len(probes))
	for i, p := range probes {
		reads[i] = broker.ReadRequest{Addr: p.Addr, Count: p.Count}
	}

	// Also read protection registers
	protectionProbes := register.BMSProtectionProbes()
	for _, p := range protectionProbes {
		reads = append(reads, broker.ReadRequest{Addr: p.Addr, Count: p.Count})
	}

	go func() {
		defer sec.reading.Store(false)

		// Step 1: Read BMS info + protection registers
		results := h.broker.ReadBatch(h.ctx, reads)

		// Step 2: Detect topology from 0x900D
		var detectedStr string
		var mismatch bool
		for i, p := range probes {
			if p.Addr == 0x900D && i < len(results) && results[i].Err == nil && len(results[i].Data) >= 2 {
				val := binary.BigEndian.Uint16(results[i].Data[:2])
				pStr, pPack := register.DecodeTopology(val)
				detectedStr = fmt.Sprintf("%d strings x %d packs", pStr, pPack)
				totalTowers := inputs * towers
				if pStr != totalTowers || pPack != packs {
					mismatch = true
				}
				break
			}
		}

		// Step 3: Extract bitmap from 0x9022 standard read result (no write cycle needed)
		totalTowers := inputs * towers
		onlineBitmaps := make([]uint16, totalTowers)
		for i, p := range probes {
			if p.Addr == 0x9022 && i < len(results) && results[i].Err == nil && len(results[i].Data) >= 2 {
				bitmapVal := binary.BigEndian.Uint16(results[i].Data[:2])
				// Apply same bitmap to all towers at overview level
				// Phase 5 will implement per-tower cycling via 0x9020 writes
				for t := 0; t < totalTowers; t++ {
					onlineBitmaps[t] = bitmapVal
				}
				break
			}
		}

		// Step 4: Build BMS info GroupData
		groupDataSlice := h.buildBMSGroupData(groups, probes, results[:len(probes)])

		// Step 5: Add bitmap group (Type="bitmap")
		bitmapGroup := GroupData{
			Name: "Battery Topology",
			Type: "bitmap",
			Bitmap: &BitmapData{
				Towers:           totalTowers,
				PacksPerTower:    packs,
				Online:           onlineBitmaps,
				DetectedTopology: detectedStr,
				Mismatch:         mismatch,
			},
		}
		groupDataSlice = append(groupDataSlice, bitmapGroup)

		// Step 6: Add protection group (Type="protection")
		protResults := results[len(probes):]
		protGroup := h.buildProtectionGroup(protectionProbes, protResults)
		groupDataSlice = append(groupDataSlice, protGroup)

		h.results <- sectionResult{
			section: "bms",
			msg:     NewGroupedSectionData("bms", groupDataSlice, nil),
		}
	}()
}

// buildGroupedResult builds a standard grouped OutboundMessage from probe groups and results.
// Used by battery and stats sections that have no special composition logic.
func (h *Hub) buildGroupedResult(section string, groups []register.ProbeGroup, probes []register.Probe, results []broker.Result) OutboundMessage {
	var hasError bool
	var errMsg string

	for _, r := range results {
		if r.Err != nil {
			hasError = true
			errMsg = r.Err.Error()
		}
	}

	groupDataSlice := make([]GroupData, 0, len(groups))
	probeIdx := 0
	for _, g := range groups {
		gd := GroupData{
			Name:   g.Name,
			Layout: g.Layout,
			Items:  make(map[string]string),
		}
		for _, p := range g.Probes {
			if probeIdx >= len(results) {
				break
			}
			r := results[probeIdx]
			probeIdx++
			if r.Err != nil {
				continue
			}
			gd.Items[p.Name] = FormatValue(p, r.Data)
		}
		groupDataSlice = append(groupDataSlice, gd)
	}

	if hasError {
		return NewSectionError(section, errMsg)
	}
	return NewGroupedSectionData(section, groupDataSlice, nil)
}

// buildBMSGroupData builds GroupData for BMS info groups with special handling for
// BMS system clock composition (0x9004+0x9005 -> DecodeBMSClock) and SW version composition.
func (h *Hub) buildBMSGroupData(groups []register.ProbeGroup, probes []register.Probe, results []broker.Result) []GroupData {
	groupDataSlice := make([]GroupData, 0, len(groups))
	probeIdx := 0

	for _, g := range groups {
		gd := GroupData{
			Name:   g.Name,
			Layout: g.Layout,
			Items:  make(map[string]string),
		}

		// Collect BMS clock registers for composition
		var clockHi, clockLo uint16
		var hasClockHi, hasClockLo bool

		// Collect SW version components
		var swChar string
		var swMajor, swNonStd, swMinor uint16
		var hasSWChar, hasSWMajor, hasSWNonStd, hasSWMinor bool

		for _, p := range g.Probes {
			if probeIdx >= len(results) {
				break
			}
			r := results[probeIdx]
			probeIdx++

			if r.Err != nil {
				continue
			}

			// Handle BMS clock composition: 0x9004 (hi) + 0x9005 (lo) -> DecodeBMSClock
			switch p.Addr {
			case 0x9004:
				if len(r.Data) >= 2 {
					clockHi = binary.BigEndian.Uint16(r.Data[:2])
					hasClockHi = true
				}
				continue
			case 0x9005:
				if len(r.Data) >= 2 {
					clockLo = binary.BigEndian.Uint16(r.Data[:2])
					hasClockLo = true
				}
				continue
			// Handle SW version composition: 0x9018 (char) + 0x9019 (major) + 0x901A (non-std) + 0x901B (minor)
			case 0x9018:
				swChar = FormatValue(p, r.Data)
				hasSWChar = true
				continue
			case 0x9019:
				if len(r.Data) >= 2 {
					swMajor = binary.BigEndian.Uint16(r.Data[:2])
					hasSWMajor = true
				}
				continue
			case 0x901A:
				if len(r.Data) >= 2 {
					swNonStd = binary.BigEndian.Uint16(r.Data[:2])
					hasSWNonStd = true
				}
				continue
			case 0x901B:
				if len(r.Data) >= 2 {
					swMinor = binary.BigEndian.Uint16(r.Data[:2])
					hasSWMinor = true
				}
				continue
			case 0x900D:
				// Topology: decode and show human-readable
				if len(r.Data) >= 2 {
					val := binary.BigEndian.Uint16(r.Data[:2])
					pStr, pPack := register.DecodeTopology(val)
					gd.Items[p.Name] = fmt.Sprintf("%d strings x %d packs (0x%04X)", pStr, pPack, val)
				}
				continue
			}

			gd.Items[p.Name] = FormatValue(p, r.Data)
		}

		// Compose BMS clock if both halves present
		if hasClockHi && hasClockLo {
			clockVal := uint32(clockHi)<<16 | uint32(clockLo)
			gd.Items["System Clock"] = register.DecodeBMSClock(clockVal)
		}

		// Compose SW version if all components present
		if hasSWChar && hasSWMajor && hasSWNonStd && hasSWMinor {
			gd.Items["SW Version"] = fmt.Sprintf("%s%d.%d.%d", swChar, swMajor, swNonStd, swMinor)
		}

		groupDataSlice = append(groupDataSlice, gd)
	}

	return groupDataSlice
}

// buildProtectionGroup creates a GroupData with Type="protection" from BMS protection/alarm registers.
// Values are formatted as hex for bitmap inspection.
func (h *Hub) buildProtectionGroup(probes []register.Probe, results []broker.Result) GroupData {
	gd := GroupData{
		Name:  "Protection & Alarms",
		Type:  "protection",
		Items: make(map[string]string),
	}
	for i, p := range probes {
		if i >= len(results) || results[i].Err != nil {
			continue
		}
		if len(results[i].Data) >= 2 {
			val := binary.BigEndian.Uint16(results[i].Data[:2])
			gd.Items[p.Name] = fmt.Sprintf("0x%04X", val)
		}
	}
	return gd
}

// sendToClient marshals a message and sends it to a single client.
// Non-blocking: if the client's send buffer is full, the client is closed.
func (h *Hub) sendToClient(c *Client, msg OutboundMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error("marshal outbound message", "error", err)
		return
	}
	select {
	case c.send <- data:
	default:
		// Slow client -- close it (gorilla chat pattern, T-02-05)
		h.logger.Warn("client send buffer full, closing", "section", c.section)
		h.removeClient(c)
	}
}

// broadcastAll sends a message to every connected client.
func (h *Hub) broadcastAll(msg OutboundMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error("marshal broadcast message", "error", err)
		return
	}
	for client := range h.clients {
		select {
		case client.send <- data:
		default:
			h.removeClient(client)
		}
	}
}

// broadcastToSection sends a message to all subscribers of a named section.
func (h *Hub) broadcastToSection(sectionName string, msg OutboundMessage) {
	sec, ok := h.sections[sectionName]
	if !ok {
		return
	}
	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error("marshal section broadcast", "error", err)
		return
	}
	for client := range sec.subscribers {
		select {
		case client.send <- data:
		default:
			// Slow client -- close it (gorilla chat pattern)
			h.removeClient(client)
		}
	}
}

// removeClient unsubscribes a client from its section, removes from clients map, and closes send channel.
func (h *Hub) removeClient(c *Client) {
	if _, ok := h.clients[c]; !ok {
		return
	}
	if c.section != "" {
		h.unsubscribeClient(c, c.section)
	}
	delete(h.clients, c)
	close(c.send)
	h.logger.Debug("client removed", "clients", len(h.clients))
}

// shutdown closes all client connections and clears maps.
func (h *Hub) shutdown() {
	for c := range h.clients {
		close(c.send)
		delete(h.clients, c)
	}
	for _, sec := range h.sections {
		sec.stopTimer()
	}
	h.logger.Info("hub shut down")
}

// registerBuiltinSections registers core monitoring sections on hub startup.
func (h *Hub) registerBuiltinSections() {
	h.RegisterGroupedSection("system", register.SystemGroups)
	h.RegisterGroupedSection("grid", register.GridGroups)
	h.RegisterGroupedSection("eps", register.EPSGroups)
	h.RegisterGroupedSection("pv", register.GeneratePVGroups(h.defaultPVChannels))
	h.RegisterGroupedSection("battery", register.GenerateBatteryGroups(2)) // default 2 channels, auto-detect on read
	h.registerBMSSection()
	h.RegisterGroupedSection("stats", register.StatisticsGroups())
}

// registerBMSSection creates the BMS section. BMS has a custom read cycle
// (write-read for bitmap), so it is registered separately.
func (h *Hub) registerBMSSection() {
	groups := register.BMSInfoGroups()
	sec := newGroupedSection("bms", groups, h.timerCh, h.logger)
	if h.refreshOverride > 0 {
		sec.SetInterval(h.refreshOverride)
	}
	h.sections["bms"] = sec
}

// handleConfigure handles configure messages (D-15).
// Supports "pv" (channel count) and "bms" (topology: inputs, towers, packs).
func (h *Hub) handleConfigure(cmd ClientCommand) {
	msg := cmd.Message
	if msg.Config == nil {
		return
	}

	switch msg.Section {
	case "pv":
		channels := msg.Config.Channels
		// Clamp to valid range (T-03-03)
		if channels < 2 {
			channels = 2
		}
		if channels > 16 {
			channels = 16
		}

		// Rebuild PV section with new channel count
		newGroups := register.GeneratePVGroups(channels)
		sec, ok := h.sections["pv"]
		if !ok {
			return
		}

		sec.Groups = newGroups
		sec.Probes = flattenProbeGroups(newGroups)

		h.logger.Info("pv section reconfigured", "channels", channels)

		// Trigger immediate re-read if connected and section has subscribers
		if h.connected && sec.SubscriberCount() > 0 {
			h.triggerSectionRead("pv")
		}

	case "bms":
		inputs := msg.Config.BatInputs
		towers := msg.Config.BatTowers
		packs := msg.Config.BatPacks
		// Clamp to valid ranges (T-04-03)
		if inputs < 1 {
			inputs = 1
		}
		if inputs > 2 {
			inputs = 2
		}
		if towers < 1 {
			towers = 1
		}
		if towers > 4 {
			towers = 4
		}
		if packs < 4 {
			packs = 4
		}
		if packs > 10 {
			packs = 10
		}
		h.defaultBatInputs = inputs
		h.defaultBatTowers = towers
		h.defaultBatPacks = packs
		h.logger.Info("bms section reconfigured", "inputs", inputs, "towers", towers, "packs", packs)
		sec, ok := h.sections["bms"]
		if !ok {
			return
		}
		if h.connected && sec.SubscriberCount() > 0 {
			h.triggerSectionRead("bms")
		}
	}
}

// === Phase 05 Plan 02: Pack selection and data retrieval ===

// handleSelectPack validates pack coordinates and triggers the write-settle-read cycle.
func (h *Hub) handleSelectPack(cmd ClientCommand) {
	msg := cmd.Message
	input := msg.Input
	tower := msg.Tower
	pack := msg.Pack

	// Validate and clamp to topology bounds (T-05-02)
	if input < 1 {
		input = 1
	}
	if input > h.defaultBatInputs {
		input = h.defaultBatInputs
	}
	if tower < 1 {
		tower = 1
	}
	if tower > h.defaultBatTowers {
		tower = h.defaultBatTowers
	}
	if pack < 1 {
		pack = 1
	}
	if pack > h.defaultBatPacks {
		pack = h.defaultBatPacks
	}

	// Store selection for auto-refresh
	h.selectedPack = &packSelection{input: input, tower: tower, pack: pack, client: cmd.Client}

	h.triggerPackRead(input, tower, pack, cmd.Client)
}

// triggerPackRead performs the write-settle-read cycle for pack data retrieval.
// Writes 0x9020 to select the pack, waits for settle, then reads 3 register blocks.
func (h *Hub) triggerPackRead(input, tower, pack int, client *Client) {
	towersPerInput := h.defaultBatTowers
	queryWord := register.EncodePackQuery(input, tower, pack, towersPerInput)

	// Get probe definitions for building the response
	rtProbes := register.PackRTProbes()
	infoProbes := register.PackInfoProbes()
	temps58Probes := register.PackTemps58Probes()

	go func() {
		// Step 1: Write 0x9020 to select pack (function 0x10 via WriteRegister)
		err := h.broker.WriteRegister(h.ctx, 0x9020, queryWord)
		if err != nil {
			h.logger.Warn("pack select write failed, retrying with 2s settle", "error", err)
			// Retry once with 2s settle
			time.Sleep(2 * time.Second)
			err = h.broker.WriteRegister(h.ctx, 0x9020, queryWord)
			if err != nil {
				h.logger.Error("pack select write failed after retry", "error", err)
				h.sendPackError(client, input, tower, pack, "timeout writing pack selection after retry")
				return
			}
		}

		// Step 2: Wait 1s settle time
		time.Sleep(1 * time.Second)

		// Step 3: Read Pack RT data (0x9044-0x907F = 60 registers)
		rtReads := []broker.ReadRequest{{Addr: 0x9044, Count: 60}}
		rtResults := h.broker.ReadBatch(h.ctx, rtReads)
		if len(rtResults) == 0 || rtResults[0].Err != nil {
			errMsg := "timeout reading pack RT data"
			if len(rtResults) > 0 && rtResults[0].Err != nil {
				errMsg = rtResults[0].Err.Error()
			}
			h.sendPackError(client, input, tower, pack, errMsg)
			return
		}

		// Step 4: Read Pack Info (0x9104-0x9126 = 35 registers)
		infoReads := []broker.ReadRequest{{Addr: 0x9104, Count: 35}}
		infoResults := h.broker.ReadBatch(h.ctx, infoReads)

		// Step 5: Read Temps 5-8 (0x90BC-0x90BF = 4 registers)
		temps58Reads := []broker.ReadRequest{{Addr: 0x90BC, Count: 4}}
		temps58Results := h.broker.ReadBatch(h.ctx, temps58Reads)

		// Step 6: Build and send pack data message
		msg := h.buildPackDataMessage(input, tower, pack, rtProbes, infoProbes, temps58Probes, rtResults, infoResults, temps58Results)
		h.sendPackDataToClient(client, msg)
	}()
}

// buildPackDataMessage assembles a PackDataMessage from the 3 register read results.
func (h *Hub) buildPackDataMessage(
	input, tower, pack int,
	rtProbes, infoProbes, temps58Probes []register.Probe,
	rtResults, infoResults, temps58Results []broker.Result,
) PackDataMessage {
	msg := PackDataMessage{
		Type:      MsgTypePackData,
		Section:   "bms",
		Input:     input,
		Tower:     tower,
		Pack:      pack,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	var rtData []byte
	if len(rtResults) > 0 && rtResults[0].Err == nil {
		rtData = rtResults[0].Data
	}

	var infoData []byte
	if len(infoResults) > 0 && infoResults[0].Err == nil {
		infoData = infoResults[0].Data
	}

	var temps58Data []byte
	if len(temps58Results) > 0 && temps58Results[0].Err == nil {
		temps58Data = temps58Results[0].Data
	}

	// Group 1: Pack Info (from info block)
	packInfoGroup := PackGroup{
		Name:  "Pack Info",
		Items: make(map[string]string),
	}
	if infoData != nil {
		for _, p := range infoProbes {
			// Skip alarm/protection/fault registers (handled in Pack Status group)
			if p.Addr >= 0x9124 && p.Addr <= 0x9126 {
				continue
			}
			offset := int(p.Addr-0x9104) * 2
			end := offset + int(p.Count)*2
			if offset >= 0 && end <= len(infoData) {
				packInfoGroup.Items[p.Name] = FormatValue(p, infoData[offset:end])
			}
		}
	}
	// Also add some RT info items
	if rtData != nil {
		// Pack ID, Serial Number from RT block
		for _, p := range rtProbes {
			if p.Name == "Pack ID" || p.Name == "Serial Number" ||
				p.Name == "Remaining Capacity" || p.Name == "Full Charge Capacity" ||
				p.Name == "Cycle Count" || p.Name == "Total Voltage" || p.Name == "SOC" ||
				p.Name == "Total Packs" || p.Name == "Cell Count" || p.Name == "Current" {
				offset := int(p.Addr-0x9044) * 2
				end := offset + int(p.Count)*2
				if offset >= 0 && end <= len(rtData) {
					packInfoGroup.Items[p.Name] = FormatValue(p, rtData[offset:end])
				}
			}
		}
	}
	msg.Groups = append(msg.Groups, packInfoGroup)

	// Group 2: Cell Voltages (type="cell_grid")
	cellGroup := PackGroup{
		Name: "Cell Voltages",
		Type: "cell_grid",
	}
	if rtData != nil {
		cells := make([]int, 24)
		for i := 0; i < 24; i++ {
			cells[i] = int(extractU16(rtData, 0x9044, uint16(0x9051+i)))
		}
		cellGroup.Cells = cells
		cellGroup.MaxCell = int(extractU16(rtData, 0x9044, 0x9069))
		cellGroup.MinCell = int(extractU16(rtData, 0x9044, 0x906A))

		// Compute max/min cell index by scanning the 24 values
		maxIdx, minIdx := 1, 1
		maxVal, minVal := cells[0], cells[0]
		for i, v := range cells {
			if v > maxVal {
				maxVal = v
				maxIdx = i + 1
			}
			if v < minVal || (minVal == 0 && v > 0) {
				minVal = v
				minIdx = i + 1
			}
		}
		cellGroup.MaxCellIndex = maxIdx
		cellGroup.MinCellIndex = minIdx
	}
	msg.Groups = append(msg.Groups, cellGroup)

	// Group 3: Temperatures
	tempGroup := PackGroup{
		Name:  "Temperatures",
		Items: make(map[string]string),
	}
	tempRaw := make([]int, 0)
	if rtData != nil {
		// Temps 1-4 from RT block
		for _, p := range rtProbes {
			if strings.HasPrefix(p.Name, "Temp ") || p.Name == "MOS Temp" || p.Name == "Env Temp" {
				offset := int(p.Addr-0x9044) * 2
				end := offset + int(p.Count)*2
				if offset >= 0 && end <= len(rtData) {
					tempGroup.Items[p.Name] = FormatValue(p, rtData[offset:end])
					tempRaw = append(tempRaw, int(extractS16(rtData, 0x9044, p.Addr)))
				}
			}
		}
	}
	if temps58Data != nil {
		// Temps 5-8 from temps58 block
		for _, p := range temps58Probes {
			offset := int(p.Addr-0x90BC) * 2
			end := offset + int(p.Count)*2
			if offset >= 0 && end <= len(temps58Data) {
				tempGroup.Items[p.Name] = FormatValue(p, temps58Data[offset:end])
				tempRaw = append(tempRaw, int(extractS16(temps58Data, 0x90BC, p.Addr)))
			}
		}
	}
	tempGroup.TempRaw = tempRaw
	msg.Groups = append(msg.Groups, tempGroup)

	// Group 4: Pack Status (type="pack_status")
	statusGroup := PackGroup{
		Name: "Pack Status",
		Type: "pack_status",
	}
	if rtData != nil {
		statusGroup.Alarm = int(extractU16(rtData, 0x9044, 0x9076))
		statusGroup.Protection = int(extractU16(rtData, 0x9044, 0x9077))
		statusGroup.Fault = int(extractU16(rtData, 0x9044, 0x9078))
	}
	if infoData != nil {
		statusGroup.Alarm2 = int(extractU16(infoData, 0x9104, 0x9124))
		statusGroup.Protection2 = int(extractU16(infoData, 0x9104, 0x9125))
		statusGroup.Fault2 = int(extractU16(infoData, 0x9104, 0x9126))
	}
	// Decode all bitmaps
	var decoded []string
	decoded = append(decoded, register.DecodeBMSBitmap(uint16(statusGroup.Alarm), register.BMSAlarmBits, 0x9076)...)
	decoded = append(decoded, register.DecodeBMSBitmap(uint16(statusGroup.Protection), register.BMSProtectionBits, 0x9077)...)
	decoded = append(decoded, register.DecodeBMSBitmap(uint16(statusGroup.Fault), register.BMSFaultBits, 0x9078)...)
	decoded = append(decoded, register.DecodeBMSBitmap(uint16(statusGroup.Alarm2), register.BMSAlarm2Bits, 0x9124)...)
	decoded = append(decoded, register.DecodeBMSBitmap(uint16(statusGroup.Protection2), register.BMSProtection2Bits, 0x9125)...)
	decoded = append(decoded, register.DecodeBMSBitmap(uint16(statusGroup.Fault2), register.BMSFault2Bits, 0x9126)...)
	statusGroup.Decoded = decoded
	msg.Groups = append(msg.Groups, statusGroup)

	// Group 5: Balance State (type="balance")
	balanceGroup := PackGroup{
		Name: "Balance State",
		Type: "balance",
	}
	if rtData != nil {
		balanceGroup.BalanceBitmap = int(extractU16(rtData, 0x9044, 0x9075))
	}
	msg.Groups = append(msg.Groups, balanceGroup)

	return msg
}

// extractU16 extracts a uint16 at targetAddr from a data block starting at baseAddr.
func extractU16(data []byte, baseAddr, targetAddr uint16) uint16 {
	offset := int(targetAddr-baseAddr) * 2
	if offset < 0 || offset+2 > len(data) {
		return 0
	}
	return binary.BigEndian.Uint16(data[offset : offset+2])
}

// extractS16 extracts an int16 at targetAddr from a data block starting at baseAddr.
func extractS16(data []byte, baseAddr, targetAddr uint16) int16 {
	return int16(extractU16(data, baseAddr, targetAddr))
}

// sendPackError sends a pack_error message to a specific client.
func (h *Hub) sendPackError(client *Client, input, tower, pack int, errMsg string) {
	msg := PackErrorMessage{
		Type:    MsgTypePackError,
		Section: "bms",
		Input:   input,
		Tower:   tower,
		Pack:    pack,
		Error:   errMsg,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error("marshal pack error message", "error", err)
		return
	}
	select {
	case client.send <- data:
	default:
	}
}

// sendPackDataToClient sends a PackDataMessage to a specific client.
func (h *Hub) sendPackDataToClient(client *Client, msg PackDataMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error("marshal pack data message", "error", err)
		return
	}
	select {
	case client.send <- data:
	default:
	}
}
