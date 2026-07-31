package cortex

import (
	"testing"
)

func BenchmarkCortexCycle(b *testing.B) {
	// Initialize a cortex instance once to benchmark steady-state cycle cost.
	c := &Cortex{}
	c.initialize()
	// Disable spontaneous firing to reduce randomness in timing.
	c.spontaneousFire = 0

	// Reset benchmark timer to exclude setup cost.
	b.ResetTimer()

	// Run cycles back-to-back to measure core loop performance.
	for i := 0; i < b.N; i++ {
		c.cycle()
	}
}
