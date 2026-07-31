// Package chaos provides mathematical chaos analysis tools for SNN dynamics.
//
// This package implements metrics that can help tune neural network parameters
// by measuring the "edge of chaos" - the optimal regime for computation.
//
// Key Metrics:
//
//   - Lyapunov Exponent: Measures trajectory divergence (chaos indicator)
//   - Hurst Exponent: Measures long-range memory/persistence
//   - Wavelet Transform: Multi-resolution frequency analysis
//
// Usage with SNN:
//
// The typical workflow is to sample spike train data or firing rates over time,
// then compute these metrics to assess network health:
//
//	le := chaos.LyapunovExponent(firingRates)
//	if le > 0.5 {
//	    // Network is too chaotic, reduce dopamine
//	} else if le < -0.5 {
//	    // Network is too stable/dead, increase spontaneous firing
//	}
//
//	he := chaos.HurstExponent(firingRates)
//	if he > 0.7 {
//	    // Strong temporal correlation - network has learned structure
//	} else if he < 0.4 {
//	    // Anti-correlated - possibly pathological oscillation
//	}
package chaos
