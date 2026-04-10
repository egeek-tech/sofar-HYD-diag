package hub

import (
	"log/slog"
	"time"

	"sofar-hyd-diag/internal/register"
)

// NewTestHub creates a Hub for testing with default 2 PV channels and default topology.
// Uses a default logger.
func NewTestHub(b BrokerInterface) *Hub {
	return NewHub(b, slog.Default(), 2, 1, 2, 10)
}

// NewTestHubWithInterval creates a Hub for testing with a custom refresh interval.
func NewTestHubWithInterval(b BrokerInterface, interval time.Duration) *Hub {
	h := NewHub(b, slog.Default(), 2, 1, 2, 10)
	h.SetRefreshOverride(interval)
	return h
}

// NewTestHubWithPVChannels creates a Hub for testing with a specified PV channel count.
func NewTestHubWithPVChannels(b BrokerInterface, pvChannels int) *Hub {
	return NewHub(b, slog.Default(), pvChannels, 1, 2, 10)
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

// HasSection returns true if the hub has a section with the given name.
// Thread-safe: routes the query through the hub event loop.
func (h *Hub) HasSection(name string) bool {
	var exists bool
	h.RunFunc(func() {
		_, exists = h.sections[name]
	})
	return exists
}
