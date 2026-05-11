// Package broker serializes all Modbus operations through a single goroutine,
// owning the TCP connection, handling auto-reconnection with exponential
// backoff, and emitting connection state change events.
package broker

import "time"

// Backoff implements exponential backoff with a cap.
type Backoff struct {
	base    time.Duration
	max     time.Duration
	current time.Duration
}

// NewBackoff creates a backoff starting at base, capping at maxDelay.
func NewBackoff(base, maxDelay time.Duration) *Backoff {
	return &Backoff{base: base, max: maxDelay, current: base}
}

// Next returns the current delay and doubles it for next call (capped at max).
func (b *Backoff) Next() time.Duration {
	d := b.current
	b.current *= 2
	if b.current > b.max {
		b.current = b.max
	}
	return d
}

// Reset resets the backoff to the base duration.
func (b *Backoff) Reset() {
	b.current = b.base
}
