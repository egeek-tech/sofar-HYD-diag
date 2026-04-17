package hub

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log/slog"
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
// msg is interface{} to support OutboundMessage, RegisterValueMessage, SectionCompleteMessage,
// and SectionSchemaMessage polymorphically during streaming reads.
type sectionResult struct {
	section string
	msg     interface{}
}

// === Battery Topology Constants (Phase 06, D-02) ===
// Hardcoded to match actual hardware: 1 input, 2 towers, 10 packs/tower, 16 cells/pack.
const (
	TopoInputs        = 1  // single battery input supported
	TopoTowers        = 2  // 2 towers (groups/strings)
	TopoPacksPerTower = 10 // 10 packs per tower
	TopoCellsPerPack  = 16 // 16 cells per battery pack
)

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
	ctx        context.Context
	cancel     context.CancelFunc
	connected          bool          // tracks whether broker is connected (D-28)
	defaultPVChannels  int           // default PV channel count for PV section (D-16)
	selectedPack       *packSelection // currently selected pack for auto-refresh (nil = no pack selected)
	readDelayMs        int           // configurable inter-read delay in milliseconds (D-04, default 500)
	packSettleMs       int           // configurable pack settle time in milliseconds (D-04, default 1000)
	packSpanTracker *SpanTracker // per-pack span degradation tracker (Phase 25, D-04)
}

// packSelection tracks the currently selected pack for auto-refresh re-triggering.
type packSelection struct {
	input, tower, pack int
	client             *Client
}

// NewHub creates a new Hub. Call Run() in a goroutine to start processing.
// pvChannels sets the default number of PV channels (clamped to 2-16).
func NewHub(b BrokerInterface, logger *slog.Logger, pvChannels int) *Hub {
	if pvChannels < 2 {
		pvChannels = 2
	}
	if pvChannels > 16 {
		pvChannels = 16
	}
	h := &Hub{
		broker:            b,
		logger:            logger.With("component", "hub"),
		clients:           make(map[*Client]bool),
		sections:          make(map[string]*Section),
		register:          make(chan *Client, 16),
		unregister:        make(chan *Client, 16),
		commands:          make(chan ClientCommand, 64),
		funcs:             make(chan func(), 8),
		results:           make(chan sectionResult, 32),
		defaultPVChannels: pvChannels,
	}
	h.readDelayMs = 500
	h.packSettleMs = 1000
	h.packSpanTracker = NewSpanTracker(DefaultDegradationThreshold, h.logger.With("component", "pack-tracker"))
	// Initialize ctx/cancel so ClientCount and RunFunc are safe before Run().
	// Run() replaces this with the caller-provided context.
	h.ctx, h.cancel = context.WithCancel(context.Background())
	return h
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
	sec := newSection(name, probes, h.logger)
	h.sections[name] = sec
}

// RegisterGroupedSection creates a ProbeGroup-based Section and adds it to the hub.
// Must be called before Run() or from within the Run goroutine.
func (h *Hub) RegisterGroupedSection(name string, groups []register.ProbeGroup) {
	sec := newGroupedSection(name, groups, h.logger)
	h.sections[name] = sec
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
	h.cancel() // cancel the placeholder context from NewHub
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
		case res := <-h.results:
			// D-09: Mark read-once sections as cached when section_complete arrives
			if _, ok := res.msg.(SectionCompleteMessage); ok {
				if sec, ok := h.sections[res.section]; ok && sec.readOnce && !sec.hasReadOnce {
					sec.hasReadOnce = true
					h.logger.Debug("read-once section cached", "section", res.section)
				}
			}
			h.broadcastResultToSection(res.section, res.msg)
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
		// D-01: Cancel all section reads FIRST so streaming goroutines exit
		for _, sec := range h.sections {
			sec.cancelRead()
		}
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
		// D-09: Reset read-once cache on explicit refresh
		if sec, ok := h.sections[msg.Section]; ok && sec.readOnce {
			sec.hasReadOnce = false
		}
		h.triggerSectionRead(msg.Section)
	case MsgTypeReadCycle:
		h.handleReadCycle(cmd.Client, msg)
	case MsgTypeConfigure:
		h.handleConfigure(cmd)
	case MsgTypeSelectPack:
		h.handleSelectPack(cmd)
	default:
		h.logger.Debug("unknown message type", "type", msg.Type)
	}
}

// handleStateEvent broadcasts state changes to all clients and cancels reads on disconnect.
func (h *Hub) handleStateEvent(evt broker.StateEvent) {
	var errMsg string
	if evt.Err != nil {
		errMsg = evt.Err.Error()
	}
	h.broadcastAll(NewStateMessage(evt.State.String(), errMsg))

	switch evt.State {
	case broker.StateConnected:
		h.connected = true
		// D-02: Reset SpanTrackers on reconnect to give spans a fresh start.
		for _, sec := range h.sections {
			if sec.SpanTracker != nil {
				sec.SpanTracker.Reset()
			}
		}
		if h.packSpanTracker != nil {
			h.packSpanTracker.Reset()
		}
	case broker.StateDisconnected, broker.StateReconnecting:
		h.connected = false
		// Cancel all in-progress reads (D-13: disconnection stops reads)
		for _, sec := range h.sections {
			sec.cancelRead()
		}
	}
}

// subscribeClient subscribes a client to a section.
// Unsubscribes from any previous section first (D-18: one section at a time).
// Triggers an immediate read (D-20).
func (h *Hub) subscribeClient(c *Client, sectionName string) {
	// Unsubscribe from previous section if any (D-18)
	if c.section != "" && c.section != sectionName {
		// Cancel in-progress read for old section (D-02)
		if oldSec, ok := h.sections[c.section]; ok {
			oldSec.cancelRead()
		}
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

	// Send section schema for skeleton pre-rendering (STREAM-02, D-02)
	if sec.Groups != nil {
		schema := h.buildSectionSchema(sectionName, sec)
		schemaData, err := json.Marshal(schema)
		if err == nil {
			select {
			case c.send <- schemaData:
			default:
				h.removeClient(c)
				return
			}
		}
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
	// Cancel in-progress read if no subscribers remain
	if sec.SubscriberCount() == 0 {
		sec.cancelRead()
	}
	h.logger.Debug("client unsubscribed", "section", sectionName, "subscribers", sec.SubscriberCount())
}

// handleReadCycle handles a browser-driven read_cycle message (REFR-01).
// Triggers a section read if connected, has subscribers, and no read is in progress.
func (h *Hub) handleReadCycle(c *Client, msg InboundMessage) {
	if !h.connected {
		return
	}
	sec, ok := h.sections[msg.Section]
	if !ok {
		return
	}
	if sec.SubscriberCount() == 0 {
		return
	}
	// Skip if already reading (same logic as old handleTimerTick D-24)
	if sec.reading.Load() {
		h.logger.Debug("skipping overlapping read_cycle", "section", msg.Section)
		return
	}
	// D-09: Skip re-read for cached read-once sections
	if sec.readOnce && sec.hasReadOnce {
		h.logger.Debug("skipping read_cycle for cached read-once section", "section", msg.Section)
		return
	}
	// If BMS with selected pack, trigger pack streaming read instead
	if msg.Section == "bms" && h.selectedPack != nil {
		sec.cancelRead()
		readCtx, cancel := context.WithCancel(h.ctx)
		sec.readCancel = cancel
		sec.reading.Store(true)
		h.streamPackBatchRead(h.selectedPack.input, h.selectedPack.tower, h.selectedPack.pack, h.selectedPack.client, readCtx)
		return
	}
	h.triggerSectionRead(msg.Section)
}

// triggerSectionRead initiates a Modbus read for all probes in a section.
// The read runs in a goroutine; results are broadcast to section subscribers.
// Cancels any previous in-progress read before starting (D-02).
// BMS and battery sections have custom read cycles; all others use the standard path.
func (h *Hub) triggerSectionRead(sectionName string) {
	sec, ok := h.sections[sectionName]
	if !ok {
		return
	}

	// Cancel previous read if still running (D-02: prevents orphaned goroutines)
	sec.cancelRead()

	readCtx, cancel := context.WithCancel(h.ctx)
	sec.readCancel = cancel
	sec.reading.Store(true)

	// Dispatch to streaming read handlers (Phase 07: per-register streaming)
	switch sectionName {
	case "bms":
		h.streamBMSBatchRead(sec, readCtx)
		return
	case "battery":
		h.streamBatteryBatchRead(sec, readCtx)
		return
	}

	// Standard streaming read path for system, grid, eps, pv
	h.streamStandardRead(sectionName, sec, readCtx)
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

// broadcastResultToSection marshals any message type (OutboundMessage, RegisterValueMessage,
// SectionCompleteMessage, SectionSchemaMessage) and sends to all section subscribers.
// Used by the streaming results channel which carries interface{} messages.
func (h *Hub) broadcastResultToSection(sectionName string, msg interface{}) {
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
	c.closed.Store(true) // CR-01: signal goroutines before closing channel
	close(c.send)
	h.logger.Debug("client removed", "clients", len(h.clients))
}

// shutdown closes all client connections and clears maps.
func (h *Hub) shutdown() {
	for c := range h.clients {
		c.closed.Store(true) // CR-01: signal goroutines before closing channel
		close(c.send)
		delete(h.clients, c)
	}
	for _, sec := range h.sections {
		sec.cancelRead()
	}
	h.logger.Info("hub shut down")
}

// registerBuiltinSections registers core monitoring sections on hub startup.
func (h *Hub) registerBuiltinSections() {
	h.RegisterGroupedSection("system", append(register.SystemGroups, register.StatisticsGroups()...))
	h.RegisterGroupedSection("configuration", register.ConfigurationGroups) // D-07: after system, D-09: read-once
	h.RegisterGroupedSection("grid", register.GridGroups)
	h.RegisterGroupedSection("eps", register.EPSGroups)
	h.RegisterGroupedSection("pv", register.GeneratePVGroups(h.defaultPVChannels))
	h.RegisterGroupedSection("battery", append(register.GenerateBatteryGroups(2), register.InternalInfoGroups()...)) // default 2 channels, auto-detect on read
	h.registerBMSSection()

	// New sections from XLSX register discovery (Phase 17)
	h.RegisterGroupedSection("meter", register.MeterGroups)
	// DCDC section removed: all registers returned illegal data address / timeout
	// on real hardware (2026-04-15). See tools/section-sweep/results.json.
	h.RegisterGroupedSection("pcu", register.PCUGroups)
	h.RegisterGroupedSection("bdu", register.BDUGroups)

	// D-09: Configuration section uses read-once caching (static device settings)
	if sec, ok := h.sections["configuration"]; ok {
		sec.readOnce = true
	}
}

// registerBMSSection creates the BMS section. BMS has a custom read cycle
// (write-read for bitmap), so it is registered separately.
func (h *Hub) registerBMSSection() {
	groups := register.BMSInfoGroups()
	sec := newGroupedSection("bms", groups, h.logger)
	h.sections["bms"] = sec
}

// handleConfigure handles configure messages (D-15).
// Supports "pv" (channel count). BMS topology is now hardcoded via constants (Phase 06).
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
		sec.BatchPlan = register.AnalyzeBatchPlan(newGroups)
		sec.SpanTracker.Reset()

		h.logger.Info("pv section reconfigured", "channels", channels)

		// Trigger immediate re-read if connected and section has subscribers
		if h.connected && sec.SubscriberCount() > 0 {
			h.triggerSectionRead("pv")
		}
	case "timing":
		if msg.TimingConfig == nil {
			return
		}
		tc := msg.TimingConfig

		// Update read delay with server-side clamping (T-07-01, T-07-02)
		if tc.ReadDelayMs > 0 {
			delay := tc.ReadDelayMs
			if delay < 50 {
				delay = 50
			}
			if delay > 5000 {
				delay = 5000
			}
			h.readDelayMs = delay
			// Update broker runtime delay via command channel (thread-safe per Pitfall 3)
			go func() {
				if err := h.broker.SetDelayRuntime(h.ctx, time.Duration(delay)*time.Millisecond); err != nil {
					h.logger.Error("failed to update broker delay", "error", err)
				}
			}()
			h.logger.Info("read delay updated", "ms", delay)
		}

		// Update pack settle time with server-side clamping
		if tc.PackSettleMs > 0 {
			settle := tc.PackSettleMs
			if settle < 500 {
				settle = 500
			}
			if settle > 10000 {
				settle = 10000
			}
			h.packSettleMs = settle
			h.logger.Info("pack settle time updated", "ms", settle)
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

	// Validate and clamp to topology bounds (T-05-02, T-06-01)
	if input < 1 || input > TopoInputs {
		input = 1
	}
	if tower < 1 {
		tower = 1
	}
	if tower > TopoTowers {
		tower = TopoTowers
	}
	if pack < 1 {
		pack = 1
	}
	if pack > TopoPacksPerTower {
		pack = TopoPacksPerTower
	}

	// Store selection for auto-refresh
	h.selectedPack = &packSelection{input: input, tower: tower, pack: pack, client: cmd.Client}

	// D-05: Reset pack SpanTracker on pack switch (Phase 25, D-04)
	h.packSpanTracker.Reset()

	// Set up BMS section read context for pack streaming
	sec, ok := h.sections["bms"]
	if !ok {
		return
	}
	sec.cancelRead()
	readCtx, cancel := context.WithCancel(h.ctx)
	sec.readCancel = cancel
	sec.reading.Store(true)

	h.streamPackBatchRead(input, tower, pack, cmd.Client, readCtx)
}

// sendPackError sends a pack_error message to a specific client.
// Checks client.closed to avoid panic on closed channel (CR-01).
func (h *Hub) sendPackError(client *Client, input, tower, pack int, errMsg string) {
	if client.closed.Load() {
		return
	}
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
		h.logger.Warn("pack error message dropped (client buffer full)")
		// Route removal through hub event loop (sendPackError runs outside it).
		select {
		case h.funcs <- func() { h.removeClient(client) }:
		default:
		}
	}
}
