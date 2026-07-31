package core_logic

import (
	"testing"
	"time"

	"github.com/DjarinDin/TuringSNN/internal/conf"
	"github.com/DjarinDin/TuringSNN/internal/core_logic/hub"
	"github.com/DjarinDin/TuringSNN/pkg/comms"
)

func TestSimCortexHubEndToEnd(t *testing.T) {
	// Boot the full backend wiring so fan-out and hub topics are exercised together.
	backend := NewBackend()
	ch := backend.GetChannels()

	// Disable real data mode so internal generators can emit signals.
	ch.SimControlCh <- comms.IntMsg{Type: comms.SimControlRealDataMode, Value: 0}

	// Lock sim to cortex ticks so timing is deterministic.
	ch.SimControlCh <- comms.IntMsg{Type: comms.SimControlCortexSync, Value: 1}

	// Silence all generators except signal1 to keep the expected spike unambiguous.
	ch.SimControlCh <- comms.IntMsg{Type: comms.SimControlNoiseRun, Value: 0}
	ch.SimControlCh <- comms.IntMsg{Type: comms.SimControlSignal2Run, Value: 0}
	ch.SimControlCh <- comms.IntMsg{Type: comms.SimControlRandomWalkRun, Value: 0}
	ch.SimControlCh <- comms.IntMsg{Type: comms.SimControlTSGen0Run, Value: 0}
	ch.SimControlCh <- comms.IntMsg{Type: comms.SimControlTSGen1Run, Value: 0}
	ch.SimControlCh <- comms.IntMsg{Type: comms.SimControlTSGen2Run, Value: 0}
	ch.SimControlCh <- comms.IntMsg{Type: comms.SimControlTSGen3Run, Value: 0}

	// Force signal1 to always emit by setting its run state and volume to max.
	ch.SimControlCh <- comms.IntMsg{Type: comms.SimControlSignal1Run, Value: 1}
	ch.SimControlCh <- comms.IntMsg{Type: comms.SimControlSignal1Volume, Value: 10}

	// Subscribe to hub sensor spikes to verify fan-out delivery from Cortex.
	sensorCh := backend.GetHub().Subscribe("test-sensor-spikes", hub.TopicSensorSpikes)

	// Wait for cortex async initialization before stepping.
	time.Sleep(600 * time.Millisecond)

	// Step cortex twice: first tick triggers sim, second tick ingests accumulator.
	ch.CortexControlCh <- comms.ControlMsg{Type: comms.CortexControlStep}
	ch.CortexControlCh <- comms.ControlMsg{Type: comms.CortexControlStep}

	// Compute the expected sensor index for signal1 at simTickNum=0.
	expected := (conf.SimSignalPatternLength - 1) % conf.NumSensors

	// Assert we observe a spike payload that includes the expected sensor.
	timeout := time.After(2 * time.Second)
	for {
		select {
		case msg := <-sensorCh:
			spikes, ok := msg.([]bool)
			if !ok {
				continue
			}
			if len(spikes) == 0 {
				continue
			}
			if spikes[expected] {
				return
			}
		case <-timeout:
			t.Fatalf("expected sensor spike %d via hub fan-out", expected)
		}
	}
}
