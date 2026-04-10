package hub

import (
	"log/slog"
	"time"
)

// NewTestHub creates a Hub for testing.
// Uses a default logger.
func NewTestHub(b BrokerInterface) *Hub {
	return NewHub(b, slog.Default())
}

// NewTestHubWithInterval creates a Hub for testing with a custom refresh interval.
func NewTestHubWithInterval(b BrokerInterface, interval time.Duration) *Hub {
	h := NewHub(b, slog.Default())
	h.SetRefreshOverride(interval)
	return h
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
