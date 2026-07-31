package cortex

import "testing"

func TestReplaySignatureDeterministic(t *testing.T) {
	// Same seed + same cycle count must yield identical signatures.
	const seed = 9001
	const cycles = 10

	first := ReplaySignature(cycles, seed)
	second := ReplaySignature(cycles, seed)

	if first != second {
		t.Fatalf("expected deterministic replay signature, got %d vs %d", first, second)
	}
}

func TestReplaySignatureDifferentSeeds(t *testing.T) {
	// Different seeds should produce different signatures to prove sensitivity.
	const cycles = 10

	first := ReplaySignature(cycles, 1)
	second := ReplaySignature(cycles, 2)

	if first == second {
		t.Fatalf("expected different replay signatures for different seeds, got %d", first)
	}
}
