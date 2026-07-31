package cortex

import (
	"math/rand"
	"testing"

	"github.com/DjarinDin/TuringSNN/internal/conf"
)

func TestInitializeDeterminism(t *testing.T) {
	// This test locks the deterministic initialization path by re-seeding before each init.
	const seed = 1337

	first := &Cortex{}
	rand.Seed(seed)
	first.initialize()

	second := &Cortex{}
	rand.Seed(seed)
	second.initialize()

	// Probe a focused subset of weights/traces that are sufficient to detect drift.
	if first.layer1.Weights[0][0] != second.layer1.Weights[0][0] {
		t.Fatalf("layer1 weight mismatch: %d vs %d", first.layer1.Weights[0][0], second.layer1.Weights[0][0])
	}
	if first.layer2.Weights[7][13] != second.layer2.Weights[7][13] {
		t.Fatalf("layer2 weight mismatch: %d vs %d", first.layer2.Weights[7][13], second.layer2.Weights[7][13])
	}
	if first.outputWeights[0][0] != second.outputWeights[0][0] {
		t.Fatalf("output weight mismatch: %d vs %d", first.outputWeights[0][0], second.outputWeights[0][0])
	}
}

func TestHardWTASelectsSingleWinner(t *testing.T) {
	// This test verifies that Hard WTA produces a single winning spike per cycle.
	prevMode := conf.ZeitgeistInhibitionMode
	prevK := conf.ZeitgeistWTAK
	conf.ZeitgeistInhibitionMode = conf.ZeitgeistInhibitionHardWTA
	conf.ZeitgeistWTAK = 1
	defer func() {
		conf.ZeitgeistInhibitionMode = prevMode
		conf.ZeitgeistWTAK = prevK
	}()

	layer := &layer{}
	cyclePhase := 0
	cortexTick := 1

	// Set thresholds low to make potentials the only determinant of winners.
	for nrn := 0; nrn < conf.NumNeurons; nrn++ {
		layer.Thresholds[nrn] = 0
	}

	// Establish a single clear winner at neuron 3.
	for nrn := 0; nrn < conf.NumNeurons; nrn++ {
		layer.Potentials[nrn] = 1
	}
	winner := 3
	layer.Potentials[winner] = 10

	// Run spike calculation with no spontaneous firing to keep behavior deterministic.
	layer.calculateSpikes(cyclePhase, 0, cortexTick)

	// Assert only the winner spiked in the Gestalt field this cycle.
	for nrn := 0; nrn < conf.NumNeurons; nrn++ {
		spike := layer.Spikes[cyclePhase][conf.QuadGestaltStart+nrn]
		if nrn == winner && !spike {
			t.Fatalf("expected winner neuron %d to spike", winner)
		}
		if nrn != winner && spike {
			t.Fatalf("expected neuron %d to be suppressed by WTA", nrn)
		}
	}
}

func TestDeterministicReplayCycle(t *testing.T) {
	// This test guards against non-deterministic cycle behavior under fixed seed.
	const seed = 4242

	first := &Cortex{}
	rand.Seed(seed)
	first.initialize()
	// Disable spontaneous firing so the cycle is fully deterministic.
	first.spontaneousFire = 0

	second := &Cortex{}
	rand.Seed(seed)
	second.initialize()
	// Disable spontaneous firing so the cycle is fully deterministic.
	second.spontaneousFire = 0

	// Run a short replay window to compare structural state.
	for i := 0; i < 5; i++ {
		first.cycle()
		second.cycle()
	}

	// Compare a stable subset of state that would diverge if determinism breaks.
	if first.layer1.Potentials[0] != second.layer1.Potentials[0] {
		t.Fatalf("layer1 potential mismatch: %f vs %f", first.layer1.Potentials[0], second.layer1.Potentials[0])
	}
	if first.layer2.OutputTraces[10] != second.layer2.OutputTraces[10] {
		t.Fatalf("layer2 trace mismatch: %f vs %f", first.layer2.OutputTraces[10], second.layer2.OutputTraces[10])
	}
	if first.outputPotentials[2] != second.outputPotentials[2] {
		t.Fatalf("output potential mismatch: %f vs %f", first.outputPotentials[2], second.outputPotentials[2])
	}
}
