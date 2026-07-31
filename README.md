# TuringSNN

A spiking neural network cortex, built from first principles in Go, compiled to WebAssembly and running live in-browser with no backend.

Live demo: [temper.ai/turing/app](https://temper.ai/turing/app), running this cortex engine in-browser. This repository includes only the built-in synthetic pattern generator (`internal/core_logic/sim`) as a sensor input, so it builds and runs standalone with no external dependencies or API keys.

## What's here

- **Three-layer spiking cortex** (`internal/core_logic/cortex`) with axonal delay modeling, feedback/resonance/zeitgeist connectivity between layers, and dopamine-modulated STDP learning.
- **Three-factor learning rule** (`cortex/layer.go`, `calculateWeights`): signed eligibility traces derived from spike-timing coincidence, accumulated across three timescales (short/mid/long, seconds-to-minutes half-lives), gated by a dopamine signal applied to the combined eligibility. This is a working implementation of neuromodulated STDP as described in the reward-modulated plasticity literature (Frémaux & Gerstner, *Neuromodulated Spike-Timing-Dependent Plasticity, and Theory of Three-Factor Learning Rules*, 2016) — nothing here is withheld; the code is presented as-is, including any rough edges.
- **Real-time chaos metrics** (`internal/chaos`): Lyapunov exponent, Hurst exponent, and wavelet analysis over the network's firing-rate dynamics, used to keep the network operating near the edge of chaos rather than collapsing into either silence or noise.
- **Pub/sub telemetry hub** (`internal/core_logic/hub`) decoupling the cortex from any particular consumer — the WASM bridge and a headless HTTP/WebSocket service (`internal/service`) both subscribe to it independently.
- **WASM bridge** (`internal/bridge`) exposing the cortex to JavaScript with no server round-trip — the entire simulation runs client-side.
- **Pluggable data sources**: the cortex's sensor input is supplied by anything implementing a two-method `WorldSource` interface (`internal/core_logic/backend.go`). This repository registers none beyond the built-in synthetic simulator; a deployment wiring in a live feed supplies its own factory at the outermost entry point.

## Building

Requires Go 1.25+.

```bash
# Native build (cmd/cortexd — headless HTTP/WebSocket daemon, sim-driven)
go build ./cmd/cortexd

# WASM build (cmd/wasm — the browser demo target)
GOOS=js GOARCH=wasm go build -o turing.wasm ./cmd/wasm
```

`go build ./...` and `go vet ./...` are both clean.

## Background

This is one piece of an independent research platform (temper.ai) begun in 2002, exploring spiking neural networks and distributed neuromorphic architectures as structurally different alternatives to centralized, transformer-based AI. Writing on the broader motivation — instruction/data conflation, synthetic feedback loops, and why distributed architectures resist some of the failure modes centralized ones don't — is at [temper.ai/articles](https://temper.ai/articles/).

## License

MIT — see [LICENSE](LICENSE).
