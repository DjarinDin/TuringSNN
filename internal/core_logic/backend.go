package core_logic

/*
backend.go is the "Grand Central Station" of the Turing architecture.

It acts as the initialization factory and wiring diagram for the entire backend system,
performing 6 critical functions:

 1. Dependency Injection & Wiring: It connects all components (Cortex, Sim, Hub).
 2. Channel Factory: It initializes ~50 buffered channels that act as the system's "nervous system".
 3. Hub Creation: It spins up the central Event Hub (Pub/Sub) for decoupling components.
 4. Fan-Out Setup: It wires the Hub to "spy" on the Cortex. Signals are split/fan-out:
    - One copy goes to the JS Frontend (via Bridge) for visualization.
    - One copy goes to the Hub for internal monitoring.
 5. Lifecycle Management: It boots up the Cortex (Brain) and the Sim (Sensors).
 6. API Surface: It wraps everything in a Backend struct for the WASM Bridge.
*/

import (
	"github.com/DjarinDin/TuringSNN/internal/chaos"
	"github.com/DjarinDin/TuringSNN/internal/core_logic/cortex"
	"github.com/DjarinDin/TuringSNN/internal/core_logic/hub"
	"github.com/DjarinDin/TuringSNN/internal/core_logic/sim"
	"github.com/DjarinDin/TuringSNN/pkg/api"
	"github.com/DjarinDin/TuringSNN/pkg/comms"
	"log"
	"runtime"
)

// WorldSource feeds the cortex's sensor channel with an external data stream.
// This repository ships no implementation beyond the synthetic Sim itself —
// a deployment wanting a different input source supplies its own via
// NewBackend's variadic sources parameter.
type WorldSource interface {
	Start()
	Stop()
}

// WorldSourceFactory constructs a WorldSource once the sensor channel and hub
// exist (they're created inside NewBackend, so sources can't be constructed
// before it runs). A private deployment defines these against its own
// unexported source types and passes the factories in.
type WorldSourceFactory func(sensorCh chan int, h *hub.Hub) WorldSource

type Backend struct {
	channels       *api.BackendChannels
	cortex         *cortex.Cortex
	sim            *sim.Sim
	worldSources   []WorldSource
	hub            *hub.Hub
	metricsHandler *chaos.MetricsHandler
	broker         *sim.PositionTracker
}

func NewBackend(sources ...WorldSourceFactory) *Backend {
	// Check if running in WASM
	isWASM := runtime.GOOS == "js"

	// WASM requires buffered channels to prevent deadlocks in single-threaded environment
	bufSize := 0
	if isWASM {
		bufSize = 1000
	}

	// Build channels
	// CONTROL
	simControlCh := make(chan comms.IntMsg, bufSize)

	mainPanelControlCh := make(chan comms.ControlMsg, bufSize)

	cortexControlCh := make(chan comms.ControlMsg, bufSize)
	focusNeuronCh := make(chan comms.ControlMsg, bufSize)

	cortexResetCh := make(chan bool, bufSize)
	cortexSoftResetCh := make(chan bool, bufSize)
	sensorSpikesCh := make(chan []bool, bufSize)
	layer1FeedbackSpikesCh := make(chan []bool, bufSize)
	layer1ResonanceSpikesCh := make(chan []bool, bufSize)
	layer1ZeitgeistSpikesCh := make(chan []bool, bufSize)
	layer2FeedbackSpikesCh := make(chan []bool, bufSize)
	layer2ResonanceSpikesCh := make(chan []bool, bufSize)
	layer2ZeitgeistSpikesCh := make(chan []bool, bufSize)
	layer3FeedbackSpikesCh := make(chan []bool, bufSize)
	layer3ResonanceSpikesCh := make(chan []bool, bufSize)
	layer3ZeitgeistSpikesCh := make(chan []bool, bufSize)
	layer2InputSpikesCh := make(chan []bool, bufSize)
	layer3InputSpikesCh := make(chan []bool, bufSize)

	// HEARTBEATS
	simHeartbeatGUICh := make(chan int, bufSize)
	cortexHeartbeatGUICh := make(chan int, bufSize)

	cortexSensorCh := make(chan int, bufSize)

	l1PotsAndSpikesCh := make(chan comms.PotentialsAndSpikesMsg, bufSize)
	l2PotsAndSpikesCh := make(chan comms.PotentialsAndSpikesMsg, bufSize)
	l3PotsAndSpikesCh := make(chan comms.PotentialsAndSpikesMsg, bufSize)

	rawSensorSpikesCh := make(chan int, bufSize)
	rawSensorBlankCh := make(chan bool, bufSize)

	layer1InputWeightsCh := make(chan []int, bufSize)
	layer1InputTracesCh := make(chan []float64, bufSize)
	layer1ResonanceWeightsCh := make(chan []int, bufSize)
	layer1ResonanceTracesCh := make(chan []float64, bufSize)
	layer1ZeitgeistWeightsCh := make(chan []int, bufSize)
	layer1ZeitgeistTracesCh := make(chan []float64, bufSize)
	layer1FeedbackWeightsCh := make(chan []int, bufSize)
	layer1FeedbackTracesCh := make(chan []float64, bufSize)
	layer1OutputTracesCh := make(chan []float64, bufSize)

	layer2InputWeightsCh := make(chan []int, bufSize)
	layer2InputTracesCh := make(chan []float64, bufSize)
	layer2ResonanceWeightsCh := make(chan []int, bufSize)
	layer2ResonanceTracesCh := make(chan []float64, bufSize)
	layer2ZeitgeistWeightsCh := make(chan []int, bufSize)
	layer2ZeitgeistTracesCh := make(chan []float64, bufSize)
	layer2FeedbackWeightsCh := make(chan []int, bufSize)
	layer2FeedbackTracesCh := make(chan []float64, bufSize)
	layer2OutputTracesCh := make(chan []float64, bufSize)

	layer3InputWeightsCh := make(chan []int, bufSize)
	layer3InputTracesCh := make(chan []float64, bufSize)
	layer3ResonanceWeightsCh := make(chan []int, bufSize)
	layer3ResonanceTracesCh := make(chan []float64, bufSize)
	layer3ZeitgeistWeightsCh := make(chan []int, bufSize)
	layer3ZeitgeistTracesCh := make(chan []float64, bufSize)
	layer3FeedbackWeightsCh := make(chan []int, bufSize)
	layer3FeedbackTracesCh := make(chan []float64, bufSize)
	layer3OutputTracesCh := make(chan []float64, bufSize)

	outputAWeightsCh := make(chan []int, bufSize)
	outputBWeightsCh := make(chan []int, bufSize)
	outputCWeightsCh := make(chan []int, bufSize)
	outputDWeightsCh := make(chan []int, bufSize)

	focusPotentialHistoryCh := make(chan comms.BoolMsg, bufSize)
	outputAHistoryCh := make(chan comms.BoolMsg, bufSize)
	outputBHistoryCh := make(chan comms.BoolMsg, bufSize)
	outputCHistoryCh := make(chan comms.BoolMsg, bufSize)
	outputDHistoryCh := make(chan comms.BoolMsg, bufSize)

	cumulativeStatsGUICh := make(chan comms.StringMsg, 300+bufSize)
	perHeartbeatStatsGUICh := make(chan comms.StringMsg, 300+bufSize)

	outputGUICh := make(chan int, bufSize)

	// Initialize hub for multi-client support (primary telemetry path).
	log.Println("backend: Initializing message hub...")
	h := hub.NewHub()
	if isWASM {
		h.SetBufferSize(1000) // Increase buffer for WASM to prevent dropped frames/slow scrolling
	}

	// Add test subscriber to verify hub receives data
	go func() {
		testCh := h.Subscribe("test-monitor",
			hub.TopicCortexHeartbeat,
			hub.TopicLayer1Potentials,
		)
		heartbeatCount := 0
		layer1Count := 0

		for msg := range testCh {
			switch msg.(type) {
			case int:
				heartbeatCount++
				if heartbeatCount%100 == 0 {
					log.Printf("hub: Test monitor - %d heartbeats, %d layer1 updates", heartbeatCount, layer1Count)
					sent, dropped, subs := h.GetStats()
					log.Printf("hub: Stats - Sent: %d, Dropped: %d, Subscribers: %d", sent, dropped, subs)
				}
			case []float64:
				layer1Count++
			}
		}
	}()

	log.Println("backend: Hub initialized (telemetry flows via hub)")

	// UDP and Websocket code removed as dependencies are gone.

	// Start cortex with direct channels (hub publishes are handled inside cortex).
	c := &cortex.Cortex{}
	c.RunCortex(
		h,                    // 0 - Hub
		cortexControlCh,      // 1 (not fan-out - control input)
		cortexHeartbeatGUICh, // 2
		cortexSensorCh,       // 3 (not fan-out - cortex input)
		cortexResetCh,        // 4 (not fan-out - control input)
		cortexSoftResetCh,    // 4b (not fan-out - control input)
		rawSensorSpikesCh,    // 5
		rawSensorBlankCh,     // 6
		sensorSpikesCh,       // 7
		focusNeuronCh,        // 8 (not fan-out - control input)
		l1PotsAndSpikesCh,    // 9
		l2PotsAndSpikesCh,    // 10
		l3PotsAndSpikesCh,    // 11
		layer1InputWeightsCh,
		layer1InputTracesCh,
		layer1ResonanceWeightsCh,
		layer1ResonanceTracesCh,
		layer1ZeitgeistWeightsCh,
		layer1ZeitgeistTracesCh,
		layer1FeedbackWeightsCh, // 14
		layer1FeedbackTracesCh,  // 15
		layer1OutputTracesCh,    // 16
		layer2InputWeightsCh,
		layer2InputTracesCh,
		layer2ResonanceWeightsCh,
		layer2ResonanceTracesCh,
		layer2ZeitgeistWeightsCh,
		layer2ZeitgeistTracesCh,
		layer2FeedbackWeightsCh, // 17
		layer2FeedbackTracesCh,  // 18
		layer2OutputTracesCh,    // 19
		layer3InputWeightsCh,
		layer3InputTracesCh,
		layer3ResonanceWeightsCh,
		layer3ResonanceTracesCh,
		layer3ZeitgeistWeightsCh,
		layer3ZeitgeistTracesCh,
		layer3FeedbackWeightsCh, // 20
		layer3FeedbackTracesCh,  // 21
		layer3OutputTracesCh,    // 22
		focusPotentialHistoryCh,
		outputAHistoryCh,
		outputBHistoryCh,
		outputCHistoryCh,
		outputDHistoryCh,
		cumulativeStatsGUICh,
		perHeartbeatStatsGUICh,
		outputGUICh,
		layer1FeedbackSpikesCh,
		layer1ResonanceSpikesCh,
		layer1ZeitgeistSpikesCh,
		layer2FeedbackSpikesCh,
		layer2ResonanceSpikesCh,
		layer2ZeitgeistSpikesCh,
		layer3FeedbackSpikesCh,
		layer3ResonanceSpikesCh,
		layer3ZeitgeistSpikesCh,
		layer2InputSpikesCh,
		layer3InputSpikesCh,
		mainPanelControlCh, // (not fan-out - control input)
		outputAWeightsCh,
		outputBWeightsCh,
		outputCWeightsCh,
		outputDWeightsCh,
	)

	// Start simulation with direct heartbeat channel (hub is used for sync mode).
	simulation := sim.NewSim(cortexSensorCh, simHeartbeatGUICh, h)
	go simulation.Run(simControlCh)

	// Instantiate and start any external data sources the caller supplied.
	// This repository defines none — a deployment wanting a different input
	// source passes its own factories into NewBackend.
	worldSources := make([]WorldSource, 0, len(sources))
	for _, factory := range sources {
		ws := factory(cortexSensorCh, h)
		ws.Start()
		worldSources = append(worldSources, ws)
	}

	metricsHandler := chaos.NewMetricsHandler(h)
	metricsHandler.Start()
	log.Println("backend: MetricsHandler started (chaos metrics at 10Hz)")

	// Start virtual position tracker (Broker) for Phase 5
	broker := sim.NewPositionTracker(h, c)
	broker.Start()

	channels := &api.BackendChannels{
		SimControlCh:       simControlCh,
		CortexControlCh:    cortexControlCh,
		CortexResetCh:      cortexResetCh,
		CortexSoftResetCh:  cortexSoftResetCh,
		MainPanelControlCh: mainPanelControlCh,
	}

	return &Backend{
		channels:       channels,
		cortex:         c,
		sim:            simulation,
		worldSources:   worldSources,
		hub:            h,
		metricsHandler: metricsHandler,
		broker:         broker,
	}
}

// ResumeWorldSources starts (or resumes) every external data source supplied
// to NewBackend. No-op if none were supplied (the sim-only, open-source case).
func (b *Backend) ResumeWorldSources() {
	for _, ws := range b.worldSources {
		ws.Start()
	}
}

// PauseWorldSources stops every external data source supplied to NewBackend.
func (b *Backend) PauseWorldSources() {
	for _, ws := range b.worldSources {
		ws.Stop()
	}
}

func (b *Backend) GetCortex() *cortex.Cortex {
	return b.cortex
}

func (b *Backend) GetChannels() *api.BackendChannels {
	return b.channels
}

func (b *Backend) GetHub() *hub.Hub {
	return b.hub
}

func (b *Backend) Start() {
	// Already started in NewBackend for now, but can be moved here if needed.
}

func (b *Backend) Stop() {
	// Implement stop logic if needed
}
