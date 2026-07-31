//go:build js && wasm
// +build js,wasm

package bridge

import (
	"encoding/json"
	"github.com/DjarinDin/TuringSNN/internal/chaos"
	"github.com/DjarinDin/TuringSNN/internal/conf"
	"github.com/DjarinDin/TuringSNN/internal/core_logic"
	"github.com/DjarinDin/TuringSNN/internal/core_logic/hub"
	"github.com/DjarinDin/TuringSNN/internal/core_logic/sim"
	"github.com/DjarinDin/TuringSNN/pkg/comms"
	"log"
	"runtime"
	"runtime/debug"
	"sync"
	"syscall/js"
	"time"
)

// WASMBridge manages the cortex and simulation, exposing them to JavaScript
type WASMBridge struct {
	backend      *core_logic.Backend
	worldSources []core_logic.WorldSourceFactory
	callbacks    map[string]js.Func

	// Update callback
	onUpdateCallback js.Value
	isRunning        bool
}

// NewWASMBridge creates a bridge between Go cortex and JavaScript.
// sources is optional: this build passes none (sim-only data); a deployment
// wanting a different input source passes its own factories.
func NewWASMBridge(sources ...core_logic.WorldSourceFactory) js.Value {
	bridge := &WASMBridge{
		callbacks:    make(map[string]js.Func),
		worldSources: sources,
	}

	// Create JavaScript object with methods
	obj := js.Global().Get("Object").New()

	// Expose methods to JavaScript
	startFunc := js.FuncOf(bridge.start)
	resetFunc := js.FuncOf(bridge.reset)
	softResetFunc := js.FuncOf(bridge.softReset)
	stopFunc := js.FuncOf(bridge.stop)
	dopamineFunc := js.FuncOf(bridge.injectDopamine)
	onUpdateFunc := js.FuncOf(bridge.onUpdate)
	setFocusFunc := js.FuncOf(bridge.setFocus)
	retributionFunc := js.FuncOf(bridge.applyRetribution)

	// Store funcs for cleanup
	bridge.callbacks["start"] = startFunc
	bridge.callbacks["reset"] = resetFunc
	bridge.callbacks["softReset"] = softResetFunc
	bridge.callbacks["stop"] = stopFunc
	bridge.callbacks["dopamine"] = dopamineFunc
	bridge.callbacks["onUpdate"] = onUpdateFunc
	bridge.callbacks["setThreshold"] = js.FuncOf(bridge.setThreshold)
	bridge.callbacks["setFocus"] = setFocusFunc

	obj.Set("start", startFunc)
	obj.Set("reset", resetFunc)
	obj.Set("softReset", softResetFunc)
	obj.Set("stop", stopFunc)
	obj.Set("injectDopamine", dopamineFunc)
	obj.Set("onUpdate", onUpdateFunc)
	obj.Set("setThreshold", js.FuncOf(bridge.setThreshold))
	obj.Set("setFocus", setFocusFunc)
	obj.Set("setSimSignal", js.FuncOf(bridge.setSimSignal))
	js.Global().Set("setSpontaneousFire", js.FuncOf(bridge.setSpontaneousFire))
	js.Global().Set("setAdaptability", js.FuncOf(bridge.setAdaptability))
	js.Global().Set("resetAccumulator", js.FuncOf(bridge.resetAccumulator))
	obj.Set("setSimState", js.FuncOf(bridge.setSimState))
	obj.Set("setTickRate", js.FuncOf(bridge.setTickRate))
	obj.Set("setRunState", js.FuncOf(bridge.setRunState))
	obj.Set("step", js.FuncOf(bridge.step))
	obj.Set("setCortexControl", js.FuncOf(bridge.setCortexControl))
	obj.Set("setRealDataMode", js.FuncOf(bridge.setRealDataMode))
	obj.Set("applyRetribution", retributionFunc)

	// Expose configuration constants
	obj.Set("config", bridge.getConfig())

	return obj
}

// getConfig returns configuration constants as a JS object
func (b *WASMBridge) getConfig() js.Value {
	config := js.Global().Get("Object").New()
	config.Set("numSensors", conf.NumSensors)
	config.Set("numNeurons", conf.NumNeurons)
	config.Set("numOutputNeurons", conf.NumOutputNeurons)
	config.Set("maxTraceAndWeight", conf.MaxTraceAndWeight)
	config.Set("threshold", conf.BaseThreshold)
	return config
}

// start initializes and starts the cortex simulation
func (b *WASMBridge) start(this js.Value, args []js.Value) interface{} {
	if b.isRunning {
		return nil
	}

	// Initialize Backend (handles cortex, sim, hub, and buffered channels)
	b.backend = core_logic.NewBackend(b.worldSources...)
	log.Println("bridge: Backend initialized")
	b.backend.Start()
	log.Println("bridge: Backend started")

	// Get channels for control
	channels := b.backend.GetChannels()

	// WASM FIX: Set moderate tick rate BEFORE cortex starts (10ms = desktop speed)
	// 10ms = 100 ticks/sec (same as desktop), combined with 60Hz GUI updates = smooth demo
	// Note: backend.Start() might have already started cortex, but sending this control msg is fine.
	// Actually, NewBackend() starts the cortex/sim goroutines.
	// WASM FIX: Set tick rate to 10ms (100Hz) to match Fyne desktop version
	// This ensures physics and scrolling speed are identical (100px/sec)
	channels.CortexControlCh <- comms.ControlMsg{Type: comms.CortexControlTickRate, IntValue: 10}

	// Start message collector that calls JavaScript callback
	go b.collectAndSendUpdates(b.backend.GetHub())

	b.isRunning = true

	return nil
}

// collectAndSendUpdates subscribes to hub topics and uses time-based batching
// to prevent overwhelming the JavaScript event loop
func (b *WASMBridge) collectAndSendUpdates(h *hub.Hub) {

	// Batching for WASM updates
	// Use a SLICE to preserve order and allow multiple messages of the same topic per frame
	updateBatch := make([]map[string]interface{}, 0)
	var batchMutex sync.Mutex

	// Helper to store updates safely
	storeFunc := func(key string, val interface{}) {
		batchMutex.Lock()
		defer batchMutex.Unlock()
		// val is already a map with "topic" set by collectTopicToBatch
		if v, ok := val.(map[string]interface{}); ok {
			updateBatch = append(updateBatch, v)
		}
	}

	// Time-based batch flush for smooth WASM GUI updates (~60Hz = every 16ms)
	// This is independent of cortex heartbeat - purely for GUI responsiveness
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("PANIC in WASMBridge update ticker: %v\nStack: %s", r, debug.Stack())
			}
		}()
		ticker := time.NewTicker(time.Millisecond * 16) // ~60 updates/sec
		defer ticker.Stop()

		for range ticker.C {
			batchMutex.Lock()

			// Copy batch and clear original
			batchCopy := make([]map[string]interface{}, len(updateBatch))
			copy(batchCopy, updateBatch)
			updateBatch = make([]map[string]interface{}, 0)
			batchMutex.Unlock()

			// Send single batched update to JavaScript (one boundary crossing per frame)
			if len(batchCopy) > 0 {
				// Convert to []interface{} for syscall/js compatibility
				// js.ValueOf panics on []map[string]interface{}
				interfaceSlice := make([]interface{}, len(batchCopy))
				for i, v := range batchCopy {
					interfaceSlice[i] = v
				}
				b.sendUpdateToJS("batch", interfaceSlice)
			}
		}
	}()

	// Memory Stats Ticker (1s interval)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("PANIC in Memory Stats Ticker: %v\nStack: %s", r, debug.Stack())
			}
		}()
		ticker := time.NewTicker(time.Second * 1)
		defer ticker.Stop()

		for range ticker.C {
			// Read memory stats
			var m runtime.MemStats
			runtime.ReadMemStats(&m)

			// Send update with topic so JS can route it correctly
			stats := map[string]interface{}{
				"topic": "memoryStats",
				"value": map[string]interface{}{
					"syst":  m.Sys / 1024 / 1024,
					"heap":  m.HeapAlloc / 1024 / 1024,
					"stack": m.StackInuse / 1024,
				},
			}
			b.sendUpdateToJS("memoryStats", stats)
		}
	}()

	// Monitor heartbeat to keep subscription alive (prevents channel blocking)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("PANIC in WASMBridge heartbeat monitor: %v\nStack: %s", r, debug.Stack())
			}
		}()
		heartbeatCh := h.Subscribe("wasm-heartbeat-monitor", hub.TopicCortexHeartbeat)
		for range heartbeatCh {
			// Silently consume heartbeats
		}
	}()

	// Subscribe to all topics - they update the batch instead of sending directly
	go b.collectTopicToBatch(h.Subscribe("wasm-sim-heartbeat", hub.TopicSimHeartbeat), "simHeartbeat", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-cortex-heartbeat", hub.TopicCortexHeartbeat), "cortexHeartbeat", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-output-heartbeat", hub.TopicOutputA), "outputHeartbeat", storeFunc) // Use OutputA as heartbeat
	go b.collectTopicToBatch(h.Subscribe("wasm-sensors", hub.TopicSensorSpikes), "sensorSpikes", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-raw-sensors", hub.TopicRawSensorSpikes), "rawSensorSpikes", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-tracker-status", hub.TopicDecisionTrackerStatus), "brokerStatus", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-tracker-ledger", hub.TopicDecisionLedger), "brokerLedger", storeFunc)

	go b.collectTopicToBatch(h.Subscribe("wasm-stats-cum", hub.TopicCumulativeStats), "cumulativeStats", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-stats-hb", hub.TopicPerHeartbeatStats), "perHeartbeatStats", storeFunc)

	go b.collectTopicToBatch(h.Subscribe("wasm-l1-pots", hub.TopicLayer1Potentials), "layer1Potentials", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-l1-spikes", hub.TopicLayer1Spikes), "layer1Spikes", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-l1-fb-weights", hub.TopicLayer1FeedbackWeights), "layer1FeedbackWeights", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-l1-fb-traces", hub.TopicLayer1FeedbackTraces), "layer1FeedbackTraces", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-l1-feedback-spikes", hub.Topic("layer1.feedback.spikes")), "layer1FeedbackSpikes", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-l1-resonance-spikes", hub.Topic("layer1.resonance.spikes")), "layer1ResonanceSpikes", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-l1-zeitgeist-spikes", hub.Topic("layer1.zeitgeist.spikes")), "layer1ZeitgeistSpikes", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-l1-out-traces", hub.TopicLayer1OutputTraces), "layer1OutputTraces", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-l1-in-traces", hub.TopicLayer1InputTraces), "layer1InputTraces", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-l1-in-weights", hub.TopicLayer1InputWeights), "layer1InputWeights", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-l1-resonance-traces", hub.Topic("layer1.resonance.traces")), "layer1ResonanceTraces", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-l1-resonance-weights", hub.Topic("layer1.resonance.weights")), "layer1ResonanceWeights", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-l1-zeitgeist-traces", hub.Topic("layer1.zeitgeist.traces")), "layer1ZeitgeistTraces", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-l1-zeitgeist-weights", hub.Topic("layer1.zeitgeist.weights")), "layer1ZeitgeistWeights", storeFunc)

	go b.collectTopicToBatch(h.Subscribe("wasm-l2-pots", hub.TopicLayer2Potentials), "layer2Potentials", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-l2-spikes", hub.TopicLayer2Spikes), "layer2Spikes", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-l2-in-weights", hub.TopicLayer2InputWeights), "layer2InputWeights", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-l2-resonance-traces", hub.Topic("layer2.resonance.traces")), "layer2ResonanceTraces", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-l2-resonance-weights", hub.Topic("layer2.resonance.weights")), "layer2ResonanceWeights", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-l2-zeitgeist-traces", hub.Topic("layer2.zeitgeist.traces")), "layer2ZeitgeistTraces", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-l2-zeitgeist-weights", hub.Topic("layer2.zeitgeist.weights")), "layer2ZeitgeistWeights", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-l2-fb-weights", hub.TopicLayer2FeedbackWeights), "layer2FeedbackWeights", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-l2-fb-traces", hub.TopicLayer2FeedbackTraces), "layer2FeedbackTraces", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-l2-feedback-spikes", hub.Topic("layer2.feedback.spikes")), "layer2FeedbackSpikes", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-l2-resonance-spikes", hub.Topic("layer2.resonance.spikes")), "layer2ResonanceSpikes", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-l2-zeitgeist-spikes", hub.Topic("layer2.zeitgeist.spikes")), "layer2ZeitgeistSpikes", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-l2-out-traces", hub.TopicLayer2OutputTraces), "layer2OutputTraces", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-l2-in-traces", hub.TopicLayer2InputTraces), "layer2InputTraces", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-l2-input-spikes", hub.TopicLayer2InputSpikes), "layer2InputSpikes", storeFunc)

	go b.collectTopicToBatch(h.Subscribe("wasm-l3-pots", hub.TopicLayer3Potentials), "layer3Potentials", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-l3-spikes", hub.TopicLayer3Spikes), "layer3Spikes", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-l3-in-weights", hub.TopicLayer3InputWeights), "layer3InputWeights", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-l3-resonance-traces", hub.Topic("layer3.resonance.traces")), "layer3ResonanceTraces", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-l3-resonance-weights", hub.Topic("layer3.resonance.weights")), "layer3ResonanceWeights", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-l3-zeitgeist-traces", hub.Topic("layer3.zeitgeist.traces")), "layer3ZeitgeistTraces", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-l3-zeitgeist-weights", hub.Topic("layer3.zeitgeist.weights")), "layer3ZeitgeistWeights", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-l3-fb-weights", hub.TopicLayer3FeedbackWeights), "layer3FeedbackWeights", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-l3-fb-traces", hub.TopicLayer3FeedbackTraces), "layer3FeedbackTraces", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-l3-feedback-spikes", hub.TopicLayer3FeedbackSpikes), "layer3FeedbackSpikes", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-l3-resonance-spikes", hub.Topic("layer3.resonance.spikes")), "layer3ResonanceSpikes", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-l3-zeitgeist-spikes", hub.Topic("layer3.zeitgeist.spikes")), "layer3ZeitgeistSpikes", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-l3-out-traces", hub.TopicLayer3OutputTraces), "layer3OutputTraces", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-l3-in-traces", hub.TopicLayer3InputTraces), "layer3InputTraces", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-l3-input-spikes", hub.TopicLayer3InputSpikes), "layer3InputSpikes", storeFunc)

	go b.collectTopicToBatch(h.Subscribe("wasm-output-a", hub.TopicOutputAHistory), "outputA", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-output-b", hub.TopicOutputBHistory), "outputB", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-output-a-weights", hub.TopicOutputAWeights), "outputAWeights", storeFunc)
	go b.collectTopicToBatch(h.Subscribe("wasm-output-b-weights", hub.TopicOutputBWeights), "outputBWeights", storeFunc)

	go b.collectTopicToBatch(h.Subscribe("wasm-densities", hub.TopicDensities), "densities", storeFunc)

	go b.collectTopicToBatch(h.Subscribe("wasm-metrics", hub.TopicMetrics), "chaosMetrics", storeFunc)

	go b.collectTopicToBatch(h.Subscribe("wasm-focus", hub.TopicFocusPotentialHistory), "focusPotential", storeFunc)
}

// collectTopicToBatch stores messages in the batch map instead of sending directly
func (b *WASMBridge) collectTopicToBatch(ch <-chan interface{}, jsKey string, storeFunc func(string, interface{})) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC in collectTopicToBatch(%s): %v\nStack: %s", jsKey, r, debug.Stack())
		}
	}()
	for msg := range ch {
		// Type the message and add to batch
		var update map[string]interface{}
		switch m := msg.(type) {
		case int:
			update = map[string]interface{}{"value": m}
		case []bool:
			update = map[string]interface{}{"value": boolSliceToIndices(m)}
		case []float64:
			update = map[string]interface{}{"value": m}
		case []float32: // Added for densities
			update = map[string]interface{}{"value": m}
		case []int:
			update = map[string]interface{}{"value": m}
		case comms.PotentialsAndSpikesMsg:
			update = map[string]interface{}{
				"potentials": m.Pots,
				"spikes":     boolSliceToIndices(m.Spikes),
			}
		case comms.BoolMsg:
			update = map[string]interface{}{
				"potential": m.Pot,
				"spike":     m.Spikes,
			}
		case comms.StringMsg:
			update = map[string]interface{}{
				"value": m.Value,
				"type":  m.Type,
			}
		case string:
			update = map[string]interface{}{"value": m}
		case bool:
			update = map[string]interface{}{"value": m}
		case runtime.MemStats: // New case for memory stats
			update = map[string]interface{}{
				"syst":  m.Sys / 1024 / 1024,
				"heap":  m.HeapAlloc / 1024 / 1024,
				"stack": m.StackInuse / 1024,
			}
		case chaos.MetricsMessage: // Chaos metrics
			update = map[string]interface{}{
				"lyapunov":       m.Lyapunov,
				"hurst":          m.Hurst,
				"sensorLyapunov": m.SensorLyapunov,
				"sensorHurst":    m.SensorHurst,
				"focusLyapunov":  m.FocusLyapunov,
				"focusHurst":     m.FocusHurst,
				"avgLyapunov":    m.AvgLyapunov,
				"avgHurst":       m.AvgHurst,
				"entropy":        m.Entropy,
				"thresholdDev":   m.ThresholdDev,
			}
		case sim.LedgerState: // Full investment state
			update = map[string]interface{}{
				"openLots":   m.OpenLots,
				"history":    m.History,
				"totalROI":   m.TotalReturn,
				"currentNet": m.CurrentNet,
			}
		default:
			log.Printf("bridge: Unknown message type for topic %s: %T", jsKey, msg)
			continue
		}

		update["topic"] = jsKey
		storeFunc(jsKey, update)
	}
}

// sendUpdateToJS sends a single update to JavaScript
func (b *WASMBridge) sendUpdateToJS(topic string, msg interface{}) {
	if b.onUpdateCallback.IsUndefined() || b.onUpdateCallback.IsNull() {
		return
	}

	// Convert to JSON (Bytes)
	jsonData, err := json.Marshal(msg)
	if err != nil {
		log.Printf("bridge: Error marshaling update for %s: %v", topic, err)
		return
	}

	// Create JS Uint8Array of the correct size
	jsArray := js.Global().Get("Uint8Array").New(len(jsonData))

	// Copy bytes to JS (avoids passing large string via syscall/json.stringVal)
	js.CopyBytesToJS(jsArray, jsonData)

	// Parse JSON in JavaScript using helper (window.parseTuringJSON)
	// Fallback to JSON.parse if helper missing (dev safety)
	var jsData js.Value
	parseFunc := js.Global().Get("parseTuringJSON")
	if !parseFunc.IsUndefined() {
		jsData = parseFunc.Invoke(jsArray)
	} else {
		// Fallback (might crash if string too large, but better than nothing)
		log.Println("bridge: parseTuringJSON missing, falling back to string passing")
		jsData = js.Global().Get("JSON").Call("parse", string(jsonData))
	}

	b.onUpdateCallback.Invoke(jsData)
}

// boolSliceToIndices converts []bool to array of spike indices (more compact)
func boolSliceToIndices(spikes []bool) []int {
	indices := make([]int, 0, len(spikes)/8) // Estimate ~12.5% spike rate
	for i, spike := range spikes {
		if spike {
			indices = append(indices, i)
		}
	}
	return indices
}

// stop pauses the cortex simulation
func (b *WASMBridge) stop(this js.Value, args []js.Value) interface{} {
	if !b.isRunning {
		return nil
	}

	log.Println("bridge: Stopping cortex...")
	b.backend.GetChannels().CortexControlCh <- comms.ControlMsg{Type: comms.CortexControlTickRate, BoolValue: false}
	b.isRunning = false

	return nil
}

// reset resets the cortex to initial state (preserves sim signal states)
func (b *WASMBridge) reset(this js.Value, args []js.Value) interface{} {
	log.Println("bridge: Hard Resetting cortex (preserving sim state)...")
	b.backend.GetChannels().CortexResetCh <- true
	// NOTE: Do NOT reset sim - preserve which signals are running
	// Ensure cortex is running after reset (unpause if paused)
	b.backend.GetChannels().CortexControlCh <- comms.ControlMsg{Type: comms.CortexControlPause, BoolValue: false}
	return nil
}

// softReset re-randomizes weights but preserves parameters
func (b *WASMBridge) softReset(this js.Value, args []js.Value) interface{} {
	log.Println("bridge: Soft Resetting cortex...")
	b.backend.GetChannels().CortexSoftResetCh <- true
	// Ensure cortex is running after reset (unpause if paused)
	b.backend.GetChannels().CortexControlCh <- comms.ControlMsg{Type: comms.CortexControlPause, BoolValue: false}
	return nil
}

// injectDopamine triggers dopamine injection
func (b *WASMBridge) injectDopamine(this js.Value, args []js.Value) interface{} {
	log.Println("bridge: Injecting dopamine...")
	b.backend.GetChannels().MainPanelControlCh <- comms.ControlMsg{Type: 100, BoolValue: true}
	return nil
}

// setThreshold updates the threshold parameter (No-op for now)
func (b *WASMBridge) setThreshold(this js.Value, args []js.Value) interface{} {
	log.Println("bridge: setThreshold called (deprecated)")
	return nil
}

// setFocus updates the focus neuron and layer
func (b *WASMBridge) setFocus(this js.Value, args []js.Value) interface{} {
	if len(args) < 2 {
		log.Println("bridge: setFocus requires layer and neuronID")
		return nil
	}

	layer := args[0].Int()
	neuronID := args[1].Int()

	// log.Printf("bridge: Setting focus to Layer %d, Neuron %d", layer, neuronID)

	// Send control message to Cortex
	// Type 3 = Set Focus (see internal/core_logic/cortex/cortex.go handleControlMessages)
	msg := comms.ControlMsg{
		Type:     comms.CortexControlFocus,
		IntValue: neuronID,
		Layer:    layer,
	}

	// Send to CortexControlCh (which Cortex listens to)
	// Note: In the desktop app, this is sent to MANY channels.
	// Here, we primarily need the Cortex to know so it sends the right weights/traces.
	// The JS side handles the visual highlighting.
	b.backend.GetChannels().CortexControlCh <- msg

	return nil
}

// onUpdate registers a callback for cortex updates
func (b *WASMBridge) onUpdate(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return nil
	}

	callback := args[0]
	if callback.Type() != js.TypeFunction {
		return nil
	}

	b.onUpdateCallback = callback

	return nil
}

// setSimSignal updates the signal volume
func (b *WASMBridge) setSimSignal(this js.Value, args []js.Value) interface{} {
	if len(args) < 2 {
		log.Println("bridge: setSimSignal requires index and value")
		return nil
	}

	index := args[0].Int()
	value := args[1].Int()

	// Offset by 8 for volume control (see internal/gui/panels/simControl.go)
	msg := comms.IntMsg{
		Type:  comms.SimControlNoiseVolume + index,
		Value: value,
	}

	b.backend.GetChannels().SimControlCh <- msg
	return nil
}

// setAdaptability function to WASMBridge
func (b *WASMBridge) setAdaptability(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return nil
	}

	val := args[0].Int()
	adaptability := float64(val) / 100.0
	// Clamp
	if adaptability < 0 {
		adaptability = 0
	}
	if adaptability > 1 {
		adaptability = 1
	}

	b.backend.GetCortex().Adaptability = adaptability
	log.Printf("Adaptability set to: %f", adaptability)
	b.backend.GetCortex().Adaptability = adaptability
	log.Printf("Adaptability set to: %f", adaptability)
	return nil
}

// setSpontaneousFire updates the spontaneous firing rate
func (b *WASMBridge) setSpontaneousFire(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return nil
	}
	val := args[0].Int()
	b.backend.GetChannels().CortexControlCh <- comms.ControlMsg{
		Type:     comms.CortexControlSpontaneousFire,
		IntValue: val,
	}
	return nil
}

// resetAccumulator clears the sensor accumulator buffer.
func (b *WASMBridge) resetAccumulator(this js.Value, args []js.Value) interface{} {
	b.backend.GetChannels().CortexControlCh <- comms.ControlMsg{
		Type: comms.CortexControlResetAccumulator,
	}
	return nil
}

// setSimState updates the signal on/off state
func (b *WASMBridge) setSimState(this js.Value, args []js.Value) interface{} {
	if len(args) < 2 {
		log.Println("bridge: setSimState requires index and state")
		return nil
	}

	index := args[0].Int()
	state := args[1].Int() // 1 for on, 0 for off

	// Type 0-7 for signal toggle (see internal/gui/panels/simControl.go)
	msg := comms.IntMsg{
		Type:  index,
		Value: state,
	}

	b.backend.GetChannels().SimControlCh <- msg
	return nil
}

// setTickRate updates the simulation tick rate (RT vs Slow)
func (b *WASMBridge) setTickRate(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		log.Println("bridge: setTickRate requires boolean state")
		return nil
	}

	isRealTime := args[0].Bool()
	var period int

	if isRealTime {
		period = conf.RealTimeTickPeriod
	} else {
		period = conf.SlowTickPeriod
	}

	log.Printf("bridge: Setting tick rate to %d ms (RT=%v)", period, isRealTime)

	// Send to Cortex (ControlMsg Type 0)
	b.backend.GetChannels().CortexControlCh <- comms.ControlMsg{
		Type:     comms.CortexControlTickRate,
		IntValue: period,
	}

	// Send to Sim (IntMsg Type 90)
	b.backend.GetChannels().SimControlCh <- comms.IntMsg{
		Type:  comms.SimControlTickRate,
		Value: period,
	}

	return nil
}

// setCortexControl sends generic control messages to the cortex
// Type 1: Dopamine (0-100)
// Type 5: Spontaneous (0-100)
// Type 6: Reset accumulator
func (b *WASMBridge) setCortexControl(this js.Value, args []js.Value) interface{} {
	if len(args) < 2 {
		log.Println("bridge: setCortexControl requires type and value")
		return nil
	}

	msgType := args[0].Int()
	value := args[1].Int()

	// Send to Cortex (ControlMsg)
	b.backend.GetChannels().CortexControlCh <- comms.ControlMsg{
		Type:     msgType,
		IntValue: value,
	}

	return nil
}

// setRunState pauses or resumes the cortex
func (b *WASMBridge) setRunState(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		log.Println("bridge: setRunState requires boolean state")
		return nil
	}

	shouldRun := args[0].Bool()
	log.Printf("bridge: Setting run state to %v", shouldRun)

	// Send to Cortex (ControlMsg Type 2)
	// Cortex Type 2: true = Pause, false = Run
	b.backend.GetChannels().CortexControlCh <- comms.ControlMsg{
		Type:      comms.CortexControlPause,
		BoolValue: !shouldRun, // Invert: Run(true) -> false(Run), Pause(false) -> true(Pause)
	}

	// Send to Sim (IntMsg Type 80)
	// Sim Type 80: 1 = Run, 0 = Pause
	simVal := 0
	if shouldRun {
		simVal = 1
	}
	b.backend.GetChannels().SimControlCh <- comms.IntMsg{
		Type:  comms.SimControlSimRun,
		Value: simVal,
	}

	return nil
}

// step advances the cortex by one tick
func (b *WASMBridge) step(this js.Value, args []js.Value) interface{} {
	log.Println("bridge: Stepping...")

	// Send to Cortex (ControlMsg Type 4)
	b.backend.GetChannels().CortexControlCh <- comms.ControlMsg{
		Type: comms.CortexControlStep,
	}
	// Also step the simulator
	b.backend.GetChannels().SimControlCh <- comms.IntMsg{
		Type: comms.SimControlStep,
	}

	return nil
}

// setRealDataMode toggles registered WorldSources on or off, pausing the
// built-in simulation's silence-internal-signals behavior in tandem.
func (b *WASMBridge) setRealDataMode(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		log.Println("bridge: setRealDataMode requires boolean state")
		return nil
	}

	enabled := args[0].Bool()
	log.Printf("bridge: Setting RealDataMode to %v", enabled)

	// 1. Tell Sim to silence internal signals
	val := 0
	if enabled {
		val = 1
	}
	b.backend.GetChannels().SimControlCh <- comms.IntMsg{
		Type:  comms.SimControlRealDataMode,
		Value: val,
	}

	// 2. Start/stop whatever external data sources are registered (none in
	//    the open-source build; a private deployment's live feed otherwise).
	if enabled {
		b.backend.ResumeWorldSources()
	} else {
		b.backend.PauseWorldSources()
	}

	return nil
}

// applyRetribution triggers the global dopamine reinforcement logic
func (b *WASMBridge) applyRetribution(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		log.Println("bridge: applyRetribution requires profit value")
		return nil
	}

	profit := args[0].Float()
	log.Printf("bridge: Applying retribution pulse (profit: %f)", profit)

	// Trigger the reinforcement Capture phase in Cortex
	b.backend.GetCortex().ApplyRetribution(profit)

	return nil
}
