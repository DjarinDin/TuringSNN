package chaos

import (
	"math"
)

// HurstExponent calculates the Hurst exponent of a time series using R/S analysis.
//
// Interpretation:
//   - H = 0.5: Random walk (no memory, Brownian motion)
//   - H > 0.5: Persistent (trending, positive autocorrelation)
//   - H < 0.5: Anti-persistent (mean-reverting, negative autocorrelation)
//
// Fractal Dimension can be derived as: FD = 2 - H
func HurstExponent(data []float64) float64 {
	if len(data) < 20 {
		return 0.5 // Not enough data, assume random
	}

	// Convert to returns (differences)
	returns := make([]float64, len(data)-1)
	for i := 0; i < len(data)-1; i++ {
		returns[i] = data[i+1] - data[i]
	}

	// R/S analysis across multiple bin sizes
	minBin := 2
	maxBin := len(returns) / 2
	if maxBin < minBin {
		return 0.5
	}

	var logN, logRS []float64

	for binSize := minBin; binSize <= maxBin; binSize++ {
		numBins := len(returns) / binSize
		if numBins < 1 {
			continue
		}

		var rsSum float64
		for b := 0; b < numBins; b++ {
			start := b * binSize
			end := start + binSize
			bin := returns[start:end]

			// Calculate mean of bin
			mean := arrayMean(bin)

			// Calculate cumulative deviation from mean
			cumDev := make([]float64, binSize)
			var sum float64
			for i, v := range bin {
				sum += v - mean
				cumDev[i] = sum
			}

			// Range = max(cumDev) - min(cumDev)
			minDev, maxDev := cumDev[0], cumDev[0]
			for _, v := range cumDev {
				if v < minDev {
					minDev = v
				}
				if v > maxDev {
					maxDev = v
				}
			}
			R := maxDev - minDev

			// Standard deviation of bin
			S := stdDev(bin, mean)
			if S > 0 {
				rsSum += R / S
			}
		}

		if numBins > 0 {
			avgRS := rsSum / float64(numBins)
			if avgRS > 0 {
				logN = append(logN, math.Log(float64(binSize)))
				logRS = append(logRS, math.Log(avgRS))
			}
		}
	}

	if len(logN) < 2 {
		return 0.5
	}

	// Linear regression: log(R/S) = H * log(n) + c
	// Slope = H
	return linearRegressionSlope(logN, logRS)
}

// HurstFromInts is a convenience wrapper for integer arrays
func HurstFromInts(data []int) float64 {
	floats := make([]float64, len(data))
	for i, v := range data {
		floats[i] = float64(v)
	}
	return HurstExponent(floats)
}

// arrayMean calculates the arithmetic mean
func arrayMean(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	var sum float64
	for _, v := range data {
		sum += v
	}
	return sum / float64(len(data))
}

// stdDev calculates the standard deviation given a precomputed mean
func stdDev(data []float64, mean float64) float64 {
	if len(data) < 2 {
		return 0
	}
	var sumSq float64
	for _, v := range data {
		diff := v - mean
		sumSq += diff * diff
	}
	return math.Sqrt(sumSq / float64(len(data)-1))
}

// linearRegressionSlope calculates the slope of a least-squares regression line
func linearRegressionSlope(x, y []float64) float64 {
	if len(x) != len(y) || len(x) < 2 {
		return 0
	}

	n := float64(len(x))
	var sumX, sumY, sumXY, sumX2 float64

	for i := 0; i < len(x); i++ {
		sumX += x[i]
		sumY += y[i]
		sumXY += x[i] * y[i]
		sumX2 += x[i] * x[i]
	}

	denominator := n*sumX2 - sumX*sumX
	if denominator == 0 {
		return 0
	}

	return (n*sumXY - sumX*sumY) / denominator
}
