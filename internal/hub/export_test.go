package hub

import (
	"log/slog"

	"sofar-hyd-diag/internal/register"
)

// CountBatteryChannels exports countBatteryChannels for external test packages.
var CountBatteryChannels = countBatteryChannels

// NewTestHub creates a Hub for testing with default 2 PV channels.
// Uses a default logger. Topology uses package-level constants.
func NewTestHub(b BrokerInterface) *Hub {
	return NewHub(b, slog.Default(), 2)
}

// NewTestHubWithPVChannels creates a Hub for testing with a specified PV channel count.
func NewTestHubWithPVChannels(b BrokerInterface, pvChannels int) *Hub {
	return NewHub(b, slog.Default(), pvChannels)
}

// NewTestClient creates a Client for testing with a provided send channel.
// No WebSocket connection is needed.
func NewTestClient(h *Hub, send chan []byte) *Client {
	return &Client{
		hub:    h,
		send:   send,
		logger: slog.Default(),
	}
}

// SendReadCycle is a test helper that sends a read_cycle command to the hub.
func SendReadCycle(h *Hub, c *Client, section string) {
	h.Command(c, InboundMessage{Type: MsgTypeReadCycle, Section: section})
}

// GetSectionProbes returns the probe slice for a named section.
// Thread-safe: routes the query through the hub event loop.
func (h *Hub) GetSectionProbes(name string) []Probe {
	var probes []Probe
	h.RunFunc(func() {
		sec, ok := h.sections[name]
		if ok {
			probes = sec.Probes
		}
	})
	return probes
}

// GetSectionGroups returns the ProbeGroup slice for a named section.
// Thread-safe: routes the query through the hub event loop.
func (h *Hub) GetSectionGroups(name string) []register.ProbeGroup {
	var groups []register.ProbeGroup
	h.RunFunc(func() {
		sec, ok := h.sections[name]
		if ok {
			groups = sec.Groups
		}
	})
	return groups
}

// GetSectionBatchPlan returns the BatchPlan for a named section.
// Thread-safe: routes the query through the hub event loop.
func (h *Hub) GetSectionBatchPlan(name string) register.BatchPlan {
	var plan register.BatchPlan
	h.RunFunc(func() {
		sec, ok := h.sections[name]
		if ok {
			plan = sec.BatchPlan
		}
	})
	return plan
}

// HasSection returns true if the hub has a section with the given name.
// Thread-safe: routes the query through the hub event loop.
func (h *Hub) HasSection(name string) bool {
	var exists bool
	h.RunFunc(func() {
		_, exists = h.sections[name]
	})
	return exists
}

// GetTimingConfig returns the hub's current timing configuration values.
// Thread-safe: routes the query through the hub event loop.
func (h *Hub) GetTimingConfig() (readDelayMs, packSettleMs int) {
	h.RunFunc(func() {
		readDelayMs = h.readDelayMs
		packSettleMs = h.packSettleMs
	})
	return
}

// BuildPackSchema exposes buildPackSchema for testing.
func (h *Hub) BuildPackSchema(input, tower, pack int, groups []register.ProbeGroup) SectionSchemaMessage {
	return h.buildPackSchema(input, tower, pack, groups)
}

// GetPackSpanTracker returns the pack drill-down SpanTracker.
// Thread-safe: routes the query through the hub event loop.
func (h *Hub) GetPackSpanTracker() *SpanTracker {
	var tracker *SpanTracker
	h.RunFunc(func() {
		tracker = h.packSpanTracker
	})
	return tracker
}

// GetPackSpanState returns the SpanState for a given span address in the pack SpanTracker.
// Thread-safe: routes the query through the hub event loop.
func (h *Hub) GetPackSpanState(startAddr uint16) SpanState {
	var state SpanState
	h.RunFunc(func() {
		if h.packSpanTracker != nil {
			state = h.packSpanTracker.State(startAddr)
		}
	})
	return state
}

// GetSectionReadOnce returns the readOnce flag for a named section.
// Thread-safe: routes the query through the hub event loop.
func (h *Hub) GetSectionReadOnce(name string) bool {
	var readOnce bool
	h.RunFunc(func() {
		sec, ok := h.sections[name]
		if ok {
			readOnce = sec.readOnce
		}
	})
	return readOnce
}

// GetSectionHasReadOnce returns the hasReadOnce flag for a named section.
// Thread-safe: routes the query through the hub event loop.
func (h *Hub) GetSectionHasReadOnce(name string) bool {
	var hasReadOnce bool
	h.RunFunc(func() {
		sec, ok := h.sections[name]
		if ok {
			hasReadOnce = sec.hasReadOnce
		}
	})
	return hasReadOnce
}

// GetSectionSpanTracker returns the SpanTracker for a named section.
// Thread-safe: routes the query through the hub event loop.
func (h *Hub) GetSectionSpanTracker(name string) *SpanTracker {
	var tracker *SpanTracker
	h.RunFunc(func() {
		sec, ok := h.sections[name]
		if ok {
			tracker = sec.SpanTracker
		}
	})
	return tracker
}

// GetSpanState returns the SpanState for a given span address in a named section.
// Thread-safe: reads SpanTracker state on the hub event loop to avoid data races.
func (h *Hub) GetSpanState(section string, startAddr uint16) SpanState {
	var state SpanState
	h.RunFunc(func() {
		sec, ok := h.sections[section]
		if ok && sec.SpanTracker != nil {
			state = sec.SpanTracker.State(startAddr)
		}
	})
	return state
}
