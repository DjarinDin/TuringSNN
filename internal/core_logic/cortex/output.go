package cortex

import (
	"github.com/DjarinDin/TuringSNN/internal/conf"
	"github.com/DjarinDin/TuringSNN/internal/core_logic/hub"
	"github.com/DjarinDin/TuringSNN/pkg/comms"
)

// ////////////////////////////////////////////////////////////////////////////////////////////////////////////
// ////////////////////////////////////////////////////////////////////////////////////////////////////////////
// /////////////////////////////////////////// OUTPUT LAYER ///////////////////////////////////////////////////
// ////////////////////////////////////////////////////////////////////////////////////////////////////////////
// ////////////////////////////////////////////////////////////////////////////////////////////////////////////
// ////////////////////////////////////////////////////////////////////////////////////////////////////////////
//
// The output layer is the "decision maker" of the network. It integrates information from
// all three cortical layers plus direct sensor inputs to form predictions or decisions.
// Unlike the cortical layers which process sequentially, the output layer sees the complete
// state of the network simultaneously, allowing it to make holistic judgments.
//
// The output layer has a wider input field than cortical layers:
//   - Sensor inputs: [0:NumSensors]
//   - Layer 1 outputs: [NumSensors:NumSensors+NumNeurons]
//   - Layer 2 outputs: [NumSensors+NumNeurons:NumSensors+2*NumNeurons]
//   - Layer 3 outputs: [NumSensors+2*NumNeurons:NumSensors+3*NumNeurons]
//
// This function performs three main operations:
//  1. Calculate potentials (weighted sum of all inputs)
//  2. Calculate spikes (fire if above threshold, leak otherwise)
//  3. Learn weights via STDP (modulated by holistic density)

// calculateOutputLayer processes the output layer neurons, which integrate inputs from
// all three cortical layers plus direct sensor inputs to make decisions or predictions.
func (c *Cortex) calculateOutputLayer() {
	// ******************************************************************************************** //
	// **************************  CALCULATE OUTPUT POTENTIALS  ********************************** //
	// ******************************************************************************************** //
	for outputNrn := 0; outputNrn < conf.NumOutputNeurons; outputNrn++ {
		// Output Refractory Period Check
		if c.outputRefractionCycles[outputNrn] > 0 {
			continue
		}

		if outputNrn < 2 {
			// PRE-MOTOR STAGE (Neurons 0,1): Integration of all cortical layers + sensors
			for syn := 0; syn < conf.NumSensors; syn++ {
				c.outputPotentials[outputNrn] += float64(c.outputWeights[outputNrn][syn]) * float64(c.sensorTraces[syn])
			}
			for layerIdx := 1; layerIdx <= 3; layerIdx++ {
				var traces []float64
				switch layerIdx {
				case 1:
					traces = c.layer1.OutputTraces[:]
				case 2:
					traces = c.layer2.OutputTraces[:]
				case 3:
					traces = c.layer3.OutputTraces[:]
				}
				offset := conf.NumSensors + (conf.NumNeurons * (layerIdx - 1))
				for syn := offset; syn < offset+conf.NumNeurons; syn++ {
					c.outputPotentials[outputNrn] +=
						float64(c.outputWeights[outputNrn][syn]) * traces[syn-offset]
				}
			}
		} else {
			// MOTOR STAGE (Neurons 2,3): Integration of Pre-Motor "Conviction"
			preMotorIdx := outputNrn - 2
			// The Motor neuron acts as a pure integrator of its partner's trace.
			// We scale by MaxTraceAndWeight (2^31) to bring the conviction signal
			// into the same magnitude range as the Pre-Motor integration (~2^62).
			c.outputPotentials[outputNrn] += float64(c.outputSomaTraces[preMotorIdx]) * float64(conf.MaxTraceAndWeight)
		}
	}

	// ******************************************************************************************** //
	// **************************  CALCULATE OUTPUT SPIKES  ************************************* //
	// ******************************************************************************************** //

	for outputNrn := 0; outputNrn < conf.NumOutputNeurons; outputNrn++ {
		if c.outputRefractionCycles[outputNrn] > 0 {
			c.outputRefractionCycles[outputNrn]--
			c.outputSpikes[c.cyclePhase][outputNrn] = false
			continue
		}

		// Differentiated Thresholds: Motor neurons (2,3) have much higher firing bar
		threshold := c.outputThresholds[outputNrn]
		if outputNrn >= 2 {
			threshold *= 10.0 // Motor conviction requires much more integration
		}

		if c.outputPotentials[outputNrn] < threshold {
			// LEAK
			c.outputSpikes[c.cyclePhase][outputNrn] = false
			// Differentiated Decay: Motor neurons (2,3) have high viscosity (slow decay)
			decayFactor := conf.PotentialPassiveDecayFactor // 0.8
			if outputNrn >= 2 {
				decayFactor = 0.99 // "Memory" lasts much longer in the action stage
			}
			c.outputPotentials[outputNrn] *= decayFactor
			c.outputSomaTraces[outputNrn] = c.outputSomaTraces[outputNrn] - (c.outputSomaTraces[outputNrn] / conf.TraceLeakDivisor)

			// HPIE: Sensitize
			if c.outputThresholds[outputNrn] > conf.MinThreshold {
				c.outputThresholds[outputNrn] *= conf.ThresholdDecay
			}
		} else {
			// FIRE
			c.outputSpikes[c.cyclePhase][outputNrn] = true
			c.outputThresholds[outputNrn] *= conf.ThresholdGrowth
			c.outputPotentials[outputNrn] = 0
			c.outputSomaTraces[outputNrn] = conf.MaxTraceAndWeight
			// Set refractory period: Motor neurons take a longer break
			if outputNrn >= 2 {
				c.outputRefractionCycles[outputNrn] = conf.NumRefractionCycles * 10 // 20 Ticks
			} else {
				c.outputRefractionCycles[outputNrn] = conf.NumRefractionCycles
			}

			// SYNAPTIC TAGGING (Reinforcement Phase 4)
			// Tag every input synapse that contributed significantly to this spike.
			// 1. Sensors
			for syn := 0; syn < conf.NumSensors; syn++ {
				traceVal := c.sensorTraces[syn]
				if traceVal > (conf.MaxTraceAndWeight / 10) {
					c.outputTagTicks[outputNrn][syn] = c.cortexTick
					c.outputTagStrengths[outputNrn][syn] = traceVal
				}
			}
			// 2. Layer 1
			for syn := conf.NumSensors; syn < conf.NumSensors+conf.NumNeurons; syn++ {
				traceVal := c.layer1.OutputTraces[syn-conf.NumSensors]
				if traceVal > (float64(conf.MaxTraceAndWeight) / 10.0) {
					c.outputTagTicks[outputNrn][syn] = c.cortexTick
					c.outputTagStrengths[outputNrn][syn] = int(traceVal)
				}
			}
			// 3. Layer 2
			for syn := conf.NumSensors + conf.NumNeurons; syn < conf.NumSensors+2*conf.NumNeurons; syn++ {
				traceVal := c.layer2.OutputTraces[syn-(conf.NumSensors+conf.NumNeurons)]
				if traceVal > (float64(conf.MaxTraceAndWeight) / 10.0) {
					c.outputTagTicks[outputNrn][syn] = c.cortexTick
					c.outputTagStrengths[outputNrn][syn] = int(traceVal)
				}
			}
			// 4. Layer 3
			for syn := conf.NumSensors + 2*conf.NumNeurons; syn < conf.NumSensors+3*conf.NumNeurons; syn++ {
				traceVal := c.layer3.OutputTraces[syn-(conf.NumSensors+2*conf.NumNeurons)]
				if traceVal > (float64(conf.MaxTraceAndWeight) / 10.0) {
					c.outputTagTicks[outputNrn][syn] = c.cortexTick
					c.outputTagStrengths[outputNrn][syn] = int(traceVal)
				}
			}

			// Dispatch signals based on hierarchy
			switch outputNrn {
			case 0: // Pre-Motor BUY
				if c.hub != nil {
					c.hub.Publish(hub.TopicOutputA, true)
				}
			case 1: // Pre-Motor SELL
				if c.hub != nil {
					c.hub.Publish(hub.TopicOutputB, true)
				}
			case 2: // MOTOR BUY (Execution)
				if c.hub != nil {
					c.hub.Publish(hub.TopicOutputC, true)
				}
				// Trigger action in main panel/broker
				select {
				case c.mainPanelControlCh <- comms.ControlMsg{Type: 1, IntValue: 0}:
				default:
				}
			case 3: // MOTOR SELL (Execution)
				if c.hub != nil {
					c.hub.Publish(hub.TopicOutputD, true)
				}
				select {
				case c.mainPanelControlCh <- comms.ControlMsg{Type: 1, IntValue: 1}:
				default:
				}
			}
		}
	}

	// ******************************************************************************************** //
	// **************************  OUTPUT LAYER LEARNING (STDP)  ********************************* //
	// ******************************************************************************************** //

	// HOLISTIC DENSITY MODULATION
	// The output neuron sees the entire state of the network (sensors + all three layers).
	// If the global state is chaotic (high density across all layers), it should reduce
	// learning to maintain homeostasis, even for sparse sensor inputs. This prevents the
	// output layer from overreacting to chaotic conditions.
	//
	// We calculate the average density across all input sources to get a holistic measure
	// of network chaos. High average density → slow learning, low density → fast learning.
	avgDensity := (c.MetricSensorDensity + c.MetricLayer1Density + c.MetricLayer2Density + c.MetricLayer3Density) / 4.0
	learningModulation := c.Adaptability * (1.0 - float64(avgDensity))

	// For each output neuron, update weights for all input synapses using STDP.
	// The learning rule is: if pre-synaptic spike came before post-synaptic spike (REWARD),
	// strengthen the connection. If post came before pre (PUNISH), weaken it.
	for outputNrn := 0; outputNrn < conf.NumOutputNeurons; outputNrn++ {
		// SENSOR INPUT FIELD LEARNING
		// Learn connections from sensors to this output neuron. These weights determine
		// how much the output neuron responds to raw sensory input.
		for syn := 0; syn < conf.NumSensors; syn++ {
			// Calculate learning amplitude based on trace overlap (how close in time were
			// the pre- and post-synaptic spikes?). Higher overlap = stronger learning signal.
			c.halfAmplitude = float64(c.sensorTraces[syn]) * float64(c.outputSomaTraces[outputNrn]) / (conf.MaxTrSquared)
			// Modulate by holistic density (chaos slows learning)
			c.halfAmplitude *= learningModulation

			// STDP Rule: Compare pre-synaptic trace (sensor input) to post-synaptic trace (output neuron)
			if c.sensorTraces[syn] <= c.outputSomaTraces[outputNrn] {
				// REWARD: Pre-synaptic spike came before (or at same time as) post-synaptic spike.
				// Strengthen this connection - the sensor input helped predict the output firing.
				c.outputWeights[outputNrn][syn] += int(conf.OutputDopamine * c.halfAmplitude * float64(conf.MaxTraceAndWeight-c.outputWeights[outputNrn][syn]))
			} else {
				// PUNISH: Post-synaptic spike came before pre-synaptic spike.
				// Weaken this connection - the sensor input was not predictive.
				c.outputWeights[outputNrn][syn] += int((-1.0) * conf.OutputDopamine * c.halfAmplitude * float64(c.outputWeights[outputNrn][syn]))
			}
		}

		// LAYER 1 FIELD LEARNING
		// Learn connections from Layer 1 neurons to this output neuron. These weights
		// determine how much the output neuron uses low-level features in its decisions.
		for syn := conf.NumSensors; syn < conf.NumSensors+(conf.NumNeurons*1); syn++ {
			// Map synapse index to Layer 1 neuron index
			trace := c.layer1.OutputTraces[syn-conf.NumSensors]
			c.halfAmplitude = trace * float64(c.outputSomaTraces[outputNrn]) / (conf.MaxTrSquared)
			c.halfAmplitude *= learningModulation

			if trace <= float64(c.outputSomaTraces[outputNrn]) { // REWARD
				c.outputWeights[outputNrn][syn] += int(conf.OutputDopamine * c.halfAmplitude * float64(conf.MaxTraceAndWeight-c.outputWeights[outputNrn][syn]))
			} else { // PUNISH
				c.outputWeights[outputNrn][syn] += int((-1.0) * conf.OutputDopamine * c.halfAmplitude * float64(c.outputWeights[outputNrn][syn]))
			}
		}

		// LAYER 2 FIELD LEARNING
		// Learn connections from Layer 2 neurons to this output neuron. These weights
		// determine how much the output neuron uses mid-level abstractions.
		for syn := conf.NumSensors + (conf.NumNeurons * 1); syn < conf.NumSensors+(conf.NumNeurons*2); syn++ {
			// Map synapse index to Layer 2 neuron index
			trace := c.layer2.OutputTraces[syn-(conf.NumSensors+(conf.NumNeurons*1))]
			c.halfAmplitude = trace * float64(c.outputSomaTraces[outputNrn]) / (conf.MaxTrSquared)
			c.halfAmplitude *= learningModulation

			if trace <= float64(c.outputSomaTraces[outputNrn]) { // REWARD
				c.outputWeights[outputNrn][syn] += int(conf.OutputDopamine * c.halfAmplitude * float64(conf.MaxTraceAndWeight-c.outputWeights[outputNrn][syn]))
			} else { // PUNISH
				c.outputWeights[outputNrn][syn] += int((-1.0) * conf.OutputDopamine * c.halfAmplitude * float64(c.outputWeights[outputNrn][syn]))
			}
		}

		// LAYER 3 FIELD LEARNING
		// Learn connections from Layer 3 neurons to this output neuron. These weights
		// determine how much the output neuron uses high-level abstractions and predictions.
		for syn := conf.NumSensors + (conf.NumNeurons * 2); syn < conf.NumSensors+(conf.NumNeurons*3); syn++ {
			// Map synapse index to Layer 3 neuron index
			trace := c.layer3.OutputTraces[syn-(conf.NumSensors+(conf.NumNeurons*2))]
			c.halfAmplitude = trace * float64(c.outputSomaTraces[outputNrn]) / (conf.MaxTrSquared)
			c.halfAmplitude *= learningModulation

			if trace <= float64(c.outputSomaTraces[outputNrn]) { // REWARD
				c.outputWeights[outputNrn][syn] += int(conf.OutputDopamine * c.halfAmplitude * float64(conf.MaxTraceAndWeight-c.outputWeights[outputNrn][syn]))
			} else { // PUNISH
				c.outputWeights[outputNrn][syn] += int((-1.0) * conf.OutputDopamine * c.halfAmplitude * float64(c.outputWeights[outputNrn][syn]))
			}
		}
	}
}
