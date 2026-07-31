package cortex

import (
	"encoding/binary"
	"hash/fnv"
	"math"
	"math/rand"
)

// ReplaySignature runs a fixed number of cycles and returns a compact state hash.
// This provides a deterministic replay guardrail for cross-build comparisons.
func ReplaySignature(cycles int, seed int64) uint64 {
	// Seed the RNG so initialization and any random operations are reproducible.
	rand.Seed(seed)

	// Initialize a fresh cortex and disable spontaneous firing for determinism.
	c := &Cortex{}
	c.initialize()
	c.spontaneousFire = 0

	// Build a streaming hash so we can compare signatures cheaply.
	hasher := fnv.New64a()
	buf := make([]byte, 8)

	// Step the cortex and hash a stable subset of state each cycle.
	for i := 0; i < cycles; i++ {
		// Inject a deterministic sensor spike so weights influence potentials.
		c.accumulator[0] = true

		c.cycle()

		// Hash key timing markers to lock ordering determinism.
		binary.LittleEndian.PutUint64(buf, uint64(c.cyclePhase))
		_, _ = hasher.Write(buf)
		binary.LittleEndian.PutUint64(buf, uint64(c.cortexTick))
		_, _ = hasher.Write(buf)

		// Hash representative potentials and traces to detect state drift.
		binary.LittleEndian.PutUint64(buf, math.Float64bits(c.layer1.Potentials[0]))
		_, _ = hasher.Write(buf)
		binary.LittleEndian.PutUint64(buf, math.Float64bits(c.layer1.OutputTraces[0]))
		_, _ = hasher.Write(buf)
		binary.LittleEndian.PutUint64(buf, math.Float64bits(c.outputPotentials[0]))
		_, _ = hasher.Write(buf)
		binary.LittleEndian.PutUint64(buf, uint64(c.sensorTraces[0]))
		_, _ = hasher.Write(buf)
	}

	// Return a compact fingerprint for easy comparisons across builds.
	return hasher.Sum64()
}
