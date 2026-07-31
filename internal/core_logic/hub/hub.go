// Package hub provides a publish/subscribe message routing system
// that enables multiple clients (desktop GUI, web browser, etc.) to
// connect to a single cortex instance.
package hub

/*
hub.go is the "Central Nervous System" of the architecture.

It implements a thread-safe Publish/Subscribe (Pub/Sub) event bus that decouples
data producers (Cortex) from data consumers (GUI, Network, Loggers).

Key features:
 1. Topic-Based Routing: Messages are routed based on string topics ("cortex.heartbeat", "layer1.spikes").
 2. Non-Blocking Delivery: Publishers are never blocked by slow subscribers. If a subscriber's
    buffer is full, the message is dropped for that specific subscriber (Real-time priority).
 3. Dynamic Subscription: Clients can subscribe/unsubscribe at runtime.
 4. Fan-Out: A single message from the Cortex is efficienty delivered to N subscribers.

This component allows the physical simulation (Cortex) to run at full speed, indepedent
of the rendering frame rate of the consumer.
*/

import (
	"log"
	"sync"
	"time"
)

// Topic represents a data stream topic that clients can subscribe to
type Topic string

// Common topics that the cortex publishes
const (
	// Heartbeat topics
	TopicCortexHeartbeat Topic = "cortex.heartbeat"
	TopicSimHeartbeat    Topic = "sim.heartbeat"
	TopicDensities       Topic = "cortex.densities"
	TopicCortexTick      Topic = "cortex.tick" // 100Hz tick for sim sync mode

	// Layer data topics
	TopicLayer1Potentials Topic = "layer1.potentials"
	TopicLayer1Spikes     Topic = "layer1.spikes"
	TopicLayer2Potentials Topic = "layer2.potentials"
	TopicLayer2Spikes     Topic = "layer2.spikes"
	TopicLayer3Potentials Topic = "layer3.potentials"
	TopicLayer3Spikes     Topic = "layer3.spikes"

	// Input Spikes (from previous layer/sensors)
	TopicLayer2InputSpikes Topic = "layer2.input.spikes"
	TopicLayer3InputSpikes Topic = "layer3.input.spikes"

	// Weight topics
	TopicLayer1InputWeights    Topic = "layer1.weights.input"
	TopicLayer1FeedbackWeights Topic = "layer1.weights.feedback"
	TopicLayer2InputWeights    Topic = "layer2.weights.input"
	TopicLayer2FeedbackWeights Topic = "layer2.weights.feedback"
	TopicLayer3InputWeights    Topic = "layer3.weights.input"
	TopicLayer3FeedbackWeights Topic = "layer3.weights.feedback"

	// Trace topics
	TopicLayer1InputTraces    Topic = "layer1.traces.input"
	TopicLayer1FeedbackTraces Topic = "layer1.traces.feedback"
	TopicLayer1OutputTraces   Topic = "layer1.traces.output"
	TopicLayer2InputTraces    Topic = "layer2.traces.input"
	TopicLayer2FeedbackTraces Topic = "layer2.traces.feedback"
	TopicLayer2OutputTraces   Topic = "layer2.traces.output"
	TopicLayer3InputTraces    Topic = "layer3.traces.input"
	TopicLayer3FeedbackTraces Topic = "layer3.traces.feedback"
	TopicLayer3OutputTraces   Topic = "layer3.traces.output"
	TopicLayer3FeedbackSpikes Topic = "layer3.feedback.spikes"

	// Sensor topics
	TopicSensorSpikes    Topic = "sensor.spikes"
	TopicRawSensorSpikes Topic = "sensor.raw.spikes"
	TopicRawSensorBlank  Topic = "sensor.raw.blank"

	// Output topics
	TopicOutputA        Topic = "output.a"
	TopicOutputB        Topic = "output.b"
	TopicOutputC        Topic = "output.c"
	TopicOutputD        Topic = "output.d"
	TopicOutputAWeights Topic = "output.a.weights"
	TopicOutputBWeights Topic = "output.b.weights"
	TopicOutputCWeights Topic = "output.c.weights"
	TopicOutputDWeights Topic = "output.d.weights"
	TopicOutputAHistory Topic = "output.a.history"
	TopicOutputBHistory Topic = "output.b.history"
	TopicOutputCHistory Topic = "output.c.history"
	TopicOutputDHistory Topic = "output.d.history"

	// Focus neuron topics
	TopicFocusPotentialHistory Topic = "focus.potential.history"

	// Stats topics
	TopicCumulativeStats   Topic = "stats.cumulative"
	TopicPerHeartbeatStats Topic = "stats.per_heartbeat"

	// Chaos/health metrics
	TopicMetrics Topic = "chaos.metrics"
	// Learning dynamics (dopamine, adaptability, etc.)
	TopicLearningState Topic = "cortex.learning.state"
	// Trace display mode (0=Short, 1=Mid, 2=Long)
	TopicTraceDisplayMode Topic = "cortex.trace.display_mode"
	// Hub observability
	TopicHubDrops Topic = "hub.drops"

	// Broker status topic (external data-source connection state, if any)
	TopicDecisionTrackerStatus Topic = "tracker.status"
	TopicExternalSignal        Topic = "tracker.signal"           // Latest mid-price for PnL
	TopicOpenEngagements       Topic = "tracker.open_engagements" // Current virtual position
	TopicDecisionLedger        Topic = "tracker.ledger"           // Full investment state (lots, history)

	// Control topics (for commands from clients)
	TopicControlReset    Topic = "control.reset"
	TopicControlDopamine Topic = "control.dopamine"
	TopicControlStep     Topic = "control.step"
	TopicControlFocus    Topic = "control.focus"
)

// Subscriber represents a channel that receives messages for a topic
type Subscriber struct {
	ID      string
	Channel chan interface{}
	Topics  map[Topic]bool
	Created time.Time
}

// Hub manages pub/sub message routing between cortex and multiple clients
type Hub struct {
	// Map of topic -> list of subscriber channels
	subscribers map[Topic][]*Subscriber

	// Map of subscriber ID -> subscriber
	subscribersByID map[string]*Subscriber

	// Mutex to protect subscribers map
	mu sync.RWMutex

	// Control channels (merged from all clients)
	controlCh chan ControlMessage

	// Statistics
	messagesSent    uint64
	messagesDropped uint64
	dropsByTopic    map[Topic]uint64
	statsMu         sync.Mutex

	// Configuration
	bufferSize int
}

// ControlMessage represents a control command from a client
type ControlMessage struct {
	Type      string
	Value     interface{}
	SessionID string
	Timestamp time.Time
}

// NewHub creates a new message hub
func NewHub() *Hub {
	return &Hub{
		subscribers:     make(map[Topic][]*Subscriber),
		subscribersByID: make(map[string]*Subscriber),
		controlCh:       make(chan ControlMessage, 100),
		bufferSize:      100, // Default buffer size for subscriber channels
		dropsByTopic:    make(map[Topic]uint64),
	}
}

// SetBufferSize sets the default buffer size for new subscriber channels
func (h *Hub) SetBufferSize(size int) {
	h.bufferSize = size
}

// Subscribe creates a new subscription to one or more topics
// Returns a channel that will receive messages published to those topics
func (h *Hub) Subscribe(subscriberID string, topics ...Topic) <-chan interface{} {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Check if subscriber already exists
	if sub, exists := h.subscribersByID[subscriberID]; exists {
		// Add new topics to existing subscriber
		for _, topic := range topics {
			if !sub.Topics[topic] {
				sub.Topics[topic] = true
				h.subscribers[topic] = append(h.subscribers[topic], sub)
			}
		}
		return sub.Channel
	}

	// Create new subscriber
	ch := make(chan interface{}, h.bufferSize)
	topicMap := make(map[Topic]bool)

	subscriber := &Subscriber{
		ID:      subscriberID,
		Channel: ch,
		Topics:  topicMap,
		Created: time.Now(),
	}

	h.subscribersByID[subscriberID] = subscriber

	// Add to topic subscriptions
	for _, topic := range topics {
		topicMap[topic] = true
		h.subscribers[topic] = append(h.subscribers[topic], subscriber)
	}

	return ch
}

// Unsubscribe removes a subscriber from all topics
func (h *Hub) Unsubscribe(subscriberID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	subscriber, exists := h.subscribersByID[subscriberID]
	if !exists {
		return
	}

	// Remove from all topic lists
	for topic := range subscriber.Topics {
		subs := h.subscribers[topic]
		for i, sub := range subs {
			if sub.ID == subscriberID {
				// Remove from slice
				h.subscribers[topic] = append(subs[:i], subs[i+1:]...)
				break
			}
		}

		// Clean up empty topic lists
		if len(h.subscribers[topic]) == 0 {
			delete(h.subscribers, topic)
		}
	}

	// Close channel and remove subscriber
	close(subscriber.Channel)
	delete(h.subscribersByID, subscriberID)

	log.Printf("hub: Subscriber '%s' unregistered", subscriberID)
}

// Publish sends a message to all subscribers of a topic
// Uses non-blocking sends to avoid slow subscribers blocking the publisher
func (h *Hub) Publish(topic Topic, message interface{}) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	subscribers := h.subscribers[topic]
	if len(subscribers) == 0 {
		return // No subscribers, nothing to do
	}

	// Send to all subscribers
	for _, subscriber := range subscribers {
		select {
		case subscriber.Channel <- message:
			h.statsMu.Lock()
			h.messagesSent++
			h.statsMu.Unlock()
		default:
			// Channel full, drop message (non-blocking)
			h.statsMu.Lock()
			h.messagesDropped++
			h.dropsByTopic[topic]++
			h.statsMu.Unlock()
			// Could log this if drops become an issue
		}
	}
}

// PublishBlocking sends a message to all subscribers, blocking until all receive it
// Use this for critical messages that must not be dropped
func (h *Hub) PublishBlocking(topic Topic, message interface{}) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	subscribers := h.subscribers[topic]
	for _, subscriber := range subscribers {
		subscriber.Channel <- message
		h.statsMu.Lock()
		h.messagesSent++
		h.statsMu.Unlock()
	}
}

// GetControlChannel returns the channel for receiving control messages from clients
func (h *Hub) GetControlChannel() <-chan ControlMessage {
	return h.controlCh
}

// SendControl sends a control message (called by clients)
func (h *Hub) SendControl(msg ControlMessage) {
	msg.Timestamp = time.Now()
	select {
	case h.controlCh <- msg:
	default:
		log.Printf("hub: Control channel full, dropping message from %s", msg.SessionID)
	}
}

// GetStats returns hub statistics
func (h *Hub) GetStats() (sent, dropped uint64, activeSubscribers int) {
	h.statsMu.Lock()
	sent = h.messagesSent
	dropped = h.messagesDropped
	h.statsMu.Unlock()

	h.mu.RLock()
	activeSubscribers = len(h.subscribersByID)
	h.mu.RUnlock()

	return sent, dropped, activeSubscribers
}

// GetDropStats returns total drops and per-topic drop counts.
func (h *Hub) GetDropStats() (uint64, map[Topic]uint64) {
	h.statsMu.Lock()
	defer h.statsMu.Unlock()

	byTopic := make(map[Topic]uint64, len(h.dropsByTopic))
	for topic, count := range h.dropsByTopic {
		byTopic[topic] = count
	}
	return h.messagesDropped, byTopic
}

// GetSubscribers returns a list of active subscriber IDs
func (h *Hub) GetSubscribers() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	ids := make([]string, 0, len(h.subscribersByID))
	for id := range h.subscribersByID {
		ids = append(ids, id)
	}
	return ids
}

// GetTopics returns a list of all topics with active subscribers
func (h *Hub) GetTopics() []Topic {
	h.mu.RLock()
	defer h.mu.RUnlock()

	topics := make([]Topic, 0, len(h.subscribers))
	for topic := range h.subscribers {
		topics = append(topics, topic)
	}
	return topics
}
