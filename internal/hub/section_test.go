package hub

import (
	"encoding/json"
	"log/slog"
	"testing"
	"time"

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
			if got != tt.want {
				t.Errorf("toSnakeCase(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestNewSection verifies that newSection initializes all fields correctly.
func TestNewSection(t *testing.T) {
	probes := []register.Probe{
		{Name: "P1", Addr: 0x0404, Count: 1},
	}
	sec := newSection("test", probes, slog.Default())

	if sec.Name != "test" {
		t.Errorf("Name = %q, want %q", sec.Name, "test")
	}
	if len(sec.Probes) != 1 {
		t.Errorf("len(Probes) = %d, want 1", len(sec.Probes))
	}
	if sec.SubscriberCount() != 0 {
		t.Errorf("SubscriberCount() = %d, want 0", sec.SubscriberCount())
	}
	if len(sec.BatchPlan.Spans) != 0 {
		t.Errorf("BatchPlan.Spans should be empty, got %d spans", len(sec.BatchPlan.Spans))
	}
	if sec.SpanTracker != nil {
		t.Errorf("SpanTracker should be nil for newSection, got %v", sec.SpanTracker)
	}
}

// TestRegisterSection verifies that RegisterSection adds a section to the hub's map.
func TestRegisterSection(t *testing.T) {
	h := NewHub(nil, slog.Default(), 2)

	probes := []register.Probe{
		{Name: "P1", Addr: 0x0404, Count: 1},
	}
	h.RegisterSection("test-sec", probes)

	sec, ok := h.sections["test-sec"]
	if !ok {
		t.Fatal("section 'test-sec' not found in hub.sections")
	}
	if sec.Name != "test-sec" {
		t.Errorf("section Name = %q, want %q", sec.Name, "test-sec")
	}
	if len(sec.Probes) != 1 {
		t.Errorf("section probe count = %d, want 1", len(sec.Probes))
	}
	if sec.SubscriberCount() != 0 {
		t.Errorf("SubscriberCount() = %d, want 0", sec.SubscriberCount())
	}
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
		if err := json.Unmarshal(data, &received); err != nil {
			t.Fatalf("failed to unmarshal received message: %v", err)
		}
		if received.Type != "test" {
			t.Errorf("received Type = %q, want %q", received.Type, "test")
		}
		if received.Data["k"] != "v" {
			t.Errorf("received Data[k] = %q, want %q", received.Data["k"], "v")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for broadcast message")
	}

	// Test with nonexistent section -- should be a no-op, no panic
	h.broadcastToSection("nonexistent", msg)
}
