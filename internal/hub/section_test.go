package hub

import (
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sofar-hyd-diag/internal/register"
)

// TestToSnakeCase verifies the toSnakeCase conversion for various input patterns.
func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Inverter SN", "inverter_sn"},
		{"Total BUS voltage", "total_bus_voltage"},
		{"PCC active power (2)", "pcc_active_power_2"},
		{"already_snake", "already_snake"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := toSnakeCase(tt.input)
			assert.Equal(t, tt.want, got, "toSnakeCase(%q)", tt.input)
		})
	}
}

// TestNewSection verifies that newSection initializes all fields correctly.
func TestNewSection(t *testing.T) {
	probes := []register.Probe{
		{Name: "P1", Addr: 0x0404, Count: 1},
	}
	sec := newSection("test", probes, slog.Default())

	assert.Equal(t, "test", sec.Name)
	assert.Len(t, sec.Probes, 1)
	assert.Equal(t, 0, sec.SubscriberCount())
	assert.Empty(t, sec.BatchPlan.Spans, "BatchPlan.Spans should be empty")
	assert.Nil(t, sec.SpanTracker, "SpanTracker should be nil for newSection")
}

// TestRegisterSection verifies that RegisterSection adds a section to the hub's map.
func TestRegisterSection(t *testing.T) {
	h := NewHub(nil, slog.Default(), 2)

	probes := []register.Probe{
		{Name: "P1", Addr: 0x0404, Count: 1},
	}
	h.RegisterSection("test-sec", probes)

	sec, ok := h.sections["test-sec"]
	require.True(t, ok, "section 'test-sec' not found in hub.sections")
	assert.Equal(t, "test-sec", sec.Name)
	assert.Len(t, sec.Probes, 1)
	assert.Equal(t, 0, sec.SubscriberCount())
}

// TestBroadcastToSection verifies that broadcastToSection delivers messages to subscribers
// and handles nonexistent sections gracefully (no panic).
func TestBroadcastToSection(t *testing.T) {
	h := NewHub(nil, slog.Default(), 2)

	probes := []register.Probe{
		{Name: "P1", Addr: 0x0404, Count: 1},
	}
	h.RegisterSection("bcast", probes)

	// Create a client with a buffered send channel
	send := make(chan []byte, 8)
	c := &Client{
		hub:    h,
		send:   send,
		logger: slog.Default(),
	}

	// Add the client as a subscriber directly (package-internal access)
	h.sections["bcast"].subscribers[c] = true

	// Broadcast a message
	msg := OutboundMessage{
		Type:    "test",
		Section: "bcast",
		Data:    map[string]string{"k": "v"},
	}
	h.broadcastToSection("bcast", msg)

	// Read from send channel with a short timeout
	select {
	case data := <-send:
		var received OutboundMessage
		require.NoError(t, json.Unmarshal(data, &received), "failed to unmarshal received message")
		assert.Equal(t, "test", received.Type)
		assert.Equal(t, "v", received.Data["k"])
	case <-time.After(1 * time.Second):
		require.Fail(t, "timed out waiting for broadcast message")
	}

	// Test with nonexistent section -- should be a no-op, no panic
	h.broadcastToSection("nonexistent", msg)
}
