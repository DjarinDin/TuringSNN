# Simulation Package Documentation

The `sim` package provides a comprehensive simulation environment for generating various types of signals and time series data to be used as input for a spiking neural network cortex.

## Overview

The simulation system generates eight distinct signal sources that can be individually controlled, mixed, and sent to the cortex's sensor array. Each signal source has independent run/stop control and volume (probability) settings.

---

## Signal Sources

### 1. Noise Signal (`ControlNoiseRun` = 0, `ControlNoiseVolume` = 8)

**Purpose**: Generates random background noise to test the network's ability to distinguish signal from noise.

**Behavior**:
- Randomly selects a sensor position (0 to NumSensors-1) each tick
- Fires with probability based on volume level (0-10)
- No pattern - purely stochastic

**Use Cases**:
- Testing noise tolerance
- Creating baseline activity
- Simulating environmental interference

**Implementation**: `rand.Intn(int(conf.NumSensors))`

---

### 2. Signal 1 - Ramp Pattern (`ControlSignal1Run` = 1, `ControlSignal1Volume` = 9)

**Purpose**: Primary structured signal with a predictable ascending pattern.

**Behavior**:
- Generates a "ramp up" pattern across all sensors
- Pattern repeats every `SimSignalPatternLength` ticks
- Pattern: sensor positions increment sequentially (0, 1, 2, ..., NumSensors-1, 0, ...)
- **Default state**: ACTIVE (enabled by default)

**Use Cases**:
- Training the network on sequential patterns
- Testing temporal learning
- Establishing baseline response patterns

**Implementation**:
```go
signal1Pattern[SimSignalPatternLength-i-1] = (i % NumSensors)
signal1Pattern[simTickNum % SimSignalPatternLength]
```

---

### 3. Signal 2 - Stealth/Random Pattern (`ControlSignal2Run` = 2, `ControlSignal2Volume` = 10)

**Purpose**: Secondary structured signal with a pseudo-random but repeatable pattern.

**Behavior**:
- Pattern is randomly generated at initialization but remains constant during simulation
- Repeats every `SimSignalPatternLength` ticks
- **Default state**: INACTIVE (disabled by default)

**Use Cases**:
- Testing discrimination between multiple structured signals
- Creating competing patterns
- Simulating complex environmental signals

**Implementation**:
```go
signal2Pattern[i] = rand.Intn(int(NumSensors))
signal2Pattern[simTickNum % SimSignalPatternLength]
```

---

### 4. Random Walk Signal (`ControlRandomWalkRun` = 3, `ControlRandomWalkVolume` = 11)

**Purpose**: Continuous random walk across the sensor array with spatial continuity.

**Behavior**:
- Starts at the middle sensor (NumSensors / 2)
- Each tick: randomly moves -1, 0, or +1 positions
- Bounded: stays within [0, NumSensors-1]
- **Default state**: INACTIVE

**Use Cases**:
- Simulating moving stimuli
- Testing spatial tracking
- Creating smoother, more naturalistic input patterns

**Implementation**:
```go
move := rand.Intn(3) - 1  // -1, 0, or 1
randomWalkValue += move
// Clamp to [0, NumSensors-1]
```

---

### 5. Time Series Generator 0 - Random Walk (`ControlTSGen0Run` = 4, `ControlTSGen0Volume` = 12)

**Purpose**: Continuous-valued random walk mapped to sensor positions.

**Behavior**:
- Generates continuous floating-point values using Gaussian noise
- Each step: `value += rand.NormFloat64() * 0.01`
- Scaled and mapped to discrete sensor positions
- **Default state**: INACTIVE

**Generator Type**: `RandomWalk{}`

**Use Cases**:
- Brownian motion simulation
- Financial time series modeling
- Natural drift patterns

---

### 6. Time Series Generator 1 - Sine Wave (`ControlTSGen1Run` = 5, `ControlTSGen1Volume` = 13)

**Purpose**: Pure sinusoidal oscillation across the sensor array.

**Behavior**:
- Frequency: 1.0 Hz
- Amplitude: 1.0
- Formula: `amplitude * sin(2π * frequency * time)`
- Time step: 0.01 per tick
- **Default state**: INACTIVE

**Generator Type**: `SineWave{frequency: 1.0, amplitude: 1}`

**Use Cases**:
- Periodic pattern learning
- Oscillatory input testing
- Rhythm detection tasks

---

### 7. Time Series Generator 2 - Noisy Sine Wave (`ControlTSGen2Run` = 6, `ControlTSGen2Volume` = 14)

**Purpose**: Sinusoidal pattern with added Gaussian noise for realistic signal corruption.

**Behavior**:
- Base sine wave: frequency 1.0 Hz, amplitude 0.5
- Noise amplitude: 0.2 (added Gaussian noise)
- Formula: `sineWave.Next() + rand.NormFloat64() * noiseAmp`
- **Default state**: INACTIVE

**Generator Type**: `NoisySineWave{sineWave: SineWave{frequency: 1.0, amplitude: 0.5}, noiseAmp: 0.2}`

**Use Cases**:
- Testing robustness to noisy periodic signals
- Realistic sensor data simulation
- Signal extraction from noise

---

### 8. Time Series Generator 3 - Random Walk (Variant) (`ControlTSGen3Run` = 7, `ControlTSGen3Volume` = 15)

**Purpose**: Additional random walk generator for creating independent stochastic processes.

**Behavior**:
- Same behavior as Time Series Generator 0
- Provides independent random walk for multi-source scenarios
- **Default state**: INACTIVE

**Generator Type**: `RandomWalk{}`

**Use Cases**:
- Multi-source random processes
- Testing ability to track multiple independent signals
- Creating complex stochastic environments

---

## Control System

### Control Message Types

All signal sources are controlled via `comms.IntMsg` with the following structure:
- `Type`: Control message type (see constants)
- `Value`: Control value (0/1 for run state, 0-10 for volume)

### Run State Controls (0-7)

Enable/disable individual signal sources:
- **Value = 1**: Enable signal
- **Value = 0**: Disable signal

### Volume Controls (8-15)

Control signal firing probability (0-10 scale):
- **Volume = 0**: Never fires
- **Volume = 5**: Fires ~50% of eligible ticks
- **Volume = 10**: Fires ~100% of eligible ticks

**Probability Formula**: `rand.Intn(10) >= (10 - volumeLevel)`

### System Controls

- **ControlTickRate (90)**: Adjust simulation speed
  - Formula: `rate = value / SimTickRateMultiplier` milliseconds

- **ControlReset (99)**: Reset simulation to initial state
  - Resets tick counters
  - Regenerates signal patterns
  - Reinitializes all generators

---

## Time Series Scaling

All time series generators produce continuous floating-point values that are mapped to discrete sensor positions:

```go
scaledValue := int(math.Floor(value * float64(NumSensors) / 2)) + NumSensors/2
sensorPosition := max(0, min(scaledValue, NumSensors-1))
```

This ensures:
- Values are centered around the middle sensor
- All values are clamped to valid sensor range [0, NumSensors-1]
- Smooth mapping from continuous to discrete space

---

## Event-Driven Architecture

The simulation uses an event-driven model with three event types:

### TickEvent
- Fired by the simulation ticker at regular intervals
- Triggers `cycle()` to generate and send signals

### HeartbeatEvent
- Fired at `HeartbeatRate` intervals
- Sends heartbeat updates to GUI
- Increments heartbeat counter

### ControlEvent
- Fired when control messages are received
- Handles all user interactions and configuration changes
- Dispatches to appropriate handlers

---

## Integration

### Channels

**Input Channels**:
- `simControlCh chan comms.IntMsg`: Receives control commands from GUI

**Output Channels**:
- `cortexSensorCh chan int`: Sends sensor activations to cortex (sensor position 0-63)
- `simHeartbeatGUICh chan int`: Sends heartbeat updates to GUI

### Lifecycle

1. **Initialization**: `NewSim()` creates and configures the simulation
2. **Start**: `Run()` launches event listeners and generators
3. **Operation**: Continuously processes events and generates signals
4. **Reset**: `ControlReset` reinitializes without stopping

---

## Design Patterns

### Event Pattern
All external interactions are converted to events and processed through a single event channel, ensuring thread-safe operation and consistent handling.

### Strategy Pattern
The `TimeSeriesGenerator` interface allows pluggable signal generation strategies without modifying core simulation logic.

### Ticker Pattern
Independent tickers for simulation and heartbeat ensure decoupled timing concerns.

---

## Performance Characteristics

- **Tick Rate**: Configurable, default `RealTimeTickPeriod / SimTickRateMultiplier` ms
- **Heartbeat Rate**: Fixed at `conf.HeartbeatRate`
- **Signal Generation**: O(n) where n = number of active generators (max 8)
- **Memory**: Fixed allocation for patterns and generators

---

## Future Extensions

The architecture supports easy addition of new signal types:

1. Implement `TimeSeriesGenerator` interface
2. Add to `timeSeriesGenerators` slice in `initialize()`
3. Add corresponding control constants
4. Update GUI to expose new controls

Example generator types that could be added:
- Square wave
- Chirp signals (frequency sweeps)
- Markov chains
- Fractal noise (1/f noise)
- Burst patterns
- Bimodal distributions
