package hub

import (
	"fmt"
	"log/slog"
)

// DefaultDegradationThreshold is the number of consecutive batch failures
// before a span degrades to individual reads (D-04).
const DefaultDegradationThreshold = 3

// SpanTracker tracks consecutive batch failures per span for graceful degradation.
// When a span fails DefaultDegradationThreshold times consecutively, it is marked
// as degraded and should be read using individual register reads instead of batch.
// A single success resets the failure counter and clears degradation (D-04).
//
// SpanTracker is NOT goroutine-safe. It is designed to be used from a single
// goroutine (the hub's streaming goroutine), consistent with Section which also
// lacks a mutex.
type SpanTracker struct {
	failCounts map[uint16]int
	degraded   map[uint16]bool
	threshold  int
	logger     *slog.Logger
}

// NewSpanTracker creates a SpanTracker with the given degradation threshold.
func NewSpanTracker(threshold int, logger *slog.Logger) *SpanTracker {
	return &SpanTracker{
		failCounts: make(map[uint16]int),
		degraded:   make(map[uint16]bool),
		threshold:  threshold,
		logger:     logger,
	}
}

// RecordSuccess resets the failure counter for a span and clears degradation.
// If the span was degraded, logs a recovery event.
func (st *SpanTracker) RecordSuccess(startAddr uint16) {
	if st.failCounts[startAddr] > 0 || st.degraded[startAddr] {
		if st.degraded[startAddr] {
			st.logger.Info("batch span recovered from degradation",
				"startAddr", fmt.Sprintf("0x%04X", startAddr))
		}
		st.failCounts[startAddr] = 0
		st.degraded[startAddr] = false
	}
}

// RecordFailure increments the failure counter for a span. Returns true if
// the span is degraded (either became degraded now or was already degraded).
func (st *SpanTracker) RecordFailure(startAddr uint16) bool {
	st.failCounts[startAddr]++
	if !st.degraded[startAddr] && st.failCounts[startAddr] >= st.threshold {
		st.degraded[startAddr] = true
		st.logger.Warn("batch span degraded to individual reads",
			"startAddr", fmt.Sprintf("0x%04X", startAddr),
			"consecutiveFailures", st.failCounts[startAddr])
	}
	return st.degraded[startAddr]
}

// IsDegraded returns whether a span should use individual reads instead of batch.
func (st *SpanTracker) IsDegraded(startAddr uint16) bool {
	return st.degraded[startAddr]
}
