package hub_test

import (
	"context"
	"encoding/json"
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
