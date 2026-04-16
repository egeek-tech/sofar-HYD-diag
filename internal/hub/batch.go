package hub

import (
	"fmt"
	"log/slog"
)

// SpanState represents the degradation level of a batch span.
type SpanState int

const (
	SpanNormal   SpanState = iota // batch reads as usual
	SpanDegraded                  // skip batch, individual reads only
	SpanSkipped                   // no reads at all
)

// String returns a human-readable name for the state.
func (s SpanState) String() string {
	switch s {
	case SpanNormal:
		return "normal"
	case SpanDegraded:
		return "degraded"
	case SpanSkipped:
		return "skipped"
	default:
		return "unknown"
	}
}

// DefaultDegradationThreshold is the number of consecutive batch failures
// before a span degrades to individual reads (D-04).
const DefaultDegradationThreshold = 3

// DefaultProbeInterval is the number of read cycles between probe attempts
// for degraded or skipped spans (D-07).
const DefaultProbeInterval = 10

// SpanTracker tracks consecutive batch failures per span for graceful degradation.
// Spans transition through three states:
//   - SpanNormal: batch reads as usual
//   - SpanDegraded: batch read failed repeatedly, use individual register reads
//   - SpanSkipped: individual reads also failed, skip entirely
//
// A single success (RecordSuccess) resets any span back to SpanNormal (D-08).
// Degraded and skipped spans are periodically probed (every probeInterval cycles)
// for recovery (D-07).
//
// SpanTracker is NOT goroutine-safe. It is designed to be used from a single
// goroutine (the hub's streaming goroutine), consistent with Section which also
// lacks a mutex.
type SpanTracker struct {
	failCounts    map[uint16]int
	states        map[uint16]SpanState
	indivFails    map[uint16]int
	threshold     int
	cycle         int
	probeInterval int
	logger        *slog.Logger
}

// NewSpanTracker creates a SpanTracker with the given degradation threshold.
func NewSpanTracker(threshold int, logger *slog.Logger) *SpanTracker {
	return &SpanTracker{
		failCounts:    make(map[uint16]int),
		states:        make(map[uint16]SpanState),
		indivFails:    make(map[uint16]int),
		threshold:     threshold,
		probeInterval: DefaultProbeInterval,
		logger:        logger,
	}
}

// RecordSuccess resets the failure counter for a span and clears degradation.
// If the span was degraded or skipped, logs a recovery event and resets to
// SpanNormal (D-08: full reset, not back one level).
func (st *SpanTracker) RecordSuccess(startAddr uint16) {
	if st.failCounts[startAddr] > 0 || st.states[startAddr] != SpanNormal {
		if st.states[startAddr] != SpanNormal {
			st.logger.Info("batch span recovered",
				"startAddr", fmt.Sprintf("0x%04X", startAddr),
				"fromState", st.states[startAddr].String())
		}
		st.states[startAddr] = SpanNormal
		st.failCounts[startAddr] = 0
		st.indivFails[startAddr] = 0
	}
}

// RecordFailure increments the failure counter for a span. Returns true if
// the span is degraded (either became degraded now or was already degraded/skipped).
// Note: additional batch failures beyond the threshold keep the span at SpanDegraded;
// they do NOT advance it to SpanSkipped (that requires RecordIndividualFailure).
func (st *SpanTracker) RecordFailure(startAddr uint16) bool {
	st.failCounts[startAddr]++
	if st.states[startAddr] == SpanNormal && st.failCounts[startAddr] >= st.threshold {
		st.states[startAddr] = SpanDegraded
		st.logger.Warn("batch span degraded to individual reads",
			"startAddr", fmt.Sprintf("0x%04X", startAddr),
			"consecutiveFailures", st.failCounts[startAddr])
	}
	return st.states[startAddr] >= SpanDegraded
}

// IsDegraded returns whether a span should use individual reads instead of batch.
// Returns true for both SpanDegraded and SpanSkipped (backward compatible with
// existing callers that check "is this span not normal?").
func (st *SpanTracker) IsDegraded(startAddr uint16) bool {
	return st.states[startAddr] >= SpanDegraded
}

// State returns the current SpanState for a span. Unknown spans return SpanNormal
// (the zero value).
func (st *SpanTracker) State(startAddr uint16) SpanState {
	return st.states[startAddr]
}

// IsSkipped returns whether a span is fully skipped (no reads at all).
func (st *SpanTracker) IsSkipped(startAddr uint16) bool {
	return st.states[startAddr] == SpanSkipped
}

// RecordIndividualFailure increments the individual-read failure counter for a
// degraded span. After 2 consecutive individual-read cycle failures, the span
// transitions to SpanSkipped. Returns true if the span is now skipped.
func (st *SpanTracker) RecordIndividualFailure(startAddr uint16) bool {
	st.indivFails[startAddr]++
	if st.indivFails[startAddr] >= 2 {
		st.states[startAddr] = SpanSkipped
		st.logger.Warn("span fully skipped after individual read failures",
			"startAddr", fmt.Sprintf("0x%04X", startAddr))
	}
	return st.states[startAddr] == SpanSkipped
}

// Tick increments the read cycle counter. Called once per read cycle to advance
// the probe scheduling clock.
func (st *SpanTracker) Tick() {
	st.cycle++
}

// ShouldProbe returns true if a degraded or skipped span should be attempted
// this cycle for recovery. Normal spans are never probed (they read normally).
// Probe attempts occur every probeInterval cycles (D-07).
// Cycle 0 never probes — this prevents immediate probe storms after Reset().
func (st *SpanTracker) ShouldProbe(startAddr uint16) bool {
	if st.states[startAddr] == SpanNormal {
		return false
	}
	return st.cycle > 0 && st.cycle%st.probeInterval == 0
}

// Reset clears all tracking state back to initial values. Used on reconnection
// to give all spans a fresh start.
func (st *SpanTracker) Reset() {
	clear(st.states)
	clear(st.failCounts)
	clear(st.indivFails)
	st.cycle = 0
}
