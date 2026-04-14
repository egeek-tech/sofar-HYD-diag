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

	// 4th failure should still return degraded
	if got := st.RecordFailure(0x0484); !got {
		t.Errorf("RecordFailure #4 (already degraded): got %v, want true", got)
	}
	if got := st.IsDegraded(0x0484); !got {
		t.Errorf("IsDegraded after 4 failures: got %v, want true", got)
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
