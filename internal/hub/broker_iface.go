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
	// CurrentState returns the broker's current connection state.
	CurrentState() broker.State
	// StateEvents returns a channel emitting connection state changes.
	StateEvents() <-chan broker.StateEvent
}
