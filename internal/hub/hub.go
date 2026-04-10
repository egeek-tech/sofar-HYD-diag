package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"sofar-hyd-diag/internal/broker"
)

// ClientCommand wraps an inbound message with the originating client.
type ClientCommand struct {
	Client  *Client
	Message InboundMessage
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
	queries    chan chan int // thread-safe client count queries
	timerCh    chan string
	ctx        context.Context
	cancel     context.CancelFunc
	connected  bool // tracks whether broker is connected for timer pause/resume (D-28)
}

// NewHub creates a new Hub. Call Run() in a goroutine to start processing.
func NewHub(b BrokerInterface, logger *slog.Logger) *Hub {
	return &Hub{
		broker:     b,
		logger:     logger.With("component", "hub"),
		clients:    make(map[*Client]bool),
		sections:   make(map[string]*Section),
		register:   make(chan *Client, 16),
		unregister: make(chan *Client, 16),
		commands:   make(chan ClientCommand, 64),
		queries:    make(chan chan int, 8),
		timerCh:    make(chan string, 16),
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
	h.sections[name] = sec
}

// ClientCount returns the number of registered clients.
// Thread-safe: routes the query through the hub event loop.
func (h *Hub) ClientCount() int {
	reply := make(chan int, 1)
	h.queries <- reply
	return <-reply
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
			h.sendToClient(client, NewStateMessage(state.String()))
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
		case reply := <-h.queries:
			reply <- len(h.clients)
		}
	}
}

// handleCommand dispatches based on message type.
func (h *Hub) handleCommand(cmd ClientCommand) {
	msg := cmd.Message
	switch msg.Type {
	case MsgTypeConnect:
		addr := fmt.Sprintf("%s:%d", msg.Host, msg.Port)
		if err := h.broker.Reconfigure(h.ctx, addr, byte(msg.SlaveID)); err != nil {
			h.sendToClient(cmd.Client, NewSectionError("", err.Error()))
		}
	case MsgTypeDisconnect:
		if err := h.broker.Disconnect(h.ctx); err != nil {
			h.logger.Error("disconnect failed", "error", err)
		}
	case MsgTypeSubscribe:
		h.subscribeClient(cmd.Client, msg.Section)
	case MsgTypeUnsubscribe:
		h.unsubscribeClient(cmd.Client, msg.Section)
	case MsgTypeRefresh:
		h.triggerSectionRead(msg.Section)
	case MsgTypeAutoRefresh:
		h.handleAutoRefreshToggle(cmd.Client, msg)
	default:
		h.logger.Debug("unknown message type", "type", msg.Type)
	}
}

// handleStateEvent broadcasts state changes to all clients and manages timer pause/resume.
func (h *Hub) handleStateEvent(evt broker.StateEvent) {
	h.broadcastAll(NewStateMessage(evt.State.String()))

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

	go func() {
		defer sec.reading.Store(false)

		results := h.broker.ReadBatch(h.ctx, reads)

		// Format results and check for errors
		data := make(map[string]string)
		var hasError bool
		var errMsg string

		for i, r := range results {
			if r.Err != nil {
				hasError = true
				errMsg = r.Err.Error()
				continue
			}
			key := toSnakeCase(sec.Probes[i].Name)
			data[key] = FormatValue(sec.Probes[i], r.Data)
		}

		if hasError {
			h.broadcastToSection(sectionName, NewSectionError(sectionName, errMsg))
		}
		if len(data) > 0 {
			h.broadcastToSection(sectionName, NewSectionData(sectionName, data))
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

// registerBuiltinSections registers the default sections.
func (h *Hub) registerBuiltinSections() {
	// Demo "status" section (D-25): SN + Running State + Internal Temp
	statusProbes := []Probe{
		{Name: "Inverter SN", Addr: 0x0445, Count: 10, IsASCII: true},
		{Name: "System running state", Addr: 0x0404, Count: 1},
		{Name: "Ambient temp 1", Addr: 0x0418, Count: 1, Signed: true, Unit: "\u00b0C", Scale: 1},
	}
	h.RegisterSection("status", statusProbes)
}
