package chaos

import (
	"log"
	"runtime/debug"
	"sync"
	"time"

	"github.com/DjarinDin/TuringSNN/internal/core_logic/hub"
	"github.com/DjarinDin/TuringSNN/pkg/comms"
)

// MetricsHandler subscribes to hub topics and computes chaos metrics
// This is a standalone component that doesn't modify the cortex directly
type MetricsHandler struct {
	hub       *hub.Hub
	collector *MetricsCollector

	// Configuration
	publishInterval time.Duration

	// State
	running bool
	stopCh  chan struct{}
	mu      sync.RWMutex

	// Recent data for metric computation
	lastDensities [4]float32 // sensor, L1, L2, L3
}

// NewMetricsHandler creates a new metrics handler
func NewMetricsHandler(h *hub.Hub) *MetricsHandler {
	return &MetricsHandler{
		hub:             h,
		collector:       NewMetricsCollector(100, 1), // 100 tick buffer, compute every tick
		publishInterval: 10 * time.Millisecond,       // 100Hz updates (match main app rate)
		stopCh:          make(chan struct{}),
	}
}

// Start begins collecting metrics and publishing to hub
func (mh *MetricsHandler) Start() {
	mh.mu.Lock()
	if mh.running {
		mh.mu.Unlock()
		return
	}
	mh.running = true
	mh.mu.Unlock()

	// Subscribe to layer spikes directly (100Hz - no throttle)
	go mh.collectLayerSpikes(0, hub.TopicLayer1Spikes, "metrics-l1-spikes")
	go mh.collectLayerSpikes(1, hub.TopicLayer2Spikes, "metrics-l2-spikes")
	go mh.collectLayerSpikes(2, hub.TopicLayer3Spikes, "metrics-l3-spikes")

	// Subscribe to sensor spikes directly for sensor layer (100Hz - no throttle)
	go mh.collectSensorSpikes()

	// Subscribe to focus potential topic for focus neuron metrics
	go mh.collectFocusPotential()

	// Periodic publishing of computed metrics
	go mh.publishMetrics()

	log.Println("MetricsHandler: Started (all layers at 100Hz)")
}

// Stop halts metrics collection
func (mh *MetricsHandler) Stop() {
	mh.mu.Lock()
	defer mh.mu.Unlock()

	if !mh.running {
		return
	}

	close(mh.stopCh)
	mh.running = false
	log.Println("MetricsHandler: Stopped")
}

// collectLayerSpikes subscribes to a layer's spike topic at 100Hz (no throttle)
// Computes density from the raw spike array each tick
func (mh *MetricsHandler) collectLayerSpikes(layerIndex int, topic hub.Topic, subscriberName string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC in MetricsHandler.collectLayerSpikes[%d]: %v\nStack: %s", layerIndex, r, debug.Stack())
		}
	}()

	spikesCh := mh.hub.Subscribe(subscriberName, topic)

	for {
		select {
		case <-mh.stopCh:
			return
		case msg := <-spikesCh:
			if spikes, ok := msg.([]bool); ok && len(spikes) > 0 {
				// Calculate density from spike array
				count := 0
				for _, s := range spikes {
					if s {
						count++
					}
				}
				density := float64(count) / float64(len(spikes))
				mh.collector.RecordTick(layerIndex, density, 0, 0, 0)
			}
		}
	}
}

// collectSensorSpikes subscribes directly to sensor spikes (100Hz, no throttle)
// Tracks the PATTERN of sensor indices, not just firing density.
// This allows chaos metrics to distinguish between predictable (ramp) and random (noise) signals.
func (mh *MetricsHandler) collectSensorSpikes() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC in MetricsHandler.collectSensorSpikes: %v\nStack: %s", r, debug.Stack())
		}
	}()

	spikesCh := mh.hub.Subscribe("metrics-sensor-spikes", hub.TopicSensorSpikes)

	for {
		select {
		case <-mh.stopCh:
			return
		case msg := <-spikesCh:
			if spikes, ok := msg.([]bool); ok && len(spikes) > 0 {
				// Find which sensor(s) fired and compute a representative value
				// For chaos analysis, we want the PATTERN of sensor indices over time
				var sensorIndex float64 = -1
				sensorCount := 0
				for idx, s := range spikes {
					if s {
						if sensorCount == 0 {
							sensorIndex = float64(idx)
						} else {
							// Multiple sensors fired - use center of mass
							sensorIndex = (sensorIndex*float64(sensorCount) + float64(idx)) / float64(sensorCount+1)
						}
						sensorCount++
					}
				}

				if sensorCount > 0 {
					// Normalize to 0-1 range for consistent chaos analysis
					normalizedIndex := sensorIndex / float64(len(spikes)-1)
					mh.collector.RecordTick(3, normalizedIndex, 0, 0, 0) // Sensor = layer index 3
				}
			}
		}
	}
}

// publishMetrics periodically sends computed metrics to the hub
func (mh *MetricsHandler) publishMetrics() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC in MetricsHandler.publishMetrics: %v\nStack: %s", r, debug.Stack())
		}
	}()

	ticker := time.NewTicker(mh.publishInterval)
	defer ticker.Stop()

	for {
		select {
		case <-mh.stopCh:
			return
		case <-ticker.C:
			metrics := mh.collector.GetMetrics()
			if metrics != nil {
				// Convert to a format suitable for JSON serialization
				msg := MetricsMessage{
					Lyapunov:       [3]float64{metrics.Lyapunov[0], metrics.Lyapunov[1], metrics.Lyapunov[2]},
					Hurst:          [3]float64{metrics.Hurst[0], metrics.Hurst[1], metrics.Hurst[2]},
					SensorLyapunov: metrics.SensorLyapunov,
					SensorHurst:    metrics.SensorHurst,
					FocusLyapunov:  metrics.FocusLyapunov,
					FocusHurst:     metrics.FocusHurst,
					AvgLyapunov:    metrics.AvgLyapunov,
					AvgHurst:       metrics.AvgHurst,
					Entropy:        [3]float64{metrics.Entropy[0], metrics.Entropy[1], metrics.Entropy[2]},
					ThresholdDev:   [3]float64{metrics.ThresholdDev[0], metrics.ThresholdDev[1], metrics.ThresholdDev[2]},
				}
				mh.hub.Publish(hub.TopicMetrics, msg)
			}
		}
	}
}

// GetMetrics returns the current metrics (for direct access without hub)
func (mh *MetricsHandler) GetMetrics() *Metrics {
	return mh.collector.GetMetrics()
}

// collectFocusPotential subscribes to focus neuron potential updates
func (mh *MetricsHandler) collectFocusPotential() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC in MetricsHandler.collectFocusPotential: %v\nStack: %s", r, debug.Stack())
		}
	}()

	focusCh := mh.hub.Subscribe("metrics-focus", hub.TopicFocusPotentialHistory)

	for {
		select {
		case <-mh.stopCh:
			return
		case msg := <-focusCh:
			// Focus potential message is comms.BoolMsg with Pot and Spikes fields
			if potMsg, ok := msg.(comms.BoolMsg); ok {
				// Normalize potential to 0-1 range for chaos analysis
				// Potential typically ranges from 0 to ~1e18 (threshold)
				normPot := potMsg.Pot / 1e18
				if normPot > 1 {
					normPot = 1
				}
				mh.collector.RecordTick(4, normPot, 0, 0, 0) // Focus = layer index 4
			}
		}
	}
}

// MetricsMessage is the wire format sent to JavaScript
type MetricsMessage struct {
	Lyapunov       [3]float64 `json:"lyapunov"`
	Hurst          [3]float64 `json:"hurst"`
	SensorLyapunov float64    `json:"sensorLyapunov"`
	SensorHurst    float64    `json:"sensorHurst"`
	FocusLyapunov  float64    `json:"focusLyapunov"`
	FocusHurst     float64    `json:"focusHurst"`
	AvgLyapunov    float64    `json:"avgLyapunov"`
	AvgHurst       float64    `json:"avgHurst"`
	Entropy        [3]float64 `json:"entropy"`
	ThresholdDev   [3]float64 `json:"thresholdDev"`
}

// Dummy import to satisfy compiler when comms package is imported but not used
var _ comms.ControlMsg
