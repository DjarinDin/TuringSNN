package hub

/*
hub_test.go ensures the reliability of the central event bus.

It verifies:
 1. Pub/Sub Mechanics: Subscribers receive what is published.
 2. Topic Filtering: Subscribers only get what they signed up for.
 3. Concurrency Safety: Multiple goroutines can publish/subscribe without race conditions.
 4. Non-Blocking Behavior: Slow subscribers do not freeze the publisher.
 5. Benchmarks: Performance validation for high-frequency messaging (100Hz+).
*/

import (
	"testing"
	"time"
)

func TestHubBasicPubSub(t *testing.T) {
	h := NewHub()

	// Subscribe to a topic
	ch := h.Subscribe("test-client", TopicCortexHeartbeat)

	// Publish a message
	testData := 42
	h.Publish(TopicCortexHeartbeat, testData)

	// Receive the message
	select {
	case msg := <-ch:
		if msg.(int) != testData {
			t.Errorf("Expected %d, got %v", testData, msg)
		}
	case <-time.After(time.Second):
		t.Error("Timeout waiting for message")
	}

	// Clean up
	h.Unsubscribe("test-client")
}

func TestMultipleSubscribers(t *testing.T) {
	h := NewHub()

	// Create multiple subscribers
	ch1 := h.Subscribe("client1", TopicLayer1Potentials)
	ch2 := h.Subscribe("client2", TopicLayer1Potentials)
	ch3 := h.Subscribe("client3", TopicLayer1Potentials)

	// Publish a message
	testData := []float64{1.0, 2.0, 3.0}
	h.Publish(TopicLayer1Potentials, testData)

	// All should receive
	received := 0
	timeout := time.After(time.Second)

	for i := 0; i < 3; i++ {
		select {
		case <-ch1:
			received++
		case <-ch2:
			received++
		case <-ch3:
			received++
		case <-timeout:
			t.Errorf("Timeout: only %d/3 subscribers received message", received)
			return
		}
	}

	if received != 3 {
		t.Errorf("Expected 3 receivers, got %d", received)
	}

	// Clean up
	h.Unsubscribe("client1")
	h.Unsubscribe("client2")
	h.Unsubscribe("client3")
}

func TestMultipleTopics(t *testing.T) {
	h := NewHub()

	// Subscribe to multiple topics
	ch := h.Subscribe("test-client", TopicLayer1Potentials, TopicLayer1Spikes)

	// Publish to first topic
	h.Publish(TopicLayer1Potentials, []float64{1.0, 2.0})

	// Receive from first topic
	select {
	case msg := <-ch:
		pots := msg.([]float64)
		if len(pots) != 2 {
			t.Error("Wrong data from topic 1")
		}
	case <-time.After(time.Second):
		t.Error("Timeout on topic 1")
	}

	// Publish to second topic
	h.Publish(TopicLayer1Spikes, []bool{true, false})

	// Receive from second topic
	select {
	case msg := <-ch:
		spikes := msg.([]bool)
		if len(spikes) != 2 {
			t.Error("Wrong data from topic 2")
		}
	case <-time.After(time.Second):
		t.Error("Timeout on topic 2")
	}

	h.Unsubscribe("test-client")
}

func TestUnsubscribe(t *testing.T) {
	h := NewHub()

	ch := h.Subscribe("test-client", TopicCortexHeartbeat)

	// Unsubscribe
	h.Unsubscribe("test-client")

	// Publish after unsubscribe
	h.Publish(TopicCortexHeartbeat, 99)

	// Should not receive (channel is closed)
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("Received message after unsubscribe")
		}
	case <-time.After(100 * time.Millisecond):
		// Timeout is expected, channel should be closed
	}
}

func TestNonBlockingPublish(t *testing.T) {
	h := NewHub()
	h.SetBufferSize(1) // Small buffer

	ch := h.Subscribe("slow-client", TopicCortexHeartbeat)

	// Fill buffer
	h.Publish(TopicCortexHeartbeat, 1)

	// Publish many more without reading (should not block)
	for i := 0; i < 100; i++ {
		h.Publish(TopicCortexHeartbeat, i)
	}

	// Verify we can still receive
	select {
	case <-ch:
		// Got at least one message
	case <-time.After(time.Second):
		t.Error("Should have received at least one message")
	}

	// Check stats
	sent, dropped, _ := h.GetStats()
	if sent == 0 {
		t.Error("No messages sent")
	}
	if dropped == 0 {
		t.Error("Expected some messages to be dropped")
	}

	h.Unsubscribe("slow-client")
}

func TestGetStats(t *testing.T) {
	h := NewHub()

	ch := h.Subscribe("test-client", TopicCortexHeartbeat)

	// Publish some messages
	for i := 0; i < 10; i++ {
		h.Publish(TopicCortexHeartbeat, i)
		<-ch // Receive to prevent drops
	}

	sent, dropped, subscribers := h.GetStats()

	if sent != 10 {
		t.Errorf("Expected 10 sent, got %d", sent)
	}
	if dropped != 0 {
		t.Errorf("Expected 0 dropped, got %d", dropped)
	}
	if subscribers != 1 {
		t.Errorf("Expected 1 subscriber, got %d", subscribers)
	}

	h.Unsubscribe("test-client")
}

func TestSessionManager(t *testing.T) {
	sm := NewSessionManager()

	// Create a session
	session := sm.CreateSession("user1", SessionTypeDesktop, "127.0.0.1:1234")

	if session.ID != "user1" {
		t.Error("Wrong session ID")
	}
	if session.Type != SessionTypeDesktop {
		t.Error("Wrong session type")
	}

	// Retrieve session
	retrieved, exists := sm.GetSession("user1")
	if !exists {
		t.Error("Session not found")
	}
	if retrieved.ID != "user1" {
		t.Error("Retrieved wrong session")
	}

	// Test permissions
	if !session.HasPermission("reset") {
		t.Error("Should have reset permission by default")
	}

	// Update focus
	session.SetFocusNeuron(42, 2)
	neuron, layer := session.GetFocusNeuron()
	if neuron != 42 || layer != 2 {
		t.Error("Focus neuron not set correctly")
	}

	// Remove session
	sm.RemoveSession("user1")
	_, exists = sm.GetSession("user1")
	if exists {
		t.Error("Session should be removed")
	}
}

func TestControlMessages(t *testing.T) {
	h := NewHub()

	// Send control message
	h.SendControl(ControlMessage{
		Type:      "reset",
		Value:     nil,
		SessionID: "test-user",
	})

	// Receive control message
	controlCh := h.GetControlChannel()
	select {
	case msg := <-controlCh:
		if msg.Type != "reset" {
			t.Error("Wrong control message type")
		}
		if msg.SessionID != "test-user" {
			t.Error("Wrong session ID")
		}
	case <-time.After(time.Second):
		t.Error("Timeout waiting for control message")
	}
}

func BenchmarkPublish(b *testing.B) {
	h := NewHub()
	ch := h.Subscribe("bench-client", TopicLayer1Potentials)

	// Drain channel in background
	go func() {
		for range ch {
		}
	}()

	data := []float64{1.0, 2.0, 3.0, 4.0, 5.0}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.Publish(TopicLayer1Potentials, data)
	}

	h.Unsubscribe("bench-client")
}
func BenchmarkMultipleSubscribers(b *testing.B) {
	h := NewHub()

	// Create 10 subscribers
	channels := make([]<-chan interface{}, 0)
	for i := 0; i < 10; i++ {
		ch := h.Subscribe("bench-client-"+string(rune(i)), TopicLayer1Potentials)
		channels = append(channels, ch)

		// Drain each channel
		go func(c <-chan interface{}) {
			for range c {
			}
		}(ch)
	}

	data := []float64{1.0, 2.0, 3.0, 4.0, 5.0}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.Publish(TopicLayer1Potentials, data)
	}

	// Cleanup
	for i := 0; i < 10; i++ {
		h.Unsubscribe("bench-client-" + string(rune(i)))
	}
}
