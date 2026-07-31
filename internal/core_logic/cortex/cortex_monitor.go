package cortex

import (
	"github.com/DjarinDin/TuringSNN/internal/conf"
	"github.com/DjarinDin/TuringSNN/internal/core_logic/hub"
	"github.com/DjarinDin/TuringSNN/pkg/comms"
	"math"
	"strconv"
)

// sendStateToGUI broadcasts the current neural state (potentials, weights, traces)
// to the GUI via non-blocking channel sends.
// Traces are selected based on traceDisplayMode (Short/Mid/Long).
func (c *Cortex) sendStateToGUI() {
	select {
	case c.layer1InputTracesCh <- c.getTraceSlice(&c.layer1, conf.QuadPhenomenonStart, conf.QuadPhenomenonEnd):
	default:
	}

	select {
	case c.layer1InputWeightsCh <- c.layer1.Weights[c.focusNeuron][conf.QuadPhenomenonStart:conf.QuadPhenomenonEnd]:
	default:
	}
	select {
	case c.layer1ResonanceWeightsCh <- c.layer1.Weights[c.focusNeuron][conf.QuadResonanceStart:conf.QuadResonanceEnd]:
	default:
	}
	select {
	case c.layer1ResonanceTracesCh <- c.getTraceSlice(&c.layer1, conf.QuadResonanceStart, conf.QuadResonanceEnd):
	default:
	}
	select {
	case c.layer1ZeitgeistWeightsCh <- c.layer1.Weights[c.focusNeuron][conf.QuadZeitgeistStart:conf.QuadZeitgeistEnd]:
	default:
	}
	select {
	case c.layer1ZeitgeistTracesCh <- c.getTraceSlice(&c.layer1, conf.QuadZeitgeistStart, conf.QuadZeitgeistEnd):
	default:
	}
	select {
	case c.layer1FeedbackWeightsCh <- c.layer1.Weights[c.focusNeuron][conf.QuadGestaltStart:conf.QuadGestaltEnd]:
	default:
	}
	select {
	case c.layer1FeedbackTracesCh <- c.getTraceSlice(&c.layer1, conf.QuadGestaltStart, conf.QuadGestaltEnd):
	default:
	}
	select {
	case c.layer1OutputTracesCh <- c.layer1.OutputTraces[:]:
	default:
	}

	select {
	case c.layer2InputWeightsCh <- c.layer2.Weights[c.focusNeuron][conf.QuadPhenomenonStart:conf.QuadPhenomenonEnd]:
	default:
	}
	select {
	case c.layer2InputTracesCh <- c.getTraceSlice(&c.layer2, conf.QuadPhenomenonStart, conf.QuadPhenomenonEnd):
	default:
	}
	select {
	case c.layer2ResonanceWeightsCh <- c.layer2.Weights[c.focusNeuron][conf.QuadResonanceStart:conf.QuadResonanceEnd]:
	default:
	}
	select {
	case c.layer2ResonanceTracesCh <- c.getTraceSlice(&c.layer2, conf.QuadResonanceStart, conf.QuadResonanceEnd):
	default:
	}
	select {
	case c.layer2ZeitgeistWeightsCh <- c.layer2.Weights[c.focusNeuron][conf.QuadZeitgeistStart:conf.QuadZeitgeistEnd]:
	default:
	}
	select {
	case c.layer2ZeitgeistTracesCh <- c.getTraceSlice(&c.layer2, conf.QuadZeitgeistStart, conf.QuadZeitgeistEnd):
	default:
	}
	select {
	case c.layer2FeedbackWeightsCh <- c.layer2.Weights[c.focusNeuron][conf.QuadGestaltStart:conf.QuadGestaltEnd]:
	default:
	}
	select {
	case c.layer2FeedbackTracesCh <- c.getTraceSlice(&c.layer2, conf.QuadGestaltStart, conf.QuadGestaltEnd):
	default:
	}
	select {
	case c.layer2OutputTracesCh <- c.layer2.OutputTraces[:]:
	default:
	}

	select {
	case c.layer3InputWeightsCh <- c.layer3.Weights[c.focusNeuron][conf.QuadPhenomenonStart:conf.QuadPhenomenonEnd]:
	default:
	}
	select {
	case c.layer3InputTracesCh <- c.getTraceSlice(&c.layer3, conf.QuadPhenomenonStart, conf.QuadPhenomenonEnd):
	default:
	}
	select {
	case c.layer3ResonanceWeightsCh <- c.layer3.Weights[c.focusNeuron][conf.QuadResonanceStart:conf.QuadResonanceEnd]:
	default:
	}
	select {
	case c.layer3ResonanceTracesCh <- c.getTraceSlice(&c.layer3, conf.QuadResonanceStart, conf.QuadResonanceEnd):
	default:
	}
	select {
	case c.layer3ZeitgeistWeightsCh <- c.layer3.Weights[c.focusNeuron][conf.QuadZeitgeistStart:conf.QuadZeitgeistEnd]:
	default:
	}
	select {
	case c.layer3ZeitgeistTracesCh <- c.getTraceSlice(&c.layer3, conf.QuadZeitgeistStart, conf.QuadZeitgeistEnd):
	default:
	}
	select {
	case c.layer3FeedbackWeightsCh <- c.layer3.Weights[c.focusNeuron][conf.QuadGestaltStart:conf.QuadGestaltEnd]:
	default:
	}
	select {
	case c.layer3FeedbackTracesCh <- c.getTraceSlice(&c.layer3, conf.QuadGestaltStart, conf.QuadGestaltEnd):
	default:
	}
	select {
	case c.layer3OutputTracesCh <- c.layer3.OutputTraces[:]:
	default:
	}

	select {
	case c.outputAWeightsCh <- c.outputWeights[0][:]:
	default:
	}
	select {
	case c.outputBWeightsCh <- c.outputWeights[1][:]:
	default:
	}
	select {
	case c.outputCWeightsCh <- c.outputWeights[2][:]:
	default:
	}
	select {
	case c.outputDWeightsCh <- c.outputWeights[3][:]:
	default:
	}

	if c.focusLayer == 1 {
		select {
		case c.focusPotHistoryCh <- comms.BoolMsg{Pot: c.layer1.Potentials[c.focusNeuron], Spikes: c.layer1.Spikes[c.cyclePhase][conf.QuadGestaltStart+c.focusNeuron]}:
		default:
		}
	} else if c.focusLayer == 2 {
		select {
		case c.focusPotHistoryCh <- comms.BoolMsg{Pot: c.layer2.Potentials[c.focusNeuron], Spikes: c.layer2.Spikes[c.cyclePhase][conf.QuadGestaltStart+c.focusNeuron]}:
		default:
		}
	} else if c.focusLayer == 3 {
		select {
		case c.focusPotHistoryCh <- comms.BoolMsg{Pot: c.layer3.Potentials[c.focusNeuron], Spikes: c.layer3.Spikes[c.cyclePhase][conf.QuadGestaltStart+c.focusNeuron]}:
		default:
		}
	}

	// Read output neuron spikes from spike matrix at current cycle
	outputASpike := c.outputSpikes[c.cyclePhase][0]
	outputBSpike := c.outputSpikes[c.cyclePhase][1]
	outputCSpike := c.outputSpikes[c.cyclePhase][2]
	outputDSpike := c.outputSpikes[c.cyclePhase][3]

	select {
	case c.outputAHistoryCh <- comms.BoolMsg{Pot: c.outputPotentials[0], Spikes: outputASpike}:
	default:
	}
	select {
	case c.outputBHistoryCh <- comms.BoolMsg{Pot: c.outputPotentials[1], Spikes: outputBSpike}:
	default:
	}
	select {
	case c.outputCHistoryCh <- comms.BoolMsg{Pot: c.outputPotentials[2], Spikes: outputCSpike}:
	default:
	}
	select {
	case c.outputDHistoryCh <- comms.BoolMsg{Pot: c.outputPotentials[3], Spikes: outputDSpike}:
	default:
	}

	// Extract current cycle spikes from Gestalt field [192-255] for each layer
	layer1Spikes := make([]bool, conf.NumNeurons)
	layer2Spikes := make([]bool, conf.NumNeurons)
	layer3Spikes := make([]bool, conf.NumNeurons)
	for nrn := 0; nrn < conf.NumNeurons; nrn++ {
		gestaltSyn := conf.QuadGestaltStart + nrn // [192-255]
		layer1Spikes[nrn] = c.layer1.Spikes[c.cyclePhase][gestaltSyn]
		layer2Spikes[nrn] = c.layer2.Spikes[c.cyclePhase][gestaltSyn]
		layer3Spikes[nrn] = c.layer3.Spikes[c.cyclePhase][gestaltSyn]
	}
	select {
	case c.layer1PotsAndSpikesCh <- comms.PotentialsAndSpikesMsg{Pots: c.layer1.Potentials[:], Spikes: layer1Spikes}:
	default:
	}
	select {
	case c.layer2PotsAndSpikesCh <- comms.PotentialsAndSpikesMsg{Pots: c.layer2.Potentials[:], Spikes: layer2Spikes}:
	default:
	}
	select {
	case c.layer3PotsAndSpikesCh <- comms.PotentialsAndSpikesMsg{Pots: c.layer3.Potentials[:], Spikes: layer3Spikes}:
	default:
	}
}

// getTraceSlice returns the appropriate trace slice for a layer based on the current display mode.
// mode: 0=Short (InputTraces), 1=Mid (MidTermTraces), 2=Long (LongTermTraces)
//
// IMPORTANT: Mid/Long traces are small floats (typically 0-100), while Short traces are large
// integers (0 to MaxTraceAndWeight ~2^31). The GUI histogram normalizes against MaxTraceAndWeight,
// so mid/long traces must be auto-scaled to be visible.
func (c *Cortex) getTraceSlice(l *layer, start, end int) []float64 {
	var src []float64
	needsScaling := false

	switch c.traceDisplayMode {
	case comms.TraceDisplayMid:
		src = l.MidTermTraces[c.focusNeuron][start:end]
		needsScaling = true
	case comms.TraceDisplayLong:
		src = l.LongTermTraces[c.focusNeuron][start:end]
		needsScaling = true
	default: // TraceDisplayShort
		src = l.InputTraces[c.focusNeuron][start:end]
	}

	if !needsScaling {
		return src
	}

	// Auto-scale mid/long traces: find max absolute value, then scale so max maps to
	// half of MaxTraceAndWeight (leaves headroom for peaks).
	maxAbs := 0.0
	for _, v := range src {
		if abs := math.Abs(v); abs > maxAbs {
			maxAbs = abs
		}
	}

	// Avoid division by zero; if all traces are zero, return zeros
	if maxAbs < 1e-12 {
		return make([]float64, end-start)
	}

	// Scale factor: map maxAbs -> 0.5 * MaxTraceAndWeight
	scale := 0.5 * float64(conf.MaxTraceAndWeight) / maxAbs
	result := make([]float64, end-start)
	for i, v := range src {
		result[i] = v * scale
	}

	return result
}

// sendStateToHub broadcasts the current neural state via hub topics.
// This allows the GUI to run in a separate process without direct channels.
func (c *Cortex) sendStateToHub() {
	// Skip publish if the hub isn't wired (keeps core logic independent).
	if c.hub == nil {
		return
	}

	// Publish traces and weights for each layer (focus neuron slices).
	// Traces are selected based on traceDisplayMode (Short/Mid/Long).
	c.hub.Publish(hub.TopicLayer1InputTraces, c.getTraceSlice(&c.layer1, conf.QuadPhenomenonStart, conf.QuadPhenomenonEnd))
	c.hub.Publish(hub.TopicLayer1InputWeights, c.layer1.Weights[c.focusNeuron][conf.QuadPhenomenonStart:conf.QuadPhenomenonEnd])
	c.hub.Publish(hub.Topic("layer1.resonance.weights"), c.layer1.Weights[c.focusNeuron][conf.QuadResonanceStart:conf.QuadResonanceEnd])
	c.hub.Publish(hub.Topic("layer1.resonance.traces"), c.getTraceSlice(&c.layer1, conf.QuadResonanceStart, conf.QuadResonanceEnd))
	c.hub.Publish(hub.Topic("layer1.zeitgeist.weights"), c.layer1.Weights[c.focusNeuron][conf.QuadZeitgeistStart:conf.QuadZeitgeistEnd])
	c.hub.Publish(hub.Topic("layer1.zeitgeist.traces"), c.getTraceSlice(&c.layer1, conf.QuadZeitgeistStart, conf.QuadZeitgeistEnd))
	c.hub.Publish(hub.TopicLayer1FeedbackWeights, c.layer1.Weights[c.focusNeuron][conf.QuadGestaltStart:conf.QuadGestaltEnd])
	c.hub.Publish(hub.TopicLayer1FeedbackTraces, c.getTraceSlice(&c.layer1, conf.QuadGestaltStart, conf.QuadGestaltEnd))
	c.hub.Publish(hub.TopicLayer1OutputTraces, c.layer1.OutputTraces[:])

	c.hub.Publish(hub.TopicLayer2InputWeights, c.layer2.Weights[c.focusNeuron][conf.QuadPhenomenonStart:conf.QuadPhenomenonEnd])
	c.hub.Publish(hub.TopicLayer2InputTraces, c.getTraceSlice(&c.layer2, conf.QuadPhenomenonStart, conf.QuadPhenomenonEnd))
	c.hub.Publish(hub.Topic("layer2.resonance.weights"), c.layer2.Weights[c.focusNeuron][conf.QuadResonanceStart:conf.QuadResonanceEnd])
	c.hub.Publish(hub.Topic("layer2.resonance.traces"), c.getTraceSlice(&c.layer2, conf.QuadResonanceStart, conf.QuadResonanceEnd))
	c.hub.Publish(hub.Topic("layer2.zeitgeist.weights"), c.layer2.Weights[c.focusNeuron][conf.QuadZeitgeistStart:conf.QuadZeitgeistEnd])
	c.hub.Publish(hub.Topic("layer2.zeitgeist.traces"), c.getTraceSlice(&c.layer2, conf.QuadZeitgeistStart, conf.QuadZeitgeistEnd))
	c.hub.Publish(hub.TopicLayer2FeedbackWeights, c.layer2.Weights[c.focusNeuron][conf.QuadGestaltStart:conf.QuadGestaltEnd])
	c.hub.Publish(hub.TopicLayer2FeedbackTraces, c.getTraceSlice(&c.layer2, conf.QuadGestaltStart, conf.QuadGestaltEnd))
	c.hub.Publish(hub.TopicLayer2OutputTraces, c.layer2.OutputTraces[:])

	c.hub.Publish(hub.TopicLayer3InputWeights, c.layer3.Weights[c.focusNeuron][conf.QuadPhenomenonStart:conf.QuadPhenomenonEnd])
	c.hub.Publish(hub.TopicLayer3InputTraces, c.getTraceSlice(&c.layer3, conf.QuadPhenomenonStart, conf.QuadPhenomenonEnd))
	c.hub.Publish(hub.Topic("layer3.resonance.weights"), c.layer3.Weights[c.focusNeuron][conf.QuadResonanceStart:conf.QuadResonanceEnd])
	c.hub.Publish(hub.Topic("layer3.resonance.traces"), c.getTraceSlice(&c.layer3, conf.QuadResonanceStart, conf.QuadResonanceEnd))
	c.hub.Publish(hub.Topic("layer3.zeitgeist.weights"), c.layer3.Weights[c.focusNeuron][conf.QuadZeitgeistStart:conf.QuadZeitgeistEnd])
	c.hub.Publish(hub.Topic("layer3.zeitgeist.traces"), c.getTraceSlice(&c.layer3, conf.QuadZeitgeistStart, conf.QuadZeitgeistEnd))
	c.hub.Publish(hub.TopicLayer3FeedbackWeights, c.layer3.Weights[c.focusNeuron][conf.QuadGestaltStart:conf.QuadGestaltEnd])
	c.hub.Publish(hub.TopicLayer3FeedbackTraces, c.getTraceSlice(&c.layer3, conf.QuadGestaltStart, conf.QuadGestaltEnd))
	c.hub.Publish(hub.TopicLayer3OutputTraces, c.layer3.OutputTraces[:])

	// Publish output weights so remote GUIs can render them.
	c.hub.Publish(hub.TopicOutputAWeights, c.outputWeights[0][:])
	c.hub.Publish(hub.TopicOutputBWeights, c.outputWeights[1][:])
	c.hub.Publish(hub.TopicOutputCWeights, c.outputWeights[2][:])
	c.hub.Publish(hub.TopicOutputDWeights, c.outputWeights[3][:])

	// Publish focus neuron history and output histories.
	if c.focusLayer == 1 {
		c.hub.Publish(hub.TopicFocusPotentialHistory, comms.BoolMsg{Pot: c.layer1.Potentials[c.focusNeuron], Spikes: c.layer1.Spikes[c.cyclePhase][conf.QuadGestaltStart+c.focusNeuron]})
	} else if c.focusLayer == 2 {
		c.hub.Publish(hub.TopicFocusPotentialHistory, comms.BoolMsg{Pot: c.layer2.Potentials[c.focusNeuron], Spikes: c.layer2.Spikes[c.cyclePhase][conf.QuadGestaltStart+c.focusNeuron]})
	} else if c.focusLayer == 3 {
		c.hub.Publish(hub.TopicFocusPotentialHistory, comms.BoolMsg{Pot: c.layer3.Potentials[c.focusNeuron], Spikes: c.layer3.Spikes[c.cyclePhase][conf.QuadGestaltStart+c.focusNeuron]})
	}

	// Publish output histories using the current cycle's spike matrix.
	c.hub.Publish(hub.TopicOutputAHistory, comms.BoolMsg{Pot: c.outputPotentials[0], Spikes: c.outputSpikes[c.cyclePhase][0]})
	c.hub.Publish(hub.TopicOutputBHistory, comms.BoolMsg{Pot: c.outputPotentials[1], Spikes: c.outputSpikes[c.cyclePhase][1]})
	c.hub.Publish(hub.TopicOutputCHistory, comms.BoolMsg{Pot: c.outputPotentials[2], Spikes: c.outputSpikes[c.cyclePhase][2]})
	c.hub.Publish(hub.TopicOutputDHistory, comms.BoolMsg{Pot: c.outputPotentials[3], Spikes: c.outputSpikes[c.cyclePhase][3]})

	// Publish layer potentials/spikes for heatmap rendering.
	layer1Spikes := make([]bool, conf.NumNeurons)
	layer2Spikes := make([]bool, conf.NumNeurons)
	layer3Spikes := make([]bool, conf.NumNeurons)
	for nrn := 0; nrn < conf.NumNeurons; nrn++ {
		gestaltSyn := conf.QuadGestaltStart + nrn
		layer1Spikes[nrn] = c.layer1.Spikes[c.cyclePhase][gestaltSyn]
		layer2Spikes[nrn] = c.layer2.Spikes[c.cyclePhase][gestaltSyn]
		layer3Spikes[nrn] = c.layer3.Spikes[c.cyclePhase][gestaltSyn]
	}
	c.hub.Publish(hub.TopicLayer1Potentials, comms.PotentialsAndSpikesMsg{Pots: c.layer1.Potentials[:], Spikes: layer1Spikes})
	c.hub.Publish(hub.TopicLayer2Potentials, comms.PotentialsAndSpikesMsg{Pots: c.layer2.Potentials[:], Spikes: layer2Spikes})
	c.hub.Publish(hub.TopicLayer3Potentials, comms.PotentialsAndSpikesMsg{Pots: c.layer3.Potentials[:], Spikes: layer3Spikes})

	// Publish raw layer spike arrays for chaos metrics (MetricsHandler expects these topics).
	c.hub.Publish(hub.TopicLayer1Spikes, layer1Spikes)
	c.hub.Publish(hub.TopicLayer2Spikes, layer2Spikes)
	c.hub.Publish(hub.TopicLayer3Spikes, layer3Spikes)
}

// sendLowLevelTelemetryToGUI sends performance and throughput statistics.
func (c *Cortex) sendLowLevelTelemetryToHub() {
	// Skip publish if the hub isn't wired (keeps core logic independent).
	if c.hub == nil {
		return
	}

	// Publish per-heartbeat stats for remote dashboards.
	c.hub.Publish(hub.TopicPerHeartbeatStats, comms.StringMsg{Type: 0, Value: strconv.Itoa(c.cortexTick)})
	c.hub.Publish(hub.TopicPerHeartbeatStats, comms.StringMsg{Type: 1, Value: strconv.Itoa(c.cortexTick - c.lastCortexTick)})
	c.hub.Publish(hub.TopicPerHeartbeatStats, comms.StringMsg{Type: 2, Value: strconv.Itoa(c.signalsReceivedThisHeartbeat - c.signalsAccumulatedThisHeartbeat)})

	// Publish cumulative stats to track long-term throughput.
	c.hub.Publish(hub.TopicCumulativeStats, comms.StringMsg{Type: 0, Value: strconv.Itoa(int(c.totalSignalsReceived))})
	c.hub.Publish(hub.TopicCumulativeStats, comms.StringMsg{Type: 1, Value: strconv.Itoa(int(c.totalSignalsAccumulated))})
	c.hub.Publish(hub.TopicCumulativeStats, comms.StringMsg{
		Type: 2,
		Value: strconv.Itoa(int(c.totalSignalsReceived-c.totalSignalsAccumulated)) +
			" [" + strconv.FormatFloat(
			100.0*float64(c.totalSignalsReceived-c.totalSignalsAccumulated)/float64(c.totalSignalsReceived),
			'f', 1, 32) + " %]",
	})

	// Publish hub drop stats for observability.
	if totalDrops, byTopic := c.hub.GetDropStats(); totalDrops > 0 {
		dropMap := make(map[string]uint64, len(byTopic))
		for topic, count := range byTopic {
			dropMap[string(topic)] = count
		}
		c.hub.Publish(hub.TopicHubDrops, comms.HubDropStats{
			Total:   totalDrops,
			ByTopic: dropMap,
		})
	}
}

// sendLearningStateToHub publishes high-level learning controls for debugging.
func (c *Cortex) sendLearningStateToHub() {
	// Skip publish if the hub isn't wired (keeps runtime behavior unchanged).
	if c.hub == nil {
		return
	}

	// Emit a compact snapshot so tools can track learning dynamics over time.
	residualMean, residualMax := c.focusResidualStats()
	c.hub.Publish(hub.TopicLearningState, comms.LearningStateMsg{
		Tick:                      c.cortexTick,
		Dopamine:                  c.dopamine,
		Adaptability:              c.Adaptability,
		SpontaneousFire:           c.spontaneousFire,
		ShortTraceHalfLifeSeconds: conf.ShortTraceHalfLifeSeconds,
		MidTraceHalfLifeSeconds:   conf.MidTraceHalfLifeSeconds,
		LongTraceHalfLifeSeconds:  conf.LongTraceHalfLifeSeconds,
		LongEligibilityGate:       conf.LongEligibilityGate,
		ResidualAbsMeanFocus:      residualMean,
		ResidualAbsMaxFocus:       residualMax,
	})

	// Publish which trace type is currently being visualized (Short/Mid/Long).
	c.hub.Publish(hub.TopicTraceDisplayMode, c.traceDisplayMode)
}

func (c *Cortex) focusResidualStats() (float64, float64) {
	var target *layer
	switch c.focusLayer {
	case 1:
		target = &c.layer1
	case 2:
		target = &c.layer2
	case 3:
		target = &c.layer3
	default:
		return 0, 0
	}

	sum := 0.0
	max := 0.0
	for syn := 0; syn < conf.NumTotSynapses; syn++ {
		val := math.Abs(target.WeightResidual[c.focusNeuron][syn])
		sum += val
		if val > max {
			max = val
		}
	}
	if conf.NumTotSynapses == 0 {
		return 0, 0
	}
	return sum / float64(conf.NumTotSynapses), max
}

// sendSpikesToGUI sends the current cycle's spikes for all Quad fields to the GUI.
func (c *Cortex) sendSpikesToHub(layer3VisibleFeedback []bool) {
	// Skip publish if the hub isn't wired (keeps core logic independent).
	if c.hub == nil {
		return
	}

	// Publish densities to Hub (throttled to 10Hz).
	if c.cortexTick%10 == 0 {
		c.hub.Publish(hub.TopicDensities, []float32{
			c.MetricSensorDensity,
			c.MetricLayer1Density,
			c.MetricLayer2Density,
			c.MetricLayer3Density,
		})
	}

	// Layer 1 spikes (phenomenon, resonance, zeitgeist, gestalt).
	c.hub.Publish(hub.TopicSensorSpikes, c.layer1.Spikes[c.cyclePhase][0:conf.NumSensors])
	c.hub.Publish(hub.Topic("layer1.resonance.spikes"), c.layer1.getResonanceSpikes(c.cyclePhase, c.focusNeuron))
	c.hub.Publish(hub.Topic("layer1.zeitgeist.spikes"), c.layer1.getZeitgeistSpikes(c.cyclePhase))
	c.hub.Publish(hub.Topic("layer1.feedback.spikes"), c.layer1.getVisibleFeedbackSpikes(c.cyclePhase))

	// Layer 2 spikes (phenomenon, resonance, zeitgeist, gestalt).
	c.hub.Publish(hub.TopicLayer2InputSpikes, c.layer2.Spikes[c.cyclePhase][0:conf.NumSensors])
	c.hub.Publish(hub.Topic("layer2.resonance.spikes"), c.layer2.getResonanceSpikes(c.cyclePhase, c.focusNeuron))
	c.hub.Publish(hub.Topic("layer2.zeitgeist.spikes"), c.layer2.getZeitgeistSpikes(c.cyclePhase))
	c.hub.Publish(hub.Topic("layer2.feedback.spikes"), c.layer2.getVisibleFeedbackSpikes(c.cyclePhase))

	// Layer 3 spikes (phenomenon, resonance, zeitgeist, gestalt).
	c.hub.Publish(hub.TopicLayer3InputSpikes, c.layer3.Spikes[c.cyclePhase][0:conf.NumSensors])
	c.hub.Publish(hub.Topic("layer3.resonance.spikes"), c.layer3.getResonanceSpikes(c.cyclePhase, c.focusNeuron))
	c.hub.Publish(hub.Topic("layer3.zeitgeist.spikes"), c.layer3.getZeitgeistSpikes(c.cyclePhase))
	c.hub.Publish(hub.TopicLayer3FeedbackSpikes, layer3VisibleFeedback)
}
