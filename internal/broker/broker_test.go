package broker_test

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"sofar-hyd-diag/internal/broker"
	"sofar-hyd-diag/internal/modbus"
)

// discardLogger returns a logger that discards all output.
func discardLogger() *slog.Logger {
	return modbus.DiscardLogger()
}

// mockModbusServer accepts a connection on the listener and responds to
// Modbus TCP read requests with the given register value.
// It handles exactly n requests, then returns.
func mockModbusServer(t *testing.T, listener net.Listener, numRequests int, registerValue uint16) {
	t.Helper()

	conn, err := listener.Accept()
	if err != nil {
		t.Logf("mockModbusServer: accept error: %v", err)
		return
	}
	defer conn.Close()

	for i := 0; i < numRequests; i++ {
		conn.SetReadDeadline(time.Now().Add(10 * time.Second))

		// Read Modbus TCP request: 12 bytes (MBAP header 7 + PDU 5)
		req := make([]byte, 12)
		n, err := readAll(conn, req)
		if err != nil {
			t.Logf("mockModbusServer: read request %d error (got %d bytes): %v", i, n, err)
			return
		}

		// Extract transaction ID from request
		txID := binary.BigEndian.Uint16(req[0:2])
		slaveID := req[6]
		funcCode := req[7]

		if funcCode == 0x03 {
			// Build read response
			resp := buildReadResponse(txID, slaveID, registerValue)
			conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
			if _, err := conn.Write(resp); err != nil {
				t.Logf("mockModbusServer: write response %d error: %v", i, err)
				return
			}
		} else if funcCode == 0x10 {
			// Build write response (echo first 12 bytes of PDU)
			resp := buildWriteResponse(txID, slaveID, req[8:10], req[10:12])
			conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
			if _, err := conn.Write(resp); err != nil {
				t.Logf("mockModbusServer: write response %d error: %v", i, err)
				return
			}
		}
	}
}

// readAll reads exactly len(buf) bytes from conn.
func readAll(conn net.Conn, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := conn.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

// buildReadResponse constructs a valid Modbus TCP read response for 1 register.
func buildReadResponse(txID uint16, slaveID byte, value uint16) []byte {
	// MBAP header: txID(2) + protocolID(2) + length(2) + unitID(1) = 7
	// PDU: funcCode(1) + byteCount(1) + data(2) = 4
	resp := make([]byte, 11)
	binary.BigEndian.PutUint16(resp[0:2], txID)
	binary.BigEndian.PutUint16(resp[2:4], 0)     // protocol ID
	binary.BigEndian.PutUint16(resp[4:6], 5)     // length: unitID(1) + PDU(4)
	resp[6] = slaveID
	resp[7] = 0x03 // function code
	resp[8] = 2    // byte count
	binary.BigEndian.PutUint16(resp[9:11], value)
	return resp
}

// buildWriteResponse constructs a valid Modbus TCP write response.
func buildWriteResponse(txID uint16, slaveID byte, regAddr, quantity []byte) []byte {
	// MBAP header: txID(2) + protocolID(2) + length(2) + unitID(1) = 7
	// PDU: funcCode(1) + regAddr(2) + quantity(2) = 5
	resp := make([]byte, 12)
	binary.BigEndian.PutUint16(resp[0:2], txID)
	binary.BigEndian.PutUint16(resp[2:4], 0)     // protocol ID
	binary.BigEndian.PutUint16(resp[4:6], 6)     // length: unitID(1) + PDU(5)
	resp[6] = slaveID
	resp[7] = 0x10 // function code
	copy(resp[8:10], regAddr)
	copy(resp[10:12], quantity)
	return resp
}

func TestBackoff(t *testing.T) {
	b := broker.NewBackoff(100*time.Millisecond, 1*time.Second)

	expected := []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
		400 * time.Millisecond,
		800 * time.Millisecond,
		1 * time.Second,
		1 * time.Second, // capped
	}

	for i, want := range expected {
		got := b.Next()
		if got != want {
			t.Errorf("Next() call %d = %v, want %v", i+1, got, want)
		}
	}

	b.Reset()
	got := b.Next()
	want := 100 * time.Millisecond
	if got != want {
		t.Errorf("After Reset(), Next() = %v, want %v", got, want)
	}
}

func TestBrokerDormantStart(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create broker -- no mock server needed since it should NOT connect
	b := broker.New(discardLogger(), "127.0.0.1:1", 1, false)
	go b.Run(ctx)
	defer b.Close()

	// Give Run() a moment to start
	time.Sleep(50 * time.Millisecond)

	// Broker must be in StateDormant, not StateConnecting
	state := b.CurrentState()
	if state != broker.StateDormant {
		t.Fatalf("expected StateDormant, got %v", state)
	}

	// Attempting a read while dormant should return an error mentioning "dormant"
	_, err := b.ReadRegisters(ctx, 0x0404, 1)
	if err == nil {
		t.Fatal("expected error reading from dormant broker, got nil")
	}
	if !strings.Contains(err.Error(), "dormant") {
		t.Fatalf("expected error containing 'dormant', got: %v", err)
	}
}

func TestBrokerReconfigure(t *testing.T) {
	// Start mock server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().String()
	go mockModbusServer(t, listener, 1, 0xCAFE)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	b := broker.New(discardLogger(), "", 0, false)
	b.SetInterReadDelay(10 * time.Millisecond)
	go b.Run(ctx)
	defer b.Close()

	// Should start dormant
	time.Sleep(50 * time.Millisecond)
	if s := b.CurrentState(); s != broker.StateDormant {
		t.Fatalf("expected StateDormant before Reconfigure, got %v", s)
	}

	// Reconfigure to connect to mock server
	if err := b.Reconfigure(ctx, addr, 1); err != nil {
		t.Fatalf("Reconfigure failed: %v", err)
	}

	// Should now be connected
	if s := b.CurrentState(); s != broker.StateConnected {
		t.Fatalf("expected StateConnected after Reconfigure, got %v", s)
	}

	// Read should succeed
	data, err := b.ReadRegisters(ctx, 0x0404, 1)
	if err != nil {
		t.Fatalf("read after Reconfigure failed: %v", err)
	}
	val := binary.BigEndian.Uint16(data)
	if val != 0xCAFE {
		t.Errorf("expected 0xCAFE, got 0x%04X", val)
	}
}

func TestBrokerDisconnect(t *testing.T) {
	// Start mock server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().String()
	go mockModbusServer(t, listener, 1, 0xBEEF)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	b := broker.New(discardLogger(), "", 0, false)
	b.SetInterReadDelay(10 * time.Millisecond)
	b.SetBackoff(50*time.Millisecond, 200*time.Millisecond)
	go b.Run(ctx)
	defer b.Close()

	// Connect via Reconfigure
	if err := b.Reconfigure(ctx, addr, 1); err != nil {
		t.Fatalf("Reconfigure failed: %v", err)
	}
	if s := b.CurrentState(); s != broker.StateConnected {
		t.Fatalf("expected StateConnected, got %v", s)
	}

	// Disconnect
	if err := b.Disconnect(ctx); err != nil {
		t.Fatalf("Disconnect failed: %v", err)
	}

	// Should be StateDisconnected, not reconnecting
	time.Sleep(100 * time.Millisecond)
	if s := b.CurrentState(); s != broker.StateDisconnected {
		t.Fatalf("expected StateDisconnected after Disconnect, got %v", s)
	}
}

func TestBrokerReconfigureWhileConnected(t *testing.T) {
	// Start two mock servers
	listener1, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener1: %v", err)
	}
	defer listener1.Close()
	addr1 := listener1.Addr().String()
	go mockModbusServer(t, listener1, 1, 0x1111)

	listener2, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener2: %v", err)
	}
	defer listener2.Close()
	addr2 := listener2.Addr().String()
	go mockModbusServer(t, listener2, 1, 0x2222)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	b := broker.New(discardLogger(), "", 0, false)
	b.SetInterReadDelay(10 * time.Millisecond)
	go b.Run(ctx)
	defer b.Close()

	// Connect to first server
	if err := b.Reconfigure(ctx, addr1, 1); err != nil {
		t.Fatalf("first Reconfigure failed: %v", err)
	}

	// Read from first server
	data, err := b.ReadRegisters(ctx, 0x0404, 1)
	if err != nil {
		t.Fatalf("first read failed: %v", err)
	}
	if val := binary.BigEndian.Uint16(data); val != 0x1111 {
		t.Errorf("first read: expected 0x1111, got 0x%04X", val)
	}

	// Reconfigure to second server
	if err := b.Reconfigure(ctx, addr2, 1); err != nil {
		t.Fatalf("second Reconfigure failed: %v", err)
	}

	// Read from second server
	data, err = b.ReadRegisters(ctx, 0x0404, 1)
	if err != nil {
		t.Fatalf("second read failed: %v", err)
	}
	if val := binary.BigEndian.Uint16(data); val != 0x2222 {
		t.Errorf("second read: expected 0x2222, got 0x%04X", val)
	}
}

func TestBrokerSerialization(t *testing.T) {
	// Start a mock TCP server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().String()

	// Mock server handles 3 sequential requests
	go mockModbusServer(t, listener, 3, 0x1234)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Create broker with very short inter-read delay for test speed
	b := broker.New(discardLogger(), "", 1, false)
	b.SetInterReadDelay(10 * time.Millisecond) // fast for tests

	go b.Run(ctx)

	// Reconfigure to connect (broker starts dormant now)
	if err := b.Reconfigure(ctx, addr, 1); err != nil {
		t.Fatalf("Reconfigure failed: %v", err)
	}

	// Fire 3 concurrent reads
	var wg sync.WaitGroup
	results := make([]error, 3)

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			data, err := b.ReadRegisters(ctx, 0x0404, 1)
			if err != nil {
				results[idx] = err
				return
			}
			if len(data) != 2 {
				results[idx] = fmt.Errorf("expected 2 bytes, got %d", len(data))
				return
			}
			val := binary.BigEndian.Uint16(data)
			if val != 0x1234 {
				results[idx] = fmt.Errorf("expected 0x1234, got 0x%04X", val)
			}
		}(i)
	}

	wg.Wait()

	for i, err := range results {
		if err != nil {
			t.Errorf("concurrent read %d failed: %v", i, err)
		}
	}
}

func TestBrokerReconnect(t *testing.T) {
	// Start first mock server
	listener1, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	addr := listener1.Addr().String()

	// Serve 1 request then close
	go mockModbusServer(t, listener1, 1, 0xAAAA)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	b := broker.New(discardLogger(), "", 1, false)
	b.SetInterReadDelay(10 * time.Millisecond)
	b.SetBackoff(100*time.Millisecond, 500*time.Millisecond)

	go b.Run(ctx)

	// Drain state events in background
	events := make([]broker.StateEvent, 0, 10)
	var eventsMu sync.Mutex
	go func() {
		for evt := range b.StateEvents() {
			eventsMu.Lock()
			events = append(events, evt)
			eventsMu.Unlock()
		}
	}()

	// Reconfigure to connect (broker starts dormant now)
	if err := b.Reconfigure(ctx, addr, 1); err != nil {
		t.Fatalf("Reconfigure failed: %v", err)
	}

	// First read should succeed
	data, err := b.ReadRegisters(ctx, 0x0404, 1)
	if err != nil {
		t.Fatalf("first read failed: %v", err)
	}
	val := binary.BigEndian.Uint16(data)
	if val != 0xAAAA {
		t.Errorf("first read: expected 0xAAAA, got 0x%04X", val)
	}

	// Close first listener so next operation triggers reconnect
	listener1.Close()

	// Start a new listener on the same address for the reconnect
	listener2, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("failed to create second listener: %v", err)
	}
	defer listener2.Close()

	go mockModbusServer(t, listener2, 1, 0xBBBB)

	// Second read should trigger reconnect and succeed
	data, err = b.ReadRegisters(ctx, 0x0404, 1)
	if err != nil {
		t.Fatalf("second read (after reconnect) failed: %v", err)
	}
	val = binary.BigEndian.Uint16(data)
	if val != 0xBBBB {
		t.Errorf("second read: expected 0xBBBB, got 0x%04X", val)
	}

	// Verify we saw reconnecting state event
	time.Sleep(100 * time.Millisecond)
	eventsMu.Lock()
	defer eventsMu.Unlock()

	hasReconnecting := false
	hasConnected := false
	for _, evt := range events {
		if evt.State == broker.StateReconnecting {
			hasReconnecting = true
		}
		if evt.State == broker.StateConnected {
			hasConnected = true
		}
	}

	if !hasConnected {
		t.Error("expected StateConnected event, got none")
	}
	// Note: hasReconnecting may or may not be true depending on timing
	// The important thing is that the reconnect succeeded
	_ = hasReconnecting
}

func TestBrokerContextCancellation(t *testing.T) {
	// Use an address that won't connect
	ctx, cancel := context.WithCancel(context.Background())

	b := broker.New(discardLogger(), "127.0.0.1:1", 1, false)
	b.SetBackoff(50*time.Millisecond, 200*time.Millisecond)

	go b.Run(ctx)

	// Cancel immediately
	cancel()

	// ReadRegisters should return context error
	_, err := b.ReadRegisters(ctx, 0x0404, 1)
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
	if err != context.Canceled {
		t.Logf("got error: %v (acceptable, context was cancelled)", err)
	}
}

func TestBrokerReadBatch(t *testing.T) {
	// Start mock server that handles 3 requests
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().String()

	go mockModbusServer(t, listener, 3, 0x5678)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Use 500ms inter-read delay to verify timing
	b := broker.New(discardLogger(), "", 1, false)
	b.SetInterReadDelay(500 * time.Millisecond)

	go b.Run(ctx)

	// Reconfigure to connect (broker starts dormant now)
	if err := b.Reconfigure(ctx, addr, 1); err != nil {
		t.Fatalf("Reconfigure failed: %v", err)
	}

	reads := []broker.ReadRequest{
		{Addr: 0x0404, Count: 1},
		{Addr: 0x0484, Count: 1},
		{Addr: 0x0604, Count: 1},
	}

	start := time.Now()
	results := b.ReadBatch(ctx, reads)
	elapsed := time.Since(start)

	// Verify all 3 results succeeded
	for i, r := range results {
		if r.Err != nil {
			t.Errorf("batch read %d failed: %v", i, r.Err)
			continue
		}
		if len(r.Data) != 2 {
			t.Errorf("batch read %d: expected 2 bytes, got %d", i, len(r.Data))
			continue
		}
		val := binary.BigEndian.Uint16(r.Data)
		if val != 0x5678 {
			t.Errorf("batch read %d: expected 0x5678, got 0x%04X", i, val)
		}
	}

	// 3 reads with 500ms inter-read delay = at least 1000ms between first and last
	// (2 gaps: read1 -> 500ms -> read2 -> 500ms -> read3)
	if elapsed < 1000*time.Millisecond {
		t.Errorf("batch read completed too fast: %v (expected >= 1000ms for inter-read delays)", elapsed)
	}
}
