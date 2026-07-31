// Package hub - Adapter to bridge existing channel-based cortex to the hub
package hub

/*
adapter.go acts as the translation layer between the Cortex's channel-based architecture
and the Hub's Pub/Sub system.

It performs 3 critical functions:

 1. Channel Draining: It launches lightweight goroutines (one per channel) to continuously
    read data from the Cortex's output channels.
 2. Protocol Translation: It takes raw channel data (int, []bool, structs) and wraps it
    into Hub messages associated with specific "Topics".
 3. Decoupling: It allows the Cortex to remain purely channel-based (standard Go concurrency pattern)
    while enabling the infinite fan-out capabilities of the Hub.

Without this adapter, the Cortex would need to know about the Hub directly, violating
separation of concerns. This adapter allows the "Fan-Out" architecture to exist non-invasively.
*/

import (
	"github.com/DjarinDin/TuringSNN/pkg/comms"
	"log"
)

// CortexAdapter bridges between the existing channel-based cortex
// and the new hub-based pub/sub system. This allows us to add the hub
// without breaking existing code.
type CortexAdapter struct {
	hub *Hub
}

// NewCortexAdapter creates a new adapter
func NewCortexAdapter(hub *Hub) *CortexAdapter {
	return &CortexAdapter{hub: hub}
}

// BridgeHeartbeat bridges cortex heartbeat channel to hub
func (a *CortexAdapter) BridgeHeartbeat(ch <-chan int, topic Topic) {
	go func() {
		for data := range ch {
			a.hub.Publish(topic, data)
		}
	}()
}

// BridgePotentialsAndSpikes bridges layer data channel to hub
func (a *CortexAdapter) BridgePotentialsAndSpikes(ch <-chan comms.PotentialsAndSpikesMsg, potentialsTopic, spikesTopic Topic) {
	go func() {
		for data := range ch {
			// Publish potentials
			a.hub.Publish(potentialsTopic, data.Pots)
			// Publish spikes
			a.hub.Publish(spikesTopic, data.Spikes)
		}
	}()
}

// BridgeWeights bridges weight channel to hub
func (a *CortexAdapter) BridgeWeights(ch <-chan []int, topic Topic) {
	go func() {
		for data := range ch {
			a.hub.Publish(topic, data)
		}
	}()
}

// BridgeTraces bridges trace channel to hub
func (a *CortexAdapter) BridgeTraces(ch <-chan []float64, topic Topic) {
	go func() {
		for data := range ch {
			a.hub.Publish(topic, data)
		}
	}()
}

// BridgeSpikes bridges spike channel to hub
func (a *CortexAdapter) BridgeSpikes(ch <-chan []bool, topic Topic) {
	go func() {
		for data := range ch {
			a.hub.Publish(topic, data)
		}
	}()
}

// BridgeRawSensorSpikes bridges raw sensor spike channel to hub
func (a *CortexAdapter) BridgeRawSensorSpikes(ch <-chan int, topic Topic) {
	go func() {
		for data := range ch {
			a.hub.Publish(topic, data)
		}
	}()
}

// BridgeRawSensorBlank bridges raw sensor blank channel to hub
func (a *CortexAdapter) BridgeRawSensorBlank(ch <-chan bool, topic Topic) {
	go func() {
		for data := range ch {
			a.hub.Publish(topic, data)
		}
	}()
}

// BridgeFocusPotentialHistory bridges focus potential history to hub
func (a *CortexAdapter) BridgeFocusPotentialHistory(ch <-chan comms.BoolMsg, topic Topic) {
	go func() {
		for data := range ch {
			a.hub.Publish(topic, data)
		}
	}()
}

// BridgeOutputHistory bridges output history to hub
func (a *CortexAdapter) BridgeOutputHistory(ch <-chan comms.BoolMsg, topic Topic) {
	go func() {
		for data := range ch {
			a.hub.Publish(topic, data)
		}
	}()
}

// BridgeStats bridges stats channels to hub
func (a *CortexAdapter) BridgeStats(ch <-chan comms.StringMsg, topic Topic) {
	go func() {
		for data := range ch {
			a.hub.Publish(topic, data)
		}
	}()
}

// BridgeOutput bridges output channel to hub
func (a *CortexAdapter) BridgeOutput(ch <-chan int, topic Topic) {
	go func() {
		for data := range ch {
			a.hub.Publish(topic, data)
		}
	}()
}

// BridgeAllCortexChannels sets up all bridges for cortex output channels
// This is a convenience function to bridge all standard cortex channels at once
func (a *CortexAdapter) BridgeAllCortexChannels(
	simHeartbeatCh <-chan int,
	cortexHeartbeatCh <-chan int,
	rawSensorSpikesCh <-chan int,
	rawSensorBlankCh <-chan bool,
	sensorSpikesCh <-chan []bool,
	l1PotsAndSpikesCh <-chan comms.PotentialsAndSpikesMsg,
	l2PotsAndSpikesCh <-chan comms.PotentialsAndSpikesMsg,
	l3PotsAndSpikesCh <-chan comms.PotentialsAndSpikesMsg,
	layer1InputWeightsCh <-chan []int,
	layer1InputTracesCh <-chan []float64,
	layer1FeedbackWeightsCh <-chan []int,
	layer1FeedbackTracesCh <-chan []float64,
	layer1OutputTracesCh <-chan []float64,
	layer2InputWeightsCh <-chan []int,
	layer2FeedbackWeightsCh <-chan []int,
	layer2FeedbackTracesCh <-chan []float64,
	layer2OutputTracesCh <-chan []float64,
	layer3InputWeightsCh <-chan []int,
	layer3FeedbackWeightsCh <-chan []int,
	layer3FeedbackTracesCh <-chan []float64,
	layer3OutputTracesCh <-chan []float64,
	focusPotentialHistoryCh <-chan comms.BoolMsg,
	outputAHistoryCh <-chan comms.BoolMsg,
	outputBHistoryCh <-chan comms.BoolMsg,
	layer1FeedbackSpikesCh <-chan []bool,
	layer2FeedbackSpikesCh <-chan []bool,
	layer3FeedbackSpikesCh <-chan []bool,
	cumulativeStatsCh <-chan comms.StringMsg,
	perHeartbeatStatsCh <-chan comms.StringMsg,
	outputCh <-chan int,
	outputAWeightsCh <-chan []int,
	outputBWeightsCh <-chan []int,
) {
	log.Println("hub: Bridging all cortex channels to hub...")

	// Heartbeats
	a.BridgeHeartbeat(simHeartbeatCh, TopicSimHeartbeat)
	a.BridgeHeartbeat(cortexHeartbeatCh, TopicCortexHeartbeat)

	// Sensors
	a.BridgeRawSensorSpikes(rawSensorSpikesCh, TopicRawSensorSpikes)
	a.BridgeRawSensorBlank(rawSensorBlankCh, TopicRawSensorBlank)
	a.BridgeSpikes(sensorSpikesCh, TopicSensorSpikes)

	// Layer potentials and spikes
	a.BridgePotentialsAndSpikes(l1PotsAndSpikesCh, TopicLayer1Potentials, TopicLayer1Spikes)
	a.BridgePotentialsAndSpikes(l2PotsAndSpikesCh, TopicLayer2Potentials, TopicLayer2Spikes)
	a.BridgePotentialsAndSpikes(l3PotsAndSpikesCh, TopicLayer3Potentials, TopicLayer3Spikes)

	// Weights
	a.BridgeWeights(layer1InputWeightsCh, TopicLayer1InputWeights)
	a.BridgeWeights(layer1FeedbackWeightsCh, TopicLayer1FeedbackWeights)
	a.BridgeWeights(layer2InputWeightsCh, TopicLayer2InputWeights)
	a.BridgeWeights(layer2FeedbackWeightsCh, TopicLayer2FeedbackWeights)
	a.BridgeWeights(layer3InputWeightsCh, TopicLayer3InputWeights)
	a.BridgeWeights(layer3FeedbackWeightsCh, TopicLayer3FeedbackWeights)
	a.BridgeWeights(outputAWeightsCh, TopicOutputAWeights)
	a.BridgeWeights(outputBWeightsCh, TopicOutputBWeights)

	// Traces
	a.BridgeTraces(layer1InputTracesCh, TopicLayer1InputTraces)
	a.BridgeTraces(layer1FeedbackTracesCh, TopicLayer1FeedbackTraces)
	a.BridgeTraces(layer1OutputTracesCh, TopicLayer1OutputTraces)
	a.BridgeTraces(layer2FeedbackTracesCh, TopicLayer2FeedbackTraces)
	a.BridgeTraces(layer2OutputTracesCh, TopicLayer2OutputTraces)
	a.BridgeTraces(layer3FeedbackTracesCh, TopicLayer3FeedbackTraces)
	a.BridgeTraces(layer3OutputTracesCh, TopicLayer3OutputTraces)

	// Feedback spikes
	a.BridgeSpikes(layer1FeedbackSpikesCh, "layer1.feedback.spikes")
	a.BridgeSpikes(layer2FeedbackSpikesCh, "layer2.feedback.spikes")
	a.BridgeSpikes(layer3FeedbackSpikesCh, "layer3.feedback.spikes")

	// Focus and output
	a.BridgeFocusPotentialHistory(focusPotentialHistoryCh, TopicFocusPotentialHistory)
	a.BridgeOutputHistory(outputAHistoryCh, TopicOutputAHistory)
	a.BridgeOutputHistory(outputBHistoryCh, TopicOutputBHistory)

	// Stats
	a.BridgeStats(cumulativeStatsCh, TopicCumulativeStats)
	a.BridgeStats(perHeartbeatStatsCh, TopicPerHeartbeatStats)

	// Output
	a.BridgeOutput(outputCh, TopicOutputA) // or create separate topic

	log.Println("hub: All channels bridged successfully")
}
