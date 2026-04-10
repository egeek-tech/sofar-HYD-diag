package hub

import (
	"context"

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
	// WriteRegister writes a single 16-bit value to a Modbus register.
	WriteRegister(ctx context.Context, addr uint16, value uint16) error
	// CurrentState returns the broker's current connection state.
	CurrentState() broker.State
	// StateEvents returns a channel emitting connection state changes.
	StateEvents() <-chan broker.StateEvent
}
