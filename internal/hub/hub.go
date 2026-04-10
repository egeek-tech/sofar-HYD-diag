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
	h.logger.Debug("client subscribed", "section", sectionName, "subscribers", sec.SubscriberCount())

	// Trigger immediate read (D-20)
	if h.connected {
		h.triggerSectionRead(sectionName)
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
	h.triggerSectionRead(sectionName)
}

// triggerSectionRead initiates a Modbus read for all probes in a section.
// The read runs in a goroutine; results are broadcast to section subscribers.
func (h *Hub) triggerSectionRead(sectionName string) {
	sec, ok := h.sections[sectionName]
	if !ok {
		return
	}

	sec.reading.Store(true)

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

// registerBuiltinSections registers the 4 core monitoring sections (D-24: demo "status" retired).
func (h *Hub) registerBuiltinSections() {
	h.RegisterGroupedSection("system", register.SystemGroups)
	h.RegisterGroupedSection("grid", register.GridGroups)
	h.RegisterGroupedSection("eps", register.EPSGroups)
	h.RegisterGroupedSection("pv", register.GeneratePVGroups(h.defaultPVChannels))
}

// handleConfigure handles configure messages (D-15).
// Currently only "pv" section supports reconfiguration (channel count).
func (h *Hub) handleConfigure(cmd ClientCommand) {
	msg := cmd.Message
	if msg.Section != "pv" || msg.Config == nil {
		return // silently ignore non-pv configure or missing config
	}

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
}
