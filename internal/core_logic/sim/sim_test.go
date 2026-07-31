package sim

import (
	"math/rand"
	"testing"

	"github.com/DjarinDin/TuringSNN/internal/conf"
)

func TestCortexSyncTickFiresDeterministicSignal(t *testing.T) {
	// Seed RNG so any incidental randomness is stable across runs.
	rand.Seed(42)

	// Prepare a buffered channel to capture emitted sensor IDs.
	sensorCh := make(chan int, 10)
	heartbeatCh := make(chan int, 1)

	// Build a sim instance with cortex-sync enabled and real data mode disabled.
	s := NewSim(sensorCh, heartbeatCh, nil)
	s.cortexSync = true
	s.realDataMode = false

	// Disable all generators except signal1 so the expected output is unambiguous.
	s.noiseRun = false
	s.signal2Run = false
	s.randomWalkRun = false
	for i := range s.tsGenRun {
		s.tsGenRun[i] = false
	}

	// Force signal1 to always fire by setting maximum volume.
	s.signal1Run = true
	s.signal1VolumeLevel = 10

	// Trigger a cortex-synced tick; this should run exactly one deterministic cycle.
	CortexTickEvent{}.Handle(s)

	// Compute the expected signal1 sensor ID at simTickNum=0.
	expected := (conf.SimSignalPatternLength - 1) % conf.NumSensors

	// Assert that exactly one signal fired and it matches the expected pattern.
	select {
	case got := <-sensorCh:
		if got != expected {
			t.Fatalf("expected signal1 sensor %d, got %d", expected, got)
		}
	default:
		t.Fatalf("expected a signal to fire in cortex-sync mode")
	}

	// Drain any additional signals to ensure no extra generators fired.
	select {
	case extra := <-sensorCh:
		t.Fatalf("unexpected extra signal fired: %d", extra)
	default:
	}

	// Confirm the sim tick advanced exactly one step.
	if s.simTickNum != 1 {
		t.Fatalf("expected simTickNum=1, got %d", s.simTickNum)
	}
}

func TestCortexTickIgnoredWhenNotSynced(t *testing.T) {
	// Seed RNG to keep any internal state consistent.
	rand.Seed(7)

	// Prepare channels and sim in non-sync mode.
	sensorCh := make(chan int, 10)
	heartbeatCh := make(chan int, 1)
	s := NewSim(sensorCh, heartbeatCh, nil)
	s.cortexSync = false
	s.realDataMode = false

	// Force signal1 on to prove no output is produced when unsynced.
	s.signal1Run = true
	s.signal1VolumeLevel = 10

	// Trigger a cortex tick; should be ignored in async mode.
	CortexTickEvent{}.Handle(s)

	// Verify no signals were emitted.
	select {
	case got := <-sensorCh:
		t.Fatalf("unexpected signal fired in async mode: %d", got)
	default:
	}

	// Verify no tick advancement occurred.
	if s.simTickNum != 0 {
		t.Fatalf("expected simTickNum=0, got %d", s.simTickNum)
	}
}

func TestRealDataModeSuppressesSignals(t *testing.T) {
	// Seed RNG to keep incidental randomness stable.
	rand.Seed(99)

	// Prepare channels and sim in cortex-sync mode.
	sensorCh := make(chan int, 10)
	heartbeatCh := make(chan int, 1)
	s := NewSim(sensorCh, heartbeatCh, nil)
	s.cortexSync = true
	s.realDataMode = true

	// Even with signal1 enabled, real data mode should suppress outputs.
	s.signal1Run = true
	s.signal1VolumeLevel = 10

	// Trigger a cortex-synced tick while in real data mode.
	CortexTickEvent{}.Handle(s)

	// Verify no signals were emitted due to real data mode gating.
	select {
	case got := <-sensorCh:
		t.Fatalf("unexpected signal fired in real data mode: %d", got)
	default:
	}

	// Verify the tick still advanced to keep deterministic time.
	if s.simTickNum != 1 {
		t.Fatalf("expected simTickNum=1, got %d", s.simTickNum)
	}
}
