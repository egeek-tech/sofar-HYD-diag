package broker

// State represents the broker's connection state.
type State int

const (
	StateDisconnected State = iota
	StateConnecting
	StateConnected
	StateReconnecting
)

// String returns a human-readable state name.
func (s State) String() string {
	switch s {
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
