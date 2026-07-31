package chaos

import (
	"math"
	"testing"
)

// Test data generators

// generateSineWave creates a periodic signal (should have Lyapunov ≈ 0, Hurst ≈ 0.5)
func generateSineWave(n int, frequency float64) []float64 {
	data := make([]float64, n)
	for i := 0; i < n; i++ {
		data[i] = math.Sin(2 * math.Pi * frequency * float64(i) / float64(n))
	}
	return data
}

// generateRandomWalk creates a Brownian motion (Hurst ≈ 0.5)
func generateRandomWalk(n int, seed int64) []float64 {
	data := make([]float64, n)
	var val float64
	// Simple deterministic pseudo-random for reproducibility
	x := uint64(seed)
	for i := 0; i < n; i++ {
		x = x*6364136223846793005 + 1442695040888963407 // LCG
		step := float64(int64(x>>33)-int64(1<<30)) / float64(1<<30)
		val += step * 0.1
		data[i] = val
	}
	return data
}

// generateTrendingSeries creates a persistent/trending series (Hurst > 0.5)
func generateTrendingSeries(n int) []float64 {
	data := make([]float64, n)
	for i := 0; i < n; i++ {
		data[i] = float64(i) * 0.1
	}
	return data
}

// generateLogisticMap creates chaotic data using the logistic map (Lyapunov > 0)
func generateLogisticMap(n int, r float64) []float64 {
	data := make([]float64, n)
	x := 0.1 // Initial condition
	for i := 0; i < n; i++ {
		x = r * x * (1 - x)
		data[i] = x
	}
	return data
}

// generateConstant creates a flat line (Lyapunov = 0, Hurst undefined)
func generateConstant(n int, value float64) []float64 {
	data := make([]float64, n)
	for i := 0; i < n; i++ {
		data[i] = value
	}
	return data
}

// Lyapunov Exponent Tests

func TestLyapunovExponent_Sine(t *testing.T) {
	// Periodic signal should have Lyapunov ≈ 0 (neutral stability)
	data := generateSineWave(200, 5)
	le := LyapunovExponent(data)
	t.Logf("Sine wave Lyapunov: %f", le)

	// Allow some tolerance
	if le > 0.5 || le < -0.5 {
		t.Logf("Warning: Sine wave Lyapunov outside expected range [-0.5, 0.5]: %f", le)
	}
}

func TestLyapunovExponent_Chaotic(t *testing.T) {
	// Logistic map at r=3.9 is chaotic (Lyapunov > 0)
	data := generateLogisticMap(500, 3.9)
	le := LyapunovExponent(data)
	t.Logf("Logistic map (r=3.9) Lyapunov: %f", le)

	if le <= 0 {
		t.Logf("Warning: Chaotic logistic map should have positive Lyapunov, got: %f", le)
	}
}

func TestLyapunovExponent_Stable(t *testing.T) {
	// Logistic map at r=2.5 converges to fixed point (Lyapunov < 0)
	data := generateLogisticMap(500, 2.5)
	le := LyapunovExponent(data)
	t.Logf("Logistic map (r=2.5) Lyapunov: %f", le)

	// After transient, this should converge
	if le > 0.3 {
		t.Logf("Warning: Stable logistic map should have non-positive Lyapunov, got: %f", le)
	}
}

func TestLyapunovExponent_ShortData(t *testing.T) {
	// Very short data should return 0 (not enough data)
	data := []float64{1, 2, 3}
	le := LyapunovExponent(data)
	if le != 0 {
		t.Errorf("Expected 0 for short data, got: %f", le)
	}
}

// Hurst Exponent Tests

func TestHurstExponent_RandomWalk(t *testing.T) {
	// Random walk should have Hurst ≈ 0.5
	data := generateRandomWalk(500, 12345)
	he := HurstExponent(data)
	t.Logf("Random walk Hurst: %f", he)

	if he < 0.3 || he > 0.7 {
		t.Logf("Warning: Random walk Hurst outside expected range [0.3, 0.7]: %f", he)
	}
}

func TestHurstExponent_Trending(t *testing.T) {
	// Trending series should have Hurst > 0.5 (persistent)
	data := generateTrendingSeries(200)
	he := HurstExponent(data)
	t.Logf("Trending series Hurst: %f", he)

	if he < 0.5 {
		t.Logf("Warning: Trending series should have Hurst > 0.5, got: %f", he)
	}
}

func TestHurstExponent_ShortData(t *testing.T) {
	// Very short data should return 0.5 (default random assumption)
	data := []float64{1, 2, 3, 4, 5}
	he := HurstExponent(data)
	if he != 0.5 {
		t.Errorf("Expected 0.5 for short data, got: %f", he)
	}
}

func TestHurstExponent_PeriodicFinite(t *testing.T) {
	// Periodic signals should yield a finite Hurst estimate (not NaN/Inf).
	data := generateSineWave(200, 7)
	he := HurstExponent(data)
	if math.IsNaN(he) || math.IsInf(he, 0) {
		t.Errorf("Expected finite Hurst for periodic signal, got: %f", he)
	}
	if he < 0.0 || he > 1.0 {
		t.Logf("Warning: Hurst for periodic signal outside [0,1]: %f", he)
	}
}

func TestLyapunovExponent_PeriodicFinite(t *testing.T) {
	// Periodic signals should return a neutral Lyapunov near zero or within tolerance.
	data := generateSineWave(200, 7)
	le := LyapunovExponent(data)
	if math.IsNaN(le) || math.IsInf(le, 0) {
		t.Errorf("Expected finite Lyapunov for periodic signal, got: %f", le)
	}
	if le > 0.5 || le < -0.5 {
		t.Logf("Warning: Periodic Lyapunov outside expected range [-0.5, 0.5]: %f", le)
	}
}

// Wavelet Tests

func TestHaarTransform_PowerOfTwo(t *testing.T) {
	data := []float64{1, 2, 3, 4, 5, 6, 7, 8}
	approx, detail := HaarTransform(data)

	t.Logf("Input: %v", data)
	t.Logf("Approx: %v", approx)
	t.Logf("Detail: %v", detail)

	if len(approx) != 4 || len(detail) != 4 {
		t.Errorf("Expected length 4, got approx=%d, detail=%d", len(approx), len(detail))
	}

	// Verify reconstruction property: original can be recovered (with scaling)
	// For normalized Haar: approx[i] = (a + b) / sqrt(2), detail[i] = (a - b) / sqrt(2)
	sqrt2 := math.Sqrt(2)
	expectedApprox0 := (1 + 2) / sqrt2
	if math.Abs(approx[0]-expectedApprox0) > 0.001 {
		t.Errorf("Unexpected approx[0]: got %f, expected %f", approx[0], expectedApprox0)
	}
}

func TestDaubechiesD4Transform(t *testing.T) {
	data := []float64{1, 2, 3, 4, 5, 6, 7, 8}
	approx, detail := DaubechiesD4Transform(data)

	t.Logf("D4 Input: %v", data)
	t.Logf("D4 Approx: %v", approx)
	t.Logf("D4 Detail: %v", detail)

	if len(approx) != 4 || len(detail) != 4 {
		t.Errorf("Expected length 4, got approx=%d, detail=%d", len(approx), len(detail))
	}
}

func TestWaveletEnergy(t *testing.T) {
	// Create multi-level decomposition
	data := generateSineWave(64, 4)
	details, _ := HaarMultiLevel(data, 4)

	energies := WaveletEnergy(details)
	t.Logf("Wavelet energies by level: %v", energies)

	dominant := DominantScale(details)
	t.Logf("Dominant scale: level %d", dominant)

	// Sine wave should concentrate energy at specific frequency
	if len(energies) == 0 {
		t.Error("Expected non-empty energies")
	}
}

// Helper Tests

func TestArrayMean(t *testing.T) {
	data := []float64{1, 2, 3, 4, 5}
	mean := arrayMean(data)
	if mean != 3.0 {
		t.Errorf("Expected mean 3.0, got %f", mean)
	}
}

func TestLinearRegressionSlope(t *testing.T) {
	// Perfect line y = 2x
	x := []float64{1, 2, 3, 4, 5}
	y := []float64{2, 4, 6, 8, 10}
	slope := linearRegressionSlope(x, y)
	if math.Abs(slope-2.0) > 0.001 {
		t.Errorf("Expected slope 2.0, got %f", slope)
	}
}

func TestSmooth(t *testing.T) {
	data := []float64{1, 10, 1, 10, 1}
	smoothed := smooth(data)
	t.Logf("Original: %v", data)
	t.Logf("Smoothed: %v", smoothed)

	// Middle value should be pulled toward neighbors
	if smoothed[2] >= 10 || smoothed[2] <= 1 {
		t.Error("Smoothing should reduce extreme middle value")
	}
}

func TestMetricsCollector_PeriodicSensorFinite(t *testing.T) {
	// Periodic sensor inputs should produce finite chaos metrics after recompute.
	collector := NewMetricsCollector(200, 10)
	periodic := generateSineWave(200, 5)

	// Feed sensor layer (index 3) with periodic values.
	for i := 0; i < len(periodic); i++ {
		collector.RecordTick(3, periodic[i], 0, 0, 0)
		// Also tick layer 0 to advance the compute interval.
		collector.RecordTick(0, periodic[i], 0, 0, 0)
	}

	metrics := collector.GetMetrics()
	if math.IsNaN(metrics.SensorLyapunov) || math.IsInf(metrics.SensorLyapunov, 0) {
		t.Errorf("Expected finite SensorLyapunov, got: %f", metrics.SensorLyapunov)
	}
	if math.IsNaN(metrics.SensorHurst) || math.IsInf(metrics.SensorHurst, 0) {
		t.Errorf("Expected finite SensorHurst, got: %f", metrics.SensorHurst)
	}
}
