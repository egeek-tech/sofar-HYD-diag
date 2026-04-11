package hub_test

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"sofar-hyd-diag/internal/broker"
	"sofar-hyd-diag/internal/hub"
	"sofar-hyd-diag/internal/register"
)

func TestBrokerSatisfiesInterface(t *testing.T) {
	// Compile-time check that broker.Broker satisfies hub.BrokerInterface
	var _ hub.BrokerInterface = (*broker.Broker)(nil)
}

// writeCall records a WriteRegister invocation for test assertions.
type writeCall struct {
	Addr  uint16
	Value uint16
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
	// WriteRegister tracking
	writeCalls    []writeCall
	writeErr      error   // if set, WriteRegister returns this error
	writeErrCount int     // number of times to return writeErr (0=forever)
	writeErrQueue []error // per-call error queue: pops from front; if empty, falls back to writeErr
	// Per-call batch results: if set, each ReadBatch call pops from this queue
	batchResultQueue [][]broker.Result
	// SetDelayRuntime tracking
	lastDelay time.Duration
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

	// If per-call queue is set, pop the first entry
	if len(m.batchResultQueue) > 0 {
		results := m.batchResultQueue[0]
		m.batchResultQueue = m.batchResultQueue[1:]
		m.mu.Unlock()
		if delay > 0 {
			time.Sleep(delay)
		}
		return results
	}

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

func (m *mockBroker) WriteRegister(ctx context.Context, addr uint16, value uint16) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writeCalls = append(m.writeCalls, writeCall{Addr: addr, Value: value})
	// Per-call error queue takes priority
	if len(m.writeErrQueue) > 0 {
		err := m.writeErrQueue[0]
		m.writeErrQueue = m.writeErrQueue[1:]
		return err
	}
	if m.writeErr != nil {
		if m.writeErrCount <= 0 {
			// Return error forever
			return m.writeErr
		}
		m.writeErrCount--
		return m.writeErr
	}
	return nil
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

func (m *mockBroker) ReadRegisters(ctx context.Context, addr uint16, count uint16) ([]byte, error) {
	results := m.ReadBatch(ctx, []broker.ReadRequest{{Addr: addr, Count: count}})
	if len(results) > 0 {
		return results[0].Data, results[0].Err
	}
	return nil, fmt.Errorf("no mock result")
}

func (m *mockBroker) SetDelayRuntime(ctx context.Context, d time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastDelay = d
	return nil
}

func (m *mockBroker) getWriteCalls() []writeCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]writeCall, len(m.writeCalls))
	copy(cp, m.writeCalls)
	return cp
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

// makeMockResultsForSection builds mock batch results for a given section.
// Returns enough results for all probes in the groups plus fault registers if system.
func makeMockResultsForSection(groups []register.ProbeGroup, isFault bool) []broker.Result {
	total := 0
	for _, g := range groups {
		total += len(g.Probes)
	}
	if isFault {
		total += len(register.FaultRegisters) // 2 fault batch reads
	}
	results := make([]broker.Result, total)
	for i := range results {
		results[i] = broker.Result{Data: []byte{0x00, 0x00}, Err: nil}
	}
	return results
}

// uint16Bytes returns a 2-byte big-endian encoding of a uint16 value.
func uint16Bytes(v uint16) []byte {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, v)
	return b
}

// makeFaultBatchData creates multi-register fault batch data (count*2 bytes, all zeros).
func makeFaultBatchData(count uint16) []byte {
	return make([]byte, int(count)*2)
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
	msg := hub.NewSectionData("test", map[string]string{"test": "value"})
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

// === Section registry, timer management ===

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

	// Subscribe to "grid" section (grouped section without faults - simpler)
	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeSubscribe,
		Section: "grid",
	})

	// Should receive section_data from immediate read (D-20)
	msgs := collectClientMessages(t, send, 1, 500*time.Millisecond)
	if msgs[0].Type != hub.MsgTypeSectionData {
		t.Errorf("expected type %q, got %q", hub.MsgTypeSectionData, msgs[0].Type)
	}
	if msgs[0].Section != "grid" {
		t.Errorf("expected section 'grid', got %q", msgs[0].Section)
	}

	// Verify ReadBatch was called
	if got := mb.getBatchCallCount(); got < 1 {
		t.Errorf("expected at least 1 ReadBatch call, got %d", got)
	}

	// Verify groups are present (grouped section)
	if len(msgs[0].Groups) == 0 {
		t.Error("expected non-empty groups in section_data")
	}
}

func TestSingleSectionPerClient(t *testing.T) {
	mb := newMockBroker()
	h, c, send, cancel := setupConnectedHub(t, mb, 0)
	defer cancel()

	// Subscribe to "grid"
	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeSubscribe,
		Section: "grid",
	})
	time.Sleep(100 * time.Millisecond)
	drainClientMessages(send, 100*time.Millisecond)

	// Subscribe to unknown section -- should get error, but client should be
	// unsubscribed from "grid" first (D-18)
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
		Section: "grid",
	})

	// Wait for immediate read + at least 2 timer ticks
	time.Sleep(200 * time.Millisecond)

	// Collect all messages -- should have multiple section_data messages
	msgs := drainClientMessages(send, 300*time.Millisecond)

	dataCount := 0
	for _, m := range msgs {
		if m.Type == hub.MsgTypeSectionData && m.Section == "grid" {
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
		Section: "grid",
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
		Section: "grid",
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
		Section: "grid",
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
	// Configure mock to return errors for all reads (grid has 27 probes)
	mb.mu.Lock()
	errResults := make([]broker.Result, 27)
	for i := range errResults {
		errResults[i] = broker.Result{Data: nil, Err: fmt.Errorf("modbus timeout")}
	}
	mb.batchResults = errResults
	mb.mu.Unlock()

	h, c, send, cancel := setupConnectedHub(t, mb, 0)
	defer cancel()

	// Subscribe -- immediate read should fail
	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeSubscribe,
		Section: "grid",
	})

	// Should receive section_error message (D-09, RT-04)
	// May also get a section_data with empty groups, so collect up to 2
	msgs := drainClientMessages(send, 500*time.Millisecond)
	foundError := false
	for _, m := range msgs {
		if m.Type == hub.MsgTypeSectionErr {
			foundError = true
			if m.Section != "grid" {
				t.Errorf("expected section 'grid', got %q", m.Section)
			}
			if m.Error == "" {
				t.Error("expected non-empty error message")
			}
		}
	}
	if !foundError {
		t.Error("expected at least one section_error message")
	}
}

func TestManualRefresh(t *testing.T) {
	mb := newMockBroker()
	h, c, send, cancel := setupConnectedHub(t, mb, 0)
	defer cancel()

	// Subscribe first
	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeSubscribe,
		Section: "grid",
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
		Section: "grid",
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

func TestAutoRefreshToggleStopsTimer(t *testing.T) {
	mb := newMockBroker()
	h, c, send, cancel := setupConnectedHub(t, mb, 200*time.Millisecond)
	defer cancel()

	// Subscribe to grid
	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeSubscribe,
		Section: "grid",
	})
	time.Sleep(50 * time.Millisecond)
	drainClientMessages(send, 200*time.Millisecond)

	// Toggle auto-refresh OFF
	enabled := false
	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeAutoRefresh,
		Section: "grid",
		Enabled: &enabled,
	})
	time.Sleep(50 * time.Millisecond)
	drainClientMessages(send, 100*time.Millisecond)

	// Wait for what would be 2 timer ticks (400ms)
	time.Sleep(450 * time.Millisecond)

	// Should NOT have received any auto-refresh data
	msgs := drainClientMessages(send, 100*time.Millisecond)
	if len(msgs) > 0 {
		t.Errorf("expected no messages after disabling auto-refresh, got %d", len(msgs))
	}
}

func TestSubscribeWhileDisconnectedSendsError(t *testing.T) {
	mb := newMockBroker()
	h := hub.NewTestHub(mb)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)
	time.Sleep(20 * time.Millisecond)

	// Do NOT connect the broker -- leave disconnected

	send := make(chan []byte, 256)
	c := hub.NewTestClient(h, send)
	h.Register(c)
	time.Sleep(20 * time.Millisecond)
	drainClientMessages(send, 100*time.Millisecond)

	// Subscribe while disconnected
	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeSubscribe,
		Section: "battery",
	})

	msgs := collectClientMessages(t, send, 1, 500*time.Millisecond)
	if len(msgs) == 0 {
		t.Fatal("expected a message after subscribing while disconnected")
	}
	if msgs[0].Type != hub.MsgTypeSectionErr {
		t.Errorf("expected section_error, got %q", msgs[0].Type)
	}
	if msgs[0].Section != "battery" {
		t.Errorf("expected section 'battery', got %q", msgs[0].Section)
	}
}

// === Phase 03 Plan 02: Grouped section tests ===

func TestGroupedSectionRegistered(t *testing.T) {
	mb := newMockBroker()
	h := hub.NewTestHub(mb)
	ctx, cancel := context.WithCancel(context.Background())
	go h.Run(ctx)
	time.Sleep(30 * time.Millisecond)
	defer cancel()

	// Verify all 4 sections exist
	for _, name := range []string{"system", "grid", "eps", "pv"} {
		if !h.HasSection(name) {
			t.Errorf("expected section %q to exist", name)
		}
		groups := h.GetSectionGroups(name)
		if groups == nil {
			t.Errorf("expected section %q to have groups", name)
		}
	}
}

func TestStatusSectionRemoved(t *testing.T) {
	mb := newMockBroker()
	h := hub.NewTestHub(mb)
	ctx, cancel := context.WithCancel(context.Background())
	go h.Run(ctx)
	time.Sleep(30 * time.Millisecond)
	defer cancel()

	// Verify "status" section does NOT exist (D-24: demo retired)
	if h.HasSection("status") {
		t.Error("expected 'status' section to NOT exist (D-24: demo retired)")
	}
}

func TestSystemSectionGroupedData(t *testing.T) {
	mb := newMockBroker()
	// Build mock results: 19 probe results + 2 fault batch results = 21
	results := makeMockResultsForSection(register.SystemGroups, true)
	mb.mu.Lock()
	mb.batchResults = results
	mb.mu.Unlock()

	h, c, send, cancel := setupConnectedHub(t, mb, 0)
	defer cancel()

	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeSubscribe,
		Section: "system",
	})

	msgs := collectClientMessages(t, send, 1, 500*time.Millisecond)
	msg := msgs[0]

	if msg.Type != hub.MsgTypeSectionData {
		t.Fatalf("expected type %q, got %q", hub.MsgTypeSectionData, msg.Type)
	}
	if msg.Section != "system" {
		t.Fatalf("expected section 'system', got %q", msg.Section)
	}

	// Verify groups array has 5 groups
	if len(msg.Groups) != 5 {
		t.Fatalf("expected 5 groups, got %d", len(msg.Groups))
	}

	// Verify group names
	expectedNames := []string{"Identity", "Firmware", "Status", "Temperatures", "Protection"}
	for i, name := range expectedNames {
		if msg.Groups[i].Name != name {
			t.Errorf("group[%d] name = %q, want %q", i, msg.Groups[i].Name, name)
		}
	}
}

func TestSystemSectionFaults(t *testing.T) {
	mb := newMockBroker()
	// Build mock results with all-zero fault data -> empty faults array
	results := makeMockResultsForSection(register.SystemGroups, true)
	mb.mu.Lock()
	mb.batchResults = results
	mb.mu.Unlock()

	h, c, send, cancel := setupConnectedHub(t, mb, 0)
	defer cancel()

	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeSubscribe,
		Section: "system",
	})

	msgs := collectClientMessages(t, send, 1, 500*time.Millisecond)
	msg := msgs[0]

	if msg.Type != hub.MsgTypeSectionData {
		t.Fatalf("expected type %q, got %q", hub.MsgTypeSectionData, msg.Type)
	}

	// When all fault registers are zero, there should be no active faults.
	// The faults field may be nil (omitted by JSON omitempty) or an empty array --
	// both are acceptable when there are zero active faults.
	if len(msg.Faults) != 0 {
		t.Errorf("expected 0 faults (all zeros), got %d", len(msg.Faults))
	}

	// Verify that the system section CAN produce faults by checking with non-zero data.
	// This is tested more thoroughly in TestSystemSectionFaultsActive below.
}

func TestSystemSectionFaultsActive(t *testing.T) {
	mb := newMockBroker()
	results := makeMockResultsForSection(register.SystemGroups, true)

	// Set fault batch 1 result (index 19): first register 0x0405, bit 0 = "Grid over-voltage"
	// Fault batch 1 reads 18 registers starting at 0x0405. We need 18*2=36 bytes.
	batch1Data := make([]byte, 36)
	// Set register 0x0405 (offset 0) bit 0
	binary.BigEndian.PutUint16(batch1Data[0:2], 0x0001) // bit 0 set
	results[19] = broker.Result{Data: batch1Data, Err: nil}

	// Fault batch 2 (index 20): 12 registers, all zeros
	batch2Data := make([]byte, 24)
	results[20] = broker.Result{Data: batch2Data, Err: nil}

	mb.mu.Lock()
	mb.batchResults = results
	mb.mu.Unlock()

	h, c, send, cancel := setupConnectedHub(t, mb, 0)
	defer cancel()

	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeSubscribe,
		Section: "system",
	})

	msgs := collectClientMessages(t, send, 1, 500*time.Millisecond)
	msg := msgs[0]

	if msg.Type != hub.MsgTypeSectionData {
		t.Fatalf("expected type %q, got %q", hub.MsgTypeSectionData, msg.Type)
	}

	// Should have exactly 1 active fault
	if len(msg.Faults) != 1 {
		t.Fatalf("expected 1 active fault, got %d", len(msg.Faults))
	}
	if msg.Faults[0].Name != "Grid over-voltage" {
		t.Errorf("fault name = %q, want 'Grid over-voltage'", msg.Faults[0].Name)
	}
}

func TestSystemSectionTimeComposition(t *testing.T) {
	mb := newMockBroker()

	// Build mock results for system section
	// System probes in order (19 total):
	// 0: Inverter SN (Identity)
	// 1-5: Firmware (HW, Comm, Master, Slave, Safety)
	// 6: Running state (Status)
	// 7: Year, 8: Month, 9: Day, 10: Hour, 11: Min, 12: Sec (Status)
	// 13-16: Temperatures
	// 17-18: Protection
	// 19-20: Fault batch reads (2)
	results := makeMockResultsForSection(register.SystemGroups, true)

	// Set time register values: 2026-03-16 14:30:45
	results[7] = broker.Result{Data: uint16Bytes(26), Err: nil}  // Year (offset from 2000)
	results[8] = broker.Result{Data: uint16Bytes(3), Err: nil}   // Month
	results[9] = broker.Result{Data: uint16Bytes(16), Err: nil}  // Day
	results[10] = broker.Result{Data: uint16Bytes(14), Err: nil} // Hour
	results[11] = broker.Result{Data: uint16Bytes(30), Err: nil} // Min
	results[12] = broker.Result{Data: uint16Bytes(45), Err: nil} // Sec

	mb.mu.Lock()
	mb.batchResults = results
	mb.mu.Unlock()

	h, c, send, cancel := setupConnectedHub(t, mb, 0)
	defer cancel()

	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeSubscribe,
		Section: "system",
	})

	msgs := collectClientMessages(t, send, 1, 500*time.Millisecond)
	msg := msgs[0]

	// Find the "Status" group
	var statusGroup *hub.GroupData
	for i := range msg.Groups {
		if msg.Groups[i].Name == "Status" {
			statusGroup = &msg.Groups[i]
			break
		}
	}
	if statusGroup == nil {
		t.Fatal("Status group not found in system section")
	}

	// Verify composed "System time" key
	systemTime, ok := statusGroup.Items["System time"]
	if !ok {
		t.Fatal("expected 'System time' key in Status group items")
	}
	expected := "2026-03-16 14:30:45"
	if systemTime != expected {
		t.Errorf("System time = %q, want %q", systemTime, expected)
	}

	// Verify individual time registers are NOT present
	for _, name := range []string{"System time (Year)", "System time (Month)", "System time (Day)", "System time (Hour)", "System time (Min)", "System time (Sec)"} {
		if _, exists := statusGroup.Items[name]; exists {
			t.Errorf("individual time register %q should NOT be in Status group items", name)
		}
	}
}

func TestSystemSectionEnumLabel(t *testing.T) {
	mb := newMockBroker()
	results := makeMockResultsForSection(register.SystemGroups, true)

	// Set Running state register (index 6) to 0x0002 -> "Grid-connected"
	results[6] = broker.Result{Data: uint16Bytes(2), Err: nil}

	mb.mu.Lock()
	mb.batchResults = results
	mb.mu.Unlock()

	h, c, send, cancel := setupConnectedHub(t, mb, 0)
	defer cancel()

	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeSubscribe,
		Section: "system",
	})

	msgs := collectClientMessages(t, send, 1, 500*time.Millisecond)
	msg := msgs[0]

	// Find the "Status" group
	var statusGroup *hub.GroupData
	for i := range msg.Groups {
		if msg.Groups[i].Name == "Status" {
			statusGroup = &msg.Groups[i]
			break
		}
	}
	if statusGroup == nil {
		t.Fatal("Status group not found in system section")
	}

	// Verify "Running state" shows enum label
	runState, ok := statusGroup.Items["Running state"]
	if !ok {
		t.Fatal("expected 'Running state' key in Status group items")
	}
	if runState != "Grid-connected" {
		t.Errorf("Running state = %q, want 'Grid-connected'", runState)
	}
}

func TestGridSectionGroupedLayout(t *testing.T) {
	mb := newMockBroker()
	h, c, send, cancel := setupConnectedHub(t, mb, 0)
	defer cancel()

	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeSubscribe,
		Section: "grid",
	})

	msgs := collectClientMessages(t, send, 1, 500*time.Millisecond)
	msg := msgs[0]

	if msg.Type != hub.MsgTypeSectionData {
		t.Fatalf("expected type %q, got %q", hub.MsgTypeSectionData, msg.Type)
	}

	// Find "Phase R" group and check layout
	var phaseR *hub.GroupData
	for i := range msg.Groups {
		if msg.Groups[i].Name == "Phase R" {
			phaseR = &msg.Groups[i]
			break
		}
	}
	if phaseR == nil {
		t.Fatal("Phase R group not found in grid section")
	}
	if phaseR.Layout != "column" {
		t.Errorf("Phase R layout = %q, want 'column'", phaseR.Layout)
	}
}

func TestNonSystemNoFaults(t *testing.T) {
	mb := newMockBroker()
	h, c, send, cancel := setupConnectedHub(t, mb, 0)
	defer cancel()

	// Subscribe to "grid" (non-system section)
	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeSubscribe,
		Section: "grid",
	})

	msgs := collectClientMessages(t, send, 1, 500*time.Millisecond)
	msg := msgs[0]

	if msg.Type != hub.MsgTypeSectionData {
		t.Fatalf("expected type %q, got %q", hub.MsgTypeSectionData, msg.Type)
	}

	// Grid section should NOT have faults field
	// JSON omitempty means nil slice -> no faults key, empty slice -> faults: []
	// For non-system sections, faultEntries is never set, so it stays nil
	if msg.Faults != nil {
		t.Errorf("expected nil faults for grid section, got %d faults", len(msg.Faults))
	}
}

// === Task 2: Configure message tests ===

func TestConfigurePVChannels(t *testing.T) {
	mb := newMockBroker()
	h, c, send, cancel := setupConnectedHub(t, mb, 0)
	defer cancel()

	// Send configure message to change PV channels to 4
	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeConfigure,
		Section: "pv",
		Config:  &hub.ConfigPayload{Channels: 4},
	})
	time.Sleep(50 * time.Millisecond)

	// Subscribe to PV section
	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeSubscribe,
		Section: "pv",
	})

	msgs := collectClientMessages(t, send, 1, 500*time.Millisecond)
	msg := msgs[0]

	if msg.Type != hub.MsgTypeSectionData {
		t.Fatalf("expected type %q, got %q", hub.MsgTypeSectionData, msg.Type)
	}

	// Should have 5 groups: PV 1, PV 2, PV 3, PV 4, Total PV Power
	if len(msg.Groups) != 5 {
		t.Fatalf("expected 5 groups after configure(channels=4), got %d", len(msg.Groups))
	}

	expectedNames := []string{"PV 1", "PV 2", "PV 3", "PV 4", "Total PV Power"}
	for i, name := range expectedNames {
		if msg.Groups[i].Name != name {
			t.Errorf("group[%d] name = %q, want %q", i, msg.Groups[i].Name, name)
		}
	}
}

func TestConfigureClampRange(t *testing.T) {
	mb := newMockBroker()
	h := hub.NewTestHub(mb)
	ctx, cancel := context.WithCancel(context.Background())
	go h.Run(ctx)
	time.Sleep(20 * time.Millisecond)
	defer cancel()

	// Connect broker
	mb.mu.Lock()
	mb.state = broker.StateConnected
	mb.mu.Unlock()
	mb.statesCh <- broker.StateEvent{State: broker.StateConnected}
	time.Sleep(30 * time.Millisecond)

	send := make(chan []byte, 256)
	c := hub.NewTestClient(h, send)
	h.Register(c)
	time.Sleep(20 * time.Millisecond)
	drainClientMessages(send, 100*time.Millisecond)

	// Configure with channels=0 (should clamp to 2)
	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeConfigure,
		Section: "pv",
		Config:  &hub.ConfigPayload{Channels: 0},
	})
	time.Sleep(50 * time.Millisecond)

	groups := h.GetSectionGroups("pv")
	// Should have 3 groups: PV 1, PV 2, Total PV Power (clamped to 2 channels)
	if len(groups) != 3 {
		t.Errorf("expected 3 groups after clamp(0->2), got %d", len(groups))
	}

	// Configure with channels=20 (should clamp to 16)
	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeConfigure,
		Section: "pv",
		Config:  &hub.ConfigPayload{Channels: 20},
	})
	time.Sleep(50 * time.Millisecond)

	groups = h.GetSectionGroups("pv")
	// Should have 17 groups: PV 1-16, Total PV Power (clamped to 16 channels)
	if len(groups) != 17 {
		t.Errorf("expected 17 groups after clamp(20->16), got %d", len(groups))
	}
}

func TestConfigureNonPVIgnored(t *testing.T) {
	mb := newMockBroker()
	h := hub.NewTestHub(mb)
	ctx, cancel := context.WithCancel(context.Background())
	go h.Run(ctx)
	time.Sleep(20 * time.Millisecond)
	defer cancel()

	send := make(chan []byte, 256)
	c := hub.NewTestClient(h, send)
	h.Register(c)
	time.Sleep(20 * time.Millisecond)
	drainClientMessages(send, 100*time.Millisecond)

	// Configure for "grid" section -- should be silently ignored
	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeConfigure,
		Section: "grid",
		Config:  &hub.ConfigPayload{Channels: 4},
	})
	time.Sleep(50 * time.Millisecond)

	// Grid groups should be unchanged (7 groups)
	groups := h.GetSectionGroups("grid")
	if len(groups) != 7 {
		t.Errorf("expected 7 grid groups (unchanged), got %d", len(groups))
	}
}

func TestConfigureTriggersReread(t *testing.T) {
	mb := newMockBroker()
	h, c, send, cancel := setupConnectedHub(t, mb, 0)
	defer cancel()

	// Subscribe to PV first
	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeSubscribe,
		Section: "pv",
	})
	time.Sleep(100 * time.Millisecond)
	drainClientMessages(send, 200*time.Millisecond)

	// Reset batch count
	mb.mu.Lock()
	mb.batchCallCount = 0
	mb.mu.Unlock()

	// Configure PV channels while subscribed -- should trigger re-read
	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeConfigure,
		Section: "pv",
		Config:  &hub.ConfigPayload{Channels: 4},
	})

	// Should receive new section_data from the triggered re-read
	msgs := collectClientMessages(t, send, 1, 500*time.Millisecond)
	if msgs[0].Type != hub.MsgTypeSectionData {
		t.Errorf("expected type %q, got %q", hub.MsgTypeSectionData, msgs[0].Type)
	}

	// Verify a new ReadBatch was triggered
	if got := mb.getBatchCallCount(); got < 1 {
		t.Errorf("expected at least 1 ReadBatch call from configure re-read, got %d", got)
	}

	// Verify the data has 5 groups (4 PV + Total)
	if len(msgs[0].Groups) != 5 {
		t.Errorf("expected 5 groups after reconfigure, got %d", len(msgs[0].Groups))
	}
}

// === Phase 05 Plan 02: Pack selection and data retrieval tests ===

// makePackRTData builds a 120-byte (60 register) block of mock pack RT data.
// Sets cell voltages 1-24 to incrementing millivolt values starting at 3200mV,
// max cell at 0x9069 = 3223mV, min cell at 0x906A = 3200mV,
// temps at 285 (28.5C), alarm/protection/fault to zero, balance to 0.
func makePackRTData() []byte {
	data := make([]byte, 120) // 60 registers * 2 bytes
	// 0x9044 = Pack ID (offset 0) = 1
	binary.BigEndian.PutUint16(data[0:2], 1)
	// 0x9045-0x9046 = Timestamp Hi/Lo (offsets 2-6)
	// 0x9047-0x9050 = Serial Number (10 registers, offsets 6-26) - leave as zeros (ASCII)

	// Cell voltages 0x9051-0x9060 (offsets 26-58): 16 cells at 3200+i mV (D-05)
	for i := 0; i < 16; i++ {
		offset := (0x9051 - 0x9044 + uint16(i)) * 2
		binary.BigEndian.PutUint16(data[offset:offset+2], uint16(3200+i))
	}

	// 0x9069 = Max Cell Voltage (offset 74) = 3215 mV (cell 16)
	maxOffset := (0x9069 - 0x9044) * 2
	binary.BigEndian.PutUint16(data[maxOffset:maxOffset+2], 3215)

	// 0x906A = Min Cell Voltage (offset 76) = 3200 mV (cell 1)
	minOffset := (0x906A - 0x9044) * 2
	binary.BigEndian.PutUint16(data[minOffset:minOffset+2], 3200)

	// Temps 1-4: 0x906B-0x906E = 285 (28.5C as S16 * 0.1)
	for i := uint16(0); i < 4; i++ {
		tOffset := (0x906B - 0x9044 + i) * 2
		binary.BigEndian.PutUint16(data[tOffset:tOffset+2], 285)
	}

	// 0x906F = MOS Temp = 310 (31.0C)
	mosOffset := (0x906F - 0x9044) * 2
	binary.BigEndian.PutUint16(data[mosOffset:mosOffset+2], 310)

	// 0x9070 = Env Temp = 250 (25.0C)
	envOffset := (0x9070 - 0x9044) * 2
	binary.BigEndian.PutUint16(data[envOffset:envOffset+2], 250)

	// 0x9075 = Balance State = 0 (balanced)
	balOffset := (0x9075 - 0x9044) * 2
	binary.BigEndian.PutUint16(data[balOffset:balOffset+2], 0)

	// 0x9076 = Alarm Status = 0
	// 0x9077 = Protection Status = 0
	// 0x9078 = Fault Status = 0
	// Leave as zeros

	return data
}

// makePackInfoData builds a 70-byte (35 register) block of mock pack info data.
func makePackInfoData() []byte {
	data := make([]byte, 70) // 35 registers * 2 bytes
	// 0x9104 = Balanced Bus Voltage = 520 (52.0V)
	binary.BigEndian.PutUint16(data[0:2], 520)
	// 0x9124 = Alarm Status 2 (offset (0x9124-0x9104)*2 = 64)
	binary.BigEndian.PutUint16(data[64:66], 0)
	// 0x9125 = Protection Status 2 (offset 66)
	binary.BigEndian.PutUint16(data[66:68], 0)
	// 0x9126 = Fault Status 2 (offset 68)
	binary.BigEndian.PutUint16(data[68:70], 0)
	return data
}

// makePackTemps58Data builds an 8-byte (4 register) block of mock temps 5-8 data.
func makePackTemps58Data() []byte {
	data := make([]byte, 8) // 4 registers * 2 bytes
	for i := 0; i < 4; i++ {
		binary.BigEndian.PutUint16(data[i*2:(i+1)*2], 275) // 27.5C
	}
	return data
}

// collectPackDataMessages reads raw JSON from the send channel and attempts to unmarshal
// as PackDataMessage. Returns all successfully parsed pack_data messages.
func collectPackDataMessages(t *testing.T, send chan []byte, count int, timeout time.Duration) []hub.PackDataMessage {
	t.Helper()
	var msgs []hub.PackDataMessage
	deadline := time.After(timeout)
	for len(msgs) < count {
		select {
		case raw, ok := <-send:
			if !ok {
				t.Fatalf("send channel closed after %d pack messages, wanted %d", len(msgs), count)
			}
			var msg hub.PackDataMessage
			if err := json.Unmarshal(raw, &msg); err == nil && msg.Type == hub.MsgTypePackData {
				msgs = append(msgs, msg)
			}
		case <-deadline:
			t.Fatalf("timeout after %v: got %d pack_data messages, wanted %d", timeout, len(msgs), count)
		}
	}
	return msgs
}

// collectPackErrorMessages reads raw JSON from the send channel and attempts to unmarshal
// as PackErrorMessage. Returns all successfully parsed pack_error messages.
func collectPackErrorMessages(t *testing.T, send chan []byte, count int, timeout time.Duration) []hub.PackErrorMessage {
	t.Helper()
	var msgs []hub.PackErrorMessage
	deadline := time.After(timeout)
	for len(msgs) < count {
		select {
		case raw, ok := <-send:
			if !ok {
				t.Fatalf("send channel closed after %d pack error messages, wanted %d", len(msgs), count)
			}
			var msg hub.PackErrorMessage
			if err := json.Unmarshal(raw, &msg); err == nil && msg.Type == hub.MsgTypePackError {
				msgs = append(msgs, msg)
			}
		case <-deadline:
			t.Fatalf("timeout after %v: got %d pack_error messages, wanted %d", timeout, len(msgs), count)
		}
	}
	return msgs
}

func TestHandleSelectPack(t *testing.T) {
	mb := newMockBroker()

	// Set up per-call batch results: 3 ReadBatch calls (RT, Info, Temps58)
	mb.mu.Lock()
	mb.batchResultQueue = [][]broker.Result{
		{{Data: makePackRTData(), Err: nil}},     // RT block: 1 read request
		{{Data: makePackInfoData(), Err: nil}},    // Info block: 1 read request
		{{Data: makePackTemps58Data(), Err: nil}},  // Temps58 block: 1 read request
	}
	mb.mu.Unlock()

	h, c, send, cancel := setupConnectedHub(t, mb, 0)
	defer cancel()

	// Send select_pack message
	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeSelectPack,
		Section: "bms",
		Input:   1,
		Tower:   1,
		Pack:    1,
	})

	// Wait for async processing (write + 1s settle + 3 reads)
	time.Sleep(2 * time.Second)
	// Drain any messages from send channel
	drainClientMessages(send, 200*time.Millisecond)

	// Verify WriteRegister was called with addr 0x9020
	writes := mb.getWriteCalls()
	if len(writes) == 0 {
		t.Fatal("expected WriteRegister to be called, got 0 calls")
	}
	foundWrite := false
	for _, w := range writes {
		if w.Addr == 0x9020 {
			foundWrite = true
			break
		}
	}
	if !foundWrite {
		t.Errorf("expected WriteRegister call with addr=0x9020, got calls: %+v", writes)
	}

	// Verify ReadBatch was called 3 times (RT, Info, Temps58)
	if got := mb.getBatchCallCount(); got < 3 {
		t.Errorf("expected at least 3 ReadBatch calls, got %d", got)
	}
}

func TestPackDataMessageShape(t *testing.T) {
	mb := newMockBroker()

	// Set up per-call batch results
	mb.mu.Lock()
	mb.batchResultQueue = [][]broker.Result{
		{{Data: makePackRTData(), Err: nil}},
		{{Data: makePackInfoData(), Err: nil}},
		{{Data: makePackTemps58Data(), Err: nil}},
	}
	mb.mu.Unlock()

	h, c, send, cancel := setupConnectedHub(t, mb, 0)
	defer cancel()

	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeSelectPack,
		Section: "bms",
		Input:   1,
		Tower:   1,
		Pack:    1,
	})

	msgs := collectPackDataMessages(t, send, 1, 3*time.Second)
	msg := msgs[0]

	if msg.Type != hub.MsgTypePackData {
		t.Fatalf("expected type %q, got %q", hub.MsgTypePackData, msg.Type)
	}
	if msg.Section != "bms" {
		t.Errorf("expected section 'bms', got %q", msg.Section)
	}
	if msg.Input != 1 {
		t.Errorf("expected input=1, got %d", msg.Input)
	}
	if msg.Tower != 1 {
		t.Errorf("expected tower=1, got %d", msg.Tower)
	}
	if msg.Pack != 1 {
		t.Errorf("expected pack=1, got %d", msg.Pack)
	}

	// Should have 5 groups
	if len(msg.Groups) != 5 {
		t.Fatalf("expected 5 groups, got %d: %+v", len(msg.Groups), msg.Groups)
	}

	// Verify group names and types
	expectedGroups := []struct {
		name  string
		gtype string
	}{
		{"Pack Info", ""},
		{"Cell Voltages", "cell_grid"},
		{"Temperatures", ""},
		{"Pack Status", "pack_status"},
		{"Balance State", "balance"},
	}
	for i, eg := range expectedGroups {
		if msg.Groups[i].Name != eg.name {
			t.Errorf("group[%d] name = %q, want %q", i, msg.Groups[i].Name, eg.name)
		}
		if msg.Groups[i].Type != eg.gtype {
			t.Errorf("group[%d] type = %q, want %q", i, msg.Groups[i].Type, eg.gtype)
		}
	}

	// Verify cell grid has 16 cells with correct values (D-05)
	cellGroup := msg.Groups[1]
	if len(cellGroup.Cells) != 16 {
		t.Fatalf("expected 16 cells, got %d", len(cellGroup.Cells))
	}
	// Cell 1 should be 3200mV, Cell 16 should be 3215mV
	if cellGroup.Cells[0] != 3200 {
		t.Errorf("cell[0] = %d, want 3200", cellGroup.Cells[0])
	}
	if cellGroup.Cells[15] != 3215 {
		t.Errorf("cell[15] = %d, want 3215", cellGroup.Cells[15])
	}
	if cellGroup.MaxCell != 3215 {
		t.Errorf("MaxCell = %d, want 3215", cellGroup.MaxCell)
	}
	if cellGroup.MinCell != 3200 {
		t.Errorf("MinCell = %d, want 3200", cellGroup.MinCell)
	}

	// Verify temperatures group has TempRaw
	tempGroup := msg.Groups[2]
	if len(tempGroup.TempRaw) == 0 {
		t.Error("expected non-empty TempRaw in Temperatures group")
	}

	// Verify timestamp is set
	if msg.Timestamp == "" {
		t.Error("expected non-empty timestamp")
	}
}

func TestPackErrorOnWriteTimeout(t *testing.T) {
	mb := newMockBroker()

	// Configure WriteRegister to fail on all calls
	mb.mu.Lock()
	mb.writeErr = fmt.Errorf("modbus timeout")
	mb.writeErrCount = 0 // fail forever
	mb.mu.Unlock()

	h, c, send, cancel := setupConnectedHub(t, mb, 0)
	defer cancel()

	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeSelectPack,
		Section: "bms",
		Input:   1,
		Tower:   1,
		Pack:    1,
	})

	// Wait for write + retry + settle times
	msgs := collectPackErrorMessages(t, send, 1, 5*time.Second)
	msg := msgs[0]

	if msg.Type != hub.MsgTypePackError {
		t.Fatalf("expected type %q, got %q", hub.MsgTypePackError, msg.Type)
	}
	if msg.Section != "bms" {
		t.Errorf("expected section 'bms', got %q", msg.Section)
	}
	if msg.Input != 1 || msg.Tower != 1 || msg.Pack != 1 {
		t.Errorf("expected input=1,tower=1,pack=1, got %d,%d,%d", msg.Input, msg.Tower, msg.Pack)
	}
	if msg.Error == "" {
		t.Error("expected non-empty error message")
	}
}

func TestEncodePackQueryInHandler(t *testing.T) {
	mb := newMockBroker()

	// Set up per-call batch results
	mb.mu.Lock()
	mb.batchResultQueue = [][]broker.Result{
		{{Data: makePackRTData(), Err: nil}},
		{{Data: makePackInfoData(), Err: nil}},
		{{Data: makePackTemps58Data(), Err: nil}},
	}
	mb.mu.Unlock()

	// Create hub with 2 towers per input (default)
	h, c, send, cancel := setupConnectedHub(t, mb, 0)
	defer cancel()

	// select_pack with input=1, tower=2, pack=5
	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeSelectPack,
		Section: "bms",
		Input:   1,
		Tower:   2,
		Pack:    5,
	})

	// Wait for async processing
	time.Sleep(2 * time.Second)
	drainClientMessages(send, 500*time.Millisecond)

	// Verify WriteRegister was called with the correctly encoded value
	// EncodePackQuery(1, 2, 5, 2): group = (1-1)*2 + (2-1) = 1, packIdx = 5-1 = 4
	// value = 4 | (1 << 8) = 0x0104
	expectedValue := register.EncodePackQuery(1, 2, 5, 2)
	writes := mb.getWriteCalls()
	found := false
	for _, w := range writes {
		if w.Addr == 0x9020 && w.Value == expectedValue {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected WriteRegister(0x9020, 0x%04X), got calls: %+v", expectedValue, writes)
	}
}

func TestTopologyConstants(t *testing.T) {
	if hub.TopoTowers != 2 {
		t.Errorf("TopoTowers = %d, want 2", hub.TopoTowers)
	}
	if hub.TopoPacksPerTower != 10 {
		t.Errorf("TopoPacksPerTower = %d, want 10", hub.TopoPacksPerTower)
	}
	if hub.TopoCellsPerPack != 16 {
		t.Errorf("TopoCellsPerPack = %d, want 16", hub.TopoCellsPerPack)
	}
}

// makeBMSInfoResults builds mock batch results for the initial BMS info + protection ReadBatch.
// Returns 19 BMS info probe results + 6 protection probe results = 25 total.
// Probe index 6 (0x900D Topology Params) returns 0x020A (2 strings x 10 packs).
// Probe index 6 (0x900D) returns 0x020A (2 strings x 10 packs).
// Probe index 18 (0x9022) is the tower bitmap — bit N = tower N online.
func makeBMSInfoResults() []broker.Result {
	bmsInfoCount := 19 // BMSInfoGroups has 19 probes
	protCount := 6     // BMSProtectionProbes has 6 probes
	total := bmsInfoCount + protCount
	results := make([]broker.Result, total)
	for i := range results {
		results[i] = broker.Result{Data: []byte{0x00, 0x00}, Err: nil}
	}
	// Set topology at index 6 (0x900D): 0x020A = 2 strings, 10 packs
	results[6] = broker.Result{Data: uint16Bytes(0x020A), Err: nil}
	return results
}

// makeBMSInfoResultsWithBitmap builds mock results with a specific tower bitmap at 0x9022 (index 18).
func makeBMSInfoResultsWithBitmap(towerBitmap uint16) []broker.Result {
	results := makeBMSInfoResults()
	results[18] = broker.Result{Data: uint16Bytes(towerBitmap), Err: nil}
	return results
}

func TestBMSTowerBitmap(t *testing.T) {
	mb := newMockBroker()

	// 0x9022 tower bitmap: 0x0003 = bits 0 and 1 set = towers 1 and 2 online
	mb.mu.Lock()
	mb.batchResultQueue = [][]broker.Result{
		makeBMSInfoResultsWithBitmap(0x0003), // Both towers online
	}
	mb.mu.Unlock()

	h, c, send, cancel := setupConnectedHub(t, mb, 0)
	defer cancel()

	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeSubscribe,
		Section: "bms",
	})

	msgs := collectClientMessages(t, send, 1, 5*time.Second)
	msg := msgs[0]

	if msg.Type != hub.MsgTypeSectionData {
		t.Fatalf("expected type %q, got %q", hub.MsgTypeSectionData, msg.Type)
	}

	// Find the bitmap group
	var bitmapGroup *hub.GroupData
	for i := range msg.Groups {
		if msg.Groups[i].Type == "bitmap" {
			bitmapGroup = &msg.Groups[i]
			break
		}
	}
	if bitmapGroup == nil {
		t.Fatal("expected bitmap group in BMS section data, found none")
	}
	if bitmapGroup.Bitmap == nil {
		t.Fatal("bitmap group has nil Bitmap field")
	}

	// Verify bitmap structure
	if bitmapGroup.Bitmap.Towers != 2 {
		t.Errorf("Bitmap.Towers = %d, want 2", bitmapGroup.Bitmap.Towers)
	}
	if bitmapGroup.Bitmap.PacksPerTower != 10 {
		t.Errorf("Bitmap.PacksPerTower = %d, want 10", bitmapGroup.Bitmap.PacksPerTower)
	}

	// Both towers online: each should have all 10 packs marked available (0x03FF)
	if len(bitmapGroup.Bitmap.Online) != 2 {
		t.Fatalf("Bitmap.Online length = %d, want 2", len(bitmapGroup.Bitmap.Online))
	}
	if bitmapGroup.Bitmap.Online[0] != 0x03FF {
		t.Errorf("Bitmap.Online[0] = 0x%04X, want 0x03FF (tower 1 online, all packs available)", bitmapGroup.Bitmap.Online[0])
	}
	if bitmapGroup.Bitmap.Online[1] != 0x03FF {
		t.Errorf("Bitmap.Online[1] = 0x%04X, want 0x03FF (tower 2 online, all packs available)", bitmapGroup.Bitmap.Online[1])
	}

	// No WriteRegister calls to 0x9020 — bitmap is read from standard batch, no cycling
	writes := mb.getWriteCalls()
	for _, w := range writes {
		if w.Addr == 0x9020 {
			t.Errorf("unexpected WriteRegister to 0x9020 (bitmap cycling removed): value=0x%04X", w.Value)
		}
	}

	// Only 1 ReadBatch call (no separate bitmap reads)
	if got := mb.getBatchCallCount(); got != 1 {
		t.Errorf("expected 1 ReadBatch call, got %d", got)
	}
}

func TestBMSTowerBitmapPartialOnline(t *testing.T) {
	mb := newMockBroker()

	// 0x9022 tower bitmap: 0x0001 = only bit 0 set = only tower 1 online
	mb.mu.Lock()
	mb.batchResultQueue = [][]broker.Result{
		makeBMSInfoResultsWithBitmap(0x0001), // Only tower 1 online
	}
	mb.mu.Unlock()

	h, c, send, cancel := setupConnectedHub(t, mb, 0)
	defer cancel()

	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeSubscribe,
		Section: "bms",
	})

	msgs := collectClientMessages(t, send, 1, 5*time.Second)
	msg := msgs[0]

	if msg.Type != hub.MsgTypeSectionData {
		t.Fatalf("expected type %q, got %q", hub.MsgTypeSectionData, msg.Type)
	}

	var bitmapGroup *hub.GroupData
	for i := range msg.Groups {
		if msg.Groups[i].Type == "bitmap" {
			bitmapGroup = &msg.Groups[i]
			break
		}
	}
	if bitmapGroup == nil {
		t.Fatal("expected bitmap group in BMS section data, found none")
	}

	if len(bitmapGroup.Bitmap.Online) != 2 {
		t.Fatalf("Bitmap.Online length = %d, want 2", len(bitmapGroup.Bitmap.Online))
	}
	// Tower 1 online: all packs available
	if bitmapGroup.Bitmap.Online[0] != 0x03FF {
		t.Errorf("Bitmap.Online[0] = 0x%04X, want 0x03FF (tower 1 online)", bitmapGroup.Bitmap.Online[0])
	}
	// Tower 2 offline: no packs available
	if bitmapGroup.Bitmap.Online[1] != 0x0000 {
		t.Errorf("Bitmap.Online[1] = 0x%04X, want 0x0000 (tower 2 offline)", bitmapGroup.Bitmap.Online[1])
	}
}

// === Wave 0 Test Stubs (Phase 7) ===
// These stubs define the behavioral expectations for each phase requirement.
// They are intentionally t.Skip'd so they compile and appear in test output
// but do not fail. Plan 02 will remove Skip and implement the test bodies.

// TestStreamingRead verifies STREAM-01: subscribing to a section causes individual
// register_value messages to be sent per register, followed by a section_complete message.
func TestStreamingRead(t *testing.T) {
	t.Skip("Wave 0 stub: will be implemented when streaming read methods exist (Plan 02)")
	// Setup: connected hub, subscribe to "grid" section
	// Expect: multiple register_value messages (one per probe in grid section)
	// Expect: final section_complete message with timestamp
	// Verify: mockBroker.ReadRegisters called once per probe (not ReadBatch)
}

// TestSectionSchema verifies STREAM-02: a section_schema message is sent to the client
// on subscribe, before any register values.
func TestSectionSchema(t *testing.T) {
	t.Skip("Wave 0 stub: will be implemented when schema-on-subscribe exists (Plan 02)")
	// Setup: connected hub, subscribe to "grid" section
	// Expect: first message received is section_schema with groups matching grid section
	// Expect: each group has name, layout, and register name list
	// Verify: schema message arrives before any register_value messages
}

// TestTimingConfigure verifies TIMING-01: a configure message with timing section
// updates the broker's inter-read delay.
func TestTimingConfigure(t *testing.T) {
	t.Skip("Wave 0 stub: will be implemented when timing configure handler exists (Plan 02)")
	// Setup: connected hub
	// Send: configure message with section="timing", timing_config={read_delay_ms: 200}
	// Verify: mockBroker.lastDelay == 200ms (SetDelayRuntime was called)
	// Verify: values outside [100, 5000] are clamped
}

// TestPackSettleConfigure verifies TIMING-02: a configure message with timing section
// updates the hub's pack settle time.
func TestPackSettleConfigure(t *testing.T) {
	t.Skip("Wave 0 stub: will be implemented when timing configure handler exists (Plan 02)")
	// Setup: connected hub
	// Send: configure message with section="timing", timing_config={pack_settle_ms: 2000}
	// Verify: hub.GetTimingConfig() returns packSettleMs == 2000
	// Verify: values outside [500, 10000] are clamped
}
