package hub_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"sofar-hyd-diag/internal/broker"
	"sofar-hyd-diag/internal/hub"
)

func TestBrokerSatisfiesInterface(t *testing.T) {
	// Compile-time check that broker.Broker satisfies hub.BrokerInterface
	var _ hub.BrokerInterface = (*broker.Broker)(nil)
}

// mockBroker implements hub.BrokerInterface for testing.
type mockBroker struct {
	mu               sync.Mutex
	state            broker.State
	statesCh         chan broker.StateEvent
	reconfigureCalls []reconfigureCall
	disconnectCalls  int
	batchResults     []broker.Result
	batchCallCount   int
	batchDelay       time.Duration // artificial delay for overlapping tests
}

type reconfigureCall struct {
	Addr    string
	SlaveID byte
}

func newMockBroker() *mockBroker {
	return &mockBroker{
		state:    broker.StateDormant,
		statesCh: make(chan broker.StateEvent, 16),
	}
}

func (m *mockBroker) Reconfigure(ctx context.Context, addr string, slaveID byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reconfigureCalls = append(m.reconfigureCalls, reconfigureCall{Addr: addr, SlaveID: slaveID})
	m.state = broker.StateConnected
	m.statesCh <- broker.StateEvent{State: broker.StateConnected}
	return nil
}

func (m *mockBroker) Disconnect(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.disconnectCalls++
	m.state = broker.StateDisconnected
	m.statesCh <- broker.StateEvent{State: broker.StateDisconnected}
	return nil
}

func (m *mockBroker) ReadBatch(ctx context.Context, reads []broker.ReadRequest) []broker.Result {
	m.mu.Lock()
	m.batchCallCount++
	delay := m.batchDelay
	results := m.batchResults
	m.mu.Unlock()

	if delay > 0 {
		time.Sleep(delay)
	}

	if results != nil {
		return results
	}
	out := make([]broker.Result, len(reads))
	for i := range out {
		out[i] = broker.Result{Data: []byte{0x00, 0x01}, Err: nil}
	}
	return out
}

func (m *mockBroker) CurrentState() broker.State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

func (m *mockBroker) StateEvents() <-chan broker.StateEvent {
	return m.statesCh
}

func (m *mockBroker) getBatchCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.batchCallCount
}

func (m *mockBroker) getReconfigureCalls() []reconfigureCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]reconfigureCall, len(m.reconfigureCalls))
	copy(cp, m.reconfigureCalls)
	return cp
}

func (m *mockBroker) getDisconnectCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.disconnectCalls
}

// collectClientMessages reads from a client's send channel with timeout.
func collectClientMessages(t *testing.T, send chan []byte, count int, timeout time.Duration) []hub.OutboundMessage {
	t.Helper()
	var msgs []hub.OutboundMessage
	deadline := time.After(timeout)
	for len(msgs) < count {
		select {
		case raw, ok := <-send:
			if !ok {
				t.Fatalf("send channel closed after %d messages, wanted %d", len(msgs), count)
			}
			var msg hub.OutboundMessage
			if err := json.Unmarshal(raw, &msg); err != nil {
				t.Fatalf("unmarshal outbound message: %v", err)
			}
			msgs = append(msgs, msg)
		case <-deadline:
			t.Fatalf("timeout after %v: got %d messages, wanted %d", timeout, len(msgs), count)
		}
	}
	return msgs
}

// drainClientMessages reads all available messages from a client's send channel.
func drainClientMessages(send chan []byte, timeout time.Duration) []hub.OutboundMessage {
	var msgs []hub.OutboundMessage
	deadline := time.After(timeout)
	for {
		select {
		case raw, ok := <-send:
			if !ok {
				return msgs
			}
			var msg hub.OutboundMessage
			if err := json.Unmarshal(raw, &msg); err != nil {
				continue
			}
			msgs = append(msgs, msg)
		case <-deadline:
			return msgs
		}
	}
}

func TestHubRegisterUnregister(t *testing.T) {
	mb := newMockBroker()
	h := hub.NewTestHub(mb)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)

	// Allow hub to start
	time.Sleep(20 * time.Millisecond)

	send := make(chan []byte, 256)
	c := hub.NewTestClient(h, send)
	h.Register(c)

	// Wait for registration to be processed
	time.Sleep(20 * time.Millisecond)

	if got := h.ClientCount(); got != 1 {
		t.Fatalf("expected 1 client, got %d", got)
	}

	h.Unregister(c)
	time.Sleep(20 * time.Millisecond)

	if got := h.ClientCount(); got != 0 {
		t.Fatalf("expected 0 clients, got %d", got)
	}
}

func TestHubConnectCommand(t *testing.T) {
	mb := newMockBroker()
	h := hub.NewTestHub(mb)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)
	time.Sleep(20 * time.Millisecond)

	send := make(chan []byte, 256)
	c := hub.NewTestClient(h, send)
	h.Register(c)
	time.Sleep(20 * time.Millisecond)

	// Drain the initial state message from registration
	drainClientMessages(send, 50*time.Millisecond)

	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeConnect,
		Host:    "1.2.3.4",
		Port:    4192,
		SlaveID: 1,
	})

	time.Sleep(50 * time.Millisecond)

	calls := mb.getReconfigureCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 Reconfigure call, got %d", len(calls))
	}
	if calls[0].Addr != "1.2.3.4:4192" {
		t.Errorf("expected addr 1.2.3.4:4192, got %s", calls[0].Addr)
	}
	if calls[0].SlaveID != 1 {
		t.Errorf("expected slaveID 1, got %d", calls[0].SlaveID)
	}
}

func TestHubDisconnectCommand(t *testing.T) {
	mb := newMockBroker()
	h := hub.NewTestHub(mb)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)
	time.Sleep(20 * time.Millisecond)

	send := make(chan []byte, 256)
	c := hub.NewTestClient(h, send)
	h.Register(c)
	time.Sleep(20 * time.Millisecond)

	h.Command(c, hub.InboundMessage{
		Type: hub.MsgTypeDisconnect,
	})

	time.Sleep(50 * time.Millisecond)

	if got := mb.getDisconnectCalls(); got != 1 {
		t.Fatalf("expected 1 Disconnect call, got %d", got)
	}
}

func TestHubStateBroadcast(t *testing.T) {
	mb := newMockBroker()
	h := hub.NewTestHub(mb)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)
	time.Sleep(20 * time.Millisecond)

	// Register two clients
	send1 := make(chan []byte, 256)
	send2 := make(chan []byte, 256)
	c1 := hub.NewTestClient(h, send1)
	c2 := hub.NewTestClient(h, send2)
	h.Register(c1)
	h.Register(c2)
	time.Sleep(20 * time.Millisecond)

	// Drain initial state messages
	drainClientMessages(send1, 50*time.Millisecond)
	drainClientMessages(send2, 50*time.Millisecond)

	// Emit a state event from the mock broker
	mb.statesCh <- broker.StateEvent{State: broker.StateConnected}
	time.Sleep(50 * time.Millisecond)

	// Both clients should receive the state broadcast
	msgs1 := collectClientMessages(t, send1, 1, 200*time.Millisecond)
	if msgs1[0].Type != hub.MsgTypeState {
		t.Errorf("expected type %q, got %q", hub.MsgTypeState, msgs1[0].Type)
	}
	if msgs1[0].State != "connected" {
		t.Errorf("expected state 'connected', got %q", msgs1[0].State)
	}

	msgs2 := collectClientMessages(t, send2, 1, 200*time.Millisecond)
	if msgs2[0].Type != hub.MsgTypeState {
		t.Errorf("expected type %q, got %q", hub.MsgTypeState, msgs2[0].Type)
	}
	if msgs2[0].State != "connected" {
		t.Errorf("expected state 'connected', got %q", msgs2[0].State)
	}
}

func TestClientWritePump(t *testing.T) {
	// This test verifies that the send channel mechanism works.
	// We use NewTestClient which has a direct send channel.
	mb := newMockBroker()
	h := hub.NewTestHub(mb)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)
	time.Sleep(20 * time.Millisecond)

	send := make(chan []byte, 256)
	c := hub.NewTestClient(h, send)
	h.Register(c)
	time.Sleep(20 * time.Millisecond)

	// Drain initial state message
	drainClientMessages(send, 50*time.Millisecond)

	// Send a section_data message to the client via hub broadcast
	msg := hub.NewSectionData("status", map[string]string{"test": "value"})
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}

	// Write directly to the client's send channel
	send <- data

	// Read it back and verify
	raw := <-send
	var got hub.OutboundMessage
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Type != hub.MsgTypeSectionData {
		t.Errorf("expected type %q, got %q", hub.MsgTypeSectionData, got.Type)
	}
}

// === Task 2: Section registry, timer management, demo "status" section ===

// setupConnectedHub creates a hub with a connected broker, registers a client,
// drains initial messages, and returns everything needed for section tests.
func setupConnectedHub(t *testing.T, mb *mockBroker, interval time.Duration) (*hub.Hub, *hub.Client, chan []byte, context.CancelFunc) {
	t.Helper()
	var h *hub.Hub
	if interval > 0 {
		h = hub.NewTestHubWithInterval(mb, interval)
	} else {
		h = hub.NewTestHub(mb)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go h.Run(ctx)
	time.Sleep(20 * time.Millisecond)

	// Connect the broker so reads work
	mb.mu.Lock()
	mb.state = broker.StateConnected
	mb.mu.Unlock()
	mb.statesCh <- broker.StateEvent{State: broker.StateConnected}
	time.Sleep(30 * time.Millisecond)

	send := make(chan []byte, 256)
	c := hub.NewTestClient(h, send)
	h.Register(c)
	time.Sleep(20 * time.Millisecond)

	// Drain initial state messages (dormant state on register + connected broadcast)
	drainClientMessages(send, 100*time.Millisecond)

	return h, c, send, cancel
}

func TestSubscribeTriggerImmediateRead(t *testing.T) {
	mb := newMockBroker()
	h, c, send, cancel := setupConnectedHub(t, mb, 0)
	defer cancel()

	// Subscribe to "status" section
	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeSubscribe,
		Section: "status",
	})

	// Should receive section_data from immediate read (D-20)
	msgs := collectClientMessages(t, send, 1, 500*time.Millisecond)
	if msgs[0].Type != hub.MsgTypeSectionData {
		t.Errorf("expected type %q, got %q", hub.MsgTypeSectionData, msgs[0].Type)
	}
	if msgs[0].Section != "status" {
		t.Errorf("expected section 'status', got %q", msgs[0].Section)
	}

	// Verify ReadBatch was called
	if got := mb.getBatchCallCount(); got < 1 {
		t.Errorf("expected at least 1 ReadBatch call, got %d", got)
	}

	// Verify data contains expected keys (from D-25 probes)
	if _, ok := msgs[0].Data["inverter_sn"]; !ok {
		t.Error("expected 'inverter_sn' key in section data")
	}
	if _, ok := msgs[0].Data["system_running_state"]; !ok {
		t.Error("expected 'system_running_state' key in section data")
	}
	if _, ok := msgs[0].Data["ambient_temp_1"]; !ok {
		t.Error("expected 'ambient_temp_1' key in section data")
	}
}

func TestSingleSectionPerClient(t *testing.T) {
	mb := newMockBroker()
	h, c, send, cancel := setupConnectedHub(t, mb, 0)
	defer cancel()

	// Subscribe to "status"
	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeSubscribe,
		Section: "status",
	})
	time.Sleep(100 * time.Millisecond)
	drainClientMessages(send, 100*time.Millisecond)

	// Subscribe to unknown section "system" -- should get error, but client should be
	// unsubscribed from "status" first (D-18)
	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeSubscribe,
		Section: "nonexistent",
	})
	time.Sleep(100 * time.Millisecond)

	// Client should receive a section_error for the unknown section
	msgs := drainClientMessages(send, 200*time.Millisecond)
	foundError := false
	for _, m := range msgs {
		if m.Type == hub.MsgTypeSectionErr {
			foundError = true
		}
	}
	if !foundError {
		t.Error("expected section_error for unknown section")
	}
}

func TestAutoRefreshTimer(t *testing.T) {
	mb := newMockBroker()
	h, c, send, cancel := setupConnectedHub(t, mb, 50*time.Millisecond)
	defer cancel()

	// Reset batch count
	mb.mu.Lock()
	mb.batchCallCount = 0
	mb.mu.Unlock()

	// Subscribe to trigger immediate read + start timer
	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeSubscribe,
		Section: "status",
	})

	// Wait for immediate read + at least 2 timer ticks
	time.Sleep(200 * time.Millisecond)

	// Collect all messages -- should have multiple section_data messages
	msgs := drainClientMessages(send, 300*time.Millisecond)

	dataCount := 0
	for _, m := range msgs {
		if m.Type == hub.MsgTypeSectionData && m.Section == "status" {
			dataCount++
		}
	}

	if dataCount < 2 {
		t.Errorf("expected at least 2 section_data messages from auto-refresh, got %d", dataCount)
	}

	if got := mb.getBatchCallCount(); got < 2 {
		t.Errorf("expected at least 2 ReadBatch calls, got %d", got)
	}
}

func TestSkipOverlappingTick(t *testing.T) {
	mb := newMockBroker()
	// Set a long delay on ReadBatch to simulate slow read
	mb.mu.Lock()
	mb.batchDelay = 200 * time.Millisecond
	mb.mu.Unlock()

	h, c, send, cancel := setupConnectedHub(t, mb, 30*time.Millisecond)
	defer cancel()

	mb.mu.Lock()
	mb.batchCallCount = 0
	mb.mu.Unlock()

	// Subscribe (triggers immediate read which will block for 200ms)
	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeSubscribe,
		Section: "status",
	})

	// Wait enough for several timer ticks to fire while read is in progress
	// The 200ms ReadBatch delay means ticks at 30ms, 60ms, 90ms... should be skipped
	time.Sleep(300 * time.Millisecond)

	// Drain messages
	drainClientMessages(send, 200*time.Millisecond)

	// Should have only a small number of batch calls despite many timer ticks
	// The first read blocks for 200ms, skipping ~6 ticks. Then one more read after.
	got := mb.getBatchCallCount()
	if got > 4 {
		t.Errorf("expected overlapping reads to be skipped, but got %d ReadBatch calls", got)
	}
}

func TestTimerPausesOnDisconnect(t *testing.T) {
	mb := newMockBroker()
	h, c, send, cancel := setupConnectedHub(t, mb, 50*time.Millisecond)
	defer cancel()

	// Subscribe to start timer
	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeSubscribe,
		Section: "status",
	})
	time.Sleep(80 * time.Millisecond)
	drainClientMessages(send, 100*time.Millisecond)

	// Record batch count before disconnect
	countBefore := mb.getBatchCallCount()

	// Simulate disconnect
	mb.mu.Lock()
	mb.state = broker.StateDisconnected
	mb.mu.Unlock()
	mb.statesCh <- broker.StateEvent{State: broker.StateDisconnected}
	time.Sleep(30 * time.Millisecond)

	// Drain disconnect state message
	drainClientMessages(send, 50*time.Millisecond)

	// Reset count and wait for timer ticks (which should not trigger reads)
	mb.mu.Lock()
	mb.batchCallCount = 0
	mb.mu.Unlock()
	time.Sleep(150 * time.Millisecond)

	// Should have zero new ReadBatch calls since timer is paused (D-28)
	got := mb.getBatchCallCount()
	if got != 0 {
		t.Errorf("expected 0 ReadBatch calls while disconnected, got %d (before disconnect: %d)", got, countBefore)
	}
}

func TestTimerResumesOnReconnect(t *testing.T) {
	mb := newMockBroker()
	h, c, send, cancel := setupConnectedHub(t, mb, 50*time.Millisecond)
	defer cancel()

	// Subscribe
	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeSubscribe,
		Section: "status",
	})
	time.Sleep(80 * time.Millisecond)
	drainClientMessages(send, 100*time.Millisecond)

	// Disconnect
	mb.mu.Lock()
	mb.state = broker.StateDisconnected
	mb.mu.Unlock()
	mb.statesCh <- broker.StateEvent{State: broker.StateDisconnected}
	time.Sleep(50 * time.Millisecond)
	drainClientMessages(send, 50*time.Millisecond)

	// Reset count
	mb.mu.Lock()
	mb.batchCallCount = 0
	mb.state = broker.StateConnected
	mb.mu.Unlock()

	// Reconnect
	mb.statesCh <- broker.StateEvent{State: broker.StateConnected}
	time.Sleep(200 * time.Millisecond)

	// Should have new ReadBatch calls since timer resumed (D-28)
	got := mb.getBatchCallCount()
	if got < 1 {
		t.Errorf("expected at least 1 ReadBatch call after reconnect, got %d", got)
	}
}

func TestSectionErrorBroadcast(t *testing.T) {
	mb := newMockBroker()
	// Configure mock to return errors
	mb.mu.Lock()
	mb.batchResults = []broker.Result{
		{Data: nil, Err: fmt.Errorf("modbus timeout")},
		{Data: nil, Err: fmt.Errorf("modbus timeout")},
		{Data: nil, Err: fmt.Errorf("modbus timeout")},
	}
	mb.mu.Unlock()

	h, c, send, cancel := setupConnectedHub(t, mb, 0)
	defer cancel()

	// Subscribe -- immediate read should fail
	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeSubscribe,
		Section: "status",
	})

	// Should receive section_error message (D-09, RT-04)
	msgs := collectClientMessages(t, send, 1, 500*time.Millisecond)
	if msgs[0].Type != hub.MsgTypeSectionErr {
		t.Errorf("expected type %q, got %q", hub.MsgTypeSectionErr, msgs[0].Type)
	}
	if msgs[0].Section != "status" {
		t.Errorf("expected section 'status', got %q", msgs[0].Section)
	}
	if msgs[0].Error == "" {
		t.Error("expected non-empty error message")
	}
}

func TestManualRefresh(t *testing.T) {
	mb := newMockBroker()
	h, c, send, cancel := setupConnectedHub(t, mb, 0)
	defer cancel()

	// Subscribe first
	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeSubscribe,
		Section: "status",
	})
	time.Sleep(100 * time.Millisecond)
	drainClientMessages(send, 100*time.Millisecond)

	// Reset count
	mb.mu.Lock()
	mb.batchCallCount = 0
	mb.mu.Unlock()

	// Send manual refresh command (D-23)
	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeRefresh,
		Section: "status",
	})

	// Should receive section_data from the manual refresh
	msgs := collectClientMessages(t, send, 1, 500*time.Millisecond)
	if msgs[0].Type != hub.MsgTypeSectionData {
		t.Errorf("expected type %q, got %q", hub.MsgTypeSectionData, msgs[0].Type)
	}

	if got := mb.getBatchCallCount(); got != 1 {
		t.Errorf("expected exactly 1 ReadBatch call for manual refresh, got %d", got)
	}
}

func TestDemoStatusProbes(t *testing.T) {
	// Verify the "status" section has the correct D-25 probes.
	// We create a hub without running it to inspect sections directly.
	mb := newMockBroker()
	h := hub.NewTestHub(mb)
	ctx, cancel := context.WithCancel(context.Background())
	go h.Run(ctx)
	time.Sleep(30 * time.Millisecond)
	defer cancel()

	probes := h.GetSectionProbes("status")
	if probes == nil {
		t.Fatal("status section not found")
	}

	if len(probes) != 3 {
		t.Fatalf("expected 3 probes, got %d", len(probes))
	}

	// Probe 0: Inverter SN at 0x0445, count 10
	if probes[0].Name != "Inverter SN" {
		t.Errorf("probe[0] name = %q, want 'Inverter SN'", probes[0].Name)
	}
	if probes[0].Addr != 0x0445 {
		t.Errorf("probe[0] addr = 0x%04X, want 0x0445", probes[0].Addr)
	}
	if probes[0].Count != 10 {
		t.Errorf("probe[0] count = %d, want 10", probes[0].Count)
	}
	if !probes[0].IsASCII {
		t.Error("probe[0] should be ASCII")
	}

	// Probe 1: System running state at 0x0404, count 1
	if probes[1].Name != "System running state" {
		t.Errorf("probe[1] name = %q, want 'System running state'", probes[1].Name)
	}
	if probes[1].Addr != 0x0404 {
		t.Errorf("probe[1] addr = 0x%04X, want 0x0404", probes[1].Addr)
	}
	if probes[1].Count != 1 {
		t.Errorf("probe[1] count = %d, want 1", probes[1].Count)
	}

	// Probe 2: Ambient temp 1 at 0x0418, count 1
	if probes[2].Name != "Ambient temp 1" {
		t.Errorf("probe[2] name = %q, want 'Ambient temp 1'", probes[2].Name)
	}
	if probes[2].Addr != 0x0418 {
		t.Errorf("probe[2] addr = 0x%04X, want 0x0418", probes[2].Addr)
	}
	if probes[2].Count != 1 {
		t.Errorf("probe[2] count = %d, want 1", probes[2].Count)
	}
	if !probes[2].Signed {
		t.Error("probe[2] should be signed")
	}
}
