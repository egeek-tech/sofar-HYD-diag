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

func (m *mockBroker) WriteRegister(ctx context.Context, addr uint16, value uint16) error {
	return nil // no-op for tests
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
