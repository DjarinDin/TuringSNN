package sim

import (
	"github.com/DjarinDin/TuringSNN/internal/core_logic/hub"
	"math"
	"testing"
)

type retributionSpy struct {
	calls []float64
}

func (r *retributionSpy) ApplyRetribution(value float64) {
	r.calls = append(r.calls, value)
}

func TestPositionTrackerFIFOCloseReward(t *testing.T) {
	h := hub.NewHub()
	pt := NewPositionTracker(h, nil)
	spy := &retributionSpy{}
	pt.retribution = spy
	pt.running = true
	pt.policy.MinHoldTicks = 0
	pt.currentTick = 1
	pt.currentValue = 100

	pt.handleOpen()
	if len(pt.lots) != 1 {
		t.Fatalf("expected 1 open lot, got %d", len(pt.lots))
	}

	pt.currentTick = 2
	pt.currentValue = 110
	pt.handleClose()

	if len(pt.lots) != 0 {
		t.Fatalf("expected 0 open lots after sell, got %d", len(pt.lots))
	}
	if len(pt.history) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(pt.history))
	}
	expectedReturn := (110.0-100.0)/100.0 - pt.policy.Friction
	if math.Abs(pt.history[0].Return-expectedReturn) > 1e-9 {
		t.Fatalf("expected return %.6f, got %.6f", expectedReturn, pt.history[0].Return)
	}
	if len(spy.calls) != 1 {
		t.Fatalf("expected 1 retribution call, got %d", len(spy.calls))
	}
	if math.Abs(spy.calls[0]-expectedReturn) > 1e-9 {
		t.Fatalf("expected retribution %.6f, got %.6f", expectedReturn, spy.calls[0])
	}
}

func TestPositionTrackerMinHoldBlocksExit(t *testing.T) {
	h := hub.NewHub()
	pt := NewPositionTracker(h, nil)
	spy := &retributionSpy{}
	pt.retribution = spy
	pt.running = true
	pt.policy.MinHoldTicks = 10
	pt.currentTick = 1
	pt.currentValue = 100

	pt.handleOpen()
	if len(pt.lots) != 1 {
		t.Fatalf("expected 1 open lot, got %d", len(pt.lots))
	}

	pt.currentTick = 5
	pt.currentValue = 110
	pt.handleClose()

	if len(pt.lots) != 1 {
		t.Fatalf("expected lot to remain open, got %d", len(pt.lots))
	}
	if len(pt.history) != 0 {
		t.Fatalf("expected no history entries, got %d", len(pt.history))
	}
	if len(spy.calls) != 0 {
		t.Fatalf("expected no retribution calls, got %d", len(spy.calls))
	}
}

func TestPositionTrackerLossPenalty(t *testing.T) {
	h := hub.NewHub()
	pt := NewPositionTracker(h, nil)
	spy := &retributionSpy{}
	pt.retribution = spy
	pt.running = true
	pt.policy.MinHoldTicks = 0
	pt.currentTick = 1
	pt.currentValue = 100

	pt.handleOpen()
	pt.currentTick = 2
	pt.currentValue = 90
	pt.handleClose()

	expectedReturn := (90.0-100.0)/100.0 - pt.policy.Friction
	expectedRetribution := expectedReturn * pt.policy.PenaltyMultiplier
	if len(spy.calls) != 1 {
		t.Fatalf("expected 1 retribution call, got %d", len(spy.calls))
	}
	if math.Abs(spy.calls[0]-expectedRetribution) > 1e-9 {
		t.Fatalf("expected retribution %.6f, got %.6f", expectedRetribution, spy.calls[0])
	}
}

func TestPositionTrackerGlobalFlush(t *testing.T) {
	h := hub.NewHub()
	pt := NewPositionTracker(h, nil)
	spy := &retributionSpy{}
	pt.retribution = spy
	pt.running = true
	pt.policy.MinHoldTicks = 0
	pt.policy.ExitMode = ExitModeGlobalFlush

	pt.currentTick = 1
	pt.currentValue = 100
	pt.handleOpen()
	pt.currentTick = 2
	pt.currentValue = 110
	pt.handleOpen()
	pt.currentTick = 3
	pt.currentValue = 120
	pt.handleOpen()

	if len(pt.lots) != 3 {
		t.Fatalf("expected 3 open lots, got %d", len(pt.lots))
	}

	pt.currentTick = 10
	pt.currentValue = 130
	pt.handleClose()

	if len(pt.lots) != 0 {
		t.Fatalf("expected all lots closed, got %d", len(pt.lots))
	}
	if len(pt.history) != 3 {
		t.Fatalf("expected 3 history entries, got %d", len(pt.history))
	}
	if len(spy.calls) != 3 {
		t.Fatalf("expected 3 retribution calls, got %d", len(spy.calls))
	}
}
