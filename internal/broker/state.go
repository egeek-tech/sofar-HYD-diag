package broker

// State represents the broker's connection state.
type State int

// Broker connection lifecycle states. StateDormant means no host configured;
// the other states track active connection attempts and outcomes.
const (
	StateDormant      State = -1
	StateDisconnected State = 0
	StateConnecting   State = 1
	StateConnected    State = 2
	StateReconnecting State = 3
)

// String returns a human-readable state name.
func (s State) String() string {
	switch s {
	case StateDormant:
		return "dormant"
	case StateDisconnected:
		return "disconnected"
	case StateConnecting:
		return "connecting"
	case StateConnected:
		return "connected"
	case StateReconnecting:
		return "reconnecting"
	default:
		return "unknown"
	}
}

// StateEvent represents a connection state change.
type StateEvent struct {
	State State
	Err   error // non-nil if state change was caused by an error
}
