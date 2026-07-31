package cortex

import (
	"github.com/DjarinDin/TuringSNN/internal/conf"
	"log"
	"runtime/debug"
)

/*
SENSOR RETINA ENCODING (128-sensor unified array)

The SNN receives all external inputs through a 128-sensor "Retina." These arrive
asynchronously as integer IDs and are collected in the accumulator before being
moved into the synchronous spike matrix for processing.

The cortex itself is agnostic to what feeds each channel — sensor-region
conventions (which ranges mean what) are established by whichever
WorldSource(s) are registered with backend.go's NewBackend, not by the
cortex. This repository registers only the built-in synthetic Sim
(internal/core_logic/sim), which drives patterns across the sensor range
on its own.
*/

// accumulateSensorSignals is a background goroutine that listens for sensor
// events from external pipes (Pipes/Pps) and populates the accumulator.
func (c *Cortex) accumulateSensorSignals() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC in accumulateSensorSignals: %v\nStack: %s", r, debug.Stack())
		}
	}()
	for sensorID := range c.cortexSensorCh {
		c.accumulator[sensorID] = true
		select {
		case c.rawSensorSpikesCh <- sensorID: // Mirror to monitor
		default:
		}
		c.signalsReceivedThisHeartbeat++
		c.totalSignalsReceived++
	}
}

// moveAccumulatorToSpikeMatrixAndReset transfers all accumulated sensor spikes
// into the current cycle's spike matrix for L1 and then clears the accumulator.
func (c *Cortex) moveAccumulatorToSpikeMatrixAndReset() {
	for sensorID, signal := range c.accumulator {
		if signal {
			c.layer1.Spikes[c.cyclePhase][sensorID] = true
			c.signalsAccumulatedThisHeartbeat++
			c.totalSignalsAccumulated++
		} else {
			c.layer1.Spikes[c.cyclePhase][sensorID] = false
		}
	}
	c.resetAccumulator()
}

// resetAccumulator clears the sensor integration buffer.
func (c *Cortex) resetAccumulator() {
	for sensorID := 0; sensorID < conf.NumSensors; sensorID++ {
		c.accumulator[sensorID] = false
	}
}

// updateSensorTraces advances the global temporal history for all receptors.
func (c *Cortex) updateSensorTraces() {
	for s := 0; s < conf.NumSensors; s++ {
		if c.layer1.Spikes[c.cyclePhase][s] {
			c.sensorTraces[s] = conf.MaxTraceAndWeight
		} else {
			c.sensorTraces[s] -= c.sensorTraces[s] / conf.TraceLeakDivisor
		}
	}
}

// calculateMetricDensity computes the fraction of active (spiking) sites
// in a boolean vector. Used to modulate learning based on chaos levels.
func (c *Cortex) calculateMetricDensity(spikes []bool) float32 {
	if len(spikes) == 0 {
		return 0
	}
	count := 0
	for _, s := range spikes {
		if s {
			count++
		}
	}
	return float32(count) / float32(len(spikes))
}
