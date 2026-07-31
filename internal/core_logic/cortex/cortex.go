package cortex

import (
	"log"
	"math/rand"
	"runtime/debug"
	"time"

	"github.com/DjarinDin/TuringSNN/internal/conf"
	"github.com/DjarinDin/TuringSNN/internal/core_logic/hub"
	"github.com/DjarinDin/TuringSNN/pkg/comms"
)

// Cortex is the central orchestrator of the neuromorphic simulation. It maintains the state
// of three hierarchical layers (L1, L2, L3), an output layer, and all the communication channels
// that connect the simulation to the GUI and external sensors. The Cortex processes sensory
// inputs through its layers, performs spike-timing-dependent plasticity (STDP) learning, and
// manages the temporal dynamics of the neural network.
type Cortex struct {
	// ******************************************************************************************** //
	// **************************  TIMING & CONTROL STATE  ************************************** //
	// ******************************************************************************************** //

	// cortexHeartbeatNum tracks the number of heartbeat cycles that have elapsed. The heartbeat
	// is a slower metronome that governs telemetry and GUI updates, separate from the core
	// simulation tick rate.
	cortexHeartbeatNum int
	// cortexTick is the current simulation cycle number. This increments with each pass through
	// the main processing loop, tracking the absolute time in the simulation.
	cortexTick int
	// lastCortexTick stores the tick count from the previous heartbeat, used to calculate
	// ticks-per-heartbeat metrics for performance monitoring.
	lastCortexTick int
	// cyclePhase is the current position in the circular buffer of cycles. It wraps around
	// using modulo arithmetic to create a sliding window of temporal history for axonal delays.
	cyclePhase int
	// focusNeuron is the neuron ID currently selected for detailed visualization in the GUI.
	// This allows users to zoom in on a single neuron's weights, traces, and potential.
	focusNeuron int
	// focusLayer indicates which layer (1, 2, or 3) contains the focusNeuron being visualized.
	focusLayer int
	// traceDisplayMode selects which trace type to visualize (0=Short, 1=Mid, 2=Long).
	// This allows examining eligibility at different timescales.
	traceDisplayMode int

	// ******************************************************************************************** //
	// **************************  SIGNAL ACCUMULATION METRICS  ********************************** //
	// ******************************************************************************************** //

	// signalsReceivedThisHeartbeat counts how many sensor signals arrived during the current
	// heartbeat period. This is reset each heartbeat for per-period statistics.
	signalsReceivedThisHeartbeat int
	// totalSignalsReceived is the cumulative count of all sensor signals since initialization
	// or the last reset. Used for lifetime statistics.
	totalSignalsReceived int
	// signalsAccumulatedThisHeartbeat tracks how many signals were successfully written to
	// the accumulator (not dropped due to collisions). Reset each heartbeat.
	signalsAccumulatedThisHeartbeat int
	// totalSignalsAccumulated is the cumulative count of successfully accumulated signals.
	// The difference between received and accumulated indicates signal loss/collisions.
	totalSignalsAccumulated int
	// lastTickerDuration stores the current duration between core simulation ticks. This can
	// be dynamically adjusted via control messages to speed up or slow down the simulation.
	lastTickerDuration time.Duration

	// ******************************************************************************************** //
	// **************************  LEARNING & MODULATION PARAMETERS  **************************** //
	// ******************************************************************************************** //

	// amplitude is a temporary variable used during weight calculations to hold the learning
	// amplitude based on trace overlap between pre- and post-synaptic spikes.
	amplitude float64
	// halfAmplitude is used in output layer learning calculations, storing half the amplitude
	// for symmetry in the STDP learning rule.
	halfAmplitude float64
	// dopamine controls the strength of synaptic plasticity. Higher values increase the rate
	// of weight changes during learning. This is the neuromodulator that gates plasticity.
	dopamine float64
	// spontaneousFire is the probability (per 10,000) that a neuron will fire spontaneously
	// even when its potential is below threshold. This introduces stochasticity and prevents
	// the network from getting stuck in silent states.
	spontaneousFire float64

	// ******************************************************************************************** //
	// **************************  CHAOS METRICS (DENSITY)  ************************************** //
	// ******************************************************************************************** //

	// MetricSensorDensity measures the fraction of sensor inputs that are active (spiking)
	// in the current cycle. High density indicates chaotic or saturated input.
	MetricSensorDensity float32
	// MetricLayer1Density measures the fraction of L1 neurons that spiked, representing
	// the activity level of the first cortical layer.
	MetricLayer1Density float32
	// MetricLayer2Density measures the fraction of L2 neurons that spiked, representing
	// the activity level of the second cortical layer.
	MetricLayer2Density float32
	// MetricLayer3Density measures the fraction of L3 neurons that spiked, representing
	// the activity level of the third cortical layer.
	MetricLayer3Density float32

	// Adaptability controls the learning rate modulation based on density. When set to 1.0,
	// learning is fully modulated by chaos metrics (high density = slower learning). When
	// set to 0.0, learning is frozen regardless of density. This allows fine-grained control
	// over the network's responsiveness to environmental chaos.
	Adaptability float64 // 0.0 to 1.0, controls learning rate modulation based on density

	// ******************************************************************************************** //
	// **************************  TIMERS & EVENT SOURCES  ************************************** //
	// ******************************************************************************************** //

	// cortexHeartbeatTicker fires at a fixed rate (HeartbeatRate) to trigger periodic
	// telemetry updates and GUI refresh. This is slower than the core ticker.
	cortexHeartbeatTicker *time.Ticker
	// cortexCoreTicker drives the main simulation loop. Its rate can be dynamically adjusted
	// via control messages, allowing real-time speed control of the simulation.
	cortexCoreTicker *time.Ticker

	// hub is the central message bus that allows the Cortex to publish events (like tick
	// notifications and density metrics) that other components (like the simulator) can subscribe to.
	hub *hub.Hub

	// controlMessage is a temporary holder for control messages being processed.
	controlMessage comms.ControlMsg

	// ******************************************************************************************** //
	// **************************  GUI COMMUNICATION CHANNELS  ********************************** //
	// ******************************************************************************************** //

	// cumulativeStatsCh sends lifetime statistics (total signals, collisions, etc.) to the GUI
	// for display in the monitoring panel.
	cumulativeStatsCh chan comms.StringMsg
	// perHeartbeatStatsCh sends per-heartbeat statistics (ticks per heartbeat, collisions
	// this period) to the GUI for real-time monitoring.
	perHeartbeatStatsCh chan comms.StringMsg

	// Layer 1 GUI channels: These send the current state of Layer 1 to the GUI for visualization.
	// The GUI displays weights, traces, and spikes for the currently focused neuron.
	// Quad Architecture: Four fields per layer
	layer1InputWeightsCh     chan []int     // Phenomenon field weights [0-63] for the focus neuron
	layer1InputTracesCh      chan []float64 // Phenomenon field traces [0-63] (sensor inputs)
	layer1ResonanceWeightsCh chan []int     // Resonance field weights [64-127] for the focus neuron
	layer1ResonanceTracesCh  chan []float64 // Resonance field traces [64-127]
	layer1ZeitgeistWeightsCh chan []int     // Zeitgeist field weights [128-191] for the focus neuron
	layer1ZeitgeistTracesCh  chan []float64 // Zeitgeist field traces [128-191]
	layer1FeedbackWeightsCh  chan []int     // Gestalt field weights [192-255] for the focus neuron
	layer1FeedbackTracesCh   chan []float64 // Gestalt field traces [192-255] (from higher layers)
	layer1OutputTracesCh     chan []float64 // Output traces (soma traces) for all L1 neurons

	// Layer 2 GUI channels: Same structure as Layer 1, but for the second cortical layer.
	layer2InputWeightsCh     chan []int     // Phenomenon [0-63]
	layer2InputTracesCh      chan []float64 // Phenomenon [0-63] Traces
	layer2ResonanceWeightsCh chan []int     // Resonance [64-127]
	layer2ResonanceTracesCh  chan []float64 // Resonance [64-127]
	layer2ZeitgeistWeightsCh chan []int     // Zeitgeist [128-191]
	layer2ZeitgeistTracesCh  chan []float64 // Zeitgeist [128-191]
	layer2FeedbackWeightsCh  chan []int     // Gestalt [192-255]
	layer2FeedbackTracesCh   chan []float64 // Gestalt [192-255]
	layer2OutputTracesCh     chan []float64

	// Layer 3 GUI channels: Same structure as Layer 1, but for the third cortical layer.
	layer3InputWeightsCh     chan []int     // Phenomenon [0-63]
	layer3InputTracesCh      chan []float64 // Phenomenon [0-63] Traces
	layer3ResonanceWeightsCh chan []int     // Resonance [64-127]
	layer3ResonanceTracesCh  chan []float64 // Resonance [64-127]
	layer3ZeitgeistWeightsCh chan []int     // Zeitgeist [128-191]
	layer3ZeitgeistTracesCh  chan []float64 // Zeitgeist [128-191]
	layer3FeedbackWeightsCh  chan []int     // Gestalt [192-255]
	layer3FeedbackTracesCh   chan []float64 // Gestalt [192-255]
	layer3OutputTracesCh     chan []float64

	// Output layer GUI channels: Send weights for the two output neurons to the GUI.
	outputAWeightsCh chan []int // Weights for output neuron A
	outputBWeightsCh chan []int // Weights for output neuron B
	outputCWeightsCh chan []int // Weights for output neuron C
	outputDWeightsCh chan []int // Weights for output neuron D

	// ******************************************************************************************** //
	// **************************  SENSOR ACCUMULATOR  ******************************************** //
	// ******************************************************************************************** //

	// The accumulator is a boolean array that tracks whether a sensor has received a spike
	// during the current cycle. Sensor signals arrive asynchronously from external sources
	// (simulator or real sensors), and the accumulator collects them before they are moved
	// into the spike matrix at the start of each cycle. This decouples signal arrival from
	// the synchronous processing loop.
	accumulator [conf.NumSensors]bool
	// sensorTraces provides a unified global temporal history for all receptors.
	sensorTraces [conf.NumSensors]int

	// ******************************************************************************************** //
	// **************************  CORTICAL LAYERS  ********************************************** //
	// ******************************************************************************************** //

	// layer1 is the first cortical layer, receiving direct input from sensors. It performs
	// initial feature extraction and pattern recognition on the raw sensory stream.
	layer1 layer
	// layer2 is the second cortical layer, receiving input from layer1. It builds more
	// abstract representations by combining features detected in layer1.
	layer2 layer
	// layer3 is the third cortical layer, receiving input from layer2. It forms the highest
	// level abstractions and predictions in the hierarchy.
	layer3 layer

	// ******************************************************************************************** //
	// **************************  OUTPUT LAYER  ************************************************** //
	// ******************************************************************************************** //

	// outputPotentials are the membrane potentials for the output neurons. These neurons
	// integrate inputs from all three cortical layers plus direct sensor inputs to make
	// decisions or predictions.
	outputPotentials [conf.NumOutputNeurons]float64
	// outputThresholds control the firing sensitivity of output neurons. These are dynamically
	// adjusted via HPIE (Homeostatic Plasticity of Intrinsic Excitability) based on firing history.
	outputThresholds [conf.NumOutputNeurons]float64
	// outputWeights are the synaptic weights connecting inputs (sensors + all three layers)
	// to the output neurons. These are learned via STDP.
	outputWeights [conf.NumOutputNeurons][conf.OutputNeuronInputWidth]int
	// outputSomaTraces track the time since each output neuron last fired. Used for STDP
	// learning to determine temporal relationships between pre- and post-synaptic spikes.
	outputSomaTraces [conf.NumOutputNeurons]int
	// outputRefractionCycles tracks the refractory period for output neurons.
	outputRefractionCycles [conf.NumOutputNeurons]int
	// outputTagTicks and outputTagStrengths track when a synapse contributed to an output spike.
	// Used for distal reinforcement (Dopamine) of the decision maker.
	outputTagTicks     [conf.NumOutputNeurons][conf.OutputNeuronInputWidth]int
	outputTagStrengths [conf.NumOutputNeurons][conf.OutputNeuronInputWidth]int
	// outputSpikes is a 2D array that tracks the spikes of output neurons over time.
	// Similar to layer.Spikes, this allows tracking spike history for axonal delays
	// and visualization. [cycle][neuron]
	outputSpikes [conf.NumCycles][conf.NumOutputNeurons]bool

	// Spike visualization channels: Send spike patterns to the GUI for rendering.
	// These allow the GUI to display which neurons are active in each layer.
	sensorSpikesCh          chan []bool // Sensor input spikes (from outside the cortex)
	layer1FeedbackSpikesCh  chan []bool // L1 feedback spikes (to higher layers)
	layer1ResonanceSpikesCh chan []bool // L1 Resonance field spikes (T-1 from Gestalt)
	layer1ZeitgeistSpikesCh chan []bool // L1 Zeitgeist field spikes (T-1 from Gestalt)

	layer2InputSpikesCh     chan []bool // L2 input spikes (from L1)
	layer2FeedbackSpikesCh  chan []bool // L2 feedback spikes (to higher layers)
	layer2ResonanceSpikesCh chan []bool // L2 Resonance field spikes (T-1 from Gestalt)
	layer2ZeitgeistSpikesCh chan []bool // L2 Zeitgeist field spikes (T-1 from Gestalt)

	layer3InputSpikesCh     chan []bool // L3 input spikes (from L2)
	layer3FeedbackSpikesCh  chan []bool // L3 feedback spikes (to higher layers)
	layer3ResonanceSpikesCh chan []bool // L3 Resonance field spikes (T-1 from Gestalt)
	layer3ZeitgeistSpikesCh chan []bool // L3 Zeitgeist field spikes (T-1 from Gestalt)

	// mainPanelControlCh sends control messages from output neurons back to the main
	// control panel (e.g., when an output neuron fires, it can trigger UI actions).
	mainPanelControlCh chan comms.ControlMsg

	// focusPotHistoryCh sends the potential and spike history for the currently focused
	// neuron to the GUI for time-series visualization.
	focusPotHistoryCh chan comms.BoolMsg

	// Output neuron history channels: Send potential and spike data for output neurons
	// to the GUI for visualization.
	outputAHistoryCh chan comms.BoolMsg // Output neuron A history
	outputBHistoryCh chan comms.BoolMsg // Output neuron B history
	outputCHistoryCh chan comms.BoolMsg // MOTOR BUY history
	outputDHistoryCh chan comms.BoolMsg // MOTOR SELL history

	// Layer visualization channels: Send complete potential and spike arrays for each
	// layer to the GUI for heatmap and spike raster visualization.
	layer1PotsAndSpikesCh chan comms.PotentialsAndSpikesMsg
	layer2PotsAndSpikesCh chan comms.PotentialsAndSpikesMsg
	layer3PotsAndSpikesCh chan comms.PotentialsAndSpikesMsg

	// Raw sensor channels: Used for monitoring and debugging. These show the raw
	// sensor signals before they enter the accumulator.
	rawSensorSpikesCh chan int  // Individual sensor spike events (for monitoring)
	rawSensorBlankCh  chan bool // Blank signal (currently unused)

	// ******************************************************************************************** //
	// **************************  CONTROL & INPUT CHANNELS  ************************************ //
	// ******************************************************************************************** //

	// cortexSensorCh receives sensor spike events from external sources (simulator or
	// real sensors). These arrive asynchronously and are accumulated before processing.
	cortexSensorCh chan int

	// cortexControlCh receives control messages from the GUI or external controllers.
	// Message types include: tick rate changes, dopamine adjustments, pause/resume, etc.
	cortexControlCh chan comms.ControlMsg
	// cortexResetCh triggers a full reset of the Cortex, reinitializing all weights,
	// traces, and state to random initial values.
	cortexResetCh chan bool
	// cortexSoftResetCh triggers a soft reset that preserves control parameters (dopamine,
	// tick rate, focus) but resets all neural state (weights, traces, potentials).
	cortexSoftResetCh chan bool
	// focusNeuronCh receives messages to change which neuron is being focused for
	// detailed visualization (currently unused, focus changed via control messages).
	focusNeuronCh chan comms.ControlMsg
	// cortexHeartbeatGUICh sends the current heartbeat number to the GUI for display.
	cortexHeartbeatGUICh chan int
	// paused indicates whether the simulation is currently paused. When paused, the
	// core ticker is stopped but control messages can still be processed.
	paused bool

	// retributionCh receives profit/loss signals from the virtual broker (Phase 5).
	// Signals are queued and applied synchronously at the start of each cycle.
	retributionCh chan float64
}

// initialize sets up the Cortex to a fresh, randomized state. This function is called
// both at startup and when a full reset is requested. It randomizes all weights, traces,
// and potentials to create a "blank slate" network ready for learning.
//
// This was extracted from a hypothetical init() function because it needs to be callable
// separately when a reset is triggered via the resetCh channel. The separation allows
// the Cortex to be reinitialized at runtime without recreating the entire struct.
func (c *Cortex) initialize() {
	// Reset timing and control state
	c.cortexTick = 1           // Start at tick 1 (tick 0 is the initial state)
	c.dopamine = conf.Dopamine // Set to default dopamine level
	c.Adaptability = 1.0       // Default to 1.0 (100% - full adaptability)
	c.lastTickerDuration = time.Millisecond * time.Duration(conf.RealTimeTickPeriod)
	c.paused = false                          // Ensure we start in running state
	c.retributionCh = make(chan float64, 100) // Buffer for pending reinforcement pulses
	c.layer1.ID = 1
	c.layer2.ID = 2
	c.layer3.ID = 3

	// Set default focus for GUI visualization
	c.focusLayer = 1  // Focus on layer 1 by default
	c.focusNeuron = 0 // Focus on neuron 0 by default

	// Clear the sensor accumulator and traces to start fresh
	c.resetAccumulator()
	for i := 0; i < conf.NumSensors; i++ {
		c.sensorTraces[i] = 0
	}

	// ******************************************************************************************** //
	// **************************  INITIALIZE CORTICAL LAYERS  ********************************** //
	// ******************************************************************************************** //

	// Initialize all three cortical layers with randomized state. Each neuron gets:
	// - A threshold set to the base threshold
	// - A potential just below threshold (so it's ready to fire but not immediately)
	// - A random output trace (simulating recent firing history)
	// - Random weights for all synapses (both sensor and feedback inputs)
	// - Random input traces (simulating recent input activity)
	// - Zero refraction cycles (neurons are immediately active)
	for nrn := 0; nrn < conf.NumNeurons; nrn++ {
		// Layer 1 initialization: First cortical layer, receives sensor input
		c.layer1.Thresholds[nrn] = conf.BaseThreshold
		c.layer1.Potentials[nrn] = conf.BaseThreshold - 1 // Start just below threshold
		c.layer1.OutputTraces[nrn] = 0                    // Cold start: No recent history

		// Layer 2 initialization: Second cortical layer, receives L1 output
		c.layer2.Thresholds[nrn] = conf.BaseThreshold
		c.layer2.Potentials[nrn] = conf.BaseThreshold - 1 // Start just below threshold
		c.layer2.OutputTraces[nrn] = 0                    // Cold start: No recent history

		// Layer 3 initialization: Third cortical layer, receives L2 output
		c.layer3.Thresholds[nrn] = conf.BaseThreshold
		c.layer3.Potentials[nrn] = conf.BaseThreshold - 1 // Start just below threshold
		c.layer3.OutputTraces[nrn] = 0                    // Cold start: No recent history

		// Initialize all synaptic weights and input traces for this neuron
		// Each neuron has NumTotSynapses synapses (sensor inputs + feedback from higher layers)
		for input := 0; input < conf.NumTotSynapses; input++ {
			// Random weights: Each synapse starts with a random strength between 0 and MaxTraceAndWeight
			c.layer1.Weights[nrn][input] = rand.Intn(conf.MaxTraceAndWeight)
			c.layer1.InputTraces[nrn][input] = 0 // Cold start: No recent signals
			c.layer1.WeightResidual[nrn][input] = 0
			c.layer1.MidTermTraces[nrn][input] = 0
			c.layer1.LongTermTraces[nrn][input] = 0

			c.layer2.Weights[nrn][input] = rand.Intn(conf.MaxTraceAndWeight)
			c.layer2.InputTraces[nrn][input] = 0 // Cold start: No recent signals
			c.layer2.WeightResidual[nrn][input] = 0
			c.layer2.MidTermTraces[nrn][input] = 0
			c.layer2.LongTermTraces[nrn][input] = 0

			c.layer3.Weights[nrn][input] = rand.Intn(conf.MaxTraceAndWeight)
			c.layer3.InputTraces[nrn][input] = 0 // Cold start: No recent signals
			c.layer3.WeightResidual[nrn][input] = 0
			c.layer3.MidTermTraces[nrn][input] = 0
			c.layer3.LongTermTraces[nrn][input] = 0
		}
		// All neurons start with zero refraction cycles, meaning they're immediately active
		// and can fire in the first cycle if their potential exceeds threshold
		c.layer1.RefractionCycles[nrn] = 0
		c.layer2.RefractionCycles[nrn] = 0
		c.layer3.RefractionCycles[nrn] = 0
	}

	// ******************************************************************************************** //
	// **************************  INITIALIZE OUTPUT LAYER  ************************************* //
	// ******************************************************************************************** //

	// Output neurons integrate inputs from all layers plus sensors to make decisions.
	// They start with zero potential (completely at rest) and random weights.
	for outputNeuron := 0; outputNeuron < conf.NumOutputNeurons; outputNeuron++ {
		c.outputThresholds[outputNeuron] = conf.BaseThreshold
		c.outputPotentials[outputNeuron] = 0 // Start at rest (zero potential)
		c.outputSomaTraces[outputNeuron] = 0 // No recent firing history
		c.outputRefractionCycles[outputNeuron] = 0
		// Initialize weights for all input synapses (sensors + L1 + L2 + L3)
		for syn := 0; syn < conf.OutputNeuronInputWidth; syn++ {
			c.outputWeights[outputNeuron][syn] = rand.Intn(conf.MaxTraceAndWeight)
			c.outputTagTicks[outputNeuron][syn] = 0
			c.outputTagStrengths[outputNeuron][syn] = 0
		}
	}
	// Initialize output spike matrix (Cold start: all false)
	for cycle := 0; cycle < conf.NumCycles; cycle++ {
		for outputNeuron := 0; outputNeuron < conf.NumOutputNeurons; outputNeuron++ {
			c.outputSpikes[cycle][outputNeuron] = false
		}
	}

	// ******************************************************************************************** //
	// **************************  INITIALIZE SPIKE MATRICES  *********************************** //
	// ******************************************************************************************** //

	// The spike matrices are circular buffers that store spike history for axonal delays.
	// We initialize all entries to false (Cold Start) to ensure the network
	// starts from a clean, silent state.
	for cycle := 0; cycle < conf.NumCycles; cycle++ {
		for input := 0; input < conf.NumTotSynapses; input++ {
			c.layer1.Spikes[cycle][input] = false // Cold start: silent
			c.layer2.Spikes[cycle][input] = false // Cold start: silent
			c.layer3.Spikes[cycle][input] = false // Cold start: silent
		}
	}
}

// RunCortex is the central orchestrator that brings the Cortex to life. This function
// establishes all communication channels, initializes the neural network state, and
// launches the asynchronous handlers that process sensory inputs, execute neural
// computations, and manage the simulation's temporal dynamics.
//
// The function accepts a large number of channels as parameters - this is the "wiring"
// that connects the Cortex to the GUI, sensors, and control systems. Each channel serves
// a specific purpose in the overall architecture.
//
// CRITICAL WASM NOTE: The initialization is wrapped in a goroutine with a delay to
// prevent blocking in WASM's single-threaded environment and to allow the GUI to fully
// initialize before the Cortex starts bombarding it with messages.
func (c *Cortex) RunCortex(
	h *hub.Hub, // 0 - Hub
	ctxControlCh chan comms.ControlMsg, // 1
	ctxHeartbeatGUICh chan int, // 2
	ctxSensorCh chan int, // 3
	ctxResetCh chan bool, // 4
	ctxSoftResetCh chan bool, // 4b
	rawSnsSpikesCh chan int, // 5
	rawSnsrBlankCh chan bool, // 6
	snsSpikesCh chan []bool, // 7
	focusNeuronCh chan comms.ControlMsg, // 8
	l1PotsAndSpikesCh chan comms.PotentialsAndSpikesMsg, // 9
	l2PotsAndSpikesCh chan comms.PotentialsAndSpikesMsg, // 10
	l3PotsAndSpikesCh chan comms.PotentialsAndSpikesMsg, // 11
	l1InputWeightsCh chan []int,
	l1InputTracesCh chan []float64,
	l1ResonanceWeightsCh chan []int,
	l1ResonanceTracesCh chan []float64,
	l1ZeitgeistWeightsCh chan []int,
	l1ZeitgeistTracesCh chan []float64,
	l1FeedbackWeightsCh chan []int, // 14
	l1FeedbackTracesCh chan []float64, // 15
	l1OutputTracesCh chan []float64, // 16
	l2InputWeightsCh chan []int,
	l2InputTracesCh chan []float64,
	l2ResonanceWeightsCh chan []int,
	l2ResonanceTracesCh chan []float64,
	l2ZeitgeistWeightsCh chan []int,
	l2ZeitgeistTracesCh chan []float64,
	l2FeedbackWeightsCh chan []int, // 17
	l2FeedbackTracesCh chan []float64, // 18
	l2OutputTracesCh chan []float64, // 19
	l3InputWeightsCh chan []int,
	l3InputTracesCh chan []float64,
	l3ResonanceWeightsCh chan []int,
	l3ResonanceTracesCh chan []float64,
	l3ZeitgeistWeightsCh chan []int,
	l3ZeitgeistTracesCh chan []float64,
	l3FeedbackWeightsCh chan []int, // 20
	l3FeedbackTracesCh chan []float64, // 21
	l3OutputTracesCh chan []float64, // 22
	fcsPotHistoryCh chan comms.BoolMsg,
	outAHistoryCh chan comms.BoolMsg,
	outBHistoryCh chan comms.BoolMsg,
	outCHistoryCh chan comms.BoolMsg,
	outDHistoryCh chan comms.BoolMsg,
	cumulativeStatsChan chan comms.StringMsg,
	perHeartbeatStatsChan chan comms.StringMsg,
	outGUICh chan int,
	l1FdbkSpikesCh chan []bool,
	l1ResonanceSpikesCh chan []bool,
	l1ZeitgeistSpikesCh chan []bool,
	l2FdbkSpikesCh chan []bool,
	l2ResonanceSpikesCh chan []bool,
	l2ZeitgeistSpikesCh chan []bool,
	l3FdbkSpikesCh chan []bool,
	l3ResonanceSpikesCh chan []bool,
	l3ZeitgeistSpikesCh chan []bool,
	l2InputSpikesCh chan []bool,
	l3InputSpikesCh chan []bool,
	mainPanelCtrlCh chan comms.ControlMsg,
	outAWeightsCh chan []int,
	outBWeightsCh chan []int,
	outCWeightsCh chan []int,
	outDWeightsCh chan []int,
) {
	// ******************************************************************************************** //
	// **************************  WIRE UP COMMUNICATION CHANNELS  ******************************* //
	// ******************************************************************************************** //

	// Connect the hub (message bus) for publishing events
	c.hub = h

	// Wire up statistics channels for monitoring and telemetry
	c.cumulativeStatsCh = cumulativeStatsChan
	c.perHeartbeatStatsCh = perHeartbeatStatsChan

	// Wire up Layer 1 visualization channels (weights and traces for GUI display)
	// Quad Architecture: Four fields per layer
	c.layer1InputWeightsCh = l1InputWeightsCh
	c.layer1InputTracesCh = l1InputTracesCh
	c.layer1ResonanceWeightsCh = l1ResonanceWeightsCh
	c.layer1ResonanceTracesCh = l1ResonanceTracesCh
	c.layer1ZeitgeistWeightsCh = l1ZeitgeistWeightsCh
	c.layer1ZeitgeistTracesCh = l1ZeitgeistTracesCh
	c.layer1FeedbackWeightsCh = l1FeedbackWeightsCh
	c.layer1FeedbackTracesCh = l1FeedbackTracesCh
	c.layer1OutputTracesCh = l1OutputTracesCh

	// Wire up Layer 2 visualization channels
	c.layer2InputWeightsCh = l2InputWeightsCh
	c.layer2InputTracesCh = l2InputTracesCh
	c.layer2ResonanceWeightsCh = l2ResonanceWeightsCh
	c.layer2ResonanceTracesCh = l2ResonanceTracesCh
	c.layer2ZeitgeistWeightsCh = l2ZeitgeistWeightsCh
	c.layer2ZeitgeistTracesCh = l2ZeitgeistTracesCh
	c.layer2FeedbackWeightsCh = l2FeedbackWeightsCh
	c.layer2FeedbackTracesCh = l2FeedbackTracesCh
	c.layer2OutputTracesCh = l2OutputTracesCh

	// Wire up Layer 3 visualization channels
	c.layer3InputWeightsCh = l3InputWeightsCh
	c.layer3InputTracesCh = l3InputTracesCh
	c.layer3ResonanceWeightsCh = l3ResonanceWeightsCh
	c.layer3ResonanceTracesCh = l3ResonanceTracesCh
	c.layer3ZeitgeistWeightsCh = l3ZeitgeistWeightsCh
	c.layer3ZeitgeistTracesCh = l3ZeitgeistTracesCh
	c.layer3FeedbackWeightsCh = l3FeedbackWeightsCh
	c.layer3FeedbackTracesCh = l3FeedbackTracesCh
	c.layer3OutputTracesCh = l3OutputTracesCh

	// Wire up output layer visualization channels
	c.outputAWeightsCh = outAWeightsCh
	c.outputBWeightsCh = outBWeightsCh
	c.outputCWeightsCh = outCWeightsCh
	c.outputDWeightsCh = outDWeightsCh

	// Wire up spike visualization and state channels
	c.sensorSpikesCh = snsSpikesCh
	c.layer1FeedbackSpikesCh = l1FdbkSpikesCh
	c.layer1ResonanceSpikesCh = l1ResonanceSpikesCh
	c.layer1ZeitgeistSpikesCh = l1ZeitgeistSpikesCh
	c.layer2FeedbackSpikesCh = l2FdbkSpikesCh
	c.layer2ResonanceSpikesCh = l2ResonanceSpikesCh
	c.layer2ZeitgeistSpikesCh = l2ZeitgeistSpikesCh
	c.layer3FeedbackSpikesCh = l3FdbkSpikesCh
	c.layer3ResonanceSpikesCh = l3ResonanceSpikesCh
	c.layer3ZeitgeistSpikesCh = l3ZeitgeistSpikesCh
	c.layer2InputSpikesCh = l2InputSpikesCh
	c.layer3InputSpikesCh = l3InputSpikesCh
	c.mainPanelControlCh = mainPanelCtrlCh
	c.focusPotHistoryCh = fcsPotHistoryCh
	c.outputAHistoryCh = outAHistoryCh
	c.outputBHistoryCh = outBHistoryCh
	c.outputCHistoryCh = outCHistoryCh
	c.outputDHistoryCh = outDHistoryCh
	c.layer1PotsAndSpikesCh = l1PotsAndSpikesCh
	c.layer2PotsAndSpikesCh = l2PotsAndSpikesCh
	c.layer3PotsAndSpikesCh = l3PotsAndSpikesCh

	// Wire up raw sensor monitoring channels
	c.rawSensorSpikesCh = rawSnsSpikesCh
	c.rawSensorBlankCh = rawSnsrBlankCh

	// Wire up input and control channels
	c.cortexSensorCh = ctxSensorCh
	c.cortexControlCh = ctxControlCh
	c.cortexResetCh = ctxResetCh
	c.cortexSoftResetCh = ctxSoftResetCh
	c.focusNeuronCh = focusNeuronCh
	c.cortexHeartbeatGUICh = ctxHeartbeatGUICh

	// ******************************************************************************************** //
	// **************************  ASYNCHRONOUS INITIALIZATION  ********************************** //
	// ******************************************************************************************** //

	// WASM FIX: Use goroutine to avoid blocking in WASM single-threaded environment.
	// This also de-synchronizes the sim and cortex initialization, and allows the GUI
	// to fully spin up before the Cortex starts bombarding it with messages. Without
	// this delay, the GUI channels can fill up and block, causing deadlocks.
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("PANIC in RunCortex init: %v\nStack: %s", r, debug.Stack())
			}
		}()

		// Delay initialization to allow GUI to fully start up. This prevents channel
		// buffer overflows and ensures the GUI is ready to receive messages.
		time.Sleep(time.Millisecond * 510)

		// Set spontaneous firing probability from configuration
		c.spontaneousFire = float64(conf.SpontaneousSpikeProbability)

		// Initialize all neural state (weights, traces, potentials) to random values
		c.initialize()

		// CRITICAL: Process any queued control messages (e.g., WASM tick rate) BEFORE creating ticker.
		// This is synchronous to avoid race conditions in single-threaded WASM. If a tick rate
		// message was sent before initialization, we need to process it now so the ticker
		// is created with the correct duration.
		select {
		case controlMsg := <-c.cortexControlCh:
			if controlMsg.Type == 0 { // Tick rate control message
				c.lastTickerDuration = time.Millisecond * time.Duration(controlMsg.IntValue)
			}
		default:
			// No queued message, use default from initialize()
		}

		// Create the tickers that drive the simulation. The heartbeat ticker runs at a
		// fixed rate for telemetry, while the core ticker can be dynamically adjusted.
		c.cortexHeartbeatTicker = time.NewTicker(conf.HeartbeatRate)
		c.cortexCoreTicker = time.NewTicker(c.lastTickerDuration)

		// ******************************************************************************************** //
		// **************************  LAUNCH ASYNCHRONOUS HANDLERS  ****************************** //
		// ******************************************************************************************** //

		// Start all the goroutines that handle different aspects of the simulation:
		// - Control messages: Handle user commands (pause, speed, dopamine, etc.)
		// - Heartbeat: Periodic telemetry and GUI updates
		// - Core tick: Main simulation loop (processes cycles)
		// - Reset handlers: Respond to reset requests
		// - Sensor accumulator: Collect asynchronous sensor signals
		go c.handleControlMessages()
		// go c.handleFocusNeuronChanges() // REMOVED: Dead code, potential race condition
		go c.handleHeartbeat()
		go c.handleCoreTick()
		go c.handleReset()
		go c.handleSoftReset()
		go c.accumulateSensorSignals()
	}()
}

// runListeners is a legacy function that launches all the handler goroutines.
// This is kept for compatibility but is not currently used (handlers are launched
// directly from RunCortex). It may be useful for testing or alternative initialization paths.
func (c *Cortex) runListeners() {
	go c.handleControlMessages()
	// go c.handleFocusNeuronChanges() // REMOVED: Dead code
	go c.handleHeartbeat()
	go c.handleCoreTick()
	go c.handleReset()
	go c.handleSoftReset()

	// This loop ensures the main function doesn't exit
	//select {}
}

// handleControlMessages processes control commands from the GUI or external controllers.
// This handler runs in its own goroutine and continuously listens for control messages,
// allowing real-time adjustment of simulation parameters without blocking the main loop.
//
// Message types:
//
//	0: Adjust core tick rate (simulation speed)
//	1: Adjust dopamine level (learning rate)
//	2: Pause/resume simulation
//	3: Set focus neuron for visualization
//	4: Single-step the simulation (manual cycle)
//	5: Adjust spontaneous firing probability
//	6: Reset sensor accumulator (discard intra-tick arrivals)
//	7: Full reset (reinitialize cortex)
//	8: Soft reset (reinitialize, preserve control params)
func (c *Cortex) handleControlMessages() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC in handleControlMessages: %v\nStack: %s", r, debug.Stack())
		}
	}()
	for controlMessage := range c.cortexControlCh {
		switch controlMessage.Type {
		case comms.CortexControlTickRate: // RESET core speed
			// Adjust the simulation tick rate. The IntValue is in milliseconds per tick.
			c.lastTickerDuration = time.Millisecond * time.Duration(controlMessage.IntValue)
			// Only reset if ticker exists AND we are not paused (can't reset a stopped ticker)
			if c.cortexCoreTicker != nil && !c.paused {
				c.cortexCoreTicker.Reset(c.lastTickerDuration)
			}
		case comms.CortexControlDopamine: // dopamine
			// Set dopamine level. IntValue is in percentage (0-100), convert to 0.0-1.0 range.
			c.dopamine = float64(controlMessage.IntValue) / 100.0
		case comms.CortexControlPause: // pause / resume
			// Pause or resume the simulation. When paused, the core ticker stops but
			// control messages can still be processed.
			if c.cortexCoreTicker != nil {
				if controlMessage.BoolValue {
					c.cortexCoreTicker.Stop()
					c.paused = true
				} else {
					c.cortexCoreTicker.Reset(c.lastTickerDuration)
					c.paused = false
				}
			}
		case comms.CortexControlFocus: // set focus neuron
			// Change which neuron is being visualized in detail. The GUI will display
			// this neuron's weights, traces, and potential history.
			// log.Printf("cortex: ControlMsg Type 3 received. Setting focus to Layer %d, Neuron %d", controlMessage.Layer, controlMessage.IntValue)
			c.focusNeuron = controlMessage.IntValue
			c.focusLayer = controlMessage.Layer
		case comms.CortexControlStep: // step cycle
			// Manually trigger a single simulation cycle. Useful for debugging or
			// step-by-step analysis of network behavior.
			c.cycle()
		case comms.CortexControlSpontaneousFire: // spontaneous fire probability
			// Adjust the probability of spontaneous firing. IntValue is per 10,000 chance.
			c.spontaneousFire = float64(controlMessage.IntValue)
		case comms.CortexControlShortTraceHalfLifeMs: // short-trace half-life (ms)
			conf.SetShortTraceHalfLifeSeconds(float64(controlMessage.IntValue) / 1000.0)
		case comms.CortexControlMidTraceHalfLifeMs: // mid-trace half-life (ms)
			conf.SetMidTraceHalfLifeSeconds(float64(controlMessage.IntValue) / 1000.0)
		case comms.CortexControlLongTraceHalfLifeMs: // long-trace half-life (ms)
			conf.SetLongTraceHalfLifeSeconds(float64(controlMessage.IntValue) / 1000.0)
		case comms.CortexControlLongEligibilityGate: // eligibility gate (scaled)
			conf.SetLongEligibilityGate(float64(controlMessage.IntValue) / 1000.0)
		case comms.CortexControlTraceDisplayMode: // trace display mode (0=Short, 1=Mid, 2=Long)
			if controlMessage.IntValue >= comms.TraceDisplayShort && controlMessage.IntValue <= comms.TraceDisplayLong {
				c.traceDisplayMode = controlMessage.IntValue
			}
		case comms.CortexControlResetAccumulator: // reset accumulator
			// Clears the sensor integration buffer without touching any learned state.
			c.resetAccumulator()
		case comms.CortexControlReset: // full reset
			c.initialize()
		case comms.CortexControlSoftReset: // soft reset
			c.softInitialize()
		}
	}
}

// func (c *Cortex) handleFocusNeuronChanges() {
// 	for val := range c.focusNeuronCh {
// 		c.focusNeuron = val.IntValue
// 		c.focusLayer = val.Layer
// 	}
// }

// handleHeartbeat runs at a fixed rate (HeartbeatRate) and performs periodic maintenance:
// - Sends heartbeat number to GUI
// - Sends telemetry statistics
// - Resets per-heartbeat counters
// The heartbeat is slower than the core tick, providing a metronome for GUI updates
// and statistics collection without overwhelming the system.
func (c *Cortex) handleHeartbeat() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC in handleHeartbeat: %v\nStack: %s", r, debug.Stack())
		}
	}()
	for range c.cortexHeartbeatTicker.C {
		// Send heartbeat number to GUI (non-blocking)
		select {
		case c.cortexHeartbeatGUICh <- c.cortexHeartbeatNum:
		default:
		}
		// Send all telemetry data (tick counts, signal statistics, etc.)
		c.sendLowLevelTelemetryToHub()
		// Publish learning controls for observability tools.
		c.sendLearningStateToHub()
		// Save current tick for next heartbeat's delta calculation
		c.lastCortexTick = c.cortexTick
		// Reset per-heartbeat counters for fresh statistics
		c.signalsReceivedThisHeartbeat = 0
		c.signalsAccumulatedThisHeartbeat = 0
		// Increment heartbeat counter
		c.cortexHeartbeatNum++
	}
}

// handleCoreTick drives the main simulation loop. Each tick triggers a complete cycle
// of neural processing: traces, potentials, spikes, learning, and state updates.
// This is the heartbeat of the simulation, running at the rate specified by
// lastTickerDuration (which can be dynamically adjusted via control messages).
func (c *Cortex) handleCoreTick() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC in handleCoreTick: %v\nStack: %s", r, debug.Stack())
		}
	}()
	for range c.cortexCoreTicker.C {
		c.cycle()
	}
}

// handleReset listens for reset requests and performs a full reinitialization of
// the Cortex. This resets all weights, traces, and state to random initial values,
// effectively creating a fresh network ready for new learning.
func (c *Cortex) handleReset() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC in handleReset: %v\nStack: %s", r, debug.Stack())
		}
	}()
	for range c.cortexResetCh {
		c.initialize()
	}
}

// handleSoftReset listens for soft reset requests. Unlike a full reset, a soft reset
// preserves control parameters (dopamine, tick rate, focus neuron) but resets all
// neural state. This is useful for testing learning dynamics without losing the
// current experimental configuration.
func (c *Cortex) handleSoftReset() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC in handleSoftReset: %v\nStack: %s", r, debug.Stack())
		}
	}()
	for range c.cortexSoftResetCh {
		c.softInitialize()
	}
}

// softInitialize performs a "gentle" reset that preserves control parameters (dopamine,
// tick rate, focus neuron) while resetting all neural state. This is useful when you
// want to test learning dynamics with the same experimental setup but a fresh network.
//
// Unlike initialize(), this function does NOT reset:
//   - c.dopamine (preserved)
//   - c.lastTickerDuration (preserved)
//   - c.focusLayer (preserved)
//   - c.focusNeuron (preserved)
//
// Everything else (weights, traces, potentials, thresholds) is reset to random values.
func (c *Cortex) softInitialize() {
	// Reset tick counter but preserve control parameters
	c.cortexTick = 1
	// Preserved: c.dopamine, c.lastTickerDuration, c.focusLayer, c.focusNeuron

	// Clear the sensor accumulator and traces
	c.resetAccumulator()
	for i := 0; i < conf.NumSensors; i++ {
		c.sensorTraces[i] = 0
	}

	// Reset all cortical neurons to random initial state
	for nrn := 0; nrn < conf.NumNeurons; nrn++ {
		// Note: Thresholds are reset (not preserved) - they're part of neural state
		c.layer1.Potentials[nrn] = conf.BaseThreshold - 1
		c.layer1.OutputTraces[nrn] = 0

		c.layer2.Potentials[nrn] = conf.BaseThreshold - 1
		c.layer2.OutputTraces[nrn] = 0

		c.layer3.Potentials[nrn] = conf.BaseThreshold - 1
		c.layer3.OutputTraces[nrn] = 0

		// Randomize all weights and input traces
		for input := 0; input < conf.NumTotSynapses; input++ {
			c.layer1.Weights[nrn][input] = rand.Intn(conf.MaxTraceAndWeight)
			c.layer1.InputTraces[nrn][input] = 0
			c.layer1.WeightResidual[nrn][input] = 0
			c.layer1.MidTermTraces[nrn][input] = 0
			c.layer1.LongTermTraces[nrn][input] = 0

			c.layer2.Weights[nrn][input] = rand.Intn(conf.MaxTraceAndWeight)
			c.layer2.InputTraces[nrn][input] = 0
			c.layer2.WeightResidual[nrn][input] = 0
			c.layer2.MidTermTraces[nrn][input] = 0
			c.layer2.LongTermTraces[nrn][input] = 0

			c.layer3.Weights[nrn][input] = rand.Intn(conf.MaxTraceAndWeight)
			c.layer3.InputTraces[nrn][input] = 0
			c.layer3.WeightResidual[nrn][input] = 0
			c.layer3.MidTermTraces[nrn][input] = 0
			c.layer3.LongTermTraces[nrn][input] = 0
		}
		// Reset refraction cycles (neurons are immediately active)
		c.layer1.RefractionCycles[nrn] = 0
		c.layer2.RefractionCycles[nrn] = 0
		c.layer3.RefractionCycles[nrn] = 0
	}
	// Reset output neurons
	for outputNeuron := 0; outputNeuron < conf.NumOutputNeurons; outputNeuron++ {
		c.outputThresholds[outputNeuron] = conf.BaseThreshold
		c.outputPotentials[outputNeuron] = 0
		c.outputSomaTraces[outputNeuron] = 0
		// Randomize output weights
		for syn := 0; syn < conf.OutputNeuronInputWidth; syn++ {
			c.outputWeights[outputNeuron][syn] = rand.Intn(conf.MaxTraceAndWeight)
		}
	}
	// Reset spike matrices to warm start (all true)
	// Reset output spike matrix (Cold start: false)
	for cycle := 0; cycle < conf.NumCycles; cycle++ {
		for outputNeuron := 0; outputNeuron < conf.NumOutputNeurons; outputNeuron++ {
			c.outputSpikes[cycle][outputNeuron] = false
		}
	}
	// Reset layer spike matrices (Cold start: false)
	for cycle := 0; cycle < conf.NumCycles; cycle++ {
		for input := 0; input < conf.NumTotSynapses; input++ {
			c.layer1.Spikes[cycle][input] = false
			c.layer2.Spikes[cycle][input] = false
			c.layer3.Spikes[cycle][input] = false
		}
	}
}

// 🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁
// CYCLE: The Heartbeat of the Simulation
// 🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁  🏁
//
// cycle performs a single pass of the Cortex's processing pipeline. This is the main
// simulation loop that orchestrates all neural computations in the correct order:
//
// 1. Move accumulated sensor signals into the spike matrix
// 2. Process each cortical layer (L1 → L2 → L3) sequentially:
//   - Calculate input traces (decay from previous cycle, update from spikes)
//   - Calculate membrane potentials (weighted sum of inputs)
//   - Calculate spikes (fire if above threshold, leak otherwise)
//   - Update weights via STDP learning (modulated by density and adaptability)
//
// 3. Propagate spikes from each layer to the next
// 4. Process the output layer (integrates all layers + sensors)
// 5. Send feedback spikes to higher layers
// 6. Send state to GUI for visualization
// 7. Advance to the next cycle phase
// 8. Publish tick event for simulator synchronization
//
// The cycle is called repeatedly by handleCoreTick() at the rate specified by
// lastTickerDuration, creating the temporal dynamics of the simulation.
func (c *Cortex) cycle() {
	// Step 0: Process any pending reinforcement pulses (Dopamine retribution)
	// This ensures weight changes happen synchronously between simulation steps.
	for {
		select {
		case profit := <-c.retributionCh:
			c.applyRetributionSynchronous(profit)
		default:
			goto doneRetribution
		}
	}
doneRetribution:

	// Step 1: Move any sensor signals that arrived asynchronously into the spike matrix
	// for this cycle. The accumulator collects signals between cycles, and now we
	// transfer them to the synchronous processing pipeline.
	c.moveAccumulatorToSpikeMatrixAndReset()
	c.updateSensorTraces()

	// Step 2: Process each cortical layer sequentially. The layers form a hierarchy:
	// L1 receives sensor input, L2 receives L1 output, L3 receives L2 output.
	layers := []*layer{&c.layer1, &c.layer2, &c.layer3}
	for i, layer := range layers {
		// Process this layer: traces → potentials → spikes → learning
		c.processLayer(layer)

		// Step 3: Propagate this layer's spikes to the next layer (if not the last layer).
		// The spikes are written into the next layer's spike matrix at the appropriate
		// future cycle to account for axonal delays.
		if i < len(layers)-1 {
			c.propagateSpikesToNextLayer(layer, layers[i+1])
		}
	}

	// Step 4: Process the output layer, which integrates inputs from all three layers
	// plus direct sensor inputs to make decisions or predictions.
	c.calculateOutputLayer()

	// Step 5: Send feedback spikes (from higher layers back to lower layers) and
	// calculate density metrics for chaos-based learning modulation.
	c.sendFeedbackSpikes()

	// Step 6: Send current state (potentials, spikes, weights, traces) to the hub
	// for visualization. This keeps GUI decoupled from the core process.
	c.sendStateToHub()

	// Step 7: Advance the cycle phase (circular buffer position) and increment tick counter
	c.prepareNextCycle()

	// Step 8: Publish tick event to the hub for simulator synchronization.
	// This happens AFTER processing, so the simulator fires signals into the NEXT
	// tick's accumulator, maintaining proper temporal ordering.
	if c.hub != nil {
		c.hub.Publish(hub.TopicCortexTick, c.cortexTick)
	}
}

// processLayer orchestrates the complete processing pipeline for a single cortical layer.
// This function performs all the neural computations in the correct order:
//
// 1. Calculate Input Traces: Decay traces from previous cycle, update with new spikes
// 2. Calculate Potentials: Sum weighted inputs to get membrane potential
// 3. Calculate Spikes: Determine which neurons fire (above threshold) or leak (below)
// 4. Learning: Update synaptic weights via STDP, modulated by density and adaptability
//
// The density metric used for learning modulation depends on which layer this is:
// - L1 uses sensor density (how chaotic are the inputs?)
// - L2 uses L1 density (how chaotic is L1's representation?)
// - L3 uses L2 density (how chaotic is L2's representation?)
//
// This creates a cascade where each layer adapts to the chaos level of its inputs,
// preventing runaway learning in chaotic conditions.
func (c *Cortex) processLayer(layer *layer) {
	// Determine inputDensity for this layer to modulate learning and quenching.
	// L1 learns from Sensors, L2 from L1, L3 from L2.
	var inputDensity float32
	if layer == &c.layer1 {
		inputDensity = c.MetricSensorDensity
	} else if layer == &c.layer2 {
		inputDensity = c.MetricLayer1Density
	} else if layer == &c.layer3 {
		inputDensity = c.MetricLayer2Density
	}

	// 1. Calculate Input Traces
	// Traces decay over time and are refreshed when spikes arrive. They represent
	// the "memory" of recent activity, used both for potential calculation and learning.
	layer.calculateInputTraces(c.cyclePhase)

	// 2. Calculate Potentials
	// Each neuron's potential is the weighted sum of its inputs. If potential exceeds
	// threshold, the neuron will fire. Otherwise, it will leak (decay).
	// We pass the inputDensity to apply non-linear damping (Zeitgeist Field 3).
	layer.calculatePotentials(float64(inputDensity))

	// 3. Calculate Spikes (Fire/Leak)
	// Neurons above threshold fire (reset potential, enter refraction). Neurons below
	// threshold leak (decay potential). Spontaneous firing can occur even below threshold.
	layer.calculateSpikes(c.cyclePhase, c.spontaneousFire, c.cortexTick)

	// 4. Learning (Weights)
	// Adaptability = 0 => Rate = 0 (Freeze learning completely)
	// Adaptability = 1 => Rate = Base * (1 - Density) (Fast adaptation, slowed by chaos)
	// The formula (1 - Density) means: high density (chaos) → slow learning, low density → fast learning
	learningModulation := c.Adaptability * (1.0 - float64(inputDensity))

	// Update weights via STDP (Spike-Timing-Dependent Plasticity). The learning rate
	// is modulated by both the adaptability parameter and the density metric.
	layer.calculateWeights(learningModulation, c.dopamine)
}

// propagateSpikesToNextLayer copies spikes from the current layer's output to the next
// layer's input. This creates the hierarchical flow: L1 → L2 → L3.
//
// With the inverted delay mechanism, spikes are written to the current cycle's vertical
// column. When the next layer reads these spikes, it will read diagonally (from past cycles
// based on source neuron delays). To ensure spikes are visible at the right time, we write
// them to the appropriate cycle in the next layer's spike matrix.
func (c *Cortex) propagateSpikesToNextLayer(currentLayer, nextLayer *layer) {
	// Copy spikes from current layer's output field to next layer's input field
	// Current layer's neurons fire at cyclePhase and write to cyclePhase
	// Next layer will read these spikes at nextCycle (cyclePhase + 1), reading diagonally
	// So we write to nextCycle in nextLayer's spike matrix
	nextCycle := (c.cyclePhase + 1) % conf.NumCycles
	for inputSlot := 0; inputSlot < conf.NumSensors; inputSlot++ {
		// Current layer's output is at Gestalt field [192-255]
		// Each inputSlot corresponds to a neuron in currentLayer (neuron index = inputSlot)
		// Current layer neuron fires at cyclePhase, writes to Gestalt field
		// Read from current layer's current cycle, Gestalt field
		gestaltSyn := conf.QuadGestaltStart + inputSlot // [192-255]
		nextLayer.Spikes[nextCycle][inputSlot] = currentLayer.Spikes[c.cyclePhase][gestaltSyn]
	}
}

// prepareNextCycle advances the simulation to the next temporal step. This updates
// the cycle phase (position in the circular buffer) and increments the tick counter.
// The cycle phase wraps around using modulo arithmetic to create a circular buffer
// that stores spike history for axonal delays.
func (c *Cortex) prepareNextCycle() {
	c.cyclePhase = (c.cyclePhase + 1) % conf.NumCycles // Wrap around circular buffer
	c.cortexTick++                                     // Increment tick for the next cycle
}

// ApplyRetribution pushes a profit/loss signal to the retribution queue.
// This is thread-safe and will be processed at the start of the next cycle.
func (c *Cortex) ApplyRetribution(profit float64) {
	select {
	case c.retributionCh <- profit:
	default:
		// Channel full, drop retribution (should be rare)
		log.Printf("cortex: Retribution queue full, dropping pulse (%.4f)", profit)
	}
}

func (c *Cortex) applyRetributionSynchronous(profit float64) {
	// 1. Reinforce Cortical Layers
	layers := []*layer{&c.layer1, &c.layer2, &c.layer3}
	for _, l := range layers {
		l.applyRetribution(profit, c.cortexTick)
	}

	// 2. Reinforce Output Layer (The Decision Maker)
	for nrn := 0; nrn < conf.NumOutputNeurons; nrn++ {
		for syn := 0; syn < conf.OutputNeuronInputWidth; syn++ {
			tagTick := c.outputTagTicks[nrn][syn]
			if tagTick == 0 {
				continue
			}

			age := c.cortexTick - tagTick
			if age >= 0 && age < conf.MaxTagAge {
				// Capture the tag: Proportionally strengthen or weaken based on profit
				strengthRatio := float64(c.outputTagStrengths[nrn][syn]) / float64(conf.MaxTraceAndWeight)
				delta := int(profit * conf.DopamineGain * float64(conf.MaxTraceAndWeight) * strengthRatio)
				c.outputWeights[nrn][syn] += delta

				// Clamp weights
				if c.outputWeights[nrn][syn] > conf.MaxTraceAndWeight {
					c.outputWeights[nrn][syn] = conf.MaxTraceAndWeight
				} else if c.outputWeights[nrn][syn] < 0 {
					c.outputWeights[nrn][syn] = 0
				}

				// Clear tag after capture
				c.outputTagTicks[nrn][syn] = 0
				c.outputTagStrengths[nrn][syn] = 0
			} else if age > conf.MaxTagAge*2 || age < 0 {
				// Garbage collect very old or invalid tags
				c.outputTagTicks[nrn][syn] = 0
				c.outputTagStrengths[nrn][syn] = 0
			}
		}
	}
}

func (l *layer) applyRetribution(profit float64, currentTick int) {
	for nrn := 0; nrn < conf.NumNeurons; nrn++ {
		for syn := 0; syn < conf.NumTotSynapses; syn++ {
			tagTick := l.TagTicks[nrn][syn]
			if tagTick == 0 {
				continue
			}

			age := currentTick - tagTick
			if age >= 0 && age < conf.MaxTagAge {
				// Capture the tag: Proportionally strengthen or weaken based on profit
				// We scale the impact by how strongly this synapse was contributing (TagStrength)
				strengthRatio := float64(l.TagStrengths[nrn][syn]) / float64(conf.MaxTraceAndWeight)
				delta := int(profit * conf.DopamineGain * float64(conf.MaxTraceAndWeight) * strengthRatio)
				l.Weights[nrn][syn] += delta

				// Clamp weights to valid range
				if l.Weights[nrn][syn] > conf.MaxTraceAndWeight {
					l.Weights[nrn][syn] = conf.MaxTraceAndWeight
				} else if l.Weights[nrn][syn] < 0 {
					l.Weights[nrn][syn] = 0
				}

				// Clear tag after capture
				l.TagTicks[nrn][syn] = 0
				l.TagStrengths[nrn][syn] = 0
			} else if age > conf.MaxTagAge*2 || age < 0 {
				// Garbage collect very old or invalid tags
				l.TagTicks[nrn][syn] = 0
				l.TagStrengths[nrn][syn] = 0
			}
		}
	}
}

// sendFeedbackSpikes determines the collective firing density of each layer
// and triggers the GUI update for all Quad Architecture fields.
func (c *Cortex) sendFeedbackSpikes() {
	// 1. Logic (Densities)
	// Sensor and input spikes are read from current cycle (vertical column)
	c.MetricSensorDensity = c.calculateMetricDensity(c.layer1.Spikes[c.cyclePhase][0:conf.NumSensors])
	c.MetricLayer1Density = c.calculateMetricDensity(c.layer2.Spikes[c.cyclePhase][0:conf.NumSensors]) // L1 -> L2 inputs
	c.MetricLayer2Density = c.calculateMetricDensity(c.layer3.Spikes[c.cyclePhase][0:conf.NumSensors]) // L2 -> L3 inputs

	// Feedback spikes are read diagonally (accounting for axonal delays)
	layer3VisibleFeedback := c.layer3.getVisibleFeedbackSpikes(c.cyclePhase)
	c.MetricLayer3Density = c.calculateMetricDensity(layer3VisibleFeedback)

	// 2. Monitoring (Hub Wiring)
	c.sendSpikesToHub(layer3VisibleFeedback)
}

// ////////////////////////////////////////////////////////////////////////////////////////////////////////////
// calculateOutputLayer is defined in output.go
