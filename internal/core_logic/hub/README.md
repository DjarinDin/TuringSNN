# Hub Package

The hub package provides a high-performance publish/subscribe message routing system that enables multiple clients (desktop GUI, web browsers, mobile apps, etc.) to connect to a single Turing SNN cortex instance.

## Overview

The hub acts as a central message broker between the cortex and multiple clients:

```
[Cortex] → [Hub] → [Desktop GUI]
            ↓
            → [Browser Client]
            ↓
            → [Mobile App]
            ↓
            → [API Client]
```

## Key Features

- **High Performance**: ~11M messages/sec single subscriber, ~635K messages/sec with 10 subscribers
- **Non-Blocking**: Slow subscribers don't block publishers or other subscribers
- **Type-Safe Topics**: Predefined topic constants for all cortex data streams
- **Session Management**: Track multiple users with permissions and view preferences
- **Zero Breaking Changes**: Integrates with existing channel-based architecture via adapters

## Architecture

### Components

1. **Hub**: Core pub/sub router
2. **SessionManager**: Multi-user session tracking
3. **CortexAdapter**: Bridges existing Go channels to hub

### Data Flow

```
Cortex Channels → Adapter → Hub → Subscribers
     ↓                              ↓
Unchanged code                  New clients
```

## Usage

### Basic Pub/Sub

```go
import "turing/internal/hub"

// Create hub
h := hub.NewHub()

// Subscribe to topics
ch := h.Subscribe("my-client",
    hub.TopicLayer1Potentials,
    hub.TopicLayer1Spikes,
)

// Receive messages
go func() {
    for msg := range ch {
        // Process message
        switch data := msg.(type) {
        case []float64:
            // Layer potentials
        case []bool:
            // Layer spikes
        }
    }
}()

// Publish from cortex
h.Publish(hub.TopicLayer1Potentials, potentials)
h.Publish(hub.TopicLayer1Spikes, spikes)

// Cleanup
h.Unsubscribe("my-client")
```

### Bridging Existing Channels

```go
// Bridge all cortex channels to hub (in main.go)
adapter := hub.NewCortexAdapter(h)

adapter.BridgeAllCortexChannels(
    simHeartbeatCh,
    cortexHeartbeatCh,
    rawSensorSpikesCh,
    // ... all other channels
)

// Now hub receives all cortex data automatically!
// Existing GUI code continues to work unchanged
```

### Session Management

```go
sm := hub.NewSessionManager()

// Create session for new client
session := sm.CreateSession("user123", hub.SessionTypeDesktop, "192.168.1.5:12345")

// Check permissions
if session.HasPermission("reset") {
    // Allow reset
}

// Track user focus
session.SetFocusNeuron(42, 2) // Neuron 42, Layer 2

// Get stats
sessions := sm.GetAllSessions()
fmt.Printf("Active users: %d\n", len(sessions))
```

### Control Messages

```go
// Client sends control message
h.SendControl(hub.ControlMessage{
    Type:      "dopamine",
    Value:     0.5,
    SessionID: "user123",
})

// Cortex receives control messages
controlCh := h.GetControlChannel()
go func() {
    for msg := range controlCh {
        switch msg.Type {
        case "reset":
            cortex.Reset()
        case "dopamine":
            cortex.SetDopamine(msg.Value.(float64))
        }
    }
}()
```

## Available Topics

### Heartbeats
- `TopicCortexHeartbeat` - Cortex tick events
- `TopicSimHeartbeat` - Simulation tick events

### Layer Data
- `TopicLayer1Potentials`, `TopicLayer1Spikes`
- `TopicLayer2Potentials`, `TopicLayer2Spikes`
- `TopicLayer3Potentials`, `TopicLayer3Spikes`

### Weights
- `TopicLayer1InputWeights`, `TopicLayer1FeedbackWeights`
- `TopicLayer2InputWeights`, `TopicLayer2FeedbackWeights`
- `TopicLayer3InputWeights`, `TopicLayer3FeedbackWeights`

### Traces
- `TopicLayer1InputTraces`, `TopicLayer1FeedbackTraces`, `TopicLayer1OutputTraces`
- (Similar for Layer2 and Layer3)

### Sensors
- `TopicSensorSpikes` - Processed sensor data
- `TopicRawSensorSpikes` - Raw sensor input
- `TopicRawSensorBlank` - Blank periods

### Outputs
- `TopicOutputA`, `TopicOutputB` - Output neuron states
- `TopicOutputAWeights`, `TopicOutputBWeights` - Output weights
- `TopicOutputAHistory`, `TopicOutputBHistory` - Historical data

### Stats
- `TopicCumulativeStats` - Cumulative statistics
- `TopicPerHeartbeatStats` - Per-heartbeat statistics

### Control
- `TopicControlReset` - Reset command
- `TopicControlDopamine` - Dopamine adjustment
- `TopicControlStep` - Single step
- `TopicControlFocus` - Focus neuron change

## Performance

Benchmarked on Intel Core i7-9750H @ 2.60GHz:

| Scenario | Throughput | Latency | Memory |
|----------|------------|---------|--------|
| Single subscriber | 11.5M msg/s | 87 ns | 24 B/op |
| 10 subscribers | 635K msg/s | 1.6 μs | 24 B/op |

## Design Decisions

### Non-Blocking Sends
The hub uses non-blocking sends to prevent slow subscribers from affecting the cortex or other clients. Messages are dropped if a subscriber's buffer is full.

**Rationale**: Real-time neural network visualization can tolerate occasional dropped frames. It's better to drop a message than to slow down the entire system.

### Buffered Channels
Default buffer size: 100 messages per subscriber.

**Rationale**: Balances memory usage with the ability to handle brief subscriber slowdowns.

### Type Safety
Topic constants are strongly typed (`Topic` type) to prevent typos.

**Rationale**: Compile-time safety for a critical communication system.

## Integration Strategy

The hub is designed for **gradual adoption**:

### Phase 1: Add Hub (Current)
- Hub runs alongside existing channels
- No changes to cortex or GUI code
- Adapter bridges channels to hub

### Phase 2: Optional Hub Mode
- Desktop app can run standalone OR connect to hub
- Controlled by command-line flag
- Backward compatible

### Phase 3: Multi-Client Support
- UDP server publishes hub data to network
- Browser clients connect via WebSocket
- Desktop and web coexist

### Phase 4: Hub-Native
- New features use hub directly
- Legacy channel code gradually migrated
- Full multi-user support

## Testing

```bash
# Run tests
go test ./internal/hub/

# Run benchmarks
go test -bench=. -benchmem ./internal/hub/

# Run with race detector
go test -race ./internal/hub/
```

## Future Enhancements

- [ ] Message replay buffer (for late-joining clients)
- [ ] Topic filters (subscribe to patterns like "layer*.potentials")
- [ ] Message priority levels
- [ ] Compression for high-bandwidth topics
- [ ] Metrics/monitoring dashboard
- [ ] Message persistence/logging

## See Also

- [Cortex Documentation](../cortex/README.md)
- [Network Protocol](../net/README.md) (coming in Phase 2)
- [Web Client](../../web/README.md) (coming in Phase 3)
