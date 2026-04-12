package broker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"sofar-hyd-diag/internal/modbus"
)

// ErrBrokerClosed is returned when an operation is attempted on a closed broker.
var ErrBrokerClosed = errors.New("broker closed")

// CmdType represents the type of command sent to the broker.
type CmdType int

const (
	CmdRead CmdType = iota
	CmdWrite
	CmdReadBatch
	CmdReconfigure
	CmdDisconnect
	CmdSetDelay
)

// ReconfigureRequest carries the new address and slave ID for runtime reconfiguration.
type ReconfigureRequest struct {
	Addr    string
	SlaveID byte
}

// ReadRequest describes a register read operation.
type ReadRequest struct {
	Addr  uint16
	Count uint16
}

// WriteRequest describes a register write operation.
type WriteRequest struct {
	Addr  uint16
	Value uint16
}

// BatchReadRequest describes multiple register read operations.
type BatchReadRequest struct {
	Reads []ReadRequest
}

// SetDelayRequest carries the new inter-read delay for runtime update.
type SetDelayRequest struct {
	InterReadDelay time.Duration
}

// Result contains the outcome of a single register operation.
type Result struct {
	Data []byte
	Err  error
}

// BatchResult contains the outcomes of multiple register operations.
type BatchResult struct {
	Results []Result
}

// command is an internal message sent through the broker's command channel.
type command struct {
	cmdType  CmdType
	request  interface{}
	response chan<- interface{}
}

// Broker serializes all Modbus operations through a single goroutine.
// It owns the TCP connection, handles auto-reconnection with exponential backoff,
// and emits connection state change events.
type Broker struct {
	commands       chan command
	done           chan struct{}
	logger         *slog.Logger
	addr           string
	slaveID        byte
	useRTU         bool
	conn           net.Conn
	connMu         sync.Mutex // protects conn for abortRead() from outside Run()
	aborting       atomic.Bool // set by abortRead(), checked by executeRead retry
	state          atomic.Int32
	stateCh        chan StateEvent
	backoff        *Backoff
	interReadDelay time.Duration
	lastReadTime   time.Time
	dormant        bool
}

// New creates a new Broker. Call Run() in a goroutine to start processing commands.
func New(logger *slog.Logger, addr string, slaveID byte, useRTU bool) *Broker {
	b := &Broker{
		commands:       make(chan command, 32),
		done:           make(chan struct{}),
		logger:         logger,
		addr:           addr,
		slaveID:        slaveID,
		useRTU:         useRTU,
		stateCh:        make(chan StateEvent, 16),
		backoff:        NewBackoff(1*time.Second, 30*time.Second),
		interReadDelay: 500 * time.Millisecond,
		dormant:        true,
	}
	b.state.Store(int32(StateDormant))
	return b
}

// SetInterReadDelay overrides the default 500ms inter-read delay.
// Must be called before Run().
func (b *Broker) SetInterReadDelay(d time.Duration) {
	b.interReadDelay = d
}

// SetBackoff overrides the default backoff parameters.
// Must be called before Run().
func (b *Broker) SetBackoff(base, max time.Duration) {
	b.backoff = NewBackoff(base, max)
}

// Run starts the broker's command processing loop. It blocks until ctx is cancelled.
// The broker starts in dormant state -- call Reconfigure() to initiate a connection.
func (b *Broker) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			b.cleanup()
			return
		case <-b.done:
			b.cleanup()
			return
		case cmd := <-b.commands:
			b.execute(ctx, cmd)
		}
	}
}

// ReadRegisters reads holding registers from the inverter.
// Safe for concurrent callers -- operations are serialized through the command channel.
func (b *Broker) ReadRegisters(ctx context.Context, addr uint16, count uint16) ([]byte, error) {
	respCh := make(chan interface{}, 1)
	select {
	case b.commands <- command{
		cmdType:  CmdRead,
		request:  ReadRequest{Addr: addr, Count: count},
		response: respCh,
	}:
	case <-b.done:
		return nil, ErrBrokerClosed
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	select {
	case res := <-respCh:
		r := res.(Result)
		return r.Data, r.Err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// WriteRegister writes a value to a single register on the inverter.
// Safe for concurrent callers.
func (b *Broker) WriteRegister(ctx context.Context, addr uint16, value uint16) error {
	respCh := make(chan interface{}, 1)
	select {
	case b.commands <- command{
		cmdType:  CmdWrite,
		request:  WriteRequest{Addr: addr, Value: value},
		response: respCh,
	}:
	case <-b.done:
		return ErrBrokerClosed
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case res := <-respCh:
		r := res.(Result)
		return r.Err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ReadBatch reads multiple register groups in a single queued operation.
// Inter-read delays are enforced between each read.
func (b *Broker) ReadBatch(ctx context.Context, reads []ReadRequest) []Result {
	respCh := make(chan interface{}, 1)
	select {
	case b.commands <- command{
		cmdType:  CmdReadBatch,
		request:  BatchReadRequest{Reads: reads},
		response: respCh,
	}:
	case <-b.done:
		results := make([]Result, len(reads))
		for i := range results {
			results[i] = Result{Err: ErrBrokerClosed}
		}
		return results
	case <-ctx.Done():
		results := make([]Result, len(reads))
		for i := range results {
			results[i] = Result{Err: ctx.Err()}
		}
		return results
	}
	select {
	case res := <-respCh:
		r := res.(BatchResult)
		return r.Results
	case <-ctx.Done():
		results := make([]Result, len(reads))
		for i := range results {
			results[i] = Result{Err: ctx.Err()}
		}
		return results
	}
}

// CurrentState returns the broker's current connection state.
// Safe for concurrent use from any goroutine.
func (b *Broker) CurrentState() State {
	return State(b.state.Load())
}

// StateEvents returns a read-only channel for connection state change events.
func (b *Broker) StateEvents() <-chan StateEvent {
	return b.stateCh
}

// Address returns the configured inverter address.
func (b *Broker) Address() string {
	return b.addr
}

// Close shuts down the broker. Safe to call from any goroutine.
// Signals the Run loop and all pending callers to return.
func (b *Broker) Close() {
	close(b.done)
}

// abortRead forces any in-progress socket read to fail immediately
// by setting the read deadline to now. Safe to call from any goroutine.
// Must be non-blocking to avoid deadlock with the Run() loop.
func (b *Broker) abortRead() {
	b.aborting.Store(true)
	b.connMu.Lock()
	defer b.connMu.Unlock()
	if b.conn != nil {
		b.conn.SetReadDeadline(time.Now())
	}
}

// Reconfigure closes any existing connection and connects to a new address with a new slave ID.
// The operation is serialized through the command channel.
func (b *Broker) Reconfigure(ctx context.Context, addr string, slaveID byte) error {
	respCh := make(chan interface{}, 1)
	select {
	case b.commands <- command{
		cmdType:  CmdReconfigure,
		request:  ReconfigureRequest{Addr: addr, SlaveID: slaveID},
		response: respCh,
	}:
	case <-b.done:
		return ErrBrokerClosed
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case resp := <-respCh:
		if err, ok := resp.(error); ok {
			return err
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Disconnect closes the current connection without triggering auto-reconnect.
// The broker returns to a disconnected state and will not reconnect until Reconfigure is called.
func (b *Broker) Disconnect(ctx context.Context) error {
	b.abortRead() // unblock any pending read FIRST (D-02)

	respCh := make(chan interface{}, 1)
	select {
	case b.commands <- command{
		cmdType:  CmdDisconnect,
		request:  nil,
		response: respCh,
	}:
	case <-b.done:
		return ErrBrokerClosed
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case <-respCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// SetDelayRuntime updates the inter-read delay at runtime via the command channel.
// Safe for concurrent callers -- the update is serialized through the Run() goroutine
// to avoid data races on the interReadDelay field.
func (b *Broker) SetDelayRuntime(ctx context.Context, d time.Duration) error {
	respCh := make(chan interface{}, 1)
	select {
	case b.commands <- command{
		cmdType:  CmdSetDelay,
		request:  SetDelayRequest{InterReadDelay: d},
		response: respCh,
	}:
	case <-b.done:
		return ErrBrokerClosed
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case <-respCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// setState updates the broker's connection state and emits a state event.
func (b *Broker) setState(s State, err error) {
	b.state.Store(int32(s))
	select {
	case b.stateCh <- StateEvent{State: s, Err: err}:
	default:
		// Don't block if nobody is listening
	}
	if err != nil {
		b.logger.Info("connection state changed", "state", s.String(), "error", err)
	} else {
		b.logger.Info("connection state changed", "state", s.String())
	}
}

// execute dispatches a command to the appropriate handler.
func (b *Broker) execute(ctx context.Context, cmd command) {
	switch cmd.cmdType {
	case CmdRead:
		req := cmd.request.(ReadRequest)
		result := b.executeRead(ctx, req)
		cmd.response <- result
	case CmdWrite:
		req := cmd.request.(WriteRequest)
		result := b.executeWrite(ctx, req)
		cmd.response <- result
	case CmdReadBatch:
		req := cmd.request.(BatchReadRequest)
		result := b.executeBatch(ctx, req)
		cmd.response <- result
	case CmdReconfigure:
		req := cmd.request.(ReconfigureRequest)
		b.executeReconfigure(ctx, req)
		cmd.response <- Result{}
	case CmdDisconnect:
		b.executeDisconnect()
		cmd.response <- Result{}
	case CmdSetDelay:
		req := cmd.request.(SetDelayRequest)
		b.interReadDelay = req.InterReadDelay
		b.logger.Info("inter-read delay updated", "delay", req.InterReadDelay)
		cmd.response <- Result{}
	}
}

// executeRead performs a single register read with inter-read delay enforcement.
// On communication error, it closes the connection, reconnects, and retries once
// (matching the original readWithRetry pattern from main.go.bak).
func (b *Broker) executeRead(ctx context.Context, req ReadRequest) Result {
	const maxAttempts = 2

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := b.ensureConnected(ctx); err != nil {
			return Result{Err: err}
		}

		b.enforceInterReadDelay()

		var data []byte
		var err error
		if b.useRTU {
			data, err = modbus.ReadHoldingRegistersRTU(b.conn, b.logger, b.slaveID, req.Addr, req.Count)
		} else {
			data, err = modbus.ReadHoldingRegistersTCP(b.conn, b.logger, b.slaveID, req.Addr, req.Count)
		}

		b.lastReadTime = time.Now()

		if err == nil {
			return Result{Data: data}
		}

		b.handleError(err)

		// If abortRead() was called (disconnect in progress), skip retry
		if b.aborting.Load() {
			return Result{Err: err}
		}

		if attempt == maxAttempts {
			return Result{Err: err}
		}

		b.logger.Debug("retrying read after error", "addr", fmt.Sprintf("0x%04X", req.Addr), "attempt", attempt)
	}

	// Unreachable, but satisfies compiler
	return Result{Err: fmt.Errorf("exhausted retry attempts")}
}

// executeWrite performs a single register write with one retry on failure.
func (b *Broker) executeWrite(ctx context.Context, req WriteRequest) Result {
	const maxAttempts = 2

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := b.ensureConnected(ctx); err != nil {
			return Result{Err: err}
		}

		var err error
		if b.useRTU {
			err = modbus.WriteSingleRegisterRTU(b.conn, b.logger, b.slaveID, req.Addr, req.Value)
		} else {
			err = modbus.WriteMultipleRegistersTCP(b.conn, b.logger, b.slaveID, req.Addr, req.Value)
		}

		if err == nil {
			return Result{}
		}

		b.handleError(err)

		if attempt == maxAttempts {
			return Result{Err: err}
		}

		b.logger.Debug("retrying write after error", "addr", fmt.Sprintf("0x%04X", req.Addr), "attempt", attempt)
	}

	return Result{Err: fmt.Errorf("exhausted retry attempts")}
}

// executeBatch performs multiple reads with inter-read delays between each.
func (b *Broker) executeBatch(ctx context.Context, req BatchReadRequest) BatchResult {
	results := make([]Result, len(req.Reads))
	for i, read := range req.Reads {
		if ctx.Err() != nil {
			for j := i; j < len(req.Reads); j++ {
				results[j] = Result{Err: ctx.Err()}
			}
			break
		}
		results[i] = b.executeRead(ctx, read)
	}
	return BatchResult{Results: results}
}

// executeReconfigure closes any existing connection and connects to a new address.
// Uses a single dial attempt (5s timeout) to avoid blocking the command loop.
// If connection fails, the broker returns to StateDisconnected so the user can retry.
func (b *Broker) executeReconfigure(_ context.Context, req ReconfigureRequest) {
	// Close existing connection if any
	b.connMu.Lock()
	if b.conn != nil {
		b.conn.Close()
		b.conn = nil
	}
	b.connMu.Unlock()
	b.addr = req.Addr
	b.slaveID = req.SlaveID
	b.dormant = false
	b.backoff.Reset()
	b.logger.Info("broker reconfigured", "addr", req.Addr, "slaveID", req.SlaveID)

	// Single dial attempt — don't use ensureConnected (infinite retry loop would block command channel)
	b.setState(StateConnecting, nil)
	conn, err := modbus.Connect(b.addr)
	if err != nil {
		b.setState(StateDisconnected, err)
		b.logger.Error("connection failed", "addr", b.addr, "error", err)
		return
	}
	b.connMu.Lock()
	b.conn = conn
	b.connMu.Unlock()
	b.setState(StateConnected, nil)
}

// executeDisconnect closes the connection and enters dormant-like disconnected state.
// No auto-reconnect will occur until Reconfigure is called again.
func (b *Broker) executeDisconnect() {
	b.aborting.Store(false)
	b.connMu.Lock()
	if b.conn != nil {
		b.conn.Close()
		b.conn = nil
	}
	b.connMu.Unlock()
	b.dormant = true
	b.setState(StateDisconnected, nil)
	b.logger.Info("broker disconnected by request")
}

// enforceInterReadDelay sleeps if needed to maintain the minimum delay between reads.
func (b *Broker) enforceInterReadDelay() {
	elapsed := time.Since(b.lastReadTime)
	if elapsed < b.interReadDelay {
		time.Sleep(b.interReadDelay - elapsed)
	}
}

// ensureConnected establishes a connection if one doesn't exist.
// Returns an error if the broker is dormant (no address configured).
// On failure, enters reconnection loop with exponential backoff.
func (b *Broker) ensureConnected(ctx context.Context) error {
	if b.conn != nil {
		return nil
	}

	if b.dormant {
		return fmt.Errorf("broker is dormant -- call Reconfigure to connect")
	}

	b.setState(StateConnecting, nil)

	for {
		conn, err := modbus.Connect(b.addr)
		if err == nil {
			b.conn = conn
			b.backoff.Reset()
			b.setState(StateConnected, nil)
			return nil
		}

		b.setState(StateReconnecting, err)

		delay := b.backoff.Next()
		b.logger.Debug("reconnect backoff", "delay", delay, "error", err)

		select {
		case <-ctx.Done():
			b.setState(StateDisconnected, ctx.Err())
			return ctx.Err()
		case <-time.After(delay):
			// Try again
		}
	}
}

// handleError closes the connection and updates state on communication errors.
// If the broker is dormant, it transitions to StateDisconnected instead of StateReconnecting
// to prevent auto-reconnection after an explicit Disconnect.
func (b *Broker) handleError(err error) {
	b.logger.Error("modbus operation failed", "error", err)
	b.connMu.Lock()
	if b.conn != nil {
		b.conn.Close()
		b.conn = nil
	}
	b.connMu.Unlock()
	if b.dormant {
		b.setState(StateDisconnected, err)
	} else {
		b.setState(StateReconnecting, err)
	}
}

// cleanup closes resources when the broker stops.
func (b *Broker) cleanup() {
	b.connMu.Lock()
	if b.conn != nil {
		b.conn.Close()
		b.conn = nil
	}
	b.connMu.Unlock()
	b.setState(StateDisconnected, nil)
	close(b.stateCh) // signal consumers the broker is done
}
