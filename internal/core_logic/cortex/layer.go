package cortex

import (
	"github.com/DjarinDin/TuringSNN/internal/conf"
	"math"
	"math/rand"
	"sort"
)

// layer represents a horizontal slice of the cortex (L1, L2, L3).
// Think of it as a self-contained microcircuit:
//   - It carries its own spike history (Spikes) for axonal delays.
//   - It maintains synaptic weights and traces for learning.
//   - It exposes compact snapshots for visualization and telemetry.
type layer struct {
	// Potentials are membrane integrators: weighted sum of the input traces.
	// They are reset on spikes and decay on leaks.
	Potentials [conf.NumNeurons]float64
	// RefractionCycles enforce a brief "sleep" after a spike.
	RefractionCycles [conf.NumNeurons]int
	// Weights are bounded integers for stability and deterministic serialization.
	Weights [conf.NumNeurons][conf.NumTotSynapses]int // [neuron][weight]
	// WeightResidual keeps fractional learning so small updates are not lost.
	WeightResidual [conf.NumNeurons][conf.NumTotSynapses]float64
	// InputTraces are eligibility traces for each synapse in the Quad Architecture.
	// Each neuron carries its own private trace array (especially for Resonance).
	InputTraces [conf.NumNeurons][conf.NumTotSynapses]float64
	// Mid/Long term traces store *eligibility* over longer windows.
	// They only become weight changes when dopamine gates them.
	MidTermTraces  [conf.NumNeurons][conf.NumTotSynapses]float64
	LongTermTraces [conf.NumNeurons][conf.NumTotSynapses]float64
	// OutputTraces are soma traces (pre-axon); they represent "recently fired" history.
	// Other neurons only see delayed spikes from Spikes (post-axon).
	OutputTraces [conf.NumNeurons]float64
	// Spikes is the circular buffer of axonal output spikes (with delays).
	Spikes [conf.NumCycles][conf.NumTotSynapses]bool
	// Thresholds are dynamic firing thresholds (homeostasis via growth/decay).
	Thresholds [conf.NumNeurons]float64
	// Tags capture synapses that contributed to a spike (for distal reinforcement).
	TagTicks     [conf.NumNeurons][conf.NumTotSynapses]int // [neuron][synapse]
	TagStrengths [conf.NumNeurons][conf.NumTotSynapses]int // [neuron][synapse]
	// ID is the layer number (1, 2, or 3), used for routing/telemetry.
	ID int
}

// getVisibleFeedbackSpikes returns the Gestalt (feedback) spikes that are currently "visible"
// at the given cyclePhase, accounting for diagonal reading based on axonal delays.
// Each neuron's spike is read from (cyclePhase - (neuronDelay + 1)) cycles ago.
// This is used for GUI display and metrics to show what spikes are actually being
// processed (after diagonal reading), not just what was written.
// Gestalt field is at [192-255] in the Quad Architecture.
func (l *layer) getVisibleFeedbackSpikes(cyclePhase int) []bool {
	// We read diagonally so each neuron's unique axonal delay is honored.
	// "Visible" means what downstream neurons can actually see at this phase.
	visible := make([]bool, conf.NumNeurons)
	for nrn := 0; nrn < conf.NumNeurons; nrn++ {
		gestaltSyn := conf.QuadGestaltStart + nrn // [192-255]
		// Read diagonally: neuron nrn's spike is read from (cyclePhase - (nrn + 1)) cycles ago
		readCycle := (cyclePhase - (nrn + 1) + conf.NumCycles) % conf.NumCycles
		visible[nrn] = l.Spikes[readCycle][gestaltSyn]
	}
	return visible
}

// getResonanceSpikes returns the temporal history (T-0 to T-63) of the focused neuron.
// NOTE: We read from T-0 (current cyclePhase) so that spikes are visible IMMEDIATELY.
func (l *layer) getResonanceSpikes(cyclePhase int, focusNeuron int) []bool {
	// Resonance is the private temporal history of the focus neuron.
	// We render T-0..T-63 so the GUI shows immediate and recent activity.
	resonance := make([]bool, conf.NumNeurons)
	gestaltSyn := conf.QuadGestaltStart + focusNeuron // [192-255]
	for t := 0; t < conf.NumNeurons; t++ {
		readCycle := (cyclePhase - t + conf.NumCycles) % conf.NumCycles
		resonance[t] = l.Spikes[readCycle][gestaltSyn]
	}
	return resonance
}

// getZeitgeistSpikes returns the Zeitgeist field spikes from T-1 (previous cycle).
// Zeitgeist tracks the collective state of all neurons at T-1. For visualization,
// we show all neurons' spikes from T-1, which is what each neuron sees in its Zeitgeist field.
func (l *layer) getZeitgeistSpikes(cyclePhase int) []bool {
	// Zeitgeist is "what the layer just did" (T-1), shared across all neurons.
	zeitgeist := make([]bool, conf.NumNeurons)
	prevCycle := (cyclePhase - 1 + conf.NumCycles) % conf.NumCycles
	for nrn := 0; nrn < conf.NumNeurons; nrn++ {
		gestaltSyn := conf.QuadGestaltStart + nrn // [192-255]
		// Read from T-1 (previous cycle) - this is what Zeitgeist field sees
		zeitgeist[nrn] = l.Spikes[prevCycle][gestaltSyn]
	}
	return zeitgeist
}

// ********************************************************************************* //
// *** I ********.......**......******.******.....***.......***.....**************** //
// *** N ***********.*****.*****.****.*.****.*****.**.********.********************* //
// *** P ***********.*****......****.***.***.********....******.....**************** //
// *** U ***********.*****.****.***.......**.*****.**.**************.*************** //
// *** T ***********.*****.*****.**.*****.***.....***.......***.....**************** //
// ********************************************************************************* //

// calculateInputTraces calculates the traces for the passed-in layer.
// Quad Architecture: Four fields of 128 synapses each (512 total)
// 1. Phenomenon [0-127]: Raw sensor inputs (Shared unified array)
// 2. Resonance [128-255]: Individual neuron history (Private)
// 3. Zeitgeist [256-383]: Collective state of layer at T-1 (Shared)
// 4. Gestalt [384-511]: Unified prediction (Feedback)
func (l *layer) calculateInputTraces(cyclePhase int) {
	// The trace update is the "eligibility backbone":
	// spikes refresh traces; absence causes exponential decay.
	// ============================================================================================
	// SHARED FIELDS: Phenomenon, Zeitgeist, Gestalt
	// These signals are the same for every neuron in the layer.
	// ============================================================================================

	// Prepare shared spike state for Fields 3 and 4
	prevCycle := (cyclePhase - 1 + conf.NumCycles) % conf.NumCycles

	for nrn := 0; nrn < conf.NumNeurons; nrn++ {
		// Field 1: Phenomenon [0-127] (sensors) — shared across all neurons.
		for sensorSyn := conf.QuadPhenomenonStart; sensorSyn < conf.QuadPhenomenonEnd; sensorSyn++ {
			if l.Spikes[cyclePhase][sensorSyn] {
				l.InputTraces[nrn][sensorSyn] = float64(conf.MaxTraceAndWeight)
			} else {
				l.InputTraces[nrn][sensorSyn] = l.InputTraces[nrn][sensorSyn] * conf.ShortTraceDecayFactor
			}
		}

		// Field 3: Zeitgeist [256-383] — shared T-1 layer state.
		for zNrn := 0; zNrn < conf.NumNeurons; zNrn++ {
			zeitgeistSyn := conf.QuadZeitgeistStart + zNrn // [256-383]
			gSyn := conf.QuadGestaltStart + zNrn           // [384-511]
			if l.Spikes[prevCycle][gSyn] {
				l.InputTraces[nrn][zeitgeistSyn] = float64(conf.MaxTraceAndWeight)
			} else {
				l.InputTraces[nrn][zeitgeistSyn] = l.InputTraces[nrn][zeitgeistSyn] * conf.ShortTraceDecayFactor
			}
		}

		// Field 4: Gestalt [384-511] — delayed feedback from higher layers.
		for gNrn := 0; gNrn < conf.NumNeurons; gNrn++ {
			gestaltSyn := conf.QuadGestaltStart + gNrn // [384-511]
			readCycle := (cyclePhase - (gNrn + 1) + conf.NumCycles) % conf.NumCycles
			if l.Spikes[readCycle][gestaltSyn] {
				l.InputTraces[nrn][gestaltSyn] = float64(conf.MaxTraceAndWeight)
			} else {
				l.InputTraces[nrn][gestaltSyn] = l.InputTraces[nrn][gestaltSyn] * conf.ShortTraceDecayFactor
			}
		}

		// ============================================================================================
		// PRIVATE FIELD: Resonance [128-255] — each neuron’s own history line.
		// Each neuron tracks its OWN history over time.
		// Synapse (128+t) = this neuron's spike at T-(t+1)
		// ============================================================================================
		myGestaltSyn := conf.QuadGestaltStart + nrn // My spike location in Gestalt field
		for t := 0; t < conf.NumNeurons; t++ {
			resonanceSyn := conf.QuadResonanceStart + t // [128-255]
			readCycle := (cyclePhase - (t + 1) + conf.NumCycles) % conf.NumCycles
			if l.Spikes[readCycle][myGestaltSyn] {
				l.InputTraces[nrn][resonanceSyn] = float64(conf.MaxTraceAndWeight)
			} else {
				l.InputTraces[nrn][resonanceSyn] = l.InputTraces[nrn][resonanceSyn] * conf.ShortTraceDecayFactor
			}
		}

	}
}

// *************************************************************************************************** //
// *************************************************************************************************** //
// *********......****.....***.......**.......**..****.**.......**.*****.*****.********.....********** //
// *********.*****.**.*****.*****.*****.********.*.***.*****.*****.****.*.****.*******.*************** //
// *********......***.*****.*****.*****....*****.**.**.*****.*****.***.***.***.********.....********** //
// *********.********.*****.*****.*****.********.***.*.*****.*****.**.......**.*************.********* //
// *********.*********.....******.*****.......**.****..*****.*****.**.*****.**.......**.....********** //
// *************************************************************************************************** //
// *************************************************************************************************** //

func (l *layer) calculatePotentials(density float64) {
	// Potentials integrate current traces into a membrane state.
	// Density controls inhibition so chaos damps itself.
	// Density Damping: Inhibition scales non-linearly with total layer activity
	// This ensures that as the 'Storm' builds, the 'Quencher' grows even faster.
	inhibitionDamping := 1.0 + (density * density * 2.0) // Quadatric damping boost

	for nrn := 0; nrn < conf.NumNeurons; nrn++ { // for each neuron
		if l.RefractionCycles[nrn] == 0 {
			// ============================================================================================
			// LANDMARKS: THE BRAIN'S COMPETING VOICES
			// ============================================================================================

			// FIELD 1: PHENOMENON [0-63] - Raw sensor inputs (full weight)
			for syn := conf.QuadPhenomenonStart; syn < conf.QuadPhenomenonEnd; syn++ {
				l.Potentials[nrn] += float64(l.Weights[nrn][syn]) * l.InputTraces[nrn][syn]
			}
			// FIELD 2: RESONANCE [64-127] - Individual neuron over time (full weight)
			for syn := conf.QuadResonanceStart; syn < conf.QuadResonanceEnd; syn++ {
				l.Potentials[nrn] += float64(l.Weights[nrn][syn]) * l.InputTraces[nrn][syn]
			}

			// INHIBITION ◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌
			// FIELD 3: ZEITGEIST [128-191] - Adaptive lateral inhibition
			// This field is strictly INHIBITORY. It tracks the collective state
			// and subtracts it from the potential. Scaled by Inhibition Damping.
			for syn := conf.QuadZeitgeistStart; syn < conf.QuadZeitgeistEnd; syn++ {
				l.Potentials[nrn] -= float64(l.Weights[nrn][syn]) * l.InputTraces[nrn][syn] * inhibitionDamping
			}
			// ◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌◌

			// FIELD 4: GESTALT [192-255] - Feedback from higher layers
			for syn := conf.QuadGestaltStart; syn < conf.QuadGestaltEnd; syn++ {
				l.Potentials[nrn] += float64(l.Weights[nrn][syn]) * l.InputTraces[nrn][syn] / conf.SensorToFeedbackRatio
			}
		} else {
			l.RefractionCycles[nrn]--
		}
	}
}

// ******************************************************************************************** //
// ************************  *****************************************  *********************** //
// ******************************************************************************************** //
//	To spike,                ***.....***......***.**.*****.**.......**                  Whether //
//	or not to spike,         **.********.*****.**.**.***.****.********              'tis nobler //
//	...                      ***.....***......***.**...******....*****                      ... //
//	that is                  ********.**.********.**.***.****.********              in the mind //
//	the question.            ***.....***.********.**.*****.**.......**                 to spike //
// ******************************************************************************************** //
// ************************  *****************************************  *********************** //
// ******************************************************************************************** //

// calculateSpikes calculates the spikes for each neuron in the Cortex.
// It iterates over each neuron and checks if its potential is below the threshold.
// If the potential is below the threshold, it checks if a spontaneous random fire should occur.
// If so, it sets the spike in the spike matrix, updates the current epoch spikes,
// resets the potential, sets the refraction cycles, and updates the soma traces.
// If the potential is above the threshold, it sets the spike in the spike matrix,
// updates the current epoch spikes, resets the potential, sets the refraction cycles,
// and updates the soma traces.
func (l *layer) calculateSpikes(cyclePhase int, spontaneousFire float64, cortexTick int) {
	// Spiking is a three-phase pipeline:
	// 1) find candidates (threshold + stochastic exploration)
	// 2) apply inhibition policy (soft vs hard WTA)
	// 3) commit winners and leak the rest
	// Candidate captures neurons that WOULD fire before inhibition policy is applied.
	type candidate struct {
		nrn       int
		potential float64
	}

	candidates := make([]candidate, 0, conf.NumNeurons)
	willFire := make([]bool, conf.NumNeurons)

	// Phase 1: Determine which neurons are candidates based on thresholds/spontaneous fire.
	for nrn := 0; nrn < conf.NumNeurons; nrn++ {
		if l.Potentials[nrn] <= l.Thresholds[nrn] {
			// Below threshold: allow rare stochastic firing to keep exploration alive.
			if rand.Intn(10000) < int(spontaneousFire) {
				candidates = append(candidates, candidate{nrn: nrn, potential: l.Potentials[nrn]})
			}
			continue
		}

		// Above threshold: deterministic candidate for firing.
		candidates = append(candidates, candidate{nrn: nrn, potential: l.Potentials[nrn]})
	}

	// Phase 2: Apply the Zeitgeist inhibition policy (soft vs hard WTA).
	switch conf.ZeitgeistInhibitionMode {
	case conf.ZeitgeistInhibitionHardWTA:
		// Hard WTA keeps only the top-k candidates by potential (ties resolved by neuron index).
		sort.Slice(candidates, func(i, j int) bool {
			if candidates[i].potential == candidates[j].potential {
				return candidates[i].nrn < candidates[j].nrn
			}
			return candidates[i].potential > candidates[j].potential
		})

		// Mark the winners; everyone else will leak this cycle.
		k := conf.ZeitgeistWTAK
		if k > len(candidates) {
			k = len(candidates)
		}
		for i := 0; i < k; i++ {
			willFire[candidates[i].nrn] = true
		}
	default:
		// Soft inhibition: all candidates are allowed to fire.
		for _, cand := range candidates {
			willFire[cand.nrn] = true
		}
	}

	// Phase 3: Commit the decision: fire winners, leak everyone else.
	for nrn := 0; nrn < conf.NumNeurons; nrn++ {
		if willFire[nrn] {
			// Winner spikes: set refractory state and desensitize threshold.
			l.fire(nrn, cyclePhase, cortexTick)
			l.Thresholds[nrn] *= conf.ThresholdGrowth
			continue
		}

		// Losers and non-candidates leak and sensitize to stay competitive.
		l.leak(nrn, cyclePhase)
		if l.Thresholds[nrn] > conf.MinThreshold {
			l.Thresholds[nrn] *= conf.ThresholdDecay
		}
	}
}

// FIRE  ↑↑↑ ▲▲▲▲▲▲▲▲ ▲▲▲▲▲▲▲▲ ▲▲▲▲▲▲▲▲ ▲▲▲▲▲▲▲▲ ▲▲▲▲▲▲▲▲ ▲▲▲▲▲▲▲▲ ▲▲▲▲▲▲▲▲ ▲▲▲▲▲▲▲▲
// fire triggers the firing of a neuron in the Cortex.
func (l *layer) fire(nrn int, cyclePhase int, cortexTick int) {
	// A spike is both a signal (write to spike matrix) and a learning event
	// (tagging + output trace refresh).
	// Neuron fires and writes to the current cycle's vertical column in the Gestalt field.
	// The axonal delay is achieved by reading diagonally (from past cycles)
	// when other neurons read this spike. Each neuron reads from a different
	// past cycle based on the source neuron's delay.
	// Quad Architecture: Gestalt field is at [192-255]
	l.Spikes[cyclePhase][conf.QuadGestaltStart+nrn] = true

	// SYNAPTIC TAGGING (Reinforcement Phase 4 - Proportional Upgrade)
	// Tag every synapse that has an active trace (contributed to this spike).
	// We store the TickID for age checks and the current Trace value for proportional reward.
	for syn := 0; syn < conf.NumTotSynapses; syn++ {
		traceVal := l.InputTraces[nrn][syn]
		if traceVal > (float64(conf.MaxTraceAndWeight) / 10.0) { // Lower floor to capture more nuanced hypotheses
			// Tag the synapse so delayed dopamine can still credit/blame it.
			l.TagTicks[nrn][syn] = cortexTick
			l.TagStrengths[nrn][syn] = int(traceVal)
		}
	}

	// However, the potential is calculated in the here and now. Since this neuron
	// just spiked, potential is reset to zero.
	l.Potentials[nrn] = conf.PotentialRefractoryLevel
	// set number of refraction cycles to max, putting the neuron to sleep for those cycles
	l.RefractionCycles[nrn] = conf.NumRefractionCycles
	// Also, the trace matters in the current epoch to the weight adjustment process.
	// The traces are used as a measure of the time distance (negative or positive) of
	// the pre and post synapse spikes.  If the pre spike is larger (the pre-neuron
	// spiked first),the weight will be increased below, in this epoch, and vice-versa.
	l.OutputTraces[nrn] = float64(conf.MaxTraceAndWeight)
}

// LEAK ▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼
// leak updates the state of the Cortex by performing various operations on the given neuron.
func (l *layer) leak(nrn int, cyclePhase int) {
	// Leak is the "no spike" path: decay potential and traces.
	// Neuron leaks and writes to the current cycle's vertical column in the Gestalt field.
	// The axonal delay is achieved by reading diagonally when other neurons read this spike.
	// Quad Architecture: Gestalt field is at [192-255]
	l.Spikes[cyclePhase][conf.QuadGestaltStart+nrn] = false
	l.Potentials[nrn] *= conf.PotentialPassiveDecayFactor
	l.OutputTraces[nrn] = l.OutputTraces[nrn] * conf.ShortTraceDecayFactor
}

// ******************************************************************************************** //
// ******************************************************************************************** //
// ***********************.********.......*****.*****......***..****.************************** //
// ***********************.********.**********.*.****.*****.**.*.***.************************** //
// ***********************.********....******.***.***......***.**.**.************************** //
// ***********************.********.********.......**.****.***.***.*.************************** //
// ***********************.......**.......**.*****.**.*****.**.****..************************** //
// ******************************************************************************************** //
// ******************************************************************************************** //

// ******************************************************************************************** //
// ******************************************************************************************** //
// ********  All weight adjustments are based on spikes from the previous epoch.       ******** //
// ********  This is the short term memory - it is based on the lifetime of the traces ******** //
// ********  Long-term memory integrates dopamine-modulated traces over time.          ******** //
// ******************************************************************************************** //
// ******************************************************************************************** //

// calculateWeights calculates the new weights for each neuron in the Cortex.
func (l *layer) calculateWeights(learningRate float64, dopamine float64) {
	// Learning is local per-synapse but globally gated by dopamine.
	// The three-factor rule here is:
	//   (1) coincidence traces -> eligibility
	//   (2) eligibility traces -> mid/long memory
	//   (3) dopamine gate -> actual weight updates
	for nrn := 0; nrn < conf.NumNeurons; nrn++ { // for all neurons
		// Sensor weights
		for syn := 0; syn < conf.NumTotSynapses; syn++ { // for all synapses
			// ============================================================================================
			// LITERATE OVERVIEW: ELIGIBILITY TRACES + DOPAMINE GATING (THREE-FACTOR RULE)
			// ============================================================================================
			// We compute a *signed eligibility* value from the classic STDP traces:
			//   - InputTraces[nrn][syn]  = how recently THIS synapse saw a pre-synaptic spike
			//   - OutputTraces[nrn]      = how recently THIS neuron fired a post-synaptic spike
			//
			// Their product (normalized by MaxTrSquared) gives a unitless "coincidence amplitude"
			// for this synapse during THIS tick. We then *bake timing polarity into sign*:
			//   - If pre <= post, eligibility is positive (LTP candidate).
			//   - If pre  > post, eligibility is negative (LTD candidate).
			//
			// Crucially, eligibility accumulates in mid/long traces *regardless of dopamine*.
			// Dopamine is applied later as a gate to the *combined eligibility* so that
			// delayed dopamine can still stamp whatever eligibility remains.
			// ============================================================================================

			amplitude := l.InputTraces[nrn][syn] * l.OutputTraces[nrn] / (conf.MaxTrSquared)
			amplitude *= learningRate // Apply modulation

			// ELIGIBILITY GATING:
			// Only compute eligibility when there is meaningful local coincidence.
			eligible := amplitude >= conf.MinEligibilityAmp &&
				l.InputTraces[nrn][syn] >= conf.TraceFloor &&
				l.OutputTraces[nrn] >= conf.TraceFloor

			// SIGNED ELIGIBILITY:
			// Estimate Δt from exponentially decayed traces:
			//   trace = Max * decay^k  =>  k ≈ log(trace/Max) / log(decay)
			// We compute k_pre (input) and k_post (output), then Δt = k_pre - k_post.
			// Positive Δt => pre before post (LTP). Negative Δt => post before pre (LTD).
			// Magnitude shrinks with |Δt| via an exponential kernel exp(-|Δt|/tau).
			eligibility := 0.0
			if eligible {
				maxTrace := float64(conf.MaxTraceAndWeight)
				logDecay := math.Log(conf.ShortTraceDecayFactor)
				kPre := math.Log(l.InputTraces[nrn][syn]/maxTrace) / logDecay
				kPost := math.Log(l.OutputTraces[nrn]/maxTrace) / logDecay
				deltaTicks := kPre - kPost
				kernel := math.Exp(-math.Abs(deltaTicks) / conf.StdpTauTicks)

				eligibility = amplitude * kernel
				if deltaTicks < 0 {
					eligibility = -eligibility
				}
			}

			// MID-TERM TRACE (seconds scale):
			// 1) Exponentially decay prior eligibility by multiplying by MidTraceDecayFactor.
			//    This decay factor is derived from a half-life in seconds and the tick rate:
			//       decay = 0.5^(tickSeconds / halfLifeSeconds)
			// 2) Add the new signed eligibility, scaled by MidTermTraceGain.
			// Result: a slow accumulator of *candidates* (not yet rewarded),
			// preserving causal hypotheses across seconds.
			// Update mid-term traces (dopamine-modulated, seconds-scale memory).
			midTrace := l.MidTermTraces[nrn][syn] * conf.MidTraceDecayFactor
			midTrace += conf.MidTermTraceGain * eligibility
			l.MidTermTraces[nrn][syn] = midTrace

			// LONG-TERM TRACE (minutes scale):
			// Same logic as mid-term, but with a much longer half-life and its own gain.
			// To reduce false credit, we only *write* to long-term memory when mid-trace
			// magnitude exceeds a gate threshold, signaling repeated/robust eligibility.
			// Update long-term traces (dopamine-modulated, minutes-scale memory).
			longTrace := l.LongTermTraces[nrn][syn] * conf.LongTraceDecayFactor
			if math.Abs(midTrace) > conf.LongEligibilityGate {
				longTrace += conf.LongTermTraceGain * eligibility
			}
			l.LongTermTraces[nrn][syn] = longTrace

			// COMBINED ELIGIBILITY:
			// We blend all three time scales into one signed eligibility value.
			//   combinedElig = eligibility
			//                + (midTermLearningRate  * midTrace)
			//                + (longTermLearningRate * longTrace)
			//
			// This ensures:
			//   - Eligibility is preserved for delayed dopamine.
			//   - The learning rates are explicit knobs for balancing temporal horizons.
			combinedEligibility := eligibility + conf.MidTermLearningRate*midTrace + conf.LongTermLearningRate*longTrace

			// DOPAMINE GATE:
			// Dopamine arrives as a scalar gate that stamps *whatever eligibility remains*.
			combined := dopamine * combinedEligibility

			// WEIGHT UPDATE (SIGN-CONSISTENT SOFT BOUNDS):
			// If combined is positive, move weight toward max.
			// If combined is negative, move weight toward zero.
			var delta float64
			if combined >= 0 {
				delta = combined * (float64(conf.MaxTraceAndWeight) - float64(l.Weights[nrn][syn])) / 2.0
			} else {
				delta = combined * (float64(l.Weights[nrn][syn]) / 2.0)
			}

			// Accumulate sub-integer updates to preserve smooth learning.
			l.WeightResidual[nrn][syn] += delta
			step := int(math.Floor(l.WeightResidual[nrn][syn]))
			l.WeightResidual[nrn][syn] -= float64(step)
			l.Weights[nrn][syn] += step

			// Clamp weights to valid range after update.
			if l.Weights[nrn][syn] <= 0 {
				l.Weights[nrn][syn] = 0
				if l.WeightResidual[nrn][syn] < 0 {
					l.WeightResidual[nrn][syn] = 0
				}
			} else if l.Weights[nrn][syn] >= conf.MaxTraceAndWeight {
				l.Weights[nrn][syn] = conf.MaxTraceAndWeight
				if l.WeightResidual[nrn][syn] > 0 {
					l.WeightResidual[nrn][syn] = 0
				}
			}
		}
	}
}
