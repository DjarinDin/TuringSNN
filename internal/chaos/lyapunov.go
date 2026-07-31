// Package chaos provides mathematical chaos analysis tools for SNN behavior monitoring.
// These metrics can be used to tune network parameters and maintain optimal dynamics.
package chaos

import (
	"math"
)

// LyapunovExponent calculates the maximal Lyapunov exponent of a time series.
//
// Interpretation:
//   - λ < 0: Stable attractor (converging, damped)
//   - λ = 0: Steady state (conservative, neutral)
//   - λ > 0: Chaotic! (sensitive dependence on initial conditions)
//
// The "edge of chaos" (λ ≈ 0 but slightly positive) is often optimal for computation.
func LyapunovExponent(data []float64) float64 {
	if len(data) < 10 {
		return 0 // Not enough data
	}

	// Smooth the data to reduce noise artifacts
	smoothed := smooth(data)

	// Minimum distance threshold: pairs closer than this are skipped
	// to prevent near-constant signals from producing spurious high Lyapunov
	// (tiny distances create huge ratios even with small absolute changes)
	const minDistThreshold = 0.01

	var sum float64
	var count int

	for p1 := 0; p1 < len(smoothed)-1; p1++ {
		// Find the closest point to p1 (that isn't p1 or adjacent)
		// but also above the minimum distance threshold
		minDist := math.MaxFloat64
		p2 := -1

		for j := 0; j < len(smoothed); j++ {
			if j == p1 || j == p1-1 || j == p1+1 {
				continue
			}
			dist := math.Abs(smoothed[j] - smoothed[p1])
			if dist > minDistThreshold && dist < minDist {
				minDist = dist
				p2 = j
			}
		}

		if p2 < 0 || p2 >= len(smoothed)-1 {
			continue
		}

		// Calculate divergence after one step
		initialDist := math.Abs(smoothed[p1] - smoothed[p2])
		finalDist := math.Abs(smoothed[p1+1] - smoothed[p2+1])

		if initialDist > minDistThreshold && finalDist > 0 {
			ratio := finalDist / initialDist
			if ratio > 0 {
				sum += math.Log(ratio)
				count++
			}
		}
	}

	if count == 0 {
		return 0 // Near-constant signal - return neutral
	}

	return sum / float64(count)
}

// smooth applies a 3-point weighted average to reduce noise
func smooth(data []float64) []float64 {
	if len(data) < 3 {
		return data
	}

	result := make([]float64, len(data))
	result[0] = data[0]
	result[len(data)-1] = data[len(data)-1]

	for i := 1; i < len(data)-1; i++ {
		// Weighted average: 1:6:1 (heavily weighted to center)
		result[i] = (data[i-1] + 6*data[i] + data[i+1]) / 8
	}

	return result
}

// LyapunovFromInts is a convenience wrapper for integer arrays
func LyapunovFromInts(data []int) float64 {
	floats := make([]float64, len(data))
	for i, v := range data {
		floats[i] = float64(v)
	}
	return LyapunovExponent(floats)
}
