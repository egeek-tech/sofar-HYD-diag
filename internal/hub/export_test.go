package hub

import "log/slog"

// NewTestHub creates a Hub for testing without built-in section registration.
// Uses a discard logger.
func NewTestHub(b BrokerInterface) *Hub {
	return NewHub(b, slog.Default())
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
