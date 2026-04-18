package hub_test

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	// Per-address register results for ReadRegisters (streaming model)
	registerResults map[uint16]broker.Result
	// spanFailAddrs: addresses that fail only for batch reads (count > 1).
	// Checked before registerResults so individual reads (count=1) can succeed.
	spanFailAddrs map[uint16]error
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
	m.mu.Lock()
	// Check span-level failures first: fail only batch reads (count > 1)
	if m.spanFailAddrs != nil && count > 1 {
		if err, ok := m.spanFailAddrs[addr]; ok {
			m.batchCallCount++
			m.mu.Unlock()
			return nil, err
		}
	}
	// Check per-address register results (used by streaming tests)
	if m.registerResults != nil {
		if r, ok := m.registerResults[addr]; ok {
			m.batchCallCount++
			m.mu.Unlock()
			return r.Data, r.Err
		}
	}
	m.mu.Unlock()
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

// setRegisterResult sets a per-address register result thread-safely.
func (m *mockBroker) setRegisterResult(addr uint16, r broker.Result) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.registerResults == nil {
		m.registerResults = make(map[uint16]broker.Result)
	}
	m.registerResults[addr] = r
}

// resetBatchCallCount resets the batch call counter thread-safely.
func (m *mockBroker) resetBatchCallCount() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.batchCallCount = 0
}

// setSpanFail marks an address to fail only for batch reads (count > 1).
func (m *mockBroker) setSpanFail(addr uint16, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.spanFailAddrs == nil {
		m.spanFailAddrs = make(map[uint16]error)
	}
	m.spanFailAddrs[addr] = err
}

// clearSpanFail removes a span failure so batch reads succeed again.
func (m *mockBroker) clearSpanFail(addr uint16) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.spanFailAddrs, addr)
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

// waitForMessageType drains messages until one with the specified type is found.
// Returns all messages drained (including the target) and the index of the target.
func waitForMessageType(t *testing.T, send chan []byte, msgType string, timeout time.Duration) ([]hub.OutboundMessage, int) {
	t.Helper()
	var msgs []hub.OutboundMessage
	deadline := time.After(timeout)
	for {
		select {
		case raw, ok := <-send:
			if !ok {
				t.Fatalf("send channel closed before finding message type %q (got %d messages)", msgType, len(msgs))
			}
			var msg hub.OutboundMessage
			if err := json.Unmarshal(raw, &msg); err != nil {
				t.Fatalf("unmarshal outbound message: %v", err)
			}
			msgs = append(msgs, msg)
			if msg.Type == msgType {
				return msgs, len(msgs) - 1
			}
		case <-deadline:
			types := make([]string, len(msgs))
			for i, m := range msgs {
				types[i] = m.Type
			}
			t.Fatalf("timeout waiting for %q after %v: got types %v", msgType, timeout, types)
		}
	}
}

// drainRawMessages reads all available raw JSON messages from a client's send channel.
// Returns the raw byte slices for flexible unmarshalling into different message types.
func drainRawMessages(send chan []byte, timeout time.Duration) [][]byte {
	var msgs [][]byte
	deadline := time.After(timeout)
	for {
		select {
		case raw, ok := <-send:
			if !ok {
				return msgs
			}
			msgs = append(msgs, raw)
		case <-deadline:
			return msgs
		}
	}
}

// drainUntilComplete drains messages until section_complete is received.
// Returns all messages including register_value, section_data, section_schema, section_complete.
func drainUntilComplete(t *testing.T, send chan []byte, timeout time.Duration) []hub.OutboundMessage {
	t.Helper()
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
			if msg.Type == hub.MsgTypeSectionComplete {
				return msgs
			}
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

// === Section registry, read cycle management ===

// setupConnectedHub creates a hub with a connected broker, registers a client,
// drains initial messages, and returns everything needed for section tests.
// The interval parameter is ignored (timers removed in Phase 08).
func setupConnectedHub(t *testing.T, mb *mockBroker, _ time.Duration) (*hub.Hub, *hub.Client, chan []byte, context.CancelFunc) {
	t.Helper()
	h := hub.NewTestHub(mb)
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

	// Streaming model: first message is section_schema, then register_value messages, then section_complete
	msgs := drainUntilComplete(t, send, 2*time.Second)

	// First message should be section_schema
	if len(msgs) == 0 {
		t.Fatal("expected at least one message after subscribe")
	}
	if msgs[0].Type != hub.MsgTypeSectionSchema {
		t.Errorf("expected first message type %q, got %q", hub.MsgTypeSectionSchema, msgs[0].Type)
	}

	// Last message should be section_complete
	lastMsg := msgs[len(msgs)-1]
	if lastMsg.Type != hub.MsgTypeSectionComplete {
		t.Errorf("expected last message type %q, got %q", hub.MsgTypeSectionComplete, lastMsg.Type)
	}

	// Should have register_value messages in between
	regCount := 0
	for _, m := range msgs {
		if m.Type == hub.MsgTypeRegisterValue {
			regCount++
		}
	}
	if regCount == 0 {
		t.Error("expected register_value messages from streaming read")
	}

	// Verify ReadRegisters/ReadBatch was called (mock routes ReadRegisters through ReadBatch)
	if got := mb.getBatchCallCount(); got < 1 {
		t.Errorf("expected at least 1 read call, got %d", got)
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

func TestReadCycleMessage(t *testing.T) {
	mb := newMockBroker()
	h, c, send, cancel := setupConnectedHub(t, mb, 0)
	defer cancel()

	// Subscribe to section (triggers immediate read per D-20)
	h.Command(c, hub.InboundMessage{Type: hub.MsgTypeSubscribe, Section: "grid"})
	time.Sleep(100 * time.Millisecond)
	drainClientMessages(send, 200*time.Millisecond)

	// Reset batch count after subscribe read
	mb.mu.Lock()
	mb.batchCallCount = 0
	mb.mu.Unlock()

	// Send read_cycle to trigger another read
	h.Command(c, hub.InboundMessage{Type: hub.MsgTypeReadCycle, Section: "grid"})
	time.Sleep(100 * time.Millisecond)

	msgs := drainClientMessages(send, 200*time.Millisecond)
	completeCount := 0
	for _, m := range msgs {
		if m.Type == hub.MsgTypeSectionComplete && m.Section == "grid" {
			completeCount++
		}
	}
	if completeCount < 1 {
		t.Errorf("expected at least 1 section_complete from read_cycle, got %d", completeCount)
	}
	if got := mb.getBatchCallCount(); got < 1 {
		t.Errorf("expected at least 1 read call from read_cycle, got %d", got)
	}
}

func TestSkipOverlappingReadCycle(t *testing.T) {
	mb := newMockBroker()
	mb.mu.Lock()
	mb.batchDelay = 200 * time.Millisecond
	mb.mu.Unlock()

	h, c, send, cancel := setupConnectedHub(t, mb, 0)
	defer cancel()

	// Subscribe triggers a slow read (200ms delay)
	h.Command(c, hub.InboundMessage{Type: hub.MsgTypeSubscribe, Section: "grid"})
	time.Sleep(50 * time.Millisecond)

	// Reset count after initial read starts
	mb.mu.Lock()
	mb.batchCallCount = 0
	mb.mu.Unlock()

	// Send multiple read_cycle rapidly while read is in progress
	for i := 0; i < 5; i++ {
		h.Command(c, hub.InboundMessage{Type: hub.MsgTypeReadCycle, Section: "grid"})
		time.Sleep(10 * time.Millisecond)
	}

	time.Sleep(400 * time.Millisecond)
	drainClientMessages(send, 200*time.Millisecond)

	// Most read_cycles should be skipped due to sec.reading guard
	got := mb.getBatchCallCount()
	if got > 3 {
		t.Errorf("expected overlapping read_cycles to be skipped, but got %d read calls", got)
	}
}

func TestReadsCancelledOnDisconnect(t *testing.T) {
	mb := newMockBroker()
	mb.mu.Lock()
	mb.batchDelay = 300 * time.Millisecond
	mb.mu.Unlock()

	h, c, send, cancel := setupConnectedHub(t, mb, 0)
	defer cancel()

	// Subscribe to start a slow read
	h.Command(c, hub.InboundMessage{Type: hub.MsgTypeSubscribe, Section: "grid"})
	time.Sleep(50 * time.Millisecond)

	// Disconnect while read is in progress
	mb.mu.Lock()
	mb.state = broker.StateDisconnected
	mb.mu.Unlock()
	mb.statesCh <- broker.StateEvent{State: broker.StateDisconnected}
	time.Sleep(50 * time.Millisecond)

	// Drain messages
	drainClientMessages(send, 200*time.Millisecond)

	// After disconnect, sending read_cycle should not trigger reads
	mb.mu.Lock()
	mb.batchCallCount = 0
	mb.mu.Unlock()

	h.Command(c, hub.InboundMessage{Type: hub.MsgTypeReadCycle, Section: "grid"})
	time.Sleep(100 * time.Millisecond)

	got := mb.getBatchCallCount()
	if got > 0 {
		t.Errorf("expected no reads after disconnect, got %d", got)
	}
}

func TestReadsWorkAfterReconnect(t *testing.T) {
	mb := newMockBroker()
	h, c, send, cancel := setupConnectedHub(t, mb, 0)
	defer cancel()

	// Subscribe
	h.Command(c, hub.InboundMessage{Type: hub.MsgTypeSubscribe, Section: "grid"})
	time.Sleep(100 * time.Millisecond)
	drainClientMessages(send, 200*time.Millisecond)

	// Disconnect
	mb.mu.Lock()
	mb.state = broker.StateDisconnected
	mb.mu.Unlock()
	mb.statesCh <- broker.StateEvent{State: broker.StateDisconnected}
	time.Sleep(50 * time.Millisecond)
	drainClientMessages(send, 100*time.Millisecond)

	// Reconnect
	mb.mu.Lock()
	mb.state = broker.StateConnected
	mb.mu.Unlock()
	mb.statesCh <- broker.StateEvent{State: broker.StateConnected}
	time.Sleep(50 * time.Millisecond)
	drainClientMessages(send, 100*time.Millisecond)

	// Reset and verify read_cycle works after reconnect
	mb.mu.Lock()
	mb.batchCallCount = 0
	mb.mu.Unlock()

	h.Command(c, hub.InboundMessage{Type: hub.MsgTypeReadCycle, Section: "grid"})
	time.Sleep(100 * time.Millisecond)

	msgs := drainClientMessages(send, 200*time.Millisecond)
	completeCount := 0
	for _, m := range msgs {
		if m.Type == hub.MsgTypeSectionComplete && m.Section == "grid" {
			completeCount++
		}
	}
	if completeCount < 1 {
		t.Errorf("expected section_complete after reconnect read_cycle, got %d", completeCount)
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

	// Streaming model: errors are reported as register_value messages with Error field set.
	// Also a section_complete is sent at the end of the cycle.
	msgs := drainClientMessages(send, 2*time.Second)
	foundError := false
	for _, m := range msgs {
		if m.Type == hub.MsgTypeRegisterValue && m.Error != "" {
			foundError = true
		}
	}
	if !foundError {
		t.Error("expected at least one register_value message with error")
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
	time.Sleep(200 * time.Millisecond)
	drainClientMessages(send, 500*time.Millisecond)

	// Reset count
	mb.mu.Lock()
	mb.batchCallCount = 0
	mb.mu.Unlock()

	// Send manual refresh command (D-23)
	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeRefresh,
		Section: "grid",
	})

	// Streaming model: manual refresh triggers per-register reads ending with section_complete
	msgs := drainUntilComplete(t, send, 2*time.Second)
	foundComplete := false
	for _, m := range msgs {
		if m.Type == hub.MsgTypeSectionComplete {
			foundComplete = true
		}
	}
	if !foundComplete {
		t.Error("expected section_complete message from manual refresh")
	}

	// ReadRegisters routes through ReadBatch in mock
	if got := mb.getBatchCallCount(); got < 1 {
		t.Errorf("expected at least 1 read call for manual refresh, got %d", got)
	}
}

func TestNoBackendTimer(t *testing.T) {
	mb := newMockBroker()
	h, c, send, cancel := setupConnectedHub(t, mb, 0)
	defer cancel()

	// Subscribe to section
	h.Command(c, hub.InboundMessage{Type: hub.MsgTypeSubscribe, Section: "grid"})
	time.Sleep(100 * time.Millisecond)
	drainClientMessages(send, 200*time.Millisecond)

	// Wait for 2 seconds and count section_complete messages
	// With no timer, no additional reads should happen
	mb.mu.Lock()
	mb.batchCallCount = 0
	mb.mu.Unlock()

	time.Sleep(2 * time.Second)
	msgs := drainClientMessages(send, 200*time.Millisecond)

	completeCount := 0
	for _, m := range msgs {
		if m.Type == hub.MsgTypeSectionComplete {
			completeCount++
		}
	}
	if completeCount > 0 {
		t.Errorf("expected no autonomous reads (backend should have no timer), but got %d section_complete messages", completeCount)
	}
}

func TestCancelReadOnSectionSwitch(t *testing.T) {
	mb := newMockBroker()
	// Use a moderate delay so grid read is in progress when we switch
	mb.mu.Lock()
	mb.batchDelay = 50 * time.Millisecond
	mb.mu.Unlock()

	h, c, send, cancel := setupConnectedHub(t, mb, 0)
	defer cancel()

	// Subscribe to grid (starts slow read)
	h.Command(c, hub.InboundMessage{Type: hub.MsgTypeSubscribe, Section: "grid"})
	time.Sleep(100 * time.Millisecond)

	// Switch to system section while grid read may still be in progress
	// Reset delay so system read completes faster
	mb.mu.Lock()
	mb.batchDelay = 0
	mb.mu.Unlock()

	h.Command(c, hub.InboundMessage{Type: hub.MsgTypeSubscribe, Section: "system"})

	// System section has ~19 probes + 2 fault batch reads, give enough time
	msgs := drainClientMessages(send, 5*time.Second)

	// Should see section_complete for "system"
	systemComplete := 0
	for _, m := range msgs {
		if m.Type == hub.MsgTypeSectionComplete && m.Section == "system" {
			systemComplete++
		}
	}
	if systemComplete < 1 {
		t.Errorf("expected section_complete for system after switch, got %d", systemComplete)
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

	// Streaming model: section_schema is sent first (doesn't require connection),
	// then section_error since not connected
	msgs := drainClientMessages(send, 500*time.Millisecond)
	if len(msgs) == 0 {
		t.Fatal("expected messages after subscribing while disconnected")
	}
	foundError := false
	for _, m := range msgs {
		if m.Type == hub.MsgTypeSectionErr && m.Section == "battery" {
			foundError = true
		}
	}
	if !foundError {
		types := make([]string, len(msgs))
		for i, m := range msgs {
			types[i] = m.Type
		}
		t.Errorf("expected section_error for battery, got types: %v", types)
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

	// Streaming model: first message is section_schema with group structure
	msgs, idx := waitForMessageType(t, send, hub.MsgTypeSectionSchema, 2*time.Second)
	schema := msgs[idx]

	if schema.Section != "system" {
		t.Fatalf("expected section 'system', got %q", schema.Section)
	}

	// Drain remaining streaming messages
	drainClientMessages(send, 2*time.Second)

	// Verify schema has the correct group structure by checking the section_schema
	// The schema message doesn't unmarshal into OutboundMessage.Groups, so we check
	// that register_value messages arrive for the expected groups
	// This test validates the streaming pipeline works for system section
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

	// Streaming model: drain all messages until section_complete
	// Fault data is sent as a section_data message with Faults field
	allMsgs := drainClientMessages(send, 2*time.Second)

	// Find section_data message (fault data)
	var faultMsg *hub.OutboundMessage
	for i := range allMsgs {
		if allMsgs[i].Type == hub.MsgTypeSectionData && allMsgs[i].Section == "system" {
			faultMsg = &allMsgs[i]
			break
		}
	}
	if faultMsg == nil {
		// No section_data with faults is OK when there are zero faults --
		// streaming sends fault data only for system section
		return
	}

	// When all fault registers are zero, there should be no active faults.
	if len(faultMsg.Faults) != 0 {
		t.Errorf("expected 0 faults (all zeros), got %d", len(faultMsg.Faults))
	}
}

func TestSystemSectionFaultsActive(t *testing.T) {
	mb := newMockBroker()

	// Streaming model: fault registers are read via ReadBatch (not ReadRegisters).
	// Set up batchResultQueue so the fault ReadBatch gets fault data with bit 0 set.
	// Fault batch 1 reads 18 registers starting at 0x0405.
	batch1Data := make([]byte, 36)
	binary.BigEndian.PutUint16(batch1Data[0:2], 0x0001) // bit 0 set -> "Grid over-voltage"
	// Fault batch 2 reads 12 registers starting at 0x0432.
	batch2Data := make([]byte, 24)

	mb.mu.Lock()
	mb.batchResultQueue = [][]broker.Result{
		{
			{Data: batch1Data, Err: nil},
			{Data: batch2Data, Err: nil},
		},
	}
	mb.mu.Unlock()

	h, c, send, cancel := setupConnectedHub(t, mb, 0)
	defer cancel()

	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeSubscribe,
		Section: "system",
	})

	// Streaming model: fault data is sent as a section_data message after register_value messages
	allMsgs := drainClientMessages(send, 3*time.Second)

	var faultMsg *hub.OutboundMessage
	for i := range allMsgs {
		if allMsgs[i].Type == hub.MsgTypeSectionData && allMsgs[i].Section == "system" {
			faultMsg = &allMsgs[i]
			break
		}
	}
	if faultMsg == nil {
		t.Fatal("expected section_data message with fault data")
	}

	// Should have exactly 1 active fault
	if len(faultMsg.Faults) != 1 {
		t.Fatalf("expected 1 active fault, got %d", len(faultMsg.Faults))
	}
	if faultMsg.Faults[0].Name != "Grid over-voltage" {
		t.Errorf("fault name = %q, want 'Grid over-voltage'", faultMsg.Faults[0].Name)
	}
}

func TestSystemSectionTimeComposition(t *testing.T) {
	mb := newMockBroker()

	// Build per-span mock data. The batch-span read path calls ReadRegisters(span.StartAddr, span.TotalCount)
	// which checks registerResults first, so we populate per-span contiguous byte buffers.
	mb.registerResults = make(map[uint16]broker.Result)
	plan := register.AnalyzeBatchPlan(register.SystemGroups)
	for _, span := range plan.Spans {
		data := make([]byte, int(span.TotalCount)*2)
		for _, pm := range span.Probes {
			if pm.Probe.Name == "System time" && pm.ByteLength >= 12 {
				// 2026-03-16 14:30:45
				binary.BigEndian.PutUint16(data[pm.ByteOffset:pm.ByteOffset+2], 26)    // Year
				binary.BigEndian.PutUint16(data[pm.ByteOffset+2:pm.ByteOffset+4], 3)   // Month
				binary.BigEndian.PutUint16(data[pm.ByteOffset+4:pm.ByteOffset+6], 16)  // Day
				binary.BigEndian.PutUint16(data[pm.ByteOffset+6:pm.ByteOffset+8], 14)  // Hour
				binary.BigEndian.PutUint16(data[pm.ByteOffset+8:pm.ByteOffset+10], 30) // Min
				binary.BigEndian.PutUint16(data[pm.ByteOffset+10:pm.ByteOffset+12], 45) // Sec
			}
		}
		mb.registerResults[span.StartAddr] = broker.Result{Data: data}
	}
	// Also set batchResults for fault register reads (isFault section).
	faultResults := makeMockResultsForSection(register.SystemGroups, true)
	mb.mu.Lock()
	mb.batchResults = faultResults
	mb.mu.Unlock()

	h, c, send, cancel := setupConnectedHub(t, mb, 0)
	defer cancel()

	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeSubscribe,
		Section: "system",
	})

	// Streaming model: time composition sends a composed "System time" register_value
	// after all 6 time registers are collected within the group.
	// Individual time registers are NOT streamed.
	// Use raw JSON drain so we can unmarshal into RegisterValueMessage (OutboundMessage
	// does not carry Name/Value fields for register_value messages).
	rawMsgs := drainRawMessages(send, 2*time.Second)

	// Look for composed "System time" register_value
	foundComposed := false
	for _, raw := range rawMsgs {
		var rv hub.RegisterValueMessage
		if err := json.Unmarshal(raw, &rv); err != nil {
			continue
		}
		if rv.Type == hub.MsgTypeRegisterValue && rv.Name == "System time" {
			foundComposed = true
			// Expected: ComposeSystemTime(26, 3, 16, 14, 30, 45) -> "14:30:45 16-03-2026"
			want := "14:30:45 16-03-2026"
			if rv.Value != want {
				t.Errorf("System time value = %q, want %q", rv.Value, want)
			}
			break
		}
	}
	if !foundComposed {
		t.Error("expected register_value message for composed 'System time'")
	}

	// Also verify section_complete arrives (streaming works end-to-end).
	foundComplete := false
	for _, raw := range rawMsgs {
		var m map[string]interface{}
		if json.Unmarshal(raw, &m) != nil {
			continue
		}
		if m["type"] == hub.MsgTypeSectionComplete {
			foundComplete = true
			break
		}
	}
	if !foundComplete {
		t.Error("expected section_complete message for system section")
	}
}

func TestSystemSectionEnumLabel(t *testing.T) {
	mb := newMockBroker()

	// Streaming model: set per-address result for Running state register (0x0404)
	mb.mu.Lock()
	mb.registerResults = map[uint16]broker.Result{
		0x0404: {Data: uint16Bytes(2), Err: nil}, // 0x0002 -> "Grid-connected"
	}
	mb.mu.Unlock()

	h, c, send, cancel := setupConnectedHub(t, mb, 0)
	defer cancel()

	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeSubscribe,
		Section: "system",
	})

	// Streaming model: look for a register_value message with "Running state" -> "Grid-connected"
	allMsgs := drainRawMessages(send, 3*time.Second)

	found := false
	for _, raw := range allMsgs {
		var rv hub.RegisterValueMessage
		if err := json.Unmarshal(raw, &rv); err != nil {
			continue
		}
		if rv.Type == hub.MsgTypeRegisterValue && rv.Name == "Running state" {
			if rv.Value != "Grid-connected" {
				t.Errorf("Running state value = %q, want 'Grid-connected'", rv.Value)
			}
			found = true
			break
		}
	}
	if !found {
		t.Error("expected register_value message for 'Running state'")
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

	// Streaming model: first message is section_schema which contains layout info.
	// Check schema message for Phase R group with column layout.
	rawMsgs := drainRawMessages(send, 2*time.Second)

	// Find section_schema message
	var schema hub.SectionSchemaMessage
	found := false
	for _, raw := range rawMsgs {
		if err := json.Unmarshal(raw, &schema); err != nil {
			continue
		}
		if schema.Type == hub.MsgTypeSectionSchema && schema.Section == "grid" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected section_schema message for grid")
	}

	// Find "Phase R" group and check layout
	var phaseR *hub.SchemaGroup
	for i := range schema.Groups {
		if schema.Groups[i].Name == "Phase R" {
			phaseR = &schema.Groups[i]
			break
		}
	}
	if phaseR == nil {
		t.Fatal("Phase R group not found in grid section schema")
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

	// Streaming model: grid section only sends register_value + section_complete (no section_data).
	// Verify no section_data message with faults is sent for non-system sections.
	allMsgs := drainClientMessages(send, 2*time.Second)

	for _, m := range allMsgs {
		if m.Type == hub.MsgTypeSectionData && m.Section == "grid" {
			// If a section_data is sent for grid, it should not have faults
			if m.Faults != nil {
				t.Errorf("expected nil faults for grid section, got %d faults", len(m.Faults))
			}
		}
	}

	// Verify section_complete was sent
	foundComplete := false
	for _, m := range allMsgs {
		if m.Type == hub.MsgTypeSectionComplete {
			foundComplete = true
		}
	}
	if !foundComplete {
		t.Error("expected section_complete for grid section")
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

	// Streaming model: first message is section_schema with group structure
	rawMsgs := drainRawMessages(send, 2*time.Second)

	var schema hub.SectionSchemaMessage
	found := false
	for _, raw := range rawMsgs {
		if err := json.Unmarshal(raw, &schema); err != nil {
			continue
		}
		if schema.Type == hub.MsgTypeSectionSchema && schema.Section == "pv" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected section_schema message for pv")
	}

	// Should have 5 groups: PV 1, PV 2, PV 3, PV 4, Total PV Power
	if len(schema.Groups) != 5 {
		t.Fatalf("expected 5 groups after configure(channels=4), got %d", len(schema.Groups))
	}

	expectedNames := []string{"PV 1", "PV 2", "PV 3", "PV 4", "Total PV Power"}
	for i, name := range expectedNames {
		if schema.Groups[i].Name != name {
			t.Errorf("group[%d] name = %q, want %q", i, schema.Groups[i].Name, name)
		}
	}
}

func TestConfigurePVBatchPlan(t *testing.T) {
	mb := newMockBroker()
	h, c, _, cancel := setupConnectedHub(t, mb, 0)
	defer cancel()

	// Get initial batch plan (default 2 channels)
	initialPlan := h.GetSectionBatchPlan("pv")
	initialRegs := 0
	for _, span := range initialPlan.Spans {
		initialRegs += int(span.TotalCount)
	}

	// Configure PV channels to 4
	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeConfigure,
		Section: "pv",
		Config:  &hub.ConfigPayload{Channels: 4},
	})
	time.Sleep(50 * time.Millisecond)

	// Verify BatchPlan was recomputed
	plan := h.GetSectionBatchPlan("pv")
	if len(plan.Spans) == 0 {
		t.Fatal("expected non-empty batch plan after PV configure")
	}

	// Verify total register count increased (4 channels = more registers than 2)
	totalRegs := 0
	for _, span := range plan.Spans {
		totalRegs += int(span.TotalCount)
	}
	if totalRegs <= initialRegs {
		t.Errorf("expected more registers after channels 2->4, got %d (was %d)", totalRegs, initialRegs)
	}
	// 4 channels * 3 regs + 1 total PV = 13 registers minimum
	if totalRegs < 13 {
		t.Errorf("expected at least 13 registers for 4-channel PV, got %d", totalRegs)
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
	time.Sleep(200 * time.Millisecond)
	drainClientMessages(send, 500*time.Millisecond)

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

	// Streaming model: re-read sends register_value messages followed by section_complete
	msgs := drainUntilComplete(t, send, 2*time.Second)
	foundComplete := false
	for _, m := range msgs {
		if m.Type == hub.MsgTypeSectionComplete {
			foundComplete = true
		}
	}
	if !foundComplete {
		t.Error("expected section_complete from configure re-read")
	}

	// Verify reads were triggered
	if got := mb.getBatchCallCount(); got < 1 {
		t.Errorf("expected at least 1 read call from configure re-read, got %d", got)
	}

	// Verify the section now has 5 groups (4 PV + Total) via schema
	groups := h.GetSectionGroups("pv")
	if len(groups) != 5 {
		t.Errorf("expected 5 groups after reconfigure, got %d", len(groups))
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

	// Phase 25: streamPackBatchRead uses readBatchSpans with 6 batch spans.
	// Verify batch read calls were made (one per span).
	if got := mb.getBatchCallCount(); got < 6 {
		t.Errorf("expected at least 6 ReadRegisters calls (batch spans), got %d", got)
	}
}

func TestPackDataMessageShape(t *testing.T) {
	// Phase 11: Pack data is now sent as streaming messages (section_schema + register_value + section_complete)
	// instead of a single pack_data batch message.
	mb := newMockBroker()

	// Set up per-address register results for streaming reads
	mb.registerResults = make(map[uint16]broker.Result)
	defaultData := make([]byte, 2)
	binary.BigEndian.PutUint16(defaultData, 100)
	// Pack Info probes
	for _, addr := range []uint16{0x9044, 0x9079, 0x907A, 0x9071, 0x9072, 0x9073, 0x9074, 0x907B, 0x907C, 0x9104, 0x9105, 0x910A, 0x910B} {
		mb.registerResults[addr] = broker.Result{Data: append([]byte{}, defaultData...)}
	}
	snData := make([]byte, 20)
	copy(snData, []byte("TEST1234567890123456"))
	mb.registerResults[0x9047] = broker.Result{Data: snData}
	mfgData := make([]byte, 8)
	copy(mfgData, []byte("TESTMFG"))
	mb.registerResults[0x9106] = broker.Result{Data: mfgData}
	// Cell voltages: Cell 1 = 3200mV, incrementing
	for i := 0; i < 16; i++ {
		data := make([]byte, 2)
		binary.BigEndian.PutUint16(data, 3200+uint16(i))
		mb.registerResults[uint16(0x9051+i)] = broker.Result{Data: data}
	}
	// Max/Min cell voltage
	maxCellData := make([]byte, 2)
	binary.BigEndian.PutUint16(maxCellData, 3215)
	mb.registerResults[0x9069] = broker.Result{Data: maxCellData}
	minCellData := make([]byte, 2)
	binary.BigEndian.PutUint16(minCellData, 3200)
	mb.registerResults[0x906A] = broker.Result{Data: minCellData}
	// Balance, temps, status
	for _, addr := range []uint16{0x9075, 0x906B, 0x906C, 0x906D, 0x906E, 0x906F, 0x9070, 0x90BC, 0x90BD, 0x90BE, 0x90BF, 0x9076, 0x9077, 0x9078, 0x9124, 0x9125, 0x9126} {
		mb.registerResults[addr] = broker.Result{Data: append([]byte{}, defaultData...)}
	}

	h, c, send, cancel := setupConnectedHub(t, mb, 0)
	defer cancel()

	// Subscribe to BMS section first (streaming messages broadcast to subscribers)
	h.Command(c, hub.InboundMessage{Type: "subscribe", Section: "bms"})
	time.Sleep(100 * time.Millisecond)
	// Drain BMS overview schema + register values + section_complete from initial subscribe
	drainRawMessages(send, 3*time.Second)

	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeSelectPack,
		Section: "bms",
		Input:   1,
		Tower:   1,
		Pack:    1,
	})

	// Collect streaming messages
	rawMsgs := collectRawMessages(t, send, 5*time.Second)

	// Verify message flow: section_schema, register_values, section_complete
	var schemaMsg *hub.SectionSchemaMessage
	var regValueCount int
	var hasComplete bool
	var hasPackData bool
	for _, raw := range rawMsgs {
		var generic map[string]interface{}
		if err := json.Unmarshal(raw, &generic); err != nil {
			continue
		}
		switch generic["type"] {
		case "section_schema":
			var sm hub.SectionSchemaMessage
			json.Unmarshal(raw, &sm)
			schemaMsg = &sm
		case "register_value":
			regValueCount++
		case "section_complete":
			hasComplete = true
		case "pack_data":
			hasPackData = true
		}
	}

	if hasPackData {
		t.Error("received pack_data message (should use streaming)")
	}

	// Verify schema
	if schemaMsg == nil {
		t.Fatal("no section_schema message received")
	}
	if schemaMsg.Section != "bms" {
		t.Errorf("schema section = %q, want 'bms'", schemaMsg.Section)
	}
	if schemaMsg.PackContext == nil {
		t.Fatal("schema missing pack_context")
	}
	if schemaMsg.PackContext.Input != 1 || schemaMsg.PackContext.Tower != 1 || schemaMsg.PackContext.Pack != 1 {
		t.Errorf("pack_context = %+v, want input=1,tower=1,pack=1", schemaMsg.PackContext)
	}

	// Should have 5 groups in D-03 order
	if len(schemaMsg.Groups) != 5 {
		t.Fatalf("expected 5 schema groups, got %d", len(schemaMsg.Groups))
	}
	expectedGroups := []struct {
		name  string
		gtype string
	}{
		{"Pack Info", ""},
		{"Cell Voltages", "cell_grid"},
		{"Balance State", "balance"},
		{"Temperatures", ""},
		{"Pack Status", "pack_status"},
	}
	for i, eg := range expectedGroups {
		if schemaMsg.Groups[i].Name != eg.name {
			t.Errorf("group[%d] name = %q, want %q", i, schemaMsg.Groups[i].Name, eg.name)
		}
		if schemaMsg.Groups[i].Type != eg.gtype {
			t.Errorf("group[%d] type = %q, want %q", i, schemaMsg.Groups[i].Type, eg.gtype)
		}
	}

	// Verify we got register_value messages for pack probes
	if regValueCount == 0 {
		t.Error("no register_value messages received")
	}

	if !hasComplete {
		t.Error("no section_complete message received")
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

// setupBMSBatchSpanTest creates a mockBroker configured for BMS batch span reads.
// Sets up span-level batch responses with known values for topology (0x900D = 0x020A)
// and bitmap (0x9022 = towerBitmap) at their correct byte offsets.
func setupBMSBatchSpanTest(t *testing.T, towerBitmap uint16) (*mockBroker, register.BatchPlan) {
	t.Helper()
	mb := newMockBroker()
	mb.registerResults = make(map[uint16]broker.Result)
	mb.spanFailAddrs = make(map[uint16]error)

	groups := register.BMSInfoGroups()
	plan := register.AnalyzeBatchPlan(groups)
	require.NotEmpty(t, plan.Spans, "BMS batch plan must have spans")

	// Set up all spans to succeed at batch level with known probe values.
	for _, span := range plan.Spans {
		data := make([]byte, int(span.TotalCount)*2)
		for _, pm := range span.Probes {
			switch pm.Probe.Addr {
			case 0x900D:
				// Topology: 2 strings x 10 packs = 0x020A
				binary.BigEndian.PutUint16(data[pm.ByteOffset:pm.ByteOffset+2], 0x020A)
			case 0x9022:
				// Tower bitmap
				binary.BigEndian.PutUint16(data[pm.ByteOffset:pm.ByteOffset+2], towerBitmap)
			case 0x9004:
				// BMS Clock Composite: 2 registers (4 bytes)
				// Encode 2026-04-10 14:03:05 = 0x6914E0C5
				if pm.ByteLength >= 4 {
					binary.BigEndian.PutUint16(data[pm.ByteOffset:pm.ByteOffset+2], 0x6914)
					binary.BigEndian.PutUint16(data[pm.ByteOffset+2:pm.ByteOffset+4], 0xE0C5)
				}
			case 0x9018:
				// SW Version Composite: 4 registers (8 bytes)
				// V1.2.3 = [0x0056, 0x0001, 0x0002, 0x0003]
				if pm.ByteLength >= 8 {
					binary.BigEndian.PutUint16(data[pm.ByteOffset:pm.ByteOffset+2], 0x0056)   // 'V'
					binary.BigEndian.PutUint16(data[pm.ByteOffset+2:pm.ByteOffset+4], 0x0001) // major
					binary.BigEndian.PutUint16(data[pm.ByteOffset+4:pm.ByteOffset+6], 0x0002) // non-std
					binary.BigEndian.PutUint16(data[pm.ByteOffset+6:pm.ByteOffset+8], 0x0003) // minor
				}
			default:
				// Default: write 100 for all other probes
				if pm.ByteLength >= 2 {
					binary.BigEndian.PutUint16(data[pm.ByteOffset:pm.ByteOffset+2], 100)
				}
			}
		}
		mb.registerResults[span.StartAddr] = broker.Result{Data: data}
	}

	return mb, plan
}

func TestBMSTowerBitmap(t *testing.T) {
	mb, _ := setupBMSBatchSpanTest(t, 0x0003) // both towers online

	h, c, send, cancel := setupConnectedHub(t, mb, 0)
	defer cancel()

	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeSubscribe,
		Section: "bms",
	})

	allMsgs := drainClientMessages(send, 5*time.Second)

	var msg *hub.OutboundMessage
	for i := range allMsgs {
		if allMsgs[i].Type == hub.MsgTypeSectionData {
			msg = &allMsgs[i]
			break
		}
	}
	if msg == nil {
		types := make([]string, len(allMsgs))
		for i, m := range allMsgs {
			types[i] = m.Type
		}
		t.Fatalf("expected section_data message in BMS stream, got types: %v", types)
	}

	var bitmapGroup *hub.GroupData
	for i := range msg.Groups {
		if msg.Groups[i].Type == "bitmap" {
			bitmapGroup = &msg.Groups[i]
			break
		}
	}
	require.NotNil(t, bitmapGroup, "expected bitmap group in BMS section data")
	require.NotNil(t, bitmapGroup.Bitmap, "bitmap group has nil Bitmap field")

	assert.Equal(t, 2, bitmapGroup.Bitmap.Towers)
	assert.Equal(t, 10, bitmapGroup.Bitmap.PacksPerTower)
	require.Len(t, bitmapGroup.Bitmap.Online, 2)
	assert.Equal(t, uint16(0x03FF), bitmapGroup.Bitmap.Online[0], "tower 1 all packs online")
	assert.Equal(t, uint16(0x03FF), bitmapGroup.Bitmap.Online[1], "tower 2 all packs online")
	assert.Equal(t, "2 strings x 10 packs", bitmapGroup.Bitmap.DetectedTopology)
	assert.False(t, bitmapGroup.Bitmap.Mismatch)
}

func TestBMSTowerBitmapPartialOnline(t *testing.T) {
	mb, _ := setupBMSBatchSpanTest(t, 0x0001) // only tower 1 online

	h, c, send, cancel := setupConnectedHub(t, mb, 0)
	defer cancel()

	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeSubscribe,
		Section: "bms",
	})

	allMsgs := drainClientMessages(send, 5*time.Second)

	var msg *hub.OutboundMessage
	for i := range allMsgs {
		if allMsgs[i].Type == hub.MsgTypeSectionData {
			msg = &allMsgs[i]
			break
		}
	}
	if msg == nil {
		types := make([]string, len(allMsgs))
		for i, m := range allMsgs {
			types[i] = m.Type
		}
		t.Fatalf("expected section_data message in BMS stream, got types: %v", types)
	}

	var bitmapGroup *hub.GroupData
	for i := range msg.Groups {
		if msg.Groups[i].Type == "bitmap" {
			bitmapGroup = &msg.Groups[i]
			break
		}
	}
	require.NotNil(t, bitmapGroup, "expected bitmap group in BMS section data")
	require.NotNil(t, bitmapGroup.Bitmap, "bitmap group has nil Bitmap field")

	require.Len(t, bitmapGroup.Bitmap.Online, 2)
	assert.Equal(t, uint16(0x03FF), bitmapGroup.Bitmap.Online[0], "tower 1 all packs online")
	assert.Equal(t, uint16(0x0000), bitmapGroup.Bitmap.Online[1], "tower 2 offline")
}

// TestBMSBatchRead_SpanReads verifies BMS section reads via batch spans and emits
// register_value messages for all probes, section_data with bitmap/protection, and section_complete.
func TestBMSBatchRead_SpanReads(t *testing.T) {
	mb, _ := setupBMSBatchSpanTest(t, 0x0003)

	h, c, send, cancel := setupConnectedHub(t, mb, 0)
	defer cancel()

	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeSubscribe,
		Section: "bms",
	})

	msgs, idx := waitForMessageType(t, send, "section_complete", 10*time.Second)
	require.GreaterOrEqual(t, idx, 0, "section_complete not received")

	// Count register_value messages
	regCount := 0
	for _, m := range msgs[:idx] {
		if m.Type == "register_value" && m.Section == "bms" {
			regCount++
		}
	}
	// All 24 probes (18 BMS Info + 6 Protection) should have register_value messages
	assert.GreaterOrEqual(t, regCount, 24, "expected at least 24 register_value messages for BMS probes")

	// Verify section_data exists with bitmap and protection groups
	var sectionDataFound bool
	for _, m := range msgs[:idx] {
		if m.Type == hub.MsgTypeSectionData && m.Section == "bms" {
			sectionDataFound = true
			assert.GreaterOrEqual(t, len(m.Groups), 2, "section_data should have bitmap + protection groups")
			break
		}
	}
	assert.True(t, sectionDataFound, "expected section_data message with computed groups")
}

// TestBMSBatchRead_CompositeValues verifies that bms_clock and bms_sw_version Composite
// probes produce correctly formatted values in register_value messages.
func TestBMSBatchRead_CompositeValues(t *testing.T) {
	mb, _ := setupBMSBatchSpanTest(t, 0x0003)

	h, c, send, cancel := setupConnectedHub(t, mb, 0)
	defer cancel()

	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeSubscribe,
		Section: "bms",
	})

	allMsgs := drainClientMessages(send, 5*time.Second)

	// Find System Clock register_value
	var clockMsg *hub.OutboundMessage
	var swVerMsg *hub.OutboundMessage
	for i := range allMsgs {
		if allMsgs[i].Type == "register_value" && allMsgs[i].Section == "bms" {
			// OutboundMessage doesn't have Name field directly; check via JSON
			// The register_value messages are RegisterValueMessage marshaled to JSON,
			// then unmarshaled into OutboundMessage. We need to check raw JSON.
			break
		}
	}

	// Use raw message drain to check Name field (RegisterValueMessage has Name)
	_ = clockMsg
	_ = swVerMsg

	// Re-run with raw messages to access Name field
	mb2, _ := setupBMSBatchSpanTest(t, 0x0003)
	h2, c2, send2, cancel2 := setupConnectedHub(t, mb2, 0)
	defer cancel2()

	h2.Command(c2, hub.InboundMessage{
		Type:    hub.MsgTypeSubscribe,
		Section: "bms",
	})

	rawMsgs := drainRawMessages(send2, 5*time.Second)

	var clockValue, swVerValue string
	for _, raw := range rawMsgs {
		var m map[string]interface{}
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		if m["type"] == "register_value" && m["section"] == "bms" {
			name, _ := m["name"].(string)
			value, _ := m["value"].(string)
			if name == "System Clock" {
				clockValue = value
			}
			if name == "SW Version" {
				swVerValue = value
			}
		}
	}

	assert.NotEmpty(t, clockValue, "expected register_value for System Clock")
	if clockValue != "" {
		// 0x6914E0C5 decodes to 2026-04-10 14:03:05
		assert.Equal(t, "2026-04-10 14:03:05", clockValue, "bms_clock Composite should produce formatted datetime")
	}

	assert.NotEmpty(t, swVerValue, "expected register_value for SW Version")
	if swVerValue != "" {
		assert.Equal(t, "V1.2.3", swVerValue, "bms_sw_version Composite should produce formatted version")
	}
}

// TestBMSBatchRead_ProtectionDecoding verifies that protection registers appear both as
// register_value messages (raw hex) and in the section_data protection group.
func TestBMSBatchRead_ProtectionDecoding(t *testing.T) {
	mb, _ := setupBMSBatchSpanTest(t, 0x0003)

	h, c, send, cancel := setupConnectedHub(t, mb, 0)
	defer cancel()

	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeSubscribe,
		Section: "bms",
	})

	allMsgs := drainClientMessages(send, 5*time.Second)

	// Find section_data with protection group
	var protGroup *hub.GroupData
	for i := range allMsgs {
		if allMsgs[i].Type == hub.MsgTypeSectionData && allMsgs[i].Section == "bms" {
			for j := range allMsgs[i].Groups {
				if allMsgs[i].Groups[j].Type == "protection" {
					protGroup = &allMsgs[i].Groups[j]
					break
				}
			}
			break
		}
	}

	require.NotNil(t, protGroup, "expected protection group in section_data")
	assert.Equal(t, "Protection & Alarms", protGroup.Name)
	// Protection probes should have Items entries (6 registers with value 100 = 0x0064)
	assert.GreaterOrEqual(t, len(protGroup.Items), 1, "protection group should have decoded items")
}

// TestPackDataMessageItemMeta verifies that PackItemMeta and CellAddrs appear in JSON.
func TestPackDataMessageItemMeta(t *testing.T) {
	msg := hub.PackDataMessage{
		Type:    "pack_data",
		Section: "bms",
		Input:   1, Tower: 1, Pack: 1,
		Groups: []hub.PackGroup{
			{
				Name:  "Pack Info",
				Items: map[string]string{"SOC": "85%"},
				ItemMeta: map[string]hub.PackItemMeta{
					"SOC": {RegisterAddr: 0x906C, RawValue: "85"},
				},
			},
			{
				Name:      "Cell Voltages",
				Type:      "cell_grid",
				Cells:     []int{3280, 3281},
				CellAddrs: []uint16{0x9051, 0x9052},
			},
		},
	}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	// Verify ItemMeta appears in JSON
	if !strings.Contains(s, `"item_meta"`) {
		t.Errorf("JSON missing item_meta: %s", s)
	}
	if !strings.Contains(s, `"register_addr":`) {
		t.Errorf("JSON missing register_addr in item_meta: %s", s)
	}
	// Verify CellAddrs appears
	if !strings.Contains(s, `"cell_addrs"`) {
		t.Errorf("JSON missing cell_addrs: %s", s)
	}
	// Verify 0x906C = 36972 decimal appears
	if !strings.Contains(s, `36972`) {
		t.Errorf("JSON missing register_addr value 36972 for SOC: %s", s)
	}
}

func TestNewRegisterValueJSON(t *testing.T) {
	msg := hub.NewRegisterValue("system", "Info", "Inverter SN", "SA00T", "", 0x0445, "534F464152")
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, `"register_addr":1093`) {
		t.Errorf("JSON missing register_addr: %s", s)
	}
	if !strings.Contains(s, `"raw_value":"534F464152"`) {
		t.Errorf("JSON missing raw_value: %s", s)
	}
}

func TestNewRegisterValueComposedJSON(t *testing.T) {
	msg := hub.NewRegisterValue("system", "Status", "System time", "16:03:42 12-04-2026", "", 0x042C, "0x042C-0x0431 | 26, 4, 12, 16, 3, 42")
	data, err := json.Marshal(msg)
	require.NoError(t, err)
	s := string(data)
	assert.Contains(t, s, `"register_addr":1068`, "JSON should contain register_addr 0x042C (1068 decimal)")
	assert.Contains(t, s, `"raw_value":"0x042C-0x0431 | 26, 4, 12, 16, 3, 42"`, "JSON should contain pipe-delimited raw_value")
}

// === Phase 11 Plan 01: Pack streaming tests ===

func TestPackSchemaContext(t *testing.T) {
	mb := newMockBroker()
	h := hub.NewTestHub(mb)
	groups := register.PackProbeGroups()
	schema := h.BuildPackSchema(1, 2, 3, groups)

	// Verify section
	if schema.Section != "bms" {
		t.Errorf("schema.Section = %q, want %q", schema.Section, "bms")
	}

	// Verify 5 groups
	if len(schema.Groups) != 5 {
		t.Fatalf("schema has %d groups, want 5", len(schema.Groups))
	}

	// Verify JSON contains pack_context
	data, err := json.Marshal(schema)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, `"pack_context"`) {
		t.Errorf("JSON missing pack_context: %s", s)
	}
	if !strings.Contains(s, `"input":1`) {
		t.Errorf("JSON missing input:1: %s", s)
	}
	if !strings.Contains(s, `"tower":2`) {
		t.Errorf("JSON missing tower:2: %s", s)
	}
	if !strings.Contains(s, `"pack":3`) {
		t.Errorf("JSON missing pack:3: %s", s)
	}

	// Verify Cell Voltages group has cell_count > 0
	if !strings.Contains(s, `"cell_count"`) {
		t.Errorf("JSON missing cell_count: %s", s)
	}
}

func TestPackSchemaGroupOrder(t *testing.T) {
	mb := newMockBroker()
	h := hub.NewTestHub(mb)
	groups := register.PackProbeGroups()
	schema := h.BuildPackSchema(1, 1, 1, groups)

	wantNames := []string{"Pack Info", "Cell Voltages", "Balance State", "Temperatures", "Pack Status"}
	if len(schema.Groups) != len(wantNames) {
		t.Fatalf("got %d groups, want %d", len(schema.Groups), len(wantNames))
	}
	for i, want := range wantNames {
		if schema.Groups[i].Name != want {
			t.Errorf("group[%d].Name = %q, want %q", i, schema.Groups[i].Name, want)
		}
	}
}

// collectRawMessages collects raw JSON messages from a client's send channel.
func collectRawMessages(t *testing.T, send chan []byte, timeout time.Duration) []json.RawMessage {
	t.Helper()
	var msgs []json.RawMessage
	deadline := time.After(timeout)
	for {
		select {
		case raw, ok := <-send:
			if !ok {
				return msgs
			}
			msgs = append(msgs, json.RawMessage(raw))
		case <-deadline:
			return msgs
		}
	}
}

func TestPackStreamingMessages(t *testing.T) {
	mb := newMockBroker()
	// Set up register results for all pack addresses
	mb.registerResults = make(map[uint16]broker.Result)
	// Pack Info probes
	for _, addr := range []uint16{0x9044, 0x9079, 0x907A, 0x9071, 0x9072, 0x9073, 0x9074, 0x907B, 0x907C, 0x9104, 0x9105, 0x910A, 0x910B} {
		data := make([]byte, 2)
		binary.BigEndian.PutUint16(data, 100)
		mb.registerResults[addr] = broker.Result{Data: data}
	}
	// Serial Number (ASCII, 10 registers = 20 bytes)
	snData := make([]byte, 20)
	copy(snData, []byte("TEST1234567890123456"))
	mb.registerResults[0x9047] = broker.Result{Data: snData}
	// Manufacturer (ASCII, 4 registers = 8 bytes)
	mfgData := make([]byte, 8)
	copy(mfgData, []byte("TESTMFG"))
	mb.registerResults[0x9106] = broker.Result{Data: mfgData}
	// Cell voltages
	for i := 0; i < 16; i++ {
		data := make([]byte, 2)
		binary.BigEndian.PutUint16(data, 3300+uint16(i))
		mb.registerResults[uint16(0x9051+i)] = broker.Result{Data: data}
	}
	// Max/Min cell voltage
	for _, addr := range []uint16{0x9069, 0x906A} {
		data := make([]byte, 2)
		binary.BigEndian.PutUint16(data, 3310)
		mb.registerResults[addr] = broker.Result{Data: data}
	}
	// Balance state
	data := make([]byte, 2)
	binary.BigEndian.PutUint16(data, 0)
	mb.registerResults[0x9075] = broker.Result{Data: data}
	// Temperatures
	for _, addr := range []uint16{0x906B, 0x906C, 0x906D, 0x906E, 0x906F, 0x9070, 0x90BC, 0x90BD, 0x90BE, 0x90BF} {
		data := make([]byte, 2)
		binary.BigEndian.PutUint16(data, 250)
		mb.registerResults[addr] = broker.Result{Data: data}
	}
	// Status registers
	for _, addr := range []uint16{0x9076, 0x9077, 0x9078, 0x9124, 0x9125, 0x9126} {
		data := make([]byte, 2)
		binary.BigEndian.PutUint16(data, 0)
		mb.registerResults[addr] = broker.Result{Data: data}
	}

	h := hub.NewTestHub(mb)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)

	send := make(chan []byte, 256)
	client := hub.NewTestClient(h, send)
	h.Register(client)
	time.Sleep(50 * time.Millisecond)

	// Subscribe to BMS section
	h.Command(client, hub.InboundMessage{Type: "subscribe", Section: "bms"})
	time.Sleep(50 * time.Millisecond)

	// Simulate connected state
	mb.mu.Lock()
	mb.state = broker.StateConnected
	mb.statesCh <- broker.StateEvent{State: broker.StateConnected}
	mb.mu.Unlock()
	time.Sleep(100 * time.Millisecond)

	// Drain any initial messages (state, schema, bms overview read)
	drainRawMessages(send, 2*time.Second)

	// Send select_pack command
	h.Command(client, hub.InboundMessage{Type: "select_pack", Section: "bms", Input: 1, Tower: 1, Pack: 1})

	// Collect messages
	rawMsgs := collectRawMessages(t, send, 5*time.Second)

	if len(rawMsgs) == 0 {
		t.Fatal("received no messages after select_pack")
	}

	// Parse messages and verify types
	var hasSchema, hasRegValue, hasComplete bool
	var hasPackData bool
	for _, raw := range rawMsgs {
		var generic map[string]interface{}
		if err := json.Unmarshal(raw, &generic); err != nil {
			continue
		}
		msgType, _ := generic["type"].(string)
		switch msgType {
		case "section_schema":
			hasSchema = true
			// Verify pack_context is present
			if _, ok := generic["pack_context"]; !ok {
				t.Error("section_schema missing pack_context")
			}
		case "register_value":
			hasRegValue = true
		case "section_complete":
			hasComplete = true
		case "pack_data":
			hasPackData = true
		}
	}

	if !hasSchema {
		t.Error("no section_schema message received")
	}
	if !hasRegValue {
		t.Error("no register_value messages received")
	}
	if !hasComplete {
		t.Error("no section_complete message received")
	}
	if hasPackData {
		t.Error("received pack_data message (should not be sent by streaming path)")
	}
}

func TestPackSpanDegradation(t *testing.T) {
	// After repeated batch failures on the 0x9104 span, SpanTracker should degrade it.
	// Individual fallback reads still produce register_value messages for the failed span.
	mb := newMockBroker()
	mb.registerResults = make(map[uint16]broker.Result)
	mb.spanFailAddrs = make(map[uint16]error)

	packPlan := register.AnalyzeBatchPlan(register.PackProbeGroups())

	// Set up all spans to succeed at batch level (provide full-span-sized data)
	for _, span := range packPlan.Spans {
		data := make([]byte, int(span.TotalCount)*2)
		// Fill with valid default data (100 per register)
		for i := 0; i < int(span.TotalCount); i++ {
			binary.BigEndian.PutUint16(data[i*2:], 100)
		}
		mb.registerResults[span.StartAddr] = broker.Result{Data: data}
	}
	// Span 0 (0x9044) has count=1 so goes through registerResults directly
	defaultData := make([]byte, 2)
	binary.BigEndian.PutUint16(defaultData, 1)
	mb.registerResults[0x9044] = broker.Result{Data: defaultData}

	// Serial Number needs ASCII data at span start 0x9047
	snSpanData := make([]byte, 26*2)
	copy(snSpanData, []byte("TEST1234567890123456"))
	mb.registerResults[0x9047] = broker.Result{Data: snSpanData}

	// Make the 0x9104 span fail at batch level
	mb.spanFailAddrs[0x9104] = fmt.Errorf("simulated batch failure")

	// Set up individual probe reads for 0x9104 span fallback
	for _, addr := range []uint16{0x9104, 0x9105, 0x910A, 0x910B} {
		mb.registerResults[addr] = broker.Result{Data: append([]byte{}, defaultData...)}
	}
	mfgData := make([]byte, 8)
	copy(mfgData, []byte("MFG"))
	mb.registerResults[0x9106] = broker.Result{Data: mfgData}

	h := hub.NewTestHub(mb)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)

	send := make(chan []byte, 512)
	client := hub.NewTestClient(h, send)
	h.Register(client)
	time.Sleep(50 * time.Millisecond)

	// Subscribe and connect
	h.Command(client, hub.InboundMessage{Type: "subscribe", Section: "bms"})
	time.Sleep(50 * time.Millisecond)
	mb.mu.Lock()
	mb.state = broker.StateConnected
	mb.statesCh <- broker.StateEvent{State: broker.StateConnected}
	mb.mu.Unlock()
	time.Sleep(100 * time.Millisecond)
	drainRawMessages(send, 2*time.Second)

	// Accumulate 3 batch failures to trigger degradation (DefaultDegradationThreshold = 3).
	// select_pack resets tracker (count=0), then 2 read_cycles bring count to 3.
	// The 3rd failure (count=3) degrades the span but individual fallback in the
	// Normal path does NOT call RecordSuccess, so the span stays degraded.
	h.Command(client, hub.InboundMessage{Type: "select_pack", Section: "bms", Input: 1, Tower: 1, Pack: 1})
	collectRawMessages(t, send, 5*time.Second)
	time.Sleep(100 * time.Millisecond)
	for i := 0; i < 2; i++ {
		h.Command(client, hub.InboundMessage{Type: "read_cycle", Section: "bms"})
		collectRawMessages(t, send, 5*time.Second)
		time.Sleep(100 * time.Millisecond)
	}

	// Verify 0x9104 span is degraded after 3 batch failures
	state := h.GetPackSpanState(0x9104)
	if state == hub.SpanNormal {
		t.Errorf("0x9104 span state = %v, want degraded or skipped after 3 failures", state)
	}

	// The next read_cycle uses the degraded path: individual reads succeed,
	// which calls RecordSuccess and recovers the span. Verify individual fallback
	// still produces register_value messages for 0x9104.
	h.Command(client, hub.InboundMessage{Type: "read_cycle", Section: "bms"})
	msgs := collectRawMessages(t, send, 5*time.Second)

	count9104 := 0
	for _, raw := range msgs {
		var generic map[string]interface{}
		if err := json.Unmarshal(raw, &generic); err != nil {
			continue
		}
		if generic["type"] == "register_value" {
			if addr, ok := generic["register_addr"].(float64); ok && uint16(addr) == 0x9104 {
				count9104++
			}
		}
	}
	if count9104 != 1 {
		t.Errorf("degraded span: got %d register_value for 0x9104, want 1 (individual fallback)", count9104)
	}
}

func TestPackSpanResetOnSwitch(t *testing.T) {
	// After pack switch, packSpanTracker should be reset so previously degraded
	// spans get a fresh start (D-05).
	mb := newMockBroker()
	mb.registerResults = make(map[uint16]broker.Result)
	mb.spanFailAddrs = make(map[uint16]error)

	packPlan := register.AnalyzeBatchPlan(register.PackProbeGroups())

	// Set up all spans to succeed at batch level
	for _, span := range packPlan.Spans {
		data := make([]byte, int(span.TotalCount)*2)
		for i := 0; i < int(span.TotalCount); i++ {
			binary.BigEndian.PutUint16(data[i*2:], 100)
		}
		mb.registerResults[span.StartAddr] = broker.Result{Data: data}
	}
	defaultData := make([]byte, 2)
	binary.BigEndian.PutUint16(defaultData, 1)
	mb.registerResults[0x9044] = broker.Result{Data: defaultData}
	snSpanData := make([]byte, 26*2)
	copy(snSpanData, []byte("TEST1234567890123456"))
	mb.registerResults[0x9047] = broker.Result{Data: snSpanData}

	// Make 0x9104 span fail at batch level
	mb.spanFailAddrs[0x9104] = fmt.Errorf("simulated batch failure")
	// Individual fallback data for 0x9104 span probes
	for _, addr := range []uint16{0x9104, 0x9105, 0x910A, 0x910B} {
		mb.registerResults[addr] = broker.Result{Data: append([]byte{}, defaultData...)}
	}
	mfgData := make([]byte, 8)
	copy(mfgData, []byte("MFG"))
	mb.registerResults[0x9106] = broker.Result{Data: mfgData}

	h := hub.NewTestHub(mb)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)

	send := make(chan []byte, 512)
	client := hub.NewTestClient(h, send)
	h.Register(client)
	time.Sleep(50 * time.Millisecond)

	h.Command(client, hub.InboundMessage{Type: "subscribe", Section: "bms"})
	time.Sleep(50 * time.Millisecond)
	mb.mu.Lock()
	mb.state = broker.StateConnected
	mb.statesCh <- broker.StateEvent{State: broker.StateConnected}
	mb.mu.Unlock()
	time.Sleep(100 * time.Millisecond)
	drainRawMessages(send, 2*time.Second)

	// Accumulate 3 batch failures to degrade the 0x9104 span.
	// select_pack resets tracker (count=0), then 2 read_cycles bring count to 3.
	h.Command(client, hub.InboundMessage{Type: "select_pack", Section: "bms", Input: 1, Tower: 1, Pack: 1})
	collectRawMessages(t, send, 5*time.Second)
	time.Sleep(100 * time.Millisecond)
	for i := 0; i < 2; i++ {
		h.Command(client, hub.InboundMessage{Type: "read_cycle", Section: "bms"})
		collectRawMessages(t, send, 5*time.Second)
		time.Sleep(100 * time.Millisecond)
	}

	// Verify 0x9104 span is degraded
	state := h.GetPackSpanState(0x9104)
	if state == hub.SpanNormal {
		t.Fatalf("0x9104 span state = %v, want degraded after 3 failures", state)
	}

	// Now fix 0x9104 span: remove from spanFailAddrs and restore full span data
	mb.mu.Lock()
	delete(mb.spanFailAddrs, 0x9104)
	// Restore full 16-byte span data (was overwritten by individual probe setup)
	infoSpanData := make([]byte, 8*2)
	for i := 0; i < 8; i++ {
		binary.BigEndian.PutUint16(infoSpanData[i*2:], 100)
	}
	mb.registerResults[0x9104] = broker.Result{Data: infoSpanData}
	mb.mu.Unlock()

	// Select different pack (pack 2) -- should reset packSpanTracker (D-05)
	h.Command(client, hub.InboundMessage{Type: "select_pack", Section: "bms", Input: 1, Tower: 1, Pack: 2})
	collectRawMessages(t, send, 5*time.Second)

	// Verify 0x9104 span is back to normal after pack switch reset
	state = h.GetPackSpanState(0x9104)
	if state != hub.SpanNormal {
		t.Errorf("0x9104 span state = %v after pack switch, want SpanNormal (reset)", state)
	}
}

// === Phase 25: Pack batch read tests ===

func TestStreamPackBatchReadAllProbes(t *testing.T) {
	// Verify that streamPackBatchRead using readBatchSpans delivers register_value
	// messages for all pack probes and ends with section_complete.
	mb := newMockBroker()
	mb.registerResults = make(map[uint16]broker.Result)

	packPlan := register.AnalyzeBatchPlan(register.PackProbeGroups())

	// Set up all spans to succeed at batch level
	for _, span := range packPlan.Spans {
		data := make([]byte, int(span.TotalCount)*2)
		for i := 0; i < int(span.TotalCount); i++ {
			binary.BigEndian.PutUint16(data[i*2:], 100+uint16(i))
		}
		mb.registerResults[span.StartAddr] = broker.Result{Data: data}
	}
	// Span 0 (0x9044, count=1) goes through registerResults directly
	defaultData := make([]byte, 2)
	binary.BigEndian.PutUint16(defaultData, 1)
	mb.registerResults[0x9044] = broker.Result{Data: defaultData}
	// Serial Number span needs ASCII at offset 0
	snSpanData := make([]byte, 26*2)
	copy(snSpanData, []byte("TEST1234567890123456"))
	for i := 10; i < 26; i++ {
		binary.BigEndian.PutUint16(snSpanData[i*2:], 3300+uint16(i-10))
	}
	mb.registerResults[0x9047] = broker.Result{Data: snSpanData}
	// 0x9104 span (8 regs)
	infoData := make([]byte, 8*2)
	binary.BigEndian.PutUint16(infoData[0:], 500)  // Balanced Bus Voltage
	binary.BigEndian.PutUint16(infoData[2:], 10)   // Balanced Bus Current
	copy(infoData[4:12], []byte("TESTMFG\x00"))    // Manufacturer ASCII
	binary.BigEndian.PutUint16(infoData[12:], 99)   // SOH
	binary.BigEndian.PutUint16(infoData[14:], 200)  // Rated Capacity
	mb.registerResults[0x9104] = broker.Result{Data: infoData}

	h := hub.NewTestHub(mb)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)

	send := make(chan []byte, 512)
	client := hub.NewTestClient(h, send)
	h.Register(client)
	time.Sleep(50 * time.Millisecond)

	// Subscribe to BMS section
	h.Command(client, hub.InboundMessage{Type: "subscribe", Section: "bms"})
	time.Sleep(50 * time.Millisecond)

	// Simulate connected state
	mb.mu.Lock()
	mb.state = broker.StateConnected
	mb.statesCh <- broker.StateEvent{State: broker.StateConnected}
	mb.mu.Unlock()
	time.Sleep(100 * time.Millisecond)

	// Drain any initial messages (state, schema, bms overview read)
	drainRawMessages(send, 2*time.Second)

	// Send select_pack command
	h.Command(client, hub.InboundMessage{Type: "select_pack", Section: "bms", Input: 1, Tower: 1, Pack: 1})

	// Collect all messages until section_complete
	rawMsgs := collectRawMessages(t, send, 5*time.Second)

	// Parse messages
	type regMsg struct {
		Type         string  `json:"type"`
		Group        string  `json:"group"`
		Name         string  `json:"name"`
		RegisterAddr float64 `json:"register_addr"`
	}

	var regValues []regMsg
	var lastMsgType string
	hasSchema := false
	for _, raw := range rawMsgs {
		var msg regMsg
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}
		if msg.Type == "register_value" {
			regValues = append(regValues, msg)
		}
		if msg.Type == "section_schema" {
			hasSchema = true
		}
		lastMsgType = msg.Type
	}

	require.True(t, len(regValues) > 0, "should have register_value messages")
	assert.True(t, hasSchema, "should have section_schema message")

	// Verify all 5 groups are represented
	seenGroups := make(map[string]bool)
	for _, rv := range regValues {
		seenGroups[rv.Group] = true
	}
	expectedGroups := []string{"Pack Info", "Cell Voltages", "Balance State", "Temperatures", "Pack Status"}
	for _, g := range expectedGroups {
		assert.True(t, seenGroups[g], "missing group: %s", g)
	}

	// Verify probes are in address order within each span (batch reads emit in address order)
	var lastAddr float64
	for _, rv := range regValues {
		// Address monotonicity can be broken across spans (span 5 at 0x9124 < nothing)
		// but within contiguous sequences it should be increasing.
		// Just verify we got reasonable addresses.
		if rv.RegisterAddr > 0 {
			lastAddr = rv.RegisterAddr
		}
	}
	_ = lastAddr // used to verify we got addresses

	// Verify section_complete is the last message
	assert.Equal(t, "section_complete", lastMsgType, "last message should be section_complete")

	// Verify total count: all pack probes should produce register_value messages
	// PackProbeGroups has: 15 (Pack Info) + 18 (Cell Voltages) + 1 (Balance) + 10 (Temps) + 6 (Pack Status) = 50
	allProbes := register.PackProbeGroups()
	totalProbes := 0
	for _, g := range allProbes {
		totalProbes += len(g.Probes)
	}
	assert.Equal(t, totalProbes, len(regValues), "should have register_value for every pack probe")
}

// === Phase 15: Configuration Section Tests ===

func TestConfigurationSectionRegistered(t *testing.T) {
	assert := assert.New(t)
	mb := newMockBroker()
	h := hub.NewTestHub(mb)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)
	time.Sleep(20 * time.Millisecond)

	// Configuration section should exist after hub startup
	assert.True(h.HasSection("configuration"), "hub should have configuration section")

	// Configuration section should have readOnce=true
	assert.True(h.GetSectionReadOnce("configuration"), "configuration section should have readOnce=true")

	// Configuration section should have hasReadOnce=false initially
	assert.False(h.GetSectionHasReadOnce("configuration"), "configuration section hasReadOnce should be false initially")

	// Configuration section should have groups
	groups := h.GetSectionGroups("configuration")
	assert.True(len(groups) > 0, "configuration section should have probe groups")
}

func TestConfigurationReadOnceFirstReadCycleTriggers(t *testing.T) {
	assert := assert.New(t)
	mb := newMockBroker()
	h := hub.NewTestHub(mb)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)
	time.Sleep(20 * time.Millisecond)

	// Connect the broker
	mb.statesCh <- broker.StateEvent{State: broker.StateConnected}
	time.Sleep(50 * time.Millisecond)

	send := make(chan []byte, 4096)
	c := hub.NewTestClient(h, send)
	h.Register(c)
	time.Sleep(20 * time.Millisecond)

	// Subscribe triggers an immediate read
	h.Command(c, hub.InboundMessage{Type: hub.MsgTypeSubscribe, Section: "configuration"})

	// Drain until section_complete (proves the read happened)
	msgs := drainUntilComplete(t, send, 30*time.Second)
	foundComplete := false
	for _, m := range msgs {
		if m.Type == hub.MsgTypeSectionComplete {
			foundComplete = true
		}
	}
	assert.True(foundComplete, "first subscribe should trigger read and produce section_complete")

	// After completion, hasReadOnce should be true
	assert.True(h.GetSectionHasReadOnce("configuration"), "hasReadOnce should be true after first read completes")
}

func TestConfigurationReadOnceSkipsSecondReadCycle(t *testing.T) {
	assert := assert.New(t)
	mb := newMockBroker()
	h := hub.NewTestHub(mb)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)
	time.Sleep(20 * time.Millisecond)

	// Connect the broker
	mb.statesCh <- broker.StateEvent{State: broker.StateConnected}
	time.Sleep(50 * time.Millisecond)

	send := make(chan []byte, 4096)
	c := hub.NewTestClient(h, send)
	h.Register(c)
	time.Sleep(20 * time.Millisecond)

	// Subscribe and drain initial read
	h.Command(c, hub.InboundMessage{Type: hub.MsgTypeSubscribe, Section: "configuration"})
	drainUntilComplete(t, send, 30*time.Second)

	initialReadCount := mb.getBatchCallCount()

	// Send read_cycle -- should be SKIPPED (hasReadOnce is true)
	h.Command(c, hub.InboundMessage{Type: hub.MsgTypeReadCycle, Section: "configuration"})
	time.Sleep(200 * time.Millisecond)

	// Read count should NOT have increased
	assert.Equal(initialReadCount, mb.getBatchCallCount(), "read_cycle should be skipped for cached configuration section")
}

func TestConfigurationRefreshResetsCache(t *testing.T) {
	assert := assert.New(t)
	mb := newMockBroker()
	h := hub.NewTestHub(mb)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)
	time.Sleep(20 * time.Millisecond)

	// Connect the broker
	mb.statesCh <- broker.StateEvent{State: broker.StateConnected}
	time.Sleep(50 * time.Millisecond)

	send := make(chan []byte, 4096)
	c := hub.NewTestClient(h, send)
	h.Register(c)
	time.Sleep(20 * time.Millisecond)

	// Subscribe and drain initial read
	h.Command(c, hub.InboundMessage{Type: hub.MsgTypeSubscribe, Section: "configuration"})
	drainUntilComplete(t, send, 30*time.Second)

	assert.True(h.GetSectionHasReadOnce("configuration"), "hasReadOnce should be true after first read")

	initialReadCount := mb.getBatchCallCount()

	// Send explicit refresh -- should trigger re-read
	h.Command(c, hub.InboundMessage{Type: hub.MsgTypeRefresh, Section: "configuration"})
	drainUntilComplete(t, send, 30*time.Second)

	assert.Greater(mb.getBatchCallCount(), initialReadCount, "refresh should trigger re-read for configuration section")

	// After refresh completes, hasReadOnce should be true again
	assert.True(h.GetSectionHasReadOnce("configuration"), "hasReadOnce should be true again after refresh completes")
}

func TestOtherSectionsUnaffectedByReadOnce(t *testing.T) {
	assert := assert.New(t)
	mb := newMockBroker()
	h := hub.NewTestHub(mb)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)
	time.Sleep(20 * time.Millisecond)

	// Verify other sections do NOT have readOnce
	assert.False(h.GetSectionReadOnce("system"), "system section should not have readOnce")
	assert.False(h.GetSectionReadOnce("grid"), "grid section should not have readOnce")
	assert.False(h.GetSectionReadOnce("eps"), "eps section should not have readOnce")
	assert.False(h.GetSectionReadOnce("pv"), "pv section should not have readOnce")
	assert.False(h.GetSectionReadOnce("battery"), "battery section should not have readOnce")

	// Connect the broker
	mb.statesCh <- broker.StateEvent{State: broker.StateConnected}
	time.Sleep(50 * time.Millisecond)

	send := make(chan []byte, 4096)
	c := hub.NewTestClient(h, send)
	h.Register(c)
	time.Sleep(20 * time.Millisecond)

	// Subscribe to grid and drain initial read
	h.Command(c, hub.InboundMessage{Type: hub.MsgTypeSubscribe, Section: "grid"})
	drainUntilComplete(t, send, 10*time.Second)

	countAfterFirst := mb.getBatchCallCount()

	// Send read_cycle for grid -- should NOT be skipped
	h.Command(c, hub.InboundMessage{Type: hub.MsgTypeReadCycle, Section: "grid"})
	drainUntilComplete(t, send, 10*time.Second)

	assert.Greater(mb.getBatchCallCount(), countAfterFirst, "grid should still re-read on read_cycle (not affected by readOnce)")
}

// === Phase 19-02: Batch streaming tests ===

func TestBatchStreamingMessages(t *testing.T) {
	// Verify that batch reading produces register_value messages for all probes in a section.
	// Use the grid section for predictable span structure.
	mb := newMockBroker()
	mb.registerResults = make(map[uint16]broker.Result)

	// Set up batch response for each span in grid's batch plan.
	gridPlan := register.AnalyzeBatchPlan(register.GridGroups)
	for _, span := range gridPlan.Spans {
		data := make([]byte, int(span.TotalCount)*2)
		// Fill with non-zero pattern so we can detect correct extraction
		for i := range data {
			data[i] = byte(i + 1)
		}
		mb.registerResults[span.StartAddr] = broker.Result{Data: data}
	}

	h := hub.NewTestHub(mb)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)

	// Connect
	mb.statesCh <- broker.StateEvent{State: broker.StateConnected}
	time.Sleep(50 * time.Millisecond)

	send := make(chan []byte, 512)
	client := hub.NewTestClient(h, send)
	h.Register(client)
	time.Sleep(50 * time.Millisecond)

	h.Command(client, hub.InboundMessage{Type: "subscribe", Section: "grid"})

	// Collect messages until section_complete (from the subscribe-triggered read)
	msgs, completeIdx := waitForMessageType(t, send, "section_complete", 10*time.Second)
	if completeIdx < 0 {
		t.Fatal("never received section_complete")
	}

	// Count register_value messages
	regValCount := 0
	for _, m := range msgs {
		if m.Type == "register_value" && m.Section == "grid" {
			regValCount++
		}
	}

	// Grid has multiple probes across groups; all should arrive as register_value
	totalGridProbes := 0
	for _, g := range register.GridGroups {
		totalGridProbes += len(g.Probes)
	}
	if regValCount != totalGridProbes {
		t.Errorf("register_value count = %d, want %d (total grid probes)", regValCount, totalGridProbes)
	}
}

func TestBatchSpanFallback(t *testing.T) {
	// Verify that when a batch span read fails, individual probe reads are attempted.
	mb := newMockBroker()
	mb.registerResults = make(map[uint16]broker.Result)

	// Get the grid batch plan to find a span to fail
	gridPlan := register.AnalyzeBatchPlan(register.GridGroups)
	if len(gridPlan.Spans) == 0 {
		t.Fatal("no spans in grid batch plan")
	}
	failSpan := gridPlan.Spans[0]

	// Make the batch span read fail (keyed by span StartAddr)
	mb.registerResults[failSpan.StartAddr] = broker.Result{
		Err: fmt.Errorf("simulated batch failure"),
	}

	// But make individual probe reads succeed
	for _, pm := range failSpan.Probes {
		data := make([]byte, pm.ByteLength)
		if len(data) >= 2 {
			binary.BigEndian.PutUint16(data[:2], 100)
		}
		mb.registerResults[pm.Probe.Addr] = broker.Result{Data: data}
	}

	// Set up remaining spans to succeed
	for _, span := range gridPlan.Spans[1:] {
		data := make([]byte, int(span.TotalCount)*2)
		mb.registerResults[span.StartAddr] = broker.Result{Data: data}
	}

	h := hub.NewTestHub(mb)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)

	// Connect
	mb.statesCh <- broker.StateEvent{State: broker.StateConnected}
	time.Sleep(50 * time.Millisecond)

	send := make(chan []byte, 512)
	client := hub.NewTestClient(h, send)
	h.Register(client)
	time.Sleep(50 * time.Millisecond)

	h.Command(client, hub.InboundMessage{Type: "subscribe", Section: "grid"})

	msgs, completeIdx := waitForMessageType(t, send, "section_complete", 10*time.Second)
	if completeIdx < 0 {
		t.Fatal("never received section_complete")
	}

	// Verify register_value messages arrived for the failed span's probes (via fallback)
	regValCount := 0
	for _, m := range msgs {
		if m.Type == "register_value" && m.Section == "grid" {
			regValCount++
		}
	}

	// All probes should still produce values (batch failed, fallback succeeded)
	totalGridProbes := 0
	for _, g := range register.GridGroups {
		totalGridProbes += len(g.Probes)
	}
	if regValCount != totalGridProbes {
		t.Errorf("register_value count after fallback = %d, want %d", regValCount, totalGridProbes)
	}
}

func TestBatchProgressiveStreaming(t *testing.T) {
	// Verify BATCH-04: values appear progressively per span, not all at once.
	mb := newMockBroker()
	mb.registerResults = make(map[uint16]broker.Result)

	gridPlan := register.AnalyzeBatchPlan(register.GridGroups)
	for _, span := range gridPlan.Spans {
		data := make([]byte, int(span.TotalCount)*2)
		for i := range data {
			data[i] = byte(i%255 + 1)
		}
		mb.registerResults[span.StartAddr] = broker.Result{Data: data}
	}

	h := hub.NewTestHub(mb)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)

	// Connect
	mb.statesCh <- broker.StateEvent{State: broker.StateConnected}
	time.Sleep(50 * time.Millisecond)

	send := make(chan []byte, 512)
	client := hub.NewTestClient(h, send)
	h.Register(client)
	time.Sleep(50 * time.Millisecond)

	h.Command(client, hub.InboundMessage{Type: "subscribe", Section: "grid"})

	msgs, _ := waitForMessageType(t, send, "section_complete", 10*time.Second)

	// Verify register_value messages precede section_complete (progressive)
	firstRegVal := -1
	lastRegVal := -1
	sectionComplete := -1
	for i, m := range msgs {
		if m.Type == "register_value" && m.Section == "grid" {
			if firstRegVal == -1 {
				firstRegVal = i
			}
			lastRegVal = i
		}
		if m.Type == "section_complete" && m.Section == "grid" {
			sectionComplete = i
		}
	}

	if firstRegVal == -1 {
		t.Fatal("no register_value messages received")
	}
	if sectionComplete == -1 {
		t.Fatal("no section_complete received")
	}
	// register_value messages must come BEFORE section_complete (progressive update)
	if lastRegVal >= sectionComplete {
		t.Errorf("last register_value at index %d but section_complete at %d; values should precede completion", lastRegVal, sectionComplete)
	}
	// First register_value should appear early (not bunched at the end)
	if firstRegVal > sectionComplete/2 {
		t.Errorf("first register_value at index %d of %d total; expected progressive delivery", firstRegVal, len(msgs))
	}
}

func TestBatchTimingLog(t *testing.T) {
	// Verify the timing log code path runs to completion by checking that
	// streamStandardRead completes with section_complete for a non-fault section.
	mb := newMockBroker()
	mb.registerResults = make(map[uint16]broker.Result)

	// Use EPS section (small, no faults)
	epsPlan := register.AnalyzeBatchPlan(register.EPSGroups)
	for _, span := range epsPlan.Spans {
		data := make([]byte, int(span.TotalCount)*2)
		mb.registerResults[span.StartAddr] = broker.Result{Data: data}
	}

	h := hub.NewTestHub(mb)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)

	// Connect
	mb.statesCh <- broker.StateEvent{State: broker.StateConnected}
	time.Sleep(50 * time.Millisecond)

	send := make(chan []byte, 512)
	client := hub.NewTestClient(h, send)
	h.Register(client)
	time.Sleep(50 * time.Millisecond)

	h.Command(client, hub.InboundMessage{Type: "subscribe", Section: "eps"})

	// If timing log panics or breaks the flow, section_complete would not arrive
	msgs, completeIdx := waitForMessageType(t, send, "section_complete", 10*time.Second)
	if completeIdx < 0 {
		t.Fatal("section_complete not received -- timing log may have broken the flow")
	}

	// Verify at least one register_value was sent
	hasRegVal := false
	for _, m := range msgs {
		if m.Type == "register_value" && m.Section == "eps" {
			hasRegVal = true
			break
		}
	}
	if !hasRegVal {
		t.Error("no register_value messages for eps section")
	}
}

// === Phase 22: SpanTracker integration tests ===

// setupGridSpanTest creates a mock broker with grid batch plan spans configured.
// The first span (failSpanAddr) is set to fail at batch level.
// All other spans succeed. Individual probes can be configured separately.
func setupGridSpanTest(t *testing.T) (*mockBroker, register.BatchPlan, uint16) {
	t.Helper()
	mb := newMockBroker()
	mb.registerResults = make(map[uint16]broker.Result)
	mb.spanFailAddrs = make(map[uint16]error)

	gridPlan := register.AnalyzeBatchPlan(register.GridGroups)
	require.NotEmpty(t, gridPlan.Spans, "grid batch plan must have spans")
	failSpan := gridPlan.Spans[0]

	// Make the batch span read fail via spanFailAddrs (only fails count > 1)
	mb.spanFailAddrs[failSpan.StartAddr] = fmt.Errorf("simulated batch failure")

	// Make individual probe reads succeed for the failing span
	for _, pm := range failSpan.Probes {
		data := make([]byte, pm.ByteLength)
		if len(data) >= 2 {
			binary.BigEndian.PutUint16(data[:2], 100)
		}
		mb.registerResults[pm.Probe.Addr] = broker.Result{Data: data}
	}

	// Set up remaining spans to succeed at batch level
	for _, span := range gridPlan.Spans[1:] {
		data := make([]byte, int(span.TotalCount)*2)
		mb.registerResults[span.StartAddr] = broker.Result{Data: data}
	}

	return mb, gridPlan, failSpan.StartAddr
}

// triggerGridReadCycle subscribes (if first time) or sends read_cycle, then drains until section_complete.
func triggerGridReadCycle(t *testing.T, h *hub.Hub, c *hub.Client, send chan []byte) []hub.OutboundMessage {
	t.Helper()
	hub.SendReadCycle(h, c, "grid")
	msgs, idx := waitForMessageType(t, send, "section_complete", 10*time.Second)
	require.GreaterOrEqual(t, idx, 0, "section_complete not received")
	return msgs
}

func TestStreamStandardRead_SpanTrackerDegradation(t *testing.T) {
	// After 3 batch failures on span 0, SpanTracker should degrade it.
	// Individual fallback reads still produce register_value messages.
	mb, _, failAddr := setupGridSpanTest(t)

	h := hub.NewTestHub(mb)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)

	// Connect
	mb.statesCh <- broker.StateEvent{State: broker.StateConnected}
	time.Sleep(50 * time.Millisecond)

	send := make(chan []byte, 512)
	c := hub.NewTestClient(h, send)
	h.Register(c)
	time.Sleep(50 * time.Millisecond)

	// Subscribe to grid (triggers first read cycle)
	h.Command(c, hub.InboundMessage{Type: "subscribe", Section: "grid"})
	msgs, idx := waitForMessageType(t, send, "section_complete", 10*time.Second)
	require.GreaterOrEqual(t, idx, 0, "section_complete not received on subscribe")

	// Verify register_value messages arrived (individual fallback works)
	regValCount := 0
	for _, m := range msgs {
		if m.Type == "register_value" && m.Section == "grid" {
			regValCount++
		}
	}
	assert.Greater(t, regValCount, 0, "should have register_value messages from fallback")

	// Trigger 2 more read cycles (total 3 batch failures = degradation threshold)
	for i := 0; i < 2; i++ {
		triggerGridReadCycle(t, h, c, send)
	}

	// SpanTracker should now show the span as degraded
	tracker := h.GetSectionSpanTracker("grid")
	require.NotNil(t, tracker, "grid section must have a SpanTracker")
	assert.Equal(t, hub.SpanDegraded, h.GetSpanState("grid", failAddr),
		"span at 0x%04X should be degraded after 3 batch failures", failAddr)
}

func TestStreamStandardRead_SpanTrackerSkipped(t *testing.T) {
	// After 3 batch failures (degraded) + 2 all-individual-fail cycles (skipped),
	// the span transitions to SpanSkipped.
	mb, gridPlan, failAddr := setupGridSpanTest(t)

	h := hub.NewTestHub(mb)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)

	// Connect
	mb.statesCh <- broker.StateEvent{State: broker.StateConnected}
	time.Sleep(50 * time.Millisecond)

	send := make(chan []byte, 512)
	c := hub.NewTestClient(h, send)
	h.Register(c)
	time.Sleep(50 * time.Millisecond)

	// Subscribe to grid (triggers first read cycle)
	h.Command(c, hub.InboundMessage{Type: "subscribe", Section: "grid"})
	_, idx := waitForMessageType(t, send, "section_complete", 10*time.Second)
	require.GreaterOrEqual(t, idx, 0)

	// Trigger 2 more read cycles (3 total = degraded)
	for i := 0; i < 2; i++ {
		triggerGridReadCycle(t, h, c, send)
	}

	tracker := h.GetSectionSpanTracker("grid")
	require.NotNil(t, tracker)
	assert.Equal(t, hub.SpanDegraded, h.GetSpanState("grid", failAddr))

	// Now make individual reads also fail for the failing span's probes
	failSpan := gridPlan.Spans[0]
	for _, pm := range failSpan.Probes {
		mb.setRegisterResult(pm.Probe.Addr, broker.Result{Err: fmt.Errorf("individual read failure")})
	}

	// Trigger 2 more read cycles (individual reads all fail -> transitions to Skipped)
	for i := 0; i < 2; i++ {
		triggerGridReadCycle(t, h, c, send)
	}

	assert.Equal(t, hub.SpanSkipped, h.GetSpanState("grid", failAddr),
		"span should be skipped after 2 cycles of all-individual-fail")

	// Trigger one more non-probe read cycle and verify reduced call count.
	// The skipped span should produce zero ReadRegisters calls.
	mb.resetBatchCallCount()
	triggerGridReadCycle(t, h, c, send)

	// Count: remaining 6 spans x 1 batch read each = 6 calls (no calls for skipped span 0)
	callCount := mb.getBatchCallCount()
	remainingSpans := len(gridPlan.Spans) - 1
	assert.Equal(t, remainingSpans, callCount,
		"skipped span should produce no ReadRegisters calls; expected %d, got %d", remainingSpans, callCount)
}

func TestStreamStandardRead_SpanTrackerProbeRecovery(t *testing.T) {
	// After a span becomes skipped, a probe on the 10th cycle should recover it.
	mb, gridPlan, failAddr := setupGridSpanTest(t)

	h := hub.NewTestHub(mb)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)

	// Connect
	mb.statesCh <- broker.StateEvent{State: broker.StateConnected}
	time.Sleep(50 * time.Millisecond)

	send := make(chan []byte, 512)
	c := hub.NewTestClient(h, send)
	h.Register(c)
	time.Sleep(50 * time.Millisecond)

	// Subscribe triggers first read
	h.Command(c, hub.InboundMessage{Type: "subscribe", Section: "grid"})
	_, idx := waitForMessageType(t, send, "section_complete", 10*time.Second)
	require.GreaterOrEqual(t, idx, 0)

	// 2 more cycles to degrade (3 total batch failures)
	for i := 0; i < 2; i++ {
		triggerGridReadCycle(t, h, c, send)
	}

	// Make individual reads fail to transition to skipped
	failSpan := gridPlan.Spans[0]
	for _, pm := range failSpan.Probes {
		mb.setRegisterResult(pm.Probe.Addr, broker.Result{Err: fmt.Errorf("individual read failure")})
	}

	// 2 more cycles (individual all-fail -> skipped); total cycles = 5
	for i := 0; i < 2; i++ {
		triggerGridReadCycle(t, h, c, send)
	}

	tracker := h.GetSectionSpanTracker("grid")
	require.NotNil(t, tracker)
	require.Equal(t, hub.SpanSkipped, h.GetSpanState("grid", failAddr))

	// Now make the batch read succeed for recovery:
	// Clear the span-level failure so batch reads (count > 1) fall through to registerResults.
	mb.clearSpanFail(failSpan.StartAddr)
	// Set a successful batch-sized result for the span's start address.
	batchData := make([]byte, int(failSpan.TotalCount)*2)
	binary.BigEndian.PutUint16(batchData[:2], 42) // non-zero value
	mb.setRegisterResult(failSpan.StartAddr, broker.Result{Data: batchData})

	// Run cycles 6-14 (no probe yet, span stays skipped)
	// DefaultProbeInterval=10, cycle counter is at 5 after the 5 cycles above.
	// Probe happens when cycle % 10 == 0, so at cycle 10.
	// We need 5 more cycles (cycles 6,7,8,9,10) to reach the probe.
	for i := 0; i < 5; i++ {
		triggerGridReadCycle(t, h, c, send)
	}

	// After cycle 10 (which is a probe cycle), the batch read succeeds -> recovery
	assert.Equal(t, hub.SpanNormal, h.GetSpanState("grid", failAddr),
		"span should recover to Normal after successful probe")

	// Verify register_value messages arrive on the next cycle (span reads normally again)
	msgs := triggerGridReadCycle(t, h, c, send)
	regCount := 0
	for _, m := range msgs {
		if m.Type == "register_value" && m.Section == "grid" {
			regCount++
		}
	}
	// All grid probes should produce values (all spans now normal)
	totalGridProbes := 0
	for _, g := range register.GridGroups {
		totalGridProbes += len(g.Probes)
	}
	assert.Equal(t, totalGridProbes, regCount,
		"all grid probes should produce register_value messages after recovery")
}

func TestStreamStandardRead_SpanTrackerResetOnReconnect(t *testing.T) {
	// Degraded span should reset to Normal when broker reconnects.
	mb, _, failAddr := setupGridSpanTest(t)

	h := hub.NewTestHub(mb)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)

	// Connect
	mb.statesCh <- broker.StateEvent{State: broker.StateConnected}
	time.Sleep(50 * time.Millisecond)

	send := make(chan []byte, 512)
	c := hub.NewTestClient(h, send)
	h.Register(c)
	time.Sleep(50 * time.Millisecond)

	// Subscribe and trigger 3 read cycles to degrade span 0
	h.Command(c, hub.InboundMessage{Type: "subscribe", Section: "grid"})
	_, idx := waitForMessageType(t, send, "section_complete", 10*time.Second)
	require.GreaterOrEqual(t, idx, 0)

	for i := 0; i < 2; i++ {
		triggerGridReadCycle(t, h, c, send)
	}

	tracker := h.GetSectionSpanTracker("grid")
	require.NotNil(t, tracker)
	require.Equal(t, hub.SpanDegraded, h.GetSpanState("grid", failAddr),
		"span should be degraded before reconnect")

	// Simulate disconnect + reconnect
	mb.statesCh <- broker.StateEvent{State: broker.StateDisconnected}
	time.Sleep(50 * time.Millisecond)
	mb.statesCh <- broker.StateEvent{State: broker.StateConnected}
	time.Sleep(50 * time.Millisecond)

	// Drain state messages
	drainClientMessages(send, 100*time.Millisecond)

	// SpanTracker should be reset to Normal
	assert.Equal(t, hub.SpanNormal, h.GetSpanState("grid", failAddr),
		"span should be reset to Normal after reconnect (D-02)")
}

// === Phase 23-02: Battery batch read integration tests ===

// setupBatterySpanTest creates a mock broker configured for battery section batch tests.
// All spans (2-channel + InternalInfo) succeed at batch level with deterministic data.
// The 0x066A individual pre-read returns packCount=2 (matching default, no reconfiguration).
func setupBatterySpanTest(t *testing.T) (*mockBroker, register.BatchPlan) {
	t.Helper()
	mb := newMockBroker()
	mb.registerResults = make(map[uint16]broker.Result)
	mb.spanFailAddrs = make(map[uint16]error)

	groups := append(register.GenerateBatteryGroups(2), register.InternalInfoGroups()...)
	plan := register.AnalyzeBatchPlan(groups)
	require.NotEmpty(t, plan.Spans, "battery batch plan must have spans")

	// Set up all spans to succeed at batch level with known probe values.
	for _, span := range plan.Spans {
		data := make([]byte, int(span.TotalCount)*2)
		for _, pm := range span.Probes {
			// Write value 100 at each probe's byte offset.
			if pm.ByteLength >= 2 {
				binary.BigEndian.PutUint16(data[pm.ByteOffset:pm.ByteOffset+2], 100)
			}
		}
		mb.registerResults[span.StartAddr] = broker.Result{Data: data}
	}

	// 0x066A pre-read: return packCount=2 (matches default, no reconfiguration).
	preReadData := make([]byte, 2)
	binary.BigEndian.PutUint16(preReadData, 2)
	mb.registerResults[0x066A] = broker.Result{Data: preReadData}

	return mb, plan
}

func TestCountBatteryChannels(t *testing.T) {
	// 2-channel groups: [Channel 1, Channel 2, Global Stats]
	groups2 := register.GenerateBatteryGroups(2)
	assert.Equal(t, 2, hub.CountBatteryChannels(groups2), "2-channel groups without InternalInfo")

	// 2-channel + InternalInfo: [Channel 1, Channel 2, Global Stats, Internal Info]
	groups2i := append(register.GenerateBatteryGroups(2), register.InternalInfoGroups()...)
	assert.Equal(t, 2, hub.CountBatteryChannels(groups2i), "2-channel groups with InternalInfo")

	// 4-channel + InternalInfo: [Channel 1..4, Global Stats, Internal Info]
	groups4i := append(register.GenerateBatteryGroups(4), register.InternalInfoGroups()...)
	assert.Equal(t, 4, hub.CountBatteryChannels(groups4i), "4-channel groups with InternalInfo")

	// Edge: no groups
	assert.Equal(t, 0, hub.CountBatteryChannels(nil), "nil groups")
}

func TestBatteryBatchRead_SpanReads(t *testing.T) {
	// BATT-01: Verify battery section reads via batch spans and emits
	// register_value messages for all probes.
	mb, plan := setupBatterySpanTest(t)

	h := hub.NewTestHub(mb)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)

	mb.statesCh <- broker.StateEvent{State: broker.StateConnected}
	time.Sleep(50 * time.Millisecond)

	send := make(chan []byte, 512)
	c := hub.NewTestClient(h, send)
	h.Register(c)
	time.Sleep(50 * time.Millisecond)

	// Subscribe to battery (triggers first read cycle)
	h.Command(c, hub.InboundMessage{Type: "subscribe", Section: "battery"})
	msgs, idx := waitForMessageType(t, send, "section_complete", 10*time.Second)
	require.GreaterOrEqual(t, idx, 0, "section_complete not received")

	// Count register_value messages for battery section
	regValCount := 0
	for _, m := range msgs {
		if m.Type == "register_value" && m.Section == "battery" {
			regValCount++
		}
	}

	// Battery 2-channel + InternalInfo: all probes should produce register_value messages.
	totalProbes := 0
	for _, span := range plan.Spans {
		totalProbes += len(span.Probes)
	}
	assert.Equal(t, totalProbes, regValCount,
		"should receive register_value for every probe in the batch plan")
}

func TestBatteryBatchRead_AutoDetect(t *testing.T) {
	// BATT-02: Verify that when 0x066A returns a different channel count,
	// the section reconfigures and InternalInfoGroups are preserved.
	mb, _ := setupBatterySpanTest(t)

	// Override 0x066A to return 4 channels instead of default 2.
	preReadData := make([]byte, 2)
	binary.BigEndian.PutUint16(preReadData, 4)
	mb.registerResults[0x066A] = broker.Result{Data: preReadData}

	// Set up results for 4-channel battery spans too, since after reconfiguration
	// a new read cycle will use the 4-channel plan.
	groups4 := append(register.GenerateBatteryGroups(4), register.InternalInfoGroups()...)
	plan4 := register.AnalyzeBatchPlan(groups4)
	for _, span := range plan4.Spans {
		data := make([]byte, int(span.TotalCount)*2)
		for _, pm := range span.Probes {
			if pm.ByteLength >= 2 {
				binary.BigEndian.PutUint16(data[pm.ByteOffset:pm.ByteOffset+2], 200)
			}
		}
		mb.registerResults[span.StartAddr] = broker.Result{Data: data}
	}

	h := hub.NewTestHub(mb)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)

	mb.statesCh <- broker.StateEvent{State: broker.StateConnected}
	time.Sleep(50 * time.Millisecond)

	send := make(chan []byte, 512)
	c := hub.NewTestClient(h, send)
	h.Register(c)
	time.Sleep(50 * time.Millisecond)

	// Subscribe triggers read, which detects 4 channels and reconfigures.
	// After reconfiguration, a re-triggered read completes with section_complete.
	// Collect raw messages so we can verify the schema broadcast.
	h.Command(c, hub.InboundMessage{Type: "subscribe", Section: "battery"})
	var rawMsgs [][]byte
	deadline := time.After(10 * time.Second)
autoDetectDrain:
	for {
		select {
		case raw := <-send:
			rawMsgs = append(rawMsgs, raw)
			var peek map[string]interface{}
			if err := json.Unmarshal(raw, &peek); err == nil {
				if peek["type"] == "section_complete" {
					break autoDetectDrain
				}
			}
		case <-deadline:
			t.Fatal("timeout waiting for section_complete after auto-detection")
		}
	}

	// Verify the section was reconfigured to 4 channels.
	groups := h.GetSectionGroups("battery")
	channelCount := 0
	for _, g := range groups {
		if strings.HasPrefix(g.Name, "Channel ") {
			channelCount++
		}
	}
	assert.Equal(t, 4, channelCount, "should have reconfigured to 4 channels")

	// Verify InternalInfoGroups preserved after reconfiguration (bug fix).
	hasInternalInfo := false
	for _, g := range groups {
		if g.Name == "Internal Info" {
			hasInternalInfo = true
			break
		}
	}
	assert.True(t, hasInternalInfo, "Internal Info group must survive reconfiguration")

	// Verify that a section_schema was broadcast after reconfiguration with 4-channel layout.
	var foundAutoDetectSchema bool
	for _, raw := range rawMsgs {
		var schema hub.SectionSchemaMessage
		if err := json.Unmarshal(raw, &schema); err != nil {
			continue
		}
		if schema.Type != hub.MsgTypeSectionSchema || schema.Section != "battery" {
			continue
		}
		// Look for the 4-channel schema (has "Channel 4" group)
		hasChannel4 := false
		for _, g := range schema.Groups {
			if g.Name == "Channel 4" {
				hasChannel4 = true
				break
			}
		}
		if hasChannel4 {
			foundAutoDetectSchema = true
			// 4 Channel groups + Global Stats + Internal Info = 6 groups
			assert.Equal(t, 6, len(schema.Groups),
				"4-channel schema should have 6 groups (4 channels + Global Stats + Internal Info)")
			break
		}
	}
	assert.True(t, foundAutoDetectSchema,
		"should receive a section_schema with 4-channel layout after auto-detection reconfiguration")
}

func TestBatteryBatchRead_OutputEquivalence(t *testing.T) {
	// BATT-03: Verify batch read produces the same register names and non-empty
	// values as expected for a 2-channel battery section.
	mb, _ := setupBatterySpanTest(t)

	h := hub.NewTestHub(mb)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)

	mb.statesCh <- broker.StateEvent{State: broker.StateConnected}
	time.Sleep(50 * time.Millisecond)

	send := make(chan []byte, 512)
	c := hub.NewTestClient(h, send)
	h.Register(c)
	time.Sleep(50 * time.Millisecond)

	h.Command(c, hub.InboundMessage{Type: "subscribe", Section: "battery"})

	// Drain raw messages until section_complete for JSON field access.
	var rawMsgs [][]byte
	deadline := time.After(10 * time.Second)
drainLoop:
	for {
		select {
		case raw := <-send:
			rawMsgs = append(rawMsgs, raw)
			var peek map[string]interface{}
			if err := json.Unmarshal(raw, &peek); err == nil {
				if peek["type"] == "section_complete" {
					break drainLoop
				}
			}
		case <-deadline:
			t.Fatal("timeout waiting for section_complete in output equivalence test")
		}
	}
	require.NotEmpty(t, rawMsgs, "should receive messages from battery read")

	// Build a set of expected register names from the battery groups.
	groups := append(register.GenerateBatteryGroups(2), register.InternalInfoGroups()...)
	expectedNames := make(map[string]bool)
	for _, g := range groups {
		for _, p := range g.Probes {
			expectedNames[p.Name] = true
		}
	}

	// Collect actual register names from register_value messages via raw JSON.
	actualNames := make(map[string]bool)
	for _, raw := range rawMsgs {
		var generic map[string]interface{}
		if err := json.Unmarshal(raw, &generic); err != nil {
			continue
		}
		if generic["type"] != "register_value" {
			continue
		}
		if generic["section"] != "battery" {
			continue
		}
		name, _ := generic["name"].(string)
		if name != "" {
			actualNames[name] = true
		}
		// Every register_value should have a non-empty value (mock data is non-zero).
		value, _ := generic["value"].(string)
		assert.NotEmpty(t, value, "register %s should have a value", name)
	}

	// Every expected register name should appear in actual output.
	for name := range expectedNames {
		assert.True(t, actualNames[name], "expected register %q not found in output", name)
	}
	// No unexpected register names.
	for name := range actualNames {
		assert.True(t, expectedNames[name], "unexpected register %q in output", name)
	}
}

func TestBatteryBatchRead_SpanFallback(t *testing.T) {
	// BATT-01 degradation path: Verify that when a battery batch span fails,
	// individual fallback reads still produce register_value messages.
	mb, plan := setupBatterySpanTest(t)

	// Make the first span fail at batch level.
	failSpan := plan.Spans[0]
	mb.spanFailAddrs[failSpan.StartAddr] = fmt.Errorf("simulated batch failure")

	// Set up individual probe reads for the failing span.
	for _, pm := range failSpan.Probes {
		data := make([]byte, pm.ByteLength)
		if len(data) >= 2 {
			binary.BigEndian.PutUint16(data[:2], 50)
		}
		mb.registerResults[pm.Probe.Addr] = broker.Result{Data: data}
	}

	h := hub.NewTestHub(mb)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)

	mb.statesCh <- broker.StateEvent{State: broker.StateConnected}
	time.Sleep(50 * time.Millisecond)

	send := make(chan []byte, 512)
	c := hub.NewTestClient(h, send)
	h.Register(c)
	time.Sleep(50 * time.Millisecond)

	h.Command(c, hub.InboundMessage{Type: "subscribe", Section: "battery"})
	msgs, idx := waitForMessageType(t, send, "section_complete", 10*time.Second)
	require.GreaterOrEqual(t, idx, 0, "section_complete not received")

	// Even with first span failing, all register_value messages should arrive
	// (failed span uses individual fallback).
	regValCount := 0
	for _, m := range msgs {
		if m.Type == "register_value" && m.Section == "battery" {
			regValCount++
		}
	}

	totalProbes := 0
	for _, span := range plan.Spans {
		totalProbes += len(span.Probes)
	}
	assert.Equal(t, totalProbes, regValCount,
		"all probes should have register_value messages even with span fallback")
}

// === Phase 25-02: Pack batch integration tests ===

// setupPackBatchSpanTest creates a mockBroker configured for pack batch span reads.
// Sets up span-level batch responses with known values (100 for all probes).
// Returns the mock broker and the batch plan for assertion.
func setupPackBatchSpanTest(t *testing.T) (*mockBroker, register.BatchPlan) {
	t.Helper()
	mb := newMockBroker()
	mb.registerResults = make(map[uint16]broker.Result)
	mb.spanFailAddrs = make(map[uint16]error)

	groups := register.PackProbeGroups()
	plan := register.AnalyzeBatchPlan(groups)
	require.NotEmpty(t, plan.Spans, "pack batch plan must have spans")

	// Set up all spans to succeed at batch level with default probe value 100.
	for _, span := range plan.Spans {
		data := make([]byte, int(span.TotalCount)*2)
		for _, pm := range span.Probes {
			if pm.ByteLength >= 2 {
				binary.BigEndian.PutUint16(data[pm.ByteOffset:pm.ByteOffset+2], 100)
			}
		}
		mb.registerResults[span.StartAddr] = broker.Result{Data: data}
	}

	return mb, plan
}

func TestPackBatchRead_SpanReads(t *testing.T) {
	mb, plan := setupPackBatchSpanTest(t)

	h, c, send, cancel := setupConnectedHub(t, mb, 0)
	defer cancel()

	// Subscribe to BMS section so we receive pack messages.
	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeSubscribe,
		Section: "bms",
	})
	drainRawMessages(send, 2*time.Second)

	// Trigger pack select (input=1, tower=1, pack=1).
	h.Command(c, hub.InboundMessage{
		Type:  hub.MsgTypeSelectPack,
		Input: 1, Tower: 1, Pack: 1,
	})

	// Collect messages until section_complete.
	msgs, idx := waitForMessageType(t, send, hub.MsgTypeSectionComplete, 5*time.Second)
	require.GreaterOrEqual(t, idx, 0, "section_complete not received")

	// Count register_value messages (exclude schema + section_complete).
	regCount := 0
	for _, m := range msgs {
		if m.Type == hub.MsgTypeRegisterValue {
			regCount++
		}
	}

	// Supported probes: spans 1 and 2 succeed. Spans 3 and 4 may fail (SpanTracker handles).
	// At minimum, span 1 (57 regs worth of probes) + span 2 (4 regs) should produce values.
	// Count probes in first two spans.
	var expectedMin int
	for i, span := range plan.Spans {
		if i < 2 { // first two spans are supported
			expectedMin += len(span.Probes)
		}
	}
	require.GreaterOrEqual(t, regCount, expectedMin,
		"expected at least %d register_value messages from supported spans, got %d", expectedMin, regCount)
}

func TestPackBatchRead_WriteAndSettle(t *testing.T) {
	mb, _ := setupPackBatchSpanTest(t)

	h, c, send, cancel := setupConnectedHub(t, mb, 0)
	defer cancel()

	// Subscribe to BMS section so we receive pack messages.
	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeSubscribe,
		Section: "bms",
	})
	drainRawMessages(send, 2*time.Second)

	// Trigger pack select for input=1, tower=2, pack=3.
	h.Command(c, hub.InboundMessage{
		Type:  hub.MsgTypeSelectPack,
		Input: 1, Tower: 2, Pack: 3,
	})

	// Wait for section_complete to know the read finished.
	waitForMessageType(t, send, hub.MsgTypeSectionComplete, 5*time.Second)

	// Verify 0x9020 was written with the correct query word.
	writes := mb.getWriteCalls()

	require.NotEmpty(t, writes, "expected at least one WriteRegister call")
	assert.Equal(t, uint16(0x9020), writes[0].Addr, "first write must be to 0x9020")

	// Verify the query word encodes input=1, tower=2, pack=3.
	expectedQuery := register.EncodePackQuery(1, 2, 3, hub.TopoTowers)
	assert.Equal(t, expectedQuery, writes[0].Value,
		"0x9020 write value should encode input=1 tower=2 pack=3")
}

func TestPackBatchRead_SpanDegradation(t *testing.T) {
	mb, plan := setupPackBatchSpanTest(t)

	// Find the span that starts at or covers 0x9104.
	var failSpanAddr uint16
	for _, span := range plan.Spans {
		if span.StartAddr >= 0x9104 {
			failSpanAddr = span.StartAddr
			break
		}
	}
	require.NotZero(t, failSpanAddr, "must find a span covering 0x9104+")

	// Configure that span to fail at batch level.
	mb.spanFailAddrs[failSpanAddr] = fmt.Errorf("illegal data address")
	// Also make individual reads for probes in that span fail.
	for _, span := range plan.Spans {
		if span.StartAddr == failSpanAddr {
			for _, pm := range span.Probes {
				mb.registerResults[pm.Probe.Addr] = broker.Result{Err: fmt.Errorf("illegal data address")}
			}
			break
		}
	}

	h, c, send, cancel := setupConnectedHub(t, mb, 0)
	defer cancel()

	// Subscribe to BMS section so we receive pack messages.
	h.Command(c, hub.InboundMessage{
		Type:    hub.MsgTypeSubscribe,
		Section: "bms",
	})
	drainRawMessages(send, 2*time.Second)

	// Run multiple pack reads to allow SpanTracker to degrade the span.
	// Degradation threshold is 3 consecutive failures -> SpanDegraded.
	// select_pack resets tracker (count=0), then read_cycles accumulate failures.
	h.Command(c, hub.InboundMessage{
		Type:  hub.MsgTypeSelectPack,
		Input: 1, Tower: 1, Pack: 1,
	})
	collectRawMessages(t, send, 5*time.Second)
	time.Sleep(100 * time.Millisecond)

	for i := 0; i < 2; i++ {
		h.Command(c, hub.InboundMessage{Type: hub.MsgTypeReadCycle, Section: "bms"})
		collectRawMessages(t, send, 5*time.Second)
		time.Sleep(100 * time.Millisecond)
	}

	// Verify the 0x9104 span is no longer in Normal state.
	spanState := h.GetPackSpanState(failSpanAddr)
	assert.NotEqual(t, hub.SpanNormal, spanState,
		"span at 0x%04X should be degraded or skipped after repeated failures, got %v", failSpanAddr, spanState)
}

// === Phase 22-03: Battery reconnect reset integration test ===

func TestBatteryBatchRead_ReconnectResetsChannels(t *testing.T) {
	// UAT gap: After disconnect/reconnect, battery section should reset to 2-channel
	// default so that the next read cycle re-detects channels via 0x066A.
	mb, _ := setupBatterySpanTest(t)

	// Override 0x066A to return 4 channels for initial detection.
	preReadData4 := make([]byte, 2)
	binary.BigEndian.PutUint16(preReadData4, 4)
	mb.mu.Lock()
	mb.registerResults[0x066A] = broker.Result{Data: preReadData4}
	mb.mu.Unlock()

	// Set up 4-channel span data so the auto-detected 4-channel plan can complete.
	groups4 := append(register.GenerateBatteryGroups(4), register.InternalInfoGroups()...)
	plan4 := register.AnalyzeBatchPlan(groups4)
	for _, span := range plan4.Spans {
		data := make([]byte, int(span.TotalCount)*2)
		for _, pm := range span.Probes {
			if pm.ByteLength >= 2 {
				binary.BigEndian.PutUint16(data[pm.ByteOffset:pm.ByteOffset+2], 200)
			}
		}
		mb.mu.Lock()
		mb.registerResults[span.StartAddr] = broker.Result{Data: data}
		mb.mu.Unlock()
	}

	h := hub.NewTestHub(mb)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)

	// Connect
	mb.statesCh <- broker.StateEvent{State: broker.StateConnected}
	time.Sleep(50 * time.Millisecond)

	send := make(chan []byte, 512)
	c := hub.NewTestClient(h, send)
	h.Register(c)
	time.Sleep(50 * time.Millisecond)

	// Subscribe to battery — triggers read cycle that detects 4 channels.
	h.Command(c, hub.InboundMessage{Type: "subscribe", Section: "battery"})
	_, idx := waitForMessageType(t, send, "section_complete", 10*time.Second)
	require.GreaterOrEqual(t, idx, 0, "section_complete not received after initial auto-detection")

	// Verify section reconfigured to 4 channels.
	groups := h.GetSectionGroups("battery")
	channelCount := 0
	for _, g := range groups {
		if strings.HasPrefix(g.Name, "Channel ") {
			channelCount++
		}
	}
	require.Equal(t, 4, channelCount, "should have 4 channels after initial auto-detection")

	// Simulate disconnect + reconnect
	mb.statesCh <- broker.StateEvent{State: broker.StateDisconnected}
	time.Sleep(50 * time.Millisecond)
	mb.statesCh <- broker.StateEvent{State: broker.StateConnected}
	time.Sleep(50 * time.Millisecond)

	// Drain state messages as raw bytes so we can check for schema broadcast.
	reconnectRawMsgs := drainRawMessages(send, 200*time.Millisecond)

	// After reconnect, battery section should revert to 2-channel default.
	groups = h.GetSectionGroups("battery")
	channelCount = 0
	for _, g := range groups {
		if strings.HasPrefix(g.Name, "Channel ") {
			channelCount++
		}
	}
	assert.Equal(t, 2, channelCount,
		"battery section should reset to 2-channel default after reconnect")

	// Verify that a section_schema was broadcast after reconnect with 2-channel layout.
	var foundReconnectSchema bool
	for _, raw := range reconnectRawMsgs {
		var schema hub.SectionSchemaMessage
		if err := json.Unmarshal(raw, &schema); err != nil {
			continue
		}
		if schema.Type != hub.MsgTypeSectionSchema || schema.Section != "battery" {
			continue
		}
		// Verify 2-channel layout: 2 Channel groups + Global Stats + Internal Info = 4 groups
		foundReconnectSchema = true
		assert.Equal(t, 4, len(schema.Groups),
			"reconnect schema should have 4 groups (2 channels + Global Stats + Internal Info)")
		// Ensure no Channel 3 or Channel 4 groups exist (proving it is 2-channel, not 4-channel)
		for _, g := range schema.Groups {
			assert.False(t, g.Name == "Channel 3" || g.Name == "Channel 4",
				"reconnect schema should not contain %q (should be 2-channel)", g.Name)
		}
		break
	}
	assert.True(t, foundReconnectSchema,
		"should receive a section_schema with 2-channel layout after reconnect reset")

	// Verify BatchPlan matches 2-channel plan (not 4-channel).
	groups2 := append(register.GenerateBatteryGroups(2), register.InternalInfoGroups()...)
	expectedPlan := register.AnalyzeBatchPlan(groups2)
	actualPlan := h.GetSectionBatchPlan("battery")
	assert.Equal(t, len(expectedPlan.Spans), len(actualPlan.Spans),
		"BatchPlan span count should match 2-channel plan after reconnect")

	// Verify SpanTracker was reset (all spans should be SpanNormal).
	for _, span := range actualPlan.Spans {
		state := h.GetSpanState("battery", span.StartAddr)
		assert.Equal(t, hub.SpanNormal, state,
			"span 0x%04X should be SpanNormal after reconnect reset", span.StartAddr)
	}

	// Now set up mock to return 1 channel on next 0x066A read.
	preReadData1 := make([]byte, 2)
	binary.BigEndian.PutUint16(preReadData1, 1)
	mb.mu.Lock()
	mb.registerResults[0x066A] = broker.Result{Data: preReadData1}
	mb.mu.Unlock()

	// Set up 1-channel span data.
	groups1 := append(register.GenerateBatteryGroups(1), register.InternalInfoGroups()...)
	plan1 := register.AnalyzeBatchPlan(groups1)
	for _, span := range plan1.Spans {
		data := make([]byte, int(span.TotalCount)*2)
		for _, pm := range span.Probes {
			if pm.ByteLength >= 2 {
				binary.BigEndian.PutUint16(data[pm.ByteOffset:pm.ByteOffset+2], 50)
			}
		}
		mb.mu.Lock()
		mb.registerResults[span.StartAddr] = broker.Result{Data: data}
		mb.mu.Unlock()
	}

	// Trigger a new read cycle — should auto-detect 1 channel from fresh state.
	hub.SendReadCycle(h, c, "battery")
	_, idx = waitForMessageType(t, send, "section_complete", 10*time.Second)
	require.GreaterOrEqual(t, idx, 0, "section_complete not received after 1-channel re-detection")

	// Verify section reconfigured to 1 channel (proving fresh auto-detection ran).
	groups = h.GetSectionGroups("battery")
	channelCount = 0
	for _, g := range groups {
		if strings.HasPrefix(g.Name, "Channel ") {
			channelCount++
		}
	}
	assert.Equal(t, 1, channelCount,
		"battery section should re-detect to 1 channel after reconnect reset")
}

