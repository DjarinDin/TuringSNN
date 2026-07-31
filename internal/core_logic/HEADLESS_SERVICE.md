# Cortex Headless Service (Remote GUI)

This note documents how to run the cortex as a standalone service and connect
the GUI over the network. The goal is **full process separation**: cortex in the
cloud, GUI on a local machine.

--------------------------------------------------------------------------------
ARCHITECTURE
--------------------------------------------------------------------------------

Process split:
- Cortex Service (headless): runs the backend, publishes telemetry to Hub topics,
  exposes control endpoints.
- GUI Client (browser): connects to WebSocket stream, sends controls over HTTP.

Core data flow:
  Cortex -> Hub topics -> WebSocket stream -> GUI
  GUI -> HTTP control endpoints -> Cortex/Sim control channels

--------------------------------------------------------------------------------
SERVICE ENTRYPOINT
--------------------------------------------------------------------------------

Binary:
- `cmd/cortexd/main.go`

Run locally:
  go run ./cmd/cortexd

Default ports:
- WebSocket stream: ws://localhost:8080/stream
- HTTP control:     http://localhost:8080/control
- Sim control:      http://localhost:8080/sim-control
- Health check:     http://localhost:8080/health

--------------------------------------------------------------------------------
GUI REMOTE MODE
--------------------------------------------------------------------------------

Use query flags to enable remote mode:

  /turing/index.html?remote=1

Override endpoints (optional):

  /turing/index.html?remote=1&ws=ws://HOST:8080/stream&http=http://HOST:8080

--------------------------------------------------------------------------------
CONTROL PAYLOADS
--------------------------------------------------------------------------------

HTTP POST /control (cortex commands):
{
  "channel": "cortex",
  "type": 4,
  "intValue": 0,
  "boolValue": false,
  "layer": 0,
  "cycle": 0
}

HTTP POST /sim-control (sim commands):
{
  "channel": "sim",
  "type": 1,
  "intValue": 1
}

Notes:
- `type` values mirror existing control enums in `internal/core_logic/sim/sim.go`
  and `pkg/comms/comms.go`.
- Cortex control `type` 6 clears the sensor accumulator (discard intra-tick arrivals).

Control enums (canonical source: `pkg/comms/comms.go`):
- Cortex: 0 tick rate, 1 dopamine, 2 pause, 3 focus, 4 step, 5 spontaneous, 6 reset accumulator, 7 reset, 8 soft reset,
  9 short trace half-life (ms), 10 mid trace half-life (ms), 11 long trace half-life (ms), 12 long eligibility gate (x1000)
- Sim: 0-7 run toggles, 8-15 volume, 80 run, 81 step, 82 cortex sync, 83 real data, 90 tick rate, 99 reset

--------------------------------------------------------------------------------
STREAM ENVELOPE (WS /stream)
--------------------------------------------------------------------------------

Each WebSocket message is a JSON envelope:
{
  "schemaVersion": 1,
  "topic": "layer1.spikes",
  "payloadType": "bool[]",
  "cycle": 1234,
  "payload": [...]
}

Notes:
- `cycle` is optional and only present when available (e.g., cortex tick or BoolArrayMsg).
- `payloadType` is a best-effort hint for decoding.
- Clients should warn (not fail) on unknown `payloadType` values within a schema version.

Known `payloadType` values (schema v1):
- Scalars: `bool`, `int`, `float64`, `string`
- Arrays: `bool[]`, `int[]`, `float32[]`, `float64[]`
- Structs: `string-msg`, `bool-msg`, `bool-array-msg`, `potentials-and-spikes`, `learning-state`
- Decision tracker: `decision-ledger`, `open-engagements`, `tracker-status`, `external-signal`
- Hub: `hub-drops` (per-topic drop counters)

`learning-state` fields include dopamine/adaptability plus trace tuning values:
- `ShortTraceHalfLifeSeconds`, `MidTraceHalfLifeSeconds`, `LongTraceHalfLifeSeconds`
- `LongEligibilityGate`
- `ResidualAbsMeanFocus`, `ResidualAbsMaxFocus` (focus neuron residual diagnostics)

--------------------------------------------------------------------------------
TROUBLESHOOTING
--------------------------------------------------------------------------------

- No updates in GUI:
  - Verify WebSocket URL and that `/stream` is reachable.
  - Check that the service is running and `/health` returns `ok`.

- Commands not applied:
  - Confirm HTTP base URL (`http=` query param).
  - Inspect server logs for control handler errors.
