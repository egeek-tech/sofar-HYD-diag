package hub_test

import (
	"log/slog"
	"testing"

	"sofar-hyd-diag/internal/hub"
)

func TestSpanTracker_NewTracker(t *testing.T) {
	st := hub.NewSpanTracker(3, slog.Default())
	if st == nil {
		t.Fatal("NewSpanTracker returned nil")
	}
	if got := st.IsDegraded(0x0484); got {
		t.Errorf("IsDegraded for unknown span: got %v, want false", got)
	}
	// New: State() for unknown span should return SpanNormal
	if got := st.State(0x0484); got != hub.SpanNormal {
		t.Errorf("State for unknown span: got %v, want SpanNormal", got)
	}
}

func TestSpanTracker_SingleFailureNotDegraded(t *testing.T) {
	st := hub.NewSpanTracker(3, slog.Default())

	degraded := st.RecordFailure(0x0484)
	if degraded {
		t.Errorf("RecordFailure after 1 failure: got %v, want false", degraded)
	}
	if got := st.IsDegraded(0x0484); got {
		t.Errorf("IsDegraded after 1 failure: got %v, want false", got)
	}
}

func TestSpanTracker_DegradeAtThreshold(t *testing.T) {
	st := hub.NewSpanTracker(3, slog.Default())

	// First two failures should not degrade
	if got := st.RecordFailure(0x0484); got {
		t.Errorf("RecordFailure #1: got %v, want false", got)
	}
	if got := st.RecordFailure(0x0484); got {
		t.Errorf("RecordFailure #2: got %v, want false", got)
	}

	// Third failure should trigger degradation
	if got := st.RecordFailure(0x0484); !got {
		t.Errorf("RecordFailure #3: got %v, want true", got)
	}
	if got := st.IsDegraded(0x0484); !got {
		t.Errorf("IsDegraded after threshold: got %v, want true", got)
	}
}

func TestSpanTracker_RecoveryOnSuccess(t *testing.T) {
	st := hub.NewSpanTracker(3, slog.Default())

	// Degrade the span
	st.RecordFailure(0x0484)
	st.RecordFailure(0x0484)
	st.RecordFailure(0x0484)
	if got := st.IsDegraded(0x0484); !got {
		t.Fatalf("expected degraded after 3 failures, got %v", got)
	}

	// Recover via success
	st.RecordSuccess(0x0484)
	if got := st.IsDegraded(0x0484); got {
		t.Errorf("IsDegraded after recovery: got %v, want false", got)
	}

	// Counter was reset, so one failure should not degrade
	if got := st.RecordFailure(0x0484); got {
		t.Errorf("RecordFailure after recovery: got %v, want false (counter was reset)", got)
	}
}

func TestSpanTracker_SuccessWithoutPriorFailure(t *testing.T) {
	st := hub.NewSpanTracker(3, slog.Default())

	// Should not panic on fresh tracker
	st.RecordSuccess(0x0484)
	if got := st.IsDegraded(0x0484); got {
		t.Errorf("IsDegraded after success on fresh tracker: got %v, want false", got)
	}
}

func TestSpanTracker_IndependentSpans(t *testing.T) {
	st := hub.NewSpanTracker(3, slog.Default())

	// Degrade span 0x0484
	st.RecordFailure(0x0484)
	st.RecordFailure(0x0484)
	st.RecordFailure(0x0484)

	// Different span should be independent
	if got := st.IsDegraded(0x0488); got {
		t.Errorf("IsDegraded for independent span 0x0488: got %v, want false", got)
	}
	if got := st.State(0x0488); got != hub.SpanNormal {
		t.Errorf("State for independent span 0x0488: got %v, want SpanNormal", got)
	}

	// Success on independent span should not affect degraded span
	st.RecordSuccess(0x0488)
	if got := st.IsDegraded(0x0484); !got {
		t.Errorf("IsDegraded for 0x0484 after success on 0x0488: got %v, want true", got)
	}
}

func TestSpanTracker_StaysDegradedOnMoreFailures(t *testing.T) {
	st := hub.NewSpanTracker(3, slog.Default())

	// Degrade the span
	st.RecordFailure(0x0484)
	st.RecordFailure(0x0484)
	st.RecordFailure(0x0484)

	// 4th failure should still return degraded (stays SpanDegraded, not advance to SpanSkipped)
	if got := st.RecordFailure(0x0484); !got {
		t.Errorf("RecordFailure #4 (already degraded): got %v, want true", got)
	}
	if got := st.IsDegraded(0x0484); !got {
		t.Errorf("IsDegraded after 4 failures: got %v, want true", got)
	}
	if got := st.State(0x0484); got != hub.SpanDegraded {
		t.Errorf("State after 4 batch failures: got %v, want SpanDegraded", got)
	}
}

func TestSpanTracker_ReDegradesAfterRecovery(t *testing.T) {
	st := hub.NewSpanTracker(3, slog.Default())

	// First degradation cycle
	st.RecordFailure(0x0484)
	st.RecordFailure(0x0484)
	st.RecordFailure(0x0484)
	if got := st.IsDegraded(0x0484); !got {
		t.Fatalf("expected degraded after first cycle")
	}

	// Recover
	st.RecordSuccess(0x0484)
	if got := st.IsDegraded(0x0484); got {
		t.Fatalf("expected not degraded after recovery")
	}

	// Second degradation cycle
	st.RecordFailure(0x0484)
	st.RecordFailure(0x0484)
	if got := st.IsDegraded(0x0484); got {
		t.Errorf("IsDegraded after 2 failures in second cycle: got %v, want false", got)
	}
	if got := st.RecordFailure(0x0484); !got {
		t.Errorf("RecordFailure #3 in second cycle: got %v, want true", got)
	}
	if got := st.IsDegraded(0x0484); !got {
		t.Errorf("IsDegraded after re-degradation: got %v, want true", got)
	}
}

// === New three-state tests ===

func TestSpanTracker_ThreeStateDegradation(t *testing.T) {
	st := hub.NewSpanTracker(3, slog.Default())

	// 3 batch failures -> SpanDegraded
	st.RecordFailure(0x0484)
	st.RecordFailure(0x0484)
	st.RecordFailure(0x0484)
	if got := st.State(0x0484); got != hub.SpanDegraded {
		t.Errorf("State after 3 batch failures: got %v, want SpanDegraded", got)
	}

	// 1st individual failure -> still SpanDegraded
	skipped := st.RecordIndividualFailure(0x0484)
	if skipped {
		t.Errorf("RecordIndividualFailure #1: got skipped=true, want false")
	}
	if got := st.State(0x0484); got != hub.SpanDegraded {
		t.Errorf("State after 1 individual failure: got %v, want SpanDegraded", got)
	}

	// 2nd individual failure -> SpanSkipped
	skipped = st.RecordIndividualFailure(0x0484)
	if !skipped {
		t.Errorf("RecordIndividualFailure #2: got skipped=false, want true")
	}
	if got := st.State(0x0484); got != hub.SpanSkipped {
		t.Errorf("State after 2 individual failures: got %v, want SpanSkipped", got)
	}
}

func TestSpanTracker_RecoveryFromDegraded(t *testing.T) {
	st := hub.NewSpanTracker(3, slog.Default())

	// Degrade span
	st.RecordFailure(0x0484)
	st.RecordFailure(0x0484)
	st.RecordFailure(0x0484)
	if got := st.State(0x0484); got != hub.SpanDegraded {
		t.Fatalf("expected SpanDegraded, got %v", got)
	}

	// RecordSuccess resets to Normal
	st.RecordSuccess(0x0484)
	if got := st.State(0x0484); got != hub.SpanNormal {
		t.Errorf("State after recovery from degraded: got %v, want SpanNormal", got)
	}
	if got := st.IsDegraded(0x0484); got {
		t.Errorf("IsDegraded after recovery: got %v, want false", got)
	}
}

func TestSpanTracker_RecoveryFromSkipped(t *testing.T) {
	st := hub.NewSpanTracker(3, slog.Default())

	// Degrade then skip
	st.RecordFailure(0x0484)
	st.RecordFailure(0x0484)
	st.RecordFailure(0x0484)
	st.RecordIndividualFailure(0x0484)
	st.RecordIndividualFailure(0x0484)
	if got := st.State(0x0484); got != hub.SpanSkipped {
		t.Fatalf("expected SpanSkipped, got %v", got)
	}

	// D-08: RecordSuccess resets to Normal (not back to Degraded)
	st.RecordSuccess(0x0484)
	if got := st.State(0x0484); got != hub.SpanNormal {
		t.Errorf("State after recovery from skipped: got %v, want SpanNormal", got)
	}
	if got := st.IsDegraded(0x0484); got {
		t.Errorf("IsDegraded after recovery from skipped: got %v, want false", got)
	}
	if got := st.IsSkipped(0x0484); got {
		t.Errorf("IsSkipped after recovery from skipped: got %v, want false", got)
	}
}

func TestSpanTracker_IndividualFailureOneNotSkipped(t *testing.T) {
	st := hub.NewSpanTracker(3, slog.Default())

	// Degrade first
	st.RecordFailure(0x0484)
	st.RecordFailure(0x0484)
	st.RecordFailure(0x0484)

	// 1 individual failure should NOT skip
	skipped := st.RecordIndividualFailure(0x0484)
	if skipped {
		t.Errorf("RecordIndividualFailure #1: got skipped=true, want false")
	}
	if got := st.State(0x0484); got != hub.SpanDegraded {
		t.Errorf("State after 1 individual failure: got %v, want SpanDegraded", got)
	}
	if got := st.IsSkipped(0x0484); got {
		t.Errorf("IsSkipped after 1 individual failure: got %v, want false", got)
	}
}

func TestSpanTracker_ProbeScheduling(t *testing.T) {
	st := hub.NewSpanTracker(3, slog.Default())

	// Degrade a span
	st.RecordFailure(0x0484)
	st.RecordFailure(0x0484)
	st.RecordFailure(0x0484)

	// Tick 10 times -> probe interval hit
	for i := 0; i < 10; i++ {
		st.Tick()
	}
	if got := st.ShouldProbe(0x0484); !got {
		t.Errorf("ShouldProbe at cycle 10 for degraded span: got %v, want true", got)
	}

	// Tick once more -> cycle 11, not a probe cycle
	st.Tick()
	if got := st.ShouldProbe(0x0484); got {
		t.Errorf("ShouldProbe at cycle 11 for degraded span: got %v, want false", got)
	}
}

func TestSpanTracker_ProbeNotForNormal(t *testing.T) {
	st := hub.NewSpanTracker(3, slog.Default())

	// Tick 10 times
	for i := 0; i < 10; i++ {
		st.Tick()
	}

	// Normal spans should never be probed
	if got := st.ShouldProbe(0x0484); got {
		t.Errorf("ShouldProbe at cycle 10 for normal span: got %v, want false", got)
	}
}

func TestSpanTracker_ProbeForSkipped(t *testing.T) {
	st := hub.NewSpanTracker(3, slog.Default())

	// Skip the span
	st.RecordFailure(0x0484)
	st.RecordFailure(0x0484)
	st.RecordFailure(0x0484)
	st.RecordIndividualFailure(0x0484)
	st.RecordIndividualFailure(0x0484)
	if got := st.State(0x0484); got != hub.SpanSkipped {
		t.Fatalf("expected SpanSkipped, got %v", got)
	}

	// Tick 10 times -> probe interval
	for i := 0; i < 10; i++ {
		st.Tick()
	}
	if got := st.ShouldProbe(0x0484); !got {
		t.Errorf("ShouldProbe at cycle 10 for skipped span: got %v, want true", got)
	}
}

func TestSpanTracker_Reset(t *testing.T) {
	st := hub.NewSpanTracker(3, slog.Default())

	// Degrade one span, skip another
	st.RecordFailure(0x0484)
	st.RecordFailure(0x0484)
	st.RecordFailure(0x0484)

	st.RecordFailure(0x0488)
	st.RecordFailure(0x0488)
	st.RecordFailure(0x0488)
	st.RecordIndividualFailure(0x0488)
	st.RecordIndividualFailure(0x0488)

	// Tick several times
	for i := 0; i < 5; i++ {
		st.Tick()
	}

	// Reset everything
	st.Reset()

	// All states should be SpanNormal
	if got := st.State(0x0484); got != hub.SpanNormal {
		t.Errorf("State of 0x0484 after reset: got %v, want SpanNormal", got)
	}
	if got := st.State(0x0488); got != hub.SpanNormal {
		t.Errorf("State of 0x0488 after reset: got %v, want SpanNormal", got)
	}
	if got := st.IsDegraded(0x0484); got {
		t.Errorf("IsDegraded for 0x0484 after reset: got %v, want false", got)
	}
	if got := st.IsSkipped(0x0488); got {
		t.Errorf("IsSkipped for 0x0488 after reset: got %v, want false", got)
	}

	// ShouldProbe should be false (cycle reset to 0)
	if got := st.ShouldProbe(0x0484); got {
		t.Errorf("ShouldProbe after reset: got %v, want false", got)
	}
}

func TestSpanTracker_IsSkipped(t *testing.T) {
	st := hub.NewSpanTracker(3, slog.Default())

	// Normal span: IsSkipped false
	if got := st.IsSkipped(0x0484); got {
		t.Errorf("IsSkipped for normal span: got %v, want false", got)
	}

	// Degraded span: IsSkipped false
	st.RecordFailure(0x0484)
	st.RecordFailure(0x0484)
	st.RecordFailure(0x0484)
	if got := st.IsSkipped(0x0484); got {
		t.Errorf("IsSkipped for degraded span: got %v, want false", got)
	}

	// Skipped span: IsSkipped true
	st.RecordIndividualFailure(0x0484)
	st.RecordIndividualFailure(0x0484)
	if got := st.IsSkipped(0x0484); !got {
		t.Errorf("IsSkipped for skipped span: got %v, want true", got)
	}
}
