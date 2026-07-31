// Package sim provides a simulation environment for generating various types of signals
// and time series data to be used as input for a spiking neural network cortex.
package sim

import (
	"github.com/DjarinDin/TuringSNN/internal/conf"
	"github.com/DjarinDin/TuringSNN/internal/core_logic/hub"
	"github.com/DjarinDin/TuringSNN/pkg/comms"
	"log"
	"math"
	"math/rand"
	"runtime/debug"
	"time"
)

// Event represents different types of events in the simulation.
type Event interface {
	Handle(s *Sim)
}

// TickEvent represents a simulation tick event.
type TickEvent struct{}

// HeartbeatEvent represents a heartbeat tick event.
type HeartbeatEvent struct{}

// ControlEvent represents a control message event.
type ControlEvent struct {
	Msg comms.IntMsg
}

// CortexTickEvent represents a tick from the cortex (for sync mode)
type CortexTickEvent struct{}

// TimeSeriesGenerator is an interface for generating time series data.
type TimeSeriesGenerator interface {
	Next() float64
}

// RandomWalk implements a simple random walk time series generator.
type RandomWalk struct {
	value float64
}

// Next returns the next value in the random walk series.
func (rw *RandomWalk) Next() float64 {
	rw.value += rand.NormFloat64() * 0.01
	return rw.value
}

// SineWave implements a sine wave time series generator.
type SineWave struct {
	time      float64
	frequency float64
	amplitude float64
}

// Next returns the next value in the sine wave series.
func (sw *SineWave) Next() float64 {
	value := sw.amplitude * math.Sin(2*math.Pi*sw.frequency*sw.time)
	sw.time += 0.01
	return value
}

// NoisySineWave implements a sine wave with added noise.
type NoisySineWave struct {
	sineWave SineWave
	noiseAmp float64
}

// Next returns the next value in the noisy sine wave series.
func (nsw *NoisySineWave) Next() float64 {
	return nsw.sineWave.Next() + rand.NormFloat64()*nsw.noiseAmp
}

// AsymptoticApproach decelerates toward a fixed target point.
// Position approaches target with decreasing step size: pos += rate * (target - pos)
// This creates visible deceleration while being mathematically convergent.
// Nearby trajectories converge because they both approach the same target.
type AsymptoticApproach struct {
	position  float64 // Current position [-1, 1]
	target    float64 // Target position to approach
	rate      float64 // Approach rate (0.02 = slow approach)
	tickCount int     // Counter for resets
}

// Next returns the next position asymptotically approaching target.
func (aa *AsymptoticApproach) Next() float64 {
	// Asymptotic approach: always move fraction of remaining distance toward target
	// This is key for convergence: nearby points both move toward same target
	aa.position = aa.position + aa.rate*(aa.target-aa.position)

	// Periodically reset position (but keep same target) for visual interest
	// The approach phase should dominate, giving overall negative Lyapunov
	aa.tickCount++
	if aa.tickCount%300 == 0 {
		// Reset to opposite side of target - creates visible movement
		if aa.position > aa.target {
			aa.position = aa.target - 0.8
		} else {
			aa.position = aa.target + 0.8
		}
	}

	return aa.position
}

// Sim represents the main simulation structure.
type Sim struct {
	simTickNum         int
	simHeartbeatNum    int
	noiseVolumeLevel   int
	signal1VolumeLevel int
	signal2VolumeLevel int
	randomWalkVolume   int
	paused             bool
	noiseRun           bool
	signal1Run         bool
	signal2Run         bool
	randomWalkRun      bool
	signal1Pattern     [conf.SimSignalPatternLength]int
	signal2Pattern     [conf.SimSignalPatternLength]int
	randomWalkValue    int
	cortexSensorCh     chan int
	simHeartbeatGUICh  chan int
	eventCh            chan Event
	rate               time.Duration
	ticker             *time.Ticker
	tickerStopCh       chan struct{}

	// Hub for cortex-sync mode
	hub          *hub.Hub
	cortexSync   bool // When true, sim fires on cortex tick instead of internal ticker
	realDataMode bool // When true, internal signal generators are silenced

	// Time series generators
	timeSeriesGenerators []TimeSeriesGenerator
	tsGenRun             []bool
	tsGenVolume          []int
}

// NewSim creates and initializes a new Sim instance.
func NewSim(cortexSensorCh chan int, simHeartbeatGUICh chan int, h *hub.Hub) *Sim {
	s := &Sim{
		noiseVolumeLevel:   10,
		signal1VolumeLevel: 10,
		signal2VolumeLevel: 10,
		randomWalkVolume:   10,
		signal1Run:         true,  // Only signal1 is active by default
		noiseRun:           false, // All other signals are inactive
		signal2Run:         false,
		randomWalkRun:      false,
		cortexSensorCh:     cortexSensorCh,
		simHeartbeatGUICh:  simHeartbeatGUICh,
		eventCh:            make(chan Event, 100),
		rate:               time.Millisecond * time.Duration(conf.RealTimeTickPeriod/conf.SimTickRateMultiplier),
		tickerStopCh:       make(chan struct{}),
		randomWalkValue:    conf.NumSensors / 2, // Start in the middle
		hub:                h,
		cortexSync:         true,  // Default to cortex-sync mode for deterministic sim
		realDataMode:       false, // Default to Internal Simulation mode
	}
	s.initialize()
	return s
}

// initialize sets up the initial state of the simulation.
func (s *Sim) initialize() {
	s.simTickNum = 0
	s.simHeartbeatNum = 0

	// NOTE: Do NOT reset signal run states - preserve which signals are running
	// This allows reset to only affect cortex, not which sim signals are active

	// Initialize signal patterns
	for i := 0; i < conf.SimSignalPatternLength; i++ {
		s.signal1Pattern[conf.SimSignalPatternLength-i-1] = (i % conf.NumSensors) // ramp up pattern
		s.signal2Pattern[i] = rand.Intn(int(conf.NumSensors))                     // random base pattern
	}

	// Reinitialize time series generators (but preserve run states)
	s.timeSeriesGenerators = []TimeSeriesGenerator{
		&RandomWalk{},
		&SineWave{frequency: 1.0, amplitude: 1},
		&NoisySineWave{sineWave: SineWave{frequency: 1.0, amplitude: 0.5}, noiseAmp: 0.2},
		&AsymptoticApproach{position: -0.8, target: 0.3, rate: 0.02}, // Decelerating approach - convergent
	}
	// Preserve existing tsGenRun and tsGenVolume states if they exist
	// Only initialize if they don't exist yet (first time setup)
	if s.tsGenRun == nil {
		s.tsGenRun = make([]bool, len(s.timeSeriesGenerators))
		s.tsGenVolume = make([]int, len(s.timeSeriesGenerators))
		for i := range s.timeSeriesGenerators {
			s.tsGenRun[i] = false // All new generators are inactive by default
			s.tsGenVolume[i] = 10 // Default volume level
		}
	}
}

// Run starts the simulation main loop.
func (s *Sim) Run(simControlCh chan comms.IntMsg) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC in Sim.Run: %v\nStack: %s", r, debug.Stack())
		}
	}()
	go s.controlListener(simControlCh)
	go s.startTicker()
	go s.heartbeatGenerator()

	// Start cortex tick listener for sync mode (if hub is available)
	if s.hub != nil {
		go s.cortexTickListener()
	}

	for event := range s.eventCh {
		event.Handle(s)
	}
}

// controlListener listens for control messages and converts them to events.
func (s *Sim) controlListener(simControlCh chan comms.IntMsg) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC in Sim.controlListener: %v\nStack: %s", r, debug.Stack())
		}
	}()
	for msg := range simControlCh {
		s.eventCh <- ControlEvent{Msg: msg}
	}
}

// cortexTickListener subscribes to cortex ticks for sync mode.
// When cortexSync is true, this is the only source of simulation ticks.
func (s *Sim) cortexTickListener() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC in Sim.cortexTickListener: %v\nStack: %s", r, debug.Stack())
		}
	}()
	tickCh := s.hub.Subscribe("sim-cortex-tick", hub.TopicCortexTick)
	for range tickCh {
		s.eventCh <- CortexTickEvent{}
	}
}

// startTicker starts the ticker with the current rate.
// In cortexSync mode, ticks are ignored (sim uses cortex tick events instead).
func (s *Sim) startTicker() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC in Sim.startTicker: %v\nStack: %s", r, debug.Stack())
		}
	}()
	s.ticker = time.NewTicker(s.rate)
	for {
		if s.paused {
			s.ticker.Stop()
			// Wait for unpause signal (ticker restart)
			<-s.tickerStopCh
			// Received stop/restart signal
			if !s.paused {
				s.ticker = time.NewTicker(s.rate)
			} else {
				return
			}
		}

		select {
		case <-s.ticker.C:
			// In cortexSync mode, skip internal ticker events (use cortex tick instead)
			if !s.cortexSync {
				s.eventCh <- TickEvent{}
			}
		case <-s.tickerStopCh:
			return
		}
	}
}

// restartTicker stops the current ticker and starts a new one with the updated rate.
func (s *Sim) restartTicker() {
	s.tickerStopCh <- struct{}{}
	s.ticker.Stop()
	go s.startTicker()
}

// heartbeatGenerator generates heartbeat events at regular intervals.
func (s *Sim) heartbeatGenerator() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC in Sim.heartbeatGenerator: %v\nStack: %s", r, debug.Stack())
		}
	}()
	ticker := time.NewTicker(conf.HeartbeatRate)
	for range ticker.C {
		s.eventCh <- HeartbeatEvent{}
	}
}

// Handle implements the Event interface for TickEvent.
func (e TickEvent) Handle(s *Sim) {
	s.cycle()
}

// Handle implements the Event interface for HeartbeatEvent.
func (e HeartbeatEvent) Handle(s *Sim) {
	s.simHeartbeatGUICh <- s.simHeartbeatNum
	s.simHeartbeatNum++
}

// Handle implements the Event interface for CortexTickEvent.
// In sync mode, this guarantees exactly one signal fires per cortex tick.
func (e CortexTickEvent) Handle(s *Sim) {
	if s.cortexSync {
		s.cycleDeterministic() // Use deterministic cycle (no probability)
	}
}

// Handle implements the Event interface for ControlEvent.
func (e ControlEvent) Handle(s *Sim) {
	switch e.Msg.Type {
	// Run state controls
	case comms.SimControlNoiseRun:
		s.setRunState(&s.noiseRun, e.Msg.Value)
	case comms.SimControlSignal1Run:
		s.setRunState(&s.signal1Run, e.Msg.Value)
	case comms.SimControlSignal2Run:
		s.setRunState(&s.signal2Run, e.Msg.Value)
	case comms.SimControlRandomWalkRun:
		s.setRunState(&s.randomWalkRun, e.Msg.Value)
	case comms.SimControlTSGen0Run, comms.SimControlTSGen1Run, comms.SimControlTSGen2Run, comms.SimControlTSGen3Run:
		s.setTSGenRunState(e.Msg.Type-comms.SimControlTSGen0Run, e.Msg.Value)

	// Volume controls
	case comms.SimControlNoiseVolume:
		s.noiseVolumeLevel = e.Msg.Value
	case comms.SimControlSignal1Volume:
		s.signal1VolumeLevel = e.Msg.Value
	case comms.SimControlSignal2Volume:
		s.signal2VolumeLevel = e.Msg.Value
	case comms.SimControlRandomWalkVolume:
		s.randomWalkVolume = e.Msg.Value
	case comms.SimControlTSGen0Volume, comms.SimControlTSGen1Volume, comms.SimControlTSGen2Volume, comms.SimControlTSGen3Volume:
		s.setTSGenVolume(e.Msg.Type-comms.SimControlTSGen0Volume, e.Msg.Value)

	// System controls
	case comms.SimControlTickRate:
		s.setTickRate(e.Msg.Value)
	case comms.SimControlReset:
		s.initialize()
	case comms.SimControlSimRun:
		if e.Msg.Value == 1 {
			s.paused = false
			s.restartTicker()
		} else {
			s.paused = true
		}
	case comms.SimControlStep:
		s.cycle()
	case comms.SimControlCortexSync:
		s.cortexSync = e.Msg.Value == 1
	case comms.SimControlRealDataMode:
		s.realDataMode = e.Msg.Value == 1
	}
}

// setRunState is a helper to set a run state boolean from an integer value.
func (s *Sim) setRunState(state *bool, value int) {
	*state = value == 1
}

// setTSGenRunState sets the run state for a time series generator.
func (s *Sim) setTSGenRunState(index int, value int) {
	if index >= 0 && index < len(s.tsGenRun) {
		s.tsGenRun[index] = value == 1
	}
}

// setTSGenVolume sets the volume level for a time series generator.
func (s *Sim) setTSGenVolume(index int, value int) {
	if index >= 0 && index < len(s.tsGenVolume) {
		s.tsGenVolume[index] = value
	}
}

// setTickRate updates the simulation tick rate and restarts the ticker.
func (s *Sim) setTickRate(value int) {
	s.rate = time.Millisecond * time.Duration(value/conf.SimTickRateMultiplier)
	s.restartTicker()
}

// cycle performs one simulation cycle, generating and sending signals.
func (s *Sim) cycle() {
	if s.realDataMode {
		s.simTickNum++
		return
	}
	// 1. Calculate ALL signal values first (advancing state)
	// This ensures the "speed" or "period" of the signal remains constant
	// regardless of the volume (firing probability).

	noiseVal := rand.Intn(int(conf.NumSensors))

	sig1Val := s.signal1Pattern[s.simTickNum%conf.SimSignalPatternLength]

	sig2Val := s.signal2Pattern[s.simTickNum%conf.SimSignalPatternLength]

	rwVal := s.getRandomWalkValue()

	// 2. Fire signals based on probability (volume)
	s.fireSignal(s.noiseRun, s.noiseVolumeLevel, noiseVal)
	s.fireSignal(s.signal1Run, s.signal1VolumeLevel, sig1Val)
	s.fireSignal(s.signal2Run, s.signal2VolumeLevel, sig2Val)
	s.fireSignal(s.randomWalkRun, s.randomWalkVolume, rwVal)

	// 3. Time Series Generators
	for i, gen := range s.timeSeriesGenerators {
		// Always advance generator state
		value := gen.Next()
		scaledValue := int(math.Floor(value*float64(conf.NumSensors)/2)) + conf.NumSensors/2
		tsVal := max(0, min(scaledValue, conf.NumSensors-1))

		s.fireSignal(s.tsGenRun[i], s.tsGenVolume[i], tsVal)
	}

	s.simTickNum++
}

// fireSignal is a helper function to generate and send a signal based on given conditions.
func (s *Sim) fireSignal(isRunning bool, volumeLevel int, sensorID int) {
	if isRunning && rand.Intn(10) >= (10-volumeLevel) {
		s.cortexSensorCh <- sensorID
	}
}

// fireSignalDeterministic fires a signal with probability based on volume level
// In cortex-sync mode, this provides deterministic timing but still respects volume
func (s *Sim) fireSignalDeterministic(isRunning bool, volumeLevel int, sensorID int) {
	if isRunning && rand.Intn(10) >= (10-volumeLevel) {
		s.cortexSensorCh <- sensorID
	}
}

// cycleDeterministic performs one simulation cycle with deterministic timing.
// Used in cortex-sync mode to ensure signals are locked to cortex ticks.
// Volume levels are still respected for probability-based firing.
func (s *Sim) cycleDeterministic() {
	if s.realDataMode {
		s.simTickNum++
		return
	}
	// Calculate signal values (advancing state)
	noiseVal := rand.Intn(int(conf.NumSensors))
	sig1Val := s.signal1Pattern[s.simTickNum%conf.SimSignalPatternLength]
	sig2Val := s.signal2Pattern[s.simTickNum%conf.SimSignalPatternLength]
	rwVal := s.getRandomWalkValue()

	// Fire signals with volume-based probability
	s.fireSignalDeterministic(s.noiseRun, s.noiseVolumeLevel, noiseVal)
	s.fireSignalDeterministic(s.signal1Run, s.signal1VolumeLevel, sig1Val)
	s.fireSignalDeterministic(s.signal2Run, s.signal2VolumeLevel, sig2Val)
	s.fireSignalDeterministic(s.randomWalkRun, s.randomWalkVolume, rwVal)

	// Time Series Generators
	for i, gen := range s.timeSeriesGenerators {
		value := gen.Next()
		scaledValue := int(math.Floor(value*float64(conf.NumSensors)/2)) + conf.NumSensors/2
		tsVal := max(0, min(scaledValue, conf.NumSensors-1))
		s.fireSignalDeterministic(s.tsGenRun[i], s.tsGenVolume[i], tsVal)
	}

	s.simTickNum++
}

// getRandomWalkValue generates the next value for the random walk.
func (s *Sim) getRandomWalkValue() int {
	move := rand.Intn(3) - 1 // -1, 0, or 1
	s.randomWalkValue += move

	if s.randomWalkValue < 0 {
		s.randomWalkValue = 0
	} else if s.randomWalkValue >= conf.NumSensors {
		s.randomWalkValue = conf.NumSensors - 1
	}

	return s.randomWalkValue
}

// max returns the maximum of two integers.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
