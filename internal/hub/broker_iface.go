package hub

import (
	"context"
	"time"

	"sofar-hyd-diag/internal/broker"
)

// BrokerInterface abstracts the Modbus broker for hub testing.
// The real broker.Broker satisfies this interface.
type BrokerInterface interface {
	// Reconfigure closes current connection and connects to new address.
	Reconfigure(ctx context.Context, addr string, slaveID byte) error
	// Disconnect closes connection without auto-reconnect.
	Disconnect(ctx context.Context) error
	// ReadBatch reads multiple register groups in one serialized operation.
	ReadBatch(ctx context.Context, reads []broker.ReadRequest) []broker.Result
	// ReadRegisters reads a single register group (used for per-register streaming).
	ReadRegisters(ctx context.Context, addr uint16, count uint16) ([]byte, error)
	// WriteRegister writes a single 16-bit value to a Modbus register.
	WriteRegister(ctx context.Context, addr uint16, value uint16) error
	// SetDelayRuntime updates the inter-read delay at runtime via the command channel.
	SetDelayRuntime(ctx context.Context, d time.Duration) error
	// CurrentState returns the broker's current connection state.
	CurrentState() broker.State
	// StateEvents returns a channel emitting connection state changes.
	StateEvents() <-chan broker.StateEvent
}
