package chaos

import (
	"math"
)

// Daubechies D4 wavelet coefficients
var (
	h0 = (1 + math.Sqrt(3)) / (4 * math.Sqrt(2))
	h1 = (3 + math.Sqrt(3)) / (4 * math.Sqrt(2))
	h2 = (3 - math.Sqrt(3)) / (4 * math.Sqrt(2))
	h3 = (1 - math.Sqrt(3)) / (4 * math.Sqrt(2))

	g0 = h3
	g1 = -h2
	g2 = h1
	g3 = -h0
)

// HaarTransform performs a single-level Haar wavelet transform.
// Returns two arrays: approximation coefficients (low-freq) and detail coefficients (high-freq).
// Input length must be a power of 2.
func HaarTransform(data []float64) (approx, detail []float64) {
	n := len(data) / 2
	if n < 1 {
		return data, nil
	}

	approx = make([]float64, n)
	detail = make([]float64, n)

	for i := 0; i < n; i++ {
		approx[i] = (data[2*i] + data[2*i+1]) / math.Sqrt(2)
		detail[i] = (data[2*i] - data[2*i+1]) / math.Sqrt(2)
	}

	return approx, detail
}

// HaarMultiLevel performs multi-level Haar wavelet decomposition.
// Returns a slice of detail coefficients at each level, plus the final approximation.
func HaarMultiLevel(data []float64, levels int) (details [][]float64, finalApprox []float64) {
	current := data
	details = make([][]float64, 0, levels)

	for level := 0; level < levels && len(current) >= 2; level++ {
		approx, detail := HaarTransform(current)
		details = append(details, detail)
		current = approx
	}

	return details, current
}

// DaubechiesD4Transform performs a single-level Daubechies D4 wavelet transform.
// This provides better frequency localization than Haar.
// Input length must be a power of 2 and >= 4.
func DaubechiesD4Transform(data []float64) (approx, detail []float64) {
	n := len(data)
	if n < 4 {
		return HaarTransform(data) // Fall back to Haar for small data
	}

	halfN := n / 2
	approx = make([]float64, halfN)
	detail = make([]float64, halfN)

	for i := 0; i < halfN; i++ {
		// Wrap-around indexing for edges
		i0 := 2 * i
		i1 := (i0 + 1) % n
		i2 := (i0 + 2) % n
		i3 := (i0 + 3) % n

		// Scaling function (low-pass)
		approx[i] = h0*data[i0] + h1*data[i1] + h2*data[i2] + h3*data[i3]

		// Wavelet function (high-pass)
		detail[i] = g0*data[i0] + g1*data[i1] + g2*data[i2] + g3*data[i3]
	}

	return approx, detail
}

// WaveletEnergy calculates the energy (sum of squares) at each decomposition level.
// Useful for detecting which timescales contain the most activity.
func WaveletEnergy(details [][]float64) []float64 {
	energies := make([]float64, len(details))
	for i, level := range details {
		var sum float64
		for _, v := range level {
			sum += v * v
		}
		energies[i] = sum
	}
	return energies
}

// DominantScale returns the decomposition level with the highest energy.
// Level 0 = finest detail (highest frequency), higher levels = coarser (lower frequency).
func DominantScale(details [][]float64) int {
	energies := WaveletEnergy(details)
	maxEnergy := 0.0
	maxLevel := 0

	for i, e := range energies {
		if e > maxEnergy {
			maxEnergy = e
			maxLevel = i
		}
	}

	return maxLevel
}
