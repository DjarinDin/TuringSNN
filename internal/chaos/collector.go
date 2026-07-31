package chaos

import (
	"math"
	"sync"
)

// MetricsCollector maintains rolling buffers and computes chaos metrics
// for the neural network. It tracks firing rates, thresholds, weight entropy,
// and potential variance for each layer.
// Layer indices: 0=L1, 1=L2, 2=L3, 3=Sensor, 4=Focus
type MetricsCollector struct {
	mu sync.RWMutex

	// Rolling buffers for each layer (100 ticks each)
	// Indices: 0=L1, 1=L2, 2=L3, 3=Sensor, 4=Focus
	firingRates [5][]float64 // Spikes per neuron this tick
	thresholds  [5][]float64 // Mean threshold deviation
	entropy     [5][]float64 // Weight entropy
	potVar      [5][]float64 // Potential variance

	bufferSize int
	tickCount  int

	// Cached computed metrics (updated every computeInterval ticks)
	cachedMetrics   *Metrics
	computeInterval int
}

// Metrics contains the computed chaos metrics for the network
type Metrics struct {
	// Lyapunov exponent for each layer's firing rate
	// Indices: 0=L1, 1=L2, 2=L3
	Lyapunov [3]float64 `json:"lyapunov"`

	// Hurst exponent for each layer's firing rate
	Hurst [3]float64 `json:"hurst"`

	// Sensor layer metrics
	SensorLyapunov float64 `json:"sensorLyapunov"`
	SensorHurst    float64 `json:"sensorHurst"`

	// Focus neuron metrics
	FocusLyapunov float64 `json:"focusLyapunov"`
	FocusHurst    float64 `json:"focusHurst"`

	// Mean weight entropy for each layer (higher = more random, lower = specialized)
	Entropy [3]float64 `json:"entropy"`

	// Mean threshold deviation from base (homeostasis indicator)
	ThresholdDev [3]float64 `json:"thresholdDev"`

	// Potential variance (network differentiation)
	PotentialVar [3]float64 `json:"potentialVar"`

	// Network-wide aggregates
	AvgLyapunov float64 `json:"avgLyapunov"`
	AvgHurst    float64 `json:"avgHurst"`
	AvgEntropy  float64 `json:"avgEntropy"`
}

// NewMetricsCollector creates a new metrics collector
// bufferSize: number of ticks to keep in rolling buffer (e.g., 100)
// computeInterval: how often to recompute chaos metrics (e.g., 10)
func NewMetricsCollector(bufferSize, computeInterval int) *MetricsCollector {
	mc := &MetricsCollector{
		bufferSize:      bufferSize,
		computeInterval: computeInterval,
		cachedMetrics:   &Metrics{},
	}

	// Initialize buffers for all 5 layers
	for i := 0; i < 5; i++ {
		mc.firingRates[i] = make([]float64, 0, bufferSize)
		mc.thresholds[i] = make([]float64, 0, bufferSize)
		mc.entropy[i] = make([]float64, 0, bufferSize)
		mc.potVar[i] = make([]float64, 0, bufferSize)
	}

	return mc
}

// RecordTick records metrics for a single tick
// layer: 0=L1, 1=L2, 2=L3, 3=Sensor, 4=Focus
// firingRate: spikes/neuron this tick (0-1) or normalized potential for focus
// thresholdDev: mean threshold deviation from base
// weightEntropy: Shannon entropy of weight distribution
// potentialVar: variance of membrane potentials
func (mc *MetricsCollector) RecordTick(layer int, firingRate, thresholdDev, weightEntropy, potentialVar float64) {
	if layer < 0 || layer > 4 {
		return
	}

	mc.mu.Lock()
	defer mc.mu.Unlock()

	// Add to rolling buffers
	mc.firingRates[layer] = appendRolling(mc.firingRates[layer], firingRate, mc.bufferSize)
	mc.thresholds[layer] = appendRolling(mc.thresholds[layer], thresholdDev, mc.bufferSize)
	mc.entropy[layer] = appendRolling(mc.entropy[layer], weightEntropy, mc.bufferSize)
	mc.potVar[layer] = appendRolling(mc.potVar[layer], potentialVar, mc.bufferSize)

	// Only increment tick count once (when layer 0 is recorded)
	if layer == 0 {
		mc.tickCount++

		// Recompute metrics every N ticks
		if mc.tickCount%mc.computeInterval == 0 {
			mc.recomputeMetrics()
		}
	}
}

// GetMetrics returns the most recently computed metrics
func (mc *MetricsCollector) GetMetrics() *Metrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.cachedMetrics
}

// recomputeMetrics calculates chaos metrics from the rolling buffers
// Called with lock held
func (mc *MetricsCollector) recomputeMetrics() {
	m := &Metrics{}

	// Layers L1, L2, L3 (indices 0, 1, 2)
	for i := 0; i < 3; i++ {
		// Need sufficient data for chaos metrics
		if len(mc.firingRates[i]) >= 20 {
			m.Lyapunov[i] = LyapunovExponent(mc.firingRates[i])
			m.Hurst[i] = HurstExponent(mc.firingRates[i])
		}

		// Simple averages for recent values
		if len(mc.entropy[i]) > 0 {
			m.Entropy[i] = mean(mc.entropy[i])
		}
		if len(mc.thresholds[i]) > 0 {
			m.ThresholdDev[i] = mean(mc.thresholds[i])
		}
		if len(mc.potVar[i]) > 0 {
			m.PotentialVar[i] = mean(mc.potVar[i])
		}
	}

	// Sensor layer (index 3) - tracks sensor INDEX pattern, not density
	if len(mc.firingRates[3]) >= 20 {
		m.SensorLyapunov = LyapunovExponent(mc.firingRates[3])
		m.SensorHurst = HurstExponent(mc.firingRates[3])
	}

	// Focus neuron (index 4)
	if len(mc.firingRates[4]) >= 20 {
		m.FocusLyapunov = LyapunovExponent(mc.firingRates[4])
		m.FocusHurst = HurstExponent(mc.firingRates[4])
	}

	// Network-wide aggregates (layers only, not sensor/focus)
	m.AvgLyapunov = (m.Lyapunov[0] + m.Lyapunov[1] + m.Lyapunov[2]) / 3
	m.AvgHurst = (m.Hurst[0] + m.Hurst[1] + m.Hurst[2]) / 3
	m.AvgEntropy = (m.Entropy[0] + m.Entropy[1] + m.Entropy[2]) / 3

	mc.cachedMetrics = m
}

// appendRolling adds a value to a slice, maintaining max size
func appendRolling(slice []float64, val float64, maxSize int) []float64 {
	slice = append(slice, val)
	if len(slice) > maxSize {
		slice = slice[1:]
	}
	return slice
}

// mean calculates the arithmetic mean of a slice
func mean(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	var sum float64
	for _, v := range data {
		sum += v
	}
	return sum / float64(len(data))
}

// CalculateWeightEntropy computes Shannon entropy of a weight array
// Returns a value where 0 = all weights identical, higher = more distributed
func CalculateWeightEntropy(weights []float64) float64 {
	if len(weights) == 0 {
		return 0
	}

	// Normalize to probabilities
	var sum float64
	for _, w := range weights {
		sum += math.Abs(w)
	}
	if sum == 0 {
		return 0
	}

	var entropy float64
	for _, w := range weights {
		p := math.Abs(w) / sum
		if p > 0 {
			entropy -= p * math.Log2(p)
		}
	}

	return entropy
}

// CalculatePotentialVariance computes the variance of membrane potentials
func CalculatePotentialVariance(potentials []float64) float64 {
	if len(potentials) < 2 {
		return 0
	}

	m := mean(potentials)
	var sumSq float64
	for _, p := range potentials {
		diff := p - m
		sumSq += diff * diff
	}

	return sumSq / float64(len(potentials)-1)
}
